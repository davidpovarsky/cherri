package main

import (
	"fmt"
	"reflect"
	"strings"
)

type numberFunctionScenario struct {
	Group, Title, Scope, RuntimeTest string
	UnitTest, NativeTest             string
	AdditionalTests, ProductionPaths []string
}

var numberFunctionScenarios = map[string]numberFunctionScenario{
	"N01": {
		Group: "numbers", Title: "Fractional arithmetic including 5/2 and constraints",
		Scope:       "Actual fractional arithmetic and compound-assignment native dataflow, exact numeric diagnostics, rejection before emission, and healthy recovery",
		RuntimeTest: "ios27-runtime-poc:numbers",
		UnitTest:    "TestP15N01NumericConstraints", NativeTest: "TestP15N01FractionalNativeDataflow",
		ProductionPaths: []string{
			"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "shortcutgen.go",
			"p15_numeric_fraction_acceptance_test.go", "p15_types_acceptance_test.go",
			"p15_short_circuit_test.go", "p15_call_order_test.go",
		},
	},
	"FN01": {
		Group: "functions", Title: "AST-defined function call returns then caller continues",
		Scope:       "Actual AST function typing and rejection before emission; complete native argument, local-return and caller-continuation graph with causal corruptions",
		RuntimeTest: "ios27-runtime-poc:functions",
		UnitTest:    "TestP15FN01FunctionTyping", NativeTest: "TestP15FN01ReturnAndContinuationNative",
		ProductionPaths: []string{
			"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/call_arguments.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "shortcutgen.go",
			"p15_function_acceptance_test.go", "p15_function_native_oracle_test.go",
			"p15_types_acceptance_test.go", "p15_short_circuit_test.go", "p15_call_order_test.go", "p15_literal_collections_acceptance_test.go",
		},
	},
	"FN02": {
		Group: "functions", Title: "Presence-tagged argument defaults include false/0/empty",
		Scope:       "Actual typed defaults, supplied false/zero/empty versus omission, presence-guard selection through native return and caller consumer, plus existing typed envelope and guard controls",
		RuntimeTest: "ios27-runtime-poc:functions",
		UnitTest:    "TestP15FN02DefaultTyping", NativeTest: "TestP15FN02PresenceDefaultsNative",
		AdditionalTests: []string{"TestP15T01FunctionPresenceEnvelope", "TestP15T01DefaultsGuardedByPresence"},
		ProductionPaths: []string{
			"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/call_arguments.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "shortcutgen.go",
			"p15_function_acceptance_test.go", "p15_function_native_oracle_test.go", "p15_binding_defaults_acceptance_test.go",
			"p15_types_acceptance_test.go", "p15_short_circuit_test.go", "p15_call_order_test.go", "p15_literal_collections_acceptance_test.go",
			"tests/runtime_poc/CherriTypedDefaultsPOC.cherri",
		},
	},
	"FN03": {
		Group: "functions", Title: "No implicit runtime capture; terminating recursive call",
		Scope:       "Actual exact capture diagnostics without emission, explicit parameter control and recursive AST types; complete native base-case, decreasing recursive argument, typed product and final consumer graph",
		RuntimeTest: "ios27-runtime-poc:functions",
		UnitTest:    "TestP15FN03CaptureAndRecursionTyping", NativeTest: "TestP15FN03TerminatingRecursionNative",
		ProductionPaths: []string{
			"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/call_arguments.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "shortcutgen.go",
			"p15_function_acceptance_test.go", "p15_function_native_oracle_test.go",
			"p15_types_acceptance_test.go", "p15_short_circuit_test.go", "p15_call_order_test.go", "p15_literal_collections_acceptance_test.go",
		},
	},
}

func (scenario numberFunctionScenario) tests() []string {
	return append([]string{scenario.UnitTest, scenario.NativeTest}, scenario.AdditionalTests...)
}

func (scenario numberFunctionScenario) testIDs() []string {
	var result []string
	for _, test := range scenario.tests() {
		result = append(result, "github.com/electrikmilk/cherri:"+test)
	}
	return result
}

func (scenario numberFunctionScenario) producer(tier string) string {
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + scenario.UnitTest
	case "native-structure":
		return "github.com/electrikmilk/cherri:" + scenario.NativeTest
	case "ios-runtime":
		return scenario.RuntimeTest
	}
	return ""
}

func (r *Runner) runNumberFunctionRequirement(id string) {
	var scenario, present = numberFunctionScenarios[id]
	if !present {
		return
	}
	var tests = scenario.tests()
	var passed, detail = executeNamedProductionTests(tests)
	var first = len(r.records)
	r.Record(id, scenario.Group, scenario.Title, "unit+native+iOS", []string{"unit", "native-structure"}, passed, "PASSED",
		detail+"; "+scenario.Scope+"; actual Apple values remain separately required")
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(tests)
		record.Assertions["required_production_tests"] = strings.Join(tests, "\n")
		record.Assertions["scope"] = scenario.Scope
	}
}

func validateNumberFunctionRecord(id string, record EvidenceRecord) error {
	var scenario, present = numberFunctionScenarios[id]
	if !present {
		return nil
	}
	var producer = scenario.producer(record.Tier)
	if producer == "" || record.TestID != producer {
		return fmt.Errorf("numeric/function record %s substitutes an unexecuted producer or tier for %s", record.EvidenceID, id)
	}
	if record.Tier == "ios-runtime" {
		// Existing source-bound runtime intake validates the real Apple artifacts
		// and observations. A local graph or diagnostic never supplies this tier.
		return nil
	}
	var tests = scenario.tests()
	if !reflect.DeepEqual(record.Requirements, []string{id}) ||
		!reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(tests, "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("numeric/function record %s is detached from its complete executed %s scenario", record.EvidenceID, id)
	}
	return nil
}
