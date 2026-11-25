package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

// One real Server and Service keep the document throughout each scenario.
// Each batch crosses Run's actual JSON-RPC framing, including split UTF-8 reads.
type p15BRG27Session struct {
	t             *testing.T
	input, output bytes.Buffer
	server        *Server
}

func p15BRG27New(t *testing.T) *p15BRG27Session {
	t.Helper()
	s := &p15BRG27Session{t: t}
	s.server = NewServer(p15ByteReader{&s.input}, &s.output, schema.DefaultRegistry())
	s.exchange(p15Request("initialize", "initialize", map[string]any{}))
	return s
}
func (s *p15BRG27Session) exchange(messages ...map[string]any) []map[string]any {
	s.t.Helper()
	var request bytes.Buffer
	for _, msg := range messages {
		request.Write(p15Frame(s.t, msg))
	}
	s.input.Write(request.Bytes())
	before := s.output.Len()
	if err := s.server.Run(); err != nil {
		s.t.Fatal(err)
	}
	raw := append([]byte(nil), s.output.Bytes()[before:]...)
	trace, err := json.Marshal(map[string]any{"request": request.String(), "response": string(raw)})
	if err != nil {
		s.t.Fatal(err)
	}
	s.t.Logf("BRG27_WIRE %s", trace)
	return p15DecodeFrames(s.t, raw)
}
func p15BRG27Open(version int, text string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI, "version": version, "languageId": "cherri", "text": text}}}
}
func p15BRG27Change(version int, text string) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI, "version": version}, "contentChanges": []any{map[string]any{"text": text}}}}
}
func p15BRG27DiagnosticError(messages []map[string]any, version int, names ...string) error {
	if len(messages) != 1 || messages[0]["method"] != "textDocument/publishDiagnostics" {
		return fmt.Errorf("missing exact diagnostics notification")
	}
	p, ok := messages[0]["params"].(map[string]any)
	if !ok || p["uri"] != p15LSPURI || p["version"] != float64(version) {
		return fmt.Errorf("diagnostics lost active document/version: %#v", p)
	}
	ds, ok := p["diagnostics"].([]any)
	if !ok || len(ds) != len(names) {
		return fmt.Errorf("diagnostics count differs: %#v", p)
	}
	seen := map[string]bool{}
	for _, raw := range ds {
		d, ok := raw.(map[string]any)
		if !ok || d["code"] != "E_UNKNOWN_NAME" || d["severity"] != float64(1) {
			return fmt.Errorf("wrong diagnostic: %#v", d)
		}
		match := ""
		for _, name := range names {
			if d["message"] == "unknown identifier \""+name+"\"" {
				match = name
			}
		}
		if match == "" || seen[match] {
			return fmt.Errorf("foreign or duplicate diagnostic")
		}
		seen[match] = true
	}
	return nil
}
func p15BRG27RequireDiagnostics(t *testing.T, m []map[string]any, v int, names ...string) {
	t.Helper()
	if err := p15BRG27DiagnosticError(m, v, names...); err != nil {
		t.Fatal(err)
	}
}
func p15BRG27Apply(t *testing.T, text string, raw any) string {
	t.Helper()
	edit := p15Object(t, raw)
	rng := p15Object(t, edit["range"])
	a, err := p15E06ByteOffset(text, rng["start"])
	if err != nil {
		t.Fatal(err)
	}
	b, err := p15E06ByteOffset(text, rng["end"])
	if err != nil {
		t.Fatal(err)
	}
	value, ok := edit["newText"].(string)
	if !ok || a > b {
		t.Fatal("invalid actual wire edit", edit)
	}
	return text[:a] + value + text[b:]
}

func TestBackendRecovery27LSPVersionedJoinedDocument(t *testing.T) {
	s := p15BRG27New(t)
	healthy := "let ערך = \"literal ערך 🙂\"\n// ערך stays\nshow(f\"🙂{ערך}\")\n"
	old := healthy + "show(missingOld)\nshow(secondOld)\n"
	newer := healthy + "show(missingNewest)\n"
	first := s.exchange(p15BRG27Open(5, old))
	p15BRG27RequireDiagnostics(t, first, 5, "missingOld", "secondOld")
	next := s.exchange(p15BRG27Change(8, newer))
	p15BRG27RequireDiagnostics(t, next, 8, "missingNewest")
	stale := s.exchange(p15BRG27Change(6, old))
	p15BRG27RequireDiagnostics(t, stale, 8, "missingNewest")
	p15BRG27RequireDiagnostics(t, s.exchange(p15BRG27Change(9, healthy)), 9)
	offset := strings.Index(healthy, "{ערך}") + 1
	pos := p15E06Position(healthy, offset)
	params := p15PositionParams(pos["line"], pos["character"])
	rename := p15PositionParams(pos["line"], pos["character"])
	rename["newName"] = "סכום"
	m := s.exchange(p15Request("hover", "textDocument/hover", params), p15Request("definition", "textDocument/definition", params), p15Request("rename", "textDocument/rename", rename))
	hover := p15Object(t, p15Result(t, m, "hover"))
	if !p15E06JSONEqual(hover["range"], p15E06Range(healthy, offset, len("ערך"))) || !strings.Contains(fmt.Sprint(hover["contents"]), "(variable) ערך") {
		t.Fatal("hover lost newest UTF-16 source ownership", hover)
	}
	definition := p15Object(t, p15Result(t, m, "definition"))
	if !p15E06JSONEqual(definition, map[string]any{"uri": p15LSPURI, "range": p15E06Range(healthy, strings.Index(healthy, "ערך"), len("ערך"))}) {
		t.Fatal("definition points to another symbol", definition)
	}
	changes := p15Object(t, p15Object(t, p15Result(t, m, "rename"))["changes"])
	edits := p15Array(t, changes[p15LSPURI])
	if len(changes) != 1 || len(edits) != 2 {
		t.Fatal("rename escaped its semantic declaration and reference", changes)
	}
	expected := "let סכום = \"literal ערך 🙂\"\n// ערך stays\nshow(f\"🙂{סכום}\")\n"
	// Apply descending actual UTF-16 spans without invoking the source mapper.
	a, b := p15Object(t, edits[0]), p15Object(t, edits[1])
	ap, _ := p15E06ByteOffset(healthy, p15Object(t, a["range"])["start"])
	bp, _ := p15E06ByteOffset(healthy, p15Object(t, b["range"])["start"])
	if ap < bp {
		a, b = b, a
	}
	renamed := p15BRG27Apply(t, p15BRG27Apply(t, healthy, a), b)
	if renamed != expected {
		t.Fatalf("rename changed complete source: %q", renamed)
	}
	p15BRG27RequireDiagnostics(t, s.exchange(p15BRG27Change(10, renamed)), 10)
	// The retained old response still belongs to version5 after later edits.
	p15BRG27RequireDiagnostics(t, first, 5, "missingOld", "secondOld")
}

func TestBackendRecovery27LSPCompletionEdits(t *testing.T) {
	const declarations = "enum Color { Red = \"native-red\", Blue = \"native-blue\" }\nfunction paint(color: Color, note: Text = \"\") -> Text { return note }\n"
	cases := []struct {
		name, marked, label, old, newText, want, suffix string
		kind                                            int
	}{
		{"parameter_prefix", "resizeImage(Clipboard, width: 800, he|", "height", "he", "height: ", "resizeImage(Clipboard, width: 800, height: ", "600)\n", 5},
		{"parameter_middle_existing_colon", "resizeImage(Clipboard, width: 800, he|ighx: 600)\n", "height", "heighx: ", "height: ", "resizeImage(Clipboard, width: 800, height: 600)\n", "", 5},
		{"enum_member_suffix", "setBackgroundSound(.Ra|n)\n", "Rain", "Ran", "Rain", "setBackgroundSound(.Rain)\n", "", 20},
		{"enum_implicit_dot", "setBackgroundSound(Ra|)\n", "Rain", "Ra", ".Rain", "setBackgroundSound(.Rain)\n", "", 20},
		{"qualified_enum", declarations + "paint(Color.Re|x)\n", "Red", "Rex", "Red", declarations + "paint(Color.Red)\n", "", 20},
		{"hebrew_interpolation", "let סכום = 7\nshow(f\"🙂{סכ|ו}\")\n", "סכום", "סכו", "סכום", "let סכום = 7\nshow(f\"🙂{סכום}\")\n", "", 6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s := p15BRG27New(t)
			cursor := strings.Index(tc.marked, "|")
			if cursor < 0 {
				t.Fatal("missing authored cursor")
			}
			text := strings.Replace(tc.marked, "|", "", 1)
			s.exchange(p15BRG27Open(1, text))
			pos := p15E06Position(text, cursor)
			m := s.exchange(p15Request("complete", "textDocument/completion", p15PositionParams(pos["line"], pos["character"])))
			var selected map[string]any
			for _, raw := range p15Array(t, p15Result(t, m, "complete")) {
				item := p15Object(t, raw)
				if item["label"] == tc.label && item["kind"] == float64(tc.kind) {
					if selected != nil {
						t.Fatal("duplicate completion")
					}
					selected = item
				}
			}
			if selected == nil {
				t.Fatal("missing contextual item", tc.label)
			}
			edit, ok := selected["textEdit"].(map[string]any)
			start := strings.LastIndex(text, tc.old)
			want := map[string]any{"range": p15E06Range(text, start, len(tc.old)), "newText": tc.newText}
			if start < 0 || !ok || !p15E06JSONEqual(edit, want) {
				t.Fatalf("completion must return exact shared edit: got=%#v want=%#v", edit, want)
			}
			actual := p15BRG27Apply(t, text, edit)
			if actual != tc.want {
				t.Fatalf("completion changed whole source: got=%q want=%q", actual, tc.want)
			}
			p15BRG27RequireDiagnostics(t, s.exchange(p15BRG27Change(2, actual+tc.suffix)), 2)
		})
	}
}

func TestBackendRecovery27LSPExactFormatting(t *testing.T) {
	for _, tc := range []struct{ name, text, want string }{
		{"lf", "// ערך 🙂 stays\nlet ערך=1\nshow(\"literal ערך 🙂\")\n", "// ערך 🙂 stays\nlet ערך = 1\n\nshow(\"literal ערך 🙂\")\n"},
		{"crlf_no_final_newline", "// ערך 🙂 stays\r\nlet ערך=1\r\nshow(\"literal ערך 🙂\")", "// ערך 🙂 stays\r\nlet ערך = 1\n\nshow(\"literal ערך 🙂\")\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s := p15BRG27New(t)
			s.exchange(p15BRG27Open(1, tc.text))
			params := map[string]any{"textDocument": map[string]any{"uri": p15LSPURI}, "options": map[string]any{"tabSize": 4, "insertSpaces": true}}
			m := s.exchange(p15Request("format", "textDocument/formatting", params))
			edits := p15Array(t, p15Result(t, m, "format"))
			if len(edits) != 1 {
				t.Fatal("expected one exact source edit", edits)
			}
			edit := p15Object(t, edits[0])
			want := map[string]any{"range": p15E06Range(tc.text, 0, len(tc.text)), "newText": tc.want}
			if !p15E06JSONEqual(edit, want) {
				t.Fatalf("formatter lost actual EOF or literal/comment: got=%#v want=%#v", edit, want)
			}
			actual := p15BRG27Apply(t, tc.text, edit)
			if actual != tc.want {
				t.Fatal("formatted source changed", actual)
			}
			p15BRG27RequireDiagnostics(t, s.exchange(p15BRG27Change(2, actual)), 2)
			m = s.exchange(p15Request("second", "textDocument/formatting", params))
			again := p15Array(t, p15Result(t, m, "second"))
			if len(again) != 1 || p15BRG27Apply(t, actual, again[0]) != actual {
				t.Fatal("format is not idempotent", again)
			}
		})
	}
}

func TestBackendRecovery27VersionOracleRejectsChanges(t *testing.T) {
	s := p15BRG27New(t)
	original := s.exchange(p15BRG27Open(4, "show(missing)\n"))
	if err := p15BRG27DiagnosticError(original, 4, "missing"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_version", "older_version", "newer_version", "foreign_uri", "missing_diagnostic", "wrong_diagnostic"} {
		t.Run(fault, func(t *testing.T) {
			data, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var changed []map[string]any
			if err = json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			p := changed[0]["params"].(map[string]any)
			switch fault {
			case "missing_version":
				delete(p, "version")
			case "older_version":
				p["version"] = float64(3)
			case "newer_version":
				p["version"] = float64(5)
			case "foreign_uri":
				p["uri"] = "file:///foreign.cherri"
			case "missing_diagnostic":
				p["diagnostics"] = []any{}
			case "wrong_diagnostic":
				p["diagnostics"].([]any)[0].(map[string]any)["message"] = "unknown identifier \"other\""
			}
			if p15BRG27DiagnosticError(changed, 4, "missing") == nil {
				t.Fatal("accepted corrupted active snapshot", fault)
			}
		})
	}
}
