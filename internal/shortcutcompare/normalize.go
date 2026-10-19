/*
 * Copyright (c) Cherri Language v2.0
 * Semantic Shortcut Normalizer for Differential Parity Verification
 */

package shortcutcompare

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

type ForwardTarget struct {
	ProducerUUID    string
	VariableName    string
	SystemToken     string
	SourceVariable  string
	Aggrandizements []any
}

func extractSystemToken(input any) string {
	switch typed := input.(type) {
	case map[string]any:
		if val, ok := typed["Value"].(map[string]any); ok {
			return extractSystemToken(val)
		}
		if t, ok := typed["Type"].(string); ok {
			switch t {
			case "ExtensionInput":
				return "ShortcutInput"
			case "Clipboard":
				return "Clipboard"
			case "CurrentDate":
				return "CurrentDate"
			case "DeviceDetails":
				return "DeviceDetails"
			case "Variable":
				if vName, ok := typed["VariableName"].(string); ok {
					switch vName {
					case "ShortcutInput":
						return "ShortcutInput"
					case "Clipboard":
						return "Clipboard"
					case "Device":
						return "DeviceDetails"
					}
				}
			}
		}
	}
	return ""
}

// NormalizeDocumentForParity applies narrow, semantics-preserving normalization rules
// that eliminate known compiler artifacts (such as SSA variable forwarding and redundant
// empty operations) to allow direct semantic comparison of Apple Shortcuts documents.
func NormalizeDocumentForParity(doc map[string]any) map[string]any {
	if doc == nil {
		return nil
	}
	cloned := deepCloneMap(doc)

	actionsRaw, ok := cloned["WFWorkflowActions"].([]any)
	if !ok {
		return cloned
	}

	// 0. Witness and normalize function envelope (Contract B2 vs Legacy)
	actionsRaw = normalizeFunctionEnvelope(actionsRaw)
	cloned["WFWorkflowActions"] = actionsRaw

	// 1. Remove Legacy Cherri top-level WFSerializationType quirk from action parameters
	for _, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				delete(params, "WFSerializationType")
			}
		}
	}

	// 1b. Fold procedural list creation (setvariable + appendvariable chains) into canonical list actions
	actionsRaw = foldProceduralLists(actionsRaw)
	cloned["WFWorkflowActions"] = actionsRaw

	// 2. Discover single-write setvariable actions for SSA Forwarding
	varWriteCounts := make(map[string]int)
	for _, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok {
			actID, _ := actMap["WFWorkflowActionIdentifier"].(string)
			if actID == "is.workflow.actions.setvariable" ||
				actID == "is.workflow.actions.appendvariable" ||
				actID == "is.workflow.actions.addtovariable" {
				if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
					if varName, ok := params["WFVariableName"].(string); ok && varName != "" {
						varWriteCounts[varName]++
					}
				}
			}
		}
	}

	forwardMap := make(map[string]ForwardTarget)
	setVarIndicesToRemove := make(map[int]bool)

	for i, actRaw := range actionsRaw {
		actMap, ok := actRaw.(map[string]any)
		if !ok || actMap["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
			continue
		}
		params, ok := actMap["WFWorkflowActionParameters"].(map[string]any)
		if !ok {
			continue
		}
		varName, _ := params["WFVariableName"].(string)
		if varName == "_" {
			setVarIndicesToRemove[i] = true
			continue
		}
		if varName == "" || varWriteCounts[varName] > 1 {
			continue // Only normalize single-write SSA variables
		}

		producerUUID := extractOutputUUID(params["WFInput"])
		systemToken := extractSystemToken(params["WFInput"])
		varRefName, aggrs := extractVariableRefAndAggrandizements(params["WFInput"])
		if producerUUID != "" {
			forwardMap[varName] = ForwardTarget{
				ProducerUUID:    producerUUID,
				VariableName:    varName,
				Aggrandizements: extractAggrandizements(params["WFInput"]),
			}
			setVarIndicesToRemove[i] = true

			// Rename the producer's CustomOutputName to the variable name
			for _, pActRaw := range actionsRaw {
				if pActMap, ok := pActRaw.(map[string]any); ok {
					if pParams, ok := pActMap["WFWorkflowActionParameters"].(map[string]any); ok {
						if u, _ := pParams["UUID"].(string); u == producerUUID {
							pParams["CustomOutputName"] = varName
							break
						}
					}
				}
			}
		} else if systemToken != "" {
			forwardMap[varName] = ForwardTarget{
				VariableName: varName,
				SystemToken:  systemToken,
			}
			setVarIndicesToRemove[i] = true
		} else if varRefName != "" {
			if prevTarget, ok := forwardMap[varRefName]; ok && prevTarget.ProducerUUID != "" {
				combinedAggrs := append([]any{}, prevTarget.Aggrandizements...)
				combinedAggrs = append(combinedAggrs, aggrs...)
				forwardMap[varName] = ForwardTarget{
					ProducerUUID:    prevTarget.ProducerUUID,
					VariableName:    varName,
					Aggrandizements: combinedAggrs,
				}
				setVarIndicesToRemove[i] = true
			} else {
				forwardMap[varName] = ForwardTarget{
					SourceVariable:  varRefName,
					VariableName:    varName,
					Aggrandizements: aggrs,
				}
				setVarIndicesToRemove[i] = true
			}
		}
	}

	// 3. Rewrite downstream references from Variable -> ActionOutput
	if len(forwardMap) > 0 {
		for _, actRaw := range actionsRaw {
			if actMap, ok := actRaw.(map[string]any); ok {
				if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
					rewriteVariableReferences(params, forwardMap)
				}
			}
		}
	}

	// 4. Identify unused 'is.workflow.actions.nothing' actions and redundant empty list actions
	nothingIndicesToRemove := make(map[int]bool)
	usedUUIDs := collectAllReferencedUUIDs(actionsRaw)
	for i, actRaw := range actionsRaw {
		actMap, ok := actRaw.(map[string]any)
		if !ok {
			continue
		}
		if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
			params, _ := actMap["WFWorkflowActionParameters"].(map[string]any)
			u, _ := params["UUID"].(string)
			if u == "" || !usedUUIDs[u] {
				nothingIndicesToRemove[i] = true
			}
		} else if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" {
			params, _ := actMap["WFWorkflowActionParameters"].(map[string]any)
			items, _ := toSlice(params["WFItems"])
			if len(items) == 0 {
				u, _ := params["UUID"].(string)
				if u != "" {
					refCount := 0
					var setVarParam map[string]any
					for _, otherAct := range actionsRaw {
						if otherMap, ok := otherAct.(map[string]any); ok && otherMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
							if oParams, ok := otherMap["WFWorkflowActionParameters"].(map[string]any); ok {
								if extractOutputUUID(oParams["WFInput"]) == u {
									refCount++
									setVarParam = oParams
								}
							}
						}
					}
					if refCount == 1 && setVarParam != nil {
						delete(setVarParam, "WFInput")
						nothingIndicesToRemove[i] = true
					}
				}
			}
		}
	}

	// 4b. Repair legacy compiler quirk where dictionary was emitted but not linked to immediately following setvariable
	for i, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok && actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
			params, _ := actMap["WFWorkflowActionParameters"].(map[string]any)
			if params != nil && params["WFInput"] == nil {
				for j := i - 1; j >= 0; j-- {
					prevMap, okPrev := actionsRaw[j].(map[string]any)
					if !okPrev {
						break
					}
					prevId, _ := prevMap["WFWorkflowActionIdentifier"].(string)
					if prevId == "is.workflow.actions.nothing" {
						continue
					}
					if prevId == "is.workflow.actions.dictionary" {
						prevParams, _ := prevMap["WFWorkflowActionParameters"].(map[string]any)
						if prevParams != nil {
							u, _ := prevParams["UUID"].(string)
							if u != "" && !usedUUIDs[u] {
								params["WFInput"] = map[string]any{
									"WFSerializationType": "WFTextTokenAttachment",
									"Value": map[string]any{
										"Type":       "ActionOutput",
										"OutputUUID": u,
										"OutputName": "Dictionary",
									},
								}
							}
						}
					}
					break
				}
			}
		}
	}

	// 5. Build filtered action array
	var filteredActions []any
	for i, actRaw := range actionsRaw {
		if setVarIndicesToRemove[i] || nothingIndicesToRemove[i] {
			continue
		}
		filteredActions = append(filteredActions, actRaw)
	}
	cloned["WFWorkflowActions"] = filteredActions

	// 6. Prune unreferenced action UUIDs and normalize compatibility keys
	finalUsedUUIDs := collectAllReferencedUUIDs(filteredActions)
	for _, actRaw := range filteredActions {
		if actMap, ok := actRaw.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				// Prune unreferenced action UUIDs (Apple Shortcuts does not require UUID if output is unused)
				if u, ok := params["UUID"].(string); ok && u != "" && !finalUsedUUIDs[u] {
					delete(params, "UUID")
				}
				// Normalize WFConditionalLegacyComparisonBehavior (emitted by v2 for backward compatibility)
				if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
					delete(params, "WFConditionalLegacyComparisonBehavior")
				}
				// Normalize volatile compile-time UUID in WFWorkflow descriptors (run, runSelf, openShortcut)
				if wf, ok := params["WFWorkflow"].(map[string]any); ok {
					if _, hasId := wf["workflowIdentifier"]; hasId {
						wf["workflowIdentifier"] = "NORMALIZED-WORKFLOW-ID"
					}
					if isSelf, _ := wf["isSelf"].(bool); isSelf {
						wf["workflowName"] = ""
					}
				}
				// Normalize CustomOutputName display label
				delete(params, "CustomOutputName")

				// Normalize calculateexpression with % into math Modulus (Apple Shortcuts interchangeable representation)
				if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.calculateexpression" {
					if inputStr, ok := params["Input"].(string); ok {
						parts := strings.Split(inputStr, " % ")
						if len(parts) == 2 {
							actMap["WFWorkflowActionIdentifier"] = "is.workflow.actions.math"
							params["WFMathOperation"] = "…"
							params["WFScientificMathOperation"] = "Modulus"
							params["WFInput"] = strings.TrimSpace(parts[0])
							params["WFScientificMathOperand"] = strings.TrimSpace(parts[1])
							delete(params, "Input")
						}
					}
				}
			}
		}
	}

	// 7. Normalize triggers: prune empty parameters and normalize volatile compile-time WFTriggerUUID
	if triggersRaw, ok := cloned["WFWorkflowTriggers"].([]any); ok {
		for _, trRaw := range triggersRaw {
			if trMap, ok := trRaw.(map[string]any); ok {
				if p, ok := trMap["WFTriggerSerializedParameters"].(map[string]any); ok && len(p) == 0 {
					delete(trMap, "WFTriggerSerializedParameters")
				}
				if _, hasUUID := trMap["WFTriggerUUID"]; hasUUID {
					trMap["WFTriggerUUID"] = "NORMALIZED-TRIGGER-UUID"
				}
			}
		}
	}

	// 8. Normalize import questions order by ActionIndex
	if questionsRaw, ok := cloned["WFWorkflowImportQuestions"].([]any); ok {
		sort.SliceStable(questionsRaw, func(i, j int) bool {
			qI, okI := questionsRaw[i].(map[string]any)
			qJ, okJ := questionsRaw[j].(map[string]any)
			if okI && okJ {
				idxI, _ := toNumeric(qI["ActionIndex"])
				idxJ, _ := toNumeric(qJ["ActionIndex"])
				return idxI < idxJ
			}
			return false
		})
	}

	// 9. Normalize ActionOutput display names and native system token representations across all parameters
	normalizeActionOutputNames(cloned)

	// 10. Normalize default zero-components in CoreGraphics CGColor representations
	normalizeCGColors(cloned)

	// 11. Normalize empty dictionary representations in scalar slots (nil handling)
	normalizeEmptyMapsInScalars(cloned)

	// 12. Normalize redundant WFDictionaryFieldValue wrapping in nested dictionary field values
	normalizeNestedDictionaryFieldValues(cloned)

	// 13. Normalize equivalent aggrandizements (e.g. rich text coercion vs property)
	normalizeAggrandizements(cloned)

	return cloned
}

func normalizeAggrandizements(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		if t, ok := typed["Type"].(string); ok && t == "WFPropertyVariableAggrandizement" {
			if prop, ok := typed["PropertyName"].(string); ok && strings.EqualFold(prop, "richtext") {
				typed["Type"] = "WFCoercionVariableAggrandizement"
				typed["CoercionItemClass"] = "WFRichTextContentItem"
				delete(typed, "PropertyName")
				delete(typed, "PropertyUserInfo")
			}
		}
		for _, v := range typed {
			normalizeAggrandizements(v)
		}
	case []any:
		for _, item := range typed {
			normalizeAggrandizements(item)
		}
	}
}

func normalizeCGColors(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		if rep, ok := typed["WFColorRepresentationType"].(string); ok && rep == "WFColorRepresentationTypeCGColor" {
			for _, comp := range []string{"redComponent", "greenComponent", "blueComponent"} {
				if val, exists := typed[comp]; exists {
					if n, ok := toNumeric(val); ok && n == 0 {
						delete(typed, comp)
					}
				}
			}
		}
		for _, v := range typed {
			normalizeCGColors(v)
		}
	case []any:
		for _, item := range typed {
			normalizeCGColors(item)
		}
	}
}

func normalizeEmptyMapsInScalars(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		for k, v := range typed {
			if m, ok := v.(map[string]any); ok && len(m) == 0 {
				switch k {
				case "WFNumberValue", "WFRepeatCount", "WFInput":
					delete(typed, k)
				case "WFValue", "WFMenuItemTitle", "WFMenuItemAttributedTitle":
					typed[k] = ""
				}
			} else if s, ok := v.(string); ok && s == "map[]" {
				switch k {
				case "WFValue", "WFMenuItemTitle", "WFMenuItemAttributedTitle":
					typed[k] = ""
				}
			} else {
				normalizeEmptyMapsInScalars(v)
			}
		}
	case []any:
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				if val, exists := m["WFValue"]; exists {
					if vm, ok := val.(map[string]any); ok && len(vm) == 0 {
						m["WFValue"] = ""
					} else if s, ok := val.(string); ok && s == "map[]" {
						m["WFValue"] = ""
					}
				}
			}
			normalizeEmptyMapsInScalars(item)
		}
	}
}

func normalizeActionOutputNames(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		if prop, ok := typed["PropertyName"].(string); ok {
			switch prop {
			case "Model":
				typed["PropertyName"] = "Device Model"
			case "OS":
				typed["PropertyName"] = "System Version"
			}
		}
		if t, ok := typed["Type"].(string); ok {
			if t == "ActionOutput" {
				delete(typed, "OutputName")
			} else if t == "ExtensionInput" {
				delete(typed, "VariableName")
				typed["OutputName"] = "ShortcutInput"
			} else if t == "Variable" {
				if vName, ok := typed["VariableName"].(string); ok {
					switch vName {
					case "Device":
						typed["Type"] = "DeviceDetails"
						delete(typed, "VariableName")
					case "CurrentDate":
						typed["Type"] = "CurrentDate"
						delete(typed, "VariableName")
					case "ShortcutInput":
						typed["Type"] = "ExtensionInput"
						delete(typed, "VariableName")
						typed["OutputName"] = "ShortcutInput"
					}
				}
			}
		}
		for _, v := range typed {
			normalizeActionOutputNames(v)
		}
	case []any:
		for _, item := range typed {
			normalizeActionOutputNames(item)
		}
	}
}

func extractOutputUUID(input any) string {
	switch typed := input.(type) {
	case map[string]any:
		if u, ok := typed["OutputUUID"].(string); ok && u != "" {
			return u
		}
		if val, ok := typed["Value"].(map[string]any); ok {
			if u, ok := val["OutputUUID"].(string); ok && u != "" {
				return u
			}
		}
		if variable, ok := typed["Variable"].(map[string]any); ok {
			if val, ok := variable["Value"].(map[string]any); ok {
				if u, ok := val["OutputUUID"].(string); ok && u != "" {
					return u
				}
			}
		}
	}
	return ""
}

func rewriteVariableReferences(obj any, forwardMap map[string]ForwardTarget) {
	switch typed := obj.(type) {
	case map[string]any:
		// Check if this map is a Variable reference
		isVarRef := false
		varName := ""
		if t, ok := typed["Type"].(string); ok && t == "Variable" {
			isVarRef = true
			if v, ok := typed["VariableName"].(string); ok {
				varName = v
			}
			if val, ok := typed["Value"].(map[string]any); ok {
				if v, ok := val["VariableName"].(string); ok {
					varName = v
				}
			}
		}
		if varName == "" {
			if val, ok := typed["Value"].(map[string]any); ok {
				if t, ok := val["Type"].(string); ok && t == "Variable" {
					if v, ok := val["VariableName"].(string); ok {
						isVarRef = true
						varName = v
					}
				}
			}
		}

		if isVarRef && varName != "" {
			if target, found := forwardMap[varName]; found {
				if target.SystemToken != "" {
					if ser, hasSer := typed["WFSerializationType"].(string); hasSer && ser == "WFTextTokenAttachment" {
						delete(typed, "Type")
						delete(typed, "VariableName")
						val, _ := typed["Value"].(map[string]any)
						if val == nil {
							val = make(map[string]any)
							typed["Value"] = val
						}
						if target.SystemToken == "DeviceDetails" {
							val["Type"] = "DeviceDetails"
							delete(val, "VariableName")
						} else {
							val["Type"] = "Variable"
							val["VariableName"] = target.SystemToken
						}
						delete(val, "OutputUUID")
						delete(val, "OutputName")
					} else {
						if target.SystemToken == "DeviceDetails" {
							typed["Type"] = "DeviceDetails"
							delete(typed, "VariableName")
						} else {
							typed["Type"] = "Variable"
							typed["VariableName"] = target.SystemToken
						}
						delete(typed, "OutputUUID")
						delete(typed, "OutputName")
						delete(typed, "Value")
					}
					return
				}

				if target.ProducerUUID != "" {
					if ser, hasSer := typed["WFSerializationType"].(string); hasSer && ser == "WFTextTokenAttachment" {
						// Standalone WFTextTokenAttachment wrapper
						delete(typed, "Type")
						delete(typed, "VariableName")
						val, _ := typed["Value"].(map[string]any)
						if val == nil {
							val = make(map[string]any)
							typed["Value"] = val
						}
						val["Type"] = "ActionOutput"
						val["OutputUUID"] = target.ProducerUUID
						val["OutputName"] = target.VariableName
						delete(val, "VariableName")
						if len(target.Aggrandizements) > 0 {
							val["Aggrandizements"] = target.Aggrandizements
						}
					} else {
						// Direct token inside attachmentsByRange or parameters
						typed["Type"] = "ActionOutput"
						typed["OutputUUID"] = target.ProducerUUID
						typed["OutputName"] = target.VariableName
						delete(typed, "VariableName")
						delete(typed, "Value")
						if len(target.Aggrandizements) > 0 {
							typed["Aggrandizements"] = target.Aggrandizements
						}
					}
					return
				} else if target.SourceVariable != "" {
					if ser, hasSer := typed["WFSerializationType"].(string); hasSer && ser == "WFTextTokenAttachment" {
						val, _ := typed["Value"].(map[string]any)
						if val == nil {
							val = make(map[string]any)
							typed["Value"] = val
						}
						val["Type"] = "Variable"
						val["VariableName"] = target.SourceVariable
						if len(target.Aggrandizements) > 0 {
							val["Aggrandizements"] = target.Aggrandizements
						}
					} else {
						typed["Type"] = "Variable"
						typed["VariableName"] = target.SourceVariable
						if len(target.Aggrandizements) > 0 {
							typed["Aggrandizements"] = target.Aggrandizements
						}
					}
					return
				}
			}
		}

		for _, v := range typed {
			rewriteVariableReferences(v, forwardMap)
		}
	case []any:
		for _, item := range typed {
			rewriteVariableReferences(item, forwardMap)
		}
	}
}

func collectAllReferencedUUIDs(actions []any) map[string]bool {
	used := make(map[string]bool)
	for _, actRaw := range actions {
		if actMap, ok := actRaw.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				findUUIDsInObject(params, used)
			}
		}
	}
	return used
}

func findUUIDsInObject(obj any, used map[string]bool) {
	switch typed := obj.(type) {
	case map[string]any:
		for k, v := range typed {
			if k == "OutputUUID" {
				if u, ok := v.(string); ok && u != "" {
					used[u] = true
				}
			} else {
				findUUIDsInObject(v, used)
			}
		}
	case []any:
		for _, item := range typed {
			findUUIDsInObject(item, used)
		}
	}
}

func deepCloneMap(m map[string]any) map[string]any {
	res := make(map[string]any, len(m))
	for k, v := range m {
		res[k] = deepCloneValue(v)
	}
	return res
}

func deepCloneValue(val any) any {
	switch typed := val.(type) {
	case map[string]any:
		return deepCloneMap(typed)
	case []any:
		cloned := make([]any, len(typed))
		for i, item := range typed {
			cloned[i] = deepCloneValue(item)
		}
		return cloned
	default:
		return val
	}
}

func extractVariableRefAndAggrandizements(input any) (string, []any) {
	switch typed := input.(type) {
	case map[string]any:
		if val, ok := typed["Value"].(map[string]any); ok {
			return extractVariableRefAndAggrandizements(val)
		}
		if t, ok := typed["Type"].(string); ok && t == "Variable" {
			vName, _ := typed["VariableName"].(string)
			aggrs, _ := typed["Aggrandizements"].([]any)
			return vName, aggrs
		}
	}
	return "", nil
}

func extractAggrandizements(input any) []any {
	switch typed := input.(type) {
	case map[string]any:
		if val, ok := typed["Value"].(map[string]any); ok {
			return extractAggrandizements(val)
		}
		if aggrs, ok := typed["Aggrandizements"].([]any); ok && len(aggrs) > 0 {
			return deepCloneValue(aggrs).([]any)
		}
	}
	return nil
}

func forwardSingleWriteVariables(actionsRaw []any) []any {
	varWriteCounts := make(map[string]int)
	for _, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok {
			actID, _ := actMap["WFWorkflowActionIdentifier"].(string)
			if actID == "is.workflow.actions.setvariable" ||
				actID == "is.workflow.actions.appendvariable" ||
				actID == "is.workflow.actions.addtovariable" {
				if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
					if varName, ok := params["WFVariableName"].(string); ok && varName != "" {
						varWriteCounts[varName]++
					}
				}
			}
		}
	}

	forwardMap := make(map[string]ForwardTarget)
	setVarIndicesToRemove := make(map[int]bool)

	for i, actRaw := range actionsRaw {
		actMap, ok := actRaw.(map[string]any)
		if !ok || actMap["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" {
			continue
		}
		params, ok := actMap["WFWorkflowActionParameters"].(map[string]any)
		if !ok {
			continue
		}
		varName, _ := params["WFVariableName"].(string)
		if varName == "_" {
			setVarIndicesToRemove[i] = true
			continue
		}
		if varName == "" || varWriteCounts[varName] > 1 {
			continue
		}

		producerUUID := extractOutputUUID(params["WFInput"])
		systemToken := extractSystemToken(params["WFInput"])
		varRefName, aggrs := extractVariableRefAndAggrandizements(params["WFInput"])
		if producerUUID != "" {
			forwardMap[varName] = ForwardTarget{
				ProducerUUID:    producerUUID,
				VariableName:    varName,
				Aggrandizements: extractAggrandizements(params["WFInput"]),
			}
			setVarIndicesToRemove[i] = true

			for _, pActRaw := range actionsRaw {
				if pActMap, ok := pActRaw.(map[string]any); ok {
					if pParams, ok := pActMap["WFWorkflowActionParameters"].(map[string]any); ok {
						if u, _ := pParams["UUID"].(string); u == producerUUID {
							pParams["CustomOutputName"] = varName
							break
						}
					}
				}
			}
		} else if systemToken != "" {
			forwardMap[varName] = ForwardTarget{
				VariableName: varName,
				SystemToken:  systemToken,
			}
			setVarIndicesToRemove[i] = true
		} else if varRefName != "" {
			if prevTarget, ok := forwardMap[varRefName]; ok && prevTarget.ProducerUUID != "" {
				combinedAggrs := append([]any{}, prevTarget.Aggrandizements...)
				combinedAggrs = append(combinedAggrs, aggrs...)
				forwardMap[varName] = ForwardTarget{
					ProducerUUID:    prevTarget.ProducerUUID,
					VariableName:    varName,
					Aggrandizements: combinedAggrs,
				}
				setVarIndicesToRemove[i] = true
			} else {
				forwardMap[varName] = ForwardTarget{
					SourceVariable:  varRefName,
					VariableName:    varName,
					Aggrandizements: aggrs,
				}
				setVarIndicesToRemove[i] = true
			}
		}
	}

	if len(forwardMap) > 0 {
		for _, actRaw := range actionsRaw {
			if actMap, ok := actRaw.(map[string]any); ok {
				if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
					rewriteVariableReferences(params, forwardMap)
				}
			}
		}
	}

	if len(setVarIndicesToRemove) > 0 {
		var filtered []any
		for i, actRaw := range actionsRaw {
			if !setVarIndicesToRemove[i] {
				filtered = append(filtered, actRaw)
			}
		}
		return filtered
	}
	return actionsRaw
}

func foldProceduralLists(actions []any) []any {
	var newActions []any
	i := 0
	for i < len(actions) {
		act, _ := actions[i].(map[string]any)
		if act != nil && act["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
			params, _ := act["WFWorkflowActionParameters"].(map[string]any)
			vName, _ := params["WFVariableName"].(string)
			if vName != "" && params["WFInput"] == nil {
				// Check for subsequent (producer + appendvariable) pairs
				var items []any
				j := i + 1
				for j+1 < len(actions) {
					pAct, _ := actions[j].(map[string]any)
					aAct, _ := actions[j+1].(map[string]any)
					if pAct == nil || aAct == nil {
						break
					}
					if aAct["WFWorkflowActionIdentifier"] != "is.workflow.actions.appendvariable" {
						break
					}
					aParams, _ := aAct["WFWorkflowActionParameters"].(map[string]any)
					if aParams["WFVariableName"] != vName {
						break
					}
					pParams, _ := pAct["WFWorkflowActionParameters"].(map[string]any)
					pUUID, _ := pParams["UUID"].(string)
					// Check if aAct WFInput references pUUID
					inVal, _ := aParams["WFInput"].(map[string]any)
					valObj, _ := inVal["Value"].(map[string]any)
					if valObj["OutputUUID"] != pUUID && pUUID != "" {
						break
					}
					// Extract literal item value from pAct
					if pAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
						textVal := pParams["WFTextActionText"]
						items = append(items, map[string]any{
							"WFItemType": 0,
							"WFValue":    textVal,
						})
					} else if pAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
						numVal := pParams["WFNumberActionNumber"]
						items = append(items, map[string]any{
							"WFItemType": 0,
							"WFValue":    numVal,
						})
					} else {
						break
					}
					j += 2
				}
				if len(items) > 0 {
					listUUID := "PROCEDURAL-LIST-" + vName
					listNode := map[string]any{
						"WFWorkflowActionIdentifier": "is.workflow.actions.list",
						"WFWorkflowActionParameters": map[string]any{
							"UUID":    listUUID,
							"WFItems": items,
						},
					}
					setVarNode := map[string]any{
						"WFWorkflowActionIdentifier": "is.workflow.actions.setvariable",
						"WFWorkflowActionParameters": map[string]any{
							"WFVariableName": vName,
							"WFInput": map[string]any{
								"WFSerializationType": "WFTextTokenAttachment",
								"Value": map[string]any{
									"Type":       "ActionOutput",
									"OutputUUID": listUUID,
									"OutputName": vName,
								},
							},
						},
					}
					newActions = append(newActions, listNode, setVarNode)
					i = j
					continue
				}
			}
		} else if act != nil && act["WFWorkflowActionIdentifier"] == "is.workflow.actions.list" && i+1 < len(actions) {
			listParams, _ := act["WFWorkflowActionParameters"].(map[string]any)
			listUUID, _ := listParams["UUID"].(string)
			setAct, _ := actions[i+1].(map[string]any)
			if setAct != nil && setAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
				setParams, _ := setAct["WFWorkflowActionParameters"].(map[string]any)
				vName, _ := setParams["WFVariableName"].(string)
				inVal, _ := setParams["WFInput"].(map[string]any)
				valObj, _ := inVal["Value"].(map[string]any)
				if (valObj["OutputUUID"] == listUUID && listUUID != "") || inVal == nil {
					items, _ := toSlice(listParams["WFItems"])
					origLen := len(items)
					j := i + 2
					for j+1 < len(actions) {
						pAct, _ := actions[j].(map[string]any)
						aAct, _ := actions[j+1].(map[string]any)
						if pAct == nil || aAct == nil {
							break
						}
						if aAct["WFWorkflowActionIdentifier"] != "is.workflow.actions.appendvariable" {
							break
						}
						aParams, _ := aAct["WFWorkflowActionParameters"].(map[string]any)
						if aParams["WFVariableName"] != vName {
							break
						}
						pParams, _ := pAct["WFWorkflowActionParameters"].(map[string]any)
						pUUID, _ := pParams["UUID"].(string)
						ainVal, _ := aParams["WFInput"].(map[string]any)
						avalObj, _ := ainVal["Value"].(map[string]any)
						if avalObj["OutputUUID"] != pUUID && pUUID != "" {
							break
						}
						if pAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
							textVal := pParams["WFTextActionText"]
							items = append(items, map[string]any{
								"WFItemType": 0,
								"WFValue":    textVal,
							})
						} else if pAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
							numVal := pParams["WFNumberActionNumber"]
							items = append(items, map[string]any{
								"WFItemType": 0,
								"WFValue":    numVal,
							})
						} else {
							break
						}
						j += 2
					}
					if len(items) > origLen {
						listParams["WFItems"] = items
						newActions = append(newActions, act, setAct)
						i = j
						continue
					}
				}
			}
		}
		newActions = append(newActions, actions[i])
		i++
	}
	return newActions
}

func normalizeNestedDictionaryFieldValues(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		if val, ok := typed["Value"].(map[string]any); ok {
			if ser, okSer := val["WFSerializationType"].(string); okSer && ser == "WFDictionaryFieldValue" {
				if innerVal, okInner := val["Value"].(map[string]any); okInner {
					if _, hasItems := innerVal["WFDictionaryFieldValueItems"]; hasItems {
						typed["Value"] = innerVal
					}
				}
			}
		}
		for _, v := range typed {
			normalizeNestedDictionaryFieldValues(v)
		}
	case []any:
		for _, item := range typed {
			normalizeNestedDictionaryFieldValues(item)
		}
	}
}

func getVariableNameFromInput(inp any) string {
	m, ok := inp.(map[string]any)
	if !ok {
		return ""
	}
	if v, ok := m["VariableName"].(string); ok && v != "" {
		return v
	}
	if vMap, ok := m["Variable"].(map[string]any); ok {
		if v, ok := vMap["VariableName"].(string); ok && v != "" {
			return v
		}
		if valMap, ok := vMap["Value"].(map[string]any); ok {
			if v, ok := valMap["VariableName"].(string); ok && v != "" {
				return v
			}
		}
	}
	return ""
}

func unwrapSingleTokenAttachment(val any) any {
	m, ok := val.(map[string]any)
	if !ok {
		return val
	}
	if m["WFSerializationType"] == "WFTextTokenString" {
		if valMap, ok := m["Value"].(map[string]any); ok {
			if str, ok := valMap["string"].(string); ok && str == "\ufffc" {
				if attrs, ok := valMap["attachmentsByRange"].(map[string]any); ok && len(attrs) == 1 {
					for _, tok := range attrs {
						return map[string]any{
							"WFSerializationType": "WFTextTokenAttachment",
							"Value":               tok,
						}
					}
				}
			}
		}
	}
	return val
}

func normalizeCallArgumentValue(valWrapper any) any {
	m, ok := valWrapper.(map[string]any)
	if !ok {
		return valWrapper
	}
	v := m["WFValue"]
	curr := v
	for {
		cMap, ok := curr.(map[string]any)
		if !ok {
			break
		}
		vInner, okInner := cMap["Value"].(map[string]any)
		if !okInner {
			break
		}
		curr = vInner
	}
	if cMap, ok := curr.(map[string]any); ok {
		if items, ok := toSlice(cMap["WFDictionaryFieldValueItems"]); ok && len(items) == 1 {
			if itMap, ok := items[0].(map[string]any); ok {
				if keyMap, ok := itMap["WFKey"].(map[string]any); ok {
					if keyVal, ok := keyMap["Value"].(map[string]any); ok {
						if keyStr, ok := keyVal["string"].(string); ok && keyStr == "array" {
							m["WFValue"] = itMap["WFValue"]
							m["WFItemType"] = 0
						}
					}
				}
			}
		}
	}
	m["WFValue"] = unwrapSingleTokenAttachment(m["WFValue"])
	m["WFItemType"] = 0
	return m
}

func normalizeRunWorkflowCallDict(params map[string]any) bool {
	wfItems, ok := params["WFItems"].(map[string]any)
	if !ok {
		return false
	}
	val, ok := wfItems["Value"].(map[string]any)
	if !ok {
		return false
	}
	rawItems, ok := toSlice(val["WFDictionaryFieldValueItems"])
	if !ok || len(rawItems) == 0 {
		return false
	}

	isFnDict := false
	funcName := ""
	for _, itRaw := range rawItems {
		it, ok := itRaw.(map[string]any)
		if !ok {
			continue
		}
		kMap, _ := it["WFKey"].(map[string]any)
		kVal, _ := kMap["Value"].(map[string]any)
		keyStr, _ := kVal["string"].(string)

		vMap, _ := it["WFValue"].(map[string]any)
		vVal, _ := vMap["Value"].(map[string]any)
		vStr, _ := vVal["string"].(string)

		if keyStr == "cherri_functions" || (keyStr == "_cherri_magic" && vStr == "__cherri_fn_envelope_v2__") {
			isFnDict = true
		}
		if keyStr == "function" {
			if vStr != "" {
				funcName = vStr
			} else if s, ok := it["WFValue"].(string); ok {
				funcName = s
			}
		}
	}

	if !isFnDict {
		return false
	}

	var args []any
	v2Args := make(map[int]any)
	for _, itRaw := range rawItems {
		it, ok := itRaw.(map[string]any)
		if !ok {
			continue
		}
		kMap, _ := it["WFKey"].(map[string]any)
		kVal, _ := kMap["Value"].(map[string]any)
		keyStr, _ := kVal["string"].(string)
		v := it["WFValue"]

		if keyStr == "arguments" {
			if vMap, ok := v.(map[string]any); ok {
				if arr, ok := toSlice(vMap["Value"]); ok {
					for _, a := range arr {
						args = append(args, normalizeCallArgumentValue(deepCloneValue(a)))
					}
					break
				}
			}
		} else if strings.HasPrefix(keyStr, "arg_") {
			idx, err := strconv.Atoi(strings.TrimPrefix(keyStr, "arg_"))
			if err == nil {
				v2Args[idx] = v
			}
		}
	}

	if len(v2Args) > 0 {
		var keys []int
		for k := range v2Args {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		args = nil
		for _, k := range keys {
			rawVal := unwrapSingleTokenAttachment(v2Args[k])
			args = append(args, map[string]any{
				"WFItemType": 0,
				"WFValue":    rawVal,
			})
		}
	}

	canonicalItems := []any{
		map[string]any{
			"WFItemType": 0,
			"WFKey": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value":               map[string]any{"string": "function"},
			},
			"WFValue": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value":               map[string]any{"string": funcName},
			},
		},
		map[string]any{
			"WFItemType": 2,
			"WFKey": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value":               map[string]any{"string": "arguments"},
			},
			"WFValue": map[string]any{
				"WFSerializationType": "WFArrayParameterState",
				"Value":               args,
			},
		},
	}
	val["WFDictionaryFieldValueItems"] = canonicalItems
	return true
}

func forwardIdentityGettext(actions []any) []any {
	uuidMap := make(map[string]string)
	indicesToRemove := make(map[int]bool)

	for i, aRaw := range actions {
		act, ok := aRaw.(map[string]any)
		if !ok || act["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			continue
		}
		u, _ := p["UUID"].(string)
		textMap, _ := p["WFTextActionText"].(map[string]any)
		if textMap != nil && textMap["WFSerializationType"] == "WFTextTokenString" {
			if valMap, ok := textMap["Value"].(map[string]any); ok {
				if str, ok := valMap["string"].(string); ok && str == "\ufffc" {
					if attrs, ok := valMap["attachmentsByRange"].(map[string]any); ok && len(attrs) == 1 {
						for _, tokRaw := range attrs {
							if tok, ok := tokRaw.(map[string]any); ok {
								if srcU, ok := tok["OutputUUID"].(string); ok && srcU != "" && u != "" {
									uuidMap[u] = srcU
									indicesToRemove[i] = true
								}
							}
						}
					}
				}
			}
		}
	}

	if len(uuidMap) == 0 {
		return actions
	}

	var res []any
	for i, aRaw := range actions {
		if !indicesToRemove[i] {
			res = append(res, aRaw)
		}
	}
	rewriteActionUUIDReferences(res, uuidMap)
	return res
}

func rewriteActionUUIDReferences(obj any, uuidMap map[string]string) {
	switch typed := obj.(type) {
	case map[string]any:
		if u, ok := typed["OutputUUID"].(string); ok {
			if target, found := uuidMap[u]; found {
				typed["OutputUUID"] = target
			}
		}
		for _, v := range typed {
			rewriteActionUUIDReferences(v, uuidMap)
		}
	case []any:
		for _, item := range typed {
			rewriteActionUUIDReferences(item, uuidMap)
		}
	}
}

type CanonicalFunction struct {
	Name       string
	Parameters []CanonicalParam
	Body       []any
	VarMap     map[string]string
	UUIDMap    map[string]string
}

type CanonicalParam struct {
	Name           string `json:"Name"`
	Index          int    `json:"Index"`
	DefaultActions []any  `json:"DefaultActions,omitempty"`
}

func foldGettextIntoOutput(actions []any) []any {
	var res []any
	for i := 0; i < len(actions); i++ {
		act, _ := actions[i].(map[string]any)
		if act != nil && act["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && i+1 < len(actions) {
			nextAct, _ := actions[i+1].(map[string]any)
			if nextAct != nil && nextAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.output" {
				p, _ := act["WFWorkflowActionParameters"].(map[string]any)
				nextP, _ := nextAct["WFWorkflowActionParameters"].(map[string]any)
				if nextP == nil {
					nextP = make(map[string]any)
					nextAct["WFWorkflowActionParameters"] = nextP
				}
				if textVal, ok := p["WFTextActionText"]; ok {
					nextP["WFOutput"] = deepCloneValue(textVal)
					res = append(res, nextAct)
					i++
					continue
				}
			}
		}
		res = append(res, actions[i])
	}
	return res
}

func unwrapMathSingleTokens(actions []any) {
	for _, aRaw := range actions {
		act, _ := aRaw.(map[string]any)
		if act != nil && act["WFWorkflowActionIdentifier"] == "is.workflow.actions.math" {
			p, _ := act["WFWorkflowActionParameters"].(map[string]any)
			if p != nil {
				if inp, ok := p["WFInput"]; ok {
					p["WFInput"] = unwrapSingleTokenAttachment(inp)
				}
				if op, ok := p["WFMathOperand"]; ok {
					p["WFMathOperand"] = unwrapSingleTokenAttachment(op)
				}
			}
		}
	}
}

func parseFunctionsLegacy(actions []any) (map[string]*CanonicalFunction, []any) {
	funcs := make(map[string]*CanonicalFunction)
	var mainActions []any

	dispatcherEnd := -1
	dispatcherGid := ""

	for i := 0; i < len(actions); i++ {
		act, _ := actions[i].(map[string]any)
		if act == nil || act["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			continue
		}
		modeStr := fmt.Sprintf("%v", p["WFControlFlowMode"])
		gid, _ := p["GroupingIdentifier"].(string)

		if modeStr == "0" && dispatcherGid == "" {
			// First conditional in legacy is the dispatcher wrapper (if ShortcutInput)
			dispatcherGid = gid
		} else if modeStr == "2" && gid == dispatcherGid && dispatcherGid != "" {
			dispatcherEnd = i
			break
		}

		if modeStr != "0" || fmt.Sprintf("%v", p["WFCondition"]) != "4" {
			continue
		}
		fnName, _ := p["WFConditionalActionString"].(string)
		if fnName == "" || strings.HasPrefix(fnName, "_") || fnName == "1" {
			continue
		}

		endIdx := -1
		for j := i + 1; j < len(actions); j++ {
			a2, _ := actions[j].(map[string]any)
			if a2 != nil && a2["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				p2, _ := a2["WFWorkflowActionParameters"].(map[string]any)
				if p2 != nil && p2["GroupingIdentifier"] == gid && fmt.Sprintf("%v", p2["WFControlFlowMode"]) == "2" {
					endIdx = j
					break
				}
			}
		}
		if endIdx == -1 {
			continue
		}

		fnActs := actions[i+1 : endIdx]
		fn := &CanonicalFunction{
			Name:    fnName,
			VarMap:  make(map[string]string),
			UUIDMap: make(map[string]string),
		}

		var getItemIndices []int
		for k, aRaw := range fnActs {
			kA, _ := aRaw.(map[string]any)
			if kA != nil && kA["WFWorkflowActionIdentifier"] == "is.workflow.actions.getitemfromlist" {
				getItemIndices = append(getItemIndices, k)
			}
		}

		lastParamEnd := 0
		for pi, gIdx := range getItemIndices {
			gAct, _ := fnActs[gIdx].(map[string]any)
			gParams, _ := gAct["WFWorkflowActionParameters"].(map[string]any)
			outName, _ := gParams["CustomOutputName"].(string)

			rawParamName := ""
			prefix := fmt.Sprintf("_cherri_%s_arg_%d_", fnName, pi+1)
			if strings.HasPrefix(outName, prefix) {
				rawParamName = strings.TrimPrefix(outName, prefix)
			} else {
				prefixLegacy := fmt.Sprintf("_cherri_%s_arg_", fnName)
				if strings.HasPrefix(outName, prefixLegacy) {
					rest := strings.TrimPrefix(outName, prefixLegacy)
					parts := strings.SplitN(rest, "_", 2)
					if len(parts) == 2 {
						rawParamName = parts[1]
					}
				}
			}
			cleanParamName := strings.TrimPrefix(rawParamName, "_")

			nextGIdx := len(fnActs)
			if pi+1 < len(getItemIndices) {
				nextGIdx = getItemIndices[pi+1]
			}

			var defActions []any
			m := gIdx + 1
			for m < nextGIdx {
				mA, _ := fnActs[m].(map[string]any)
				if mA == nil {
					m++
					continue
				}
				mId, _ := mA["WFWorkflowActionIdentifier"].(string)
				mP, _ := mA["WFWorkflowActionParameters"].(map[string]any)
				if mId == "is.workflow.actions.setvariable" {
					v, _ := mP["WFVariableName"].(string)
					if v == rawParamName || v == cleanParamName || rawParamName == "" {
						if rawParamName == "" {
							rawParamName = v
							cleanParamName = strings.TrimPrefix(rawParamName, "_")
						}
						m++
						lastParamEnd = m
						// Check if followed by default value conditional (condition 101)
						if m < nextGIdx {
							cAct, _ := fnActs[m].(map[string]any)
							if cAct != nil && cAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
								cP, _ := cAct["WFWorkflowActionParameters"].(map[string]any)
								if cP != nil && fmt.Sprintf("%v", cP["WFControlFlowMode"]) == "0" && fmt.Sprintf("%v", cP["WFCondition"]) == "101" {
									cGid, _ := cP["GroupingIdentifier"].(string)
									cStart := m + 1
									cEnd := -1
									for d := cStart; d < nextGIdx; d++ {
										dA, _ := fnActs[d].(map[string]any)
										if dA != nil && dA["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
											dP, _ := dA["WFWorkflowActionParameters"].(map[string]any)
											if dP != nil && dP["GroupingIdentifier"] == cGid && fmt.Sprintf("%v", dP["WFControlFlowMode"]) == "2" {
												cEnd = d
												break
											}
										}
									}
									if cEnd != -1 {
										for d := cStart; d < cEnd; d++ {
											dA, _ := fnActs[d].(map[string]any)
											if dA != nil {
												dId, _ := dA["WFWorkflowActionIdentifier"].(string)
												if dId != "is.workflow.actions.setvariable" && dId != "is.workflow.actions.nothing" {
													defActions = append(defActions, deepCloneValue(dA))
												}
											}
										}
										m = cEnd + 1
										lastParamEnd = m
									}
								}
							}
						}
						continue
					}
					if strings.Contains(v, "_array_item") || strings.HasPrefix(v, "_cherri_") {
						m++
						continue
					}
					// A setvariable for another variable means the body has begun
					break
				} else if mId == "is.workflow.actions.repeat.each" && fmt.Sprintf("%v", mP["WFControlFlowMode"]) == "2" {
					m++
					lastParamEnd = m
					continue
				} else if mId == "is.workflow.actions.gettext" ||
					mId == "is.workflow.actions.number" ||
					mId == "is.workflow.actions.dictionary" ||
					mId == "is.workflow.actions.list" ||
					mId == "is.workflow.actions.detect.dictionary" ||
					mId == "is.workflow.actions.detect.number" ||
					mId == "is.workflow.actions.getvalueforkey" ||
					mId == "is.workflow.actions.nothing" ||
					(mId == "is.workflow.actions.repeat.each" && fmt.Sprintf("%v", mP["WFControlFlowMode"]) == "0") ||
					mId == "is.workflow.actions.appendvariable" {
					m++
					continue
				} else {
					break
				}
				m++
			}

			paramUUID := fmt.Sprintf("PARAM-%s-%s", fnName, cleanParamName)
			fn.Parameters = append(fn.Parameters, CanonicalParam{
				Name:           cleanParamName,
				Index:          pi,
				DefaultActions: defActions,
			})
			if rawParamName != "" {
				fn.VarMap[rawParamName] = paramUUID
			}
			if cleanParamName != "" {
				fn.VarMap[cleanParamName] = paramUUID
			}
		}

		for lastParamEnd < len(fnActs) {
			bAct, _ := fnActs[lastParamEnd].(map[string]any)
			if bAct != nil && bAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
				lastParamEnd++
			} else {
				break
			}
		}

		var rawBody []any
		for b := lastParamEnd; b < len(fnActs); b++ {
			bAct, _ := fnActs[b].(map[string]any)
			if bAct != nil && bAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" && b == len(fnActs)-1 {
				continue
			}
			rawBody = append(rawBody, deepCloneValue(fnActs[b]))
		}
		fn.Body = rawBody
		funcs[fnName] = fn
	}

	if dispatcherEnd >= 0 && dispatcherEnd+1 < len(actions) {
		mainActions = append([]any{}, actions[dispatcherEnd+1:]...)
	}

	for len(mainActions) > 0 {
		firstAct, _ := mainActions[0].(map[string]any)
		if firstAct != nil && firstAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" {
			mainActions = mainActions[1:]
		} else {
			break
		}
	}

	return funcs, mainActions
}

func parseFunctionsV2(actions []any) (map[string]*CanonicalFunction, []any) {
	funcs := make(map[string]*CanonicalFunction)
	var mainActions []any

	mainCondIdx := -1
	mainGroup := ""
	for i, aRaw := range actions {
		act, ok := aRaw.(map[string]any)
		if !ok || act["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p != nil && fmt.Sprintf("%v", p["WFControlFlowMode"]) == "0" {
			v := getVariableNameFromInput(p["WFInput"])
			if v == "_cherri_is_fn" {
				mainCondIdx = i
				mainGroup, _ = p["GroupingIdentifier"].(string)
				break
			}
		}
	}

	otherwiseIdx := -1
	endIdx := -1
	if mainCondIdx >= 0 {
		for i := mainCondIdx + 1; i < len(actions); i++ {
			act, ok := actions[i].(map[string]any)
			if !ok || act["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
				continue
			}
			p, _ := act["WFWorkflowActionParameters"].(map[string]any)
			if p == nil {
				continue
			}
			g, _ := p["GroupingIdentifier"].(string)
			modeStr := fmt.Sprintf("%v", p["WFControlFlowMode"])
			if g == mainGroup {
				if modeStr == "1" {
					otherwiseIdx = i
				} else if modeStr == "2" {
					endIdx = i
					break
				}
			}
		}
	}

	if otherwiseIdx != -1 && endIdx != -1 {
		mainActions = append([]any{}, actions[otherwiseIdx+1:endIdx]...)
	} else if mainCondIdx >= 0 {
		mainActions = append([]any{}, actions[mainCondIdx:]...)
	}

	var fnBlock []any
	if mainCondIdx >= 0 && otherwiseIdx > mainCondIdx {
		fnBlock = actions[mainCondIdx+1 : otherwiseIdx]
	}

	for i := 0; i < len(fnBlock); i++ {
		act, _ := fnBlock[i].(map[string]any)
		if act == nil || act["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			continue
		}
		if fmt.Sprintf("%v", p["WFControlFlowMode"]) != "0" || fmt.Sprintf("%v", p["WFCondition"]) != "4" {
			continue
		}
		fnName, _ := p["WFConditionalActionString"].(string)
		if fnName == "" || strings.HasPrefix(fnName, "_") || fnName == "1" {
			continue
		}
		gid, _ := p["GroupingIdentifier"].(string)

		fnEndIdx := -1
		for j := i + 1; j < len(fnBlock); j++ {
			a2, _ := fnBlock[j].(map[string]any)
			if a2 != nil && a2["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				p2, _ := a2["WFWorkflowActionParameters"].(map[string]any)
				if p2 != nil && p2["GroupingIdentifier"] == gid && fmt.Sprintf("%v", p2["WFControlFlowMode"]) == "2" {
					fnEndIdx = j
					break
				}
			}
		}
		if fnEndIdx == -1 {
			continue
		}

		fnActs := fnBlock[i+1 : fnEndIdx]
		fn := &CanonicalFunction{
			Name:    fnName,
			VarMap:  make(map[string]string),
			UUIDMap: make(map[string]string),
		}

		lastParamEnd := 0
		paramIdx := 0
		k := 0
		for k < len(fnActs) {
			kAct, _ := fnActs[k].(map[string]any)
			if kAct == nil {
				k++
				continue
			}
			id, _ := kAct["WFWorkflowActionIdentifier"].(string)
			kParams, _ := kAct["WFWorkflowActionParameters"].(map[string]any)

			if id == "is.workflow.actions.getvalueforkey" {
				key, _ := kParams["WFDictionaryKey"].(string)
				if strings.HasPrefix(key, "arg_") {
					paramName, _ := kParams["CustomOutputName"].(string)
					cleanParamName := strings.TrimPrefix(paramName, "_")
					u, _ := kParams["UUID"].(string)
					k++
					lastParamEnd = k
					if k < len(fnActs) {
						nxtAct, _ := fnActs[k].(map[string]any)
						if nxtAct != nil && nxtAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.number" {
							nxtP, _ := nxtAct["WFWorkflowActionParameters"].(map[string]any)
							if nxtP != nil {
								if nxtU, ok := nxtP["UUID"].(string); ok && nxtU != "" {
									u = nxtU
								}
								if nxtName, ok := nxtP["CustomOutputName"].(string); ok && nxtName != "" {
									paramName = nxtName
									cleanParamName = strings.TrimPrefix(paramName, "_")
								}
							}
							k++
							lastParamEnd = k
						}
					}
					paramUUID := fmt.Sprintf("PARAM-%s-%s", fnName, cleanParamName)
					fn.Parameters = append(fn.Parameters, CanonicalParam{
						Name:  cleanParamName,
						Index: paramIdx,
					})
					if u != "" {
						fn.UUIDMap[u] = paramUUID
					}
					paramIdx++
					continue
				} else if strings.HasPrefix(key, "has_arg_") {
					k++
					lastParamEnd = k
					var defActions []any
					paramName := ""
					u := ""
					if k < len(fnActs) {
						cAct, _ := fnActs[k].(map[string]any)
						if cAct != nil && cAct["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
							cP, _ := cAct["WFWorkflowActionParameters"].(map[string]any)
							cGid, _ := cP["GroupingIdentifier"].(string)
							otherwiseIdx := -1
							cEndIdx := -1
							for m := k + 1; m < len(fnActs); m++ {
								mA, _ := fnActs[m].(map[string]any)
								if mA != nil && mA["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
									mP, _ := mA["WFWorkflowActionParameters"].(map[string]any)
									if mP != nil && mP["GroupingIdentifier"] == cGid {
										mode := fmt.Sprintf("%v", mP["WFControlFlowMode"])
										if mode == "1" {
											otherwiseIdx = m
										} else if mode == "2" {
											cEndIdx = m
											paramName, _ = mP["CustomOutputName"].(string)
											u, _ = mP["UUID"].(string)
											break
										}
									}
								}
							}
							if otherwiseIdx != -1 && cEndIdx != -1 {
								for d := otherwiseIdx + 1; d < cEndIdx; d++ {
									dAct, _ := fnActs[d].(map[string]any)
									if dAct != nil {
										dId, _ := dAct["WFWorkflowActionIdentifier"].(string)
										if dId != "is.workflow.actions.nothing" {
											defActions = append(defActions, deepCloneValue(dAct))
										}
									}
								}
								k = cEndIdx + 1
								lastParamEnd = k
							}
						}
					}
					cleanParamName := strings.TrimPrefix(paramName, "_")
					paramUUID := fmt.Sprintf("PARAM-%s-%s", fnName, cleanParamName)
					fn.Parameters = append(fn.Parameters, CanonicalParam{
						Name:           cleanParamName,
						Index:          paramIdx,
						DefaultActions: defActions,
					})
					if u != "" {
						fn.UUIDMap[u] = paramUUID
					}
					paramIdx++
					continue
				}
			}
			k++
		}

		for b := lastParamEnd; b < len(fnActs); b++ {
			fn.Body = append(fn.Body, deepCloneValue(fnActs[b]))
		}
		for len(fn.Body) > 0 {
			lastAct, _ := fn.Body[len(fn.Body)-1].(map[string]any)
			if lastAct == nil {
				fn.Body = fn.Body[:len(fn.Body)-1]
				continue
			}
			lastId, _ := lastAct["WFWorkflowActionIdentifier"].(string)
			lastP, _ := lastAct["WFWorkflowActionParameters"].(map[string]any)
			if lastId == "is.workflow.actions.nothing" || (lastId == "is.workflow.actions.output" && (lastP == nil || len(lastP) == 0)) {
				fn.Body = fn.Body[:len(fn.Body)-1]
			} else {
				break
			}
		}

		funcs[fnName] = fn
	}

	return funcs, mainActions
}

func normalizeFunctionEnvelope(actions []any) []any {
	if len(actions) == 0 {
		return actions
	}

	isLegacyFn := false
	isV2Fn := false

	for _, aRaw := range actions {
		act, ok := aRaw.(map[string]any)
		if !ok {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			continue
		}
		if p["WFDictionaryKey"] == "cherri_functions" {
			isLegacyFn = true
			break
		}
		if p["WFConditionalActionString"] == "__cherri_fn_envelope_v2__" {
			isV2Fn = true
			break
		}
		if p["WFVariableName"] == "_cherri_is_fn" {
			isV2Fn = true
			break
		}
	}

	if !isLegacyFn && !isV2Fn {
		return actions
	}

	var funcs map[string]*CanonicalFunction
	var mainActions []any

	if isV2Fn {
		funcs, mainActions = parseFunctionsV2(actions)
	} else if isLegacyFn {
		funcs, mainActions = parseFunctionsLegacy(actions)
	}

	mainActions = forwardIdentityGettext(mainActions)
	for _, aRaw := range mainActions {
		act, ok := aRaw.(map[string]any)
		if !ok {
			continue
		}
		p, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if p == nil {
			continue
		}
		if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
			normalizeRunWorkflowCallDict(p)
		} else if act["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
			if wf, ok := p["WFWorkflow"].(map[string]any); ok {
				if isSelf, _ := wf["isSelf"].(bool); isSelf || p["WFWorkflowName"] == "v2" || p["WFWorkflowName"] == "Shortcut" || p["WFWorkflowName"] == "tmp_v2" {
					p["WFWorkflowName"] = ""
					delete(wf, "workflowIdentifier")
					delete(wf, "workflowName")
				}
			}
		}
	}

	var sortedFnNames []string
	for fn := range funcs {
		sortedFnNames = append(sortedFnNames, fn)
	}
	sort.Strings(sortedFnNames)

	var result []any

	for _, fnName := range sortedFnNames {
		fn := funcs[fnName]

		var paramsList []any
		for _, p := range fn.Parameters {
			normDefActions := deepCloneValue(p.DefaultActions).([]any)
			normDefActions = forwardIdentityGettext(normDefActions)
			unwrapMathSingleTokens(normDefActions)
			for _, dActRaw := range normDefActions {
				if dAct, ok := dActRaw.(map[string]any); ok {
					if dParams, ok := dAct["WFWorkflowActionParameters"].(map[string]any); ok {
						delete(dParams, "CustomOutputName")
						delete(dParams, "UUID")
					}
				}
			}
			pMap := map[string]any{
				"Name":           p.Name,
				"Index":          p.Index,
				"DefaultActions": normDefActions,
			}
			paramsList = append(paramsList, pMap)
		}

		declAct := map[string]any{
			"WFWorkflowActionIdentifier": "cherri.canonical.function.decl",
			"WFWorkflowActionParameters": map[string]any{
				"FunctionName": fnName,
				"Parameters":   paramsList,
			},
		}
		result = append(result, declAct)

		body := deepCloneValue(fn.Body).([]any)
		if isLegacyFn {
			fwdMap := make(map[string]ForwardTarget)
			for vName, pUUID := range fn.VarMap {
				fwdMap[vName] = ForwardTarget{
					ProducerUUID: pUUID,
					VariableName: vName,
				}
			}
			rewriteVariableReferences(body, fwdMap)
			body = forwardSingleWriteVariables(body)
		} else if isV2Fn {
			body = foldGettextIntoOutput(body)
			rewriteActionUUIDReferences(body, fn.UUIDMap)
		}

		body = forwardIdentityGettext(body)
		unwrapMathSingleTokens(body)

		for _, bAct := range body {
			if m, ok := bAct.(map[string]any); ok {
				if p, ok := m["WFWorkflowActionParameters"].(map[string]any); ok {
					if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
						normalizeRunWorkflowCallDict(p)
					}
					if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
						if wf, ok := p["WFWorkflow"].(map[string]any); ok {
							if isSelf, _ := wf["isSelf"].(bool); isSelf || p["WFWorkflowName"] == "v2" || p["WFWorkflowName"] == "Shortcut" || p["WFWorkflowName"] == "tmp_v2" {
								p["WFWorkflowName"] = ""
								delete(wf, "workflowIdentifier")
								delete(wf, "workflowName")
							}
						}
					}
				}
			}
		}

		result = append(result, body...)

		endAct := map[string]any{
			"WFWorkflowActionIdentifier": "cherri.canonical.function.end",
			"WFWorkflowActionParameters": map[string]any{
				"FunctionName": fnName,
			},
		}
		result = append(result, endAct)
	}

	mainEntryAct := map[string]any{
		"WFWorkflowActionIdentifier": "cherri.canonical.main.entry",
	}
	result = append(result, mainEntryAct)
	result = append(result, mainActions...)

	return result
}
