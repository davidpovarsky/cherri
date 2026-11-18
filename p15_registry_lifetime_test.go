package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func p15RegistryDeclaration(name, parameter, identifier string) string {
	return fmt.Sprintf("action %s(%s: Text) -> Text {\nidentifier: %q\nprimary: %q\nbindings: {%s: {key: \"Value\", codec: \"text\"}}\n}\n", name, parameter, identifier, parameter, parameter)
}

func TestP15RegistryRepeatedCompileIsolation(t *testing.T) {
	for _, mode := range []string{"successful", "failed", "diagnostic_only"} {
		t.Run(mode, func(t *testing.T) {
			// Reset only the test's starting state. There is no reset between inputs.
			schema.ResetDefaultRegistry()
			t.Cleanup(schema.ResetDefaultRegistry)
			base := schema.DefaultRegistry()
			fingerprint := base.Fingerprint()
			before, _ := base.LookupAction("url")
			count := len(base.AllActions())
			source := p15RegistryDeclaration("url", "value", "com.example.registry.local") + "url(value: \"LOCAL\")\n"
			if mode == "failed" {
				source += "unknownRegistryAction()\n"
			}
			if mode == "diagnostic_only" {
				collectV2CompileErrorLines("registry-isolation.cherri", source+"unknownRegistryAction()\n")
			} else {
				actions, err := p15C03Actions(source, false)
				if mode == "failed" {
					if err == nil {
						t.Error("invalid source unexpectedly compiled")
					}
				} else if err != nil || len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != "com.example.registry.local" || actions[0].WFWorkflowActionParameters["Value"] != "LOCAL" {
					t.Errorf("custom source did not retain its own action: actions=%#v err=%v", actions, err)
				}
			}
			after, _ := base.LookupAction("url")
			if after.AppleIdentifier != before.AppleIdentifier || after.Parameters[0].Label != before.Parameters[0].Label {
				t.Errorf("custom source changed the shared canonical action: before=%s/%s after=%s/%s", before.AppleIdentifier, before.Parameters[0].Label, after.AppleIdentifier, after.Parameters[0].Label)
			}
			if len(base.AllActions()) != count || base.Fingerprint() != fingerprint || len(base.LookupByIdentifier("com.example.registry.local")) != 0 {
				t.Error("custom action leaked through canonical count, fingerprint, or native identifier index")
			}
			actions, err := p15C03Actions("url(url: [\"https://example.invalid/independent\"])\n", false)
			if err != nil {
				t.Fatalf("later independent compilation inherited another source's schema: %v", err)
			}
			if len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.url" {
				t.Fatalf("later compilation lost canonical native identity: %#v", actions)
			}
		})
	}
}

func TestP15RegistryCustomOutputReferenceIsolation(t *testing.T) {
	schema.ResetDefaultRegistry()
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, id := range []string{"com.example.registry.first", "com.example.registry.second"} {
		src := p15RegistryDeclaration("url", "value", id) + "let local: Text = url(value: \"LOCAL\")\ntext(f\"{local}\")\n"
		actions, err := p15C03Actions(src, false)
		if err != nil {
			t.Fatal(err)
		}
		if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != id || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
			t.Fatalf("custom source graph changed: %#v", actions)
		}
		uuid, err := p15C03TokenUUID(actions[1].WFWorkflowActionParameters["WFTextActionText"])
		if err != nil || uuid != actions[0].WFWorkflowActionParameters["UUID"] {
			t.Fatalf("custom output did not retain its producer reference: uuid=%q err=%v", uuid, err)
		}
	}
	_, err := p15C03Actions("url(value: \"not-a-canonical-label\")\n", false)
	if err == nil || !strings.Contains(err.Error(), "E_UNKNOWN_ARGUMENT") {
		t.Fatalf("independent canonical call accepted a departed custom parameter: %v", err)
	}
}
