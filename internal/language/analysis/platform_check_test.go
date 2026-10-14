package analysis

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func runAnalyzer(t *testing.T, code string) *Analyzer {
	t.Helper()
	file := source.NewFile("test.cherri", "file:///test.cherri", 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("unexpected parse errors: %v", parser.Errors())
	}
	reg := schema.DefaultRegistry()
	analyzer := NewAnalyzer(reg)
	analyzer.Analyze(prog)
	return analyzer
}

func TestMacActionInNonMacShortcut_Baseline(t *testing.T) {
	code := `shortcut "Test" {
    mac: false
}
makeDiskImage("test", "content")
`
	analyzer := runAnalyzer(t, code)
	hasPlatformError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			hasPlatformError = true
			break
		}
	}
	if !hasPlatformError {
		t.Errorf("FAIL: Expected CodeUnsupportedTarget for Mac action in non-Mac shortcut, but got none! Diagnostics: %v", analyzer.Diagnostics())
	}
}

func TestMacActionInMacShortcut_Allowed(t *testing.T) {
	code := `shortcut "Test" {
    mac: true
}
makeDiskImage("test", "content")
`
	analyzer := runAnalyzer(t, code)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			t.Errorf("FAIL: Unexpected CodeUnsupportedTarget for Mac action in Mac shortcut: %v", diag)
		}
	}
}

func TestNonMacActionInMacShortcut_Error(t *testing.T) {
	code := `shortcut "Test" {
    mac: true
}
vibrate()
`
	analyzer := runAnalyzer(t, code)
	hasPlatformError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			hasPlatformError = true
			break
		}
	}
	if !hasPlatformError {
		t.Errorf("FAIL: Expected CodeUnsupportedTarget for NonMac action in Mac shortcut, but got none! Diagnostics: %v", analyzer.Diagnostics())
	}
}

func TestNonMacActionInNonMacShortcut_Allowed(t *testing.T) {
	code := `shortcut "Test" {
    mac: false
}
vibrate()
`
	analyzer := runAnalyzer(t, code)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			t.Errorf("FAIL: Unexpected CodeUnsupportedTarget for NonMac action in non-Mac shortcut: %v", diag)
		}
	}
}

func TestEmptyDictionaryKey_MapLiteral(t *testing.T) {
	code := `let dict = {"": "value"}`
	analyzer := runAnalyzer(t, code)
	hasEmptyKeyError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeArgumentType && diag.Message == "empty dictionary key is not allowed" {
			hasEmptyKeyError = true
			break
		}
	}
	if !hasEmptyKeyError {
		t.Errorf("FAIL: Expected CodeArgumentType for empty key in map literal, got: %v", analyzer.Diagnostics())
	}
}

func TestEmptyDictionaryKey_IndexExpr(t *testing.T) {
	code := `let dict = {"valid": "val"}
let v = dict[""]
`
	analyzer := runAnalyzer(t, code)
	hasEmptyKeyError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeArgumentType && diag.Message == "empty dictionary key is not allowed" {
			hasEmptyKeyError = true
			break
		}
	}
	if !hasEmptyKeyError {
		t.Errorf("FAIL: Expected CodeArgumentType for empty key in index expression, got: %v", analyzer.Diagnostics())
	}
}

func TestEmptyDictionaryKey_GetValueCall(t *testing.T) {
	code := `let dict = {"valid": "val"}
getValue(dict, key: "")
`
	analyzer := runAnalyzer(t, code)
	hasEmptyKeyError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeArgumentType && diag.Message == "empty dictionary key is not allowed" {
			hasEmptyKeyError = true
			break
		}
	}
	if !hasEmptyKeyError {
		t.Errorf("FAIL: Expected CodeArgumentType for empty key in getValue call, got: %v", analyzer.Diagnostics())
	}
}

func TestExprTypesTracking(t *testing.T) {
	code := `let dict = {"hello": "world"}
let k = "hello"
let v = dict[k]
`
	analyzer := runAnalyzer(t, code)
	exprTypes := analyzer.ExprTypes()
	if len(exprTypes) == 0 {
		t.Fatalf("expected exprTypes to be populated, but got 0 entries")
	}

	foundMap := false
	foundText := false
	for _, tVal := range exprTypes {
		if tVal != nil {
			if tVal.Kind() == types.KindMap {
				foundMap = true
			}
			if tVal.Equals(types.Text) {
				foundText = true
			}
		}
	}
	if !foundMap {
		t.Errorf("expected to find KindMap in exprTypes")
	}
	if !foundText {
		t.Errorf("expected to find Text in exprTypes")
	}
}
