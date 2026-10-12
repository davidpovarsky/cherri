package main

import (
	"fmt"
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
		if !strings.Contains(err.Error(), "unsupported import") {
			t.Errorf("expected error mentioning unsupported import, got: %v", err)
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
	t.Run("ValidEnum", func(t *testing.T) {
		codeEnum := `
enum Environment {
    staging = "STAGING",
    prod = "PRODUCTION"
}
quicklook(Environment.prod)
`
		session := compileMetaSourceToSession(t, "lang002_enum", codeEnum)
		defer session.Close()

		found := false
		for _, act := range session.shortcut.WFWorkflowActions {
			if act.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				found = true
				input := act.WFWorkflowActionParameters["WFInput"]
				if input != "PRODUCTION" {
					t.Errorf("expected quicklook input to be 'PRODUCTION', got %v", input)
				}
			}
		}
		if !found {
			t.Fatalf("expected quicklook action")
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
native.plist("key")
`
		err := compileMetaSourceExpectingError(t, "lang003_plist", codePlist)
		if err == nil || !strings.Contains(err.Error(), "native.plist is unsupported") {
			t.Fatalf("expected explicit error for native.plist, got: %v", err)
		}
	})

	t.Run("NativeRef", func(t *testing.T) {
		codeRef := `
native.ref("output")
`
		err := compileMetaSourceExpectingError(t, "lang003_ref", codeRef)
		if err == nil || !strings.Contains(err.Error(), "native.ref is unsupported") {
			t.Fatalf("expected explicit error for native.ref, got: %v", err)
		}
	})

	t.Run("NativeWorkflow", func(t *testing.T) {
		codeWf := `
native.workflow("wf")
`
		err := compileMetaSourceExpectingError(t, "lang003_wf", codeWf)
		if err == nil || !strings.Contains(err.Error(), "native.workflow is unsupported") {
			t.Fatalf("expected explicit error for native.workflow, got: %v", err)
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
