package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// Migrated sources live outside the repository. Stage only the public dependency
// required by the unchanged fixture; compiler workspace access stays fail-closed.
func stageFixtureAssets(name, workspace, repoRoot string) error {
	if name != "menus.cherri" {
		return nil
	}
	var relative = filepath.Join("assets", "cherri_icon.png")
	var data, err = os.ReadFile(filepath.Join(repoRoot, relative))
	if err != nil {
		return fmt.Errorf("read canonical fixture asset %s: %w", relative, err)
	}
	if len(data) == 0 {
		return fmt.Errorf("canonical fixture asset %s is empty", relative)
	}
	var destination = filepath.Join(workspace, relative)
	if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
		return fmt.Errorf("prepare fixture asset directory: %w", err)
	}
	if err := os.WriteFile(destination, data, 0600); err != nil {
		return fmt.Errorf("stage canonical fixture asset %s: %w", relative, err)
	}
	return nil
}
