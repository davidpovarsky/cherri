package lower

import (
	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func isNativeConditionPredicate(call *syntax.CallExpr) bool {
	id, ok := call.Callee.(*syntax.IdentExpr)
	if !ok {
		return false
	}
	switch id.Name {
	case "isTrue", "isFalse", "isToday", "isBetween", "between":
		return true
	}
	return false
}

// A predicate used as a value must produce a Boolean value. Reuse the same
// native condition emitter as statement predicates, with explicit true/false
// yields. In a lazy expression this runs only inside the selected branch.
func (l *Lowerer) lowerNativePredicateValue(call *syntax.CallExpr) (*ir.AttachmentToken, error) {
	yield := func(value bool) *syntax.BlockStmt {
		token, text := syntax.TokenFalse, "false"
		if value {
			token, text = syntax.TokenTrue, "true"
		}
		return &syntax.BlockStmt{Span: call.Span, Statements: []syntax.Statement{
			&syntax.YieldStmt{Span: call.Span, Value: &syntax.LiteralExpr{Span: call.Span, Type: token, Value: text}},
		}}
	}
	return l.lowerIfWithOutput(&syntax.IfStmt{
		Span: call.Span, Condition: call, ThenBlock: yield(true), ElseBlock: yield(false),
	}, true)
}

// Authored Bool helpers have the same value comparison as == true / != true.
// Imported predicates and explicit native Boolean coercions retain their unary
// native representation; neither is a compiler-owned numeric Bool carrier.
func (l *Lowerer) authoredBooleanPredicateValue(expr syntax.Expression, input *ir.AttachmentToken) bool {
	if input == nil || !l.isBooleanCondition(expr) || l.importedPresenceOperand(expr) {
		return false
	}
	for _, ag := range input.Aggrandizements {
		if ag["Type"] == "WFCoercionVariableAggrandizement" && ag["CoercionItemClass"] == "WFBooleanContentItem" {
			return false
		}
	}
	return true
}
