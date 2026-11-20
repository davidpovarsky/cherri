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

const nativePathsTitle = "Native action/block/full workflow paths"
const nativePathsScenario = "Import unknown nested/top-level keys, descriptors, variants, references, binary/date/large integer data across supported input formats."
const nativePathsExpected = "Exact tree and reference graph survive; full-native bypass actually works; typed readability retained for supported cases."
const nativePathsScope = "Independent BRG18 actual public V2 import and compile of a complete thirteen-action native workflow across XML plist, binary plist, unsigned Shortcut and JSON, each in hybrid and lossless mode. Exact typed tree, unknown header/action/descriptor fields, static and unmodeled variants, ordered transformations, output references, opaque balanced region and outward result, integer precision, data/date, omission and explicit empty/false/zero values are preserved. JSON strings remain strings; actual data/date categories are exercised in plist inputs. A single explicit native.workflow call independently proves full-native bypass. Complete native-tree and parsed-readability corruption controls are mandatory, including six actual compiled exact-native substitutions for each supported readable call. This does not execute Apple actions, validate unknown action runtime behavior, or grant R01/R02/R04/R05/R06/RP35/RP36 or other related-ID credit."

var nativePathsTests = []string{
	"TestBackendRecovery18PublicNativeImportPaths",
	"TestBackendRecovery18ExactWorkflowBypass",
	"TestBackendRecovery18RejectsNativeAndReadabilityFaults",
	"TestBackendRecovery18ReadabilityRejectsCompiledFallbacks",
}
var nativePathsTiers = []string{"native-roundtrip", "integration"}
var nativePathsArtifacts = []string{"brg18-process.json", "brg18-go.jsonl", "brg18-stderr.log"}

// Immutable Git bytes bind the public entrypoints, independent native oracle and intake.
var nativePathsProductionPaths = []string{
	"main.go",
	"args.go",
	"import.go",
	"import_fidelity.go",
	"import_provenance.go",
	"import_provenance_restore.go",
	"import_provenance_oracle_nopatch.go",
	"hybrid_import.go",
	"semantic_import_planner.go",
	"semantic_import_native_operands.go",
	"semantic_import_native_loop_results.go",
	"language_cli_adapter.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"language_emit_adapter.go",
	"language_value_adapter.go",
	"mobile_compiler.go",
	"shared_codec.go",
	"shortcutgen.go",
	"shortcut.go",
	"raw_actions.go",
	"native_action_envelope.go",
	"native_action_result_binding.go",
	"internal/language/backend/contracts.go",
	"internal/language/backend/native_envelope.go",
	"internal/language/lower/lower.go",
	"internal/language/lower/native_action_envelope.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/syntax/parser.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/schema/schema.go",
	"internal/shortcutcompare/compare.go",
	"internal/shortcutcompare/edit.go",
	"internal/shortcutcompare/normalize.go",
	"internal/shortcutcompare/strict_scalars.go",
	"go.mod",
	"go.sum",
	"p15_native_paths_gate_test.go",
	"p15_native_acceptance_test.go",
	"p15_brg19_safe_import_edit_test.go",
	"p15_cli_acceptance_test.go",
	"p10r9_hybrid_test.go",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/native_paths_execution.go",
	"tools/language-acceptance/native_paths_execution_test.go",
	"tools/language-acceptance/structural_migration_execution_test.go",
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
	"internal/language/analysis/diagnostics.go",
	"internal/language/lower/main_output.go",
	"internal/language/lower/module_scope.go",
	"internal/language/migrate/main_output.go",
	"internal/language/migrate/migrate.go",
	"internal/language/service/module_analysis.go",
	"internal/language/syntax/main_output.go",
	"tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/local_gate_source_binding_test.go",
}

func nativePathsRequiredEvents() []string {
	var names = append([]string{}, nativePathsTests...)
	for _, format := range []string{"xml", "binary", "shortcut", "json"} {
		var parent = nativePathsTests[0] + "/" + format
		names = append(names, parent, parent+"/hybrid", parent+"/lossless")
	}
	names = append(names, nativePathsTests[1]+"/xml", nativePathsTests[1]+"/binary")
	for _, fault := range []string{"header", "action_field", "data_type", "data_byte", "date_type", "date_value", "large_integer", "negative_integer", "false_type", "zero_type", "empty_text", "empty_array", "empty_map", "order", "descriptor", "variant", "producer_uuid", "consumer_uuid", "chain_order", "region_group", "region_result", "omitted_default", "explicit_false", "missing_parameters", "question_index", "provenance", "plan_natural", "plan_opaque", "source_call", "encoded_comment_only"} {
		names = append(names, nativePathsTests[2]+"/"+fault)
	}
	for _, name := range []string{"comment", "static_variant", "reference", "omitted_parameter", "explicit_false", "empty_parameters"} {
		names = append(names, nativePathsTests[3]+"/"+name)
	}
	return names
}
func nativePathsTestIDs() []string {
	var ids []string
	for _, name := range nativePathsTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func nativePathsProducer(tier string) string {
	if tier == "integration" {
		return nativePathsTestIDs()[0]
	}
	if tier == "native-roundtrip" {
		return nativePathsTestIDs()[1]
	}
	return ""
}
func parseNativePathsOutput(raw []byte) error {
	names := nativePathsRequiredEvents()
	if err := parseStrictNamedProductionOutput(names, productionTestTimeout(nativePathsTests)+time.Minute, raw); err != nil {
		return err
	}
	// Route parents own fault children. A selected intermediate parent cannot
	// complete early even if the outer Go test remains active.
	selected, run, passed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range names {
		selected[name] = true
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
				if selected[parent] && (!run[parent] || passed[parent]) {
					return fmt.Errorf("BRG18 child outside selected route parent")
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("BRG18 route passed before required child")
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validateNativePathsRecord(id string, record EvidenceRecord) error {
	if id != "BRG18" {
		return nil
	}
	producer := nativePathsProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(nativePathsTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(nativePathsTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(nativePathsRequiredEvents(), "\n") ||
		record.Assertions["scope"] != nativePathsScope {
		return fmt.Errorf("BRG18 record detached from complete public native import/bypass scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, nativePathsArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[nativePathsArtifacts[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[nativePathsArtifacts[1]], data[nativePathsArtifacts[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, nativePathsRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, nativePathsRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete BRG18 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(nativePathsTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = localGateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, nativePathsProductionPaths); err != nil {
		return err
	}
	if err = parseNativePathsOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeNativePathsProduction() ([]EvidenceArtifact, string, bool, error) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("BRG18 requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, "brg18-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: "BRG18", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(nativePathsTests), RequiredTests: nativePathsRequiredEvents()}
	receipt.SourceFiles, err = localGateSourceFiles(root, before, nativePathsProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(nativePathsTests)+time.Minute)
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
	parseErr := parseNativePathsOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = nativePathsRequiredEvents()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{nativePathsArtifacts[0]: append(encoded, '\n'), nativePathsArtifacts[1]: raw, nativePathsArtifacts[2]: stderr}
	var artifacts []EvidenceArtifact
	for _, name := range nativePathsArtifacts {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual BRG18 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed the full public native import/compile cross-product, full-workflow bypass and independent complete-tree/readability corruption controls", true, nil
}
func (r *Runner) runNativePathsRequirement(spec RecoveryGate) bool {
	if spec.ID != "BRG18" {
		return false
	}
	level := strings.Join(spec.MinimumEvidenceTiers, "+")
	if spec.Title != nativePathsTitle || spec.Scenario != nativePathsScenario || spec.ExpectedObservation != nativePathsExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), nativePathsTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "BRG18 producer does not match the immutable full scenario"}
		return true
	}
	artifacts, detail, executed, err := r.executeNativePathsProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(spec.ID, "recovery_gate", spec.Title, level, nativePathsTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = nativePathsProducer(record.Tier)
		record.Command = productionTestCommand(nativePathsTests)
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(nativePathsTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(nativePathsRequiredEvents(), "\n")
		record.Assertions["scope"] = nativePathsScope
	}
	return true
}
