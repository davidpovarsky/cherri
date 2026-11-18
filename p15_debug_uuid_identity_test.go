package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// Shortcuts serializes ActionOutput bindings as strings. The macro must keep the
// exact identifier allocated by its frontend; changing case leaves a dangling
// binding in Legacy even though the underlying UUID bytes would be equivalent.
func TestP15DebugMacroPreservesFrontendResultIdentity(t *testing.T) {
	id := "a12bC345-678d-4e90-A123-456789abcDEF"
	for _, macro := range []struct {
		name   string
		params map[string]any
	}{
		{"debugLog", map[string]any{"WFInput": debugOutputRefNamed("source", "Text")}},
		{"debugTraceEvent", map[string]any{"WFDebugTraceStage": "before", "WFDebugTraceLabel": "qa"}},
		{"debugExportTrace", map[string]any{"WFDebugTraceRunID": "qa-uuid"}},
	} {
		t.Run(macro.name, func(t *testing.T) {
			acts, err := buildDebugStdlibActions(macro.name, macro.params, id, "result")
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, act := range acts {
				if act.WFWorkflowActionParameters["CustomOutputName"] == "result" {
					found = true
					if got := act.WFWorkflowActionParameters["UUID"]; got != id {
						t.Fatalf("frontend identity changed: got %v, want %s", got, id)
					}
				}
			}
			if !found {
				t.Fatal("macro output producer missing")
			}
		})
	}
}

func TestP15SignedFixtureExactProducerReferences(t *testing.T) {
	for _, dialect := range []string{"legacy", "v2"} {
		t.Run(dialect, func(t *testing.T) {
			path := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", "qa-functional-"+dialect+".cherri")
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			acts := compileDebugFixture(t, string(src), dialect == "legacy")
			producers := map[string]int{}
			refs := 0
			var visit func(any, int)
			visit = func(value any, index int) {
				switch v := value.(type) {
				case map[string]any:
					if v["Type"] == "ActionOutput" {
						id, ok := v["OutputUUID"].(string)
						if !ok || id == "" {
							t.Fatalf("action %d has empty output binding", index)
						}
						previous, exists := producers[id]
						if !exists || previous >= index {
							t.Errorf("action %d has missing or nonpreceding exact output binding %q", index, id)
						}
						refs++
					}
					for _, child := range v {
						visit(child, index)
					}
				case []any:
					for _, child := range v {
						visit(child, index)
					}
				}
			}
			for i, act := range acts {
				// Decode the native representation, including typed token attachments.
				raw, err := json.Marshal(act.WFWorkflowActionParameters)
				if err != nil {
					t.Fatal(err)
				}
				var params map[string]any
				if err = json.Unmarshal(raw, &params); err != nil {
					t.Fatal(err)
				}
				visit(params, i)
				if id, ok := params["UUID"].(string); ok && id != "" {
					if prev, duplicate := producers[id]; duplicate {
						t.Fatalf("duplicate producer at %d and %d", prev, i)
					}
					producers[id] = i
				}
			}
			if refs < 8 {
				t.Fatalf("fixture binding coverage unexpectedly small: %d", refs)
			}
			t.Log(fmt.Sprintf("verified %d exact preceding ActionOutput bindings in %d actions", refs, len(acts)))
		})
	}
}
