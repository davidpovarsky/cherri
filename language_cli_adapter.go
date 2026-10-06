/*
 * Copyright (c) Cherri Language v2.0
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/electrikmilk/args-parser"
	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lsp"
	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func init() {
	args.Register(args.Argument{
		Name:        "capabilities-json",
		Description: "Print language v2.0 capabilities and schema metadata as JSON.",
	})
	args.Register(args.Argument{
		Name:         "action-schema-json",
		Description:  "Print action schema definition as JSON.",
		ExpectsValue: true,
	})
	args.Register(args.Argument{
		Name:         "check",
		Description:  "Analyze a Cherri v2 source file without compiling or signing.",
		ExpectsValue: true,
	})
	args.Register(args.Argument{
		Name:        "json",
		Description: "Output results in JSON format.",
	})
	args.Register(args.Argument{
		Name:         "format",
		Description:  "Format a Cherri v2 source file.",
		ExpectsValue: true,
	})
	args.Register(args.Argument{
		Name:        "write",
		Description: "Write formatted output back to file in-place.",
	})
}

// HandleLanguageV2CLI processes v2 CLI commands. Returns true if handled.
func HandleLanguageV2CLI() bool {
	// 0. compile <file> [--output <out>] [--skip-sign]
	if len(os.Args) > 1 && os.Args[1] == "compile" {
		var compPath string
		var outPath string
		skipSign := false
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if strings.HasPrefix(arg, "--output=") {
				outPath = strings.TrimPrefix(arg, "--output=")
			} else if arg == "-o" || arg == "--output" {
				if i+1 < len(os.Args) {
					outPath = os.Args[i+1]
					i++
				}
			} else if arg == "--skip-sign" {
				skipSign = true
			} else if !strings.HasPrefix(arg, "-") && compPath == "" {
				compPath = arg
			}
		}
		if compPath == "" {
			fmt.Fprintf(os.Stderr, "Usage: cherri compile <file.cherri> [--output <path>] [--skip-sign]\n")
			os.Exit(1)
		}
		if !darwin {
			skipSign = true
		}
		if err := CompileFileV2(compPath, outPath, skipSign); err != nil {
			fmt.Fprintf(os.Stderr, "Compilation error: %v\n", err)
			os.Exit(1)
		}
		return true
	}

	// 0b. import <file> [--output <out>] [--unsigned]
	if len(os.Args) > 1 && os.Args[1] == "import" {
		var importPath string
		var outPath string
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if strings.HasPrefix(arg, "--output=") {
				outPath = strings.TrimPrefix(arg, "--output=")
			} else if arg == "-o" || arg == "--output" {
				if i+1 < len(os.Args) {
					outPath = os.Args[i+1]
					i++
				}
			} else if !strings.HasPrefix(arg, "-") && importPath == "" {
				importPath = arg
			}
		}
		if importPath == "" {
			fmt.Fprintf(os.Stderr, "Usage: cherri import <file.shortcut> [--output <path>]\n")
			os.Exit(1)
		}
		shortcutBytes := importShortcut(importPath)
		if outPath != "" {
			outputPath = outPath
			if args.Args == nil {
				args.Args = make(map[string]string)
			}
			args.Args["output"] = outPath
		}
		decompile(shortcutBytes)
		if decompContent, err := os.ReadFile(outputPath); err == nil {
			migrated := migrate.MigrateSource(string(decompContent))
			_ = os.WriteFile(outputPath, []byte(migrated), 0644)
		}
		return true
	}

	// 0c. migrate <file> [--write] [--check]
	if len(os.Args) > 1 && os.Args[1] == "migrate" {
		var migPath string
		writeFlag := false
		checkFlag := false
		for i := 2; i < len(os.Args); i++ {
			arg := os.Args[i]
			if arg == "--write" || arg == "-w" {
				writeFlag = true
			} else if arg == "--check" || arg == "-c" {
				checkFlag = true
			} else if !strings.HasPrefix(arg, "-") && migPath == "" {
				migPath = arg
			}
		}
		if migPath == "" {
			fmt.Fprintf(os.Stderr, "Usage: cherri migrate <file.cherri> [--write] [--check]\n")
			os.Exit(1)
		}
		content, err := os.ReadFile(migPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", migPath, err)
			os.Exit(1)
		}
		migrated := migrate.MigrateSource(string(content))
		if checkFlag {
			file := source.NewFile(source.DocumentID(migPath), migPath, 1, migrated)
			p := syntax.NewParser(file)
			_ = p.ParseProgram()
			if len(p.Errors()) > 0 {
				fmt.Fprintf(os.Stderr, "Migration check failed for %s: %v\n", migPath, p.Errors())
				os.Exit(1)
			}
			fmt.Printf("File %s can be migrated cleanly.\n", migPath)
			return true
		}
		if writeFlag {
			if err := os.WriteFile(migPath, []byte(migrated), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing %s: %v\n", migPath, err)
				os.Exit(1)
			}
			fmt.Printf("Migrated and updated %s successfully.\n", migPath)
			return true
		}
		fmt.Print(migrated)
		return true
	}

	// 1. --capabilities-json
	if args.Using("capabilities-json") {
		caps := protocol.BuildCapabilities(schema.DefaultRegistry())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(caps)
		return true
	}

	// 1b. --action-schema-json[=actionName]
	if args.Using("action-schema-json") || (len(os.Args) > 1 && strings.HasPrefix(os.Args[1], "--action-schema-json")) {
		actName := args.Value("action-schema-json")
		if actName == "" {
			for _, arg := range os.Args {
				if strings.HasPrefix(arg, "--action-schema-json=") {
					actName = strings.TrimPrefix(arg, "--action-schema-json=")
				}
			}
		}
		reg := schema.DefaultRegistry()
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if actName != "" {
			act, ok := reg.LookupAction(actName)
			if !ok {
				fmt.Fprintf(os.Stderr, "Error: action %q not found in schema\n", actName)
				os.Exit(1)
			}
			_ = enc.Encode(act)
		} else {
			_ = enc.Encode(reg.AllActions())
		}
		return true
	}

	// 2. check <file> [--json]
	isCheck := (len(os.Args) > 1 && os.Args[1] == "check") || (args.Using("check") && args.Value("check") != "")
	var checkPath string
	if len(os.Args) > 1 && os.Args[1] == "check" {
		for i := 2; i < len(os.Args); i++ {
			if !strings.HasPrefix(os.Args[i], "-") {
				checkPath = os.Args[i]
				break
			}
		}
	} else if args.Using("check") {
		checkPath = args.Value("check")
	}

	if isCheck && checkPath != "" {
		content, err := os.ReadFile(checkPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", checkPath, err)
			os.Exit(1)
		}

		svc := service.NewService(schema.DefaultRegistry())
		uri := "file:///" + checkPath
		svc.OpenDocument(uri, 1, string(content))
		diags, fp := svc.Analyze(uri)

		hasErrors := false
		for _, d := range diags {
			if d.Severity == analysis.SeverityError {
				hasErrors = true
				break
			}
		}

		isJSON := args.Using("json") || strings.Contains(strings.Join(os.Args, " "), "--json")
		if isJSON {
			var wireDiags []protocol.DiagnosticItem
			for _, d := range diags {
				wireDiags = append(wireDiags, protocol.ToDiagnosticItem(d))
			}
			resp := protocol.AnalyzeResponse{
				URI:               uri,
				Version:           1,
				LanguageVersion:   schema.LanguageVersion,
				SchemaFingerprint: fp,
				Diagnostics:       wireDiags,
				Valid:             !hasErrors,
			}
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			_ = enc.Encode(resp)
		} else {
			if len(diags) == 0 {
				fmt.Printf("File %s is valid Cherri v2.0.\n", checkPath)
			} else {
				for _, d := range diags {
					fmt.Println(d.String())
				}
			}
		}

		if hasErrors {
			os.Exit(1)
		}
		return true
	}

	// 3. format / fmt <file> [--write] [--check]
	isFormat := (len(os.Args) > 1 && (os.Args[1] == "format" || os.Args[1] == "fmt")) || (args.Using("format") && args.Value("format") != "")
	var fmtPath string
	if len(os.Args) > 1 && (os.Args[1] == "format" || os.Args[1] == "fmt") {
		for i := 2; i < len(os.Args); i++ {
			if !strings.HasPrefix(os.Args[i], "-") {
				fmtPath = os.Args[i]
				break
			}
		}
	} else if args.Using("format") {
		fmtPath = args.Value("format")
	}

	if isFormat && fmtPath != "" {
		content, err := os.ReadFile(fmtPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading file %s: %v\n", fmtPath, err)
			os.Exit(1)
		}

		file := source.NewFile(source.DocumentID(fmtPath), fmtPath, 1, string(content))
		parser := syntax.NewParser(file)
		prog := parser.ParseProgram()
		if len(parser.Errors()) > 0 {
			fmt.Fprintf(os.Stderr, "Cannot format file with syntax errors: %s\n", parser.Errors()[0])
			os.Exit(1)
		}

		formatted := syntax.Format(prog)
		isCheckFmt := false
		for _, a := range os.Args {
			if a == "--check" {
				isCheckFmt = true
				break
			}
		}
		if isCheckFmt {
			if string(content) != formatted {
				fmt.Fprintf(os.Stderr, "File %s requires formatting\n", fmtPath)
				os.Exit(1)
			}
			return true
		}

		isWrite := args.Using("write") || strings.Contains(strings.Join(os.Args, " "), "--write")
		if isWrite {
			if err := os.WriteFile(fmtPath, []byte(formatted), 0644); err != nil {
				fmt.Fprintf(os.Stderr, "Error writing formatted file: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Formatted %s\n", fmtPath)
		} else {
			fmt.Print(formatted)
		}
		return true
	}

	// 4. lsp
	if len(os.Args) > 1 && os.Args[1] == "lsp" {
		srv := lsp.NewServer(os.Stdin, os.Stdout, schema.DefaultRegistry())
		if err := srv.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "LSP server error: %v\n", err)
			os.Exit(1)
		}
		return true
	}

	return false
}
