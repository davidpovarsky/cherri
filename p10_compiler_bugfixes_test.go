package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/lower"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"howett.net/plist"
)

func compileV2Helper(t *testing.T, name, code string) (*CanonicalBackendSession, error) {
	t.Helper()
	file := source.NewFile(source.DocumentID(name), name, 1, code)
	p := syntax.NewParser(file)
	prog := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, fmt.Errorf("%v", p.Errors()[0])
	}

	reg := schema.DefaultRegistry()
	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, diag := range analyzer.Diagnostics() {
		if diag.Severity == analysis.SeverityError {
			return nil, fmt.Errorf("%s", diag.Message)
		}
	}

	session := NewCanonicalBackendSession()
	// Even an unexpected success in a negative test must release state on Fatal.
	t.Cleanup(session.Close)
	lowerer := lower.NewLowererWithSession(reg, session)
	lowerer.WorkflowName = name
	if err := lowerer.LowerProgramToSession(prog); err != nil {
		session.Close()
		return nil, err
	}
	return session, nil
}

// 1. Task 1219750185341254: run(server, payload) must retain WFInput
func TestP10_RunActionPreservesWFInput(t *testing.T) {
	v2Code := `let payload = "data"
run("General Context Server", input: payload)
`
	session, err := compileV2Helper(t, "test_run_v2", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var runAct *ShortcutAction
	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.runworkflow" {
			runAct = &act
			break
		}
	}
	if runAct == nil {
		t.Fatalf("expected runworkflow action not found")
	}

	params := runAct.WFWorkflowActionParameters
	if params["WFWorkflowName"] != "General Context Server" {
		t.Errorf("expected WFWorkflowName == 'General Context Server', got %v", params["WFWorkflowName"])
	}

	if params["WFInput"] == nil {
		t.Errorf("CRITICAL BUG REGRESSION: WFInput parameter is nil/missing in run action")
	}

	wfMap, ok := params["WFWorkflow"].(map[string]any)
	if !ok {
		t.Fatalf("expected WFWorkflow dictionary in run action parameters, got %T", params["WFWorkflow"])
	}
	if wfMap["workflowName"] != "General Context Server" {
		t.Errorf("expected WFWorkflow.workflowName == 'General Context Server', got %v", wfMap["workflowName"])
	}
}

// 2. Task 1219750250211915: statistic action must emit WFInput, not Input
func TestP10_StatisticActionEmitsWFInputNotInput(t *testing.T) {
	v2Code := `let nums = [1, 2, 3]
statistic("Sum", input: nums)
`
	session, err := compileV2Helper(t, "test_statistic_v2", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var statAct *ShortcutAction
	for _, act := range actions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.statistics" {
			statAct = &act
			break
		}
	}
	if statAct == nil {
		t.Fatalf("expected is.workflow.actions.statistics action not found")
	}

	params := statAct.WFWorkflowActionParameters
	if params["WFInput"] == nil {
		t.Errorf("CRITICAL: statistic action missing WFInput wire key; params: %v", params)
	}
	if params["Input"] != nil {
		t.Errorf("CRITICAL: statistic action incorrectly emitted 'Input' wire key instead of 'WFInput'")
	}
	if params["WFStatisticsOperation"] != "Sum" {
		t.Errorf("expected WFStatisticsOperation == 'Sum', got %v", params["WFStatisticsOperation"])
	}
}

// 3. Task 1219750434313969 (B): if matches == [] array comparison must fail closed
func TestP10_ArrayEqualityComparison_FailsClosed(t *testing.T) {
	v2Code := `let matches = ["apple", "banana"]
if matches == [] {
    alert(alert: "Empty")
}
`
	_, err := compileV2Helper(t, "test_array_cond", v2Code)
	if err == nil {
		t.Fatalf("expected compilation to fail for array == [] comparison, but succeeded")
	}
	if !strings.Contains(err.Error(), "Invalid type 'array' for conditional '=='") {
		t.Fatalf("expected diagnostic 'Invalid type 'array' for conditional '=='' but got: %v", err)
	}
}

// 4. Task 1219750434313969 (C): regex quantifiers {8}, {4}, {12} must not turn into variable tokens
func TestP10_RegexQuantifiersPreserved(t *testing.T) {
	v2Code := `let pattern = "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}"
matchText("[0-9a-fA-F]{8}-[0-9a-fA-F]{4}", text: pattern)
`
	session, err := compileV2Helper(t, "test_regex", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	actions := session.shortcut.WFWorkflowActions
	var textAct *ShortcutAction
	var matchAct *ShortcutAction
	for i := range actions {
		act := &actions[i]
		switch act.WFWorkflowActionIdentifier {
		case "is.workflow.actions.gettext":
			textAct = act
		case "is.workflow.actions.text.match":
			matchAct = act
		}
	}
	if textAct == nil {
		t.Fatalf("expected gettext action not found")
	}
	if matchAct == nil {
		t.Fatalf("expected text.match action not found")
	}

	assertUnadulteratedRegex := func(label string, rawVal any) {
		switch v := rawVal.(type) {
		case string:
			if v != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
				t.Fatalf("CRITICAL BUG (%s): regex pattern string corrupted: got %q", label, v)
			}
		case map[string]any:
			if ser, hasSer := v["WFSerializationType"]; hasSer && ser == "WFTextTokenString" {
				val := v["Value"].(map[string]any)
				if s, ok := val["string"].(string); ok && s != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
					t.Fatalf("CRITICAL BUG (%s): regex WFTextTokenString corrupted: got %q", label, s)
				}
				if atts, hasAtts := val["attachmentsByRange"].(map[string]any); hasAtts && len(atts) > 0 {
					t.Fatalf("CRITICAL BUG (%s): regex quantifier turned into variable attachments: %v", label, atts)
				}
			}
		case WFTextTokenString:
			if v.Value.String != "[0-9a-fA-F]{8}-[0-9a-fA-F]{4}" {
				t.Fatalf("CRITICAL BUG (%s): regex WFTextTokenString struct corrupted: got %q", label, v.Value.String)
			}
			if len(v.Value.AttachmentsByRange) > 0 {
				t.Fatalf("CRITICAL BUG (%s): regex quantifier turned into variable attachments: %v", label, v.Value.AttachmentsByRange)
			}
		default:
			t.Fatalf("unexpected type for %s: %T (%v)", label, rawVal, rawVal)
		}
	}

	assertUnadulteratedRegex("gettext.WFTextActionText", textAct.WFWorkflowActionParameters["WFTextActionText"])
	assertUnadulteratedRegex("matchText.WFMatchTextPattern", matchAct.WFWorkflowActionParameters["WFMatchTextPattern"])

	// Each public compiler owns the shared native state for its session.
	// Finish the V2 transaction before entering the separate Legacy compiler.
	session.Close()

	// V2 literal tokens now bypass Legacy interpolation by design. Exercise
	// the unchanged public Legacy compiler too, so the source mutation of
	// its inline-variable serializer still reaches an observable native wire.
	legacy := compileDebugFixture(t, "#include 'actions/text'\n@pattern = \"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}\"\nmatchText(\"[0-9a-fA-F]{8}-[0-9a-fA-F]{4}\", @pattern)\n", true)
	matches := 0
	for _, action := range legacy {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.text.match" {
			matches++
			assertUnadulteratedRegex("legacy.matchText.WFMatchTextPattern", action.WFWorkflowActionParameters["WFMatchTextPattern"])
		}
	}
	if matches != 1 {
		t.Fatalf("expected one actual Legacy regex consumer, got %d", matches)
	}
}

// 5. Task 1219800392994574: JSON braces in string must not be parsed as inline variables
func TestP10_JSONBracesInStringNotParsedAsInlineVars(t *testing.T) {
	resetParser()
	loadStandardActions()

	// Double quotes with JSON braces
	strWithJSON := `{"model":"test"}`
	checkInlineVars(&strWithJSON) // Must NOT throw parserError

	emptyJSON := `{}`
	checkInlineVars(&emptyJSON) // Must NOT throw parserError

	spacedJSON := `{ }`
	checkInlineVars(&spacedJSON) // Must NOT throw parserError
}

// 6. Task 1219750434313969 (A): Conditional getValue(job, "server") string comparison
// keeps native String coercion for the five dynamically typed lookups. The
// explicitly typed .text comparison uses N03's exact Text producer and its
// authored Boolean guard; numeric and missing-value cases retain their routes.
func TestP10_ConditionalGetValueStringComparison_EmitsStringCoercionAndStringTarget(t *testing.T) {
	v2Code := `let job = {"server": "General Context Server", "retries": 3}
let srv = getValue(job, key: "server")
let expectedVar = "General Context Server"

// Case 1: getValue variable compared to string literal
if srv == "General Context Server" {
	alert(alert: "Matched getValue literal")
}

// Case 2: inline getValue call compared to string literal
if getValue(job, key: "server") == "General Context Server" {
	alert(alert: "Matched inline getValue")
}

// Case 3: dictionary bracket lookup compared to string literal
if job["server"] == "General Context Server" {
	alert(alert: "Matched bracket lookup")
}

// Case 4: dictionary dot property lookup compared to string literal
if job.server == "General Context Server" {
	alert(alert: "Matched dot lookup")
}

// Case 5: getValue variable compared to string variable
if srv == expectedVar {
	alert(alert: "Matched string variable")
}

// Case 6: explicit .text coercion
if srv.text == "General Context Server" {
	alert(alert: "Matched explicit .text")
}

// Case 7: explicit .number coercion compared to number
let retriesVal = getValue(job, key: "retries")
if retriesVal.number == 3 {
	alert(alert: "Matched explicit .number")
}

// Case 8: missing key / empty check
let missingVal = getValue(job, key: "nonexistent")
if !missingVal {
	alert(alert: "Missing key is empty")
}
`
	session, err := compileV2Helper(t, "test_cond_getvalue_a3", v2Code)
	if err != nil {
		t.Fatalf("v2 compile failed: %v", err)
	}
	defer session.Close()

	// Inspect the actual serialization of this same compiled session. No
	// second compiler invocation is needed to validate the N03 native region.
	encoded, err := plist.Marshal(session.shortcut, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if _, err := plist.Unmarshal(encoded, &document); err != nil {
		t.Fatal(err)
	}
	var native []map[string]any
	for _, action := range document["WFWorkflowActions"].([]any) {
		native = append(native, action.(map[string]any))
	}
	if err := p10ConditionalExplicitTextError(native); err != nil {
		t.Fatal(err)
	}
	regions, err := p15N03ExactNativeRegions(native, 1)
	if err != nil {
		t.Fatal(err)
	}

	var condStarts []ShortcutAction
	for _, act := range session.shortcut.WFWorkflowActions {
		if act.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			if mode, ok := act.WFWorkflowActionParameters["WFControlFlowMode"].(int); ok && mode == 0 {
				condStarts = append(condStarts, act)
			}
		}
	}
	// Eight authored Ifs plus the single independently validated inner
	// exact-Text value comparison. The extra guard is not an authored case.
	if len(condStarts) != 8+len(regions) {
		t.Fatalf("expected eight authored Ifs and one connected exact-Text guard, got %d", len(condStarts))
	}

	assertHasCoercionClass := func(caseIdx int, act ShortcutAction, wantClass string) {
		t.Helper()
		wfInput, ok := act.WFWorkflowActionParameters["WFInput"].(map[string]any)
		if !ok {
			t.Fatalf("case %d: missing WFInput map: %T", caseIdx+1, act.WFWorkflowActionParameters["WFInput"])
		}
		varRaw := wfInput["Variable"]
		var aggrs []map[string]any
		switch v := varRaw.(type) {
		case WFTextTokenAttachment:
			for _, a := range v.Value.Aggrandizements {
				aggrs = append(aggrs, map[string]any{
					"Type":              a.Type,
					"CoercionItemClass": a.CoercionItemClass,
					"DictionaryKey":     a.DictionaryKey,
					"PropertyName":      a.PropertyName,
				})
			}
		case map[string]any:
			valMap, _ := v["Value"].(map[string]any)
			if rawList, ok := valMap["Aggrandizements"].([]any); ok {
				for _, item := range rawList {
					if m, isM := item.(map[string]any); isM {
						aggrs = append(aggrs, m)
					}
				}
			} else if rawMaps, ok := valMap["Aggrandizements"].([]map[string]any); ok {
				aggrs = append(aggrs, rawMaps...)
			}
		default:
			t.Fatalf("case %d: unexpected Variable type %T", caseIdx+1, varRaw)
		}

		foundCoercion := false
		for _, ag := range aggrs {
			if ag["Type"] == "WFCoercionVariableAggrandizement" && ag["CoercionItemClass"] == wantClass {
				foundCoercion = true
			}
			if caseIdx == 3 && ag["Type"] == "WFPropertyVariableAggrandizement" {
				t.Errorf("case 4 (job.server): emitted WFPropertyVariableAggrandizement instead of WFDictionaryValueVariableAggrandizement: %v", ag)
			}
		}
		if !foundCoercion {
			t.Errorf("case %d: WFInput.Variable missing WFCoercionVariableAggrandizement=%s; got aggrandizements=%v", caseIdx+1, wantClass, aggrs)
		}
	}

	// Cases 1..5 retain their dynamically typed native String comparison.
	// Case 6 is checked as the complete exact Text region above.
	for i := 0; i < 5; i++ {
		act := condStarts[i]
		params := act.WFWorkflowActionParameters
		if params["WFCondition"] != 4 {
			t.Errorf("case %d: expected WFCondition == 4, got %v", i+1, params["WFCondition"])
		}
		if params["WFConditionalActionString"] == nil {
			t.Errorf("case %d: expected WFConditionalActionString to be populated, got nil (params=%v)", i+1, params)
		}
		if params["WFNumberValue"] != nil {
			t.Errorf("case %d: CRITICAL BUG: WFNumberValue is populated (%v) on string comparison!", i+1, params["WFNumberValue"])
		}
		assertHasCoercionClass(i, act, "WFStringContentItem")
	}

	// Case 7: numeric comparison (WFCondition=4, WFNumberValue=3, WFConditionalActionString nil, WFNumberContentItem coercion)
	{
		act := condStarts[7]
		params := act.WFWorkflowActionParameters
		if params["WFNumberValue"] == nil {
			t.Errorf("case 7: expected WFNumberValue to be populated, got nil")
		}
		if params["WFConditionalActionString"] != nil {
			t.Errorf("case 7: unexpected WFConditionalActionString on numeric comparison: %v", params["WFConditionalActionString"])
		}
		assertHasCoercionClass(6, act, "WFNumberContentItem")
	}

	// Case 8: missing key / empty check (WFCondition=101)
	{
		act := condStarts[8]
		params := act.WFWorkflowActionParameters
		if params["WFCondition"] != 101 {
			t.Errorf("case 8: expected WFCondition == 101 (Does Not Have Any Value), got %v", params["WFCondition"])
		}
	}

	for _, fault := range []string{"wrong_text_source", "casefold_match", "wrong_match_count", "inverted_exact_operator", "detached_authored_if", "orphan_quote", "wrong_branch_body"} {
		t.Run(fault, func(t *testing.T) {
			bad := p15SCClone(t, native)
			if err := p10ConditionalExplicitTextError(bad); err != nil {
				t.Fatal("unchanged serialized clone rejected", err)
			}
			region := regions[0]
			switch fault {
			case "wrong_text_source":
				wrong := p15SCCopy(region.Left).(map[string]any)
				for _, action := range bad {
					if p15Param(action)["CustomOutputName"] == "job" {
						wrong["OutputUUID"] = p15Param(action)["UUID"]
					}
				}
				p15Param(bad[region.LeftSnapshot])["WFTextActionText"] = p15N03AffixedTokenString("~", wrong, "")
			case "casefold_match":
				p15Param(bad[region.Match])["WFMatchTextCaseSensitive"] = false
			case "wrong_match_count":
				p15Param(bad[region.Count])["Input"] = p15N03Attachment(region.Left.(map[string]any))
			case "inverted_exact_operator":
				p15Param(bad[region.InnerStart])["WFCondition"] = uint64(5)
			case "detached_authored_if":
				p15Param(bad[region.Next])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15N03Attachment(region.Left.(map[string]any))}
			case "orphan_quote":
				orphan := p15SCCopy(bad[region.Quote]).(map[string]any)
				p15Param(orphan)["UUID"] = "4fe62a08-dbd6-4f58-afd0-000000000099"
				bad = append(bad, orphan)
			case "wrong_branch_body":
				p15Param(bad[region.Next+1])["WFAlertActionMessage"] = "WRONG_EXPLICIT_TEXT_BRANCH"
			}
			if err := p15SCReferences(bad); err != nil {
				t.Fatal("mutation broke producer identity before its intended semantic gate", err)
			}
			if err := p10ConditionalExplicitTextError(bad); err == nil {
				t.Fatal("damaged exact Text route accepted")
			}
		})
	}
}

func p10ConditionalExplicitTextError(actions []map[string]any) error {
	if err := p15SCReferences(actions); err != nil {
		return err
	}
	if err := p15N03ExactControlGroupsError(actions); err != nil {
		return err
	}
	regions, err := p15N03ExactNativeRegions(actions, 1)
	if err != nil {
		return err
	}
	region := regions[0]
	job, srv := "", ""
	jobIndex, srvIndex := -1, -1
	for index, action := range actions {
		parameters := p15Param(action)
		switch parameters["CustomOutputName"] {
		case "job":
			if job != "" || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
				return fmt.Errorf("explicit Text fixture dictionary producer differs")
			}
			job, _ = parameters["UUID"].(string)
			jobIndex = index
		case "srv":
			if srv != "" || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvalueforkey" || parameters["WFDictionaryKey"] != "server" {
				return fmt.Errorf("explicit Text fixture lookup producer differs")
			}
			srv, _ = parameters["UUID"].(string)
			srvIndex = index
		}
	}
	if job == "" || srv == "" || jobIndex >= srvIndex || srvIndex >= region.Start {
		return fmt.Errorf("explicit Text fixture source producer order differs")
	}
	input, err := p15N03ExactToken(p15Param(actions[srvIndex])["WFInput"])
	if err != nil || input != job {
		return fmt.Errorf("explicit Text lookup does not read the authored dictionary")
	}
	left := map[string]any{"Type": "ActionOutput", "OutputUUID": srv, "OutputName": "srv",
		"Aggrandizements": []any{map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFStringContentItem"}}}
	if region.Operator != 4 || !reflect.DeepEqual(region.Left, left) || region.Right != "General Context Server" || len(region.Result) != 2 {
		return fmt.Errorf("explicit .text comparison lost its source/coercion/operator/literal binding")
	}
	group, err := p15N03ExactResultGuard(actions, region, region.Next)
	if err != nil {
		return err
	}
	if region.Next+2 >= len(actions) || actions[region.Next+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.alert" ||
		p15Param(actions[region.Next+1])["WFAlertActionMessage"] != "Matched explicit .text" {
		return fmt.Errorf("exact Text true branch lost its authored alert")
	}
	end := actions[region.Next+2]
	if end["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" ||
		!reflect.DeepEqual(p15Param(end), map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(2)}) {
		return fmt.Errorf("exact Text authored branch end differs")
	}
	return nil
}
