package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// This records only compiled probe structure. It cannot establish native Repeat
// Results nesting, and must never mint the full F04 acceptance result.
func TestP15F04NestedListProbeStructure(t *testing.T) {
	for _, name := range []string{"qa-f04-yield-list-v2", "qa-f04-yield-empty-list-v2"} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", name+".cherri")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"ANCHOR_CARD_01_TOP_HEADER", "ANCHOR_CARD_28_BOTTOM_FOOTER", "debugClearLogs()", "debugTraceEvent(stage: \"before\"", "debugTraceEvent(stage: \"after\"", "debugExportTrace(runId: \"" + name + "\""} {
				if !strings.Contains(string(source), required) {
					t.Fatal("required native-probe correlation structure missing", required)
				}
			}
			actions, err := p15C03Actions(string(source), false)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15LegacyInlineReferenceOracle(actions); err != nil {
				t.Fatal(err)
			}
			inputs := ""
			for _, a := range actions {
				if a.WFWorkflowActionIdentifier != "is.workflow.actions.list" {
					continue
				}
				items := p15C03Array(a.WFWorkflowActionParameters["WFItems"])
				if len(items) != 2 {
					continue
				}
				first, second := debugParamMap(items[0]), debugParamMap(items[1])
				if first["WFValue"] == "LEFT" && second["WFValue"] == "RIGHT" {
					if inputs != "" {
						t.Fatal("input producer duplicated")
					}
					inputs, _ = a.WFWorkflowActionParameters["UUID"].(string)
				}
			}
			if inputs == "" {
				t.Fatal("exact synthetic two-element input producer missing")
			}
			loops := 0
			for start, a := range actions {
				if a.WFWorkflowActionIdentifier != "is.workflow.actions.repeat.each" || p15Int(a.WFWorkflowActionParameters["WFControlFlowMode"]) != 0 {
					continue
				}
				ref := p15OutputReference(a.WFWorkflowActionParameters["WFInput"])
				if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != inputs {
					continue
				}
				loops++
				group := a.WFWorkflowActionParameters["GroupingIdentifier"]
				end := -1
				for i := start + 1; i < len(actions); i++ {
					x := actions[i]
					if x.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" && x.WFWorkflowActionParameters["GroupingIdentifier"] == group && p15Int(x.WFWorkflowActionParameters["WFControlFlowMode"]) == 2 {
						end = i
						break
					}
				}
				if end <= start+1 {
					t.Fatal("per-iteration producer/result boundary missing")
				}
				resultID, _ := actions[end].WFWorkflowActionParameters["UUID"].(string)
				if resultID == "" {
					t.Fatal("per-iteration producer/result boundary missing")
				}
				for _, x := range actions[start+1 : end] {
					if x.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
						t.Fatal("probe added an accumulator that could mask native Repeat Results")
					}
				}
				last := actions[end-1]
				if loops == 1 && name == "qa-f04-yield-list-v2" {
					if last.WFWorkflowActionIdentifier != "is.workflow.actions.list" || len(p15C03Array(last.WFWorkflowActionParameters["WFItems"])) != 2 {
						t.Fatal("first loop no longer yields the original two-element List")
					}
				} else if last.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
					t.Fatal("loop does not end with its explicit yielded value bridge")
				}
				resultUUID := actions[end].WFWorkflowActionParameters["UUID"]
				observed := false
				for _, consumer := range actions[end+1:] {
					if consumer.WFWorkflowActionIdentifier != "is.workflow.actions.count" {
						continue
					}
					ref := p15OutputReference(consumer.WFWorkflowActionParameters["Input"])
					if ref["Type"] == "ActionOutput" && ref["OutputUUID"] == resultUUID {
						observed = true
					}
				}
				if !observed {
					t.Fatal("native count does not observe the original Repeat Results UUID")
				}
			}
			if loops != 2 {
				t.Fatalf("need one List-yield loop and independent scalar control, got%d", loops)
			}
			t.Log("Compiled diagnostic only; semantic nested/empty List cardinality remains unverified until exact signed native observations.")
		})
	}
}
