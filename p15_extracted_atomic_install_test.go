package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

type p15AtomicToolState struct {
	files      map[string][]byte
	docsCommit string
}

func p15AtomicSnapshot(t *testing.T, f *p15ExtractedInstall) p15AtomicToolState {
	t.Helper()
	s := p15AtomicToolState{files: map[string][]byte{}, docsCommit: p15ExtractedRevision(t, filepath.Join(f.home, "docs"))}
	for _, name := range []string{"bin/cherri", "bin/cherri.commit", "bin/cherri.sha256", "bin/cherri-migrate", "bin/cherri-migrate.commit", "bin/cherri-migrate.sha256", "docs/fork-language/actions.md"} {
		s.files[name] = p15InstallRead(t, filepath.Join(f.home, filepath.FromSlash(name)))
	}
	return s
}

func p15AtomicStateReceipt(s p15AtomicToolState) map[string]any {
	files := map[string]any{}
	for path, data := range s.files {
		files[path] = map[string]any{"sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "bytes": len(data)}
	}
	return map[string]any{"files": files, "docs_commit": s.docsCommit}
}

func p15AtomicAssertRestored(t *testing.T, f *p15ExtractedInstall, before p15AtomicToolState) {
	t.Helper()
	after := p15AtomicSnapshot(t, f)
	if after.docsCommit != before.docsCommit {
		t.Errorf("interrupted update changed docs pin: %s -> %s", before.docsCommit, after.docsCommit)
	}
	for path, data := range before.files {
		if !bytes.Equal(after.files[path], data) {
			t.Errorf("interrupted update changed prior bytes: %s", path)
		}
	}
	if _, err := os.Stat(filepath.Join(f.home, ".install-lock")); !os.IsNotExist(err) {
		t.Errorf("completed rollback retained install lock: %v", err)
	}
	for _, pattern := range []string{filepath.Join(f.home, "bin", ".cherri-install.*"), filepath.Join(f.home, ".cherri-docs.*")} {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) != 0 {
			t.Errorf("completed rollback retained candidate state: %s %v %v", pattern, matches, err)
		}
	}
	if output, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "doctor.sh"), f.env); err != nil {
		t.Errorf("prior actual installation is unusable after rollback: %v\n%s", err, output)
	}
}

func p15AtomicUpdateEnv(t *testing.T, f *p15ExtractedInstall, compiler, migration []byte, docsCommit string) (map[string]string, *atomic.Int32, *atomic.Int32) {
	t.Helper()
	compilerURL, compilerRequests := p15InstallServer(t, compiler)
	migrationURL, migrationRequests := p15InstallServer(t, migration)
	env := p15InstallEnvCopy(f.env)
	env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = compilerURL, fmt.Sprintf("%x", sha256.Sum256(compiler))
	env["CHERRI_MIGRATE_PREBUILT_URL"], env["CHERRI_MIGRATE_PREBUILT_SHA256"] = migrationURL, fmt.Sprintf("%x", sha256.Sum256(migration))
	env["CHERRI_DOCS_REF"] = docsCommit
	return env, compilerRequests, migrationRequests
}

func TestP15RP45ExtractedInstallAndRollback(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP45", false)
	compilerCandidate := p15InstallRead(t, f.compiler)
	migrationPath := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15ExtractedRequireRole(t, migrationPath, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	migrationCandidate := p15InstallRead(t, migrationPath)
	docsRepo := filepath.FromSlash(f.env["CHERRI_DOCS_REPO"])
	originalDocs := p15ExtractedGit(t, docsRepo, "show", f.docsSHA+":fork-language/actions.md")
	// A small bare repository advances the branch to a different immutable
	// commit without another full checkout. The installer must choose the pin,
	// not that later branch; embedded Go provenance independently observes it.
	advancedRepo := filepath.Join(t.TempDir(), "advanced-source.git")
	p15ExtractedGit(t, f.root, "clone", "--quiet", "--bare", "--shared", f.root, advancedRepo)
	tree := strings.TrimSpace(string(p15ExtractedGit(t, advancedRepo, "rev-parse", f.sourceSHA+"^{tree}")))
	advancedSource := strings.TrimSpace(string(p15ExtractedGit(t, advancedRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit-tree", tree, "-p", f.sourceSHA, "-m", "Later branch commit must not be installed")))
	p15ExtractedGit(t, advancedRepo, "update-ref", "refs/heads/rp45-later", advancedSource)
	p15ExtractedGit(t, advancedRepo, "symbolic-ref", "HEAD", "refs/heads/rp45-later")
	updatedDocs := append(append([]byte{}, originalDocs...), []byte("\nRP45 reviewed update document fixture.\n")...)
	p15InstallWrite(t, filepath.Join(docsRepo, "fork-language", "actions.md"), updatedDocs)
	updatedDocsCommit := p15ExtractedCommit(t, docsRepo, "Later generated-docs fixture commit")
	if advancedSource == f.sourceSHA || updatedDocsCommit == f.docsSHA {
		t.Fatal("pin controls did not advance both branches")
	}
	f.env["CHERRI_REPO"] = filepath.ToSlash(advancedRepo)
	if err := os.RemoveAll(f.home); err != nil {
		t.Fatal(err)
	} // only this test's seeded temporary home
	setup := filepath.Join(f.skill, "scripts", "setup.sh")
	output, err := p15InstallRun(t, setup, f.env)
	p15ExtractedRecordControl(f, "fresh_exact_source_install", []byte(output), err)
	if err != nil {
		t.Fatalf("actual extracted fresh two-tool source install: %v\n%s", err, output)
	}
	p15ExtractedRequireRole(t, f.compiler, f.sourceSHA, "github.com/electrikmilk/cherri")
	p15ExtractedRequireRole(t, f.migration, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	if p15ExtractedRevision(t, filepath.Join(f.home, "src")) != f.sourceSHA {
		t.Fatal("installer followed the advanced source branch")
	}
	before := p15AtomicSnapshot(t, f)
	if before.docsCommit != f.docsSHA || !bytes.Equal(before.files["docs/fork-language/actions.md"], originalDocs) {
		t.Fatal("installer followed the advanced docs branch")
	}
	for _, name := range []string{"cherri", "cherri-migrate"} {
		if string(before.files["bin/"+name+".commit"]) != f.sourceSHA+"\n" || strings.TrimSpace(string(before.files["bin/"+name+".sha256"])) != fmt.Sprintf("%x", sha256.Sum256(before.files["bin/"+name])) {
			t.Fatalf("installed %s receipts do not match actual source/bytes", name)
		}
	}
	if bytes.Equal(before.files["bin/cherri"], compilerCandidate) || bytes.Equal(before.files["bin/cherri-migrate"], migrationCandidate) {
		t.Fatal("update controls require distinct actual clean build variants")
	}
	f.receipt["fresh_install_state"] = p15AtomicStateReceipt(before)
	f.receipt["advanced_source_branch"], f.receipt["advanced_docs_branch"] = advancedSource, updatedDocsCommit

	t.Run("offline_compatible_cache", func(t *testing.T) {
		var requests atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			http.Error(w, "offline installation must not request this endpoint", http.StatusServiceUnavailable)
		}))
		defer server.Close()
		env := p15InstallEnvCopy(f.env)
		env["CHERRI_REPO"], env["CHERRI_DOCS_REPO"] = server.URL+"/compiler", server.URL+"/docs"
		output, err := p15InstallRun(t, setup, env, "--offline")
		p15ExtractedRecordControl(f, t.Name(), []byte(output), err)
		if err != nil || requests.Load() != 0 {
			t.Fatalf("offline compatible two-tool cache: %v requests=%d\n%s", err, requests.Load(), output)
		}
		p15AtomicAssertRestored(t, f, before)
	})
	for _, member := range []string{"compiler", "migration"} {
		t.Run("corrupt_download_"+member, func(t *testing.T) {
			marker := filepath.Join(t.TempDir(), "must-not-execute")
			corrupt := []byte("#!/bin/sh\nprintf executed > " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\nexit 0\n")
			mainBytes, migrationBytes := compilerCandidate, migrationCandidate
			if member == "compiler" {
				mainBytes = corrupt
			} else {
				migrationBytes = corrupt
			}
			env, mainRequests, migrationRequests := p15AtomicUpdateEnv(t, f, mainBytes, migrationBytes, updatedDocsCommit)
			// Keep the authentic expected candidate digest while the loopback
			// service actually returns the explicitly adversarial control bytes.
			if member == "compiler" {
				env["CHERRI_PREBUILT_SHA256"] = fmt.Sprintf("%x", sha256.Sum256(compilerCandidate))
			} else {
				env["CHERRI_MIGRATE_PREBUILT_SHA256"] = fmt.Sprintf("%x", sha256.Sum256(migrationCandidate))
			}
			output, err := p15InstallRun(t, setup, env, "--update")
			p15ExtractedRecordControl(f, t.Name(), []byte(output), err)
			if err == nil || !strings.Contains(output, "CHECKSUM_MISMATCH") {
				t.Errorf("wrong corrupt %s download result: %v\n%s", member, err, output)
			}
			wantMigration := int32(0)
			if member == "migration" {
				wantMigration = 1
			}
			if mainRequests.Load() != 1 || migrationRequests.Load() != wantMigration {
				t.Errorf("fault did not observe the intended actual download: compiler=%d migration=%d", mainRequests.Load(), migrationRequests.Load())
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("checksum-invalid replacement executed: %v", err)
			}
			p15AtomicAssertRestored(t, f, before)
		})
	}
	for _, kind := range []string{"missing", "wrong_role"} {
		t.Run("explicit_migration_override_"+kind, func(t *testing.T) {
			// Stage the existing authentic bytes and exact docs pin. A false
			// success cannot contaminate the later interruption controls, but the
			// published selected companion must still be the one doctor accepts.
			env, mainRequests, migrationRequests := p15AtomicUpdateEnv(t, f, before.files["bin/cherri"], before.files["bin/cherri-migrate"], f.docsSHA)
			override := filepath.Join(t.TempDir(), "missing-migration")
			if kind == "wrong_role" {
				override = f.compiler
			}
			env["CHERRI_MIGRATE_BIN"] = filepath.ToSlash(override)
			output, err := p15InstallRun(t, setup, env, "--update")
			p15ExtractedRecordControl(f, t.Name(), []byte(output), err)
			if err == nil {
				t.Error("update reported a ready installation with an invalid explicitly selected migration tool")
			}
			if mainRequests.Load() != 1 || migrationRequests.Load() != 1 {
				t.Errorf("selected-companion control did not stage both authentic candidates: %d/%d", mainRequests.Load(), migrationRequests.Load())
			}
			p15AtomicAssertRestored(t, f, before)
		})
	}
	for _, tc := range []struct {
		name, signal, target string
		backup               bool
	}{
		{"migration_backup", "TERM", "previous-migrate", true},
		{"migration_binary", "HUP", "bin/cherri-migrate", false},
		{"migration_source_receipt", "INT", "bin/cherri-migrate.commit", false},
		{"migration_checksum_receipt", "TERM", "bin/cherri-migrate.sha256", false},
		{"documentation", "TERM", "docs", false},
	} {
		t.Run("interrupt_"+tc.name, func(t *testing.T) {
			env, mainRequests, migrationRequests := p15AtomicUpdateEnv(t, f, compilerCandidate, migrationCandidate, updatedDocsCommit)
			shim, marker := filepath.Join(t.TempDir(), "shim"), filepath.Join(t.TempDir(), "actual-rename-observed")
			pattern := p15ExtractedShellQuote(filepath.ToSlash(filepath.Join(f.home, filepath.FromSlash(tc.target))))
			if tc.backup {
				pattern = "*/" + tc.target
			}
			body := "#!/bin/sh\n" + p15ExtractedShellQuote(p15InstallRealCommand(t, "mv")) + " \"$@\" || exit $?\nif [ ! -e " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + " ]; then\n case \"${2:-}\" in\n " + pattern + ")\n  printf '%s\\n' \"$1\" \"$2\" > " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\n  kill -s " + tc.signal + " \"$PPID\"\n ;;\n esac\nfi\n"
			p15InstallWrite(t, filepath.Join(shim, "mv"), []byte(body))
			env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = shim, "mv"
			output, err := p15InstallRun(t, setup, env, "--update")
			p15ExtractedRecordControl(f, t.Name(), []byte(output), err)
			if err == nil {
				t.Error("actual signal interruption reported successful installation")
			}
			observed := p15InstallRead(t, marker)
			lines := strings.Split(strings.TrimSpace(string(observed)), "\n")
			if len(lines) != 2 || !strings.HasSuffix(strings.TrimSpace(lines[1]), tc.target) {
				t.Fatalf("signal did not follow the reviewed real rename: %s", observed)
			}
			if mainRequests.Load() != 1 || migrationRequests.Load() != 1 {
				t.Errorf("interruption did not stage both actual candidates: %d/%d", mainRequests.Load(), migrationRequests.Load())
			}
			p15AtomicAssertRestored(t, f, before)
			t.Logf("RP45_ACTUAL_SIGNAL signal=%s phase=%s rename_observed=true full_prior_pair_and_docs_restored=true", tc.signal, tc.name)
		})
	}
	t.Run("successful_complete_update", func(t *testing.T) {
		env, mainRequests, migrationRequests := p15AtomicUpdateEnv(t, f, compilerCandidate, migrationCandidate, updatedDocsCommit)
		output, err := p15InstallRun(t, setup, env, "--update")
		p15ExtractedRecordControl(f, t.Name(), []byte(output), err)
		if err != nil {
			t.Fatalf("actual complete two-tool update: %v\n%s", err, output)
		}
		after := p15AtomicSnapshot(t, f)
		if !bytes.Equal(after.files["bin/cherri"], compilerCandidate) || !bytes.Equal(after.files["bin/cherri-migrate"], migrationCandidate) || after.docsCommit != updatedDocsCommit || !bytes.Equal(after.files["docs/fork-language/actions.md"], updatedDocs) {
			t.Fatal("successful update did not activate the complete exact candidate installation")
		}
		for _, name := range []string{"cherri", "cherri-migrate"} {
			if string(after.files["bin/"+name+".commit"]) != f.sourceSHA+"\n" || strings.TrimSpace(string(after.files["bin/"+name+".sha256"])) != fmt.Sprintf("%x", sha256.Sum256(after.files["bin/"+name])) {
				t.Fatalf("successful update left inconsistent %s receipts", name)
			}
		}
		if mainRequests.Load() != 1 || migrationRequests.Load() != 1 {
			t.Errorf("successful update did not acquire exactly both candidates: %d/%d", mainRequests.Load(), migrationRequests.Load())
		}
		verifiedEnv := p15InstallEnvCopy(f.env)
		verifiedEnv["CHERRI_DOCS_REF"] = updatedDocsCommit
		if out, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "doctor.sh"), verifiedEnv); err != nil {
			t.Fatalf("updated actual installation is not usable: %v\n%s", err, out)
		}
		legacy := filepath.Join(t.TempDir(), "installed-update.cherri")
		p15InstallWrite(t, legacy, []byte("const value = text(\"RP45 actual installed migration\")\nshow(value)\n"))
		if out, stderr, err := f.run(t, f.migration, filepath.Dir(legacy), legacy, "--check"); err != nil || !bytes.Contains(out, []byte("can be migrated cleanly")) {
			t.Fatalf("updated migration tool cannot perform the public operation: %v %s %s", err, out, stderr)
		}
		f.receipt["successful_update_state"] = p15AtomicStateReceipt(after)
	})
}
