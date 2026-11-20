package main

import (
	"fmt"
	"testing"
)

// These graphs independently specify all actions and edges of the source
// scenarios. They inspect native serialization, not Apple's runtime behavior.
func p15OptionalIndexListGraph(g *p15FunctionGraph, name string) string {
	values := []any{}
	switch name {
	case "text", "integral_decimal", "integral_exponent":
		values = []any{"ready"}
	case "empty_text":
		values = []any{""}
	case "false":
		values = []any{false}
	case "zero":
		value := g.value("number", map[string]any{"WFNumberActionNumber": 0})
		values = []any{p15FunctionEdge(value)}
	case "empty_map", "map":
		rows := []any{}
		if name == "map" {
			rows = append(rows, map[string]any{"WFKey": p15FunctionText("key"), "WFItemType": 0, "WFValue": p15FunctionText("VALUE")})
		}
		dictionary := g.value("dictionary", map[string]any{"WFItems": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": rows}}})
		values = []any{p15FunctionEdge(dictionary)}
	case "second_slot":
		values = []any{"first", "second"}
	case "capture":
		before := g.value("gettext", map[string]any{"WFTextActionText": "BEFORE"})
		g.emit("setvariable", map[string]any{"WFVariableName": "state", "WFInput": p15FunctionEdge(before)})
		early := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("state")})
		condition := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		group := g.conditional(4, p15FunctionEdge(condition), "WFNumberValue", 1)
		after := g.value("gettext", map[string]any{"WFTextActionText": "AFTER"})
		g.emit("setvariable", map[string]any{"WFVariableName": "state", "WFInput": p15FunctionEdge(after)})
		g.value("gettext", map[string]any{"WFTextActionText": "SECOND"})
		g.boundary(group, 1, false)
		g.value("gettext", map[string]any{"WFTextActionText": "UNUSED"})
		later := g.boundary(group, 2, true)
		values = []any{p15FunctionEdge(early), p15FunctionEdge(later)}
	default:
		g.err = fmt.Errorf("unmodeled static index source %q", name)
	}
	rows := []any{}
	for _, value := range values {
		rows = append(rows, map[string]any{"WFItemType": 0, "WFValue": value})
	}
	return g.value("list", map[string]any{"WFItems": rows})
}

func p15OptionalIndexSelectionGraph(g *p15FunctionGraph, name, list string) string {
	if name == "integral_decimal" || name == "integral_exponent" {
		count := g.value("count", map[string]any{"Input": p15FunctionEdge(list), "WFCountType": "Items"})
		group := g.conditional(3, p15FunctionEdge(count), "WFNumberValue", 1)
		g.value("getitemfromlist", map[string]any{"WFInput": p15FunctionEdge(list), "WFItemSpecifier": "Item At Index", "WFItemIndex": 1})
		g.boundary(group, 1, false)
		g.emit("nothing", nil)
		return g.boundary(group, 2, true)
	}
	index := 0
	if name == "second_slot" {
		index = 1
	}
	oneBased := g.value("math", map[string]any{"WFInput": index, "WFMathOperation": "+", "WFMathOperand": 1})
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(list), "WFCountType": "Items"})
	difference := g.math("-", count, p15FunctionEdge(oneBased))
	outer := g.conditional(3, p15FunctionEdge(oneBased), "WFNumberValue", 1)
	inner := g.conditional(3, p15FunctionEdge(difference), "WFNumberValue", 0)
	g.value("getitemfromlist", map[string]any{"WFInput": p15FunctionEdge(list), "WFItemSpecifier": "Item At Index", "WFItemIndex": p15FunctionEdge(oneBased)})
	g.boundary(inner, 1, false)
	g.emit("nothing", nil)
	g.boundary(inner, 2, true)
	g.boundary(outer, 1, false)
	g.emit("nothing", nil)
	return g.boundary(outer, 2, true)
}

func p15OptionalIndexGraphError(actions []map[string]any, name string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	value, presence := p15DecodeOptionalArgumentGraph(g, envelope, 0)
	p15ForwardResultGraph(g, value, presence)
	g.caller()
	list := p15OptionalIndexListGraph(g, name)
	selected := p15OptionalIndexSelectionGraph(g, name, list)
	output, _ := p15ForwardCallGraph(g, "echo", p15FunctionArgument{true, 0, p15FunctionEdge(selected)}, "1", nil, nil)
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(output)})
	return g.finish()
}

func p15OptionalIndexGraphSource(name string) string {
	if name == "capture" {
		return `function echo(value:Text?)->Text?{return value}
var state="BEFORE"
getVariable(echo([state,if true{state="AFTER";yield "SECOND"}else{yield "UNUSED"}][0]))`
	}
	for _, tc := range p15OptionalIndexCases() {
		if tc.name == name {
			return p15OptionalIndexSource(tc, "argument")
		}
	}
	panic("unknown index graph source")
}

func TestP15OptionalStaticIndexCompleteNativeGraph(t *testing.T) {
	names := []string{"capture"}
	for _, tc := range p15OptionalIndexCases() {
		names = append(names, tc.name)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			if err := p15OptionalIndexGraphError(p15CompiledActions(t, p15OptionalIndexGraphSource(name)), name); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15MutateOptionalIndexGraph(actions []map[string]any, fault string) ([]map[string]any, bool) {
	for i, action := range actions {
		p := p15Param(action)
		kind, _ := action["WFWorkflowActionIdentifier"].(string)
		if kind == "is.workflow.actions.list" {
			rows, ok := p["WFItems"].([]any)
			if !ok || len(rows) != 2 {
				continue
			}
			switch fault {
			case "missing_slot":
				p["WFItems"] = rows[:1]
				return actions, true
			case "reordered_slots":
				rows[0], rows[1] = rows[1], rows[0]
				return actions, true
			case "deferred_reference":
				rows[0].(map[string]any)["WFValue"] = p15FunctionVariable("state")
				return actions, true
			case "empty_instead_of_value":
				rows[0].(map[string]any)["WFValue"] = ""
				return actions, true
			case "coerced_row":
				rows[0].(map[string]any)["WFItemType"] = 3
				return actions, true
			}
		}
		if kind == "is.workflow.actions.getvariable" && p["UUID"] != nil {
			if fault == "missing_early_capture" {
				return append(actions[:i], actions[i+1:]...), true
			}
		}
		if kind == "is.workflow.actions.math" && p["WFMathOperation"] == "+" && fault == "wrong_index" {
			p["WFInput"] = 1
			return actions, true
		}
		if kind == "is.workflow.actions.count" && fault == "wrong_count_source" {
			p["Input"] = p15FunctionVariable("state")
			return actions, true
		}
		if kind == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0 && p15Int(p["WFCondition"]) == 3 {
			switch fault {
			case "missing_bound":
				return append(actions[:i], actions[i+1:]...), true
			case "wrong_bound":
				p["WFNumberValue"] = 9
				return actions, true
			case "truthiness_bound":
				p["WFCondition"] = 100
				delete(p, "WFNumberValue")
				return actions, true
			}
		}
		if kind == "is.workflow.actions.getitemfromlist" && fault == "wrong_selected_list" {
			p["WFInput"] = p15FunctionVariable("state")
			return actions, true
		}
		if kind == "is.workflow.actions.dictionary" {
			fields, err := p15ContainerFields(p["WFItems"])
			if err != nil || fields["_cherri_optional_argument_v1"] == nil {
				continue
			}
			switch fault {
			case "wrong_presence":
				fields["has_value"]["WFValue"] = p15FunctionText("0")
				return actions, true
			case "value_as_presence":
				fields["has_value"]["WFValue"] = fields["value"]["WFValue"]
				return actions, true
			case "missing_carrier":
				return append(actions[:i], actions[i+1:]...), true
			case "presence_number":
				fields["has_value"]["WFItemType"] = 3
				return actions, true
			case "missing_value":
				fields["value"]["WFKey"] = p15FunctionText("wrong")
				return actions, true
			}
		}
	}
	return actions, false
}

func TestP15OptionalStaticIndexNativeOracleRejectsCorruption(t *testing.T) {
	original := p15CompiledActions(t, p15OptionalIndexGraphSource("capture"))
	if err := p15OptionalIndexGraphError(original, "capture"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_slot", "reordered_slots", "deferred_reference", "empty_instead_of_value", "coerced_row", "missing_early_capture", "wrong_index", "wrong_count_source", "missing_bound", "wrong_bound", "truthiness_bound", "wrong_selected_list", "wrong_presence", "value_as_presence", "missing_carrier", "presence_number", "missing_value"} {
		t.Run(fault, func(t *testing.T) {
			clone := p15ContainerCloneNative(t, original)
			if err := p15OptionalIndexGraphError(clone, "capture"); err != nil {
				t.Fatal("unchanged clone rejected", err)
			}
			changed, applied := p15MutateOptionalIndexGraph(clone, fault)
			if !applied {
				t.Fatal("mutation not applied")
			}
			if err := p15OptionalIndexGraphError(changed, "capture"); err == nil {
				t.Fatal("native graph accepted corruption", fault)
			}
		})
	}
}
