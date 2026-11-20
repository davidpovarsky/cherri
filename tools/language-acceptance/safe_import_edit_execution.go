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

const safeImportEditScope = "Independent BRG19 execution of Legacy metadata/reference/include migration, unsupported preview/write rejection with original byte preservation, default-V2 rejection of Legacy, public hybrid import, exact unedited XML/binary native roundtrips and one SHA-bound authorized scalar edit. Parser-span editing, complete typed/identity preservation and native/policy corruption controls are mandatory. No R07,CL03,RP33,RP34,RP37 or other related acceptance credit is inherited."
const safeImportEditScenario = "Migrate legacy metadata/references/includes, fail an unsupported conversion, then perform one authorized parameter edit on imported data."
const safeImportEditExpected = "Originals survive failed conversion; intended delta only; no regex directive deletion; public output is v2, not legacy retry."

var safeImportEditTests = []string{
	"TestBackendRecovery19StructuralMigrationIntegration",
	"TestBackendRecovery19AuthorizedNativeRoundTrip",
}
var safeImportEditTiers = []string{"integration", "native-roundtrip"}
var safeImportEditArtifacts = []string{"brg19-process.json", "brg19-go.jsonl", "brg19-stderr.log"}
var safeImportEditSyntaxControls = []string{"metadata", "include", "reference"}
var safeImportEditNativeControls = []string{"metadata_loss", "opaque_data_type", "opaque_large_integer", "opaque_date", "opaque_false_zero", "opaque_identity", "target_before", "target_type", "action_identifier", "sibling_parameter", "action_order", "action_removed", "producer_reference_rename", "raw_folder", "provenance_leak"}
var safeImportEditPolicyControls = []string{"missing", "wrong_original", "wrong_before", "wrong_target", "wrong_after", "duplicate_path", "wrong_type", "empty_changes"}
var safeImportEditProductionPaths = []string{
	"internal/language/migrate/migrate.go", "internal/language/migrate/lexical_source.go", "internal/language/migrate/references.go",
	"internal/language/migrate/optional_bindings.go", "internal/language/migrate/variadic.go", "internal/language/migrate/native_lists.go",
	"internal/language/syntax/ast.go", "internal/language/syntax/parser.go", "internal/language/syntax/lexer.go", "internal/language/syntax/format.go", "internal/language/source/span.go",
	"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/exhaustive_validation.go",
	"internal/language/backend/contracts.go", "internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"language_cli_adapter.go", "language_backend_session.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go",
	"parser.go", "includes.go", "raw_actions.go", "action.go", "actions_std.go", "actions/basic.cherri", "actions/documents.cherri",
	"import.go", "import_fidelity.go", "import_provenance.go", "import_provenance_restore.go", "import_provenance_oracle_nopatch.go", "hybrid_import.go",
	"semantic_import_planner.go", "semantic_import_native_operands.go", "semantic_import_native_loop_results.go", "native_action_envelope.go", "native_action_result_binding.go",
	"internal/shortcutcompare/compare.go", "internal/shortcutcompare/normalize.go", "internal/shortcutcompare/question_targets.go", "internal/shortcutcompare/edit.go",
	"internal/shortcutcompare/strict_scalars.go", "internal/shortcutcompare/p15_snapshot_symbols.go",
	"p15_brg19_safe_import_edit_test.go", "p15_cli_acceptance_test.go",
	"tests/language-v2/repair-regression-plan.json", "tests/language-v2/acceptance-contract.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/safe_import_edit_execution.go", "tools/language-acceptance/safe_import_edit_execution_test.go",
	"tools/language-acceptance/strict_go_events.go", "tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go", "tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
}

func safeImportEditRequiredEvents() []string {
	names := append([]string{}, safeImportEditTests...)
	for _, child := range safeImportEditSyntaxControls {
		names = append(names, safeImportEditTests[0]+"/"+child)
	}
	for _, format := range []string{"xml", "binary"} {
		parent := safeImportEditTests[1] + "/" + format
		names = append(names, parent)
		for _, child := range safeImportEditNativeControls {
			names = append(names, parent+"/candidate_"+child)
		}
		for _, child := range safeImportEditPolicyControls {
			names = append(names, parent+"/policy_"+child)
		}
	}
	return names
}

func safeImportEditTestIDs() []string {
	var ids []string
	for _, name := range safeImportEditTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func safeImportEditProducer(tier string) string {
	if tier == "integration" {
		return safeImportEditTestIDs()[0]
	}
	if tier == "native-roundtrip" {
		return safeImportEditTestIDs()[1]
	}
	return ""
}
func parseSafeImportEditOutput(raw []byte) error {
	if err := parseStrictNamedProductionOutput(safeImportEditRequiredEvents(), productionTestTimeout(safeImportEditTests)+time.Minute, raw); err != nil {
		return err
	}
	return safeImportEditNestedEvents(raw)
}
func validateSafeImportEditRecord(id string, record EvidenceRecord) error {
	if id != "BRG19" {
		return nil
	}
	var producer = safeImportEditProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"BRG19"}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(safeImportEditTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(safeImportEditTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(safeImportEditRequiredEvents(), "\n") ||
		record.Assertions["scope"] != safeImportEditScope {
		return fmt.Errorf("BRG19 record is detached from complete safe migration/import/edit production")
	}
	var data, err = stateReadArtifacts(record.Artifacts, safeImportEditArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["brg19-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data["brg19-go.jsonl"], data["brg19-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "BRG19" || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, safeImportEditRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, safeImportEditRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete BRG19 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(safeImportEditTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, safeImportEditProductionPaths); err != nil {
		return err
	}
	if err = parseSafeImportEditOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeSafeImportEditProduction() ([]EvidenceArtifact, string, bool, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("BRG19 producer requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	var directory, dirErr = stateEvidenceDirectory(r, "brg19-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: "BRG19", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(safeImportEditTests), RequiredTests: safeImportEditRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, safeImportEditProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(safeImportEditTests)+time.Minute)
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
	var parseErr = parseSafeImportEditOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = safeImportEditRequiredEvents()
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	var contents = map[string][]byte{"brg19-process.json": append(encoded, '\n'), "brg19-go.jsonl": raw, "brg19-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range safeImportEditArtifacts {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual BRG19 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed the full migration/failure/import/edit scenario with public V2 CLI, exact XML/binary native comparisons, original-file hashes and authorization/collateral corruption controls", true, nil
}
func (r *Runner) runSafeImportEditRequirement(spec RecoveryGate) bool {
	id, title, levels := spec.ID, spec.Title, spec.MinimumEvidenceTiers
	if id != "BRG19" {
		return false
	}
	if title != "Safe import editing and structural migration" || spec.Scenario != safeImportEditScenario || spec.ExpectedObservation != safeImportEditExpected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), safeImportEditTiers) {
		r.results[id] = CaseResult{ID: id, Group: "recovery_gate", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "BRG19 producer does not match the immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeSafeImportEditProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "recovery_gate", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, "recovery_gate", title, strings.Join(levels, "+"), safeImportEditTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = safeImportEditProducer(record.Tier)
		record.Command = productionTestCommand(safeImportEditTests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(safeImportEditTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(safeImportEditRequiredEvents(), "\n")
		record.Assertions["scope"] = safeImportEditScope
	}
	return true
}

// BRG19 has real nested XML/binary input cases with native and policy
// corruption controls. Bind every selected ancestor, including intermediate selected
// cases; an early nested parent pass cannot cover later child observations.
func safeImportEditNestedEvents(raw []byte) error {
	required, run, passed := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, name := range safeImportEditRequiredEvents() {
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
					return fmt.Errorf("BRG19 nested child outside selected ancestor %s", parent)
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range required {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("BRG19 nested parent passed before child %s", name)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
