package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Preserve the authored header producer and pagination's exact aggregate in the
// same complete graph. In particular, do not trim a prefix to satisfy the old
// initializer-at-index-zero assumption.
func TestP15PaginateTraversalHeaderResultOriginRejectsFaults(t *testing.T) {
	tc := p15PaginateTraversalCases[0]
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			source := p15PaginateTraversalSource(tc, legacy, false, true, 3, 6, true)
			actions, err := p15PaginateTraversalCompile(t, source, legacy, BuildProfileDebug)
			if err != nil {
				t.Fatal(err)
			}
			check := func(graph []ShortcutAction) error {
				if err := p15PaginateTraversalGraphError(graph, tc, legacy); err != nil {
					return err
				}
				return p15PaginateHeadersError(graph, true)
			}
			if err := check(actions); err != nil {
				t.Fatal("actual complete header/result graph", err)
			}
			if len(actions) < 3 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
				actions[0].WFWorkflowActionParameters["WFTextActionText"] != p15PaginateSyntheticAuthorization ||
				actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.list" ||
				actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" {
				t.Fatal("authored header and native initializer are not consecutive in source order")
			}
			for _, fault := range []string{
				"missing_header_source", "changed_header_source", "header_source_after_request",
				"missing_initializer", "nonempty_initializer", "store_before_initializer",
				"initializer_wrong_output", "duplicate_initializer_uuid", "missing_result_store",
				"duplicate_result_store", "read_other_aggregate", "append_header_instead_of_item",
			} {
				t.Run(fault, func(t *testing.T) {
					data, err := json.Marshal(actions)
					if err != nil {
						t.Fatal(err)
					}
					var broken []ShortcutAction
					if err := json.Unmarshal(data, &broken); err != nil {
						t.Fatal(err)
					}
					resultName := broken[2].WFWorkflowActionParameters["WFVariableName"]
					switch fault {
					case "missing_header_source":
						broken = broken[1:]
					case "changed_header_source":
						broken[0].WFWorkflowActionParameters["WFTextActionText"] = "CHANGED_AUTHORED_VALUE"
					case "header_source_after_request":
						request := -1
						for index, action := range broken {
							if action.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
								request = index
							}
						}
						if request < 1 {
							t.Fatal("actual request missing")
						}
						first := broken[0]
						broken = append(broken[1:request+1], append([]ShortcutAction{first}, broken[request+1:]...)...)
					case "missing_initializer":
						broken = append(broken[:1], broken[2:]...)
					case "nonempty_initializer":
						broken[1].WFWorkflowActionParameters["WFItems"] = []any{"FABRICATED"}
					case "store_before_initializer":
						broken[1], broken[2] = broken[2], broken[1]
					case "initializer_wrong_output":
						broken[2].WFWorkflowActionParameters["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = broken[0].WFWorkflowActionParameters["UUID"]
					case "duplicate_initializer_uuid":
						broken[0].WFWorkflowActionParameters["UUID"] = broken[1].WFWorkflowActionParameters["UUID"]
					case "missing_result_store":
						broken = append(broken[:2], broken[3:]...)
					case "duplicate_result_store":
						broken = append(broken[:3], append([]ShortcutAction{broken[2]}, broken[3:]...)...)
					case "read_other_aggregate":
						consumer := len(broken) - 1
						if !legacy {
							consumer--
						}
						other := ""
						for _, action := range broken {
							if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" &&
								action.WFWorkflowActionParameters["WFVariableName"] != resultName {
								other, _ = action.WFWorkflowActionParameters["WFVariableName"].(string)
								break
							}
						}
						if other == "" {
							t.Fatal("actual independent aggregate missing")
						}
						broken[consumer-1].WFWorkflowActionParameters["WFVariable"].(map[string]any)["Value"].(map[string]any)["VariableName"] = other
					case "append_header_instead_of_item":
						found := false
						for _, action := range broken {
							if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" &&
								action.WFWorkflowActionParameters["WFVariableName"] == resultName {
								action.WFWorkflowActionParameters["WFInput"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = broken[0].WFWorkflowActionParameters["UUID"]
								found = true
							}
						}
						if !found {
							t.Fatal("actual result append missing")
						}
					}
					if check(broken) == nil {
						t.Fatal("corrupted full header/result graph accepted", fault)
					}
				})
			}
		})
	}
}
