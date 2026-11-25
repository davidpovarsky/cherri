package main

import "strings"

const requireSourceTypeKey = "__CherriRequireSourceType"

func requireSemanticCategory(name string) string {
	name = strings.TrimSuffix(name, "?")
	switch name {
	case "Text", "String":
		return "text"
	case "Dict", "Dictionary", "Record":
		return "dictionary"
	case "Arr", "List":
		return "list"
	case "Bool", "Boolean", "Number", "Integer", "Float", "File", "Image", "URL", "Date", "Contact", "CalendarEvent":
		return "items"
	}
	if strings.HasPrefix(name, "List<") {
		return "list"
	}
	if strings.HasPrefix(name, "Map<") {
		return "dictionary"
	}
	return ""
}

func requireLiteralCategory(value any) string {
	switch value.(type) {
	case string, WFTextTokenString:
		return "text"
	case WFDictionaryFieldValue:
		return "dictionary"
	case []any, WFArrayValue:
		return "list"
	case bool, int, int64, uint64, float64, float32:
		return "items"
	}
	return ""
}

func requireLegacySourceType(arg actionArgument) string {
	kind := arg.valueType
	if kind == Variable {
		ref, ok := arg.value.(varValue)
		if !ok {
			return ""
		}
		name, ok := ref.value.(string)
		if !ok {
			return ""
		}
		// System magic variables such as Clipboard/ShortcutInput are dynamic even
		// where the Legacy parser uses String as a permissive placeholder type.
		actual, found := variables[name]
		if !found {
			return ""
		}
		kind = actual.valueType
	}
	switch kind {
	case String, RawString:
		return "Text"
	case Integer, Float:
		return "Number"
	case Bool:
		return "Bool"
	case Dict:
		return "Dictionary"
	case Arr:
		return "List"
	}
	return ""
}

// A condition has a stricter proof than a content-category hint: Optional,
// transformed references and unknown native content cannot acquire truthiness.
const requireConditionTypeKey = "__CherriRequireConditionType"

func requireLegacyConditionType(argument actionArgument) string {
	if legacyScopedValueType(argument, variables, nil) == Bool {
		return "Bool"
	}
	return ""
}

func requireConditionReference(raw any) bool {
	valid := func(kind, output, name string, transformed bool) bool {
		if transformed {
			return false
		}
		return (kind == "ActionOutput" && output != "") || (kind == "Variable" && name != "")
	}
	switch value := raw.(type) {
	case WFTextTokenAttachment:
		return value.WFSerializationType == "WFTextTokenAttachment" &&
			valid(value.Value.Type, value.Value.OutputUUID, value.Value.VariableName, len(value.Value.Aggrandizements) != 0)
	case *WFTextTokenAttachment:
		return value != nil && requireConditionReference(*value)
	case map[string]any:
		body, okay := value["Value"].(map[string]any)
		if !okay || value["WFSerializationType"] != "WFTextTokenAttachment" {
			return false
		}
		kind, _ := body["Type"].(string)
		output, _ := body["OutputUUID"].(string)
		name, _ := body["VariableName"].(string)
		transformed := false
		if transforms, found := body["Aggrandizements"]; found {
			items, okay := transforms.([]any)
			if !okay {
				return false
			}
			transformed = len(items) != 0
		}
		return valid(kind, output, name, transformed)
	default:
		return false
	}
}

func requireConditionInputProved(params map[string]any, raw any) bool {
	hint, present := params[requireConditionTypeKey]
	if !present {
		hint, present = params[requireSourceTypeKey]
	}
	if present {
		name, okay := hint.(string)
		if !okay || (name != "Bool" && name != "Boolean") {
			return false
		}
		if _, literal := raw.(bool); literal {
			return true
		}
		return requireConditionReference(raw)
	}
	// Direct backend callers can prove an actual Boolean literal without a
	// frontend hint. Native references require a source type witness.
	_, literal := raw.(bool)
	return literal
}
