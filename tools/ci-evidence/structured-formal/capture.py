#!/usr/bin/env python3
"""One bounded hosted observation of the frozen structured-value formal intake.

The controller checkout is a distinct source from the immutable compiler under
test. No original reports are rewritten and no global179/Apple credit is issued.
"""
import argparse
from collections import Counter
import ctypes
import datetime
import hashlib
import json
import math
import os
from pathlib import Path
import re
import signal
import stat
import subprocess
import sys
import time

SOURCE = "12f07521b3e9612b97f4e1fc52ccbee40e3b867a"
TREE = "7e445fe46da2c26832b5532b3cb344bbb1843c1c"
SELECTION_SHA = "512c70cafcbf2a893351012b495b8531adc2507cf3f1227d9c7f7afb269203b8"
WITNESSES_SHA = "8e2254802eace10f5f02618c4afec999e77de47636374b7feef11ae538fcbf7e"
INVENTORY_SHA = "e13092e2a74d0a41d050d3191e7d1439156c2e27b11785e45b053529c3e2674c"
WORKFLOW = ".github/workflows/p10r15-structured-values.yml"
HERE = Path(__file__).resolve().parent
MAX_STREAM = 16 * 1024 * 1024
MAX_ARTIFACT = 64 * 1024 * 1024
OWNER_SECONDS = 540
PROCESS_RESERVE = 25


class Rejected(ValueError):
    pass


def require(ok, code):
    if not ok:
        raise Rejected(code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), allow_nan=False) + "\n").encode()


def strict_json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    def invalid(value):
        raise Rejected("NONFINITE_JSON")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)


def write_new(path, raw):
    with Path(path).open("xb") as stream:
        stream.write(raw)


def record(path, value):
    raw = canonical(value)
    write_new(path, raw)
    return {"bytes": len(raw), "sha256": sha(raw)}


def read_regular(path, limit=MAX_ARTIFACT):
    path = Path(path).absolute()
    before = path.lstat()
    require(stat.S_ISREG(before.st_mode) and before.st_size <= limit, "INPUT_REGULAR_SIZE")
    require(all(not p.is_symlink() for p in path.parents), "INPUT_ANCESTOR_SYMLINK")
    with path.open("rb") as stream:
        opened = os.fstat(stream.fileno())
        raw = stream.read(limit + 1)
    after = path.lstat()
    identity = lambda v: (v.st_dev, v.st_ino, v.st_size, v.st_mtime_ns, v.st_ctime_ns)
    require(identity(before) == identity(opened) == identity(after), "INPUT_CHANGED")
    require(len(raw) == before.st_size and len(raw) <= limit, "INPUT_LENGTH")
    return raw


def safe_env():
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_CONFIG_NOSYSTEM="1",
               GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
    return env


def git(root, *args, deadline=None):
    remaining = 20 if deadline is None else min(20, deadline - time.monotonic())
    require(remaining > 0, "OWNER_DEADLINE")
    proc = subprocess.run(["git", "--no-replace-objects", "-C", str(root), *args],
                          env=safe_env(), stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          timeout=remaining, check=False)
    require(proc.returncode == 0, "GIT_QUERY_FAILED")
    return proc.stdout


def snapshot(root, expected, tree=None, deadline=None):
    root = Path(root).resolve(strict=True)
    require(re.fullmatch(r"[0-9a-f]{40}", expected) is not None, "SOURCE_SHA")
    require(git(root, "rev-parse", "--show-toplevel", deadline=deadline).decode().strip()
            == str(root), "SOURCE_ROOT")
    require(git(root, "rev-parse", "HEAD", deadline=deadline).decode().strip()
            == expected, "SOURCE_HEAD")
    actual_tree = git(root, "rev-parse", "HEAD^{tree}", deadline=deadline).decode().strip()
    require(tree is None or actual_tree == tree, "SOURCE_TREE")
    require(git(root, "rev-parse", "--is-shallow-repository", deadline=deadline).strip()
            == b"false", "SHALLOW_SOURCE")
    gitdir = Path(git(root, "rev-parse", "--absolute-git-dir", deadline=deadline).decode().strip())
    common = Path(git(root, "rev-parse", "--git-common-dir", deadline=deadline).decode().strip())
    if not common.is_absolute():
        common = root / common
    require(gitdir.resolve() == (root / ".git").resolve() == common.resolve(),
            "SOURCE_NOT_ORDINARY_CHECKOUT")
    require(not (gitdir / "objects/info/alternates").exists()
            and not (gitdir / "info/grafts").exists(), "SOURCE_OBJECT_REDIRECTION")
    require(not git(root, "for-each-ref", "refs/replace", deadline=deadline), "SOURCE_REPLACEMENTS")
    require(not git(root, "status", "--porcelain=v1", "--untracked-files=all",
                    deadline=deadline), "SOURCE_DIRTY")
    rows = git(root, "ls-tree", "-rz", "--full-tree", "HEAD", deadline=deadline).split(b"\0")
    files, index = [], []
    total = 0
    for row in rows:
        if not row:
            continue
        metadata, raw_path = row.split(b"\t", 1)
        mode, kind, blob = metadata.decode().split()
        path = raw_path.decode("utf-8", "strict")
        require(mode in ("100644", "100755") and kind == "blob", "SOURCE_NONREGULAR")
        require(not path.startswith("/") and ".." not in Path(path).parts, "SOURCE_PATH")
        raw = read_regular(root / path)
        require(hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
                == blob, "SOURCE_WORKING_BLOB:" + path)
        total += len(raw)
        require(total <= 1024 * 1024 * 1024, "SOURCE_AGGREGATE_LIMIT")
        files.append({"path": path, "mode": mode, "blob": blob,
                      "bytes": len(raw), "sha256": sha(raw)})
        index.append((mode + " " + blob + " 0\t").encode() + raw_path + b"\0")
        if deadline is not None:
            require(time.monotonic() < deadline, "OWNER_DEADLINE")
    require(git(root, "ls-files", "--stage", "-z", deadline=deadline) == b"".join(index),
            "SOURCE_INDEX")
    require(len(files) == len({v["path"] for v in files}) and files, "SOURCE_INVENTORY")
    return {"commit": expected, "tree": actual_tree, "root": str(root),
            "sourceClean": True, "files": files, "totalBytes": total}


def verify_selection(source_snapshot, selection_raw, witnesses_raw, inventory_raw):
    require(sha(selection_raw) == SELECTION_SHA, "SELECTION_PIN")
    require(sha(witnesses_raw) == WITNESSES_SHA, "WITNESSES_PIN")
    require(sha(inventory_raw) == INVENTORY_SHA, "CHILD_INVENTORY_PIN")
    selection, witnesses, inventory = map(strict_json,
                                          (selection_raw, witnesses_raw, inventory_raw))
    require(selection["sourceCommit"] == SOURCE and selection["sourceTree"] == TREE,
            "SELECTION_SOURCE")
    require(selection["goTimeoutSeconds"] == 480
            and selection["ownerTimeoutSeconds"] == OWNER_SECONDS, "ORIGINAL_BOUNDS")
    require(selection["expectedNamedEventCount"] == len(selection["expectedNamedEvents"]) == 117
            and len(set(selection["expectedNamedEvents"])) == 117, "OUTER_INVENTORY")
    require(selection["requiredEnvironment"] == [
        "CHERRI_STRUCTURED_ACCEPTANCE_ARTIFACTS", "CHERRI_STRUCTURED_VALUE_ARTIFACTS"],
        "ARTIFACT_ENVIRONMENTS")
    actual = {v["path"]: v for v in source_snapshot["files"]}
    require(len(witnesses) == 196 and sum(v["bytes"] for v in witnesses) == 3514475,
            "SOURCE_WITNESSES")
    require(len({v["path"] for v in witnesses}) == 196, "WITNESS_DUPLICATE")
    for row in witnesses:
        require(all(actual.get(row["path"], {}).get(k) == row[k]
                    for k in ("path", "bytes", "sha256")), "WITNESS_MISMATCH")
    require(len(inventory["tests"]) == 102 and len(inventory["events"]) == 1356
            and len(inventory["packages"]) == 3, "CHILD_COUNTS")
    require(sum(len(v) for v in inventory["original_f04_330"].values()) == 330,
            "ORIGINAL_F04_COUNT")
    for package, names in inventory["original_f04_330"].items():
        require(set(names) <= set(inventory["packages"][package]), "ORIGINAL_F04_MISSING")
    return selection, witnesses, inventory


def exact_events(raw, packages):
    require(isinstance(raw, bytes) and 0 < len(raw) <= MAX_STREAM
            and raw.endswith(b"\n") and not raw.endswith(b"\n\n"), "GO_STREAM_FRAMING")
    expected = {(pkg, name) for pkg, names in packages.items() for name in names}
    require(len(expected) == sum(len(names) for names in packages.values()), "EXPECTED_DUPLICATE")
    runs, passes, package_passes = Counter(), Counter(), Counter()
    for line in raw[:-1].split(b"\n"):
        require(line.strip(), "GO_EMPTY_EVENT")
        event = strict_json(line)
        require(isinstance(event, dict), "GO_EVENT_OBJECT")
        package, name, action = event.get("Package"), event.get("Test"), event.get("Action")
        require(package in packages, "GO_FOREIGN_PACKAGE")
        require(action in ("start", "run", "pause", "cont", "output", "pass"),
                "GO_FAILED_SKIPPED_UNKNOWN")
        if name is not None:
            require((package, name) in expected, "GO_FOREIGN_TEST")
            if action == "run":
                runs[(package, name)] += 1
            elif action == "pass":
                require(runs[(package, name)] == 1, "GO_PASS_WITHOUT_RUN")
                passes[(package, name)] += 1
        elif action == "pass":
            package_passes[package] += 1
        elif action in ("run", "pause", "cont"):
            raise Rejected("GO_TEST_NAME_MISSING")
    require(set(runs) == set(passes) == expected
            and all(v == 1 for v in runs.values())
            and all(v == 1 for v in passes.values()), "GO_EXACT_RUN_PASS")
    require(package_passes == Counter({p: 1 for p in packages}), "GO_PACKAGE_TERMINALS")
    return {"namedRunEvents": len(runs), "namedPassEvents": len(passes),
            "packages": {p: len(names) for p, names in packages.items()}}


def proc_table():
    rows = {}
    for path in Path("/proc").glob("[0-9]*/stat"):
        try:
            text = path.read_text()
            tail = text[text.rindex(")") + 2:].split()
            pid = int(path.parent.name)
            rows[pid] = {"pid": pid, "state": tail[0], "ppid": int(tail[1]),
                         "pgid": int(tail[2]), "startedTicks": int(tail[19])}
        except (OSError, ValueError, IndexError):
            continue
    return rows


def descendants(table, leader, known, baseline):
    # PR_SET_CHILD_SUBREAPER makes orphaned descendants observable by this
    # controller. There are no concurrent controller children in this phase.
    owned = {pid for pid, row in table.items()
             if (pid == leader or (row["ppid"] == os.getpid()
                 and baseline.get(pid) != row["startedTicks"])
                 or (pid in known and known[pid]["startedTicks"] == row["startedTicks"]))}
    while True:
        found = owned | {pid for pid, row in table.items() if row["ppid"] in owned}
        if found == owned:
            break
        owned = found
    require(len(owned) <= 4096, "OWNED_PROCESS_LIMIT")
    for pid in owned:
        known[pid] = table[pid]
    return {pid: table[pid] for pid in owned}


def capture(command, cwd, env, out, deadline):
    require(sys.platform == "linux" and os.name == "posix", "LINUX_PROCESS_OWNER_REQUIRED")
    libc = ctypes.CDLL(None, use_errno=True)
    prior_subreaper = ctypes.c_int()
    require(libc.prctl(37, ctypes.byref(prior_subreaper), 0, 0, 0) == 0,
            "SUBREAPER_QUERY_FAILED")
    require(libc.prctl(36, 1, 0, 0, 0) == 0, "SUBREAPER_UNAVAILABLE")
    baseline = {pid: row["startedTicks"] for pid, row in proc_table().items()}
    known, signals, interrupted = {}, [], []
    def stop(signum, frame):
        interrupted.append(signum)
    old = {sig: signal.signal(sig, stop) for sig in (signal.SIGTERM, signal.SIGINT)}
    receipt = {"command": command, "cwd": str(cwd), "startedUTC": datetime.datetime.now(
        datetime.timezone.utc).isoformat(), "pid": None, "reason": None,
        "exitCode": None, "ownedGroupsSignalled": signals, "ownedGroupAbsent": False}
    start, process = time.monotonic(), None
    try:
        with (out / "go-events.original.jsonl").open("xb") as stdout, \
             (out / "go-stderr.original.txt").open("xb") as stderr:
            process = subprocess.Popen(command, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                                       stdout=stdout, stderr=stderr, start_new_session=True)
            receipt["pid"] = process.pid
            record(out / "process-start.original.json", receipt)
            while True:
                descendants(proc_table(), process.pid, known, baseline)
                code = process.poll()
                overflow = stdout.tell() > MAX_STREAM or stderr.tell() > MAX_STREAM
                if interrupted or overflow or time.monotonic() >= deadline-PROCESS_RESERVE or code is not None:
                    receipt["reason"] = ("SIGNAL" if interrupted else "OUTPUT_LIMIT" if overflow else
                        "OWNER_PROCESS_DEADLINE" if code is None else None)
                    break
                time.sleep(0.05)
    except BaseException as error:
        # Even a failed journal/write after Popen must clean the owned tree.
        receipt["reason"] = "CAPTURE_ERROR:" + type(error).__name__
    finally:
        if process is not None:
            try:
                code = process.poll()
                owned = descendants(proc_table(), process.pid, known, baseline)
                live = {pid: v for pid, v in owned.items() if v["state"] != "Z"}
                if code is not None and live:
                    receipt["reason"] = receipt["reason"] or "OWNED_DESCENDANTS_REMAINED"
                if process.poll() is None or live:
                    for sig, seconds in ((signal.SIGTERM, 2), (signal.SIGKILL, 3)):
                        owned = descendants(proc_table(), process.pid, known, baseline)
                        groups = sorted({v["pgid"] for v in owned.values() if v["state"] != "Z"})
                        require(os.getpgrp() not in groups, "CONTROLLER_GROUP_REJECTED")
                        for group in groups:
                            try:
                                os.killpg(group, sig)
                                signals.append({"pgid": group, "signal": sig})
                            except ProcessLookupError:
                                pass
                        until = min(time.monotonic()+seconds, deadline-15)
                        while time.monotonic() < until:
                            process.poll()
                            if not any(v["state"] != "Z" for v in
                                       descendants(proc_table(), process.pid, known, baseline).values()):
                                break
                            time.sleep(0.05)
                receipt["exitCode"] = process.wait(timeout=max(0.1, min(3, deadline-time.monotonic())))
                for owned_pid in sorted(known):
                    if owned_pid == process.pid:
                        continue
                    try:
                        os.waitpid(owned_pid, os.WNOHANG)
                    except ChildProcessError:
                        pass
                remaining = descendants(proc_table(), process.pid, known, baseline)
                receipt["ownedGroupAbsent"] = not remaining
                if remaining:
                    receipt["reason"] = receipt["reason"] or "OWNED_GROUP_REMAINED"
            except BaseException as error:
                receipt["reason"] = "CLEANUP_UNVERIFIED:" + type(error).__name__
        for sig, handler in old.items():
            signal.signal(sig, handler)
        if libc.prctl(36, prior_subreaper.value, 0, 0, 0) != 0:
            receipt["reason"] = "SUBREAPER_RESTORE_FAILED"
        receipt.update(finishedUTC=datetime.datetime.now(datetime.timezone.utc).isoformat(),
                       elapsedSeconds=time.monotonic()-start,
                       observedOwnedProcesses=sorted(known.values(), key=lambda row: row["pid"]),
                       receivedSignals=interrupted)
        record(out / "process-result.original.json", receipt)
    return receipt

def contained(root, path):
    path = Path(path)
    require(path.is_absolute() and root in path.resolve().parents, "ARTIFACT_ESCAPE")
    return read_regular(path)


def validate_products(out, selection, inventory, witnesses):
    matches = list((out / "acceptance-artifacts").rglob("evidence-manifest.original.json"))
    require(len(matches) == 1, "ORIGINAL_MANIFEST_UNIQUE")
    directory = matches[0].parent
    manifest_raw = read_regular(matches[0])
    report_raw = read_regular(directory / "intake-report.original.json")
    audit_raw = read_regular(directory / "execution-audit.original.json")
    manifest, report, audit = map(strict_json, (manifest_raw, report_raw, audit_raw))
    require(manifest["implementation_sha"] == SOURCE and len(manifest["records"]) == 8,
            "ORIGINAL_MANIFEST_SOURCE_COUNT")
    wanted = {(ident, tier) for ident in ("T05", "L01", "F04", "RP20")
              for tier in ("unit", "native-structure")}
    observed = set()
    original_artifacts = None
    for row in manifest["records"]:
        require(len(row["requirements"]) == 1 and row["implementation_sha"] == SOURCE
                and row["result"] == "passed" and type(row["exit_code"]) is int
                and row["exit_code"] == 0, "ORIGINAL_RECORD")
        key = (row["requirements"][0], row["tier"])
        require(key in wanted and key not in observed, "ORIGINAL_RECORD_IDENTITY")
        observed.add(key)
        if original_artifacts is None:
            original_artifacts = row["artifacts"]
        require(canonical(row["artifacts"]) == canonical(original_artifacts),
                "DETACHED_COHORT_ARTIFACTS")
        for artifact in row["artifacts"]:
            require(sha(contained(out, artifact["path"])) == artifact["sha256"], "ARTIFACT_HASH")
    require(observed == wanted, "FOUR_DISTINCT_REQUIREMENTS")
    require(report["total_cases"] == report["passed_cases"] == 4
            and report["failed_cases"] == report["skipped_cases"] == 0
            and report["closure_complete"] is False, "ORIGINAL_REPORT")
    require({v["id"] for v in report["results"]} == {key[0] for key in wanted}
            and all(v["status"] == "PASSED" and v["passed"] is True for v in report["results"]),
            "ORIGINAL_REPORT_ROWS")
    require(audit["closure_complete"] is False and len(audit["rows"]) == 179
            and len({v["id"] for v in audit["rows"]}) == 179, "ORIGINAL_GLOBAL_AUDIT")
    require({v["id"] for v in audit["rows"] if v["status"] == "PASS"}
            == {key[0] for key in wanted}
            and all(v["source_sha"] == SOURCE for v in audit["rows"]), "AUDIT_NO_INHERITED_PASS")
    paths = {Path(v["path"]).name: Path(v["path"]) for v in original_artifacts}
    require(len(paths) == len(original_artifacts), "ARTIFACT_BASENAME_DUPLICATE")
    events_raw = contained(out, paths["structured-values-cohort-go.jsonl"])
    child_events = exact_events(events_raw, inventory["packages"])
    process = strict_json(contained(out, paths["structured-values-cohort-process.json"]))
    require(process["source_commit"] == SOURCE and process["source_clean_before"] is True
            and process["source_clean_after"] is True and process["timed_out"] is False
            and type(process["exit_code"]) is int and process["exit_code"] == 0
            and process["raw_sha256"] == sha(events_raw) and process["raw_bytes"] == len(events_raw),
            "CHILD_PROCESS_SOURCE")
    expected_sources = [{"path": v["path"], "sha256": v["sha256"]} for v in witnesses]
    require(process["source_files"] == expected_sources, "CHILD_PROCESS_WITNESSES")
    return {"requirements": sorted({key[0] for key in wanted}), "records": 8,
            "childEvents": child_events, "originalF04Identities": 330,
            "globalAuditRows": 179, "globalClosureComplete": False,
            "originalIntake": {p.name: {"path": str(p.relative_to(out)),
                "sha256": sha(read_regular(p))} for p in (
                matches[0], directory/"intake-report.original.json",
                directory/"execution-audit.original.json")}}


def capacity_observation(paths):
    observed = {"observedUTC": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "observationOnly": True, "filesystems": []}
    for path in paths:
        path = Path(path).resolve(strict=True)
        value = os.statvfs(path)
        observed["filesystems"].append({
            "path": str(path), "device": path.stat().st_dev,
            "blockBytes": value.f_frsize,
            "availableBytes": value.f_bavail * value.f_frsize,
            "freeBytes": value.f_bfree * value.f_frsize,
            "totalBytes": value.f_blocks * value.f_frsize})
    return observed


def main(argv=None):
    parser = argparse.ArgumentParser()
    parser.add_argument("--compiler-root", type=Path, required=True)
    parser.add_argument("--controller-root", type=Path, required=True)
    parser.add_argument("--controller-sha", required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args(argv)
    started = time.monotonic()
    deadline = started + OWNER_SECONDS
    out = args.out.absolute()
    require(args.compiler_root.resolve() not in out.resolve().parents
            and args.controller_root.resolve() not in out.resolve().parents
            and out.resolve() not in (args.compiler_root.resolve(), args.controller_root.resolve()),
            "OUTPUT_OUTSIDE_SOURCE")
    require(not out.exists() and not out.is_symlink(), "FRESH_OUTPUT")
    out.mkdir(parents=True, mode=0o700)
    result = {"schema": "cherri-structured-formal-hosted-capture/1",
              "compilerSource": SOURCE, "controllerSource": args.controller_sha,
              "status": "NOT_RUN", "formal179Granted": False, "nativeExecution": False}
    before = controller_before = None
    try:
        record(out/"capacity.before.original.json", capacity_observation(
            (args.controller_root, args.compiler_root, out)))
        require(sys.platform == "linux" and os.environ.get("GITHUB_ACTIONS") == "true",
                "ACTUAL_GITHUB_LINUX_REQUIRED")
        require(os.environ.get("GITHUB_SHA") == args.controller_sha
                and os.environ.get("GITHUB_REPOSITORY") == "davidpovarsky/cherri",
                "ACTUAL_CONTROLLER_CONTEXT")
        require(args.compiler_root.resolve() != args.controller_root.resolve()
                and out.resolve() not in (args.compiler_root.resolve(), args.controller_root.resolve()),
                "SEPARATE_CHECKOUTS")
        require(HERE == args.controller_root.resolve()/"tools/ci-evidence/structured-formal",
                "CONTROLLER_EXECUTABLE_SOURCE")
        require(re.fullmatch(r"[1-9][0-9]*", os.environ.get("GITHUB_RUN_ID", ""))
                and re.fullmatch(r"[1-9][0-9]*", os.environ.get("GITHUB_RUN_ATTEMPT", "")),
                "ACTUAL_RUN_IDENTITY")
        result["githubContext"] = {key: os.environ.get(key) for key in (
            "GITHUB_RUN_ID", "GITHUB_RUN_ATTEMPT", "GITHUB_SHA", "GITHUB_REF",
            "GITHUB_WORKFLOW_REF", "GITHUB_WORKFLOW_SHA", "GITHUB_REPOSITORY", "RUNNER_OS", "RUNNER_ARCH")}
        controller_before = snapshot(args.controller_root, args.controller_sha, deadline=deadline)
        before = snapshot(args.compiler_root, SOURCE, TREE, deadline=deadline)
        record(out/"controller-source.before.original.json", controller_before)
        record(out/"compiler-source.before.original.json", before)
        selection_raw = read_regular(HERE/"selection.original.json")
        witnesses_raw = read_regular(HERE/"source-witnesses.original.json")
        inventory_raw = read_regular(args.compiler_root/"tools/language-acceptance/structured_value_inventory.json")
        selection, witnesses, inventory = verify_selection(before, selection_raw, witnesses_raw, inventory_raw)
        for name, raw in (("selection.original.json", selection_raw),
                          ("source-witnesses.original.json", witnesses_raw),
                          ("child-inventory.original.json", inventory_raw)):
            write_new(out/name, raw)
        version = subprocess.run(["go", "version"], stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                 timeout=10, check=True)
        write_new(out/"go-version.original.txt", version.stdout)
        require(version.stdout.startswith(b"go version go1.24.13 linux/"), "GO_TOOLCHAIN")
        env = safe_env()
        for key in list(env):
            if key.startswith("CHERRI_") or key in ("GH_TOKEN", "GITHUB_TOKEN"):
                del env[key]
        env.update(GOMAXPROCS="2", GOFLAGS="-p=1 -mod=readonly", GOWORK="off",
                   GOTOOLCHAIN="local", PYTHONDONTWRITEBYTECODE="1",
                   CHERRI_STRUCTURED_ACCEPTANCE_ARTIFACTS=str(out/"acceptance-artifacts"),
                   CHERRI_STRUCTURED_VALUE_ARTIFACTS=str(out/"value-artifacts"))
        result["artifactEnvironments"] = {k: env[k] for k in selection["requiredEnvironment"]}
        result["bounds"] = {"goTimeoutSeconds": 480, "ownerTimeoutSeconds": OWNER_SECONDS,
                            "postProcessReserveSeconds": PROCESS_RESERVE}
        command = ["go", "test", "-count=1", "-json", "-timeout", "480s",
                   "./tools/language-acceptance", "-run", selection["goPattern"]]
        record(out/"intent.original.json", {**result, "command": command,
            "selectionSHA256": SELECTION_SHA, "sourceWitnessesSHA256": WITNESSES_SHA,
            "workflowSHA256": sha(read_regular(args.controller_root/WORKFLOW))})
        process = capture(command, args.compiler_root, env, out, deadline)
        result["status"] = "FAIL"
        require(process["exitCode"] == 0 and process["reason"] is None
                and process["ownedGroupAbsent"], "ACTUAL_PROCESS_FAILED")
        parent_raw = read_regular(out/"go-events.original.jsonl", MAX_STREAM)
        result["outerEvents"] = exact_events(parent_raw, {
            selection["packageIDs"][0]: selection["expectedNamedEvents"]})
        result["products"] = validate_products(out, selection, inventory, witnesses)
        result["status"] = "PASS"
    except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
        result["errorType"] = type(error).__name__
        result["error"] = str(error)[:512]
    finally:
        if before is not None and controller_before is not None:
            try:
                after = snapshot(args.compiler_root, SOURCE, TREE, deadline=deadline)
                control_after = snapshot(args.controller_root, args.controller_sha, deadline=deadline)
                record(out/"compiler-source.after.original.json", after)
                record(out/"controller-source.after.original.json", control_after)
                result["compilerSourceUnchanged"] = canonical(before) == canonical(after)
                result["controllerSourceUnchanged"] = canonical(controller_before) == canonical(control_after)
                require(result["compilerSourceUnchanged"] and result["controllerSourceUnchanged"],
                        "SOURCE_CHANGED_DURING_EXECUTION")
            except (OSError, ValueError, KeyError, TypeError, subprocess.SubprocessError) as error:
                result.update(status="FAIL", sourceRecheckError=str(error)[:512])
        try:
            record(out/"capacity.after.original.json", capacity_observation(
                (args.controller_root, args.compiler_root, out)))
        except OSError as error:
            result.update(status="FAIL", capacityObservationError=str(error)[:512])
        result["elapsedSeconds"] = time.monotonic()-started
        result["finishedUTC"] = datetime.datetime.now(datetime.timezone.utc).isoformat()
        if result["elapsedSeconds"] >= OWNER_SECONDS:
            result.update(status="FAIL", ownerDeadlineExceeded=True)
        inventory = []
        for path in sorted(out.rglob("*")):
            if time.monotonic() >= deadline:
                result.update(status="FAIL", originalInventoryIncomplete=True)
                break
            if path.is_dir():
                continue
            raw = read_regular(path)
            inventory.append({"path": path.relative_to(out).as_posix(),
                              "bytes": len(raw), "sha256": sha(raw)})
        record(out/"retained-originals.inventory.json", inventory)
        record(out/"result.original.json", result)
    print(json.dumps({"status": result["status"], "compilerSource": SOURCE,
                      "controllerSource": args.controller_sha, "formal179Granted": False}))
    return 0 if result["status"] == "PASS" else 1


if __name__ == "__main__":
    raise SystemExit(main())
