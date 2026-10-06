package lsp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func TestLSPServerInitializeAndDidOpen(t *testing.T) {
	// Prepare input requests: initialize followed by didOpen
	var inBuf bytes.Buffer
	var outBuf bytes.Buffer

	// 1. initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]interface{}{},
	}
	body, _ := json.Marshal(initReq)
	fmt.Fprintf(&inBuf, "Content-Length: %d\r\n\r\n%s", len(body), body)

	// 2. didOpen request with invalid code (reassigning let)
	didOpen := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "textDocument/didOpen",
		"params": map[string]interface{}{
			"textDocument": map[string]interface{}{
				"uri":        "file:///test.cherri",
				"version":    1,
				"languageId": "cherri",
				"text":       "let x = 1\nx = 2\n",
			},
		},
	}
	body2, _ := json.Marshal(didOpen)
	fmt.Fprintf(&inBuf, "Content-Length: %d\r\n\r\n%s", len(body2), body2)

	server := NewServer(&inBuf, &outBuf, schema.DefaultRegistry())
	err := server.Run()
	if err != nil && err != io.EOF {
		t.Fatalf("unexpected LSP server error: %v", err)
	}

	outStr := outBuf.String()
	if !strings.Contains(outStr, "capabilities") {
		t.Errorf("expected server capabilities in response, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "publishDiagnostics") {
		t.Errorf("expected publishDiagnostics notification, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "E_ASSIGN_IMMUTABLE") {
		t.Errorf("expected E_ASSIGN_IMMUTABLE in diagnostics, got:\n%s", outStr)
	}
}
