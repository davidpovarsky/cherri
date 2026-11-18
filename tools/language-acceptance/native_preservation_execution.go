package main

import "strings"

// These producers execute the canonical compiler and real native import pipeline.
// Their native tiers inspect serialized Apple-format data; they do not claim
// Apple runtime execution, device availability, or unknown-action behavior.
var nativePreservationProductionTests = map[string][]string{
	"V04": {"TestP15V04CollectionAndOutputBindings"},
	"C05": {"TestP15C05UnknownEnumNativeImport", "TestP15NativeRoundTripOracleRejectsCorruption"},
	"R01": {
		"TestP15R01UnknownNativeFieldsRoundTrip",
		"TestP15NativeActionEnvelopePreservesExactSource",
		"TestP15NativeActionEnvelopeOldParametersRemainData",
		"TestP15NativeActionEnvelopeRejectsConflicts",
		"TestP15NativeActionEnvelopeBeforeOverlayAndCompatibilityRenderers",
		"TestP15NativeActionEnvelopeFullWorkflowAndAuthoredEdit",
		"TestP15NativeActionEnvelopePrivateIdentityCannotEscape",
		"TestP15NativeActionEnvelopeLegacyArity",
		"TestP15NativeRoundTripOracleRejectsCorruption",
	},
	"R02": {"TestP15R02KnownNativeUnmodeledVariants", "TestP15NativeRoundTripOracleRejectsCorruption"},
	"R03": {"TestP15R03OrderedAggrandizementsRoundTrip", "TestP15NativeRoundTripOracleRejectsCorruption"},
	"R04": {"TestP15R04ImportedOmissionAndExplicitValues", "TestP15NativeActionEnvelopePreservesExactSource", "TestP15NativeRoundTripOracleRejectsCorruption"},
	"R05": {"TestP15R05RawExportsAndInvalidBoundary", "TestP15R05OpaqueRegionsAndWorkflowFallback", "TestP15NativeActionEnvelopeFullWorkflowAndAuthoredEdit", "TestP15RawNativeResultFinalizationRejectsIdentityCorruption"},
}

type nativePreservationRequirement struct {
	group string
	title string
	tiers []string
}

var nativePreservationRequirements = map[string]nativePreservationRequirement{
	"V04": {"binding", "let binds lists/maps/action outputs without old const restrictions", []string{"unit", "native"}},
	"C05": {"calls", "Unknown enum wire value survives native import", []string{"native-roundtrip"}},
	"R01": {"native", "Unknown action and unknown nested/top-level fields round-trip", []string{"native-roundtrip"}},
	"R02": {"native", "Known identifier with unmodeled value/static variant", []string{"native-roundtrip"}},
	"R03": {"native", "Coercion-only/property-only/chained aggrandizements", []string{"native-roundtrip"}},
	"R04": {"native", "Original missing parameters not default-filled", []string{"native-roundtrip"}},
	"R05": {"native", "Opaque/native regions, exports and workflow fallback", []string{"unit", "native-roundtrip"}},
}

var nativePreservationProductionPaths = []string{
	"import_fidelity.go", "hybrid_import.go", "semantic_import_planner.go",
	"internal/language/lower/lower.go", "internal/language/lower/native_action_envelope.go",
	"internal/language/backend/native_envelope.go", "native_action_envelope.go",
	"native_action_result_binding.go", "shortcut.go", "raw_actions.go",
	"p15_native_acceptance_test.go", "p15_native_envelope_test.go", "p15_raw_native_result_test.go",
}

func (r *Runner) runNativePreservationRequirement(id string) {
	spec := nativePreservationRequirements[id]
	tests := nativePreservationProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	level := strings.Join(spec.tiers, "+")
	r.Record(id, spec.group, spec.title, level, parseRequiredTiers(level), passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].Command = productionTestCommand(tests)
	}
}
