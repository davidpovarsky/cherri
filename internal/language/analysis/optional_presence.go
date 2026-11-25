package analysis

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Provenance records whether lowering can obtain an independent presence flag,
// not whether the value is present. It follows bindings and checked function
// carriers and complete normal branch values; arbitrary native content has none.
func (a *Analyzer) expressionHasPresence(expr syntax.Expression, value types.Type, scope *Scope) bool {
	if types.DefinitelyPresent(value) {
		return true
	}
	switch node := expr.(type) {
	case *syntax.LiteralExpr:
		return node.Type == syntax.TokenNone
	case *syntax.IdentExpr:
		binding, found := scope.Lookup(node.Name)
		return found && binding.Initialized && binding.optionalPresence
	case *syntax.CallExpr:
		return a.exprPresence[expr]
	case *syntax.IfStmt, *syntax.MenuStmt:
		return a.controlValueHasPresence(expr)
	case *syntax.IndexExpr:
		if types.TaggedListElements(a.exprTypes[node.Target]) || types.TaggedMapValues(a.exprTypes[node.Target]) {
			return true
		}
		if a.loopResultLocalValues[node.Target] != nil {
			return true
		}
		if a.dictionaryIndexes[expr] {
			return true
		}
		element, known := syntax.StaticListElement(node)
		return known && types.DefinitelyPresent(a.exprTypes[element])
	}
	return false
}

func (a *Analyzer) requireCompilerPresence(expr syntax.Expression, context string) {
	if expr != nil && !a.exprPresence[expr] {
		a.error(CodeTargetValueShape, expr.NodeSpan(), fmt.Sprintf("current %s transport requires compiler-owned value presence; opaque native output or an untagged optional value cannot supply it", context))
	}
}

// The inferred Unknown? type alone cannot prove a destination's inner type.
// A source-owned, path-local None fact can satisfy an Optional destination
// without inventing any present inner value or weakening unknown-type rules.
func (a *Analyzer) optionalAssignmentHasEvidence(actual, expected types.Type, expr syntax.Expression) bool {
	return assignmentHasEvidence(actual, expected, expr) || (a.exprAbsent[expr] && types.MayBeAbsent(expected))
}
