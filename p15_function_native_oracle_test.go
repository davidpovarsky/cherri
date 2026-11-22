package main

import (
	"fmt"
	"reflect"
	"sort"
)

// This oracle authors the required native operations and edges independently
// from the compiler. UUIDs and group labels are alpha names only; all semantic
// parameters, control nesting, argument types and final consumers are compared.
// It inspects serialization. It does not execute Apple's runtime.
type p15FunctionGraph struct {
	actual, expected []map[string]any
	groups           map[string]bool
	outer, body      string
	functionName     string
	workflowName     string
	err              error
}

func p15FunctionComparable(value any, parameters bool) any {
	switch v := value.(type) {
	case map[string]any:
		result := map[string]any{}
		for key, item := range v {
			if parameters && (key == "UUID" || key == "CustomOutputName") {
				continue
			}
			if v["Type"] == "ActionOutput" && key == "OutputName" {
				continue
			}
			result[key] = p15FunctionComparable(item, key == "WFWorkflowActionParameters")
		}
		return result
	case []any:
		result := make([]any, len(v))
		for i, item := range v {
			result[i] = p15FunctionComparable(item, false)
		}
		return result
	case int:
		return float64(v)
	case int64:
		return float64(v)
	case uint64:
		return float64(v)
	default:
		return value
	}
}

func p15FunctionAttachment(descriptor map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": descriptor}
}

func p15FunctionEdge(uuid string) map[string]any {
	return p15FunctionAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": uuid})
}

func p15FunctionVariable(name string) map[string]any {
	return p15FunctionAttachment(map[string]any{"Type": "Variable", "VariableName": name})
}

func p15FunctionText(literal string, refs ...string) map[string]any {
	value := map[string]any{"string": literal}
	if len(refs) != 0 {
		attachments := map[string]any{}
		for i, ref := range refs {
			attachments[fmt.Sprintf("{%d, 1}", i*2)] = map[string]any{"Type": "ActionOutput", "OutputUUID": ref}
		}
		value["attachmentsByRange"] = attachments
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": value}
}

func (g *p15FunctionGraph) emit(kind string, parameters map[string]any) string {
	index := len(g.expected)
	action := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + kind}
	if len(parameters) != 0 {
		action["WFWorkflowActionParameters"] = parameters
	}
	g.expected = append(g.expected, action)
	if index >= len(g.actual) {
		g.err = fmt.Errorf("missing native action %d (%s)", index, kind)
		return ""
	}
	uuid, _ := p15C01Params(g.actual[index])["UUID"].(string)
	return uuid
}

func (g *p15FunctionGraph) value(kind string, parameters map[string]any) string {
	uuid := g.emit(kind, parameters)
	if uuid == "" {
		g.err = fmt.Errorf("native %s result has no producer identity", kind)
	}
	return uuid
}

func (g *p15FunctionGraph) conditional(condition int, input any, field string, operand any) string {
	index := len(g.expected)
	group := ""
	if index < len(g.actual) {
		group, _ = p15C01Params(g.actual[index])["GroupingIdentifier"].(string)
	}
	if group == "" || g.groups[group] {
		g.err = fmt.Errorf("absent or reused native control identity")
	}
	g.groups[group] = true
	p := map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0, "WFCondition": condition,
		"WFConditionalLegacyComparisonBehavior": true, "WFInput": map[string]any{"Type": "Variable", "Variable": input}}
	if field != "" {
		p[field] = operand
	}
	g.emit("conditional", p)
	return group
}

func (g *p15FunctionGraph) boundary(group string, mode int, result bool) string {
	p := map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": mode}
	if result {
		return g.value("conditional", p)
	}
	return g.emit("conditional", p)
}

func (g *p15FunctionGraph) get(key, source string) string {
	return g.value("getvalueforkey", map[string]any{"WFDictionaryKey": key, "WFGetDictionaryValueType": "Value", "WFInput": p15FunctionEdge(source)})
}

func (g *p15FunctionGraph) numberCast(source string) string {
	return g.value("detect.number", map[string]any{"WFInput": p15FunctionEdge(source)})
}

func (g *p15FunctionGraph) math(op, left string, right any) string {
	return g.value("math", map[string]any{"WFInput": p15FunctionEdge(left), "WFMathOperation": op, "WFMathOperand": right})
}

func (g *p15FunctionGraph) output(source string) {
	g.emit("output", map[string]any{"WFOutput": p15FunctionText("\ufffc", source)})
}

func (g *p15FunctionGraph) header(name string) string {
	input := p15FunctionAttachment(map[string]any{"Type": "ExtensionInput"})
	zero := g.value("gettext", map[string]any{"WFTextActionText": "0"})
	g.emit("setvariable", map[string]any{"WFVariableName": "_cherri_is_fn", "WFInput": p15FunctionEdge(zero)})
	g.emit("setvariable", map[string]any{"WFVariableName": "_cherri_caller_input", "WFInput": input})
	provided := g.conditional(100, input, "", nil)
	possible := g.value("detect.dictionary", map[string]any{"WFInput": input})
	marker := g.get("cherri_functions", possible)
	magic := g.get("_cherri_magic", possible)
	valid := g.conditional(4, p15FunctionEdge(magic), "WFConditionalActionString", "__cherri_fn_envelope_v2__")
	marked := g.conditional(4, p15FunctionEdge(marker), "WFConditionalActionString", "1")
	one := g.value("gettext", map[string]any{"WFTextActionText": "1"})
	g.emit("setvariable", map[string]any{"WFVariableName": "_cherri_is_fn", "WFInput": p15FunctionEdge(one)})
	caller := g.get("_cherri_caller_input", possible)
	g.emit("setvariable", map[string]any{"WFVariableName": "_cherri_caller_input", "WFInput": p15FunctionEdge(caller)})
	g.boundary(marked, 2, false)
	g.boundary(valid, 2, false)
	g.boundary(provided, 2, false)
	g.outer = g.conditional(4, p15FunctionVariable("_cherri_is_fn"), "WFConditionalActionString", "1")
	envelope := g.value("detect.dictionary", map[string]any{"WFInput": input})
	function := g.get("function", envelope)
	g.functionName = function
	g.get("arguments", envelope)
	g.body = g.conditional(4, p15FunctionEdge(function), "WFConditionalActionString", name)
	return envelope
}

func (g *p15FunctionGraph) nextFunction(name string) {
	g.boundary(g.body, 2, false)
	g.body = g.conditional(4, p15FunctionEdge(g.functionName), "WFConditionalActionString", name)
}

func (g *p15FunctionGraph) caller() {
	g.boundary(g.body, 2, false)
	g.emit("exit", map[string]any{})
	g.boundary(g.outer, 1, false)
}

type p15FunctionArgument struct {
	present bool
	kind    int
	value   any
}

func p15FunctionNumber(value string) p15FunctionArgument {
	return p15FunctionArgument{true, 3, p15FunctionText(value)}
}

func p15FunctionString(value string) p15FunctionArgument {
	return p15FunctionArgument{true, 0, p15FunctionText(value)}
}

func (g *p15FunctionGraph) payload(name string, args ...p15FunctionArgument) string {
	fields := map[string]p15FunctionArgument{
		"_cherri_caller_input": {true, 0, p15FunctionVariable("_cherri_caller_input")},
		"_cherri_magic":        p15FunctionString("__cherri_fn_envelope_v2__"),
		"cherri_functions":     p15FunctionString("1"), "function": p15FunctionString(name),
	}
	values := []any{}
	for i, arg := range args {
		presence := "0"
		if arg.present {
			fields[fmt.Sprintf("arg_%d", i)] = arg
			presence = "1"
		} else {
			arg = p15FunctionString("")
		}
		fields[fmt.Sprintf("has_arg_%d", i)] = p15FunctionString(presence)
		values = append(values, map[string]any{"WFItemType": arg.kind, "WFValue": arg.value})
	}
	array := map[string]any{"WFSerializationType": "WFArrayParameterState"}
	if len(args) != 0 {
		array["Value"] = values
	}
	// The legacy positional mirror for a function with zero parameters has no
	// rows. This is not the representation of a supplied empty List argument.
	fields["arguments"] = p15FunctionArgument{true, 2, array}
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

func (g *p15FunctionGraph) call(payload string) string {
	name := g.workflowName
	if name == "" {
		name = "p15-short-circuit"
	}
	return g.value("runworkflow", map[string]any{"WFInput": p15FunctionEdge(payload), "WFWorkflowName": name,
		"WFWorkflow": map[string]any{"isSelf": true, "workflowName": name}})
}

func (g *p15FunctionGraph) finish() error {
	g.boundary(g.outer, 2, false)
	// These caller fixtures fall through after the dispatcher; keep the
	// complete native graph, including the empty main result.
	g.emit("nothing", nil)
	return g.compare()
}

// Explicit main returns already own their result; only close the dispatcher.
// This deliberately differs from finish(), whose authored callers fall through.
func (g *p15FunctionGraph) finishExplicitMainReturn() error {
	g.boundary(g.outer, 2, false)
	return g.compare()
}

// compare also checks complete workflows without the function dispatcher.
func (g *p15FunctionGraph) compare() error {
	if g.err != nil {
		return g.err
	}
	if len(g.actual) != len(g.expected) {
		return fmt.Errorf("unmodeled native action or missing operation: %d versus %d", len(g.actual), len(g.expected))
	}
	positions := map[string]int{}
	for index, action := range g.actual {
		if uuid, present := p15C01Params(action)["UUID"]; present {
			name, ok := uuid.(string)
			if !ok || name == "" {
				return fmt.Errorf("invalid native identity at %d", index)
			}
			if _, duplicate := positions[name]; duplicate {
				return fmt.Errorf("duplicate native producer identity")
			}
			positions[name] = index
		}
		if !reflect.DeepEqual(p15FunctionComparable(action, false), p15FunctionComparable(g.expected[index], false)) {
			return fmt.Errorf("native operation/edge/control mismatch at %d: got %#v want %#v", index, p15FunctionComparable(action, false), p15FunctionComparable(g.expected[index], false))
		}
	}
	var edges func(any, int) error
	edges = func(value any, consumer int) error {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" {
				id, _ := v["OutputUUID"].(string)
				producer, found := positions[id]
				if !found || producer >= consumer {
					return fmt.Errorf("missing or forward native producer at %d", consumer)
				}
			}
			for _, child := range v {
				if err := edges(child, consumer); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := edges(child, consumer); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for index, action := range g.actual {
		if err := edges(action, index); err != nil {
			return err
		}
	}
	return nil
}

func p15FN01NativeError(actions []map[string]any, first, factor string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("multiply")
	left := g.numberCast(g.get("arg_0", envelope))
	right := g.numberCast(g.get("arg_1", envelope))
	g.output(g.math("×", left, p15FunctionEdge(right)))
	g.caller()
	result := g.numberCast(g.call(g.payload("multiply", p15FunctionNumber(first), p15FunctionNumber(factor))))
	continued := g.math("+", result, 1)
	g.emit("setclipboard", map[string]any{"WFInput": p15FunctionText("\ufffc:\ufffc", result, continued)})
	return g.finish()
}

func p15FN02NativeError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("defaults")
	selected := []string{}
	for slot, fallback := range []any{1, 9, "def"} {
		presence := g.get(fmt.Sprintf("has_arg_%d", slot), envelope)
		guard := g.conditional(4, p15FunctionEdge(presence), "WFConditionalActionString", "1")
		g.get(fmt.Sprintf("arg_%d", slot), envelope)
		g.boundary(guard, 1, false)
		if slot == 2 {
			g.value("gettext", map[string]any{"WFTextActionText": fallback})
		} else {
			g.value("number", map[string]any{"WFNumberActionNumber": fallback})
		}
		value := g.boundary(guard, 2, true)
		if slot != 2 {
			value = g.numberCast(value)
		}
		selected = append(selected, value)
	}
	joined := g.value("gettext", map[string]any{"WFTextActionText": p15FunctionText("\ufffc:\ufffc:\ufffc", selected...)})
	g.output(joined)
	g.caller()
	explicit := g.call(g.payload("defaults", p15FunctionNumber("0"), p15FunctionNumber("0"), p15FunctionString("")))
	omitted := g.call(g.payload("defaults", p15FunctionArgument{}, p15FunctionArgument{}, p15FunctionArgument{}))
	mixed := g.call(g.payload("defaults", p15FunctionNumber("0"), p15FunctionArgument{}, p15FunctionArgument{}))
	g.emit("setclipboard", map[string]any{"WFInput": p15FunctionText("\ufffc|\ufffc|\ufffc", explicit, omitted, mixed)})
	return g.finish()
}

func p15FN03NativeError(actions []map[string]any, argument string) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	envelope := g.header("fact")
	n := g.numberCast(g.get("arg_0", envelope))
	base := g.conditional(1, p15FunctionEdge(n), "WFNumberValue", 1)
	one := g.value("number", map[string]any{"WFNumberActionNumber": 1})
	g.output(one)
	g.boundary(base, 2, false)
	decreased := g.math("-", n, 1)
	previous := g.numberCast(g.call(g.payload("fact", p15FunctionArgument{true, 0, p15FunctionEdge(decreased)})))
	g.output(g.math("×", n, p15FunctionEdge(previous)))
	g.caller()
	result := g.numberCast(g.call(g.payload("fact", p15FunctionNumber(argument))))
	g.emit("setclipboard", map[string]any{"WFInput": p15FunctionText("\ufffc", result)})
	return g.finish()
}
