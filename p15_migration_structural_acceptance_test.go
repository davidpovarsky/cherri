package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func TestP15RP33SyntaxAwareMigrationDifferential(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	reference := base64.StdEncoding.EncodeToString([]byte(`{"fileLocation":{"fileProviderDomainID":"com.apple.files","relativeSubpath":"rp33-public.txt"}}`))
	full, _ := p15RP33FullProgram()
	cases := []struct {
		name, source string
		helper       bool
	}{
		{"literal_text_control", `text("migration-control")
`, false},
		{"raw_backslash_text", `text('raw \n')
`, false},
		{"full_program", full, true},
		{"literal_condition", `text("if @word !contains marker { literal")
`, false},
		{"escaped_interpolation", `@word = "value"
text("quoted \"hi\" {@word}")
`, false},
		{"raw_interpolation_text", `@word = "value"
text('raw {@word}')
`, false},
		{"directive_in_block_comment", `#define name RP33_REAL
#define color red
/*
#define color blue
*/
text("comment-safe")
`, false},
		{"directive_in_multiline_literal", `#define name RP33_REAL
text("literal-start
#define color blue
literal-end")
`, false},
	}
	cases = append(cases, []struct {
		name, source string
		helper       bool
	}{
		{"json_escape_rules", `@box = {"letter": "\u05d0"}
show(@box['letter'])
`, false},
		{"legacy_unknown_escape", `text("\u0041")
`, false},
		{"placeholder_collision", `@word = "value"
text("__cherri_migration_literal_0__ {@word}")
`, false},
		{"literal_braces_with_interpolation", `@word = "value"
text("before { literal {@word} after }")
`, false},
		{"include_in_comment", `/*
#include 'helper.cherri'
#define color blue
*/
text("inactive-include")
`, true},
		{"raw_action_literal", `@word = "must-not-be-attached"
rawAction("is.workflow.actions.gettext", {"WFTextActionText": "{@word}"})
`, false},
		{"raw_action_dollar_reference", `@word = "raw-action-output"
rawAction("is.workflow.actions.showresult", {"Text": "${@word}"})
`, false},
		{"raw_action_mixed_text", `@word = "raw-action-output"
rawAction("is.workflow.actions.gettext", {"WFTextActionText": "prefix:{@word}:suffix"})
`, false},
		{"reference_callable_collision", "#ref rp33Same " + reference + `
#include 'actions/documents'
function rp33Same(text value): text {
    output("SAME:{@value}")
}
const echoed = rp33Same("local")
show(echoed)
saveFile("rp33-public.txt", echoed, false, rp33Same)
`, false},
	}...)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "legacy.cherri")
			if err := os.WriteFile(input, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.helper {
				_, helper := p15RP33FullProgram()
				if err := os.WriteFile(filepath.Join(directory, "helper.cherri"), []byte(helper), 0600); err != nil {
					t.Fatal(err)
				}
			}
			legacy := filepath.Join(directory, "legacy.shortcut")
			stdout, stderr, err := p15RunCLI(t, binary, directory, nil, input, "--legacy", "--skip-sign", "--output="+legacy)
			if err != nil {
				t.Fatalf("LEGACY_FIXTURE_INVALID: %v stdout=%s stderr=%s", err, stdout, stderr)
			}
			converted, stderr, err := p15RunCLI(t, binary, directory, nil, "migrate", input)
			if err != nil {
				t.Fatalf("MIGRATION_FAILED_AFTER_VALID_LEGACY: %v stdout=%s stderr=%s", err, converted, stderr)
			}
			after, err := os.ReadFile(input)
			if err != nil || !bytes.Equal(after, []byte(tc.source)) {
				t.Fatal("preview changed input")
			}
			v2source := filepath.Join(directory, "converted.cherri")
			if err := os.WriteFile(v2source, converted, 0600); err != nil {
				t.Fatal(err)
			}
			v2 := filepath.Join(directory, "converted.shortcut")
			stdout, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", v2source, "--skip-sign", "--output="+v2)
			if err != nil {
				t.Fatalf("V2_FAILED_AFTER_VALID_LEGACY: %v stdout=%s stderr=%s migrated=%s", err, stdout, stderr, converted)
			}
			if tc.name == "full_program" {
				p15RP33QuestionTargets(t, legacy, v2)
			}
			comparison, err := shortcutcompare.CompareFilesNormalized(legacy, v2)
			if err != nil {
				t.Fatal(err)
			}
			encoded, _ := json.Marshal(map[string]any{"case": tc.name, "migrated": string(converted), "equal": comparison.Equal, "differences": comparison.Differences})
			t.Log("RP33_NATIVE_DIFFERENTIAL:" + string(encoded))
			if !comparison.Equal {
				t.Errorf("MIGRATION_NATIVE_DIFFERENCE: %v", comparison.Differences)
			}
		})
	}
}
