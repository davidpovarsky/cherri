package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"howett.net/plist"
)

func p15AssertNoSigningScratch(t *testing.T, directory string) {
	t.Helper()
	for _, pattern := range []string{"cherri-sign-*", "cherri-compile-*", "cherri-atomic-*.tmp"} {
		var paths, err = filepath.Glob(filepath.Join(directory, pattern))
		if err != nil || len(paths) != 0 {
			t.Fatalf("signing leaked scratch artifacts: %v %v", paths, err)
		}
	}
}

func TestP15SigningFailureRetainsPublicDestinations(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	// This body exercises the existing structural transport check only. It is
	// deliberately not an Apple signature, import, or device-execution proof.
	var protocolBody = []byte("AEA1P15_PROTOCOL_ONLY_NO_APPLE_SIGNATURE")
	var response atomic.Int32
	var requests atomic.Int32
	type upload struct {
		data []byte
		name string
	}
	var uploads = make(chan upload, 64)
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		var file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, "missing actual unsigned upload", http.StatusBadRequest)
			return
		}
		defer file.Close()
		var data, readErr = io.ReadAll(file)
		if readErr != nil {
			http.Error(w, "unsigned upload unreadable", http.StatusBadRequest)
			return
		}
		uploads <- upload{data: data, name: header.Filename}
		switch response.Load() {
		case 0:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"message": "P15_ATOMIC_SIGNER_FAILURE"}})
		case 1:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("AEA1"))
		case 2:
			w.Header().Set("Content-Type", "application/x-plist")
			_, _ = w.Write(data)
		case 3:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "file": map[string]any{"content_base64": base64.StdEncoding.EncodeToString(protocolBody)}})
		}
	}))
	defer server.Close()

	var source = []byte("text(\"P15_SIGNING_TRANSACTION\")\n")
	var unsigned, err = CompileSourceToPlist("source.cherri", string(source))
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy", "sign"} {
		t.Run(mode, func(t *testing.T) {
			var directory = t.TempDir()
			var input = filepath.Join(directory, "source.cherri")
			var unsignedPath = filepath.Join(directory, "source_unsigned.shortcut")
			var output = filepath.Join(directory, "retained שלום result.shortcut")
			var original = []byte("P15_PRIOR_DESTINATION\x00\xff")
			p15SchemaWrite(t, input, source)
			p15SchemaWrite(t, unsignedPath, unsigned)
			var arguments []string
			switch mode {
			case "compile_v2":
				arguments = []string{"compile", input}
			case "file_v2":
				arguments = []string{input}
			case "compile_legacy":
				arguments = []string{"compile", input, "--legacy", "--load-all-actions"}
			case "file_legacy":
				arguments = []string{input, "--legacy", "--load-all-actions"}
			case "sign":
				arguments = []string{"sign", unsignedPath}
			}
			arguments = append(arguments, "--output="+output, "--signing-server="+server.URL+"/sign?response=json", "--no-ansi")
			for fault, diagnostic := range []string{"P15_ATOMIC_SIGNER_FAILURE", "truncated AEA1 archive", "unsigned XML/plist"} {
				response.Store(int32(fault))
				p15SchemaWrite(t, output, original)
				var before = requests.Load()
				var stdout, stderr, commandErr = p15RunCLI(t, binary, directory, nil, arguments...)
				if commandErr == nil || requests.Load() != before+1 || !strings.Contains(string(stdout)+string(stderr), diagnostic) {
					t.Fatalf("signing fault did not reach real public request: mode=%s fault=%d requests=%d error=%v stdout=%s stderr=%s", mode, fault, requests.Load()-before, commandErr, stdout, stderr)
				}
				var observed = <-uploads
				if mode == "sign" && !bytes.Equal(observed.data, unsigned) {
					t.Fatal("exact unsigned input changed before signing")
				}
				var native map[string]any
				if _, err := plist.Unmarshal(observed.data, &native); err != nil || native["WFWorkflowActions"] == nil {
					t.Fatalf("signer did not receive an actual compiled plist: %v", err)
				}
				var retained, readErr = os.ReadFile(output)
				if readErr != nil || !bytes.Equal(retained, original) {
					t.Fatalf("signing failure destroyed pre-existing destination: %v", readErr)
				}
				var retainedSource, sourceErr = os.ReadFile(input)
				if sourceErr != nil || !bytes.Equal(retainedSource, source) {
					t.Fatal("signing failure changed source")
				}
				if mode == "compile_v2" || mode == "file_v2" || mode == "sign" {
					var retainedUnsigned, unsignedErr = os.ReadFile(unsignedPath)
					if unsignedErr != nil || !bytes.Equal(retainedUnsigned, unsigned) {
						t.Fatal("public signing changed pre-existing unsigned artifact")
					}
				}
				p15AssertNoSigningScratch(t, directory)
			}
			response.Store(3)
			var stdout, stderr, commandErr = p15RunCLI(t, binary, directory, nil, arguments...)
			if commandErr != nil {
				t.Fatalf("restored protocol signing failed: %v %s %s", commandErr, stdout, stderr)
			}
			<-uploads
			var written, readErr = os.ReadFile(output)
			if readErr != nil || !bytes.Equal(written, protocolBody) {
				t.Fatal("restored protocol result was not committed exactly")
			}
			p15AssertNoSigningScratch(t, directory)
		})
	}
	t.Log("All five public signing routes retained prior destinations on JSON rejection, truncated archive and unsigned response; restored protocol-only response replaced destination exactly. No Apple signature/runtime claim.")
}

func TestP15SigningCommitFailurePreservesDestination(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var directory = t.TempDir()
	var input = filepath.Join(directory, "source.cherri")
	p15SchemaWrite(t, input, []byte("text(\"P15_SIGNING_COMMIT_FAILURE\")\n"))
	var output = filepath.Join(directory, "existing.shortcut")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	var retained = filepath.Join(output, "retained.txt")
	p15SchemaWrite(t, retained, []byte("MUST_SURVIVE"))
	var calls atomic.Int32
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write([]byte("AEA1P15_PROTOCOL_ONLY_NO_APPLE_SIGNATURE"))
	}))
	defer server.Close()
	var stdout, stderr, err = p15RunCLI(t, binary, directory, nil, "compile", input, "--output="+output, "--signing-server="+server.URL+"/sign", "--no-ansi")
	if err == nil || calls.Load() != 1 || !strings.Contains(string(stdout)+string(stderr), "failed to atomically rename") {
		t.Fatalf("real output commit failure was not surfaced: calls=%d error=%v stdout=%s stderr=%s", calls.Load(), err, stdout, stderr)
	}
	var data, readErr = os.ReadFile(retained)
	if readErr != nil || string(data) != "MUST_SURVIVE" {
		t.Fatalf("failed commit damaged prior destination: %s %v", data, readErr)
	}
	p15AssertNoSigningScratch(t, directory)
	t.Log(fmt.Sprintf("Real OS rename into a nonempty directory failed after %d observed signing request; prior destination survived and private stages were removed.", calls.Load()))
}
