package main

import "fmt"

// Imported Repeat Results denote the existing native output sequence. They do
// not promise one nested List per iteration. Keep the V2 loop in statement form
// and bind a consumed result through an explicit native attachment afterward.
// A native.ref binding emits no action, so both control markers and the last
// native body action retain their original runtime cardinality and wire shape.
func (ctx *semanticImportContext) nativeLoopResultBindings(endRec NativeActionRecord) (legacyPrefix, v2End string, ok bool) {
	legacyPrefix, _, ok = ctx.controlEndAssignPrefix(endRec)
	if !ok {
		return "", "", false
	}
	v2End = "}"
	if legacyPrefix == "" {
		return legacyPrefix, v2End, true
	}
	symbol, exists := ctx.UUIDToSymbol[endRec.UUID]
	if !exists || symbol == "" {
		return "", "", false
	}
	args := `"ActionOutput", uuid: ` + formatCherriDoubleQuotedString(endRec.UUID)
	meta := ctx.Symbols[symbol]
	if hasName, _ := meta["HasOutputName"].(bool); hasName {
		name, _ := meta["OutputName"].(string)
		args += ", outputName: " + formatCherriDoubleQuotedString(name)
	}
	return legacyPrefix, fmt.Sprintf("}\nlet %s = native.ref(%s)", symbol, args), true
}
