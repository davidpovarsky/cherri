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

const structuralMigrationScope = "Independent RP33 execution of the complete metadata/setup/ref/include/interpolation/index/has-value/function scenario through structural migration and actual Legacy/V2 native comparison. Exact import-question targets, raw native references, rawAction compatibility, canonical native payloads, source-relative includes, lexical scope and corruption controls are mandatory. Ambiguous Legacy includes inside multiline literals remain explicitly rejected with originals preserved. No linked acceptance or BRG19 credit is inherited."
const structuralMigrationScenario = "Migrate a program containing metadata/setup/ref directives, old interpolation, indices, includes, has-value comparisons and functions."
const structuralMigrationExpected = "Meaning is preserved through structural transformation; no global directive deletion/regex-only rewrite."

var structuralMigrationTests = []string{
	"TestP15RP33StructuralMigrationProgram",
	"TestP15RP33ReferenceLexicalScope",
	"TestP15RP33SyntaxAwareMigrationDifferential",
	"TestP15RP33ReferenceRawParameter",
	"TestP15RP33RawCarrierDetachment",
	"TestP15RP33CanonicalNativePayload",
	"TestP15RP33AmbiguousLegacyIncludeFailsClosed",
	"TestP15RP33SourceRelativeInclude",
}
var structuralMigrationTiers = []string{"migration", "native-differential"}
var structuralMigrationArtifacts = []string{"rp33-process.json", "rp33-go.jsonl", "rp33-stderr.log"}
var structuralMigrationSyntaxControls = []string{"metadata", "setup", "include", "interpolation", "reference", "index", "has_value", "legacy_repeat"}
var structuralMigrationNativeCases = []string{
	"literal_text_control", "raw_backslash_text", "full_program", "literal_condition", "escaped_interpolation", "raw_interpolation_text",
	"directive_in_block_comment", "directive_in_multiline_literal", "json_escape_rules", "legacy_unknown_escape", "placeholder_collision",
	"literal_braces_with_interpolation", "include_in_comment", "raw_action_literal", "raw_action_dollar_reference", "raw_action_mixed_text",
	"raw_action_literal_key", "reference_callable_collision",
}
var structuralMigrationQuestionControls = []string{"target_zero", "target_previous", "target_next", "parameter_key", "prompt", "default"}
var structuralMigrationProductionPaths = []string{
	"internal/language/migrate/migrate.go", "internal/language/migrate/lexical_source.go", "internal/language/migrate/references.go",
	"internal/language/migrate/optional_bindings.go", "internal/language/migrate/variadic.go", "internal/language/migrate/native_lists.go",
	"internal/language/syntax/ast.go", "internal/language/syntax/parser.go", "internal/language/syntax/lexer.go", "internal/language/syntax/format.go",
	"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/loop_index.go",
	"internal/language/backend/contracts.go", "internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"language_cli_adapter.go", "cmd/cherri-migrate/main.go", "tools/language-migrate/main.go", "tools/legacy-parity/main.go",
	"language_backend_session.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go",
	"parser.go", "includes.go", "raw_actions.go", "action.go", "actions_std.go", "actions/basic.cherri", "actions/documents.cherri",
	"internal/shortcutcompare/compare.go", "internal/shortcutcompare/normalize.go", "internal/shortcutcompare/question_targets.go",
	"internal/shortcutcompare/strict_scalars.go", "internal/shortcutcompare/p15_snapshot_symbols.go",
	"p15_migration_structure_test.go", "p15_migration_structural_acceptance_test.go", "p15_migration_reference_wire_test.go",
	"p15_migration_question_target_test.go", "p15_migration_native_literal_test.go", "p15_migration_ambiguous_include_test.go",
	"p15_migration_include_path_test.go", "p15_cli_acceptance_test.go",
	"tests/language-v2/repair-regression-plan.json", "tests/language-v2/acceptance-contract.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/structural_migration_execution.go", "tools/language-acceptance/structural_migration_execution_test.go",
	"tools/language-acceptance/strict_go_events.go", "tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go", "tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
}

func structuralMigrationRequiredEvents() []string {
	names := append([]string{}, structuralMigrationTests...)
	add := func(parent string, children []string) {
		for _, name := range children {
			names = append(names, parent+"/"+name)
		}
	}
	add(structuralMigrationTests[0], structuralMigrationSyntaxControls)
	add(structuralMigrationTests[2], structuralMigrationNativeCases)
	for document := 0; document < 2; document++ {
		add(structuralMigrationTests[2]+fmt.Sprintf("/full_program/question_document_%d", document), structuralMigrationQuestionControls)
	}
	add(structuralMigrationTests[3], []string{"json_reference", "nested_carrier", "opaque_typed_payload"})
	for _, name := range []string{"plain_parameter", "explicit_raw_string", "explicit_native_plist"} {
		add(structuralMigrationTests[5], []string{name, name + "/payload_mutation"})
	}
	add(structuralMigrationTests[6], []string{"preview", "write"})
	add(structuralMigrationTests[7], []string{"preview", "write"})
	return names
}

func structuralMigrationTestIDs() []string {
	var ids []string
	for _, name := range structuralMigrationTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func structuralMigrationProducer(tier string) string {
	if tier == "migration" {
		return structuralMigrationTestIDs()[0]
	}
	if tier == "native-differential" {
		return structuralMigrationTestIDs()[2]
	}
	return ""
}
func parseStructuralMigrationOutput(raw []byte) error {
	if err := parseStrictNamedProductionOutput(structuralMigrationRequiredEvents(), productionTestTimeout(structuralMigrationTests)+time.Minute, raw); err != nil {
		return err
	}
	return structuralMigrationNestedEvents(raw)
}
func validateStructuralMigrationRecord(id string, record EvidenceRecord) error {
	if id != "RP33" {
		return nil
	}
	var producer = structuralMigrationProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"RP33"}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(structuralMigrationTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(structuralMigrationTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(structuralMigrationRequiredEvents(), "\n") ||
		record.Assertions["scope"] != structuralMigrationScope {
		return fmt.Errorf("RP33 record is detached from complete syntax-aware migration production")
	}
	var data, err = stateReadArtifacts(record.Artifacts, structuralMigrationArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["rp33-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data["rp33-go.jsonl"], data["rp33-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "RP33" || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, structuralMigrationRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, structuralMigrationRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete RP33 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(structuralMigrationTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, structuralMigrationProductionPaths); err != nil {
		return err
	}
	if err = parseStructuralMigrationOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeStructuralMigrationProduction() ([]EvidenceArtifact, string, bool, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("RP33 producer requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	var directory, dirErr = stateEvidenceDirectory(r, "rp33-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: "RP33", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(structuralMigrationTests), RequiredTests: structuralMigrationRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, structuralMigrationProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(structuralMigrationTests)+time.Minute)
	defer cancel()
	var command = exec.CommandContext(ctx, receipt.Command[0], receipt.Command[1:]...)
	command.Dir = root
	configureEditorProductionProcess(command)
	var output, errors loopBoundedOutput
	command.Stdout = &output
	command.Stderr = &errors
	var started = time.Now()
	receipt.StartedUTC = started.UTC().Format(time.RFC3339Nano)
	var processErr = command.Run()
	receipt.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.ElapsedSeconds = time.Since(started).Seconds()
	var exitCode = -1
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	var timedOut = ctx.Err() != nil
	receipt.ExitCode = &exitCode
	receipt.TimedOut = &timedOut
	var after, afterClean, afterErr = editorSourceState(root)
	receipt.SourceCleanAfter = afterErr == nil && afterClean && after == before
	var raw, stderr = output.Bytes(), errors.Bytes()
	receipt.RawSHA256 = editorSHA(raw)
	receipt.RawBytes = len(raw)
	receipt.StderrSHA256 = editorSHA(stderr)
	receipt.StderrBytes = len(stderr)
	var parseErr = parseStructuralMigrationOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = structuralMigrationRequiredEvents()
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	var contents = map[string][]byte{"rp33-process.json": append(encoded, '\n'), "rp33-go.jsonl": raw, "rp33-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range structuralMigrationArtifacts {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual RP33 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed the complete structural migration scenario, actual Legacy/V2 native comparisons, source-directory controls, exact question-target and payload corruptions", true, nil
}
func (r *Runner) runStructuralMigrationRequirement(spec RepairCaseSpec) bool {
	id, title, levels := spec.ID, spec.Title, spec.RequiredTestLevels
	if id != "RP33" {
		return false
	}
	if title != "Syntax-aware migration" || spec.Scenario != structuralMigrationScenario || spec.ExpectedObservation != structuralMigrationExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), structuralMigrationTiers) {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "RP33 producer does not match the immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeStructuralMigrationProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, "repair", title, strings.Join(levels, "+"), structuralMigrationTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = structuralMigrationProducer(record.Tier)
		record.Command = productionTestCommand(structuralMigrationTests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(structuralMigrationTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(structuralMigrationRequiredEvents(), "\n")
		record.Assertions["scope"] = structuralMigrationScope
	}
	return true
}

// RP33 has real nested cases (full program/question target and native payload
// controls). Bind every selected ancestor, including intermediate selected
// cases; an early nested parent pass cannot cover later child observations.
func structuralMigrationNestedEvents(raw []byte) error {
	required, run, passed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range structuralMigrationRequiredEvents() {
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
			parent := event.Test
			for {
				slash := strings.LastIndexByte(parent, '/')
				if slash < 0 {
					break
				}
				parent = parent[:slash]
				if required[parent] && (!run[parent] || passed[parent]) {
					return fmt.Errorf("RP33 nested child outside selected ancestor %s", parent)
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range required {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("RP33 nested parent passed before child %s", name)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
