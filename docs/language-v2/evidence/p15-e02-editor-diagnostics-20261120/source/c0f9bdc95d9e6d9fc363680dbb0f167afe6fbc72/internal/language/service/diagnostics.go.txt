package service

import "github.com/electrikmilk/cherri/internal/language/analysis"

// Parser recovery may report the identical missing closing token while
// unwinding each enclosing expression. Preserve first-observed order and all
// independent codes, messages, severities and complete source ranges.
func uniqueDocumentDiagnostics(input []analysis.Diagnostic) []analysis.Diagnostic {
	seen := make(map[analysis.Diagnostic]bool, len(input))
	result := make([]analysis.Diagnostic, 0, len(input))
	for _, diagnostic := range input {
		if seen[diagnostic] {
			continue
		}
		seen[diagnostic] = true
		result = append(result, diagnostic)
	}
	return result
}
