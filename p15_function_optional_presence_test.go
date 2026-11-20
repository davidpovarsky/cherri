package main

import (
	"fmt"
	"sort"
	"testing"
)

// This models the complete native graph, not an Apple runtime. Argument
// omission/default selection and the value's own presence are independent.
type p15FunctionOptionalCase struct {
	name, typ, argument, fallback string
	supplied, valuePresent, named bool
	value                         p15FunctionArgument
}

func p15FunctionOptionalCases() []p15FunctionOptionalCase {
	return []p15FunctionOptionalCase{
		{name: "none", typ: "Text", argument: "none", supplied: true},
		{name: "empty_text", typ: "Text", argument: "\"\"", supplied: true, valuePresent: true, value: p15FunctionString("")},
		{name: "false", typ: "Bool", argument: "false", supplied: true, valuePresent: true, value: p15FunctionNumber("0")},
		{name: "zero", typ: "Number", argument: "0", supplied: true, valuePresent: true, value: p15FunctionNumber("0")},
		{name: "empty_list", typ: "List<AnyContent>", argument: "[]", supplied: true, valuePresent: true,
			value: p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}}},
		{name: "empty_map", typ: "Map<Text,AnyContent>", argument: "{}", supplied: true, valuePresent: true,
			value: p15FunctionArgument{true, 1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}}}},
		{name: "none_skips_default", typ: "Text", argument: "none", fallback: "DEFAULT", supplied: true},
		{name: "empty_skips_default", typ: "Text", argument: "\"\"", fallback: "DEFAULT", supplied: true, valuePresent: true, value: p15FunctionString("")},
		{name: "omitted_selects_default", typ: "Text", fallback: "DEFAULT"},
		{name: "named_none_skips_default", typ: "Text", argument: "none", fallback: "DEFAULT", supplied: true, named: true},
	}
}

func p15FunctionOptionalSource(tc p15FunctionOptionalCase) string {
	parameters := "value:" + tc.typ + "?"
	if tc.fallback != "" {
		parameters += "=\"" + tc.fallback + "\""
	}
	argument := tc.argument
	if tc.named {
		parameters = "first:Text," + parameters
		argument = "\"FIRST\",value:" + argument
	}
	return "function echo(" + parameters + ")->" + tc.typ + "?{return value}\ngetVariable(echo(" + argument + "))"
}

func p15FunctionOptionalPayload(g *p15FunctionGraph, tc p15FunctionOptionalCase) string {
	fields := map[string]p15FunctionArgument{
		"_cherri_caller_input": {true, 0, p15FunctionVariable("_cherri_caller_input")},
		"_cherri_magic":        p15FunctionString("__cherri_fn_envelope_v2__"),
		"cherri_functions":     p15FunctionString("1"), "function": p15FunctionString("echo"),
	}
	slot := 0
	values := []any{}
	if tc.named {
		fields["arg_0"] = p15FunctionString("FIRST")
		fields["has_arg_0"] = p15FunctionString("1")
		values = append(values, map[string]any{"WFItemType": 0, "WFValue": p15FunctionText("FIRST")})
		slot++
	}
	supplied, present := "0", "0"
	if tc.supplied {
		supplied = "1"
	}
	value := p15FunctionString("") // legacy positional mirror; flags own meaning
	if tc.valuePresent {
		present = "1"
		value = tc.value
		fields[fmt.Sprintf("arg_%d", slot)] = tc.value
	}
	fields[fmt.Sprintf("has_arg_%d", slot)] = p15FunctionString(supplied)
	fields[fmt.Sprintf("has_value_%d", slot)] = p15FunctionString(present)
	values = append(values, map[string]any{"WFItemType": value.kind, "WFValue": value.value})
	fields["arguments"] = p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": values}}
	keys := []string{}
	for key := range fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	rows := []any{}
	for _, key := range keys {
		field := fields[key]
		rows = append(rows, map[string]any{"WFKey": p15FunctionText(key), "WFItemType": field.kind, "WFValue": field.value})
	}
	return g.value("dictionary", map[string]any{"WFItems": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": rows}}})
}

func p15FunctionOptionalError(actions []map[string]any, tc p15FunctionOptionalCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	slot := 0
	if tc.named {
		g.get("arg_0", envelope)
		slot++
	}
	defaultGroup := ""
	if tc.fallback != "" {
		supplied := g.get(fmt.Sprintf("has_arg_%d", slot), envelope)
		defaultGroup = g.conditional(4, p15FunctionEdge(supplied), "WFConditionalActionString", "1")
	}
	presence := g.get(fmt.Sprintf("has_value_%d", slot), envelope)
	valueGroup := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	g.get(fmt.Sprintf("arg_%d", slot), envelope)
	g.boundary(valueGroup, 1, false)
	g.emit("nothing", nil)
	value := g.boundary(valueGroup, 2, true)
	if tc.fallback != "" {
		g.boundary(defaultGroup, 1, false)
		g.value("gettext", map[string]any{"WFTextActionText": tc.fallback})
		value = g.boundary(defaultGroup, 2, true)
	}
	g.output(value)
	g.caller()
	result := g.call(p15FunctionOptionalPayload(g, tc))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15FunctionOptionalPresenceWire(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases() {
		t.Run(tc.name, func(t *testing.T) {
			source := p15FunctionOptionalSource(tc)
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal("optional value/presence must be a valid call", diagnostics)
			}
			if err := p15FunctionOptionalError(p15CompiledActions(t, source), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15FunctionOptionalPresenceOracleRejectsCorruption(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases() {
		if tc.name != "none_skips_default" && tc.name != "empty_skips_default" {
			continue
		}
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, p15FunctionOptionalSource(tc))
			if err := p15FunctionOptionalError(actions, tc); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"value_flag", "omission_flag", "flag_type", "decode_argument_as_presence", "decode_truthiness", "no_nothing", "default_in_supplied_branch", "duplicate_value_decoder", "wrong_join", "none_as_empty_value", "call_input", "final_consumer"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15FunctionOptionalError(bad, tc); err != nil {
						t.Fatal("unchanged clone must pass before corruption", err)
					}
					fields, err := p15FunctionContainerPayload(bad)
					if err != nil {
						t.Fatal(err)
					}
					decoder, condition, nothing, fallback, join, call, consumer := -1, -1, -1, -1, -1, -1, -1
					for index, action := range bad {
						p := p15Param(action)
						switch action["WFWorkflowActionIdentifier"] {
						case "is.workflow.actions.getvalueforkey":
							if p["WFDictionaryKey"] == "has_value_0" {
								decoder = index
								condition = index + 1
							}
						case "is.workflow.actions.nothing":
							nothing = index
							join = index + 1
						case "is.workflow.actions.gettext":
							if p["WFTextActionText"] == "DEFAULT" {
								fallback = index
							}
						case "is.workflow.actions.runworkflow":
							call = index
						case "is.workflow.actions.getvariable":
							consumer = index
						}
					}
					if decoder < 0 || condition < 0 || nothing < 0 || fallback < 0 || join < 0 || call < 0 || consumer < 0 {
						t.Fatal("corruption target missing")
					}
					switch fault {
					case "value_flag":
						wrong := "1"
						if tc.valuePresent {
							wrong = "0"
						}
						fields["has_value_0"]["WFValue"] = p15FunctionText(wrong)
					case "omission_flag":
						fields["has_arg_0"]["WFValue"] = p15FunctionText("0")
					case "flag_type":
						fields["has_value_0"]["WFItemType"] = uint64(4)
					case "decode_argument_as_presence":
						p15Param(bad[decoder])["WFDictionaryKey"] = "arg_0"
					case "decode_truthiness":
						p15Param(bad[condition])["WFCondition"] = uint64(100)
					case "no_nothing":
						bad[nothing]["WFWorkflowActionIdentifier"] = "is.workflow.actions.comment"
					case "default_in_supplied_branch":
						bad[nothing], bad[fallback] = bad[fallback], bad[nothing]
					case "duplicate_value_decoder":
						bad = append(bad[:decoder], append([]map[string]any{bad[decoder]}, bad[decoder:]...)...)
					case "wrong_join":
						p15Param(bad[join])["GroupingIdentifier"] = "wrong-join"
					case "none_as_empty_value":
						if tc.valuePresent {
							fields["arg_0"]["WFValue"] = p15FunctionText("changed")
						} else {
							fields["has_value_0"]["WFKey"] = p15FunctionText("arg_0")
							fields["has_value_0"]["WFValue"] = p15FunctionText("")
						}
					case "call_input":
						p15Param(bad[call])["WFInput"] = p15FunctionEdge("missing-payload")
					case "final_consumer":
						p15Param(bad[consumer])["WFVariable"] = p15FunctionEdge("missing-result")
					}
					if p15FunctionOptionalError(bad, tc) == nil {
						t.Fatal("damaged value-presence graph accepted")
					}
				})
			}
		})
	}
}
