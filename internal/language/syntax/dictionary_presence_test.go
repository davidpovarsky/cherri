package syntax

import (
	"github.com/electrikmilk/cherri/internal/language/source"
	"testing"
)

func TestStaticDictionaryElementRequiresCompleteLiteralKeys(t *testing.T) {
	for _, tc := range []struct {
		name, code string
		found      bool
	}{
		{"literal", `{"key":""}["key"]`, true},
		{"raw_key", `{'key':false}["key"]`, true},
		{"raw_index", `{"key":0}['key']`, true},
		{"unicode", `{"מפתח🔑":""}["מפתח🔑"]`, true},
		{"none_is_only_slot_proof", `{"key":none}["key"]`, true},
		{"empty", `{}["key"]`, false},
		{"missing", `{"other":1}["key"]`, false},
		{"duplicate", `{"key":1,"key":2}["key"]`, false},
		{"duplicate_sibling", `{"key":1,"other":2,"other":3}["key"]`, false},
		{"computed_key", `{text("key"):1}["key"]`, false},
		{"computed_sibling", `{"key":1,text("other"):2}["key"]`, false},
		{"number_index", `{"key":1}[0]`, false},
		{"computed_index", `{"key":1}[text("key")]`, false},
		{"empty_index", `{"key":1}[""]`, false},
		{"empty_sibling", `{"key":1,"":2}["key"]`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := NewParser(source.NewFile("dictionary.cherri", "dictionary.cherri", 1, "let value="+tc.code))
			program := p.ParseProgram()
			if len(p.Errors()) != 0 {
				t.Fatal(p.Errors())
			}
			before := Format(program)
			index := program.Statements[0].(*BindingStmt).Value.(*IndexExpr)
			dictionary := index.Target.(*MapExpr)
			element, found := StaticDictionaryElement(dictionary, index.Index)
			if found != tc.found || (!found && element != nil) || (found && element != dictionary.Entries[0].Value) {
				t.Fatal("incorrect authored slot", element, found)
			}
			if Format(program) != before {
				t.Fatal("selection mutated source AST")
			}
		})
	}
	if value, found := StaticDictionaryElement(nil, nil); found || value != nil {
		t.Fatal("nil source minted slot")
	}
}
