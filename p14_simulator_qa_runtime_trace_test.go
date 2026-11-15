package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP14_TraceEventAndCollectorNDJSONEscapingAndUTF16Ranges(t *testing.T) {
	specialStages := []struct {
		name  string
		stage string
		label string
		runID string
	}{
		{
			name:  "quotes-backslashes-newlines",
			stage: "before",
			label: "step \"quoted\" \\path\\ line1\nline2\t!",
			runID: "run-\"special\"-01",
		},
		{
			name:  "hebrew-rtl-and-non-bmp-emoji",
			stage: "after",
			label: "בדיקת קיצור דרך 🧪👋🏽 עם אמוג׳י",
			runID: "run-rtl-🧪-02",
		},
		{
			name:  "embedded-ufffc-and-json-tokens",
			stage: "end",
			label: "marker \uFFFC and {\"nested\": [1, true, null]}",
			runID: "run-ufffc-\uFFFC-03",
		},
	}

	for _, tc := range specialStages {
		t.Run(tc.name, func(t *testing.T) {
			evTok := debugTraceEventToken(tc.stage, tc.label, "is.workflow.actions.gettext", 3)
			valMap, ok := evTok["Value"].(map[string]any)
			if !ok {
				t.Fatalf("expected map Value on debugTraceEventToken, got %T", evTok["Value"])
			}
			rawStr, _ := valMap["string"].(string)
			attMap, _ := valMap["attachmentsByRange"].(map[string]any)
			if len(attMap) != 1 {
				t.Fatalf("expected exactly 1 attachment range in trace event token, got %d", len(attMap))
			}

			// Verify UTF-16 code-unit offset points to the exact U+FFFC placeholder rune
			for rangeKey := range attMap {
				var utf16Off, utf16Len int
				if _, err := fmt.Sscanf(rangeKey, "{%d, %d}", &utf16Off, &utf16Len); err != nil {
					t.Fatalf("invalid range key %q: %v", rangeKey, err)
				}
				runeIdx := utf16OffsetToRuneIndex(rawStr, utf16Off)
				runes := []rune(rawStr)
				if runeIdx < 0 || runeIdx >= len(runes) || runes[runeIdx] != '\uFFFC' {
					t.Fatalf("UTF-16 offset %d (runeIdx=%d) in %q did not land on U+FFFC", utf16Off, runeIdx, rawStr)
				}
			}

			// Simulate runtime interpolation of CurrentDate ISO8601 into the event line
			simulatedEventLine := strings.Replace(rawStr, "\uFFFC", "2026-11-15T20:00:00Z", 1)
			var parsedEvent QATraceRecord
			if err := json.Unmarshal([]byte(simulatedEventLine), &parsedEvent); err != nil {
				t.Fatalf("interpolated trace event is not valid JSON: %v\nline: %s", err, simulatedEventLine)
			}
			if parsedEvent.Kind != "INLINE_DEBUG_ACTION" || parsedEvent.Stage != tc.stage || parsedEvent.Label != tc.label {
				t.Fatalf("unexpected parsed event: %+v", parsedEvent)
			}

			// Now test collector envelope token UTF-16 offset and simulated runtime expansion
			envTok := debugExportTraceEnvelopeToken(tc.runID, "MOCK-COMBINED-UUID")
			envVal, ok := envTok["Value"].(map[string]any)
			if !ok {
				t.Fatalf("expected map Value on debugExportTraceEnvelopeToken, got %T", envTok["Value"])
			}
			envStr, _ := envVal["string"].(string)
			envAtt, _ := envVal["attachmentsByRange"].(map[string]any)
			if len(envAtt) != 1 {
				t.Fatalf("expected 1 attachment in envelope token, got %d", len(envAtt))
			}
			for rangeKey := range envAtt {
				var utf16Off, utf16Len int
				if _, err := fmt.Sscanf(rangeKey, "{%d, %d}", &utf16Off, &utf16Len); err != nil {
					t.Fatalf("invalid envelope range key %q: %v", rangeKey, err)
				}
				runeIdx := utf16OffsetToRuneIndex(envStr, utf16Off)
				runes := []rune(envStr)
				if runeIdx < 0 || runeIdx >= len(runes) || runes[runeIdx] != '\uFFFC' {
					t.Fatalf("envelope UTF-16 offset %d (runeIdx=%d) did not land on U+FFFC", utf16Off, runeIdx)
				}
			}

			// Simulate Combine Text expanding 2 multiline NDJSON events into the envelope
			ev2Tok := debugTraceEventToken("end", "done-"+tc.label, "is.workflow.actions.output", 9)
			ev2 := strings.Replace(
				ev2Tok["Value"].(map[string]any)["string"].(string),
				"\uFFFC",
				"2026-11-15T20:00:01Z",
				1,
			)
			combinedEvents := simulatedEventLine + "\n" + ev2
			fullRuntimePayload := strings.Replace(envStr, "\uFFFC", combinedEvents, 1)

			records, err := ParseAndValidateQATracePayload(fullRuntimePayload, tc.runID, true)
			if err != nil {
				t.Fatalf("ParseAndValidateQATracePayload failed on valid expanded NDJSON: %v\npayload:\n%s", err, fullRuntimePayload)
			}
			if len(records) != 4 {
				t.Fatalf("expected 4 NDJSON records (begin, 2 inline, end), got %d", len(records))
			}
		})
	}
}

func TestP14_TracePayloadValidatorFailsClosedOnCorruptedPayloads(t *testing.T) {
	validPayload := strings.Join([]string{
		"===CHERRI_QA_TRACE_NDJSON_V14===",
		`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_begin","runId":"qa-run-1","traceTransport":"ndjson-stream"}`,
		`{"schemaVersion":"1.4","kind":"INLINE_DEBUG_ACTION","stage":"before","label":"step-1","stepSequence":1,"timestamp":"2026-11-15T20:00:00Z"}`,
		`{"schemaVersion":"1.4","kind":"INLINE_DEBUG_ACTION","stage":"end","label":"done","stepSequence":2,"timestamp":"2026-11-15T20:00:01Z"}`,
		`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_end","runId":"qa-run-1","status":"OK"}`,
		"===CHERRI_QA_TRACE_NDJSON_V14===",
	}, "\n")

	if _, err := ParseAndValidateQATracePayload(validPayload, "qa-run-1", true); err != nil {
		t.Fatalf("expected valid payload to pass: %v", err)
	}

	negatives := []struct {
		name        string
		payload     string
		expectedRun string
	}{
		{
			name:        "empty-payload",
			payload:     "",
			expectedRun: "qa-run-1",
		},
		{
			name:        "corrupted-json-line",
			payload:     strings.Replace(validPayload, `"stage":"before"`, `"stage":"before`, 1),
			expectedRun: "qa-run-1",
		},
		{
			name:        "wrong-run-id",
			payload:     validPayload,
			expectedRun: "qa-run-999",
		},
		{
			name:        "missing-collector-end",
			payload:     strings.Replace(validPayload, `"stage":"collector_end"`, `"stage":"other"`, 1),
			expectedRun: "qa-run-1",
		},
		{
			name: "missing-inline-events",
			payload: strings.Join([]string{
				`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_begin","runId":"qa-run-1","traceTransport":"ndjson-stream"}`,
				`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_end","runId":"qa-run-1","status":"OK"}`,
			}, "\n"),
			expectedRun: "qa-run-1",
		},
	}

	for _, neg := range negatives {
		t.Run(neg.name, func(t *testing.T) {
			if _, err := ParseAndValidateQATracePayload(neg.payload, neg.expectedRun, true); err == nil {
				t.Fatalf("expected %s to fail closed, but it passed", neg.name)
			}
		})
	}
}

func TestP14_ClassifierDoesNotMisclassifyUserTextContainingDebugSubstrings(t *testing.T) {
	v2Source := `
let spoof1 = text("User text with literal \"kind\":\"INLINE_DEBUG_ACTION\" inside")
let spoof2 = text("User text with literal \"kind\":\"COLLECTOR_EVENT\" and __cherri_qa_trace")
let spoof3 = text("")
debugClearLogs()
debugTraceEvent(stage: "before", label: "real-trace")
let realVal = number(42)
debugLog(withDates: true)
let env = debugExportTrace(runId: "classifier-collision-test")
output(spoof1)
`
	v2Uninstrumented := `
let spoof1 = text("User text with literal \"kind\":\"INLINE_DEBUG_ACTION\" inside")
let spoof2 = text("User text with literal \"kind\":\"COLLECTOR_EVENT\" and __cherri_qa_trace")
let spoof3 = text("")
let realVal = number(42)
output(spoof1)
`
	instActions := compileDebugFixture(t, v2Source, false)
	uninstActions := compileDebugFixture(t, v2Uninstrumented, false)

	instClassified := ClassifyShortcutTraceActions(instActions)
	uninstClassified := ClassifyShortcutTraceActions(uninstActions)

	var instSourceIDs, uninstSourceIDs []string
	for _, c := range instClassified {
		if c.Kind == "SOURCE_ACTION" {
			instSourceIDs = append(instSourceIDs, c.ActionIdentifier)
		}
	}
	for _, c := range uninstClassified {
		if c.Kind == "SOURCE_ACTION" {
			uninstSourceIDs = append(uninstSourceIDs, c.ActionIdentifier)
		}
	}

	if len(uninstSourceIDs) != 5 {
		t.Fatalf("expected all 5 uninstrumented actions (including spoofed text literals and empty text) to be SOURCE_ACTION, got %d: %v", len(uninstSourceIDs), uninstSourceIDs)
	}
	if strings.Join(instSourceIDs, ",") != strings.Join(uninstSourceIDs, ",") {
		t.Fatalf("instrumented vs uninstrumented SOURCE_ACTION mismatch:\n  inst:   %v\n  uninst: %v", instSourceIDs, uninstSourceIDs)
	}
}

func TestP14_GateStateMachineAndAntiFalsePassNegativeMatrix(t *testing.T) {
	validTrace := strings.Join([]string{
		"===CHERRI_QA_TRACE_NDJSON_V14===",
		`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_begin","runId":"qa-functional-v2","traceTransport":"ndjson-stream"}`,
		`{"schemaVersion":"1.4","kind":"INLINE_DEBUG_ACTION","stage":"before","label":"step-1-greeting","stepSequence":1,"timestamp":"2026-11-15T20:00:00Z"}`,
		`{"schemaVersion":"1.4","kind":"INLINE_DEBUG_ACTION","stage":"end","label":"workflow-complete","stepSequence":2,"timestamp":"2026-11-15T20:00:01Z"}`,
		`{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_end","runId":"qa-functional-v2","status":"OK"}`,
		"===CHERRI_QA_TRACE_NDJSON_V14===",
	}, "\n")

	baseValidInput := func() SimulatorQAOracleInput {
		return SimulatorQAOracleInput{
			RunnerAvailable:         true,
			SimulatorBooted:         true,
			ShortcutsAppInstalled:   true,
			FileImportAttempted:     true,
			HostSignSucceeded:       true,
			SignedArtifactPresent:   true,
			SignedArtifactValidAEA1: true,
			ImportConfirmedInUI:     true,
			ExpectedShortcutName:    "qa-functional-v2",
			ExpectedRunID:           "run-nonce-001",
			ExpectedTraceRunID:      "qa-functional-v2",
			ExpectedSemanticOutput:  "CHERRI_QA_RESULT:ok:count=2:mode=v2",
			RequireAnchorPrefixTop:  "ANCHOR_CARD_01",
			RequireAnchorPrefixBot:  "ANCHOR_CARD_28",
			EditorCoverage: SimulatorQAEditorCoverage{
				SchemaVersion:              "1.4",
				ShortcutLaunched:           true,
				ShortcutForegroundVerified: true,
				ExpectedShortcutName:       "qa-functional-v2",
				ShortcutExistsInLibrary:    true,
				EditorOpened:               true,
				EditorIdentityVerified:     true,
				ScrollContainerFound:       true,
				ScrollContainerType:        "CollectionView",
				TotalPagesCaptured:         4,
				MaxEditorPages:             20,
				CaptureStatus:              "EDITOR_CAPTURE_COMPLETE",
				DetectedActionCards:        []string{"Cell:Comment", "Cell:Text", "Cell:Stop and Output"},
				DetectedAnchors:            []string{"ANCHOR_CARD_01_TOP_HEADER", "ANCHOR_CARD_15_MID_CENTER", "ANCHOR_CARD_28_BOTTOM_FOOTER"},
				BottomReachedEvidence:      "MULTI_SIGNAL_STALL_AT_PAGE_4",
			},
			CallbackEvents: []SimulatorQACallbackEvent{
				{
					SchemaVersion:    "1.4",
					RunID:            "receiver-self-test",
					Status:           "SUCCESS",
					Host:             "success",
					ResultSummary:    "CHERRI_QA_RECEIVER_OK",
					ReceiverSelfTest: true,
				},
				{
					SchemaVersion:    "1.4",
					RunID:            "run-nonce-001",
					FixtureID:        "qa-functional-v2",
					Status:           "SUCCESS",
					Host:             "success",
					ResultSummary:    "CHERRI_QA_RESULT:ok:count=2:mode=v2\n" + validTrace,
					ReceiverSelfTest: false,
				},
			},
		}
	}

	t.Run("positive-all-simulator-gates-pass", func(t *testing.T) {
		eval := EvaluateSimulatorQAGates(baseValidInput())
		if eval.Gates["SHORTCUT_RUN_COMPLETED"] != "PASS" ||
			eval.Gates["TRACE_CAPTURED"] != "PASS" ||
			eval.Gates["ACTION_EDITOR_CAPTURED"] != "EDITOR_CAPTURE_COMPLETE" ||
			eval.Gates["PHYSICAL_DEVICE_TESTED"] != "BLOCKED_DEVICE" ||
			eval.ReleaseReadiness != "READY_SIMULATOR_VERIFIED" {
			t.Fatalf("unexpected positive evaluation: %+v", eval)
		}
	})

	t.Run("negative-self-test-callback-cannot-satisfy-shortcut-run", func(t *testing.T) {
		in := baseValidInput()
		in.CallbackEvents = []SimulatorQACallbackEvent{
			{
				SchemaVersion:    "1.4",
				RunID:            "run-nonce-001",
				Status:           "SUCCESS",
				ResultSummary:    "CHERRI_QA_RESULT:ok:count=2:mode=v2\n" + validTrace,
				ReceiverSelfTest: true, // Spoofed self-test!
			},
		}
		eval := EvaluateSimulatorQAGates(in)
		if eval.Gates["SHORTCUT_RUN_COMPLETED"] == "PASS" || eval.Gates["TRACE_CAPTURED"] == "PASS" {
			t.Fatalf("self-test callback must not satisfy SHORTCUT_RUN_COMPLETED: %+v", eval)
		}
	})

	t.Run("negative-stale-run-id-rejected", func(t *testing.T) {
		in := baseValidInput()
		in.CallbackEvents[1].RunID = "stale-run-nonce-000"
		eval := EvaluateSimulatorQAGates(in)
		if eval.Gates["SHORTCUT_RUN_COMPLETED"] != "FAIL_RUNTIME" {
			t.Fatalf("stale runId must fail runtime gate, got %s", eval.Gates["SHORTCUT_RUN_COMPLETED"])
		}
	})

	t.Run("negative-duplicate-run-id-replay-rejected", func(t *testing.T) {
		in := baseValidInput()
		in.CallbackEvents = append(in.CallbackEvents, in.CallbackEvents[1])
		eval := EvaluateSimulatorQAGates(in)
		if eval.Gates["SHORTCUT_RUN_COMPLETED"] != "FAIL_RUNTIME" {
			t.Fatalf("duplicate callback replay must fail runtime gate, got %s", eval.Gates["SHORTCUT_RUN_COMPLETED"])
		}
	})

	t.Run("negative-error-or-cancelled-callback-rejected", func(t *testing.T) {
		for _, status := range []string{"ERROR", "CANCELLED"} {
			in := baseValidInput()
			in.CallbackEvents[1].Status = status
			eval := EvaluateSimulatorQAGates(in)
			if eval.Gates["SHORTCUT_RUN_COMPLETED"] != "FAIL_RUNTIME" {
				t.Fatalf("status=%s must fail runtime gate, got %s", status, eval.Gates["SHORTCUT_RUN_COMPLETED"])
			}
		}
	})

	t.Run("negative-wrong-output-string-rejected", func(t *testing.T) {
		in := baseValidInput()
		in.CallbackEvents[1].ResultSummary = "CHERRI_QA_UNEXPECTED_BRANCH\n" + validTrace
		eval := EvaluateSimulatorQAGates(in)
		if eval.Gates["SHORTCUT_RUN_COMPLETED"] != "FAIL_RUNTIME" {
			t.Fatalf("wrong output string must fail runtime gate, got %s", eval.Gates["SHORTCUT_RUN_COMPLETED"])
		}
	})

	t.Run("negative-empty-editor-cannot-claim-complete", func(t *testing.T) {
		in := baseValidInput()
		in.EditorCoverage.DetectedAnchors = nil
		in.EditorCoverage.CaptureStatus = "EDITOR_CAPTURE_COMPLETE"
		eval := EvaluateSimulatorQAGates(in)
		if eval.Gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE" {
			t.Fatalf("empty anchors must never allow EDITOR_CAPTURE_COMPLETE")
		}
		if len(eval.Violations) == 0 {
			t.Fatalf("expected violation when claiming EDITOR_CAPTURE_COMPLETE with empty anchors")
		}
	})

	t.Run("negative-blocked-signing-prevents-downstream-pass", func(t *testing.T) {
		in := baseValidInput()
		in.HostSignSucceeded = false
		in.SignedArtifactPresent = false
		eval := EvaluateSimulatorQAGates(in)
		for _, gate := range []string{
			"COMPILER_ARTIFACT_IMPORTED",
			"SHORTCUT_EXISTS_IN_LIBRARY",
			"SHORTCUT_RUN_ATTEMPTED",
			"SHORTCUT_RUN_COMPLETED",
			"TRACE_CAPTURED",
		} {
			if eval.Gates[gate] != "BLOCKED_SIGNING" {
				t.Fatalf("expected %s=BLOCKED_SIGNING when signing unavailable, got %s", gate, eval.Gates[gate])
			}
		}
		if eval.ReleaseReadiness != "PARTIAL_READY_WITH_BLOCKED_SIGNING" {
			t.Fatalf("expected PARTIAL_READY_WITH_BLOCKED_SIGNING, got %s", eval.ReleaseReadiness)
		}
	})
}

func TestP14_PrivacySanitizerAndBinaryCanaryScanner(t *testing.T) {
	secrets := []string{
		"ghp_FAKE_SECRET_TOKEN_1234567890",
		"-----BEGIN PRIVATE KEY-----",
		"Bearer eyJhbGciOiJIUzI1NiJ9.fake",
		"cherri-shortcut-audit-20261110/035.plist",
		"david@example.com",
		"/Users/runner/Library/Shortcuts/secret.plist",
		"D9A9954E-1234-5678-9ABC-DEF012345678",
	}
	for _, s := range secrets {
		got := SanitizeQAUILabel(s)
		if got != "[REDACTED_SECRET_OR_PII]" {
			t.Fatalf("SanitizeQAUILabel(%q) = %q, expected [REDACTED_SECRET_OR_PII]", s, got)
		}
		findings := ScanArtifactBytesForPrivacyViolations([]byte("PNG\x89\x50\x4e\x47... " + s))
		if len(findings) == 0 {
			t.Fatalf("ScanArtifactBytesForPrivacyViolations failed to detect %q inside binary stream", s)
		}
	}

	nonAllowlisted := SanitizeQAUILabel("My Personal Notes Shortcut Title")
	if !strings.HasPrefix(nonAllowlisted, "[REDACTED_NON_ALLOWLISTED_LEN_") {
		t.Fatalf("expected non-allowlisted string to be redacted with length, got %q", nonAllowlisted)
	}

	allowlisted := []string{
		"CHERRI_QA_RESULT:ok:count=2:mode=v2",
		"ANCHOR_CARD_01_TOP_HEADER",
		"All Shortcuts",
		"Add Shortcut",
		"בדיקת קיצור דרך סינתטי בעברית - iPadOS 27",
	}
	for _, a := range allowlisted {
		if got := SanitizeQAUILabel(a); got != a {
			t.Fatalf("allowlisted string %q was unexpectedly modified to %q", a, got)
		}
	}
}

func TestP14_WorkflowOrchestratorAndSwiftHarnessRegressionLocks(t *testing.T) {
	shBytes, err := os.ReadFile(filepath.Join("scripts", "ios27_shortcuts_simulator_qa.sh"))
	if err != nil {
		t.Fatalf("read orchestrator script: %v", err)
	}
	sh := string(shBytes)

	// Bug #1 regression lock: must never hardcode BLOCKED_IMPORT inside the library=PASS runtime branch
	if strings.Contains(sh, "GATE_SHORTCUT_RUN_COMPLETED=\"BLOCKED_IMPORT\"\n      GATE_TRACE_CAPTURED=\"BLOCKED_IMPORT\"") {
		t.Fatalf("orchestrator script still contains P10R13 hardcoded BLOCKED_IMPORT overwrite inside library=PASS branch")
	}
	for _, req := range []string{
		"REQUIRE_RUNTIME",
		"is_structural_inline_trace_text",
		"is_structural_collector_envelope_text",
		"EXEC_RUN_NONCE",
		"READY_SIMULATOR_VERIFIED",
		"PARTIAL_READY_WITH_BLOCKED_SIGNING",
	} {
		if !strings.Contains(sh, req) {
			t.Fatalf("orchestrator script missing required P10R14 token %q", req)
		}
	}

	wfBytes, err := os.ReadFile(filepath.Join(".github", "workflows", "cherri-ios27-shortcuts-simulator-qa.yml"))
	if err != nil {
		t.Fatalf("read workflow: %v", err)
	}
	wf := string(wfBytes)
	for _, req := range []string{
		"agent/cherri-p10r14-*",
		"require_runtime:",
		"steps.privacy_gate.outputs.passed == 'true'",
	} {
		if !strings.Contains(wf, req) {
			t.Fatalf("workflow missing required P10R14 token %q", req)
		}
	}

	uiBytes, err := os.ReadFile(filepath.Join("tools", "shortcuts-simulator-qa", "XcodeProject", "UITests", "CherriSimulatorQAUITests.swift"))
	if err != nil {
		t.Fatalf("read CherriSimulatorQAUITests.swift: %v", err)
	}
	ui := string(uiBytes)
	if strings.Contains(ui, "shortcuts.searchFields.firstMatch.exists ||") {
		t.Fatalf("CherriSimulatorQAUITests.swift still contains P10R13 false-positive editorOpened check on generic searchFields")
	}
	for _, req := range []string{
		"evaluateEditorState",
		"editorIdentityVerified",
		"shortcutExistsInLibrary",
		"[REDACTED_SECRET_OR_PII]",
		"[REDACTED_NON_ALLOWLISTED_LEN_",
		"test03_SanitizerRedactsSecretsAndNonAllowlistedStrings",
	} {
		if !strings.Contains(ui, req) {
			t.Fatalf("CherriSimulatorQAUITests.swift missing required P10R14 token %q", req)
		}
	}

	if _, err := os.Stat(filepath.Join("tools", "shortcuts-simulator-qa", "XcodeProject", "ReceiverApp", "Info.plist")); err != nil {
		t.Fatalf("ReceiverApp/Info.plist must exist in repository: %v", err)
	}
}

