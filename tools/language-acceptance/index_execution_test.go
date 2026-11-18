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

func TestIndexProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "L02" || c.ID == "L04" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 2 {
		t.Fatal("missing immutable index contracts")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	for _, contract := range selected.Cases {
		id := contract.ID
		runner.runIndexRequirement(id)
		wantTiers := []string{"native-roundtrip"}
		if id == "L02" {
			wantTiers = []string{"unit", "native-roundtrip"}
		}
		result := runner.results[id]
		if result.Passed || result.Status != "PENDING_EXTERNAL" ||
			result.Requirement != contract.Requirement || result.Group != contract.Group ||
			result.TestLevel != contract.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, wantTiers) {
			t.Fatalf("%s lost contract or local scope: %+v", id, result)
		}
		tests := literalCollectionProductionTests[id]
		for _, path := range literalCollectionProductionPaths[id] {
			if info, err := os.Stat(filepath.Join(resolveRepoRoot(), path)); err != nil || !info.Mode().IsRegular() {
				t.Fatalf("missing production path %s: %v", path, err)
			}
		}
		var observed []string
		for _, record := range runner.records {
			if !reflect.DeepEqual(record.Requirements, []string{id}) {
				continue
			}
			observed = append(observed, record.Tier)
			if record.ImplementationSHA != runner.implSHA || record.Result != "passed" || record.ExitCode != 0 ||
				record.TestID != literalCollectionProductionTestForTier(id, record.Tier) ||
				!reflect.DeepEqual(record.Command, productionTestCommand(tests)) {
				t.Fatalf("detached index producer: %+v", record)
			}
			for _, name := range tests {
				if !strings.Contains(record.Detail, name) {
					t.Fatalf("missing actual execution %s", name)
				}
			}
		}
		if !reflect.DeepEqual(observed, wantTiers) {
			t.Fatalf("%s tiers: %v", id, observed)
		}
	}
	if len(runner.results) != 2 || len(runner.records) != 3 || len(runner.observations) != 0 {
		t.Fatal("index producer changed unrelated accounting")
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 2 || report.ClosureComplete {
		t.Fatalf("local index proof closed iOS: report=%+v error=%v", report, err)
	}
	for _, result := range report.Results {
		if result.Status != "NOT_RUN" || result.Detail != "Pending external / CI execution tier" {
			t.Fatalf("wrong pending result: %+v", result)
		}
	}
	if final, err := ValidateEvidenceManifest(manifest, contracts, "final"); err == nil || final != nil && final.ClosureComplete {
		t.Fatal("local index proofs closed final acceptance")
	}
	for _, fault := range []string{"wrong_producer", "cross_tier_producer", "fake_ios_from_go", "L04_native_without_roundtrip"} {
		t.Run(fault, func(t *testing.T) {
			broken := *manifest
			broken.Records = append([]EvidenceRecord{}, manifest.Records...)
			switch fault {
			case "wrong_producer":
				broken.Records[0].TestID = "tools/language-acceptance:runCollections"
			case "cross_tier_producer":
				broken.Records[1].TestID = broken.Records[0].TestID
			case "fake_ios_from_go":
				fake := broken.Records[0]
				fake.EvidenceID = "ev-local-L02-fabricated-ios"
				fake.Tier = "ios-runtime"
				fake.Assertions = map[string]string{"index_trace": "alpha:gamma:::::beta"}
				broken.Records = append(broken.Records, fake)
			case "L04_native_without_roundtrip":
				broken.Records[2].TestID = "github.com/electrikmilk/cherri:TestP15L04DynamicFractionalIndexGuard"
			}
			if _, err := ValidateEvidenceManifest(&broken, contracts, "local"); err == nil || !strings.Contains(err.Error(), "not authorized for requirement L0") {
				t.Fatalf("%s did not reject producer/tier substitution: %v", fault, err)
			}
		})
	}
	for drop := range manifest.Records {
		broken := *manifest
		broken.Records = append([]EvidenceRecord{}, manifest.Records[:drop]...)
		broken.Records = append(broken.Records, manifest.Records[drop+1:]...)
		partial, err := ValidateEvidenceManifest(&broken, contracts, "local")
		if partial == nil || partial.PassedCases != 0 || partial.ClosureComplete {
			t.Fatalf("missing tier promoted: %+v error=%v", partial, err)
		}
	}
	auditPath := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, all, auditPath); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(auditPath)
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
		t.Fatal("index proof changed formal accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "L02" || row.ID == "L04" {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || row.Blocker == "" ||
				!reflect.DeepEqual(row.TestNames, literalCollectionProductionTests[row.ID]) ||
				!reflect.DeepEqual(row.Tier, runner.results[row.ID].ExecutedLevels) {
				t.Fatalf("lost bounded index proof: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited index success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "L02/L04 exact canonical index control tree and native import round trips only; Apple first/last/no-item behavior, RP21 and overall closure remain unproved",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_INDEX_RECEIPT:" + string(receipt))
}
