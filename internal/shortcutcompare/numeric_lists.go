package shortcutcompare

import (
	"fmt"
	"math"
)

// Numeric list rows must retain Number outputs. In WFContentArrayParameter,
// an inline text-row scalar is not a substitute for a typed Number reference.
// Only the complete, witnessed cleared-variable construction is rewritten.
// A failed witness disables document normalization instead of entering the
// historical text fold, which used to erase numeric producers and their uses.
func foldNumericProceduralLists(document map[string]any, actions []any, origins *parityQuestionOrigins) ([]any, bool) {
	producerCounts := map[string]int{}
	referenceCounts := map[string]int{}
	for _, raw := range actions {
		if action, ok := raw.(map[string]any); ok {
			if params, ok := action["WFWorkflowActionParameters"].(map[string]any); ok {
				if id, ok := params["UUID"].(string); ok && id != "" {
					producerCounts[id]++
				}
			}
		}
	}
	if !nativeNumericListNamesConsistent(actions, producerCounts) {
		return nil, false
	}
	countNumericListReferences(document, referenceCounts)
	originKey := ""
	if origins != nil {
		originKey = origins.key
	}

	var result []any
	for i := 0; i < len(actions); {
		action, _ := actions[i].(map[string]any)
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
			result = append(result, actions[i])
			i++
			continue
		}
		// The older List+set+append shape is intentionally not newly certified.
		// Detect it so its numeric rows cannot reach the old scalar fold.
		priorList := false
		if i > 0 {
			prior, _ := actions[i-1].(map[string]any)
			priorList = prior["WFWorkflowActionIdentifier"] == "is.workflow.actions.list"
		}
		if params["WFInput"] != nil && !priorList {
			result = append(result, actions[i])
			i++
			continue
		}
		end := i + 1
		hasNumber := false
		for end < len(actions) {
			if end+1 >= len(actions) {
				break
			}
			appendAction, _ := actions[end+1].(map[string]any)
			if appendAction["WFWorkflowActionIdentifier"] != "is.workflow.actions.appendvariable" {
				break
			}
			producer, _ := actions[end].(map[string]any)
			if producer["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
				hasNumber = true
			}
			end += 2
		}
		if !hasNumber {
			result = append(result, actions[i])
			i++
			continue
		}
		if priorList || end == i+1 || i == 0 || params["WFInput"] != nil {
			return nil, false
		}
		// A bare Set Variable means "clear" only when fed by a real Nothing.
		clear, _ := actions[i-1].(map[string]any)
		clearParams, clearParamsOkay := clear["WFWorkflowActionParameters"].(map[string]any)
		_, clearParamsPresent := clear["WFWorkflowActionParameters"]
		if clearParamsPresent && !clearParamsOkay {
			return nil, false
		}
		if clear["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" ||
			!numericListKnownKeys(clear, "", "WFWorkflowActionIdentifier", "WFWorkflowActionParameters") ||
			len(clearParams) != 0 {
			return nil, false
		}
		variable, ok := params["WFVariableName"].(string)
		if !ok || variable == "" ||
			!numericListKnownKeys(action, "", "WFWorkflowActionIdentifier", "WFWorkflowActionParameters") ||
			!numericListKnownKeys(params, "", "WFVariableName") {
			return nil, false
		}
		expectedWrites := 1 + (end-i-1)/2
		actualWrites := 0
		for index, raw := range actions {
			act, _ := raw.(map[string]any)
			p, _ := act["WFWorkflowActionParameters"].(map[string]any)
			if p["WFVariableName"] == variable {
				actualWrites++
			}
			if index < end && numericListReadsVariable(raw, variable) {
				return nil, false
			}
		}
		if actualWrites != expectedWrites {
			return nil, false
		}
		for key, value := range document {
			if key != "WFWorkflowActions" && numericListReadsVariable(value, variable) {
				return nil, false
			}
		}
		var numbers, rows []any
		for j := i + 1; j < end; j += 2 {
			producer, _ := actions[j].(map[string]any)
			pp, _ := producer["WFWorkflowActionParameters"].(map[string]any)
			appendAction, _ := actions[j+1].(map[string]any)
			ap, _ := appendAction["WFWorkflowActionParameters"].(map[string]any)
			id, idOK := pp["UUID"].(string)
			if !idOK || id == "" || producerCounts[id] != 1 || referenceCounts[id] != 1 ||
				appendAction["WFWorkflowActionIdentifier"] != "is.workflow.actions.appendvariable" ||
				!numericListKnownKeys(appendAction, "", "WFWorkflowActionIdentifier", "WFWorkflowActionParameters") ||
				!numericListKnownKeys(ap, "", "WFInput", "WFVariableName") || ap["WFVariableName"] != variable ||
				!numericListExactAttachment(ap["WFInput"], id) ||
				!numericListReferenceNameMatches(ap["WFInput"], pp) {
				return nil, false
			}
			var value any
			switch producer["WFWorkflowActionIdentifier"] {
			case "is.workflow.actions.number":
				if !numericListKnownKeys(producer, originKey, "WFWorkflowActionIdentifier", "WFWorkflowActionParameters") ||
					!numericListKnownKeys(pp, "", "UUID", "CustomOutputName", "WFNumberActionNumber") ||
					!numericListNumber(pp["WFNumberActionNumber"]) {
					return nil, false
				}
				numbers = append(numbers, producer)
				value = deepCloneValue(ap["WFInput"])
			case "is.workflow.actions.gettext":
				if !numericListKnownKeys(producer, "", "WFWorkflowActionIdentifier", "WFWorkflowActionParameters") ||
					!numericListKnownKeys(pp, "", "UUID", "CustomOutputName", "WFTextActionText") {
					return nil, false
				}
				text, ok := pp["WFTextActionText"].(string)
				if !ok {
					return nil, false
				}
				value = text
			default:
				return nil, false
			}
			if label, exists := pp["CustomOutputName"]; exists {
				if _, ok := label.(string); !ok {
					return nil, false
				}
			}
			rows = append(rows, map[string]any{"WFItemType": 0, "WFValue": value})
		}
		// Do not reuse an identity already declared or referenced anywhere.
		listID := fmt.Sprintf("PROCEDURAL-NUMERIC-LIST-%d", i)
		for producerCounts[listID] != 0 || referenceCounts[listID] != 0 {
			listID += "_"
		}
		producerCounts[listID] = 1
		list := map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.list",
			"WFWorkflowActionParameters": map[string]any{"UUID": listID, "WFItems": rows},
		}
		set := map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
			"WFWorkflowActionParameters": map[string]any{
				"WFVariableName": variable,
				"WFInput": map[string]any{
					"WFSerializationType": "WFTextTokenAttachment",
					"Value":               map[string]any{"Type": "ActionOutput", "OutputUUID": listID, "OutputName": variable},
				},
			},
		}
		result = append(result, numbers...)
		result = append(result, list, set)
		i = end
	}
	return result, true
}

func numericListKnownKeys(value map[string]any, originKey string, allowed ...string) bool {
	if value == nil {
		return false
	}
	for key := range value {
		if originKey != "" && key == originKey {
			continue
		}
		found := false
		for _, permitted := range allowed {
			if key == permitted {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func numericListExactAttachment(value any, id string) bool {
	wrapper, ok := value.(map[string]any)
	if !ok || !numericListKnownKeys(wrapper, "", "WFSerializationType", "Value") ||
		wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
		return false
	}
	ref, ok := wrapper["Value"].(map[string]any)
	if !ok || !numericListKnownKeys(ref, "", "Type", "OutputUUID", "OutputName") ||
		ref["Type"] != "ActionOutput" || ref["OutputUUID"] != id {
		return false
	}
	if label, exists := ref["OutputName"]; exists {
		if _, ok := label.(string); !ok {
			return false
		}
	}
	return true
}

func numericListNumber(value any) bool {
	switch n := value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		return true
	case float64:
		return !math.IsNaN(n) && !math.IsInf(n, 0)
	case float32:
		return !math.IsNaN(float64(n)) && !math.IsInf(float64(n), 0)
	default:
		return false
	}
}

func countNumericListReferences(value any, counts map[string]int) {
	switch node := value.(type) {
	case map[string]any:
		for key, child := range node {
			if key == "OutputUUID" {
				if id, ok := child.(string); ok {
					counts[id]++
				}
			}
			countNumericListReferences(child, counts)
		}
	case []any:
		for _, child := range node {
			countNumericListReferences(child, counts)
		}
	}
}

func numericListReadsVariable(value any, name string) bool {
	switch node := value.(type) {
	case map[string]any:
		if node["Type"] == "Variable" && node["VariableName"] == name {
			return true
		}
		for _, child := range node {
			if numericListReadsVariable(child, name) {
				return true
			}
		}
	case []any:
		for _, child := range node {
			if numericListReadsVariable(child, name) {
				return true
			}
		}
	}
	return false
}

func numericListReferenceNameMatches(value any, producer map[string]any) bool {
	wrapper, ok := value.(map[string]any)
	if !ok {
		return false
	}
	ref, ok := wrapper["Value"].(map[string]any)
	if !ok {
		return false
	}
	name, okay := producer["CustomOutputName"].(string)
	return okay && name != "" && ref["OutputName"] == name
}

// Check the corresponding native List shape too. Otherwise normalizing the
// procedural side could expose a conflicting native OutputName to the later
// display-name deletion and falsely make the two graphs equal.
func nativeNumericListNamesConsistent(actions []any, counts map[string]int) bool {
	producers := map[string]map[string]any{}
	for _, raw := range actions {
		action, _ := raw.(map[string]any)
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		if id, ok := params["UUID"].(string); ok && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
			producers[id] = params
		}
	}
	for _, raw := range actions {
		action, _ := raw.(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" {
			continue
		}
		params, _ := action["WFWorkflowActionParameters"].(map[string]any)
		items, _ := toSlice(params["WFItems"])
		for _, rawItem := range items {
			item, _ := rawItem.(map[string]any)
			wrapper, _ := item["WFValue"].(map[string]any)
			if wrapper["WFSerializationType"] != "WFTextTokenAttachment" {
				continue
			}
			ref, _ := wrapper["Value"].(map[string]any)
			if ref["Type"] != "ActionOutput" {
				continue
			}
			id, _ := ref["OutputUUID"].(string)
			if producer, okay := producers[id]; okay {
				if counts[id] != 1 || !numericListReferenceNameMatches(wrapper, producer) {
					return false
				}
			}
		}
	}
	return true
}
