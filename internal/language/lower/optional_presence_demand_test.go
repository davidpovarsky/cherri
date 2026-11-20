package lower

import (
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func TestOptionalPresenceDemandFollowsActualConsumers(t *testing.T) {
	cases := []struct {
		name, source string
		needed       []string
	}{
		{"none_comparison", `var absent:Text?=none;var unused:Text?=none;getVariable(absent==none)`, []string{"absent"}},
		{"reassignment_alias", `var first:Text?=none;var second:Text?="";var alias:Text?=first;alias=second;getVariable(alias!=none)`, []string{"alias", "first", "second"}},
		{"return", `function f()->Text?{var first:Text?=none;let alias=first;return alias}`, []string{"alias", "first"}},
		{"argument", `function f(value:Text?)->Text?{return value};var first:Text?=none;f(first)`, []string{"first"}},
		{"named_argument", `function f(seed:Text,value:Text?)->Text?{return value};var first:Text?=none;f("seed",value:first)`, []string{"first"}},
		{"qualified_argument", `function f(value:Text?)->Text?{return value};var first:Text?=none;dep.f(first)`, []string{"first"}},
		{"branch_use", `var first:Text?=none;if true{first="";getVariable(first==none)}`, []string{"first"}},
		{"loop_use", `var first:Text?=none;repeat 2{first="";getVariable(first==none)}`, []string{"first"}},
		{"shadow_retention", `var first:Text?=none;{var first:Text?="";getVariable(first==none)}`, []string{"first"}},
		{"canonical_value_only", `var first:Text?=none;count(native.ref("Variable",name:"first"));getVariable(first)`, []string{}},
		{"unused_alias", `var first:Text?=none;let alias:Text?=first;getVariable(alias)`, []string{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := syntax.NewParser(source.NewFile("presence-demand.cherri", "presence-demand.cherri", 1, tc.source))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			before := syntax.Format(program)
			lowerer := NewLowerer(nil)
			lowerer.prepareOptionalPresenceDemand(program)
			got := map[string]bool{}
			for binding, needed := range lowerer.optionalPresenceDemand {
				if needed {
					got[binding.Name] = true
				}
			}
			want := map[string]bool{}
			for _, name := range tc.needed {
				want[name] = true
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("presence demand got%v want%v", got, want)
			}
			if syntax.Format(program) != before {
				t.Fatal("demand pass mutated authored syntax")
			}
			// A cloned/renamed compiler binding is outside this exact source map.
			// It must retain tracking even if its spelling collides with an unused source name.
			if !lowerer.optionalPresenceNeeded(&syntax.BindingStmt{Name: "first"}) {
				t.Fatal("unknown generated binding lost conservative tracking")
			}
		})
	}
}
