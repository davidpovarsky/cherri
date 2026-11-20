package main

import (
	"fmt"
	"strings"
	"testing"
)

func p15BindingResultTemplate() string {
	return "CHERRI_BINDING_CAPTURE_OK\nNUM_CAPTURED=\ufffc\nNUM_AGAIN=\ufffc\nCLIP_CAPTURED=\ufffc\nCLIP_AGAIN=\ufffc"
}

func p15BindingResultRanges() []string {
	var ranges []string
	for offset, char := range []rune(p15BindingResultTemplate()) {
		if char == '\ufffc' {
			ranges = append(ranges, fmt.Sprintf("{%d, 1}", offset))
		}
	}
	return ranges
}

func TestP15BindingCaptureTerminalCausality(t *testing.T) {
	actions := p15CompiledActions(t, p15BindingFixture(t, "CherriBindingCapturePOC"))
	for _, kind := range []string{"Variable", "Clipboard"} {
		t.Run(kind, func(t *testing.T) {
			t.Run("unchanged_control", func(t *testing.T) {
				if err := p15BindingSnapshotError(actions, kind); err != nil {
					t.Fatal(err)
				}
			})
			for _, mutation := range []string{"missing_terminal", "wrong_terminal_producer", "changed_result_label", "swapped_capture_fields", "transformed_capture", "extra_attachment"} {
				t.Run(mutation, func(t *testing.T) {
					broken := p15C01Clone(t, actions)
					resultIndex := -1
					var references map[string]any
					for index, action := range broken {
						text, refs, ok := p15BindingTokenText(p15C01Params(action)["WFTextActionText"])
						if ok && strings.HasPrefix(text, "CHERRI_BINDING_CAPTURE_OK\n") {
							resultIndex, references = index, refs
						}
					}
					if resultIndex < 0 || len(references) != 4 || len(broken) < 3 || broken[len(broken)-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setclipboard" || p15M02MainVoidBoundaryError(broken) != nil {
						t.Fatal("actual capture fixture has no exact observed terminal graph")
					}
					ranges := p15BindingResultRanges()
					switch mutation {
					case "missing_terminal":
						broken = append(broken[:len(broken)-2], broken[len(broken)-1])
					case "wrong_terminal_producer":
						terminal := p15BindingDescriptors(p15C01Params(broken[len(broken)-2])["WFInput"], "ActionOutput")
						if len(terminal) != 1 {
							t.Fatal("missing actual final clipboard input")
						}
						terminal[0]["OutputUUID"] = p15C01Params(broken[0])["UUID"]
					case "changed_result_label":
						body := p15C01Params(broken[resultIndex])["WFTextActionText"].(map[string]any)["Value"].(map[string]any)
						body["string"] = strings.Replace(body["string"].(string), "NUM_CAPTURED=", "NUM_WRONG___=", 1)
					case "swapped_capture_fields":
						references[ranges[0]], references[ranges[2]] = references[ranges[2]], references[ranges[0]]
					case "transformed_capture":
						target := 0
						if kind == "Clipboard" {
							target = 2
						}
						ref := references[ranges[target]].(map[string]any)
						ref["Aggrandizements"] = []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFURLContentItem"}}
					case "extra_attachment":
						references["{999, 1}"] = map[string]any{"Type": "Variable", "VariableName": "unrelated"}
					}
					if err := p15BindingSnapshotError(broken, kind); err == nil {
						t.Fatalf("actual native graph mutation escaped the capture oracle: %s", mutation)
					}
				})
			}
		})
	}
}

func p15BindingTerminalError(actions []map[string]any, kind, captureID string, later int, positions map[string]int) error {
	resultIndex := -1
	resultID := ""
	for index, action := range actions {
		identifier, _ := action["WFWorkflowActionIdentifier"].(string)
		switch identifier {
		case "is.workflow.actions.conditional", "is.workflow.actions.repeat.each", "is.workflow.actions.repeat.count", "is.workflow.actions.choosefrommenu":
			return fmt.Errorf("straight-line capture fixture acquired conditional execution")
		}
		parameters := p15C01Params(action)
		text, refs, ok := p15BindingTokenText(parameters["WFTextActionText"])
		if !ok || !strings.HasPrefix(text, "CHERRI_BINDING_CAPTURE_OK\n") {
			continue
		}
		if resultIndex >= 0 || identifier != "is.workflow.actions.gettext" || text != p15BindingResultTemplate() || len(refs) != 4 {
			return fmt.Errorf("capture result lost its single exact template and four observations")
		}
		resultIndex = index
		resultID, _ = parameters["UUID"].(string)
		if resultID == "" || index <= later {
			return fmt.Errorf("capture result has no exact producer after the mutation")
		}
		firstSlot := 0
		if kind == "Clipboard" {
			firstSlot = 2
		}
		for slot, key := range p15BindingResultRanges() {
			ref, ok := refs[key].(map[string]any)
			if !ok || ref["Type"] != "ActionOutput" {
				return fmt.Errorf("capture result field %d is not an output attachment", slot)
			}
			uuid, ok := ref["OutputUUID"].(string)
			at, exists := positions[uuid]
			if !ok || uuid == "" || !exists || at >= index {
				return fmt.Errorf("capture result field %d has no preceding producer", slot)
			}
			if transformed, exists := ref["Aggrandizements"]; exists {
				transforms, valid := transformed.([]any)
				if !valid || len(transforms) != 0 {
					return fmt.Errorf("capture result field %d transforms its captured value", slot)
				}
			}
			wantCapture := slot == firstSlot || slot == firstSlot+1
			if (uuid == captureID) != wantCapture {
				return fmt.Errorf("capture result field %d points to the wrong %s observation", slot, kind)
			}
		}
	}
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		return err
	}
	if resultIndex < 0 || resultIndex != len(actions)-3 {
		return fmt.Errorf("capture result is not immediately followed by its terminal clipboard write")
	}
	terminal := actions[len(actions)-2]
	if terminal["WFWorkflowActionIdentifier"] != "is.workflow.actions.setclipboard" {
		return fmt.Errorf("capture result has no terminal clipboard write")
	}
	wrapper, ok := p15C01Params(terminal)["WFInput"].(map[string]any)
	if !ok || wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("terminal clipboard input is not a native attachment")
	}
	ref, ok := wrapper["Value"].(map[string]any)
	if !ok || ref["Type"] != "ActionOutput" || ref["OutputUUID"] != resultID {
		return fmt.Errorf("terminal clipboard write does not consume the exact observed result")
	}
	if transformed, exists := ref["Aggrandizements"]; exists {
		transforms, valid := transformed.([]any)
		if !valid || len(transforms) != 0 {
			return fmt.Errorf("terminal clipboard write transforms its observed result")
		}
	}
	return nil
}
