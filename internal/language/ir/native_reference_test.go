package ir

import (
	"reflect"
	"testing"
)

func TestNativeReferenceKindContract(t *testing.T) {
	for _, tc := range []struct{ source, uuid, name, want string }{
		{" ActionOutput ", "exact-uuid", "", "ActionOutput"},
		{"output", "exact-uuid", "", "ActionOutput"},
		{"var", "", "exact name", "Variable"},
		{"Variable", "", "exact name", "Variable"},
		{"ShortcutInput", "", "", "ExtensionInput"},
		{"extensioninput", "", "", "ExtensionInput"},
		{"input", "", "", "ExtensionInput"},
		{"clipboard", "", "", "Clipboard"},
		{"currentdate", "", "", "CurrentDate"},
		{"date", "", "", "CurrentDate"},
		{"device", "", "", "DeviceDetails"},
		{"devicedetails", "", "", "DeviceDetails"},
		{"ask", "", "", "Ask"},
		{"askeachtime", "", "", "Ask"},
	} {
		t.Run(tc.source, func(t *testing.T) {
			got, err := NormalizeNativeReferenceKind(tc.source, tc.uuid, tc.name)
			if err != nil || got != tc.want {
				t.Fatalf("kind=%q err=%v", got, err)
			}
		})
	}
	for _, tc := range []struct{ kind, uuid, name string }{
		{"", "", ""}, {"UnknownKind", "", ""}, {"7", "", ""},
		{"ActionOutput", "", ""}, {"output", "  ", ""},
		{"Variable", "", ""}, {"var", "", "  "},
	} {
		if _, err := NormalizeNativeReferenceKind(tc.kind, tc.uuid, tc.name); err == nil {
			t.Fatalf("invalid shape accepted: %#v", tc)
		}
	}
}

func TestNativeReferenceAggrandizementShapePreservesOpaqueFields(t *testing.T) {
	item := map[string]any{
		"Type":               "WFPropertyVariableAggrandizement",
		"PropertyName":       "Height",
		"PropertyUserInfo":   map[string]any{"opaque": []any{int64(7), false, []byte{1, 2}}},
		"UnknownNativeField": "preserve exactly",
	}
	for _, raw := range []any{[]any{item}, map[string]any{"Aggrandizements": []any{item}, "UnrelatedEnvelopeField": true}} {
		got, err := NativeReferenceAggrandizements(raw)
		if err != nil || len(got) != 1 || !reflect.DeepEqual(got[0], item) {
			t.Fatalf("opaque fields changed: %#v %v", got, err)
		}
	}
	if got, err := NativeReferenceAggrandizements([]any{}); err != nil || len(got) != 0 {
		t.Fatalf("empty aggrandizements changed: %#v %v", got, err)
	}
	for _, raw := range []any{nil, 7, "text", map[string]any{}, []any{7}, map[string]any{"Aggrandizements": []any{nil}}} {
		if _, err := NativeReferenceAggrandizements(raw); err == nil {
			t.Fatalf("non-dictionary shape accepted: %#v", raw)
		}
	}
}
