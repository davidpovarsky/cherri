/*
 * Copyright (c) Cherri Language v2.0
 * Shared Authoritative Control-Flow Builder
 */

package main

import "github.com/electrikmilk/cherri/internal/language/backend"

const (
	ControlFlowStart = backend.ControlFlowStart
	ControlFlowElse  = backend.ControlFlowElse
	ControlFlowEnd   = backend.ControlFlowEnd
)

// WFMenuItem aliases the authoritative backend.WFMenuItem type.
type WFMenuItem = backend.WFMenuItem

// BuildIfStart constructs the beginning action parameters for a conditional block.
func BuildIfStart(groupingID string, input any, conditionCode int, compareVal any, isLegacy bool) map[string]any {
	return backend.BuildIfStart(groupingID, input, conditionCode, compareVal, isLegacy)
}

// BuildIfElse constructs the else branch action parameters.
func BuildIfElse(groupingID string) map[string]any {
	return backend.BuildIfElse(groupingID)
}

// BuildIfEnd constructs the closing action parameters for a conditional block.
func BuildIfEnd(groupingID string, outputUUID string) map[string]any {
	return backend.BuildIfEnd(groupingID, outputUUID)
}

// BuildRepeatCountStart constructs the beginning action parameters for a repeat count loop.
func BuildRepeatCountStart(groupingID string, count any) map[string]any {
	return backend.BuildRepeatCountStart(groupingID, count)
}

// BuildRepeatEachStart constructs the beginning action parameters for a repeat each loop.
func BuildRepeatEachStart(groupingID string, iterable any) map[string]any {
	return backend.BuildRepeatEachStart(groupingID, iterable)
}

// BuildLoopEnd constructs the closing action parameters for a loop block.
func BuildLoopEnd(groupingID string, outputUUID string) map[string]any {
	return backend.BuildLoopEnd(groupingID, outputUUID)
}

// BuildMenuStart constructs the beginning action parameters for a menu block.
func BuildMenuStart(groupingID string, prompt any, itemTitles []string) map[string]any {
	return backend.BuildMenuStart(groupingID, prompt, itemTitles)
}

// BuildMenuCase constructs a case branch action parameters for a menu.
func BuildMenuCase(groupingID string, title string) map[string]any {
	return backend.BuildMenuCase(groupingID, title)
}

// BuildMenuEnd constructs the closing action parameters for a menu block.
func BuildMenuEnd(groupingID string, outputUUID string) map[string]any {
	return backend.BuildMenuEnd(groupingID, outputUUID)
}
