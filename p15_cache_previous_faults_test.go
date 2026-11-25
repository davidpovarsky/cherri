package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// These mutations start from an actually compiled persistent cache. They change
// the capture/restore graph while keeping the authored source and body intact.
func TestP15CachePreviousSnapshotOracleFaults(t *testing.T) {
	tc := p15CachePreviousCases()[0]
	for _, legacy := range []bool{false, true} {
		for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, profile), func(t *testing.T) {
				baseline, err := p15CachePreviousCompile(t, p15CachePreviousSource(tc, legacy, 300, false), "snapshot-fault-baseline", legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				actual, err := p15CachePreviousCompile(t, p15CachePreviousSource(tc, legacy, 300, true), "snapshot-fault-positive", legacy, profile)
				if err != nil {
					t.Fatal(err)
				}
				if err = p15PreviousEquivalence(baseline, actual, tc, profile); err != nil {
					t.Fatal(err)
				}
				if err = p15CachePreviousSnapshotShape(actual, tc, 300); err != nil {
					t.Fatal(err)
				}
				capture, restore := -1, -1
				slot := ""
				for i, action := range actual {
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
						name, _ := action.WFWorkflowActionParameters["WFVariableName"].(string)
						if len(name) >= 6 && name[len(name)-6:] == "_input" {
							if capture >= 0 {
								t.Fatal("positive control has multiple captures")
							}
							capture, slot = i, name
						}
					}
				}
				for i, action := range actual {
					if action.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
						continue
					}
					ref, e := p15PreviousObject(action.WFWorkflowActionParameters["WFVariable"])
					if e == nil && ref["Type"] == "Variable" && ref["VariableName"] == slot {
						if restore >= 0 {
							t.Fatal("positive control has multiple restores")
						}
						restore = i
					}
				}
				if capture < 1 || restore <= capture || restore+1 >= len(actual) {
					t.Fatal("positive graph lacks its native snapshot pair")
				}
				for _, fault := range []string{
					"explicit_capture_input", "late_capture", "wrong_restore_slot",
					"restore_before_miss", "restore_after_body", "helper_after_restore",
				} {
					t.Run(fault, func(t *testing.T) {
						raw, e := json.Marshal(actual)
						if e != nil {
							t.Fatal(e)
						}
						var changed []ShortcutAction
						if e = json.Unmarshal(raw, &changed); e != nil {
							t.Fatal(e)
						}
						switch fault {
						case "explicit_capture_input":
							changed[capture].WFWorkflowActionParameters["WFInput"] = map[string]any{
								"WFSerializationType": "WFTextTokenAttachment",
								"Value": map[string]any{
									"Type":       "ActionOutput",
									"OutputUUID": changed[capture-1].WFWorkflowActionParameters["UUID"],
								},
							}
						case "late_capture":
							changed[capture], changed[capture+1] = changed[capture+1], changed[capture]
						case "wrong_restore_slot":
							ref, e := p15PreviousObject(changed[restore].WFWorkflowActionParameters["WFVariable"])
							if e != nil {
								t.Fatal(e)
							}
							ref["VariableName"] = "P15_FOREIGN_INPUT_SLOT"
						case "restore_before_miss":
							changed[restore], changed[restore-1] = changed[restore-1], changed[restore]
						case "restore_after_body":
							changed[restore], changed[restore+1] = changed[restore+1], changed[restore]
						case "helper_after_restore":
							helper := ShortcutAction{
								WFWorkflowActionIdentifier: "is.workflow.actions.number",
								WFWorkflowActionParameters: map[string]any{"UUID": "P15-CAUSAL-PRIVATE-HELPER", "WFNumberActionNumber": 99},
							}
							changed = append(changed[:restore+1], append([]ShortcutAction{helper}, changed[restore+1:]...)...)
						}
						if p15CachePreviousSnapshotShape(changed, tc, 300) == nil {
							t.Fatal("snapshot oracle accepted causal fault " + fault)
						}
						if fault != "explicit_capture_input" && fault != "restore_before_miss" &&
							p15PreviousEquivalence(baseline, changed, tc, profile) == nil {
							t.Fatal("authored-input oracle accepted changed Previous " + fault)
						}
					})
				}
			})
		}
	}
}
