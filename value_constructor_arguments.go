package main

import "fmt"

// Intrinsics do not pass through collectAction's required-argument check.
// Reuse their canonical parameter definitions before a serializer indexes them.
func collectValueConstructorArguments() []actionArgument {
	var arguments = collectArguments()
	currentAction.arguments = arguments
	checkRequiredArgs()
	for index, parameter := range currentAction.definition.parameters {
		if index < len(arguments) && arguments[index].valueType != Nil {
			continue
		}
		if parameter.defaultValue == nil {
			parserError(fmt.Sprintf("Missing required argument %q in %s()", parameter.name, currentAction.identifier))
		}
		var argument = actionArgument{valueType: parameter.validType, value: parameter.defaultValue}
		if index < len(arguments) {
			arguments[index] = argument
		} else {
			arguments = append(arguments, argument)
		}
	}
	return arguments
}
