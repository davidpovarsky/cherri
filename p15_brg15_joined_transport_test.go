package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
)

func p15BRG15Source(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("tests", "language-v2", "v2-brg15-joined-transport.cherri"))
	if err != nil || len(raw) == 0 {
		t.Fatal("missing distinct BRG15 source", err)
	}
	return string(raw)
}

func p15BRG15Rows() []any {
	return []any{false, p15ContainerNumber("0"), "", []any{}, map[string]any{}}
}

func p15BRG15Fallback() map[string]any {
	return map[string]any{"kind": "DEFAULT", "rows": p15BRG15Rows()}
}

func p15BRG15Supplied() map[string]any {
	return map[string]any{
		"cherri_functions": "1", "function": "recurse", "arguments": []any{},
		"rows": p15BRG15Rows(), "WFSerializationType": "WFTextTokenAttachment", "Value": false,
	}
}

// BRG15 owns a complete joined graph. The recursive call and the outer call
// transport the same Map by typed producer edges; no related test supplies it.
func p15BRG15Graph(actions []map[string]any) (*p15FTCursor, error) {
	c := p15FTNew(actions)
	c.header()
	caller := p15SCVariable("_cherri_caller_input")
	c.beginFunction("recurse")
	c.mark("depth_slot")
	depth := c.field("arg_0", c.envelope, "depth")
	depth = c.numberCast(depth, "depth")
	c.mark("recursive_value_slot")
	value := c.field("arg_1", c.envelope, "value")
	c.mark("base_guard")
	base := c.condition(depth, 1, "WFNumberValue", float64(0))
	c.mark("base_return")
	c.output(value)
	c.end(base, "", false)
	c.mark("decrement")
	previous := c.math("-", depth, uint64(1), "")
	c.mark("recursive_payload")
	payload := c.payload("recurse",
		p15FTArgument{true, p15ContainerReference(previous)},
		p15FTArgument{true, p15ContainerReference(value)})
	c.mark("recursive_call")
	nested := c.call(payload, "nested")
	c.mark("recursive_return")
	c.output(nested)
	c.beginFunction("route")
	c.mark("supplied_flag")
	supplied := c.field("has_arg_0", c.envelope, "")
	c.mark("supplied_guard")
	branch := c.condition(supplied, 4, "WFConditionalActionString", "1")
	c.mark("supplied_value")
	c.field("arg_0", c.envelope, "")
	c.mark("default_else")
	c.alternate(branch)
	c.mark("default_value")
	c.dictionary("Dictionary", p15BRG15Fallback())
	c.mark("default_join")
	value = c.end(branch, "value", true)
	c.mark("nested_payload")
	payload = c.payload("recurse",
		p15FTArgument{true, p15ContainerNumber("2")},
		p15FTArgument{true, p15ContainerReference(value)})
	c.mark("nested_call")
	result := c.call(payload, "result")
	c.mark("route_result")
	result = c.dictionary("Dictionary", map[string]any{
		"value":  p15ContainerReference(result),
		"caller": p15ContainerReference(caller), "shortcut": p15ContainerReference(caller),
	})
	c.output(result)
	c.caller()
	c.mark("before")
	before := c.take("gettext", "before", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
	results := map[string]map[string]any{}
	for _, name := range []string{"omitted", "empty", "supplied"} {
		argument := p15FTArgument{}
		if name == "empty" {
			argument = p15FTArgument{true, map[string]any{}}
		}
		if name == "supplied" {
			argument = p15FTArgument{true, p15BRG15Supplied()}
		}
		c.mark(name + "_payload")
		payload = c.payload("route", argument)
		c.mark(name + "_call")
		results[name] = c.call(payload, name)
	}
	c.mark("after")
	after := c.take("gettext", "after", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
	c.mark("observation")
	result = c.dictionary("Dictionary", map[string]any{
		"omitted": p15ContainerReference(results["omitted"]), "empty": p15ContainerReference(results["empty"]),
		"supplied": p15ContainerReference(results["supplied"]),
		"before":   p15ContainerReference(before), "after": p15ContainerReference(after),
	})
	c.mark("main_return")
	c.output(result)
	if err := c.finish(); err != nil {
		return c, err
	}
	if _, err := p15FEActivationWitness(actions); err != nil {
		return c, err
	}
	return c, nil
}

func TestBackendRecovery15JoinedTransportUnit(t *testing.T) {
	source := p15BRG15Source(t)
	p15FunctionTypedProgram(t, source, map[string]string{
		"depth <= 0": "Bool", "depth - 1": "Number",
		"recurse(depth - 1, value)": "Map<Text, AnyContent>",
		"recurse(2, value)":         "Map<Text, AnyContent>",
		"route()":                   "Map<Text, AnyContent>", "route({})": "Map<Text, AnyContent>",
	})
	for _, item := range []struct{ name, source, target, code, message string }{
		{"wrong_recursive_argument",
			"function recurse(depth:Number,value:Map<Text,Number>)->Map<Text,Number>{if depth<=0{return value};return recurse(depth-1,\"bad\")}\nreturn recurse(2,{})",
			"\"bad\"", analysis.CodeArgumentType, "Map<Text, Number>"},
		{"wrong_default",
			"function route(value:Map<Text,Number>=\"bad\")->Map<Text,Number>{return value}\nreturn route()",
			"\"bad\"", analysis.CodeArgumentType, "default value of type"},
		{"wrong_recursive_return",
			"function recurse(depth:Number,value:Map<Text,Number>)->Map<Text,Number>{if depth<=0{return \"bad\"};return recurse(depth-1,value)}\nreturn recurse(2,{})",
			"return \"bad\"", analysis.CodeReturnType, "Map<Text, Number>"},
	} {
		t.Run(item.name, func(t *testing.T) {
			p15FunctionRejects(t, item.source, item.target, item.code, item.message)
		})
	}
	t.Log("BRG15 semantic joined-scenario observations only; native graph and runtime values have separate producers.")
}

var p15BRG15Faults = []string{
	"magic_guard", "marker_guard", "recursive_slot", "base_predicate", "base_threshold",
	"base_result", "recursive_decrement", "recursive_argument", "recursive_caller",
	"recursive_callee", "recursive_self_target", "recursive_result",
	"presence_reads_value", "presence_truthiness", "default_branch", "default_false_kind",
	"default_empty_shape", "nested_depth", "nested_caller", "route_caller",
	"omitted_marked_present", "empty_marked_absent", "supplied_shape", "supplied_false_kind",
	"supplied_caller", "main_callee", "before_input", "after_input", "final_consumer", "main_return",
}

func TestBackendRecovery15JoinedTransportNative(t *testing.T) {
	actions := p15FTCompile(t, p15BRG15Source(t))
	view, err := p15BRG15Graph(actions)
	if err != nil {
		t.Fatal(err)
	}
	verify := func(a []map[string]any) error { _, err := p15BRG15Graph(a); return err }
	p15FTCommonControls(t, actions, verify)
	for _, fault := range p15BRG15Faults {
		t.Run("oracle/"+fault, func(t *testing.T) {
			bad := p15SCClone(t, actions)
			transport := p15ContainerReference(map[string]any{"Type": "ExtensionInput"})
			switch fault {
			case "magic_guard":
				p15SCParams(bad[7])["WFConditionalActionString"] = "USER"
			case "marker_guard":
				p15SCParams(bad[8])["WFCondition"] = uint64(100)
			case "recursive_slot":
				p15SCParams(bad[view.points["recursive_value_slot"]])["WFDictionaryKey"] = "arg_0"
			case "base_predicate":
				p15SCParams(bad[view.points["base_guard"]])["WFCondition"] = uint64(4)
			case "base_threshold":
				p15SCParams(bad[view.points["base_guard"]])["WFNumberValue"] = float64(1)
			case "base_result":
				p15SCParams(bad[view.points["base_return"]])["WFOutput"] = p15ContainerText("BASE")
			case "recursive_decrement":
				p15SCParams(bad[view.points["decrement"]])["WFMathOperation"] = "+"
			case "recursive_argument":
				p15FN05Fields(t, bad, view.points["recursive_payload"])["arg_1"]["WFValue"] = p15ContainerText("LOST")
			case "recursive_caller", "nested_caller", "supplied_caller":
				point := strings.TrimSuffix(fault, "_caller") + "_payload"
				p15FN05Fields(t, bad, view.points[point])["_cherri_caller_input"]["WFValue"] = transport
			case "recursive_callee":
				p15FN05Fields(t, bad, view.points["recursive_payload"])["function"]["WFValue"] = p15ContainerText("route")
			case "recursive_self_target":
				p15SCParams(bad[view.points["recursive_call"]])["WFWorkflow"].(map[string]any)["isSelf"] = false
			case "recursive_result":
				p15SCParams(bad[view.points["recursive_return"]])["WFOutput"] = p15ContainerText("LOST")
			case "presence_reads_value":
				p15SCParams(bad[view.points["supplied_flag"]])["WFDictionaryKey"] = "arg_0"
			case "presence_truthiness":
				p15SCParams(bad[view.points["supplied_guard"]])["WFCondition"] = uint64(100)
			case "default_branch":
				p15SCParams(bad[view.points["default_else"]])["WFControlFlowMode"] = uint64(2)
			case "default_false_kind", "default_empty_shape":
				value := p15BRG15Fallback()
				rows := value["rows"].([]any)
				if fault == "default_false_kind" {
					rows[0] = p15ContainerNumber("0")
				} else {
					rows[3] = map[string]any{}
				}
				_, wire := p15ContainerFieldValue(value)
				p15SCParams(bad[view.points["default_value"]])["WFItems"] = wire
			case "nested_depth":
				p15FN05Fields(t, bad, view.points["nested_payload"])["arg_0"]["WFValue"] = p15ContainerText("3")
			case "route_caller":
				p15FN05Fields(t, bad, view.points["route_result"])["caller"]["WFValue"] = transport
			case "omitted_marked_present":
				p15FN05Fields(t, bad, view.points["omitted_payload"])["has_arg_0"]["WFValue"] = p15ContainerText("1")
			case "empty_marked_absent":
				p15FN05Fields(t, bad, view.points["empty_payload"])["has_arg_0"]["WFValue"] = p15ContainerText("0")
			case "supplied_shape", "supplied_false_kind":
				value := p15BRG15Supplied()
				if fault == "supplied_shape" {
					value["arguments"] = map[string]any{}
				} else {
					value["Value"] = p15ContainerNumber("0")
				}
				_, wire := p15ContainerFieldValue(value)
				p15FN05Fields(t, bad, view.points["supplied_payload"])["arg_0"]["WFValue"] = wire
			case "main_callee":
				p15FN05Fields(t, bad, view.points["empty_payload"])["function"]["WFValue"] = p15ContainerText("recurse")
			case "before_input", "after_input":
				point := strings.TrimSuffix(fault, "_input")
				p15SCParams(bad[view.points[point]])["WFTextActionText"] = p15FEHealthyText(map[string]any{"Type": "ExtensionInput"}, map[string]any{"Type": "ExtensionInput"})
			case "final_consumer":
				p15FN05Fields(t, bad, view.points["observation"])["empty"]["WFValue"] = p15ContainerText("LOST")
			case "main_return":
				p15SCParams(bad[view.points["main_return"]])["WFOutput"] = p15ContainerText("LOST")
			}
			if reflect.DeepEqual(actions, bad) {
				t.Fatal("mutation missed its actual target", fault)
			}
			if verify(bad) == nil {
				t.Fatal("joined BRG15 transport corruption accepted", fault)
			}
		})
	}
	t.Logf("BRG15 complete native structure has %d actual actions; no recursive values, caller continuation or user-input behavior on Apple is inferred.", len(actions))
}

func TestBackendRecovery15LoweringFailurePropagation(t *testing.T) {
	base := p15BRG15Source(t)
	if backendEmitHook != nil {
		t.Fatal("BRG15 does not own an existing emitter hook")
	}
	for _, item := range []struct{ name, anchor, context string }{
		{"recursive_body", "    if depth <= 0 { return value }", "function recurse:"},
		{"nested_body", "    let result = recurse(2, value)", "function route:"},
		{"caller_body", "let before = f\"{system.input}|{ShortcutInput}\"", "lowering error:"},
	} {
		t.Run(item.name, func(t *testing.T) {
			if strings.Count(base, item.anchor) != 1 {
				t.Fatal("lowering-fault source anchor changed")
			}
			literal := "BRG15_LOWERING_FAILURE_" + item.name
			source := strings.Replace(base, item.anchor, "text(\""+literal+"\")\n"+item.anchor, 1)
			sentinel := errors.New(literal)
			calls, matched := 0, false
			previous := backendEmitHook
			defer func() { backendEmitHook = previous }()
			backendEmitHook = func(call backend.ResolvedCall) error {
				calls++
				matched = len(call.Arguments) == 1 && call.Arguments[0].Value.Kind == backend.ValString && call.Arguments[0].Value.StrVal == literal
				if !matched {
					return fmt.Errorf("BRG15 unexpected injection call: %s", call.DefinitionID)
				}
				return sentinel
			}
			raw, err := CompileSourceToPlist("p15-brg15-lowering-failure.cherri", source)
			backendEmitHook = previous
			if len(raw) != 0 || !errors.Is(err, sentinel) || !strings.Contains(err.Error(), item.context) || calls != 1 || !matched {
				t.Fatalf("lowering failure lost cause/context or emitted partial output: calls=%d matched=%t bytes=%d error=%v", calls, matched, len(raw), err)
			}
			// A subsequent actual canonical compile proves transaction ownership
			// was released and no partially emitted graph survived the failure.
			healthy := p15FTCompile(t, base)
			if _, err := p15BRG15Graph(healthy); err != nil {
				t.Fatal("canonical recovery differs", err)
			}
			t.Logf("BRG15 lowering site=%s calls=%d partial_bytes=%d cause_preserved=true recovery_graph_verified=true", item.name, calls, len(raw))
		})
	}
}
