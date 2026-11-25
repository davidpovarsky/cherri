package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type takeoverFixture struct {
	root, source, implementation, directory string
	config                                  takeoverConfig
}

func takeoverFixtureNew(t *testing.T) *takeoverFixture {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	module, err := os.ReadFile(resolveRepoPath(takeoverSource))
	if err != nil {
		t.Fatal(err)
	}
	repositories := []map[string]string{}
	for _, role := range []string{"compiler", "documentation"} {
		root := filepath.Join(base, role)
		if err := os.Mkdir(root, 0700); err != nil {
			t.Fatal(err)
		}
		finalProvenanceTestGit(t, root, "init", "-b", "agent/fixture-source")
		repository := "davidpovarsky/cherri"
		if role == "documentation" {
			repository = "davidpovarsky/cherrilang.org"
		}
		finalProvenanceTestGit(t, root, "remote", "add", "origin", "https://github.com/"+repository+".git")
		if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("original\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if role == "compiler" {
			p := filepath.Join(root, filepath.FromSlash(takeoverSource))
			if err := os.MkdirAll(filepath.Dir(p), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(p, module, 0600); err != nil {
				t.Fatal(err)
			}
		}
		finalProvenanceTestGit(t, root, "add", ".")
		finalProvenanceTestGit(t, root, "commit", "-m", "Explicit takeover control fixture")
		head := finalProvenanceTestGit(t, root, "rev-parse", "HEAD")
		finalProvenanceTestGit(t, root, "branch", "unrelated-agent", head)
		if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("staged\n"), 0600); err != nil {
			t.Fatal(err)
		}
		finalProvenanceTestGit(t, root, "add", "seed.txt")
		if err := os.WriteFile(filepath.Join(root, "seed.txt"), []byte("working\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, "unpublished-א.txt"), []byte("private\x00draft"), 0600); err != nil {
			t.Fatal(err)
		}
		repositories = append(repositories, map[string]string{
			"role": role, "repository": repository, "root": root, "head": head,
			"branch": "refs/heads/agent/fixture-source", "recoveryRef": "refs/heads/agent/takeover-control",
		})
	}
	f := &takeoverFixture{root: repositories[0]["root"], implementation: repositories[0]["head"],
		directory: filepath.Join(base, "verification"),
		config:    takeoverConfig{ExpectedPath: filepath.Join(base, "expected.json")}}
	f.source = filepath.Join(f.root, filepath.FromSlash(takeoverSource))
	if err := os.Mkdir(f.directory, 0700); err != nil {
		t.Fatal(err)
	}
	input := map[string]any{"schema": "cherri-takeover-spec/1", "implementationSHA": f.implementation, "repositories": repositories}
	b, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	spec := filepath.Join(base, "spec.json")
	if err := os.WriteFile(spec, b, 0600); err != nil {
		t.Fatal(err)
	}
	python, err := acceptancePython3("CHERRI_TAKEOVER_PYTHON")
	if err != nil {
		t.Fatal(err)
	}
	originals := filepath.Join(base, "originals")
	for _, args := range [][]string{
		{"produce", "--spec", spec, "--spec-sha256", finalProvenanceDigest(b), "--output", originals},
		{"select", "--output", originals, "--expected", f.config.ExpectedPath},
	} {
		command := exec.Command(python, append([]string{"-I", "-B", f.source}, args...)...)
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("actual fixture operation: %v: %s", err, output)
		}
	}
	b, err = os.ReadFile(f.config.ExpectedPath)
	if err != nil {
		t.Fatal(err)
	}
	f.config.ExpectedSHA256 = finalProvenanceDigest(b)
	return f
}

func (f *takeoverFixture) record(t *testing.T) EvidenceRecord {
	t.Helper()
	o, err := executeTakeoverVerification(f.config, f.implementation, f.root, f.directory)
	if err != nil {
		t.Fatal(err)
	}
	b, err := json.MarshalIndent(o, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.directory, "takeover-observation.json"), append(b, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	record := EvidenceRecord{EvidenceID: "fixture-only-BRG01", Requirements: []string{"BRG01"}, Tier: "repository",
		ImplementationSHA: f.implementation, TestID: takeoverTestID, Command: []string{"internal", takeoverSchema},
		ExitCode: 0, Result: "passed", Detail: takeoverScope,
		Assertions: map[string]string{"scope": takeoverScope, "expected_context_sha256": f.config.ExpectedSHA256,
			"original_receipt_sha256": o.Verification.ReceiptSHA256, "closure_complete": "false"}}
	for _, name := range []string{"takeover-observation.json", "takeover-verifier.py", "takeover-verifier.stdout", "takeover-verifier.stderr"} {
		p := filepath.Join(f.directory, name)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: p, SHA256: finalProvenanceDigest(b)})
	}
	return record
}

func TestTakeoverDedicatedOriginalIntake(t *testing.T) {
	f := takeoverFixtureNew(t)
	t.Setenv("CHERRI_TAKEOVER_EXPECTED", f.config.ExpectedPath)
	t.Setenv("CHERRI_TAKEOVER_EXPECTED_SHA256", f.config.ExpectedSHA256)
	record := f.record(t)
	if err := validateTakeoverRecordAt("BRG01", record, f.root); err != nil {
		t.Fatal(err)
	}
	// These are real transactions in explicitly temporary fixture repositories.
	// They never supply evidence for a production candidate's acceptance.
	if record.ImplementationSHA == resolveCurrentGitSHA() {
		t.Fatal("fixture inherited production identity")
	}
	for _, id := range []string{"B01", "END01", "BRG02"} {
		if err := validateTakeoverRecordAt(id, record, f.root); err == nil {
			t.Fatalf("takeover granted %s", id)
		}
	}
}

func TestTakeoverOriginalIntakeMutations(t *testing.T) {
	for _, name := range []string{"summary-only", "other-requirement", "weaker-tier", "claimed-closure", "wrong-expected-pin",
		"missing-artifact", "changed-current-source", "changed-original-stream", "retagged-production", "missing-process-exit",
		"changed-module", "unbounded-time"} {
		t.Run(name, func(t *testing.T) {
			f := takeoverFixtureNew(t)
			t.Setenv("CHERRI_TAKEOVER_EXPECTED", f.config.ExpectedPath)
			t.Setenv("CHERRI_TAKEOVER_EXPECTED_SHA256", f.config.ExpectedSHA256)
			record := f.record(t)
			mutateObservation := func(edit func(*takeoverObservation)) {
				p := filepath.Join(f.directory, "takeover-observation.json")
				b, err := os.ReadFile(p)
				if err != nil {
					t.Fatal(err)
				}
				var o takeoverObservation
				if err := decodeEditorJSON(b, &o); err != nil {
					t.Fatal(err)
				}
				edit(&o)
				b, err = json.Marshal(o)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, b, 0600); err != nil {
					t.Fatal(err)
				}
				for i := range record.Artifacts {
					if record.Artifacts[i].Path == p {
						record.Artifacts[i].SHA256 = finalProvenanceDigest(b)
					}
				}
			}
			switch name {
			case "summary-only":
				record.TestID = baselineTestID
			case "other-requirement":
				record.Requirements = []string{"BRG01", "B01"}
			case "weaker-tier":
				record.Tier = "unit"
			case "claimed-closure":
				record.Assertions["closure_complete"] = "true"
			case "wrong-expected-pin":
				t.Setenv("CHERRI_TAKEOVER_EXPECTED_SHA256", strings.Repeat("0", 64))
			case "missing-artifact":
				record.Artifacts = record.Artifacts[:3]
			case "changed-current-source":
				if err := os.WriteFile(filepath.Join(f.root, "seed.txt"), []byte("overwritten"), 0600); err != nil {
					t.Fatal(err)
				}
			case "changed-original-stream":
				p := filepath.Join(filepath.Dir(f.root), "originals", "commands", "compiler-before-head.stdout")
				if err := os.WriteFile(p, []byte("wrong"), 0600); err != nil {
					t.Fatal(err)
				}
			case "retagged-production":
				record.ImplementationSHA = resolveCurrentGitSHA()
			case "missing-process-exit":
				mutateObservation(func(o *takeoverObservation) { o.ExitCode = nil })
			case "changed-module":
				p := filepath.Join(f.directory, "takeover-verifier.py")
				b := []byte("print('PASSED')\n")
				if err := os.WriteFile(p, b, 0600); err != nil {
					t.Fatal(err)
				}
				for i := range record.Artifacts {
					if record.Artifacts[i].Path == p {
						record.Artifacts[i].SHA256 = finalProvenanceDigest(b)
					}
				}
				mutateObservation(func(o *takeoverObservation) { o.ProducerSHA256 = finalProvenanceDigest(b) })
			case "unbounded-time":
				mutateObservation(func(o *takeoverObservation) { o.ElapsedSeconds = takeoverTimeout.Seconds() + 1 })
			}
			if err := validateTakeoverRecordAt("BRG01", record, f.root); err == nil {
				t.Fatal("mutated original accepted")
			}
		})
	}
}

func TestTakeoverUnconfiguredAndMalformedRemainOpen(t *testing.T) {
	t.Setenv("CHERRI_TAKEOVER_EXPECTED", "")
	t.Setenv("CHERRI_TAKEOVER_EXPECTED_SHA256", "")
	r := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
	if !r.runTakeoverRequirement("BRG01", "preserve both worktrees") || r.results["BRG01"].Status != "NOT_RUN" || len(r.records) != 0 {
		t.Fatal("unconfigured takeover closed")
	}
	t.Setenv("CHERRI_TAKEOVER_EXPECTED", "relative.json")
	if !r.runTakeoverRequirement("BRG01", "preserve both worktrees") || r.results["BRG01"].Status != "FAILED" || len(r.records) != 0 {
		t.Fatal("malformed takeover context admitted")
	}
	if r.runTakeoverRequirement("B01", "other") {
		t.Fatal("takeover intercepted baseline")
	}
	if !reflect.DeepEqual(deriveGateTestIDs("BRG01", ""), []string{takeoverTestID}) {
		t.Fatal("generic summary still supplies takeover evidence")
	}
	if !reflect.DeepEqual(deriveGateFixtures("BRG01"), takeoverSources) {
		t.Fatal("missing takeover sources")
	}
}
