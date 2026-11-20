package main

// This is an independent complete execution of BRG09. Its related original and
// repair IDs remain navigation only; no Optional, Apple or nested native List
// runtime claim can be inherited from these native serialization observations.
var containerRecoveryScenario = supplementalProductionScenario{
	PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
	TestNames: []string{
		"TestBackendRecovery09SemanticContainersAndNativeTemplates",
		"TestBackendRecovery09NativeRoundTrip",
		"TestBackendRecovery09OracleRejectsCorruption",
	},
	Tiers: []string{"native-structure", "native-roundtrip"},
	ProductionPaths: []string{
		"internal/language/lower/lower.go", "internal/language/lower/literal_list.go",
		"internal/language/backend/contracts.go", "language_backend_adapter.go",
		"language_typed_literal_adapter.go", "shared_codec.go", "typed_array_value.go",
		"import_fidelity.go", "hybrid_import.go", "semantic_import_planner.go",
		"native_action_envelope.go", "shortcut.go", "p15_brg09_container_test.go",
		"p15_container_shape_acceptance_test.go", "p15_metadata_acceptance_test.go",
		"p15_native_acceptance_test.go",
	},
}

const containerRecoveryScope = "Actual typed Dictionary arrays and native Lists of Dictionary producers retain nested numbers/bools/empties/references; explicit native.plist templates stay opaque; real XML/binary hybrid/lossless Legacy/V2 filesystem import/recompile preserves exact native trees. No Apple execution, authored JSON null or standalone native List<List> support is claimed."

func containerRecoveryProducer(tier string) string {
	switch tier {
	case "native-structure":
		return "github.com/electrikmilk/cherri:TestBackendRecovery09SemanticContainersAndNativeTemplates"
	case "native-roundtrip":
		return "github.com/electrikmilk/cherri:TestBackendRecovery09NativeRoundTrip"
	}
	return ""
}
