package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// Exercise the actual report-writing CLI. An intentionally empty final manifest
// must remain incomplete; a failed mandatory mapping write must be visible.
func TestAcceptanceCLIOutputIsolationAndWriteFailure(t *testing.T) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "language-acceptance")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./tools/language-acceptance")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual CLI: %v\n%s", err, out)
	}
	evidence := filepath.Join(tmp, "empty-evidence.json")
	manifest := EvidenceManifest{
		SchemaVersion: "1.0", ImplementationSHA: resolveCurrentGitSHA(),
		DocsSHA: resolveCurrentDocsSHA(), SchemaFingerprint: strings.Repeat("1", 64),
		CodecFingerprint: strings.Repeat("2", 64), Records: []EvidenceRecord{},
	}
	raw, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(evidence, raw, 0644); err != nil {
		t.Fatal(err)
	}
	trackedMap := filepath.Join(root, "tests", "language-v2", "requirements-evidence-map.json")
	before, err := os.ReadFile(trackedMap)
	if err != nil {
		t.Fatal(err)
	}
	for _, blockMap := range []bool{false, true} {
		name := "external-output"
		if blockMap {
			name = "map-write-failure"
		}
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(tmp, name)
			if err := os.MkdirAll(dir, 0755); err != nil {
				t.Fatal(err)
			}
			mapPath := filepath.Join(dir, "requirements-evidence-map.json")
			if blockMap {
				if err := os.Mkdir(mapPath, 0755); err != nil {
					t.Fatal(err)
				}
			}
			reportPath := filepath.Join(dir, "report.json")
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "--phase=final", "--evidence="+evidence, "--out="+reportPath)
			cmd.Dir = root
			out, runErr := cmd.CombinedOutput()
			if runErr == nil {
				t.Fatalf("empty final evidence unexpectedly passed:\n%s", out)
			}
			if ctx.Err() != nil {
				t.Fatalf("CLI timed out: %v", ctx.Err())
			}
			var report AcceptanceReport
			data, err := os.ReadFile(reportPath)
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &report); err != nil {
				t.Fatal(err)
			}
			if report.TotalCases != 179 || report.ClosureComplete {
				t.Fatalf("unexpected final scope: %+v", report)
			}
			if blockMap {
				if !bytes.Contains(out, []byte("Failed to write evidence map")) {
					t.Fatalf("mandatory map write failure was hidden:\n%s", out)
				}
				fi, err := os.Stat(mapPath)
				if err != nil || !fi.IsDir() {
					t.Fatalf("conflicting directory was changed: %v", err)
				}
			} else {
				var mapping RequirementsEvidenceMapFile
				data, err := os.ReadFile(mapPath)
				if err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(data, &mapping); err != nil {
					t.Fatal(err)
				}
				if mapping.TotalRequirements != 179 {
					t.Fatalf("mapping count: %d", mapping.TotalRequirements)
				}
			}
			after, err := os.ReadFile(trackedMap)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("external output changed tracked map: %v", err)
			}
		})
	}
}

func TestAcceptanceCLIOutputNameCannotBypassFinalValidation(t *testing.T) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "acceptance-name-control")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	build := exec.Command("go", "build", "-o", binary, "./tools/language-acceptance")
	build.Dir = root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual CLI: %v\n%s", err, out)
	}
	evidence := filepath.Join(tmp, "empty-final-evidence.json")
	if err := os.WriteFile(evidence, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	run := func(args ...string) ([]byte, error) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Dir = root
		out, err := cmd.CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("CLI timed out: %v", ctx.Err())
		}
		return out, err
	}
	t.Run("substring-does-not-select-generation", func(t *testing.T) {
		dir := filepath.Join(tmp, "requirements-evidence-map.json-report-directory")
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		outPath := filepath.Join(dir, "final.json")
		out, err := run("--phase=final", "--evidence="+evidence, "--out="+outPath)
		if err == nil {
			t.Fatalf("output filename bypassed final validation; exit=0, output=%s", out)
		}
		var report AcceptanceReport
		data, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(data, &report); err != nil {
			t.Fatal(err)
		}
		if report.TotalCases != 179 || report.ClosureComplete {
			t.Fatalf("final report replaced by another artifact: %+v", report)
		}
	})
	for _, name := range []string{
		"requirements-evidence-map.json", "REQUIREMENTS-EVIDENCE-MAP.JSON",
		"acceptance-local-evidence.json", "acceptance-local-observations.json",
		"acceptance-execution-audit.json",
	} {
		t.Run("reserved-"+name, func(t *testing.T) {
			outPath := filepath.Join(t.TempDir(), name)
			marker := []byte("preexisting-output-must-stay\n")
			if err := os.WriteFile(outPath, marker, 0644); err != nil {
				t.Fatal(err)
			}
			out, err := run("--phase=final", "--evidence="+evidence, "--out="+outPath)
			if err == nil || !bytes.Contains(out, []byte("Output filename is reserved")) {
				t.Errorf("reserved output did not fail closed: error=%v", err)
			}
			after, err := os.ReadFile(outPath)
			if err != nil || !bytes.Equal(marker, after) {
				t.Errorf("reserved output changed existing bytes: %v", err)
			}
		})
	}
	t.Run("explicit-generation-remains-available", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "requirements-evidence-map.json")
		out, err := run("--build-evidence-map", target)
		if err != nil {
			t.Fatalf("explicit generation failed: %v\n%s", err, out)
		}
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		var mapping RequirementsEvidenceMapFile
		if err := json.Unmarshal(data, &mapping); err != nil {
			t.Fatal(err)
		}
		if mapping.TotalRequirements != 179 {
			t.Fatalf("generated mapping count: %d", mapping.TotalRequirements)
		}
	})
	t.Run("generation-cannot-hide-final-evidence-check", func(t *testing.T) {
		target := filepath.Join(t.TempDir(), "requirements-evidence-map.json")
		marker := []byte("existing-map-must-stay\n")
		if err := os.WriteFile(target, marker, 0644); err != nil {
			t.Fatal(err)
		}
		out, err := run("--phase=final", "--evidence="+evidence, "--build-evidence-map", target)
		if err == nil || !bytes.Contains(out, []byte("Evidence-map generation cannot be combined")) {
			t.Errorf("map-only mode hid final validation: error=%v", err)
		}
		data, err := os.ReadFile(target)
		if err != nil || !bytes.Equal(marker, data) {
			t.Errorf("ambiguous mode changed existing map: %v", err)
		}
	})
}
