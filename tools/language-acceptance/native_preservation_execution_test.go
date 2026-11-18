package main

import "testing"

func TestNativePreservationProductionEvidence(t *testing.T) {
	for _, id := range []string{"V04", "C05", "R01", "R02", "R03", "R04", "R05"} {
		t.Run(id, func(t *testing.T) {
			testNamedProductionEvidence(t, id, nativePreservationProductionTests[id], nil,
				func(r *Runner) { r.runNativePreservationRequirement(id) },
				"P15_NATIVE_PRESERVATION_RECEIPT:",
				"Only "+id+" source/import/serialized-native contract with positive and causal negative checks; no Apple runtime execution, unknown-action availability, other acceptance ID, linked repair gate or release closure.")
		})
	}
}
