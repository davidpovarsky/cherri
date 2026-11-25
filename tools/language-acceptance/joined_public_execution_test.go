package main

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// These are intake/metadata controls only. They do not execute a compiler,
// fabricate production evidence, or grant any native requirement credit.
func joinedPublicContractForTest(t *testing.T, id string) RecoveryGate {
	t.Helper()
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, g := range all.Gates.Gates {
		if g.ID == id {
			return g
		}
	}
	t.Fatal("immutable gate absent", id)
	return RecoveryGate{}
}
func TestJoinedPublicInventoryMatchesImmutableContracts(t *testing.T) {
	counts := map[string]map[string]int{"BRG25": {"routes": 58}, "BRG27": {"lsp": 18, "mobile": 18}}
	for id, spec := range joinedPublicScenarios {
		t.Run(id, func(t *testing.T) {
			if !spec.matches(joinedPublicContractForTest(t, id)) {
				t.Fatal("joined producer changed its original requirement")
			}
			if len(spec.testIDs()) != len(spec.tests()) {
				t.Fatal("producer identity inventory differs")
			}
			for _, step := range spec.Steps {
				if len(step.Events) != counts[id][step.Name] || !sort.StringsAreSorted(step.Events) {
					t.Fatal("exact named child inventory changed", id, step.Name)
				}
				seen := map[string]bool{}
				for _, event := range step.Events {
					if seen[event] {
						t.Fatal("duplicate required event")
					}
					seen[event] = true
					owned := false
					for _, parent := range step.Tests {
						if !strings.HasPrefix(parent, spec.Prefix) {
							t.Fatal("foreign test family")
						}
						owned = owned || event == parent || strings.HasPrefix(event, parent+"/")
					}
					if !owned {
						t.Fatal("foreign required child", event)
					}
				}
				for _, parent := range step.Tests {
					if !seen[parent] {
						t.Fatal("missing parent")
					}
				}
			}
			for _, tier := range spec.LocalTiers {
				step, found := spec.tierStep(tier)
				if !found || len(step.Tests) == 0 {
					t.Fatal("local tier has no exact command")
				}
				if tier == "ios-build" || tier == "ios-ui" {
					t.Fatal("Go producer claimed native tier")
				}
			}
			paths := joinedPublicSourcePaths(id)
			if len(paths) == 0 || !sort.StringsAreSorted(paths) {
				t.Fatal("unsorted or empty immutable source inventory")
			}
			for i := 1; i < len(paths); i++ {
				if paths[i] == paths[i-1] {
					t.Fatal("duplicate source path")
				}
			}
		})
	}
}
func TestJoinedPublicGateCannotCloseFromLocalTiers(t *testing.T) {
	for id, spec := range joinedPublicScenarios {
		t.Run(id, func(t *testing.T) {
			runner := &Runner{results: map[string]CaseResult{}}
			runner.Record(id, "recovery_gate", spec.Title, strings.Join(spec.RequiredTiers, "+"), spec.LocalTiers, true, "PASSED", "metadata-only control")
			got := runner.results[id]
			if got.Passed || got.Status != "PENDING_EXTERNAL" || !reflect.DeepEqual(got.ExecutedLevels, spec.LocalTiers) {
				t.Fatalf("local metadata closed joined native requirement: %+v", got)
			}
			for _, record := range runner.records {
				if record.Tier == "ios-build" || record.Tier == "ios-ui" {
					t.Fatal("native record was manufactured")
				}
			}
		})
	}
}
func TestJoinedPublicRequirementRejectsChangedContract(t *testing.T) {
	for id := range joinedPublicScenarios {
		for _, fault := range []string{"title", "scenario", "expected", "tiers", "prefix", "mandatory"} {
			t.Run(id+"/"+fault, func(t *testing.T) {
				gate := joinedPublicContractForTest(t, id)
				switch fault {
				case "title":
					gate.Title += " changed"
				case "scenario":
					gate.Scenario += " changed"
				case "expected":
					gate.ExpectedObservation += " changed"
				case "tiers":
					gate.MinimumEvidenceTiers = []string{"integration"}
				case "prefix":
					gate.ProposedTestPrefix = "TestUnrelated"
				case "mandatory":
					gate.Mandatory = false
				}
				runner := &Runner{results: map[string]CaseResult{}}
				if !runner.runJoinedPublicRequirement(gate) || runner.results[id].Status != "FAILED" || len(runner.records) != 0 {
					t.Fatal("changed contract reached production or gained a record")
				}
			})
		}
	}
}
func TestJoinedPublicRecordRejectsDetachedScope(t *testing.T) {
	for id, spec := range joinedPublicScenarios {
		for _, tier := range spec.LocalTiers {
			step, _ := spec.tierStep(tier)
			original := EvidenceRecord{Requirements: []string{id}, Tier: tier, TestID: step.PackageID + ":" + step.Tests[0],
				Command: editorContractCommand(step), ImplementationSHA: strings.Repeat("a", 40), ExitCode: 0, Result: "passed",
				Assertions: map[string]string{"joined_public_contract": spec.binding(), "scope": spec.Scope}}
			if err := validateJoinedPublicRecord(id, original, nil); err == nil || !strings.Contains(err.Error(), "complete state process artifact set") {
				t.Fatalf("plausible metadata must still require real originals: %v", err)
			}
			for _, fault := range []string{"result", "exit", "requirement", "producer", "command", "scope", "inventory", "tier", "native_relabel"} {
				t.Run(id+"/"+tier+"/"+fault, func(t *testing.T) {
					var changed EvidenceRecord
					raw, _ := json.Marshal(original)
					if err := json.Unmarshal(raw, &changed); err != nil {
						t.Fatal(err)
					}
					switch fault {
					case "result":
						changed.Result = "failed"
					case "exit":
						changed.ExitCode = 1
					case "requirement":
						changed.Requirements = append(changed.Requirements, "E01")
					case "producer":
						changed.TestID += "/unselected"
					case "command":
						changed.Command = append(changed.Command, "-short")
					case "scope":
						changed.Assertions["scope"] = "partial probe"
					case "inventory":
						delete(changed.Assertions, "joined_public_contract")
					case "tier":
						changed.Tier = "repository"
					case "native_relabel":
						changed.Tier = "ios-ui"
					}
					if err := validateJoinedPublicRecord(id, changed, nil); err == nil {
						t.Fatal("detached metadata accepted")
					}
				})
			}
		}
	}
}
