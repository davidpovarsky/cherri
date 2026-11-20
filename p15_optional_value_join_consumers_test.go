package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15OptionalJoinConsumerCases() []p15OptionalValueJoinCase {
	const choose = `if flag {yield none} else {yield ""}`
	const echo = "function echo(value:Text?)->Text?{return value}\n"
	return []p15OptionalValueJoinCase{
		{"annotation", "var flag:Bool=true\nlet value:Text? = " + choose + "\ngetVariable(value==none)"},
		{"return", "function pick(flag:Bool)->Text?{return " + choose + "}\ngetVariable(pick(true))"},
		{"direct_argument", echo + "var flag:Bool=true\ngetVariable(echo(" + choose + "))"},
		{"inferred_argument", echo + "var flag:Bool=true\nlet value=" + choose + "\ngetVariable(echo(value))"},
		{"alias_argument", echo + "var flag:Bool=true\nlet initial=" + choose + "\nlet value=initial\ngetVariable(echo(value))"},
		{"reassignment", echo + "var flag:Bool=true\nvar value:Text?=none\nvalue=" + choose + "\ngetVariable(echo(value))"},
		{"nested_if", "var flag:Bool=true\nlet value=if flag {if flag {yield none}else{yield \"\"}}else{yield \"END\"}\ngetVariable(value==none)"},
		{"yielded_if", "var flag:Bool=true\nlet value=if flag {yield " + choose + "}else{yield \"END\"}\ngetVariable(value==none)"},
		{"nested_menu", "var flag:Bool=true\nlet value=if flag {menu(\"Pick\"){case \"A\"{yield none}case \"B\"{yield \"\"}}}else{yield \"END\"}\ngetVariable(value==none)"},
	}
}

func p15OptionalJoinReturnGraph(g *p15FunctionGraph, value string, presence any) {
	carrier := p15OptionalJoinDynamicCarrier(g, "_cherri_optional_result_v1", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, presence)
	g.output(carrier)
}

func p15OptionalJoinCallResultGraph(g *p15FunctionGraph, name string, argument p15FunctionArgument) {
	called := g.call(g.payload(name, argument))
	decoded, _ := p15DecodeOptionalGraph(g, called, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(decoded)})
}

func p15OptionalJoinNestedGraph(g *p15FunctionGraph, variant string) (value, presence string) {
	outer := g.conditional(4, p15FunctionVariable("flag"), "WFNumberValue", 1)
	if variant == "nested_menu" {
		inner := p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, inner, "A")
		p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
		p15OptionalJoinMenuCase(g, inner, "B")
		p15OptionalJoinCarrierGraph(g, true, p15FunctionString(""))
		p15OptionalJoinMenuEnd(g, inner, false)
	} else if variant == "nested_if" {
		inner := g.conditional(4, p15FunctionVariable("flag"), "WFNumberValue", 1)
		p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
		g.boundary(inner, 1, false)
		p15OptionalJoinCarrierGraph(g, true, p15FunctionString(""))
		g.boundary(inner, 2, false)
	} else {
		decoded, flag := p15OptionalJoinControlGraph(g, "if", p15FunctionVariable("flag"), p15FunctionString(""), false)
		p15OptionalJoinDynamicCarrier(g, "_cherri_optional_join_v1", p15FunctionArgument{true, 0, p15FunctionEdge(decoded)}, p15FunctionEdge(flag))
	}
	g.boundary(outer, 1, false)
	p15OptionalJoinCarrierGraph(g, true, p15FunctionString("END"))
	joined := g.boundary(outer, 2, true)
	return p15DecodeOptionalGraph(g, joined, "has_value", "value")
}

func p15OptionalJoinConsumerGraphError(actions []map[string]any, variant string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	if variant == "annotation" || strings.HasPrefix(variant, "nested_") || variant == "yielded_if" {
		p15OptionalJoinFlagGraph(g)
		var presence string
		if variant == "annotation" {
			_, presence = p15OptionalJoinControlGraph(g, "if", p15FunctionVariable("flag"), p15FunctionString(""), false)
		} else {
			_, presence = p15OptionalJoinNestedGraph(g, variant)
		}
		absent := p15OptionalJoinNonePredicateGraph(g, p15FunctionEdge(presence))
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(absent)})
		g.emit("nothing", nil)
		return g.compare()
	}
	if variant == "return" {
		envelope := g.header("pick")
		flag := g.numberCast(g.get("arg_0", envelope))
		value, presence := p15OptionalJoinControlGraph(g, "if", p15FunctionEdge(flag), p15FunctionString(""), false)
		p15OptionalJoinReturnGraph(g, value, p15FunctionEdge(presence))
		g.caller()
		p15OptionalJoinCallResultGraph(g, "pick", p15FunctionNumber("1"))
		return g.finish()
	}
	envelope := g.header("echo")
	value, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	p15OptionalJoinReturnGraph(g, value, p15FunctionEdge(presence))
	g.caller()
	p15OptionalJoinFlagGraph(g)
	flagName := ""
	if variant == "reassignment" {
		zero := g.value("gettext", map[string]any{"WFTextActionText": "0"})
		if len(g.expected) < len(g.actual) {
			flagName, _ = p15C01Params(g.actual[len(g.expected)])["WFVariableName"].(string)
		}
		if !strings.HasPrefix(flagName, "_cherri_optional_presence_") {
			g.err = fmt.Errorf("missing independent mutable presence identity")
		}
		g.emit("setvariable", map[string]any{"WFVariableName": flagName, "WFInput": p15FunctionEdge(zero)})
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "value"})
	}
	value, presence = p15OptionalJoinControlGraph(g, "if", p15FunctionVariable("flag"), p15FunctionString(""), false)
	valueEdge, presenceEdge := p15FunctionEdge(value), p15FunctionEdge(presence)
	if variant == "reassignment" {
		g.emit("setvariable", map[string]any{"WFVariableName": flagName, "WFInput": presenceEdge})
		g.emit("setvariable", map[string]any{"WFVariableName": "value", "WFInput": valueEdge})
		valueEdge, presenceEdge = p15FunctionVariable("value"), p15FunctionVariable(flagName)
	}
	carrier := p15OptionalJoinDynamicCarrier(g, "_cherri_optional_argument_v1", p15FunctionArgument{true, 0, valueEdge}, presenceEdge)
	p15OptionalJoinCallResultGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionEdge(carrier)})
	return g.finish()
}

func TestP15OptionalValueJoinsCompleteConsumerGraphs(t *testing.T) {
	for _, tc := range p15OptionalJoinConsumerCases() {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			p15FunctionBoundaryRetain(t, "optional-join-complete-"+tc.name, actions)
			if err := p15OptionalJoinConsumerGraphError(actions, tc.name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalValueJoinsConsumerGraphRejectsCorruption(t *testing.T) {
	for _, tc := range p15OptionalJoinConsumerCases() {
		t.Run(tc.name, func(t *testing.T) {
			original := p15CompiledActions(t, tc.source)
			if err := p15OptionalJoinConsumerGraphError(original, tc.name); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"wrong_join_marker", "value_as_flag", "unrelated_input", "lost_operation", "extra_operation"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					if err := p15OptionalJoinConsumerGraphError(bad, tc.name); err != nil {
						t.Fatal("unchanged copy rejected", err)
					}
					changed := false
					for _, action := range bad {
						p := p15C01Params(action)
						if fault == "wrong_join_marker" {
							fields, err := p15ContainerFields(p["WFItems"])
							if err == nil && fields["_cherri_optional_join_v1"] != nil {
								fields["_cherri_optional_join_v1"]["WFValue"] = p15FunctionText("forged")
								changed = true
								break
							}
						} else if p["WFDictionaryKey"] == "has_value" {
							if fault == "value_as_flag" {
								p["WFDictionaryKey"] = "value"
							} else if fault == "unrelated_input" {
								p["WFInput"] = p15FunctionVariable("not_the_carrier")
							} else {
								continue
							}
							changed = true
							break
						}
					}
					if fault == "lost_operation" {
						bad = bad[1:]
						changed = true
					}
					if fault == "extra_operation" {
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
						changed = true
					}
					if !changed || p15OptionalJoinConsumerGraphError(bad, tc.name) == nil {
						t.Fatal("damaged consumer graph accepted or target missing")
					}
				})
			}
		})
	}
}

func p15OptionalJoinEarlyReturnCases() []p15OptionalValueJoinCase {
	cases := []p15OptionalValueJoinCase{
		{"if", `if flag {return "EARLY"}else{yield none}`},
		{"else", `if flag {yield none}else{return "EARLY"}`},
		{"menu", `menu("Pick"){case "A"{return "EARLY"}case "B"{yield none}}`},
		{"nested_if", `if flag {if flag{return "EARLY"}else{yield none}}else{yield ""}`},
		{"nested_menu", `if flag {menu("Pick"){case "A"{return "EARLY"}case "B"{yield none}}}else{yield ""}`},
	}
	for i := range cases {
		cases[i].source = "function pick(flag:Bool)->Text?{\nlet value=" + cases[i].source + "\nreturn value\n}\ngetVariable(pick(true))"
	}
	return cases
}

func p15OptionalJoinEarlyReturnGraphError(actions []map[string]any, variant string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("pick")
	flag := g.numberCast(g.get("arg_0", envelope))
	early := func() {
		carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionString("EARLY"))
		g.output(carrier)
	}
	var joined string
	if variant == "menu" {
		group := p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, group, "A")
		early()
		p15OptionalJoinMenuCase(g, group, "B")
		p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
		joined = p15OptionalJoinMenuEnd(g, group, true)
	} else {
		group := g.conditional(4, p15FunctionEdge(flag), "WFNumberValue", 1)
		switch variant {
		case "else":
			p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
		case "nested_if":
			inner := g.conditional(4, p15FunctionEdge(flag), "WFNumberValue", 1)
			early()
			g.boundary(inner, 1, false)
			p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
			g.boundary(inner, 2, false)
		case "nested_menu":
			inner := p15OptionalJoinMenuStart(g)
			p15OptionalJoinMenuCase(g, inner, "A")
			early()
			p15OptionalJoinMenuCase(g, inner, "B")
			p15OptionalJoinCarrierGraph(g, false, p15FunctionArgument{})
			p15OptionalJoinMenuEnd(g, inner, false)
		default:
			early()
		}
		g.boundary(group, 1, false)
		if variant == "else" {
			early()
		} else {
			p15OptionalJoinCarrierGraph(g, strings.HasPrefix(variant, "nested_"), p15FunctionString(""))
		}
		joined = g.boundary(group, 2, true)
	}
	value, presence := p15DecodeOptionalGraph(g, joined, "has_value", "value")
	p15OptionalJoinReturnGraph(g, value, p15FunctionEdge(presence))
	g.caller()
	p15OptionalJoinCallResultGraph(g, "pick", p15FunctionNumber("1"))
	return g.finish()
}

func TestP15OptionalValueJoinsEarlyReturnOwnership(t *testing.T) {
	for _, tc := range p15OptionalJoinEarlyReturnCases() {
		t.Run(tc.name, func(t *testing.T) {
			p15FunctionTypedProgram(t, tc.source, map[string]string{})
			original := p15CompiledActions(t, tc.source)
			p15FunctionBoundaryRetain(t, "optional-join-early-"+tc.name, original)
			if err := p15OptionalJoinEarlyReturnGraphError(original, tc.name); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"return_as_join", "join_as_return", "missing_early_output", "output_after_join"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					changed := false
					firstOutput := -1
					for i, action := range bad {
						p := p15C01Params(action)
						if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.output" && firstOutput < 0 {
							firstOutput = i
						}
						fields, err := p15ContainerFields(p["WFItems"])
						if err != nil {
							continue
						}
						if fault == "return_as_join" && fields["_cherri_optional_result_v1"] != nil {
							fields["_cherri_optional_result_v1"]["WFKey"] = p15FunctionText("_cherri_optional_join_v1")
							changed = true
							break
						}
						if fault == "join_as_return" && fields["_cherri_optional_join_v1"] != nil {
							fields["_cherri_optional_join_v1"]["WFKey"] = p15FunctionText("_cherri_optional_result_v1")
							changed = true
							break
						}
					}
					if fault == "missing_early_output" && firstOutput >= 0 {
						bad = append(bad[:firstOutput], bad[firstOutput+1:]...)
						changed = true
					}
					if fault == "output_after_join" && firstOutput >= 0 && firstOutput+2 < len(bad) {
						bad[firstOutput], bad[firstOutput+2] = bad[firstOutput+2], bad[firstOutput]
						changed = true
					}
					if !changed || p15OptionalJoinEarlyReturnGraphError(bad, tc.name) == nil {
						t.Fatal("early return lost function-result ownership")
					}
				})
			}
		})
	}
}
