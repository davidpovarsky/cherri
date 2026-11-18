package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// The public entry points must choose one source frontend explicitly. The
// shared action declarations/native backend are deliberately still shared.
func TestP15U04PublicFrontendRouteMatrix(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	const v2 = "let route = text(\"ROUTE_V2_ONLY\")\nshow(route)\n"
	const legacy = "const route = text(\"ROUTE_LEGACY_ONLY\")\nshow(route)\n"
	cases := []struct {
		name, source, marker, diagnostic string
		selectors                        []string
		pass                             bool
	}{
		{"default_v2", v2, "ROUTE_V2_ONLY", "", nil, true},
		{"explicit_v2", v2, "ROUTE_V2_ONLY", "", []string{"--mode=v2"}, true},
		{"default_rejects_legacy", legacy, "", "E_LEGACY_SYNTAX", nil, false},
		{"explicit_v2_rejects_legacy", legacy, "", "E_LEGACY_SYNTAX", []string{"--mode=v2"}, false},
		{"legacy_flag", legacy, "ROUTE_LEGACY_ONLY", "", []string{"--legacy"}, true},
		{"legacy_mode", legacy, "ROUTE_LEGACY_ONLY", "", []string{"--mode=legacy"}, true},
		{"invalid_mode", v2, "", "mode", []string{"--mode=not-a-language"}, false},
		{"conflicting_modes", legacy, "", "mode", []string{"--legacy", "--mode=v2"}, false},
	}
	for _, form := range []string{"file", "compile"} {
		for _, tc := range cases {
			t.Run(form+"/"+tc.name, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "route.cherri")
				target := filepath.Join(dir, "route.shortcut")
				if err := os.WriteFile(source, []byte(tc.source), 0600); err != nil {
					t.Fatal(err)
				}
				args := []string{source, "--skip-sign", "--no-ansi", "--load-all-actions", "--output=" + target}
				args = append(args, tc.selectors...)
				if form == "compile" {
					args = append([]string{"compile"}, args...)
				}
				stdout, stderr, err := p15RunCLI(t, binary, dir, nil, args...)
				combined := string(stdout) + string(stderr)
				if !tc.pass {
					if err == nil || !strings.Contains(combined, tc.diagnostic) {
						t.Fatalf("route accepted rejected source or lost diagnostic %q: %v\n%s", tc.diagnostic, err, combined)
					}
					if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
						t.Fatalf("rejected route emitted an artifact: %v", statErr)
					}
					return
				}
				if err != nil {
					t.Fatalf("selected route failed: %v\n%s", err, combined)
				}
				raw, readErr := os.ReadFile(target)
				if readErr != nil {
					t.Fatal(readErr)
				}
				var workflow map[string]any
				if _, decodeErr := plist.Unmarshal(raw, &workflow); decodeErr != nil {
					t.Fatal(decodeErr)
				}
				actions, ok := workflow["WFWorkflowActions"].([]any)
				if !ok {
					t.Fatalf("no decoded action array: %T", workflow["WFWorkflowActions"])
				}
				typed := make([]map[string]any, len(actions))
				for index, item := range actions {
					action, ok := item.(map[string]any)
					if !ok {
						t.Fatalf("malformed action: %T", item)
					}
					typed[index] = action
				}
				if err := p15U04PublicSourceWire(typed, tc.marker); err != nil {
					t.Fatal(err)
				}
				broken := p15V02CorruptLastReference(t, typed, false)
				if err := p15U04PublicSourceWire(broken, tc.marker); err == nil {
					t.Fatal("wrong native output reference satisfied the public route oracle")
				}
			})
		}
	}
}

func p15U04PublicSourceWire(actions []map[string]any, marker string) error {
	producerCount, consumerCount := 0, 0
	producerID := ""
	var consumer map[string]any
	for _, action := range actions {
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		switch action["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.gettext":
			if parameters["WFTextActionText"] == marker {
				producerCount++
				producerID, _ = parameters["UUID"].(string)
			}
		case "is.workflow.actions.showresult":
			consumerCount++
			consumer = parameters
		}
	}
	if producerCount != 1 || consumerCount != 1 || producerID == "" {
		return fmt.Errorf("wrong public-source result: producers=%d consumers=%d producerUUID=%q", producerCount, consumerCount, producerID)
	}
	outputs, variables := p15V02NativeReferences(consumer["Text"])
	if len(outputs) != 1 || outputs[0] != producerID || len(variables) != 0 {
		return fmt.Errorf("Show Result lost exact source identity: outputs=%v variables=%v want=%q", outputs, variables, producerID)
	}
	return nil
}
