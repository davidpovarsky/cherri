package main

import "testing"

func TestCallArgumentsProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "C02", callProductionTests["C02"], nil, (*Runner).runCallArgumentRequirement,
		"P15_CALL_ARGUMENTS_RECEIPT:",
		"Only C02 ordinary action/custom-action/user-function argument diagnostic contract; actual shared analysis, native serialization, CLI and service checks. No Apple execution, source effect ordering, variadic migration or release closure.")
}

func TestEnumProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "C04", callProductionTests["C04"], map[string]string{"service": "TestP15C04EnumServiceAndCLI"}, (*Runner).runEnumRequirement,
		"P15_ENUM_RECEIPT:",
		"Only C04 enum source resolution, exact serialized wire and shared service completion/diagnostic unit+service tiers; no Apple execution, full UI, C05 imported-unknown-enum contract or release closure.")
}

func TestCallOrderProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "C01", callProductionTests["C01"], map[string]string{"native-structure": "TestP15C01SourceOrderNativeStructure"}, (*Runner).runCallOrderRequirement,
		"P15_CALL_ORDER_RECEIPT:",
		"Only C01 unit and actual native-structure tiers: source effect order, once-only supplied/default producers, UUID/value binding and CLI/service checks. No Apple function execution, function bridge runtime, system-value snapshot or release closure.")
}

func TestVariadicMigrationProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "C03", callProductionTests["C03"], map[string]string{"native-structure": "TestP15C03NamedListNativeStructure"}, (*Runner).runVariadicMigrationRequirement,
		"P15_VARIADIC_MIGRATION_RECEIPT:",
		"Only C03 unit and native-structure: all eight canonical variadics, empty/single/multiple explicit named packs, exact native item/UUID order, no recursive flattening, source/shadow preservation, actual original CLI differentials and causal mutations. No Apple execution, unsupported Legacy inline-action parity, dynamic runtime spreading, or release closure.")
}
