"""A separate, single no-input launch for the qualified R2 function fixture.

The b82 launch module remains unchanged. Only its scalar/JSON/path helpers and
the reviewed process capture are reused. R2 compiler, harness and built app
identities remain separate; this module never grants Apple or formal acceptance.
"""
import argparse
import os
from pathlib import Path
import re
import subprocess
import sys
import time
from urllib.parse import quote

from function_envelope_launch import (
    canonical, fixed_query, now, owned_root, record, regular, sha, strict_json,
    write_new,
)
import project_prerequisite_observation as processes

FIXTURE = "qa-function-envelope-presence-v2-r2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbeR2"
COMPILER_SOURCE = "9b350816f27cf06bf7194e19a1dcc5a97c94f42b"
SCHEMA = "cherri-function-envelope-no-input-launch/1"
MODE = "primary-no-input"
ENVIRONMENT = "CHERRI_QA_PRIMARY_NO_INPUT_CONTRACT_JSON"
RECEIVER = "org.cherrilang.qa.Receiver"
RELATIVE = "runtime/function-envelope-r2-launch"
TRIGGERS = "runtime/function-envelope-r2-trigger"
MAX_JSON = 256 * 1024
SOURCE_PATHS = (
    "tools/shortcuts-simulator-qa/function_envelope_r2_launch.py",
    "tools/shortcuts-simulator-qa/function_envelope_r2_runtime.py",
    "tools/shortcuts-simulator-qa/function_envelope_r2_graph.py",
    "tools/shortcuts-simulator-qa/function_envelope_launch.py",
    "tools/shortcuts-simulator-qa/function_envelope_observation.py",
    "tools/shortcuts-simulator-qa/function_envelope_graph.py",
    "tools/shortcuts-simulator-qa/native_comparison_graph.py",
    "tools/shortcuts-simulator-qa/nested_list_runtime.py",
    "tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
    "tools/shortcuts-simulator-qa/XcodeProject/Shared/PrimaryNoInputLaunch.swift",
    "tools/shortcuts-simulator-qa/XcodeProject/ReceiverApp/ReceiverApp.swift",
    "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".cherri",
    "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json",
    "tools/shortcuts-simulator-qa/testdata/function-envelope-r2-compiler-export.original.json",
)


def require(ok, code):
    if not ok:
        raise ValueError("FUNCTION_R2_LAUNCH_" + code)


def validate_contract(raw):
    require(isinstance(raw, bytes) and 0 < len(raw) <= 65536, "CONTRACT_SIZE")
    value = strict_json(raw)
    keys = {"schema", "mode", "fixtureId", "workflowName", "runNonce", "primaryURL"}
    require(isinstance(value, dict) and set(value) == keys
            and all(isinstance(x, str) for x in value.values()), "CONTRACT_FIELDS")
    require(canonical(value) == raw, "CONTRACT_CANONICAL")
    require((value["schema"], value["mode"], value["fixtureId"], value["workflowName"])
            == (SCHEMA, MODE, FIXTURE, WORKFLOW), "CONTRACT_IDENTITY")
    nonce = value["runNonce"]
    require(re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce) is not None, "NONCE")
    outer = fixed_query(value["primaryURL"], "shortcuts", "x-callback-url",
                        "/run-shortcut", ("name", "x-source", "x-success", "x-error", "x-cancel"))
    require(outer["name"].encode("utf-8") == WORKFLOW.encode("utf-8")
            and outer["x-source"] == "CherriSimulatorQAReceiver", "PRIMARY_IDENTITY")
    for key, host in (("x-success", "success"), ("x-error", "error"), ("x-cancel", "cancel")):
        row = fixed_query(outer[key], "cherriqa", host, "",
                          ("runId", "fixtureId", "shortcutName", "source"))
        require(row == {"runId": nonce, "fixtureId": FIXTURE, "shortcutName": WORKFLOW,
                        "source": "shortcuts-x-callback"}, "CALLBACK_IDENTITY")
    return value


def make_contract(nonce):
    require(isinstance(nonce, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce), "NONCE")
    def query(values):
        return "&".join(quote(k, safe="") + "=" + quote(v, safe="") for k, v in values)
    callback = query((("runId", nonce), ("fixtureId", FIXTURE),
                      ("shortcutName", WORKFLOW), ("source", "shortcuts-x-callback")))
    url = "shortcuts://x-callback-url/run-shortcut?" + query((
        ("name", WORKFLOW), ("x-source", "CherriSimulatorQAReceiver"),
        ("x-success", "cherriqa://success?" + callback),
        ("x-error", "cherriqa://error?" + callback),
        ("x-cancel", "cherriqa://cancel?" + callback)))
    raw = canonical({"schema": SCHEMA, "mode": MODE, "fixtureId": FIXTURE,
                     "workflowName": WORKFLOW, "runNonce": nonce, "primaryURL": url})
    validate_contract(raw)
    return raw


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
    require(not git("for-each-ref", "--format=%(refname)", "refs/replace"), "SOURCE_REPLACEMENT")
    for name in ("info/grafts", "objects/info/alternates"):
        path = Path(git("rev-parse", "--git-path", name).decode().strip())
        if not path.is_absolute():
            path = repo / path
        require(not path.exists() and not path.is_symlink(), "SOURCE_OBJECT_REDIRECT")
    require(all(row.startswith(b"H ") for row in git("ls-files", "-v", "-z").split(b"\0") if row),
            "SOURCE_HIDDEN_INDEX_FLAGS")
    files = []
    for name in SOURCE_PATHS:
        expected = git("show", commit + ":" + name)
        require(regular(repo / name, 2 * 1024 * 1024) == expected, "SOURCE_BYTES")
        files.append({"path": name, "bytes": len(expected), "sha256": sha(expected)})
    return {"commit": commit, "tree": git("rev-parse", "HEAD^{tree}").decode().strip(),
            "sourceClean": True, "files": files}


def prepare(repo, root, harness_commit, compiler_commit, run_id, attempt, nonce, udid):
    repo, root = owned_root(repo), owned_root(root)
    require(root != repo and repo not in root.parents and root not in repo.parents, "OUTPUT_OUTSIDE_SOURCE")
    require(compiler_commit == COMPILER_SOURCE, "COMPILER_IDENTITY")
    require(isinstance(run_id, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", run_id)
            and type(attempt) is int and 1 <= attempt <= 100, "RUN_CONTEXT")
    require(isinstance(udid, str)
            and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", udid), "DEVICE")
    raw = make_contract(nonce)
    contract = validate_contract(raw)
    before = source_snapshot(repo, harness_commit)
    target = owned_root(root / "runtime") / "function-envelope-r2-launch"
    target.mkdir(mode=0o700)
    write_new(target / "contract.original.json", raw)
    write_new(target / "primary-url.original.txt", contract["primaryURL"].encode("utf-8"))
    record(target / "source.before.original.json", before)
    context = {
        "schema": "cherri-function-envelope-r2-launch-preparation/1",
        "harnessCommitSha": harness_commit, "compilerCommitSha": compiler_commit,
        "runId": run_id, "runAttempt": attempt, "fixtureId": FIXTURE,
        "workflowName": WORKFLOW, "mode": MODE, "runNonce": nonce, "udid": udid,
        "contractSHA256": sha(raw), "contractBytes": len(raw), "preparedUTC": now(),
        "semanticAttemptsAllowed": 1, "fallbackAllowed": False,
        "sourceSnapshotSHA256": sha(regular(target / "source.before.original.json")),
        "nativeExecutionGranted": False, "formalAcceptanceGranted": False}
    record(target / "prepared.original.json", context)
    return context


def load_prepared(root):
    target = owned_root(root) / RELATIVE
    context = strict_json(regular(target / "prepared.original.json"))
    raw = regular(target / "contract.original.json", 65536)
    contract = validate_contract(raw)
    require(context.get("schema") == "cherri-function-envelope-r2-launch-preparation/1"
            and context.get("compilerCommitSha") == COMPILER_SOURCE
            and isinstance(context.get("harnessCommitSha"), str)
            and re.fullmatch(r"[a-f0-9]{40}", context["harnessCommitSha"])
            and isinstance(context.get("runId"), str)
            and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", context["runId"])
            and type(context.get("runAttempt")) is int and 1 <= context["runAttempt"] <= 100
            and isinstance(context.get("udid"), str)
            and re.fullmatch(r"[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}", context["udid"])
            and context.get("fixtureId") == FIXTURE and context.get("workflowName") == WORKFLOW
            and context.get("mode") == MODE and context.get("runNonce") == contract["runNonce"]
            and context.get("contractSHA256") == sha(raw) and type(context.get("contractBytes")) is int
            and context["contractBytes"] == len(raw)
            and type(context.get("semanticAttemptsAllowed")) is int
            and context["semanticAttemptsAllowed"] == 1 and context.get("fallbackAllowed") is False
            and context.get("nativeExecutionGranted") is False
            and context.get("formalAcceptanceGranted") is False, "PREPARATION")
    require(regular(target / "primary-url.original.txt", 8192)
            == contract["primaryURL"].encode("utf-8"), "PRIMARY_BYTES")
    before = regular(target / "source.before.original.json")
    require(sha(before) == context.get("sourceSnapshotSHA256"), "PREPARED_SOURCE")
    return target, context, raw, contract, strict_json(before)


def command_for(context, raw, contract):
    require(validate_contract(raw) == contract, "COMMAND_CONTRACT")
    return ["/usr/bin/env", "SIMCTL_CHILD_" + ENVIRONMENT + "=" + raw.decode("utf-8"),
            "SIMCTL_CHILD_CHERRI_TRIGGER_SHORTCUT_URL=" + contract["primaryURL"],
            "/usr/bin/xcrun", "simctl", "launch", context["udid"], RECEIVER]


def execute(repo, root):
    require(sys.platform == "darwin", "ACTUAL_DARWIN_REQUIRED")
    started = time.monotonic()
    deadline = started + 35
    target, context, raw, contract, before = load_prepared(root)
    require(source_snapshot(repo, context["harnessCommitSha"]) == before, "SOURCE_BEFORE_LAUNCH")
    from function_envelope_r2_runtime import provenance
    bindings = provenance(repo, root, FIXTURE, context["compilerCommitSha"], context["harnessCommitSha"])
    argv = command_for(context, raw, contract)
    claim = {
        "schema": "cherri-function-envelope-r2-semantic-launch-claim/1",
        "startedUTC": now(), "contextSHA256": sha(regular(target / "prepared.original.json")),
        "contractSHA256": sha(raw), "command": argv,
        "harnessCommitSha": context["harnessCommitSha"], "compilerCommitSha": context["compilerCommitSha"],
        "runNonce": context["runNonce"], "attemptOrdinal": 1, "semanticAttemptCount": 1,
        "automaticRetry": False, "fallbackAllowed": False, "processTimeoutSeconds": 25,
        "wholeLaunchOwnerSeconds": 35, "sourceAndSignatureBindings": bindings}
    record(target / "attempt.claim.original.json", claim)
    result = {
        "schema": "cherri-function-envelope-r2-launch-result/1", "status": "FAIL",
        "contextSHA256": claim["contextSHA256"], "contractSHA256": sha(raw),
        "harnessCommitSha": context["harnessCommitSha"], "compilerCommitSha": context["compilerCommitSha"],
        "runNonce": context["runNonce"], "command": argv, "semanticAttemptCount": 1,
        "automaticRetry": False, "fallbackAllowed": False,
        "nativeExecutionAuthenticated": False, "formalAcceptanceGranted": False}
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
        result.update(receiverPID=int(ids[0]), status="PASS")
    except BaseException as exc:
        result.update(errorType=type(exc).__name__, reason=str(exc)[:512])
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
            p = target / name
            if p.is_file():
                data = regular(p, 65537 if name in ("stdout.original", "stderr.original") else MAX_JSON, empty=True)
                result[name] = {"bytes": len(data), "sha256": sha(data)}
        record(target / "result.original.json", result)
    return result


def collect_receiver(root, documents):
    target, context, raw, contract, before = load_prepared(root)
    source = owned_root(owned_root(documents) / "primary-no-input-launch" / sha(raw))
    paths = sorted(source.iterdir())
    require(4 <= len(paths) <= 34 and all(p.is_file() and not p.is_symlink() for p in paths),
            "TRIGGER_INVENTORY")
    require({p.name for p in paths} >= {"owner.original.json", "contract.original.json"}
            and all(p.name in ("owner.original.json", "contract.original.json")
                    or re.fullmatch(r"[0-9]{3}-(?:intent|completion|rejection)\.original\.json", p.name)
                    for p in paths), "TRIGGER_ORIGINALS")
    destination = owned_root(root) / TRIGGERS
    destination.mkdir(mode=0o700)
    rows = []
    for p in paths:
        data = regular(p)
        write_new(destination / p.name, data)
        rows.append({"path": p.name, "bytes": len(data), "sha256": sha(data)})
    require(sorted(p.name for p in source.iterdir()) == [p.name for p in paths], "TRIGGER_CHANGED")
    for row in rows:
        require(sha(regular(source / row["path"])) == row["sha256"], "TRIGGER_CHANGED")
    result = {
        "schema": "cherri-function-envelope-r2-trigger-original-acquisition/1",
        "collectedUTC": now(), "contractSHA256": sha(raw), "sourceDirectory": str(source),
        "files": rows, "byteIdentical": True, "nativeExecutionAuthenticated": False,
        "formalAcceptanceGranted": False}
    record(owned_root(root) / "runtime/function-envelope-r2-trigger-acquisition.original.json", result)
    return result


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="operation", required=True)
    prep = commands.add_parser("prepare")
    for name in ("repo", "root", "harness-source", "compiler-source", "run-id", "nonce", "udid"):
        prep.add_argument("--" + name, required=True)
    prep.add_argument("--attempt", type=int, required=True)
    run = commands.add_parser("launch")
    run.add_argument("--repo", required=True)
    run.add_argument("--root", required=True)
    collect = commands.add_parser("collect")
    collect.add_argument("--root", required=True)
    collect.add_argument("--documents", required=True)
    args = parser.parse_args()
    if args.operation == "prepare":
        value = prepare(args.repo, args.root, args.harness_source, args.compiler_source,
                        args.run_id, args.attempt, args.nonce, args.udid)
    elif args.operation == "launch":
        value = execute(args.repo, args.root)
    else:
        value = collect_receiver(args.root, args.documents)
    print(canonical(value).decode("utf-8"))
    return 1 if value.get("status") == "FAIL" else 0


if __name__ == "__main__":
    raise SystemExit(main())
