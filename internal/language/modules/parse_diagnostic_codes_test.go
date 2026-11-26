package modules

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestModuleParseDiagnosticCodesSurviveMigrationBoundary(t *testing.T) {
	var cases = []struct{ name, text, want string }{
		{"legacy", "@value = \"historical\"\n", syntax.CodeLegacySyntax},
		{"syntax", "function broken(value: Text -> Text {\n return value\n}\n", syntax.CodeSyntax},
		{"lookalike", "function broken() E_LEGACY_SYNTAX\n", syntax.CodeSyntax},
	}
	for _, tc := range cases {
		for _, migrated := range []bool{false, true} {
			var name = tc.name + "/without_migrator"
			if migrated {
				name = tc.name + "/after_migrator"
			}
			t.Run(name, func(t *testing.T) {
				var parser = syntax.NewParser(source.NewFile("main.cherri", "main.cherri", 1, "import \"./dependency.cherri\" as dependency\n"))
				var program = parser.ParseProgram()
				if len(parser.Errors()) != 0 {
					t.Fatal(parser.Errors())
				}
				var migrate func(string) string
				var calls int
				if migrated {
					migrate = func(text string) string { calls++; return text }
				}
				var graph, issue = Resolve(program, "main.cherri", map[string]string{"dependency.cherri": tc.text}, schema.DefaultRegistry(), migrate)
				if graph != nil || issue == nil || issue.Code != tc.want || issue.Span.Start.Line != 1 {
					t.Fatalf("module rejection lost the original parser classification: graph=%v issue=%+v", graph, issue)
				}
				if migrated && calls != 1 || !migrated && calls != 0 {
					t.Fatal("migration boundary was not exercised", calls)
				}
			})
		}
	}
}
