package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var nativeEncodingAssertionKeys = []string{
	"required_expect_nonzero", "exact_corruption_rejected", "expected_rejection_passed",
	"per_form_outcomes_preserved", "restored_required_set_passed", "apple_engine_executed",
}

func nativeEncodingTestID(id string) string {
	if id != "CI01" && id != "RP47" {
		return ""
	}
	return "tools/shortcut-runtime:required-encoding-" + strings.ToLower(id)
}

func applyNativeEncodingMapping(id string, testIDs *[]string, assertions map[string]string) bool {
	testID := nativeEncodingTestID(id)
	if testID == "" {
		return false
	}
	*testIDs = []string{testID}
	for _, key := range nativeEncodingAssertionKeys {
		assertions[key] = "true"
	}
	return true
}

func (r *Runner) runNativeEncodingRequirement() {
	r.recordUnexecutedRequirement("CI01", "verification", "Encoding EXPECT intentionally broken in test", []string{"negative-ci"})
	result := r.results["CI01"]
	result.Detail = "Requires the source-bound macOS WorkflowKit REQUIRED encoding gate: correct set, one isolated corrupted EXPECT with exact nonzero diagnostic, restored set. Local process fixtures do not execute Apple code."
	r.results["CI01"] = result
}

// The producer runs actual /usr/bin/osascript with a pinned prelude. Intake must
// retain its process and per-form evidence, not accept an unrelated local smoke.
func validateNativeEncodingRecord(id string, record EvidenceRecord) error {
	expectedID := nativeEncodingTestID(id)
	if expectedID == "" {
		return nil
	}
	fail := func(detail string) error {
		return fmt.Errorf("required encoding record %s for %s: %s", record.EvidenceID, id, detail)
	}
	if record.TestID != expectedID || record.Tier != "negative-ci" {
		return fail("only the dedicated negative-ci WorkflowKit producer is authorized")
	}
	for _, key := range nativeEncodingAssertionKeys {
		if record.Assertions[key] != "true" {
			return fail("missing true assertion " + key)
		}
	}
	artifacts := make(map[string]EvidenceArtifact)
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		if _, duplicate := artifacts[name]; duplicate {
			return fail("duplicate artifact basename")
		}
		if len(artifact.SHA256) != 64 || artifact.RunID == "" {
			return fail("every actual phase artifact requires its digest and CI run")
		}
		artifacts[name] = artifact
	}
	summaryArtifact, ok := artifacts["required-encoding-gate.json"]
	if !ok {
		return fail("missing actual gate summary")
	}
	data, err := os.ReadFile(summaryArtifact.Path)
	if err != nil {
		return fail("actual gate summary is unavailable")
	}
	var summary struct {
		SchemaVersion           int    `json:"schemaVersion"`
		Status                  string `json:"status"`
		ImplementationSHA       string `json:"implementationSha"`
		RunID                   string `json:"runId"`
		AppleEngineExecuted     bool   `json:"appleEngineExecuted"`
		MockEngineAllowed       *bool  `json:"mockEngineAllowed"`
		ShortcutRuntimeExecuted *bool  `json:"shortcutRuntimeExecuted"`
		InputBytesUnchanged     bool   `json:"inputBytesUnchanged"`
		RequiredCases           int    `json:"requiredCases"`
		ExpectedRejections      int    `json:"expectedRejections"`
		NegativeControlExit     int    `json:"negativeControlExit"`
		Phases                  []struct {
			Name                       string `json:"phase"`
			ExitCode                   *int   `json:"exitCode"`
			NegativeDiagnosticObserved bool   `json:"negativeDiagnosticObserved"`
			ReportSHA256               string `json:"reportSha256"`
			StdoutSHA256               string `json:"stdoutSha256"`
			StderrSHA256               string `json:"stderrSha256"`
		} `json:"phases"`
	}
	if json.Unmarshal(data, &summary) != nil || summary.SchemaVersion != 1 || summary.Status != "PASS" ||
		!summary.AppleEngineExecuted || summary.MockEngineAllowed == nil || *summary.MockEngineAllowed ||
		summary.ShortcutRuntimeExecuted == nil || *summary.ShortcutRuntimeExecuted || !summary.InputBytesUnchanged ||
		summary.RequiredCases != 23 || summary.ExpectedRejections != 1 || summary.NegativeControlExit != 1 ||
		summary.ImplementationSHA != record.ImplementationSHA || summary.RunID != summaryArtifact.RunID ||
		len(summary.Phases) != 3 {
		return fail("gate summary does not establish the exact real-engine triplet")
	}
	for i, name := range []string{"positive", "corrupt", "restored"} {
		phase := summary.Phases[i]
		expectedExit := 0
		if i == 1 {
			expectedExit = 1
		}
		if phase.Name != name || phase.ExitCode == nil || *phase.ExitCode != expectedExit || (i == 1 && !phase.NegativeDiagnosticObserved) {
			return fail("phase order, exit, or corrupt-EXPECT diagnostic is invalid")
		}
		for suffix, expectedDigest := range map[string]string{
			".json": phase.ReportSHA256, ".stdout.log": phase.StdoutSHA256, ".stderr.log": phase.StderrSHA256,
		} {
			artifact, present := artifacts[name+suffix]
			if !present || artifact.RunID != summary.RunID || len(expectedDigest) != 64 || artifact.SHA256 != expectedDigest {
				return fail("phase report or process output is absent or not bound to the summary")
			}
		}
		if artifact, present := artifacts[name+".process.json"]; !present || artifact.RunID != summary.RunID {
			return fail("actual phase process receipt is missing")
		}
	}
	return reconcileNativeEncodingArtifacts(record, artifacts, data)
}
