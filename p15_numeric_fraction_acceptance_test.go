package main

import (
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// This deliberately small native-graph oracle permits only the arithmetic,
// variable assignment, and terminal consumer required by these public cases.
// It does not simulate Apple execution or accept extra rounding/coercion steps.
func p15N01GraphResult(actions []map[string]any) (float64, error) {
	var outputs = map[string]float64{}
	var variables = map[string]float64{}
	var read func(any) (float64, error)
	read = func(value any) (float64, error) {
		switch number := value.(type) {
		case int:
			return float64(number), nil
		case int64:
			return float64(number), nil
		case uint64:
			return float64(number), nil
		case float64:
			if !math.IsNaN(number) && !math.IsInf(number, 0) {
				return number, nil
			}
		}
		var token, ok = value.(map[string]any)
		if !ok || len(token) != 2 || token["WFSerializationType"] != "WFTextTokenAttachment" {
			return 0, fmt.Errorf("numeric operand is neither a native number nor an exact attachment")
		}
		var descriptor, valid = token["Value"].(map[string]any)
		if !valid {
			return 0, fmt.Errorf("missing numeric producer descriptor")
		}
		if descriptor["Type"] == "ActionOutput" {
			for key := range descriptor {
				if key != "Type" && key != "OutputUUID" && key != "OutputName" {
					return 0, fmt.Errorf("unmodeled numeric transformation %s", key)
				}
			}
			var id, _ = descriptor["OutputUUID"].(string)
			if number, found := outputs[id]; found {
				return number, nil
			}
		} else if descriptor["Type"] == "Variable" && len(descriptor) == 2 {
			var name, _ = descriptor["VariableName"].(string)
			if number, found := variables[name]; found {
				return number, nil
			}
		}
		return 0, fmt.Errorf("numeric consumer precedes or loses its actual producer")
	}
	var allowed = map[string][]string{
		"number":       {"UUID", "CustomOutputName", "WFNumberActionNumber"},
		"math":         {"UUID", "CustomOutputName", "WFInput", "WFMathOperation", "WFMathOperand"},
		"setvariable":  {"WFInput", "WFVariableName"},
		"setclipboard": {"WFInput"},
	}
	var result float64
	var consumers int
	for index, action := range actions {
		var identifier, valid = action["WFWorkflowActionIdentifier"].(string)
		if !valid || !strings.HasPrefix(identifier, "is.workflow.actions.") {
			return 0, fmt.Errorf("missing native arithmetic identifier")
		}
		var kind = strings.TrimPrefix(identifier, "is.workflow.actions.")
		var keys, exists = allowed[kind]
		if !exists {
			return 0, fmt.Errorf("unmodeled arithmetic action %q", kind)
		}
		var parameters = p15C01Params(action)
		for key := range parameters {
			var known bool
			for _, permitted := range keys {
				known = known || key == permitted
			}
			if !known {
				return 0, fmt.Errorf("unmodeled arithmetic parameter %s", key)
			}
		}
		var number float64
		var err error
		switch kind {
		case "number":
			number, err = read(parameters["WFNumberActionNumber"])
		case "math":
			var left, leftErr = read(parameters["WFInput"])
			var right, rightErr = read(parameters["WFMathOperand"])
			if leftErr != nil || rightErr != nil {
				return 0, fmt.Errorf("lost arithmetic operands: %v / %v", leftErr, rightErr)
			}
			switch parameters["WFMathOperation"] {
			case "+":
				number = left + right
			case "-":
				number = left - right
			case "×":
				number = left * right
			case "÷":
				if right == 0 {
					return 0, fmt.Errorf("zero native divisor")
				}
				number = left / right
			default:
				return 0, fmt.Errorf("wrong native arithmetic operation")
			}
		case "setvariable":
			number, err = read(parameters["WFInput"])
			var name, ok = parameters["WFVariableName"].(string)
			if !ok || name == "" || err != nil {
				return 0, fmt.Errorf("lost numeric variable assignment: %v", err)
			}
			variables[name] = number
			continue
		case "setclipboard":
			result, err = read(parameters["WFInput"])
			if err != nil || index != len(actions)-1 {
				return 0, fmt.Errorf("lost terminal numeric consumer: %v", err)
			}
			consumers++
			continue
		}
		var id, ok = parameters["UUID"].(string)
		if err != nil || !ok || id == "" || math.IsNaN(number) || math.IsInf(number, 0) {
			return 0, fmt.Errorf("invalid native numeric producer: %v", err)
		}
		if _, duplicate := outputs[id]; duplicate {
			return 0, fmt.Errorf("duplicate numeric producer identity")
		}
		outputs[id] = number
	}
	if consumers != 1 {
		return 0, fmt.Errorf("missing exact terminal numeric observation")
	}
	return result, nil
}

func TestP15N01FractionalNativeDataflow(t *testing.T) {
	for _, sample := range []struct {
		name, source string
		want         float64
	}{
		{"five_over_two", "let result = 5 / 2", 2.5},
		{"negative", "let result = -5 / 2", -2.5},
		{"fractional_numerator", "let result = 5.5 / 2", 2.75},
		{"fractional_denominator", "let result = 5 / 0.5", 10},
		{"small_fraction", "let result = 1 / 8", 0.125},
		{"nested_expression", "let result = (5 / 2) * 3", 7.5},
		{"named_result", "let half = 5 / 2\nlet result = half / 2", 1.25},
		{"dynamic_operand", "var source = 5\nlet result = source / 2", 2.5},
		{"compound_fraction", "var result = 5\nresult /= 2", 2.5},
		{"compound_chain", "var result = 10\nresult += 3\nresult *= 2\nresult -= 4\nresult /= 2", 11},
	} {
		t.Run(sample.name, func(t *testing.T) {
			var actions = p15CompiledActions(t, sample.source+"\nsetClipboard(result)\n")
			if actual, err := p15N01GraphResult(actions); err != nil || actual != sample.want {
				t.Fatalf("native fractional dataflow got %g want %g: %v", actual, sample.want, err)
			}
		})
	}
	var actions = p15CompiledActions(t, "let result = 5 / 2\nsetClipboard(result)\n")
	for _, fault := range []string{"integer_fold", "wrong_divisor", "wrong_operation", "text_operand", "wrong_output", "coercion", "rounding", "missing_observation"} {
		t.Run("corrupt/"+fault, func(t *testing.T) {
			var broken = p15C01Clone(t, actions)
			var p = p15C01Params(broken[0])
			switch fault {
			case "integer_fold":
				broken[0]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
				broken[0]["WFWorkflowActionParameters"] = map[string]any{"UUID": p["UUID"], "WFNumberActionNumber": 2}
			case "wrong_divisor":
				p["WFMathOperand"] = 5
			case "wrong_operation":
				p["WFMathOperation"] = "×"
			case "text_operand":
				p["WFInput"] = "5"
			case "wrong_output", "coercion":
				var input = p15C01Params(broken[len(broken)-1])["WFInput"].(map[string]any)["Value"].(map[string]any)
				if fault == "wrong_output" {
					input["OutputUUID"] = "not-the-fraction"
				} else {
					input["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
				}
			case "rounding":
				broken = append(broken[:1], append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.round", "WFWorkflowActionParameters": map[string]any{}}}, broken[1:]...)...)
			case "missing_observation":
				broken = broken[:len(broken)-1]
			}
			if actual, err := p15N01GraphResult(broken); err == nil && actual == 2.5 {
				t.Fatal("damaged native fraction graph retained a passing observation")
			}
		})
	}
}

func TestP15N01NumericConstraints(t *testing.T) {
	for _, sample := range []struct{ name, expression, operand, message string }{
		{"divide_zero", "5 / 0", "0", "constant zero"},
		{"divide_negative_zero", "5 / -0", "-0", "constant zero"},
		{"modulus_zero", "5 % 0", "0", "constant zero"},
		{"text_numerator", "\"five\" / 2", "\"five\"", "requires Number"},
		{"text_denominator", "5 / \"two\"", "\"two\"", "requires Number"},
		{"bool_denominator", "5 / false", "false", "requires Number"},
	} {
		t.Run(sample.name, func(t *testing.T) {
			var source = "let result = " + sample.expression
			var diagnostics = p15TypeDiagnostics(t, source)
			if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, sample.message) {
				t.Fatalf("missing precise numeric constraint: %+v", diagnostics)
			}
			if err := p15ExactLiteralDiagnostic(diagnostics, source, sample.operand, analysis.CodeArgumentType); err != nil {
				t.Fatal(err)
			}
			if raw, err := CompileSourceToPlist("p15-n01-constraint.cherri", source); err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
				t.Fatalf("invalid numeric input emitted an artifact: %d %v", len(raw), err)
			}
			var wrong = append([]analysis.Diagnostic(nil), diagnostics...)
			wrong[0].Span.Start.ByteOffset++
			if p15ExactLiteralDiagnostic(wrong, source, sample.operand, analysis.CodeArgumentType) == nil {
				t.Fatal("wrong constraint source span accepted")
			}
			wrong = append([]analysis.Diagnostic(nil), diagnostics...)
			wrong[0].Code = analysis.CodeUnknownName
			if p15ExactLiteralDiagnostic(wrong, source, sample.operand, analysis.CodeArgumentType) == nil {
				t.Fatal("unrelated diagnostic accepted")
			}
		})
	}
	var valid = "let result = 5 / 2\nsetClipboard(result)\n"
	if diagnostics := p15TypeDiagnostics(t, valid); len(diagnostics) != 0 {
		t.Fatalf("valid fractional input rejected after numeric errors: %+v", diagnostics)
	}
	if actual, err := p15N01GraphResult(p15CompiledActions(t, valid)); err != nil || actual != 2.5 {
		t.Fatalf("healthy compiler did not recover after numeric errors: %g %v", actual, err)
	}
}
