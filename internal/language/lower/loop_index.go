package lower

import "github.com/electrikmilk/cherri/internal/language/ir"

// Apple's native Repeat Index is one-based. Modern source loop bindings expose
// a zero-based Number, captured inside the owning loop before its body runs.
// Raw RepeatIndex references and the preserved Legacy repeat spelling bypass
// this conversion deliberately.
func (l *Lowerer) zeroBasedLoopIndex(nativeName string) *ir.AttachmentToken {
	var uuid = l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          uuid,
		AppleIdentifier: "is.workflow.actions.math",
		OutputUUID:      uuid,
		Parameters: map[string]interface{}{
			"WFInput":         &ir.AttachmentToken{Type: "Variable", OutputName: nativeName},
			"WFMathOperation": "-",
			"WFMathOperand":   1.0,
		},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid}
}
