package main

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestSupplementalAtomicContractsProductionEvidence(t *testing.T) {
	var all, err = LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	var wanted = map[string]bool{"RP04": true, "RP34": true}
	var selected = &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, scenario := range all.Repair.Cases {
		if wanted[scenario.ID] {
			selected.Repair.Cases = append(selected.Repair.Cases, scenario)
		}
	}
	selected.Repair.CaseCount = len(selected.Repair.Cases)
	if selected.Repair.CaseCount != 2 {
		t.Fatal("immutable atomic-contract selection changed")
	}
	var runner = NewRunner()
	runner.runRepairCases(selected.Repair)
	if len(runner.results) != 2 || len(runner.records) != 5 || len(runner.observations) != 0 {
		t.Fatalf("executed scope mismatch: results=%d records=%d observations=%d", len(runner.results), len(runner.records), len(runner.observations))
	}
	for id, result := range runner.results {
		var scenario = supplementalProductionScenarios[id]
		if !wanted[id] || !result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
			t.Fatalf("incomplete actual atomic scenario %s: %+v", id, result)
		}
	}
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	var report, validationErr = ValidateEvidenceManifest(manifest, selected, "local")
	if validationErr != nil || report.PassedCases != 2 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatalf("local scenario not bound or improperly promoted: %+v %v", report, validationErr)
	}
	var raw, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for id := range wanted {
		for _, fault := range []string{"missing_producer", "wrong_command", "wrong_test", "stale_sha", "invented_native", "missing_tier"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				var broken EvidenceManifest
				if err := json.Unmarshal(raw, &broken); err != nil {
					t.Fatal(err)
				}
				var index = -1
				for i, record := range broken.Records {
					if record.Requirements[0] == id {
						index = i
						break
					}
				}
				if index < 0 {
					t.Fatal("missing actual record")
				}
				switch fault {
				case "missing_producer":
					delete(broken.Records[index].Assertions, "required_production_tests")
				case "wrong_command":
					broken.Records[index].Command = productionTestCommand([]string{"TestP15UnrelatedAtomicSmoke"})
				case "wrong_test":
					broken.Records[index].TestID = "github.com/electrikmilk/cherri:TestP15UnrelatedAtomicSmoke"
				case "stale_sha":
					broken.Records[index].ImplementationSHA = strings.Repeat("f", 40)
				case "invented_native":
					broken.Records[index].Tier = "ios-runtime"
				case "missing_tier":
					broken.Records = append(broken.Records[:index], broken.Records[index+1:]...)
				}
				var rejected, err = ValidateEvidenceManifest(&broken, selected, "local")
				if rejected != nil && (p15VerificationResult(t, rejected, id).Passed || rejected.ClosureComplete) {
					t.Fatal("damaged atomic evidence passed")
				}
				if fault != "missing_tier" && err == nil {
					t.Fatal("detached atomic producer evidence not rejected")
				}
			})
		}
	}
	var receipt, receiptErr = json.Marshal(map[string]any{"source_sha": runner.implSHA, "report": report, "records": runner.records, "scope": "RP04 checked compile and RP34 atomic migration only; synthetic loopback signer and explicit test-replica OS rename fault; no native Apple execution or release closure"})
	if receiptErr != nil {
		t.Fatal(receiptErr)
	}
	t.Log("P15_ATOMIC_CONTRACT_RECEIPT:" + string(receipt))
}
