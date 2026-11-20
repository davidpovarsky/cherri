package main

import (
	"bytes"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15ExtractedRecordControl(f *p15ExtractedInstall, name string, output []byte, err error) {
	controls, _ := f.receipt["identity_controls"].([]any)
	f.receipt["identity_controls"] = append(controls, map[string]any{
		"name": name, "output_base64": base64.StdEncoding.EncodeToString(output), "success": err == nil,
	})
}

func p15ExtractedRequireRole(t *testing.T, binary, sourceSHA, module string) {
	t.Helper()
	p15ExtractedBuildIdentity(t, binary, sourceSHA)
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != module {
		t.Fatalf("actual binary role differs: got %q want %q", info.Path, module)
	}
}

// These controls use two real clean builds with identical language contracts.
// Reconciled sidecars cannot turn the distinct same-tree build into the pinned source.
func TestP15RP45InstalledBinaryOwnership(t *testing.T) {
	f := p15ExtractedFixture(t, "RP45", false)
	migration := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15ExtractedRequireRole(t, f.compiler, f.sourceSHA, "github.com/electrikmilk/cherri")
	p15ExtractedRequireRole(t, migration, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	p15InstallWrite(t, f.migration, p15InstallRead(t, migration))
	p15InstallWrite(t, f.migration+".commit", []byte(f.sourceSHA+"\n"))
	p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
	doctor := filepath.Join(f.skill, "scripts", "doctor.sh")
	f.env["CHERRI_BIN"] = filepath.ToSlash(f.compiler)
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy real pair: %v\n%s", err, output)
	}
	// A real distinct same-tree commit isolates source identity even after
	// later semantic changes alter the current capability fingerprints.
	otherRepo := filepath.Join(t.TempDir(), "same-tree-source")
	p15ExtractedGit(t, f.root, "clone", "--quiet", "--shared", "--no-checkout", f.root, otherRepo)
	p15ExtractedGit(t, otherRepo, "config", "core.autocrlf", "false")
	p15ExtractedGit(t, otherRepo, "checkout", "--quiet", "--detach", f.sourceSHA)
	p15ExtractedGit(t, otherRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "--allow-empty", "-qm", "Distinct clean source identity control")
	previous := p15ExtractedRevision(t, otherRepo)
	if previous == f.sourceSHA || !bytes.Equal(p15ExtractedGit(t, otherRepo, "rev-parse", "HEAD^{tree}"), p15ExtractedGit(t, f.root, "rev-parse", "HEAD^{tree}")) {
		t.Fatal("source control requires distinct real commits with identical trees")
	}
	oldCompiler := p15ExtractedBuild(t, otherRepo, previous, ".")
	p15ExtractedRequireRole(t, oldCompiler, previous, "github.com/electrikmilk/cherri")
	var contracts [2]map[string]any
	for i, binary := range []string{f.compiler, oldCompiler} {
		out, stderr, err := f.run(t, binary, t.TempDir(), "--capabilities-json")
		if err != nil || len(stderr) != 0 {
			t.Fatalf("real source-control capabilities: %v %s", err, stderr)
		}
		if err := json.Unmarshal(out, &contracts[i]); err != nil {
			t.Fatal(err)
		}
	}
	for _, key := range []string{"languageVersion", "catalogSchemaVersion", "protocolVersion", "schemaFingerprint", "codecFingerprint", "macroFingerprint"} {
		if contracts[0][key] == nil || contracts[0][key] != contracts[1][key] {
			t.Fatalf("source-only control changed %s", key)
		}
	}
	if p15ExtractedHash(t, f.compiler) == p15ExtractedHash(t, oldCompiler) {
		t.Fatal("distinct source builds unexpectedly have identical bytes")
	}
	f.receipt["distinct_compiler_source"] = previous
	f.receipt["distinct_compiler_sha256"] = p15ExtractedHash(t, oldCompiler)
	for _, tc := range []struct {
		name, diagnostic string
		mutate           func(t *testing.T)
	}{
		{"reconciled_wrong_source", "source revision", func(t *testing.T) {
			p15InstallWrite(t, f.compiler, p15InstallRead(t, oldCompiler))
			p15InstallWrite(t, f.compiler+".sha256", []byte(p15ExtractedHash(t, f.compiler)+"\n"))
			// The forged sidecar claims the desired current commit; the actual
			// Go binary still independently identifies the distinct clean commit.
			p15InstallWrite(t, f.compiler+".commit", []byte(f.sourceSHA+"\n"))
		}},
		{"wrong_migration_role", "tool role", func(t *testing.T) {
			p15InstallWrite(t, f.migration, p15InstallRead(t, f.compiler))
			p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
		}},
		{"missing_migration_binary", "migration", func(t *testing.T) {
			if err := os.Remove(f.migration); err != nil {
				t.Fatal(err)
			}
		}},
		{"migration_checksum_before_execution", "CHECKSUM_MISMATCH", func(t *testing.T) {
			p15InstallWrite(t, f.migration, append(p15InstallRead(t, f.migration), []byte("RP45 changed companion bytes")...))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			original := map[string][]byte{}
			for _, path := range []string{f.compiler, f.compiler + ".commit", f.compiler + ".sha256", f.migration, f.migration + ".commit", f.migration + ".sha256"} {
				original[path] = p15InstallRead(t, path)
			}
			t.Cleanup(func() {
				for path, data := range original {
					p15InstallWrite(t, path, data)
				}
			})
			tc.mutate(t)
			output, err := p15InstallRun(t, doctor, f.env)
			p15ExtractedRecordControl(f, tc.name, []byte(output), err)
			if err == nil || !strings.Contains(output, tc.diagnostic) {
				t.Fatalf("doctor accepted/misdiagnosed actual ownership failure %s: %v expected=%s\n%s", tc.name, err, tc.diagnostic, output)
			}
		})
	}
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy pair after every ownership control: %v\n%s", err, output)
	}
}

func TestP15RP45RuntimePackageOwnership(t *testing.T) {
	f := p15ExtractedFixture(t, "RP45", false)
	migration := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15ExtractedRequireRole(t, migration, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	for _, tc := range []struct{ name, binary, diagnostic string }{
		{"missing_companion", f.compiler, "migration tool"},
		{"wrong_compiler_role", migration, "main package"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			archive := filepath.Join(t.TempDir(), "rejected.zip")
			args := []string{"create", "--repo", f.packageRepo, "--skill-commit", f.skillSHA, "--kind", "runtime", "--compiler", tc.binary, "--archive", archive}
			output, err := p15PackageRun(t, f.root, args...)
			p15ExtractedRecordControl(f, tc.name, []byte(output), err)
			if err == nil || !strings.Contains(output, tc.diagnostic) {
				t.Fatalf("packager accepted/misdiagnosed actual role/pair failure: %v expected=%s\n%s", err, tc.diagnostic, output)
			}
			if _, statErr := os.Stat(archive); !os.IsNotExist(statErr) {
				t.Fatalf("invalid package left an apparent delivery archive: %v", statErr)
			}
		})
	}
}

// Each replacement is confined to a disposable repository. An immutable package
// request must produce the original committed bytes even when Git's convenience
// replacement refs would make ordinary tree/blob reads return another object.
func TestP15RP45PackageIgnoresGitReplacements(t *testing.T) {
	f := p15ExtractedFixture(t, "RP45", false)
	want := f.receipt["archive_sha256"].(string)
	for _, kind := range []string{"commit", "tree", "blob"} {
		t.Run(kind, func(t *testing.T) {
			repo := filepath.Join(t.TempDir(), "replaced-skill")
			p15ExtractedGit(t, f.root, "clone", "--quiet", "--shared", "--no-checkout", f.packageRepo, repo)
			p15ExtractedGit(t, repo, "config", "core.autocrlf", "false")
			p15ExtractedGit(t, repo, "checkout", "--quiet", "--detach", f.skillSHA)
			path := "skills/cherri-shortcuts/SKILL.md"
			original := p15ExtractedGit(t, repo, "show", f.skillSHA+":"+path)
			p15InstallWrite(t, filepath.Join(repo, filepath.FromSlash(path)), append(append([]byte{}, original...), []byte("\nRP45 replacement object must not be packaged.\n")...))
			replacement := p15ExtractedCommit(t, repo, "Disposable replacement object")
			oldOID, newOID := f.skillSHA, replacement
			if kind == "tree" {
				oldOID += "^{tree}"
				newOID += "^{tree}"
			}
			if kind == "blob" {
				oldOID += ":" + path
				newOID += ":" + path
			}
			oldOID = strings.TrimSpace(string(p15ExtractedGit(t, repo, "rev-parse", oldOID)))
			newOID = strings.TrimSpace(string(p15ExtractedGit(t, repo, "rev-parse", newOID)))
			if oldOID == newOID {
				t.Fatal("replacement control did not change its exact object")
			}
			p15ExtractedGit(t, repo, "replace", oldOID, newOID)
			archive := filepath.Join(t.TempDir(), "immutable.zip")
			args := []string{"--repo", repo, "--skill-commit", f.skillSHA, "--kind", "source", "--archive", archive}
			output, err := p15PackageRun(t, f.root, append([]string{"create"}, args...)...)
			p15ExtractedRecordControl(f, "replace_"+kind, []byte(output), err)
			if err != nil {
				t.Fatalf("immutable original package request failed under replacement: %v\n%s", err, output)
			}
			if verify, err := p15PackageRun(t, f.root, append([]string{"verify"}, args...)...); err != nil {
				t.Fatalf("actual package verifier: %v\n%s", err, verify)
			}
			if got := p15ExtractedHash(t, archive); got != want {
				t.Fatalf("package rebound an immutable %s to replacement bytes: original_archive=%s replacement_archive=%s old=%s new=%s", kind, want, got, oldOID, newOID)
			}
			entries := p15PackageEntries(t, archive)
			found := false
			for _, entry := range entries {
				if entry.header.Name == "cherri-shortcuts/SKILL.md" {
					found = true
					if !bytes.Equal(entry.data, original) {
						t.Fatal("immutable Skill source bytes differ")
					}
				}
			}
			if !found {
				t.Fatal("actual package omitted the original Skill source")
			}
		})
	}
}
