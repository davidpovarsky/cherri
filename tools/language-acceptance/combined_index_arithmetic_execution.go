package main

import (
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

type combinedIndexArithmeticScenario struct {
	Group, Title, Unit, Native, Scope string
}

var combinedIndexArithmeticScenarios = map[string]combinedIndexArithmeticScenario{
	"RP21": {"repair", "Safe zero-based indexing", "TestP15RP21SafeIndexUnit", "TestP15RP21SafeIndexNative",
		"Distinct complete safe-index scenario: first/last/empty/static/dynamic/negative/fractional/out-of-range/uint64-max and nonnumeric indices; actual semantic Text? checks and original typed native graphs bind the same collection, single lowered index, integral/lower/upper guard ancestry, explicit Nothing fallbacks and returned complete result. Lookup escape, wrong collection/index, missing absence, malformed identities and precise type-error controls are independently rejected. Actual Apple Text/None results and absence of runtime lookup on invalid branches remain unproved."},
	"BRG16": {"recovery_gate", "Bounds and numeric semantics", "TestP15BRG16NumericBoundsUnit", "TestP15BRG16NumericBoundsNative",
		"Distinct joined numeric-bounds scenario: actual action-derived 5/2 and 10%3 feed four ordered compound updates and then guarded dynamic indexing; separate static and dynamic first/last/empty/negative/out-of-range/fractional cases retain complete typed native graphs. Exact division/Modulus, scalar kinds, all data edges, integer guard before bounds, zero-based adjustment and Nothing branches are checked; integer truncation, arithmetic substitution and unsafe lookup ancestry controls are causal. Prior N01/L03/RP15/RP17 events provide no inherited credit. Actual Apple numeric and Text/None observations remain unproved."},
}

// The immutable inventories are fixed after development and reviewed against
// every named clause. They are never inferred from a current process result.
//
//go:embed combined_index_arithmetic_inventory.json
var combinedIndexArithmeticInventory []byte
var combinedIndexArithmeticEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(combinedIndexArithmeticInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var combinedIndexArithmeticRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var combinedIndexArithmeticLocalTiers = []string{"unit", "native-structure"}

// This reviewed witness set covers this scenario and its actual intake helpers.
// Exact clean Git source identity separately binds the whole checkout. Preserve
// the shared 128-file, 4MiB-per-blob, 16MiB-total and 15s limits.
const combinedIndexArithmeticSourceWitnessSHA256 = "0889e41841fc38f79de586a1e6c374e4a7bcde312916eb84971a5d02caa4c252"

func validateCombinedIndexArithmeticSourceInventory(names []string) error {
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded combined-index-arithmetic source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered combined-index-arithmetic source inventory")
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return fmt.Errorf("duplicate combined-index-arithmetic source witness")
		}
	}
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != combinedIndexArithmeticSourceWitnessSHA256 {
		return fmt.Errorf("combined-index-arithmetic inventory omits or changes reviewed source witnesses")
	}
	return nil
}

func (s combinedIndexArithmeticScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s combinedIndexArithmeticScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s combinedIndexArithmeticScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-structure" {
		return s.testIDs()[1]
	}
	return ""
}
func combinedIndexArithmeticPrefix(id string) string {
	return "combined-index-arithmetic-" + strings.ToLower(id)
}

func (r *Runner) executeCombinedIndexArithmetic(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, ok := combinedIndexArithmeticScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown combined-index-arithmetic scenario")
	}
	if err := validateCombinedIndexArithmeticSourceInventory(combinedIndexArithmeticEvidence.SourcePaths); err != nil {
		return nil, "", false, err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("combined-index-arithmetic source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("combined-index-arithmetic requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	directory, err := stateEvidenceDirectory(r, combinedIndexArithmeticPrefix(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(scenario.tests())
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, id, combinedIndexArithmeticPrefix(id), directory, command, budget, combinedIndexArithmeticEvidence.Events[id], combinedIndexArithmeticEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, combinedIndexArithmeticPrefix(id)+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(combinedIndexArithmeticEvidence.Events[id], budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, scenario.Scope, true, nil
}
func (r *Runner) runCombinedIndexArithmeticRequirement(id, group, title string, levels []string) bool {
	scenario, ok := combinedIndexArithmeticScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), combinedIndexArithmeticRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "combined-index-arithmetic producer differs from immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeCombinedIndexArithmetic(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), combinedIndexArithmeticLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(combinedIndexArithmeticEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateCombinedIndexArithmeticRecord(id string, record EvidenceRecord) error {
	scenario, ok := combinedIndexArithmeticScenarios[id]
	if !ok {
		return nil
	}
	if err := validateCombinedIndexArithmeticSourceInventory(combinedIndexArithmeticEvidence.SourcePaths); err != nil {
		return err
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(combinedIndexArithmeticEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete combined-index-arithmetic scenario %s", id)
	}
	prefix := combinedIndexArithmeticPrefix(id)
	data, err := stateReadArtifacts(record.Artifacts, editorContractArtifacts(prefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"]
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, combinedIndexArithmeticEvidence.Events[id], raw, stderr, budget, combinedIndexArithmeticEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(combinedIndexArithmeticEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
