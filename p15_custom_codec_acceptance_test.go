package main

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
	"howett.net/plist"
)

const p15TokenDeclaration = "action p15Token(value: AnyContent) -> Text {\nidentifier: \"is.workflow.actions.detect.text\"\nprimary: \"value\"\nbindings: {value: {key: \"WFInput\", codec: \"token\"}}\n}\n"

func p15CodecNative(t *testing.T, source string) map[string]any {
	t.Helper()
	raw, err := CompileSourceToPlist("custom-codec.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	root, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func p15TokenCodecGraphError(root map[string]any, producer string) error {
	rows, ok := root["WFWorkflowActions"].([]any)
	wantCount := 1
	if producer != "" {
		wantCount = 2
	}
	if !ok || len(rows) != wantCount {
		return fmt.Errorf("expected %d native actions, got %d", wantCount, len(rows))
	}
	target := rows[len(rows)-1].(map[string]any)
	if target["WFWorkflowActionIdentifier"] != "is.workflow.actions.detect.text" {
		return fmt.Errorf("wrong target identity")
	}
	params := target["WFWorkflowActionParameters"].(map[string]any)
	if len(params) != 1 {
		return fmt.Errorf("unexpected target fields: %#v", params)
	}
	token, ok := params["WFInput"].(map[string]any)
	if !ok || token["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("token codec emitted a non-attachment: %#v", params["WFInput"])
	}
	value, ok := token["Value"].(map[string]any)
	if !ok {
		return fmt.Errorf("attachment has no dictionary value")
	}
	if producer == "" {
		if len(value) != 1 || value["Type"] != "ExtensionInput" {
			return fmt.Errorf("system reference changed: %#v", value)
		}
		return nil
	}
	first := rows[0].(map[string]any)
	if first["WFWorkflowActionIdentifier"] != producer {
		return fmt.Errorf("literal producer changed: %#v", first)
	}
	pp := first["WFWorkflowActionParameters"].(map[string]any)
	uuid, ok := pp["UUID"].(string)
	name, named := pp["CustomOutputName"].(string)
	if !named && producer == "is.workflow.actions.gettext" {
		// Ordinary canonical text calls omit CustomOutputName while their
		// reference uses the native action's default display name.
		name = "Text"
	}
	if !ok || uuid == "" || value["Type"] != "ActionOutput" || value["OutputUUID"] != uuid || value["OutputName"] != name {
		return fmt.Errorf("attachment lost exact producer edge: %#v", value)
	}
	if producer == "is.workflow.actions.gettext" {
		if _, ok := pp["WFTextActionText"].(string); !ok {
			return fmt.Errorf("literal text producer lost literal bytes")
		}
	}
	return nil
}

func TestP15CustomTokenCodecCanonicalParity(t *testing.T) {
	for _, tc := range []struct{ name, canonical, custom, decl, producer string }{
		{"text_positional", `getText("hello")`, `p15Token("hello")`, "", "is.workflow.actions.gettext"},
		{"text_named", `getText(input: "hello")`, `p15Token(value: "hello")`, "", "is.workflow.actions.gettext"},
		{"empty_text", `getText("")`, `p15Token("")`, "", "is.workflow.actions.gettext"},
		{"zero", `getText(0)`, `p15Token(0)`, "", "is.workflow.actions.number"},
		{"fraction", `getText(2.5)`, `p15Token(2.5)`, "", "is.workflow.actions.number"},
		{"negative", `getText(-2.5)`, `p15Token(-2.5)`, "", "is.workflow.actions.number"},
		{"false", `getText(false)`, `p15Token(false)`, "", "is.workflow.actions.number"},
		{"default_text", `getText("hello")`, `p15Token()`, strings.Replace(p15TokenDeclaration, "value: AnyContent)", "value: AnyContent = \"hello\")", 1), "is.workflow.actions.gettext"},
		{"default_zero", `getText(0)`, `p15Token()`, strings.Replace(p15TokenDeclaration, "value: AnyContent)", "value: AnyContent = 0)", 1), "is.workflow.actions.number"},
		{"system_reference", `getText(ShortcutInput)`, `p15Token(ShortcutInput)`, "", ""},
		{"named_system_reference", `getText(input: ShortcutInput)`, `p15Token(value: ShortcutInput)`, "", ""},
		{"producer_reference", `getText(text("hello"))`, `p15Token(text("hello"))`, "", "is.workflow.actions.gettext"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			decl := tc.decl
			if decl == "" {
				decl = p15TokenDeclaration
			}
			canonical := p15CodecNative(t, tc.canonical)
			custom := p15CodecNative(t, decl+tc.custom)
			if err := p15TokenCodecGraphError(canonical, tc.producer); err != nil {
				t.Fatal("canonical", err)
			}
			if err := p15TokenCodecGraphError(custom, tc.producer); err != nil {
				t.Fatal("custom", err)
			}
			if result := shortcutcompare.CompareDocumentsStrict(canonical, custom); !result.Equal {
				t.Fatalf("public custom codec differs from canonical codec: %+v", result.Differences)
			}
			for _, fault := range []string{"bare_literal", "missing_input", "wrong_token_type", "wrong_target"} {
				changed := deepClonePlistMap(custom)
				rows := changed["WFWorkflowActions"].([]any)
				target := rows[len(rows)-1].(map[string]any)
				params := target["WFWorkflowActionParameters"].(map[string]any)
				switch fault {
				case "bare_literal":
					params["WFInput"] = "hello"
				case "missing_input":
					delete(params, "WFInput")
				case "wrong_token_type":
					params["WFInput"].(map[string]any)["Value"].(map[string]any)["Type"] = "Clipboard"
				case "wrong_target":
					target["WFWorkflowActionIdentifier"] = "com.example.p15.wrong"
				}
				if p15TokenCodecGraphError(changed, tc.producer) == nil || shortcutcompare.CompareDocumentsStrict(canonical, changed).Equal {
					t.Fatal("corrupted codec output passed", fault)
				}
			}
		})
	}
}

func TestP15CustomTokenCodecSecondaryAndImplicitBindings(t *testing.T) {
	for _, source := range []string{
		"action p15Second(label: Text, value: AnyContent) {identifier: \"com.example.p15.two\"\nbindings:{label:{key:\"Label\",codec:\"text\"},value:{key:\"Payload\",codec:\"token\"}}}\np15Second(\"label\",\"hello\")",
		"action p15Second(label: Text, value: AnyContent) {identifier: \"com.example.p15.two\"\nbindings:{label:{key:\"Label\",codec:\"text\"},value:{key:\"Payload\",codec:\"token\"}}}\np15Second(value:\"hello\",label:\"label\")",
	} {
		root := p15CodecNative(t, source)
		rows := root["WFWorkflowActions"].([]any)
		if len(rows) != 2 {
			t.Fatalf("secondary token literal was not materialized: %#v", rows)
		}
		producer := rows[0].(map[string]any)
		if producer["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
			t.Fatal("wrong token producer")
		}
		pp := producer["WFWorkflowActionParameters"].(map[string]any)
		if pp["WFTextActionText"] != "hello" {
			t.Fatal("token literal changed")
		}
		target := rows[1].(map[string]any)
		params := target["WFWorkflowActionParameters"].(map[string]any)
		token, ok := params["Payload"].(map[string]any)
		if target["WFWorkflowActionIdentifier"] != "com.example.p15.two" || params["Label"] != "label" || !ok || token["WFSerializationType"] != "WFTextTokenAttachment" {
			t.Fatalf("secondary binding wrong: %#v", target)
		}
		value := token["Value"].(map[string]any)
		if value["Type"] != "ActionOutput" || value["OutputUUID"] != pp["UUID"] {
			t.Fatal("secondary token references another producer")
		}
	}
	for _, codec := range []string{"", ",codec:\"text\""} {
		source := "action p15Plain(value:Text) {identifier:\"is.workflow.actions.gettext\"\nbindings:{value:{key:\"WFTextActionText\"" + codec + "}}}\np15Plain(\"hello\")"
		custom := p15CodecNative(t, source)
		if !shortcutcompare.CompareDocumentsStrict(p15CodecNative(t, "text(\"hello\")"), custom).Equal {
			t.Fatal("non-token custom declaration changed")
		}
	}
}

func TestP15CustomCodecSharedReferenceCausality(t *testing.T) {
	original := SharedCodecHook
	defer func() { SharedCodecHook = original }()
	for _, source := range []string{"getText(ShortcutInput)", p15TokenDeclaration + "p15Token(ShortcutInput)"} {
		before := p15CodecNative(t, source)
		if err := p15TokenCodecGraphError(before, ""); err != nil {
			t.Fatal(err)
		}
		calls := 0
		SharedCodecHook = func(desc *ReferenceDescriptor, value *Value) {
			if original != nil {
				original(desc, value)
			}
			if desc.Kind == "ExtensionInput" {
				calls++
				value.Type = "Clipboard"
			}
		}
		changed := p15CodecNative(t, source)
		SharedCodecHook = original
		if calls != 1 || p15TokenCodecGraphError(changed, "") == nil || shortcutcompare.CompareDocumentsStrict(before, changed).Equal {
			t.Fatalf("shared codec mutation did not govern route: calls=%d", calls)
		}
		restored := p15CodecNative(t, source)
		if !shortcutcompare.CompareDocumentsStrict(before, restored).Equal {
			t.Fatal("codec restoration did not restore production wire")
		}
	}
}

func TestP15CustomTokenCodecPublicRoutes(t *testing.T) {
	source := p15TokenDeclaration + "p15Token(value: \"hello\")"
	expected := p15CodecNative(t, "getText(\"hello\")")
	svc := service.NewService(nil)
	svc.OpenDocument("file:///custom-codec.cherri", 1, source)
	diagnostics, _ := svc.Analyze("file:///custom-codec.cherri")
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	mobile := compileForMobileMode(source, "Custom codec", "v2", false)
	if !mobile.OK {
		t.Fatal(mobile.Error)
	}
	raw, err := base64.StdEncoding.DecodeString(mobile.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	var mobileRoot map[string]any
	if _, err := plist.Unmarshal(raw, &mobileRoot); err != nil {
		t.Fatal(err)
	}
	if err := p15TokenCodecGraphError(mobileRoot, "is.workflow.actions.gettext"); err != nil {
		t.Fatal("mobile", err)
	}
	if !shortcutcompare.CompareDocumentsStrict(expected, mobileRoot).Equal {
		t.Fatal("mobile codec differs")
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "codec.cherri"), filepath.Join(dir, "codec.shortcut")
	if err := os.WriteFile(input, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	binary := p15AcceptanceCLI(t)
	stdout, stderr, err := p15RunCLI(t, binary, dir, nil, "check", input, "--json")
	if err != nil || p15DecodeOneJSON(t, stdout).(map[string]any)["valid"] != true {
		t.Fatalf("CLI validation %v %s", err, stderr)
	}
	stdout, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--skip-sign", "--output="+output)
	if err != nil {
		t.Fatalf("CLI compile %v %s %s", err, stdout, stderr)
	}
	raw, err = os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	cliRoot, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15TokenCodecGraphError(cliRoot, "is.workflow.actions.gettext"); err != nil {
		t.Fatal("CLI", err)
	}
	if !shortcutcompare.CompareDocumentsStrict(expected, cliRoot).Equal {
		t.Fatal("CLI codec differs")
	}
}
