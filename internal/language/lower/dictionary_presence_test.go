package lower

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestScopedBlockDictionaryPresenceSurvivesRenaming(t *testing.T) {
	code := `let result=measure("lookup"){let checked=require({"key":""},type:"dictionary");let selected:Text?=checked["key"];selected!=none}`
	parser := syntax.NewParser(source.NewFile("dictionary.cherri", "dictionary.cherri", 1, code))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	before := syntax.Format(program)
	a := analysis.NewAnalyzer(schema.DefaultRegistry())
	a.Analyze(program)
	if len(a.Diagnostics()) != 0 {
		t.Fatal(a.Diagnostics())
	}
	l := NewLowerer(a.Registry())
	l.SetExprTypes(a.ExprTypes())
	l.SetKnownPresentDictionaryIndexes(a.KnownPresentDictionaryIndexes())
	call := program.Statements[0].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	original := call.Block.Body.Statements[1].(*syntax.BindingStmt).Value
	renamer := scopedBlockRenamer{lowerer: l, prefix: "__cherri_dictionary_scope"}
	body, err := renamer.block(call.Block.Body, nil)
	if err != nil {
		t.Fatal(err)
	}
	cloned := body.Statements[1].(*syntax.BindingStmt).Value
	if original == cloned || !l.knownDictionaryIndexes[original] || !l.knownDictionaryIndexes[cloned] {
		t.Fatal("exact source index fact was lost or source node reused")
	}
	if l.exprTypes[cloned] == nil || !l.exprTypes[cloned].Equals(l.exprTypes[original]) || syntax.Format(program) != before {
		t.Fatal("clone lost type evidence or mutated source")
	}
}
