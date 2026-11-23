package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

// This helper calls exactly the JSON implementation exported by the native C ABI.
// The witness is the retained response from the SAME compile that supplied the
// tested preview actions, not a recompile or a map reconstructed from payloads.
func rp41ApplyPreviewEdit(t *testing.T, source string, preview mobileCompileResponse, actionIndex int, key, expression string) (string, error) {
	t.Helper()
	request := rp41PreviewEditRequest(source, preview, actionIndex, key, expression)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := editPreviewSourceForMobile(string(encoded))
	if !response.OK {
		return response.Source, fmt.Errorf("%s", response.Error)
	}
	return response.Source, nil
}

func rp41PreviewEditRequest(source string, preview mobileCompileResponse, actionIndex int, key, expression string) previewSourceEditRequest {
	profile := ""
	if preview.PreviewSourceMap != nil {
		profile = preview.PreviewSourceMap.BuildProfile
	}
	return previewSourceEditRequest{
		Source: source, Name: preview.Name, Mode: preview.Mode, BuildProfile: profile,
		PlistBase64: preview.PlistBase64, PreviewSourceMap: preview.PreviewSourceMap,
		ActionIndex: actionIndex, ParameterKey: key, NewExpression: expression,
	}
}

func rp41PreviewControlCompile(t *testing.T, source string, options CompileOptions) (mobileCompileResponse, []ShortcutAction) {
	t.Helper()
	preview := compileForMobileModeWithOptions(source, "RP41MappingControls", "v2", false, options)
	if !preview.OK || preview.PreviewSourceMap == nil || preview.SignedBase64 != "" {
		t.Fatalf("actual unsigned preview compile failed: %+v", preview)
	}
	raw, err := base64.StdEncoding.DecodeString(preview.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := decodeShortcutRawMap(raw)
	if err != nil {
		t.Fatal(err)
	}
	wire, ok := workflow["WFWorkflowActions"].([]any)
	if !ok || len(wire) != preview.PreviewSourceMap.ActionCount {
		t.Fatal("sidecar does not bind the actual complete native action array")
	}
	actions := make([]ShortcutAction, len(wire))
	for index, item := range wire {
		action, ok := item.(map[string]any)
		if !ok {
			t.Fatal("native action is not a dictionary")
		}
		identifier, _ := action["WFWorkflowActionIdentifier"].(string)
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		actions[index] = ShortcutAction{WFWorkflowActionIdentifier: identifier, WFWorkflowActionParameters: parameters}
	}
	return preview, actions
}

func TestP15RP41PreviewSourceMapOriginBoundaries(t *testing.T) {
	const waitID = "is.workflow.actions.delay"
	const waitKey = "WFDelayTime"
	for _, test := range []struct {
		name               string
		source             string
		waitCount          int
		readOnlyBeforeLast bool
		modules            map[string]string
	}{
		{"identical_native_payloads", "if true { wait(2) }\nwait(2)\nwait(2)\n", 3, false, nil},
		{"function_body_and_envelope", "function helper() -> Void { wait(2) }\nhelper()\nwait(2)\n", 2, true, nil},
		{"copy_expansions", "copy repeated { wait(2) }\npaste repeated\npaste repeated\nwait(2)\n", 3, true, nil},
		{"raw_lookalike", "native.action(\"is.workflow.actions.delay\", {\"WFDelayTime\":2})\nwait(2)\n", 2, true, nil},
		{"imported_body", "import \"./helper.cherri\" as helper\nhelper.execute()\nwait(2)\n", 2, true,
			map[string]string{"./helper.cherri": "function execute() -> Void { wait(2) }\n"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			preview, actions := rp41PreviewControlCompile(t, test.source, CompileOptions{VirtualModules: test.modules})
			var indices []int
			for index, action := range actions {
				if action.WFWorkflowActionIdentifier == waitID {
					indices = append(indices, index)
				}
			}
			if len(indices) != test.waitCount {
				t.Fatalf("actual native Wait inventory: got %v, want %d", indices, test.waitCount)
			}
			last := strings.LastIndex(test.source, "wait(2)")
			want := test.source[:last] + "wait(9)" + test.source[last+len("wait(2)"):]
			updated, err := rp41ApplyPreviewEdit(t, test.source, preview, indices[len(indices)-1], waitKey, "9")
			if err != nil || updated != want {
				t.Fatalf("actual native identity did not edit only the last authored call: %v\n%s", err, updated)
			}
			if test.name == "identical_native_payloads" {
				middle := strings.Index(test.source, "}\nwait(2)") + len("}\n")
				wantMiddle := test.source[:middle] + "wait(9)" + test.source[middle+len("wait(2)"):]
				updated, err := rp41ApplyPreviewEdit(t, test.source, preview, indices[1], waitKey, "9")
				if err != nil || updated != wantMiddle {
					t.Fatalf("identical native payload selected the wrong authored occurrence: %v\n%s", err, updated)
				}
			}
			readOnly := indices[:1]
			if test.readOnlyBeforeLast {
				readOnly = indices[:len(indices)-1]
			}
			for _, index := range readOnly {
				updated, err := rp41ApplyPreviewEdit(t, test.source, preview, index, waitKey, "9")
				if err == nil || !strings.Contains(err.Error(), "read-only") || updated != test.source {
					t.Fatalf("generated/nested/copied/imported native action %d was editable: %v", index, err)
				}
			}
		})
	}
	for _, test := range []struct{ name, source, old, replacement, identifier, key string }{
		{"binding_call", "let retained=number(2)\nwait(1)\n", "number(2)", "number(9)", "is.workflow.actions.number", "WFNumberActionNumber"},
		{"named_argument", "// Exact comment\nwait(seconds: 2)\n", "wait(seconds: 2)", "wait(seconds: 9)", waitID, waitKey},
	} {
		t.Run(test.name, func(t *testing.T) {
			preview, actions := rp41PreviewControlCompile(t, test.source, CompileOptions{})
			index := -1
			for i, action := range actions {
				if action.WFWorkflowActionIdentifier == test.identifier {
					if index != -1 {
						t.Fatal("nonunique native fixture target")
					}
					index = i
				}
			}
			if index < 0 {
				t.Fatal("native fixture target is absent")
			}
			updated, err := rp41ApplyPreviewEdit(t, test.source, preview, index, test.key, "9")
			if err != nil || updated != strings.Replace(test.source, test.old, test.replacement, 1) {
				t.Fatalf("explicit authored argument was not preserved/mapped: %v\n%s", err, updated)
			}
		})
	}
	for _, test := range []struct{ name, source, identifier, key string }{
		{"omitted_optional", "alert(\"body\")\n", "is.workflow.actions.alert", "WFAlertActionTitle"},
		{"omitted_default", "formatNumber(2)\n", "is.workflow.actions.format.number", "WFNumberFormatDecimalPlaces"},
		{"variadic_wire", "url(\"https://one.invalid\", \"https://two.invalid\")\n", "is.workflow.actions.url", "WFURLActionURL"},
		{"custom_builder", "makeVCard(\"Title\", subtitle: \"Subtitle\")\n", "is.workflow.actions.gettext", "WFTextActionText"},
	} {
		t.Run(test.name, func(t *testing.T) {
			preview, actions := rp41PreviewControlCompile(t, test.source, CompileOptions{})
			index := -1
			for i, action := range actions {
				if action.WFWorkflowActionIdentifier == test.identifier {
					if index != -1 {
						t.Fatal("nonunique native read-only fixture target")
					}
					index = i
				}
			}
			if index < 0 {
				t.Fatal("native read-only fixture target is absent")
			}
			updated, err := rp41ApplyPreviewEdit(t, test.source, preview, index, test.key, "9")
			if err == nil || !strings.Contains(err.Error(), "read-only") || updated != test.source {
				t.Fatalf("omitted/default/variadic/custom wire acquired a guessed source binding: %v", err)
			}
		})
	}
	t.Run("native_boolean_checkbox", func(t *testing.T) {
		const source = "// Preserve the direct checkbox argument\nquicklook(\"body\", fullScreen: true)\n"
		preview, actions := rp41PreviewControlCompile(t, source, CompileOptions{})
		index := -1
		for i, action := range actions {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				if index != -1 {
					t.Fatal("nonunique native Boolean fixture target")
				}
				index = i
			}
		}
		if index < 0 {
			t.Fatal("native Boolean fixture target is absent")
		}
		value, ok := actions[index].WFWorkflowActionParameters["WFQuickLookActionFullScreen"].(bool)
		if !ok || !value {
			t.Fatal("fixture did not produce the native Boolean checkbox value")
		}
		updated, err := rp41ApplyPreviewEdit(t, source, preview, index, "WFQuickLookActionFullScreen", "false")
		if err != nil || updated != strings.Replace(source, "fullScreen: true", "fullScreen: false", 1) {
			t.Fatalf("actual native checkbox did not edit only its explicit Boolean argument: %v\n%s", err, updated)
		}
	})
	const backfill = "debugClearLogs()\nlet retained=base64Decode(\"AQ==\")\ndebugLog()\nwait(2)\ndebugShowLogs(withDates:true)\n"
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		t.Run("backfill_"+string(profile), func(t *testing.T) {
			preview, actions := rp41PreviewControlCompile(t, backfill, CompileOptions{BuildProfile: profile})
			index, date := -1, -1
			for i, action := range actions {
				if action.WFWorkflowActionIdentifier == waitID {
					index = i
				}
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.detect.date" {
					date = i
				}
			}
			if index < 0 || (profile == BuildProfileDebug && (date < 0 || date >= index)) || (profile == BuildProfileRelease && date >= 0) {
				t.Fatalf("actual profile/backfill fixture did not exercise the required boundary: wait=%d date=%d", index, date)
			}
			updated, err := rp41ApplyPreviewEdit(t, backfill, preview, index, waitKey, "9")
			if err != nil || updated != strings.Replace(backfill, "wait(2)", "wait(9)", 1) {
				t.Fatalf("final native index lost the exact call through backfill/profile: %v", err)
			}
		})
	}
}

func TestP15RP41PreviewSourceMapRejectsStaleAndCorruptWitnesses(t *testing.T) {
	const source = "// שלום 🍒\nwait(2)\nwait(3)\n"
	preview, actions := rp41PreviewControlCompile(t, source, CompileOptions{})
	index := -1
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.delay" {
			index = i
			break
		}
	}
	if index < 0 || len(preview.PreviewSourceMap.Entries) != 2 {
		t.Fatal("positive mapping fixture is incomplete")
	}
	baseline := rp41PreviewEditRequest(source, preview, index, "WFDelayTime", "9")
	if got, err := applyPreviewSourceEdit(baseline); err != nil || got != strings.Replace(source, "wait(2)", "wait(9)", 1) {
		t.Fatalf("healthy actual preview binding rejected: %v", err)
	}
	for _, test := range []struct {
		name, reason string
		change       func(*previewSourceEditRequest)
	}{
		{"stale_source", "different source", func(r *previewSourceEditRequest) { r.Source += "// newer edit\n" }},
		{"stale_name", "name, mode and profile", func(r *previewSourceEditRequest) { r.Name += " changed" }},
		{"stale_mode", "name, mode and profile", func(r *previewSourceEditRequest) { r.Mode = "legacy" }},
		{"stale_profile", "name, mode and profile", func(r *previewSourceEditRequest) { r.BuildProfile = "release" }},
		{"missing_map", "missing", func(r *previewSourceEditRequest) { r.PreviewSourceMap = nil }},
		{"changed_plist", "plist does not match", func(r *previewSourceEditRequest) {
			r.PlistBase64 = base64.StdEncoding.EncodeToString([]byte("different"))
		}},
		{"unreviewed_schema", "missing", func(r *previewSourceEditRequest) { r.PreviewSourceMap.Version++ }},
		{"map_integrity", "integrity", func(r *previewSourceEditRequest) { r.PreviewSourceMap.Entries[0].CallStart++ }},
		{"negative_index", "index or count", func(r *previewSourceEditRequest) { r.ActionIndex = -1 }},
		{"absent_wire", "read-only", func(r *previewSourceEditRequest) { r.ParameterKey = "NotTheWaitParameter" }},
		{"invalid_expression", "invalid replacement", func(r *previewSourceEditRequest) { r.NewExpression = ";;;" }},
		{"rehashed_bad_count", "index or count", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.ActionCount++
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
		{"rehashed_wrong_identifier", "identity", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.Entries[0].ActionIdentifier = "is.workflow.actions.number"
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
		{"rehashed_duplicate_action", "ambiguous action", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.Entries = append(r.PreviewSourceMap.Entries[:1], r.PreviewSourceMap.Entries[0])
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
		{"rehashed_structural_key", "ambiguous argument", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.Entries[0].Parameters[0].WireKey = "UUID"
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
		{"rehashed_partial_argument", "existing argument", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.Entries[0].Parameters[0].ArgumentStart--
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
		{"rehashed_other_call_argument", "ambiguous argument", func(r *previewSourceEditRequest) {
			r.PreviewSourceMap.Entries[0].Parameters = append([]previewSourceParameter{}, r.PreviewSourceMap.Entries[1].Parameters...)
			r.PreviewSourceMap.IntegritySHA256 = previewMapIntegrity(*r.PreviewSourceMap)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := json.Marshal(baseline)
			if err != nil {
				t.Fatal(err)
			}
			var request previewSourceEditRequest
			if err := json.Unmarshal(encoded, &request); err != nil {
				t.Fatal(err)
			}
			test.change(&request)
			encoded, err = json.Marshal(request)
			if err != nil {
				t.Fatal(err)
			}
			response := editPreviewSourceForMobile(string(encoded))
			if response.OK || response.Source != request.Source || !strings.Contains(response.Error, test.reason) {
				t.Fatalf("causal rejection was missing or reached another guard: %+v", response)
			}
		})
	}
}

func TestP15RP41PreviewMapPreservesNativeBytesAndASTCompatibility(t *testing.T) {
	const source = "wait(2)\nwait(3)\n"
	ordinary, err := CompileSourceToPlist("RP41NativeBytes.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var mapping previewSourceMap
	withMap, err := CompileSourceToPlistWithOptions("RP41NativeBytes.cherri", source, CompileOptions{previewSourceMap: &mapping})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(ordinary, withMap) || len(mapping.Entries) != 2 || mapping.PlistSHA256 != previewBytesSHA(withMap) {
		t.Fatal("opt-in provenance changed native bytes or failed to capture the actual output")
	}
	const oldSource = "let first=1\nlet second=2\nshow(\"Old Title\")\n"
	svc := service.NewService(schema.DefaultRegistry())
	updated, err := svc.ApplyVisualSourceEdit(oldSource, 2, "text", "\"New Title\"")
	if err != nil || updated != strings.Replace(oldSource, "Old Title", "New Title", 1) {
		t.Fatalf("existing AST-index API changed semantics: %v", err)
	}
}
