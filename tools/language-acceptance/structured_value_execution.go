package main

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

type structuredValueScenario struct{ Group, Title, Unit, Native, Scope string }

var structuredValueScenarios = map[string]structuredValueScenario{
	"T05": {"types", "Optional presence differs from optional value and JSON null", "TestP15T05CompleteOptionalUnit", "TestP15T05CompleteOptionalNative",
		"Original T05 unit/native-structure clause: independent semantic presence for None, false, zero, empty Text/List/Map and present JsonNull; exact joined native cell graph, Optional contexts, dynamic/control/function transport and guarded native boundaries. No fabricated plist null and no Apple execution claim."},
	"L01": {"collections", "Nested lists/maps preserve shape, cardinality and element order", "TestP15L01CompleteHierarchyUnit", "TestP15L01CompleteHierarchyNative",
		"Original L01 unit/native-structure clause: nested Dictionary arrays and source-owned nested List cells preserve type hierarchy, ordered elements, empty containers and independent Optional presence; dynamic aliases/consumers and explicit unencodable native boundaries are exercised. No Apple execution claim."},
	"F04": {"control", "Value loop collects one semantic element per iteration", "TestP15F04CompleteIterationUnit", "TestP15F04CompleteIterationNative",
		"Original F04 unit/native-structure clause: zero, one and multiple iterations retain one final semantic element through exact cleared accumulators, independent cell presence, nested/null payloads, for/repeat, mutation, aliases, storage, append, indexing and branch/loop flow. The original 330-event declaration-storage regression and all enrolled older facets remain required; no Apple execution claim."},
	"RP20": {"repair", "Container and optional shape", "TestP15RP20JoinedValueSemantics", "TestP15RP20JoinedNativeStructure",
		"Distinct original RP20 joined unit/native-structure scenario: authored nested lists/maps simultaneously contain false, zero, empty Text/List/Map and explicit Optional/JsonNull contexts; exact types, order, cardinality, hierarchy and independent presence are checked with causal graph corruptions. No related-ID inheritance, fabricated plist null or Apple execution claim."},
}

type structuredValueInventoryFile struct {
	Tests       []string            `json:"tests"`
	Events      []string            `json:"events"`
	Packages    map[string][]string `json:"packages"`
	OriginalF04 map[string][]string `json:"original_f04_330"`
	SourcePaths []string            `json:"source_paths"`
}

//go:embed structured_value_inventory.json
var structuredValueInventoryRaw []byte
var structuredValueInventory = func() structuredValueInventoryFile {
	var value structuredValueInventoryFile
	if err := json.Unmarshal(structuredValueInventoryRaw, &value); err != nil {
		panic(err)
	}
	return value
}()

// This digest enrolls the reviewed cohort, original 330 identities and sources.
const structuredValueInventorySHA256 = "STRUCTURED_INVENTORY_DIGEST"
const structuredValueCohortID = "structured-values-T05-L01-F04-RP20"
const structuredValueCohortPrefix = "structured-values-cohort"

var structuredValueTiers = []string{"unit", "native-structure"}
var structuredValuePackagePaths = []string{".", "./internal/language/lower", "./internal/language/schema"}
var structuredValuePackageIDs = []string{"github.com/electrikmilk/cherri", "github.com/electrikmilk/cherri/internal/language/lower", "github.com/electrikmilk/cherri/internal/language/schema"}

type structuredValueExecution struct {
	Source    string
	Artifacts []EvidenceArtifact
	Err       error
}

func (s structuredValueScenario) producer(tier string) string {
	name := ""
	if tier == "unit" {
		name = s.Unit
	}
	if tier == "native-structure" {
		name = s.Native
	}
	if name == "" {
		return ""
	}
	return "github.com/electrikmilk/cherri:" + name
}
func structuredValueTestIDs(id string) []string {
	s, ok := structuredValueScenarios[id]
	if !ok {
		return nil
	}
	return []string{s.producer("unit"), s.producer("native-structure")}
}
func structuredValueCommand() []string {
	row := structuredValueInventory
	command := productionTestCommand(row.Tests)
	// Preserve the existing bounded Go invocation; enroll each actual package.
	command = append(append(append([]string(nil), command[:7]...), structuredValuePackagePaths[1:]...), command[7:]...)
	return command
}
func validateStructuredValueInventory() error {
	row := structuredValueInventory
	var decoded structuredValueInventoryFile
	if err := json.Unmarshal(structuredValueInventoryRaw, &decoded); err != nil || !reflect.DeepEqual(decoded, row) {
		return fmt.Errorf("structured runtime inventory differs from original embedded bytes")
	}
	if len(structuredValueScenarios) != 4 || editorSHA(structuredValueInventoryRaw) != structuredValueInventorySHA256 {
		return fmt.Errorf("unreviewed structured-value cohort inventory")
	}
	if len(row.Tests) < 8 || len(row.Tests) > 128 || !sort.StringsAreSorted(row.Tests) ||
		len(row.Events) < len(row.Tests) || len(row.Events) > 8192 || !sort.StringsAreSorted(row.Events) ||
		len(row.SourcePaths) == 0 || len(row.SourcePaths) > 256 || !sort.StringsAreSorted(row.SourcePaths) ||
		len(row.Packages) != 3 || len(row.OriginalF04) != 3 {
		return fmt.Errorf("invalid bounded structured-value witnesses")
	}
	parents := map[string]bool{}
	for _, name := range row.Tests {
		if parents[name] || !strings.HasPrefix(name, "Test") || strings.Contains(name, "/") {
			return fmt.Errorf("duplicate or foreign structured parent")
		}
		parents[name] = true
	}
	seen := map[string]bool{}
	for _, name := range row.Events {
		if seen[name] || !parents[strings.SplitN(name, "/", 2)[0]] {
			return fmt.Errorf("duplicate or foreign structured child")
		}
		seen[name] = true
	}
	for name := range parents {
		if !seen[name] {
			return fmt.Errorf("missing structured parent event")
		}
	}
	for _, s := range structuredValueScenarios {
		if !parents[s.Unit] || !parents[s.Native] {
			return fmt.Errorf("distinct structured producer omitted")
		}
	}
	flattened := []string{}
	originalTotal := 0
	originalSeen := map[string]bool{}
	for _, pkg := range structuredValuePackageIDs {
		events, found := row.Packages[pkg]
		if !found || len(events) == 0 || !sort.StringsAreSorted(events) {
			return fmt.Errorf("missing actual structured package")
		}
		enrolled := map[string]bool{}
		for _, name := range events {
			if enrolled[name] || !seen[name] {
				return fmt.Errorf("duplicate or foreign package event")
			}
			enrolled[name] = true
			flattened = append(flattened, name)
		}
		for _, name := range row.OriginalF04[pkg] {
			key := pkg + ":" + name
			if originalSeen[key] || !enrolled[name] {
				return fmt.Errorf("original F04 identity missing or duplicated")
			}
			originalSeen[key] = true
			originalTotal++
		}
	}
	sort.Strings(flattened)
	if !reflect.DeepEqual(flattened, row.Events) || originalTotal != 330 ||
		len(row.OriginalF04[structuredValuePackageIDs[0]]) != 260 {
		return fmt.Errorf("structured cohort lost original package-bound F04 260+70 identities")
	}
	for i, path := range row.SourcePaths {
		if i > 0 && path == row.SourcePaths[i-1] {
			return fmt.Errorf("duplicate structured source witness")
		}
	}
	return nil
}
func parseStructuredValueOutput(raw []byte, budget time.Duration) error {
	if len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return fmt.Errorf("structured original Go stream is incomplete")
	}
	rows := map[string][]byte{}
	for _, line := range bytes.Split(raw[:len(raw)-1], []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			return fmt.Errorf("structured original Go stream contains an empty event")
		}
		if _, err := decodeNativeEncodingObject(line); err != nil {
			return fmt.Errorf("invalid structured original JSON: %w", err)
		}
		var e struct{ Package string }
		if err := json.Unmarshal(line, &e); err != nil {
			return err
		}
		if _, ok := structuredValueInventory.Packages[e.Package]; !ok {
			return fmt.Errorf("foreign structured package %q", e.Package)
		}
		rows[e.Package] = append(rows[e.Package], line...)
		rows[e.Package] = append(rows[e.Package], '\n')
	}
	for _, pkg := range structuredValuePackageIDs {
		if err := parseStrictNamedProductionOutputInPackage(pkg, structuredValueInventory.Packages[pkg], budget, rows[pkg]); err != nil {
			return err
		}
	}
	return nil
}
func (r *Runner) executeStructuredValueCohort(root, head string) *structuredValueExecution {
	result := &structuredValueExecution{Source: head}
	directory, err := stateEvidenceDirectory(r, structuredValueCohortPrefix+"-")
	if err != nil {
		result.Err = err
		return result
	}
	row := structuredValueInventory
	budget := productionTestTimeout(row.Tests) + time.Minute
	receipt, artifacts, err := runExactSourceProcessBounded(root, head, structuredValueCohortID, structuredValueCohortPrefix, directory, structuredValueCommand(), budget, row.Events, row.SourcePaths, 256)
	result.Artifacts = artifacts
	if err == nil {
		var raw []byte
		raw, err = os.ReadFile(filepath.Join(directory, structuredValueCohortPrefix+"-go.jsonl"))
		if err == nil {
			err = parseStructuredValueOutput(raw, budget)
		}
		if err == nil {
			err = loopOutputWithinProcess(raw, receipt)
		}
	}
	result.Err = err
	return result
}
func (r *Runner) runStructuredValueRequirement(id, group, title string, levels []string) bool {
	s, ok := structuredValueScenarios[id]
	if !ok {
		return false
	}
	level := strings.Join(levels, "+")
	fail := func(status string, err error) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: level, Status: status, Detail: err.Error()}
	}
	if group != s.Group || title != s.Title || !reflect.DeepEqual(parseRequiredTiers(level), structuredValueTiers) {
		fail("FAILED", fmt.Errorf("structured producer differs from immutable contract"))
		return true
	}
	if err := validateStructuredValueInventory(); err != nil {
		fail("NOT_RUN", err)
		return true
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		fail("NOT_RUN", err)
		return true
	}
	head, clean, err := editorSourceState(root)
	if err != nil || !clean || head != r.implSHA {
		fail("NOT_RUN", fmt.Errorf("structured producer requires exact clean source: expected=%s observed=%s clean=%t error=%v", r.implSHA, head, clean, err))
		return true
	}
	if r.structuredValues == nil {
		r.structuredValues = r.executeStructuredValueCohort(root, head)
	}
	observed := r.structuredValues
	if observed.Source != head {
		fail("NOT_RUN", fmt.Errorf("structured cohort belongs to a different source"))
		return true
	}
	if len(observed.Artifacts) == 0 {
		fail("NOT_RUN", fmt.Errorf("structured producer did not execute: %v", observed.Err))
		return true
	}
	if _, recorded := r.results[id]; recorded {
		return true
	}
	result, status, exit, detail := "passed", "PASSED", 0, s.Scope
	if observed.Err != nil {
		result, status, exit, detail = "failed", "FAILED", 1, observed.Err.Error()
	}
	r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: level, ExecutedLevels: append([]string(nil), structuredValueTiers...), Passed: observed.Err == nil, Status: status, Detail: detail}
	// Only this explicit four-scenario cohort is reused within this Runner.
	// Generic Record retains all historical incomplete-scope guards.
	for _, tier := range structuredValueTiers {
		r.records = append(r.records, EvidenceRecord{
			EvidenceID:   "ev-structured-" + strings.ToLower(id) + "-" + tier,
			Requirements: []string{id}, Tier: tier, ImplementationSHA: r.implSHA, TestID: s.producer(tier),
			Command: structuredValueCommand(), ExitCode: exit, Result: result, Detail: detail, Artifacts: append([]EvidenceArtifact(nil), observed.Artifacts...),
			Assertions: map[string]string{"scope": s.Scope, "cohort": structuredValueCohortID,
				"required_production_tests": strings.Join(structuredValueInventory.Tests, "\n"),
				"required_go_events":        strings.Join(structuredValueInventory.Events, "\n")},
		})
	}
	return true
}
func validateStructuredValueRecord(id string, record EvidenceRecord) error {
	s, ok := structuredValueScenarios[id]
	if !ok {
		return nil
	}
	if err := validateStructuredValueInventory(); err != nil {
		return err
	}
	row := structuredValueInventory
	producer := s.producer(record.Tier)
	if producer == "" || record.TestID != producer || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || !reflect.DeepEqual(record.Command, structuredValueCommand()) ||
		record.Assertions["scope"] != s.Scope || record.Assertions["cohort"] != structuredValueCohortID ||
		record.Assertions["required_production_tests"] != strings.Join(row.Tests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(row.Events, "\n") {
		return fmt.Errorf("detached or partial structured-value producer %s", id)
	}
	prefix := structuredValueCohortPrefix
	data, err := stateReadArtifacts(record.Artifacts, editorContractArtifacts(prefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"]
	budget := productionTestTimeout(row.Tests) + time.Minute
	if err = validateExactSourceProcessBounded(structuredValueCohortID, record.ImplementationSHA, receipt, structuredValueCommand(), row.Events, raw, stderr, budget, row.SourcePaths, 256); err != nil {
		return err
	}
	if err = parseStructuredValueOutput(raw, budget); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
