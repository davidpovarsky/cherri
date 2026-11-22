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

const functionReturnTitle = "Function return and recursion"
const functionReturnScenario = "Call add(2,b:3) then multiply its result by 2; run a small terminating recursion; reject illegal capture."
const functionReturnExpected = "Result is 10, caller continues, recursion returns correctly and no implicit main runtime capture occurs."
const functionReturnScope = "Distinct RP24 joined source invokes add(2,b:3), multiplies its result by 2, invokes terminating factorial(3), continues with +1 and returns all three actual producer references. Independent semantic types, exact illegal-capture diagnostics and no-emission controls accompany the complete typed native dispatcher/function/caller graph and causal recursion/continuation corruptions. This proves only unit/native-structure; actual Apple values 10, 6 and 7, related requirement IDs and full179 closure remain unproved."
const functionReturnPrefix = "function-return-rp24"

var functionReturnTests = []string{"TestP15RP24CombinedReturnRecursionUnit", "TestP15RP24CombinedReturnRecursionNative"}
var functionReturnRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}

// RP24 uses the same transport implementation and strict graph cursor, but owns
// its source fixture, independent tests, event inventory and process evidence.
var functionReturnProductionPaths = append(append([]string{}, functionTransportProductionPaths...),
	"p15_combined_function_transport_test.go", "tests/language-v2/v2-rp24-return-recursion.cherri",
	"tools/language-acceptance/function_return_execution.go", "tools/language-acceptance/function_return_execution_test.go",
)

func functionReturnRequiredEvents() []string {
	var names = append([]string{}, functionReturnTests...)
	for _, name := range []string{"immutable_capture", "mutable_capture", "later_capture", "wrong_argument", "wrong_return"} {
		names = append(names, functionReturnTests[0]+"/"+name)
	}
	for _, fault := range []string{"missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled", "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code", "named_right_slot", "add_operator", "add_return", "base_predicate", "base_threshold", "base_result", "base_return", "decrement_operator", "recursive_argument", "recursive_callee", "recursive_self_target", "recursive_cast", "recursive_product", "recursive_return", "main_first_argument", "main_named_argument", "main_argument_presence", "main_callee", "main_number_cast", "continuation_product", "initial_recursion_argument", "continuation_add", "final_observation", "final_return", "authored_name_reconciled"} {
		names = append(names, functionReturnTests[1]+"/oracle/"+fault)
	}
	sort.Strings(names)
	return names
}

func functionReturnTestIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + functionReturnTests[0], "github.com/electrikmilk/cherri:" + functionReturnTests[1]}
}

func functionReturnProducer(tier string) string {
	for index, allowed := range functionTransportTiers {
		if tier == allowed {
			return functionReturnTestIDs()[index]
		}
	}
	return ""
}

func (r *Runner) runFunctionReturnRequirement(c RepairCaseSpec) bool {
	if c.ID != "RP24" {
		return false
	}
	var level = strings.Join(c.RequiredTestLevels, "+")
	if c.Title != functionReturnTitle || c.Scenario != functionReturnScenario || c.ExpectedObservation != functionReturnExpected || !reflect.DeepEqual(parseRequiredTiers(level), functionReturnRequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "FAILED", Detail: "RP24 producer differs from its immutable full scenario"}
		return true
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, functionReturnPrefix+"-")
	}
	if err != nil {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var command = productionTestCommand(functionReturnTests)
	var budget = productionTestTimeout(functionReturnTests) + time.Minute
	var receipt, artifacts, runErr = runExactSourceProcess(root, r.implSHA, c.ID, functionReturnPrefix, directory, command, budget, functionReturnRequiredEvents(), functionReturnProductionPaths)
	if len(artifacts) == 0 {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, functionReturnPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseStrictNamedProductionOutput(functionReturnRequiredEvents(), budget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	var detail = functionReturnScope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(c.ID, "repair", c.Title, level, functionTransportTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = functionReturnProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(functionReturnTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(functionReturnRequiredEvents(), "\n")
		record.Assertions["scope"] = functionReturnScope
	}
	return true
}

func validateFunctionReturnRecord(id string, record EvidenceRecord) error {
	if id != "RP24" {
		return nil
	}
	var producer = functionReturnProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(functionReturnTests)) || record.Assertions["required_production_tests"] != strings.Join(functionReturnTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(functionReturnRequiredEvents(), "\n") || record.Assertions["scope"] != functionReturnScope {
		return fmt.Errorf("RP24 record is detached from its complete return/recursion scenario; actual Apple producer is still required")
	}
	var data, err = stateReadArtifacts(record.Artifacts, editorContractArtifacts(functionReturnPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[functionReturnPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[functionReturnPrefix+"-go.jsonl"], data[functionReturnPrefix+"-stderr.log"]
	var budget = productionTestTimeout(functionReturnTests) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, functionReturnRequiredEvents(), raw, stderr, budget, functionReturnProductionPaths); err != nil {
		return err
	}
	if err = parseStrictNamedProductionOutput(functionReturnRequiredEvents(), budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
