package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Normal package tests must inspect the same repository inventory as the CLI gate.
func TestGate_RepositoryFixtureInventory(t *testing.T) {
	var fixturesDir = filepath.Join(findRepoRoot(), "tests")
	var manifestPath = filepath.Join(fixturesDir, "fixtures-manifest.json")
	var data, err = os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	if err := json.Unmarshal(data, &names); err != nil {
		t.Fatalf("invalid canonical fixture manifest: %v", err)
	}
	if len(names) == 0 {
		t.Fatal("canonical fixture manifest is empty")
	}
	var listed = make(map[string]bool, len(names))
	for _, name := range names {
		if filepath.Base(name) != name || !strings.HasSuffix(name, ".cherri") {
			t.Errorf("manifest entry %q is not a root .cherri fixture", name)
			continue
		}
		if listed[name] {
			t.Errorf("canonical fixture %q is listed more than once", name)
		}
		listed[name] = true
		var info, err = os.Stat(filepath.Join(fixturesDir, name))
		if err != nil {
			t.Errorf("manifest fixture %q is missing: %v", name, err)
		} else if !info.Mode().IsRegular() {
			t.Errorf("manifest fixture %q is not a regular file", name)
		}
	}
	var fixtures, globErr = filepath.Glob(filepath.Join(fixturesDir, "*.cherri"))
	if globErr != nil {
		t.Fatal(globErr)
	}
	if len(fixtures) == 0 {
		t.Fatal("repository contains no root .cherri fixtures")
	}
	for _, fixture := range fixtures {
		var name = filepath.Base(fixture)
		if !listed[name] {
			t.Errorf("repository fixture %q is not in the canonical manifest", name)
		}
	}
}
