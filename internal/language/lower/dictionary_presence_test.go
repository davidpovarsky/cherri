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
	kind := l.exprTypes[original]
	restore, err := l.beginScopedBlockScope(call.Block.Body, "__cherri_dictionary_scope")
	if err != nil {
		t.Fatal(err)
	}
	defer restore()
	if call.Block.Body.Statements[1].(*syntax.BindingStmt).Value != original || !l.knownDictionaryIndexes[original] ||
		l.exprTypes[original] == nil || !l.exprTypes[original].Equals(kind) || syntax.Format(program) != before {
		t.Fatal("actual source index lost its presence/type fact")
	}
	cloned := *original.(*syntax.IndexExpr)
	if l.knownDictionaryIndexes[&cloned] || l.exprTypes[&cloned] != nil {
		t.Fatal("same-text unrelated index inherited source-owned facts")
	}
}
