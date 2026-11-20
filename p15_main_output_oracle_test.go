package main

import "fmt"

// Keep all decoded actions and require the explicit empty result of an authored
// V2 main. Native imports and Legacy sources select their own output contract.
func p15M02MainVoidRowsError(rows []any) error {
	actions := make([]map[string]any, len(rows))
	for i, raw := range rows {
		var ok bool
		actions[i], ok = raw.(map[string]any)
		if !ok {
			return fmt.Errorf("native action %d is not a dictionary", i)
		}
	}
	return p15M02MainVoidBoundaryError(actions)
}

// Preserve the entire decoded action sequence; this checks only its explicit
// authored-main boundary. Each caller also asserts its complete scenario.
func p15M02MainVoidDebugError(actions []ShortcutAction) error {
	if len(actions) == 0 {
		return fmt.Errorf("main fallthrough has no explicit empty result")
	}
	last := actions[len(actions)-1]
	if last.WFWorkflowActionIdentifier != "is.workflow.actions.nothing" || len(last.WFWorkflowActionParameters) != 0 {
		return fmt.Errorf("main fallthrough has no exact empty terminal action")
	}
	if last.rawNativeEnvelope != nil {
		return p15M02MainVoidBoundaryError([]map[string]any{last.rawNativeEnvelope})
	}
	return nil
}
