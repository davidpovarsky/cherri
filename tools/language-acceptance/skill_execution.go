package main

// Real installs and isolated failures execute the production shell scripts with
// actual compiler binaries, controlled Git refs and byte-preservation assertions.
var skillProductionTests = map[string][]string{
	"K02": {"TestP15SkillFreshPinnedInstall", "TestP15K02ActualCacheAndBundleSelection"},
	"K03": {
		"TestP15SkillFreshPinnedInstall",
		"TestP15K03LateDocsFailurePreservesWorkingPair",
		"TestP15K03DownloadedChecksumBeforeExecution",
		"TestP15K03PromotionFailuresRestoreActualPair",
		"TestP15K03SuccessfulUpdateCommitsValidatedPair",
		"TestP15K03ImmutableInstallRefsRequired",
		"TestP15K03LocalStateIsPreserved",
	},
	"K04": {"TestP15K04OfflineCacheProduction"},
}

var skillProductionPaths = map[string][]string{
	"K02": {"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/doctor.sh", "p15_skill_installation_test.go"},
	"K03": {"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/doctor.sh", "p15_skill_installation_test.go"},
	"K04": {"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/doctor.sh", "skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh", "p15_skill_acceptance_test.go"},
}

func (r *Runner) runSkillProductionRequirement(id, requirement string) {
	tests := skillProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "skill", requirement, "integration", []string{"integration"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}

func (r *Runner) runSkillInstallRequirement() {
	r.runSkillProductionRequirement("K02", "Fresh install and incompatible cached/bundled binary")
}
func (r *Runner) runSkillUpdateRequirement() {
	r.runSkillProductionRequirement("K03", "Exact commit install and checksum/atomic update")
}
func (r *Runner) runSkillOfflineRequirement() {
	r.runSkillProductionRequirement("K04", "Offline compatible cache and docs mismatch")
}
