/*
 * Copyright (c) Cherri
 */

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// NormalizedValue is the privacy-safe structural form of any Shortcut
// parameter value. Personal content never survives normalization: free-text
// strings become typed placeholders, while system constants and enum-like
// tokens are preserved verbatim because they define action structure.
type NormalizedValue struct {
	Kind          string                      `json:"kind"`
	Constant      string                      `json:"constant,omitempty"`
	TextClass     string                      `json:"textClass,omitempty"`
	Serialization string                      `json:"serialization,omitempty"`
	Items         []*NormalizedValue          `json:"items,omitempty"`
	Fields        map[string]*NormalizedValue `json:"fields,omitempty"`
}

const (
	kindNull       = "null"
	kindString     = "string"
	kindNumber     = "number"
	kindBoolean    = "boolean"
	kindArray      = "array"
	kindDictionary = "dict"
)

var ignoredParameterKeys = map[string]bool{
	"UUID":             true,
	"CustomOutputName": true,
}

// normalizeValue converts raw Shortcut data into its normalized structural
// form. Numeric literals and booleans are structural and kept; strings go
// through sanitization; serialized payloads record their type.
func normalizeValue(value any) *NormalizedValue {
	switch typed := value.(type) {
	case nil:
		return &NormalizedValue{Kind: kindNull}
	case bool:
		return &NormalizedValue{Kind: kindBoolean, Constant: fmt.Sprintf("%t", typed)}
	case float64:
		return &NormalizedValue{Kind: kindNumber, Constant: formatNumber(typed)}
	case int:
		return &NormalizedValue{Kind: kindNumber, Constant: fmt.Sprintf("%d", typed)}
	case string:
		sanitized, class := sanitizeText(typed)
		return &NormalizedValue{Kind: kindString, Constant: sanitized, TextClass: string(class)}
	case []any:
		var items []*NormalizedValue
		for _, item := range typed {
			items = append(items, normalizeValue(item))
		}
		if items == nil {
			items = []*NormalizedValue{}
		}
		return &NormalizedValue{Kind: kindArray, Items: items}
	case map[string]any:
		return normalizeSerializedMap(typed)
	default:
		return &NormalizedValue{Kind: fmt.Sprintf("%T", value)}
	}
}

func normalizeSerializedMap(value map[string]any) *NormalizedValue {
	serialization, _ := value["WFSerializationType"].(string)
	normalized := &NormalizedValue{
		Kind:          kindDictionary,
		Serialization: serialization,
		Fields:        map[string]*NormalizedValue{},
	}
	for key, field := range value {
		if key == "WFSerializationType" {
			continue
		}
		normalized.Fields[key] = normalizeValue(field)
	}
	return normalized
}

func formatNumber(value float64) string {
	if value == float64(int64(value)) {
		return fmt.Sprintf("%d", int64(value))
	}
	return fmt.Sprintf("%g", value)
}

// Fingerprint identifies one unique structural action shape: identifier plus
// parameter keys plus normalized parameter structure. Identical shapes across
// files deduplicate into a single record with growing evidence.
func fingerprintAction(identifier string, parameters map[string]*NormalizedValue) string {
	canonical := struct {
		Identifier string                      `json:"identifier"`
		Parameters map[string]*NormalizedValue `json:"parameters"`
	}{Identifier: identifier, Parameters: dropIgnoredKeys(parameters)}

	encoded, err := json.Marshal(canonical)
	if err != nil {
		return ""
	}
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func dropIgnoredKeys(parameters map[string]*NormalizedValue) map[string]*NormalizedValue {
	result := make(map[string]*NormalizedValue, len(parameters))
	for key, value := range parameters {
		if ignoredParameterKeys[key] {
			continue
		}
		result[key] = value
	}
	return result
}

func sortedKeys(parameters map[string]*NormalizedValue) []string {
	keys := make([]string, 0, len(parameters))
	for key := range parameters {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// catalogEntry mirrors the subset of `cherri --actions-json` entries used for
// comparison. Only reliable metadata published by Cherri is consulted.
type catalogEntry struct {
	Name               string             `json:"name"`
	ShortcutIdentifier string             `json:"shortcutIdentifier"`
	Title              string             `json:"title"`
	Category           string             `json:"category"`
	Parameters         []catalogParameter `json:"parameters"`
	Builtin            bool               `json:"builtin"`
}

type catalogParameter struct {
	Name       string   `json:"name"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Enum       string   `json:"enum"`
	EnumValues []string `json:"enumValues"`
}

// actionCatalog is the loaded view of Cherri's machine-readable catalog.
type actionCatalog struct {
	byIdentifier map[string]*catalogEntry
	knownKeys    map[string]map[string]bool
	count        int
	source       string
}

func loadCatalogFromFile(path string) (*actionCatalog, error) {
	data, err := readFile(path)
	if err != nil {
		return nil, fmt.Errorf("catalog file %s: %w", path, err)
	}
	return parseCatalog(data, path)
}

func parseCatalog(data []byte, source string) (*actionCatalog, error) {
	var response struct {
		OK      bool           `json:"ok"`
		Error   string         `json:"error"`
		Actions []catalogEntry `json:"actions"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("catalog %s is not valid JSON: %w", source, err)
	}
	if !response.OK {
		return nil, fmt.Errorf("catalog %s reported error: %s", source, response.Error)
	}

	catalog := &actionCatalog{
		byIdentifier: map[string]*catalogEntry{},
		knownKeys:    map[string]map[string]bool{},
		source:       filepath.Base(source),
	}
	for index := range response.Actions {
		entry := &response.Actions[index]
		if entry.ShortcutIdentifier == "" {
			continue
		}
		catalog.byIdentifier[entry.ShortcutIdentifier] = entry
		keys, exists := catalog.knownKeys[entry.ShortcutIdentifier]
		if !exists {
			keys = map[string]bool{}
			catalog.knownKeys[entry.ShortcutIdentifier] = keys
		}
		for _, parameter := range entry.Parameters {
			key := parameter.Key
			if key == "" {
				key = parameter.Name
			}
			if key != "" {
				keys[key] = true
			}
		}
	}
	catalog.count = len(catalog.byIdentifier)
	return catalog, nil
}

// knownParameterKeys returns the union of parameter keys Cherri models for an
// identifier across every definition sharing it (several Cherri actions can
// target one identifier), or nil when the identifier is unknown.
func (catalog *actionCatalog) knownParameterKeys(identifier string) map[string]bool {
	return catalog.knownKeys[identifier]
}

func (catalog *actionCatalog) known(identifier string) bool {
	_, found := catalog.byIdentifier[identifier]
	return found
}

// classification constants represent analyzer verdicts. Observed data alone
// never promotes a record past NEEDS_REVIEW into production use.
const (
	classKnown                   = "KNOWN"
	classNew                     = "NEW"
	classVariant                 = "VARIANT"
	classThirdParty              = "THIRD_PARTY"
	classUnknown                 = "UNKNOWN"
	classNeedsReview             = "NEEDS_REVIEW"
	classSafeCandidate           = "SAFE_CANDIDATE"
	classCustomImplementationReq = "CUSTOM_IMPLEMENTATION_REQUIRED"
)

// classify assigns a classification to a normalized action shape using the
// catalog and previously observed forms of the same identifier.
func classify(record *ActionRecord, catalog *actionCatalog) {
	identifier := record.Identifier

	if !strings.HasPrefix(identifier, "is.workflow.") && strings.Contains(identifier, ".") {
		record.Classification = classThirdParty
		record.Evidence.Status = "observed"
		return
	}

	knownKeys := catalog.knownParameterKeys(identifier)
	observed := observedKeySet(record.Parameters)

	switch {
	case knownKeys == nil:
		record.Classification = classUnknown
		record.Notes = append(record.Notes, "identifier not present in Cherri catalog")
	case keySubset(observed, unionSets(knownKeys, ignoredParameterKeys)):
		record.Classification = classKnown
		record.Evidence.Status = "observed"
	default:
		record.Classification = classVariant
		record.Notes = append(record.Notes, "parameter shape differs from Cherri definition")
	}
}

// refineClassification applies candidate-safety analysis to records that are
// new to Cherri. It never mutates KNOWN records.
func refineClassification(record *ActionRecord) {
	if record.Classification != classUnknown {
		return
	}
	if requiresCustomImplementation(record) {
		record.Classification = classCustomImplementationReq
		return
	}
	record.Classification = classSafeCandidate
	record.Notes = append(record.Notes, "declarative shape suitable for generated candidate")
}

var complexSerializationTypes = map[string]bool{
	"WFQuantityFieldValue":            true,
	"WFContentPredicateTableTemplate": true,
	"WFAppIntentDescriptor":           true,
}

func requiresCustomImplementation(record *ActionRecord) bool {
	for _, value := range record.Parameters {
		if serializationNeedsCustom(value) {
			return true
		}
	}
	return false
}

func serializationNeedsCustom(value *NormalizedValue) bool {
	if value == nil {
		return false
	}
	if complexSerializationTypes[value.Serialization] {
		return true
	}
	if value.Serialization == "WFAppIntentDescriptor" {
		return true
	}
	for _, child := range value.Items {
		if serializationNeedsCustom(child) {
			return true
		}
	}
	for _, child := range value.Fields {
		if serializationNeedsCustom(child) {
			return true
		}
	}
	return false
}

func observedKeySet(parameters map[string]*NormalizedValue) map[string]bool {
	keys := map[string]bool{}
	for key := range parameters {
		keys[key] = true
	}
	return keys
}

func keySubset(observed, allowed map[string]bool) bool {
	for key := range observed {
		if !allowed[key] {
			return false
		}
	}
	return true
}

func unionSets(a, b map[string]bool) map[string]bool {
	union := make(map[string]bool, len(a)+len(b))
	for key := range a {
		union[key] = true
	}
	for key := range b {
		union[key] = true
	}
	return union
}
