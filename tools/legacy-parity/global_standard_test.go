package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func TestGate_PublicGlobalsAndStandard(t *testing.T) {
	var bin, err = ensureCompilerBinary()
	if err != nil {
		t.Fatal(err)
	}
	var root = findRepoRoot()
	for _, name := range []string{"globals.cherri", "standard.cherri"} {
		t.Run(name, func(t *testing.T) {
			var dir = t.TempDir()
			var result = testFixture(bin, filepath.Join(root, "tests", name), dir, root)
			if !result.Equal || (result.Status != "VERIFIED_PARITY" && result.Status != "PROVEN_EQUIVALENT") {
				t.Fatalf("original public fixture rejected: %s %s %+v", result.Status, result.Detail, result.Differences)
			}
			if name != "standard.cherri" {
				return
			}
			var base = strings.TrimSuffix(name, ".cherri")
			var legacy, err = shortcutcompare.LoadShortcutDocument(filepath.Join(dir, base+"_legacy.shortcut"))
			if err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"body_value", "wrong_module_call", "unknown_function_call", "parameter_edge"} {
				t.Run(mutation, func(t *testing.T) {
					var candidate, err = shortcutcompare.LoadShortcutDocument(filepath.Join(dir, base+"_v2.shortcut"))
					if err != nil {
						t.Fatal(err)
					}
					var changed = false
					var visit func(any)
					visit = func(value any) {
						switch node := value.(type) {
						case map[string]any:
							if changed {
								return
							}
							if mutation == "body_value" && node["WFTextActionText"] == "Hello, World" {
								node["WFTextActionText"] = "Changed body"
								changed = true
								return
							}
							if mutation == "parameter_edge" && node["Type"] == "ActionOutput" && strings.Contains(strings.ToLower(strings.TrimSpace(toText(node["OutputName"]))), "script") {
								node["OutputUUID"] = "wrong-parameter-identity"
								changed = true
								return
							}
							if text, ok := node["string"].(string); ok && text == "module:builtin:stdfunc::runJS" {
								if mutation == "wrong_module_call" {
									node["string"] = "module:file:other::runJS"
									changed = true
									return
								}
								if mutation == "unknown_function_call" {
									node["string"] = "module:builtin:stdfunc::missing"
									changed = true
									return
								}
							}
							for _, child := range node {
								visit(child)
							}
						case []any:
							for _, child := range node {
								visit(child)
							}
						}
					}
					visit(candidate)
					if !changed {
						t.Fatalf("mutation target %s not found in actual fixture output", mutation)
					}
					if result := shortcutcompare.CompareDocumentsNormalized(legacy, candidate); result.Equal {
						t.Fatalf("real fixture corruption %s escaped", mutation)
					}
				})
			}
		})
	}
}
func toText(value any) string { var text, _ = value.(string); return text }
