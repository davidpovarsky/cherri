package main

// An imported native operand proves its attachment shape, not a scalar result
// type. Retain that explicit boundary through the existing native reference
// renderer so ordinary authored scalar checks remain strict.
func (ctx *semanticImportContext) preserveNativeOperandReference(value any, pair semanticExprPair) semanticExprPair {
	attachment, ok := value.(map[string]any)
	if !ok {
		return pair
	}
	if attachment["WFSerializationType"] == "WFTextTokenAttachment" {
		attachment, ok = attachment["Value"].(map[string]any)
		if !ok {
			return pair
		}
	}
	if reference, ok := ctx.renderSymbolicRawRefPair(attachment, false); ok {
		pair = reference
	}
	return pair
}
