"""Exact native acquisition for two fixed public nested-List observations.

The independent fixture contracts retain contradictory values. Successful
acquisition never proves nested List semantics or grants formal acceptance.
"""
import json
from pathlib import Path
import plistlib
import re

import nested_list_probe as probe

MODE = "NESTED_LIST_OBSERVATION_ONLY"
MAX_CALLBACK = 256 * 1024
MAX_PLIST = 2 * 1024 * 1024


def require(ok, code):
    if not ok:
        raise ValueError("NESTED_RUNTIME_" + code)


def canonical(value):
    return (json.dumps(value, ensure_ascii=False, sort_keys=True,
                       separators=(",", ":"), allow_nan=False) + "\n").encode("utf-8")


def read(root, relative, maximum):
    root = Path(root).resolve()
    path = root / relative
    require(path.is_file() and not path.is_symlink(), "INPUT_FILE")
    require(all(not part.is_symlink() for part in [path] + list(path.parents)
                if part != root and root in part.parents), "INPUT_SYMLINK")
    require(root in path.resolve().parents, "INPUT_ESCAPE")
    require(0 < path.stat().st_size <= maximum, "INPUT_SIZE")
    raw = path.read_bytes()
    require(0 < len(raw) <= maximum, "INPUT_SIZE")
    return raw


def prepared(repo, fixture, source, unsigned):
    if fixture not in probe.FIXTURES:
        return None
    require(isinstance(unsigned, bytes) and 0 < len(unsigned) <= MAX_PLIST, "UNSIGNED_SIZE")
    value = probe.contract(repo, fixture)
    workflow = plistlib.loads(unsigned)
    require(isinstance(workflow, dict), "WORKFLOW")
    actions = workflow.get("WFWorkflowActions")
    return {"fixtureId": fixture, "source": probe.verify_source(repo, fixture, source),
            "unsignedPlistSha256": probe.sha(unsigned),
            "graph": probe.verify_graph(value, fixture, actions),
            "causalMutations": probe.verify_mutations(value, fixture, actions),
            "nativeListTypeProved": False, "formalAcceptanceGranted": False}


def provenance(repo, root, fixture, commit):
    require(fixture in probe.FIXTURES and isinstance(commit, str)
            and re.fullmatch(r"[0-9a-f]{40}", commit), "SOURCE_IDENTITY")
    paths = {"compilerManifest": "compiler-manifest.json",
             "compiledSource": "builds/" + fixture + ".cherri",
             "unsignedShortcut": "builds/" + fixture + ".shortcut",
             "signedShortcut": "builds/signed/" + fixture + ".shortcut",
             "signingProvenance": "runtime/signing-provenance.json"}
    raw = {key: read(root, path, 10 * 1024 * 1024 if key == "signedShortcut" else MAX_PLIST)
           for key, path in paths.items()}
    manifest = probe._json(raw["compilerManifest"])
    require(isinstance(manifest, dict) and manifest.get("syntheticOnly") is True
            and manifest.get("compilerCommitSha") == commit, "MANIFEST_IDENTITY")
    rows = manifest.get("fixtures")
    require(isinstance(rows, list) and all(isinstance(row, dict) for row in rows), "MANIFEST_ROWS")
    ids = [row.get("fixtureId") for row in rows]
    require(all(isinstance(fid, str) for fid in ids) and len(ids) == len(set(ids)), "MANIFEST_DUPLICATES")
    selected = [row for row in rows if row.get("fixtureId") == fixture]
    require(len(selected) == 1, "MANIFEST_FIXTURE")
    row = selected[0]
    proof = prepared(repo, fixture, raw["compiledSource"], raw["unsignedShortcut"])
    expected = {"fileName": fixture + ".cherri", "dialect": "v2", "buildProfile": "debug",
                "runtimeTraceExpectation": "REQUIRED_NDJSON",
                "sourceSha256": probe.SOURCE_HASHES[fixture],
                "unsignedPlistSha256": proof["unsignedPlistSha256"],
                "nativeActionCount": probe.COUNTS[fixture], "nestedListProbe": proof}
    for key, value in expected.items():
        require(probe._same(row.get(key), value), "PREPARED_" + key)
    signing = probe._json(raw["signingProvenance"])
    signed = raw["signedShortcut"]
    require(signed.startswith(b"AEA1") and len(signed) > 4, "SIGNED_BYTES")
    require(isinstance(signing, dict)
            and signing.get("compilerCommitSha") == commit
            and signing.get("signedArtifactValidAEA1") is True
            and signing.get("signedArtifactSha256") == probe.sha(signed)
            and type(signing.get("signedByteSize")) is int
            and signing["signedByteSize"] == len(signed), "SIGNING_SUMMARY")
    target = signing.get("targetProvenance")
    require(isinstance(target, dict), "EXACT_SIGNING_PROVENANCE")
    expected_signing = {
        "compilerCommitSha": commit, "sourceSha256": probe.SOURCE_HASHES[fixture],
        "unsignedPlistSha256": probe.sha(raw["unsignedShortcut"]),
        "unsignedByteSize": len(raw["unsignedShortcut"]), "nativeActionCount": probe.COUNTS[fixture],
        "signedArtifactSha256": probe.sha(signed), "signedByteSize": len(signed), "headerMagic": "AEA1",
    }
    for key, value in expected_signing.items():
        require(probe._same(target.get(key), value), "SIGNED_" + key)
    return {key: {"path": paths[key], "bytes": len(data), "sha256": probe.sha(data)}
            for key, data in raw.items()}


def trace_records(text, fixture):
    require(isinstance(text, str) and 0 < len(text.encode("utf-8")) <= 8192, "TRACE_SIZE")
    lines = text.splitlines()
    require(len(lines) == 4 and all(line.strip() for line in lines), "TRACE_FOUR_EVENTS")
    records = [probe._json(line) for line in lines]
    for index, stage in ((0, "collector_begin"), (3, "collector_end")):
        expected = probe._json(probe._collector(fixture, stage))
        require(probe._same(records[index], expected), "TRACE_COLLECTOR")
    for index, stage, sequence, action in ((1, "before", 3, "setvariable"),
                                            (2, "after", 49 if fixture == probe.CONTROL else 71, "comment")):
        record = records[index]
        require(isinstance(record, dict), "TRACE_RECORD")
        stamp = record.get("timestamp")
        require(isinstance(stamp, str) and 0 < len(stamp.encode("utf-8")) <= 256
                and stamp.strip() and "\ufffc" not in stamp and "\x00" not in stamp, "TRACE_TIMESTAMP")
        expected = {"schemaVersion": "1.4", "kind": "INLINE_DEBUG_ACTION", "stage": stage,
                    "label": fixture[3:-3], "stepSequence": sequence,
                    "actionIdentifier": "is.workflow.actions." + action,
                    "timestampPrecision": "apple-currentdate", "valueType": "UNKNOWN_AT_TRACE_EVENT",
                    "timestamp": stamp}
        require(probe._same(record, expected), "TRACE_INLINE")
    return records


def trace_bytes(records):
    return b"".join(canonical(record) for record in records)


def evaluate(repo, root, fixture, commit, nonce, expected_output):
    result = {
        "schemaVersion": "1.0", "runtimeContractMode": MODE, "diagnosticOnly": True,
        "targetFixtureId": fixture, "compilerCommitSha": commit,
        "buildProfile": "debug", "traceExpectation": "REQUIRED_NDJSON", "releaseBuildVerified": False,
        "sourceSha256": probe.SOURCE_HASHES.get(fixture, ""), "unsignedPlistSha256": "",
        "expectedContractSha256": probe.CONTRACT_HASHES.get(fixture, ""),
        "runGate": "FAIL_RUNTIME", "traceGate": "FAIL_RUNTIME", "reason": "not observed",
        "inputBindings": {}, "observation": None, "traceRecords": [],
        "observedCorrelatedCallbackStatus": "UNKNOWN", "runtimeObservationVerified": False,
        "semanticContractVerified": False, "nativeListTypeProved": False, "formalAcceptanceGranted": False,
    }
    try:
        require(fixture in probe.FIXTURES and expected_output == "", "DIAGNOSTIC_MODE")
        require(isinstance(nonce, str) and re.fullmatch(r"[A-Za-z0-9_-]{1,180}", nonce), "FRESH_NONCE")
        result["inputBindings"] = provenance(repo, root, fixture, commit)
        result["unsignedPlistSha256"] = result["inputBindings"]["unsignedShortcut"]["sha256"]
        raw = read(root, "runtime/callback-events.json", MAX_CALLBACK)
        result["inputBindings"]["callbackEvents"] = {"path": "runtime/callback-events.json",
                                                    "bytes": len(raw), "sha256": probe.sha(raw)}
        events = probe._json(raw)
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
        observation = probe.validate_summary(event.get("resultSummary"), fixture)
        result["observation"] = observation
        result["runGate"] = "PASS"
        records = trace_records(event.get("tracePayload"), fixture)
        result["traceRecords"] = records
        result["tracePayloadSha256"] = probe.sha(event["tracePayload"].encode("utf-8"))
        result["normalizedTraceSha256"] = probe.sha(trace_bytes(records))
        result["traceGate"] = "PASS"
        result["runtimeObservationVerified"] = True
        result["reason"] = "fresh signed public observation and exact four-event trace; semantic acceptance remains unproved"
    except (OSError, ValueError, TypeError, KeyError, AttributeError, OverflowError) as exc:
        # Do not echo arbitrary source, callback or platform error text.
        result["reason"] = str(exc) if isinstance(exc, ValueError) and str(exc).startswith(("NESTED_RUNTIME_", "NESTED_LIST_")) else "NESTED_RUNTIME_INVALID_INPUT"
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
    return {**value, "callbackOracleAgreement": agreement,
            "runtimeObservationVerified": value["runtimeObservationVerified"] and agreement}


def apply_assertions(assertions, oracle, repo, root, commit, expected_output):
    fixture = assertions["targetFixtureId"]
    fresh = evaluate(repo, root, fixture, commit, assertions["expectedRunId"], expected_output)
    agreement = (probe._same(oracle, fresh)
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
        nativeListTypeProved=False, formalAcceptanceGranted=False,
        expectedSemanticOutput="", expectedContractSha256=probe.CONTRACT_HASHES[fixture],
        runtimeContractVerified=verified, runtimeObservationVerified=verified,
        compiledShortcutCallbackVerified=verified, compiledShortcutRuntimeExecuted=verified,
        traceObserved="NDJSON_VERIFIED" if verified else "UNVERIFIED",
        observedCorrelatedCallbackStatus=fresh["observedCorrelatedCallbackStatus"],
        callbackOracleAgreement=agreement,
        diagnosticReason=document["reason"] if agreement else "NESTED_RUNTIME_ORACLE_REPORT_MISMATCH",
        notes="Diagnostic acquisition only. Values are observations under a fixed independent contract; contrary values are retained. No nested List type or formal requirement is accepted.")
    destination = Path(root) / "runtime/nested-list-observation.json"
    destination.write_bytes(canonical(document))
    return document


def validate_public_diagnostic(value, root):
    repo = Path(__file__).resolve().parents[2]
    assertions = probe._json(read(root, "runtime/output-assertions.json", 256 * 1024))
    require(isinstance(assertions, dict), "PUBLIC_ASSERTIONS")
    fixture, commit = assertions.get("targetFixtureId"), assertions.get("compilerCommitSha")
    fresh = evaluate(repo, root, fixture, commit, assertions.get("expectedRunId"),
                     assertions.get("expectedSemanticOutput"))
    agreement = assertions.get("callbackOracleAgreement")
    expected = observation_document(fresh, agreement)
    require(probe._same(value, expected), "PUBLIC_RECOMPUTATION")
    verified = expected["runtimeObservationVerified"]
    for key, wanted in {
        "runtimeContractMode": MODE, "diagnosticOnly": True, "semanticContractVerified": False,
        "nativeListTypeProved": False, "formalAcceptanceGranted": False,
        "runtimeContractVerified": verified, "runtimeObservationVerified": verified,
        "compiledShortcutCallbackVerified": verified, "compiledShortcutRuntimeExecuted": verified,
        "expectedSemanticOutput": "", "expectedContractSha256": probe.CONTRACT_HASHES.get(fixture),
    }.items():
        require(probe._same(assertions.get(key), wanted), "PUBLIC_ASSERTION_" + key)
    if verified:
        require(assertions.get("shortcutRunCompletedGate") == "PASS"
                and assertions.get("traceCapturedGate") == "PASS", "PUBLIC_GATES")
        require(read(root, "runtime/runtime-trace.jsonl", 16384) == trace_bytes(fresh["traceRecords"]),
                "PUBLIC_TRACE")
    return value
