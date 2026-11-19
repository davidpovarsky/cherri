package main

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/backend"
)

// configureAssets keeps source location and cache ownership explicit for each
// V2 compilation. The root is opened lazily, only when embedFile is actually used.
func (s *CanonicalBackendSession) configureAssets(entry string, cache *assets.Cache) {
	s.assetEntryDocument = entry
	s.assetCache = cache
}

func (s *CanonicalBackendSession) prepareEmbeddedAsset(call backend.ResolvedCall, ref *actionReference) error {
	if ref.identifier != "embedFile" {
		return nil
	}
	if len(ref.arguments) != 1 {
		return fmt.Errorf("[%s] embedFile requires one local path", analysis.CodeLiteralRequired)
	}
	requested, ok := getArgValue(ref.arguments[0]).(string)
	if !ok {
		return fmt.Errorf("[%s] embedFile requires a compile-time text path", analysis.CodeLiteralRequired)
	}
	if s.assetScope == nil {
		entry := s.assetEntryDocument
		if entry == "" {
			entry = "main.cherri"
		}
		var err error
		s.assetScope, err = assets.OpenScope(entry, s.assetCache)
		if err != nil {
			return fmt.Errorf("[%s] embedded local asset: %w", analysis.CodeWorkspaceAccess, err)
		}
	}
	snapshot, err := s.assetScope.Read(call.SourceDocument, requested)
	if err != nil {
		return fmt.Errorf("[%s] embedded local asset: %w", analysis.CodeWorkspaceAccess, err)
	}
	ref.embeddedAsset = &snapshot
	return nil
}
