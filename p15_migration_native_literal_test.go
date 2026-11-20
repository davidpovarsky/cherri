package main

import (
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func TestP15RP33CanonicalNativePayload(t *testing.T) {
	cases := []struct {
		name, source string
		attachment   bool
	}{
		{"plain_parameter", `native.action("is.workflow.actions.gettext", parameters: {"WFTextActionText": "{name}"})`, true},
		{"explicit_raw_string", `native.action("is.workflow.actions.gettext", parameters: {"WFTextActionText": r"{name}"})`, true},
		{"explicit_native_plist", `native.action("is.workflow.actions.gettext", parameters: native.plist("{\"WFTextActionText\":\"{name}\"}"))`, false},
	}
	for _, fixture := range cases {
		t.Run(fixture.name, func(t *testing.T) {
			before, err := CompileSourceToPlist("rp33-before.cherri", fixture.source)
			if err != nil {
				t.Fatal("invalid original canonical native fixture", err)
			}
			converted, diagnostics, err := migrate.MigrateSourceWithDiagnostics(fixture.source)
			if err != nil || len(diagnostics) != 0 {
				t.Fatal("valid native source migration rejected", err, diagnostics)
			}
			after, err := CompileSourceToPlist("rp33-after.cherri", converted)
			if err != nil {
				t.Fatalf("migration changed an explicit native literal into executable syntax: %v\n%s", err, converted)
			}
			var beforeDocument, afterDocument map[string]any
			if _, err := plist.Unmarshal(before, &beforeDocument); err != nil {
				t.Fatal(err)
			}
			if _, err := plist.Unmarshal(after, &afterDocument); err != nil {
				t.Fatal(err)
			}
			actions := beforeDocument["WFWorkflowActions"].([]any)
			var expected any = "{name}"
			if fixture.attachment {
				// Ordinary native parameter strings already use the native escape's
				// symbolic-reference grammar; only native.plist is exact raw data.
				expected = map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "name"}}
			}
			if len(actions) != 1 || !reflect.DeepEqual(actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"], expected) {
				t.Fatalf("original native payload witness changed: %#v", actions)
			}
			if comparison := shortcutcompare.CompareDocumentsNormalized(beforeDocument, afterDocument); !comparison.Equal {
				t.Fatal("canonical raw payload changed during migration", comparison.Differences)
			}
			t.Run("payload_mutation", func(t *testing.T) {
				var changed map[string]any
				if _, err := plist.Unmarshal(before, &changed); err != nil {
					t.Fatal(err)
				}
				params := changed["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
				if fixture.attachment {
					params["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["VariableName"] = "other"
				} else {
					params["WFTextActionText"] = "{other}"
				}
				if shortcutcompare.CompareDocumentsNormalized(beforeDocument, changed).Equal {
					t.Fatal("changed native payload passed differential oracle")
				}
			})
		})
	}
}
