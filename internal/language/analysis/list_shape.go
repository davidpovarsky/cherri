package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A Dictionary field's literal array is serialized inline by the semantic
// Dictionary codec. Calls and value controls still evaluate ordinary runtime
// Lists; they must not inherit that literal-data exception.
func (a *Analyzer) analyzeDictionaryValue(expr syntax.Expression, scope *Scope) types.Type {
	previous := a.inlineDictionaryArray
	_, a.inlineDictionaryArray = expr.(*syntax.ListExpr)
	defer func() { a.inlineDictionaryArray = previous }()
	return a.analyzeExpression(expr, scope)
}

func (a *Analyzer) isNativeListElement(expr syntax.Expression, value types.Type, scope *Scope) bool {
	if mayHaveListCardinality(value) {
		return true
	}
	// A broad annotation changes the assignment contract, but an immutable
	// binding retains the actual initializer's List shape. This fact is used
	// only by the target diagnostic; it does not narrow the language type.
	if value == nil || value.Kind() == types.KindUnknown || value.Kind() == types.KindAnyContent {
		if name, ok := expr.(*syntax.IdentExpr); ok {
			if symbol, found := scope.Lookup(name.Name); found {
				return symbol.Initialized && symbol.nativeListValue
			}
		}
	}
	return false
}
