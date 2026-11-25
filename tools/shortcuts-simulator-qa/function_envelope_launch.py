"""One original no-input Receiver launch for the fixed function diagnostic.

This is a constrained engineering producer. It does not run an arbitrary
command, sign or import a Shortcut, retry a semantic invocation, or grant
Apple/runtime/179 acceptance. The outer QA owner must supply the owned device,
actual source, imported fixture and authenticated original acquisition.
"""
import argparse
import base64
import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import subprocess
import sys
import time
from urllib.parse import quote, unquote, urlsplit

import project_prerequisite_observation as processes

FIXTURE = "qa-function-envelope-presence-v2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbe"
MODE = "primary-no-input"
SCHEMA = "cherri-function-envelope-no-input-launch/1"
ENVIRONMENT = "CHERRI_QA_PRIMARY_NO_INPUT_CONTRACT_JSON"
RECEIVER = "org.cherrilang.qa.Receiver"
RELATIVE = "runtime/function-envelope-launch"
TRIGGERS = "runtime/function-envelope-trigger"
MAX_JSON = 256 * 1024
SOURCE_PATHS = (
    "scripts/ios27_shortcuts_simulator_qa.sh",
    "tools/shortcuts-simulator-qa/function_envelope_launch.py",
    "tools/shortcuts-simulator-qa/function_envelope_runtime.py",
    "tools/shortcuts-simulator-qa/function_envelope_observation.py",
    "tools/shortcuts-simulator-qa/function_envelope_graph.py",
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/PrimaryNoInputLaunch.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift",
    "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".cherri",
    "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json",
    "tools/shortcuts-simulator-qa/testdata/function-envelope-compiler-export.original.json",
)


def require(ok, code):
    if not ok:
        raise ValueError("FUNCTION_LAUNCH_" + code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def canonical(value):
    return json.dumps(value, ensure_ascii=False, sort_keys=True,
                      separators=(",", ":"), allow_nan=False).encode("utf-8")


def strict_json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    def invalid(_):
        raise ValueError("FUNCTION_LAUNCH_NONFINITE_JSON")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def fixed_query(raw, scheme, host, path, keys):
    require(isinstance(raw, str) and 0 < len(raw.encode("utf-8")) <= 8192
            and all(0x21 <= ord(ch) <= 0x7e for ch in raw)
            and re.search(r"%(?![0-9A-Fa-f]{2})", raw) is None, "URL_BYTES")
    require(raw.startswith(scheme + "://" + host + path + "?"), "URL_ROUTE")
    parts = urlsplit(raw)
    require(parts.scheme == scheme and parts.netloc == host and parts.path == path
            and parts.fragment == "" and parts.username is None
            and parts.password is None and parts.port is None, "URL_ROUTE")
    pairs = parts.query.split("&")
    require(len(pairs) == len(keys), "URL_KEYS")
    values = {}
    for pair in pairs:
        require("=" in pair, "URL_VALUE")
        key, value = pair.split("=", 1)
        # URLComponents query items do not apply form-urlencoded '+' coercion.
        key, value = (unquote(v, encoding="utf-8", errors="strict") for v in (key, value))
        require(key in keys and key not in values, "URL_KEYS")
        values[key] = value
    require(set(values) == set(keys), "URL_KEYS")
    return values


def validate_contract(raw):
    require(isinstance(raw, bytes) and 0 < len(raw) <= 65536, "CONTRACT_SIZE")
    value = strict_json(raw)
    require(isinstance(value, dict) and set(value) == {
        "schema", "mode", "fixtureId", "workflowName", "runNonce", "primaryURL"}
        and all(isinstance(v, str) for v in value.values()), "CONTRACT_FIELDS")
    require(canonical(value) == raw, "CONTRACT_CANONICAL")
    require((value["schema"], value["mode"], value["fixtureId"], value["workflowName"])
            == (SCHEMA, MODE, FIXTURE, WORKFLOW), "CONTRACT_IDENTITY")
    nonce = value["runNonce"]
    require(re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce) is not None, "NONCE")
    outer = fixed_query(value["primaryURL"], "shortcuts", "x-callback-url",
                        "/run-shortcut", ("name", "x-source", "x-success", "x-error", "x-cancel"))
    require(outer["name"].encode() == WORKFLOW.encode()
            and outer["x-source"].encode() == b"CherriSimulatorQAReceiver", "PRIMARY_IDENTITY")
    for key, host in (("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel")):
        callback = fixed_query(outer[key], "cherriqa", host, "",
                               ("runId", "fixtureId", "shortcutName", "source"))
        require(callback == {"runId": nonce, "fixtureId": FIXTURE,
                             "shortcutName": WORKFLOW, "source": "shortcuts-x-callback"},
                "CALLBACK_IDENTITY")
    return value


def make_contract(nonce):
    require(isinstance(nonce, str)
            and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce) is not None, "NONCE")
    def query(values):
        return "&".join(quote(k, safe="") + "=" + quote(v, safe="") for k, v in values)
    callback = query((("runId", nonce), ("fixtureId", FIXTURE),
                      ("shortcutName", WORKFLOW), ("source", "shortcuts-x-callback")))
    primary = "shortcuts://x-callback-url/run-shortcut?" + query((
        ("name", WORKFLOW), ("x-source", "CherriSimulatorQAReceiver"),
        ("x-success", "cherriqa://success?" + callback),
        ("x-error", "cherriqa://error?" + callback),
        ("x-cancel", "cherriqa://cancel?" + callback)))
    raw = canonical({"schema": SCHEMA, "mode": MODE, "fixtureId": FIXTURE,
                     "workflowName": WORKFLOW, "runNonce": nonce, "primaryURL": primary})
    validate_contract(raw)
    return raw


def regular(path, maximum=MAX_JSON, empty=False):
    path = Path(path).absolute()
    require(path.is_file() and not path.is_symlink()
            and all(not p.is_symlink() for p in path.parents), "ORIGINAL_FILE")
    require(path.stat().st_size <= maximum, "ORIGINAL_SIZE")
    raw = path.read_bytes()
    require((empty or bool(raw)) and len(raw) <= maximum, "ORIGINAL_SIZE")
    return raw


def write_new(path, raw):
    require(isinstance(raw, bytes) and len(raw) <= MAX_JSON, "WRITE_SIZE")
    path = Path(path)
    require(path.parent.is_dir() and not path.parent.is_symlink()
            and all(not p.is_symlink() for p in path.parents), "WRITE_PARENT")
    fd = os.open(path, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, "wb") as f:
        f.write(raw)
        f.flush()
        os.fsync(f.fileno())
    require(regular(path, empty=True) == raw, "WRITE_READBACK")


def record(path, value):
    write_new(path, canonical(value) + b"\n")


def owned_root(root):
    root = Path(root).absolute()
    require(root.is_dir() and not root.is_symlink()
            and all(not p.is_symlink() for p in root.parents), "ROOT")
    return root


def source_snapshot(repo, commit):
    repo = owned_root(repo)
    require(isinstance(commit, str) and re.fullmatch(r"[a-f0-9]{40}", commit), "SOURCE_COMMIT")
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_CONFIG_NOSYSTEM="1",
               GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
    deadline = time.monotonic() + 10
    def git(*args):
        remaining = deadline - time.monotonic()
        require(remaining > 0, "SOURCE_DEADLINE")
        return subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), *args],
            env=env, stdin=subprocess.DEVNULL, timeout=remaining)
    require(git("rev-parse", "HEAD").decode().strip() == commit
            and not git("status", "--porcelain=v1", "--untracked-files=all"), "SOURCE_CLEAN")
    # Replacements, grafts and object alternates are not part of this authority.
    require(not git("for-each-ref", "--format=%(refname)", "refs/replace"), "SOURCE_REPLACEMENT")
    for name in ("info/grafts", "objects/info/alternates"):
        path = Path(git("rev-parse", "--git-path", name).decode().strip())
        if not path.is_absolute():
            path = repo / path
        require(not path.exists() and not path.is_symlink(), "SOURCE_OBJECT_REDIRECT")
    require(all(row.startswith(b"H ") for row in git("ls-files", "-v", "-z").split(b"\0") if row),
            "SOURCE_HIDDEN_INDEX_FLAGS")
    rows = []
    for name in SOURCE_PATHS:
        expected = git("show", commit + ":" + name)
        require(regular(repo / name, 2 * 1024 * 1024) == expected, "SOURCE_BYTES")
        rows.append({"path": name, "bytes": len(expected), "sha256": sha(expected)})
    return {"commit": commit, "tree": git("rev-parse", "HEAD^{tree}").decode().strip(),
            "sourceClean": True, "files": rows}


def prepare(repo, root, harness_commit, compiler_commit, run_id, attempt, nonce, udid):
    root, repo = owned_root(root), owned_root(repo)
    require(root != repo and repo not in root.parents and root not in repo.parents, "OUTPUT_OUTSIDE_SOURCE")
    require(isinstance(run_id, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", run_id)
            and type(attempt) is int and 1 <= attempt <= 100, "RUN_CONTEXT")
    require(isinstance(udid, str) and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid), "DEVICE")
    contract_raw = make_contract(nonce)
    contract = validate_contract(contract_raw)
    require(isinstance(compiler_commit, str) and re.fullmatch(r"[a-f0-9]{40}", compiler_commit),
            "COMPILER_COMMIT")
    before = source_snapshot(repo, harness_commit)
    parent = owned_root(root / "runtime")
    target = parent / "function-envelope-launch"
    target.mkdir(mode=0o700)
    write_new(target / "contract.original.json", contract_raw)
    write_new(target / "primary-url.original.txt", contract["primaryURL"].encode("utf-8"))
    record(target / "source.before.original.json", before)
    context = {"schema": "cherri-function-envelope-launch-preparation/1",
               "harnessCommitSha": harness_commit, "compilerCommitSha": compiler_commit,
               "runId": run_id, "runAttempt": attempt,
               "fixtureId": FIXTURE, "workflowName": WORKFLOW, "mode": MODE,
               "runNonce": nonce, "udid": udid, "contractSHA256": sha(contract_raw),
               "contractBytes": len(contract_raw), "preparedUTC": now(),
               "semanticAttemptsAllowed": 1, "fallbackAllowed": False,
               "sourceSnapshotSHA256": sha(regular(target / "source.before.original.json")),
               "nativeExecutionGranted": False, "formalAcceptanceGranted": False}
    record(target / "prepared.original.json", context)
    return context


def load_prepared(root):
    target = owned_root(root) / RELATIVE
    context_raw = regular(target / "prepared.original.json")
    context = strict_json(context_raw)
    contract_raw = regular(target / "contract.original.json", 65536)
    contract = validate_contract(contract_raw)
    require(context["schema"] == "cherri-function-envelope-launch-preparation/1"
            and isinstance(context["compilerCommitSha"], str)
            and re.fullmatch(r"[0-9a-f]{40}", context["compilerCommitSha"])
            and isinstance(context["harnessCommitSha"], str)
            and re.fullmatch(r"[0-9a-f]{40}", context["harnessCommitSha"])
            and isinstance(context["runId"], str)
            and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", context["runId"])
            and type(context["runAttempt"]) is int and 1 <= context["runAttempt"] <= 100
            and isinstance(context["udid"], str)
            and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", context["udid"])
            and context["fixtureId"] == FIXTURE
            and context["workflowName"] == WORKFLOW and context["mode"] == MODE
            and context["runNonce"] == contract["runNonce"]
            and context["contractSHA256"] == sha(contract_raw)
            and context["contractBytes"] == len(contract_raw)
            and type(context["semanticAttemptsAllowed"]) is int
            and context["semanticAttemptsAllowed"] == 1
            and context["fallbackAllowed"] is False, "PREPARATION")
    require(regular(target / "primary-url.original.txt", 8192)
            == contract["primaryURL"].encode("utf-8"), "PRIMARY_BYTES")
    before_raw = regular(target / "source.before.original.json")
    require(sha(before_raw) == context["sourceSnapshotSHA256"], "PREPARED_SOURCE")
    return target, context, contract_raw, contract, strict_json(before_raw)


def command_for(context, contract_raw, contract):
    return ["/usr/bin/env",
            "SIMCTL_CHILD_" + ENVIRONMENT + "=" + contract_raw.decode("utf-8"),
            "SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL=" + contract["primaryURL"],
            "/usr/bin/xcrun", "simctl", "launch", context["udid"], RECEIVER]


def execute(repo, root):
    require(sys.platform == "darwin", "ACTUAL_DARWIN_REQUIRED")
    started = time.monotonic()
    deadline = started + 35
    target, context, contract_raw, contract, before = load_prepared(root)
    require(source_snapshot(repo, context["harnessCommitSha"]) == before, "SOURCE_BEFORE_LAUNCH")
    # The fixed graph, synthetic source, signed bytes and manifest are required
    # before opening the receiver. This is still not an import/execution grant.
    from function_envelope_runtime import provenance
    bindings = provenance(repo, root, FIXTURE, context["compilerCommitSha"], context["harnessCommitSha"])
    argv = command_for(context, contract_raw, contract)
    claim = {"schema": "cherri-function-envelope-semantic-launch-claim/1",
             "startedUTC": now(), "contextSHA256": sha(regular(target / "prepared.original.json")),
             "contractSHA256": sha(contract_raw), "command": argv,
             "harnessCommitSha": context["harnessCommitSha"], "compilerCommitSha": context["compilerCommitSha"],
             "runNonce": context["runNonce"],
             "attemptOrdinal": 1, "semanticAttemptCount": 1, "automaticRetry": False,
             "fallbackAllowed": False, "processTimeoutSeconds": 25,
             "wholeLaunchOwnerSeconds": 35, "sourceAndSignatureBindings": bindings}
    # This claim also blocks restarting the Python owner for the same run.
    record(target / "attempt.claim.original.json", claim)
    result = {"schema": "cherri-function-envelope-launch-result/1", "status": "FAIL",
              "contextSHA256": claim["contextSHA256"], "contractSHA256": sha(contract_raw),
              "harnessCommitSha": context["harnessCommitSha"], "compilerCommitSha": context["compilerCommitSha"],
             "runNonce": context["runNonce"],
              "command": argv, "semanticAttemptCount": 1, "automaticRetry": False,
              "fallbackAllowed": False, "nativeExecutionAuthenticated": False,
              "formalAcceptanceGranted": False}
    observed = details = None
    try:
        with (target / "stdout.original").open("xb") as out, (target / "stderr.original").open("xb") as err:
            observed, details = processes.capture(
                argv, 25, output=out, error_output=err, hard_deadline=deadline,
                stream_limit=64 * 1024,
                journal=lambda event, row: record(target / ("process-" + event + ".original.json"), row))
        record(target / "process.original.json", {"command": argv, "observation": observed, "details": details})
        require(observed["result"] == "PASS" and type(observed["exitCode"]) is int
                and observed["exitCode"] == 0 and details["cleanupParentReaped"] is True
                and details["groupPresentAfterCleanup"] is False, "LAUNCH_PROCESS")
        output = regular(target / "stdout.original", 65537, empty=True)
        ids = re.findall(rb"(?m)^org\.cherrilang\.qa\.Receiver: ([1-9][0-9]*)\r?$", output)
        require(len(ids) == 1, "RECEIVER_PID")
        result["receiverPID"] = int(ids[0])
        result["status"] = "PASS"
    except BaseException as exc:
        result["errorType"] = type(exc).__name__
        result["reason"] = str(exc)[:512]
        raise
    finally:
        result["sourceUnchanged"] = False
        try:
            after = source_snapshot(repo, context["harnessCommitSha"])
            record(target / "source.after.original.json", after)
            result["sourceUnchanged"] = after == before
        except BaseException as exc:
            result["sourceRecheckError"] = type(exc).__name__ + ":" + str(exc)[:512]
        result["elapsedSeconds"] = time.monotonic() - started
        if not result["sourceUnchanged"] or result["elapsedSeconds"] >= 35:
            result["status"] = "FAIL"
        for name in ("stdout.original", "stderr.original", "process.original.json",
                     "process-started.original.json", "process-finished.original.json"):
            path = target / name
            if path.is_file():
                data = regular(path, 65537 if name in ("stdout.original", "stderr.original") else MAX_JSON, empty=True)
                result[name] = {"bytes": len(data), "sha256": sha(data)}
        record(target / "result.original.json", result)
    return result


def collect_receiver(root, documents):
    target, context, contract_raw, contract, before = load_prepared(root)
    source = owned_root(documents) / "primary-no-input-launch" / sha(contract_raw)
    source = owned_root(source)
    paths = sorted(source.iterdir())
    require(4 <= len(paths) <= 34 and all(p.is_file() and not p.is_symlink() for p in paths),
            "TRIGGER_INVENTORY")
    require({p.name for p in paths} >= {"owner.original.json", "contract.original.json"},
            "TRIGGER_ORIGINALS")
    require(all(p.name in ("owner.original.json", "contract.original.json")
                or re.fullmatch(r"[0-9]{3}-(?:intent|completion|rejection)\.original\.json", p.name)
                for p in paths), "TRIGGER_UNKNOWN_FILE")
    destination = owned_root(root) / TRIGGERS
    destination.mkdir(mode=0o700)
    rows = []
    for path in paths:
        data = regular(path)
        write_new(destination / path.name, data)
        rows.append({"path": path.name, "bytes": len(data), "sha256": sha(data)})
    require(sorted(p.name for p in source.iterdir()) == [p.name for p in paths], "TRIGGER_CHANGED")
    for row in rows:
        require(sha(regular(source / row["path"])) == row["sha256"], "TRIGGER_CHANGED")
    acquisition = {"schema": "cherri-function-envelope-trigger-original-acquisition/1",
                   "collectedUTC": now(), "contractSHA256": sha(contract_raw),
                   "sourceDirectory": str(source), "files": rows,
                   "byteIdentical": True, "nativeExecutionAuthenticated": False,
                   "formalAcceptanceGranted": False}
    record(owned_root(root) / "runtime/function-envelope-trigger-acquisition.original.json", acquisition)
    return acquisition


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="operation", required=True)
    prepare_parser = commands.add_parser("prepare")
    for name in ("repo", "root", "harness-source", "compiler-source", "run-id", "nonce", "udid"):
        prepare_parser.add_argument("--" + name, required=True)
    prepare_parser.add_argument("--attempt", type=int, required=True)
    run_parser = commands.add_parser("launch")
    run_parser.add_argument("--repo", required=True)
    run_parser.add_argument("--root", required=True)
    collect_parser = commands.add_parser("collect")
    collect_parser.add_argument("--root", required=True)
    collect_parser.add_argument("--documents", required=True)
    args = parser.parse_args()
    if args.operation == "prepare":
        result = prepare(args.repo, args.root, args.harness_source, args.compiler_source,
                         args.run_id, args.attempt, args.nonce, args.udid)
    elif args.operation == "launch":
        result = execute(args.repo, args.root)
    else:
        result = collect_receiver(args.root, args.documents)
    print(json.dumps(result, sort_keys=True))
    return 1 if result.get("status") == "FAIL" else 0


if __name__ == "__main__":
    raise SystemExit(main())
