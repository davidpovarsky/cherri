package main

import "fmt"

// argumentUsesAttachmentCarrier keeps reference contents separate from the
// parameter envelope. Apple VariableString state uses token strings. The
// existing numeric image exceptions use NumberString state instead. Imported
// attachment-valued text fields are represented by explicit rawRef/native.ref
// source; plain references in the same imported workflow still use text state.
func argumentUsesAttachmentCarrier(arg actionArgument, parameter parameterDefinition, index int) bool {
	if isNumericOrTextImageParamKey(parameter.key) {
		return true
	}
	value, ok := arg.value.(varValue)
	if !ok {
		return false
	}
	if value.descriptor == nil && value.getAs == "" && value.coerce == "" {
		if resolved, found := getVariableValue(fmt.Sprint(value.value)); found && resolved != nil {
			value = *resolved
		}
	}
	if value.descriptor == nil || !value.descriptor.ExplicitNative {
		return false
	}
	identifier := getFullActionIdentifier()
	return isAttachmentCapableTextParam(identifier, parameter.key) ||
		((identifier == "is.workflow.actions.text.split" || identifier == "is.workflow.actions.text.combine") && index == 1)
}
