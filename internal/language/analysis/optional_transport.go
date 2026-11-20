package analysis

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// The current ordinary value envelope has no independent absence flag. Do not
// silently turn a supplied None into empty Text or a missing container element.
// This is a compiler transport limitation, not a claim about native capability.
// Raw native parameter metadata retains its separate authored omission rules.
func (a *Analyzer) requirePresentTransport(expr syntax.Expression, value types.Type, context string) {
	if !a.rawNativeDataContext && types.MayBeAbsent(value) {
		a.error(CodeTargetValueShape, expr.NodeSpan(), fmt.Sprintf("current %s transport cannot preserve optional absence; use a proven present value", context))
	}
}
