package lower

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
)

// beginIntegralIndexGuard protects the native one-based item lookup from
// fractional dynamic indexes. index is already lowered: both the remainder
// and the existing bounds calculation consume that same value exactly once.
func (l *Lowerer) beginIntegralIndexGuard(index any) (group, end string) {
	switch index.(type) {
	case int64, uint64:
		return "", ""
	}
	remainder := l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID: remainder, AppleIdentifier: "is.workflow.actions.math", OutputUUID: remainder,
		Parameters: map[string]any{
			"WFMathOperation":           "…",
			"WFScientificMathOperation": "Modulus",
			"WFInput":                   index,
			"WFScientificMathOperand":   1.0,
		},
	})
	group, end = l.GenerateUUID(), l.GenerateUUID()
	l.emitIfStartTyped(group, &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: remainder},
		4, 0.0, nil, backend.OperandNumber, true)
	return group, end
}

// endIntegralIndexGuard retains the bounds branch's value as the complete If
// result. A fractional index yields Nothing without executing native lookup.
func (l *Lowerer) endIntegralIndexGuard(group, end, boundedResult string) string {
	if group == "" {
		return boundedResult
	}
	l.emitIfElse(group)
	l.emitAction(&ir.NativeActionNode{
		NodeID: l.GenerateUUID(), AppleIdentifier: "is.workflow.actions.nothing",
		Parameters: map[string]any{},
	})
	l.emitIfEnd(group, end, "")
	return end
}
