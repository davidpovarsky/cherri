package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	gotoken "go/token"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func p15E06MobilePosition(text string, offset int) protocol.Position {
	prefix := text[:offset]
	start := strings.LastIndexByte(prefix, '\n') + 1
	return protocol.Position{Line: strings.Count(prefix, "\n"), Character: len(utf16.Encode([]rune(prefix[start:])))}
}
func p15E06MobileRange(text string, offset, size int) protocol.Range {
	return protocol.Range{Start: p15E06MobilePosition(text, offset), End: p15E06MobilePosition(text, offset+size)}
}

type p15E06MobileSnapshot struct {
	Hover      protocol.HoverResponse
	Definition protocol.DefinitionResponse
	Rename     protocol.RenameResponse
}

func p15E06MobileObserve(text string) p15E06MobileSnapshot {
	position := p15E06MobilePosition(text, strings.Index(text, "{ערך}")+1)
	return p15E06MobileSnapshot{
		Hover:      hoverForMobile(text, position.Line+1, position.Character+1),
		Definition: definitionForMobile(text, position.Line+1, position.Character+1),
		Rename:     renameForMobile(text, position.Line+1, position.Character+1, "renamed"),
	}
}
func p15E06MobileError(text string, actual p15E06MobileSnapshot) error {
	declaration, reference := strings.Index(text, "ערך"), strings.Index(text, "{ערך}")+1
	wantHover := protocol.HoverResponse{OK: true, Contents: "\x60\x60\x60cherri\n(variable) ערך\n\x60\x60\x60"}
	hoverRange := p15E06MobileRange(text, reference, len("ערך"))
	wantHover.Range = &hoverRange
	if !reflect.DeepEqual(actual.Hover, wantHover) {
		return fmt.Errorf("mobile hover selected wrong UTF16 target: %+v", actual.Hover)
	}
	defRange := p15E06MobileRange(text, declaration, len("ערך"))
	wantDefinition := protocol.DefinitionResponse{OK: true, URI: "file:///mobile.cherri", Range: &defRange}
	if !reflect.DeepEqual(actual.Definition, wantDefinition) {
		return fmt.Errorf("mobile definition selected wrong UTF16 target: %+v", actual.Definition)
	}
	expected := text[:reference] + "renamed" + text[reference+len("ערך"):]
	expected = expected[:declaration] + "renamed" + expected[declaration+len("ערך"):]
	if !actual.Rename.OK || actual.Rename.Error != "" || actual.Rename.Source != expected || len(actual.Rename.Edits) != 2 {
		return fmt.Errorf("mobile rename changed wrong source: %+v", actual.Rename)
	}
	want := []protocol.TextEditItem{{Range: defRange, NewText: "renamed"}, {Range: hoverRange, NewText: "renamed"}}
	matched := map[int]bool{}
	for _, edit := range actual.Rename.Edits {
		found := false
		for i, item := range want {
			if !matched[i] && reflect.DeepEqual(edit, item) {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("mobile rename has wrong wire range/text: %+v", edit)
		}
	}
	return nil
}
func TestP15E06MobileUTF16SymbolCoordinates(t *testing.T) {
	for _, tc := range []struct{ name, prefix, newline string }{
		{"ascii_lf", "plain", "\n"}, {"hebrew_lf", "שלום", "\n"}, {"emoji_crlf", "🙂🙂🙂🙂", "\r\n"}, {"zwj_combining_crlf", "👩‍💻👩‍💻e\u0301שלום", "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := "let ערך = \"literal ערך\"" + tc.newline + "// ערך remains" + tc.newline + "show(f\"" + tc.prefix + " {ערך}\")" + tc.newline
			if err := p15E06MobileError(text, p15E06MobileObserve(text)); err != nil {
				t.Fatal(err)
			}
		})
	}
}
func TestP15E06MobileUTF16CallCoordinates(t *testing.T) {
	for _, tc := range []struct{ name, prefix, newline string }{
		{"ascii_lf", "plain", "\n"}, {"hebrew_lf", "שלום", "\n"}, {"emoji_crlf", "🙂🙂🙂🙂", "\r\n"}, {"zwj_combining_crlf", "👩‍💻👩‍💻e\u0301שלום", "\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := "function target(first: Text, number: Number) -> Number { return number }" + tc.newline + "let numberValue = 3" + tc.newline + "let textValue = \"word\"" + tc.newline + "target(\"" + tc.prefix + "\", number: )"
			position := p15E06MobilePosition(text, len(text)-1)
			response := completeForMobile(text, position.Line+1, position.Character+1)
			if response.URI != "file:///mobile.cherri" || response.Version != 1 || response.LanguageVersion != schema.LanguageVersion || response.SchemaFingerprint == "" {
				t.Fatal("completion response identity changed", response)
			}
			var variables []string
			for _, item := range response.Items {
				if item.Kind == int(service.CompletionKindVariable) {
					variables = append(variables, item.Label)
				}
			}
			sort.Strings(variables)
			if !reflect.DeepEqual(variables, []string{"numberValue"}) {
				t.Errorf("UTF16 caret lost expected Number context: %v", variables)
			}
			sig := signatureHelpForMobile(text, position.Line+1, position.Character+1)
			if !sig.OK || !strings.HasPrefix(sig.Label, "function target(") || sig.ActiveParameter != 1 || !reflect.DeepEqual(sig.Parameters, []string{"first: Text", "number: Number"}) {
				t.Errorf("UTF16 signature target/slot changed: %+v", sig)
			}
		})
	}
}
func TestP15E06MobileCoordinateOracleCorruptions(t *testing.T) {
	text := "let ערך = \"literal ערך\"\r\nshow(f\"👩‍💻e\u0301 {ערך}\")\r\n"
	original := p15E06MobileObserve(text)
	if err := p15E06MobileError(text, original); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"hover_missing", "hover_target", "hover_column", "definition_document", "definition_reference", "rename_source", "rename_missing", "rename_extra", "rename_literal", "rename_text"} {
		t.Run(name, func(t *testing.T) {
			b, _ := json.Marshal(original)
			var changed p15E06MobileSnapshot
			if err := json.Unmarshal(b, &changed); err != nil {
				t.Fatal(err)
			}
			switch name {
			case "hover_missing":
				changed.Hover.OK = false
			case "hover_target":
				changed.Hover.Contents = "wrong"
			case "hover_column":
				changed.Hover.Range.Start.Character--
			case "definition_document":
				changed.Definition.URI = "file:///foreign.cherri"
			case "definition_reference":
				changed.Definition.Range = changed.Hover.Range
			case "rename_source":
				changed.Rename.Source = strings.Replace(changed.Rename.Source, "literal ערך", "literal renamed", 1)
			case "rename_missing":
				changed.Rename.Edits = changed.Rename.Edits[:1]
			case "rename_extra":
				changed.Rename.Edits = append(changed.Rename.Edits, changed.Rename.Edits[0])
			case "rename_literal":
				changed.Rename.Edits[0].Range = p15E06MobileRange(text, strings.Index(text, "literal ערך")+len("literal "), len("ערך"))
			case "rename_text":
				changed.Rename.Edits[0].NewText = "wrong"
			}
			if p15E06MobileError(text, changed) == nil {
				t.Fatal("accepted actual mobile response corruption", name)
			}
		})
	}
}
func TestP15E06MobileInvalidCoordinates(t *testing.T) {
	text := "let value = \"🙂\"\nshow(value)\n"
	for _, tc := range []struct {
		name         string
		line, column int
	}{{"zero_line", 0, 1}, {"negative_line", -1, 1}, {"zero_column", 2, 0}, {"negative_column", 2, -1}, {"past_line", 99, 6}, {"past_column", 2, 99}, {"surrogate_half", 1, 15}} {
		t.Run(tc.name, func(t *testing.T) {
			if got := hoverForMobile(text, tc.line, tc.column); got.OK {
				t.Error("invalid position produced hover", got)
			}
			if got := definitionForMobile(text, tc.line, tc.column); got.OK {
				t.Error("invalid position produced definition", got)
			}
			if got := renameForMobile(text, tc.line, tc.column, "renamed"); got.OK {
				t.Error("invalid position changed source", got)
			}
			if got := signatureHelpForMobile(text, tc.line, tc.column); got.OK {
				t.Error("invalid position produced signature", got)
			}
			if got := completeForMobile(text, tc.line, tc.column); len(got.Items) != 0 {
				t.Errorf("invalid position produced%d completions", len(got.Items))
			}
		})
	}
}

// This is an actual source/AST delegation check, not C/iOS execution.
func TestP15E06CExportsDelegateCoordinateProducers(t *testing.T) {
	raw, err := os.ReadFile("ios_bridge.go")
	if err != nil {
		t.Fatal(err)
	}
	file, err := parser.ParseFile(gotoken.NewFileSet(), "ios_bridge.go", raw, 0)
	if err != nil {
		t.Fatal(err)
	}
	for export, producer := range map[string]string{"CherriComplete": "completeForMobile", "CherriHover": "hoverForMobile", "CherriDefinition": "definitionForMobile", "CherriSignatureHelp": "signatureHelpForMobile", "CherriRename": "renameForMobile"} {
		t.Run(export, func(t *testing.T) {
			var target *ast.FuncDecl
			for _, decl := range file.Decls {
				if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == export {
					target = fn
				}
			}
			if target == nil {
				t.Fatal("missing actual C export")
			}
			calls := 0
			ast.Inspect(target.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				if name, ok := call.Fun.(*ast.Ident); ok && name.Name == producer {
					calls++
					if len(call.Args) < 3 {
						t.Error("coordinate arguments missing")
						return true
					}
					for index, argName := range []string{"line", "column"} {
						convert, ok := call.Args[index+1].(*ast.CallExpr)
						if !ok || len(convert.Args) != 1 {
							t.Error("coordinate forwarding changed")
							continue
						}
						callee, ok := convert.Fun.(*ast.Ident)
						if !ok || callee.Name != "int" {
							t.Error("coordinate conversion changed")
						}
						value, ok := convert.Args[0].(*ast.Ident)
						if !ok || value.Name != argName {
							t.Error("coordinate ownership changed")
						}
					}
				}
				if name, ok := call.Fun.(*ast.SelectorExpr); ok {
					switch name.Sel.Name {
					case "Complete", "Hover", "Definition", "SignatureHelp", "Rename":
						t.Error("C export bypasses actual mobile producer")
					}
				}
				return true
			})
			if calls != 1 {
				t.Fatal("C export must delegate exactly once", calls)
			}
		})
	}
}
