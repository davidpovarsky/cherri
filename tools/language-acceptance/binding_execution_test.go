package main

import "testing"

func TestBindingScopeProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "V02", bindingProductionTests["V02"], nil, (*Runner).runBindingScopeRequirement,
		"P15_BINDING_SCOPE_RECEIPT:",
		"Only V02 unit contract: production lexical/flow diagnostics, original-expression type facts, CLI/service checks and serialized native scope identities with imported original-wire controls. No Apple runtime, V01 snapshot runtime guarantee, native optional guard execution or release closure.")
}
