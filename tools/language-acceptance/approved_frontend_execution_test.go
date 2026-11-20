package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestP15BRG33ProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG33" {
			selected.Gates.Gates = append(selected.Gates.Gates, gate)
		}
	}
	selected.Gates.GateCount = len(selected.Gates.Gates)
	scenario := supplementalProductionScenarios["BRG33"]
	if selected.Gates.GateCount != 1 || selected.Gates.Gates[0].Title != "Approved frontend semantics remain enforced" ||
		!reflect.DeepEqual(parseRequiredTiers(strings.Join(selected.Gates.Gates[0].MinimumEvidenceTiers, "+")), scenario.Tiers) {
		t.Fatal("BRG33 producer differs from immutable original scenario")
	}
	expectedOriginalLevels := map[string]string{"B02": "repository", "C05": "native-roundtrip", "F05": "unit", "P04": "unit", "T04": "unit+service", "T06": "unit", "T07": "unit", "T08": "unit+service", "T09": "unit+native"}
	seenOriginal := map[string]bool{}
	for _, original := range all.Acceptance.Cases {
		if level, ok := expectedOriginalLevels[original.ID]; ok {
			if original.MinimumTestLevel != level {
				t.Fatalf("original requirement %s had its minimum tier changed", original.ID)
			}
			seenOriginal[original.ID] = true
		}
	}
	if len(seenOriginal) != 9 || len(selected.Gates.Gates[0].RelatedOriginalIDs) != 9 {
		t.Fatal("approved semantic contract inventory changed")
	}
	for _, id := range selected.Gates.Gates[0].RelatedOriginalIDs {
		if !seenOriginal[id] {
			t.Fatal("uncovered original approved-semantic requirement", id)
		}
	}
	expectedTests := []string{"TestP15B02AC01CatalogMigrationAndNativeOutputs", "TestP15B02AC01RejectsCatalogIdentityFaults", "TestP15B02AC01RejectsNativeCaptureFaults", "TestP15C05UnknownEnumNativeImport", "TestP15NativeRoundTripOracleRejectsCorruption", "TestP15F05UnsupportedLoopControlProduction", "TestP15F05SupportedLoopsAndKeywordText", "TestP15P04ReturnTerminators", "TestP15T04OpaqueTypeProduction", "TestP15T04OpaqueTypeService", "TestP15T06RecordAnnotationsDoNotValidateExternalValues", "TestP15T06RecordProofTypeRelations", "TestP15T06ControlResultsRequireAllBranches", "TestP15T07PerPlatformVersionProduction", "TestP15T07MissingPlatformIsUnverified", "TestP15T07MalformedPlatformMetadataFailsClosed", "TestP15T07PlatformVersionService", "TestP15T07PlatformVersionSchemaIdentity", "TestP15T07CanonicalCatalogPropagation", "TestP15T08ConditionalCardinalityProduction", "TestP15T08ConditionalCardinalityService", "TestP15T09ContentConversionEvidence", "TestP15T09ContentConversionNativeWire"}
	if !reflect.DeepEqual(scenario.TestNames, expectedTests) {
		t.Fatal("complete actual semantic scenario selection changed")
	}
	runner := NewRunner()
	runner.runRecoveryGates(selected.Gates)
	result, found := runner.results["BRG33"]
	if !found || len(runner.results) != 1 || len(runner.records) != 4 || len(runner.observations) != 0 ||
		!result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
		t.Fatalf("complete approved frontend semantics not actually executed: %+v", result)
	}
	for _, record := range runner.records {
		if record.ImplementationSHA != runner.implSHA || !reflect.DeepEqual(record.Requirements, []string{"BRG33"}) ||
			record.Result != "passed" || record.ExitCode != 0 || !strings.Contains(record.Detail, scenario.TestNames[0]) {
			t.Fatal("approved frontend semantics receipt lacks actual observed scope")
		}
		for _, name := range expectedTests {
			if !strings.Contains(record.Detail, name) {
				t.Fatal("missing actual semantic test discovery", name)
			}
		}
		if err := validateSupplementalProductionRecord("BRG33", record); err != nil {
			t.Fatal(err)
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.SkippedCases != 0 || report.ClosureComplete {
		t.Fatalf("bounded BRG33 manifest failed or inherited closure: %+v %v", report, err)
	}
	encoded, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"zero_selection", "wrong_producer", "missing_assertion", "stale_source", "missing_unit", "missing_analysis", "missing_service", "missing_integration", "invented_runtime", "wrong_requirement"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			missing := ""
			switch fault {
			case "zero_selection":
				changed.Records[0].Command = productionTestCommand([]string{"TestBRG33NoActualProducer"})
			case "wrong_producer":
				changed.Records[0].TestID = "github.com/electrikmilk/cherri:TestAlphaRenamingPasses"
			case "missing_assertion":
				delete(changed.Records[0].Assertions, "required_production_tests")
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "missing_unit":
				missing = "unit"
			case "missing_analysis":
				missing = "analysis"
			case "missing_integration":
				missing = "integration"
			case "missing_service":
				missing = "service"
			case "invented_runtime":
				changed.Records[0].Tier = "ios-runtime"
			case "wrong_requirement":
				changed.Records[0].Requirements = []string{"BRG05"}
			}
			if missing != "" {
				kept := make([]EvidenceRecord, 0)
				for _, record := range changed.Records {
					if record.Tier != missing {
						kept = append(kept, record)
					}
				}
				changed.Records = kept
			}
			rejected, validationErr := ValidateEvidenceManifest(&changed, selected, "local")
			if p15VerificationResult(t, rejected, "BRG33").Passed || rejected.ClosureComplete {
				t.Fatal("detached or incomplete BRG33 evidence passed")
			}
			if missing == "" && validationErr == nil {
				t.Fatal("invalid BRG33 evidence did not fail validation")
			}
		})
	}
	path := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, path); err != nil {
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
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("bounded BRG33 selection changed full original accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "BRG33" {
			if row.Status != "PASS" || !row.PositiveAndNegative || !reflect.DeepEqual(row.TestNames, scenario.TestNames) || !reflect.DeepEqual(row.ProductionPath, scenario.ProductionPaths) {
				t.Fatalf("BRG33 audit detached from actual producer: %+v", row)
			}
		} else if row.Status == "PASS" || row.PositiveAndNegative {
			t.Fatalf("unexecuted row inherited BRG33 evidence: %+v", row)
		}
	}
	receipt, err := json.Marshal(map[string]any{"source_sha": runner.implSHA, "scope": "Independently executed complete approved frontend scenarios across actual parser, analyzer, service, native encoding/import and catalog propagation; original minimum levels and independent IDs remain mandatory.", "report": report, "records": runner.records})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_APPROVED_FRONTEND_PRODUCER_RECEIPT:" + string(receipt))
}
