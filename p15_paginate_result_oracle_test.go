package main

import (
	"encoding/json"
	"fmt"
	"testing"
)

// Follow the actual authored consumer and its prior native aggregate producer.
// Keep the main empty result in the graph and require a concrete owned UUID;
// two absent fields can never establish a producer-consumer relationship.
func p15PaginateResultError(actions []ShortcutAction, legacy bool) error {
	consumer := len(actions) - 1
	if !legacy {
		if err := p15M02MainVoidDebugError(actions); err != nil {
			return err
		}
		consumer--
	}
	if consumer < 1 || actions[consumer].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" ||
		actions[consumer-1].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
		return fmt.Errorf("pagination result producer/consumer order changed")
	}
	uuid, okay := actions[consumer-1].WFWorkflowActionParameters["UUID"].(string)
	if !okay || uuid == "" {
		return fmt.Errorf("pagination aggregate producer has no concrete UUID")
	}
	owners := 0
	for _, action := range actions {
		if action.WFWorkflowActionParameters["UUID"] == uuid {
			owners++
		}
	}
	if owners != 1 {
		return fmt.Errorf("pagination aggregate UUID has %d owners", owners)
	}
	input, okay := actions[consumer].WFWorkflowActionParameters["WFVariable"].(map[string]any)
	if !okay || len(input) != 2 || input["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("pagination consumer lost its exact native attachment")
	}
	value, okay := input["Value"].(map[string]any)
	if !okay || value["Type"] != "ActionOutput" || value["OutputUUID"] != uuid {
		return fmt.Errorf("pagination consumer is detached from its actual aggregate")
	}
	for key := range value {
		if key != "Type" && key != "OutputUUID" && key != "OutputName" {
			return fmt.Errorf("pagination result gained an unmodeled transformation")
		}
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
