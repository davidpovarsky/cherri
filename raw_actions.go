package main

import (
	"encoding/json"
	"regexp"
	"strconv"
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
			{
				name:      "envelope",
				optional:  true,
				validType: Dict,
			},
		},
		makeParams: func(args []actionArgument) map[string]any {
			if envelope, provided, err := rawLegacyActionEnvelope(args); provided {
				if err != nil {
					exit(err.Error())
				}
				if params, ok := envelope["WFWorkflowActionParameters"].(map[string]any); ok {
					return deepClonePlistMap(params)
				}
				return map[string]any{}
			}
			if len(args) == 1 {
				return map[string]any{}
			}
			if raw, ok := args[1].value.(rawPlistDict); ok {
				return deepClonePlistMap(raw.Data)
			}

			var params = getArgValue(args[1]).(map[string]interface{})
			handleRawActionParams(params)

			return params
		},
		outputType: Variable,
	}
}

func handleRawParams(params map[string]any) {
	handleRawParamsWithOptions(params, false)
}

func handleRawActionParams(params map[string]any) {
	handleRawParamsWithOptions(params, true)
}

func handleRawParamsWithOptions(params map[string]any, normalizeIntentApp bool) {
	for key, value := range params {
		if key == "WFWorkflow" {
			continue
		}
		if key == "WFVariableName" {
			if varName, ok := value.(string); ok && varName != "" {
				if _, exists := variables[varName]; !exists {
					variables[varName] = varValue{
						variableType: "Variable",
						value:        varName,
					}
				}
			}
		}
		if key == "AppIntentDescriptor" || (normalizeIntentApp && key == "IntentAppDefinition") {
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

// JSON normally prints an integral real as an integer token. Keep its lexical
// kind so the native-parameter decoder can preserve it without rewriting any
// symbolic references or converting the whole action to an opaque payload.
func rawActionJSONNumbers(value any) any {
	switch number := value.(type) {
	case float64:
		var text = strconv.FormatFloat(number, 'g', -1, 64)
		if !strings.ContainsAny(text, ".eE") {
			text += ".0"
		}
		return json.Number(text)
	case float32:
		return rawActionJSONNumbers(float64(number))
	case map[string]any:
		var result = make(map[string]any, len(number))
		for key, child := range number {
			result[key] = rawActionJSONNumbers(child)
		}
		return result
	case []any:
		var result = make([]any, len(number))
		for index, child := range number {
			result[index] = rawActionJSONNumbers(child)
		}
		return result
	default:
		return value
	}
}
