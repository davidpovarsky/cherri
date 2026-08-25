/*
 * Copyright (c) Cherri
 */

package main

import (
	"regexp"
	"strings"
)

// The repository is public: raw Shortcut values must never reach committed
// state or reports. Sanitization replaces every free-text value with a typed
// placeholder while preserving the system constants and enum-like tokens that
// are genuinely necessary to recognize action structure.

type textClass string

const (
	classText     textClass = "text"
	classEmail    textClass = "email"
	classPhone    textClass = "phone"
	classURL      textClass = "url"
	classPath     textClass = "path"
	classSecret   textClass = "secret"
	classNumeric  textClass = "numericString"
	classUUID     textClass = "uuid"
	classDateTime textClass = "dateTime"
)

var (
	emailPattern    = regexp.MustCompile(`(?i)\b[A-Z0-9._%+\-]+@[A-Z0-9.\-]+\.[A-Z]{2,}\b`)
	phonePattern    = regexp.MustCompile(`(?:\+?\d[\d\s().-]{7,}\d)`)
	urlPattern      = regexp.MustCompile(`(?i)\b(?:https?|ftp)://\S+`)
	uuidPattern     = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
	dateTimePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}([T ]\d{2}:\d{2}(:\d{2})?)?`)
	pathPattern     = regexp.MustCompile(`(?:^|[A-Za-z]:)(?:/|\\)(?:[^/\\]+(?:/|\\))*[^/\\]*`)
	secretPattern   = regexp.MustCompile(`^[A-Za-z0-9_\-]{20,}$`)
	numericPattern  = regexp.MustCompile(`^-?\d+(?:\.\d+)?$`)

	// bundleIdentifierPattern matches dotted identifiers such as
	// com.apple.shortcuts or is.workflow.actions.gettext.
	bundleIdentifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:\.[A-Za-z0-9_-]+)+$`)
	// wfSerializationPattern matches Apple serialization type names.
	wfSerializationPattern = regexp.MustCompile(`^WF[A-Za-z0-9_]*$`)

	secretKeyHint = regexp.MustCompile(`(?i)(token|secret|password|apikey|api_key|authorization|bearer|credential)`)

	// curated system control values that appear as bare lowercase words.
	curatedConstants = map[string]bool{
		"toggle": true, "set": true, "get": true, "on": true, "off": true,
		"asc": true, "desc": true, "always": true, "never": true,
		"askwhenrun": true, "and": true, "or": true,
	}
)

const placeholderPrefix = "\uFFFD"

// sanitizeText classifies and redacts a free-text string. System constants are
// preserved verbatim; everything else becomes a typed placeholder so no
// personal value can leak into normalized records or fingerprints.
func sanitizeText(value string) (string, textClass) {
	if value == "" {
		return "", ""
	}
	switch {
	case uuidPattern.MatchString(value):
		return placeholderPrefix + string(classUUID), classUUID
	case emailPattern.MatchString(value):
		return placeholderPrefix + string(classEmail), classEmail
	case urlPattern.MatchString(value):
		return placeholderPrefix + string(classURL), classURL
	case dateTimePattern.MatchString(value):
		return placeholderPrefix + string(classDateTime), classDateTime
	case pathPattern.MatchString(value) && looksLikePath(value):
		return placeholderPrefix + string(classPath), classPath
	case numericPattern.MatchString(value):
		return placeholderPrefix + string(classNumeric), classNumeric
	}
	if isSystemConstant(value) {
		return value, ""
	}
	switch {
	case secretPattern.MatchString(value):
		return placeholderPrefix + string(classSecret), classSecret
	}
	if phonePattern.MatchString(value) && strings.ContainsAny(value, "+()-") {
		return placeholderPrefix + string(classPhone), classPhone
	}
	if secretKeyHint.MatchString(value) {
		return placeholderPrefix + string(classSecret), classSecret
	}
	return placeholderPrefix + string(classText), classText
}

func looksLikePath(value string) bool {
	if !strings.ContainsAny(value, "/\\") {
		return false
	}
	return pathPattern.MatchString(value)
}

// isSystemConstant decides whether a raw string value is a structural constant
// worth preserving exactly (serialization types, identifiers, curated enums).
func isSystemConstant(value string) bool {
	if wfSerializationPattern.MatchString(value) || bundleIdentifierPattern.MatchString(value) {
		return true
	}
	return curatedConstants[strings.ToLower(value)] && len(value) <= 12
}

// sanitizeAttachmentKey normalizes attachment range keys ("{0, 1}") which
// shift with unrelated text edits; their order still encodes structure.
func sanitizeAttachmentKey(key string) string {
	sanitized, _ := sanitizeText(strings.Trim(key, "{} "))
	return "{" + sanitized + "}"
}
