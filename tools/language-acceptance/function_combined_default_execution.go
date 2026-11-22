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

const functionCombinedDefaultTitle = "Function default and transport"
const functionCombinedDefaultScenario = "Compare omitted arguments to explicit false/0/empty text/list/map using type-correct functions and structured results."
const functionCombinedDefaultExpected = "Defaults apply only on absence; structured and optional transport does not collapse values."
const functionCombinedDefaultScope = "Thirteen distinct RP25 joined sources compare omitted, positional and named typed calls for false, zero, empty Text/List/Map and empty structured defaults. Optional variants separately compare explicit None, argument-supplied flags and value-presence flags, retaining each result/presence consumer. Complete native dispatcher, guarded default, child return, caller decoding and output graphs reject causal value, kind, presence, empty-clearing and consumer corruptions. Only unit/native-structure is established; actual Apple values/types, related requirement IDs and full179 closure remain unproved."
const functionCombinedDefaultPrefix = "function-default-rp25"

var functionCombinedDefaultTests = []string{"TestP15RP25CombinedDefaultTransportUnit", "TestP15RP25CombinedDefaultTransportNative"}
var functionCombinedDefaultRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var functionCombinedDefaultProductionPaths = append(append([]string{}, functionTransportProductionPaths...),
	"p15_combined_function_defaults_test.go", "internal/language/lower/lexical_scope.go",
	"tools/language-acceptance/function_combined_default_execution.go", "tools/language-acceptance/function_combined_default_execution_test.go",
)

func functionCombinedDefaultRequiredEvents() []string {
	var names = append([]string{}, functionCombinedDefaultTests...)
	for _, name := range []string{"false", "zero", "empty_text", "empty_list", "empty_map", "empty_list_default", "empty_map_default", "optional_false", "optional_zero", "optional_empty_text", "optional_empty_list", "optional_empty_map", "optional_none_default"} {
		names = append(names, functionCombinedDefaultTests[0]+"/"+name)
		var parent = functionCombinedDefaultTests[1] + "/" + name
		names = append(names, parent)
		var faults = []string{"missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled", "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code", "omitted_present", "positional_absent", "named_absent", "truthiness_guard", "presence_reads_value", "wrong_argument_slot", "fallback_branch", "fallback_kind", "fallback_value", "default_join", "child_return", "argument_native_kind", "payload_mirror", "supplied_value_reconciled", "caller_consumer", "result_name_reconciled"}
		if name == "empty_list_default" {
			faults = append(faults, "empty_clear", "empty_inherits_input")
		}
		if strings.HasPrefix(name, "optional_") {
			faults = append(faults, "fallback_presence", "fallback_marker", "child_truthiness", "child_nothing", "result_presence", "result_absent_value", "positional_none", "missing_present", "none_unsupplied", "caller_presence_key", "caller_truthiness", "predicate_uses_value", "predicate_polarity")
		}
		if name == "optional_false" {
			faults = append(faults, "fallback_bool_as_number")
		}
		for _, fault := range faults {
			names = append(names, parent+"/oracle/"+fault)
		}
	}
	for _, kind := range []string{"bool", "number", "text", "list", "map"} {
		for _, boundary := range []string{"wrong_default", "wrong_argument"} {
			names = append(names, functionCombinedDefaultTests[0]+"/"+boundary+"/"+kind)
		}
	}
	sort.Strings(names)
	return names
}

func functionCombinedDefaultTestIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + functionCombinedDefaultTests[0], "github.com/electrikmilk/cherri:" + functionCombinedDefaultTests[1]}
}

func functionCombinedDefaultProducer(tier string) string {
	for index, allowed := range functionTransportTiers {
		if tier == allowed {
			return functionCombinedDefaultTestIDs()[index]
		}
	}
	return ""
}

func (r *Runner) runFunctionCombinedDefaultRequirement(c RepairCaseSpec) bool {
	if c.ID != "RP25" {
		return false
	}
	var level = strings.Join(c.RequiredTestLevels, "+")
	if c.Title != functionCombinedDefaultTitle || c.Scenario != functionCombinedDefaultScenario || c.ExpectedObservation != functionCombinedDefaultExpected || !reflect.DeepEqual(parseRequiredTiers(level), functionCombinedDefaultRequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "FAILED", Detail: "RP25 producer differs from its immutable full scenario"}
		return true
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, functionCombinedDefaultPrefix+"-")
	}
	if err != nil {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var command = productionTestCommand(functionCombinedDefaultTests)
	var budget = productionTestTimeout(functionCombinedDefaultTests) + time.Minute
	var receipt, artifacts, runErr = runExactSourceProcess(root, r.implSHA, c.ID, functionCombinedDefaultPrefix, directory, command, budget, functionCombinedDefaultRequiredEvents(), functionCombinedDefaultProductionPaths)
	if len(artifacts) == 0 {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "repair", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, functionCombinedDefaultPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseFunctionScenarioOutput(functionCombinedDefaultRequiredEvents(), budget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	var detail = functionCombinedDefaultScope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(c.ID, "repair", c.Title, level, functionTransportTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = functionCombinedDefaultProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(functionCombinedDefaultTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(functionCombinedDefaultRequiredEvents(), "\n")
		record.Assertions["scope"] = functionCombinedDefaultScope
	}
	return true
}

func validateFunctionCombinedDefaultRecord(id string, record EvidenceRecord) error {
	if id != "RP25" {
		return nil
	}
	var producer = functionCombinedDefaultProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(functionCombinedDefaultTests)) || record.Assertions["required_production_tests"] != strings.Join(functionCombinedDefaultTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(functionCombinedDefaultRequiredEvents(), "\n") || record.Assertions["scope"] != functionCombinedDefaultScope {
		return fmt.Errorf("RP25 record is detached from its complete typed default/transport scenario; actual Apple producer is still required")
	}
	var data, err = stateReadArtifacts(record.Artifacts, editorContractArtifacts(functionCombinedDefaultPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[functionCombinedDefaultPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[functionCombinedDefaultPrefix+"-go.jsonl"], data[functionCombinedDefaultPrefix+"-stderr.log"]
	var budget = productionTestTimeout(functionCombinedDefaultTests) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, functionCombinedDefaultRequiredEvents(), raw, stderr, budget, functionCombinedDefaultProductionPaths); err != nil {
		return err
	}
	if err = parseFunctionScenarioOutput(functionCombinedDefaultRequiredEvents(), budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
