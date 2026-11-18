package main

var bindingProductionTests = map[string][]string{
	"V02": {
		"TestP15V02BindingFlowProduction",
		"TestP15V02BindingScopeService",
		"TestP15V02BindingServiceAndCLI",
		"TestP15V02ParameterScopes",
		"TestP15V02LexicalWire",
		"TestP15V02ImportedScopeIdentity",
		"TestP15V02RepeatedImportedLoopIdentity",
	},
}

var bindingProductionPaths = map[string][]string{
	"V02": {
		"internal/language/analysis/analyzer.go",
		"internal/language/analysis/binding_flow.go",
		"internal/language/lower/lower.go",
		"internal/language/lower/lexical_scope.go",
		"internal/language/service/service.go",
		"language_cli_adapter.go",
		"p15_binding_acceptance_test.go",
		"p15_binding_service_test.go",
		"p15_binding_scope_wire_test.go",
	},
}

func (r *Runner) runBindingScopeRequirement() {
	tests := bindingProductionTests["V02"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("V02", "binding", "Scope, shadowing, immutable reassignment and read-before-init", "unit", []string{"unit"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
	}
}
