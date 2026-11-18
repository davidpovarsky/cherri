package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestP15UnaryNumberLiteralNative(t *testing.T) {
	nativeID, nativeKey := p15T03AppleNumberContract(t)
	for _, tc := range []struct{ expression, want string }{
		{"-1", "-1"}, {"-0", "0"}, {"-1.25", "-1.25"},
		{"-9223372036854775807", "-9223372036854775807"},
		{"-9223372036854775808", "-9223372036854775808"},
		{"-(-9223372036854775808)", "9223372036854775808"},
		{"18446744073709551615", "18446744073709551615"},
	} {
		t.Run(tc.expression, func(t *testing.T) {
			actions := compileDebugFixture(t, "let value=number("+tc.expression+")\nwait(value)", false)
			if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != nativeID || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.delay" {
				t.Fatalf("literal negation added a runtime producer: %#v", actions)
			}
			value := actions[0].WFWorkflowActionParameters[nativeKey]
			validType := false
			switch value.(type) {
			case int64, uint64:
				validType = !strings.Contains(tc.want, ".")
			case float64:
				validType = strings.Contains(tc.want, ".")
			}
			if !validType || fmt.Sprint(value) != tc.want {
				t.Fatalf("literal numeric wire changed: %T %#v, want %s", value, value, tc.want)
			}
			uuid, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
			if uuid == "" || requireOutputUUID(actions[1].WFWorkflowActionParameters["WFDelayTime"]) != uuid {
				t.Fatal("exact literal producer reference changed")
			}
		})
	}
	// A runtime operand still requires the existing Math producer.
	actions := compileDebugFixture(t, "let original=number(\"3\")\nlet negative=-original\nwait(negative)", false)
	if len(actions) != 3 || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.math" {
		t.Fatalf("runtime negation path changed: %#v", actions)
	}
	operand := actions[1].WFWorkflowActionParameters
	originalUUID, _ := actions[0].WFWorkflowActionParameters["UUID"].(string)
	mathUUID, _ := operand["UUID"].(string)
	if originalUUID == "" || mathUUID == "" || operand["WFMathOperation"] != "-" ||
		requireOutputUUID(operand["WFMathOperand"]) != originalUUID ||
		requireOutputUUID(actions[2].WFWorkflowActionParameters["WFDelayTime"]) != mathUUID {
		t.Fatalf("runtime negation producer identity changed: %#v", actions)
	}
}

func TestP15UnaryQuantityLiteralNative(t *testing.T) {
	dateActions := compileDebugFixture(t, "if CurrentDate > qty(-1, \"days\") {}", false)
	if err := p15DateQuantityOracle(dateActions, "days", "-1", "1000"); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		source := "adjustDate(CurrentDate, \"Add\", qty(-1, \"days\"))"
		if legacy {
			source = "#include 'actions/calendar'\n" + source
		}
		actions := compileDebugFixture(t, source, legacy)
		if len(actions) != 1 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.adjustdate" {
			t.Fatalf("scalar duration gained a Math producer: %#v", actions)
		}
		quantity := debugParamMap(actions[0].WFWorkflowActionParameters["WFDuration"])
		value := debugParamMap(quantity["Value"])
		if quantity["WFSerializationType"] != "WFQuantityFieldValue" || value["Magnitude"] != "-1" || value["Unit"] != "days" {
			t.Fatalf("negative scalar duration was stringified from a reference: %#v", quantity)
		}
	}
}

func TestP15UnaryIntegerOverflowRejected(t *testing.T) {
	for _, expression := range []string{"-9223372036854775809", "-18446744073709551615", "-18446744073709551616", "-(+18446744073709551616)"} {
		source := "let value=number(" + expression + ")\nwait(value)"
		raw, err := CompileSourceToPlist("p15-unary-overflow.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "negative integer literal exceeds signed 64-bit range") {
			t.Fatalf("%s: unchecked integer overflow emitted bytes=%d err=%v", expression, len(raw), err)
		}
	}
}
