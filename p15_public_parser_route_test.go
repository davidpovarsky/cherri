package main

import (
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
				producer, consumer := 0, 0
				for _, item := range actions {
					action, ok := item.(map[string]any)
					if !ok {
						t.Fatalf("malformed action: %T", item)
					}
					parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
					switch action["WFWorkflowActionIdentifier"] {
					case "is.workflow.actions.gettext":
						if parameters["WFTextActionText"] == tc.marker {
							producer++
						}
					case "is.workflow.actions.showresult":
						consumer++
					}
				}
				if producer != 1 || consumer != 1 {
					t.Fatalf("wrong public-source native result: producer=%d consumer=%d actions=%#v", producer, consumer, actions)
				}
			})
		}
	}
}
