package main

import (
	"fmt"
	"reflect"
)

var workspaceProductionTests = map[string][]string{
	"IM04": {"TestP15IM04ActualWorkspaceResourceBoundaries"},
}
var workspaceProductionPaths = []string{
	"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go",
	"internal/language/modules/graph.go", "internal/language/workspace/resource.go",
	"internal/language/service/module_analysis.go", "language_emit_adapter.go", "p15_workspace_module_test.go",
}

func (r *Runner) runWorkspaceResourceRequirement() {
	tests := workspaceProductionTests["IM04"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("IM04", "modules", "Local workspace resource traversal and symlink escape", "security", []string{"security"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
		r.records[index].Assertions["required_production_tests"] = tests[0]
	}
}

func validateWorkspaceRecord(id string, record EvidenceRecord) error {
	tests, found := workspaceProductionTests[id]
	if !found {
		return nil
	}
	if record.Tier != "security" || record.TestID != "github.com/electrikmilk/cherri:"+tests[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
		record.Assertions["required_production_tests"] != tests[0] {
		return fmt.Errorf("workspace record %s is detached from the actual rooted-file production scenario", record.EvidenceID)
	}
	return nil
}
