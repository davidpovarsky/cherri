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

const upstreamBoundaryScope = "Independent U01 repository execution: reconcile all25 Legacy action declarations and preserved stdfunc input against four immutable Git baselines; retain inherited fork differences explicitly; admit only the two independently reviewed parameter-label corrections since the requested continuation base. Twelve causal boundary corruptions are rejected. No B01/BRG01, other schema/std-function row, Apple runtime or final release credit is inherited."

var upstreamBoundaryTests = []string{"TestP15U01ActualDeclarationBoundary", "TestP15U01DeclarationBoundaryControls"}
var upstreamBoundaryTiers = []string{"repository"}
var upstreamBoundaryArtifacts = []string{"u01-process.json", "u01-go.jsonl", "u01-stderr.log"}
var upstreamBoundaryControls = []string{"missing_current_file", "extra_current_file", "missing_manifest_file", "duplicate_manifest_file", "wrong_base", "wrong_baseline_digest", "wrong_baseline_size", "rewritten_unchanged_file", "forged_recorded_hash", "wrong_correction", "false_upstream_equality", "false_unchanged_class"}
var upstreamBoundaryProductionPaths = []string{
	"actions/a11y.cherri",
	"actions/basic.cherri",
	"actions/calendar.cherri",
	"actions/contacts.cherri",
	"actions/crypto.cherri",
	"actions/device.cherri",
	"actions/documents.cherri",
	"actions/dropbox.cherri",
	"actions/images.cherri",
	"actions/intelligence.cherri",
	"actions/location.cherri",
	"actions/mac.cherri",
	"actions/math.cherri",
	"actions/media.cherri",
	"actions/music.cherri",
	"actions/network.cherri",
	"actions/pdf.cherri",
	"actions/photos.cherri",
	"actions/settings.cherri",
	"actions/sharing.cherri",
	"actions/shortcuts.cherri",
	"actions/storage.cherri",
	"actions/text.cherri",
	"actions/translation.cherri",
	"actions/web.cherri",
	"stdfunc.cherri",
	"docs/language-v2/upstream-preservation-boundary.json",
	"docs/language-v2/upstream-boundary.md",
	"p15_upstream_boundary_test.go",
	"actions_catalog.go",
	"tools/language-schema/main.go",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/upstream_boundary_execution.go",
	"tools/language-acceptance/upstream_boundary_execution_test.go",
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

func upstreamBoundaryRequiredEvents() []string {
	names := append([]string{}, upstreamBoundaryTests...)
	for _, name := range upstreamBoundaryControls {
		names = append(names, upstreamBoundaryTests[1]+"/"+name)
	}
	return names
}
func upstreamBoundaryTestIDs() []string {
	var ids []string
	for _, name := range upstreamBoundaryTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func upstreamBoundaryProducer(tier string) string {
	if tier == "repository" {
		return upstreamBoundaryTestIDs()[0]
	}
	return ""
}
func parseUpstreamBoundaryOutput(raw []byte) error {
	return parseStrictNamedProductionOutput(upstreamBoundaryRequiredEvents(), productionTestTimeout(upstreamBoundaryTests)+time.Minute, raw)
}
func validateUpstreamBoundaryRecord(id string, record EvidenceRecord) error {
	if id != "U01" {
		return nil
	}
	var producer = upstreamBoundaryProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"U01"}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(upstreamBoundaryTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(upstreamBoundaryTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(upstreamBoundaryRequiredEvents(), "\n") ||
		record.Assertions["scope"] != upstreamBoundaryScope {
		return fmt.Errorf("U01 record is detached from complete actual repository declaration boundary production")
	}
	var data, err = stateReadArtifacts(record.Artifacts, upstreamBoundaryArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["u01-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data["u01-go.jsonl"], data["u01-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "U01" || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, upstreamBoundaryRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, upstreamBoundaryRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete U01 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(upstreamBoundaryTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, upstreamBoundaryProductionPaths); err != nil {
		return err
	}
	if err = parseUpstreamBoundaryOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeUpstreamBoundaryProduction() ([]EvidenceArtifact, string, bool, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("U01 producer requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	var directory, dirErr = stateEvidenceDirectory(r, "u01-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: "U01", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(upstreamBoundaryTests), RequiredTests: upstreamBoundaryRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, upstreamBoundaryProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(upstreamBoundaryTests)+time.Minute)
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
	var parseErr = parseUpstreamBoundaryOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = upstreamBoundaryRequiredEvents()
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	var contents = map[string][]byte{"u01-process.json": append(encoded, '\n'), "u01-go.jsonl": raw, "u01-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range upstreamBoundaryArtifacts {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual U01 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed actual26-file repository byte boundary and12 independent causal inventory/source/correction controls", true, nil
}
func (r *Runner) runUpstreamBoundaryRequirement(id, title string, levels []string) bool {
	if id != "U01" {
		return false
	}
	if title != "Preserve upstream declaration files when build-time adapters suffice" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), upstreamBoundaryTiers) {
		r.results[id] = CaseResult{ID: id, Group: "upstream", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "U01 producer does not match the immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeUpstreamBoundaryProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "upstream", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, "upstream", title, strings.Join(levels, "+"), upstreamBoundaryTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = upstreamBoundaryProducer(record.Tier)
		record.Command = productionTestCommand(upstreamBoundaryTests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(upstreamBoundaryTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(upstreamBoundaryRequiredEvents(), "\n")
		record.Assertions["scope"] = upstreamBoundaryScope
	}
	return true
}
