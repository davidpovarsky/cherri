package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15ProducerDiagnostic struct {
	code, marker, message string
	last                  bool
}

func p15ProducerDiagnosticError(diagnostics []analysis.Diagnostic, source string, expected []p15ProducerDiagnostic) error {
	var failures []analysis.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			failures = append(failures, d)
		}
	}
	if len(failures) != len(expected) {
		return fmt.Errorf("expected %d independent diagnostics, got %+v", len(expected), failures)
	}
	for i, want := range expected {
		start := strings.Index(source, want.marker)
		if want.last {
			start = strings.LastIndex(source, want.marker)
		}
		got := failures[i]
		if start < 0 || got.Code != want.code || got.Span.Start.ByteOffset != start ||
			got.Span.End.ByteOffset != start+len(want.marker) || !strings.Contains(got.Message, want.message) {
			return fmt.Errorf("diagnostic %d is not the declared causal source: got %+v want %+v at %d", i, got, want, start)
		}
	}
	return nil
}

func TestP15TaggedInvalidProducerDiagnostics(t *testing.T) {
	loopNone := p15ProducerDiagnostic{analysis.CodeTargetValueShape, "none", "optional absence", false}
	loopList := func(marker string) p15ProducerDiagnostic {
		return p15ProducerDiagnostic{analysis.CodeTargetValueShape, marker, "List-valued loop results are unsupported", false}
	}
	boundary := func(marker string) p15ProducerDiagnostic {
		return p15ProducerDiagnostic{analysis.CodeTargetValueShape, marker, "structured return requires an explicit external value codec", true}
	}
	for _, tc := range []struct {
		name, source string
		expected     []p15ProducerDiagnostic
	}{
		{"direct_optional", "let values=repeat 2 {yield none}\nreturn values", []p15ProducerDiagnostic{loopNone}},
		{"immutable_alias_chain", "let values=repeat 2 {yield [1]}\nlet alias=values\nlet copied=alias\nreturn copied", []p15ProducerDiagnostic{loopList("[1]")}},
		{"mutable_alias", "var values=repeat 2 {yield none}\nlet alias=values\nreturn alias", []p15ProducerDiagnostic{loopNone}},
		{"failed_replacement", "var values=[[1]]\nvalues=repeat 2 {yield [2]}\nreturn values", []p15ProducerDiagnostic{loopList("[2]")}},
		{"valid_replacement", "var values=repeat 2 {yield [1]}\nvalues=[[2]]\nreturn values", []p15ProducerDiagnostic{loopList("[1]"), boundary("values")}},
		{"compound_invalid_alias", "let invalidAlias:List<List<Number>>=[[\"wrong\"]]\nvar values=[[1]]\nvalues+=invalidAlias\nreturn values", []p15ProducerDiagnostic{{analysis.CodeArgumentType, "[[\"wrong\"]]", "not assignable", false}}},
		{"independent_unknown_name", "let values=repeat 2 {yield none}\nlet unrelated=missingName\nreturn values", []p15ProducerDiagnostic{loopNone, {analysis.CodeUnknownName, "missingName", "unknown", false}}},
		{"independent_literal_return", "let values=repeat 2 {yield none}\nreturn [[3]]", []p15ProducerDiagnostic{loopNone, boundary("[[3]]")}},
		{"shadowed_valid_producer", "let values=repeat 2 {yield none}\nif true {let values=[[3]]\nreturn values}\nreturn 0", []p15ProducerDiagnostic{loopNone, boundary("values")}},
		{"one_valid_branch", "var values=[[1]]\nif true {values=repeat 2 {yield [2]}} else {values=[[3]]}\nreturn values", []p15ProducerDiagnostic{loopList("[2]"), boundary("values")}},
		{"all_invalid_branches", "var values=[[1]]\nif true {values=repeat 2 {yield [2]}} else {values=repeat 2 {yield [3]}}\nreturn values", []p15ProducerDiagnostic{loopList("[2]"), loopList("[3]")}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ProducerDiagnosticError(diagnostics, tc.source, tc.expected); err != nil {
				t.Error("analysis:", err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-producer-diagnostic-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			observed, _ := svc.Analyze(uri)
			if err := p15ProducerDiagnosticError(observed, tc.source, tc.expected); err != nil {
				t.Error("service:", err)
			}
			raw, err := CompileSourceToPlist("p15-producer-diagnostic.cherri", tc.source)
			if err == nil || len(raw) != 0 {
				t.Errorf("rejected producer or boundary reached emission: bytes=%d error=%v", len(raw), err)
			}
			t.Logf("P15_INVALID_PRODUCER source_sha256=%x expected_errors=%d", sha256.Sum256([]byte(tc.source)), len(tc.expected))
		})
	}
}

func TestP15TaggedValidProducerStillChecksBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		expected     []p15ProducerDiagnostic
	}{
		{"valid_private_return", "let values=[[1]]\nreturn values", []p15ProducerDiagnostic{{analysis.CodeTargetValueShape, "values", "structured return requires an explicit external value codec", true}}},
		{"valid_compound_alias", "let alias=[[2]]\nvar values=[[1]]\nvalues+=alias\nreturn values", []p15ProducerDiagnostic{{analysis.CodeTargetValueShape, "values", "structured return requires an explicit external value codec", true}}},
		{"valid_local_consumer", "let values=[[1]]\nreturn count(values)", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ProducerDiagnosticError(p15TypeDiagnostics(t, tc.source), tc.source, tc.expected); err != nil {
				t.Fatal(err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-producer-reanalysis.cherri"
			svc.OpenDocument(uri, 1, "let values=repeat 2 {yield none}\nreturn values")
			initial, _ := svc.Analyze(uri)
			if len(initial) == 0 {
				t.Fatal("initial rejected producer did not produce diagnostics")
			}
			svc.OpenDocument(uri, 2, tc.source)
			current, _ := svc.Analyze(uri)
			if err := p15ProducerDiagnosticError(current, tc.source, tc.expected); err != nil {
				t.Fatal("reanalysis:", err)
			}
			raw, err := CompileSourceToPlist("p15-producer-valid.cherri", tc.source)
			if len(tc.expected) == 0 {
				if err != nil || len(raw) == 0 {
					t.Fatalf("valid local consumer rejected: bytes=%d error=%v", len(raw), err)
				}
			} else if err == nil || len(raw) != 0 {
				t.Fatalf("valid private producer bypassed external boundary: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}
