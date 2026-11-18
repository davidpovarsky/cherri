package main

import (
	"fmt"
	"reflect"
	"testing"
)

// Decode actual compiler plist output and follow each native List row to its
// Number producer. A number written directly into a text row is not accepted.
func p15NumericListValues(actions []ShortcutAction) ([]string, error) {
	producers := map[string]int{}
	listIndex := -1
	for i, action := range actions {
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok {
			producers[id] = i
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.list" {
			listIndex = i
		}
	}
	if listIndex < 0 {
		return nil, fmt.Errorf("missing native List")
	}
	rows, ok := actions[listIndex].WFWorkflowActionParameters["WFItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing native List rows")
	}
	values := make([]string, len(rows))
	seen := map[string]bool{}
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok || p15Int(row["WFItemType"]) != 0 {
			return nil, fmt.Errorf("item %d is not a content row", i)
		}
		attachment, ok := row["WFValue"].(map[string]any)
		if !ok || attachment["WFSerializationType"] != "WFTextTokenAttachment" {
			return nil, fmt.Errorf("item %d is a raw scalar in a native text row", i)
		}
		token, ok := attachment["Value"].(map[string]any)
		if !ok || token["Type"] != "ActionOutput" || token["Aggrandizements"] != nil {
			return nil, fmt.Errorf("item %d changed native identity", i)
		}
		id, _ := token["OutputUUID"].(string)
		j, exists := producers[id]
		if !exists || j >= listIndex || seen[id] {
			return nil, fmt.Errorf("item %d has invalid producer", i)
		}
		seen[id] = true
		if actions[j].WFWorkflowActionIdentifier != "is.workflow.actions.number" {
			return nil, fmt.Errorf("item %d was coerced from native Number", i)
		}
		values[i] = fmt.Sprint(actions[j].WFWorkflowActionParameters["WFNumberActionNumber"])
	}
	return values, nil
}

func TestP15NumericListRetainsNativeNumbers(t *testing.T) {
	want := []string{"0", "1", "-2.5", "99", "9007199254740993"}
	source := "[0, 1, -2.5, 99, 9007199254740993]"
	for _, tc := range []struct{ name, source string }{
		{"explicit_reference_control", "let zero=0\nlet one=1\nlet decimal=-2.5\nlet last=99\nlet large=9007199254740993\nlet items=[zero,one,decimal,last,large]\ngetVariable(items)"},
		{"immutable", "let items=" + source + "\ngetVariable(items)"},
		{"mutable", "var items=" + source + "\ngetVariable(items)"},
		{"assignment", "var items=[]\nitems=" + source + "\ngetVariable(items)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values, err := p15NumericListValues(compileDebugFixture(t, tc.source, false))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(values, want) {
				t.Fatalf("native Number values/order changed: got %v want %v", values, want)
			}
		})
	}
}

func TestP15NumericListOracleRejectsNativeRowCorruption(t *testing.T) {
	source := "let zero=0\nlet one=1\nlet items=[zero,one]\ngetVariable(items)"
	good := compileDebugFixture(t, source, false)
	if _, err := p15NumericListValues(good); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"raw_number", "missing_uuid", "text_coercion", "duplicate_reference", "wrong_producer"} {
		t.Run(fault, func(t *testing.T) {
			actions := compileDebugFixture(t, source, false)
			list := p15ActionsNamed(actions, "list")[0].WFWorkflowActionParameters["WFItems"].([]any)
			row := list[0].(map[string]any)
			token := row["WFValue"].(map[string]any)["Value"].(map[string]any)
			switch fault {
			case "raw_number":
				row["WFValue"] = 0
			case "missing_uuid":
				token["OutputUUID"] = "MISSING"
			case "text_coercion":
				token["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
			case "duplicate_reference":
				list[1] = row
			case "wrong_producer":
				for i := range actions {
					if actions[i].WFWorkflowActionParameters["UUID"] == token["OutputUUID"] {
						actions[i].WFWorkflowActionIdentifier = "is.workflow.actions.gettext"
					}
				}
			}
			if _, err := p15NumericListValues(actions); err == nil {
				t.Fatal("accepted corrupt native Number row")
			}
		})
	}
}
