package main

// These producers reconcile identities and capture actual serialized outputs.
// They do not import a historical PASS ledger or infer native device execution.
var catalogProductionTests = map[string][]string{
	"B02":  {"TestP15B02AC01CatalogMigrationAndNativeOutputs", "TestP15B02AC01RejectsCatalogIdentityFaults", "TestP15B02AC01RejectsNativeCaptureFaults"},
	"AC01": {"TestP15B02AC01CatalogMigrationAndNativeOutputs", "TestP15B02AC01RejectsCatalogIdentityFaults", "TestP15B02AC01RejectsNativeCaptureFaults", "TestP15AC01ConstructGrammarAudit", "TestP15U02GeneratedSchemaProduction"},
}

var catalogProductionPaths = []string{"actions", "actions_catalog.go", "internal/language/schema/generated_registry.go", "tools/language-schema/main.go", "p10_catalog_execution_test.go", "p15_catalog_acceptance_test.go"}

func (r *Runner) runCatalogBaselineRequirement() {
	tests := catalogProductionTests["B02"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("B02", "baseline", "Capture baseline catalog, definition identities and representative native outputs", "repository", []string{"repository"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}

func (r *Runner) runCatalogAccountingRequirement() {
	tests := catalogProductionTests["AC01"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("AC01", "schema", "All baseline definitions/variants/constructs accounted for", "generation", []string{"generation"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
