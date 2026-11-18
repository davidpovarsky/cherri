package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

func TestP15CLIOptionValuesAndLanguageMode(t *testing.T) {
	binPath := os.Getenv("CHERRI_CLI_VALUE_TEST_BIN")
	if binPath == "" {
		binPath = filepath.Join(t.TempDir(), "cherri-cli-test.exe")
		build := exec.Command("go", "build", "-o", binPath, ".")
		if output, err := build.CombinedOutput(); err != nil {
			t.Fatalf("build actual CLI: %v\n%s", err, output)
		}
	}
	run := func(t *testing.T, arguments ...string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		output, err := exec.CommandContext(ctx, binPath, arguments...).CombinedOutput()
		if ctx.Err() != nil {
			t.Fatalf("CLI timed out: %v\n%s", ctx.Err(), output)
		}
		return output, err
	}

	t.Run("SigningQueryPreserved", func(t *testing.T) {
		const suffix = "/sign?response=json&opaque=a=b=="
		const diagnostic = "P15_EXPECTED_SIGNER_QUERY_REACHED"
		for _, mode := range []string{"file_v2", "file_legacy", "compile", "compile_legacy", "sign"} {
			t.Run(mode, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "probe.cherri")
				if err := os.WriteFile(source, []byte("show(\"P15 option value probe\")\n"), 0644); err != nil {
					t.Fatal(err)
				}
				unsigned := filepath.Join(dir, "probe_unsigned.shortcut")
				if output, err := run(t, source, "--skip-sign", "--no-ansi"); err != nil {
					t.Fatalf("healthy unsigned control failed: %v\n%s", err, output)
				}
				type requestEvidence struct {
					uri, contentType string
				}
				requests := make(chan requestEvidence, 1)
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					requests <- requestEvidence{r.RequestURI, r.Header.Get("Content-Type")}
					_, _ = io.Copy(io.Discard, r.Body)
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, "{\"success\":false,\"error\":{\"message\":\""+diagnostic+"\"}}")
				}))
				defer server.Close()
				target := filepath.Join(dir, "result=with=value.shortcut")
				arguments := []string{source}
				switch mode {
				case "file_legacy":
					arguments = append(arguments, "--legacy", "--load-all-actions")
				case "compile":
					arguments = []string{"compile", source}
				case "compile_legacy":
					arguments = []string{"compile", source, "--mode=legacy", "--load-all-actions"}
				case "sign":
					arguments = []string{"sign", unsigned}
				}
				arguments = append(arguments, "--output="+target, "--signing-server="+server.URL+suffix, "--no-ansi")
				output, err := run(t, arguments...)
				if err == nil {
					t.Errorf("signer fault returned CLI success: %s", output)
				}
				select {
				case received := <-requests:
					if received.uri != suffix {
						t.Errorf("signing URL truncated: got %q, want %q", received.uri, suffix)
					}
					if !strings.HasPrefix(received.contentType, "multipart/form-data;") {
						t.Errorf("wrong signing adapter: %q", received.contentType)
					}
				default:
					t.Error("no request reached the local signing server")
				}
				if !strings.Contains(string(output), diagnostic) {
					t.Errorf("failure had the wrong cause: %s", output)
				}
				if _, err := os.Stat(target); !os.IsNotExist(err) {
					t.Errorf("failed signing left destination artifact: %v", err)
				}
			})
		}
	})

	t.Run("OutputPathPreserved", func(t *testing.T) {
		for _, form := range []string{"file_v2", "file_legacy", "compile_v2", "compile_legacy", "short_alias"} {
			t.Run(form, func(t *testing.T) {
				dir := t.TempDir()
				source := filepath.Join(dir, "probe.cherri")
				if err := os.WriteFile(source, []byte("show(\"P15 exact output path\")\n"), 0644); err != nil {
					t.Fatal(err)
				}
				target := filepath.Join(dir, "result=with=value.shortcut")
				arguments := []string{source}
				if strings.HasPrefix(form, "compile_") {
					arguments = []string{"compile", source}
				}
				if strings.HasSuffix(form, "_legacy") {
					arguments = append(arguments, "--mode=legacy", "--load-all-actions")
				}
				option := "--output="
				if form == "short_alias" {
					option = "-o="
				}
				arguments = append(arguments, option+target, "--skip-sign", "--no-ansi")
				if output, err := run(t, arguments...); err != nil {
					t.Fatalf("compile exact output path: %v\n%s", err, output)
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
					t.Fatal("exact output path did not contain a nonempty native plist")
				}
				if _, err := os.Stat(filepath.Join(dir, "result")); !os.IsNotExist(err) {
					t.Errorf("truncated output path was also written: %v", err)
				}
			})
		}
	})

	t.Run("CompileLanguageSelection", func(t *testing.T) {
		const v2 = "let message = \"V2 mode probe\"\nshow(message)\n"
		const legacy = "@message = \"Legacy mode probe\"\nshow(\"{@message}\")\n"
		cases := []struct {
			name, source, diagnostic string
			options                  []string
			success                  bool
		}{
			{"default_v2", v2, "", nil, true},
			{"explicit_v2", v2, "", []string{"--mode=v2"}, true},
			{"legacy_flag", legacy, "", []string{"--legacy"}, true},
			{"legacy_mode", legacy, "", []string{"--mode=legacy"}, true},
			{"legacy_both", legacy, "", []string{"--legacy", "--mode=legacy"}, true},
			{"legacy_is_not_default", legacy, "E_LEGACY_SYNTAX", nil, false},
			{"conflicting_modes", legacy, "conflicting compile mode", []string{"--legacy", "--mode=v2"}, false},
			{"invalid_mode", v2, "unsupported compile mode", []string{"--mode=future"}, false},
			{"missing_mode_value", v2, "--mode requires", []string{"--mode="}, false},
		}
		for _, subcommand := range []bool{false, true} {
			for _, tc := range cases {
				name := "file/" + tc.name
				if subcommand {
					name = "compile/" + tc.name
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					source := filepath.Join(dir, "probe.cherri")
					if err := os.WriteFile(source, []byte(tc.source), 0644); err != nil {
						t.Fatal(err)
					}
					arguments := []string{source}
					if subcommand {
						arguments = []string{"compile", source}
					}
					arguments = append(arguments, tc.options...)
					arguments = append(arguments, "--skip-sign", "--load-all-actions", "--no-ansi")
					output, err := run(t, arguments...)
					if (err == nil) != tc.success {
						t.Fatalf("expected compilation success=%v, error=%v\n%s", tc.success, err, output)
					}
					target := filepath.Join(dir, "probe_unsigned.shortcut")
					if tc.success {
						data, readErr := os.ReadFile(target)
						if readErr != nil {
							t.Fatal(readErr)
						}
						var native map[string]any
						if _, decodeErr := plist.Unmarshal(data, &native); decodeErr != nil {
							t.Fatal(decodeErr)
						}
						actions, ok := native["WFWorkflowActions"].([]any)
						if !ok || len(actions) == 0 {
							t.Fatal("successful CLI did not emit a nonempty native action plist")
						}
					} else {
						if tc.diagnostic != "" && !strings.Contains(string(output), tc.diagnostic) {
							t.Errorf("wrong failure cause, expected %q:\n%s", tc.diagnostic, output)
						}
						if _, statErr := os.Stat(target); !os.IsNotExist(statErr) {
							t.Errorf("rejected compile left an artifact: %v", statErr)
						}
					}
				})
			}
		}
	})
}

func TestP15CLIOptionValueDuplicatePrecedence(t *testing.T) {
	cases := []struct {
		name      string
		argv      []string
		parsed    map[string]string
		key, want string
	}{
		{"full_query", []string{"--signing-server=https://example.invalid/sign?response=json&opaque=a=b=="}, map[string]string{"signing-server": "https://example.invalid/sign?response"}, "signing-server", "https://example.invalid/sign?response=json&opaque=a=b=="},
		{"output_path", []string{"--output=result=a=b.shortcut"}, map[string]string{"output": "result"}, "output", "result=a=b.shortcut"},
		{"short_alias", []string{"-o=result=a.shortcut"}, map[string]string{"o": "result"}, "o", "result=a.shortcut"},
		{"duplicate_last", []string{"--output=first=a", "--output=second=b"}, map[string]string{"output": "second"}, "output", "second=b"},
		{"duplicate_simple_last", []string{"--output=first=a", "--output=plain"}, map[string]string{"output": "plain"}, "output", "plain"},
		{"duplicate_valueless_last", []string{"--output=first=a", "--output"}, map[string]string{"output": ""}, "output", ""},
		{"positional_untouched", []string{"output=not=anoption"}, map[string]string{"output": "not"}, "output", "not"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			preserveCLIOptionValues(tc.argv, tc.parsed)
			if got := tc.parsed[tc.key]; got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}
