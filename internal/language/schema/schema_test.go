package schema

import (
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	actions := reg.AllActions()
	if len(actions) != 505 {
		t.Fatalf("expected 505 actions, got %d", len(actions))
	}

	fp := reg.Fingerprint()
	if len(fp) != 64 {
		t.Fatalf("expected 64-char hex fingerprint, got %q", fp)
	}
	t.Logf("CURRENT_FINGERPRINT: %s", fp)

	// Verify resizeImage
	resize, ok := reg.LookupAction("resizeImage")
	if !ok {
		t.Fatalf("action resizeImage not found")
	}
	if resize.PrimaryParameterID != "image" {
		t.Errorf("expected primaryParameterId 'image', got %q", resize.PrimaryParameterID)
	}
	if resize.OutputTypeName != "Image" {
		t.Errorf("expected output type 'Image', got %q", resize.OutputTypeName)
	}

	// Verify enums
	langs, ok := reg.LookupEnum("translationLanguage")
	if !ok || len(langs) == 0 {
		t.Errorf("expected translationLanguage enum to have values")
	}
}

func TestRegisterCustomActionPopulatesEnumDomainWithoutOverridingBuiltins(t *testing.T) {
	registry := NewRegistry(nil, map[string][]string{"builtInMode": {"Original"}})
	dynamic := []string{"Fast", "Accurate"}
	registry.RegisterCustomAction(&ActionSchema{
		CallableName:    "syntheticAction",
		AppleIdentifier: "is.workflow.actions.synthetic",
		Parameters: []ParameterSchema{
			{ID: "mode", EnumName: "dynamicMode", EnumValues: dynamic},
			{ID: "builtin", EnumName: "builtInMode", EnumValues: []string{"Poisoned"}},
		},
	})
	values, present := registry.LookupEnum("dynamicMode")
	if !present || len(values) != 2 || values[0] != "Fast" || values[1] != "Accurate" {
		t.Fatalf("custom enum not registered: %v, present=%v", values, present)
	}
	// The input metadata and copied public view must not alias registry state.
	dynamic[1] = "Mutated"
	values[0] = "Mutated"
	fresh := registry.AllEnums()
	if fresh["dynamicMode"][0] != "Fast" || fresh["dynamicMode"][1] != "Accurate" {
		t.Fatalf("custom enum registration aliases mutable values: %v", fresh["dynamicMode"])
	}
	if got := fresh["builtInMode"]; len(got) != 1 || got[0] != "Original" {
		t.Fatalf("custom action overwrote preexisting native enum: %v", got)
	}
	if _, exists := registry.LookupAction("syntheticAction"); !exists {
		t.Fatal("custom action registration lost callable")
	}
}
