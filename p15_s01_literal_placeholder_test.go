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

// p15S01LiteralCarrierReference checks exact public source bytes and a real
// earlier plain-string Text producer, independently of the lowering encoder.
func p15S01LiteralCarrierReference(actions []map[string]any, literal string) (map[string]any, error) {
	var carrier map[string]any
	position := -1
	for index, action := range actions {
		p := p15Param(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && p["WFTextActionText"] == literal {
			if carrier != nil {
				return nil, fmt.Errorf("duplicate literal Text carrier")
			}
			carrier, position = p, index
		}
	}
	if carrier == nil {
		return nil, fmt.Errorf("literal UTF-8 bytes lost their plain Text carrier")
	}
	uuid, ok := carrier["UUID"].(string)
	if !ok || uuid == "" || carrier["CustomOutputName"] != "Text" {
		return nil, fmt.Errorf("literal Text carrier lost its exact native identity")
	}
	uses := 0
	var inspect func(any, int) error
	inspect = func(value any, index int) error {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" && v["OutputUUID"] == uuid {
				if index <= position || v["OutputName"] != "Text" || len(v) != 3 {
					return fmt.Errorf("literal carrier reference changed or precedes its producer")
				}
				uses++
			}
			for _, child := range v {
				if err := inspect(child, index); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := inspect(child, index); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for index, action := range actions {
		if err := inspect(action, index); err != nil {
			return nil, err
		}
	}
	if uses != 1 {
		return nil, fmt.Errorf("literal carrier has %d actual consumers", uses)
	}
	return map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "Text"}, nil
}

func TestP15S01LiteralCarrierRejectsCausalMutations(t *testing.T) {
	tc := p15UnicodeTextCases()[1]
	original := p15CompiledActions(t, p15UnicodeTextSource(tc, "immutable"))
	validate := func(actions []map[string]any) error {
		value, err := p15UnicodeTextValue(actions, "immutable")
		if err != nil {
			return err
		}
		refs, err := p15UnicodeExpectedRefs(actions, tc)
		if err != nil {
			return err
		}
		return p15UnicodeTextError(value, tc, refs)
	}
	if err := validate(original); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"drop_literal", "change_literal_bytes", "wrong_kind", "detached_uuid", "duplicate_carrier", "token_instead_of_plain", "carrier_after_consumer"} {
		t.Run(fault, func(t *testing.T) {
			actions := make([]map[string]any, len(original))
			for i, action := range original {
				actions[i] = p15LiteralClone(t, action).(map[string]any)
			}
			if err := validate(actions); err != nil {
				t.Fatal("unchanged actual clone rejected", err)
			}
			index := -1
			for i, action := range actions {
				if p15Param(action)["WFTextActionText"] == tc.literalPrefix {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("missing literal carrier")
			}
			p := p15Param(actions[index])
			switch fault {
			case "drop_literal":
				p["WFTextActionText"] = "🙂"
			case "change_literal_bytes":
				p["WFTextActionText"] = "\uFFFC🙃"
			case "wrong_kind":
				actions[index]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
			case "detached_uuid":
				p["UUID"] = "11111111-1111-1111-1111-111111111111"
			case "duplicate_carrier":
				actions = append(actions, p15LiteralClone(t, actions[index]).(map[string]any))
			case "token_instead_of_plain":
				p["WFTextActionText"] = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": tc.literalPrefix}}
			case "carrier_after_consumer":
				carrier := actions[index]
				actions = append(append(actions[:index:index], actions[index+1:]...), carrier)
			}
			if err := validate(actions); err == nil {
				t.Fatal("accepted literal carrier corruption", fault)
			}
		})
	}
}
