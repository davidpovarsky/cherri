package main

import "os"

func init() {
	skillProductionTests["K05"] = []string{
		"TestP15K05LiveSigning",
		"TestP15K05BuildPreservesExistingFiles",
		"TestP15K05PreparePreservesOriginal",
		"TestP15K05RepairBuildImportEdit",
		"TestP15K05SigningGateAndExactInput",
		"TestP15K05PrepareRequiresComparator",
	}
	skillProductionPaths["K05"] = []string{
		"skills/cherri-shortcuts/scripts/prepare-edit.sh",
		"skills/cherri-shortcuts/scripts/decompile.sh",
		"skills/cherri-shortcuts/scripts/build.sh",
		"edit_cli_adapter.go", "internal/shortcutcompare/edit.go",
		"p15_skill_edit_test.go", "p15_skill_edit_zlive_test.go",
	}
}

func (r *Runner) runSkillEditRequirement() {
	const requirement = "Structured repair/build/import/edit/sign workflow"
	if os.Getenv("CHERRI_K05_LIVE_SIGNING") == "1" {
		r.runSkillProductionRequirement("K05", requirement)
		return
	}
	tests := skillProductionTests["K05"][1:]
	passed, detail := executeNamedProductionTests(tests)
	if !passed {
		r.Record("K05", "skill", requirement, "integration", []string{"integration"}, false, "FAILED", detail)
		return
	}
	gap := "Actual configured signing service has not executed in this run; CHERRI_K05_LIVE_SIGNING=1 is required. Local policy/repair/request-failure tests do not prove signed output."
	r.observations = append(r.observations, LocalSmokeObservation{ID: "K05", Passed: true, Detail: detail, CoverageGap: gap})
	r.Record("K05", "skill", requirement, "integration", nil, false, "PENDING_EXTERNAL", detail+"; "+gap)
}
