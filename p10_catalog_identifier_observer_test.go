package main

import (
	"slices"
	"testing"

	"howett.net/plist"
)

func TestP10_CatalogRawActionIdentifierRequiresFixtureTarget(t *testing.T) {
	const target = "is.workflow.actions.custom.p10test"
	action := func(identifier, key string) map[string]any {
		return map[string]any{
			"WFWorkflowActionIdentifier": identifier,
			"WFWorkflowActionParameters": map[string]any{key: "value"},
		}
	}
	fixture := action(target, "TestKey")
	normalization := action("is.workflow.actions.nothing", "NormalizationOnly")
	wrong := action("is.workflow.actions.custom.wrong", "WrongOnly")

	for _, alias := range []struct {
		name       string
		identifier string
	}{
		{"raw", "is.workflow.actions.raw"},
		{"rawaction", "is.workflow.actions.rawaction"},
	} {
		for _, tc := range []struct {
			name    string
			actions []map[string]any
			want    string
		}{
			{"target_alone", []map[string]any{fixture}, target},
			{"target_then_normalization", []map[string]any{fixture, normalization}, target},
			{"wrong_target", []map[string]any{wrong}, ""},
			{"wrong_target_then_normalization", []map[string]any{wrong, normalization}, ""},
			{"normalization_only", []map[string]any{normalization}, ""},
			{"missing_target", []map[string]any{}, ""},
			{"catalog_placeholder_only", []map[string]any{action(alias.identifier, "PlaceholderOnly")}, ""},
		} {
			t.Run(alias.name+"/"+tc.name, func(t *testing.T) {
				data, err := plist.Marshal(map[string]any{"WFWorkflowActions": tc.actions}, plist.XMLFormat)
				if err != nil {
					t.Fatal(err)
				}
				got, keys := inspectEmittedPlistForIdentifier(data, alias.identifier)
				if got != tc.want {
					t.Fatalf("matched identifier = %q, want %q", got, tc.want)
				}
				if tc.want == "" {
					if keys != nil {
						t.Fatalf("rejected target retained parameter keys: %v", keys)
					}
				} else if !slices.Equal(keys, []string{"TestKey"}) {
					t.Fatalf("matched keys = %v, want custom target keys [TestKey]", keys)
				}
			})
		}
	}
}
