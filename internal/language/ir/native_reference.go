package ir

import (
	"fmt"
	"strings"
)

// NormalizeNativeReferenceKind is the native.ref lowerer's existing kind and
// required-field contract. It validates reference shape, not producer existence,
// runtime cardinality, scalar type, or Apple's acceptance of opaque metadata.
func NormalizeNativeReferenceKind(kind, outputUUID, outputName string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "actionoutput", "output":
		if strings.TrimSpace(outputUUID) == "" {
			return "", fmt.Errorf("native.ref for ActionOutput requires a non-empty uuid")
		}
		return "ActionOutput", nil
	case "variable", "var":
		if strings.TrimSpace(outputName) == "" {
			return "", fmt.Errorf("native.ref for Variable requires a non-empty name")
		}
		return "Variable", nil
	case "shortcutinput", "extensioninput", "input":
		return "ExtensionInput", nil
	case "clipboard":
		return "Clipboard", nil
	case "currentdate", "date":
		return "CurrentDate", nil
	case "devicedetails", "device":
		return "DeviceDetails", nil
	case "ask", "askeachtime":
		return "Ask", nil
	default:
		if kind == "" {
			return "", fmt.Errorf("native.ref requires a reference kind (e.g. ActionOutput, Variable, ShortcutInput)")
		}
		return "", fmt.Errorf("unsupported or invalid native.ref kind: %q", kind)
	}
}

// NativeReferenceAggrandizements checks only the list/dictionary envelope that
// native.ref has always required. Callers retain ownership of the original
// values; extra fields and nested opaque data are neither rewritten nor dropped.
func NativeReferenceAggrandizements(raw any) ([]map[string]any, error) {
	if object, ok := raw.(map[string]any); ok {
		if nested, exists := object["Aggrandizements"]; exists {
			raw = nested
		}
	}
	list, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("native.ref aggrandizements must be an inline list of dictionaries, got %T", raw)
	}
	var result []map[string]any
	for index, item := range list {
		object, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("native.ref aggrandizements item %d must be a dictionary, got %T", index, item)
		}
		result = append(result, object)
	}
	return result, nil
}
