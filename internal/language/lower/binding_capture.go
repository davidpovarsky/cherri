package lower

import "github.com/electrikmilk/cherri/internal/language/ir"

// Native Clipboard attachments are read by their eventual consumer. A let
// binding needs a producer at its declaration so later writes cannot replace it.
// Native execution shows Get Variable does not read a Clipboard attachment; use
// the canonical parameterless Get Clipboard action and retain the captured edge.
func (l *Lowerer) captureClipboardBinding(name string, reference *ir.AttachmentToken) BindingRef {
	var uuid = l.GenerateUUID()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          uuid,
		AppleIdentifier: "is.workflow.actions.getclipboard",
		OutputUUID:      uuid,
		OutputName:      name,
		Parameters:      map[string]interface{}{},
	})
	return BindingRef{IsVariable: false, OutputUUID: uuid, OutputName: l.effectiveOutputName(name), Aggrandizements: reference.Aggrandizements}
}
