package main

var callProductionTests = map[string][]string{
	"C02": {"TestP15C02ArgumentDiagnosticsProduction", "TestP15C02AcceptedBindingsProduction", "TestP15C02ArgumentServiceAndCLI"},
}

var callProductionPaths = map[string][]string{
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
