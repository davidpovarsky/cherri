package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15InstallIndexFlag(t *testing.T, dir, path, flag string) string {
	t.Helper()
	p15ExtractedGit(t, dir, "update-index", flag, "--", path)
	observed := string(p15ExtractedGit(t, dir, "ls-files", "-v", "--", path))
	if strings.TrimSpace(string(p15ExtractedGit(t, dir, "status", "--porcelain"))) != "" {
		t.Fatal("Git-index counterexample did not conceal the edited tracked file")
	}
	if flag == "--assume-unchanged" && !strings.HasPrefix(observed, "h ") || flag == "--skip-worktree" && !strings.HasPrefix(observed, "S ") {
		t.Fatalf("Git-index flag was not independently observed: %q", observed)
	}
	return observed
}

func p15InstallNoResidue(t *testing.T, home string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(home, ".install-lock")); !os.IsNotExist(err) {
		t.Errorf("completed operation retained install lock: %v", err)
	}
	for _, pattern := range []string{filepath.Join(home, "bin", ".cherri-install.*"), filepath.Join(home, ".cherri-docs.*")} {
		paths, err := filepath.Glob(pattern)
		if err != nil || len(paths) != 0 {
			t.Errorf("completed operation retained staging files: %v %v", paths, err)
		}
	}
}

func TestP15RP44GitIndexDocsIntegrity(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP44", true)
	docs := filepath.Join(f.home, "docs")
	rel := "fork-language/actions.md"
	path := filepath.Join(docs, filepath.FromSlash(rel))
	original := p15InstallRead(t, path)
	compiler, migration := p15InstallRead(t, f.compiler), p15InstallRead(t, f.migration)
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		t.Run(strings.TrimPrefix(flag, "--"), func(t *testing.T) {
			t.Cleanup(func() {
				p15ExtractedGit(t, docs, "update-index", "--no-assume-unchanged", "--", rel)
				p15ExtractedGit(t, docs, "update-index", "--no-skip-worktree", "--", rel)
				p15InstallWrite(t, path, original)
			})
			changed := append(append([]byte{}, original...), []byte("\nRP44 locally edited documentation hidden by an index flag.\n")...)
			p15InstallWrite(t, path, changed)
			observed := p15InstallIndexFlag(t, docs, rel, flag)
			out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "doctor.sh"), f.env)
			p15ExtractedRecordControl(f, t.Name()+"_doctor", []byte(out), err)
			if err == nil || !strings.Contains(out, "DOCS_INDEX_UNVERIFIED") {
				t.Errorf("doctor accepted or misdiagnosed hidden tracked documentation: %v\n%s", err, out)
			}
			env, _, _ := p15AtomicUpdateEnv(t, f, compiler, migration, f.docsSHA)
			out, err = p15InstallRun(t, filepath.Join(f.skill, "scripts", "setup.sh"), env, "--update")
			p15ExtractedRecordControl(f, t.Name()+"_update", []byte(out), err)
			if err == nil || !strings.Contains(out, "DOCS_INDEX_UNVERIFIED") {
				t.Errorf("update accepted or misdiagnosed hidden tracked documentation: %v\n%s", err, out)
			}
			if !bytes.Equal(p15InstallRead(t, path), changed) {
				t.Error("update overwrote the caller's hidden local documentation")
			}
			if got := string(p15ExtractedGit(t, docs, "ls-files", "-v", "--", rel)); got != observed {
				t.Errorf("validation changed the caller's index flags: before=%q after=%q", observed, got)
			}
			if !bytes.Equal(p15InstallRead(t, f.compiler), compiler) || !bytes.Equal(p15InstallRead(t, f.migration), migration) {
				t.Error("rejected docs update changed installed tool bytes")
			}
			p15InstallNoResidue(t, f.home)
		})
	}
	if out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "doctor.sh"), f.env); err != nil {
		t.Fatalf("healthy exact docs did not recover after isolated controls: %v\n%s", err, out)
	}
}

func TestP15RP45GitIndexSourceIntegrity(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP45", true)
	source := filepath.Join(f.home, "src")
	rel := "main.go"
	path := filepath.Join(source, rel)
	original := p15InstallRead(t, path)
	before := p15AtomicSnapshot(t, f)
	for _, flag := range []string{"--assume-unchanged", "--skip-worktree"} {
		t.Run(strings.TrimPrefix(flag, "--"), func(t *testing.T) {
			t.Cleanup(func() {
				p15ExtractedGit(t, source, "update-index", "--no-assume-unchanged", "--", rel)
				p15ExtractedGit(t, source, "update-index", "--no-skip-worktree", "--", rel)
				p15InstallWrite(t, path, original)
				// A failed baseline may have published a new build. Restore only
				// this disposable fixture after recording that real failure.
				for file, data := range before.files {
					p15InstallWrite(t, filepath.Join(f.home, filepath.FromSlash(file)), data)
				}
			})
			changed := append(append([]byte{}, original...), []byte("\n// RP45 owned comment must not be stamped as the unmodified source pin.\n")...)
			p15InstallWrite(t, path, changed)
			observed := p15InstallIndexFlag(t, source, rel, flag)
			marker := filepath.Join(t.TempDir(), "go-was-invoked")
			shim := filepath.Join(t.TempDir(), "shim")
			body := "#!/bin/sh\nprintf invoked > " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\nexec " + p15ExtractedShellQuote(p15InstallRealCommand(t, "go")) + " \"$@\"\n"
			p15InstallWrite(t, filepath.Join(shim, "go"), []byte(body))
			env := p15InstallEnvCopy(f.env)
			env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = shim, "go"
			out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "setup.sh"), env, "--update")
			p15ExtractedRecordControl(f, t.Name(), []byte(out), err)
			if err == nil || !strings.Contains(out, "SOURCE_INDEX_UNVERIFIED") {
				t.Errorf("update accepted or misdiagnosed hidden tracked Go source: %v\n%s", err, out)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("unverified hidden source reached the actual Go build command: %v", err)
			}
			if !bytes.Equal(p15InstallRead(t, path), changed) {
				t.Error("validation overwrote the caller's hidden local Go source")
			}
			if got := string(p15ExtractedGit(t, source, "ls-files", "-v", "--", rel)); got != observed {
				t.Errorf("validation changed caller source index flags: before=%q after=%q", observed, got)
			}
			p15AtomicAssertRestored(t, f, before)
		})
	}
}

func TestP15RP45LockAcquisitionSignals(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP45", true)
	before := p15AtomicSnapshot(t, f)
	lock := filepath.Join(f.home, ".install-lock")
	for _, signal := range []string{"HUP", "INT", "TERM"} {
		t.Run("after_mkdir_"+signal, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "owned-lock-created")
			shim := filepath.Join(t.TempDir(), "shim")
			t.Cleanup(func() {
				if _, err := os.Stat(marker); err == nil {
					if err := os.RemoveAll(lock); err != nil {
						t.Error(err)
					}
				}
			})
			body := "#!/bin/sh\n" + p15ExtractedShellQuote(p15InstallRealCommand(t, "mkdir")) + " \"$@\" || exit $?\nif [ \"${1:-}\" = " + p15ExtractedShellQuote(filepath.ToSlash(lock)) + " ] && [ ! -e " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + " ]; then\n printf '%s\\n' \"$1\" > " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\n kill -s " + signal + " \"$PPID\"\nfi\n"
			p15InstallWrite(t, filepath.Join(shim, "mkdir"), []byte(body))
			env, _, _ := p15AtomicUpdateEnv(t, f, before.files["bin/cherri"], before.files["bin/cherri-migrate"], f.docsSHA)
			env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = shim, "mkdir"
			out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "setup.sh"), env, "--update")
			p15ExtractedRecordControl(f, t.Name(), []byte(out), err)
			if err == nil {
				t.Error("handled post-mkdir signal reported install success")
			}
			if string(p15InstallRead(t, marker)) != filepath.ToSlash(lock)+"\n" {
				t.Fatal("signal did not observe the successful exact lock creation")
			}
			p15AtomicAssertRestored(t, f, before)
		})
	}
	t.Run("existing_owner_preserved", func(t *testing.T) {
		marker := filepath.Join(lock, "other-owner")
		data := []byte("RP45 independently owned active lock\n")
		p15InstallWrite(t, marker, data)
		t.Cleanup(func() {
			if err := os.RemoveAll(lock); err != nil {
				t.Error(err)
			}
		})
		env, mainRequests, migrationRequests := p15AtomicUpdateEnv(t, f, before.files["bin/cherri"], before.files["bin/cherri-migrate"], f.docsSHA)
		out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "setup.sh"), env, "--update")
		p15ExtractedRecordControl(f, t.Name(), []byte(out), err)
		if err == nil || !strings.Contains(out, "INSTALL_BUSY") || mainRequests.Load() != 0 || migrationRequests.Load() != 0 {
			t.Errorf("existing owner's lock was not rejected before candidate acquisition: %v %d/%d\n%s", err, mainRequests.Load(), migrationRequests.Load(), out)
		}
		if !bytes.Equal(p15InstallRead(t, marker), data) {
			t.Error("setup removed or altered another lock owner")
		}
		for path, data := range before.files {
			if !bytes.Equal(p15InstallRead(t, filepath.Join(f.home, filepath.FromSlash(path))), data) {
				t.Errorf("busy-lock rejection changed installed bytes: %s", path)
			}
		}
	})
}
