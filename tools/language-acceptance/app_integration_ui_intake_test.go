package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// Synthetic intake controls, never an XCTest result or an exported RP40 proof.
func rp40UIIntakeModelForTest(t *testing.T) *rp40IntakeModel {
	t.Helper()
	model := rp40IntakeModelForTest(t)
	fixture, err := os.ReadFile(resolveRepoPath(appIntegrationUIFixture))
	if err != nil {
		t.Fatal(err)
	}
	model.record.EvidenceID = "synthetic-rp40-ui-intake-control"
	model.record.Tier = "ios-ui"
	model.record.TestID = appIntegrationUITestID
	model.record.FixtureSHA256 = nativeEncodingDigest(fixture)
	model.record.Assertions = appIntegrationUIAssertions()
	model.record.Detail = appIntegrationUIScope
	method := strings.Split(appIntegrationUITestID, "/")[1]
	body := ""
	for _, stage := range appIntegrationUIStages {
		body += appIntegrationUIStagePrefix + stage + "\n"
	}
	model.raw = []byte("Test Case '-[CherriUITests.CherriRP40UITests " + method + "]' started.\n" +
		body + appIntegrationUICheckpoint + "\n" +
		"Test Case '-[CherriUITests.CherriRP40UITests " + method + "]' passed (0.123 seconds).\n")
	delete(model.diagnostic, "appIntegrationEvidence")
	model.diagnostic["testResults"] = []any{
		map[string]any{"testId": appIntegrationUITestID, "observedStatus": "passed"},
	}
	model.diagnostic["appIntegrationUIEvidence"] = map[string]any{
		"status": "VERIFIED_APP_UI", "testId": appIntegrationUITestID,
		"scope": appIntegrationUIScope, "fixtureSHA256": model.record.FixtureSHA256,
		"appIntegrationTierGranted": false,
	}
	model.seal(t)
	return model
}

func TestRP40AppUIIndependentTierAndCombinedOriginal(t *testing.T) {
	model := rp40UIIntakeModelForTest(t)
	if err := validateAppIntegrationRecord("RP40", model.record, model.manifest); err != nil {
		t.Fatal(err)
	}
	selected := rp40SelectedContracts(t)
	uiOnly, err := ValidateEvidenceManifest(model.manifest, selected, "local")
	if err == nil || uiOnly == nil || uiOnly.PassedCases != 0 || uiOnly.FailedCases != 1 ||
		!reflect.DeepEqual(uiOnly.Results[0].ExecutedLevels, []string{"ios-ui"}) {
		t.Fatalf("UI-only evidence acquired the missing hosted-app tier: %+v %v", uiOnly, err)
	}
	app := rp40IntakeModelForTest(t)
	// Both records now refer to the same combined synthetic original and its
	// coherent diagnostic. This is an intake control, not an authenticated run.
	model.raw = append(append([]byte{}, app.raw...), model.raw...)
	model.diagnostic["testResults"] = []any{
		map[string]any{"testId": appIntegrationTestID, "observedStatus": "passed"},
		map[string]any{"testId": appIntegrationUITestID, "observedStatus": "passed"},
	}
	model.diagnostic["appIntegrationEvidence"] = app.diagnostic["appIntegrationEvidence"]
	model.seal(t)
	appRecord := app.record
	appRecord.Artifacts = append([]EvidenceArtifact{}, model.record.Artifacts...)
	model.manifest.Records = []EvidenceRecord{appRecord, model.record}
	for _, record := range model.manifest.Records {
		if err := validateAppIntegrationRecord("RP40", record, model.manifest); err != nil {
			t.Fatal(err)
		}
	}
	joined, err := ValidateEvidenceManifest(model.manifest, selected, "local")
	if err != nil || joined == nil || joined.TotalCases != 1 || joined.PassedCases != 1 ||
		joined.FailedCases != 0 || joined.SkippedCases != 0 || joined.ClosureComplete ||
		!reflect.DeepEqual(joined.Results[0].ExecutedLevels, []string{"app-integration", "ios-ui"}) {
		t.Fatalf("coherent synthetic app/UI tier join failed: %+v %v", joined, err)
	}
	// The old completion-only case must never stand in for the new joined UI.
	borrowed := model.record
	borrowed.TestID = "CherriUITests/testE01CompletionDisplayAndInsertionUI"
	if err := validateAppIntegrationRecord("RP40", borrowed, model.manifest); err == nil {
		t.Fatal("borrowed E01 completion obtained RP40 UI scope")
	}
}

func TestRP40AppUIOriginalIntakeCorruptions(t *testing.T) {
	cases := []struct{ name, want string }{
		{"borrowed_hosted_app", "RP40_RECORD"},
		{"fake_hosted_tier", "RP40_RECORD"},
		{"assertion", "RP40_UI_RECORD"}, {"scope", "RP40_UI_RECORD"},
		{"command", "RP40_UI_RECORD"}, {"record_source", "RP40_UI_RECORD"},
		{"fixture", "RP40_UI_FIXTURE"}, {"missing_artifact", "RP40_UI_ARTIFACTS"},
		{"mixed_run", "RP40_UI_ARTIFACTS"}, {"digest", "RP40_UI_ARTIFACT_DIGEST"},
		{"missing_run", "RP40_UI_RUN"}, {"failed_run", "RP40_UI_RUN"},
		{"checkout", "RP40_UI_RUN"}, {"event", "RP40_UI_RUN"},
		{"workflow", "RP40_UI_RUN"}, {"duplicate_run", "RP40_UI_RUN"},
		{"process_exit", "RP40_UI_DIAGNOSTIC"}, {"diagnostic_only", "RP40_UI_DIAGNOSTIC"},
		{"diagnostic_fixture", "RP40_UI_DIAGNOSTIC"}, {"hosted_credit", "RP40_UI_DIAGNOSTIC"},
		{"missing_outcome", "RP40_UI_DIAGNOSTIC"}, {"failed_outcome", "RP40_UI_DIAGNOSTIC"},
		{"duplicate_outcome", "RP40_UI_DIAGNOSTIC"},
		{"missing_stage", "RP40_UI_STAGE"}, {"duplicate_stage", "RP40_UI_STAGE"},
		{"unknown_stage", "RP40_UI_STAGE"}, {"stage_outside", "RP40_UI_STAGE"},
		{"reordered_stage", "RP40_UI_STAGE"}, {"stage_suffix", "RP40_UI_STAGE"},
		{"missing_marker", "RP40_CHECKPOINT"}, {"duplicate_marker", "RP40_CHECKPOINT"},
		{"marker_outside", "RP40_CHECKPOINT"}, {"marker_suffix", "RP40_CHECKPOINT"},
		{"failed_case", "RP40_CASE"}, {"duplicate_case", "RP40_CASE"},
		{"incomplete_case", "RP40_CASE"}, {"wrong_target", "RP40_UI_STAGE"},
		{"rejected_marker", "RP40_CHECKPOINT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := rp40UIIntakeModelForTest(t)
			if err := validateAppIntegrationRecord("RP40", model.record, model.manifest); err != nil {
				t.Fatal("unchanged causal base rejected:", err)
			}
			repin := false
			switch tc.name {
			case "borrowed_hosted_app":
				model.record.TestID = appIntegrationTestID
			case "fake_hosted_tier":
				model.record.Tier = "app-integration"
			case "assertion":
				delete(model.record.Assertions, "semantic_rename_exact_ui")
			case "scope":
				model.record.Detail = "All app and native requirements closed"
			case "command":
				model.record.Command = []string{"xcodebuild", "build"}
			case "record_source":
				model.record.ImplementationSHA = strings.Repeat("b", 40)
			case "fixture":
				model.record.FixtureSHA256 = strings.Repeat("0", 64)
			case "missing_artifact":
				model.record.Artifacts = model.record.Artifacts[:1]
			case "mixed_run":
				model.record.Artifacts[1].RunID = "9999"
			case "digest":
				model.record.Artifacts[0].SHA256 = strings.Repeat("0", 64)
			case "missing_run":
				model.manifest.Runs = nil
			case "failed_run":
				model.manifest.Runs[0].Conclusion = "failure"
			case "checkout":
				model.manifest.Runs[0].CheckoutSHA = strings.Repeat("b", 40)
			case "event":
				model.manifest.Runs[0].Event = "schedule"
			case "workflow":
				model.manifest.Runs[0].WorkflowName = "Build & Test"
			case "duplicate_run":
				model.manifest.Runs = append(model.manifest.Runs, model.manifest.Runs[0])
			case "process_exit":
				model.diagnostic["xcodebuildExitCode"] = 65
				repin = true
			case "diagnostic_only":
				model.diagnostic["diagnosticOnly"] = true
				repin = true
			case "diagnostic_fixture":
				model.diagnostic["appIntegrationUIEvidence"].(map[string]any)["fixtureSHA256"] = strings.Repeat("0", 64)
				repin = true
			case "hosted_credit":
				model.diagnostic["appIntegrationUIEvidence"].(map[string]any)["appIntegrationTierGranted"] = true
				repin = true
			case "missing_outcome":
				model.diagnostic["testResults"] = []any{map[string]any{"testId": appIntegrationTestID, "observedStatus": "passed"}}
				repin = true
			case "failed_outcome":
				model.diagnostic["testResults"].([]any)[0].(map[string]any)["observedStatus"] = "failed"
				repin = true
			case "duplicate_outcome":
				rows := model.diagnostic["testResults"].([]any)
				model.diagnostic["testResults"] = append(rows, rows[0])
				repin = true
			case "missing_stage", "duplicate_stage", "unknown_stage", "stage_outside", "reordered_stage", "stage_suffix":
				stage := appIntegrationUIStagePrefix + appIntegrationUIStages[0] + "\n"
				text := string(model.raw)
				switch tc.name {
				case "missing_stage":
					text = strings.Replace(text, stage, "", 1)
				case "duplicate_stage":
					text = strings.Replace(text, stage, stage+stage, 1)
				case "unknown_stage":
					text = strings.Replace(text, stage, appIntegrationUIStagePrefix+"UNKNOWN\n", 1)
				case "stage_outside":
					text = stage + strings.Replace(text, stage, "", 1)
				case "reordered_stage":
					second := appIntegrationUIStagePrefix + appIntegrationUIStages[1] + "\n"
					text = strings.Replace(text, stage+second, second+stage, 1)
				case "stage_suffix":
					text = strings.Replace(text, stage, strings.TrimSuffix(stage, "\n")+" EXTRA\n", 1)
				}
				model.raw = []byte(text)
				repin = true
			case "missing_marker":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationUICheckpoint+"\n", ""))
				repin = true
			case "duplicate_marker":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationUICheckpoint, appIntegrationUICheckpoint+"\n"+appIntegrationUICheckpoint))
				repin = true
			case "marker_outside":
				model.raw = []byte(appIntegrationUICheckpoint + "\n" + strings.ReplaceAll(string(model.raw), appIntegrationUICheckpoint+"\n", ""))
				repin = true
			case "marker_suffix":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationUICheckpoint, appIntegrationUICheckpoint+" EXTRA"))
				repin = true
			case "failed_case":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), "passed (", "failed ("))
				repin = true
			case "duplicate_case":
				model.raw = append(model.raw, model.raw...)
				repin = true
			case "incomplete_case":
				model.raw = []byte(strings.Split(string(model.raw), "Test Case '-[CherriUITests.CherriRP40UITests "+strings.Split(appIntegrationUITestID, "/")[1]+"]' passed")[0])
				repin = true
			case "wrong_target":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), "CherriUITests.CherriRP40UITests", "CherriTests.CherriRP40UITests"))
				repin = true
			case "rejected_marker":
				model.raw = append(model.raw, []byte(appIntegrationUIRejected+"INVALID_STAGE\n")...)
				repin = true
			default:
				t.Fatal("unknown control")
			}
			if repin {
				model.seal(t)
			}
			if err := validateAppIntegrationRecord("RP40", model.record, model.manifest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("causal %s guard absent: %v", tc.want, err)
			}
		})
	}
}

func TestRP40AppUIRealCollectorTierJoin(t *testing.T) {
	for _, tc := range []struct {
		name     string
		combined bool
		exit     string
		want     int
	}{
		{"ui_only", false, "0", 1},
		{"combined_app_ui", true, "0", 2},
		{"failed_original", true, "65", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			model := rp40UIIntakeModelForTest(t)
			if tc.combined {
				app := rp40IntakeModelForTest(t)
				model.raw = append(append([]byte{}, app.raw...), model.raw...)
			}
			rawPath := filepath.Join(model.directory, "raw.log")
			exitPath := filepath.Join(model.directory, "xcode.exit")
			output := filepath.Join(model.directory, "collector")
			if err := os.WriteFile(rawPath, model.raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(exitPath, []byte(tc.exit), 0600); err != nil {
				t.Fatal(err)
			}
			root, err := filepath.Abs(resolveRepoRoot())
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, "python3", "-B", filepath.Join(root, "ios/scripts/collect_ios_test_evidence.py"),
				rawPath, output, model.record.ImplementationSHA, "1234", exitPath)
			cmd.Dir = root
			raw, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("actual public collector failed: %v %s", err, raw)
			}
			evidence, err := os.ReadFile(filepath.Join(output, "ios-test-evidence.json"))
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Records []EvidenceRecord }
			if err := json.Unmarshal(evidence, &result); err != nil {
				t.Fatal(err)
			}
			if len(result.Records) != tc.want {
				t.Fatalf("collector tiers=%d, want=%d", len(result.Records), tc.want)
			}
			for _, record := range result.Records {
				for i := range record.Artifacts {
					record.Artifacts[i].Path = filepath.Join(output, filepath.Base(record.Artifacts[i].Path))
				}
				if err := validateAppIntegrationRecord("RP40", record, model.manifest); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
