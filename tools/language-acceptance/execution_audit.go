package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var f01RequiredTests = []string{
	"TestP15F01StatementAndExpressionGuardRHS",
	"TestP15F01StaticSkipsMissingTarget",
	"TestP15F01BooleanStatementUsesValueNotPresence",
	"TestP15F01GuardOracleRejectsMutations",
	"TestP15F01RuntimeFixtureCompiles",
	"TestP15F01HybridEditedPredicateGuardsRHS",
}

var r06ProductionTests = []string{"TestP15R06ProductionImportFormats", "TestHYB18_SignedShortcutContainerRejected", "TestHYB20_TrailingGarbageAfterJSONRootRejected", "TestHYB21_MalformedActionEntriesRejectedWithExactIndex"}
var r07ProductionTests = []string{"TestSEM28_UserEditOnNaturalActionIsolated", "TestSEM29_UserEditInsideControlRegionIsolated", "TestSEM30_OnlyTargetActionChangesAndNoCollateralDelta", "TestSEM34_TamperDetectionAndStrictCompareAsymmetry"}

var cliProductionTests = map[string][]string{
	"CL01": {"TestP15CLIJSONCommandsAndExitStatus"},
	"CL02": {"TestP15CLIAnalysisHasNoExternalEffects"},
	"CL03": {"TestP15CLIDefaultRejectsLegacyWithGuidance"},
}

// Real install producers include source builds and multiple controlled shell
// transactions. Keep finite per-family budgets; all assertions remain unchanged.
func productionTestTimeout(required []string) time.Duration {
	budget := 3 * time.Minute
	for _, name := range required {
		if strings.HasPrefix(name, "TestP15K05") {
			return 8 * time.Minute
		}
		if strings.HasPrefix(name, "TestP15K03") {
			return 12 * time.Minute
		}
		if strings.HasPrefix(name, "TestP15K01") || strings.HasPrefix(name, "TestP15K02") || strings.HasPrefix(name, "TestP15K04") || name == "TestP15SkillFreshPinnedInstall" {
			budget = 6 * time.Minute
		}
	}
	return budget
}

// Execute named production tests with explicit discovery checks. Exit status
// alone cannot prove that any required scenario was selected.
func executeNamedProductionTests(required []string) (bool, string) {
	return executeNamedProductionTestsInPackage(".", required)
}

func executeNamedProductionTestsInPackage(packagePath string, required []string) (bool, string) {
	var ctx, cancel = context.WithTimeout(context.Background(), productionTestTimeout(required)+time.Minute)
	defer cancel()
	if len(required) == 0 {
		return false, "No production tests were selected"
	}
	command := productionTestCommandInPackage(packagePath, required)
	var cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = resolveRepoRoot()
	var output, err = cmd.CombinedOutput()
	var seen = make(map[string]bool)
	var scanner = bufio.NewScanner(bytes.NewReader(output))
	scanner.Buffer(make([]byte, 4096), 2*1024*1024)
	for scanner.Scan() {
		var event struct {
			Action string
			Test   string
		}
		if json.Unmarshal(scanner.Bytes(), &event) == nil && event.Action == "pass" {
			seen[event.Test] = true
		}
	}
	var missing []string
	for _, name := range required {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if err != nil || scanner.Err() != nil || len(missing) != 0 {
		// Retain a bounded causal diagnostic instead of discarding every nested
		// assertion or timeout line behind an undifferentiated exit status.
		tail := output
		if len(tail) > 8192 {
			tail = tail[len(tail)-8192:]
		}
		return false, fmt.Sprintf("Production tests failed or were not discovered: process=%v; missing=%v; scanner=%v; output_tail=%s", err, missing, scanner.Err(), tail)
	}
	return true, "Executed production tests: " + strings.Join(required, ", ")
}

func executeF01ProductionTests() (bool, string) {
	ok, detail := executeNamedProductionTests(f01RequiredTests)
	return ok, detail + "; signed runtime still required"
}

type ExecutionAuditRow struct {
	ID                  string   `json:"id"`
	Requirement         string   `json:"requirement"`
	Status              string   `json:"status"`
	Tier                []string `json:"tier"`
	RequiredTiers       []string `json:"required_tiers"`
	ProductionPath      []string `json:"production_path"`
	TestNames           []string `json:"test_names"`
	PositiveAndNegative bool     `json:"positive_and_negative"`
	SourceSHA           string   `json:"source_sha"`
	Evidence            []string `json:"evidence"`
	Blocker             string   `json:"blocker,omitempty"`
	Detail              string   `json:"detail"`
	Scope               string   `json:"scope"`
}

func writeExecutionAudit(runner *Runner, contracts *ContractsSet, outPath string) error {
	if err := writeLocalObservations(runner, filepath.Join(filepath.Dir(outPath), "acceptance-local-observations.json")); err != nil {
		return err
	}
	evidence, err := json.MarshalIndent(runner.records, "", "  ")
	if err != nil {
		return err
	}
	if err = os.WriteFile(filepath.Join(filepath.Dir(outPath), "acceptance-local-evidence.json"), append(evidence, '\n'), 0644); err != nil {
		return err
	}
	mapping, err := BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		return err
	}
	var rows = make([]ExecutionAuditRow, 0, mapping.TotalRequirements)
	for _, declared := range mapping.Records {
		var res, exists = runner.results[declared.ID]
		var row = ExecutionAuditRow{
			ID: declared.ID, Requirement: declared.Title, Status: "UNVERIFIED",
			Tier: []string{}, RequiredTiers: declared.RequiredTiers,
			ProductionPath: []string{"tools/language-acceptance/main.go"},
			TestNames:      []string{}, SourceSHA: runner.implSHA, Evidence: []string{},
			Scope: "Local runner assertion scope only; not universal feature, installed consumer, or Apple runtime acceptance",
		}
		if exists {
			row.Detail = res.Detail
			row.Tier = res.ExecutedLevels
			if row.Tier == nil {
				row.Tier = []string{}
			}
			switch {
			case res.Passed && res.Status == "PASSED":
				row.Status = "PASS"
			case res.Status == "FAILED":
				row.Status = "FAIL"
			case res.Status == "PENDING_EXTERNAL":
				row.Status = "BLOCKED_EXTERNAL"
				row.Blocker = "Required external tier has no validated current-SHA evidence in this local run"
			case res.Status == "AI_EVAL_NOT_RUN":
				row.Status = "BLOCKED_EXTERNAL"
				row.Blocker = "External held-out model evaluation endpoint not configured"
			default:
				row.Blocker = "Dedicated scenario producer has not executed; linked IDs do not satisfy this obligation"
			}
			for _, record := range runner.records {
				for _, id := range record.Requirements {
					if id == declared.ID {
						row.TestNames = appendUniqueAudit(row.TestNames, record.TestID)
						row.Evidence = appendUniqueAudit(row.Evidence, "acceptance-local-evidence.json#"+record.EvidenceID)
					}
				}
			}
		}
		if gap := incompleteLocalAssertions[row.ID]; gap != "" {
			row.Blocker = gap
			row.Evidence = []string{"acceptance-local-observations.json#" + row.ID}
			row.Scope = "Observed smoke only; no formal acceptance tier proven"
		}
		if row.ID == "D03" {
			row.TestNames = []string{documentationConsumerTestID, documentationAppTestID}
			row.ProductionPath = []string{"tools/p10r15/verify_documentation_consumers.py", "skills/cherri-shortcuts/scripts/search-docs.sh", "ios/scripts/collect_ios_test_evidence.py", "ios/scripts/build_documentation_index.py", "tools/language-acceptance/documentation_generation.go", "tools/language-acceptance/documentation_intake.go", "tools/language-acceptance/documentation_consumers_intake.go", "ios/CherriApp/Documentation/DocumentationIndex.swift", "ios/CherriApp/Documentation/DocumentationSearchView.swift"}
			row.Scope = "External published/installed/app consumers only; local model tests and file existence do not prove either required tier"
		}
		if tests, ok := editorProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, editorProductionPaths...)
			row.PositiveAndNegative = res.Passed
			row.Scope = "Executed local Cherri service/cache and desktop service benchmark only; no iOS UI latency or race tier inferred"
		}
		if tests, ok := formattingProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, formattingProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := workspaceProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, workspaceProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := documentationProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, documentationProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := catalogProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, catalogProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := parserProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = []string{"internal/language/syntax/lexer.go", "internal/language/syntax/parser.go", "internal/language/syntax/format.go", "internal/language/service/service.go", "p15_parser_acceptance_test.go"}
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := bindingProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, bindingProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := literalCollectionProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, literalCollectionProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed || (row.ID == "S01" || row.ID == "L02" || row.ID == "L04") && res.Status == "PENDING_EXTERNAL"
		}
		if tests, ok := metadataProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, metadataProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := nativePreservationProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, nativePreservationProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := valueFlowProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, valueFlowProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := callProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, callProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := typeProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, typeProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := skillProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, skillProductionPaths[row.ID]...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := upstreamRouteProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = append([]string{}, upstreamRouteProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := cliProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = []string{"main.go", "args.go", "language_cli_adapter.go", "p15_cli_acceptance_test.go"}
			row.PositiveAndNegative = res.Passed
		}
		if tests, ok := schemaProductionTests[row.ID]; ok {
			row.TestNames = append([]string{}, tests...)
			row.ProductionPath = []string{"actions", "tools/language-schema/main.go", "internal/language/schema/generated_registry.go", "p15_schema_production_test.go"}
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "U03" {
			row.TestNames = []string{"TestUpstreamPropagationRehearsal"}
			row.ProductionPath = []string{"actions/web.cherri", "actions/calendar.cherri", "tools/language-schema/main.go", "upstream_rehearsal_test.go", "p15_schema_production_test.go", "internal/language/schema/generated_registry.go", "internal/language/analysis/analyzer.go", "internal/language/service/service.go"}
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "F05" {
			row.TestNames = append([]string{}, loopControlScenario.TestNames...)
			row.ProductionPath = append([]string{}, loopControlScenario.ProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "AC02" {
			row.TestNames = append([]string{}, customActionScenario.TestNames...)
			row.ProductionPath = append([]string{}, customActionScenario.ProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "N02" {
			row.TestNames = []string{"tools/language-acceptance:runNumbers"}
			row.ProductionPath = []string{"internal/language/analysis/analyzer.go"}
		}
		if scenario, ok := declarationTriggerScenarios[row.ID]; ok {
			row.TestNames = append([]string{}, scenario.TestNames...)
			row.ProductionPath = append([]string{}, scenario.ProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "E07" {
			row.TestNames = []string{"tools/language-acceptance:runEditor"}
			row.ProductionPath = []string{"internal/language/service/service.go"}
			row.PositiveAndNegative = res.Status == "PENDING_EXTERNAL"
		}
		if row.ID == "R06" {
			row.TestNames = append([]string{}, r06ProductionTests...)
			row.ProductionPath = []string{"import_fidelity.go", "hybrid_import.go", "mobile_compiler.go"}
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "R07" {
			row.TestNames = append([]string{}, r07ProductionTests...)
			row.ProductionPath = []string{"semantic_import_planner.go", "hybrid_import.go", "import_provenance.go"}
			row.PositiveAndNegative = res.Passed
		}
		if producer, ok := supplementalProductionScenarios[row.ID]; ok {
			row.TestNames = append([]string{}, producer.TestNames...)
			row.ProductionPath = append([]string{}, producer.ProductionPaths...)
			row.PositiveAndNegative = res.Passed
		}
		if row.ID == "F01" {
			row.ProductionPath = []string{"internal/language/syntax/parser.go", "internal/language/analysis/analyzer.go", "internal/language/lower/lower.go", "internal/language/lower/short_circuit.go", "language_backend_adapter.go"}
			row.TestNames = append([]string{}, f01RequiredTests...)
			row.PositiveAndNegative = res.Status == "PENDING_EXTERNAL" || res.Passed
		}
		rows = append(rows, row)
	}
	var doc = map[string]any{
		"schema_version": "1", "source_sha": runner.implSHA, "generated_at": time.Now().UTC().Format(time.RFC3339),
		"acceptance_count": len(contracts.Acceptance.Cases), "repair_count": len(contracts.Repair.Cases),
		"recovery_gate_count": len(contracts.Gates.Gates), "closure_complete": false,
		"scope": "Executed local assertions and explicit unexecuted obligations; declaration mappings are not execution evidence",
		"rows":  rows,
	}
	var data, marshalErr = json.MarshalIndent(doc, "", "  ")
	if marshalErr != nil {
		return marshalErr
	}
	if err = os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(outPath, append(data, '\n'), 0644)
}

func appendUniqueAudit(items []string, item string) []string {
	for _, existing := range items {
		if strings.EqualFold(existing, item) {
			return items
		}
	}
	return append(items, item)
}
