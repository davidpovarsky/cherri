package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15OptionalJoinValue(name string) p15FunctionArgument {
	switch name {
	case "empty_text":
		return p15FunctionString("")
	case "false":
		return p15FunctionArgument{true, 4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": false}}
	case "zero":
		return p15FunctionNumber("0")
	case "empty_list":
		return p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}}
	case "empty_map":
		return p15FunctionArgument{true, 1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}}}
	}
	panic("unmodeled Optional source value: " + name)
}

func p15OptionalJoinFlagGraph(g *p15FunctionGraph) {
	value := g.value("number", map[string]any{"WFNumberActionNumber": 1})
	g.emit("setvariable", map[string]any{"WFVariableName": "flag", "WFInput": p15FunctionEdge(value)})
}

func p15OptionalJoinMenuStart(g *p15FunctionGraph) string {
	group := ""
	if len(g.expected) < len(g.actual) {
		group, _ = p15C01Params(g.actual[len(g.expected)])["GroupingIdentifier"].(string)
	}
	if group == "" || g.groups[group] {
		g.err = fmt.Errorf("missing or reused menu control identity")
	}
	g.groups[group] = true
	g.emit("choosefrommenu", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0,
		"WFMenuPrompt": "Pick", "WFMenuItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "A"},
			map[string]any{"WFItemType": 0, "WFValue": "B"},
		}})
	return group
}

func p15OptionalJoinMenuCase(g *p15FunctionGraph, group, name string) {
	g.emit("choosefrommenu", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 1,
		"WFMenuItemTitle": name, "WFMenuItemAttributedTitle": name})
}

func p15OptionalJoinMenuEnd(g *p15FunctionGraph, group string, result bool) string {
	p := map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2}
	if result {
		return g.value("choosefrommenu", p)
	}
	return g.emit("choosefrommenu", p)
}

func p15OptionalJoinCarrierGraph(g *p15FunctionGraph, present bool, value p15FunctionArgument) string {
	return p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_join_v1", present, value)
}

func p15OptionalJoinDynamicCarrier(g *p15FunctionGraph, marker string, value p15FunctionArgument, presence any) string {
	group := g.conditional(4, presence, "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", marker, true, value)
	g.boundary(group, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", marker, false, p15FunctionArgument{})
	return g.boundary(group, 2, true)
}

// The control source and branch values specify every operation. Native UUIDs
// are bound only as alpha identities at the required producer positions.
func p15OptionalJoinControlGraph(g *p15FunctionGraph, control string, input any, value p15FunctionArgument, presentFirst bool) (decoded, presence string) {
	var joined string
	if control == "if" {
		group := g.conditional(4, input, "WFNumberValue", 1)
		p15OptionalJoinCarrierGraph(g, presentFirst, value)
		g.boundary(group, 1, false)
		p15OptionalJoinCarrierGraph(g, !presentFirst, value)
		joined = g.boundary(group, 2, true)
	} else {
		group := p15OptionalJoinMenuStart(g)
		p15OptionalJoinMenuCase(g, group, "A")
		p15OptionalJoinCarrierGraph(g, presentFirst, value)
		p15OptionalJoinMenuCase(g, group, "B")
		p15OptionalJoinCarrierGraph(g, !presentFirst, value)
		joined = p15OptionalJoinMenuEnd(g, group, true)
	}
	return p15DecodeOptionalGraph(g, joined, "has_value", "value")
}

func p15OptionalJoinNonePredicateGraph(g *p15FunctionGraph, presence any) string {
	group := g.conditional(4, presence, "WFConditionalActionString", "0")
	g.emit("number", map[string]any{"WFNumberActionNumber": 1})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": 0})
	return g.boundary(group, 2, true)
}

func p15OptionalJoinCompleteGraphError(actions []map[string]any, tc p15OptionalValueJoinCase) error {
	parts := strings.Split(tc.name, "/")
	if len(parts) != 3 {
		return fmt.Errorf("unknown join source case %q", tc.name)
	}
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	p15OptionalJoinFlagGraph(g)
	value, presence := p15OptionalJoinControlGraph(g, parts[0], p15FunctionVariable("flag"), p15OptionalJoinValue(parts[1]), parts[2] == "present_first_true")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	absent := p15OptionalJoinNonePredicateGraph(g, p15FunctionEdge(presence))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(absent)})
	g.emit("nothing", nil)
	return g.compare()
}

func TestP15OptionalValueJoinsCompleteNativeGraphs(t *testing.T) {
	for _, tc := range p15OptionalValueJoinCases() {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15OptionalJoinCompleteGraphError(actions, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalValueJoinsGraphRejectsCorruption(t *testing.T) {
	for _, tc := range p15OptionalValueJoinCases() {
		t.Run(tc.name, func(t *testing.T) {
			original := p15CompiledActions(t, tc.source)
			if err := p15OptionalJoinCompleteGraphError(original, tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"swap_branch_values", "wrong_flag", "wrong_flag_type", "wrong_present_type", "wrong_present_value",
				"branch_instead_of_join", "value_instead_of_presence", "presence_instead_of_value", "truthiness_decode", "truthiness_predicate",
				"missing_clear", "wrong_end", "missing_operation", "extra_operation", "forward_value_edge", "duplicate_producer"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					if err := p15OptionalJoinCompleteGraphError(bad, tc); err != nil {
						t.Fatal("unchanged copy rejected", err)
					}
					dicts := []int{}
					decoder, valueRead, clear := -1, -1, -1
					var presentFields map[string]map[string]any
					for index, action := range bad {
						p := p15C01Params(action)
						switch action["WFWorkflowActionIdentifier"] {
						case "is.workflow.actions.dictionary":
							fields, err := p15ContainerFields(p["WFItems"])
							if err == nil && fields["_cherri_optional_join_v1"] != nil {
								dicts = append(dicts, index)
								if fields["value"] != nil {
									presentFields = fields
								}
							}
						case "is.workflow.actions.getvalueforkey":
							if p["WFDictionaryKey"] == "has_value" {
								decoder = index
							} else if p["WFDictionaryKey"] == "value" {
								valueRead = index
							}
						case "is.workflow.actions.nothing":
							if decoder >= 0 && clear < 0 {
								clear = index
							}
						}
					}
					if len(dicts) != 2 || presentFields == nil || decoder < 0 || valueRead < 0 || clear < 0 {
						t.Fatal("exact mutation targets absent")
					}
					predicate := len(bad) - 7
					if predicate < 0 || bad[predicate]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || p15Int(p15C01Params(bad[predicate])["WFControlFlowMode"]) != 0 {
						t.Fatal("exact source presence predicate target absent")
					}
					switch fault {
					case "swap_branch_values":
						left, right := p15C01Params(bad[dicts[0]]), p15C01Params(bad[dicts[1]])
						left["WFItems"], right["WFItems"] = right["WFItems"], left["WFItems"]
						// The initial marker-only boundary cannot detect this. The
						// complete source-owned branch oracle must reject it.
						if err := p15OptionalJoinPresenceBoundary(bad); err != nil {
							t.Fatal("branch-swap control should retain both marker flags", err)
						}
					case "wrong_flag":
						presentFields["has_value"]["WFValue"] = p15FunctionText("0")
					case "wrong_flag_type":
						presentFields["has_value"]["WFItemType"] = uint64(4)
					case "wrong_present_type":
						presentFields["value"]["WFItemType"] = uint64(77)
					case "wrong_present_value":
						presentFields["value"]["WFValue"] = p15FunctionText("CORRUPTED")
					case "branch_instead_of_join":
						p15C01Params(bad[decoder])["WFInput"] = p15FunctionEdge(p15C01Params(bad[dicts[0]])["UUID"].(string))
					case "value_instead_of_presence":
						p15C01Params(bad[decoder])["WFDictionaryKey"] = "value"
					case "presence_instead_of_value":
						p15C01Params(bad[valueRead])["WFDictionaryKey"] = "has_value"
					case "truthiness_decode":
						p15C01Params(bad[decoder+1])["WFCondition"] = uint64(100)
					case "truthiness_predicate":
						p15C01Params(bad[predicate])["WFCondition"] = uint64(100)
					case "missing_clear":
						bad[clear]["WFWorkflowActionIdentifier"] = "is.workflow.actions.comment"
					case "wrong_end":
						p15C01Params(bad[decoder-1])["GroupingIdentifier"] = "not-the-source-control"
					case "missing_operation":
						bad = append(bad[:clear], bad[clear+1:]...)
					case "extra_operation":
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "forward_value_edge":
						p15C01Params(bad[valueRead])["WFInput"] = p15FunctionEdge(p15C01Params(bad[len(bad)-3])["UUID"].(string))
					case "duplicate_producer":
						p15C01Params(bad[dicts[1]])["UUID"] = p15C01Params(bad[dicts[0]])["UUID"]
					}
					if p15OptionalJoinCompleteGraphError(bad, tc) == nil {
						t.Fatal("damaged complete join graph accepted")
					}
				})
			}
		})
	}
}
