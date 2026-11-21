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

type documentationRecoveryScenario struct {
	ID, Title, Group, Scope string
	Tiers, Tests            []string
}

const documentationRecoveryScope = "Actual immutable S/D/K execution: all88 compiler/docs/contextual Skill examples with exact intentional diagnostics and unsigned native plist checks; original source package queried through20 real action/catalog/current/explicit-Legacy/failure wrapper calls. BRG29 additionally requires fresh canonical reference generation,520 signature contracts with8 real drift rejections, and505 schema/CLI/mobile insertion-snippet parity. Linked D01/D02/D03/K05, native runtime, publication and final179 closure are not inherited."

var documentationRecoveryScenarios = map[string]documentationRecoveryScenario{
	"RP46": {"RP46", "Docs and Skill retrieval coherence", "repair", documentationRecoveryScope,
		[]string{"docs", "integration"}, []string{"TestP15DocumentationRecoveryConsumers"}},
	"BRG29": {"BRG29", "Generated active docs and executable examples", "recovery_gate", documentationRecoveryScope,
		[]string{"generation", "docs", "integration"}, []string{"TestP15DocumentationRecoveryConsumers", "TestP15DocumentationRecoveryGeneration"}},
}

var documentationRecoveryProductionPaths = []string{
	"p15_documentation_recovery_test.go", "p15_documentation_test.go", "p10_ssot_propagation_test.go",
	"tools/p10r15/documentation_recovery_pipeline.py",
	"tools/p10r15/documentation_recovery_intake.py",
	"tools/language-acceptance/documentation_recovery_nested.go",
	"tools/p10r15/documentation_recovery_examples.py", "tools/p10r15/test_documentation_recovery_examples.py",
	"tools/p10r15/documentation_recovery_wrappers.py", "tools/p10r15/test_documentation_recovery_wrappers.py",
	"tools/p10r15/package_skill.py",
	"skills/cherri-shortcuts/compatibility-manifest.json", "skills/cherri-shortcuts/SKILL.md",
	"skills/cherri-shortcuts/references/debug-logging.md", "skills/cherri-shortcuts/references/workflow.md",
	"skills/cherri-shortcuts/references/openminis.md", "skills/cherri-shortcuts/scripts/common.sh",
	"skills/cherri-shortcuts/scripts/action.sh", "skills/cherri-shortcuts/scripts/catalog.sh",
	"skills/cherri-shortcuts/scripts/search-docs.sh", "skills/cherri-shortcuts/scripts/json-object.awk",
	"tools/language-docs/main.go", "tools/language-docs/p15_contract_test.go",
	"internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"docs/language-v2/actions-reference.md", "docs/language-v2/language-guide.md",
	"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
	"tools/language-acceptance/documentation_recovery_execution.go",
	"tools/language-acceptance/documentation_recovery_execution_test.go",
	"tools/language-acceptance/strict_go_events.go", "tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/editor_source_binding.go", "tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go", "tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/native_encoding_intake.go", "tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/state_acceptance_execution.go", "tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go", "tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
}

func (s documentationRecoveryScenario) events() []string {
	var names []string
	for _, parent := range s.Tests {
		names = append(names, parent)
		children := []string{"all_documented_examples", "packaged_retrieval"}
		if strings.HasSuffix(parent, "Generation") {
			children = []string{"fresh_reference", "canonical_generation_contract", "schema_snippet_parity"}
		}
		for _, child := range children {
			names = append(names, parent+"/"+child)
		}
	}
	return names
}
func (s documentationRecoveryScenario) testIDs() []string {
	var ids []string
	for _, name := range s.Tests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}
func (s documentationRecoveryScenario) producer(tier string) string {
	if !documentationRecoveryContainsTier(s.Tiers, tier) {
		return ""
	}
	if tier == "generation" {
		return "github.com/electrikmilk/cherri:TestP15DocumentationRecoveryGeneration"
	}
	return "github.com/electrikmilk/cherri:TestP15DocumentationRecoveryConsumers"
}
func documentationRecoveryContainsTier(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
func (s documentationRecoveryScenario) artifacts() []string {
	prefix := strings.ToLower(s.ID)
	return []string{prefix + "-process.json", prefix + "-go.jsonl", prefix + "-stderr.log", prefix + "-nested.zip"}
}
func (s documentationRecoveryScenario) parse(raw []byte) error {
	return parseStrictNamedProductionOutput(s.events(), productionTestTimeout(s.Tests)+time.Minute, raw)
}

func validateDocumentationRecoveryRecord(id string, record EvidenceRecord) error {
	s, found := documentationRecoveryScenarios[id]
	if !found {
		return nil
	}
	producer := s.producer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) ||
		record.TestID != producer || !reflect.DeepEqual(record.Command, productionTestCommand(s.Tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(s.Tests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(s.events(), "\n") ||
		record.Assertions["scope"] != s.Scope {
		return fmt.Errorf("%s record is detached from complete immutable documentation execution", id)
	}
	names := s.artifacts()
	data, err := stateReadArtifacts(record.Artifacts, names)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[names[0]], &receipt); err != nil {
		return err
	}
	raw, stderr := data[names[1]], data[names[2]]
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != record.ImplementationSHA ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) ||
		!reflect.DeepEqual(receipt.RequiredTests, s.events()) || !reflect.DeepEqual(receipt.DiscoveredTests, s.events()) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || !utf8.Valid(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("%s incomplete documentation process/source/byte binding", id)
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(s.Tests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, documentationRecoveryProductionPaths); err != nil {
		return err
	}
	if err = s.parse(raw); err != nil {
		return err
	}
	if err = loopOutputWithinProcess(raw, receipt); err != nil {
		return err
	}
	var archive string
	for _, artifact := range record.Artifacts {
		if filepath.Base(artifact.Path) == names[3] {
			archive = artifact.Path
		}
	}
	nested, err := documentationRecoveryNestedOperation(record.ImplementationSHA, "verify", archive, "", id)
	if err != nil {
		return err
	}
	if nested.ArchiveSHA256 != editorSHA(data[names[3]]) || record.Assertions["nested_evidence_sha256"] != nested.ArchiveSHA256 {
		return fmt.Errorf("nested documentation artifact detached from original record")
	}
	return documentationRecoveryBindNestedReports(nested, raw)
}

func (r *Runner) executeDocumentationRecovery(s documentationRecoveryScenario) ([]EvidenceArtifact, string, bool, error) {
	for _, key := range []string{"CHERRI_DOCUMENTATION_SOURCE_ROOT", "CHERRI_D03_DOCS_REPO", "CHERRI_DOCUMENTATION_COMPILER"} {
		if os.Getenv(key) == "" {
			return nil, "", false, fmt.Errorf("%s requires explicit immutable input %s", s.ID, key)
		}
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	before, clean, stateErr := editorSourceState(root)
	if stateErr != nil || !clean || before != r.implSHA {
		return nil, "", false, fmt.Errorf("%s requires exact clean source: %s clean=%t error=%v", s.ID, before, clean, stateErr)
	}
	directory, err := stateEvidenceDirectory(r, strings.ToLower(s.ID)+"-")
	if err != nil {
		return nil, "", false, err
	}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: s.ID, SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(s.Tests), RequiredTests: s.events()}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, documentationRecoveryProductionPaths)
	if err != nil {
		return nil, "", false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(s.Tests)+time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, receipt.Command[0], receipt.Command[1:]...)
	command.Dir = root
	configureEditorProductionProcess(command)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "CHERRI_DOCUMENTATION_EVIDENCE_OUTPUT=") {
			command.Env = append(command.Env, entry)
		}
	}
	command.Env = append(command.Env, "CHERRI_DOCUMENTATION_EVIDENCE_OUTPUT="+directory)
	var output, errors loopBoundedOutput
	command.Stdout = &output
	command.Stderr = &errors
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
	parseErr := s.parse(raw)
	if parseErr == nil {
		parseErr = loopOutputWithinProcess(raw, receipt)
	}
	if parseErr == nil {
		receipt.DiscoveredTests = s.events()
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", command.Process != nil, err
	}
	names := s.artifacts()
	var nestedBytes []byte
	if processErr == nil && parseErr == nil && !timedOut && receipt.SourceCleanAfter {
		archive := filepath.Join(directory, names[3])
		nested, nestedErr := documentationRecoveryNestedOperation(before, "pack", archive, directory, s.ID)
		if nestedErr == nil {
			nestedErr = documentationRecoveryBindNestedReports(nested, raw)
		}
		if nestedErr != nil {
			parseErr = nestedErr
		} else {
			nestedBytes, err = os.ReadFile(archive)
			if err != nil {
				parseErr = err
			}
		}
	}
	contents := [][]byte{append(encoded, '\n'), raw, stderr, nestedBytes}
	var artifacts []EvidenceArtifact
	for index, name := range names {
		path := filepath.Join(directory, name)
		if index != 3 {
			if err = os.WriteFile(path, contents[index], 0600); err != nil {
				return artifacts, "", command.Process != nil, err
			}
		} else if len(nestedBytes) == 0 {
			continue
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[index])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", command.Process != nil, fmt.Errorf("%s actual documentation process failed: process=%v parse=%v timeout=%t clean=%t; evidence=%s", s.ID, processErr, parseErr, timedOut, receipt.SourceCleanAfter, directory)
	}
	return artifacts, "Executed original pinned documentation/package producers with complete required event inventory; original subprocess evidence is retained beside this receipt", true, nil
}

func (r *Runner) runDocumentationRecoveryRequirement(id, title string, levels []string) bool {
	s, found := documentationRecoveryScenarios[id]
	if !found {
		return false
	}
	if title != s.Title || !reflect.DeepEqual(parseRequiredTiers(strings.Join(levels, "+")), s.Tiers) {
		r.results[id] = CaseResult{ID: id, Group: s.Group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "FAILED", Detail: "documentation producer does not match the immutable requirement"}
		return true
	}
	artifacts, detail, executed, err := r.executeDocumentationRecovery(s)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: s.Group, Requirement: title, TestLevel: strings.Join(levels, "+"), Status: "NOT_RUN", Detail: detail}
		return true
	}
	first := len(r.records)
	r.Record(id, s.Group, title, strings.Join(levels, "+"), s.Tiers, err == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		record := &r.records[index]
		record.TestID = s.producer(record.Tier)
		record.Command = productionTestCommand(s.Tests)
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(s.Tests, "\n")
		record.Assertions["required_go_events"] = strings.Join(s.events(), "\n")
		record.Assertions["scope"] = s.Scope
		for _, artifact := range artifacts {
			if filepath.Base(artifact.Path) == s.artifacts()[3] {
				record.Assertions["nested_evidence_sha256"] = artifact.SHA256
			}
		}
	}
	return true
}
