/*
 * Copyright (c) Cherri Language v2.0
 * Shared Authoritative Control-Flow Builder
 */

package main

// BuildIfStart constructs the beginning action parameters for a conditional block.
func BuildIfStart(groupingID string, input any, conditionCode int, compareVal any, isLegacy bool) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  0,
	}
	if isLegacy {
		params["WFConditionalLegacyComparisonBehavior"] = true
	}
	if input != nil {
		params["WFInput"] = input
	}
	params["WFCondition"] = conditionCode

	if compareVal != nil {
		switch v := compareVal.(type) {
		case float64:
			params["WFNumberValue"] = v
		case int:
			params["WFNumberValue"] = float64(v)
		case int64:
			params["WFNumberValue"] = float64(v)
		case bool:
			bNum := 0.0
			if v {
				bNum = 1.0
			}
			params["WFNumberValue"] = bNum
		case string:
			params["WFConditionalActionString"] = v
		default:
			params["WFNumberValue"] = v
		}
	}
	return params
}

// BuildIfElse constructs the else branch action parameters.
func BuildIfElse(groupingID string) map[string]any {
	return map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  1,
	}
}

// BuildIfEnd constructs the closing action parameters for a conditional block.
func BuildIfEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  2,
	}
	if outputUUID != "" {
		params["UUID"] = outputUUID
	}
	return params
}

// BuildRepeatCountStart constructs the beginning action parameters for a repeat count loop.
func BuildRepeatCountStart(groupingID string, count any) map[string]any {
	return map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  0,
		"WFRepeatCount":      count,
	}
}

// BuildRepeatEachStart constructs the beginning action parameters for a repeat each loop.
func BuildRepeatEachStart(groupingID string, iterable any) map[string]any {
	return map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  0,
		"WFInput":            iterable,
	}
}

// BuildLoopEnd constructs the closing action parameters for a loop block.
func BuildLoopEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  2,
	}
	if outputUUID != "" {
		params["UUID"] = outputUUID
	}
	return params
}

// BuildMenuStart constructs the beginning action parameters for a menu block.
func BuildMenuStart(groupingID string, prompt any, itemTitles []string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  0,
	}
	if prompt != nil && prompt != "" {
		params["WFMenuPrompt"] = prompt
	}
	menuItems := make([]WFMenuItem, len(itemTitles))
	for i, title := range itemTitles {
		menuItems[i] = WFMenuItem{
			WFItemType: 0,
			WFValue:    title,
		}
	}
	params["WFMenuItems"] = menuItems
	return params
}

// BuildMenuCase constructs a case branch action parameters for a menu.
func BuildMenuCase(groupingID string, title string) map[string]any {
	return map[string]any{
		"GroupingIdentifier":        groupingID,
		"WFControlFlowMode":         1,
		"WFMenuItemTitle":           title,
		"WFMenuItemAttributedTitle": title,
	}
}

// BuildMenuEnd constructs the closing action parameters for a menu block.
func BuildMenuEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  2,
	}
	if outputUUID != "" {
		params["UUID"] = outputUUID
	}
	return params
}
