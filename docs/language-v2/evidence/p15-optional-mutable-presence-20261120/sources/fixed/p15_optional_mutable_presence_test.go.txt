package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
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

func TestP15OptionalCompoundAssignmentRequiresPresentOperands(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"absent_target", "function echo(seed:Text)->Number?{var candidate:Number?=none;candidate+=1;return candidate}\ngetVariable(echo(\"unused\"))"},
		{"absent_rhs", "function echo(seed:Text)->Number?{var candidate:Number?=0;candidate+=none;return candidate}\ngetVariable(echo(\"unused\"))"},
		{"optional_target", "function echo(value:Number?)->Number?{var candidate:Number?=value;candidate+=1;return candidate}\ngetVariable(echo(none))"},
		{"optional_rhs", "function echo(value:Number?)->Number?{var candidate:Number?=0;candidate+=value;return candidate}\ngetVariable(echo(none))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeArgumentType || !strings.Contains(diagnostics[0].Message, "compound assignment requires present operands") {
				t.Fatal("possibly absent arithmetic must not invent value presence", diagnostics)
			}
			if raw, err := CompileSourceToPlist("p15-optional-compound.cherri", tc.source); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("absent compound operand emitted native bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

func p15MutableOptionalCompoundError(actions []map[string]any, narrowed bool, tc p15FunctionOptionalCase) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	flag, guard := "", ""
	if narrowed {
		value, presence := p15DecodeOptionalGraph(g, envelope, "has_value_0", "arg_0")
		flag = p15MutablePresenceName(g)
		g.emit("setvariable", map[string]any{"WFVariableName": flag, "WFInput": p15FunctionEdge(presence)})
		g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
		guard = g.conditional(5, p15FunctionVariable(flag), "WFConditionalActionString", "0")
	} else {
		g.get("arg_0", envelope)
		flag = p15MutableOptionalAssignGraph(g, "", true, p15MutableOptionalPresentValues()[2])
	}
	present := g.value("gettext", map[string]any{"WFTextActionText": "1"})
	g.emit("setvariable", map[string]any{"WFVariableName": flag, "WFInput": p15FunctionEdge(present)})
	value := g.value("math", map[string]any{"WFInput": p15FunctionVariable("candidate"), "WFMathOperation": "+", "WFMathOperand": uint64(1)})
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
	if narrowed {
		g.boundary(guard, 2, false)
	}
	resultGroup := g.conditional(4, p15FunctionVariable(flag), "WFConditionalActionString", "1")
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", true, p15FunctionArgument{true, 0, p15FunctionVariable("candidate")})
	g.boundary(resultGroup, 1, false)
	p15OptionalCarrierGraph(g, "has_value", "value", "_cherri_optional_result_v1", false, p15FunctionArgument{})
	g.output(g.boundary(resultGroup, 2, true))
	g.caller()
	argument := ""
	if narrowed {
		argument = p15FunctionOptionalPayload(g, tc)
	} else {
		argument = g.payload("echo", p15FunctionString("unused"))
	}
	result := g.call(argument)
	result, _ = p15DecodeOptionalGraph(g, result, "has_value", "value")
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalMutableCompoundKeepsNarrowedPresence(t *testing.T) {
	for _, route := range []string{"present_literal", "narrowed_none", "narrowed_zero"} {
		t.Run(route, func(t *testing.T) {
			narrowed := route != "present_literal"
			tc := p15FunctionOptionalCases()[3]
			source := "function echo(seed:Text)->Number?{var candidate:Number?=0;candidate+=1;return candidate}\ngetVariable(echo(\"unused\"))"
			if narrowed {
				if route == "narrowed_none" {
					tc = p15FunctionOptionalCase{name: "none", typ: "Number", argument: "none", supplied: true}
				}
				source = "function echo(value:Number?)->Number?{var candidate:Number?=value;if candidate!=none{candidate+=1};return candidate}\ngetVariable(echo(" + tc.argument + "))"
			}
			if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
				t.Fatal(diagnostics)
			}
			if err := p15MutableOptionalCompoundError(p15CompiledActions(t, source), narrowed, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15MutableOptionalCopyError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	tc := p15MutableOptionalPresentValues()[0]
	original := p15MutableOptionalAssignGraph(g, "", false, tc)
	copied := p15MutablePresenceName(g)
	if copied == original {
		g.err = fmt.Errorf("independent mutable values share one presence variable")
	}
	g.emit("setvariable", map[string]any{"WFVariableName": copied, "WFInput": p15FunctionVariable(original)})
	g.emit("setvariable", map[string]any{"WFVariableName": "kept", "WFInput": p15FunctionVariable("candidate")})
	p15MutableOptionalAssignGraph(g, original, true, tc)
	group := g.conditional(4, p15FunctionVariable(copied), "WFConditionalActionString", "0")
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(1)})
	g.boundary(group, 1, false)
	g.emit("number", map[string]any{"WFNumberActionNumber": float64(0)})
	g.output(g.boundary(group, 2, true))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalMutableCopyHasIndependentPresence(t *testing.T) {
	source := "function echo(seed:Text)->Bool{var candidate:Text?=none;var kept:Text?=candidate;candidate=\"\";return kept==none}\ngetVariable(echo(\"unused\"))"
	if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	actions := p15CompiledActions(t, source)
	if err := p15MutableOptionalCopyError(actions); err != nil {
		t.Fatal(err)
	}
	bad := p15ContainerCloneNative(t, actions)
	if err := p15MutableOptionalCopyError(bad); err != nil {
		t.Fatal("unchanged typed clone must pass", err)
	}
	names := []string{}
	for _, action := range bad {
		if name, ok := p15Param(action)["WFVariableName"].(string); ok && strings.HasPrefix(name, "_cherri_optional_presence_") {
			if len(names) == 0 || name != names[0] {
				names = append(names, name)
			}
		}
	}
	if len(names) != 2 {
		t.Fatal("independent presence identities missing", names)
	}
	var replace func(any)
	replace = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if text, ok := child.(string); ok && text == names[1] {
					v[key] = names[0]
				} else {
					replace(child)
				}
			}
		case []any:
			for _, child := range v {
				replace(child)
			}
		}
	}
	for _, action := range bad {
		replace(action)
	}
	if p15MutableOptionalCopyError(bad) == nil {
		t.Fatal("shared mutable presence identities accepted")
	}
}

func p15MutableOptionalImplicitInputError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("echo")
	g.get("arg_0", envelope)
	g.emit("gettext", map[string]any{"WFTextActionText": "OPTIONAL_PREVIOUS_SEED"})
	authored := g.value("detect.text", map[string]any{"UUID": "alpha"})
	present := g.value("gettext", map[string]any{"WFTextActionText": "1"})
	flag := p15MutablePresenceName(g)
	g.emit("setvariable", map[string]any{"WFVariableName": flag, "WFInput": p15FunctionEdge(present)})
	value := g.value("gettext", map[string]any{"WFTextActionText": p15FunctionText("\ufffc", authored)})
	g.emit("setvariable", map[string]any{"WFVariableName": "candidate", "WFInput": p15FunctionEdge(value)})
	g.emit("detect.text", nil)
	p15MutableOptionalAssignGraph(g, flag, false, p15MutableOptionalPresentValues()[0])
	g.emit("detect.text", nil)
	g.output(g.value("gettext", map[string]any{"WFTextActionText": "DONE"}))
	g.caller()
	result := g.call(g.payload("echo", p15FunctionString("unused")))
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	return g.finish()
}

func TestP15OptionalMutableMetadataPreservesImplicitInputOrder(t *testing.T) {
	source := `function echo(seed:Text)->Text{
text("OPTIONAL_PREVIOUS_SEED")
var candidate:Text?=f"{native.action("is.workflow.actions.detect.text", {})}"
native.action("is.workflow.actions.detect.text", {})
candidate=none
native.action("is.workflow.actions.detect.text", {})
return "DONE"
}
getVariable(echo("unused"))`
	if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
		t.Fatal(diagnostics)
	}
	actions := p15CompiledActions(t, source)
	if err := p15MutableOptionalImplicitInputError(actions); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"metadata_before_rhs", "metadata_after_value", "lost_clear"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15ContainerCloneNative(t, actions)
			if err := p15MutableOptionalImplicitInputError(bad); err != nil {
				t.Fatal("unchanged typed clone must pass", err)
			}
			first, last, write := -1, -1, -1
			for i, action := range bad {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.text" {
					if first < 0 {
						first = i
					}
					last = i
				}
				if p15Param(action)["WFVariableName"] == "candidate" && write < 0 {
					write = i
				}
			}
			if first < 0 || last <= first || write <= first {
				t.Fatal("implicit-input control target missing")
			}
			switch fault {
			case "metadata_before_rhs":
				bad[first], bad[first+1] = bad[first+1], bad[first]
			case "metadata_after_value":
				metadata := bad[write-2]
				copy(bad[write-2:write], bad[write-1:write+1])
				bad[write] = metadata
			case "lost_clear":
				bad = append(bad[:last-2], bad[last-1:]...)
			}
			if p15MutableOptionalImplicitInputError(bad) == nil {
				t.Fatal("metadata changed an authored implicit input without rejection")
			}
		})
	}
}
