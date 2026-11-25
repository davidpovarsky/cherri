package service

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// AnalyzeSnapshot returns diagnostics and the version that owns those exact
// diagnostics under one lock, without replacing the active document.
func (s *Service) AnalyzeSnapshot(uri string) ([]analysis.Diagnostic, string, int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.analyzeDocumentSnapshotLocked(uri)
}

// FormatEdit binds canonical output and the exact original full-document span
// to the same immutable document snapshot. Hosts must apply this span only to
// the source/version they requested; they must not invent a sentinel EOF.
func (s *Service) FormatEdit(uri string) (TextEdit, error) {
	s.mu.RLock()
	doc, ok := s.documents[uri]
	s.mu.RUnlock()
	if !ok || doc.File == nil {
		return TextEdit{}, fmt.Errorf("document not found: %s", uri)
	}
	if len(doc.ParseErrors) != 0 {
		return TextEdit{}, fmt.Errorf("cannot format document with syntax errors")
	}
	return TextEdit{
		Span: source.Span{
			Document: doc.File.ID,
			Start:    doc.File.PositionFromByteOffset(0),
			End:      doc.File.PositionFromByteOffset(len(doc.File.Content)),
		},
		NewText: syntax.Format(doc.Program),
	}, nil
}
