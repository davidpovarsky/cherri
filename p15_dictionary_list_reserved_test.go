package main

import (
	"fmt"
	"testing"
)

func p15LiteralDictionaryFields(t *testing.T, value any) map[string]map[string]any {
	t.Helper()
	outer, ok := value.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		t.Fatalf("expected native Dictionary field value, got %#v", value)
	}
	inner, ok := outer["Value"].(map[string]any)
	if !ok {
		t.Fatalf("missing dictionary value: %#v", outer)
	}
	items, ok := inner["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		t.Fatalf("missing dictionary fields: %#v", inner)
	}
	fields := map[string]map[string]any{}
	for _, raw := range items {
		item := raw.(map[string]any)
		key := item["WFKey"].(map[string]any)["Value"].(map[string]any)["string"].(string)
		if fields[key] != nil {
			t.Fatalf("duplicate dictionary key %q", key)
		}
		fields[key] = item
	}
	return fields
}

func p15OnlyLiteralListDictionary(t *testing.T, source string) map[string]map[string]any {
	t.Helper()
	actions := p15CompiledActions(t, source)
	bodies := p15DictionaryBodies(actions)
	if len(bodies) != 1 {
		t.Fatalf("authored map must have one Dictionary producer, got %d", len(bodies))
	}
	if err := p15CheckDictionaryList(actions, bodies); err != nil {
		t.Fatal(err)
	}
	return p15LiteralDictionaryFields(t, bodies[0])
}

func TestP15DictionaryListReservedNamesAreOrdinaryFields(t *testing.T) {
	for _, serialization := range []string{"WFTextTokenString", "WFTextTokenAttachment", "WFDictionaryFieldValue", "ordinary"} {
		for _, binding := range []string{"let", "var"} {
			t.Run(binding+"_"+serialization, func(t *testing.T) {
				source := fmt.Sprintf(`%s items = [{"WFItemType": 0, "WFSerializationType": %q, "Value": "payload", "WFValue": "kept"}]
return getFirstItem(items)`, binding, serialization)
				fields := p15OnlyLiteralListDictionary(t, source)
				if len(fields) != 4 {
					t.Fatalf("reserved names lost: %#v", fields)
				}
				if p15Int(fields["WFItemType"]["WFItemType"]) != 3 {
					t.Fatal("WFItemType ordinary Number field was lost")
				}
				for key, want := range map[string]string{"WFSerializationType": serialization, "Value": "payload", "WFValue": "kept"} {
					field := fields[key]
					if p15Int(field["WFItemType"]) != 0 || field["WFValue"].(map[string]any)["Value"].(map[string]any)["string"] != want {
						t.Fatalf("ordinary field %s changed: %#v", key, field)
					}
				}
			})
		}
	}
}

func TestP15DictionaryListNestedReservedNamesPreserveTypedValues(t *testing.T) {
	fields := p15OnlyLiteralListDictionary(t, `let items = [{"child": {"WFSerializationType": "WFTextTokenAttachment", "WFItemType": 4, "Value": false, "WFValue": ["a", "b"]}}]
return getFirstItem(items)`)
	child := fields["child"]
	if p15Int(child["WFItemType"]) != 1 {
		t.Fatalf("child coerced away from Dictionary: %#v", child)
	}
	nested := p15LiteralDictionaryFields(t, child["WFValue"])
	if len(nested) != 4 || p15Int(nested["Value"]["WFItemType"]) != 4 {
		t.Fatalf("nested ordinary fields changed: %#v", nested)
	}
	boolValue := nested["Value"]["WFValue"].(map[string]any)
	if value, exists := boolValue["Value"]; !exists || value != false {
		t.Fatalf("explicit false missing: %#v", boolValue)
	}
	array := nested["WFValue"]
	if p15Int(array["WFItemType"]) != 2 {
		t.Fatal("WFValue ordinary List field was lost")
	}
	arrayValue := array["WFValue"].(map[string]any)
	if arrayValue["WFSerializationType"] != "WFArrayParameterState" {
		t.Fatal("nested List field type changed")
	}
	items := arrayValue["Value"].([]any)
	if len(items) != 2 {
		t.Fatalf("nested item count %d", len(items))
	}
	for i, want := range []string{"a", "b"} {
		if items[i].(map[string]any)["WFValue"].(map[string]any)["Value"].(map[string]any)["string"] != want {
			t.Fatal("nested order changed")
		}
	}
}

func TestP15DictionaryListPreservesActualFStringAndAttachment(t *testing.T) {
	actions := p15CompiledActions(t, "let label = text(\"a\")\nlet items = [f\"prefix:{label}\", label]\nreturn getFirstItem(items)")
	if len(p15DictionaryBodies(actions)) != 0 {
		t.Fatal("actual Text/token values were materialized as Dictionary")
	}
	var items []any
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
			items = p15Param(action)["WFItems"].([]any)
		}
	}
	if len(items) != 2 {
		t.Fatalf("list count %d", len(items))
	}
	for i, want := range []string{"WFTextTokenString", "WFTextTokenAttachment"} {
		if items[i].(map[string]any)["WFValue"].(map[string]any)["WFSerializationType"] != want {
			t.Fatalf("native token %d changed", i)
		}
	}
}

func TestP15DictionaryListPreservesExactImportedNativeFallback(t *testing.T) {
	actions := []any{map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.list",
		"WFWorkflowActionParameters": map[string]any{"UUID": "66666666-0000-4000-8000-000000000001", "WFItems": []any{
			map[string]any{"WFItemType": int64(0), "WFValue": map[string]any{"WFItemType": int64(4), "Value": false, "WFValue": "native opaque"}},
		}},
	}}
	assertP12ParDualVerified(t, "P15_RESERVED_LIST", actions,
		map[int]string{0: "EXACT_NATIVE_FALLBACK"},
		map[int]string{0: "UNREPRESENTABLE_PARAM"}, nil)
}
