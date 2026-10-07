/*
 * Copyright (c) Cherri
 */

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

func generateCandidates(state *State, outputDir string) error {
	candidateDir := filepath.Join(outputDir, reportCandidatesDir)
	if err := os.MkdirAll(candidateDir, 0755); err != nil { return err }
	fingerprints := make([]string, 0, len(state.Actions))
	for fingerprint, record := range state.Actions {
		if record.Classification == classSafeCandidate { fingerprints = append(fingerprints, fingerprint) }
	}
	sort.Strings(fingerprints)
	for _, fingerprint := range fingerprints {
		record := state.Actions[fingerprint]
		base := candidateBaseName(record.Identifier, fingerprint)
		sourcePath := filepath.Join(candidateDir, base+".cherri")
		metaPath := filepath.Join(candidateDir, base+".json")
		if err := os.WriteFile(sourcePath, []byte(candidateSource(record)), 0644); err != nil { return err }
		encoded, err := json.MarshalIndent(record, "", "  "); if err != nil { return err }
		if err = os.WriteFile(metaPath, append(encoded, '\n'), 0644); err != nil { return err }
		state.Candidates[fingerprint] = true
	}
	return nil
}

var unsafeNameChars = regexp.MustCompile(`[^A-Za-z0-9_.-]+`)

func candidateBaseName(identifier, fingerprint string) string {
	name := strings.TrimPrefix(identifier, "is.workflow.actions.")
	if name == "" || name == identifier { name = strings.NewReplacer(".", "-", ":", "-").Replace(identifier) }
	name = unsafeNameChars.ReplaceAllString(name, "-")
	short := fingerprint; if len(short) > 8 { short = short[:8] }
	return fmt.Sprintf("%s-%s", name, short)
}

func candidateSource(record *ActionRecord) string {
	var builder strings.Builder
	builder.WriteString("// CANDIDATE ACTION DEFINITION - NOT FOR PRODUCTION\n")
	builder.WriteString(fmt.Sprintf("// Identifier: %s\n", record.Identifier))
	builder.WriteString(fmt.Sprintf("// Evidence: %s x%d (confidence: %s)\n", record.Evidence.Status, record.Evidence.SampleCount, record.Evidence.Confidence))
	prefix := record.Fingerprint; if len(prefix) > 16 { prefix = prefix[:16] }
	builder.WriteString(fmt.Sprintf("// Schema fingerprint: %s\n", prefix))
	builder.WriteString("// Verify parameter order, keys, types, defaults, enums, output type, and platform behavior\n")
	builder.WriteString("// before promoting this into actions/. Parameters below are inferred.\n")
	signature := candidateSignature(record)
	if signature == "" { builder.WriteString("//\n// No parameter signature could be inferred; implement manually.\n"); return builder.String() }
	fmt.Fprintf(&builder, "//\n// %s\n", signature)
	return builder.String()
}

var cherriTypeNames = map[string]string{kindString: "text", kindNumber: "number", kindBoolean: "bool", kindDictionary: "dict"}

func candidateSignature(record *ActionRecord) string {
	var parts []string
	for _, key := range record.ParameterKeys {
		value := record.Parameters[key]; if value == nil { continue }
		cherriType, found := cherriTypeNames[value.Kind]
		if !found { if value.Kind == kindArray { cherriType = "text" } else { continue } }
		parts = append(parts, fmt.Sprintf("?%s %s '%s'", cherriType, safeParamName(key), key))
	}
	actionName := candidateActionName(record.Identifier)
	if len(parts) == 0 { return fmt.Sprintf("action %s()", actionName) }
	return fmt.Sprintf("action %s(%s)", actionName, strings.Join(parts, ", "))
}

func candidateActionName(identifier string) string {
	name := strings.TrimPrefix(identifier, "is.workflow.actions.")
	name = unsafeNameChars.ReplaceAllString(name, "")
	if name == "" { return "unknownCandidate" }
	return strings.ToUpper(name[:1]) + name[1:]
}

func safeParamName(key string) string {
	name := unsafeNameChars.ReplaceAllString(key, "")
	if name == "" { return "param" }
	return strings.ToLower(name[:1]) + name[1:]
}
