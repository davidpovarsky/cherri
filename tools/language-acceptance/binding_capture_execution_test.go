package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestBindingCaptureProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, contract := range all.Acceptance.Cases {
		if contract.ID == "V01" || contract.ID == "V03" {
			selected.Cases = append(selected.Cases, contract)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 2 {
		t.Fatal("missing immutable capture contracts")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	for _, contract := range selected.Cases {
		id := contract.ID
		runner.runBindingCaptureRequirement(id)
		result := runner.results[id]
		if result.Passed || result.Status != "PENDING_EXTERNAL" ||
			result.Group != contract.Group || result.Requirement != contract.Requirement ||
			result.TestLevel != contract.MinimumTestLevel ||
			!reflect.DeepEqual(result.ExecutedLevels, []string{"unit", "native-structure"}) {
			t.Fatalf("%s producer expanded or lost its actual scope: %+v", id, result)
		}
		for _, path := range bindingProductionPaths[id] {
			info, err := os.Stat(filepath.Join(resolveRepoRoot(), path))
			if err != nil || !info.Mode().IsRegular() {
				t.Fatalf("missing actual capture production path %s: %v", path, err)
			}
		}
	}
	if len(runner.results) != 2 || len(runner.records) != 4 || len(runner.observations) != 0 {
		t.Fatal("capture producers changed unrelated accounting")
	}
	for _, record := range runner.records {
		if len(record.Requirements) != 1 {
			t.Fatal("capture record inherited another requirement")
		}
		id := record.Requirements[0]
		if record.ImplementationSHA != runner.implSHA || record.Result != "passed" || record.ExitCode != 0 ||
			validateBindingCaptureRecord(id, record) != nil {
			t.Fatalf("detached capture production receipt: %+v", record)
		}
		for _, test := range bindingProductionTests[id] {
			if !strings.Contains(record.Detail, test) {
				t.Fatalf("missing independently executed scenario %s", test)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 2 || report.ClosureComplete {
		t.Fatalf("local capture graph closed iOS: report=%+v error=%v", report, err)
	}
	if final, err := ValidateEvidenceManifest(manifest, contracts, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("capture graph proof closed final iOS acceptance")
	}
	for _, fault := range []string{"unit_as_native", "fake_ios_from_go", "cross_requirement", "missing_route_command", "stale_source", "failed_local_record"} {
		t.Run(fault, func(t *testing.T) {
			broken := *manifest
			broken.Records = append([]EvidenceRecord{}, manifest.Records...)
			switch fault {
			case "unit_as_native":
				broken.Records[1].TestID = broken.Records[0].TestID
			case "fake_ios_from_go":
				fake := broken.Records[0]
				fake.EvidenceID, fake.Tier = "ev-forged-ios-capture", "ios-runtime"
				broken.Records = append(broken.Records, fake)
			case "cross_requirement":
				broken.Records[1].Requirements = []string{"V03"}
			case "missing_route_command":
				broken.Records[0].Command = productionTestCommand(bindingProductionTests["V01"][:2])
			case "stale_source":
				broken.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "failed_local_record":
				broken.Records[0].ExitCode = 1
			}
			if invalid, err := ValidateEvidenceManifest(&broken, contracts, "local"); err == nil || invalid.PassedCases != 0 || invalid.ClosureComplete {
				t.Fatalf("capture %s mutation accepted: report=%+v err=%v", fault, invalid, err)
			}
		})
	}
	for drop := range manifest.Records {
		t.Run("missing_"+manifest.Records[drop].EvidenceID, func(t *testing.T) {
			broken := *manifest
			broken.Records = append([]EvidenceRecord{}, manifest.Records[:drop]...)
			broken.Records = append(broken.Records, manifest.Records[drop+1:]...)
			if invalid, err := ValidateEvidenceManifest(&broken, contracts, "local"); err == nil || invalid.PassedCases != 0 || invalid.ClosureComplete {
				t.Fatal("missing required local capture tier accepted")
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
	var audit bindingCaptureAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("capture producers changed immutable accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "V01" || row.ID == "V03" {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || row.Blocker == "" ||
				!reflect.DeepEqual(row.TestNames, bindingProductionTests[row.ID]) ||
				!reflect.DeepEqual(row.Tier, []string{"unit", "native-structure"}) {
				t.Fatalf("lost bounded capture evidence: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited capture evidence: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "Independent V01/V03 unit and exact serialized native capture/consumer proofs through compiler, CLI and mobile compile routes. Actual iOS observed values, defaults/transport contracts and overall acceptance remain unproved.",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_BINDING_CAPTURE_PRODUCTION_RECEIPT:" + string(receipt))
}

func TestBindingCaptureRecordRejectsDetachedCommandsAndTiers(t *testing.T) {
	for _, id := range bindingCaptureIDs {
		for _, tier := range []string{"unit", "native-structure"} {
			t.Run(id+"/"+tier, func(t *testing.T) {
				tests := bindingProductionTests[id]
				record := EvidenceRecord{
					EvidenceID: "capture-record-control", Tier: tier,
					TestID:     bindingCaptureProductionTestForTier(id, tier),
					Command:    productionTestCommand(tests),
					Assertions: map[string]string{"required_production_tests": strings.Join(tests, "\n")},
				}
				if err := validateBindingCaptureRecord(id, record); err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"wrong_tier", "wrong_producer", "truncated_command", "missing_consumer_oracle"} {
					broken := record
					broken.Assertions = map[string]string{"required_production_tests": record.Assertions["required_production_tests"]}
					switch fault {
					case "wrong_tier":
						broken.Tier = "ios-runtime"
					case "wrong_producer":
						broken.TestID = "tools/language-acceptance:runBindings"
					case "truncated_command":
						broken.Command = productionTestCommand(tests[:1])
					case "missing_consumer_oracle":
						broken.Assertions["required_production_tests"] = tests[0] + "\n" + tests[2]
					}
					if validateBindingCaptureRecord(id, broken) == nil {
						t.Errorf("accepted %s", fault)
					}
				}
			})
		}
	}
}

type bindingCaptureAuditEnvelope struct {
	ClosureComplete bool                `json:"closure_complete"`
	Rows            []ExecutionAuditRow `json:"rows"`
}
