package shortcutcompare

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"reflect"
	"regexp"
	"strconv"
)

type EditPolicy struct {
	SchemaVersion  int          `json:"schemaVersion"`
	OriginalSHA256 string       `json:"originalSHA256"`
	Changes        []EditChange `json:"changes"`
}

type EditChange struct {
	Path   []string        `json:"path"`
	Before json.RawMessage `json:"before"`
	After  json.RawMessage `json:"after"`
}

type EditVerification struct {
	OK              bool              `json:"ok"`
	OriginalSHA256  string            `json:"originalSHA256"`
	CandidateSHA256 string            `json:"candidateSHA256"`
	AllowedChanges  int               `json:"allowedChanges"`
	Comparison      *ComparisonResult `json:"comparison"`
}

var editDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// CompareDocumentsExact preserves decoded plist types, identity fields and all
// metadata. The historical parity comparator remains useful for diagnostics,
// but none of its coercions or alpha-renaming can grant an edit authorization.
func CompareDocumentsExact(a, b map[string]any) *ComparisonResult {
	if exactEditValue(a, b) {
		return &ComparisonResult{Equal: true}
	}
	result := CompareDocumentsStrict(a, b)
	result.Equal = false
	if len(result.Differences) == 0 {
		result.Differences = []Difference{{Path: "$", Kind: "exact-typed-document-mismatch"}}
	}
	return result
}

func exactEditValue(a, b any) bool {
	if reflect.TypeOf(a) != reflect.TypeOf(b) {
		return false
	}
	switch av := a.(type) {
	case map[string]any:
		bv := b.(map[string]any)
		if (av == nil) != (bv == nil) || len(av) != len(bv) {
			return false
		}
		for k, v := range av {
			other, ok := bv[k]
			if !ok || !exactEditValue(v, other) {
				return false
			}
		}
		return true
	case []any:
		bv := b.([]any)
		if (av == nil) != (bv == nil) || len(av) != len(bv) {
			return false
		}
		for i, v := range av {
			if !exactEditValue(v, bv[i]) {
				return false
			}
		}
		return true
	case float64:
		return math.Float64bits(av) == math.Float64bits(b.(float64))
	case float32:
		return math.Float32bits(av) == math.Float32bits(b.(float32))
	default:
		return reflect.DeepEqual(a, b)
	}
}

// VerifyEditFiles reads each input once, binds the policy to the original bytes,
// applies only explicit existing scalar replacements in memory, and compares the
// complete expected document. It never writes either input or signs a Shortcut.
func VerifyEditFiles(originalPath, candidatePath, policyPath string) (*EditVerification, error) {
	original, err := os.ReadFile(originalPath)
	if err != nil {
		return nil, err
	}
	candidate, err := os.ReadFile(candidatePath)
	if err != nil {
		return nil, err
	}
	expected, err := decodeShortcutDocument(original, false)
	if err != nil {
		return nil, fmt.Errorf("original: %w", err)
	}
	actual, err := decodeShortcutDocument(candidate, false)
	if err != nil {
		return nil, fmt.Errorf("candidate: %w", err)
	}
	for name, doc := range map[string]map[string]any{"original": expected, "candidate": actual} {
		if _, ok := doc["WFWorkflowActions"].([]any); !ok {
			return nil, fmt.Errorf("%s has no workflow action array", name)
		}
	}
	result := &EditVerification{
		OriginalSHA256:  fmt.Sprintf("%x", sha256.Sum256(original)),
		CandidateSHA256: fmt.Sprintf("%x", sha256.Sum256(candidate)),
	}
	if policyPath != "" {
		data, readErr := os.ReadFile(policyPath)
		if readErr != nil {
			return nil, readErr
		}
		policy, parseErr := decodeEditPolicy(data)
		if parseErr != nil {
			return nil, parseErr
		}
		if policy.OriginalSHA256 != result.OriginalSHA256 {
			return nil, fmt.Errorf("edit policy original SHA256 mismatch")
		}
		for i, change := range policy.Changes {
			if err := applyEditChange(expected, change); err != nil {
				return nil, fmt.Errorf("edit policy change %d: %w", i, err)
			}
		}
		result.AllowedChanges = len(policy.Changes)
	}
	result.Comparison = CompareDocumentsExact(expected, actual)
	result.OK = result.Comparison.Equal
	return result, nil
}

func decodeEditPolicy(data []byte) (*EditPolicy, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := uniqueEditJSONValue(decoder); err != nil {
		return nil, fmt.Errorf("invalid edit policy: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, fmt.Errorf("edit policy must contain exactly one JSON value")
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var policy EditPolicy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("invalid edit policy: %w", err)
	}
	if policy.SchemaVersion != 1 || !editDigestPattern.MatchString(policy.OriginalSHA256) || policy.Changes == nil {
		return nil, fmt.Errorf("edit policy requires schemaVersion 1, originalSHA256 and changes array")
	}
	seen := map[string]bool{}
	for _, change := range policy.Changes {
		if len(change.Path) == 0 || len(change.Before) == 0 || len(change.After) == 0 {
			return nil, fmt.Errorf("each edit change requires path, before and after")
		}
		key, _ := json.Marshal(change.Path)
		if seen[string(key)] {
			return nil, fmt.Errorf("duplicate edit policy path")
		}
		seen[string(key)] = true
	}
	return &policy, nil
}

func uniqueEditJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	if delim != '{' && delim != '[' {
		return fmt.Errorf("unexpected JSON delimiter")
	}
	seen := map[string]bool{}
	for decoder.More() {
		if delim == '{' {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok || seen[key] {
				return fmt.Errorf("duplicate JSON field")
			}
			seen[key] = true
		}
		if err := uniqueEditJSONValue(decoder); err != nil {
			return err
		}
	}
	_, err = decoder.Token()
	return err
}

func applyEditChange(document map[string]any, change EditChange) error {
	var parent any = document
	for _, part := range change.Path[:len(change.Path)-1] {
		next, err := editPathValue(parent, part)
		if err != nil {
			return err
		}
		parent = next
	}
	last := change.Path[len(change.Path)-1]
	current, err := editPathValue(parent, last)
	if err != nil {
		return err
	}
	before, err := editScalarValue(change.Before, current)
	if err != nil {
		return fmt.Errorf("before: %w", err)
	}
	after, err := editScalarValue(change.After, current)
	if err != nil {
		return fmt.Errorf("after: %w", err)
	}
	if !exactEditValue(current, before) {
		return fmt.Errorf("before value does not match original field")
	}
	if exactEditValue(before, after) {
		return fmt.Errorf("replacement does not change the field")
	}
	switch container := parent.(type) {
	case map[string]any:
		container[last] = after
	case []any:
		index, _ := strconv.Atoi(last)
		container[index] = after
	default:
		return fmt.Errorf("path parent is not a dictionary or array")
	}
	return nil
}

func editPathValue(parent any, part string) (any, error) {
	switch container := parent.(type) {
	case map[string]any:
		value, ok := container[part]
		if !ok {
			return nil, fmt.Errorf("path does not name an existing field")
		}
		return value, nil
	case []any:
		index, err := strconv.Atoi(part)
		if err != nil || strconv.Itoa(index) != part || index < 0 || index >= len(container) {
			return nil, fmt.Errorf("invalid array index in path")
		}
		return container[index], nil
	default:
		return nil, fmt.Errorf("path crosses a scalar field")
	}
}

func editScalarValue(raw json.RawMessage, existing any) (any, error) {
	if existing == nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, fmt.Errorf("only existing non-null scalar fields are supported")
	}
	kind := reflect.TypeOf(existing).Kind()
	switch kind {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
	default:
		return nil, fmt.Errorf("only existing scalar field replacements are supported")
	}
	target := reflect.New(reflect.TypeOf(existing))
	if err := json.Unmarshal(raw, target.Interface()); err != nil {
		return nil, fmt.Errorf("value does not preserve original scalar type: %w", err)
	}
	return target.Elem().Interface(), nil
}
