/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Report names are stable so future agent sessions can consume them
// programmatically before ever opening raw corpus records.
const (
	reportSummary       = "summary.json"
	reportKnownActions  = "known-actions.json"
	reportNewActions    = "new-actions.json"
	reportVariants      = "variants.json"
	reportThirdParty    = "third-party-actions.json"
	reportNeedsReview   = "needs-review.json"
	reportHumanSummary  = "summary.md"
	reportCandidatesDir = "candidates"
)

type summaryReport struct {
	GeneratedAt string         `json:"generatedAt"`
	Catalog     catalogSummary `json:"catalog"`
	Files       filesSummary   `json:"files"`
	Actions     actionsSummary `json:"actions"`
}

type catalogSummary struct {
	Source string `json:"source"`
	Count  int    `json:"count"`
}

type filesSummary struct {
	Total            int       `json:"total"`
	Analyzed         int       `json:"analyzed"`
	SkippedKnown     int       `json:"skippedKnown"`
	SkippedRemaining int       `json:"skippedRemaining"`
	SkippedOther     int       `json:"skippedOther,omitempty"`
	KnownInState     int       `json:"knownInState"`
	Failed           []failure `json:"failed,omitempty"`
}

type actionsSummary struct {
	Total                        int `json:"total"`
	Known                        int `json:"known"`
	New                          int `json:"new"`
	Variant                      int `json:"variant"`
	ThirdParty                   int `json:"thirdParty"`
	Unknown                      int `json:"unknown"`
	SafeCandidate                int `json:"safeCandidate"`
	CustomImplementationRequired int `json:"customImplementationRequired"`
	CandidatesGenerated          int `json:"candidatesGenerated"`
}

// buildReports writes the machine-readable reports plus a concise human
// summary, and returns the summary for printing.
func buildReports(state *State, stats analyzeStats, outputDir string) summaryReport {
	buckets := map[string][]*ActionRecord{
		reportKnownActions: {},
		reportNewActions:   {},
		reportVariants:     {},
		reportThirdParty:   {},
		reportNeedsReview:  {},
	}

	for _, record := range sortedRecords(state) {
		switch record.Classification {
		case classKnown:
			buckets[reportKnownActions] = append(buckets[reportKnownActions], record)
		case classNew, classSafeCandidate, classCustomImplementationReq:
			buckets[reportNewActions] = append(buckets[reportNewActions], record)
			if record.Classification != classSafeCandidate && record.Classification != classCustomImplementationReq {
				buckets[reportNeedsReview] = append(buckets[reportNeedsReview], record)
			}
		case classVariant:
			buckets[reportVariants] = append(buckets[reportVariants], record)
			buckets[reportNeedsReview] = append(buckets[reportNeedsReview], record)
		case classThirdParty:
			buckets[reportThirdParty] = append(buckets[reportThirdParty], record)
		default:
			buckets[reportNeedsReview] = append(buckets[reportNeedsReview], record)
		}
	}

	counts := countClassifications(state)
	totals := actionsSummary{
		Total: len(state.Actions),
	}
	for classification, count := range counts {
		switch classification {
		case classKnown:
			totals.Known = count
		case classNew:
			totals.New = count
		case classVariant:
			totals.Variant = count
		case classThirdParty:
			totals.ThirdParty = count
		case classUnknown:
			totals.Unknown = count
		case classSafeCandidate:
			totals.SafeCandidate = count
		case classCustomImplementationReq:
			totals.CustomImplementationRequired = count
		}
	}
	totals.CandidatesGenerated = len(state.Candidates)

	summary := summaryReport{
		GeneratedAt: now(),
		Catalog:     catalogSummary{Source: stats.catalogSource, Count: stats.catalogCount},
		Files: filesSummary{
			Total:            stats.analyzed + stats.skippedKnown + stats.skippedRemaining + stats.skippedOther,
			Analyzed:         stats.analyzed,
			SkippedKnown:     stats.skippedKnown,
			SkippedRemaining: stats.skippedRemaining,
			SkippedOther:     stats.skippedOther,
			KnownInState:     len(state.Files),
			Failed:           stats.failed,
		},
		Actions: totals,
	}

	writeJSONReport(outputDir, reportSummary, summary)
	for name, records := range buckets {
		writeJSONReport(outputDir, name, records)
	}
	writeHumanSummary(filepath.Join(outputDir, reportHumanSummary), summary, buckets)

	return summary
}

func sortedRecords(state *State) []*ActionRecord {
	fingerprints := make([]string, 0, len(state.Actions))
	for fingerprint := range state.Actions {
		fingerprints = append(fingerprints, fingerprint)
	}
	sort.Strings(fingerprints)

	var records []*ActionRecord
	for _, fingerprint := range fingerprints {
		records = append(records, state.Actions[fingerprint])
	}
	return records
}

func countClassifications(state *State) map[string]int {
	counts := map[string]int{}
	for _, record := range state.Actions {
		counts[record.Classification]++
	}
	return counts
}

func writeJSONReport(outputDir, name string, payload any) {
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(filepath.Join(outputDir, name), append(encoded, '\n'), 0644)
}

func writeHumanSummary(path string, summary summaryReport, buckets map[string][]*ActionRecord) {
	file, err := os.Create(path)
	if err != nil {
		return
	}
	defer file.Close()

	printHumanSummary(summary, file)
	printSection(file, "Needs review", buckets[reportNeedsReview])
	printSection(file, "New actions", buckets[reportNewActions])
	printSection(file, "Variants", buckets[reportVariants])
}

func printHumanSummary(summary summaryReport, out io.Writer) {
	fmt.Fprintf(out, "Shortcut corpus analysis\n")
	fmt.Fprintf(out, "========================\n\n")
	fmt.Fprintf(out, "Catalog: %s (%d identifiers)\n", summary.Catalog.Source, summary.Catalog.Count)
	fmt.Fprintf(out, "Files: %d analyzed, %d skipped (already known), %d non-Shortcut, %d in state\n",
		summary.Files.Analyzed, summary.Files.SkippedKnown, summary.Files.SkippedOther, summary.Files.KnownInState)
	fmt.Fprintf(out, "Action shapes: %d total — known=%d new=%d variant=%d third-party=%d unknown=%d safe-candidate=%d custom-required=%d\n\n",
		summary.Actions.Total, summary.Actions.Known, summary.Actions.New,
		summary.Actions.Variant, summary.Actions.ThirdParty, summary.Actions.Unknown,
		summary.Actions.SafeCandidate, summary.Actions.CustomImplementationRequired)
	if len(summary.Files.Failed) != 0 {
		fmt.Fprintf(out, "Failures:\n")
		for _, item := range summary.Files.Failed {
			fmt.Fprintf(out, "  %s: %s\n", filepath.Base(item.Path), item.Error)
		}
		fmt.Fprint(out, "\n")
	}
	fmt.Fprintf(out, "Inspect needs-review.json first; open raw files only for NEW/VARIANT records.\n")
}

func printSection(out io.Writer, title string, records []*ActionRecord) {
	if len(records) == 0 {
		return
	}
	limit := records
	if len(limit) > 15 {
		limit = limit[:15]
	}
	fmt.Fprintf(out, "\n%s (%d):\n", title, len(records))
	for _, record := range limit {
		fmt.Fprintf(out, "  %-45s x%-3d %s\n",
			record.Identifier, record.Count, record.Classification)
	}
	if len(records) > len(limit) {
		fmt.Fprintf(out, "  ... and %d more\n", len(records)-len(limit))
	}
}
