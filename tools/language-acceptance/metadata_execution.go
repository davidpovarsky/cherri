package main

import "strings"

// Metadata evidence executes the production compiler, editor service, mobile
// bridge and real filesystem import routes. Native-roundtrip means exact
// serialized Apple-format data; no import UI or device execution is inferred.
var metadataProductionTests = map[string][]string{
	"M01": {"TestP15M01KnownHeaderMapping", "TestP15M01NativeHeaderRoundTrip"},
	"M03": {"TestP15M03SetupFinalActionBinding", "TestP15M03SetupNativeRoundTrip", "TestP15M03SetupBindingRejectsIdentityCorruption"},
	"M04": {"TestP15M04InvalidSetupAndEditorHoles", "TestP15M04SetupScopeAndUnused", "TestP15M04CatalogInsertionHoles", "TestP15AC01ConstructGrammarAudit", "TestIOS003_PaletteActionSnippetsInV2"},
}
var metadataProductionPaths = map[string][]string{
	"M01": {"internal/language/lower/lower.go", "language_backend_adapter.go", "import_fidelity.go", "hybrid_import.go", "p15_metadata_acceptance_test.go"},
	"M03": {"internal/language/lower/setup.go", "internal/language/lower/lower.go", "internal/language/backend/contracts.go", "setup_question_binding.go", "language_backend_adapter.go", "shortcutgen.go", "p15_metadata_acceptance_test.go"},
	"M04": {"internal/language/analysis/setup.go", "internal/language/analysis/analyzer.go", "actions_catalog.go", "internal/language/schema/generated_registry.go", "internal/language/service/service.go", "mobile_compiler.go", "p15_metadata_acceptance_test.go"},
}
var metadataRequirements = map[string]struct {
	title string
	tiers []string
}{
	"M01": {"Header target/input/output/no-input/icon/source surfaces", []string{"unit", "native-roundtrip"}},
	"M03": {"Setup question binds by node/parameter through lowering", []string{"unit", "native-roundtrip"}},
	"M04": {"Invalid setup use/reuse and Ask Each Time insertion", []string{"unit", "service"}},
}

func (r *Runner) runMetadataRequirement(id string) {
	spec := metadataRequirements[id]
	tests := metadataProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	first := len(r.records)
	level := strings.Join(spec.tiers, "+")
	r.Record(id, "metadata", spec.title, level, parseRequiredTiers(level), passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].Command = productionTestCommand(tests)
	}
}
