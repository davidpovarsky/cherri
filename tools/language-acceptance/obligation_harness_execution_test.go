package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestP15BRG32RequirementMapMatchesAllContracts(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	combined := allLocalContracts(all)
	mapping, err := BuildRequirementsEvidenceMap(all)
	if err != nil {
		t.Fatal(err)
	}
	if mapping.TotalRequirements != 179 || mapping.AcceptanceCount != 94 ||
		mapping.RepairCount != 52 || mapping.GatesCount != 33 ||
		len(mapping.Mappings) != 179 || len(mapping.Records) != 179 {
		t.Fatal("complete immutable obligation inventory was not generated")
	}
	seen := map[string]bool{}
	for _, row := range mapping.Records {
		if seen[row.ID] || !reflect.DeepEqual(row, mapping.Mappings[row.ID]) {
			t.Fatalf("duplicate or divergent public mapping for %s", row.ID)
		}
		seen[row.ID] = true
	}
	for _, obligation := range combined.Cases {
		row, ok := mapping.Mappings[obligation.ID]
		kind := "acceptance"
		if strings.HasPrefix(obligation.ID, "RP") {
			kind = "repair"
		} else if strings.HasPrefix(obligation.ID, "BRG") {
			kind = "recovery_gate"
		}
		if !ok || row.ID != obligation.ID || row.Title != obligation.Requirement || row.Type != kind ||
			!reflect.DeepEqual(row.RequiredTiers, parseRequiredTiers(obligation.MinimumTestLevel)) {
			t.Fatalf("contract identity/title/tiers changed for %s: %+v", obligation.ID, row)
		}
	}
	path := filepath.Join(t.TempDir(), "requirement-evidence-map.json")
	if err := WriteRequirementsEvidenceMap(mapping, path); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Compare the complete public JSON value: omitempty intentionally omits
	// empty assertion maps, so nil versus empty Go maps is not an artifact loss.
	canonical, err := json.Marshal(mapping)
	if err != nil {
		t.Fatal(err)
	}
	var expectedJSON, observedJSON any
	if err := json.Unmarshal(canonical, &expectedJSON); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &observedJSON); err != nil || !reflect.DeepEqual(expectedJSON, observedJSON) {
		t.Fatalf("actual public map artifact changed during write/read: %v", err)
	}
	t.Logf("P15_BRG32_MAP: 94 original + 52 repair + 33 recovery, 179 unique unchanged identities/tiers; artifact sha256=%x", sha256.Sum256(raw))
}

func TestP15BRG32InstalledAIHarness(t *testing.T) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "ls-files", "--", "tools/language-ai-eval")
	command.Dir = root
	output, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.Fields(string(output))
	sort.Strings(paths)
	expected := []string{
		"tools/language-ai-eval/README.md",
		"tools/language-ai-eval/runner.py",
		"tools/language-ai-eval/tasks.json",
		"tools/language-ai-eval/test_runner.py",
	}
	if !reflect.DeepEqual(paths, expected) {
		t.Fatalf("installed harness selection is not the complete tracked package: %v", paths)
	}
	installed := filepath.Join(t.TempDir(), "installed", "language-ai-eval")
	if err := os.MkdirAll(installed, 0700); err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(root, installed)
	if err != nil || relative == "." || !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		t.Fatal("harness must execute from an independent installed directory")
	}
	hashes := map[string]string{}
	for _, path := range paths {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		name := filepath.Base(path)
		hashes[name] = fmt.Sprintf("%x", sha256.Sum256(raw))
		if err := os.WriteFile(filepath.Join(installed, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	// This invokes the actual copied runner through fake adapter/validator
	// subprocesses and separately exercises absent/external configuration guards.
	// It does not invoke any live provider or produce an AI quality score.
	p15RunAIHarnessControlSuite(t, installed)
	for _, path := range paths {
		name := filepath.Base(path)
		for _, file := range []string{filepath.Join(root, filepath.FromSlash(path)), filepath.Join(installed, name)} {
			raw, err := os.ReadFile(file)
			if err != nil || fmt.Sprintf("%x", sha256.Sum256(raw)) != hashes[name] {
				t.Fatalf("installed/original harness changed during read-only controls: %s %v", name, err)
			}
		}
	}
	receipt, err := json.Marshal(map[string]any{
		"files": hashes, "installed_outside_repository": true,
		"required_python_tests": aiEvaluationPythonTests,
		"scope":                 "installed-control-flow-only", "ai_quality_measured": false,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_BRG32_INSTALLED_HARNESS:" + string(receipt))
}

func TestSupplementalObligationHarnessProductionEvidence(t *testing.T) {
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	selected := &ContractsSet{Acceptance: &ContractFile{}, Repair: &RepairContractFile{}, Gates: &GatesContractFile{}}
	for _, gate := range all.Gates.Gates {
		if gate.ID == "BRG32" {
			selected.Gates.Gates = append(selected.Gates.Gates, gate)
		}
	}
	selected.Gates.GateCount = len(selected.Gates.Gates)
	if selected.Gates.GateCount != 1 {
		t.Fatal("immutable BRG32 gate missing")
	}
	runner := NewRunner()
	runner.runRecoveryGates(selected.Gates)
	result := runner.results["BRG32"]
	scenario := supplementalProductionScenarios["BRG32"]
	if len(runner.results) != 1 || len(runner.records) != 3 || len(runner.observations) != 0 ||
		!result.Passed || result.Status != "PASSED" || !reflect.DeepEqual(result.ExecutedLevels, scenario.Tiers) {
		t.Fatalf("BRG32 did not execute its exact local gate: %+v", result)
	}
	for _, record := range runner.records {
		if !reflect.DeepEqual(record.Requirements, []string{"BRG32"}) {
			t.Fatal("related AI/RP requirement inherited execution credit")
		}
	}
	manifest := &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: runner.implSHA, Records: runner.records}
	report, err := ValidateEvidenceManifest(manifest, selected, "local")
	if err != nil || report.PassedCases != 1 || report.FailedCases != 0 || report.ClosureComplete {
		t.Fatalf("actual BRG32 evidence rejected or promoted to full closure: %+v %v", report, err)
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"partial_command", "wrong_producer", "missing_scope", "ai_quality", "missing_python_selection", "borrow_ai01", "missing_repository", "foreign_sha"} {
		t.Run(fault, func(t *testing.T) {
			var broken EvidenceManifest
			if err := json.Unmarshal(raw, &broken); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "partial_command":
				broken.Records[0].Command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames[:1])
			case "wrong_producer":
				broken.Records[0].TestID = "github.com/electrikmilk/cherri/tools/language-acceptance:TestUnrelatedHarness"
			case "missing_scope":
				delete(broken.Records[0].Assertions, "harness_scope")
			case "ai_quality":
				broken.Records[0].Assertions["ai_quality_measured"] = "true"
			case "missing_python_selection":
				delete(broken.Records[0].Assertions, "required_python_tests")
			case "borrow_ai01":
				broken.Records[0].Requirements = []string{"BRG32", "AI01"}
			case "missing_repository":
				var kept []EvidenceRecord
				for _, record := range broken.Records {
					if record.Tier != "repository" {
						kept = append(kept, record)
					}
				}
				broken.Records = kept
			case "foreign_sha":
				broken.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			}
			rejected, validationErr := ValidateEvidenceManifest(&broken, selected, "local")
			if rejected != nil && (p15VerificationResult(t, rejected, "BRG32").Passed || rejected.ClosureComplete) {
				t.Fatal("counterfeit/incomplete BRG32 evidence passed")
			}
			if fault != "missing_repository" && validationErr == nil {
				t.Fatal("counterfeit BRG32 evidence was not rejected")
			}
		})
	}
	receipt, err := json.Marshal(map[string]any{
		"implementation_sha": runner.implSHA, "records": runner.records, "report": report,
		"scope":               "179-obligation repository accounting and installed synthetic harness controls only",
		"ai_quality_measured": false, "live_external_evaluation": "NOT_RUN",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_BRG32_OBLIGATION_HARNESS_RECEIPT:" + string(receipt))
}
