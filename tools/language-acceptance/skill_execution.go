package main

// The actual compiler exercises healthy cached setup, isolated docs/commit
// faults, cache rejection, missing docs, and observable offline network denial.
var skillProductionTests = map[string][]string{
	"K04": {"TestP15K04OfflineCacheProduction"},
}

func (r *Runner) runSkillOfflineRequirement() {
	tests := skillProductionTests["K04"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("K04", "skill", "Offline compatible cache and docs mismatch", "integration", []string{"integration"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
