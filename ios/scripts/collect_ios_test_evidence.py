#!/usr/bin/env python3
"""Retain bounded synthetic iOS test diagnostics even when xcodebuild fails.

This reuses the existing requirement/test mapping. A failed or missing process
exit code yields no acceptance records; individual observed outcomes remain
available in the separate diagnostic report. Raw logs and screenshots are not
published. UI source receipts require a verified empty editor reset.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re

# UI automation tests map to (requirements, assertions, required_checkpoint_marker)
ui_test_map = {
    "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI": (
        ["E01", "RP40"],
        {"completion_contextual": "true"},
        "[CHECKPOINT:E01_COMPLETION_INSERTED]"
    ),
    "CherriEditorUITests/testE02DiagnosticsRenderingAndNavigationUI": (
        ["E02"],
        {"multi_diagnostics": "true"},
        "[CHECKPOINT:E02_DIAGNOSTICS_VERIFIED]"
    ),
    "CherriEditorUITests/testE03EditorActionsHoverRenameUI": (
        ["E03"],
        {"scoped_rename": "true"},
        "[CHECKPOINT:E03_HOVER_SIG_DEF_RENAME_VERIFIED]"
    ),
    "CherriEditorUITests/testE05EditorResponsivenessStaleEditsUI": (
        ["E05"],
        {"versioned_edits_isolated": "true"},
        "[CHECKPOINT:E05_RAPID_EDITS_VERIFIED]"
    ),
    "CherriEditorUITests/testE06BidirectionalHebrewEmojiUI": (
        ["E06"],
        {"utf16_spans_exact": "true"},
        "[CHECKPOINT:E06_UNICODE_VERIFIED]"
    ),
    "CherriEditorUITests/testE07VisualParameterEditingCanvasUI": (
        ["E07", "RP41", "BRG28"],
        {"visual_source_edit_bounded": "true"},
        "[CHECKPOINT:E07_PREVIEW_EDIT_VERIFIED]"
    ),
    "CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI": (
        ["BRG27"],
        {"multi_diagnostics": "true", "completion_contextual": "true"},
        "[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]"
    ),
}

# Service and bridge integration tests map to tier: "service" or "integration"
service_test_map = {
    "CherriCoreIntegrationTests/testE01ContextualCompletionService": (["E01"], {"completion_contextual": "true"}),
    "CherriCoreIntegrationTests/testE02MultiDiagnosticAnalysisService": (["E02"], {"multi_diagnostics": "true"}),
    "CherriCoreIntegrationTests/testE03ScopedRenameIgnoresStringsCommentsAndShadowing": (["E03"], {"scoped_rename": "true"}),
    "CherriCoreIntegrationTests/testE04FormatterPreservesCommentsAndSemantics": (["E04", "RP39"], {"formatter_idempotent_comments": "true"}),
    "CherriCoreIntegrationTests/testE05OutOfOrderVersionedEdits": (["E05"], {"versioned_edits_isolated": "true"}),
    "CherriCoreIntegrationTests/testE06UTF16DiagnosticSpansEmojiAndHebrew": (["E06"], {"utf16_spans_exact": "true"}),
    "CherriCoreIntegrationTests/testE07BoundedVisualSourceEditAndRejection": (["E07", "RP41", "BRG28"], {"visual_source_edit_bounded": "true"}),
    "CherriCoreIntegrationTests/testRP40SwiftIntegrationService": (["RP40"], {"completion_contextual": "true"}),
    "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract": (["BRG25"], {"shared_analysis_bridge": "true"}),
    "CherriCoreIntegrationTests/testUnicodeIdentifierCompilationAndAnalysis": (["P05"], {"unicode_identifiers": "true"}),
}


CASE_LINE = re.compile(r"^Test Case '-\[(CherriTests\.CherriCoreIntegrationTests|CherriUITests\.CherriEditorUITests) (test[A-Za-z0-9_]+)\]' (started\.|(?:passed|failed) \([0-9.]+ seconds\)\.)$")
FAILURE_LINE = re.compile(r"CherriEditorUITests\.swift:(\d+): error: -\[CherriUITests\.CherriEditorUITests (test[A-Za-z0-9_]+)\]")
TEST_NAME = re.compile(r"^CherriEditorUITests/(test[A-Za-z0-9_]+)$")
PRIVATE_TEXT = re.compile(r"(?i)(BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._-]+|[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}|/Users/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+)")
STAGES = {"empty-reset", "typed-source", "completion-insertion", "rapid-typed-source", "teardown"}
BOOL_FIELDS = {
    "editorExists", "editorHittable", "buildExists", "buildEnabled",
    "compiledStatusExists", "diagnosticStatusExists", "diagnosticsBannerExists", "keyboardExists",
}
CHECKPOINTS = {entry[2] for entry in ui_test_map.values()} | {"[CHECKPOINT:EDITOR_TYPED_SOURCE_EXACT]"}
RECEIPT_PREFIX = "[CHERRI_UI_RECEIPT] "


def safe_receipt(line):
    try:
        raw = json.loads(line[len(RECEIPT_PREFIX):])
    except (ValueError, TypeError):
        return None
    if (not isinstance(raw, dict) or raw.get("schemaVersion") != "1.0"
            or not isinstance(raw.get("testName"), str) or not TEST_NAME.fullmatch(raw["testName"])
            or raw.get("stage") not in STAGES):
        return None
    receipt = {"schemaVersion": "1.0", "testName": raw["testName"], "stage": raw["stage"],
               "syntheticSourceResetVerified": raw.get("syntheticSourceResetVerified") is True}
    for key in BOOL_FIELDS:
        if type(raw.get(key)) is bool:
            receipt[key] = raw[key]
    # No arbitrary AX label or diagnostic message is copied from a possibly
    # restored document. Only this test's cleared-then-typed source is eligible.
    if receipt["syntheticSourceResetVerified"]:
        for key in ("actualSource", "expectedSource"):
            value = raw.get(key)
            if isinstance(value, str):
                if len(value.encode("utf-8")) > 16384 or PRIVATE_TEXT.search(value):
                    receipt[key] = "[REDACTED_UNEXPECTED_CONTENT]"
                    receipt["sourceRedacted"] = True
                else:
                    receipt[key] = value
        if type(raw.get("actualUTF16Length")) is int and 0 <= raw["actualUTF16Length"] <= 100000:
            receipt["actualUTF16Length"] = raw["actualUTF16Length"]
        for key in ("exactUTF16Match", "sourceTruncated"):
            if type(raw.get(key)) is bool:
                receipt[key] = raw[key]
    else:
        receipt["sourceOmittedReason"] = "EMPTY_RESET_NOT_VERIFIED"
    return receipt


def bounded_log(raw_text):
    lines, receipts, failures = [], [], []
    case_events = 0
    for raw_line in raw_text.splitlines():
        line = raw_line.strip()
        if CASE_LINE.fullmatch(line):
            case_events += 1
            if case_events <= 512:
                lines.append(line)
        elif line.startswith(RECEIPT_PREFIX) and len(receipts) < 64:
            receipt = safe_receipt(line)
            if receipt:
                receipts.append(receipt)
                lines.append(RECEIPT_PREFIX + json.dumps(receipt, ensure_ascii=False, sort_keys=True))
        elif (failure := FAILURE_LINE.search(line)):
            record = {"testId": "CherriEditorUITests/" + failure.group(2), "sourceLine": int(failure.group(1))}
            if len(failures) < 64:
                failures.append(record)
                lines.append("[CHERRI_UI_FAILURE] " + json.dumps(record, sort_keys=True))
        else:
            for marker in CHECKPOINTS:
                if line.startswith(marker):
                    lines.append(marker)
                    break
    text = "\n".join(lines) + ("\n" if lines else "")
    if len(text.encode("utf-8")) > 1200000:
        raise ValueError("bounded synthetic evidence exceeded its hard size limit")
    return text, receipts, failures


def collect(log_path, output_dir, implementation_sha, run_id, exit_code_path):
    if output_dir.exists() and any(output_dir.iterdir()):
        raise ValueError("evidence output already contains files; refusing to overwrite a prior run")
    output_dir.mkdir(parents=True, exist_ok=True)
    raw_present = log_path.is_file()
    raw_text = log_path.read_text(encoding="utf-8", errors="replace") if raw_present else ""
    log_text, receipts, failures = bounded_log(raw_text)
    log_bytes = log_text.encode("utf-8")
    log_sha = hashlib.sha256(log_bytes).hexdigest()
    (output_dir / "xcodetest.log").write_bytes(log_bytes)
    try:
        xcode_exit = int(exit_code_path.read_text(encoding="utf-8").strip())
    except (OSError, ValueError):
        xcode_exit = None
    impl_sha = implementation_sha if re.fullmatch(r"[0-9a-f]{40}", implementation_sha) else "UNKNOWN"
    run_id = run_id if re.fullmatch(r"[A-Za-z0-9_-]{1,128}", run_id) else "UNKNOWN"
    if impl_sha == "UNKNOWN" or not raw_present:
        xcode_exit = None

    pattern = re.compile(
        r"Test Case '-\[(\w+)\.(\w+)\s+(\w+)\]' started\.(.*?)Test Case '-\[\1\.\2\s+\3\]' (passed|failed)",
        re.DOTALL,
    )
    occurrences = {}
    for match in pattern.finditer(log_text):
        target, suite, method, body, status = match.groups()
        if "Test Case '" in body:
            status = "ambiguous"
        occurrences.setdefault((target, suite, method), []).append((status, body))
    test_blocks, test_results = {}, []
    for (target, suite, method), entries in sorted(occurrences.items()):
        status, body = entries[0] if len(entries) == 1 else ("ambiguous", "")
        test_blocks[f"{suite}/{method}"] = (status, body)
        test_blocks[f"{target}/{method}"] = (status, body)
        test_results.append({"testId": f"{suite}/{method}", "observedStatus": status})
    completed_ids = {entry["testId"] for entry in test_results}
    for line in log_text.splitlines():
        event = CASE_LINE.fullmatch(line)
        if event and event.group(3) == "started.":
            suite = event.group(1).split(".")[1]
            test_id = f"{suite}/{event.group(2)}"
            if test_id not in completed_ids:
                test_results.append({"testId": test_id, "observedStatus": "incomplete"})
                completed_ids.add(test_id)
    passed_tests = {key for key, (status, _) in test_blocks.items() if status == "passed"}
    eligible_run = xcode_exit == 0 and bool(test_results) and all(entry["observedStatus"] == "passed" for entry in test_results)

    records = []
    for test_id, (reqs, assertions, required_checkpoint) in sorted(ui_test_map.items()):
        method = test_id.split("/")[1]
        test_entry = test_blocks.get(test_id) or test_blocks.get(f"CherriUITests/{method}")
        if not test_entry:
            continue
        test_status, test_body = test_entry
        test_passed = (test_status == "passed")
        checkpoint_verified = (required_checkpoint in test_body)

        if eligible_run and test_passed and checkpoint_verified:
            canonical_test_id = f"CherriUITests/{method}"
            records.append({
                "evidence_id": f"ev-ios-ui-{method}",
                "requirements": reqs,
                "tier": "ios-ui",
                "implementation_sha": impl_sha,
                "test_id": canonical_test_id,
                "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                "exit_code": 0,
                "result": "passed",
                "assertions": assertions,
                "artifacts": [{
                    "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/xcodetest.log",
                    "sha256": log_sha,
                    "run_id": run_id,
                }],
                "detail": f"Executed and passed authentic UI automation test {canonical_test_id} verifying {reqs} within test block in iOS Simulator",
            })
        elif test_passed and not checkpoint_verified:
            print(f"WARNING: Test {test_id} passed in Xcode but missing verified assertion marker {required_checkpoint} inside test block! Omitting evidence record.")

    for test_id, (reqs, assertions) in sorted(service_test_map.items()):
        method = test_id.split("/")[1]
        if eligible_run and (test_id in passed_tests or f"CherriTests/{method}" in passed_tests):
            safe_name = method
            records.append({
                "evidence_id": f"ev-ios-service-{safe_name}",
                "requirements": reqs,
                "tier": "service",
                "implementation_sha": impl_sha,
                "test_id": test_id,
                "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                "exit_code": 0,
                "result": "passed",
                "assertions": assertions,
                "artifacts": [{
                    "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/xcodetest.log",
                    "sha256": log_sha,
                    "run_id": run_id,
                }],
                "detail": f"Executed and passed Swift service integration test {test_id} in iOS Simulator",
            })
            if test_id == "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract":
                records.append({
                    "evidence_id": "ev-ios-build-BRG25",
                    "requirements": ["BRG25"],
                    "tier": "ios-build",
                    "implementation_sha": impl_sha,
                    "test_id": test_id,
                    "command": ["xcodebuild", "build-for-testing", "-scheme", "Cherri"],
                    "exit_code": 0,
                    "result": "passed",
                    "assertions": assertions,
                    "artifacts": [{
                        "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/xcodetest.log",
                        "sha256": log_sha,
                        "run_id": run_id,
                    }],
                    "detail": "Built iOS Simulator app and verified shared bridge contract",
                })


    run_status = "PASSED" if eligible_run else ("FAILED" if xcode_exit not in (None, 0) else "NOT_RUN_OR_INCOMPLETE")
    diagnostic = {
        "schemaVersion": "1.0", "implementationSha": impl_sha, "runId": run_id,
        "xcodebuildExitCode": xcode_exit, "runStatus": run_status,
        "rawLogPresent": raw_present, "sanitizedLogSha256": log_sha,
        "testResults": test_results, "uiReceipts": receipts, "uiFailures": failures,
        "privacy": {"rawLogsPublished": False, "screenshotsPublished": False,
                    "sourceRequiresVerifiedEmptyReset": True},
    }
    (output_dir / "ios-test-diagnostics.json").write_text(json.dumps(diagnostic, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    (output_dir / "ios-test-evidence.json").write_text(json.dumps({"records": records}, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({"runStatus": run_status, "xcodebuildExitCode": xcode_exit,
                      "observedTestCount": len(test_results), "acceptanceRecordCount": len(records),
                      "boundedUIReceiptCount": len(receipts), "uiFailures": failures}))
    return diagnostic, records


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("log_path", type=Path)
    parser.add_argument("output_dir", type=Path)
    parser.add_argument("implementation_sha")
    parser.add_argument("run_id")
    parser.add_argument("exit_code_path", type=Path)
    args = parser.parse_args()
    collect(args.log_path, args.output_dir, args.implementation_sha, args.run_id, args.exit_code_path)
