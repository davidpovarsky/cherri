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

var ignoredParameterKeys = map[string]bool{"UUID": true, "CustomOutputName": true}

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
	case int64:
		return &NormalizedValue{Kind: kindNumber, Constant: fmt.Sprintf("%d", typed)}
	case uint64:
		return &NormalizedValue{Kind: kindNumber, Constant: fmt.Sprintf("%d", typed)}
	case string:
		sanitized, class := sanitizeText(typed)
		return &NormalizedValue{Kind: kindString, Constant: sanitized, TextClass: string(class)}
	case []any:
		var items []*NormalizedValue
		for _, item := range typed { items = append(items, normalizeValue(item)) }
		if items == nil { items = []*NormalizedValue{} }
		return &NormalizedValue{Kind: kindArray, Items: items}
	case map[string]any:
		return normalizeSerializedMap(typed)
	default:
		return &NormalizedValue{Kind: fmt.Sprintf("%T", value)}
	}
}

func normalizeSerializedMap(value map[string]any) *NormalizedValue {
	serialization, _ := value["WFSerializationType"].(string)
	normalized := &NormalizedValue{Kind: kindDictionary, Serialization: serialization, Fields: map[string]*NormalizedValue{}}
	for key, field := range value {
		if key == "WFSerializationType" { continue }
		normalized.Fields[key] = normalizeValue(field)
	}
	return normalized
}

func formatNumber(value float64) string {
	if value == float64(int64(value)) { return fmt.Sprintf("%d", int64(value)) }
	return fmt.Sprintf("%g", value)
}

type schemaValue struct {
	Kind          string                 `json:"kind"`
	Serialization string                 `json:"serialization,omitempty"`
	Items         []schemaValue          `json:"items,omitempty"`
	Fields        map[string]schemaValue `json:"fields,omitempty"`
}

func valueSchema(value *NormalizedValue) schemaValue {
	if value == nil { return schemaValue{Kind: kindNull} }
	result := schemaValue{Kind: value.Kind, Serialization: value.Serialization}
	if len(value.Fields) != 0 {
		result.Fields = make(map[string]schemaValue, len(value.Fields))
		for key, child := range value.Fields { result.Fields[key] = valueSchema(child) }
	}
	if len(value.Items) != 0 {
		unique := map[string]schemaValue{}
		for _, child := range value.Items {
			schema := valueSchema(child)
			encoded, _ := json.Marshal(schema)
			unique[string(encoded)] = schema
		}
		keys := make([]string, 0, len(unique))
		for key := range unique { keys = append(keys, key) }
		sort.Strings(keys)
		for _, key := range keys { result.Items = append(result.Items, unique[key]) }
	}
	return result
}

func fingerprintAction(identifier string, parameters map[string]*NormalizedValue) string {
	trimmed := dropIgnoredKeys(parameters)
	schema := make(map[string]schemaValue, len(trimmed))
	for key, value := range trimmed { schema[key] = valueSchema(value) }
	canonical := struct {
		Identifier string                 `json:"identifier"`
		Parameters map[string]schemaValue `json:"parameters"`
	}{Identifier: identifier, Parameters: schema}
	encoded, err := json.Marshal(canonical)
	if err != nil { return "" }
	digest := sha256.Sum256(encoded)
	return hex.EncodeToString(digest[:])
}

func collectValueObservations(parameters map[string]*NormalizedValue) map[string][]string {
	observations := map[string][]string{}
	for key, value := range dropIgnoredKeys(parameters) { collectValueObservation(key, value, observations) }
	if len(observations) == 0 { return nil }
	return observations
}

func collectValueObservation(path string, value *NormalizedValue, observations map[string][]string) {
	if value == nil { return }
	if value.Kind == kindString && value.Constant != "" && !strings.HasPrefix(value.Constant, placeholderPrefix) {
		observations[path] = boundedUniqueStrings(append(observations[path], value.Constant), 32)
	}
	for _, child := range value.Items { collectValueObservation(path+"[]", child, observations) }
	for key, child := range value.Fields { collectValueObservation(path+"."+key, child, observations) }
}

func mergeValueObservations(record *ActionRecord, incoming map[string][]string) {
	if len(incoming) == 0 { return }
	if record.ValueObservations == nil { record.ValueObservations = map[string][]string{} }
	for path, values := range incoming { record.ValueObservations[path] = boundedUniqueStrings(append(record.ValueObservations[path], values...), 32) }
}

func dropIgnoredKeys(parameters map[string]*NormalizedValue) map[string]*NormalizedValue {
	result := make(map[string]*NormalizedValue, len(parameters))
	for key, value := range parameters { if !ignoredParameterKeys[key] { result[key] = value } }
	return result
}

func sortedKeys(parameters map[string]*NormalizedValue) []string {
	keys := make([]string, 0, len(parameters))
	for key := range parameters { keys = append(keys, key) }
	sort.Strings(keys)
	return keys
}

type catalogEntry struct {
	Name               string             `json:"name"`
	ShortcutIdentifier string             `json:"shortcutIdentifier"`
	Title              string             `json:"title"`
	Description        string             `json:"description"`
	Category           string             `json:"category"`
	Subcategory        string             `json:"subcategory"`
	Parameters         []catalogParameter `json:"parameters"`
	EmittedKeys        []string           `json:"emittedKeys"`
	OutputType         string             `json:"outputType"`
	MacOnly            bool               `json:"macOnly"`
	NonMacOnly         bool               `json:"nonMacOnly"`
	MinVersion         float64            `json:"minVersion"`
	MaxVersion         float64            `json:"maxVersion"`
	Builtin            bool               `json:"builtin"`
	Custom             bool               `json:"custom"`
	CompilerConstruct  bool               `json:"compilerConstruct"`
	AppIntent          *catalogAppIntent  `json:"appIntent"`
}

type catalogParameter struct {
	Name       string   `json:"name"`
	Key        string   `json:"key"`
	Type       string   `json:"type"`
	Optional   bool     `json:"optional"`
	Infinite   bool     `json:"infinite"`
	Reference  bool     `json:"reference"`
	Literal    bool     `json:"literal"`
	Enum       string   `json:"enum"`
	EnumValues []string `json:"enumValues"`
	Default    string   `json:"default"`
}

type catalogAppIntent struct {
	Name                    string `json:"name"`
	BundleIdentifier        string `json:"bundleIdentifier"`
	AppIntentIdentifier     string `json:"appIntentIdentifier"`
	TeamIdentifier          string `json:"teamIdentifier"`
	RequiresAppInstallation *bool  `json:"requiresAppInstallation"`
}

type actionCatalog struct {
	byIdentifier map[string][]*catalogEntry
	knownKeys    map[string]map[string]bool
	parameters   map[string]map[string][]catalogParameter
	count        int
	source       string
	digest       string
}

func loadCatalogFromFile(path string) (*actionCatalog, error) {
	data, err := readFile(path)
	if err != nil { return nil, fmt.Errorf("catalog file %s: %w", path, err) }
	return parseCatalog(data, path)
}

func parseCatalog(data []byte, source string) (*actionCatalog, error) {
	var response struct { OK bool `json:"ok"`; Error string `json:"error"`; Actions []catalogEntry `json:"actions"` }
	if err := json.Unmarshal(data, &response); err != nil { return nil, fmt.Errorf("catalog %s is not valid JSON: %w", source, err) }
	if !response.OK { return nil, fmt.Errorf("catalog %s reported error: %s", source, response.Error) }
	catalog := &actionCatalog{byIdentifier: map[string][]*catalogEntry{}, knownKeys: map[string]map[string]bool{}, parameters: map[string]map[string][]catalogParameter{}, source: filepath.Base(source), digest: hashBytes(data)}
	for index := range response.Actions {
		entry := &response.Actions[index]
		identifier := entry.ShortcutIdentifier
		if identifier == "" { continue }
		catalog.byIdentifier[identifier] = append(catalog.byIdentifier[identifier], entry)
		keys := catalog.knownKeys[identifier]
		if keys == nil { keys = map[string]bool{}; catalog.knownKeys[identifier] = keys }
		params := catalog.parameters[identifier]
		if params == nil { params = map[string][]catalogParameter{}; catalog.parameters[identifier] = params }
		for _, parameter := range entry.Parameters {
			key := parameter.Key; if key == "" { key = parameter.Name }
			if key != "" { keys[key] = true; params[key] = append(params[key], parameter) }
		}
		for _, key := range entry.EmittedKeys { if key != "" { keys[key] = true } }
	}
	catalog.count = len(catalog.byIdentifier)
	return catalog, nil
}

func (catalog *actionCatalog) knownParameterKeys(identifier string) map[string]bool { return catalog.knownKeys[identifier] }
func (catalog *actionCatalog) known(identifier string) bool { return len(catalog.byIdentifier[identifier]) != 0 }
func (catalog *actionCatalog) enumValues(identifier, key string) (map[string]bool, bool) {
	values := map[string]bool{}; hasEnum := false
	for _, parameter := range catalog.parameters[identifier][key] {
		if parameter.Enum == "" && len(parameter.EnumValues) == 0 { continue }
		hasEnum = true
		for _, value := range parameter.EnumValues { values[value] = true }
	}
	return values, hasEnum
}

const (
	classKnown = "KNOWN"
	classNew = "NEW"
	classVariant = "VARIANT"
	classThirdParty = "THIRD_PARTY"
	classUnknown = "UNKNOWN"
	classNeedsReview = "NEEDS_REVIEW"
	classSafeCandidate = "SAFE_CANDIDATE"
	classCustomImplementationReq = "CUSTOM_IMPLEMENTATION_REQUIRED"
)

func classify(record *ActionRecord, catalog *actionCatalog) {
	record.Classification = ""; record.Notes = nil
	identifier := record.Identifier
	if !strings.HasPrefix(identifier, "is.workflow.") && strings.Contains(identifier, ".") { record.Classification = classThirdParty; record.Evidence.Status = "observed"; return }
	knownKeys := catalog.knownParameterKeys(identifier)
	if knownKeys == nil { record.Classification = classUnknown; record.Notes = append(record.Notes, "identifier not present in Cherri catalog"); return }
	unknown := differenceKeys(observedKeySet(record.Parameters), unionSets(knownKeys, ignoredParameterKeys))
	if len(unknown) != 0 { record.Classification = classVariant; record.Notes = append(record.Notes, "unmodeled parameter keys: "+strings.Join(unknown, ", ")); return }
	for key, values := range record.ValueObservations {
		if strings.Contains(key, ".") || strings.Contains(key, "[]") { continue }
		allowed, hasEnum := catalog.enumValues(identifier, key)
		if !hasEnum || len(allowed) == 0 { continue }
		for _, value := range values {
			if !allowed[value] { record.Classification = classVariant; record.Notes = append(record.Notes, fmt.Sprintf("observed system constant %q for %s is outside Cherri enum", value, key)); return }
		}
	}
	record.Classification = classKnown; record.Evidence.Status = "observed"
}

func refineClassification(record *ActionRecord) {
	if record.Classification != classUnknown { return }
	if requiresCustomImplementation(record) { record.Classification = classCustomImplementationReq; return }
	record.Classification = classSafeCandidate
	record.Notes = append(record.Notes, "declarative schema suitable for generated candidate")
}

var complexSerializationTypes = map[string]bool{"WFQuantityFieldValue": true, "WFContentPredicateTableTemplate": true, "WFAppIntentDescriptor": true}
var complexParameterKeys = map[string]bool{"AppIntentDescriptor": true}

func requiresCustomImplementation(record *ActionRecord) bool {
	for key, value := range record.Parameters { if complexParameterKeys[key] || serializationNeedsCustom(value) { return true } }
	return false
}
func serializationNeedsCustom(value *NormalizedValue) bool {
	if value == nil { return false }
	if complexSerializationTypes[value.Serialization] { return true }
	if _, found := value.Fields["AppIntentDescriptor"]; found { return true }
	for _, child := range value.Items { if serializationNeedsCustom(child) { return true } }
	for _, child := range value.Fields { if serializationNeedsCustom(child) { return true } }
	return false
}
func observedKeySet(parameters map[string]*NormalizedValue) map[string]bool { keys := map[string]bool{}; for key := range parameters { keys[key] = true }; return keys }
func keySubset(observed, allowed map[string]bool) bool { return len(differenceKeys(observed, allowed)) == 0 }
func differenceKeys(observed, allowed map[string]bool) []string { var result []string; for key := range observed { if !allowed[key] { result = append(result, key) } }; sort.Strings(result); return result }
func unionSets(a, b map[string]bool) map[string]bool { union := make(map[string]bool, len(a)+len(b)); for key := range a { union[key] = true }; for key := range b { union[key] = true }; return union }
