package main

import (
	"strings"
	"testing"
)

func TestLoopControlProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "F05", loopControlScenario.TestNames, nil,
		(*Runner).runLoopControlRequirement, "P15_LOOP_CONTROL_RECEIPT:",
		"Actual parser and public compiler reject break/continue/while at top level and inside both supported loop forms, with exact keyword diagnostics and zero emitted native bytes. Supported loops and ordinary keyword text remain valid. The immutable F05 contract is unit-only; no Apple execution is claimed.")
}

func TestLoopControlEvidenceRejectsIncompleteSelection(t *testing.T) {
	record := EvidenceRecord{EvidenceID: "loop-control-input-control", Tier: "unit",
		TestID:     loopControlScenario.testIDs()[0],
		Command:    productionTestCommand(loopControlScenario.TestNames),
		Assertions: map[string]string{"required_production_tests": strings.Join(loopControlScenario.TestNames, "\n")},
	}
	if err := validateLoopControlRecord("F05", record); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"wrong_tier", "wrong_command", "partial_scenario"} {
		t.Run(mutation, func(t *testing.T) {
			broken := record
			broken.Assertions = map[string]string{"required_production_tests": strings.Join(loopControlScenario.TestNames, "\n")}
			switch mutation {
			case "wrong_tier":
				broken.Tier = "ios-runtime"
			case "wrong_command":
				broken.Command = []string{"go", "version"}
			case "partial_scenario":
				broken.Assertions["required_production_tests"] = loopControlScenario.TestNames[0]
			}
			if validateLoopControlRecord("F05", broken) == nil {
				t.Fatal("damaged loop-control record accepted")
			}
		})
	}
}
