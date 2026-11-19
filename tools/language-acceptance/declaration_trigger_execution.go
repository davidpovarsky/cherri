package main

import (
	"fmt"
	"reflect"
	"strings"
)

// These scenarios execute their own complete selection. Sharing underlying
// production tests with another requirement does not import that result.
var declarationTriggerScenarios = map[string]supplementalProductionScenario{
	"AC03": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15ActionDeclarationPublicValidation", "TestP15ActionDeclarationHealthyControl", "TestP15C04EnumResolutionProduction", "TestP15C02ArgumentDiagnosticsProduction", "TestUpstreamPropagationRehearsal"},
		Tiers:           []string{"generation"},
		ProductionPaths: []string{"internal/language/analysis/action_declarations.go", "internal/language/backend/parameter_codecs.go", "internal/language/syntax/parser.go", "internal/language/analysis/analyzer.go", "tools/language-schema/main.go", "p15_action_declaration_validation_test.go", "p15_enum_acceptance_test.go", "p15_call_acceptance_test.go", "p15_schema_production_test.go", "upstream_rehearsal_test.go"},
	},
	"M05": {
		PackagePath: ".", ModulePath: "github.com/electrikmilk/cherri",
		TestNames:       []string{"TestP15RP32SevenTriggerMigrationAndNativeRoundTrip"},
		Tiers:           []string{"unit", "native"},
		ProductionPaths: []string{"automations.go", "internal/language/migrate/migrate.go", "internal/language/lower/lower.go", "language_backend_adapter.go", "import_fidelity.go", "hybrid_import.go", "p15_supplemental_local_contracts_test.go"},
	},
}

func (r *Runner) runDeclarationTriggerRequirement(id string) {
	scenario := declarationTriggerScenarios[id]
	group, title := "schema", "Unregistered codec, duplicate enum/label or stale facet"
	if id == "M05" {
		group, title = "metadata", "Seven existing trigger families migrate without install claims"
	}
	passed, detail := executeNamedProductionTestsInPackage(scenario.PackagePath, scenario.TestNames)
	first := len(r.records)
	r.Record(id, group, title, strings.Join(scenario.Tiers, "+"), scenario.Tiers, passed, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		r.records[index].TestID = scenario.testIDs()[0]
		r.records[index].Command = productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)
		r.records[index].Assertions["required_production_tests"] = strings.Join(scenario.TestNames, "\n")
	}
}

func validateDeclarationTriggerRecord(id string, record EvidenceRecord) error {
	scenario, found := declarationTriggerScenarios[id]
	if !found {
		return nil
	}
	executed := false
	for _, tier := range scenario.Tiers {
		if record.Tier == tier {
			executed = true
		}
	}
	if !executed || record.TestID != scenario.testIDs()[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.TestNames, "\n") {
		return fmt.Errorf("record %s is detached from the complete declaration/trigger scenario for %s", record.EvidenceID, id)
	}
	return nil
}
