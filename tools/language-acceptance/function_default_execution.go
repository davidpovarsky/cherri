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

type functionDefaultScenario struct {
	Group, Title, Level, Expected, Scope string
	Tests, LocalTiers                    []string
	Producers                            map[string]string
}

var functionDefaultScenarios = map[string]functionDefaultScenario{
	"T01": {
		Group: "types", Title: "Typed defaults preserve false/zero/empty text/list/map vs omission", Level: "unit+native+iOS",
		Expected:   "Default applied only when argument absent",
		Scope:      "Independent typed-default execution: supplied positional and named false, zero, empty Text, List and Map versus omission; empty/default/nested/reserved-key Map values; Optional Map defaults distinguish omitted, None and present empty; authored fallback effects execute once in written order only in the absent branch; complete native graphs and all causal corruptions. Apple execution, general Optional joins and related requirement IDs are not inferred.",
		Tests:      []string{"TestP15T01TypedDefaultBoundaryGraphs", "TestP15T01TypedDefaultBoundaryCorruptions", "TestP15T01WrongTypedDefaultsRejected", "TestP15T01OptionalMapDefaultGraphs", "TestP15T01OptionalMapDefaultCorruptions", "TestP15T01MapDefaultEffectOrderAndScope"},
		LocalTiers: []string{"unit", "native-structure"},
		Producers:  map[string]string{"unit": "TestP15T01WrongTypedDefaultsRejected", "native-structure": "TestP15T01TypedDefaultBoundaryGraphs"},
	},
	"FN04": {
		Group: "functions", Title: "Caller input vs internal dispatcher envelope", Level: "native+iOS",
		Expected:   "system.input not silently replaced by transport dictionary",
		Scope:      "Independent caller-input execution: complete direct and nested function graphs preserve every authored system.input and ShortcutInput read, nested payload and final consumer through the guarded private dispatcher; source without functions retains actual ExtensionInput; all causal marker, envelope and input-edge corruptions reject. Apple execution and related defaults/recursion/Void requirements are not inferred.",
		Tests:      []string{"TestP15FN04CallerInputCompleteNativeGraphs", "TestP15FN04CallerInputWithoutFunctions", "TestP15FN04CallerInputCorruptions"},
		LocalTiers: []string{"native-structure"},
		Producers:  map[string]string{"native-structure": "TestP15FN04CallerInputCompleteNativeGraphs"},
	},
}

// The whole source commit is immutable; these blobs additionally bind the
// executed scenarios, their independent graph oracles, compiler transport and
// the complete acquisition/intake path. No working-tree filter supplies a hash.
var functionDefaultProductionPaths = []string{
	"go.mod", "go.sum",
	"internal/language/syntax/ast.go", "internal/language/syntax/parser.go",
	"internal/language/analysis/analyzer.go", "internal/language/analysis/function_arguments.go", "internal/language/analysis/optional_presence.go", "internal/language/analysis/optional_transport.go",
	"internal/language/types/types.go", "internal/language/types/optional_presence.go", "internal/language/ir/ir.go",
	"internal/language/lower/lower.go", "internal/language/lower/call_arguments.go", "internal/language/lower/function_defaults.go", "internal/language/lower/function_optional_presence.go", "internal/language/lower/optional_presence_demand.go", "internal/language/lower/optional_transport.go",
	"internal/language/lower/function_argument_containers.go", "internal/language/lower/container_evaluation_order.go", "internal/language/lower/container_keys.go", "internal/language/lower/empty_list.go", "internal/language/lower/literal_list.go", "internal/language/lower/binding_capture.go", "internal/language/lower/scoped_blocks.go",
	"internal/language/backend/contracts.go", "internal/language/schema/generated_registry.go",
	"language_backend_session.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go",
	"p15_function_default_boundary_test.go", "p15_function_caller_input_test.go", "p15_function_optional_map_default_test.go", "p15_function_map_default_effect_test.go",
	"p15_function_native_oracle_test.go", "p15_function_acceptance_test.go", "p15_function_optional_presence_test.go", "p15_container_shape_acceptance_test.go", "p15_types_acceptance_test.go", "p15_short_circuit_test.go", "p15_call_order_test.go", "p15_literal_collections_acceptance_test.go",
	"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/function_default_execution.go", "tools/language-acceptance/function_default_execution_test.go", "tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/loop_execution.go", "tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go", "tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go", "tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go", "tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go", "tools/language-acceptance/assertion_scope.go",
}

var functionDefaultArtifactNames = []string{"function-process.json", "function-go.jsonl", "function-stderr.log"}

func (s functionDefaultScenario) testIDs() []string {
	var ids []string
	for _, test := range s.Tests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+test)
	}
	return ids
}

func (s functionDefaultScenario) producer(tier string) string {
	if test := s.Producers[tier]; test != "" {
		return "github.com/electrikmilk/cherri:" + test
	}
	return ""
}

func functionDefaultRequiredEvents(id string) []string {
	s, found := functionDefaultScenarios[id]
	if !found {
		return nil
	}
	var names []string
	add := func(parent string, children []string) {
		names = append(names, parent)
		for _, child := range children {
			names = append(names, parent+"/"+child)
		}
	}
	if id == "T01" {
		cases := []string{"false", "zero", "empty_text", "empty_list", "empty_map", "empty_list_default", "empty_map_default", "reserved_map_default", "nested_empty_default"}
		faults := []string{"supplied_absent", "named_absent", "omitted_present", "truthiness_guard", "read_value_as_presence", "wrong_value_slot", "wrong_join", "default_branch_lost", "default_kind_lost", "caller_input_lost", "return_uses_default", "caller_consumer_detached"}
		add(s.Tests[0], cases)
		add(s.Tests[1], nil)
		for _, name := range cases {
			add(s.Tests[1]+"/"+name, faults)
			if name == "empty_list_default" {
				for _, fault := range []string{"empty_clear_lost", "empty_inherits_input", "empty_read_detached"} {
					names = append(names, s.Tests[1]+"/"+name+"/"+fault)
				}
			}
		}
		add(s.Tests[2], []string{"bool", "number", "text", "list", "map"})
		for _, parent := range s.Tests[3:5] {
			add(parent, nil)
			for _, name := range []string{"empty", "reserved", "nested_empty", "literal_braces"} {
				for _, mode := range []string{"omitted", "none", "empty"} {
					var children []string
					if parent == s.Tests[4] {
						children = []string{"default_absent", "default_kind", "default_value", "default_null", "truthiness", "wrong_slot", "wrong_join", "wrong_supplied_flag", "wrong_return"}
					}
					add(parent+"/"+name+"/"+mode, children)
				}
			}
		}
		add(s.Tests[5], nil)
		for _, name := range []string{"optional_false", "optional_true"} {
			add(s.Tests[5]+"/"+name, []string{"effects_swapped", "effect_duplicated", "effect_eager", "effect_lost", "previous_parameter_lost", "supplied_absent", "omitted_present", "return_always_default"})
		}
	} else {
		add(s.Tests[0], []string{"nested_false", "nested_true"})
		add(s.Tests[1], nil)
		add(s.Tests[2], nil)
		for _, name := range []string{"nested_false", "nested_true"} {
			faults := []string{"initial_snapshot_lost", "unguarded_envelope", "marker_key_wrong", "marker_value_wrong", "marker_truthiness", "restore_arguments", "restore_envelope", "body_reads_transport", "caller_reads_transport", "payload_loses_caller", "consumer_reads_transport", "call_uses_wrong_input"}
			if name == "nested_true" {
				faults = append(faults, "nested_payload_loses_caller", "nested_body_reads_transport")
			}
			add(s.Tests[2]+"/"+name, faults)
		}
	}
	return names
}

func parseFunctionDefaultOutput(id string, raw []byte) error {
	scenario, found := functionDefaultScenarios[id]
	if !found {
		return fmt.Errorf("unknown function-boundary requirement %s", id)
	}
	names := functionDefaultRequiredEvents(id)
	if err := parseStrictNamedProductionOutput(names, productionTestTimeout(scenario.Tests)+time.Minute, raw); err != nil {
		return err
	}
	// These real tests have nested corruption cases. Require every selected
	// ancestor to be active; a parent pass cannot precede its selected children.
	required, run, passed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range names {
		required[name] = true
	}
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event struct{ Action, Test string }
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		if event.Test == "" {
			continue
		}
		switch event.Action {
		case "run":
			for parent := event.Test; strings.Contains(parent, "/"); {
				parent = parent[:strings.LastIndexByte(parent, '/')]
				if required[parent] && (!run[parent] || passed[parent]) {
					return fmt.Errorf("function-boundary child outside selected ancestor %s", parent)
				}
			}
			run[event.Test] = true
		case "pass":
			for _, name := range names {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("function-boundary parent passed before required child %s", name)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}

func validateFunctionDefaultRecord(id string, record EvidenceRecord) error {
	scenario, found := functionDefaultScenarios[id]
	if !found {
		return nil
	}
	producer := scenario.producer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(scenario.Tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.Tests, "\n") || record.Assertions["required_go_events"] != strings.Join(functionDefaultRequiredEvents(id), "\n") || record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("%s record is detached from its complete function-boundary production; no Apple tier is implemented", id)
	}
	data, err := stateReadArtifacts(record.Artifacts, functionDefaultArtifactNames)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["function-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data["function-go.jsonl"], data["function-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		!reflect.DeepEqual(receipt.Command, record.Command) || !reflect.DeepEqual(receipt.RequiredTests, functionDefaultRequiredEvents(id)) || !reflect.DeepEqual(receipt.DiscoveredTests, functionDefaultRequiredEvents(id)) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) || receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete %s function-boundary process/source/byte binding", id)
	}
	if err := stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(scenario.Tests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err := stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, functionDefaultProductionPaths); err != nil {
		return err
	}
	if err := parseFunctionDefaultOutput(id, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}

func (r *Runner) executeFunctionDefaultProduction(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, found := functionDefaultScenarios[id]
	if !found {
		return nil, "", false, fmt.Errorf("unknown function-boundary requirement %s", id)
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("%s producer requires exact clean source: %s clean=%t error=%v", id, before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, strings.ToLower(id)+"-function-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: id, SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(scenario.Tests), RequiredTests: functionDefaultRequiredEvents(id)}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, functionDefaultProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(scenario.Tests)+time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, receipt.Command[0], receipt.Command[1:]...)
	command.Dir = root
	configureEditorProductionProcess(command)
	var output, errors loopBoundedOutput
	command.Stdout, command.Stderr = &output, &errors
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
	receipt.ExitCode, receipt.TimedOut = &exitCode, &timedOut
	after, afterClean, afterErr := editorSourceState(root)
	receipt.SourceCleanAfter = afterErr == nil && afterClean && after == before
	raw, stderr := output.Bytes(), errors.Bytes()
	receipt.RawSHA256, receipt.RawBytes = editorSHA(raw), len(raw)
	receipt.StderrSHA256, receipt.StderrBytes = editorSHA(stderr), len(stderr)
	parseErr := parseFunctionDefaultOutput(id, raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = functionDefaultRequiredEvents(id)
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{"function-process.json": append(encoded, '\n'), "function-go.jsonl": raw, "function-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range functionDefaultArtifactNames {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual %s process failed: process=%v parse=%v timeout=%t clean=%t", id, processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed complete source-bound function scenario, native graph and causal corruption inventory", true, nil
}

func (r *Runner) runFunctionDefaultRequirement(id string) bool {
	scenario, found := functionDefaultScenarios[id]
	if !found {
		return false
	}
	artifacts, detail, executed, err := r.executeFunctionDefaultProduction(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: scenario.Group, Requirement: scenario.Title, TestLevel: scenario.Level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, scenario.Group, scenario.Title, scenario.Level, scenario.LocalTiers, err == nil, "PASSED", detail+"; "+scenario.Scope)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = scenario.producer(record.Tier)
		record.Command = productionTestCommand(scenario.Tests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.Tests, "\n")
		record.Assertions["required_go_events"] = strings.Join(functionDefaultRequiredEvents(id), "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
