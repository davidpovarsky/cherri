package main

import (
	"encoding/base64"
	"math"
	"os"
	"testing"

	"howett.net/plist"
)

func p15CompileLegacyNumberFixture(t *testing.T, source string) ([]byte, map[string]any) {
	t.Helper()
	resetCompilerStateFully()
	result := compileForMobileMode(source, "P15LegacyRawNativeNumbers", "legacy", false)
	if !result.OK {
		t.Fatalf("compile failed: %s", result.Error)
	}
	encoded, err := base64.StdEncoding.DecodeString(result.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err = plist.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	return encoded, document
}

func p15AssertNativeNumberKinds(t *testing.T, document map[string]any) {
	t.Helper()
	actions := document["WFWorkflowActions"].([]any)
	if len(actions) != 2 {
		t.Fatalf("want producer and raw action, got %d actions", len(actions))
	}
	producer := actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	params := actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	input := params["WFInput"].(map[string]any)["Value"].(map[string]any)
	if input["Type"] != "ActionOutput" || input["OutputUUID"] != producer["UUID"] {
		t.Fatal("raw numeric preservation broke the original producer reference")
	}
	integers := map[string]uint64{"WFInteger": 1, "WFZeroInteger": 0,
		"WFLargeInteger": 9007199254740993, "WFMaxUnsignedInteger": math.MaxUint64,
		"WFContentItemLimitNumber": 5}
	for key, expected := range integers {
		if actual, ok := params[key].(uint64); !ok || actual != expected {
			t.Errorf("%s = %T(%v), want integer(%d)", key, params[key], params[key], expected)
		}
	}
	if actual, ok := params["WFNegativeInteger"].(int64); !ok || actual != -7 {
		t.Errorf("negative integer changed: %T(%v)", params["WFNegativeInteger"], params["WFNegativeInteger"])
	}
	reals := map[string]float64{"WFIntegralReal": 1, "WFFractionalReal": 1.25,
		"WFExponentReal": 1000, "WFNegativeZeroReal": math.Copysign(0, -1)}
	for key, expected := range reals {
		actual, ok := params[key].(float64)
		if !ok || math.Float64bits(actual) != math.Float64bits(expected) {
			t.Errorf("%s = %T(%v), want exact real bits %016x", key, params[key], params[key], math.Float64bits(expected))
		}
	}
	if params["WFBoolean"] != false || params["WFNumericText"] != "1" {
		t.Fatal("Boolean or numeric Text was coerced")
	}
	nested := params["WFContentItemFilter"].(map[string]any)["Value"].(map[string]any)
	if value, ok := nested["WFActionParameterFilterPrefix"].(uint64); !ok || value != 1 {
		t.Errorf("filter prefix lost native integer type: %T", nested["WFActionParameterFilterPrefix"])
	}
	op := nested["WFActionParameterFilterTemplates"].([]any)[0].(map[string]any)["Operator"]
	if value, ok := op.(uint64); !ok || value != 4 {
		t.Errorf("filter operator lost native integer type: %T", op)
	}
	values := nested["Numbers"].([]any)
	expected := []any{uint64(1), float64(1), int64(-7), math.Copysign(0, -1), uint64(9007199254740993), uint64(math.MaxUint64)}
	for i, want := range expected {
		if !plistValuesStrictlyEqual(want, values[i]) {
			t.Errorf("nested number %d = %T(%v), want %T(%v)", i, values[i], values[i], want, want)
		}
	}
}

func TestP15LegacyRawActionLiteralNumberKinds(t *testing.T) {
	source, err := os.ReadFile("tests/fixtures/p15-legacy-raw-native-numbers.cherri")
	if err != nil {
		t.Fatal(err)
	}
	_, document := p15CompileLegacyNumberFixture(t, string(source))
	p15AssertNativeNumberKinds(t, document)
}

func TestP15LegacyRawActionDecompileNumberKinds(t *testing.T) {
	source, err := os.ReadFile("tests/fixtures/p15-legacy-raw-native-numbers.cherri")
	if err != nil {
		t.Fatal(err)
	}
	_, original := p15CompileLegacyNumberFixture(t, string(source))
	// Create the independent native witness even when the original parser is broken.
	// These kinds/values are the contract, not a normalization of observed output.
	actions := original["WFWorkflowActions"].([]any)
	params := actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	params["WFInteger"], params["WFNegativeInteger"], params["WFZeroInteger"] = uint64(1), int64(-7), uint64(0)
	params["WFLargeInteger"], params["WFMaxUnsignedInteger"] = uint64(9007199254740993), uint64(math.MaxUint64)
	params["WFIntegralReal"], params["WFFractionalReal"], params["WFExponentReal"], params["WFNegativeZeroReal"] = float64(1), float64(1.25), float64(1000), math.Copysign(0, -1)
	params["WFContentItemLimitNumber"] = uint64(5)
	nested := params["WFContentItemFilter"].(map[string]any)["Value"].(map[string]any)
	nested["WFActionParameterFilterPrefix"] = uint64(1)
	nested["WFActionParameterFilterTemplates"].([]any)[0].(map[string]any)["Operator"] = uint64(4)
	nested["Numbers"] = []any{uint64(1), float64(1), int64(-7), math.Copysign(0, -1), uint64(9007199254740993), uint64(math.MaxUint64)}
	for name, format := range map[string]int{"xml": plist.XMLFormat, "binary": plist.BinaryFormat} {
		t.Run(name, func(t *testing.T) {
			input, err := plist.Marshal(original, format)
			if err != nil {
				t.Fatal(err)
			}
			resetCompilerStateFully()
			result := decompileForMobile(input, "P15LegacyRawNativeNumbers")
			if !result.OK {
				t.Fatalf("decompile failed: %s", result.Error)
			}
			output, document := p15CompileLegacyNumberFixture(t, result.Source)
			p15AssertNativeNumberKinds(t, document)
			comparison, err := CompareShortcutPlistsFullWorkflowSemantic(input, output)
			if err != nil || !comparison.Equal {
				t.Fatalf("strict native roundtrip failed: err=%v comparison=%+v", err, comparison)
			}
		})
	}
}
