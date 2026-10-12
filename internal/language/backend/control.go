/*
 * Copyright (c) Cherri Language v2.0
 * Shared Authoritative Control-Flow Builder
 */

package backend

// Apple Shortcuts Control Flow Mode constants
const (
	ControlFlowStart = 0
	ControlFlowElse  = 1
	ControlFlowEnd   = 2
)

// WFMenuItem represents an item in an Apple Shortcuts Choose from Menu action.
type WFMenuItem struct {
	WFItemType int `plist:",omitempty"`
	WFValue    any `plist:",omitempty"`
}

// BuildIfStart constructs the beginning action parameters for a conditional block.
func BuildIfStart(groupingID string, input any, conditionCode int, compareVal any, isLegacy bool) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  ControlFlowStart,
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
		"WFControlFlowMode":  ControlFlowElse,
	}
}

// BuildIfEnd constructs the closing action parameters for a conditional block.
func BuildIfEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  ControlFlowEnd,
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
		"WFControlFlowMode":  ControlFlowStart,
		"WFRepeatCount":      count,
	}
}

// BuildRepeatEachStart constructs the beginning action parameters for a repeat each loop.
func BuildRepeatEachStart(groupingID string, iterable any) map[string]any {
	return map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  ControlFlowStart,
		"WFInput":            iterable,
	}
}

// BuildLoopEnd constructs the closing action parameters for a loop block.
func BuildLoopEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  ControlFlowEnd,
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
		"WFControlFlowMode":  ControlFlowStart,
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
		"WFControlFlowMode":         ControlFlowElse,
		"WFMenuItemTitle":           title,
		"WFMenuItemAttributedTitle": title,
	}
}

// BuildMenuEnd constructs the closing action parameters for a menu block.
func BuildMenuEnd(groupingID string, outputUUID string) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": groupingID,
		"WFControlFlowMode":  ControlFlowEnd,
	}
	if outputUUID != "" {
		params["UUID"] = outputUUID
	}
	return params
}
