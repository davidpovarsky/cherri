package lower

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestOptionalControlPresenceDemandFollowsConsumers(t *testing.T) {
	const join = `if true{yield none}else{yield ""}`
	for _, tc := range []struct {
		name, source string
		needed       bool
	}{
		{"value_only", "let candidate=" + join + ";getVariable(candidate)", false},
		{"unconsumed_alias", "let candidate=" + join + ";let alias=candidate;getVariable(alias)", false},
		{"none_comparison", "let candidate=" + join + ";getVariable(candidate==none)", true},
		{"alias_comparison", "let candidate=" + join + ";let alias=candidate;getVariable(alias!=none)", true},
		{"function_argument", "function f(v:Text?)->Text?{return v};let candidate=" + join + ";f(candidate)", true},
		{"qualified_function_argument", "function f(v:Text?)->Text?{return v};let candidate=" + join + ";dep.f(candidate)", true},
		{"function_return", "function f()->Text?{let candidate=" + join + ";return candidate}", true},
		{"reassignment", "var alias:Text?=none;let candidate=" + join + ";alias=candidate;getVariable(alias==none)", true},
		{"branch_consumer", "let candidate=" + join + ";if true{getVariable(candidate==none)}", true},
		{"loop_consumer", "let candidate=" + join + ";repeat 2{getVariable(candidate==none)}", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parser := syntax.NewParser(source.NewFile("join-demand.cherri", "join-demand.cherri", 1, tc.source))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			before := syntax.Format(program)
			lowerer := NewLowerer(nil)
			lowerer.prepareOptionalPresenceDemand(program)
			var value syntax.Expression
			optionalDemandWalk(program, func(node syntax.Node) {
				if binding, ok := node.(*syntax.BindingStmt); ok && binding.Name == "candidate" {
					if value != nil {
						t.Fatal("test source has ambiguous candidate")
					}
					value = binding.Value
				}
			})
			if value == nil {
				t.Fatal("source join missing")
			}
			got, known := lowerer.optionalJoinDemand[value]
			if !known || got != tc.needed {
				t.Fatalf("join demand got %t/%t want %t", got, known, tc.needed)
			}
			if syntax.Format(program) != before {
				t.Fatal("demand analysis changed source syntax")
			}
			// Actual scoped nodes retain exact positive and negative demand,
			// without copying authority to same-text AST clones.
			lowerer.exprTypes = map[syntax.Expression]types.Type{value: types.OptionalType{Inner: types.Text}}
			body := &syntax.BlockStmt{Statements: []syntax.Statement{&syntax.ExprStmt{Expr: value}}}
			restore, err := lowerer.beginScopedBlockScope(body, "test_scope")
			if err != nil {
				t.Fatal(err)
			}
			if body.Statements[0].(*syntax.ExprStmt).Expr != value || lowerer.optionalJoinNeeded(value) != tc.needed || syntax.Format(program) != before {
				t.Fatal("actual scoped join lost its demand/type/source fact")
			}
			restore()
			unknown := &syntax.IfStmt{}
			lowerer.exprTypes[unknown] = types.OptionalType{Inner: types.Text}
			if !lowerer.optionalJoinNeeded(unknown) {
				t.Fatal("unknown generated nullable control lost conservative demand")
			}
			lowerer.exprTypes[unknown] = types.Text
			if lowerer.optionalJoinNeeded(unknown) {
				t.Fatal("definitely present generated control gained Optional carrier")
			}
		})
	}
}
