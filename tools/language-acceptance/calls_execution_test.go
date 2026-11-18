package main

import "testing"

func TestCallArgumentsProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "C02", callProductionTests["C02"], nil, (*Runner).runCallArgumentRequirement,
		"P15_CALL_ARGUMENTS_RECEIPT:",
		"Only C02 ordinary action/custom-action/user-function argument diagnostic contract; actual shared analysis, native serialization, CLI and service checks. No Apple execution, source effect ordering, variadic migration or release closure.")
}
