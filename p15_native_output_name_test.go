package main

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

func TestP15NativeOutputNameOmission(t *testing.T) {
	for _, test := range []struct {
		name, extra string
		present     bool
		value       string
	}{
		{"absent", "", false, ""},
		{"explicit_empty", `, outputName: ""`, false, ""},
		{"named", `, outputName: "Synthetic result"`, true, "Synthetic result"},
	} {
		t.Run(test.name, func(t *testing.T) {
			source := fmt.Sprintf(`text(f"{native.ref("ActionOutput", uuid: "33333333-3333-3333-3333-333333330405"%s)}")`, test.extra)
			raw, err := CompileSourceToPlist("synthetic-native-output-name.cherri", source)
			if err != nil {
				t.Fatal(err)
			}
			doc, err := DecodeNativeShortcutDocument(raw)
			if err != nil {
				t.Fatal(err)
			}
			if len(doc.Actions) != 1 {
				t.Fatalf("expected one native Text action, got %d", len(doc.Actions))
			}
			serialized, ok := doc.Actions[0].Parameters["WFTextActionText"].(map[string]any)
			if !ok {
				t.Fatal("missing token string")
			}
			value := serialized["Value"].(map[string]any)
			attachment := value["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)
			got, present := attachment["OutputName"]
			if present != test.present || present && got != test.value {
				t.Fatalf("OutputName presence/value changed: got %#v, present=%t; want %q, present=%t", got, present, test.value, test.present)
			}
			if attachment["OutputUUID"] != "33333333-3333-3333-3333-333333330405" || attachment["Type"] != "ActionOutput" {
				t.Fatalf("native identity changed: %#v", attachment)
			}
		})
	}
}

func TestP15BackendGeneratedReferenceNameDefaultPreserved(t *testing.T) {
	for _, kind := range []backend.ReferenceKind{backend.RefActionResult, backend.RefLoopResult} {
		desc := ReferenceToDescriptor(&backend.Reference{Kind: kind, ProducerID: "synthetic-generated-id"})
		if desc.OutputName != "synthetic-generated-id" {
			t.Fatalf("ordinary generated reference lost its default display name: %+v", desc)
		}
	}
}
