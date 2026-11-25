package main

import (
	"encoding/json"
	"fmt"
	"reflect"
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
	// Bind the result read to the actual empty List initializer and the
	// native item append in this fixture. An arbitrary existing variable
	// (including the separate seen-URL list) is not its result aggregate.
	attachment := func(raw any, kind string) (map[string]any, error) {
		outer, okay := raw.(map[string]any)
		if !okay || len(outer) != 2 || outer["WFSerializationType"] != "WFTextTokenAttachment" {
			return nil, fmt.Errorf("pagination aggregate lost native attachment")
		}
		value, okay := outer["Value"].(map[string]any)
		if !okay || value["Type"] != kind {
			return nil, fmt.Errorf("pagination aggregate attachment kind changed")
		}
		key := "OutputUUID"
		if kind == "Variable" {
			key = "VariableName"
		}
		id, okay := value[key].(string)
		if !okay || id == "" {
			return nil, fmt.Errorf("pagination aggregate attachment has no concrete identity")
		}
		for field := range value {
			if field != "Type" && field != key && !(kind == "ActionOutput" && field == "OutputName") {
				return nil, fmt.Errorf("pagination aggregate attachment gained a transformation")
			}
		}
		return value, nil
	}
	// Follow the actual final aggregate read to its unique declaration. Authored
	// input producers (for example a runtime authorization header) may precede
	// pagination; they remain in the full graph and all ownership checks below.
	read, err := attachment(actions[consumer-1].WFWorkflowActionParameters["WFVariable"], "Variable")
	if err != nil {
		return fmt.Errorf("pagination result lost its exact aggregate: %w", err)
	}
	name := read["VariableName"].(string)
	storeIndex := -1
	for index, action := range actions[:consumer-1] {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			action.WFWorkflowActionParameters["WFVariableName"] != name {
			continue
		}
		if storeIndex != -1 {
			return fmt.Errorf("pagination aggregate has multiple declarations")
		}
		storeIndex = index
	}
	if storeIndex < 1 || actions[storeIndex-1].WFWorkflowActionIdentifier != "is.workflow.actions.list" {
		return fmt.Errorf("pagination aggregate initialization order changed")
	}
	initializer := actions[storeIndex-1].WFWorkflowActionParameters
	items := reflect.ValueOf(initializer["WFItems"])
	if !items.IsValid() || items.Kind() != reflect.Slice || items.IsNil() || items.Len() != 0 {
		return fmt.Errorf("pagination aggregate must start as an actual empty List")
	}
	emptyID, okay := initializer["UUID"].(string)
	if !okay || emptyID == "" {
		return fmt.Errorf("pagination aggregate initializer lost identity")
	}
	initial, err := attachment(actions[storeIndex].WFWorkflowActionParameters["WFInput"], "ActionOutput")
	if err != nil || initial["OutputUUID"] != emptyID {
		return fmt.Errorf("pagination aggregate does not store its original empty List")
	}
	stores, appends, emptyOwners := 0, 0, 0
	for index, action := range actions[:consumer-1] {
		params := action.WFWorkflowActionParameters
		if params["UUID"] == emptyID {
			emptyOwners++
		}
		if params["WFVariableName"] != name {
			continue
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
			stores++
		}
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" {
			continue
		}
		appends++
		item, err := attachment(params["WFInput"], "ActionOutput")
		if err != nil {
			return err
		}
		itemOwners := 0
		for _, prior := range actions[:index] {
			if prior.WFWorkflowActionParameters["UUID"] != item["OutputUUID"] {
				continue
			}
			itemOwners++
			input, err := attachment(prior.WFWorkflowActionParameters["WFVariable"], "Variable")
			if prior.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || err != nil ||
				input["VariableName"] != "Repeat Item" {
				return fmt.Errorf("pagination aggregate append lost the original native item")
			}
		}
		if itemOwners != 1 {
			return fmt.Errorf("pagination aggregate append has %d prior item owners", itemOwners)
		}
	}
	if stores != 1 || appends != 1 || emptyOwners != 1 {
		return fmt.Errorf("pagination aggregate ownership changed: stores=%d appends=%d initializerOwners=%d", stores, appends, emptyOwners)
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
	for _, fault := range []string{"detached_consumer", "missing_consumer_input", "missing_producer_uuid", "missing_aggregate_input", "foreign_aggregate_input"} {
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
			case "missing_aggregate_input":
				delete(broken[len(broken)-3].WFWorkflowActionParameters, "WFVariable")
			case "foreign_aggregate_input":
				input, okay := broken[len(broken)-3].WFWorkflowActionParameters["WFVariable"].(map[string]any)
				if !okay {
					t.Fatal("actual aggregate read has no native input")
				}
				value, okay := input["Value"].(map[string]any)
				if !okay || value["Type"] != "Variable" {
					t.Fatal("actual aggregate read has no variable attachment")
				}
				value["VariableName"] = "UNRELATED_PAGINATION_AGGREGATE"
			}
			if p15PaginateResultError(broken, false) == nil {
				t.Fatal("detached actual pagination graph accepted", fault)
			}
		})
	}
}
