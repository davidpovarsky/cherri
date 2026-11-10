/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/icloudshortcut"
	"github.com/electrikmilk/cherri/internal/language/migrate"
)

var importPath string

type ImportOptions struct {
	InputPath  string
	OutputPath string
	Mode       string // "legacy" or "v2"
}

type ImportResult struct {
	OutputPath   string
	LegacySource string
	FinalSource  string
	Mode         string
}

func ExecuteImportPipeline(opts ImportOptions) (*ImportResult, error) {
	if opts.Mode == "" {
		opts.Mode = "v2"
	}
	shortcutBytes := importShortcut(opts.InputPath)
	if opts.OutputPath != "" {
		outputPath = opts.OutputPath
		if args.Args == nil {
			args.Args = make(map[string]string)
		}
		args.Args["output"] = opts.OutputPath
	}
	decompile(shortcutBytes)

	decompContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read decompiled output at %s: %w", outputPath, err)
	}
	legacySrc := string(decompContent)
	finalSrc := legacySrc

	if strings.ToLower(opts.Mode) == "v2" {
		migrated, _, migErr := migrate.MigrateSourceWithDiagnostics(legacySrc)
		if migErr == nil && migrated != "" {
			finalSrc = migrated
			_ = os.WriteFile(outputPath, []byte(migrated), 0644)
		} else {
			migrated2 := migrate.MigrateSource(legacySrc)
			if migrated2 != "" {
				finalSrc = migrated2
				_ = os.WriteFile(outputPath, []byte(migrated2), 0644)
			}
		}
	}

	return &ImportResult{
		OutputPath:   outputPath,
		LegacySource: legacySrc,
		FinalSource:  finalSrc,
		Mode:         opts.Mode,
	}, nil
}

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
	filename = filepath.Base(importPath)
	var ext = strings.ToLower(filepath.Ext(importPath))
	basename = strings.TrimSuffix(filename, filepath.Ext(importPath))
	if ext != ".shortcut" && ext != ".plist" && ext != ".json" && ext != ".xml" {
		exit("import: File is not a Shortcut, plist, XML, or JSON file.")
	}
	relativePath = importPath[:len(importPath)-len(filename)]
	var b, readErr = os.ReadFile(importPath)
	handle(readErr)
	return b
}

func hasSignedBytes(b []byte) bool {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return false
	}
	if len(b) < 8 { return true }
	var rawUnsignedBytes = []byte{98, 112, 108, 105, 115, 116, 48, 48}
	var unsignedBytes = []byte{60, 63, 120, 109, 108, 32, 118, 101}
	for i, ub := range unsignedBytes {
		if ub == b[i] || b[i] == rawUnsignedBytes[i] { continue }
		return true
	}
	return false
}
