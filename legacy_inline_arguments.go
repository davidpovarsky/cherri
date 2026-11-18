package main

import (
	"fmt"
	"strings"
)

// Every inline value-producing call uses the existing constant token path.
// Parsing the expression once emits its structured producer before its consumer;
// normal native argument codecs receive the exact action-output reference.
func materializeLegacyActionArgument(argument actionArgument) actionArgument {
	if argument.valueType != Action {
		return argument
	}
	call, ok := argument.value.(action)
	if !ok {
		return argument
	}
	if legacyScopedValueType(argument, variables, nil) == Nil {
		if legacyScopedParsing != nil || call.block != nil {
			parserError("E_BLOCK_VALUE_REQUIRED: a void action cannot be used as a scoped argument")
		}
		parserError("E_ARGUMENT_VALUE_REQUIRED: a void action cannot be used as an argument")
	}
	var name string
	if legacyScopedParsing != nil {
		name = legacyScopedParsing.block.bindingName()
	} else if call.block != nil {
		name = call.block.prefix + "_argument"
	} else {
		name = legacyInlineArgumentName(call)
	}
	definition := varValue{variableType: "Variable", valueType: Action, value: call, constant: true}
	variables[name] = definition
	if legacyScopedParsing != nil {
		legacyScopedParsing.step.vars[name] = definition
	}
	tokens = append(tokens, token{typeof: Variable, ident: name, valueType: Action, value: call})
	return actionArgument{valueType: Variable, value: varValue{variableType: "Variable", valueType: Action, value: name, constant: true}}
}

func legacyInlineArgumentName(call action) string {
	for attempt := 0; ; attempt++ {
		salt := fmt.Sprintf("inline-argument:%d:%d:%d:%d:%s", lineIdx, idx, len(tokens), attempt, call.ident)
		name := "__cherri_inline_" + strings.ReplaceAll(createUUID(&salt), "-", "")
		if !legacyInlineNameTaken(name) {
			return name
		}
	}
}

func legacyInlineNameTaken(name string) bool {
	if _, exists := variables[name]; exists {
		return true
	}
	if _, exists := globals[name]; exists {
		return true
	}
	if _, exists := questions[name]; exists {
		return true
	}
	if _, exists := uuids[name]; exists {
		return true
	}
	if _, exists := importedSymbolOutputNames[name]; exists {
		return true
	}
	if _, exists := importedSymbolVariableNames[name]; exists {
		return true
	}
	if _, exists := importedSymbolIsVariable[name]; exists {
		return true
	}
	if _, exists := importedSymbolValueTypes[name]; exists {
		return true
	}
	if _, exists := importedSymbolDictAggr[name]; exists {
		return true
	}
	if _, exists := importedSymbolAppendVars[name]; exists {
		return true
	}
	// Future authored declarations must also stay independent. Checking the
	// existing source buffers is only collision avoidance, never expression parsing.
	return strings.Contains(contents, name) || strings.Contains(originalContents, name)
}

// A later argument can emit actions while it is parsed. Capture earlier mutable
// inputs immediately before that stage, using the existing Get Variable action
// and constant token path. The expressions themselves are never parsed twice.
func snapshotLegacyArgumentsBefore(arguments []actionArgument, tokenStart int) {
	if len(arguments) == 0 || len(tokens) == tokenStart {
		return
	}
	later := append([]token(nil), tokens[tokenStart:]...)
	tokens = tokens[:tokenStart]
	for i := range arguments {
		arguments[i] = snapshotLegacyArgument(arguments[i])
	}
	tokens = append(tokens, later...)
}

func snapshotLegacyArgument(argument actionArgument) actionArgument {
	switch argument.valueType {
	case Variable:
		if reference, ok := argument.value.(varValue); ok && legacyInlineMutableReference(reference) {
			return materializeLegacyActionArgument(actionArgument{
				valueType: Action, value: legacyInlineSnapshotAction(argument),
			})
		}
	case String, Expression:
		if value, ok := argument.value.(string); ok {
			argument.value = snapshotLegacyInlineText(value)
		}
	}
	return argument
}

// The native operation is compiler-owned. Its captured definition must not be
// selected again from a source callable name during generation.
func legacyInlineSnapshotAction(argument actionArgument) action {
	definition := &actionDefinition{
		identifier: "getvariable",
		parameters: []parameterDefinition{{name: "variable", validType: Variable, key: "WFVariable"}},
		outputType: legacyScopedValueType(argument, variables, nil),
	}
	return action{ident: "getVariable", def: definition, args: []actionArgument{argument}}
}

func legacyInlineMutableReference(reference varValue) bool {
	if reference.descriptor != nil && reference.descriptor.Kind != "" {
		return reference.descriptor.Kind != "ActionOutput" && reference.descriptor.Kind != "ExtensionInput"
	}
	name, ok := reference.value.(string)
	if !ok {
		return false
	}
	if importedSymbolIsVariable[name] {
		return true
	}
	if value, found := variables[name]; found {
		if value.descriptor != nil && value.descriptor.Kind != "" {
			return value.descriptor.Kind != "ActionOutput" && value.descriptor.Kind != "ExtensionInput"
		}
		return !value.constant
	}
	if global, found := globals[name]; found {
		return global.variableType != "ExtensionInput"
	}
	return !reference.constant
}

func snapshotLegacyInlineText(value string) string {
	// Use the parser's existing interpolation grammar. Only the base reference
	// changes; literal text and existing property/coercion suffixes stay intact.
	matches := checkInlineVarRegex.FindAllStringSubmatchIndex(value, -1)
	var result strings.Builder
	last := 0
	for _, match := range matches {
		sourceName := value[match[2]:match[3]]
		name := strings.TrimPrefix(sourceName, "@")
		reference := varValue{variableType: "Variable", valueType: Variable, value: name}
		if definition, found := variables[name]; found {
			reference.constant = definition.constant
		}
		if !legacyInlineMutableReference(reference) {
			continue
		}
		captured := snapshotLegacyArgument(actionArgument{valueType: Variable, value: reference})
		target := captured.value.(varValue).value.(string)
		result.WriteString(value[last:match[2]])
		result.WriteString(target)
		last = match[3]
	}
	if last == 0 {
		return value
	}
	result.WriteString(value[last:])
	return result.String()
}
