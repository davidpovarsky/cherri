package main

import (
	"fmt"
	"strings"
	"testing"
)

// A mutable Optional needs a distinct runtime flag. The variable identity is
// alpha-renamable, but it may not alias the authored value or dispatcher state.
func p15MutablePresenceName(g *p15FunctionGraph) string {
	index := len(g.expected)
	if index >= len(g.actual) {
		g.err = fmt.Errorf("missing paired Optional presence assignment")
		return ""
	}
	name, _ := p15Param(g.actual[index])["WFVariableName"].(string)
	if !strings.HasPrefix(name, "_cherri_optional_presence_") || len(name) <= len("_cherri_optional_presence_") || name == "candidate" {
		g.err = fmt.Errorf("Optional presence has no independent private variable")
	}
	return name
}

func p15MutableOptionalReturnError(actions []map[string]any, tc p15FunctionOptionalCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	value, presence := p15DecodeOptionalGraph(g, envelope, "has_value_0", "arg_0")
	flag := p15MutablePresenceName(g)
	g.emit("setvariable", map[string]any{"WFVariableName": flag, "WFInput": p15FunctionEdge(presence)})
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
	resultGroup := g.conditional(4, p15FunctionVariable(flag), "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionVariable("candidate")})
	g.boundary(resultGroup, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	g.output(g.boundary(resultGroup, 2, true))
	g.caller()
	result := g.call(p15FunctionOptionalPayload(g, tc))
	unboxed, _ := p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(unboxed)})
	return g.finish()
}

func TestP15OptionalMutableBindingsKeepPresence(t *testing.T) {
	for _, tc := range p15FunctionOptionalCases()[:6] {
		t.Run(tc.name, func(t *testing.T) {
			source := strings.Replace(p15FunctionOptionalSource(tc), "{return value}", "{var candidate:"+tc.typ+"?=value;return candidate}", 1)
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if err := p15MutableOptionalReturnError(p15CompiledActions(t, source), tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

type p15MutableOptionalValue struct {
	name, typ, literal, action, key string
	value                           any
}

func p15MutableOptionalPresentValues() []p15MutableOptionalValue {
	return []p15MutableOptionalValue{
		{"empty", "Text", `""`, "gettext", "WFTextActionText", ""},
		{"false", "Bool", "false", "number", "WFNumberActionNumber", float64(0)},
		{"zero", "Number", "0", "number", "WFNumberActionNumber", uint64(0)},
	}
}

func p15MutableOptionalAssignGraph(g *p15FunctionGraph, flag string, present bool, tc p15MutableOptionalValue) string {
	text := "0"
	if present {
		text = "1"
	}
	producer := g.value("gettext", map[string]any{"WFTextActionText": text})
	if flag == "" {
		flag = p15MutablePresenceName(g)
	}
	g.emit("setvariable", map[string]any{"WFVariableName": flag, "WFInput": p15FunctionEdge(producer)})
	if !present {
		g.emit("nothing", nil)
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate"})
	} else {
		value := g.value(tc.action, map[string]any{tc.key: tc.value})
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
	}
	return flag
}

func p15MutableOptionalTransitionError(actions []map[string]any, tc p15MutableOptionalValue, toPresent bool) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	flag := p15MutableOptionalAssignGraph(g, "", !toPresent, tc)
	p15MutableOptionalAssignGraph(g, flag, toPresent, tc)
	group := g.conditional(5, p15FunctionVariable(flag), "WFConditionalActionString", "0")
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
	g.output(g.boundary(group, 2, true))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalMutableAssignmentsUpdatePresence(t *testing.T) {
	for _, tc := range p15MutableOptionalPresentValues() {
		for _, toPresent := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/to_present_%v", tc.name, toPresent), func(t *testing.T) {
				before, after := tc.literal, "none"
				if toPresent {
					before, after = after, before
				}
				source := "function echo(seed:Text)->Bool{var candidate:" + tc.typ + "?=" + before + ";candidate=" + after + ";return candidate!=none}\ngetVariable(echo(\"unused\"))"
				if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				if err := p15MutableOptionalTransitionError(p15CompiledActions(t, source), tc, toPresent); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func p15MutableOptionalSnapshotError(actions []map[string]any, returnValue bool) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	tc := p15MutableOptionalPresentValues()[0]
	flag := p15MutableOptionalAssignGraph(g, "", false, tc)
	oldPresence := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(flag)})
	oldValue := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("candidate")})
	p15MutableOptionalAssignGraph(g, flag, true, tc)
	if returnValue {
		group := g.conditional(4, p15FunctionEdge(oldPresence), "WFConditionalActionString", "1")
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionEdge(oldValue)})
		g.boundary(group, 1, false)
		p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
		g.output(g.boundary(group, 2, true))
	} else {
		group := g.conditional(4, p15FunctionEdge(oldPresence), "WFConditionalActionString", "0")
		g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
		g.boundary(group, 1, false)
		g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
		g.output(g.boundary(group, 2, true))
	}
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	if returnValue {
		result, _ = p15DecodeOptionalGraph(g, result, "has_value", "value")
	}
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalImmutableSnapshotCapturesMutablePresence(t *testing.T) {
	for _, returnValue := range []bool{false, true} {
		t.Run(fmt.Sprintf("return_value_%v", returnValue), func(t *testing.T) {
			result, expression := "Bool", "before==none"
			if returnValue {
				result, expression = "Text?", "before"
			}
			source := "function echo(seed:Text)->" + result + "{var candidate:Text?=none;let before=candidate;candidate=\"\";return " + expression + "}\ngetVariable(echo(\"unused\"))"
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			actions := p15CompiledActions(t, source)
			if err := p15MutableOptionalSnapshotError(actions, returnValue); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"live_presence", "data_as_presence", "absent_becomes_present", "stale_flag_write", "missing_snapshot"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15MutableOptionalSnapshotError(bad, returnValue); err != nil {
						t.Fatal("unchanged typed snapshot clone must pass", err)
					}
					flag, snapshot := "", -1
					writes, predicates := []int{}, []int{}
					for i, action := range bad {
						p := p15Param(action)
						if name, ok := p["WFVariableName"].(string); ok && strings.HasPrefix(name, "_cherri_optional_presence_") {
							flag = name
							writes = append(writes, i)
						}
						if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && snapshot < 0 {
							if token, ok := p["WFVariable"].(map[string]any); ok {
								if descriptor, ok := token["Value"].(map[string]any); ok && descriptor["VariableName"] == flag && flag != "" {
									snapshot = i
								}
							}
						}
						if p["WFConditionalActionString"] == "0" || (returnValue && p["WFConditionalActionString"] == "1" && i > snapshot && snapshot >= 0 && len(writes) == 2) {
							predicates = append(predicates, i)
						}
					}
					if flag == "" || snapshot < 0 || len(writes) != 2 || len(predicates) == 0 {
						t.Fatal("mutable presence corruption target missing")
					}
					p := p15Param(bad[predicates[0]])
					switch fault {
					case "live_presence":
						p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionVariable(flag)}
					case "data_as_presence":
						p["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15FunctionEdge(p15Param(bad[snapshot+1])["UUID"].(string))}
					case "absent_becomes_present":
						p15Param(bad[writes[0]-1])["WFTextActionText"] = "1"
					case "stale_flag_write":
						p15Param(bad[writes[1]])["WFInput"] = p15Param(bad[writes[0]])["WFInput"]
					case "missing_snapshot":
						bad = append(bad[:snapshot], bad[snapshot+1:]...)
					}
					if p15MutableOptionalSnapshotError(bad, returnValue) == nil {
						t.Fatal("damaged mutable Optional snapshot accepted")
					}
				})
			}
		})
	}
}
