package main

import (
	"fmt"
	"reflect"
	"strings"
)

var loopControlScenario = supplementalProductionScenario{
	PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
	TestNames: []string{
		"TestP15F05UnsupportedLoopControlProduction",
		"TestP15F05SupportedLoopsAndKeywordText",
	},
	Tiers: []string{"unit"},
	ProductionPaths: []string{
		"internal/language/syntax/parser.go", "language_emit_adapter.go",
		"p15_loop_control_acceptance_test.go",
	},
}

func (r *Runner) runLoopControlRequirement() {
	passed, detail := executeNamedProductionTests(loopControlScenario.TestNames)
	first := len(r.records)
	r.Record("F05", "control", "Stop Shortcut is not substituted for break", "unit",
		loopControlScenario.Tiers, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].TestID = loopControlScenario.testIDs()[0]
		r.records[i].Command = productionTestCommand(loopControlScenario.TestNames)
		r.records[i].Assertions["required_production_tests"] = strings.Join(loopControlScenario.TestNames, "\n")
	}
}

func validateLoopControlRecord(id string, record EvidenceRecord) error {
	if id != "F05" {
		return nil
	}
	if record.Tier != "unit" || record.TestID != loopControlScenario.testIDs()[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommand(loopControlScenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(loopControlScenario.TestNames, "\n") {
		return fmt.Errorf("record %s is detached from actual unsupported loop-control production scenarios", record.EvidenceID)
	}
	return nil
}
