package lsp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

const p15LSPURI = "file:///rp38-context.cherri"

func p15Frame(t *testing.T, value any) []byte {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return append([]byte(fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))), body...)
}

func p15Request(id, method string, params any) map[string]any {
	return map[string]any{"jsonrpc": "2.0", "id": id, "method": method, "params": params}
}

func p15PositionParams(line, character int) map[string]any {
	return map[string]any{"textDocument": map[string]any{"uri": p15LSPURI}, "position": map[string]any{"line": line, "character": character}}
}

// Decode the real response bytes independently of Server.readMessage. Length is
// in UTF-8 bytes, every frame must be complete, and JSON-RPC response envelopes
// must contain exactly one of result/error, including an explicit null result.
func p15DecodeFrames(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	r := bufio.NewReader(bytes.NewReader(body))
	var messages []map[string]any
	for r.Buffered() > 0 || len(messages) == 0 {
		header, err := r.ReadString('\n')
		if errors.Is(err, io.EOF) && header == "" {
			break
		}
		if err != nil || !strings.HasPrefix(header, "Content-Length: ") {
			t.Fatalf("invalid response header %q: %v", header, err)
		}
		n, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(header, "Content-Length: ")))
		if err != nil || n <= 0 {
			t.Fatalf("invalid response length %q", header)
		}
		if blank, err := r.ReadString('\n'); err != nil || blank != "\r\n" {
			t.Fatalf("invalid response delimiter %q: %v", blank, err)
		}
		payload := make([]byte, n)
		if _, err := io.ReadFull(r, payload); err != nil {
			t.Fatal(err)
		}
		var msg map[string]any
		if err := json.Unmarshal(payload, &msg); err != nil {
			t.Fatal(err)
		}
		if msg["jsonrpc"] != "2.0" {
			t.Fatalf("wrong JSON-RPC version: %s", payload)
		}
		if _, response := msg["id"]; response {
			_, result := msg["result"]
			_, failure := msg["error"]
			if result == failure {
				t.Fatalf("response must have exactly one result/error: %s", payload)
			}
		}
		messages = append(messages, msg)
		if _, err := r.Peek(1); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			t.Fatal(err)
		}
	}
	return messages
}

// A one-byte reader forces the actual framing path to assemble split UTF-8 and
// headers rather than relying on one read per request.
type p15ByteReader struct{ io.Reader }

func (r p15ByteReader) Read(p []byte) (int, error) {
	if len(p) > 1 {
		p = p[:1]
	}
	return r.Reader.Read(p)
}

func p15Exchange(t *testing.T, source string, requests ...map[string]any) []map[string]any {
	t.Helper()
	var input, output bytes.Buffer
	input.Write(p15Frame(t, p15Request("init", "initialize", map[string]any{"capabilities": map[string]any{"general": map[string]any{"positionEncodings": []string{"utf-8", "utf-16"}}}})))
	if source != "" {
		input.Write(p15Frame(t, map[string]any{"jsonrpc": "2.0", "method": "textDocument/didOpen", "params": map[string]any{"textDocument": map[string]any{"uri": p15LSPURI, "version": 1, "languageId": "cherri", "text": source}}}))
	}
	for _, request := range requests {
		input.Write(p15Frame(t, request))
	}
	if err := NewServer(p15ByteReader{&input}, &output, schema.DefaultRegistry()).Run(); err != nil {
		t.Fatal(err)
	}
	return p15DecodeFrames(t, output.Bytes())
}

func p15Result(t *testing.T, messages []map[string]any, id string) any {
	t.Helper()
	for _, msg := range messages {
		if msg["id"] == id {
			if msg["error"] != nil {
				t.Fatalf("%s failed: %#v", id, msg)
			}
			return msg["result"]
		}
	}
	t.Fatalf("missing response %s", id)
	return nil
}

func p15Object(t *testing.T, value any) map[string]any {
	t.Helper()
	obj, ok := value.(map[string]any)
	if !ok {
		t.Fatalf("expected object, got %#v", value)
	}
	return obj
}
func p15Array(t *testing.T, value any) []any {
	t.Helper()
	array, ok := value.([]any)
	if !ok {
		t.Fatalf("expected array, got %#v", value)
	}
	return array
}
func p15UTF16(prefix string) int { return len(utf16.Encode([]rune(prefix))) }

func TestP15RP38UTF16RequestAndResponseRanges(t *testing.T) {
	source := "let x = \"visible\"\nshow(f\"🍒🍒{x}\")\n"
	params := p15PositionParams(1, p15UTF16("show(f\"🍒🍒{"))
	rename := p15PositionParams(1, p15UTF16("show(f\"🍒🍒{"))
	rename["newName"] = "renamed"
	messages := p15Exchange(t, source, p15Request("definition", "textDocument/definition", params), p15Request("hover", "textDocument/hover", params), p15Request("rename", "textDocument/rename", rename))
	capabilities := p15Object(t, p15Object(t, p15Result(t, messages, "init"))["capabilities"])
	if capabilities["positionEncoding"] != "utf-16" {
		t.Errorf("actual selected position encoding absent: %#v", capabilities["positionEncoding"])
	}
	definition := p15Object(t, p15Result(t, messages, "definition"))
	if definition["uri"] != p15LSPURI {
		t.Fatalf("wrong definition document: %#v", definition)
	}
	p15AssertRange(t, definition["range"], 0, 4, 0, 5)
	hover := p15Object(t, p15Result(t, messages, "hover"))
	if !strings.Contains(fmt.Sprint(p15Object(t, hover["contents"])["value"]), "(variable) x") {
		t.Fatalf("wrong hover: %#v", hover)
	}
	p15AssertRange(t, hover["range"], 1, p15UTF16("show(f\"🍒🍒{"), 1, p15UTF16("show(f\"🍒🍒{x"))
	changes := p15Object(t, p15Object(t, p15Result(t, messages, "rename"))["changes"])
	edits := p15Array(t, changes[p15LSPURI])
	if len(edits) != 2 {
		t.Fatalf("rename must affect exactly declaration and reference: %#v", edits)
	}
	seen := map[int]bool{}
	for _, value := range edits {
		edit := p15Object(t, value)
		if edit["newText"] != "renamed" {
			t.Fatal(edit)
		}
		line := int(p15Object(t, p15Object(t, edit["range"])["start"])["line"].(float64))
		if seen[line] {
			t.Fatal("duplicate edit")
		}
		seen[line] = true
		if line == 0 {
			p15AssertRange(t, edit["range"], 0, 4, 0, 5)
		} else if line == 1 {
			p15AssertRange(t, edit["range"], 1, p15UTF16("show(f\"🍒🍒{"), 1, p15UTF16("show(f\"🍒🍒{x"))
		} else {
			t.Fatal(edit)
		}
	}
}

func p15AssertRange(t *testing.T, value any, startLine, startChar, endLine, endChar int) {
	t.Helper()
	want := map[string]any{"start": map[string]any{"line": float64(startLine), "character": float64(startChar)}, "end": map[string]any{"line": float64(endLine), "character": float64(endChar)}}
	if !reflect.DeepEqual(value, want) {
		t.Fatalf("range %#v, want %#v", value, want)
	}
}

func TestP15RP38MultipleDiagnosticsUTF16(t *testing.T) {
	text := "let bad = \"🙂שלום\" + badVar\nshow(otherMissing)\n"
	messages := p15Exchange(t, text)
	found := map[string]bool{}
	for _, msg := range messages {
		if msg["method"] != "textDocument/publishDiagnostics" {
			continue
		}
		for _, value := range p15Array(t, p15Object(t, msg["params"])["diagnostics"]) {
			d := p15Object(t, value)
			message := fmt.Sprint(d["message"])
			if strings.Contains(message, "badVar") {
				p15AssertRange(t, d["range"], 0, p15UTF16("let bad = \"🙂שלום\" + "), 0, p15UTF16("let bad = \"🙂שלום\" + badVar"))
				found["badVar"] = true
			}
			if strings.Contains(message, "otherMissing") {
				p15AssertRange(t, d["range"], 1, 5, 1, 17)
				found["otherMissing"] = true
			}
		}
	}
	if len(found) != 2 {
		t.Fatalf("multiple independent diagnostics not returned: %#v", found)
	}
}

func TestP15RP38ContextualLabelsEnumsAndSignatures(t *testing.T) {
	t.Run("named labels and LSP kinds", func(t *testing.T) {
		text := "resizeImage(Clipboard, width: 800, "
		messages := p15Exchange(t, text, p15Request("complete", "textDocument/completion", p15PositionParams(0, p15UTF16(text))))
		found := false
		for _, value := range p15Array(t, p15Result(t, messages, "complete")) {
			item := p15Object(t, value)
			if item["label"] == "width" && item["kind"] == float64(5) {
				t.Fatal("already-used named label offered")
			}
			if item["label"] == "height" {
				if item["kind"] != float64(5) || !strings.Contains(fmt.Sprint(item["detail"]), "height: AnyContent (parameter)") || item["insertText"] != "height: " {
					t.Fatalf("wrong typed parameter completion: %#v", item)
				}
				found = true
			}
			if item["label"] == "let" && item["kind"] != float64(14) {
				t.Fatalf("internal enum leaked as LSP kind: %#v", item)
			}
		}
		if !found {
			t.Fatal("missing contextual height parameter")
		}
	})
	t.Run("enum domain", func(t *testing.T) {
		text := "setBackgroundSound(."
		messages := p15Exchange(t, text, p15Request("complete", "textDocument/completion", p15PositionParams(0, len(text))))
		found := map[string]bool{}
		for _, value := range p15Array(t, p15Result(t, messages, "complete")) {
			item := p15Object(t, value)
			if item["kind"] == float64(20) {
				found[fmt.Sprint(item["label"])] = true
			}
		}
		if !found["Ocean"] || !found["Rain"] || found["Blue"] || found["Only"] || len(found) == 0 {
			t.Fatalf("wrong contextual enum members: %#v", found)
		}
	})
	t.Run("typed function and string commas", func(t *testing.T) {
		text := "function greet(person: Text, times: Number, again: Number) -> Text { return person }\ngreet(person: \"a,b,c\", times: "
		last := strings.Split(text, "\n")[1]
		messages := p15Exchange(t, text, p15Request("signature", "textDocument/signatureHelp", p15PositionParams(1, len(last))))
		sig := p15Object(t, p15Result(t, messages, "signature"))
		if sig["activeParameter"] != float64(1) {
			t.Fatalf("comma inside a string changed active parameter: %#v", sig)
		}
		label := fmt.Sprint(p15Object(t, p15Array(t, sig["signatures"])[0])["label"])
		if !strings.Contains(label, "person: Text") || !strings.Contains(label, "times: Number") || !strings.Contains(label, "-> Text") {
			t.Fatalf("typed signature lost source types: %s", label)
		}
	})
}

func TestP15RP38ScopedServiceCompletion(t *testing.T) {
	svc := service.NewService(schema.DefaultRegistry())
	text := "let outer = \"outside\"\nif true {\n    let inner = 7\n    show(inner)\n}\nshow(outer)\nlet future = 8\nfunction scoped(person: Text) -> Text {\n    let local = person\n    return local\n}\n"
	svc.OpenDocument(p15LSPURI, 1, text)
	for _, tc := range []struct {
		name            string
		line, col       int
		present, absent []string
	}{
		{"inner", 4, 10, []string{"inner", "outer"}, []string{"future", "person", "local"}},
		{"outer", 6, 8, []string{"outer"}, []string{"inner", "future", "person", "local"}},
		{"function", 10, 12, []string{"person", "local"}, []string{"inner"}},
		{"after_block_closer", 5, 2, []string{"outer"}, []string{"inner", "future", "person", "local"}},
		{"after_function_closer", 11, 2, []string{"outer", "future"}, []string{"inner", "person", "local"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := map[string]bool{}
			for _, item := range svc.Complete(p15LSPURI, tc.line, tc.col) {
				if item.Kind == service.CompletionKindVariable {
					found[item.Label] = true
				}
			}
			for _, name := range tc.present {
				if !found[name] {
					t.Errorf("visible variable %s missing", name)
				}
			}
			for _, name := range tc.absent {
				if found[name] {
					t.Errorf("out-of-scope variable %s offered", name)
				}
			}
		})
	}
}

func TestP15RP38NestedAndMultilineContext(t *testing.T) {
	declarations := "function describe(note: Text, width: Number, height: Number) -> Text { return note }\n"
	for _, call := range []string{"describe(note: \"width: and commas, here\", ", "describe(\n note: \"width: and commas, here\",\n "} {
		text := declarations + call
		lines := strings.Split(text, "\n")
		messages := p15Exchange(t, text, p15Request("complete", "textDocument/completion", p15PositionParams(len(lines)-1, p15UTF16(lines[len(lines)-1]))))
		found := map[string]bool{}
		for _, value := range p15Array(t, p15Result(t, messages, "complete")) {
			item := p15Object(t, value)
			if item["kind"] == float64(5) {
				found[fmt.Sprint(item["label"])] = true
			}
		}
		if !found["width"] || !found["height"] || found["note"] {
			t.Fatalf("string content or newline changed named labels: %#v", found)
		}
	}
	text := declarations + "describe(height: "
	last := strings.Split(text, "\n")[1]
	messages := p15Exchange(t, text, p15Request("signature", "textDocument/signatureHelp", p15PositionParams(1, len(last))))
	if p15Object(t, p15Result(t, messages, "signature"))["activeParameter"] != float64(2) {
		t.Fatal("named argument must select its declaration position")
	}
	for _, partial := range []string{"take([1, 2", "take((", "take({ key: 1, other: 2"} {
		source := "function take(items: List<Number>, label: Text) -> Text { return label }\n" + partial
		messages := p15Exchange(t, source, p15Request("signature", "textDocument/signatureHelp", p15PositionParams(1, len(partial))), p15Request("complete", "textDocument/completion", p15PositionParams(1, len(partial))))
		foundTypedList := false
		for _, value := range p15Array(t, p15Result(t, messages, "complete")) {
			item := p15Object(t, value)
			if item["kind"] == float64(5) && item["label"] == "items" {
				if item["detail"] != "items: List<Number> (parameter)" {
					t.Fatalf("generic parameter type lost: %#v", item)
				}
				foundTypedList = true
			}
		}
		if !foundTypedList {
			t.Fatal("typed List parameter completion missing")
		}
		signature := p15Object(t, p15Result(t, messages, "signature"))
		label := fmt.Sprint(p15Object(t, p15Array(t, signature["signatures"])[0])["label"])
		if !strings.HasPrefix(label, "function take(") || signature["activeParameter"] != float64(0) {
			t.Fatalf("nested incomplete value lost its enclosing call: %#v", signature)
		}
	}
	partial := "setBackgroundSound([\"unrelated\", "
	messages = p15Exchange(t, partial, p15Request("complete", "textDocument/completion", p15PositionParams(0, len(partial))))
	for _, value := range p15Array(t, p15Result(t, messages, "complete")) {
		if item := p15Object(t, value); item["kind"] == float64(20) {
			t.Fatalf("outer enum leaked inside a nested list value: %#v", item)
		}
	}
}

func TestP15RP38ProtocolIDsAndNotifications(t *testing.T) {
	var input, output bytes.Buffer
	input.Write(p15Frame(t, map[string]any{"jsonrpc": "2.0", "id": json.Number("9007199254740993"), "method": "initialize", "params": map[string]any{}}))
	input.Write(p15Frame(t, map[string]any{"jsonrpc": "2.0", "method": "cherri/unknownNotification", "params": map[string]any{}}))
	if err := NewServer(p15ByteReader{&input}, &output, schema.DefaultRegistry()).Run(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "\"id\":9007199254740993") {
		t.Fatalf("numeric request identity was rounded: %s", output.String())
	}
	if len(p15DecodeFrames(t, output.Bytes())) != 1 {
		t.Fatal("notification produced an unsolicited response")
	}
}

func TestP15RP38ProtocolMethodsAndNull(t *testing.T) {
	messages := p15Exchange(t, "let x = 1\n", p15Request("null", "textDocument/definition", p15PositionParams(0, 0)), p15Request("unknown", "cherri/unknownMethod", map[string]any{}), p15Request("invalid", "textDocument/completion", []any{1}), p15Request("shutdown", "shutdown", nil))
	if p15Result(t, messages, "null") != nil || p15Result(t, messages, "shutdown") != nil {
		t.Fatal("null success must encode result:null")
	}
	want := map[string]float64{"unknown": -32601, "invalid": -32602}
	for _, msg := range messages {
		id, _ := msg["id"].(string)
		if code, ok := want[id]; ok {
			if p15Object(t, msg["error"])["code"] != code {
				t.Fatalf("wrong protocol error for %s: %#v", id, msg)
			}
			delete(want, id)
		}
	}
	if len(want) != 0 {
		t.Fatalf("requests silently dropped: %#v", want)
	}
}

func TestP15RP38MalformedFramingFailsWithoutPanic(t *testing.T) {
	for _, input := range []string{"Content-Length: -1\r\n\r\n", "Content-Length: abc\r\n\r\n", "Content-Length: 999999999999999999999\r\n\r\n", "Content-Length: 2\r\nContent-Length: 2\r\n\r\n{}", "Content-Length: 3\r\n\r\n{}", "Content-Length: 3\r\n\r\n", "Content-Length: 3", "Content-Length: 0\r\n\r\n"} {
		t.Run(strings.ReplaceAll(input, "\r\n", "_"), func(t *testing.T) {
			defer func() {
				if value := recover(); value != nil {
					t.Errorf("untrusted framing panicked: %v", value)
				}
			}()
			if err := NewServer(strings.NewReader(input), io.Discard, schema.DefaultRegistry()).Run(); err == nil {
				t.Fatal("malformed/truncated framing accepted")
			}
		})
	}
}

type p15FailWriter struct{ err error }

func (w p15FailWriter) Write([]byte) (int, error) { return 0, w.err }

type p15ShortWriter struct{}

func (p15ShortWriter) Write(p []byte) (int, error) { return len(p) / 2, nil }

func TestP15RP38WriteFailuresAreObserved(t *testing.T) {
	sentinel := errors.New("reviewed output write failure")
	for _, tc := range []struct {
		name   string
		writer io.Writer
		want   error
	}{{"error", p15FailWriter{sentinel}, sentinel}, {"short", p15ShortWriter{}, io.ErrShortWrite}} {
		t.Run(tc.name, func(t *testing.T) {
			input := p15Frame(t, p15Request("init", "initialize", map[string]any{}))
			if err := NewServer(bytes.NewReader(input), tc.writer, schema.DefaultRegistry()).Run(); !errors.Is(err, tc.want) {
				t.Fatalf("lost transport failure: %v, want %v", err, tc.want)
			}
		})
	}
}
