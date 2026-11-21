package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

func p15ImportedAppendParams(actions []any, index int) map[string]any {
	action, _ := actions[index].(map[string]any)
	params, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return params
}
func p15ImportedAppendGraphError(actions []any, coerced bool) error {
	if len(actions) != 4 {
		return fmt.Errorf("native append requires exactly four actions")
	}
	ids := []string{}
	valid := regexp.MustCompile("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")
	for _, index := range []int{0, 1} {
		id, _ := p15ImportedAppendParams(actions, index)["UUID"].(string)
		if !valid.MatchString(id) || strings.EqualFold(id, "00000000-0000-0000-0000-000000000000") {
			return fmt.Errorf("invalid producer UUID at %d", index)
		}
		ids = append(ids, id)
	}
	if strings.EqualFold(ids[0], ids[1]) {
		return fmt.Errorf("duplicate producer UUID")
	}
	descriptor := func(id string) map[string]any {
		return map[string]any{"Type": "ActionOutput", "OutputName": "parsed", "OutputUUID": id}
	}
	input := descriptor(ids[1])
	aggrandizements := []any{}
	if coerced {
		aggrandizements = append(aggrandizements, map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem"})
	}
	aggrandizements = append(aggrandizements, map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "key"})
	input["Aggrandizements"] = aggrandizements
	attach := func(value any) any {
		return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": value}
	}
	action := func(id string, p map[string]any) any {
		return map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + id, "WFWorkflowActionParameters": p}
	}
	expected := []any{
		action("gettext", map[string]any{"CustomOutputName": "parsed", "UUID": ids[0], "WFTextActionText": "{\"key\":\"VALUE\"}"}),
		action("detect.dictionary", map[string]any{"CustomOutputName": "parsed", "UUID": ids[1], "WFInput": attach(descriptor(ids[0]))}),
		action("appendvariable", map[string]any{"WFVariableName": "Target", "WFInput": attach(input)}),
		action("showresult", map[string]any{"Text": map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "\ufffc", "attachmentsByRange": map[string]any{"{0, 1}": map[string]any{"Type": "Variable", "VariableName": "Target"}}}}}),
	}
	if !reflect.DeepEqual(actions, expected) {
		return fmt.Errorf("complete native append graph differs from its independent wire contract")
	}
	return nil
}
func p15ImportedAppendClone(t *testing.T, actions []any) []any {
	t.Helper()
	b, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var cloned []any
	if err := json.Unmarshal(b, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}
func p15ImportedAppendInput(actions []any) map[string]any {
	return p15ImportedAppendParams(actions, 2)["WFInput"].(map[string]any)["Value"].(map[string]any)
}
func p15ImportedAppendGraphCorruptions(t *testing.T, actions []any, coerced bool) {
	t.Helper()
	for name, mutate := range map[string]func([]any){
		"wrong_variable":          func(a []any) { p15ImportedAppendParams(a, 2)["WFVariableName"] = "Other" },
		"append_changed_to_set":   func(a []any) { a[2].(map[string]any)["WFWorkflowActionIdentifier"] = "is.workflow.actions.setvariable" },
		"missing_input":           func(a []any) { delete(p15ImportedAppendParams(a, 2), "WFInput") },
		"wrong_producer":          func(a []any) { p15ImportedAppendInput(a)["OutputUUID"] = p15ImportedAppendParams(a, 0)["UUID"] },
		"dangling_producer":       func(a []any) { p15ImportedAppendInput(a)["OutputUUID"] = "11111111-1111-4111-8111-111111111111" },
		"duplicate_producer_uuid": func(a []any) { p15ImportedAppendParams(a, 1)["UUID"] = p15ImportedAppendParams(a, 0)["UUID"] },
		"zero_producer_uuid":      func(a []any) { p15ImportedAppendParams(a, 0)["UUID"] = "00000000-0000-0000-0000-000000000000" },
		"malformed_producer_uuid": func(a []any) { p15ImportedAppendParams(a, 1)["UUID"] = "not-a-uuid" },
		"missing_producer_uuid":   func(a []any) { delete(p15ImportedAppendParams(a, 1), "UUID") },
		"wrong_dictionary_key": func(a []any) {
			agg := p15ImportedAppendInput(a)["Aggrandizements"].([]any)
			agg[len(agg)-1].(map[string]any)["DictionaryKey"] = "other"
		},
		"wrong_input_kind":       func(a []any) { p15ImportedAppendInput(a)["Type"] = "Variable" },
		"missing_aggrandizement": func(a []any) { delete(p15ImportedAppendInput(a), "Aggrandizements") },
		"extra_parameter":        func(a []any) { p15ImportedAppendParams(a, 2)["Unproved"] = true },
		"wrong_consumer": func(a []any) {
			p15ImportedAppendParams(a, 3)["Text"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["VariableName"] = "Other"
		},
		"wrong_span": func(a []any) {
			v := p15ImportedAppendParams(a, 3)["Text"].(map[string]any)["Value"].(map[string]any)
			v["attachmentsByRange"] = map[string]any{"{1, 1}": map[string]any{"Type": "Variable", "VariableName": "Target"}}
		},
		"changed_producer_label": func(a []any) { p15ImportedAppendParams(a, 1)["CustomOutputName"] = "Other" },
		"changed_literal":        func(a []any) { p15ImportedAppendParams(a, 0)["WFTextActionText"] = "{}" },
	} {
		t.Run("reject_"+name, func(t *testing.T) {
			clone := p15ImportedAppendClone(t, actions)
			mutate(clone)
			if p15ImportedAppendGraphError(clone, coerced) == nil {
				t.Fatal("corrupted native graph accepted")
			}
		})
	}
	t.Run("reject_extra_action", func(t *testing.T) {
		clone := p15ImportedAppendClone(t, actions)
		clone = append(clone, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
		if p15ImportedAppendGraphError(clone, coerced) == nil {
			t.Fatal("extra native action accepted")
		}
	})
	if coerced {
		t.Run("reject_reordered_coercion", func(t *testing.T) {
			clone := p15ImportedAppendClone(t, actions)
			agg := p15ImportedAppendInput(clone)["Aggrandizements"].([]any)
			agg[0], agg[1] = agg[1], agg[0]
			if p15ImportedAppendGraphError(clone, true) == nil {
				t.Fatal("aggrandizement order changed")
			}
		})
	}
	t.Run("consistent_uuid_alpha_rename", func(t *testing.T) {
		clone := p15ImportedAppendClone(t, actions)
		first := "11111111-1111-4111-8111-111111111111"
		second := "22222222-2222-4222-8222-222222222222"
		p15ImportedAppendParams(clone, 0)["UUID"] = first
		p15ImportedAppendParams(clone, 1)["UUID"] = second
		p15ImportedAppendParams(clone, 1)["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = first
		p15ImportedAppendInput(clone)["OutputUUID"] = second
		if err := p15ImportedAppendGraphError(clone, coerced); err != nil {
			t.Fatal(err)
		}
	})
}
