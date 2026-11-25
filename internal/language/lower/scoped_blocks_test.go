package lower

import (
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The historical test identity is retained. Native renaming now leaves the
// authored AST in place instead of enrolling same-text cloned nodes.
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
	warnings := 0
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity != analysis.SeverityWarning || diagnostic.Code != "W_SHADOWING" {
			t.Fatal(diagnostic)
		}
		warnings++
	}
	if warnings != 2 {
		t.Fatalf("the two authored shadows must retain their warnings, got %d", warnings)
	}
	l := NewLowerer(schema.DefaultRegistry())
	l.SetExprTypes(analyzer.ExprTypes())
	l.prepareOptionalPresenceDemand(program)
	if err := l.lowerLexicalBinding(program.Statements[0].(*syntax.BindingStmt)); err != nil {
		t.Fatal(err)
	}
	outer := l.bindings["outside"]
	call := program.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	body := call.Block.Body
	first := body.Statements[0].(*syntax.BindingStmt)
	object := body.Statements[1].(*syntax.BindingStmt)
	originalFirstValue, originalObjectValue := first.Value, object.Value
	restore, err := l.beginScopedBlockScope(body, "__cherri_test_scope")
	if err != nil {
		t.Fatal(err)
	}
	if err := l.lowerLexicalBinding(first); err != nil {
		t.Fatal(err)
	}
	native := l.bindings["outside"].VariableName
	if !strings.HasPrefix(native, "__cherri_test_scope_") || native == "outside" {
		t.Fatal("scoped native storage was not private")
	}
	if err := l.lowerLexicalBinding(object); err != nil {
		t.Fatal(err)
	}
	if call.Block.Body != body || first.Value != originalFirstValue || object.Value != originalObjectValue ||
		syntax.Format(program) != before || first.Name != "outside" || first.Value.(*syntax.LiteralExpr).Value != "outside" {
		t.Fatal("lexical lowering mutated authored source identities or literal data")
	}
	mapValue := object.Value.(*syntax.MapExpr)
	if mapValue.Entries[0].Key.(*syntax.LiteralExpr).Value != "outside" ||
		mapValue.Entries[0].Value.(*syntax.IdentExpr).Name != "outside" {
		t.Fatal("dictionary key/source reference changed")
	}
	var dictionary any
	count := 0
	for _, action := range l.workflow.Actions {
		if action.AppleIdentifier == "is.workflow.actions.dictionary" {
			dictionary = action.Parameters["WFItems"]
			count++
		}
	}
	if count != 1 || !scopedDictionaryReferenceMatches(dictionary, native) {
		t.Fatal("typed Dictionary lost its exact mutable source reference")
	}
	for _, fault := range []string{"wrong_variable", "wrong_reference_kind", "foreign_producer", "changed_key", "stringified_reference", "extra_scalar", "extra_entry"} {
		t.Run(fault, func(t *testing.T) {
			value := dictionary.(backend.SemanticValue)
			value.DictVal = append([]backend.DictEntry(nil), value.DictVal...)
			reference := *value.DictVal[0].Value.Ref
			value.DictVal[0].Value.Ref = &reference
			switch fault {
			case "wrong_variable":
				reference.ProducerName = "outside"
			case "wrong_reference_kind":
				reference.Kind = backend.RefActionResult
			case "foreign_producer":
				reference.ProducerID = "UNRELATED"
			case "changed_key":
				value.DictVal[0].Key = native
			case "stringified_reference":
				value.DictVal[0].Value = backend.SemanticValue{Kind: backend.ValString, StrVal: native}
			case "extra_scalar":
				value.StrVal = "UNRELATED"
			case "extra_entry":
				value.DictVal = append(value.DictVal, value.DictVal[0])
			}
			if scopedDictionaryReferenceMatches(value, native) {
				t.Fatal("accepted " + fault)
			}
		})
	}
	if !scopedDictionaryReferenceMatches(dictionary, native) {
		t.Fatal("causal mutation changed original evidence")
	}
	restore()
	if !reflect.DeepEqual(l.bindings["outside"], outer) {
		t.Fatal("outer lexical binding was not restored")
	}
	if _, leaked := l.bindings["object"]; leaked {
		t.Fatal("scoped declaration escaped")
	}
	if l.lexical.privatePrefix != "" {
		t.Fatal("private scope prefix leaked")
	}
}

func scopedDictionaryReferenceMatches(value any, native string) bool {
	expected := backend.SemanticValue{Kind: backend.ValDict, DictVal: []backend.DictEntry{{
		Key: "outside",
		Value: backend.SemanticValue{Kind: backend.ValReference, Ref: &backend.Reference{
			Kind: backend.RefMutableBinding, ProducerName: native,
		}},
	}}}
	return reflect.DeepEqual(value, expected)
}
