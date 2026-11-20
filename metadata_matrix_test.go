package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func compileMetaSourceToSession(t *testing.T, filename, code string) *CanonicalBackendSession {
	t.Helper()
	file := source.NewFile(source.DocumentID(filename), filename, 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("syntax errors: %v", parser.Errors())
	}

	session := NewCanonicalBackendSession()
	lowerer := lower.NewLowererWithSession(nil, session)
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		session.Close()
		t.Fatalf("lowering failed: %v", err)
	}
	return session
}

func compileMetaSourceExpectingError(t *testing.T, filename, code string) error {
	t.Helper()
	file := source.NewFile(source.DocumentID(filename), filename, 1, code)
	parser := syntax.NewParser(file)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		return fmt.Errorf("syntax errors: %v", parser.Errors())
	}

	session := NewCanonicalBackendSession()
	defer session.Close()
	lowerer := lower.NewLowererWithSession(nil, session)
	return lowerer.LowerProgramToSession(prog)
}

// META-001: QuickActions/share sheet, inputs/outputs, icon
func TestMETA001_WorkflowHeaderMetadata(t *testing.T) {
	code := `
shortcut "MetaShortcut" {
    icon: { glyph: 59789, color: 4282601983 }
    from: ["sharesheet", "quickactions"]
    inputs: ["text", "image"]
    outputs: ["url"]
}
quicklook("test")
`
	session := compileMetaSourceToSession(t, "meta001", code)
	defer session.Close()

	sc := session.shortcut

	// 1. ClientVersion must be canonical clientVersion (not hardcoded 4711)
	if sc.WFWorkflowClientVersion == "4711" {
		t.Errorf("expected canonical clientVersion, got hardcoded 4711")
	}
	if sc.WFWorkflowClientVersion != clientVersion {
		t.Errorf("expected clientVersion %q, got %q", clientVersion, sc.WFWorkflowClientVersion)
	}

	// 2. Icon glyph and color
	if sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber != 59789 {
		t.Errorf("expected glyph 59789, got %d", sc.WFWorkflowIcon.WFWorkflowIconGlyphNumber)
	}
	if sc.WFWorkflowIcon.WFWorkflowIconStartColor != 4282601983 {
		t.Errorf("expected color 4282601983, got %d", sc.WFWorkflowIcon.WFWorkflowIconStartColor)
	}

	// 3. WorkflowTypes: ActionExtension and QuickActions
	hasExt, hasQA := false, false
	for _, typ := range sc.WFWorkflowTypes {
		if typ == "ActionExtension" {
			hasExt = true
		}
		if typ == "QuickActions" {
			hasQA = true
		}
	}
	if !hasExt || !hasQA {
		t.Errorf("expected ActionExtension and QuickActions in WFWorkflowTypes, got %v", sc.WFWorkflowTypes)
	}

	// 4. QuickActionSurfaces: Finder and Services
	if len(sc.WFQuickActionSurfaces) == 0 {
		t.Errorf("expected QuickActionSurfaces to be populated")
	}

	// 5. InputContentItemClasses: WFStringContentItem and WFImageContentItem
	hasString, hasImage := false, false
	for _, cls := range sc.WFWorkflowInputContentItemClasses {
		if cls == "WFStringContentItem" {
			hasString = true
		}
		if cls == "WFImageContentItem" {
			hasImage = true
		}
	}
	if !hasString || !hasImage {
		t.Errorf("expected WFStringContentItem and WFImageContentItem in inputs, got %v", sc.WFWorkflowInputContentItemClasses)
	}

	// 6. OutputContentItemClasses: WFURLContentItem
	hasURL := false
	for _, cls := range sc.WFWorkflowOutputContentItemClasses {
		if cls == "WFURLContentItem" {
			hasURL = true
		}
	}
	if !hasURL {
		t.Errorf("expected WFURLContentItem in outputs, got %v", sc.WFWorkflowOutputContentItemClasses)
	}
}

// META-002: user no-input behavior, questions
func TestMETA002_NoInputBehaviorAndQuestions(t *testing.T) {
	code := `
shortcut "NoInputTest" {
    noInput: "clipboard"
}
setup ApiKey: String {
    prompt: "Enter Key"
    default: "secret"
}
quicklook(ApiKey)
`
	session := compileMetaSourceToSession(t, "meta002", code)
	defer session.Close()

	sc := session.shortcut

	// 1. WFWorkflowNoInputBehavior
	if sc.WFWorkflowNoInputBehavior == nil {
		t.Fatalf("expected WFWorkflowNoInputBehavior to be set")
	}
	if sc.WFWorkflowNoInputBehavior["Name"] != "WFWorkflowNoInputBehaviorGetClipboard" {
		t.Errorf("expected WFWorkflowNoInputBehaviorGetClipboard, got %v", sc.WFWorkflowNoInputBehavior)
	}

	// 2. WFWorkflowImportQuestions
	if len(sc.WFWorkflowImportQuestions) != 1 {
		t.Fatalf("expected 1 import question, got %d", len(sc.WFWorkflowImportQuestions))
	}
	q := sc.WFWorkflowImportQuestions[0]
	if q.Text != "Enter Key" {
		t.Errorf("expected prompt 'Enter Key', got %q", q.Text)
	}
	if q.DefaultValue != "secret" {
		t.Errorf("expected defaultValue 'secret', got %q", q.DefaultValue)
	}
	if q.ActionIndex < 0 {
		t.Errorf("expected ActionIndex >= 0, got %d", q.ActionIndex)
	}
}

// META-003: supported trigger vs unsupported trigger
func TestMETA003_Triggers(t *testing.T) {
	t.Run("Supported", func(t *testing.T) {
		codeSupported := `
shortcut "AutoShort" {
    trigger bluetooth(event: .connect)
    trigger wifi(event: .joined)
}
quicklook("ok")
`
		session := compileMetaSourceToSession(t, "meta003_ok", codeSupported)
		defer session.Close()

		sc := session.shortcut
		if len(sc.WFWorkflowTriggers) != 2 {
			t.Fatalf("expected 2 triggers, got %d", len(sc.WFWorkflowTriggers))
		}
		if sc.WFWorkflowTriggers[0].WFTriggerIdentifier != "WFBluetoothTrigger" {
			t.Errorf("expected WFBluetoothTrigger, got %q", sc.WFWorkflowTriggers[0].WFTriggerIdentifier)
		}
		if sc.WFWorkflowTriggers[1].WFTriggerIdentifier != "WFWifiTrigger" {
			t.Errorf("expected WFWifiTrigger, got %q", sc.WFWorkflowTriggers[1].WFTriggerIdentifier)
		}
	})

	t.Run("Unsupported", func(t *testing.T) {
		codeUnsupported := `
shortcut "BadTrigger" {
    trigger unknownDevice(event: .connect)
}
quicklook("bad")
`
		err := compileMetaSourceExpectingError(t, "meta003_err", codeUnsupported)
		if err == nil {
			t.Fatalf("expected unsupported trigger to fail with error, but compiled successfully")
		}
		if !strings.Contains(err.Error(), "unsupported trigger") {
			t.Errorf("expected error to mention unsupported trigger, got: %v", err)
		}
	})
}

// LANG-001: imports / custom action definitions
func TestLANG001_ImportsAndCustomAction(t *testing.T) {
	t.Run("ImportRejected", func(t *testing.T) {
		codeImport := `
import "stdlib/math" as math
quicklook("hello")
`
		err := compileMetaSourceExpectingError(t, "lang001_import", codeImport)
		if err == nil {
			t.Fatalf("expected import to fail with explicit error, but it succeeded silently")
		}
		if !strings.Contains(err.Error(), "cannot resolve module") && !strings.Contains(err.Error(), "unsupported import") {
			t.Errorf("expected error mentioning module resolution failure or unsupported import, got: %v", err)
		}
	})

	t.Run("CustomAction", func(t *testing.T) {
		codeAction := `
action customNotify(msg: String) {
    identifier: "is.workflow.actions.notification"
    primary: "msg"
    bindings: {
        msg: {
            key: "WFNotificationActionBody"
        }
    }
}
customNotify("Build Complete")
`
		session := compileMetaSourceToSession(t, "lang001_action", codeAction)
		defer session.Close()

		foundAction := false
		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.notification" {
				foundAction = true
				if act.WFWorkflowActionParameters["WFNotificationActionBody"] != "Build Complete" {
					t.Errorf("expected WFNotificationActionBody to be 'Build Complete', got %v", act.WFWorkflowActionParameters["WFNotificationActionBody"])
				}
			}
		}
		if !foundAction {
			t.Fatalf("expected custom action is.workflow.actions.notification to be emitted")
		}
	})
}

// LANG-002: enum / type aliases
func TestLANG002_EnumAndTypeAliases(t *testing.T) {
	t.Run("LiteralReferenceControl", func(t *testing.T) {
		lang002CompileNormalizedGraph(t, "lang002_literal", "quicklook(\"PRODUCTION\")")
	})
	t.Run("ValidEnum", func(t *testing.T) {
		enumGraph := lang002CompileNormalizedGraph(t, "lang002_enum", lang002EnumSource)
		literalGraph := lang002CompileNormalizedGraph(t, "lang002_literal_parity", "quicklook(\"PRODUCTION\")")
		if !reflect.DeepEqual(enumGraph, literalGraph) {
			t.Fatal("enum and ordinary literal differ beyond generated producer UUIDs")
		}
	})

	t.Run("InvalidEnum", func(t *testing.T) {
		codeInvalidEnum := `
enum Environment {
    staging = "STAGING"
}
quicklook(Environment.invalid)
`
		err := compileMetaSourceExpectingError(t, "lang002_err", codeInvalidEnum)
		if err == nil {
			t.Fatalf("expected error for invalid enum member, but compiled successfully")
		}
		if !strings.Contains(err.Error(), "has no member") {
			t.Errorf("expected error mentioning missing member, got: %v", err)
		}
	})
}

// The canonical declaration in actions/basic.cherri uses variable input: WFInput.
// Its generated schema accepts attachment(output)/attachment(variable), codec
// token. Enum strings therefore use the same one-Text producer as ordinary
// string arguments. Validate the value and exact edge, not merely a reference.
const lang002EnumSource = "enum Environment {\n    staging = \"STAGING\",\n    prod = \"PRODUCTION\"\n}\nquicklook(Environment.prod)\n"

func lang002CompileNormalizedGraph(t *testing.T, filename, source string) []ShortcutAction {
	t.Helper()
	session := compileMetaSourceToSession(t, filename, source)
	defer session.Close()
	actions := session.shortcut.WFWorkflowActions
	if err := lang002QuickLookStringGraph(actions, "PRODUCTION"); err != nil {
		t.Fatal(err)
	}
	// Snapshot the graph and release the global backend transaction before
	// compiling the independent control. Never nest canonical sessions.
	return lang002NormalizedGraph(actions)
}

func lang002QuickLookStringGraph(actions []ShortcutAction, expected string) error {
	if len(actions) != 3 {
		return fmt.Errorf("expected Text, Quick Look and main Nothing, got %d actions", len(actions))
	}
	if err := debugMainVoidBoundaryError(actions); err != nil {
		return err
	}
	producer, consumer := actions[0], actions[1]
	if producer.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		consumer.WFWorkflowActionIdentifier != "is.workflow.actions.previewdocument" {
		return fmt.Errorf("expected ordered Text -> Quick Look graph")
	}
	pp := producer.WFWorkflowActionParameters
	if pp["WFTextActionText"] != expected || pp["CustomOutputName"] != "Text" {
		return fmt.Errorf("wrong Text producer value/name: %#v", pp)
	}
	uid, ok := pp["UUID"].(string)
	if !ok || uid == "" {
		return fmt.Errorf("Text producer has no UUID")
	}
	input, ok := consumer.WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment)
	if !ok || input.WFSerializationType != "WFTextTokenAttachment" {
		return fmt.Errorf("Quick Look requires the canonical attachment, got %#v", consumer.WFWorkflowActionParameters["WFInput"])
	}
	want := Value{Type: "ActionOutput", OutputUUID: uid, OutputName: "Text"}
	if !reflect.DeepEqual(input.Value, want) {
		return fmt.Errorf("Quick Look must consume exactly the preceding Text UUID without coercions: %#v", input.Value)
	}
	return nil
}

func lang002CloneActions(actions []ShortcutAction) []ShortcutAction {
	cloned := append([]ShortcutAction(nil), actions...)
	for i := range cloned {
		cloned[i].WFWorkflowActionParameters = make(map[string]any, len(actions[i].WFWorkflowActionParameters))
		for key, value := range actions[i].WFWorkflowActionParameters {
			cloned[i].WFWorkflowActionParameters[key] = value
		}
	}
	return cloned
}

func lang002NormalizedGraph(actions []ShortcutAction) []ShortcutAction {
	normalized := lang002CloneActions(actions)
	for i := range normalized {
		if _, present := normalized[i].WFWorkflowActionParameters["UUID"]; present {
			normalized[i].WFWorkflowActionParameters["UUID"] = fmt.Sprintf("LANG002_ACTION_%d", i)
		}
	}
	input := normalized[1].WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment)
	input.Value.OutputUUID = "LANG002_ACTION_0"
	normalized[1].WFWorkflowActionParameters["WFInput"] = input
	return normalized
}

func TestLANG002ScalarReferenceOracleRejectsCorruption(t *testing.T) {
	session := compileMetaSourceToSession(t, "lang002_causal", lang002EnumSource)
	defer session.Close()
	actual := session.shortcut.WFWorkflowActions
	if err := lang002QuickLookStringGraph(actual, "PRODUCTION"); err != nil {
		t.Fatal(err)
	}
	mutations := []struct {
		name  string
		alter func([]ShortcutAction) []ShortcutAction
	}{
		{"wrong_text", func(a []ShortcutAction) []ShortcutAction {
			a[0].WFWorkflowActionParameters["WFTextActionText"] = "STAGING"
			return a
		}},
		{"unbound_uuid", func(a []ShortcutAction) []ShortcutAction {
			input := a[1].WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment)
			input.Value.OutputUUID = "UNBOUND"
			a[1].WFWorkflowActionParameters["WFInput"] = input
			return a
		}},
		{"literal_bypasses_required_reference", func(a []ShortcutAction) []ShortcutAction {
			a[1].WFWorkflowActionParameters["WFInput"] = "PRODUCTION"
			return a
		}},
		{"variable_instead_of_exact_output", func(a []ShortcutAction) []ShortcutAction {
			input := a[1].WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment)
			input.Value = Value{Type: "Variable", VariableName: "PRODUCTION"}
			a[1].WFWorkflowActionParameters["WFInput"] = input
			return a
		}},
		{"unexpected_coercion", func(a []ShortcutAction) []ShortcutAction {
			input := a[1].WFWorkflowActionParameters["WFInput"].(WFTextTokenAttachment)
			input.Value.Aggrandizements = []Aggrandizement{{Type: "WFCoercionVariableAggrandizement", CoercionItemClass: "WFNumberContentItem"}}
			a[1].WFWorkflowActionParameters["WFInput"] = input
			return a
		}},
		{"missing_main_clear", func(a []ShortcutAction) []ShortcutAction { return a[:2] }},
		{"main_clear_replaced", func(a []ShortcutAction) []ShortcutAction {
			a[2].WFWorkflowActionIdentifier = "is.workflow.actions.gettext"
			return a
		}},
		{"missing_producer", func(a []ShortcutAction) []ShortcutAction { return a[1:] }},
		{"duplicated_producer", func(a []ShortcutAction) []ShortcutAction {
			return []ShortcutAction{a[0], a[0], a[1], a[2]}
		}},
		{"consumer_before_producer", func(a []ShortcutAction) []ShortcutAction {
			return []ShortcutAction{a[1], a[0], a[2]}
		}},
		{"wrong_native_consumer", func(a []ShortcutAction) []ShortcutAction {
			a[1].WFWorkflowActionIdentifier = "is.workflow.actions.showresult"
			return a
		}},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			damaged := mutation.alter(lang002CloneActions(actual))
			if err := lang002QuickLookStringGraph(damaged, "PRODUCTION"); err == nil {
				t.Fatal("damaged actual graph was accepted")
			}
		})
	}
}

// LANG-003: all native escape APIs documented
func TestLANG003_NativeEscapeAPIs(t *testing.T) {
	t.Run("NativeAction", func(t *testing.T) {
		codeNativeAction := `
native.action("is.workflow.actions.vibrate")
`
		session := compileMetaSourceToSession(t, "lang003_act", codeNativeAction)
		defer session.Close()

		found := false
		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.vibrate" {
				found = true
			}
		}
		if !found {
			t.Fatalf("expected native.action to emit vibrate")
		}
	})

	t.Run("NativePlist", func(t *testing.T) {
		codePlist := `
native.action("com.apple.custom.data", parameters: {
    "Payload": native.plist({ "CustomKey": "CustomVal" })
})
`
		session := compileMetaSourceToSession(t, "lang003_plist", codePlist)
		defer session.Close()

		found := false
		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "com.apple.custom.data" {
				found = true
				params := act.WFWorkflowActionParameters
				if payload, ok := params["Payload"].(map[string]any); !ok || payload["CustomKey"] != "CustomVal" {
					t.Errorf("expected native.plist Payload to have CustomKey=CustomVal, got %v", params["Payload"])
				}
			}
		}
		if !found {
			t.Fatalf("expected native.action with native.plist to emit com.apple.custom.data")
		}
	})

	t.Run("NativeRef", func(t *testing.T) {
		codeRef := `
native.action("com.apple.custom.ref", parameters: {
    "Output": native.ref("ActionOutput", uuid: "11111111-2222-3333-4444-555555555555")
})
`
		session := compileMetaSourceToSession(t, "lang003_ref", codeRef)
		defer session.Close()

		found := false
		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "com.apple.custom.ref" {
				found = true
				params := act.WFWorkflowActionParameters
				if att, ok := params["Output"].(WFTextTokenAttachment); ok {
					if att.WFSerializationType != "WFTextTokenAttachment" || att.Value.OutputUUID != "11111111-2222-3333-4444-555555555555" {
						t.Errorf("unexpected attachment: %+v", att)
					}
				} else if out, ok := params["Output"].(map[string]any); ok {
					if out["WFSerializationType"] != "WFTextTokenAttachment" {
						t.Errorf("expected native.ref WFTextTokenAttachment, got %v", params["Output"])
					}
				} else {
					t.Fatalf("unexpected type for Output: %T (%+v)", params["Output"], params["Output"])
				}
			}
		}
		if !found {
			t.Fatalf("expected native.action with native.ref to emit com.apple.custom.ref")
		}
	})

	t.Run("NativeWorkflow", func(t *testing.T) {
		codeWf := `
native.workflow({
    "WFWorkflowClientVersion": "2600.0",
    "WFWorkflowActions": []
})
`
		session := compileMetaSourceToSession(t, "lang003_wf", codeWf)
		defer session.Close()

		if session.shortcut.WFWorkflowClientVersion != "2600.0" {
			t.Errorf("expected client version 2600.0, got %s", session.shortcut.WFWorkflowClientVersion)
		}
	})

	t.Run("NativeNegativeFailClosed", func(t *testing.T) {
		errEmptyAct := compileMetaSourceExpectingError(t, "lang003_neg_act", `native.action("", {})`)
		if errEmptyAct == nil {
			t.Errorf("expected empty action id to fail")
		}
		errBadRef := compileMetaSourceExpectingError(t, "lang003_neg_ref", `native.action("test", parameters: { "r": native.ref("UnknownKind") })`)
		if errBadRef == nil {
			t.Errorf("expected unknown native.ref kind to fail")
		}
	})
}

// LANG-004: unhandled declaration AST is never silently dropped
func TestLANG004_UnhandledDeclarationsNotSilent(t *testing.T) {
	code := `
import "unknown/module" as unk
`
	err := compileMetaSourceExpectingError(t, "lang004", code)
	if err == nil {
		t.Fatalf("expected unhandled declaration to produce compile error, got nil")
	}
}

// LANG-005: legacy -> v2 migration with strings/comments
func TestLANG005_MigrationSafeCommentsAndVars(t *testing.T) {
	legacyCode := `// Contact: user@example.com for help
@count = 1
@count += 1
quicklook(@count)
`
	migrated := migrate.MigrateSource(legacyCode)

	// 1. Comment must preserve @ in user@example.com
	if !strings.Contains(migrated, "user@example.com") {
		t.Errorf("expected migration to preserve user@example.com in comments, got:\n%s", migrated)
	}

	// 2. @count declared and reassigned should become var count = 1
	if !strings.Contains(migrated, "var count = 1") {
		t.Errorf("expected '@count = 1' to become 'var count = 1', got:\n%s", migrated)
	}
	if !strings.Contains(migrated, "count += 1") {
		t.Errorf("expected '@count += 1' to become 'count += 1', got:\n%s", migrated)
	}
}
