/*
 * Copyright (c) Cherri Language v2.0
 * Enum & Custom Action Wire Key Conformance Test Suite
 */

package main

import (
	"strings"
	"testing"

	"howett.net/plist"
)

func TestEnum_CustomActionWireKeysAndDefaults(t *testing.T) {
	// 1. Conformance with tests/enum.cherri
	enumCode := `enum callType {
    Audio,
    Video
}

action callOnFaceTime(contact: Variable, type: CallType = "Video") {
    identifier: "com.apple.facetime.facetime"
    bindings: {
        contact: {
            key: "WFFaceTimeContact"
        },
        type: {
            key: "WFFaceTimeType"
        },
    }
}

callOnFaceTime(Ask, "Audio")
`
	plistBytes, err := CompileSourceToPlist("enum.cherri", enumCode)
	if err != nil {
		t.Fatalf("failed to compile enum code: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	actions, ok := rawMap["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected workflow actions, got 0")
	}

	act := actions[0].(map[string]any)
	ident := act["WFWorkflowActionIdentifier"].(string)
	if ident != "com.apple.facetime.facetime" {
		t.Fatalf("expected identifier com.apple.facetime.facetime, got %s", ident)
	}

	params, ok := act["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		t.Fatalf("missing parameters dictionary")
	}

	// Must NOT contain empty key {"": "Audio"}
	if _, hasEmpty := params[""]; hasEmpty {
		t.Errorf("critical defect detected: parameter map contains empty key [\"\"]")
	}

	// Must map to WFFaceTimeType = "Audio"
	if callTypeVal, ok := params["WFFaceTimeType"].(string); !ok || callTypeVal != "Audio" {
		t.Errorf("expected WFFaceTimeType to be \"Audio\", got %v", params["WFFaceTimeType"])
	}

	// Must map to WFFaceTimeContact = Ask (WFSerializationType = WFTextTokenAttachment)
	contactVal, ok := params["WFFaceTimeContact"].(map[string]any)
	if !ok {
		t.Fatalf("missing WFFaceTimeContact in parameters: %v", params)
	}
	if contactVal["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Errorf("expected WFTextTokenAttachment serialization, got %v", contactVal["WFSerializationType"])
	}
	innerVal, ok := contactVal["Value"].(map[string]any)
	if !ok || innerVal["Type"] != "Ask" {
		t.Errorf("expected Value.Type == \"Ask\", got %v", contactVal["Value"])
	}
}

func TestEnum_DefaultParameterValueAppliedWhenOmitted(t *testing.T) {
	code := `enum callType { Audio, Video }

action callOnFaceTime(contact: Variable, type: CallType = "Video") {
    identifier: "com.apple.facetime.facetime"
    bindings: {
        contact: { key: "WFFaceTimeContact" },
        type: { key: "WFFaceTimeType" },
    }
}

callOnFaceTime(Ask)
`
	plistBytes, err := CompileSourceToPlist("enum_default.cherri", code)
	if err != nil {
		t.Fatalf("failed to compile code with default parameter: %v", err)
	}

	var rawMap map[string]any
	if _, err := plist.Unmarshal(plistBytes, &rawMap); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}

	actions := rawMap["WFWorkflowActions"].([]any)
	params := actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	if typeVal, ok := params["WFFaceTimeType"].(string); !ok || typeVal != "Video" {
		t.Errorf("expected default WFFaceTimeType to be \"Video\", got %v", params["WFFaceTimeType"])
	}
}

func TestEnum_Negative_MissingRequiredParameterFailsCompilation(t *testing.T) {
	negCode := `enum callType { Audio, Video }

action callOnFaceTime(contact: Variable, type: CallType = "Video") {
    identifier: "com.apple.facetime.facetime"
    bindings: {
        contact: { key: "WFFaceTimeContact" },
        type: { key: "WFFaceTimeType" },
    }
}

callOnFaceTime()
`
	_, err := CompileSourceToPlist("enum_neg.cherri", negCode)
	if err == nil {
		t.Fatalf("expected compile error for missing required parameter 'contact', got nil")
	}

	if !strings.Contains(err.Error(), "missing required argument") && !strings.Contains(err.Error(), "contact") {
		t.Errorf("expected error message to mention missing required argument 'contact', got: %v", err)
	}
}
