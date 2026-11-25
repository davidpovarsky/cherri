package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

const p15BRG26Earlier = "BRG26_A_EARLIER"
const p15BRG26Current = "BRG26_B_CURRENT_שלום"
const p15BRG26Name = "BRG26 B שלום"

type p15BRG26Upload struct {
	Name  string
	Bytes []byte
}
type p15BRG26Recorder struct {
	mu       sync.Mutex
	response int
	uploads  []p15BRG26Upload
	errors   []string
}

func (r *p15BRG26Recorder) choose(response int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.response = response
}
func (r *p15BRG26Recorder) captured(t *testing.T, expected int) p15BRG26Upload {
	t.Helper()
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.errors) != 0 || len(r.uploads) != expected {
		t.Fatalf("actual signing requests: got=%d expected=%d errors=%v", len(r.uploads), expected, r.errors)
	}
	return r.uploads[expected-1]
}
func p15BRG26ProtocolResult(raw []byte) []byte {
	return []byte(fmt.Sprintf("AEA1BRG26_PROTOCOL_ONLY_NOT_APPLE_SIGNED_%x", sha256.Sum256(raw)))
}
func p15BRG26Provider(t *testing.T, protocol string) (*SigningService, *p15BRG26Recorder) {
	t.Helper()
	recorder := &p15BRG26Recorder{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var observed p15BRG26Upload
		var failure error
		if request.Method != http.MethodPost {
			failure = fmt.Errorf("method=%s", request.Method)
		} else if protocol == "multipart" {
			reader, err := request.MultipartReader()
			if err != nil {
				failure = err
			} else {
				part, err := reader.NextPart()
				if err != nil {
					failure = err
				} else if part.FormName() != "file" {
					failure = fmt.Errorf("unexpected form field %s", part.FormName())
				} else {
					observed.Name = part.FileName()
					observed.Bytes, failure = io.ReadAll(io.LimitReader(part, (1<<20)+1))
					_ = part.Close()
					if _, err := reader.NextPart(); err != io.EOF {
						failure = fmt.Errorf("unexpected extra multipart member: %v", err)
					}
				}
			}
		} else {
			var payload map[string]string
			decoder := json.NewDecoder(io.LimitReader(request.Body, (1<<20)+1))
			if err := decoder.Decode(&payload); err != nil {
				failure = err
			} else if len(payload) != 2 {
				failure = fmt.Errorf("unexpected JSON fields: %d", len(payload))
			} else {
				observed.Name = payload["shortcutName"]
				observed.Bytes = []byte(payload["shortcut"])
				var extra any
				if err := decoder.Decode(&extra); err != io.EOF {
					failure = fmt.Errorf("trailing JSON data: %v", err)
				}
			}
		}
		if len(observed.Bytes) == 0 || len(observed.Bytes) > 1<<20 {
			failure = fmt.Errorf("unbounded/empty actual upload: %d", len(observed.Bytes))
		}
		recorder.mu.Lock()
		response := recorder.response
		if failure != nil {
			recorder.errors = append(recorder.errors, failure.Error())
		} else {
			recorder.uploads = append(recorder.uploads, observed)
		}
		recorder.mu.Unlock()
		if failure != nil {
			http.Error(w, "BRG26_INVALID_ACTUAL_REQUEST", http.StatusBadRequest)
			return
		}
		switch response {
		case 0:
			http.Error(w, "BRG26_FIRST_SIGNING_FAILURE", http.StatusServiceUnavailable)
		case 1:
			signed := p15BRG26ProtocolResult(observed.Bytes)
			if protocol == "multipart" {
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": true,
					"file": map[string]any{"content_base64": base64.StdEncoding.EncodeToString(signed),
						"signed_sha256": fmt.Sprintf("%x", sha256.Sum256(signed))},
				})
			} else {
				w.Header().Set("Content-Type", "application/octet-stream")
				_, _ = w.Write(signed)
			}
		default:
			w.Header().Set("Content-Type", "application/x-plist")
			_, _ = w.Write(observed.Bytes)
		}
	}))
	t.Cleanup(server.Close)
	path := "/sign"
	if protocol == "multipart" {
		path += "?response=json"
	}
	return &SigningService{name: "BRG26 owned protocol-only provider", url: server.URL + path}, recorder
}
func p15BRG26NativeLiteral(t *testing.T, raw []byte, mode, expected string) {
	t.Helper()
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	actions, ok := document["WFWorkflowActions"].([]any)
	expectedCount := 1
	if mode == "v2" {
		expectedCount++
	}
	if !ok || len(actions) != expectedCount {
		t.Fatalf("wrong complete %s literal graph: %#v", mode, actions)
	}
	if mode == "v2" {
		if err := p15M02MainVoidRowsError(actions); err != nil {
			t.Fatal(err)
		}
	}
	action, ok := actions[0].(map[string]any)
	if !ok || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
		t.Fatal("current artifact lost its exact native Text producer")
	}
	params, ok := action["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		t.Fatal("missing native parameters")
	}
	value := params["WFTextActionText"]
	if value == expected {
		return
	}
	token, tokenOK := value.(map[string]any)
	body, bodyOK := token["Value"].(map[string]any)
	if !tokenOK || !bodyOK || token["WFSerializationType"] != "WFTextTokenString" || body["string"] != expected {
		t.Fatalf("current native literal differs: %#v", value)
	}
	if attachments, exists := body["attachmentsByRange"]; exists {
		entries, ok := attachments.(map[string]any)
		if !ok || len(entries) != 0 {
			t.Fatal("literal artifact acquired stale native references")
		}
	}
}
func p15BRG26Unsigned(t *testing.T, mode, name, literal string) []byte {
	t.Helper()
	result := compileForMobileMode("text("+fmt.Sprintf("%q", literal)+")\n", name, mode, false)
	if !result.OK || result.Mode != mode || result.SignedBase64 != "" {
		t.Fatalf("actual unsigned compilation failed or signed unexpectedly: %+v", result)
	}
	raw, err := base64.StdEncoding.DecodeString(result.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	p15BRG26NativeLiteral(t, raw, mode, literal)
	return raw
}
func p15BRG26StaleState(t *testing.T, mode string) (a, b []byte, directory, input, output string) {
	t.Helper()
	oldInput, oldOutput, oldBase, oldName := inputPath, outputPath, basename, workflowName
	oldShortcut, oldArgs := shortcut, args.Args
	oldFailed, oldServiceFailed, oldBackoff := signFailed, signingServiceFailed, backoff
	t.Cleanup(func() {
		inputPath, outputPath, basename, workflowName = oldInput, oldOutput, oldBase, oldName
		shortcut, args.Args = oldShortcut, oldArgs
		signFailed, signingServiceFailed, backoff = oldFailed, oldServiceFailed, oldBackoff
	})
	a = p15BRG26Unsigned(t, mode, "BRG26 A earlier", p15BRG26Earlier)
	earlierShortcut := shortcut
	if _, err := plist.Unmarshal(a, &earlierShortcut); err != nil {
		t.Fatal("cannot restore the actual earlier A workflow", err)
	}
	b = p15BRG26Unsigned(t, mode, p15BRG26Name, p15BRG26Current)
	if bytes.Equal(a, b) {
		t.Fatal("independent A/B compilation produced equal bytes")
	}
	directory = t.TempDir()
	stale := filepath.Join(directory, "A earlier.shortcut")
	input = filepath.Join(directory, "B current unsigned.shortcut")
	output = filepath.Join(directory, p15BRG26Name+".shortcut")
	p15SchemaWrite(t, stale, a)
	p15SchemaWrite(t, input, b)
	p15SchemaWrite(t, output, []byte("BRG26_EXISTING_DESTINATION"))
	// All ambient paths and metadata intentionally name A while the explicit
	// request names B. Only actual API arguments may select the signed artifact.
	inputPath, outputPath = stale, filepath.Join(directory, "A stale output.shortcut")
	basename, workflowName, shortcut = "BRG26_STALE_A_NAME", "BRG26_STALE_A_WORKFLOW", earlierShortcut
	args.Args = map[string]string{"no-ansi": ""}
	signFailed, signingServiceFailed, backoff = false, false, 10
	return
}
func p15BRG26RequireUpload(t *testing.T, observed p15BRG26Upload, protocol, name string, expected []byte) {
	t.Helper()
	if protocol == "multipart" {
		name += ".shortcut"
	}
	if observed.Name != name || !bytes.Equal(observed.Bytes, expected) {
		t.Fatalf("explicit B artifact was replaced by ambient state: name=%q want=%q payloadSHA=%x expectedSHA=%x",
			observed.Name, name, sha256.Sum256(observed.Bytes), sha256.Sum256(expected))
	}
	t.Logf("BRG26_EXACT_UPLOAD name=%q bytes=%d sha256=%x protocol=%s scope=PROTOCOL_ONLY",
		observed.Name, len(observed.Bytes), sha256.Sum256(observed.Bytes), protocol)
}

func TestBackendRecovery26ExplicitBytesAfterSequentialCompilation(t *testing.T) {
	for _, protocol := range []string{"multipart", "json"} {
		for _, mode := range []string{"v2", "legacy"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				_, b, directory, _, _ := p15BRG26StaleState(t, mode)
				before := p15DirectoryDigest(t, directory)
				service, observed := p15BRG26Provider(t, protocol)
				for attempt := 0; attempt < 3; attempt++ {
					observed.choose(attempt)
					result, err := SignShortcutBytes(service, p15BRG26Name, b)
					p15BRG26RequireUpload(t, observed.captured(t, attempt+1), protocol, p15BRG26Name, b)
					if attempt == 1 {
						if err != nil || !bytes.Equal(result, p15BRG26ProtocolResult(b)) {
							t.Fatalf("failed first request did not recover with exact B result: %v", err)
						}
					} else if err == nil || len(result) != 0 {
						t.Fatal("failed/unsigned response masqueraded as signed output")
					}
					p15RP43AssertSameFiles(t, directory, before)
				}
			})
		}
	}
}

func TestBackendRecovery26ExplicitFileAfterSequentialCompilation(t *testing.T) {
	for _, protocol := range []string{"multipart", "json"} {
		for _, mode := range []string{"v2", "legacy"} {
			t.Run(protocol+"/"+mode, func(t *testing.T) {
				_, b, directory, input, output := p15BRG26StaleState(t, mode)
				service, observed := p15BRG26Provider(t, protocol)
				args.Args["signing-server"] = service.url
				before := p15DirectoryDigest(t, directory)
				if err := SignShortcut(input, output, "people-who-know-me"); err == nil {
					t.Fatal("first signer failure was swallowed")
				}
				p15BRG26RequireUpload(t, observed.captured(t, 1), protocol, p15BRG26Name, b)
				p15RP43AssertSameFiles(t, directory, before)
				observed.choose(1)
				if err := SignShortcut(input, output, "people-who-know-me"); err != nil {
					t.Fatal("second explicit B signing did not recover", err)
				}
				p15BRG26RequireUpload(t, observed.captured(t, 2), protocol, p15BRG26Name, b)
				actual, err := os.ReadFile(output)
				if err != nil || !bytes.Equal(actual, p15BRG26ProtocolResult(b)) {
					t.Fatal("exact protocol result was not committed to explicit B path", err)
				}
				after := p15DirectoryDigest(t, directory)
				delete(before, filepath.Base(output))
				delete(after, filepath.Base(output))
				if !reflect.DeepEqual(before, after) {
					t.Fatal("explicit signing changed A, the unsigned B artifact, or a sibling")
				}
				p15AssertNoSigningScratch(t, directory)
			})
		}
	}
}

type p15BRG26Transport func(*http.Request) (*http.Response, error)

func (f p15BRG26Transport) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestBackendRecovery26MobileAfterSequentialCompilation(t *testing.T) {
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			_, _, directory, _, _ := p15BRG26StaleState(t, mode)
			before := p15DirectoryDigest(t, directory)
			service, observed := p15BRG26Provider(t, "multipart")
			target, err := url.Parse(service.url)
			if err != nil {
				t.Fatal(err)
			}
			originalTransport := http.DefaultTransport
			transport := http.DefaultTransport.(*http.Transport).Clone()
			t.Cleanup(func() { transport.CloseIdleConnections(); http.DefaultTransport = originalTransport })
			http.DefaultTransport = p15BRG26Transport(func(request *http.Request) (*http.Response, error) {
				if request.URL.String() != hubSign().url {
					return nil, fmt.Errorf("BRG26 unexpected network target")
				}
				redirect := request.Clone(request.Context())
				redirect.URL, redirect.Host, redirect.RequestURI = target, target.Host, ""
				return transport.RoundTrip(redirect)
			})
			for attempt := 0; attempt < 3; attempt++ {
				observed.choose(attempt)
				result := compileForMobileMode("text("+fmt.Sprintf("%q", p15BRG26Current)+")\n", p15BRG26Name, mode, true)
				raw, err := base64.StdEncoding.DecodeString(result.PlistBase64)
				if err != nil || result.Name != p15BRG26Name || result.Mode != mode {
					t.Fatalf("mobile current artifact/name/mode lost: %+v error=%v", result, err)
				}
				p15BRG26NativeLiteral(t, raw, mode, p15BRG26Current)
				p15BRG26RequireUpload(t, observed.captured(t, attempt+1), "multipart", p15BRG26Name, raw)
				if attempt == 1 {
					signed, err := base64.StdEncoding.DecodeString(result.SignedBase64)
					if !result.OK || result.Error != "" || err != nil || !bytes.Equal(signed, p15BRG26ProtocolResult(raw)) {
						t.Fatal("mobile recovery returned stale or unsigned bytes", result.Error, err)
					}
				} else if result.OK || result.SignedBase64 != "" || !strings.Contains(result.Error, "Signing error:") {
					t.Fatal("mobile failed/unsigned signer result masqueraded as success", result)
				}
				p15RP43AssertSameFiles(t, directory, before)
			}
		})
	}
}
