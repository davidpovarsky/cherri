package main

// Reference tokens carry identity rather than the definition's numeric type.
// Resolve copies as upstream does, while bounding cycles in mutable aliases.
func legacyCompoundReferenceType(reference varValue) tokenType {
	var seen = make(map[string]bool)
	for {
		if reference.descriptor != nil {
			return Variable
		}
		if reference.coerce != "" {
			return tokenType(reference.coerce)
		}
		var name, named = reference.value.(string)
		if !named || seen[name] {
			return Variable
		}
		seen[name] = true
		var definition, found = getVariableValue(name)
		if !found {
			return Variable
		}
		switch definition.valueType {
		case Expression:
			return Integer
		case Action:
			if value, ok := definition.value.(action); ok && value.def != nil {
				return value.def.outputType
			}
			return Variable
		case Variable:
			if copied, ok := definition.value.(varValue); ok {
				reference = copied
				continue
			}
			return Variable
		default:
			return definition.valueType
		}
	}
}
