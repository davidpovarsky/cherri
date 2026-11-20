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

const purePublicTitle = "Authorized resource access and pure services"
const purePublicScenario = "Attempt symlink/traversal/out-of-root access, unresolved remote imports, network/process calls during analysis and invalid compile."
const purePublicExpected = "Actual loader and public APIs reject unauthorized operations and never sign/execute/install during analysis."
const purePublicScope = "Independent BRG21 execution of actual module loader and resource authorization through current service and canonical compile, including real outside bytes, traversal, file/directory symlink escapes and cached-path replacement. Complete public service/library/check/compile/file scenarios reject unresolved remote/local imports and invalid actions without network requests, signing/open/install process invocations or workspace artifacts; valid analysis and recovery remain pure. Actual loopback HTTP and executable observer controls prove the monitored production boundaries are live, and explicitly requested signing reaches the same observer after the pure-path checks. Every required symlink case must execute. This proves the named current public entrypoints and production effect paths only; it does not execute Apple actions, claim arbitrary external sandboxing or inherit IM04/IM05/CL02/RP29 or other IDs."

var purePublicTests = []string{
	"TestBackendRecovery21PublicAndServiceEffects",
	"TestP15RP29AssetAuthorizationAcrossServiceAndCompile",
	"TestP15RP29CachedResourceIsReauthorizedAfterSymlinkReplacement",
	"TestP15IM04ActualWorkspaceResourceBoundaries",
}
var purePublicTiers = []string{"security", "integration", "service"}
var purePublicArtifacts = []string{"brg21-process.json", "brg21-go.jsonl", "brg21-stderr.log"}

// Immutable Git bytes bind the public entrypoints, actual effect observers and intake.
var purePublicProductionPaths = []string{
	"main.go",
	"args.go",
	"language_cli_adapter.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"language_emit_adapter.go",
	"language_value_adapter.go",
	"mobile_compiler.go",
	"signing.go",
	"output.go",
	"legacy_output_transaction.go",
	"shortcutgen.go",
	"shortcut.go",
	"internal/language/backend/contracts.go",
	"internal/language/lower/lower.go",
	"embedded_assets.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/syntax/parser.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/schema/schema.go",
	"internal/language/modules/graph.go",
	"internal/language/modules/standard_library.go",
	"internal/language/modules/stdfunc_source_generated.go",
	"internal/language/workspace/resource.go",
	"internal/language/assets/cache.go",
	"internal/language/service/service.go",
	"internal/language/service/module_analysis.go",
	"internal/language/service/asset_analysis.go",
	"internal/language/service/registry_lifetime.go",
	"go.mod",
	"go.sum",
	"p15_pure_public_gate_test.go",
	"p15_workspace_module_test.go",
	"p15_asset_authorization_repair_test.go",
	"p15_embedded_asset_acceptance_test.go",
	"p15_asset_dependency_baseline_test.go",
	"p15_cli_acceptance_test.go",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/pure_public_execution.go",
	"tools/language-acceptance/pure_public_execution_test.go",
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
}

func purePublicRequiredEvents() []string {
	names := append([]string{}, purePublicTests...)
	for _, child := range []string{"observer_controls", "service_valid", "check_valid", "service_invalid_action", "check_invalid_action", "compile_invalid_action", "file_invalid_action", "service_remote_import", "check_remote_import", "compile_remote_import", "file_remote_import", "service_missing_local_import", "check_missing_local_import", "compile_missing_local_import", "file_missing_local_import", "valid_recovery_and_requested_signer_control"} {
		names = append(names, purePublicTests[0]+"/"+child)
	}
	for _, child := range []string{"allowed_text", "allowed_binary", "missing", "existing_parent_escape", "file_symlink_escape", "directory_symlink_escape"} {
		names = append(names, purePublicTests[1]+"/"+child)
	}
	for _, child := range []string{"allowed_._local_helper.cherri", "allowed_._local_helper", "allowed_._..safe_helper.cherri", "reject_._escape_helper.cherri", "reject_._escape_helper", "reject_.._workspace-other_helper.cherri", "missing_module"} {
		names = append(names, purePublicTests[3]+"/"+child)
	}
	return names
}
func purePublicTestIDs() []string {
	var ids []string
	for _, name := range purePublicTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func purePublicProducer(tier string) string {
	switch tier {
	case "security":
		return purePublicTestIDs()[1]
	case "integration":
		return purePublicTestIDs()[3]
	case "service":
		return purePublicTestIDs()[0]
	}
	return ""
}
func parsePurePublicOutput(raw []byte) error {
	names := purePublicRequiredEvents()
	if err := parseStrictNamedProductionOutput(names, productionTestTimeout(purePublicTests)+time.Minute, raw); err != nil {
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
					return fmt.Errorf("BRG21 child outside selected route parent")
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("BRG21 route passed before required child")
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validatePurePublicRecord(id string, record EvidenceRecord) error {
	if id != "BRG21" {
		return nil
	}
	producer := purePublicProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(purePublicTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(purePublicTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(purePublicRequiredEvents(), "\n") ||
		record.Assertions["scope"] != purePublicScope {
		return fmt.Errorf("BRG21 record detached from complete actual authorization and pure-public scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, purePublicArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[purePublicArtifacts[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[purePublicArtifacts[1]], data[purePublicArtifacts[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, purePublicRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, purePublicRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete BRG21 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(purePublicTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, purePublicProductionPaths); err != nil {
		return err
	}
	if err = parsePurePublicOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executePurePublicProduction() ([]EvidenceArtifact, string, bool, error) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("BRG21 requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, "brg21-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: "BRG21", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(purePublicTests), RequiredTests: purePublicRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, purePublicProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(purePublicTests)+time.Minute)
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
	parseErr := parsePurePublicOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = purePublicRequiredEvents()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{purePublicArtifacts[0]: append(encoded, '\n'), purePublicArtifacts[1]: raw, purePublicArtifacts[2]: stderr}
	var artifacts []EvidenceArtifact
	for _, name := range purePublicArtifacts {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual BRG21 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed complete authorized module/resource loaders and pure public service/library/check/compile/file scenarios with live effect-observer controls", true, nil
}
func (r *Runner) runPurePublicRequirement(spec RecoveryGate) bool {
	if spec.ID != "BRG21" {
		return false
	}
	level := strings.Join(spec.MinimumEvidenceTiers, "+")
	if spec.Title != purePublicTitle || spec.Scenario != purePublicScenario || spec.ExpectedObservation != purePublicExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), purePublicTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "BRG21 producer does not match the immutable full scenario"}
		return true
	}
	artifacts, detail, executed, err := r.executePurePublicProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(spec.ID, "recovery_gate", spec.Title, level, purePublicTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = purePublicProducer(record.Tier)
		record.Command = productionTestCommand(purePublicTests)
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(purePublicTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(purePublicRequiredEvents(), "\n")
		record.Assertions["scope"] = purePublicScope
	}
	return true
}
