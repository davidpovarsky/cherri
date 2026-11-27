package main

import (
	"fmt"
	"reflect"
	"testing"
)

func p15CallerInputText(literal string, refs ...map[string]any) map[string]any {
	attachments := map[string]any{}
	for index, ref := range refs {
		attachments[fmt.Sprintf("{%d, 1}", index*2)] = ref["Value"]
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{
		"string": literal, "attachmentsByRange": attachments,
	}}
}

func p15CallerInputSource(nested bool) string {
	source := `function inspect(value:Text)->Text{return f"{system.input}|{ShortcutInput}|{value}"}
`
	callee := "inspect"
	if nested {
		source += `function outer(value:Text)->Text{
let inner=inspect(value)
return f"{system.input}|{inner}|{ShortcutInput}"
}
`
		callee = "outer"
	}
	return source + `let before=f"{system.input}|{ShortcutInput}"
let result=` + callee + `("VALUE")
setClipboard(f"{before}|{result}|{system.input}|{ShortcutInput}")`
}

// The same preserved caller reference is required at every authored read and
// child payload. Only the guarded dispatcher probes inspect ExtensionInput.
// UUID alpha names are taken from the actual graph; no semantic value is.
func p15CallerInputGraph(actions []map[string]any, nested bool) (map[string]int, error) {
	return p15CallerInputGraphForWorkflow(actions, nested, "")
}

// The existing default and every semantic graph assertion stay identical;
// a signed native companion supplies its actual compiler-selected self name.
func p15CallerInputGraphForWorkflow(actions []map[string]any, nested bool, workflowName string) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}, workflowName: workflowName}
	positions := map[string]int{}
	envelope := g.header("inspect")
	caller := p15FunctionVariable("_cherri_caller_input")
	argument := g.get("arg_0", envelope)
	positions["body"] = len(g.expected)
	value := g.value("gettext", map[string]any{"WFTextActionText": p15CallerInputText("\ufffc|\ufffc|\ufffc", caller, caller, p15FunctionEdge(argument))})
	g.output(value)
	callee := "inspect"
	if nested {
		g.nextFunction("outer")
		outerArg := g.get("arg_0", envelope)
		positions["nested_payload"] = len(g.expected)
		inner := g.call(g.payload("inspect", p15FunctionArgument{true, 0, p15FunctionEdge(outerArg)}))
		positions["outer_body"] = len(g.expected)
		outer := g.value("gettext", map[string]any{"WFTextActionText": p15CallerInputText("\ufffc|\ufffc|\ufffc", caller, p15FunctionEdge(inner), caller)})
		g.output(outer)
		callee = "outer"
	}
	g.caller()
	positions["before"] = len(g.expected)
	before := g.value("gettext", map[string]any{"WFTextActionText": p15CallerInputText("\ufffc|\ufffc", caller, caller)})
	positions["payload"] = len(g.expected)
	result := g.call(g.payload(callee, p15FunctionString("VALUE")))
	positions["consumer"] = len(g.expected)
	g.emit("setclipboard", map[string]any{"WFInput": p15CallerInputText("\ufffc|\ufffc|\ufffc|\ufffc", p15FunctionEdge(before), p15FunctionEdge(result), caller, caller)})
	return positions, g.finish()
}

func TestP15FN04CallerInputCompleteNativeGraphs(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested_%t", nested), func(t *testing.T) {
			source := p15CallerInputSource(nested)
			callee := "inspect"
			if nested {
				callee = "outer"
			}
			p15FunctionTypedProgram(t, source, map[string]string{callee + `("VALUE")`: "Text"})
			actions := p15CompiledActions(t, source)
			p15FunctionBoundaryRetain(t, fmt.Sprintf("caller-input-nested-%t", nested), actions)
			if _, err := p15CallerInputGraph(actions, nested); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15FN04CallerInputWithoutFunctions(t *testing.T) {
	actions := p15CompiledActions(t, "getVariable(system.input)\ngetVariable(ShortcutInput)")
	want := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.getvariable", "WFWorkflowActionParameters": map[string]any{
		"WFVariable": p15FunctionAttachment(map[string]any{"Type": "ExtensionInput"}),
	}}
	if len(actions) != 3 {
		t.Fatal("ordinary caller input must be two input reads followed by main Nothing", len(actions))
	}
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		t.Fatal(err)
	}
	for _, action := range actions[:2] {
		if !reflect.DeepEqual(p15FunctionComparable(action, false), want) {
			t.Fatalf("ordinary system.input/ShortcutInput lost ExtensionInput: %#v", action)
		}
	}
}

func TestP15FN04CallerInputCorruptions(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprintf("nested_%t", nested), func(t *testing.T) {
			original := p15CompiledActions(t, p15CallerInputSource(nested))
			positions, err := p15CallerInputGraph(original, nested)
			if err != nil {
				t.Fatal(err)
			}
			faults := []string{"initial_snapshot_lost", "unguarded_envelope", "marker_key_wrong", "marker_value_wrong", "marker_truthiness", "restore_arguments", "restore_envelope", "body_reads_transport", "caller_reads_transport", "payload_loses_caller", "consumer_reads_transport", "call_uses_wrong_input"}
			if nested {
				faults = append(faults, "nested_payload_loses_caller", "nested_body_reads_transport")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					bad := p15C01Clone(t, original)
					transport := p15FunctionAttachment(map[string]any{"Type": "ExtensionInput"})
					switch fault {
					case "initial_snapshot_lost":
						p15C01Params(bad[2])["WFInput"] = p15FunctionText("REPLACED")
					case "unguarded_envelope":
						p15C01Params(bad[3])["WFControlFlowMode"] = 1
					case "marker_key_wrong":
						p15C01Params(bad[6])["WFDictionaryKey"] = "cherri_functions"
					case "marker_value_wrong":
						p15C01Params(bad[7])["WFConditionalActionString"] = "__wrong_envelope__"
					case "marker_truthiness":
						p15C01Params(bad[7])["WFCondition"] = 100
					case "restore_arguments":
						p15C01Params(bad[11])["WFDictionaryKey"] = "arguments"
					case "restore_envelope":
						p15C01Params(bad[12])["WFInput"] = p15FunctionEdge(p15C01Params(bad[4])["UUID"].(string))
					case "body_reads_transport", "caller_reads_transport", "nested_body_reads_transport":
						name := "body"
						if fault == "caller_reads_transport" {
							name = "before"
						} else if fault == "nested_body_reads_transport" {
							name = "outer_body"
						}
						value := p15C01Params(bad[positions[name]])["WFTextActionText"].(map[string]any)["Value"].(map[string]any)
						value["attachmentsByRange"].(map[string]any)["{0, 1}"] = transport["Value"]
					case "payload_loses_caller", "nested_payload_loses_caller":
						name := "payload"
						if fault == "nested_payload_loses_caller" {
							name = "nested_payload"
						}
						p15FunctionPayloadRow(t, bad, positions[name], "_cherri_caller_input")["WFValue"] = transport
					case "consumer_reads_transport":
						value := p15C01Params(bad[positions["consumer"]])["WFInput"].(map[string]any)["Value"].(map[string]any)
						value["attachmentsByRange"].(map[string]any)["{4, 1}"] = transport["Value"]
					case "call_uses_wrong_input":
						p15C01Params(bad[positions["payload"]+1])["WFInput"] = transport
					}
					if _, err := p15CallerInputGraph(bad, nested); err == nil {
						t.Fatal("caller/transport corruption accepted")
					}
				})
			}
		})
	}
}
