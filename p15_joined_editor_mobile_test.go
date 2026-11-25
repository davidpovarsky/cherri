package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15BRG27MobileCompletionCase struct {
	name, marked, label, replaced, inserted, expected, suffix string
	kind                                                      service.CompletionItemKind
}

func p15BRG27MobileCompletionCases() []p15BRG27MobileCompletionCase {
	const declarations = "enum Color { Red = \"native-red\", Blue = \"native-blue\" }\nfunction paint(color: Color, note: Text = \"\") -> Text { return note }\n"
	return []p15BRG27MobileCompletionCase{
		{"parameter_prefix", "resizeImage(Clipboard, width: 800, he|", "height", "he", "height: ", "resizeImage(Clipboard, width: 800, height: ", "600)\n", service.CompletionKindParameter},
		{"parameter_middle_existing_colon", "resizeImage(Clipboard, width: 800, he|ighx: 600)\n", "height", "heighx: ", "height: ", "resizeImage(Clipboard, width: 800, height: 600)\n", "", service.CompletionKindParameter},
		{"enum_member_suffix", "setBackgroundSound(.Ra|n)\n", "Rain", "Ran", "Rain", "setBackgroundSound(.Rain)\n", "", service.CompletionKindEnum},
		{"enum_implicit_dot", "setBackgroundSound(Ra|)\n", "Rain", "Ra", ".Rain", "setBackgroundSound(.Rain)\n", "", service.CompletionKindEnum},
		{"qualified_enum", declarations + "paint(Color.Re|x)\n", "Red", "Rex", "Red", declarations + "paint(Color.Red)\n", "", service.CompletionKindEnum},
		{"hebrew_interpolation", "let סכום = 7\nshow(f\"🙂{סכ|ו}\")\n", "סכום", "סכו", "סכום", "let סכום = 7\nshow(f\"🙂{סכום}\")\n", "", service.CompletionKindVariable},
	}
}
func p15BRG27MobileObserve(t *testing.T, tc p15BRG27MobileCompletionCase) (protocol.CompleteResponse, string) {
	t.Helper()
	cursor := strings.Index(tc.marked, "|")
	if cursor < 0 {
		t.Fatal("missing authored cursor")
	}
	text := strings.Replace(tc.marked, "|", "", 1)
	position := p15E06MobilePosition(text, cursor)
	actual := completeForMobile(text, position.Line+1, position.Character+1)
	// Retain actual wire bytes, rather than only checking the Go field before
	// marshaling. Native C ABI and Swift application run separately in XCTest.
	raw, err := json.Marshal(actual)
	if err != nil {
		t.Fatal(err)
	}
	var wire protocol.CompleteResponse
	if err = json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	t.Logf("BRG27_MOBILE_WIRE %s", raw)
	return wire, text
}
func p15BRG27MobileCompletionError(tc p15BRG27MobileCompletionCase, text string, response protocol.CompleteResponse) error {
	if response.URI != "file:///mobile.cherri" || response.Version != 1 || response.LanguageVersion != "2.0" || len(response.SchemaFingerprint) != 64 {
		return fmt.Errorf("wrong completion source identity")
	}
	var selected *protocol.CompletionItem
	for i := range response.Items {
		item := &response.Items[i]
		if item.Label == tc.label && item.Kind == int(tc.kind) {
			if selected != nil {
				return fmt.Errorf("duplicate contextual candidate")
			}
			selected = item
		}
	}
	if selected == nil || selected.TextEdit == nil {
		return fmt.Errorf("actual mobile JSON lost shared completion edit")
	}
	start := strings.LastIndex(text, tc.replaced)
	if start < 0 {
		return fmt.Errorf("authored replacement absent")
	}
	want := protocol.TextEditItem{Range: p15E06MobileRange(text, start, len(tc.replaced)), NewText: tc.inserted}
	if !reflect.DeepEqual(*selected.TextEdit, want) {
		return fmt.Errorf("actual mobile edit differs: got=%+v want=%+v", selected.TextEdit, want)
	}
	if text[:start]+selected.TextEdit.NewText+text[start+len(tc.replaced):] != tc.expected {
		return fmt.Errorf("complete independently authored source differs")
	}
	return nil
}
func TestBackendRecovery27MobileCompletionEdits(t *testing.T) {
	for _, tc := range p15BRG27MobileCompletionCases() {
		t.Run(tc.name, func(t *testing.T) {
			wire, text := p15BRG27MobileObserve(t, tc)
			if err := p15BRG27MobileCompletionError(tc, text, wire); err != nil {
				t.Fatal(err)
			}
			analysis := analyzeVersionedForMobile("file:///brg27-mobile-"+tc.name+".cherri", 1, tc.expected+tc.suffix)
			if !analysis.Valid || len(analysis.Diagnostics) != 0 {
				t.Fatal("actual selected edit did not recover a valid source", analysis)
			}
		})
	}
}
func TestBackendRecovery27MobileEditOracleRejectsChanges(t *testing.T) {
	tc := p15BRG27MobileCompletionCases()[5]
	actual, text := p15BRG27MobileObserve(t, tc)
	if err := p15BRG27MobileCompletionError(tc, text, actual); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing_edit", "rune_column", "wrong_end", "foreign_line", "wrong_text", "wrong_kind", "wrong_uri", "wrong_version", "missing_candidate", "duplicate_candidate"} {
		t.Run(name, func(t *testing.T) {
			raw, err := json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			var changed protocol.CompleteResponse
			if err = json.Unmarshal(raw, &changed); err != nil {
				t.Fatal(err)
			}
			var index int = -1
			for i := range changed.Items {
				if changed.Items[i].Label == tc.label && changed.Items[i].Kind == int(tc.kind) {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("lost original item")
			}
			item := &changed.Items[index]
			switch name {
			case "missing_edit":
				item.TextEdit = nil
			case "rune_column":
				item.TextEdit.Range.Start.Character--
			case "wrong_end":
				item.TextEdit.Range.End.Character++
			case "foreign_line":
				item.TextEdit.Range.Start.Line = 0
			case "wrong_text":
				item.TextEdit.NewText = "foreign"
			case "wrong_kind":
				item.Kind = int(service.CompletionKindParameter)
			case "wrong_uri":
				changed.URI = "file:///foreign.cherri"
			case "wrong_version":
				changed.Version = 2
			case "missing_candidate":
				changed.Items = append(changed.Items[:index], changed.Items[index+1:]...)
			case "duplicate_candidate":
				changed.Items = append(changed.Items, *item)
			}
			if p15BRG27MobileCompletionError(tc, text, changed) == nil {
				t.Fatal("accepted actual mobile wire corruption", name)
			}
		})
	}
}
