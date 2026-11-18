package main

import "testing"

func TestLiteralCollectionProductionEvidence(t *testing.T) {
	for _, id := range []string{"S02", "L03"} {
		t.Run(id, func(t *testing.T) {
			testNamedProductionEvidence(t, id, literalCollectionProductionTests[id], nil,
				func(r *Runner) { r.runLiteralCollectionRequirement(id) },
				"P15_LITERAL_COLLECTION_RECEIPT:",
				"Only "+id+" exact production literal/diagnostic contract. Serialized-native proof is not Apple runtime, full L01, other acceptance ID, linked repair gate or release closure.")
		})
	}
}

func TestNestedCollectionProductionScopeRemainsUnverified(t *testing.T) {
	r := NewRunner()
	r.runNestedCollectionObservation()
	result := r.results["L01"]
	if result.Passed || result.Status != "NOT_RUN" || len(r.records) != 0 || len(r.observations) != 1 || !r.observations[0].Passed {
		t.Fatalf("partial Dictionary proof minted full L01 evidence or failed: result=%+v observations=%+v records=%+v", result, r.observations, r.records)
	}
}
