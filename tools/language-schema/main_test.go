package main

import (
	"strings"
	"testing"
)

func TestValidateFacets_DetectsInvalidAction(t *testing.T) {
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{Name: "text", Parameters: []LegacyParameter{{Name: "text"}}},
		},
	}
	oldFacets := facets
	facets = map[string]ActionFacet{
		"nonexistentAction": {PrimaryParameterID: "foo"},
	}
	defer func() { facets = oldFacets }()

	err := validateFacets(cat)
	if err == nil || !strings.Contains(err.Error(), "unknown action") {
		t.Fatalf("expected error on unknown action in facet, got %v", err)
	}
}

func TestValidateFacets_DetectsInvalidParameter(t *testing.T) {
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{Name: "alert", Parameters: []LegacyParameter{{Name: "title"}}}, // missing "alert" param
		},
	}
	oldFacets := facets
	facets = map[string]ActionFacet{
		"alert": {PrimaryParameterID: "alert"},
	}
	defer func() { facets = oldFacets }()

	err := validateFacets(cat)
	if err == nil || !strings.Contains(err.Error(), "unknown primary parameter") {
		t.Fatalf("expected error on missing primary parameter in facet, got %v", err)
	}
}

func TestUpstreamPropagation_AddsParameter(t *testing.T) {
	cat := &LegacyCatalog{
		Actions: []LegacyAction{
			{
				Name:               "customAction",
				ShortcutIdentifier: "is.workflow.actions.custom",
				Parameters: []LegacyParameter{
					{Name: "param1", Key: "Param1", Type: "Text"},
					{Name: "newOptionalParam", Key: "NewOptionalParam", Type: "Number", Optional: true},
				},
			},
		},
	}
	code := generateGoCode(cat)
	if !strings.Contains(code, "newOptionalParam") {
		t.Errorf("expected generated schema to contain propagated newOptionalParam")
	}
	if !strings.Contains(code, "types.Number") {
		t.Errorf("expected generated schema to map Number type for propagated parameter")
	}
}
