package main

import (
	"github.com/electrikmilk/cherri/internal/language/schema"
	"strings"
	"testing"
)

func TestGeneratedReferenceIncludesSeparateMacroContracts(t *testing.T) {
	reg := schema.DefaultRegistry()
	reference := renderReference(reg)
	for _, want := range []string{"**Total Actions:** 505", "**Compiler Macro Fingerprint:** " + quoteCode(reg.CompilerMacroFingerprint()), "## Compiler macros", "### " + quoteCode("require"), "### " + quoteCode("measure"), "V2 scoped block"} {
		if !strings.Contains(reference, want) {
			t.Fatalf("reference missing %s", want)
		}
	}
	for _, macro := range reg.CompilerMacros() {
		if !strings.Contains(reference, buildSignature(macro)) {
			t.Fatalf("signature missing for%s", macro.CallableName)
		}
	}
	if reference != renderReference(reg) {
		t.Fatal("generation is not deterministic")
	}
}

func quoteCode(s string) string { return string(rune(96)) + s + string(rune(96)) }
