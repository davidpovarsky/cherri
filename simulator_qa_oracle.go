package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// SimulatorQACallbackEvent mirrors the JSON structure persisted by ReceiverApp in Documents/callback-events.json.
type SimulatorQACallbackEvent struct {
	SchemaVersion      string `json:"schemaVersion"`
	RunID              string `json:"runId"`
	FixtureID          string `json:"fixtureId,omitempty"`
	ExpectedShortcut   string `json:"expectedShortcutName,omitempty"`
	EventSource        string `json:"eventSource,omitempty"`
	Status             string `json:"status"`
	Host               string `json:"host"`
	ResultSummary      string `json:"resultSummary,omitempty"`
	TracePayload       string `json:"tracePayload,omitempty"`
	ErrorMessage       string `json:"errorMessage,omitempty"`
	ReceiverSelfTest   bool   `json:"receiverSelfTest"`
	TimestampISO8601   string `json:"timestampISO8601"`
}

// SimulatorQAEditorCoverage mirrors editor-action-coverage.json produced by CherriSimulatorQAUITests.
type SimulatorQAEditorCoverage struct {
	SchemaVersion              string   `json:"schemaVersion"`
	ShortcutLaunched           bool     `json:"shortcutLaunched"`
	ShortcutForegroundVerified bool     `json:"shortcutForegroundVerified"`
	ExpectedShortcutName       string   `json:"expectedShortcutName,omitempty"`
	ShortcutExistsInLibrary    bool     `json:"shortcutExistsInLibrary"`
	EditorOpened               bool     `json:"editorOpened"`
	EditorIdentityVerified     bool     `json:"editorIdentityVerified"`
	ScrollContainerFound       bool     `json:"scrollContainerFound"`
	ScrollContainerType        string   `json:"scrollContainerType,omitempty"`
	TotalPagesCaptured         int      `json:"totalPagesCaptured"`
	MaxEditorPages             int      `json:"maxEditorPages"`
	CaptureStatus              string   `json:"captureStatus"`
	DetectedActionCards        []string `json:"detectedActionCards"`
	DetectedAnchors            []string `json:"detectedAnchors"`
	BottomReachedEvidence      string   `json:"bottomReachedEvidence"`
}

// SimulatorQAOracleInput supplies all observed facts for deterministic 14-gate evaluation.
type SimulatorQAOracleInput struct {
	RunnerAvailable          bool
	SimulatorBooted          bool
	ShortcutsAppInstalled    bool
	FileImportAttempted      bool
	HostSignSucceeded        bool
	SignedArtifactPresent    bool
	SignedArtifactValidAEA1  bool
	ImportConfirmedInUI      bool
	ExpectedShortcutName     string
	ExpectedRunID            string
	ExpectedTraceRunID       string
	ExpectedSemanticOutput   string
	RequireAnchorPrefixTop   string
	RequireAnchorPrefixBot   string
	EditorCoverage           SimulatorQAEditorCoverage
	CallbackEvents           []SimulatorQACallbackEvent
	PhysicalDeviceTested     bool
}

// SimulatorQAOracleEvaluation holds the 14 progressive gates and detailed audit diagnostics.
type SimulatorQAOracleEvaluation struct {
	Gates                            map[string]string `json:"gates"`
	ReleaseReadiness                 string            `json:"releaseReadiness"`
	ReceiverSelfCallbackVerified     bool              `json:"receiverSelfCallbackVerified"`
	CompiledShortcutCallbackVerified bool              `json:"compiledShortcutCallbackVerified"`
	CompiledShortcutRuntimeExecuted  bool              `json:"compiledShortcutRuntimeExecuted"`
	ValidatedTraceRecords            []QATraceRecord   `json:"validatedTraceRecords,omitempty"`
	Violations                       []string          `json:"violations,omitempty"`
}

// EvaluateSimulatorQAGates enforces the 14-gate state machine and strict prerequisite invariants.
// No downstream gate may be marked PASS if its upstream prerequisite is blocked or failed.
func EvaluateSimulatorQAGates(in SimulatorQAOracleInput) SimulatorQAOracleEvaluation {
	gates := map[string]string{
		"RUNNER_AVAILABLE":           "NOT_RUN",
		"SIMULATOR_BOOTED":           "NOT_RUN",
		"SHORTCUTS_APP_INSTALLED":    "NOT_RUN",
		"SHORTCUTS_LAUNCHED":         "NOT_RUN",
		"EDITOR_OPENED":              "NOT_RUN",
		"FILE_IMPORT_ATTEMPTED":      "NOT_RUN",
		"COMPILER_ARTIFACT_IMPORTED": "NOT_RUN",
		"SHORTCUT_EXISTS_IN_LIBRARY": "NOT_RUN",
		"ACTION_EDITOR_CAPTURED":     "NOT_RUN",
		"SHORTCUT_RUN_ATTEMPTED":     "NOT_RUN",
		"SHORTCUT_RUN_COMPLETED":     "NOT_RUN",
		"CALLBACK_CAPTURED":          "NOT_RUN",
		"TRACE_CAPTURED":             "NOT_RUN",
		"PHYSICAL_DEVICE_TESTED":     "BLOCKED_DEVICE",
	}
	var violations []string

	if in.PhysicalDeviceTested {
		gates["PHYSICAL_DEVICE_TESTED"] = "PASS"
	}

	if !in.RunnerAvailable {
		gates["RUNNER_AVAILABLE"] = "BLOCKED_RUNNER"
		gates["SIMULATOR_BOOTED"] = "BLOCKED_RUNNER"
		gates["SHORTCUTS_APP_INSTALLED"] = "BLOCKED_RUNNER"
		return SimulatorQAOracleEvaluation{
			Gates:            gates,
			ReleaseReadiness: "BLOCKED_RUNNER",
			Violations:       violations,
		}
	}
	gates["RUNNER_AVAILABLE"] = "PASS"

	if !in.SimulatorBooted {
		gates["SIMULATOR_BOOTED"] = "BLOCKED_RUNNER"
		gates["SHORTCUTS_APP_INSTALLED"] = "BLOCKED_RUNNER"
		return SimulatorQAOracleEvaluation{
			Gates:            gates,
			ReleaseReadiness: "BLOCKED_RUNNER",
			Violations:       violations,
		}
	}
	gates["SIMULATOR_BOOTED"] = "PASS"

	if !in.ShortcutsAppInstalled {
		gates["SHORTCUTS_APP_INSTALLED"] = "BLOCKED_SIMULATOR_APP"
		gates["SHORTCUTS_LAUNCHED"] = "BLOCKED_SIMULATOR_APP"
		gates["EDITOR_OPENED"] = "BLOCKED_SIMULATOR_APP"
		gates["ACTION_EDITOR_CAPTURED"] = "BLOCKED_SIMULATOR_APP"
		return SimulatorQAOracleEvaluation{
			Gates:            gates,
			ReleaseReadiness: "BLOCKED_SIMULATOR_APP",
			Violations:       violations,
		}
	}
	gates["SHORTCUTS_APP_INSTALLED"] = "PASS"

	if in.EditorCoverage.ShortcutLaunched && in.EditorCoverage.ShortcutForegroundVerified {
		gates["SHORTCUTS_LAUNCHED"] = "PASS"
	} else {
		gates["SHORTCUTS_LAUNCHED"] = "FAIL_UI"
	}

	if in.EditorCoverage.EditorOpened && gates["SHORTCUTS_LAUNCHED"] == "PASS" {
		gates["EDITOR_OPENED"] = "PASS"
	} else {
		gates["EDITOR_OPENED"] = "FAIL_UI"
	}

	if in.FileImportAttempted {
		gates["FILE_IMPORT_ATTEMPTED"] = "PASS"
	}

	// Evaluate signing & import prerequisite chain
	signingAvailable := (in.HostSignSucceeded || in.SignedArtifactPresent) && in.SignedArtifactValidAEA1
	if !signingAvailable {
		gates["COMPILER_ARTIFACT_IMPORTED"] = "BLOCKED_SIGNING"
		gates["SHORTCUT_EXISTS_IN_LIBRARY"] = "BLOCKED_SIGNING"
	} else if !in.ImportConfirmedInUI {
		gates["COMPILER_ARTIFACT_IMPORTED"] = "BLOCKED_IMPORT"
		gates["SHORTCUT_EXISTS_IN_LIBRARY"] = "BLOCKED_IMPORT"
	} else {
		gates["COMPILER_ARTIFACT_IMPORTED"] = "PASS"
		if in.EditorCoverage.ShortcutExistsInLibrary &&
			(in.ExpectedShortcutName == "" || in.EditorCoverage.ExpectedShortcutName == in.ExpectedShortcutName) {
			gates["SHORTCUT_EXISTS_IN_LIBRARY"] = "PASS"
		} else {
			gates["SHORTCUT_EXISTS_IN_LIBRARY"] = "FAIL_UI"
			violations = append(violations, "import confirmed in UI but expected shortcut was not found in Shortcuts library")
		}
	}

	// Evaluate ACTION_EDITOR_CAPTURED:
	// Can ONLY be EDITOR_CAPTURE_COMPLETE if:
	// 1. EDITOR_OPENED == PASS
	// 2. SHORTCUT_EXISTS_IN_LIBRARY == PASS
	// 3. EditorIdentityVerified == true
	// 4. DetectedAnchors has top & bottom anchors in order and DetectedActionCards is non-empty.
	if gates["EDITOR_OPENED"] != "PASS" {
		gates["ACTION_EDITOR_CAPTURED"] = "FAIL_UI"
	} else {
		anchorsOK := verifyOrderedAnchors(in.EditorCoverage.DetectedAnchors, in.RequireAnchorPrefixTop, in.RequireAnchorPrefixBot)
		cardsOK := len(in.EditorCoverage.DetectedActionCards) > 0
		if gates["SHORTCUT_EXISTS_IN_LIBRARY"] == "PASS" &&
			in.EditorCoverage.EditorIdentityVerified &&
			anchorsOK &&
			cardsOK &&
			in.EditorCoverage.CaptureStatus == "EDITOR_CAPTURE_COMPLETE" {
			gates["ACTION_EDITOR_CAPTURED"] = "EDITOR_CAPTURE_COMPLETE"
		} else {
			if in.EditorCoverage.CaptureStatus == "EDITOR_CAPTURE_COMPLETE" {
				violations = append(violations, "editor coverage claimed EDITOR_CAPTURE_COMPLETE without library presence, verified editor identity, or required ordered anchors")
			}
			gates["ACTION_EDITOR_CAPTURED"] = "PARTIAL_EDITOR_CAPTURE"
		}
	}

	// Evaluate callback events
	selfCallbackOK := false
	for _, ev := range in.CallbackEvents {
		if ev.ReceiverSelfTest && ev.Status == "SUCCESS" && strings.HasPrefix(ev.ResultSummary, "CHERRI_QA_RECEIVER_OK") {
			selfCallbackOK = true
			break
		}
	}

	// Runtime execution gates require SHORTCUT_EXISTS_IN_LIBRARY == PASS
	var validatedTrace []QATraceRecord
	compiledCallbackOK := false
	if gates["SHORTCUT_EXISTS_IN_LIBRARY"] != "PASS" {
		upstreamBlocker := gates["SHORTCUT_EXISTS_IN_LIBRARY"]
		if upstreamBlocker != "BLOCKED_SIGNING" && upstreamBlocker != "BLOCKED_IMPORT" {
			upstreamBlocker = "BLOCKED_IMPORT"
		}
		gates["SHORTCUT_RUN_ATTEMPTED"] = upstreamBlocker
		gates["SHORTCUT_RUN_COMPLETED"] = upstreamBlocker
		gates["TRACE_CAPTURED"] = upstreamBlocker
	} else {
		gates["SHORTCUT_RUN_ATTEMPTED"] = "PASS"
		runErr, traceRecords, traceErr := validateCorrelatedRuntimeCallback(
			in.CallbackEvents,
			in.ExpectedRunID,
			in.ExpectedTraceRunID,
			in.ExpectedSemanticOutput,
		)
		if runErr != nil {
			gates["SHORTCUT_RUN_COMPLETED"] = "FAIL_RUNTIME"
			gates["TRACE_CAPTURED"] = "FAIL_RUNTIME"
			violations = append(violations, fmt.Sprintf("runtime callback validation failed: %v", runErr))
		} else {
			gates["SHORTCUT_RUN_COMPLETED"] = "PASS"
			compiledCallbackOK = true
			if traceErr != nil {
				gates["TRACE_CAPTURED"] = "FAIL_RUNTIME"
				violations = append(violations, fmt.Sprintf("runtime trace validation failed: %v", traceErr))
			} else {
				gates["TRACE_CAPTURED"] = "PASS"
				validatedTrace = traceRecords
			}
		}
	}

	if selfCallbackOK || compiledCallbackOK {
		gates["CALLBACK_CAPTURED"] = "PASS"
	} else {
		gates["CALLBACK_CAPTURED"] = "FAIL_UI"
	}

	readiness := "PARTIAL_READY"
	if gates["SHORTCUT_RUN_COMPLETED"] == "PASS" &&
		gates["TRACE_CAPTURED"] == "PASS" &&
		gates["ACTION_EDITOR_CAPTURED"] == "EDITOR_CAPTURE_COMPLETE" {
		readiness = "READY_SIMULATOR_VERIFIED"
	} else if gates["COMPILER_ARTIFACT_IMPORTED"] == "BLOCKED_SIGNING" {
		readiness = "PARTIAL_READY_WITH_BLOCKED_SIGNING"
	} else if strings.HasPrefix(gates["SHORTCUT_RUN_COMPLETED"], "FAIL") || strings.HasPrefix(gates["TRACE_CAPTURED"], "FAIL") {
		readiness = "FAIL_RUNTIME_OR_TRACE"
	}

	return SimulatorQAOracleEvaluation{
		Gates:                            gates,
		ReleaseReadiness:                 readiness,
		ReceiverSelfCallbackVerified:     selfCallbackOK,
		CompiledShortcutCallbackVerified: compiledCallbackOK,
		CompiledShortcutRuntimeExecuted:  gates["SHORTCUT_RUN_COMPLETED"] == "PASS",
		ValidatedTraceRecords:            validatedTrace,
		Violations:                       violations,
	}
}

func verifyOrderedAnchors(anchors []string, topPrefix, bottomPrefix string) bool {
	if len(anchors) == 0 {
		return false
	}
	if topPrefix == "" && bottomPrefix == "" {
		return len(anchors) > 0
	}
	topIdx := -1
	botIdx := -1
	for idx, a := range anchors {
		if topPrefix != "" && topIdx == -1 && strings.Contains(a, topPrefix) {
			topIdx = idx
		}
		if bottomPrefix != "" && strings.Contains(a, bottomPrefix) {
			botIdx = idx
		}
	}
	if topPrefix != "" && topIdx == -1 {
		return false
	}
	if bottomPrefix != "" && botIdx == -1 {
		return false
	}
	if topPrefix != "" && bottomPrefix != "" && topIdx > botIdx {
		return false
	}
	return true
}

func validateCorrelatedRuntimeCallback(
	events []SimulatorQACallbackEvent,
	expectedRunID string,
	expectedTraceRunID string,
	expectedSemanticOutput string,
) (error, []QATraceRecord, error) {
	if strings.TrimSpace(expectedRunID) == "" {
		return fmt.Errorf("expectedRunID must not be empty"), nil, nil
	}

	var matched []SimulatorQACallbackEvent
	for _, ev := range events {
		if ev.ReceiverSelfTest {
			// Self-test callbacks can NEVER satisfy compiled shortcut runtime completion
			if ev.RunID == expectedRunID {
				return fmt.Errorf("callback for runId %q is marked receiverSelfTest=true", expectedRunID), nil, nil
			}
			continue
		}
		if ev.RunID == expectedRunID {
			matched = append(matched, ev)
		}
	}

	if len(matched) == 0 {
		return fmt.Errorf("no non-self-test callback event found matching runId %q", expectedRunID), nil, nil
	}
	if len(matched) > 1 {
		return fmt.Errorf("duplicate callback events (%d) found for runId %q; replay or collision rejected", len(matched), expectedRunID), nil, nil
	}

	ev := matched[0]
	if ev.Status != "SUCCESS" {
		return fmt.Errorf("callback status for runId %q is %q (error=%q), expected SUCCESS", expectedRunID, ev.Status, ev.ErrorMessage), nil, nil
	}

	combinedOutput := ev.ResultSummary
	if ev.TracePayload != "" {
		combinedOutput = combinedOutput + "\n" + ev.TracePayload
	}
	if expectedSemanticOutput != "" && !strings.Contains(combinedOutput, expectedSemanticOutput) {
		return fmt.Errorf("callback result %q does not contain expected semantic output %q", ev.ResultSummary, expectedSemanticOutput), nil, nil
	}

	traceSource := ev.TracePayload
	if strings.TrimSpace(traceSource) == "" {
		traceSource = extractNDJSONTraceFromOutput(ev.ResultSummary)
	}
	traceRunID := expectedTraceRunID
	if traceRunID == "" {
		traceRunID = expectedRunID
	}
	records, traceErr := ParseAndValidateQATracePayload(traceSource, traceRunID, true)
	return nil, records, traceErr
}

func extractNDJSONTraceFromOutput(raw string) string {
	var traceLines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") &&
			(strings.Contains(trimmed, `"kind":"COLLECTOR_EVENT"`) || strings.Contains(trimmed, `"kind":"INLINE_DEBUG_ACTION"`)) {
			traceLines = append(traceLines, trimmed)
		}
	}
	return strings.Join(traceLines, "\n")
}

var (
	reForbiddenSecretOrCorpus = regexp.MustCompile(`(?i)(cherri-shortcut-audit-20261110|\b[0-3][0-9]\.plist\b|BEGIN [A-Z ]*PRIVATE KEY|ghp_[A-Za-z0-9_]+|github_pat_[A-Za-z0-9_]+|sk-[A-Za-z0-9]{16,}|Bearer\s+[A-Za-z0-9._\-]+|[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}|/Users/[A-Za-z0-9._\-]+|C:\\Users\\[A-Za-z0-9._\-]+)`)
	reFullUUID                = regexp.MustCompile(`\b[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}\b`)
)

var allowlistedUIPrefixes = []string{
	"CHERRI_QA_",
	"ANCHOR_",
	"qa.",
	"qa-functional",
	"qa-scroll-editor",
	"qa-rtl-hebrew",
	"Cherri QA",
	"===CHERRI_QA_TRACE_NDJSON_V14===",
	"ElementType_",
	"Cell:",
	"StaticText:",
	"Button:",
}

var allowlistedUIExactStrings = map[string]bool{
	"Shortcuts":                     true,
	"All Shortcuts":                 true,
	"Library":                       true,
	"Gallery":                       true,
	"Automation":                    true,
	"Search":                        true,
	"Search Actions":                true,
	"New Shortcut":                  true,
	"Create Shortcut":               true,
	"Add Shortcut":                  true,
	"+ Add Shortcut":                true,
	"Add Action":                    true,
	"Add":                           true,
	"Done":                          true,
	"Cancel":                        true,
	"Select":                        true,
	"More":                          true,
	"Share":                         true,
	"Duplicate":                     true,
	"Delete":                        true,
	"Rename":                        true,
	"Details":                       true,
	"Info":                          true,
	"Play":                          true,
	"Run":                           true,
	"Run Shortcut":                  true,
	"Stop":                          true,
	"Allow":                         true,
	"Always Allow":                  true,
	"Allow Once":                    true,
	"Don't Allow":                   true,
	"OK":                            true,
	"Continue":                      true,
	"Get Started":                   true,
	"Not Now":                       true,
	"Close":                         true,
	"Dismiss":                       true,
	"Report":                        true,
	"Set Up Shortcut":               true,
	"Replace":                       true,
	"About This Shortcut":           true,
	"Can Run When Locked":           true,
	"No Shortcuts":                  true,
	"Toolbar":                       true,
	"BackButton":                    true,
	"main.button.newshortcut":       true,
	"all_shortcuts_library":         true,
	"ellipsis.circle.fill":          true,
	"plus.circle.fill":              true,
	"magnifyingglass":               true,
	"Dictate":                       true,
	"Comment":                       true,
	"Text":                          true,
	"Number":                        true,
	"Dictionary":                    true,
	"Get Dictionary Value":          true,
	"If":                            true,
	"Otherwise":                     true,
	"End If":                        true,
	"Repeat":                        true,
	"End Repeat":                    true,
	"Stop and Output":               true,
	"Set Variable":                  true,
	"Add to Variable":               true,
	"Combine Text":                  true,
	"Get Dates from Input":          true,
	"Quick Look":                    true,
	"Show Alert":                    true,
	"Show Result":                   true,
	"Wait":                          true,
	"Nothing":                       true,
	"המשך":                          true,
	"אישור":                         true,
	"צור קיצור דרך":                 true,
	"הוסף קיצור דרך":                true,
	"בדיקת קיצור דרך סינתטי בעברית - iPadOS 27": true,
}

// SanitizeQAUILabel enforces strict secret/PII redaction and synthetic/UI chrome allowlisting.
func SanitizeQAUILabel(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if reForbiddenSecretOrCorpus.MatchString(trimmed) || reFullUUID.MatchString(trimmed) {
		return "[REDACTED_SECRET_OR_PII]"
	}
	if allowlistedUIExactStrings[trimmed] {
		return truncateRunes(trimmed, 256)
	}
	for _, p := range allowlistedUIPrefixes {
		if strings.HasPrefix(trimmed, p) {
			return truncateRunes(trimmed, 256)
		}
	}
	if strings.HasPrefix(trimmed, "Vertical scroll bar") || strings.HasPrefix(trimmed, "Horizontal scroll bar") {
		return truncateRunes(trimmed, 256)
	}
	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var obj map[string]any
		if json.Unmarshal([]byte(trimmed), &obj) == nil {
			if kind, ok := obj["kind"].(string); ok && (kind == "INLINE_DEBUG_ACTION" || kind == "COLLECTOR_EVENT") {
				return truncateRunes(trimmed, 4096)
			}
		}
	}
	return fmt.Sprintf("[REDACTED_NON_ALLOWLISTED_LEN_%d]", len([]rune(trimmed)))
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max])
}

// ScanArtifactBytesForPrivacyViolations inspects text or binary (e.g. PNG/xcresult) bytes for forbidden secrets or corpus markers.
func ScanArtifactBytesForPrivacyViolations(data []byte) []string {
	var findings []string
	if m := reForbiddenSecretOrCorpus.Find(data); m != nil {
		findings = append(findings, fmt.Sprintf("forbidden pattern %q", string(m)))
	}
	if m := reFullUUID.Find(data); m != nil {
		findings = append(findings, fmt.Sprintf("unmasked UUID/UDID %q", string(m)))
	}
	if bytes.Contains(data, []byte("AppleID")) || bytes.Contains(data, []byte("iCloudToken")) {
		findings = append(findings, "forbidden account token marker")
	}
	return findings
}
