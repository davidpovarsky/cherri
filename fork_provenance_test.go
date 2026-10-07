/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestForkActionProvenanceRegistry guards the structural integrity of the
// fork provenance registry so provenance cannot silently rot.
//
// Every finalized entry must reference a real firstForkCommit. When a .git
// directory is available the referenced commits are additionally verified to
// exist and be ancestors of HEAD; in source archives without .git only the
// SHA shape is checked so normal unit tests never depend on Git history.
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
			FirstForkCommit  *string  `json:"firstForkCommit"`
			Evidence         string   `json:"evidence"`
		} `json:"actions"`
		Infrastructure []struct {
			Name            string   `json:"name"`
			Locations       []string `json:"locations"`
			FirstForkCommit *string  `json:"firstForkCommit"`
		} `json:"infrastructure"`
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
	assertRecordedCommit(t, "upstream.baselineSha", registry.Upstream.BaselineSha)

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
		if entry.FirstForkCommit == nil || *entry.FirstForkCommit == "" {
			t.Errorf("%s: firstForkCommit is required once implemented; record it in a provenance-metadata commit after the implementation commit", entry.Name)
			continue
		}
		assertRecordedCommit(t, entry.Name+".firstForkCommit", *entry.FirstForkCommit)
	}

	for _, infra := range registry.Infrastructure {
		if infra.Name == "" {
			t.Fatal("infrastructure entry missing name")
		}
		if len(infra.Locations) == 0 {
			t.Errorf("infrastructure %s: at least one location is required", infra.Name)
		}
		if infra.FirstForkCommit == nil || *infra.FirstForkCommit == "" {
			t.Errorf("infrastructure %s: firstForkCommit is required once implemented", infra.Name)
			continue
		}
		assertRecordedCommit(t, "infrastructure:"+infra.Name, *infra.FirstForkCommit)
	}
}

var fullShaPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)

// assertRecordedCommit validates the SHA shape of a recorded fork/upstream
// commit and, when this checkout contains Git history, that the commit exists
// and is reachable from HEAD so stale or amended-away SHAs fail CI.
func assertRecordedCommit(t *testing.T, field string, sha string) {
	t.Helper()

	if !fullShaPattern.MatchString(sha) {
		t.Errorf("%s: %q must be a full 40-character lowercase hexadecimal SHA", field, sha)
		return
	}

	if _, statErr := os.Stat(".git"); statErr != nil {
		// Source archive without Git history: shape checks only.
		return
	}

	gitArgs := func(probe ...string) bool {
		cmd := exec.Command("git", probe...)
		return cmd.Run() == nil
	}
	if !gitArgs("cat-file", "-e", sha+"^{commit}") {
		t.Errorf("%s: commit %s does not exist in this repository (amended away or fabricated?)", field, sha)
		return
	}
	if !gitArgs("merge-base", "--is-ancestor", sha, "HEAD") {
		t.Errorf("%s: commit %s exists but is not an ancestor of HEAD", field, sha)
	}
}
