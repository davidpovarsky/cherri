package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// This is an executable exporter/intake regression with synthetic unit controls.
// Its temporary receipts are never runtime evidence for the immutable179 rows.
func TestRuntimePOCFormalExporterUsesActualGoIntake(t *testing.T) {
	var repo = resolveRepoRoot()
	var contracts, contractErr = LoadAllContracts("", "", "")
	if contractErr != nil {
		t.Fatal(contractErr)
	}
	var interpreter, interpreterErr = acceptancePython3("CHERRI_TEST_PYTHON")
	if interpreterErr != nil {
		t.Fatal("Python 3 is required to execute the actual formal PoC exporter", interpreterErr)
	}
	var toolsPath = filepath.Join(repo, "tools", "shortcuts-simulator-qa")
	var fixturePath = t.TempDir()
	var runPython = func(arguments ...string) []byte {
		var ctx, cancel = context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		var command = exec.CommandContext(ctx, interpreter, append([]string{"-B"}, arguments...)...)
		var output, err = command.CombinedOutput()
		if err != nil {
			t.Fatalf("actual Python exporter/control failed: %v context=%v\n%s", err, ctx.Err(), output)
		}
		return output
	}
	var fixtureBytes = runPython(filepath.Join(toolsPath, "test_runtime_poc_formal_evidence.py"),
		"--fixture-only", fixturePath, "--source-repo", repo)
	var control struct {
		Repo      string `json:"repo"`
		Root      string `json:"root"`
		Commit    string `json:"commit"`
		RunID     string `json:"run_id"`
		Attempt   string `json:"attempt"`
		Synthetic bool   `json:"synthetic_control_only"`
	}
	if err := json.Unmarshal(fixtureBytes, &control); err != nil || !control.Synthetic {
		t.Fatalf("invalid synthetic exporter setup: %v %s", err, fixtureBytes)
	}
	var rawPath = filepath.Join(control.Root, "runtime-diagnostics.json")
	var rawBefore, readErr = os.ReadFile(rawPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	var oldRecords []EvidenceRecord
	if err := json.Unmarshal(rawBefore, &oldRecords); err == nil {
		t.Fatal("causal baseline lost the raw bool/list diagnostic schema that cannot be formal intake")
	}
	var exportOutput = runPython(filepath.Join(toolsPath, "runtime_poc_formal_evidence.py"),
		"--repo", control.Repo, "--root", control.Root, "--commit", control.Commit,
		"--run-id", control.RunID, "--attempt", control.Attempt)
	var summary struct {
		FormalRecords      int  `json:"formalRecords"`
		FormalRequirements int  `json:"formalRequirements"`
		RawRecords         int  `json:"rawDiagnosticRecords"`
		ClosureClaimed     bool `json:"closureClaimed"`
	}
	if err := json.Unmarshal(exportOutput, &summary); err != nil || summary.FormalRecords != 6 ||
		summary.FormalRequirements != 10 || summary.RawRecords != 17 || summary.ClosureClaimed {
		t.Fatalf("exporter scope changed: %+v %v", summary, err)
	}
	var rawAfter, afterErr = os.ReadFile(rawPath)
	if afterErr != nil || string(rawAfter) != string(rawBefore) {
		t.Fatal("exporter rewrote the raw diagnostic evidence", afterErr)
	}
	var formalBytes, formalErr = os.ReadFile(filepath.Join(control.Root, "runtime-evidence.json"))
	if formalErr != nil {
		t.Fatal(formalErr)
	}
	var records []EvidenceRecord
	if err := json.Unmarshal(formalBytes, &records); err != nil || len(records) != 6 {
		t.Fatalf("actual exporter output is incompatible with production Go intake: %v", err)
	}
	var wanted = map[string]bool{"V01": true, "V03": true, "L02": true, "L04": true,
		"N01": true, "FN01": true, "FN02": true, "FN03": true, "S01": true, "F01": true}
	var seen = make(map[string]bool)
	for _, record := range records {
		if record.Tier != "ios-runtime" || record.ImplementationSHA != control.Commit || record.Result != "passed" || record.ExitCode != 0 {
			t.Fatalf("unexpected synthetic transport fixture: %+v", record)
		}
		if strings.HasPrefix(record.TestID, "ios27-runtime-poc:coverage-") {
			for key, name := range map[string]string{
				"coverage_result_sha256": "coverage-result.json", "context_sha256": "coverage-run-context.json",
				"reset_proof_sha256": "coverage-reset-proof.json", "expected_contract_sha256": "coverage-expected.json",
			} {
				var data, err = os.ReadFile(filepath.Join(control.Root, name))
				if err != nil {
					t.Fatal(err)
				}
				var digest = sha256.Sum256(data)
				if record.Assertions[key] != hex.EncodeToString(digest[:]) {
					t.Fatalf("exporter lost actual coverage receipt binding %s", key)
				}
			}
		}
		for _, id := range record.Requirements {
			if !wanted[id] || seen[id] {
				t.Fatal("unsupported or duplicate formal membership", id)
			}
			seen[id] = true
		}
		for _, artifact := range record.Artifacts {
			var content, err = os.ReadFile(filepath.Join(control.Root, filepath.Base(artifact.Path)))
			if err != nil {
				t.Fatal(err)
			}
			var target = filepath.Join(fixturePath, filepath.FromSlash(artifact.Path))
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, content, 0600); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !reflect.DeepEqual(seen, wanted) {
		t.Fatal("narrow runtime facet inventory changed", seen)
	}
	var prior, cwdErr = os.Getwd()
	if cwdErr != nil {
		t.Fatal(cwdErr)
	}
	if err := os.Chdir(fixturePath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(prior); err != nil {
			t.Error(err)
		}
	})
	var manifest = &EvidenceManifest{SchemaVersion: "1", ImplementationSHA: control.Commit, Records: records,
		Runs: []CIRunRecord{{RunID: control.RunID, HeadSHA: control.Commit, CheckoutSHA: control.Commit,
			Conclusion: "success", WorkflowName: "Synthetic exporter/intake unit control"}}}
	var report, validationErr = ValidateEvidenceManifest(manifest, contracts, "local")
	if validationErr == nil || report.ClosureComplete || report.TotalCases != 179 || report.PassedCases != 0 {
		t.Fatalf("partial runtime facets erased mandatory local tiers or promoted closure: %+v %v", report, validationErr)
	}
	for _, issue := range report.ValidationErrors {
		if !strings.HasPrefix(issue, "requirement ") || !strings.HasSuffix(issue, " has unsatisfied local tier") {
			t.Fatal("actual exporter output failed schema, provenance or assertion intake", issue)
		}
	}
	for _, result := range report.Results {
		if wanted[result.ID] {
			if !reflect.DeepEqual(result.ExecutedLevels, []string{"ios-runtime"}) || result.Passed {
				t.Fatalf("immutable non-runtime tiers disappeared: %+v", result)
			}
		} else if len(result.ExecutedLevels) != 0 || result.Passed {
			t.Fatalf("raw diagnostic membership escaped into formal scope: %+v", result)
		}
	}
	var encoded, marshalErr = json.Marshal(manifest)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	for _, fault := range []string{"stale_source", "wrong_assertion", "wrong_artifact_digest", "unknown_run", "unknown_requirement"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "stale_source":
				changed.Records[0].ImplementationSHA = strings.Repeat("f", 40)
			case "wrong_assertion":
				changed.Records[0].Assertions["num_captured"] = "2"
			case "wrong_artifact_digest":
				changed.Records[0].Artifacts[0].SHA256 = strings.Repeat("f", 64)
			case "unknown_run":
				changed.Records[0].Artifacts[0].RunID = "900000002"
			case "unknown_requirement":
				changed.Records[0].Requirements = []string{"D04"}
			}
			var rejected, err = ValidateEvidenceManifest(&changed, contracts, "local")
			var expectedIssue = map[string]string{
				"stale_source": "implementation_sha mismatch", "wrong_assertion": `assertion "num_captured" value mismatch`,
				"wrong_artifact_digest": "sha256 mismatch", "unknown_run": "does not exist in manifest runs",
				"unknown_requirement": `unknown requirement ID "D04"`,
			}[fault]
			if err == nil || rejected.ClosureComplete || !strings.Contains(strings.Join(rejected.ValidationErrors, "\n"), expectedIssue) {
				t.Fatal("detached synthetic receipt passed production intake", err)
			}
		})
	}
	for _, fault := range []string{"old_unicode_producer", "old_conditional_producer", "changed_unicode", "changed_trace", "changed_boolean", "borrowed_repair", "wrong_coverage_source", "missing_coverage_receipt", "invented_exact_match"} {
		t.Run(fault, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(encoded, &changed); err != nil {
				t.Fatal(err)
			}
			var id = "S01"
			if fault == "old_conditional_producer" || fault == "changed_trace" || fault == "changed_boolean" {
				id = "F01"
			}
			var index = -1
			for i, record := range changed.Records {
				if reflect.DeepEqual(record.Requirements, []string{id}) {
					index = i
					break
				}
			}
			if index < 0 {
				t.Fatal("coverage record absent", id)
			}
			var expectedIssue string
			switch fault {
			case "old_unicode_producer":
				changed.Records[index].TestID = "ios27-runtime-poc:unicode"
				expectedIssue = "not authorized for requirement S01"
			case "old_conditional_producer":
				changed.Records[index].TestID = "ios27-runtime-poc:conditionals"
				expectedIssue = "not authorized for requirement F01"
			case "changed_unicode":
				changed.Records[index].Assertions["s01_unicode_output"] += "!"
				expectedIssue = `assertion "s01_unicode_output" value mismatch`
			case "changed_trace":
				changed.Records[index].Assertions["f01_effect_trace"] += "|FORBIDDEN_RHS"
				expectedIssue = `assertion "f01_effect_trace" value mismatch`
			case "changed_boolean":
				changed.Records[index].Assertions["f01_boolean_results"] = "wrong"
				expectedIssue = `assertion "f01_boolean_results" value mismatch`
			case "borrowed_repair":
				changed.Records[index].Requirements = []string{"RP19"}
				expectedIssue = "not authorized for requirement RP19"
			case "wrong_coverage_source":
				changed.Records[index].FixtureSHA256 = strings.Repeat("f", 64)
				expectedIssue = "coverage runtime record"
			case "missing_coverage_receipt":
				delete(changed.Records[index].Assertions, "coverage_result_sha256")
				expectedIssue = "coverage runtime record"
			case "invented_exact_match":
				changed.Records[index].Assertions["exact_output_match"] = "false"
				expectedIssue = "coverage runtime record"
			}
			var rejected, err = ValidateEvidenceManifest(&changed, contracts, "local")
			if err == nil || rejected.ClosureComplete || !strings.Contains(strings.Join(rejected.ValidationErrors, "\n"), expectedIssue) {
				t.Fatal("old, altered, or broadened coverage entered formal intake", err)
			}
		})
	}
	var failed EvidenceManifest
	if err := json.Unmarshal(encoded, &failed); err != nil {
		t.Fatal(err)
	}
	failed.Records[0].Result, failed.Records[0].ExitCode = "failed", 7
	var failedReport, failedErr = ValidateEvidenceManifest(&failed, contracts, "local")
	if failedErr == nil || p15VerificationResult(t, failedReport, "V01").Status != "FAILED" ||
		p15VerificationResult(t, failedReport, "V03").Status != "FAILED" || failedReport.ClosureComplete {
		t.Fatal("selected failed observation was lost during production intake", failedErr)
	}
	for _, id := range []string{"V01", "V03"} {
		if !strings.Contains(strings.Join(failedReport.ValidationErrors, "\n"), "requirement "+id+" has failed evidence record") {
			t.Fatal("failed runtime record was mistaken for an unexecuted local tier", id)
		}
	}
	for index := range failed.Records {
		if strings.HasPrefix(failed.Records[index].TestID, "ios27-runtime-poc:coverage-") {
			failed.Records[index].Result, failed.Records[index].ExitCode = "failed", 1
			failed.Records[index].Assertions["exact_output_match"] = "false"
		}
	}
	var coverageFailedReport, coverageFailedErr = ValidateEvidenceManifest(&failed, contracts, "local")
	for _, id := range []string{"S01", "F01"} {
		if coverageFailedErr == nil || p15VerificationResult(t, coverageFailedReport, id).Status != "FAILED" || coverageFailedReport.ClosureComplete {
			t.Fatal("failed coverage observation disappeared during actual intake", id)
		}
	}
	t.Log("Synthetic exporter/intake controls passed; no Simulator runtime requirement is credited by this test")
}
