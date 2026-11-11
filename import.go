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
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

var importPath string

type ImportOptions struct {
	InputPath  string
	OutputPath string
	Mode       string // "legacy" or "v2"
	Fidelity   string // "lossless", "hybrid", or "semantic"
}

type ImportResult struct {
	OutputPath   string
	LegacySource string
	FinalSource  string
	Mode         string
	Fidelity     string
	Plan         *HybridImportPlan
	Document     *NativeShortcutDocument
}

func ExecuteImportPipeline(opts ImportOptions) (*ImportResult, error) {
	mode := strings.ToLower(strings.TrimSpace(opts.Mode))
	if mode == "" {
		mode = "v2"
	}
	if mode != "legacy" && mode != "v2" {
		return nil, fmt.Errorf("invalid import mode %q: expected 'legacy' or 'v2'", opts.Mode)
	}

	fidelity := strings.ToLower(strings.TrimSpace(opts.Fidelity))
	if fidelity == "" {
		fidelity = "lossless"
	}
	if fidelity != "lossless" && fidelity != "hybrid" && fidelity != "semantic" {
		return nil, fmt.Errorf("invalid import fidelity %q: expected 'lossless', 'hybrid', or 'semantic'", opts.Fidelity)
	}

	if args.Args == nil {
		args.Args = make(map[string]string)
	}
	prevFidelity, hadFidelity := args.Args["fidelity"]
	prevOutput, hadOutput := args.Args["output"]
	resetCompilerStateFully()
	defer func() {
		resetCompilerStateFully()
		if hadFidelity {
			args.Args["fidelity"] = prevFidelity
		} else {
			delete(args.Args, "fidelity")
		}
		if hadOutput {
			args.Args["output"] = prevOutput
		} else {
			delete(args.Args, "output")
		}
	}()

	shortcutBytes, err := readShortcutBytesForPipeline(opts.InputPath)
	if err != nil {
		return nil, err
	}

	doc, err := DecodeNativeShortcutDocument(shortcutBytes)
	if err != nil {
		return nil, fmt.Errorf("decode shortcut %s: %w", opts.InputPath, err)
	}

	if opts.OutputPath != "" {
		outputPath = opts.OutputPath
		args.Args["output"] = opts.OutputPath
	} else {
		delete(args.Args, "output")
	}

	resolvedOut := opts.OutputPath
	if resolvedOut == "" {
		resolvedOut = getOutputPath(basename + ".cherri")
	}

	if fidelity == "lossless" {
		outputPath = resolvedOut
		var legacySrc string
		var finalSrc string
		if mode == "legacy" {
			legacySrc, err = RenderLosslessLegacySourceFromDocument(doc)
			if err != nil {
				return nil, fmt.Errorf("render lossless legacy source: %w", err)
			}
			finalSrc = legacySrc
		} else {
			finalSrc, err = RenderLosslessV2SourceFromDocument(doc)
			if err != nil {
				return nil, fmt.Errorf("render lossless v2 source: %w", err)
			}
		}

		if writeErr := os.WriteFile(resolvedOut, []byte(finalSrc), 0644); writeErr != nil {
			return nil, fmt.Errorf("write lossless import output %s: %w", resolvedOut, writeErr)
		}

		return &ImportResult{
			OutputPath:   resolvedOut,
			LegacySource: legacySrc,
			FinalSource:  finalSrc,
			Mode:         mode,
			Fidelity:     "lossless",
			Document:     doc,
		}, nil
	}

	if fidelity == "hybrid" {
		outputPath = resolvedOut
		plan, err := PlanHybridImport(doc)
		if err != nil {
			return nil, fmt.Errorf("plan hybrid import: %w", err)
		}

		var legacySrc string
		var finalSrc string
		if mode == "legacy" {
			legacySrc, err = RenderHybridLegacySource(doc, plan)
			if err != nil {
				return nil, fmt.Errorf("render hybrid legacy source: %w", err)
			}
			finalSrc = legacySrc
		} else {
			finalSrc, err = RenderHybridV2Source(doc, plan)
			if err != nil {
				return nil, fmt.Errorf("render hybrid v2 source: %w", err)
			}
		}

		if writeErr := os.WriteFile(resolvedOut, []byte(finalSrc), 0644); writeErr != nil {
			return nil, fmt.Errorf("write hybrid import output %s: %w", resolvedOut, writeErr)
		}

		return &ImportResult{
			OutputPath:   resolvedOut,
			LegacySource: legacySrc,
			FinalSource:  finalSrc,
			Mode:         mode,
			Fidelity:     "hybrid",
			Plan:         plan,
			Document:     doc,
		}, nil
	}

	args.Args["fidelity"] = "semantic"
	if err := runSemanticDecompileSafely(shortcutBytes); err != nil {
		return nil, err
	}

	decompContent, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read decompiled output at %s: %w", outputPath, err)
	}
	legacySrc := string(decompContent)
	finalSrc := legacySrc

	if mode == "v2" {
		finalSrc, err = MigrateSemanticLegacySourceToV2Strict(legacySrc)
		if err != nil {
			return nil, err
		}
		if writeErr := os.WriteFile(outputPath, []byte(finalSrc), 0644); writeErr != nil {
			return nil, fmt.Errorf("write semantic v2 import output %s: %w", outputPath, writeErr)
		}
	}

	return &ImportResult{
		OutputPath:   outputPath,
		LegacySource: legacySrc,
		FinalSource:  finalSrc,
		Mode:         mode,
		Fidelity:     "semantic",
		Document:     doc,
	}, nil
}

// MigrateSemanticLegacySourceToV2Strict migrates semantic-decompiled Legacy source to Cherri v2
// and validates that the migrated source parses cleanly as V2 syntax without silent fallback.
func MigrateSemanticLegacySourceToV2Strict(legacySrc string) (string, error) {
	migrated, diags, migErr := migrate.MigrateSourceWithDiagnostics(legacySrc)
	if migErr != nil {
		if len(diags) > 0 {
			return "", fmt.Errorf("semantic v2 migration failed at %d:%d: %s (%w)", diags[0].Line, diags[0].Column, diags[0].Message, migErr)
		}
		return "", fmt.Errorf("semantic v2 migration failed: %w", migErr)
	}
	file := source.NewFile(source.DocumentID("semantic_import.cherri"), "semantic_import.cherri", 1, migrated)
	p := syntax.NewParser(file)
	_ = p.ParseProgram()
	if len(p.Errors()) > 0 {
		firstErr := p.Errors()[0]
		return "", fmt.Errorf("semantic v2 migration produced invalid v2 syntax at %d:%d: %s", firstErr.Span.Start.Line, firstErr.Span.Start.Column, firstErr.Message)
	}
	return migrated, nil
}

func runSemanticDecompileSafely(shortcutBytes []byte) (err error) {
	prevEmbedded := embeddedCompilerMode
	embeddedCompilerMode = true
	defer func() {
		embeddedCompilerMode = prevEmbedded
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("semantic decompile error: %s", embeddedErrorMessage(recovered))
		}
	}()
	decompile(shortcutBytes)
	return nil
}

func readShortcutBytesForPipeline(path string) ([]byte, error) {
	importPath = path
	icloudURLRegex := regexp.MustCompile(`^https://(?:www.)?icloud\.com/shortcuts/.+$`)
	var shortcutBytes []byte
	if icloudURLRegex.MatchString(importPath) {
		resolver := icloudshortcut.NewResolver()
		result, err := resolver.Resolve(context.Background(), importPath)
		if err != nil {
			return nil, fmt.Errorf("import: Failed to download Shortcut from iCloud: %w", err)
		}
		basename = result.Name
		if basename == "" {
			basename = result.ID
		}
		if args.Using("debug") {
			_ = os.WriteFile(basename+"_decompile.plist", result.Data, 0600)
		}
		shortcutBytes = result.Data
	} else {
		if _, statErr := os.Stat(importPath); os.IsNotExist(statErr) {
			return nil, fmt.Errorf("import: File does not exist: %s", importPath)
		}
		filename = filepath.Base(importPath)
		ext := strings.ToLower(filepath.Ext(importPath))
		basename = strings.TrimSuffix(filename, filepath.Ext(importPath))
		if ext != ".shortcut" && ext != ".plist" && ext != ".json" && ext != ".xml" {
			return nil, fmt.Errorf("import: File is not a Shortcut, plist, XML, or JSON file: %s", importPath)
		}
		relativePath = importPath[:len(importPath)-len(filename)]
		b, readErr := os.ReadFile(importPath)
		if readErr != nil {
			return nil, fmt.Errorf("import: Failed to read %s: %w", importPath, readErr)
		}
		shortcutBytes = b
	}

	if len(shortcutBytes) >= 4 && string(shortcutBytes[:4]) == "AEA1" {
		return nil, fmt.Errorf("import: Signed Shortcuts (AEA1 container) are not supported; provide an unsigned Shortcut plist/JSON or iCloud link")
	}
	if hasSignedBytes(shortcutBytes) {
		return nil, fmt.Errorf("import: Signed Shortcuts are currently not supported :(\nYou can use an iCloud link instead by sharing the Shortcut and selecting \"Copy iCloud Link\"")
	}
	return shortcutBytes, nil
}

// Imports a Shortcut for decompilation based on path given.
func importShortcut(path string) (shortcutBytes []byte) {
	b, err := readShortcutBytesForPipeline(path)
	if err != nil {
		exit(err.Error())
	}
	return b
}

func downloadShortcut() []byte {
	resolver := icloudshortcut.NewResolver()
	result, err := resolver.Resolve(context.Background(), importPath)
	if err != nil {
		exit("import: Failed to download Shortcut from iCloud: " + err.Error())
	}
	basename = result.Name
	if basename == "" {
		basename = result.ID
	}
	if args.Using("debug") {
		writeErr := os.WriteFile(basename+"_decompile.plist", result.Data, 0600)
		handle(writeErr)
	}
	return result.Data
}

func readShortcutFile() []byte {
	var _, statErr = os.Stat(importPath)
	if os.IsNotExist(statErr) {
		exit("import: File does not exist!")
	}
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
	if len(b) < 8 {
		return true
	}
	var rawUnsignedBytes = []byte{98, 112, 108, 105, 115, 116, 48, 48}
	var unsignedBytes = []byte{60, 63, 120, 109, 108, 32, 118, 101}
	for i, ub := range unsignedBytes {
		if ub == b[i] || b[i] == rawUnsignedBytes[i] {
			continue
		}
		return true
	}
	return false
}
