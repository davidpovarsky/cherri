package analysis

import (
	"math"
	"strconv"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A duration here is a direct native parameter constructor, not a stored
// Content Graph value. Prove only the exact scalar form that the existing qty
// lowerer routes into WFDuration; bindings and other families stay opaque.
func (a *Analyzer) isKnownDateDurationComparison(left types.Type, right syntax.Expression, operator syntax.TokenType, scope *Scope) bool {
	if operator != syntax.TokenLess && operator != syntax.TokenGreater {
		return false
	}
	date := unwrapOptionalType(left)
	if date == nil || !date.Equals(types.Date) {
		return false
	}
	call, ok := right.(*syntax.CallExpr)
	if !ok {
		return false
	}
	name, ok := call.Callee.(*syntax.IdentExpr)
	if !ok || name.Name != "qty" || call.Block != nil {
		return false
	}
	if _, shadowed := scope.Lookup(name.Name); shadowed {
		return false
	}
	arguments := make([]syntax.Expression, 0, 1+len(call.NamedArgs))
	if call.PrimaryArg != nil {
		arguments = append(arguments, call.PrimaryArg)
	}
	for _, argument := range call.NamedArgs {
		arguments = append(arguments, argument.Value)
	}
	if len(arguments) != 2 || a.registry == nil {
		return false
	}
	magnitude, ok := arguments[0].(*syntax.LiteralExpr)
	if !ok || magnitude.Type != syntax.TokenNumber {
		return false
	}
	number, err := strconv.ParseFloat(magnitude.Value, 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return false
	}
	unit, ok := arguments[1].(*syntax.LiteralExpr)
	if !ok || (unit.Type != syntax.TokenString && unit.Type != syntax.TokenRawString) {
		return false
	}
	// Reuse the canonical action catalog's exact date-unit domain.
	units, found := a.registry.LookupEnum("dateUnit")
	if !found {
		return false
	}
	for _, known := range units {
		if unit.Value == known {
			return true
		}
	}
	return false
}
