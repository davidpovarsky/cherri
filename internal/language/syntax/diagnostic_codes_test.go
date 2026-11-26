package syntax

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestParseDiagnosticCodesAreAssignedByTheParser(t *testing.T) {
	var cases = []struct{ name, text, want string }{
		{"variable", "@value = \"historical\"\n", CodeLegacySyntax},
		{"constant", "const value = \"historical\"\n", CodeLegacySyntax},
		{"directive", "#define name Historical\n", CodeLegacySyntax},
		{"reference", "show(@value)\n", CodeLegacySyntax},
		{"ordinary_syntax", "function broken(value: Text -> Text {\n return value\n}\n", CodeSyntax},
		{"authored_code_lookalike", "function broken() E_LEGACY_SYNTAX\n", CodeSyntax},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var parser = NewParser(source.NewFile("diagnostic.cherri", "diagnostic.cherri", 1, tc.text))
			parser.ParseProgram()
			if len(parser.Errors()) == 0 {
				t.Fatal("fixture did not reach a real parser rejection")
			}
			for _, observed := range parser.Errors() {
				if observed.DiagnosticCode() != tc.want || observed.Span.Start.Line < 1 || observed.Message == "" {
					t.Fatalf("specific parser classification or original location lost: %+v code=%s", observed, observed.DiagnosticCode())
				}
			}
		})
	}
}

func TestUnclassifiedParseErrorMessageCannotForgeACode(t *testing.T) {
	var observed = ParseError{Message: "Error [E_LEGACY_SYNTAX]: public authored token"}
	if observed.DiagnosticCode() != CodeSyntax {
		t.Fatal("a message forged diagnostic metadata")
	}
}
