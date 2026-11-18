package main

var documentationProductionTests = map[string][]string{
	"D01": {"TestP15D01CanonicalDocsProduction"},
	"D02": {"TestDocsExamplesExtractAndVerify"},
}
var documentationProductionPaths = map[string][]string{
	"D01": {"tools/language-docs/main.go", "tools/language-docs/p15_contract_test.go", "docs/language-v2/actions-reference.md", "p15_documentation_test.go"},
	"D02": {"docs/language-v2/language-guide.md", "docs_examples_test.go", "main.go", "language_cli_adapter.go", "language_emit_adapter.go"},
}

func (r *Runner) runDocumentationRequirement(id, requirement, level string) {
	tests := documentationProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "docs", requirement, level, parseRequiredTiers(level), passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
	}
}
func (r *Runner) runGeneratedDocumentationRequirement() {
	r.runDocumentationRequirement("D01", "All generated action signatures match assembled schema", "generation+docs")
}
func (r *Runner) runExecutableDocumentationRequirement() {
	r.runDocumentationRequirement("D02", "Runnable and deliberately-invalid docs examples tested", "docs")
}
