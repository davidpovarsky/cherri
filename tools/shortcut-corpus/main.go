/*
 * Copyright (c) Cherri
 */

// Command shortcut-corpus ingests real Apple Shortcuts JSON/plist files,
// normalizes them into privacy-safe structural records, deduplicates them by
// file hash and action fingerprint, classifies them against Cherri's
// machine-readable action catalog, and emits concise reports for review.
//
// The analyzer intentionally treats Cherri as a black box: it consumes only
// the `cherri --actions-json` catalog, never compiler internals.
package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `Shortcut corpus analyzer

Usage:
  shortcut-corpus analyze [flags] -in <file|dir> [-in <file|dir> ...]
  shortcut-corpus compare <a.shortcut> <b.shortcut> [--json]

Analyze flags:
  -in path        Input Shortcut JSON/plist file or directory (repeatable, recursive)
  -out dir        Report output directory (default ./analysis)
  -state file     Incremental processing state (default ./corpus-state.json)
  -catalog file   Cherri action catalog JSON; overrides -cherri
  -cherri path    Cherri binary used to produce the catalog via --actions-json
                  (default: $CHERRI_BIN or "cherri")
  -max-files n    Stop after n new files (0 = unlimited)
  -force          Re-analyze files already present in state
  -no-candidates  Skip candidate definition generation

Reports are written to the output directory. Raw input is never copied or committed.
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	switch os.Args[1] {
	case "analyze":
		runAnalyze(os.Args[2:])
	case "compare":
		runCompare(os.Args[2:])
	case "-h", "--help", "help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command %q\n\n%s", os.Args[1], usage)
		os.Exit(2)
	}
}

type analyzeConfig struct {
	inputs       []string
	outputDir    string
	statePath    string
	catalogPath  string
	cherriBin    string
	maxFiles     int
	force        bool
	noCandidates bool
}

func analyzeFlags(args []string) (analyzeConfig, *flag.FlagSet) {
	var config analyzeConfig
	flags := flag.NewFlagSet("analyze", flag.ExitOnError)
	flags.Func("in", "input file or directory (repeatable)", func(value string) error {
		config.inputs = append(config.inputs, value)
		return nil
	})
	flags.StringVar(&config.outputDir, "out", "./analysis", "report output directory")
	flags.StringVar(&config.statePath, "state", "./corpus-state.json", "incremental processing state")
	flags.StringVar(&config.catalogPath, "catalog", "", "Cherri action catalog JSON file")
	flags.StringVar(&config.cherriBin, "cherri", defaultCherriBin(), "Cherri binary for catalog generation")
	flags.IntVar(&config.maxFiles, "max-files", 0, "stop after n new files")
	flags.BoolVar(&config.force, "force", false, "re-analyze known files")
	flags.BoolVar(&config.noCandidates, "no-candidates", false, "skip candidate generation")
	flags.Parse(args)
	return config, flags
}

func runAnalyze(argv []string) {
	config, flags := analyzeFlags(argv)
	if len(config.inputs) == 0 || flags.NArg() != 0 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	if err := analyze(config); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func runCompare(argv []string) {
	flags := flag.NewFlagSet("compare", flag.ExitOnError)
	asJSON := flags.Bool("json", false, "emit machine-readable JSON result")
	flags.Parse(argv)
	if flags.NArg() != 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	result, err := CompareShortcuts(flags.Arg(0), flags.Arg(1))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	printComparison(result, *asJSON)
	if !result.Equal {
		os.Exit(1)
	}
}
