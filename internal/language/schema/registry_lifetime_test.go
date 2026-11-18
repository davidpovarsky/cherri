package schema

import (
	"github.com/electrikmilk/cherri/internal/language/types"
	"testing"
)

func TestRegistryReplacementIndexes(t *testing.T) {
	first := &ActionSchema{CallableName: "local", AppleIdentifier: "com.example.old"}
	sibling := &ActionSchema{CallableName: "sibling", AppleIdentifier: "com.example.old"}
	registry := NewRegistry([]*ActionSchema{first, sibling}, nil)
	original := registry.Fingerprint()
	for _, id := range []string{"com.example.new", "com.example.new", "com.example.final"} {
		registry.RegisterCustomAction(&ActionSchema{CallableName: "local", AppleIdentifier: id})
		old := registry.LookupByIdentifier("com.example.old")
		if len(old) != 1 || old[0].CallableName != "sibling" {
			t.Errorf("replaced callable remained in old identifier index: %#v", old)
		}
		current := registry.LookupByIdentifier(id)
		if len(current) != 1 || current[0].CallableName != "local" {
			t.Errorf("replacement accumulated duplicate native identities: %#v", current)
		}
	}
	if len(registry.LookupByIdentifier("com.example.new")) != 0 {
		t.Error("intermediate native identity survived a second replacement")
	}
	if registry.Fingerprint() == original || registry.Fingerprint() != NewRegistry(registry.AllActions(), registry.AllEnums()).Fingerprint() {
		t.Error("registry fingerprint does not describe its effective current actions")
	}
}

func TestRegistryCloneDetachesMetadata(t *testing.T) {
	required := true
	record := types.NewRecord([]types.RecordField{{Name: "name", Type: types.Text}})
	action := &ActionSchema{
		CallableName: "local", AppleIdentifier: "com.example.local",
		Parameters: []ParameterSchema{{Label: "value", Type: record, AcceptedForms: []string{"reference"}, EnumValues: []string{"first"}}},
		OutputType: types.NewList(record), EmittedKeys: []string{"Value"},
		Docs:             ActionDocs{Keywords: []string{"original"}},
		AppIntent:        &AppIntentDescriptor{Name: "original", RequiresAppInstallation: &required},
		StaticParameters: map[string]any{"nested": map[string]any{"values": []any{"original", []byte{1, 2}}}},
	}
	base := NewRegistry([]*ActionSchema{action}, map[string][]string{"domain": {"first"}})
	clone := base.Clone()
	if clone.Fingerprint() != base.Fingerprint() {
		t.Fatal("a snapshot changed the catalog fingerprint")
	}
	changed, _ := clone.LookupAction("local")
	if changed == action {
		t.Fatal("clone shares action pointers")
	}
	changed.Parameters[0].Label = "changed"
	changed.Parameters[0].AcceptedForms[0] = "literal"
	changed.Parameters[0].EnumValues[0] = "changed"
	changed.Parameters[0].Type.(types.RecordType).Fields["name"] = types.RecordField{Name: "name", Type: types.Number}
	changed.OutputType.(types.ListType).Element.(types.RecordType).Fields["name"] = types.RecordField{Name: "name", Type: types.Bool}
	changed.EmittedKeys[0] = "Changed"
	changed.Docs.Keywords[0] = "changed"
	changed.AppIntent.Name = "changed"
	*changed.AppIntent.RequiresAppInstallation = false
	nested := changed.StaticParameters["nested"].(map[string]any)["values"].([]any)
	nested[0] = "changed"
	nested[1].([]byte)[0] = 9
	values, _ := clone.LookupEnum("domain")
	values[0] = "changed"
	old, _ := base.LookupAction("local")
	oldValues, _ := base.LookupEnum("domain")
	originalNested := old.StaticParameters["nested"].(map[string]any)["values"].([]any)
	if old.Parameters[0].Label != "value" || old.Parameters[0].AcceptedForms[0] != "reference" || old.Parameters[0].EnumValues[0] != "first" ||
		!old.Parameters[0].Type.(types.RecordType).Fields["name"].Type.Equals(types.Text) ||
		!old.OutputType.(types.ListType).Element.(types.RecordType).Fields["name"].Type.Equals(types.Text) ||
		old.EmittedKeys[0] != "Value" || old.Docs.Keywords[0] != "original" ||
		old.AppIntent.Name != "original" || !*old.AppIntent.RequiresAppInstallation ||
		originalNested[0] != "original" || originalNested[1].([]byte)[0] != 1 || oldValues[0] != "first" {
		t.Fatal("mutating a snapshot reached the supplied catalog")
	}
	clone.RegisterCustomAction(&ActionSchema{CallableName: "local", AppleIdentifier: "com.example.replacement"})
	if old, _ := base.LookupAction("local"); old.AppleIdentifier != "com.example.local" || len(base.LookupByIdentifier("com.example.replacement")) != 0 {
		t.Fatal("registration in a snapshot changed the supplied indexes")
	}
}
