package migrate

import "github.com/electrikmilk/cherri/internal/language/syntax"

// preserveMainOutputMode annotates a parsed entry once, after the shared mode
// validator has accepted it. Existing explicit or Legacy intent is preserved;
// comments and literal contents cannot impersonate a metadata declaration.
func preserveMainOutputMode(program *syntax.Program) {
	var firstHeader *syntax.ShortcutDecl
	for _, declaration := range program.Declarations {
		header, ok := declaration.(*syntax.ShortcutDecl)
		if !ok || header == nil {
			continue
		}
		if firstHeader == nil {
			firstHeader = header
		}
		if header.Metadata != nil {
			for _, field := range header.Metadata.Fields {
				if field.Name == "outputMode" {
					return
				}
			}
		}
	}
	if firstHeader == nil {
		firstHeader = &syntax.ShortcutDecl{Span: program.Span}
		program.Declarations = append([]syntax.Declaration{firstHeader}, program.Declarations...)
	}
	if firstHeader.Metadata == nil {
		firstHeader.Metadata = &syntax.RecordExpr{Span: firstHeader.Span}
	}
	firstHeader.Metadata.Fields = append(firstHeader.Metadata.Fields, syntax.RecordFieldExpr{
		Name: "outputMode",
		Value: &syntax.LiteralExpr{Span: firstHeader.Span, Type: syntax.TokenString,
			Value: string(syntax.MainOutputLegacy)},
	})
}
