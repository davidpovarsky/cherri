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
)

// TestForkActionProvenanceRegistry guards the structural integrity of the
// fork provenance registry so provenance cannot silently rot.
func TestForkActionProvenanceRegistry(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("docs", "fork-action-provenance.json"))
	if err != nil {
		t.Fatalf("read provenance registry: %v", err)
	}

	var registry struct {
		Version  int `json:"version"`
		Upstream struct {
			Repository  string `json:"repository"`
			BaselineSha string `json:"baselineSha"`
		} `json:"upstream"`
		Actions []struct {
			Name             string   `json:"name"`
			Identifier       string   `json:"identifier"`
			ChangeType       string   `json:"changeType"`
			UpstreamStatus   string   `json:"upstreamStatus"`
			BaselineSha      string   `json:"baselineSha"`
			UpstreamLocation string   `json:"upstreamLocation"`
			ForkLocations    []string `json:"forkLocations"`
			Evidence         string   `json:"evidence"`
		} `json:"actions"`
	}
	if err = json.Unmarshal(data, &registry); err != nil {
		t.Fatalf("invalid provenance JSON: %v", err)
	}

	if registry.Version < 1 {
		t.Fatal("missing registry version")
	}
	if !strings.HasPrefix(registry.Upstream.Repository, "https://github.com/electrikmilk/cherri") {
		t.Fatalf("unexpected upstream repository %q", registry.Upstream.Repository)
	}
	if len(registry.Upstream.BaselineSha) != 40 {
		t.Fatalf("upstream baselineSha must be a full SHA, got %q", registry.Upstream.BaselineSha)
	}

	changeTypes := map[string]bool{"added-by-fork": true, "modified-by-fork": true}
	upstreamStatuses := map[string]bool{
		"absent-upstream":            true,
		"present-upstream-extended":  true,
		"present-upstream-corrected": true,
	}

	names := map[string]bool{}
	for _, entry := range registry.Actions {
		if entry.Name == "" || entry.Identifier == "" {
			t.Fatalf("entry missing name/identifier: %+v", entry)
		}
		if names[entry.Name] {
			t.Errorf("duplicate provenance entry for %s", entry.Name)
		}
		names[entry.Name] = true

		if !changeTypes[entry.ChangeType] {
			t.Errorf("%s: invalid changeType %q", entry.Name, entry.ChangeType)
		}
		if !upstreamStatuses[entry.UpstreamStatus] {
			t.Errorf("%s: invalid upstreamStatus %q", entry.Name, entry.UpstreamStatus)
		}
		if len(entry.BaselineSha) != 40 {
			t.Errorf("%s: baselineSha must be a full SHA", entry.Name)
		}
		if entry.Evidence == "" {
			t.Errorf("%s: evidence is required", entry.Name)
		}
		if len(entry.ForkLocations) == 0 {
			t.Errorf("%s: at least one fork location is required", entry.Name)
		}
		for _, location := range entry.ForkLocations {
			if _, statErr := os.Stat(location); statErr != nil {
				t.Errorf("%s: fork location does not exist: %s", entry.Name, location)
			}
		}
	}
}
