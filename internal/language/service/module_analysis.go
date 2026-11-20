package service

import (
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// The caller holds s.mu. Open document overlays are constrained by the same
// entry workspace as disk reads; unrelated open documents cannot grant access.
func (s *Service) analyzeDocumentLocked(uri string, version int, text string) *DocumentState {
	file := source.NewFile(source.DocumentID(uri), uri, version, text)
	parser := syntax.NewParser(file)
	program := parser.ParseProgram()
	analyzer := analysis.NewAnalyzer(s.registry)
	var dependencyDiagnostics []analysis.Diagnostic
	if len(parser.Errors()) == 0 && modules.HasImports(program) {
		overlays := map[string]string{}
		for documentURI, document := range s.documents {
			if document.File != nil {
				overlays[documentURI] = document.File.Content
			}
		}
		overlays[uri] = text
		graph, issue := modules.Resolve(program, uri, overlays, s.registry, migrate.MigrateModuleSource)
		if issue != nil {
			dependencyDiagnostics = append(dependencyDiagnostics, analysis.Diagnostic{
				Code: issue.Code, Severity: analysis.SeverityError, Span: issue.Span, Message: issue.Message,
			})
		} else {
			analyzer.SetModuleGraph(graph)
		}
	}
	analyzer.Analyze(program)
	return &DocumentState{
		URI: uri, Version: version, File: file, Program: program, ParseErrors: parser.Errors(),
		Analyzer: analyzer, ModuleDiagnostics: dependencyDiagnostics,
	}
}
