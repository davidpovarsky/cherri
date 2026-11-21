package service

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// PositionFromLSP adapts the wire's zero-based UTF-16 position to the shared
// service's one-based rune position, using the same document source map as
// diagnostics. CLI and embedded callers keep their existing rune coordinates.
func (s *Service) PositionFromLSP(uri string, line, character int) (source.Position, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	doc, ok := s.documents[uri]
	if !ok || doc.File == nil || line < 0 || character < 0 {
		return source.Position{}, false
	}
	return doc.File.PositionFromLSP(line, character), true
}

func annotationText(file *source.File, annotation syntax.TypeAnnotation, fallback string) string {
	if file == nil || annotation == nil {
		return fallback
	}
	span := annotation.NodeSpan()
	start, end := span.Start.ByteOffset, span.End.ByteOffset
	if start < 0 || end <= start || end > len(file.Content) {
		return fallback
	}
	return strings.TrimSpace(file.Content[start:end])
}

func parameterSignature(file *source.File, parameter syntax.ParameterDecl) string {
	name := parameter.Name
	if parameter.Optional {
		name += "?"
	}
	return name + ": " + annotationText(file, parameter.TypeExpr, "AnyContent")
}
