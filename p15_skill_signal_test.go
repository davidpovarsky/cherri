package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func p15InstallRealCommand(t *testing.T, name string) string {
	t.Helper()
	out, err := exec.Command(p15SkillBash(t), "--noprofile", "--norc", "-c", "command -v "+name).Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func TestP15K03HandledSignalRestoresFiles(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	candidate := p15InstallCandidate(t, root)
	for _, tc := range []struct {
		name, pattern string
		fresh         bool
	}{
		{"original_compiler", "*/bin/cherri", false},
		{"original_commit", "*/bin/cherri.commit", false},
		{"original_checksum", "*/bin/cherri.sha256", false},
		{"original_docs", "*/home/docs", false},
		{"candidate_compiler", "*/.cherri-install.*/candidate", false},
		{"candidate_commit", "*/.cherri-install.*/candidate.commit", false},
		{"candidate_checksum", "*/.cherri-install.*/candidate.sha256", false},
		{"candidate_docs", "*/.cherri-docs.*/candidate", false},
		{"fresh_compiler", "*/.cherri-install.*/candidate", true},
		{"fresh_docs", "*/.cherri-docs.*/candidate", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, p15InstallActualCompiler(t))
			cache := filepath.Join(f.home, "bin", "cherri")
			p15InstallWrite(t, cache+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(p15InstallRead(t, cache)))))
			old := p15InstallState(t, f)
			if tc.fresh {
				if err := os.RemoveAll(filepath.Join(f.home, "bin")); err != nil {
					t.Fatal(err)
				}
				if err := os.RemoveAll(filepath.Join(f.home, "docs")); err != nil {
					t.Fatal(err)
				}
			}
			url, requests := p15InstallServer(t, candidate)
			env := p15InstallEnvCopy(f.env)
			env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, fmt.Sprintf("%x", sha256.Sum256(candidate))
			env["CHERRI_DOCS_REF"] = f.advanceGoodDocs(t)
			if tc.fresh {
				migrationURL, _ := p15InstallServer(t, old.migration)
				env["CHERRI_MIGRATE_PREBUILT_URL"], env["CHERRI_MIGRATE_PREBUILT_SHA256"] = migrationURL, fmt.Sprintf("%x", sha256.Sum256(old.migration))
			}
			shimDir := filepath.Join(t.TempDir(), "shim")
			marker := filepath.Join(f.home, "signal-after-real-move")
			shim := "#!/bin/sh\nset -e\n" + p15InstallShellQuote(p15InstallRealCommand(t, "mv")) + " \"$@\"\ncase \"$1\" in\n" + tc.pattern + ")\nprintf 'REAL_MOVE_COMPLETED\\n' > " + p15InstallShellQuote(marker) + "\nkill -s TERM \"$PPID\"\n;;\nesac\n"
			p15InstallWrite(t, filepath.Join(shimDir, "mv"), []byte(shim))
			env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = filepath.ToSlash(shimDir), "mv"
			out, runErr := p15InstallRun(t, f.script("setup"), env, "--update")
			if runErr == nil {
				t.Fatalf("handled signal was ignored:\n%s", out)
			}
			if string(p15InstallRead(t, marker)) != "REAL_MOVE_COMPLETED\n" {
				t.Fatal("signal was not sent after a completed real move")
			}
			if requests.Load() != 1 {
				t.Errorf("signal control downloaded %d times", requests.Load())
			}
			if tc.fresh {
				for _, p := range []string{cache, cache + ".commit", cache + ".sha256", filepath.Join(f.home, "bin", "cherri-migrate"), filepath.Join(f.home, "bin", "cherri-migrate.commit"), filepath.Join(f.home, "bin", "cherri-migrate.sha256"), filepath.Join(f.home, "docs")} {
					if _, err := os.Stat(p); !os.IsNotExist(err) {
						t.Errorf("interrupted fresh install left active artifact %s: %v", p, err)
					}
				}
				p15AssertInstallCleaned(t, f)
			} else {
				if _, err := os.Stat(cache); err != nil {
					t.Fatalf("handled signal lost prior working compiler after completed move: %v", err)
				}
				p15AssertInstallPreserved(t, f, old)
			}
			t.Logf("K03_SIGNAL_CONTROL case=%s move_completed=true signal=TERM rollback=true", tc.name)
		})
	}
}

func TestP15K03PromotedCacheCannotFallBack(t *testing.T) {
	f := p15NewInstallFixture(t)
	f.seedActualPair(t, p15InstallActualCompiler(t))
	old := p15InstallState(t, f)
	bundle := filepath.Join(f.skill, "bin", "cherri")
	p15InstallCopyPair(t, filepath.Join(f.home, "bin"), filepath.Dir(bundle))
	candidate := p15InstallCandidate(t, f.root)
	url, _ := p15InstallServer(t, candidate)
	env := p15InstallEnvCopy(f.env)
	env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, fmt.Sprintf("%x", sha256.Sum256(candidate))
	env["CHERRI_DOCS_REF"] = f.advanceGoodDocs(t)
	shimDir := filepath.Join(t.TempDir(), "shim")
	marker := filepath.Join(f.home, "promoted-checksum-corrupted")
	shim := "#!/bin/sh\nset -e\n" + p15InstallShellQuote(p15InstallRealCommand(t, "mv")) + " \"$@\"\ncase \"$1\" in\n*/.cherri-install.*/candidate.sha256)\nprintf '%s\\n' '" + strings.Repeat("0", 64) + "' > \"$2\"\nprintf 'CORRUPTED_AFTER_PROMOTION\\n' > " + p15InstallShellQuote(marker) + "\n;;\nesac\n"
	p15InstallWrite(t, filepath.Join(shimDir, "mv"), []byte(shim))
	env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = filepath.ToSlash(shimDir), "mv"
	out, err := p15InstallRun(t, f.script("setup"), env, "--update")
	if string(p15InstallRead(t, marker)) != "CORRUPTED_AFTER_PROMOTION\n" {
		t.Fatal("promoted-byte fault did not execute")
	}
	if err == nil || !strings.Contains(out, "CHERRI_INSTALL_PROMOTED_INVALID") {
		t.Fatalf("promoted cache failure silently accepted a fallback: %v\n%s", err, out)
	}
	p15AssertInstallPreserved(t, f, old)
	if string(p15InstallRead(t, bundle)) != string(old.binary) {
		t.Error("rejected update changed unrelated bundled compiler")
	}
	t.Log("K03_PROMOTED_CONTROL damaged promoted candidate rejected despite healthy bundled fallback")
}
