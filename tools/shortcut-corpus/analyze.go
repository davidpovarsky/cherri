/*
 * Copyright (c) Cherri
 */

package main

import (
	"errors"
	"fmt"
	"os"
)

// analyze runs the full incremental pipeline:
//
//	discover -> hash -> dedupe -> parse -> sanitize -> fingerprint ->
//	classify vs catalog -> update state -> reports -> candidates
func analyze(config analyzeConfig) error {
	if err := os.MkdirAll(config.outputDir, 0755); err != nil {
		return fmt.Errorf("output directory: %w", err)
	}

	state, err := loadState(config.statePath)
	if err != nil {
		return err
	}
	if state.Candidates == nil {
		state.Candidates = map[string]bool{}
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
		record := &FileRecord{
			Name:       doc.name,
			ActionSeen: doc.rawActionCount,
			NewActions: newActions,
			FirstSeen:  now(),
		}
		if existing, seen := state.Files[hash]; seen && !config.force {
			record.FirstSeen = existing.FirstSeen
		}
		state.Files[hash] = record

		stats.analyzed++
		if newActions > 0 || config.force {
			stats.newFiles++
		}
	}

	if err = state.save(config.statePath); err != nil {
		return fmt.Errorf("saving state: %w", err)
	}
	if !config.noCandidates {
		if err = generateCandidates(state, config.outputDir); err != nil {
			return fmt.Errorf("candidate generation: %w", err)
		}
	}

	report := buildReports(state, stats, config.outputDir)
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

// ingestDocument normalizes every action of one document into state,
// incrementing evidence for repeated shapes and flagging variants.
func ingestDocument(state *State, doc *shortcutDocument, catalog *actionCatalog, config analyzeConfig) int {
	var newActions int
	for _, rawActionItem := range doc.actions {
		parameters := make(map[string]*NormalizedValue, len(rawActionItem.Parameters))
		for key, value := range rawActionItem.Parameters {
			parameters[key] = normalizeValue(value)
		}
		trimmed := dropIgnoredKeys(parameters)

		fingerprint := fingerprintAction(rawActionItem.Identifier, parameters)
		if fingerprint == "" {
			continue
		}

		existing, seen := state.Actions[fingerprint]
		if seen && !config.force {
			existing.Count++
			existing.Evidence = evidenceObserved(existing.Count)
			existing.LastSeen = now()
			if !containsFile(existing.Files, doc.hash) && len(existing.Files) < 32 {
				existing.Files = append(existing.Files, doc.hash)
			}
			continue
		}

		record := &ActionRecord{
			Fingerprint:   fingerprint,
			Identifier:    rawActionItem.Identifier,
			ParameterKeys: sortedKeys(trimmed),
			Parameters:    trimmed,
			Evidence:      evidenceObserved(1),
			Count:         1,
			Files:         []string{doc.hash},
			FirstSeen:     now(),
			LastSeen:      now(),
		}
		classify(record, catalog)

		// An identifier already present in another structural form is a
		// variant of that action, not an unrelated discovery.
		if record.Classification == classUnknown && state.hasOtherFingerprint(rawActionItem.Identifier, fingerprint) {
			record.Classification = classVariant
			record.Notes = append(record.Notes, "additional structural form of observed identifier")
		}
		refineClassification(record)

		state.Actions[fingerprint] = record
		newActions++
	}
	return newActions
}

func containsFile(files []string, hash string) bool {
	for _, item := range files {
		if item == hash {
			return true
		}
	}
	return false
}

// hasOtherFingerprint reports whether the same identifier was already
// observed in a different structural form.
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
