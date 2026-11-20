package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

type loopAcceptanceScenario struct {
	Group, Title, UnitTest, NativeTest, Scope string
}

var loopAcceptanceScenarios = map[string]loopAcceptanceScenario{
	"F02": {
		Group: "control", Title: "Nested for/repeat scope and zero-based indices",
		UnitTest: "TestP15F02LoopScopeUnit", NativeTest: "TestP15F02LoopNativeGraph",
		Scope: "Actual scoped analyzer types and rejection before emission; complete serialized nested for and named repeat owner/index/item/consumer graph, exact iterator order, restored outer uses and causal corruptions",
	},
	"RP22": {
		Group: "repair", Title: "Loop item and index bindings",
		UnitTest: "TestP15RP22LoopBindingsUnit", NativeTest: "TestP15RP22LoopNativeGraph",
		Scope: "Independently executed full named-repeat and tuple-for scenario with same-name outer and inner index/item shadows, zero-based captures and exact restored consumers",
	},
	"BRG12": {
		Group: "recovery_gate", Title: "Nested and mixed loop identity",
		UnitTest: "TestBackendRecovery12LoopIdentityUnit", NativeTest: "TestBackendRecovery12LoopNativeGraph",
		Scope: "Independently executed outer A/B and inner 1/2 graph, mixed repeat/for in both directions, siblings, zero-based indices, same-name shadows and outer item/index use after inner loops",
	},
}
var loopRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var loopLocalTiers = []string{"unit", "native-structure"}
var loopArtifactNames = []string{"loop-process.json", "loop-go.jsonl", "loop-stderr.log"}

var loopProductionPaths = []string{
	"internal/language/syntax/ast.go", "internal/language/syntax/parser.go", "internal/language/analysis/analyzer.go",
	"internal/language/lower/lower.go", "internal/language/lower/loop_index.go", "internal/language/lower/scoped_blocks.go",
	"internal/language/backend/contracts.go", "language_backend_session.go", "language_backend_adapter.go",
	"language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go",
	"p15_loop_index_native_test.go", "p15_loop_identity_acceptance_test.go", "p15_loop_identity_native_oracle_test.go",
	"p15_types_acceptance_test.go", "p15_parser_acceptance_test.go", "p15_short_circuit_test.go",
	"tools/language-acceptance/loop_execution.go", "tools/language-acceptance/loop_execution_test.go",
	"tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/editor_performance_execution.go", "tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go", "tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/state_acceptance_intake.go", "tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/parser_execution.go",
	"tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
	"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json", "tests/runtime_poc/CherriRuntimePOC.cherri",
}

func (scenario loopAcceptanceScenario) tests() []string {
	return []string{scenario.UnitTest, scenario.NativeTest}
}
func (scenario loopAcceptanceScenario) testIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + scenario.UnitTest, "github.com/electrikmilk/cherri:" + scenario.NativeTest}
}
func (scenario loopAcceptanceScenario) producer(tier string) string {
	if tier == "unit" {
		return "github.com/electrikmilk/cherri:" + scenario.UnitTest
	}
	if tier == "native-structure" {
		return "github.com/electrikmilk/cherri:" + scenario.NativeTest
	}
	return ""
}

// Independent inventory of every required child, including each deliberately
// corrupted native graph. A successful parent cannot hide skipped/omitted work.
func loopRequiredEvents(id string) []string {
	scenario, found := loopAcceptanceScenarios[id]
	if !found {
		return nil
	}
	names := []string{scenario.UnitTest}
	for _, child := range []string{"scoped_types", "tuple_index_does_not_escape", "tuple_item_does_not_escape", "named_repeat_index_does_not_escape", "healthy_after_rejection"} {
		names = append(names, scenario.UnitTest+"/"+child)
	}
	names = append(names, scenario.NativeTest, scenario.NativeTest+"/consistent_uuid_renaming")
	if id != "F02" {
		names = append(names, scenario.NativeTest+"/consistent_shadow_renaming")
	}
	for _, child := range []string{
		"one_based_capture", "wrong_index_owner", "wrong_item_owner", "wrong_offset", "wrong_direction",
		"string_offset", "dangling_index_edge", "duplicate_producer_identity", "reused_loop_owner", "wrong_end_owner",
		"iteration_order", "iteration_cardinality", "repeat_count", "consumer_wrong_owner", "post_inner_owner_lost",
		"capture_escaped_loop", "missing_action", "extra_action",
	} {
		names = append(names, scenario.NativeTest+"/"+child)
	}
	if id != "F02" {
		names = append(names, scenario.NativeTest+"/shadow_leaks_after_scope")
	}
	return names
}

func parseLoopProductionOutput(id string, raw []byte) error {
	return parseStrictNamedProductionOutput(loopRequiredEvents(id), productionTestTimeout(loopAcceptanceScenarios[id].tests())+time.Minute, raw)
}

func loopOutputWithinProcess(raw []byte, receipt editorProcessReceipt) error {
	lines := bytes.Split(bytes.TrimSpace(raw), []byte("\n"))
	if len(lines) == 0 {
		return fmt.Errorf("loop output has no timed events")
	}
	var first, last struct{ Time string }
	if json.Unmarshal(lines[0], &first) != nil || json.Unmarshal(lines[len(lines)-1], &last) != nil {
		return fmt.Errorf("invalid loop time bounds")
	}
	a, e1 := time.Parse(time.RFC3339Nano, first.Time)
	b, e2 := time.Parse(time.RFC3339Nano, last.Time)
	start, e3 := time.Parse(time.RFC3339Nano, receipt.StartedUTC)
	finish, e4 := time.Parse(time.RFC3339Nano, receipt.FinishedUTC)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || a.Before(start.Add(-time.Second)) || b.After(finish.Add(time.Second)) {
		return fmt.Errorf("loop events occurred outside the observed process interval")
	}
	return nil
}

func validateLoopRecord(id string, record EvidenceRecord) error {
	scenario, found := loopAcceptanceScenarios[id]
	if !found {
		return nil
	}
	producer := scenario.producer(record.Tier)
	if producer == "" {
		return fmt.Errorf("complete %s loop scenario has no executed Apple runtime producer; simple historical nested-loop traces cannot supply this tier", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(scenario.tests())) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.tests(), "\n") ||
		record.Assertions["required_go_events"] != strings.Join(loopRequiredEvents(id), "\n") ||
		record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("loop record is detached from its distinct complete scenario %s", id)
	}
	data, err := stateReadArtifacts(record.Artifacts, loopArtifactNames)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["loop-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data["loop-go.jsonl"], data["loop-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		!reflect.DeepEqual(receipt.Command, record.Command) || !reflect.DeepEqual(receipt.RequiredTests, loopRequiredEvents(id)) ||
		!reflect.DeepEqual(receipt.DiscoveredTests, loopRequiredEvents(id)) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) ||
		receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete loop process/source/byte binding for %s", id)
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(scenario.tests()) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, loopProductionPaths); err != nil {
		return err
	}
	if err := parseLoopProductionOutput(id, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}

type loopBoundedOutput struct{ bytes.Buffer }

func (out *loopBoundedOutput) Write(data []byte) (int, error) {
	if out.Len()+len(data) > 4<<20 {
		return 0, fmt.Errorf("loop process output exceeds retained limit")
	}
	return out.Buffer.Write(data)
}

func (r *Runner) executeLoopProduction(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, found := loopAcceptanceScenarios[id]
	if !found {
		return nil, "", false, fmt.Errorf("unknown loop scenario")
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, err := editorSourceState(root)
	if err != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("loop producer requires exact clean source: %s clean=%t error=%v", before, clean, err)
	}
	directory, err := stateEvidenceDirectory(r, "loop-"+strings.ToLower(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: id, SourceCommit: before, SourceCleanBefore: true,
		Command: productionTestCommand(scenario.tests()), RequiredTests: loopRequiredEvents(id)}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, loopProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(scenario.tests())+time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, receipt.Command[0], receipt.Command[1:]...)
	command.Dir = root
	configureEditorProductionProcess(command)
	var out, errors loopBoundedOutput
	command.Stdout = &out
	command.Stderr = &errors
	started := time.Now()
	receipt.StartedUTC = started.UTC().Format(time.RFC3339Nano)
	processErr := command.Run()
	receipt.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.ElapsedSeconds = time.Since(started).Seconds()
	exitCode := -1
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	timedOut := ctx.Err() != nil
	receipt.ExitCode = &exitCode
	receipt.TimedOut = &timedOut
	after, afterClean, stateErr := editorSourceState(root)
	receipt.SourceCleanAfter = stateErr == nil && afterClean && after == before
	raw, stderr := out.Bytes(), errors.Bytes()
	receipt.RawSHA256 = editorSHA(raw)
	receipt.RawBytes = len(raw)
	receipt.StderrSHA256 = editorSHA(stderr)
	receipt.StderrBytes = len(stderr)
	parseErr := parseLoopProductionOutput(id, raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = loopRequiredEvents(id)
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{"loop-process.json": append(encoded, '\n'), "loop-go.jsonl": raw, "loop-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range loopArtifactNames {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		tail := stderr
		if len(tail) > 4096 {
			tail = tail[len(tail)-4096:]
		}
		return artifacts, "", command.Process != nil, fmt.Errorf("actual %s loop process failed: process=%v parse=%v timeout=%t clean=%t stderr=%s", id, processErr, parseErr, timedOut, receipt.SourceCleanAfter, tail)
	}
	return artifacts, "Executed complete " + id + " loop scenario with all required positive/causal children; original Apple fixture and golden unchanged", true, nil
}

func (r *Runner) runLoopRequirement(id, group, title string, levels []string) bool {
	scenario, found := loopAcceptanceScenarios[id]
	if !found {
		return false
	}
	if group != scenario.Group || title != scenario.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), loopRequiredTiers) {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Passed: false, Status: "FAILED", Detail: "Loop producer does not match the immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeLoopProduction(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: group, Requirement: title, TestLevel: strings.Join(levels, "+"), Passed: false, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, group, title, strings.Join(levels, "+"), loopLocalTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.tests())
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.tests(), "\n")
		record.Assertions["required_go_events"] = strings.Join(loopRequiredEvents(id), "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
