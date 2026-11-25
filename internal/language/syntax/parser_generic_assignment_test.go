package syntax

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestGenericTypeCloseAdjacentAssignment(t *testing.T) {
	for _, text := range []string{
		"let value:List<Number>=[1,2]",
		"var value:List<List<Number>>=[[1,2]]",
		"let value:Map<Text,List<Number>>={a:[1,2]}",
		"let value:List<List<List<Number>>>=[[[1]]]",
		"let value:List<Number?>=[none]",
		"// עברית 😀\nlet value:List<Number>=[1]",
	} {
		t.Run(text, func(t *testing.T) {
			file := source.NewFile("generic.cherri", "generic.cherri", 1, text)
			parser := NewParser(file)
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 || len(program.Statements) != 1 {
				t.Fatalf("adjacent assignment failed: %v", parser.Errors())
			}
			binding, ok := program.Statements[0].(*BindingStmt)
			if !ok || binding.Value == nil {
				t.Fatal("typed binding lost initializer")
			}
			equals := strings.Index(text, "=")
			if binding.TypeExpr.NodeSpan().End != file.PositionFromByteOffset(equals) {
				t.Fatalf("generic span consumed assignment: %v", binding.TypeExpr.NodeSpan())
			}
			if binding.Value.NodeSpan().Start != file.PositionFromByteOffset(equals+1) {
				t.Fatal("initializer source position shifted")
			}
			roundTrip := NewParser(source.NewFile("formatted.cherri", "formatted.cherri", 1, Format(program)))
			roundTrip.ParseProgram()
			if len(roundTrip.Errors()) != 0 {
				t.Fatalf("formatted generic binding does not parse: %v", roundTrip.Errors())
			}
		})
	}
}

func TestGenericParameterDefaultPreservesComparison(t *testing.T) {
	text := "function compare(values:List<List<Number>>=[[1]], limit:Number=1)->Bool {return count(values)>=limit}\nlet result=compare()"
	parser := NewParser(source.NewFile("default.cherri", "default.cherri", 1, text))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 || len(program.Declarations) != 1 || len(program.Statements) != 1 {
		t.Fatalf("generic default failed: %v", parser.Errors())
	}
	fn := program.Declarations[0].(*FunctionDecl)
	if fn.Parameters[0].DefaultExpr == nil {
		t.Fatal("generic parameter default vanished")
	}
	statement := fn.Body.Statements[0].(*ReturnStmt)
	comparison, ok := statement.Value.(*BinaryExpr)
	if !ok || comparison.Op != TokenGreaterEqual {
		t.Fatalf("expression >= changed: %#v", statement.Value)
	}
}

func TestGenericAssignmentStillRejectsMalformedSyntax(t *testing.T) {
	for _, text := range []string{"let value:List<Number>=", "let value:List<Number==[1]", "let value:List<List<Number>=[1]"} {
		parser := NewParser(source.NewFile("invalid.cherri", "invalid.cherri", 1, text))
		parser.ParseProgram()
		if len(parser.Errors()) == 0 {
			t.Fatalf("malformed generic binding accepted: %s", text)
		}
	}
}
