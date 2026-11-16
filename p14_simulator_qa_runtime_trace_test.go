package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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

	for _, token := range []string{
		"sign \"$UNSIGNED_FILE\"",
		"--hubsign",
		"builds/signed/${EXPECTED_SHORTCUT_NAME}.shortcut",
		"UNSIGNED_FILE\" = \"$SIGNED_FILE\"",
		"signing-provenance.json",
		"request-signed-import.txt",
		"request-shortcut-run.txt",
		"plist-vs-editor-comparison.json",
	} {
		if !strings.Contains(sh, token) {
			t.Fatalf("ios27_shortcuts_simulator_qa.sh missing required HubSign/handshake token %q", token)
		}
	}
	for _, req := range []string{
		"importConsentButtonTapped",
		"importSheetSnapshotCaptured",
		"runtimeTriggerDispatched",
		"runtimeCallbackObservedInUI",
	} {
		if !strings.Contains(ui, req) {
			t.Fatalf("CherriSimulatorQAUITests.swift missing required P10R14 handshake/runtime field %q", req)
		}
	}
}

func TestP14_ExactFileHubSignAndProvenanceContract(t *testing.T) {
	tmpDir := t.TempDir()
	exeSuffix := ""
	if runtime.GOOS == "windows" {
		exeSuffix = ".exe"
	}
	cherriBin := filepath.Join(tmpDir, "cherri"+exeSuffix)

	buildCmd := exec.Command("go", "build", "-o", cherriBin, ".")
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build cherri binary: %v\n%s", err, string(out))
	}

	compileFixture := func(srcRel string, legacy bool, baseOut string) (string, []byte) {
		args := []string{srcRel, "--skip-sign", "--output=" + baseOut, "--no-ansi"}
		if legacy {
			args = append(args, "--legacy")
		}
		cmd := exec.Command(cherriBin, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("failed to compile %s (legacy=%v): %v\n%s", srcRel, legacy, err, string(out))
		}
		if _, statErr := os.Stat(baseOut); statErr != nil {
			t.Fatalf("expected compiled unsigned artifact at %s: %v", baseOut, statErr)
		}
		data, err := os.ReadFile(baseOut)
		if err != nil {
			t.Fatalf("read unsigned output %s: %v", baseOut, err)
		}
		if len(data) >= 4 && string(data[:4]) == "AEA1" {
			t.Fatalf("compiled --skip-sign file %s unexpectedly starts with AEA1", baseOut)
		}
		return baseOut, data
	}

	v2Src := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", "qa-functional-v2.cherri")
	legacySrc := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", "qa-functional-legacy.cherri")
	v2UnsignedPath, v2UnsignedBytes := compileFixture(v2Src, false, filepath.Join(tmpDir, "qa-functional-v2.shortcut"))
	legacyUnsignedPath, legacyUnsignedBytes := compileFixture(legacySrc, true, filepath.Join(tmpDir, "qa-functional-legacy.shortcut"))

	validAEA1 := append([]byte("AEA1"), bytes.Repeat([]byte{0x42, 0x7A, 0x19, 0xC3}, 24)...)
	validSum := sha256.Sum256(validAEA1)
	validSHA := hex.EncodeToString(validSum[:])

	var expectedUpload []byte
	mode := "valid"
	serverHits := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverHits++
		if r.URL.Path != "/api/v1/sign" {
			http.Error(w, "unexpected path", http.StatusNotFound)
			return
		}
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			http.Error(w, "bad multipart: "+err.Error(), http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer f.Close()
		uploaded, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read upload failed", http.StatusInternalServerError)
			return
		}
		if expectedUpload != nil && !bytes.Equal(uploaded, expectedUpload) {
			http.Error(w, fmt.Sprintf("exact-byte mismatch: uploaded %d bytes != expected %d bytes", len(uploaded), len(expectedUpload)), http.StatusBadRequest)
			return
		}

		switch mode {
		case "valid":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"success": true,
				"file": map[string]any{
					"format":         "aea1",
					"content_base64": base64.StdEncoding.EncodeToString(validAEA1),
					"signed_sha256":  validSHA,
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "xml":
			w.Header().Set("Content-Type", "application/xml")
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict/></dict></plist>`))
		case "html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<!doctype html><html><body>502 Bad Gateway</body></html>`))
		case "json-error":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"success":false,"error":{"code":"SIGN_REJECTED","message":"synthetic rejection"}}`))
		case "sha-mismatch":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"success": true,
				"file": map[string]any{
					"format":         "aea1",
					"content_base64": base64.StdEncoding.EncodeToString(validAEA1),
					"signed_sha256":  strings.Repeat("0", 64),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "non-aea1":
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]any{
				"success": true,
				"file": map[string]any{
					"format":         "bplist",
					"content_base64": base64.StdEncoding.EncodeToString([]byte("bplist00_not_signed_aea1_payload")),
				},
			}
			_ = json.NewEncoder(w).Encode(resp)
		case "empty":
			w.WriteHeader(http.StatusOK)
		case "http-503":
			w.Header().Set("Content-Type", "text/plain")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("service temporarily unavailable"))
		case "slow":
			time.Sleep(250 * time.Millisecond)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	signEndpoint := srv.URL + "/api/v1/sign?response=json"

	// 1. Positive exact-byte HubSign tests for BOTH V2 and Legacy compiled outputs.
	for _, tc := range []struct {
		name          string
		unsignedPath  string
		unsignedBytes []byte
		sourcePath    string
	}{
		{name: "v2", unsignedPath: v2UnsignedPath, unsignedBytes: v2UnsignedBytes, sourcePath: v2Src},
		{name: "legacy", unsignedPath: legacyUnsignedPath, unsignedBytes: legacyUnsignedBytes, sourcePath: legacySrc},
	} {
		t.Run("positive_"+tc.name, func(t *testing.T) {
			mode = "valid"
			expectedUpload = tc.unsignedBytes
			signedOut := filepath.Join(tmpDir, tc.name+"-signed.shortcut")
			manifestOut := filepath.Join(tmpDir, tc.name+"-manifest.json")

			cmd := exec.Command(cherriBin,
				"sign", tc.unsignedPath,
				"--output="+signedOut,
				"--signing-server="+signEndpoint,
				"--source-file="+tc.sourcePath,
				"--compiler-sha=deadbeef1234",
				"--manifest-out="+manifestOut,
				"--no-ansi",
			)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("cherri sign (%s) failed: %v\n%s", tc.name, err, string(out))
			}

			signedBytes, err := os.ReadFile(signedOut)
			if err != nil {
				t.Fatalf("read signed output: %v", err)
			}
			if !bytes.Equal(signedBytes, validAEA1) {
				t.Fatalf("signed bytes mismatch: got %d bytes, want %d", len(signedBytes), len(validAEA1))
			}

			manifestBytes, err := os.ReadFile(manifestOut)
			if err != nil {
				t.Fatalf("read manifest: %v", err)
			}
			if strings.Contains(string(manifestBytes), srv.URL) {
				t.Fatalf("provenance manifest leaked signing server URL %q: %s", srv.URL, string(manifestBytes))
			}
			if strings.Contains(string(out), srv.URL) {
				t.Fatalf("CLI stdout leaked signing server URL %q: %s", srv.URL, string(out))
			}

			var manifest SigningProvenanceManifest
			if err := json.Unmarshal(manifestBytes, &manifest); err != nil {
				t.Fatalf("decode manifest: %v", err)
			}
			srcBytes, _ := os.ReadFile(tc.sourcePath)
			srcSum := fmt.Sprintf("%x", sha256.Sum256(srcBytes))
			unsignedSum := fmt.Sprintf("%x", sha256.Sum256(tc.unsignedBytes))
			if manifest.HeaderMagic != "AEA1" ||
				manifest.SourceSHA256 != srcSum ||
				manifest.UnsignedPlistSHA256 != unsignedSum ||
				manifest.SignedArtifactSHA256 != validSHA ||
				manifest.NativeActionCount == 0 {
				t.Fatalf("unexpected provenance manifest fields: %+v", manifest)
			}
		})
	}

	// 2. Negative fail-closed tests (XML, HTML, JSON error, SHA256 mismatch, non-AEA1, empty, HTTP 503, unreachable, invalid file).
	expectedUpload = v2UnsignedBytes
	for _, negMode := range []string{"xml", "html", "json-error", "sha-mismatch", "non-aea1", "empty", "http-503"} {
		t.Run("negative_cli_"+negMode, func(t *testing.T) {
			mode = negMode
			signedOut := filepath.Join(tmpDir, "fail-"+negMode+".shortcut")
			cmd := exec.Command(cherriBin,
				"sign", v2UnsignedPath,
				"--output="+signedOut,
				"--signing-server="+signEndpoint,
				"--no-ansi",
			)
			if out, err := cmd.CombinedOutput(); err == nil {
				t.Fatalf("expected non-zero exit for server mode %s, got nil (out=%s)", negMode, string(out))
			}
			if _, statErr := os.Stat(signedOut); !os.IsNotExist(statErr) {
				t.Fatalf("signed output %s must not exist on failed signing mode %s", signedOut, negMode)
			}
		})
	}

	t.Run("negative_cli_unreachable", func(t *testing.T) {
		signedOut := filepath.Join(tmpDir, "fail-unreachable.shortcut")
		cmd := exec.Command(cherriBin,
			"sign", v2UnsignedPath,
			"--output="+signedOut,
			"--signing-server=http://127.0.0.1:1/api/v1/sign?response=json",
			"--no-ansi",
		)
		if _, err := cmd.CombinedOutput(); err == nil {
			t.Fatalf("expected failure for unreachable signing endpoint")
		}
		if _, statErr := os.Stat(signedOut); !os.IsNotExist(statErr) {
			t.Fatalf("signed output must not exist on unreachable endpoint failure")
		}
	})

	t.Run("negative_timeout_fail_closed", func(t *testing.T) {
		mode = "slow"
		prevTimeout := signingHTTPTimeout
		signingHTTPTimeout = 80 * time.Millisecond
		defer func() { signingHTTPTimeout = prevTimeout }()

		signedOut := filepath.Join(tmpDir, "fail-timeout.shortcut")
		svc := &SigningService{name: "MockSlowHubSign", url: signEndpoint}
		if _, err := SignUnsignedShortcutFileWithProvenance(v2UnsignedPath, signedOut, v2Src, "testsha", svc); err == nil {
			t.Fatalf("expected timeout error when server sleeps beyond signingHTTPTimeout")
		}
		if _, statErr := os.Stat(signedOut); !os.IsNotExist(statErr) {
			t.Fatalf("timeout failure must not leave signed output file behind")
		}
	})

	t.Run("negative_invalid_inputs_and_privacy_guard", func(t *testing.T) {
		mode = "valid"
		svc := &SigningService{name: "MockHubSign", url: signEndpoint}

		badFile := filepath.Join(tmpDir, "not-a-plist.shortcut")
		_ = os.WriteFile(badFile, []byte("not-a-plist"), 0600)
		if _, err := SignUnsignedShortcutFileWithProvenance(badFile, filepath.Join(tmpDir, "bad-out.shortcut"), "", "", svc); err == nil {
			t.Fatalf("expected invalid plist rejection")
		}

		alreadySigned := filepath.Join(tmpDir, "already-aea1.shortcut")
		_ = os.WriteFile(alreadySigned, validAEA1, 0600)
		if _, err := SignUnsignedShortcutFileWithProvenance(alreadySigned, filepath.Join(tmpDir, "aea1-out.shortcut"), "", "", svc); err == nil {
			t.Fatalf("expected rejection of already-signed AEA1 input")
		}

		hitsBeforePrivacy := serverHits
		privateMarkerPath := filepath.Join(tmpDir, "cherri-shortcut-audit-20261110-sample.shortcut")
		_ = os.WriteFile(privateMarkerPath, v2UnsignedBytes, 0600)
		if _, err := SignUnsignedShortcutFileWithProvenance(privateMarkerPath, filepath.Join(tmpDir, "priv-out.shortcut"), "", "", svc); err == nil {
			t.Fatalf("expected privacy guard rejection for private corpus path")
		}
		if serverHits != hitsBeforePrivacy {
			t.Fatalf("privacy guard must block before contacting signing server (hits before=%d, after=%d)", hitsBeforePrivacy, serverHits)
		}
	})

	t.Run("hubsign_fallback_shortcuty_403_cloudflare_then_routinehub_aea1", func(t *testing.T) {
		shortcutySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=UTF-8")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html lang="en-US"><head><title>Just a moment...</title></head></html>`))
		}))
		defer shortcutySrv.Close()

		var receivedShortcut string
		routineSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var payload map[string]string
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			receivedShortcut = payload["shortcut"]
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(validAEA1)
		}))
		defer routineSrv.Close()

		orig := hubSignCandidatesFunc
		hubSignCandidatesFunc = func() []SigningService {
			return []SigningService{
				{name: "Shortcuty Sign", url: shortcutySrv.URL + "/api/v1/sign?response=json"},
				{name: "RoutineHub HubSign", url: routineSrv.URL + "/sign"},
			}
		}
		defer func() { hubSignCandidatesFunc = orig }()

		signedOut := filepath.Join(tmpDir, "qa-functional-v2-fallback.shortcut")
		manifest, err := SignUnsignedShortcutFileWithProvenance(v2UnsignedPath, signedOut, v2Src, "testsha", nil)
		if err != nil {
			t.Fatalf("expected secondary HubSign fallback to succeed when primary hits Cloudflare 403: %v", err)
		}
		if receivedShortcut != string(v2UnsignedBytes) {
			t.Fatalf("secondary HubSign server did not receive exact unsigned plist bytes")
		}
		if manifest.ServiceName != "RoutineHub HubSign" || manifest.SigningMethod != "HUBSIGN_REMOTE_SERVICE" {
			t.Fatalf("unexpected fallback manifest metadata: %+v", manifest)
		}
		if manifest.HeaderMagic != "AEA1" || manifest.SignedArtifactSHA256 != validSHA {
			t.Fatalf("unexpected fallback signed artifact digest/magic: %+v", manifest)
		}
	})

	t.Run("hubsign_fallback_all_fail_closed_no_residual_file", func(t *testing.T) {
		shortcutySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusForbidden)
			_, _ = w.Write([]byte(`<!DOCTYPE html><html><title>Just a moment...</title></html>`))
		}))
		defer shortcutySrv.Close()

		routineSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?><plist version="1.0"><dict/></plist>`))
		}))
		defer routineSrv.Close()

		orig := hubSignCandidatesFunc
		hubSignCandidatesFunc = func() []SigningService {
			return []SigningService{
				{name: "Shortcuty Sign", url: shortcutySrv.URL + "/api/v1/sign?response=json"},
				{name: "RoutineHub HubSign", url: routineSrv.URL + "/sign"},
			}
		}
		defer func() { hubSignCandidatesFunc = orig }()

		signedOut := filepath.Join(tmpDir, "qa-functional-v2-both-fail.shortcut")
		_, err := SignUnsignedShortcutFileWithProvenance(v2UnsignedPath, signedOut, v2Src, "testsha", nil)
		if err == nil {
			t.Fatalf("expected error when both Shortcuty Sign and RoutineHub HubSign fail")
		}
		if !strings.Contains(err.Error(), "Shortcuty Sign") || !strings.Contains(err.Error(), "RoutineHub HubSign") {
			t.Fatalf("expected combined error mentioning both HubSign services, got: %v", err)
		}
		if _, statErr := os.Stat(signedOut); !os.IsNotExist(statErr) {
			t.Fatalf("both-fail signing must not leave residual output file behind")
		}
	})
}

