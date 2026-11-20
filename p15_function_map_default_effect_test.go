package main

import (
	"fmt"
	"testing"
)

func p15MapDefaultEffectSource(optional bool) string {
	parameter, result := "Map<Text,AnyContent>", "AnyContent"
	if optional {
		parameter += "?"
		result = parameter
	}
	return `function echo(seed:Text,value:` + parameter + ` = {"seed":seed,"first":text("FIRST"),"second":text("SECOND")})->` + result + `{return value}
let supplied=echo("SEED",value:{})
getVariable(supplied)
let omitted=echo("SEED")
getVariable(omitted)`
}

func p15MapDefaultEffectGraph(actions []map[string]any, optional bool) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	positions := map[string]int{}
	envelope := g.header("echo")
	seed := g.get("arg_0", envelope)
	hasValue := g.get("has_arg_1", envelope)
	guard := g.conditional(4, p15FunctionEdge(hasValue), "WFConditionalActionString", "1")
	g.get("arg_1", envelope)
	positions["else"] = len(g.expected)
	g.boundary(guard, 1, false)
	positions["first"] = len(g.expected)
	first := g.value("gettext", map[string]any{"WFTextActionText": "FIRST"})
	positions["second"] = len(g.expected)
	second := g.value("gettext", map[string]any{"WFTextActionText": "SECOND"})
	kind, wire := p15ContainerFieldValue(map[string]any{
		"seed":   p15ContainerReference{"Type": "ActionOutput", "OutputUUID": seed},
		"first":  p15ContainerReference{"Type": "ActionOutput", "OutputUUID": first},
		"second": p15ContainerReference{"Type": "ActionOutput", "OutputUUID": second},
	})
	positions["default"] = len(g.expected)
	if optional {
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", true, p15FunctionArgument{true, int(kind), wire})
	} else {
		g.value("dictionary", map[string]any{"WFItems": wire})
	}
	chosen := g.boundary(guard, 2, true)
	if optional {
		value, presence := p15DecodeOptionalGraph(g, chosen, "has_value", "value")
		present := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(value)})
		g.boundary(present, 1, false)
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
		chosen = g.boundary(present, 2, true)
	}
	positions["return"] = len(g.expected)
	g.output(chosen)
	g.caller()
	for _, supplied := range []bool{true, false} {
		argument := p15FunctionArgument{}
		name := "omitted"
		if supplied {
			name = "supplied"
			kind, wire := p15ContainerFieldValue(map[string]any{})
			argument = p15FunctionArgument{true, int(kind), wire}
			if optional {
				carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", true, argument)
				argument = p15FunctionArgument{true, 0, p15FunctionEdge(carrier)}
			}
		}
		positions[name] = len(g.expected)
		result := g.call(g.payload("echo", p15FunctionString("SEED"), argument))
		if optional {
			result, _ = p15DecodeOptionalGraph(g, result, "has_value", "value")
		}
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	}
	return positions, g.finish()
}

func TestP15T01MapDefaultEffectOrderAndScope(t *testing.T) {
	for _, optional := range []bool{false, true} {
		t.Run(fmt.Sprintf("optional_%t", optional), func(t *testing.T) {
			source := p15MapDefaultEffectSource(optional)
			p15FunctionTypedProgram(t, source, map[string]string{})
			original := p15CompiledActions(t, source)
			p15FunctionBoundaryRetain(t, fmt.Sprintf("map-default-effects-optional-%t", optional), original)
			positions, err := p15MapDefaultEffectGraph(original, optional)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"effects_swapped", "effect_duplicated", "effect_eager", "effect_lost", "previous_parameter_lost", "supplied_absent", "omitted_present", "return_always_default"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15C01Clone(t, original)
					switch fault {
					case "effects_swapped":
						bad[positions["first"]], bad[positions["second"]] = bad[positions["second"]], bad[positions["first"]]
					case "effect_duplicated":
						i := positions["first"]
						bad = append(bad[:i], append([]map[string]any{bad[i]}, bad[i:]...)...)
					case "effect_eager":
						bad[positions["first"]], bad[positions["else"]] = bad[positions["else"]], bad[positions["first"]]
					case "effect_lost":
						bad[positions["first"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.nothing"
					case "previous_parameter_lost":
						fields, err := p15ContainerFields(p15C01Params(bad[positions["default"]])["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						if optional {
							fields, err = p15ContainerFields(fields["value"]["WFValue"])
							if err != nil {
								t.Fatal(err)
							}
						}
						fields["seed"]["WFValue"] = p15FunctionText("REPLACED")
					case "supplied_absent":
						p15FunctionPayloadRow(t, bad, positions["supplied"], "has_arg_1")["WFValue"] = p15FunctionText("0")
					case "omitted_present":
						p15FunctionPayloadRow(t, bad, positions["omitted"], "has_arg_1")["WFValue"] = p15FunctionText("1")
					case "return_always_default":
						p15C01Params(bad[positions["return"]])["WFOutput"] = p15FunctionText("\ufffc", p15C01Params(bad[positions["default"]])["UUID"].(string))
					}
					if _, err := p15MapDefaultEffectGraph(bad, optional); err == nil {
						t.Fatal("default effect/scope corruption accepted")
					}
				})
			}
		})
	}
}
