"""Source-bound execution of the two actual P10 catalog evidence producers."""
import argparse
import ast
import datetime
import hashlib
import json
import os
import pathlib
import shutil
import signal
import stat
import subprocess
import sys
import time

PRODUCERS = [
    "TestP10_VerifyAndRecord505CatalogExecution",
    "TestP10_ParameterContractMatrix_AllCatalogActions",
]
EXPECTED_ARTIFACTS = [
    "p10-505-execution-evidence.json",
    "p10-505-parameter-contract-evidence.json",
]
GO_SELECTION = {
    "GOTOOLCHAIN": "go1.24.13",
    "GOMODCACHE": "/mnt/development/tooling/cherri-go/mod",
    "GOCACHE": "/mnt/development/tooling/cherri-go/runtime-persistent-20261123/build-cache",
    "GOTMPDIR": "/mnt/development/tooling/cherri-go/runtime-persistent-20261123/tmp-ubuntu",
    "TMPDIR": "/mnt/development/tooling/cherri-go/runtime-persistent-20261123/tmp-ubuntu",
}

def digest(raw):
    return hashlib.sha256(raw).hexdigest()

def utc():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def original(path, value):
    raw = value if isinstance(value, bytes) else (json.dumps(value, indent=2) + "\n").encode()
    with path.open("xb") as f:
        f.write(raw)

def capture(argv, repo, env=None, timeout=30):
    result = subprocess.run(argv, cwd=repo, env=git_environment() if env is None else env,
                            stdin=subprocess.DEVNULL, capture_output=True, timeout=timeout)
    if result.returncode != 0:
        raise RuntimeError("Preflight command failed: " + repr(argv) + "; exit=" + str(result.returncode))
    return result.stdout

def source(repo):
    if (repo != pathlib.Path(PIN_REPO) or not (repo / ".git").is_dir()
            or (repo / ".git").is_symlink() or (repo / ".git/objects/info/alternates").exists()):
        raise RuntimeError("Exact ordinary no-alternates source required")
    return {
        "head": capture(["git", "--no-replace-objects", "rev-parse", "HEAD"], repo).decode().strip(),
        "toplevel": capture(["git", "--no-replace-objects", "rev-parse", "--show-toplevel"], repo).decode().strip(),
        "replace_refs": capture(["git", "replace", "-l"], repo).decode(),
        "tree": capture(["git", "rev-parse", "HEAD^{tree}"], repo).decode().strip(),
        "branch": capture(["git", "branch", "--show-current"], repo).decode().strip(),
        "status": capture(["git", "status", "--porcelain=v1", "--untracked-files=all"], repo).decode(),
    }

def dependency_witness(repo):
    path = repo / "tools/p10r3/generate_catalog_and_pack_ledgers.py"
    tree = ast.parse(path.read_bytes())
    definitions = [node for node in tree.body if isinstance(node, ast.Assign)
                   and any(isinstance(target, ast.Name) and target.id == "DEPENDENCY_GLOBS"
                           for target in node.targets)]
    if len(definitions) != 1:
        raise RuntimeError("Dependency glob definition is ambiguous")
    patterns = ast.literal_eval(definitions[0].value)
    names = sorted({p.relative_to(repo).as_posix() for pattern in patterns
                    for p in repo.glob(pattern) if p.is_file()})
    rows = []
    h = hashlib.sha256()
    for name in names:
        raw = (repo / name).read_bytes()
        normalized = digest(raw.replace(b"\r\n", b"\n"))
        rows.append({"path": name, "bytes": len(raw), "sha256": digest(raw),
                     "normalized_sha256": normalized})
        h.update((name + ":" + normalized + "\n").encode())
    return {"fingerprint": h.hexdigest(), "patterns": patterns, "files": rows}

PIN_SOURCE = "57b6d919757e1d0262993c6f11c6dc1cd17d6177"
PIN_TREE = "370d85f75d8fb5bba5f9c8afe438208d97a525c8"
PIN_REPO = "/mnt/development/projects/releases/cherri-p10r15-f01-compiler-source-20261123"
PIN_OUTPUT = "/mnt/development/projects/evidence/cherri-p10r15-505-s57b6d919-20261123/raw-attempt-01"
PIN_RAW_WITNESS = "1cfc83f03850d155ca5b14feaee042b17ec2822bdbbf2467d4e3679a4572002a"
MINIMUM_FREE_BYTES = 2 * 1024**3
OWNED_OUTPUT = None

def git_environment():
    keep = ("PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "LC_CTYPE", "TZ")
    env = {key: os.environ[key] for key in keep if key in os.environ}
    env.update({"GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0",
                "GIT_TERMINAL_PROMPT": "0"})
    return env

def raw_input_witness(repo):
    pinned_path = pathlib.Path(__file__).with_name("raw-inputs.original.json")
    pinned_raw = pinned_path.read_bytes()
    if digest(pinned_raw) != PIN_RAW_WITNESS:
        raise RuntimeError("Reviewed raw-input witness changed")
    expected = json.loads(pinned_raw)
    raw_tree = capture(["git", "--no-replace-objects", "ls-tree", "-r", "-z", PIN_SOURCE], repo)
    entries = {}
    for row in raw_tree.split(b"\0"):
        if not row:
            continue
        metadata, path = row.split(b"\t", 1)
        mode, kind, blob = metadata.decode().split()
        name = path.decode()
        selected = (("/" not in name and name.endswith(".go"))
                    or name in ("go.mod", "go.sum", "stdfunc.cherri")
                    or name.startswith(("internal/", "actions/", "testdata/", "tools/language-schema/")))
        if selected:
            entries[name] = {"path": name, "mode": mode, "kind": kind, "blob": blob}
    actual = [entries[name] for name in sorted(entries)]
    if actual != expected or len(actual) != 595:
        raise RuntimeError("Actual source does not match the reviewed595-input scope")
    files = []
    for row in actual:
        path = repo / row["path"]
        info = path.lstat()
        if not stat.S_ISREG(info.st_mode):
            raise RuntimeError("Raw input is not a regular tracked file")
        raw = path.read_bytes()
        mode = "100755" if info.st_mode & 0o111 else "100644"
        blob = hashlib.sha1(b"blob " + str(len(raw)).encode() + b"\0" + raw).hexdigest()
        if blob != row["blob"] or mode != row["mode"]:
            raise RuntimeError("Raw working input differs from its frozen Git object")
        files.append({**row, "bytes": len(raw), "sha256": digest(raw)})
    return {"source": PIN_SOURCE, "tree": PIN_TREE, "count": len(files),
            "pinned_git_witness_sha256": PIN_RAW_WITNESS, "files": files}

def artifact_counts(artifacts):
    summaries, errors = [], []
    for name in EXPECTED_ARTIFACTS:
        path = artifacts / name
        if not path.is_file() or path.is_symlink():
            errors.append({"file": name, "reason": "MISSING_OR_NOT_REGULAR"})
            continue
        try:
            raw = path.read_bytes()
            value = json.loads(raw)
            if type(value) is not dict or type(value.get("entries")) is not list:
                raise ValueError("Invalid root/entries")
            execution = name == EXPECTED_ARTIFACTS[0]
            total_key = "totalCatalogEntries" if execution else "totalCatalog"
            name_key = "name" if execution else "actionName"
            dimensions_key = "parameterDimensionSummaries" if execution else "dimensionSummaries"
            scalar_keys = (
                ("totalCatalogEntries", "callableCount", "constructCount", "legacyPassCount",
                 "v2PassCount", "plistIdentifierPassCount", "parameterContractPassCount",
                 "decompilePassCount", "roundTripPassCount", "structuralRoundTripCount",
                 "nativeSpecimenParityCount", "appleRuntimeExecutedCount")
                if execution else ("totalCatalog", "callableCount", "constructCount", "passedCount", "failedCount"))
            if any(type(value.get(key)) is not int or value[key] < 0 for key in scalar_keys):
                raise ValueError("Invalid counter type")
            names = [row[name_key] for row in value["entries"]]
            dimensions = value[dimensions_key]
            if (value[total_key] != 505 or len(names) != 505 or
                any(type(item) is not str for item in names) or len(set(names)) != 505 or
                value["callableCount"] + value["constructCount"] != 505 or type(dimensions) is not list):
                raise ValueError("Invalid denominator/identity")
            if not execution and (value.get("schemaVersion") != "2.0.0" or
                                  type(value.get("allContractsPassed")) is not bool):
                raise ValueError("Invalid parameter schema")
            summaries.append({
                "file": name, "bytes": len(raw), "sha256": digest(raw),
                "actual_counters": {key: value[key] for key in scalar_keys},
                "entry_count": len(names), "unique_name_count": len(set(names)),
                "catalog_name_set_sha256": digest(json.dumps(sorted(names), separators=(",", ":")).encode()),
                "dimension_summary_count": len(dimensions),
                "schema_version": value.get("schemaVersion"),
                "all_contracts_passed": value.get("allContractsPassed"),
            })
        except (KeyError, TypeError, ValueError, UnicodeError):
            errors.append({"file": name, "reason": "INVALID_SHAPE_OR_COUNTS"})
    if len(summaries) == 2 and summaries[0]["catalog_name_set_sha256"] != summaries[1]["catalog_name_set_sha256"]:
        errors.append({"file": "BOTH", "reason": "CATALOG_NAME_SET_MISMATCH"})
    return summaries, errors


def main():
    global OWNED_OUTPUT
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", required=True)
    parser.add_argument("--expected-source", required=True)
    parser.add_argument("--output", required=True)
    parser.add_argument("--execute-reviewed-plan", action="store_true")
    args = parser.parse_args()
    if not args.execute_reviewed_plan:
        print("RAW505_SUPERVISOR_INACTIVE", file=sys.stderr)
        return 64
    if os.getuid() != 1001:
        raise RuntimeError("Source owner UID1001 required")
    repo = pathlib.Path(args.repo).resolve(strict=True)
    out = pathlib.Path(args.output).absolute()
    if len(args.expected_source) != 40 or any(c not in "0123456789abcdef" for c in args.expected_source):
        raise RuntimeError("An exact coordinator-frozen SHA is required")
    if (repo != pathlib.Path(PIN_REPO) or args.expected_source != PIN_SOURCE
            or out != pathlib.Path(PIN_OUTPUT)):
        raise RuntimeError("Exact reviewed source/output pins required")
    if out.is_relative_to(repo) or out.exists():
        raise RuntimeError("Evidence output must be new and outside the source checkout")
    if not out.parent.exists() or out.parent.resolve().is_relative_to(repo):
        raise RuntimeError("Evidence parent must already exist physically outside the source checkout")
    before = source(repo)
    if (before["head"] != args.expected_source or before["tree"] != PIN_TREE or before["status"]
            or before["toplevel"] != str(repo) or before["replace_refs"] or not before["branch"]):
        raise RuntimeError("Frozen source mismatch or dirty source; producer not started")
    if shutil.disk_usage("/mnt/development").free < MINIMUM_FREE_BYTES:
        raise RuntimeError("Fresh2GiB admission required before producer setup")
    out.mkdir(mode=0o700)
    OWNED_OUTPUT = out
    original(out / "submitted-supervisor.py", pathlib.Path(__file__).read_bytes())
    artifacts = out / "artifacts"
    artifacts.mkdir(mode=0o700)
    original(out / "source-before.original.json", before)
    witness_before = dependency_witness(repo)
    original(out / "input-witness-before.original.json", witness_before)
    raw_before = raw_input_witness(repo)
    original(out / "raw-input-witness-before.original.json", raw_before)
    keep = ("PATH", "HOME", "USER", "LOGNAME", "LANG", "LC_ALL", "LC_CTYPE", "TZ")
    env = {key: os.environ[key] for key in keep if key in os.environ}
    env.update(GO_SELECTION)
    env["CHERRI_CATALOG_EVIDENCE_DIR"] = str(artifacts)
    env["PYTHONDONTWRITEBYTECODE"] = "1"
    env.update({"GIT_NO_REPLACE_OBJECTS": "1", "GIT_OPTIONAL_LOCKS": "0", "GIT_TERMINAL_PROMPT": "0"})
    for key in ("GOMODCACHE", "GOCACHE", "GOTMPDIR", "TMPDIR"):
        path = pathlib.Path(env[key])
        if not path.is_dir() or not os.access(path, os.R_OK | os.W_OK | os.X_OK):
            raise RuntimeError("Configured development-volume cache is not writable")
    go = shutil.which("go", path=env.get("PATH"))
    if not go:
        raise RuntimeError("Go launcher absent")
    goenv_keys = ["GOVERSION", "GOOS", "GOARCH", "GOROOT", "GOTOOLDIR",
                  "GOTOOLCHAIN", "GOMODCACHE", "GOCACHE", "GOTMPDIR"]
    goenv_raw = capture([go, "env", "-json", *goenv_keys], repo, env, timeout=60)
    original(out / "go-env.original.json", goenv_raw)
    goenv = json.loads(goenv_raw)
    if goenv["GOVERSION"] != "go1.24.13":
        raise RuntimeError("Actual toolchain does not match coordinator selection")
    selected_go = pathlib.Path(goenv["GOROOT"]) / "bin/go"
    toolchain = {
        "launcher": str(pathlib.Path(go).resolve()),
        "launcher_sha256": digest(pathlib.Path(go).read_bytes()),
        "selected_go": str(selected_go),
        "selected_go_sha256": digest(selected_go.read_bytes()),
        "go_version": capture([go, "version"], repo, env).decode().strip(),
        "git_version": capture(["git", "--version"], repo).decode().strip(),
        "python_version": sys.version,
    }
    original(out / "toolchain.original.json", toolchain)
    argv = [go, "test", "-count=1", "-json", "-timeout=15m", "-parallel=2",
            "-run", "^(" + "|".join(PRODUCERS) + ")$", "."]
    intent = {
        "source": before, "argv": argv, "environment_selection": {
            **GO_SELECTION, "CHERRI_CATALOG_EVIDENCE_DIR": str(artifacts)},
        "expected_producers": PRODUCERS, "expected_artifacts": EXPECTED_ARTIFACTS,
        "test_timeout_seconds": 900, "process_wall_limit_seconds": 1200,
        "scope": "Actual local catalog execution only; no native specimen or Apple runtime credit.",
        "driver_sha256": digest(pathlib.Path(__file__).read_bytes()), "recorded_utc": utc(),
    }
    capacity = shutil.disk_usage("/mnt/development").free
    if capacity < MINIMUM_FREE_BYTES:
        raise RuntimeError("Fresh2GiB admission required immediately before producer")
    if source(repo) != before:
        raise RuntimeError("Source changed during preflight; producer not started")
    intent.update({"minimum_free_bytes": MINIMUM_FREE_BYTES, "capacity_free_before": capacity,
                   "termination_grace_seconds": 5, "kill_reap_wait_seconds": 5,
                   "final_consumer_source": None, "raw_input_count": raw_before["count"],
                   "provider_ai_private_environment_forwarded": False})
    original(out / "intent.original.json", intent)
    started_utc, started = utc(), time.monotonic()
    timed_out = False
    with (out / "stdout.original.jsonl").open("xb") as stdout, (out / "stderr.original").open("xb") as stderr:
        process = subprocess.Popen(argv, cwd=repo, env=env, stdout=stdout, stderr=stderr, start_new_session=True)
        original(out / "process-start.original.json", {"pid": process.pid, "started_utc": started_utc})
        try:
            returncode = process.wait(timeout=1200)
        except subprocess.TimeoutExpired:
            timed_out = True
            os.killpg(process.pid, signal.SIGTERM)
            try:
                returncode = process.wait(timeout=5)
            except subprocess.TimeoutExpired:
                os.killpg(process.pid, signal.SIGKILL)
                returncode = process.wait(timeout=5)
    finished_utc, elapsed = utc(), time.monotonic() - started
    original(out / "process-terminal.original.json", {
        "source": before, "argv": argv, "pid": process.pid,
        "started_utc": started_utc, "finished_utc": finished_utc,
        "elapsed_seconds": elapsed, "exit_code": returncode, "timed_out": timed_out,
        "stdout_sha256": digest((out / "stdout.original.jsonl").read_bytes()),
        "stderr_sha256": digest((out / "stderr.original").read_bytes()),
        "postflight_validation_pending": True,
    })
    after = source(repo)
    witness_after = dependency_witness(repo)
    original(out / "source-after.original.json", after)
    original(out / "input-witness-after.original.json", witness_after)
    raw_after = raw_input_witness(repo)
    original(out / "raw-input-witness-after.original.json", raw_after)
    events = []
    parse_errors = []
    with (out / "stdout.original.jsonl").open("rb") as stream:
        for line_number, line in enumerate(stream, 1):
            if not line.strip():
                continue
            try:
                event = json.loads(line)
                if not isinstance(event, dict):
                    raise ValueError("not an object")
                events.append(event)
            except (ValueError, UnicodeError) as error:
                parse_errors.append({"line": line_number, "error_type": type(error).__name__})
    test_events = [event for event in events if event.get("Test")]
    runs = [event.get("Test") for event in test_events if event.get("Action") == "run"]
    passes = [event.get("Test") for event in test_events if event.get("Action") == "pass"]
    failures = [event.get("Test") for event in events if event.get("Action") in ("fail", "skip")]
    package_pass = sum(event.get("Action") == "pass" and not event.get("Test") for event in events)
    files = [{"path": p.name, "bytes": p.stat().st_size, "sha256": digest(p.read_bytes())}
             for p in sorted(artifacts.iterdir()) if p.is_file()]
    artifact_summaries, artifact_errors = artifact_counts(artifacts)
    valid = (returncode == 0 and not timed_out and not parse_errors and not artifact_errors
             and sorted(runs) == sorted(PRODUCERS) and sorted(passes) == sorted(PRODUCERS)
             and not failures and package_pass == 1
             and before == after and witness_before == witness_after and raw_before == raw_after
             and sorted(row["path"] for row in files) == sorted(EXPECTED_ARTIFACTS))
    receipt = {
        "source": before, "argv": argv, "pid": process.pid,
        "started_utc": started_utc, "finished_utc": finished_utc,
        "elapsed_seconds": elapsed, "exit_code": returncode, "timed_out": timed_out,
        "source_after": after, "input_fingerprint_before": witness_before["fingerprint"],
        "input_fingerprint_after": witness_after["fingerprint"],
        "ran": runs, "passed": passes, "failed_or_skipped": failures,
        "package_pass_events": package_pass, "json_parse_errors": parse_errors,
        "artifact_files": files, "artifact_summaries": artifact_summaries,
        "artifact_shape_or_count_errors": artifact_errors, "execution_valid": valid,
        "raw_input_count": raw_before["count"], "raw_inputs_unchanged": raw_before == raw_after,
        "stdout_sha256": digest((out / "stdout.original.jsonl").read_bytes()),
        "stderr_sha256": digest((out / "stderr.original").read_bytes()),
        "capacity_free_before": capacity, "capacity_free_after": shutil.disk_usage("/mnt/development").free,
        "final_consumer_source": None, "status": "PASS" if valid else "FAIL",
        "artifact_semantics_pending_independent_intake": True,
        "native_or_apple_acceptance": False,
    }
    original(out / "process.original.json", receipt)
    manifest = {"files": [{"path": p.relative_to(out).as_posix(), "bytes": p.stat().st_size,
                           "sha256": digest(p.read_bytes())}
                          for p in sorted(out.rglob("*")) if p.is_file()]}
    original(out / "originals-manifest.json", manifest)
    print(json.dumps(receipt), flush=True)
    return 0 if valid else 1

if __name__ == "__main__":
    try:
        raise SystemExit(main())
    except Exception as error:
        failure = {"status": "SUPERVISOR_REJECTED", "error_type": type(error).__name__,
                   "source": PIN_SOURCE, "recorded_utc": utc(), "automatic_retry": False}
        if OWNED_OUTPUT is not None:
            original(OWNED_OUTPUT / "supervisor-rejection.original.json", failure)
        print(json.dumps(failure), file=sys.stderr)
        raise SystemExit(1)
