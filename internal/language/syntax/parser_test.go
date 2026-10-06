package syntax

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestParseProgramStatements(t *testing.T) {
	src := `let name = "David"
var count: Number = 1
let result = count + 2 * 3
show(f"Hello {name}, result is {result}")
`
	file := source.NewFile("main.cherri", "file:///main.cherri", 1, src)
	parser := NewParser(file)
	prog := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}
	if len(prog.Statements) != 4 {
		t.Fatalf("expected 4 statements, got %d", len(prog.Statements))
	}

	// Stmt 1: let name = "David"
	b1, ok := prog.Statements[0].(*BindingStmt)
	if !ok || b1.Mutable || b1.Name != "name" {
		t.Errorf("stmt 1 should be let name, got %v", prog.Statements[0])
	}

	// Stmt 2: var count: Number = 1
	b2, ok := prog.Statements[1].(*BindingStmt)
	if !ok || !b2.Mutable || b2.Name != "count" {
		t.Errorf("stmt 2 should be var count, got %v", prog.Statements[1])
	}

	// Stmt 3: count + 2 * 3 => precedence check
	b3, ok := prog.Statements[2].(*BindingStmt)
	if !ok {
		t.Fatalf("stmt 3 should be binding stmt")
	}
	bin, ok := b3.Value.(*BinaryExpr)
	if !ok || bin.Op != TokenPlus {
		t.Fatalf("b3 value should be + binary expr, got %v", b3.Value)
	}
	rightBin, ok := bin.Right.(*BinaryExpr)
	if !ok || rightBin.Op != TokenStar {
		t.Errorf("right operand of + should be * binary expr, got %v", bin.Right)
	}

	// Stmt 4: call show(f"...")
	s4, ok := prog.Statements[3].(*ExprStmt)
	if !ok {
		t.Fatalf("stmt 4 should be ExprStmt")
	}
	call, ok := s4.Expr.(*CallExpr)
	if !ok {
		t.Fatalf("stmt 4 should be CallExpr, got %v", s4.Expr)
	}
	callee, ok := call.Callee.(*IdentExpr)
	if !ok || callee.Name != "show" {
		t.Errorf("callee should be 'show', got %v", call.Callee)
	}
	if call.PrimaryArg == nil {
		t.Errorf("call should have primary argument")
	}
}

func TestParseFunctionDecl(t *testing.T) {
	src := `function multiply(value: Number, factor: Number = 2) -> Number {
    return value * factor
}
let res = multiply(7, factor: 3)
`
	file := source.NewFile("fn.cherri", "file:///fn.cherri", 1, src)
	parser := NewParser(file)
	prog := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}
	if len(prog.Declarations) != 1 {
		t.Fatalf("expected 1 declaration, got %d", len(prog.Declarations))
	}

	fn := prog.Declarations[0].(*FunctionDecl)
	if fn.Name != "multiply" {
		t.Errorf("expected function name multiply, got %q", fn.Name)
	}
	if len(fn.Parameters) != 2 {
		t.Fatalf("expected 2 parameters, got %d", len(fn.Parameters))
	}
	if fn.Parameters[0].Name != "value" || fn.Parameters[1].Name != "factor" {
		t.Errorf("parameter names mismatch: %v", fn.Parameters)
	}
	if fn.Parameters[1].DefaultExpr == nil {
		t.Errorf("parameter factor should have default expression")
	}
}

func TestParseNamedArgsCall(t *testing.T) {
	src := `resizeImage(photo, width: 800, height: 600)`
	file := source.NewFile("call.cherri", "file:///call.cherri", 1, src)
	parser := NewParser(file)
	prog := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	call := prog.Statements[0].(*ExprStmt).Expr.(*CallExpr)
	if call.PrimaryArg == nil {
		t.Fatalf("expected primary argument photo")
	}
	if len(call.NamedArgs) != 2 {
		t.Fatalf("expected 2 named args, got %d", len(call.NamedArgs))
	}
	if call.NamedArgs[0].Label != "width" || call.NamedArgs[1].Label != "height" {
		t.Errorf("named args labels mismatch: %v", call.NamedArgs)
	}
}

func TestParseValueProducingIf(t *testing.T) {
	src := `let title = if count == 0 {
    yield "No items"
} else {
    yield f"{count} items"
}
`
	file := source.NewFile("if.cherri", "file:///if.cherri", 1, src)
	parser := NewParser(file)
	prog := parser.ParseProgram()

	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}
	if len(prog.Statements) != 1 {
		t.Fatalf("expected 1 statement, got %d", len(prog.Statements))
	}

	binding := prog.Statements[0].(*BindingStmt)
	if binding.Name != "title" {
		t.Errorf("expected binding name title")
	}
	ifExpr, ok := binding.Value.(*IfStmt)
	if !ok {
		t.Fatalf("expected IfStmt as binding value, got %T", binding.Value)
	}
	if len(ifExpr.ThenBlock.Statements) != 1 {
		t.Fatalf("expected 1 stmt in then block")
	}
	yieldStmt, ok := ifExpr.ThenBlock.Statements[0].(*YieldStmt)
	if !ok {
		t.Fatalf("expected YieldStmt in then block")
	}
	lit := yieldStmt.Value.(*LiteralExpr)
	if lit.Value != "No items" {
		t.Errorf("expected 'No items', got %q", lit.Value)
	}
}
