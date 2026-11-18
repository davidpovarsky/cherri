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

func TestUnicodeTextProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractFile{}
	for _, c := range all.Acceptance.Cases {
		if c.ID == "S01" {
			selected.Cases = append(selected.Cases, c)
		}
	}
	selected.CaseCount = len(selected.Cases)
	if selected.CaseCount != 1 {
		t.Fatal("missing immutable S01 contract")
	}
	contracts := &ContractsSet{Acceptance: selected, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	runner := NewRunner()
	runner.runUnicodeTextRequirement()
	wantTiers := []string{"unit", "native-roundtrip"}
	result := runner.results["S01"]
	contract := selected.Cases[0]
	if result.Passed || result.Status != "PENDING_EXTERNAL" ||
		result.Requirement != contract.Requirement || result.Group != contract.Group ||
		result.TestLevel != contract.MinimumTestLevel || !reflect.DeepEqual(result.ExecutedLevels, wantTiers) ||
		len(runner.results) != 1 || len(runner.records) != 2 || len(runner.observations) != 0 {
		t.Fatalf("S01 lost exact local scope or invented iOS closure: result=%+v records=%+v", result, runner.records)
	}
	tests := literalCollectionProductionTests["S01"]
	for _, path := range literalCollectionProductionPaths["S01"] {
		if info, err := os.Stat(filepath.Join(resolveRepoRoot(), path)); err != nil || !info.Mode().IsRegular() {
			t.Fatalf("production path %s is not a real file: %v", path, err)
		}
	}
	for i, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"S01"}) || record.ImplementationSHA != runner.implSHA ||
			record.Tier != wantTiers[i] || record.Result != "passed" || record.ExitCode != 0 ||
			record.TestID != literalCollectionProductionTestForTier("S01", record.Tier) ||
			!reflect.DeepEqual(record.Command, productionTestCommand(tests)) {
			t.Fatalf("detached local producer: %+v", record)
		}
		for _, name := range tests {
			if !strings.Contains(record.Detail, name) {
				t.Fatalf("missing execution %s", name)
			}
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, contracts, "local")
	if err != nil || report.PassedCases != 0 || report.FailedCases != 0 || report.SkippedCases != 1 || report.ClosureComplete ||
		len(report.Results) != 1 || report.Results[0].Status != "NOT_RUN" || report.Results[0].Detail != "Pending external / CI execution tier" {
		t.Fatalf("local tiers must not close iOS: report=%+v error=%v", report, err)
	}
	if final, err := ValidateEvidenceManifest(manifest, contracts, "final"); err == nil || (final != nil && final.ClosureComplete) {
		t.Fatal("unsigned local Unicode tests closed final acceptance")
	}
	for _, fault := range []string{"wrong_producer", "cross_tier_producer", "fake_ios_from_go"} {
		t.Run(fault, func(t *testing.T) {
			broken := *manifest
			broken.Records = append([]EvidenceRecord{}, manifest.Records...)
			switch fault {
			case "wrong_producer":
				broken.Records[0].TestID = "tools/language-acceptance:runText"
			case "cross_tier_producer":
				broken.Records[1].TestID = broken.Records[0].TestID
			case "fake_ios_from_go":
				fake := broken.Records[0]
				fake.EvidenceID = "ev-local-S01-fabricated-ios"
				fake.Tier = "ios-runtime"
				fake.Assertions = map[string]string{"unicode_output": "שָׁלוֹם:🌍:1"}
				broken.Records = append(broken.Records, fake)
			}
			_, err := ValidateEvidenceManifest(&broken, contracts, "local")
			if err == nil || !strings.Contains(err.Error(), "not authorized for requirement S01") {
				t.Fatalf("%s did not reject producer/tier substitution: %v", fault, err)
			}
		})
	}
	for _, drop := range []int{0, 1} {
		broken := *manifest
		broken.Records = []EvidenceRecord{manifest.Records[1-drop]}
		partial, err := ValidateEvidenceManifest(&broken, contracts, "local")
		if partial == nil || partial.PassedCases != 0 || partial.ClosureComplete ||
			(len(partial.Results) > 0 && p15UnicodeContainsTier(partial.Results[0].ExecutedLevels, wantTiers[drop])) {
			t.Fatalf("missing local tier was promoted: %+v error=%v", partial, err)
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
		t.Fatal("local Unicode proof changed formal accounting")
	}
	for _, row := range audit.Rows {
		if row.ID == "S01" {
			if row.Status != "BLOCKED_EXTERNAL" || !row.PositiveAndNegative || !reflect.DeepEqual(row.Tier, wantTiers) ||
				!reflect.DeepEqual(row.TestNames, tests) || row.Blocker == "" {
				t.Fatalf("lost bounded Unicode proof: %+v", row)
			}
		} else if row.Status == "PASS" {
			t.Fatalf("unexecuted requirement inherited S01 success: %+v", row)
		}
	}
	status, statusErr := exec.Command("git", "-C", resolveRepoRoot(), "status", "--porcelain", "--untracked-files=normal").Output()
	receipt, err := json.Marshal(map[string]any{
		"source_sha": runner.implSHA, "source_tree_clean": statusErr == nil && len(status) == 0,
		"scope":  "S01 actual local UTF-16 token/literal and exact native import round trips only; iOS observable text, linked repairs/gates and release closure remain unproved",
		"report": report, "records": runner.records,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_UNICODE_TEXT_RECEIPT:" + string(receipt))
}

func p15UnicodeContainsTier(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
