package main

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

const debugExportMaximumRedactKeys = 64

func parseDebugExportRedactKeys(raw any) ([]string, error) {
	var values []any
	switch value := raw.(type) {
	case []any:
		values = value
	case []string:
		for _, key := range value {
			values = append(values, key)
		}
	default:
		return nil, fmt.Errorf("DEBUG_EXPORT_LITERAL: redactKeys must be a literal List<Text>")
	}
	if len(values) > debugExportMaximumRedactKeys {
		return nil, fmt.Errorf("DEBUG_EXPORT_REDACT_KEYS: at most %d keys are supported", debugExportMaximumRedactKeys)
	}
	var keys []string
	var seen = make(map[string]bool)
	for _, value := range values {
		var key, literal = value.(string)
		if !literal || key == "" || len([]rune(key)) > 128 || strings.IndexFunc(key, unicode.IsControl) >= 0 {
			return nil, fmt.Errorf("DEBUG_EXPORT_REDACT_KEYS: keys must be nonempty literal Text, at most 128 characters, without control characters")
		}
		var folded = strings.ToLower(key)
		if !seen[folded] {
			keys = append(keys, key)
			seen[folded] = true
		}
	}
	return keys, nil
}

func debugExportSensitiveKeyPattern(keys []string) string {
	// A nonempty opt-in protects common credential containers as well as the
	// exact requested fields. This pattern is applied to dictionary KEYS only.
	var protected = []string{
		"authorization", "proxy-authorization", "headers", "cookie", "cookies",
		"set-cookie", "password", "passwords", "secret", "secrets", "token",
		"access_token", "refresh_token", "id_token", "api_key", "apikey",
		"credentials", "private_key", "url", "uri", "link", "query",
		"querystring", "queryparameters",
	}
	protected = append(protected, keys...)
	for i := range protected {
		protected[i] = regexp.QuoteMeta(protected[i])
	}
	return "^(?:" + strings.Join(protected, "|") + ")$"
}

func (m *debugExportMetadata) payloadEnabled() bool {
	return !m.options.redact || len(m.options.redactKeys) > 0
}

func (m *debugExportMetadata) typedRow(status, representation string, typeValue, payload *WFTextTokenAttachment, metadata ...any) WFTextTokenAttachment {
	var details = []any{`,"representation":"` + representation + `"`}
	details = append(details, metadata...)
	return m.b.row(status, typeValue, payload, details...)
}

// The exporter observes native content items, not source-language containers.
// In particular it never guesses Boolean/Number, null/absence, or source List
// boundaries that Add to Variable did not retain.
func (m *debugExportMetadata) scalarPayload(item, actualType WFTextTokenAttachment, typeValue *WFTextTokenAttachment, nested bool, metadata ...any) WFTextTokenAttachment {
	var b = m.b
	return b.typeBranch(actualType, m.textType, func() {
		if m.options.redact {
			b.row("REDACTED_TEXT_CONTENT", typeValue, nil, metadata...)
			return
		}
		var encoded = b.base64(item)
		b.row("TEXT_PAYLOAD_ENCODED", typeValue, &encoded, metadata...)
	}, func() {
		b.typeBranch(actualType, m.numberType, func() {
			// This is the native number's displayed text, explicitly not a
			// locale-independent JSON number or reconstruction of source Bool.
			var display = b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(item)})
			var encoded = b.base64(display)
			m.typedRow("NATIVE_NUMBER_TEXT_ENCODED", "native-number-text", typeValue, &encoded, metadata...)
		}, func() {
			b.typeBranch(actualType, m.dateType, func() {
				var formatted = b.produce("format.date", map[string]any{
					"WFDate": debugDateFieldToken(item.Value), "WFDateFormatStyle": "ISO 8601", "WFISO8601IncludeTime": true,
				})
				var encoded = b.base64(formatted)
				m.typedRow("DATE_ISO8601_ENCODED", "iso8601-native-format", typeValue, &encoded, metadata...)
			}, func() {
				b.typeBranch(actualType, m.dictionaryType, func() {
					if nested {
						b.row("UNSUPPORTED_NESTED_DICTIONARY", typeValue, nil, metadata...)
						return
					}
					m.dictionaryPayload(item, typeValue, metadata...)
				}, func() {
					b.row("UNSUPPORTED_NATIVE_PAYLOAD_TYPE", typeValue, nil, metadata...)
				})
			})
		})
	})
}

func (m *debugExportMetadata) selectedPayload(item, actualType WFTextTokenAttachment, typeValue *WFTextTokenAttachment, metadata ...any) WFTextTokenAttachment {
	if !m.options.redact {
		return m.scalarPayload(item, actualType, typeValue, false, metadata...)
	}
	// Selective disclosure is explicitly opt-in and limited to Dictionary
	// diagnostics. Free text and all other top-level content remain redacted.
	return m.b.typeBranch(actualType, m.dictionaryType, func() {
		m.dictionaryPayload(item, typeValue, metadata...)
	}, func() { m.b.row("REDACTED", typeValue, nil, metadata...) })
}

func (m *debugExportMetadata) dictionaryValue(item, key WFTextTokenAttachment) WFTextTokenAttachment {
	var b = m.b
	var value = b.produce("getvalueforkey", map[string]any{
		"WFInput": item, "WFGetDictionaryValueType": "Value",
		"WFDictionaryKey": debugExportTextToken(key),
	})
	return b.numberBranch(b.count(value), 4, 0, func() {
		b.row("ABSENT_NULL_OR_EMPTY_COLLECTION", nil, nil)
	}, func() {
		b.numberBranch(b.count(value), 4, 1, func() {
			var actualType = b.kind(value)
			var typeValue *WFTextTokenAttachment
			if m.options.includeTypes {
				var encoded = b.base64(actualType)
				typeValue = &encoded
			}
			m.scalarPayload(value, actualType, typeValue, true)
		}, func() { b.row("UNSUPPORTED_MULTIPLE_NATIVE_ITEMS", nil, nil) })
	})
}

func (m *debugExportMetadata) dictionaryFieldValue(item, key WFTextTokenAttachment) WFTextTokenAttachment {
	var b = m.b
	var readable = func() WFTextTokenAttachment {
		// Native Get Dictionary Value interprets dots as key paths. A diagnostic
		// must not silently export a different field than the one enumerated.
		var dots = b.produce("text.match", map[string]any{
			"WFMatchTextPattern": "\\.", "text": debugExportTextToken(key), "WFMatchTextCaseSensitive": true,
		})
		return b.numberBranch(b.count(dots), 4, 0, func() { m.dictionaryValue(item, key) },
			func() { b.row("UNSUPPORTED_KEY_PATH", nil, nil) })
	}
	if len(m.options.redactKeys) == 0 {
		return readable()
	}
	var sensitive = b.produce("text.match", map[string]any{
		"WFMatchTextPattern": debugExportSensitiveKeyPattern(m.options.redactKeys),
		"text":               debugExportTextToken(key), "WFMatchTextCaseSensitive": false,
	})
	return b.numberBranch(b.count(sensitive), 2, 0,
		func() { b.row("REDACTED_SENSITIVE_FIELD", nil, nil) }, func() { readable() })
}

func (m *debugExportMetadata) dictionaryPayload(item WFTextTokenAttachment, typeValue *WFTextTokenAttachment, metadata ...any) WFTextTokenAttachment {
	var b = m.b
	var keys = b.produce("getvalueforkey", map[string]any{"WFInput": item, "WFGetDictionaryValueType": "All Keys"})
	var group = strings.ToUpper(nextGeneratedUUID("debug-export-dictionary"))
	b.add("repeat.each", backend.BuildRepeatEachStart(group, keys))
	var key = b.readVariable("Repeat Item")
	var keyEncoded = b.base64(key)
	var field = m.dictionaryFieldValue(item, key)
	b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(
		`{"key":{"encoding":"base64-utf8","value":"`, keyEncoded, `"},"diagnostic":`, field, "}")})
	var rows = b.produce("repeat.each", backend.BuildLoopEnd(group, ""))
	var combined = b.produce("text.combine", map[string]any{
		"text": rows, "WFTextSeparator": "Custom", "WFTextCustomSeparator": ",",
	})
	var parts = []any{
		`{"status":"DICTIONARY_FIELD_DIAGNOSTICS","payload":{"encoding":"diagnostic-fields","lossless":false,"fields":[`,
		combined, "]}",
	}
	if typeValue != nil {
		parts = append(parts, `,"nativeType":{"encoding":"base64-utf8","value":"`, *typeValue, `"}`)
	}
	parts = append(parts, metadata...)
	parts = append(parts, "}")
	return b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(parts...)})
}
