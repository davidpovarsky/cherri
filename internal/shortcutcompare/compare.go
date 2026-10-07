/*
 * Copyright (c) Cherri Language v2.0
 * Structural Shortcut Comparator with Bijective Alpha-Renaming
 */

package shortcutcompare

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"

	"howett.net/plist"
)

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

type Scope struct {
	producersA   map[string]int // UUID -> action index in A
	producersB   map[string]int // UUID -> action index in B
	producerAtoB map[string]string
	producerBtoA map[string]string
	groupAtoB    map[string]string
	groupBtoA    map[string]string
}

func NewScope() *Scope {
	return &Scope{
		producersA:   make(map[string]int),
		producersB:   make(map[string]int),
		producerAtoB: make(map[string]string),
		producerBtoA: make(map[string]string),
		groupAtoB:    make(map[string]string),
		groupBtoA:    make(map[string]string),
	}
}

func LoadShortcutDocument(path string) (map[string]any, error) {
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

func CompareFiles(pathA, pathB string) (*ComparisonResult, error) {
	docA, err := LoadShortcutDocument(pathA)
	if err != nil {
		return nil, err
	}
	docB, err := LoadShortcutDocument(pathB)
	if err != nil {
		return nil, err
	}
	return CompareDocuments(docA, docB), nil
}

func BuildDocumentScope(docA, docB map[string]any) *Scope {
	scope := NewScope()

	// Step 1: Discover declared action producers and control flow groups
	actionsA, _ := docA["WFWorkflowActions"].([]any)
	actionsB, _ := docB["WFWorkflowActions"].([]any)

	for i, act := range actionsA {
		if actMap, ok := act.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				if u, ok := params["UUID"].(string); ok && u != "" {
					scope.producersA[u] = i
				}
			}
		}
	}
	for i, act := range actionsB {
		if actMap, ok := act.(map[string]any); ok {
			if params, ok := actMap["WFWorkflowActionParameters"].(map[string]any); ok {
				if u, ok := params["UUID"].(string); ok && u != "" {
					scope.producersB[u] = i
				}
			}
		}
	}

	// Step 2: Establish bijective alpha-renaming between matching producer actions
	limit := len(actionsA)
	if len(actionsB) < limit {
		limit = len(actionsB)
	}
	for i := 0; i < limit; i++ {
		actA, okA := actionsA[i].(map[string]any)
		actB, okB := actionsB[i].(map[string]any)
		if !okA || !okB {
			continue
		}
		paramsA, _ := actA["WFWorkflowActionParameters"].(map[string]any)
		paramsB, _ := actB["WFWorkflowActionParameters"].(map[string]any)
		if paramsA != nil && paramsB != nil {
			uA, okUA := paramsA["UUID"].(string)
			uB, okUB := paramsB["UUID"].(string)
			if okUA && okUB && uA != "" && uB != "" {
				scope.producerAtoB[uA] = uB
				scope.producerBtoA[uB] = uA
			}
			gA, okGA := paramsA["GroupingIdentifier"].(string)
			gB, okGB := paramsB["GroupingIdentifier"].(string)
			if okGA && okGB && gA != "" && gB != "" {
				if scope.groupAtoB[gA] == "" && scope.groupBtoA[gB] == "" {
					scope.groupAtoB[gA] = gB
					scope.groupBtoA[gB] = gA
				}
			}
		}
	}
	return scope
}

func CompareDocuments(docA, docB map[string]any) *ComparisonResult {
	res := &ComparisonResult{Equal: true}
	scope := BuildDocumentScope(docA, docB)

	// Step 3: Compare documents recursively
	compareDict(docA, docB, "", scope, volatileTopLevelKeys, res)

	sort.Slice(res.Differences, func(i, j int) bool {
		return res.Differences[i].Path < res.Differences[j].Path
	})
	return res
}

func CompareActionParameters(legacyParams, v2Params map[string]any, scope *Scope) *ComparisonResult {
	res := &ComparisonResult{Equal: true}
	if scope == nil {
		scope = NewScope()
	}

	// If standalone, pair UUIDs and GroupingIdentifiers if present
	if uA, okA := legacyParams["UUID"].(string); okA {
		if uB, okB := v2Params["UUID"].(string); okB {
			if scope.producerAtoB[uA] == "" && scope.producerBtoA[uB] == "" {
				scope.producerAtoB[uA] = uB
				scope.producerBtoA[uB] = uA
				scope.producersA[uA] = 0
				scope.producersB[uB] = 0
			}
		}
	}
	if gA, okA := legacyParams["GroupingIdentifier"].(string); okA {
		if gB, okB := v2Params["GroupingIdentifier"].(string); okB {
			if scope.groupAtoB[gA] == "" && scope.groupBtoA[gB] == "" {
				scope.groupAtoB[gA] = gB
				scope.groupBtoA[gB] = gA
			}
		}
	}

	compareDict(legacyParams, v2Params, "WFWorkflowActionParameters", scope, nil, res)

	sort.Slice(res.Differences, func(i, j int) bool {
		return res.Differences[i].Path < res.Differences[j].Path
	})
	return res
}

func compareDict(a, b map[string]any, path string, scope *Scope, ignoredKeys map[string]bool, res *ComparisonResult) {
	if len(res.Differences) >= maxDifferences {
		res.Truncated = true
		return
	}

	union := make(map[string]bool, len(a)+len(b))
	for k := range a {
		if ignoredKeys == nil || !ignoredKeys[k] {
			union[k] = true
		}
	}
	for k := range b {
		if ignoredKeys == nil || !ignoredKeys[k] {
			union[k] = true
		}
	}

	keys := make([]string, 0, len(union))
	for k := range union {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		valA, inA := a[k]
		valB, inB := b[k]
		childPath := k
		if path != "" {
			childPath = path + "." + k
		}

		if !inA {
			addDiff(res, childPath, "missing-in-a", nil, describe(valB))
			continue
		}
		if !inB {
			addDiff(res, childPath, "missing-in-b", describe(valA), nil)
			continue
		}

		// Handle structural identity fields with bijective mapping
		if k == "UUID" && isActionParametersContext(path) {
			strA, okA := valA.(string)
			strB, okB := valB.(string)
			if okA && okB {
				expectedB, hasA := scope.producerAtoB[strA]
				expectedA, hasB := scope.producerBtoA[strB]
				if hasA && hasB && expectedB == strB && expectedA == strA {
					continue // Bijective match
				}
				addDiff(res, childPath, "producer-uuid-mismatch", strA, strB)
				continue
			}
		}

		if k == "GroupingIdentifier" {
			strA, okA := valA.(string)
			strB, okB := valB.(string)
			if okA && okB {
				if scope.groupAtoB[strA] == "" && scope.groupBtoA[strB] == "" {
					scope.groupAtoB[strA] = strB
					scope.groupBtoA[strB] = strA
				}
				if scope.groupAtoB[strA] == strB && scope.groupBtoA[strB] == strA {
					continue // Bijective group match
				}
				addDiff(res, childPath, "grouping-identifier-mismatch", strA, strB)
				continue
			}
		}

		if k == "OutputUUID" {
			strA, okA := valA.(string)
			strB, okB := valB.(string)
			if okA && okB {
				// Verify declared producer in A
				if len(scope.producersA) > 0 {
					if _, ok := scope.producersA[strA]; !ok {
						addDiff(res, childPath, "dangling-reference-in-a", strA, nil)
						continue
					}
				}
				// Verify declared producer in B
				if len(scope.producersB) > 0 {
					if _, ok := scope.producersB[strB]; !ok {
						addDiff(res, childPath, "dangling-reference-in-b", nil, strB)
						continue
					}
				}
				// Verify mapping
				if scope.producerAtoB[strA] == strB && scope.producerBtoA[strB] == strA {
					continue // Bijective reference match
				}
				addDiff(res, childPath, "output-uuid-mismatch", strA, strB)
				continue
			}
		}

		compareValues(valA, valB, childPath, scope, res)
	}
}

func isActionParametersContext(path string) bool {
	return strings.HasSuffix(path, "WFWorkflowActionParameters") || strings.Contains(path, "WFWorkflowActionParameters")
}

func compareValues(a, b any, path string, scope *Scope, res *ComparisonResult) {
	if len(res.Differences) >= maxDifferences {
		res.Truncated = true
		return
	}

	if a == nil && b == nil {
		return
	}
	if a == nil || b == nil {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Exact deep equality
	if reflect.DeepEqual(a, b) {
		return
	}

	// Map
	mapA, okMapA := a.(map[string]any)
	mapB, okMapB := b.(map[string]any)
	if okMapA && okMapB {
		compareDict(mapA, mapB, path, scope, nil, res)
		return
	}
	if okMapA != okMapB {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Slice
	sliceA, okSliceA := toSlice(a)
	sliceB, okSliceB := toSlice(b)
	if okSliceA && okSliceB {
		if len(sliceA) != len(sliceB) {
			addDiff(res, path+".length", "length", len(sliceA), len(sliceB))
		}
		limit := len(sliceA)
		if len(sliceB) < limit {
			limit = len(sliceB)
		}
		for i := 0; i < limit; i++ {
			compareValues(sliceA[i], sliceB[i], fmt.Sprintf("%s[%d]", path, i), scope, res)
		}
		return
	}
	if okSliceA != okSliceB {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Numeric comparisons
	numA, isNumA := toNumeric(a)
	numB, isNumB := toNumeric(b)
	if isNumA && isNumB {
		if numA != numB {
			addDiff(res, path, "value", a, b)
		}
		return
	}
	if isNumA != isNumB {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Booleans
	boolA, isBoolA := a.(bool)
	boolB, isBoolB := b.(bool)
	if isBoolA && isBoolB {
		if boolA != boolB {
			addDiff(res, path, "value", boolA, boolB)
		}
		return
	}
	if isBoolA != isBoolB {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Strings
	strA, isStrA := a.(string)
	strB, isStrB := b.(string)
	if isStrA && isStrB {
		if strA != strB {
			addDiff(res, path, "value", strA, strB)
		}
		return
	}
	if isStrA != isStrB {
		addDiff(res, path, "type", describe(a), describe(b))
		return
	}

	// Fallback equality
	if fmt.Sprintf("%v", a) != fmt.Sprintf("%v", b) {
		addDiff(res, path, "value", describe(a), describe(b))
	}
}

func toSlice(v any) ([]any, bool) {
	if s, ok := v.([]any); ok {
		return s, true
	}
	val := reflect.ValueOf(v)
	if val.Kind() == reflect.Slice {
		res := make([]any, val.Len())
		for i := 0; i < val.Len(); i++ {
			res[i] = val.Index(i).Interface()
		}
		return res, true
	}
	return nil, false
}

func toNumeric(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

func addDiff(res *ComparisonResult, path, kind string, a, b any) {
	if len(res.Differences) >= maxDifferences {
		res.Truncated = true
		return
	}
	res.Equal = false
	res.Differences = append(res.Differences, Difference{
		Path: path,
		Kind: kind,
		A:    a,
		B:    b,
	})
}

func describe(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
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
