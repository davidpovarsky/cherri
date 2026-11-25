package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/assets"
)

func nativeDocumentFixture(t *testing.T) (string, *mobileDocumentContext) {
	t.Helper()
	base := t.TempDir()
	if err := os.Mkdir(filepath.Join(base, "workspace"), 0700); err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(filepath.Join(base, "workspace"))
	if err != nil {
		t.Fatal(err)
	}
	return root, &mobileDocumentContext{
		URI:        (&url.URL{Scheme: "file", Path: filepath.ToSlash(filepath.Join(root, "entry.cherri"))}).String(),
		DocumentID: "6717e199-6f91-45c4-8ed7-9a12cb4b1770", Revision: 7, AssetRevision: 3,
	}
}

func nativeDocumentCompile(t *testing.T, source, mode string, document *mobileDocumentContext, cache *assets.Cache) mobileCompileResponse {
	t.Helper()
	response := compileForMobileDocumentWithOptions(source, "DocumentPreview", mode, false,
		CompileOptions{BuildProfile: BuildProfileDebug, AssetCache: cache}, document)
	if !response.OK || response.PlistBase64 == "" || response.SignedBase64 != "" {
		t.Fatalf("actual %s document compile failed: %+v", mode, response)
	}
	return response
}

func nativeDocumentTextValues(t *testing.T, response mobileCompileResponse) []string {
	t.Helper()
	data, err := base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	workflow, err := decodeShortcutRawMap(data)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := workflow["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing real native actions")
	}
	var result []string
	for _, item := range raw {
		action := item.(map[string]any)
		parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
		if text, ok := parameters["WFTextActionText"].(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func nativeDocumentAssertAsset(t *testing.T, response mobileCompileResponse, expected, forbidden []byte) {
	t.Helper()
	values := nativeDocumentTextValues(t, response)
	count := 0
	for _, value := range values {
		if value == base64.StdEncoding.EncodeToString(expected) {
			count++
		}
		if len(forbidden) != 0 && value == base64.StdEncoding.EncodeToString(forbidden) {
			t.Fatal("embedded bytes came from the wrong document directory")
		}
	}
	if count != 1 {
		t.Fatalf("expected one exact encoded asset, got %d in %v", count, values)
	}
}

func TestP15BRG28NativeDocumentRelativeAssetsAndCache(t *testing.T) {
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			root, document := nativeDocumentFixture(t)
			first := bytes.Repeat([]byte{0, 255, 65, 128, 10}, 4096)
			second := bytes.Repeat([]byte{1, 254, 66, 127, 10}, 4096)
			path := filepath.Join(root, "payload.dat")
			p15AssetWrite(t, path, first, 1)
			cache := assets.New(0, 0, 0)
			source := "embedFile(\"payload.dat\")\n"
			beforeCWD, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			response := nativeDocumentCompile(t, source, mode, document, cache)
			nativeDocumentAssertAsset(t, response, first, nil)
			firstMetrics := cache.Metrics()
			if firstMetrics.Reads != 1 || firstMetrics.Encodes != 1 {
				t.Fatalf("missing shared cache read: %+v", firstMetrics)
			}
			next := *document
			next.Revision++
			response = nativeDocumentCompile(t, source+"// unrelated edit\n", mode, &next, cache)
			nativeDocumentAssertAsset(t, response, first, nil)
			warm := cache.Metrics()
			if warm.Reads != 1 || warm.Encodes != 1 || warm.Hits <= firstMetrics.Hits {
				t.Fatalf("unrelated source edit repeated heavy asset work: before=%+v after=%+v", firstMetrics, warm)
			}
			p15AssetWrite(t, path, second, 2)
			next.AssetRevision++
			response = nativeDocumentCompile(t, source, mode, &next, cache)
			nativeDocumentAssertAsset(t, response, second, first)
			if cache.Metrics().Reads != 2 {
				t.Fatal("changed asset was not read once")
			}
			afterCWD, err := os.Getwd()
			if err != nil || afterCWD != beforeCWD {
				t.Fatal("native compile changed process cwd")
			}

			// Same bytes and same display name in another workspace must not
			// reuse the first document's asset or cache entry.
			otherRoot, other := nativeDocumentFixture(t)
			other.DocumentID = "43719a47-8fd0-4c13-830c-aa792d41c59a"
			p15AssetWrite(t, filepath.Join(otherRoot, "payload.dat"), first, 1)
			response = nativeDocumentCompile(t, source, mode, other, cache)
			nativeDocumentAssertAsset(t, response, first, second)
		})
	}
}

func TestP15BRG28NativeDocumentImportedAssetOrigin(t *testing.T) {
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			root, document := nativeDocumentFixture(t)
			expected, wrong := []byte("PUBLIC_NESTED_ORIGIN"), []byte("PUBLIC_WRONG_ROOT")
			p15AssetWrite(t, filepath.Join(root, "payload.dat"), wrong, 1)
			p15AssetWrite(t, filepath.Join(root, "sub", "payload.dat"), expected, 1)
			var source, helper string
			if mode == "v2" {
				source = "import \"sub/helper.cherri\" as helper\nlet value = helper.load()\n"
				helper = "function load() -> AnyContent { return embedFile(\"payload.dat\") }\n"
			} else {
				source = "#include 'sub/helper.cherri'\nconst value = load()\n"
				helper = "function load(): text {\n    const data = embedFile(\"payload.dat\")\n    output(data)\n}\n"
			}
			p15AssetWrite(t, filepath.Join(root, "sub", "helper.cherri"), []byte(helper), 1)
			response := nativeDocumentCompile(t, source, mode, document, assets.New(0, 0, 0))
			nativeDocumentAssertAsset(t, response, expected, wrong)
		})
	}
}

func TestP15BRG28NativeDocumentLegacyCopyAndNestedIncludeOrigin(t *testing.T) {
	root, document := nativeDocumentFixture(t)
	expected, wrong := []byte("PUBLIC_COPY_ORIGIN"), []byte("PUBLIC_WRONG_ENTRY")
	p15AssetWrite(t, filepath.Join(root, "payload.dat"), wrong, 1)
	p15AssetWrite(t, filepath.Join(root, "sub", "deep", "payload.dat"), expected, 1)
	p15AssetWrite(t, filepath.Join(root, "sub", "first.cherri"), []byte("#include 'deep/second.cherri'\n"), 1)
	p15AssetWrite(t, filepath.Join(root, "sub", "deep", "second.cherri"), []byte("copy content {\n    embedFile(\"payload.dat\")\n}\n"), 1)
	response := nativeDocumentCompile(t, "#include 'sub/first.cherri'\npaste content\n", "legacy", document, assets.New(0, 0, 0))
	nativeDocumentAssertAsset(t, response, expected, wrong)
}

func TestP15BRG28NativeDocumentMapAndRevisionBinding(t *testing.T) {
	_, document := nativeDocumentFixture(t)
	source := "// keep café 😀\nwait(2)\nshow(\"keep\")\n"
	response := nativeDocumentCompile(t, source, "v2", document, nil)
	if response.PreviewSourceMap == nil {
		t.Fatal("actual compile omitted mapping")
	}
	raw, err := json.Marshal(response.PreviewSourceMap)
	if err != nil {
		t.Fatal(err)
	}
	var mapping map[string]any
	if err = json.Unmarshal(raw, &mapping); err != nil {
		t.Fatal(err)
	}
	wanted, _ := json.Marshal(document)
	var wantedMap any
	if err = json.Unmarshal(wanted, &wantedMap); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(mapping["document"], wantedMap) {
		t.Fatalf("mapping does not bind exact native document context: %s", raw)
	}
	index := -1
	for _, entry := range response.PreviewSourceMap.Entries {
		if entry.ActionIdentifier == "is.workflow.actions.delay" {
			index = entry.ActionIndex
		}
	}
	if index < 0 {
		t.Fatal("real Wait origin missing")
	}
	request := rp41PreviewEditRequest(source, response, index, "WFDelayTime", "3")
	encoded, _ := json.Marshal(request)
	var wire map[string]any
	if err = json.Unmarshal(encoded, &wire); err != nil {
		t.Fatal(err)
	}
	wire["document"] = wantedMap
	call := func() (bool, string) {
		data, e := json.Marshal(wire)
		if e != nil {
			t.Fatal(e)
		}
		result := editPreviewSourceForMobile(string(data))
		return result.OK, result.Source
	}
	if ok, changed := call(); !ok || changed != strings.Replace(source, "wait(2)", "wait(3)", 1) {
		t.Fatalf("valid exact document edit rejected or changed unrelated text: %v %q", ok, changed)
	}
	for _, field := range []string{"uri", "documentId", "revision", "assetRevision"} {
		t.Run(field, func(t *testing.T) {
			copyMap := map[string]any{}
			for k, v := range wantedMap.(map[string]any) {
				copyMap[k] = v
			}
			if text, ok := copyMap[field].(string); ok {
				copyMap[field] = text + "-changed"
			} else {
				copyMap[field] = copyMap[field].(float64) + 1
			}
			wire["document"] = copyMap
			if ok, changed := call(); ok || changed != source {
				t.Fatal("stale/relabelled context changed source")
			}
			wire["document"] = wantedMap
		})
	}
	delete(wire, "document")
	if ok, changed := call(); ok || changed != source {
		t.Fatal("stripped document context was accepted")
	}
}

func TestP15BRG28NativeDocumentContextValidation(t *testing.T) {
	_, good := nativeDocumentFixture(t)
	for _, uri := range []string{"https://example.invalid/source.cherri", "file://remote/source.cherri", "file:relative.cherri", "file:///tmp/source.cherri?query=1", "file:///tmp/source.cherri#fragment", "file:///tmp/%00source.cherri"} {
		t.Run(uri, func(t *testing.T) {
			document := *good
			document.URI = uri
			response := compileForMobileDocumentWithOptions("show(\"never sign\")\n", "Name", "v2", true, CompileOptions{}, &document)
			if response.OK || response.PlistBase64 != "" || response.SignedBase64 != "" || !strings.Contains(response.Error, "NATIVE_DOCUMENT_CONTEXT_INVALID") {
				t.Fatalf("invalid context reached output/signing: %+v", response)
			}
		})
	}
	request := mobileDocumentCompileRequest{Source: "show(\"old route\")\n", Name: "Old", Mode: "v2", BuildProfile: BuildProfileDebug}
	raw, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	response := compileDocumentForMobile(string(raw))
	if !response.OK || response.PreviewSourceMap == nil {
		t.Fatalf("absent-context API failed: %+v", response)
	}
	mapBytes, err := json.Marshal(response.PreviewSourceMap)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(mapBytes, []byte("\"document\"")) {
		t.Fatal("absent context changed original map serialization")
	}
	for _, suffix := range []string{" {}", " false"} {
		rejected := compileDocumentForMobile(string(raw) + suffix)
		if rejected.OK || rejected.PlistBase64 != "" || rejected.SignedBase64 != "" {
			t.Fatal("trailing JSON reached compiler")
		}
	}
}

func TestP15BRG28NativeDocumentResourceReauthorization(t *testing.T) {
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			root, document := nativeDocumentFixture(t)
			path := filepath.Join(root, "payload.dat")
			payload := []byte("PUBLIC_AUTHORIZED")
			p15AssetWrite(t, path, payload, 1)
			cache := assets.New(0, 0, 0)
			source := "embedFile(\"payload.dat\")\n"
			nativeDocumentAssertAsset(t, nativeDocumentCompile(t, source, mode, document, cache), payload, nil)
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(filepath.Dir(root), "outside.dat")
			p15AssetWrite(t, outside, []byte("PUBLIC_OUTSIDE"), 1)
			cases := []struct {
				name    string
				prepare func()
				source  string
			}{
				{"missing", func() {}, source},
				{"symlink_escape", func() {
					if err := os.Symlink(outside, path); err != nil {
						t.Fatal(err)
					}
				}, source},
				{"parent_escape", func() {
					if err := os.Remove(path); err != nil {
						t.Fatal(err)
					}
				}, "embedFile(\"../outside.dat\")\n"},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					tc.prepare()
					result := compileForMobileDocumentWithOptions(tc.source, "DocumentPreview", mode, false, CompileOptions{AssetCache: cache}, document)
					if result.OK || result.PlistBase64 != "" || result.SignedBase64 != "" {
						t.Fatalf("resource guard released output: %+v", result)
					}
				})
			}
		})
	}
}

// Establish source/data validity through the actual pre-existing entry points.
// This control does not inherit any document-context result or native credit.
func TestP15BRG28NativeDocumentExistingRoutePositiveControl(t *testing.T) {
	root, _ := nativeDocumentFixture(t)
	payload := []byte("PUBLIC_ABSOLUTE_ROUTE_CONTROL")
	path := filepath.Join(root, "payload.dat")
	p15AssetWrite(t, path, payload, 1)
	raw, err := CompileSourceToPlistWithOptions(filepath.Join(root, "entry.cherri"), "embedFile(\"payload.dat\")\n", CompileOptions{})
	if err != nil {
		t.Fatal("actual existing canonical fixture control:", err)
	}
	nativeDocumentAssertAsset(t, mobileCompileResponse{PlistBase64: base64.StdEncoding.EncodeToString(raw)}, payload, nil)
	legacy := compileForMobileModeWithOptions("embedFile("+strconv.Quote(path)+")\n", "OldLegacyControl", "legacy", false, CompileOptions{})
	if !legacy.OK {
		t.Fatalf("actual existing Legacy absolute-file control: %+v", legacy)
	}
	nativeDocumentAssertAsset(t, legacy, payload, nil)
}

func TestP15BRG28NativeDocumentUnsavedHasNoResourceAuthority(t *testing.T) {
	root, document := nativeDocumentFixture(t)
	path := filepath.Join(root, "payload.dat")
	p15AssetWrite(t, path, []byte("PUBLIC_MUST_NOT_READ"), 1)
	document.URI = ""
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			result := compileForMobileDocumentWithOptions("embedFile("+strconv.Quote(path)+")\n", "Unsaved", mode, false, CompileOptions{}, document)
			if result.OK || result.PlistBase64 != "" || result.SignedBase64 != "" {
				t.Fatal("unsaved document acquired filesystem authority")
			}
		})
	}
}
