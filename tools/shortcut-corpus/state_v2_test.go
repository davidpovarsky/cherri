/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestSchemaFingerprintIgnoresOrdinaryLiteralValues(t *testing.T) {
	first := map[string]*NormalizedValue{"text": {Kind: kindString, Constant: placeholderPrefix + "text"}, "count": {Kind: kindNumber, Constant: "3"}, "toggle": {Kind: kindBoolean, Constant: "true"}}
	second := map[string]*NormalizedValue{"text": {Kind: kindString, Constant: placeholderPrefix + "url"}, "count": {Kind: kindNumber, Constant: "99"}, "toggle": {Kind: kindBoolean, Constant: "false"}}
	if fingerprintAction("is.workflow.actions.example", first) != fingerprintAction("is.workflow.actions.example", second) { t.Fatal("ordinary literal values must not create distinct action schemas") }
}

func TestSchemaFingerprintDetectsSerializationChange(t *testing.T) {
	plain := map[string]*NormalizedValue{"WFInput": {Kind: kindString, Constant: placeholderPrefix + "text"}}
	serialized := map[string]*NormalizedValue{"WFInput": {Kind: kindDictionary, Serialization: "WFTextTokenAttachment", Fields: map[string]*NormalizedValue{"Value": {Kind: kindDictionary, Fields: map[string]*NormalizedValue{"Type": {Kind: kindString, Constant: "ActionOutput"}}}}}}
	if fingerprintAction("is.workflow.actions.example", plain) == fingerprintAction("is.workflow.actions.example", serialized) { t.Fatal("material serialization changes must produce distinct schema fingerprints") }
}

func TestDistinctShortcutHashesOwnEvidenceCount(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist")); if err != nil { t.Fatal(err) }
	doc, err := parseDocument("sample-shortcut.plist", data); if err != nil { t.Fatal(err) }
	state := newState(); catalog := fixtureCatalog(t)
	ingestDocument(state, doc, catalog, analyzeConfig{}); ingestDocument(state, doc, catalog, analyzeConfig{force: true})
	for _, record := range state.Actions { if record.Count != 1 || record.Evidence.SampleCount != 1 || len(record.SampleHashes) != 1 { t.Fatalf("same Shortcut content must contribute once, got %+v", record) } }
}

func TestReclassifyUsesNewCatalogWithoutRawReparse(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist")); if err != nil { t.Fatal(err) }
	doc, err := parseDocument("sample-shortcut.plist", data); if err != nil { t.Fatal(err) }
	state := newState(); oldCatalog := fixtureCatalog(t); ingestDocument(state, doc, oldCatalog, analyzeConfig{}); reclassifyState(state, oldCatalog)
	var mystery *ActionRecord
	for _, record := range state.Actions { if record.Identifier == "is.workflow.actions.mysteryaction" { mystery = record; break } }
	if mystery == nil || mystery.Classification != classSafeCandidate { t.Fatalf("expected unresolved mystery action before catalog update, got %+v", mystery) }
	countBefore := mystery.Count
	catalogJSON := []byte(`{"ok":true,"version":"2","actions":[{"name":"comment","shortcutIdentifier":"is.workflow.actions.comment","parameters":[{"name":"text","key":"WFCommentActionText","type":"text"}]},{"name":"alert","shortcutIdentifier":"is.workflow.actions.alert","parameters":[{"name":"alert","key":"WFAlertActionTitle","type":"text"},{"name":"showCancel","key":"WFAlertActionCancelButtonShown","type":"bool"}]},{"name":"mystery","shortcutIdentifier":"is.workflow.actions.mysteryaction","parameters":[{"name":"input","key":"WFInput","type":"text"},{"name":"option","key":"WFOption","type":"text"}]}]}`)
	newCatalog, err := parseCatalog(catalogJSON, "updated-catalog.json"); if err != nil { t.Fatal(err) }
	reclassifyState(state, newCatalog)
	if mystery.Classification != classKnown { t.Fatalf("stored evidence must become KNOWN after catalog update, got %s", mystery.Classification) }
	if mystery.Count != countBefore { t.Fatal("reclassification must not change evidence count") }
	for _, item := range buildAgentQueue(state, newCatalog, false) { if item.Identifier == mystery.Identifier { t.Fatal("resolved action must disappear from agent queue") } }
}

func TestStateV1MigrationCollapsesValueSpecificFingerprints(t *testing.T) {
	legacy := State{Version: 1, Files: map[string]*FileRecord{"hash-a": {Name: "a.plist"}, "hash-b": {Name: "b.plist"}}, Actions: map[string]*ActionRecord{
		"legacy-a": {Fingerprint: "legacy-a", Identifier: "is.workflow.actions.wait", Parameters: map[string]*NormalizedValue{"WFCount": {Kind: kindNumber, Constant: "3"}}, Files: []string{"hash-a"}, Count: 1, Evidence: evidenceObserved(1)},
		"legacy-b": {Fingerprint: "legacy-b", Identifier: "is.workflow.actions.wait", Parameters: map[string]*NormalizedValue{"WFCount": {Kind: kindNumber, Constant: "5"}}, Files: []string{"hash-b"}, Count: 1, Evidence: evidenceObserved(1)},
	}}
	dir := t.TempDir(); path := filepath.Join(dir, "state.json"); encoded, _ := json.Marshal(legacy); if err := os.WriteFile(path, encoded, 0644); err != nil { t.Fatal(err) }
	migrated, err := loadState(path); if err != nil { t.Fatal(err) }
	if migrated.Version != stateVersion || len(migrated.Actions) != 1 { t.Fatalf("expected one migrated schema record, got version=%d actions=%d", migrated.Version, len(migrated.Actions)) }
	for _, record := range migrated.Actions { if record.Count != 2 || len(record.SampleHashes) != 2 { t.Fatalf("migration must preserve distinct evidence hashes: %+v", record) } }
}
