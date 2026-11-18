package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestIncompleteSmokeCannotMintFormalEvidence(t *testing.T) {
	runner := NewRunner()
	for id := range incompleteLocalAssertions {
		runner.Record(id, "scope", "larger formal requirement", "unit+native", []string{"unit", "native"}, true, "PASSED", "observed narrow smoke")
		got := runner.results[id]
		if got.Passed || got.Status != "NOT_RUN" || len(got.ExecutedLevels) != 0 {
			t.Fatalf("%s invented formal evidence: %+v", id, got)
		}
	}
	if len(runner.records) != 0 || len(runner.observations) != len(incompleteLocalAssertions) {
		t.Fatal("smoke must be retained separately without tier records")
	}
	path := filepath.Join(t.TempDir(), "observations.json")
	if err := writeLocalObservations(runner, path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Observations []LocalSmokeObservation `json:"observations"`
	}
	if err = json.Unmarshal(data, &parsed); err != nil || len(parsed.Observations) != len(incompleteLocalAssertions) {
		t.Fatalf("lost observations: %v", err)
	}
}

func TestIncompleteSmokeFailureIsStillVisible(t *testing.T) {
	runner := NewRunner()
	runner.Record("CI01", "verification", "negative CI", "negative-ci", []string{"negative-ci"}, false, "PASSED", "broken smoke")
	if got := runner.results["CI01"]; got.Status != "FAILED" || got.Passed {
		t.Fatalf("actual negative signal hidden: %+v", got)
	}
}
