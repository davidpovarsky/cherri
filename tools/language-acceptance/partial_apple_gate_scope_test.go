package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// These are metadata/intake controls with synthetic record fields. No Apple or
// compiler process is executed and no requirement evidence is produced.
func TestPartialAppleGateProbeScopes(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{
		Acceptance: &ContractFile{},
		Repair:     &RepairContractFile{},
		Gates:      &GatesContractFile{},
	}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG25" || gate.ID == "BRG27" {
			selected.Gates.Gates = append(selected.Gates.Gates, gate)
		}
	}
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Gates.GateCount != 2 {
		t.Fatal("missing immutable joined gate contracts")
	}
	mapping, err := BuildRequirementsEvidenceMap(selected)
	if err != nil {
		t.Fatal(err)
	}
	legacyUI := "CherriUITests/testBRG27EditorRealWorldWorkflowUI"
	legacyBridge := "CherriCoreIntegrationTests/testBRG25SharedAnalysisBridgeContract"
	for _, gate := range selected.Gates.Gates {
		row := mapping.Mappings[gate.ID]
		if len(row.TestIDs) == 0 {
			t.Fatal("empty authorization inventory would bypass the producer check", gate.ID)
		}
		if !reflect.DeepEqual(row.RequiredTiers, parseRequiredTiers(strings.Join(gate.MinimumEvidenceTiers, "+"))) {
			t.Fatal("immutable minimum tiers changed", gate.ID)
		}
		for _, producer := range row.TestIDs {
			if producer == legacyUI || producer == legacyBridge {
				t.Fatal("partial Apple probe still authorizes a complete joined requirement", gate.ID)
			}
		}
		if len(row.RequiredUIAssertions) != 0 {
			t.Fatal("unexecuted joined UI assertions remain attached to a partial probe", gate.ID)
		}
	}
	// An absent implementation SHA cannot authorize either enrolled producer.
	// Its real exact-source guard must leave both gates unexecuted; no compiler
	// or native process is launched by this metadata control.
	runner := &Runner{results: make(map[string]CaseResult)}
	runner.runRecoveryGates(selected.Gates)
	if len(runner.records) != 0 || len(runner.results) != 2 {
		t.Fatal("unexecuted joined gates acquired records or changed scope")
	}
	for _, gate := range selected.Gates.Gates {
		result := runner.results[gate.ID]
		if result.Passed || result.Status != "NOT_RUN" || len(result.ExecutedLevels) != 0 {
			t.Fatalf("unexecuted joined gate no longer remains NOT_RUN: %+v", result)
		}
	}
	source := strings.Repeat("a", 40)
	for _, probe := range []struct {
		id, testID, tier string
	}{
		{"BRG25", legacyBridge, "service"},
		{"BRG25", legacyBridge, "ios-build"},
		{"BRG25", legacyUI, "ios-ui"},
		{"BRG27", legacyUI, "ios-ui"},
	} {
		t.Run(probe.id+"/"+probe.tier, func(t *testing.T) {
			record := EvidenceRecord{
				EvidenceID:   "synthetic-partial-" + probe.id + "-" + probe.tier,
				Requirements: []string{probe.id}, Tier: probe.tier, TestID: probe.testID,
				ImplementationSHA: source, Result: "passed",
				Assertions: map[string]string{
					"shared_analysis_bridge": "true", "multi_diagnostics": "true", "completion_contextual": "true",
				},
			}
			manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: source, Records: []EvidenceRecord{record}}
			rejected, rejectErr := ValidateEvidenceManifest(manifest, selected, "local")
			cause := fmt.Sprintf("test_id %q in record %s is not authorized for requirement %s", probe.testID, record.EvidenceID, probe.id)
			if rejectErr == nil || rejected == nil || !strings.Contains(rejectErr.Error(), cause) {
				t.Fatalf("legacy partial producer was not rejected at its authorization boundary: %+v %v", rejected, rejectErr)
			}
			if result := p15VerificationResult(t, rejected, probe.id); result.Passed {
				t.Fatal("relabelled partial probe supplied complete joined gate credit")
			}
		})
	}
}
