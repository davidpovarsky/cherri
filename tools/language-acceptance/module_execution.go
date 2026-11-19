package main

import (
	"fmt"
	"reflect"
	"strings"
)

var moduleProductionTests = map[string][]string{
	"IM01": {
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
	},
	"IM02": {
		"TestP15IM02RejectsModuleRuntimeInitialization",
		"TestP15IM02LegacyMigrationDoesNotDropInitialization",
	},
	"IM03": {
		"TestP15IM03PreservedStandardFunctionSource",
		"TestP15IM03DirectServiceWithoutMain",
		"TestP15RP28StandardLibraryGeneration",
		"TestP15IM03StandardFunctionNativeBehavior",
		"TestP15RP28StandardFunctionLegacyDifferential",
	},
	"IM05": {
		"TestP15IM05RemoteImportsNeverFetch",
		"TestP15IM05UnresolvedLocalDependencyNoInstallation",
	},
}

var moduleProductionPaths = map[string][]string{
	"IM01": {
		"internal/language/modules/graph.go", "internal/language/workspace/resource.go",
		"internal/language/analysis/module_imports.go", "internal/language/analysis/analyzer.go",
		"internal/language/lower/module_scope.go", "internal/language/lower/lower.go",
		"internal/language/service/module_analysis.go", "language_emit_adapter.go",
		"p15_module_contracts_test.go", "p15_module_alias_case_test.go",
		"p15_module_canonical_alias_test.go", "p15_module_acceptance_test.go",
	},
	"IM02": {
		"internal/language/modules/graph.go", "internal/language/migrate/migrate.go",
		"internal/language/service/module_analysis.go", "language_emit_adapter.go",
		"p15_module_acceptance_test.go",
	},
	"IM03": {
		"stdfunc.cherri", "tools/language-stdlib/main.go",
		"internal/language/modules/stdfunc_source_generated.go",
		"internal/language/modules/standard_library.go", "internal/language/modules/graph.go",
		"internal/language/service/p15_stdfunc_direct_test.go",
		"p15_module_std_source_test.go", "p15_module_acceptance_test.go", "p15_stdfunc_native_test.go",
	},
	"IM05": {
		"internal/language/modules/graph.go", "internal/language/workspace/resource.go",
		"internal/language/service/module_analysis.go", "language_emit_adapter.go",
		"p15_module_contracts_test.go", "p15_module_acceptance_test.go",
	},
}

func moduleProductionTestForTier(id, tier string) string {
	tests := moduleProductionTests[id]
	if len(tests) == 0 {
		return ""
	}
	producer := ""
	switch id {
	case "IM01":
		if tier == "unit" {
			producer = tests[0]
		} else if tier == "service" {
			producer = "TestP15IM01ModuleServiceRefreshAndExports"
		}
	case "IM02":
		if tier == "unit" {
			producer = tests[0]
		} else if tier == "migration" {
			producer = tests[1]
		}
	case "IM03":
		if tier == "unit" {
			producer = tests[0]
		} else if tier == "native-structure" {
			producer = "TestP15IM03StandardFunctionNativeBehavior"
		}
	case "IM05":
		if tier == "security" {
			producer = tests[0]
		} else if tier == "service" {
			producer = tests[1]
		}
	}
	if producer == "" {
		return ""
	}
	return "github.com/electrikmilk/cherri:" + producer
}

func (r *Runner) runModuleRequirement(id string) {
	var title, level string
	var tiers []string
	switch id {
	case "IM01":
		title, level, tiers = "Relative aliases, duplicate imports and import cycle", "unit+service", []string{"unit", "service"}
	case "IM02":
		title, level, tiers = "Module top-level runtime initialization", "unit+migration", []string{"unit", "migration"}
	case "IM03":
		title, level, tiers = "Existing stdfunc behavior migrated from preserved upstream source", "unit+native", []string{"unit", "native-structure"}
	case "IM05":
		title, level, tiers = "Analysis with unresolved/remote package", "security+service", []string{"security", "service"}
	default:
		panic("unknown module production requirement: " + id)
	}
	tests := moduleProductionTests[id]
	passed, detail := executeNamedProductionTests(tests)
	if id == "IM03" && passed {
		detail += "; actual generator, preserved-source hash, separate service package and raw native body/argument/return/coercion contracts verified; no Apple runtime claim"
	}
	first := len(r.records)
	r.Record(id, "modules", title, level, tiers, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].TestID = moduleProductionTestForTier(id, r.records[index].Tier)
		r.records[index].Command = productionTestCommand(tests)
		r.records[index].Assertions["required_production_tests"] = strings.Join(tests, "\n")
	}
}

func validateModuleRecord(id string, record EvidenceRecord) error {
	tests, found := moduleProductionTests[id]
	if !found {
		return nil
	}
	producer := moduleProductionTestForTier(id, record.Tier)
	if producer == "" || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(tests, "\n") {
		return fmt.Errorf("module record %s is detached from the complete executed %s scenario or claims an unexecuted tier", record.EvidenceID, id)
	}
	return nil
}
