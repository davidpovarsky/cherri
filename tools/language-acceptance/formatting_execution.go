package main

import (
	"fmt"
	"reflect"
	"strings"
)

var formattingProductionTests = map[string][]string{
	"E04": {"TestP15E04SyntaxAndServiceProduction", "TestP15E04ActualCLIAndServiceNativeIdentity", "TestP15E04FormatterOracleRejectsChangedValuesAndComments"},
}
var formattingProductionPaths = []string{
	"internal/language/syntax/format.go", "internal/language/syntax/format_literals.go",
	"internal/language/service/service.go", "language_cli_adapter.go",
	"internal/language/syntax/p15_formatter_preservation_test.go",
	"internal/language/service/p15_formatter_preservation_test.go", "p15_formatter_acceptance_test.go",
}

func (r *Runner) runFormattingRequirement() {
	tests := formattingProductionTests["E04"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("E04", "editor", "Formatter idempotence and literal/comment preservation", "unit+integration", []string{"unit", "integration"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		producer := tests[0]
		if r.records[index].Tier == "integration" {
			producer = tests[1]
		}
		r.records[index].TestID = "github.com/electrikmilk/cherri:" + producer
		r.records[index].Command = productionTestCommand(tests)
		r.records[index].Assertions["required_production_tests"] = strings.Join(tests, "\n")
	}
}

func validateFormattingRecord(id string, record EvidenceRecord) error {
	tests, found := formattingProductionTests[id]
	if !found {
		return nil
	}
	producer := tests[0]
	switch record.Tier {
	case "unit":
	case "integration":
		producer = tests[1]
	default:
		return fmt.Errorf("formatter record %s claims unexecuted tier %s", record.EvidenceID, record.Tier)
	}
	if record.TestID != "github.com/electrikmilk/cherri:"+producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(tests, "\n") {
		return fmt.Errorf("formatter record %s is detached from its complete executed production scenario", record.EvidenceID)
	}
	return nil
}
