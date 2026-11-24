package lower

import (
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
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

func TestImportedOptionalPresenceDemandRetainsRealConsumers(t *testing.T) {
	cases := []struct {
		name, text string
		needed     bool
	}{
		{"ordinary_value", "target=parsed[\"key\"];target=parsed[\"key\"];show(target)", false},
		{"literal_text", "target=\"return target == none\";show(target)", false},
		{"none_comparison", "target=parsed[\"key\"];show(target==none)", true},
		{"alias", "target=parsed[\"key\"];let alias=target;show(alias!=none)", true},
		{"return", "target=parsed[\"key\"];return target", true},
		{"argument", "function echo(value:AnyContent?)->AnyContent?{return value};target=parsed[\"key\"];echo(target)", true},
		{"named_argument", "function echo(seed:Text,value:AnyContent?)->AnyContent?{return value};target=parsed[\"key\"];echo(\"seed\",value:target)", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := syntax.NewParser(source.NewFile("import-demand.cherri", "import-demand.cherri", 1, tc.text))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			before := syntax.Format(program)
			lowerer := NewLowerer(nil)
			if !lowerer.importedOptionalPresenceNeeded("target") {
				t.Fatal("unanalyzed imported name lost tracking")
			}
			lowerer.prepareOptionalPresenceDemand(program)
			if got := lowerer.importedOptionalPresenceNeeded("target"); got != tc.needed {
				t.Fatalf("actual consumer demand %v want %v", got, tc.needed)
			}
			if !lowerer.importedOptionalPresenceNeeded("unknown_generated") {
				t.Fatal("unregistered imported/generated name lost tracking")
			}
			if !lowerer.optionalPresenceNeeded(&syntax.BindingStmt{Name: "target"}) {
				t.Fatal("name demand leaked into unregistered authored/generated binding")
			}
			if syntax.Format(program) != before {
				t.Fatal("presence pass changed syntax")
			}
		})
	}
}

func TestLoopResultItemsCountPresenceDemand(t *testing.T) {
	cases := []struct {
		name, text string
		needed     bool
	}{
		{"unused_inline_repeat", "var candidate:Text?=none;var unused:Text?=none;count(repeat 2 {yield candidate})", true},
		{"unused_inline_for", "var candidate:Text?=none;var unused:Text?=none;count(for _ in [1,2] {yield candidate})", true},
		{"named_mode_keeps_boundary", "var candidate:Text?=none;var unused:Text?=none;count(repeat 2 {yield candidate},type:\"Characters\")", false},
		{"aliased_collection_keeps_boundary", "var candidate:Text?=none;var unused:Text?=none;let values=repeat 2 {yield candidate};count(values)", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			parser := syntax.NewParser(source.NewFile("loop-cell-demand.cherri", "loop-cell-demand.cherri", 1, tc.text))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			before := syntax.Format(program)
			lowerer := NewLowerer(nil)
			lowerer.prepareOptionalPresenceDemand(program)
			seen := map[string]bool{}
			for binding, needed := range lowerer.optionalPresenceDemand {
				seen[binding.Name] = needed
			}
			if seen["candidate"] != tc.needed || seen["unused"] {
				t.Fatalf("closed consumer presence demand: %v, candidate want %v", seen, tc.needed)
			}
			if syntax.Format(program) != before {
				t.Fatal("presence collection changed authored syntax")
			}
		})
	}
}

func TestLoopResultCellsRequireNormalCompletion(t *testing.T) {
	// Hold the checked element type constant while changing actual source flow.
	// This exercises lowering's guard without analysis rejecting the source first.
	cases := []struct {
		name, loop, termination string
		needed                  bool
	}{
		{"repeat_return", "repeat 2", "return 7", false},
		{"for_return", "for item in [1,2]", "return 7", false},
		{"repeat_if_all_return", "repeat 2", "if flag {return 7} else {return 8}", false},
		{"for_menu_all_return", "for item in [1,2]", "menu(\"Pick\") {case \"A\" {return 7} case \"B\" {return 8}}", false},
		{"repeat_if_one_return", "repeat 2", "if flag {return 7}", true},
		{"for_menu_one_return", "for item in [1,2]", "menu(\"Pick\") {case \"A\" {return 7} case \"B\" {let kept=8}}", true},
		{"repeat_inner_zero_return", "repeat 2", "repeat 0 {return 7}", true},
		{"for_inner_zero_return", "for item in [1,2]", "for nested in [] {return 7}", true},
		{"repeat_zero_normal_body", "repeat 0", "", true},
		{"for_empty_normal_body", "for item in []", "", true},
	}
	for _, tc := range cases {
		for _, payload := range []struct {
			name, source string
			element      types.Type
		}{
			{"list", "[\"tail\"]", types.NewList(types.Text)},
			{"optional", "none", types.OptionalType{Inner: types.Number}},
		} {
			t.Run(tc.name+"/"+payload.name, func(t *testing.T) {
				text := "let flag=true\nlet values=" + tc.loop + " {" + tc.termination + "\nyield " + payload.source + "}"
				parser := syntax.NewParser(source.NewFile("loop-normal-completion.cherri", "loop-normal-completion.cherri", 1, text))
				program := parser.ParseProgram()
				if len(parser.Errors()) != 0 {
					t.Fatal(parser.Errors())
				}
				before := syntax.Format(program)
				binding, okay := program.Statements[len(program.Statements)-1].(*syntax.BindingStmt)
				if !okay {
					t.Fatal("fixture must bind the actual parsed loop")
				}
				lowerer := NewLowerer(nil)
				lowerer.exprTypes = map[syntax.Expression]types.Type{binding.Value: types.NewList(payload.element)}
				if got := lowerer.loopResultNeedsCells(binding.Value); got != tc.needed {
					t.Fatalf("normal-completion cell demand: got %v, want %v", got, tc.needed)
				}
				// The same control flow never creates a cell demand for a
				// present scalar element.
				lowerer.exprTypes[binding.Value] = types.NewList(types.Number)
				if lowerer.loopResultNeedsCells(binding.Value) {
					t.Fatal("present scalar loop acquired a cell requirement")
				}
				if syntax.Format(program) != before {
					t.Fatal("cell-demand check changed authored syntax")
				}
			})
		}
	}
}
