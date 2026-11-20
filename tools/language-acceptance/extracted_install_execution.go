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
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

type extractedInstallScenario struct {
	Title, Scenario, Expected, Scope string
	Tests, Tiers                     []string
	Producers                        map[string]string
	Timeout                          time.Duration
}

var extractedInstallScenarios = map[string]extractedInstallScenario{
	"RP42": {
		Title:     "Real public CLI and installed tools",
		Scenario:  "Invoke actual help-documented compile/check/format/import/schema/alias commands and cherri-migrate from extracted install.",
		Expected:  "Commands really exist, share the same pipeline, and have reliable JSON/exit/output behavior.",
		Scope:     "Independent RP42 actual clean-source build, generated reference, committed disposable component tuple, ZIP create/verify/extract and source install. Public help/schema/check/format/compile/import aliases, native equality and installed cherri-migrate preview/check/write are executed. This component tuple is not the published release tuple; no CL01,K01,K03 or other linked requirement inherits credit.",
		Tests:     []string{"TestP15RP42ExtractedInstallPublicCLI", "TestP15RP42MigrationDocumentedFlags"},
		Tiers:     []string{"cli", "skill-package"},
		Producers: map[string]string{"cli": "TestP15RP42ExtractedInstallPublicCLI", "skill-package": "TestP15RP42ExtractedInstallPublicCLI"},
		Timeout:   8 * time.Minute,
	},
	"RP44": {
		Title:     "Skill compatibility rejection",
		Scenario:  "Select outdated bundled and cached compilers, invalid capabilities JSON, incompatible protocol/schema and mismatched docs.",
		Expected:  "Doctor/install fails or selects verified compatible replacement; no existence-only or language-string-only success.",
		Scope:     "Independent RP44 extracted installation with actual current and outdated compiler files, complete same-source companion, cache/bundle/explicit selection, original-byte malformed JSON and protocol/catalog/schema/codec/macro controls, and exact clean documentation provenance. Synthetic response controls must be actually observed before precise rejection; they never count as native output. The shipped offline consumer runs against actual tools and generated docs. No K02,K04 or published release credit is inherited.",
		Tests:     []string{"TestP15RP44ExtractedCapabilitiesRejection", "TestP15RP44InstalledDocsIdentity", "TestP15RP44ExtractedCacheAndBundleSelection", "TestP15RP44GitIndexDocsIntegrity", "TestP15RP44ExtractedCompatibilityConsumer"},
		Tiers:     []string{"skill-package", "integration"},
		Producers: map[string]string{"skill-package": "TestP15RP44ExtractedCapabilitiesRejection", "integration": "TestP15RP44ExtractedCacheAndBundleSelection"},
		Timeout:   12 * time.Minute,
	},
	"RP45": {
		Title:     "Exact install and rollback",
		Scenario:  "Install an exact source/digest, verify migration binary, run offline with compatible cache, corrupt replacement checksum and interrupt update.",
		Expected:  "Resolved immutable revisions match; atomic activation; compatible previous installation survives update failures.",
		Scope:     "Independent RP45 actual immutable source build after source/docs branches advance, both embedded tool roles/source/platforms and mandatory byte receipts, verified runtime ZIP extraction, offline cache, corrupt replacement rejection and real handled interruptions after filesystem operations. Six prior tool/receipt files plus exact docs survive failures; a complete update executes the installed migrator. Controlled download self-test is distinct from the real source-build route. No power-loss or concurrent-reader guarantee, published deployment, K01/K03/K04 or other linked acceptance is inferred.",
		Tests:     []string{"TestP15RP45InstalledBinaryOwnership", "TestP15RP45RuntimePackageOwnership", "TestP15RP45PackageIgnoresGitReplacements", "TestP15RP45IdentityValidationOrder", "TestP15RP45ExtractedInstallAndRollback", "TestP15RP45ExtractedRuntimePair", "TestP15RP45GitIndexSourceIntegrity", "TestP15RP45LockAcquisitionSignals"},
		Tiers:     []string{"skill-package", "integration"},
		Producers: map[string]string{"skill-package": "TestP15RP45ExtractedRuntimePair", "integration": "TestP15RP45ExtractedInstallAndRollback"},
		// One complete scenario owns multiple actual clean builds and more than
		// twenty separate shell transactions. This new producer has its own finite
		// whole-scenario budget; existing test and release deadlines are unchanged.
		Timeout: 20 * time.Minute,
	},
}

// The exact commit binds all production source. This reviewed inventory also
// binds every installer/parser/packager, root oracle and shared intake boundary
// used by the three independent processes, within the shared 64-blob limit.
var extractedInstallProductionPaths = []string{
	"skills/cherri-shortcuts/compatibility-manifest.json",
	"skills/cherri-shortcuts/scripts/setup.sh", "skills/cherri-shortcuts/scripts/installation.sh",
	"skills/cherri-shortcuts/scripts/common.sh", "skills/cherri-shortcuts/scripts/json-object.awk",
	"skills/cherri-shortcuts/scripts/doctor.sh", "skills/cherri-shortcuts/scripts/self-test.sh",
	"skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh", "skills/cherri-shortcuts/scripts/test-json-boundary.sh",
	"skills/cherri-shortcuts/scripts/real-release-integration.sh", "skills/cherri-shortcuts/scripts/build.sh",
	"skills/cherri-shortcuts/scripts/decompile.sh", "skills/cherri-shortcuts/scripts/prepare-edit.sh",
	"tools/p10r15/package_skill.py", ".github/workflows/openminis-skill.yml", ".github/workflows/p10r15-packaging-evidence.yml",
	"cmd/cherri-migrate/main.go", "cmd/cherri-migrate/migration_cli_args.go", "cmd/cherri-migrate/install_identity.go",
	"main.go", "language_cli_adapter.go", "language_backend_session.go", "language_backend_adapter.go",
	"internal/language/syntax/format.go", "internal/language/migrate/migrate.go",
	"internal/language/schema/schema.go", "internal/language/schema/generated_registry.go", "internal/language/backend/contracts.go",
	"tools/language-docs/main.go", "internal/shortcutcompare/compare.go", "internal/shortcutcompare/normalize.go",
	"p15_extracted_install_contract_test.go", "p15_install_identity_contract_test.go", "p15_install_identity_order_test.go",
	"p15_installed_docs_identity_test.go", "p15_extracted_atomic_install_test.go", "p15_install_index_and_lock_test.go",
	"p15_extracted_selection_test.go", "p15_extracted_runtime_pair_test.go", "p15_install_actual_tools_test.go",
	"p15_skill_installation_test.go", "p15_skill_packaging_test.go", "p15_skill_signal_test.go", "p15_skill_acceptance_test.go", "p15_cli_acceptance_test.go",
	"tests/language-v2/repair-regression-plan.json", "tests/language-v2/acceptance-contract.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/extracted_install_execution.go", "tools/language-acceptance/extracted_install_execution_test.go",
	"tools/language-acceptance/structural_migration_execution_test.go",
	"tools/language-acceptance/strict_go_events.go", "tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go", "tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
}

func extractedInstallRequiredEvents(id string) []string {
	scenario, ok := extractedInstallScenarios[id]
	if !ok {
		return nil
	}
	names := append([]string{}, scenario.Tests...)
	add := func(parent string, children ...string) {
		for _, child := range children {
			names = append(names, parent+"/"+child)
		}
	}
	switch id {
	case "RP42":
		add(scenario.Tests[0], "help_and_schema", "check_and_alias", "format_and_aliases", "compile_and_file_alias", "import_and_alias", "installed_migration_tool")
		add(scenario.Tests[1], "check", "write")
		for _, flag := range []string{"check", "write"} {
			add(scenario.Tests[1]+"/"+flag, "before_file", "after_file")
		}
	case "RP44":
		add(scenario.Tests[0], "missing_comma", "trailing_comma", "duplicate_schema", "duplicate_escaped_schema", "invalid_escape", "invalid_number", "unpaired_surrogate", "invalid_utf8", "raw_nul", "multiple_objects", "nested_fields_only", "missing_protocol", "wrong_protocol", "missing_catalog", "wrong_catalog", "missing_codec", "wrong_codec", "missing_macro", "wrong_schema_same_language", "not_json")
		add(scenario.Tests[1], "changed_docs_preserve_fingerprints", "missing_git_provenance", "wrong_commit", "wrong_schema", "wrong_macros")
		add(scenario.Tests[2], "outdated_bundle_prefers_cache", "outdated_cache_uses_bundle", "unselected_bundle_not_executed", "checksum_invalid_cache_not_executed", "incomplete_cache_uses_bundle", "both_outdated_rejected", "explicit_outdated_never_falls_back")
		add(scenario.Tests[3], "assume-unchanged", "skip-worktree")
	case "RP45":
		add(scenario.Tests[0], "reconciled_wrong_source", "wrong_migration_role", "missing_migration_binary", "migration_checksum_before_execution")
		add(scenario.Tests[1], "missing_companion", "wrong_compiler_role")
		add(scenario.Tests[2], "commit", "tree", "blob")
		add(scenario.Tests[3], "missing_checksum_before_execution", "migration_wrapper_cannot_borrow_identity", "different_platform_metadata")
		add(scenario.Tests[4], "offline_compatible_cache", "corrupt_download_compiler", "corrupt_download_migration", "explicit_migration_override_missing", "explicit_migration_override_wrong_role", "interrupt_migration_backup", "interrupt_migration_binary", "interrupt_migration_source_receipt", "interrupt_migration_checksum_receipt", "interrupt_documentation", "successful_complete_update")
		add(scenario.Tests[5], "exact_extracted_pair", "actual_offline_bundled_install", "shipped_compiler_and_migrator_execute", "controlled_installation_self_test")
		add(scenario.Tests[6], "assume-unchanged", "skip-worktree")
		add(scenario.Tests[7], "after_mkdir_HUP", "after_mkdir_INT", "after_mkdir_TERM", "existing_owner_preserved")
	}
	return names
}
func extractedInstallTestIDs(id string) []string {
	var result []string
	for _, name := range extractedInstallScenarios[id].Tests {
		result = append(result, "github.com/electrikmilk/cherri:"+name)
	}
	return result
}
func extractedInstallProducer(id, tier string) string {
	name := extractedInstallScenarios[id].Producers[tier]
	if name == "" {
		return ""
	}
	return "github.com/electrikmilk/cherri:" + name
}
func extractedInstallCommand(id string) []string {
	scenario := extractedInstallScenarios[id]
	var names []string
	for _, name := range scenario.Tests {
		names = append(names, regexp.QuoteMeta(name))
	}
	return []string{"go", "test", "-count=1", "-json", "-timeout", scenario.Timeout.String(), ".", "-run", "^(" + strings.Join(names, "|") + ")$"}
}
func extractedInstallArtifactNames(id string) []string {
	prefix := strings.ToLower(id)
	return []string{prefix + "-process.json", prefix + "-go.jsonl", prefix + "-stderr.log"}
}
func parseExtractedInstallOutput(id string, raw []byte) error {
	names := extractedInstallRequiredEvents(id)
	if err := parseStrictNamedProductionOutput(names, extractedInstallScenarios[id].Timeout+time.Minute, raw); err != nil {
		return err
	}
	// RP42 has selected nested check/write cases. Every selected ancestor must
	// remain active until all its required descendants have really passed.
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
					return fmt.Errorf("%s nested child outside selected ancestor", id)
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("%s parent passed before required nested child", id)
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validateExtractedInstallRecord(id string, record EvidenceRecord) error {
	scenario, selected := extractedInstallScenarios[id]
	if !selected {
		return nil
	}
	producer := extractedInstallProducer(id, record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, extractedInstallCommand(id)) || record.Assertions["required_production_tests"] != strings.Join(scenario.Tests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(extractedInstallRequiredEvents(id), "\n") || record.Assertions["scope"] != scenario.Scope || record.Assertions["component_tuple_only"] != "true" {
		return fmt.Errorf("%s record detached from complete extracted-install scenario", id)
	}
	names := extractedInstallArtifactNames(id)
	data, err := stateReadArtifacts(record.Artifacts, names)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[names[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[names[1]], data[names[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		!reflect.DeepEqual(receipt.Command, record.Command) || !reflect.DeepEqual(receipt.RequiredTests, extractedInstallRequiredEvents(id)) || !reflect.DeepEqual(receipt.DiscoveredTests, extractedInstallRequiredEvents(id)) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut || receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) ||
		receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) || receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("%s incomplete actual process/source/byte binding", id)
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (scenario.Timeout + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, extractedInstallProductionPaths); err != nil {
		return err
	}
	if err = parseExtractedInstallOutput(id, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeExtractedInstallProduction(id string) ([]EvidenceArtifact, string, bool, error) {
	scenario := extractedInstallScenarios[id]
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("%s requires exact clean source: %s clean=%t error=%v", id, before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, strings.ToLower(id)+"-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: id, SourceCommit: before, SourceCleanBefore: true, Command: extractedInstallCommand(id), RequiredTests: extractedInstallRequiredEvents(id)}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, extractedInstallProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), scenario.Timeout+time.Minute)
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
	receipt.ExitCode = &exitCode
	receipt.TimedOut = &timedOut
	after, afterClean, afterErr := editorSourceState(root)
	receipt.SourceCleanAfter = afterErr == nil && afterClean && after == before
	raw, stderr := output.Bytes(), errors.Bytes()
	receipt.RawSHA256 = editorSHA(raw)
	receipt.RawBytes = len(raw)
	receipt.StderrSHA256 = editorSHA(stderr)
	receipt.StderrBytes = len(stderr)
	parseErr := parseExtractedInstallOutput(id, raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = extractedInstallRequiredEvents(id)
	}
	encoded, encodeErr := json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	names := extractedInstallArtifactNames(id)
	contents := [][]byte{append(encoded, '\n'), raw, stderr}
	var artifacts []EvidenceArtifact
	for i, name := range names {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[i], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[i])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual %s process failed: process=%v parse=%v timeout=%t clean=%t", id, processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed the complete independent extracted-install scenario with actual tools, exact source/bytes and causal controls", true, nil
}
func (r *Runner) runExtractedInstallRequirement(spec RepairCaseSpec) bool {
	scenario, selected := extractedInstallScenarios[spec.ID]
	if !selected {
		return false
	}
	id, title, levels := spec.ID, spec.Title, spec.RequiredTestLevels
	if title != scenario.Title || spec.Scenario != scenario.Scenario || spec.ExpectedObservation != scenario.Expected || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), scenario.Tiers) {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "Extracted-install producer does not match the immutable contract"}
		return true
	}
	artifacts, detail, executed, err := r.executeExtractedInstallProduction(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, "repair", title, strings.Join(levels, "+"), scenario.Tiers, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = extractedInstallProducer(id, record.Tier)
		record.Command = extractedInstallCommand(id)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(scenario.Tests, "\n")
		record.Assertions["required_go_events"] = strings.Join(extractedInstallRequiredEvents(id), "\n")
		record.Assertions["scope"] = scenario.Scope
		record.Assertions["component_tuple_only"] = "true"
	}
	return true
}
