package backend

import (
	"fmt"
	"strings"
)

// ValidateNativeActionEnvelope validates the structural keys of an explicitly
// opaque action. Other keys are data and are neither inferred nor normalized.
func ValidateNativeActionEnvelope(identifier string, envelope map[string]any) (map[string]any, bool, error) {
	actual, ok := envelope["WFWorkflowActionIdentifier"].(string)
	if !ok || strings.TrimSpace(actual) == "" || actual != identifier {
		return nil, false, fmt.Errorf("NATIVE_ACTION_ENVELOPE_IDENTIFIER: envelope identifier must exactly match %q", identifier)
	}
	raw, present := envelope["WFWorkflowActionParameters"]
	if !present {
		return nil, false, nil
	}
	params, ok := raw.(map[string]any)
	if !ok {
		return nil, false, fmt.Errorf("NATIVE_ACTION_ENVELOPE_PARAMETERS: exact parameters must be a dictionary, got %T", raw)
	}
	return params, true, nil
}
