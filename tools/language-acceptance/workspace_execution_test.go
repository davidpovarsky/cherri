package main

import "testing"

func TestWorkspaceResourceProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "IM04", workspaceProductionTests["IM04"], nil,
		(*Runner).runWorkspaceResourceRequirement, "P15_WORKSPACE_RESOURCE_RECEIPT:",
		"Actual local module compilation through rooted file opens; healthy file/extension/..name controls, real outside symlink or Windows junction, extension traversal and missing-dependency controls, with no outside content or resolved-path disclosure. No general module-resolution or project closure claim.")
}

func TestWorkspaceEvidenceRejectsUnrelatedCommands(t *testing.T) {
	tests := workspaceProductionTests["IM04"]
	record := EvidenceRecord{EvidenceID: "workspace-input-control", Tier: "security",
		TestID:     "github.com/electrikmilk/cherri:" + tests[0],
		Command:    productionTestCommand(tests),
		Assertions: map[string]string{"required_production_tests": tests[0]},
	}
	if err := validateWorkspaceRecord("IM04", record); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"wrong_tier", "wrong_command", "missing_scenario"} {
		broken := record
		broken.Assertions = map[string]string{"required_production_tests": tests[0]}
		switch mutation {
		case "wrong_tier":
			broken.Tier = "ios"
		case "wrong_command":
			broken.Command = []string{"go", "version"}
		case "missing_scenario":
			delete(broken.Assertions, "required_production_tests")
		}
		if validateWorkspaceRecord("IM04", broken) == nil {
			t.Errorf("accepted %s", mutation)
		}
	}
}
