package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// p15VerifiedEditorMetaFixture keeps the generic FN01/CI inputs explicitly
// synthetic, but executes E01's real fixed service inventory at the clean Git
// source. No synthetic XCTest log is accepted as UI evidence. Each owning test
// creates this fixture once; mutations use independent in-memory copies.
func p15VerifiedEditorMetaFixture(t *testing.T) (*EvidenceManifest, *ContractsSet) {
	t.Helper()
	manifest, contracts := baseValidManifestAndContracts(t)
	all, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, spec := range all.Acceptance.Cases {
		if spec.ID == "E01" {
			contracts.Acceptance.Cases[1] = spec
			found = true
		}
	}
	if !found {
		t.Fatal("immutable E01 contract is absent")
	}

	runner := NewRunner()
	runner.artifactDir = t.TempDir()
	if output := os.Getenv("CHERRI_VERIFICATION_META_EVIDENCE_OUTPUT"); output != "" {
		runner.artifactDir = filepath.Join(output, t.Name())
	}
	runner.runEditorContract("E01")
	result := runner.results["E01"]
	if result.Status != "PENDING_EXTERNAL" || result.Passed || len(runner.records) != 1 {
		t.Fatalf("actual E01 service fixture did not execute: %+v", result)
	}
	record := runner.records[0]
	if err := validateEditorContractRecord("E01", record); err != nil {
		t.Fatalf("actual E01 service intake failed: %v", err)
	}
	manifest.ImplementationSHA = runner.implSHA
	for index := range manifest.Runs {
		manifest.Runs[index].HeadSHA = runner.implSHA
	}
	filtered := make([]EvidenceRecord, 0, len(manifest.Records)-1)
	for _, old := range manifest.Records {
		if len(old.Requirements) == 1 && old.Requirements[0] == "E01" {
			if old.Tier == "service" {
				filtered = append(filtered, record)
			}
			continue
		}
		old.ImplementationSHA = runner.implSHA
		filtered = append(filtered, old)
	}
	manifest.Records = filtered

	if os.Getenv("CHERRI_VERIFICATION_META_EVIDENCE_OUTPUT") != "" {
		raw, err := json.MarshalIndent(record, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(runner.artifactDir, "actual-e01-service-record.json"), append(raw, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("Actual E01 service fixture: source=%s; fixed_events=%d; UI=PENDING_EXTERNAL; generic FN01 and CI inputs are synthetic",
		runner.implSHA, len(editorContracts.Scenarios["E01"].Steps[0].Events))
	for _, phase := range []string{"local", "final"} {
		report, err := ValidateEvidenceManifest(manifest, contracts, phase)
		p15AssertMetaFixtureBaseline(t, report, err, phase)
	}
	return manifest, contracts
}

func p15CloneMetaFixture(t *testing.T, manifest *EvidenceManifest, contracts *ContractsSet) (*EvidenceManifest, *ContractsSet) {
	t.Helper()
	raw, err := json.Marshal(struct {
		Manifest  *EvidenceManifest
		Contracts *ContractsSet
	}{manifest, contracts})
	if err != nil {
		t.Fatal(err)
	}
	var copied struct {
		Manifest  *EvidenceManifest
		Contracts *ContractsSet
	}
	if err := json.Unmarshal(raw, &copied); err != nil {
		t.Fatal(err)
	}
	return copied.Manifest, copied.Contracts
}

// A positive intake control verifies every supplied record, including the
// actual E01 service result, while retaining E01's immutable service+UI tiers.
// Final closure must fail for precisely the missing UI tier, not for a stale
// service record or an unrelated error that could mask a corruption.
func p15AssertMetaFixtureBaseline(t *testing.T, report *AcceptanceReport, err error, phase string) {
	t.Helper()
	e01 := p15VerificationResult(t, report, "E01")
	if report.ClosureComplete || report.PassedCases != report.TotalCases-1 ||
		e01.Passed || len(e01.ExecutedLevels) != 1 || e01.ExecutedLevels[0] != "service" {
		t.Fatalf("positive meta fixture lost verified service or fabricated closure: %v %+v", err, report)
	}
	if phase == "local" {
		if err != nil || report.FailedCases != 0 || report.SkippedCases != 1 ||
			len(report.ValidationErrors) != 0 || e01.Status != "NOT_RUN" {
			t.Fatalf("positive local intake must await only E01 UI: %v %+v", err, report)
		}
		return
	}
	if phase != "final" || err == nil || report.FailedCases != 1 || report.SkippedCases != 0 ||
		e01.Status != "FAILED" || len(report.ValidationErrors) != 1 ||
		report.ValidationErrors[0] != "requirement E01 has unsatisfied required tiers in final phase" {
		t.Fatalf("positive final intake must fail only the unproved E01 UI tier: %v %+v", err, report)
	}
}

func p15AssertCausalMetaRejection(t *testing.T, report *AcceptanceReport, err error, cause string) {
	t.Helper()
	if err == nil || (report != nil && report.ClosureComplete) ||
		!strings.Contains(err.Error(), cause) {
		t.Fatalf("corruption did not cause its required rejection %q: err=%v report=%+v", cause, err, report)
	}
}
