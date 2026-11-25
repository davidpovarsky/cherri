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

const publicFrontendTitle = "No accidental second user frontend"
const publicFrontendScenario = "Inspect production entrypoints and compile actual old user source, while running isolated trusted old declaration/migration reference paths."
const publicFrontendExpected = "Only approved new public language is accepted; preserved upstream inputs remain build/reference resources, not hidden user execution fallback."
const publicFrontendScope = "Independent RP52 execution of actual default and explicit V2 CLI file/compile/check and mobile routes against old source, anchored Legacy markers and directives, plus healthy recovery and explicit Legacy/compatibility controls. Repository inspection binds all six C compile export signatures and exact mode/profile forwarding, and the Swift public default and complete private compile bridge body; lexical inspection is not iOS ABI execution. The same fresh selection verifies immutable upstream declaration bytes and reviewed corrections, rejects Git replacement-object evidence, and executes the actual isolated Legacy metadata/reference/include program through migration and canonical V2 compilation with native equivalence, rejection preservation and recovery. Explicitly selected Legacy remains authorized under original U04. Only RP52 receives integration/repository records; U01/U04/CL03/RP33/BRG19 and Apple runtime are not inherited."

var publicFrontendTests = []string{
	"TestP15RP52DefaultMobileFrontend",
	"TestP15RP52ExplicitLegacyReferenceModes",
	"TestP15RP52EntrypointSourceBoundaries",
	"TestP15U04PublicFrontendRouteMatrix",
	"TestP15CLIDefaultRejectsLegacyWithGuidance",
	"TestP15U01ActualDeclarationBoundary",
	"TestP15U01DeclarationBoundaryControls",
	"TestP15U01GitReplacementIsolation",
	"TestBackendRecovery19StructuralMigrationIntegration",
}
var publicFrontendTiers = []string{"integration", "repository"}
var publicFrontendArtifacts = []string{"rp52-process.json", "rp52-go.jsonl", "rp52-stderr.log"}

// Immutable Git bytes bind the public routes, source audit, trusted migration and intake.
// The fresh U01 scenario additionally binds all preserved declaration blobs to its
// independently reviewed historical manifest; it does not mint U01 evidence here.
var publicFrontendProductionPaths = []string{
	"main.go",
	"args.go",
	"language_cli_adapter.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"language_emit_adapter.go",
	"language_value_adapter.go",
	"mobile_compiler.go",
	"ios_bridge.go",
	"ios/CherriApp/Compiler/CherriCompiler.swift",
	"ios/CherriTests/CherriCoreIntegrationTests.swift",
	"shortcut.go",
	"shortcutgen.go",
	"output.go",
	"legacy_output_transaction.go",
	"hybrid_import.go",
	"import_fidelity.go",
	"import_provenance.go",
	"ios/CherriApp/App/WorkspaceView.swift",
	"internal/shortcutcompare/compare.go",
	"internal/shortcutcompare/normalize.go",
	"internal/shortcutcompare/edit.go",
	"internal/language/backend/contracts.go",
	"internal/language/lower/lower.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/syntax/parser.go",
	"internal/language/syntax/lexer.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/migrate/migrate.go",
	"internal/language/migrate/lexical_source.go",
	"internal/language/migrate/references.go",
	"internal/language/migrate/optional_bindings.go",
	"internal/language/migrate/native_lists.go",
	"internal/language/migrate/setter_mode.go",
	"internal/language/migrate/variadic.go",
	"p15_public_frontend_boundary_test.go",
	"p15_frontend_entrypoint_audit_test.go",
	"p15_public_parser_route_test.go",
	"p15_binding_scope_wire_test.go",
	"p15_cli_acceptance_test.go",
	"p15_upstream_boundary_test.go",
	"p15_brg19_safe_import_edit_test.go",
	"docs/language-v2/upstream-preservation-boundary.json",
	"go.mod",
	"go.sum",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/public_frontend_execution.go",
	"tools/language-acceptance/upstream_boundary_execution.go",
	"tools/language-acceptance/public_frontend_execution_test.go",
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
	"internal/language/service/module_analysis.go",
	"internal/language/syntax/main_output.go",
	"tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/local_gate_source_binding_test.go",
	"p15_main_void_return_test.go",
}

func publicFrontendRequiredEvents() []string {
	names := append([]string{}, publicFrontendTests...)
	add := func(index int, children []string) {
		for _, child := range children {
			names = append(names, publicFrontendTests[index]+"/"+child)
		}
	}
	for _, route := range []string{"omitted", "empty", "whitespace", "explicit_v2"} {
		add(0, []string{route + "_valid_v2", route + "_reject_plain", route + "_reject_file_marker", route + "_reject_preprocessor"})
	}
	add(1, []string{"explicit_legacy_plain", "explicit_legacy_file_marker", "explicit_legacy_preprocessor", "explicit_auto_file_marker", "explicit_auto_preprocessor", "invalid_mode", "recovery"})
	add(2, []string{"actual_go_exports", "actual_swift_default", "go_unsigned_default_auto", "go_signed_default_auto", "go_explicit_mode_dropped", "go_profile_mode_dropped", "go_default_extra_statement", "go_unknown_export", "go_missing_export", "go_swapped_parameters", "go_wrong_parameter_type", "go_wrong_return_type", "swift_auto_default", "swift_legacy_default", "swift_mode_literal", "swift_comment_shadow", "swift_string_shadow", "swift_duplicate_entry", "swift_duplicate_enum", "swift_missing_bridge", "swift_local_shadow", "swift_unterminated_comment", "swift_bridge_mode_literal", "swift_bridge_mode_pointer", "swift_bridge_swapped_abi", "swift_duplicate_bridge", "swift_inactive_enum_live_struct", "swift_nested_enum_live_struct", "nested_comment", "opaque_interpolation", "opaque_raw_string"})
	add(2, []string{"swift_request_mode_dropped", "swift_request_source_dropped", "swift_document_branch_dropped", "swift_document_payload_mode_dropped", "swift_document_payload_profile_dropped", "swift_document_payload_sign_dropped", "swift_document_payload_context_dropped", "swift_document_json_bridge_bypassed", "swift_response_document_relabel", "swift_response_map_context_dropped", "swift_response_source_request_relabel", "swift_response_mode_validation_dropped", "swift_response_ok_validation_dropped", "swift_response_memory_release_dropped", "swift_request_cancellation_check_dropped", "swift_request_security_scope_release_dropped", "swift_payload_field_shape_changed", "swift_duplicate_request_entry", "swift_duplicate_document_bridge", "swift_duplicate_response_decoder", "swift_duplicate_payload_struct", "swift_unknown_compile_overload", "go_document_request_dropped", "go_document_core_bypassed", "go_document_wrong_type", "go_document_missing_export", "go_document_extra_dispatch"})
	for _, route := range []string{"file", "compile"} {
		for _, choice := range []string{"default_v2", "explicit_v2", "default_rejects_legacy", "explicit_v2_rejects_legacy", "legacy_flag", "legacy_mode", "invalid_mode", "conflicting_modes"} {
			add(3, []string{route + "/" + choice})
		}
	}
	// Both consumers select the same reviewed U01 children; root tests remain independent.
	add(6, upstreamBoundaryControls)
	add(7, upstreamBoundaryReplacementControls)
	add(8, []string{"metadata", "include", "reference"})
	return names
}
func publicFrontendTestIDs() []string {
	var ids []string
	for _, name := range publicFrontendTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func publicFrontendProducer(tier string) string {
	switch tier {
	case "integration":
		return publicFrontendTestIDs()[0]
	case "repository":
		return publicFrontendTestIDs()[2]
	}
	return ""
}
func parsePublicFrontendOutput(raw []byte) error {
	names := publicFrontendRequiredEvents()
	if err := parseStrictNamedProductionOutput(names, productionTestTimeout(publicFrontendTests)+time.Minute, raw); err != nil {
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
					return fmt.Errorf("RP52 child outside selected route parent")
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("RP52 route passed before required child")
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validatePublicFrontendRecord(id string, record EvidenceRecord) error {
	if id != "RP52" {
		return nil
	}
	producer := publicFrontendProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(publicFrontendTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(publicFrontendTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(publicFrontendRequiredEvents(), "\n") ||
		record.Assertions["scope"] != publicFrontendScope {
		return fmt.Errorf("RP52 record detached from complete actual frontend and trusted reference scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, publicFrontendArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[publicFrontendArtifacts[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[publicFrontendArtifacts[1]], data[publicFrontendArtifacts[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, publicFrontendRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, publicFrontendRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete RP52 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(publicFrontendTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = localGateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, publicFrontendProductionPaths); err != nil {
		return err
	}
	if err = parsePublicFrontendOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executePublicFrontendProduction() ([]EvidenceArtifact, string, bool, error) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("RP52 requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, "rp52-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: "RP52", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(publicFrontendTests), RequiredTests: publicFrontendRequiredEvents()}
	receipt.SourceFiles, err = localGateSourceFiles(root, before, publicFrontendProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(publicFrontendTests)+time.Minute)
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
	parseErr := parsePublicFrontendOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = publicFrontendRequiredEvents()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{publicFrontendArtifacts[0]: append(encoded, '\n'), publicFrontendArtifacts[1]: raw, publicFrontendArtifacts[2]: stderr}
	var artifacts []EvidenceArtifact
	for _, name := range publicFrontendArtifacts {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual RP52 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed complete default-V2 public routes and repository source boundaries with actual preserved declarations and isolated Legacy migration references", true, nil
}
func (r *Runner) runPublicFrontendRequirement(spec RepairCaseSpec) bool {
	if spec.ID != "RP52" {
		return false
	}
	level := strings.Join(spec.RequiredTestLevels, "+")
	if spec.Title != publicFrontendTitle || spec.Scenario != publicFrontendScenario || spec.ExpectedObservation != publicFrontendExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), publicFrontendTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "RP52 producer does not match the immutable full scenario"}
		return true
	}
	artifacts, detail, executed, err := r.executePublicFrontendProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(spec.ID, "repair", spec.Title, level, publicFrontendTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = publicFrontendProducer(record.Tier)
		record.Command = productionTestCommand(publicFrontendTests)
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(publicFrontendTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(publicFrontendRequiredEvents(), "\n")
		record.Assertions["scope"] = publicFrontendScope
	}
	return true
}
