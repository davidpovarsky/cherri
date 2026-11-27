#!/usr/bin/env python3
"""Preparation-only process observations; no build, Simulator or XCTest execution."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import signal
import stat
import sys
import threading
import time
import types


PRODUCTION_BASE = "94efcf1c1eaff379f5ae2b38d111998d57b233bc"
PROBE_REF = "refs/heads/probe/cherri-p10r15-rp39-preparation-20261127"
WORKFLOW = ".github/workflows/p10r15-rp39-preparation.yml"
CONTROLLER = (WORKFLOW, "tools/ci/rp39_preparation/run.py",
              "tools/ci/rp39_preparation/source-profile.json",
              "tools/ci/rp39_preparation/test_run.py")
SCHEMA = "cherri-rp39-preparation-diagnostic/1"
READ_SECONDS = 15
CLEANUP_RESERVE = 2
PROBE_SECONDS = 120
SEAL_SECONDS = 300
MAX_SAMPLES = 160
MAX_PRIVATE_BYTES = 16 * 1024 * 1024
MAX_PRIVATE_FILES = 256
LIVE_FLOOR = 3 * 1024 * 1024 * 1024


def require(condition, code):
    if not condition:
        raise ValueError(code)


def canonical(value):
    return (json.dumps(value, sort_keys=True, indent=2, allow_nan=False) + "\n").encode()


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def read(path, limit=1024 * 1024):
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and 0 <= before.st_size <= limit
            and path.resolve() == path, "INPUT_FILE")
    raw = path.read_bytes()
    identity = lambda row: (row.st_dev, row.st_ino, row.st_mode, row.st_size,
                            row.st_mtime_ns, row.st_ctime_ns)
    require(len(raw) == before.st_size and identity(path.lstat()) == identity(before), "INPUT_CHANGED")
    return raw


def write(path, value):
    raw = canonical(value)
    require(len(raw) <= 1024 * 1024, "METADATA_LIMIT")
    with path.open("xb") as handle:
        handle.write(raw)
    return {"bytes": len(raw), "sha256": sha(raw)}


def module_from_bytes(name, path, raw):
    module = types.ModuleType(name)
    module.__file__ = str(path)
    module.__package__ = ""
    exec(compile(raw, str(path), "exec"), module.__dict__)
    return module


def context(env):
    require(env.get("GITHUB_REPOSITORY") == "davidpovarsky/cherri"
            and env.get("GITHUB_WORKFLOW") == "RP39 preparation diagnosis"
            and env.get("GITHUB_EVENT_NAME") == "push"
            and env.get("GITHUB_REF") == PROBE_REF
            and env.get("RUNNER_OS") == "macOS", "CONTEXT")
    source = env.get("GITHUB_SHA", "")
    require(re.fullmatch(r"[0-9a-f]{40}", source) is not None
            and env.get("GITHUB_WORKFLOW_SHA") == source, "SOURCE_CONTEXT")
    run, attempt = env.get("GITHUB_RUN_ID", ""), env.get("GITHUB_RUN_ATTEMPT", "")
    require(re.fullmatch(r"[1-9][0-9]{0,19}", run) is not None
            and re.fullmatch(r"[1-9][0-9]{0,8}", attempt) is not None, "RUN_CONTEXT")
    return {"sourceSha": source, "productionBase": PRODUCTION_BASE,
            "runId": int(run), "attempt": int(attempt),
            "repository": "davidpovarsky/cherri", "workflowPath": WORKFLOW,
            "workflowName": "RP39 preparation diagnosis", "event": "push"}


def directories(repo, ctx):
    root = Path(os.environ["RUNNER_TEMP"]).resolve(strict=True) / (
        "cherri-rp39-preparation-" + str(ctx["runId"]) + "-" + str(ctx["attempt"])
        + "-" + ctx["sourceSha"])
    require(repo not in root.parents and root != repo, "PRIVATE_ROOT")
    return root


def inputs(repo):
    profile_path = repo / "tools/ci/rp39_preparation/source-profile.json"
    profile = json.loads(read(profile_path))
    require(profile["schema"] == "cherri-rp39-preparation-source-profile/1"
            and profile["productionBase"] == PRODUCTION_BASE
            and len(profile["files"]) == 33
            and profile["diagnosticOnly"] is True
            and profile["nativeAcceptanceGranted"] is False, "SOURCE_PROFILE")
    originals = {}
    for name, recorded in profile["files"].items():
        require(not name.startswith("/") and ".." not in Path(name).parts, "SOURCE_PATH")
        path = repo / name
        raw = read(path)
        observed = {"bytes": len(raw), "sha256": sha(raw),
                    "gitBlob": hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest(),
                    "mode": "100755" if path.stat().st_mode & 0o111 else "100644"}
        require(observed == recorded, "SOURCE_PROFILE_CHANGED")
        originals[name] = raw
    controller = {}
    for name in CONTROLLER:
        raw = read(repo / name)
        controller[name] = {"bytes": len(raw), "sha256": sha(raw),
                            "mode": stat.S_IMODE((repo / name).stat().st_mode)}
    return profile, originals, controller


class ProcessSamples:
    """Read only the owned PID/group, without replacing capture's cleanup."""
    def __init__(self, image, deadline):
        self.image, self.deadline = image, deadline
        self.pid = None
        self.rows = []
        self.stop_event = threading.Event()
        self.thread = None
        self.started = time.monotonic()

    def sample(self, phase):
        if len(self.rows) >= MAX_SAMPLES or self.pid is None:
            return
        try:
            pgid = os.getpgid(self.pid)
        except ProcessLookupError:
            pgid = None
        except OSError:
            self.rows.append({"phase": phase, "elapsedMs": round((time.monotonic() - self.started) * 1000),
                              "pid": self.pid, "state": "QUERY_FAILED"})
            return
        # No arguments, other PIDs, descendants or unrelated process list are read.
        image = self.image(self.pid) if pgid == self.pid else {"state": "NOT_OBSERVED", "path": None}
        self.rows.append({"phase": phase, "elapsedMs": round((time.monotonic() - self.started) * 1000),
                          "pid": self.pid, "pgid": pgid, "ownedGroupMatched": pgid == self.pid,
                          "image": image})

    def start(self, pid):
        require(self.pid is None and type(pid) is int and pid > 1, "SAMPLER_PID")
        self.pid = pid
        self.sample("started")
        def observe():
            while not self.stop_event.wait(0.1):
                if time.monotonic() >= self.deadline or len(self.rows) >= MAX_SAMPLES - 1:
                    break
                self.sample("during")
        self.thread = threading.Thread(target=observe, name="owned-process-image", daemon=True)
        self.thread.start()

    def stop(self):
        self.stop_event.set()
        if self.thread is not None:
            self.thread.join(timeout=max(0, min(0.5, self.deadline - time.monotonic())))
        joined = self.thread is None or not self.thread.is_alive()
        if joined:
            self.sample("finished")
        return {"samples": self.rows, "samplerJoined": joined, "maximumSamples": MAX_SAMPLES,
                "startedMonotonic": self.started,
                "sampleIntervalSeconds": 0.1,
                "initialImageIsNotResidencyProof": True,
                "pidGroupObservationOnly": True}


def observed_capture(capture, image, destination, command, allowance, **kwargs):
    """Add private image observations after Popen returned; preserve all original journals."""
    sampler = ProcessSamples(image, kwargs["hard_deadline"])
    original_journal = kwargs.get("journal")
    def journal(phase, detail):
        if original_journal is not None:
            original_journal(phase, detail)
        if phase == "started":
            sampler.start(detail["pid"])
    kwargs["journal"] = journal
    # Use the deferral primitive already imported by the authenticated capture.
    # Its absolute command deadline still bounds cancellation deferral; a first
    # signal cannot interrupt capture's finally or the sampler's final retention.
    with capture.__globals__["defer_cancellation_signals"]():
        try:
            return capture(command, allowance, **kwargs)
        finally:
            observation = sampler.stop()
            write(destination, observation)
            require(observation["samplerJoined"], "SAMPLER_NOT_JOINED")


def public_summary(result):
    return {"schema": SCHEMA, "context": result["context"],
            "status": result["status"], "diagnosticOnly": True,
            "formalAcceptanceGranted": False, "nativeCommandsInvoked": 0,
            "cases": [{key: row.get(key) for key in ("name", "status", "elapsedMs", "reason")}
                      for row in result["cases"]]}


def probe(repo, root, ctx, base, profile, originals, controller):
    root.mkdir(mode=0o700)
    private = root / "private"
    private.mkdir(mode=0o700)
    started = time.monotonic()
    whole_deadline = started + PROBE_SECONDS
    result = {"schema": SCHEMA, "context": ctx, "status": "INCOMPLETE", "cases": [],
              "diagnosticOnly": True, "formalAcceptanceGranted": False,
              "nativeCommandsInvoked": 0, "sourceBefore": profile["files"],
              "controllerBefore": controller, "python": {"executable": sys.executable,
              "realExecutable": str(Path(sys.executable).resolve()), "version": sys.version},
              "platform": {"system": os.uname().sysname, "release": os.uname().release,
                           "machine": os.uname().machine},
              "budgets": {"probeSeconds": PROBE_SECONDS, "perReadSeconds": READ_SECONDS,
                          "perReadCleanupReserveSeconds": CLEANUP_RESERVE},
              "nativeAcceptanceCommandExecuted": False}
    write(private / "context.original.json", result)
    owner = None
    try:
        with base.envelope(PROBE_SECONDS):
            available = os.statvfs(root)
            require(available.f_bavail * available.f_frsize >= LIVE_FLOOR
                    + 2 * MAX_PRIVATE_BYTES + 16 * 1024 * 1024, "PROBE_CAPACITY")
            owner = base.OwnedGitReads(repo, private / "original-owned-source-binding")
            actual_capture = owner.helper.capture
            capture_count = 0
            def sampled(command, allowance, **kwargs):
                nonlocal capture_count
                capture_count += 1
                return observed_capture(actual_capture, owner.process_image,
                    private / ("process-samples-%02d.original.json" % capture_count),
                    command, allowance, **kwargs)
            owner.helper.capture = sampled
            base._owned_git = owner
            case_start = time.monotonic()
            first = {"name": "original_source_binding", "status": "INCOMPLETE"}
            result["cases"].append(first)
            try:
                binding = base.source_binding(repo, ctx["sourceSha"], min(whole_deadline, case_start + READ_SECONDS))
                write(private / "source-binding.original.json", binding)
                first["status"] = "PASS"
            except Exception as error:
                first.update(status="REJECTED", reason=type(error).__name__)
            first["elapsedMs"] = round((time.monotonic() - case_start) * 1000)
            require(all(item["capture"] is not None and item["capture"]["cleanupParentReaped"] is True
                        and item["capture"]["groupAfterCleanup"]["state"] == "ABSENT"
                        for item in owner.records), "ORIGINAL_GROUP_UNCLOSED")
            write(private / "original-owned-source-binding.original.json", owner.records)
            base._owned_git = None

            def direct(name, argv, *, seconds=READ_SECONDS, shared_deadline=None, record_case=True):
                directory = private / name
                directory.mkdir(mode=0o700)
                start = time.monotonic()
                deadline = min(start + seconds, whole_deadline,
                               whole_deadline if shared_deadline is None else shared_deadline)
                require(deadline - start > CLEANUP_RESERVE and base._first_interruption is None,
                        "NO_NEW_CHILD_AFTER_DEADLINE_OR_INTERRUPTION")
                row = {"name": name, "argv": argv, "status": "INCOMPLETE"}
                (result["cases"] if record_case else result.setdefault("closingCommands", [])).append(row)
                write(directory / "intent.original.json", row)
                def journal(phase, detail):
                    write(directory / (phase + ".original.json"), detail)
                with (directory / "stdout.original").open("xb") as stdout, \
                        (directory / "stderr.original").open("xb") as stderr:
                    observed, detail = observed_capture(actual_capture, owner.process_image,
                        directory / "process-samples.original.json", argv,
                        deadline - start - CLEANUP_RESERVE, output=stdout, error_output=stderr,
                        execution_deadline=deadline - CLEANUP_RESERVE, hard_deadline=deadline,
                        stream_limit=base.METADATA_LIMIT, journal=journal)
                row.update(status=observed["result"], elapsedMs=observed["elapsedMs"],
                           observed=observed, capture=detail)
                write(directory / "result.original.json", row)
                require(detail["cleanupParentReaped"] is True
                        and detail["groupAfterCleanup"]["state"] == "ABSENT", "DIRECT_GROUP_UNCLOSED")
                return row, read(directory / "stdout.original", base.METADATA_LIMIT)

            head_args = ["--no-replace-objects", "-C", str(repo), "rev-parse", "--verify", "HEAD"]
            inherited, raw = direct("inherited_environment_apple_git", ["/usr/bin/git", *head_args])
            if inherited["status"] == "PASS":
                require(raw.strip().decode() == ctx["sourceSha"], "INHERITED_HEAD_MISMATCH")
            located, raw = direct("find_selected_git", ["/usr/bin/xcrun", "--find", "git"])
            if located["status"] == "PASS":
                value = raw.decode("utf-8").removesuffix("\n")
                path = Path(value)
                require(len(raw) <= 4096 and path.is_absolute() and path.is_file()
                        and not any(ord(c) < 32 or ord(c) == 127 for c in value), "SELECTED_GIT_PATH")
                result["selectedGit"] = {"path": value, "realPath": str(path.resolve()),
                                         "bytes": path.stat().st_size,
                                         "sha256": sha(read(path.resolve(), 128 * 1024 * 1024))}
                env = {k: os.environ[k] for k in ("PATH", "HOME", "LANG", "LC_ALL", "TMPDIR", "DEVELOPER_DIR") if k in os.environ}
                env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_OPTIONAL_LOCKS="0", GIT_TERMINAL_PROMPT="0")
                if not env.get("DEVELOPER_DIR") and owner.selected_developer_directory:
                    env["DEVELOPER_DIR"] = owner.selected_developer_directory
                selected, raw = direct("sanitized_selected_git", ["/usr/bin/env", "-i",
                    *[k + "=" + v for k, v in sorted(env.items())], value, *head_args])
                if selected["status"] == "PASS":
                    require(raw.strip().decode() == ctx["sourceSha"], "SELECTED_HEAD_MISMATCH")
            else:
                result["cases"].append({"name": "sanitized_selected_git", "status": "NOT_RUN",
                                        "reason": "SELECTED_GIT_NOT_OBSERVED"})
            profile_after, originals_after, controller_after = inputs(repo)
            require(profile_after == profile and originals_after == originals
                    and controller_after == controller, "CLOSING_PHYSICAL_SOURCE")
            source_profile = module_from_bytes("_diagnostic_source_profile",
                repo / "ios/scripts/record_workspace_presentation_source.py",
                originals["ios/scripts/record_workspace_presentation_source.py"])
            closing_count, closing_deadline = 0, None
            def closing_git(target, args, deadline):
                nonlocal closing_count, closing_deadline
                require(target == repo, "CLOSING_REPO")
                closing_deadline = deadline if closing_deadline is None else closing_deadline
                require(deadline == closing_deadline and closing_count < 8, "CLOSING_COMMAND_SET")
                closing_count += 1
                row, raw = direct("closing-git-%02d" % closing_count,
                    ["/usr/bin/env", "GIT_OPTIONAL_LOCKS=0", "/usr/bin/git", "--no-replace-objects",
                     "-C", str(repo), *args], seconds=10, shared_deadline=deadline, record_case=False)
                require(row["status"] == "PASS", "CLOSING_GIT_READ")
                return raw
            # Preserve source_snapshot's original predicates and20s deadline;
            # replace only its subprocess transport with owned10s reads.
            source_profile.git = closing_git
            closing = source_profile.source_snapshot(repo, {"source": ctx["sourceSha"]},
                                                       source_profile.RP39_SOURCE_FILES)
            write(private / "closing-git-source.original.json", closing)
            require(closing["files"] == profile["files"], "CLOSING_GIT_SOURCE")
            raw_tree = closing_git(repo, ["ls-tree", "-r", "-z", ctx["sourceSha"], "--", *CONTROLLER], closing_deadline)
            controller_tree = {}
            for raw_row in raw_tree.split(b"\0"):
                if not raw_row:
                    continue
                metadata, name = raw_row.split(b"\t", 1)
                mode, kind, blob = metadata.decode().split()
                name = name.decode()
                require(name not in controller_tree and kind == "blob" and mode in ("100644", "100755"),
                        "CONTROLLER_GIT_ROW")
                controller_tree[name] = {"mode": mode, "gitBlob": blob}
            require(set(controller_tree) == set(CONTROLLER), "CONTROLLER_GIT_SET")
            for name, recorded in controller_tree.items():
                raw = read(repo / name)
                expected_mode = 0o755 if recorded["mode"] == "100755" else 0o644
                require(hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
                        == recorded["gitBlob"] and controller[name] ==
                        {"bytes": len(raw), "sha256": sha(raw), "mode": expected_mode}, "CONTROLLER_GIT_BINDING")
            require(not closing_git(repo, ["status", "--porcelain", "--untracked-files=no", "--", *CONTROLLER],
                                    closing_deadline).strip(), "CONTROLLER_STATUS")
            require(closing_git(repo, ["rev-parse", "HEAD"], closing_deadline).strip().decode()
                    == ctx["sourceSha"], "CLOSING_HEAD")
            write(private / "closing-controller-git.original.json", controller_tree)
            result.update(status="OBSERVATIONS_RETAINED", sourceAfter=closing,
                          controllerAfter=controller_after, controllerGitAfter=controller_tree)
    except (Exception, base.EnvelopeExpired) as error:
        result.update(status="REJECTED", failureType=type(error).__name__)
    finally:
        base._owned_git = None
        result["elapsedMs"] = round((time.monotonic() - started) * 1000)
        write(private / "result.original.json", result)
        write(root / "summary.original.json", public_summary(result))
    print(json.dumps(public_summary(result), sort_keys=True))
    return 0 if result["status"] == "OBSERVATIONS_RETAINED" else 70


def seal(repo, root, ctx, base):
    helper_path = repo / "ios/scripts/seal_rp39_diagnostic.py"
    helper_raw = read(helper_path)
    require(sha(helper_raw) == "76f28b1d0c5b46997f5a52d7e9639a2a05a468b1201e1453e1b2e206f8db7d35", "SEAL_PIN")
    helper = module_from_bytes("_preparation_seal", helper_path, helper_raw)
    private, destination = root / "private", root / "sealed"
    require(private.is_dir() and not private.is_symlink(), "PRIVATE_REQUIRED")
    destination.mkdir(mode=0o700)
    started = time.monotonic()
    with base.envelope(SEAL_SECONDS):
        members = {}
        logical = 0
        for path in sorted(private.rglob("*")):
            require(not path.is_symlink(), "PRIVATE_LINK")
            if path.is_dir():
                continue
            require(len(members) < MAX_PRIVATE_FILES, "PRIVATE_COUNT")
            name = path.relative_to(private).as_posix()
            require(len(name.encode()) <= 1024 and ".." not in Path(name).parts, "PRIVATE_NAME")
            record = helper.file_record(path, MAX_PRIVATE_BYTES)
            logical += record["bytes"]
            require(logical <= MAX_PRIVATE_BYTES, "PRIVATE_BYTES")
            members[name] = {"path": path, "original": record}
        require("context.original.json" in members and "result.original.json" in members, "PRIVATE_TERMINAL")
        require(json.loads(read(private / "result.original.json"))["context"] == ctx, "PRIVATE_CONTEXT")
        statv = os.statvfs(root)
        require(statv.f_bavail * statv.f_frsize >= LIVE_FLOOR + 2 * logical + 16 * 1024 * 1024,
                "SEAL_CAPACITY")
        def guard():
            current = os.statvfs(root)
            require(time.monotonic() < started + SEAL_SECONDS
                    and current.f_bavail * current.f_frsize >= LIVE_FLOOR, "SEAL_DEADLINE_OR_CAPACITY")
        archive = destination / "preparation-originals.zip"
        identity = {"context": ctx, "scope": "preparation-only-no-native",
                    "formalAcceptanceGranted": False, "nativeCommandsInvoked": 0}
        archive_record, manifest = helper.make_archive(archive, members, identity, guard=guard,
            manifest_schema="cherri-native-original-diagnostics/1")
        stage = destination / "public-stage"
        stage.mkdir(mode=0o700)
        capture = base.load_primitives(repo, None)
        raw_capture = capture.capture
        def protected_encryption_capture(*args, **kwargs):
            with capture.defer_cancellation_signals():
                return raw_capture(*args, **kwargs)
        capture.capture = protected_encryption_capture
        cipher, process = helper.encrypt_archive(destination, stage, archive, archive_record,
            repo / base.RECIPIENT, capture, started + SEAL_SECONDS)
        guard()
        require(all(helper.file_record(row["path"], MAX_PRIVATE_BYTES) == row["original"]
                    for row in members.values()), "SEAL_ORIGINAL_CHANGED")
        receipt = {"schema": SCHEMA, "context": ctx, "formalAcceptanceGranted": False,
                   "nativeCommandsInvoked": 0, "plaintextArchive": {k: archive_record[k] for k in ("bytes", "sha256")},
                   "ciphertext": {k: cipher[k] for k in ("bytes", "sha256")},
                   "manifest": {"bytes": len(helper.base.canonical(manifest)),
                                "sha256": sha(helper.base.canonical(manifest))},
                   "recipientSha256": base.RECIPIENT_SHA256,
                   "memberCount": len(members) + 1, "encryptionProcess": process,
                   "elapsedMs": round((time.monotonic() - started) * 1000)}
        write(stage / "receipt.json", receipt)
        require({p.name for p in stage.iterdir()} == {"rp39-originals.zip.p7m", "receipt.json"}, "PUBLIC_SET")
        stage.rename(root / "public")
    print("RP39_PREPARATION_ORIGINALS_SEALED")


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("mode", choices=("probe", "seal"))
    args = parser.parse_args()
    require(sys.flags.isolated == 1 and sys.platform == "darwin", "INTERPRETER")
    ctx = context(os.environ)
    repo = Path(os.environ["GITHUB_WORKSPACE"]).resolve(strict=True)
    profile, originals, controller = inputs(repo)
    base = module_from_bytes("_rp39_preparation_collector", repo / "ios/scripts/collect_ios_xcresult_summary.py",
                             originals["ios/scripts/collect_ios_xcresult_summary.py"])
    root = directories(repo, ctx)
    if args.mode == "probe":
        return probe(repo, root, ctx, base, profile, originals, controller)
    seal(repo, root, ctx, base)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
