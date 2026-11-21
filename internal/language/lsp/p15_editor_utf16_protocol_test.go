package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

// This oracle counts actual source UTF-16 units independently of File and the
// LSP/service conversion helpers. No compiler range is used to derive a target.
func p15E06Position(code string, offset int) map[string]int {
	prefix := code[:offset]
	lineStart := strings.LastIndexByte(prefix, '\n') + 1
	return map[string]int{"line": strings.Count(prefix, "\n"), "character": len(utf16.Encode([]rune(prefix[lineStart:])))}
}

func p15E06Range(code string, offset, length int) map[string]any {
	return map[string]any{"start": p15E06Position(code, offset), "end": p15E06Position(code, offset+length)}
}

func p15E06JSONEqual(a, b any) bool {
	left, e1 := json.Marshal(a)
	right, e2 := json.Marshal(b)
	return e1 == nil && e2 == nil && bytes.Equal(left, right)
}

func p15E06Source(newline, prefix string) string {
	return "let ערך = \"literal ערך\"" + newline + "// ערך remains a comment" + newline + "show(f\"" + prefix + " {ערך} {missing}\")" + newline
}

func p15E06Exchange(t *testing.T, code string, capabilities any, extra ...map[string]any) []map[string]any {
	t.Helper()
	var input, output bytes.Buffer
	input.Write(p15Frame(t, p15Request("init", "initialize", map[string]any{"capabilities": capabilities})))
	input.Write(p15Frame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI, "version": 5, "languageId": "cherri", "text": code}}}))
	for _, message := range extra {
		input.Write(p15Frame(t, message))
	}
	if err := NewServer(p15ByteReader{&input}, &output, schema.DefaultRegistry()).Run(); err != nil {
		t.Fatal(err)
	}
	return p15DecodeFrames(t, output.Bytes())
}

func p15E06Snapshot(t *testing.T, code string, capabilities any) []map[string]any {
	t.Helper()
	offset := strings.Index(code, "{ערך}") + 1
	position := p15E06Position(code, offset)
	params := p15PositionParams(position["line"], position["character"])
	rename := p15PositionParams(position["line"], position["character"])
	rename["newName"] = "renamed"
	return p15E06Exchange(t, code, capabilities,
		p15Request("definition", "textDocument/definition", params),
		p15Request("hover", "textDocument/hover", params),
		p15Request("rename", "textDocument/rename", rename))
}

func p15E06SnapshotError(code string, messages []map[string]any) error {
	if len(messages) != 5 {
		return fmt.Errorf("expected exactly initialize, diagnostics, definition, hover and rename")
	}
	results := map[string]any{}
	var notifications []map[string]any
	for _, message := range messages {
		if message["jsonrpc"] != "2.0" {
			return fmt.Errorf("wrong JSON-RPC envelope")
		}
		if rawID, exists := message["id"]; exists {
			id, valid := rawID.(string)
			if !valid || (id != "init" && id != "definition" && id != "hover" && id != "rename") || message["error"] != nil {
				return fmt.Errorf("foreign/error response")
			}
			if _, duplicate := results[id]; duplicate {
				return fmt.Errorf("duplicate response")
			}
			value, exists := message["result"]
			if !exists {
				return fmt.Errorf("missing response result")
			}
			results[id] = value
		} else {
			notifications = append(notifications, message)
		}
	}
	if len(results) != 4 || len(notifications) != 1 {
		return fmt.Errorf("incomplete response inventory")
	}
	init, ok := results["init"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid initialize result")
	}
	caps, ok := init["capabilities"].(map[string]any)
	if !ok || caps["positionEncoding"] != "utf-16" {
		return fmt.Errorf("default/negotiated encoding is not UTF-16")
	}
	missing := strings.Index(code, "{missing}") + 1
	wantDiagnostics := map[string]any{"jsonrpc": "2.0", "method": "textDocument/publishDiagnostics", "params": map[string]any{
		"uri": p15LSPURI, "diagnostics": []any{map[string]any{"code": "E_UNKNOWN_NAME", "severity": 1, "message": "unknown identifier \"missing\"", "range": p15E06Range(code, missing, len("missing"))}},
	}}
	if !p15E06JSONEqual(notifications[0], wantDiagnostics) {
		return fmt.Errorf("diagnostic source/range/content differs: %#v", notifications[0])
	}
	declaration := strings.Index(code, "ערך")
	reference := strings.Index(code, "{ערך}") + 1
	wantDefinition := map[string]any{"uri": p15LSPURI, "range": p15E06Range(code, declaration, len("ערך"))}
	if !p15E06JSONEqual(results["definition"], wantDefinition) {
		return fmt.Errorf("definition target/range differs")
	}
	wantHover := map[string]any{"range": p15E06Range(code, reference, len("ערך")), "contents": map[string]string{"kind": "markdown", "value": "\x60\x60\x60cherri\n(variable) ערך\n\x60\x60\x60"}}
	if !p15E06JSONEqual(results["hover"], wantHover) {
		return fmt.Errorf("hover target/range differs")
	}
	rename, ok := results["rename"].(map[string]any)
	if !ok || len(rename) != 1 {
		return fmt.Errorf("invalid rename")
	}
	changes, ok := rename["changes"].(map[string]any)
	if !ok || len(changes) != 1 {
		return fmt.Errorf("rename changed foreign documents")
	}
	edits, ok := changes[p15LSPURI].([]any)
	if !ok || len(edits) != 2 {
		return fmt.Errorf("rename must edit exactly the declaration and its semantic reference")
	}
	wantEdits := []any{
		map[string]any{"range": p15E06Range(code, declaration, len("ערך")), "newText": "renamed"},
		map[string]any{"range": p15E06Range(code, reference, len("ערך")), "newText": "renamed"},
	}
	matched := [2]bool{}
	for _, edit := range edits {
		found := false
		for index, want := range wantEdits {
			if !matched[index] && p15E06JSONEqual(edit, want) {
				matched[index], found = true, true
				break
			}
		}
		if !found {
			return fmt.Errorf("rename changed literal/comment/range/text")
		}
	}
	// Apply the actual wire edits using an independent UTF-16-to-byte decoder,
	// then compare the entire document, including unchanged CRLF and literals.
	decoded := []struct {
		start, end int
		text       string
	}{}
	for _, value := range edits {
		edit := value.(map[string]any)
		rng := edit["range"].(map[string]any)
		from, err := p15E06ByteOffset(code, rng["start"])
		if err != nil {
			return err
		}
		to, err := p15E06ByteOffset(code, rng["end"])
		if err != nil {
			return err
		}
		decoded = append(decoded, struct {
			start, end int
			text       string
		}{from, to, edit["newText"].(string)})
	}
	sort.Slice(decoded, func(i, j int) bool { return decoded[i].start > decoded[j].start })
	actual := code
	for _, edit := range decoded {
		actual = actual[:edit.start] + edit.text + actual[edit.end:]
	}
	expected := code[:reference] + "renamed" + code[reference+len("ערך"):]
	expected = expected[:declaration] + "renamed" + expected[declaration+len("ערך"):]
	if actual != expected {
		return fmt.Errorf("wire rename does not reconstruct the complete independently authored document")
	}
	return nil
}

func p15E06ByteOffset(code string, position any) (int, error) {
	p, ok := position.(map[string]any)
	if !ok {
		return 0, fmt.Errorf("position is not an object")
	}
	line, lok := p["line"].(float64)
	character, cok := p["character"].(float64)
	if !lok || !cok || line != float64(int(line)) || character != float64(int(character)) || line < 0 || character < 0 {
		return 0, fmt.Errorf("invalid wire coordinate")
	}
	at, currentLine, units := 0, 0, 0
	for at < len(code) {
		if currentLine == int(line) && units == int(character) {
			return at, nil
		}
		r, size := utf8.DecodeRuneInString(code[at:])
		if r == '\n' {
			currentLine++
			units = 0
		} else {
			units += len(utf16.Encode([]rune{r}))
		}
		at += size
	}
	if currentLine == int(line) && units == int(character) {
		return at, nil
	}
	return 0, fmt.Errorf("coordinate is not a source UTF-16 boundary")
}

func TestP15E06UTF16FramedProtocol(t *testing.T) {
	for _, source := range []struct{ name, newline, prefix string }{
		{"hebrew_lf", "\n", "שלום"},
		{"emoji_crlf", "\r\n", "🙂שלום"},
		{"zwj_combining_crlf", "\r\n", "👩‍💻e\u0301שלום"},
	} {
		t.Run(source.name, func(t *testing.T) {
			for _, encoding := range []struct {
				name         string
				capabilities any
			}{
				{"default", map[string]any{}},
				{"explicit_utf16", map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-16"}}}},
				{"utf8_then_utf16", map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}}}},
			} {
				t.Run(encoding.name, func(t *testing.T) {
					code := p15E06Source(source.newline, source.prefix)
					messages := p15E06Snapshot(t, code, encoding.capabilities)
					if err := p15E06SnapshotError(code, messages); err != nil {
						t.Fatal(err)
					}
				})
			}
		})
	}
}

func TestP15E06WireOracleCorruptions(t *testing.T) {
	code := p15E06Source("\r\n", "👩‍💻e\u0301שלום")
	original := p15E06Snapshot(t, code, map[string]any{})
	if err := p15E06SnapshotError(code, original); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"encoding", "diagnostic_uri", "diagnostic_rune_column", "diagnostic_short_end", "diagnostic_line", "missing_diagnostic", "duplicate_diagnostic", "definition_uri", "definition_range", "hover_range", "hover_target", "rename_literal", "rename_text", "rename_column", "missing_response", "duplicate_response", "foreign_response", "jsonrpc"} {
		t.Run(name, func(t *testing.T) {
			data, _ := json.Marshal(original)
			var changed []map[string]any
			if err := json.Unmarshal(data, &changed); err != nil {
				t.Fatal(err)
			}
			diagParams := changed[1]["params"].(map[string]any)
			diags := diagParams["diagnostics"].([]any)
			diagnostic := diags[0].(map[string]any)
			result := func(id string) map[string]any {
				for _, row := range changed {
					if row["id"] == id {
						return row["result"].(map[string]any)
					}
				}
				t.Fatal("real response missing", id)
				return nil
			}
			switch name {
			case "encoding":
				result("init")["capabilities"].(map[string]any)["positionEncoding"] = "utf-8"
			case "diagnostic_uri":
				diagParams["uri"] = "file:///foreign.cherri"
			case "diagnostic_rune_column":
				diagnostic["range"].(map[string]any)["start"].(map[string]any)["character"] = float64(utf8.RuneCountInString("show(f\"👩‍💻e\u0301שלום {ערך} {"))
			case "diagnostic_short_end":
				diagnostic["range"].(map[string]any)["end"].(map[string]any)["character"] = 1.0
			case "diagnostic_line":
				diagnostic["range"].(map[string]any)["start"].(map[string]any)["line"] = 0.0
			case "missing_diagnostic":
				diagParams["diagnostics"] = []any{}
			case "duplicate_diagnostic":
				diagParams["diagnostics"] = append(diags, diags[0])
			case "definition_uri":
				result("definition")["uri"] = "file:///foreign.cherri"
			case "definition_range":
				result("definition")["range"] = p15E06Range(code, strings.Index(code, "{ערך}")+1, len("ערך"))
			case "hover_range":
				result("hover")["range"] = p15E06Range(code, strings.Index(code, "ערך"), len("ערך"))
			case "hover_target":
				result("hover")["contents"].(map[string]any)["value"] = "different"
			case "rename_literal":
				x := result("rename")["changes"].(map[string]any)
				x[p15LSPURI] = append(x[p15LSPURI].([]any), map[string]any{"range": p15E06Range(code, strings.Index(code, "literal ערך")+len("literal "), len("ערך")), "newText": "renamed"})
			case "rename_text":
				result("rename")["changes"].(map[string]any)[p15LSPURI].([]any)[0].(map[string]any)["newText"] = "wrong"
			case "rename_column":
				end := result("rename")["changes"].(map[string]any)[p15LSPURI].([]any)[0].(map[string]any)["range"].(map[string]any)["end"].(map[string]any)
				end["character"] = end["character"].(float64) + 1
			case "missing_response":
				changed = changed[:len(changed)-1]
			case "duplicate_response":
				changed[4] = changed[3]
			case "foreign_response":
				changed[4]["id"] = "foreign"
			case "jsonrpc":
				changed[0]["jsonrpc"] = "1.0"
			}
			if err := p15E06SnapshotError(code, changed); err == nil {
				t.Fatal("accepted actual wire corruption", name)
			}
		})
	}
}

func TestP15E06EditedSourceRecovery(t *testing.T) {
	code := p15E06Source("\r\n", "🙂")
	healthy := strings.Replace(code, "{missing}", "{ערך}", 1)
	change := func(version int, text string) map[string]any {
		return map[string]any{"jsonrpc": "2.0", "method": "textDocument/didChange", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI, "version": version}, "contentChanges": []any{map[string]any{"text": text}}}}
	}
	offset := strings.LastIndex(healthy, "{ערך}") + 1
	pos := p15E06Position(healthy, offset)
	messages := p15E06Exchange(t, code, map[string]any{},
		change(4, healthy), change(6, healthy),
		p15Request("current", "textDocument/hover", p15PositionParams(pos["line"], pos["character"])),
		map[string]any{"jsonrpc": "2.0", "method": "textDocument/didClose", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI}}},
		p15Request("closed", "textDocument/hover", p15PositionParams(pos["line"], pos["character"])))
	var published []map[string]any
	var closed map[string]any
	for _, msg := range messages {
		if msg["method"] == "textDocument/publishDiagnostics" {
			published = append(published, msg["params"].(map[string]any))
		}
		if msg["id"] == "closed" {
			closed = msg
		}
	}
	if len(published) != 3 || !reflect.DeepEqual(published[0], published[1]) || len(published[2]["diagnostics"].([]any)) != 0 {
		t.Fatal("stale/current source diagnostics do not preserve snapshot ownership", published)
	}
	hover := p15Result(t, messages, "current")
	want := map[string]any{"range": p15E06Range(healthy, offset, len("ערך")), "contents": map[string]string{"kind": "markdown", "value": "\x60\x60\x60cherri\n(variable) ערך\n\x60\x60\x60"}}
	if !p15E06JSONEqual(hover, want) {
		t.Fatal("fresh edit used old source positions", hover)
	}
	if closed == nil || !p15E06JSONEqual(closed["error"], map[string]any{"code": -32602, "message": "Invalid document or position"}) {
		t.Fatal("closed document response borrowed stale state", closed)
	}
}
