package schema

import (
	"encoding/json"
	"testing"
)

func TestRP08SemanticConstantIdentityEncoding(t *testing.T) {
	var original, ok = DefaultRegistry().LookupAction("openURL")
	if !ok {
		t.Fatal("canonical action fixture missing")
	}
	var identity = func(constants map[string]any) string {
		var action = *original
		action.StaticParameters = constants
		return NewRegistry([]*ActionSchema{&action}, nil).Fingerprint()
	}
	var cases = []struct {
		name        string
		left, right map[string]any
	}{
		{"bool_and_text", map[string]any{"value": false}, map[string]any{"value": "false"}},
		{"number_and_text", map[string]any{"value": 0}, map[string]any{"value": "0"}},
		{"real_and_integer", map[string]any{"value": float64(0)}, map[string]any{"value": 0}},
		{"data_and_text", map[string]any{"value": []byte{1, 2}}, map[string]any{"value": "AQI="}},
		{"nil_and_text", map[string]any{"value": nil}, map[string]any{"value": "<nil>"}},
		{"empty_list_and_text", map[string]any{"value": []any{}}, map[string]any{"value": "[]"}},
		{"empty_map_and_text", map[string]any{"value": map[string]any{}}, map[string]any{"value": "map[]"}},
		{"nested_literal_type", map[string]any{"value": map[string]any{"flag": false}}, map[string]any{"value": map[string]any{"flag": "false"}}},
		{"list_cardinality", map[string]any{"value": []any{}}, map[string]any{"value": []any{""}}},
		{"key_value_separator", map[string]any{"a=b": "c"}, map[string]any{"a": "b=c"}},
		{"record_separator", map[string]any{"a": "x\n  STAT:b=y"}, map[string]any{"a": "x", "b": "y"}},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			if identity(sample.left) == identity(sample.right) {
				t.Fatalf("distinct semantic constants share one identity: %#v and %#v", sample.left, sample.right)
			}
		})
	}
	t.Run("deterministic_map_order", func(t *testing.T) {
		var left = map[string]any{"a": []any{false, "false", 0}, "b": map[string]any{"x": "", "y": true}}
		var right = map[string]any{}
		right["b"] = map[string]any{"y": true, "x": ""}
		right["a"] = []any{false, "false", 0}
		if identity(left) != identity(right) {
			t.Fatal("unchanged nested semantic values depend on map insertion order")
		}
	})
	t.Run("documentation_is_not_semantic", func(t *testing.T) {
		var action = *original
		action.Docs.Title += " documented"
		if NewRegistry([]*ActionSchema{&action}, nil).Fingerprint() != NewRegistry([]*ActionSchema{original}, nil).Fingerprint() {
			t.Fatal("presentation-only title changed semantic identity")
		}
	})
}

func TestRP08UnencodableConstantIdentityFailsClosed(t *testing.T) {
	var original, ok = DefaultRegistry().LookupAction("openURL")
	if !ok {
		t.Fatal("canonical action fixture missing")
	}
	var action = *original
	action.StaticParameters = map[string]any{"invalid": func() {}}
	defer func() {
		if recover() == nil {
			t.Error("unencodable semantic metadata received a usable fingerprint")
		}
	}()
	NewRegistry([]*ActionSchema{&action}, nil)
}

func TestRP08CustomRegistrationRejectsUnencodableMetadata(t *testing.T) {
	var original, ok = DefaultRegistry().LookupAction("openURL")
	if !ok {
		t.Fatal("canonical action fixture missing")
	}
	var registry = NewRegistry([]*ActionSchema{original}, nil)
	var before = registry.Fingerprint()
	var action = *original
	action.StaticParameters = map[string]any{"invalid": func() {}}
	var failure any
	func() {
		defer func() { failure = recover() }()
		registry.RegisterCustomAction(&action)
	}()
	if failure == nil {
		t.Error("unencodable custom metadata was accepted")
	}
	if after, found := registry.LookupAction(original.CallableName); !found || after != original {
		t.Error("failed custom registration published the invalid action")
	}
	if registry.Fingerprint() != before {
		t.Error("failed custom registration replaced the valid semantic identity")
	}
	if _, err := json.Marshal(registry.AllActions()); err != nil {
		t.Errorf("failed registration left unencodable consumer metadata: %v", err)
	}
}

func TestRP08RegistryCloneDetachesDefaultPresence(t *testing.T) {
	var original, ok = DefaultRegistry().LookupAction("openURL")
	if !ok || len(original.Parameters) == 0 {
		t.Fatal("canonical text parameter fixture missing")
	}
	for _, sample := range []struct {
		name  string
		value *string
	}{{"absent", nil}, {"empty", StringDefault("")}, {"text", StringDefault("original")}} {
		t.Run(sample.name, func(t *testing.T) {
			var action = *original
			action.Parameters = append([]ParameterSchema(nil), original.Parameters...)
			action.Parameters[0].Optional = true
			action.Parameters[0].DefaultValue = sample.value
			var base = NewRegistry([]*ActionSchema{&action}, nil)
			var clone = base.Clone()
			if clone.Fingerprint() != base.Fingerprint() {
				t.Fatal("copying an unchanged default changed semantic identity")
			}
			var copied, found = clone.LookupAction(original.CallableName)
			if !found || (copied.Parameters[0].DefaultValue == nil) != (sample.value == nil) {
				t.Fatal("cloned default presence changed")
			}
			if sample.value == nil {
				return
			}
			var before = *sample.value
			if *copied.Parameters[0].DefaultValue != before {
				t.Fatal("cloned default value changed")
			}
			if copied.Parameters[0].DefaultValue == sample.value {
				t.Error("snapshot shares a mutable default pointer")
			}
			*copied.Parameters[0].DefaultValue = "snapshot-only mutation"
			if *action.Parameters[0].DefaultValue != before {
				t.Error("snapshot mutation changed the supplied catalog default")
			}
		})
	}
}

func TestRP08CompilerMacroDefaultDescriptorsAreDetached(t *testing.T) {
	var registry = DefaultRegistry()
	var before = registry.CompilerMacroFingerprint()
	var checked = 0
	for _, descriptor := range registry.CompilerMacros() {
		var original = compilerMacroSchemas[descriptor.CallableName]
		for i := range descriptor.Parameters {
			var value = descriptor.Parameters[i].DefaultValue
			var originalValue = original.Parameters[i].DefaultValue
			if (value == nil) != (originalValue == nil) {
				t.Errorf("%s macro descriptor changed default presence", descriptor.CallableName)
				continue
			}
			if value == nil {
				continue
			}
			checked++
			if value == originalValue {
				t.Errorf("%s macro descriptor exposes mutable overlay default", descriptor.CallableName)
				continue
			}
			var saved = *originalValue
			*value = "descriptor-only mutation"
			if *originalValue != saved {
				t.Errorf("%s descriptor mutation reached the compiler macro overlay", descriptor.CallableName)
			}
		}
	}
	if checked == 0 || registry.CompilerMacroFingerprint() != before {
		t.Fatal("macro default observations missing or descriptor mutations changed executable identity")
	}
}

func TestRP08SemanticRecordBoundaryEncoding(t *testing.T) {
	var original, ok = DefaultRegistry().LookupAction("openURL")
	if !ok || len(original.Parameters) == 0 {
		t.Fatal("canonical action fixture missing")
	}
	var cases = []struct {
		name        string
		left, right func(*ActionSchema)
	}{
		{"enum_records", func(a *ActionSchema) { a.Parameters[0].EnumValues = []string{"first\n    E:second"} }, func(a *ActionSchema) { a.Parameters[0].EnumValues = []string{"first", "second"} }},
		{"parameter_fields", func(a *ActionSchema) { a.Parameters[0].ID, a.Parameters[0].Label = "first:second", "third" }, func(a *ActionSchema) { a.Parameters[0].ID, a.Parameters[0].Label = "first", "second:third" }},
		{"action_fields", func(a *ActionSchema) { a.CallableName, a.AppleIdentifier = "first:second", "third" }, func(a *ActionSchema) { a.CallableName, a.AppleIdentifier = "first", "second:third" }},
		{"accepted_form_elements", func(a *ActionSchema) { a.Parameters[0].AcceptedForms = []string{"literal,reference"} }, func(a *ActionSchema) { a.Parameters[0].AcceptedForms = []string{"literal", "reference"} }},
		{"finite_version_precision", func(a *ActionSchema) { a.MinVersion = 17.001 }, func(a *ActionSchema) { a.MinVersion = 17.002 }},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			var left, right = cloneActionSchema(original), cloneActionSchema(original)
			sample.left(left)
			sample.right(right)
			if NewRegistry([]*ActionSchema{left}, nil).Fingerprint() == NewRegistry([]*ActionSchema{right}, nil).Fingerprint() {
				t.Fatal("different public metadata fields or elements alias one semantic identity")
			}
		})
	}
}
