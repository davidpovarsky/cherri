package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func debugParamMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	encoded, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(encoded, &m); err != nil {
		return nil
	}
	return m
}

func compileDebugFixture(t *testing.T, src string, legacy bool) []ShortcutAction {
	t.Helper()
	var root map[string]any
	var err error
	if legacy {
		root, err = compileLegacySourceToRawMapInMemory(src)
	} else {
		var raw []byte
		raw, err = CompileSourceToPlist("debug-stdlib-test.cherri", src)
		if err == nil {
			root, err = decodeShortcutRawMap(raw)
		}
	}
	if err != nil {
		t.Fatalf("compile debug fixture (legacy=%t): %v", legacy, err)
	}
	wireActions, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatalf("wire has no WFWorkflowActions: %T", root["WFWorkflowActions"])
	}
	acts := make([]ShortcutAction, 0, len(wireActions))
	for i, v := range wireActions {
		m, ok := v.(map[string]any)
		if !ok {
			t.Fatalf("action %d not dict: %T", i, v)
		}
		id, _ := m["WFWorkflowActionIdentifier"].(string)
		params, _ := m["WFWorkflowActionParameters"].(map[string]any)
		acts = append(acts, ShortcutAction{WFWorkflowActionIdentifier: id, WFWorkflowActionParameters: params})
	}
	return acts
}

func actionSuffixes(acts []ShortcutAction) []string {
	names := make([]string, len(acts))
	for i, act := range acts {
		names[i] = strings.TrimPrefix(act.WFWorkflowActionIdentifier, "is.workflow.actions.")
	}
	return names
}

func TestDebugStdlibCanonicalCompilerSharedSequence(t *testing.T) {
	legacy := `debugClearLogs()
text("alpha")
debugLog()
text("beta")
debugLog()
debugShowLogs()
`
	v2 := `debugClearLogs()
text("alpha")
debugLog()
text("beta")
debugLog()
debugShowLogs()
`
	want := []string{
		"gettext", "setvariable",
		"gettext", "appendvariable",
		"gettext", "appendvariable",
		"previewdocument",
	}
	for _, tc := range []struct {
		name, src string
		legacy    bool
	}{
		{"legacy", legacy, true},
		{"v2", v2, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := compileDebugFixture(t, tc.src, tc.legacy)
			got := actionSuffixes(acts)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("wrong emitted action sequence: got %v; want %v", got, want)
			}
			if acts[1].WFWorkflowActionParameters["WFVariableName"] != debugLogVariableName {
				t.Fatalf("clear did not target debug var: %v", acts[1].WFWorkflowActionParameters)
			}
			if acts[3].WFWorkflowActionParameters["WFVariableName"] != debugLogVariableName ||
				acts[5].WFWorkflowActionParameters["WFVariableName"] != debugLogVariableName {
				t.Fatal("logging does not append to stable local debug variable")
			}
			for i, a := range acts {
				if strings.Contains(strings.ToLower(a.WFWorkflowActionIdentifier), "storecontent") ||
					strings.Contains(strings.ToLower(a.WFWorkflowActionIdentifier), "runworkflow") ||
					strings.Contains(strings.ToLower(a.WFWorkflowActionIdentifier), "runshortcut") {
					t.Fatalf("unexpected persistent storage or Run Self at action %d: %s", i, a.WFWorkflowActionIdentifier)
				}
			}
			for _, idx := range []int{3, 5} {
				a := debugParamMap(acts[idx].WFWorkflowActionParameters["WFInput"])
				if a == nil || a["WFSerializationType"] != "WFTextTokenAttachment" {
					t.Fatalf("literal debug log %d missing serialized magic-variable WFInput: %v", idx, a)
				}
				d := debugParamMap(a["Value"])
				if d["Type"] != "ActionOutput" || d["OutputUUID"] != acts[idx-1].WFWorkflowActionParameters["UUID"] || d["OutputName"] != "Text" {
					t.Fatalf("literal debug log %d not bound to its Text producer: %v", idx, d)
				}
			}
			quick := debugParamMap(acts[6].WFWorkflowActionParameters["WFInput"])
			if quick == nil {
				t.Fatal("Quick Look has no log variable reference")
			}
			v := debugParamMap(quick["Value"])
			if v["Type"] != "Variable" || v["VariableName"] != debugLogVariableName {
				t.Errorf("Quick Look references wrong variable: %v", quick)
			}
		})
	}
}

func TestDebugStdlibNoArgumentRequiresBoundSource(t *testing.T) {
	if _, err := buildDebugStdlibActions("debugLog", map[string]any{}, "", ""); err == nil {
		t.Fatal("standalone no-arg builder must reject missing native WFInput")
	}
	for _, legacy := range []bool{true, false} {
		acts := compileDebugFixture(t, "text(\"hello\")\ndebugLog()\n", legacy)
		checkDebugInputReference(t, acts, legacy)
	}
}

// checkDebugInputReference validates both the source UUID and the actual
// plist token consumed by Add to Variable. This would have caught the iPad bug.
func checkDebugInputReference(t *testing.T, acts []ShortcutAction, legacy bool) {
	t.Helper()
	if len(acts) != 2 {
		t.Fatalf("legacy=%v: expected two native actions, got %d", legacy, len(acts))
	}
	if acts[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		acts[1].WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" {
		t.Fatalf("legacy=%v: unexpected action IDs: %v", legacy, actionSuffixes(acts))
	}
	prevID, _ := acts[0].WFWorkflowActionParameters["UUID"].(string)
	if prevID == "" {
		t.Fatal("previous action UUID is empty")
	}
	if acts[1].WFWorkflowActionParameters["WFVariableName"] != debugLogVariableName {
		t.Fatal("wrong log variable")
	}
	ref := debugParamMap(acts[1].WFWorkflowActionParameters["WFInput"])
	if ref == nil || ref["WFSerializationType"] != "WFTextTokenAttachment" {
		t.Fatalf("legacy=%v: missing valid serialized WFInput: %v", legacy, ref)
	}
	value := debugParamMap(ref["Value"])
	if value == nil || value["Type"] != "ActionOutput" || value["OutputUUID"] != prevID {
		t.Fatalf("legacy=%v: WFInput does not point to preceding UUID; value=%v, UUID=%v", legacy, value, prevID)
	}
	if value["OutputName"] != "Text" {
		t.Fatalf("legacy=%v: wrong output name: %v", legacy, value)
	}
}

func TestDebugStdlibOutputUUIDIsPreserved(t *testing.T) {
	calls, err := buildDebugStdlibActions("debugLog", map[string]any{"WFInput": debugOutputRefNamed("SAMPLE-UUID", "Text")}, "expected-uuid", "Debug Result")
	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 {
		t.Fatalf("got %d actions", len(calls))
	}
	params := calls[0].WFWorkflowActionParameters
	if params["UUID"] != "expected-uuid" || params["CustomOutputName"] != "Debug Result" {
		t.Fatalf("result output metadata not preserved: %v", params)
	}
}

func TestDebugStdlibRejectsUnknownMacro(t *testing.T) {
	if _, err := buildDebugStdlibActions("debugNotReal", map[string]any{}, "", ""); err == nil {
		t.Fatal("unknown macro must fail")
	}
}

func TestDebugStdlibNoCrossShortcutPersistentState(t *testing.T) {
	for i := 0; i < 2; i++ {
		src := fmt.Sprintf("text(%q)\ndebugLog()\ndebugShowLogs()\n", fmt.Sprintf("run-%d", i))
		acts := compileDebugFixture(t, src, false)
		if len(acts) != 3 {
			t.Fatalf("run %d had %d actions", i, len(acts))
		}
		if acts[0].WFWorkflowActionParameters["WFTextActionText"] != fmt.Sprintf("run-%d", i) {
			t.Fatal("state leaked across compilation")
		}
	}
}

func TestDebugStdlibImplicitPreviousExactlyOneAppend(t *testing.T) {
	if _, err := buildDebugStdlibActions("debugLogPrevious", map[string]any{}, "", ""); err == nil {
		t.Fatal("building debugLogPrevious outside a compiler context must fail")
	}
	for _, legacy := range []bool{true, false} {
		acts := compileDebugFixture(t, "text(\"hi\")\ndebugLogPrevious()\n", legacy)
		checkDebugInputReference(t, acts, legacy)
	}
}

func TestDebugStdlibFailsClosedWithoutPreviousOutput(t *testing.T) {
	_, err := resolveDebugMacroCall("debugLog", map[string]any{}, nil, "", "")
	if err == nil {
		t.Fatal("debugLog() at start of workflow must fail")
	}
	for _, legacy := range []bool{true, false} {
		// Add to Variable does not have a usable action result: fail rather
		// than silently creating an unbound or wrong attachment.
		first := []ShortcutAction{{WFWorkflowActionIdentifier: "is.workflow.actions.appendvariable"}}
		_, err := resolveDebugMacroCall("debugLogPrevious", map[string]any{}, first, "", "")
		if err == nil {
			t.Fatalf("legacy=%v must reject non-output predecessor", legacy)
		}
	}
}

func TestDebugStdlibRejectsLiteralInsteadOfAddingText(t *testing.T) {
	producer := []ShortcutAction{{WFWorkflowActionIdentifier: "is.workflow.actions.gettext",
		WFWorkflowActionParameters: map[string]any{"WFTextActionText": "previous value"}}}
	if _, err := resolveDebugMacroCall("debugLog", map[string]any{"WFInput": "literal"}, producer, "", ""); err == nil {
		t.Fatal("debugLog(literal) must fail, never create a Text producer")
	}
	if _, err := buildDebugStdlibActions("debugLog", map[string]any{"WFInput": "literal"}, "", ""); err == nil {
		t.Fatal("builder must reject raw string WFInput")
	}
	if _, err := resolveDebugMacroCall("debugLogPrevious", map[string]any{"WFInput": "literal"}, producer, "", ""); err == nil {
		t.Fatal("alias must also reject explicit inputs")
	}
	for _, legacy := range []bool{true, false} {
		acts := compileDebugFixture(t, "text(\"message from action\")\ndebugLog()\n", legacy)
		checkDebugInputReference(t, acts, legacy)
	}
}

// Exact native reference is from the user-edited משתני לוג-2.json:
// repeat.each -> setitemname(Repeat Item, Creation Date) -> repeat.end ->
// previewdocument(Repeat Results).
func verifyNativeNamedItem(t *testing.T, action ShortcutAction) {
	t.Helper()
	if action.WFWorkflowActionIdentifier != "is.workflow.actions.setitemname" {
		t.Fatalf("Expected Set Name, got %s", action.WFWorkflowActionIdentifier)
	}
	p := action.WFWorkflowActionParameters
	original := debugParamMap(debugParamMap(p["WFInput"])["Value"])
	if original["Type"] != "Variable" || original["VariableName"] != "Repeat Item" {
		t.Fatalf("Set Name must use untouched Repeat Item: %v", original)
	}
	name := debugParamMap(p["WFName"])
	if name["WFSerializationType"] != "WFTextTokenString" {
		t.Fatalf("WFName must be Apple's native tokenized string: %v", name)
	}
	val := debugParamMap(name["Value"])
	if val["string"] != "\uFFFC" {
		t.Fatalf("WFName must be ONE native item token: %v", val)
	}
	tokens := debugParamMap(val["attachmentsByRange"])
	if len(tokens) != 1 {
		t.Fatalf("unexpected Set Name tokens: %v", tokens)
	}
	ref := debugParamMap(tokens["{0, 1}"])
	if ref["Type"] != "Variable" || ref["VariableName"] != "Repeat Item" {
		t.Fatalf("WFName is not bound to Repeat Item: %v", ref)
	}
	agg, ok := ref["Aggrandizements"].([]any)
	if !ok || len(agg) != 2 {
		t.Fatalf("creation date token missing 2 Apple aggrandizements: %v", ref)
	}
	prop := debugParamMap(agg[0])
	dt := debugParamMap(agg[1])
	if prop["Type"] != "WFPropertyVariableAggrandizement" ||
		prop["PropertyName"] != "Creation Date" ||
		prop["PropertyUserInfo"] != "WFFileCreationDate" {
		t.Fatalf("wrong creation date property: %v", prop)
	}
	if dt["Type"] != "WFDateFormatVariableAggrandizement" ||
		dt["WFDateFormatStyle"] != "Medium" ||
		dt["WFTimeFormatStyle"] != "Long" ||
		dt["WFISO8601IncludeTime"] != false {
		t.Fatalf("wrong original user date/time formatting: %v", dt)
	}
}

func verifyNativeDatedTextRow(t *testing.T, action ShortcutAction) {
	t.Helper()
	if action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
		t.Fatalf("dated text mode must use Text action: %s", action.WFWorkflowActionIdentifier)
	}
	val := debugParamMap(action.WFWorkflowActionParameters["WFTextActionText"])
	if val["WFSerializationType"] != "WFTextTokenString" {
		t.Fatalf("not native text token: %v", val)
	}
	payload := debugParamMap(val["Value"])
	if payload["string"] != "\uFFFC - \uFFFC" {
		t.Fatalf("missing value - date layout: %v", payload)
	}
	att := debugParamMap(payload["attachmentsByRange"])
	a := debugParamMap(att["{0, 1}"])
	b := debugParamMap(att["{4, 1}"])
	if a["Type"] != "Variable" || a["VariableName"] != "Repeat Item" ||
		b["Type"] != "Variable" || b["VariableName"] != "Repeat Item" {
		t.Fatalf("both row tokens must come from same Repeat Item: %v", att)
	}
	props, ok := b["Aggrandizements"].([]any)
	if !ok || len(props) != 2 {
		t.Fatalf("expected Creation Date and formatted-time token: %v", b)
	}
	prop := debugParamMap(props[0])
	dt := debugParamMap(props[1])
	if prop["PropertyUserInfo"] != "WFFileCreationDate" || prop["PropertyName"] != "Creation Date" ||
		dt["Type"] != "WFDateFormatVariableAggrandizement" || dt["WFTimeFormatStyle"] != "Long" {
		t.Fatalf("missing date formatting metadata: %v", props)
	}
}

// 6 valid independent cases, with an explicit wait between native producers.
func TestDebugStdlibSixDisplayModes(t *testing.T) {
	cases := []struct {
		name, legacyCall, v2Call string
		dates, display, asText   bool
	}{
		{"record-only", "debugLog()", "debugLog()", false, false, false},
		{"record-creation-date", "debugLog(true)", "debugLog(withDates: true)", true, false, false},
		{"native-preview", "debugLog(false, true)", "debugLog(showLogs: true)", false, true, false},
		{"native-dated-preview", "debugLog(true, true)", "debugLog(withDates: true, showLogs: true)", true, true, false},
		{"text-preview", "debugLog(false, false, true)", "debugLog(showLogsAsText: true)", false, false, true},
		{"dated-text-preview", "debugLog(true, false, true)", "debugLog(withDates: true, showLogsAsText: true)", true, false, true},
	}
	for _, legacy := range []bool{true, false} {
		for _, tc := range cases {
			t.Run(fmt.Sprintf("legacy=%t/%s", legacy, tc.name), func(t *testing.T) {
				call := tc.v2Call
				if legacy {
					call = tc.legacyCall
				}
				// Logged native Text values are separated by 1s.
				src := "text(\"first\")\n" + call + "\nwait(1)\ntext(\"second\")\n" + call + "\nwait(1)\n"
				acts := compileDebugFixture(t, src, legacy)
				names := actionSuffixes(acts)
				counts := map[string]int{}
				for _, name := range names {
					counts[name]++
				}
				if counts["appendvariable"] != 2 || counts["delay"] != 2 {
					t.Fatalf("Expected 2 typed logs and two delays: %v", names)
				}
				dateCount := 0
				if tc.dates {
					dateCount = 2
				}
				displayCount := 0
				if tc.display || tc.asText {
					displayCount = 2
				}
				wantText := 0
				if tc.asText && !tc.dates {
					wantText = 2
				}
				wantRepeat := 0
				if tc.dates && (tc.display || tc.asText) {
					wantRepeat = 4
				}
				wantName := 0
				if tc.dates && tc.display {
					wantName = 2
				}
				if counts["detect.date"] != dateCount || counts["previewdocument"] != displayCount ||
					counts["text.combine"] != wantText || counts["repeat.each"] != wantRepeat ||
					counts["setitemname"] != wantName {
					t.Fatalf("wrong action mode %s: %v", tc.name, names)
				}
				if counts["getitemfromlist"] != 0 || counts["properties.files"] != 0 {
					t.Fatalf("deleted date-list lookup returned: %v", names)
				}
				for i, a := range acts {
					p := a.WFWorkflowActionParameters
					if p["WFVariableName"] == "__cherri_debug_dates" {
						t.Fatalf("obsolete parallel date list: %v", p)
					}
					switch names[i] {
					case "appendvariable":
						if p["WFVariableName"] != debugLogVariableName {
							t.Fatalf("must append original native value: %v", p)
						}
						ref := debugParamMap(debugParamMap(p["WFInput"])["Value"])
						if ref["Type"] != "ActionOutput" || ref["OutputName"] != "Text" {
							t.Fatalf("original output was converted: %v", ref)
						}
					case "detect.date":
						if i == 0 || names[i-1] != "gettext" {
							t.Fatalf("date must appear immediately after producer: %v", names)
						}
						src := debugParamMap(debugParamMap(p["WFInput"])["Value"])
						if src["OutputUUID"] != acts[i-1].WFWorkflowActionParameters["UUID"] {
							t.Fatalf("date's original output link broken: %v", src)
						}
						props, ok := src["Aggrandizements"].([]any)
						if !ok || len(props) != 1 {
							t.Fatalf("expected Creation Date property: %v", src)
						}
						prop := debugParamMap(props[0])
						if prop["PropertyUserInfo"] != "WFFileCreationDate" ||
							prop["PropertyName"] != "Creation Date" {
							t.Fatalf("incorrect Apple Creation Date metadata: %v", prop)
						}
					case "text.combine":
						if !tc.asText || tc.dates {
							t.Fatalf("unexpected Combine Text: %v", names)
						}
						if p["WFTextSeparator"] != "New Lines" {
							t.Fatalf("wrong text separator: %v", p)
						}
						ref := debugParamMap(debugParamMap(p["text"])["Value"])
						if ref["Type"] != "Variable" || ref["VariableName"] != debugLogVariableName {
							t.Fatalf("Combine Text must use original debug log: %v", ref)
						}
					case "setitemname":
						verifyNativeNamedItem(t, a)
					case "previewdocument":
						ref := debugParamMap(debugParamMap(p["WFInput"])["Value"])
						if !tc.dates && !tc.asText {
							if ref["Type"] != "Variable" || ref["VariableName"] != debugLogVariableName {
								t.Fatalf("native preview must display original object list: %v", ref)
							}
						} else if ref["Type"] != "ActionOutput" {
							t.Fatalf("formatted view must preview output results: %v", ref)
						}
					case "delay":
						if fmt.Sprint(p["WFDelayTime"]) != "1" {
							t.Fatalf("wait not 1 second: %v", p)
						}
					}
				}
				if tc.dates && (tc.display || tc.asText) {
					loops := 0
					for i, name := range names {
						if name != "repeat.each" || i+1 >= len(names) || (names[i+1] != "setitemname" && names[i+1] != "gettext") {
							continue
						}
						loops++
						if tc.asText {
							verifyNativeDatedTextRow(t, acts[i+1])
						} else {
							verifyNativeNamedItem(t, acts[i+1])
						}
						if names[i+2] != "repeat.each" || names[i+3] != "previewdocument" {
							t.Fatalf("native loop must feed directly into Quick Look: %v", names)
						}
						start := acts[i].WFWorkflowActionParameters
						stop := acts[i+2].WFWorkflowActionParameters
						if start["GroupingIdentifier"] != stop["GroupingIdentifier"] {
							t.Fatalf("repeat group mismatch")
						}
						ref := debugParamMap(debugParamMap(acts[i+3].WFWorkflowActionParameters["WFInput"])["Value"])
						if ref["OutputUUID"] != stop["UUID"] || ref["OutputName"] != "Repeat Results" {
							t.Fatalf("Quick Look not attached to Repeat Results: %v", ref)
						}
					}
					if loops != 2 {
						t.Fatalf("expected 2 dated preview loops, got %d", loops)
					}
				}
			})
		}
	}
}

func TestDebugStdlibExplicitDisplayHelpers(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		for _, name := range []string{"debugShowLogs", "debugFinish"} {
			cases := []struct {
				args string
				want []string
			}{
				{"", []string{"gettext", "appendvariable", "previewdocument"}},
				{"false", []string{"gettext", "appendvariable", "previewdocument"}},
				{"true", []string{"gettext", "detect.date", "appendvariable", "repeat.each", "setitemname", "repeat.each", "previewdocument"}},
				{"false, true", []string{"gettext", "appendvariable", "text.combine", "previewdocument"}},
				{"true, true", []string{"gettext", "detect.date", "appendvariable", "repeat.each", "gettext", "repeat.each", "previewdocument"}},
			}
			for _, tc := range cases {
				args := tc.args
				if !legacy {
					switch args {
					case "false":
						args = "withDates: false"
					case "true":
						args = "withDates: true"
					case "false, true":
						args = "showLogsAsText: true"
					case "true, true":
						args = "withDates: true, showLogsAsText: true"
					}
				}
				src := "text(\"A\")\ndebugLog()\n" + name + "(" + args + ")\n"
				acts := compileDebugFixture(t, src, legacy)
				ids := actionSuffixes(acts)
				if !reflect.DeepEqual(ids, tc.want) {
					t.Fatalf("%s(%s) legacy=%v: got %v want %v", name, args, legacy, ids, tc.want)
				}
				if len(ids) > 4 && ids[4] == "setitemname" {
					verifyNativeNamedItem(t, acts[4])
				}
				if len(ids) > 4 && ids[4] == "gettext" {
					verifyNativeDatedTextRow(t, acts[4])
				}
			}
		}
	}
}

func TestDebugStdlibRejectsConflictingDisplayModes(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		src := "text(\"A\")\n"
		if legacy {
			src += "debugLog(false, true, true)\n"
		} else {
			src += "debugLog(showLogs: true, showLogsAsText: true)\n"
		}
		var err error
		if legacy {
			_, err = compileLegacySourceToRawMapInMemory(src)
		} else {
			_, err = CompileSourceToPlist("conflicting-debug-modes.cherri", src)
		}
		if err == nil {
			t.Fatalf("legacy=%v must reject ambiguous text+native previews", legacy)
		}
	}
}

func TestDebugStdlibDatedPassIdempotentAndNoDateOnPlainPreview(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		call := "debugShowLogs(true)"
		if !legacy {
			call = "debugShowLogs(withDates: true)"
		}
		first := "text(\"A\")\ndebugLog()\ndebugShowLogs()\ntext(\"B\")\ndebugLog()\n"
		acts := compileDebugFixture(t, first+call+"\n"+call+"\n", legacy)
		var captures, original int
		for i, a := range acts {
			p := a.WFWorkflowActionParameters
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.date" {
				captures++
				if i == 0 || acts[i-1].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
					t.Fatalf("date must appear immediately after output, not preview: %d", i)
				}
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" &&
				p["WFVariableName"] == debugLogVariableName {
				original++
			}
			if p["WFVariableName"] == "__cherri_debug_dates" {
				t.Fatalf("obsolete date list must never be emitted: %v", p)
			}
		}
		if captures != 2 || original != 2 {
			t.Fatalf("backfill must be idempotent: dates=%d logs=%d", captures, original)
		}
		plain := compileDebugFixture(t, first+"debugShowLogs()\n", legacy)
		for _, a := range plain {
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.date" {
				t.Fatal("no date extraction without any withDates:true")
			}
		}
	}
}

func TestDebugStdlibDatedClearAndTimerSynchronized(t *testing.T) {
	for _, legacy := range []bool{true, false} {
		call := "debugShowLogs(true)"
		if !legacy {
			call = "debugShowLogs(withDates: true)"
		}
		src := "debugClearLogs()\ndebugStartTimer(\"phase\")\ntext(\"output\")\ndebugLog()\ndebugStopTimer(\"phase\")\n" + call + "\n"
		acts := compileDebugFixture(t, src, legacy)
		var orig, dates int
		for i, a := range acts {
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" &&
				a.WFWorkflowActionParameters["WFVariableName"] == debugLogVariableName {
				orig++
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.detect.date" {
				dates++
				if i == 0 || a.WFWorkflowActionParameters["WFInput"] == nil {
					t.Fatalf("missing date source: %v", a)
				}
			}
		}
		if orig != 2 || dates != 2 {
			t.Fatalf("legacy=%t: both normal and timer entries need source-time dates: %d %d", legacy, orig, dates)
		}
	}
}

func TestDebugStdlibShowLogsRejectsInvalidFlag(t *testing.T) {
	if _, err := buildDebugStdlibActions("debugShowLogs", map[string]any{"WFDebugWithDates": "true"}, "", ""); err == nil {
		t.Fatal("non-boolean withDates flag accepted")
	}
}

func TestDebugStdlibMustNotRewriteExistingProducerUUID(t *testing.T) {
	originalID := "abcdef12-3456-4567-89ab-abcdef123456"
	prior := []ShortcutAction{{
		WFWorkflowActionIdentifier: "is.workflow.actions.gettext",
		WFWorkflowActionParameters: map[string]any{
			"UUID":             originalID,
			"WFTextActionText": "this output is also used later",
		},
	}}
	acts, err := resolveDebugMacroCall("debugLog", map[string]any{}, prior, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(acts) != 1 {
		t.Fatalf("must produce exactly one Add to Variable: %v", actionSuffixes(acts))
	}
	gotID, _ := prior[0].WFWorkflowActionParameters["UUID"].(string)
	if gotID != originalID {
		t.Fatalf("debugLog mutated producer UUID: was %s, now %s", originalID, gotID)
	}
	input := debugParamMap(acts[0].WFWorkflowActionParameters["WFInput"])
	val := debugParamMap(input["Value"])
	if val["OutputUUID"] != originalID || val["OutputName"] != "Text" {
		t.Fatalf("debugLog input token must use exact existing producer UUID; got %v", val)
	}
}

func TestDebugStdlibLoggingDoesNotBreakLaterConsumer(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		legacy       bool
	}{
		{"legacy", "const textResult = text(\"Hello\")\ndebugLog()\nshow(textResult)\n", true},
		{"v2", "let textResult = text(\"Hello\")\ndebugLog()\nshow(textResult)\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := compileDebugFixture(t, tc.source, tc.legacy)
			if len(acts) != 3 {
				t.Fatalf("expected Text, one Add to Variable, Show Result; got %v", actionSuffixes(acts))
			}
			originalID, _ := acts[0].WFWorkflowActionParameters["UUID"].(string)
			if originalID == "" {
				t.Fatal("producer UUID missing")
			}
			logInput := debugParamMap(acts[1].WFWorkflowActionParameters["WFInput"])
			logVal := debugParamMap(logInput["Value"])
			if logVal["OutputUUID"] != originalID {
				t.Fatalf("log reference %v differs from producer UUID %s", logVal, originalID)
			}
			// Any subsequent action-output refs to the same producer must
			// remain identical after the log macro expands.
			var found bool
			var walk func(any)
			walk = func(v any) {
				switch x := v.(type) {
				case map[string]any:
					if x["Type"] == "ActionOutput" {
						if x["OutputUUID"] == originalID {
							found = true
						}
						if x["OutputName"] == "textResult" && x["OutputUUID"] != originalID {
							t.Fatalf("later consumer carries stale producer UUID %v vs %s", x["OutputUUID"], originalID)
						}
					}
					for _, child := range x {
						walk(child)
					}
				case []any:
					for _, child := range x {
						walk(child)
					}
				}
			}
			// The legacy emitter uses typed WFTextTokenString structs, while
			// V2 may use maps. Normalize to JSON before traversing nested
			// attachments so this regression checks both accurately.
			raw, err := json.Marshal(acts[2].WFWorkflowActionParameters)
			if err != nil {
				t.Fatal(err)
			}
			var decoded map[string]any
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			walk(decoded)
			if !found {
				t.Fatalf("later consumer must be bound to producer UUID %s; got %v", originalID, acts[2].WFWorkflowActionParameters)
			}
		})
	}
}

func TestDebugStdlibTimerNativeDateFieldSerialization(t *testing.T) {
	for _, tc := range []struct {
		name   string
		legacy bool
	}{
		{"legacy", true}, {"v2", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := compileDebugFixture(t, "debugStartTimer(\"phase\")\ndebugStopTimer(\"phase\")\n", tc.legacy)
			want := []string{"setvariable", "gettimebetweendates", "appendvariable", "getvariable"}
			if !reflect.DeepEqual(actionSuffixes(acts), want) {
				t.Fatalf("wrong optimized timer sequence: got %v want %v", actionSuffixes(acts), want)
			}
			timerKey, err := debugTimerVariableName("phase")
			if err != nil {
				t.Fatal(err)
			}
			start := acts[0].WFWorkflowActionParameters
			if start["WFVariableName"] != timerKey {
				t.Fatalf("start timer saved under wrong variable: %v", start)
			}
			input := debugParamMap(start["WFInput"])
			value := debugParamMap(input["Value"])
			if input["WFSerializationType"] != "WFTextTokenAttachment" ||
				value["Type"] != "CurrentDate" || len(value) != 1 {
				t.Fatalf("start date must use built-in CurrentDate token: %v", input)
			}
			params := acts[1].WFWorkflowActionParameters
			for _, field := range []struct{ key, typ string }{
				{"WFTimeUntilFromDate", "Variable"},
				{"WFInput", "CurrentDate"},
			} {
				item := debugParamMap(params[field.key])
				if item["WFSerializationType"] != "WFTextTokenString" {
					t.Fatalf("%s requires Apple's WFTextTokenString: %v", field.key, item)
				}
				payload := debugParamMap(item["Value"])
				if payload["string"] != "\uFFFC" {
					t.Fatalf("%s missing U+FFFC character: %v", field.key, payload)
				}
				atts := debugParamMap(payload["attachmentsByRange"])
				reference := debugParamMap(atts["{0, 1}"])
				if reference["Type"] != field.typ {
					t.Fatalf("%s wrong native token type: %v", field.key, reference)
				}
				if field.typ == "Variable" && reference["VariableName"] != timerKey {
					t.Fatalf("%s points to wrong timer variable: %v", field.key, reference)
				}
				if field.typ == "CurrentDate" && len(reference) != 1 {
					t.Fatalf("%s CurrentDate must have no UUID/name: %v", field.key, reference)
				}
			}
			if params["WFTimeUntilUnit"] != "Seconds" {
				t.Fatalf("wrong time unit: %v", params)
			}
			logged := debugParamMap(acts[2].WFWorkflowActionParameters["WFInput"])
			output := debugParamMap(logged["Value"])
			if output["OutputUUID"] != params["UUID"] || output["OutputName"] != "Time Between Dates" {
				t.Fatalf("elapsed time is not linked to logging action: %v", output)
			}
		})
	}
}

func TestDebugStdlibQAStructuredTracingDoesNotPerturbDebugLogOrSourceGraph(t *testing.T) {
	uninstrumentedV2 := `let greeting = text("Hello QA")
debugLog()
show(greeting)
`
	instrumentedV2 := `debugTraceEvent(stage: "before", label: "greeting")
let greeting = text("Hello QA")
debugLog()
debugTraceEvent(stage: "after", label: "greeting")
show(greeting)
let traceSnapshot = debugExportTrace(runId: "qa-run-20261115")
`
	unActs := compileDebugFixture(t, uninstrumentedV2, false)
	inActs := compileDebugFixture(t, instrumentedV2, false)

	mappings := ClassifyShortcutTraceActions(inActs)
	var sourceActions []string
	var inlineDebugCount, collectorCount int
	for _, m := range mappings {
		switch m.Kind {
		case "SOURCE_ACTION":
			sourceActions = append(sourceActions, strings.TrimPrefix(m.ActionIdentifier, "is.workflow.actions."))
		case "INLINE_DEBUG_ACTION":
			inlineDebugCount++
		case "COLLECTOR_EVENT":
			collectorCount++
		}
	}
	wantSource := []string{"gettext", "showresult"}
	if !reflect.DeepEqual(sourceActions, wantSource) {
		t.Fatalf("source actions perturbed by QA tracing: got %v want %v", sourceActions, wantSource)
	}
	if inlineDebugCount != 5 { // 2x (gettext+appendvariable for debugTraceEvent) + 1x appendvariable for debugLog
		t.Fatalf("expected 5 inline debug actions, got %d (mappings=%+v)", inlineDebugCount, mappings)
	}
	if collectorCount != 2 { // text.combine + gettext for debugExportTrace
		t.Fatalf("expected 2 collector actions, got %d (mappings=%+v)", collectorCount, mappings)
	}
	// Verify uninstrumented source actions match the SOURCE_ACTION projection of instrumented actions
	unMappings := ClassifyShortcutTraceActions(unActs)
	var unSource []string
	for _, m := range unMappings {
		if m.Kind == "SOURCE_ACTION" {
			unSource = append(unSource, strings.TrimPrefix(m.ActionIdentifier, "is.workflow.actions."))
		}
	}
	if !reflect.DeepEqual(sourceActions, unSource) {
		t.Fatalf("instrumented source projection %v != uninstrumented %v", sourceActions, unSource)
	}
}

func TestDebugStdlibTypedProducersAndEdgeControlFlows(t *testing.T) {
	// 1. Typed producers: dictionary, number, text, date
	for _, tc := range []struct {
		name   string
		src    string
		legacy bool
	}{
		{
			name:   "v2-typed-producers",
			legacy: false,
			src: `let d = {"alpha": "one", "count": 2}
debugLog(withDates: true)
let n = number(42)
debugLog(withDates: true)
debugShowLogs(withDates: true)
`,
		},
		{
			name:   "legacy-typed-producers",
			legacy: true,
			src: `const d = {"alpha": "one", "count": 2}
debugLog(true)
const n = number(42)
debugLog(true)
debugShowLogs(true)
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			acts := compileDebugFixture(t, tc.src, tc.legacy)
			seenUUIDs := map[string]bool{}
			var appendCount, dateCount, setNameCount int
			for i, a := range acts {
				p := a.WFWorkflowActionParameters
				if u, ok := p["UUID"].(string); ok && u != "" {
					seenUUIDs[u] = true
				}
				switch strings.TrimPrefix(a.WFWorkflowActionIdentifier, "is.workflow.actions.") {
				case "detect.date":
					dateCount++
					src, ok := debugLogOutputRef(p["WFInput"])
					if !ok || !seenUUIDs[src.OutputUUID] {
						t.Fatalf("action %d: detect.date references missing or forward UUID %q", i, src.OutputUUID)
					}
				case "appendvariable":
					if p["WFVariableName"] == debugLogVariableName {
						appendCount++
						src, ok := debugLogOutputRef(p["WFInput"])
						if !ok || !seenUUIDs[src.OutputUUID] {
							t.Fatalf("action %d: appendvariable references missing or forward UUID %q", i, src.OutputUUID)
						}
					}
				case "setitemname":
					setNameCount++
					verifyNativeNamedItem(t, a)
				}
			}
			if appendCount != 2 || dateCount != 2 || setNameCount != 1 {
				t.Fatalf("unexpected counts: append=%d date=%d setName=%d", appendCount, dateCount, setNameCount)
			}
		})
	}

	// 2. Early exit, conditional not-taken, nested repeat, and deliberate debugAssert failure
	edgeV2 := `debugTraceEvent(stage: "before", label: "start")
let flag = false
if flag {
    text("branch-not-taken")
    debugLog()
} else {
    repeat 2 {
        let items = list("x", "y")
        for item in items {
            text("nested")
            debugLog()
        }
    }
}
debugAssert(condition: false, message: "Deliberate QA assertion failure")
stop()
debugTraceEvent(stage: "end", label: "unreachable-after-stop")
let exported = debugExportTrace(runId: "qa-edge-cases")
`
	acts := compileDebugFixture(t, edgeV2, false)
	suffixes := actionSuffixes(acts)
	var hasAlert, hasExit, hasRepeatCount, hasRepeatEach bool
	for _, s := range suffixes {
		switch s {
		case "alert":
			hasAlert = true
		case "exit":
			hasExit = true
		case "repeat.count":
			hasRepeatCount = true
		case "repeat.each":
			hasRepeatEach = true
		}
	}
	if !hasAlert || !hasExit || !hasRepeatCount || !hasRepeatEach {
		t.Fatalf("edge control-flow fixture missing expected constructs: %v", suffixes)
	}
}
