package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
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
	var interpreter string
	var candidates = []string{"python3", "python"}
	if runtime.GOOS == "windows" {
		candidates = []string{"python", "python3"}
	}
	for _, candidate := range candidates {
		if path, err := exec.LookPath(candidate); err == nil {
			interpreter = path
			break
		}
	}
	if interpreter == "" {
		t.Fatal("Python 3 is required to execute the actual formal PoC exporter")
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
	if err := json.Unmarshal(exportOutput, &summary); err != nil || summary.FormalRecords != 4 ||
		summary.FormalRequirements != 8 || summary.RawRecords != 17 || summary.ClosureClaimed {
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
	if err := json.Unmarshal(formalBytes, &records); err != nil || len(records) != 4 {
		t.Fatalf("actual exporter output is incompatible with production Go intake: %v", err)
	}
	var wanted = map[string]bool{"V01": true, "V03": true, "L02": true, "L04": true,
		"N01": true, "FN01": true, "FN02": true, "FN03": true}
	var seen = make(map[string]bool)
	for _, record := range records {
		if record.Tier != "ios-runtime" || record.ImplementationSHA != control.Commit || record.Result != "passed" || record.ExitCode != 0 {
			t.Fatalf("unexpected synthetic transport fixture: %+v", record)
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
	t.Log("Synthetic exporter/intake controls passed; no Simulator runtime requirement is credited by this test")
}
