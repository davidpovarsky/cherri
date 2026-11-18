package main

var typeProductionTests = map[string][]string{
	"T02": {"TestP15T02LiteralRequiredProduction"},
	"T06": {"TestP15T06RecordAnnotationsDoNotValidateExternalValues", "TestP15T06RecordProofTypeRelations"},
}

var typeProductionPaths = map[string][]string{
	"T02": {"internal/language/analysis/analyzer.go", "internal/language/analysis/literal_arguments.go", "internal/language/lower/lower.go", "language_emit_adapter.go", "p15_types_acceptance_test.go"},
	"T06": {"internal/language/types/types.go", "internal/language/types/record_evidence.go", "internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/syntax/parser.go", "p15_record_acceptance_test.go"},
}

func (r *Runner) runTypeLiteralRequirement() {
	r.runTypeProduction("T02", "Runtime immutable output does not satisfy literal-required slot")
}

func (r *Runner) runTypeRecordRequirement() {
	r.runTypeProduction("T06", "User record annotation is not runtime JSON validation")
}

func (r *Runner) runTypeProduction(id, requirement string) {
	tests := typeProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "types", requirement, "unit", []string{"unit"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
