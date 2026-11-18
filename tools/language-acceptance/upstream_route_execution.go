package main

var upstreamRouteProductionTests = map[string][]string{
	"U04": {"TestP15U04PublicFrontendRouteMatrix", "TestP15CLIDefaultRejectsLegacyWithGuidance"},
}

var upstreamRouteProductionPaths = []string{
	"main.go", "args.go", "language_cli_adapter.go", "p15_public_parser_route_test.go", "p15_cli_acceptance_test.go",
}

func (r *Runner) runPublicParserRouteRequirement() {
	tests := upstreamRouteProductionTests["U04"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	detail += "; explicit P10R15 contract: default V2 without implicit Legacy fallback; opt-in Legacy preserved under mandate E3/F1 and D005"
	r.Record("U04", "upstream", "Check normal public binary does not invoke a legacy script parser",
		"integration", []string{"integration"}, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
