package service

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"strings"
	"testing"
)

func TestP15E04IncompleteDocumentFormatFailsClosed(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///p15-format-incomplete.cherri"
	for _, code := range []string{"let value =\n", "let value = \"unterminated\n", "if true { show(\"x\")\n"} {
		svc.OpenDocument(uri, 1, code)
		if formatted, err := svc.Format(uri); err == nil || formatted != "" {
			t.Errorf("incomplete source produced a replacement instead of an error: output=%q error=%v", formatted, err)
		}
		diagnostics, _ := svc.Analyze(uri)
		if len(diagnostics) == 0 || svc.DocumentVersion(uri) != 1 {
			t.Fatal("formatting mutated or lost the original invalid document")
		}
	}
	svc.OpenDocument(uri, 2, "let value = r\"C:\\data\"\nshow(value)\n")
	formatted, err := svc.Format(uri)
	if err != nil || !strings.Contains(formatted, `r"C:\data"`) {
		t.Fatalf("healthy raw-string control: %v %q", err, formatted)
	}
}
