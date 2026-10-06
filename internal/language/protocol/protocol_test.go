package protocol

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
)

func TestBuildCapabilities(t *testing.T) {
	caps := BuildCapabilities(nil)
	if caps.LanguageVersion != "2.0" {
		t.Errorf("expected LanguageVersion 2.0, got %s", caps.LanguageVersion)
	}
	if caps.CatalogSchemaVersion != "3.0" {
		t.Errorf("expected CatalogSchemaVersion 3.0, got %s", caps.CatalogSchemaVersion)
	}
	if caps.ActionCount != 461 {
		t.Errorf("expected ActionCount 461, got %d", caps.ActionCount)
	}
	if len(caps.SchemaFingerprint) != 64 {
		t.Errorf("expected 64-char fingerprint, got %s", caps.SchemaFingerprint)
	}
}

func TestToDiagnosticItem(t *testing.T) {
	d := analysis.Diagnostic{
		Code:     analysis.CodeAssignImmutable,
		Severity: analysis.SeverityError,
		Message:  "cannot assign immutable",
		Span: source.Span{
			Start: source.Position{Line: 5, Column: 2},
			End:   source.Position{Line: 5, Column: 6},
		},
	}
	item := ToDiagnosticItem(d)
	if item.Code != "E_ASSIGN_IMMUTABLE" {
		t.Errorf("expected code E_ASSIGN_IMMUTABLE, got %s", item.Code)
	}
	if item.Range.Start.Line != 4 || item.Range.Start.Character != 1 {
		t.Errorf("expected 0-based start 4:1, got %d:%d", item.Range.Start.Line, item.Range.Start.Character)
	}
}
