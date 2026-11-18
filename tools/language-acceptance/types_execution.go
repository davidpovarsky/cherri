package main

var typeProductionTests = map[string][]string{
	"T02": {"TestP15T02LiteralRequiredProduction"},
}

func (r *Runner) runTypeLiteralRequirement() {
	tests := typeProductionTests["T02"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("T02", "types", "Runtime immutable output does not satisfy literal-required slot", "unit", []string{"unit"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
