package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func TestCompileSemanticErrorsRetainIndependentDiagnostics(t *testing.T) {
	cases := []struct {
		name   string
		source string
		codes  []string
	}{
		{
			"date_quantity_and_literal_contracts",
			"let unit=text(\"hr\")\nif CurrentDate > qty(3, unit) {}",
			[]string{analysis.CodeLiteralRequired, analysis.CodeArgumentType},
		},
		{
			"independent_statements",
			"let first:Number=\"text\"\nlet second:Text=3",
			[]string{analysis.CodeArgumentType, analysis.CodeArgumentType},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			var codes []string
			var messages []string
			var lines []int
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity != analysis.SeverityError {
					continue
				}
				codes = append(codes, diagnostic.Code)
				messages = append(messages, diagnostic.Message)
				lines = append(lines, diagnostic.Span.Start.Line)
			}
			if !reflect.DeepEqual(codes, tc.codes) {
				t.Fatalf("source no longer exercises both independent semantic contracts: %v", diagnostics)
			}
			raw, err := CompileSourceToPlist("all-errors.cherri", tc.source)
			if err == nil || len(raw) != 0 {
				t.Fatalf("invalid source reached a native artifact: bytes=%d err=%v", len(raw), err)
			}
			remaining := err.Error()
			for index, message := range messages {
				needle := fmt.Sprintf("semantic error [%s]: %s at ", codes[index], message)
				offset := strings.Index(remaining, needle)
				if offset < 0 {
					t.Errorf("compile hid or reordered diagnostic %q: %v", needle, err)
					continue
				}
				remaining = remaining[offset+len(needle):]
			}
			errorLines, isolationErr := collectV2CompileErrorLines("all-errors.cherri", tc.source)
			if isolationErr == nil || isolationErr.Error() != err.Error() || !reflect.DeepEqual(errorLines, lines) {
				t.Fatalf("isolation and public compilation disagree: lines=%v want=%v error=%v compile=%v", errorLines, lines, isolationErr, err)
			}
		})
	}
}

func TestCompileSemanticErrorsPreserveSingleErrorAndSuccess(t *testing.T) {
	source := "let amount:Number=\"text\""
	diagnostics := p15TypeDiagnostics(t, source)
	var errors []analysis.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			errors = append(errors, diagnostic)
		}
	}
	if len(errors) != 1 {
		t.Fatal("single-error control changed", diagnostics)
	}
	raw, err := CompileSourceToPlist("single-error.cherri", source)
	want := fmt.Sprintf("semantic error [%s]: %s at ", errors[0].Code, errors[0].Message)
	if err == nil || len(raw) != 0 || !strings.HasPrefix(err.Error(), want) || strings.Contains(err.Error(), "\n") {
		t.Fatalf("single-error formatting or rejection changed: bytes=%d err=%v", len(raw), err)
	}
	source = "text(\"valid source\")"
	raw, err = CompileSourceToPlist("no-errors.cherri", source)
	if err != nil || len(raw) == 0 {
		t.Fatalf("valid compilation changed: bytes=%d err=%v", len(raw), err)
	}
	lines, err := collectV2CompileErrorLines("no-errors.cherri", source)
	if err != nil || len(lines) != 0 {
		t.Fatalf("valid source gained isolation errors: lines=%v err=%v", lines, err)
	}
}
