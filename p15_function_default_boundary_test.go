package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// These expectations specify authored values, complete native graphs and
// presence decisions. No Apple execution or final T01 acceptance is inferred.
type p15TypedDefaultCase struct {
	name, annotation, supplied, fallback string
	argument                             p15FunctionArgument
	defaultValue                         any
	numeric                              bool
}

func p15TypedDefaultCases() []p15TypedDefaultCase {
	list := func(value []any) p15FunctionArgument {
		kind, wire := p15ContainerFieldValue(value)
		return p15FunctionArgument{true, int(kind), wire}
	}
	dict := func(value map[string]any) p15FunctionArgument {
		kind, wire := p15ContainerFieldValue(value)
		return p15FunctionArgument{true, int(kind), wire}
	}
	return []p15TypedDefaultCase{
		{"false", "Bool", "false", "true", p15FunctionNumber("0"), true, true},
		{"zero", "Number", "0", "9", p15FunctionNumber("0"), p15ContainerNumber("9"), true},
		{"empty_text", "Text", `""`, `"DEFAULT"`, p15FunctionString(""), "DEFAULT", false},
		{"empty_list", "List<Text>", "[]", `["DEFAULT"]`, list([]any{}), []any{"DEFAULT"}, false},
		{"empty_map", "Map<Text,Number>", "{}", `{"DEFAULT":9}`, dict(map[string]any{}), map[string]any{"DEFAULT": p15ContainerNumber("9")}, false},
		{"empty_list_default", "List<Text>", `["SUPPLIED"]`, "[]", list([]any{"SUPPLIED"}), []any{}, false},
		{"empty_map_default", "Map<Text,Number>", `{"SUPPLIED":7}`, "{}", dict(map[string]any{"SUPPLIED": p15ContainerNumber("7")}), map[string]any{}, false},
		{"reserved_map_default", "Map<Text,AnyContent>", "{}", `{"WFSerializationType":"WFTextTokenString","Value":"ordinary"}`, dict(map[string]any{}), map[string]any{"WFSerializationType": "WFTextTokenString", "Value": "ordinary"}, false},
		{"nested_empty_default", "Map<Text,AnyContent>", "{}", `{"list":[],"map":{}}`, dict(map[string]any{}), map[string]any{"list": []any{}, "map": map[string]any{}}, false},
	}
}

func p15TypedDefaultSource(tc p15TypedDefaultCase) string {
	return fmt.Sprintf("function echo(value:%s = %s)->AnyContent{return value}\nlet supplied=echo(%s)\ngetVariable(supplied)\nlet omitted=echo()\ngetVariable(omitted)\nlet named=echo(value:%s)\ngetVariable(named)", tc.annotation, tc.fallback, tc.supplied, tc.supplied)
}

func p15TypedDefaultValueGraph(g *p15FunctionGraph, value any) {
	switch v := value.(type) {
	case bool:
		n := 0
		if v {
			n = 1
		}
		g.value("number", map[string]any{"WFNumberActionNumber": n})
	case p15ContainerNumber:
		// The authored fallback is deliberately distinct from the supplied zero.
		g.value("number", map[string]any{"WFNumberActionNumber": 9})
	case string:
		g.value("gettext", map[string]any{"WFTextActionText": v})
	case []any:
		if len(v) == 0 {
			p15TypedEmptyDefaultGraph(g)
			return
		}
		rows := []any{}
		for _, item := range v {
			rows = append(rows, map[string]any{"WFItemType": 0, "WFValue": item})
		}
		g.value("list", map[string]any{"WFItems": rows})
	case map[string]any:
		_, wire := p15ContainerFieldValue(v)
		g.value("dictionary", map[string]any{"WFItems": wire})
	default:
		g.err = fmt.Errorf("unmodeled authored fallback %T", value)
	}
}

// Empty runtime Lists already use the verified clear/Get Variable bridge.
// The private name and UUID are alpha identities; every operation, missing
// Set input, consumed edge and branch position remains independently authored.
func p15TypedEmptyDefaultGraph(g *p15FunctionGraph) {
	index := len(g.expected)
	if index+2 >= len(g.actual) {
		g.err = fmt.Errorf("missing empty default clear/producer sequence")
		return
	}
	name, _ := p15C01Params(g.actual[index+1])["WFVariableName"].(string)
	uuid, _ := p15C01Params(g.actual[index+2])["UUID"].(string)
	if uuid == "" || !strings.HasPrefix(name, "__cherri_scope_") || !strings.HasSuffix(name, "_emptyList_"+uuid) {
		g.err = fmt.Errorf("empty default lacks a private producer-bound identity")
	}
	g.emit("nothing", nil)
	g.emit("setvariable", map[string]any{"WFVariableName": name})
	g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(name)})
}

func p15TypedDefaultGraph(actions []map[string]any, tc p15TypedDefaultCase) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	positions := map[string]int{}
	envelope := g.header("echo")
	positions["presence"] = len(g.expected)
	present := g.get("has_arg_0", envelope)
	positions["guard"] = len(g.expected)
	guard := g.conditional(4, p15FunctionEdge(present), "WFConditionalActionString", "1")
	positions["value"] = len(g.expected)
	g.get("arg_0", envelope)
	positions["else"] = len(g.expected)
	g.boundary(guard, 1, false)
	positions["default"] = len(g.expected)
	p15TypedDefaultValueGraph(g, tc.defaultValue)
	positions["default_result"] = len(g.expected) - 1
	positions["join"] = len(g.expected)
	value := g.boundary(guard, 2, true)
	if tc.numeric {
		value = g.numberCast(value)
	}
	positions["return"] = len(g.expected)
	g.output(value)
	g.caller()
	for _, call := range []struct {
		name string
		arg  p15FunctionArgument
	}{{"supplied", tc.argument}, {"omitted", p15FunctionArgument{}}, {"named", tc.argument}} {
		positions[call.name] = len(g.expected)
		result := g.call(g.payload("echo", call.arg))
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	}
	return positions, g.finish()
}

func p15FunctionBoundaryRetain(t *testing.T, name string, actions []map[string]any) {
	t.Helper()
	if root := os.Getenv("CHERRI_FUNCTION_BOUNDARY_DIAGNOSTICS"); root != "" {
		if err := os.MkdirAll(root, 0700); err != nil {
			t.Fatal(err)
		}
		data, err := json.MarshalIndent(actions, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, name+".json"), append(data, '\n'), 0600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestP15T01TypedDefaultBoundaryGraphs(t *testing.T) {
	for _, tc := range p15TypedDefaultCases() {
		t.Run(tc.name, func(t *testing.T) {
			source := p15TypedDefaultSource(tc)
			p15FunctionTypedProgram(t, source, map[string]string{
				"echo(" + tc.supplied + ")": "AnyContent", "echo()": "AnyContent", "echo(value:" + tc.supplied + ")": "AnyContent",
			})
			actions := p15CompiledActions(t, source)
			p15FunctionBoundaryRetain(t, "typed-default-"+tc.name, actions)
			if _, err := p15TypedDefaultGraph(actions, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15T01TypedDefaultBoundaryCorruptions(t *testing.T) {
	for _, tc := range p15TypedDefaultCases() {
		t.Run(tc.name, func(t *testing.T) {
			original := p15CompiledActions(t, p15TypedDefaultSource(tc))
			positions, err := p15TypedDefaultGraph(original, tc)
			if err != nil {
				t.Fatal(err)
			}
			faults := []string{"supplied_absent", "named_absent", "omitted_present", "truthiness_guard", "read_value_as_presence", "wrong_value_slot", "wrong_join", "default_branch_lost", "default_kind_lost", "caller_input_lost", "return_uses_default", "caller_consumer_detached"}
			if list, emptyList := tc.defaultValue.([]any); emptyList && len(list) == 0 {
				faults = append(faults, "empty_clear_lost", "empty_inherits_input", "empty_read_detached")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					bad := p15C01Clone(t, original)
					switch fault {
					case "supplied_absent", "named_absent":
						name := "supplied"
						if fault == "named_absent" {
							name = "named"
						}
						p15FunctionPayloadRow(t, bad, positions[name], "has_arg_0")["WFValue"] = p15FunctionText("0")
					case "omitted_present":
						p15FunctionPayloadRow(t, bad, positions["omitted"], "has_arg_0")["WFValue"] = p15FunctionText("1")
					case "truthiness_guard":
						p15C01Params(bad[positions["guard"]])["WFCondition"] = 100
					case "read_value_as_presence":
						p15C01Params(bad[positions["presence"]])["WFDictionaryKey"] = "arg_0"
					case "wrong_value_slot":
						p15C01Params(bad[positions["value"]])["WFDictionaryKey"] = "has_arg_0"
					case "wrong_join":
						p15C01Params(bad[positions["join"]])["GroupingIdentifier"] = p15C01Params(bad[7])["GroupingIdentifier"]
					case "default_branch_lost":
						p15C01Params(bad[positions["else"]])["WFControlFlowMode"] = 2
					case "default_kind_lost":
						bad[positions["default_result"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.nothing"
					case "caller_input_lost":
						p15FunctionPayloadRow(t, bad, positions["supplied"], "_cherri_caller_input")["WFValue"] = p15FunctionText("FORGED")
					case "return_uses_default":
						p15C01Params(bad[positions["return"]])["WFOutput"] = p15FunctionText("\ufffc", p15C01Params(bad[positions["default_result"]])["UUID"].(string))
					case "empty_clear_lost":
						bad[positions["default"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.getvariable"
					case "empty_inherits_input":
						p15C01Params(bad[positions["default"]+1])["WFInput"] = p15FunctionText("STALE")
					case "empty_read_detached":
						p15C01Params(bad[positions["default_result"]])["WFVariable"] = p15FunctionVariable("unrelated")
					case "caller_consumer_detached":
						p15C01Params(bad[positions["supplied"]+2])["WFVariable"] = p15FunctionVariable("unrelated")
					}
					if _, err := p15TypedDefaultGraph(bad, tc); err == nil {
						t.Fatal("corrupted default/argument graph accepted")
					}
				})
			}
		})
	}
}

func TestP15T01WrongTypedDefaultsRejected(t *testing.T) {
	for _, tc := range []struct{ name, annotation, value string }{
		{"bool", "Bool", `"false"`}, {"number", "Number", `"0"`}, {"text", "Text", "false"},
		{"list", "List<Text>", "[1]"}, {"map", "Map<Text,Number>", `{"key":false}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := fmt.Sprintf("function bad(value:%s = %s)->AnyContent{return value}\nbad()", tc.annotation, tc.value)
			p15FunctionRejects(t, source, tc.value, analysis.CodeArgumentType, "default value of type")
		})
	}
}
