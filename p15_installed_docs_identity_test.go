package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP15RP44InstalledDocsIdentity(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP44", false)
	migration := p15InstallActualTool(t, f.root, f.sourceSHA, "./cmd/cherri-migrate", "")
	p15InstallWrite(t, f.migration, p15InstallRead(t, migration))
	p15InstallWrite(t, f.migration+".commit", []byte(f.sourceSHA+"\n"))
	p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
	f.env["CHERRI_BIN"] = filepath.ToSlash(f.compiler)
	doctor := filepath.Join(f.skill, "scripts", "doctor.sh")
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy real tools and generated pinned docs: %v\n%s", err, output)
	}
	docs := filepath.Join(f.home, "docs")
	path := filepath.Join(docs, "fork-language", "actions.md")
	original := p15InstallRead(t, path)
	manifest := f.receipt["component_manifest"].(map[string]any)
	for _, name := range []string{"changed_docs_preserve_fingerprints", "missing_git_provenance", "wrong_commit", "wrong_schema", "wrong_macros"} {
		t.Run(name, func(t *testing.T) {
			env := p15InstallEnvCopy(f.env)
			t.Cleanup(func() { p15InstallWrite(t, path, original) })
			diagnostic := ""
			switch name {
			case "changed_docs_preserve_fingerprints":
				p15InstallWrite(t, path, append(append([]byte{}, original...), []byte("\nRP44 uncommitted documentation drift.\n")...))
				if p15ExtractedRevision(t, docs) != f.docsSHA {
					t.Fatal("dirty docs control changed the commit pin")
				}
				diagnostic = "DOCS_DIRTY"
			case "missing_git_provenance":
				gitPath, saved := filepath.Join(docs, ".git"), filepath.Join(t.TempDir(), "original-git-directory")
				if err := os.Rename(gitPath, saved); err != nil {
					t.Fatal(err)
				}
				t.Cleanup(func() {
					if err := os.Rename(saved, gitPath); err != nil {
						t.Error(err)
					}
				})
				diagnostic = "DOCS_GIT_MISSING"
			case "wrong_commit":
				env["CHERRI_DOCS_REF"] = strings.Repeat("f", 40)
				diagnostic = "STALE_DOCS_COMMIT"
			case "wrong_schema", "wrong_macros":
				key := "schemaFingerprint"
				diagnostic = "STALE_DOCS_SCHEMA"
				if name == "wrong_macros" {
					key = "macroFingerprint"
					diagnostic = "STALE_DOCS_MACROS"
				}
				changed := bytes.Replace(original, []byte(manifest[key].(string)), []byte(strings.Repeat("0", 64)), 1)
				if bytes.Equal(changed, original) {
					t.Fatal("actual generated docs fingerprint control did not change bytes")
				}
				p15InstallWrite(t, path, changed)
			}
			output, err := p15InstallRun(t, doctor, env)
			p15ExtractedRecordControl(f, name, []byte(output), err)
			if err == nil || !strings.Contains(output, diagnostic) {
				t.Fatalf("doctor accepted/misdiagnosed installed documentation drift %s: %v expected=%s\n%s", name, err, diagnostic, output)
			}
		})
	}
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("actual pinned docs did not recover after each isolated control: %v\n%s", err, output)
	}
}
