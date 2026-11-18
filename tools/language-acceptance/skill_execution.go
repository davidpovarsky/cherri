package main

// Real installs and isolated failures execute the production shell scripts with
// actual compiler binaries, controlled Git refs and byte-preservation assertions.
var skillProductionTests = map[string][]string{
	"K01": {"TestP15K01UnpackedPackageContract"},
	"K02": {"TestP15SkillFreshPinnedInstall", "TestP15K02ActualCacheAndBundleSelection"},
	"K03": {
		"TestP15SkillFreshPinnedInstall",
		"TestP15K03LateDocsFailurePreservesWorkingPair",
		"TestP15K03DownloadedChecksumBeforeExecution",
		"TestP15K03PromotionFailuresRestoreActualPair",
		"TestP15K03SuccessfulUpdateCommitsValidatedPair",
		"TestP15K03ImmutableInstallRefsRequired",
		"TestP15K03LocalStateIsPreserved",
		"TestP15K03HandledSignalRestoresFiles",
		"TestP15K03PromotedCacheCannotFallBack",
	},
	"K04": {"TestP15K04OfflineCacheProduction"},
}

var skillProductionPaths = map[string][]string{
	"K01": {"tools/p10r15/package_skill.py", "p15_skill_packaging_test.go", "skills/cherri-shortcuts/scripts/self-test.sh", ".github/workflows/openminis-skill.yml"},
	"K02": {"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/doctor.sh", "p15_skill_installation_test.go"},
	"K03": {"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/doctor.sh", "p15_skill_installation_test.go", "p15_skill_signal_test.go"},
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

// The packaging tier is emitted only after actual archive verification, unpacked
// execution and causal corruption tests. Generic Record intentionally omits it.
func (r *Runner) runSkillPackageRequirement() {
	tests := skillProductionTests["K01"]
	passed, detail := executeNamedProductionTests(tests)
	r.Record("K01", "skill", "Complete updated package and unpacked self-test", "packaging", []string{"skill-package"}, passed, "PASSED", detail)
	result, exitCode := "failed", 1
	if passed {
		result, exitCode = "passed", 0
	}
	r.records = append(r.records, EvidenceRecord{
		EvidenceID: "ev-local-K01-skill-package", Requirements: []string{"K01"},
		Tier: "skill-package", ImplementationSHA: r.implSHA,
		TestID:  "github.com/electrikmilk/cherri:" + tests[0],
		Command: productionTestCommand(tests), ExitCode: exitCode, Result: result,
		Assertions: map[string]string{"assertion": detail}, Detail: detail,
	})
}
