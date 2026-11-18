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
