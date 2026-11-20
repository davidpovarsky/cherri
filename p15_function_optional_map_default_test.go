package main

import (
	"fmt"
	"testing"
)

type p15OptionalMapDefaultCase struct {
	name, source string
	value        map[string]any
}

func p15OptionalMapDefaultCases() []p15OptionalMapDefaultCase {
	return []p15OptionalMapDefaultCase{
		{"empty", `{}`, map[string]any{}},
		{"reserved", `{"WFSerializationType":"WFTextTokenString","Value":"ordinary"}`, map[string]any{"WFSerializationType": "WFTextTokenString", "Value": "ordinary"}},
		{"nested_empty", `{"list":[],"map":{},"flag":false,"zero":0,"text":""}`, map[string]any{"list": []any{}, "map": map[string]any{}, "flag": false, "zero": p15ContainerNumber("0"), "text": ""}},
		{"literal_braces", `{"text":"{ordinary}"}`, map[string]any{"text": "{ordinary}"}},
	}
}

func p15OptionalMapDefaultSource(tc p15OptionalMapDefaultCase, mode string) string {
	argument := ""
	if mode == "none" {
		argument = "none"
	} else if mode == "empty" {
		argument = "{}"
	}
	return `function echo(value:Map<Text,AnyContent>? = ` + tc.source + `)->Map<Text,AnyContent>? {return value}
let result=echo(` + argument + `)
getVariable(result)`
}

// Default presence, supplied-argument presence and returned value presence are
// separate. The complete expected graph is authored from source values; only
// UUID/control alpha names are borrowed from the observed compiled graph.
func p15OptionalMapDefaultGraph(actions []map[string]any, tc p15OptionalMapDefaultCase, mode string) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	positions := map[string]int{}
	envelope := g.header("echo")
	supplied := g.get("has_arg_0", envelope)
	positions["guard"] = len(g.expected)
	guard := g.conditional(4, p15FunctionEdge(supplied), "WFConditionalActionString", "1")
	positions["value"] = len(g.expected)
	g.get("arg_0", envelope)
	g.boundary(guard, 1, false)
	positions["default"] = len(g.expected)
	kind, wire := p15ContainerFieldValue(tc.value)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", true, p15FunctionArgument{true, int(kind), wire})
	positions["join"] = len(g.expected)
	chosen := g.boundary(guard, 2, true)
	value, presence := p15DecodeOptionalGraph(g, chosen, "has_value", "value")
	resultGroup := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(value)})
	g.boundary(resultGroup, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	returned := g.boundary(resultGroup, 2, true)
	positions["return"] = len(g.expected)
	g.output(returned)
	g.caller()
	argument := p15FunctionArgument{}
	if mode != "omitted" {
		kind, wire := p15ContainerFieldValue(map[string]any{})
		carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", mode == "empty", p15FunctionArgument{true, int(kind), wire})
		argument = p15FunctionArgument{true, 0, p15FunctionEdge(carrier)}
	}
	positions["payload"] = len(g.expected)
	result := g.call(g.payload("echo", argument))
	unboxed, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(unboxed)})
	return positions, g.finish()
}

func TestP15T01OptionalMapDefaultGraphs(t *testing.T) {
	for _, tc := range p15OptionalMapDefaultCases() {
		for _, mode := range []string{"omitted", "none", "empty"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				source := p15OptionalMapDefaultSource(tc, mode)
				p15FunctionTypedProgram(t, source, map[string]string{})
				actions := p15CompiledActions(t, source)
				p15FunctionBoundaryRetain(t, "optional-map-default-"+tc.name+"-"+mode, actions)
				if _, err := p15OptionalMapDefaultGraph(actions, tc, mode); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15T01OptionalMapDefaultCorruptions(t *testing.T) {
	for _, tc := range p15OptionalMapDefaultCases() {
		for _, mode := range []string{"omitted", "none", "empty"} {
			t.Run(tc.name+"/"+mode, func(t *testing.T) {
				original := p15CompiledActions(t, p15OptionalMapDefaultSource(tc, mode))
				positions, err := p15OptionalMapDefaultGraph(original, tc, mode)
				if err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"default_absent", "default_kind", "default_value", "default_null", "truthiness", "wrong_slot", "wrong_join", "wrong_supplied_flag", "wrong_return"} {
					t.Run(fault, func(t *testing.T) {
						bad := p15C01Clone(t, original)
						switch fault {
						case "default_absent":
							p15FunctionPayloadRow(t, bad, positions["default"], "has_value")["WFValue"] = p15FunctionText("0")
						case "default_kind":
							p15FunctionPayloadRow(t, bad, positions["default"], "value")["WFItemType"] = 0
						case "default_value":
							p15FunctionPayloadRow(t, bad, positions["default"], "value")["WFValue"] = p15FunctionText("REPLACED")
						case "default_null":
							p15FunctionPayloadRow(t, bad, positions["default"], "value")["WFValue"] = nil
						case "truthiness":
							p15C01Params(bad[positions["guard"]])["WFCondition"] = 100
						case "wrong_slot":
							p15C01Params(bad[positions["value"]])["WFDictionaryKey"] = "has_arg_0"
						case "wrong_join":
							p15C01Params(bad[positions["join"]])["GroupingIdentifier"] = p15C01Params(bad[7])["GroupingIdentifier"]
						case "wrong_supplied_flag":
							flag := "0"
							if mode == "omitted" {
								flag = "1"
							}
							p15FunctionPayloadRow(t, bad, positions["payload"], "has_arg_0")["WFValue"] = p15FunctionText(flag)
						case "wrong_return":
							p15C01Params(bad[positions["return"]])["WFOutput"] = p15FunctionText("FORGED")
						default:
							t.Fatal(fmt.Sprintf("unknown fault %s", fault))
						}
						if _, err := p15OptionalMapDefaultGraph(bad, tc, mode); err == nil {
							t.Fatal("Optional default corruption accepted")
						}
					})
				}
			})
		}
	}
}
