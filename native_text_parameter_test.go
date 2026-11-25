package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

type nativeTextParameterCase struct{ call, action, key string }

func nativeTextParameterCases() []nativeTextParameterCase {
	var cases []nativeTextParameterCase
	for _, name := range []string{"uppercase", "lowercase", "capitalize", "capitalizeAll", "titleCase", "alternatingCase"} {
		cases = append(cases, nativeTextParameterCase{name + "(value)", "text.changecase", "text"})
	}
	cases = append(cases,
		nativeTextParameterCase{`correctSpelling(value)`, "correctspelling", "text"},
		nativeTextParameterCase{`splitText(value, ",")`, "text.split", "text"},
		nativeTextParameterCase{`openURL(value)`, "openurl", "WFInput"},
		nativeTextParameterCase{`getURLs(value)`, "detect.link", "WFInput"},
		nativeTextParameterCase{`expandURL(value)`, "url.expand", "URL"},
	)
	for _, component := range []string{"Scheme", "Host", "Path", "Query", "Fragment"} {
		cases = append(cases, nativeTextParameterCase{fmt.Sprintf("getURLDetail(value, %q)", component), "geturlcomponent", "WFURL"})
	}
	return append(cases,
		nativeTextParameterCase{`splitText("alpha,BETA", value)`, "text.split", "WFTextCustomSeparator"},
		nativeTextParameterCase{`joinText(parts, value)`, "text.combine", "WFTextCustomSeparator"},
	)
}

// The envelope and placeholder come from captured Apple state serialization,
// independently of the Cherri encoder and generated accepted-form catalog.
func nativeTextAppleSample(t *testing.T, mutable bool) map[string]any {
	t.Helper()
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/encoding-table.json")
	if err != nil {
		t.Fatal(err)
	}
	var table struct {
		StateClasses map[string]map[string]any
	}
	if err := json.Unmarshal(raw, &table); err != nil {
		t.Fatal(err)
	}
	key := "variable:ActionOutput"
	if mutable {
		key = "variable:MyVar"
	}
	sample, ok := table.StateClasses["WFVariableStringParameterState"][key].(map[string]any)
	if !ok {
		t.Fatalf("native text sample is not a map: %#v", table.StateClasses["WFVariableStringParameterState"][key])
	}
	if sample["WFSerializationType"] != "WFTextTokenString" {
		t.Fatalf("native text-state sample missing: %#v", sample)
	}
	return sample
}

func nativeTextWireError(actual, sample any) error {
	if !reflect.DeepEqual(actual, sample) {
		return fmt.Errorf("text reference differs from native state serialization: got %#v; want %#v", actual, sample)
	}
	return nil
}

func TestNativeTextParameterProductionReferences(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/parameter-encodings.json")
	if err != nil {
		t.Fatal(err)
	}
	var encodings struct {
		ActionParameters map[string][]struct{ Key, Class, SingleStateClass string }
	}
	if err := json.Unmarshal(raw, &encodings); err != nil {
		t.Fatal(err)
	}
	for _, fixture := range nativeTextParameterCases() {
		matches := 0
		for _, p := range encodings.ActionParameters["is.workflow.actions."+fixture.action] {
			if p.Key == fixture.key {
				matches++
				if p.Class != "WFTextInputParameter" || p.SingleStateClass != "WFVariableStringParameterState" {
					t.Fatalf("native binding changed for %+v: %+v", fixture, p)
				}
			}
		}
		if matches != 1 {
			t.Fatalf("missing unique native text field for %+v", fixture)
		}
	}
	for _, legacy := range []bool{false, true} {
		for _, mutable := range []bool{false, true} {
			t.Run(fmt.Sprintf("legacy_%v/mutable_%v", legacy, mutable), func(t *testing.T) {
				prefix := "let "
				if legacy {
					prefix = "const "
				}
				binding := prefix + "value = text(\"https://example.org/alpha/beta?mode=qa#section\")\n"
				if mutable {
					binding = "var value = text(\"https://example.org/alpha/beta?mode=qa#section\")\n"
					if legacy {
						binding = "@value = text(\"https://example.org/alpha/beta?mode=qa#section\")\n"
					}
				}
				source := binding + prefix + "parts = splitText(\"alpha,BETA\", \",\")\n"
				for _, fixture := range nativeTextParameterCases() {
					call := fixture.call
					if legacy && mutable {
						call = strings.ReplaceAll(call, "value", "@value")
					}
					source += call + "\n"
				}
				if legacy {
					source = "#include 'actions/text'\n#include 'actions/web'\n" + source
				}
				actions := p15DateFieldActions(t, source, legacy)
				producer := p15Param(actions[0])
				if actions[0]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || producer["UUID"] == nil {
					t.Fatalf("text producer missing: %#v", actions[0])
				}
				wantRef := map[string]any{"Type": "ActionOutput", "OutputUUID": producer["UUID"], "OutputName": producer["CustomOutputName"]}
				first := 2
				if mutable {
					wantRef = map[string]any{"Type": "Variable", "VariableName": "value"}
					first++
				}
				sample := nativeTextAppleSample(t, mutable)
				sample["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"] = wantRef
				for index, fixture := range nativeTextParameterCases() {
					action := actions[first+index]
					if action["WFWorkflowActionIdentifier"] != "is.workflow.actions."+fixture.action {
						t.Fatalf("call order changed at %s: %#v", fixture.call, action)
					}
					if err := nativeTextWireError(p15Param(action)[fixture.key], sample); err != nil {
						t.Errorf("%s.%s: %v", fixture.call, fixture.key, err)
					}
					if fixture.action == "text.combine" {
						list := p15Param(action)["text"].(map[string]any)
						parts := p15Param(actions[first-1])
						want := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": parts["UUID"], "OutputName": parts["CustomOutputName"]}}
						if !reflect.DeepEqual(list, want) {
							t.Fatalf("join list input lost its declared Variable transport: %#v", list)
						}
					}
				}
			})
		}
	}
}

func TestNativeTextParameterOracleRejectsCorruption(t *testing.T) {
	sample := nativeTextAppleSample(t, false)
	if err := nativeTextWireError(deepClonePlistMap(sample), sample); err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(map[string]any){
		"bare-attachment": func(v map[string]any) {
			v["WFSerializationType"] = "WFTextTokenAttachment"
			v["Value"] = v["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"]
		},
		"literal-escape":    func(v map[string]any) { v["Value"].(map[string]any)["string"] = `\uFFFC` },
		"missing-reference": func(v map[string]any) { delete(v["Value"].(map[string]any), "attachmentsByRange") },
		"wrong-utf16-range": func(v map[string]any) {
			m := v["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
			m["{1, 1}"] = m["{0, 1}"]
			delete(m, "{0, 1}")
		},
		"wrong-producer": func(v map[string]any) {
			v["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["OutputUUID"] = "22222222-2222-2222-2222-222222222222"
		},
	} {
		t.Run(name, func(t *testing.T) {
			changed := deepClonePlistMap(sample)
			change(changed)
			if nativeTextWireError(changed, sample) == nil {
				t.Fatal("native text corruption accepted")
			}
		})
	}
}

func TestNativeTextLiteralControl(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		source := `getURLDetail("https://example.org/alpha/beta?mode=qa#section", "Host")`
		if legacy {
			source = "#include 'actions/web'\n" + source
		}
		actions := p15DateFieldActions(t, source, legacy)
		value := p15Param(actions[0])["WFURL"]
		if text, ok := value.(string); !ok || text != "https://example.org/alpha/beta?mode=qa#section" {
			t.Fatalf("literal URL changed: %#v", value)
		}
	}
}

func TestNativeTextImportedEnvelopesAndAuthoredEdit(t *testing.T) {
	uuid := "10000000-0000-4000-8000-000000000033"
	body := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "address"}
	token := nativeTextAppleSample(t, false)
	token["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"] = body
	qualifiedToken := deepClonePlistMap(token)
	qualifiedToken["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["OutputName"] = "Native display name"
	doc, _ := semBuildDocFromActions(t, []any{
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "https://example.org/alpha/beta", "UUID": uuid, "CustomOutputName": "address"}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.geturlcomponent", "WFWorkflowActionParameters": map[string]any{"WFURL": map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": body}, "WFURLComponent": "Host"}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.geturlcomponent", "WFWorkflowActionParameters": map[string]any{"WFURL": token, "WFURLComponent": "Path"}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.geturlcomponent", "WFWorkflowActionParameters": map[string]any{"WFURL": qualifiedToken, "WFURLComponent": "Query"}},
	}, plist.XMLFormat)
	plan, legacyMap, v2Map, pre, err := verifyCorpusDualHybridWithPreOverlay(doc, "native-text-mixed", true)
	if err != nil {
		t.Fatal(err)
	}
	if plan.NaturalVerifiedCount != 4 {
		t.Fatalf("mixed imported envelopes must stay natural: %+v", plan.Decisions)
	}
	for i := range doc.Actions {
		for _, dialect := range []string{"legacy", "v2"} {
			if err := verifyPreOverlayActionSemanticParity("native-text-mixed", dialect, i, doc.Actions[i], pre[dialect][i].(map[string]any)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !plistValuesStrictlyEqual(doc.RawRoot, legacyMap) || !plistValuesStrictlyEqual(doc.RawRoot, v2Map) {
		t.Fatal("imported mixed envelopes changed")
	}
	for _, legacy := range []bool{false, true} {
		var source string
		if legacy {
			source, err = formatHybridLegacySource(doc, plan)
		} else {
			source, err = formatHybridV2Source(doc, plan)
		}
		if err != nil {
			t.Fatal(err)
		}
		if legacy {
			source += "\nconst fresh = text(\"https://example.org/authored\")\n"
		} else {
			source += "\nlet fresh = text(\"https://example.org/authored\")\n"
		}
		source += "getURLDetail(fresh, \"Scheme\")\n"
		actions := p15DateFieldActions(t, source, legacy)
		var authored map[string]any
		for _, action := range actions {
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.geturlcomponent" && p15Param(action)["WFURLComponent"] == "Scheme" {
				authored = p15Param(action)
			}
		}
		if authored == nil {
			t.Fatal("authored edit not emitted")
		}
		wrapper, ok := authored["WFURL"].(map[string]any)
		if !ok || wrapper["WFSerializationType"] != "WFTextTokenString" {
			t.Fatalf("import provenance changed a new authored reference's carrier: %#v", authored)
		}
	}
}
