package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	"howett.net/plist"
)

// The fixture is authored directly, so no canonical action initialization or
// compiler transaction warms the caches before the competing entry points.
func p15StateImportExercise() (map[string]any, error) {
	var sourceFixture = map[string]any{
		"WFWorkflowClientVersion": versions["27"],
		"WFWorkflowClientRelease": "27.0",
		"WFWorkflowName":          "P15_Import_Reference",
		"WFWorkflowActions": []any{map[string]any{
			"WFWorkflowActionIdentifier": "is.workflow.actions.gettext",
			"WFWorkflowActionParameters": map[string]any{"WFTextActionText": "P15_IMPORT_REFERENCE"},
		}},
	}
	var reference, err = plist.Marshal(sourceFixture, plist.XMLFormat)
	if err != nil {
		return nil, err
	}
	var start = make(chan struct{})
	var failures = make(chan error, 4)
	var counts = make([]int, 4)
	var workers sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		workers.Add(1)
		go func(worker int) {
			defer workers.Done()
			<-start
			for round := 0; round < 8; round++ {
				var payload = fmt.Sprintf("P15_IMPORT_WORKER_%d_ROUND_%d", worker, round)
				var failure error
				switch worker {
				case 0:
					failure = p15StateCompile(payload, "canonical")
				case 1:
					var workflow, compileErr = compileLegacySourceToRawMapInMemory(fmt.Sprintf("text(%q)\n", payload))
					if compileErr != nil {
						failure = compileErr
						break
					}
					var data, marshalErr = plist.Marshal(workflow, plist.XMLFormat)
					if marshalErr != nil {
						failure = marshalErr
						break
					}
					failure = p15StateAssertText(data, payload, false)
				case 2:
					var response = decompileForMobileWithOptions(reference, "P15_Import_Reference", "v2", "hybrid")
					if !response.OK || response.Error != "" || response.Source == "" {
						failure = fmt.Errorf("hybrid import failed: %#v", response)
						break
					}
					var data, compileErr = CompileSourceToPlist("P15_Imported_Reference.cherri", response.Source)
					if compileErr != nil {
						failure = compileErr
						break
					}
					failure = p15StateAssertText(data, "P15_IMPORT_REFERENCE", true)
				case 3:
					var definitions = EnsureCanonicalStandardActions()
					var definition = definitions["text"]
					if definition == nil || catalogShortcutIdentifier("text", definition) != "is.workflow.actions.gettext" {
						failure = fmt.Errorf("canonical action initialization lost Text identity")
					}
				}
				if failure != nil {
					failures <- fmt.Errorf("import worker %d round %d: %w", worker, round, failure)
					return
				}
				counts[worker]++
			}
		}(worker)
	}
	close(start)
	workers.Wait()
	close(failures)
	for failure := range failures {
		if failure != nil {
			return nil, failure
		}
	}
	var completed int
	for _, count := range counts {
		completed += count
	}
	if completed != 32 {
		return nil, fmt.Errorf("incomplete import isolation scenario: %d", completed)
	}
	if err := p15StateCompile("P15_AFTER_IMPORTS", "canonical"); err != nil {
		return nil, err
	}
	return map[string]any{"race_instrumented": p15StateRaceInstrumented, "concurrent_workers": 4, "concurrent_rounds": 8, "completed_cases": completed, "entry_points": []string{"canonical", "legacy-in-memory", "hybrid-v2", "canonical-catalog-cold"}}, nil
}

func TestBackendRecovery06ImportEntryPoints(t *testing.T) {
	if os.Getenv("CHERRI_P15_IMPORT_STATE_CHILD") == "1" {
		var observation, err = p15StateImportExercise()
		if err != nil {
			t.Fatal(err)
		}
		var data, encodeErr = json.Marshal(observation)
		if encodeErr != nil {
			t.Fatal(encodeErr)
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
	var command = exec.CommandContext(ctx, executable, "-test.run=^TestBackendRecovery06ImportEntryPoints$", "-test.v")
	command.Env = append(os.Environ(), "CHERRI_P15_IMPORT_STATE_CHILD=1")
	var output, runErr = command.CombinedOutput()
	if runErr != nil {
		t.Fatalf("actual import state process failed: %v (context %v)\n%s", runErr, ctx.Err(), output)
	}
	var witnesses int
	for _, line := range strings.Split(string(output), "\n") {
		if !strings.HasPrefix(line, p15StateMarker) {
			continue
		}
		var observation struct {
			RaceInstrumented  bool     `json:"race_instrumented"`
			ConcurrentWorkers int      `json:"concurrent_workers"`
			ConcurrentRounds  int      `json:"concurrent_rounds"`
			CompletedCases    int      `json:"completed_cases"`
			EntryPoints       []string `json:"entry_points"`
		}
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, p15StateMarker)), &observation); err != nil {
			t.Fatal(err)
		}
		if observation.RaceInstrumented != p15StateRaceInstrumented || observation.ConcurrentWorkers != 4 || observation.ConcurrentRounds != 8 || observation.CompletedCases != 32 || strings.Join(observation.EntryPoints, ",") != "canonical,legacy-in-memory,hybrid-v2,canonical-catalog-cold" {
			t.Fatalf("incomplete import evidence: %#v", observation)
		}
		witnesses++
		t.Log(line)
	}
	if witnesses != 1 {
		t.Fatalf("expected one complete import witness, got %d\n%s", witnesses, output)
	}
}
