package main

var callProductionTests = map[string][]string{
	"C01": {"TestP15C01SourceOrderFlow", "TestP15C01SourceOrderNativeStructure", "TestP15C01DefaultEffectsOnce", "TestP15C01DefaultGuardStructure", "TestP15C01ArgumentValueSnapshot", "TestP15C01SnapshotBoundaries", "TestP15C01NestedCustomResult", "TestP15C01AggregateArgumentSnapshots", "TestP15C01CLIAndService"},
	"C04": {"TestP15C04EnumResolutionProduction", "TestP15C04EnumServiceAndCLI", "TestP15C04EnumContextsAndShadowing", "TestP15C04EnumFunctionService", "TestP15C04ProvidedRegistryEnumDomain"},
	"C02": {"TestP15C02ArgumentDiagnosticsProduction", "TestP15C02AcceptedBindingsProduction", "TestP15C02ArgumentServiceAndCLI"},
}

var callProductionPaths = map[string][]string{
	"C01": {"internal/language/analysis/function_arguments.go", "internal/language/analysis/binding_flow.go", "internal/language/lower/call_arguments.go", "internal/language/lower/lower.go", "internal/language/service/service.go", "language_cli_adapter.go", "p15_call_order_test.go", "p15_call_order_oracle_test.go"},
	"C04": {"internal/language/enums/catalog.go", "internal/language/syntax/parser.go", "internal/language/syntax/format.go", "internal/language/analysis/enums.go", "internal/language/analysis/function_arguments.go", "internal/language/lower/enum_values.go", "internal/language/lower/lower.go", "language_emit_adapter.go", "internal/language/service/enum_completion.go", "p15_enum_acceptance_test.go", "p15_enum_registry_test.go"},
	"C02": {"internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/schema/schema.go", "internal/language/lower/lower.go", "internal/language/service/service.go", "language_cli_adapter.go", "p15_call_acceptance_test.go"},
}

func (r *Runner) runCallArgumentRequirement() {
	tests := callProductionTests["C02"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("C02", "calls", "Duplicate, missing, unknown labels and extra positional arguments", "unit", []string{"unit"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}

func (r *Runner) runEnumRequirement() {
	tests := callProductionTests["C04"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("C04", "calls", "Enum contextual and qualified members resolve", "unit+service", []string{"unit", "service"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
		if r.records[index].Tier == "service" {
			r.records[index].TestID = "github.com/electrikmilk/cherri:" + tests[1]
		}
	}
}

func (r *Runner) runCallOrderRequirement() {
	tests := callProductionTests["C01"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("C01", "calls", "Named arguments out of declaration order evaluate in source order", "unit+native", []string{"unit", "native-structure"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
		if r.records[index].Tier == "native-structure" {
			r.records[index].TestID = "github.com/electrikmilk/cherri:TestP15C01SourceOrderNativeStructure"
		}
	}
}
