package schema

import (
	"testing"
)

func TestDefaultRegistry(t *testing.T) {
	reg := DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	actions := reg.AllActions()
	if len(actions) != 461 {
		t.Fatalf("expected 461 actions, got %d", len(actions))
	}

	fp := reg.Fingerprint()
	if len(fp) != 64 {
		t.Fatalf("expected 64-char hex fingerprint, got %q", fp)
	}

	// Verify resizeImage
	resize, ok := reg.LookupAction("resizeImage")
	if !ok {
		t.Fatalf("action resizeImage not found")
	}
	if resize.PrimaryParameterID != "image" {
		t.Errorf("expected primaryParameterId 'image', got %q", resize.PrimaryParameterID)
	}
	if resize.OutputTypeName != "Image" {
		t.Errorf("expected output type 'Image', got %q", resize.OutputTypeName)
	}

	// Verify enums
	langs, ok := reg.LookupEnum("translationLanguage")
	if !ok || len(langs) == 0 {
		t.Errorf("expected translationLanguage enum to have values")
	}
}
