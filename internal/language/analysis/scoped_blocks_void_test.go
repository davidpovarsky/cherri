package analysis

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestScopedMeasureStatementBodiesHaveExactVoidType(t *testing.T) {
	for name, body := range map[string]string{
		"empty":           "",
		"wait":            "wait(1)",
		"binding":         "let item = text(\"kept\")",
		"assignment":      "var item = 1; item = 2",
		"if":              "if true { wait(1) } else { wait(2) }",
		"if_missing_else": "if true { wait(1) }",
		"repeat":          "repeat 2 { wait(1) }",
		"nested":          "measure(\"inner\") { wait(1) }",
	} {
		t.Run(name, func(t *testing.T) {
			code := "measure(\"scope\") { " + body + " }\n"
			parser := syntax.NewParser(source.NewFile("void.cherri", "void.cherri", 1, code))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			before := syntax.Format(program)
			analyzer := NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			for _, diagnostic := range analyzer.Diagnostics() {
				if diagnostic.Severity == SeverityError {
					t.Fatal(diagnostic)
				}
			}
			call := program.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr)
			got := analyzer.ExprTypes()[call]
			if got == nil || got.Kind() != types.KindVoid {
				t.Fatalf("statement body invented a result: %v", got)
			}
			if syntax.Format(program) != before {
				t.Fatal("Void analysis changed the authored body")
			}
		})
	}
}
