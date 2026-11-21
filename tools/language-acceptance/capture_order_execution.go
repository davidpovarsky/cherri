package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"time"
)

type captureOrderScenario struct {
	Group, Title, Unit, Native, Scope string
}

var captureOrderScenarios = map[string]captureOrderScenario{
	"RP14": {"repair", "Binding capture and native types", "TestP15RP14TypedCaptureUnit", "TestP15RP14TypedCaptureNative", "Independent complete local typed-capture scenario: Number, Bool, Text, List, Map and Clipboard saved before later writes; semantic type checks, exact saved-producer consumers, numeric zero, false, empty Text and explicit empty array/dictionary fields, no generic Text conversion. Standalone Bool uses the established numeric carrier; actual Apple saved values and native runtime types remain unproved."},

	"RP12": {"repair", "Named argument effect order",
		"TestP15RP12NamedEffectsUnit", "TestP15RP12NamedEffectsNative",
		"Independent complete local written-order scenario: reverse/mixed/positional/alias/native calls, producer ownership and once-only graphs, actual flow/duplicate/unknown-label rejection, default guards and scoped scalar/aggregate argument snapshots. Actual Apple evaluation effects remain unproved."},
	"BRG10": {"recovery_gate", "Capture and written-order evaluation",
		"TestBackendRecovery10CaptureOrderUnit", "TestBackendRecovery10CaptureOrderNative",
		"Independent combined local capture/order scenario: one real compiled program captures mutable value and clipboard before later writes, consumes each saved producer twice and supplies labelled effects out of schema order; complete snapshot/scope/order and terminal-consumer causal controls. Actual Apple saved values and evaluation effects remain unproved."},
}

// The immutable inventories are fixed after development and reviewed against
// every named clause. They are never inferred from a current process result.
//
//go:embed capture_order_inventory.json
var captureOrderInventory []byte
var captureOrderEvidence = func() struct {
	Events      map[string][]string `json:"events"`
	SourcePaths []string            `json:"source_paths"`
} {
	var value struct {
		Events      map[string][]string `json:"events"`
		SourcePaths []string            `json:"source_paths"`
	}
	if err := json.Unmarshal(captureOrderInventory, &value); err != nil {
		panic(err)
	}
	return value
}()
var captureOrderRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var captureOrderLocalTiers = []string{"unit", "native-structure"}

func (s captureOrderScenario) tests() []string { return []string{s.Unit, s.Native} }
func (s captureOrderScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + s.Unit, "github.com/electrikmilk/cherri:" + s.Native}
}
func (s captureOrderScenario) producer(tier string) string {
	if tier == "unit" {
		return s.testIDs()[0]
	}
	if tier == "native-structure" {
		return s.testIDs()[1]
	}
	return ""
}
func captureOrderPrefix(id string) string { return "capture-order-" + strings.ToLower(id) }

func (r *Runner) executeCaptureOrder(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, ok := captureOrderScenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown capture/order scenario")
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	source, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("capture/order requires exact clean source")
	}
	directory, err := stateEvidenceDirectory(r, captureOrderPrefix(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	command := productionTestCommand(scenario.tests())
	budget := productionTestTimeout(scenario.tests()) + time.Minute
	receipt, artifacts, err := runExactSourceProcess(root, source, id, captureOrderPrefix(id), directory, command, budget, captureOrderEvidence.Events[id], captureOrderEvidence.SourcePaths)
	if err != nil {
		return artifacts, "", len(artifacts) > 0, err
	}
	raw, err := os.ReadFile(filepath.Join(directory, captureOrderPrefix(id)+"-go.jsonl"))
	if err != nil {
		return artifacts, "", true, err
	}
	if err = parseStrictNamedProductionOutput(captureOrderEvidence.Events[id], budget, raw); err != nil {
		return artifacts, "", true, err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return artifacts, "", true, err
	}
	return artifacts, scenario.Scope, true, nil
}
func (r *Runner) runCaptureOrderRequirement(id, group, title string, levels []string) bool {
	scenario, ok := captureOrderScenarios[id]
	if !ok {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), captureOrderRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "capture/order producer differs from immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeCaptureOrder(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), captureOrderLocalTiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(captureOrderEvidence.Events[id], "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
func validateCaptureOrderRecord(id string, record EvidenceRecord) error {
	scenario, ok := captureOrderScenarios[id]
	if !ok {
		return nil
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s has no verified actual Apple runtime producer", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(captureOrderEvidence.Events[id], "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("detached complete capture/order scenario %s", id)
	}
	prefix := captureOrderPrefix(id)
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
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, captureOrderEvidence.Events[id], raw, stderr, budget, captureOrderEvidence.SourcePaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(captureOrderEvidence.Events[id], budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
