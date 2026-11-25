package main

import "fmt"

// Conditional inputs use the same structured producer tokens as authored
// constant bindings. Parse each input once at its written position; preserve
// the native value type and let the canonical backend encode its output.
func materializeLegacyConditionalInput(argument actionArgument) actionArgument {
	if argument.valueType == Variable {
		if _, okay := argAsVarValue(argument); !okay {
			parserError("E_CONDITION_INPUT: a conditional reference has no structured value")
		}
		return argument
	}
	if argument.valueType == Action {
		return materializeLegacyActionArgument(argument)
	}
	switch argument.valueType {
	case Integer, Float, Bool, String, RawString, Expression, Dict:
		if argument.value == nil {
			parserError("E_CONDITION_INPUT: a conditional input requires a value")
		}
	default:
		parserError(fmt.Sprintf("E_CONDITION_INPUT: type %q cannot be used as a conditional input; use an explicit supported value producer", argument.valueType))
	}
	name := ""
	if legacyScopedParsing != nil {
		name = legacyScopedParsing.block.bindingName()
	} else {
		name = legacyInlineArgumentName(action{ident: "conditional-input"})
	}
	definition := varValue{variableType: "Variable", valueType: argument.valueType, value: argument.value, constant: true}
	variables[name] = definition
	if legacyScopedParsing != nil {
		legacyScopedParsing.step.vars[name] = definition
	}
	tokens = append(tokens, token{typeof: Variable, ident: name, valueType: argument.valueType, value: argument.value})
	return actionArgument{valueType: Variable, value: varValue{variableType: "Variable", valueType: argument.valueType, value: name, constant: true}}
}

// Imported and authored control tokens share this checked boundary. Invalid
// internal inputs produce a compiler diagnostic instead of an unchecked cast.
func legacyConditionalReference(arguments []actionArgument) varValue {
	if len(arguments) == 0 || arguments[0].valueType != Variable {
		exit("E_CONDITION_INPUT: conditional input was not materialized")
	}
	reference, okay := argAsVarValue(arguments[0])
	if !okay {
		exit("E_CONDITION_INPUT: conditional input has no structured reference")
	}
	return reference
}
