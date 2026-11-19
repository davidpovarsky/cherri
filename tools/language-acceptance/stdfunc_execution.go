package main

// Supplemental rows execute their own complete producer selection. They do not
// inherit a result from IM03 or from another requirement's evidence record.
var standardFunctionRepairScenario = supplementalProductionScenario{
	PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
	TestNames: []string{
		"TestP15IM03PreservedStandardFunctionSource",
		"TestP15RP28StandardLibraryGeneration",
		"TestP15IM03StandardFunctionNativeBehavior",
		"TestP15RP28StandardFunctionLegacyDifferential",
	},
	Tiers: []string{"generation", "native-structure"},
	ProductionPaths: []string{
		"stdfunc.cherri", "tools/language-stdlib/main.go",
		"internal/language/modules/stdfunc_source_generated.go",
		"internal/language/modules/standard_library.go", "internal/language/modules/graph.go",
		"internal/language/migrate/migrate.go", "internal/language/lower/module_scope.go",
		"p15_module_std_source_test.go", "p15_stdfunc_native_test.go",
	},
}

var moduleRecoveryScenario = supplementalProductionScenario{
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
		"TestP15IM03PreservedStandardFunctionSource",
		"TestP15IM03DirectServiceWithoutMain",
		"TestP15RP28StandardLibraryGeneration",
		"TestP15IM03StandardFunctionNativeBehavior",
		"TestP15RP28StandardFunctionLegacyDifferential",
	},
	Tiers: []string{"unit", "service", "native-structure"},
	ProductionPaths: []string{
		"internal/language/modules/graph.go", "internal/language/workspace/resource.go",
		"internal/language/analysis/module_imports.go", "internal/language/lower/module_scope.go",
		"internal/language/service/module_analysis.go", "internal/language/migrate/migrate.go",
		"stdfunc.cherri", "tools/language-stdlib/main.go",
		"internal/language/modules/stdfunc_source_generated.go",
		"internal/language/modules/standard_library.go", "language_emit_adapter.go",
		"p15_module_contracts_test.go", "p15_module_acceptance_test.go",
		"p15_module_alias_case_test.go", "p15_module_canonical_alias_test.go",
		"p15_module_std_source_test.go", "internal/language/service/p15_stdfunc_direct_test.go",
		"p15_stdfunc_native_test.go",
	},
}
