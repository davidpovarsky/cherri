/*
 * Copyright (c) Cherri
 */

package main

import (
	"errors"
	"fmt"
	"os"
	"sort"
)

// analyze runs the full incremental pipeline:
//
//	discover -> hash -> dedupe -> parse -> sanitize -> schema fingerprint ->
//	update evidence -> reclassify all state vs current catalog -> reports/queue
func analyze(config analyzeConfig) error {
	if err := os.MkdirAll(config.outputDir, 0755); err != nil {
		return fmt.Errorf("output directory: %w", err)
	}

	state, err := loadState(config.statePath)
	if err != nil {
		return err
	}

	catalog, err := obtainCatalog(config)
	if err != nil {
		return err
	}

	files, err := discoverInputs(config.inputs)
	if err != nil {
		return err
	}

	stats := analyzeStats{catalogCount: catalog.count, catalogSource: catalog.source}
	for _, path := range files {
		if config.maxFiles > 0 && stats.newFiles >= config.maxFiles {
			stats.skippedRemaining++
			continue
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			stats.failed = append(stats.failed, failure{path, readErr.Error()})
			continue
		}
		hash := hashBytes(data)
		if !config.force {
			if _, seen := state.Files[hash]; seen {
				stats.skippedKnown++
				continue
			}
		}

		doc, parseErr := parseDocument(path, data)
		if parseErr != nil {
			if errors.Is(parseErr, errNotAShortcut) {
				stats.skippedOther++
				continue
			}
			stats.failed = append(stats.failed, failure{path, parseErr.Error()})
			continue
		}

		newActions := ingestDocument(state, doc, catalog, config)
		record := &FileRecord{Name: doc.name, ActionSeen: doc.rawActionCount, NewActions: newActions, FirstSeen: now()}
		if existing, seen := state.Files[hash]; seen {
			record.FirstSeen = existing.FirstSeen
		}
		state.Files[hash] = record

		stats.analyzed++
		if newActions > 0 {
			stats.newFiles++
		}
	}

	reclassifyState(state, catalog)
	state.CatalogDigest = catalog.digest
	if !config.noCandidates {
		if err = generateCandidates(state, config.outputDir); err != nil {
			return fmt.Errorf("candidate generation: %w", err)
		}
	}
	if err = state.save(config.statePath); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}

	report := buildReports(state, stats, config.outputDir)
	if err = writeAgentQueue(state, catalog, config.outputDir, config.maxActionable, config.includeThirdParty); err != nil {
		return fmt.Errorf("agent queue: %w", err)
	}
	printHumanSummary(report, os.Stdout)
	return nil
}

type failure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

type analyzeStats struct {
	catalogCount     int
	catalogSource    string
	analyzed         int
	skippedKnown     int
	skippedRemaining int
	skippedOther     int
	newFiles         int
	failed           []failure
}

func ingestDocument(state *State, doc *shortcutDocument, catalog *actionCatalog, config analyzeConfig) int {
	var newActions int
	for _, rawActionItem := range doc.actions {
		parameters := make(map[string]*NormalizedValue, len(rawActionItem.Parameters))
		for key, value := range rawActionItem.Parameters {
			parameters[key] = normalizeValue(value)
		}
		trimmed := dropIgnoredKeys(parameters)
		fingerprint := fingerprintAction(rawActionItem.Identifier, trimmed)
		if fingerprint == "" {
			continue
		}

		if existing, seen := state.Actions[fingerprint]; seen {
			if addSampleHash(existing, doc.hash) {
				existing.LastSeen = now()
			}
			mergeValueObservations(existing, collectValueObservations(trimmed))
			continue
		}

		record := &ActionRecord{
			Fingerprint:       fingerprint,
			Identifier:        rawActionItem.Identifier,
			ParameterKeys:     sortedKeys(trimmed),
			Parameters:        trimmed,
			ValueObservations: collectValueObservations(trimmed),
			Evidence:          evidenceObserved(1),
			Count:             1,
			SampleHashes:      []string{doc.hash},
			Files:             []string{doc.hash},
			FirstSeen:         now(),
			LastSeen:          now(),
		}
		classify(record, catalog)
		if record.Classification == classUnknown && state.hasOtherFingerprint(record.Identifier, fingerprint) {
			record.Classification = classVariant
			record.Notes = []string{"additional structural schema of unresolved identifier"}
		} else {
			refineClassification(record)
		}
		state.Actions[fingerprint] = record
		newActions++
	}
	return newActions
}

// reclassifyState makes classification a derived view of normalized evidence
// and the current Cherri catalog. No raw Shortcut file needs to be reparsed.
func reclassifyState(state *State, catalog *actionCatalog) {
	groups := map[string][]*ActionRecord{}
	for _, record := range state.Actions {
		normalizeRecordSamples(record)
		classify(record, catalog)
		if record.Classification == classUnknown {
			groups[record.Identifier] = append(groups[record.Identifier], record)
		}
	}

	for _, records := range groups {
		sort.Slice(records, func(i, j int) bool {
			if records[i].FirstSeen != records[j].FirstSeen {
				return records[i].FirstSeen < records[j].FirstSeen
			}
			return records[i].Fingerprint < records[j].Fingerprint
		})
		for index, record := range records {
			if index == 0 {
				refineClassification(record)
				continue
			}
			record.Classification = classVariant
			record.Notes = []string{"additional structural schema of unresolved identifier"}
		}
	}

	for fingerprint := range state.Candidates {
		record := state.Actions[fingerprint]
		if record == nil || record.Classification != classSafeCandidate {
			delete(state.Candidates, fingerprint)
		}
	}
}

func reclassifyExisting(config analyzeConfig) error {
	if err := os.MkdirAll(config.outputDir, 0755); err != nil {
		return fmt.Errorf("output directory: %w", err)
	}
	state, err := loadState(config.statePath)
	if err != nil {
		return err
	}
	catalog, err := obtainCatalog(config)
	if err != nil {
		return err
	}
	reclassifyState(state, catalog)
	state.CatalogDigest = catalog.digest
	if !config.noCandidates {
		if err = generateCandidates(state, config.outputDir); err != nil {
			return fmt.Errorf("candidate generation: %w", err)
		}
	}
	if err = state.save(config.statePath); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	stats := analyzeStats{catalogCount: catalog.count, catalogSource: catalog.source}
	report := buildReports(state, stats, config.outputDir)
	if err = writeAgentQueue(state, catalog, config.outputDir, config.maxActionable, config.includeThirdParty); err != nil {
		return fmt.Errorf("agent queue: %w", err)
	}
	printHumanSummary(report, os.Stdout)
	return nil
}

func containsFile(files []string, hash string) bool {
	for _, item := range files {
		if item == hash {
			return true
		}
	}
	return false
}

func (state *State) hasOtherFingerprint(identifier, fingerprint string) bool {
	for _, record := range state.Actions {
		if record.Identifier == identifier && record.Fingerprint != fingerprint {
			return true
		}
	}
	return false
}

func defaultCherriBin() string {
	if bin := os.Getenv("CHERRI_BIN"); bin != "" {
		return bin
	}
	return "cherri"
}
