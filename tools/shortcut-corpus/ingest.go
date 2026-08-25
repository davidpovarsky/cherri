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

// errNotAShortcut marks parsable files that are not Shortcut documents, such
// as catalog JSON or unrelated data that happens to sit in an input directory.
var errNotAShortcut = errors.New("not a Shortcut document")

const stateVersion = 1

// Evidence records how much a fact can be trusted. Corpus observations are
// always "observed"; only Cherri catalog facts are "confirmed", and analyzer
// heuristics are "inferred". A single sample never promotes a fact.
type Evidence struct {
	Status      string `json:"status"`
	Confidence  string `json:"confidence,omitempty"`
	SampleCount int    `json:"sampleCount,omitempty"`
}

var (
	evidenceObserved = func(count int) Evidence {
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
)

// FileRecord tracks an ingested source file by content hash so identical
// files are never re-analyzed, regardless of filename or location.
type FileRecord struct {
	Name       string `json:"name"`
	ActionSeen int    `json:"actions"`
	NewActions int    `json:"newActions"`
	FirstSeen  string `json:"firstSeen"`
}

// ActionRecord is one unique structural action shape observed in the corpus.
type ActionRecord struct {
	Fingerprint    string                      `json:"fingerprint"`
	Identifier     string                      `json:"identifier"`
	Classification string                      `json:"classification"`
	ParameterKeys  []string                    `json:"parameterKeys"`
	Parameters     map[string]*NormalizedValue `json:"parameters"`
	Evidence       Evidence                    `json:"evidence"`
	Count          int                         `json:"count"`
	Files          []string                    `json:"files"`
	Notes          []string                    `json:"notes,omitempty"`
	FirstSeen      string                      `json:"firstSeen"`
	LastSeen       string                      `json:"lastSeen"`
}

// State is the deterministic, committed-optional processing state. It contains
// only sanitized structural data and file hashes, never raw Shortcut content.
type State struct {
	Version    int                      `json:"version"`
	Files      map[string]*FileRecord   `json:"files"`
	Actions    map[string]*ActionRecord `json:"actions"`
	Candidates map[string]bool          `json:"candidates,omitempty"`
}

func newState() *State {
	return &State{
		Version: stateVersion,
		Files:   map[string]*FileRecord{},
		Actions: map[string]*ActionRecord{},
	}
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
	if state.Files == nil {
		state.Files = map[string]*FileRecord{}
	}
	if state.Actions == nil {
		state.Actions = map[string]*ActionRecord{}
	}
	state.Version = stateVersion
	return &state, nil
}

func (state *State) save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	encoded, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err = os.WriteFile(temp, encoded, 0644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

type shortcutDocument struct {
	hash           string
	name           string
	path           string
	clientVersion  string
	types          []string
	actions        []rawAction
	rawActionCount int
}

type rawAction struct {
	Identifier string
	Parameters map[string]any
}

// discoverInputs walks the given inputs and returns candidate Shortcut files.
// Directories are traversed recursively; hidden directories are skipped.
func discoverInputs(inputs []string) ([]string, error) {
	var files []string
	for _, input := range inputs {
		info, err := os.Stat(input)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", input, err)
		}
		if !info.IsDir() {
			files = append(files, input)
			continue
		}
		err = filepath.WalkDir(input, func(path string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if strings.HasPrefix(entry.Name(), ".") && path != input {
					return filepath.SkipDir
				}
				return nil
			}
			if isShortcutCandidate(entry.Name()) {
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(files)
	return files, nil
}

func isShortcutCandidate(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".json", ".plist", ".shortcut":
		return true
	}
	return false
}

func hashBytes(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

// parseDocument parses plist XML/binary/JSON Shortcut data into the generic
// document model. Real corpora contain more than one serialization dialect,
// so nothing here assumes a single schema beyond Apple's top-level keys.
func parseDocument(path string, data []byte) (*shortcutDocument, error) {
	var decoded any
	if _, err := plist.Unmarshal(data, &decoded); err != nil {
		if jsonErr := json.Unmarshal(data, &decoded); jsonErr != nil {
			return nil, fmt.Errorf("not a parsable plist/json Shortcut: %w", err)
		}
	}

	root, ok := decoded.(map[string]any)
	if !ok {
		return nil, errNotAShortcut
	}

	doc := &shortcutDocument{
		hash: hashBytes(data),
		name: filepath.Base(path),
		path: path,
	}
	doc.clientVersion = stringValue(root["WFWorkflowClientVersion"])
	for _, value := range sliceValue(root["WFWorkflowTypes"]) {
		sanitizedType, _ := sanitizeText(stringValue(value))
		doc.types = append(doc.types, sanitizedType)
	}

	rawActions, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		return nil, fmt.Errorf("%w: no WFWorkflowActions array", errNotAShortcut)
	}
	for index, item := range rawActions {
		entry, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("action %d is not a dictionary", index)
		}
		parameters, _ := entry["WFWorkflowActionParameters"].(map[string]any)
		if parameters == nil {
			parameters = map[string]any{}
		}
		doc.actions = append(doc.actions, rawAction{
			Identifier: stringValue(entry["WFWorkflowActionIdentifier"]),
			Parameters: parameters,
		})
	}
	doc.rawActionCount = len(doc.actions)
	return doc, nil
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return text
	}
	return ""
}

func sliceValue(value any) []any {
	if items, ok := value.([]any); ok {
		return items
	}
	return nil
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
