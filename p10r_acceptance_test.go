package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func maskVolatileUUIDs(val any) any {
	switch v := val.(type) {
	case map[string]any:
		res := make(map[string]any, len(v))
		for k, valChild := range v {
			if k == "UUID" || k == "GroupingIdentifier" || k == "OutputUUID" {
				res[k] = "<MASKED_UUID>"
			} else {
				res[k] = maskVolatileUUIDs(valChild)
			}
		}
		return res
	case []any:
		res := make([]any, len(v))
		for i, item := range v {
			res[i] = maskVolatileUUIDs(item)
		}
		return res
	default:
		return v
	}
}

func assertActionsStructurallyEqual(t *testing.T, expectedDoc, actualDoc map[string]any, ignoredKeys ...string) {
	t.Helper()
	expActions, ok1 := expectedDoc["WFWorkflowActions"].([]any)
	actActions, ok2 := actualDoc["WFWorkflowActions"].([]any)
	if !ok1 || !ok2 {
		t.Fatalf("missing WFWorkflowActions in expected or actual doc")
	}
	if len(expActions) != len(actActions) {
		t.Fatalf("action count mismatch: expected %d, got %d", len(expActions), len(actActions))
	}

	ignoreMap := make(map[string]bool)
	for _, k := range ignoredKeys {
		ignoreMap[k] = true
	}

	for i := range expActions {
		expMap := expActions[i].(map[string]any)
		actMap := actActions[i].(map[string]any)

		if expMap["WFWorkflowActionIdentifier"] != actMap["WFWorkflowActionIdentifier"] {
			t.Fatalf("action %d identifier mismatch: expected %s, got %s", i, expMap["WFWorkflowActionIdentifier"], actMap["WFWorkflowActionIdentifier"])
		}

		expParams := expMap["WFWorkflowActionParameters"].(map[string]any)
		actParams := actMap["WFWorkflowActionParameters"].(map[string]any)

		for k, vExp := range expParams {
			if ignoreMap[k] || k == "UUID" || k == "GroupingIdentifier" {
				continue
			}
			vAct, found := actParams[k]
			if !found {
				t.Fatalf("action %d parameter %q missing in actual", i, k)
			}
			expJson, _ := json.Marshal(maskVolatileUUIDs(vExp))
			actJson, _ := json.Marshal(maskVolatileUUIDs(vAct))
			if string(expJson) != string(actJson) {
				t.Fatalf("action %d parameter %q mismatch:\nexpected: %s\nactual:   %s", i, k, expJson, actJson)
			}
		}
	}
}

// R08: Native compound AND/OR filter produces correct structural plist, decompile-recompile structural equality,
// and fails closed on invalid nested compound filters (which are unsupported by Apple Shortcuts WFContentItemFilter).
func TestP10R_R08_FilterTwoLevelAndOrRoundTrip(t *testing.T) {
	r := newRoundTripRunner(t)

	// Real compilation of Cherri source with flat compound filter
	src := `let input_files = "files"
let res = filterFiles(input_files, filter: { "compound": "or", "predicates": [ { "property": "File Size", "operator": "greater than", "value": 1024 }, { "property": "File Extension", "operator": "equals", "value": "png" } ] })
show(res)
`
	plistPath := r.compileSource("r08_filter_nested", src)
	doc := loadRoundTripDocument(t, plistPath)
	actionsList, ok := doc["WFWorkflowActions"].([]any)
	if !ok || len(actionsList) == 0 {
		t.Fatalf("expected actions in compiled plist")
	}

	var filterAct map[string]any
	for _, a := range actionsList {
		actMap := a.(map[string]any)
		if actMap["WFWorkflowActionIdentifier"] == "is.workflow.actions.filter.files" {
			filterAct = actMap
			break
		}
	}
	if filterAct == nil {
		t.Fatalf("expected is.workflow.actions.filter.files in compiled actions")
	}
	params := filterAct["WFWorkflowActionParameters"].(map[string]any)
	if params["WFCompoundType"] != "1" {
		t.Errorf("expected compound type '1' (OR), got %v", params["WFCompoundType"])
	}
	filt, ok := params["WFContentItemFilter"].(map[string]any)
	if !ok || filt["WFSerializationType"] != "WFContentPredicateTableTemplate" {
		t.Fatalf("expected WFContentPredicateTableTemplate, got %v", filt)
	}
	valMap := filt["Value"].(map[string]any)
	templates := valMap["WFActionParameterFilterTemplates"].([]any)
	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}
	t0 := templates[0].(map[string]any)
	if t0["Property"] != "File Size" || fmt.Sprint(t0["Operator"]) != "2" {
		t.Errorf("template 0 mismatch: %v", t0)
	}
	t1 := templates[1].(map[string]any)
	if t1["Property"] != "File Extension" || fmt.Sprint(t1["Operator"]) != "4" {
		t.Errorf("template 1 mismatch: %v", t1)
	}

	// Decompilation triggers lossless rawAction fallback
	decomp := r.decompileShortcut("r08_filter_nested", plistPath)
	t.Logf("DECOMP:\n%s", decomp)
	if !strings.Contains(decomp, "rawAction(\"is.workflow.actions.filter.files\"") {
		t.Errorf("expected rawAction fallback for unrepresentable filter table, got:\n%s", decomp)
	}

	// Recompile decompiled code and verify structural equality
	recompiledPlist := r.compileSource("r08_recompiled", decomp)
	recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
	assertActionsStructurallyEqual(t, doc, recompiledDoc, "WFContentItemInputParameter")

	// Adversarial: nested compound filter must fail closed
	nestedPanicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				nestedPanicked = true
			}
		}()
		buildContentItemFilter(map[string]any{
			"compound": "or",
			"predicates": []any{
				map[string]any{
					"compound": "and",
					"predicates": []any{
						map[string]any{"property": "Name", "operator": "equals", "value": "test"},
					},
				},
			},
		})
	}()
	if !nestedPanicked {
		t.Fatalf("expected nested compound filter to fail closed with panic, but succeeded")
	}
}

// R09: Filter operator parsing behaves deterministically and fails closed on invalid operators
func TestP10R_R09_FilterOperatorParsing(t *testing.T) {
	ops := map[string]int{
		"equals":                   4,
		"==":                       4,
		"is not":                   5,
		"!=":                       5,
		"contains":                 99,
		"does not contain":         999,
		"greater than":             2,
		"greater than or equal to": 3,
		"less than":                0,
		"less than or equal to":    1,
		"between":                  1003,
		"is today":                 1002,
	}

	for opStr, expectedCode := range ops {
		code := parseFilterOperator(opStr)
		if code != expectedCode {
			t.Errorf("expected operator %q -> %d, got %d", opStr, expectedCode, code)
		}
	}

	// Negative mutation 1: unknown operator MUST panic (never default to 4)
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for unknown operator 'nonsense', but returned without panic")
			}
			msg := fmt.Sprint(r)
			if !strings.Contains(msg, "unrecognized filter operator \"nonsense\"") {
				t.Errorf("expected diagnostic mentioning unrecognized operator, got: %s", msg)
			}
		}()
		parseFilterOperator("nonsense")
	}()

	// Negative mutation 2: invalid operator type MUST panic
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid operator type bool, but returned without panic")
			}
		}()
		parseFilterOperator(true)
	}()

	// Negative mutation 3: between operator with invalid bounds in buildContentItemFilter MUST panic
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for between operator with missing bounds, but returned without panic")
			}
		}()
		buildContentItemFilter(map[string]any{
			"predicates": []any{
				map[string]any{
					"property": "File Size",
					"operator": "between",
					"value":    []any{1024}, // only 1 bound!
				},
			},
		})
	}()
}

// R10: Limit absent; sort-only mode does NOT emit WFContentItemLimitEnabled
func TestP10R_R10_LimitAbsentSortOnly(t *testing.T) {
	r := newRoundTripRunner(t)

	// Case 1: sort-only, limit omitted
	srcSortOnly := `let input_files = "files"
let res = filterFiles(input_files, sortBy: "Name", orderBy: "A to Z")
show(res)
`
	plistSortOnly := r.compileSource("r10_sort_only", srcSortOnly)
	docSortOnly := loadRoundTripDocument(t, plistSortOnly)
	var filterAct1 map[string]any
	for _, a := range docSortOnly["WFWorkflowActions"].([]any) {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.filter.files" {
			filterAct1 = m
			break
		}
	}
	if filterAct1 == nil {
		t.Fatalf("filterFiles action missing from plistSortOnly")
	}
	p1 := filterAct1["WFWorkflowActionParameters"].(map[string]any)
	if p1["WFContentItemLimitEnabled"] == true {
		t.Errorf("WFContentItemLimitEnabled must NOT be true when limit is absent: %v", p1)
	}

	// Case 2: limit is provided
	srcWithLimit := `let input_files = "files"
let res = filterFiles(input_files, limit: 10, sortBy: "Name", orderBy: "A to Z")
show(res)
`
	plistWithLimit := r.compileSource("r10_with_limit", srcWithLimit)
	docWithLimit := loadRoundTripDocument(t, plistWithLimit)
	var filterAct2 map[string]any
	for _, a := range docWithLimit["WFWorkflowActions"].([]any) {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.filter.files" {
			filterAct2 = m
			break
		}
	}
	if filterAct2 == nil {
		t.Fatalf("filterFiles action missing from plistWithLimit")
	}
	p2 := filterAct2["WFWorkflowActionParameters"].(map[string]any)
	if p2["WFContentItemLimitEnabled"] != true {
		t.Errorf("expected WFContentItemLimitEnabled: true when limit is provided: %v", p2)
	}
}

// R11: editReminder invalid detail name panics under BOTH Set and Remove modes, never defaults to Title
func TestP10R_R11_EditReminderInvalidDetail(t *testing.T) {
	def := actions["editReminder"]
	if def == nil {
		t.Fatalf("editReminder action definition not found")
	}

	// 1. Invalid detail under Set mode
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid detail property under Set, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid detail property \"InvalidProperty\"") {
				t.Errorf("expected diagnostic mentioning InvalidProperty, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myReminder"},
			{valueType: String, value: "InvalidProperty"},
			{valueType: String, value: "New Value"},
		}
		def.appendParamsFunc(args)
	}()

	// 2. Invalid detail under Remove mode MUST FAIL with the exact same diagnostic
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid detail property under Remove, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid detail property \"InvalidProperty\"") {
				t.Errorf("expected diagnostic mentioning InvalidProperty, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myReminder"},
			{valueType: String, value: "InvalidProperty"},
			{valueType: Nil, value: nil},
			{valueType: String, value: "Remove"},
		}
		def.appendParamsFunc(args)
	}()

	// 3. Invalid mode MUST FAIL
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid mode, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid setter mode \"NonsenseMode\"") {
				t.Errorf("expected diagnostic mentioning invalid setter mode, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myReminder"},
			{valueType: String, value: "Title"},
			{valueType: String, value: "New Title"},
			{valueType: String, value: "NonsenseMode"},
		}
		def.appendParamsFunc(args)
	}()
}

// R12: editEvent invalid detail name panics under BOTH Set and Remove modes, never defaults to Start Date
func TestP10R_R12_EditEventInvalidDetail(t *testing.T) {
	def := actions["editEvent"]
	if def == nil {
		t.Fatalf("editEvent action definition not found")
	}

	// 1. Invalid detail under Set mode
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid detail property under Set, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid detail property \"NonExistentField\"") {
				t.Errorf("expected diagnostic mentioning NonExistentField, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myEvent"},
			{valueType: String, value: "NonExistentField"},
			{valueType: String, value: "New Date"},
		}
		def.appendParamsFunc(args)
	}()

	// 2. Invalid detail under Remove mode MUST FAIL with the exact same diagnostic
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid detail property under Remove, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid detail property \"NonExistentField\"") {
				t.Errorf("expected diagnostic mentioning NonExistentField, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myEvent"},
			{valueType: String, value: "NonExistentField"},
			{valueType: Nil, value: nil},
			{valueType: String, value: "Remove"},
		}
		def.appendParamsFunc(args)
	}()

	// 3. Invalid mode MUST FAIL
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected panic for invalid mode, but succeeded")
			}
			errMsg := fmt.Sprint(r)
			if !strings.Contains(errMsg, "invalid setter mode \"Replace\"") {
				t.Errorf("expected diagnostic mentioning invalid setter mode, got %q", errMsg)
			}
		}()
		args := []actionArgument{
			{valueType: Variable, value: "myEvent"},
			{valueType: String, value: "Title"},
			{valueType: String, value: "New Title"},
			{valueType: String, value: "Replace"},
		}
		def.appendParamsFunc(args)
	}()
}

// R13: Setter modes (Set vs Remove) produce distinct wire shapes and correct representability
func TestP10R_R13_SetterModesAndDecompile(t *testing.T) {
	def := actions["editReminder"]
	if def == nil {
		t.Fatalf("editReminder action definition not found")
	}

	// 1. Set mode
	argsSet := []actionArgument{
		{valueType: Variable, value: "rem"},
		{valueType: String, value: "Notes"},
		{valueType: String, value: "Updated note"},
		{valueType: String, value: "Set"},
	}
	pSet := def.appendParamsFunc(argsSet)
	if pSet["Mode"] != "Set" || pSet["ValueLabel"] != true || pSet["WFReminderContentItemNotes"] != "Updated note" || pSet["WFContentItemPropertyName"] != "Notes" {
		t.Errorf("unexpected Set mode parameters: %v", pSet)
	}

	// 2. Remove mode
	argsRemove := []actionArgument{
		{valueType: Variable, value: "rem"},
		{valueType: String, value: "Notes"},
		{valueType: Nil, value: nil},
		{valueType: String, value: "Remove"},
	}
	pRemove := def.appendParamsFunc(argsRemove)
	if pRemove["Mode"] != "Remove" || pRemove["ValueLabel"] != nil || pRemove["WFContentItemPropertyName"] != "Notes" {
		t.Errorf("unexpected Remove mode parameters: %v", pRemove)
	}

	// 3. Remove with RemoveSpecifiedValue
	argsRemoveSpecific := []actionArgument{
		{valueType: Variable, value: "rem"},
		{valueType: String, value: "Tags"},
		{valueType: String, value: "oldTag"},
		{valueType: String, value: "Remove"},
	}
	pRemoveSpecific := def.appendParamsFunc(argsRemoveSpecific)
	if pRemoveSpecific["Mode"] != "Remove" || pRemoveSpecific["RemoveSpecifiedValue"] != "oldTag" {
		t.Errorf("unexpected RemoveSpecifiedValue: %v", pRemoveSpecific)
	}

	// 4. Decompiler representability: valid setter actions are representable
	actionSet := &ShortcutAction{
		WFWorkflowActionIdentifier: "is.workflow.actions.setters.reminders",
		WFWorkflowActionParameters: map[string]any{
			"WFInput":                    "rem",
			"WFContentItemPropertyName":  "Notes",
			"Mode":                       "Set",
			"ValueLabel":                 true,
			"WFReminderContentItemNotes": "Updated note",
		},
	}
	if !isActionRepresentable(actionSet, def) {
		t.Errorf("expected actionSet to be representable")
	}

	// 5. Decompiler rejects invalid Mode
	actionInvalidMode := &ShortcutAction{
		WFWorkflowActionIdentifier: "is.workflow.actions.setters.reminders",
		WFWorkflowActionParameters: map[string]any{
			"WFInput":                   "rem",
			"WFContentItemPropertyName": "Notes",
			"Mode":                      "InvalidMode",
		},
	}
	if isActionRepresentable(actionInvalidMode, def) {
		t.Errorf("expected actionInvalidMode to be unrepresentable, falling back to rawAction")
	}
}

// R17: Changing EvidenceStatus on action or parameter alters Registry fingerprint
func TestP10R_R17_FingerprintDriftOnEvidenceStatus(t *testing.T) {
	baseAction := &schema.ActionSchema{
		ID:              "testAct",
		CallableName:    "testAct",
		AppleIdentifier: "is.workflow.actions.test",
		EvidenceStatus:  schema.EvidenceInferred,
		Parameters: []schema.ParameterSchema{
			{
				ID:             "p1",
				Label:          "p1",
				TypeName:       "Text",
				EvidenceStatus: schema.EvidenceInferred,
			},
		},
	}

	reg1 := schema.NewRegistry([]*schema.ActionSchema{baseAction}, nil)
	fp1 := reg1.Fingerprint()

	// Mutate action evidence status
	mutatedAction := &schema.ActionSchema{
		ID:              "testAct",
		CallableName:    "testAct",
		AppleIdentifier: "is.workflow.actions.test",
		EvidenceStatus:  schema.EvidenceConfirmed,
		Parameters: []schema.ParameterSchema{
			{
				ID:             "p1",
				Label:          "p1",
				TypeName:       "Text",
				EvidenceStatus: schema.EvidenceInferred,
			},
		},
	}
	reg2 := schema.NewRegistry([]*schema.ActionSchema{mutatedAction}, nil)
	fp2 := reg2.Fingerprint()

	if fp1 == fp2 {
		t.Fatalf("expected fingerprint to drift when Action.EvidenceStatus changes, but both are %q", fp1)
	}

	// Mutate parameter evidence status
	mutatedParam := &schema.ActionSchema{
		ID:              "testAct",
		CallableName:    "testAct",
		AppleIdentifier: "is.workflow.actions.test",
		EvidenceStatus:  schema.EvidenceInferred,
		Parameters: []schema.ParameterSchema{
			{
				ID:             "p1",
				Label:          "p1",
				TypeName:       "Text",
				EvidenceStatus: schema.EvidenceConfirmed,
			},
		},
	}
	reg3 := schema.NewRegistry([]*schema.ActionSchema{mutatedParam}, nil)
	fp3 := reg3.Fingerprint()

	if fp1 == fp3 {
		t.Fatalf("expected fingerprint to drift when Parameter.EvidenceStatus changes, but both are %q", fp1)
	}

	// Deterministic identity under unchanged inputs
	reg1Copy := schema.NewRegistry([]*schema.ActionSchema{baseAction}, nil)
	if reg1.Fingerprint() != reg1Copy.Fingerprint() {
		t.Fatalf("fingerprint must be deterministic under unchanged inputs")
	}
}

// R18: Semantic version comparison correctly orders 27.10 > 27.2 and checks action availability in compiler
func TestP10R_R18_SemanticVersionComparison(t *testing.T) {
	// Unit comparison
	if schema.CompareVersions("27.10", "27.2") <= 0 {
		t.Errorf("expected 27.10 > 27.2")
	}
	if schema.CompareVersions("27.2", "27.10") >= 0 {
		t.Errorf("expected 27.2 < 27.10")
	}
	if schema.CompareVersions("16.4.1", "16.4") <= 0 {
		t.Errorf("expected 16.4.1 > 16.4")
	}

	// Real compiler test: Action availability based on version string
	testActID := "versionTestFutureAction"
	actions[testActID] = &actionDefinition{
		identifier:       "is.workflow.actions.futureTest",
		minVersionString: "27.10",
	}
	defer delete(actions, testActID)

	// 1. Under #version 27.2 -> checkActionVersion MUST trigger parserError
	embeddedCompilerMode = true
	defer func() { embeddedCompilerMode = false }()

	iosVersionString = "27.2"
	iosVersion = 27.2
	currentAction = actionReference{
		identifier: testActID,
		definition: *actions[testActID],
	}
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatalf("expected parserError for 27.10 action under 27.2 version, but succeeded")
			}
			msg := fmt.Sprint(r)
			if !strings.Contains(msg, "not available in set minimum version '27.2'") {
				t.Errorf("expected diagnostic mentioning version 27.2, got: %s", msg)
			}
		}()
		checkActionVersion()
	}()

	// 2. Under #version 27.10 -> checkActionVersion MUST succeed without panic
	iosVersionString = "27.10"
	iosVersion = 27.10
	checkActionVersion()

	// Reset state
	resetCompilerState()
}

// R20: Known wire key with unsupported nested value in decompiler triggers lossless rawAction fallback
func TestP10R_R20_UnrepresentableValueRawActionFallback(t *testing.T) {
	r := newRoundTripRunner(t)

	// Compile a rawAction containing unrepresentable custom serialization
	src := `rawAction("is.workflow.actions.filter.files", {
    "WFContentItemInputParameter": "files",
    "WFContentItemFilter": {
        "WFSerializationType": "WFUnknownCustomSerializedType",
        "CustomData": "payload_123"
    }
})
`
	plistPath := r.compileSource("r20_unrepresentable", src)
	doc := loadRoundTripDocument(t, plistPath)
	actionsList := doc["WFWorkflowActions"].([]any)
	if len(actionsList) == 0 {
		t.Fatalf("expected actions in plist")
	}

	// Decompile: must fall back to rawAction
	decomp := r.decompileShortcut("r20_unrepresentable", plistPath)
	if !strings.Contains(decomp, "rawAction(\"is.workflow.actions.filter.files\"") {
		t.Errorf("expected rawAction fallback in decompiled code, got:\n%s", decomp)
	}
	if !strings.Contains(decomp, "WFUnknownCustomSerializedType") {
		t.Errorf("decompiled rawAction lost custom serialization type:\n%s", decomp)
	}

	// Recompile: succeeds structurally and matches original actions
	recompiledPlist := r.compileSource("r20_recompiled", decomp)
	recompiledDoc := loadRoundTripDocument(t, recompiledPlist)
	assertActionsStructurallyEqual(t, doc, recompiledDoc)
}

// R21: Real token-string interpolation encode/decode with Hebrew + emoji measures UTF-16 code units
func TestP10R_R21_HebrewEmojiUtf16Tokens(t *testing.T) {
	r := newRoundTripRunner(t)

	// In UTF-16 code units:
	// "שלום " = 5 code units ('ש', 'ל', 'ו', 'ם', ' ')
	src := `let name = "ישראל 🍎"
show("שלום {name}!")
`
	plistPath := r.compileSource("r21_hebrew_tokens", src)
	doc := loadRoundTripDocument(t, plistPath)
	actionsList := doc["WFWorkflowActions"].([]any)
	if len(actionsList) < 2 {
		t.Fatalf("expected at least 2 actions in plist, got %d", len(actionsList))
	}

	// Find the show action
	var showAct map[string]any
	for _, a := range actionsList {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.showresult" {
			showAct = m
			break
		}
	}
	if showAct == nil {
		t.Fatalf("is.workflow.actions.showresult action not found in plist")
	}

	params := showAct["WFWorkflowActionParameters"].(map[string]any)
	textVal, ok := params["Text"].(map[string]any)
	if !ok {
		t.Fatalf("Text parameter is not a dictionary: %v", params["Text"])
	}

	if textVal["WFSerializationType"] != "WFTextTokenString" {
		t.Errorf("expected WFSerializationType WFTextTokenString, got %v", textVal["WFSerializationType"])
	}
	valObj := textVal["Value"].(map[string]any)
	strVal := valObj["string"].(string)
	if !strings.Contains(strVal, "שלום") {
		t.Errorf("expected string to contain Hebrew text, got %q", strVal)
	}

	ranges := valObj["attachmentsByRange"].(map[string]any)
	if len(ranges) == 0 {
		t.Fatalf("attachmentsByRange is empty")
	}

	// Exact UTF-16 code units offset verification:
	// "שלום " in UTF-16 has 5 code units ('ש', 'ל', 'ו', 'ם', ' '), attachment is 1 unit.
	// Exact range key MUST BE "{5, 1}"
	if _, ok := ranges["{5, 1}"]; !ok {
		t.Fatalf("expected exact attachment range key '{5, 1}' for Hebrew prefix 'שלום ', got: %v", ranges)
	}

	// Case 2: Emoji surrogate pair verification
	srcEmoji := `let fruit = "תפוח"
show("🍎 {fruit}!")
`
	plistPathEmoji := r.compileSource("r21_emoji_tokens", srcEmoji)
	docEmoji := loadRoundTripDocument(t, plistPathEmoji)
	var showActEmoji map[string]any
	for _, a := range docEmoji["WFWorkflowActions"].([]any) {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.showresult" {
			showActEmoji = m
			break
		}
	}
	if showActEmoji == nil {
		t.Fatalf("show action missing for emoji token test")
	}
	paramsEmoji := showActEmoji["WFWorkflowActionParameters"].(map[string]any)
	textValEmoji := paramsEmoji["Text"].(map[string]any)["Value"].(map[string]any)
	rangesEmoji := textValEmoji["attachmentsByRange"].(map[string]any)
	// 🍎 is a surrogate pair (2 UTF-16 units) + ' ' (1 unit) = 3 code units.
	// Exact range key MUST BE "{3, 1}"
	if _, ok := rangesEmoji["{3, 1}"]; !ok {
		t.Fatalf("expected exact attachment range key '{3, 1}' for emoji prefix '🍎 ', got: %v", rangesEmoji)
	}
}

// R25: Action catalog and consumer export contract (STATIC_CONTRACT tier)
func TestP10R_R25_ActionCatalogAndGoBridgeExport(t *testing.T) {
	reg := schema.DefaultRegistry()
	if reg == nil {
		t.Fatalf("DefaultRegistry returned nil")
	}

	scan, ok := reg.LookupAction("scanBarcode")
	if !ok {
		t.Fatalf("scanBarcode action must be registered in DefaultRegistry")
	}

	if scan.AppleIdentifier != "is.workflow.actions.scanbarcode" {
		t.Errorf("expected AppleIdentifier is.workflow.actions.scanbarcode, got %q", scan.AppleIdentifier)
	}

	if len(scan.Parameters) != 2 {
		t.Errorf("expected 2 parameters for scanBarcode, got %d", len(scan.Parameters))
	}

	// Full compiler end-to-end compilation of scanBarcode
	r := newRoundTripRunner(t)
	src := `let b = scanBarcode()
show(b)
`
	plist := r.compileSource("r25_scan_barcode", src)
	doc := loadRoundTripDocument(t, plist)
	actionsList := doc["WFWorkflowActions"].([]any)
	foundScan := false
	for _, a := range actionsList {
		m := a.(map[string]any)
		if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.scanbarcode" {
			foundScan = true
			break
		}
	}
	if !foundScan {
		t.Fatalf("R25: compiled plist missing is.workflow.actions.scanbarcode")
	}

	t.Log("R25 contract verified across Schema DefaultRegistry, compiler end-to-end, and action catalog [STATIC_CONTRACT]")
}

// R26: Provenance registry enforces valid immutable commit SHAs in docs/fork-action-provenance.json
func TestP10R_R26_ProvenanceRegistryIntegrity(t *testing.T) {
	data, err := os.ReadFile("docs/fork-action-provenance.json")
	if err != nil {
		t.Fatalf("failed to read docs/fork-action-provenance.json: %v", err)
	}
	var prov map[string]any
	if err := json.Unmarshal(data, &prov); err != nil {
		t.Fatalf("failed to parse fork action provenance: %v", err)
	}

	actionsList, ok := prov["actions"].([]any)
	if !ok || len(actionsList) == 0 {
		t.Fatalf("provenance actions list is empty or missing")
	}

	// Deduplicate unique SHAs to verify quickly and reliably
	uniqueShas := make(map[string]string)
	for _, infoObj := range actionsList {
		info := infoObj.(map[string]any)
		actName := info["name"].(string)
		sha, ok := info["firstForkCommit"].(string)
		if !ok || len(sha) != 40 {
			t.Fatalf("action %q has invalid firstForkCommit: %v", actName, info["firstForkCommit"])
		}
		uniqueShas[sha] = actName
	}

	for sha, actName := range uniqueShas {
		cmd := exec.Command("git", "cat-file", "-e", sha+"^{commit}")
		if err := cmd.Run(); err != nil {
			t.Errorf("action %q references non-existent git commit %s: %v", actName, sha, err)
		}
		cmdAncestor := exec.Command("git", "merge-base", "--is-ancestor", sha, "HEAD")
		if err := cmdAncestor.Run(); err != nil {
			t.Errorf("action %q firstForkCommit %s is not an ancestor of HEAD: %v", actName, sha, err)
		}
	}

	// Adversarial check: verify that a forged / non-existent commit SHA is detected and rejected
	forgedSha := "0000000000000000000000000000000000000000"
	cmd := exec.Command("git", "cat-file", "-e", forgedSha+"^{commit}")
	if err := cmd.Run(); err == nil {
		t.Fatalf("adversarial check failed: forged SHA %s unexpectedly exists as commit!", forgedSha)
	}
	cmdAnc := exec.Command("git", "merge-base", "--is-ancestor", forgedSha, "HEAD")
	if err := cmdAnc.Run(); err == nil {
		t.Fatalf("adversarial check failed: forged SHA %s unexpectedly validated as ancestor!", forgedSha)
	}
}

// R28: Acceptance suite completeness: all 56 gates verified via executable runner
func TestP10R_R28_AcceptanceSuiteCompleteness(t *testing.T) {
	// Execute the authoritative Python gate runner in verify-registry mode
	cmd := exec.Command("python", "tools/p10r3/run_release_gates.py", "--verify-registry")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gate runner verify-registry failed:\n%s", out)
	}
	if !strings.Contains(string(out), "SUCCESS: All 56 required gates registered correctly") {
		t.Fatalf("expected confirmation of 56 registered gates, got:\n%s", out)
	}

	// Meta-verification / mutation test: running gate runner with --mutate-missing MUST fail with exit code 1
	mutCmd := exec.Command("python", "tools/p10r3/run_release_gates.py", "--mutate-missing", "R05")
	mutOut, mutErr := mutCmd.CombinedOutput()
	if mutErr == nil {
		t.Fatalf("meta-test failed: gate runner with missing gate unexpectedly passed:\n%s", mutOut)
	}
	if !strings.Contains(strings.ToLower(string(mutOut)), "missing required gate") {
		t.Fatalf("expected missing gate diagnostic in gate runner output, got:\n%s", mutOut)
	}
}
