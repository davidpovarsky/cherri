package main

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// These signatures use the analyzed call's exact source identity. The cursor
// can be on another line, after Unicode text, or inside a same-name nested call.
func TestP15SignatureHelpBindsAnalyzedCallee(t *testing.T) {
	for _, tc := range []struct {
		name, source, result string
		parameter            int
	}{
		{"multiline_scalar", "let one = chooseFromList(\n [\"a\"],\n selectMultiple: false|\n)\nlet many = chooseFromList([\"b\"], selectMultiple: true)", "Text", 2},
		{"multiline_list", "let one = chooseFromList([\"a\"])\nlet many = chooseFromList(\n [\"b\"],\n selectMultiple: true|\n)", "List<Text>", 2},
		{"nested_inner", "let many = chooseFromList([chooseFromList([\"a\"], selectMultiple: false|)], selectMultiple: true)", "Text", 2},
		{"nested_outer", "let many = chooseFromList([chooseFromList([\"a\"], selectMultiple: false)], selectMultiple: true|)", "List<Text>", 2},
		{"unicode_number", "let number = prompt(\"ערך 😀, q\", inputType: .Nu|mber)", "Number", 1},
		{"distinct_prompt", "let number = prompt(\"n\", inputType: .Number)\nlet textValue = prompt(\n \"t\", inputType: .Text|\n)", "Text", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			offset := strings.Index(tc.source, "|")
			if offset < 0 || strings.Count(tc.source, "|") != 1 {
				t.Fatal("fixture requires exactly one cursor")
			}
			prefix := tc.source[:offset]
			source := tc.source[:offset] + tc.source[offset+1:]
			line := strings.Count(prefix, "\n") + 1
			column := utf8.RuneCountInString(prefix[strings.LastIndex(prefix, "\n")+1:]) + 1
			svc := service.NewService(nil)
			const uri = "file:///p15-signature-callee.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ := svc.Analyze(uri)
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			signature := svc.SignatureHelp(uri, line, column)
			if signature == nil || !strings.HasSuffix(signature.Label, " -> "+tc.result) || signature.ActiveParameter != tc.parameter {
				t.Fatalf("cursor %d:%d: want result %s parameter %d, got %+v", line, column, tc.result, tc.parameter, signature)
			}
		})
	}
}
