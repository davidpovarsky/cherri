package main

import (
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

const numberWireScope = "Independent RP11 execution: semantic Number width, exact literal text, mutable variables, action outputs and contextual Unicode tokenized text traverse real analysis and canonical emission across captured Apple NumberString, Number and VariableString parameter states. Eleven positive native comparisons and 58 causal wire/reference/producer/range corruptions run; linked T03/T09/R03, Apple runtime and full179 closure are not inherited."

var numberWireTests = []string{"TestP15RP11SemanticNumberAndContext", "TestP15RP11NativeParameterDifferential", "TestP15RP11NativeOracleRejectsFaults"}
var numberWireTiers = []string{"unit", "native-differential"}
var numberWireArtifacts = []string{"rp11-process.json", "rp11-go.jsonl", "rp11-stderr.log"}
var numberWireCases = []string{"semantic_width", "bound_semantic_width", "literal_width_text", "mutable_width", "output_width", "literal_stepper", "mutable_stepper", "output_stepper", "mutable_text", "output_text", "contextual_text"}

func numberWireFaults(name string) []string {
	faults := []string{"wrong_action", "missing_parameter", "blanket_float", "blanket_attachment"}
	switch name {
	case "semantic_width", "literal_width_text", "literal_stepper":
	default:
		faults = append(faults, "wrong_reference")
	}
	switch name {
	case "bound_semantic_width", "output_width", "output_stepper", "output_text":
		faults = append(faults, "producer_literal")
	}
	if name == "contextual_text" {
		faults = append(faults, "utf8_range", "lost_context")
	}
	return faults
}

var numberWireProductionPaths = []string{
	"internal/language/syntax/ast.go",
	"internal/language/syntax/parser.go",
	"internal/language/syntax/lexer.go",
	"internal/language/source/span.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/types/types.go",
	"internal/language/lower/lower.go",
	"internal/language/lower/binding_capture.go",
	"internal/language/lower/exhaustive_validation.go",
	"internal/language/backend/contracts.go",
	"internal/language/backend/parameter_codecs.go",
	"internal/language/schema/schema.go",
	"internal/language/schema/generated_registry.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"language_emit_adapter.go",
	"language_value_adapter.go",
	"shared_codec.go",
	"shortcutgen.go",
	"action.go",
	"import_fidelity.go",
	"debug_stdlib_test.go",
	"p15_cardinality_acceptance_test.go",
	"p15_parser_acceptance_test.go",
	"p15_rp11_parameter_wire_test.go",
	"docs/apple-runtime/macos-27/encoding-table.json",
	"docs/apple-runtime/macos-27/parameter-encodings.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/number_wire_execution.go",
	"tools/language-acceptance/number_wire_execution_test.go",
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
}

func numberWireRequiredEvents() []string {
	var names []string
	for _, parent := range numberWireTests[:2] {
		names = append(names, parent)
		for _, name := range numberWireCases {
			names = append(names, parent+"/"+name)
		}
	}
	names = append(names, numberWireTests[2])
	for _, name := range numberWireCases {
		for _, fault := range numberWireFaults(name) {
			names = append(names, numberWireTests[2]+"/"+name+"/"+fault)
		}
	}
	return names
}
func numberWireTestIDs() []string {
	var ids []string
	for _, name := range numberWireTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func numberWireProducer(tier string) string {
	if tier == "unit" {
		return numberWireTestIDs()[0]
	}
	if tier == "native-differential" {
		return numberWireTestIDs()[1]
	}
	return ""
}
func parseNumberWireOutput(raw []byte) error {
	return parseStrictNamedProductionOutput(numberWireRequiredEvents(), productionTestTimeout(numberWireTests)+time.Minute, raw)
}
func validateNumberWireRecord(id string, record EvidenceRecord) error {
	if id != "RP11" {
		return nil
	}
	var producer = numberWireProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"RP11"}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(numberWireTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(numberWireTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(numberWireRequiredEvents(), "\n") ||
		record.Assertions["scope"] != numberWireScope {
		return fmt.Errorf("RP11 record is detached from complete per-parameter native differential production")
	}
	var data, err = stateReadArtifacts(record.Artifacts, numberWireArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["rp11-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data["rp11-go.jsonl"], data["rp11-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "RP11" || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, numberWireRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, numberWireRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete RP11 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(numberWireTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, numberWireProductionPaths); err != nil {
		return err
	}
	if err = parseNumberWireOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeNumberWireProduction() ([]EvidenceArtifact, string, bool, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("RP11 producer requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	var directory, dirErr = stateEvidenceDirectory(r, "rp11-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: "RP11", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(numberWireTests), RequiredTests: numberWireRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, numberWireProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(numberWireTests)+time.Minute)
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
	var parseErr = parseNumberWireOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = numberWireRequiredEvents()
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	var contents = map[string][]byte{"rp11-process.json": append(encoded, '\n'), "rp11-go.jsonl": raw, "rp11-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range numberWireArtifacts {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual RP11 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed semantic Number checks, eleven independent captured-Apple parameter comparisons and58 causal wire/reference/producer/range controls", true, nil
}
func (r *Runner) runNumberWireRequirement(id, title string, levels []string) bool {
	if id != "RP11" {
		return false
	}
	if title != "Number-as-text and reference forms" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), numberWireTiers) {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "RP11 producer does not match the immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeNumberWireProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, "repair", title, strings.Join(levels, "+"), numberWireTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = numberWireProducer(record.Tier)
		record.Command = productionTestCommand(numberWireTests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(numberWireTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(numberWireRequiredEvents(), "\n")
		record.Assertions["scope"] = numberWireScope
	}
	return true
}
