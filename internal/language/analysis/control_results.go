package analysis

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// controlExpressionResultType joins the result that can reach each native
// control boundary. The first encountered yield is not proof of another
// branch's output, and an early yield can be followed by a different action.
func (a *Analyzer) controlExpressionResultType(expr syntax.Expression) types.Type {
	switch control := expr.(type) {
	case *syntax.IfStmt, *syntax.MenuStmt:
		return joinControlValueEvidence(a.normalControlValueEvidence(expr))
	case *syntax.ForStmt:
		return types.NewList(a.controlBranchResultType(control.BodyBlock))
	case *syntax.RepeatStmt:
		return types.NewList(a.controlBranchResultType(control.BodyBlock))

	}
	return types.AnyContent
}

// Preserve the existing untyped native-last-action fallback. Only a final
// explicit yield or a final nested control expression establishes a static
// result contract; missing/overwritten yields remain arbitrary content.
func (a *Analyzer) controlBranchResultType(body *syntax.BlockStmt) types.Type {
	result := syntax.BlockResultExpression(body)
	if result == nil {
		return types.AnyContent
	}
	if _, explicit := body.Statements[len(body.Statements)-1].(*syntax.YieldStmt); explicit {
		if inferred := a.exprTypes[result]; inferred != nil {
			return inferred
		}
		return types.AnyContent
	}
	switch result.(type) {
	case *syntax.IfStmt, *syntax.ForStmt, *syntax.RepeatStmt, *syntax.MenuStmt:
		return a.controlExpressionResultType(result)
	default:
		return types.AnyContent
	}
}

// A return terminates the current invocation and does not contribute a value
// to an If/Menu join. Retain the existing loop result policy separately.
func (a *Analyzer) normalControlBranchResultType(body *syntax.BlockStmt) types.Type {
	if !statementMayContinue(body) {
		return nil
	}
	return a.controlBranchResultType(body)
}
