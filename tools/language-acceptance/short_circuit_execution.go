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

type shortCircuitScenario struct {
	Group, Title, Unit, Native, Scope string
}

var shortCircuitScenarios = map[string]shortCircuitScenario{
	"RP18": {"repair", "Observable short circuit", "TestP15RP18ObservableShortCircuitUnit", "TestP15RP18ObservableShortCircuitNative",
		"Distinct complete local scenario: action-derived Bool seeds, all eight AND/OR cases and four mixed trees with 18 lexical RHS expression helpers, once-only counter/order writes in each branch, full ancestor and producer identity, observed result/count/order output and causal corruptions. Actual Apple skipped/required effects, execution order and Bool transport remain unproved."},
	"BRG13": {"recovery_gate", "Typed conditions and dynamic short circuit", "TestBackendRecovery13TypedShortCircuitUnit", "TestBackendRecovery13TypedShortCircuitNative",
		"Distinct joined local scenario: action-derived Bool inputs in all eight AND/OR cases and four mixed trees, counted ordered RHS effects, typed Number/Text/Bool semantics, four dynamic same/different-case Text comparisons and six Number comparisons with exact operators/operands/results, separate case-sensitive constant-fold control and no generic comparison addition. Actual Apple effect count/order, Bool transport and dynamic Text case sensitivity remain unproved."},
}

// The immutable inventories are fixed after development and reviewed against
// every named clause. They are never inferred from a current process result.
//
//go:embed short_circuit_inventory.json
var shortCircuitInventory []byte
var shortCircuitEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(shortCircuitInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var shortCircuitRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var shortCircuitLocalTiers = []string{"unit", "native-structure"}

// This reviewed witness set covers this scenario and its actual intake helpers.
// Exact clean Git source identity separately binds the whole checkout. Preserve
// the shared 128-file, 4MiB-per-blob, 16MiB-total and 15s limits.
const shortCircuitSourceWitnessSHA256 = "dda7690f8d40b070692a62f305fe9d083b096eeca769a91906a6f1c07d56c363"

func validateShortCircuitSourceInventory(names []string) error {
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded short-circuit source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered short-circuit source inventory")
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return fmt.Errorf("duplicate short-circuit source witness")
		}
	}
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != shortCircuitSourceWitnessSHA256 {
		return fmt.Errorf("short-circuit inventory omits or changes reviewed source witnesses")
	}
	return nil
}

func (s shortCircuitScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s shortCircuitScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s shortCircuitScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-structure" {
		return s.testIDs()[1]
	}
	return ""
}
func shortCircuitPrefix(id string) string { return "short-circuit-" + strings.ToLower(id) }

func (r *Runner) executeShortCircuit(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, ok := shortCircuitScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown combined short-circuit scenario")
	}
	if err := validateShortCircuitSourceInventory(shortCircuitEvidence.SourcePaths); err != nil {
		return nil, "", false, err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("combined short-circuit source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("combined short-circuit requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	directory, err := stateEvidenceDirectory(r, shortCircuitPrefix(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(scenario.tests())
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, id, shortCircuitPrefix(id), directory, command, budget, shortCircuitEvidence.Events[id], shortCircuitEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, shortCircuitPrefix(id)+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(shortCircuitEvidence.Events[id], budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, scenario.Scope, true, nil
}
func (r *Runner) runShortCircuitRequirement(id, group, title string, levels []string) bool {
	scenario, ok := shortCircuitScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), shortCircuitRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "combined short-circuit producer differs from immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeShortCircuit(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), shortCircuitLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(shortCircuitEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateShortCircuitRecord(id string, record EvidenceRecord) error {
	scenario, ok := shortCircuitScenarios[id]
	if !ok {
		return nil
	}
	if err := validateShortCircuitSourceInventory(shortCircuitEvidence.SourcePaths); err != nil {
		return err
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(shortCircuitEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete combined short-circuit scenario %s", id)
	}
	prefix := shortCircuitPrefix(id)
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
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, shortCircuitEvidence.Events[id], raw, stderr, budget, shortCircuitEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(shortCircuitEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
