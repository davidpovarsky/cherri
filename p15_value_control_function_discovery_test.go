package main

import (
	"fmt"
	"strings"
	"testing"
)

// Every fixture has an explicit main return. The oracle below models the
// complete function dispatcher, caller branch, authored control and output;
// it does not execute Apple's runtime or claim collected Optional fidelity.
type p15ValueControlFunctionCase struct {
	name, control, resultType, functionValue, main string
}

func p15ValueControlFunctionCases() []p15ValueControlFunctionCase {
	return []p15ValueControlFunctionCase{
		{"direct_number", "direct", "Number", "7", "return target()"},
		{"direct_bool", "direct", "Bool", "true", "return target()"},
		{"direct_text", "direct", "Text", `"Pick"`, "return target()"},
		{"direct_list", "direct", "List<Text>", `["row"]`, "return target()"},
		{"direct_optional", "direct", "Text?", "none", "return target()"},
		{"if_value", "if_value", "Number", "7", "let value=if true {yield target()} else {yield 9}\nreturn value"},
		{"for_value", "for_value", "Number", "7", "let value=for item in [\"row\"] {yield target()}\nreturn value"},
		{"repeat_value", "repeat_value", "Number", "7", "let value=repeat 2 {yield target()}\nreturn value"},
		{"menu_value", "menu_value", "Number", "7", "let value=menu(\"Pick\") {case \"A\" {yield target()} case \"B\" {yield 9}}\nreturn value"},
		{"if_condition", "if_condition", "Bool", "true", "let value=if target() {yield 7} else {yield 9}\nreturn value"},
		{"repeat_count", "repeat_count", "Number", "2", "let value=repeat target() {yield 7}\nreturn value"},
		{"for_iterable", "for_iterable", "List<Text>", `["row"]`, "let value=for item in target() {yield 7}\nreturn value"},
		{"menu_prompt", "menu_prompt", "Text", `"Pick"`, "let value=menu(target()) {case \"A\" {yield 7} case \"B\" {yield 9}}\nreturn value"},
		{"optional_repeat", "repeat_value", "Text?", "none", "let value=repeat 2 {yield target()}\nreturn value"},
		{"nested_value", "nested_value", "Number", "7", "let value=if true {yield repeat 2 {yield target()}} else {yield repeat 1 {yield 9}}\nreturn value"},
		{"statement_repeat", "statement_repeat", "Number", "7", "repeat 2 {target()}\nreturn 9"},
		{"unused_function", "unused", "Number", "7", "return 9"},
	}
}

func (tc p15ValueControlFunctionCase) source() string {
	return "function target()->" + tc.resultType + "{return " + tc.functionValue + "}\n" + tc.main
}

func p15DiscoveryGroup(g *p15FunctionGraph) string {
	index := len(g.expected)
	group := ""
	if index < len(g.actual) {
		group, _ = p15C01Params(g.actual[index])["GroupingIdentifier"].(string)
	}
	if group == "" || g.groups[group] {
		g.err = fmt.Errorf("missing or reused authored control identity at %d", index)
	}
	g.groups[group] = true
	return group
}

func p15DiscoveryLoop(g *p15FunctionGraph, kind, field string, input any) string {
	group := p15DiscoveryGroup(g)
	g.emit(kind, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0, field: input})
	return group
}

func p15DiscoveryLoopEnd(g *p15FunctionGraph, kind, group string, result bool) string {
	p := map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2}
	if result {
		return g.value(kind, p)
	}
	return g.emit(kind, p)
}

func p15DiscoveryMenu(g *p15FunctionGraph, prompt any) string {
	group := p15DiscoveryGroup(g)
	g.emit("choosefrommenu", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0,
		"WFMenuPrompt": prompt, "WFMenuItems": []any{
			map[string]any{"WFItemType": 0, "WFValue": "A"},
			map[string]any{"WFItemType": 0, "WFValue": "B"},
		}})
	return group
}

func p15DiscoveryLiteral(g *p15FunctionGraph, tc p15ValueControlFunctionCase) string {
	switch tc.resultType {
	case "Number":
		number := 7
		if tc.functionValue == "2" {
			number = 2
		}
		return g.value("number", map[string]any{"WFNumberActionNumber": number})
	case "Bool":
		return g.value("number", map[string]any{"WFNumberActionNumber": 1})
	case "Text":
		return g.value("gettext", map[string]any{"WFTextActionText": "Pick"})
	case "List<Text>":
		return g.value("list", map[string]any{"WFItems": []any{map[string]any{"WFItemType": 0, "WFValue": "row"}}})
	case "Text?":
		return p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	default:
		panic("unmodeled discovery function type")
	}
}

func p15DiscoveryCall(g *p15FunctionGraph, tc p15ValueControlFunctionCase) string {
	result := g.call(g.payload("target"))
	if tc.resultType == "Number" {
		return g.numberCast(result)
	}
	if tc.resultType == "Text?" {
		value, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
		return value
	}
	return result
}

func p15ValueControlFunctionGraphError(actions []map[string]any, tc p15ValueControlFunctionCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	number := func(value int) string { return g.value("number", map[string]any{"WFNumberActionNumber": value}) }
	if tc.control == "unused" {
		g.output(number(9))
		return g.compare()
	}
	g.header("target")
	g.output(p15DiscoveryLiteral(g, tc))
	g.caller()
	var result string
	switch tc.control {
	case "direct":
		result = p15DiscoveryCall(g, tc)
	case "if_value", "if_condition", "nested_value":
		condition := ""
		if tc.control == "if_condition" {
			condition = p15DiscoveryCall(g, tc)
		} else {
			condition = number(1)
		}
		group := g.conditional(4, p15FunctionEdge(condition), "WFNumberValue", 1)
		switch tc.control {
		case "if_value":
			p15DiscoveryCall(g, tc)
		case "if_condition":
			number(7)
		case "nested_value":
			loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 2)
			p15DiscoveryCall(g, tc)
			p15DiscoveryLoopEnd(g, "repeat.count", loop, true)
		}
		g.boundary(group, 1, false)
		if tc.control == "nested_value" {
			loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", 1)
			number(9)
			p15DiscoveryLoopEnd(g, "repeat.count", loop, true)
		} else {
			number(9)
		}
		result = g.boundary(group, 2, true)
	case "repeat_value", "repeat_count", "statement_repeat":
		var count any = 2
		if tc.control == "repeat_count" {
			count = p15FunctionEdge(p15DiscoveryCall(g, tc))
		}
		loop := p15DiscoveryLoop(g, "repeat.count", "WFRepeatCount", count)
		if tc.control == "repeat_count" {
			number(7)
		} else {
			p15DiscoveryCall(g, tc)
		}
		result = p15DiscoveryLoopEnd(g, "repeat.count", loop, tc.control != "statement_repeat")
		if tc.control == "statement_repeat" {
			result = number(9)
		}
	case "for_value", "for_iterable":
		var items string
		if tc.control == "for_iterable" {
			items = p15DiscoveryCall(g, tc)
		} else {
			items = g.value("list", map[string]any{"WFItems": []any{map[string]any{"WFItemType": 0, "WFValue": "row"}}})
		}
		loop := p15DiscoveryLoop(g, "repeat.each", "WFInput", p15FunctionEdge(items))
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionVariable("Repeat Item")})
		if tc.control == "for_iterable" {
			number(7)
		} else {
			p15DiscoveryCall(g, tc)
		}
		result = p15DiscoveryLoopEnd(g, "repeat.each", loop, true)
	case "menu_value", "menu_prompt":
		var prompt any = "Pick"
		if tc.control == "menu_prompt" {
			prompt = p15FunctionEdge(p15DiscoveryCall(g, tc))
		}
		menu := p15DiscoveryMenu(g, prompt)
		p15OptionalJoinMenuCase(g, menu, "A")
		if tc.control == "menu_prompt" {
			number(7)
		} else {
			p15DiscoveryCall(g, tc)
		}
		p15OptionalJoinMenuCase(g, menu, "B")
		number(9)
		result = p15OptionalJoinMenuEnd(g, menu, true)
	default:
		return fmt.Errorf("unmodeled discovery scenario %q", tc.control)
	}
	g.output(result)
	return g.finishExplicitMainReturn()
}

func TestP15ValueControlFunctionDiscoveryCompleteGraph(t *testing.T) {
	for _, tc := range p15ValueControlFunctionCases() {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ValueControlFunctionGraphError(p15CompiledActions(t, tc.source()), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15ValueControlFunctionDiscoveryRejectsCorruption(t *testing.T) {
	for _, tc := range p15ValueControlFunctionCases() {
		if tc.control == "unused" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			original := p15CompiledActions(t, tc.source())
			if err := p15ValueControlFunctionGraphError(original, tc); err != nil {
				t.Fatal("original complete graph is not valid", err)
			}
			for _, fault := range []string{"missing_dispatcher", "wrong_dispatch_name", "wrong_magic", "wrong_self_target", "wrong_payload_edge", "wrong_caller_branch", "wrong_result_edge", "extra_action"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, original)
					if err := p15ValueControlFunctionGraphError(bad, tc); err != nil {
						t.Fatal("unchanged copy rejected", err)
					}
					call, caller, body, magic, output := -1, -1, -1, -1, -1
					for i, action := range bad {
						kind, _ := action["WFWorkflowActionIdentifier"].(string)
						p := p15C01Params(action)
						if strings.HasSuffix(kind, ".runworkflow") {
							call = i
						}
						if strings.HasSuffix(kind, ".conditional") {
							if p["WFConditionalActionString"] == "target" {
								body = i
							}
							if p["WFConditionalActionString"] == "__cherri_fn_envelope_v2__" {
								magic = i
							}
							if i > 0 && bad[i-1]["WFWorkflowActionIdentifier"] == "is.workflow.actions.exit" {
								caller = i
							}
						}
						if strings.HasSuffix(kind, ".output") {
							output = i
						}
					}
					if call < 0 || caller < 0 || body < 0 || magic < 0 || output < 0 {
						t.Fatal("mutation target is absent")
					}
					switch fault {
					case "missing_dispatcher":
						bad = bad[caller+1 : len(bad)-1]
					case "wrong_dispatch_name":
						p15C01Params(bad[body])["WFConditionalActionString"] = "foreign"
					case "wrong_magic":
						p15C01Params(bad[magic])["WFConditionalActionString"] = "foreign-envelope"
					case "wrong_self_target":
						p15C01Params(bad[call])["WFWorkflow"] = map[string]any{"isSelf": false, "workflowName": "foreign"}
					case "wrong_payload_edge":
						p15C01Params(bad[call])["WFInput"] = p15FunctionEdge(p15C01Params(bad[0])["UUID"].(string))
					case "wrong_caller_branch":
						p15C01Params(bad[caller])["WFControlFlowMode"] = 0
					case "wrong_result_edge":
						p15C01Params(bad[output])["WFOutput"] = p15FunctionText("\ufffc", p15C01Params(bad[0])["UUID"].(string))
					case "extra_action":
						bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					}
					if err := p15ValueControlFunctionGraphError(bad, tc); err == nil {
						t.Fatal("complete native oracle accepted", fault)
					}
				})
			}
		})
	}
}
