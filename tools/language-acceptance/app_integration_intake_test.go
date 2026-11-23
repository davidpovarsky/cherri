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

// Synthetic intake controls only. These files are never exported as actual
// RP40 app execution or as evidence of a successful authenticated CI run.
type rp40IntakeModel struct {
	record     EvidenceRecord
	manifest   *EvidenceManifest
	raw        []byte
	diagnostic map[string]any
	directory  string
}

func rp40IntakeModelForTest(t *testing.T) *rp40IntakeModel {
	t.Helper()
	fixture, err := os.ReadFile(resolveRepoPath(appIntegrationFixture))
	if err != nil {
		t.Fatal(err)
	}
	source := strings.Repeat("a", 40)
	model := &rp40IntakeModel{directory: t.TempDir()}
	model.raw = []byte("Test Case '-[CherriTests.CherriCoreIntegrationTests testRP40ActualAppEntrypointsJoinedContract]' started.\n" +
		appIntegrationCheckpoint + "\n" +
		"Test Case '-[CherriTests.CherriCoreIntegrationTests testRP40ActualAppEntrypointsJoinedContract]' passed (0.123 seconds).\n")
	model.record = EvidenceRecord{
		EvidenceID: "synthetic-rp40-intake-control", Requirements: []string{"RP40"},
		Tier: "app-integration", ImplementationSHA: source, FixtureSHA256: nativeEncodingDigest(fixture),
		TestID: appIntegrationTestID, Command: []string{"xcodebuild", "test", "-scheme", "Cherri"},
		Result: "passed", ExitCode: 0, Assertions: appIntegrationAssertions(), Detail: appIntegrationScope,
	}
	model.diagnostic = map[string]any{
		"schemaVersion": "1.0", "implementationSha": source, "runId": "1234",
		"xcodebuildExitCode": 0, "runStatus": "PASSED", "rawLogPresent": true, "diagnosticOnly": false,
		"testResults": []any{map[string]any{"testId": appIntegrationTestID, "observedStatus": "passed"}},
		"appIntegrationEvidence": map[string]any{
			"status": "VERIFIED_APP_INTEGRATION", "testId": appIntegrationTestID,
			"scope": appIntegrationScope, "fixtureSHA256": model.record.FixtureSHA256, "uiTierGranted": false,
		},
	}
	model.manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: source,
		Runs: []CIRunRecord{{RunID: "1234", Repository: "davidpovarsky/cherri", WorkflowName: "iOS Build",
			Event: "workflow_dispatch", Attempt: 1, HeadSHA: source, CheckoutSHA: source, Conclusion: "success"}}}
	model.seal(t)
	return model
}

func (model *rp40IntakeModel) seal(t *testing.T) {
	t.Helper()
	model.diagnostic["sanitizedLogSha256"] = nativeEncodingDigest(model.raw)
	diagnostic, err := json.Marshal(model.diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	model.record.Artifacts = nil
	for _, item := range []struct {
		name string
		data []byte
	}{
		{"xcodetest.log", model.raw}, {"ios-test-diagnostics.json", diagnostic},
	} {
		path := filepath.Join(model.directory, item.name)
		if err := os.WriteFile(path, item.data, 0600); err != nil {
			t.Fatal(err)
		}
		model.record.Artifacts = append(model.record.Artifacts, EvidenceArtifact{
			Path: path, SHA256: nativeEncodingDigest(item.data), RunID: "1234"})
	}
	model.manifest.Records = []EvidenceRecord{model.record}
}

func rp40SelectedContracts(t *testing.T) *ContractsSet {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, row := range all.Repair.Cases {
		if row.ID == "RP40" {
			selected.Repair.Cases = append(selected.Repair.Cases, row)
		}
	}
	if len(selected.Repair.Cases) != 1 {
		t.Fatal("immutable RP40 scenario absent")
	}
	selected.Repair.CaseCount = 1
	return selected
}

func TestRP40AppIntegrationOriginalContractAndTierBoundary(t *testing.T) {
	selected := rp40SelectedContracts(t)
	row := selected.Repair.Cases[0]
	if row.Title != "Actual Swift integration" ||
		row.Scenario != "Build the real app, open v2 source, request completion/diagnostics/rename and compile through the app on iOS." ||
		row.ExpectedObservation != "Actual Swift call paths use modern service/protocol and shared registry; C export presence alone is insufficient." ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(row.RequiredTestLevels, "+")), []string{"app-integration", "ios-ui"}) {
		t.Fatal("RP40 original contract or UI tier changed")
	}
	if !reflect.DeepEqual(deriveRepairTestIDs("RP40", row.Title), []string{appIntegrationTestID}) ||
		!reflect.DeepEqual(deriveRepairFixtures("RP40"), appIntegrationSourcePaths) {
		t.Fatal("RP40 dedicated app producer source mapping differs")
	}
	ids := []string{}
	assertions, ui := map[string]string{}, map[string]string{}
	applyRuntimeAndUIMappings("RP40", []string{"app-integration", "ios-ui"}, &ids, assertions, ui)
	if !reflect.DeepEqual(ids, []string{appIntegrationUITestID}) || len(assertions) != 0 ||
		!reflect.DeepEqual(ui, appIntegrationUIAssertions()) {
		t.Fatal("RP40 UI mapping must authorize only its dedicated joined scenario")
	}
	model := rp40IntakeModelForTest(t)
	if err := validateAppIntegrationRecord("RP40", model.record, model.manifest); err != nil {
		t.Fatal(err)
	}
	report, err := ValidateEvidenceManifest(model.manifest, selected, "local")
	if err != nil || report == nil || report.TotalCases != 1 || report.PassedCases != 0 ||
		report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete ||
		len(report.Results) != 1 || report.Results[0].Status != "NOT_RUN" ||
		!reflect.DeepEqual(report.Results[0].ExecutedLevels, []string{"app-integration"}) {
		t.Fatalf("app proof completed absent UI tier: %+v %v", report, err)
	}
	for _, tier := range []string{"service", "integration", "ios-ui"} {
		record := model.record
		record.Tier = tier
		if err := validateAppIntegrationRecord("RP40", record, model.manifest); err == nil {
			t.Fatal("unowned tier accepted", tier)
		}
	}
	final, err := ValidateEvidenceManifest(model.manifest, selected, "final")
	if err == nil || (final != nil && final.ClosureComplete) ||
		!strings.Contains(err.Error(), "requirement RP40 has unsatisfied required tiers") {
		t.Fatal("missing UI did not block final RP40 closure", err)
	}
}

func TestRP40AppIntegrationOriginalIntakeCorruptions(t *testing.T) {
	cases := []struct{ name, want string }{
		{"borrowed_service", "RP40_RECORD"}, {"fake_ui", "RP40_RECORD"}, {"scope", "RP40_RECORD"},
		{"assertion", "RP40_RECORD"}, {"command", "RP40_RECORD"}, {"record_source", "RP40_RECORD"},
		{"fixture", "RP40_FIXTURE"}, {"missing_artifact", "RP40_ARTIFACTS"}, {"mixed_run", "RP40_ARTIFACTS"},
		{"digest", "RP40_ARTIFACT_DIGEST"}, {"missing_run", "RP40_RUN"}, {"failed_run", "RP40_RUN"},
		{"checkout", "RP40_RUN"}, {"workflow", "RP40_RUN"}, {"event", "RP40_RUN"}, {"duplicate_run", "RP40_RUN"},
		{"process_exit", "RP40_DIAGNOSTIC"}, {"diagnostic_only", "RP40_DIAGNOSTIC"},
		{"failed_outcome", "RP40_DIAGNOSTIC"}, {"duplicate_outcome", "RP40_DIAGNOSTIC"},
		{"missing_joined_outcome", "RP40_DIAGNOSTIC"}, {"ui_scope", "RP40_DIAGNOSTIC"},
		{"diagnostic_fixture", "RP40_DIAGNOSTIC"}, {"marker_missing", "RP40_CHECKPOINT"},
		{"marker_outside", "RP40_CHECKPOINT"}, {"marker_duplicate", "RP40_CHECKPOINT"},
		{"marker_suffix", "RP40_CHECKPOINT"}, {"failed_case", "RP40_CASE"}, {"duplicate_case", "RP40_CASE"},
		{"incomplete_case", "RP40_CASE"}, {"wrong_target", "RP40_CHECKPOINT"}, {"rejected_marker", "RP40_CHECKPOINT"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			model := rp40IntakeModelForTest(t)
			// Every negative first proves the exact unmodified base is admissible.
			if err := validateAppIntegrationRecord("RP40", model.record, model.manifest); err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "borrowed_service":
				model.record.TestID = "CherriCoreIntegrationTests/testRP40SwiftIntegrationService"
			case "fake_ui":
				model.record.Tier = "ios-ui"
			case "scope":
				model.record.Detail = "Actual UI passed"
			case "assertion":
				delete(model.record.Assertions, "semantic_rename_exact")
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
			case "workflow":
				model.manifest.Runs[0].WorkflowName = "Build & Test"
			case "event":
				model.manifest.Runs[0].Event = "schedule"
			case "duplicate_run":
				model.manifest.Runs = append(model.manifest.Runs, model.manifest.Runs[0])
			case "process_exit":
				model.diagnostic["xcodebuildExitCode"] = 65
				model.seal(t)
			case "diagnostic_only":
				model.diagnostic["diagnosticOnly"] = true
				model.seal(t)
			case "failed_outcome":
				model.diagnostic["testResults"].([]any)[0].(map[string]any)["observedStatus"] = "failed"
				model.seal(t)
			case "duplicate_outcome":
				rows := model.diagnostic["testResults"].([]any)
				model.diagnostic["testResults"] = append(rows, rows[0])
				model.seal(t)
			case "missing_joined_outcome":
				model.diagnostic["testResults"] = []any{map[string]any{"testId": "CherriCoreIntegrationTests/testRP40SwiftIntegrationService", "observedStatus": "passed"}}
				model.seal(t)
			case "ui_scope":
				model.diagnostic["appIntegrationEvidence"].(map[string]any)["uiTierGranted"] = true
				model.seal(t)
			case "diagnostic_fixture":
				model.diagnostic["appIntegrationEvidence"].(map[string]any)["fixtureSHA256"] = strings.Repeat("0", 64)
				model.seal(t)
			case "marker_missing":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationCheckpoint+"\n", ""))
				model.seal(t)
			case "marker_outside":
				model.raw = []byte(appIntegrationCheckpoint + "\n" + strings.ReplaceAll(string(model.raw), appIntegrationCheckpoint+"\n", ""))
				model.seal(t)
			case "marker_duplicate":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationCheckpoint, appIntegrationCheckpoint+"\n"+appIntegrationCheckpoint))
				model.seal(t)
			case "marker_suffix":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), appIntegrationCheckpoint, appIntegrationCheckpoint+" EXTRA"))
				model.seal(t)
			case "failed_case":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), "passed (", "failed ("))
				model.seal(t)
			case "duplicate_case":
				model.raw = append(model.raw, model.raw...)
				model.seal(t)
			case "incomplete_case":
				model.raw = []byte(strings.Split(string(model.raw), "Test Case '-[CherriTests.CherriCoreIntegrationTests testRP40ActualAppEntrypointsJoinedContract]' passed")[0])
				model.seal(t)
			case "wrong_target":
				model.raw = []byte(strings.ReplaceAll(string(model.raw), "CherriTests.CherriCoreIntegrationTests", "OtherTarget.CherriCoreIntegrationTests"))
				model.seal(t)
			case "rejected_marker":
				model.raw = append(model.raw, []byte(appIntegrationRejected+"INVALID_CHECKPOINT\n")...)
				model.seal(t)
			default:
				t.Fatal("unknown control")
			}
			err := validateAppIntegrationRecord("RP40", model.record, model.manifest)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("causal guard %s absent: got %v", tc.want, err)
			}
		})
	}
}

func TestRP40AppIntegrationRealCollectorContract(t *testing.T) {
	// Execute only the existing public collector on synthetic bounded XCTest
	// originals. This is a transport/intake control, never a native execution.
	for _, exit := range []string{"0", "65"} {
		t.Run("xcode_exit_"+exit, func(t *testing.T) {
			model := rp40IntakeModelForTest(t)
			rawPath := filepath.Join(model.directory, "raw.log")
			exitPath := filepath.Join(model.directory, "xcode.exit")
			output := filepath.Join(model.directory, "collector")
			if err := os.WriteFile(rawPath, model.raw, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(exitPath, []byte(exit), 0600); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			root, err := filepath.Abs(resolveRepoRoot())
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.CommandContext(ctx, "python3", "-B", filepath.Join(root, "ios/scripts/collect_ios_test_evidence.py"),
				rawPath, output, model.record.ImplementationSHA, "1234", exitPath)
			cmd.Dir = root
			raw, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("actual collector control failed: %v %s", err, raw)
			}
			evidence, err := os.ReadFile(filepath.Join(output, "ios-test-evidence.json"))
			if err != nil {
				t.Fatal(err)
			}
			var result struct{ Records []EvidenceRecord }
			if err = json.Unmarshal(evidence, &result); err != nil {
				t.Fatal(err)
			}
			if exit != "0" {
				if len(result.Records) != 0 {
					t.Fatal("failed original acquired app credit")
				}
				return
			}
			if len(result.Records) != 1 {
				t.Fatal("collector omitted or expanded joined app record")
			}
			record := result.Records[0]
			for i := range record.Artifacts {
				record.Artifacts[i].Path = filepath.Join(output, filepath.Base(record.Artifacts[i].Path))
			}
			if err = validateAppIntegrationRecord("RP40", record, model.manifest); err != nil {
				t.Fatal(err)
			}
		})
	}
}
