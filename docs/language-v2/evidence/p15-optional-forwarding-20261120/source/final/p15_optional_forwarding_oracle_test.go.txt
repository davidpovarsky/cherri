package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15ForwardCarrierGraph(g *p15FunctionGraph, marker string, value p15FunctionArgument, presence any) string {
	if flag, known := presence.(string); known {
		return p15OptionalCarrierGraph(g, "has_value", "value", marker, flag == "1", value)
	}
	group := g.conditional(4, presence, "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", marker, true, value)
	g.boundary(group, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", marker, false, p15FunctionArgument{})
	return g.boundary(group, 2, true)
}

func p15ForwardCallGraph(g *p15FunctionGraph, name string, value p15FunctionArgument, presence any, before, after []p15FunctionArgument) (string, string) {
	carrier := p15ForwardCarrierGraph(g, "_cherri_optional_argument_v1", value, presence)
	args := append([]p15FunctionArgument(nil), before...)
	args = append(args, p15FunctionArgument{true, 0, p15FunctionEdge(carrier)})
	args = append(args, after...)
	result := g.call(g.payload(name, args...))
	return p15DecodeOptionalGraph(g, result, "has_value", "value")
}

func p15ForwardResultGraph(g *p15FunctionGraph, value, presence string) {
	carrier := p15ForwardCarrierGraph(g, "_cherri_optional_result_v1", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence))
	g.output(carrier)
}

func p15ForwardParameterGraph(g *p15FunctionGraph, envelope string, slot int, fallback bool) (value, presence string) {
	if !fallback {
		return p15DecodeOptionalArgumentGraph(g, envelope, slot)
	}
	supplied := g.get(fmt.Sprintf("has_arg_%d", slot), envelope)
	group := g.conditional(4, p15FunctionEdge(supplied), "WFConditionalActionString", "1")
	g.get(fmt.Sprintf("arg_%d", slot), envelope)
	g.boundary(group, 1, false)
	producer := g.value("gettext", map[string]any{"WFTextActionText": "DEFAULT"})
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(producer)})
	carrier := g.boundary(group, 2, true)
	return p15DecodeOptionalGraph(g, carrier, "has_value", "value")
}

// Every operation is independently authored. Actual UUIDs are alpha names;
// complete ordering, parameter shape, control joins and edges must match.
func p15OptionalForwardingGraphError(actions []map[string]any, name string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	first := "echo"
	if name == "later_assignment" || name == "implicit_input" {
		first = "pick"
	}
	envelope := g.header(first)
	value, presence := p15ForwardParameterGraph(g, envelope, 0, name == "returned_none_default")
	if first == "pick" {
		g.get("arg_1", envelope)
	}
	p15ForwardResultGraph(g, value, presence)
	second := ""
	if strings.HasPrefix(name, "parameter_") || name == "named_parameter" {
		second = "relay"
	}
	if name == "former_function_forward" {
		second = "forward"
	}
	if name == "former_function_result" {
		second = "absent"
	}
	if second != "" {
		g.nextFunction(second)
		if second == "absent" {
			g.output(p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{}))
		} else {
			slot := 0
			if name == "named_parameter" {
				g.get("arg_0", envelope)
				slot = 1
			}
			value, presence = p15DecodeOptionalArgumentGraph(g, envelope, slot)
			value, presence = p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence), nil, nil)
			p15ForwardResultGraph(g, value, presence)
		}
	}
	g.caller()
	var output string
	switch {
	case name == "immutable_none" || name == "inferred_none" || name == "former_function_bound":
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
		output, _ = p15ForwardCallGraph(g, "echo", p15FunctionArgument{}, "0", nil, nil)
	case name == "immutable_alias" || name == "inferred_alias":
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "initial"})
		g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("initial")})
		output, _ = p15ForwardCallGraph(g, "echo", p15FunctionArgument{}, "0", nil, nil)
	case name == "mutable_none" || name == "former_function_mutable":
		flag := p15MutableOptionalAssignGraph(g, "", false, p15MutableOptionalPresentValues()[0])
		output, _ = p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionVariable("candidate")}, p15FunctionVariable(flag), nil, nil)
	case name == "later_assignment":
		flag := p15MutableOptionalAssignGraph(g, "", false, p15MutableOptionalPresentValues()[0])
		oldPresence := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(flag)})
		oldValue := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("candidate")})
		condition := g.value("number", map[string]any{"WFNumberActionNumber": float64(1)})
		group := g.conditional(4, p15FunctionEdge(condition), "WFNumberValue", float64(1))
		p15MutableOptionalAssignGraph(g, flag, true, p15MutableOptionalPresentValues()[0])
		g.value("gettext", map[string]any{"WFTextActionText": "WRITTEN"})
		g.boundary(group, 1, false)
		g.value("gettext", map[string]any{"WFTextActionText": "UNUSED"})
		later := g.boundary(group, 2, true)
		output, _ = p15ForwardCallGraph(g, "pick", p15FunctionArgument{true, 0, p15FunctionEdge(oldValue)}, p15FunctionEdge(oldPresence), nil, []p15FunctionArgument{{true, 0, p15FunctionEdge(later)}})
	case name == "implicit_input":
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "supplied"})
		g.emit("gettext", map[string]any{"WFTextActionText": "INPUT"})
		late := g.value("hash", map[string]any{"WFHashType": "SHA256"})
		output, _ = p15ForwardCallGraph(g, "pick", p15FunctionArgument{}, "0", nil, []p15FunctionArgument{{true, 0, p15FunctionEdge(late)}})
	case name == "former_function_result":
		call := g.call(g.payload("absent"))
		value, presence = p15DecodeOptionalGraph(g, call, "has_value", "value")
		output, _ = p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence), nil, nil)
	case second != "":
		var before []p15FunctionArgument
		if name == "named_parameter" {
			before = []p15FunctionArgument{p15FunctionString("SEED")}
		}
		flag, argument := "0", p15FunctionArgument{}
		if name == "parameter_empty" || name == "former_function_forward" {
			flag, argument = "1", p15FunctionString("")
		}
		output, _ = p15ForwardCallGraph(g, second, argument, flag, before, nil)
	case strings.HasPrefix(name, "returned_"):
		key := strings.TrimPrefix(name, "returned_")
		if key == "none_default" {
			key = "none"
		}
		found := false
		for _, tc := range p15FunctionOptionalCases()[:6] {
			if tc.name != key {
				continue
			}
			found = true
			flag := "0"
			if tc.valuePresent {
				flag = "1"
			}
			value, presence = p15ForwardCallGraph(g, "echo", tc.value, flag, nil, nil)
			output, _ = p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence), nil, nil)
		}
		if !found {
			return fmt.Errorf("unmodeled Optional forwarding value %q", key)
		}
	default:
		return fmt.Errorf("unmodeled Optional forwarding scenario %q", name)
	}
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(output)})
	return g.finish()
}

func TestP15OptionalForwardingCompleteNativeGraph(t *testing.T) {
	for _, tc := range p15OptionalForwardingCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15OptionalForwardingGraphError(p15CompiledActions(t, tc.source), tc.name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15MutateOptionalForwardingGraph(actions []map[string]any, fault string) ([]map[string]any, bool) {
	for i, action := range actions {
		p := p15Param(action)
		kind, _ := action["WFWorkflowActionIdentifier"].(string)
		if kind == "is.workflow.actions.dictionary" {
			fields, err := p15ContainerFields(p["WFItems"])
			if err != nil {
				continue
			}
			if fields["_cherri_optional_argument_v1"] != nil {
				switch fault {
				case "carrier_marker":
					fields["_cherri_optional_argument_v1"]["WFValue"] = p15FunctionText("0")
					return actions, true
				case "value_flag":
					wrong := "1"
					if fields["value"] != nil {
						wrong = "0"
					}
					fields["has_value"]["WFValue"] = p15FunctionText(wrong)
					return actions, true
				case "flag_type":
					fields["has_value"]["WFItemType"] = uint64(4)
					return actions, true
				case "empty_text_alias":
					row := fields["value"]
					if row == nil {
						row = fields["has_value"]
						row["WFKey"] = p15FunctionText("value")
					}
					row["WFValue"] = p15FunctionText("WRONG")
					return actions, true
				}
			}
			if fields["_cherri_magic"] != nil && fields["has_arg_0"] != nil {
				switch fault {
				case "argument_omission":
					fields["has_arg_0"]["WFValue"] = p15FunctionText("0")
					return actions, true
				case "carrier_edge":
					fields["arg_0"]["WFValue"] = p15FunctionEdge("missing-carrier")
					return actions, true
				}
			}
		}
		if p["WFDictionaryKey"] == "has_value" {
			switch fault {
			case "decode_value_as_presence":
				p["WFDictionaryKey"] = "value"
				return actions, true
			case "truthiness":
				if i+1 >= len(actions) {
					return actions, false
				}
				p15Param(actions[i+1])["WFCondition"] = uint64(100)
				return actions, true
			}
		}
		if kind == "is.workflow.actions.nothing" && fault == "none_as_text" {
			action["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
			action["WFWorkflowActionParameters"] = map[string]any{"WFTextActionText": ""}
			return actions, true
		}
		if kind == "is.workflow.actions.conditional" && p["UUID"] != nil && fault == "wrong_join" {
			p["GroupingIdentifier"] = "wrong-join"
			return actions, true
		}
		if kind == "is.workflow.actions.runworkflow" {
			switch fault {
			case "call_input":
				p["WFInput"] = p15FunctionEdge("missing-payload")
				return actions, true
			case "duplicate_call":
				return append(append(actions[:i+1:i+1], action), actions[i+1:]...), true
			}
		}
	}
	return actions, false
}

func TestP15OptionalForwardingOracleRejectsCorruption(t *testing.T) {
	for _, tc := range p15OptionalForwardingCases() {
		switch tc.name {
		case "returned_none", "returned_empty_text", "later_assignment", "implicit_input", "named_parameter", "returned_none_default":
		default:
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15OptionalForwardingGraphError(actions, tc.name); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"carrier_marker", "value_flag", "flag_type", "empty_text_alias", "argument_omission", "carrier_edge", "decode_value_as_presence", "truthiness", "none_as_text", "wrong_join", "call_input", "duplicate_call"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15OptionalForwardingGraphError(bad, tc.name); err != nil {
						t.Fatal("unchanged clone must pass", err)
					}
					bad, changed := p15MutateOptionalForwardingGraph(bad, fault)
					if !changed {
						t.Fatal("missing precise corruption target")
					}
					if p15OptionalForwardingGraphError(bad, tc.name) == nil {
						t.Fatal("corrupted Optional carrier graph accepted")
					}
				})
			}
		})
	}
}

func TestP15OptionalForwardingSourceOrderOracle(t *testing.T) {
	for _, tc := range p15OptionalForwardingCases() {
		if tc.name != "later_assignment" && tc.name != "implicit_input" && tc.name != "returned_none_default" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15OptionalForwardingGraphError(actions, tc.name); err != nil {
				t.Fatal(err)
			}
			bad := p15ContainerCloneNative(t, actions)
			if err := p15OptionalForwardingGraphError(bad, tc.name); err != nil {
				t.Fatal("unchanged clone must pass", err)
			}
			changed := false
			for i, action := range bad {
				p := p15Param(action)
				if tc.name == "later_assignment" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" {
					wire, ok := p["WFVariable"].(map[string]any)
					if !ok {
						continue
					}
					descriptor, ok := wire["Value"].(map[string]any)
					if !ok {
						continue
					}
					name, _ := descriptor["VariableName"].(string)
					if strings.HasPrefix(name, "_cherri_optional_presence_") {
						// The independent presence snapshot cannot read the data.
						p["WFVariable"] = p15FunctionVariable("candidate")
						changed = true
						break
					}
				}
				if tc.name == "implicit_input" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.hash" && i+1 < len(bad) {
					// Moving the pure carrier before this opaque authored action
					// changes its implicit input even if the final payload is equal.
					bad[i], bad[i+1] = bad[i+1], bad[i]
					changed = true
					break
				}
				if tc.name == "returned_none_default" && p["WFTextActionText"] == "DEFAULT" {
					for j := 0; j < i; j++ {
						if p15Param(bad[j])["WFDictionaryKey"] == "has_arg_0" {
							moved := bad[i]
							bad = append(bad[:i:i], bad[i+1:]...)
							bad = append(bad[:j+1:j+1], append([]map[string]any{moved}, bad[j+1:]...)...)
							changed = true
							break
						}
					}
					break
				}
			}
			if !changed {
				t.Fatal("missing exact order mutation target")
			}
			if p15OptionalForwardingGraphError(bad, tc.name) == nil {
				t.Fatal("wrong snapshot/effect/implicit-input order accepted")
			}
		})
	}
}
