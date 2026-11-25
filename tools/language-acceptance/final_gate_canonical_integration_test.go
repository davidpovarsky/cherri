package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// These tests use real canonical mapping, manifest validation and audit paths.
// The candidate records are deliberate forgeries, not workflow observations.
func TestFinalRequiredGateCanonicalInventory(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		t.Fatal(err)
	}
	if doc.TotalRequirements != 179 || len(doc.Records) != 179 {
		t.Fatalf("original requirement inventory changed: %d/%d", doc.TotalRequirements, len(doc.Records))
	}
	for _, id := range []string{"RP49", "BRG31"} {
		t.Run(id, func(t *testing.T) {
			got := doc.Mappings[id]
			if !reflect.DeepEqual(got.TestIDs, []string{finalGateTestID(id)}) ||
				!reflect.DeepEqual(got.FixtureIdentities, finalGateSources) ||
				!reflect.DeepEqual(got.RequiredTiers, finalGateTiers(id)) {
				t.Fatalf("dedicated final gate identity/source/tier mapping missing: %#v", got)
			}
			for key, want := range map[string]string{
				"scope": finalGateScope(id), "global_closure_complete": "false",
				"selected_required_gates_successful": "true", "predeclared_selection": "true",
				"stale_and_missing_candidates_rejected": "true",
			} {
				if got.RequiredAssertions[key] != want {
					t.Fatalf("missing dedicated assertion %q", key)
				}
			}
			if len(got.RequiredAssertions) != 5 || len(got.RequiredUIAssertions) != 0 {
				t.Fatalf("generic smoke assertions survived: %#v", got.RequiredAssertions)
			}
			matches := 0
			for _, row := range doc.Records {
				if row.ID == id {
					matches++
					if !reflect.DeepEqual(row, got) {
						t.Fatal("serialized record differs from canonical mapping")
					}
				}
			}
			if matches != 1 {
				t.Fatalf("dedicated row count: %d", matches)
			}
			ids, assertions, ui := []string{}, map[string]string{}, map[string]string{}
			applyRuntimeAndUIMappings(id, []string{"ios-runtime"}, &ids, assertions, ui)
			if len(ids)+len(assertions)+len(ui) != 0 {
				t.Fatal("generic runtime observation can still enroll a dedicated final gate")
			}
		})
	}
	t.Run("source_includes_canonical_callers", func(t *testing.T) {
		for _, want := range []string{
			"tools/language-acceptance/main.go",
			"tools/language-acceptance/evidence_map.go",
			"tools/language-acceptance/execution_audit.go",
			"tools/language-acceptance/final_gate_canonical_integration_test.go",
		} {
			found := false
			for _, got := range finalGateSources {
				found = found || got == want
			}
			if !found {
				t.Fatalf("canonical caller is absent from source witnesses: %s", want)
			}
		}
	})
}

func TestFinalRequiredGateCanonicalValidation(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"RP49", "BRG31", "CI03"} {
		t.Run(id, func(t *testing.T) {
			producer := id
			if id == "CI03" {
				producer = "RP49"
			}
			raw := []byte("{}\n")
			artifact := filepath.Join(t.TempDir(), "forged-observation.json")
			if err := os.WriteFile(artifact, raw, 0600); err != nil {
				t.Fatal(err)
			}
			source := strings.Repeat("a", 40)
			record := EvidenceRecord{
				EvidenceID: "deliberately-forged-final-" + id, Requirements: []string{id},
				Tier: "repository", ImplementationSHA: source, TestID: finalGateTestID(producer),
				Command: []string{"internal", finalGateSchema, producer}, ExitCode: 0, Result: "passed",
				Assertions: map[string]string{
					"scope": finalGateScope(producer), "expected_context_sha256": strings.Repeat("b", 64),
					"manifest_sha256": strings.Repeat("c", 64), "global_closure_complete": "false",
					"selected_required_gates_successful": "true", "predeclared_selection": "true",
					"stale_and_missing_candidates_rejected": "true",
				},
				Artifacts: []EvidenceArtifact{{Path: artifact, SHA256: ciIdentityDigest(raw)}},
			}
			want := validateFinalRequiredGateRecord(id, record)
			if want == nil {
				t.Fatal("the deliberate forged observation is not a negative control")
			}
			report, err := ValidateEvidenceManifest(&EvidenceManifest{
				SchemaVersion: "1", ImplementationSHA: source, Records: []EvidenceRecord{record},
			}, contracts, "local")
			if err != nil || report == nil {
				t.Fatalf("canonical manifest validator failed to return its report: %v", err)
			}
			found := false
			for _, detail := range report.ValidationErrors {
				found = found || detail == want.Error()
			}
			if !found {
				t.Fatalf("canonical validator did not invoke dedicated validator: want=%q got=%#v", want, report.ValidationErrors)
			}
			if report.ClosureComplete || report.PassedCases != 0 {
				t.Fatal("forged or cross-requirement final evidence acquired acceptance")
			}
		})
	}
}

func TestFinalRequiredGateCanonicalAudit(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	runner := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{
		"RP49":  {ID: "RP49", Status: "NOT_RUN", Detail: "dedicated context is absent"},
		"BRG31": {ID: "BRG31", Status: "FAILED", Detail: "original native closure failed"},
	}}
	name := filepath.Join(t.TempDir(), "audit.json")
	if err := writeExecutionAudit(runner, contracts, name); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		ClosureComplete bool                `json:"closure_complete"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	if document.ClosureComplete || len(document.Rows) != 179 {
		t.Fatal("audit changed the original acceptance boundary")
	}
	for _, id := range []string{"RP49", "BRG31"} {
		t.Run(id, func(t *testing.T) {
			count := 0
			for _, row := range document.Rows {
				if row.ID != id {
					continue
				}
				count++
				if !reflect.DeepEqual(row.TestNames, []string{finalGateTestID(id)}) ||
					!reflect.DeepEqual(row.ProductionPath, finalGateSources) ||
					row.Scope != finalGateScope(id) || row.PositiveAndNegative ||
					row.Blocker != runner.results[id].Detail {
					t.Fatalf("dedicated original failure/absence was misreported: %#v", row)
				}
			}
			if count != 1 {
				t.Fatalf("dedicated audit row count: %d", count)
			}
		})
	}
}
