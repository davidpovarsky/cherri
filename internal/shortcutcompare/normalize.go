/*
 * Copyright (c) Cherri Language v2.0
 * Semantic Shortcut Normalizer for Differential Parity Verification
 */

package shortcutcompare

import (
	"sort"
	"strings"
)

type ForwardTarget struct {
	ProducerUUID string
	VariableName string
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

	// 1. Remove Legacy Cherri top-level WFSerializationType quirk from action parameters
	for _, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				delete(params, "WFSerializationType")
			}
		}
	}

	// 2. Discover single-write setvariable actions for SSA Forwarding
	varWriteCounts := make(map[string]int)
	for _, actRaw := range actionsRaw {
		if actMap, ok := actRaw.(map[string]any); ok {
			if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" {
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
		if varName == "" || varWriteCounts[varName] > 1 {
			continue // Only normalize single-write SSA variables
		}

		producerUUID := extractOutputUUID(params["WFInput"])
		if producerUUID != "" {
			forwardMap[varName] = ForwardTarget{
				ProducerUUID: producerUUID,
				VariableName: varName,
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

	// 4. Identify unused 'is.workflow.actions.nothing' actions
	nothingIndicesToRemove := make(map[int]bool)
	usedUUIDs := collectAllReferencedUUIDs(actionsRaw)
	for i, actRaw := range actionsRaw {
		actMap, ok := actRaw.(map[string]any)
		if !ok || actMap["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
			continue
		}
		params, _ := actMap["WFWorkflowActionParameters"].(map[string]any)
		u, _ := params["UUID"].(string)
		if u == "" || !usedUUIDs[u] {
			nothingIndicesToRemove[i] = true
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

	return cloned
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

func normalizeActionOutputNames(obj any) {
	switch typed := obj.(type) {
	case map[string]any:
		if t, ok := typed["Type"].(string); ok {
			if t == "ActionOutput" {
				delete(typed, "OutputName")
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
				} else {
					// Direct token inside attachmentsByRange or parameters
					typed["Type"] = "ActionOutput"
					typed["OutputUUID"] = target.ProducerUUID
					typed["OutputName"] = target.VariableName
					delete(typed, "VariableName")
					delete(typed, "Value")
				}
				return
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
