/*
 * Copyright (c) Cherri Language v2.0
 * Backend Recovery Initial Red Tests
 */

package main

import (
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/ir"
)

func TestBackendRecoveryInitial_TransformsSurviveEmission(t *testing.T) {
	want := []map[string]interface{}{
		{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"},
		{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"},
	}
	token := &ir.AttachmentToken{
		Type:            "ActionOutput",
		OutputUUID:      "producer-A",
		OutputName:      "Photo",
		Aggrandizements: want,
	}
	encoded, ok := transformIRParamValue(token).(map[string]any)
	if !ok {
		t.Fatal("expected encoded attachment map at audited entrypoint")
	}
	value, ok := encoded["Value"].(map[string]any)
	if !ok {
		t.Fatal("missing inner reference value")
	}
	if !reflect.DeepEqual(value["Aggrandizements"], want) {
		t.Fatalf("lost or changed transformations: got %#v; want %#v", value["Aggrandizements"], want)
	}
}

func TestBackendRecoveryInitial_RejectsUnrelatedTokens(t *testing.T) {
	output := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "producer-A"}}
	unrelated := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "DifferentContact"}}
	if semanticValueEqual(output, unrelated) {
		t.Fatal("comparator accepted unrelated producer and variable reference")
	}
}

func TestBackendRecoveryInitial_RejectsDifferentTokenText(t *testing.T) {
	output := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "producer-A"}}
	text := map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "unrelated literal", "attachmentsByRange": map[string]any{}}}
	if semanticValueEqual(output, text) {
		t.Fatal("comparator accepted a reference and unrelated token-string")
	}
}

func TestBackendRecoveryInitial_RejectsExtraKeysSymmetrically(t *testing.T) {
	a := map[string]any{"k": 1}
	b := map[string]any{"k": 1, "unexpected": 2}
	if semanticValueEqual(a, b) || semanticValueEqual(b, a) {
		t.Fatal("comparator accepted a changed key set")
	}
}
