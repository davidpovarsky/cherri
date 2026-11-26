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
import os
from pathlib import Path
import re

import documentation_evidence as docs
import joined_editor_evidence as joined
import preview_native_evidence as preview
import e07_pane_observations as e07

# UI automation tests map to (requirements, assertions, required_checkpoint_marker)
# The BRG25 bridge probe and BRG27 single-document build remain diagnostics.
# Their full joined requirements need dedicated complete producers.
ui_test_map = {
    "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI": (
        ["E01"],
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
    "CherriCoreIntegrationTests/testUnicodeIdentifierCompilationAndAnalysis": (["P05"], {"unicode_identifiers": "true"}),
}


# This dedicated hosted-app scenario does not supply its separate UI tier.
RP40_APP_TEST = "CherriCoreIntegrationTests/testRP40ActualAppEntrypointsJoinedContract"
RP40_APP_CHECKPOINT = "[CHECKPOINT:RP40_APP_ENTRYPOINTS_VERIFIED]"
RP40_APP_REJECTED = "[CHERRI_RP40_REJECTED] "
RP40_APP_FIXTURE = "ios/CherriTests/CherriCoreIntegrationTests.swift"
RP40_APP_ASSERTIONS = {
    "actual_app_entrypoints": "true",
    "current_completion_snapshot": "true",
    "same_document_diagnostics": "true",
    "semantic_rename_exact": "true",
    "unsigned_v2_compile": "true",
    "catalog_completion_compile_agree": "true",
    "negative_controls_executed": "true",
}
RP40_APP_SCOPE = (
    "Hosted Cherri app XCTest executes the production document, completion, "
    "analysis, rename and unsigned v2 compile APIs on one synthetic document; "
    "actual UI gestures, document-browser opening and rendered publication remain unproved."
)


# Dedicated RP40 UI producer. A hosted-app record cannot supply this tier.
RP40_UI_TEST = "CherriRP40UITests/testRP40ActualAppDocumentWorkflowUI"
RP40_UI_CHECKPOINT = "[CHECKPOINT:RP40_APP_UI_JOINED_VERIFIED]"
RP40_UI_REJECTED = "[CHERRI_RP40_UI_REJECTED] "
RP40_UI_STAGE_PREFIX = "[CHERRI_RP40_UI_STAGE] "
RP40_UI_STAGES = [
    "opened-synthetic-document", "contextual-completion", "unfilled-rejected",
    "source-compiled", "diagnostics-rendered", "semantic-rename", "renamed-source-compiled",
]
RP40_UI_FIXTURE = "ios/CherriUITests/CherriRP40UITests.swift"
RP40_UI_ASSERTIONS = {
    "app_document_ui": "true",
    "contextual_service_completion_ui": "true",
    "unfilled_editor_hole_rejected_ui": "true",
    "same_document_diagnostics_rendered": "true",
    "semantic_rename_exact_ui": "true",
    "unsigned_build_requested_ui": "true",
    "renamed_source_build_recovered": "true",
    "synthetic_reset_verified": "true",
}
RP40_UI_SCOPE = (
    "Genuine XCUITest drives one opened Cherri document through a verified empty "
    "reset, contextual completion, rejected unfilled input, unsigned Build, "
    "rendered diagnostics, exact semantic rename and renamed-source Build; "
    "the separate hosted-app integration tier is not supplied by this UI record."
)


CASE_LINE = re.compile(r"^Test Case '-\[(CherriTests\.(?:CherriCoreIntegrationTests|CherriEditorInputTests|CherriPreviewLifecycleTests|DocumentationIndexTests|CherriJoinedBridgeTests|CherriCompletionTextEditTests|CherriPreviewDocumentContextTests)|CherriUITests\.(?:CherriEditorUITests|CherriRP40UITests|CherriJoinedContractUITests|CherriPreviewDocumentUITests)) (test[A-Za-z0-9_]+)\]' (started\.|(?:passed|failed) \([0-9.]+ seconds\)\.)$")
FAILURE_LINE = re.compile(r"(CherriEditorUITests|CherriRP40UITests|CherriEditorInputTests|CherriPreviewLifecycleTests|CherriJoinedBridgeTests|CherriJoinedContractUITests|CherriCompletionTextEditTests|CherriPreviewDocumentContextTests|CherriPreviewDocumentUITests)\.swift:(\d+): error: -\[(?:CherriUITests|CherriTests)\.\1 (test[A-Za-z0-9_]+)\]")
TEST_NAME = re.compile(r"^CherriEditorUITests/(test[A-Za-z0-9_]+)$")
PRIVATE_TEXT = re.compile(r"(?i)(BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._-]+|[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}|/Users/[^/\s]+|[A-Z]:\\Users\\[^\\\s]+)")
STAGES = {"empty-reset", "typed-source", "completion-insertion", "rapid-typed-source", "teardown", "completion-candidates"}
BOOL_FIELDS = {
    "editorExists", "editorHittable", "buildExists", "buildEnabled",
    "compiledStatusExists", "diagnosticStatusExists", "diagnosticsBannerExists", "keyboardExists",
    "showCompletionExists", "showCompletionHittable", "showSuggestionExists", "showSuggestionHittable",
}
CHECKPOINTS = {entry[2] for entry in ui_test_map.values()} | {
    "[CHECKPOINT:EDITOR_TYPED_SOURCE_EXACT]", "[CHECKPOINT:E01_PALETTE_EXACT]",
    "[CHECKPOINT:BRG27_WORKFLOW_VERIFIED]",  # Bounded diagnostic only.
}
RECEIPT_PREFIX = "[CHERRI_UI_RECEIPT] "
STARTUP_PREFIX = "[CHERRI_UI_STARTUP] "
STARTUP_TIMING_PREFIX = "[CHERRI_UI_STARTUP_TIMING] "
STARTUP_STAGES = {"window-wait", "document-wait", "document-check", "editor-wait"}
STARTUP_BOOL_FIELDS = {"waitCompleted", "windowExists", "appRunningForeground",
                       "createDocumentExists", "createDocumentHittable", "editorExists", "editorHittable"}
INPUT_PREFIX = "[CHERRI_INPUT_RECEIPT] "
STOCK_PASTE_PREFIX = "[CHERRI_STOCK_PASTE_DIAGNOSTIC] "
RUNTIME_PREFIX = "[CHERRI_NATIVE_RUNTIME_EVENT] "
INPUT_FIXTURES = {"nested-call", "list", "dictionary", "curly-block", "block-comment", "single-character-paste"}
INPUT_TEST_NAME = re.compile(r"^CherriEditorInputTests/test[A-Za-z0-9_]+$")
CANDIDATE_ID = re.compile(r"^(?:completionItem|suggestionItem)_[A-Za-z_][A-Za-z0-9_]{0,95}$")
RUNTIME_SOURCE = re.compile(r"(?P<file>CodeStorage(?:Delegate)?|CodeView|Tokeniser|LineMap|ContiguousArrayBuffer|StringUTF16View)\.swift:(?P<line>[0-9]{1,6})")



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
        if (raw["testName"] == "CherriEditorUITests/testE01CompletionDisplayAndInsertionUI"
                and receipt.get("actualSource") == "sho"):
            candidates = raw.get("completionCandidateIdentifiers")
            if isinstance(candidates, list):
                receipt["completionCandidateIdentifiers"] = [
                    value for value in candidates[:8] if isinstance(value, str) and CANDIDATE_ID.fullmatch(value)]
    else:
        receipt["sourceOmittedReason"] = "EMPTY_RESET_NOT_VERIFIED"
    return receipt


def safe_startup_receipt(line, active_test):
    try:
        raw = json.loads(line[len(STARTUP_PREFIX):])
    except (ValueError, TypeError):
        return None
    if (not isinstance(raw, dict) or raw.get("schemaVersion") not in ("1.0", "1.1")
            or not isinstance(active_test, str) or not TEST_NAME.fullmatch(active_test)
            or raw.get("testName") != active_test
            or not isinstance(raw.get("stage"), str) or raw["stage"] not in STARTUP_STAGES
            or any(type(raw.get(key)) is not bool for key in STARTUP_BOOL_FIELDS)):
        return None
    # Startup has not established an empty document. Even a forged reset flag
    # cannot authorize source, document names, AX labels, or an arbitrary tree.
    receipt = {key: raw[key] for key in ("schemaVersion", "testName", "stage", *sorted(STARTUP_BOOL_FIELDS))}
    timing = {"elapsedMilliseconds", "budgetMilliseconds", "documentCreationIssued"}
    if raw["schemaVersion"] == "1.1":
        scope = raw.get("controlScope")
        if (not isinstance(scope, str) or scope not in ("document-and-editor", "editor-only")
                or not timing.issubset(raw)):
            return None
        if scope == "editor-only" and (
                raw["stage"] != "editor-wait" or raw.get("documentCreationIssued") is not True
                or raw["createDocumentExists"] is not False or raw["createDocumentHittable"] is not False):
            return None
        receipt["controlScope"] = scope
    elif "controlScope" in raw:
        return None
    if raw["stage"] == "document-check" and (
            raw["waitCompleted"] is not False or not timing.issubset(raw)):
        return None
    if timing.intersection(raw):
        if (type(raw.get("elapsedMilliseconds")) is not int or not 0 <= raw["elapsedMilliseconds"] <= 120000
                or type(raw.get("budgetMilliseconds")) is not int or raw["budgetMilliseconds"] != 28000
                or type(raw.get("documentCreationIssued")) is not bool):
            return None
        receipt.update({key: raw[key] for key in timing})
    return receipt


def safe_startup_timing(line, active_test):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError("duplicate timing key")
            result[key] = value
        return result
    try:
        raw = json.loads(line[len(STARTUP_TIMING_PREFIX):], object_pairs_hook=unique)
    except (ValueError, TypeError):
        return None
    expected = {"schemaVersion", "testName", "budgetMilliseconds", "elapsedMilliseconds", "entries", "droppedEntries", "inFlightObservations"}
    if (not isinstance(raw, dict) or set(raw) != expected or raw["schemaVersion"] != "1.0"
            or not isinstance(active_test, str) or not TEST_NAME.fullmatch(active_test)
            or raw["testName"] != active_test
            or type(raw["budgetMilliseconds"]) is not int or raw["budgetMilliseconds"] != 28000
            or type(raw["elapsedMilliseconds"]) is not int or not 0 <= raw["elapsedMilliseconds"] <= 120000
            or type(raw["droppedEntries"]) is not int or not 0 <= raw["droppedEntries"] <= 10000
            or type(raw["inFlightObservations"]) is not int or not 0 <= raw["inFlightObservations"] <= 128
            or not isinstance(raw["entries"], list) or len(raw["entries"]) > 128):
        return None
    snapshots = {"foreground", "window", "known-controls", "create-exists", "create-hittable", "editor-exists", "editor-hittable", "foreground-final"}
    operations = {"window-snapshot": snapshots, "document-snapshot": snapshots, "final-snapshot": snapshots,
                  "window-wait": {"entered", "returned"}, "document-tap": {"entered", "returned"},
                  "document-wait": snapshots | {"entered", "returned"},
                  "editor-wait": snapshots | {"entered", "returned"}}
    for index, entry in enumerate(raw["entries"]):
        if (not isinstance(entry, dict) or set(entry) != {"sequence", "phase", "operation", "startedMilliseconds", "completedMilliseconds", "value", "mainThread"}
                or type(entry["sequence"]) is not int or entry["sequence"] != index
                or not isinstance(entry["phase"], str) or entry["phase"] not in operations
                or not isinstance(entry["operation"], str) or entry["operation"] not in operations[entry["phase"]]
                or type(entry["value"]) is not bool or type(entry["mainThread"]) is not bool
                or type(entry["startedMilliseconds"]) is not int or type(entry["completedMilliseconds"]) is not int
                or not 0 <= entry["startedMilliseconds"] <= entry["completedMilliseconds"] <= raw["elapsedMilliseconds"]):
            return None
        if entry["operation"] in {"entered", "returned"} and entry["startedMilliseconds"] != entry["completedMilliseconds"]:
            return None
    if raw["droppedEntries"] and len(raw["entries"]) != 128:
        return None
    # Overlap is observable: a blocked predicate can still be in flight when the
    # waiter returns. Do not normalize such timing into an invented serial order.
    # Only fixed control Boolean values and timing are retained. This diagnostic
    # cannot supply source, satisfy a behavioral checkpoint, or grant acceptance.
    return raw


def safe_input_receipt(line, active_test):
    try:
        raw = json.loads(line[len(INPUT_PREFIX):])
    except (ValueError, TypeError):
        return None
    if (not isinstance(raw, dict) or raw.get("schemaVersion") != "1.0"
            or raw.get("testName") != active_test or not isinstance(active_test, str)
            or not INPUT_TEST_NAME.fullmatch(active_test)
            or raw.get("isolatedSyntheticEditor") is not True
            or raw.get("fixtureId") not in INPUT_FIXTURES):
        return None
    paste = raw["fixtureId"] == "single-character-paste"
    stages = {"before-paste", "after-paste-call", "after-paste-wait", "paste-native-text-change", "paste-predicate-match"} if paste else {"before-key", "after-key", "fixture-complete"}
    if raw.get("stage") not in stages:
        return None
    if paste and (active_test != "CherriEditorInputTests/testSingleCharacterPasteDoesNotConsumeGeneratedText"
                  or type(raw.get("pasteElapsedMilliseconds")) is not int
                  or not 0 <= raw["pasteElapsedMilliseconds"] <= 120000
                  or any(type(raw.get(key)) is not bool for key in ("viewIsFirstResponder", "windowAttached", "windowIsKey"))):
        return None
    receipt = {key: raw[key] for key in (
        "schemaVersion", "testName", "isolatedSyntheticEditor", "fixtureId", "stage")}
    for key in ("keyIndex", "actualUTF16Length", "selectionLocation", "selectionLength"):
        if type(raw.get(key)) is not int or not 0 <= raw[key] <= 100000:
            return None
        receipt[key] = raw[key]
    if paste and ((raw["stage"] == "paste-native-text-change" and not 1 <= raw.get("keyIndex", 0) <= 4)
                  or (raw["stage"] == "paste-predicate-match" and raw.get("keyIndex") != 0)):
        return None
    units = raw.get("inputUTF16")
    if not isinstance(units, list) or len(units) > 8 or any(type(unit) is not int or not 0 <= unit <= 65535 for unit in units):
        return None
    receipt["inputUTF16"] = units
    source = raw.get("actualSource")
    if not isinstance(source, str):
        return None
    receipt["actualSource"] = source if len(source.encode("utf-8")) <= 16384 and not PRIVATE_TEXT.search(source) else "[REDACTED_UNEXPECTED_CONTENT]"
    if paste:
        allowed = {"", 'text("x")', 'text("x"))', 'text("x")))', 'text("x"))))'}
        if source not in allowed:
            receipt["actualSource"] = "[REDACTED_UNEXPECTED_CONTENT]"
        receipt.update({key: raw[key] for key in ("pasteElapsedMilliseconds", "viewIsFirstResponder", "windowAttached", "windowIsKey")})
    return receipt


def safe_stock_paste_diagnostic(line, active_test):
    def unique(pairs):
        value = {}
        for key, item in pairs:
            if key in value:
                raise ValueError("duplicate stock paste key")
            value[key] = item
        return value
    try:
        if len(line) > 8192:
            return None
        raw = json.loads(line[len(STOCK_PASTE_PREFIX):], object_pairs_hook=unique)
    except (ValueError, TypeError):
        return None
    target = "CherriEditorInputTests/testSingleCharacterPasteDoesNotConsumeGeneratedText"
    fixed = {"schemaVersion": "1.0", "testName": target,
             "fixtureId": "stock-ui-text-view-after-code-view",
             "ordering": "AFTER_ORIGINAL_CODEVIEW_ASSERTION", "scope": "DIAGNOSTIC_ONLY_WARM_PROCESS"}
    booleans = {"isolatedSyntheticEditor", "viewIsFirstResponder", "windowAttached", "windowIsKey",
                "syntheticClipboardVerified", "pasteIssued", "exactExpectedText"}
    integers = {"budgetMilliseconds", "elapsedMilliseconds", "actualUTF16Length", "selectionLocation", "selectionLength"}
    expected_keys = set(fixed) | booleans | integers | {"stage", "actualSource", "waitResult"}
    if (active_test != target or not isinstance(raw, dict) or set(raw) != expected_keys
            or any(raw.get(key) != value for key, value in fixed.items())
            or any(type(raw.get(key)) is not bool for key in booleans)
            or raw["isolatedSyntheticEditor"] is not True
            or any(type(raw.get(key)) is not int for key in integers)
            or raw["budgetMilliseconds"] != 3000 or not 0 <= raw["elapsedMilliseconds"] <= 120000
            or not 0 <= raw["actualUTF16Length"] <= 100000
            or not 0 <= raw["selectionLocation"] <= raw["actualUTF16Length"]
            or not 0 <= raw["selectionLength"] <= raw["actualUTF16Length"] - raw["selectionLocation"]
            or not isinstance(raw["stage"], str)
            or raw["stage"] not in {"before-paste", "after-paste-call", "native-text-change", "after-paste-wait", "setup-failed"}):
        return None
    results = {"COMPLETED", "TIMED_OUT", "INCORRECT_ORDER", "INVERTED_FULFILLMENT", "INTERRUPTED", "UNKNOWN"}
    if (not isinstance(raw["waitResult"], str)
            or (raw["stage"] == "after-paste-wait" and raw["waitResult"] not in results)
            or (raw["stage"] != "after-paste-wait" and raw["waitResult"] != "NOT_WAITED")
            or raw["pasteIssued"] != (raw["stage"] not in {"before-paste", "setup-failed"})
            or (raw["stage"] != "setup-failed" and raw["syntheticClipboardVerified"] is not True)):
        return None
    source = raw["actualSource"]
    if not isinstance(source, str) or raw["exactExpectedText"] != (source == 'text("x"))'):
        return None
    allowed = {"", 'text("x")', 'text("x"))', 'text("x")))', 'text("x"))))'}
    if source in allowed:
        if raw["actualUTF16Length"] != len(source.encode("utf-16-le")) // 2:
            return None
    else:
        raw["actualSource"] = "[REDACTED_UNEXPECTED_CONTENT]"
    return raw


def safe_runtime_event(line, active_test):
    if not active_test:
        return None
    categories = (
        ("Fatal error: Index out of range", "INDEX_OUT_OF_RANGE"),
        ("Fatal error: String index is out of bounds", "STRING_INDEX_OUT_OF_BOUNDS"),
        ("NSRangeException", "NS_RANGE_EXCEPTION"),
        ("NSInternalInconsistencyException", "NS_INTERNAL_INCONSISTENCY"),
        ("Fatal error:", "RUNTIME_FATAL_ERROR"),
        ("Restarting after unexpected exit, crash, or test timeout", "XCTEST_HOST_RESTART"),
    )
    code = next((code for marker, code in categories if marker in line), None)
    if not code:
        return None
    record = {"testId": active_test, "code": code}
    if match := RUNTIME_SOURCE.search(line):
        record.update(sourceFile=match["file"] + ".swift", sourceLine=int(match["line"]))
    return record


def failure_category(message):
    """Retain only a fixed XCTest failure category, never the AX/error payload.

    Match the immediate failure-message prefix, not arbitrary assertion text or
    restored document content later in a log line. Unknown forms stay unknown.
    This diagnostic has no role in deciding whether a test or run passed.
    """
    prefix = message.lstrip(" :")[:160]
    patterns = (
        (r"Failed to get matching snapshot(?:s)?\b", "SNAPSHOT_QUERY_FAILURE"),
        (r"Failed to get attributes\b", "ATTRIBUTE_QUERY_FAILURE"),
        (r"Multiple matching elements found\b", "AMBIGUOUS_ELEMENT"),
        (r"No matches found for\b", "ELEMENT_NOT_FOUND"),
        (r"Application is not running\b", "APPLICATION_NOT_RUNNING"),
        (r"XCTAssert[A-Za-z0-9_]* failed\b", "ASSERTION_FAILURE"),
    )
    for pattern, category in patterns:
        if re.match(pattern, prefix):
            return category
    return "UNCLASSIFIED"


def bounded_log(raw_text):
    lines, receipts, failures = [], [], []
    case_events = 0
    input_events = 0
    stock_paste_events = 0
    original_paste_wait_observed = False
    runtime_events = 0
    startup_events = 0
    startup_timed_tests = set()
    active_test = None
    documentation_events = 0
    documentation_ui_events = 0
    rp40_ui_stage_index = 0
    joined_guard = joined.LogGuard(Path(__file__).resolve().parents[2])
    preview_guard = preview.LogGuard(Path(__file__).resolve().parents[2])
    e07_guard = e07.LogGuard()
    for raw_line in raw_text.splitlines():
        line = raw_line.strip()
        e07_line = e07_guard.observe(raw_line, active_test)
        if e07_line is not None:
            lines.append(e07_line)
            continue
        joined_line = joined_guard.observe(raw_line)
        preview_line = preview_guard.observe(raw_line)
        if preview_line is not None:
            lines.append(preview_line)
            if CASE_LINE.fullmatch(line) is None:
                continue
        if joined_line is not None:
            lines.append(joined_line)
            # A rejection must never erase an original supported test terminal:
            # failed remains failed, and the next case keeps its own identity.
            if CASE_LINE.fullmatch(line) is None:
                continue
        if case := CASE_LINE.fullmatch(line):
            e07_boundary = e07_guard.boundary(case.group(1), case.group(2), case.group(3))
            if e07_boundary is not None:
                lines.append(e07_boundary)
            active_test = (case.group(1).split(".")[1] + "/" + case.group(2)) if case.group(3) == "started." else None
            original_paste_wait_observed = False
            if active_test == RP40_UI_TEST:
                rp40_ui_stage_index = 0
            case_events += 1
            if case_events <= 512:
                lines.append(line)
        elif line.startswith(RP40_UI_STAGE_PREFIX):
            token = line[len(RP40_UI_STAGE_PREFIX):]
            if (active_test == RP40_UI_TEST and rp40_ui_stage_index < len(RP40_UI_STAGES)
                    and token == RP40_UI_STAGES[rp40_ui_stage_index]):
                lines.append(line)
                rp40_ui_stage_index += 1
            else:
                lines.append(RP40_UI_REJECTED + "INVALID_STAGE")
        elif line.startswith(RP40_UI_CHECKPOINT) or line.startswith(RP40_UI_REJECTED):
            if (line == RP40_UI_CHECKPOINT and active_test == RP40_UI_TEST
                    and rp40_ui_stage_index == len(RP40_UI_STAGES)):
                lines.append(line)
            else:
                lines.append(RP40_UI_REJECTED + "INVALID_CHECKPOINT")
        elif line.startswith(RP40_APP_CHECKPOINT) or line.startswith(RP40_APP_REJECTED):
            if line == RP40_APP_CHECKPOINT and active_test == RP40_APP_TEST:
                lines.append(line)
            else:
                lines.append(RP40_APP_REJECTED + "INVALID_CHECKPOINT")
        elif line.startswith(docs.UI_STATE_PREFIX):
            documentation_ui_events += 1
            if documentation_ui_events <= 8:
                try:
                    state = docs.parse_ui_state(line, active_test)
                    lines.append(docs.UI_STATE_PREFIX + json.dumps(state, sort_keys=True))
                except docs.EvidenceError:
                    pass
        elif line.startswith(docs.PREFIX):
            documentation_events += 1
            try:
                docs.require(active_test == docs.POSITIVE, "RECEIPT_OUTSIDE_POSITIVE_TEST")
                docs.require(documentation_events <= 4, "RECEIPT_LIMIT")
                docs.parse_receipt(line)
                # Preserve the accepted raw JSON line; do not reconstruct a
                # success-shaped receipt or publish rejected arbitrary fields.
                lines.append(raw_line)
            except docs.EvidenceError:
                if documentation_events <= 5:
                    lines.append(docs.REJECTED_PREFIX + "INVALID_RECEIPT")
        elif line.startswith(docs.CHECKPOINT):
            if line == docs.CHECKPOINT and active_test == docs.UI:
                lines.append(line)
            else:
                lines.append(docs.REJECTED_PREFIX + "INVALID_CHECKPOINT")
        elif line.startswith(docs.REJECTED_PREFIX):
            lines.append(docs.REJECTED_PREFIX + "UNTRUSTED_MARKER")
        elif line.startswith("Test Case '"):
            if active_test in docs.TEST_IDS:
                lines.append(docs.REJECTED_PREFIX + "UNSUPPORTED_TEST_BOUNDARY")
            active_test = None
            original_paste_wait_observed = False
        elif line.startswith(STARTUP_TIMING_PREFIX) and len(startup_timed_tests) < 16:
            receipt = safe_startup_timing(line, active_test)
            if receipt and active_test not in startup_timed_tests:
                startup_timed_tests.add(active_test)
                lines.append(STARTUP_TIMING_PREFIX + json.dumps(receipt, sort_keys=True))
        elif line.startswith(STARTUP_PREFIX) and startup_events < 64:
            receipt = safe_startup_receipt(line, active_test)
            if receipt:
                startup_events += 1
                lines.append(STARTUP_PREFIX + json.dumps(receipt, sort_keys=True))
        elif line.startswith(INPUT_PREFIX) and input_events < 512:
            receipt = safe_input_receipt(line, active_test)
            if receipt:
                input_events += 1
                lines.append(INPUT_PREFIX + json.dumps(receipt, ensure_ascii=False, sort_keys=True))
                if (receipt.get("fixtureId") == "single-character-paste"
                        and receipt.get("stage") == "after-paste-wait"):
                    original_paste_wait_observed = True
        elif line.startswith(STOCK_PASTE_PREFIX) and stock_paste_events < 8 and original_paste_wait_observed:
            receipt = safe_stock_paste_diagnostic(line, active_test)
            if receipt:
                stock_paste_events += 1
                lines.append(STOCK_PASTE_PREFIX + json.dumps(receipt, sort_keys=True))
        elif line.startswith(RECEIPT_PREFIX) and len(receipts) < 64:
            receipt = safe_receipt(line)
            if receipt:
                receipts.append(receipt)
                lines.append(RECEIPT_PREFIX + json.dumps(receipt, ensure_ascii=False, sort_keys=True))
        elif (failure := FAILURE_LINE.search(line)):
            record = {"testId": failure.group(1) + "/" + failure.group(3), "sourceLine": int(failure.group(2)),
                      "failureCategory": failure_category(line[failure.end():])}
            if len(failures) < 64:
                failures.append(record)
                lines.append("[CHERRI_UI_FAILURE] " + json.dumps(record, sort_keys=True))
        elif runtime_events < 64 and (event := safe_runtime_event(line, active_test)):
            runtime_events += 1
            lines.append(RUNTIME_PREFIX + json.dumps(event, sort_keys=True))
        else:
            for marker in CHECKPOINTS:
                if line.startswith(marker):
                    lines.append(marker)
                    break
    text = "\n".join(lines) + ("\n" if lines else "")
    if len(text.encode("utf-8")) > 1200000:
        raise ValueError("bounded synthetic evidence exceeded its hard size limit")
    return text, receipts, failures


def collect(log_path, output_dir, implementation_sha, run_id, exit_code_path,
            documentation_index=None, documentation_manifest=None, documentation_receipt=None,
            diagnostic_only=False):
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

    acceptance_eligible = eligible_run and not diagnostic_only
    records = []
    for test_id, (reqs, assertions, required_checkpoint) in sorted(ui_test_map.items()):
        method = test_id.split("/")[1]
        test_entry = test_blocks.get(test_id) or test_blocks.get(f"CherriUITests/{method}")
        if not test_entry:
            continue
        test_status, test_body = test_entry
        test_passed = (test_status == "passed")
        checkpoint_verified = (required_checkpoint in test_body)

        if acceptance_eligible and test_passed and checkpoint_verified:
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
        if acceptance_eligible and (test_id in passed_tests or f"CherriTests/{method}" in passed_tests):
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



    app_integration = {
        "status": "NOT_OBSERVED",
        "testId": RP40_APP_TEST,
        "scope": RP40_APP_SCOPE,
        "uiTierGranted": False,
    }
    app_integration_record = None
    app_entry = test_blocks.get(RP40_APP_TEST)
    if app_entry is not None:
        app_integration["status"] = "INELIGIBLE_OR_INCOMPLETE"
        app_status, app_body = app_entry
        if (acceptance_eligible and run_id != "UNKNOWN" and app_status == "passed"
                and app_body.splitlines().count(RP40_APP_CHECKPOINT) == 1
                and RP40_APP_REJECTED not in log_text):
            try:
                fixture = (Path(__file__).resolve().parents[2] / RP40_APP_FIXTURE).read_bytes()
                if not fixture or len(fixture) > 1024 * 1024:
                    raise ValueError("bounded RP40 fixture unavailable")
                fixture_sha = hashlib.sha256(fixture).hexdigest()
                app_integration.update(status="VERIFIED_APP_INTEGRATION", fixtureSHA256=fixture_sha)
                app_integration_record = {
                    "evidence_id": "ev-ios-app-integration-RP40",
                    "requirements": ["RP40"], "tier": "app-integration",
                    "implementation_sha": impl_sha,
                    "fixture_sha256": fixture_sha,
                    "test_id": RP40_APP_TEST,
                    "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                    "exit_code": 0, "result": "passed",
                    "assertions": dict(RP40_APP_ASSERTIONS),
                    "detail": RP40_APP_SCOPE,
                    "artifacts": [{
                        "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/xcodetest.log",
                        "sha256": log_sha, "run_id": run_id,
                    }],
                }
            except (OSError, ValueError):
                app_integration["status"] = "SOURCE_FIXTURE_UNAVAILABLE"

    app_ui = {
        "status": "NOT_OBSERVED",
        "testId": RP40_UI_TEST,
        "scope": RP40_UI_SCOPE,
        "appIntegrationTierGranted": False,
    }
    app_ui_record = None
    ui_entry = test_blocks.get(RP40_UI_TEST)
    if ui_entry is not None:
        app_ui["status"] = "INELIGIBLE_OR_INCOMPLETE"
        ui_status, ui_body = ui_entry
        if (acceptance_eligible and run_id != "UNKNOWN" and ui_status == "passed"
                and ui_body.splitlines().count(RP40_UI_CHECKPOINT) == 1
                and RP40_UI_REJECTED not in log_text
                and [line[len(RP40_UI_STAGE_PREFIX):] for line in ui_body.splitlines()
                     if line.startswith(RP40_UI_STAGE_PREFIX)] == RP40_UI_STAGES):
            try:
                fixture = (Path(__file__).resolve().parents[2] / RP40_UI_FIXTURE).read_bytes()
                if not fixture or len(fixture) > 1024 * 1024:
                    raise ValueError("bounded RP40 fixture unavailable")
                fixture_sha = hashlib.sha256(fixture).hexdigest()
                app_ui.update(status="VERIFIED_APP_UI", fixtureSHA256=fixture_sha)
                app_ui_record = {
                    "evidence_id": "ev-ios-ui-RP40",
                    "requirements": ["RP40"], "tier": "ios-ui",
                    "implementation_sha": impl_sha,
                    "fixture_sha256": fixture_sha,
                    "test_id": RP40_UI_TEST,
                    "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                    "exit_code": 0, "result": "passed",
                    "assertions": dict(RP40_UI_ASSERTIONS),
                    "detail": RP40_UI_SCOPE,
                    "artifacts": [{
                        "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/xcodetest.log",
                        "sha256": log_sha, "run_id": run_id,
                    }],
                }
            except (OSError, ValueError):
                app_ui["status"] = "SOURCE_FIXTURE_UNAVAILABLE"

    documentation = {"status": "NOT_REQUESTED", "errorCodes": []}
    documentation_files = {}
    documentation_record = None
    documentation_requested = (any(test in test_blocks for test in docs.TEST_IDS)
                               or docs.PREFIX in log_text or docs.REJECTED_PREFIX in log_text
                               or any(path is not None for path in (
                                   documentation_index, documentation_manifest, documentation_receipt)))
    if documentation_requested:
        documentation["status"] = "NOT_VERIFIED"
        try:
            index, documentation_files = docs.load_artifacts(
                documentation_index, documentation_manifest, documentation_receipt)
            # Inputs come only from explicit build paths, never from log values.
            for name, payload in documentation_files.items():
                (output_dir / name).write_bytes(payload)
            docs.require(documentation_receipt is not None, "MISSING_GENERATION_RECEIPT")
            docs.require(not diagnostic_only, "DIAGNOSTIC_ONLY_SCOPE")
            docs.require(eligible_run and run_id != "UNKNOWN", "RUN_NOT_ELIGIBLE")
            documentation_value = docs.receipt_from_log(log_text)
            docs.bind_receipt(documentation_value, index,
                              documentation_files["documentation-index.json"])
            documentation.update(status="VERIFIED_APP_TESTS", testIds=list(docs.TEST_IDS),
                                 bundleSHA256=documentation_value["bundleSHA256"])
            artifact_base = "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/"
            documentation_record = {
                "evidence_id": "ev-ios-integration-D03",
                "requirements": ["D03"], "tier": "integration",
                "implementation_sha": impl_sha,
                "test_id": "CherriUITests/testD03CurrentDocumentationSearchUI",
                "command": ["xcodebuild", "test", "-scheme", "Cherri"],
                "exit_code": 0, "result": "passed",
                "assertions": {"current_docs_bundle": "true",
                               "historical_docs_rejected": "true",
                               "current_docs_ui_search": "true"},
                "artifacts": [{"path": artifact_base + "xcodetest.log",
                               "sha256": log_sha, "run_id": run_id}] + [
                    {"path": artifact_base + name,
                     "sha256": hashlib.sha256(payload).hexdigest(), "run_id": run_id}
                    for name, payload in sorted(documentation_files.items())],
                "detail": "Three unique passed documentation app tests and their raw receipt "
                          "match the exact bundled index and supplied compatibility manifest; "
                          "website and installed Skill evidence remain separate",
            }
        except docs.EvidenceError as error:
            documentation["errorCodes"].append(str(error))

    try:
        joined_attempt = int(os.environ.get("GITHUB_RUN_ATTEMPT", ""))
    except ValueError:
        joined_attempt = None
    joined_summary, joined_files, joined_records = joined.collect(
        Path(__file__).resolve().parents[2], log_bytes, test_results, impl_sha,
        run_id, joined_attempt, acceptance_eligible, diagnostic_only)
    for name, raw in joined_files.items():
        (output_dir / name).write_bytes(raw)
    preview_summary, preview_files, preview_records = preview.collect(
        Path(__file__).resolve().parents[2], log_bytes, test_results, impl_sha,
        run_id, joined_attempt, acceptance_eligible, diagnostic_only)
    for name, raw in preview_files.items():
        (output_dir / name).write_bytes(raw)

    run_status = "PASSED" if eligible_run else ("FAILED" if xcode_exit not in (None, 0) else "NOT_RUN_OR_INCOMPLETE")
    diagnostic = {
        "schemaVersion": "1.0", "implementationSha": impl_sha, "runId": run_id,
        "xcodebuildExitCode": xcode_exit, "runStatus": run_status,
        "rawLogPresent": raw_present, "sanitizedLogSha256": log_sha,
        "diagnosticOnly": diagnostic_only,
        "testResults": test_results, "uiReceipts": receipts, "uiFailures": failures,
        "documentationEvidence": documentation,
        "appIntegrationEvidence": app_integration,
        "appIntegrationUIEvidence": app_ui,
        "joinedEditorEvidence": joined_summary,
        "previewNativeEvidence": preview_summary,
        "documentationUIStates": [json.loads(line[len(docs.UI_STATE_PREFIX):]) for line in log_text.splitlines() if line.startswith(docs.UI_STATE_PREFIX)],
        "startupReceipts": [json.loads(line[len(STARTUP_PREFIX):]) for line in log_text.splitlines() if line.startswith(STARTUP_PREFIX)],
        "startupTimingTraces": [json.loads(line[len(STARTUP_TIMING_PREFIX):]) for line in log_text.splitlines() if line.startswith(STARTUP_TIMING_PREFIX)],
        "inputReceipts": [json.loads(line[len(INPUT_PREFIX):]) for line in log_text.splitlines() if line.startswith(INPUT_PREFIX)],
        "stockPasteDiagnostics": [json.loads(line[len(STOCK_PASTE_PREFIX):]) for line in log_text.splitlines() if line.startswith(STOCK_PASTE_PREFIX)],
        "nativeRuntimeEvents": [json.loads(line[len(RUNTIME_PREFIX):]) for line in log_text.splitlines() if line.startswith(RUNTIME_PREFIX)],
        "privacy": {"rawLogsPublished": False, "screenshotsPublished": False,
                    "sourceRequiresVerifiedEmptyReset": True, "startupReceiptsContainOnlyKnownControlState": True},
    }
    (output_dir / "ios-test-diagnostics.json").write_text(json.dumps(diagnostic, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
    for preview_record in preview_records:
        preview_record["artifacts"].append({
            "path": preview.ARTIFACT_BASE + "ios-test-diagnostics.json",
            "sha256": preview.digest((output_dir / "ios-test-diagnostics.json").read_bytes()),
            "run_id": run_id})
        records.append(preview_record)
    for joined_record in joined_records:
        joined_record["artifacts"].append({
            "path": joined.ARTIFACT_BASE + "ios-test-diagnostics.json",
            "sha256": joined.digest((output_dir / "ios-test-diagnostics.json").read_bytes()),
            "run_id": run_id})
        records.append(joined_record)
    if app_integration_record is not None:
        diagnostic_bytes = (output_dir / "ios-test-diagnostics.json").read_bytes()
        app_integration_record["artifacts"].append({
            "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios-test-diagnostics.json",
            "sha256": hashlib.sha256(diagnostic_bytes).hexdigest(), "run_id": run_id})
        records.append(app_integration_record)
    if app_ui_record is not None:
        diagnostic_bytes = (output_dir / "ios-test-diagnostics.json").read_bytes()
        app_ui_record["artifacts"].append({
            "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios-test-diagnostics.json",
            "sha256": hashlib.sha256(diagnostic_bytes).hexdigest(), "run_id": run_id})
        records.append(app_ui_record)
    if documentation_record is not None:
        diagnostic_bytes = (output_dir / "ios-test-diagnostics.json").read_bytes()
        documentation_record["artifacts"].append({
            "path": "artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios-test-diagnostics.json",
            "sha256": hashlib.sha256(diagnostic_bytes).hexdigest(), "run_id": run_id})
        records.append(documentation_record)
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
    parser.add_argument("--documentation-index", type=Path)
    parser.add_argument("--documentation-manifest", type=Path)
    parser.add_argument("--documentation-receipt", type=Path)
    parser.add_argument("--diagnostic-only", action="store_true",
                        help="Retain actual outcomes without emitting any formal acceptance records")
    args = parser.parse_args()
    collect(args.log_path, args.output_dir, args.implementation_sha, args.run_id, args.exit_code_path,
            args.documentation_index, args.documentation_manifest, args.documentation_receipt,
            diagnostic_only=args.diagnostic_only)
