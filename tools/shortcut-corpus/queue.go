/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	reportAgentQueueJSON = "agent-queue.json"
	reportAgentQueueMD   = "agent-queue.md"
)

type agentQueueItem struct {
	Rank                int                 `json:"rank"`
	Identifier          string              `json:"identifier"`
	Classification      string              `json:"classification"`
	Reason              []string            `json:"reason,omitempty"`
	SampleCount         int                 `json:"sampleCountDistinctContent"`
	FirstSeen           string              `json:"firstSeen,omitempty"`
	LastSeen            string              `json:"lastSeen,omitempty"`
	ParameterKeys       []string            `json:"parameterKeys,omitempty"`
	UnknownKeys         []string            `json:"unknownKeysComparedToCatalog,omitempty"`
	SchemaFingerprint   string              `json:"schemaFingerprint"`
	Serializations      map[string][]string `json:"serializations,omitempty"`
	ValueObservations   map[string][]string `json:"sanitizedValueObservations,omitempty"`
	EvidenceHashes      []string            `json:"evidenceHashes,omitempty"`
	CandidatePath       string              `json:"candidatePath,omitempty"`
	RecommendedNextStep string              `json:"recommendedNextStep"`
}

func writeAgentQueue(state *State, catalog *actionCatalog, outputDir string, maxItems int, includeThirdParty bool) error {
	items := buildAgentQueue(state, catalog, includeThirdParty)
	if maxItems > 0 && len(items) > maxItems { items = items[:maxItems] }
	for index := range items { items[index].Rank = index + 1 }
	encoded, err := json.MarshalIndent(items, "", "  "); if err != nil { return err }
	if err = os.WriteFile(filepath.Join(outputDir, reportAgentQueueJSON), append(encoded, '\n'), 0644); err != nil { return err }
	var builder strings.Builder
	builder.WriteString("# Agent action queue\n\n")
	builder.WriteString("Derived from privacy-safe structural evidence and the current Cherri catalog. KNOWN actions are intentionally absent.\n\n")
	if len(items) == 0 {
		builder.WriteString("No actionable new or variant action schemas are currently queued.\n")
	} else {
		fmt.Fprintf(&builder, "Actionable items: **%d**\n\n", len(items))
		for _, item := range items {
			fmt.Fprintf(&builder, "## %d. `%s` — %s\n\n", item.Rank, item.Identifier, item.Classification)
			fmt.Fprintf(&builder, "- Distinct Shortcut samples: %d\n", item.SampleCount)
			if len(item.UnknownKeys) != 0 { fmt.Fprintf(&builder, "- Unmodeled keys: `%s`\n", strings.Join(item.UnknownKeys, "`, `")) }
			if len(item.Reason) != 0 { fmt.Fprintf(&builder, "- Evidence note: %s\n", strings.Join(item.Reason, "; ")) }
			if item.CandidatePath != "" { fmt.Fprintf(&builder, "- Candidate: `%s`\n", item.CandidatePath) }
			fmt.Fprintf(&builder, "- Next: %s\n\n", item.RecommendedNextStep)
		}
	}
	return os.WriteFile(filepath.Join(outputDir, reportAgentQueueMD), []byte(builder.String()), 0644)
}

func buildAgentQueue(state *State, catalog *actionCatalog, includeThirdParty bool) []agentQueueItem {
	var items []agentQueueItem
	for _, record := range state.Actions {
		if record.Classification == classKnown { continue }
		if record.Classification == classThirdParty && !includeThirdParty { continue }
		item := agentQueueItem{
			Identifier: record.Identifier, Classification: record.Classification, Reason: append([]string(nil), record.Notes...),
			SampleCount: len(record.SampleHashes), FirstSeen: record.FirstSeen, LastSeen: record.LastSeen,
			ParameterKeys: append([]string(nil), record.ParameterKeys...), SchemaFingerprint: record.Fingerprint,
			Serializations: collectSerializations(record.Parameters), ValueObservations: record.ValueObservations,
			EvidenceHashes: boundedUniqueStrings(record.SampleHashes, 8), RecommendedNextStep: queueNextStep(record),
		}
		if known := catalog.knownParameterKeys(record.Identifier); known != nil { item.UnknownKeys = differenceKeys(observedKeySet(record.Parameters), unionSets(known, ignoredParameterKeys)) }
		if record.Classification == classSafeCandidate { item.CandidatePath = filepath.ToSlash(filepath.Join(reportCandidatesDir, candidateBaseName(record.Identifier, record.Fingerprint)+".cherri")) }
		items = append(items, item)
	}
	sort.Slice(items, func(i, j int) bool {
		pi, pj := queuePriority(items[i]), queuePriority(items[j]); if pi != pj { return pi < pj }
		if items[i].SampleCount != items[j].SampleCount { return items[i].SampleCount > items[j].SampleCount }
		if items[i].Identifier != items[j].Identifier { return items[i].Identifier < items[j].Identifier }
		return items[i].SchemaFingerprint < items[j].SchemaFingerprint
	})
	return items
}

func queuePriority(item agentQueueItem) int {
	switch item.Classification {
	case classSafeCandidate: if item.SampleCount >= 2 { return 0 }; return 2
	case classVariant: if item.SampleCount >= 2 { return 1 }; return 3
	case classCustomImplementationReq: if item.SampleCount >= 2 { return 2 }; return 4
	case classNeedsReview, classUnknown, classNew: return 5
	case classThirdParty: return 9
	default: return 7
	}
}

func queueNextStep(record *ActionRecord) string {
	switch record.Classification {
	case classSafeCandidate: return "Verify the generated candidate against distinct real Shortcut samples, then promote only confirmed semantics into Cherri."
	case classVariant: return "Compare this schema delta with the current Cherri definition and minimal distinct raw samples; update the existing action only if the variant is proven."
	case classCustomImplementationReq: return "Read the technical reference, inspect the closest existing custom action, and verify the complex serialization before implementing."
	case classThirdParty: return "Keep as third-party evidence unless explicit project policy or user request calls for a curated wrapper."
	default: return "Inspect normalized evidence first and open only the minimum raw samples needed to resolve ambiguity."
	}
}

func collectSerializations(parameters map[string]*NormalizedValue) map[string][]string {
	result := map[string][]string{}; for key, value := range parameters { collectSerialization(key, value, result) }; if len(result) == 0 { return nil }; return result
}
func collectSerialization(path string, value *NormalizedValue, result map[string][]string) {
	if value == nil { return }
	if value.Serialization != "" { result[path] = boundedUniqueStrings(append(result[path], value.Serialization), 16) }
	for _, child := range value.Items { collectSerialization(path+"[]", child, result) }
	for key, child := range value.Fields { collectSerialization(path+"."+key, child, result) }
}
