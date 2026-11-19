package shortcutcompare

import (
	"fmt"
	"strings"
)

// Builtin module qualification is an internal dispatcher symbol. Keep external
// modules and colliding local declarations distinct, and never rewrite payload
// text merely because it happens to contain a function name.
func normalizeBuiltinFunctionSymbols(functions map[string]*CanonicalFunction, main []any) {
	const prefix = "module:builtin:stdfunc::"
	var aliases = map[string]string{}
	var parameters = map[string]string{}
	for name, fn := range functions {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		var bare = strings.TrimPrefix(name, prefix)
		if bare == "" || strings.Contains(bare, "::") || functions[bare] != nil {
			continue
		}
		aliases[name] = bare
		for _, param := range fn.Parameters {
			parameters[fmt.Sprintf("PARAM-%s-%s", name, param.Name)] = fmt.Sprintf("PARAM-%s-%s", bare, param.Name)
		}
	}
	if len(aliases) == 0 {
		return
	}
	for original, canonical := range aliases {
		var fn = functions[original]
		delete(functions, original)
		fn.Name = canonical
		functions[canonical] = fn
	}
	var rewriteCalls = func(actions []any) {
		for _, raw := range actions {
			var action, ok = raw.(map[string]any)
			if !ok || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
				continue
			}
			var params, _ = action["WFWorkflowActionParameters"].(map[string]any)
			var items, _ = params["WFItems"].(map[string]any)
			var value, _ = items["Value"].(map[string]any)
			var fields, _ = toSlice(value["WFDictionaryFieldValueItems"])
			var hasEnvelope bool
			var functionValues []map[string]any
			for _, rawField := range fields {
				var field, _ = rawField.(map[string]any)
				var key, _ = field["WFKey"].(map[string]any)
				var keyValue, _ = key["Value"].(map[string]any)
				var content, _ = field["WFValue"].(map[string]any)
				var contentValue, _ = content["Value"].(map[string]any)
				if keyValue["string"] == "_cherri_magic" && contentValue["string"] == "__cherri_fn_envelope_v2__" {
					hasEnvelope = true
				}
				if keyValue["string"] == "function" {
					functionValues = append(functionValues, contentValue)
				}
			}
			if hasEnvelope && len(functionValues) == 1 {
				var name, _ = functionValues[0]["string"].(string)
				if replacement, ok := aliases[name]; ok {
					functionValues[0]["string"] = replacement
				}
			}
		}
	}
	rewriteCalls(main)
	for _, fn := range functions {
		for original, target := range fn.UUIDMap {
			if replacement, ok := parameters[target]; ok {
				fn.UUIDMap[original] = replacement
			}
		}
		for original, target := range fn.VarMap {
			if replacement, ok := parameters[target]; ok {
				fn.VarMap[original] = replacement
			}
		}
		rewriteActionUUIDReferences(fn.Body, parameters)
		rewriteCalls(fn.Body)
		for _, param := range fn.Parameters {
			rewriteActionUUIDReferences(param.DefaultActions, parameters)
			rewriteCalls(param.DefaultActions)
		}
	}
}

// A Set Variable from Clipboard evaluates a snapshot at that position. Replacing
// its downstream uses with a live Clipboard token would erase that read and can
// hide a mutation between capture and use. Canonicalize only the exact native
// untransformed input to an explicit Get Clipboard producer.
func normalizeClipboardSnapshot(action, params map[string]any, actions []any, index int) (string, bool) {
	for key := range params {
		switch key {
		case "WFInput", "WFVariableName", "UUID", "CustomOutputName":
		default:
			return "", false
		}
	}
	var input, ok = params["WFInput"].(map[string]any)
	if !ok || len(input) != 2 || input["WFSerializationType"] != "WFTextTokenAttachment" {
		return "", false
	}
	var value, valueOK = input["Value"].(map[string]any)
	if !valueOK || len(value) != 1 || value["Type"] != "Clipboard" {
		return "", false
	}
	var uuid string
	if raw, exists := params["UUID"]; exists {
		uuid, ok = raw.(string)
		if !ok || uuid == "" {
			return "", false
		}
	} else {
		var used = collectAllReferencedUUIDs(actions)
		for _, raw := range actions {
			var item, _ = raw.(map[string]any)
			var p, _ = item["WFWorkflowActionParameters"].(map[string]any)
			if id, ok := p["UUID"].(string); ok {
				used[id] = true
			}
		}
		uuid = fmt.Sprintf("PARITY-CLIPBOARD-CAPTURE-%d", index)
		for used[uuid] {
			uuid += "-"
		}
	}
	action["WFWorkflowActionIdentifier"] = "is.workflow.actions.getclipboard"
	action["WFWorkflowActionParameters"] = map[string]any{"UUID": uuid}
	return uuid, true
}
