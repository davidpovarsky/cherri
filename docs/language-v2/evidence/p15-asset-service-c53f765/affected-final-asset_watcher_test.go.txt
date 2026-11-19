package lsp

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func TestLSPAssetWatcherInvalidatesPreservedMetadata(t *testing.T) {
	dir := t.TempDir()
	uri := filepath.Join(dir, "entry.cherri")
	path := filepath.Join(dir, "payload.dat")
	stamp := time.Unix(1700200000, 0)
	write := func(data string) {
		t.Helper()
		if err := os.WriteFile(path, []byte(data), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	write("PUBLIC_INITIAL")
	cache := assets.New(0, 0, 0)
	var output bytes.Buffer
	server := NewServer(strings.NewReader(""), &output, nil)
	server.svc = service.NewServiceWithAssetCache(nil, cache)
	send := func(method string, params any) {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		server.handleMessage(&rpcMessage{JSONRPC: "2.0", Method: method, Params: raw})
	}
	send("textDocument/didOpen", map[string]any{"textDocument": map[string]any{"uri": uri, "version": 1, "text": "embedFile(\"payload.dat\")\n"}})
	if cache.Metrics().Reads != 1 {
		t.Fatal("open did not read the actual local asset")
	}
	initial := server.svc.AssetDependencies(uri)
	if len(initial) != 1 {
		t.Fatal("missing initial dependency")
	}
	write("PUBLIC_CHANGED") // identical length and timestamp
	send("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": path, "type": 2}}})
	updated := server.svc.AssetDependencies(uri)
	if len(updated) != 1 || updated[0].SHA256 == initial[0].SHA256 || cache.Metrics().Reads != 2 || cache.Metrics().Invalidations != 1 {
		t.Fatal("actual LSP file event did not invalidate and republish the dependency")
	}
	if strings.Count(output.String(), "textDocument/publishDiagnostics") != 2 {
		t.Fatal("file event did not republish open document diagnostics")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	send("workspace/didChangeWatchedFiles", map[string]any{"changes": []any{map[string]any{"uri": path, "type": 3}}})
	if !strings.Contains(output.String(), "E_WORKSPACE_ACCESS") || len(server.svc.AssetDependencies(uri)) != 0 {
		t.Fatal("deleted asset remained silently cached after LSP event")
	}
}
