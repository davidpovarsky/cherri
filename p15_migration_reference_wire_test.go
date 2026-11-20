package main

import (
	"encoding/base64"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/ir"
	"howett.net/plist"
)

func TestP15RP33ReferenceRawParameter(t *testing.T) {
	folder := map[string]any{"fileLocation": map[string]any{"fileProviderDomainID": "com.apple.files", "relativeSubpath": "rp33-public.txt"}}
	encodedJSON, err := json.Marshal(folder)
	if err != nil {
		t.Fatal(err)
	}
	opaque := map[string]any{
		"fileLocation": folder["fileLocation"],
		"Empty":        "", "False": false, "Zero": int64(0), "Bytes": []byte{1, 2, 3, 4},
		"WFSerializationType": "ordinary-opaque-key", "Value": map[string]any{"ordinary": true},
	}
	xml, err := plist.Marshal(opaque, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	var decodedOpaque map[string]any
	if _, err := plist.Unmarshal(xml, &decodedOpaque); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, expression string
		want             map[string]any
	}{
		{"json_reference", "native.plist(" + strconv.Quote(string(encodedJSON)) + ")", folder},
		{"nested_carrier", `native.plist({"fileLocation": native.plist({"fileProviderDomainID": "com.apple.files", "relativeSubpath": "rp33-public.txt"})})`, folder},
		{"opaque_typed_payload", "native.plist(" + strconv.Quote("b64:"+base64.StdEncoding.EncodeToString(xml)) + ")", decodedOpaque},
	}
	for _, fixture := range cases {
		t.Run(fixture.name, func(t *testing.T) {
			source := `saveFile("rp33-public.txt", content: "public-payload", overwrite: false, folder: ` + fixture.expression + ")\n"
			actions := compileDebugFixture(t, source, false)
			if len(actions) != 2 || actions[0].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" || actions[0].WFWorkflowActionParameters["WFTextActionText"] != "public-payload" || actions[1].WFWorkflowActionIdentifier != "is.workflow.actions.documentpicker.save" {
				t.Fatalf("content producer or raw reference consumer changed: %#v", actions)
			}
			input := actions[1].WFWorkflowActionParameters["WFInput"].(map[string]any)["Value"].(map[string]any)
			if input["Type"] != "ActionOutput" || input["OutputUUID"] != actions[0].WFWorkflowActionParameters["UUID"] {
				t.Fatal("content input is detached from its original Text producer", input)
			}
			got := actions[1].WFWorkflowActionParameters["WFFolder"]
			if !reflect.DeepEqual(got, fixture.want) {
				t.Fatalf("opaque native reference changed keys, scalar kinds, bytes, presence or wrapper: got=%#v want=%#v", got, fixture.want)
			}
		})
	}
}

func TestP15RP33RawCarrierDetachment(t *testing.T) {
	// Both IR carrier forms and nesting must unwrap before the plist boundary.
	// Ordinary raw dictionaries and typed Data retain their actual kinds.
	inner := map[string]any{"False": false, "Zero": int64(0), "Empty": "", "Data": []byte{1, 2}}
	input := &ir.RawPlistValue{Data: map[string]any{"nested": ir.RawPlistValue{Data: inner}}}
	got := ConvertSemanticValueToParam(backend.SemanticValue{Kind: backend.ValRawNative, RawNativeVal: input})
	want := map[string]any{"nested": inner}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("raw carrier leaked or changed native data: got=%#v want=%#v", got, want)
	}
	got.(map[string]any)["nested"].(map[string]any)["False"] = true
	if inner["False"] != false {
		t.Fatal("normalization aliased an input native dictionary")
	}
	ordinary := map[string]any{"WFSerializationType": "literal", "Value": map[string]any{"ordinary": true}}
	if got := ConvertSemanticValueToParam(backend.SemanticValue{Kind: backend.ValRawNative, RawNativeVal: ordinary}); !reflect.DeepEqual(got, ordinary) {
		t.Fatal("raw native normalization reinterpreted ordinary reserved-looking keys", got)
	}
}
