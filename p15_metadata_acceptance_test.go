package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

func p15MetadataNative(t *testing.T, name, source string) map[string]any {
	t.Helper()
	data, err := CompileSourceToPlist(name+".cherri", source)
	if err != nil {
		t.Fatalf("actual compiler: %v", err)
	}
	var root map[string]any
	if _, err := plist.Unmarshal(data, &root); err != nil {
		t.Fatal(err)
	}
	return root
}

func p15SetupQuestionError(root map[string]any, prompt, actionID, key, expected string) error {
	raw, ok := root["WFWorkflowImportQuestions"].([]any)
	if !ok {
		return fmt.Errorf("native import questions absent")
	}
	found := 0
	for _, value := range raw {
		q, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("question is not a dictionary")
		}
		if q["Text"] != prompt {
			continue
		}
		found++
		index, exists := q["ActionIndex"]
		if !exists {
			return fmt.Errorf("ActionIndex missing, including zero")
		}
		var n int
		switch x := index.(type) {
		case uint64:
			n = int(x)
		case int64:
			n = int(x)
		case int:
			n = x
		default:
			return fmt.Errorf("invalid ActionIndex %T", index)
		}
		actions, ok := root["WFWorkflowActions"].([]any)
		if !ok || n < 0 || n >= len(actions) {
			return fmt.Errorf("question points outside actions: %d", n)
		}
		action := actions[n].(map[string]any)
		if action["WFWorkflowActionIdentifier"] != actionID {
			return fmt.Errorf("question %q points to %d %v, expected %s", prompt, n, action["WFWorkflowActionIdentifier"], actionID)
		}
		if q["ParameterKey"] != key {
			return fmt.Errorf("wrong parameter: %v", q)
		}
		params := action["WFWorkflowActionParameters"].(map[string]any)
		if !plistValuesStrictlyEqual(params[key], expected) {
			return fmt.Errorf("question %q points to wrong value at %d: %v", prompt, n, params[key])
		}
	}
	if found != 1 {
		return fmt.Errorf("question %q occurs %d times", prompt, found)
	}
	return nil
}

func TestP15M03SetupFinalActionBinding(t *testing.T) {
	const setup = "setup message: Text {\n prompt: \"P15 question\"\n default: \"P15 default\"\n}\n"
	cases := []struct{ name, source, id, key string }{
		{"action_zero", setup + "text(message)", "is.workflow.actions.gettext", "WFTextActionText"},
		{"later_argument_producer", setup + "alert(message, title: text(\"P15 title\"))", "is.workflow.actions.alert", "WFAlertActionMessage"},
		{"function_expansion", setup + "function greeting()->Text { return \"hello\" }\nlet result=greeting()\ntext(message)", "is.workflow.actions.gettext", "WFTextActionText"},
		{"later_date_backfill", setup + "let observed=text(\"P15 log\")\ndebugLog()\ntext(message)\ndebugShowLogs(withDates: true)", "is.workflow.actions.gettext", "WFTextActionText"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := p15MetadataNative(t, tc.name, tc.source)
			if err := p15SetupQuestionError(root, "P15 question", tc.id, tc.key, "P15 default"); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("question_name_collides_with_wire_key", func(t *testing.T) {
		source := "setup first: Text {\n prompt: \"first\"\n default: \"A\"\n}\nsetup WFTextActionText: Text {\n prompt: \"second\"\n default: \"B\"\n}\ntext(first)\ntext(WFTextActionText)"
		root := p15MetadataNative(t, "name-collision", source)
		for _, tc := range []struct{ prompt, value string }{{"first", "A"}, {"second", "B"}} {
			if err := p15SetupQuestionError(root, tc.prompt, "is.workflow.actions.gettext", "WFTextActionText", tc.value); err != nil {
				t.Error(err)
			}
		}
	})
}

func TestP15M04InvalidSetupAndEditorHoles(t *testing.T) {
	const setup = "setup message: Text {\n prompt: \"P15 question\"\n default: \"P15 default\"\n}\n"
	for name, body := range map[string]string{
		"alias":             "let copied=message\ntext(copied)",
		"expression":        "text(message+\"suffix\")",
		"reuse":             "text(message)\ntext(message)",
		"two_parameters":    "alert(message,title:message)",
		"function_argument": "function echo(value:Text)->Text {return value}\necho(message)",
		"macro":             "require(message)",
	} {
		t.Run(name, func(t *testing.T) {
			source := setup + body
			data, err := CompileSourceToPlist(name+".cherri", source)
			if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeSetupBinding) {
				t.Errorf("unsupported setup reached native output or wrong diagnostic: bytes=%d err=%v", len(data), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-" + name + ".cherri"
			svc.OpenDocument(uri, 1, source)
			diags, _ := svc.Analyze(uri)
			found := false
			for _, d := range diags {
				if d.Code == analysis.CodeSetupBinding && d.Severity == analysis.SeverityError {
					found = true
				}
			}
			if !found {
				t.Errorf("editor did not reject unsupported setup: %v", diags)
			}
		})
	}
	t.Run("palette_unfilled_argument", func(t *testing.T) {
		svc := service.NewService(nil)
		uri := "file:///p15-hole.cherri"
		svc.OpenDocument(uri, 1, "")
		var insertion string
		for _, item := range svc.Complete(uri, 1, 1) {
			if item.Label == "text" {
				insertion = item.InsertText
				break
			}
		}
		if insertion == "" {
			t.Fatal("text completion missing")
		}
		if !strings.Contains(insertion, "<#") {
			t.Errorf("missing required-argument hole: %s", insertion)
		}
		if data, err := CompileSourceToPlist("unfilled.cherri", insertion); err == nil || len(data) != 0 {
			t.Errorf("unfilled palette insertion silently compiled: %s", insertion)
		}
		svc.OpenDocument(uri, 2, insertion)
		diagnostics, _ := svc.Analyze(uri)
		rejected := false
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == analysis.SeverityError {
				rejected = true
			}
		}
		if !rejected {
			t.Fatal("editor service did not report the unfilled insertion", diagnostics)
		}
		filled := strings.ReplaceAll(insertion, "<#text#>", "\"P15_FILLED\"")
		p15MetadataNative(t, "filled-insertion", filled)
	})
	t.Run("explicit_ask_control", func(t *testing.T) {
		root := p15MetadataNative(t, "explicit-ask", "text(Ask)")
		data, _ := plist.Marshal(root, plist.XMLFormat)
		if !strings.Contains(string(data), "Ask") {
			t.Fatal("explicit runtime Ask lost")
		}
	})
}

func TestP15M01KnownHeaderMapping(t *testing.T) {
	source := `shortcut "P15 metadata" {
 from: ["sharesheet","menubar","quickactions","notifications","watch","search","spotlight","sleepmode","onscreen"]
 targets: ["ios","ipados"]
 inputs: ["text","image","WFCustomFutureContentItem"]
 outputs: ["url"]
 noInput: "clipboard"
 icon: {glyph: 59789, color: 4282601983}
}
text("P15_HEADER")
`
	root := p15MetadataNative(t, "p15-header", source)
	expected := map[string]any{
		"WFWorkflowTypes":                    []any{"ActionExtension", "MenuBar", "QuickActions", "NCWidget", "Watch", "WFWorkflowTypeShowInSearch", "WFWorkflowTypeReceivesInputFromSearch", "Sleep", "ReceivesOnScreenContent"},
		"WFQuickActionSurfaces":              []any{"Finder", "Services"},
		"WFWorkflowInputContentItemClasses":  []any{"WFStringContentItem", "WFImageContentItem", "WFCustomFutureContentItem"},
		"WFWorkflowOutputContentItemClasses": []any{"WFURLContentItem"},
		"WFWorkflowNoInputBehavior":          map[string]any{"Name": "WFWorkflowNoInputBehaviorGetClipboard"},
		"WFWorkflowIcon":                     map[string]any{"WFWorkflowIconGlyphNumber": uint64(59789), "WFWorkflowIconStartColor": uint64(4282601983)},
	}
	for key, value := range expected {
		if !plistValuesStrictlyEqual(value, root[key]) {
			t.Errorf("known field %s differs: got %#v expected %#v", key, root[key], value)
		}
	}
	for name, wire := range map[string]string{"ask": "WFWorkflowNoInputBehaviorAskForInput", "error": "WFWorkflowNoInputBehaviorShowError", "P15FutureBehavior": "P15FutureBehavior"} {
		got := p15MetadataNative(t, "no-input-"+name, "shortcut \"Meta\" { noInput: \""+name+"\" }\ntext(\"control\")")
		if got["WFWorkflowNoInputBehavior"].(map[string]any)["Name"] != wire {
			t.Fatal("no-input mapping changed", name, got)
		}
	}
	// Targets are compiler availability constraints. They must not overwrite the
	// existing native invocation-surface field with OS platform spellings.
	for _, target := range []string{"ios", "ipados"} {
		if data, err := CompileSourceToPlist("target.cherri", "shortcut \"Target\" { target: \""+target+"\" }\nmakeDiskImage(\"synthetic\")"); err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeUnsupportedTarget) {
			t.Errorf("target availability not enforced: %v", err)
		}
	}
}

func TestP15M01NativeHeaderRoundTrip(t *testing.T) {
	root := baseTestHeader()
	root["WFWorkflowTypes"] = []any{"ActionExtension", "P15FutureSurface", "QuickActions"}
	root["WFQuickActionSurfaces"] = []any{"Finder", "P15FutureQuickAction"}
	root["WFWorkflowInputContentItemClasses"] = []any{"WFStringContentItem", "P15FutureInput"}
	root["WFWorkflowOutputContentItemClasses"] = []any{"WFURLContentItem", "P15FutureOutput"}
	root["WFWorkflowNoInputBehavior"] = map[string]any{"Name": "WFWorkflowNoInputBehaviorShowError", "CustomErrorMessage": "synthetic", "P15Future": []any{false, uint64(0), ""}}
	root["WFWorkflowIcon"] = map[string]any{"WFWorkflowIconGlyphNumber": uint64(59789), "WFWorkflowIconStartColor": uint64(4282601983), "P15FutureIcon": []byte{1, 2, 3}}
	root["P15FutureHeader"] = map[string]any{"nested": []any{false, uint64(0), "", "Ask", "source"}}
	root["WFWorkflowActions"] = []any{p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "P15_METADATA"})}
	p15NativeImportRoundTrip(t, "metadata-future-header", root, plist.XMLFormat, plist.BinaryFormat)
	for _, key := range []string{"WFWorkflowTypes", "WFQuickActionSurfaces", "WFWorkflowInputContentItemClasses", "WFWorkflowOutputContentItemClasses", "WFWorkflowNoInputBehavior", "WFWorkflowIcon", "P15FutureHeader"} {
		t.Run("reject_omitted_"+key, func(t *testing.T) {
			changed := clonePlistMapDeep(root)
			delete(changed, key)
			if err := p15NativePreservationError(root, changed); err == nil {
				t.Fatal("metadata deletion passed exact native oracle")
			}
		})
	}
}

func TestP15M03SetupNativeRoundTrip(t *testing.T) {
	zeroSource := "setup first: Text { prompt: \"Zero question\" default: \"first\" }\ntext(first)"
	zero := p15MetadataNative(t, "setup-zero", zeroSource)
	if err := p15SetupQuestionError(zero, "Zero question", "is.workflow.actions.gettext", "WFTextActionText", "first"); err != nil {
		t.Fatal(err)
	}
	p15NativeImportRoundTrip(t, "setup-zero", zero, plist.XMLFormat, plist.BinaryFormat)
	source := "setup message: Text {\n prompt: \"P15 question\"\n default: \"P15 default\"\n}\nalert(message,title:text(\"title\"))"
	root := p15MetadataNative(t, "setup-native", source)
	if err := p15SetupQuestionError(root, "P15 question", "is.workflow.actions.alert", "WFAlertActionMessage", "P15 default"); err != nil {
		t.Fatal(err)
	}
	p15NativeImportRoundTrip(t, "setup-native", root, plist.XMLFormat, plist.BinaryFormat)
	for _, fault := range []string{"missing_index", "wrong_index", "wrong_parameter", "wrong_value", "duplicate_question"} {
		t.Run(fault, func(t *testing.T) {
			changed := clonePlistMapDeep(root)
			questions := changed["WFWorkflowImportQuestions"].([]any)
			q := questions[0].(map[string]any)
			switch fault {
			case "missing_index":
				delete(q, "ActionIndex")
			case "wrong_index":
				q["ActionIndex"] = uint64(0)
			case "wrong_parameter":
				q["ParameterKey"] = "P15_OTHER"
			case "wrong_value":
				changed["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFAlertActionMessage"] = "P15_OTHER"
			case "duplicate_question":
				changed["WFWorkflowImportQuestions"] = append(questions, q)
			}
			if err := p15SetupQuestionError(changed, "P15 question", "is.workflow.actions.alert", "WFAlertActionMessage", "P15 default"); err == nil {
				t.Fatal("question corruption passed")
			}
		})
	}
}

func p15FilledConstructSnippet(t *testing.T, name, insertion string) string {
	t.Helper()
	if strings.Contains(insertion, "<#") {
		if bytes, err := CompileSourceToPlist("unfilled-construct.cherri", insertion); err == nil || len(bytes) != 0 {
			t.Fatal("unfilled construct inserted a runtime value", name)
		}
	}
	return strings.NewReplacer("<#condition#>", "true", "<#count#>", "1", "<#items#>", `["P15_ITEM"]`, "<#value#>", `"P15_VALUE"`).Replace(insertion)
}

func TestP15M04SetupScopeAndUnused(t *testing.T) {
	const setup = "setup question: Text { prompt: \"P15 question\" default: \"P15 default\" }\n"
	unused := p15MetadataNative(t, "unused-setup", setup+"text(\"ordinary\")")
	if values, ok := unused["WFWorkflowImportQuestions"].([]any); ok && len(values) > 0 {
		t.Fatal("unused setup emitted an unrelated question")
	}
	svc := service.NewService(nil)
	uri := "file:///unused-setup.cherri"
	svc.OpenDocument(uri, 1, setup+"text(\"ordinary\")")
	diagnostics, _ := svc.Analyze(uri)
	warning := false
	for _, d := range diagnostics {
		if d.Code == analysis.CodeUnusedSetup && d.Severity == analysis.SeverityWarning {
			warning = true
		}
	}
	if !warning {
		t.Fatal("unused setup warning missing", diagnostics)
	}
	for _, body := range []string{
		"if true { let question=\"inner\"; text(question) }\ntext(question)",
		"function inner(question:Text)->Text { return text(question) }\nlet result=inner(\"local\")\ntext(question)",
		"function prompt()->Text { return text(question) }\nprompt()\nprompt()",
	} {
		root := p15MetadataNative(t, "setup-scope", setup+body)
		if err := p15SetupQuestionError(root, "P15 question", "is.workflow.actions.gettext", "WFTextActionText", "P15 default"); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{
		"text(f\"value={question}\")",
		"let values=[question]\nquicklook(values)",
		"native.action(\"is.workflow.actions.gettext\", WFTextActionText:question)",
		"setBackgroundSound(question)",
	} {
		data, err := CompileSourceToPlist("setup-reject.cherri", setup+body)
		if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeSetupBinding) {
			t.Errorf("setup misuse not rejected at shared analysis boundary: %v", err)
		}
	}
}

func TestP15M04CatalogInsertionHoles(t *testing.T) {
	catalog, _ := p15CurrentCatalog(t)
	count := 0
	for _, entry := range catalog {
		if strings.Contains(entry.InsertionSnippet, "<#") {
			count++
			if data, err := CompileSourceToPlist("unfilled-palette.cherri", entry.InsertionSnippet); err == nil || len(data) != 0 {
				t.Errorf("unfilled %s emitted native output", entry.Name)
			}
			mobile := compileForMobileMode(entry.InsertionSnippet, "Unfilled palette", "v2", false)
			if mobile.OK {
				t.Errorf("mobile compiler accepted unfilled %s", entry.Name)
			}
		}
		if strings.Contains(entry.InsertionSnippet, "(Ask") || strings.Contains(entry.InsertionSnippet, ", Ask") || strings.Contains(entry.InsertionSnippet, "= Ask") || strings.Contains(entry.InsertionSnippet, "in Ask") {
			t.Error("palette silently supplies Ask", entry.Name)
		}
	}
	if count < 100 {
		t.Fatal("required palette holes not covered", count)
	}
	t.Logf("actual unfilled palette cases rejected by compiler and mobile bridge: %d", count)
}

func TestP15M03SetupBindingRejectsIdentityCorruption(t *testing.T) {
	for _, fault := range []string{"control", "missing_node", "duplicate_node", "missing_parameter", "reused_question", "late_prefix", "uuid_rename"} {
		t.Run(fault, func(t *testing.T) {
			session := compileMetaSourceToSession(t, "setup-identity", "setup item: Text { prompt: \"identity\" default: \"value\" }\ntext(item)")
			defer session.Close()
			if len(session.shortcut.WFWorkflowImportQuestions) != 1 || len(session.importQuestionBindings) != 1 {
				t.Fatal("binding missing")
			}
			binding := session.importQuestionBindings[0]
			failedAtBind := false
			switch fault {
			case "missing_node":
				session.shortcut.WFWorkflowActions = nil
			case "duplicate_node":
				session.shortcut.WFWorkflowActions = append(session.shortcut.WFWorkflowActions, session.shortcut.WFWorkflowActions[0])
			case "missing_parameter":
				delete(session.shortcut.WFWorkflowActions[0].WFWorkflowActionParameters, "WFTextActionText")
			case "reused_question":
				failedAtBind = session.BindImportQuestion("item", "WFTextActionText", binding.nodeID) != nil
			case "late_prefix":
				session.shortcut.WFWorkflowActions = append([]ShortcutAction{{WFWorkflowActionIdentifier: "is.workflow.actions.comment", WFWorkflowActionParameters: map[string]any{"WFCommentActionText": "later inserted"}}}, session.shortcut.WFWorkflowActions...)
			case "uuid_rename":
				session.shortcut.WFWorkflowActions[0].WFWorkflowActionParameters["UUID"] = "10000000-0000-4000-8000-000000000001"
			}
			data, err := session.Finalize()
			if fault == "control" || fault == "late_prefix" || fault == "uuid_rename" {
				if err != nil {
					t.Fatal(err)
				}
				var root map[string]any
				if _, err := plist.Unmarshal(data, &root); err != nil {
					t.Fatal(err)
				}
				if err := p15SetupQuestionError(root, "identity", "is.workflow.actions.gettext", "WFTextActionText", "value"); err != nil {
					t.Fatal(err)
				}
				// Finalization can be repeated without losing declaration indices.
				second, err := session.Finalize()
				if err != nil || string(data) != string(second) {
					t.Fatal("finalization not stable", err)
				}
			} else if fault == "reused_question" {
				if !failedAtBind {
					t.Fatal("reused node/parameter binding accepted")
				}
			} else if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeSetupBinding) {
				t.Fatal("corrupt identity emitted a shortcut", fault, err)
			}
		})
	}
}
