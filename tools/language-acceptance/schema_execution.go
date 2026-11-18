package main

var schemaProductionTests = map[string][]string{
	"U02": {"TestP15U02GeneratedSchemaProduction"},
	"U03": {"TestUpstreamPropagationRehearsal"},
}

func (r *Runner) runSchemaPropagationRequirement() {
	tests := schemaProductionTests["U03"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("U03", "upstream", "Simulate an upstream parameter change used by a facet", "integration", []string{"integration"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}

func (r *Runner) runSchemaGenerationRequirement() {
	tests := schemaProductionTests["U02"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("U02", "upstream", "Regenerate assembled schema from original declarations and narrow facets", "unit+generation", []string{"unit", "generation"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
