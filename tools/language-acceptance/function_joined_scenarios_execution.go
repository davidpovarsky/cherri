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

// RP26 and BRG15 run their own joined sources. Related requirement IDs and the
// historical runtime smoke are traceability only, never execution evidence.
type distinctFunctionScenario struct {
	ID, Group, Title, Scenario, Expected, Prefix, Scope string
	RequiredTiers, LocalTiers, Tests, Sources           []string
}

var distinctFunctionScenarios = map[string]distinctFunctionScenario{
	"RP26": {
		ID: "RP26", Group: "repair", Title: "Caller input and Void",
		Scenario:      "Pass user input through a function chain; use a dictionary resembling a partial dispatch marker; test bare return/fallthrough.",
		Expected:      "system.input remains user input; incomplete envelopes do not dispatch; Void does not leak incidental output.",
		Prefix:        "caller-input-void-rp26",
		Scope:         "RP26 owns a two-call Map input chain, both child Void forms and three separate main value/bare/fallthrough compiles. Whole native graphs bind caller-input restoration and transport, both ShortcutInput aliases, incidental-output clearing, continuation and each terminal. Seven fixed entry inputs are evaluated against the actual emitted two-marker activation predicate, not a replacement Shortcuts interpreter or general malformed-envelope validator. Only native structure is observed; actual user-input values, dispatch and Void output on Apple/iOS, all related IDs and final179 remain unproved.",
		RequiredTiers: []string{"native-structure", "ios-runtime"},
		LocalTiers:    []string{"native-structure"},
		Tests:         []string{"TestP15RP26CallerInputVoidNative", "TestP15RP26PartialMarkerInputBoundaries"},
		Sources:       []string{"p15_rp26_caller_input_void_test.go", "tests/language-v2/v2-rp26-caller-input-void.cherri", "tests/language-v2/rp26-partial-marker-inputs.json"},
	},
	"BRG15": {
		ID: "BRG15", Group: "recovery_gate", Title: "Function transport and recursion",
		Scenario:      "Nested and recursive calls with presence defaults, structured arguments/results, caller input and envelope-lookalike input; inject lowering failure.",
		Expected:      "Shared canonical transport, correct caller continuation, preserved shapes/presence/input, complete error propagation, no accidental dispatch.",
		Prefix:        "joined-transport-brg15",
		Scope:         "BRG15 owns one nested and recursive Map program with omitted, explicit empty and supplied structured inputs. Typed semantic observations and complete native graphs bind default presence, mixed nested scalar/empty shapes, recursive arguments/results, every caller-input payload, partial-marker user data and final consumers. Three distinct actual canonical-emitter failures preserve cause/context, emit zero partial bytes and permit a subsequent healthy compile. Only unit/native structure is observed; actual recursive values, dispatch, caller continuation and user input on Apple/iOS, all related IDs and final179 remain unproved.",
		RequiredTiers: []string{"unit", "native-structure", "ios-runtime"},
		LocalTiers:    []string{"unit", "native-structure"},
		Tests:         []string{"TestBackendRecovery15JoinedTransportUnit", "TestBackendRecovery15JoinedTransportNative", "TestBackendRecovery15LoweringFailurePropagation"},
		Sources:       []string{"p15_brg15_joined_transport_test.go", "tests/language-v2/v2-brg15-joined-transport.cherri"},
	},
}

func (s distinctFunctionScenario) testIDs() []string {
	var ids []string
	for _, name := range s.Tests {
		ids = append(ids, "github.com/electrikmilk/cherri:"+name)
	}
	return ids
}

func (s distinctFunctionScenario) producer(tier string) string {
	for index, local := range s.LocalTiers {
		if local == tier {
			return s.testIDs()[index]
		}
	}
	return ""
}

func distinctFunctionSourcePaths(id string) []string {
	var s, found = distinctFunctionScenarios[id]
	if !found {
		return nil
	}
	var paths = append([]string{}, functionTransportProductionPaths...)
	paths = append(paths,
		"p15_function_envelope_completeness_test.go",
		"tools/language-acceptance/function_joined_scenarios_execution.go",
		"tools/language-acceptance/function_joined_scenarios_execution_test.go",
	)
	return append(paths, s.Sources...)
}

func distinctFunctionRequiredEvents(id string) []string {
	var s, found = distinctFunctionScenarios[id]
	if !found {
		return nil
	}
	var names = append([]string{}, s.Tests...)
	var common = []string{"missing_action", "extra_action", "zero_producer_reconciled", "malformed_producer_reconciled", "duplicate_producer", "zero_group_reconciled", "malformed_group_reconciled", "real_control_code"}
	switch id {
	case "RP26":
		var faults = []string{
			"magic_guard", "marker_guard", "restore_input", "inspect_slot", "inspect_caller",
			"inner_payload_input", "outer_payload_input", "lookalike_marker", "self_target",
			"bare_clear", "bare_return", "fallthrough_clear", "fallthrough_return",
			"between_input", "after_input", "main_terminal",
		}
		for _, mode := range []string{"value", "bare", "fallthrough"} {
			var parent = s.Tests[0] + "/" + mode
			names = append(names, parent)
			for _, fault := range append(append([]string{}, common...), faults...) {
				names = append(names, parent+"/oracle/"+fault)
			}
		}
		for _, input := range []string{"empty_dictionary", "marker_only", "magic_only", "wrong_magic", "wrong_marker", "present_empty_magic", "valid_private_markers"} {
			names = append(names, s.Tests[1]+"/"+input)
		}
	case "BRG15":
		for _, negative := range []string{"wrong_recursive_argument", "wrong_default", "wrong_recursive_return"} {
			names = append(names, s.Tests[0]+"/"+negative)
		}
		var faults = []string{
			"magic_guard", "marker_guard", "recursive_slot", "base_predicate", "base_threshold",
			"base_result", "recursive_decrement", "recursive_argument", "recursive_caller",
			"recursive_callee", "recursive_self_target", "recursive_result",
			"presence_reads_value", "presence_truthiness", "default_branch", "default_false_kind",
			"default_empty_shape", "nested_depth", "nested_caller", "route_caller",
			"omitted_marked_present", "empty_marked_absent", "supplied_shape", "supplied_false_kind",
			"supplied_caller", "main_callee", "before_input", "after_input", "final_consumer", "main_return",
		}
		for _, fault := range append(append([]string{}, common...), faults...) {
			names = append(names, s.Tests[1]+"/oracle/"+fault)
		}
		for _, site := range []string{"recursive_body", "nested_body", "caller_body"} {
			names = append(names, s.Tests[2]+"/"+site)
		}
	}
	sort.Strings(names)
	return names
}

func (r *Runner) runCallerInputVoidRequirement(c RepairCaseSpec) bool {
	if c.ID != "RP26" {
		return false
	}
	var s = distinctFunctionScenarios[c.ID]
	var level = strings.Join(c.RequiredTestLevels, "+")
	if c.Title != s.Title || c.Scenario != s.Scenario || c.ExpectedObservation != s.Expected ||
		!reflect.DeepEqual(c.OriginalRequirementIDs, []string{"FN04", "M02"}) ||
		!reflect.DeepEqual(c.RequiredTestLevels, []string{"native", "iOS"}) ||
		!reflect.DeepEqual(parseRequiredTiers(level), s.RequiredTiers) {
		r.results[c.ID] = CaseResult{ID: c.ID, Group: s.Group, Requirement: c.Title, TestLevel: level, Status: "FAILED", Detail: "RP26 producer differs from its immutable distinct scenario"}
		return true
	}
	r.runDistinctFunctionScenario(s, level)
	return true
}

func (r *Runner) runJoinedFunctionTransportRequirement(g RecoveryGate) bool {
	if g.ID != "BRG15" {
		return false
	}
	var s = distinctFunctionScenarios[g.ID]
	var level = strings.Join(g.MinimumEvidenceTiers, "+")
	if g.Title != s.Title || g.Scenario != s.Scenario || g.ExpectedObservation != s.Expected ||
		!g.Mandatory || g.ProposedTestPrefix != "TestBackendRecovery15" ||
		!reflect.DeepEqual(g.RelatedOriginalIDs, []string{"FN01", "FN02", "FN03", "FN04", "FN05"}) ||
		!reflect.DeepEqual(g.RelatedRepairIDs, []string{"RP24", "RP25", "RP26"}) ||
		!reflect.DeepEqual(g.MinimumEvidenceTiers, s.RequiredTiers) ||
		!reflect.DeepEqual(parseRequiredTiers(level), s.RequiredTiers) {
		r.results[g.ID] = CaseResult{ID: g.ID, Group: s.Group, Requirement: g.Title, TestLevel: level, Status: "FAILED", Detail: "BRG15 producer differs from its immutable distinct scenario"}
		return true
	}
	r.runDistinctFunctionScenario(s, level)
	return true
}

func (r *Runner) runDistinctFunctionScenario(s distinctFunctionScenario, level string) {
	var root, err = filepath.Abs(resolveRepoRoot())
	var directory string
	if err == nil {
		directory, err = stateEvidenceDirectory(r, s.Prefix+"-")
	}
	if err != nil {
		r.results[s.ID] = CaseResult{ID: s.ID, Group: s.Group, Requirement: s.Title, TestLevel: level, Status: "NOT_RUN", Detail: err.Error()}
		return
	}
	var command = productionTestCommand(s.Tests)
	var budget = productionTestTimeout(s.Tests) + time.Minute
	var events = distinctFunctionRequiredEvents(s.ID)
	var receipt, artifacts, runErr = runExactSourceProcess(root, r.implSHA, s.ID, s.Prefix, directory, command, budget, events, distinctFunctionSourcePaths(s.ID))
	if len(artifacts) == 0 {
		r.results[s.ID] = CaseResult{ID: s.ID, Group: s.Group, Requirement: s.Title, TestLevel: level, Status: "NOT_RUN", Detail: fmt.Sprint(runErr)}
		return
	}
	var raw, readErr = os.ReadFile(filepath.Join(directory, s.Prefix+"-go.jsonl"))
	if runErr == nil {
		runErr = readErr
	}
	if runErr == nil {
		runErr = parseFunctionScenarioOutput(events, budget, raw)
	}
	if runErr == nil {
		runErr = loopOutputWithinProcess(raw, receipt)
	}
	var detail = s.Scope
	if runErr != nil {
		detail = runErr.Error()
	}
	var first = len(r.records)
	r.Record(s.ID, s.Group, s.Title, level, s.LocalTiers, runErr == nil, "PASSED", detail)
	for index := first; index < len(r.records); index++ {
		var record = &r.records[index]
		record.TestID = s.producer(record.Tier)
		record.Command = command
		record.Artifacts = append([]EvidenceArtifact(nil), artifacts...)
		record.Assertions["required_production_tests"] = strings.Join(s.Tests, "\n")
		record.Assertions["required_go_events"] = strings.Join(events, "\n")
		record.Assertions["scope"] = s.Scope
	}
}

func validateDistinctFunctionRecord(id string, record EvidenceRecord) error {
	var s, found = distinctFunctionScenarios[id]
	if !found {
		return nil
	}
	var producer = s.producer(record.Tier)
	var events = distinctFunctionRequiredEvents(id)
	if producer == "" || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != producer ||
		!reflect.DeepEqual(record.Command, productionTestCommand(s.Tests)) ||
		record.Assertions["required_production_tests"] != strings.Join(s.Tests, "\n") ||
		record.Assertions["required_go_events"] != strings.Join(events, "\n") ||
		record.Assertions["scope"] != s.Scope {
		return fmt.Errorf("%s record is detached from its distinct joined source and complete local production; native runtime remains required", id)
	}
	var data, err = stateReadArtifacts(record.Artifacts, editorContractArtifacts(s.Prefix))
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data[s.Prefix+"-process.json"], &receipt); err != nil {
		return err
	}
	var raw, stderr = data[s.Prefix+"-go.jsonl"], data[s.Prefix+"-stderr.log"]
	var budget = productionTestTimeout(s.Tests) + time.Minute
	if err = validateExactSourceProcess(id, record.ImplementationSHA, receipt, record.Command, events, raw, stderr, budget, distinctFunctionSourcePaths(id)); err != nil {
		return err
	}
	if err = parseFunctionScenarioOutput(events, budget, raw); err != nil {
		return err
	}
	return loopOutputWithinProcess(raw, receipt)
}
