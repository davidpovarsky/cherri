package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

func p15AssetSHA(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func p15AssetWrite(t *testing.T, path string, data []byte, version int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1700100000+int64(version), 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func p15AssetAnalyze(t *testing.T, svc *service.Service, uri string) {
	t.Helper()
	diagnostics, _ := svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatalf("healthy asset document rejected: %v", diagnostic)
		}
	}
}

func TestP15S03SharedAssetCacheAcrossServiceEditsAndCanonicalCompile(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		kind string
	}{
		{"large_text", bytes.Repeat([]byte("PUBLIC_UTF8_TEXT\n"), 1<<19), "text"},
		{"large_binary", bytes.Repeat([]byte{0, 255, 128, 'A', 1, 2, 3, 4}, 1<<20), "binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			uri := filepath.Join(dir, "entry.cherri")
			p15AssetWrite(t, filepath.Join(dir, "payload.dat"), tc.data, 1)
			cache := assets.New(0, 0, 0)
			svc := service.NewServiceWithAssetCache(nil, cache)
			source := "embedFile(\"payload.dat\")\n"
			svc.OpenDocument(uri, 1, source)
			p15AssetAnalyze(t, svc, uri)
			for version := 2; version <= 26; version++ {
				svc.ChangeDocument(uri, version, source+"// unrelated edit "+strings.Repeat("x", version)+"\n")
				p15AssetAnalyze(t, svc, uri)
			}
			dependencies := svc.AssetDependencies(uri)
			if len(dependencies) != 1 || dependencies[0].SHA256 != p15AssetSHA(tc.data) || dependencies[0].Kind != tc.kind || dependencies[0].Base64 != "" {
				t.Fatal("editor dependency metadata lost file identity or retained encoded bytes")
			}
			out, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
			if err != nil {
				t.Fatal(err)
			}
			decoded, err := base64.StdEncoding.DecodeString(p15AssetText(t, out))
			if err != nil || !bytes.Equal(decoded, tc.data) {
				t.Fatal("canonical compiler did not emit the shared asset bytes")
			}
			metrics := cache.Metrics()
			if metrics.Reads != 1 || metrics.Encodes != 1 || metrics.ReadBytes != uint64(len(tc.data)) || metrics.Hits < 51 {
				t.Fatalf("unrelated editor work or canonical compile repeated asset I/O/encoding: %+v", metrics)
			}
			t.Logf("S03_ASSET_REUSE kind=%s bytes=%d sha256=%s reads=%d encodes=%d hits=%d", tc.kind, len(tc.data), p15AssetSHA(tc.data), metrics.Reads, metrics.Encodes, metrics.Hits)
		})
	}
}

func TestP15S03EditorAssetChangesAndFailureRecovery(t *testing.T) {
	dir := t.TempDir()
	uri, path := filepath.Join(dir, "entry.cherri"), filepath.Join(dir, "payload.dat")
	first, second := []byte("PUBLIC_INITIAL"), []byte("PUBLIC_CHANGED")
	p15AssetWrite(t, path, first, 1)
	cache := assets.New(0, 0, 0)
	svc := service.NewServiceWithAssetCache(nil, cache)
	source := "embedFile(\"payload.dat\")\n"
	svc.OpenDocument(uri, 1, source)
	p15AssetAnalyze(t, svc, uri)
	p15AssetWrite(t, path, second, 2)
	p15AssetAnalyze(t, svc, uri)
	if got := svc.AssetDependencies(uri); len(got) != 1 || got[0].SHA256 != p15AssetSHA(second) {
		t.Fatal("asset edit did not refresh editor dependency")
	}
	if cache.Metrics().Reads != 2 {
		t.Fatal("asset edit failed to invalidate heavy cache")
	}

	// Preserve both size and mtime: the actual host notification must invalidate.
	preserved := []byte("PUBLIC_REPLACE")
	p15AssetWrite(t, path, preserved, 2)
	uris := svc.InvalidateAssets()
	if len(uris) != 1 || uris[0] != uri {
		t.Fatal("watcher did not return the open document")
	}
	p15AssetAnalyze(t, svc, uri)
	if got := svc.AssetDependencies(uri); len(got) != 1 || got[0].SHA256 != p15AssetSHA(preserved) || cache.Metrics().Reads != 3 {
		t.Fatal("explicit file event did not refresh preserved metadata")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	diagnostics, _ := svc.Analyze(uri)
	if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeWorkspaceAccess || len(svc.AssetDependencies(uri)) != 0 {
		t.Fatalf("missing cached asset silently retained: %+v", diagnostics)
	}
	out, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
	if err == nil || len(out) != 0 || !strings.Contains(err.Error(), analysis.CodeWorkspaceAccess) {
		t.Fatal("missing asset emitted an artifact or wrong error")
	}
	p15AssetWrite(t, path, first, 3)
	p15AssetAnalyze(t, svc, uri)
	out, err = CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
	if err != nil || p15AssetText(t, out) != base64.StdEncoding.EncodeToString(first) {
		t.Fatal("failure poisoned the next compile")
	}

	before := cache.Metrics().Reads
	svc.ChangeDocument(uri, 2, "let =\n"+source)
	if diagnostics, _ := svc.Analyze(uri); len(diagnostics) == 0 {
		t.Fatal("invalid edit unexpectedly accepted")
	}
	if cache.Metrics().Reads != before {
		t.Fatal("syntax-error edit forced a heavy asset reread")
	}
	svc.ChangeDocument(uri, 3, source)
	p15AssetAnalyze(t, svc, uri)
	if cache.Metrics().Reads != before {
		t.Fatal("recovered unrelated edit discarded valid asset bytes")
	}
}

func TestP15S03ShadowedNameCannotReadAssets(t *testing.T) {
	dir := t.TempDir()
	for _, source := range []string{
		"function embedFile(path: Text) -> Text { return path }\nlet value = embedFile(\"../not-authorized.txt\")\n",
		"action embedFile(path: Text) -> Text { identifier: \"is.workflow.actions.gettext\", primary: \"path\", bindings: { path: { key: \"WFTextActionText\", codec: \"text\" } } }\nlet value = embedFile(\"../not-authorized.txt\")\n",
	} {
		cache := assets.New(0, 0, 0)
		svc := service.NewServiceWithAssetCache(nil, cache)
		uri := filepath.Join(dir, "entry.cherri")
		svc.OpenDocument(uri, 1, source)
		p15AssetAnalyze(t, svc, uri)
		if cache.Metrics().MetadataChecks != 0 || len(svc.AssetDependencies(uri)) != 0 {
			t.Fatal("a shadowed action name authorized a local resource read")
		}
	}
}

func TestP15S03ImportedAssetKeepsEntryRootAndModuleSource(t *testing.T) {
	dir := t.TempDir()
	uri := filepath.Join(dir, "entry.cherri")
	expected := []byte("PUBLIC_MODULE_ASSET")
	p15AssetWrite(t, filepath.Join(dir, "payload.dat"), []byte("PUBLIC_WRONG_ROOT"), 1)
	p15AssetWrite(t, filepath.Join(dir, "sub", "payload.dat"), expected, 1)
	p15AssetWrite(t, filepath.Join(dir, "sub", "helper.cherri"), []byte("function load() -> AnyContent { return embedFile(\"payload.dat\") }\n"), 1)
	source := "import \"sub/helper.cherri\" as helper\nlet value = helper.load()\n"
	cache := assets.New(0, 0, 0)
	svc := service.NewServiceWithAssetCache(nil, cache)
	svc.OpenDocument(uri, 1, source)
	p15AssetAnalyze(t, svc, uri)
	dependencies := svc.AssetDependencies(uri)
	if len(dependencies) != 1 || dependencies[0].RelativePath != "sub/payload.dat" || dependencies[0].SHA256 != p15AssetSHA(expected) {
		t.Fatalf("module dependency resolved against wrong directory: %+v", dependencies)
	}
	out, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
	if err != nil {
		t.Fatal(err)
	}
	var workflow Shortcut
	if _, err := plist.Unmarshal(out, &workflow); err != nil {
		t.Fatal(err)
	}
	found := 0
	for _, action := range workflow.WFWorkflowActions {
		if text, ok := action.WFWorkflowActionParameters["WFTextActionText"].(string); ok {
			if text == base64.StdEncoding.EncodeToString(expected) {
				found++
			}
			if text == base64.StdEncoding.EncodeToString([]byte("PUBLIC_WRONG_ROOT")) {
				t.Fatal("compiler opened root asset for imported call")
			}
		}
	}
	if found != 1 || cache.Metrics().Reads != 1 {
		t.Fatalf("module source context or shared cache lost: found=%d metrics=%+v", found, cache.Metrics())
	}
}

func TestP15S03ModuleCallControl(t *testing.T) {
	for _, alias := range []string{"helper", "Helper"} {
		t.Run(alias, func(t *testing.T) {
			dir := t.TempDir()
			p15AssetWrite(t, filepath.Join(dir, "sub", "helper.cherri"), []byte("function load() -> Text { return \"PUBLIC_MODULE_LITERAL\" }\n"), 1)
			source := "import \"sub/helper.cherri\" as " + alias + "\nlet value = " + alias + ".load()\n"
			if _, err := CompileSourceToPlist(filepath.Join(dir, "entry.cherri"), source); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15S03BuiltinModuleAliasUsesCanonicalAsset(t *testing.T) {
	for _, alias := range []string{"", "files", "Files"} {
		name := alias
		if name == "" {
			name = "unqualified"
		}
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			uri := filepath.Join(dir, "entry.cherri")
			payload := []byte("PUBLIC_QUALIFIED_BUILTIN_ASSET")
			p15AssetWrite(t, filepath.Join(dir, "payload.dat"), payload, 1)
			prefix, callee := "", "embedFile"
			if alias != "" {
				prefix = "import \"builtin\" as " + alias + "\n"
				callee = alias + ".embedFile"
			}
			source := prefix + callee + "(\"payload.dat\")\n"
			cache := assets.New(0, 0, 0)
			svc := service.NewServiceWithAssetCache(nil, cache)
			svc.OpenDocument(uri, 1, source)
			p15AssetAnalyze(t, svc, uri)
			dependencies := svc.AssetDependencies(uri)
			if len(dependencies) != 1 || dependencies[0].SHA256 != p15AssetSHA(payload) {
				t.Errorf("resolved built-in asset identity was lost: dependencies=%+v", dependencies)
			}
			out, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
			if err != nil {
				t.Errorf("canonical built-in alias failed compilation: %v", err)
			} else if p15AssetText(t, out) != base64.StdEncoding.EncodeToString(payload) {
				t.Error("qualified builtin produced different bytes")
			}
			if alias == "" {
				return
			}
			svc.ChangeDocument(uri, 2, prefix+callee+"(\"../outside.dat\")\n")
			diagnostics, _ := svc.Analyze(uri)
			if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeWorkspaceAccess {
				t.Errorf("qualified asset escaped service authorization: %+v", diagnostics)
			}
			out, err = CompileSourceToPlistWithOptions(uri, prefix+callee+"(\"../outside.dat\")\n", CompileOptions{AssetCache: cache})
			if err == nil || len(out) != 0 || !strings.Contains(err.Error(), analysis.CodeWorkspaceAccess) {
				t.Errorf("qualified asset did not fail closed before emission: bytes=%d err=%v", len(out), err)
			}
		})
	}
}
