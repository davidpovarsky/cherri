/*
 * Copyright (c) Cherri
 */

package main

import (
	"flag"
	"fmt"
	"os"
)

const usage = `Shortcut corpus analyzer

Usage:
  shortcut-corpus analyze [flags] -in <file|dir> [-in <file|dir> ...]
  shortcut-corpus reclassify [flags]
  shortcut-corpus compare <a.shortcut> <b.shortcut> [--json]
  shortcut-corpus sources
  shortcut-corpus collect [flags]
  shortcut-corpus session [flags]

Common analysis flags:
  -out dir              Report output directory (default ./analysis)
  -state file           Incremental semantic state (default ./corpus-state.json)
  -catalog file         Cherri action catalog JSON; overrides -cherri
  -cherri path          Cherri binary used to produce --actions-json
  -max-actionable n     Limit agent queue output (0 = unlimited)
  -include-third-party  Include third-party actions in agent queue

Raw corpus input and acquisition state are environment artifacts and must never be committed.
`

func main() {
	if len(os.Args) < 2 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	switch os.Args[1] {
	case "analyze": runAnalyze(os.Args[2:])
	case "reclassify": runReclassify(os.Args[2:])
	case "compare": runCompare(os.Args[2:])
	case "sources": runSources(os.Args[2:])
	case "collect": runCollect(os.Args[2:])
	case "session": runSession(os.Args[2:])
	case "-h", "--help", "help": fmt.Print(usage)
	default: fmt.Fprintf(os.Stderr, "Unknown command %q\n\n%s", os.Args[1], usage); os.Exit(2)
	}
}

type analyzeConfig struct {
	inputs []string
	outputDir string
	statePath string
	catalogPath string
	cherriBin string
	maxFiles int
	force bool
	noCandidates bool
	maxActionable int
	includeThirdParty bool
}

func addAnalyzeFlags(flags *flag.FlagSet, config *analyzeConfig, allowInput bool) {
	if allowInput {
		flags.Func("in", "input file or directory (repeatable)", func(value string) error { config.inputs = append(config.inputs, value); return nil })
	}
	flags.StringVar(&config.outputDir, "out", "./analysis", "report output directory")
	flags.StringVar(&config.statePath, "state", "./corpus-state.json", "incremental semantic state")
	flags.StringVar(&config.catalogPath, "catalog", "", "Cherri action catalog JSON file")
	flags.StringVar(&config.cherriBin, "cherri", defaultCherriBin(), "Cherri binary for catalog generation")
	flags.IntVar(&config.maxFiles, "max-files", 0, "stop after n files containing new schemas")
	flags.BoolVar(&config.force, "force", false, "reparse known files without double-counting evidence")
	flags.BoolVar(&config.noCandidates, "no-candidates", false, "skip candidate generation")
	flags.IntVar(&config.maxActionable, "max-actionable", 0, "limit agent queue items (0 = unlimited)")
	flags.BoolVar(&config.includeThirdParty, "include-third-party", false, "include third-party actions in agent queue")
}

func analyzeFlags(args []string) (analyzeConfig, *flag.FlagSet) {
	var config analyzeConfig
	flags := flag.NewFlagSet("analyze", flag.ExitOnError)
	addAnalyzeFlags(flags, &config, true); flags.Parse(args); return config, flags
}

func runAnalyze(argv []string) {
	config, flags := analyzeFlags(argv)
	if len(config.inputs) == 0 || flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	if err := analyze(config); err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
}

func runReclassify(argv []string) {
	var config analyzeConfig
	flags := flag.NewFlagSet("reclassify", flag.ExitOnError); addAnalyzeFlags(flags, &config, false); flags.Parse(argv)
	if flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	if err := reclassifyExisting(config); err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
}

func runCompare(argv []string) {
	flags := flag.NewFlagSet("compare", flag.ExitOnError); asJSON := flags.Bool("json", false, "emit machine-readable JSON result"); flags.Parse(argv)
	if flags.NArg() != 2 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }
	result, err := CompareShortcuts(flags.Arg(0), flags.Arg(1)); if err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
	printComparison(result, *asJSON); if !result.Equal { os.Exit(1) }
}
