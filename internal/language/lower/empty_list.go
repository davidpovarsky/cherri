package lower

import "github.com/electrikmilk/cherri/internal/language/ir"

func isEmptyRuntimeList(value interface{}) bool {
	items, ok := value.([]interface{})
	return ok && len(items) == 0
}

// Empty native List action rows were observed as one item in the signed
// differential fixture. Reuse the existing Legacy clearing mechanism instead:
// Nothing prevents a prior action's output from becoming implicit Set input.
// This helper is only for authored runtime values, never inline native metadata.
func (l *Lowerer) emitClearedVariable(variableName string) {
	bindingName := l.detachBindingName()
	defer func() { l.currentBindingName = bindingName }()
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.nothing",
		Parameters:      map[string]interface{}{},
	})
	l.emitAction(&ir.NativeActionNode{
		NodeID:          l.GenerateUUID(),
		AppleIdentifier: "is.workflow.actions.setvariable",
		Parameters:      map[string]interface{}{"WFVariableName": variableName},
	})
}

// Immutable/temporary empty values retain a concrete producer UUID through the
// existing Get Variable bridge. Each private variable is cleared once and is not
// exposed as a source binding. The UUID's hyphens cannot collide with an authored
// identifier; the existing lexical allocator also checks imported variable names.
func (l *Lowerer) materializeEmptyList(uuid, outputName string) *ir.AttachmentToken {
	variableName := l.freshLexicalName("emptyList_" + uuid)
	l.emitClearedVariable(variableName)
	l.emitAction(&ir.NativeActionNode{
		NodeID:          uuid,
		AppleIdentifier: "is.workflow.actions.getvariable",
		OutputUUID:      uuid,
		OutputName:      outputName,
		Parameters: map[string]interface{}{
			"WFVariable": &ir.AttachmentToken{Type: "Variable", OutputName: variableName},
		},
	})
	return &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: uuid, OutputName: outputName}
}
