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
