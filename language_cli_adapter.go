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
	// 1. --capabilities-json
	if args.Using("capabilities-json") {
		caps := protocol.BuildCapabilities(schema.DefaultRegistry())
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(caps)
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

	// 3. format <file> [--write]
	isFormat := (len(os.Args) > 1 && os.Args[1] == "format") || (args.Using("format") && args.Value("format") != "")
	var fmtPath string
	if len(os.Args) > 1 && os.Args[1] == "format" {
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
