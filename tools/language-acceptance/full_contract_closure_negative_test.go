package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

type p15FullClosureExpectedRow struct {
	group, requirement, tiers string
}

// The fixed fixture records the normalized tiers of all three immutable
// contracts. It deliberately does not call the production tier normalizer.
func p15FullClosureInventory(t *testing.T, contracts *ContractsSet) map[string]p15FullClosureExpectedRow {
	t.Helper()
	if contracts.Acceptance.CaseCount != 94 || len(contracts.Acceptance.Cases) != 94 ||
		contracts.Repair.CaseCount != 52 || len(contracts.Repair.Cases) != 52 ||
		contracts.Gates.GateCount != 33 || len(contracts.Gates.Gates) != 33 {
		t.Fatal("the immutable 94 + 52 + 33 contract inventory changed")
	}
	rows := make(map[string]p15FullClosureExpectedRow, 179)
	add := func(id, group, requirement string) {
		if _, duplicate := rows[id]; duplicate || id == "" {
			t.Fatalf("invalid or duplicate immutable requirement %q", id)
		}
		rows[id] = p15FullClosureExpectedRow{group: group, requirement: requirement}
	}
	for _, spec := range contracts.Acceptance.Cases {
		add(spec.ID, spec.Group, spec.Requirement)
	}
	for _, spec := range contracts.Repair.Cases {
		add(spec.ID, "repair", spec.Title)
	}
	for _, spec := range contracts.Gates.Gates {
		add(spec.ID, "gate", spec.Title)
	}
	raw, err := os.ReadFile(filepath.Join(resolveRepoRoot(), "tools", "language-acceptance", "testdata", "full179-required-tiers.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool, 179)
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "\t")
		if len(parts) != 3 || seen[parts[0]] || parts[2] == "" {
			t.Fatalf("invalid fixed contract tier row: %q", line)
		}
		row, present := rows[parts[0]]
		if !present || row.group != parts[1] {
			t.Fatalf("fixed tier row does not match an immutable contract: %q", line)
		}
		row.tiers = parts[2]
		rows[parts[0]] = row
		seen[parts[0]] = true
	}
	if len(rows) != 179 || len(seen) != 179 {
		t.Fatalf("full contract inventory lost rows: contracts=%d fixture=%d", len(rows), len(seen))
	}
	return rows
}

// These are explicitly synthetic validator inputs. No producer is executed and
// no result from this negative test is exported as production or Apple evidence.
// Their sole positive purpose is to prove that an authorized FN01 unit/native
// input receives exactly those two tiers, while linked RP/BRG rows receive none.
func p15FullClosureFN01Inputs(implSHA string) []EvidenceRecord {
	tests := []string{"TestP15FN01FunctionTyping", "TestP15FN01ReturnAndContinuationNative"}
	const scope = "Actual AST function typing and rejection before emission; complete native argument, local-return and caller-continuation graph with causal corruptions"
	var records []EvidenceRecord
	for index, tier := range []string{"unit", "native-structure"} {
		records = append(records, EvidenceRecord{
			EvidenceID:        "synthetic-validator-only-FN01-" + tier,
			Requirements:      []string{"FN01"},
			Tier:              tier,
			ImplementationSHA: implSHA,
			TestID:            "github.com/electrikmilk/cherri:" + tests[index],
			Command:           productionTestCommand(tests),
			ExitCode:          0,
			Result:            "passed",
			Assertions: map[string]string{
				"fixture_kind":              "synthetic validator input; production tests not executed",
				"required_production_tests": strings.Join(tests, "\n"),
				"scope":                     scope,
			},
		})
	}
	return records
}

func p15AssertFullClosureGaps(t *testing.T, report *AcceptanceReport, err error,
	rows map[string]p15FullClosureExpectedRow, credited map[string][]string, attacked string,
	authorizationErrors []string) {
	t.Helper()
	if err == nil || report == nil || report.ClosureComplete || report.Phase != "final" ||
		report.TotalCases != 179 || len(report.Results) != 179 || report.PassedCases != 0 ||
		report.FailedCases != 178 || report.SkippedCases != 1 {
		t.Fatalf("unexecuted full contract fabricated closure or changed accounting: err=%v report=%+v", err, report)
	}
	expectedErrors := append([]string(nil), authorizationErrors...)
	seen := make(map[string]bool, 179)
	for _, result := range report.Results {
		row, present := rows[result.ID]
		if !present || seen[result.ID] {
			t.Fatalf("unknown or duplicate report row: %+v", result)
		}
		seen[result.ID] = true
		if result.Group != row.group || result.Requirement != row.requirement || result.TestLevel != row.tiers ||
			result.Passed || !reflect.DeepEqual(result.ExecutedLevels, credited[result.ID]) {
			t.Fatalf("row changed contract scope or gained unexecuted tiers: got=%+v want=%+v levels=%v",
				result, row, credited[result.ID])
		}
		switch result.ID {
		case "AI01":
			if result.Status != "BLOCKED_EXTERNAL" {
				t.Fatalf("unexecuted AI evaluation was not blocked: %+v", result)
			}
		case "T05":
			if result.Status != "NOT_RUN" || !strings.HasPrefix(result.Detail, "Formal scope UNVERIFIED: ") {
				t.Fatalf("incomplete JSON-null semantics were hidden: %+v", result)
			}
			expectedErrors = append(expectedErrors, "requirement T05 has incomplete JSON-null semantics")
		default:
			if result.Status != "FAILED" {
				t.Fatalf("unexecuted requirement did not fail final closure: %+v", result)
			}
			if result.ID == attacked {
				if result.Detail != "Evidence record reported failure" {
					t.Fatalf("retagged producer did not cause the attacked row failure: %+v", result)
				}
				expectedErrors = append(expectedErrors, "requirement "+result.ID+" has failed evidence record")
			} else {
				if result.Detail != "Unsatisfied required tiers in final closure" {
					t.Fatalf("an unrelated failure masked missing tiers: %+v", result)
				}
				expectedErrors = append(expectedErrors, "requirement "+result.ID+" has unsatisfied required tiers in final phase")
			}
		}
	}
	if len(seen) != len(rows) {
		t.Fatalf("report omitted immutable requirements: got=%d want=%d", len(seen), len(rows))
	}
	actualErrors := append([]string(nil), report.ValidationErrors...)
	sort.Strings(actualErrors)
	sort.Strings(expectedErrors)
	if !reflect.DeepEqual(actualErrors, expectedErrors) {
		t.Fatalf("final rejection must have exactly the intended causes:\ngot: %q\nwant: %q", actualErrors, expectedErrors)
	}
}

func TestFull179ContractRequirements_RejectsUnexecutedScenarioClosure(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	rows := p15FullClosureInventory(t, contracts)
	originalDiscovery, originalPaths := ciRunDiscoveryFunc, defaultEvidencePathsFunc
	t.Cleanup(func() {
		ciRunDiscoveryFunc, defaultEvidencePathsFunc = originalDiscovery, originalPaths
	})
	const implSHA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const docsSHA = "4218b152f6aea5e8e17912a8b4a8924ffd819f00"
	// Synthetic successful workflow metadata removes unrelated CI absence from
	// this validator-only control. It is never published as actual CI evidence.
	ciRunDiscoveryFunc = func(commitSHA string) ([]CIRunRecord, error) {
		if commitSHA != implSHA {
			return nil, fmt.Errorf("unexpected synthetic implementation identity %q", commitSHA)
		}
		var runs []CIRunRecord
		for index, workflow := range []string{"Build & Test", "OpenMinis Skill", "iOS Build", "iOS 27 Shortcuts Runtime PoC"} {
			runs = append(runs, CIRunRecord{
				RunID: fmt.Sprint(201 + index), Repository: "davidpovarsky/cherri",
				HeadSHA: implSHA, CheckoutSHA: implSHA, Conclusion: "success", WorkflowName: workflow,
			})
		}
		return runs, nil
	}
	for _, test := range []struct {
		name, attacked string
		includeFN01    bool
	}{
		{name: "EmptyIngestion"},
		{name: "LinkedFN01DoesNotExecuteRepairOrGate", includeFN01: true},
		{name: "RetaggedFN01CannotExecuteRP24", includeFN01: true, attacked: "RP24"},
		{name: "RetaggedFN01CannotExecuteBRG15", includeFN01: true, attacked: "BRG15"},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			records := []EvidenceRecord{}
			credited := make(map[string][]string)
			if test.includeFN01 {
				records = p15FullClosureFN01Inputs(implSHA)
				credited["FN01"] = []string{"unit", "native-structure"}
			}
			var expectedAuthorizationErrors []string
			if test.attacked != "" {
				for _, record := range p15FullClosureFN01Inputs(implSHA) {
					record.EvidenceID = "synthetic-retag-" + test.attacked + "-" + record.Tier
					record.Requirements = []string{test.attacked}
					records = append(records, record)
					expectedAuthorizationErrors = append(expectedAuthorizationErrors,
						fmt.Sprintf("test_id %q in record %s is not authorized for requirement %s",
							record.TestID, record.EvidenceID, test.attacked))
					if test.attacked == "RP24" {
						// The distinct RP24 producer now also rejects a borrowed FN01
						// record at its exact command/event/scope binding boundary.
						expectedAuthorizationErrors = append(expectedAuthorizationErrors,
							"RP24 record is detached from its complete return/recursion scenario; actual Apple producer is still required")
					}
				}
			}
			raw, err := json.MarshalIndent(records, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			input := filepath.Join(dir, "synthetic-validator-inputs.json")
			if err := os.WriteFile(input, raw, 0600); err != nil {
				t.Fatal(err)
			}
			defaultEvidencePathsFunc = func() []string { return []string{input} }
			output := filepath.Join(dir, "final-manifest.json")
			if len(records) == 0 {
				// A declared empty file is rejected at ingestion. Assert that
				// precise boundary, then test final closure with no declared
				// evidence files. Neither path can acquire executed tiers.
				emptyManifest, emptyErr := BuildFinalEvidenceManifest(contracts, implSHA, docsSHA, output)
				want := fmt.Sprintf("failed to ingest declared evidence file %s: no evidence records parsed from %s", input, input)
				if emptyManifest != nil || emptyErr == nil || emptyErr.Error() != want {
					t.Fatalf("empty declared input must fail only at ingestion: manifest=%+v err=%v want=%q", emptyManifest, emptyErr, want)
				}
				if _, statErr := os.Stat(output); !os.IsNotExist(statErr) {
					t.Fatalf("rejected empty input wrote a manifest: %v", statErr)
				}
				defaultEvidencePathsFunc = func() []string { return nil }
			}
			manifest, err := BuildFinalEvidenceManifest(contracts, implSHA, docsSHA, output)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(manifest.Records, records) {
				t.Fatal("manifest ingestion changed the declared synthetic inputs")
			}
			written, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			var reloaded EvidenceManifest
			if err := json.Unmarshal(written, &reloaded); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(&reloaded, manifest) {
				t.Fatal("written manifest differs from the ingested manifest")
			}
			report, err := ValidateEvidenceManifest(&reloaded, contracts, "final")
			p15AssertFullClosureGaps(t, report, err, rows, credited, test.attacked, expectedAuthorizationErrors)
		})
	}
}
