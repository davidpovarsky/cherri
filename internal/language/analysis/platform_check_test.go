package analysis

import (
	"strings"
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

func TestMixedTargets_RejectsNonPortableActions(t *testing.T) {
	// Mixed targets ["macos", "ios"]: both MacOnly (makeDiskImage) and NonMacOnly (vibrate) must fail
	code := `shortcut "CrossPlatform" {
    targets: ["macos", "ios"]
}
makeDiskImage("test", "content")
vibrate()
`
	analyzer := runAnalyzer(t, code)
	macError := false
	nonMacError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			if strings.Contains(diag.Message, "makeDiskImage") {
				macError = true
			}
			if strings.Contains(diag.Message, "vibrate") {
				nonMacError = true
			}
		}
	}
	if !macError {
		t.Errorf("expected CodeUnsupportedTarget for makeDiskImage on mixed target, got diagnostics: %v", analyzer.Diagnostics())
	}
	if !nonMacError {
		t.Errorf("expected CodeUnsupportedTarget for vibrate on mixed target, got diagnostics: %v", analyzer.Diagnostics())
	}
}

func TestExplicitTargetString(t *testing.T) {
	code := `shortcut "IOSOnly" {
    target: "ios"
}
makeDiskImage("test", "content")
`
	analyzer := runAnalyzer(t, code)
	foundError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("expected CodeUnsupportedTarget for makeDiskImage targeting ios")
	}
}

func TestUnspecifiedTarget_EmitsWarning(t *testing.T) {
	// When target is unspecified, platform-specific actions emit a warning rather than breaking
	code := `makeDiskImage("test", "content")`
	analyzer := runAnalyzer(t, code)
	foundWarning := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == SeverityWarning && diag.Code == CodeUnverifiedForm {
			foundWarning = true
			break
		}
	}
	if !foundWarning {
		t.Errorf("expected SeverityWarning for platform-specific action with unspecified target, got: %v", analyzer.Diagnostics())
	}
}

func TestVersionBounds_MinVersion(t *testing.T) {
	// askModel has MinVersion 26
	codeLow := `shortcut "LowVersion" {
    target: "ios"
    minVersion: 15
}
askModel("prompt")
`
	analyzerLow := runAnalyzer(t, codeLow)
	foundError := false
	for _, diag := range analyzerLow.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget && strings.Contains(diag.Message, "minimum version") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("expected CodeUnsupportedTarget for minVersion lower than action requirement, got: %v", analyzerLow.Diagnostics())
	}

	codeHigh := `shortcut "HighVersion" {
    target: "ios"
    minVersion: 27
}
askModel("prompt")
`
	analyzerHigh := runAnalyzer(t, codeHigh)
	for _, diag := range analyzerHigh.Diagnostics() {
		if diag.Code == CodeUnsupportedTarget {
			t.Errorf("unexpected CodeUnsupportedTarget when minVersion satisfies requirement: %v", diag)
		}
	}
}

func TestAcceptedForms_RejectsLiteralForAttachmentOnlyParam(t *testing.T) {
	// addEvent parameter 'startDate' requires attachment, rejects literal
	code := `addEvent("Meeting", startDate: "tomorrow")`
	analyzer := runAnalyzer(t, code)
	foundError := false
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeArgumentType && strings.Contains(diag.Message, "does not accept literal values") {
			foundError = true
			break
		}
	}
	if !foundError {
		t.Errorf("expected CodeArgumentType rejecting literal for attachment-only parameter, got: %v", analyzer.Diagnostics())
	}
}

func TestAcceptedForms_AcceptsVariableAttachment(t *testing.T) {
	code := `let d = date()
addEvent("Meeting", startDate: d)
`
	analyzer := runAnalyzer(t, code)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Code == CodeArgumentType && strings.Contains(diag.Message, "does not accept literal values") {
			t.Errorf("unexpected argument type error for valid variable attachment: %v", diag)
		}
	}
}
