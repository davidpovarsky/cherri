package main

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func p15DocumentationRecoveryPipeline(t *testing.T, mode string) map[string]json.RawMessage {
	t.Helper()
	source, docs, binary := os.Getenv("CHERRI_DOCUMENTATION_SOURCE_ROOT"), os.Getenv("CHERRI_D03_DOCS_REPO"), os.Getenv("CHERRI_DOCUMENTATION_COMPILER")
	if source == "" || docs == "" || binary == "" {
		t.Skip("requires explicit immutable compiler source, docs checkout and genuine compiler binary; no documentation acceptance is granted")
	}
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("git", "--no-replace-objects", "rev-parse", "HEAD")
	command.Dir = root
	raw, err := command.Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(raw))
	evidence := os.Getenv("CHERRI_DOCUMENTATION_EVIDENCE_OUTPUT")
	if evidence == "" {
		evidence = t.TempDir()
	}
	output, err := os.MkdirTemp(evidence, mode+"-")
	if err != nil {
		t.Fatal(err)
	}
	output = filepath.Join(output, "collection")
	python := os.Getenv("CHERRI_DOCS_PYTHON")
	if python == "" {
		python = resolvePythonCommand()
	}
	args := []string{"-B", filepath.Join(root, "tools", "p10r15", "documentation_recovery_pipeline.py"),
		"--mode", mode, "--compiler-source", source, "--consumer-source", root,
		"--docs", docs, "--compiler", binary, "--implementation-sha", commit,
		"--output", output, "--bash", p15SkillBash(t)}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command = exec.CommandContext(ctx, python, args...)
	command.Dir = root
	stdout, err := command.CombinedOutput()
	if writeErr := os.WriteFile(filepath.Join(filepath.Dir(output), "pipeline-output.log"), stdout, 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatalf("real immutable documentation %s pipeline: %v\n%s", mode, err, stdout)
	}
	original, err := os.ReadFile(filepath.Join(output, "report.json"))
	if err != nil {
		t.Fatal(err)
	}
	var report map[string]json.RawMessage
	if err = json.Unmarshal(original, &report); err != nil {
		t.Fatal(err)
	}
	if string(report["status"]) != `"PASS_DOCUMENTATION_RECOVERY"` || string(report["mode"]) != `"`+mode+`"` ||
		string(report["implementationSHA"]) != `"`+commit+`"` || string(report["inputsUnchangedAfter"]) != "true" ||
		string(report["formalAcceptanceGranted"]) != "false" || string(report["nativeStatus"]) != `"NOT_RUN"` {
		t.Fatalf("incomplete documentation producer receipt: %s", original)
	}
	t.Logf("P15_DOCUMENTATION_RECOVERY_ORIGINAL_REPORT:%s", filepath.Join(output, "report.json"))
	t.Logf("P15_DOCUMENTATION_RECOVERY_TUPLE:%s", report["tuple"])
	return report
}

func TestP15DocumentationRecoveryConsumers(t *testing.T) {
	report := p15DocumentationRecoveryPipeline(t, "consumers")
	t.Run("all_documented_examples", func(t *testing.T) {
		var actual struct {
			Summary struct{ Executed, Passed, Failed int }
		}
		if err := json.Unmarshal(report["examples"], &actual); err != nil {
			t.Fatal(err)
		}
		if actual.Summary.Executed != 88 || actual.Summary.Passed != 88 || actual.Summary.Failed != 0 {
			t.Fatalf("all88 compiler/docs/contextual Skill examples required: %s", report["examples"])
		}
	})
	t.Run("packaged_retrieval", func(t *testing.T) {
		var actual struct {
			Status               string
			Calls                []json.RawMessage
			InputsUnchangedAfter bool
			Checks               map[string]json.RawMessage
		}
		if err := json.Unmarshal(report["wrappers"], &actual); err != nil {
			t.Fatal(err)
		}
		if actual.Status != "PASS_PACKAGED_DOCUMENTATION_WRAPPERS" || len(actual.Calls) != 20 || !actual.InputsUnchangedAfter {
			t.Fatalf("incomplete actual packaged wrapper execution: %s", report["wrappers"])
		}
		for _, name := range []string{"action-show", "action-resizeImage", "action-round", "complete-catalog", "filtered-catalog", "no-match-empty-success", "invalid-regex-propagates", "missing-current-does-not-use-readable-legacy"} {
			if string(actual.Checks[name]) != "true" {
				t.Fatalf("missing actual wrapper observation %s", name)
			}
		}
		for _, name := range []string{"docs-active-0", "docs-active-1", "docs-active-2", "explicit-legacy"} {
			var match struct{ Matches int }
			if json.Unmarshal(actual.Checks[name], &match) != nil || match.Matches <= 0 {
				t.Fatalf("missing source-line retrieval %s", name)
			}
		}
	})
}

func TestP15DocumentationRecoveryGeneration(t *testing.T) {
	report := p15DocumentationRecoveryPipeline(t, "generation")
	t.Run("fresh_reference", func(t *testing.T) {
		var actual struct {
			Bytes                                      int
			SHA256                                     string
			MatchesCompilerBlob, MatchesPinnedDocsBody bool
		}
		if err := json.Unmarshal(report["generation"], &actual); err != nil {
			t.Fatal(err)
		}
		if actual.Bytes < 1000 || len(actual.SHA256) != 64 || !actual.MatchesCompilerBlob || !actual.MatchesPinnedDocsBody {
			t.Fatalf("fresh canonical generator did not match both immutable sources: %s", report["generation"])
		}
	})
	t.Run("canonical_generation_contract", TestP15D01CanonicalDocsProduction)
	t.Run("schema_snippet_parity", TestP10_CatalogCLIAndMobileRegistryParity)
}
