package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15EditFixture struct {
	root, skill, home, binary string
	env                       map[string]string
}

func p15NewEditFixture(t *testing.T) p15EditFixture {
	t.Helper()
	root := t.TempDir()
	skill := filepath.Join(root, "skill")
	for _, relative := range []string{"compatibility-manifest.json", "scripts/common.sh",
		"scripts/prepare-edit.sh", "scripts/decompile.sh", "scripts/build.sh"} {
		p15InstallWrite(t, filepath.Join(skill, relative),
			p15InstallRead(t, filepath.Join("skills/cherri-shortcuts", relative)))
	}
	f := p15EditFixture{root: root, skill: skill, home: filepath.Join(root, "empty-home"), binary: roundTripBinary(t)}
	f.env = map[string]string{"SKILL_DIR": filepath.ToSlash(skill), "CHERRI_HOME": filepath.ToSlash(f.home),
		"CHERRI_BIN": filepath.ToSlash(f.binary)}
	return f
}

func p15EditPath(path string) string {
	value := filepath.ToSlash(path)
	if isWindowsRuntime() && len(value) > 2 && value[1] == ':' {
		return "/" + strings.ToLower(value[:1]) + value[2:]
	}
	return value
}

func (f p15EditFixture) run(t *testing.T, script string, argv ...string) (string, error) {
	t.Helper()
	return p15InstallRun(t, filepath.Join(f.skill, "scripts", script+".sh"), f.env, argv...)
}

func (f p15EditFixture) source(t *testing.T, name, value string) string {
	t.Helper()
	path := filepath.Join(f.root, name+".cherri")
	p15InstallWrite(t, path, []byte(fmt.Sprintf("let message = text(%q)\nshow(message)\n", value)))
	return path
}

func (f p15EditFixture) compile(t *testing.T, source, output string) []byte {
	t.Helper()
	cmd := exec.Command(f.binary, "compile", source, "--skip-sign", "--output="+output, "--no-ansi")
	cmd.Env = p15InstallEnv(nil)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual unsigned CLI: %v\n%s", err, out)
	}
	return p15InstallRead(t, output)
}

func p15EditDocument(t *testing.T, path string) map[string]any {
	t.Helper()
	var doc map[string]any
	if _, err := plist.Unmarshal(p15InstallRead(t, path), &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

func TestP15K05BuildPreservesExistingFiles(t *testing.T) {
	for _, test := range []string{"source_destination_alias", "invalid_source_keeps_previous_output", "healthy_unsigned_control"} {
		t.Run(test, func(t *testing.T) {
			f := p15NewEditFixture(t)
			source := f.source(t, "source", "P15_K05_ORIGINAL")
			original := p15InstallRead(t, source)
			output := filepath.Join(f.root, "delivery.shortcut")
			old := []byte("P15_PREVIOUS_DELIVERY")
			p15InstallWrite(t, output, old)
			switch test {
			case "source_destination_alias":
				out, err := f.run(t, "build", p15EditPath(source), p15EditPath(source), "--unsigned")
				if err == nil {
					t.Errorf("source/output alias unexpectedly succeeded: %s", out)
				}
				now, e := os.ReadFile(source)
				if e != nil || string(now) != string(original) {
					t.Errorf("build destroyed caller source: read=%v before=%x after=%x", e, sha256.Sum256(original), sha256.Sum256(now))
				}
			case "invalid_source_keeps_previous_output":
				p15InstallWrite(t, source, []byte("let broken = definitelyMissingAction()\n"))
				out, err := f.run(t, "build", p15EditPath(source), p15EditPath(output), "--unsigned")
				if err == nil {
					t.Errorf("invalid source unexpectedly built: %s", out)
				}
				now, e := os.ReadFile(output)
				if e != nil || string(now) != string(old) {
					t.Errorf("failed build destroyed prior delivery: read=%v before=%x after=%x", e, sha256.Sum256(old), sha256.Sum256(now))
				}
			case "healthy_unsigned_control":
				out, err := f.run(t, "build", p15EditPath(source), p15EditPath(output), "--unsigned")
				if err != nil {
					t.Fatalf("healthy actual wrapper: %v\n%s", err, out)
				}
				doc := p15EditDocument(t, output)
				if len(doc["WFWorkflowActions"].([]any)) != 2 {
					t.Fatal("healthy build did not produce expected two-action workflow")
				}
				if string(p15InstallRead(t, source)) != string(original) {
					t.Fatal("healthy build mutated source")
				}
			}
		})
	}
}

func TestP15K05PreparePreservesOriginal(t *testing.T) {
	f := p15NewEditFixture(t)
	source := f.source(t, "first", "P15_K05_FIRST")
	first := filepath.Join(f.root, "first.shortcut")
	firstBytes := f.compile(t, source, first)
	workspace := filepath.Join(f.root, "prepared workspace")
	out, err := f.run(t, "prepare-edit", p15EditPath(first), p15EditPath(workspace))
	if err != nil {
		paths, _ := filepath.Glob(filepath.Join(workspace, "source", "*.cherri"))
		for _, path := range paths {
			t.Logf("prepared source: %s", p15InstallRead(t, path))
		}
		t.Logf("original document: %#v", p15EditDocument(t, first))
		validationPath := filepath.Join(workspace, "builds", "validation.shortcut")
		if _, statErr := os.Stat(validationPath); statErr == nil {
			t.Logf("validation document: %#v", p15EditDocument(t, validationPath))
		}
		t.Fatalf("healthy actual prepare: %v\n%s", err, out)
	}
	original := filepath.Join(workspace, "original", "shortcut.plist")
	if string(p15InstallRead(t, original)) != string(firstBytes) {
		t.Fatal("initial original copy differs")
	}
	secondSource := f.source(t, "second", "P15_K05_SECOND")
	second := filepath.Join(f.root, "second.shortcut")
	f.compile(t, secondSource, second)
	out, err = f.run(t, "prepare-edit", p15EditPath(second), p15EditPath(workspace))
	if err == nil {
		t.Errorf("occupied workspace accepted a new original: %s", out)
	}
	if string(p15InstallRead(t, original)) != string(firstBytes) {
		t.Error("second prepare overwrote immutable original")
	}
	if string(p15InstallRead(t, first)) != string(firstBytes) {
		t.Error("prepare changed caller input")
	}
}

func (f p15EditFixture) prepare(t *testing.T) (string, string, string, []byte) {
	t.Helper()
	source := f.source(t, "workflow", "P15_K05_ORIGINAL")
	input := filepath.Join(f.root, "input.shortcut")
	f.compile(t, source, input)
	doc := p15EditDocument(t, input)
	doc["P15PreservedMetadata"] = map[string]any{"text": "P15_METADATA_KEEP", "blob": []byte{0, 1, 255}, "large": uint64(9007199254740993), "flag": false}
	raw, err := plist.Marshal(doc, plist.BinaryFormat)
	if err != nil {
		t.Fatal(err)
	}
	p15InstallWrite(t, input, raw)
	workspace := filepath.Join(f.root, "edit-workspace")
	out, err := f.run(t, "prepare-edit", p15EditPath(input), p15EditPath(workspace))
	if err != nil {
		t.Fatalf("actual binary-plist prepare: %v\n%s", err, out)
	}
	paths, err := filepath.Glob(filepath.Join(workspace, "source", "*.cherri"))
	if err != nil || len(paths) != 1 {
		t.Fatalf("expected one source, got %v %v", paths, err)
	}
	prepared := paths[0]
	content := string(p15InstallRead(t, prepared))
	if !strings.HasPrefix(strings.TrimSpace(content), "native.workflow(native.plist(") {
		t.Fatalf("canonical hybrid provenance missing; actual generated source:\n%s", content)
	}
	cmd := exec.Command(f.binary, "check", prepared, "--json")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("prepared source is not valid V2: %v\n%s", err, output)
	}
	original := filepath.Join(workspace, "original", "shortcut.plist")
	if string(p15InstallRead(t, original)) != string(raw) || string(p15InstallRead(t, input)) != string(raw) {
		t.Fatal("original bytes changed")
	}
	return workspace, prepared, original, raw
}

func p15EditReplaceBody(t *testing.T, path, before, after string) {
	t.Helper()
	lines := strings.SplitAfter(string(p15InstallRead(t, path)), "\n")
	count := 0
	for _, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "native.workflow(") {
			count += strings.Count(line, before)
		}
	}
	if count != 1 {
		t.Fatalf("expected one editable body literal, got %d", count)
	}
	for i, line := range lines {
		if !strings.HasPrefix(strings.TrimSpace(line), "native.workflow(") {
			lines[i] = strings.Replace(line, before, after, 1)
		}
	}
	p15InstallWrite(t, path, []byte(strings.Join(lines, "")))
}

func p15EditWritePolicy(t *testing.T, path string, original []byte) {
	t.Helper()
	policy := map[string]any{"schemaVersion": 1, "originalSHA256": fmt.Sprintf("%x", sha256.Sum256(original)),
		"changes": []any{map[string]any{"path": []string{"WFWorkflowActions", "0", "WFWorkflowActionParameters", "WFTextActionText"}, "before": "P15_K05_ORIGINAL", "after": "P15_K05_EDITED"}}}
	raw, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p15InstallWrite(t, path, append(raw, '\n'))
}

func TestP15K05RepairBuildImportEdit(t *testing.T) {
	f := p15NewEditFixture(t)
	workspace, source, original, originalBytes := f.prepare(t)
	good := p15InstallRead(t, source)
	// A real named-argument diagnostic is repaired in the actual generated V2
	// body; neither the original nor its provenance is rewritten.
	p15InstallWrite(t, source, append(append([]byte(nil), good...), []byte("\nshow(\"invalid\", unexpected: true)\n")...))
	cmd := exec.Command(f.binary, "check", source, "--json")
	out, err := cmd.CombinedOutput()
	if err == nil || !strings.Contains(string(out), "E_UNKNOWN_ARGUMENT") {
		t.Fatalf("actual repair diagnostic absent: %v\n%s", err, out)
	}
	p15InstallWrite(t, source, good)
	p15EditReplaceBody(t, source, "P15_K05_ORIGINAL", "P15_K05_EDITED")
	policy := filepath.Join(workspace, "allowed-delta.json")
	p15EditWritePolicy(t, policy, originalBytes)
	output := filepath.Join(workspace, "builds", "edited.shortcut")
	outText, err := f.run(t, "build", p15EditPath(source), p15EditPath(output), "--unsigned", "--original="+p15EditPath(original), "--allowed-delta="+p15EditPath(policy))
	if err != nil {
		t.Fatalf("authorized actual edit failed: %v\n%s", err, outText)
	}
	expected := p15EditDocument(t, original)
	expected["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFTextActionText"] = "P15_K05_EDITED"
	if !reflect.DeepEqual(expected, p15EditDocument(t, output)) {
		t.Fatal("actual output differs beyond the requested Text field")
	}
	if string(p15InstallRead(t, original)) != string(originalBytes) {
		t.Fatal("original changed after repair/edit")
	}
}

type p15EditCapture struct{ log, gate, input, manifest, receipt string }

func (f p15EditFixture) observe(t *testing.T, denyComparison bool) p15EditCapture {
	t.Helper()
	capture := p15EditCapture{log: filepath.Join(f.root, "commands.log"), gate: filepath.Join(f.root, "gated.shortcut"),
		input: filepath.Join(f.root, "sign-input.shortcut"), manifest: filepath.Join(f.root, "sign-provenance.json"), receipt: filepath.Join(f.root, "gate.json")}
	shim := filepath.Join(f.root, "observed-compiler")
	script := "#!/bin/sh\nset -eu\nprintf '%s\\n' \"$1\" >> " + p15InstallShellQuote(capture.log) + "\n"
	if denyComparison {
		script += "if [ \"$1\" = verify-edit ]; then echo '[P15_COMPARATOR_REMOVED]' >&2; exit 127; fi\n"
	}
	script += "if [ \"$1\" = verify-edit ]; then\n  cp \"$3\" " + p15InstallShellQuote(capture.gate) + "\n  " + p15InstallShellQuote(f.binary) + " \"$@\" > " + p15InstallShellQuote(capture.receipt) + "\n  cat " + p15InstallShellQuote(capture.receipt) + "\n  exit 0\nfi\n"
	script += "if [ \"$1\" = sign ]; then\n  cp \"$2\" " + p15InstallShellQuote(capture.input) + "\n  exec " + p15InstallShellQuote(f.binary) + " \"$@\" " + p15InstallShellQuote("--manifest-out="+capture.manifest) + " " + p15InstallShellQuote("--compiler-sha="+p15InstallGit(t, ".", "rev-parse", "HEAD")) + "\nfi\n"
	script += "exec " + p15InstallShellQuote(f.binary) + " \"$@\"\n"
	p15InstallWrite(t, shim, []byte(script))
	f.env["CHERRI_BIN"] = filepath.ToSlash(shim)
	return capture
}

func TestP15K05SigningGateAndExactInput(t *testing.T) {
	f := p15NewEditFixture(t)
	workspace, source, original, originalBytes := f.prepare(t)
	p15EditReplaceBody(t, source, "P15_K05_ORIGINAL", "P15_K05_EDITED")
	validSource := p15InstallRead(t, source)
	policy := filepath.Join(workspace, "allowed-delta.json")
	p15EditWritePolicy(t, policy, originalBytes)
	output := filepath.Join(workspace, "builds", "delivery.shortcut")
	oldOutput := []byte("P15_PREVIOUS_SIGNED_DELIVERY")
	requests := make(chan []byte, 8)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(4 << 20); err != nil {
			requests <- nil
			http.Error(w, "bad multipart", 400)
			return
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			requests <- nil
			http.Error(w, "missing file", 400)
			return
		}
		data, err := io.ReadAll(file)
		_ = file.Close()
		if err != nil {
			requests <- nil
			http.Error(w, "bad body", 400)
			return
		}
		requests <- data
		http.Error(w, "P15_SYNTHETIC_SIGNING_FAILURE", 503)
	}))
	defer server.Close()
	f.env["CHERRI_SIGNING_SERVER"] = server.URL + "/sign?response=json"
	for _, fault := range []string{"missing_policy", "missing_comparator", "extra_action", "metadata_loss", "reference_damage", "valid_gate_signer_failure"} {
		t.Run(fault, func(t *testing.T) {
			p15InstallWrite(t, source, validSource)
			p15InstallWrite(t, output, oldOutput)
			capture := f.observe(t, fault == "missing_comparator")
			_ = os.Remove(capture.log)
			argv := []string{p15EditPath(source), p15EditPath(output), "--signed", "--original=" + p15EditPath(original), "--allowed-delta=" + p15EditPath(policy)}
			switch fault {
			case "missing_policy":
				argv = argv[:3]
			case "extra_action":
				p15InstallWrite(t, source, append(append([]byte(nil), validSource...), []byte("\nshow(\"P15_UNAUTHORIZED\")\n")...))
			case "metadata_loss":
				p15EditRemoveMetadata(t, source)
			case "reference_damage":
				p15EditReplaceBody(t, source, `f"{message}"`, `"P15_UNAUTHORIZED_REFERENCE"`)
			}
			out, err := f.run(t, "build", argv...)
			if err == nil {
				t.Fatalf("fault unexpectedly finalized: %s", out)
			}
			if string(p15InstallRead(t, output)) != string(oldOutput) {
				t.Fatal("failed finalization destroyed previous output")
			}
			if string(p15InstallRead(t, original)) != string(originalBytes) {
				t.Fatal("failed finalization changed original")
			}
			log, _ := os.ReadFile(capture.log)
			if fault == "valid_gate_signer_failure" {
				select {
				case request := <-requests:
					gated, input := p15InstallRead(t, capture.gate), p15InstallRead(t, capture.input)
					if len(request) == 0 || string(request) != string(gated) || string(request) != string(input) {
						t.Fatal("signing request was not the exact gated bytes")
					}
					if strings.Count(string(log), "compile\n") != 1 || strings.Count(string(log), "sign\n") != 1 || strings.Index(string(log), "verify-edit\n") > strings.Index(string(log), "sign\n") {
						t.Fatalf("wrong production event order: %s", log)
					}
					if !strings.Contains(out, "P15_SYNTHETIC_SIGNING_FAILURE") {
						t.Fatalf("expected real signer failure missing: %s", out)
					}
				default:
					t.Fatalf("valid gate never reached real signing HTTP path: %s", out)
				}
			} else {
				select {
				case <-requests:
					t.Fatal("unauthorized edit reached signing service")
				default:
				}
				if strings.Contains(string(log), "sign\n") {
					t.Fatalf("gate failure invoked signer: %s", log)
				}
				if !strings.Contains(out, "CHERRI_EDIT_") {
					t.Fatalf("failure was not the edit gate: %s", out)
				}
			}
		})
	}
}

func TestP15K05PrepareRequiresComparator(t *testing.T) {
	f := p15NewEditFixture(t)
	source := f.source(t, "source", "P15_K05_ORIGINAL")
	input := filepath.Join(f.root, "input.shortcut")
	original := f.compile(t, source, input)
	f.observe(t, true)
	workspace := filepath.Join(f.root, "blocked-workspace")
	out, err := f.run(t, "prepare-edit", p15EditPath(input), p15EditPath(workspace))
	if err == nil || !strings.Contains(out, "P15_COMPARATOR_REMOVED") {
		t.Fatalf("missing comparison was accepted or wrong failure: %v\n%s", err, out)
	}
	if string(p15InstallRead(t, input)) != string(original) || string(p15InstallRead(t, filepath.Join(workspace, "original", "shortcut.plist"))) != string(original) {
		t.Fatal("missing-comparator path changed original")
	}
}

func p15EditRemoveMetadata(t *testing.T, path string) {
	t.Helper()
	header, body, ok := strings.Cut(string(p15InstallRead(t, path)), "\n")
	const prefix = "native.workflow(native.plist("
	const suffix = "), exact: true)"
	if !ok || !strings.HasPrefix(header, prefix) || !strings.HasSuffix(header, suffix) {
		t.Fatal("canonical V2 workflow header not found")
	}
	literal := strings.TrimSuffix(strings.TrimPrefix(header, prefix), suffix)
	payload, err := strconv.Unquote(literal)
	if err != nil {
		t.Fatal(err)
	}
	var native map[string]any
	if _, err := plist.Unmarshal([]byte(payload), &native); err != nil {
		t.Fatal(err)
	}
	if _, ok := native["P15PreservedMetadata"]; !ok {
		t.Fatal("metadata loss mutation anchor absent")
	}
	delete(native, "P15PreservedMetadata")
	changed, err := plist.Marshal(native, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	p15InstallWrite(t, path, []byte(prefix+strconv.Quote(string(changed))+suffix+"\n"+body))
}

func TestP15K05ExplicitLegacyImportRemainsAvailable(t *testing.T) {
	f := p15NewEditFixture(t)
	source := f.source(t, "legacy-control", "P15_EXPLICIT_LEGACY")
	original := filepath.Join(f.root, "legacy-original.shortcut")
	f.compile(t, source, original)
	directory := filepath.Join(f.root, "explicit-legacy")
	out, err := f.run(t, "decompile", p15EditPath(original), p15EditPath(directory), "--mode=legacy")
	if err != nil {
		t.Fatalf("explicit Legacy import: %v\n%s", err, out)
	}
	legacySource := filepath.Join(directory, "shortcut.cherri")
	content := p15InstallRead(t, legacySource)
	if !strings.HasPrefix(string(content), "#workflowheader ") {
		t.Fatal("explicit Legacy output lost native provenance")
	}
	target := filepath.Join(directory, "legacy.shortcut")
	cmd := exec.Command(f.binary, "compile", legacySource, "--legacy", "--skip-sign", "--output="+target, "--no-ansi")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("public explicit Legacy compile: %v\n%s", err, output)
	}
	if _, err := os.Stat(target); os.IsNotExist(err) {
		generated, err := filepath.Glob(filepath.Join(directory, "*_unsigned.shortcut"))
		if err != nil || len(generated) != 1 {
			t.Fatalf("expected one explicit Legacy unsigned output: %v %v", generated, err)
		}
		target = generated[0]
	}
	if !reflect.DeepEqual(p15EditDocument(t, original), p15EditDocument(t, target)) {
		t.Fatal("explicit Legacy wrapper/control round trip changed native document")
	}
}
