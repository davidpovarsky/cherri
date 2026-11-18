package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15MetadataActions(t *testing.T, expression string) []any {
	t.Helper()
	source := `native.action("com.example.cherri.metadata", parameters: {"Value": ` + expression + `})`
	data, err := CompileSourceToPlist("metadata.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	return root["WFWorkflowActions"].([]any)
}

func p15MetadataValue(t *testing.T, actions []any) any {
	t.Helper()
	if len(actions) != 1 {
		t.Fatalf("raw metadata materialized %d runtime actions instead of exactly one explicit native action", len(actions))
	}
	action := actions[0].(map[string]any)
	if action["WFWorkflowActionIdentifier"] != "com.example.cherri.metadata" {
		t.Fatal("raw metadata changed action identity")
	}
	return action["WFWorkflowActionParameters"].(map[string]any)["Value"]
}

func TestP15NativeReferenceMetadataStaysInline(t *testing.T) {
	const raw = `[{"Type":"WFCoercionVariableAggrandizement","CoercionItemClass":"WFDictionaryContentItem"},{"Type":"WFDictionaryValueVariableAggrandizement","DictionaryKey":"payload"},{"Type":"WFPropertyVariableAggrandizement","PropertyName":"File Size","PropertyUserInfo":[{"WFItemType":"ordinary","Value":false}]}]`
	expected := []any{
		map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem"},
		map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "payload"},
		map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "File Size", "PropertyUserInfo": []any{map[string]any{"WFItemType": "ordinary", "Value": false}}},
	}
	for _, input := range []string{raw, `native.plist(` + raw + `)`, `{"Aggrandizements":` + raw + `}`} {
		t.Run(fmt.Sprint(len(input)), func(t *testing.T) {
			value := p15MetadataValue(t, p15MetadataActions(t, `native.ref("ActionOutput", uuid:"B2C3D4E5-F6A7-8901-BCDE-F12345678901", aggrandizements:`+input+`)`)).(map[string]any)
			if value["WFSerializationType"] != "WFTextTokenAttachment" {
				t.Fatal("raw reference lost attachment type")
			}
			ref := value["Value"].(map[string]any)
			if ref["OutputUUID"] != "B2C3D4E5-F6A7-8901-BCDE-F12345678901" || !reflect.DeepEqual(ref["Aggrandizements"], expected) {
				t.Fatalf("raw chain changed: %+v", ref)
			}
		})
	}
}

func TestP15NativePlistArrayStaysInline(t *testing.T) {
	value := p15MetadataValue(t, p15MetadataActions(t, `native.plist([{"WFItemType":"ordinary","Value":false},[{"WFSerializationType":"ordinary","Value":"literal"}]])`))
	expected := []any{map[string]any{"WFItemType": "ordinary", "Value": false}, []any{map[string]any{"WFSerializationType": "ordinary", "Value": "literal"}}}
	if !reflect.DeepEqual(value, expected) {
		t.Fatalf("raw plist array changed: %+v", value)
	}
}

func TestP15NativeReferenceMalformedMetadataRejects(t *testing.T) {
	for _, input := range []string{`1`, `["wrong"]`, `{"wrong":true}`} {
		source := `native.action("com.example.cherri.metadata", parameters: {"Value": native.ref("ActionOutput", uuid:"known", aggrandizements:` + input + `)})`
		_, err := CompileSourceToPlist("bad-metadata.cherri", source)
		if err == nil || !strings.Contains(err.Error(), "native.ref aggrandizements") {
			t.Fatalf("malformed metadata silently dropped: %s: %v", input, err)
		}
	}
}
