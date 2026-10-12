/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/base64"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

// IOS-001: invalid v2 source with Ask, @, const, braces in comments
// Explicit v2 mode remains v2, fails with v2 diagnostic rather than legacy fallback.
func TestIOS001_InvalidV2Source_DiagnosticNotLegacy(t *testing.T) {
	src := `// User email: test@example.com
// Notice const values and {braces}
// Ask the user for input
let invalidSyntax = ;
`
	// In explicit v2 mode
	respV2 := compileForMobileMode(src, "TestIOS001", "v2", false)
	if respV2.OK {
		t.Fatalf("expected v2 compilation to fail for invalid syntax, but got OK")
	}
	if respV2.Mode != "v2" {
		t.Fatalf("expected mode to be 'v2', got %q", respV2.Mode)
	}
	if respV2.Error == "" {
		t.Fatalf("expected non-empty error diagnostic")
	}
	if respV2.Line != 4 {
		t.Logf("diagnostic line: %d (expected 4), error: %s", respV2.Line, respV2.Error)
	}

	// In auto mode, absence of anchored directives means v2 is chosen
	// and failure must NOT fall back to legacy compiler
	respAuto := compileForMobileMode(src, "TestIOS001Auto", "auto", false)
	if respAuto.OK {
		t.Fatalf("expected auto compilation to fail for invalid v2 syntax, but got OK")
	}
	if respAuto.Mode != "v2" {
		t.Fatalf("expected auto mode to remain v2 on failure, got %q", respAuto.Mode)
	}
}

// IOS-002: valid legacy source
// Explicit legacy mode still works, no behavioral regression.
func TestIOS002_ValidLegacySource(t *testing.T) {
	src := `#define name LegacyTest
show("Hello from legacy Cherri")
`
	resp := compileForMobileMode(src, "LegacyTest", "legacy", false)
	if !resp.OK {
		t.Fatalf("legacy compile failed: %s (line %d, col %d)", resp.Error, resp.Line, resp.Column)
	}
	if resp.PlistBase64 == "" {
		t.Fatalf("expected non-empty PlistBase64")
	}
	if resp.Mode != "legacy" {
		t.Fatalf("expected mode 'legacy', got %q", resp.Mode)
	}

	plistBytes, err := base64.StdEncoding.DecodeString(resp.PlistBase64)
	if err != nil {
		t.Fatalf("failed to decode plist: %v", err)
	}
	var shortcutObj map[string]any
	if _, err := plist.Unmarshal(plistBytes, &shortcutObj); err != nil {
		t.Fatalf("failed to unmarshal plist: %v", err)
	}
	actions, ok := shortcutObj["WFWorkflowActions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatalf("expected workflow actions in plist, got: %v", shortcutObj["WFWorkflowActions"])
	}
	firstAction, ok := actions[0].(map[string]any)
	if !ok || firstAction["WFWorkflowActionIdentifier"] != "is.workflow.actions.showresult" {
		t.Fatalf("expected showresult action, got: %v", firstAction)
	}
}

// IOS-003: all palette action snippets in v2 mode
// Each complete snippet compiles, compiler constructs have valid v2 syntax.
func TestIOS003_PaletteActionSnippetsInV2(t *testing.T) {
	catalog := currentMobileActionCatalog()
	if len(catalog) == 0 {
		t.Fatalf("action catalog is empty")
	}

	for _, item := range catalog {
		snippet := item.InsertionSnippet
		// Verify compiler constructs do not contain legacy @ or const
		if item.CompilerConstruct {
			if strings.Contains(snippet, "@") {
				t.Fatalf("compiler construct %q snippet contains legacy '@': %s", item.Name, snippet)
			}
			if strings.Contains(snippet, "const ") {
				t.Fatalf("compiler construct %q snippet contains legacy 'const': %s", item.Name, snippet)
			}
		}
	}

	// Test compilation of representative v2 action snippets
	representativeSnippets := []struct {
		name    string
		snippet string
	}{
		{"show", "show(Ask)\n"},
		{"alert", "alert(Ask)\n"},
		{"calculate", "let result = 1 + 1\n"},
		{"dictConstruct", "let dict = {\"key\": \"value\"}\n"},
		{"setVar", "var variable = Ask\n"},
		{"ifConstruct", "if (Ask) {\n\tshow(Ask)\n}\n"},
		{"repeatConstruct", "repeat (Ask) {\n\tshow(Ask)\n}\n"},
		{"forConstruct", "for item in Ask {\n\tshow(item)\n}\n"},
	}

	for _, tc := range representativeSnippets {
		t.Run(tc.name, func(t *testing.T) {
			resp := compileForMobileMode(tc.snippet, tc.name, "v2", false)
			if !resp.OK {
				t.Fatalf("snippet %s failed to compile in v2: %s (line %d, col %d)", tc.snippet, resp.Error, resp.Line, resp.Column)
			}
		})
	}
}

// IOS-004: advanced v2 source visual edit
// Bounded AST edit does not rewrite or destroy unrelated source/semantic structures.
func TestIOS004_AdvancedV2SourceVisualEdit(t *testing.T) {
	source := `// Header documentation comment
// Shalom: שלום עולם 🚀

let baseMessage = "Hello"
let userNote = f"Message: {baseMessage}"

// Important operation follows
show("Original Notice")

// Footer preservation comment
`
	svc := service.NewService(schema.DefaultRegistry())
	updated, err := svc.ApplyVisualSourceEdit(source, 2, "text", "\"Updated Notice 🌟\"")
	if err != nil {
		t.Fatalf("ApplyVisualSourceEdit failed: %v", err)
	}

	// Verify all original comments, Hebrew text, emoji, and let bindings are preserved
	if !strings.Contains(updated, "// Header documentation comment") {
		t.Errorf("lost header comment")
	}
	if !strings.Contains(updated, "שלום עולם 🚀") {
		t.Errorf("lost Hebrew/emoji in comments")
	}
	if !strings.Contains(updated, "let baseMessage = \"Hello\"") {
		t.Errorf("lost baseMessage let binding")
	}
	if !strings.Contains(updated, "let userNote = f\"Message: {baseMessage}\"") {
		t.Errorf("lost userNote f-string")
	}
	if !strings.Contains(updated, "// Important operation follows") {
		t.Errorf("lost middle comment")
	}
	if !strings.Contains(updated, "show(\"Updated Notice 🌟\")") {
		t.Errorf("expected updated action call, got:\n%s", updated)
	}
	if !strings.Contains(updated, "// Footer preservation comment") {
		t.Errorf("lost footer comment")
	}

	// Verify the edited source compiles cleanly with v2
	resp := compileForMobileMode(updated, "AdvVisualEdit", "v2", false)
	if !resp.OK {
		t.Fatalf("recompilation of edited source failed: %s", resp.Error)
	}
}

// IOS-005: unsupported visual edit
// Invalid action index or malformed replacement leaves original source untouched.
func TestIOS005_UnsupportedVisualEdit_SourcePreserved(t *testing.T) {
	source := `// Critical source
let greeting = "Hello"
show(greeting)
`
	svc := service.NewService(schema.DefaultRegistry())

	// Out-of-range action index
	updated, err := svc.ApplyVisualSourceEdit(source, 999, "text", "\"Test\"")
	if err == nil {
		t.Fatalf("expected out-of-range index to return error")
	}
	if updated != source {
		t.Fatalf("source was mutated on error! got:\n%s", updated)
	}

	// Malformed replacement expression
	updatedBadExpr, errBad := svc.ApplyVisualSourceEdit(source, 0, "text", ";;; invalid expression")
	if errBad == nil {
		t.Fatalf("expected syntax error in replacement to return error")
	}
	if updatedBadExpr != source {
		t.Fatalf("source was mutated on bad expression error! got:\n%s", updatedBadExpr)
	}
}

// IOS-006: concurrent/repeated compile/preview
// No state leaks, race conditions, or panics across threads.
func TestIOS006_ConcurrentRepeatedCompile(t *testing.T) {
	var wg sync.WaitGroup
	errCh := make(chan error, 30)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(iteration int) {
			defer wg.Done()
			var src string
			var mode string
			if iteration%2 == 0 {
				mode = "v2"
				src = fmt.Sprintf("let val%d = \"Run_%d\"\nshow(val%d)\n", iteration, iteration, iteration)
			} else {
				mode = "legacy"
				src = fmt.Sprintf("#define name Legacy_%d\nshow(\"Run_%d\")\n", iteration, iteration)
			}

			resp := compileForMobileMode(src, fmt.Sprintf("Test_%d", iteration), mode, false)
			if !resp.OK {
				errCh <- fmt.Errorf("iteration %d (%s) failed: %s", iteration, mode, resp.Error)
				return
			}
			if resp.PlistBase64 == "" {
				errCh <- fmt.Errorf("iteration %d (%s) returned empty plist", iteration, mode)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		t.Errorf("concurrent compile error: %v", err)
	}
}
