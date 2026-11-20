package main

import (
	"strings"
	"testing"
)

// The uploaded iPadOS 27 shortcut proves Apple's built-in Current Date
// can be used directly as Set Variable WFInput (WFTextTokenAttachment)
// and as Get Time Between Dates WFInput (WFTextTokenString).
// Both representations are deliberately distinct and must stay so.
func TestDebugTimerNativeDateInputsAcrossLegacyAndV2(t *testing.T) {
	const src = "debugStartTimer(\"api\")\ndebugStopTimer(\"api\")\ndebugShowLogs()\n"
	expected := []string{
		"setvariable",
		"gettimebetweendates", "appendvariable", "getvariable",
		"previewdocument",
	}
	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{"Legacy", true}, {"V2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := compileDebugFixture(t, src, tc.legacy)
			checkDebugFallthroughSequence(t, actions, tc.legacy, expected...)
			timerName, err := debugTimerVariableName("api")
			if err != nil {
				t.Fatal(err)
			}

			// Only ONE native action for debugStartTimer:
			// Set Variable -> Apple's built-in Current Date magic variable.
			saved := actions[0].WFWorkflowActionParameters
			if saved["WFVariableName"] != timerName {
				t.Fatalf("timer start saved to wrong variable: %v", saved)
			}
			savedToken := debugParamMap(saved["WFInput"])
			if savedToken["WFSerializationType"] != "WFTextTokenAttachment" {
				t.Fatalf("start CurrentDate must be WFTextTokenAttachment: %v", savedToken)
			}
			savedRef := debugParamMap(savedToken["Value"])
			if len(savedRef) != 1 || savedRef["Type"] != "CurrentDate" {
				t.Fatalf("start must embed CurrentDate directly (no producer UUID): %v", savedRef)
			}

			// Only ONE native Get Time Between Dates action at stop,
			// with built-in CurrentDate in its second (WFInput) date field.
			between := actions[1].WFWorkflowActionParameters
			if between["WFTimeUntilUnit"] != "Seconds" {
				t.Fatalf("invalid time unit: %v", between)
			}
			for _, key := range []string{"WFTimeUntilFromDate", "WFInput"} {
				token := debugParamMap(between[key])
				if token["WFSerializationType"] != "WFTextTokenString" {
					t.Fatalf("%s: Apple requires WFTextTokenString: %#v", key, token)
				}
				data := debugParamMap(token["Value"])
				if data["string"] != "\uFFFC" {
					t.Fatalf("%s: expected U+FFFC token string, got %v", key, data)
				}
				atts := debugParamMap(data["attachmentsByRange"])
				if len(atts) != 1 {
					t.Fatalf("%s: wrong attachmentsByRange: %v", key, atts)
				}
				ref := debugParamMap(atts["{0, 1}"])
				if key == "WFTimeUntilFromDate" {
					if ref["Type"] != "Variable" || ref["VariableName"] != timerName {
						t.Fatalf("start date is not saved timer variable: %v", ref)
					}
				} else {
					if len(ref) != 1 || ref["Type"] != "CurrentDate" {
						t.Fatalf("end date must use CurrentDate (no previous Date output): %v", ref)
					}
				}
			}
			elapsedUUID, ok := between["UUID"].(string)
			if !ok || elapsedUUID == "" {
				t.Fatalf("missing duration UUID: %v", between)
			}
			appendParams := actions[2].WFWorkflowActionParameters
			if appendParams["WFVariableName"] != debugLogVariableName {
				t.Fatalf("duration not logged to debug variable: %v", appendParams)
			}
			logged := debugParamMap(appendParams["WFInput"])
			loggedRef := debugParamMap(logged["Value"])
			if logged["WFSerializationType"] != "WFTextTokenAttachment" ||
				loggedRef["Type"] != "ActionOutput" ||
				loggedRef["OutputUUID"] != elapsedUUID ||
				loggedRef["OutputName"] != "Time Between Dates" {
				t.Fatalf("duration not connected to log: %v", logged)
			}
			result := debugParamMap(actions[3].WFWorkflowActionParameters["WFVariable"])
			value := debugParamMap(result["Value"])
			if value["OutputUUID"] != elapsedUUID {
				t.Fatalf("getvariable does not return duration: %v", value)
			}
			for _, action := range actions {
				if strings.HasSuffix(action.WFWorkflowActionIdentifier, ".date") ||
					strings.HasSuffix(action.WFWorkflowActionIdentifier, ".gettext") {
					t.Fatalf("timer macro inserted unnecessary Date/Text: %s", action.WFWorkflowActionIdentifier)
				}
			}
		})
	}
}

func TestDebugTimerTwoIndependentLabels(t *testing.T) {
	const src = "debugStartTimer(\"first\")\ndebugStartTimer(\"second\")\ndebugStopTimer(\"first\")\ndebugStopTimer(\"second\")\n"
	for _, legacy := range []bool{true, false} {
		acts := compileDebugFixture(t, src, legacy)
		want := []string{"setvariable", "setvariable",
			"gettimebetweendates", "appendvariable", "getvariable",
			"gettimebetweendates", "appendvariable", "getvariable"}
		checkDebugFallthroughSequence(t, acts, legacy, want...)
		nameFirst, err := debugTimerVariableName("first")
		if err != nil {
			t.Fatal(err)
		}
		nameSecond, err := debugTimerVariableName("second")
		if err != nil {
			t.Fatal(err)
		}
		if nameFirst == nameSecond {
			t.Fatal("distinct labels collide")
		}
		if acts[0].WFWorkflowActionParameters["WFVariableName"] != nameFirst ||
			acts[1].WFWorkflowActionParameters["WFVariableName"] != nameSecond {
			t.Fatal("timer start variable mismatch")
		}
		for _, it := range []struct {
			idx  int
			name string
		}{{2, nameFirst}, {5, nameSecond}} {
			tok := debugParamMap(acts[it.idx].WFWorkflowActionParameters["WFTimeUntilFromDate"])
			data := debugParamMap(tok["Value"])
			ref := debugParamMap(debugParamMap(data["attachmentsByRange"])["{0, 1}"])
			if ref["VariableName"] != it.name {
				t.Fatalf("legacy=%t: wrong timer label: %v", legacy, ref)
			}
		}
	}
}
