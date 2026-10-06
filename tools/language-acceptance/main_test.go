package main

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func TestAcceptanceRunner(t *testing.T) {
	runner := NewRunner()
	runner.runBaseline()
	runner.runUpstream()
	runner.runParser()
	runner.runBinding()
	runner.runCalls()
	runner.runTypes()
	runner.runText()
	runner.runCollections()
	runner.runNumbers()
	runner.runControl()
	runner.runFunctions()
	runner.runMetadata()
	runner.runModules()
	runner.runSchema()
	runner.runNative()
	runner.runEditor()
	runner.runCLI()
	runner.runDocs()
	runner.runSkill()
	runner.runVerification()
	runner.runEvaluation()
	runner.runDelivery()

	if len(runner.results) != 94 {
		t.Fatalf("expected 94 cases evaluated, got %d", len(runner.results))
	}

	for id, res := range runner.results {
		if id == "AI01" {
			if res.Status != "AI_EVAL_NOT_RUN" {
				t.Errorf("expected AI01 status AI_EVAL_NOT_RUN, got %s", res.Status)
			}
			continue
		}
		if !res.Passed || res.Status != "PASSED" {
			t.Errorf("case %s (%s) failed: %s", id, res.Requirement, res.Detail)
		}
	}
}

func TestSchemaFingerprintDeterministic(t *testing.T) {
	reg := schema.DefaultRegistry()
	expected := "32cd14d86ebf5c6ebbe67a6f435468fd9647a3382cf47c09d845b74b9d263694"
	if reg.Fingerprint() != expected {
		t.Fatalf("fingerprint mismatch: got %s, expected %s", reg.Fingerprint(), expected)
	}
}
