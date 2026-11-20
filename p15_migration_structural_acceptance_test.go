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
	full := `#define name RP33_FLOW
#define glyph apple
#define color red
#define inputs text
#define outputs text
#question recipient "Recipient?" "RP33"
#ref publicDocument ` + reference + `
#include 'helper.cherri'
#include 'actions/documents'
alert(recipient, "RP33 setup")
@word = "value"
@box = {"field": "DICT"}
const greeting = rp33Greet(@word)
show(greeting)
show(@box['field'])
saveFile("rp33-public.txt", @word, false, publicDocument)
if @word {
    show("present:{@word}")
}
repeat index for 2 {
    show("index:{@index}")
}
`
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
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			input := filepath.Join(directory, "legacy.cherri")
			if err := os.WriteFile(input, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			if tc.helper {
				helper := `function rp33Greet(text value): text {
    output("FUNC:{@value}")
}
`
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
