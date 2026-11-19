package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

const p15VCardPrefix = "BEGIN:VCARD\nVERSION:3.0\nN;CHARSET=utf-8:"

func p15VCardTokenStringOracle(actions []ShortcutAction) error {
	if len(actions) != 3 {
		return fmt.Errorf("want two producers and one VCard text action, got %d", len(actions))
	}
	for i, value := range []string{"P15_TITLE", "P15_SUBTITLE"} {
		if actions[i].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
			actions[i].WFWorkflowActionParameters["WFTextActionText"] != value {
			return fmt.Errorf("producer %d lost its literal identity: %#v", i, actions[i])
		}
	}
	if actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
		return fmt.Errorf("VCard is not emitted by the established Text action")
	}
	token := debugParamMap(actions[2].WFWorkflowActionParameters["WFTextActionText"])
	if token["WFSerializationType"] != "WFTextTokenString" {
		return fmt.Errorf("VCard lost structured token string: %#v", token)
	}
	value := debugParamMap(token["Value"])
	titlePrefix := p15VCardPrefix + "\U0001f30d "
	subtitlePrefix := titlePrefix + "\ufffc\nORG:e\u0301 "
	want := subtitlePrefix + "\ufffc\nEND:VCARD"
	if value["string"] != want {
		return fmt.Errorf("VCard text changed: got %#v, want %q", value["string"], want)
	}
	attachments := debugParamMap(value["attachmentsByRange"])
	if len(attachments) != 2 {
		return fmt.Errorf("want exactly two runtime attachments, got %#v", attachments)
	}
	seen := map[string]bool{}
	for i, prefix := range []string{titlePrefix, subtitlePrefix} {
		key := fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(prefix))))
		ref := debugParamMap(attachments[key])
		producer, _ := actions[i].WFWorkflowActionParameters["UUID"].(string)
		if producer == "" || seen[producer] || ref["Type"] != "ActionOutput" || ref["OutputUUID"] != producer {
			return fmt.Errorf("attachment %s does not reference producer %d exactly: %#v", key, i, ref)
		}
		seen[producer] = true
	}
	return nil
}

func p15CloneVCardActions(t *testing.T, actions []ShortcutAction) []ShortcutAction {
	t.Helper()
	data, err := json.Marshal(actions)
	if err != nil {
		t.Fatal(err)
	}
	var cloned []ShortcutAction
	if err := json.Unmarshal(data, &cloned); err != nil {
		t.Fatal(err)
	}
	return cloned
}

func TestP15VCardTokenStringProduction(t *testing.T) {
	action, ok := schema.DefaultRegistry().LookupAction("makeVCard")
	if !ok || len(action.Parameters) < 2 {
		t.Fatal("canonical makeVCard missing")
	}
	for _, p := range action.Parameters[:2] {
		if !p.Literal || !strings.Contains("|"+strings.Join(p.AcceptedForms, "|")+"|", "|tokenString|") {
			t.Fatalf("fixture must exercise canonical literal template capability: %#v", p)
		}
	}
	legacy := "const p15Title = text(\"P15_TITLE\")\nconst p15Subtitle = text(\"P15_SUBTITLE\")\nmakeVCard(\"\U0001f30d {p15Title}\", \"e\u0301 {p15Subtitle}\")\n"
	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(legacy)
	if err != nil || len(diags) != 0 {
		t.Fatalf("actual migration: %v %v", err, diags)
	}
	v2 := "let p15Title = text(\"P15_TITLE\")\nlet p15Subtitle = text(\"P15_SUBTITLE\")\n"
	for _, tc := range []struct {
		name, source string
		legacy       bool
	}{
		{"legacy_control", legacy, true},
		{"migrated_legacy", migrated, false},
		{"authored_named", v2 + "makeVCard(title: f\"\U0001f30d {p15Title}\", subtitle: f\"e\u0301 {p15Subtitle}\")", false},
		{"authored_named_reordered", v2 + "makeVCard(subtitle: f\"e\u0301 {p15Subtitle}\", title: f\"\U0001f30d {p15Title}\")", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := compileDebugFixture(t, tc.source, tc.legacy)
			if err := p15VCardTokenStringOracle(actions); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"drop_attachment", "wrong_utf16_offset", "retarget_output", "stringify_token", "wrong_producer"} {
				broken := p15CloneVCardActions(t, actions)
				params := broken[2].WFWorkflowActionParameters
				token := debugParamMap(params["WFTextActionText"])
				value := debugParamMap(token["Value"])
				attachments := debugParamMap(value["attachmentsByRange"])
				key := fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(p15VCardPrefix+"\U0001f30d "))))
				switch mutation {
				case "drop_attachment":
					delete(attachments, key)
				case "wrong_utf16_offset":
					attachments["{0, 1}"] = attachments[key]
					delete(attachments, key)
				case "retarget_output":
					debugParamMap(attachments[key])["OutputUUID"] = broken[1].WFWorkflowActionParameters["UUID"]
				case "stringify_token":
					params["WFTextActionText"] = value["string"]
				case "wrong_producer":
					broken[0].WFWorkflowActionParameters["WFTextActionText"] = "NOT_THE_TITLE"
				}
				if p15VCardTokenStringOracle(broken) == nil {
					t.Fatalf("oracle accepted causal mutation %q", mutation)
				}
			}
		})
	}
}

func TestP15VCardTokenStringCapabilityMetadata(t *testing.T) {
	card, ok := schema.DefaultRegistry().LookupAction("makeVCard")
	if !ok {
		t.Fatal("canonical VCard action missing")
	}
	text, ok := schema.DefaultRegistry().LookupAction("text")
	if !ok {
		t.Fatal("canonical Text producer missing")
	}
	source := "let value = text(\"P15\")\nmakeVCard(f\"{value}\", subtitle: \"B\")"
	argument := "f\"{value}\""
	for _, enabled := range []bool{true, false} {
		copyCard := *card
		copyCard.Parameters = append([]schema.ParameterSchema(nil), card.Parameters...)
		if !enabled {
			copyCard.Parameters[0].AcceptedForms = []string{"literal"}
		}
		registry := schema.NewRegistry([]*schema.ActionSchema{&copyCard, text}, nil)
		program, errors := p15ParseProgram(source)
		if len(errors) != 0 {
			t.Fatal(errors)
		}
		analyzer := analysis.NewAnalyzer(registry)
		analyzer.Analyze(program)
		diagnostics := analyzer.Diagnostics()
		if !enabled {
			if err := p15ExactLiteralDiagnostic(diagnostics, source, argument, analysis.CodeLiteralRequired); err != nil {
				t.Fatalf("removing the actual capability did not close the template boundary: %v", err)
			}
			continue
		}
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatalf("canonical capability did not permit a template: %v", diagnostic)
			}
		}
	}
}

func TestP15VCardLiteralBoundary(t *testing.T) {
	for _, tc := range []struct{ name, source, argument string }{
		{"title_binding", "let value = text(\"A\")\nmakeVCard(value, subtitle: \"B\")", "value"},
		{"subtitle_binding", "let value = text(\"B\")\nmakeVCard(\"A\", subtitle: value)", "value"},
		{"title_action", "makeVCard(text(\"A\"), subtitle: \"B\")", "text(\"A\")"},
		{"subtitle_action", "makeVCard(\"A\", subtitle: text(\"B\"))", "text(\"B\")"},
		{"runtime_concatenation", "let value = text(\"A\")\nmakeVCard(\"prefix\" + value, subtitle: \"B\")", "\"prefix\" + value"},
		{"confirmed_literal_enum", "let value = text(\"Ocean\")\nsetBackgroundSound(f\"{value}\")", "f\"{value}\""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := p15ExactLiteralDiagnostic(p15TypeDiagnostics(t, tc.source), tc.source, tc.argument, analysis.CodeLiteralRequired); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-vcard-negative.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeLiteralRequired) {
				t.Fatalf("literal violation reached emission: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}

func TestP15MenusMigrationCLIParity(t *testing.T) {
	fixture := filepath.Join("tests", "menus.cherri")
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		after, err := os.ReadFile(fixture)
		if err != nil || !bytes.Equal(original, after) {
			t.Errorf("canonical menus fixture changed: %v", err)
		}
	}()
	migrated, diags, err := migrate.MigrateSourceWithDiagnostics(string(original))
	if err != nil || len(diags) != 0 {
		t.Fatalf("actual migration failed: %v %v", err, diags)
	}
	if !strings.Contains(migrated, "f\"Title {i}\"") || !strings.Contains(migrated, "f\"Subtitle {i}\"") {
		t.Fatalf("migration lost explicit runtime templates:\n%s", migrated)
	}
	tmp := t.TempDir()
	p15StageMenusAsset(t, tmp)
	v2Source := filepath.Join(tmp, "menus_v2.cherri")
	if err := os.WriteFile(v2Source, []byte(migrated), 0600); err != nil {
		t.Fatal(err)
	}
	legacyOut := filepath.Join(tmp, "menus_legacy.shortcut")
	v2Out := filepath.Join(tmp, "menus_v2.shortcut")
	bin := roundTripBinary(t)
	for _, arguments := range [][]string{
		{fixture, "--legacy", "--skip-sign", "-o", legacyOut, "--load-all-actions"},
		{"compile", v2Source, "--skip-sign", "-o", v2Out},
	} {
		stdout, stderr, err := p15RunCLI(t, bin, "", nil, arguments...)
		if err != nil {
			t.Fatalf("actual CLI failed: %v\nstdout=%s\nstderr=%s", err, stdout, stderr)
		}
	}
	result, err := shortcutcompare.CompareFiles(legacyOut, v2Out)
	if err != nil {
		t.Fatal(err)
	}
	mode := "canonical"
	if !result.Equal {
		result, err = shortcutcompare.CompareFilesNormalized(legacyOut, v2Out)
		mode = "existing_normalized"
	}
	if err != nil || !result.Equal {
		t.Fatalf("original menus differential gate: err=%v differences=%+v", err, result)
	}
	t.Logf("unchanged canonical menus fixture compiled through both actual CLI paths; parity=%s", mode)
}
