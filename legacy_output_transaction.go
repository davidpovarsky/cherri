package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/electrikmilk/args-parser"
)

// The caller handles the returned error only after private staging cleanup has
// completed. The source sibling is an output only when it was actually selected.
func writeLegacyShortcutOutput(destination string) error {
	var relativeFile = fmt.Sprintf("%s%s", relativePath, workflowName)
	if args.Using("debug") {
		if err := writeShortcut(relativeFile+".plist", workflowName+".plist"); err != nil {
			return err
		}
	}
	if args.Using("skip-sign") {
		var unsignedPath = relativeFile + unsignedEnd
		if hasExplicitOutput() {
			unsignedPath = destination
		}
		return writeShortcut(unsignedPath, workflowName+unsignedEnd)
	}

	var directory, err = os.MkdirTemp(filepath.Dir(destination), "cherri-compile-*")
	if err != nil {
		return fmt.Errorf("failed to create compilation staging directory: %w", err)
	}
	defer os.RemoveAll(directory)
	var unsignedPath = filepath.Join(directory, filepath.Base(workflowName)+unsignedEnd)
	if err := writeShortcut(unsignedPath, workflowName+unsignedEnd); err != nil {
		return err
	}
	var mode = "people-who-know-me"
	if args.Using("share") && args.Value("share") == "anyone" {
		mode = "anyone"
	}
	return SignShortcut(unsignedPath, destination, mode)
}
