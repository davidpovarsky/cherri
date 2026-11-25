package main

import (
	"fmt"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// Emit the callback only in the already validated page region. Entry is the
// actual native Dictionary, even on an empty-items page; no helper follows the
// entry before authored body effects. Nothing is the explicit Void boundary.
func (b *paginateExpansion) emitPageCallback(page WFTextTokenAttachment) {
	if b.callback == nil || b.err != nil {
		return
	}
	if b.callback.InputName == "" || b.callback.EmitAction == nil || b.callback.Body == nil {
		b.err = fmt.Errorf("E_PAGINATE_CALLBACK: incomplete page callback contract")
		return
	}
	id := strings.ToUpper(nextGeneratedUUID("paginate"))
	b.add("getvariable", map[string]any{
		"WFVariable": page, "UUID": id, "CustomOutputName": b.callback.InputName,
	})
	if b.err != nil {
		return
	}
	b.err = b.callback.Body(backend.Reference{
		Kind: backend.RefActionResult, ProducerID: id, ProducerName: b.callback.InputName,
	})
	if b.err != nil {
		return
	}
	b.add("nothing", map[string]any{})
}
