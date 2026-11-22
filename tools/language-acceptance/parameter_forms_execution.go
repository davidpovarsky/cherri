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

const parameterFormsTitle = "Parameter forms and typed defaults"
const parameterFormsScenario = "Test omitted and explicit false/zero/empty/default values, literal-only, reference-only, numeric-text codecs, enum, named variadic list, extra positional args."
const parameterFormsExpected = "Valid representations accepted by canonical backend; wrong forms diagnosed; no default/presence collapse or implicit argument shifting."
const parameterFormsScope = "Distinct BRG11 joined source compares omitted, explicit and reordered named Bool/Number/Text/List/Map defaults, then executes canonical literal-only enum, materialized/reference token, exact numeric-text conversion/consumer and ordered named variadic List routes. Complete typed native graph, exact invalid-form diagnostics and causal controls cover every default slot, presence decision and final consumer. Only unit/native-structure is established; actual Apple observations, related requirement IDs and full179 closure remain unproved."
const parameterFormsPrefix = "parameter-forms-brg11"

var parameterFormsTests = []string{"TestP15BRG11CombinedParameterFormsUnit", "TestP15BRG11CombinedParameterFormsNative"}
var parameterFormsRequiredTiers = []string{"unit", "native-structure", "ios-runtime"}
var parameterFormsProductionPaths = append(append([]string{}, functionTransportProductionPaths...),
	"p15_combined_function_defaults_test.go", "p15_combined_parameter_forms_test.go",
	"tools/language-acceptance/parameter_forms_execution.go", "tools/language-acceptance/parameter_forms_execution_test.go",
	"actions/a11y.cherri", "actions/basic.cherri", "actions/text.cherri", "actions/math.cherri", "actions_std.go",
	"internal/language/schema/schema.go", "internal/language/schema/variadic.go", "internal/language/analysis/literal_arguments.go", "internal/language/analysis/enums.go",
	"internal/language/enums/catalog.go", "internal/language/lower/enum_values.go", "internal/language/lower/literal_text_carriers.go",
)

func parameterFormsRequiredEvents() []string {
	var names = append([]string{}, parameterFormsTests...)
	for _, name := range []string{"literal_binding", "literal_runtime", "enum_invalid", "numeric_text_without_conversion", "native_extra", "native_extra_after_named", "duplicate_primary", "unknown_label", "missing_required", "function_reserved_slot", "duplicate_variadic_pack"} {
		names = append(names, parameterFormsTests[0]+"/"+name)
	}
	for _, kind := range []string{"bool", "number", "text", "list", "map"} {
		names = append(names, parameterFormsTests[0]+"/wrong_default/"+kind)
	}
	for _, fault := range []string{"missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled", "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code"} {
		names = append(names, parameterFormsTests[1]+"/oracle/"+fault)
	}
	for _, name := range []string{"flag", "count", "label", "items", "mapping"} {
		for _, fault := range []string{"slot", "presence_slot", "truthiness", "omitted_present", "explicit_absent", "named_absent", "fallback_kind", "join"} {
			names = append(names, parameterFormsTests[1]+"/defaults/"+name+"/"+fault)
		}
	}
	for _, fault := range []string{"literal_value", "enum_value", "literal_as_reference", "token_as_literal", "reference_producer", "number_as_real", "numeric_text_changed", "numeric_consumer", "variadic_cardinality", "variadic_order", "variadic_reference", "variadic_native_kind", "structured_result", "final_return"} {
		names = append(names, parameterFormsTests[1]+"/forms/"+fault)
	}
	sort.Strings(names)
	return names
}

func parameterFormsTestIDs() []string {
	return []string{"github.com/electrikmilk/cherri:" + parameterFormsTests[0], "github.com/electrikmilk/cherri:" + parameterFormsTests[1]}
}

func parameterFormsProducer(tier string) string {
	for index, allowed := range functionTransportTiers {
		if tier == allowed {
			return parameterFormsTestIDs()[index]
		}
	}
	return ""
}

func (r *Runner) runParameterFormsRequirement(c RecoveryGate) bool {
	if c.ID != "BRG11" {
		return false
	}
	var level = strings.Join(c.MinimumEvidenceTiers, "+")
	if !c.Mandatory || c.Title != parameterFormsTitle || c.Scenario != parameterFormsScenario || c.ExpectedObservation != parameterFormsExpected || !reflect.DeepEqual(parseRequiredTiers(level), parameterFormsRequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title, TestLevel: level, Status: "FAILED", Detail: "BRG11 producer differs from its immutable full scenario"}
		return true
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, parameterFormsPrefix+"-")
	}
	if err != nil {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return true
	}
	var command = productionTestCommand(parameterFormsTests)
	var budget = productionTestTimeout(parameterFormsTests) + time.Minute
	var receipt, artifacts, runErr = runExactSourceProcess(root, r.implSHA, c.ID, parameterFormsPrefix, directory, command, budget, parameterFormsRequiredEvents(), parameterFormsProductionPaths)
	if len(artifacts) == 0 {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: "recovery_gate", Requirement: c.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return true
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, parameterFormsPrefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseFunctionScenarioOutput(parameterFormsRequiredEvents(), budget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	var detail = parameterFormsScope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(c.ID, "recovery_gate", c.Title, level, functionTransportTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = parameterFormsProducer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(parameterFormsTests, "\n")
		record.Assertions["required_go_events"] = strings.Join(parameterFormsRequiredEvents(), "\n")
		record.Assertions["scope"] = parameterFormsScope
	}
	return true
}

func validateParameterFormsRecord(id string, record EvidenceRecord) error {
	if id != "BRG11" {
		return nil
	}
	var producer = parameterFormsProducer(record.Tier)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(parameterFormsTests)) || record.Assertions["required_production_tests"] != strings.Join(parameterFormsTests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(parameterFormsRequiredEvents(), "\n") || record.Assertions["scope"] != parameterFormsScope {
		return fmt.Errorf("BRG11 record is detached from its complete canonical parameter-forms scenario; actual Apple producer is still required")
	}
	var data, err = stateReadArtifacts(record.Artifacts, editorContractArtifacts(parameterFormsPrefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[parameterFormsPrefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[parameterFormsPrefix+"-go.jsonl"], data[parameterFormsPrefix+"-stderr.log"]
	var budget = productionTestTimeout(parameterFormsTests) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, parameterFormsRequiredEvents(), raw, stderr, budget, parameterFormsProductionPaths); err != nil {
		return err
	}
	if err = parseFunctionScenarioOutput(parameterFormsRequiredEvents(), budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
