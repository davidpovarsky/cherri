package main

import (
	"regexp"
	"strings"
)

var rawActionVariableValueRegex = regexp.MustCompile(`^\$\{@?.*}$`)

func defineRawAction() {
	actions["rawAction"] = &actionDefinition{
		parameters: []parameterDefinition{
			{
				name:      "identifier",
				validType: String,
			},
			{
				name:      "parameters",
				optional:  true,
				validType: Dict,
			},
		},
		makeParams: func(args []actionArgument) map[string]any {
			if len(args) == 1 {
				return map[string]any{}
			}

			var params = getArgValue(args[1]).(map[string]interface{})
			handleRawParams(params)

			return params
		},
		outputType: Variable,
	}
}

func handleRawParams(params map[string]any) {
	for key, value := range params {
		if key == "WFWorkflow" {
			continue
		}
		if key == "AppIntentDescriptor" {
			params[key] = normalizeNativeDescriptor(value)
			continue
		}
		params[key] = normalizeRawActionParamValue(value)
	}
}

func isPlainNativeDictionary(v map[string]any) bool {
	if _, ok := v["AppIntentIdentifier"]; ok {
		return true
	}
	if _, ok := v["ActionRequiresAppInstallation"]; ok {
		return true
	}
	if _, ok := v["BundleIdentifier"]; ok {
		if _, ok2 := v["TeamIdentifier"]; ok2 {
			return true
		}
	}
	if _, ok := v["fileLocation"]; ok {
		return true
	}
	return false
}

func normalizeNativeDescriptor(value any) any {
	if value == nil {
		return nil
	}
	switch m := value.(type) {
	case map[string]any:
		res := make(map[string]any, len(m))
		for k, v := range m {
			res[k] = normalizeNativeScalar(v)
		}
		return res
	default:
		return value
	}
}

func normalizeNativeScalar(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case string:
		if strings.EqualFold(val, "true") {
			return true
		}
		if strings.EqualFold(val, "false") {
			return false
		}
		return val
	case map[string]any:
		return normalizeNativeDescriptor(val)
	default:
		return val
	}
}

func normalizeRawActionParamValue(value any) any {
	if value == nil {
		return nil
	}

	switch v := value.(type) {
	case string:
		if !strings.ContainsAny(v, "{}") {
			return value
		}
		if rawActionVariableValueRegex.MatchString(v) {
			return variableValue(varValue{
				value: strings.Trim(v, "${@}"),
			})
		}
		return attachmentValues(v)
	case map[string]any:
		if _, hasType := v["WFSerializationType"]; hasType {
			return v
		}
		if isPlainNativeDictionary(v) {
			return normalizeNativeDescriptor(v)
		}
		var normalized any = normalizeRawActionDictionary(v)
		return makeDictionaryValue(&normalized)
	case []any:
		return makeRawActionArrayValue(v)
	default:
		return value
	}
}

func normalizeRawActionDictionary(value map[string]any) map[string]any {
	var normalized = make(map[string]any, len(value))
	for key, item := range value {
		normalized[key] = normalizeRawActionParamValue(item)
	}
	return normalized
}

func makeRawActionArrayValue(value []any) WFArrayValue {
	var items = make([]WFDictionaryFieldValueItem, 0, len(value))
	for _, item := range value {
		items = append(items, makeDictionaryItem("", normalizeRawActionParamValue(item)))
	}
	return makeArrayValue(items)
}
