#!/usr/bin/env python3
"""Run the original R22 gate once and retain its bounded child originals.

The existing RuntimePoC workflow owns the single five-fixture invocation.
This observer does not substitute a prior PoC result or grant other gates.
"""
import argparse
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import sys
import time
import types
import zipfile

GATE_PATH = "tools/p10r3/run_release_gates.py"
GATE_SHA = "9cba82f54ec4fa6715fb8fe7b38c3fa238f96fd45d8ba12d5eb1cedf112bacf3"
SCRIPT_PATH = "scripts/ios27_runtime_poc.sh"
SCRIPT_SHA = "06a2225c0b13ef85a1701237ef6ea97ab5ed0480623f2ccfe0800dd82f8b6e1a"
QA = "tools/shortcuts-simulator-qa"
SCHEMA = "cherri-original-r22-observation/1"
WHOLE_SECONDS = 3900
NATIVE_SECONDS = 3600
FINAL_RESERVE = 120
STREAM_LIMIT = 2 * 1024 * 1024
ARCHIVE_LIMIT = 9 * 1024 * 1024
PRIVATE_LIMIT = 24 * 1024 * 1024
MAX_FILES = 10000
SIMCTL = ["xcrun", "simctl", "list", "runtimes"]
NATIVE = ["bash", SCRIPT_PATH]


class Rejected(ValueError):
    pass


class OwnerInterrupted(BaseException):
    pass


def require(ok, code):
    if not ok:
        raise Rejected(code)


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"),
                       ensure_ascii=True, allow_nan=False) + "\n").encode()


def write_original(path, raw):
    path = Path(path)
    require(not path.is_symlink(), "ORIGINAL_SYMLINK")
    with path.open("xb") as stream:
        os.chmod(path, 0o600)
        stream.write(raw)
        stream.flush()
        os.fsync(stream.fileno())
    require(path.read_bytes() == raw, "ORIGINAL_READBACK")


def record(path, value):
    write_original(path, canonical(value))


def context_from_environment(env):
    require(env.get("GITHUB_ACTIONS") == "true"
            and env.get("GITHUB_REPOSITORY") == "davidpovarsky/cherri"
            and env.get("GITHUB_WORKFLOW") == "iOS 27 Shortcuts Runtime PoC",
            "WORKFLOW_CONTEXT")
    source, run, attempt = (env.get(k, "") for k in
                            ("GITHUB_SHA", "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT"))
    require(re.fullmatch(r"[0-9a-f]{40}", source) is not None
            and re.fullmatch(r"[1-9][0-9]{0,19}", run) is not None
            and re.fullmatch(r"[1-9][0-9]{0,3}", attempt) is not None,
            "WORKFLOW_IDENTITY")
    return {"sourceCommitSha": source, "runId": run, "runAttempt": int(attempt)}


def load_gate(repo):
    path = repo / GATE_PATH
    require(digest(path.read_bytes()) == GATE_SHA
            and digest((repo / SCRIPT_PATH).read_bytes()) == SCRIPT_SHA,
            "ORIGINAL_GATE_OR_SCRIPT_CHANGED")
    sys.path.insert(0, str(path.parent))
    spec = importlib.util.spec_from_file_location("original_r22_release_gate", path)
    module = importlib.util.module_from_spec(spec)
    sys.modules[spec.name] = module
    spec.loader.exec_module(module)
    require(module.REPO_ROOT.resolve() == repo, "ORIGINAL_GATE_ROOT")
    return module


def capture_pass(row):
    details = row["capture"]
    return (row["result"] == "PASS" and row["exitCode"] == 0
            and details["cleanupParentReaped"]
            and details["groupAfterCleanup"]["state"] == "ABSENT"
            and all(details[k]["eof"] and not details[k]["overflow"]
                    for k in ("stdout", "stderr")))


class OriginalGateChildren:
    """A closed adapter for the two commands called by the original gate."""
    def __init__(self, repo, private, capture, deadline):
        self.repo, self.private, self.capture = repo, private, capture
        self.deadline = deadline
        self.rows = []

    def run(self, argv, **kwargs):
        expected = SIMCTL if not self.rows else NATIVE
        require(len(self.rows) < 2 and argv == expected, "GATE_CHILD_ROUTE")
        require(set(kwargs) == {"cwd", "capture_output", "text", "encoding", "errors"}
                and Path(kwargs["cwd"]).resolve() == self.repo
                and kwargs["capture_output"] is True and kwargs["text"] is True
                and kwargs["encoding"] == "utf-8" and kwargs["errors"] == "replace",
                "ORIGINAL_CHILD_OPTIONS")
        label = "01-platform" if not self.rows else "02-native-poc"
        allowance = 15 if not self.rows else NATIVE_SECONDS
        require(time.monotonic() + allowance + FINAL_RESERVE < self.deadline,
                "CHILD_BUDGET_UNAVAILABLE")
        start = time.monotonic()
        intent = {"argv": argv, "cwd": str(self.repo), "stdin": "DEVNULL",
                  "originalOptions": {k: str(v) if isinstance(v, Path) else v
                                      for k, v in kwargs.items()},
                  "allowanceSeconds": allowance, "startedMonotonic": start}
        record(self.private / (label + ".intent.original.json"), intent)
        row = {"label": label, "argv": argv, "result": "NOT_RETURNED", "exitCode": None,
               "spawned": False, "pid": None}
        self.rows.append(row)  # No second launch if capture or journaling raises.
        def journal(phase, details):
            if phase == "started":
                row.update(spawned=True, pid=details["pid"])
            record(self.private / (label + "." + phase + ".original.json"), details)
        stdout_path = self.private / (label + ".stdout.original")
        stderr_path = self.private / (label + ".stderr.original")
        try:
            with stdout_path.open("xb") as stdout, stderr_path.open("xb") as stderr:
                observed, details = self.capture(
                    argv, allowance, output=stdout, error_output=stderr,
                    execution_deadline=start + allowance,
                    hard_deadline=min(self.deadline - FINAL_RESERVE, start + allowance + 5),
                    stream_limit=STREAM_LIMIT, journal=journal)
            row.update(observed, capture=details)
        finally:
            record(self.private / (label + ".process.original.json"), row)
        # A completed nonzero child remains the original gate's own decision.
        require(row["result"] in {"PASS", "EXIT_NONZERO", "OWNED_GROUP_REMAINED"}
                and type(row["exitCode"]) is int, "CHILD_CAPTURE_INCOMPLETE")
        stdout, stderr = stdout_path.read_bytes(), stderr_path.read_bytes()
        require(len(stdout) <= STREAM_LIMIT and len(stderr) <= STREAM_LIMIT,
                "CHILD_STREAM_BOUND")
        return subprocess.CompletedProcess(argv, row["exitCode"],
            stdout.decode("utf-8", errors="replace"), stderr.decode("utf-8", errors="replace"))


def invoke_original(gate, children):
    original_dependency = gate.subprocess
    gate.subprocess = types.SimpleNamespace(run=children.run)
    try:
        # Execute the actual source-owned gate, using its real host platform.
        return gate.execute_gate("R22", gate.GATE_DEFINITIONS["R22"])
    finally:
        gate.subprocess = original_dependency


def verify_gate_row(row, children, private):
    require(isinstance(row, dict) and row.get("id") == "R22", "GATE_ROW")
    require(row.get("status") == "PASS" and row.get("exitCode") == 0,
            "ORIGINAL_GATE_NOT_PASS")
    require(len(children.rows) == 2 and all(capture_pass(r) for r in children.rows),
            "CHILD_PROCESS_NOT_COMPLETE")
    for child in children.rows:
        for name in ("stdout", "stderr"):
            raw = (private / (child["label"] + "." + name + ".original")).read_bytes()
            captured = child["capture"][name]
            require(len(raw) == captured["bytes"] and digest(raw) == captured["sha256"],
                    "CHILD_ORIGINAL_HASH_CHANGED")
    stdout = (private / "02-native-poc.stdout.original").read_bytes()
    stderr = (private / "02-native-poc.stderr.original").read_bytes()
    combined = stdout.decode("utf-8", errors="replace") + stderr.decode("utf-8", errors="replace")
    require("IOS27_SHORTCUTS_RUNTIME_POC=PASS" in combined
            and row.get("outputSha256") == digest(combined.encode("utf-8"))
            and row.get("command") == "bash scripts/ios27_runtime_poc.sh",
            "ORIGINAL_GATE_OUTPUT_BINDING")


def git_capture(repo, private, capture, label, args, deadline):
    command = ["git", "--no-replace-objects", "-C", str(repo), *args]
    with (private / (label + ".stdout.original")).open("xb") as stdout, \
         (private / (label + ".stderr.original")).open("xb") as stderr:
        observed, details = capture(command, 15, output=stdout, error_output=stderr,
            execution_deadline=min(deadline - 30, time.monotonic() + 15),
            hard_deadline=deadline, stream_limit=STREAM_LIMIT)
    row = {"argv": command, **observed, "capture": details}
    record(private / (label + ".process.original.json"), row)
    require(capture_pass(row), "GIT_CAPTURE_FAILED")
    return (private / (label + ".stdout.original")).read_bytes()


def snapshot(repo, private, capture, context, phase, deadline):
    head = git_capture(repo, private, capture, phase + "-head", ["rev-parse", "HEAD"], deadline).strip().decode()
    tree = git_capture(repo, private, capture, phase + "-tree", ["rev-parse", "HEAD^{tree}"], deadline).strip().decode()
    status = git_capture(repo, private, capture, phase + "-status",
                         ["status", "--porcelain=v1", "--untracked-files=no"], deadline)
    require(head == context["sourceCommitSha"] and not status, "TRACKED_SOURCE_DIRTY")
    listing = git_capture(repo, private, capture, phase + "-files", ["ls-tree", "-rz", "HEAD"], deadline)
    rows = {}
    for entry in listing.split(b"\0"):
        if not entry:
            continue
        metadata, name = entry.split(b"\t", 1)
        mode, kind, blob = metadata.decode("ascii").split()
        relative = name.decode("utf-8")
        require(kind == "blob" and mode in {"100644", "100755"}
                and not Path(relative).is_absolute() and ".." not in Path(relative).parts,
                "SOURCE_MEMBER_TYPE")
        path = repo / relative
        require(path.is_file() and not path.is_symlink()
                and not any((repo.joinpath(*Path(relative).parts[:i])).is_symlink()
                            for i in range(1, len(Path(relative).parts)))
                and bool(path.stat().st_mode & 0o111) == (mode == "100755"),
                "SOURCE_MEMBER_MISSING")
        size = path.stat().st_size
        content, identity = hashlib.sha256(), hashlib.sha1(b"blob " + str(size).encode() + b"\0")
        with path.open("rb") as stream:
            while block := stream.read(1024 * 1024):
                require(time.monotonic() < deadline - 30, "SOURCE_OBSERVATION_DEADLINE")
                content.update(block)
                identity.update(block)
        require(identity.hexdigest() == blob and len(rows) < MAX_FILES,
                "SOURCE_PHYSICAL_BLOB_CHANGED")
        rows[relative] = {"blob": blob, "mode": mode, "bytes": size, "sha256": content.hexdigest()}
    require(rows, "SOURCE_EMPTY")
    value = {"head": head, "tree": tree, "trackedPhysicalInputs": rows,
             "trackedStatus": status.decode("utf-8"),
             "untrackedNativeProductsExcluded": True}
    record(private / ("source." + phase + ".original.json"), value)
    return value


def seal_originals(private, public, capture_module, recipient, context, deadline):
    require(time.monotonic() + 25 < deadline, "SEAL_BUDGET_UNAVAILABLE")
    paths = sorted(p for p in private.iterdir() if p.is_file())
    require(paths and len(paths) <= 64 and not any(p.is_symlink() for p in private.iterdir()),
            "ARCHIVE_MEMBERS")
    rows, total = [], 0
    buffer = io.BytesIO()
    with zipfile.ZipFile(buffer, "w", compression=zipfile.ZIP_DEFLATED) as archive:
        for path in paths:
            require(time.monotonic() + 25 < deadline, "SEAL_BUDGET_UNAVAILABLE")
            require(path.stat().st_size <= PRIVATE_LIMIT - total, "PRIVATE_ORIGINAL_BOUND")
            raw = path.read_bytes()
            total += len(raw)
            require(total <= PRIVATE_LIMIT, "PRIVATE_ORIGINAL_BOUND")
            rows.append({"name": path.name, "bytes": len(raw), "sha256": digest(raw)})
            archive.writestr(path.name, raw)
        manifest = canonical({"schema": SCHEMA, **context, "members": rows})
        archive.writestr("manifest.original.json", manifest)
    encoded = buffer.getvalue()
    require(len(encoded) <= ARCHIVE_LIMIT, "ENCRYPTED_ARCHIVE_BOUND")
    with zipfile.ZipFile(io.BytesIO(encoded)) as check:
        require(set(check.namelist()) == {r["name"] for r in rows} | {"manifest.original.json"},
                "ARCHIVE_MEMBERS_CHANGED")
        for row in rows:
            raw = check.read(row["name"])
            require(len(raw) == row["bytes"] and digest(raw) == row["sha256"], "ARCHIVE_BYTES_CHANGED")
        require(check.read("manifest.original.json") == manifest, "ARCHIVE_MANIFEST_CHANGED")
    archive_path = private / "r22-originals.zip"
    write_original(archive_path, encoded)
    binding = {"bytes": len(encoded), "sha256": digest(encoded)}
    cipher = capture_module.encrypt_archive(private, public, archive_path, binding, recipient, deadline)
    original_cipher = public / "project-prerequisite.zip.p7m"
    target = public / "r22-originals.zip.p7m"
    require(not target.exists() and not target.is_symlink(), "CIPHER_COLLISION")
    original_cipher.rename(target)  # Generic existing encryption transport; bytes unchanged.
    require(target.stat().st_size == cipher["bytes"] and digest(target.read_bytes()) == cipher["sha256"],
            "CIPHER_RENAME_CHANGED")
    return {"archive": binding, "ciphertext": cipher, "members": len(rows)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--private", type=Path, required=True)
    parser.add_argument("--public", type=Path, required=True)
    args = parser.parse_args()
    repo, private, public = (p.resolve() for p in (args.repo, args.private, args.public))
    require(repo.is_dir() and private != public and not private.exists() and not public.exists(),
            "FRESH_ROOTS_REQUIRED")
    require(repo not in private.parents and private not in repo.parents
            and private not in public.parents and public not in private.parents, "ROOT_ALIAS")
    context = context_from_environment(os.environ)
    require(sys.platform == "darwin", "MACOS_REQUIRED")
    os.umask(0o077)
    private.mkdir(mode=0o700)
    public.mkdir(mode=0o700)
    started, old_cwd = time.monotonic(), Path.cwd()
    deadline = started + WHOLE_SECONDS
    sys.path.insert(0, str(repo / QA))
    import project_prerequisite_observation as capture_module
    require(capture_module.STREAM_LIMIT == STREAM_LIMIT and capture_module.ARCHIVE_LIMIT == ARCHIVE_LIMIT,
            "QUALIFIED_CAPTURE_LIMITS_CHANGED")
    recipient = repo / QA / "xcode_environment_inventory_recipient.pem"
    require(digest(recipient.read_bytes()) == capture_module.RECIPIENT_SHA256,
            "PINNED_RECIPIENT_CHANGED")
    state = {"schema": SCHEMA, **context, "status": "FAILED", "gateInvocations": 0,
             "nativeScriptInvocations": 0, "wholeSeconds": WHOLE_SECONDS,
             "nativeSeconds": NATIVE_SECONDS, "originalNativeContractChanged": False,
             "otherGatesAccepted": False, "final179Acceptance": False, "errors": []}
    children, before, gate_row, sealed = None, None, None, None
    handlers = {s: signal.getsignal(s) for s in (signal.SIGALRM, signal.SIGTERM, signal.SIGINT)}
    def interrupted(signum, _frame):
        state["interruptionSignal"] = signum
        raise OwnerInterrupted()
    try:
        for signum in handlers:
            signal.signal(signum, interrupted)
        signal.setitimer(signal.ITIMER_REAL, WHOLE_SECONDS - FINAL_RESERVE)
        os.chdir(repo)
        record(private / "context.original.json", context)
        record(private / "owner-process.original.json", {
            "pid": os.getpid(), "actualProcessArgv": sys.orig_argv,
            "effectivePythonArgv": sys.argv, "cwd": str(repo),
            "startedMonotonic": started, "wholeDeadlineMonotonic": deadline,
            "wholeSeconds": WHOLE_SECONDS, "nativeSeconds": NATIVE_SECONDS,
            "newBoundedOwner": True, "historicalNativeTimeoutClaimed": False})
        before = snapshot(repo, private, capture_module.capture, context, "before", deadline)
        gate = load_gate(repo)
        children = OriginalGateChildren(repo, private, capture_module.capture, deadline)
        state["gateInvocations"] = 1
        gate_row = invoke_original(gate, children)
        record(private / "gate-result.original.json", gate_row)
        verify_gate_row(gate_row, children, private)
        state["status"] = "PASS"
    except BaseException as error:
        state["status"] = "FAILED"
        state["errors"].append(type(error).__name__)
    finally:
        signal.setitimer(signal.ITIMER_REAL, max(0.001, deadline - time.monotonic()))
        try:
            try:
                after = snapshot(repo, private, capture_module.capture, context, "after", deadline)
                require(before is not None and before == after, "SOURCE_DRIFT")
                state["sourceUnchanged"] = True
            except BaseException as error:
                state["sourceUnchanged"] = False
                state["status"] = "FAILED"
                state["errors"].append(type(error).__name__)
            require(time.monotonic() < deadline, "WHOLE_OWNER_DEADLINE")
            state["nativeScriptInvocations"] = sum(r["argv"] == NATIVE and r["spawned"]
                                                   for r in children.rows) if children else 0
            state["elapsedSeconds"] = time.monotonic() - started
            state["originalGateStatus"] = gate_row.get("status") if isinstance(gate_row, dict) else None
            record(private / "observation-result.original.json", state)
            try:
                sealed = seal_originals(private, public, capture_module, recipient, context, deadline)
            except BaseException as error:
                state["status"] = "FAILED"
                state["errors"].append(type(error).__name__)
            require(time.monotonic() < deadline, "WHOLE_OWNER_DEADLINE")
            if state["errors"]:
                state["status"] = "FAILED"
            summary = {"schema": SCHEMA, **context, "status": state["status"],
                       "gateInvocations": state["gateInvocations"],
                       "nativeScriptInvocations": state["nativeScriptInvocations"],
                       "originalGateStatus": state["originalGateStatus"],
                       "sourceUnchanged": state["sourceUnchanged"],
                       "privateOriginalsEncrypted": sealed is not None, "encryption": sealed,
                       "gateResultSHA256": digest(canonical(gate_row)) if gate_row is not None else None,
                       "elapsedSeconds": time.monotonic() - started,
                       "otherGatesAccepted": False, "final179Acceptance": False,
                       "errorCategories": state["errors"]}
            record(public / "summary.json", summary)
            if sealed is not None:
                require({p.name for p in public.iterdir()} == {"summary.json", "r22-originals.zip.p7m"}
                        and all(p.is_file() and not p.is_symlink() for p in public.iterdir()),
                        "PUBLICATION_MEMBERS_CHANGED")
        finally:
            signal.setitimer(signal.ITIMER_REAL, 0)
            for signum, handler in handlers.items():
                signal.signal(signum, handler)
            os.chdir(old_cwd)
    print(json.dumps(summary, sort_keys=True))
    return 0 if summary["status"] == "PASS" and summary["privateOriginalsEncrypted"] else 1


if __name__ == "__main__":
    raise SystemExit(main())
