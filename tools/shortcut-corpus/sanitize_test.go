/*
 * Copyright (c) Cherri
 */

package main

import (
	"strings"
	"testing"
)

func TestSanitizeTextRedactsPersonalValues(t *testing.T) {
	cases := []struct {
		input string
		want  string
		class textClass
	}{
		{"jane.doe@example.com", "\uFFFDemail", classEmail},
		{"https://api.example.com/v1?key=abc", "\uFFFDurl", classURL},
		{"/Users/janedoe/Secrets/file.txt", "\uFFFDpath", classPath},
		{"C:\\Users\\janedoe\\file.txt", "\uFFFDpath", classPath},
		{"11111111-2222-3333-4444-555555555555", "\uFFFDuuid", classUUID},
		{"sk-live-abcdef0123456789abcdef", "\uFFFDsecret", classSecret},
		{"555-010-9999", "\uFFFDphone", classPhone},
		{"2026-08-25T10:00:00Z", "\uFFFDdateTime", classDateTime},
		{"call me about the project", "\uFFFDtext", classText},
	}
	for _, item := range cases {
		got, class := sanitizeText(item.input)
		if got != item.want || class != item.class {
			t.Errorf("sanitizeText(%q) = (%q,%q), want (%q,%q)", item.input, got, class, item.want, item.class)
		}
	}
}

func TestSanitizeTextPreservesSystemConstants(t *testing.T) {
	constants := []string{
		"WFTextTokenString",
		"WFSerializationType",
		"is.workflow.actions.gettext",
		"com.apple.shortcuts",
		"notion.id.CreatePageIntent",
		"toggle",
		"WFRichTextContentItem",
	}
	for _, value := range constants {
		got, class := sanitizeText(value)
		if got != value {
			t.Errorf("system constant %q was rewritten to %q", value, got)
		}
		if class != "" {
			t.Errorf("system constant %q should have empty class, got %q", value, class)
		}
	}
}

func TestNormalizeValueShapes(t *testing.T) {
	value := normalizeValue(map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string":             "hello jane.doe@example.com",
			"attachmentsByRange": map[string]any{},
		},
	})
	if value.Serialization != "WFTextTokenString" {
		t.Fatalf("expected serialization preserved, got %q", value.Serialization)
	}
	nested := value.Fields["Value"]
	if nested == nil || nested.Kind != kindDictionary {
		t.Fatalf("expected nested dict field")
	}
	text := nested.Fields["string"]
	if text == nil || text.TextClass != string(classEmail) {
		t.Fatalf("expected email placeholder inside serialized payload")
	}
	if !strings.HasPrefix(text.Constant, placeholderPrefix) {
		t.Fatalf("expected placeholder prefix")
	}
}

func TestFingerprintIgnoresIgnoredKeysAndPersonalValues(t *testing.T) {
	base := map[string]*NormalizedValue{
		"WFInput": {Kind: kindString, Constant: "\uFFFDtext", TextClass: "text"},
		"UUID":    {Kind: kindString, Constant: "\uFFFDuuid"},
	}
	other := map[string]*NormalizedValue{
		"WFInput": {Kind: kindString, Constant: "\uFFFDtext", TextClass: "text"},
	}
	if fingerprintAction("is.workflow.actions.x", base) != fingerprintAction("is.workflow.actions.x", other) {
		t.Fatal("ignored keys should not affect fingerprint")
	}

	personal := map[string]*NormalizedValue{
		"WFInput": normalizeValue("different content entirely"),
	}
	if fingerprintAction("is.workflow.actions.x", base) != fingerprintAction("is.workflow.actions.x", personal) {
		t.Fatal("free-text values with identical structure should share a fingerprint")
	}

	constant := map[string]*NormalizedValue{
		"operation": {Kind: kindString, Constant: "set"},
	}
	if fingerprintAction("is.workflow.actions.x", base) == fingerprintAction("is.workflow.actions.x", constant) {
		t.Fatal("distinct constants must produce distinct fingerprints")
	}
}
