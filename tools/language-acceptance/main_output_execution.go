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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

type mainOutputScenario struct {
	Group, Title, Level, Expected, Scope string
	Tests, LocalTiers                    []string
	Producers                            map[string]string
}

var mainOutputScenarios = map[string]mainOutputScenario{
	"M02": {
		Group: "metadata", Title: "Main body explicit return and Void fallthrough", Level: "native+iOS",
		Expected:   "No accidental last-action result leakage",
		Scope:      "Independent M02 actual canonical compilation of complete main-output graphs: empty/default/explicit-mode Void fallthrough clears inherited or incidental values, partial/zero-iteration paths retain the empty boundary, explicit Text/empty Text/false/zero/bare/None returns consume the exact authored value and stop, and child returns preserve parent continuation. Actual terminal raw/resolved actions, full native.workflow documents and explicit Legacy entry mode retain their defined output. Shared analyzer/service/checked-lowering mode validation rejects malformed or duplicate metadata before emission; real entry/module migration and idempotence preserve context and exact native output. Complete native graph and causal mutations are mandatory. Apple execution and BRG14/RP26/RP30 or other related scopes are not inferred.",
		Tests:      []string{"TestP15M02MainVoidFallthrough", "TestP15M02ExplicitMainReturnBaseline", "TestP15M02CompleteMainGraphs", "TestP15M02CompleteGraphCausalControls", "TestP15M02ExactNativeWorkflowBoundary", "TestP15M02MigratedEntryOutput", "TestP15M02ValidatedModeRoutes", "TestP15M02InvalidModeFailsBeforeEmission", "TestP15M02MigrationUsesEntryContext", "TestP15M02MigrationIsIdempotent", "TestP15M02MigrationRejectsInvalidExistingMode"},
		LocalTiers: []string{"native-structure"},
		Producers:  map[string]string{"native-structure": "TestP15M02CompleteMainGraphs"},
	},
}

// The whole source commit is immutable; these blobs additionally bind the
// executed scenarios, their independent graph oracles, compiler transport and
// the complete acquisition/intake path. No working-tree filter supplies a hash.
var mainOutputProductionPaths = []string{
	"go.mod",
	"go.sum",
	"internal/language/syntax/ast.go",
	"internal/language/syntax/lexer.go",
	"internal/language/syntax/parser.go",
	"internal/language/syntax/format.go",
	"internal/language/syntax/main_output.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/analysis/diagnostics.go",
	"internal/language/analysis/optional_presence.go",
	"internal/language/analysis/optional_transport.go",
	"internal/language/types/types.go",
	"internal/language/types/optional_presence.go",
	"internal/language/ir/ir.go",
	"internal/language/lower/lower.go",
	"internal/language/lower/main_output.go",
	"internal/language/lower/exhaustive_validation.go",
	"internal/language/lower/module_scope.go",
	"internal/language/lower/function_optional_presence.go",
	"internal/language/lower/function_defaults.go",
	"internal/language/lower/function_argument_containers.go",
	"internal/language/lower/optional_presence_demand.go",
	"internal/language/lower/optional_transport.go",
	"internal/language/lower/binding_capture.go",
	"internal/language/lower/call_arguments.go",
	"internal/language/lower/scoped_blocks.go",
	"internal/language/migrate/migrate.go",
	"internal/language/migrate/main_output.go",
	"internal/language/migrate/lexical_source.go",
	"internal/language/migrate/references.go",
	"internal/language/migrate/optional_bindings.go",
	"internal/language/modules/graph.go",
	"internal/language/modules/standard_library.go",
	"internal/language/modules/stdfunc_source_generated.go",
	"internal/language/service/service.go",
	"internal/language/service/module_analysis.go",
	"internal/language/backend/contracts.go",
	"internal/language/schema/generated_registry.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"language_emit_adapter.go",
	"language_value_adapter.go",
	"shared_codec.go",
	"shortcutgen.go",
	"p15_main_void_return_test.go",
	"p15_main_output_mode_test.go",
	"p15_main_output_graph_test.go",
	"p15_main_output_oracle_test.go",
	"p15_none_clear_boundary_test.go",
	"p15_function_native_oracle_test.go",
	"p15_function_acceptance_test.go",
	"p15_function_optional_presence_test.go",
	"p15_container_shape_acceptance_test.go",
	"p15_types_acceptance_test.go",
	"p15_short_circuit_test.go",
	"p15_call_order_test.go",
	"p15_literal_collections_acceptance_test.go",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/main_output_execution.go",
	"tools/language-acceptance/main_output_execution_test.go",
	"tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
}

var mainOutputArtifactNames = []string{"main-output-process.json", "main-output-go.jsonl", "main-output-stderr.log"}

func (s mainOutputScenario) testIDs() []string {
	var ids []string
	for _, test := range s.Tests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+test)
	}
	return ids
}

func (s mainOutputScenario) producer(tier string) string {
	if test := s.Producers[tier]; test != "" {
		return "github.com/electrikmilk/cherri:" + test
	}
	return ""
}

func mainOutputRequiredEvents(id string) []string {
	scenario, found := mainOutputScenarios[id]
	if !found {
		return nil
	}
	var names []string
	add := func(parent string, children ...string) {
		names = append(names, parent)
		for _, child := range children {
			names = append(names, parent+"/"+child)
		}
	}
	add(scenario.Tests[0], "empty", "unused_text", "unused_binding", "void_action_after_value", "partial_early_return", "function_caller_fallthrough")
	add(scenario.Tests[1], "text", "empty_text", "false", "zero", "bare", "none")
	cases := []string{"empty_input_boundary", "unused_text", "unused_number_binding", "void_action_after_value", "partial_early_return", "both_branches_return", "zero_iteration_return", "child_return_parent_falls_through", "child_bare_return_parent_continues", "parent_explicit_return_after_child", "raw_nothing_then_resolved_text", "native_word_is_only_text", "legacy_mode_preserves_incidental_result", "authored_native_output", "authored_native_stop", "explicit_mode_clears_incidental_result", "return_text", "return_empty_text", "return_false", "return_zero", "return_empty_", "return_empty_none"}
	add(scenario.Tests[2], cases...)
	add(scenario.Tests[3])
	for _, name := range cases {
		faults := []string{"delete_terminal", "append_stale", "alter_first_parameter"}
		if name == "return_text" || name == "return_empty_text" || name == "return_false" || name == "return_zero" {
			faults = append(faults, "return_value", "return_stale_producer", "return_missing_output")
		}
		add(scenario.Tests[3]+"/"+name, faults...)
	}
	add(scenario.Tests[4], "actions_0", "actions_1")
	add(scenario.Tests[5], "source_0", "source_20", "source_51")
	add(scenario.Tests[6], "default", "explicit", "legacy")
	add(scenario.Tests[7], "unknown", "wrong_case", "empty", "number", "boolean", "none", "dynamic", "call", "list", "duplicate_field", "duplicate_header")
	add(scenario.Tests[8])
	add(scenario.Tests[9], "empty", "legacy_body", "legacy_header", "existing_v2_header", "existing_legacy_mode", "existing_explicit_mode", "decoy_comment", "decoy_literal")
	add(scenario.Tests[10])
	return names
}

func mainOutputSourceFiles(repository, commit string) ([]editorSourceFile, error) {
	if len(mainOutputProductionPaths) == 0 || len(mainOutputProductionPaths) > 128 {
		return nil, fmt.Errorf("invalid bounded M02 source inventory")
	}
	seen := map[string]bool{}
	for _, name := range mainOutputProductionPaths {
		if seen[name] {
			return nil, fmt.Errorf("duplicate M02 source path")
		}
		seen[name] = true
	}
	if err := mainOutputVerifySourceBudget(repository, commit); err != nil {
		return nil, err
	}
	var files []editorSourceFile
	for start := 0; start < len(mainOutputProductionPaths); start += 64 {
		end := start + 64
		if end > len(mainOutputProductionPaths) {
			end = len(mainOutputProductionPaths)
		}
		batch, err := editorGitSourceFiles(repository, commit, mainOutputProductionPaths[start:end])
		if err != nil {
			return nil, err
		}
		files = append(files, batch...)
	}
	return files, nil
}

var mainOutputSourceBudgetCache sync.Map

func mainOutputVerifySourceBudget(repository, commit string) error {
	if !documentationHash(commit, 40) {
		return fmt.Errorf("invalid M02 immutable source commit")
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	key := root + "\x00" + commit + "\x00" + strings.Join(mainOutputProductionPaths, "\n")
	if _, found := mainOutputSourceBudgetCache.Load(key); found {
		return nil
	}
	var requests strings.Builder
	for _, name := range mainOutputProductionPaths {
		requests.WriteString(commit + ":" + name + "\n")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "-C", root, "cat-file", "--batch-check")
	command.Stdin = strings.NewReader(requests.String())
	command.WaitDelay = 2 * time.Second
	var output editorGitSourceOutput
	var errors bytes.Buffer
	command.Stdout, command.Stderr = &output, &errors
	if err := command.Run(); err != nil {
		return fmt.Errorf("immutable M02 source size preflight unavailable: %w", err)
	}
	if err := mainOutputSourceBudget(output.Bytes(), len(mainOutputProductionPaths)); err != nil {
		return err
	}
	mainOutputSourceBudgetCache.Store(key, true)
	return nil
}
func mainOutputSourceBudget(raw []byte, required int) error {
	if required < 1 || required > 128 || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return fmt.Errorf("invalid bounded M02 source size preflight")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	if len(lines) != required {
		return fmt.Errorf("incomplete M02 source size inventory")
	}
	var total int64
	for _, line := range lines {
		header := strings.Fields(string(line))
		if len(header) != 3 || !documentationHash(header[0], 40) || header[1] != "blob" {
			return fmt.Errorf("invalid immutable M02 source object")
		}
		size, err := strconv.ParseInt(header[2], 10, 64)
		if err != nil || size < 0 || size > 4<<20 {
			return fmt.Errorf("invalid M02 source object size")
		}
		total += int64(len(line)) + 1 + size + 1
		if total > 16<<20 {
			return fmt.Errorf("M02 source aggregate exceeds16MiB")
		}
	}
	return nil
}

func parseMainOutputOutput(id string, raw []byte) error {
	scenario, found := mainOutputScenarios[id]
	if !found {
		return fmt.Errorf("unknown main-output requirement %s", id)
	}
	names := mainOutputRequiredEvents(id)
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
					return fmt.Errorf("main-output child outside selected ancestor %s", parent)
				}
			}
			run[event.Test] = true
		case "pass":
			for _, name := range names {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("main-output parent passed before required child %s", name)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}

func validateMainOutputRecord(id string, record EvidenceRecord) error {
	scenario, found := mainOutputScenarios[id]
	if !found {
		return nil
	}
	producer := scenario.producer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(scenario.Tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(scenario.Tests, "\n") || record.Assertions["required_go_events"] != strings.Join(mainOutputRequiredEvents(id), "\n") || record.Assertions["scope"] != scenario.Scope {
		return fmt.Errorf("%s record is detached from its complete main-output production; no Apple tier is implemented", id)
	}
	data, err := stateReadArtifacts(record.Artifacts, mainOutputArtifactNames)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err := decodeEditorJSON(data["main-output-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data["main-output-go.jsonl"], data["main-output-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		!reflect.DeepEqual(receipt.Command, record.Command) || !reflect.DeepEqual(receipt.RequiredTests, mainOutputRequiredEvents(id)) || !reflect.DeepEqual(receipt.DiscoveredTests, mainOutputRequiredEvents(id)) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) || receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete %s main-output process/source/byte binding", id)
	}
	if err := stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(scenario.Tests) + time.Minute).Seconds()); err != nil {
		return err
	}
	immutable, err := mainOutputSourceFiles(resolveRepoRoot(), record.ImplementationSHA)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(receipt.SourceFiles, immutable) {
		return fmt.Errorf("M02 immutable source manifest mismatch")
	}
	if err := parseMainOutputOutput(id, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}

func (r *Runner) executeMainOutputProduction(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario, found := mainOutputScenarios[id]
	if !found {
		return nil, "", false, fmt.Errorf("unknown main-output requirement %s", id)
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("%s producer requires exact clean source: %s clean=%t error=%v", id, before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, strings.ToLower(id)+"-main-output-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: id, SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(scenario.Tests), RequiredTests: mainOutputRequiredEvents(id)}
	receipt.SourceFiles, err = mainOutputSourceFiles(root, before)
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
	parseErr := parseMainOutputOutput(id, raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = mainOutputRequiredEvents(id)
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{"main-output-process.json": append(encoded, '\n'), "main-output-go.jsonl": raw, "main-output-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range mainOutputArtifactNames {
		path := filepath.Join(directory, name)
		if err := os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual %s process failed: process=%v parse=%v timeout=%t clean=%t", id, processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed complete source-bound M02 main-output scenario, native graphs, mode/migration routes and causal corruption inventory", true, nil
}

func (r *Runner) runMainOutputRequirement(id string) bool {
	scenario, found := mainOutputScenarios[id]
	if !found {
		return false
	}
	artifacts, detail, executed, err := r.executeMainOutputProduction(id)
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
		record.Assertions["required_go_events"] = strings.Join(mainOutputRequiredEvents(id), "\n")
		record.Assertions["scope"] = scenario.Scope
	}
	return true
}
