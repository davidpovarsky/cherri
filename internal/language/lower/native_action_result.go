package lower

import "github.com/electrikmilk/cherri/internal/language/ir"

// Production lowering uses the existing canonical session's consumption hook.
// The private handle follows normal binding/alias/reference lowering; it never
// replaces or renames an exact raw producer's native parameter identity.
func (l *Lowerer) captureNativeActionResult(node *ir.NativeActionNode, generatedUUID string, explicitUUID bool) (*ir.AttachmentToken, error) {
	resultUUID := generatedUUID
	if session, ok := l.session.(interface {
		BindNativeActionResult(referenceUUID, defaultUUID string, exact, explicitUUID bool, requestedUUID string) error
	}); ok {
		resultUUID = l.GenerateUUID()
		if err := session.BindNativeActionResult(resultUUID, generatedUUID, node.ExactRawParameters, explicitUUID, node.OutputUUID); err != nil {
			return nil, err
		}
		// Retain IR producer lookup for polymorphic/reference analysis.
		node.NodeID = resultUUID
	}
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: resultUUID}, nil
}
