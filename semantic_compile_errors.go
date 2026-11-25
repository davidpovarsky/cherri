package main

import (
	"errors"
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// semanticCompileErrors preserves all analyzer errors in their original order.
// A more specific nested-constructor diagnostic must not hide an independent
// enclosing-expression error. One error retains the existing public message.
func semanticCompileErrors(diagnostics []analysis.Diagnostic) error {
	var reported []error
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			reported = append(reported, fmt.Errorf("semantic error [%s]: %s at %v", diagnostic.Code, diagnostic.Message, diagnostic.Span))
		}
	}
	return errors.Join(reported...)
}
