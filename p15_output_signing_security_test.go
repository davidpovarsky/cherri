package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/electrikmilk/args-parser"
)

func TestP15RP43EncodingFaultProcessHelper(t *testing.T) {
	var target = os.Getenv("CHERRI_RP43_ENCODING_OUTPUT")
	if target == "" {
		t.Skip("only the owned encoding-fault subprocess invokes this helper")
	}
	args.Args = map[string]string{"skip-sign": "", "output": target, "no-ansi": ""}
	workflowName = "encoding"
	relativePath = filepath.Dir(target) + string(os.PathSeparator)
	shortcut = BuildWorkflowHeader()
	importedWorkflowHeader = map[string]any{"P15ActualEncoderFault": make(chan int)}
	fmt.Println("P15_RP43_ENCODING_FAULT_REACHED")
	createShortcut()
	t.Fatal("the real output entrypoint swallowed the injected encoding failure")
}

func TestP15RP43LegacyEncodingFailurePreservesDestination(t *testing.T) {
	var directory = t.TempDir()
	var output = filepath.Join(directory, "encoding_unsigned.shortcut")
	var original = []byte("P15_RP43_ENCODING_PRIOR_DESTINATION\x00\xff")
	p15SchemaWrite(t, output, original)
	var before = p15DirectoryDigest(t, directory)
	var executable, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, executable, "-test.run=^TestP15RP43EncodingFaultProcessHelper$")
	command.Dir = directory
	command.Env = append(os.Environ(), "CHERRI_RP43_ENCODING_OUTPUT="+output)
	var diagnostic, commandErr = command.CombinedOutput()
	var exit, ok = commandErr.(*exec.ExitError)
	if ctx.Err() != nil || !ok || exit.ExitCode() <= 0 || !bytes.Contains(diagnostic, []byte("P15_RP43_ENCODING_FAULT_REACHED")) || !bytes.Contains(diagnostic, []byte("chan int")) {
		t.Fatalf("did not observe the real encoder error/exit: err=%v context=%v output=%s", commandErr, ctx.Err(), diagnostic)
	}
	p15RP43AssertSameFiles(t, directory, before)
}

func TestP15RP43EncodingFailureReturnsNoPartialBytes(t *testing.T) {
	for _, format := range []string{"xml", "binary"} {
		t.Run(format, func(t *testing.T) {
			var injected int
			factory := func(fault bool) func() *CanonicalBackendSession {
				return func() *CanonicalBackendSession {
					var session = NewCanonicalBackendSession()
					session.hasRawWorkflow = true
					session.rawWorkflow = map[string]any{"P15EncodingControl": "healthy"}
					if format == "binary" {
						session.rawWorkflow["P15BinaryControl"] = "\x00"
					}
					if fault {
						// A synthetic emitter fault reaches the real final encoder.
						// No alternate encoder or production failure hook is used.
						session.rawWorkflow["P15EncodingControl"] = make(chan int)
						injected++
					}
					return session
				}
			}
			var source = "text(\"" + p15RP43Text + "\")\n"
			var data, err = compileSourceToPlistWithSession("encoding.cherri", source, CompileOptions{}, factory(true))
			if err == nil || len(data) != 0 || injected != 1 || !strings.Contains(err.Error(), "plist encoding error") {
				t.Fatalf("real final encoding fault was swallowed or exposed partial bytes: injected=%d bytes=%d err=%v", injected, len(data), err)
			}
			data, err = compileSourceToPlistWithSession("encoding.cherri", source, CompileOptions{}, factory(false))
			if err != nil {
				t.Fatalf("valid encoding did not recover after failed session: %v", err)
			}
			if (format == "binary") != bytes.HasPrefix(data, []byte("bplist00")) {
				t.Fatalf("restored control did not exercise %s encoder", format)
			}
			p15RP43AssertNativeOutput(t, data)
		})
	}
}

func TestP15RP43PublicSignerFailureBoundaries(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var protocolOnly = []byte("AEA1P15_RP43_PROTOCOL_ONLY_NO_APPLE_SIGNATURE")
	type upload struct {
		uri, name string
		data      []byte
	}
	var uploads []upload
	var lock sync.Mutex
	var response atomic.Int32
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var file, header, err = r.FormFile("file")
		if err != nil {
			http.Error(w, "P15_RP43_MISSING_UPLOAD", http.StatusBadRequest)
			return
		}
		defer file.Close()
		var data, readErr = io.ReadAll(io.LimitReader(file, 1<<20))
		if readErr != nil || len(data) == 1<<20 {
			http.Error(w, "P15_RP43_INVALID_UPLOAD", http.StatusBadRequest)
			return
		}
		lock.Lock()
		uploads = append(uploads, upload{r.RequestURI, header.Filename, data})
		lock.Unlock()
		switch response.Load() {
		case 0:
			http.Error(w, "P15_RP43_HTTP_FAILURE", http.StatusServiceUnavailable)
		case 1:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":false,"error":{"message":"P15_RP43_REJECTED"}}`)
		case 2:
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"success":`)
		case 3:
			w.Header().Set("Content-Type", "application/x-plist")
			_, _ = w.Write(data)
		case 4:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write([]byte("AEA1"))
		case 5:
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(append([]byte("AEA1"), make([]byte, 20)...))
		default:
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "file": map[string]any{"content_base64": base64.StdEncoding.EncodeToString(protocolOnly)}})
		}
	}))
	defer server.Close()
	const suffix = "/sign?response=json&identity=rp43"
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy", "sign"} {
		t.Run(route, func(t *testing.T) {
			var directory = t.TempDir()
			var input = filepath.Join(directory, "sign source שלום.cherri")
			p15SchemaWrite(t, input, []byte("text(\""+p15RP43Text+"\")\n"))
			var unsignedPath = filepath.Join(directory, "exact-input.shortcut")
			var stdout, stderr, compileErr = p15RunCLI(t, binary, directory, nil, "compile", input, "--skip-sign", "--output="+unsignedPath)
			if compileErr != nil {
				t.Fatalf("actual unsigned preparation failed: %v %s %s", compileErr, stdout, stderr)
			}
			var unsigned, readErr = os.ReadFile(unsignedPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			p15RP43AssertNativeOutput(t, unsigned)
			var sibling = strings.TrimSuffix(input, ".cherri") + "_unsigned.shortcut"
			p15SchemaWrite(t, sibling, []byte("P15_RP43_PRIOR_UNSIGNED_SIBLING\x00\xff"))
			var output = filepath.Join(directory, "signed שלום result.shortcut")
			var manifest = filepath.Join(directory, "signing-manifest.json")
			var original = []byte("P15_RP43_PRIOR_SIGNED_DESTINATION\x00\xff")
			p15SchemaWrite(t, output, original)
			p15SchemaWrite(t, manifest, []byte("P15_RP43_PRIOR_MANIFEST"))
			var arguments = p15RP43CompileArgs(route, input, output, "--signing-server="+server.URL+suffix)
			if route == "sign" {
				arguments = []string{"sign", unsignedPath, "--output=" + output, "--source-file=" + input, "--manifest-out=" + manifest, "--signing-server=" + server.URL + suffix, "--no-ansi"}
			}
			for fault, diagnostic := range []string{"P15_RP43_HTTP_FAILURE", "P15_RP43_REJECTED", "corrupted signing service JSON", "unsigned XML/plist", "truncated AEA1 archive", "all zero"} {
				t.Run(fmt.Sprintf("fault_%d", fault), func(t *testing.T) {
					p15SchemaWrite(t, sibling, []byte("P15_RP43_PRIOR_UNSIGNED_SIBLING\x00\xff"))
					p15SchemaWrite(t, output, original)
					p15SchemaWrite(t, manifest, []byte("P15_RP43_PRIOR_MANIFEST"))
					response.Store(int32(fault))
					var before = p15DirectoryDigest(t, directory)
					lock.Lock()
					var count = len(uploads)
					lock.Unlock()
					var stdout, stderr, err = p15RunCLI(t, binary, directory, nil, arguments...)
					lock.Lock()
					var observed = append([]upload(nil), uploads[count:]...)
					lock.Unlock()
					if err == nil || len(observed) != 1 || !strings.Contains(string(stdout)+string(stderr), diagnostic) {
						t.Errorf("public signer failure was not observed correctly: uploads=%d err=%v stdout=%s stderr=%s", len(observed), err, stdout, stderr)
					}
					if len(observed) == 1 {
						if observed[0].uri != suffix || observed[0].name == "" {
							t.Error("explicit local signer request identity changed")
						}
						p15RP43AssertNativeOutput(t, observed[0].data)
						if route == "sign" && !bytes.Equal(observed[0].data, unsigned) {
							t.Error("explicit signing changed the exact input bytes")
						}
					}
					p15RP43AssertSameFiles(t, directory, before)
				})
			}
			response.Store(6)
			stdout, stderr, compileErr = p15RunCLI(t, binary, directory, nil, arguments...)
			if compileErr != nil {
				t.Fatalf("restored protocol response failed: %v %s %s", compileErr, stdout, stderr)
			}
			var data, err = os.ReadFile(output)
			if err != nil || !bytes.Equal(data, protocolOnly) {
				t.Fatalf("restored protocol bytes were not written exactly: %v", err)
			}
			if route == "sign" {
				var receipt SigningProvenanceManifest
				var encoded, err = os.ReadFile(manifest)
				if err != nil || json.Unmarshal(encoded, &receipt) != nil || receipt.VerificationTier != TierAEA1HeaderPass || receipt.UnsignedPlistSHA256 != fmt.Sprintf("%x", sha256.Sum256(unsigned)) || receipt.SignedArtifactSHA256 != fmt.Sprintf("%x", sha256.Sum256(protocolOnly)) {
					t.Fatalf("protocol-only provenance overstated signing or lost exact byte identity: %v %s", err, encoded)
				}
			}
			p15AssertNoSigningScratch(t, directory)
		})
	}
}

func TestP15RP43UnsignedAndAnalysisBoundaries(t *testing.T) {
	if runtime.GOOS == "darwin" {
		t.Skip("implicit unsigned default belongs to non-Darwin public routes")
	}
	var binary = p15AcceptanceCLI(t)
	var requests atomic.Int32
	var receiver = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		http.Error(w, "P15_RP43_NETWORK_OBSERVED", http.StatusServiceUnavailable)
	}))
	defer receiver.Close()
	var networkEnv = []string{"HTTP_PROXY=" + receiver.URL, "HTTPS_PROXY=" + receiver.URL, "NO_PROXY=", "http_proxy=" + receiver.URL, "https_proxy=" + receiver.URL, "no_proxy="}
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy"} {
		var scope = "implicit_unsigned/"
		if route == "file_legacy" {
			scope = "explicit_unsigned/"
		}
		t.Run(scope+route, func(t *testing.T) {
			var directory = t.TempDir()
			var input = filepath.Join(directory, "source--hubsign.cherri")
			p15SchemaWrite(t, input, []byte("text(\""+p15RP43Text+"\")\n"))
			var output = filepath.Join(directory, "implicit.shortcut")
			var before = requests.Load()
			var options []string
			if route == "file_legacy" {
				// Bare Legacy file invocation retains its documented signing
				// default. An explicit local-only request is the contract here.
				options = append(options, "--skip-sign")
			}
			var stdout, stderr, err = p15RunCLI(t, binary, directory, networkEnv, p15RP43CompileArgs(route, input, output, options...)...)
			if err != nil || requests.Load() != before {
				t.Fatalf("implicit unsigned compilation attempted signing: requests=%d err=%v stdout=%s stderr=%s", requests.Load()-before, err, stdout, stderr)
			}
			var data, readErr = os.ReadFile(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			p15RP43AssertNativeOutput(t, data)
			if tier, err := ValidateSignedShortcutBytes(data); err == nil || tier != "" {
				t.Fatalf("unsigned output was claimed signed: tier=%q err=%v", tier, err)
			}
		})
	}
	var helpers = t.TempDir()
	var helperSource = filepath.Join(helpers, "observer.go")
	p15SchemaWrite(t, helperSource, []byte("package main\nimport \"os\"\nfunc main(){if err:=os.WriteFile(os.Getenv(\"CHERRI_RP43_COMMAND_MARKER\"),[]byte(\"called\"),0600);err!=nil{os.Exit(94)};os.Exit(93)}\n"))
	var suffix string
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	var executable = filepath.Join(helpers, "observer"+suffix)
	var ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	if data, err := exec.CommandContext(ctx, "go", "build", "-o", executable, helperSource).CombinedOutput(); err != nil {
		t.Fatalf("build actual process observer: %v %s", err, data)
	}
	var observer, err = os.ReadFile(executable)
	if err != nil {
		t.Fatal(err)
	}
	for _, command := range []string{"shortcuts", "open"} {
		if err := os.WriteFile(filepath.Join(helpers, command+suffix), observer, 0700); err != nil {
			t.Fatal(err)
		}
	}
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprintf("check_valid_%t", valid), func(t *testing.T) {
			var directory = t.TempDir()
			var input = filepath.Join(directory, "analysis.cherri")
			var source = "text(\"" + p15RP43Text + "\")\n"
			if !valid {
				source += "number()\n"
			}
			p15SchemaWrite(t, input, []byte(source))
			var output = filepath.Join(directory, "retained.shortcut")
			p15SchemaWrite(t, output, []byte("P15_RP43_CHECK_RETAIN\x00\xff"))
			var marker = filepath.Join(directory, "command-observed")
			var environment = append(append([]string{}, networkEnv...), "PATH="+helpers+string(os.PathListSeparator)+os.Getenv("PATH"), "CHERRI_RP43_COMMAND_MARKER="+marker)
			for _, command := range []string{"shortcuts", "open"} {
				var _, _, err = p15RunCLI(t, filepath.Join(helpers, command+suffix), directory, environment)
				var exit, ok = err.(*exec.ExitError)
				if !ok || exit.ExitCode() != 93 {
					t.Fatalf("process observer did not execute: %v", err)
				}
				if data, err := os.ReadFile(marker); err != nil || string(data) != "called" {
					t.Fatalf("process observer did not record command: %v", err)
				}
				if err := os.Remove(marker); err != nil {
					t.Fatal(err)
				}
			}
			var before = p15DirectoryDigest(t, directory)
			var beforeRequests = requests.Load()
			var stdout, stderr, err = p15RunCLI(t, binary, directory, environment, "check", input, "--json", "--open", "--hubsign", "--signing-server="+receiver.URL+"/sign", "--output="+output)
			var response, ok = p15DecodeOneJSON(t, stdout).(map[string]any)
			if (err == nil) != valid || !ok || response["valid"] != valid || requests.Load() != beforeRequests {
				t.Fatalf("check changed execution/signing semantics: valid=%t requests=%d err=%v stdout=%s stderr=%s", valid, requests.Load()-beforeRequests, err, stdout, stderr)
			}
			p15RP43AssertSameFiles(t, directory, before)
		})
	}
	t.Run("explicit_remote_request_control", func(t *testing.T) {
		var directory = t.TempDir()
		var input = filepath.Join(directory, "explicit.cherri")
		p15SchemaWrite(t, input, []byte("text(\""+p15RP43Text+"\")\n"))
		var output = filepath.Join(directory, "retained.shortcut")
		p15SchemaWrite(t, output, []byte("P15_RP43_EXPLICIT_RETAIN\x00\xff"))
		var before = p15DirectoryDigest(t, directory)
		var beforeRequests = requests.Load()
		var stdout, stderr, err = p15RunCLI(t, binary, directory, networkEnv, "compile", input, "--signing-server="+receiver.URL+"/sign", "--output="+output, "--no-ansi")
		if err == nil || requests.Load() != beforeRequests+1 || !strings.Contains(string(stdout)+string(stderr), "P15_RP43_NETWORK_OBSERVED") {
			t.Fatalf("explicit local remote-sign request was not observed: requests=%d err=%v stdout=%s stderr=%s", requests.Load()-beforeRequests, err, stdout, stderr)
		}
		p15RP43AssertSameFiles(t, directory, before)
	})
}
