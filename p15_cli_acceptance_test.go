package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"howett.net/plist"
)

func p15AcceptanceCLI(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "cherri-acceptance.exe")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var buildStarted = time.Now()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", bin, ".")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build current production CLI: %v (context: %v, elapsed: %s)\n%s", err, ctx.Err(), time.Since(buildStarted).Round(time.Millisecond), out)
	}
	return bin
}

func p15RunCLI(t *testing.T, bin, cwd string, env []string, arguments ...string) ([]byte, []byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, arguments...)
	cmd.Dir = cwd
	cmd.Env = append(os.Environ(), env...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("CLI timeout is not an accepted failure: %v", ctx.Err())
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func p15DecodeOneJSON(t *testing.T, data []byte) any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	var value any
	if err := dec.Decode(&value); err != nil {
		t.Fatalf("stdout was not JSON only: %v\n%s", err, data)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		t.Fatalf("stdout contained trailing data: %v\n%s", err, data)
	}
	return value
}

func TestP15CLIJSONCommandsAndExitStatus(t *testing.T) {
	bin := p15AcceptanceCLI(t)
	dir := t.TempDir()
	for _, option := range []string{"--capabilities-json", "--actions-json", "--action-schema-json", "--action-schema-json=show"} {
		t.Run(option, func(t *testing.T) {
			out, stderr, err := p15RunCLI(t, bin, dir, nil, option)
			if err != nil {
				t.Fatalf("healthy JSON command failed: %v\n%s", err, stderr)
			}
			value := p15DecodeOneJSON(t, out)
			if option == "--capabilities-json" {
				obj, ok := value.(map[string]any)
				if !ok || obj["languageVersion"] != "2.0" {
					t.Fatalf("wrong capabilities: %T", value)
				}
				fp, _ := obj["schemaFingerprint"].(string)
				if len(fp) != 64 {
					t.Fatalf("missing schema provenance: %q", fp)
				}
			}
		})
	}
	for _, tc := range []struct {
		name, source string
		valid        bool
	}{
		{"valid", "let marker = text(\"P15 JSON\")\nshow(marker)\n", true},
		{"invalid", "let marker = missingAction()\n", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name+".cherri")
			if err := os.WriteFile(path, []byte(tc.source), 0600); err != nil {
				t.Fatal(err)
			}
			out, stderr, err := p15RunCLI(t, bin, dir, nil, "check", path, "--json")
			if (err == nil) != tc.valid {
				t.Fatalf("incorrect exit status valid=%v err=%v stderr=%s", tc.valid, err, stderr)
			}
			obj, ok := p15DecodeOneJSON(t, out).(map[string]any)
			if !ok || obj["valid"] != tc.valid {
				t.Fatalf("JSON validity disagrees: %v", obj)
			}
			if !tc.valid {
				diagnostics, ok := obj["diagnostics"].([]any)
				if !ok || len(diagnostics) == 0 {
					t.Fatal("invalid source lacked diagnostics")
				}
			}
		})
	}
	for _, args := range [][]string{
		{"--action-schema-json=P15_missing_action"},
		{"check", filepath.Join(dir, "absent.cherri"), "--json"},
	} {
		out, stderr, err := p15RunCLI(t, bin, dir, nil, args...)
		if err == nil || len(out) != 0 || len(stderr) == 0 {
			t.Fatalf("invalid request lacked nonzero diagnostic: %v %s %s", err, out, stderr)
		}
	}
	t.Run("FilenameDoesNotSelectJSON", func(t *testing.T) {
		path := filepath.Join(dir, "plain--json.cherri")
		if err := os.WriteFile(path, []byte("text(\"plain\")\n"), 0600); err != nil {
			t.Fatal(err)
		}
		out, stderr, err := p15RunCLI(t, bin, dir, nil, "check", path)
		if err != nil || json.Valid(out) || !strings.Contains(string(out), "valid Cherri") {
			t.Fatalf("filename changed output protocol: %v %s %s", err, out, stderr)
		}
	})
}

func p15DirectoryDigest(t *testing.T, dir string) map[string][32]byte {
	t.Helper()
	result := map[string][32]byte{}
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		name, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		result[name] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}

func TestP15CLIAnalysisHasNoExternalEffects(t *testing.T) {
	bin := p15AcceptanceCLI(t)
	dir := t.TempDir()
	source := filepath.Join(dir, "analysis.cherri")
	if err := os.WriteFile(source, []byte("let marker = text(\"P15 must not execute\")\nshow(marker)\n"), 0600); err != nil {
		t.Fatal(err)
	}
	helperDir := t.TempDir()
	helperSource := filepath.Join(helperDir, "probe.go")
	helperBody := "package main\nimport (\"os\")\nfunc main(){ _=os.WriteFile(os.Getenv(\"CHERRI_SIDE_EFFECT_MARKER\"),[]byte(\"called\"),0600);os.Exit(93) }\n"
	if err := os.WriteFile(helperSource, []byte(helperBody), 0600); err != nil {
		t.Fatal(err)
	}
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	helper := filepath.Join(helperDir, "probe"+suffix)
	if out, err := exec.Command("go", "build", "-o", helper, helperSource).CombinedOutput(); err != nil {
		t.Fatalf("instrumentation build: %v %s", err, out)
	}
	data, err := os.ReadFile(helper)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shortcuts", "open"} {
		if err := os.WriteFile(filepath.Join(helperDir, name+suffix), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	marker := filepath.Join(dir, "external-command-invoked")
	environment := []string{"PATH=" + helperDir + string(os.PathListSeparator) + os.Getenv("PATH"), "CHERRI_SIDE_EFFECT_MARKER=" + marker}
	for _, name := range []string{"shortcuts", "open"} {
		_, _, err := p15RunCLI(t, filepath.Join(helperDir, name+suffix), dir, environment)
		exit, ok := err.(*exec.ExitError)
		if !ok || exit.ExitCode() != 93 {
			t.Fatalf("external-command observer is not operational: %v", err)
		}
		if _, err := os.Stat(marker); err != nil {
			t.Fatalf("observer did not record invocation: %v", err)
		}
		if err := os.Remove(marker); err != nil {
			t.Fatal(err)
		}
	}
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"success\":false,\"error\":{\"message\":\"P15_ANALYSIS_SIGNER_CONTROL\"}}")
	}))
	defer server.Close()
	target := filepath.Join(dir, "must-not-exist.shortcut")
	before := p15DirectoryDigest(t, dir)
	out, stderr, err := p15RunCLI(t, bin, dir, environment, "check", source, "--json", "--hubsign", "--open", "--signing-server="+server.URL+"/sign?response=json", "--output="+target)
	if err != nil {
		t.Fatalf("analysis route failed: %v %s", err, stderr)
	}
	obj, ok := p15DecodeOneJSON(t, out).(map[string]any)
	if !ok || obj["valid"] != true {
		t.Fatalf("analysis did not complete: %v", obj)
	}
	after := p15DirectoryDigest(t, dir)
	if len(before) != len(after) {
		t.Fatalf("analysis created/deleted files: before=%v after=%v", before, after)
	}
	for name, sum := range before {
		if after[name] != sum {
			t.Errorf("analysis modified %s", name)
		}
	}
	if requests.Load() != 0 {
		t.Fatalf("analysis attempted signing: %d requests", requests.Load())
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("analysis invoked shortcuts/open: %v", err)
	}
	// Positive control uses the same binary, signer and output route after the
	// pure-analysis assertions; a failed signer cannot masquerade as no attempt.
	_, stderr, err = p15RunCLI(t, bin, dir, environment, "compile", source, "--hubsign", "--signing-server="+server.URL+"/sign?response=json", "--output="+target)
	if err == nil || requests.Load() == 0 || !strings.Contains(string(stderr), "P15_ANALYSIS_SIGNER_CONTROL") {
		t.Fatalf("signing observer not reached by control: requests=%d err=%v stderr=%s", requests.Load(), err, stderr)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("failed signing left target: %v", err)
	}
}

func TestP15CLIDefaultRejectsLegacyWithGuidance(t *testing.T) {
	bin := p15AcceptanceCLI(t)
	dir := t.TempDir()
	cases := []string{"const value = text(\"legacy\")\nshow(value)\n", "@value = \"legacy\"\nshow(@value)\n", "#include 'actions/web.cherri'\n"}
	for i, src := range cases {
		path := filepath.Join(dir, string(rune('a'+i))+".cherri")
		if err := os.WriteFile(path, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		for _, form := range []string{"file", "compile", "check"} {
			args := []string{path, "--skip-sign", "--no-ansi"}
			if form == "compile" {
				args = append([]string{"compile"}, args...)
			}
			if form == "check" {
				args = []string{"check", path, "--json"}
			}
			out, stderr, err := p15RunCLI(t, bin, dir, nil, args...)
			combined := string(out) + string(stderr)
			if err == nil || !strings.Contains(combined, "E_LEGACY_SYNTAX") || !strings.Contains(strings.ToLower(combined), "migrat") {
				t.Fatalf("%s allowed legacy or lost migration diagnostic: %v\n%s", form, err, combined)
			}
			matches, globErr := filepath.Glob(filepath.Join(dir, "*.shortcut"))
			if globErr != nil || len(matches) != 0 {
				t.Fatalf("rejected source emitted a Shortcut: %v %v", matches, globErr)
			}
		}
	}
	// The user's explicitly selected Legacy frontend remains supported.
	path := filepath.Join(dir, "legacy-control.cherri")
	if err := os.WriteFile(path, []byte(cases[0]), 0600); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(dir, "legacy-control.shortcut")
	out, stderr, err := p15RunCLI(t, bin, dir, nil, "compile", path, "--mode=legacy", "--load-all-actions", "--skip-sign", "--output="+target)
	if err != nil {
		t.Fatalf("explicit Legacy control failed: %v %s %s", err, out, stderr)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if _, err := plist.Unmarshal(data, &native); err != nil {
		t.Fatal(err)
	}
	if actions, ok := native["WFWorkflowActions"].([]any); !ok || len(actions) == 0 {
		t.Fatal("explicit Legacy output empty")
	}
}

func TestBuildProfileRequireRemainsSemanticSafety(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		src := "let original = text(\"P15 typed original\")\nlet checked = require(original, type: \"text\", nonEmpty: true, message: \"P15 semantic guard\")\ndebugTrace(\"P15 diagnostic only\")\nquicklook(checked)"
		if legacy {
			src = "const original = text(\"P15 typed original\")\nconst checked = require(original, \"text\", true, \"P15 semantic guard\")\ndebugTrace(\"P15 diagnostic only\")\nquicklook(checked)"
		}
		baseline, err := compileBuildProfileFixture(t, src, legacy, BuildProfileDebug)
		if err != nil {
			t.Fatal(err)
		}
		release, err := compileBuildProfileFixture(t, src, legacy, BuildProfileRelease)
		if err != nil {
			t.Fatal(err)
		}
		counts := func(actions []ShortcutAction) map[string]int {
			out := map[string]int{}
			for _, a := range actions {
				out[a.WFWorkflowActionIdentifier]++
			}
			return out
		}
		b, r := counts(baseline), counts(release)
		for _, id := range []string{"conditional", "alert", "exit", "getitemtype", "count"} {
			key := "is.workflow.actions." + id
			if r[key] == 0 || r[key] != b[key] {
				t.Fatalf("Legacy=%v release changed semantic guard %s: debug=%d release=%d", legacy, id, b[key], r[key])
			}
		}
		if r["is.workflow.actions.appendvariable"] != 0 {
			t.Fatal("Release retained diagnostic log")
		}
		producerID := ""
		checkedID := ""
		for _, a := range release {
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && a.WFWorkflowActionParameters["WFTextActionText"] == "P15 typed original" {
				producerID, _ = a.WFWorkflowActionParameters["UUID"].(string)
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
				if ref, ok := debugLogOutputRef(a.WFWorkflowActionParameters["WFVariable"]); ok && ref.OutputUUID == producerID {
					checkedID, _ = a.WFWorkflowActionParameters["UUID"].(string)
				}
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.previewdocument" {
				ref, ok := debugLogOutputRef(a.WFWorkflowActionParameters["WFInput"])
				if !ok || checkedID == "" || ref.OutputUUID != checkedID {
					t.Fatal("Release broke original producer -> require -> consumer reference chain")
				}
			}
		}
		if producerID == "" || checkedID == "" {
			t.Fatal("Release omitted original producer or passthrough")
		}
	}
}
