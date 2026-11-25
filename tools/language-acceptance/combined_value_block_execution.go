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
	ObservationTests                  []string
}

const combinedValueBlockF04Gap = "The complete F04 scenario is NOT_RUN: closed Count and enrolled immutable local cells preserve one Dictionary cell per normal iteration with explicit payload presence; general nested List and Optional sequence export, function/native/aggregate interfaces, mutable or widened aliases, and unenrolled consumers remain unsupported. These bounded observations do not grant complete F04 tier credit."

var combinedValueBlockF04ObservationTests = []string{
	"TestP15F04CellPayloadContextBoundaries",
	"TestP15F04ClosedItemsCountGraphCorruptions",
	"TestP15F04ClosedItemsCountNativeGraphs",
	"TestP15F04ClosedItemsCountPreservesUnsupportedBoundaries",
	"TestP15F04KnownListLoopResultsFailClosed",
	"TestP15F04LocalCellAdmissionBoundaries",
	"TestP15F04LocalCellBoundaryPromotions",
	"TestP15F04LocalCellConsumerNativeGraphs",
	"TestP15F04LocalCellGraphRejectsCorruption",
	"TestP15F04LocalCellOptionalDemand",
	"TestP15F04LocalCellScopeRestoration",
	"TestP15F04LocalCellUnsupportedInterfaces",
	"TestP15F04OptionalLoopPresentScalarGraphs",
	"TestP15F04OptionalLoopResultsFailClosed",
}

var combinedValueBlockScenarios = map[string]combinedValueBlockScenario{
	"F04": {"control", "Value loop collects one semantic element per iteration", "TestP15F04ValueLoopBoundaryUnit", "TestP15F04ValueLoopBoundaryNative",
		"Bounded supported facets only: original scalar/for-item graphs plus closed Items Count and enrolled immutable local bindings, aliases, checked indexing, decoded for iteration, independent presence and scope restoration. Complete native graph and causal controls cover zero/multiple iterations, None/false/zero/empty payloads and private-cell escape rejection. General sequence and interface support remains incomplete; no complete F04 tier credit or Apple execution is claimed.", []string{"unit", "native-structure"}, combinedValueBlockF04ObservationTests},
	"RP23": {"repair", "Value block and loop yields", "TestP15RP23ValueBlockLoopUnit", "TestP15RP23ValueBlockLoopNative",
		"Distinct local joined scenario: Number if yields, Bool menu yields, zero/multiple Text loop yields, two-item for results and one final consumer. Exact semantic types, all native branch/result edges and absence of overwritten outputs are independently checked; missing/illegal/overwritten yields have causal source-bound diagnostics and no emitted artifact. This local supported scenario supplies no F04 nested List/Optional or Apple result evidence.", []string{"unit", "native-structure"}, nil},
	"BRG14": {"recovery_gate", "Value blocks, loop results and return", "TestBackendRecovery14ValueBlockReturnUnit", "TestBackendRecovery14ValueBlockReturnNative",
		"Distinct joined supported value-block scenario plus actual complete main/child early-return, bare-return, both-branch return and zero-iteration return graphs. Typed values, complete dispatch/function/caller ancestry, exact output/Nothing boundaries and final unused Text are independently specified and causally corrupted. No M02/F04/FN evidence inheritance; nested List/Optional transport and actual Apple value/cardinality/frame outcomes remain unproved.", []string{"unit", "native-structure", "ios-runtime"}, nil},
}

//go:embed combined_value_block_inventory.json
var combinedValueBlockInventory []byte
var combinedValueBlockEvidence = func() struct {
	Events         map[string][]string `json:"events"`
	SourcePaths    []string            `json:"source_paths"`
	F04SourcePaths []string            `json:"f04_source_paths"`
} {
	var value struct {
		Events         map[string][]string `json:"events"`
		SourcePaths    []string            `json:"source_paths"`
		F04SourcePaths []string            `json:"f04_source_paths"`
	}
	if err := json.Unmarshal(combinedValueBlockInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var combinedValueBlockLocalTiers = []string{"unit", "native-structure"}

const combinedValueBlockSourceWitnessSHA256 = "8815479e1809972ff46ad19856b708a6cef1f994137b1250e1914d18bb096f6c"

const combinedValueBlockF04TestsSHA256 = "a960a6db2660a4982e5edb0b933163621cb27f45c734c7dac4e39837c3626ae3"
const combinedValueBlockF04EventsSHA256 = "08cf4bf7caeb0a07dc2b0b0ff187c2e702617803e8a666661a6da507fe416da0"
const combinedValueBlockF04SourceWitnessSHA256 = "9a2c6d6ca839e4249f9803404b72756a674930800507e4b1f7908d7c0f07e127"

func validateCombinedValueBlockSourceInventory(names []string) error {
	return validateCombinedValueBlockSourceWitness(names, combinedValueBlockSourceWitnessSHA256)
}

func validateCombinedValueBlockSourceWitness(names []string, expected string) error {
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
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != expected {
		return fmt.Errorf("combined-value-block inventory omits or changes reviewed source witnesses")
	}
	return nil
}
func (s combinedValueBlockScenario) tests() []string {
	return append([]string{s.Unit, s.Native}, s.ObservationTests...)
}
func (s combinedValueBlockScenario) testIDs() []string {
	var ids []string
	for _, name := range s.tests() {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
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

func combinedValueBlockSourcePaths(id string) []string {
	if id == "F04" {
		return combinedValueBlockEvidence.F04SourcePaths
	}
	return combinedValueBlockEvidence.SourcePaths
}

// F04 has a fixed observation family. Existing process, source and event
// validators remain authoritative; no observation becomes an acceptance record.
func validateCombinedValueBlockF04ObservationInventory(tests, events, paths []string) error {
	if len(tests) != 16 || editorSHA([]byte(strings.Join(tests, "\n")+"\n")) != combinedValueBlockF04TestsSHA256 {
		return fmt.Errorf("F04 observation parent inventory changed")
	}
	if len(events) != 242 || !sort.StringsAreSorted(events) ||
		editorSHA([]byte(strings.Join(events, "\n")+"\n")) != combinedValueBlockF04EventsSHA256 {
		return fmt.Errorf("F04 observation named-event inventory changed")
	}
	parents := map[string]bool{}
	for _, name := range tests {
		if parents[name] || strings.Contains(name, "/") || !strings.HasPrefix(name, "TestP15F04") {
			return fmt.Errorf("F04 observation parent is duplicate or foreign")
		}
		parents[name] = true
	}
	seen := map[string]bool{}
	for _, name := range events {
		if seen[name] || !parents[strings.SplitN(name, "/", 2)[0]] {
			return fmt.Errorf("F04 observation event is duplicate or foreign")
		}
		seen[name] = true
	}
	for name := range parents {
		if !seen[name] {
			return fmt.Errorf("F04 observation parent event missing")
		}
	}
	return validateCombinedValueBlockSourceWitness(paths, combinedValueBlockF04SourceWitnessSHA256)
}

func validateCombinedValueBlockScenarioInventory(id string) error {
	if id == "F04" {
		return validateCombinedValueBlockF04ObservationInventory(
			combinedValueBlockScenarios[id].tests(), combinedValueBlockEvidence.Events[id],
			combinedValueBlockSourcePaths(id))
	}
	return validateCombinedValueBlockSourceInventory(combinedValueBlockSourcePaths(id))
}

func combinedValueBlockPrefix(id string) string { return "combined-value-block-" + strings.ToLower(id) }
func (r *Runner) executeCombinedValueBlock(id string) ([]EvidenceArtifact, string, bool, error) {
	var scenario, ok = combinedValueBlockScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown value-block scenario")
	}
	if err := validateCombinedValueBlockScenarioInventory(id); err != nil {
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
	var receipt, artifacts, runErr = runExactSourceProcess(root, source, id, combinedValueBlockPrefix(id), directory, command, budget, combinedValueBlockEvidence.Events[id], combinedValueBlockSourcePaths(id))
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
	if err := validateCombinedValueBlockScenarioInventory(id); err != nil {
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
	if err = validateExactSourceProcess(id, source, receipt, productionTestCommand(scenario.tests()), combinedValueBlockEvidence.Events[id], raw, stderr, budget, combinedValueBlockSourcePaths(id)); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(combinedValueBlockEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func validateCombinedValueBlockRecord(id string, record EvidenceRecord) error {
	if id == "F04" && record.TestID == structuredValueScenarios["F04"].producer(record.Tier) && record.TestID != "" {
		return validateStructuredValueRecord(id, record)
	}
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
