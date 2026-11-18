package main

import "strings"

var typeProductionTests = map[string][]string{
	"T03": {"TestP15T03NumberStringSemanticSignature", "TestP15T03NumberStringNativeWire"},
	"T02": {"TestP15T02LiteralRequiredProduction"},
	"T06": {"TestP15T06RecordAnnotationsDoNotValidateExternalValues", "TestP15T06RecordProofTypeRelations", "TestP15T06ControlResultsRequireAllBranches"},
	"T08": {"TestP15T08ConditionalCardinalityProduction", "TestP15T08ConditionalCardinalityService"},
}
var typeProductionTiers = map[string][]string{
	"T03": {"unit", "native"},
	"T02": {"unit"},
	"T06": {"unit"},
	"T08": {"unit", "service"},
}
var typeProductionPaths = map[string][]string{
	"T03": {"actions/basic.cherri", "actions/math.cherri", "internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "shortcutgen.go", "p15_number_text_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json", "docs/apple-runtime/macos-27/parameter-encodings.json", "docs/apple-runtime/macos-27/encoding-table.json"},
	"T02": {"internal/language/analysis/analyzer.go", "internal/language/analysis/literal_arguments.go", "internal/language/lower/lower.go", "language_emit_adapter.go", "p15_types_acceptance_test.go"},
	"T06": {"internal/language/types/types.go", "internal/language/types/record_evidence.go", "internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/syntax/parser.go", "internal/language/analysis/control_results.go", "p15_record_acceptance_test.go", "p15_control_type_acceptance_test.go"},
	"T08": {"internal/language/analysis/analyzer.go", "internal/language/analysis/cardinality.go", "internal/language/service/service.go", "internal/language/service/call_result.go", "p15_cardinality_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json"},
}

func (r *Runner) runTypeNumberStringRequirement() {
	r.runTypeProduction("T03", "Semantic Number with number-as-text wire codec")
}

func (r *Runner) runTypeLiteralRequirement() {
	r.runTypeProduction("T02", "Runtime immutable output does not satisfy literal-required slot")
}
func (r *Runner) runTypeRecordRequirement() {
	r.runTypeProduction("T06", "User record annotation is not runtime JSON validation")
}
func (r *Runner) runTypeCardinalityRequirement() {
	r.runTypeProduction("T08", "Variant-dependent output type inferred from literal mode")
}
func (r *Runner) runTypeProduction(id, requirement string) {
	tests := typeProductionTests[id]
	tiers := typeProductionTiers[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "types", requirement, strings.Join(tiers, "+"), parseRequiredTiers(strings.Join(tiers, "+")), passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
		if id == "T03" && r.records[i].Tier == "native-structure" {
			r.records[i].TestID = "github.com/electrikmilk/cherri:" + tests[1]
		}
		// T08's service tier is produced by the actual Service APIs test.
		if id == "T08" && r.records[i].Tier == "service" {
			r.records[i].TestID = "github.com/electrikmilk/cherri:" + tests[1]
		}
	}
}

// A semantic analysis producer cannot stand in for T03's decoded native wire.
// Resolve the producer from the existing named test inventory, not a second
// catalog. Other type requirements retain their current authorization policy.
func typeProductionTestForTier(id, tier string) string {
	if id != "T03" {
		return ""
	}
	tests := typeProductionTests[id]
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + tests[0]
	case "native-structure":
		return "github.com/electrikmilk/cherri:" + tests[1]
	default:
		return ""
	}
}
