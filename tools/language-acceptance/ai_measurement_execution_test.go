package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func p15ClearAIMeasurementEnvironment(t *testing.T) {
	t.Helper()
	for _, name := range aiMeasurementEnvironment {
		t.Setenv(name, "")
	}
}

func p15AIMeasurementBool(value bool) *bool { return &value }
func p15AIMeasurementInt(value int) *int    { return &value }

// This is deliberately synthetic shape-test data. It is never exported as an
// EvidenceRecord, never invokes a provider, and cannot pass original intake.
func p15AIMeasurementSummaryFixture(t *testing.T) (aiMeasurementSummary, aiMeasurementConfig, string, string) {
	t.Helper()
	var directory = t.TempDir()
	var source = strings.Repeat("a", 40)
	var config = aiMeasurementConfig{
		Root: filepath.Join(directory, "source"), Bundle: filepath.Join(directory, "bundle"),
		StoryID: "1220000000000001", TextSHA256: strings.Repeat("1", 64),
		Before: aiMeasurementTrust{Path: filepath.Join(directory, "before.original.json"), SHA256: strings.Repeat("2", 64), ObservedUTC: "2026-11-21T00:01:00Z"},
		After:  aiMeasurementTrust{Path: filepath.Join(directory, "after.original.json"), SHA256: strings.Repeat("3", 64), ObservedUTC: "2026-11-21T00:11:00Z"},
	}
	var report = aiMeasurementSummary{
		Schema: "cherri-ai-evaluation-intake/v1", Status: "VERIFIED_MEASUREMENT", ImplementationSHA: source,
		ManifestSHA256: strings.Repeat("4", 64), SuiteSHA256: strings.Repeat("5", 64),
		Compiler:                   aiMeasurementCompiler{SourceSHA: strings.Repeat("b", 40), BinarySHA: strings.Repeat("6", 64)},
		TrustedAsanaResponseSHA256: config.After.SHA256,
		Anchor:                     aiMeasurementAnchor{StoryID: config.StoryID, TaskID: aiMeasurementTask, TextSHA256: config.TextSHA256, CreatedUTC: "2026-11-21T00:00:00Z"},
		AuthenticationBoundary:     aiMeasurementTrustBoundary, QualityMeasured: true, ExpectedOutcomes: 12,
		ProvenanceScope: aiMeasurementScope, UnseenPatternProved: p15AIMeasurementBool(false),
		AppleRuntimeProved: p15AIMeasurementBool(false), FormalAcceptanceGranted: p15AIMeasurementBool(false),
		MeasuredResults:   map[string]aiMeasurementScore{"legacy": {Passed: 6, Total: 6}, "v2": {Passed: 6, Total: 6}},
		ExecutionInterval: aiMeasurementInterval{StartedUTC: "2026-11-21T00:02:00Z", FinishedUTC: "2026-11-21T00:10:00Z"},
	}
	for _, task := range aiMeasurementTaskIDs {
		for _, arm := range []string{"legacy", "v2"} {
			index := len(report.Records) + 1
			report.Records = append(report.Records, aiMeasurementOutcome{
				TaskID: task, Arm: arm, RequestID: fmt.Sprintf("%064x", index),
				ProviderReceiptSHA256:   fmt.Sprintf("%064x", 100+index),
				ProviderRequestIDSHA256: fmt.Sprintf("%064x", 200+index),
				ValidatorReceiptSHA256:  fmt.Sprintf("%064x", 300+index),
				SourceSHA256:            fmt.Sprintf("%064x", 400+index), Status: "MEASURED",
				Usage:  &aiMeasurementUsage{Input: p15AIMeasurementInt(100), Output: p15AIMeasurementInt(20)},
				Checks: []aiMeasurementCheck{{Name: "synthetic-shape-control", Passed: p15AIMeasurementBool(true)}},
				Passed: p15AIMeasurementBool(true),
			})
		}
	}
	return report, config, source, "2026-11-21T00:12:00Z"
}

func p15AIMeasurementRaw(t *testing.T, value any) []byte {
	t.Helper()
	var raw, err = json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func TestAIMeasurementTwelveOutcomeShapeAndCausalNegatives(t *testing.T) {
	var baseline, config, source, observed = p15AIMeasurementSummaryFixture(t)
	var raw = p15AIMeasurementRaw(t, baseline)
	if _, err := validateAIMeasurementSummary(raw, source, config, config.After, observed); err != nil {
		t.Fatal("synthetic shape control failed; this is not model evidence", err)
	}
	var faults = []struct {
		name, cause string
		change      func(*aiMeasurementSummary, *aiMeasurementConfig)
	}{
		{"exit_zero_incomplete", "all twelve measured outcomes", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Status = "VERIFIED_INCOMPLETE_OUTCOMES"
			r.QualityMeasured = false
		}},
		{"incomplete_claims_quality", "all twelve measured outcomes", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Status = "VERIFIED_INCOMPLETE_OUTCOMES" }},
		{"missing_outcome", "all twelve measured outcomes", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records = r.Records[:11] }},
		{"duplicate_task_arm", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Records[1].TaskID = r.Records[0].TaskID
			r.Records[1].Arm = r.Records[0].Arm
		}},
		{"duplicate_request", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[1].RequestID = r.Records[0].RequestID }},
		{"duplicate_provider_request", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Records[1].ProviderRequestIDSHA256 = r.Records[0].ProviderRequestIDSHA256
		}},
		{"foreign_task", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].TaskID = "unrelated-task" }},
		{"foreign_arm", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].Arm = "fake" }},
		{"stale_source", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.ImplementationSHA = strings.Repeat("f", 40) }},
		{"unbound_trusted_response", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.TrustedAsanaResponseSHA256 = strings.Repeat("f", 64)
		}},
		{"wrong_anchor_story", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Anchor.StoryID = "1220000000000002" }},
		{"wrong_anchor_task", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Anchor.TaskID = "1220000000000002" }},
		{"changed_anchor_body", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Anchor.TextSHA256 = strings.Repeat("f", 64) }},
		{"self_asserted_authentication", "source, compiler or independent Asana", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.AuthenticationBoundary = "JSON says authenticated"
		}},
		{"unseen_pattern_claim", "cannot grant unseen-pattern", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.UnseenPatternProved = p15AIMeasurementBool(true)
		}},
		{"apple_claim", "cannot grant unseen-pattern", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.AppleRuntimeProved = p15AIMeasurementBool(true)
		}},
		{"global_closure_claim", "cannot grant unseen-pattern", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.FormalAcceptanceGranted = p15AIMeasurementBool(true)
		}},
		{"missing_scope_witness", "cannot grant unseen-pattern", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.UnseenPatternProved = nil }},
		{"missing_actual_usage", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].Usage.Input = nil }},
		{"excess_output_budget", "identity or actual usage", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Records[0].Usage.Output = p15AIMeasurementInt(2001)
		}},
		{"network_is_not_measured_failure", "transport, protocol or unadjudicated", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].Status = "PROVIDER_ERROR" }},
		{"unsupported_graph_is_incomplete", "transport, protocol or unadjudicated", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].Status = "VALIDATOR_UNADJUDICATED" }},
		{"refusal_cannot_pass", "must remain a measured task failure", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Records[0].Status = "MEASURED_PROVIDER_FAILURE"
			r.Records[0].ProviderStatus = "REFUSED"
			r.Records[0].ValidatorReceiptSHA256 = ""
			r.Records[0].Checks = nil
		}},
		{"missing_check_inventory", "lacks original source or validator checks", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Records[0].Checks = nil }},
		{"contradictory_semantic_pass", "differs from its checks", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.Records[0].Checks[0].Passed = p15AIMeasurementBool(false)
		}},
		{"denominator_shrunk", "six-per-arm denominator", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.MeasuredResults["legacy"] = aiMeasurementScore{Passed: 6, Total: 5}
		}},
		{"inflated_score", "six-per-arm denominator", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.MeasuredResults["legacy"] = aiMeasurementScore{Passed: 7, Total: 6}
		}},
		{"before_read_after_request", "do not bracket", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			c.Before.ObservedUTC = r.ExecutionInterval.StartedUTC
		}},
		{"after_read_before_validator", "do not bracket", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			c.After.ObservedUTC = r.ExecutionInterval.FinishedUTC
		}},
		{"anchor_created_after_before_read", "do not bracket", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { r.Anchor.CreatedUTC = "2026-11-21T00:01:30Z" }},
		{"future_authenticated_read", "do not bracket", func(r *aiMeasurementSummary, c *aiMeasurementConfig) { c.After.ObservedUTC = "2026-11-21T00:13:00Z" }},
		{"reversed_original_interval", "do not bracket", func(r *aiMeasurementSummary, c *aiMeasurementConfig) {
			r.ExecutionInterval.FinishedUTC = "2026-11-21T00:01:30Z"
		}},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			var changed aiMeasurementSummary
			if err := json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			var changedConfig = config
			fault.change(&changed, &changedConfig)
			if _, err := validateAIMeasurementSummary(p15AIMeasurementRaw(t, changed), source, changedConfig, changedConfig.After, observed); err == nil || !strings.Contains(err.Error(), fault.cause) {
				t.Fatalf("specific contrary gate %q did not reject: %v", fault.cause, err)
			}
		})
	}
	for _, status := range []string{"REFUSED", "TRUNCATED", "SEMANTIC_FAILURE"} {
		t.Run("measured_failure_"+strings.ToLower(status), func(t *testing.T) {
			var changed aiMeasurementSummary
			if err := json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			var outcome = &changed.Records[0]
			outcome.Passed = p15AIMeasurementBool(false)
			if status == "SEMANTIC_FAILURE" {
				outcome.Checks[0].Passed = p15AIMeasurementBool(false)
			} else {
				outcome.Status, outcome.ProviderStatus = "MEASURED_PROVIDER_FAILURE", status
				outcome.ValidatorReceiptSHA256, outcome.Checks = "", nil
			}
			changed.MeasuredResults["legacy"] = aiMeasurementScore{Passed: 5, Total: 6}
			if _, err := validateAIMeasurementSummary(p15AIMeasurementRaw(t, changed), source, config, config.After, observed); err != nil {
				t.Fatal("valid measured task failure was removed from denominator", err)
			}
		})
	}
	for _, fault := range []string{"boolean_integer", "duplicate_json_key", "unknown_json_field"} {
		t.Run(fault, func(t *testing.T) {
			var changed = string(raw)
			switch fault {
			case "boolean_integer":
				changed = strings.Replace(changed, "\"input_tokens\":100", "\"input_tokens\":true", 1)
			case "duplicate_json_key":
				changed = "{\"status\":\"VERIFIED_MEASUREMENT\"," + changed[1:]
			case "unknown_json_field":
				changed = "{\"caller_verified\":true," + changed[1:]
			}
			if _, err := validateAIMeasurementSummary([]byte(changed), source, config, config.After, observed); err == nil || !strings.Contains(err.Error(), "summary JSON") {
				t.Fatal("ambiguous or caller-invented summary was accepted", err)
			}
		})
	}
}

func TestAIMeasurementAbsentAndRejectedConfigurationRemainDistinct(t *testing.T) {
	p15ClearAIMeasurementEnvironment(t)
	var absent = NewRunner()
	absent.runEvaluation()
	if absent.results["AI01"].Status != "AI_EVAL_NOT_RUN" || len(absent.records) != 0 {
		t.Fatal("absent configuration minted actual model evidence")
	}
	t.Setenv("CHERRI_AI_EVALUATION_BUNDLE", filepath.Join(t.TempDir(), "missing-bundle"))
	var invalid = NewRunner()
	invalid.runEvaluation()
	if invalid.results["AI01"].Status != "FAILED" || len(invalid.records) != 1 ||
		invalid.records[0].Result != "rejected" || invalid.records[0].ExitCode != -1 ||
		len(invalid.records[0].Artifacts) != 0 || invalid.results["AI01"].Passed {
		t.Fatal("configured rejection was relabeled as unavailable endpoint or successful execution")
	}
	if err := validateAIMeasurementRecord("AI01", invalid.records[0]); err == nil {
		t.Fatal("configured preflight failure received evaluation credit")
	}
}

func TestAIMeasurementStandaloneRecordsCannotGrantFormalCredit(t *testing.T) {
	p15ClearAIMeasurementEnvironment(t)
	for _, id := range []string{"AI01", "RP50"} {
		t.Run(id, func(t *testing.T) {
			var record = EvidenceRecord{EvidenceID: "ev-ai-measurement-" + id, Requirements: []string{id}, Tier: "external-eval",
				ImplementationSHA: strings.Repeat("a", 40), TestID: aiMeasurementTestID(id), Result: "passed",
				Artifacts: make([]EvidenceArtifact, 8)}
			if err := validateAIMeasurementRecord(id, record); err == nil || !strings.Contains(err.Error(), "independent operator configuration") {
				t.Fatal("standalone PASS reached formal credit without independent pins", err)
			}
			record.TestID = "tools/language-acceptance:runEvaluation"
			if err := validateAIMeasurementRecord(id, record); err == nil || !strings.Contains(err.Error(), "actual complete source-bound") {
				t.Fatal("old generic evaluation record bypassed actual intake", err)
			}
			record.TestID = aiMeasurementTestID(id)
			record.Requirements = []string{"AI01", "RP50"}
			if err := validateAIMeasurementRecord(id, record); err == nil {
				t.Fatal("one linked-ID record supplied two obligations")
			}
			record.Requirements = []string{id}
			record.Tier = "ios-runtime"
			if err := validateAIMeasurementRecord(id, record); err == nil {
				t.Fatal("local AI receipt claimed Apple runtime")
			}
			record.Tier = "external-eval"
			if err := validateAIMeasurementRecord("BRG32", record); err == nil {
				t.Fatal("measurement replaced the distinct BRG32 harness/obligation contract")
			}
		})
	}
	if !reflect.DeepEqual(deriveAcceptanceTestIDs("AI01", "evaluation"), []string{aiMeasurementTestID("AI01")}) {
		t.Fatal("AI01 still authorizes an unbound generic producer")
	}
}

func TestAIMeasurementCanonicalIntakeRejectsMissingOriginals(t *testing.T) {
	p15ClearAIMeasurementEnvironment(t)
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	var source, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean {
		t.Fatal("requires clean actual source", stateErr)
	}
	var directory = t.TempDir()
	var bundle = filepath.Join(directory, "bundle")
	if err := os.Mkdir(bundle, 0700); err != nil {
		t.Fatal(err)
	}
	var before = filepath.Join(directory, "before.original.json")
	var after = filepath.Join(directory, "after.original.json")
	var control = []byte("{}\n")
	for _, path := range []string{before, after} {
		if err := os.WriteFile(path, control, 0600); err != nil {
			t.Fatal(err)
		}
	}
	var now = time.Now().UTC()
	var config = aiMeasurementConfig{Root: root, Bundle: bundle, StoryID: "1220000000000001", TextSHA256: strings.Repeat("a", 64),
		Before: aiMeasurementTrust{Path: before, SHA256: editorSHA(control), ObservedUTC: now.Add(-2 * time.Minute).Format(time.RFC3339Nano)},
		After:  aiMeasurementTrust{Path: after, SHA256: editorSHA(control), ObservedUTC: now.Add(-time.Minute).Format(time.RFC3339Nano)}}
	if err := aiMeasurementConfigShape(config); err != nil {
		t.Fatal(err)
	}
	var output = filepath.Join(directory, "actual-intake")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	var result = aiMeasurementRun(source, config, output)
	if result.Err == nil || !strings.Contains(result.Err.Error(), "AI canonical intake before process failed") ||
		len(result.Artifacts) != 3 || result.Summary.QualityMeasured {
		t.Fatal("missing original bundle did not fail actual canonical process", result.Err)
	}
	var receipt editorProcessReceipt
	var processRaw, readErr = os.ReadFile(filepath.Join(output, "ai-measurement-before-process.json"))
	if readErr != nil || decodeEditorJSON(processRaw, &receipt) != nil || receipt.ExitCode == nil || *receipt.ExitCode != 1 ||
		receipt.TimedOut == nil || *receipt.TimedOut || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		receipt.SourceCommit != source || len(receipt.SourceFiles) != len(aiMeasurementSources) {
		t.Fatal("actual canonical rejection lacks its exact-source receipt", readErr)
	}
	var stderr, stderrErr = os.ReadFile(filepath.Join(output, "ai-measurement-before-stderr.log"))
	if stderrErr != nil || strings.TrimSpace(string(stderr)) != "EVALUATION_INTAKE_REJECTED" {
		t.Fatal("expected original-only rejection was not observed", stderrErr, string(stderr))
	}
	if _, err := os.Stat(filepath.Join(output, "ai-measurement-before-intake.json")); !os.IsNotExist(err) {
		t.Fatal("failed intake wrote a stale success report")
	}
	t.Logf("Actual canonical Python intake rejected missing originals at %s; %d immutable source witnesses; no provider invocation or quality evidence", source, len(receipt.SourceFiles))
}

func p15MutateAIBoundHarnessOriginal(t *testing.T, record *EvidenceRecord, fault string) {
	t.Helper()
	var directory = t.TempDir()
	var data = map[string][]byte{}
	for _, artifact := range record.Artifacts {
		raw, err := os.ReadFile(artifact.Path)
		if err != nil {
			t.Fatal(err)
		}
		data[filepath.Base(artifact.Path)] = raw
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["ai-harness-process.json"], &receipt); err != nil {
		t.Fatal(err)
	}
	switch fault {
	case "wrong_harness_source_digest":
		if len(receipt.SourceFiles) == 0 {
			t.Fatal("actual harness source witnesses missing")
		}
		receipt.SourceFiles[0].SHA256 = strings.Repeat("f", 64)
	case "missing_harness_python_child":
		var raw = data["ai-harness-go.jsonl"]
		var needle = []byte("test_runner.EvaluationHarnessTests.test_hung_adapter_times_out")
		if !bytes.Contains(raw, needle) {
			t.Fatal("actual Python inventory marker absent")
		}
		raw = bytes.ReplaceAll(raw, needle, []byte("test_runner.EvaluationHarnessTests.unexecuted_control"))
		data["ai-harness-go.jsonl"] = raw
		receipt.RawSHA256, receipt.RawBytes = editorSHA(raw), len(raw)
	default:
		t.Fatal("unknown actual harness mutation", fault)
	}
	data["ai-harness-process.json"] = p15AIMeasurementRaw(t, receipt)
	for index := range record.Artifacts {
		name := filepath.Base(record.Artifacts[index].Path)
		raw := data[name]
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		record.Artifacts[index].Path, record.Artifacts[index].SHA256 = path, editorSHA(raw)
	}
}

func TestAIMeasurementTrustPinsRejectBundleAndChronologySubstitution(t *testing.T) {
	var _, baseline, _, _ = p15AIMeasurementSummaryFixture(t)
	if err := aiMeasurementConfigShape(baseline); err != nil {
		t.Fatal("synthetic configuration shape failed", err)
	}
	var faults = []struct {
		name   string
		change func(*aiMeasurementConfig)
	}{
		{"before_inside_bundle", func(c *aiMeasurementConfig) { c.Before.Path = filepath.Join(c.Bundle, "asana.json") }},
		{"after_inside_bundle", func(c *aiMeasurementConfig) { c.After.Path = filepath.Join(c.Bundle, "asana.json") }},
		{"same_retained_path", func(c *aiMeasurementConfig) { c.After.Path = c.Before.Path }},
		{"missing_independent_pin", func(c *aiMeasurementConfig) { c.Before.SHA256 = "" }},
		{"relative_input", func(c *aiMeasurementConfig) { c.Bundle = "caller-bundle" }},
		{"missing_story_identity", func(c *aiMeasurementConfig) { c.StoryID = "" }},
		{"reversed_rereads", func(c *aiMeasurementConfig) { c.After.ObservedUTC = c.Before.ObservedUTC }},
	}
	for _, fault := range faults {
		t.Run(fault.name, func(t *testing.T) {
			var changed = baseline
			fault.change(&changed)
			if err := aiMeasurementConfigShape(changed); err == nil {
				t.Fatal("operator pin boundary accepted substitution")
			}
		})
	}
	t.Run("bounded_file_hash_input", func(t *testing.T) {
		var path = filepath.Join(t.TempDir(), "original.json")
		if err := os.WriteFile(path, []byte("unchanged"), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := aiMeasurementReadRegular(path, 3); err == nil {
			t.Fatal("oversized retained input bypassed the actual bounded read")
		}
		var raw, err = aiMeasurementReadRegular(path, 9)
		if err != nil || string(raw) != "unchanged" {
			t.Fatal("bounded regular control", err)
		}
	})
}

func TestAIMeasurementOriginalAnnouncementCannotContradictSummary(t *testing.T) {
	var report, _, _, _ = p15AIMeasurementSummaryFixture(t)
	var actualShape = []byte(`{"status":"VERIFIED_MEASUREMENT","ai_quality_measured":true}`)
	if err := validateAIMeasurementAnnouncement(actualShape, report); err != nil {
		t.Fatal("synthetic announcement shape control failed", err)
	}
	for _, fault := range []struct{ name, raw string }{
		{"incomplete", `{"status":"VERIFIED_INCOMPLETE_OUTCOMES","ai_quality_measured":false}`},
		{"false_quality", `{"status":"VERIFIED_MEASUREMENT","ai_quality_measured":false}`},
		{"missing_quality", `{"status":"VERIFIED_MEASUREMENT"}`},
		{"string_quality", `{"status":"VERIFIED_MEASUREMENT","ai_quality_measured":"true"}`},
		{"caller_claim", `{"status":"VERIFIED_MEASUREMENT","ai_quality_measured":true,"caller_verified":true}`},
		{"duplicate_status", `{"status":"VERIFIED_MEASUREMENT","status":"VERIFIED_MEASUREMENT","ai_quality_measured":true}`},
	} {
		t.Run(fault.name, func(t *testing.T) {
			if err := validateAIMeasurementAnnouncement([]byte(fault.raw), report); err == nil {
				t.Fatal("original stdout contradiction bypassed intake")
			}
		})
	}
}
