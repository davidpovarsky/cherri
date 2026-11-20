package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func p15BRG21ProcessObservers(t *testing.T, marker string) string {
	t.Helper()
	var directory = t.TempDir()
	var source = filepath.Join(directory, "observer.go")
	var body = "package main\nimport (\"os\";\"path/filepath\")\nfunc main(){if err:=os.WriteFile(os.Getenv(\"CHERRI_BRG21_PROCESS_MARKER\"),[]byte(filepath.Base(os.Args[0])),0600);err!=nil{os.Exit(94)};os.Exit(93)}\n"
	p15ModuleWrite(t, source, body)
	var suffix string
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	var binary = filepath.Join(directory, "observer"+suffix)
	var ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, "go", "build", "-o", binary, source)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build actual process observer: %v context=%v\n%s", err, ctx.Err(), output)
	}
	var data, err = os.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"shortcuts", "open", "git", "curl", "xcrun"} {
		if err := os.WriteFile(filepath.Join(directory, name+suffix), data, 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("CHERRI_BRG21_PROCESS_MARKER", marker)
	return directory
}

func TestBackendRecovery21PublicAndServiceEffects(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var workspace = t.TempDir()
	var entry = filepath.Join(workspace, "entry.cherri")
	var target = filepath.Join(workspace, "must-not-exist.shortcut")
	var marker = filepath.Join(workspace, "process-was-invoked")
	var observers = p15BRG21ProcessObservers(t, marker)
	var requests atomic.Int32
	var server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, "{\"success\":false,\"error\":{\"message\":\"BRG21_ACTUAL_SIGNER_OBSERVER\"}}")
	}))
	defer server.Close()
	t.Run("observer_controls", func(t *testing.T) {
		var suffix string
		if runtime.GOOS == "windows" {
			suffix = ".exe"
		}
		for _, name := range []string{"shortcuts", "open", "git", "curl", "xcrun"} {
			var _, _, err = p15RunCLI(t, filepath.Join(observers, name+suffix), workspace, nil)
			exit, okay := err.(*exec.ExitError)
			if !okay || exit.ExitCode() != 93 {
				t.Fatal("actual process observer was not executed", name, err)
			}
			var observed, readErr = os.ReadFile(marker)
			if readErr != nil || string(observed) != name+suffix {
				t.Fatal("process observer identity was not recorded", name, readErr)
			}
			if err := os.Remove(marker); err != nil {
				t.Fatal(err)
			}
		}
		var client = &http.Client{Timeout: 5 * time.Second}
		var response, err = client.Get(server.URL + "/positive-observer-control")
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		if err := response.Body.Close(); err != nil {
			t.Fatal(err)
		}
		if requests.Load() != 1 {
			t.Fatal("actual network observer did not see exactly one request")
		}
		requests.Store(0)
	})
	var valid = fmt.Sprintf("openURL(%q)\n", server.URL+"/authored-runtime-action")
	var specimens = []struct {
		name, source, diagnostic string
		valid                    bool
	}{
		{"valid", valid, "", true},
		{"invalid_action", valid + "BRG21MissingAction()\n", "BRG21MissingAction", false},
		{"remote_import", fmt.Sprintf("import %q as remote\n", server.URL+"/remote.cherri"), "E_WORKSPACE_ACCESS", false},
		{"missing_local_import", "import \"./missing.cherri\" as localDependency\n", "E_MODULE_DEPENDENCY", false},
	}
	var assertNoEffects = func(t *testing.T, before map[string][32]byte) {
		t.Helper()
		if requests.Load() != 0 {
			t.Fatalf("pure/rejected operation made %d actual network requests", requests.Load())
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("pure/rejected operation invoked a signing/open/install process: %v", err)
		}
		if after := p15DirectoryDigest(t, workspace); !reflect.DeepEqual(before, after) {
			t.Fatal("pure/rejected operation modified source, installed a dependency or emitted an artifact")
		}
	}
	var svc = service.NewService(nil)
	for index, specimen := range specimens {
		p15ModuleWrite(t, entry, specimen.source)
		var before = p15DirectoryDigest(t, workspace)
		t.Run("service_"+specimen.name, func(t *testing.T) {
			if index == 0 {
				svc.OpenDocument(entry, 1, specimen.source)
			} else {
				svc.ChangeDocument(entry, index+1, specimen.source)
			}
			var diagnostics, _ = svc.Analyze(entry)
			var errors []analysis.Diagnostic
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					errors = append(errors, diagnostic)
				}
			}
			if specimen.valid {
				if len(errors) != 0 {
					t.Fatal("valid analysis failed", errors)
				}
			} else if len(errors) == 0 || !strings.Contains(fmt.Sprint(errors), specimen.diagnostic) {
				t.Fatal("service omitted the actual rejection cause", errors)
			}
			assertNoEffects(t, before)
			var output, err = CompileSourceToPlist(entry, specimen.source)
			if specimen.valid {
				if err != nil || !p15ModuleContains(t, output, server.URL+"/authored-runtime-action") {
					t.Fatal("valid compile did not serialize the actual authored runtime action", err)
				}
			} else if err == nil || len(output) != 0 || !strings.Contains(err.Error(), specimen.diagnostic) {
				t.Fatalf("invalid public library compile returned bytes or wrong rejection: %d %v", len(output), err)
			}
			assertNoEffects(t, before)
		})
		for _, route := range []string{"check", "compile", "file"} {
			if specimen.valid && route != "check" {
				continue
			}
			t.Run(route+"_"+specimen.name, func(t *testing.T) {
				var arguments = []string{entry, "--hubsign", "--open", "--no-ansi", "--signing-server=" + server.URL + "/sign?response=json", "--output=" + target}
				if route != "file" {
					arguments = append([]string{route}, arguments...)
				}
				if route == "check" {
					arguments = append(arguments, "--json")
				}
				var output, stderr, err = p15RunCLI(t, binary, workspace, nil, arguments...)
				if (err == nil) != specimen.valid {
					t.Fatalf("public %s validity changed: %v\n%s\n%s", route, err, output, stderr)
				}
				if route == "check" {
					object, okay := p15DecodeOneJSON(t, output).(map[string]any)
					if !okay || object["valid"] != specimen.valid {
						t.Fatal("check did not produce its real validity report", object)
					}
				}
				if !specimen.valid && !strings.Contains(string(output)+string(stderr), specimen.diagnostic) {
					t.Fatal("public route omitted the exact rejection cause", specimen.name, string(output), string(stderr))
				}
				assertNoEffects(t, before)
			})
		}
	}
	t.Run("valid_recovery_and_requested_signer_control", func(t *testing.T) {
		p15ModuleWrite(t, entry, valid)
		svc.ChangeDocument(entry, len(specimens)+1, valid)
		var before = p15DirectoryDigest(t, workspace)
		var diagnostics, _ = svc.Analyze(entry)
		for _, diagnostic := range diagnostics {
			if diagnostic.Severity == analysis.SeverityError {
				t.Fatal("failed operations contaminated valid service recovery", diagnostic)
			}
		}
		var output, err = CompileSourceToPlist(entry, valid)
		if err != nil || !p15ModuleContains(t, output, server.URL+"/authored-runtime-action") {
			t.Fatal("failed operations contaminated valid compile recovery", err)
		}
		assertNoEffects(t, before)
		var _, stderr, signErr = p15RunCLI(t, binary, workspace, nil, "compile", entry, "--hubsign", "--signing-server="+server.URL+"/sign?response=json", "--output="+target)
		if signErr == nil || requests.Load() == 0 || !strings.Contains(string(stderr), "BRG21_ACTUAL_SIGNER_OBSERVER") {
			t.Fatalf("explicit signing positive control never reached the same observer: %v requests=%d %s", signErr, requests.Load(), stderr)
		}
		if after := p15DirectoryDigest(t, workspace); !reflect.DeepEqual(before, after) {
			t.Fatal("known failed signer modified the source or left an artifact")
		}
	})
}
