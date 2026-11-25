package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/protocol"
	"howett.net/plist"
)

type p15BRG25Fixture struct {
	name, source, literal, code, message string
	start, end                           protocol.Position
}

func p15BRG25Fixtures() []p15BRG25Fixture {
	return []p15BRG25Fixture{
		{name: "email", source: "show(\"owner@example.invalid\")\n", literal: "owner@example.invalid"},
		{name: "ask", source: "show(\"Ask\")\n", literal: "Ask"},
		{name: "directive_literal", source: "show(\"#define from sharesheet // cherri: legacy\")\n", literal: "#define from sharesheet // cherri: legacy"},
		{name: "type_error", source: "let typed: Text = 123\nshow(typed)\n", code: "E_ARGUMENT_TYPE", message: "type Number is not assignable to declared type Text", start: protocol.Position{Line: 0, Character: 18}, end: protocol.Position{Line: 0, Character: 21}},
		{name: "malformed_function", source: "function broken(value: Text -> Text {\n return value\n}\nshow(\"after\")\n", code: "E_SYNTAX", message: "expected 43, got 52 (\"->\")", start: protocol.Position{Line: 0, Character: 28}, end: protocol.Position{Line: 0, Character: 30}},
	}
}
func p15BRG25LiteralError(raw []byte, literal string) error {
	var workflow map[string]any
	if _, err := plist.Unmarshal(raw, &workflow); err != nil {
		return err
	}
	want := []any{
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.showresult", "WFWorkflowActionParameters": map[string]any{"Text": literal}},
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"},
	}
	if !reflect.DeepEqual(workflow["WFWorkflowActions"], want) {
		return fmt.Errorf("literal/default-v2 main output graph differs: %#v", workflow["WFWorkflowActions"])
	}
	return nil
}
func p15BRG25RequireMobile(t *testing.T, response mobileCompileResponse, fixture p15BRG25Fixture) {
	t.Helper()
	if fixture.code != "" {
		if response.OK || response.PlistBase64 != "" || response.SignedBase64 != "" || response.Mode != "v2" ||
			!strings.Contains(response.Error, fixture.code) || response.Line != fixture.start.Line+1 || response.Column != fixture.start.Character+1 {
			t.Fatalf("rejected mobile route lost precise diagnostics or emitted bytes: %+v", response)
		}
		return
	}
	if !response.OK || response.Mode != "v2" || response.SignedBase64 != "" {
		t.Fatalf("literal source selected wrong mobile frontend: %+v", response)
	}
	raw, err := base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15BRG25LiteralError(raw, fixture.literal); err != nil {
		t.Fatal(err)
	}
}

type p15BRG25Transport func(*http.Request) (*http.Response, error)

func (f p15BRG25Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func TestBackendRecovery25ActualMobileRoute(t *testing.T) {
	var signs atomic.Int32
	old := http.DefaultTransport
	http.DefaultTransport = p15BRG25Transport(func(req *http.Request) (*http.Response, error) {
		signs.Add(1)
		return &http.Response{StatusCode: 400, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader("{\"success\":false,\"error\":{\"message\":\"BRG25_SIGNER_TRIPWIRE\"}}")), Request: req}, nil
	})
	t.Cleanup(func() { http.DefaultTransport = old })
	for _, mode := range []string{"omitted", "v2", "auto"} {
		for _, fixture := range p15BRG25Fixtures() {
			t.Run(mode+"/"+fixture.name, func(t *testing.T) {
				before := signs.Load()
				var response mobileCompileResponse
				if mode == "omitted" {
					response = compileForMobile(fixture.source, "BRG25", fixture.code != "")
				} else {
					response = compileForMobileMode(fixture.source, "BRG25", mode, fixture.code != "")
				}
				p15BRG25RequireMobile(t, response, fixture)
				if signs.Load() != before {
					t.Fatal("semantic/syntax rejection reached signing")
				}
				trace, _ := json.Marshal(map[string]any{"mode": mode, "source": fixture.source, "response": response, "signerCalls": signs.Load() - before})
				t.Logf("BRG25_MOBILE %s", trace)
			})
		}
	}
	t.Run("signer_positive_control", func(t *testing.T) {
		before := signs.Load()
		response := compileForMobile("show(\"Ask\")\n", "BRG25 signer control", true)
		if response.OK || response.SignedBase64 != "" || signs.Load() <= before {
			t.Fatal("controlled signer observer was not reached", response)
		}
	})
	t.Run("explicit_legacy_control", func(t *testing.T) {
		const code = "const route = text(\"BRG25_REFERENCE\")\nshow(route)\n"
		rejected := compileForMobile(code, "BRG25 default", false)
		if rejected.OK || rejected.PlistBase64 != "" || !strings.Contains(rejected.Error, "E_LEGACY_SYNTAX") {
			t.Fatal("default accepted old source", rejected)
		}
		actual := compileForMobileMode(code, "BRG25 explicit legacy", "legacy", false)
		p15RP52MobileWire(t, actual, "BRG25_REFERENCE", false)
		if actual.Mode != "legacy" {
			t.Fatal("explicit Legacy route was lost")
		}
	})
}
func TestBackendRecovery25AutoModeLexicalBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source, literal string }{
		{"raw_multiline", "show(r\"\"\"\n#define from sharesheet\n// cherri: legacy\nAsk owner@example.invalid\n\"\"\")\n", "\n#define from sharesheet\n// cherri: legacy\nAsk owner@example.invalid\n"},
		{"block_comment", "/* ordinary comment\n#define from sharesheet\n// cherri: legacy\n*/\nshow(\"Ask\")\n", "Ask"},
		{"block_marker_inside_comment", "/* ordinary comment\n/* cherri: legacy */\n*/\nshow(\"Ask\")\n", "Ask"},
		{"inline_marker", "show(\"Ask\") // cherri: legacy\n", "Ask"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if detectLegacySource(tc.source) {
				t.Fatal("literal/comment contents selected Legacy")
			}
			response := compileForMobileMode(tc.source, "BRG25 lexical", "auto", false)
			p15BRG25RequireMobile(t, response, p15BRG25Fixture{literal: tc.literal})
		})
	}
	for _, tc := range []struct {
		name, source string
		legacy       bool
	}{
		{"real_legacy_marker", "// cherri: legacy\nconst x = 1\n", true},
		{"real_v1_marker", "/* cherri: v1 */\nconst x = 1\n", true},
		{"real_directive", "#define from sharesheet\nconst x = 1\n", true},
		{"v2_marker_first", "// cherri: v2\n#define from sharesheet\n", false},
		{"directive_prefix_collision", "#included 'actions/basic.cherri'\n", false},
		{"marker_prefix_collision", "// cherri: legacySuffix\nshow(\"Ask\")\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := detectLegacySource(tc.source); got != tc.legacy {
				t.Fatalf("actual anchored directive selection=%t want=%t", got, tc.legacy)
			}
		})
	}
}

type p15BRG25CoveredCLI struct {
	binary      string
	first, last int
}

func p15BRG25BuildCoveredCLI(t *testing.T) p15BRG25CoveredCLI {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "cherri-brg25.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := []string{"build", "-cover", "-covermode=count", "-coverpkg=github.com/electrikmilk/cherri", "-o", bin, "."}
	cmd := exec.CommandContext(ctx, "go", command...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("actual production CLI coverage build: %v %s", err, output)
	}
	set := gotoken.NewFileSet()
	file, err := goparser.ParseFile(set, "parser.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range file.Decls {
		if fn, ok := declaration.(*ast.FuncDecl); ok && fn.Name.Name == "initParse" {
			return p15BRG25CoveredCLI{bin, set.Position(fn.Body.Lbrace).Line, set.Position(fn.Body.Rbrace).Line}
		}
	}
	t.Fatal("actual Legacy parser boundary disappeared")
	return p15BRG25CoveredCLI{}
}
func (cli p15BRG25CoveredCLI) run(t *testing.T, cwd string, args ...string) ([]byte, []byte, error, uint64) {
	t.Helper()
	coverage := t.TempDir()
	stdout, stderr, runErr := p15RunCLI(t, cli.binary, cwd, []string{"GOCOVERDIR=" + coverage}, args...)
	profile := filepath.Join(t.TempDir(), "coverage.txt")
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := []string{"tool", "covdata", "textfmt", "-i=" + coverage, "-o=" + profile}
	cmd := exec.CommandContext(ctx, "go", command...)
	data, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("actual runtime counters unavailable: %v %s", err, data)
	}
	raw, err := os.ReadFile(profile)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	first := int(^uint(0) >> 1)
	var calls uint64
	for _, line := range strings.Split(string(raw), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		colon := strings.LastIndex(fields[0], ":")
		if colon < 0 || !strings.HasSuffix(fields[0][:colon], "/parser.go") {
			continue
		}
		var sl, sc, el, ec int
		if n, _ := fmt.Sscanf(fields[0][colon+1:], "%d.%d,%d.%d", &sl, &sc, &el, &ec); n != 4 {
			t.Fatal("malformed original coverage row")
		}
		if sl < cli.first || el > cli.last {
			continue
		}
		count, e := strconv.ParseUint(fields[2], 10, 64)
		if e != nil {
			t.Fatal(e)
		}
		if sl < first {
			found = true
			first = sl
			calls = count
		}
	}
	if !found || first != cli.first {
		t.Fatal("coverage omitted actual Legacy parser entry block", first, cli.first)
	}
	var packed bytes.Buffer
	zipper := gzip.NewWriter(&packed)
	if _, e := zipper.Write(raw); e != nil {
		t.Fatal(e)
	}
	if e := zipper.Close(); e != nil {
		t.Fatal(e)
	}
	coverageSHA := sha256.Sum256(raw)
	trace, _ := json.Marshal(map[string]any{"argv": args, "stdout": string(stdout), "stderr": string(stderr), "coverageCommand": command, "coverageGzipBase64": base64.StdEncoding.EncodeToString(packed.Bytes()), "coverageBytes": len(raw), "coverageSha256": fmt.Sprintf("%x", coverageSHA), "legacyParserEntries": calls})
	t.Logf("BRG25_CLI_ORIGINAL %s", trace)
	return stdout, stderr, runErr, calls
}
func TestBackendRecovery25ActualCLIRoute(t *testing.T) {
	cli := p15BRG25BuildCoveredCLI(t)
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(400)
		_, _ = io.WriteString(w, "{\"success\":false,\"error\":{\"message\":\"BRG25_SIGNER_TRIPWIRE\"}}")
	}))
	defer server.Close()
	for _, form := range []string{"file", "compile"} {
		for _, mode := range []string{"default", "v2"} {
			for _, fixture := range p15BRG25Fixtures() {
				t.Run(form+"/"+mode+"/"+fixture.name, func(t *testing.T) {
					dir := t.TempDir()
					source := filepath.Join(dir, "source.cherri")
					target := filepath.Join(dir, "result.shortcut")
					if err := os.WriteFile(source, []byte(fixture.source), 0600); err != nil {
						t.Fatal(err)
					}
					args := []string{source, "--no-ansi", "--load-all-actions", "--output=" + target}
					if fixture.code == "" {
						args = append(args, "--skip-sign")
					} else {
						args = append(args, "--signing-server="+server.URL)
					}
					if mode == "v2" {
						args = append(args, "--mode=v2")
					}
					if form == "compile" {
						args = append([]string{"compile"}, args...)
					}
					before := requests.Load()
					out, stderr, err, calls := cli.run(t, dir, args...)
					if calls != 0 {
						t.Fatalf("V2 public request entered Legacy parser %d times", calls)
					}
					if fixture.code != "" {
						if err == nil || !strings.Contains(string(out)+string(stderr), fixture.code) {
							t.Fatal("rejected source lost semantic/syntax failure", err, string(out), string(stderr))
						}
						if _, e := os.Stat(target); !os.IsNotExist(e) {
							t.Fatal("rejected source emitted output", e)
						}
						if requests.Load() != before {
							t.Fatal("rejected source attempted signing")
						}
					} else {
						if err != nil {
							t.Fatal("valid literal failed", err, string(out), string(stderr))
						}
						raw, e := os.ReadFile(target)
						if e != nil {
							t.Fatal(e)
						}
						if e = p15BRG25LiteralError(raw, fixture.literal); e != nil {
							t.Fatal(e)
						}
					}
				})
			}
		}
	}
	for _, fixture := range p15BRG25Fixtures() {
		t.Run("check/"+fixture.name, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "source.cherri")
			if err := os.WriteFile(source, []byte(fixture.source), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err, calls := cli.run(t, dir, "check", source, "--json")
			if calls != 0 {
				t.Fatal("analysis entered Legacy source parser")
			}
			var response protocol.AnalyzeResponse
			if e := json.Unmarshal(out, &response); e != nil {
				t.Fatal(e, string(out), string(stderr))
			}
			valid := fixture.code == ""
			if (err == nil) != valid || response.Valid != valid {
				t.Fatal("analysis validity/exit disagrees", response, err)
			}
			if valid {
				if len(response.Diagnostics) != 0 {
					t.Fatal("literal source gained diagnostics", response)
				}
			} else {
				want := []protocol.DiagnosticItem{{Code: fixture.code, Severity: "ERROR", Message: fixture.message, Range: protocol.Range{Start: fixture.start, End: fixture.end}}}
				if !reflect.DeepEqual(response.Diagnostics, want) {
					t.Fatalf("precise diagnostic differs: got=%+v want=%+v", response.Diagnostics, want)
				}
			}
		})
	}
	for _, form := range []string{"file", "compile"} {
		t.Run("legacy_positive/"+form, func(t *testing.T) {
			dir := t.TempDir()
			source := filepath.Join(dir, "legacy.cherri")
			target := filepath.Join(dir, "legacy.shortcut")
			if err := os.WriteFile(source, []byte("const route = text(\"BRG25_REFERENCE\")\nshow(route)\n"), 0600); err != nil {
				t.Fatal(err)
			}
			args := []string{source, "--legacy", "--skip-sign", "--no-ansi", "--load-all-actions", "--output=" + target}
			if form == "compile" {
				args = append([]string{"compile"}, args...)
			}
			out, stderr, err, calls := cli.run(t, dir, args...)
			if err != nil || calls == 0 {
				t.Fatal("same instrumented binary did not reach real Legacy parser", err, calls, string(out), string(stderr))
			}
			raw, e := os.ReadFile(target)
			if e != nil {
				t.Fatal(e)
			}
			var wf map[string]any
			if _, e = plist.Unmarshal(raw, &wf); e != nil {
				t.Fatal(e)
			}
			var actions []map[string]any
			for _, item := range wf["WFWorkflowActions"].([]any) {
				actions = append(actions, item.(map[string]any))
			}
			if e = p15U04PublicSourceWire(actions, "BRG25_REFERENCE", false); e != nil {
				t.Fatal(e)
			}
		})
	}
	t.Run("signer_positive_control", func(t *testing.T) {
		dir := t.TempDir()
		source := filepath.Join(dir, "healthy.cherri")
		target := filepath.Join(dir, "healthy.shortcut")
		if err := os.WriteFile(source, []byte("show(\"Ask\")\n"), 0600); err != nil {
			t.Fatal(err)
		}
		before := requests.Load()
		_, _, err, calls := cli.run(t, dir, "compile", source, "--signing-server="+server.URL, "--output="+target, "--no-ansi")
		if err == nil || calls != 0 || requests.Load() <= before {
			t.Fatal("same CLI signer observer not reached by healthy V2", err, calls, requests.Load())
		}
		if _, e := os.Stat(target); !os.IsNotExist(e) {
			t.Fatal("failed signing emitted final target", e)
		}
	})
}
