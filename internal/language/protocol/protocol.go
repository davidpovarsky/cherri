package protocol

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
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

// CompletionItem represents a completion candidate for wire protocols.
type CompletionItem struct {
	Label         string `json:"label"`
	Kind          int    `json:"kind"`
	Detail        string `json:"detail,omitempty"`
	Documentation string `json:"documentation,omitempty"`
	InsertText    string `json:"insertText,omitempty"`
}

// CompleteResponse contains completion results.
type CompleteResponse struct {
	URI               string           `json:"uri"`
	Version           int              `json:"version"`
	LanguageVersion   string           `json:"languageVersion"`
	SchemaFingerprint string           `json:"schemaFingerprint"`
	Items             []CompletionItem `json:"items"`
}

// HoverResponse contains hover documentation and range.
type HoverResponse struct {
	OK       bool   `json:"ok"`
	Contents string `json:"contents,omitempty"`
	Range    *Range `json:"range,omitempty"`
}

// DefinitionResponse contains go-to-definition target location.
type DefinitionResponse struct {
	OK    bool   `json:"ok"`
	URI   string `json:"uri,omitempty"`
	Range *Range `json:"range,omitempty"`
}

// SignatureHelpResponse contains signature help for an active call.
type SignatureHelpResponse struct {
	OK              bool     `json:"ok"`
	Label           string   `json:"label,omitempty"`
	Documentation   string   `json:"documentation,omitempty"`
	Parameters      []string `json:"parameters,omitempty"`
	ActiveParameter int      `json:"activeParameter"`
}

// TextEditItem represents a source text replacement over a UTF-16 range.
type TextEditItem struct {
	Range   Range  `json:"range"`
	NewText string `json:"newText"`
}

// RenameResponse contains the result of a scoped symbol rename.
type RenameResponse struct {
	OK     bool           `json:"ok"`
	Source string         `json:"source,omitempty"`
	Edits  []TextEditItem `json:"edits,omitempty"`
	Error  string         `json:"error,omitempty"`
}

// FormatResponse contains canonical formatted source.
type FormatResponse struct {
	OK     bool   `json:"ok"`
	Source string `json:"source,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SourceEditResponse contains the result of a bounded AST visual source edit.
type SourceEditResponse struct {
	OK     bool   `json:"ok"`
	Source string `json:"source,omitempty"`
	Error  string `json:"error,omitempty"`
}

// CapabilitiesResponse is emitted by `cherri --capabilities-json`.
type CapabilitiesResponse struct {
	LanguageVersion      string   `json:"languageVersion"`
	CatalogSchemaVersion string   `json:"catalogSchemaVersion"`
	ProtocolVersion      string   `json:"protocolVersion"`
	SchemaFingerprint    string   `json:"schemaFingerprint"`
	CodecFingerprint     string   `json:"codecFingerprint"`
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
				Character: d.Span.Start.LSPCharacterColumn(),
			},
			End: Position{
				Line:      d.Span.End.LSPLine(),
				Character: d.Span.End.LSPCharacterColumn(),
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
		CodecFingerprint:     backend.CodecFingerprint(),
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
