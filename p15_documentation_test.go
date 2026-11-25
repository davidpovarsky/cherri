package main

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestP15D01CanonicalDocsProduction(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	names := []string{"TestP15D01PrimaryParameterContract", "TestP15D01AllGeneratedSignaturesAndCategories", "TestP15D01ScopedMacroDialectAndBodyContract", "TestP15D01CheckGateProducesAndRejectsFaults"}
	cmd := exec.CommandContext(ctx, "go", "test", "./tools/language-docs", "-count=1", "-json", "-run", "^("+strings.Join(names, "|")+")$")
	raw, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("actual generator/CLI tests failed: %v\n%s", err, raw)
	}
	passed := map[string]bool{}
	for _, line := range bytes.Split(raw, []byte("\n")) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct{ Action, Test string }
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatalf("invalid actual test event: %v\n%s", err, line)
		}
		if event.Action == "fail" || event.Action == "skip" {
			t.Fatalf("generator case did not pass: %s", line)
		}
		if event.Action == "pass" && event.Test != "" {
			passed[event.Test] = true
		}
	}
	for _, name := range names {
		if !passed[name] {
			t.Fatalf("actual generator did not execute %s", name)
		}
	}
	t.Log("D01: all505 native definitions,16 separate macros including cache, complete categories, typed defaults/primary optionality, full canonical artifact, reviewed scoped-body dialects and10 real CLI drift rejections")
}
func p15DocsSearchFixture(t *testing.T, active bool) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"language/guide.md":   "scope-marker LEGACY_RETIRED @oldVariable\n",
		"compiler/parsing.md": "scope-marker UPSTREAM_COMPILER_REFERENCE\n",
		"README.md":           "scope-marker REPOSITORY_INTERNAL\n",
	}
	if active {
		files["fork-language/guide.md"] = "scope-marker V2_CURRENT let value = text(\"current\")\n"
	}
	for name, text := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
func p15RunDocsSearch(t *testing.T, docs string, args ...string) (string, error) {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	script := filepath.ToSlash(filepath.Join(root, "skills/cherri-shortcuts/scripts/search-docs.sh"))
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, p15SkillBash(t), append([]string{"--noprofile", "--norc", script}, args...)...)
	cmd.Dir = root
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if !strings.HasPrefix(key, "CHERRI_") && key != "SKILL_DIR" {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "CHERRI_DOCS_DIR="+filepath.ToSlash(docs))
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		t.Fatal("docs search timeout is not a negative assertion")
	}
	return string(out), err
}
func TestP15D03SearchScope(t *testing.T) {
	docs := p15DocsSearchFixture(t, true)
	out, err := p15RunDocsSearch(t, docs, "scope-marker")
	if err != nil || !strings.Contains(out, "V2_CURRENT") {
		t.Fatalf("active docs not found: %v\n%s", err, out)
	}
	for _, retired := range []string{"LEGACY_RETIRED", "UPSTREAM_COMPILER_REFERENCE", "REPOSITORY_INTERNAL"} {
		if strings.Contains(out, retired) {
			t.Errorf("default authoring search recommended %s:\n%s", retired, out)
		}
	}
}
func TestP15D03ExplicitLegacyReference(t *testing.T) {
	out, err := p15RunDocsSearch(t, p15DocsSearchFixture(t, true), "--legacy-reference", "scope-marker")
	if err != nil || !strings.Contains(out, "LEGACY_RETIRED") || strings.Contains(out, "V2_CURRENT") {
		t.Fatalf("explicit historical scope wrong: %v\n%s", err, out)
	}
}
func TestP15D03MissingActiveSubtreeFailsClosed(t *testing.T) {
	out, err := p15RunDocsSearch(t, p15DocsSearchFixture(t, false), "scope-marker")
	if err == nil || !strings.Contains(out, "Active V2 documentation directory is missing") || strings.Contains(out, "LEGACY_RETIRED") {
		t.Fatalf("missing active docs fell back to old recommendations: %v\n%s", err, out)
	}
}
func TestP15D03SearchExecutionErrorsRemainFailures(t *testing.T) {
	out, err := p15RunDocsSearch(t, p15DocsSearchFixture(t, true), "[")
	if err == nil {
		t.Fatalf("invalid grep expression was silently successful: %s", out)
	}
}
func TestP15D03NoMatchAndEmptyQuery(t *testing.T) {
	docs := p15DocsSearchFixture(t, true)
	out, err := p15RunDocsSearch(t, docs, "never-present-synthetic-marker")
	if err != nil || strings.TrimSpace(out) != "" {
		t.Fatalf("healthy no-match: %v\n%s", err, out)
	}
	out, err = p15RunDocsSearch(t, docs)
	if err != nil || !strings.HasSuffix(strings.TrimSpace(out), "/fork-language") {
		t.Fatalf("empty query did not resolve active subtree: %v\n%s", err, out)
	}
}
