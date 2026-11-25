package service

import (
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// Source ownership comes from the actual AST for interpolation references and
// the production lexer for incomplete identifiers/labels/enum members. Quoted
// text and comment words cannot become identifier replacement ranges.
func completionReplacementSpan(doc *DocumentState, line, col int) source.Span {
	offset := byteOffsetAtLineCol(doc.File, line, col)
	span := doc.File.SpanForOffsets(offset, offset)
	if ref, _, _, found := semanticIdentifierAt(doc, line, col); found {
		return ref.span
	}
	lexer := syntax.NewLexer(doc.File)
	for {
		token := lexer.NextToken()
		if token.Type == syntax.TokenEOF || token.Span.Start.ByteOffset > offset {
			break
		}
		if token.Type == syntax.TokenIdent && token.Span.Start.ByteOffset <= offset && offset <= token.Span.End.ByteOffset {
			return token.Span
		}
	}
	return span
}

func attachCompletionEdits(doc *DocumentState, line, col int, items []CompletionItem) {
	if !validDocumentPosition(doc, line, col) {
		return
	}
	span := completionReplacementSpan(doc, line, col)
	labelSpan := span
	// A selected parameter replaces an existing label delimiter as one edit.
	// Only horizontal whitespace belongs to that delimiter, never a following
	// line, comment or argument value.
	if span.End.ByteOffset > span.Start.ByteOffset {
		text := doc.File.Content
		end := span.End.ByteOffset
		for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
			end++
		}
		if end < len(text) && text[end] == ':' {
			end++
			for end < len(text) && (text[end] == ' ' || text[end] == '\t') {
				end++
			}
			labelSpan = doc.File.SpanForOffsets(span.Start.ByteOffset, end)
		}
	}
	for i := range items {
		replacement := span
		if items[i].Kind == CompletionKindParameter {
			replacement = labelSpan
		}
		text := items[i].InsertText
		if text == "" {
			text = items[i].Label
		}
		items[i].TextEdit = &TextEdit{Span: replacement, NewText: text}
	}
}
