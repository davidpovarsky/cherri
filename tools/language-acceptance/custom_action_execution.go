package main

import (
	"fmt"
	"reflect"
	"strings"
)

var customActionScenario = supplementalProductionScenario{
	PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
	TestNames: []string{
		"TestP15AC02AllCodecFamiliesNativeShape",
		"TestP15AC02DeclaredSchemaRegistryAndRejections",
		"TestP15CustomTokenCodecCanonicalParity",
		"TestP15CustomTokenCodecSecondaryAndImplicitBindings",
		"TestP15CustomCodecSharedReferenceCausality",
		"TestP15CustomTokenCodecPublicRoutes",
		"TestP15ActionDeclarationPublicValidation",
		"TestP15ActionDeclarationHealthyControl",
		"TestP15RegistryRepeatedCompileIsolation",
		"TestP15RegistryCustomOutputReferenceIsolation",
		"TestP15C02ArgumentDiagnosticsProduction",
		"TestP15C02AcceptedBindingsProduction",
		"TestP15C04EnumResolutionProduction",
	},
	Tiers: []string{"unit", "native"},
	ProductionPaths: []string{
		"internal/language/syntax/parser.go", "internal/language/analysis/action_declarations.go",
		"internal/language/analysis/analyzer.go", "internal/language/analysis/registry_lifetime.go",
		"internal/language/schema/schema.go", "internal/language/backend/parameter_codecs.go",
		"internal/language/lower/parameter_codecs.go", "internal/language/lower/lower.go",
		"language_backend_adapter.go", "shared_codec.go", "p15_custom_codec_acceptance_test.go",
		"p15_custom_codec_families_test.go", "p15_action_declaration_validation_test.go",
		"p15_registry_lifetime_test.go", "p15_call_acceptance_test.go", "p15_enum_acceptance_test.go",
	},
}

func (r *Runner) runCustomActionRequirement() {
	passed, detail := executeNamedProductionTestsInPackage(customActionScenario.PackagePath, customActionScenario.TestNames)
	first := len(r.records)
	r.Record("AC02", "schema", "Public custom action declaration feeds same schema/codecs", "unit+native", customActionScenario.Tiers, passed, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		r.records[i].TestID = customActionScenario.testIDs()[0]
		r.records[i].Command = productionTestCommand(customActionScenario.TestNames)
		r.records[i].Assertions["required_production_tests"] = strings.Join(customActionScenario.TestNames, "\n")
	}
}

func validateCustomActionRecord(id string, record EvidenceRecord) error {
	if id != "AC02" {
		return nil
	}
	if record.Tier != "unit" && record.Tier != "native" ||
		record.TestID != customActionScenario.testIDs()[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommand(customActionScenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(customActionScenario.TestNames, "\n") {
		return fmt.Errorf("record %s is detached from the complete public custom-action production scenario", record.EvidenceID)
	}
	return nil
}
