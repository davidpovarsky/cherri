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
	if record.TestID != scenario.testIDs()[0] ||
		!reflect.DeepEqual(record.Command, productionTestCommandInPackage(scenario.PackagePath, scenario.TestNames)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.TestNames, "\n") {
		return fmt.Errorf("record %s is detached from the complete supplemental production scenario for %s", record.EvidenceID, id)
	}
	return nil
}
