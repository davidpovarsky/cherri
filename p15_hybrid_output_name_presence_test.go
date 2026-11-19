package main

import (
	"testing"

	"howett.net/plist"
)

func p15OutputNameToken(t *testing.T, root map[string]any) map[string]any {
	t.Helper()
	actions, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 2 {
		t.Fatal("synthetic output-name fixture must contain two actions")
	}
	action := actions[1].(map[string]any)
	params := action["WFWorkflowActionParameters"].(map[string]any)
	serialized := params["WFTextActionText"].(map[string]any)
	value := serialized["Value"].(map[string]any)
	return value["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)
}

// A native token with an explicitly empty field is distinct from an absent
// field. Verify imported wire identity directly; authored native.ref clear-name
// syntax alone does not establish this native payload contract.
func TestP15HybridNativeOutputNamePresence(t *testing.T) {
	for _, test := range []struct {
		name    string
		present bool
		value   string
	}{
		{"absent", false, ""},
		{"explicit_empty", true, ""},
		{"named", true, "Synthetic producer"},
	} {
		t.Run(test.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("synthetic-native-name-presence.cherri", "let producer = number(7)\ntext(f\"{producer}\")")
			if err != nil {
				t.Fatal(err)
			}
			doc, err := DecodeNativeShortcutDocument(raw)
			if err != nil {
				t.Fatal(err)
			}
			token := p15OutputNameToken(t, doc.RawRoot)
			if test.present {
				token["OutputName"] = test.value
			} else {
				delete(token, "OutputName")
			}
			raw, err = plist.Marshal(doc.RawRoot, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			doc, err = DecodeNativeShortcutDocument(raw)
			if err != nil {
				t.Fatal(err)
			}
			plan, legacy, v2, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "synthetic native output-name presence", true)
			if err != nil {
				t.Fatal(err)
			}
			for dialect, compiled := range map[string]map[string]any{"legacy": legacy, "v2": v2} {
				if !plistValuesStrictlyEqual(doc.RawRoot, compiled) {
					t.Fatalf("%s changed native workflow wire", dialect)
				}
				actual := p15OutputNameToken(t, compiled)
				name, present := actual["OutputName"]
				if present != test.present || present && name != test.value {
					t.Fatalf("%s changed native field: present=%t value=%#v, want present=%t value=%q", dialect, present, name, test.present, test.value)
				}
				for i, decision := range plan.Decisions {
					if decision.Classification == "NATURAL_VERIFIED" {
						if err := verifyPreOverlayActionSemanticParity("synthetic native output-name presence", dialect, i, doc.Actions[i], pre[dialect][i].(map[string]any)); err != nil {
							t.Fatal(err)
						}
					}
				}
			}
			demoted := 0
			if plan.Lifecycle != nil {
				demoted = plan.Lifecycle.DemotedActionCount
			}
			t.Logf("NATIVE_NAME_PRESENCE field=%s natural=%d exactFallback=%d stage3=%d", test.name, plan.NaturalVerifiedCount, plan.ExactNativeCount, demoted)
		})
	}
}
