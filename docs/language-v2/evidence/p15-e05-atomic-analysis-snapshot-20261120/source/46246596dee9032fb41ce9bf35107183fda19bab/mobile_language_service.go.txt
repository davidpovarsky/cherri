package main

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
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
