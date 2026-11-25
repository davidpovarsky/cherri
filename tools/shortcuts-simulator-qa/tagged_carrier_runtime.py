"""Source-bound acquisition for the fixed tagged-carrier engineering fixture.

The payload parser and full graph guard remain separate original authorities.
This module joins their inputs, signatures and exact callback/trace bytes. The
outer owner must authenticate the actual Apple run and transport separately;
synthetic controls or AEA1 magic alone never grant that authority.
"""
import hashlib
import json
from pathlib import Path
import re
from xml.parsers.expat import ExpatError

import tagged_carrier_graph as graph
import tagged_carrier_observation as observation
from nested_list_runtime import read, canonical, trace_bytes

FIXTURE = observation.FIXTURE
MODE = "TAGGED_CARRIER_ENGINEERING_OBSERVATION_ONLY"
DIAGNOSTIC = "runtime/tagged-carrier-observation.json"
ENROLLMENT = "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".enrollment.json"
ENROLLMENT_SHA256 = "8597fbb0d690fb1a894cfdd8d492e61f293722bca83d5d56f1a50e1ae6c296a9"
MAX_CALLBACK = 256 * 1024
MAX_PLIST = 2 * 1024 * 1024


def require(ok, code):
    if not ok:
        raise ValueError("TAGGED_RUNTIME_" + code)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def same(left, right):
    return canonical(left) == canonical(right)


def strict_json(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, "DUPLICATE_JSON_KEY")
            result[key] = value
        return result
    def invalid(_):
        raise ValueError("TAGGED_RUNTIME_NONFINITE_JSON")
    return json.loads(raw, object_pairs_hook=unique, parse_constant=invalid)


def enrollment(repo):
    raw = read(repo, ENROLLMENT, 16 * 1024)
    require(sha(raw) == ENROLLMENT_SHA256, "ENROLLMENT_SOURCE")
    return strict_json(raw)


def prepared(repo, fixture, source, unsigned):
    if fixture != FIXTURE:
        return None
    enrolled = enrollment(repo)
    contract = read(repo, "tools/shortcuts-simulator-qa/fixtures/" + FIXTURE + ".expected.json", 8192)
    try:
        proof = graph.validate_compiled(source, contract, unsigned)
    except ExpatError as exc:
        raise ValueError("TAGGED_RUNTIME_UNSIGNED_PLIST") from exc
    require(proof["actionCount"] == enrolled["nativeActionCount"], "ACTION_COUNT")
    return {
        "fixtureId": FIXTURE, "sourceSha256": graph.SOURCE_SHA256,
        "expectedContractSha256": graph.CONTRACT_SHA256,
        "enrollmentSha256": ENROLLMENT_SHA256, "unsignedPlistSha256": sha(unsigned),
        "graph": proof, "diagnosticOnly": True,
        "nativeCarrierPreservationVerified": False, "formalAcceptanceGranted": False,
    }


def provenance(repo, root, fixture, commit):
    require(fixture == FIXTURE and isinstance(commit, str)
            and re.fullmatch(r"[0-9a-f]{40}", commit), "SOURCE_IDENTITY")
    paths = {"compilerManifest": "compiler-manifest.json",
             "compiledSource": "builds/" + fixture + ".cherri",
             "unsignedShortcut": "builds/" + fixture + ".shortcut",
             "signedShortcut": "builds/signed/" + fixture + ".shortcut",
             "signingProvenance": "runtime/signing-provenance.json"}
    raw = {key: read(root, path, 10 * 1024 * 1024 if key == "signedShortcut" else MAX_PLIST)
           for key, path in paths.items()}
    manifest = strict_json(raw["compilerManifest"])
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == commit, "MANIFEST_IDENTITY")
    rows = manifest.get("fixtures")
    require(isinstance(rows, list) and all(isinstance(row, dict) for row in rows), "MANIFEST_ROWS")
    ids = [row.get("fixtureId") for row in rows]
    require(all(isinstance(fid, str) for fid in ids) and len(ids) == len(set(ids)), "MANIFEST_DUPLICATES")
    selected = [row for row in rows if row.get("fixtureId") == fixture]
    require(len(selected) == 1, "MANIFEST_FIXTURE")
    proof = prepared(repo, fixture, raw["compiledSource"], raw["unsignedShortcut"])
    expected = {"fileName": fixture + ".cherri", "dialect": "v2", "buildProfile": "debug",
                "runtimeTraceExpectation": "REQUIRED_NDJSON",
                "sourceSha256": graph.SOURCE_SHA256,
                "unsignedPlistSha256": proof["unsignedPlistSha256"],
                "nativeActionCount": 225, "taggedCarrierProbe": proof}
    for key, value in expected.items():
        require(same(selected[0].get(key), value), "PREPARED_" + key)
    signing = strict_json(raw["signingProvenance"])
    signed = raw["signedShortcut"]
    require(signed.startswith(b"AEA1") and len(signed) >= 16, "SIGNED_BYTES")
    require(isinstance(signing, dict)
            and signing.get("compilerCommitSha") == commit
            and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == sha(signed)
            and type(signing.get("signedByteSize")) is int
            and signing["signedByteSize"] == len(signed), "SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(isinstance(target, dict), "EXACT_SIGNING_PROVENANCE")
    expected_signing = {
        "compilerCommitSha": commit, "sourceSha256": graph.SOURCE_SHA256,
        "unsignedPlistSha256": sha(raw["unsignedShortcut"]),
        "unsignedByteSize": len(raw["unsignedShortcut"]), "nativeActionCount": 225,
        "signedArtifactSha256": sha(signed), "signedByteSize": len(signed), "headerMagic": "AEA1",
    }
    for key, value in expected_signing.items():
        require(same(target.get(key), value), "SIGNED_" + key)
    return {key: {"path": paths[key], "bytes": len(data), "sha256": sha(data)}
            for key, data in raw.items()}


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
    for index, stage, sequence, action in ((1, "before", 3, "setvariable"),
                                          (2, "after", 218, "comment")):
        record = records[index]
        require(isinstance(record, dict), "TRACE_RECORD")
        stamp = record.get("timestamp")
        require(isinstance(stamp, str) and 0 < len(stamp.encode("utf-8")) <= 256
                and stamp.strip() and "\ufffc" not in stamp and "\x00" not in stamp, "TRACE_TIMESTAMP")
        expected = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
                    "label": "tagged-value-carrier", "stepSequence": sequence,
                    "actionIdentifier": "is.workflow.actions." + action,
                    "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                    "timestamp": stamp}
        require(same(record, expected), "TRACE_INLINE")
    return records


def evaluate(repo, root, fixture, commit, nonce, expected_output):
    result = {
        "schemaVersion": "1.0", "runtimeContractMode": MODE, "diagnosticOnly": True,
        "targetFixtureId": fixture, "compilerCommitSha": commit,
        "buildProfile": "debug", "traceExpectation": "REQUIRED_NDJSON", "releaseBuildVerified": False,
        "sourceSha256": graph.SOURCE_SHA256 if fixture == FIXTURE else "", "unsignedPlistSha256": "",
        "expectedContractSha256": graph.CONTRACT_SHA256, "enrollmentSha256": ENROLLMENT_SHA256,
        "runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": "not observed",
        "inputBindings": {}, "observation": None, "traceRecords": [],
        "observedCorrelatedCallbackStatus": "UNKNOWN", "runtimeObservationVerified": False,
        "nativeCarrierGoalsMatched": False, "semanticContractVerified": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
    }
    try:
        require(fixture == FIXTURE and expected_output == "", "DIAGNOSTIC_MODE")
        require(isinstance(nonce, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", nonce), "FRESH_NONCE")
        result["inputBindings"] = provenance(repo, root, fixture, commit)
        result["unsignedPlistSha256"] = result["inputBindings"]["unsignedShortcut"]["sha256"]
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
                and event.get("fixtureId") == fixture and event.get("expectedShortcutName") == fixture,
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
        result["nativeCarrierGoalsMatched"] = result["observation"]["semanticGoalsMatched"]
        result["reason"] = "source-bound public observation and exact trace; actual Apple transport remains independently authenticated"
    except (OSError, ValueError, TypeError, KeyError, AttributeError, OverflowError) as exc:
        allowed = ("TAGGED_RUNTIME_", "TAGGED_CARRIER_", "NESTED_RUNTIME_")
        result["reason"] = str(exc) if isinstance(exc, ValueError) and str(exc).startswith(allowed) else "TAGGED_RUNTIME_INVALID_INPUT"
    return result


def emit(repo, cb_path, nonce, fixture, expected_output, trace_path, manifest_path, commit):
    root = Path(manifest_path).resolve().parent
    require(Path(cb_path).resolve() == root / "runtime/callback-events.json"
            and Path(trace_path).resolve() == root / "runtime/runtime-trace.jsonl", "OUTPUT_PATH")
    value = evaluate(repo, root, fixture, commit, nonce, expected_output)
    if value["traceGate"] == "PASS":
        data = trace_bytes(value["traceRecords"])
        path = Path(trace_path)
        if path.exists():
            require(path.read_bytes() == data, "TRACE_ALREADY_DIFFERENT")
        else:
            path.write_bytes(data)
    print(json.dumps(value, ensure_ascii=False, sort_keys=True))


def observation_document(value, agreement):
    require(type(agreement) is bool, "ORACLE_AGREEMENT_TYPE")
    verified = value["runtimeObservationVerified"] and agreement
    return {**value, "callbackOracleAgreement": agreement,
            "runtimeObservationVerified": verified,
            "nativeCarrierGoalsMatched": value["nativeCarrierGoalsMatched"] and verified}


def apply_assertions(assertions, oracle, repo, root, commit, expected_output):
    fixture = assertions["targetFixtureId"]
    fresh = evaluate(repo, root, fixture, commit, assertions["expectedRunId"], expected_output)
    agreement = (same(oracle, fresh)
                 and assertions.get("shortcutRunCompletedGate") == fresh["runGate"]
                 and assertions.get("traceCapturedGate") == fresh["traceGate"])
    document = observation_document(fresh, agreement)
    verified = document["runtimeObservationVerified"]
    if verified:
        require(read(root, "runtime/runtime-trace.jsonl", 16384) == trace_bytes(fresh["traceRecords"]),
                "TRACE_EXPORT_DIFFERENT")
    assertions.update(
        compilerCommitSha=commit, compiledSourceSha256=fresh["sourceSha256"],
        unsignedPlistSha256=fresh["unsignedPlistSha256"], buildProfile="debug",
        traceExpectation="REQUIRED_NDJSON", releaseBuildVerified=False,
        runtimeContractMode=MODE, diagnosticOnly=True, semanticContractVerified=False,
        appleExecutionAuthenticated=False, formalAcceptanceGranted=False,
        expectedSemanticOutput="", expectedContractSha256=graph.CONTRACT_SHA256,
        enrollmentSha256=ENROLLMENT_SHA256, nativeCarrierGoalsMatched=document["nativeCarrierGoalsMatched"],
        runtimeContractVerified=verified, runtimeObservationVerified=verified,
        compiledShortcutCallbackVerified=verified, compiledShortcutRuntimeExecuted=verified,
        traceObserved="NDJSON_VERIFIED" if verified else "UNVERIFIED",
        observedCorrelatedCallbackStatus=fresh["observedCorrelatedCallbackStatus"],
        callbackOracleAgreement=agreement,
        diagnosticReason=document["reason"] if agreement else "TAGGED_RUNTIME_ORACLE_REPORT_MISMATCH",
        notes="Engineering acquisition only. Contradictory fields and absent witnesses remain original observations. The source, signed bytes, callback and trace are joined here; the actual Apple run/transport is independently authenticated. No full language requirement is accepted.")
    (Path(root) / DIAGNOSTIC).write_bytes(canonical(document))
    return document


def validate_public_diagnostic(value, root):
    repo = Path(__file__).resolve().parents[2]
    assertions = strict_json(read(root, "runtime/output-assertions.json", 256 * 1024))
    require(isinstance(assertions, dict), "PUBLIC_ASSERTIONS")
    fixture, commit = assertions.get("targetFixtureId"), assertions.get("compilerCommitSha")
    fresh = evaluate(repo, root, fixture, commit, assertions.get("expectedRunId"),
                     assertions.get("expectedSemanticOutput"))
    expected = observation_document(fresh, assertions.get("callbackOracleAgreement"))
    require(same(value, expected), "PUBLIC_RECOMPUTATION")
    verified = expected["runtimeObservationVerified"]
    for key, wanted in {
        "runtimeContractMode": MODE, "diagnosticOnly": True, "semanticContractVerified": False,
        "appleExecutionAuthenticated": False, "formalAcceptanceGranted": False,
        "runtimeContractVerified": verified, "runtimeObservationVerified": verified,
        "compiledShortcutCallbackVerified": verified, "compiledShortcutRuntimeExecuted": verified,
        "nativeCarrierGoalsMatched": expected["nativeCarrierGoalsMatched"],
        "expectedSemanticOutput": "", "expectedContractSha256": graph.CONTRACT_SHA256,
        "enrollmentSha256": ENROLLMENT_SHA256,
    }.items():
        require(same(assertions.get(key), wanted), "PUBLIC_ASSERTION_" + key)
    if verified:
        require(assertions.get("shortcutRunCompletedGate") == "PASS"
                and assertions.get("traceCapturedGate") == "PASS", "PUBLIC_GATES")
        require(read(root, "runtime/runtime-trace.jsonl", 16384) == trace_bytes(fresh["traceRecords"]),
                "PUBLIC_TRACE")
    return value
