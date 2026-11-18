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
