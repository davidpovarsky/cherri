package main

import (
	"fmt"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const combinedEditorTitle = "Format, rename and stale edits"
const combinedEditorScenario = "Format twice; rename shadowed Unicode symbols; preserve same words in strings/comments; deliver out-of-order analysis responses."
const combinedEditorExpected = "Stable contents, semantic symbol-only edits, and stale-response rejection with correct caret spans."
const combinedEditorScope = "Distinct RP39 execution on one real Go Service and one Unicode document: format and commit then format again; outer and inner semantic rename with independently authored full text, exact declaration ownership and byte/rune/UTF-16 spans; unchanged comments, strings and non-target scopes; actual newer, stale and recovery AnalyzeVersioned calls with retained old response ownership and active document identity. Causal trace corruptions and strict source/process/event intake accompany the same-document path. Only unit/service are proved. Delayed response data and stale service inputs are checked locally; actual iOS editor publication, caret behavior, C ABI and UI execution remain unproved. Related E03/E04/E05/P05 IDs and full179 closure are not inherited."
const combinedEditorPrefix = "combined-editor-rp39"
const combinedEditorPackage = "./internal/language/service"
const combinedEditorPackageID = "github.com/electrikmilk/cherri/internal/language/service"

var combinedEditorTests = []string{"TestP15RP39CombinedDocumentService", "TestP15RP39CombinedDocumentOracleMutations"}
var combinedEditorLocalTiers = []string{"unit", "service"}
var combinedEditorRequiredTiers = []string{"unit", "service", "ios-ui"}

// The clean commit owns all source. These exact witnesses additionally bind the
// executed service/formatter/resolver, independent fixture/oracles and intake.
var combinedEditorProductionPaths = []string{
	"go.mod",
	"go.sum",
	"internal/language/service/service.go",
	"internal/language/service/semantic_symbols.go",
	"internal/language/service/context_position.go",
	"internal/language/service/diagnostics.go",
	"internal/language/service/module_analysis.go",
	"internal/language/service/asset_analysis.go",
	"internal/language/service/registry_lifetime.go",
	"internal/language/source/span.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/analysis/diagnostics.go",
	"internal/language/types/types.go",
	"internal/language/syntax/parser.go",
	"internal/language/syntax/lexer.go",
	"internal/language/syntax/ast.go",
	"internal/language/syntax/token.go",
	"internal/language/syntax/identifier.go",
	"internal/language/syntax/format.go",
	"internal/language/syntax/format_literals.go",
	"internal/language/syntax/format_precedence.go",
	"internal/language/schema/schema.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/schema/registry_clone.go",
	"internal/language/service/p15_editor_diagnostics_contract_test.go",
	"internal/language/service/p15_editor_symbols_contract_test.go",
	"internal/language/service/p15_editor_combined_contract_test.go",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/combined_editor_execution.go",
	"tools/language-acceptance/combined_editor_execution_test.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
	"tools/language-acceptance/parser_execution.go",
	"tools/language-acceptance/editor_contract_execution.go",
	"tools/language-acceptance/editor_contract_inventory.json",
	"tools/language-acceptance/editor_contract_execution_test.go",
	"tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/structural_migration_execution_test.go",
}

func combinedEditorRequiredEvents() []string {
	var names = append([]string{}, combinedEditorTests...)
	for _, fault := range []string{
		"format_first",
		"format_not_idempotent",
		"format_changes_document",
		"wrong_scope",
		"literal_changed",
		"comment_changed",
		"missing_edit",
		"byte_span",
		"utf16_span",
		"foreign_edit_document",
		"inner_changes_outer",
		"wrong_definition",
		"rename_changes_document",
		"schema",
		"old_relabelled_new",
		"new_uses_old_diagnostic",
		"stale_returns_old",
		"stale_drops_error",
		"delayed_old_mutated",
		"stale_replaces_active",
		"recovery_retains_error",
		"lower_version_overwrites_recovery",
	} {
		names = append(names, combinedEditorTests[1]+"/"+fault)
	}
	sort.Strings(names)
	return names
}

func combinedEditorCommand() []string {
	return productionTestCommandInPackage(combinedEditorPackage, combinedEditorTests)
}

func combinedEditorTestIDs() []string {
	return []string{combinedEditorPackageID + ":" + combinedEditorTests[0], combinedEditorPackageID + ":" + combinedEditorTests[1]}
}

func combinedEditorProducer(tier string) string {
	switch tier {
	case "service":
		return combinedEditorTestIDs()[0]
	case "unit":
		return combinedEditorTestIDs()[1]
	}
	return ""
}

func combinedEditorValidateOriginals(source string, artifacts []EvidenceArtifact) error {
	var data, err = stateReadArtifacts(artifacts, editorContractArtifacts(combinedEditorPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[combinedEditorPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[combinedEditorPrefix+"-go.jsonl"], data[combinedEditorPrefix+"-stderr.log"]
	var budget = productionTestTimeout(combinedEditorTests) + time.Minute
	if err = validateExactSourceProcess("RP39", source, receipt, combinedEditorCommand(), combinedEditorRequiredEvents(), raw, stderr, budget, combinedEditorProductionPaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutputInPackage(combinedEditorPackageID, combinedEditorRequiredEvents(), budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}

func (r *Runner) runCombinedEditorRequirement(spec RepairCaseSpec) bool {
	if spec.ID != "RP39" {
		return false
	}
	var level = strings.Join(spec.RequiredTestLevels, "+")
	if spec.Title != combinedEditorTitle || spec.Scenario != combinedEditorScenario || spec.ExpectedObservation != combinedEditorExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), combinedEditorRequiredTiers) {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "FAILED", Detail: "RP39 producer differs from its immutable combined scenario"}
		return true
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, combinedEditorPrefix+"-")
	}
	if err != nil {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var command = combinedEditorCommand()
	var budget = productionTestTimeout(combinedEditorTests) + time.Minute
	var _, artifacts, runErr = runExactSourceProcess(root, r.implSHA, spec.ID, combinedEditorPrefix, directory, command, budget, combinedEditorRequiredEvents(), combinedEditorProductionPaths)
	if len(artifacts) == 0 {
		r.results[spec.ID] = CaseResult{ID: spec.ID, Group: "repair", Requirement: spec.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	if runErr == nil {
		runErr = combinedEditorValidateOriginals(r.implSHA, artifacts)
	}
	var detail = combinedEditorScope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(spec.ID, "repair", spec.Title, level, combinedEditorLocalTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = combinedEditorProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact{}, artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(combinedEditorTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(combinedEditorRequiredEvents(), "\n")
		record.Assertions["scope"] = combinedEditorScope
	}
	return true
}

func validateCombinedEditorRecord(id string, record EvidenceRecord) error {
	if id != "RP39" {
		return nil
	}
	var producer = combinedEditorProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, combinedEditorCommand()) ||
		record.Assertions["required_production_tests"] != strings.Join(combinedEditorTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(combinedEditorRequiredEvents(), "\n") ||
		record.Assertions["scope"] != combinedEditorScope {
		return fmt.Errorf("RP39 record detached from complete same-document service execution; actual combined UI evidence remains required")
	}
	return combinedEditorValidateOriginals(record.ImplementationSHA, record.Artifacts)
}
