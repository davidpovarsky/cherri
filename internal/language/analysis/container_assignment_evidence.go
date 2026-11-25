package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Literal children have individual checked types. Use those exact AST nodes
// instead of a union that has forgotten which member was the empty literal.
// Opaque aliases and native outputs never receive this literal-only evidence.
func (a *Analyzer) literalContainerAssignmentEvidence(actual, expected types.Type, expr syntax.Expression) (bool, bool) {
	if actual == nil || expected == nil {
		return false, false
	}
	switch expr.(type) {
	case *syntax.ListExpr, *syntax.MapExpr:
	default:
		return false, false
	}
	if optional, okay := expected.(types.OptionalType); okay && actual.Kind() != types.KindOptional {
		return a.literalContainerAssignmentEvidence(actual, optional.Inner, expr)
	}
	if union, okay := expected.(types.UnionType); okay {
		handled := false
		for _, member := range union.Members {
			matches, selected := a.literalContainerAssignmentEvidence(actual, member, expr)
			handled = handled || selected
			if selected && matches {
				return true, true
			}
			if !selected && assignmentHasEvidence(actual, member, expr) {
				return true, true
			}
		}
		return false, handled
	}
	switch literal := expr.(type) {
	case *syntax.ListExpr:
		target, okay := expected.(types.ListType)
		if !okay {
			return false, false
		}
		if _, sourceList := actual.(types.ListType); !sourceList {
			return false, true
		}
		for _, element := range literal.Elements {
			if !a.optionalAssignmentHasEvidence(a.exprTypes[element], target.Element, element) {
				return false, true
			}
		}
		return true, true
	case *syntax.MapExpr:
		target, okay := expected.(types.MapType)
		if !okay {
			return false, false
		}
		if _, sourceMap := actual.(types.MapType); !sourceMap {
			return false, true
		}
		for _, entry := range literal.Entries {
			if !a.optionalAssignmentHasEvidence(a.exprTypes[entry.Key], target.Key, entry.Key) ||
				!a.optionalAssignmentHasEvidence(a.exprTypes[entry.Value], target.Value, entry.Value) {
				return false, true
			}
		}
		return true, true
	}
	return false, false
}
