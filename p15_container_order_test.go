package main

import (
	"fmt"
	"reflect"
	"testing"
)

const p15ContainerWriteExpression = `if true {state="P15_CONTAINER_AFTER";yield "done"} else {yield "unused"}`

func p15ContainerOrderedPayload(actions []map[string]any, function bool) (any, error) {
	if function {
		fields, err := p15FunctionContainerPayload(actions)
		if err != nil {
			return nil, err
		}
		return fields["arg_0"]["WFValue"], nil
	}
	var result any
	count := 0
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			result = p15Param(action)["WFItems"]
			count++
		}
	}
	if count != 1 {
		return nil, fmt.Errorf("expected one semantic Dictionary, found %d", count)
	}
	return result, nil
}

func p15ContainerEarlyReference(payload any, route string) (any, error) {
	fields, err := p15ContainerFields(payload)
	if err != nil {
		return nil, err
	}
	switch route {
	case "map", "function_map":
		return fields["before"]["WFValue"], nil
	case "nested_map":
		nested, err := p15ContainerFields(fields["before"]["WFValue"])
		if err != nil {
			return nil, err
		}
		return nested["value"]["WFValue"], nil
	case "array":
		array, ok := fields["rows"]["WFValue"].(map[string]any)
		if !ok || array["WFSerializationType"] != "WFArrayParameterState" {
			return nil, fmt.Errorf("missing typed array")
		}
		rows, ok := array["Value"].([]any)
		if !ok || len(rows) != 2 {
			return nil, fmt.Errorf("array order/cardinality changed")
		}
		return rows[0].(map[string]any)["WFValue"], nil
	}
	return nil, fmt.Errorf("unrecognized route %s", route)
}

func p15ContainerEntryCaptureError(actions []map[string]any, route string) error {
	payload, err := p15ContainerOrderedPayload(actions, route == "function_map")
	if err != nil {
		return err
	}
	value, err := p15ContainerEarlyReference(payload, route)
	if err != nil {
		return err
	}
	wrapper, ok := value.(map[string]any)
	if !ok || wrapper["WFSerializationType"] != "WFTextTokenAttachment" || len(wrapper) != 2 {
		return fmt.Errorf("early container value is not a typed reference")
	}
	token, ok := wrapper["Value"].(map[string]any)
	if !ok || token["Type"] != "ActionOutput" || len(token) != 2 {
		return fmt.Errorf("early mutable reference was not snapshotted before a later entry's write")
	}
	uuid, ok := token["OutputUUID"].(string)
	if !ok || uuid == "" {
		return fmt.Errorf("snapshot identity missing")
	}
	init, write, capture, firstBranch, effects := -1, -1, -1, -1, 0
	stores := 0
	for index, action := range actions {
		p := p15Param(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p["WFTextActionText"] == "P15_CONTAINER_AFTER" {
			effects++
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "state" {
			stores++
			if init < 0 {
				init = index
			} else {
				write = index
			}
		}
		if init >= 0 && firstBranch < 0 && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
			firstBranch = index
		}
		if p["UUID"] == uuid {
			if capture >= 0 || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" || !reflect.DeepEqual(p["WFVariable"], map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "Variable", "VariableName": "state"}}) {
				return fmt.Errorf("snapshot is not one direct read of the original mutable binding")
			}
			capture = index
		}
	}
	if stores != 2 || effects != 1 || !(init < capture && capture < firstBranch && firstBranch < write) {
		return fmt.Errorf("container entry effects or capture order changed: init=%d capture=%d branch=%d write=%d stores=%d effects=%d", init, capture, firstBranch, write, stores, effects)
	}
	return nil
}

func p15ContainerEntrySource(route string) string {
	prefix := "var state=\"P15_CONTAINER_BEFORE\"\n"
	value := `{"before":state,"write":` + p15ContainerWriteExpression + `}`
	switch route {
	case "nested_map":
		value = `{"before":{"value":state},"write":` + p15ContainerWriteExpression + `}`
	case "array":
		value = `{"rows":[state,` + p15ContainerWriteExpression + `]}`
	case "function_map":
		return "function echo(value:Map<Text,AnyContent>)->Map<Text,AnyContent>{return value}\n" + prefix + "let result=echo(" + value + ")\ngetVariable(result)"
	}
	return prefix + "let data=" + value + "\ngetVariable(data)"
}

func TestP15SemanticContainerEntriesCaptureBeforeLaterWrite(t *testing.T) {
	for _, route := range []string{"map", "function_map", "nested_map", "array"} {
		t.Run(route, func(t *testing.T) {
			if err := p15ContainerEntryCaptureError(p15CompiledActions(t, p15ContainerEntrySource(route)), route); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15SemanticContainerEntryCaptureOracleRejectsCorruption(t *testing.T) {
	original := p15CompiledActions(t, p15ContainerEntrySource("map"))
	if err := p15ContainerEntryCaptureError(original, "map"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"live_reference", "missing_producer", "after_write", "inside_branch", "wrong_variable", "duplicate_producer", "duplicate_effect"} {
		t.Run(fault, func(t *testing.T) {
			actions := p15ContainerCloneNative(t, original)
			payload, err := p15ContainerOrderedPayload(actions, false)
			if err != nil {
				t.Fatal(err)
			}
			value, err := p15ContainerEarlyReference(payload, "map")
			if err != nil {
				t.Fatal(err)
			}
			wrapper := value.(map[string]any)
			uuid := wrapper["Value"].(map[string]any)["OutputUUID"]
			capture := -1
			for index, action := range actions {
				if p15Param(action)["UUID"] == uuid {
					capture = index
				}
			}
			if capture < 0 {
				t.Fatal("positive control lacks capture")
			}
			switch fault {
			case "live_reference":
				wrapper["Value"] = map[string]any{"Type": "Variable", "VariableName": "state"}
			case "missing_producer":
				actions = append(actions[:capture], actions[capture+1:]...)
			case "after_write":
				producer := actions[capture]
				actions = append(actions[:capture], actions[capture+1:]...)
				actions = append(actions, producer)
			case "inside_branch":
				producer := actions[capture]
				actions = append(actions[:capture], actions[capture+1:]...)
				branch := -1
				for index, action := range actions {
					if index >= capture && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
						branch = index
						break
					}
				}
				if branch < 0 {
					t.Fatal("positive control lacks later branch")
				}
				tail := append([]map[string]any(nil), actions[branch+1:]...)
				actions = append(append(actions[:branch+1], producer), tail...)
			case "wrong_variable":
				p15Param(actions[capture])["WFVariable"].(map[string]any)["Value"].(map[string]any)["VariableName"] = "unrelated"
			case "duplicate_producer":
				actions = append(actions, deepClonePlistMap(actions[capture]))
			case "duplicate_effect":
				for _, action := range actions {
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p15Param(action)["WFTextActionText"] == "P15_CONTAINER_AFTER" {
						actions = append(actions, deepClonePlistMap(action))
						break
					}
				}
			}
			if err := p15ContainerEntryCaptureError(actions, "map"); err == nil {
				t.Fatal("container entry oracle accepted " + fault)
			}
		})
	}
}

func TestP15SemanticContainerDoesNotCaptureUnwrittenBinding(t *testing.T) {
	for name, later := range map[string]string{
		"pure":      `state`,
		"unrelated": `if true {other="changed";yield "done"} else {yield "unused"}`,
		"shadowed":  `if true {var state="local";state="changed";yield state} else {yield "unused"}`,
	} {
		t.Run(name, func(t *testing.T) {
			actions := p15CompiledActions(t, "var state=\"before\"\nvar other=\"other\"\nlet data={\"before\":state,\"write\":"+later+"}\ngetVariable(data)")
			payload, err := p15ContainerOrderedPayload(actions, false)
			if err != nil {
				t.Fatal(err)
			}
			value, err := p15ContainerEarlyReference(payload, "map")
			_, expected := p15ContainerFieldValue(p15ContainerReference{"Type": "Variable", "VariableName": "state"})
			if err != nil || !reflect.DeepEqual(value, expected) {
				t.Fatal("unwritten outer binding lost its original native representation", err)
			}
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" && reflect.DeepEqual(p15Param(action)["WFVariable"], expected) {
					t.Fatal("unwritten outer binding gained an unnecessary read action")
				}
			}
		})
	}
}
