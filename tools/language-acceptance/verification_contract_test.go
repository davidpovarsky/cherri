package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func p15VerificationResult(t *testing.T, report *AcceptanceReport, id string) CaseResult {
	t.Helper()
	if report == nil {
		t.Fatal("validator returned no report")
	}
	var seen = make(map[string]bool)
	var found *CaseResult
	for _, result := range report.Results {
		if seen[result.ID] {
			t.Fatalf("requirement %s has multiple statuses", result.ID)
		}
		seen[result.ID] = true
		if result.ID == id {
			var copy = result
			found = &copy
		}
	}
	if found == nil {
		t.Fatalf("requirement %s has no status", id)
	}
	return *found
}

// The fixture's runtime/artifact data is simulated input to the real validator;
// no Apple execution or FN01/E01 production acceptance is claimed by this test.
func TestP15VerificationInvalidRecordsNeverPass(t *testing.T) {
	for _, mutation := range []struct {
		name  string
		apply func(*EvidenceManifest)
	}{
		{"empty_test_id", func(m *EvidenceManifest) { m.Records[0].TestID = "" }},
		{"wrong_test_id", func(m *EvidenceManifest) { m.Records[0].TestID = "unrelated:TestNothing" }},
		{"empty_evidence_id", func(m *EvidenceManifest) { m.Records[0].EvidenceID = "" }},
		{"duplicate_evidence_id", func(m *EvidenceManifest) { m.Records = append(m.Records, m.Records[0]) }},
		{"wrong_source_sha", func(m *EvidenceManifest) { m.Records[2].ImplementationSHA = "different-source" }},
		{"missing_runtime_artifact", func(m *EvidenceManifest) { m.Records[2].Artifacts = nil }},
		{"empty_artifact_path", func(m *EvidenceManifest) { m.Records[2].Artifacts[0].Path = "" }},
		{"missing_artifact_digest", func(m *EvidenceManifest) { m.Records[2].Artifacts[0].SHA256 = "" }},
		{"missing_artifact_run", func(m *EvidenceManifest) { m.Records[2].Artifacts[0].RunID = "" }},
		{"missing_artifact_file", func(m *EvidenceManifest) { m.Records[2].Artifacts[0].Path += ".missing" }},
		{"stale_artifact", func(m *EvidenceManifest) { m.Records[2].Artifacts[0].SHA256 = strings.Repeat("0", 64) }},
		{"foreign_runtime_sha", func(m *EvidenceManifest) { m.Runs[3].HeadSHA = "different-runtime-source" }},
		{"failed_runtime_run", func(m *EvidenceManifest) { m.Runs[3].Conclusion = "failure" }},
		{"unknown_requirement", func(m *EvidenceManifest) { m.Records[0].Requirements = append(m.Records[0].Requirements, "UNKNOWN") }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			for _, phase := range []string{"local", "final"} {
				var manifest, contracts = baseValidManifestAndContracts(t)
				var baseline, err = ValidateEvidenceManifest(manifest, contracts, phase)
				if err != nil || !p15VerificationResult(t, baseline, "FN01").Passed {
					t.Fatalf("causal validator control failed: %v %+v", err, baseline)
				}
				mutation.apply(manifest)
				var report, changedErr = ValidateEvidenceManifest(manifest, contracts, phase)
				var result = p15VerificationResult(t, report, "FN01")
				if changedErr == nil || report.ClosureComplete || result.Passed || result.Status == "PASSED" {
					t.Fatalf("%s invalid receipt still counted as success: err=%v result=%+v", phase, changedErr, result)
				}
			}
		})
	}
}

func TestP15VerificationMalformedProducerFailsClosed(t *testing.T) {
	var originalDiscovery, originalPaths = ciRunDiscoveryFunc, defaultEvidencePathsFunc
	t.Cleanup(func() {
		ciRunDiscoveryFunc, defaultEvidencePathsFunc = originalDiscovery, originalPaths
	})
	var manifest, contracts = baseValidManifestAndContracts(t)
	var dir = t.TempDir()
	var validPath = filepath.Join(dir, "producer.json")
	var raw, err = json.Marshal(manifest.Records)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(validPath, raw, 0600); err != nil {
		t.Fatal(err)
	}
	ciRunDiscoveryFunc = func(string) ([]CIRunRecord, error) { return manifest.Runs, nil }
	defaultEvidencePathsFunc = func() []string { return []string{validPath} }
	var outPath = filepath.Join(dir, "manifest.json")
	var built, buildErr = BuildFinalEvidenceManifest(contracts, manifest.ImplementationSHA, manifest.DocsSHA, outPath)
	if buildErr != nil {
		t.Fatal(buildErr)
	}
	if report, err := ValidateEvidenceManifest(built, contracts, "final"); err != nil || !report.ClosureComplete {
		t.Fatalf("valid producer intake control failed: %v %+v", err, report)
	}
	var before, readErr = os.ReadFile(outPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, payload := range []string{"[", "[]", "{}", "{\"records\":\"not-an-array\"}"} {
		t.Run(payload, func(t *testing.T) {
			var malformedPath = filepath.Join(dir, "malformed.json")
			if err := os.WriteFile(malformedPath, []byte(payload), 0600); err != nil {
				t.Fatal(err)
			}
			defaultEvidencePathsFunc = func() []string { return []string{validPath, malformedPath} }
			var result, err = BuildFinalEvidenceManifest(contracts, manifest.ImplementationSHA, manifest.DocsSHA, outPath)
			if err == nil || result != nil || !strings.Contains(err.Error(), "failed to ingest declared evidence file") {
				t.Fatalf("malformed producer was silently ignored: %+v %v", result, err)
			}
			var after, readErr = os.ReadFile(outPath)
			if readErr != nil || !bytes.Equal(before, after) {
				t.Fatal("failed ingestion overwrote the previously validated manifest")
			}
		})
	}
}

func TestP15RP02ParserOnlyAndForeignSHA(t *testing.T) {
	var contract = &ContractFile{CaseCount: 1, Cases: []CaseSpec{
		{ID: "FN01", Group: "functions", Requirement: "Function native/runtime contract", MinimumTestLevel: "unit+native+iOS"},
	}}
	var partial = map[string]CaseResult{"FN01": {
		ID: "FN01", ExecutedLevels: []string{"unit"}, Passed: false, Status: "NOT_RUN",
		Detail: "Only parser assertions executed; native and runtime tiers did not run",
	}}
	var local, err = ValidateAndAggregateWithPhase(contract, partial, "fixture-fingerprint", "local")
	if err != nil || local.PassedCases != 0 || local.SkippedCases != 1 || local.ClosureComplete {
		t.Fatalf("partial parser report changed into success: %v %+v", err, local)
	}
	var result = p15VerificationResult(t, local, "FN01")
	if result.Status != "NOT_RUN" || len(result.ExecutedLevels) != 1 || result.ExecutedLevels[0] != "unit" {
		t.Fatalf("actual partial levels lost: %+v", result)
	}
	if final, err := ValidateAndAggregateWithPhase(contract, partial, "fixture-fingerprint", "final"); err == nil || final.ClosureComplete {
		t.Fatal("parser-only report closed a native/runtime contract")
	}

	var manifest, contracts = baseValidManifestAndContracts(t)
	if report, err := ValidateEvidenceManifest(manifest, contracts, "final"); err != nil || !p15VerificationResult(t, report, "FN01").Passed {
		t.Fatalf("valid source-binding control failed: %v", err)
	}
	manifest.Runs[3].HeadSHA = "foreign-runtime-sha"
	manifest.Records[2].ImplementationSHA = "foreign-runtime-sha"
	var rejected, rejectErr = ValidateEvidenceManifest(manifest, contracts, "final")
	if rejectErr == nil || p15VerificationResult(t, rejected, "FN01").Passed || rejected.ClosureComplete {
		t.Fatal("foreign-SHA runtime evidence was counted as acceptance")
	}
}

func p15BuildAcceptanceCLI(t *testing.T, root string) string {
	t.Helper()
	var binary = filepath.Join(t.TempDir(), "acceptance-contract")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var command = exec.CommandContext(ctx, "go", "build", "-o", binary, "./tools/language-acceptance")
	command.Dir = root
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build production acceptance CLI: %v\n%s", err, output)
	}
	return binary
}

func p15RunAcceptanceCLI(t *testing.T, binary, cwd string, args ...string) ([]byte, error) {
	t.Helper()
	var ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	var command = exec.CommandContext(ctx, binary, args...)
	command.Dir = cwd
	var output, err = command.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal(ctx.Err())
	}
	return output, err
}

func TestP15VerificationCLIInvalidInputs(t *testing.T) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	var binary = p15BuildAcceptanceCLI(t, root)
	var sourceSHA = resolveCurrentGitSHA()
	var baseline = EvidenceManifest{SchemaVersion: "1", ImplementationSHA: sourceSHA,
		SchemaFingerprint: strings.Repeat("1", 64), CodecFingerprint: strings.Repeat("2", 64),
		Records: []EvidenceRecord{{
			EvidenceID: "unit-fixture-only", Requirements: []string{"P01"}, Tier: "unit",
			ImplementationSHA: sourceSHA, TestID: "github.com/electrikmilk/cherri:TestP15P01LiteralAndInterpolationValues",
			Result: "passed", Assertions: map[string]string{"fixture": "validator input only"},
		}},
	}
	var run = func(t *testing.T, payload []byte) (AcceptanceReport, []byte, error) {
		t.Helper()
		var dir = t.TempDir()
		var evidence, out = filepath.Join(dir, "evidence.json"), filepath.Join(dir, "report.json")
		if err := os.WriteFile(evidence, payload, 0600); err != nil {
			t.Fatal(err)
		}
		var output, err = p15RunAcceptanceCLI(t, binary, root, "--phase=final", "--evidence="+evidence, "--out="+out)
		var report AcceptanceReport
		if data, readErr := os.ReadFile(out); readErr == nil {
			if jsonErr := json.Unmarshal(data, &report); jsonErr != nil {
				t.Fatal(jsonErr)
			}
		}
		return report, output, err
	}
	var valid, marshalErr = json.Marshal(baseline)
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	var control, _, controlErr = run(t, valid)
	if controlErr == nil || control.TotalCases != 179 || control.ClosureComplete || !p15VerificationResult(t, &control, "P01").Passed {
		t.Fatal("validator CLI control did not validate the selected fixture while rejecting incomplete release closure")
	}
	for _, mutation := range []struct {
		name, diagnostic string
		apply            func(*EvidenceManifest)
	}{
		{"false_result", "has failed evidence record", func(m *EvidenceManifest) { m.Records[0].ExitCode = 1; m.Records[0].Result = "failed" }},
		{"contradictory_status", "contradictory record", func(m *EvidenceManifest) { m.Records[0].ExitCode = 1 }},
		{"duplicate_id", "duplicate evidence_id", func(m *EvidenceManifest) { m.Records = append(m.Records, m.Records[0]) }},
		{"unknown_id", "unknown requirement ID", func(m *EvidenceManifest) { m.Records[0].Requirements = append(m.Records[0].Requirements, "UNKNOWN") }},
		{"missing_id", "requirement P01 has unsatisfied", func(m *EvidenceManifest) { m.Records = nil }},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			var changed EvidenceManifest
			if err := json.Unmarshal(valid, &changed); err != nil {
				t.Fatal(err)
			}
			mutation.apply(&changed)
			var payload, err = json.Marshal(changed)
			if err != nil {
				t.Fatal(err)
			}
			var report, output, runErr = run(t, payload)
			if runErr == nil || !strings.Contains(string(output), mutation.diagnostic) ||
				report.ClosureComplete || p15VerificationResult(t, &report, "P01").Passed {
				t.Fatalf("invalid input did not fail the actual closure process: %v\n%s", runErr, output)
			}
		})
	}
	t.Run("malformed_evidence", func(t *testing.T) {
		var report, output, err = run(t, []byte("["))
		if err == nil || report.ClosureComplete || !strings.Contains(string(output), "Could not parse evidence manifest") {
			t.Fatalf("malformed evidence did not fail actual CLI: %v\n%s", err, output)
		}
	})
}

func TestP15RP03FreshCloneContractIntake(t *testing.T) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	var clone = filepath.Join(t.TempDir(), "fresh")
	var ctx, cancel = context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// Exercise Windows checkout conversion without inheriting a user-specific Git setting.
	var command = exec.CommandContext(ctx, "git", "-c", "core.autocrlf=true", "clone", "--no-local", "--depth=1", "--quiet", root, clone)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("fresh clone: %v\n%s", err, output)
	}
	var head = exec.Command("git", "rev-parse", "HEAD")
	head.Dir = clone
	var clonedSHA, headErr = head.Output()
	if headErr != nil || strings.TrimSpace(string(clonedSHA)) != resolveCurrentGitSHA() {
		t.Fatalf("fresh clone resolved a different source: %v %s", headErr, clonedSHA)
	}
	var paths = []string{"acceptance-contract.json", "repair-regression-plan.json", "backend-recovery-gates.json"}
	var expected = []string{ExpectedContractSHA256, ExpectedRepairSHA256, ExpectedGatesSHA256}
	for index, name := range paths {
		var data, err = os.ReadFile(filepath.Join(clone, "tests", "language-v2", name))
		if err != nil {
			t.Fatal(err)
		}
		var hash = sha256.Sum256(data)
		if hex.EncodeToString(hash[:]) != expected[index] {
			t.Fatalf("fresh clone changed immutable contract %s", name)
		}
	}
	var binary = p15BuildAcceptanceCLI(t, clone)
	for _, location := range []string{"repository_default", "unrelated_explicit"} {
		t.Run(location, func(t *testing.T) {
			var out = filepath.Join(t.TempDir(), "map.json")
			var cwd = clone
			var args = []string{"--build-evidence-map", out}
			if location == "unrelated_explicit" {
				cwd = t.TempDir()
				args = append(args, "--contract="+filepath.Join(clone, "tests", "language-v2", paths[0]),
					"--repair-contract="+filepath.Join(clone, "tests", "language-v2", paths[1]),
					"--gates="+filepath.Join(clone, "tests", "language-v2", paths[2]))
			}
			if output, err := p15RunAcceptanceCLI(t, binary, cwd, args...); err != nil {
				t.Fatalf("portable contract intake failed: %v\n%s", err, output)
			}
			var data, err = os.ReadFile(out)
			if err != nil {
				t.Fatal(err)
			}
			var mapping RequirementsEvidenceMapFile
			if err := json.Unmarshal(data, &mapping); err != nil {
				t.Fatal(err)
			}
			if mapping.AcceptanceCount != 94 || mapping.RepairCount != 52 || mapping.GatesCount != 33 || mapping.TotalRequirements != 179 {
				t.Fatalf("portable intake changed immutable obligations: %+v", mapping)
			}
		})
	}
}
