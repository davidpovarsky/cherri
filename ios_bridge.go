//go:build ios && cgo

/*
 * Copyright (c) Cherri
 */

package main

/*
#include <stdlib.h>
*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"unsafe"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// CherriCompile compiles Cherri source in-process with default v2 mode and returns a JSON response.
//
//export CherriCompile
func CherriCompile(source *C.char, requestedName *C.char) *C.char {
	return encodeMobileResponse(compileForMobileMode(C.GoString(source), C.GoString(requestedName), string(MobileCompileModeV2), false))
}

// CherriCompileMode compiles Cherri source in-process with explicit mode ("v2", "legacy", "auto").
//
//export CherriCompileMode
func CherriCompileMode(source *C.char, requestedName *C.char, mode *C.char) *C.char {
	return encodeMobileResponse(compileForMobileMode(C.GoString(source), C.GoString(requestedName), C.GoString(mode), false))
}

// CherriCompileSigned compiles Cherri source with default v2 mode and signs it with HubSign.
//
//export CherriCompileSigned
func CherriCompileSigned(source *C.char, requestedName *C.char) *C.char {
	return encodeMobileResponse(compileForMobileMode(C.GoString(source), C.GoString(requestedName), string(MobileCompileModeV2), true))
}

// CherriCompileSignedMode compiles Cherri source with explicit mode and signs it with HubSign.
//
//export CherriCompileSignedMode
func CherriCompileSignedMode(source *C.char, requestedName *C.char, mode *C.char) *C.char {
	return encodeMobileResponse(compileForMobileMode(C.GoString(source), C.GoString(requestedName), C.GoString(mode), true))
}

// CherriDecompilePlist decompiles Shortcut plist data using Cherri's decompiler.
//
//export CherriDecompilePlist
func CherriDecompilePlist(plistBase64 *C.char, requestedName *C.char) *C.char {
	encoded := C.GoString(plistBase64)
	plistBytes, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return encodeMobileResponse(mobileCompileResponse{OK: false, Error: "Invalid base64 Shortcut data."})
	}
	return encodeMobileResponse(decompileForMobile(plistBytes, C.GoString(requestedName)))
}

// CherriActionCatalog returns the action definitions currently available to the compiler.
//
//export CherriActionCatalog
func CherriActionCatalog() *C.char {
	mobileCompileMu.Lock()
	defer mobileCompileMu.Unlock()

	catalog := currentMobileActionCatalog()
	encoded, err := json.Marshal(mobileActionCatalogResponse{OK: true, Actions: catalog})
	if err != nil {
		encoded, _ = json.Marshal(mobileActionCatalogResponse{OK: false, Error: err.Error()})
	}
	return C.CString(string(encoded))
}

// CherriAnalyze parses and analyzes Cherri v2 source and returns JSON diagnostics.
//
//export CherriAnalyze
func CherriAnalyze(source *C.char) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	diags, fp := svc.Analyze(uri)

	wireDiags := make([]protocol.DiagnosticItem, 0)
	hasErrors := false
	for _, d := range diags {
		wireDiags = append(wireDiags, protocol.ToDiagnosticItem(d))
		if d.Severity == analysis.SeverityError {
			hasErrors = true
		}
	}

	resp := protocol.AnalyzeResponse{
		URI:               uri,
		Version:           1,
		LanguageVersion:   schema.LanguageVersion,
		SchemaFingerprint: fp,
		Diagnostics:       wireDiags,
		Valid:             !hasErrors,
	}

	encoded, _ := json.Marshal(resp)
	return C.CString(string(encoded))
}

// CherriAnalyzeVersioned updates a versioned document in the shared mobile service
// (ignoring stale lower-version updates) and returns the active version's diagnostics.
//
//export CherriAnalyzeVersioned
func CherriAnalyzeVersioned(uri *C.char, version C.int, source *C.char) *C.char {
	docURI := C.GoString(uri)
	if docURI == "" {
		docURI = "file:///mobile.cherri"
	}
	ver := int(version)
	src := C.GoString(source)
	svc := sharedMobileService()
	svc.ChangeDocument(docURI, ver, src)
	diags, fp := svc.Analyze(docURI)
	activeVer := svc.DocumentVersion(docURI)

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

	encoded, _ := json.Marshal(resp)
	return C.CString(string(encoded))
}

// CherriComplete computes contextual completions at (line, column) in source.
//
//export CherriComplete
func CherriComplete(source *C.char, line C.int, column C.int) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	items := svc.Complete(uri, int(line), int(column))

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

	encoded, _ := json.Marshal(resp)
	return C.CString(string(encoded))
}

// CherriHover returns hover documentation and range at (line, column) in source.
//
//export CherriHover
func CherriHover(source *C.char, line C.int, column C.int) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	res := svc.Hover(uri, int(line), int(column))
	if res == nil {
		encoded, _ := json.Marshal(protocol.HoverResponse{OK: false})
		return C.CString(string(encoded))
	}
	rng := protocol.Range{
		Start: protocol.Position{Line: res.Span.Start.LSPLine(), Character: res.Span.Start.LSPCharacterColumn()},
		End:   protocol.Position{Line: res.Span.End.LSPLine(), Character: res.Span.End.LSPCharacterColumn()},
	}
	encoded, _ := json.Marshal(protocol.HoverResponse{
		OK:       true,
		Contents: res.Contents,
		Range:    &rng,
	})
	return C.CString(string(encoded))
}

// CherriDefinition returns the declaration span for the symbol at (line, column).
//
//export CherriDefinition
func CherriDefinition(source *C.char, line C.int, column C.int) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	span := svc.Definition(uri, int(line), int(column))
	if span == nil {
		encoded, _ := json.Marshal(protocol.DefinitionResponse{OK: false})
		return C.CString(string(encoded))
	}
	rng := protocol.Range{
		Start: protocol.Position{Line: span.Start.LSPLine(), Character: span.Start.LSPCharacterColumn()},
		End:   protocol.Position{Line: span.End.LSPLine(), Character: span.End.LSPCharacterColumn()},
	}
	encoded, _ := json.Marshal(protocol.DefinitionResponse{
		OK:    true,
		URI:   uri,
		Range: &rng,
	})
	return C.CString(string(encoded))
}

// CherriSignatureHelp returns signature help at (line, column) in source.
//
//export CherriSignatureHelp
func CherriSignatureHelp(source *C.char, line C.int, column C.int) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	sig := svc.SignatureHelp(uri, int(line), int(column))
	if sig == nil {
		encoded, _ := json.Marshal(protocol.SignatureHelpResponse{OK: false})
		return C.CString(string(encoded))
	}
	encoded, _ := json.Marshal(protocol.SignatureHelpResponse{
		OK:              true,
		Label:           sig.Label,
		Documentation:   sig.Documentation,
		Parameters:      sig.Parameters,
		ActiveParameter: sig.ActiveParameter,
	})
	return C.CString(string(encoded))
}

// CherriRename performs a scoped symbol rename at (line, column) in source.
//
//export CherriRename
func CherriRename(source *C.char, line C.int, column C.int, newName *C.char) *C.char {
	src := C.GoString(source)
	targetName := C.GoString(newName)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	updated, edits, err := svc.Rename(uri, int(line), int(column), targetName)
	if err != nil {
		encoded, _ := json.Marshal(protocol.RenameResponse{OK: false, Error: err.Error()})
		return C.CString(string(encoded))
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
	encoded, _ := json.Marshal(protocol.RenameResponse{
		OK:     true,
		Source: updated,
		Edits:  wireEdits,
	})
	return C.CString(string(encoded))
}

// CherriFormat formats Cherri v2 source while preserving comments.
//
//export CherriFormat
func CherriFormat(source *C.char) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	uri := "file:///mobile.cherri"
	svc.OpenDocument(uri, 1, src)
	formatted, err := svc.Format(uri)
	if err != nil {
		encoded, _ := json.Marshal(protocol.FormatResponse{OK: false, Error: err.Error()})
		return C.CString(string(encoded))
	}
	encoded, _ := json.Marshal(protocol.FormatResponse{OK: true, Source: formatted})
	return C.CString(string(encoded))
}

// CherriEditSourceNode applies a bounded AST visual source edit for actionIndex and paramLabel.
//
//export CherriEditSourceNode
func CherriEditSourceNode(source *C.char, actionIndex C.int, paramLabel *C.char, newExpr *C.char) *C.char {
	src := C.GoString(source)
	svc := service.NewService(schema.DefaultRegistry())
	updated, err := svc.ApplyVisualSourceEdit(src, int(actionIndex), C.GoString(paramLabel), C.GoString(newExpr))
	if err != nil {
		encoded, _ := json.Marshal(protocol.SourceEditResponse{OK: false, Source: src, Error: err.Error()})
		return C.CString(string(encoded))
	}
	encoded, _ := json.Marshal(protocol.SourceEditResponse{OK: true, Source: updated})
	return C.CString(string(encoded))
}

func encodeMobileResponse(response mobileCompileResponse) *C.char {
	encoded, err := json.Marshal(response)
	if err != nil {
		encoded = []byte(`{"ok":false,"error":"unable to encode compiler response"}`)
	}
	return C.CString(string(encoded))
}

// CherriFree releases strings returned by the mobile bridge.
//
//export CherriFree
func CherriFree(pointer *C.char) {
	C.free(unsafe.Pointer(pointer))
}
