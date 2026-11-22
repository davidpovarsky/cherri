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

type combinedArithmeticScenario struct {
	Group, Title, Unit, Native, Scope string
}

var combinedArithmeticScenarios = map[string]combinedArithmeticScenario{
	"RP15": {"repair", "Compound assignment", "TestP15RP15CompoundOnceUnit", "TestP15RP15CompoundOnceNative",
		"Distinct complete local compound scenario: start Number 10 then +=3, *=2, -=4, /=2; four lexical RHS expressions each have exact branch ancestry, one counter and ordered write per selected branch, typed Number yields and accumulator stores; final Number reference and count/order observation are bound to the full native graph. Actual Apple Number11, four RHS evaluations and observed order remain unproved."},
	"RP17": {"repair", "Comparisons and arithmetic", "TestP15RP17ComparisonsArithmeticUnit", "TestP15RP17ComparisonsArithmeticNative",
		"Distinct joined local scenario: action-derived Number2/3/5/10 feed >, ==, !=, fractional division, scientific Modulus and a mixed unary/parenthesized tree; semantic Bool/Number, precise typed native fields, all producers/references and the returned observations are independently checked. Wrong operators, integer truncation, generic addition and malformed identities are rejected. Actual Apple Bool/Number results remain unproved."},
}

// The immutable inventories are fixed after development and reviewed against
// every named clause. They are never inferred from a current process result.
//
//go:embed combined_arithmetic_inventory.json
var combinedArithmeticInventory []byte
var combinedArithmeticEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(combinedArithmeticInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var combinedArithmeticRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var combinedArithmeticLocalTiers = []string{"unit", "native-structure"}

// This reviewed witness set covers this scenario and its actual intake helpers.
// Exact clean Git source identity separately binds the whole checkout. Preserve
// the shared 128-file, 4MiB-per-blob, 16MiB-total and 15s limits.
const combinedArithmeticSourceWitnessSHA256 = "f5f53158ce87b52322e94059401ca279d4820772ed1b22d1e6c65afe916e2cfc"

func validateCombinedArithmeticSourceInventory(names []string) error {
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded combined-arithmetic source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered combined-arithmetic source inventory")
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return fmt.Errorf("duplicate combined-arithmetic source witness")
		}
	}
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != combinedArithmeticSourceWitnessSHA256 {
		return fmt.Errorf("combined-arithmetic inventory omits or changes reviewed source witnesses")
	}
	return nil
}

func (s combinedArithmeticScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s combinedArithmeticScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s combinedArithmeticScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-structure" {
		return s.testIDs()[1]
	}
	return ""
}
func combinedArithmeticPrefix(id string) string { return "combined-arithmetic-" + strings.ToLower(id) }

func (r *Runner) executeCombinedArithmetic(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, ok := combinedArithmeticScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown combined combined-arithmetic scenario")
	}
	if err := validateCombinedArithmeticSourceInventory(combinedArithmeticEvidence.SourcePaths); err != nil {
		return nil, "", false, err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("combined combined-arithmetic source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("combined combined-arithmetic requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	directory, err := stateEvidenceDirectory(r, combinedArithmeticPrefix(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(scenario.tests())
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, id, combinedArithmeticPrefix(id), directory, command, budget, combinedArithmeticEvidence.Events[id], combinedArithmeticEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, combinedArithmeticPrefix(id)+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(combinedArithmeticEvidence.Events[id], budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, scenario.Scope, true, nil
}
func (r *Runner) runCombinedArithmeticRequirement(id, group, title string, levels []string) bool {
	scenario, ok := combinedArithmeticScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), combinedArithmeticRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "combined combined-arithmetic producer differs from immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeCombinedArithmetic(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), combinedArithmeticLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(combinedArithmeticEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateCombinedArithmeticRecord(id string, record EvidenceRecord) error {
	scenario, ok := combinedArithmeticScenarios[id]
	if !ok {
		return nil
	}
	if err := validateCombinedArithmeticSourceInventory(combinedArithmeticEvidence.SourcePaths); err != nil {
		return err
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(combinedArithmeticEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete combined combined-arithmetic scenario %s", id)
	}
	prefix := combinedArithmeticPrefix(id)
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
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, combinedArithmeticEvidence.Events[id], raw, stderr, budget, combinedArithmeticEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(combinedArithmeticEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
