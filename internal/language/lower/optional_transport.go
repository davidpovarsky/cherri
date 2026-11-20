package lower

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// Keep checked lowering fail-closed as well as editor analysis. This helper is
// called only by ordinary value routes, never opaque native parameter metadata.
func (l *Lowerer) requirePresentTransport(expr syntax.Expression, context string) error {
	if expr == nil {
		return nil
	}
	absent := false
	if literal, ok := expr.(*syntax.LiteralExpr); ok {
		absent = literal.Type == syntax.TokenNone
	}
	inferred := l.exprTypes[expr]
	if inferred == nil {
		if identifier, ok := expr.(*syntax.IdentExpr); ok {
			if binding, found := l.bindings[identifier.Name]; found {
				inferred = binding.Type
			}
		}
	}
	if absent || types.MayBeAbsent(inferred) {
		return fmt.Errorf("E_TARGET_VALUE_SHAPE: current %s transport cannot preserve optional absence at %s; use a proven present value", context, expr.NodeSpan())
	}
	return nil
}
