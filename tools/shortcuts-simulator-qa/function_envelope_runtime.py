"""Original joins for the fixed function-envelope engineering observation.

The graph and payload parser remain independent authorities. This joins exact
compiler/signed bytes, the one no-input launch and Receiver originals, callback
and trace. Actual Apple execution and import authority remain with the outer
source-bound QA owner; no RP26/BRG15 or global-179 acceptance is granted here.
"""
import base64
import datetime
import json
from pathlib import Path
import re
import subprocess
import os
import time

import function_envelope_graph as graph
import function_envelope_observation as observation
import function_envelope_launch as launch
from nested_list_runtime import read, trace_bytes

FIXTURE = observation.FIXTURE
WORKFLOW = graph.WORKFLOW
MODE = "FUNCTION_ENVELOPE_ENGINEERING_OBSERVATION_ONLY"
DIAGNOSTIC = "runtime/function-envelope-observation.json"
MAX_CALLBACK = 256 * 1024
COMPILER_SOURCE = "b82bd63dad0b72078d98d57438a61b463b7dcff7"
COMPILER_EXPORT_SHA256 = "dc22aeac90454dd18a9b27eaee0fa5f14ad9b3f763158985309e7ef946a6655d"
UNSIGNED_SHA256 = "54889840e546acc60ffe05c8399e1d4bd54f0dc846ec3fd520b4fffc0aa6c7d0"


def require(ok, code):
    if not ok:
        raise ValueError("FUNCTION_RUNTIME_" + code)


def sha(raw):
    return launch.sha(raw)


def same(left, right):
    return launch.canonical(left) == launch.canonical(right)


def strict_json(raw):
    return launch.strict_json(raw)


def prepared(repo, fixture, source, unsigned):
    if fixture != FIXTURE:
        return None
    contract = read(repo, "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json", 8192)
    proof = graph.validate_compiled(source, contract, unsigned)
    require(proof["actionCount"] == 379, "ACTION_COUNT")
    return {"fixtureId": FIXTURE, "workflowName": WORKFLOW,
            "sourceSha256": graph.SOURCE_SHA256,
            "expectedContractSha256": graph.CONTRACT_SHA256,
            "unsignedPlistSha256": sha(unsigned), "graph": proof,
            "requiredLaunchMode": "primary-no-input",
            "firstAndOnlySemanticInvocation": True, "inputChangingFallbackAllowed": False,
            "diagnosticOnly": True, "formalAcceptanceGranted": False}


def provenance(repo, root, fixture, commit, harness_commit):
    require(fixture == FIXTURE and isinstance(commit, str)
            and commit == COMPILER_SOURCE
            and isinstance(harness_commit, str) and re.fullmatch(r"[0-9a-f]{40}", harness_commit),
            "SOURCE_IDENTITY")
    paths = {"compilerManifest": "compiler-manifest.json",
             "compiledSource": "builds/" + WORKFLOW + ".cherri",
             "unsignedShortcut": "builds/" + WORKFLOW + ".shortcut",
             "signedShortcut": "builds/signed/" + WORKFLOW + ".shortcut",
             "signingProvenance": "runtime/signing-provenance.json",
             "compilerExportOriginal": "builds/function-envelope-compiler-export.original.json"}
    raw = {key: read(root, path, 10 * 1024 * 1024 if key == "signedShortcut" else 2 * 1024 * 1024)
           for key, path in paths.items()}
    manifest = strict_json(raw["compilerManifest"])
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == commit
            and manifest.get("harnessCommitSha") == harness_commit
            and manifest.get("compilerArtifactOrigin") == "retained-original-preparation",
            "MANIFEST_IDENTITY")
    require(sha(raw["compilerExportOriginal"]) == COMPILER_EXPORT_SHA256
            and sha(raw["unsignedShortcut"]) == UNSIGNED_SHA256, "RETAINED_COMPILER_ORIGINAL")
    compiler_export = strict_json(raw["compilerExportOriginal"])
    require(compiler_export["sourceCommit"] == commit
            and compiler_export["actualTest"] == "TestP15FunctionEnvelopePresenceProbePreparation"
            and compiler_export["nativeExecution"] is False
            and compiler_export["unsigned"]["sha256"] == sha(raw["unsignedShortcut"])
            and compiler_export["unsigned"]["bytes"] == len(raw["unsignedShortcut"]),
            "COMPILER_EXPORT_IDENTITY")
    rows = manifest.get("fixtures")
    require(isinstance(rows, list) and all(isinstance(row, dict) for row in rows), "MANIFEST_ROWS")
    ids = [row.get("fixtureId") for row in rows]
    require(all(isinstance(fid, str) for fid in ids) and len(ids) == len(set(ids)), "MANIFEST_DUPLICATES")
    selected = [row for row in rows if row.get("fixtureId") == fixture]
    require(len(selected) == 1, "MANIFEST_FIXTURE")
    proof = prepared(repo, fixture, raw["compiledSource"], raw["unsignedShortcut"])
    expected = {"fileName": WORKFLOW + ".cherri", "workflowName": WORKFLOW,
                "dialect": "v2", "buildProfile": "debug", "runtimeTraceExpectation": "REQUIRED_NDJSON",
                "sourceSha256": graph.SOURCE_SHA256, "unsignedPlistSha256": sha(raw["unsignedShortcut"]),
                "nativeActionCount": 379, "functionEnvelopeProbe": proof}
    for key, value in expected.items():
        require(same(selected[0].get(key), value), "PREPARED_" + key)
    signing = strict_json(raw["signingProvenance"])
    signed = raw["signedShortcut"]
    require(signed.startswith(b"AEA1") and len(signed) >= 16, "SIGNED_BYTES")
    require(isinstance(signing, dict) and signing.get("compilerCommitSha") == commit
            and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == sha(signed)
            and type(signing.get("signedByteSize")) is int
            and signing["signedByteSize"] == len(signed), "SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(isinstance(target, dict), "EXACT_SIGNING_PROVENANCE")
    for key, value in {
        "compilerCommitSha": commit, "sourceSha256": graph.SOURCE_SHA256,
        "unsignedPlistSha256": sha(raw["unsignedShortcut"]), "unsignedByteSize": len(raw["unsignedShortcut"]),
        "nativeActionCount": 379, "signedArtifactSha256": sha(signed),
        "signedByteSize": len(signed), "headerMagic": "AEA1",
    }.items():
        require(same(target.get(key), value), "SIGNED_" + key)
    return {key: {"path": paths[key], "bytes": len(data), "sha256": sha(data)}
            for key, data in raw.items()}


def utc(value):
    require(isinstance(value, str) and len(value) <= 64, "TIME")
    parsed = datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    require(parsed.tzinfo is not None and parsed.utcoffset() == datetime.timedelta(), "TIME_ZONE")
    return parsed


def verify_source_snapshot(repo, snapshot, commit):
    require(isinstance(snapshot, dict) and snapshot.get("commit") == commit
            and snapshot.get("sourceClean") is True, "LAUNCH_SOURCE")
    files = snapshot.get("files")
    require(isinstance(files, list) and [v.get("path") for v in files] == list(launch.SOURCE_PATHS),
            "LAUNCH_SOURCE_INVENTORY")
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_NO_REPLACE_OBJECTS="1", GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL="/dev/null",
               GIT_TERMINAL_PROMPT="0")
    deadline = time.monotonic() + 10
    def git(*args):
        remaining = deadline - time.monotonic()
        require(remaining > 0, "LAUNCH_SOURCE_DEADLINE")
        return subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), *args],
                                       stdin=subprocess.DEVNULL, env=env, timeout=remaining)
    require(git("rev-parse", commit + "^{tree}").decode().strip() == snapshot.get("tree"),
            "LAUNCH_SOURCE_TREE")
    for row in files:
        raw = git("show", commit + ":" + row["path"])
        require(type(row.get("bytes")) is int and len(raw) == row["bytes"]
                and sha(raw) == row.get("sha256"), "LAUNCH_SOURCE_GIT")


def launch_provenance(repo, root, commit, harness_commit, nonce):
    target, context, contract_raw, contract, before = launch.load_prepared(root)
    require(context["compilerCommitSha"] == commit and context["harnessCommitSha"] == harness_commit and context["runNonce"] == nonce, "LAUNCH_CONTEXT")
    claim_raw = launch.regular(target / "attempt.claim.original.json")
    result_raw = launch.regular(target / "result.original.json")
    process_raw = launch.regular(target / "process.original.json")
    claim, result, process = map(strict_json, (claim_raw, result_raw, process_raw))
    command = launch.command_for(context, contract_raw, contract)
    require(claim.get("schema") == "cherri-function-envelope-semantic-launch-claim/1"
            and claim.get("command") == command
            and claim.get("contextSHA256") == sha(launch.regular(target / "prepared.original.json"))
            and claim.get("contractSHA256") == sha(contract_raw)
            and claim.get("compilerCommitSha") == commit and claim.get("harnessCommitSha") == harness_commit and claim.get("runNonce") == nonce,
            "LAUNCH_CLAIM")
    for value in (claim, result):
        require(type(value.get("semanticAttemptCount")) is int and value["semanticAttemptCount"] == 1
                and value.get("automaticRetry") is False and value.get("fallbackAllowed") is False,
                "LAUNCH_ONE_ATTEMPT")
    require(type(claim.get("attemptOrdinal")) is int and claim["attemptOrdinal"] == 1
            and claim.get("processTimeoutSeconds") == 25
            and claim.get("wholeLaunchOwnerSeconds") == 35, "LAUNCH_BOUNDS")
    require(result.get("schema") == "cherri-function-envelope-launch-result/1"
            and result.get("status") == "PASS" and result.get("sourceUnchanged") is True
            and result.get("contextSHA256") == claim["contextSHA256"]
            and result.get("contractSHA256") == sha(contract_raw)
            and result.get("compilerCommitSha") == commit and result.get("harnessCommitSha") == harness_commit and result.get("runNonce") == nonce
            and result.get("command") == command
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
    for name, key in (("stdout.original", "stdout"), ("stderr.original", "stderr")):
        raw = launch.regular(target / name, 65537, empty=True)
        require(same(result.get(name), {"bytes": len(raw), "sha256": sha(raw)})
                and type(details[key]["bytes"]) is int
                and details[key]["bytes"] == len(raw) and details[key]["sha256"] == sha(raw)
                and details[key]["eof"] is True and details[key]["overflow"] is False, "LAUNCH_STREAM")
    require(same(result.get("process.original.json"), {"bytes": len(process_raw), "sha256": sha(process_raw)}),
            "LAUNCH_PROCESS_HASH")
    for name in ("process-started.original.json", "process-finished.original.json"):
        original = launch.regular(target / name)
        require(same(result.get(name), {"bytes": len(original), "sha256": sha(original)}),
                "LAUNCH_JOURNAL_HASH")
    started = strict_json(launch.regular(target / "process-started.original.json"))
    finished = strict_json(launch.regular(target / "process-finished.original.json"))
    require(type(details.get("pid")) is int and details["pid"] > 0
            and started.get("pid") == details["pid"]
            and started.get("startedAt") == details.get("startedAt")
            and same(finished, details), "LAUNCH_JOURNAL")
    require(utc(details["startedAt"]) <= utc(details["finishedAt"]), "LAUNCH_PROCESS_TIME")
    out = launch.regular(target / "stdout.original", 65537, empty=True)
    ids = re.findall(rb"(?m)^org\.cherrilang\.qa\.Receiver: ([1-9][0-9]*)\r?$", out)
    require(len(ids) == 1 and type(result.get("receiverPID")) is int
            and result["receiverPID"] == int(ids[0]), "LAUNCH_RECEIVER_PID")
    after_raw = launch.regular(target / "source.after.original.json")
    require(after_raw == launch.regular(target / "source.before.original.json"), "LAUNCH_SOURCE_CHANGED")
    verify_source_snapshot(repo, before, harness_commit)
    require(same(claim.get("sourceAndSignatureBindings"), provenance(repo, root, FIXTURE, commit, harness_commit)),
            "LAUNCH_ARTIFACTS_CHANGED")

    acquisition_raw = read(root, "runtime/function-envelope-trigger-acquisition.original.json", MAX_CALLBACK)
    acquisition = strict_json(acquisition_raw)
    directory = Path(root) / launch.TRIGGERS
    files = sorted(p.name for p in directory.iterdir())
    expected_files = ["001-intent.original.json", "002-completion.original.json",
                      "contract.original.json", "owner.original.json"]
    require(files == expected_files, "TRIGGER_EXACT_ORIGINALS")
    rows = acquisition.get("files")
    require(acquisition.get("schema") == "cherri-function-envelope-trigger-original-acquisition/1"
            and acquisition.get("byteIdentical") is True
            and acquisition.get("contractSHA256") == sha(contract_raw)
            and isinstance(rows, list) and [v.get("path") for v in rows] == expected_files,
            "TRIGGER_ACQUISITION")
    for row in rows:
        raw = launch.regular(directory / row["path"])
        require(same(row, {"path": row["path"], "bytes": len(raw), "sha256": sha(raw)}), "TRIGGER_HASH")
    require(launch.regular(directory / "contract.original.json") == contract_raw, "TRIGGER_CONTRACT")
    owner = strict_json(launch.regular(directory / "owner.original.json"))
    require(same(owner, {"schema": "cherri-function-envelope-trigger-owner/1",
                      "processID": result["receiverPID"], "contractByteCount": len(contract_raw),
                      "contractSHA256": sha(contract_raw), "contractOriginalRetained": True}), "TRIGGER_OWNER")
    primary = contract["primaryURL"].encode("utf-8")
    primary64 = base64.b64encode(primary).decode("ascii")
    tokens = []
    timestamps = []
    base_keys = {
        "schema", "mode", "sequence", "event", "attemptOrdinal", "processID", "timestampISO8601",
        "contractSHA256", "contractByteCount", "source", "attemptConsumed",
        "requestedURLUTF8Bytes", "requestedURLSHA256", "requestedURLOriginalRetained",
        "contractOriginalRetained", "retentionFailure", "requestedURL", "requestedURLUTF8Base64",
        "fixtureId", "workflowName", "runNonce", "primaryURL", "primaryURLUTF8Base64",
        "primaryURLUTF8Bytes", "primaryURLSHA256", "invocationToken"}
    for sequence, event in ((1, "intent"), (2, "completion")):
        row = strict_json(launch.regular(directory / ("%03d-%s.original.json" % (sequence, event))))
        require(set(row) == base_keys | ({"success"} if event == "completion" else set()), "TRIGGER_FIELDS")
        expected = {
            "schema": "cherri-function-envelope-trigger-receipt/1", "mode": launch.MODE,
            "sequence": sequence, "event": event, "attemptOrdinal": 1, "processID": result["receiverPID"],
            "contractSHA256": sha(contract_raw), "contractByteCount": len(contract_raw),
            "source": "env", "attemptConsumed": True, "requestedURLUTF8Bytes": len(primary),
            "requestedURLSHA256": sha(primary), "requestedURLOriginalRetained": True,
            "contractOriginalRetained": True, "retentionFailure": False,
            "requestedURL": contract["primaryURL"], "requestedURLUTF8Base64": primary64,
            "fixtureId": FIXTURE, "workflowName": WORKFLOW, "runNonce": nonce,
            "primaryURL": contract["primaryURL"], "primaryURLUTF8Base64": primary64,
            "primaryURLUTF8Bytes": len(primary), "primaryURLSHA256": sha(primary)}
        if event == "completion":
            expected["success"] = True
        for key, value in expected.items():
            require(same(row[key], value), "TRIGGER_" + key)
        require(isinstance(row["invocationToken"], str)
                and re.fullmatch(r"[0-9A-F]{8}(?:-[0-9A-F]{4}){3}-[0-9A-F]{12}", row["invocationToken"]),
                "TRIGGER_TOKEN")
        tokens.append(row["invocationToken"])
        timestamps.append(utc(row["timestampISO8601"]))
    require(tokens[0] == tokens[1], "TRIGGER_TOKEN_PAIR")
    require(utc(details["startedAt"]).replace(microsecond=0) <= timestamps[0]
            <= timestamps[1] <= utc(acquisition["collectedUTC"]), "TRIGGER_TIME_ORDER")
    return {"compilerCommitSha": commit, "harnessCommitSha": harness_commit,
            "contractSHA256": sha(contract_raw), "launchResultSHA256": sha(result_raw),
            "launchProcessSHA256": sha(process_raw), "triggerAcquisitionSHA256": sha(acquisition_raw),
            "receiverPID": result["receiverPID"], "runId": context["runId"],
            "runAttempt": context["runAttempt"], "udid": context["udid"],
            "oneNoInputLaunchContractVerified": True, "semanticAttemptCount": 1,
            "inputChangingFallbackAllowed": False, "appleExecutionAuthenticated": False}


def trace_records(text):
    require(isinstance(text, str) and 0 < len(text.encode("utf-8")) <= 8192, "TRACE_SIZE")
    lines = text.splitlines()
    require(len(lines) == 4 and all(line.strip() for line in lines), "TRACE_FOUR_EVENTS")
    records = [strict_json(line) for line in lines]
    for index, stage in ((0, "collector_begin"), (3, "collector_end")):
        expected = {"schemaVersion": "1.4", "kind": "COLLECTOR_EVENT", "stage": stage,
                    "runId": FIXTURE, "traceTransport": "ndjson-stream",
                    "collectorSentinel": "===CHERRI_QA_TRACE_NDJSON_V14==="}
        require(same(records[index], expected), "TRACE_COLLECTOR")
    for index, stage, sequence, action in ((1, "before", 360, "setvariable"),
                                          (2, "after", 370, "runworkflow")):
        row = records[index]
        require(isinstance(row, dict), "TRACE_RECORD")
        stamp = row.get("timestamp")
        require(isinstance(stamp, str) and 0 < len(stamp.encode("utf-8")) <= 256
                and stamp.strip() and "\ufffc" not in stamp and "\x00" not in stamp, "TRACE_TIMESTAMP")
        expected = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
                    "label": "function-envelope-presence", "stepSequence": sequence,
                    "actionIdentifier": "is.workflow.actions." + action,
                    "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                    "timestamp": stamp}
        require(same(row, expected), "TRACE_INLINE")
    return records


def evaluate(repo, root, fixture, commit, harness_commit, nonce, expected_output):
    result = {
        "schemaVersion": "1.0", "runtimeContractMode": MODE, "diagnosticOnly": True,
        "targetFixtureId": fixture, "workflowName": WORKFLOW, "compilerCommitSha": commit,
        "harnessCommitSha": harness_commit,
        "buildProfile": "debug", "traceExpectation": "REQUIRED_NDJSON", "releaseBuildVerified": False,
        "sourceSha256": graph.SOURCE_SHA256 if fixture == FIXTURE else "", "unsignedPlistSha256": "",
        "expectedContractSha256": graph.CONTRACT_SHA256,
        "runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": "not observed",
        "inputBindings": {}, "launchBinding": None, "observation": None, "traceRecords": [],
        "observedCorrelatedCallbackStatus": "UNKNOWN", "runtimeObservationVerified": False,
        "declaredNoInputGoalsMatched": False, "oneNoInputLaunchContractVerified": False,
        "semanticContractVerified": False, "importTransportAuthenticated": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
    }
    try:
        require(fixture == FIXTURE and expected_output == "", "DIAGNOSTIC_MODE")
        require(isinstance(nonce, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,128}", nonce), "FRESH_NONCE")
        result["inputBindings"] = provenance(repo, root, fixture, commit, harness_commit)
        result["unsignedPlistSha256"] = result["inputBindings"]["unsignedShortcut"]["sha256"]
        result["launchBinding"] = launch_provenance(repo, root, commit, harness_commit, nonce)
        result["oneNoInputLaunchContractVerified"] = True
        raw = read(root, "runtime/callback-events.json", MAX_CALLBACK)
        result["inputBindings"]["callbackEvents"] = {
            "path": "runtime/callback-events.json", "bytes": len(raw), "sha256": sha(raw)}
        events = strict_json(raw)
        require(isinstance(events, list) and all(isinstance(event, dict) for event in events), "CALLBACK_ARRAY")
        matched = [event for event in events if event.get("runId") == nonce
                   and event.get("receiverSelfTest") is not True]
        require(len(matched) == 1, "CALLBACK_UNIQUE_NONCE")
        event = matched[0]
        require(event.get("receiverSelfTest") is False and event.get("eventSource") == "shortcuts-x-callback"
                and event.get("fixtureId") == fixture and event.get("expectedShortcutName") == WORKFLOW,
                "CALLBACK_ORIGIN")
        status = event.get("status")
        result["observedCorrelatedCallbackStatus"] = status if status in ("SUCCESS", "ERROR", "CANCELLED") else "UNKNOWN"
        require(status == "SUCCESS", "CALLBACK_SUCCESS")
        result["observation"] = observation.validate_summary(event.get("resultSummary"))
        result["runGate"] = "PASS"
        records = trace_records(event.get("tracePayload"))
        result["traceRecords"] = records
        result["tracePayloadSha256"] = sha(event["tracePayload"].encode("utf-8"))
        result["normalizedTraceSha256"] = sha(trace_bytes(records))
        result["traceGate"] = "PASS"
        result["runtimeObservationVerified"] = True
        result["declaredNoInputGoalsMatched"] = result["observation"]["declaredNoInputGoalsMatched"]
        result["reason"] = "source, signed bytes, one no-input receiver invocation and original CASE/trace joined; actual Apple import and execution require independent outer authority"
    except (OSError, ValueError, TypeError, KeyError, AttributeError, OverflowError, subprocess.SubprocessError) as exc:
        prefixes = ("FUNCTION_RUNTIME_", "FUNCTION_LAUNCH_", "FUNCTION_ENVELOPE_", "FUNCTION_PRESENCE_GRAPH_", "NESTED_RUNTIME_")
        result["reason"] = str(exc) if isinstance(exc, ValueError) and str(exc).startswith(prefixes) else "FUNCTION_RUNTIME_INVALID_INPUT"
    return result


def emit(repo, cb_path, nonce, fixture, expected_output, trace_path, manifest_path, commit, harness_commit):
    root = Path(manifest_path).resolve().parent
    require(Path(cb_path).resolve() == root / "runtime/callback-events.json"
            and Path(trace_path).resolve() == root / "runtime/runtime-trace.jsonl", "OUTPUT_PATH")
    value = evaluate(repo, root, fixture, commit, harness_commit, nonce, expected_output)
    if value["traceGate"] == "PASS":
        path, data = Path(trace_path), trace_bytes(value["traceRecords"])
        if path.exists():
            require(launch.regular(path) == data, "TRACE_ALREADY_DIFFERENT")
        else:
            launch.write_new(path, data)
    print(json.dumps(value, ensure_ascii=False, sort_keys=True))


def observation_document(value, agreement):
    require(type(agreement) is bool, "ORACLE_AGREEMENT_TYPE")
    verified = value["runtimeObservationVerified"] and agreement
    return {**value, "callbackOracleAgreement": agreement,
            "runtimeObservationVerified": verified,
            "declaredNoInputGoalsMatched": value["declaredNoInputGoalsMatched"] and verified}


def apply_assertions(assertions, oracle, repo, root, commit, harness_commit, expected_output):
    fresh = evaluate(repo, root, assertions["targetFixtureId"], commit, harness_commit,
                     assertions["expectedRunId"], expected_output)
    agreement = (same(oracle, fresh)
                 and assertions.get("shortcutRunCompletedGate") == fresh["runGate"]
                 and assertions.get("traceCapturedGate") == fresh["traceGate"])
    document = observation_document(fresh, agreement)
    verified = document["runtimeObservationVerified"]
    if verified:
        require(read(root, "runtime/runtime-trace.jsonl", 16384) == trace_bytes(fresh["traceRecords"]),
                "TRACE_EXPORT_DIFFERENT")
    assertions.update(
        compilerCommitSha=commit, harnessCommitSha=harness_commit, compiledSourceSha256=fresh["sourceSha256"],
        unsignedPlistSha256=fresh["unsignedPlistSha256"], buildProfile="debug",
        traceExpectation="REQUIRED_NDJSON", releaseBuildVerified=False,
        runtimeContractMode=MODE, diagnosticOnly=True, semanticContractVerified=False,
        importTransportAuthenticated=False, appleExecutionAuthenticated=False, formalAcceptanceGranted=False,
        expectedSemanticOutput="", expectedContractSha256=graph.CONTRACT_SHA256,
        declaredNoInputGoalsMatched=document["declaredNoInputGoalsMatched"],
        oneNoInputLaunchContractVerified=fresh["oneNoInputLaunchContractVerified"],
        runtimeContractVerified=verified, runtimeObservationVerified=verified,
        compiledShortcutCallbackVerified=verified, compiledShortcutRuntimeExecuted=verified,
        traceObserved="NDJSON_VERIFIED" if verified else "UNVERIFIED",
        observedCorrelatedCallbackStatus=fresh["observedCorrelatedCallbackStatus"],
        callbackOracleAgreement=agreement,
        diagnosticReason=document["reason"] if agreement else "FUNCTION_RUNTIME_ORACLE_REPORT_MISMATCH",
        notes="Engineering observation only. Missing caller key and contrary fields remain actual outcomes; the no-input hypotheses are not acceptance conditions. Apple import/execution authority is separately required.")
    launch.record(Path(root) / DIAGNOSTIC, document)
    return document


def validate_public_diagnostic(value, root):
    repo = Path(__file__).resolve().parents[2]
    assertions = strict_json(read(root, "runtime/output-assertions.json", MAX_CALLBACK))
    require(isinstance(assertions, dict), "PUBLIC_ASSERTIONS")
    fresh = evaluate(repo, root, assertions.get("targetFixtureId"), assertions.get("compilerCommitSha"),
                     assertions.get("harnessCommitSha"), assertions.get("expectedRunId"), assertions.get("expectedSemanticOutput"))
    expected = observation_document(fresh, assertions.get("callbackOracleAgreement"))
    require(same(value, expected), "PUBLIC_RECOMPUTATION")
    for key, wanted in {
        "runtimeContractMode": MODE, "diagnosticOnly": True, "semanticContractVerified": False,
        "compilerCommitSha": fresh["compilerCommitSha"], "harnessCommitSha": fresh["harnessCommitSha"],
        "importTransportAuthenticated": False, "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
        "runtimeContractVerified": expected["runtimeObservationVerified"],
        "runtimeObservationVerified": expected["runtimeObservationVerified"],
        "compiledShortcutCallbackVerified": expected["runtimeObservationVerified"],
        "compiledShortcutRuntimeExecuted": expected["runtimeObservationVerified"],
        "declaredNoInputGoalsMatched": expected["declaredNoInputGoalsMatched"],
        "oneNoInputLaunchContractVerified": expected["oneNoInputLaunchContractVerified"],
        "expectedSemanticOutput": "", "expectedContractSha256": graph.CONTRACT_SHA256,
    }.items():
        require(same(assertions.get(key), wanted), "PUBLIC_ASSERTION_" + key)
    if expected["runtimeObservationVerified"]:
        require(assertions.get("shortcutRunCompletedGate") == "PASS"
                and assertions.get("traceCapturedGate") == "PASS", "PUBLIC_GATES")
        require(read(root, "runtime/runtime-trace.jsonl", 16384) == trace_bytes(fresh["traceRecords"]),
                "PUBLIC_TRACE")
    return value
