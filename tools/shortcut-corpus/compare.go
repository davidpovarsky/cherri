/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"howett.net/plist"
)

// The structural comparator compares two Shortcut documents by meaningful
// semantics instead of bytes: action order, identifiers, parameter keys and
// structures, fixed values, nested serialization, and variable/output
// relationships. Naturally dynamic values (UUIDs) are canonicalized
// consistently within each document so relationships remain comparable.

const maxDifferences = 50

type Difference struct {
	Path string `json:"path"`
	Kind string `json:"kind"`
	A    any    `json:"a,omitempty"`
	B    any    `json:"b,omitempty"`
}

type ComparisonResult struct {
	Equal       bool         `json:"equal"`
	Differences []Difference `json:"differences,omitempty"`
	Truncated   bool         `json:"truncated,omitempty"`
}

var volatileTopLevelKeys = map[string]bool{
	"WFWorkflowClientVersion": true,
}

type canonicalizer struct {
	uuidOrder []string
	uuidSeen  map[string]string
}

func loadShortcutDocument(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var decoded any
	if _, err = plist.Unmarshal(data, &decoded); err != nil {
		if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil {
			return nil, fmt.Errorf("%s: not a parsable plist/json Shortcut", path)
		}
	}
	doc, ok := decoded.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("%s: top level is not a dictionary", path)
	}
	return doc, nil
}

func CompareShortcuts(pathA, pathB string) (*ComparisonResult, error) {
	docA, err := loadShortcutDocument(pathA)
	if err != nil {
		return nil, err
	}
	docB, err := loadShortcutDocument(pathB)
	if err != nil {
		return nil, err
	}

	result := &ComparisonResult{Equal: true}
	compareCanonical(canonicalize(docA), canonicalize(docB), "", result)
	sort.Slice(result.Differences, func(i, j int) bool {
		return result.Differences[i].Path < result.Differences[j].Path
	})
	return result, nil
}

// canonicalize drops volatile metadata and maps every UUID to a stable
// placeholder based on first appearance, preserving relationship structure.
func canonicalize(doc map[string]any) any {
	canonicalizer := &canonicalizer{uuidSeen: map[string]string{}}
	cleaned := make(map[string]any, len(doc))
	for key, value := range doc {
		if volatileTopLevelKeys[key] {
			continue
		}
		cleaned[key] = canonicalizer.walk(value)
	}
	return cleaned
}

func (c *canonicalizer) walk(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		// Sorted traversal makes UUID labeling a deterministic function of
		// document content; otherwise equivalent documents could receive
		// different placeholder numbers from random map iteration.
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)

		result := make(map[string]any, len(typed))
		for _, key := range keys {
			result[key] = c.walk(typed[key])
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = c.walk(item)
		}
		return result
	case string:
		if uuidPattern.MatchString(typed) {
			return c.mapUUID(typed)
		}
		return typed
	default:
		return value
	}
}

func (c *canonicalizer) mapUUID(uuid string) string {
	if label, seen := c.uuidSeen[uuid]; seen {
		return label
	}
	label := fmt.Sprintf("%s%d", placeholderPrefix+"uuid:", len(c.uuidOrder)+1)
	c.uuidSeen[uuid] = label
	c.uuidOrder = append(c.uuidOrder, uuid)
	return label
}

func compareCanonical(a, b any, path string, result *ComparisonResult) {
	if len(result.Differences) > maxDifferences {
		result.Truncated = true
		return
	}
	switch typedA := a.(type) {
	case map[string]any:
		typedB, ok := b.(map[string]any)
		if !ok {
			addDifference(result, path, "type", describe(a), describe(b))
			return
		}
		for _, key := range sortedStringKeys(unionMaps(typedA, typedB)) {
			valueA, inA := typedA[key]
			valueB, inB := typedB[key]
			childPath := path + "." + key
			switch {
			case !inA:
				addDifference(result, childPath, "missing-in-a", nil, describe(valueB))
			case !inB:
				addDifference(result, childPath, "missing-in-b", describe(valueA), nil)
			default:
				compareCanonical(valueA, valueB, childPath, result)
			}
		}
	case []any:
		typedB, ok := b.([]any)
		if !ok {
			addDifference(result, path, "type", describe(a), describe(b))
			return
		}
		if len(typedA) != len(typedB) {
			addDifference(result, path+".length", "length", len(typedA), len(typedB))
		}
		limit := len(typedA)
		if len(typedB) < limit {
			limit = len(typedB)
		}
		for index := 0; index < limit; index++ {
			compareCanonical(typedA[index], typedB[index], fmt.Sprintf("%s[%d]", path, index), result)
		}
	default:
		if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
			addDifference(result, path, "value", describe(a), describe(b))
		}
	}
}

func addDifference(result *ComparisonResult, path, kind string, a, b any) {
	if len(result.Differences) >= maxDifferences {
		result.Truncated = true
		return
	}
	result.Equal = false
	result.Differences = append(result.Differences, Difference{Path: path, Kind: kind, A: a, B: b})
}

func unionMaps(a, b map[string]any) map[string]bool {
	union := make(map[string]bool, len(a)+len(b))
	for key := range a {
		union[key] = true
	}
	for key := range b {
		union[key] = true
	}
	return union
}

func sortedStringKeys(set map[string]bool) []string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func describe(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return "object{" + strings.Join(keys, ",") + "}"
	case []any:
		return fmt.Sprintf("array(%d)", len(typed))
	case nil:
		return nil
	default:
		return value
	}
}

func printComparison(result *ComparisonResult, asJSON bool) {
	if asJSON {
		encoded, _ := json.MarshalIndent(result, "", "  ")
		fmt.Println(string(encoded))
		return
	}
	if result.Equal {
		fmt.Println("Structurally equal.")
		return
	}
	fmt.Printf("Structural differences (%d):\n", len(result.Differences))
	for _, difference := range result.Differences {
		fmt.Printf("  %-12s %s | a=%v b=%v\n", difference.Kind, difference.Path, difference.A, difference.B)
	}
	if result.Truncated {
		fmt.Println("  ... additional differences truncated")
	}
}
