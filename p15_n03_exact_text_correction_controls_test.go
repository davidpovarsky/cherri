package main

import (
	"encoding/json"
	"testing"
)

// These controls exercise the oracle's representation boundary only. They do
// not equate a real number, string or Boolean with a native control integer.
func TestP15N03ExactTextOracleScalarControls(t *testing.T) {
	expected := map[string]any{
		"WFCondition": float64(4), "WFControlFlowMode": float64(0),
		"WFNumberValue": float64(1), "WFConditionalLegacyComparisonBehavior": true,
	}
	makeActual := func() map[string]any {
		return map[string]any{
			"WFCondition": uint64(4), "WFControlFlowMode": uint64(0),
			"WFNumberValue": float64(1), "WFConditionalLegacyComparisonBehavior": true,
		}
	}
	t.Run("native_unsigned", func(t *testing.T) {
		if !p15N03WireFieldsEqual(makeActual(), expected) {
			t.Fatal("exact native unsigned control fields rejected")
		}
	})
	t.Run("json_integral", func(t *testing.T) {
		if !p15N03WireFieldsEqual(expected, expected) {
			t.Fatal("exact retained JSON control fields rejected")
		}
	})
	for _, fault := range []string{
		"condition_string", "condition_boolean", "condition_fraction", "condition_negative",
		"condition_wrong_integer", "mode_fraction", "number_integer", "legacy_integer",
		"missing_control", "extra_field",
	} {
		t.Run(fault, func(t *testing.T) {
			actual := makeActual()
			switch fault {
			case "condition_string":
				actual["WFCondition"] = "4"
			case "condition_boolean":
				actual["WFCondition"] = true
			case "condition_fraction":
				actual["WFCondition"] = 4.5
			case "condition_negative":
				actual["WFCondition"] = -4.0
			case "condition_wrong_integer":
				actual["WFCondition"] = uint64(5)
			case "mode_fraction":
				actual["WFControlFlowMode"] = 0.5
			case "number_integer":
				actual["WFNumberValue"] = uint64(1)
			case "legacy_integer":
				actual["WFConditionalLegacyComparisonBehavior"] = uint64(1)
			case "missing_control":
				delete(actual, "WFControlFlowMode")
			case "extra_field":
				actual["unexpected"] = true
			}
			if p15N03WireFieldsEqual(actual, expected) {
				t.Fatal("oracle accepted wire type/value/inventory corruption", fault)
			}
		})
	}
}

func TestP15N03ExactTextLogicalFlagIdentityControls(t *testing.T) {
	const source = "let flag=number(1)==1\nlet left=text(\"AbC\")\nlet right=text(\"abc\")\nlet answer=flag && (left==right)\ngetVariable(answer)\n"
	good := p15N03ExactCompile(t, source)
	_, guard, err := p15N03ExactLogicalFlag(good)
	if err != nil {
		t.Fatal(err)
	}
	number := -1
	for index, action := range good {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" &&
			p15N03ExactParams(action)["CustomOutputName"] == "flag" {
			number = index
		}
	}
	if number < 0 || guard <= number+5 {
		t.Fatal("actual source fixture has no complete numeric flag and later consumer")
	}
	t.Run("json_integral", func(t *testing.T) {
		raw, err := json.Marshal(good)
		if err != nil {
			t.Fatal(err)
		}
		var actions []map[string]any
		if err := json.Unmarshal(raw, &actions); err != nil {
			t.Fatal(err)
		}
		if _, _, err := p15N03ExactLogicalFlag(actions); err != nil {
			t.Fatal("exact retained JSON authored integer rejected", err)
		}
	})
	for _, fault := range []string{
		"number_value", "comparison_code", "comparison_operand", "true_yield", "false_yield",
		"result_name", "duplicate_result_uuid", "consumer_uses_number", "duplicate_number_producer",
		"number_string", "number_boolean", "number_fraction", "number_negative",
		"number_unsigned_wrong", "number_missing",
	} {
		t.Run(fault, func(t *testing.T) {
			raw, err := json.Marshal(good)
			if err != nil {
				t.Fatal(err)
			}
			var actions []map[string]any
			if err := json.Unmarshal(raw, &actions); err != nil {
				t.Fatal(err)
			}
			inputID, _ := p15N03ExactParams(actions[number])["UUID"].(string)
			switch fault {
			case "number_value":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = float64(0)
			case "number_string":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = "1"
			case "number_boolean":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = true
			case "number_fraction":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = 1.5
			case "number_negative":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = -1.0
			case "number_unsigned_wrong":
				p15N03ExactParams(actions[number])["WFNumberActionNumber"] = uint64(0)
			case "number_missing":
				delete(p15N03ExactParams(actions[number]), "WFNumberActionNumber")
			case "comparison_code":
				p15N03ExactParams(actions[number+1])["WFCondition"] = float64(5)
			case "comparison_operand":
				left, _, err := p15N03ExactNamedReference(actions, "left")
				if err != nil {
					t.Fatal(err)
				}
				p15N03ExactParams(actions[number+1])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15N03Attachment(left)}
			case "true_yield":
				p15N03ExactParams(actions[number+2])["WFNumberActionNumber"] = float64(0)
			case "false_yield":
				p15N03ExactParams(actions[number+4])["WFNumberActionNumber"] = float64(1)
			case "result_name":
				p15N03ExactParams(actions[number+5])["CustomOutputName"] = "different"
			case "duplicate_result_uuid":
				p15N03ExactParams(actions[number+5])["UUID"] = inputID
			case "consumer_uses_number":
				input := map[string]any{"Type": "ActionOutput", "OutputUUID": inputID, "OutputName": "flag"}
				p15N03ExactParams(actions[guard])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15N03Attachment(input)}
			case "duplicate_number_producer":
				actions = append(actions, map[string]any{
					"WFWorkflowActionIdentifier": "is.workflow.actions.number",
					"WFWorkflowActionParameters": map[string]any{
						"WFNumberActionNumber": float64(1), "CustomOutputName": "flag", "UUID": "N03_DUPLICATE_FLAG_SOURCE",
					},
				})
			}
			if _, _, err := p15N03ExactLogicalFlag(actions); err == nil {
				t.Fatal("accepted source producer/Boolean-result/consumer corruption", fault)
			}
		})
	}
}
