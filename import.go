/*
 * Copyright (c) Cherri
 */

package main

import (
	"context"
	"os"
	"regexp"
	"strings"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/icloudshortcut"
)

var importPath string

// Imports a Shortcut for decompilation based on path given.
func importShortcut(path string) (shortcutBytes []byte) {
	importPath = path
	var icloudURLRegex = regexp.MustCompile(`^https://(?:www.)?icloud\.com/shortcuts/.+$`)
	if icloudURLRegex.MatchString(importPath) {
		shortcutBytes = downloadShortcut()
	} else {
		shortcutBytes = readShortcutFile()
	}

	if hasSignedBytes(shortcutBytes) {
		exit("import: Signed Shortcuts are currently not supported :(\nYou can use an iCloud link instead by sharing the Shortcut and selecting \"Copy iCloud Link\".")
	}
	return
}

func downloadShortcut() []byte {
	resolver := icloudshortcut.NewResolver()
	result, err := resolver.Resolve(context.Background(), importPath)
	if err != nil {
		exit("import: Failed to download Shortcut from iCloud: " + err.Error())
	}
	basename = result.Name
	if basename == "" { basename = result.ID }
	if args.Using("debug") {
		writeErr := os.WriteFile(basename+"_decompile.plist", result.Data, 0600)
		handle(writeErr)
	}
	return result.Data
}

func readShortcutFile() []byte {
	var _, statErr = os.Stat(importPath)
	if os.IsNotExist(statErr) { exit("import: File does not exist!") }
	var segments = strings.Split(importPath, "/")
	filename = segments[len(segments)-1]
	var nameSegments = strings.Split(filename, ".")
	basename = nameSegments[0]
	var extension = nameSegments[len(nameSegments)-1]
	if extension != "shortcut" && extension != "plist" { exit("import: File is not a Shortcut or property list (plist) file.") }
	relativePath = strings.Replace(importPath, filename, "", 1)
	var b, readErr = os.ReadFile(importPath)
	handle(readErr)
	return b
}

func hasSignedBytes(b []byte) bool {
	if len(b) < 8 { return true }
	var rawUnsignedBytes = []byte{98, 112, 108, 105, 115, 116, 48, 48}
	var unsignedBytes = []byte{60, 63, 120, 109, 108, 32, 118, 101}
	for i, ub := range unsignedBytes {
		if ub == b[i] || b[i] == rawUnsignedBytes[i] { continue }
		return true
	}
	return false
}
