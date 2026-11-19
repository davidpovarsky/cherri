package main

var literalCollectionProductionTests = map[string][]string{
	"L02": {"TestP15L02IndexBoundaryWire", "TestP15L04IndexOracleRejectsCorruption", "TestP15L02L04IndexNativeRoundTrip"},
	"L04": {"TestP15L04DynamicFractionalIndexGuard", "TestP15L04IndexOracleRejectsCorruption", "TestP15L02L04IndexNativeRoundTrip"},
	"S01": {"TestP15S01ExactUTF16TokensAndLiteralBytes", "TestP15S01UnicodeOracleRejectsCorruption", "TestP15S01UnicodeNativeRoundTrip"},
	"S02": {"TestP15S02ExactLiteralBytes", "TestP15S02ExplicitInterpolationAndLegacyBoundary", "TestP15S02LiteralOracleRejectsCorruption", "TestP15S02LiteralNativeRoundTrip"},
	"L03": {"TestP15L03DuplicateAndInvalidIndexDiagnostics"},
}
var literalCollectionProductionPaths = map[string][]string{
	"L02": {"internal/language/lower/lower.go", "internal/language/lower/index_bounds.go", "internal/language/backend/control.go", "language_backend_adapter.go", "p15_indexing_acceptance_test.go", "decompile.go", "import_fidelity.go", "semantic_import_planner.go"},
	"L04": {"internal/language/lower/lower.go", "internal/language/lower/index_bounds.go", "internal/language/backend/control.go", "language_backend_adapter.go", "p15_indexing_acceptance_test.go", "decompile.go", "import_fidelity.go", "semantic_import_planner.go"},
	"S01": {"internal/language/syntax/lexer.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "language_typed_literal_adapter.go", "shared_codec.go", "decompile.go", "import_fidelity.go", "semantic_import_planner.go", "p15_unicode_text_acceptance_test.go"},
	"S02": {"internal/language/syntax/lexer.go", "internal/language/lower/lower.go", "internal/language/lower/literal_list.go", "language_backend_adapter.go", "language_typed_literal_adapter.go", "typed_array_value.go", "shared_codec.go", "p15_literal_collections_acceptance_test.go", "p15_literal_codec_controls_test.go"},
	"L03": {"internal/language/analysis/analyzer.go", "internal/language/service/service.go", "p15_literal_collections_acceptance_test.go"},
}
var nestedCollectionObservationTests = []string{
	"TestP15L01NestedDictionaryShape",
	"TestP15L01NestedDictionaryOracleRejectsCorruption",
	"TestP15L01UnverifiedNativeNestedLiteralFailsClosed",
	"TestP15L01InferredNestedNativeListFailsClosed",
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
		detail+"; explicit Dictionary arrays preserve empty/false/zero/text/map shape. Direct and inferred List-valued native rows fail closed; dynamic AnyContent and standalone native List<List> runtime hierarchy remain UNVERIFIED.")
}

// Only an actually executed producer can supply each S01 tier. A native tree
// round trip cannot manufacture the separate observable iOS text result.
func literalCollectionProductionTestForTier(id, tier string) string {
	if id == "L02" || id == "L04" {
		switch tier {
		case "unit":
			if id == "L02" {
				return "github.com/electrikmilk/cherri:TestP15L02IndexBoundaryWire"
			}
		case "native-roundtrip":
			return "github.com/electrikmilk/cherri:TestP15L02L04IndexNativeRoundTrip"
		case "ios-runtime":
			return "ios27-runtime-poc:indexing"
		}
		return ""
	}
	if id != "S01" {
		return ""
	}
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + literalCollectionProductionTests[id][0]
	case "native-roundtrip":
		return "github.com/electrikmilk/cherri:" + literalCollectionProductionTests[id][2]
	case "ios-runtime":
		return "ios27-runtime-poc:coverage-s01"
	}
	return ""
}

func (r *Runner) runUnicodeTextRequirement() {
	tests := literalCollectionProductionTests["S01"]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record("S01", "text", "Multiple variable tokens after Hebrew/emoji/combining marks", "unit+native-roundtrip+iOS",
		[]string{"unit", "native-roundtrip"}, passed, "PASSED", detail+"; no iOS execution or observable native text result is claimed")
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
		r.records[i].TestID = literalCollectionProductionTestForTier("S01", r.records[i].Tier)
	}
}

// runIndexRequirement records only executed canonical wire and import proofs.
// The separate Apple runtime result remains required by both immutable cases.
func (r *Runner) runIndexRequirement(id string) {
	tests := literalCollectionProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	title, level := "Zero-based first/last/empty/out-of-range/negative indexing", "unit+native+iOS"
	tiers := []string{"unit", "native-roundtrip"}
	if id == "L04" {
		title, level = "Dynamic bounds checking does not execute unsafe native lookup", "native+iOS"
		tiers = []string{"native-roundtrip"}
	}
	first := len(r.records)
	r.Record(id, "collections", title, level, tiers, passed, "PASSED",
		detail+"; exact native control ancestry and input/result references verified; Apple runtime values and no-item behavior remain pending")
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
		r.records[i].TestID = literalCollectionProductionTestForTier(id, r.records[i].Tier)
	}
}
