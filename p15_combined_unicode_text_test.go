package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

// These joined scenarios run the actual parser, semantic analyzer, compiler and
// filesystem import pipeline. Their independent expectations cover local text
// bytes, native references and complete action graphs; they do not execute or
// replace Apple's text renderer.
const p15UTRP19Template = "שָׁלוֹם👩‍💻é|42false\ufffc\ufffc|é\ufffc|🌍\ufffc\n"
const p15UTBRG08Template = "שָׁלוֹם👩‍💻é|{literal}|\ufffc\ufffc|é\ufffc|🌍\ufffc\ufffc\n"
const p15UTPlain = "{seed}|JSON:{\"n\":42}|\\path"
const p15UTRaw = "\\d+{seed}\\n"

func p15UTSource(t *testing.T, id string) string {
	t.Helper()
	var name = map[string]string{"RP19": "v2-rp19-literal-unicode.cherri", "BRG08": "v2-brg08-inline-codec.cherri"}[id]
	if name == "" {
		t.Fatal("unknown joined Unicode scenario", id)
	}
	var data, err = os.ReadFile(filepath.Join("tests", "language-v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func p15UTLiteral(kind, value string) any { return []any{"literal", kind, value} }
func p15UTIdent(name string) any          { return []any{"identifier", name} }
func p15UTCall(name string, primary any, named ...any) any {
	if named == nil {
		named = []any{}
	}
	return []any{"call", name, primary, named}
}
func p15UTFString(parts ...any) any { return []any{"f-string", parts} }
func p15UTName(expr syntax.Expression) string {
	switch expr := expr.(type) {
	case *syntax.IdentExpr:
		return expr.Name
	case *syntax.MemberExpr:
		return p15UTName(expr.Target) + "." + expr.Property
	default:
		return fmt.Sprintf("unsupported:%T", expr)
	}
}
func p15UTShape(expr syntax.Expression) any {
	switch expr := expr.(type) {
	case nil:
		return nil
	case *syntax.LiteralExpr:
		var kind = map[syntax.TokenType]string{syntax.TokenString: "Text", syntax.TokenRawString: "RawText", syntax.TokenNumber: "Number", syntax.TokenFalse: "Bool", syntax.TokenTrue: "Bool"}[expr.Type]
		return p15UTLiteral(kind, expr.Value)
	case *syntax.IdentExpr:
		return p15UTIdent(expr.Name)
	case *syntax.CallExpr:
		if expr.Block != nil {
			return []any{"unexpected-block"}
		}
		var named = []any{}
		for _, arg := range expr.NamedArgs {
			named = append(named, []any{arg.Label, p15UTShape(arg.Value)})
		}
		return p15UTCall(p15UTName(expr.Callee), p15UTShape(expr.PrimaryArg), named...)
	case *syntax.FStringExpr:
		var parts = []any{}
		for _, part := range expr.Parts {
			if part.IsExpr {
				parts = append(parts, p15UTShape(part.Expr))
			} else {
				parts = append(parts, []any{"text", part.Text})
			}
		}
		return p15UTFString(parts...)
	case *syntax.ListExpr:
		var values = []any{}
		for _, value := range expr.Elements {
			values = append(values, p15UTShape(value))
		}
		return []any{"list", values}
	case *syntax.MapExpr:
		var values = []any{}
		for _, value := range expr.Entries {
			values = append(values, []any{p15UTShape(value.Key), p15UTShape(value.Value)})
		}
		return []any{"map", values}
	default:
		return []any{"unsupported", fmt.Sprintf("%T", expr)}
	}
}
func p15UTMapShape(pairs ...string) any {
	var entries = []any{}
	for i := 0; i < len(pairs); i += 2 {
		entries = append(entries, []any{p15UTLiteral("Text", pairs[i]), p15UTLiteral("Text", pairs[i+1])})
	}
	return []any{"map", entries}
}
func p15UTChain() []any {
	return []any{
		map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem"},
		map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "payload"},
		map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"},
	}
}

type p15UTBinding struct {
	name  string
	kind  types.Type
	shape any
}

func p15UTBindings(id string) []p15UTBinding {
	if id == "RP19" {
		return []p15UTBinding{
			{"first", types.Text, p15UTCall("text", p15UTLiteral("Text", "α"))},
			{"second", types.Text, p15UTCall("text", p15UTLiteral("Text", "ב"))},
			{"result", types.Text, p15UTFString(
				[]any{"text", "שָׁלוֹם👩‍💻é|"}, p15UTLiteral("Number", "42"), p15UTLiteral("Bool", "false"),
				p15UTIdent("first"), p15UTIdent("second"), []any{"text", "|é"}, p15UTIdent("first"),
				[]any{"text", "|🌍"}, p15UTCall("text", p15UTLiteral("Text", "Ω")), []any{"text", "\n"})},
		}
	}
	var chain = []any{"list", []any{
		p15UTMapShape("Type", "WFCoercionVariableAggrandizement", "CoercionItemClass", "WFDictionaryContentItem"),
		p15UTMapShape("Type", "WFDictionaryValueVariableAggrandizement", "DictionaryKey", "payload"),
		p15UTMapShape("Type", "WFCoercionVariableAggrandizement", "CoercionItemClass", "WFStringContentItem"),
	}}
	return []p15UTBinding{
		{"seed", types.NewMap(types.Text, types.Text), p15UTMapShape("payload", "Δ")},
		{"transformed", types.AnyContent, p15UTCall("native.ref", p15UTIdent("seed"), []any{"aggrandizements", chain})},
		{"direct", types.AnyContent, p15UTCall("getVariable", p15UTIdent("transformed"))},
		{"plain", types.Text, p15UTLiteral("Text", p15UTPlain)},
		{"raw", types.Text, p15UTLiteral("RawText", p15UTRaw)},
		{"result", types.Text, p15UTFString(
			[]any{"text", "שָׁלוֹם👩‍💻é|{literal}|"}, p15UTIdent("transformed"), p15UTIdent("transformed"),
			[]any{"text", "|é"}, p15UTIdent("direct"), []any{"text", "|🌍"}, p15UTIdent("plain"), p15UTIdent("raw"), []any{"text", "\n"})},
	}
}
func p15UTSemanticError(source, id string) error {
	var program, parseErrors = p15ParseProgram(source)
	if len(parseErrors) != 0 {
		return fmt.Errorf("joined source parse: %v", parseErrors)
	}
	var analyzer = analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, diagnostic := range analyzer.Diagnostics() {
		if diagnostic.Severity == analysis.SeverityError {
			return fmt.Errorf("joined source analysis: %v", diagnostic)
		}
	}
	var expected = p15UTBindings(id)
	if len(program.Statements) != len(expected)+1 || len(program.Declarations) != 0 {
		return fmt.Errorf("joined source statement inventory")
	}
	for i, want := range expected {
		var binding, ok = program.Statements[i].(*syntax.BindingStmt)
		if !ok || binding.Mutable || binding.Name != want.name || binding.TypeExpr != nil ||
			!reflect.DeepEqual(p15UTShape(binding.Value), want.shape) {
			return fmt.Errorf("joined source binding shape %s", want.name)
		}
		var actual = analyzer.ExprTypes()[binding.Value]
		if actual == nil || !actual.Equals(want.kind) {
			return fmt.Errorf("joined source binding type %s: %v", want.name, actual)
		}
		if id == "RP19" && want.name == "result" {
			var f = binding.Value.(*syntax.FStringExpr)
			for index, kind := range map[int]types.Type{1: types.Number, 2: types.Bool, 3: types.Text, 4: types.Text, 6: types.Text, 8: types.Text} {
				var got = analyzer.ExprTypes()[f.Parts[index].Expr]
				if got == nil || !got.Equals(kind) {
					return fmt.Errorf("joined interpolation semantic type at %d: %v", index, got)
				}
			}
		}
	}
	var returned, ok = program.Statements[len(expected)].(*syntax.ReturnStmt)
	if !ok || !reflect.DeepEqual(p15UTShape(returned.Value), p15UTIdent("result")) {
		return fmt.Errorf("joined source returned observation")
	}
	return nil
}
func p15UTUnit(t *testing.T, id string) {
	t.Helper()
	var original = p15UTSource(t, id)
	t.Run("joined_semantics", func(t *testing.T) {
		if err := p15UTSemanticError(original, id); err != nil {
			t.Fatal(err)
		}
	})
	var controls = []struct{ name, before, after string }{
		{"combining_bytes", "é", "é"},
		{"zwj_bytes", "👩‍💻", "👩💻"},
		{"returned_observation", "return result", "return first"},
	}
	if id == "RP19" {
		controls = append(controls,
			struct{ name, before, after string }{"number_as_text_same_format", "{42}", "{\"42\"}"},
			struct{ name, before, after string }{"false_as_number", "{false}", "{0}"},
			struct{ name, before, after string }{"adjacent_owner_order", "{first}{second}", "{second}{first}"},
			struct{ name, before, after string }{"inline_action_value", "text(\"Ω\")", "text(\"ω\")"},
			struct{ name, before, after string }{"producer_type", "text(\"ב\")", "number(2)"})
	} else {
		controls[2].after = "return seed"
		controls = append(controls,
			struct{ name, before, after string }{"transform_key", "\"DictionaryKey\":\"payload\"", "\"DictionaryKey\":\"other\""},
			struct{ name, before, after string }{"transform_kind", "WFDictionaryContentItem", "WFStringContentItem"},
			struct{ name, before, after string }{"direct_owner", "getVariable(transformed)", "getVariable(seed)"},
			struct{ name, before, after string }{"inline_owner", "{transformed}{transformed}", "{seed}{transformed}"},
			struct{ name, before, after string }{"literal_braces", "{seed}|JSON", "seed|JSON"},
			struct{ name, before, after string }{"raw_string_boundary", "let raw = r\"", "let raw = \""})
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			var changed = strings.Replace(original, control.before, control.after, 1)
			if changed == original {
				t.Fatal("source control did not change")
			}
			// These are valid alternative programs where applicable, not rejected
			// language features. They must not count as the contracted scenario.
			if err := p15UTSemanticError(changed, id); err == nil {
				t.Fatal("accepted changed joined scenario")
			}
		})
	}
}
func TestP15RP19LiteralUnicodeUnit(t *testing.T)        { p15UTUnit(t, "RP19") }
func TestBackendRecovery08InlineCodecUnit(t *testing.T) { p15UTUnit(t, "BRG08") }

func p15UTCopy(value any) any {
	switch value := value.(type) {
	case map[string]any:
		var out = make(map[string]any, len(value))
		for key, child := range value {
			out[key] = p15UTCopy(child)
		}
		return out
	case []any:
		var out = make([]any, len(value))
		for i, child := range value {
			out[i] = p15UTCopy(child)
		}
		return out
	default:
		return value // Preserve native uint64, float64 and bool without JSON normalization.
	}
}
func p15UTParams(action map[string]any) map[string]any {
	var value, _ = action["WFWorkflowActionParameters"].(map[string]any)
	return value
}
func p15UTActions(root map[string]any) ([]map[string]any, error) {
	var values, ok = root["WFWorkflowActions"].([]any)
	if !ok {
		return nil, fmt.Errorf("joined native action list absent")
	}
	var actions = make([]map[string]any, len(values))
	for i, value := range values {
		var action, okay = value.(map[string]any)
		if !okay {
			return nil, fmt.Errorf("joined native action %d is not a dictionary", i)
		}
		actions[i] = action
	}
	return actions, nil
}
func p15UTUUID(value string) bool {
	if len(value) != 36 || strings.EqualFold(value, "00000000-0000-0000-0000-000000000000") {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}
func p15UTReferences(actions []map[string]any) error {
	var positions = map[string]int{}
	for i, action := range actions {
		if value, exists := p15UTParams(action)["UUID"]; exists {
			var uuid, okay = value.(string)
			if !okay || !p15UTUUID(uuid) {
				return fmt.Errorf("joined native producer UUID shape")
			}
			var key = strings.ToLower(uuid)
			if _, exists := positions[key]; exists {
				return fmt.Errorf("joined native duplicate producer UUID")
			}
			positions[key] = i
		}
	}
	var walk func(any, int) error
	walk = func(value any, consumer int) error {
		switch value := value.(type) {
		case map[string]any:
			if value["Type"] == "ActionOutput" {
				var uuid, okay = value["OutputUUID"].(string)
				var producer, found = positions[strings.ToLower(uuid)]
				if !okay || !p15UTUUID(uuid) || !found || producer >= consumer {
					return fmt.Errorf("joined native missing or late producer")
				}
			}
			for _, child := range value {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range value {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for i, action := range actions {
		if err := walk(p15UTParams(action), i); err != nil {
			return err
		}
	}
	return nil
}
func p15UTToken(text string, offsets []int, refs []map[string]any) map[string]any {
	var attachments = map[string]any{}
	for i, offset := range offsets {
		attachments[fmt.Sprintf("{%d, 1}", offset)] = refs[i]
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": text, "attachmentsByRange": attachments}}
}
func p15UTAttachment(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
}
func p15UTTokenError(actual any, expected map[string]any) error {
	var token, okay = actual.(map[string]any)
	if !okay || len(token) != 2 || token["WFSerializationType"] != "WFTextTokenString" {
		return fmt.Errorf("joined token wrapper")
	}
	var body, ok = token["Value"].(map[string]any)
	if !ok || len(body) != 2 {
		return fmt.Errorf("joined token body")
	}
	var want = expected["Value"].(map[string]any)
	if body["string"] != want["string"] {
		return fmt.Errorf("joined token exact text bytes")
	}
	var refs, valid = body["attachmentsByRange"].(map[string]any)
	var wants = want["attachmentsByRange"].(map[string]any)
	if !valid || len(refs) != len(wants) {
		return fmt.Errorf("joined token UTF-16 range inventory")
	}
	for key, reference := range wants {
		var actualRef, exists = refs[key]
		if !exists {
			return fmt.Errorf("joined token UTF-16 range %s", key)
		}
		if !reflect.DeepEqual(actualRef, reference) {
			return fmt.Errorf("joined token attachment %s", key)
		}
	}
	if err := p15S01CompletePlaceholderCoverage(actual); err != nil {
		return fmt.Errorf("joined token placeholder: %w", err)
	}
	return nil
}

type p15UTCursor struct {
	actions []map[string]any
	index   int
	err     error
}

func (c *p15UTCursor) take(kind, name string, fields map[string]any, output bool) map[string]any {
	if c.err != nil {
		return nil
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("joined missing native %s", kind)
		return nil
	}
	var action = c.actions[c.index]
	var parameters = p15UTParams(action)
	var reference map[string]any
	if output {
		var uuid, okay = parameters["UUID"].(string)
		if !okay || !p15UTUUID(uuid) {
			c.err = fmt.Errorf("joined native %s output UUID", kind)
			return nil
		}
		fields["UUID"] = uuid
		reference = map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": name}
	}
	if name != "" {
		fields["CustomOutputName"] = name
	}
	for _, key := range []string{"WFTextActionText", "WFOutput"} {
		if expected, okay := fields[key].(map[string]any); okay && expected["WFSerializationType"] == "WFTextTokenString" {
			if err := p15UTTokenError(parameters[key], expected); err != nil {
				c.err = err
				return nil
			}
		}
	}
	if len(action) != 2 || action["WFWorkflowActionIdentifier"] != "is.workflow.actions."+kind || !reflect.DeepEqual(parameters, fields) {
		c.err = fmt.Errorf("joined native %s fields at %d: got %#v want %#v", kind, c.index, parameters, fields)
		return nil
	}
	c.index++
	return reference
}

type p15UTView struct {
	text, output int
	offsets      []int
	template     string
}

func p15UTGraph(root map[string]any, id string) (p15UTView, error) {
	var view p15UTView
	var actions, err = p15UTActions(root)
	if err != nil {
		return view, err
	}
	if err = p15UTReferences(actions); err != nil {
		return view, err
	}
	var c = p15UTCursor{actions: actions}
	var refs []map[string]any
	if id == "RP19" {
		var first = c.take("gettext", "first", map[string]any{"WFTextActionText": "α"}, true)
		var second = c.take("gettext", "second", map[string]any{"WFTextActionText": "ב"}, true)
		var inline = c.take("gettext", "Text", map[string]any{"WFTextActionText": "Ω"}, true)
		refs = []map[string]any{first, second, first, inline}
		view.template, view.offsets = p15UTRP19Template, []int{22, 23, 26, 30}
	} else if id == "BRG08" {
		var _, dictionary = p15ContainerFieldValue(map[string]any{"payload": "Δ"})
		var seed = c.take("dictionary", "seed", map[string]any{"WFItems": dictionary}, true)
		var transformed = map[string]any{"Type": "ActionOutput", "OutputName": "transformed", "Aggrandizements": p15UTChain()}
		if seed != nil {
			transformed["OutputUUID"] = seed["OutputUUID"]
		}
		var direct = c.take("getvariable", "direct", map[string]any{"WFVariable": p15UTAttachment(transformed)}, true)
		var plain = c.take("gettext", "plain", map[string]any{"WFTextActionText": p15UTPlain}, true)
		var raw = c.take("gettext", "raw", map[string]any{"WFTextActionText": p15UTRaw}, true)
		refs = []map[string]any{transformed, transformed, direct, plain, raw}
		view.template, view.offsets = p15UTBRG08Template, []int{25, 26, 29, 33, 34}
	} else {
		return view, fmt.Errorf("unknown joined text scenario")
	}
	view.text = c.index
	var result = c.take("gettext", "result", map[string]any{"WFTextActionText": p15UTToken(view.template, view.offsets, refs)}, true)
	view.output = c.index
	c.take("output", "", map[string]any{"WFOutput": p15UTToken("\ufffc", []int{0}, []map[string]any{result})}, false)
	if c.err != nil {
		return view, c.err
	}
	if c.index != len(actions) {
		return view, fmt.Errorf("joined unexpected native actions after output")
	}
	return view, nil
}
func p15UTRename(value any, before, after string) {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if (key == "UUID" || key == "OutputUUID") && child == before {
				value[key] = after
			} else {
				p15UTRename(child, before, after)
			}
		}
	case []any:
		for _, child := range value {
			p15UTRename(child, before, after)
		}
	}
}
func p15UTNativeLog(t *testing.T, id, stage string, root map[string]any) {
	t.Helper()
	var raw, err = plist.Marshal(root, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var receipt, marshalErr = json.Marshal(map[string]any{
		"requirement": id, "stage": stage, "native_bytes": len(raw), "native_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)),
		"native_xml_base64":      base64.StdEncoding.EncodeToString(raw),
		"apple_runtime_observed": false, "rendered_text_observed": false,
	})
	if marshalErr != nil {
		t.Fatal(marshalErr)
	}
	t.Log("P15_JOINED_UNICODE_SPECIMEN:" + string(receipt))
}
func p15UTNative(t *testing.T, id string) {
	t.Helper()
	var root = p15MetadataNative(t, strings.ToLower(id)+"-joined", p15UTSource(t, id))
	var view, err = p15UTGraph(root, id)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("joined_graph", func(t *testing.T) { p15UTNativeLog(t, id, "actual-compiled", root) })
	var controls = []string{
		"rune_offsets", "byte_offsets", "shift_one", "range_length_two", "missing_attachment", "extra_attachment",
		"combining_bytes", "hebrew_mark", "zwj_bytes", "unbound_placeholder", "swapped_producer",
		"late_producer", "duplicate_uuid", "casefold_uuid_collision", "malformed_uuid", "zero_uuid",
		"hidden_output", "missing_output", "extra_action", "wrong_output_name",
	}
	if id == "RP19" {
		controls = append(controls, "number_literal", "bool_literal", "duplicate_inline_action", "late_inline_action")
	}
	if id == "BRG08" {
		controls = append(controls, "native_numeric_kind", "literal_braces", "raw_escape", "reconciled_wrong_transform")
	}
	for _, fault := range controls {
		t.Run(fault, func(t *testing.T) {
			var broken = p15UTCopy(root).(map[string]any)
			if !reflect.DeepEqual(root, broken) {
				t.Fatal("native clone changed scalar kinds")
			}
			if _, err := p15UTGraph(broken, id); err != nil {
				t.Fatal("unchanged graph control", err)
			}
			var actions, _ = p15UTActions(broken)
			var token = p15UTParams(actions[view.text])["WFTextActionText"].(map[string]any)
			var body = token["Value"].(map[string]any)
			var refs = body["attachmentsByRange"].(map[string]any)
			var first = fmt.Sprintf("{%d, 1}", view.offsets[0])
			var cause = "joined token"
			switch fault {
			case "rune_offsets", "byte_offsets":
				var replaced = map[string]any{}
				var n, runeIndex = 0, 0
				for byteIndex, r := range view.template {
					if r == '\ufffc' {
						var offset = runeIndex
						if fault == "byte_offsets" {
							offset = byteIndex
						}
						replaced[fmt.Sprintf("{%d, 1}", offset)] = refs[fmt.Sprintf("{%d, 1}", view.offsets[n])]
						n++
					}
					runeIndex++
				}
				body["attachmentsByRange"] = replaced
				cause = "UTF-16 range"
			case "shift_one":
				refs[fmt.Sprintf("{%d, 1}", view.offsets[0]+1)] = refs[first]
				delete(refs, first)
				cause = "UTF-16 range"
			case "range_length_two":
				refs[fmt.Sprintf("{%d, 2}", view.offsets[0])] = refs[first]
				delete(refs, first)
				cause = "UTF-16 range"
			case "missing_attachment":
				delete(refs, first)
				cause = "UTF-16 range"
			case "extra_attachment":
				refs["{0, 1}"] = refs[first]
				cause = "UTF-16 range"
			case "combining_bytes":
				body["string"] = strings.Replace(view.template, "é", "é", 1)
				cause = "exact text bytes"
			case "hebrew_mark":
				body["string"] = strings.Replace(view.template, "ָ", "", 1)
				cause = "exact text bytes"
			case "zwj_bytes":
				body["string"] = strings.Replace(view.template, "\u200d", "", 1)
				cause = "exact text bytes"
			case "unbound_placeholder":
				body["string"] = view.template + "\ufffc"
				cause = "exact text bytes"
			case "swapped_producer":
				var other = 1
				if id == "BRG08" {
					other = 2
				}
				refs[first] = p15UTCopy(refs[fmt.Sprintf("{%d, 1}", view.offsets[other])])
				cause = "token attachment"
			case "late_producer", "late_inline_action":
				var index = 0
				if fault == "late_inline_action" {
					index = 2
				}
				var rows = broken["WFWorkflowActions"].([]any)
				var moved = rows[index]
				rows = append(rows[:index], rows[index+1:]...)
				var insertion = len(rows) - 1
				rows = append(rows[:insertion], append([]any{moved}, rows[insertion:]...)...)
				broken["WFWorkflowActions"] = rows
				cause = "missing or late producer"
			case "duplicate_uuid":
				p15UTParams(actions[1])["UUID"] = p15UTParams(actions[0])["UUID"]
				cause = "duplicate producer UUID"
			case "casefold_uuid_collision":
				var oldFirst = p15UTParams(actions[0])["UUID"].(string)
				var firstID = "abcde123-1234-4234-8234-123456789abc"
				p15UTRename(broken, oldFirst, firstID)
				if _, err := p15UTGraph(broken, id); err != nil {
					t.Fatal("valid reconciled identity rename", err)
				}
				var oldSecond = p15UTParams(actions[1])["UUID"].(string)
				p15UTRename(broken, oldSecond, strings.ToUpper(firstID))
				cause = "duplicate producer UUID"
			case "malformed_uuid", "zero_uuid":
				var value = "malformed"
				if fault == "zero_uuid" {
					value = "00000000-0000-0000-0000-000000000000"
				}
				p15UTRename(broken, p15UTParams(actions[0])["UUID"].(string), value)
				cause = "producer UUID shape"
			case "hidden_output":
				p15UTParams(actions[view.output])["WFOutput"] = "fabricated rendered result"
				cause = "token wrapper"
			case "missing_output":
				broken["WFWorkflowActions"] = broken["WFWorkflowActions"].([]any)[:view.output]
				cause = "missing native output"
			case "extra_action":
				broken["WFWorkflowActions"] = append(broken["WFWorkflowActions"].([]any), map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
				cause = "unexpected native actions"
			case "wrong_output_name":
				refs[first].(map[string]any)["OutputName"] = "foreign"
				cause = "token attachment"
			case "number_literal":
				body["string"] = strings.Replace(view.template, "42false", "43false", 1)
				cause = "exact text bytes"
			case "bool_literal":
				body["string"] = strings.Replace(view.template, "42false", "42true", 1)
				cause = "exact text bytes"
			case "duplicate_inline_action":
				var rows = broken["WFWorkflowActions"].([]any)
				var duplicate = p15UTCopy(rows[2]).(map[string]any)
				p15UTParams(duplicate)["UUID"] = "fedcba12-1234-4234-8234-123456789abc"
				broken["WFWorkflowActions"] = append(rows[:3], append([]any{duplicate}, rows[3:]...)...)
				cause = "joined token wrapper"
			case "native_numeric_kind":
				var fields = p15LiteralDictionaryFields(t, p15UTParams(actions[0])["WFItems"])
				if fields["payload"]["WFItemType"] != uint64(0) {
					t.Fatal("native integer control absent")
				}
				fields["payload"]["WFItemType"] = float64(0)
				cause = "native dictionary fields"
			case "literal_braces":
				p15UTParams(actions[2])["WFTextActionText"] = strings.Replace(p15UTPlain, "{seed}", "seed", 1)
				cause = "native gettext fields"
			case "raw_escape":
				p15UTParams(actions[3])["WFTextActionText"] = strings.Replace(p15UTRaw, "\\n", "\n", 1)
				cause = "native gettext fields"
			case "reconciled_wrong_transform":
				var direct = p15UTParams(actions[1])["WFVariable"].(map[string]any)["Value"].(map[string]any)
				direct["Aggrandizements"].([]any)[1].(map[string]any)["DictionaryKey"] = "other"
				for _, offset := range view.offsets[:2] {
					refs[fmt.Sprintf("{%d, 1}", offset)].(map[string]any)["Aggrandizements"].([]any)[1].(map[string]any)["DictionaryKey"] = "other"
				}
				cause = "native getvariable fields"
			}
			if _, err := p15UTGraph(broken, id); err == nil || !strings.Contains(err.Error(), cause) {
				t.Fatalf("causal native corruption %s missed %q: %v", fault, cause, err)
			}
		})
	}
	if id == "BRG08" {
		for _, path := range []string{"direct", "inline_first", "inline_second"} {
			for _, fault := range []string{"drop_chain", "swap_chain", "wrong_key", "wrong_coercion", "output_name"} {
				t.Run(path+"/"+fault, func(t *testing.T) {
					var broken = p15UTCopy(root).(map[string]any)
					if _, err := p15UTGraph(broken, id); err != nil {
						t.Fatal("unchanged graph control", err)
					}
					var actions, _ = p15UTActions(broken)
					var reference map[string]any
					var cause = "token attachment"
					if path == "direct" {
						reference = p15UTParams(actions[1])["WFVariable"].(map[string]any)["Value"].(map[string]any)
						cause = "native getvariable fields"
					} else {
						var n = 0
						if path == "inline_second" {
							n = 1
						}
						reference = p15UTParams(actions[view.text])["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)[fmt.Sprintf("{%d, 1}", view.offsets[n])].(map[string]any)
					}
					var chain = reference["Aggrandizements"].([]any)
					switch fault {
					case "drop_chain":
						delete(reference, "Aggrandizements")
					case "swap_chain":
						chain[0], chain[1] = chain[1], chain[0]
					case "wrong_key":
						chain[1].(map[string]any)["DictionaryKey"] = "other"
					case "wrong_coercion":
						chain[2].(map[string]any)["CoercionItemClass"] = "WFNumberContentItem"
					case "output_name":
						reference["OutputName"] = "foreign"
					}
					if _, err := p15UTGraph(broken, id); err == nil || !strings.Contains(err.Error(), cause) {
						t.Fatalf("causal transform control %s/%s: %v", path, fault, err)
					}
				})
			}
		}
	}
	t.Run("roundtrip", func(t *testing.T) {
		var observations = p15NativeImportRoundTrip(t, strings.ToLower(id)+"-joined", root, plist.XMLFormat, plist.BinaryFormat)
		if len(observations) != 8 {
			t.Fatal("incomplete XML/binary, hybrid/lossless, Legacy/V2 import inventory", len(observations))
		}
		for index, observation := range observations {
			if _, err := p15UTGraph(observation.compiled, id); err != nil {
				t.Fatalf("complete joined graph after import %d: %v", index, err)
			}
			p15UTNativeLog(t, id, fmt.Sprintf("roundtrip-%d-%s-%s", index, observation.fidelity, observation.mode), observation.compiled)
		}
	})
}
func TestP15RP19LiteralUnicodeNativeRoundTrip(t *testing.T)        { p15UTNative(t, "RP19") }
func TestBackendRecovery08InlineCodecNativeRoundTrip(t *testing.T) { p15UTNative(t, "BRG08") }
