package lower

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A native predicate table evaluates already-produced inputs. Lowering a call
// while assembling that table would execute it before the logical guard.
func (l *Lowerer) nativePredicateOperand(expr syntax.Expression) bool {
	switch e := expr.(type) {
	case *syntax.IdentExpr, *syntax.LiteralExpr, *syntax.EnumMemberExpr:
		return true
	case *syntax.MemberExpr:
		// Member/property/coercion syntax on a reference only extends its native
		// attachment. Never accept an action producer hidden under that syntax.
		return l.nativePredicateReference(e.Target)
	case *syntax.IndexExpr:
		// A literal dictionary/property key is an attachment aggrandizement.
		// Dynamic and numeric indexing may emit actions, so remain lazy.
		key, ok := e.Index.(*syntax.LiteralExpr)
		return ok && (key.Type == syntax.TokenString || key.Type == syntax.TokenRawString) && key.Value != "" &&
			!l.isArrayExpr(e.Target) && l.nativePredicateReference(e.Target)
	case *syntax.FStringExpr:
		// Native token strings are assembled without executing actions when
		// each interpolation is itself a reference or literal.
		for _, part := range e.Parts {
			if part.IsExpr && !l.nativePredicateOperand(part.Expr) {
				return false
			}
		}
		return true
	case *syntax.CallExpr:
		if _, ok := conditionHelperStringArg(e, "numeric"); ok {
			return true
		}
		if _, ok := conditionHelperStringArg(e, "condEnum"); ok {
			return true
		}
		member, ok := e.Callee.(*syntax.MemberExpr)
		if !ok {
			return false
		}
		ns, ok := member.Target.(*syntax.IdentExpr)
		if !ok || ns.Name != "native" {
			return false
		}
		switch member.Property {
		case "plist":
			// Imported exact metadata is an encoded literal. An arbitrary
			// expression here could execute a producer before the branch.
			lit, ok := e.PrimaryArg.(*syntax.LiteralExpr)
			return ok && (lit.Type == syntax.TokenString || lit.Type == syntax.TokenRawString) && len(e.NamedArgs) == 0
		case "ref":
			if e.PrimaryArg != nil && !l.nativePredicateOperand(e.PrimaryArg) {
				return false
			}
			for _, arg := range e.NamedArgs {
				if !l.nativePredicateOperand(arg.Value) {
					return false
				}
			}
			return true
		}
	}
	return false
}

func (l *Lowerer) nativePredicateReference(expr syntax.Expression) bool {
	switch e := expr.(type) {
	case *syntax.IdentExpr:
		return true
	case *syntax.MemberExpr, *syntax.IndexExpr:
		return l.nativePredicateOperand(expr)
	case *syntax.CallExpr:
		member, ok := e.Callee.(*syntax.MemberExpr)
		if !ok || member.Property != "ref" {
			return false
		}
		ns, ok := member.Target.(*syntax.IdentExpr)
		return ok && ns.Name == "native" && l.nativePredicateOperand(expr)
	}
	return false
}

func (l *Lowerer) nativePredicateClauses(clauses []syntax.Expression, allowPresence bool) bool {
	for _, expr := range clauses {
		negated := false
		if unary, ok := expr.(*syntax.UnaryExpr); ok && unary.Op == syntax.TokenBang {
			expr = unary.Operand
			negated = true
		}
		switch e := expr.(type) {
		case *syntax.BinaryExpr:
			if l.exactTextComparison(e) {
				return false
			}
			// These operators have no native inverse in lowerConditionClause.
			// Compute and negate their Boolean result in the lazy branch instead.
			if negated && (e.Op == syntax.TokenBeginsWith || e.Op == syntax.TokenEndsWith) {
				return false
			}
			switch e.Op {
			case syntax.TokenEqual, syntax.TokenNotEqual, syntax.TokenGreater,
				syntax.TokenGreaterEqual, syntax.TokenLess, syntax.TokenLessEqual,
				syntax.TokenContains, syntax.TokenBeginsWith, syntax.TokenEndsWith:
				if !l.nativePredicateOperand(e.Left) || !l.nativePredicateOperand(e.Right) {
					return false
				}
			default:
				return false
			}
		case *syntax.CallExpr:
			if allowPresence && l.nativePredicateReference(e) && (!l.isBooleanCondition(e) || l.importedPresenceOperand(e)) {
				continue
			}
			// Negated helper predicates produce a Boolean value; treating that
			// value as Has No Value would make false indistinguishable from true.
			if negated {
				return false
			}
			var id, ok = e.Callee.(*syntax.IdentExpr)
			if !ok || !l.nativePredicateOperand(e.PrimaryArg) {
				return false
			}
			switch id.Name {
			case "isToday", "isTrue", "isFalse":
			case "isBetween", "between":
				if len(e.NamedArgs) < 2 {
					return false
				}
				for _, arg := range e.NamedArgs {
					if !l.nativePredicateOperand(arg.Value) {
						return false
					}
				}
			default:
				return false
			}
		default:
			if !allowPresence || !l.nativePredicateOperand(expr) || (l.isBooleanCondition(expr) && !l.importedPresenceOperand(expr)) {
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

// Import source retains native presence semantics for an original imported
// symbol or its native reference aggrandizements. Header provenance alone must
// not change new authored Bool bindings or
// logical expressions into HasAnyValue. An edited condition on an imported Bool
// can express value intent unambiguously as "value == true" or isTrue(value).
func (l *Lowerer) importedPresenceOperand(expr syntax.Expression) bool {
	if !l.hasImportProvenance {
		return false
	}
	if unary, ok := expr.(*syntax.UnaryExpr); ok && unary.Op == syntax.TokenBang {
		expr = unary.Operand
	}
	switch e := expr.(type) {
	case *syntax.IdentExpr:
		return l.provenanceSymbols[e.Name]
	case *syntax.MemberExpr:
		return l.nativePredicateReference(e) && l.importedPresenceOperand(e.Target)
	case *syntax.IndexExpr:
		return l.nativePredicateReference(e) && l.importedPresenceOperand(e.Target)
	case *syntax.CallExpr:
		return l.nativePredicateReference(e) && e.PrimaryArg != nil && l.importedPresenceOperand(e.PrimaryArg)
	}
	return false
}
