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
