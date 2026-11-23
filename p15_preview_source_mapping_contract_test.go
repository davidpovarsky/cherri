package main

import (
	"encoding/base64"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

// This single sequence owns a new RP41 local integration observation. It invokes
// the real unsigned mobile compiler and the JSON implementation behind the C
// export. It does not execute that ios&&cgo export, Swift, WebView or an Apple UI.
func TestP15RP41CombinedPreviewEditProduction(t *testing.T) {
	const source = `// Preserve the original comment: שלום 🍒
if (true) {
	wait(1)
}

// Only the next authored Wait parameter may change.
wait(2) // editable call
wait(3)
wait(4)
wait(5)
wait(6)
wait(7)
wait(8)
// Preserve the final comment and all unrelated calls.
`
	action, ok := schema.DefaultRegistry().LookupAction("wait")
	if !ok {
		t.Fatal("canonical Wait action is unavailable")
	}
	parameter, ok := action.PrimaryParameter()
	if !ok || parameter.WireKey == "" {
		t.Fatal("canonical Wait primary parameter is unavailable")
	}
	literalFor := func(text string) any {
		t.Helper()
		actions := rp41MobilePreviewActions(t, "wait("+text+")\n")
		var values []any
		for _, emitted := range actions {
			if emitted.WFWorkflowActionIdentifier == action.AppleIdentifier {
				value, present := emitted.WFWorkflowActionParameters[parameter.WireKey]
				if !present {
					t.Fatal("independent one-call literal has no native parameter")
				}
				values = append(values, value)
			}
		}
		if len(values) != 1 {
			t.Fatal("independent one-call literal has ambiguous native identity")
		}
		return values[0]
	}
	oldValue, firstValue, secondValue := literalFor("2"), literalFor("9"), literalFor("10")
	var original mobileCompileResponse
	before := rp41MobilePreviewActions(t, source, &original)
	index := rp41NativeLiteralIndex(t, before, action.AppleIdentifier, parameter.WireKey, oldValue)
	if index == 1 || original.PreviewSourceMap == nil {
		t.Fatal("joined fixture did not retain native/AST index divergence and same-compile map")
	}

	// A real generated action from this SAME preview is explanatory read-only.
	generated := -1
	for i, emitted := range before {
		if emitted.WFWorkflowActionIdentifier == "is.workflow.actions.number" {
			if _, present := emitted.WFWorkflowActionParameters["WFNumberActionNumber"]; present {
				if generated != -1 {
					t.Fatal("generated condition Number is ambiguous")
				}
				generated = i
			}
		}
	}
	if generated < 0 {
		t.Fatal("joined fixture has no generated condition Number")
	}
	unchanged, err := rp41ApplyPreviewEdit(t, source, original, generated, "WFNumberActionNumber", "0")
	if err == nil || !strings.Contains(err.Error(), "read-only") || unchanged != source {
		t.Fatalf("generated preview action was not explanatory/read-only: %v", err)
	}

	first, err := rp41ApplyPreviewEdit(t, source, original, index, parameter.WireKey, "9")
	wantFirst := strings.Replace(source, "wait(2)", "wait(9)", 1)
	if err != nil || first != wantFirst {
		t.Fatalf("first native edit changed the wrong source region: %v\n%s", err, first)
	}
	var rebuilt mobileCompileResponse
	after := rp41MobilePreviewActions(t, first, &rebuilt)
	checkNative := func(before, after []ShortcutAction, target int, value any) {
		t.Helper()
		if len(before) != len(after) {
			t.Fatal("bounded source edit changed native action cardinality")
		}
		for i := range before {
			if before[i].WFWorkflowActionIdentifier != after[i].WFWorkflowActionIdentifier {
				t.Fatalf("bounded source edit changed native action identity at %d", i)
			}
			if before[i].WFWorkflowActionIdentifier == action.AppleIdentifier {
				want := before[i].WFWorkflowActionParameters[parameter.WireKey]
				if i == target {
					want = value
				}
				if !reflect.DeepEqual(after[i].WFWorkflowActionParameters[parameter.WireKey], want) {
					t.Fatalf("wrong rebuilt native Wait value at %d", i)
				}
			}
		}
		if !reflect.DeepEqual(before[generated].WFWorkflowActionParameters["WFNumberActionNumber"],
			after[generated].WFWorkflowActionParameters["WFNumberActionNumber"]) {
			t.Fatal("bounded edit changed the unrelated condition producer")
		}
	}
	checkNative(before, after, index, firstValue)

	// Reusing the previous preview against the new source must fail, then a
	// fresh same-compile witness must permit the next bounded edit.
	unchanged, err = rp41ApplyPreviewEdit(t, first, original, index, parameter.WireKey, "10")
	if err == nil || !strings.Contains(err.Error(), "different source") || unchanged != first {
		t.Fatalf("stale preview did not preserve the current source: %v", err)
	}
	newIndex := rp41NativeLiteralIndex(t, after, action.AppleIdentifier, parameter.WireKey, firstValue)
	if newIndex != index {
		t.Fatal("first edit unexpectedly changed the selected native action position")
	}
	second, err := rp41ApplyPreviewEdit(t, first, rebuilt, newIndex, parameter.WireKey, "10")
	wantSecond := strings.Replace(source, "wait(2)", "wait(10)", 1)
	if err != nil || second != wantSecond {
		t.Fatalf("fresh witness did not edit the same exact authored region: %v\n%s", err, second)
	}
	var final mobileCompileResponse
	last := rp41MobilePreviewActions(t, second, &final)
	checkNative(after, last, newIndex, secondValue)

	unchanged, err = rp41ApplyPreviewEdit(t, second, final, newIndex, parameter.WireKey, ";;;")
	if err == nil || !strings.Contains(err.Error(), "invalid replacement") || unchanged != second {
		t.Fatalf("invalid replacement was not rejected without source mutation: %v", err)
	}
	for _, observation := range []struct {
		source  string
		preview mobileCompileResponse
	}{{source, original}, {first, rebuilt}, {second, final}} {
		plist, err := base64.StdEncoding.DecodeString(observation.preview.PlistBase64)
		if err != nil || observation.preview.PreviewSourceMap == nil ||
			observation.preview.PreviewSourceMap.SourceSHA256 != previewBytesSHA([]byte(observation.source)) ||
			observation.preview.PreviewSourceMap.PlistSHA256 != previewBytesSHA(plist) ||
			observation.preview.SignedBase64 != "" {
			t.Fatal("joined observation lost its actual unsigned source/plist witness")
		}
	}
	t.Logf("RP41_COMBINED_LOCAL: native_target=%d actions=%d source_before=%s source_after=%s generated_read_only=true stale_rejected=true invalid_expression_rejected=true native_ui_executed=false",
		index, len(before), previewBytesSHA([]byte(source)), previewBytesSHA([]byte(second)))
}
