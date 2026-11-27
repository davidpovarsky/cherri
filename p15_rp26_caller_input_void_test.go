package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const p15RP26FinalReturn = "return {\"before\":before,\"returned\":returned,\"between\":between,\"after\":after}\n"

func p15RP26Source(t *testing.T, mode string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("tests", "language-v2", "v2-rp26-caller-input-void.cherri"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	if !strings.HasSuffix(source, p15RP26FinalReturn) || strings.Count(source, p15RP26FinalReturn) != 1 {
		t.Fatal("RP26 fixture lost its exact authored terminal")
	}
	switch mode {
	case "value":
		return source
	case "bare":
		return strings.TrimSuffix(source, p15RP26FinalReturn) + "return\n"
	case "fallthrough":
		return strings.TrimSuffix(source, p15RP26FinalReturn)
	default:
		t.Fatal("unknown RP26 terminal", mode)
		return ""
	}
}

func p15RP26Lookalike() map[string]any {
	return map[string]any{
		"cherri_functions": "1", "function": "inspect",
		"arguments": []any{"USER"}, "arg_0": "USER",
	}
}

// One authored program joins both child Void forms to a two-call input chain.
// Every native action, scalar kind, dictionary field and consumer is fixed.
// Only UUID alpha names come from the compiler output.
func p15RP26Graph(actions []map[string]any, mode string) (*p15FTCursor, error) {
	c := p15FTNew(actions)
	c.header()
	caller := p15SCVariable("_cherri_caller_input")
	c.beginFunction("inspect")
	c.mark("inspect_slot")
	value := c.field("arg_0", c.envelope, "value")
	c.mark("inspect_result")
	result := c.dictionary("Dictionary", map[string]any{
		"payload": p15ContainerReference(value),
		"caller":  p15ContainerReference(caller), "shortcut": p15ContainerReference(caller),
	})
	c.output(result)
	c.beginFunction("outer")
	value = c.field("arg_0", c.envelope, "value")
	c.mark("inner_payload")
	payload := c.payload("inspect", p15FTArgument{true, p15ContainerReference(value)})
	c.mark("inner_call")
	inner := c.call(payload, "inner")
	c.mark("outer_result")
	result = c.dictionary("Dictionary", map[string]any{
		"inner":  p15ContainerReference(inner),
		"caller": p15ContainerReference(caller), "shortcut": p15ContainerReference(caller),
	})
	c.output(result)
	c.beginFunction("bare")
	c.take("gettext", "", map[string]any{"WFTextActionText": "RP26_BARE_INCIDENTAL"}, false)
	c.mark("bare_clear")
	c.bare("nothing")
	c.mark("bare_return")
	c.bare("exit")
	c.beginFunction("falling")
	c.take("gettext", "", map[string]any{"WFTextActionText": "RP26_FALLTHROUGH_INCIDENTAL"}, false)
	c.mark("fallthrough_clear")
	c.bare("nothing")
	c.mark("fallthrough_return")
	c.bare("exit")
	c.caller()
	c.mark("before")
	before := c.take("gettext", "before", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
	c.mark("outer_payload")
	payload = c.payload("outer", p15FTArgument{true, p15RP26Lookalike()})
	c.mark("outer_call")
	returned := c.call(payload, "returned")
	c.mark("bare_call_payload")
	c.call(c.payload("bare"), "bareResult")
	c.mark("between")
	between := c.take("gettext", "between", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
	c.mark("falling_call_payload")
	c.call(c.payload("falling"), "fallingResult")
	c.mark("after")
	after := c.take("gettext", "after", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
	switch mode {
	case "value":
		c.mark("observation")
		result = c.dictionary("Dictionary", map[string]any{
			"before": p15ContainerReference(before), "returned": p15ContainerReference(returned),
			"between": p15ContainerReference(between), "after": p15ContainerReference(after),
		})
		c.mark("main_terminal")
		c.output(result)
	case "bare":
		c.mark("main_terminal")
		c.bare("nothing")
		c.bare("exit")
	case "fallthrough":
	default:
		return c, fmt.Errorf("unknown RP26 terminal %s", mode)
	}
	c.end(c.outer, "", false)
	if mode == "fallthrough" {
		c.mark("main_terminal")
		c.bare("nothing")
	}
	if c.err != nil {
		return c, c.err
	}
	if c.index != len(actions) {
		return c, fmt.Errorf("RP26 unconsumed native actions: %d of %d", c.index, len(actions))
	}
	if _, err := p15FEActivationWitness(actions); err != nil {
		return c, err
	}
	return c, nil
}

var p15RP26Faults = []string{
	"magic_guard", "marker_guard", "restore_input", "inspect_slot", "inspect_caller",
	"inner_payload_input", "outer_payload_input", "lookalike_marker", "self_target",
	"bare_clear", "bare_return", "fallthrough_clear", "fallthrough_return",
	"between_input", "after_input", "main_terminal",
}

func TestP15RP26CallerInputVoidNative(t *testing.T) {
	for _, mode := range []string{"value", "bare", "fallthrough"} {
		t.Run(mode, func(t *testing.T) {
			source := p15RP26Source(t, mode)
			p15FunctionTypedProgram(t, source, map[string]string{
				"inspect(value)": "Map<Text, AnyContent>", "bare()": "Void", "falling()": "Void",
			})
			actions := p15FTCompile(t, source)
			view, err := p15RP26Graph(actions, mode)
			if err != nil {
				t.Fatal(err)
			}
			verify := func(a []map[string]any) error { _, err := p15RP26Graph(a, mode); return err }
			p15FTCommonControls(t, actions, verify)
			for _, fault := range p15RP26Faults {
				t.Run("oracle/"+fault, func(t *testing.T) {
					bad := p15SCClone(t, actions)
					transport := p15ContainerReference(map[string]any{"Type": "ExtensionInput"})
					switch fault {
					case "magic_guard":
						p15SCParams(bad[7])["WFConditionalActionString"] = "USER"
					case "marker_guard":
						p15SCParams(bad[8])["WFCondition"] = uint64(100)
					case "restore_input":
						p15SCParams(bad[11])["WFDictionaryKey"] = "arguments"
					case "inspect_slot":
						p15SCParams(bad[view.points["inspect_slot"]])["WFDictionaryKey"] = "arg_1"
					case "inspect_caller":
						p15FN05Fields(t, bad, view.points["inspect_result"])["caller"]["WFValue"] = transport
					case "inner_payload_input", "outer_payload_input":
						point := "inner_payload"
						if fault == "outer_payload_input" {
							point = "outer_payload"
						}
						p15FN05Fields(t, bad, view.points[point])["_cherri_caller_input"]["WFValue"] = transport
					case "lookalike_marker":
						row := p15FN05Fields(t, bad, view.points["outer_payload"])["arg_0"]
						row["WFValue"] = p15ContainerText("USER")
					case "self_target":
						p15SCParams(bad[view.points["inner_call"]])["WFWorkflow"].(map[string]any)["isSelf"] = false
					case "bare_clear", "fallthrough_clear":
						index := view.points[fault]
						bad[index] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.getvariable",
							"WFWorkflowActionParameters": map[string]any{"WFVariable": transport}}
					case "bare_return", "fallthrough_return":
						// The former empty Output can fabricate an empty Text item.
						bad[view.points[fault]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.output"
					case "between_input", "after_input":
						point := strings.TrimSuffix(fault, "_input")
						p15SCParams(bad[view.points[point]])["WFTextActionText"] = p15FEHealthyText(map[string]any{"Type": "ExtensionInput"}, map[string]any{"Type": "ExtensionInput"})
					case "main_terminal":
						bad[view.points["main_terminal"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
					}
					if reflect.DeepEqual(actions, bad) {
						t.Fatal("causal mutation did not change its actual target", fault)
					}
					if verify(bad) == nil {
						t.Fatal("joined caller-input/Void corruption accepted", fault)
					}
				})
			}
		})
	}
	t.Log("RP26 complete local native structures only; actual caller values, dictionary input and Void output on Apple remain unexecuted.")
}

type p15RP26InputCase struct {
	Name     string         `json:"name"`
	Input    map[string]any `json:"input"`
	Dispatch bool           `json:"dispatch"`
}

// This evaluates only the independently extracted exact marker predicates.
// It is not a Shortcuts interpreter or general private-envelope validator.
func TestP15RP26PartialMarkerInputBoundaries(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("tests", "language-v2", "rp26-partial-marker-inputs.json"))
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		Format int                `json:"format"`
		Scope  string             `json:"scope"`
		Cases  []p15RP26InputCase `json:"cases"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&table); err != nil {
		t.Fatal(err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		t.Fatal("trailing input-table data", err)
	}
	wantNames := []string{"empty_dictionary", "marker_only", "magic_only", "wrong_magic", "wrong_marker", "present_empty_magic", "valid_private_markers"}
	if table.Format != 1 || len(table.Cases) != len(wantNames) || table.Scope == "" {
		t.Fatal("changed closed input table")
	}
	actions := p15FTCompile(t, p15RP26Source(t, "value"))
	if _, err := p15RP26Graph(actions, "value"); err != nil {
		t.Fatal(err)
	}
	witness, err := p15FEActivationWitness(actions)
	if err != nil {
		t.Fatal(err)
	}
	for index, item := range table.Cases {
		if item.Name != wantNames[index] || item.Dispatch != (item.Name == "valid_private_markers") {
			t.Fatal("changed input identity or expected branch", item.Name)
		}
		t.Run(item.Name, func(t *testing.T) {
			selected, matched := true, 0
			for _, guard := range witness.Guards {
				if guard.Field != "_cherri_magic" && guard.Field != "cherri_functions" {
					continue
				}
				matched++
				value, ok := item.Input[guard.Field].(string)
				selected = selected && guard.Then && ok && value == guard.Value
			}
			if matched != 2 || selected != item.Dispatch {
				t.Fatalf("actual emitted marker predicates select=%t want=%t, matched=%d", selected, item.Dispatch, matched)
			}
		})
	}
	t.Log("Only exact emitted marker selection over seven fixed inputs is observed locally; malformed complete-envelope fields and Apple dispatch remain separate obligations.")
}
