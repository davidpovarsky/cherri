package main

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

const p15StateMarker = "P15_STATE_EXERCISE="

type p15StateExercise struct {
	RaceInstrumented  bool     `json:"race_instrumented"`
	SequentialRounds  int      `json:"sequential_rounds"`
	ConcurrentWorkers int      `json:"concurrent_workers"`
	ConcurrentRounds  int      `json:"concurrent_rounds"`
	CompletedCases    int      `json:"completed_cases"`
	EmitterFaults     []string `json:"emitter_faults,omitempty"`
}

// Decode real output and check the independently authored payload and action.
// Successful status or nonempty bytes are insufficient for state isolation.
func p15StateAssertText(data []byte, payload string, canonical bool) error {
	var workflow map[string]any
	if _, err := plist.Unmarshal(data, &workflow); err != nil {
		return err
	}
	var actions, ok = workflow["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 1 {
		return fmt.Errorf("expected one native Text action, got %#v", workflow["WFWorkflowActions"])
	}
	var action, valid = actions[0].(map[string]any)
	var params, _ = action["WFWorkflowActionParameters"].(map[string]any)
	if !valid || action["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" || params["WFTextActionText"] != payload {
		return fmt.Errorf("native request payload leaked or changed: want %q, got %#v", payload, action)
	}
	if canonical && workflow["WFWorkflowClientVersion"] != versions["27"] {
		return fmt.Errorf("canonical platform metadata leaked: got %#v, want %q", workflow["WFWorkflowClientVersion"], versions["27"])
	}
	return nil
}

func p15StateCompile(payload, mode string) error {
	var src = fmt.Sprintf("text(%q)\n", payload)
	if mode == "canonical" {
		var data, err = CompileSourceToPlist("P15_State_Request.cherri", src)
		if err != nil {
			return err
		}
		return p15StateAssertText(data, payload, true)
	}
	if mode == "legacy" {
		src = "#define version 18\n#define color red\n" + src
	}
	var response = compileForMobileMode(src, "P15_State_Mobile", mode, false)
	if !response.OK || response.Error != "" || response.Mode != mode || response.SignedBase64 != "" {
		return fmt.Errorf("mobile %s response failed: %#v", mode, response)
	}
	var data, err = base64.StdEncoding.DecodeString(response.PlistBase64)
	if err != nil {
		return err
	}
	return p15StateAssertText(data, payload, mode == "v2")
}

func p15StateCatalog() (string, error) {
	var catalog = currentMobileActionCatalog()
	var textCount int
	for _, item := range catalog {
		if item.Name == "text" {
			textCount++
			if item.ShortcutIdentifier != "is.workflow.actions.gettext" {
				return "", fmt.Errorf("catalog Text identity changed")
			}
		}
	}
	if textCount != 1 {
		return "", fmt.Errorf("catalog needs exactly one canonical Text action, got %d", textCount)
	}
	var data, err = json.Marshal(catalog)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", sha256.Sum256(data)), nil
}

func p15StateAnalyze(svc *service.Service, identity string, revision int) error {
	var uri = "file:///P15_State_" + identity + ".cherri"
	var binding = "state_" + identity
	var missing = "missing_" + identity
	var valid = fmt.Sprintf("let %s = %q\ntext(%s)\n", binding, identity, binding)
	svc.ChangeDocument(uri, revision*3+1, valid)
	var diagnostics, fingerprint = svc.Analyze(uri)
	if fingerprint != schema.DefaultRegistry().Fingerprint() {
		return fmt.Errorf("analysis registry fingerprint changed")
	}
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			return fmt.Errorf("valid analysis failed: %v", diagnostics)
		}
	}
	var found bool
	for _, item := range svc.Complete(uri, 3, 1) {
		if item.Label == binding {
			found = true
		}
		if strings.HasPrefix(item.Label, "state_") && item.Label != binding {
			return fmt.Errorf("completion leaked another document: %q", item.Label)
		}
	}
	if !found {
		return fmt.Errorf("completion lost the current document binding %q", binding)
	}
	svc.ChangeDocument(uri, revision*3+2, "text("+missing+")\n")
	diagnostics, _ = svc.Analyze(uri)
	var detected bool
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError && strings.Contains(diagnostic.Message, missing) {
			detected = true
		}
	}
	if !detected {
		return fmt.Errorf("analysis swallowed the injected unresolved reference")
	}
	svc.ChangeDocument(uri, revision*3+3, valid)
	diagnostics, _ = svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			return fmt.Errorf("analysis retained a previous request error")
		}
	}
	if svc.DocumentVersion(uri) != revision*3+3 {
		return fmt.Errorf("analysis document version changed")
	}
	return nil
}

func p15StateMobileErrors() error {
	for _, request := range []struct{ source, mode string }{{"text(\"unfinished\n", "v2"}, {"text(\"synthetic\")", "invalid"}} {
		var response = compileForMobileMode(request.source, "P15_State_Error", request.mode, false)
		if response.OK || response.Error == "" || response.PlistBase64 != "" || response.SignedBase64 != "" {
			return fmt.Errorf("mobile failure returned success or partial artifact: %#v", response)
		}
	}
	return p15StateCompile("P15_ERROR_THEN_SUCCESS", "v2")
}

func p15StateMixedExercise() (p15StateExercise, error) {
	var result = p15StateExercise{RaceInstrumented: p15StateRaceInstrumented, SequentialRounds: 3, ConcurrentWorkers: 8, ConcurrentRounds: 8}
	var reference, err = CompileSourceToPlist("P15_State_Reference.cherri", "text(\"P15_DECOMPILE_REFERENCE\")\n")
	if err != nil {
		return result, err
	}
	if err = p15StateAssertText(reference, "P15_DECOMPILE_REFERENCE", true); err != nil {
		return result, err
	}
	var decompiled = decompileForMobile(reference, "P15_State_Reference")
	if !decompiled.OK || decompiled.Source == "" || !strings.Contains(decompiled.Source, "P15_DECOMPILE_REFERENCE") {
		return result, fmt.Errorf("reference decompilation failed: %#v", decompiled)
	}
	var catalog, catalogErr = p15StateCatalog()
	if catalogErr != nil {
		return result, catalogErr
	}
	var svc = service.NewService(schema.DefaultRegistry())
	var decompile = func() error {
		var response = decompileForMobile(reference, "P15_State_Reference")
		if !response.OK || response.Source != decompiled.Source {
			return fmt.Errorf("decompile output changed across other requests: %#v", response)
		}
		return nil
	}
	var checkCatalog = func() error {
		var digest, err = p15StateCatalog()
		if err != nil {
			return err
		}
		if digest != catalog {
			return fmt.Errorf("catalog changed after compile/decompile/analysis")
		}
		return nil
	}
	for i := 0; i < result.SequentialRounds; i++ {
		for _, mode := range []string{"canonical", "legacy", "v2"} {
			if err = p15StateCompile("P15_SEQUENTIAL_"+mode, mode); err != nil {
				return result, err
			}
		}
		if err = decompile(); err != nil {
			return result, err
		}
		if err = checkCatalog(); err != nil {
			return result, err
		}
		if err = p15StateAnalyze(svc, "sequential", i); err != nil {
			return result, err
		}
		if err = p15StateMobileErrors(); err != nil {
			return result, err
		}
	}
	var start = make(chan struct{})
	var errorsFound = make(chan error, result.ConcurrentWorkers)
	var counts = make([]int, result.ConcurrentWorkers)
	var workers sync.WaitGroup
	for worker := 0; worker < result.ConcurrentWorkers; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			for round := 0; round < result.ConcurrentRounds; round++ {
				var err error
				var payload = fmt.Sprintf("P15_WORKER_%d_ROUND_%d", worker, round)
				switch worker {
				case 0:
					err = p15StateCompile(payload, "canonical")
				case 1:
					err = p15StateCompile(payload, "v2")
				case 2:
					err = p15StateCompile(payload, "legacy")
				case 3:
					err = decompile()
				case 4:
					err = checkCatalog()
				case 5, 6:
					err = p15StateAnalyze(svc, fmt.Sprintf("worker_%d", worker), round)
				case 7:
					err = p15StateMobileErrors()
				}
				if err != nil {
					errorsFound <- fmt.Errorf("worker %d round %d: %w", worker, round, err)
					return
				}
				counts[worker]++
			}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(errorsFound)
	for err = range errorsFound {
		if err != nil {
			return result, err
		}
	}
	for _, count := range counts {
		result.CompletedCases += count
	}
	if result.CompletedCases != result.ConcurrentWorkers*result.ConcurrentRounds {
		return result, fmt.Errorf("incomplete concurrent exercise")
	}
	if err = p15StateCompile("P15_FINAL_CANONICAL", "canonical"); err != nil {
		return result, err
	}
	if err = p15StateCompile("P15_FINAL_MOBILE", "v2"); err != nil {
		return result, err
	}
	if err = decompile(); err != nil {
		return result, err
	}
	if err = checkCatalog(); err != nil {
		return result, err
	}
	return result, nil
}

// A process boundary records a genuine legacy fatal error or deadlock as a
// failed scenario without losing the parent test's structured evidence.
func TestBackendRecovery06MixedEntryPoints(t *testing.T) {
	if os.Getenv("CHERRI_P15_STATE_CHILD") == "1" {
		var result, err = p15StateMixedExercise()
		if err != nil {
			t.Fatal(err)
		}
		var data, marshalErr = json.Marshal(result)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		fmt.Println(p15StateMarker + string(data))
		return
	}
	var executable, err = os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var cmd = exec.CommandContext(ctx, executable, "-test.run=^TestBackendRecovery06MixedEntryPoints$", "-test.v")
	cmd.Env = append(os.Environ(), "CHERRI_P15_STATE_CHILD=1")
	var output, runErr = cmd.CombinedOutput()
	if runErr != nil {
		t.Fatalf("actual mixed entry-point process failed: %v (context %v)\n%s", runErr, ctx.Err(), output)
	}
	var markers int
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, p15StateMarker) {
			continue
		}
		var result p15StateExercise
		if err = json.Unmarshal([]byte(strings.TrimPrefix(line, p15StateMarker)), &result); err != nil {
			t.Fatal(err)
		}
		if result.RaceInstrumented != p15StateRaceInstrumented || result.SequentialRounds != 3 || result.ConcurrentWorkers != 8 || result.ConcurrentRounds != 8 || result.CompletedCases != 64 {
			t.Fatalf("incomplete mixed request evidence: %#v", result)
		}
		markers++
		t.Log(line)
	}
	if markers != 1 {
		t.Fatalf("expected exactly one completed child exercise; got %d\n%s", markers, output)
	}
}

func TestBackendRecovery06EmitterErrorAndCancellation(t *testing.T) {
	var previousHook = backendEmitHook
	defer func() { backendEmitHook = previousHook }()
	var originalCatalog, catalogErr = p15StateCatalog()
	if catalogErr != nil {
		t.Fatal(catalogErr)
	}
	for _, fault := range []struct {
		name string
		err  error
	}{{"emitter_error", errors.New("P15_EMITTER_SENTINEL")}, {"cancellation", context.Canceled}} {
		t.Run(fault.name, func(t *testing.T) {
			var calls int
			backendEmitHook = func(call backend.ResolvedCall) error {
				calls++
				if calls == 2 {
					return fault.err
				}
				return nil
			}
			var data, err = CompileSourceToPlist("P15_Failed_Emission.cherri", "text(\"P15_FIRST\")\ntext(\"P15_SECOND\")\ntext(\"P15_NEVER\")\n")
			backendEmitHook = previousHook
			if !errors.Is(err, fault.err) || len(data) != 0 || calls != 2 {
				t.Fatalf("emitter fault was swallowed, returned partial bytes, or continued: calls=%d err=%v bytes=%d", calls, err, len(data))
			}
			if !reflect.DeepEqual(currentAction, actionReference{}) || len(tokens) != 0 || contents != "" || len(variables) != 0 {
				t.Fatal("failed emission left process-global compiler state")
			}
			if err = p15StateCompile("P15_AFTER_"+fault.name, "canonical"); err != nil {
				t.Fatal(err)
			}
			if err = p15StateCompile("P15_MOBILE_AFTER_"+fault.name, "v2"); err != nil {
				t.Fatal(err)
			}
			var catalog, catalogError = p15StateCatalog()
			if catalogError != nil || catalog != originalCatalog {
				t.Fatalf("fault contaminated catalog: %v", catalogError)
			}
		})
	}
	var result = p15StateExercise{RaceInstrumented: p15StateRaceInstrumented, EmitterFaults: []string{"emitter_error", "cancellation"}}
	var data, err = json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(p15StateMarker + string(data))
}
