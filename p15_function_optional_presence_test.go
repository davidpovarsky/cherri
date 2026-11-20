package main

import (
	"fmt"
	"reflect"
	"sort"
	"testing"
)

// This models the complete native graph, not an Apple runtime. Argument
// omission/default selection and the value's own presence are independent.
type p15FunctionOptionalCase struct {
	name, typ, argument, fallback string
	supplied, valuePresent, named bool
	defaultEffect                 bool
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
		{name: "omitted_none_default", typ: "Text", fallback: "none"},
		{name: "explicit_none_with_none_default", typ: "Text", argument: "none", fallback: "none", supplied: true},
		{name: "named_none_skips_default", typ: "Text", argument: "none", fallback: "DEFAULT", supplied: true, named: true},
		{name: "empty_list_default", typ: "List<AnyContent>", fallback: "[]"},
		{name: "none_skips_empty_list_default", typ: "List<AnyContent>", argument: "none", fallback: "[]", supplied: true},
		{name: "empty_map_default", typ: "Map<Text,AnyContent>", fallback: "{}"},
		{name: "none_skips_empty_map_default", typ: "Map<Text,AnyContent>", argument: "none", fallback: "{}", supplied: true},
		{name: "omitted_runs_default_effect", typ: "Text", fallback: "DEFAULT", defaultEffect: true},
		{name: "none_skips_default_effect", typ: "Text", argument: "none", fallback: "DEFAULT", defaultEffect: true, supplied: true},
		{name: "empty_skips_default_effect", typ: "Text", argument: "\"\"", fallback: "DEFAULT", defaultEffect: true, supplied: true, valuePresent: true, value: p15FunctionString("")},
	}
}

func p15FunctionOptionalSource(tc p15FunctionOptionalCase) string {
	parameters := "value:" + tc.typ + "?"
	if tc.fallback == "none" || tc.fallback == "[]" || tc.fallback == "{}" {
		parameters += "=" + tc.fallback
	} else if tc.defaultEffect {
		parameters += "=text(\"" + tc.fallback + "\")"
	} else if tc.fallback != "" {
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
	args := []p15FunctionArgument{}
	if tc.named {
		args = append(args, p15FunctionString("FIRST"))
	}
	value := p15FunctionArgument{}
	if tc.supplied {
		carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", tc.valuePresent, tc.value)
		value = p15FunctionArgument{true, 0, p15FunctionEdge(carrier)}
	}
	args = append(args, value)
	return g.payload("echo", args...)
}

func p15DecodeOptionalArgumentGraph(g *p15FunctionGraph, envelope string, slot int) (value, presence string) {
	carrier := g.get(fmt.Sprintf("arg_%d", slot), envelope)
	return p15DecodeOptionalGraph(g, carrier, "has_value", "value")
}

// Resolve the actual supplied static carrier through its exact envelope edge.
// This helper selects mutation targets; the full graph remains the oracle.
func p15OptionalArgumentFields(actions []map[string]any, slot int) (map[string]map[string]any, error) {
	fields, err := p15FunctionContainerPayload(actions)
	if err != nil {
		return nil, err
	}
	row := fields[fmt.Sprintf("arg_%d", slot)]
	if row == nil {
		return nil, fmt.Errorf("supplied Optional carrier is absent")
	}
	wire, ok := row["WFValue"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("Optional argument is not a native reference")
	}
	descriptor, ok := wire["Value"].(map[string]any)
	if !ok || descriptor["Type"] != "ActionOutput" {
		return nil, fmt.Errorf("Optional argument has no exact carrier edge")
	}
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" || p15Param(action)["UUID"] != descriptor["OutputUUID"] {
			continue
		}
		inner, err := p15ContainerFields(p15Param(action)["WFItems"])
		if err != nil {
			return nil, err
		}
		if marker := inner["_cherri_optional_argument_v1"]; marker == nil || !reflect.DeepEqual(marker["WFValue"], p15FunctionText("1")) {
			return nil, fmt.Errorf("wrong Optional argument carrier marker")
		}
		return inner, nil
	}
	return nil, fmt.Errorf("Optional argument carrier edge is unresolved")
}

func p15OptionalCarrierGraph(g *p15FunctionGraph, presenceKey, valueKey, marker string, present bool, value p15FunctionArgument) string {
	fields := map[string]p15FunctionArgument{presenceKey: p15FunctionString("0")}
	if marker != "" {
		fields[marker] = p15FunctionString("1")
	}
	if present {
		fields[presenceKey] = p15FunctionString("1")
		fields[valueKey] = value
	}
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

func p15DecodeOptionalGraph(g *p15FunctionGraph, envelope, presenceKey, valueKey string) (value, presence string) {
	presence = g.get(presenceKey, envelope)
	group := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	g.get(valueKey, envelope)
	g.boundary(group, 1, false)
	g.emit("nothing", nil)
	value = g.boundary(group, 2, true)
	return value, presence
}

func p15FunctionOptionalError(actions []map[string]any, tc p15FunctionOptionalCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	slot := 0
	if tc.named {
		g.get("arg_0", envelope)
		slot++
	}
	valueKey := fmt.Sprintf("arg_%d", slot)
	if tc.fallback != "" {
		supplied := g.get(fmt.Sprintf("has_arg_%d", slot), envelope)
		defaultGroup := g.conditional(4, p15FunctionEdge(supplied), "WFConditionalActionString", "1")
		g.get(valueKey, envelope)
		g.boundary(defaultGroup, 1, false)
		fallback := p15FunctionString(tc.fallback)
		switch tc.fallback {
		case "[]":
			fallback = p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}}
		case "{}":
			fallback = p15FunctionArgument{true, 1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{}}}}
		}
		if tc.defaultEffect {
			producer := g.value("gettext", map[string]any{"WFTextActionText": tc.fallback})
			fallback = p15FunctionArgument{true, 0, p15FunctionEdge(producer)}
		}
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_argument_v1", tc.fallback != "none", fallback)
		envelope = g.boundary(defaultGroup, 2, true)
	} else {
		envelope = g.get(valueKey, envelope)
	}
	value, presence := p15DecodeOptionalGraph(g, envelope, "has_value", "value")
	resultGroup := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(value)})
	g.boundary(resultGroup, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	carrier := g.boundary(resultGroup, 2, true)
	g.output(carrier)
	g.caller()
	result := g.call(p15FunctionOptionalPayload(g, tc))
	unboxed, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(unboxed)})
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

func p15OptionalEmptyListResultError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("make")
	g.get("arg_0", envelope)
	g.emit("gettext", map[string]any{"WFTextActionText": "MUST_NOT_LEAK"})
	carrier := p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true,
		p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}})
	g.output(carrier)
	g.caller()
	result := g.call(g.payload("make", p15FunctionString("unused")))
	value, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	return g.finish()
}

func TestP15OptionalEmptyListResultIsPresent(t *testing.T) {
	source := "function make(seed:Text)->List<AnyContent>?{text(\"MUST_NOT_LEAK\");return []}\ngetVariable(make(\"unused\"))"
	actions := p15CompiledActions(t, source)
	if err := p15OptionalEmptyListResultError(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"absent_flag", "missing_array_value", "empty_text_alias", "wrong_value_kind", "stale_output", "truthiness_decoder"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15ContainerCloneNative(t, actions)
			if err := p15OptionalEmptyListResultError(bad); err != nil {
				t.Fatal("unchanged clone must pass", err)
			}
			var fields map[string]map[string]any
			output, presence := -1, -1
			for i, action := range bad {
				p := p15Param(action)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
					if candidate, err := p15ContainerFields(p["WFItems"]); err == nil && candidate["_cherri_optional_result_v1"] != nil {
						fields = candidate
					}
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.output" {
					output = i
				}
				if p["WFDictionaryKey"] == "has_value" {
					presence = i + 1
				}
			}
			if fields == nil || output < 0 || presence < 0 {
				t.Fatal("mutation target absent")
			}
			switch fault {
			case "absent_flag":
				fields["has_value"]["WFValue"] = p15FunctionText("0")
			case "missing_array_value":
				delete(fields["value"]["WFValue"].(map[string]any), "Value")
			case "empty_text_alias":
				fields["value"]["WFValue"] = p15FunctionText("")
			case "wrong_value_kind":
				fields["value"]["WFItemType"] = uint64(0)
			case "stale_output":
				p15Param(bad[output])["WFOutput"] = p15FunctionText("MUST_NOT_LEAK")
			case "truthiness_decoder":
				p15Param(bad[presence])["WFCondition"] = uint64(100)
			}
			if p15OptionalEmptyListResultError(bad) == nil {
				t.Fatal("damaged present-empty result accepted")
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
					inner, err := p15OptionalArgumentFields(bad, 0)
					if err != nil {
						t.Fatal(err)
					}
					decoder, condition, nothing, fallback, join, call, consumer := -1, -1, -1, -1, -1, -1, -1
					for index, action := range bad {
						p := p15Param(action)
						switch action["WFWorkflowActionIdentifier"] {
						case "is.workflow.actions.getvalueforkey":
							if decoder < 0 && p["WFDictionaryKey"] == "has_value" {
								decoder = index
								condition = index + 1
							}
						case "is.workflow.actions.nothing":
							// Select the argument decoder paired with the first presence
							// condition, never the later caller decoder or main boundary.
							if decoder >= 0 && nothing < 0 {
								nothing = index
								join = index + 1
							}
						case "is.workflow.actions.dictionary":
							if fields, err := p15ContainerFields(p["WFItems"]); err == nil {
								if row := fields["value"]; row != nil && reflect.DeepEqual(row["WFValue"], p15FunctionText("DEFAULT")) {
									fallback = index
								}
							}
						case "is.workflow.actions.runworkflow":
							call = index
						case "is.workflow.actions.getvariable":
							consumer = index
						}
					}
					if decoder < 0 || condition < 0 || nothing < 0 || fallback < 0 || join < 0 || join >= len(bad) || call < 0 || consumer < 0 {
						t.Fatal("corruption target missing")
					}
					if bad[join]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || p15Param(bad[join])["GroupingIdentifier"] != p15Param(bad[condition])["GroupingIdentifier"] {
						t.Fatal("corruption target is not the matching argument presence join")
					}
					switch fault {
					case "value_flag":
						wrong := "1"
						if tc.valuePresent {
							wrong = "0"
						}
						inner["has_value"]["WFValue"] = p15FunctionText(wrong)
					case "omission_flag":
						fields["has_arg_0"]["WFValue"] = p15FunctionText("0")
					case "flag_type":
						inner["has_value"]["WFItemType"] = uint64(4)
					case "decode_argument_as_presence":
						p15Param(bad[decoder])["WFDictionaryKey"] = "value"
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
							inner["value"]["WFValue"] = p15FunctionText("changed")
						} else {
							inner["has_value"]["WFKey"] = p15FunctionText("value")
							inner["has_value"]["WFValue"] = p15FunctionText("")
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
