package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func p15AppendAssetAuthorizationSelection(t *testing.T, all, selected *ContractsSet) int {
	t.Helper()
	if _, shortcut := supplementalProductionScenarios["RP29"]; shortcut {
		t.Fatal("RP29 must retain its raw benchmark and security discovery producer")
	}
	for _, scenario := range selected.Repair.Cases {
		if scenario.ID == "RP29" {
			t.Fatal("RP29 was selected twice")
		}
	}
	for _, scenario := range all.Repair.Cases {
		if scenario.ID == "RP29" {
			if !reflect.DeepEqual(parseRequiredTiers(strings.Join(scenario.RequiredTestLevels, "+")), []string{"security", "service", "benchmark"}) ||
				!reflect.DeepEqual(assetAuthorizationScenario.Tiers, []string{"security", "service", "benchmark"}) {
				t.Fatal("asset authorization producer changed immutable RP29 tiers")
			}
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
			selected.Repair.CaseCount = len(selected.Repair.Cases)
			return len(assetAuthorizationScenario.Tiers)
		}
	}
	t.Fatal("immutable RP29 requirement is absent")
	return 0
}

func TestAssetAuthorizationCannotChangeRequiredTiers(t *testing.T) {
	for _, levels := range [][]string{nil, {"service", "benchmark"}, {"security"}, {"security", "service", "ios-runtime"}} {
		var runner = NewRunner()
		if runner.runAssetAuthorizationRequirement("RP29", "Asset and path authorization", levels) || len(runner.records) != 0 || len(runner.results) != 0 {
			t.Fatal("partial asset authorization minted evidence", levels)
		}
	}
	for _, id := range []string{"IM04", "S03", "E08"} {
		var runner = NewRunner()
		if runner.runAssetAuthorizationRequirement(id, "Asset and path authorization", assetAuthorizationScenario.Tiers) || len(runner.records) != 0 || len(runner.results) != 0 {
			t.Fatal("related requirement inherited RP29 execution", id)
		}
	}
}

func assetAuthorizationChangeStream(t *testing.T, raw []byte, target string, skip bool) []byte {
	t.Helper()
	var output bytes.Buffer
	var changed bool
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte{'\n'}) {
		var event map[string]any
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event["Test"] == target {
			if !skip {
				changed = true
				continue
			}
			if event["Action"] == "pass" {
				changed = true
				event["Action"] = "skip"
			}
		}
		var encoded, err = json.Marshal(event)
		if err != nil {
			t.Fatal(err)
		}
		output.Write(encoded)
		output.WriteByte('\n')
	}
	if !changed {
		t.Fatal("security stream control did not address a discovered scenario", target)
	}
	return output.Bytes()
}

func TestAssetAuthorizationRequiresEveryActualSecurityChild(t *testing.T) {
	var required = assetAuthorizationScenario.TestNames
	var raw = editorModelStream(t, "RP29", editorIntakeModel())
	if _, _, err := parseEditorProductionOutput(raw, required); err != nil {
		t.Fatal(err)
	}
	for _, child := range assetAuthorizationSecuritySubtests {
		t.Run("missing/"+child, func(t *testing.T) {
			var changed = assetAuthorizationChangeStream(t, raw, child, false)
			if _, _, err := parseEditorProductionOutput(changed, required); err == nil || !strings.Contains(err.Error(), "missing actual security child execution") {
				t.Fatal("passing parents hid missing security assertions", child, err)
			}
		})
	}
	for _, child := range []string{assetAuthorizationSecuritySubtests[4], assetAuthorizationSecuritySubtests[5], "TestP15RP29CachedResourceIsReauthorizedAfterSymlinkReplacement"} {
		t.Run("skipped/"+child, func(t *testing.T) {
			var changed = assetAuthorizationChangeStream(t, raw, child, true)
			if _, _, err := parseEditorProductionOutput(changed, required); err == nil || !strings.Contains(err.Error(), "production skip") {
				t.Fatal("unsupported symlink execution became security proof", child, err)
			}
		})
	}
	var record = editorModelRecord(t, "RP29", editorIntakeModel())
	delete(record.Assertions, "required_security_subtests")
	if err := validateEditorPerformanceRecord("RP29", record); err == nil {
		t.Fatal("missing security discovery binding accepted")
	}
}

// Corrupt copied actual artifacts and reconcile every outer hash. The resulting
// rejection must come from missing/skipped execution, not a stale digest.
func assetAuthorizationCorruptActualStream(t *testing.T, manifest *EvidenceManifest, skip bool) {
	t.Helper()
	var directory = t.TempDir()
	var copied = map[string]string{}
	for index := range manifest.Records {
		for artifactIndex := range manifest.Records[index].Artifacts {
			var artifact = &manifest.Records[index].Artifacts[artifactIndex]
			var destination, exists = copied[artifact.Path]
			if !exists {
				destination = filepath.Join(directory, filepath.Base(artifact.Path))
				var data, err = os.ReadFile(artifact.Path)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(destination, data, 0600); err != nil {
					t.Fatal(err)
				}
				copied[artifact.Path] = destination
			}
			artifact.Path = destination
		}
	}
	var rawPath = filepath.Join(directory, editorArtifactName("RP29", "-go.jsonl"))
	var raw, err = os.ReadFile(rawPath)
	if err != nil {
		t.Fatal(err)
	}
	raw = assetAuthorizationChangeStream(t, raw, assetAuthorizationSecuritySubtests[4], skip)
	if err := os.WriteFile(rawPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var processPath = filepath.Join(directory, editorArtifactName("RP29", "-process.json"))
	var process, readErr = os.ReadFile(processPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var receipt editorProcessReceipt
	if err := json.Unmarshal(process, &receipt); err != nil {
		t.Fatal(err)
	}
	receipt.RawSHA256, receipt.RawBytes = editorSHA(raw), len(raw)
	process, err = json.Marshal(receipt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(processPath, process, 0600); err != nil {
		t.Fatal(err)
	}
	for index := range manifest.Records {
		for artifactIndex := range manifest.Records[index].Artifacts {
			var artifact = &manifest.Records[index].Artifacts[artifactIndex]
			var data, err = os.ReadFile(artifact.Path)
			if err != nil {
				t.Fatal(err)
			}
			artifact.SHA256 = editorSHA(data)
		}
	}
}

func TestP15RP29AssetAuthorizationProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	if p15AppendAssetAuthorizationSelection(t, all, selected) != 3 || selected.Repair.CaseCount != 1 {
		t.Fatal("RP29 independent selection drifted")
	}
	var runner = NewRunner()
	runner.artifactDir = os.Getenv("CHERRI_EDITOR_EVIDENCE_DIR")
	if runner.artifactDir == "" {
		runner.artifactDir = t.TempDir()
	}
	runner.runRepairCases(selected.Repair)
	var result = runner.results["RP29"]
	if len(runner.results) != 1 || len(runner.records) != 3 || len(runner.observations) != 0 ||
		!result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, assetAuthorizationScenario.Tiers) {
		t.Fatalf("actual RP29 security/service/benchmark scenario failed: %+v", result)
	}
	for _, record := range runner.records {
		if err := validateEditorPerformanceRecord("RP29", record); err != nil {
			t.Fatal(err)
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, validationErr = ValidateEvidenceManifest(manifest, selected, "local")
	if validationErr != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("actual RP29 proof rejected or broadened: %+v %v", report, validationErr)
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, fault := range []string{"missing_security_tier", "wrong_security_producer", "missing_security_binding", "missing_symlink_child", "skipped_symlink_child", "stale_source", "invented_runtime", "borrowed_original", "missing_raw_stream"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "missing_security_tier":
				changed.Records = changed.Records[1:]
			case "wrong_security_producer":
				changed.Records[0].TestID = "github.com/electrikmilk/cherri:" + editorBenchmarkTest
			case "missing_security_binding":
				delete(changed.Records[0].Assertions, "required_security_subtests")
			case "missing_symlink_child", "skipped_symlink_child":
				assetAuthorizationCorruptActualStream(t, &changed, fault == "skipped_symlink_child")
				var expected = "missing actual security child execution"
				if fault == "skipped_symlink_child" {
					expected = "production skip"
				}
				if err := validateEditorPerformanceRecord("RP29", changed.Records[0]); err == nil || !strings.Contains(err.Error(), expected) {
					t.Fatal("reconciled hashes hid missing actual security execution", err)
				}
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "borrowed_original":
				changed.Records[0].Requirements = []string{"IM04"}
			case "missing_raw_stream":
				changed.Records[0].Artifacts = changed.Records[0].Artifacts[1:]
			}
			var rejected, err = ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "RP29").Passed || rejected.ClosureComplete {
				t.Fatal("partial or corrupted RP29 proof accepted")
			}
			if fault != "missing_security_tier" && err == nil {
				t.Fatal("invalid RP29 evidence raised no validation error")
			}
		})
	}
	var auditPath = filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	var raw, readErr = os.ReadFile(auditPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var audit struct {
		Rows            []ExecutionAuditRow `json:"rows"`
		ClosureComplete bool                `json:"closure_complete"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil || len(audit.Rows) != 179 || audit.ClosureComplete {
		t.Fatal("RP29 changed immutable179 scope", err)
	}
	for _, row := range audit.Rows {
		if row.ID == "RP29" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, assetAuthorizationScenario.TestNames) ||
				!reflect.DeepEqual(row.ProductionPath, assetAuthorizationScenario.ProductionPaths) {
				t.Fatalf("RP29 audit detached from actual producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatal("related obligation inherited RP29 evidence", row.ID)
		}
	}
	var receipt, receiptErr = json.Marshal(map[string]any{"source_sha": runner.implSHA, "report": report, "records": runner.records,
		"artifact_paths": editorSortedArtifactPaths(runner.records), "scope": "Actual RP29 authorization, reauthorization and independent local service/cache benchmark only; no related-ID, native UI, race or release closure credit"})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_RP29_PRODUCTION_RECEIPT:" + string(receipt))
}
