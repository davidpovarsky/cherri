package analysis

import "github.com/electrikmilk/cherri/internal/language/syntax"

// Explicit yields establish a value contract for every normal path. The
// all-implicit native-last-action path remains deliberately untyped.
func (a *Analyzer) requireIfYieldResults(control *syntax.IfStmt) {
	a.requireNormalYieldResult(control.ThenBlock)
	switch alternate := control.ElseBlock.(type) {
	case *syntax.BlockStmt:
		a.requireNormalYieldResult(alternate)
	case *syntax.IfStmt:
		a.requireIfYieldResults(alternate)
	default:
		a.error(CodeYieldContext, control.Span, "value-producing 'if' expression requires 'else'")
	}
}

func (a *Analyzer) requireMenuYieldResults(control *syntax.MenuStmt) {
	if len(control.Cases) == 0 {
		a.error(CodeYieldContext, control.Span, "value-producing 'menu' requires a value-producing case")
		return
	}
	for _, item := range control.Cases {
		a.requireNormalYieldResult(item.BodyBlock)
	}
}

func (a *Analyzer) requireNormalYieldResult(body *syntax.BlockStmt) {
	if body == nil || !statementMayContinue(body) {
		return
	}
	if len(body.Statements) != 0 {
		if _, explicit := body.Statements[len(body.Statements)-1].(*syntax.YieldStmt); explicit {
			return
		}
	}
	switch result := syntax.BlockResultExpression(body).(type) {
	case *syntax.IfStmt:
		a.requireIfYieldResults(result)
	case *syntax.MenuStmt:
		a.requireMenuYieldResults(result)
	case *syntax.ForStmt, *syntax.RepeatStmt:
		// A nested value loop has its existing Repeat Results boundary.
		// Its element/cardinality contract is checked independently.
		return
	default:
		a.error(CodeYieldContext, body.Span, "each normal value branch must end in yield or a value-producing control expression")
	}
}
