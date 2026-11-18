package service

import "github.com/electrikmilk/cherri/internal/language/schema"

// Each immutable document snapshot owns the declarations used by analysis,
// completion, hover, signatures and enum context. The service catalog remains
// the canonical base used for compatibility fingerprints and future documents.
func (s *Service) documentRegistry(doc *DocumentState) *schema.Registry {
	if doc != nil && doc.Analyzer != nil {
		return doc.Analyzer.Registry()
	}
	return s.registry
}
