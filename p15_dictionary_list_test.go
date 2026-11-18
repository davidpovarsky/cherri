package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// The reference deliberately uses separately authored Dictionary producers,
// not the collection literal path being repaired.
const p15DictionaryListReference = `
let first = {"name": "first", "flag": false, "zero": 0, "empty": ""}
let second = {"name": "second", "nested": ["a", "b"], "child": {"flag": false}}
let items = [first, second]
return getFirstItem(items)
`

const p15DictionaryListLiteral = `[{"name": "first", "flag": false, "zero": 0, "empty": ""}, {"name": "second", "nested": ["a", "b"], "child": {"flag": false}}]`

func p15DictionaryBodies(actions []map[string]any) []map[string]any {
	var bodies []map[string]any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			bodies = append(bodies, p15Param(action)["WFItems"].(map[string]any))
		}
	}
	return bodies
}

// Reads the emitted plist without normalization. A List content row must point
// to the original native Dictionary producer; a raw dictionary in a text row,
// text coercion, dangling/case-changed UUID, duplicate item, or reordered item
// is not accepted as preservation of the dictionary value.
func p15CheckDictionaryList(actions []map[string]any, want []map[string]any) error {
	listIndex := -1
	producers := make(map[string]int)
	for i, action := range actions {
		if uuid, _ := p15Param(action)["UUID"].(string); uuid != "" {
			producers[uuid] = i
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
			listIndex = i
		}
	}
	if listIndex < 0 {
		return fmt.Errorf("no native List action")
	}
	items, ok := p15Param(actions[listIndex])["WFItems"].([]any)
	if !ok || len(items) != len(want) {
		return fmt.Errorf("list item count mismatch: %d", len(items))
	}
	seen := make(map[string]bool)
	for i, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok || p15Int(item["WFItemType"]) != 0 {
			return fmt.Errorf("item %d is not a content row", i)
		}
		value, ok := item["WFValue"].(map[string]any)
		if !ok || value["WFSerializationType"] != "WFTextTokenAttachment" {
			return fmt.Errorf("item %d is not an original native value reference", i)
		}
		token, ok := value["Value"].(map[string]any)
		if !ok || token["Type"] != "ActionOutput" || token["Aggrandizements"] != nil {
			return fmt.Errorf("item %d contains an altered value reference", i)
		}
		uuid, _ := token["OutputUUID"].(string)
		producer, exists := producers[uuid]
		if !exists || producer >= listIndex || seen[uuid] {
			return fmt.Errorf("item %d has an invalid or repeated producer", i)
		}
		seen[uuid] = true
		if actions[producer]["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			return fmt.Errorf("item %d does not retain a Dictionary producer", i)
		}
		if !reflect.DeepEqual(p15Param(actions[producer])["WFItems"], want[i]) {
			return fmt.Errorf("item %d dictionary value/order changed", i)
		}
	}
	return nil
}

func TestP15DictionaryListLiteralPreservesNativeValues(t *testing.T) {
	reference := p15CompiledActions(t, p15DictionaryListReference)
	want := p15DictionaryBodies(reference)
	if len(want) != 2 {
		t.Fatalf("explicit Dictionary reference expected 2 producers, got %d", len(want))
	}
	if err := p15CheckDictionaryList(reference, want); err != nil {
		t.Fatal("independent reference path:", err)
	}
	for _, tc := range []struct{ name, source string }{
		{"immutable", "let items = " + p15DictionaryListLiteral + "\nreturn getFirstItem(items)"},
		{"mutable", "var items = " + p15DictionaryListLiteral + "\nreturn getFirstItem(items)"},
		{"assignment", "var items = []\nitems = " + p15DictionaryListLiteral + "\nreturn getFirstItem(items)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, tc.source)
			if err := p15CheckDictionaryList(actions, want); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15DictionaryListOracleRejectsOriginalDefectAndTampering(t *testing.T) {
	reference := p15CompiledActions(t, p15DictionaryListReference)
	want := p15DictionaryBodies(reference)
	for _, mutation := range []string{"raw_dictionary_text_row", "missing_producer", "swapped_order", "text_coercion"} {
		t.Run(mutation, func(t *testing.T) {
			encoded, _ := json.Marshal(reference)
			var actions []map[string]any
			if err := json.Unmarshal(encoded, &actions); err != nil {
				t.Fatal(err)
			}
			list := p15Param(actions[2])["WFItems"].([]any)
			first := list[0].(map[string]any)
			switch mutation {
			case "raw_dictionary_text_row":
				first["WFValue"] = map[string]any{"name": "first", "flag": false}
			case "missing_producer":
				first["WFValue"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "MISSING"
			case "swapped_order":
				list[0], list[1] = list[1], list[0]
			case "text_coercion":
				first["WFValue"].(map[string]any)["Value"].(map[string]any)["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}
			}
			// JSON cloning changes only numeric Go representation. Use the same
			// cloned reference bodies so rejection must be the intended mutation.
			if err := p15CheckDictionaryList(actions, p15DictionaryBodies(actions)); err == nil {
				t.Fatalf("oracle accepted %s (reference bodies %d)", mutation, len(want))
			}
		})
	}
}

func TestP15DictionaryListUsesDocumentedNativeContentArrayContract(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]map[string]any
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	list := metadata["is.workflow.actions.list"]
	parameters := list["Parameters"].([]any)
	if len(parameters) != 1 || parameters[0].(map[string]any)["Key"] != "WFItems" || parameters[0].(map[string]any)["Class"] != "WFContentArrayParameter" {
		t.Fatal("native List contract changed; revalidate the content row representation")
	}
	if !strings.Contains(list["Description"].(map[string]any)["DescriptionNote"].(string), "variable") {
		t.Fatal("native List metadata no longer documents variable contents")
	}
}
