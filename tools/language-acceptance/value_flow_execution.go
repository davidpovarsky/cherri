package main

var valueFlowProductionTests = map[string][]string{
	"F03": {
		"TestP15F03YieldTypesAndNormalPaths",
		"TestP15F03TerminatedPathsDoNotBecomeValues",
		"TestP15F03YieldDiagnosticsProduction",
		"TestP15F03ImplicitResultBoundary",
		"TestP15F03NativeBranchResults",
		"TestP15F03WireOracleMutations",
		"TestP15F03CLIAndService",
		"TestP15F03NoneYieldClearsPrecedingOutput",
		"TestP15F03LiteralYieldNativeFields",
	},
}

var valueFlowProductionPaths = []string{
	"internal/language/analysis/analyzer.go",
	"internal/language/analysis/control_results.go",
	"internal/language/analysis/value_yields.go",
	"internal/language/lower/lower.go",
	"language_emit_adapter.go",
	"p15_value_flow_test.go",
	"p15_value_flow_native_test.go",
}

func (r *Runner) runValueBranchRequirement() {
	tests := valueFlowProductionTests["F03"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("F03", "control", "Value if/menu yield typing and normal-path coverage", "unit+native", []string{"unit", "native-structure"}, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
		if r.records[index].Tier == "native-structure" {
			r.records[index].TestID = "github.com/electrikmilk/cherri:TestP15F03NativeBranchResults"
		}
	}
}
