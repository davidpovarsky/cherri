package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"howett.net/plist"
)

func TestP15RP04CheckedCompileRejectsSemanticErrors(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var requests atomic.Int32
	var signer = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"message": "P15_RP04_SIGNER_OBSERVER"}})
	}))
	defer signer.Close()
	var originalHook = backendEmitHook
	defer func() { backendEmitHook = originalHook }()
	var emitted int
	backendEmitHook = func(call backend.ResolvedCall) error { emitted++; return nil }
	var cases = []struct{ name, invalid, code string }{
		{"immutable_reassignment", "let value = 1\nvalue = 2\n", analysis.CodeAssignImmutable},
		{"wrong_type", "let value: Number = \"not a number\"\n", analysis.CodeArgumentType},
		{"duplicate_label", "number(1, number: 2)\n", analysis.CodeDuplicateArgument},
		{"unknown_label", "number(number: 1, mystery: 2)\n", analysis.CodeUnknownArgument},
		{"missing_argument", "number()\n", analysis.CodeMissingArgument},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var source = "text(\"MUST_NOT_EMIT_BEFORE_ERROR\")\n" + tc.invalid
			if _, errors := p15ParseProgram(source); len(errors) != 0 {
				t.Fatalf("test must exercise semantic analysis after successful parsing: %v", errors)
			}
			emitted = 0
			var raw, err = CompileSourceToPlist("semantic-rejection.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) || emitted != 0 {
				t.Fatalf("embedded compile bypassed full analysis: bytes=%d emitted=%d error=%v", len(raw), emitted, err)
			}
			var mobile = compileForMobileMode(source, "Semantic rejection", "v2", false)
			if mobile.OK || mobile.PlistBase64 != "" || mobile.SignedBase64 != "" || !strings.Contains(mobile.Error, tc.code) || emitted != 0 {
				t.Fatalf("mobile embedded route bypassed full analysis: response=%+v emitted=%d", mobile, emitted)
			}
			var dir = t.TempDir()
			var input, output = filepath.Join(dir, "source.cherri"), filepath.Join(dir, "retained.shortcut")
			var original = []byte("P15_PREEXISTING_DESTINATION\x00\xff")
			p15SchemaWrite(t, input, []byte(source))
			p15SchemaWrite(t, output, original)
			var before = p15DirectoryDigest(t, dir)
			var out, stderr, cliErr = p15RunCLI(t, binary, dir, nil, "check", input, "--json", "--hubsign", "--signing-server="+signer.URL+"/sign?response=json", "--output="+output)
			var result protocol.AnalyzeResponse
			if cliErr == nil || json.Unmarshal(out, &result) != nil || result.Valid {
				t.Fatalf("check accepted semantic error: err=%v stdout=%s stderr=%s", cliErr, out, stderr)
			}
			var matched bool
			for _, diagnostic := range result.Diagnostics {
				if diagnostic.Code == tc.code && diagnostic.Severity == "ERROR" {
					matched = true
				}
			}
			if !matched {
				t.Fatalf("check returned the wrong diagnostic: %+v", result.Diagnostics)
			}
			for _, args := range [][]string{
				{"compile", input, "--hubsign", "--signing-server=" + signer.URL + "/sign?response=json", "--output=" + output, "--no-ansi"},
				{input, "--hubsign", "--signing-server=" + signer.URL + "/sign?response=json", "--output=" + output, "--no-ansi"},
			} {
				out, stderr, cliErr = p15RunCLI(t, binary, dir, nil, args...)
				if cliErr == nil || !strings.Contains(string(out)+string(stderr), tc.code) {
					t.Fatalf("public compile bypassed analysis: err=%v stdout=%s stderr=%s", cliErr, out, stderr)
				}
			}
			var after = p15DirectoryDigest(t, dir)
			if len(after) != len(before) {
				t.Fatal("rejected request added or removed a source/output artifact")
			}
			for name, digest := range before {
				if after[name] != digest {
					t.Fatalf("rejected request changed %s", name)
				}
			}
			if requests.Load() != 0 {
				t.Fatalf("invalid source reached signing: %d", requests.Load())
			}
		})
	}
	var validSource = "text(\"P15_CHECKED_COMPILE_RESTORED\")\n"
	emitted = 0
	var raw, err = CompileSourceToPlist("corrected.cherri", validSource)
	if err != nil || emitted != 1 {
		t.Fatalf("restored compiler/hook did not run: emitted=%d error=%v", emitted, err)
	}
	var native map[string]any
	if _, err := plist.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	var actions, ok = native["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 1 {
		t.Fatalf("corrected source had unexpected native effects: %#v", native)
	}
	var dir = t.TempDir()
	var input, output = filepath.Join(dir, "corrected.cherri"), filepath.Join(dir, "retained.shortcut")
	var original = []byte("P15_RETAIN_ON_SIGNING_ERROR")
	p15SchemaWrite(t, input, []byte(validSource))
	p15SchemaWrite(t, output, original)
	var stderr []byte
	_, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--hubsign", "--signing-server="+signer.URL+"/sign?response=json", "--output="+output)
	if err == nil || requests.Load() != 1 || !strings.Contains(string(stderr), "P15_RP04_SIGNER_OBSERVER") {
		t.Fatalf("real signer control was not reached once: requests=%d error=%v stderr=%s", requests.Load(), err, stderr)
	}
	var retained, readErr = os.ReadFile(output)
	if readErr != nil || !bytes.Equal(retained, original) {
		t.Fatal("failed signing changed the existing destination")
	}
	_, stderr, err = p15RunCLI(t, binary, dir, nil, "compile", input, "--skip-sign", "--output="+output)
	if err != nil {
		t.Fatalf("restored public unsigned compile failed: %v %s", err, stderr)
	}
	raw, err = os.ReadFile(output)
	if err != nil || bytes.Equal(raw, original) {
		t.Fatal("healthy public compile failed to replace requested destination")
	}
	if _, err := plist.Unmarshal(raw, &native); err != nil {
		t.Fatal(err)
	}
	t.Log("Five parser-valid semantic failures rejected across unit, embedded/mobile, check and both public compile forms; zero native emissions/signing requests and original files retained; corrected compilation and one real loopback signing attempt observed.")
}
