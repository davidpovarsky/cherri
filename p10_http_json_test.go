package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Helper to reconstruct a JSON object from Apple Shortcuts WFDictionaryFieldValue
// as Apple's WFDownloadURLAction serializes WFJSONValues at runtime.
func reconstructJSONFromWFDictionaryFieldValue(t *testing.T, raw any, resolvedVars map[string]any) map[string]any {
	t.Helper()
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected WFDictionaryFieldValue map, got %T (%v)", raw, raw)
	}
	if m["WFSerializationType"] != "WFDictionaryFieldValue" {
		t.Fatalf("expected WFSerializationType == WFDictionaryFieldValue, got %v", m["WFSerializationType"])
	}
	valMap, ok := m["Value"].(map[string]any)
	if !ok {
		t.Fatalf("WFDictionaryFieldValue missing Value map: %v", m)
	}
	if innerVal, hasInner := valMap["Value"].(map[string]any); hasInner && valMap["WFSerializationType"] == "WFDictionaryFieldValue" {
		valMap = innerVal
	}
	items, ok := valMap["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		t.Fatalf("missing WFDictionaryFieldValueItems slice in %v", valMap)
	}
	out := make(map[string]any, len(items))
	for _, rawItem := range items {
		item := rawItem.(map[string]any)
		keyStr := resolveWFTokenString(t, item["WFKey"], resolvedVars)
		out[keyStr] = reconstructWFDictionaryItemValue(t, item, resolvedVars)
	}
	return out
}

func resolveWFTokenString(t *testing.T, raw any, resolvedVars map[string]any) string {
	t.Helper()
	if s, ok := raw.(string); ok {
		return s
	}
	m, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("expected WFTextTokenString map or string, got %T (%v)", raw, raw)
	}
	valMap, ok := m["Value"].(map[string]any)
	if !ok {
		return fmt.Sprint(m["Value"])
	}
	strVal, _ := valMap["string"].(string)
	if strVal == "\uFFFC" {
		if atts, ok := valMap["attachmentsByRange"].(map[string]any); ok {
			if att0, ok := atts["{0, 1}"].(map[string]any); ok {
				if name, ok := att0["OutputName"].(string); ok && resolvedVars != nil {
					if v, exists := resolvedVars[name]; exists {
						return fmt.Sprint(v)
					}
				}
				if name, ok := att0["VariableName"].(string); ok && resolvedVars != nil {
					if v, exists := resolvedVars[name]; exists {
						return fmt.Sprint(v)
					}
				}
			}
		}
	}
	return strVal
}

func reconstructWFDictionaryItemValue(t *testing.T, item map[string]any, resolvedVars map[string]any) any {
	t.Helper()
	var itemType uint64
	switch v := item["WFItemType"].(type) {
	case uint64:
		itemType = v
	case int64:
		itemType = uint64(v)
	case int:
		itemType = uint64(v)
	}
	wfVal, _ := item["WFValue"].(map[string]any)
	switch itemType {
	case 0: // Text
		if wfVal["WFSerializationType"] == "WFTextTokenAttachment" {
			valObj, _ := wfVal["Value"].(map[string]any)
			if name, ok := valObj["OutputName"].(string); ok && resolvedVars != nil {
				if v, exists := resolvedVars[name]; exists {
					return v
				}
			}
			if name, ok := valObj["VariableName"].(string); ok && resolvedVars != nil {
				if v, exists := resolvedVars[name]; exists {
					return v
				}
			}
		}
		return resolveWFTokenString(t, wfVal, resolvedVars)
	case 1: // Dictionary
		return reconstructJSONFromWFDictionaryFieldValue(t, wfVal, resolvedVars)
	case 2: // Array
		arrSlice, ok := wfVal["Value"].([]any)
		if !ok {
			t.Fatalf("expected array Value slice, got %T (%v)", wfVal["Value"], wfVal)
		}
		outArr := make([]any, len(arrSlice))
		for i, rawElem := range arrSlice {
			switch elem := rawElem.(type) {
			case map[string]any:
				if _, hasItemType := elem["WFItemType"]; hasItemType {
					outArr[i] = reconstructWFDictionaryItemValue(t, elem, resolvedVars)
				} else if elem["WFSerializationType"] == "WFDictionaryFieldValue" {
					outArr[i] = reconstructJSONFromWFDictionaryFieldValue(t, elem, resolvedVars)
				} else if elem["WFSerializationType"] == "WFTextTokenString" {
					outArr[i] = resolveWFTokenString(t, elem, resolvedVars)
				} else {
					outArr[i] = elem
				}
			default:
				outArr[i] = elem
			}
		}
		return outArr
	case 3: // Number
		s := resolveWFTokenString(t, wfVal, resolvedVars)
		var num float64
		fmt.Sscanf(s, "%f", &num)
		return num
	case 4: // Bool
		if b, ok := wfVal["Value"].(bool); ok {
			return b
		}
		return false
	default:
		return nil
	}
}

// Phase B1 & B2 (Asana 1219800356179243 & 1219800511200890 & 1219800392994574):
// Comprehensive verification of jsonRequest, dynamic JSON bodies, nested structures,
// fail-closed rejection of non-literal WFJSONValues, fileRequest dynamic JSON alternative,
// and local HTTP echo server validation.
func TestP10_B1_B2_JsonRequest_DynamicAndLiteralContract_And_LocalEchoServer(t *testing.T) {
	r := newRoundTripRunner(t)

	// 1. Dictionary literal with static and dynamic field values, nested dictionaries,
	// arrays, nested arrays, integers, booleans, and null/empty values -> succeeds and
	// round-trips through local HTTP echo server.
	t.Run("literal-dict-with-dynamic-fields-and-local-http-echo", func(t *testing.T) {
		var receivedHeaders http.Header
		var receivedBody map[string]any

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			if req.Method != http.MethodPost {
				t.Errorf("expected POST request, got %s", req.Method)
			}
			receivedHeaders = req.Header.Clone()
			bodyBytes, err := io.ReadAll(req.Body)
			if err != nil {
				t.Errorf("read request body: %v", err)
			}
			if err := json.Unmarshal(bodyBytes, &receivedBody); err != nil {
				t.Errorf("invalid JSON received by echo server: %v (raw: %s)", err, string(bodyBytes))
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(bodyBytes)
		}))
		defer srv.Close()

		v2Src := fmt.Sprintf(`let dynamicModel = "gpt-4o-mini"
let resp = jsonRequest(
	%q,
	method: "POST",
	body: {
		"model": dynamicModel,
		"stream": false,
		"max_tokens": 256,
		"empty_val": nil,
		"nested": {
			"env": "prod",
			"flags": [true, false],
			"matrix": [[1, 2], [3, 4]]
		}
	},
	headers: {
		"Authorization": "Bearer test-token-123",
		"X-Custom-Client": "Cherri-P10"
	}
)
`, srv.URL)

		plistPath := r.compileSource("b1_json_literal_dynamic_fields", v2Src)
		doc := loadRoundTripDocument(t, plistPath)

		var dlAct map[string]any
		for _, a := range doc["WFWorkflowActions"].([]any) {
			m := a.(map[string]any)
			if m["WFWorkflowActionIdentifier"] == "is.workflow.actions.downloadurl" {
				dlAct = m
				break
			}
		}
		if dlAct == nil {
			t.Fatalf("expected is.workflow.actions.downloadurl action in plist")
		}

		params := dlAct["WFWorkflowActionParameters"].(map[string]any)
		if params["WFHTTPMethod"] != "POST" {
			t.Errorf("expected WFHTTPMethod == 'POST', got %v", params["WFHTTPMethod"])
		}
		if params["WFHTTPBodyType"] != "JSON" {
			t.Errorf("expected WFHTTPBodyType == 'JSON', got %v", params["WFHTTPBodyType"])
		}

		// Reconstruct the JSON body and headers from the compiled plist and send to local echo server
		resolved := map[string]any{"dynamicModel": "gpt-4o-mini"}
		jsonPayload := reconstructJSONFromWFDictionaryFieldValue(t, params["WFJSONValues"], resolved)
		headerMap := reconstructJSONFromWFDictionaryFieldValue(t, params["WFHTTPHeaders"], resolved)

		payloadBytes, err := json.Marshal(jsonPayload)
		if err != nil {
			t.Fatalf("marshal reconstructed JSON payload: %v", err)
		}

		req, err := http.NewRequest(http.MethodPost, srv.URL, bytes.NewReader(payloadBytes))
		if err != nil {
			t.Fatalf("new request: %v", err)
		}
		req.Header.Set("Content-Type", "application/json")
		for hk, hv := range headerMap {
			req.Header.Set(hk, fmt.Sprint(hv))
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("echo server request failed: %v", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 OK from echo server, got %d", resp.StatusCode)
		}
		if receivedHeaders.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type application/json, got %q", receivedHeaders.Get("Content-Type"))
		}
		if receivedHeaders.Get("Authorization") != "Bearer test-token-123" {
			t.Errorf("expected Authorization header 'Bearer test-token-123', got %q", receivedHeaders.Get("Authorization"))
		}
		if receivedBody["model"] != "gpt-4o-mini" {
			t.Errorf("expected echoed model 'gpt-4o-mini', got %v", receivedBody["model"])
		}
		if receivedBody["stream"] != false {
			t.Errorf("expected echoed stream == false, got %v", receivedBody["stream"])
		}
		if receivedBody["max_tokens"] != float64(256) {
			t.Errorf("expected echoed max_tokens == 256, got %v", receivedBody["max_tokens"])
		}
		nested, ok := receivedBody["nested"].(map[string]any)
		if !ok || nested["env"] != "prod" {
			t.Errorf("expected nested.env == 'prod', got %v", receivedBody["nested"])
		}
	})

	// 2. Negative tests: Passing a dictionary variable, getDictionary(), or ShortcutInput
	// directly to jsonRequest body (WFJSONValues) MUST fail closed in both V2 and Legacy!
	negativeCases := []struct {
		name      string
		v2Src     string
		legacySrc string
	}{
		{
			name: "dictionary-variable-in-jsonRequest-body",
			v2Src: `let payload = {"model": "gpt-4"}
jsonRequest("https://example.com/api", method: "POST", body: payload)
`,
			legacySrc: `#include 'actions/web'
@payload = {"model": "gpt-4"}
jsonRequest("https://example.com/api", "POST", @payload)
`,
		},
		{
			name: "getDictionary-dynamic-input-in-jsonRequest-body",
			v2Src: `let raw = "{\"model\":\"gpt-4\"}"
let dict = getDictionary(raw)
jsonRequest("https://example.com/api", method: "POST", body: dict)
`,
			legacySrc: `#include 'actions/web'
@raw = '{"model":"gpt-4"}'
@dict = getDictionary(@raw)
jsonRequest("https://example.com/api", "POST", @dict)
`,
		},
		{
			name: "ShortcutInput-dictionary-in-jsonRequest-body",
			v2Src: `let inDict = getDictionary(ShortcutInput)
jsonRequest("https://example.com/api", method: "POST", body: inDict)
`,
			legacySrc: `#include 'actions/web'
@inDict = getDictionary(ShortcutInput)
jsonRequest("https://example.com/api", "POST", @inDict)
`,
		},
	}

	for _, tc := range negativeCases {
		t.Run("fails-closed-"+tc.name, func(t *testing.T) {
			_, v2Err := compileV2Helper(t, "neg_v2_"+tc.name, tc.v2Src)
			if v2Err == nil {
				t.Fatalf("%s (V2): expected compilation to fail closed for non-literal WFJSONValues body, but succeeded!", tc.name)
			}
			if !strings.Contains(v2Err.Error(), "WFJSONValues") && !strings.Contains(v2Err.Error(), "literal") {
				t.Errorf("%s (V2): expected clear diagnostic mentioning WFJSONValues/literal and fileRequest, got: %v", tc.name, v2Err)
			}

			forkBin := roundTripBinary(t)
			tmpDir := t.TempDir()
			srcPath := filepath.Join(tmpDir, "neg_leg_"+tc.name+".cherri")
			if err := os.WriteFile(srcPath, []byte(tc.legacySrc), 0o600); err != nil {
				t.Fatalf("write %s: %v", srcPath, err)
			}
			cmd := exec.Command(forkBin, srcPath, "--legacy", "--skip-sign", "--no-ansi")
			cmd.Dir = tmpDir
			legOutBytes, legErr := cmd.CombinedOutput()
			legOut := string(legOutBytes)
			if legErr == nil {
				t.Fatalf("%s (Legacy): expected compilation to fail closed for non-literal WFJSONValues body, but succeeded!", tc.name)
			}
			if !strings.Contains(legOut, "does not allow variable values") && !strings.Contains(legOut, "literal") {
				t.Errorf("%s (Legacy): unexpected error output: %s", tc.name, legOut)
			}
		})
	}

	// 3. Supported alternative for dynamic JSON/dictionary variable body: fileRequest (WFHTTPBodyType=File, WFRequestVariable)
	t.Run("dynamic-json-variable-via-fileRequest-succeeds", func(t *testing.T) {
		v2Src := `let rawBody = "{\"model\":\"gpt-4\",\"messages\":[{\"role\":\"user\",\"content\":\"hello\"}]}"
let resp = fileRequest(
	"https://example.com/v1/chat/completions",
	method: "POST",
	body: rawBody,
	headers: {
		"Content-Type": "application/json",
		"Authorization": "Bearer sk-test"
	}
)
`
		plistPath := r.compileSource("b1_filerequest_dynamic_json", v2Src)
		doc := loadRoundTripDocument(t, plistPath)

		var textAct, dlAct map[string]any
		for _, a := range doc["WFWorkflowActions"].([]any) {
			m := a.(map[string]any)
			switch m["WFWorkflowActionIdentifier"] {
			case "is.workflow.actions.gettext":
				textAct = m
			case "is.workflow.actions.downloadurl":
				dlAct = m
			}
		}
		if textAct == nil || dlAct == nil {
			t.Fatalf("expected gettext and downloadurl actions in fileRequest plist")
		}
		textUUID := textAct["WFWorkflowActionParameters"].(map[string]any)["UUID"].(string)
		dlParams := dlAct["WFWorkflowActionParameters"].(map[string]any)

		if dlParams["WFHTTPMethod"] != "POST" {
			t.Errorf("expected WFHTTPMethod == 'POST', got %v", dlParams["WFHTTPMethod"])
		}
		if dlParams["WFHTTPBodyType"] != "File" {
			t.Errorf("expected WFHTTPBodyType == 'File', got %v", dlParams["WFHTTPBodyType"])
		}
		reqVar, ok := dlParams["WFRequestVariable"].(map[string]any)
		if !ok || reqVar["WFSerializationType"] != "WFTextTokenAttachment" {
			t.Fatalf("expected WFRequestVariable to be WFTextTokenAttachment, got %v", dlParams["WFRequestVariable"])
		}
		reqVal := reqVar["Value"].(map[string]any)
		if reqVal["OutputUUID"] != textUUID {
			t.Errorf("expected WFRequestVariable.Value.OutputUUID == %q, got %v", textUUID, reqVal["OutputUUID"])
		}
	})
}

// Phase B3 (Asana 1219604360875370):
// Raw fileRequest 4-action reproduction vs chained replaceText -> fileRequest comparison.
func TestP10_B3_FileRequest_Raw4Action_Vs_ChainedReplaceText(t *testing.T) {
	r := newRoundTripRunner(t)

	// 1. Minimal 4-action raw fileRequest shortcut (gettext -> fileRequest -> getDictionary -> show)
	rawSrc := `let bodyText = "{\"prompt\":\"hello world\",\"stream\":false}"
let response = fileRequest(
	"https://httpbin.org/post",
	method: "POST",
	body: bodyText,
	headers: {
		"Content-Type": "application/json",
		"Accept": "application/json"
	}
)
let parsed = getDictionary(response)
show(parsed)
`
	// 2. Chained replaceText -> fileRequest shortcut
	chainedSrc := `let template = "{\"prompt\":\"__PROMPT__\",\"stream\":false}"
let bodyText = replaceText("__PROMPT__", "hello world", template)
let response = fileRequest(
	"https://httpbin.org/post",
	method: "POST",
	body: bodyText,
	headers: {
		"Content-Type": "application/json",
		"Accept": "application/json"
	}
)
let parsed = getDictionary(response)
show(parsed)
`

	rawPlist := r.compileSource("b3_raw_4action", rawSrc)
	chainedPlist := r.compileSource("b3_chained_replace", chainedSrc)

	rawDoc := loadRoundTripDocument(t, rawPlist)
	chainedDoc := loadRoundTripDocument(t, chainedPlist)

	rawActs := rawDoc["WFWorkflowActions"].([]any)
	chainedActs := chainedDoc["WFWorkflowActions"].([]any)

	if len(rawActs) != 4 {
		t.Fatalf("expected exactly 4 actions in raw fileRequest fixture, got %d", len(rawActs))
	}
	if len(chainedActs) != 5 {
		t.Fatalf("expected 5 actions in chained replaceText -> fileRequest fixture, got %d", len(chainedActs))
	}

	rawProducerUUID := rawActs[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"].(string)
	rawDLParams := rawActs[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	chainedProducerUUID := chainedActs[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["UUID"].(string)
	chainedDLParams := chainedActs[2].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)

	for label, p := range map[string]map[string]any{"raw": rawDLParams, "chained": chainedDLParams} {
		if p["WFHTTPMethod"] != "POST" {
			t.Errorf("%s: expected WFHTTPMethod == POST, got %v", label, p["WFHTTPMethod"])
		}
		if p["WFHTTPBodyType"] != "File" {
			t.Errorf("%s: expected WFHTTPBodyType == File, got %v", label, p["WFHTTPBodyType"])
		}
		if p["WFHTTPHeaders"] == nil {
			t.Errorf("%s: expected WFHTTPHeaders to be populated when headers are provided, got nil", label)
		}
		reqVar := p["WFRequestVariable"].(map[string]any)["Value"].(map[string]any)
		if reqVar["Type"] != "ActionOutput" {
			t.Errorf("%s: expected WFRequestVariable Type == ActionOutput, got %v", label, reqVar["Type"])
		}
	}

	rawReqUUID := rawDLParams["WFRequestVariable"].(map[string]any)["Value"].(map[string]any)["OutputUUID"].(string)
	if rawReqUUID != rawProducerUUID {
		t.Errorf("raw fileRequest WFRequestVariable OutputUUID %q != gettext UUID %q", rawReqUUID, rawProducerUUID)
	}

	chainedReqUUID := chainedDLParams["WFRequestVariable"].(map[string]any)["Value"].(map[string]any)["OutputUUID"].(string)
	if chainedReqUUID != chainedProducerUUID {
		t.Errorf("chained fileRequest WFRequestVariable OutputUUID %q != replaceText UUID %q", chainedReqUUID, chainedProducerUUID)
	}
}

// Phase B4 (Asana 1219604672882961):
// Direct HTML rendering via gettext -> setName("...html") -> showWebpage without base64/data:text/html.
func TestP10_B4_DirectHTMLRender_SetNameHtml_ShowWebpage_NoBase64(t *testing.T) {
	r := newRoundTripRunner(t)

	v2Src := `let htmlDoc = "<!DOCTYPE html><html dir=\"rtl\" lang=\"he\"><head><meta charset=\"utf-8\"><style>body { font-family: -apple-system, sans-serif; background: #111827; color: #f9fafb; padding: 24px; } .card { border: 1px solid #374151; border-radius: 12px; padding: 16px; }</style></head><body><div class=\"card\"><h1>תוצאות מסלולים</h1><p>בדיקת תצוגת HTML ישירה בעברית עם גרשיים ו־CSS ללא Base64.</p></div></body></html>"
let namedHtml = setName(htmlDoc, name: "תוצאות מסלולים.html")
showWebpage(namedHtml)
`
	plistPath := r.compileSource("b4_html_preview_direct", v2Src)
	doc := loadRoundTripDocument(t, plistPath)

	acts := doc["WFWorkflowActions"].([]any)
	if len(acts) != 3 {
		t.Fatalf("expected 3 actions (gettext, setitemname, showwebpage), got %d", len(acts))
	}

	for _, a := range acts {
		m := a.(map[string]any)
		id := fmt.Sprint(m["WFWorkflowActionIdentifier"])
		if strings.Contains(id, "base64") {
			t.Fatalf("CRITICAL: HTML preview shortcut must not use base64 action, found %s", id)
		}
	}

	textAct := acts[0].(map[string]any)
	setNameAct := acts[1].(map[string]any)
	showWebAct := acts[2].(map[string]any)

	if textAct["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
		t.Errorf("action[0]: expected is.workflow.actions.gettext, got %v", textAct["WFWorkflowActionIdentifier"])
	}
	if setNameAct["WFWorkflowActionIdentifier"] != "is.workflow.actions.setitemname" {
		t.Errorf("action[1]: expected is.workflow.actions.setitemname, got %v", setNameAct["WFWorkflowActionIdentifier"])
	}
	if showWebAct["WFWorkflowActionIdentifier"] != "is.workflow.actions.showwebpage" {
		t.Errorf("action[2]: expected is.workflow.actions.showwebpage, got %v", showWebAct["WFWorkflowActionIdentifier"])
	}

	setNameParams := setNameAct["WFWorkflowActionParameters"].(map[string]any)
	if setNameParams["WFName"] != "תוצאות מסלולים.html" {
		t.Errorf("expected WFName == 'תוצאות מסלולים.html', got %v", setNameParams["WFName"])
	}
	if dontInc, exists := setNameParams["WFDontIncludeFileExtension"]; exists && dontInc == true {
		t.Errorf("expected WFDontIncludeFileExtension to be false/omitted so .html extension is preserved, got %v", dontInc)
	}

	setNameUUID := setNameParams["UUID"].(string)
	showWebParams := showWebAct["WFWorkflowActionParameters"].(map[string]any)
	wfURL, ok := showWebParams["WFURL"].(map[string]any)
	if !ok {
		t.Fatalf("expected showwebpage WFURL parameter map, got %T (%v)", showWebParams["WFURL"], showWebParams["WFURL"])
	}
	// Verify showwebpage references the output of setitemname
	var foundSetNameRef bool
	if wfURL["WFSerializationType"] == "WFTextTokenString" {
		atts := wfURL["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
		if att0, ok := atts["{0, 1}"].(map[string]any); ok && att0["OutputUUID"] == setNameUUID {
			foundSetNameRef = true
		}
	} else if wfURL["WFSerializationType"] == "WFTextTokenAttachment" {
		val := wfURL["Value"].(map[string]any)
		if val["OutputUUID"] == setNameUUID {
			foundSetNameRef = true
		}
	}
	if !foundSetNameRef {
		t.Errorf("showwebpage WFURL did not reference setitemname UUID %q: %v", setNameUUID, wfURL)
	}
}

func TestP10_IPadAcceptanceKit_CompileAndStructure(t *testing.T) {
	r := newRoundTripRunner(t)
	files, err := filepath.Glob(filepath.Join("tests", "ipad_acceptance", "*.cherri"))
	if err != nil {
		t.Fatalf("glob tests/ipad_acceptance/*.cherri: %v", err)
	}
	if len(files) < 9 {
		t.Fatalf("expected at least 9 iPad acceptance fixtures in tests/ipad_acceptance/, found %d", len(files))
	}

	for _, fpath := range files {
		base := strings.TrimSuffix(filepath.Base(fpath), ".cherri")
		t.Run(base, func(t *testing.T) {
			srcBytes, err := os.ReadFile(fpath)
			if err != nil {
				t.Fatalf("read %s: %v", fpath, err)
			}
			plistPath := r.compileSource("ipad_"+base, string(srcBytes))
			doc := loadRoundTripDocument(t, plistPath)
			acts, ok := doc["WFWorkflowActions"].([]any)
			if !ok || len(acts) == 0 {
				t.Fatalf("%s: compiled plist has no WFWorkflowActions", base)
			}
			decomp := r.decompileShortcut("ipad_decomp_"+base, plistPath)
			if len(strings.TrimSpace(decomp)) == 0 {
				t.Fatalf("%s: decompiled output is empty", base)
			}
			recompPlist := r.compileSource("ipad_recomp_"+base, decomp)
			recompDoc := loadRoundTripDocument(t, recompPlist)
			recompActs, ok := recompDoc["WFWorkflowActions"].([]any)
			if !ok || len(recompActs) != len(acts) {
				t.Fatalf("%s: recompiled action count %d != original %d", base, len(recompActs), len(acts))
			}
		})
	}
}
