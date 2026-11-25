package main

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestConfirmedTextFormsFollowNativeStateEnvelope(t *testing.T) {
	raw, err := os.ReadFile("../../docs/apple-runtime/macos-27/encoding-table.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		StateClasses map[string]map[string]any
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range []struct {
		class, state, serialization string
		forms                       []string
	}{
		{"WFTextInputParameter", "WFVariableStringParameterState", "WFTextTokenString", []string{"literal", "tokenString"}},
		{"WFNumberFieldParameter", "WFNumberStringSubstitutableState", "WFTextTokenAttachment", []string{"literal", "attachment(output)", "attachment(variable)"}},
	} {
		for _, sample := range []string{"variable:MyVar", "variable:ActionOutput"} {
			if table.StateClasses[fixture.state][sample].(map[string]any)["WFSerializationType"] != fixture.serialization {
				t.Fatalf("native %s.%s envelope changed", fixture.state, sample)
			}
		}
		for _, variableTypes := range [][]string{nil, {"Variable", "ActionOutput"}} {
			forms, confirmed := determineConfirmedForms(&AppleParameterEncoding{Class: fixture.class, SingleStateClass: fixture.state, VariableTypes: variableTypes}, "text")
			if !confirmed || !reflect.DeepEqual(forms, fixture.forms) {
				t.Errorf("%s with variable kinds %v: got %v confirmed=%v; native envelope requires %v", fixture.state, variableTypes, forms, confirmed, fixture.forms)
			}
		}
	}
}
