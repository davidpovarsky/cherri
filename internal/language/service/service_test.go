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
