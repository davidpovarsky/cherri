package main

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func TestP15SchemaCanonicalLabels(t *testing.T) {
	catalog := p15SchemaJSON(t, p15SchemaRun(t, ".", "go", "run", ".", "--actions-json"))
	actions, ok := catalog["actions"].([]any)
	if !ok || len(actions) != 505 {
		t.Fatalf("unexpected live action catalog: %T", catalog["actions"])
	}
	for _, raw := range actions {
		action := raw.(map[string]any)
		seen := map[string]bool{}
		params, _ := action["parameters"].([]any)
		for _, p := range params {
			label := p.(map[string]any)["name"].(string)
			if label == "" || seen[label] {
				t.Errorf("canonical action %s has empty/duplicate label %q", action["name"], label)
			}
			seen[label] = true
		}
	}
}

func TestP15SchemaCorrectedLabelNativeBindings(t *testing.T) {
	for _, tc := range []struct {
		name, source, identifier string
		labels                   map[string]string
		text                     map[string]string
	}{
		{
			"streetAddress",
			"streetAddress(addressLine1: \"Line one\", addressLine2: \"Line two\", city: \"City\", state: \"State\", country: \"Country\", zipCode: 12345)",
			"is.workflow.actions.address",
			map[string]string{"addressLine1": "WFAddressLine1", "addressLine2": "WFAddressLine2"},
			map[string]string{"WFAddressLine1": "Line one", "WFAddressLine2": "Line two", "WFCity": "City", "WFState": "State", "WFCountry": "Country"},
		},
		{
			"getDeviceUsage",
			"getDeviceUsage(usageType: \"all\", during: \"today\", startTime: \"09:00\", endTime: \"17:00\")",
			"com.apple.intelligenceplatform.IntelligencePlatform.IntelligencePlatformDataActionsAppIntentsExtension.CalculateAppUsageIntent",
			map[string]string{"startTime": "startTime", "endTime": "endTime"},
			map[string]string{"startTime": "09:00", "endTime": "17:00"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			action, found := schema.DefaultRegistry().LookupAction(tc.name)
			if !found {
				t.Fatal("canonical action missing")
			}
			for label, wire := range tc.labels {
				param, found := action.ParameterByLabel(label)
				if !found || param.WireKey != wire {
					t.Fatalf("label %s did not resolve to %s", label, wire)
				}
			}
			actions := compileDebugFixture(t, tc.source, false)
			if len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != tc.identifier {
				t.Fatalf("wrong native action identity/cardinality: %+v", actions)
			}
			for wire, want := range tc.text {
				value := actions[0].WFWorkflowActionParameters[wire]
				if err := p15CheckLiteralText(value, want); err != nil {
					t.Fatalf("%s: %v", wire, err)
				}
				if p15CheckLiteralText(value, want+"corrupt") == nil {
					t.Fatal("native label oracle accepted a wrong value")
				}
			}
			if tc.name == "streetAddress" && fmt.Sprint(actions[0].WFWorkflowActionParameters["WFPostalCode"]) != "12345" {
				t.Fatal("postal code changed")
			}
		})
	}
}
