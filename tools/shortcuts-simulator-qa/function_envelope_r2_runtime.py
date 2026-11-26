"""Original R2 graph, signing, one-shot launch and callback joins.

The historical b82 export and runtime are not relabeled. The R2 compiler export
is retained with its historical FAILED/PENDING fields; the independent graph
validator and actual native owner supply their own separate authority.
"""
import base64
import datetime
import os
from pathlib import Path
import re
import subprocess
import time

import function_envelope_r2_graph as graph
import function_envelope_r2_launch as launch
import function_envelope_observation as grammar
from nested_list_runtime import read, trace_bytes

FIXTURE = "qa-function-envelope-presence-v2-r2"
WORKFLOW = "CherriFunctionEnvelopePresenceProbeR2"
COMPILER_SOURCE = "9b350816f27cf06bf7194e19a1dcc5a97c94f42b"
COMPILER_TREE = "6ba3ca788ffcd8b5bc2ddaedaff1a22463aa7253"
EXPORT_SHA256 = "4c694c7054fe640a78a5e15e3546546d31f8d2e34d70f633b156de2d492f7c10"
UNSIGNED_SHA256 = "65ebfde24e0653ddf35e6277adde9a2f7e3835207eb2c403f0bed241d17372e9"
SOURCE_SHA256 = "8c044ebef8e0ecd1481a99eb030ef843de2a9ddc4b4fb129545501d1bdb28be1"
CONTRACT_SHA256 = "f27cc210f0656118d650d53a05f14d7ae2843e600c78b84a9b0e7823e73673de"
MODE = "FUNCTION_ENVELOPE_R2_ENGINEERING_OBSERVATION_ONLY"
MAX_CALLBACK = 256 * 1024


def require(ok, code):
    if not ok:
        raise ValueError("FUNCTION_R2_RUNTIME_" + code)


sha = launch.sha
strict_json = launch.strict_json


def same(a, b):
    return launch.canonical(a) == launch.canonical(b)


def prepared(repo, fixture, source, unsigned):
    require(fixture == FIXTURE, "FIXTURE")
    contract = read(repo, "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json", 8192)
    require(sha(source) == SOURCE_SHA256 and sha(contract) == CONTRACT_SHA256
            and sha(unsigned) == UNSIGNED_SHA256, "PREPARED_BYTES")
    proof = graph.validate_compiled(source, contract, unsigned)
    require(proof["actionCount"] == 379, "ACTION_COUNT")
    return {
        "fixtureId": FIXTURE, "workflowName": WORKFLOW, "sourceSha256": SOURCE_SHA256,
        "expectedContractSha256": CONTRACT_SHA256, "unsignedPlistSha256": sha(unsigned),
        "graph": proof, "requiredLaunchMode": "primary-no-input",
        "firstAndOnlySemanticInvocation": True, "inputChangingFallbackAllowed": False,
        "diagnosticOnly": True, "formalAcceptanceGranted": False}


def validate_compiler_export(raw, source, unsigned, compiler):
    require(compiler == COMPILER_SOURCE and sha(raw) == EXPORT_SHA256, "COMPILER_EXPORT_BYTES")
    value = strict_json(raw)
    expected = {
        "schema": "cherri-function-presence-r2-compiler-export/1",
        "compilerCommitSha": COMPILER_SOURCE, "compilerTreeSha": COMPILER_TREE,
        "fixtureId": FIXTURE, "workflowName": WORKFLOW, "sourceSha256": sha(source),
        "unsignedSha256": sha(unsigned), "unsignedBytes": len(unsigned), "actionCount": 379,
        "goTest": "TestP15FunctionEnvelopePresenceProbeR2Preparation",
        "nativeInvocation": False, "formalAcceptance": False,
        "ownerStatusPreserved": "FAILED",
        "fullGraphValidation": "PENDING_SEPARATE_QUALIFICATION",
        "independentIntakeSha256": "411161d46d83d26cb2362cf5d94c569c9742c3485090661378425247b7745d5a"}
    require(sha(source) == SOURCE_SHA256 and sha(unsigned) == UNSIGNED_SHA256
            and len(unsigned) == 269874, "RETAINED_COMPILER_ARTIFACT")
    for key, wanted in expected.items():
        require(same(value.get(key), wanted), "COMPILER_EXPORT_" + key)
    return value


def provenance(repo, root, fixture, compiler, harness):
    require(fixture == FIXTURE and compiler == COMPILER_SOURCE
            and isinstance(harness, str) and re.fullmatch(r"[a-f0-9]{40}", harness), "SOURCE_IDENTITY")
    paths = {
        "compilerManifest": "compiler-manifest.json",
        "compiledSource": "builds/" + WORKFLOW + ".cherri",
        "unsignedShortcut": "builds/" + WORKFLOW + ".shortcut",
        "signedShortcut": "builds/signed/" + WORKFLOW + ".shortcut",
        "signingProvenance": "runtime/signing-provenance.json",
        "compilerExportOriginal": "builds/function-envelope-r2-compiler-export.original.json"}
    raw = {key: read(root, path, 10 * 1024 * 1024 if key == "signedShortcut" else 2 * 1024 * 1024)
           for key, path in paths.items()}
    validate_compiler_export(raw["compilerExportOriginal"], raw["compiledSource"],
                             raw["unsignedShortcut"], compiler)
    manifest = strict_json(raw["compilerManifest"])
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == compiler
            and manifest.get("harnessCommitSha") == harness
            and manifest.get("compilerArtifactOrigin") == "retained-original-r2-preparation",
            "MANIFEST_IDENTITY")
    rows = manifest.get("fixtures")
    require(isinstance(rows, list) and len(rows) == 1 and isinstance(rows[0], dict)
            and rows[0].get("fixtureId") == FIXTURE, "MANIFEST_EXACT_FIXTURE")
    proof = prepared(repo, fixture, raw["compiledSource"], raw["unsignedShortcut"])
    expected = {
        "fixtureId": FIXTURE, "fileName": WORKFLOW + ".cherri", "workflowName": WORKFLOW,
        "dialect": "v2", "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
        "sourceSha256": SOURCE_SHA256, "unsignedPlistSha256": UNSIGNED_SHA256,
        "nativeActionCount": 379, "functionEnvelopeR2Probe": proof}
    for key, wanted in expected.items():
        require(same(rows[0].get(key), wanted), "PREPARED_" + key)
    signed = raw["signedShortcut"]
    require(signed.startswith(b"AEA1") and len(signed) >= 16, "SIGNED_BYTES")
    signing = strict_json(raw["signingProvenance"])
    require(isinstance(signing, dict) and signing.get("compilerCommitSha") == compiler
            and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == sha(signed)
            and type(signing.get("signedByteSize")) is int
            and signing["signedByteSize"] == len(signed), "SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(isinstance(target, dict), "SIGNING_TARGET")
    for key, wanted in {
        "compilerCommitSha": compiler, "sourceSha256": SOURCE_SHA256,
        "unsignedPlistSha256": UNSIGNED_SHA256, "unsignedByteSize": 269874,
        "nativeActionCount": 379, "signedArtifactSha256": sha(signed),
        "signedByteSize": len(signed), "headerMagic": "AEA1",
    }.items():
        require(same(target.get(key), wanted), "SIGNED_" + key)
    return {key: {"path": paths[key], "bytes": len(data), "sha256": sha(data)}
            for key, data in raw.items()}


def utc(value):
    require(isinstance(value, str) and len(value) <= 64, "TIME")
    stamp = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(stamp.tzinfo is not None and stamp.utcoffset() == datetime.timedelta(), "TIME_ZONE")
    return stamp


def verify_source_snapshot(repo, snapshot, commit):
    require(isinstance(snapshot, dict) and snapshot.get("commit") == commit
            and snapshot.get("sourceClean") is True, "LAUNCH_SOURCE")
    files = snapshot.get("files")
    require(isinstance(files, list) and [r.get("path") for r in files] == list(launch.SOURCE_PATHS),
            "LAUNCH_SOURCE_INVENTORY")
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_CONFIG_NOSYSTEM="1",
               GIT_CONFIG_GLOBAL="/dev/null", GIT_TERMINAL_PROMPT="0")
    deadline = time.monotonic() + 10
    def git(*args):
        remaining = deadline - time.monotonic()
        require(remaining > 0, "LAUNCH_SOURCE_DEADLINE")
        return subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), *args],
                                       stdin=subprocess.DEVNULL, env=env, timeout=remaining)
    require(git("rev-parse", commit + "^{tree}").decode().strip() == snapshot.get("tree"), "LAUNCH_SOURCE_TREE")
    for row in files:
        raw = git("show", commit + ":" + row["path"])
        require(type(row.get("bytes")) is int and len(raw) == row["bytes"]
                and sha(raw) == row.get("sha256"), "LAUNCH_SOURCE_GIT")


def launch_provenance(repo, root, compiler, harness, nonce):
    target, context, contract_raw, contract, before = launch.load_prepared(root)
    require(context["compilerCommitSha"] == compiler and context["harnessCommitSha"] == harness
            and context["runNonce"] == nonce, "LAUNCH_CONTEXT")
    claim_raw = launch.regular(target / "attempt.claim.original.json")
    result_raw = launch.regular(target / "result.original.json")
    process_raw = launch.regular(target / "process.original.json")
    claim, result, process = map(strict_json, (claim_raw, result_raw, process_raw))
    command = launch.command_for(context, contract_raw, contract)
    common = {
        "command": command, "contextSHA256": sha(launch.regular(target / "prepared.original.json")),
        "contractSHA256": sha(contract_raw), "compilerCommitSha": compiler,
        "harnessCommitSha": harness, "runNonce": nonce,
        "semanticAttemptCount": 1, "automaticRetry": False, "fallbackAllowed": False}
    for row in (claim, result):
        for key, wanted in common.items():
            require(same(row.get(key), wanted), "LAUNCH_" + key)
    require(claim.get("schema") == "cherri-function-envelope-r2-semantic-launch-claim/1"
            and type(claim.get("attemptOrdinal")) is int and claim["attemptOrdinal"] == 1
            and type(claim.get("processTimeoutSeconds")) is int and claim["processTimeoutSeconds"] == 25
            and type(claim.get("wholeLaunchOwnerSeconds")) is int and claim["wholeLaunchOwnerSeconds"] == 35,
            "LAUNCH_CLAIM")
    require(result.get("schema") == "cherri-function-envelope-r2-launch-result/1"
            and result.get("status") == "PASS" and result.get("sourceUnchanged") is True
            and type(result.get("elapsedSeconds")) in (int, float)
            and 0 <= result["elapsedSeconds"] < 35
            and result.get("nativeExecutionAuthenticated") is False
            and result.get("formalAcceptanceGranted") is False, "LAUNCH_RESULT")
    require(process.get("command") == command, "LAUNCH_COMMAND")
    observed, details = process.get("observation"), process.get("details")
    require(isinstance(observed, dict) and observed.get("result") == "PASS"
            and type(observed.get("exitCode")) is int and observed["exitCode"] == 0
            and isinstance(details, dict) and details.get("cleanupParentReaped") is True
            and details.get("groupPresentAfterCleanup") is False, "LAUNCH_PROCESS")
    for name, side in (("stdout.original", "stdout"), ("stderr.original", "stderr")):
        data = launch.regular(target / name, 65537, empty=True)
        require(same(result.get(name), {"bytes": len(data), "sha256": sha(data)})
                and type(details[side]["bytes"]) is int and details[side]["bytes"] == len(data)
                and details[side]["sha256"] == sha(data) and details[side]["eof"] is True
                and details[side]["overflow"] is False, "LAUNCH_STREAM")
    for name in ("process.original.json", "process-started.original.json", "process-finished.original.json"):
        data = launch.regular(target / name)
        require(same(result.get(name), {"bytes": len(data), "sha256": sha(data)}), "LAUNCH_JOURNAL_HASH")
    started = strict_json(launch.regular(target / "process-started.original.json"))
    finished = strict_json(launch.regular(target / "process-finished.original.json"))
    require(type(details.get("pid")) is int and details["pid"] > 0 and started.get("pid") == details["pid"]
            and started.get("startedAt") == details.get("startedAt") and same(finished, details), "LAUNCH_JOURNAL")
    require(utc(details["startedAt"]) <= utc(details["finishedAt"]), "LAUNCH_PROCESS_TIME")
    stdout = launch.regular(target / "stdout.original", 65537, empty=True)
    ids = re.findall(rb"(?m)^org\.cherrilang\.qa\.Receiver: ([1-9][0-9]*)\r?$", stdout)
    require(len(ids) == 1 and type(result.get("receiverPID")) is int
            and result["receiverPID"] == int(ids[0]), "LAUNCH_RECEIVER_PID")
    require(launch.regular(target / "source.after.original.json")
            == launch.regular(target / "source.before.original.json"), "LAUNCH_SOURCE_CHANGED")
    verify_source_snapshot(repo, before, harness)
    require(same(claim.get("sourceAndSignatureBindings"), provenance(repo, root, FIXTURE, compiler, harness)),
            "LAUNCH_ARTIFACTS_CHANGED")
    acquisition_raw = read(root, "runtime/function-envelope-r2-trigger-acquisition.original.json", MAX_CALLBACK)
    acquisition = strict_json(acquisition_raw)
    directory = Path(root) / launch.TRIGGERS
    expected_files = ["001-intent.original.json", "002-completion.original.json",
                      "contract.original.json", "owner.original.json"]
    require(sorted(p.name for p in directory.iterdir()) == expected_files, "TRIGGER_EXACT_ORIGINALS")
    rows = acquisition.get("files")
    require(acquisition.get("schema") == "cherri-function-envelope-r2-trigger-original-acquisition/1"
            and acquisition.get("byteIdentical") is True and acquisition.get("contractSHA256") == sha(contract_raw)
            and isinstance(rows, list) and [r.get("path") for r in rows] == expected_files, "TRIGGER_ACQUISITION")
    for row in rows:
        data = launch.regular(directory / row["path"])
        require(same(row, {"path": row["path"], "bytes": len(data), "sha256": sha(data)}), "TRIGGER_HASH")
    require(launch.regular(directory / "contract.original.json") == contract_raw, "TRIGGER_CONTRACT")
    owner = strict_json(launch.regular(directory / "owner.original.json"))
    require(same(owner, {
        "schema": "cherri-function-envelope-trigger-owner/1", "processID": result["receiverPID"],
        "contractByteCount": len(contract_raw), "contractSHA256": sha(contract_raw),
        "contractOriginalRetained": True}), "TRIGGER_OWNER")
    primary = contract["primaryURL"].encode("utf-8")
    encoded = base64.b64encode(primary).decode("ascii")
    expected = {
        "schema": "cherri-function-envelope-trigger-receipt/1", "mode": launch.MODE,
        "attemptOrdinal": 1, "processID": result["receiverPID"],
        "contractSHA256": sha(contract_raw), "contractByteCount": len(contract_raw),
        "source": "env", "attemptConsumed": True, "requestedURLUTF8Bytes": len(primary),
        "requestedURLSHA256": sha(primary), "requestedURLOriginalRetained": True,
        "contractOriginalRetained": True, "retentionFailure": False,
        "requestedURL": contract["primaryURL"], "requestedURLUTF8Base64": encoded,
        "fixtureId": FIXTURE, "workflowName": WORKFLOW, "runNonce": nonce,
        "primaryURL": contract["primaryURL"], "primaryURLUTF8Base64": encoded,
        "primaryURLUTF8Bytes": len(primary), "primaryURLSHA256": sha(primary)}
    tokens, times = [], []
    for sequence, event in ((1, "intent"), (2, "completion")):
        row = strict_json(launch.regular(directory / ("%03d-%s.original.json" % (sequence, event))))
        wanted = {**expected, "sequence": sequence, "event": event}
        if event == "completion":
            wanted["success"] = True
        require(set(row) == set(wanted) | {"timestampISO8601", "invocationToken"}, "TRIGGER_FIELDS")
        for key, value in wanted.items():
            require(same(row[key], value), "TRIGGER_" + key)
        require(isinstance(row["invocationToken"], str)
                and re.fullmatch(r"[0-9A-F]{8}(?:-[0-9A-F]{4}){3}-[0-9A-F]{12}", row["invocationToken"]),
                "TRIGGER_TOKEN")
        tokens.append(row["invocationToken"])
        times.append(utc(row["timestampISO8601"]))
    require(tokens[0] == tokens[1], "TRIGGER_TOKEN_PAIR")
    require(utc(details["startedAt"]).replace(microsecond=0) <= times[0]
            <= times[1] <= utc(acquisition["collectedUTC"]), "TRIGGER_TIME_ORDER")
    return {
        "compilerCommitSha": compiler, "harnessCommitSha": harness,
        "contractSHA256": sha(contract_raw), "launchResultSHA256": sha(result_raw),
        "launchProcessSHA256": sha(process_raw), "triggerAcquisitionSHA256": sha(acquisition_raw),
        "receiverPID": result["receiverPID"], "runId": context["runId"],
        "runAttempt": context["runAttempt"], "udid": context["udid"],
        "oneNoInputLaunchContractVerified": True, "semanticAttemptCount": 1,
        "inputChangingFallbackAllowed": False, "appleExecutionAuthenticated": False}


def parse_cases(summary):
    # Reuse only the pure, unauthenticated three-CASE grammar. Its predecessor
    # fixture label is retained explicitly as grammar metadata, never rebound as
    # an original R2 runtime record.
    parsed = grammar.validate_summary(summary)
    for flag in ("launchInputAuthenticated", "sourceAuthenticated",
                 "appleExecutionAuthenticated", "formalAcceptanceGranted"):
        require(parsed.get(flag) is False, "GRAMMAR_CANNOT_SUPPLY_AUTHORITY")
    return {
        "status": "ENGINEERING_OBSERVATION_PARSED", "fixtureId": FIXTURE,
        "grammarFixtureId": parsed["fixtureId"], "grammarOnlyReuse": True,
        "caseObservations": parsed["caseObservations"],
        "caseNoInputGoalsMatched": parsed["caseNoInputGoalsMatched"],
        "declaredNoInputGoalsMatched": parsed["declaredNoInputGoalsMatched"],
        "requiredLaunchMode": "no-input-first-and-only-semantic-invocation",
        "launchInputAuthenticated": False, "sourceAuthenticated": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False}


def trace_records(text):
    require(isinstance(text, str) and 0 < len(text.encode("utf-8")) <= 8192, "TRACE_SIZE")
    lines = text.splitlines()
    require(len(lines) == 4 and all(line.strip() for line in lines), "TRACE_FOUR_EVENTS")
    rows = [strict_json(line) for line in lines]
    for index, stage in ((0, "collector_begin"), (3, "collector_end")):
        require(same(rows[index], {
            "schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
            "runId": FIXTURE, "traceTransport": "ndjson-stream",
            "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}), "TRACE_COLLECTOR")
    for index, stage, sequence, action in ((1, "before", 360, "setvariable"),
                                          (2, "after", 370, "runworkflow")):
        row = rows[index]
        require(isinstance(row, dict), "TRACE_RECORD")
        stamp = row.get("timestamp")
        require(isinstance(stamp, str) and 0 < len(stamp.encode("utf-8")) <= 256
                and stamp.strip() and "\ufffc" not in stamp and "\x00" not in stamp, "TRACE_TIMESTAMP")
        require(same(row, {
            "schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
            "label": "function-envelope-presence", "stepSequence": sequence,
            "actionIdentifier": "is.workflow.actions." + action,
            "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
            "timestamp": stamp}), "TRACE_INLINE")
    return rows


def evaluate(repo, root, fixture, compiler, harness, nonce, expected_output):
    result = {
        "schemaVersion": "1.0", "runtimeContractMode": MODE, "diagnosticOnly": True,
        "targetFixtureId": fixture, "workflowName": WORKFLOW, "compilerCommitSha": compiler,
        "harnessCommitSha": harness, "buildProfile": "debug", "traceExpectation": "REQUIRED_NDJSON",
        "sourceSha256": SOURCE_SHA256 if fixture == FIXTURE else "",
        "unsignedPlistSha256": "", "expectedContractSha256": CONTRACT_SHA256,
        "runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": "not observed",
        "inputBindings": {}, "launchBinding": None, "observation": None, "traceRecords": [],
        "observedCorrelatedCallbackStatus": "UNKNOWN", "runtimeObservationVerified": False,
        "declaredNoInputGoalsMatched": False, "oneNoInputLaunchContractVerified": False,
        "semanticContractVerified": False, "importTransportAuthenticated": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False}
    try:
        require(fixture == FIXTURE and expected_output == "", "DIAGNOSTIC_MODE")
        require(isinstance(nonce, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce), "FRESH_NONCE")
        result["inputBindings"] = provenance(repo, root, fixture, compiler, harness)
        result["unsignedPlistSha256"] = result["inputBindings"]["unsignedShortcut"]["sha256"]
        result["launchBinding"] = launch_provenance(repo, root, compiler, harness, nonce)
        result["oneNoInputLaunchContractVerified"] = True
        raw = read(root, "runtime/callback-events.json", MAX_CALLBACK)
        result["inputBindings"]["callbackEvents"] = {
            "path": "runtime/callback-events.json", "bytes": len(raw), "sha256": sha(raw)}
        events = strict_json(raw)
        require(isinstance(events, list) and all(isinstance(e, dict) for e in events), "CALLBACK_ARRAY")
        matched = [e for e in events if e.get("runId") == nonce and e.get("receiverSelfTest") is not True]
        require(len(matched) == 1, "CALLBACK_UNIQUE_NONCE")
        event = matched[0]
        require(event.get("receiverSelfTest") is False and event.get("eventSource") == "shortcuts-x-callback"
                and event.get("fixtureId") == FIXTURE and event.get("expectedShortcutName") == WORKFLOW,
                "CALLBACK_ORIGIN")
        status = event.get("status")
        result["observedCorrelatedCallbackStatus"] = status if status in ("SUCCESS", "ERROR", "CANCELLED") else "UNKNOWN"
        require(status == "SUCCESS", "CALLBACK_SUCCESS")
        result["observation"] = parse_cases(event.get("resultSummary"))
        result["runGate"] = "PASS"
        rows = trace_records(event.get("tracePayload"))
        result.update(traceRecords=rows, traceGate="PASS", runtimeObservationVerified=True,
                      tracePayloadSha256=sha(event["tracePayload"].encode("utf-8")),
                      normalizedTraceSha256=sha(trace_bytes(rows)),
                      declaredNoInputGoalsMatched=result["observation"]["declaredNoInputGoalsMatched"],
                      reason="Exact R2 compiler and graph, signed bytes, first-only no-input launch and original CASE/trace joined; outer owner must independently prove native import and execution.")
    except (OSError, ValueError, TypeError, KeyError, AttributeError, OverflowError, subprocess.SubprocessError) as exc:
        prefixes = ("FUNCTION_R2_", "FUNCTION_LAUNCH_", "FUNCTION_ENVELOPE_", "FUNCTION_PRESENCE_", "NESTED_RUNTIME_")
        result["reason"] = str(exc) if isinstance(exc, ValueError) and str(exc).startswith(prefixes) else "FUNCTION_R2_RUNTIME_INVALID_INPUT"
    return result
