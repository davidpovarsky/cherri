package main

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

func TestP15RP33CanonicalNativePayload(t *testing.T) {
	cases := []struct{ name, source string }{
		{"plain_parameter", `native.action("is.workflow.actions.gettext", parameters: {"WFTextActionText": "{name}"})`},
		{"explicit_raw_string", `native.action("is.workflow.actions.gettext", parameters: {"WFTextActionText": r"{name}"})`},
		{"explicit_native_plist", `native.action("is.workflow.actions.gettext", parameters: native.plist("{\"WFTextActionText\":\"{name}\"}"))`},
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
			if len(actions) != 1 || actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] != "{name}" {
				t.Fatal("original literal witness changed")
			}
			if comparison := shortcutcompare.CompareDocumentsNormalized(beforeDocument, afterDocument); !comparison.Equal {
				t.Fatal("canonical raw payload changed during migration", comparison.Differences)
			}
		})
	}
}
