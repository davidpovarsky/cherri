package main

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// analyzeVersionedForMobile is the exact response producer used by the C ABI.
func analyzeVersionedForMobile(docURI string, ver int, src string) protocol.AnalyzeResponse {
	if docURI == "" {
		docURI = "file:///mobile.cherri"
	}
	svc := sharedMobileService()
	diags, fp, activeVer := svc.AnalyzeVersioned(docURI, ver, src)

	wireDiags := make([]protocol.DiagnosticItem, 0)
	hasErrors := false
	for _, d := range diags {
		wireDiags = append(wireDiags, protocol.ToDiagnosticItem(d))
		if d.Severity == analysis.SeverityError {
			hasErrors = true
		}
	}

	resp := protocol.AnalyzeResponse{
		URI:               docURI,
		Version:           activeVer,
		LanguageVersion:   schema.LanguageVersion,
		SchemaFingerprint: fp,
		Diagnostics:       wireDiags,
		Valid:             !hasErrors,
	}

	return resp
}

// completeForMobile is the exact response producer used by the C export.
func completeForMobile(src string, line, column int) protocol.CompleteResponse {
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	items := svc.Complete(uri, line, column)

	wireItems := make([]protocol.CompletionItem, 0)
	for _, it := range items {
		wireItems = append(wireItems, protocol.CompletionItem{
			Label:         it.Label,
			Kind:          int(it.Kind),
			Detail:        it.Detail,
			Documentation: it.Documentation,
			InsertText:    it.InsertText,
		})
	}

	resp := protocol.CompleteResponse{
		URI:               uri,
		Version:           1,
		LanguageVersion:   schema.LanguageVersion,
		SchemaFingerprint: svc.SchemaFingerprint(),
		Items:             wireItems,
	}

	return resp
}

// hoverForMobile is the exact response producer used by the C export.
func hoverForMobile(src string, line, column int) protocol.HoverResponse {
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	res := svc.Hover(uri, line, column)
	if res == nil {
		return protocol.HoverResponse{OK: false}
	}
	rng := protocol.Range{
		Start: protocol.Position{Line: res.Span.Start.LSPLine(), Character: res.Span.Start.LSPCharacterColumn()},
		End:   protocol.Position{Line: res.Span.End.LSPLine(), Character: res.Span.End.LSPCharacterColumn()},
	}
	return protocol.HoverResponse{
		OK:       true,
		Contents: res.Contents,
		Range:    &rng,
	}
}

// definitionForMobile is the exact response producer used by the C export.
func definitionForMobile(src string, line, column int) protocol.DefinitionResponse {
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	span := svc.Definition(uri, line, column)
	if span == nil {
		return protocol.DefinitionResponse{OK: false}
	}
	rng := protocol.Range{
		Start: protocol.Position{Line: span.Start.LSPLine(), Character: span.Start.LSPCharacterColumn()},
		End:   protocol.Position{Line: span.End.LSPLine(), Character: span.End.LSPCharacterColumn()},
	}
	return protocol.DefinitionResponse{
		OK:    true,
		URI:   uri,
		Range: &rng,
	}
}

// signatureHelpForMobile is the exact response producer used by the C export.
func signatureHelpForMobile(src string, line, column int) protocol.SignatureHelpResponse {
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	sig := svc.SignatureHelp(uri, line, column)
	if sig == nil {
		return protocol.SignatureHelpResponse{OK: false}
	}
	return protocol.SignatureHelpResponse{
		OK:              true,
		Label:           sig.Label,
		Documentation:   sig.Documentation,
		Parameters:      sig.Parameters,
		ActiveParameter: sig.ActiveParameter,
	}
}

// renameForMobile is the exact response producer used by the C export.
func renameForMobile(src string, line, column int, targetName string) protocol.RenameResponse {
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	updated, edits, err := svc.Rename(uri, line, column, targetName)
	if err != nil {
		return protocol.RenameResponse{OK: false, Error: err.Error()}
	}
	wireEdits := make([]protocol.TextEditItem, len(edits))
	for i, ed := range edits {
		wireEdits[i] = protocol.TextEditItem{
			Range: protocol.Range{
				Start: protocol.Position{Line: ed.Span.Start.LSPLine(), Character: ed.Span.Start.LSPCharacterColumn()},
				End:   protocol.Position{Line: ed.Span.End.LSPLine(), Character: ed.Span.End.LSPCharacterColumn()},
			},
			NewText: ed.NewText,
		}
	}
	return protocol.RenameResponse{
		OK:     true,
		Source: updated,
		Edits:  wireEdits,
	}
}
