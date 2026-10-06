package protocol

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

// ProtocolVersion is the wire protocol version.
const ProtocolVersion = "1.0"

// Position represents a 0-based UTF-16 position for LSP and JSON protocols.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// Range represents a 0-based UTF-16 range for LSP and JSON protocols.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// DiagnosticItem represents a serialized diagnostic.
type DiagnosticItem struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Message  string `json:"message"`
	Range    Range  `json:"range"`
}

// AnalyzeResponse contains analysis results.
type AnalyzeResponse struct {
	URI               string           `json:"uri"`
	Version           int              `json:"version"`
	LanguageVersion   string           `json:"languageVersion"`
	SchemaFingerprint string           `json:"schemaFingerprint"`
	Diagnostics       []DiagnosticItem `json:"diagnostics"`
	Valid             bool             `json:"valid"`
}

// CapabilitiesResponse is emitted by `cherri --capabilities-json`.
type CapabilitiesResponse struct {
	LanguageVersion      string   `json:"languageVersion"`
	CatalogSchemaVersion string   `json:"catalogSchemaVersion"`
	ProtocolVersion      string   `json:"protocolVersion"`
	SchemaFingerprint    string   `json:"schemaFingerprint"`
	ActionCount          int      `json:"actionCount"`
	Features             []string `json:"features"`
}

// ToDiagnosticItem converts an internal Diagnostic to a wire protocol DiagnosticItem.
func ToDiagnosticItem(d analysis.Diagnostic) DiagnosticItem {
	return DiagnosticItem{
		Code:     d.Code,
		Severity: d.Severity.String(),
		Message:  d.Message,
		Range: Range{
			Start: Position{
				Line:      d.Span.Start.LSPLine(),
				Character: d.Span.Start.Column - 1,
			},
			End: Position{
				Line:      d.Span.End.LSPLine(),
				Character: d.Span.End.Column - 1,
			},
		},
	}
}

// BuildCapabilities returns current capabilities metadata.
func BuildCapabilities(reg *schema.Registry) CapabilitiesResponse {
	if reg == nil {
		reg = schema.DefaultRegistry()
	}
	return CapabilitiesResponse{
		LanguageVersion:      schema.LanguageVersion,
		CatalogSchemaVersion: schema.CatalogSchemaVersion,
		ProtocolVersion:      ProtocolVersion,
		SchemaFingerprint:    reg.Fingerprint(),
		ActionCount:          len(reg.AllActions()),
		Features: []string{
			"let_var_bindings",
			"named_call_arguments",
			"f_strings",
			"raw_strings",
			"typed_collections",
			"value_producing_blocks",
			"ast_functions",
			"native_preservation",
			"shared_analysis_service",
			"zero_based_indexing",
		},
	}
}
