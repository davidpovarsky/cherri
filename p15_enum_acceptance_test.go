package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

const p15C04Declarations = "enum Mode { Blue = \"native-blue\", Empty = \"\", Quoted = \"\\\"kept\\\"\", \u03a9 = \"wire-\u05d0\" }\n" +
	"enum Other { Only = \"other-wire\" }\n" +
	"action c04Pick(mode: Mode = .Blue, note: Text = \"\", other: Other = .Only) -> Text {\n" +
	"identifier: \"com.example.p15.enum\"\n" +
	"bindings: { mode: {key: \"Mode\", codec: \"text\"}, note: {key: \"Note\", codec: \"text\"}, other: {key: \"Other\", codec: \"text\"} }\n}\n"

const p15C04NativeAlias = "enum backgroundSound { Coast = \"Ocean\" }\n"

type p15C04WireCase struct{ name, source, id, key, want string }

var p15C04WireCases = []p15C04WireCase{
	{"native_context_primary", "setBackgroundSound(.Ocean)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Ocean"},
	{"native_context_named", "setBackgroundSound(sound: .Rain)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Rain"},
	{"native_qualified", "setBackgroundSound(backgroundSound.Ocean)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Ocean"},
	{"native_qualified_named", "setBackgroundSound(sound: backgroundSound.Rain)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Rain"},
	{"custom_context", p15C04Declarations + "c04Pick(.Blue)", "com.example.p15.enum", "Mode", "native-blue"},
	{"custom_named", p15C04Declarations + "c04Pick(note: \"label\", mode: .Blue)", "com.example.p15.enum", "Mode", "native-blue"},
	{"custom_qualified", p15C04Declarations + "c04Pick(Mode.Blue)", "com.example.p15.enum", "Mode", "native-blue"},
	{"custom_empty", p15C04Declarations + "c04Pick(Mode.Empty)", "com.example.p15.enum", "Mode", ""},
	{"custom_quoted", p15C04Declarations + "c04Pick(Mode.Quoted)", "com.example.p15.enum", "Mode", "\"kept\""},
	{"custom_unicode", p15C04Declarations + "c04Pick(Mode.\u03a9)", "com.example.p15.enum", "Mode", "wire-\u05d0"},
	{"custom_default", p15C04Declarations + "c04Pick()", "com.example.p15.enum", "Mode", "native-blue"},
	{"custom_second_domain", p15C04Declarations + "c04Pick(other: .Only)", "com.example.p15.enum", "Other", "other-wire"},
	{"qualified_expression", p15C04Declarations + "text(Mode.Blue)", "is.workflow.actions.gettext", "WFTextActionText", "native-blue"},
	{"qualified_empty_expression", p15C04Declarations + "text(Mode.Empty)", "is.workflow.actions.gettext", "WFTextActionText", ""},
	{"lowercase_declared", "enum tone { Low = \"low-wire\" }\naction c04Tone(value:tone) {identifier:\"com.example.p15.tone\"}\nc04Tone(tone.Low)", "com.example.p15.tone", "value", "low-wire"},
	{"native_source_alias_context", p15C04NativeAlias + "setBackgroundSound(.Coast)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Ocean"},
	{"native_source_alias_qualified", p15C04NativeAlias + "setBackgroundSound(backgroundSound.Coast)", "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "backgroundSound", "Ocean"},
}

func p15C04Wire(actions []ShortcutAction, tc p15C04WireCase) error {
	matched := 0
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier != tc.id {
			continue
		}
		value, ok := action.WFWorkflowActionParameters[tc.key]
		if !ok || value != tc.want {
			return fmt.Errorf("%s wire %s = %#v, want %q", tc.name, tc.key, value, tc.want)
		}
		matched++
	}
	if matched != 1 {
		return fmt.Errorf("%s matched %d native actions, want one", tc.name, matched)
	}
	return nil
}

var p15C04Invalid = []struct{ name, source, argument, code string }{
	{"native_domain_override_context", "enum backgroundSound { Coast=\"not-native\" }\nsetBackgroundSound(.Coast)", ".Coast", analysis.CodeEnumMember},
	{"native_domain_override_qualified", "enum backgroundSound { Coast=\"not-native\" }\nsetBackgroundSound(backgroundSound.Coast)", "backgroundSound.Coast", analysis.CodeEnumMember},
	{"native_domain_override_literal", "enum backgroundSound { Coast=\"not-native\" }\nsetBackgroundSound(\"not-native\")", "\"not-native\"", analysis.CodeEnumMember},
	{"bare_enum_type", "enum Mode {Blue=\"wire\"}\ntext(Mode)", "Mode", analysis.CodeEnumMember},
	{"branch_raw_domain", "enum Mode {Blue=\"wire\"}\nlet value:Mode=if number(0)==1 {\"wrong\"} else {.Blue}", "\"wrong\"", analysis.CodeEnumMember},
	{"function_member", p15C04Function + "c04Function(.Missing)", ".Missing", analysis.CodeEnumMember},
	{"function_raw_domain", p15C04Function + "c04Function(\"wrong\")", "\"wrong\"", analysis.CodeEnumMember},
	{"return_member", "enum Mode {Blue=\"wire\"}\nfunction c04Bad()->Mode{return .Missing}\nc04Bad()", ".Missing", analysis.CodeEnumMember},
	{"binding_member", "enum Mode {Blue=\"wire\"}\nlet bad:Mode=.Missing", ".Missing", analysis.CodeEnumMember},
	{"assignment_member", "enum Mode {Blue=\"wire\"}\nvar bad:Mode=.Blue\nbad=.Missing", ".Missing", analysis.CodeEnumMember},
	{"wrong_qualified_domain", p15C04Declarations + "c04Pick(other:Mode.Blue)", "Mode.Blue", analysis.CodeEnumMember},
	{"duplicate_member", "enum Duplicate {One=\"a\", One=\"b\"}\ntext(\"unused\")", "One", analysis.CodeDuplicateName},
	{"native_missing", "setBackgroundSound(.Missing)", ".Missing", analysis.CodeEnumMember},
	{"native_qualified_missing", "setBackgroundSound(backgroundSound.Missing)", "backgroundSound.Missing", analysis.CodeEnumMember},
	{"custom_missing", p15C04Declarations + "c04Pick(.Missing)", ".Missing", analysis.CodeEnumMember},
	{"qualified_missing", p15C04Declarations + "text(Mode.Missing)", "Mode.Missing", analysis.CodeEnumMember},
	{"unknown_qualified", "text(NoSuchEnum.Member)", "NoSuchEnum.Member", analysis.CodeEnumMember},
	{"unbound_context", "let orphan = .Blue", ".Blue", analysis.CodeEnumMember},
	{"wrong_custom_wire", p15C04Declarations + "c04Pick(\"not-declared\")", "\"not-declared\"", analysis.CodeEnumMember},
	{"wrong_domain_member", p15C04Declarations + "c04Pick(other: .Blue)", ".Blue", analysis.CodeEnumMember},
}

func TestP15C04EnumResolutionProduction(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	// The expected native ID/wire key above are independently anchored in the
	// canonical source declaration, not read back from the generated registry.
	raw, err := os.ReadFile("actions/a11y.cherri")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "UASetBackgroundSoundIntent' setBackgroundSound(backgroundSound ?sound: 'backgroundSound'") ||
		!strings.Contains(string(raw), "'Ocean'") {
		t.Fatal("canonical native enum oracle changed")
	}
	for _, tc := range p15C04WireCases {
		t.Run(tc.name, func(t *testing.T) {
			program, parseErrors := p15ParseProgram(tc.source)
			if len(parseErrors) != 0 {
				t.Fatal(parseErrors)
			}
			formattedBefore := syntax.Format(program)
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			for _, d := range analyzer.Diagnostics() {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
			if syntax.Format(program) != formattedBefore {
				t.Fatal("semantic enum resolution rewrote the source AST")
			}
			actions := compileDebugFixture(t, tc.source, false)
			if err := p15C04Wire(actions, tc); err != nil {
				t.Fatal(err)
			}
			// Empty and quoted wire values must survive canonical source formatting.
			formatted := syntax.Format(program)
			if err := p15C04Wire(compileDebugFixture(t, formatted, false), tc); err != nil {
				t.Fatal("format changed enum wire", err)
			}
			for i := range actions {
				if actions[i].WFWorkflowActionIdentifier == tc.id {
					old := actions[i].WFWorkflowActionParameters[tc.key]
					actions[i].WFWorkflowActionParameters[tc.key] = "WRONG_ENUM_WIRE"
					if p15C04Wire(actions, tc) == nil {
						t.Fatal("wrong-wire mutation passed")
					}
					actions[i].WFWorkflowActionParameters[tc.key] = old
				}
			}
		})
	}
	for _, tc := range p15C04Invalid {
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, tc.code); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-c04-invalid.cherri", tc.source)
			if err == nil || len(raw) != 0 {
				t.Fatal("invalid enum reached native emission")
			}
			for i := range diagnostics {
				if diagnostics[i].Severity == analysis.SeverityError {
					diagnostics[i].Code = analysis.CodeUnknownName
				}
			}
			if p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, tc.code) == nil {
				t.Fatal("wrong-diagnostic mutation passed")
			}
		})
	}
}

func p15C04CompletionPosition(text string, offset int) (int, int) {
	prefix := text[:offset]
	line := strings.Count(prefix, "\n") + 1
	tail := prefix[strings.LastIndex(prefix, "\n")+1:]
	return line, utf8.RuneCountInString(tail) + 1
}

func p15C04ApplyCompletion(text string, offset int, insertion string) string {
	start := offset
	for start > 0 {
		r, n := utf8.DecodeLastRuneInString(text[:start])
		if r != '_' && !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			break
		}
		start -= n
	}
	return text[:start] + insertion + text[offset:]
}

func TestP15C04EnumServiceAndCLI(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	svc := service.NewService(nil)
	uri := "file:///p15-c04-completion.cherri"
	cases := []struct {
		name, marked, member, domain string
		wire                         p15C04WireCase
	}{
		{"quoted_member", "enum Space {\"space name\"=\"wire-space\"}\naction c04Space(value:Space){identifier:\"com.example.p15.space\"}\nc04Space(sp|)", "space name", "Space", p15C04WireCase{"quoted_member", "", "com.example.p15.space", "value", "wire-space"}},
		{"native_source_alias", p15C04NativeAlias + "setBackgroundSound(.Co|)", "Coast", "backgroundSound", p15C04WireCases[0]},
		{"scope_ends_at_block", p15C04Declarations + "let choice=measure(\"scope\") {let Mode={\"Blue\":\"dynamic\"}; Mode.Blue}\nc04Pick(Mode.Bl|)", "Blue", "Mode", p15C04WireCases[4]},
		{"native_bare", "setBackgroundSound(O|)", "Ocean", "backgroundSound", p15C04WireCases[0]},
		{"native_empty", "setBackgroundSound(|)", "Ocean", "backgroundSound", p15C04WireCases[0]},
		{"native_dot_empty", "setBackgroundSound(.|)", "Ocean", "backgroundSound", p15C04WireCases[0]},
		{"custom_nested_call", p15C04Declarations + "c04Pick(note: text(\"comma,(paren)\"), mode: .Bl|)", "Blue", "Mode", p15C04WireCases[4]},
		{"custom_comment", p15C04Declarations + "c04Pick(note: \"ok\", /* fake(, other:) */ mode: .Bl|)", "Blue", "Mode", p15C04WireCases[4]},
		{"native_dot", "setBackgroundSound(.O|)", "Ocean", "backgroundSound", p15C04WireCases[0]},
		{"native_qualified", "setBackgroundSound(backgroundSound.O|)", "Ocean", "backgroundSound", p15C04WireCases[0]},
		{"native_named_multiline", "setBackgroundSound(\n sound: .R|)", "Rain", "backgroundSound", p15C04WireCases[1]},
		{"custom_dot", p15C04Declarations + "c04Pick(.Bl|)", "Blue", "Mode", p15C04WireCases[4]},
		{"custom_qualified", p15C04Declarations + "c04Pick(Mode.Em|)", "Empty", "Mode", p15C04WireCases[7]},
		{"custom_second_domain", p15C04Declarations + "c04Pick(note: \"comma,(paren)\", other: .On|)", "Only", "Other", p15C04WireCases[11]},
		{"custom_unicode", p15C04Declarations + "c04Pick(note: \"\U0001f9ea\", mode: .\u03a9|)", "\u03a9", "Mode", p15C04WireCases[9]},
	}
	completed := []string{}
	for i, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offset := strings.Index(tc.marked, "|")
			if offset < 0 {
				t.Fatal("cursor marker absent")
			}
			text := strings.Replace(tc.marked, "|", "", 1)
			svc.OpenDocument(uri, i+1, text)
			line, col := p15C04CompletionPosition(text, offset)
			items := svc.Complete(uri, line, col)
			var selected *service.CompletionItem
			for index := range items {
				item := &items[index]
				if item.Kind != service.CompletionKindEnum {
					continue
				}
				if item.Detail != "enum "+tc.domain {
					t.Fatalf("unrelated enum offered: %+v", *item)
				}
				// Editor prefix filtering is on Label, without a leading dot.
				if item.Label == tc.member {
					selected = item
				}
			}
			if selected == nil {
				t.Fatalf("contextual member %q absent", tc.member)
			}
			result := p15C04ApplyCompletion(text, offset, selected.InsertText)
			if err := p15C04Wire(compileDebugFixture(t, result, false), tc.wire); err != nil {
				t.Fatal("completion did not compile to the promised wire", err)
			}
			completed = append(completed, result)
			broken := p15C04ApplyCompletion(text, offset, ".NOT_A_MEMBER")
			if raw, err := CompileSourceToPlist("p15-c04-broken-completion.cherri", broken); err == nil || len(raw) != 0 {
				t.Fatal("invalid completion mutation reached native output")
			}
		})
	}
	for _, marked := range []string{p15C04Declarations + "c04Pick(note: |)", "let object={\"Blue\":\"dynamic\"}\ntext(object.|)", p15C04Declarations + "function c04Shadow(Mode:Map<Text,Text>)->Text{return Mode.|}", p15C04Declarations + "let choice=measure(\"scope\") { let Mode={\"Blue\":\"dynamic\"}; Mode.B| }", p15C04Declarations + "c04Pick(mode: \"Bl|\")"} {
		offset := strings.Index(marked, "|")
		text := strings.Replace(marked, "|", "", 1)
		svc.OpenDocument(uri, 30, text)
		line, col := p15C04CompletionPosition(text, offset)
		for _, item := range svc.Complete(uri, line, col) {
			if item.Kind == service.CompletionKindEnum {
				t.Fatal("unrelated enum leaked into non-enum context", item)
			}
		}
	}
	bin := p15AcceptanceCLI(t)
	dir := t.TempDir()
	for index, src := range completed {
		path := filepath.Join(dir, fmt.Sprintf("completed-%d.cherri", index))
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		out, stderr, err := p15RunCLI(t, bin, dir, nil, "check", path, "--json")
		if err != nil {
			t.Fatalf("completed CLI failed: %v %s %s", err, out, stderr)
		}
		var result protocol.AnalyzeResponse
		if err := json.Unmarshal(out, &result); err != nil || !result.Valid {
			t.Fatalf("invalid CLI completion response: %s", out)
		}
	}
	for index, tc := range p15C04Invalid {
		svc.ChangeDocument(uri, 100+index*2, tc.source)
		diagnostics, _ := svc.Analyze(uri)
		if err := p15ExactLiteralDiagnostic(diagnostics, tc.source, tc.argument, tc.code); err != nil {
			t.Fatal("service", err)
		}
		svc.ChangeDocument(uri, 101+index*2, "number(0)")
		diagnostics, _ = svc.Analyze(uri)
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal("corrected service retained enum error", diagnostic)
			}
		}
		path := filepath.Join(dir, tc.name+".cherri")
		if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		out, _, err := p15RunCLI(t, bin, dir, nil, "check", path, "--json")
		if err == nil {
			t.Fatal("CLI accepted invalid enum", tc.name)
		}
		var result protocol.AnalyzeResponse
		if err := json.Unmarshal(out, &result); err != nil {
			t.Fatal(err)
		}
		if result.Valid || len(result.Diagnostics) != 1 || result.Diagnostics[0].Code != tc.code ||
			result.Diagnostics[0].Severity != "ERROR" || result.Diagnostics[0].Range != p15C02ExpectedRange(tc.source, tc.argument) {
			t.Fatalf("CLI enum diagnostic mismatch: %+v", result)
		}
	}
}

const p15C04Function = "enum Mode { Blue = \"native-blue\", Empty = \"\" }\nfunction c04Function(value: Mode = .Blue) -> Mode { return value }\n"

func TestP15C04EnumContextsAndShadowing(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	for _, tc := range []struct {
		name, source string
		wires        []string
	}{
		{"binding", "enum Mode { Blue=\"native-blue\" }\nlet choice:Mode = .Blue\ntext(choice)", []string{"native-blue"}},
		{"assignment", "enum Mode { Blue=\"native-blue\", Empty=\"\" }\nvar choice:Mode = .Blue\nchoice = .Empty\ntext(choice)", []string{"native-blue", ""}},
		{"optional", "enum Mode { Blue=\"native-blue\" }\nlet choice:Mode? = .Blue\ncomment(\"optional enum\")", []string{"native-blue"}},
		{"function_primary", p15C04Function + "c04Function(.Empty)", []string{"native-blue", ""}},
		{"function_named", p15C04Function + "c04Function(value:.Empty)", []string{"native-blue", ""}},
		{"function_return", "enum Mode { Blue=\"native-blue\" }\nfunction c04Return()->Mode {return .Blue}\nc04Return()", []string{"native-blue"}},
		{"branch_result", "enum Mode { Blue=\"native-blue\", Empty=\"\" }\nlet choice:Mode = if number(0)==1 {.Blue} else {.Empty}\ntext(choice)", []string{"native-blue", ""}},
		{"measure_result", "enum Mode { Blue=\"native-blue\" }\nlet choice:Mode = measure(\"enum\") {.Blue}\ntext(choice)", []string{"native-blue"}},
		{"quoted_member", "enum Mode { \"space name\"=\"wire-space\", \"if\"=\"wire-keyword\" }\naction c04Quoted(value:Mode) {identifier:\"com.example.p15.quoted\"}\nc04Quoted(\"wire-space\")", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, errors := p15ParseProgram(tc.source)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			for _, diagnostic := range analyzer.Diagnostics() {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			got := map[string]bool{}
			for expression, wire := range analyzer.EnumValues() {
				got[wire] = true
				if actual := analyzer.ExprTypes()[expression]; actual == nil || actual.Name() != "Text" {
					t.Fatalf("resolved enum did not infer Text: %v", actual)
				}
			}
			for _, wire := range tc.wires {
				if !got[wire] {
					t.Fatalf("expected exact resolved wire %q missing", wire)
				}
			}
			compileDebugFixture(t, tc.source, false)
			formatted := syntax.Format(program)
			compileDebugFixture(t, formatted, false)
		})
	}
	// Enum qualifiers must not steal typed dictionary parameters. Both defaulted
	// and required parameters retain their original native producer and key path.
	for _, tc := range []struct{ name, defaultExpr string }{
		{"Mode", ""}, {"Mode", " = {\"Blue\":\"fallback\"}"},
		{"tone", ""}, {"backgroundSound", ""},
	} {
		t.Run("shadow_"+tc.name+tc.defaultExpr, func(t *testing.T) {
			src := "enum " + tc.name + " { Blue=\"enum-wire\" }\nfunction c04Shadow(" + tc.name + ":Map<Text,Text>" + tc.defaultExpr + ")->Text{return " + tc.name + ".Blue}\nc04Shadow({\"Blue\":\"record-wire\"})"
			program, errors := p15ParseProgram(src)
			if len(errors) != 0 {
				t.Fatal(errors)
			}
			analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
			analyzer.Analyze(program)
			for _, diagnostic := range analyzer.Diagnostics() {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			if len(analyzer.EnumValues()) != 0 {
				t.Fatal("dictionary field incorrectly became a static enum")
			}
			actions := compileDebugFixture(t, src, false)
			if err := p15C04ShadowWire(actions, tc.name); err != nil {
				t.Fatal(err)
			}
			for index := range actions {
				if actions[index].WFWorkflowActionIdentifier == "is.workflow.actions.output" {
					actions[index].WFWorkflowActionParameters["WFOutput"] = "enum-wire"
				}
			}
			if p15C04ShadowWire(actions, tc.name) == nil {
				t.Fatal("static enum replacement mutation passed")
			}
		})
	}
}

func p15C04FunctionWire(t *testing.T, source, want string) {
	t.Helper()
	actions := p15CompiledActions(t, source)
	found := 0
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		parameters := action["WFWorkflowActionParameters"].(map[string]any)
		fields := p15LiteralDictionaryFields(t, parameters["WFItems"])
		if fields["_cherri_magic"] == nil {
			continue
		}
		found++
		presence := fields["has_arg_0"]["WFValue"].(map[string]any)["Value"].(map[string]any)["string"]
		if presence != "1" {
			t.Fatalf("explicit enum lost presence: %#v", presence)
		}
		value := fields["arg_0"]
		if value == nil || p15Int(value["WFItemType"]) != 0 {
			t.Fatalf("enum argument lost Text item: %#v", value)
		}
		token := value["WFValue"].(map[string]any)
		actual := token["Value"].(map[string]any)["string"]
		if token["WFSerializationType"] != "WFTextTokenString" || actual != want {
			t.Fatalf("function enum wire=%#v, want %q", token, want)
		}
	}
	if found != 1 {
		t.Fatalf("expected one function payload, got %d", found)
	}
}

func TestP15C04EnumFunctionService(t *testing.T) {
	t.Cleanup(schema.ResetDefaultRegistry)
	svc := service.NewService(nil)
	uri := "file:///p15-c04-function.cherri"
	for index, marked := range []string{
		p15C04Function + "c04Function(.Em|)",
		p15C04Function + "c04Function(value: .Em|)",
		p15C04Function + "c04Function(Mode.Em|)",
		strings.Replace(p15C04Function, "value: Mode", "_value: Mode", 1) + "c04Function(value: .Em|)",
	} {
		// Keep the underscore-parameter body lexically consistent.
		if strings.Contains(marked, "_value: Mode") {
			marked = strings.Replace(marked, "return value", "return _value", 1)
		}
		offset := strings.Index(marked, "|")
		source := strings.Replace(marked, "|", "", 1)
		svc.OpenDocument(uri, index+1, source)
		line, col := p15C04CompletionPosition(source, offset)
		found := false
		for _, item := range svc.Complete(uri, line, col) {
			if item.Kind != service.CompletionKindEnum {
				continue
			}
			if item.Detail != "enum Mode" {
				t.Fatal("wrong function parameter domain", item)
			}
			if item.Label == "Empty" {
				result := p15C04ApplyCompletion(source, offset, item.InsertText)
				p15C04FunctionWire(t, result, "")
				found = true
			}
		}
		if !found {
			t.Fatal("function enum completion missing")
		}
	}
	// Explicit value and default use different presence tags; enum resolution
	// must not silently convert an explicit empty argument to the default.
	p15C04FunctionWire(t, p15C04Function+"c04Function(.Empty)", "")
	p15C04FunctionWire(t, p15C04Function+"c04Function(.Blue)", "native-blue")
}

func p15C04ShadowWire(actions []ShortcutAction, name string) error {
	raw, err := json.Marshal(actions)
	if err != nil {
		return err
	}
	var native []map[string]any
	if err = json.Unmarshal(raw, &native); err != nil {
		return err
	}
	producer := ""
	for _, action := range native {
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		if parameters["CustomOutputName"] == name {
			if uuid, ok := parameters["UUID"].(string); ok {
				producer = uuid
			}
		}
	}
	if producer == "" {
		return fmt.Errorf("missing typed parameter producer for %s", name)
	}
	var reference func(any) bool
	reference = func(value any) bool {
		switch value := value.(type) {
		case map[string]any:
			if value["Type"] == "ActionOutput" && value["OutputUUID"] == producer {
				aggs, _ := value["Aggrandizements"].([]any)
				if len(aggs) == 1 {
					agg, _ := aggs[0].(map[string]any)
					if agg["Type"] == "WFDictionaryValueVariableAggrandizement" && agg["DictionaryKey"] == "Blue" {
						return true
					}
				}
			}
			for _, child := range value {
				if reference(child) {
					return true
				}
			}
		case []any:
			for _, child := range value {
				if reference(child) {
					return true
				}
			}
		}
		return false
	}
	for _, action := range native {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.output" {
			parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
			if reference(parameters["WFOutput"]) {
				return nil
			}
		}
	}
	return fmt.Errorf("return did not reference %s's original Dictionary key", name)
}
