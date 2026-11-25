package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type ownedBaselineFixture struct {
	root     string
	oracle   *finalProvenanceOracleFixture
	expected baselineExpected
	manifest baselineManifest
	config   finalProvenanceConfig
}

func ownedBaselineFixtureNew(t *testing.T) *ownedBaselineFixture {
	t.Helper()
	root := t.TempDir()
	finalProvenanceTestGit(t, root, "init", "-b", "agent/synthetic-source")
	finalProvenanceTestGit(t, root, "remote", "add", "origin", "https://github.com/davidpovarsky/cherri.git")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("base\n"), 0600); err != nil {
		t.Fatal(err)
	}
	finalProvenanceTestGit(t, root, "add", "source.txt")
	finalProvenanceTestGit(t, root, "commit", "-m", "Synthetic baseline")
	base := finalProvenanceTestGit(t, root, "rev-parse", "HEAD")
	if err := os.WriteFile(filepath.Join(root, "source.txt"), []byte("current\n"), 0600); err != nil {
		t.Fatal(err)
	}
	finalProvenanceTestGit(t, root, "commit", "-am", "Synthetic descendant")
	sha, tree := finalProvenanceTestGit(t, root, "rev-parse", "HEAD"), finalProvenanceTestGit(t, root, "rev-parse", "HEAD^{tree}")
	// Real isolated Git observations are combined with explicitly synthetic
	// publication originals. These controls never perform or claim a real push.
	x := finalProvenanceOracleFixtureNew(t)
	replace := strings.NewReplacer(strings.Repeat("a", 40), sha, strings.Repeat("b", 40), tree, strings.Repeat("d", 40), base)
	b, err := json.Marshal(x.o)
	if err != nil {
		t.Fatal(err)
	}
	x.o, err = decodeNativeEncodingObject([]byte(replace.Replace(string(b))))
	if err != nil {
		t.Fatal(err)
	}
	for name, pair := range x.p.Streams {
		for field, id := range map[string]string{"stdout": pair.Stdout, "stderr": pair.Stderr} {
			b, err := os.ReadFile(x.f.manifest.Files[id])
			if err != nil {
				t.Fatal(err)
			}
			x.stream(t, name, field, []byte(replace.Replace(string(b))))
		}
	}
	x.save(t)
	s := x.f.expected.Sources["compiler"]
	s.SHA, s.Tree = sha, tree
	f := &ownedBaselineFixture{root: root, oracle: x,
		expected: baselineExpected{Schema: "cherri-baseline-expected/1", BaseSHA: base, Source: s, Publication: x.p, Files: x.f.expected.Files},
		manifest: baselineManifest{Schema: "cherri-baseline-locators/1", SourceDirectory: root, Files: x.f.manifest.Files},
		config:   finalProvenanceConfig{ExpectedPath: filepath.Join(t.TempDir(), "expected.json"), ManifestPath: filepath.Join(t.TempDir(), "locators.json")}}
	f.pin(t)
	return f
}

func (f *ownedBaselineFixture) pin(t *testing.T) {
	t.Helper()
	b, err := json.MarshalIndent(f.manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if err = os.WriteFile(f.config.ManifestPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	f.expected.ManifestSHA256 = finalProvenanceDigest(b)
	b, err = json.MarshalIndent(f.expected, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	b = append(b, '\n')
	if err = os.WriteFile(f.config.ExpectedPath, b, 0600); err != nil {
		t.Fatal(err)
	}
	f.config.ExpectedSHA256 = finalProvenanceDigest(b)
}

func TestOwnedBaselineControls(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testing.T, *ownedBaselineFixture)
	}{
		{"positive-owned-descendant", nil},
		{"dirty-current", func(t *testing.T, f *ownedBaselineFixture) {
			os.WriteFile(filepath.Join(f.root, "source.txt"), []byte("dirty"), 0600)
		}},
		{"untracked-current", func(t *testing.T, f *ownedBaselineFixture) {
			os.WriteFile(filepath.Join(f.root, "untracked.txt"), []byte("dirty"), 0600)
		}},
		{"hidden-index", func(t *testing.T, f *ownedBaselineFixture) {
			finalProvenanceTestGit(t, f.root, "update-index", "--assume-unchanged", "source.txt")
		}},
		{"wrong-local-ref", func(t *testing.T, f *ownedBaselineFixture) {
			finalProvenanceTestGit(t, f.root, "checkout", "-b", "agent/unrelated")
		}},
		{"wrong-origin", func(t *testing.T, f *ownedBaselineFixture) {
			finalProvenanceTestGit(t, f.root, "remote", "set-url", "origin", "https://github.com/example/unrelated.git")
		}},
		{"self-base", func(t *testing.T, f *ownedBaselineFixture) { f.expected.BaseSHA = f.expected.Source.SHA }},
		{"missing-base-object", func(t *testing.T, f *ownedBaselineFixture) { f.expected.BaseSHA = strings.Repeat("e", 40) }},
		{"nonancestor-base", func(t *testing.T, f *ownedBaselineFixture) {
			f.expected.BaseSHA = finalProvenanceTestGit(t, f.root, "commit-tree", f.expected.Source.Tree, "-m", "Synthetic unrelated root")
		}},
		{"wrong-tree", func(t *testing.T, f *ownedBaselineFixture) { f.expected.Source.Tree = strings.Repeat("e", 40) }},
		{"replacement-ref", func(t *testing.T, f *ownedBaselineFixture) {
			finalProvenanceTestGit(t, f.root, "replace", f.expected.Source.SHA, f.expected.BaseSHA)
		}},
		{"alternate-object-store", func(t *testing.T, f *ownedBaselineFixture) {
			os.WriteFile(filepath.Join(f.root, ".git/objects/info/alternates"), []byte("/tmp/other\n"), 0600)
		}},
		{"graft-file", func(t *testing.T, f *ownedBaselineFixture) {
			os.WriteFile(filepath.Join(f.root, ".git/info/grafts"), []byte(""), 0600)
		}},
		{"additional-worktree", func(t *testing.T, f *ownedBaselineFixture) {
			finalProvenanceTestGit(t, f.root, "worktree", "add", "-b", "agent/other", filepath.Join(t.TempDir(), "other"), f.expected.BaseSHA)
		}},
		{"linked-worktree", func(t *testing.T, f *ownedBaselineFixture) {
			other := filepath.Join(t.TempDir(), "linked")
			finalProvenanceTestGit(t, f.root, "worktree", "add", "--detach", other, f.expected.Source.SHA)
			f.manifest.SourceDirectory = other
		}},
		{"missing-publication-original", func(t *testing.T, f *ownedBaselineFixture) {
			os.Remove(f.manifest.Files[f.expected.Publication.Result])
		}},
		{"repinned-unrelated-ref-mutation", func(t *testing.T, f *ownedBaselineFixture) {
			x := f.oracle
			pair := x.p.Streams["14-remote-refs-after"]
			b, _ := os.ReadFile(x.f.manifest.Files[pair.Stdout])
			x.stream(t, "14-remote-refs-after", "stdout", append(b, []byte(strings.Repeat("f", 40)+"\trefs/heads/unrelated\n")...))
			x.save(t)
		}},
		{"unrepinned-stream-tamper", func(t *testing.T, f *ownedBaselineFixture) {
			os.WriteFile(f.manifest.Files[f.expected.Publication.Streams["03-head-before"].Stdout], []byte("wrong\n"), 0600)
		}},
		{"wrong-publication-format", func(t *testing.T, f *ownedBaselineFixture) { f.expected.Publication.Format = "source-create-only/1" }},
		{"unexpected-original", func(t *testing.T, f *ownedBaselineFixture) {
			f.expected.Files["extra"] = finalProvenanceFile{strings.Repeat("a", 64), 0}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := ownedBaselineFixtureNew(t)
			if tc.mutate != nil {
				tc.mutate(t, f)
			}
			f.pin(t)
			observation, err := executeOwnedBaseline(f.config, f.expected.Source.SHA, "")
			if tc.mutate == nil {
				if err != nil || observation.Status != "PASSED" || !observation.Before.Clean || !observation.After.Clean || len(observation.Commands) != 2 || observation.ClosureComplete {
					t.Fatalf("positive control failed: %v %+v", err, observation)
				}
			} else if err == nil || observation.Status == "PASSED" {
				t.Fatalf("corruption accepted: %+v", observation)
			}
		})
	}
}

func TestOwnedBaselineDedicatedRecord(t *testing.T) {
	for _, key := range []string{"CHERRI_BASELINE_EXPECTED", "CHERRI_BASELINE_EXPECTED_SHA256", "CHERRI_BASELINE_MANIFEST"} {
		t.Setenv(key, "")
	}
	r := &Runner{results: map[string]CaseResult{}}
	r.runOwnedBaselineRequirement()
	if r.results["B01"].Status != "NOT_RUN" || len(r.records) != 0 {
		t.Fatal("absence granted baseline credit")
	}
	if err := validateOwnedBaselineRecord("B01", EvidenceRecord{TestID: "tools/language-acceptance:runBaseline"}); err == nil {
		t.Fatal("historical TestID spoof accepted")
	}
	if err := validateOwnedBaselineRecord("BRG01", EvidenceRecord{TestID: baselineTestID}); err == nil {
		t.Fatal("B01 granted BRG01")
	}
	f := ownedBaselineFixtureNew(t)
	t.Setenv("CHERRI_BASELINE_EXPECTED", f.config.ExpectedPath)
	t.Setenv("CHERRI_BASELINE_EXPECTED_SHA256", f.config.ExpectedSHA256)
	t.Setenv("CHERRI_BASELINE_MANIFEST", f.config.ManifestPath)
	r = &Runner{implSHA: f.expected.Source.SHA, results: map[string]CaseResult{}, artifactDir: t.TempDir()}
	r.runOwnedBaselineRequirement()
	if !r.results["B01"].Passed || len(r.records) != 1 {
		t.Fatalf("dedicated producer did not record one baseline: %+v", r.results["B01"])
	}
	if err := validateOwnedBaselineRecord("B01", r.records[0]); err != nil {
		t.Fatal(err)
	}
	record := r.records[0]
	record.Requirements = []string{"B01", "BRG01"}
	if err := validateOwnedBaselineRecord("B01", record); err == nil {
		t.Fatal("linked requirement inheritance accepted")
	}
	if err := os.WriteFile(filepath.Join(f.root, "source.txt"), []byte("changed after producer\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := validateOwnedBaselineRecord("B01", r.records[0]); err == nil {
		t.Fatal("revalidation accepted changed source")
	}
}
