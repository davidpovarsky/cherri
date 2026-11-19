package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// These files are synthetic inputs to the real evidence validator. They do not
// run WorkflowKit and cannot produce CI01/RP47 production acceptance records.
func nativeEncodingIntakeFixture(t *testing.T, id, sourceSHA string) (EvidenceRecord, map[string]any) {
	t.Helper()
	dir := t.TempDir()
	record := EvidenceRecord{
		EvidenceID: "synthetic-required-" + id, Requirements: []string{id},
		Tier: "negative-ci", TestID: nativeEncodingTestID(id), ImplementationSHA: sourceSHA,
		ExitCode: 0, Result: "passed", Assertions: map[string]string{},
	}
	for _, key := range nativeEncodingAssertionKeys {
		record.Assertions[key] = "true"
	}
	write := func(name string, data []byte) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: path, SHA256: hash, RunID: "101"})
		return hash
	}
	phases := []map[string]any{}
	for i, name := range []string{"positive", "corrupt", "restored"} {
		code := 0
		if i == 1 {
			code = 1
		}
		phase := map[string]any{
			"phase": name, "exitCode": code, "negativeDiagnosticObserved": i == 1,
			"reportSha256": write(name+".json", []byte("{\"syntheticIntakeControl\": true}\n")),
			"stdoutSha256": write(name+".stdout.log", []byte("synthetic fixture only\n")),
			"stderrSha256": write(name+".stderr.log", []byte("synthetic fixture only\n")),
		}
		data, err := json.Marshal(phase)
		if err != nil {
			t.Fatal(err)
		}
		write(name+".process.json", data)
		phases = append(phases, phase)
	}
	summary := map[string]any{
		"schemaVersion": 1, "status": "PASS", "implementationSha": sourceSHA, "runId": "101",
		"appleEngineExecuted": true, "mockEngineAllowed": false, "shortcutRuntimeExecuted": false,
		"inputBytesUnchanged": true, "requiredCases": 23, "expectedRejections": 1,
		"negativeControlExit": 1, "phases": phases,
	}
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	write("required-encoding-gate.json", data)
	return record, summary
}

func rewriteNativeEncodingSummary(t *testing.T, record *EvidenceRecord, summary map[string]any) {
	t.Helper()
	data, err := json.Marshal(summary)
	if err != nil {
		t.Fatal(err)
	}
	for i := range record.Artifacts {
		artifact := &record.Artifacts[i]
		if filepath.Base(artifact.Path) == "required-encoding-gate.json" {
			if err := os.WriteFile(artifact.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(data)
			artifact.SHA256 = hex.EncodeToString(sum[:])
			return
		}
	}
	t.Fatal("summary artifact missing from fixture")
}

func TestRequiredEncodingLocalScopeRemainsUnverified(t *testing.T) {
	runner := &Runner{results: make(map[string]CaseResult)}
	runner.runNativeEncodingRequirement()
	if result := runner.results["CI01"]; result.Passed || result.Status != "NOT_RUN" || len(result.ExecutedLevels) != 0 || len(runner.records) != 0 {
		t.Fatalf("local placeholder minted native encoding acceptance: %+v", result)
	}
}

func TestRequiredEncodingMappingHasDedicatedProducerAndAssertions(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	mapping, err := BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"CI01", "RP47"} {
		row := mapping.Mappings[id]
		if len(row.TestIDs) != 1 || row.TestIDs[0] != nativeEncodingTestID(id) {
			t.Fatalf("%s retained an unrelated smoke producer: %+v", id, row.TestIDs)
		}
		for _, key := range nativeEncodingAssertionKeys {
			if row.RequiredAssertions[key] != "true" {
				t.Fatalf("%s is missing %s", id, key)
			}
		}
	}
}

func TestRequiredEncodingIntakeRejectsIncompleteOrGenericEvidence(t *testing.T) {
	mutations := []struct {
		name  string
		apply func(*EvidenceRecord, map[string]any)
	}{
		{"literal_smoke_id", func(r *EvidenceRecord, _ map[string]any) { r.TestID = "tools/language-acceptance:runVerification" }},
		{"mock_process_tier", func(r *EvidenceRecord, _ map[string]any) { r.Tier = "unit" }},
		{"missing_semantic_assertion", func(r *EvidenceRecord, _ map[string]any) { delete(r.Assertions, "expected_rejection_passed") }},
		{"false_native_assertion", func(r *EvidenceRecord, _ map[string]any) { r.Assertions["apple_engine_executed"] = "false" }},
		{"missing_artifacts", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts = nil }},
		{"missing_digest", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts[0].SHA256 = "" }},
		{"missing_ci_run", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts[0].RunID = "" }},
		{"duplicate_artifact", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts = append(r.Artifacts, r.Artifacts[0]) }},
		{"wrong_report_digest", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts[0].SHA256 = r.Artifacts[1].SHA256 }},
		{"missing_per_form_report", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts = r.Artifacts[1:] }},
		{"missing_process_receipt", func(r *EvidenceRecord, _ map[string]any) { r.Artifacts = append(r.Artifacts[:3], r.Artifacts[4:]...) }},
		{"no_engine", func(_ *EvidenceRecord, s map[string]any) { s["appleEngineExecuted"] = false }},
		{"mock_engine_allowed", func(_ *EvidenceRecord, s map[string]any) { s["mockEngineAllowed"] = true }},
		{"unknown_mock_scope", func(_ *EvidenceRecord, s map[string]any) { delete(s, "mockEngineAllowed") }},
		{"wrong_source", func(_ *EvidenceRecord, s map[string]any) { s["implementationSha"] = "other-source" }},
		{"wrong_run", func(_ *EvidenceRecord, s map[string]any) { s["runId"] = "unrelated" }},
		{"mutated_inputs", func(_ *EvidenceRecord, s map[string]any) { s["inputBytesUnchanged"] = false }},
		{"no_required_rejection", func(_ *EvidenceRecord, s map[string]any) { s["expectedRejections"] = 0 }},
		{"partial_required_set", func(_ *EvidenceRecord, s map[string]any) { s["requiredCases"] = 22 }},
		{"missing_restored_phase", func(_ *EvidenceRecord, s map[string]any) { s["phases"] = s["phases"].([]map[string]any)[:2] }},
		{"missing_positive_exit", func(_ *EvidenceRecord, s map[string]any) { delete(s["phases"].([]map[string]any)[0], "exitCode") }},
		{"negative_exit_zero", func(_ *EvidenceRecord, s map[string]any) { s["phases"].([]map[string]any)[1]["exitCode"] = 0 }},
		{"negative_timeout", func(_ *EvidenceRecord, s map[string]any) { s["phases"].([]map[string]any)[1]["exitCode"] = 124 }},
		{"missing_exact_diagnostic", func(_ *EvidenceRecord, s map[string]any) {
			s["phases"].([]map[string]any)[1]["negativeDiagnosticObserved"] = false
		}},
		{"restored_failure", func(_ *EvidenceRecord, s map[string]any) { s["phases"].([]map[string]any)[2]["exitCode"] = 1 }},
	}
	for _, id := range []string{"CI01", "RP47"} {
		for _, mutation := range mutations {
			t.Run(id+"/"+mutation.name, func(t *testing.T) {
				manifest, contracts := baseValidManifestAndContracts(t)
				record, summary := nativeEncodingIntakeFixture(t, id, manifest.ImplementationSHA)
				if err := validateNativeEncodingRecord(id, record); err != nil {
					t.Fatalf("causal intake control: %v", err)
				}
				contracts.Acceptance.Cases = append(contracts.Acceptance.Cases, CaseSpec{ID: "CI01", Group: "verification", MinimumTestLevel: "negative-CI"})
				contracts.Acceptance.CaseCount++
				contracts.Repair.Cases = append(contracts.Repair.Cases, RepairCaseSpec{ID: "RP47", Title: "Required encoding gate", RequiredTestLevels: []string{"negative-CI"}})
				contracts.Repair.CaseCount++
				otherID := "RP47"
				if id == "RP47" {
					otherID = "CI01"
				}
				other, _ := nativeEncodingIntakeFixture(t, otherID, manifest.ImplementationSHA)
				manifest.Records = append(manifest.Records, other, record)
				if report, err := ValidateEvidenceManifest(manifest, contracts, "final"); err != nil || !p15VerificationResult(t, report, id).Passed {
					t.Fatalf("causal full validator control: %v %+v", err, report)
				}
				mutation.apply(&record, summary)
				if len(record.Artifacts) > 0 && filepath.Base(record.Artifacts[len(record.Artifacts)-1].Path) == "required-encoding-gate.json" {
					rewriteNativeEncodingSummary(t, &record, summary)
				}
				manifest.Records[len(manifest.Records)-1] = record
				report, err := ValidateEvidenceManifest(manifest, contracts, "final")
				result := p15VerificationResult(t, report, id)
				if err == nil || result.Passed || report.ClosureComplete {
					t.Fatalf("invalid required encoding evidence counted as success: %v %+v", err, result)
				}
			})
		}
	}
}
