/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

func fixtureCatalog(t *testing.T) *actionCatalog {
	t.Helper()
	catalog, err := loadCatalogFromFile(filepath.Join("testdata", "catalog-fixture.json"))
	if err != nil {
		t.Fatalf("load catalog fixture: %v", err)
	}
	return catalog
}

func analyzeFixtureState(t *testing.T, inputs ...string) (*State, *analyzeStats) {
	t.Helper()
	state := newState()
	catalog := fixtureCatalog(t)
	for _, input := range inputs {
		data, err := os.ReadFile(input)
		if err != nil {
			t.Fatalf("read %s: %v", input, err)
		}
		doc, err := parseDocument(input, data)
		if err != nil {
			t.Fatalf("parse %s: %v", input, err)
		}
		ingestDocument(state, doc, catalog, analyzeConfig{})
		record := &FileRecord{Name: doc.name, ActionSeen: doc.rawActionCount, FirstSeen: now()}
		state.Files[doc.hash] = record
	}
	return state, nil
}

func TestAnalyzeClassificationsAndEvidence(t *testing.T) {
	state, _ := analyzeFixtureState(t, filepath.Join("testdata", "sample-shortcut.plist"))

	var alert, mystery, thirdParty *ActionRecord
	for _, record := range state.Actions {
		switch record.Identifier {
		case "is.workflow.actions.alert":
			alert = record
		case "is.workflow.actions.mysteryaction":
			mystery = record
		case "com.example.superapp.makeWidget":
			thirdParty = record
		}
	}

	if alert == nil || alert.Classification != classKnown {
		t.Fatalf("expected alert to be KNOWN, got %+v", alert)
	}
	if alert.Evidence.Status != "observed" || alert.Evidence.SampleCount != 1 {
		t.Fatalf("single sample must stay observed/1: %+v", alert.Evidence)
	}
	if mystery == nil || mystery.Classification != classSafeCandidate {
		t.Fatalf("expected unknown declarative action as SAFE_CANDIDATE, got %+v", mystery)
	}
	if mystery == nil || len(mystery.ParameterKeys) != 2 {
		t.Fatalf("ignored keys (CustomOutputName) must be excluded from parameter keys")
	}
	if thirdParty == nil || thirdParty.Classification != classThirdParty {
		t.Fatalf("expected THIRD_PARTY classification, got %+v", thirdParty)
	}

	// Personal values must not survive normalization anywhere in state.
	encoded, _ := json.Marshal(state)
	for _, secret := range []string{"jane.doe@example.invalid", "janedoe", "sk-live-abcdef0123456789abcdef", "555) 010-9999", "11111111-2222-3333-4444-555555555555"} {
		if got := containsFold(string(encoded), secret); got {
			t.Errorf("personal value %q leaked into state", secret)
		}
	}
}

func TestFileDeduplicationIgnoresFilenames(t *testing.T) {
	state, _ := analyzeFixtureState(t, filepath.Join("testdata", "sample-shortcut.plist"))
	totalAfterFirst := len(state.Files)

	source, _ := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist"))
	copyPath := filepath.Join(t.TempDir(), "totally-different-name.plist")
	if err := os.WriteFile(copyPath, source, 0644); err != nil {
		t.Fatal(err)
	}

	doc, err := parseDocument(copyPath, source)
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := state.Files[doc.hash]; !exists {
		t.Fatal("identical content under a new name must map to the known hash")
	}
	if totalAfterFirst != 1 {
		t.Fatalf("expected exactly one file record")
	}
}

func TestVariantDetectionAndEvidenceGrowth(t *testing.T) {
	state, _ := analyzeFixtureState(t,
		filepath.Join("testdata", "sample-shortcut.plist"),
		filepath.Join("testdata", "sample-variant.plist"),
	)

	variants := 0
	var mysteryForms []*ActionRecord
	for _, record := range state.Actions {
		if record.Identifier == "is.workflow.actions.mysteryaction" {
			mysteryForms = append(mysteryForms, record)
		}
		if record.Classification == classVariant {
			variants++
		}
	}
	if len(mysteryForms) != 2 {
		t.Fatalf("expected two structural forms of mysteryaction, got %d", len(mysteryForms))
	}
	foundVariant := false
	for _, form := range mysteryForms {
		if form.Classification == classVariant {
			foundVariant = true
		}
	}
	if !foundVariant || variants < 1 {
		t.Fatal("second structural form of a known identifier must classify as VARIANT")
	}

	// The alert appears in both fixtures with different text but the same
	// structure; it should dedupe into one record with evidence count 2.
	alerts := 0
	for _, record := range state.Actions {
		if record.Identifier == "is.workflow.actions.alert" {
			alerts++
			if record.Count != 2 {
				t.Fatalf("repeated shape should increment count to 2, got %d", record.Count)
			}
			if record.Evidence.SampleCount != 2 {
				t.Fatalf("evidence sample count should grow with observations")
			}
		}
	}
	if alerts != 1 {
		t.Fatalf("structurally identical actions must dedupe to one record")
	}
}

func TestReportsAndCandidatesWritten(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	outputDir := filepath.Join(dir, "analysis")

	config := analyzeConfig{
		inputs:    []string{filepath.Join("testdata")},
		outputDir: outputDir,
		statePath: statePath,
	}
	if err := os.Setenv("CHERRI_BIN", ""); err == nil {
		defer os.Unsetenv("CHERRI_BIN")
	}
	config.cherriBin = ""
	config.catalogPath = filepath.Join("testdata", "catalog-fixture.json")

	if err := analyze(config); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	for _, report := range []string{reportSummary, reportKnownActions, reportNewActions, reportVariants, reportThirdParty, reportNeedsReview, reportHumanSummary} {
		if _, err := os.Stat(filepath.Join(outputDir, report)); err != nil {
			t.Errorf("missing report %s", report)
		}
	}

	var summary summaryReport
	readJSON(t, filepath.Join(outputDir, reportSummary), &summary)
	if summary.Actions.Total == 0 || summary.Catalog.Count != 3 {
		t.Fatalf("unexpected summary: %+v", summary)
	}
	if summary.Files.Analyzed != 2 {
		t.Fatalf("expected both plist fixtures analyzed, got %d", summary.Files.Analyzed)
	}

	candidates, err := os.ReadDir(filepath.Join(outputDir, reportCandidatesDir))
	if err != nil || len(candidates) == 0 {
		t.Fatalf("expected candidate artifacts for safe candidates")
	}
	for _, entry := range candidates {
		content, readErr := os.ReadFile(filepath.Join(outputDir, reportCandidatesDir, entry.Name()))
		if readErr == nil && strings.HasSuffix(entry.Name(), ".cherri") {
			if !containsFold(string(content), "NOT FOR PRODUCTION") {
				t.Error("candidates must be clearly marked as non-production")
			}
		}
	}

	// Incremental run: same inputs must skip everything and regenerate nothing.
	before := len(readStateCandidates(t, statePath))
	if err = analyze(config); err != nil {
		t.Fatalf("second analyze: %v", err)
	}
	var second summaryReport
	readJSON(t, filepath.Join(outputDir, reportSummary), &second)
	if second.Files.SkippedKnown != 2 || second.Files.Analyzed != 0 {
		t.Fatalf("incremental run should skip known files: %+v", second.Files)
	}
	if after := len(readStateCandidates(t, statePath)); after != before {
		t.Fatal("candidates must not be regenerated for already-emitted fingerprints")
	}
}

func TestPlistAndJsonFormsProduceSameFingerprints(t *testing.T) {
	plistBytes, err := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist"))
	if err != nil {
		t.Fatal(err)
	}
	var decoded any
	if _, err = plist.Unmarshal(plistBytes, &decoded); err != nil {
		t.Fatal(err)
	}
	jsonBytes, jsonErr := json.Marshal(decoded)
	if jsonErr != nil {
		t.Fatal(jsonErr)
	}

	docFromPlist, err := parseDocument("a.plist", plistBytes)
	if err != nil {
		t.Fatal(err)
	}
	docFromJSON, err := parseDocument("a.json", jsonBytes)
	if err != nil {
		t.Fatal(err)
	}

	fingerprint := func(doc *shortcutDocument) string {
		result := ""
		for _, item := range doc.actions {
			if item.Identifier == "is.workflow.actions.mysteryaction" {
				parameters := map[string]*NormalizedValue{}
				for key, value := range item.Parameters {
					parameters[key] = normalizeValue(value)
				}
				result = fingerprintAction(item.Identifier, parameters)
			}
		}
		return result
	}

	if fingerprint(docFromPlist) == "" || fingerprint(docFromPlist) != fingerprint(docFromJSON) {
		t.Fatal("plist and JSON encodings of identical data must produce identical fingerprints")
	}
}

func TestXMLShortcutFilesAreDiscovered(t *testing.T) {
	if !isShortcutCandidate("exported-shortcut.xml") {
		t.Fatal("canonical Apple XML plist Shortcut exports must be treated as candidates")
	}
	plistBytes, err := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	xmlPath := filepath.Join(dir, "exported-shortcut.xml")
	if err = os.WriteFile(xmlPath, plistBytes, 0644); err != nil {
		t.Fatal(err)
	}
	discovered, err := discoverInputs([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(discovered) != 1 || discovered[0] != xmlPath {
		t.Fatalf("expected XML export to be discovered, got %v", discovered)
	}
	doc, err := parseDocument(xmlPath, plistBytes)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.actions) == 0 {
		t.Fatal("parsed XML document must contain actions")
	}
}

func TestNestedPlainAppIntentDescriptorRequiresCustomImplementation(t *testing.T) {
	// Real App Intent-backed filter actions carry their descriptor as a plain
	// nested dictionary without a WFSerializationType marker.
	record := &ActionRecord{
		Identifier: "is.workflow.actions.filter.notes",
		Parameters: map[string]*NormalizedValue{
			"AppIntentDescriptor": {
				Kind: kindDictionary,
				Fields: map[string]*NormalizedValue{
					"ActionRequiresAppInstallation": {Kind: kindBoolean, Constant: "true"},
					"BundleIdentifier":              {Kind: kindString, Constant: "com.apple.mobilenotes", TextClass: "constant"},
				},
			},
			"WFContentItemLimitEnabled": {Kind: kindBoolean, Constant: "true"},
		},
	}
	if !requiresCustomImplementation(record) {
		t.Fatal("plain nested AppIntentDescriptor must require custom implementation")
	}

	deepNested := &ActionRecord{
		Identifier: "com.example.app.SomeIntent",
		Parameters: map[string]*NormalizedValue{
			"container": {
				Kind: kindDictionary,
				Fields: map[string]*NormalizedValue{
					"inner": {
						Kind: kindDictionary,
						Fields: map[string]*NormalizedValue{
							"AppIntentDescriptor": {Kind: kindDictionary},
						},
					},
				},
			},
		},
	}
	if !requiresCustomImplementation(deepNested) {
		t.Fatal("deeply nested AppIntentDescriptor field must require custom implementation")
	}

	declarative := &ActionRecord{
		Identifier: "is.workflow.actions.example",
		Parameters: map[string]*NormalizedValue{
			"WFTextActionText": {Kind: kindString, Constant: "hello"},
		},
	}
	if requiresCustomImplementation(declarative) {
		t.Fatal("declarative shape must not require custom implementation")
	}
}

func readJSON(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(data, target); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func readStateCandidates(t *testing.T, statePath string) map[string]bool {
	t.Helper()
	var state State
	readJSON(t, statePath, &state)
	if state.Candidates == nil {
		return map[string]bool{}
	}
	return state.Candidates
}

func containsFold(haystack, needle string) bool {
	return len(needle) > 0 && len(haystack) >= len(needle) && indexOfFold(haystack, needle) >= 0
}

func indexOfFold(haystack, needle string) int {
	h := strings.ToLower(haystack)
	n := strings.ToLower(needle)
	for i := 0; i+len(n) <= len(h); i++ {
		if h[i:i+len(n)] == n {
			return i
		}
	}
	return -1
}
