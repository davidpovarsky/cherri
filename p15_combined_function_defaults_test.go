package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15RP25Case struct {
	name, typ, supplied, fallback string
	value, fallbackValue          any
	optional, fallbackPresent     bool
}

// Each source joins omitted, positional and named calls to one typed function.
// Optional cases also supply None and observe each result's separate presence.
func p15RP25Cases() []p15RP25Case {
	return []p15RP25Case{
		{"false", "Bool", "false", "true", p15ContainerNumber("0"), p15ContainerNumber("1"), false, true},
		{"zero", "Number", "0", "9", p15ContainerNumber("0"), p15ContainerNumber("9"), false, true},
		{"empty_text", "Text", `""`, `"DEFAULT"`, "", "DEFAULT", false, true},
		{"empty_list", "List<Text>", "[]", `["DEFAULT"]`, []any{}, []any{"DEFAULT"}, false, true},
		{"empty_map", "Map<Text,Number>", "{}", `{"DEFAULT":9}`, map[string]any{}, map[string]any{"DEFAULT": p15ContainerNumber("9")}, false, true},
		{"empty_list_default", "List<Text>", `["SUPPLIED"]`, "[]", []any{"SUPPLIED"}, []any{}, false, true},
		{"empty_map_default", "Map<Text,Number>", `{"SUPPLIED":7}`, "{}", map[string]any{"SUPPLIED": p15ContainerNumber("7")}, map[string]any{}, false, true},
		// Direct Optional defaults retain the semantic Bool field; supplied
		// arguments use the existing numeric function-call normalization.
		{"optional_false", "Bool?", "false", "true", p15ContainerNumber("0"), true, true, true},
		{"optional_zero", "Number?", "0", "9", p15ContainerNumber("0"), p15ContainerNumber("9"), true, true},
		{"optional_empty_text", "Text?", `""`, `"DEFAULT"`, "", "DEFAULT", true, true},
		{"optional_empty_list", "List<Text>?", "[]", `["DEFAULT"]`, []any{}, []any{"DEFAULT"}, true, true},
		{"optional_empty_map", "Map<Text,Number>?", "{}", `{"DEFAULT":9}`, map[string]any{}, map[string]any{"DEFAULT": p15ContainerNumber("9")}, true, true},
		{"optional_none_default", "Text?", `""`, "none", "", nil, true, false},
	}
}

func p15RP25Source(tc p15RP25Case) string {
	var source = fmt.Sprintf("function choose(value:%s = %s)->%s{return value}\nlet omitted=choose()\nlet positional=choose(%s)\nlet named=choose(value:%s)\n", tc.typ, tc.fallback, tc.typ, tc.supplied, tc.supplied)
	if !tc.optional {
		return source + "return {\"omitted\":omitted,\"positional\":positional,\"named\":named}\n"
	}
	source += "let missing=choose(none)\n"
	var parts []string
	for _, name := range []string{"omitted", "positional", "named", "missing"} {
		source += "let " + name + "Present=" + name + "!=none\n"
		parts = append(parts, "{"+name+"Present}:{"+name+"}")
	}
	return source + "return f\"" + strings.Join(parts, "|") + "\"\n"
}

func TestP15RP25CombinedDefaultTransportUnit(t *testing.T) {
	for _, tc := range p15RP25Cases() {
		t.Run(tc.name, func(t *testing.T) {
			var want = strings.ReplaceAll(tc.typ, ",", ", ")
			var expressions = map[string]string{"value": want, "choose()": want, "choose(" + tc.supplied + ")": want, "choose(value:" + tc.supplied + ")": want}
			if tc.optional {
				expressions["choose(none)"] = want
				for _, name := range []string{"omitted", "positional", "named", "missing"} {
					expressions[name+"!=none"] = "Bool"
				}
			}
			p15FunctionTypedProgram(t, p15RP25Source(tc), expressions)
		})
	}
	for _, tc := range []struct{ name, typ, invalid string }{
		{"bool", "Bool", `"false"`}, {"number", "Number", `"0"`}, {"text", "Text", "false"},
		{"list", "List<Text>", "[1]"}, {"map", "Map<Text,Number>", `{"key":false}`},
	} {
		t.Run("wrong_default/"+tc.name, func(t *testing.T) {
			var source = fmt.Sprintf("function choose(value:%s = %s)->%s{return value}\nlet result=choose()\nreturn result", tc.typ, tc.invalid, tc.typ)
			p15FunctionRejects(t, source, tc.invalid, analysis.CodeArgumentType, "default value of type")
		})
		t.Run("wrong_argument/"+tc.name, func(t *testing.T) {
			var source = fmt.Sprintf("function choose(value:%s)->%s{return value}\nlet result=choose(%s)\nreturn result", tc.typ, tc.typ, tc.invalid)
			p15FunctionRejects(t, source, tc.invalid, analysis.CodeArgumentType, "argument for value expects "+strings.ReplaceAll(tc.typ, ",", ", "))
		})
	}
}

func p15RP25Default(c *p15FTCursor, tc p15RP25Case) {
	if tc.optional {
		c.optionalCarrier("_cherri_optional_argument_v1", tc.fallbackPresent, tc.fallbackValue)
		return
	}
	switch tc.typ {
	case "Bool":
		c.boolean(1, "", true)
	case "Number":
		c.number(9, "Number", true)
	case "Text":
		c.take("gettext", "Text", map[string]any{"WFTextActionText": "DEFAULT"}, true)
	case "List<Text>":
		if tc.name != "empty_list_default" {
			c.take("list", "List", map[string]any{"WFItems": []any{map[string]any{"WFItemType": uint64(0), "WFValue": "DEFAULT"}}}, true)
			return
		}
		// The private variable is derived from the final producer UUID. The
		// authored source has no earlier private lexical allocation.
		if c.err != nil || c.index+2 >= len(c.actions) {
			if c.err == nil {
				c.err = fmt.Errorf("missing empty default clear/store/read")
			}
			return
		}
		var uuid, _ = p15SCParams(c.actions[c.index+2])["UUID"].(string)
		var name = "__cherri_scope_1_emptyList_" + uuid
		c.bare("nothing")
		c.take("setvariable", "", map[string]any{"WFVariableName": name}, false)
		c.take("getvariable", "List", map[string]any{"WFVariable": p15SCAttachment(p15SCVariable(name))}, true)
	case "Map<Text,Number>":
		c.dictionary("Dictionary", tc.fallbackValue.(map[string]any))
	default:
		c.err = fmt.Errorf("unknown authored default type %s", tc.typ)
	}
}

func p15RP25Graph(actions []map[string]any, tc p15RP25Case) (*p15FTCursor, error) {
	var c = p15FTNew(actions)
	c.header()
	c.beginFunction("choose")
	c.mark("supplied")
	var supplied = c.field("has_arg_0", c.envelope, "")
	c.mark("supplied_guard")
	var branch = c.condition(supplied, 4, "WFConditionalActionString", "1")
	c.mark("child_slot")
	c.field("arg_0", c.envelope, "")
	c.mark("fallback_branch")
	c.alternate(branch)
	c.mark("fallback")
	p15RP25Default(c, tc)
	c.mark("fallback_end")
	var name = "value"
	if tc.optional {
		name = "OptionalArgument"
	}
	c.mark("default_join")
	var value = c.end(branch, name, true)
	if tc.optional {
		var presence map[string]any
		value, presence = c.decodeOptional(value, "value", "child")
		c.mark("result_guard")
		var resultGroup = c.condition(presence, 4, "WFConditionalActionString", "1")
		c.mark("result_present")
		c.optionalCarrier("_cherri_optional_result_v1", true, p15ContainerReference(value))
		c.alternate(resultGroup)
		c.mark("result_absent")
		c.optionalCarrier("_cherri_optional_result_v1", false, nil)
		value = c.end(resultGroup, "OptionalValue", true)
	} else if tc.typ == "Bool" || tc.typ == "Number" {
		value = c.numberCast(value, "value")
	}
	c.mark("child_return")
	c.output(value)
	c.caller()
	var names = []string{"omitted", "positional", "named"}
	if tc.optional {
		names = append(names, "missing")
	}
	var results, presences = map[string]map[string]any{}, map[string]map[string]any{}
	for _, name := range names {
		var argument = p15FTArgument{}
		if name != "omitted" {
			argument = p15FTArgument{true, tc.value}
			if tc.optional {
				c.mark(name + "_carrier")
				argument.value = p15ContainerReference(c.optionalCarrier("_cherri_optional_argument_v1", name != "missing", tc.value))
			}
		}
		c.mark(name + "_payload")
		var payload = c.payload("choose", argument)
		c.mark(name + "_call")
		var result = c.call(payload, name)
		if tc.optional {
			result, presences[name] = c.decodeOptional(result, name, name)
		} else if tc.typ == "Number" {
			result = c.numberCast(result, name)
		}
		results[name] = result
	}
	var observation map[string]any
	if tc.optional {
		var predicate = map[string]map[string]any{}
		for _, name := range names {
			c.mark(name + "_predicate")
			var group = c.condition(presences[name], 5, "WFConditionalActionString", "0")
			c.boolean(1, "", false)
			c.alternate(group)
			c.boolean(0, "", false)
			predicate[name] = c.end(group, name+"Present", true)
		}
		var fields = map[string]any{}
		for index, name := range names {
			fields[fmt.Sprintf("{%d, 1}", index*4)] = predicate[name]
			fields[fmt.Sprintf("{%d, 1}", index*4+2)] = results[name]
		}
		c.mark("observation")
		observation = c.take("gettext", "Text", map[string]any{"WFTextActionText": p15SCToken("\ufffc:\ufffc|\ufffc:\ufffc|\ufffc:\ufffc|\ufffc:\ufffc", fields)}, true)
	} else {
		c.mark("observation")
		observation = c.dictionary("Dictionary", map[string]any{"omitted": p15ContainerReference(results["omitted"]), "positional": p15ContainerReference(results["positional"]), "named": p15ContainerReference(results["named"])})
	}
	c.mark("final_return")
	c.output(observation)
	return c, c.finish()
}

func p15RP25Faults(tc p15RP25Case) []string {
	var faults = []string{"omitted_present", "positional_absent", "named_absent", "truthiness_guard", "presence_reads_value", "wrong_argument_slot", "fallback_branch", "fallback_kind", "fallback_value", "default_join", "child_return", "argument_native_kind", "payload_mirror", "supplied_value_reconciled", "caller_consumer", "result_name_reconciled"}
	if tc.name == "empty_list_default" {
		faults = append(faults, "empty_clear", "empty_inherits_input")
	}
	if tc.optional {
		faults = append(faults, "fallback_presence", "fallback_marker", "child_truthiness", "child_nothing", "result_presence", "result_absent_value", "positional_none", "missing_present", "none_unsupplied", "caller_presence_key", "caller_truthiness", "predicate_uses_value", "predicate_polarity")
	}
	if tc.name == "optional_false" {
		faults = append(faults, "fallback_bool_as_number")
	}
	return faults
}

func TestP15RP25CombinedDefaultTransportNative(t *testing.T) {
	for _, tc := range p15RP25Cases() {
		t.Run(tc.name, func(t *testing.T) {
			var actions = p15FTCompile(t, p15RP25Source(tc))
			var view, err = p15RP25Graph(actions, tc)
			if err != nil {
				t.Fatal(err)
			}
			var verify = func(a []map[string]any) error { _, err := p15RP25Graph(a, tc); return err }
			p15FTCommonControls(t, actions, verify)
			for _, fault := range p15RP25Faults(tc) {
				t.Run("oracle/"+fault, func(t *testing.T) {
					var changed = p15SCClone(t, actions)
					var parameters = func(point string) map[string]any { return p15SCParams(changed[view.points[point]]) }
					var fields = func(point string) map[string]map[string]any { return p15FN05Fields(t, changed, view.points[point]) }
					switch fault {
					case "omitted_present":
						fields("omitted_payload")["has_arg_0"]["WFValue"] = p15ContainerText("1")
					case "positional_absent", "named_absent":
						fields(strings.TrimSuffix(fault, "_absent") + "_payload")["has_arg_0"]["WFValue"] = p15ContainerText("0")
					case "truthiness_guard":
						parameters("supplied_guard")["WFCondition"] = uint64(100)
					case "presence_reads_value":
						parameters("supplied")["WFDictionaryKey"] = "arg_0"
					case "wrong_argument_slot":
						parameters("child_slot")["WFDictionaryKey"] = "arg_1"
					case "fallback_branch":
						parameters("fallback_branch")["WFControlFlowMode"] = uint64(2)
					case "fallback_kind":
						changed[view.points["fallback_end"]-1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
					case "fallback_value":
						if tc.optional {
							if tc.fallbackPresent {
								fields("fallback")["value"]["WFValue"] = p15ContainerText("WRONG")
							} else {
								parameters("fallback")["WFItems"] = p15RP25UnexpectedAbsentValue()
							}
						} else {
							switch tc.typ {
							case "Bool":
								parameters("fallback")["WFNumberActionNumber"] = float64(0)
							case "Number":
								parameters("fallback")["WFNumberActionNumber"] = uint64(8)
							case "Text":
								parameters("fallback")["WFTextActionText"] = "WRONG"
							case "List<Text>":
								if tc.name == "empty_list_default" {
									p15SCParams(changed[view.points["fallback_end"]-1])["WFVariable"] = p15SCAttachment(p15SCVariable("unrelated"))
								} else {
									parameters("fallback")["WFItems"] = []any{}
								}
							case "Map<Text,Number>":
								var _, wire = p15ContainerFieldValue(map[string]any{"WRONG": p15ContainerNumber("8")})
								parameters("fallback")["WFItems"] = wire
							}
						}
					case "default_join":
						parameters("default_join")["GroupingIdentifier"] = p15SCParams(changed[3])["GroupingIdentifier"]
					case "child_return":
						parameters("child_return")["WFOutput"] = p15ContainerText("WRONG")
					case "argument_native_kind":
						var row = fields("positional_payload")["arg_0"]
						row["WFItemType"] = float64(row["WFItemType"].(uint64))
					case "payload_mirror":
						fields("positional_payload")["arguments"]["WFValue"] = map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}
					case "supplied_value_reconciled":
						if tc.optional {
							fields("positional_carrier")["value"]["WFValue"] = p15ContainerText("WRONG")
						} else {
							var rows = fields("positional_payload")
							rows["arg_0"]["WFValue"] = p15ContainerText("WRONG")
							rows["arguments"]["WFValue"].(map[string]any)["Value"].([]any)[0].(map[string]any)["WFValue"] = p15ContainerText("WRONG")
						}
					case "caller_consumer":
						parameters("final_return")["WFOutput"] = p15ContainerText("PRECOMPUTED_EXPECTED_VALUES")
					case "result_name_reconciled":
						for _, action := range changed {
							p15FTRewrite(action, "CustomOutputName", "positional", "unrelated")
							p15FTRewrite(action, "OutputName", "positional", "unrelated")
						}
					case "empty_clear":
						changed[view.points["fallback"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
					case "empty_inherits_input":
						p15SCParams(changed[view.points["fallback"]+1])["WFInput"] = p15ContainerText("STALE")
					case "fallback_presence":
						var flag = "0"
						if !tc.fallbackPresent {
							flag = "1"
						}
						fields("fallback")["has_value"]["WFValue"] = p15ContainerText(flag)
					case "fallback_marker":
						fields("fallback")["_cherri_optional_argument_v1"]["WFValue"] = p15ContainerText("0")
					case "fallback_bool_as_number":
						var row = fields("fallback")["value"]
						row["WFItemType"], row["WFValue"] = uint64(3), p15ContainerText("1")
					case "child_truthiness":
						parameters("child_guard")["WFCondition"] = uint64(100)
					case "child_nothing":
						changed[view.points["child_nothing"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
					case "result_presence":
						fields("result_present")["has_value"]["WFValue"] = p15ContainerText("0")
					case "result_absent_value":
						var _, wire = p15ContainerFieldValue(map[string]any{"_cherri_optional_result_v1": "1", "has_value": "0", "value": "WRONG"})
						parameters("result_absent")["WFItems"] = wire
					case "positional_none":
						fields("positional_carrier")["has_value"]["WFValue"] = p15ContainerText("0")
					case "missing_present":
						fields("missing_carrier")["has_value"]["WFValue"] = p15ContainerText("1")
					case "none_unsupplied":
						fields("missing_payload")["has_arg_0"]["WFValue"] = p15ContainerText("0")
					case "caller_presence_key":
						parameters("positional_presence")["WFDictionaryKey"] = "value"
					case "caller_truthiness":
						parameters("positional_guard")["WFCondition"] = uint64(100)
					case "predicate_uses_value":
						parameters("positional_predicate")["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": parameters("positional_value")["UUID"], "OutputName": "positional"})}
					case "predicate_polarity":
						parameters("positional_predicate")["WFCondition"] = uint64(4)
					}
					if verify(changed) == nil {
						t.Fatal("joined default/value/presence corruption accepted", fault)
					}
				})
			}
		})
	}
}

func p15RP25UnexpectedAbsentValue() any {
	var _, wire = p15ContainerFieldValue(map[string]any{"_cherri_optional_argument_v1": "1", "has_value": "0", "value": "WRONG"})
	return wire
}
