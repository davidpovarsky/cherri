package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/service"
)

func TestP15RawNativeJSONNullCannotDisappear(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"action_field", "native.action(\"com.example.p15.null\", {\"Present\":false,\"Probe\":null})"},
		{"nested_map", "native.action(\"com.example.p15.null\", {\"Nested\":{\"Probe\":null}})"},
		{"nested_array", "native.action(\"com.example.p15.null\", {\"Rows\":[false,0,\"\",null]})"},
		{"plist_literal", "native.action(\"com.example.p15.null\", native.plist({\"Present\":false,\"Probe\":null}))"},
		{"after_valid_action", "text(\"BEFORE_NULL\")\nnative.action(\"com.example.p15.null\",{\"Probe\":null})"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15T05NullDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source); err != nil {
				t.Error(err)
			}
			raw, err := CompileSourceToPlist("p15-raw-null.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), p15T05NullCode) {
				if len(raw) > 0 {
					t.Log("P15_RAW_NULL_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
				}
				t.Errorf("raw JSON null crossed native serialization instead of an exact limitation: native_bytes=%d error=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-raw-null-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diagnostics, _ := svc.Analyze(uri)
			if err := p15T05NullDiagnostic(diagnostics, tc.source); err != nil {
				t.Error(err)
			}
		})
	}
}

func TestP15NativePlistEncodedNullCannotDisappear(t *testing.T) {
	for _, tc := range []struct{ payload, path string }{
		{"{\"Present\":false,\"Probe\":null}", "$[\"Probe\"]"},
		{"{\"Nested\":{\"Probe\":null}}", "$[\"Nested\"][\"Probe\"]"},
		{"{\"Rows\":[false,0,\"\",null]}", "$[\"Rows\"][3]"},
	} {
		t.Run(tc.payload, func(t *testing.T) {
			literal, err := json.Marshal(tc.payload)
			if err != nil {
				t.Fatal(err)
			}
			source := "native.action(\"com.example.p15.null\",native.plist(" + string(literal) + "))"
			raw, err := CompileSourceToPlist("p15-encoded-null.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), p15T05NullCode) || !strings.Contains(err.Error(), tc.path) {
				if len(raw) > 0 {
					t.Log("P15_RAW_NULL_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
				}
				t.Fatalf("encoded JSON null was dropped/coerced at native boundary: native_bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

func TestP15JSONNullTextRemainsData(t *testing.T) {
	const payload = "{\"missing\":null,\"false\":false,\"zero\":0,\"empty\":\"\",\"list\":[],\"map\":{}}"
	literal, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{
		"native.action(\"com.example.p15.json\",{\"JSON\":" + string(literal) + ",\"Present\":false})",
		"native.action(\"com.example.p15.json\",native.plist({\"JSON\":" + string(literal) + ",\"Present\":false}))",
	} {
		actions := p15CompiledActions(t, source)
		if len(actions) != 1 || actions[0]["WFWorkflowActionIdentifier"] != "com.example.p15.json" {
			t.Fatal("literal JSON specimen lost its native action identity")
		}
		if !reflect.DeepEqual(p15Param(actions[0]), map[string]any{"JSON": payload, "Present": false}) {
			t.Fatalf("literal JSON text or explicit false was reinterpreted: %#v", p15Param(actions[0]))
		}
	}
	// This is real source/native-byte proof. It does not execute JSON decoding
	// in Shortcuts or invent a plist-null representation for the JSON text.
}
