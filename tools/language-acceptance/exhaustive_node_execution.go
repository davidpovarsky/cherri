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

const exhaustiveNodeScope = "Independent RP16 execution: all 37 approved declaration/statement/expression role-kinds plus 5 type-annotation descendants pass real checked lowering and canonical native witnesses; every inventory omission and native witness corruption is rejected; 23 injected unknown-node contexts reject before canonical finalization with healthy subsequent sessions. Related P03/F03/FN01/M03 requirements are not inherited."

var exhaustiveNodeTests = []string{"TestP15RP16ApprovedNodeInventory", "TestP15RP16InventoryControls", "TestP15RP16UnknownNodesCanonicalBoundary", "TestP15RP16UnknownTypeAnnotationsCanonicalBoundary"}
var exhaustiveNodeTiers = []string{"unit", "native-negative"}
var exhaustiveNodeArtifacts = []string{"rp16-process.json", "rp16-go.jsonl", "rp16-stderr.log"}
var exhaustiveNodeKinds = []string{
	"declaration_ImportDecl", "declaration_ShortcutDecl", "declaration_TriggerDecl", "declaration_SetupDecl", "declaration_TypeDecl", "declaration_EnumDecl", "declaration_FunctionDecl", "declaration_ActionDecl", "declaration_CopyBlockStmt",
	"statement_BindingStmt", "statement_AssignStmt", "statement_ExprStmt", "statement_ReturnStmt", "statement_YieldStmt", "statement_BlockStmt", "statement_IfStmt", "statement_ForStmt", "statement_RepeatStmt", "statement_MenuStmt", "statement_CopyBlockStmt", "statement_PasteStmt",
	"expression_LiteralExpr", "expression_FStringExpr", "expression_IdentExpr", "expression_MemberExpr", "expression_IndexExpr", "expression_CallExpr", "expression_UnaryExpr", "expression_BinaryExpr", "expression_ListExpr", "expression_MapExpr", "expression_RecordExpr", "expression_EnumMemberExpr", "expression_IfStmt", "expression_ForStmt", "expression_RepeatStmt", "expression_MenuStmt",
	"type_annotation_NamedTypeAnnotation", "type_annotation_OptionalTypeAnnotation", "type_annotation_GenericTypeAnnotation", "type_annotation_UnionTypeAnnotation", "type_annotation_RecordTypeAnnotation",
}
var exhaustiveNodeNegativeContexts = []string{"declaration", "statement", "expression", "call_argument", "map_key", "map_value", "list_item", "text_reference", "called_function", "unused_function", "unused_copy", "metadata_record"}
var exhaustiveNodeTypeContexts = []string{"alias", "binding", "function_parameter", "function_return", "action_parameter", "action_return", "setup", "optional_child", "generic_child", "union_child", "record_child"}
var exhaustiveNodeProductionPaths = []string{
	"internal/language/syntax/ast.go", "internal/language/syntax/parser.go", "internal/language/syntax/lexer.go", "internal/language/source/span.go",
	"internal/language/analysis/analyzer.go", "internal/language/analysis/module_imports.go", "internal/language/modules/graph.go",
	"internal/language/lower/lower.go", "internal/language/lower/exhaustive_validation.go", "internal/language/lower/module_scope.go",
	"internal/language/lower/scoped_blocks.go", "internal/language/lower/loop_index.go", "internal/language/lower/lexical_scope.go",
	"internal/language/backend/contracts.go", "internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"language_backend_session.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go", "shortcutgen.go",
	"p15_exhaustive_ast_boundary_test.go", "p15_exhaustive_ast_type_boundary_test.go", "p15_exhaustive_ast_inventory_test.go", "p15_exhaustive_ast_native_witness_test.go",
	"tests/language-v2/rp16-approved-nodes.json", "tests/language-v2/repair-regression-plan.json", "tests/language-v2/acceptance-contract.json", "tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/exhaustive_node_execution.go", "tools/language-acceptance/exhaustive_node_execution_test.go", "tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/loop_execution.go", "tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go", "tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/state_acceptance_intake.go", "tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go", "tools/language-acceptance/execution_audit.go",
}

func exhaustiveNodeRequiredEvents() []string {
	var names = []string{exhaustiveNodeTests[0]}
	for _, kind := range exhaustiveNodeKinds {
		names = append(names, exhaustiveNodeTests[0]+"/"+kind)
	}
	names = append(names, exhaustiveNodeTests[1])
	for _, kind := range exhaustiveNodeKinds {
		names = append(names, exhaustiveNodeTests[1]+"/missing_"+kind)
	}
	for _, name := range []string{"duplicate_kind", "unknown_kind", "wrong_role"} {
		names = append(names, exhaustiveNodeTests[1]+"/"+name)
	}
	names = append(names, exhaustiveNodeTests[2])
	for _, name := range exhaustiveNodeNegativeContexts {
		names = append(names, exhaustiveNodeTests[2]+"/"+name)
	}
	names = append(names, exhaustiveNodeTests[3])
	for _, name := range exhaustiveNodeTypeContexts {
		names = append(names, exhaustiveNodeTests[3]+"/"+name)
	}
	return names
}
func exhaustiveNodeTestIDs() []string {
	var ids []string
	for _, name := range exhaustiveNodeTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func exhaustiveNodeProducer(tier string) string {
	if tier == "unit" {
		return exhaustiveNodeTestIDs()[0]
	}
	if tier == "native-negative" {
		return exhaustiveNodeTestIDs()[2]
	}
	return ""
}
func parseExhaustiveNodeOutput(raw []byte) error {
	return parseStrictNamedProductionOutput(exhaustiveNodeRequiredEvents(), productionTestTimeout(exhaustiveNodeTests)+time.Minute, raw)
}
func validateExhaustiveNodeRecord(id string, record EvidenceRecord) error {
	if id != "RP16" {
		return nil
	}
	var producer = exhaustiveNodeProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"RP16"}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(exhaustiveNodeTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(exhaustiveNodeTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(exhaustiveNodeRequiredEvents(), "\n") ||
		record.Assertions["scope"] != exhaustiveNodeScope {
		return fmt.Errorf("RP16 record is detached from complete exhaustive-node production")
	}
	var data, err = stateReadArtifacts(record.Artifacts, exhaustiveNodeArtifacts)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["rp16-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data["rp16-go.jsonl"], data["rp16-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "RP16" || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, exhaustiveNodeRequiredEvents()) || !reflect.DeepEqual(receipt.DiscoveredTests, exhaustiveNodeRequiredEvents()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete RP16 process/source/byte binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(exhaustiveNodeTests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, exhaustiveNodeProductionPaths); err != nil {
		return err
	}
	if err = parseExhaustiveNodeOutput(raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
func (r *Runner) executeExhaustiveNodeProduction() ([]EvidenceArtifact, string, bool, error) {
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("RP16 producer requires exact clean source: %s clean=%t error=%v", before, clean, stateErr)
	}
	var directory, dirErr = stateEvidenceDirectory(r, "rp16-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: "RP16", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(exhaustiveNodeTests), RequiredTests: exhaustiveNodeRequiredEvents()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, exhaustiveNodeProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(exhaustiveNodeTests)+time.Minute)
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
	var parseErr = parseExhaustiveNodeOutput(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = exhaustiveNodeRequiredEvents()
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return nil, "", command.Process != nil, encodeErr
	}
	var contents = map[string][]byte{"rp16-process.json": append(encoded, '\n'), "rp16-go.jsonl": raw, "rp16-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range exhaustiveNodeArtifacts {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", command.Process != nil, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("actual RP16 process failed: process=%v parse=%v timeout=%t clean=%t", processErr, parseErr, timedOut, receipt.SourceCleanAfter)
	}
	return artifacts, "Executed every approved AST role-kind, native witness/inventory controls and complete actual canonical unknown-node boundaries", true, nil
}
func (r *Runner) runExhaustiveNodeRequirement(id, title string, levels []string) bool {
	if id != "RP16" {
		return false
	}
	if title != "Exhaustive node handling" || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), exhaustiveNodeTiers) {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "RP16 producer does not match the immutable contract"}
		return true
	}
	var artifacts, detail, executed, err = r.executeExhaustiveNodeProduction()
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "repair", Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	var first = len(r.records)
	r.Record(id, "repair", title, strings.Join(levels, "+"), exhaustiveNodeTiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = exhaustiveNodeProducer(record.Tier)
		record.Command = productionTestCommand(exhaustiveNodeTests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(exhaustiveNodeTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(exhaustiveNodeRequiredEvents(), "\n")
		record.Assertions["scope"] = exhaustiveNodeScope
	}
	return true
}
