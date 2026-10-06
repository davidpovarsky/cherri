package analysis

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func parseAndAnalyze(code string) []Diagnostic {
	file := source.NewFile("test.cherri", "file:///test.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	analyzer := NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(prog)

	var diags []Diagnostic
	for _, pe := range parser.Errors() {
		code := CodeSyntax
		if strings.Contains(pe.Message, CodeLegacySyntax) {
			code = CodeLegacySyntax
		}
		diags = append(diags, Diagnostic{
			Code:     code,
			Severity: SeverityError,
			Span:     pe.Span,
			Message:  pe.Message,
		})
	}
	diags = append(diags, analyzer.Diagnostics()...)
	return diags
}

func hasErrorCode(diags []Diagnostic, code string) bool {
	for _, d := range diags {
		if d.Code == code {
			return true
		}
	}
	return false
}

func TestAnalyzerAssignImmutable(t *testing.T) {
	code := `let x = 42
x = 10
`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeAssignImmutable) {
		t.Fatalf("expected E_ASSIGN_IMMUTABLE, got %v", diags)
	}
}

func TestAnalyzerReadBeforeInit(t *testing.T) {
	code := `var x: Number
let y = x + 1
`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeReadBeforeInit) {
		t.Fatalf("expected E_READ_BEFORE_INIT, got %v", diags)
	}
}

func TestAnalyzerUnknownArgument(t *testing.T) {
	code := `alert(fakeArg: "oops")`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeUnknownArgument) {
		t.Fatalf("expected E_UNKNOWN_ARGUMENT, got %v", diags)
	}
}

func TestAnalyzerMissingArgument(t *testing.T) {
	code := `resizeImage()`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeMissingArgument) {
		t.Fatalf("expected E_MISSING_ARGUMENT, got %v", diags)
	}
}

func TestAnalyzerDuplicateArgument(t *testing.T) {
	code := `let img = "sample"
resizeImage(img, width: 100, width: 200)
`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeDuplicateArgument) {
		t.Fatalf("expected E_DUPLICATE_ARGUMENT, got %v", diags)
	}
}

func TestAnalyzerIfConditionBool(t *testing.T) {
	code := `if 42 {
    let x = 1
}
`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeArgumentType) {
		t.Fatalf("expected E_ARGUMENT_TYPE on if condition, got %v", diags)
	}
}

func TestAnalyzerYieldOutsideValueBlock(t *testing.T) {
	code := `yield "hello"`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeYieldContext) {
		t.Fatalf("expected E_YIELD_CONTEXT, got %v", diags)
	}
}

func TestAnalyzerLegacySyntax(t *testing.T) {
	code := `@myVar = 10`
	diags := parseAndAnalyze(code)
	if !hasErrorCode(diags, CodeLegacySyntax) {
		t.Fatalf("expected E_LEGACY_SYNTAX on @myVar, got %v", diags)
	}
}

func TestAnalyzerValidProgram(t *testing.T) {
	code := `let name = "David"
var count = 1
count = 2
let greeting = f"Hello {name}, count is {count}"
show(greeting)
`
	diags := parseAndAnalyze(code)
	for _, d := range diags {
		if d.Severity == SeverityError {
			t.Fatalf("unexpected error diagnostic: %v", d)
		}
	}
}
