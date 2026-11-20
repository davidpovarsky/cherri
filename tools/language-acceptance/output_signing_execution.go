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
	"runtime"
	"strings"
	"time"
	"unicode/utf8"
)

const outputSigningTitle = "Output/signing failure behavior"
const outputSigningScenario = "Test output paths/spaces/non-ASCII/permissions, failed encoding/signer, explicit Linux remote-sign request and unsigned check."
const outputSigningExpected = "No swallowed errors or wrong destination; no unsolicited signing; no unsigned bytes claimed signed."
const outputSigningScope = "Independent RP43 actual Linux CLI execution: four public V2/Legacy compile routes use exact paths with spaces and non-ASCII names, preserve preexisting destination/source siblings on filesystem and encoding failures, and recover after real same-process permission denial. Six actual owned loopback signer faults are observed independently on five public routes, with exact destination/input preservation and healthy protocol-only recovery. Unsigned/default V2/compile and explicit Legacy --skip-sign plus check keep process/network/output boundaries; documented bare Legacy signing defaults are unchanged. AEA1_HEADER_PASS is a protocol classification, never an Apple signature/runtime claim. No CL01/CL02/CI04/BRG26 or other linked requirement inherits credit."

var outputSigningTests = []string{
	"TestP15RP43PublicOutputDestinations",
	"TestP15RP43LinuxOutputPermissions",
	"TestP15RP43LegacyEncodingFailurePreservesDestination",
	"TestP15RP43EncodingFailureReturnsNoPartialBytes",
	"TestP15RP43PublicSignerFailureBoundaries",
	"TestP15RP43UnsignedAndAnalysisBoundaries",
}
var outputSigningTiers = []string{"integration", "security"}
var outputSigningArtifacts = []string{"rp43-process.json", "rp43-go.jsonl", "rp43-stderr.log"}

// The exact immutable commit binds the complete program. This additional
// inventory binds the actual public entry points, original tests and intake.
var outputSigningProductionPaths = []string{
	"main.go", "args.go", "output.go", "legacy_output_transaction.go", "signing.go",
	"language_cli_adapter.go", "language_backend_session.go", "language_backend_adapter.go",
	"language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go", "parser.go",
	"internal/language/backend/contracts.go", "internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"internal/language/lower/lower.go", "internal/language/analysis/analyzer.go", "internal/language/syntax/parser.go",
	"internal/language/syntax/lexer.go", "go.mod", "go.sum",
	"p15_output_signing_contract_test.go", "p15_output_signing_security_test.go", "p15_cli_acceptance_test.go",
	"p15_signing_atomic_contract_test.go", "p15_schema_production_test.go",
	"tests/language-v2/repair-regression-plan.json", "tests/language-v2/acceptance-contract.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/output_signing_execution.go", "tools/language-acceptance/output_signing_execution_test.go",
	"tools/language-acceptance/structural_migration_execution_test.go", "tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/loop_execution.go", "tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/editor_performance_execution.go", "tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go", "tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/state_acceptance_intake.go", "tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/parser_execution.go",
	"tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
	".github/workflows/p10r15-cli-output-evidence.yml",
}

type outputSigningProcessReceipt struct {
	editorProcessReceipt
	GOOS string `json:"goos"`
}

func outputSigningRequiredEvents() []string {
	names := append([]string{}, outputSigningTests...)
	add := func(parent string, children ...string) {
		for _, child := range children {
			names = append(names, parent+"/"+child)
		}
	}
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy"} {
		add(outputSigningTests[0], route)
		add(outputSigningTests[0]+"/"+route, "absolute_unicode", "relative_unicode", "destination_directory", "parent_is_file", "missing_parent")
		add(outputSigningTests[1], route)
	}
	add(outputSigningTests[3], "xml", "binary")
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy", "sign"} {
		add(outputSigningTests[4], route)
		for fault := 0; fault < 6; fault++ {
			add(outputSigningTests[4]+"/"+route, fmt.Sprintf("fault_%d", fault))
		}
	}
	add(outputSigningTests[5], "implicit_unsigned/compile_v2", "implicit_unsigned/file_v2", "implicit_unsigned/compile_legacy", "explicit_unsigned/file_legacy", "check_valid_true", "check_valid_false", "explicit_remote_request_control")
	return names
}
func outputSigningTestIDs() []string {
	var ids []string
	for _, name := range outputSigningTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func outputSigningProducer(tier string) string {
	if tier == "integration" {
		return outputSigningTestIDs()[0]
	}
	if tier == "security" {
		return outputSigningTestIDs()[4]
	}
	return ""
}
func parseOutputSigningOutput(raw []byte) error {
	names := outputSigningRequiredEvents()
	if err := parseStrictNamedProductionOutput(names, productionTestTimeout(outputSigningTests)+time.Minute, raw); err != nil {
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
					return fmt.Errorf("RP43 child outside selected route parent")
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("RP43 route passed before required child")
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validateOutputSigningRecord(id string, record EvidenceRecord) error {
	if id != "RP43" {
		return nil
	}
	producer := outputSigningProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(outputSigningTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(outputSigningTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(outputSigningRequiredEvents(), "\n") ||
		record.Assertions["scope"] != outputSigningScope || record.Assertions["required_host_os"] != "linux" {
		return fmt.Errorf("RP43 record detached from complete Linux output/signing scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, outputSigningArtifacts)
	if err != nil {
		return err
	}
	var receipt outputSigningProcessReceipt
	if err = decodeEditorJSON(data[outputSigningArtifacts[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[outputSigningArtifacts[1]], data[outputSigningArtifacts[2]]
	if receipt.GOOS != "linux" || receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, outputSigningRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, outputSigningRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete RP43 Linux process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(outputSigningTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, outputSigningProductionPaths); err != nil {
		return err
	}
	if err = parseOutputSigningOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt.editorProcessReceipt)
}
func (r *Runner) executeOutputSigningProduction() ([]EvidenceArtifact, string, bool, error) {
	if runtime.GOOS != "linux" {
		return nil, "", false, fmt.Errorf("RP43 requires actual Linux permission and explicit remote-sign execution; current host is %s", runtime.GOOS)
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("RP43 requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, "rp43-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := outputSigningProcessReceipt{editorProcessReceipt: editorProcessReceipt{FormatVersion: 1, Requirement: "RP43", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(outputSigningTests), RequiredTests: outputSigningRequiredEvents()}, GOOS: runtime.GOOS}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, outputSigningProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(outputSigningTests)+time.Minute)
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
	parseErr := parseOutputSigningOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt.editorProcessReceipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = outputSigningRequiredEvents()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{outputSigningArtifacts[0]: append(encoded, '\n'), outputSigningArtifacts[1]: raw, outputSigningArtifacts[2]: stderr}
	var artifacts []EvidenceArtifact
	for _, name := range outputSigningArtifacts {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual RP43 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed full Linux public-output, actual encoding/permission faults, explicit owned signer and unsigned/check boundaries", true, nil
}
func (r *Runner) runOutputSigningRequirement(spec RepairCaseSpec) bool {
	if spec.ID != "RP43" {
		return false
	}
	level := strings.Join(spec.RequiredTestLevels, "+")
	if spec.Title != outputSigningTitle || spec.Scenario != outputSigningScenario || spec.ExpectedObservation != outputSigningExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), outputSigningTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "RP43 producer does not match the immutable full scenario"}
		return true
	}
	artifacts, detail, executed, err := r.executeOutputSigningProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(spec.ID, "repair", spec.Title, level, outputSigningTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = outputSigningProducer(record.Tier)
		record.Command = productionTestCommand(outputSigningTests)
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(outputSigningTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(outputSigningRequiredEvents(), "\n")
		record.Assertions["scope"] = outputSigningScope
		record.Assertions["required_host_os"] = "linux"
	}
	return true
}
