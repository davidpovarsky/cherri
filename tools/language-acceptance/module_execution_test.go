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

func TestModuleProductionEvidence(t *testing.T) {
	for _, id := range []string{"IM01", "IM02", "IM05"} {
		t.Run(id, func(t *testing.T) {
			producerByTier := map[string]string{}
			switch id {
			case "IM01":
				producerByTier["service"] = "TestP15IM01ModuleServiceRefreshAndExports"
			case "IM02":
				producerByTier["migration"] = moduleProductionTests[id][1]
			case "IM05":
				producerByTier["service"] = moduleProductionTests[id][1]
			}
			testNamedProductionEvidence(t, id, moduleProductionTests[id], producerByTier,
				func(r *Runner) { r.runModuleRequirement(id) }, "P15_MODULE_PRODUCTION_RECEIPT:",
				"Actual module compiler/service resolution, exact native dispatcher identity, canonical cycles and aliases, migration rejection, and observed no-network/no-install boundaries. Only the selected immutable requirement tiers; no native execution or overall closure.")
		})
	}
}

func TestModuleProductionPathsExist(t *testing.T) {
	for id, paths := range moduleProductionPaths {
		for _, path := range paths {
			info, err := os.Stat(filepath.Join(resolveRepoRoot(), path))
			if err != nil || !info.Mode().IsRegular() {
				t.Errorf("%s production path missing/non-file %s: %v", id, path, err)
			}
		}
	}
}

func TestModuleEvidenceRejectsDetachedAndCrossTierRecords(t *testing.T) {
	for _, id := range []string{"IM01", "IM02", "IM03", "IM05"} {
		tests := moduleProductionTests[id]
		tiers := []string{"unit"}
		switch id {
		case "IM01":
			tiers = []string{"unit", "service"}
		case "IM02":
			tiers = []string{"unit", "migration"}
		case "IM05":
			tiers = []string{"security", "service"}
		}
		for _, tier := range tiers {
			t.Run(id+"/"+tier, func(t *testing.T) {
				record := EvidenceRecord{
					EvidenceID: "module-causal-record", Tier: tier,
					TestID: moduleProductionTestForTier(id, tier), Command: productionTestCommand(tests),
					Assertions: map[string]string{"required_production_tests": strings.Join(tests, "\n")},
				}
				if err := validateModuleRecord(id, record); err != nil {
					t.Fatal(err)
				}
				for _, mutation := range []string{"wrong_producer", "unexecuted_native", "truncated_command", "missing_required_test", "cross_tier_producer"} {
					broken := record
					broken.Assertions = map[string]string{"required_production_tests": record.Assertions["required_production_tests"]}
					switch mutation {
					case "wrong_producer":
						broken.TestID = "tools/language-acceptance:runModules"
					case "unexecuted_native":
						broken.Tier = "native"
					case "truncated_command":
						broken.Command = productionTestCommand(tests[:1])
					case "missing_required_test":
						broken.Assertions["required_production_tests"] = tests[0]
					case "cross_tier_producer":
						broken.TestID = "github.com/electrikmilk/cherri:" + tests[len(tests)-1]
					}
					if broken.TestID == record.TestID && mutation == "cross_tier_producer" {
						broken.TestID = "github.com/electrikmilk/cherri:" + tests[0]
					}
					if err := validateModuleRecord(id, broken); err == nil {
						t.Errorf("accepted detached %s record: %s", id, mutation)
					}
				}
			})
		}
	}
}

type moduleAuditEnvelope struct {
	ClosureComplete bool                `json:"closure_complete"`
	Rows            []ExecutionAuditRow `json:"rows"`
}

func TestModuleStandardLibraryUnitEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, contract := range all.Acceptance.Cases {
		if contract.ID == "IM03" {
			selected.Cases = append(selected.Cases, contract)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable IM03 contract")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runModuleRequirement("IM03")
	contract := selected.Cases[0]
	result := runner.results["IM03"]
	if len(runner.results) != 1 || len(runner.records) != 0 || len(runner.observations) != 1 ||
		result.Passed || result.Status != "NOT_RUN" || result.Requirement != contract.Requirement ||
		result.Group != contract.Group || result.TestLevel != contract.MinimumTestLevel ||
		!reflect.DeepEqual(result.ExecutedLevels, []string{"unit"}) {
		t.Fatalf("stdlib producer expanded/lost its observed unit scope: %+v", result)
	}
	observation := runner.observations[0]
	if observation.ID != "IM03" || !observation.Passed || observation.CoverageGap == "" ||
		!reflect.DeepEqual(observation.ClaimedLevels, []string{"unit"}) {
		t.Fatalf("detached stdlib unit observation: %+v", observation)
	}
	for _, name := range moduleProductionTests["IM03"] {
		if !strings.Contains(observation.Detail, name) {
			t.Fatalf("missing actual executed producer %s", name)
		}
	}
	report, err := ValidateAndAggregateWithPhase(selected, runner.results, runner.reg.Fingerprint(), "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete {
		t.Fatalf("local stdlib observations closed native behavior: report=%+v error=%v", report, err)
	}
	if final, err := ValidateAndAggregateWithPhase(selected, runner.results, runner.reg.Fingerprint(), "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("local stdlib observations closed final acceptance")
	}
	t.Run("unit_only_pass_claim_rejected", func(t *testing.T) {
		broken := result
		broken.Passed, broken.Status = true, "PASSED"
		if invalid, err := ValidateAndAggregateWithPhase(selected, map[string]CaseResult{"IM03": broken}, runner.reg.Fingerprint(), "local"); err == nil || invalid.PassedCases != 0 {
			t.Fatal("unit-only result claimed native acceptance")
		}
	})
	// A genuine unit row is still insufficient for the immutable unit+native
	// manifest. Do not weaken that rule or relabel native as external.
	unit := EvidenceRecord{
		EvidenceID: "ev-observed-unit-IM03", Requirements: []string{"IM03"}, Tier: "unit",
		ImplementationSHA: runner.implSHA, Result: "passed", ExitCode: 0,
		TestID: moduleProductionTestForTier("IM03", "unit"), Command: productionTestCommand(moduleProductionTests["IM03"]),
		Assertions: map[string]string{"required_production_tests": strings.Join(moduleProductionTests["IM03"], "\n")},
		Detail:     observation.Detail,
	}
	if err := validateModuleRecord("IM03", unit); err != nil {
		t.Fatal(err)
	}
	t.Run("unit_manifest_cannot_close_local_native_tier", func(t *testing.T) {
		manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: []EvidenceRecord{unit}}
		if invalid, err := ValidateEvidenceManifest(manifest, contracts, "local"); err == nil || invalid.PassedCases != 0 || invalid.ClosureComplete {
			t.Fatal("unit-only manifest satisfied required native tier")
		}
	})
	t.Run("fake_native_record_rejected", func(t *testing.T) {
		fake := unit
		fake.EvidenceID, fake.Tier = "ev-fake-native-IM03", "native"
		manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: []EvidenceRecord{unit, fake}}
		if invalid, err := ValidateEvidenceManifest(manifest, contracts, "local"); err == nil || invalid.PassedCases != 0 || invalid.ClosureComplete {
			t.Fatal("unit producer minted a native tier")
		}
	})
	t.Run("direct_service_package_omission_rejected", func(t *testing.T) {
		broken := unit
		broken.Command = productionTestCommand(moduleProductionTests["IM03"][:1])
		broken.Assertions = map[string]string{"required_production_tests": moduleProductionTests["IM03"][0]}
		if err := validateModuleRecord("IM03", broken); err == nil {
			t.Fatal("record omitted the actual separate service-package execution")
		}
	})
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatal(err)
	}
	var audit moduleAuditEnvelope
	if err := json.Unmarshal(raw, &audit); err != nil {
		t.Fatal(err)
	}
	if audit.ClosureComplete || len(audit.Rows) != 179 {
		t.Fatal("module observations altered immutable accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "IM03" {
			if row.Status != "UNVERIFIED" || row.PositiveAndNegative || row.Blocker == "" ||
				!reflect.DeepEqual(row.TestNames, moduleProductionTests["IM03"]) ||
				!reflect.DeepEqual(row.Tier, []string{"unit"}) ||
				!reflect.DeepEqual(row.Evidence, []string{"acceptance-local-observations.json#IM03"}) {
				t.Fatalf("lost bounded stdlib observations: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited stdlib success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":        "Actual preserved stdfunc source hash and migrated compile structure, including separate service-package execution without main. Only unit observations; the required local native tier and formal IM03 remain NOT_RUN.",
		"unit_command": productionTestCommand(moduleProductionTests["IM03"]),
		"report":       report, "observations": runner.observations, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_MODULE_STDFUNC_UNIT_RECEIPT:" + string(receipt))
}
