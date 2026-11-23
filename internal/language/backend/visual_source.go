package backend

import "github.com/electrikmilk/cherri/internal/language/source"

// VisualSourceOrigin is optional compiler-only provenance. ArgumentSpans follows
// ResolvedCall.Arguments in its actual binding order. It is never an Apple field.
type VisualSourceOrigin struct {
	CallSpan      source.Span
	ArgumentSpans []source.Span
}
