package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// variantOutputType refines only a native action whose checked-in Apple
// contract identifies both input-dependent element type and literal selection
// mode. Other actions retain their declared (possibly Unknown) output category.
func (a *Analyzer) variantOutputType(action *schema.ActionSchema, arguments map[string]syntax.Expression) (types.Type, bool) {
	if action.AppleIdentifier != "is.workflow.actions.choosefromlist" {
		return nil, false
	}
	// Apple WFChooseFromListAction: InputTypeDeterminesOutputType=true.
	// Non-list/opaque input does not establish the selected element's type.
	var element types.Type = types.Unknown
	if input, ok := a.exprTypes[arguments["WFInput"]].(types.ListType); ok {
		element = input.Element
	}
	many := types.NewList(element)
	mode := arguments["WFChooseFromListActionSelectMultiple"]
	if mode == nil {
		// The native WFSwitchParameter's DefaultValue is false.
		return element, true
	}
	if multiple, known := literalBoolValue(mode); known {
		if multiple {
			return many, true
		}
		return element, true
	}
	// A runtime value, including an immutable let, cannot prove either mode.
	return types.NewUnion(element, many), true
}

func mayHaveListCardinality(value types.Type) bool {
	switch value := value.(type) {
	case types.ListType:
		return true
	case types.OptionalType:
		return mayHaveListCardinality(value.Inner)
	case types.UnionType:
		for _, member := range value.Members {
			if mayHaveListCardinality(member) {
				return true
			}
		}
	}
	return false
}
