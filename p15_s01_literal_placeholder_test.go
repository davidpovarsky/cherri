package main

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

// The real K Apple coverage run 44653155762 lost the authored literal U+FFFC
// and later variable values. A token template may not leave a replacement
// character without its own native attachment. This checks serialized graph
// safety only; the unchanged runtime fixture still requires actual Apple replay.
func p15S01CompletePlaceholderCoverage(value any) error {
	switch v := value.(type) {
	case map[string]any:
		if v["WFSerializationType"] == "WFTextTokenString" {
			body, ok := v["Value"].(map[string]any)
			if !ok {
				return fmt.Errorf("token string lost its native body")
			}
			text, ok := body["string"].(string)
			if !ok {
				return fmt.Errorf("token string lost its exact text")
			}
			attachments, _ := body["attachmentsByRange"].(map[string]any)
			expected := make(map[string]bool)
			offset := 0
			for _, r := range text {
				if r == '\uFFFC' {
					expected["{"+strconv.Itoa(offset)+", 1}"] = true
				}
				offset++
				if r > 0xffff {
					offset++
				}
			}
			if len(attachments) != len(expected) {
				return fmt.Errorf("native template has %d placeholders but %d attachments", len(expected), len(attachments))
			}
			for key, attachment := range attachments {
				if !expected[key] {
					return fmt.Errorf("native attachment misses its UTF-16 placeholder: %s", key)
				}
				descriptor, ok := attachment.(map[string]any)
				if !ok || descriptor["Type"] == nil {
					return fmt.Errorf("native attachment lacks a real descriptor: %s", key)
				}
			}
		}
		for _, item := range v {
			if err := p15S01CompletePlaceholderCoverage(item); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := p15S01CompletePlaceholderCoverage(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func TestP15S01CompiledTokenTemplatesCoverEveryPlaceholder(t *testing.T) {
	tc := p15UnicodeTextCases()[1]
	for _, route := range []string{"argument", "immutable", "mutable", "assignment", "list", "dictionary"} {
		t.Run(route, func(t *testing.T) {
			actions := p15CompiledActions(t, p15UnicodeTextSource(tc, route))
			for index, action := range actions {
				if err := p15S01CompletePlaceholderCoverage(action); err != nil {
					t.Fatalf("action %d has the native literal-character collision: %v", index, err)
				}
			}
		})
	}
}

func TestP15S01PlaceholderCoverageRejectsCausalMutations(t *testing.T) {
	tc := p15UnicodeTextCases()[0]
	actions := p15CompiledActions(t, p15UnicodeTextSource(tc, "immutable"))
	original, err := p15UnicodeTextValue(actions, "immutable")
	if err != nil {
		t.Fatal(err)
	}
	if err := p15S01CompletePlaceholderCoverage(original); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"unbound_prefix", "missing_attachment", "wrong_range", "extra_attachment", "missing_descriptor"} {
		t.Run(fault, func(t *testing.T) {
			value := p15LiteralClone(t, original).(map[string]any)
			if err := p15S01CompletePlaceholderCoverage(value); err != nil {
				t.Fatal("unchanged actual graph clone rejected", err)
			}
			body := value["Value"].(map[string]any)
			attachments := body["attachmentsByRange"].(map[string]any)
			first := "{15, 1}"
			switch fault {
			case "unbound_prefix":
				body["string"] = "\uFFFC" + body["string"].(string)
			case "missing_attachment":
				delete(attachments, first)
			case "wrong_range":
				attachments["{14, 1}"] = attachments[first]
				delete(attachments, first)
			case "extra_attachment":
				attachments["{0, 1}"] = attachments[first]
			case "missing_descriptor":
				attachments[first] = map[string]any{}
			}
			if err := p15S01CompletePlaceholderCoverage(value); err == nil {
				t.Fatal("accepted native template corruption", fault)
			}
		})
	}
	// A plain native string is not a serialized attachment template.
	if err := p15S01CompletePlaceholderCoverage("\uFFFC" + strings.Repeat("🙂", 2)); err != nil {
		t.Fatal(err)
	}
}
