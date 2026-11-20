package service

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
)

// Independent byte/rune/UTF-16 oracle: it never calls a production position
// conversion helper, lexer, parser or diagnostic constructor.
func p15EditorExpectedPosition(text string, offset int) source.Position {
	prefix := text[:offset]
	linePrefix := prefix[strings.LastIndex(prefix, "\n")+1:]
	return source.Position{
		Line: strings.Count(prefix, "\n") + 1, Column: utf8.RuneCountInString(linePrefix) + 1,
		UTF16Column: len(utf16.Encode([]rune(linePrefix))) + 1,
		ByteOffset:  offset, UTF16Offset: len(utf16.Encode([]rune(prefix))),
	}
}

func p15EditorSpanError(text, uri string, span source.Span) error {
	start, end := span.Start.ByteOffset, span.End.ByteOffset
	if start < 0 || end < start || end > len(text) || !utf8.ValidString(text[:start]) || !utf8.ValidString(text[:end]) {
		return fmt.Errorf("invalid authored UTF-8 span %+v", span)
	}
	expected := source.Span{Document: source.DocumentID(uri), Start: p15EditorExpectedPosition(text, start), End: p15EditorExpectedPosition(text, end)}
	if span != expected {
		return fmt.Errorf("complete source range differs: actual %+v expected %+v", span, expected)
	}
	return nil
}

type p15EditorDiagnosticRequirement struct{ code, needle string }

func p15E02DiagnosticError(text, uri string, actual []analysis.Diagnostic, want []p15EditorDiagnosticRequirement, syntaxRequired bool, max int) error {
	if len(actual) > max {
		return fmt.Errorf("diagnostic flood: %d exceeds %d; actual %+v", len(actual), max, actual)
	}
	found := make([]bool, len(want))
	seen := map[string]bool{}
	syntaxFound := false
	for _, d := range actual {
		if err := p15EditorSpanError(text, uri, d.Span); err != nil {
			return err
		}
		if d.Message == "" {
			return fmt.Errorf("diagnostic message absent")
		}
		key := fmt.Sprintf("%s/%d/%d/%d/%s", d.Code, d.Severity, d.Span.Start.ByteOffset, d.Span.End.ByteOffset, d.Message)
		if seen[key] {
			return fmt.Errorf("duplicate diagnostic %s", key)
		}
		seen[key] = true
		if d.Code == analysis.CodeSyntax && d.Severity == analysis.SeverityError {
			syntaxFound = true
		}
		for i, w := range want {
			start := strings.Index(text, w.needle)
			if start < 0 {
				return fmt.Errorf("independent expected source token absent %q", w.needle)
			}
			if d.Code == w.code && d.Span.Start.ByteOffset == start && d.Span.End.ByteOffset == start+len(w.needle) {
				if d.Severity != analysis.SeverityError {
					return fmt.Errorf("required error severity changed")
				}
				found[i] = true
			}
		}
	}
	for i, yes := range found {
		if !yes {
			return fmt.Errorf("independent diagnostic absent %+v; actual %+v", want[i], actual)
		}
	}
	if syntaxRequired && !syntaxFound {
		return fmt.Errorf("incomplete source lost syntax diagnostic")
	}
	return nil
}

const p15E02IndependentPrefix = "let fixed = 1\nfixed = 2\nshow(f\"\u05e9\u05dc\u05d5\u05dd\U0001F642{missingOne}\")\nshow(missingTwo)\n"

func TestP15E02IncompleteIndependentDiagnostics(t *testing.T) {
	wanted := []p15EditorDiagnosticRequirement{
		{analysis.CodeAssignImmutable, "fixed = 2"},
		{analysis.CodeUnknownName, "missingOne"},
		{analysis.CodeUnknownName, "missingTwo"},
	}
	for _, tc := range []struct {
		name, suffix string
		limit        int
	}{
		{"trailing_initializer", "let unfinished =", 6},
		{"unclosed_call", "show(", 6},
		{"incomplete_named_value", "formatNumber(1, decimalPlaces: )", 7},
		{"nested_incomplete_calls", strings.Repeat("round(", 20), 6},
		{"incomplete_unicode_string", "let unfinished = \"\u05e9\u05dc\u05d5\u05dd\U0001F642", 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e02-incomplete.cherri"
			text := p15E02IndependentPrefix + tc.suffix
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, text)
			actual, fingerprint := svc.Analyze(uri)
			if len(fingerprint) != 64 {
				t.Fatal("schema identity missing")
			}
			if err := p15E02DiagnosticError(text, uri, actual, wanted, true, tc.limit); err != nil {
				t.Fatal(err)
			}
			again, _ := svc.Analyze(uri)
			if !reflect.DeepEqual(actual, again) {
				t.Fatal("identical document analysis changed diagnostics")
			}
			svc.ChangeDocument(uri, 2, "let fixed=1\nshow(fixed)\n")
			healthy, _ := svc.Analyze(uri)
			if len(healthy) != 0 {
				t.Fatal("recovery retained stale diagnostics", healthy)
			}
		})
	}
}

func TestP15E02RangeAndDiagnosticOracleMutations(t *testing.T) {
	const uri = "file:///p15-e02-oracle.cherri"
	text := p15E02IndependentPrefix
	svc := NewService(nil)
	svc.OpenDocument(uri, 1, text)
	actual, _ := svc.Analyze(uri)
	want := []p15EditorDiagnosticRequirement{{analysis.CodeAssignImmutable, "fixed = 2"}, {analysis.CodeUnknownName, "missingOne"}, {analysis.CodeUnknownName, "missingTwo"}}
	if err := p15E02DiagnosticError(text, uri, actual, want, false, 3); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "duplicate", "wrong_code", "warning", "wrong_document", "start_byte", "end_byte", "rune_column", "utf16_column", "utf16_offset", "message_lost"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]analysis.Diagnostic{}, actual...)
			target := -1
			for i, d := range changed {
				if d.Code == analysis.CodeUnknownName && text[d.Span.Start.ByteOffset:d.Span.End.ByteOffset] == "missingOne" {
					target = i
				}
			}
			if target < 0 {
				t.Fatal("actual Unicode diagnostic target absent")
			}
			switch name {
			case "missing":
				changed = append(changed[:target], changed[target+1:]...)
			case "duplicate":
				changed = append(changed, changed[target])
			case "wrong_code":
				changed[target].Code = "E_FOREIGN"
			case "warning":
				changed[target].Severity = analysis.SeverityWarning
			case "wrong_document":
				changed[target].Span.Document = "file:///other.cherri"
			case "start_byte":
				changed[target].Span.Start.ByteOffset++
			case "end_byte":
				changed[target].Span.End.ByteOffset--
			case "rune_column":
				changed[target].Span.Start.Column++
			case "utf16_column":
				changed[target].Span.Start.UTF16Column = changed[target].Span.Start.Column
			case "utf16_offset":
				changed[target].Span.End.UTF16Offset--
			case "message_lost":
				changed[target].Message = ""
			}
			if p15E02DiagnosticError(text, uri, changed, want, false, 3) == nil {
				t.Fatal("corrupted actual diagnostic accepted", name)
			}
		})
	}
}

func TestP15E02ClosedNestedCallControls(t *testing.T) {
	t.Run("closed_number_chain_is_healthy", func(t *testing.T) {
		const uri = "file:///p15-e02-closed-number.cherri"
		code := strings.Repeat("round(", 20) + "1" + strings.Repeat(")", 20)
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		actual, _ := svc.Analyze(uri)
		if len(actual) != 0 {
			t.Fatalf("number-result chain is not a valid independent control: %+v", actual)
		}
	})
	t.Run("distinct_void_argument_errors_remain", func(t *testing.T) {
		const uri = "file:///p15-e02-closed-void.cherri"
		code := strings.Repeat("show(", 8) + "\"value\"" + strings.Repeat(")", 8)
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		actual, _ := svc.Analyze(uri)
		if len(actual) != 7 {
			t.Fatalf("all7 independent Void argument errors must remain: %+v", actual)
		}
		ranges := map[source.Span]bool{}
		for _, d := range actual {
			if d.Code != analysis.CodeArgumentType || d.Severity != analysis.SeverityError || !strings.Contains(d.Message, "Void cannot be passed") {
				t.Fatal("wrong independent semantic error", d)
			}
			if err := p15EditorSpanError(code, uri, d.Span); err != nil {
				t.Fatal(err)
			}
			if ranges[d.Span] {
				t.Fatal("distinct call errors lost range identity")
			}
			ranges[d.Span] = true
		}
	})
}

func TestP15E02DistinctDiagnosticLocationsAndRecovery(t *testing.T) {
	t.Run("repeated_name_at_distinct_source_ranges", func(t *testing.T) {
		const uri = "file:///p15-e02-distinct.cherri"
		const code = "show(unknown)\nshow(unknown)\n"
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		actual, _ := svc.Analyze(uri)
		if len(actual) != 2 {
			t.Fatalf("distinct authored errors must both remain: %+v", actual)
		}
		starts := map[int]bool{}
		for _, d := range actual {
			if d.Code != analysis.CodeUnknownName || d.Severity != analysis.SeverityError {
				t.Fatal("wrong independent error", d)
			}
			if err := p15EditorSpanError(code, uri, d.Span); err != nil {
				t.Fatal(err)
			}
			starts[d.Span.Start.ByteOffset] = true
		}
		if !starts[5] || !starts[19] {
			t.Fatal("distinct occurrence positions lost", starts)
		}
	})
	t.Run("different_missing_tokens_at_same_eof", func(t *testing.T) {
		const uri = "file:///p15-e02-distinct-recovery.cherri"
		const code = "show(["
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		actual, _ := svc.Analyze(uri)
		messages := map[string]bool{}
		for _, d := range actual {
			if err := p15EditorSpanError(code, uri, d.Span); err != nil {
				t.Fatal(err)
			}
			if d.Code == analysis.CodeSyntax {
				if d.Span.Start.ByteOffset != len(code) || d.Span.End.ByteOffset != len(code) {
					t.Fatal("missing-token range moved", d)
				}
				if messages[d.Message] {
					t.Fatal("duplicate exact recovery diagnostic", d)
				}
				messages[d.Message] = true
			}
		}
		if len(messages) < 2 || len(messages) > 4 {
			t.Fatal("distinct missing delimiters lost or flooded", actual)
		}
	})
}
