package main

import (
	"fmt"
	"reflect"
	"testing"
)

// Semantic import may uniquify source identifiers while each native output
// keeps its UUID-bound Number -> List -> Statistics dataflow. Validate that
// graph independently; the existing hybrid import path proves metadata equality.
func p15StatisticInputGraph(actions []ShortcutAction, operation string) error {
	values, err := p15NumericListValues(actions)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(values, []string{"10", "20", "30"}) {
		return fmt.Errorf("statistic numeric values/order changed: %v", values)
	}
	producers := map[string]int{}
	listID := ""
	listIndex := -1
	statIndex := -1
	var stat map[string]any
	for i, a := range actions {
		p := a.WFWorkflowActionParameters
		if id, ok := p["UUID"].(string); ok && id != "" {
			if _, exists := producers[id]; exists {
				return fmt.Errorf("duplicate producer UUID")
			}
			producers[id] = i
		}
		switch a.WFWorkflowActionIdentifier {
		case "is.workflow.actions.list":
			if listIndex != -1 {
				return fmt.Errorf("multiple List producers")
			}
			listIndex, listID = i, fmt.Sprint(p["UUID"])
		case "is.workflow.actions.statistics":
			if stat != nil {
				return fmt.Errorf("multiple statistics consumers")
			}
			stat, statIndex = p, i
		}
	}
	if stat == nil || listID == "" || listIndex >= statIndex {
		return fmt.Errorf("missing or out-of-order List -> Statistics graph")
	}
	if stat["WFStatisticsOperation"] != operation || stat["WFInput"] != nil {
		return fmt.Errorf("statistics operation/wire key changed")
	}
	wrapper, ok := stat["Input"].(map[string]any)
	if !ok || wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("statistics lost typed input attachment")
	}
	value, ok := wrapper["Value"].(map[string]any)
	if !ok || value["Type"] != "ActionOutput" || value["OutputUUID"] != listID || value["Aggrandizements"] != nil {
		return fmt.Errorf("statistics does not consume original native List")
	}
	return nil
}

func p15CheckStatisticDocument(t *testing.T, doc map[string]any, operation string) {
	t.Helper()
	var actions []ShortcutAction
	for _, raw := range doc["WFWorkflowActions"].([]any) {
		action := raw.(map[string]any)
		// Parameterless native actions may omit the dictionary in release output.
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		actions = append(actions, ShortcutAction{
			WFWorkflowActionIdentifier: action["WFWorkflowActionIdentifier"].(string),
			WFWorkflowActionParameters: parameters,
		})
	}
	if err := p15StatisticInputGraph(actions, operation); err != nil {
		t.Fatal(err)
	}
}

func TestP15StatisticInputGraphRejectsCausalCorruption(t *testing.T) {
	source := "let nums = [10, 20, 30]\nstatistic(\"Sum\", input: nums)"
	good := compileDebugFixture(t, source, false)
	if err := p15StatisticInputGraph(good, "Sum"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"wrong_number", "wrong_operation", "retarget_consumer", "unsupported_input", "duplicate_producer"} {
		t.Run(fault, func(t *testing.T) {
			actions := compileDebugFixture(t, source, false)
			numbers := p15ActionsNamed(actions, "number")
			stat := p15ActionsNamed(actions, "statistics")[0].WFWorkflowActionParameters
			switch fault {
			case "wrong_number":
				numbers[1].WFWorkflowActionParameters["WFNumberActionNumber"] = 21
			case "wrong_operation":
				stat["WFStatisticsOperation"] = "Maximum"
			case "retarget_consumer":
				stat["Input"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = numbers[0].WFWorkflowActionParameters["UUID"]
			case "unsupported_input":
				stat["WFInput"] = stat["Input"]
				delete(stat, "Input")
			case "duplicate_producer":
				numbers[1].WFWorkflowActionParameters["UUID"] = numbers[0].WFWorkflowActionParameters["UUID"]
			}
			if err := p15StatisticInputGraph(actions, "Sum"); err == nil {
				t.Fatal("accepted corrupt statistic graph")
			}
		})
	}
}
