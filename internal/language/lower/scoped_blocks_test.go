package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestScopedBlockRenamingPreservesSourceAndLiteralValues(t *testing.T) {
	code := `let outside = "outer"
let result = measure("scope") {
    var outside = "outside"
    let object = {"outside": outside}
    if true { var outside = "branch"; text(outside) }
    object
}`
	parser := syntax.NewParser(source.NewFile("block.cherri", "block.cherri", 1, code))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	before := syntax.Format(program)
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	lowerer := NewLowerer(schema.DefaultRegistry())
	lowerer.SetExprTypes(analyzer.ExprTypes())
	call := program.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	renamer := scopedBlockRenamer{lowerer: lowerer, prefix: "__cherri_test_scope"}
	body, err := renamer.block(call.Block.Body, nil)
	if err != nil {
		t.Fatal(err)
	}
	if syntax.Format(program) != before {
		t.Fatal("scope lowering mutated original source AST")
	}
	first := body.Statements[0].(*syntax.BindingStmt)
	if !strings.HasPrefix(first.Name, "__cherri_test_scope_") || first.Value.(*syntax.LiteralExpr).Value != "outside" {
		t.Fatal("hygienic renaming modified literal data")
	}
	object := body.Statements[1].(*syntax.BindingStmt).Value.(*syntax.MapExpr)
	if object.Entries[0].Key.(*syntax.LiteralExpr).Value != "outside" || object.Entries[0].Value.(*syntax.IdentExpr).Name != first.Name {
		t.Fatal("dictionary key/reference scope confused")
	}
	final := syntax.BlockResultExpression(body)
	if lowerer.exprTypes[final] == nil {
		t.Fatal("cloned expression lost its analyzed result type")
	}
}
