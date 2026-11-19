package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExplicitDiagnosticManifestCannotBeReplacedByFormalSubset(t *testing.T) {
	valid := `[{"evidence_id":"ev-runtime-menu-alpha","result":"passed","runtime_pass":true},{"evidence_id":"ev-runtime-menu-beta","result":"passed","runtime_pass":true}]`
	for _, test := range []struct {
		name, data string
		wantPass   bool
	}{
		{"both_diagnostic_rows", valid, true},
		{"missing", "", false},
		{"malformed", "[", false},
		{"formal_subset", `[{"evidence_id":"ev-runtime-variables","result":"passed","runtime_pass":true}]`, false},
		{"missing_beta", `[{"evidence_id":"ev-runtime-menu-alpha","result":"passed","runtime_pass":true}]`, false},
		{"duplicate_alpha", strings.TrimSuffix(valid, "]") + `,{"evidence_id":"ev-runtime-menu-alpha","result":"passed","runtime_pass":true}]`, false},
		{"failed_result", strings.Replace(valid, `"result":"passed"`, `"result":"failed"`, 1), false},
		{"false_runtime", strings.Replace(valid, `"runtime_pass":true`, `"runtime_pass":false`, 1), false},
		{"untyped_runtime", strings.Replace(valid, `"runtime_pass":true`, `"runtime_pass":"true"`, 1), false},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, value := range map[string]string{
				"ui_helper_menu_select_alpha.log": mockLog("Alpha", "menu_select_alpha", "staticText", "SpringBoard"),
				"menu-alpha-clipboard-result.txt": "CHERRI_MENU_ALPHA\n",
				"ui_helper_menu_select_beta.log":  mockLog("Beta", "menu_select_beta", "staticText", "SpringBoard"),
				"menu-beta-clipboard-result.txt":  "CHERRI_MENU_BETA\n",
				// A valid obsolete file must never mask a broken explicitly selected diagnostic.
				"runtime-evidence.json": valid,
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(value), 0644); err != nil {
					t.Fatal(err)
				}
			}
			if test.data != "" {
				if err := os.WriteFile(filepath.Join(dir, "runtime-diagnostics.json"), []byte(test.data), 0644); err != nil {
					t.Fatal(err)
				}
			}
			report, err := VerifyDirectoryWithEvidence(dir, "runtime-diagnostics.json")
			if test.wantPass {
				if err != nil || !report.Passed {
					t.Fatalf("valid diagnostic failed: %v", err)
				}
			} else if err == nil || report.Passed {
				t.Fatal("missing, contradictory or partial diagnostic evidence passed")
			}
		})
	}
}
