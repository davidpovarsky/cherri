package main

import (
	"fmt"
	"reflect"
	"strings"
)

func p15RP16Map(value any) map[string]any { result, _ := value.(map[string]any); return result }
func p15RP16Array(value any) []any        { result, _ := value.([]any); return result }
func p15RP16String(value any) string      { result, _ := value.(string); return result }
func p15RP16Number(value any, want float64) bool {
	switch number := value.(type) {
	case uint64:
		return float64(number) == want
	case int64:
		return float64(number) == want
	case float64:
		return number == want
	case int:
		return float64(number) == want
	}
	return false
}
func p15RP16TextValue(value any, want string) bool {
	if text, ok := value.(string); ok {
		return text == want
	}
	var token = p15RP16Map(value)
	var content = p15RP16Map(token["Value"])
	return token["WFSerializationType"] == "WFTextTokenString" && content["string"] == want && len(p15RP16Map(content["attachmentsByRange"])) == 0
}
func p15RP16Reference(value any) map[string]any {
	var token = p15RP16Map(value)
	if token["WFSerializationType"] == "WFTextTokenAttachment" {
		return p15RP16Map(token["Value"])
	}
	if token["WFSerializationType"] == "WFTextTokenString" {
		var content = p15RP16Map(token["Value"])
		var ranges = p15RP16Map(content["attachmentsByRange"])
		if content["string"] == "\uFFFC" && len(ranges) == 1 {
			return p15RP16Map(ranges["{0, 1}"])
		}
	}
	return nil
}
func p15RP16OutputReference(value any, uuid string) bool {
	var reference = p15RP16Reference(value)
	return uuid != "" && reference["Type"] == "ActionOutput" && reference["OutputUUID"] == uuid
}
func p15RP16VariableReference(value any, name string) bool {
	var reference = p15RP16Reference(value)
	return reference["Type"] == "Variable" && reference["VariableName"] == name
}

// The inventory witnesses are independent native wire observations, not action
// counts or snapshots copied from a successful run. Generated identities are
// compared through their actual producer/consumer edges.
func p15RP16NativeWitness(fixture p15RP16ApprovedCase, document map[string]any) error {
	var raw = p15RP16Array(document["WFWorkflowActions"])
	if len(raw) == 0 {
		return fmt.Errorf("no native actions")
	}
	var actions = make([]map[string]any, len(raw))
	var params = make([]map[string]any, len(raw))
	var identities = map[string]bool{}
	for index, item := range raw {
		actions[index] = p15RP16Map(item)
		if p15RP16String(actions[index]["WFWorkflowActionIdentifier"]) == "" {
			return fmt.Errorf("invalid native action %d", index)
		}
		params[index] = p15RP16Map(actions[index]["WFWorkflowActionParameters"])
		if uuid, present := params[index]["UUID"]; present {
			var id = p15RP16String(uuid)
			if id == "" || identities[id] {
				return fmt.Errorf("missing or reused producer identity")
			}
			identities[id] = true
		}
	}
	var visitReferences func(any) error
	visitReferences = func(value any) error {
		switch value := value.(type) {
		case map[string]any:
			if value["Type"] == "ActionOutput" && !identities[p15RP16String(value["OutputUUID"])] {
				return fmt.Errorf("dangling native output edge")
			}
			for _, child := range value {
				if err := visitReferences(child); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := visitReferences(child); err != nil {
					return err
				}
			}
		}
		return nil
	}
	if err := visitReferences(raw); err != nil {
		return err
	}
	var groups []string
	var usedGroups = map[string]bool{}
	for index, action := range actions {
		var id = p15RP16String(action["WFWorkflowActionIdentifier"])
		if id != "is.workflow.actions.conditional" && id != "is.workflow.actions.repeat.each" && id != "is.workflow.actions.repeat.count" && id != "is.workflow.actions.choosefrommenu" {
			continue
		}
		var p = params[index]
		var group = p15RP16String(p["GroupingIdentifier"])
		if group == "" {
			return fmt.Errorf("missing native control owner")
		}
		switch {
		case p15RP16Number(p["WFControlFlowMode"], 0):
			if usedGroups[group] {
				return fmt.Errorf("reused native control owner")
			}
			usedGroups[group] = true
			groups = append(groups, group)
		case p15RP16Number(p["WFControlFlowMode"], 1):
			if len(groups) == 0 || groups[len(groups)-1] != group {
				return fmt.Errorf("orphan native middle")
			}
		case p15RP16Number(p["WFControlFlowMode"], 2):
			if len(groups) == 0 || groups[len(groups)-1] != group {
				return fmt.Errorf("orphan native end")
			}
			groups = groups[:len(groups)-1]
		default:
			return fmt.Errorf("invalid native control mode")
		}
	}
	if len(groups) != 0 {
		return fmt.Errorf("unclosed native controls")
	}
	var idAt = func(index int) string {
		if index < 0 || index >= len(actions) {
			return ""
		}
		return p15RP16String(actions[index]["WFWorkflowActionIdentifier"])
	}
	last := actions[len(actions)-1]
	if last["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" || len(params[len(params)-1]) != 0 {
		return fmt.Errorf("approved-node main is missing its explicit Void boundary")
	}
	var sequence = func(shortNames ...string) bool {
		shortNames = append(append([]string(nil), shortNames...), "nothing")
		if len(shortNames) != len(actions) {
			return false
		}
		for index, name := range shortNames {
			if idAt(index) != "is.workflow.actions."+name {
				return false
			}
		}
		return true
	}
	var textAt = func(index int, want string) bool {
		return idAt(index) == "is.workflow.actions.gettext" && p15RP16TextValue(params[index]["WFTextActionText"], want)
	}
	var outputAt = func(consumer int, key string, producer int) bool {
		return p15RP16OutputReference(params[consumer][key], p15RP16String(params[producer]["UUID"]))
	}
	var showOutput = func(consumer, producer int) bool {
		return idAt(consumer) == "is.workflow.actions.showresult" && outputAt(consumer, "Text", producer)
	}
	var showVariable = func(consumer int, name string) bool {
		return idAt(consumer) == "is.workflow.actions.showresult" && p15RP16VariableReference(params[consumer]["Text"], name)
	}
	var listAt = func(index int, expected ...string) bool {
		if idAt(index) != "is.workflow.actions.list" {
			return false
		}
		var items = p15RP16Array(params[index]["WFItems"])
		if len(items) != len(expected) {
			return false
		}
		for i, want := range expected {
			var item = p15RP16Map(items[i])
			if !p15RP16Number(item["WFItemType"], 0) || !p15RP16TextValue(item["WFValue"], want) {
				return false
			}
		}
		return true
	}
	var dictRows = func(index int) []any {
		if idAt(index) != "is.workflow.actions.dictionary" {
			return nil
		}
		var value = p15RP16Map(params[index]["WFItems"])
		if value["WFSerializationType"] != "WFDictionaryFieldValue" {
			return nil
		}
		return p15RP16Array(p15RP16Map(value["Value"])["WFDictionaryFieldValueItems"])
	}
	var dictionaryAt = func(index int, want string) bool {
		var rows = dictRows(index)
		if len(rows) != 1 {
			return false
		}
		var item = p15RP16Map(rows[0])
		return p15RP16Number(item["WFItemType"], 0) && p15RP16TextValue(item["WFKey"], "key") && p15RP16TextValue(item["WFValue"], want)
	}
	var okay bool
	switch fixture.Witness {
	case "text":
		okay = sequence("gettext") && textAt(0, fixture.Value)
	case "metadata":
		var icon = p15RP16Map(document["WFWorkflowIcon"])
		okay = sequence("gettext") && textAt(0, fixture.Value+"_EFFECT") && p15RP16Number(icon["WFWorkflowIconGlyphNumber"], 59789) && p15RP16Number(icon["WFWorkflowIconStartColor"], 4282601983)
	case "trigger":
		var triggers = p15RP16Array(document["WFWorkflowTriggers"])
		if len(triggers) == 1 {
			var trigger = p15RP16Map(triggers[0])
			okay = sequence("gettext") && textAt(0, fixture.Value) && trigger["WFTriggerIdentifier"] == "WFBluetoothTrigger" && p15RP16String(trigger["WFTriggerUUID"]) != "" && reflect.DeepEqual(p15RP16Map(trigger["WFTriggerSerializedParameters"]), map[string]any{"WFBluetoothConnectionType": "connect"})
		}
	case "setup":
		var questions = p15RP16Array(document["WFWorkflowImportQuestions"])
		if len(questions) == 1 {
			var question = p15RP16Map(questions[0])
			okay = sequence("gettext") && textAt(0, fixture.Value) && len(question) == 5 && p15RP16Number(question["ActionIndex"], 0) && question["Category"] == "Parameter" && question["DefaultValue"] == fixture.Value && question["ParameterKey"] == "WFTextActionText" && question["Text"] == "RP16_PROMPT"
		}
	case "reference":
		okay = sequence("gettext", "showresult") && textAt(0, fixture.Value) && showOutput(1, 0)
	case "assignment":
		okay = sequence("number", "setvariable", "number", "setvariable", "showresult") && p15RP16Number(params[0]["WFNumberActionNumber"], 1) && p15RP16Number(params[2]["WFNumberActionNumber"], 2) && params[1]["WFVariableName"] == "value" && params[3]["WFVariableName"] == "value" && outputAt(1, "WFInput", 0) && outputAt(3, "WFInput", 2) && showVariable(4, "value")
	case "fstring":
		if sequence("gettext", "gettext") {
			var token = p15RP16Map(params[1]["WFTextActionText"])
			var value = p15RP16Map(token["Value"])
			var ranges = p15RP16Map(value["attachmentsByRange"])
			var reference = p15RP16Map(ranges["{13, 1}"])
			okay = textAt(0, "RP16_TOKEN") && token["WFSerializationType"] == "WFTextTokenString" && value["string"] == fixture.Value+"\uFFFC" && len(ranges) == 1 && reference["Type"] == "ActionOutput" && reference["OutputUUID"] == params[0]["UUID"]
		}
	case "dictionary", "member":
		if sequence("dictionary", "showresult") && dictionaryAt(0, fixture.Value) && showOutput(1, 0) {
			var ref = p15RP16Reference(params[1]["Text"])
			if fixture.Witness == "dictionary" {
				okay = len(p15RP16Array(ref["Aggrandizements"])) == 0
			} else {
				okay = reflect.DeepEqual(p15RP16Array(ref["Aggrandizements"]), []any{map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "key"}})
			}
		}
	case "list":
		okay = sequence("list", "showresult") && listAt(0, fixture.Value+"_A", fixture.Value+"_B") && showOutput(1, 0)
	case "unary", "binary":
		if sequence("number", "setvariable", "math", "showresult") {
			okay = p15RP16Number(params[0]["WFNumberActionNumber"], 2) && params[1]["WFVariableName"] == "input" && outputAt(1, "WFInput", 0) && showOutput(3, 2)
			if fixture.Witness == "unary" {
				okay = okay && params[2]["WFMathOperation"] == "-" && p15RP16Number(params[2]["WFInput"], 0) && p15RP16VariableReference(params[2]["WFMathOperand"], "input")
			} else {
				okay = okay && params[2]["WFMathOperation"] == "+" && p15RP16VariableReference(params[2]["WFInput"], "input") && p15RP16Number(params[2]["WFMathOperand"], 3)
			}
		}
	case "index":
		if sequence("list", "math", "count", "math", "conditional", "conditional", "getitemfromlist", "conditional", "nothing", "conditional", "conditional", "nothing", "conditional", "showresult") {
			okay = listAt(0, fixture.Value) && params[1]["WFMathOperation"] == "+" && p15RP16Number(params[1]["WFInput"], 0) && p15RP16Number(params[1]["WFMathOperand"], 1) && outputAt(2, "Input", 0) && params[2]["WFCountType"] == "Items" && params[3]["WFMathOperation"] == "-" && outputAt(3, "WFInput", 2) && outputAt(3, "WFMathOperand", 1) && params[6]["WFItemSpecifier"] == "Item At Index" && outputAt(6, "WFInput", 0) && outputAt(6, "WFItemIndex", 1) && showOutput(13, 12)
		}
	case "if_statement", "if_expression":
		var expression = fixture.Witness == "if_expression"
		var names = []string{"number", "conditional", "gettext", "conditional", "gettext", "conditional"}
		if expression {
			names = append(names, "showresult")
		}
		if sequence(names...) {
			okay = p15RP16Number(params[0]["WFNumberActionNumber"], 1) && p15RP16Number(params[1]["WFCondition"], 4) && p15RP16Number(params[1]["WFNumberValue"], 1) && p15RP16OutputReference(p15RP16Map(params[1]["WFInput"])["Variable"], p15RP16String(params[0]["UUID"])) && textAt(2, fixture.Value) && textAt(4, fixture.Value+"_OTHER")
			if expression {
				okay = okay && showOutput(6, 5)
			}
		}
	case "for_statement", "for_expression":
		var expression = fixture.Witness == "for_expression"
		var names = []string{"list", "repeat.each", "setvariable", "showresult", "repeat.each"}
		if expression {
			names = []string{"list", "repeat.each", "setvariable", "getvariable", "repeat.each", "showresult"}
		}
		if sequence(names...) {
			okay = listAt(0, fixture.Value) && outputAt(1, "WFInput", 0) && params[2]["WFVariableName"] == "item" && p15RP16VariableReference(params[2]["WFInput"], "Repeat Item")
			if expression {
				okay = okay && p15RP16VariableReference(params[3]["WFVariable"], "item") && showOutput(5, 4)
			} else {
				okay = okay && showVariable(3, "item")
			}
		}
	case "repeat_statement", "repeat_expression":
		var expression = fixture.Witness == "repeat_expression"
		var names = []string{"repeat.count", "math", "setvariable", "showresult", "repeat.count"}
		if expression {
			names = []string{"repeat.count", "math", "setvariable", "getvariable", "repeat.count", "showresult"}
		}
		if sequence(names...) {
			okay = p15RP16Number(params[0]["WFRepeatCount"], 2) && params[1]["WFMathOperation"] == "-" && p15RP16Number(params[1]["WFMathOperand"], 1) && p15RP16VariableReference(params[1]["WFInput"], "Repeat Index") && params[2]["WFVariableName"] == "index" && outputAt(2, "WFInput", 1)
			if expression {
				okay = okay && p15RP16VariableReference(params[3]["WFVariable"], "index") && showOutput(5, 4)
			} else {
				okay = okay && showVariable(3, "index")
			}
		}
	case "menu_statement", "menu_expression":
		var expression = fixture.Witness == "menu_expression"
		var names = []string{"choosefrommenu", "choosefrommenu", "gettext", "choosefrommenu", "gettext", "choosefrommenu"}
		if expression {
			names = append(names, "showresult")
		}
		if sequence(names...) {
			var items = p15RP16Array(params[0]["WFMenuItems"])
			okay = params[0]["WFMenuPrompt"] == fixture.Value && len(items) == 2 && p15RP16Map(items[0])["WFValue"] == "A" && p15RP16Map(items[1])["WFValue"] == "B" && params[1]["WFMenuItemTitle"] == "A" && params[1]["WFMenuItemAttributedTitle"] == "A" && params[3]["WFMenuItemTitle"] == "B" && params[3]["WFMenuItemAttributedTitle"] == "B" && textAt(2, fixture.Value+"_A") && textAt(4, fixture.Value+"_B")
			if expression {
				okay = okay && showOutput(6, 5)
			}
		}
	case "function":
		var marker, call, payload, returned, shown = -1, -1, -1, false, false
		for index := range actions {
			if textAt(index, fixture.Value) {
				if marker != -1 {
					return fmt.Errorf("duplicate function body effect")
				}
				marker = index
			}
			if idAt(index) == "is.workflow.actions.runworkflow" {
				if call != -1 {
					return fmt.Errorf("duplicate function call")
				}
				call = index
			}
			for _, rawRow := range dictRows(index) {
				var row = p15RP16Map(rawRow)
				if p15RP16TextValue(row["WFKey"], "function") && p15RP16TextValue(row["WFValue"], "target") {
					payload = index
				}
			}
		}
		if marker >= 0 && call >= 0 && payload >= 0 {
			for index := range actions {
				if idAt(index) == "is.workflow.actions.output" && outputAt(index, "WFOutput", marker) {
					returned = true
				}
				if showOutput(index, call) {
					shown = true
				}
			}
			okay = returned && shown && outputAt(call, "WFInput", payload) && p15RP16Map(params[call]["WFWorkflow"])["isSelf"] == true && params[call]["WFWorkflowName"] == "rp16-approved"
		}
	default:
		return fmt.Errorf("unknown native witness %q", fixture.Witness)
	}
	if !okay {
		return fmt.Errorf("native witness failed for %s/%s (%s)", fixture.Role, fixture.Kind, fixture.Witness)
	}
	return nil
}

func p15RP16CorruptWitness(fixture p15RP16ApprovedCase, document map[string]any) bool {
	var actions = p15RP16Array(document["WFWorkflowActions"])
	var parameters = func(index int) map[string]any {
		return p15RP16Map(p15RP16Map(actions[index])["WFWorkflowActionParameters"])
	}
	switch fixture.Witness {
	case "metadata":
		p15RP16Map(document["WFWorkflowIcon"])["WFWorkflowIconGlyphNumber"] = uint64(61440)
		return true
	case "trigger":
		p15RP16Map(p15RP16Map(p15RP16Array(document["WFWorkflowTriggers"])[0])["WFTriggerSerializedParameters"])["WFBluetoothConnectionType"] = "disconnect"
		return true
	case "setup":
		p15RP16Map(p15RP16Array(document["WFWorkflowImportQuestions"])[0])["ParameterKey"] = "RP16_WRONG_KEY"
		return true
	case "assignment":
		parameters(2)["WFNumberActionNumber"] = uint64(3)
		return true
	case "unary", "binary":
		parameters(2)["WFMathOperation"] = "*"
		return true
	case "index":
		parameters(6)["WFItemSpecifier"] = "Last Item"
		return true
	}
	if strings.Contains(fixture.Witness, "_statement") || strings.Contains(fixture.Witness, "_expression") {
		for index, item := range actions {
			var id = p15RP16String(p15RP16Map(item)["WFWorkflowActionIdentifier"])
			if id == "is.workflow.actions.conditional" || id == "is.workflow.actions.repeat.each" || id == "is.workflow.actions.repeat.count" || id == "is.workflow.actions.choosefrommenu" {
				parameters(index)["WFControlFlowMode"] = uint64(1)
				return true
			}
		}
	}
	var changed bool
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if text, ok := child.(string); ok && strings.Contains(text, fixture.Value) {
					value[key] = "RP16_CORRUPTED"
					changed = true
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	walk(document)
	return changed
}
