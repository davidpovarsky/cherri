package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceProductionPathsExist(t *testing.T) {
	for _, path := range workspaceProductionPaths {
		info, err := os.Stat(filepath.Join(resolveRepoRoot(), path))
		if err != nil || !info.Mode().IsRegular() {
			t.Errorf("workspace producer points to absent/non-file production path %s: %v", path, err)
		}
	}
}
