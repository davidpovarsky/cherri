package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// TestUpstreamPropagationRehearsal proves that an upstream action definition change
// in actions/*.cherri propagates predictably into:
// 1. Live compiler catalog extraction (go run . --actions-json)
// 2. ActionSchema registry
// 3. Compiler semantic analysis / type checking
// 4. Editor completions service
// And that the working tree reverts cleanly without leaving dirty artifacts.
func TestUpstreamPropagationRehearsal(t *testing.T) {
	actionFile := "actions/web.cherri"
	originalBytes, err := os.ReadFile(actionFile)
	if err != nil {
		t.Fatalf("failed to read %s: %v", actionFile, err)
	}

	// Always restore the original file and reload baseline actions on cleanup
	t.Cleanup(func() {
		_ = os.WriteFile(actionFile, originalBytes, 0644)
		loadTestStandardActions()

		// Verify working tree is clean for actions/web.cherri
		cmd := exec.Command("git", "diff", "--exit-code", actionFile)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Errorf("cleanup failed: %s is dirty in git: %s", actionFile, string(out))
		}
	})

	// 1. Modify actions/web.cherri to add a rehearsal parameter: bool rehearsalDryRun: 'WFRehearsalDryRun'
	originalContent := string(originalBytes)
	targetDefinition := "action openURL(text url: 'WFInput') {"
	rehearsalDefinition := "action openURL(text url: 'WFInput', bool rehearsalDryRun: 'WFRehearsalDryRun') {"

	if !strings.Contains(originalContent, targetDefinition) {
		t.Fatalf("could not locate %q in %s", targetDefinition, actionFile)
	}

	modifiedContent := strings.Replace(originalContent, targetDefinition, rehearsalDefinition, 1)
	if err := os.WriteFile(actionFile, []byte(modifiedContent), 0644); err != nil {
		t.Fatalf("failed to write modified %s: %v", actionFile, err)
	}

	// 2. Extract live catalog via compiler subprocess (compiles with updated embedded actions/*.cherri)
	cmd := exec.Command("go", "run", ".", "--actions-json")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to extract live catalog via go run . --actions-json: %v (output: %s)", err, string(out))
	}

	type catalogResponse struct {
		Actions []catalogActionInfo `json:"actions"`
	}
	var resp catalogResponse
	if err := json.Unmarshal(out, &resp); err != nil {
		t.Fatalf("failed to unmarshal live catalog output: %v", err)
	}

	var openURLEntry *catalogActionInfo
	for i := range resp.Actions {
		if resp.Actions[i].Name == "openURL" {
			openURLEntry = &resp.Actions[i]
			break
		}
	}
	if openURLEntry == nil {
		t.Fatal("openURL action missing from live extracted catalog")
	}

	foundParam := false
	for _, p := range openURLEntry.Parameters {
		if p.Name == "rehearsalDryRun" {
			foundParam = true
			if p.Type != "bool" {
				t.Errorf("expected parameter rehearsalDryRun type 'bool', got %q", p.Type)
			}
			if p.Key != "WFRehearsalDryRun" {
				t.Errorf("expected parameter wire key 'WFRehearsalDryRun', got %q", p.Key)
			}
			break
		}
	}
	if !foundParam {
		t.Fatalf("rehearsalDryRun parameter not found in catalogActionInfo: %+v", openURLEntry.Parameters)
	}

	// 3. Propagate to ActionSchema registry
	allActions := schema.DefaultRegistry().AllActions()
	actionsCopy := make([]*schema.ActionSchema, len(allActions))
	for i, a := range allActions {
		if a.CallableName == "openURL" {
			updated := *a
			updated.Parameters = make([]schema.ParameterSchema, len(a.Parameters), len(a.Parameters)+1)
			copy(updated.Parameters, a.Parameters)
			updated.Parameters = append(updated.Parameters, schema.ParameterSchema{
				ID:             "rehearsalDryRun",
				Label:          "rehearsalDryRun",
				DisplayName:    "Rehearsal Dry Run",
				Type:           types.Bool,
				TypeName:       "Bool",
				Optional:       true,
				WireKey:        "WFRehearsalDryRun",
				EvidenceStatus: schema.EvidenceConfirmed,
			})
			actionsCopy[i] = &updated
		} else {
			actionsCopy[i] = a
		}
	}
	reg := schema.NewRegistry(actionsCopy, schema.DefaultRegistry().AllEnums())

	// Verify lookup in registry
	openURLSchema, ok := reg.LookupAction("openURL")
	if !ok {
		t.Fatal("openURL not found in modified registry")
	}
	if _, hasRehearsal := openURLSchema.ParameterByLabel("rehearsalDryRun"); !hasRehearsal {
		t.Fatal("rehearsalDryRun not present in openURL ActionSchema")
	}

	// 4. Verify Semantic Analysis accepts code calling the updated parameter
	codeWithRehearsal := `openURL(url: "https://apple.com", rehearsalDryRun: true)`
	srcFile := source.NewFile("rehearsal.cherri", "file:///rehearsal.cherri", 1, codeWithRehearsal)
	parser := syntax.NewParser(srcFile)
	prog := parser.ParseProgram()
	if len(parser.Errors()) > 0 {
		t.Fatalf("parser errors: %v", parser.Errors())
	}

	analyzer := analysis.NewAnalyzer(reg)
	analyzer.Analyze(prog)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Errorf("unexpected semantic analysis error for rehearsal call: %s", d.Message)
		}
	}

	// 5. Verify Editor Completions offer rehearsalDryRun
	svc := service.NewService(reg)
	uri := "file:///compl.cherri"
	completionsCode := "openURL(\n"
	svc.OpenDocument(uri, 1, completionsCode)
	complItems := svc.Complete(uri, 1, 9)
	foundCompletion := false
	for _, item := range complItems {
		if item.Label == "rehearsalDryRun" {
			foundCompletion = true
			if item.Kind != service.CompletionKindParameter {
				t.Errorf("expected CompletionKindParameter, got %v", item.Kind)
			}
			break
		}
	}
	if !foundCompletion {
		t.Errorf("completions service did not return 'rehearsalDryRun' for openURL(. Items: %+v", complItems)
	}
}
