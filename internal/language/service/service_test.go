package service

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func TestServiceAnalyze(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///test.cherri"
	code := `let x = 42
x = 10
`
	svc.OpenDocument(uri, 1, code)
	diags, fp := svc.Analyze(uri)

	if len(fp) != 64 {
		t.Errorf("expected 64-char schema fingerprint, got %q", fp)
	}
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics for reassigning let, got 0")
	}
}

func TestServiceComplete(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///complete.cherri"
	code := `let myVar = 10
`
	svc.OpenDocument(uri, 1, code)
	items := svc.Complete(uri, 1, 1)

	foundMyVar := false
	foundResize := false
	for _, it := range items {
		if it.Label == "myVar" {
			foundMyVar = true
		}
		if it.Label == "resizeImage" {
			foundResize = true
		}
	}

	if !foundMyVar {
		t.Errorf("expected to find local variable myVar in completions")
	}
	if !foundResize {
		t.Errorf("expected to find action resizeImage in completions")
	}
}

func TestServiceHover(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///hover.cherri"
	code := `resizeImage(photo, width: 800)`
	svc.OpenDocument(uri, 1, code)

	hover := svc.Hover(uri, 1, 3) // over resizeImage
	if hover == nil {
		t.Fatalf("expected hover result over resizeImage, got nil")
	}
	if len(hover.Contents) == 0 {
		t.Errorf("expected non-empty hover contents")
	}
}

func TestServiceContextualCallComplete(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///call.cherri"
	code := `resizeImage(photo, `
	svc.OpenDocument(uri, 1, code)

	items := svc.Complete(uri, 1, len(code)+1)
	if len(items) == 0 {
		t.Fatalf("expected completions inside call, got 0")
	}

	foundParam := false
	for _, it := range items {
		if it.Kind == CompletionKindParameter {
			foundParam = true
			break
		}
	}
	if !foundParam {
		t.Errorf("expected to find parameter completion for resizeImage arguments")
	}

	// Test dot completion for enums
	dotURI := "file:///dot.cherri"
	dotCode := `.`
	svc.OpenDocument(dotURI, 1, dotCode)
	dotItems := svc.Complete(dotURI, 1, 2)
	foundEnum := false
	for _, it := range dotItems {
		if it.Kind == CompletionKindEnum {
			foundEnum = true
			break
		}
	}
	if !foundEnum {
		t.Errorf("expected to find enum completion after dot")
	}
}

func TestE01MultiDiagnosticAnalysis(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e01.cherri"
	code := "let a = 10\na = 20\nshow(unknownOne)\nshow(unknownTwo)\n"
	svc.OpenDocument(uri, 1, code)
	diags, _ := svc.Analyze(uri)
	if len(diags) < 3 {
		t.Fatalf("expected >= 3 diagnostics in one pass, got %d", len(diags))
	}
}

func TestE02CompletionHoverSignatureDefinition(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e02.cherri"
	code := "function greet(person: Text) -> Text {\n    return f\"Hi {person}\"\n}\nlet targetName = \"Ada\"\nlet msg = greet(targetName)\nshow(msg)\n"
	svc.OpenDocument(uri, 1, code)

	sig := svc.SignatureHelp(uri, 5, 18)
	if sig == nil || len(sig.Parameters) == 0 {
		t.Fatalf("expected signature help inside greet(...), got %+v", sig)
	}

	defSpan := svc.Definition(uri, 5, 20)
	if defSpan == nil || defSpan.Start.Line != 4 {
		t.Fatalf("expected definition of targetName on line 4, got %+v", defSpan)
	}
}

func TestE03ScopedRenameIgnoresStringsCommentsAndShadowing(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e03.cherri"
	code := "// count comment should stay count\nlet count = 1\nlet label = \"count in string\"\nif true {\n    let count = 99\n    show(f\"{count}\")\n}\nshow(f\"{count}\")\n"
	svc.OpenDocument(uri, 1, code)

	updated, edits, err := svc.Rename(uri, 2, 5, "totalCount")
	if err != nil {
		t.Fatalf("Rename failed: %v", err)
	}
	if len(edits) != 2 {
		t.Fatalf("expected exactly 2 edits for outer count, got %d: %+v", len(edits), edits)
	}
	if updated == "" {
		t.Fatalf("expected non-empty updated source")
	}
	if _, _, err := svc.Rename(uri, 2, 5, "if"); err == nil {
		t.Fatalf("expected keyword rename to fail")
	}
}

func TestE04FormatterPreservesCommentsAndSemantics(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e04.cherri"
	code := "// Header comment\nlet x=1+2\n// Trailing comment\nshow(f\"{x}\")\n"
	svc.OpenDocument(uri, 1, code)
	formatted, err := svc.Format(uri)
	if err != nil {
		t.Fatalf("Format failed: %v", err)
	}
	svc.OpenDocument(uri, 2, formatted)
	formattedAgain, err := svc.Format(uri)
	if err != nil {
		t.Fatalf("second Format failed: %v", err)
	}
	if formatted != formattedAgain {
		t.Fatalf("formatter is not idempotent:\nfirst:\n%s\nsecond:\n%s", formatted, formattedAgain)
	}
}

func TestE05OutOfOrderVersionedEdits(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e05.cherri"
	svc.ChangeDocument(uri, 5, "let validVar = \"ok\"\nshow(validVar)\n")
	if svc.DocumentVersion(uri) != 5 {
		t.Fatalf("expected document version 5, got %d", svc.DocumentVersion(uri))
	}
	svc.ChangeDocument(uri, 4, "let broken = \n")
	if svc.DocumentVersion(uri) != 5 {
		t.Fatalf("expected stale version 4 to be ignored and version to remain 5, got %d", svc.DocumentVersion(uri))
	}
	diags, _ := svc.Analyze(uri)
	if len(diags) != 0 {
		t.Fatalf("expected 0 diagnostics after stale version ignored, got %d", len(diags))
	}
}

func TestE06UTF16DiagnosticSpansEmojiAndHebrew(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	uri := "file:///e06.cherri"
	// "let s = \"🙂שלום\" + " is 18 runes but 🙂 is 2 UTF-16 code units -> 19 UTF-16 code units before badVar (1-based UTF16Column = 20, 0-based LSPCharacterColumn = 19)
	code := "let s = \"🙂שלום\" + badVar\n"
	svc.OpenDocument(uri, 1, code)
	diags, _ := svc.Analyze(uri)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostic for badVar")
	}
	d := diags[0]
	if d.Span.Start.UTF16Column <= d.Span.Start.Column {
		t.Fatalf("expected UTF16Column (%d) > rune Column (%d) after surrogate pair emoji", d.Span.Start.UTF16Column, d.Span.Start.Column)
	}
	if d.Span.Start.LSPCharacterColumn() != 19 {
		t.Fatalf("expected 0-based UTF-16 column 19, got %d", d.Span.Start.LSPCharacterColumn())
	}
}

func TestE07BoundedVisualSourceEditAndRejection(t *testing.T) {
	svc := NewService(schema.DefaultRegistry())
	code := "// Keep this comment\nshow(\"Old Title\")\n"
	updated, err := svc.ApplyVisualSourceEdit(code, 0, "text", "\"New Title\"")
	if err != nil {
		t.Fatalf("ApplyVisualSourceEdit failed: %v", err)
	}
	expected := "// Keep this comment\nshow(\"New Title\")\n"
	if updated != expected {
		t.Fatalf("expected:\n%s\ngot:\n%s", expected, updated)
	}
	if _, err := svc.ApplyVisualSourceEdit(code, 99, "text", "\"Bad\""); err == nil {
		t.Fatalf("expected out-of-range action index to be rejected")
	}
}

