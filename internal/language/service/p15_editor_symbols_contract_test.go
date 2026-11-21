package service

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/source"
)

// The marked authored source defines every and only occurrence that may change.
// No production lexer, resolver or edit helper is used to derive the oracle.
func p15E03MarkedSource(marked, name, replacement string) (string, string, []int) {
	token := "\u00ab" + name + "\u00bb"
	var code, expected strings.Builder
	var offsets []int
	for {
		at := strings.Index(marked, token)
		if at < 0 {
			code.WriteString(marked)
			expected.WriteString(marked)
			break
		}
		code.WriteString(marked[:at])
		expected.WriteString(marked[:at])
		offsets = append(offsets, code.Len())
		code.WriteString(name)
		expected.WriteString(replacement)
		marked = marked[at+len(token):]
	}
	return code.String(), expected.String(), offsets
}

func p15E03RenameError(code, uri, name, replacement, actual, expected string, edits []TextEdit, offsets []int) error {
	if actual != expected || len(edits) != len(offsets) {
		return fmt.Errorf("semantic rename differs: edits=%+v expectedOffsets=%v actual=%q expected=%q", edits, offsets, actual, expected)
	}
	for i, offset := range offsets {
		edit := edits[i]
		if edit.NewText != replacement || edit.Span.Start.ByteOffset != offset || edit.Span.End.ByteOffset != offset+len(name) {
			return fmt.Errorf("wrong edited occurrence %d: %+v expected offset %d", i, edit, offset)
		}
		if err := p15EditorSpanError(code, uri, edit.Span); err != nil {
			return err
		}
	}
	return nil
}

func p15E03Healthy(t *testing.T, svc *Service, uri string) {
	t.Helper()
	ds, fingerprint := svc.Analyze(uri)
	if len(fingerprint) != 64 {
		t.Fatal("canonical service identity absent")
	}
	for _, d := range ds {
		if d.Severity == analysis.SeverityError {
			t.Fatalf("authored positive source is not valid: %+v", ds)
		}
	}
}

func TestP15E03ExactScopedSymbols(t *testing.T) {
	for _, tc := range []struct {
		name, symbol, replacement, marked string
		function                          bool
	}{
		{"outer_unicode", "\u05e2\u05e8\u05da", "outerRenamed", "function echo(person: Text) -> Text { return person }\nlet \u00ab\u05e2\u05e8\u05da\u00bb = 7\nlet label = \"\u05e2\u05e8\u05da echo\"\n// \u05e2\u05e8\u05da echo\nif true {\n let \u05e2\u05e8\u05da = 8\n show(f\"{\u05e2\u05e8\u05da}\")\n}\nshow(f\"\U0001F642\u05e9\u05dc\u05d5\u05dd {\u00ab\u05e2\u05e8\u05da\u00bb}\")\nshow(\u00ab\u05e2\u05e8\u05da\u00bb)\necho(\"x\")\n", false},
		{"inner_shadow", "count", "innerRenamed", "let count = 1\nif true {\n let \u00abcount\u00bb = 2\n show(f\"{\u00abcount\u00bb}\")\n}\nshow(count)\n", false},
		{"function_and_call", "greet", "welcome", "function \u00abgreet\u00bb(person: Text) -> Text { return person }\nlet label = \"greet\"\n// greet\nshow(\u00abgreet\u00bb(\"x\"))\n", true},
		{"parameter_and_body", "person", "guest", "function greet(\u00abperson\u00bb: Text) -> Text { return f\"Hi {\u00abperson\u00bb}\" }\nshow(greet(\"x\"))\n", false},
		{"loop_shadow", "item", "entry", "let item = \"outer\"\nfor \u00abitem\u00bb in [\"a\"] { show(\u00abitem\u00bb) }\nshow(item)\n", false},
		{"unrelated_shadow_remains", "original", "target", "let \u00aboriginal\u00bb = 1\nif true { let target = 2; show(target) }\nshow(\u00aboriginal\u00bb)\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e03-symbol.cherri"
			code, expected, offsets := p15E03MarkedSource(tc.marked, tc.symbol, tc.replacement)
			if len(offsets) < 2 || strings.ContainsAny(code, "\u00ab\u00bb") {
				t.Fatal("independent marked source is malformed")
			}
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, code)
			p15E03Healthy(t, svc, uri)
			for _, offset := range offsets {
				position := p15EditorExpectedPosition(code, offset)
				definition := svc.Definition(uri, position.Line, position.Column)
				if definition == nil || definition.Start.ByteOffset != offsets[0] || definition.End.ByteOffset != offsets[0]+len(tc.symbol) {
					t.Fatalf("definition must select exact lexical owner at %d: %+v", offset, definition)
				}
				if err := p15EditorSpanError(code, uri, *definition); err != nil {
					t.Fatal(err)
				}
				hover := svc.Hover(uri, position.Line, position.Column)
				if hover == nil || hover.Span.Start.ByteOffset != offset || hover.Span.End.ByteOffset != offset+len(tc.symbol) {
					t.Fatalf("hover target differs at %d: %+v", offset, hover)
				}
				if err := p15EditorSpanError(code, uri, hover.Span); err != nil {
					t.Fatal(err)
				}
				want := "(variable) " + tc.symbol
				if tc.function {
					want = "function " + tc.symbol + "("
				}
				if !strings.Contains(hover.Contents, want) || strings.Contains(hover.Contents, "**Action:**") {
					t.Fatal("hover selected a different semantic owner", hover)
				}
			}
			position := p15EditorExpectedPosition(code, offsets[len(offsets)-1])
			actual, edits, err := svc.Rename(uri, position.Line, position.Column, tc.replacement)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15E03RenameError(code, uri, tc.symbol, tc.replacement, actual, expected, edits, offsets); err != nil {
				t.Fatal(err)
			}
			svc.ChangeDocument(uri, 2, actual)
			p15E03Healthy(t, svc, uri)
		})
	}
}

func TestP15E03NonSymbolPositions(t *testing.T) {
	for _, tc := range []struct{ name, marked string }{
		{"quoted_literal", "let value=1\nlet text=\"va|lue\"\nshow(value)\n"},
		{"line_comment", "let value=1\n// va|lue\nshow(value)\n"},
		{"fstring_literal", "let value=1\nshow(f\"va|lue {value}\")\n"},
		{"dictionary_key", "let value=1\nlet data={\"va|lue\": 2}\nshow(value)\n"},
		{"canonical_named_label", "let decimalPlaces=2\nformatNumber(1, decimal|Places: decimalPlaces)\n"},
		{"record_member_name", "let value=1\nlet data={\"value\":2}\nshow(data.va|lue)\n"},
		{"type_annotation", "let Text=1\nlet value: Te|xt=\"x\"\nshow(Text)\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e03-nonsymbol.cherri"
			code, line, column := p15EditorCursor(t, tc.marked)
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, code)
			p15E03Healthy(t, svc, uri)
			if got := svc.Definition(uri, line, column); got != nil {
				t.Errorf("non-reference obtained a local definition: %+v", got)
			}
			if got := svc.Hover(uri, line, column); got != nil {
				t.Errorf("non-reference obtained local hover: %+v", got)
			}
			if updated, edits, err := svc.Rename(uri, line, column, "changed"); err == nil || updated != "" || len(edits) != 0 {
				t.Errorf("non-reference renamed source: error=%v edits=%+v source=%q", err, edits, updated)
			}
		})
	}
	t.Run("invalid_column_is_not_clamped", func(t *testing.T) {
		const uri = "file:///p15-e03-position.cherri"
		const code = "let value=1\nshow(value)\nvalue"
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		for _, column := range []int{0, -1, 1000} {
			if got := svc.Definition(uri, 3, column); got != nil {
				t.Errorf("out-of-range definition column%d: %+v", column, got)
			}
			if got := svc.Hover(uri, 3, column); got != nil {
				t.Errorf("out-of-range hover column%d: %+v", column, got)
			}
			if updated, edits, err := svc.Rename(uri, 3, column, "changed"); err == nil || updated != "" || len(edits) != 0 {
				t.Errorf("out-of-range rename column%d: %v %+v", column, err, edits)
			}
		}
	})
}

func TestP15E03AuthoredSignatureOwnership(t *testing.T) {
	for _, tc := range []struct {
		name, marked, label string
		parameters          []string
		active              int
	}{
		{"function_shadows_action", "function round(value: Text) -> Text { return value }\nround(\"x|y\")", "function round(value: Text) -> Text", []string{"value: Text"}, 0},
		{"named_parameter_order", "function combine(value: Text, count: Number = 1) -> Text { return value }\ncombine(count: 2|, value: \"x\")", "function combine(value: Text, count: Number) -> Text", []string{"value: Text", "count: Number"}, 1},
		{"nested_call_owner", "function outer(value: Number) -> Number { return value }\nfunction inner(value: Text) -> Number { return 1 }\nouter(inner(\"x|y\"))", "function inner(value: Text) -> Number", []string{"value: Text"}, 0},
		{"multiline_named", "function combine(value: Text, count: Number = 1) -> Text { return value }\ncombine(\n count: 2,\n value: \"x|y\"\n)", "function combine(value: Text, count: Number) -> Text", []string{"value: Text", "count: Number"}, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e03-signature.cherri"
			code, line, column := p15EditorCursor(t, tc.marked)
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, code)
			p15E03Healthy(t, svc, uri)
			got := svc.SignatureHelp(uri, line, column)
			if got == nil || got.Label != tc.label || !reflect.DeepEqual(got.Parameters, tc.parameters) || got.ActiveParameter != tc.active {
				t.Fatalf("wrong exact owned signature: %+v want %q/%v/%d", got, tc.label, tc.parameters, tc.active)
			}
		})
	}
	for _, tc := range []struct{ name, marked, want string }{
		{"authored_function_hover", "function round(value: Text) -> Text { return value }\nro|und(\"x\")", "function round(value)"},
		{"local_variable_hover", "let round=7\nshow(ro|und)", "(variable) round"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e03-hover.cherri"
			code, line, column := p15EditorCursor(t, tc.marked)
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, code)
			p15E03Healthy(t, svc, uri)
			got := svc.Hover(uri, line, column)
			if got == nil || !strings.Contains(got.Contents, tc.want) || strings.Contains(got.Contents, "**Action:**") {
				t.Fatalf("canonical action replaced authored owner: %+v", got)
			}
		})
	}
}

func TestP15E03RenameMustNotCapture(t *testing.T) {
	for _, tc := range []struct{ name, marked, newName string }{
		{"same_scope_duplicate", "let fi|rst=1\nlet second=2\nshow(first)\nshow(second)\n", "second"},
		{"selected_reference_captured", "let so|urce=1\nif true { let target=2; show(source); show(target) }\n", "target"},
		{"unselected_reference_captured", "let target=1\nif true { let so|urce=2; show(target); show(source) }\n", "target"},
		{"canonical_call_captured", "let so|urce=1\nshow(round(2))\nshow(source)\n", "round"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			const uri = "file:///p15-e03-capture.cherri"
			code, line, column := p15EditorCursor(t, tc.marked)
			svc := NewService(nil)
			svc.OpenDocument(uri, 1, code)
			p15E03Healthy(t, svc, uri)
			if updated, edits, err := svc.Rename(uri, line, column, tc.newName); err == nil || updated != "" || len(edits) != 0 {
				t.Fatalf("rename changed lexical identity: error=%v edits=%+v source=%q", err, edits, updated)
			}
			after, _ := svc.Analyze(uri)
			before := NewService(nil)
			before.OpenDocument(uri, 1, code)
			original, _ := before.Analyze(uri)
			if !reflect.DeepEqual(after, original) {
				t.Fatal("rejected operation mutated service state")
			}
		})
	}
}

func TestP15E03RenameCompleteOracleMutations(t *testing.T) {
	const uri = "file:///p15-e03-rename-oracle.cherri"
	const name = "\u05e2\u05e8\u05da"
	code, expected, offsets := p15E03MarkedSource("let \u00ab\u05e2\u05e8\u05da\u00bb=1\n// \u05e2\u05e8\u05da\nshow(f\"\U0001F642{\u00ab\u05e2\u05e8\u05da\u00bb}\")\n", name, "renamed")
	svc := NewService(nil)
	svc.OpenDocument(uri, 1, code)
	p15E03Healthy(t, svc, uri)
	pos := p15EditorExpectedPosition(code, offsets[1])
	actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "renamed")
	if err != nil {
		t.Fatal(err)
	}
	if err := p15E03RenameError(code, uri, name, "renamed", actual, expected, edits, offsets); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "extra", "comment", "replacement", "start", "end", "document", "rune", "utf16", "text_only"} {
		t.Run(name, func(t *testing.T) {
			changed := append([]TextEdit{}, edits...)
			text := actual
			switch name {
			case "missing":
				changed = changed[:1]
			case "extra":
				changed = append(changed, changed[0])
			case "comment":
				off := strings.Index(code, "// ") + 3
				changed[1].Span = source.Span{Document: uri, Start: p15EditorExpectedPosition(code, off), End: p15EditorExpectedPosition(code, off+len("\u05e2\u05e8\u05da"))}
			case "replacement":
				changed[1].NewText = "other"
			case "start":
				changed[1].Span.Start.ByteOffset++
			case "end":
				changed[1].Span.End.ByteOffset--
			case "document":
				changed[1].Span.Document = "file:///wrong.cherri"
			case "rune":
				changed[1].Span.Start.Column++
			case "utf16":
				changed[1].Span.Start.UTF16Column = changed[1].Span.Start.Column
			case "text_only":
				text = code
			}
			if p15E03RenameError(code, uri, "\u05e2\u05e8\u05da", "renamed", text, expected, changed, offsets) == nil {
				t.Fatal("corrupted actual semantic rename accepted", name)
			}
		})
	}
}

func TestP15E03NamedParameterRenameOwnership(t *testing.T) {
	const uri = "file:///p15-e03-parameter-label.cherri"
	const marked = "function greet(\u00abperson\u00bb: Text) -> Text { return \u00abperson\u00bb }\nfunction other(person: Text) -> Text { return person }\nlet person=\"local\"\nshow(greet(\u00abperson\u00bb: person))\nshow(other(person: person))\n"
	code, expected, offsets := p15E03MarkedSource(marked, "person", "guest")
	svc := NewService(nil)
	svc.OpenDocument(uri, 1, code)
	p15E03Healthy(t, svc, uri)
	for _, offset := range offsets {
		pos := p15EditorExpectedPosition(code, offset)
		def := svc.Definition(uri, pos.Line, pos.Column)
		if def == nil || def.Start.ByteOffset != offsets[0] || def.End.ByteOffset != offsets[0]+len("person") {
			t.Errorf("named parameter did not resolve exact owner at%d: %+v", offset, def)
		}
	}
	pos := p15EditorExpectedPosition(code, offsets[0])
	actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "guest")
	if err != nil {
		t.Fatal(err)
	}
	if err := p15E03RenameError(code, uri, "person", "guest", actual, expected, edits, offsets); err != nil {
		t.Fatal(err)
	}
	svc.ChangeDocument(uri, 2, actual)
	p15E03Healthy(t, svc, uri)
}

func TestP15E03UnusedSameScopeRenameCollision(t *testing.T) {
	const uri = "file:///p15-e03-unused-collision.cherri"
	const code = "let first=1\nlet second=2\nshow(first)\n"
	svc := NewService(nil)
	svc.OpenDocument(uri, 1, code)
	p15E03Healthy(t, svc, uri)
	if updated, edits, err := svc.Rename(uri, 1, 5, "second"); err == nil || updated != "" || len(edits) != 0 {
		t.Fatalf("unused same-scope duplicate accepted: %v %+v %q", err, edits, updated)
	}
}

func TestP15E03ParameterLabelAndScopeControls(t *testing.T) {
	t.Run("rename_from_own_label", func(t *testing.T) {
		const uri = "file:///p15-e03-label-origin.cherri"
		const marked = "show(greet(\u00abperson\u00bb: \"x\"))\nfunction greet(\u00abperson\u00bb: Text) -> Text { return \u00abperson\u00bb }\n"
		code, expected, offsets := p15E03MarkedSource(marked, "person", "guest")
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		pos := p15EditorExpectedPosition(code, offsets[0])
		actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "guest")
		if err != nil {
			t.Fatal(err)
		}
		if err := p15E03RenameError(code, uri, "person", "guest", actual, expected, edits, offsets); err != nil {
			t.Fatal(err)
		}
		svc.ChangeDocument(uri, 2, actual)
		p15E03Healthy(t, svc, uri)
	})
	t.Run("parameter_external_label", func(t *testing.T) {
		const uri = "file:///p15-e03-external-label.cherri"
		const code = "function greet(_person: Text) -> Text { return _person }\nshow(greet(person: \"x\"))\n"
		const expected = "function greet(_guest: Text) -> Text { return _guest }\nshow(greet(guest: \"x\"))\n"
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		actual, edits, err := svc.Rename(uri, 1, 16, "_guest")
		if err != nil || actual != expected || len(edits) != 3 {
			t.Fatalf("internal identifier and external label differ: %v %+v %q", err, edits, actual)
		}
		for _, edit := range edits {
			if err := p15EditorSpanError(code, uri, edit.Span); err != nil {
				t.Fatal(err)
			}
		}
		svc.ChangeDocument(uri, 2, actual)
		p15E03Healthy(t, svc, uri)
	})
	t.Run("parameter_label_collision", func(t *testing.T) {
		const uri = "file:///p15-e03-parameter-collision.cherri"
		const code = "function greet(person: Text, other: Text) -> Text { return person }\nshow(greet(\"x\", other:\"y\"))\n"
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		if actual, edits, err := svc.Rename(uri, 1, 16, "_other"); err == nil || actual != "" || len(edits) != 0 {
			t.Fatalf("duplicate external label accepted: %v %+v", err, edits)
		}
	})
	t.Run("sibling_scopes_may_reuse_name", func(t *testing.T) {
		const uri = "file:///p15-e03-sibling-scopes.cherri"
		const marked = "if true { let \u00abfirst\u00bb=1; show(\u00abfirst\u00bb) }\nif true { let second=2; show(second) }\n"
		code, expected, offsets := p15E03MarkedSource(marked, "first", "second")
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		pos := p15EditorExpectedPosition(code, offsets[1])
		actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "second")
		if err != nil {
			t.Fatal(err)
		}
		if err := p15E03RenameError(code, uri, "first", "second", actual, expected, edits, offsets); err != nil {
			t.Fatal(err)
		}
		svc.ChangeDocument(uri, 2, actual)
		p15E03Healthy(t, svc, uri)
	})
	t.Run("computed_dictionary_key_is_a_reference", func(t *testing.T) {
		const uri = "file:///p15-e03-computed-key.cherri"
		const marked = "let \u00abkey\u00bb=\"item\"\nlet data={\u00abkey\u00bb: 1}\nshow(data)\n"
		code, expected, offsets := p15E03MarkedSource(marked, "key", "field")
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		pos := p15EditorExpectedPosition(code, offsets[1])
		actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "field")
		if err != nil {
			t.Fatal(err)
		}
		if err := p15E03RenameError(code, uri, "key", "field", actual, expected, edits, offsets); err != nil {
			t.Fatal(err)
		}
		svc.ChangeDocument(uri, 2, actual)
		p15E03Healthy(t, svc, uri)
	})
	t.Run("later_parameter_default_uses_renamed_owner", func(t *testing.T) {
		const uri = "file:///p15-e03-default-reference.cherri"
		const marked = "function choose(\u00abfirst\u00bb: Text = \"x\", second: Text = \u00abfirst\u00bb) -> Text { return second }\nshow(choose(\u00abfirst\u00bb: \"y\"))\n"
		code, expected, offsets := p15E03MarkedSource(marked, "first", "initial")
		svc := NewService(nil)
		svc.OpenDocument(uri, 1, code)
		p15E03Healthy(t, svc, uri)
		pos := p15EditorExpectedPosition(code, offsets[1])
		actual, edits, err := svc.Rename(uri, pos.Line, pos.Column, "initial")
		if err != nil {
			t.Fatal(err)
		}
		if err := p15E03RenameError(code, uri, "first", "initial", actual, expected, edits, offsets); err != nil {
			t.Fatal(err)
		}
		svc.ChangeDocument(uri, 2, actual)
		p15E03Healthy(t, svc, uri)
	})
}
