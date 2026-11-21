package main

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"howett.net/plist"
)

var p15LegacyMathUUID = regexp.MustCompile("^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$")

// This checks the complete three-action producer/binding/consumer graph. Numeric
// Text is an intentional Legacy wire type; scientific notation must be complete
// and parse to the independently specified operand, not a truncated prefix.
// It establishes emitted structure, not actual Apple execution.
func p15ValidateLegacyMathOperandGraph(document map[string]any, left, right float64, operation string) error {
	actions, ok := document["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 3 {
		return fmt.Errorf("want exactly math, binding and show actions")
	}
	action, ok := actions[0].(map[string]any)
	if !ok {
		return fmt.Errorf("math action is not a dictionary")
	}
	params, ok := action["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		return fmt.Errorf("math parameters are not a dictionary")
	}
	read := func(key string, expected float64) (string, error) {
		text, ok := params[key].(string)
		if !ok {
			return "", fmt.Errorf("%s lost its numeric Text wire type", key)
		}
		value, err := strconv.ParseFloat(text, 64)
		if err != nil || math.IsInf(value, 0) || math.IsNaN(value) || value != expected {
			return "", fmt.Errorf("%s is malformed or changed value: %q, want %.17g", key, text, expected)
		}
		return text, nil
	}
	leftText, err := read("WFInput", left)
	if err != nil {
		return err
	}
	rightText, err := read("WFMathOperand", right)
	if err != nil {
		return err
	}
	id, ok := params["UUID"].(string)
	if !ok || !p15LegacyMathUUID.MatchString(id) || strings.EqualFold(id, "00000000-0000-0000-0000-000000000000") {
		return fmt.Errorf("math producer UUID is invalid")
	}
	expected := []any{
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.math",
			"WFWorkflowActionParameters": map[string]any{
				"CustomOutputName": "Expression", "UUID": id,
				"WFInput": leftText, "WFMathOperand": rightText, "WFMathOperation": operation,
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFInput": map[string]any{
					"Value":               map[string]any{"OutputName": "Expression", "OutputUUID": id, "Type": "ActionOutput"},
					"WFSerializationType": "WFTextTokenAttachment",
				},
				"WFSerializationType": "WFTextTokenAttachment",
				"WFVariableName":      "answer",
			},
		},
		map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
			"WFWorkflowActionParameters": map[string]any{
				"Text": map[string]any{
					"Value": map[string]any{
						"attachmentsByRange": map[string]any{"{0, 1}": map[string]any{"Type": "Variable", "VariableName": "answer"}},
						"string":             "\ufffc",
					},
					"WFSerializationType": "WFTextTokenString",
				},
			},
		},
	}
	if !reflect.DeepEqual(actions, expected) {
		return fmt.Errorf("complete Legacy math graph differs from its independent producer/binding/consumer oracle")
	}
	return nil
}

func p15LegacyMathRoundtrip(t *testing.T, document map[string]any, format int) map[string]any {
	t.Helper()
	raw, err := plist.Marshal(document, format)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if _, err := plist.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	return decoded
}

func TestP15LegacyMathOperandBoundaries(t *testing.T) {
	for _, test := range []struct {
		name, expression, operation string
		left, right                 float64
	}{
		{"calibration_sum", "916736855 + 229323196", "+", 916736855, 229323196},
		{"calibration_edit", "738462586 + 65292533", "+", 738462586, 65292533},
		{"positive_exponent_rhs", "7 + 1000000", "+", 7, 1000000},
		{"negative_exponent_rhs", "7 - 0.0000000125", "-", 7, 1.25e-8},
		{"negative_left", "-7 - 2", "-", -7, 2},
		{"negative_right", "7 - -2", "-", 7, -2},
		{"both_negative", "-7 - -2", "-", -7, -2},
		{"small_addition_control", "7 + 2", "+", 7, 2},
		{"multiplication_control", "7 * -2", "×", 7, -2},
		{"division_control", "7 / -2", "÷", 7, -2},
		{"large_multiplication_control", "7 * 229323196", "×", 7, 229323196},
		{"fractional_addition_control", "7 + 0.125", "+", 7, 0.125},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := "@answer = " + test.expression + "\nshow(@answer)\n"
			document, err := compileLegacySourceToRawMapInMemory(source)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15ValidateLegacyMathOperandGraph(document, test.left, test.right, test.operation); err != nil {
				t.Fatal(err)
			}
			for _, format := range []int{plist.XMLFormat, plist.BinaryFormat} {
				decoded := p15LegacyMathRoundtrip(t, document, format)
				if err := p15ValidateLegacyMathOperandGraph(decoded, test.left, test.right, test.operation); err != nil {
					t.Fatalf("format %d changed numeric types, values or causal links: %v", format, err)
				}
			}
		})
	}
}

func TestP15LegacyMathOperandOracleRejectsCorruption(t *testing.T) {
	document, err := compileLegacySourceToRawMapInMemory("@answer = 7 + 2\nshow(@answer)\n")
	if err != nil {
		t.Fatal(err)
	}
	if err := p15ValidateLegacyMathOperandGraph(document, 7, 2, "+"); err != nil {
		t.Fatalf("healthy control was not accepted: %v", err)
	}
	parameters := func(document map[string]any, index int) map[string]any {
		return document["WFWorkflowActions"].([]any)[index].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	}
	for _, test := range []struct {
		name   string
		mutate func(map[string]any)
	}{
		{"malformed_positive_exponent", func(d map[string]any) { parameters(d, 0)["WFMathOperand"] = "2e+" }},
		{"malformed_negative_exponent", func(d map[string]any) { parameters(d, 0)["WFMathOperand"] = "2e-" }},
		{"wrong_left_value", func(d map[string]any) { parameters(d, 0)["WFInput"] = "-7" }},
		{"wrong_right_value", func(d map[string]any) { parameters(d, 0)["WFMathOperand"] = "3" }},
		{"wrong_operator", func(d map[string]any) { parameters(d, 0)["WFMathOperation"] = "-" }},
		{"integer_instead_of_numeric_text", func(d map[string]any) { parameters(d, 0)["WFMathOperand"] = uint64(2) }},
		{"zero_uuid", func(d map[string]any) { parameters(d, 0)["UUID"] = "00000000-0000-0000-0000-000000000000" }},
		{"disconnected_binding", func(d map[string]any) {
			parameters(d, 1)["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "11111111-1111-4111-8111-111111111111"
		}},
		{"renamed_result_variable", func(d map[string]any) { parameters(d, 1)["WFVariableName"] = "other" }},
		{"hardcoded_show_output", func(d map[string]any) { parameters(d, 2)["Text"] = "9" }},
		{"extra_effect", func(d map[string]any) {
			d["WFWorkflowActions"] = append(d["WFWorkflowActions"].([]any), map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.getclipboard"})
		}},
		{"extra_parameter", func(d map[string]any) { parameters(d, 0)["Unexpected"] = true }},
	} {
		t.Run(test.name, func(t *testing.T) {
			corrupt := p15LegacyMathRoundtrip(t, document, plist.BinaryFormat)
			test.mutate(corrupt)
			if err := p15ValidateLegacyMathOperandGraph(corrupt, 7, 2, "+"); err == nil {
				t.Fatal("single causal corruption was accepted")
			}
		})
	}
}
