/*
 * Copyright (c) Cherri
 */

package main

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/icloudshortcut"
)

type acquisitionRoundTripFunc func(*http.Request) (*http.Response, error)

func (fn acquisitionRoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) { return fn(req) }

func acquisitionResponse(status int, body []byte) *http.Response {
	return &http.Response{StatusCode: status, Body: io.NopCloser(strings.NewReader(string(body))), Header: make(http.Header)}
}

func TestCollectDedupesContentAndFeedsAnalyzer(t *testing.T) {
	fixture, err := os.ReadFile(filepath.Join("testdata", "sample-shortcut.plist"))
	if err != nil { t.Fatal(err) }

	const idA = "11111111111111111111111111111111"
	const idB = "22222222222222222222222222222222"
	client := &http.Client{Transport: acquisitionRoundTripFunc(func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/shortcuts/api/records/") {
			id := filepath.Base(req.URL.Path)
			body := []byte(`{"fields":{"name":{"value":"Fixture"},"shortcut":{"value":{"downloadURL":"https://cvws.icloud-content.com/` + id + `"}}}}`)
			return acquisitionResponse(http.StatusOK, body), nil
		}
		if req.URL.Host == "cvws.icloud-content.com" { return acquisitionResponse(http.StatusOK, fixture), nil }
		return acquisitionResponse(http.StatusNotFound, []byte("missing")), nil
	})}
	resolver := &icloudshortcut.Resolver{Client: client, MaxBytes: int64(len(fixture) + 1024), RecordsBaseURL: "https://www.icloud.com/shortcuts/api/records/"}

	dir := t.TempDir()
	seed := filepath.Join(dir, "seed.txt")
	if err = os.WriteFile(seed, []byte("https://www.icloud.com/shortcuts/"+idA+"\nhttps://www.icloud.com/shortcuts/"+idB+"\n"), 0600); err != nil { t.Fatal(err) }
	inbox := filepath.Join(dir, "corpus-inbox")
	acquisitionPath := filepath.Join(dir, "corpus-acquisition-state.json")
	stats, err := collect(collectConfig{Sources: "seed", SeedPath: seed, Inbox: inbox, StatePath: acquisitionPath, Resolver: resolver})
	if err != nil { t.Fatal(err) }
	if stats.Downloaded != 2 || stats.DistinctContent != 1 || stats.DuplicateContent != 1 { t.Fatalf("unexpected acquisition stats: %+v", stats) }

	acquisition, err := loadAcquisitionState(acquisitionPath); if err != nil { t.Fatal(err) }
	if len(acquisition.ICloud) != 2 || len(acquisition.Content) != 1 { t.Fatalf("expected two iCloud IDs pointing to one content record, got %+v", acquisition) }
	for _, content := range acquisition.Content {
		if len(content.ICloudIDs) != 2 { t.Fatalf("content provenance should preserve both iCloud IDs: %+v", content) }
		if _, statErr := os.Stat(content.Path); statErr != nil { t.Fatalf("raw plist missing: %v", statErr) }
	}

	semanticState := filepath.Join(dir, "corpus-state.json")
	analysisDir := filepath.Join(dir, "analysis")
	catalogPath := filepath.Join("testdata", "catalog-fixture.json")
	if err = analyze(analyzeConfig{inputs: []string{inbox}, outputDir: analysisDir, statePath: semanticState, catalogPath: catalogPath}); err != nil { t.Fatal(err) }
	state, err := loadState(semanticState); if err != nil { t.Fatal(err) }
	for _, record := range state.Actions {
		if record.Count != 1 { t.Fatalf("identical plist content must contribute one semantic sample, got %+v", record) }
	}
	queueBefore, err := os.ReadFile(filepath.Join(analysisDir, reportAgentQueueJSON)); if err != nil { t.Fatal(err) }
	if !strings.Contains(string(queueBefore), "is.workflow.actions.mysteryaction") { t.Fatal("expected mystery action in initial agent queue") }

	updatedCatalog := filepath.Join(dir, "catalog-updated.json")
	catalog := `{"ok":true,"version":"2","actions":[{"name":"comment","shortcutIdentifier":"is.workflow.actions.comment","parameters":[{"name":"text","key":"WFCommentActionText","type":"text"}]},{"name":"alert","shortcutIdentifier":"is.workflow.actions.alert","parameters":[{"name":"alert","key":"WFAlertActionTitle","type":"text"},{"name":"showCancel","key":"WFAlertActionCancelButtonShown","type":"bool"}]},{"name":"mystery","shortcutIdentifier":"is.workflow.actions.mysteryaction","parameters":[{"name":"input","key":"WFInput","type":"text"},{"name":"option","key":"WFOption","type":"text"}]}]}`
	if err = os.WriteFile(updatedCatalog, []byte(catalog), 0600); err != nil { t.Fatal(err) }
	afterDir := filepath.Join(dir, "after")
	if err = reclassifyExisting(analyzeConfig{outputDir: afterDir, statePath: semanticState, catalogPath: updatedCatalog}); err != nil { t.Fatal(err) }
	queueAfter, err := os.ReadFile(filepath.Join(afterDir, reportAgentQueueJSON)); if err != nil { t.Fatal(err) }
	if strings.Contains(string(queueAfter), "is.workflow.actions.mysteryaction") { t.Fatal("resolved action must disappear after catalog-only reclassification") }
}

func TestSourceURLAllowlist(t *testing.T) {
	allowed := []string{"routinehub.co", "www.routinehub.co"}
	if err := validateSourceURL("https://routinehub.co/feed/", allowed); err != nil { t.Fatal(err) }
	for _, raw := range []string{"http://routinehub.co/feed/", "https://example.com/feed/", "https://routinehub.co.example.com/feed/"} {
		if err := validateSourceURL(raw, allowed); err == nil { t.Fatalf("expected source URL rejection: %s", raw) }
	}
}

func TestMergeProvenanceDoesNotDuplicateSources(t *testing.T) {
	state := newAcquisitionState()
	state.Content["sha"] = &acquisitionContent{SHA256: "sha"}
	mergeProvenance(state, "sha", "routinehub", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	mergeProvenance(state, "sha", "shortcutsbench", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	mergeProvenance(state, "sha", "routinehub", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	content := state.Content["sha"]
	if len(content.Sources) != 2 || len(content.ICloudIDs) != 2 { t.Fatalf("provenance should merge unique sources and iCloud IDs: %+v", content) }
}
