package lower

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A native predicate table evaluates already-produced inputs. Lowering a call
// while assembling that table would execute it before the logical guard.
func nativePredicateOperand(expr syntax.Expression) bool {
	switch e := expr.(type) {
	case *syntax.IdentExpr, *syntax.LiteralExpr, *syntax.EnumMemberExpr:
		return true
	case *syntax.CallExpr:
		if _, ok := conditionHelperStringArg(e, "numeric"); ok {
			return true
		}
		if _, ok := conditionHelperStringArg(e, "condEnum"); ok {
			return true
		}
	}
	return false
}

func nativePredicateClauses(clauses []syntax.Expression, allowPresence bool) bool {
	for _, expr := range clauses {
		if unary, ok := expr.(*syntax.UnaryExpr); ok && unary.Op == syntax.TokenBang {
			expr = unary.Operand
		}
		switch e := expr.(type) {
		case *syntax.BinaryExpr:
			switch e.Op {
			case syntax.TokenEqual, syntax.TokenNotEqual, syntax.TokenGreater,
				syntax.TokenGreaterEqual, syntax.TokenLess, syntax.TokenLessEqual,
				syntax.TokenContains, syntax.TokenBeginsWith, syntax.TokenEndsWith:
				if !nativePredicateOperand(e.Left) || !nativePredicateOperand(e.Right) {
					return false
				}
			default:
				return false
			}
		case *syntax.CallExpr:
			var id, ok = e.Callee.(*syntax.IdentExpr)
			if !ok || !nativePredicateOperand(e.PrimaryArg) {
				return false
			}
			switch id.Name {
			case "isToday", "isTrue", "isFalse":
			case "isBetween", "between":
				if len(e.NamedArgs) < 2 {
					return false
				}
				for _, arg := range e.NamedArgs {
					if !nativePredicateOperand(arg.Value) {
						return false
					}
				}
			default:
				return false
			}
		default:
			if !allowPresence || !nativePredicateOperand(expr) {
				return false
			}
		}
	}
	return true
}

func (l *Lowerer) isBooleanCondition(expr syntax.Expression) bool {
	if unary, ok := expr.(*syntax.UnaryExpr); ok && unary.Op == syntax.TokenBang {
		return l.isBooleanCondition(unary.Operand)
	}
	if t := l.exprTypes[expr]; t != nil && t.Equals(types.Bool) {
		return true
	}
	switch e := expr.(type) {
	case *syntax.LiteralExpr:
		return e.Type == syntax.TokenTrue || e.Type == syntax.TokenFalse
	case *syntax.IdentExpr:
		var ref, ok = l.bindings[e.Name]
		return ok && ref.Type != nil && ref.Type.Equals(types.Bool)
	case *syntax.BinaryExpr:
		return e.Op == syntax.TokenAnd || e.Op == syntax.TokenOr
	}
	return false
}
