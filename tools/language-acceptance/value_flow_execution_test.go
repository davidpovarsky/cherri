package main

import "testing"

func TestValueBranchProductionEvidence(t *testing.T) {
	testNamedProductionEvidence(t, "F03", valueFlowProductionTests["F03"], map[string]string{"native-structure": "TestP15F03NativeBranchResults"}, (*Runner).runValueBranchRequirement,
		"P15_VALUE_BRANCH_RECEIPT:",
		"Only F03 unit and serialized-native structure: complete normal paths, typed final results, terminating paths, exact result UUIDs, None output isolation, typed literals, source-bound CLI/service diagnostics and causal corruptions. No Apple execution, F04 collection cardinality, or release closure.")
}

func TestValueLoopScopeRemainsUnverified(t *testing.T) {
	if _, incomplete := incompleteLocalAssertions["F04"]; !incomplete {
		t.Fatal("unproved native Repeat Results cardinality acquired a full-contract producer")
	}
}
