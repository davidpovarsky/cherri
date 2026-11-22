package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15FN05Case struct {
	name, typ, literal string
	optional, present  bool
	value              any
}

func p15FN05Cases() []p15FN05Case {
	return []p15FN05Case{
		{"list_bound_numbers", "List<Number>", `[0,2.5]`, false, true, []any{p15ContainerNumber("0"), p15ContainerNumber("2.5")}},
		{"list_empty", "List<AnyContent>", `[]`, false, true, []any{}},
		{"map_reserved_nested", "Map<Text,AnyContent>", `{"WFSerializationType":"WFTextTokenAttachment","Value":false,"rows":[0,"",[],{}],"{literal}":"{literal}"}`, false, true, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": false, "rows": []any{p15ContainerNumber("0"), "", []any{}, map[string]any{}}, "{literal}": "{literal}"}},
		{"map_empty", "Map<Text,AnyContent>", `{}`, false, true, map[string]any{}},
		{"optional_none", "Text?", "none", true, false, nil},
		{"optional_empty_text", "Text?", `""`, true, true, ""},
		{"optional_false", "Bool?", "false", true, true, p15ContainerNumber("0")},
		{"optional_zero", "Number?", "0", true, true, p15ContainerNumber("0")},
		{"optional_empty_list", "List<AnyContent>?", "[]", true, true, []any{}},
		{"optional_empty_map", "Map<Text,AnyContent>?", "{}", true, true, map[string]any{}},
		{"optional_list_numbers", "List<Number>?", `[0,2.5]`, true, true, []any{p15ContainerNumber("0"), p15ContainerNumber("2.5")}},
		{"optional_map_reserved_nested", "Map<Text,AnyContent>?", `{"WFSerializationType":"WFTextTokenAttachment","Value":false,"rows":[0,"",[],{}],"{literal}":"{literal}"}`, true, true, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": false, "rows": []any{p15ContainerNumber("0"), "", []any{}, map[string]any{}}, "{literal}": "{literal}"}},
	}
}
func p15FN05Source(tc p15FN05Case) string {
	var setup, argument = "", tc.literal
	if tc.name == "list_bound_numbers" {
		setup = "let provided=" + tc.literal + "\n"
		argument = "provided"
	}
	return "function echo(value:" + tc.typ + ")->" + tc.typ + "{return value}\n" + setup + "let result=echo(" + argument + ")\nreturn result\n"
}
func p15FN05LimitCases() []p15OptionalTransportCase {
	return []p15OptionalTransportCase{
		{name: "list_nested_none", source: "function echo(value:List<AnyContent>)->List<AnyContent>{return value}\nlet result=echo([false,none,\"\"])\nreturn result", target: "none", context: "List element"},
		{name: "map_nested_none", source: "function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}\nlet result=echo({\"present\":none,\"empty\":\"\"})\nreturn result", target: "none", context: "Dictionary value"},
	}
}
func TestP15FN05CombinedStructuredTransportUnit(t *testing.T) {
	for _, tc := range p15FN05Cases() {
		t.Run(tc.name, func(t *testing.T) {
			var source = p15FN05Source(tc)
			var want = strings.ReplaceAll(tc.typ, "Map<Text,AnyContent>", "Map<Text, AnyContent>")
			var argument = tc.literal
			if tc.name == "list_bound_numbers" {
				argument = "provided"
			}
			p15FunctionTypedProgram(t, source, map[string]string{"value": want, "echo(" + argument + ")": want, "result": want})
		})
	}
	for _, tc := range p15FN05LimitCases() {
		t.Run(tc.name, func(t *testing.T) {
			var diagnostics = p15TypeDiagnostics(t, tc.source)
			if len(diagnostics) != 1 {
				t.Fatalf("exact documented limitation count: %#v", diagnostics)
			}
			if err := p15OptionalTransportDiagnostic(diagnostics, tc); err != nil {
				t.Fatal(err)
			}
			var raw, err = CompileSourceToPlist("p15-fn05-limitation.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) || !strings.Contains(err.Error(), tc.context) {
				t.Fatalf("nested absence lost precise fail-closed limitation: bytes=%d error=%v", len(raw), err)
			}
			for _, fault := range []string{"code", "start", "end", "context", "severity", "missing", "extra"} {
				t.Run("oracle/"+fault, func(t *testing.T) {
					var changed = append([]analysis.Diagnostic(nil), diagnostics...)
					switch fault {
					case "code":
						changed[0].Code = "E_UNRELATED"
					case "start":
						changed[0].Span.Start.ByteOffset++
					case "end":
						changed[0].Span.End.ByteOffset--
					case "context":
						changed[0].Message = "current unrelated optional absence"
					case "severity":
						changed[0].Severity = analysis.SeverityWarning
					case "missing":
						changed = nil
					case "extra":
						changed = append(changed, changed[0])
					}
					if p15OptionalTransportDiagnostic(changed, tc) == nil {
						t.Fatal("damaged precise transport limitation accepted")
					}
				})
			}
		})
	}
	t.Run("nested_native_list_limitation", func(t *testing.T) {
		// This is the exact previously drafted unsupported source, now explicitly
		// classified as a known target limitation rather than a positive List row.
		var tc = p15NativeListAnalysisCase{name: "function_nested_list", source: "function echo(value:List<AnyContent>)->List<AnyContent>{return value}\nlet result=echo([0,false,\"\",[],{},[1,2]])\nreturn result\n", targets: []string{"[]", "[1,2]"}}
		var diagnostics = p15TypeDiagnostics(t, tc.source)
		if len(diagnostics) != 2 {
			t.Fatalf("exact nested List limitation count: %#v", diagnostics)
		}
		if err := p15NativeListAnalysisError(diagnostics, tc); err != nil {
			t.Fatal(err)
		}
		var raw, err = CompileSourceToPlist("p15-fn05-nested-list-limitation.cherri", tc.source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_NESTED_NATIVE_LIST") {
			t.Fatalf("nested List lost explicit fail-closed limitation: bytes=%d error=%v", len(raw), err)
		}
		for _, fault := range []string{"code", "start", "end", "context", "severity", "missing", "extra"} {
			t.Run("oracle/"+fault, func(t *testing.T) {
				var changed = append([]analysis.Diagnostic(nil), diagnostics...)
				switch fault {
				case "code":
					changed[0].Code = "E_UNRELATED"
				case "start":
					changed[0].Span.Start.ByteOffset++
				case "end":
					changed[0].Span.End.ByteOffset--
				case "context":
					changed[0].Message = "unrelated current limitation"
				case "severity":
					changed[0].Severity = analysis.SeverityWarning
				case "missing":
					changed = changed[:1]
				case "extra":
					changed = append(changed, changed[0])
				}
				if p15NativeListAnalysisError(changed, tc) == nil {
					t.Fatal("damaged precise nested List limitation accepted")
				}
			})
		}
	})
	t.Log("FN05 unit scope: structured and Optional semantic types plus exact nested-absence limitation; no Apple outcomes inferred.")
}
func p15FN05Graph(actions []map[string]any, tc p15FN05Case) (*p15FTCursor, error) {
	var c = p15FTNew(actions)
	c.header()
	c.beginFunction("echo")
	c.mark("child_argument")
	var value map[string]any
	if tc.optional {
		var carrier = c.field("arg_0", c.envelope, "")
		var presence map[string]any
		value, presence = c.decodeOptional(carrier, "value", "child")
		c.mark("child_result_guard")
		var group = c.condition(presence, 4, "WFConditionalActionString", "1")
		c.mark("child_result_present")
		c.optionalCarrier("_cherri_optional_result_v1", true, p15ContainerReference(value))
		c.alternate(group)
		c.mark("child_result_absent")
		c.optionalCarrier("_cherri_optional_result_v1", false, nil)
		value = c.end(group, "OptionalValue", true)
	} else {
		value = c.field("arg_0", c.envelope, "value")
	}
	c.mark("child_output")
	c.output(value)
	c.caller()
	var argument = tc.value
	if tc.name == "list_bound_numbers" {
		c.mark("list_zero")
		var zero = c.number(0, "Number", true)
		c.mark("list_fraction")
		var fraction = c.take("number", "Number1", map[string]any{"WFNumberActionNumber": float64(2.5)}, true)
		c.mark("list_producer")
		var list = c.take("list", "provided", map[string]any{"WFItems": []any{
			map[string]any{"WFItemType": uint64(0), "WFValue": p15SCAttachment(zero)},
			map[string]any{"WFItemType": uint64(0), "WFValue": p15SCAttachment(fraction)},
		}}, true)
		argument = p15ContainerReference(list)
	}
	if tc.optional {
		c.mark("supplied_carrier")
		argument = p15ContainerReference(c.optionalCarrier("_cherri_optional_argument_v1", tc.present, tc.value))
	}
	c.mark("payload")
	var payload = c.payload("echo", p15FTArgument{true, argument})
	c.mark("call")
	var result = c.call(payload, "result")
	if tc.optional {
		result, _ = c.decodeOptional(result, "result", "caller")
	}
	c.mark("caller_output")
	c.output(result)
	return c, c.finish()
}
func p15FN05Fields(t *testing.T, a []map[string]any, index int) map[string]map[string]any {
	t.Helper()
	var fields, err = p15ContainerFields(p15SCParams(a[index])["WFItems"])
	if err != nil {
		t.Fatal(err)
	}
	return fields
}
func TestP15FN05CombinedStructuredTransportNative(t *testing.T) {
	for _, tc := range p15FN05Cases() {
		t.Run(tc.name, func(t *testing.T) {
			var actions = p15FTCompile(t, p15FN05Source(tc))
			var view, err = p15FN05Graph(actions, tc)
			if err != nil {
				t.Fatal(err)
			}
			var verify = func(a []map[string]any) error { _, err := p15FN05Graph(a, tc); return err }
			p15FTCommonControls(t, actions, verify)
			for _, fault := range []string{"child_slot", "child_output", "payload_arg_removed", "payload_presence", "payload_mirror", "payload_field_kind", "self_target", "call_input", "caller_output", "result_name_reconciled"} {
				t.Run("oracle/"+fault, func(t *testing.T) {
					var changed = p15SCClone(t, actions)
					switch fault {
					case "child_slot":
						p15SCParams(changed[view.points["child_argument"]])["WFDictionaryKey"] = "arg_1"
					case "child_output":
						p15SCParams(changed[view.points["child_output"]])["WFOutput"] = p15ContainerText("UNRELATED")
					case "payload_arg_removed":
						var items = p15SCParams(changed[view.points["payload"]])["WFItems"].(map[string]any)["Value"].(map[string]any)
						var rows = items["WFDictionaryFieldValueItems"].([]any)
						var kept = []any{}
						for _, raw := range rows {
							if !reflect.DeepEqual(raw.(map[string]any)["WFKey"], p15ContainerText("arg_0")) {
								kept = append(kept, raw)
							}
						}
						if len(kept) != len(rows)-1 {
							t.Fatal("causal arg row absent")
						}
						items["WFDictionaryFieldValueItems"] = kept
					case "payload_presence":
						p15FN05Fields(t, changed, view.points["payload"])["has_arg_0"]["WFValue"] = p15ContainerText("0")
					case "payload_mirror":
						p15FN05Fields(t, changed, view.points["payload"])["arguments"]["WFValue"] = map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}
					case "payload_field_kind":
						var row = p15FN05Fields(t, changed, view.points["payload"])["arg_0"]
						row["WFItemType"] = float64(row["WFItemType"].(uint64))
					case "self_target":
						p15SCParams(changed[view.points["call"]])["WFWorkflow"].(map[string]any)["isSelf"] = false
					case "call_input":
						p15SCParams(changed[view.points["call"]])["WFInput"] = p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": p15SCParams(changed[0])["UUID"]})
					case "caller_output":
						p15SCParams(changed[view.points["caller_output"]])["WFOutput"] = p15ContainerText("EXPECTED_SHAPE_WITHOUT_CALL")
					case "result_name_reconciled":
						for _, action := range changed {
							p15FTRewrite(action, "CustomOutputName", "result", "wrongResult")
							p15FTRewrite(action, "OutputName", "result", "wrongResult")
						}
					}
					if verify(changed) == nil {
						t.Fatal("joined transport corruption accepted")
					}
				})
			}
			if tc.name == "list_bound_numbers" {
				for _, fault := range []string{"raw_number_row", "swapped_number_rows", "wrong_number_kind", "payload_uses_scalar"} {
					t.Run("oracle/"+fault, func(t *testing.T) {
						var changed = p15SCClone(t, actions)
						var rows = p15SCParams(changed[view.points["list_producer"]])["WFItems"].([]any)
						switch fault {
						case "raw_number_row":
							rows[0].(map[string]any)["WFValue"] = uint64(0)
						case "swapped_number_rows":
							rows[0], rows[1] = rows[1], rows[0]
						case "wrong_number_kind":
							p15SCParams(changed[view.points["list_zero"]])["WFNumberActionNumber"] = float64(0)
						case "payload_uses_scalar":
							var ref = p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": p15SCParams(changed[view.points["list_zero"]])["UUID"], "OutputName": "Number"})
							var fields = p15FN05Fields(t, changed, view.points["payload"])
							fields["arg_0"]["WFValue"] = ref
							fields["arguments"]["WFValue"].(map[string]any)["Value"].([]any)[0].(map[string]any)["WFValue"] = ref
						}
						if verify(changed) == nil {
							t.Fatal("native List producer/transport corruption accepted")
						}
					})
				}
			}
			if strings.HasSuffix(tc.name, "map_reserved_nested") {
				for _, fault := range []string{"map_false_as_number", "map_empty_list_as_map", "map_array_order", "map_reserved_key_lost"} {
					t.Run("oracle/"+fault, func(t *testing.T) {
						var changed = p15SCClone(t, actions)
						var mutate = func(wire any) {
							var fields, err = p15ContainerFields(wire)
							if err != nil {
								t.Fatal(err)
							}
							switch fault {
							case "map_false_as_number":
								fields["Value"]["WFItemType"] = uint64(3)
								fields["Value"]["WFValue"] = p15ContainerText("0")
							case "map_empty_list_as_map":
								var rows = fields["rows"]["WFValue"].(map[string]any)["Value"].([]any)
								var kind, value = p15ContainerFieldValue(map[string]any{})
								rows[2] = map[string]any{"WFItemType": kind, "WFValue": value}
							case "map_array_order":
								var rows = fields["rows"]["WFValue"].(map[string]any)["Value"].([]any)
								rows[0], rows[1] = rows[1], rows[0]
							case "map_reserved_key_lost":
								var dictionary = wire.(map[string]any)["Value"].(map[string]any)
								var rows = dictionary["WFDictionaryFieldValueItems"].([]any)
								var kept []any
								for _, raw := range rows {
									if !reflect.DeepEqual(raw.(map[string]any)["WFKey"], p15ContainerText("WFSerializationType")) {
										kept = append(kept, raw)
									}
								}
								if len(kept) != len(rows)-1 {
									t.Fatal("causal reserved-key target absent")
								}
								dictionary["WFDictionaryFieldValueItems"] = kept
							}
						}
						if tc.optional {
							mutate(p15FN05Fields(t, changed, view.points["supplied_carrier"])["value"]["WFValue"])
						} else {
							var fields = p15FN05Fields(t, changed, view.points["payload"])
							// Change both envelope copies so the control cannot fail only
							// because the positional mirror disagrees with arg_0.
							mutate(fields["arg_0"]["WFValue"])
							mutate(fields["arguments"]["WFValue"].(map[string]any)["Value"].([]any)[0].(map[string]any)["WFValue"])
						}
						if verify(changed) == nil {
							t.Fatal("nested Map kind, cardinality, order or reserved-key loss accepted")
						}
					})
				}
			}
			if tc.optional {
				for _, fault := range []string{"argument_marker", "argument_presence", "child_truthiness", "child_nothing", "result_presence", "result_value_edge", "result_absence_value", "caller_truthiness", "caller_nothing", "caller_presence_key"} {
					t.Run("oracle/"+fault, func(t *testing.T) {
						var changed = p15SCClone(t, actions)
						switch fault {
						case "argument_marker":
							p15FN05Fields(t, changed, view.points["supplied_carrier"])["_cherri_optional_argument_v1"]["WFValue"] = p15ContainerText("0")
						case "argument_presence":
							var want = "1"
							if tc.present {
								want = "0"
							}
							p15FN05Fields(t, changed, view.points["supplied_carrier"])["has_value"]["WFValue"] = p15ContainerText(want)
						case "child_truthiness":
							p15SCParams(changed[view.points["child_guard"]])["WFCondition"] = uint64(100)
						case "child_nothing":
							changed[view.points["child_nothing"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
						case "result_presence":
							p15FN05Fields(t, changed, view.points["child_result_present"])["has_value"]["WFValue"] = p15ContainerText("0")
						case "result_value_edge":
							p15FN05Fields(t, changed, view.points["child_result_present"])["value"]["WFValue"] = p15ContainerText("UNRELATED")
						case "result_absence_value":
							var fields = p15SCParams(changed[view.points["child_result_absent"]])["WFItems"].(map[string]any)["Value"].(map[string]any)
							var rows = fields["WFDictionaryFieldValueItems"].([]any)
							fields["WFDictionaryFieldValueItems"] = append(rows, map[string]any{"WFKey": p15ContainerText("value"), "WFItemType": uint64(0), "WFValue": p15ContainerText("")})
						case "caller_truthiness":
							p15SCParams(changed[view.points["caller_guard"]])["WFCondition"] = uint64(100)
						case "caller_nothing":
							changed[view.points["caller_nothing"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
						case "caller_presence_key":
							p15SCParams(changed[view.points["caller_presence"]])["WFDictionaryKey"] = "value"
						}
						if verify(changed) == nil {
							t.Fatal("optional presence/result corruption accepted")
						}
					})
				}
			}
			t.Logf("FN05 complete native structure: case=%s actions=%d; Apple values/types unobserved", tc.name, len(actions))
		})
	}
}
