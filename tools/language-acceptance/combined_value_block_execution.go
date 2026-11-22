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

type combinedValueBlockScenario struct {
	Group, Title, Unit, Native, Scope string
	Required                          []string
}

const combinedValueBlockF04Gap = "The complete F04 scenario is NOT_RUN: nested List and Optional loop elements have no verified native one-element/no-flatten/presence transport. Supported scalar and for-item graphs plus current target rejection are bounded observations only."

var combinedValueBlockScenarios = map[string]combinedValueBlockScenario{
	"F04": {"control", "Value loop collects one semantic element per iteration", "TestP15F04ValueLoopBoundaryUnit", "TestP15F04ValueLoopBoundaryNative",
		"Bounded supported facets only: actual zero/multiple repeat and for-item sources retain typed Text/Number/Bool results, exact complete native graphs and causal corruption controls. Nested List/Optional target rejection is explicitly observed and is not cardinality fulfillment. No complete F04 tier credit or Apple execution.", []string{"unit", "native-structure"}},
	"RP23": {"repair", "Value block and loop yields", "TestP15RP23ValueBlockLoopUnit", "TestP15RP23ValueBlockLoopNative",
		"Distinct local joined scenario: Number if yields, Bool menu yields, zero/multiple Text loop yields, two-item for results and one final consumer. Exact semantic types, all native branch/result edges and absence of overwritten outputs are independently checked; missing/illegal/overwritten yields have causal source-bound diagnostics and no emitted artifact. This local supported scenario supplies no F04 nested List/Optional or Apple result evidence.", []string{"unit", "native-structure"}},
	"BRG14": {"recovery_gate", "Value blocks, loop results and return", "TestBackendRecovery14ValueBlockReturnUnit", "TestBackendRecovery14ValueBlockReturnNative",
		"Distinct joined supported value-block scenario plus actual complete main/child early-return, bare-return, both-branch return and zero-iteration return graphs. Typed values, complete dispatch/function/caller ancestry, exact output/Nothing boundaries and final unused Text are independently specified and causally corrupted. No M02/F04/FN evidence inheritance; nested List/Optional transport and actual Apple value/cardinality/frame outcomes remain unproved.", []string{"unit", "native-structure", "ios-runtime"}},
}

//go:embed combined_value_block_inventory.json
var combinedValueBlockInventory []byte
var combinedValueBlockEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(combinedValueBlockInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var combinedValueBlockLocalTiers = []string{"unit", "native-structure"}

const combinedValueBlockSourceWitnessSHA256 = "8815479e1809972ff46ad19856b708a6cef1f994137b1250e1914d18bb096f6c"

func validateCombinedValueBlockSourceInventory(names []string) error {
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded combined-value-block source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered combined-value-block source inventory")
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return fmt.Errorf("duplicate combined-value-block source witness")
		}
	}
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != combinedValueBlockSourceWitnessSHA256 {
		return fmt.Errorf("combined-value-block inventory omits or changes reviewed source witnesses")
	}
	return nil
}
func (s combinedValueBlockScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s combinedValueBlockScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s combinedValueBlockScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-structure" {
		return s.testIDs()[1]
	}
	return ""
}
func combinedValueBlockPrefix(id string) string { return "combined-value-block-" + strings.ToLower(id) }
func (r *Runner) executeCombinedValueBlock(id string) ([]EvidenceArtifact, string, bool, error) {
	var scenario, ok = combinedValueBlockScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown value-block scenario")
	}
	if err := validateCombinedValueBlockSourceInventory(combinedValueBlockEvidence.SourcePaths); err != nil {
		return nil, "", false, err
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var source, clean, stateErr = editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("value-block source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("value-block requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	var directory, e = stateEvidenceDirectory(r, combinedValueBlockPrefix(id)+"-")
	if e != nil {
		return nil, "", false, e
	}
	var command = productionTestCommand(scenario.tests())
	var budget = productionTestTimeout(scenario.tests()) + time.Minute
	var receipt, artifacts, runErr = runExactSourceProcess(root, source, id, combinedValueBlockPrefix(id), directory, command, budget, combinedValueBlockEvidence.Events[id], combinedValueBlockEvidence.SourcePaths)
	if runErr != nil {
		return artifacts, "", len(artifacts) > 0, runErr
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, combinedValueBlockPrefix(id)+"-go.jsonl"))
	if readErr != nil {
		return artifacts, "", true, readErr
	}
	if e = parseStrictNamedProductionOutput(combinedValueBlockEvidence.Events[id], budget, raw); e != nil {
		return artifacts, "", true, e
	}
	if e = loopOutputWithinProcess(raw, receipt); e != nil {
		return artifacts, "", true, e
	}
	return artifacts, scenario.Scope, true, nil
}

type combinedValueBlockFacetObservation struct {
	Schema                      string             `json:"schema"`
	ImplementationSHA           string             `json:"implementation_sha"`
	Requirement                 string             `json:"requirement"`
	Result                      string             `json:"result"`
	Tests                       []string           `json:"tests"`
	Scope                       string             `json:"scope"`
	Artifacts                   []EvidenceArtifact `json:"artifacts"`
	CompleteRequirementExecuted bool               `json:"complete_requirement_executed"`
	FormalAcceptanceGranted     bool               `json:"formal_acceptance_granted"`
}

func (r *Runner) runCombinedValueBlockRequirement(id, group, title string, levels []string) bool {
	var scenario, ok = combinedValueBlockScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), scenario.Required) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "value-block producer differs from immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeCombinedValueBlock(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	if id == "F04" {
		var result = "DIAGNOSTIC_OBSERVATION"
		if err != nil {
			result = "FAILED_SUPPORTED_FACET"
		}
		var observation = combinedValueBlockFacetObservation{Schema: "cherri.value-loop-facets/v1", ImplementationSHA: r.implSHA, Requirement: id, Result: result, Tests: scenario.tests(), Scope: scenario.Scope, Artifacts: artifacts}
		var raw, _ = json.Marshal(observation)
		r.observations = append(r.observations, LocalSmokeObservation{ID: id, Passed: err == nil, Detail: string(raw), CoverageGap: combinedValueBlockF04Gap})
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: combinedValueBlockF04Gap + " Supported process: " + detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), combinedValueBlockLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		var record = &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(combinedValueBlockEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateCombinedValueBlockArtifacts(id, source string, artifacts []EvidenceArtifact) error {
	var scenario, ok = combinedValueBlockScenarios[id]
	if !ok {
		return fmt.Errorf("unknown value-block artifact scenario")
	}
	if err := validateCombinedValueBlockSourceInventory(combinedValueBlockEvidence.SourcePaths); err != nil {
		return err
	}
	var prefix = combinedValueBlockPrefix(id)
	var data, err = stateReadArtifacts(artifacts, editorContractArtifacts(prefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"]
	var budget = productionTestTimeout(scenario.tests()) + time.Minute
	if err = validateExactSourceProcess(id, source, receipt, productionTestCommand(scenario.tests()), combinedValueBlockEvidence.Events[id], raw, stderr, budget, combinedValueBlockEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(combinedValueBlockEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func validateCombinedValueBlockRecord(id string, record EvidenceRecord) error {
	var scenario, ok = combinedValueBlockScenarios[id]
	if !ok {
		return nil
	}
	if id == "F04" {
		return fmt.Errorf("%s", combinedValueBlockF04Gap)
	}
	var producer = scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(combinedValueBlockEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete value-block scenario %s", id)
	}
	return validateCombinedValueBlockArtifacts(id, record.ImplementationSHA, record.Artifacts)
}
