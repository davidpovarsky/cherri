package main

import (
	"fmt"
	"reflect"
	"strings"
)

type supplementalProductionScenario struct {
	PackagePath     string
	ModulePath      string
	TestNames       []string
	Tiers           []string
	ProductionPaths []string
}

// These are independent executions of the complete supplemental scenarios.
// Related original IDs are navigation only and never supply an inherited pass.
var supplementalProductionScenarios = map[string]supplementalProductionScenario{
	"RP08": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames: []string{"TestP15RP08SchemaIdentityProduction"},
		Tiers:     []string{"generation", "unit"},
		ProductionPaths: []string{
			"actions/web.cherri", "actions_catalog.go", "tools/language-schema/main.go",
			"internal/language/schema/generated_registry.go", "internal/language/schema/schema.go",
			"internal/language/schema/semantic_metadata.go", "internal/language/schema/registry_clone.go",
			"internal/language/schema/macro_metadata.go", "internal/language/backend/contracts.go",
			"internal/language/protocol/protocol.go", "internal/language/analysis/analyzer.go",
			"p15_schema_production_test.go", "p15_schema_identity_recovery_test.go",
		},
	},

	"BRG33": approvedFrontendScenario,
	"BRG24": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames: []string{"TestP15CanonicalBehaviorSourcePropagation"},
		Tiers:     []string{"repository", "integration", "native-structure"},
		ProductionPaths: []string{
			"actions/crypto.cherri", "actions_std.go", "shared_codec.go", "action.go",
			"language_backend_adapter.go", "language_emit_adapter.go", "language_registry_adapter.go",
			"internal/language/schema/generated_registry.go", "internal/language/lower/lower.go",
			"hybrid_import.go", "debug_stdlib_test.go", "p15_schema_production_test.go",
			"p15_schema_generation_recovery_test.go", "p15_canonical_source_propagation_test.go",
		},
	},

	"RP06": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15SchemaOptionalDeclarationPropagation"},
		Tiers:           []string{"generation", "integration"},
		ProductionPaths: []string{"actions/basic.cherri", "actions/calendar.cherri", "actions/device.cherri", "actions/location.cherri", "actions_catalog.go", "tools/language-schema/main.go", "tools/language-docs/main.go", "internal/language/schema/generated_registry.go", "internal/language/schema/schema.go", "internal/language/backend/parameter_codecs.go", "internal/language/analysis/analyzer.go", "internal/language/service/service.go", "language_emit_adapter.go", "language_backend_adapter.go", "p15_schema_production_test.go", "p15_schema_generation_recovery_test.go", "p15_schema_optional_propagation_test.go", "p15_schema_labels_recovery_test.go"},
	},
	"RP07": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15SchemaGenerationFailureControls"},
		Tiers:           []string{"generation", "negative-integration"},
		ProductionPaths: []string{"actions/basic.cherri", "actions/calendar.cherri", "actions/device.cherri", "actions/location.cherri", "actions_catalog.go", "tools/language-schema/main.go", "tools/language-docs/main.go", "internal/language/schema/generated_registry.go", "internal/language/schema/schema.go", "internal/language/backend/parameter_codecs.go", "internal/language/analysis/analyzer.go", "internal/language/service/service.go", "language_emit_adapter.go", "language_backend_adapter.go", "p15_schema_production_test.go", "p15_schema_generation_recovery_test.go", "p15_schema_optional_propagation_test.go", "p15_schema_labels_recovery_test.go"},
	},
	"BRG22": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15SchemaGenerationFailureControls", "TestP15SchemaCanonicalLabels", "TestP15SchemaCorrectedLabelNativeBindings"},
		Tiers:           []string{"generation", "integration"},
		ProductionPaths: []string{"actions/basic.cherri", "actions/calendar.cherri", "actions/device.cherri", "actions/location.cherri", "actions_catalog.go", "tools/language-schema/main.go", "tools/language-docs/main.go", "internal/language/schema/generated_registry.go", "internal/language/schema/schema.go", "internal/language/backend/parameter_codecs.go", "internal/language/analysis/analyzer.go", "internal/language/service/service.go", "language_emit_adapter.go", "language_backend_adapter.go", "p15_schema_production_test.go", "p15_schema_generation_recovery_test.go", "p15_schema_optional_propagation_test.go", "p15_schema_labels_recovery_test.go"},
	},
	"BRG23": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15SchemaOptionalDeclarationPropagation"},
		Tiers:           []string{"repository", "integration", "generation"},
		ProductionPaths: []string{"actions/basic.cherri", "actions/calendar.cherri", "actions/device.cherri", "actions/location.cherri", "actions_catalog.go", "tools/language-schema/main.go", "tools/language-docs/main.go", "internal/language/schema/generated_registry.go", "internal/language/schema/schema.go", "internal/language/backend/parameter_codecs.go", "internal/language/analysis/analyzer.go", "internal/language/service/service.go", "language_emit_adapter.go", "language_backend_adapter.go", "p15_schema_production_test.go", "p15_schema_generation_recovery_test.go", "p15_schema_optional_propagation_test.go", "p15_schema_labels_recovery_test.go"},
	},
	"BRG32": {
		PackagePath: "./tools/language-acceptance",
		ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
		TestNames: []string{
			"TestRealContractsLoadAndVerifyDigests",
			"TestLocalReportIncludesAll179WithoutInheritedClosure",
			"TestP15RP03FreshCloneContractIntake",
			"TestP15BRG32RequirementMapMatchesAllContracts",
			"TestP15BRG32InstalledAIHarness",
			"TestAIEvaluationHarnessCannotChangeRequiredTiers",
		},
		Tiers: []string{"unit", "integration", "repository"},
		ProductionPaths: []string{
			"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json",
			"tests/language-v2/backend-recovery-gates.json", "tools/language-acceptance/main.go",
			"tools/language-acceptance/aggregate_contracts.go", "tools/language-acceptance/evidence_map.go",
			"tools/language-acceptance/supplemental_execution.go", "tools/language-acceptance/execution_truth_test.go",
			"tools/language-acceptance/verification_contract_test.go", "tools/language-acceptance/main_test.go",
			"tools/language-acceptance/obligation_harness_execution_test.go",
			"tools/language-acceptance/ai_evaluation_execution.go", "tools/language-acceptance/ai_evaluation_execution_test.go",
			"tools/language-ai-eval/README.md",
			"tools/language-ai-eval/anthropic-adapter.md",
			"tools/language-ai-eval/anthropic_adapter.py",
			"tools/language-ai-eval/evaluation_bundle.py",
			"tools/language-ai-eval/evaluation_intake.py",
			"tools/language-ai-eval/fresh-instance-protocol-v2.md",
			"tools/language-ai-eval/fresh-instance-protocol.md",
			"tools/language-ai-eval/graph_validator.py",
			"tools/language-ai-eval/runner.py",
			"tools/language-ai-eval/tasks.json",
			"tools/language-ai-eval/test_anthropic_adapter.py",
			"tools/language-ai-eval/test_evaluation_bundle.py",
			"tools/language-ai-eval/test_evaluation_intake.py",
			"tools/language-ai-eval/test_graph_validator.py",
			"tools/language-ai-eval/test_process_group_cleanup.py",
			"tools/language-ai-eval/test_runner.py",
			"tools/language-ai-eval/test_sealed_runner.py",
		},
	},
	"RP27": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15IM01ModuleNamespaceDispatch",
			"TestP15IM01NestedModuleNamespaceIsolation",
			"TestP15IM01CanonicalRelativeModuleIdentity",
			"TestP15IM01ModuleServiceResolvesActualDependencies",
			"TestP15IM01DuplicateImportAndLocalNamesStayDistinct",
			"TestP15IM01ModuleServiceRefreshAndExports",
			"TestP15IM01ModuleAliasCasePreservesNamespace",
			"TestP15IM01UppercaseModuleRejectsUnexportedCallable",
			"TestP15IM01BuiltinModuleAliasesPreserveNativeParity",
			"TestP15IM01BuiltinModuleAliasesRetainSchemaIdentity",
			"TestP15IM01CyclePathAndDuplicateAliasDiagnostics",
			"TestP15IM02RejectsModuleRuntimeInitialization",
			"TestP15IM02LegacyMigrationDoesNotDropInitialization",
			"TestP15IM05RemoteImportsNeverFetch",
			"TestP15IM05UnresolvedLocalDependencyNoInstallation",
		},
		Tiers: []string{"unit", "service", "migration"},
		ProductionPaths: []string{
			"internal/language/modules/graph.go", "internal/language/workspace/resource.go",
			"internal/language/analysis/module_imports.go", "internal/language/lower/module_scope.go",
			"internal/language/service/module_analysis.go", "internal/language/migrate/migrate.go",
			"language_emit_adapter.go", "p15_module_contracts_test.go",
			"p15_module_alias_case_test.go", "p15_module_canonical_alias_test.go", "p15_module_acceptance_test.go",
		},
	},
	"RP38": {
		PackagePath: "./internal/language/lsp", ModulePath: "github.com/electrikmilk/cherri/internal/language/lsp",
		TestNames: []string{
			"TestP15RP38UTF16RequestAndResponseRanges",
			"TestP15RP38MultipleDiagnosticsUTF16",
			"TestP15RP38ContextualLabelsEnumsAndSignatures",
			"TestP15RP38ScopedServiceCompletion",
			"TestP15RP38NestedAndMultilineContext",
			"TestP15RP38ProtocolIDsAndNotifications",
			"TestP15RP38ProtocolMethodsAndNull",
			"TestP15RP38MalformedFramingFailsWithoutPanic",
			"TestP15RP38WriteFailuresAreObserved",
		},
		Tiers:           []string{"service", "protocol"},
		ProductionPaths: []string{"internal/language/lsp/server.go", "internal/language/lsp/position_wire.go", "internal/language/service/service.go", "internal/language/service/context_position.go", "internal/language/service/enum_completion.go", "internal/language/source/span.go", "internal/language/lsp/p15_context_protocol_test.go"},
	},
	"RP04": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP04CheckedCompileRejectsSemanticErrors"},
		Tiers:           []string{"unit", "cli", "embedded"},
		ProductionPaths: []string{"language_cli_adapter.go", "language_emit_adapter.go", "mobile_compiler.go", "internal/language/analysis/analyzer.go", "p15_checked_compile_contract_test.go"},
	},
	"RP34": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP34RejectedMigrationPreservesOriginal", "TestP15RP34ActualWriteFailurePreservesOriginal"},
		Tiers:           []string{"integration", "negative"},
		ProductionPaths: []string{"language_cli_adapter.go", "internal/language/migrate/migrate.go", "p15_migration_atomic_contract_test.go"},
	},
	"BRG09": containerRecoveryScenario,
	"RP28":  standardFunctionRepairScenario,
	"BRG20": moduleRecoveryScenario,
	"RP05": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP05LiteralPublicRoutes"},
		Tiers:           []string{"unit", "cli", "embedded"},
		ProductionPaths: []string{"internal/language/syntax/lexer.go", "internal/language/syntax/parser.go", "language_emit_adapter.go", "language_cli_adapter.go", "mobile_compiler.go", "p15_supplemental_local_contracts_test.go"},
	},
	"RP09": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP09StaticVariantNativeDifferential"},
		Tiers:           []string{"native-differential"},
		ProductionPaths: []string{"actions/images.cherri", "actions/crypto.cherri", "actions/settings.cherri", "action.go", "language_backend_adapter.go", "p15_supplemental_local_contracts_test.go"},
	},
	"RP10": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15CanonicalCallbackCausality", "TestCompilerEndToEndParityMatrix", "TestComparatorMutationMatrix", "TestAppIntentDescriptorGeneralization", "TestCuratedAppleIntentObservedShapesRoundTrip"},
		Tiers:           []string{"native-differential"},
		ProductionPaths: []string{"action.go", "actions_std.go", "language_backend_adapter.go", "action_parity_test.go", "app_intent_test.go", "p15_supplemental_local_contracts_test.go"},
	},
	"RP30": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15M01KnownHeaderMapping", "TestP15M01NativeHeaderRoundTrip", "TestP15RP30HeaderAbsenceAndNameRoundTrip"},
		Tiers:           []string{"unit", "native-roundtrip"},
		ProductionPaths: []string{"internal/language/lower/lower.go", "language_backend_adapter.go", "import_fidelity.go", "hybrid_import.go", "p15_metadata_acceptance_test.go", "p15_supplemental_local_contracts_test.go"},
	},
	"RP31": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15M03SetupFinalActionBinding", "TestP15M03SetupNativeRoundTrip", "TestP15M03SetupBindingRejectsIdentityCorruption", "TestP15M04InvalidSetupAndEditorHoles", "TestP15M04SetupScopeAndUnused", "TestP15M04CatalogInsertionHoles"},
		Tiers:           []string{"unit", "service", "native-roundtrip"},
		ProductionPaths: []string{"internal/language/analysis/setup.go", "internal/language/lower/setup.go", "internal/language/service/service.go", "setup_question_binding.go", "language_backend_adapter.go", "shortcutgen.go", "p15_metadata_acceptance_test.go"},
	},
	"RP32": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP32SevenTriggerMigrationAndNativeRoundTrip", "TestP15ActionDeclarationPublicValidation", "TestP15ActionDeclarationHealthyControl"},
		Tiers:           []string{"unit", "native-structure"},
		ProductionPaths: []string{"internal/language/migrate/migrate.go", "internal/language/lower/lower.go", "internal/language/analysis/action_declarations.go", "internal/language/backend/parameter_codecs.go", "language_backend_adapter.go", "automations.go", "p15_action_declaration_validation_test.go", "p15_supplemental_local_contracts_test.go"},
	},
	"BRG05": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15CanonicalCallbackCausality", "TestArchitecture_A_ProductionCompilerInvokesCanonicalBackend", "TestArchitecture_B_DisablingCanonicalBackendBreaksCompilation", "TestArchitecture_D_SharedCodecMaintenance", "TestCompilerEndToEndParityMatrix", "TestCORE003_UnknownDefinitionIDHardError"},
		Tiers:           []string{"unit", "native-structure", "integration"},
		ProductionPaths: []string{"language_backend_adapter.go", "language_registry_adapter.go", "shared_codec.go", "action.go", "actions_std.go", "architecture_cutover_test.go", "action_parity_test.go", "p15_supplemental_local_contracts_test.go"},
	},
	"BRG17": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15M01KnownHeaderMapping", "TestP15M01NativeHeaderRoundTrip", "TestP15RP30HeaderAbsenceAndNameRoundTrip", "TestP15M03SetupFinalActionBinding", "TestP15M03SetupNativeRoundTrip", "TestP15M03SetupBindingRejectsIdentityCorruption", "TestP15RP32SevenTriggerMigrationAndNativeRoundTrip"},
		Tiers:           []string{"unit", "native-roundtrip"},
		ProductionPaths: []string{"internal/language/lower/lower.go", "internal/language/lower/setup.go", "internal/language/migrate/migrate.go", "setup_question_binding.go", "language_backend_adapter.go", "import_fidelity.go", "automations.go", "p15_metadata_acceptance_test.go", "p15_supplemental_local_contracts_test.go"},
	},
	"RP01": {
		PackagePath: "./tools/language-acceptance",
		ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
		TestNames: []string{
			"TestMeta_AssertionReturnsFalse",
			"TestMeta_ContradictoryPassedStatus",
			"TestMeta_MissingRequiredID",
			"TestMeta_UnknownIDInjected",
			"TestMeta_DuplicateEvidenceID",
			"TestEndToEndAggregator_All12AdversarialAttacks",
			"TestP15VerificationInvalidRecordsNeverPass",
			"TestP15VerificationMalformedProducerFailsClosed",
			"TestP15VerificationCLIInvalidInputs",
		},
		Tiers: []string{"unit", "integration"},
		ProductionPaths: []string{
			"tools/language-acceptance/main.go",
			"tools/language-acceptance/manifest_generator.go",
			"tools/language-acceptance/main_test.go",
			"tools/language-acceptance/verification_contract_test.go",
		},
	},
	"RP02": {
		PackagePath: "./tools/language-acceptance",
		ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
		TestNames: []string{
			"TestP15RP02ParserOnlyAndForeignSHA",
			"TestMeta_UnitAndNativeCannotSatisfyIOSInFinal",
			"TestMeta_FalsePassedClaimWithMissingTiersRejected",
			"TestMeta_WeakTestLevelRejection",
		},
		Tiers: []string{"unit", "integration"},
		ProductionPaths: []string{
			"tools/language-acceptance/main.go",
			"tools/language-acceptance/main_test.go",
			"tools/language-acceptance/verification_contract_test.go",
		},
	},
	"RP03": {
		PackagePath: "./tools/language-acceptance",
		ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
		TestNames: []string{
			"TestP15RP03FreshCloneContractIntake",
			"TestRealContractsLoadAndVerifyDigests",
			"TestMeta_ContractHashMismatch",
		},
		Tiers: []string{"integration"},
		ProductionPaths: []string{
			"tools/language-acceptance/main.go",
			"tools/language-acceptance/evidence_map.go",
			"tools/language-acceptance/verification_contract_test.go",
		},
	},
	"BRG04": {
		PackagePath: "./tools/language-acceptance",
		ModulePath:  "github.com/electrikmilk/cherri/tools/language-acceptance",
		TestNames: []string{
			"TestP15VerificationInvalidRecordsNeverPass",
			"TestP15VerificationMalformedProducerFailsClosed",
			"TestP15VerificationCLIInvalidInputs",
			"TestMeta_ContractHashMismatch",
			"TestMeta_AssertionReturnsFalse",
			"TestMeta_WeakTestLevelRejection",
			"TestMeta_UnitAndNativeCannotSatisfyIOSInFinal",
			"TestSupplementalAcceptanceMissingProducerFailsClosed",
			"TestLocalReportIncludesAll179WithoutInheritedClosure",
			"TestEndToEndAggregator_All12AdversarialAttacks",
		},
		Tiers: []string{"unit", "integration"},
		ProductionPaths: []string{
			"tools/language-acceptance/main.go",
			"tools/language-acceptance/manifest_generator.go",
			"tools/language-acceptance/execution_audit.go",
			"tools/language-acceptance/verification_contract_test.go",
			"tools/language-acceptance/supplemental_execution_test.go",
		},
	},
	"BRG02": {
		PackagePath: "./internal/shortcutcompare",
		ModulePath:  "github.com/electrikmilk/cherri/internal/shortcutcompare",
		TestNames: []string{
			"TestP15BRG02ReferenceTopologyProduction",
		},
		Tiers: []string{"unit", "native-structure"},
		ProductionPaths: []string{
			"internal/shortcutcompare/compare.go",
			"internal/shortcutcompare/p15_recovery_contract_test.go",
		},
	},
	"BRG03": {
		PackagePath: "./internal/shortcutcompare",
		ModulePath:  "github.com/electrikmilk/cherri/internal/shortcutcompare",
		TestNames: []string{
			"TestP15BRG03TypedTreeProduction",
		},
		Tiers: []string{"unit", "native-structure"},
		ProductionPaths: []string{
			"internal/shortcutcompare/compare.go",
			"internal/shortcutcompare/strict_scalars.go",
			"internal/shortcutcompare/p15_recovery_contract_test.go",
		},
	},
	"RP13": {
		PackagePath: ".",
		ModulePath:  "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15C03NamedListNativeStructure",
			"TestP15C03LegacyWireControl",
			"TestP15C03LiteralOracleMutations",
			"TestP15C03ElementOrderAndReferences",
			"TestP15C03PackBoundaries",
			"TestP15C03SingletonReferencesAndTemplates",
			"TestP15C03NestedMigrationAndSourceIdentity",
			"TestP15C03EmptyIsNotAnEmptyItem",
		},
		Tiers: []string{"unit", "native-structure"},
		ProductionPaths: []string{
			"internal/language/schema/variadic.go", "internal/language/analysis/variadic.go",
			"internal/language/lower/lower.go", "internal/language/lower/call_arguments.go",
			"internal/language/migrate/variadic.go", "action.go", "actions_std.go",
			"language_backend_adapter.go", "shortcutgen.go",
			"p15_variadic_migration_test.go", "p15_variadic_structure_test.go",
		},
	},
	"BRG07": {
		PackagePath: ".",
		ModulePath:  "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15BRG07DirectReferenceKindsRoundTrip",
			"TestP15BRG07ReferenceMutationControls",
			"TestP15BRG07SharedReferencePaths",
		},
		Tiers: []string{"native-structure", "native-roundtrip"},
		ProductionPaths: []string{
			"internal/language/lower/lower.go", "internal/language/backend/contracts.go",
			"language_backend_adapter.go", "language_value_adapter.go", "shared_codec.go",
			"shortcut.go", "native_aggrandizement_fields.go", "import_fidelity.go",
			"internal/shortcutcompare/compare.go", "p15_direct_reference_recovery_test.go",
			"tests/language-v2/v2-direct-reference-chains.cherri",
		},
	},
	"RP35": {
		PackagePath: ".",
		ModulePath:  "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15R01UnknownNativeFieldsRoundTrip",
			"TestP15R02KnownNativeUnmodeledVariants",
			"TestP15R03OrderedAggrandizementsRoundTrip",
			"TestP15R04ImportedOmissionAndExplicitValues",
			"TestP15C05UnknownEnumNativeImport",
			"TestP15R05RawExportsAndInvalidBoundary",
			"TestP15NativeRoundTripOracleRejectsCorruption",
		},
		Tiers: []string{"native-roundtrip"},
		ProductionPaths: []string{"import_fidelity.go", "hybrid_import.go", "semantic_import_planner.go",
			"internal/language/lower/native_action_envelope.go", "native_action_envelope.go",
			"p15_native_acceptance_test.go", "p15_native_envelope_test.go"},
	},
	"RP37": {
		PackagePath: ".",
		ModulePath:  "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15K05RepairBuildImportEdit",
			"TestP15K05SigningGateAndExactInput",
			"TestP15K05PrepareRequiresComparator",
		},
		Tiers: []string{"integration", "negative"},
		ProductionPaths: []string{"skills/cherri-shortcuts/scripts/build.sh",
			"skills/cherri-shortcuts/scripts/prepare-edit.sh", "internal/shortcutcompare/edit.go",
			"p15_skill_edit_test.go"},
	},
	"RP36": {
		PackagePath: ".",
		ModulePath:  "github.com/electrikmilk/cherri",
		TestNames: []string{
			"TestP15R06ProductionImportFormats",
			"TestHYB18_SignedShortcutContainerRejected",
			"TestHYB20_TrailingGarbageAfterJSONRootRejected",
			"TestHYB21_MalformedActionEntriesRejectedWithExactIndex",
		},
		Tiers: []string{"integration"},
		ProductionPaths: []string{
			"import_fidelity.go",
			"hybrid_import.go",
			"mobile_compiler.go",
			"p15_intake_acceptance_test.go",
			"p10r9_hybrid_test.go",
		},
	},
}

func (scenario supplementalProductionScenario) testIDs() []string {
	var ids []string
	for _, name := range scenario.TestNames {
		ids = append(ids, scenario.ModulePath+":"+name)
	}
	return ids
}

func (r *Runner) runSupplementalProduction(id, group, title string, levels []string) bool {
	var scenario, found = supplementalProductionScenarios[id]
	if !found || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), scenario.Tiers) {
		return false
	}
	var passed, detail = executeNamedProductionTestsInPackage(scenario.PackagePath, scenario.TestNames)
	var firstRecord = len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), scenario.Tiers, passed, "PASSED", detail)
	for index := firstRecord; index < len(r.records); index++ {
		r.records[index].TestID = scenario.testIDs()[0]
		r.records[index].Command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)
		r.records[index].Assertions["required_production_tests"] = strings.Join(scenario.TestNames, "\n")
		if id == "BRG09" {
			r.records[index].TestID = containerRecoveryProducer(r.records[index].Tier)
			r.records[index].Assertions["container_scope"] = containerRecoveryScope
		}
		if id == "BRG32" {
			r.records[index].Assertions["required_python_tests"] = strings.Join(aiEvaluationPythonTests, "\n")
			r.records[index].Assertions["harness_scope"] = "installed-control-flow-only"
			r.records[index].Assertions["ai_quality_measured"] = "false"
		}
	}
	return true
}

func validateSupplementalProductionRecord(id string, record EvidenceRecord) error {
	var scenario, found = supplementalProductionScenarios[id]
	if !found {
		return nil
	}
	var executedTier bool
	for _, tier := range scenario.Tiers {
		if tier == record.Tier {
			executedTier = true
		}
	}
	if !executedTier {
		return fmt.Errorf("record %s claims unexecuted supplemental tier %s for %s", record.EvidenceID, record.Tier, id)
	}
	var producer = scenario.testIDs()[0]
	if id == "BRG09" {
		producer = containerRecoveryProducer(record.Tier)
		if !reflect.DeepEqual(record.Requirements, []string{"BRG09"}) || record.Assertions["container_scope"] != containerRecoveryScope {
			return fmt.Errorf("record %s is detached from the independent BRG09 native container scenario", record.EvidenceID)
		}
	}
	if record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.TestNames, "\n") {
		return fmt.Errorf("record %s is detached from the complete supplemental production scenario for %s", record.EvidenceID, id)
	}
	if id == "BRG32" && (!reflect.DeepEqual(record.Requirements, []string{"BRG32"}) ||
		record.Assertions["required_python_tests"] != strings.Join(aiEvaluationPythonTests, "\n") ||
		record.Assertions["harness_scope"] != "installed-control-flow-only" ||
		record.Assertions["ai_quality_measured"] != "false") {
		return fmt.Errorf("record %s is detached from installed BRG32 controls or claims AI quality", record.EvidenceID)
	}
	return nil
}
