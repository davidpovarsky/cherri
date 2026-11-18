package main

import "testing"

func TestMetadataProductionEvidence(t *testing.T) {
	for _, id := range []string{"M01", "M03", "M04"} {
		t.Run(id, func(t *testing.T) {
			testNamedProductionEvidence(t, id, metadataProductionTests[id], nil,
				func(r *Runner) { r.runMetadataRequirement(id) },
				"P15_METADATA_RECEIPT:",
				"Only "+id+" production source/service and serialized-native metadata contract with causal controls; no Swift UI execution, Apple import-question UI, other acceptance ID, linked repair gate or release closure.")
		})
	}
}
