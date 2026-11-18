package analysis

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestScopedBlockInferredTypeAndLexicalScope(t *testing.T) {
	code := `var outside: Number = 7
let value = measure("outer") {
    var outside: Number = 2
    let nested = measure("inner") { [outside, 3] }
    nested
}
let kept: Number = outside
`
	parser := syntax.NewParser(source.NewFile("block.cherri", "block.cherri", 1, code))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	analyzer := NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == SeverityError {
			t.Fatal(diagnostic)
		}
	}
	call := program.Statements[1].(*syntax.BindingStmt).Value
	if inferred := analyzer.ExprTypes()[call]; inferred == nil || inferred.Kind() != types.KindList {
		t.Fatalf("result type lost: %v", inferred)
	}
	if !hasErrorCode(parseAndAnalyze(code+"\ntext(nested)"), CodeUnknownName) {
		t.Fatal("block local escaped its lexical scope")
	}
}

func TestScopedBlockDiagnosticsAndBranchTypes(t *testing.T) {
	for _, test := range []struct{ source, code string }{
		{`measure("x")`, "E_BLOCK_REQUIRED"},
		{`text("x") { text("ignored") }`, "E_BLOCK_UNSUPPORTED"},
		{`measure("") { 1 }`, CodeLiteralRequired},
		{`let label = "x"; measure(label) { 1 }`, CodeLiteralRequired},
		{`measure("x") {}`, "E_BLOCK_VALUE_REQUIRED"},
		{`let x = measure("x") { wait(1) }`, "E_BLOCK_VALUE_REQUIRED"},
		{`measure("x") { yield 1; text("later") }`, "E_BLOCK_EARLY_YIELD"},
		{`let x: Number = measure("x") { text("wrong") }`, CodeArgumentType},
		{`let x: Number = measure("x") { if true { yield 1 } else { yield "wrong" } }`, CodeArgumentType},
		{`measure("x") { if true { yield 1; comment("not a result") } else { yield 2 } }`, "E_BLOCK_CONTROL_RESULT"},
		{`collect([1]) { text("no redundant alias") }`, "E_BLOCK_UNSUPPORTED"},
	} {
		diagnostics := parseAndAnalyze(test.source)
		if !hasErrorCode(diagnostics, test.code) {
			t.Errorf("%s: expected %s, got %v", test.source, test.code, diagnostics)
		}
		for _, diagnostic := range diagnostics {
			if diagnostic.Span.Start.Line < 1 {
				t.Errorf("missing source span: %v", diagnostic)
			}
		}
	}
}

func TestScopedBlockDoesNotReserveExistingFunctionName(t *testing.T) {
	code := `function measure(label: Text) -> Text { return label }
let result = measure("existing function")
`
	for _, diagnostic := range parseAndAnalyze(code) {
		if diagnostic.Severity == SeverityError {
			t.Fatal(diagnostic)
		}
	}
}
