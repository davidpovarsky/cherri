package main

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// Requiredness applies to the supplied pack, not its cardinality. An explicit
// named [] preserves the existing zero-item native encoding; omission is distinct.
func TestP15RequiredVariadicPresence(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, tc := range p15C03Cases {
		t.Run(tc.name, func(t *testing.T) {
			action, found := schema.DefaultRegistry().LookupAction(tc.name)
			if !found || len(action.Parameters) != 1 || !action.Parameters[0].Variadic {
				t.Fatal("fixture is not an original canonical variadic")
			}
			source := p15C03Source(tc, 0, false)
			diagnostics := p15TypeDiagnostics(t, source)
			svc := service.NewService(nil)
			uri := "file:///p15-required-variadic.cherri"
			svc.OpenDocument(uri, 1, source)
			shared, _ := svc.Analyze(uri)
			if !action.Parameters[0].Optional {
				for _, observed := range [][]analysis.Diagnostic{diagnostics, shared} {
					if err := p15ExactLiteralDiagnostic(observed, source, strings.TrimSpace(source), analysis.CodeMissingArgument); err != nil {
						t.Fatal(err)
					}
				}
				raw, err := CompileSourceToPlist("missing-variadic.cherri", source)
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeMissingArgument) {
					t.Fatalf("omitted required pack emitted native output: %d %v", len(raw), err)
				}
			} else {
				for _, observed := range [][]analysis.Diagnostic{diagnostics, shared} {
					for _, diagnostic := range observed {
						if diagnostic.Severity == analysis.SeverityError {
							t.Fatal("optional variadic omission rejected", diagnostic)
						}
					}
				}
			}
			actions, err := p15C03Actions(p15C03Source(tc, 0, true), false)
			if err != nil {
				t.Fatal("supplied empty pack rejected", err)
			}
			if err := p15C03LiteralOracle(actions, tc, 0); err != nil {
				t.Fatal("empty pack changed canonical wire", err)
			}
		})
	}
}
