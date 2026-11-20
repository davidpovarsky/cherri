package main

import (
	"encoding/base64"
	"fmt"
	"howett.net/plist"
	"reflect"
	"testing"
)

func p15NoneActions(t *testing.T, source string) ([]map[string]any, []byte) {
	t.Helper()
	raw, err := CompileSourceToPlist("p15-none-clear.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err = plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	rows, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing canonical actions")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		var valid bool
		actions[i], valid = row.(map[string]any)
		if !valid {
			t.Fatal("invalid native action")
		}
	}
	return actions, raw
}
func p15NoneBindingError(actions []map[string]any) error {
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		return err
	}
	if len(actions) < 2 || actions[len(actions)-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
		return fmt.Errorf("missing final observed value consumer")
	}
	_, err := p15EmptyOrigin(actions, len(actions)-2, p15Param(actions[len(actions)-2])["WFVariable"])
	return err
}
func p15NoneReturnError(actions []map[string]any, tagged ...bool) error {
	marker := -1
	for i, a := range actions {
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p15Param(a)["WFTextActionText"] == "MUST_NOT_LEAK" {
			if marker >= 0 {
				return fmt.Errorf("preceding effect duplicated")
			}
			marker = i
		}
	}
	if marker < 0 || marker+2 >= len(actions) {
		return fmt.Errorf("missing return observation")
	}
	if len(tagged) > 0 && tagged[0] {
		if actions[marker+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			return fmt.Errorf("optional function return lost its tagged None carrier")
		}
		fields, err := p15ContainerFields(p15Param(actions[marker+1])["WFItems"])
		if err != nil || len(fields) != 2 {
			return fmt.Errorf("None carrier has an invented value or malformed fields: %v", err)
		}
		for key, want := range map[string]string{"_cherri_optional_result_v1": "1", "has_value": "0"} {
			row := fields[key]
			if row == nil || row["WFItemType"] != uint64(0) || !reflect.DeepEqual(row["WFValue"], p15ContainerText(want)) {
				return fmt.Errorf("None carrier marker/value-presence changed")
			}
		}
		uuid, _ := p15Param(actions[marker+1])["UUID"].(string)
		if uuid == "" || actions[marker+2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" ||
			!reflect.DeepEqual(p15FunctionComparable(p15Param(actions[marker+2])["WFOutput"], false), p15FunctionText("\ufffc", uuid)) {
			return fmt.Errorf("None carrier was not the explicit stopped result")
		}
		return nil
	}
	if actions[marker+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" || len(p15Param(actions[marker+1])) != 0 {
		return fmt.Errorf("none return does not explicitly clear the preceding output")
	}
	if actions[marker+2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" || len(p15Param(actions[marker+2])) != 0 {
		return fmt.Errorf("none return fabricated an output value or did not stop")
	}
	return nil
}
func TestP15NoneBindingsClearPreviousOutput(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"immutable", "text(\"STALE_BEFORE_NONE\")\nlet value:Text?=none\ngetVariable(value)"},
		{"mutable", "text(\"STALE_BEFORE_NONE\")\nvar value:Text?=none\ngetVariable(value)"},
		{"assignment", "var value:Text?=\"old\"\ntext(\"STALE_BEFORE_NONE\")\nvalue=none\ngetVariable(value)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, raw := p15NoneActions(t, tc.source)
			if err := p15NoneBindingError(actions); err != nil {
				t.Log("P15_NONE_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
				t.Fatal(err)
			}
		})
	}
}
func TestP15NoneReturnClearsPreviousOutput(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"top_level", "text(\"MUST_NOT_LEAK\")\nreturn none"},
		{"function", "function absent()->Text?{text(\"MUST_NOT_LEAK\");return none}\nlet value=absent()\ngetVariable(value)"},
		{"nested_branch", "function absent(flag:Bool)->Text?{if flag{text(\"MUST_NOT_LEAK\");return none};return \"retained\"}\nlet value=absent(true)\ngetVariable(value)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions, raw := p15NoneActions(t, tc.source)
			if err := p15NoneReturnError(actions, tc.name != "top_level"); err != nil {
				t.Log("P15_NONE_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
				t.Fatal(err)
			}
		})
	}
}

func TestP15TaggedNoneResultRejectsStaleAndInventedValues(t *testing.T) {
	source := "function absent()->Text?{text(\"MUST_NOT_LEAK\");return none}\nlet value=absent()\ngetVariable(value)"
	actions, _ := p15NoneActions(t, source)
	if err := p15NoneReturnError(actions, true); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"present_flag", "flag_type", "marker", "invented_value", "stale_output", "continued_instead"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15ContainerCloneNative(t, actions)
			if err := p15NoneReturnError(bad, true); err != nil {
				t.Fatal("unchanged carrier clone must pass", err)
			}
			index := -1
			for i, action := range bad {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p15Param(action)["WFTextActionText"] == "MUST_NOT_LEAK" {
					index = i + 1
				}
			}
			if index < 0 {
				t.Fatal("carrier mutation target absent")
			}
			fields, err := p15ContainerFields(p15Param(bad[index])["WFItems"])
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "present_flag":
				fields["has_value"]["WFValue"] = p15ContainerText("1")
			case "flag_type":
				fields["has_value"]["WFItemType"] = uint64(4)
			case "marker":
				fields["_cherri_optional_result_v1"]["WFValue"] = p15ContainerText("wrong")
			case "invented_value":
				inner := p15Param(bad[index])["WFItems"].(map[string]any)["Value"].(map[string]any)
				inner["WFDictionaryFieldValueItems"] = append(inner["WFDictionaryFieldValueItems"].([]any), map[string]any{"WFKey": p15ContainerText("value"), "WFItemType": uint64(0), "WFValue": p15ContainerText("")})
			case "stale_output":
				p15Param(bad[index+1])["WFOutput"] = p15FunctionText("MUST_NOT_LEAK")
			case "continued_instead":
				bad[index+1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.comment"
			}
			if p15NoneReturnError(bad, true) == nil {
				t.Fatal("corrupted tagged None result accepted")
			}
		})
	}
}
func p15PresentBoundaryError(actions []map[string]any, actionID, key string, want any) error {
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		return err
	}
	// Present values reach their authored consumer before main clears its own unused result.
	for _, a := range actions[:len(actions)-1] {
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
			return fmt.Errorf("present false/zero/empty was cleared")
		}
	}
	if len(actions) < 3 {
		return fmt.Errorf("missing explicit value, consumer and main boundary")
	}
	if actions[len(actions)-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
		return fmt.Errorf("present value lost authored consumer")
	}
	consumer := p15Param(actions[len(actions)-2])["WFVariable"]
	wrapper, ok := consumer.(map[string]any)
	if !ok {
		return fmt.Errorf("missing consumer attachment")
	}
	token, ok := wrapper["Value"].(map[string]any)
	if !ok {
		return fmt.Errorf("missing consumer identity")
	}
	uuid, _ := token["OutputUUID"].(string)
	if token["Type"] == "Variable" {
		name, _ := token["VariableName"].(string)
		found := false
		for i := len(actions) - 3; i >= 0; i-- {
			if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p15Param(actions[i])["WFVariableName"] != name {
				continue
			}
			input, ok := p15Param(actions[i])["WFInput"].(map[string]any)
			if !ok {
				return fmt.Errorf("present assignment has implicit input")
			}
			ref, ok := input["Value"].(map[string]any)
			if !ok || ref["Type"] != "ActionOutput" {
				return fmt.Errorf("present assignment lost producer")
			}
			uuid, _ = ref["OutputUUID"].(string)
			found = true
			break
		}
		if !found {
			return fmt.Errorf("present variable has no assignment")
		}
	} else if token["Type"] != "ActionOutput" {
		return fmt.Errorf("wrong observed reference kind")
	}
	count := 0
	for _, a := range actions {
		p := p15Param(a)
		if uuid != "" && p["UUID"] == uuid {
			count++
			if a["WFWorkflowActionIdentifier"] != actionID || !reflect.DeepEqual(p[key], want) {
				return fmt.Errorf("present value changed: %#v", a)
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("present value has no unique producer")
	}
	return nil
}
func TestP15NoneClearingPreservesPresentEmptyValues(t *testing.T) {
	for _, tc := range []struct {
		name, typ, literal, action, key string
		want                            any
	}{
		{"false", "Bool?", "false", "is.workflow.actions.number", "WFNumberActionNumber", float64(0)},
		{"zero", "Number?", "0", "is.workflow.actions.number", "WFNumberActionNumber", uint64(0)},
		{"empty", "Text?", "\"\"", "is.workflow.actions.gettext", "WFTextActionText", ""},
	} {
		for _, route := range []string{"let", "var", "assign"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				source := "text(\"PRECEDING_CONTENT\")\n" + route + " value:" + tc.typ + "=" + tc.literal + "\ngetVariable(value)"
				if route == "assign" {
					source = "var value:" + tc.typ + "=" + tc.literal + "\ntext(\"PRECEDING_CONTENT\")\nvalue=" + tc.literal + "\ngetVariable(value)"
				}
				actions, _ := p15NoneActions(t, source)
				if err := p15PresentBoundaryError(actions, tc.action, tc.key, tc.want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
func TestP15NoneClearOracleRejectsCorruption(t *testing.T) {
	actions, _ := p15NoneActions(t, "text(\"STALE_BEFORE_NONE\")\nvar value:Text?=none\ngetVariable(value)")
	if err := p15NoneBindingError(actions); err != nil {
		t.Fatal(err)
	}
	set := -1
	for i, a := range actions {
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
			set = i
		}
	}
	if set < 1 {
		t.Fatal("missing clear/set pair")
	}
	for _, fault := range []string{"missing_clear", "empty_text", "implicit_stale", "missing_write", "wrong_consumer"} {
		t.Run("binding/"+fault, func(t *testing.T) {
			bad := p15C01Clone(t, actions)
			switch fault {
			case "missing_clear":
				bad = append(bad[:set-1], bad[set:]...)
			case "empty_text":
				bad[set-1] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": ""}}
			case "implicit_stale":
				p15Param(bad[set])["WFInput"] = "STALE_BEFORE_NONE"
			case "missing_write":
				bad = append(bad[:set], bad[set+1:]...)
			case "wrong_consumer":
				p15Param(bad[len(bad)-2])["WFVariable"] = p15FunctionVariable("unwritten")
			}
			if p15NoneBindingError(bad) == nil {
				t.Fatal("corrupted none boundary accepted")
			}
		})
	}
	returns, _ := p15NoneActions(t, "text(\"MUST_NOT_LEAK\")\nreturn none")
	if err := p15NoneReturnError(returns); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_clear", "empty_text", "invented_output", "continued_instead"} {
		t.Run("return/"+fault, func(t *testing.T) {
			bad := p15C01Clone(t, returns)
			switch fault {
			case "missing_clear":
				bad = append(bad[:1], bad[2:]...)
			case "empty_text":
				bad[1] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": ""}}
			case "invented_output":
				bad[2]["WFWorkflowActionParameters"] = map[string]any{"WFOutput": "MUST_NOT_LEAK"}
			case "continued_instead":
				bad[2] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.comment", "WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "still running"}}
			}
			if p15NoneReturnError(bad) == nil {
				t.Fatal("corrupted none return accepted")
			}
		})
	}
}
