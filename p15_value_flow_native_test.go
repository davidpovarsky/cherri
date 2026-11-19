package main

import (
	"fmt"
	"reflect"
	"testing"
)

func p15F03LiteralProducer(actions []ShortcutAction) (ShortcutAction, error) {
	if len(actions) == 0 {
		return ShortcutAction{}, fmt.Errorf("missing reference producer")
	}
	last := actions[len(actions)-1]
	ref := p15OutputReference(last.WFWorkflowActionParameters["WFVariable"])
	if last.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || ref["Type"] != "ActionOutput" {
		return ShortcutAction{}, fmt.Errorf("literal control did not retain its native producer")
	}
	for _, action := range actions[:len(actions)-1] {
		if action.WFWorkflowActionParameters["UUID"] == ref["OutputUUID"] {
			return action, nil
		}
	}
	return ShortcutAction{}, fmt.Errorf("literal control has a dangling output")
}

func p15F03ExactLiteralFields(action ShortcutAction) map[string]any {
	fields := map[string]any{}
	for key, value := range action.WFWorkflowActionParameters {
		if key != "UUID" && key != "CustomOutputName" {
			fields[key] = value
		}
	}
	return fields
}

// The independent control uses ordinary bindings, rather than the yield
// materializer. Every non-identity native field, including false/zero/empty
// presence and container row types, must agree at each chosen branch boundary.
func p15F03LiteralBranchOracle(actions []ShortcutAction, want []ShortcutAction) error {
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return err
	}
	last := actions[len(actions)-1]
	ref := p15OutputReference(last.WFWorkflowActionParameters["WFVariable"])
	end := -1
	for index, action := range actions[:len(actions)-1] {
		if action.WFWorkflowActionParameters["UUID"] == ref["OutputUUID"] {
			end = index
		}
	}
	if end < 0 || p15Int(actions[end].WFWorkflowActionParameters["WFControlFlowMode"]) != 2 {
		return fmt.Errorf("literal result consumer bypassed the control result")
	}
	control := actions[end]
	group := control.WFWorkflowActionParameters["GroupingIdentifier"]
	var separators []int
	start := -1
	for i, action := range actions[:end] {
		if action.WFWorkflowActionIdentifier != control.WFWorkflowActionIdentifier || action.WFWorkflowActionParameters["GroupingIdentifier"] != group {
			continue
		}
		switch p15Int(action.WFWorkflowActionParameters["WFControlFlowMode"]) {
		case 0:
			start = i
		case 1:
			separators = append(separators, i)
		}
	}
	var branchEnds []int
	switch control.WFWorkflowActionIdentifier {
	case "is.workflow.actions.conditional":
		if len(separators) != 1 {
			return fmt.Errorf("literal If lost its two branches")
		}
		branchEnds = []int{separators[0] - 1, end - 1}
	case "is.workflow.actions.choosefrommenu":
		if len(separators) != len(want) || separators[0] != start+1 {
			return fmt.Errorf("literal Menu lost its case boundaries")
		}
		for _, i := range separators[1:] {
			branchEnds = append(branchEnds, i-1)
		}
		branchEnds = append(branchEnds, end-1)
	default:
		return fmt.Errorf("unexpected literal branch action")
	}
	if start < 0 || len(branchEnds) != len(want) {
		return fmt.Errorf("literal result paths changed")
	}
	for i, index := range branchEnds {
		if index <= start {
			return fmt.Errorf("literal branch has no producer")
		}
		actual := actions[index]
		if actual.WFWorkflowActionIdentifier != want[i].WFWorkflowActionIdentifier ||
			!reflect.DeepEqual(p15F03ExactLiteralFields(actual), p15F03ExactLiteralFields(want[i])) {
			return fmt.Errorf("branch %d changed typed native value: got %#v want %#v", i, actual, want[i])
		}
	}
	return nil
}

func TestP15F03LiteralYieldNativeFields(t *testing.T) {
	for _, tc := range []struct{ name, left, right string }{
		{"empty_text", "\"\"", "\"VALUE\""},
		{"zero_number", "0", "17"},
		{"large_integer", "9007199254740993", "-2.5"},
		{"false", "false", "true"},
		{"list", "[\"A\",\"B\"]", "[\"C\"]"},
		{"map", "{\"flag\":false}", "{\"flag\":true}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var want []ShortcutAction
			for _, literal := range []string{tc.left, tc.right} {
				control, err := p15C03Actions("let expected="+literal+"\ngetVariable(expected)\n", false)
				if err != nil {
					t.Fatal(err)
				}
				producer, err := p15F03LiteralProducer(control)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, producer)
			}
			for _, expression := range []string{
				"if flag {yield " + tc.left + "} else {yield " + tc.right + "}",
				"menu(\"Pick\"){case \"A\"{yield " + tc.left + "} case \"B\"{yield " + tc.right + "}}",
			} {
				actions, err := p15C03Actions("let flag=true\nlet value="+expression+"\ngetVariable(value)\n", false)
				if err != nil {
					t.Fatal(err)
				}
				if err = p15F03LiteralBranchOracle(actions, want); err != nil {
					t.Fatal(err)
				}
				broken := p15C03Clone(t, actions)
				for i := range broken {
					for _, producer := range want {
						if broken[i].WFWorkflowActionIdentifier == producer.WFWorkflowActionIdentifier {
							for key := range p15F03ExactLiteralFields(producer) {
								delete(broken[i].WFWorkflowActionParameters, key)
							}
						}
					}
				}
				if p15F03LiteralBranchOracle(broken, want) == nil {
					t.Fatal("missing native literal value/presence passed the branch oracle")
				}
			}
		})
	}
}
