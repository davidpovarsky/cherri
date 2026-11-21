package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"howett.net/plist"
)

const p15RP43Text = "P15_RP43_NATIVE_OUTPUT"

func p15RP43CompileArgs(route, input, output string, options ...string) []string {
	var arguments []string
	if strings.HasPrefix(route, "compile_") {
		arguments = append(arguments, "compile")
	}
	arguments = append(arguments, input)
	if strings.HasSuffix(route, "_legacy") {
		arguments = append(arguments, "--legacy", "--load-all-actions")
	}
	arguments = append(arguments, "--output="+output, "--no-ansi")
	return append(arguments, options...)
}

func p15RP43AssertNativeOutput(t *testing.T, data []byte, legacy bool) {
	t.Helper()
	var document map[string]any
	if _, err := plist.Unmarshal(data, &document); err != nil {
		t.Fatalf("public output is not a native plist: %v", err)
	}
	var actions, ok = document["WFWorkflowActions"].([]any)
	count := 1
	if !legacy {
		count++
		if err := p15M02MainVoidRowsError(actions); err != nil {
			t.Fatal(err)
		}
	}
	if !ok || len(actions) != count {
		t.Fatalf("source mode changed the complete output graph: legacy=%t actions=%#v", legacy, document["WFWorkflowActions"])
	}
	var action, actionOK = actions[0].(map[string]any)
	var parameters, paramsOK = action["WFWorkflowActionParameters"].(map[string]any)
	if !actionOK || !paramsOK || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" {
		t.Fatalf("wrong native action in requested output: %#v", actions[0])
	}
	var value = parameters["WFTextActionText"]
	if value == p15RP43Text {
		return
	}
	// The output-path contract does not select a particular supported literal
	// text wire form, but it must preserve the complete literal with no refs.
	var token, tokenOK = value.(map[string]any)
	var body, bodyOK = token["Value"].(map[string]any)
	if !tokenOK || !bodyOK || token["WFSerializationType"] != "WFTextTokenString" || body["string"] != p15RP43Text {
		t.Fatalf("requested source payload was lost: %#v", value)
	}
	if attachments, exists := body["attachmentsByRange"]; exists {
		var entries, valid = attachments.(map[string]any)
		if !valid || len(entries) != 0 {
			t.Fatalf("literal source acquired native references: %#v", attachments)
		}
	}
}

func p15RP43AssertSameFiles(t *testing.T, directory string, before map[string][32]byte) {
	t.Helper()
	var after = p15DirectoryDigest(t, directory)
	if !reflect.DeepEqual(after, before) {
		t.Fatalf("failed public operation changed source/destination/sibling files: before=%v after=%v", before, after)
	}
	p15AssertNoSigningScratch(t, directory)
}

func TestP15RP43PublicOutputDestinations(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy"} {
		t.Run(route, func(t *testing.T) {
			for _, kind := range []string{"absolute_unicode", "relative_unicode", "destination_directory", "parent_is_file", "missing_parent"} {
				t.Run(kind, func(t *testing.T) {
					var directory = t.TempDir()
					var input = filepath.Join(directory, "source שלום.cherri")
					var source = []byte("text(\"" + p15RP43Text + "\")\n")
					p15SchemaWrite(t, input, source)
					var sibling = strings.TrimSuffix(input, ".cherri") + "_unsigned.shortcut"
					var originalSibling = []byte("P15_RP43_PRIOR_SIBLING\x00\xff")
					p15SchemaWrite(t, sibling, originalSibling)
					var output = filepath.Join(directory, "result שלום space.shortcut")
					var original = []byte("P15_RP43_PRIOR_DESTINATION\x00\xff")
					var success = kind == "absolute_unicode" || kind == "relative_unicode"
					switch kind {
					case "absolute_unicode", "relative_unicode":
						p15SchemaWrite(t, output, original)
					case "destination_directory":
						if err := os.Mkdir(output, 0700); err != nil {
							t.Fatal(err)
						}
						p15SchemaWrite(t, filepath.Join(output, "retained.txt"), original)
					case "parent_is_file":
						p15SchemaWrite(t, output, original)
						output = filepath.Join(output, "child.shortcut")
					case "missing_parent":
						output = filepath.Join(directory, "missing", "child.shortcut")
					}
					var argumentOutput = output
					if kind == "relative_unicode" {
						argumentOutput = filepath.Base(output)
					}
					var before = p15DirectoryDigest(t, directory)
					var stdout, stderr, err = p15RunCLI(t, binary, directory, nil, p15RP43CompileArgs(route, input, argumentOutput, "--skip-sign")...)
					if (err == nil) != success {
						t.Errorf("wrong public exit status for output destination: success=%v err=%v stdout=%s stderr=%s", success, err, stdout, stderr)
					}
					if !success {
						if len(bytes.TrimSpace(append(stdout, stderr...))) == 0 {
							t.Error("failed destination returned no diagnostic")
						}
						p15RP43AssertSameFiles(t, directory, before)
						return
					}
					var actual, readErr = os.ReadFile(output)
					if readErr != nil || bytes.Equal(actual, original) {
						t.Fatalf("requested destination was not replaced: %v", readErr)
					}
					p15RP43AssertNativeOutput(t, actual, strings.HasSuffix(route, "_legacy"))
					var after = p15DirectoryDigest(t, directory)
					delete(before, filepath.Base(output))
					delete(after, filepath.Base(output))
					if !reflect.DeepEqual(after, before) {
						t.Fatalf("explicit output modified an unrelated source/sibling path: before=%v after=%v", before, after)
					}
					p15AssertNoSigningScratch(t, directory)
				})
			}
		})
	}
}

func TestP15RP43LinuxOutputPermissions(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("actual Linux permission and remote-selection proof requires a Linux process")
	}
	var binary = p15AcceptanceCLI(t)
	for _, route := range []string{"compile_v2", "file_v2", "compile_legacy", "file_legacy"} {
		t.Run(route, func(t *testing.T) {
			var directory = t.TempDir()
			var input = filepath.Join(directory, "source.cherri")
			p15SchemaWrite(t, input, []byte("text(\""+p15RP43Text+"\")\n"))
			var outputDirectory = filepath.Join(directory, "denied שלום")
			if err := os.Mkdir(outputDirectory, 0700); err != nil {
				t.Fatal(err)
			}
			var output = filepath.Join(outputDirectory, "retained.shortcut")
			p15SchemaWrite(t, output, []byte("P15_RP43_PERMISSION_RETAIN\x00\xff"))
			if err := os.Chmod(outputDirectory, 0500); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = os.Chmod(outputDirectory, 0700) })
			// Observe denial from the same identity before testing the compiler;
			// chmod under a privileged runner cannot count as a permission test.
			var probe = filepath.Join(outputDirectory, "denial-control")
			if err := os.WriteFile(probe, []byte("must fail"), 0600); !os.IsPermission(err) {
				t.Fatalf("runner did not observe an actual write permission denial: %v", err)
			}
			var before = p15DirectoryDigest(t, directory)
			var stdout, stderr, err = p15RunCLI(t, binary, directory, nil, p15RP43CompileArgs(route, input, output, "--skip-sign")...)
			if err == nil || !strings.Contains(strings.ToLower(string(stdout)+string(stderr)), "permission denied") {
				t.Errorf("permission failure was swallowed or misreported: err=%v stdout=%s stderr=%s", err, stdout, stderr)
			}
			p15RP43AssertSameFiles(t, directory, before)
			if err := os.Chmod(outputDirectory, 0700); err != nil {
				t.Fatal(err)
			}
			stdout, stderr, err = p15RunCLI(t, binary, directory, nil, p15RP43CompileArgs(route, input, output, "--skip-sign")...)
			if err != nil {
				t.Fatalf("restored output permission did not restore compilation: %v %s %s", err, stdout, stderr)
			}
			var data, readErr = os.ReadFile(output)
			if readErr != nil {
				t.Fatal(readErr)
			}
			p15RP43AssertNativeOutput(t, data, strings.HasSuffix(route, "_legacy"))
		})
	}
	t.Log(fmt.Sprintf("Observed real output permission denial and restored compilation on %s/%s", runtime.GOOS, runtime.GOARCH))
}
