package syntax

// Parse diagnostic identity comes from the detecting parser branch, never from
// authored text interpolated into a human-readable message.
const (
	CodeSyntax       = "E_SYNTAX"
	CodeLegacySyntax = "E_LEGACY_SYNTAX"
)

func (e ParseError) DiagnosticCode() string {
	if e.code != "" {
		return e.code
	}
	return CodeSyntax
}
