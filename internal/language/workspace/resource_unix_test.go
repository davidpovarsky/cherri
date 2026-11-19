//go:build !windows

package workspace

import (
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestRootedRelativeSymlinkWithinWorkspace(t *testing.T) {
	directory := t.TempDir()
	entry := filepath.Join(directory, "entry.cherri")
	writeFixture(t, entry, "// public fixture")
	writeFixture(t, filepath.Join(directory, "assets", "value.txt"), "RELATIVE_LINK_ALLOWED")
	if err := os.Symlink("assets", filepath.Join(directory, "relative-link")); err != nil {
		t.Fatal(err)
	}
	root, err := Open(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	resource, err := root.Open("entry.cherri", "relative-link/value.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer resource.Close()
	raw, err := io.ReadAll(resource.File)
	if err != nil || string(raw) != "RELATIVE_LINK_ALLOWED" {
		t.Fatalf("authorized relative link did not preserve native bytes: %q %v", raw, err)
	}
}
