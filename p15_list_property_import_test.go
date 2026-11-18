package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func p15PropertyImportFixture(t *testing.T, list, variable, tokenString bool) *NativeShortcutDocument {
	t.Helper()
	const uuid = "f1500044-0000-4000-8000-000000000001"
	producer := map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
		"WFWorkflowActionParameters": map[string]any{"UUID": uuid, "WFTextActionText": "synthetic"},
	}
	outputName := "Text"
	if list {
		producer = map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.detect.images",
			"WFWorkflowActionParameters": map[string]any{
				"UUID":    uuid,
				"WFInput": map[string]any{"Value": map[string]any{"Type": "ExtensionInput"}, "WFSerializationType": "WFTextTokenAttachment"},
			},
		}
		outputName = "Images"
	}
	actions := []any{producer}
	plain := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": outputName}
	if variable {
		actions = append(actions, map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.appendvariable",
			"WFWorkflowActionParameters": map[string]any{"WFVariableName": "SyntheticItems", "WFInput": p12RawActionOutAttachment(uuid, outputName)},
		})
		plain = map[string]any{"Type": "Variable", "VariableName": "SyntheticItems"}
	}
	decorated := map[string]any{}
	for key, value := range plain {
		decorated[key] = value
	}
	decorated["Aggrandizements"] = []any{map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"}}
	attachment := func(value map[string]any) map[string]any {
		return map[string]any{"Value": value, "WFSerializationType": "WFTextTokenAttachment"}
	}
	var input any = attachment(decorated)
	var name any = "renamed"
	if tokenString {
		input = attachment(plain)
		name = map[string]any{
			"Value":               map[string]any{"string": "prefix-\ufffc", "attachmentsByRange": map[string]any{"{7, 1}": decorated}},
			"WFSerializationType": "WFTextTokenString",
		}
	}
	actions = append(actions, map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.setitemname",
		"WFWorkflowActionParameters": map[string]any{"WFInput": input, "WFName": name},
	})
	return p12BuildNativeDoc(t, "P15_LIST_PROPERTY", actions)
}

func TestP15ImportedListPropertyPreservesNativeReference(t *testing.T) {
	for _, variable := range []bool{false, true} {
		for _, tokenString := range []bool{false, true} {
			t.Run(fmt.Sprintf("variable_%v_token_%v", variable, tokenString), func(t *testing.T) {
				doc := p15PropertyImportFixture(t, true, variable, tokenString)
				plan, legacy, v2, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15_LIST_PROPERTY", true)
				if err != nil {
					t.Fatal(err)
				}
				if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 || plan.NaturalVerifiedCount != len(doc.Actions) {
					t.Fatalf("native collection property was demoted: natural=%d total=%d lifecycle=%+v", plan.NaturalVerifiedCount, len(doc.Actions), plan.Lifecycle)
				}
				for _, pair := range []struct {
					name   string
					output map[string]any
				}{{"legacy", legacy}, {"v2", v2}} {
					if !plistValuesStrictlyEqual(doc.RawRoot, pair.output) {
						t.Fatalf("%s full-tree wire mismatch", pair.name)
					}
					for index := range doc.Actions {
						action, ok := pre[pair.name][index].(map[string]any)
						if !ok {
							t.Fatalf("%s missing pre-overlay action", pair.name)
						}
						if err := verifyPreOverlayActionSemanticParity("P15_LIST_PROPERTY", pair.name, index, doc.Actions[index], action); err != nil {
							t.Fatal(err)
						}
					}
					// A provenance overlay can restore omitted metadata. The pre-overlay
					// oracle must still reject losing the native property operation itself.
					last := len(doc.Actions) - 1
					original := pre[pair.name][last].(map[string]any)
					changed := map[string]any{}
					for key, value := range original {
						changed[key] = value
					}
					params := map[string]any{}
					for key, value := range original["WFWorkflowActionParameters"].(map[string]any) {
						params[key] = value
					}
					params["WFInput"] = p12RawActionOutAttachment("f1500044-0000-4000-8000-000000000001", "Images")
					params["WFName"] = "renamed"
					changed["WFWorkflowActionParameters"] = params
					if verifyPreOverlayActionSemanticParity("P15_LIST_PROPERTY", pair.name, last, doc.Actions[last], changed) == nil {
						t.Fatal("lost-property mutation passed the pre-overlay oracle")
					}
				}
				legSrc, err := RenderHybridLegacySource(doc, plan)
				if err != nil {
					t.Fatal(err)
				}
				v2Src, err := RenderHybridV2Source(doc, plan)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(legSrc, "setName(") || !strings.Contains(v2Src, "setName(") {
					t.Fatal("consumer action no longer has natural editable syntax")
				}
				if !strings.Contains(legSrc, "rawRef(") || !strings.Contains(v2Src, "native.ref(") {
					t.Fatal("collection property must retain an explicit native attachment")
				}
				// Import provenance is not permission to treat an authored List as a
				// single record. The same document must reject newly authored .Name.
				checkedAuthored := false
				for symbol, metadata := range plan.Symbols {
					if strVal(metadata["UUID"]) != "f1500044-0000-4000-8000-000000000001" {
						continue
					}
					checkedAuthored = true
					authored := v2Src + "\ntext(" + symbol + ".Name)\n"
					raw, err := CompileSourceToPlist("p15-imported-authored-field.cherri", authored)
					if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), string(analysis.CodeArgumentType)) {
						t.Fatal("import header bypassed authored List property validation")
					}
					break
				}
				if !checkedAuthored {
					t.Fatal("fixture did not expose its canonical List producer symbol")
				}
			})
		}
	}
}

func TestP15ImportedScalarPropertyKeepsNaturalSyntax(t *testing.T) {
	doc := p15PropertyImportFixture(t, false, false, false)
	plan, _, _, _, err := verifyCorpusDualHybridWithPreOverlay(doc, "P15_SCALAR_PROPERTY", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 || plan.NaturalVerifiedCount != len(doc.Actions) {
		t.Fatal("scalar property control failed")
	}
	v2Src, err := RenderHybridV2Source(doc, plan)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(v2Src, ".Name") || strings.Contains(v2Src, "native.ref(") {
		t.Fatal("ordinary scalar property was unnecessarily rewritten")
	}
}

func TestP15ImportedCollectionHintDoesNotGuessVariants(t *testing.T) {
	const id = "com.example.p15.collection"
	const uuid = "f1500044-0000-4000-8000-000000000001"
	makeContext := func(candidates []ReverseActionCandidate) *semanticImportContext {
		return &semanticImportContext{
			Doc:          &NativeShortcutDocument{Actions: []NativeActionRecord{{Identifier: id, Parameters: map[string]any{"UnknownShape": true}}}, ProducerUUIDIndex: map[string]int{uuid: 0}},
			Registry:     &ReverseActionRegistry{ByAppleIdentifier: map[string][]ReverseActionCandidate{id: candidates}},
			UUIDToSymbol: map[string]string{uuid: "SyntheticOutput"}, Symbols: map[string]map[string]any{"SyntheticOutput": {"ValueType": ""}},
		}
	}
	att := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid}
	candidate := ReverseActionCandidate{CallableName: "syntheticList", AppleIdentifier: id, Definition: actionDefinition{outputType: Arr}}
	if got := makeContext([]ReverseActionCandidate{candidate}).attachmentBaseValueType(att); got != "List" {
		t.Fatalf("unique array output lost: %q", got)
	}
	other := candidate
	other.CallableName = "syntheticScalar"
	other.Definition.outputType = String
	for _, candidates := range [][]ReverseActionCandidate{{candidate, other}, {other, candidate}} {
		if got := makeContext(candidates).attachmentBaseValueType(att); got != "" {
			t.Fatalf("ambiguous output type was guessed: %q", got)
		}
	}
	if got := makeContext(nil).attachmentBaseValueType(att); got != "" {
		t.Fatalf("absent output contract was guessed: %q", got)
	}
}
