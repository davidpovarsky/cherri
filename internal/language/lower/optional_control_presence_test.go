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
			// Renamed scoped bodies retain exact positive and negative demand,
			// together with the checked nullable type of the source expression.
			lowerer.exprTypes = map[syntax.Expression]types.Type{value: types.OptionalType{Inner: types.Text}}
			renamer := scopedBlockRenamer{lowerer: lowerer, prefix: "test_scope"}
			cloned, err := renamer.expression(value, map[string]string{})
			if err != nil || cloned == value || lowerer.optionalJoinNeeded(cloned) != tc.needed {
				t.Fatalf("renamed join lost its demand/type fact: %v", err)
			}
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
