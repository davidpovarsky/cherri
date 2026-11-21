package main

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func p15EnvelopeCall(t *testing.T, envelope map[string]any, legacy bool) string {
	t.Helper()
	record := NativeActionRecord{Identifier: envelope["WFWorkflowActionIdentifier"].(string), RawAction: envelope}
	var source string
	var err error
	if legacy {
		source, err = formatRawLegacyActionCall(record)
	} else {
		source, err = formatRawV2ActionCall(record)
	}
	if err != nil {
		t.Fatal(err)
	}
	return source
}

func p15NormalizedNativeActions(t *testing.T, actions []any) []any {
	t.Helper()
	root := baseTestHeader()
	root["WFWorkflowActions"] = actions
	raw := buildNativePlistBytes(t, root, plist.XMLFormat)
	doc, err := DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc.RawRoot["WFWorkflowActions"].([]any)
}

func p15OutputIdentities(value any) []string {
	var found []string
	var visit func(any)
	visit = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" {
				id, _ := v["OutputUUID"].(string)
				found = append(found, id)
			}
			for _, item := range v {
				visit(item)
			}
		case []any:
			for _, item := range v {
				visit(item)
			}
		}
	}
	visit(value)
	return found
}

func TestP15NativeActionEnvelopePreservesExactSource(t *testing.T) {
	for _, present := range []bool{false, true} {
		envelope := map[string]any{
			"WFWorkflowActionIdentifier": "com.example.p15.envelope",
			"P15Future": map[string]any{
				"ordered":   []any{false, int64(0), "", int64(9007199254740993)},
				"emptyList": []any{}, "emptyMap": map[string]any{},
				"bytes": []byte{0, 1, 254, 255},
				"date":  time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC),
			},
		}
		if present {
			envelope["WFWorkflowActionParameters"] = map[string]any{}
		}
		for _, legacy := range []bool{true, false} {
			t.Run(fmt.Sprintf("present=%t/legacy=%t", present, legacy), func(t *testing.T) {
				actual, err := compileHybridSource(p15EnvelopeCall(t, envelope, legacy)+"\n", legacy)
				if err != nil {
					t.Fatal(err)
				}
				wantRows := []any{envelope}
				if !legacy {
					wantRows = append(wantRows, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
				}
				want := p15NormalizedNativeActions(t, wantRows)
				got := actual["WFWorkflowActions"]
				if !plistValuesStrictlyEqual(want, got) {
					t.Fatalf("exact envelope changed at %s", firstPlistDiffPath(want, got, "actions"))
				}
				// JSON consumers must see the same unknown envelope fields as plist
				// consumers. Dates/binary deliberately use JSON's normal encoding.
				action := ShortcutAction{WFWorkflowActionIdentifier: envelope["WFWorkflowActionIdentifier"].(string), rawNativeEnvelope: envelope}
				if present {
					action.WFWorkflowActionParameters = map[string]any{}
				}
				encoded, err := json.Marshal(action)
				if err != nil {
					t.Fatal(err)
				}
				var decoded map[string]any
				if err := json.Unmarshal(encoded, &decoded); err != nil {
					t.Fatal(err)
				}
				if _, ok := decoded["P15Future"]; !ok {
					t.Fatal("JSON action output discarded the unknown envelope")
				}
				if _, ok := decoded["WFWorkflowActionParameters"]; ok != present {
					t.Fatal("JSON action output changed parameter presence")
				}
			})
		}
	}
}

func TestP15NativeActionEnvelopeOldParametersRemainData(t *testing.T) {
	params := map[string]any{
		"WFWorkflowActionIdentifier": "ordinary nested value",
		"WFWorkflowActionParameters": map[string]any{"envelope": false},
		"envelope":                   []any{"literal", int64(0)},
		"Value":                      "", "WFValue": map[string]any{}, "WFSerializationType": "ordinary field",
	}
	literal, err := encodePlistValueToLiteral(params)
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var source string
			if legacy {
				source = "rawAction(\"com.example.p15.old-params\", rawPlist(" + formatCherriDoubleQuotedString(literal) + "))\n"
			} else {
				source = "native.action(\"com.example.p15.old-params\", parameters: native.plist(" + formatCherriDoubleQuotedString(literal) + "))\n"
			}
			actual, err := compileHybridSource(source, legacy)
			if err != nil {
				t.Fatal(err)
			}
			wantRows := []any{p15AcceptanceAction("com.example.p15.old-params", params)}
			if !legacy {
				wantRows = append(wantRows, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			}
			expected := p15NormalizedNativeActions(t, wantRows)
			if !plistValuesStrictlyEqual(expected, actual["WFWorkflowActions"]) {
				t.Fatal("ordinary parameters were reinterpreted as an envelope")
			}
		})
	}
}

func TestP15NativeActionEnvelopeRejectsConflicts(t *testing.T) {
	const id = "com.example.p15.envelope"
	valid := map[string]any{"WFWorkflowActionIdentifier": id, "WFWorkflowActionParameters": map[string]any{}, "P15Future": "exact"}
	literal, err := encodePlistValueToLiteral(valid)
	if err != nil {
		t.Fatal(err)
	}
	quoted := formatCherriDoubleQuotedString(literal)
	v2 := "native.plist(" + quoted + ")"
	legacy := "rawPlist(" + quoted + ")"
	cases := []struct {
		name, source, code string
		legacy             bool
	}{
		{"v2-plain-envelope", "native.action(\"" + id + "\", envelope: {\"WFWorkflowActionIdentifier\":\"" + id + "\"})", "NATIVE_ACTION_ENVELOPE_EXACT_REQUIRED", false},
		{"v2-array-envelope", "native.action(\"" + id + "\", envelope: native.plist([]))", "NATIVE_ACTION_ENVELOPE_SHAPE", false},
		{"v2-wrong-identifier", "native.action(\"com.example.different\", envelope: " + v2 + ")", "NATIVE_ACTION_ENVELOPE_IDENTIFIER", false},
		{"v2-parameters-conflict", "native.action(\"" + id + "\", parameters: {}, envelope: " + v2 + ")", "NATIVE_ACTION_ENVELOPE_CONFLICT", false},
		{"v2-extra-key-conflict", "native.action(\"" + id + "\", P15Extra: false, envelope: " + v2 + ")", "NATIVE_ACTION_ENVELOPE_CONFLICT", false},
		{"v2-duplicate-envelope", "native.action(\"" + id + "\", envelope: " + v2 + ", envelope: " + v2 + ")", "NATIVE_ACTION_ENVELOPE_ARGUMENTS", false},
		{"v2-wrong-parameters-shape", "native.action(\"" + id + "\", envelope: native.plist({\"WFWorkflowActionIdentifier\":\"" + id + "\",\"WFWorkflowActionParameters\":[]}))", "NATIVE_ACTION_ENVELOPE_PARAMETERS", false},
		{"legacy-plain-envelope", "rawAction(\"" + id + "\", nil, {\"WFWorkflowActionIdentifier\":\"" + id + "\"})", "NATIVE_ACTION_ENVELOPE_EXACT_REQUIRED", true},
		{"legacy-wrong-identifier", "rawAction(\"com.example.different\", nil, " + legacy + ")", "NATIVE_ACTION_ENVELOPE_IDENTIFIER", true},
		{"legacy-parameters-conflict", "rawAction(\"" + id + "\", {}, " + legacy + ")", "NATIVE_ACTION_ENVELOPE_CONFLICT", true},
		{"legacy-bound-envelope", "const result = rawAction(\"" + id + "\", nil, " + legacy + ")", "NATIVE_ACTION_ENVELOPE_BINDING", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := compileHybridSource(tc.source+"\n", tc.legacy)
			if err == nil || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("expected %s, got %v", tc.code, err)
			}
			if actual != nil {
				t.Fatal("rejected envelope returned a native workflow")
			}
		})
	}
}

func TestP15NativeActionEnvelopeBeforeOverlayAndCompatibilityRenderers(t *testing.T) {
	root := p15UnknownNativeAcceptanceRoot()
	want := p15NormalizedNativeActions(t, root["WFWorkflowActions"].([]any))
	doc, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{true, false} {
		t.Run(fmt.Sprintf("legacy=%t", legacy), func(t *testing.T) {
			var sources []string
			if legacy {
				direct, err := RenderLosslessLegacySourceFromDocument(doc)
				if err != nil {
					t.Fatal(err)
				}
				compat, err := renderLosslessLegacySource(doc.RawRoot)
				if err != nil {
					t.Fatal(err)
				}
				sources = []string{direct, compat}
			} else {
				direct, err := RenderLosslessV2SourceFromDocument(doc)
				if err != nil {
					t.Fatal(err)
				}
				compat, err := renderLosslessV2Source(doc.RawRoot)
				if err != nil {
					t.Fatal(err)
				}
				sources = []string{direct, compat}
			}
			for index, source := range sources {
				var captured []any
				previous := PreOverlayCaptureHook
				PreOverlayCaptureHook = func(_ string, actions []any) {
					captured = deepClonePlistSlice(actions)
				}
				compiled, err := compileHybridSource(source, legacy)
				PreOverlayCaptureHook = previous
				if err != nil {
					t.Fatalf("renderer %d: %v", index, err)
				}
				if !plistValuesStrictlyEqual(want, captured) {
					t.Fatalf("renderer %d already lost the envelope before any overlay: %s", index, firstPlistDiffPath(want, captured, "actions"))
				}
				if err := p15NativePreservationError(doc.RawRoot, compiled); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}

func TestP15NativeActionEnvelopeFullWorkflowAndAuthoredEdit(t *testing.T) {
	root := p15UnknownNativeAcceptanceRoot()
	literal, err := encodePlistValueToLiteral(root)
	if err != nil {
		t.Fatal(err)
	}
	source := "native.workflow(native.plist(" + formatCherriDoubleQuotedString(literal) + "), exact: true)\n"
	actual, err := compileHybridSource(source, false)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	if err := p15NativePreservationError(doc.RawRoot, actual); err != nil {
		t.Fatal(err)
	}

	// A later ordinary action must not replace or rehydrate the earlier envelope.
	actual, err = compileHybridSource(source+"comment(\"P15_APPENDED\")\n", false)
	if err != nil {
		t.Fatal(err)
	}
	actions := actual["WFWorkflowActions"].([]any)
	expected := doc.RawRoot["WFWorkflowActions"].([]any)
	if len(actions) != len(expected)+1 || !plistValuesStrictlyEqual(expected, actions[:len(expected)]) {
		t.Fatal("appended action changed the native workflow prefix")
	}
	if actions[len(expected)].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.comment" {
		t.Fatal("appended source action disappeared")
	}

	// Rebuilding a source edit uses its new full envelope, not original input metadata.
	edited := deepClonePlistMap(expected[0].(map[string]any))
	edited["P15UnknownActionField"] = []any{"edited", false}
	for _, legacy := range []bool{true, false} {
		actual, err := compileHybridSource(p15EnvelopeCall(t, edited, legacy)+"\n", legacy)
		if err != nil {
			t.Fatal(err)
		}
		wantRows := []any{edited}
		if !legacy {
			wantRows = append(wantRows, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
		}
		if !plistValuesStrictlyEqual(p15NormalizedNativeActions(t, wantRows), actual["WFWorkflowActions"]) {
			t.Fatal("source-authored envelope edit was replaced by original import metadata")
		}
	}
}

func TestP15R05RawExportsAndInvalidBoundary(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	explicit := p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"UUID": id, "CustomOutputName": "Original native name", "WFTextActionText": "P15_EXPORT"})
	explicit["P15Future"] = map[string]any{"ordinary": false}
	missing := deepClonePlistMap(explicit)
	delete(missing["WFWorkflowActionParameters"].(map[string]any), "UUID")
	validCall := p15EnvelopeCall(t, explicit, false)
	missingCall := p15EnvelopeCall(t, missing, false)
	cases := []struct {
		name, source, code string
		references         int
	}{
		{"let-export", "let exported = " + validCall + "\noutput(exported)\n", "", 1},
		{"alias-export", "let exported = " + validCall + "\nlet alias = exported\noutput(alias)\n", "", 1},
		{"explicit-native-ref", validCall + "\nlet ref = native.ref(\"ActionOutput\", uuid: \"" + id + "\")\noutput(ref)\n", "", 1},
		{"unused-missing-identity", "let unused = " + missingCall + "\n", "", 0},
		{"consumed-missing-identity", "let exported = " + missingCall + "\noutput(exported)\n", "NATIVE_OUTPUT_ID_MISSING", 0},
		{"consumed-ambiguous-identity", "let exported = " + validCall + "\n" + validCall + "\noutput(exported)\n", "NATIVE_OUTPUT_AMBIGUOUS", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actual, err := compileHybridSource(tc.source, false)
			if tc.code != "" {
				if err == nil || !strings.Contains(err.Error(), tc.code) || actual != nil {
					t.Fatalf("invalid authored output boundary expected %s, got %v", tc.code, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			actions := actual["WFWorkflowActions"].([]any)
			want := explicit
			if tc.references == 0 {
				want = missing
			}
			if !plistValuesStrictlyEqual(p15NormalizedNativeActions(t, []any{want})[0], actions[0]) {
				t.Fatal("output binding renamed or mutated exact native envelope")
			}
			refs := p15OutputIdentities(actions)
			if len(refs) != tc.references {
				t.Fatalf("expected %d exact output references, got %v", tc.references, refs)
			}
			for _, reference := range refs {
				if reference != id {
					t.Fatalf("export points to invented native identity %q", reference)
				}
			}
		})
	}
	// Legacy exact envelopes deliberately retain their source UUID. An explicit
	// native reference is supported; assigning the exact envelope itself is diagnosed above.
	legacyConsumer := p15AcceptanceAction("is.workflow.actions.getvariable", map[string]any{"WFVariable": p15AcceptanceRef(id, nil)})
	legacyConsumer["P15Future"] = "explicit consumer"
	actual, err := compileHybridSource(p15EnvelopeCall(t, explicit, true)+"\n"+p15EnvelopeCall(t, legacyConsumer, true)+"\n", true)
	if err != nil {
		t.Fatal(err)
	}
	if !plistValuesStrictlyEqual(p15NormalizedNativeActions(t, []any{explicit, legacyConsumer}), actual["WFWorkflowActions"]) {
		t.Fatal("Legacy explicit native output reference changed")
	}
}

func TestP15R05OpaqueRegionsAndWorkflowFallback(t *testing.T) {
	const inputID = "11111111-1111-4111-8111-111111111101"
	const endID = "11111111-1111-4111-8111-111111111102"
	root := baseTestHeader()
	root["P15UnknownWorkflow"] = map[string]any{"region": "opaque"}
	root["WFWorkflowActions"] = []any{
		p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "alpha", "UUID": inputID}),
		p15AcceptanceAction("is.workflow.actions.conditional", map[string]any{
			"GroupingIdentifier": "P15_OPAQUE_REGION", "WFControlFlowMode": int64(0),
			"WFCondition": int64(4), "WFInput": p15AcceptanceRef(inputID, nil),
			"WFConditionalActionString": "alpha", "WFNumberValue": "9",
		}),
		p15AcceptanceAction("com.example.p15.future-body", map[string]any{"P15Body": []any{false, "exact"}}),
		p15AcceptanceAction("is.workflow.actions.conditional", map[string]any{
			"GroupingIdentifier": "P15_OPAQUE_REGION", "WFControlFlowMode": int64(2),
			"UUID": endID, "CustomOutputName": "Synthetic output",
		}),
		p15AcceptanceAction("is.workflow.actions.getvariable", map[string]any{"WFVariable": p15AcceptanceRef(endID, nil)}),
	}
	for _, observed := range p15NativeImportRoundTrip(t, "opaque-region-export", root, plist.XMLFormat, plist.BinaryFormat) {
		if observed.plan != nil {
			for _, index := range []int{1, 3} {
				if observed.plan.Decisions[index].Classification != "EXACT_NATIVE_FALLBACK" {
					t.Fatalf("opaque paired marker %d was partially promoted: %+v", index, observed.plan.Decisions[index])
				}
			}
			if observed.plan.NaturalVerifiedCount == 0 {
				t.Fatal("opaque region erased the independent readable control")
			}
		}
		actions := observed.compiled["WFWorkflowActions"].([]any)
		refs := p15OutputIdentities(actions[4])
		if len(refs) != 1 || refs[0] != endID {
			t.Fatalf("outward region result identity changed: %v", refs)
		}
	}
	// Preexisting dangling raw references remain exact input data, not newly
	// authored let-result exports. Add an orphan control marker to verify the
	// existing SOURCE_INVALID classification without inventing a global UUID gate.
	invalid := deepClonePlistMap(root)
	actions := invalid["WFWorkflowActions"].([]any)
	actions[4].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariable"] = p15AcceptanceRef("P15_MISSING_IMPORTED_ID", nil)
	invalid["WFWorkflowActions"] = append(actions, p15AcceptanceAction("is.workflow.actions.conditional", map[string]any{"GroupingIdentifier": "P15_ORPHAN_IMPORTED_GROUP", "WFControlFlowMode": int64(2)}))
	for _, observed := range p15NativeImportRoundTrip(t, "opaque-invalid-existing-reference", invalid) {
		if observed.plan != nil && observed.plan.SourceInvalidCount == 0 {
			t.Fatal("preexisting orphan control marker was relabelled valid")
		}
	}
}

func TestP15NativeActionEnvelopePrivateIdentityCannotEscape(t *testing.T) {
	for _, tokenReference := range []bool{false, true} {
		t.Run(fmt.Sprintf("reference=%t", tokenReference), func(t *testing.T) {
			session := NewCanonicalBackendSession()
			defer session.Close()
			if err := session.EmitExactRawAction("is.workflow.actions.gettext", map[string]any{"UUID": "native-producer", "WFTextActionText": "synthetic"}, true); err != nil {
				t.Fatal(err)
			}
			if err := session.BindNativeActionResult("private-result", "unused-generated", true, false, ""); err != nil {
				t.Fatal(err)
			}
			var future any = "private-result"
			if tokenReference {
				future = map[string]any{"Type": "ActionOutput", "OutputUUID": "private-result"}
			}
			envelope := map[string]any{"WFWorkflowActionIdentifier": "com.example.p15.future", "P15Future": future}
			if err := session.EmitExactRawActionEnvelope("com.example.p15.future", envelope); err != nil {
				t.Fatal(err)
			}
			data, err := session.Finalize()
			if tokenReference {
				if err == nil || !strings.Contains(err.Error(), "NATIVE_OUTPUT_REFERENCE_UNRESOLVED") || len(data) != 0 {
					t.Fatalf("compiler-private identity escaped through an outer action field: %v", err)
				}
			} else if err != nil || len(data) == 0 {
				t.Fatalf("ordinary literal data was misclassified as a compiler reference: %v", err)
			}
		})
	}
}

func TestP15NativeActionEnvelopeLegacyArity(t *testing.T) {
	args := []actionArgument{{valueType: RawString, value: "com.example.p15.extra"}, {valueType: Nil}, {valueType: Nil}, {valueType: RawString, value: "extra"}}
	if _, _, err := rawLegacyActionEnvelope(args); err == nil || !strings.Contains(err.Error(), "NATIVE_ACTION_ENVELOPE_ARGUMENTS") {
		t.Fatalf("nil third argument bypassed envelope arity: %v", err)
	}
	actual, err := compileHybridSource("rawAction(\"com.example.p15.extra\", nil, nil, false)\n", true)
	if err == nil || actual != nil {
		t.Fatalf("production Legacy parser accepted extra raw argument: %v", err)
	}
}
