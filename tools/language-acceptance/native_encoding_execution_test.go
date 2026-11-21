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
	canonicalBytes, err := os.ReadFile(resolveRepoPath("tools/shortcut-runtime/required-encoding-expectations.json"))
	if err != nil {
		t.Fatal(err)
	}
	var canonical struct {
		Required []map[string]any `json:"REQUIRED"`
	}
	if err := json.Unmarshal(canonicalBytes, &canonical); err != nil {
		t.Fatal(err)
	}
	fixtureSum := sha256.Sum256(canonicalBytes)
	record := EvidenceRecord{
		EvidenceID: "synthetic-required-" + id, Requirements: []string{id},
		Tier: "negative-ci", TestID: nativeEncodingTestID(id), ImplementationSHA: sourceSHA,
		ExitCode: 0, Result: "passed", Assertions: map[string]string{},
		FixtureSHA256: hex.EncodeToString(fixtureSum[:]),
	}
	for _, key := range nativeEncodingAssertionKeys {
		record.Assertions[key] = "true"
	}
	write := func(name string, data []byte) string {
		t.Helper()
		artifactPath := filepath.Join(dir, name)
		if err := os.WriteFile(artifactPath, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		hash := hex.EncodeToString(sum[:])
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: artifactPath, SHA256: hash, RunID: "101"})
		return hash
	}
	forms := []string{"string", "number", "bool", "attachment(variable)", "attachment(output)", "tokenString", "wrappedSubject"}
	matrix := map[string]map[string]string{}
	for _, row := range canonical.Required {
		state := row["stateClass"].(string)
		if matrix[state] == nil {
			matrix[state] = map[string]string{}
			for _, form := range forms {
				matrix[state][form] = "rejected"
			}
		}
		if row["expect"] == "accept" {
			matrix[state][row["form"].(string)] = "roundtrip"
		}
	}
	phases := []map[string]any{}
	for i, name := range []string{"positive", "corrupt", "restored"} {
		code, status := 0, "PASS"
		expected := make([]map[string]any, len(canonical.Required))
		observations, failures := []map[string]any{}, []map[string]any{}
		for k, original := range canonical.Required {
			row := map[string]any{}
			for key, value := range original {
				row[key] = value
			}
			if i == 1 && row["id"] == "plainString.WFStringParameterState.string" {
				row["expect"] = "reject"
			}
			expected[k] = row
			outcome := matrix[row["stateClass"].(string)][row["form"].(string)]
			actual := "reject"
			if outcome == "roundtrip" {
				actual = "accept"
			}
			passed := actual == row["expect"]
			observations = append(observations, map[string]any{
				"id": row["id"], "kind": row["kind"], "stateClass": row["stateClass"], "form": row["form"],
				"expected": row["expect"], "actual": actual, "observation": outcome, "passed": passed,
			})
			if !passed {
				failures = append(failures, map[string]any{"code": "E_REQUIRED_ENCODING_EXPECTATION", "id": row["id"], "expected": row["expect"], "actual": actual})
			}
		}
		stderr := []byte{}
		if i == 1 {
			code, status = 1, "FAIL"
			stderr = []byte("E_REQUIRED_ENCODING_EXPECTATION: plainString.WFStringParameterState.string\n")
		}
		report := map[string]any{
			"schemaVersion": 2, "status": status, "engine": "WorkflowKit parameter-state API", "engineLoaded": true,
			"forms": forms, "requiredExpectations": expected, "requiredResults": observations, "results": matrix,
			"failures": failures, "discovery": map[string]any{
				"parameterStateClasses": len(matrix), "appIntentStateClasses": 1, "uniqueStateClasses": len(matrix),
			},
		}
		reportBytes, err := json.Marshal(report)
		if err != nil {
			t.Fatal(err)
		}
		command := []string{"/usr/bin/osascript", "-l", "JavaScript", "/synthetic/gate/verify.js",
			"/synthetic/repo/docs/apple-runtime/parameter-encodings.json", "/synthetic/repo/docs/apple-runtime/encoding-table.json",
			"/synthetic/gate/" + name + ".json", "/synthetic/repo/tools/shortcut-runtime/required-encoding-expectations.json"}
		if i == 1 {
			command = append(command, "/synthetic/gate/isolated-corrupt-expectations.json")
		}
		phase := map[string]any{
			"phase": name, "exitCode": code, "command": command, "negativeDiagnosticObserved": i == 1,
			"reportSha256": write(name+".json", reportBytes),
			"stdoutSha256": write(name+".stdout.log", []byte("synthetic fixture only\n")),
			"stderrSha256": write(name+".stderr.log", stderr),
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
	baseManifest, baseContracts := p15VerifiedEditorMetaFixture(t)
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
				manifest, contracts := p15CloneMetaFixture(t, baseManifest, baseContracts)
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
				baseline, baselineErr := ValidateEvidenceManifest(manifest, contracts, "final")
				p15AssertMetaFixtureBaseline(t, baseline, baselineErr, "final")
				if !p15VerificationResult(t, baseline, id).Passed {
					t.Fatal("causal encoding intake control did not pass its own requirement")
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

func rewriteNativeEncodingArtifact(t *testing.T, record *EvidenceRecord, summary map[string]any, phaseName, suffix string, change func(map[string]any), changeSummary bool) {
	t.Helper()
	index := -1
	for i := range record.Artifacts {
		if filepath.Base(record.Artifacts[i].Path) == phaseName+suffix {
			index = i
			break
		}
	}
	if index < 0 {
		t.Fatal("artifact absent from synthetic fixture")
	}
	var value map[string]any
	data, err := os.ReadFile(record.Artifacts[index].Path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &value); err != nil {
		t.Fatal(err)
	}
	change(value)
	write := func(i int, value map[string]any) string {
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(record.Artifacts[i].Path, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		record.Artifacts[i].SHA256 = hex.EncodeToString(sum[:])
		return record.Artifacts[i].SHA256
	}
	hash := write(index, value)
	for _, phase := range summary["phases"].([]map[string]any) {
		if phase["phase"] != phaseName {
			continue
		}
		if suffix == ".json" {
			phase["reportSha256"] = hash
			for i := range record.Artifacts {
				if filepath.Base(record.Artifacts[i].Path) != phaseName+".process.json" {
					continue
				}
				var process map[string]any
				data, err := os.ReadFile(record.Artifacts[i].Path)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &process); err != nil {
					t.Fatal(err)
				}
				process["reportSha256"] = hash
				write(i, process)
			}
		} else if changeSummary {
			for key, item := range value {
				phase[key] = item
			}
		}
	}
	rewriteNativeEncodingSummary(t, record, summary)
}

func TestRequiredEncodingPhaseArtifactReconciliation(t *testing.T) {
	baseManifest, baseContracts := p15VerifiedEditorMetaFixture(t)
	mutations := []struct {
		name, phase, suffix string
		change              func(map[string]any)
		changeSummary       bool
	}{
		{"changed_required_verdict", "positive", ".json", func(v map[string]any) { v["requiredResults"].([]any)[0].(map[string]any)["passed"] = false }, false},
		{"changed_actual_matrix", "positive", ".json", func(v map[string]any) {
			row := v["requiredResults"].([]any)[0].(map[string]any)
			v["results"].(map[string]any)[row["stateClass"].(string)].(map[string]any)[row["form"].(string)] = "rejected"
		}, false},
		{"missing_required_result", "restored", ".json", func(v map[string]any) { v["requiredResults"] = v["requiredResults"].([]any)[1:] }, false},
		{"wrong_applied_expectation", "positive", ".json", func(v map[string]any) { v["requiredExpectations"].([]any)[0].(map[string]any)["expect"] = "reject" }, false},
		{"negative_report_claims_pass", "corrupt", ".json", func(v map[string]any) { v["status"] = "PASS" }, false},
		{"missing_actual_forms", "restored", ".json", func(v map[string]any) { v["forms"] = []any{} }, false},
		{"process_exit_disagrees", "corrupt", ".process.json", func(v map[string]any) { v["exitCode"] = 0 }, false},
		{"process_command_disagrees", "positive", ".process.json", func(v map[string]any) { v["command"].([]any)[0] = "/usr/bin/true" }, false},
		{"wrong_engine_even_when_summary_agrees", "positive", ".process.json", func(v map[string]any) { v["command"].([]any)[0] = "/usr/bin/true" }, true},
	}
	for _, id := range []string{"CI01", "RP47"} {
		for _, mutation := range mutations {
			t.Run(id+"/"+mutation.name, func(t *testing.T) {
				manifest, contracts := p15CloneMetaFixture(t, baseManifest, baseContracts)
				record, summary := nativeEncodingIntakeFixture(t, id, manifest.ImplementationSHA)
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
				baseline, baselineErr := ValidateEvidenceManifest(manifest, contracts, "final")
				p15AssertMetaFixtureBaseline(t, baseline, baselineErr, "final")
				if !p15VerificationResult(t, baseline, id).Passed {
					t.Fatal("complete synthetic encoding intake did not pass its own requirement")
				}
				rewriteNativeEncodingArtifact(t, &record, summary, mutation.phase, mutation.suffix, mutation.change, mutation.changeSummary)
				manifest.Records[len(manifest.Records)-1] = record
				report, err := ValidateEvidenceManifest(manifest, contracts, "final")
				result := p15VerificationResult(t, report, id)
				if err == nil || result.Passed || report.ClosureComplete {
					t.Fatalf("content mutation with recomputed digests still accepted: err=%v result=%+v", err, result)
				}
			})
		}
	}
}

func TestRequiredEncodingStrictJSONRejectsAmbiguity(t *testing.T) {
	if _, err := decodeNativeEncodingObject([]byte(`{"schemaVersion":2,"engineLoaded":true,"requiredResults":[]}`)); err != nil {
		t.Fatalf("valid JSON control: %v", err)
	}
	for _, text := range []string{
		`{"engineLoaded":false,"engineLoaded":true}`,
		`{"requiredResults":[{"passed":false,"passed":true}]}`,
		`{"exitCode":1} {"exitCode":0}`,
		`{"results":`,
		`[]`,
	} {
		t.Run(text, func(t *testing.T) {
			if _, err := decodeNativeEncodingObject([]byte(text)); err == nil {
				t.Fatal("ambiguous, malformed, or non-object receipt accepted")
			}
		})
	}
}

func TestRequiredEncodingRecomputesDiagnosticFromActualStderr(t *testing.T) {
	record, summary := nativeEncodingIntakeFixture(t, "CI01", "synthetic-source")
	if err := validateNativeEncodingRecord("CI01", record); err != nil {
		t.Fatal(err)
	}
	phase := summary["phases"].([]map[string]any)[1]
	changed := []byte("generic infrastructure failure; required diagnostic absent\n")
	for i := range record.Artifacts {
		artifact := &record.Artifacts[i]
		if filepath.Base(artifact.Path) != "corrupt.stderr.log" {
			continue
		}
		if err := os.WriteFile(artifact.Path, changed, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(changed)
		artifact.SHA256 = hex.EncodeToString(sum[:])
		phase["stderrSha256"] = artifact.SHA256
	}
	for i := range record.Artifacts {
		artifact := &record.Artifacts[i]
		if filepath.Base(artifact.Path) != "corrupt.process.json" {
			continue
		}
		data, err := json.Marshal(phase)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(artifact.Path, data, 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		artifact.SHA256 = hex.EncodeToString(sum[:])
	}
	rewriteNativeEncodingSummary(t, &record, summary)
	if err := validateNativeEncodingRecord("CI01", record); err == nil {
		t.Fatal("a true diagnostic flag hid the actual generic failure despite recomputed digests")
	}
}
