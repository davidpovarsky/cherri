package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"howett.net/plist"
)

func p15NativeLoopResultFixture(t *testing.T, kind, body string, outputName bool) *NativeShortcutDocument {
	t.Helper()
	source := "for _ in native.ref(\"ShortcutInput\") { " + body + " }"
	if kind == "count" {
		source = "repeat _ for 2 { " + body + " }"
	}
	raw, err := CompileSourceToPlist("synthetic-native-statement-loop.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	actions, ok := doc.RawRoot["WFWorkflowActions"].([]any)
	if !ok || len(actions) < 3 {
		t.Fatalf("fixture must have a balanced native loop: %T %d", doc.RawRoot["WFWorkflowActions"], len(actions))
	}
	end := actions[len(actions)-1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
	const resultUUID = "33333333-3333-3333-3333-333333330404"
	end["UUID"] = resultUUID
	token := map[string]any{"Type": "ActionOutput", "OutputUUID": resultUUID}
	if outputName {
		end["CustomOutputName"] = "Synthetic Results"
		token["OutputName"] = "Synthetic Results"
	}
	actions = append(actions, map[string]any{
		"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
		"WFWorkflowActionParameters": map[string]any{
			"WFTextActionText": map[string]any{
				"WFSerializationType": "WFTextTokenString",
				"Value":               map[string]any{"string": "\uFFFC", "attachmentsByRange": map[string]any{"{0, 1}": token}},
			},
		},
	})
	doc.RawRoot["WFWorkflowActions"] = actions
	raw, err = plist.Marshal(doc.RawRoot, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	doc, err = DecodeNativeShortcutDocument(raw)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// These fixtures begin with native statement-loop semantics. Their result token
// denotes existing native Repeat Results; it does not promise one nested List
// value per iteration. Import must preserve the existing graph and attachment.
func TestP15HybridNativeLoopOutputReference(t *testing.T) {
	for _, kind := range []string{"each", "count"} {
		for _, body := range []struct {
			name, source string
			fallback     int
		}{
			{"nested_each", `for _ in native.ref("ShortcutInput") { text("BODY") }`, 0},
			{"nested_count", `repeat _ for 1 { text("BODY") }`, 0},
			{"list", `list(listItem: ["LEFT", "TAIL"])`, 0},
			{"empty_list", `list(listItem: [])`, 1},
		} {
			for _, named := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/%s/named_%t", kind, body.name, named), func(t *testing.T) {
					doc := p15NativeLoopResultFixture(t, kind, body.source, named)
					plan, legMap, v2Map, preByDialect, err := verifyCorpusDualHybridWithPreOverlay(doc, "synthetic native loop output", true)
					if err != nil {
						t.Fatal(err)
					}
					if plan.Lifecycle == nil || plan.Lifecycle.DemotedActionCount != 0 {
						t.Fatalf("existing native result demoted: %+v", plan.Lifecycle)
					}
					if plan.NaturalVerifiedCount != len(doc.Actions)-body.fallback {
						t.Fatalf("unexpected native fixture classification: %+v", plan.Decisions)
					}
					for _, dialect := range []string{"legacy", "v2"} {
						if len(preByDialect[dialect]) != len(doc.Actions) {
							t.Fatal("native output reference added or removed an action")
						}
						for i := range doc.Actions {
							if err := verifyPreOverlayActionSemanticParity("synthetic native loop output", dialect, i, doc.Actions[i], preByDialect[dialect][i].(map[string]any)); err != nil {
								t.Fatal(err)
							}
						}
					}
					if !plistValuesStrictlyEqual(doc.RawRoot, legMap) || !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
						t.Fatal("native workflow wire identity changed")
					}
					rendered, err := RenderHybridV2Source(doc, plan)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(rendered, `native.ref("ActionOutput", uuid: `) {
						t.Fatal("native result boundary must stay explicit")
					}
					if strings.Contains(rendered, " = for ") || strings.Contains(rendered, " = repeat ") {
						t.Fatal("native result must not claim authored value-loop cardinality")
					}
				})
			}
		}
	}
}

// Importing native Repeat Results must not relax F04's authored List contract.
func TestP15HybridNativeLoopOutputPreservesAuthoredListRejection(t *testing.T) {
	for _, source := range []string{
		`let result = for _ in ["one", "two"] { yield ["LEFT", "TAIL"] }`,
		`let result = repeat _ for 2 { yield [] }`,
	} {
		raw, err := CompileSourceToPlist("synthetic-authored-nested-list.cherri", source)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
			t.Fatalf("authored List loop boundary changed: bytes=%d err=%v", len(raw), err)
		}
	}
}
