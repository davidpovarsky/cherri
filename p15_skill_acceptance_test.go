package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

var p15K04Cases = []string{
	"offline_cache_healthy", "stale_schema_rejected", "restore_after_schema",
	"stale_docs_commit_rejected", "restore_after_commit",
	"incompatible_cache_rejected", "restore_cache", "missing_docs_rejected",
	"no_network_observed",
}

func p15SkillBash(t *testing.T) string {
	t.Helper()
	if isWindowsRuntime() {
		if p := filepath.Join(os.Getenv("ProgramFiles"), "Git", "bin", "bash.exe"); p != "" {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	p, err := exec.LookPath("bash")
	if err != nil {
		t.Fatalf("K04 production execution requires bash: %v", err)
	}
	return p
}

func p15RunSkillIsolation(t *testing.T, root, binary string) (string, error) {
	t.Helper()
	rev, err := exec.Command("git", "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	env := make([]string, 0, len(os.Environ())+2)
	for _, value := range os.Environ() {
		key := strings.SplitN(value, "=", 2)[0]
		if strings.HasPrefix(key, "CHERRI_") || key == "SKILL_DIR" {
			continue
		}
		env = append(env, value)
	}
	env = append(env, "CHERRI_BIN="+filepath.ToSlash(binary), "CHERRI_REF="+strings.TrimSpace(string(rev)))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	script := filepath.Join(root, "skills", "cherri-shortcuts", "scripts", "test-compatibility-isolation.sh")
	cmd := exec.CommandContext(ctx, p15SkillBash(t), "--noprofile", "--norc", filepath.ToSlash(script))
	cmd.Dir, cmd.Env = root, env
	output, err := cmd.CombinedOutput()
	return string(output), err
}

func p15AssertSkillCases(t *testing.T, output string) {
	t.Helper()
	for _, name := range p15K04Cases {
		if count := strings.Count(output, "CHERRI_K04_CASE:"+name+"\n"); count != 1 {
			t.Fatalf("expected exactly one executed %s case, got %d:\n%s", name, count, output)
		}
	}
	if !strings.Contains(output, "PASS: 18 compatibility isolation checks using real compiler and canonical docs") {
		t.Fatalf("complete production script did not finish:\n%s", output)
	}
}

func TestP15K04OfflineCacheProduction(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	output, err := p15RunSkillIsolation(t, root, roundTripBinary(t))
	if err != nil {
		t.Fatalf("real compiler offline compatibility failed: %v\n%s", err, output)
	}
	p15AssertSkillCases(t, output)
}

func p15CopySkillOracle(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, relative := range []string{
		"skills/cherri-shortcuts/compatibility-manifest.json",
		"skills/cherri-shortcuts/scripts/common.sh",
		"skills/cherri-shortcuts/scripts/doctor.sh",
		"skills/cherri-shortcuts/scripts/setup.sh",
		"skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh",
		"docs/language-v2/actions-reference.md",
	} {
		data, err := os.ReadFile(relative)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(root, filepath.FromSlash(relative))
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestP15K04OfflineOracleRejectsBypasses(t *testing.T) {
	binary := roundTripBinary(t)
	root := p15CopySkillOracle(t)
	healthy, err := p15RunSkillIsolation(t, root, binary)
	if err != nil {
		t.Fatalf("isolated healthy control failed: %v\n%s", err, healthy)
	}
	p15AssertSkillCases(t, healthy)
	for _, mutation := range []struct {
		name, file, old, replacement, diagnostic string
	}{
		{"docs_schema_bypass", "common.sh",
			"if [ \"$_docs_fp\" != \"$MANIFEST_SCHEMA_FP\" ]; then",
			"if false; then",
			"FAIL: expected nonzero status and CHERRI_STALE_DOCS_SCHEMA"},
		{"missing_docs_bypass", "setup.sh",
			"    validate_docs_dir \"$CHERRI_DOCS_DIR\"\n",
			"    : # deliberately bypass missing docs validation\n",
			"FAIL: expected offline failure: missing_docs_rejected"},
		{"unexpected_offline_network", "setup.sh",
			"mkdir -p \"$CHERRI_HOME/bin\"",
			"git ls-remote origin >/dev/null 2>&1 || true\nmkdir -p \"$CHERRI_HOME/bin\"",
			"FAIL: offline mode attempted network access"},
	} {
		t.Run(mutation.name, func(t *testing.T) {
			path := filepath.Join(root, "skills/cherri-shortcuts/scripts", mutation.file)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(original), mutation.old) {
				t.Fatalf("causal mutation anchor missing: %s", mutation.name)
			}
			changed := strings.Replace(string(original), mutation.old, mutation.replacement, 1)
			if err := os.WriteFile(path, []byte(changed), 0755); err != nil {
				t.Fatal(err)
			}
			output, runErr := p15RunSkillIsolation(t, root, binary)
			if err := os.WriteFile(path, original, 0755); err != nil {
				t.Fatal(err)
			}
			if runErr == nil || !strings.Contains(output, mutation.diagnostic) {
				t.Fatalf("wrong failure for %s: err=%v; expected %q\n%s", mutation.name, runErr, mutation.diagnostic, output)
			}
		})
	}
}
