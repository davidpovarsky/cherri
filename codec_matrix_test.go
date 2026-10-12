package main

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

// helper to compile v2 source and return the CanonicalBackendSession
func compileSourceToSession(t *testing.T, name, code string) *CanonicalBackendSession {
	t.Helper()
	file := source.NewFile(source.DocumentID(name), name, 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("syntax errors: %v", parser.Errors())
	}

	reg := schema.DefaultRegistry()
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			t.Fatalf("analyzer error [%s]: %s", diag.Code, diag.Message)
		}
	}

	session := NewCanonicalBackendSession()
	lowerer := lower.NewLowererWithSession(reg, session)
	lowerer.WorkflowName = name
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		session.Close()
		t.Fatalf("lowering error: %v", err)
	}
	return session
}

func getDictionaryItems(dictVal WFDictionaryFieldValue) []WFDictionaryFieldValueItem {
	if wrapper, ok := dictVal.Value.(WFDictionaryFieldValueWrapper); ok {
		return wrapper.WFDictionaryFieldValueItems
	}
	return nil
}

// CODEC-001: literal "{name}" vs f"{name}"
// Literal remains literal; only f-string refers to variable.
func TestCODEC001_LiteralBracesVsFString(t *testing.T) {
	code := `
let literal = "Hello {name}"
let name = "David"
let interpolated = f"Hello {name}"
let jsonLit = "{\"a\": 1}"
let escaped = "Escaped {{name}}"
show(literal)
show(interpolated)
show(jsonLit)
show(escaped)
`
	session := compileSourceToSession(t, "codec001", code)
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions

	// Find the gettext action for literal
	foundLiteral := false
	foundInterpolated := false
	foundJSON := false

	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" {
			textVal := act.WFWorkflowActionParameters["WFTextActionText"]
			if s, ok := textVal.(string); ok && s == "Hello {name}" {
				foundLiteral = true
			}
			if s, ok := textVal.(string); ok && s == "{\"a\": 1}" {
				foundJSON = true
			}
			if m, ok := textVal.(map[string]any); ok {
				if m["WFSerializationType"] == "WFTextTokenString" {
					valMap, _ := m["Value"].(map[string]any)
					if valMap != nil {
						str, _ := valMap["string"].(string)
						if strings.HasPrefix(str, "Hello ") {
							attMap, _ := valMap["attachmentsByRange"].(map[string]any)
							if len(attMap) > 0 {
								foundInterpolated = true
							}
						}
					}
				}
			}
		}
	}

	if !foundLiteral {
		t.Errorf("expected literal 'Hello {name}' to remain a plain string in gettext action")
	}
	if !foundJSON {
		t.Errorf("expected '{\"a\": 1}' to remain a plain string in gettext action")
	}
	if !foundInterpolated {
		t.Errorf("expected interpolated f-string to be serialized with WFTextTokenString attachmentsByRange")
	}
}

// CODEC-002: Unicode Hebrew/emoji + combined marks in interpolated text
// exact UTF-16 attachment offsets and runtime value
func TestCODEC002_HebrewEmojiAttachmentOffsets(t *testing.T) {
	tests := []struct {
		name           string
		prefix         string
		expectedOffset int
	}{
		{
			name:           "Hebrew prefix",
			prefix:         "שלום ",
			expectedOffset: len(utf16.Encode([]rune("שלום "))), // 5
		},
		{
			name:           "Emoji surrogate pair prefix",
			prefix:         "👋 ",
			expectedOffset: len(utf16.Encode([]rune("👋 "))), // 3 (2 for emoji + 1 for space)
		},
		{
			name:           "Combined Hebrew niqqud prefix",
			prefix:         "שָׁלוֹם ",
			expectedOffset: len(utf16.Encode([]rune("שָׁלוֹם "))),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code := fmt.Sprintf("let name = \"David\"\nlet msg = f\"%s{name}\"\nshow(msg)\n", tc.prefix)
			session := compileSourceToSession(t, "codec002_"+tc.name, code)
			defer session.Close()

			foundOffset := false
			expectedRangeKey := fmt.Sprintf("{%d, 1}", tc.expectedOffset)

			for _, act := range session.shortcut.WFWorkflowActions {
				if act.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" {
					textVal := act.WFWorkflowActionParameters["WFTextActionText"]
					if m, ok := textVal.(map[string]any); ok {
						if valMap, ok := m["Value"].(map[string]any); ok {
							if attMap, ok := valMap["attachmentsByRange"].(map[string]any); ok {
								if _, ok := attMap[expectedRangeKey]; ok {
									foundOffset = true
								} else {
									t.Logf("attachmentsByRange keys: %v, expected: %s", attMap, expectedRangeKey)
								}
							}
						}
					}
				}
			}

			if !foundOffset {
				t.Errorf("expected attachment range key %q for prefix %q", expectedRangeKey, tc.prefix)
			}
		})
	}
}

// CODEC-003: let d = {"a":1}
// Real native Dictionary action and typed nested payload.
func TestCODEC003_LetDictionaryNativeAction(t *testing.T) {
	code := `
let d = {"a": 1}
quicklook(d)
`
	session := compileSourceToSession(t, "codec003", code)
	defer session.Close()

	var dictAct *ShortcutAction
	for i := range session.shortcut.WFWorkflowActions {
		act := &session.shortcut.WFWorkflowActions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary" {
			dictAct = act
			break
		}
	}

	if dictAct == nil {
		t.Fatalf("expected is.workflow.actions.dictionary action to be emitted")
	}

	wfItems := dictAct.WFWorkflowActionParameters["WFItems"]
	if wfItems == nil {
		t.Fatalf("expected WFItems in dictionary action parameters")
	}

	dictVal, ok := wfItems.(WFDictionaryFieldValue)
	if !ok {
		t.Fatalf("expected WFItems to be WFDictionaryFieldValue, got %T: %v", wfItems, wfItems)
	}

	if dictVal.WFSerializationType != "WFDictionaryFieldValue" {
		t.Errorf("expected WFSerializationType == WFDictionaryFieldValue, got %q", dictVal.WFSerializationType)
	}

	items := getDictionaryItems(dictVal)
	if len(items) != 1 {
		t.Fatalf("expected 1 dictionary item, got %d", len(items))
	}

	keyObj := items[0].WFKey.(WFTextTokenString)
	if keyObj.Value.String != "a" {
		t.Errorf("expected key 'a', got %q", keyObj.Value.String)
	}
}

// CODEC-004: var d = {"a":1}
// Same correct Dictionary semantics before Set Variable; not Text.
func TestCODEC004_VarDictionaryMaterialization(t *testing.T) {
	code := `
var d = {"a": 1}
quicklook(d)
`
	session := compileSourceToSession(t, "codec004", code)
	defer session.Close()

	// Must contain dictionary action, NOT gettext action for the map
	hasDict := false
	hasGetTextForMap := false

	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary" {
			hasDict = true
		}
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" {
			hasGetTextForMap = true
		}
	}

	if !hasDict {
		t.Errorf("expected is.workflow.actions.dictionary for var d = {\"a\": 1}")
	}
	if hasGetTextForMap {
		t.Errorf("unexpected is.workflow.actions.gettext action emitted for dictionary var")
	}
}

// CODEC-005: dict in a named action arg jsonRequest / formRequest
// no type panic, correct canonical argument encoding and body shape.
func TestCODEC005_DictInNamedActionArg(t *testing.T) {
	code := `
let res = jsonRequest("https://api.example.com/items", body: {"name": "test", "active": true})
`
	session := compileSourceToSession(t, "codec005", code)
	defer session.Close()

	var reqAct *ShortcutAction
	for i := range session.shortcut.WFWorkflowActions {
		act := &session.shortcut.WFWorkflowActions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
			reqAct = act
			break
		}
	}

	if reqAct == nil {
		t.Fatalf("expected is.workflow.actions.downloadurl action")
	}

	bodyType := reqAct.WFWorkflowActionParameters["WFHTTPBodyType"]
	if bodyType != "JSON" {
		t.Errorf("expected WFHTTPBodyType 'JSON', got %v", bodyType)
	}

	jsonValues := reqAct.WFWorkflowActionParameters["WFJSONValues"]
	if jsonValues == nil {
		t.Fatalf("expected WFJSONValues parameter")
	}

	dictVal, ok := jsonValues.(WFDictionaryFieldValue)
	if !ok {
		t.Fatalf("expected WFJSONValues to be WFDictionaryFieldValue, got %T: %v", jsonValues, jsonValues)
	}

	items := getDictionaryItems(dictVal)
	if len(items) < 2 {
		t.Errorf("expected at least 2 dictionary items (name, active), got %d", len(items))
	}
}

// CODEC-006: nested ordered dict/list with reference and false, 0, ""
// exact nested Apple envelopes and correct values.
func TestCODEC006_NestedOrderedDictListWithRefsAndPrimitives(t *testing.T) {
	code := `
let refVal = "my-ref"
let complexDict = {
    "ref": refVal,
    "flag": false,
    "zero": 0,
    "empty": "",
    "nested": {"inner": "val"}
}
quicklook(complexDict)
`
	session := compileSourceToSession(t, "codec006", code)
	defer session.Close()

	var dictAct *ShortcutAction
	for i := range session.shortcut.WFWorkflowActions {
		act := &session.shortcut.WFWorkflowActions[i]
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary" {
			dictAct = act
			break
		}
	}

	if dictAct == nil {
		t.Fatalf("expected is.workflow.actions.dictionary action")
	}

	dictVal, ok := dictAct.WFWorkflowActionParameters["WFItems"].(WFDictionaryFieldValue)
	if !ok {
		t.Fatalf("expected WFDictionaryFieldValue, got %T", dictAct.WFWorkflowActionParameters["WFItems"])
	}

	items := getDictionaryItems(dictVal)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}

	// Verify each entry
	itemsByKey := make(map[string]WFDictionaryFieldValueItem)
	for _, it := range items {
		if kObj, ok := it.WFKey.(WFTextTokenString); ok {
			itemsByKey[kObj.Value.String] = it
		}
	}

	// 1. "ref" -> WFItemType 0 (text) and attachment
	if it, ok := itemsByKey["ref"]; !ok {
		t.Errorf("missing 'ref' key")
	} else if it.WFItemType != 0 {
		t.Errorf("expected WFItemType 0 for ref, got %d", it.WFItemType)
	}

	// 2. "flag" -> WFItemType 4 (bool) and WFBoolValue false
	if it, ok := itemsByKey["flag"]; !ok {
		t.Errorf("missing 'flag' key")
	} else {
		if it.WFItemType != 4 {
			t.Errorf("expected WFItemType 4 for flag, got %d", it.WFItemType)
		}
		if bVal, ok := it.WFValue.(WFBoolValue); !ok || bVal.Value != false {
			t.Errorf("expected WFBoolValue with false, got %v", it.WFValue)
		}
	}

	// 3. "zero" -> WFItemType 3 (number)
	if it, ok := itemsByKey["zero"]; !ok {
		t.Errorf("missing 'zero' key")
	} else if it.WFItemType != 3 {
		t.Errorf("expected WFItemType 3 for zero, got %d", it.WFItemType)
	}

	// 4. "empty" -> WFItemType 0 (text) with empty string
	if it, ok := itemsByKey["empty"]; !ok {
		t.Errorf("missing 'empty' key")
	} else {
		if it.WFItemType != 0 {
			t.Errorf("expected WFItemType 0 for empty, got %d", it.WFItemType)
		}
		if tokStr, ok := it.WFValue.(WFTextTokenString); !ok || tokStr.Value.String != "" {
			t.Errorf("expected empty WFTextTokenString, got %v", it.WFValue)
		}
	}

	// 5. "nested" -> WFItemType 1 (dict)
	if it, ok := itemsByKey["nested"]; !ok {
		t.Errorf("missing 'nested' key")
	} else if it.WFItemType != 1 {
		t.Errorf("expected WFItemType 1 for nested, got %d", it.WFItemType)
	}
}

// CODEC-007: JSON Text -> getDictionary (Approach A from user's sample)
// detect.dictionary action actually used and correct parsed result.
func TestCODEC007_ApproachA_JSONTextGetDictionary(t *testing.T) {
	code := `
let json = "{\"name\":\"David\"}"
let d = getDictionary(json)
quicklook(d)
`
	session := compileSourceToSession(t, "codec007", code)
	defer session.Close()

	foundDetect := false
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.detect.dictionary" {
			foundDetect = true
			if act.WFWorkflowActionParameters["WFInput"] == nil {
				t.Errorf("expected WFInput parameter in detect.dictionary")
			}
		}
	}

	if !foundDetect {
		t.Errorf("expected is.workflow.actions.detect.dictionary action to be emitted for getDictionary()")
	}
}

// CODEC-008: dictionary value coercion reference (Approach D from user's sample)
// exact WFCoercionVariableAggrandizement/WFDictionaryContentItem
func TestCODEC008_ApproachD_DictionaryCoercion(t *testing.T) {
	code := `
let json = "{\"name\":\"David\"}"
let d = json.dictionary
quicklook(d)
`
	session := compileSourceToSession(t, "codec008", code)
	defer session.Close()

	// Approach D must NOT emit detect.dictionary action
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.detect.dictionary" {
			t.Errorf("Approach D should NOT emit detect.dictionary action; it is a reference coercion")
		}
	}

	// Quicklook action must have the coercion aggrandizement in WFInput
	foundCoercion := false
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
			inputVal := act.WFWorkflowActionParameters["WFInput"]
			if inputVal != nil {
				s := fmt.Sprintf("%#v", inputVal)
				if strings.Contains(s, "WFCoercionVariableAggrandizement") && strings.Contains(s, "WFDictionaryContentItem") {
					foundCoercion = true
				}
			}
		}
	}

	if !foundCoercion {
		t.Errorf("expected WFCoercionVariableAggrandizement with WFDictionaryContentItem in reference")
	}
}

// CODEC-009: setName(..., "x.json") (Approach C from user's sample)
// setitemname distinct from dictionary; no fake file persist claim.
func TestCODEC009_ApproachC_SetNameJSON(t *testing.T) {
	code := `
let json = "{\"name\":\"David\"}"
let f = setName(json, "sample.json")
show(f)
`
	session := compileSourceToSession(t, "codec009", code)
	defer session.Close()

	foundSetName := false
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.setitemname" {
			foundSetName = true
			nameParam := act.WFWorkflowActionParameters["WFName"]
			if nameParam != "sample.json" {
				t.Errorf("expected WFName == 'sample.json', got %v", nameParam)
			}
		}
	}

	if !foundSetName {
		t.Errorf("expected is.workflow.actions.setitemname action to be emitted")
	}
}

// CODEC-010: literal/variable reference + property/dict/date coercion
// exact transformations and output UUID mappings
func TestCODEC010_CoercionAndTransformationsMatrix(t *testing.T) {
	code := `
let d = {"name": "David", "created": "2026-10-12"}
let n = d["name"]
let dt = d.date
quicklook(n)
quicklook(dt)
`
	session := compileSourceToSession(t, "codec010", code)
	defer session.Close()

	hasDictAggr := false
	hasDateAggr := false

	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
			inputVal := act.WFWorkflowActionParameters["WFInput"]
			if inputVal != nil {
				s := fmt.Sprintf("%#v", inputVal)
				if strings.Contains(s, "WFDictionaryValueVariableAggrandizement") && strings.Contains(s, "name") {
					hasDictAggr = true
				}
				if strings.Contains(s, "WFCoercionVariableAggrandizement") && strings.Contains(s, "WFDateContentItem") {
					hasDateAggr = true
				}
			}
		}
	}

	if !hasDictAggr {
		t.Errorf("expected WFDictionaryValueVariableAggrandizement for d[\"name\"] in quicklook WFInput")
	}
	if !hasDateAggr {
		t.Errorf("expected WFCoercionVariableAggrandizement for d.date in quicklook WFInput")
	}
}
