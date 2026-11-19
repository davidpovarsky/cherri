package main

import (
	"fmt"
	"testing"
)

// The Apple runtime diagnostic at run 44466065158 observed two entries after
// debugClearLogs plus one logged Text: the old empty Text seed was a real item.
// The independent nil-clear control at run 44401936674 verified Nothing followed
// by Set Variable without WFInput as native zero-item clearing.
func TestP15DebugClearLogsLeavesNoSyntheticEntry(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, source := range []string{
			"debugClearLogs()\ndebugExport()\n",
			"text(\"discarded\")\ndebugLog()\ndebugClearLogs()\ndebugExport()\n",
			"debugClearLogs()\ntext(\"\")\ndebugLog()\ndebugExport()\n",
		} {
			t.Run(fmt.Sprintf("legacy=%t/source=%s", legacy, source), func(t *testing.T) {
				actions := compileDebugFixture(t, source, legacy)
				clearCount := 0
				for i, action := range actions {
					p := action.WFWorkflowActionParameters
					if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
						p["WFVariableName"] != debugLogVariableName {
						continue
					}
					clearCount++
					if i == 0 || actions[i-1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" {
						t.Fatal("clearing the log must not insert a synthetic empty Text item")
					}
					if _, present := p["WFInput"]; present {
						t.Fatal("clear must omit WFInput, matching the native empty-value control")
					}
					if _, present := actions[i-1].WFWorkflowActionParameters["WFInput"]; present {
						t.Fatal("Nothing must not consume the preceding logged value")
					}
				}
				if clearCount != 1 {
					t.Fatalf("expected one explicit clear, got %d", clearCount)
				}
				// Actual empty Text is valid user input; clearing fixes the seed,
				// not a blanket filter that silently removes logged empty values.
				if source == "debugClearLogs()\ntext(\"\")\ndebugLog()\ndebugExport()\n" {
					if actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
						actions[2].WFWorkflowActionParameters["WFTextActionText"] != "" ||
						actions[3].WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" {
						t.Fatal("an explicitly logged empty Text must be preserved")
					}
					ref := debugParamMap(debugParamMap(actions[3].WFWorkflowActionParameters["WFInput"])["Value"])
					if ref["OutputUUID"] != actions[2].WFWorkflowActionParameters["UUID"] {
						t.Fatal("empty Text log lost its exact native producer")
					}
				}
			})
		}
	}
}

// validateP15DiagnosticZeroClear permits the single run-local debug clear only.
// Nothing is not a general allowed effect: its exact adjacent store has no input,
// and no action may consume its empty output as an authored value.
func validateP15DiagnosticZeroClear(actions []ShortcutAction) (int, error) {
	clearIndex := -1
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.nothing" {
			continue
		}
		if clearIndex != -1 || i+1 >= len(actions) {
			return -1, fmt.Errorf("extra or unpaired Nothing action")
		}
		p := action.WFWorkflowActionParameters
		id, _ := p["UUID"].(string)
		if len(p) != 1 || id == "" {
			return -1, fmt.Errorf("zero clear Nothing must have only its output identity")
		}
		next := actions[i+1]
		if next.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			len(next.WFWorkflowActionParameters) != 1 ||
			next.WFWorkflowActionParameters["WFVariableName"] != debugLogVariableName {
			return -1, fmt.Errorf("zero clear requires adjacent input-free debug log store")
		}
		clearIndex = i
	}
	if clearIndex == -1 {
		return -1, fmt.Errorf("missing zero-item debug clear")
	}
	clearUUID := actions[clearIndex].WFWorkflowActionParameters["UUID"]
	consumed := false
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			if value["OutputUUID"] == clearUUID {
				consumed = true
			}
			for _, child := range value {
				walk(child)
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" &&
			action.WFWorkflowActionParameters["WFVariableName"] == debugLogVariableName &&
			i != clearIndex+1 {
			return -1, fmt.Errorf("extra debug log initializer")
		}
		walk(action.WFWorkflowActionParameters)
	}
	if consumed {
		return -1, fmt.Errorf("zero-clear output must not feed an authored value")
	}
	return clearIndex, nil
}
