package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
)

const previewEditTitle = "Preview bounded source edit"
const previewEditScenario = "Edit one mapped action parameter in rich preview; attempt a generated/unmapped action edit."
const previewEditExpected = "Only correct source region changes; unrelated comments/code preserved; unmapped edits are explained/read-only."
const previewEditScope = "Distinct RP41 local integration executes the real unsigned mobile compiler, same-compile native source map and JSON edit implementation on one expanded Unicode/comment source: generated read-only rejection, exact mapped edit, recompile, stale-witness rejection, fresh mapped edit and invalid-expression rejection. All original origin/corruption controls also execute. This proves only Go integration; the ios&&cgo export, Swift transaction, rendered WebView and iOS UI remain unexecuted. No E07, BRG28 or global closure is inferred."
const previewEditPrefix = "preview-edit-rp41"

// Preserve the original focused RP41 Go and process deadlines. The additional
// joined scenario must fit these same bounds; no retry or timeout relaxation.
const previewEditProcessBudget = 300 * time.Second

var previewEditTests = []string{
	"TestP15RP41CombinedPreviewEditProduction",
	"TestP15RP41NativePreviewIndexSourceEdit",
	"TestP15RP41PreviewSourceMapOriginBoundaries",
	"TestP15RP41PreviewSourceMapRejectsStaleAndCorruptWitnesses",
	"TestP15RP41PreviewMapPreservesNativeBytesAndASTCompatibility",
}
var previewEditRequiredTiers = []string{"integration", "ios-ui"}
var previewEditLocalTiers = []string{"integration"}
var previewEditProductionPaths = []string{
	"go.mod",
	"go.sum",
	"action.go",
	"actions_catalog.go",
	"actions_std.go",
	"actions/basic.cherri",
	"language_backend_adapter.go",
	"language_backend_session.go",
	"language_emit_adapter.go",
	"language_registry_adapter.go",
	"language_value_adapter.go",
	"mobile_compiler.go",
	"preview_source_mapping.go",
	"shortcutgen.go",
	"shared_codec.go",
	"hybrid_import.go",
	"ios_bridge.go",
	"internal/language/analysis/analyzer.go",
	"internal/language/backend/contracts.go",
	"internal/language/backend/parameter_codecs.go",
	"internal/language/lower/lower.go",
	"internal/language/schema/generated_registry.go",
	"internal/language/schema/schema.go",
	"internal/language/service/service.go",
	"internal/language/source/span.go",
	"internal/language/syntax/ast.go",
	"internal/language/syntax/lexer.go",
	"internal/language/syntax/parser.go",
	"internal/language/protocol/protocol.go",
	"p15_preview_source_mapping_test.go",
	"p15_preview_source_mapping_controls_test.go",
	"p15_preview_source_mapping_contract_test.go",
	"tools/language-acceptance/preview_edit_execution.go",
	"tools/language-acceptance/preview_edit_execution_test.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/editor_contract_execution.go",
	"tools/language-acceptance/editor_contract_execution_test.go",
	"tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/native_encoding_intake.go",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
}

func previewEditRequiredEvents() []string {
	names := append([]string{}, previewEditTests...)
	for _, child := range []string{
		"mapped_without_expansion", "mapped_after_native_expansion", "generated_condition_value_read_only",
	} {
		names = append(names, previewEditTests[1]+"/"+child)
	}
	for _, child := range []string{
		"identical_native_payloads", "function_body_and_envelope", "copy_expansions", "raw_lookalike", "imported_body",
		"binding_call", "named_argument", "omitted_optional", "omitted_default", "variadic_wire", "custom_builder",
		"native_boolean_checkbox", "callback_transformed_scalar_read_only", "backfill_debug", "backfill_release",
	} {
		names = append(names, previewEditTests[2]+"/"+child)
	}
	for _, child := range []string{
		"stale_source", "stale_name", "stale_mode", "stale_profile", "missing_map", "changed_plist",
		"unreviewed_schema", "map_integrity", "negative_index", "absent_wire", "invalid_expression",
		"rehashed_bad_count", "rehashed_wrong_identifier", "rehashed_duplicate_action", "rehashed_structural_key",
		"rehashed_partial_argument", "rehashed_other_call_argument",
	} {
		names = append(names, previewEditTests[3]+"/"+child)
	}
	sort.Strings(names)
	return names
}

func previewEditCommand() []string {
	return []string{"go", "test", "-json", "-count=1", "-timeout=60s",
		"-run", "^(" + strings.Join(previewEditTests, "|") + ")$", "."}
}

func previewEditTestIDs() []string {
	ids := make([]string, 0, len(previewEditTests))
	for _, name := range previewEditTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}

func (r *Runner) runPreviewEditRequirement(c RepairCaseSpec) bool {
	if c.ID != "RP41" {
		return false
	}
	level := strings.Join(c.RequiredTestLevels, "+")
	if c.Title != previewEditTitle || c.Scenario != previewEditScenario ||
		c.ExpectedObservation != previewEditExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), previewEditRequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level,
			Status: "FAILED", Detail: "RP41 producer differs from its immutable complete scenario"}
		return true
	}
	root, err := filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, previewEditPrefix+"-")
	}
	if err != nil {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level,
			Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	command := previewEditCommand()
	receipt, artifacts, runErr := runExactSourceProcess(root, r.implSHA, c.ID, previewEditPrefix,
		directory, command, previewEditProcessBudget, previewEditRequiredEvents(), previewEditProductionPaths)
	if len(artifacts) == 0 {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level,
			Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	raw, readErr := os.ReadFile(filepath.Join(directory, previewEditPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseStrictNamedProductionOutput(previewEditRequiredEvents(), previewEditProcessBudget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	detail := previewEditScope
	if runErr != nil {
		detail = runErr.Error()
	}
	first := len(r.records)
	r.Record(c.ID, "repair", c.Title, level, previewEditLocalTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = previewEditTestIDs()[0]
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(previewEditTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(previewEditRequiredEvents(), "\n")
		record.Assertions["scope"] = previewEditScope
	}
	return true
}

func validatePreviewEditRecord(id string, record EvidenceRecord) error {
	if id != "RP41" {
		return nil
	}
	if record.Tier != "integration" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != previewEditTestIDs()[0] ||
		!reflect.DeepEqual(record.Command, previewEditCommand()) ||
		record.Assertions["required_production_tests"] != strings.Join(previewEditTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(previewEditRequiredEvents(), "\n") ||
		record.Assertions["scope"] != previewEditScope {
		return fmt.Errorf("RP41 record is detached from its complete local preview-edit scenario; actual iOS UI producer is still required")
	}
	data, err := stateReadArtifacts(record.Artifacts, editorContractArtifacts(previewEditPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[previewEditPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data[previewEditPrefix+"-go.jsonl"], data[previewEditPrefix+"-stderr.log"]
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command,
		previewEditRequiredEvents(), raw, stderr, previewEditProcessBudget, previewEditProductionPaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(previewEditRequiredEvents(), previewEditProcessBudget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
