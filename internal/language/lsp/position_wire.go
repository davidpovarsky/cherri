package lsp

import (
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
)

func wireRange(span source.Span) map[string]any {
	return map[string]any{
		"start": map[string]int{"line": span.Start.LSPLine(), "character": span.Start.LSPCharacterColumn()},
		"end":   map[string]int{"line": span.End.LSPLine(), "character": span.End.LSPCharacterColumn()},
	}
}

// The shared service's enum is intentionally independent of the LSP wire enum.
// Values follow the official LSP CompletionItemKind, not the Go iota ordinal.
func completionWireItems(items []service.CompletionItem) []map[string]any {
	wire := make([]map[string]any, 0, len(items))
	for _, item := range items {
		kind := 1 // Text, for future service kinds the LSP does not know.
		switch item.Kind {
		case service.CompletionKindFunction:
			kind = 3
		case service.CompletionKindVariable:
			kind = 6
		case service.CompletionKindParameter:
			kind = 5
		case service.CompletionKindEnum:
			kind = 20
		case service.CompletionKindKeyword:
			kind = 14
		case service.CompletionKindSnippet:
			kind = 15
		}
		value := map[string]any{"label": item.Label, "kind": kind}
		if item.Detail != "" {
			value["detail"] = item.Detail
		}
		if item.Documentation != "" {
			value["documentation"] = item.Documentation
		}
		if item.InsertText != "" {
			value["insertText"] = item.InsertText
		}
		if item.TextEdit != nil {
			value["textEdit"] = map[string]any{
				"range": wireRange(item.TextEdit.Span), "newText": item.TextEdit.NewText,
			}
		}
		wire = append(wire, value)
	}
	return wire
}
