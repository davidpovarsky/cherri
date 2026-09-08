/*
 * Copyright (c) Cherri
 */

package main

import (
	"flag"
	"fmt"
	"os"
)

// runSession is the normal recurring agent entrypoint: acquire only fresh
// public evidence, analyze the ignored inbox incrementally, reclassify all
// stored evidence against today's Cherri catalog, and emit the agent queue.
func runSession(argv []string) {
	var collectConfig collectConfig
	var analyzeConfig analyzeConfig
	flags := flag.NewFlagSet("session", flag.ExitOnError)
	addCollectFlags(flags, &collectConfig)
	flags.StringVar(&analyzeConfig.outputDir, "out", "./analysis/latest", "analysis/report output directory")
	flags.StringVar(&analyzeConfig.statePath, "state", "./corpus-state.json", "incremental semantic state")
	flags.StringVar(&analyzeConfig.catalogPath, "catalog", "", "Cherri action catalog JSON file")
	flags.StringVar(&analyzeConfig.cherriBin, "cherri", defaultCherriBin(), "Cherri binary for catalog generation")
	flags.IntVar(&analyzeConfig.maxActionable, "max-actionable", 20, "maximum ranked agent queue items")
	flags.BoolVar(&analyzeConfig.includeThirdParty, "include-third-party", false, "include third-party actions in agent queue")
	flags.Parse(argv)
	if flags.NArg() != 0 { fmt.Fprint(os.Stderr, usage); os.Exit(2) }

	stats, collectErr := collect(collectConfig)
	if collectErr != nil { fmt.Fprintf(os.Stderr, "Acquisition error: %v\n", collectErr) }
	analyzeConfig.inputs = []string{collectConfig.Inbox}
	if _, statErr := os.Stat(collectConfig.Inbox); os.IsNotExist(statErr) {
		if mkdirErr := os.MkdirAll(collectConfig.Inbox, 0700); mkdirErr != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", mkdirErr); os.Exit(1) }
	}
	if err := analyze(analyzeConfig); err != nil { fmt.Fprintf(os.Stderr, "Error: %v\n", err); os.Exit(1) }
	fmt.Printf("Session acquisition: discovered=%d known=%d downloaded=%d distinct=%d duplicate-content=%d failed=%d\n", stats.Discovered, stats.KnownItems, stats.Downloaded, stats.DistinctContent, stats.DuplicateContent, stats.Failed)
	if collectErr != nil { fmt.Fprintln(os.Stderr, "Session completed analysis from available evidence despite acquisition-source errors.") }
}
