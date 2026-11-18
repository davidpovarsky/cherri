package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Authored annotations and function contracts assert type knowledge. Native
// attachment compatibility is a separate parameter contract and cannot supply
// that knowledge merely because the same value can be sent to a native action.
func assignmentHasEvidence(actual, expected types.Type, expression syntax.Expression) bool {
	if expected == nil || actual == nil {
		return false
	}
	if union, ok := expected.(types.UnionType); ok {
		for _, member := range union.Members {
			if assignmentHasEvidence(actual, member, expression) {
				return true
			}
		}
		return false
	}
	if literal, ok := expression.(*syntax.LiteralExpr); ok && literal.Type == syntax.TokenNone {
		return expected.Kind() == types.KindOptional
	}
	if optional, ok := expected.(types.OptionalType); ok && actual.Kind() != types.KindOptional {
		return assignmentHasEvidence(actual, optional.Inner, expression)
	}
	// An authored empty literal has no element/value that could contradict its
	// destination. An opaque List<Unknown> or Map<Text,Unknown> is not empty proof.
	if list, ok := expression.(*syntax.ListExpr); ok && len(list.Elements) == 0 {
		if _, isList := expected.(types.ListType); isList {
			return true
		}
	}
	if dictionary, ok := expression.(*syntax.MapExpr); ok && len(dictionary.Entries) == 0 {
		if _, isMap := expected.(types.MapType); isMap {
			return true
		}
	}
	return types.HasAssignmentEvidence(actual, expected)
}

func isVoidValue(t types.Type) bool { return t != nil && t.Kind() == types.KindVoid }
