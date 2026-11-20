package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15OptionalValueJoinsRefineOnlySourceOwnedAbsence(t *testing.T) {
	for _, tc := range []struct {
		name, setup, expression string
		want                    types.Type
	}{
		{"literal_text", "", `if true{yield none}else{yield ""}`, types.OptionalType{Inner: types.Text}},
		{"literal_zero", "", `if true{yield none}else{yield 0}`, types.OptionalType{Inner: types.Number}},
		{"literal_false", "", `if true{yield none}else{yield false}`, types.OptionalType{Inner: types.Bool}},
		{"known_none_alias", "let missing=none\n", `if true{yield missing}else{yield ""}`, types.OptionalType{Inner: types.Text}},
		{"all_absent", "", `if true{yield none}else{yield none}`, types.OptionalType{Inner: types.Unknown}},
		{"all_present", "", `if true{yield ""}else{yield "VALUE"}`, types.Text},
		{"menu", "", `menu("Pick"){case "A"{yield ""}case "B"{yield none}}`, types.OptionalType{Inner: types.Text}},
		{"nested", "", `if true{if true{yield none}else{yield ""}}else{yield "VALUE"}`, types.OptionalType{Inner: types.Text}},
		{"union", "", `if true{yield none}else{if true{yield ""}else{yield 0}}`, types.OptionalType{Inner: types.NewUnion(types.Text, types.Number)}},
		{"opaque_not_absent", "", `if true{yield native.action("is.workflow.actions.getclipboard")}else{yield none}`, types.OptionalType{Inner: types.AnyContent}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := tc.setup + "let candidate=" + tc.expression + "\ngetVariable(candidate)"
			program, analyzer := p15F03Analysis(t, source)
			if errors := p15F03Errors(analyzer); len(errors) != 0 {
				t.Fatal(errors)
			}
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, "candidate")]
			if actual == nil || !actual.Equals(tc.want) || types.MayBeAbsent(actual) != types.MayBeAbsent(tc.want) {
				t.Fatalf("joined type erased absence or invented a present inner value: got %v want %v", actual, tc.want)
			}
		})
	}
}

func TestP15OptionalValueJoinsAnalysisReuse(t *testing.T) {
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	for _, opaque := range []bool{false, true, false} {
		value := `""`
		if opaque {
			value = `native.action("is.workflow.actions.getclipboard")`
		}
		source := "function echo(value:AnyContent?)->AnyContent?{return value}\ngetVariable(echo(if true{yield none}else{yield " + value + "}))"
		program, parseErrors := p15ParseProgram(source)
		if len(parseErrors) != 0 {
			t.Fatal(parseErrors)
		}
		analyzer.Analyze(program)
		errors := p15F03Errors(analyzer)
		if !opaque && len(errors) != 0 {
			t.Fatal("valid join retained stale absent/presence facts", errors)
		}
		if opaque && (len(errors) != 1 || errors[0].Code != analysis.CodeTargetValueShape || !strings.Contains(errors[0].Message, "compiler-owned")) {
			t.Fatal("opaque native branch inherited an earlier source-owned presence fact", errors)
		}
	}
}

func p15OptionalJoinEffectsSource(control string) string {
	expression := `if flag{text("LEFT");yield none}else{text("RIGHT");yield text("")}`
	if control == "menu" {
		expression = `menu("Pick"){case "A"{text("LEFT");yield none}case "B"{text("RIGHT");yield text("")}}`
	}
	return "var flag:Bool=true\nlet candidate=" + expression + "\ngetVariable(candidate==none)"
}

func p15OptionalJoinEffectsGraphError(actions []map[string]any, control string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	p15OptionalJoinFlagGraph(g)
	var group string
	if control == "menu" {
		group = p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, group, "A")
	} else {
		group = g.conditional(4, p15FunctionVariable("flag"), "WFNumberValue", 1)
	}
	g.emit("gettext", map[string]any{"WFTextActionText": "LEFT"})
	p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
	if control == "menu" {
		p15OptionalJoinMenuCase(g, group, "B")
	} else {
		g.boundary(group, 1, false)
	}
	g.emit("gettext", map[string]any{"WFTextActionText": "RIGHT"})
	value := g.value("gettext", map[string]any{"WFTextActionText": ""})
	p15OptionalJoinCarrierGraph(g, true, p15FunctionArgument{true, 0, p15FunctionEdge(value)})
	var joined string
	if control == "menu" {
		joined = p15OptionalJoinMenuEnd(g, group, true)
	} else {
		joined = g.boundary(group, 2, true)
	}
	_, presence := p15DecodeOptionalGraph(g, joined, "has_value", "value")
	absent := p15OptionalJoinNonePredicateGraph(g, p15FunctionEdge(presence))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(absent)})
	return g.compare()
}

func p15OptionalJoinArgumentsSource(reverse bool) string {
	first := `if flag{text("FIRST");yield none}else{yield ""}`
	arguments := first + `,later:text("SECOND")`
	if reverse {
		arguments = `later:text("SECOND"),value:` + first
	}
	return "function pick(value:Text?,later:Text)->Text?{return value}\nvar flag:Bool=true\ngetVariable(pick(" + arguments + "))"
}

func p15OptionalJoinArgumentsGraphError(actions []map[string]any, reverse bool) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("pick")
	value, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	g.get("arg_1", envelope)
	p15OptionalJoinReturnGraph(g, value, p15FunctionEdge(presence))
	g.caller()
	p15OptionalJoinFlagGraph(g)
	later := ""
	if reverse {
		later = g.value("gettext", map[string]any{"WFTextActionText": "SECOND"})
	}
	group := g.conditional(4, p15FunctionVariable("flag"), "WFNumberValue", 1)
	g.emit("gettext", map[string]any{"WFTextActionText": "FIRST"})
	p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
	g.boundary(group, 1, false)
	p15OptionalJoinCarrierGraph(g, true, p15FunctionString(""))
	joined := g.boundary(group, 2, true)
	value, presence = p15DecodeOptionalGraph(g, joined, "has_value", "value")
	if !reverse {
		later = g.value("gettext", map[string]any{"WFTextActionText": "SECOND"})
	}
	carrier := p15OptionalJoinDynamicCarrier(g, "_cherri_optional_argument_v1", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence))
	result := g.call(g.payload("pick", p15FunctionArgument{true, 0, p15FunctionEdge(carrier)}, p15FunctionArgument{true, 0, p15FunctionEdge(later)}))
	decoded, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(decoded)})
	return g.finish()
}

func TestP15OptionalValueJoinsPreserveAuthoredEffectsAndOrder(t *testing.T) {
	cases := []struct {
		name, source string
		check        func([]map[string]any) error
	}{
		{"if_branch_effects", p15OptionalJoinEffectsSource("if"), func(a []map[string]any) error { return p15OptionalJoinEffectsGraphError(a, "if") }},
		{"menu_branch_effects", p15OptionalJoinEffectsSource("menu"), func(a []map[string]any) error { return p15OptionalJoinEffectsGraphError(a, "menu") }},
		{"argument_order", p15OptionalJoinArgumentsSource(false), func(a []map[string]any) error { return p15OptionalJoinArgumentsGraphError(a, false) }},
		{"named_written_order", p15OptionalJoinArgumentsSource(true), func(a []map[string]any) error { return p15OptionalJoinArgumentsGraphError(a, true) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p15FunctionTypedProgram(t, tc.source, map[string]string{})
			original := p15CompiledActions(t, tc.source)
			p15FunctionBoundaryRetain(t, "optional-join-effects-"+tc.name, original)
			if err := tc.check(original); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"move_effect", "duplicate_effect", "drop_effect", "wrong_effect_value"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					target := -1
					for index, action := range bad {
						p := p15C01Params(action)
						if p["WFTextActionText"] == "LEFT" || p["WFTextActionText"] == "FIRST" {
							target = index
							break
						}
					}
					if target < 0 || target+1 >= len(bad) {
						t.Fatal("effect mutation target absent")
					}
					switch fault {
					case "move_effect":
						bad[target], bad[target+1] = bad[target+1], bad[target]
					case "duplicate_effect":
						bad = append(bad[:target], append([]map[string]any{bad[target]}, bad[target:]...)...)
					case "drop_effect":
						bad = append(bad[:target], bad[target+1:]...)
					case "wrong_effect_value":
						p15C01Params(bad[target])["WFTextActionText"] = "WRONG"
					}
					if tc.check(bad) == nil {
						t.Fatal("authored effect loss, duplication or reorder was accepted")
					}
				})
			}
		})
	}
}
