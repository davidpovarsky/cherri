package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These are synthetic validator controls in temporary directories. They do not
// execute Xcode, export native acceptance evidence or close the actual BRG28 gate.
func previewNativeCanonicalContracts(t *testing.T) *ContractsSet {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG28" {
			return &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{},
				Gates: &GatesContractFile{GateCount: 1, Gates: []RecoveryGate{gate}}}
		}
	}
	t.Fatal("original BRG28 clause missing")
	return nil
}

func TestPreviewNativeCanonicalMapping(t *testing.T) {
	mapping, err := BuildRequirementsEvidenceMap(previewNativeCanonicalContracts(t))
	if err != nil {
		t.Fatal(err)
	}
	row := mapping.Mappings["BRG28"]
	if !reflect.DeepEqual(row.TestIDs, previewNativeTestIDs("BRG28")) ||
		!reflect.DeepEqual(row.RequiredTiers, []string{"integration", "ios-ui", "benchmark"}) ||
		!reflect.DeepEqual(row.RequiredUIAssertions, previewNativeUIAssertions("BRG28")) ||
		len(row.RequiredAssertions) != 0 {
		t.Fatalf("dedicated immutable BRG28 producer/tier/assertion mapping differs: %+v", row)
	}
	for _, path := range previewNativeSourcePaths {
		found := false
		for _, actual := range row.FixtureIdentities {
			found = found || actual == path
		}
		if !found {
			t.Fatal("dedicated native source witness omitted:", path)
		}
	}
	for _, id := range row.TestIDs {
		if strings.Contains(id, "E07") || strings.Contains(id, "TestDecomp") {
			t.Fatal("generic historical producer inherited BRG28")
		}
	}
}

func TestPreviewNativeCanonicalManifestBoundary(t *testing.T) {
	contracts := previewNativeCanonicalContracts(t)
	var complete *EvidenceManifest
	for _, test := range previewNativeTestIDs("BRG28") {
		t.Run(test, func(t *testing.T) {
			model := newPreviewNativeControlModel(t, test)
			model.record.EvidenceID += "-" + model.record.Tier
			if err := validatePreviewNativeRecord("BRG28", model.record, model.manifest); err != nil {
				t.Fatal("independent healthy validator control rejected:", err)
			}
			model.manifest.Records = []EvidenceRecord{model.record}
			report, _ := ValidateEvidenceManifest(model.manifest, contracts, "local")
			if report == nil {
				t.Fatal("canonical route returned no report")
			}
			result := p15VerificationResult(t, report, "BRG28")
			if result.Passed || report.ClosureComplete || !reflect.DeepEqual(result.ExecutedLevels, []string{model.record.Tier}) {
				t.Fatalf("one dedicated record inherited other required tiers: %+v errors=%v", result, report.ValidationErrors)
			}
			if complete == nil {
				copy := *model.manifest
				copy.Records = nil
				complete = &copy
			}
			complete.Records = append(complete.Records, model.record)
		})
	}
	if complete == nil || len(complete.Records) != 3 {
		t.Fatal("synthetic complete boundary control missing")
	}
	report, err := ValidateEvidenceManifest(complete, contracts, "local")
	if err != nil || report == nil || report.PassedCases != 1 || report.ClosureComplete {
		t.Fatalf("canonical complete synthetic boundary rejected or grants global closure: %+v %v", report, err)
	}
}

func TestPreviewNativeCanonicalOriginalCorruptions(t *testing.T) {
	for _, test := range previewNativeTestIDs("BRG28") {
		for _, fault := range []string{"stage", "proof", "producer", "generic_e07", "cross_tier", "terminal_target", "missing_method"} {
			t.Run(test+"/"+fault, func(t *testing.T) {
				model := newPreviewNativeControlModel(t, test)
				if err := validatePreviewNativeRecord("BRG28", model.record, model.manifest); err != nil {
					t.Fatal("causal baseline rejected:", err)
				}
				switch fault {
				case "stage":
					model.changeLog(strings.Replace(string(model.raw), "actual-source-built", "foreign-stage", 1))
				case "proof":
					model.proof["noCrossTierInheritance"] = false
				case "producer":
					model.record.TestID += "/borrowed"
				case "generic_e07":
					model.record.TestID = "CherriUITests/testE07VisualParameterEditingCanvasUI"
				case "cross_tier":
					model.record.Tier = "ios-runtime"
				case "terminal_target":
					parts := strings.SplitN(previewNativeIntegration, "/", 2)
					before := "Test Case '-[CherriTests." + parts[0] + " " + parts[1] + "]' passed"
					after := strings.Replace(before, "CherriTests.", "ForeignTests.", 1)
					changed := strings.Replace(string(model.raw), before, after, 1)
					if changed == string(model.raw) {
						t.Fatal("terminal-only causal mutation missed actual target")
					}
					model.changeLog(changed)
				case "missing_method":
					model.changeLog(string(previewNativeControlLog(t, previewNativeMethods[1:])))
				}
				model.seal(t)
				model.manifest.Records = []EvidenceRecord{model.record}
				report, err := ValidateEvidenceManifest(model.manifest, previewNativeCanonicalContracts(t), "local")
				if err == nil || report == nil || !strings.Contains(err.Error(), "PREVIEW_") {
					t.Fatalf("canonical route skipped dedicated original validation: fault=%s error=%v", fault, err)
				}
				result := p15VerificationResult(t, report, "BRG28")
				if result.Passed || len(result.ExecutedLevels) != 0 || report.ClosureComplete {
					t.Fatalf("corrupted originals acquired a native tier: %+v", result)
				}
			})
		}
	}
}

func TestPreviewNativeCanonicalAuditDoesNotInventExecution(t *testing.T) {
	model := newPreviewNativeControlModel(t, previewNativeUI)
	runner := &Runner{implSHA: model.record.ImplementationSHA,
		results: map[string]CaseResult{"BRG28": {ID: "BRG28", Status: "NOT_RUN"}},
		records: []EvidenceRecord{model.record}}
	path := filepath.Join(t.TempDir(), "synthetic-preview-intake-audit.json")
	if err := writeExecutionAudit(runner, previewNativeCanonicalContracts(t), path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var audit struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if len(audit.Rows) != 1 || audit.ClosureComplete {
		t.Fatal("dedicated scope changed global accounting")
	}
	row := audit.Rows[0]
	if row.Status == "PASS" || len(row.Tier) != 0 || row.PositiveAndNegative ||
		!reflect.DeepEqual(row.TestNames, previewNativeMethods) ||
		!reflect.DeepEqual(row.ProductionPath, previewNativeSourcePaths) ||
		!strings.Contains(row.Scope, "seven-method") {
		t.Fatalf("audit invented execution or lost exact native ownership: %+v", row)
	}
}

func TestPreviewNativeCanonicalLocalRunnerRemainsUnexecuted(t *testing.T) {
	r := NewRunner()
	r.runRecoveryGates(previewNativeCanonicalContracts(t).Gates)
	result := r.results["BRG28"]
	if result.Status != "NOT_RUN" || result.Passed || len(result.ExecutedLevels) != 0 ||
		len(r.records) != 0 || len(r.observations) != 0 ||
		!strings.Contains(result.Detail, "seven-method") {
		t.Fatalf("unexecuted native producer acquired local credit: %+v", result)
	}
}
