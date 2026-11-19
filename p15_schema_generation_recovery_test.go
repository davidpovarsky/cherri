package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// These scenarios use a detached Git worktree with the actual committed source.
// No baseline catalog, generated output or fixture in the original checkout is edited.
func p15SchemaAcceptanceWorktree(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	head := p15SchemaRun(t, root, "git", "rev-parse", "HEAD")
	before := p15SchemaRun(t, root, "git", "status", "--porcelain", "--untracked-files=normal")
	replica := filepath.Join(t.TempDir(), "worktree")
	p15SchemaRun(t, root, "git", "worktree", "add", "--detach", replica, strings.TrimSpace(string(head)))
	t.Cleanup(func() {
		out, err := p15SchemaCommand(t, root, nil, "git", "worktree", "remove", "--force", replica)
		if err != nil {
			t.Errorf("remove owned disposable worktree: %v %s", err, out)
		}
		after := p15SchemaRun(t, root, "git", "status", "--porcelain", "--untracked-files=normal")
		if !bytes.Equal(before, after) || !bytes.Equal(head, p15SchemaRun(t, root, "git", "rev-parse", "HEAD")) {
			t.Error("schema acceptance changed the original source worktree")
		}
	})
	return replica
}

func p15SchemaAction(t *testing.T, catalog map[string]any, name string) map[string]any {
	t.Helper()
	for _, raw := range catalog["actions"].([]any) {
		a := raw.(map[string]any)
		if a["name"] == name {
			return a
		}
	}
	t.Fatalf("canonical action %q missing", name)
	return nil
}

func p15SchemaGenerationReject(t *testing.T, repo, generator, input, output string, env []string, expected ...string) {
	t.Helper()
	original, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	argv := []string{generator}
	if input != "" {
		argv = append(argv, input, output)
	}
	out, err := p15SchemaCommand(t, repo, env, argv...)
	if err == nil {
		t.Fatalf("invalid production generation succeeded: %s", out)
	}
	for _, fragment := range expected {
		if !bytes.Contains(out, []byte(fragment)) {
			t.Fatalf("wrong specific generation error; missing %q in %s", fragment, out)
		}
	}
	actual, readErr := os.ReadFile(output)
	if readErr != nil || !bytes.Equal(original, actual) {
		t.Fatal("failed generation replaced the healthy generated registry")
	}
}

// RP07 / BRG22: every mutation is applied to the real exported catalog or the
// real codec binding, then exercised through a built production generator CLI.
func TestP15SchemaGenerationFailureControls(t *testing.T) {
	repo := p15SchemaAcceptanceWorktree(t)
	compiler, generator := filepath.Join(t.TempDir(), "compiler.exe"), filepath.Join(t.TempDir(), "generator.exe")
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", compiler, ".")
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", generator, "./tools/language-schema")
	catalogBytes := p15SchemaRun(t, repo, compiler, "--actions-json")
	if !bytes.Equal(catalogBytes, p15SchemaRun(t, repo, compiler, "--actions-json")) {
		t.Fatal("live export is nondeterministic")
	}
	input := filepath.Join(t.TempDir(), "catalog.json")
	output := filepath.Join(t.TempDir(), "generated.go")
	second := filepath.Join(t.TempDir(), "second.go")
	p15SchemaWrite(t, input, catalogBytes)
	p15SchemaRun(t, repo, generator, input, output)
	p15SchemaRun(t, repo, generator, input, second)
	healthy, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	repeated, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	registry := filepath.Join(repo, "internal/language/schema/generated_registry.go")
	checked, err := os.ReadFile(registry)
	if err != nil || !bytes.Equal(healthy, repeated) || !bytes.Equal(healthy, checked) {
		t.Fatal("two real generations differ from each other or the actual checked-in registry")
	}
	t.Run("unchanged_live_generation", func(t *testing.T) {
		p15SchemaRun(t, repo, generator, "--verify-generated")
	})
	for _, tc := range []struct {
		name     string
		mutate   func(*testing.T, map[string]any)
		expected []string
	}{
		{"duplicate_label", func(t *testing.T, c map[string]any) {
			a := p15SchemaAction(t, c, "quicklook")
			p := a["parameters"].([]any)
			p[1].(map[string]any)["name"] = p[0].(map[string]any)["name"]
		}, []string{"Catalog validation failed:", "duplicate parameter label", "quicklook"}},
		{"duplicate_action", func(t *testing.T, c map[string]any) {
			c["actions"] = append(c["actions"].([]any), p15SchemaAction(t, c, "quicklook"))
		}, []string{"Catalog validation failed:", "duplicate action", "quicklook"}},
		{"duplicate_enum_value", func(t *testing.T, c map[string]any) {
			for _, raw := range c["actions"].([]any) {
				a := raw.(map[string]any)
				params, _ := a["parameters"].([]any)
				for _, rawP := range params {
					p := rawP.(map[string]any)
					values, _ := p["enumValues"].([]any)
					if len(values) > 0 {
						p["enumValues"] = append(values, values[0])
						return
					}
				}
			}
			t.Fatal("live catalog has no enum values")
		}, []string{"Catalog validation failed:", "duplicate enum value"}},
		{"conflicting_enum", func(t *testing.T, c map[string]any) {
			seen := map[string]bool{}
			for _, raw := range c["actions"].([]any) {
				a := raw.(map[string]any)
				params, _ := a["parameters"].([]any)
				for _, rawP := range params {
					p := rawP.(map[string]any)
					name, _ := p["enum"].(string)
					values, _ := p["enumValues"].([]any)
					if name != "" && len(values) > 0 {
						if seen[name] {
							p["enumValues"] = append(values, "P15_CONFLICTING_ENUM_VALUE")
							return
						}
						seen[name] = true
					}
				}
			}
			t.Fatal("live catalog has no repeated enum definition")
		}, []string{"Catalog validation failed:", "conflicting enum definition"}},
		{"stale_facet", func(t *testing.T, c map[string]any) {
			a := p15SchemaAction(t, c, "adjustDate")
			for _, raw := range a["parameters"].([]any) {
				p := raw.(map[string]any)
				if p["name"] == "date" {
					p["name"] = "p15ChangedDate"
					return
				}
			}
			t.Fatal("actual facet primary parameter missing")
		}, []string{"Facet validation failed:", "adjustDate", "unknown primary parameter \"date\""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			catalog := p15SchemaJSON(t, catalogBytes)
			tc.mutate(t, catalog)
			changed, err := json.Marshal(catalog)
			if err != nil {
				t.Fatal(err)
			}
			p15SchemaWrite(t, input, changed)
			p15SchemaGenerationReject(t, repo, generator, input, output, nil, tc.expected...)
			p15SchemaWrite(t, input, catalogBytes)
			p15SchemaRun(t, repo, generator, input, output, "--verify-generated")
		})
	}
	t.Run("live_export_failure", func(t *testing.T) {
		p15SchemaGenerationReject(t, repo, generator, "", registry, []string{"CHERRI_LIVE_CATALOG_CMD=go version-nonexistent-flag"}, "Error extracting live catalog", "fail-closed, no baseline fallback")
	})
	t.Run("malformed_live_export", func(t *testing.T) {
		p15SchemaGenerationReject(t, repo, generator, "", registry, []string{"CHERRI_LIVE_CATALOG_CMD=go version"}, "Error parsing catalog JSON:")
	})
	t.Run("unknown_codec_binding", func(t *testing.T) {
		path := filepath.Join(repo, "tools/language-schema/main.go")
		original, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		target := "return backend.CodecText\n"
		if strings.Count(string(original), target) != 1 {
			t.Fatal("actual text codec binding changed")
		}
		p15SchemaWrite(t, path, []byte(strings.Replace(string(original), target, "return \"p15-unregistered-codec\"\n", 1)))
		badGenerator := filepath.Join(t.TempDir(), "generator-bad-codec.exe")
		p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", badGenerator, "./tools/language-schema")
		p15SchemaGenerationReject(t, repo, badGenerator, input, output, nil, "Catalog validation failed:", "unregistered codec \"p15-unregistered-codec\"")
		p15SchemaWrite(t, path, original)
	})
	p15SchemaRun(t, repo, generator, "--verify-generated")
	t.Log("Actual current definitions, repeated generation, specific export/facet/codec/label/enum failures and unchanged healthy generated bytes verified; no native/runtime evidence fabricated.")
}
