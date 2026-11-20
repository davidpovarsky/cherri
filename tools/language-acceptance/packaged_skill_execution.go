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
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const packagedSkillTitle = "Complete extracted Skill and immutable compatibility"
const packagedSkillScenario = "Fresh/extracted/offline/cache-old-bundle/docs-mismatch/schema-mismatch/corrupt-download/update-rollback workflows through actual scripts."
const packagedSkillExpected = "Exact compatible manifest identities; resolver selects validated binary; failed update preserves usable installation; no branch-only lock."
const packagedSkillScope = "Independent BRG30 fresh execution of complete source/runtime ZIP creation, verification, extraction and actual installed public tools. Actual current and historical d705 compilers exercise bundle/cache/explicit selection; original-byte capability/protocol/catalog/schema/codec/macro and exact-document identity faults must be observed before rejection. Actual immutable source/docs branches advance while installation retains exact pins; offline request counters remain zero, corrupt downloaded tools never execute, and handled update/lock interruptions preserve both prior tools, all receipts, exact docs and usable doctor. Runtime ZIP members and shipped migration/public commands execute. Disposable committed component tuples derive from this exact source and real generated docs; they are not the published release tuple. Controlled adversarial response/transport tests remain explicit controls. No K01-K05, RP42/RP44/RP45/RP46, published deployment, Apple execution, power-loss or concurrent-reader guarantee is inherited."

// The complete combined scenario contains the three existing 8m/12m/20m
// production selections. Per-operation deadlines remain unchanged.
const packagedSkillTimeout = 40 * time.Minute

var packagedSkillTests = []string{
	"TestP15RP42ExtractedInstallPublicCLI",
	"TestP15RP42MigrationDocumentedFlags",
	"TestP15RP44ExtractedCapabilitiesRejection",
	"TestP15RP44InstalledDocsIdentity",
	"TestP15RP44ExtractedCacheAndBundleSelection",
	"TestP15RP44GitIndexDocsIntegrity",
	"TestP15RP44ExtractedCompatibilityConsumer",
	"TestP15RP45InstalledBinaryOwnership",
	"TestP15RP45RuntimePackageOwnership",
	"TestP15RP45PackageIgnoresGitReplacements",
	"TestP15RP45IdentityValidationOrder",
	"TestP15RP45ExtractedInstallAndRollback",
	"TestP15RP45ExtractedRuntimePair",
	"TestP15RP45GitIndexSourceIntegrity",
	"TestP15RP45LockAcquisitionSignals",
}
var packagedSkillTiers = []string{"skill-package", "integration"}
var packagedSkillArtifacts = []string{"brg30-process.json", "brg30-go.jsonl", "brg30-stderr.log"}

// Immutable Git bytes bind actual shipped package/install scripts, compiler and
// migration entrypoints, every selected root oracle and this dedicated intake.
var packagedSkillProductionPaths = []string{
	"skills/cherri-shortcuts/compatibility-manifest.json",
	"skills/cherri-shortcuts/scripts/setup.sh",
	"skills/cherri-shortcuts/scripts/installation.sh",
	"skills/cherri-shortcuts/scripts/common.sh",
	"skills/cherri-shortcuts/scripts/json-object.awk",
	"skills/cherri-shortcuts/scripts/doctor.sh",
	"skills/cherri-shortcuts/scripts/self-test.sh",
	"skills/cherri-shortcuts/scripts/test-compatibility-isolation.sh",
	"skills/cherri-shortcuts/scripts/test-json-boundary.sh",
	"skills/cherri-shortcuts/scripts/real-release-integration.sh",
	"skills/cherri-shortcuts/scripts/build.sh",
	"skills/cherri-shortcuts/scripts/decompile.sh",
	"skills/cherri-shortcuts/scripts/prepare-edit.sh",
	"tools/p10r15/package_skill.py",
	".github/workflows/openminis-skill.yml",
	".github/workflows/p10r15-packaging-evidence.yml",
	"cmd/cherri-migrate/main.go",
	"cmd/cherri-migrate/migration_cli_args.go",
	"cmd/cherri-migrate/install_identity.go",
	"main.go",
	"language_cli_adapter.go",
	"language_backend_session.go",
	"language_backend_adapter.go",
	"internal/language/syntax/format.go",
	"internal/language/migrate/migrate.go",
	"internal/language/schema/schema.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/backend/contracts.go",
	"tools/language-docs/main.go",
	"internal/shortcutcompare/compare.go",
	"internal/shortcutcompare/normalize.go",
	"p15_extracted_install_contract_test.go",
	"p15_install_identity_contract_test.go",
	"p15_install_identity_order_test.go",
	"p15_installed_docs_identity_test.go",
	"p15_extracted_atomic_install_test.go",
	"p15_install_index_and_lock_test.go",
	"p15_extracted_selection_test.go",
	"p15_extracted_runtime_pair_test.go",
	"p15_install_actual_tools_test.go",
	"p15_skill_installation_test.go",
	"p15_skill_packaging_test.go",
	"p15_skill_signal_test.go",
	"p15_skill_acceptance_test.go",
	"p15_cli_acceptance_test.go",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/packaged_skill_execution.go",
	"tools/language-acceptance/packaged_skill_execution_test.go",
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
	"go.mod",
	"go.sum",
	"internal/language/analysis/analyzer.go",
	"internal/language/analysis/diagnostics.go",
	"internal/language/lower/lower.go",
	"internal/language/lower/main_output.go",
	"internal/language/lower/module_scope.go",
	"internal/language/migrate/main_output.go",
	"internal/language/service/module_analysis.go",
	"internal/language/syntax/main_output.go",
}

func packagedSkillRequiredEvents() []string {
	names := append([]string{}, packagedSkillTests...)
	add := func(index int, children ...string) {
		for _, child := range children {
			names = append(names, packagedSkillTests[index]+"/"+child)
		}
	}
	add(0, "help_and_schema", "check_and_alias", "format_and_aliases", "compile_and_file_alias", "import_and_alias", "installed_migration_tool")
	add(1, "check", "write", "check/before_file", "check/documented_after_file", "write/before_file", "write/documented_after_file")
	add(2, "missing_comma", "trailing_comma", "duplicate_schema", "duplicate_escaped_schema", "invalid_escape", "invalid_number", "unpaired_surrogate", "invalid_utf8", "raw_nul", "multiple_objects", "nested_fields_only", "missing_protocol", "wrong_protocol", "missing_catalog", "wrong_catalog", "missing_codec", "wrong_codec", "missing_macro", "wrong_schema_same_language", "not_json")
	add(3, "changed_docs_preserve_fingerprints", "missing_git_provenance", "wrong_commit", "wrong_schema", "wrong_macros")
	add(4, "outdated_bundle_prefers_cache", "outdated_cache_uses_bundle", "unselected_bundle_not_executed", "checksum_invalid_cache_not_executed", "incomplete_cache_uses_bundle", "both_outdated_rejected", "explicit_outdated_never_falls_back")
	add(5, "assume-unchanged", "skip-worktree")
	add(7, "reconciled_wrong_source", "wrong_migration_role", "missing_migration_binary", "migration_checksum_before_execution")
	add(8, "missing_companion", "wrong_compiler_role")
	add(9, "commit", "tree", "blob")
	add(10, "missing_checksum_before_execution", "migration_wrapper_cannot_borrow_identity", "different_platform_metadata")
	add(11, "offline_compatible_cache", "corrupt_download_compiler", "corrupt_download_migration", "explicit_migration_override_missing", "explicit_migration_override_wrong_role", "interrupt_migration_backup", "interrupt_migration_binary", "interrupt_migration_source_receipt", "interrupt_migration_checksum_receipt", "interrupt_documentation", "successful_complete_update")
	add(12, "exact_extracted_pair", "actual_offline_bundled_install", "shipped_compiler_and_migrator_execute", "controlled_installation_self_test")
	add(13, "assume-unchanged", "skip-worktree")
	add(14, "after_mkdir_HUP", "after_mkdir_INT", "after_mkdir_TERM", "existing_owner_preserved")
	return names
}
func packagedSkillTestIDs() []string {
	var ids []string
	for _, name := range packagedSkillTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func packagedSkillProducer(tier string) string {
	switch tier {
	case "skill-package":
		return packagedSkillTestIDs()[12]
	case "integration":
		return packagedSkillTestIDs()[11]
	}
	return ""
}
func packagedSkillCommand() []string {
	var names []string
	for _, name := range packagedSkillTests {
		names = append(names, regexp.QuoteMeta(name))
	}
	return []string{"go", "test", "-count=1", "-json", "-timeout", packagedSkillTimeout.String(), ".", "-run", "^(" + strings.Join(names, "|") + ")$"}
}

// This complete manifest exceeds one 64-object read. Preflight the same
// 4MiB/object and16MiB aggregate limits before two bounded immutable batches.
func packagedSkillSourceFiles(repository, commit string) ([]editorSourceFile, error) {
	if len(packagedSkillProductionPaths) == 0 || len(packagedSkillProductionPaths) > 128 {
		return nil, fmt.Errorf("invalid bounded BRG30 source inventory")
	}
	seen := map[string]bool{}
	for _, name := range packagedSkillProductionPaths {
		if seen[name] {
			return nil, fmt.Errorf("duplicate BRG30 source path")
		}
		seen[name] = true
	}
	if err := packagedSkillVerifySourceBudget(repository, commit); err != nil {
		return nil, err
	}
	var files []editorSourceFile
	for start := 0; start < len(packagedSkillProductionPaths); start += 64 {
		end := start + 64
		if end > len(packagedSkillProductionPaths) {
			end = len(packagedSkillProductionPaths)
		}
		batch, err := editorGitSourceFiles(repository, commit, packagedSkillProductionPaths[start:end])
		if err != nil {
			return nil, err
		}
		files = append(files, batch...)
	}
	return files, nil
}

var packagedSkillSourceBudgetCache sync.Map

func packagedSkillVerifySourceBudget(repository, commit string) error {
	if !documentationHash(commit, 40) {
		return fmt.Errorf("invalid BRG30 immutable source commit")
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return err
	}
	key := root + "\x00" + commit + "\x00" + strings.Join(packagedSkillProductionPaths, "\n")
	if _, found := packagedSkillSourceBudgetCache.Load(key); found {
		return nil
	}
	var requests strings.Builder
	for _, name := range packagedSkillProductionPaths {
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
		return fmt.Errorf("immutable BRG30 source size preflight unavailable: %w", err)
	}
	if err := packagedSkillSourceBudget(output.Bytes(), len(packagedSkillProductionPaths)); err != nil {
		return err
	}
	packagedSkillSourceBudgetCache.Store(key, true)
	return nil
}
func packagedSkillSourceBudget(raw []byte, required int) error {
	if required < 1 || required > 128 || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return fmt.Errorf("invalid bounded BRG30 source size preflight")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	if len(lines) != required {
		return fmt.Errorf("incomplete BRG30 source size inventory")
	}
	var total int64
	for _, line := range lines {
		header := strings.Fields(string(line))
		if len(header) != 3 || !documentationHash(header[0], 40) || header[1] != "blob" {
			return fmt.Errorf("invalid immutable BRG30 source object")
		}
		size, err := strconv.ParseInt(header[2], 10, 64)
		if err != nil || size < 0 || size > 4<<20 {
			return fmt.Errorf("invalid BRG30 source object size")
		}
		total += int64(len(line)) + 1 + size + 1
		if total > 16<<20 {
			return fmt.Errorf("BRG30 source aggregate exceeds16MiB")
		}
	}
	return nil
}
func parsePackagedSkillOutput(raw []byte) error {
	names := packagedSkillRequiredEvents()
	if err := parseStrictNamedProductionOutput(names, packagedSkillTimeout+time.Minute, raw); err != nil {
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
					return fmt.Errorf("BRG30 child outside selected route parent")
				}
			}
			run[event.Test] = true
		case "pass":
			for name := range selected {
				if strings.HasPrefix(name, event.Test+"/") && !passed[name] {
					return fmt.Errorf("BRG30 route passed before required child")
				}
			}
			passed[event.Test] = true
		}
	}
	return nil
}
func validatePackagedSkillRecord(id string, record EvidenceRecord) error {
	if id != "BRG30" {
		return nil
	}
	producer := packagedSkillProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, packagedSkillCommand()) ||
		record.Assertions["required_production_tests"] != strings.Join(packagedSkillTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(packagedSkillRequiredEvents(), "\n") ||
		record.Assertions["scope"] != packagedSkillScope || record.Assertions["component_tuple_only"] != "true" {
		return fmt.Errorf("BRG30 record detached from complete actual extracted-Skill and immutable compatibility scenario")
	}
	data, err := stateReadArtifacts(record.Artifacts, packagedSkillArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[packagedSkillArtifacts[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[packagedSkillArtifacts[1]], data[packagedSkillArtifacts[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, packagedSkillRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, packagedSkillRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete BRG30 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (packagedSkillTimeout + time.Minute).Seconds()); err != nil {
		return err
	}
	immutable, err := packagedSkillSourceFiles(resolveRepoRoot(), record.ImplementationSHA)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(receipt.SourceFiles, immutable) {
		return fmt.Errorf("BRG30 immutable source manifest mismatch")
	}

	if err = parsePackagedSkillOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executePackagedSkillProduction() ([]EvidenceArtifact, string, bool, error) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("BRG30 requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, "brg30-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: "BRG30", SourceCommit: before, SourceCleanBefore: true, Command: packagedSkillCommand(), RequiredTests: packagedSkillRequiredEvents()}
	receipt.SourceFiles, err = packagedSkillSourceFiles(root, before)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), packagedSkillTimeout+time.Minute)
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
	parseErr := parsePackagedSkillOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = packagedSkillRequiredEvents()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	contents := map[string][]byte{packagedSkillArtifacts[0]: append(encoded, '\n'), packagedSkillArtifacts[1]: raw, packagedSkillArtifacts[2]: stderr}
	var artifacts []EvidenceArtifact
	for _, name := range packagedSkillArtifacts {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual BRG30 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed complete BRG30 extracted source/runtime tools, immutable compatibility, offline selection and actual update rollback scenarios", true, nil
}
func (r *Runner) runPackagedSkillRequirement(spec RecoveryGate) bool {
	if spec.ID != "BRG30" {
		return false
	}
	level := strings.Join(spec.MinimumEvidenceTiers, "+")
	if spec.Title != packagedSkillTitle || spec.Scenario != packagedSkillScenario || spec.ExpectedObservation != packagedSkillExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), packagedSkillTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "BRG30 producer does not match the immutable full scenario"}
		return true
	}
	artifacts, detail, executed, err := r.executePackagedSkillProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "recovery_gate", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(spec.ID, "recovery_gate", spec.Title, level, packagedSkillTiers, err == nil, "PASSED", detail)
	// Generic Record cannot mint package evidence. This dedicated producer
	// appends it only after the actual complete process has been observed.
	packageResult, packageExit := "failed", 1
	if err == nil {
		packageResult, packageExit = "passed", 0
	}
	r.records = append(r.records, EvidenceRecord{EvidenceID: "ev-local-BRG30-skill-package", Requirements: []string{"BRG30"}, Tier: "skill-package", ImplementationSHA: r.implSHA, ExitCode: packageExit, Result: packageResult, Assertions: map[string]string{"assertion": detail}, Detail: detail})
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = packagedSkillProducer(record.Tier)
		record.Command = packagedSkillCommand()
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(packagedSkillTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(packagedSkillRequiredEvents(), "\n")
		record.Assertions["scope"] = packagedSkillScope
		record.Assertions["component_tuple_only"] = "true"
	}
	return true
}
