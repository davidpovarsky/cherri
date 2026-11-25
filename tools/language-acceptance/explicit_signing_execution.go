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

const explicitSigningTitle = "Exact explicit signing payload"
const explicitSigningScenario = "Compile A then B and sign with injected provider; stale globals/files and failed first signing; real sanitized HubSign/native flow after local tests."
const explicitSigningExpected = "Request bytes equal intended artifact, correct name/path/error handling, no unsigned masquerade; real imported artifact belongs to current fixture."
const explicitSigningScope = "Actual BRG26 local unit/integration execution compiles distinct A then B in both dialects, poisons ambient names/paths/workflow with A, and observes exact B request bytes/name through multipart and JSON providers. Bytes, explicit-file and mobile routes retain failure, recovery and unsigned-response boundaries, destination/sibling ownership and cleanup. The local provider returns clearly labeled protocol-only AEA1 test bytes. No Apple signature, import, execution, related requirement or full179 acceptance is established."
const explicitSigningPrefix = "explicit-signing-brg26"

var explicitSigningTests = []string{
	"TestBackendRecovery26ExplicitBytesAfterSequentialCompilation",
	"TestBackendRecovery26ExplicitFileAfterSequentialCompilation",
	"TestBackendRecovery26MobileAfterSequentialCompilation",
}
var explicitSigningRequiredTiers = []string{"unit", "integration", "ios-runtime"}
var explicitSigningLocalTiers = []string{"unit", "integration"}
var explicitSigningProductionPaths = []string{
	"signing.go", "shortcut.go", "shortcutgen.go", "mobile_compiler.go",
	"language_cli_adapter.go", "language_backend_session.go", "language_backend_adapter.go",
	"language_emit_adapter.go", "language_value_adapter.go", "shared_codec.go",
	"language_typed_literal_adapter.go",
	"internal/language/service/service.go", "internal/language/syntax/parser.go",
	"internal/language/schema/schema.go", "internal/language/schema/generated_registry.go",
	"internal/language/analysis/analyzer.go", "internal/language/lower/lower.go",
	"internal/language/backend/contracts.go", "internal/language/syntax/main_output.go",
	"p15_signing_explicit_joined_test.go", "p15_main_output_oracle_test.go",
	"p15_cli_acceptance_test.go", "p15_output_signing_contract_test.go",
	"p15_signing_atomic_contract_test.go", "p15_schema_production_test.go",
	"tools/language-acceptance/explicit_signing_execution.go",
	"tools/language-acceptance/explicit_signing_execution_test.go",
	"tools/language-acceptance/editor_contract_execution.go",
	"tools/language-acceptance/editor_source_binding.go",
	"tools/language-acceptance/editor_performance_execution.go",
	"tools/language-acceptance/editor_process_other.go",
	"tools/language-acceptance/editor_process_windows.go",
	"tools/language-acceptance/state_acceptance_execution.go",
	"tools/language-acceptance/state_acceptance_intake.go",
	"tools/language-acceptance/strict_go_events.go",
	"tools/language-acceptance/function_transport_execution.go",
	"tools/language-acceptance/loop_execution.go",
	"tools/language-acceptance/local_gate_source_binding.go",
	"tools/language-acceptance/native_encoding_intake.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/parser_execution.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/repair-regression-plan.json",
	"tests/language-v2/backend-recovery-gates.json",
}

// This is the authored complete inventory, not a list inferred from a passing
// stream. Each slash-separated name is one actual t.Run in the original test.
func explicitSigningRequiredEvents() []string {
	names := append([]string{}, explicitSigningTests...)
	for _, parent := range explicitSigningTests[:2] {
		for _, protocol := range []string{"multipart", "json"} {
			for _, dialect := range []string{"v2", "legacy"} {
				names = append(names, parent+"/"+protocol+"/"+dialect)
			}
		}
	}
	for _, dialect := range []string{"v2", "legacy"} {
		names = append(names, explicitSigningTests[2]+"/"+dialect)
	}
	sort.Strings(names)
	return names
}

func explicitSigningTestIDs() []string {
	var ids []string
	for _, name := range explicitSigningTests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}

func explicitSigningProducer(tier string) string {
	for i, allowed := range explicitSigningLocalTiers {
		if tier == allowed {
			return explicitSigningTestIDs()[i]
		}
	}
	return ""
}

func (r *Runner) runExplicitSigningRequirement(c RecoveryGate) bool {
	if c.ID != "BRG26" {
		return false
	}
	level := strings.Join(c.MinimumEvidenceTiers, "+")
	if !c.Mandatory || c.Title != explicitSigningTitle || c.Scenario != explicitSigningScenario ||
		c.ExpectedObservation != explicitSigningExpected ||
		!reflect.DeepEqual(parseRequiredTiers(level), explicitSigningRequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title,
			TestLevel: level, Status: "FAILED", Detail: "BRG26 producer differs from its complete immutable signing contract"}
		return true
	}
	root, err := filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, explicitSigningPrefix+"-")
	}
	if err != nil {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title,
			TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	command := productionTestCommand(explicitSigningTests)
	budget := productionTestTimeout(explicitSigningTests) + time.Minute
	receipt, artifacts, runErr := runExactSourceProcess(root, r.implSHA, c.ID, explicitSigningPrefix,
		directory, command, budget, explicitSigningRequiredEvents(), explicitSigningProductionPaths)
	if len(artifacts) == 0 {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title,
			TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	raw, readErr := os.ReadFile(filepath.Join(directory, explicitSigningPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseFunctionScenarioOutput(explicitSigningRequiredEvents(), budget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	detail := explicitSigningScope
	if runErr != nil {
		detail = runErr.Error()
	}
	first := len(r.records)
	r.Record(c.ID, "recovery_gate", c.Title, level, explicitSigningLocalTiers, runErr == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		record := &r.records[i]
		record.TestID = explicitSigningProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(explicitSigningTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(explicitSigningRequiredEvents(), "\n")
		record.Assertions["scope"] = explicitSigningScope
		record.Assertions["signing_scope"] = "LOCAL_PROTOCOL_ONLY"
		record.Assertions["apple_signature_authenticated"] = "false"
		record.Assertions["apple_runtime_observed"] = "false"
	}
	return true
}

func validateExplicitSigningRecord(id string, record EvidenceRecord) error {
	if id != "BRG26" {
		return nil
	}
	producer := explicitSigningProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(explicitSigningTests)) ||
		record.Assertions["required_production_tests"] != strings.Join(explicitSigningTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(explicitSigningRequiredEvents(), "\n") ||
		record.Assertions["scope"] != explicitSigningScope ||
		record.Assertions["signing_scope"] != "LOCAL_PROTOCOL_ONLY" ||
		record.Assertions["apple_signature_authenticated"] != "false" ||
		record.Assertions["apple_runtime_observed"] != "false" {
		return fmt.Errorf("BRG26 record is detached from complete explicit signing execution; actual Apple signing/import/runtime remains separately required")
	}
	data, err := stateReadArtifacts(record.Artifacts, editorContractArtifacts(explicitSigningPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[explicitSigningPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data[explicitSigningPrefix+"-go.jsonl"], data[explicitSigningPrefix+"-stderr.log"]
	budget := productionTestTimeout(explicitSigningTests) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command,
		explicitSigningRequiredEvents(), raw, stderr, budget, explicitSigningProductionPaths); err != nil {
		return err
	}
	if err = parseFunctionScenarioOutput(explicitSigningRequiredEvents(), budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
