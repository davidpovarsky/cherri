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
	WFItemType int `plist:"WFItemType"`
	WFValue    any `plist:",omitempty"`
}

// ConditionOperandType represents the type of a comparison operand.
type ConditionOperandType int

const (
	OperandDefault ConditionOperandType = iota
	OperandNumber
	OperandString
	OperandDate
	OperandQuantity
)

// ConditionSpec encapsulates all parameters required to build an Apple Shortcuts conditional start action.
type ConditionSpec struct {
	GroupingIdentifier string
	IsLegacy           bool

	// Multi-condition representation (iOS 18+ WFConditions)
	Conditions any

	// Single condition representation (WFCondition / legacy / iOS < 18)
	Input         any
	ConditionCode int

	// Comparison operands (routed by operand type or set directly)
	DurationValue any // WFDuration
	DateValue     any // WFDate
	AnotherDate   any // WFAnotherDate
	NumberValue   any // WFNumberValue
	AnotherNumber any // WFAnotherNumber
	StringValue   any // WFConditionalActionString
}

// RouteComparisonValue routes a comparison value into the appropriate wire field.
func (s *ConditionSpec) RouteComparisonValue(val any, opType ConditionOperandType) {
	switch opType {
	case OperandDate:
		if s.DateValue == nil {
			s.DateValue = val
		} else {
			s.AnotherDate = val
		}
	case OperandNumber:
		if b, isBool := val.(bool); isBool {
			bNum := 0
			if b {
				bNum = 1
			}
			if s.NumberValue == nil {
				s.NumberValue = bNum
			} else {
				s.AnotherNumber = bNum
			}
		} else {
			if s.NumberValue == nil {
				s.NumberValue = val
			} else {
				s.AnotherNumber = val
			}
		}
	case OperandQuantity:
		s.DurationValue = val
	default:
		s.StringValue = val
	}
}

// BuildConditionalStart constructs the parameter map for an Apple Shortcuts conditional start action.
func BuildConditionalStart(spec ConditionSpec) map[string]any {
	params := map[string]any{
		"GroupingIdentifier": spec.GroupingIdentifier,
		"WFControlFlowMode":  ControlFlowStart,
	}
	if spec.IsLegacy {
		params["WFConditionalLegacyComparisonBehavior"] = true
	}
	if spec.Conditions != nil {
		params["WFConditions"] = spec.Conditions
		return params
	}

	if spec.Input != nil {
		params["WFInput"] = spec.Input
	}
	params["WFCondition"] = spec.ConditionCode

	if spec.DurationValue != nil {
		params["WFDuration"] = spec.DurationValue
	}
	if spec.DateValue != nil {
		params["WFDate"] = spec.DateValue
	}
	if spec.AnotherDate != nil {
		params["WFAnotherDate"] = spec.AnotherDate
	}
	if spec.NumberValue != nil {
		params["WFNumberValue"] = spec.NumberValue
	}
	if spec.AnotherNumber != nil {
		params["WFAnotherNumber"] = spec.AnotherNumber
	}
	if spec.StringValue != nil {
		params["WFConditionalActionString"] = spec.StringValue
	}

	return params
}

// BuildIfStartTyped constructs the beginning action parameters for a conditional block with explicit operand typing.
func BuildIfStartTyped(groupingID string, input any, conditionCode int, compareVal any, compareVal2 any, opType ConditionOperandType, isLegacy bool) map[string]any {
	spec := ConditionSpec{
		GroupingIdentifier: groupingID,
		Input:              input,
		ConditionCode:      conditionCode,
		IsLegacy:           isLegacy,
	}
	if compareVal != nil {
		switch v := compareVal.(type) {
		case int:
			compareVal = float64(v)
		case int64:
			compareVal = float64(v)
		case bool:
			bNum := 0.0
			if v {
				bNum = 1.0
			}
			compareVal = bNum
		}
		if opType != OperandDefault {
			spec.RouteComparisonValue(compareVal, opType)
		} else {
			switch v := compareVal.(type) {
			case float64:
				spec.NumberValue = v
			case string:
				spec.StringValue = v
			default:
				spec.NumberValue = v
			}
		}
	}
	if compareVal2 != nil {
		switch v := compareVal2.(type) {
		case int:
			compareVal2 = float64(v)
		case int64:
			compareVal2 = float64(v)
		}
		if opType != OperandDefault {
			spec.RouteComparisonValue(compareVal2, opType)
		} else {
			spec.NumberValue = compareVal2
		}
	}
	return BuildConditionalStart(spec)
}

// BuildIfStart constructs the beginning action parameters for a conditional block.
func BuildIfStart(groupingID string, input any, conditionCode int, compareVal any, isLegacy bool) map[string]any {
	return BuildIfStartTyped(groupingID, input, conditionCode, compareVal, nil, OperandDefault, isLegacy)
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
