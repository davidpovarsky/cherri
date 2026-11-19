package main

import "strings"

var typeProductionTests = map[string][]string{
	"T07": {"TestP15T07PerPlatformVersionProduction", "TestP15T07MissingPlatformIsUnverified", "TestP15T07MalformedPlatformMetadataFailsClosed", "TestP15T07PlatformVersionService", "TestP15T07PlatformVersionSchemaIdentity", "TestP15T07CanonicalCatalogPropagation"},
	"T05": {"TestP15T05PresenceBinding", "TestP15T05PresenceNativeWire", "TestP15T05AuthoredNullRejected"},
	"T09": {"TestP15T09ContentConversionEvidence", "TestP15T09ContentConversionNativeWire"},
	"T04": {"TestP15T04OpaqueTypeProduction", "TestP15T04OpaqueTypeService"},
	"T03": {"TestP15T03NumberStringSemanticSignature", "TestP15T03NumberStringNativeWire"},
	"T02": {"TestP15T02LiteralRequiredProduction"},
	"T06": {"TestP15T06RecordAnnotationsDoNotValidateExternalValues", "TestP15T06RecordProofTypeRelations", "TestP15T06ControlResultsRequireAllBranches"},
	"T08": {"TestP15T08ConditionalCardinalityProduction", "TestP15T08ConditionalCardinalityService"},
}
var typeProductionTiers = map[string][]string{
	"T07": {"unit"},
	"T05": {"unit", "native"},
	"T09": {"unit", "native"},
	"T04": {"unit", "service"},
	"T03": {"unit", "native"},
	"T02": {"unit"},
	"T06": {"unit"},
	"T08": {"unit", "service"},
}
var typeProductionPaths = map[string][]string{
	"T07": {"action.go", "actions_catalog.go", "tools/language-schema/main.go", "p15_schema_production_test.go", "internal/language/schema/platform_versions.go", "internal/language/schema/schema.go", "internal/language/schema/registry_clone.go", "internal/language/analysis/target_versions.go", "internal/language/analysis/platform_versions.go", "language_platform_version_adapter.go", "language_backend_adapter.go", "language_emit_adapter.go", "p15_platform_version_acceptance_test.go"},
	"T05": {"internal/language/analysis/analyzer.go", "internal/language/syntax/parser.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "language_typed_literal_adapter.go", "p15_optional_presence_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json"},
	"T09": {"internal/language/types/record_evidence.go", "internal/language/analysis/assignment_evidence.go", "internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "p15_content_conversion_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json"},
	"T04": {"internal/language/types/types.go", "internal/language/types/record_evidence.go", "internal/language/analysis/assignment_evidence.go", "internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/service/service.go", "p15_opaque_type_acceptance_test.go", "p15_record_acceptance_test.go"},
	"T03": {"actions/basic.cherri", "actions/math.cherri", "internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "shortcutgen.go", "p15_number_text_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json", "docs/apple-runtime/macos-27/parameter-encodings.json", "docs/apple-runtime/macos-27/encoding-table.json"},
	"T02": {"internal/language/analysis/analyzer.go", "internal/language/analysis/literal_arguments.go", "internal/language/lower/lower.go", "language_emit_adapter.go", "p15_types_acceptance_test.go"},
	"T06": {"internal/language/types/types.go", "internal/language/types/record_evidence.go", "internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/syntax/parser.go", "internal/language/analysis/control_results.go", "p15_record_acceptance_test.go", "p15_control_type_acceptance_test.go"},
	"T08": {"internal/language/analysis/analyzer.go", "internal/language/analysis/cardinality.go", "internal/language/service/service.go", "internal/language/service/call_result.go", "p15_cardinality_acceptance_test.go", "docs/apple-runtime/macos-27/builtin-actions.json"},
}

// The actual bounded tests execute, but the full JSON-null contract is still
// listed in incompleteLocalAssertions and cannot produce formal tier records.
func (r *Runner) runTypeOptionalPresenceRequirement() {
	r.runTypeProduction("T05", "Optional presence differs from optional value and JSON null")
}

func (r *Runner) runTypeContentConversionRequirement() {
	r.runTypeProduction("T09", "Known Content Graph conversions vs unverified coercion")
}

func (r *Runner) runTypeOpaqueRequirement() {
	r.runTypeProduction("T04", "Unknown differs from AnyContent and Void")
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
func (r *Runner) runTypePlatformVersionRequirement() {
	r.runTypeProduction("T07", "Per-platform versions compare components rather than floats")
}
func (r *Runner) runTypeProduction(id, requirement string) {
	tests := typeProductionTests[id]
	tiers := typeProductionTiers[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	r.Record(id, "types", requirement, strings.Join(tiers, "+"), parseRequiredTiers(strings.Join(tiers, "+")), passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
		if (id == "T03" || id == "T09") && r.records[i].Tier == "native-structure" {
			r.records[i].TestID = "github.com/electrikmilk/cherri:" + tests[1]
		}
		// T08's service tier is produced by the actual Service APIs test.
		if (id == "T08" || id == "T04") && r.records[i].Tier == "service" {
			r.records[i].TestID = "github.com/electrikmilk/cherri:" + tests[1]
		}
	}
}

// A semantic analysis producer cannot stand in for a required wire or service producer.
// Resolve the producer from the existing named test inventory, not a second
// catalog. Other type requirements retain their current authorization policy.
func typeProductionTestForTier(id, tier string) string {
	if id != "T03" && id != "T04" && id != "T09" {
		return ""
	}
	tests := typeProductionTests[id]
	switch tier {
	case "unit":
		return "github.com/electrikmilk/cherri:" + tests[0]
	case "native-structure":
		if id == "T03" || id == "T09" {
			return "github.com/electrikmilk/cherri:" + tests[1]
		}
	case "service":
		if id == "T04" {
			return "github.com/electrikmilk/cherri:" + tests[1]
		}
	default:
		return ""
	}
	return ""
}

// An executed rejection/ordinary-value test does not establish lossless native
// JSON-null semantics. Do not let a caller relabel these bounded observations
// into passing formal tiers while that required part of T05 is unsupported.
func partialTypeProducer(id, testID string) bool {
	if id != "T05" {
		return false
	}
	for _, name := range typeProductionTests[id] {
		full := "github.com/electrikmilk/cherri:" + name
		if testID == full || strings.HasPrefix(testID, full+"/") {
			return true
		}
	}
	return false
}
