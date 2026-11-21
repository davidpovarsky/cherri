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

type combinedUnicodeScenario struct {
	Group, Title, Unit, Native, Scope string
}

var combinedUnicodeScenarios = map[string]combinedUnicodeScenario{
	"RP19": {"repair", "Literal and Unicode f-strings", "TestP15RP19LiteralUnicodeUnit", "TestP15RP19LiteralUnicodeNativeRoundTrip",
		"Distinct complete local joined scenario: Number 42 and Bool false literals plus real action outputs, adjacent/repeated tokens after Hebrew, non-BMP ZWJ emoji and combining marks; independent semantic and exact UTF-16/text expectations, complete native producer/consumer/output graph, causal corruptions and actual XML/binary import/recompile through Legacy/V2 in hybrid/lossless modes. Actual Apple rendered content and action-value formatting remain unproved."},
	"BRG08": {"recovery_gate", "Inline text references share canonical codec", "TestBackendRecovery08InlineCodecUnit", "TestBackendRecovery08InlineCodecNativeRoundTrip",
		"Distinct complete local joined scenario: repeated Unicode references, ordinary and raw literal braces/backslashes, one independently specified Dictionary/coercion/key/Text transform retained on direct and repeated inline paths, complete native graph and final output, typed native field and per-path corruption controls, actual XML/binary hybrid/lossless Legacy/V2 roundtrip. Actual Apple transformed values and rendered text remain unproved."},
}

// The immutable inventories are fixed after development and reviewed against
// every named clause. They are never inferred from a current process result.
//
//go:embed combined_unicode_inventory.json
var combinedUnicodeInventory []byte
var combinedUnicodeEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(combinedUnicodeInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var combinedUnicodeRequiredTiers = []string{"unit", "native-roundtrip", "ios-runtime"}
var combinedUnicodeLocalTiers = []string{"unit", "native-roundtrip"}

// This reviewed witness set covers this scenario and its actual intake helpers.
// Exact clean Git source identity separately binds the whole checkout. Preserve
// the shared 128-file, 4MiB-per-blob, 16MiB-total and 15s limits.
const combinedUnicodeSourceWitnessSHA256 = "bebcedcda5c5e62ab1c824cb171416b18028976edb22381c74cdee8533d4203d"

func validateCombinedUnicodeSourceInventory(names []string) error {
	if len(names) == 0 || len(names) > 128 {
		return fmt.Errorf("invalid bounded combined-unicode source inventory")
	}
	if !sort.StringsAreSorted(names) {
		return fmt.Errorf("unordered combined-unicode source inventory")
	}
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return fmt.Errorf("duplicate combined-unicode source witness")
		}
	}
	if editorSHA([]byte(strings.Join(names, "\n")+"\n")) != combinedUnicodeSourceWitnessSHA256 {
		return fmt.Errorf("combined-unicode inventory omits or changes reviewed source witnesses")
	}
	return nil
}

func (s combinedUnicodeScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s combinedUnicodeScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s combinedUnicodeScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-roundtrip" {
		return s.testIDs()[1]
	}
	return ""
}
func combinedUnicodePrefix(id string) string { return "combined-unicode-" + strings.ToLower(id) }

func (r *Runner) executeCombinedUnicode(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, ok := combinedUnicodeScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown joined Unicode scenario")
	}
	if err := validateCombinedUnicodeSourceInventory(combinedUnicodeEvidence.SourcePaths); err != nil {
		return nil, "", false, err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil {
		return nil, "", false, fmt.Errorf("joined Unicode source verification failed: %w", stateErr)
	}
	if !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("joined Unicode requires exact clean source: expected_head=%s observed_head=%s clean=%t", r.implSHA, source, clean)
	}
	directory, err := stateEvidenceDirectory(r, combinedUnicodePrefix(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(scenario.tests())
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, id, combinedUnicodePrefix(id), directory, command, budget, combinedUnicodeEvidence.Events[id], combinedUnicodeEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, combinedUnicodePrefix(id)+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(combinedUnicodeEvidence.Events[id], budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, scenario.Scope, true, nil
}
func (r *Runner) runCombinedUnicodeRequirement(id, group, title string, levels []string) bool {
	scenario, ok := combinedUnicodeScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), combinedUnicodeRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "joined Unicode producer differs from immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeCombinedUnicode(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), combinedUnicodeLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(combinedUnicodeEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateCombinedUnicodeRecord(id string, record EvidenceRecord) error {
	scenario, ok := combinedUnicodeScenarios[id]
	if !ok {
		return nil
	}
	if err := validateCombinedUnicodeSourceInventory(combinedUnicodeEvidence.SourcePaths); err != nil {
		return err
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(combinedUnicodeEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete joined Unicode scenario %s", id)
	}
	prefix := combinedUnicodePrefix(id)
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
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, combinedUnicodeEvidence.Events[id], raw, stderr, budget, combinedUnicodeEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(combinedUnicodeEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
