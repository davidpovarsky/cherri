package service

import (
	"fmt"
	"sort"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// refreshAssetDiagnosticsLocked validates only calls that semantic analysis
// resolved to the canonical embedFile action. A user function with the same
// spelling is never treated as permission to read a file.
func (s *Service) refreshAssetDiagnosticsLocked(doc *DocumentState) {
	doc.AssetDiagnostics = nil
	doc.AssetDependencies = nil
	if doc.Analyzer == nil || len(doc.ParseErrors) != 0 || len(doc.ModuleDiagnostics) != 0 {
		return
	}
	for _, diagnostic := range doc.Analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			return
		}
	}
	var calls []*syntax.CallExpr
	for expression := range doc.Analyzer.ExprTypes() {
		call, ok := expression.(*syntax.CallExpr)
		if !ok {
			continue
		}
		action := doc.Analyzer.ResolvedAction(call)
		if action != nil && action.Builtin && !action.Custom && action.ID == "embedFile" {
			calls = append(calls, call)
		}
	}
	sort.Slice(calls, func(i, j int) bool {
		a, b := calls[i].Span, calls[j].Span
		if a.Document != b.Document {
			return a.Document < b.Document
		}
		return a.Start.ByteOffset < b.Start.ByteOffset
	})
	if len(calls) == 0 {
		return
	}
	scope, openErr := assets.OpenScope(doc.URI, s.assetCache)
	if scope != nil {
		defer scope.Close()
	}
	for _, call := range calls {
		expression := call.PrimaryArg
		if expression == nil {
			for _, arg := range call.NamedArgs {
				if arg.Label == "filePath" {
					expression = arg.Value
					break
				}
			}
		}
		literal, ok := expression.(*syntax.LiteralExpr)
		if !ok || (literal.Type != syntax.TokenString && literal.Type != syntax.TokenRawString) {
			// Dynamic expressions keep their ordinary semantic diagnostics.
			// The backend independently requires a compile-time text path.
			continue
		}
		err := openErr
		var snapshot assets.Snapshot
		if err == nil {
			snapshot, err = scope.Read(string(call.Span.Document), literal.Value)
		}
		if err != nil {
			doc.AssetDiagnostics = append(doc.AssetDiagnostics, analysis.Diagnostic{
				Code: analysis.CodeWorkspaceAccess, Severity: analysis.SeverityError,
				Span: literal.Span, Message: fmt.Sprintf("embedded local asset: %v", err),
			})
			continue
		}
		// Documents retain only dependency metadata. Large encoded values stay
		// in the bounded shared cache and are consumed by the canonical emitter.
		snapshot.Base64 = ""
		doc.AssetDependencies = append(doc.AssetDependencies, snapshot)
	}
}

// AssetDependencies returns the most recently validated dependency metadata.
// Encoded file contents are deliberately excluded from this inspection API.
func (s *Service) AssetDependencies(uri string) []assets.Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if doc := s.documents[uri]; doc != nil {
		return append([]assets.Snapshot(nil), doc.AssetDependencies...)
	}
	return nil
}

func (s *Service) AssetCacheMetrics() assets.Metrics { return s.assetCache.Metrics() }

// InvalidateAssets accepts an explicit filesystem-change event. Metadata checks
// already catch ordinary edits; this also covers preserved size/timestamp edits.
// Returning open URIs lets hosts refresh their existing diagnostic publications.
func (s *Service) InvalidateAssets() []string {
	s.assetCache.Invalidate()
	s.mu.RLock()
	defer s.mu.RUnlock()
	uris := make([]string, 0, len(s.documents))
	for uri := range s.documents {
		uris = append(uris, uri)
	}
	sort.Strings(uris)
	return uris
}
