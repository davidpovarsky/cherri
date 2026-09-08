/*
 * Copyright (c) Cherri
 */

package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"howett.net/plist"
)

var errNotAShortcut = errors.New("not a Shortcut document")

const stateVersion = 2

type Evidence struct {
	Status      string `json:"status"`
	Confidence  string `json:"confidence,omitempty"`
	SampleCount int    `json:"sampleCount,omitempty"`
}

var evidenceObserved = func(count int) Evidence {
	status := "observed"
	confidence := "low"
	switch {
	case count >= 10:
		confidence = "high"
	case count >= 5:
		confidence = "medium"
	}
	return Evidence{Status: status, Confidence: confidence, SampleCount: count}
}

type FileRecord struct {
	Name       string `json:"name"`
	ActionSeen int    `json:"actions"`
	NewActions int    `json:"newActions"`
	FirstSeen  string `json:"firstSeen"`
}

type ActionRecord struct {
	Fingerprint       string                      `json:"fingerprint"`
	Identifier        string                      `json:"identifier"`
	Classification    string                      `json:"classification"`
	ParameterKeys     []string                    `json:"parameterKeys"`
	Parameters        map[string]*NormalizedValue `json:"parameters"`
	ValueObservations map[string][]string          `json:"valueObservations,omitempty"`
	Evidence          Evidence                    `json:"evidence"`
	Count             int                         `json:"count"`
	SampleHashes      []string                    `json:"sampleHashes,omitempty"`
	Files             []string                    `json:"files,omitempty"`
	Notes             []string                    `json:"notes,omitempty"`
	FirstSeen         string                      `json:"firstSeen"`
	LastSeen          string                      `json:"lastSeen"`
}

type State struct {
	Version       int                      `json:"version"`
	CatalogDigest string                   `json:"catalogDigest,omitempty"`
	Files         map[string]*FileRecord   `json:"files"`
	Actions       map[string]*ActionRecord `json:"actions"`
	Candidates    map[string]bool          `json:"candidates,omitempty"`
}

func newState() *State {
	return &State{Version: stateVersion, Files: map[string]*FileRecord{}, Actions: map[string]*ActionRecord{}, Candidates: map[string]bool{}}
}

func loadState(path string) (*State, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return newState(), nil
		}
		return nil, err
	}
	var state State
	if err = json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("corrupt state file %s: %w", path, err)
	}
	if state.Version > stateVersion {
		return nil, fmt.Errorf("state file %s has unsupported version %d", path, state.Version)
	}
	if state.Files == nil { state.Files = map[string]*FileRecord{} }
	if state.Actions == nil { state.Actions = map[string]*ActionRecord{} }
	if state.Candidates == nil { state.Candidates = map[string]bool{} }
	if err = migrateState(&state); err != nil {
		return nil, fmt.Errorf("migrating state file %s: %w", path, err)
	}
	return &state, nil
}

func migrateState(state *State) error {
	if state.Version >= stateVersion {
		for _, record := range state.Actions { normalizeRecordSamples(record) }
		state.Version = stateVersion
		return nil
	}
	migrated := map[string]*ActionRecord{}
	for _, record := range state.Actions {
		if record.Parameters == nil { record.Parameters = map[string]*NormalizedValue{} }
		record.Fingerprint = fingerprintAction(record.Identifier, record.Parameters)
		if record.Fingerprint == "" { continue }
		normalizeRecordSamples(record)
		if record.ValueObservations == nil { record.ValueObservations = collectValueObservations(record.Parameters) }
		if existing, found := migrated[record.Fingerprint]; found {
			mergeActionRecords(existing, record)
			continue
		}
		migrated[record.Fingerprint] = record
	}
	state.Actions = migrated
	state.Version = stateVersion
	state.CatalogDigest = ""
	return nil
}

func normalizeRecordSamples(record *ActionRecord) {
	if record.SampleHashes == nil { record.SampleHashes = uniqueStrings(record.Files) }
	if record.Files == nil { record.Files = append([]string(nil), record.SampleHashes...) }
	record.SampleHashes = uniqueStrings(record.SampleHashes)
	record.Files = boundedUniqueStrings(record.Files, 32)
	record.Count = len(record.SampleHashes)
	if record.Count == 0 && record.Evidence.SampleCount > 0 { record.Count = record.Evidence.SampleCount }
	record.Evidence = evidenceObserved(record.Count)
}

func mergeActionRecords(target, source *ActionRecord) {
	for _, hash := range source.SampleHashes { addSampleHash(target, hash) }
	target.Files = boundedUniqueStrings(append(target.Files, source.Files...), 32)
	mergeValueObservations(target, source.ValueObservations)
	if target.FirstSeen == "" || (source.FirstSeen != "" && source.FirstSeen < target.FirstSeen) { target.FirstSeen = source.FirstSeen }
	if source.LastSeen > target.LastSeen { target.LastSeen = source.LastSeen }
}

func (state *State) save(path string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil { return err }
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil { return err }
	temp := path + ".tmp"
	if err = os.WriteFile(temp, encoded, 0644); err != nil { return err }
	return os.Rename(temp, path)
}

type shortcutDocument struct {
	hash string
	name string
	path string
	clientVersion string
	types []string
	actions []rawAction
	rawActionCount int
}

type rawAction struct { Identifier string; Parameters map[string]any }

func discoverInputs(inputs []string) ([]string, error) {
	var files []string
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil { return nil, fmt.Errorf("%s: %w", input, err) }
		if !info.IsDir() { files = append(files, input); continue }
		err = filepath.WalkDir(input, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil { return walkErr }
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") && path != input { return filepath.SkipDir }
				return nil
			}
			if isShortcutCandidate(entry.Name()) { files = append(files, path) }
			return nil
		})
		if err != nil { return nil, err }
	}
	sort.Strings(files)
	return files, nil
}

func isShortcutCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json", ".plist", ".shortcut", ".xml": return true
	}
	return false
}

func hashBytes(data []byte) string { digest := sha256.Sum256(data); return hex.EncodeToString(digest[:]) }

func parseDocument(path string, data []byte) (*shortcutDocument, error) {
	var decoded any
	if _, err := plist.Unmarshal(data, &decoded); err != nil {
		if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil { return nil, fmt.Errorf("not a parsable plist/json Shortcut: %w", err) }
	}
	root, ok := decoded.(map[string]any)
	if !ok { return nil, errNotAShortcut }
	doc := &shortcutDocument{hash: hashBytes(data), name: filepath.Base(path), path: path}
	doc.clientVersion = stringValue(root["WFWorkflowClientVersion"])
	for _, value := range sliceValue(root["WFWorkflowTypes"]) { sanitizedType, _ := sanitizeText(stringValue(value)); doc.types = append(doc.types, sanitizedType) }
	rawActions, ok := root["WFWorkflowActions"].([]any)
	if !ok { return nil, fmt.Errorf("%w: no WFWorkflowActions array", errNotAShortcut) }
	for index, item := range rawActions {
		entry, ok := item.(map[string]any)
		if !ok { return nil, fmt.Errorf("action %d is not a dictionary", index) }
		parameters, _ := entry["WFWorkflowActionParameters"].(map[string]any)
		if parameters == nil { parameters = map[string]any{} }
		doc.actions = append(doc.actions, rawAction{Identifier: stringValue(entry["WFWorkflowActionIdentifier"]), Parameters: parameters})
	}
	doc.rawActionCount = len(doc.actions)
	return doc, nil
}

func stringValue(value any) string { if text, ok := value.(string); ok { return text }; return "" }
func sliceValue(value any) []any { if items, ok := value.([]any); ok { return items }; return nil }
func now() string { return time.Now().UTC().Format(time.RFC3339) }

func uniqueStrings(values []string) []string {
	seen := map[string]bool{}; result := make([]string, 0, len(values))
	for _, value := range values { if value == "" || seen[value] { continue }; seen[value] = true; result = append(result, value) }
	sort.Strings(result); return result
}
func boundedUniqueStrings(values []string, limit int) []string { result := uniqueStrings(values); if limit > 0 && len(result) > limit { return result[:limit] }; return result }
func addSampleHash(record *ActionRecord, hash string) bool {
	if hash == "" || containsFile(record.SampleHashes, hash) { return false }
	record.SampleHashes = append(record.SampleHashes, hash); record.SampleHashes = uniqueStrings(record.SampleHashes); record.Count = len(record.SampleHashes); record.Evidence = evidenceObserved(record.Count)
	if !containsFile(record.Files, hash) && len(record.Files) < 32 { record.Files = append(record.Files, hash); sort.Strings(record.Files) }
	return true
}
