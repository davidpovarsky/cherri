package syntax

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
)

func parseScopedFixture(t *testing.T, code string) *Program {
	t.Helper()
	parser := NewParser(source.NewFile("block.cherri", "block.cherri", 1, code))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatalf("parse errors: %v\n%s", parser.Errors(), code)
	}
	return program
}

func TestScopedBlockParseFormatAndUnicode(t *testing.T) {
	code := `let result = measure("מדידה {🥑}") {
    let inner = measure("same") { text("{value}") }
    yield inner
}
if (measure("condition") { true }) { text("yes") }
for item in (measure("items") { [1, 2] }) { text(item) }
repeat (measure("count") { 2 }) { text("tick") }
`
	program := parseScopedFixture(t, code)
	call := program.Statements[0].(*BindingStmt).Value.(*CallExpr)
	if call.Block == nil || BlockResultExpression(call.Block.Body) == nil {
		t.Fatal("block/result missing")
	}
	formatted := Format(program)
	again := Format(parseScopedFixture(t, formatted))
	if formatted != again {
		t.Fatalf("formatting is not stable:\n%s\n%s", formatted, again)
	}
	if !strings.Contains(formatted, "מדידה") {
		t.Fatal("Unicode label lost")
	}
}

func TestScopedBlockDoesNotConsumeControlBodies(t *testing.T) {
	program := parseScopedFixture(t, `if number(1) { text("yes") }
for item in list([1]) { text(item) }
repeat number(2) { text("tick") }
`)
	condition := program.Statements[0].(*IfStmt)
	if condition.Condition.(*CallExpr).Block != nil || len(condition.ThenBlock.Statements) != 1 {
		t.Fatal("if body consumed by call")
	}
	loop := program.Statements[1].(*ForStmt)
	if loop.Iterable.(*CallExpr).Block != nil || len(loop.BodyBlock.Statements) != 1 {
		t.Fatal("for body consumed by call")
	}
	repeat := program.Statements[2].(*RepeatStmt)
	if repeat.Count.(*CallExpr).Block != nil || len(repeat.BodyBlock.Statements) != 1 {
		t.Fatal("repeat body consumed by call")
	}
}

func TestScopedBlockNestedArgumentHasOwnBoundary(t *testing.T) {
	program := parseScopedFixture(t, `if check(measure("nested") { true }) { text("ok") }`)
	condition := program.Statements[0].(*IfStmt)
	outer := condition.Condition.(*CallExpr)
	if outer.Block != nil || outer.PrimaryArg.(*CallExpr).Block == nil {
		t.Fatal("nested argument/control block boundary changed")
	}
}
