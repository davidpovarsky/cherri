package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Initial checkpoint: the previous inline consumer check, extracted without
// changing its predicate so the actual detached-graph controls can expose it.
func p15PaginateResultError(actions []ShortcutAction, legacy bool) error {
	if len(actions) < 2 {
		return fmt.Errorf("missing pagination result")
	}
	returned := actions[len(actions)-2].WFWorkflowActionParameters["UUID"]
	tail := debugParamMap(debugParamMap(actions[len(actions)-1].WFWorkflowActionParameters["WFVariable"])["Value"])
	if tail["OutputUUID"] != returned {
		return fmt.Errorf("consumer lost original typed aggregate result")
	}
	return nil
}

func TestP15PaginateResultOracleRejectsDetachedGraph(t *testing.T) {
	source := "let rows = paginate(\"https://example.invalid/pages/one.json\", \"rows\", \"following\", 3, 6)\ngetVariable(rows)\n"
	actions, err := compileBuildProfileFixture(t, source, false, BuildProfileDebug)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15M02MainVoidDebugError(actions); err != nil {
		t.Fatal(err)
	}
	if len(actions) < 3 || actions[len(actions)-2].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
		t.Fatal("actual pagination consumer missing")
	}
	if uuid, okay := actions[len(actions)-3].WFWorkflowActionParameters["UUID"].(string); !okay || uuid == "" {
		t.Fatal("actual aggregate result has no concrete producer UUID")
	}
	if err := p15PaginateResultError(actions, false); err != nil {
		t.Fatal("healthy actual graph", err)
	}
	for _, fault := range []string{"detached_consumer", "missing_consumer_input", "missing_producer_uuid"} {
		t.Run(fault, func(t *testing.T) {
			raw, err := json.Marshal(actions)
			if err != nil {
				t.Fatal(err)
			}
			var broken []ShortcutAction
			if err := json.Unmarshal(raw, &broken); err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "detached_consumer":
				broken[len(broken)-2].WFWorkflowActionParameters["WFVariable"] = p13TokenAttachment("unrelated-pagination-output", "List")
			case "missing_consumer_input":
				delete(broken[len(broken)-2].WFWorkflowActionParameters, "WFVariable")
			case "missing_producer_uuid":
				delete(broken[len(broken)-3].WFWorkflowActionParameters, "UUID")
			}
			if p15PaginateResultError(broken, false) == nil {
				t.Fatal("detached actual pagination graph accepted", fault)
			}
		})
	}
}
