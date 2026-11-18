package main

import (
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func p15InstallGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func p15InstallWrite(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0755); err != nil {
		t.Fatal(err)
	}
}

func p15InstallRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func p15InstallEnv(values map[string]string) []string {
	env := make([]string, 0, len(os.Environ())+len(values))
	for _, value := range os.Environ() {
		key := strings.SplitN(value, "=", 2)[0]
		if strings.HasPrefix(key, "CHERRI_") || key == "SKILL_DIR" {
			continue
		}
		env = append(env, value)
	}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}

func p15InstallRun(t *testing.T, script string, env map[string]string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	all := append([]string{"--noprofile", "--norc", filepath.ToSlash(script)}, args...)
	if env["P15_INSTALL_SHIM_DIR"] != "" {
		all = append([]string{"--noprofile", "--norc", "-c", "if [ -n \"$P15_INSTALL_SHIM_DIR\" ]; then\n  _p15_shim=$P15_INSTALL_SHIM_DIR\n  if command -v cygpath >/dev/null 2>&1; then _p15_shim=$(cygpath -u \"$_p15_shim\"); fi\n  PATH=\"$_p15_shim:$PATH\"\n  export PATH\n  if [ \"$(command -v \"$P15_INSTALL_SHIM_COMMAND\")\" != \"$_p15_shim/$P15_INSTALL_SHIM_COMMAND\" ]; then\n    echo '[P15_SHIM_NOT_SELECTED]' >&2\n    exit 89\n  fi\nfi\n. \"$0\"", filepath.ToSlash(script)}, args...)
	}
	cmd := exec.CommandContext(ctx, p15SkillBash(t), all...)
	cmd.Env = p15InstallEnv(env)
	output, err := cmd.CombinedOutput()
	return string(output), err
}

type p15InstallFixture struct {
	root, skill, home, docsRepo, sourceSHA, docsSHA string
	env                                             map[string]string
}

func p15NewInstallFixture(t *testing.T) p15InstallFixture {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	skillRoot := p15CopySkillOracle(t)
	skill := filepath.Join(skillRoot, "skills", "cherri-shortcuts")
	tmp := t.TempDir()
	docsRepo := filepath.Join(tmp, "canonical-docs")
	p15InstallWrite(t, filepath.Join(docsRepo, "fork-language", "actions.md"),
		p15InstallRead(t, filepath.Join(root, "docs", "language-v2", "actions-reference.md")))
	p15InstallGit(t, docsRepo, "init", "-q")
	p15InstallGit(t, docsRepo, "-c", "core.autocrlf=false", "add", "fork-language/actions.md")
	p15InstallGit(t, docsRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "-qm", "Canonical real generated docs")
	f := p15InstallFixture{
		root: root, skill: skill, home: filepath.Join(tmp, "home"), docsRepo: docsRepo,
		sourceSHA: p15InstallGit(t, root, "rev-parse", "HEAD"),
		docsSHA:   p15InstallGit(t, docsRepo, "rev-parse", "HEAD"),
	}
	f.env = map[string]string{
		"SKILL_DIR": filepath.ToSlash(f.skill), "CHERRI_HOME": filepath.ToSlash(f.home),
		"CHERRI_REPO": filepath.ToSlash(f.root), "CHERRI_REF": f.sourceSHA,
		"CHERRI_DOCS_REPO": filepath.ToSlash(f.docsRepo), "CHERRI_DOCS_REF": f.docsSHA,
	}
	return f
}

func (f p15InstallFixture) script(name string) string {
	return filepath.Join(f.skill, "scripts", name+".sh")
}

func (f p15InstallFixture) seedActualPair(t *testing.T, binary string) {
	t.Helper()
	p15InstallWrite(t, filepath.Join(f.home, "bin", "cherri"), p15InstallRead(t, binary))
	p15InstallWrite(t, filepath.Join(f.home, "bin", "cherri.commit"), []byte(f.sourceSHA+"\n"))
	p15InstallGit(t, f.root, "clone", "--quiet", "--no-hardlinks", f.docsRepo, filepath.Join(f.home, "docs"))
	out, err := p15InstallRun(t, f.script("doctor"), f.env)
	if err != nil {
		t.Fatalf("healthy actual binary and canonical docs control failed: %v\n%s", err, out)
	}
}

func TestP15K03LateDocsFailurePreservesWorkingPair(t *testing.T) {
	f := p15NewInstallFixture(t)
	f.seedActualPair(t, roundTripBinary(t))
	oldPath := filepath.Join(f.home, "bin", "cherri")
	oldHash := sha256.Sum256(p15InstallRead(t, oldPath))
	oldCommit := p15InstallRead(t, oldPath+".commit")
	oldDoc := p15InstallRead(t, filepath.Join(f.home, "docs", "fork-language", "actions.md"))

	// A second real compiler build has different bytes but identical reviewed
	// source/capabilities. The update must not commit it before docs validation.
	nextBinary := filepath.Join(t.TempDir(), "next-cherri")
	if isWindowsRuntime() {
		nextBinary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	build := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-buildid=cherri-k03-controlled-build", "-o", nextBinary, ".")
	build.Dir = f.root
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("second actual compiler build: %v\n%s", err, out)
	}
	nextBytes := p15InstallRead(t, nextBinary)
	nextHash := sha256.Sum256(nextBytes)
	if nextHash == oldHash {
		t.Fatal("atomic-update control requires distinct real compiler bytes")
	}
	var downloads atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cherri" {
			http.NotFound(w, r)
			return
		}
		downloads.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(nextBytes)
	}))
	defer server.Close()

	var manifest map[string]any
	if err := json.Unmarshal(p15InstallRead(t, filepath.Join(f.skill, "compatibility-manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	expectedFP := manifest["schemaFingerprint"].(string)
	badDoc := strings.Replace(string(oldDoc), expectedFP, strings.Repeat("0", 64), 1)
	if badDoc == string(oldDoc) {
		t.Fatal("canonical docs schema fault anchor absent")
	}
	p15InstallWrite(t, filepath.Join(f.docsRepo, "fork-language", "actions.md"), []byte(badDoc))
	p15InstallGit(t, f.docsRepo, "-c", "core.autocrlf=false", "add", "fork-language/actions.md")
	p15InstallGit(t, f.docsRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "-qm", "Isolated invalid docs schema")
	badDocsSHA := p15InstallGit(t, f.docsRepo, "rev-parse", "HEAD")
	f.env["CHERRI_PREBUILT_URL"] = server.URL + "/cherri"
	f.env["CHERRI_PREBUILT_SHA256"] = fmt.Sprintf("%x", nextHash)
	f.env["CHERRI_DOCS_REF"] = badDocsSHA
	out, err := p15InstallRun(t, f.script("setup"), f.env, "--update")
	if err == nil || !strings.Contains(out, "CHERRI_STALE_DOCS_SCHEMA") {
		t.Fatalf("expected isolated late docs rejection: %v\n%s", err, out)
	}
	if downloads.Load() != 1 {
		t.Fatalf("expected exactly one controlled binary request, got %d", downloads.Load())
	}
	gotHash := sha256.Sum256(p15InstallRead(t, oldPath))
	if gotHash != oldHash {
		t.Errorf("late docs failure partially replaced working compiler: old=%x new=%x staged=%x", oldHash, gotHash, nextHash)
	}
	if string(p15InstallRead(t, oldPath+".commit")) != string(oldCommit) {
		t.Error("late docs failure changed installed compiler commit receipt")
	}
	gotDocsSHA := p15InstallGit(t, filepath.Join(f.home, "docs"), "rev-parse", "HEAD")
	if gotDocsSHA != f.docsSHA || string(p15InstallRead(t, filepath.Join(f.home, "docs", "fork-language", "actions.md"))) != string(oldDoc) {
		t.Errorf("late docs failure partially replaced working docs: old=%s new=%s", f.docsSHA, gotDocsSHA)
	}
	f.env["CHERRI_DOCS_REF"] = f.docsSHA
	delete(f.env, "CHERRI_PREBUILT_URL")
	delete(f.env, "CHERRI_PREBUILT_SHA256")
	if after, err := p15InstallRun(t, f.script("doctor"), f.env); err != nil {
		t.Errorf("previous verified pair is unusable after failed update: %v\n%s", err, after)
	}
	t.Logf("K03_LATE_DOCS_CONTROL source=%s old_binary=%x candidate_binary=%x old_docs=%s invalid_docs=%s requests=%d",
		f.sourceSHA, oldHash, nextHash, f.docsSHA, badDocsSHA, downloads.Load())
}

func p15InstallCandidate(t *testing.T, root string) []byte {
	t.Helper()
	path := filepath.Join(t.TempDir(), "candidate-cherri")
	if isWindowsRuntime() {
		path += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-trimpath", "-ldflags=-buildid=cherri-k03-controlled-build", "-o", path, ".")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual candidate build: %v\n%s", err, out)
	}
	return p15InstallRead(t, path)
}

func p15InstallServer(t *testing.T, body []byte) (string, *atomic.Int32) {
	t.Helper()
	count := new(atomic.Int32)
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cherri" {
			http.NotFound(w, r)
			return
		}
		count.Add(1)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(body)
	}))
	t.Cleanup(s.Close)
	return s.URL + "/cherri", count
}

func p15InstallEnvCopy(original map[string]string) map[string]string {
	result := make(map[string]string, len(original))
	for k, v := range original {
		result[k] = v
	}
	return result
}

func p15InstallShellQuote(value string) string {
	return "'" + strings.ReplaceAll(filepath.ToSlash(value), "'", "'\"'\"'") + "'"
}

type p15InstalledState struct {
	binary, commit, checksum, docs []byte
	hasChecksum                    bool
	docsSHA                        string
}

func p15InstallState(t *testing.T, f p15InstallFixture) p15InstalledState {
	t.Helper()
	p := filepath.Join(f.home, "bin", "cherri")
	s := p15InstalledState{
		binary: p15InstallRead(t, p), commit: p15InstallRead(t, p+".commit"),
		docs:    p15InstallRead(t, filepath.Join(f.home, "docs", "fork-language", "actions.md")),
		docsSHA: p15InstallGit(t, filepath.Join(f.home, "docs"), "rev-parse", "HEAD"),
	}
	var err error
	s.checksum, err = os.ReadFile(p + ".sha256")
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	s.hasChecksum = err == nil
	return s
}

func p15AssertInstallPreserved(t *testing.T, f p15InstallFixture, old p15InstalledState) {
	t.Helper()
	got := p15InstallState(t, f)
	for _, field := range []struct {
		name string
		a, b []byte
	}{
		{"compiler", old.binary, got.binary}, {"commit receipt", old.commit, got.commit},
		{"checksum receipt", old.checksum, got.checksum}, {"docs content", old.docs, got.docs},
	} {
		if string(field.a) != string(field.b) {
			t.Errorf("failed update changed %s", field.name)
		}
	}
	if old.hasChecksum != got.hasChecksum {
		t.Error("failed update changed checksum receipt presence")
	}
	if old.docsSHA != got.docsSHA {
		t.Errorf("failed update changed docs checkout: %s -> %s", old.docsSHA, got.docsSHA)
	}
	if out, err := p15InstallRun(t, f.script("doctor"), f.env); err != nil {
		t.Errorf("restored actual pair is unusable: %v\n%s", err, out)
	}
	p15AssertInstallCleaned(t, f)
}

func p15AssertInstallCleaned(t *testing.T, f p15InstallFixture) {
	t.Helper()
	for _, pattern := range []string{filepath.Join(f.home, "bin", ".cherri-install.*"), filepath.Join(f.home, ".cherri-docs.*")} {
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) != 0 {
			t.Errorf("owned staging files left after handled result: %v %v", matches, err)
		}
	}
	if _, err := os.Stat(filepath.Join(f.home, ".install-lock")); !os.IsNotExist(err) {
		t.Errorf("handled install retained lock: %v", err)
	}
}

func (f p15InstallFixture) advanceGoodDocs(t *testing.T) string {
	t.Helper()
	p15InstallWrite(t, filepath.Join(f.docsRepo, "release-note.txt"), []byte("Controlled compatible documentation update.\n"))
	p15InstallGit(t, f.docsRepo, "add", "release-note.txt")
	p15InstallGit(t, f.docsRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "-qm", "Compatible docs update")
	return p15InstallGit(t, f.docsRepo, "rev-parse", "HEAD")
}

func TestP15K03DownloadedChecksumBeforeExecution(t *testing.T) {
	for _, tc := range []struct {
		name, checksum, diagnostic string
		downloads                  int32
	}{
		{"missing", "", "CHERRI_PREBUILT_CHECKSUM_REQUIRED", 0},
		{"malformed", "not-a-sha256", "CHERRI_PREBUILT_CHECKSUM_REQUIRED", 0},
		{"mismatch", strings.Repeat("0", 64), "CHERRI_CHECKSUM_MISMATCH", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, roundTripBinary(t))
			old := p15InstallState(t, f)
			marker := filepath.Join(f.home, "must-not-execute")
			payload := []byte("#!/bin/sh\nprintf executed > " + p15InstallShellQuote(marker) + "\nexit 91\n")
			url, requests := p15InstallServer(t, payload)
			env := p15InstallEnvCopy(f.env)
			env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, tc.checksum
			out, err := p15InstallRun(t, f.script("setup"), env, "--update")
			if err == nil || !strings.Contains(out, tc.diagnostic) {
				t.Fatalf("wrong isolated checksum rejection: %v\n%s", err, out)
			}
			if requests.Load() != tc.downloads {
				t.Errorf("downloads=%d, want %d", requests.Load(), tc.downloads)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("unverified download was executed: %v", err)
			}
			p15AssertInstallPreserved(t, f, old)
			t.Logf("K03_CHECKSUM_CONTROL case=%s requests=%d unverified_execution=false", tc.name, requests.Load())
		})
	}
}

func TestP15K03PromotionFailuresRestoreActualPair(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	candidate := p15InstallCandidate(t, root)
	candidateHash := sha256.Sum256(candidate)
	for _, tc := range []struct{ name, pattern string }{
		{"commit_receipt_move", "*/.cherri-install.*/candidate.commit"},
		{"checksum_receipt_move", "*/.cherri-install.*/candidate.sha256"},
		{"docs_directory_move", "*/.cherri-docs.*/candidate"},
		{"final_docs_validation", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, roundTripBinary(t))
			old := p15InstallState(t, f)
			if sha256.Sum256(old.binary) == candidateHash {
				t.Fatal("rollback control needs distinct real binaries")
			}
			// Exercise rollback of an existing checksum receipt as well as binary/commit/docs.
			p15InstallWrite(t, filepath.Join(f.home, "bin", "cherri.sha256"), []byte(fmt.Sprintf("%x\n", sha256.Sum256(old.binary))))
			old = p15InstallState(t, f)
			newDocsSHA := f.advanceGoodDocs(t)
			url, requests := p15InstallServer(t, candidate)
			env := p15InstallEnvCopy(f.env)
			env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, fmt.Sprintf("%x", candidateHash)
			env["CHERRI_DOCS_REF"] = newDocsSHA
			shimDir := filepath.Join(t.TempDir(), "shim")
			marker := filepath.Join(f.home, "injected-failure")
			command := "mv"
			if tc.pattern == "" {
				command = "git"
			}
			find := exec.Command(p15SkillBash(t), "--noprofile", "--norc", "-c", "command -v "+command)
			actual, err := find.Output()
			if err != nil {
				t.Fatal(err)
			}
			observe := "sha256sum " + p15InstallShellQuote(filepath.Join(f.home, "bin", "cherri")) + " > " + p15InstallShellQuote(marker) + "\n"
			var shim string
			if tc.pattern != "" {
				shim = "#!/bin/sh\ncase \"$1\" in\n" + tc.pattern + ")\n" + observe + "echo '[P15_INJECTED_PROMOTION_FAILURE]' >&2\nexit 73\n;;\nesac\n"
			} else {
				shim = "#!/bin/sh\nif [ \"$1\" = '-C' ] && [ \"$2\" = " + p15InstallShellQuote(filepath.Join(f.home, "docs")) + " ] && [ \"$3\" = 'rev-parse' ]; then\n" + observe + "exit 74\nfi\n"
			}
			shim += "exec " + p15InstallShellQuote(strings.TrimSpace(string(actual))) + " \"$@\"\n"
			p15InstallWrite(t, filepath.Join(shimDir, command), []byte(shim))
			env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = filepath.ToSlash(shimDir), command
			out, runErr := p15InstallRun(t, f.script("setup"), env, "--update")
			diagnostic := "P15_INJECTED_PROMOTION_FAILURE"
			if tc.pattern == "" {
				diagnostic = "CHERRI_DOCS_GIT_INVALID"
			}
			if runErr == nil || !strings.Contains(out, diagnostic) {
				t.Fatalf("wrong controlled replacement failure: %v\n%s", runErr, out)
			}
			observation := p15InstallRead(t, marker)
			if !strings.HasPrefix(string(observation), fmt.Sprintf("%x", candidateHash)) {
				t.Fatalf("fault did not occur after actual compiler promotion: %s", observation)
			}
			if requests.Load() != 1 {
				t.Errorf("candidate downloaded %d times", requests.Load())
			}
			p15AssertInstallPreserved(t, f, old)
			t.Logf("K03_ROLLBACK_CONTROL case=%s candidate_was_active=true prior_pair_restored=true", tc.name)
		})
	}
}

func TestP15K03SuccessfulUpdateCommitsValidatedPair(t *testing.T) {
	f := p15NewInstallFixture(t)
	f.seedActualPair(t, roundTripBinary(t))
	old := p15InstallState(t, f)
	candidate := p15InstallCandidate(t, f.root)
	wantHash := sha256.Sum256(candidate)
	if sha256.Sum256(old.binary) == wantHash {
		t.Fatal("success control needs distinct real compiler bytes")
	}
	url, requests := p15InstallServer(t, candidate)
	newDocsSHA := f.advanceGoodDocs(t)
	env := p15InstallEnvCopy(f.env)
	env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, fmt.Sprintf("%x", wantHash)
	env["CHERRI_DOCS_REF"] = newDocsSHA
	out, err := p15InstallRun(t, f.script("setup"), env, "--update")
	if err != nil {
		t.Fatalf("compatible actual update failed: %v\n%s", err, out)
	}
	got := p15InstallState(t, f)
	if sha256.Sum256(got.binary) != wantHash {
		t.Error("successful update did not install the candidate compiler")
	}
	if string(got.commit) != f.sourceSHA+"\n" {
		t.Errorf("wrong installed source receipt: %s", got.commit)
	}
	if !got.hasChecksum || strings.TrimSpace(string(got.checksum)) != fmt.Sprintf("%x", wantHash) {
		t.Error("successful update has no matching byte receipt")
	}
	if got.docsSHA != newDocsSHA {
		t.Errorf("successful update installed wrong docs: %s", got.docsSHA)
	}
	if requests.Load() != 1 {
		t.Errorf("success downloaded %d candidates", requests.Load())
	}
	delete(env, "CHERRI_PREBUILT_URL")
	delete(env, "CHERRI_PREBUILT_SHA256")
	if out, err := p15InstallRun(t, f.script("doctor"), env); err != nil {
		t.Errorf("updated actual pair fails doctor: %v\n%s", err, out)
	}
	p15AssertInstallCleaned(t, f)
	t.Logf("K03_SUCCESS_CONTROL source=%s binary=%x docs=%s requests=%d", f.sourceSHA, wantHash, newDocsSHA, requests.Load())
}

func TestP15SkillFreshPinnedInstall(t *testing.T) {
	f := p15NewInstallFixture(t)
	compilerRepo := filepath.Join(t.TempDir(), "compiler")
	p15InstallGit(t, f.root, "clone", "--quiet", "--shared", f.root, compilerRepo)
	p15InstallWrite(t, filepath.Join(compilerRepo, "p15-unselected-source.txt"), []byte("Later branch commit must not be installed.\n"))
	p15InstallGit(t, compilerRepo, "add", "p15-unselected-source.txt")
	p15InstallGit(t, compilerRepo, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "-qm", "Advance source branch after pinned commit")
	advancedSource := p15InstallGit(t, compilerRepo, "rev-parse", "HEAD")
	advancedDocs := f.advanceGoodDocs(t)
	if advancedSource == f.sourceSHA || advancedDocs == f.docsSHA {
		t.Fatal("pin control did not advance both branches")
	}
	f.env["CHERRI_REPO"] = filepath.ToSlash(compilerRepo)
	out, err := p15InstallRun(t, f.script("setup"), f.env)
	if err != nil {
		t.Fatalf("actual source-build fresh install: %v\n%s", err, out)
	}
	got := p15InstallState(t, f)
	if got.docsSHA != f.docsSHA || strings.TrimSpace(string(got.commit)) != f.sourceSHA {
		t.Fatalf("fresh install did not honor exact source/docs pins: docs=%s commit=%s", got.docsSHA, got.commit)
	}
	if p15InstallGit(t, filepath.Join(f.home, "src"), "rev-parse", "HEAD") != f.sourceSHA {
		t.Error("source checkout followed advanced branch")
	}
	if _, err := os.Stat(filepath.Join(f.home, "src", "p15-unselected-source.txt")); !os.IsNotExist(err) {
		t.Error("advanced source content leaked into pinned checkout")
	}
	if !got.hasChecksum || strings.TrimSpace(string(got.checksum)) != fmt.Sprintf("%x", sha256.Sum256(got.binary)) {
		t.Error("actual source build lacks matching byte receipt")
	}
	info, err := buildinfo.ReadFile(filepath.Join(f.home, "bin", "cherri"))
	if err != nil {
		t.Fatal(err)
	}
	settings := make(map[string]string)
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs.revision"] != f.sourceSHA || settings["vcs.modified"] != "false" {
		t.Fatalf("actual compiler build provenance differs from clean pinned checkout: %v", settings)
	}
	if doctor, err := p15InstallRun(t, f.script("doctor"), f.env); err != nil {
		t.Fatalf("actual fresh install doctor: %v\n%s", err, doctor)
	}
	p15AssertInstallCleaned(t, f)
	t.Logf("K02_K03_FRESH_SOURCE source=%s branch=%s docs=%s docs_branch=%s vcs_clean=true binary=%x", f.sourceSHA, advancedSource, f.docsSHA, advancedDocs, sha256.Sum256(got.binary))
}

func TestP15K02ActualCacheAndBundleSelection(t *testing.T) {
	actual := p15InstallRead(t, roundTripBinary(t))
	for _, name := range []string{"cache_preferred_without_executing_bad_bundle", "bad_cache_uses_verified_bundle", "tampered_cache_not_executed", "both_incompatible_fail", "explicit_incompatible_never_falls_back"} {
		t.Run(name, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, roundTripBinary(t))
			cache := filepath.Join(f.home, "bin", "cherri")
			bundle := filepath.Join(f.skill, "bin", "cherri")
			hash := fmt.Sprintf("%x\n", sha256.Sum256(actual))
			p15InstallWrite(t, cache+".sha256", []byte(hash))
			p15InstallWrite(t, bundle, actual)
			p15InstallWrite(t, bundle+".commit", []byte(f.sourceSHA+"\n"))
			p15InstallWrite(t, bundle+".sha256", []byte(hash))
			env := p15InstallEnvCopy(f.env)
			selected := cache
			wantFailure := ""
			marker := filepath.Join(f.home, "must-not-execute")
			payload := []byte("#!/bin/sh\nprintf executed > " + p15InstallShellQuote(marker) + "\nexit 92\n")
			switch name {
			case "cache_preferred_without_executing_bad_bundle":
				p15InstallWrite(t, bundle, payload)
				p15InstallWrite(t, bundle+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(payload))))
			case "bad_cache_uses_verified_bundle":
				p15InstallWrite(t, cache+".commit", []byte(strings.Repeat("0", 40)+"\n"))
				selected = bundle
			case "tampered_cache_not_executed":
				p15InstallWrite(t, cache, payload)
				selected = bundle
			case "both_incompatible_fail":
				p15InstallWrite(t, cache+".commit", []byte(strings.Repeat("0", 40)+"\n"))
				p15InstallWrite(t, bundle+".commit", []byte(strings.Repeat("1", 40)+"\n"))
				wantFailure = "no compatible cached or bundled binary available"
				shim := filepath.Join(t.TempDir(), "path")
				p15InstallWrite(t, filepath.Join(shim, "cherri"), []byte("#!/bin/sh\nexit 93\n"))
				env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = filepath.ToSlash(shim), "cherri"
			case "explicit_incompatible_never_falls_back":
				explicit := filepath.Join(t.TempDir(), "explicit-cherri")
				p15InstallWrite(t, explicit, actual)
				p15InstallWrite(t, explicit+".commit", []byte(strings.Repeat("2", 40)+"\n"))
				p15InstallWrite(t, explicit+".sha256", []byte(hash))
				env["CHERRI_BIN"] = filepath.ToSlash(explicit)
				wantFailure = "Explicit CHERRI_BIN failed compatibility validation"
			}
			beforeCache := p15InstallRead(t, cache)
			beforeBundle := p15InstallRead(t, bundle)
			out, runErr := p15InstallRun(t, f.script("setup"), env, "--offline")
			doctor, doctorErr := p15InstallRun(t, f.script("doctor"), env)
			if wantFailure != "" {
				if runErr == nil || !strings.Contains(out, wantFailure) {
					t.Fatalf("wrong isolated compatibility failure: %v\n%s", runErr, out)
				}
				if doctorErr == nil || !strings.Contains(doctor, "MISSING or INCOMPATIBLE") {
					t.Fatalf("doctor silently fell back: %v\n%s", doctorErr, doctor)
				}
			} else {
				if runErr != nil || doctorErr != nil {
					t.Fatalf("verified pair failed: setup=%v doctor=%v\n%s\n%s", runErr, doctorErr, out, doctor)
				}
				p15AssertDoctorBinary(t, doctor, selected)
			}
			if string(p15InstallRead(t, cache)) != string(beforeCache) || string(p15InstallRead(t, bundle)) != string(beforeBundle) {
				t.Error("offline selection changed installed compiler bytes")
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("ineligible cached/bundled payload executed: %v", err)
			}
			if p15InstallGit(t, filepath.Join(f.home, "docs"), "rev-parse", "HEAD") != f.docsSHA {
				t.Error("offline selection moved docs")
			}
			p15AssertInstallCleaned(t, f)
			t.Logf("K02_SELECTION_CONTROL case=%s exact_selection_or_rejection=true unverified_execution=false", name)
		})
	}
}

func p15AssertDoctorBinary(t *testing.T, output, expected string) {
	t.Helper()
	chosen := ""
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(line, "Cherri: ") {
			if chosen != "" {
				t.Fatal("doctor emitted more than one compiler selection")
			}
			chosen = strings.TrimSpace(strings.TrimPrefix(line, "Cherri: "))
		}
	}
	if chosen == "" {
		t.Fatalf("doctor omitted selected compiler:\n%s", output)
	}
	if isWindowsRuntime() {
		cmd := exec.Command(p15SkillBash(t), "--noprofile", "--norc", "-c", "cygpath -w \"$1\"", "p15-path", chosen)
		converted, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		chosen = strings.TrimSpace(string(converted))
	}
	got, err := os.Stat(chosen)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.Stat(expected)
	if err != nil {
		t.Fatal(err)
	}
	if !os.SameFile(got, want) {
		t.Fatalf("doctor selected %s, expected the actual file %s", chosen, expected)
	}
}

func TestP15K03ImmutableInstallRefsRequired(t *testing.T) {
	for _, key := range []string{"CHERRI_REF", "CHERRI_DOCS_REF"} {
		t.Run(key, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, roundTripBinary(t))
			old := p15InstallState(t, f)
			url, requests := p15InstallServer(t, p15InstallRead(t, roundTripBinary(t)))
			env := p15InstallEnvCopy(f.env)
			env[key] = "main"
			env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, strings.Repeat("0", 64)
			out, err := p15InstallRun(t, f.script("setup"), env, "--update")
			if err == nil || !strings.Contains(out, "CHERRI_INSTALL_IMMUTABLE_REFS_REQUIRED") {
				t.Fatalf("mutable pin accepted or wrong failure: %v\n%s", err, out)
			}
			if requests.Load() != 0 {
				t.Fatal("mutable pin triggered a download")
			}
			p15AssertInstallPreserved(t, f, old)
		})
	}
}

func TestP15K03LocalStateIsPreserved(t *testing.T) {
	for _, name := range []string{"dirty_docs", "invalid_docs_git", "dirty_source"} {
		t.Run(name, func(t *testing.T) {
			f := p15NewInstallFixture(t)
			f.seedActualPair(t, roundTripBinary(t))
			old := p15InstallState(t, f)
			docs := filepath.Join(f.home, "docs")
			userPath := filepath.Join(docs, "fork-language", "actions.md")
			userBytes := old.docs
			wantDiagnostic := "CHERRI_DOCS_DIRTY"
			env := p15InstallEnvCopy(f.env)
			switch name {
			case "dirty_docs":
				userBytes = append(append([]byte{}, old.docs...), []byte("\nLocal user note must be preserved.\n")...)
				p15InstallWrite(t, userPath, userBytes)
			case "invalid_docs_git":
				if err := os.Rename(filepath.Join(docs, ".git"), filepath.Join(docs, ".git.saved")); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(filepath.Join(docs, ".git"), 0755); err != nil {
					t.Fatal(err)
				}
				wantDiagnostic = "CHERRI_DOCS_GIT_INVALID"
			case "dirty_source":
				source := filepath.Join(f.home, "src")
				p15InstallGit(t, f.root, "clone", "--quiet", "--shared", f.root, source)
				userPath = filepath.Join(source, "p15-local-source.txt")
				userBytes = []byte("Uncommitted source work must be preserved.\n")
				p15InstallWrite(t, userPath, userBytes)
				wantDiagnostic = "CHERRI_SOURCE_DIRTY"
			}
			if name != "dirty_source" {
				actual := p15InstallRead(t, roundTripBinary(t))
				url, _ := p15InstallServer(t, actual)
				env["CHERRI_PREBUILT_URL"], env["CHERRI_PREBUILT_SHA256"] = url, fmt.Sprintf("%x", sha256.Sum256(actual))
			}
			out, err := p15InstallRun(t, f.script("setup"), env, "--update")
			if err == nil || !strings.Contains(out, wantDiagnostic) {
				t.Fatalf("local state overwritten or wrong failure: %v\n%s", err, out)
			}
			if string(p15InstallRead(t, userPath)) != string(userBytes) {
				t.Error("installer changed local user content")
			}
			cache := filepath.Join(f.home, "bin", "cherri")
			if string(p15InstallRead(t, cache)) != string(old.binary) || string(p15InstallRead(t, cache+".commit")) != string(old.commit) {
				t.Error("rejected local-state update changed active compiler")
			}
			if name == "invalid_docs_git" {
				if _, err := os.Stat(filepath.Join(docs, ".git.saved", "HEAD")); err != nil {
					t.Error("invalid Git metadata rejection lost saved repository")
				}
			} else {
				if p15InstallGit(t, docs, "rev-parse", "HEAD") != old.docsSHA {
					t.Error("rejected local-state update moved docs")
				}
			}
			p15AssertInstallCleaned(t, f)
			t.Logf("K03_LOCAL_STATE_CONTROL case=%s precise_rejection=true user_bytes_preserved=true", name)
		})
	}
}
