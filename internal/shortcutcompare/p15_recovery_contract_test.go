package shortcutcompare

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func p15RecoveryParameters(doc map[string]any, index int) map[string]any {
	return doc["WFWorkflowActions"].([]any)[index].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
}

func p15RecoveryInputValue(doc map[string]any) map[string]any {
	return p15RecoveryParameters(doc, 1)["WFInput"].(map[string]any)["Value"].(map[string]any)
}

func p15RecoveryEqualBothWays(t *testing.T, a, b map[string]any) {
	t.Helper()
	for _, pair := range [][2]map[string]any{{a, b}, {b, a}} {
		var result = CompareDocumentsStrict(pair[0], pair[1])
		if !result.Equal || len(result.Differences) != 0 {
			t.Fatalf("valid native pair rejected: %+v", result)
		}
	}
}

func p15RecoveryRejectBothWays(t *testing.T, a, b map[string]any, path string) {
	t.Helper()
	for direction, pair := range [][2]map[string]any{{a, b}, {b, a}} {
		var result = CompareDocumentsStrict(pair[0], pair[1])
		if result.Equal || len(result.Differences) == 0 || result.Truncated {
			t.Fatalf("direction %d accepted mutation at %s: %+v", direction, path, result)
		}
		var encoded, err = json.Marshal(result)
		if err != nil {
			t.Fatal(err)
		}
		var wire struct {
			Differences []map[string]any
		}
		if err := json.Unmarshal(encoded, &wire); err != nil {
			t.Fatal(err)
		}
		for _, difference := range wire.Differences {
			_, hasA := difference["a"]
			_, hasB := difference["b"]
			if !hasA || !hasB {
				t.Fatalf("serialized diagnostics lost false/zero/absent expected or actual value: %s", encoded)
			}
		}
		var found bool
		for _, difference := range result.Differences {
			if strings.HasPrefix(difference.Path, path) && difference.Kind != "" &&
				!reflect.DeepEqual(difference.A, difference.B) {
				found = true
			}
		}
		if !found {
			t.Fatalf("direction %d has no expected/actual mismatch at %s: %+v", direction, path, result.Differences)
		}
	}
}

// BRG02 executes the shared strict comparator, including the positive bijection
// control and each contract mutation independently in both directions.
func TestP15BRG02ReferenceTopologyProduction(t *testing.T) {
	t.Run("bijective_renaming", func(t *testing.T) {
		var a, b = validBaseDoc(), validBaseDoc()
		p15RecoveryEqualBothWays(t, a, b)
		p15RecoveryParameters(b, 0)["UUID"] = "BBBBBBBB-1111-1111-1111-BBBBBBBBBBBB"
		p15RecoveryParameters(b, 1)["UUID"] = "BBBBBBBB-2222-2222-2222-BBBBBBBBBBBB"
		p15RecoveryInputValue(b)["OutputUUID"] = p15RecoveryParameters(b, 0)["UUID"]
		for _, index := range []int{2, 3} {
			p15RecoveryParameters(b, index)["GroupingIdentifier"] = "CCCCCCCC-AAAA-AAAA-AAAA-CCCCCCCCCCCC"
		}
		p15RecoveryEqualBothWays(t, a, b)
	})
	for _, scenario := range []struct {
		name, path string
		prepare    func(map[string]any)
		mutate     func(map[string]any)
	}{
		{
			name: "swapped_producer", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.Value.OutputUUID",
			mutate: func(doc map[string]any) {
				p15RecoveryInputValue(doc)["OutputUUID"] = p15RecoveryParameters(doc, 1)["UUID"]
			},
		},
		{
			name: "dangling_reference", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.Value.OutputUUID",
			mutate: func(doc map[string]any) {
				p15RecoveryInputValue(doc)["OutputUUID"] = "DEADBEEF-DEAD-BEEF-DEAD-BEEFDEADBEEF"
			},
		},
		{
			name: "scoped_variable", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.Value.VariableName",
			prepare: func(doc map[string]any) {
				p15RecoveryParameters(doc, 1)["WFInput"] = map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "Variable", "VariableName": "outerValue"},
				}
			},
			mutate: func(doc map[string]any) { p15RecoveryInputValue(doc)["VariableName"] = "innerValue" },
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			// Fresh typed fixtures avoid JSON cloning silently changing every integer.
			var a, b = validBaseDoc(), validBaseDoc()
			if scenario.prepare != nil {
				scenario.prepare(a)
				scenario.prepare(b)
			}
			p15RecoveryEqualBothWays(t, a, b)
			scenario.mutate(b)
			p15RecoveryRejectBothWays(t, a, b, scenario.path)
		})
	}
}

// BRG03 asserts native tree meaning, not stringified or normalized equality.
// Every negative begins with the same typed pair passing in both directions.
func TestP15BRG03TypedTreeProduction(t *testing.T) {
	for _, scenario := range []struct {
		name, path string
		prepare    func(map[string]any)
		mutate     func(map[string]any)
	}{
		{
			name: "nested_key_set", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.Value.extra",
			mutate: func(doc map[string]any) { p15RecoveryInputValue(doc)["extra"] = false },
		},
		{
			name: "native_number_to_text", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFNumberActionNumber",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = uint64(42) },
			mutate:  func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = "42" },
		},
		{
			name: "native_integer_to_real", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFNumberActionNumber",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = uint64(42) },
			mutate:  func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = float64(42) },
		},
		{
			name: "large_integer_precision", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFNumberActionNumber",
			prepare: func(doc map[string]any) {
				p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = uint64(9007199254740992)
			},
			mutate: func(doc map[string]any) {
				p15RecoveryParameters(doc, 0)["WFNumberActionNumber"] = uint64(9007199254740993)
			},
		},
		{
			name: "native_data_to_array", path: "WFWorkflowActions[0].WFWorkflowActionParameters.BinaryPayload",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["BinaryPayload"] = []byte{1, 2} },
			mutate:  func(doc map[string]any) { p15RecoveryParameters(doc, 0)["BinaryPayload"] = []any{uint8(1), uint8(2)} },
		},
		{
			name: "false_omission", path: "WFWorkflowActions[0].WFWorkflowActionParameters.Optional",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["Optional"] = false },
			mutate:  func(doc map[string]any) { delete(p15RecoveryParameters(doc, 0), "Optional") },
		},
		{
			name: "empty_text_omission", path: "WFWorkflowActions[0].WFWorkflowActionParameters.Optional",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["Optional"] = "" },
			mutate:  func(doc map[string]any) { delete(p15RecoveryParameters(doc, 0), "Optional") },
		},
		{
			name: "token_kind", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.WFSerializationType",
			mutate: func(doc map[string]any) {
				p15RecoveryParameters(doc, 1)["WFInput"].(map[string]any)["WFSerializationType"] = "WFTextTokenString"
			},
		},
		{
			name: "literal_text", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFTextActionText",
			mutate: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFTextActionText"] = "Different literal" },
		},
		{
			name: "escaped_looking_literal", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFTextActionText",
			prepare: func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFTextActionText"] = "#" },
			mutate:  func(doc map[string]any) { p15RecoveryParameters(doc, 0)["WFTextActionText"] = "u0023" },
		},
		{
			name: "uuid_shaped_literal", path: "WFWorkflowActions[0].WFWorkflowActionParameters.LiteralID",
			mutate: func(doc map[string]any) {
				p15RecoveryParameters(doc, 0)["LiteralID"] = "88888888-8888-8888-8888-888888888888"
			},
		},
		{
			name: "transform_order", path: "WFWorkflowActions[1].WFWorkflowActionParameters.WFInput.Value.Aggrandizements[0]",
			mutate: func(doc map[string]any) {
				var transforms = p15RecoveryInputValue(doc)["Aggrandizements"].([]any)
				transforms[0], transforms[1] = transforms[1], transforms[0]
			},
		},
		{
			name: "utf16_attachment_range", path: "WFWorkflowActions[0].WFWorkflowActionParameters.WFTextActionText.Value.attachmentsByRange",
			prepare: func(doc map[string]any) {
				p15RecoveryParameters(doc, 0)["WFTextActionText"] = map[string]any{
					"WFSerializationType": "WFTextTokenString",
					"Value": map[string]any{
						"string":             "🍎 \uFFFC",
						"attachmentsByRange": map[string]any{"{3, 1}": map[string]any{"Type": "Variable", "VariableName": "value"}},
					},
				}
			},
			mutate: func(doc map[string]any) {
				var value = p15RecoveryParameters(doc, 0)["WFTextActionText"].(map[string]any)["Value"].(map[string]any)
				var ranges = value["attachmentsByRange"].(map[string]any)
				ranges["{4, 1}"] = ranges["{3, 1}"]
				delete(ranges, "{3, 1}")
			},
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			var a, b = validBaseDoc(), validBaseDoc()
			if scenario.prepare != nil {
				scenario.prepare(a)
				scenario.prepare(b)
			}
			p15RecoveryEqualBothWays(t, a, b)
			scenario.mutate(b)
			p15RecoveryRejectBothWays(t, a, b, scenario.path)
		})
	}
}

func TestP15StrictComparisonKeepsExplicitParityBehavior(t *testing.T) {
	var a, b = validBaseDoc(), validBaseDoc()
	p15RecoveryParameters(a, 0)["WFNumberActionNumber"] = uint64(42)
	p15RecoveryParameters(b, 0)["WFNumberActionNumber"] = "42"
	p15RecoveryParameters(a, 0)["WFTextActionText"] = "#"
	p15RecoveryParameters(b, 0)["WFTextActionText"] = "u0023"
	if result := CompareDocumentsNormalized(a, b); !result.Equal {
		t.Fatalf("explicit parity behavior changed: %+v", result)
	}
	p15RecoveryRejectBothWays(t, a, b, "WFWorkflowActions[0].WFWorkflowActionParameters.WFNumberActionNumber")
}

func TestP15StrictComparisonNativeIntegerWidths(t *testing.T) {
	var a, b = validBaseDoc(), validBaseDoc()
	p15RecoveryParameters(a, 0)["WFNumberActionNumber"] = int64(42)
	p15RecoveryParameters(b, 0)["WFNumberActionNumber"] = uint64(42)
	p15RecoveryEqualBothWays(t, a, b)
}
