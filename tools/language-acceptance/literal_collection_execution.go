package main

var literalCollectionProductionTests = map[string][]string{
	"S02": {"TestP15S02ExactLiteralBytes", "TestP15S02ExplicitInterpolationAndLegacyBoundary", "TestP15S02LiteralOracleRejectsCorruption", "TestP15S02LiteralNativeRoundTrip"},
	"L03": {"TestP15L03DuplicateAndInvalidIndexDiagnostics"},
}
var literalCollectionProductionPaths = map[string][]string{
	"S02": {"internal/language/syntax/lexer.go", "internal/language/lower/lower.go", "internal/language/lower/literal_list.go", "language_backend_adapter.go", "language_typed_literal_adapter.go", "typed_array_value.go", "shared_codec.go", "p15_literal_collections_acceptance_test.go", "p15_literal_codec_controls_test.go"},
	"L03": {"internal/language/analysis/analyzer.go", "internal/language/service/service.go", "p15_literal_collections_acceptance_test.go"},
}
var nestedCollectionObservationTests = []string{
	"TestP15L01NestedDictionaryShape",
	"TestP15L01NestedDictionaryOracleRejectsCorruption",
	"TestP15L01UnverifiedNativeNestedLiteralFailsClosed",
}

func (r *Runner) runLiteralCollectionRequirement(id string) {
	group, title, level := "text", "Raw regex and ordinary JSON/JS/CSS text keep braces/backslashes", "unit+native"
	if id == "L03" {
		group, title, level = "collections", "Duplicate literal keys and invalid index type", "unit"
	}
	tests := literalCollectionProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, group, title, level, parseRequiredTiers(level), passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}

func (r *Runner) runNestedCollectionObservation() {
	passed, detail := executeNamedProductionTests(nestedCollectionObservationTests)
	// L01 intentionally stays in incompleteLocalAssertions. These are genuine
	// production Dictionary-field observations, not native List<List> proof.
	r.Record("L01", "collections", "Nested lists/maps preserve shape, cardinality and element order", "unit+native", nil, passed, "PASSED",
		detail+"; explicit Dictionary arrays preserve empty/false/zero/text/map shape. Standalone native List<List> representation and runtime hierarchy remain UNVERIFIED.")
}
