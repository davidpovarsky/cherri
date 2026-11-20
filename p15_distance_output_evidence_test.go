package main

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestP15DistanceIndependentNativeOutput(t *testing.T) {
	raw, err := os.ReadFile("docs/apple-runtime/macos-27/builtin-actions.json")
	if err != nil {
		t.Fatal(err)
	}
	var metadata map[string]struct {
		ActionClass string
		Output      struct {
			Multiple   bool
			OutputName string
			Types      []string
		}
	}
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	entry, exists := metadata["is.workflow.actions.getdistance"]
	if !exists || entry.ActionClass != "WFGetDistanceAction" || entry.Output.Multiple ||
		entry.Output.OutputName != "Distance" || !slices.Equal(entry.Output.Types, []string{"NSNumber"}) {
		t.Fatal("independent single-number distance contract changed")
	}
	registry := schema.DefaultRegistry()
	action, exists := registry.LookupAction("getDistance")
	if !exists || action.AppleIdentifier != "is.workflow.actions.getdistance" ||
		!action.OutputType.Equals(types.Number) || action.OutputTypeName != "Number" ||
		action.RawOutputType != "number" || len(registry.AllActions()) != 505 {
		t.Fatalf("canonical distance contract lost Number: %+v", action)
	}
}

func p15DistanceWire(actions []ShortcutAction, legacy bool) error {
	wantCount := 4
	if !legacy {
		wantCount++
		if err := debugMainVoidBoundaryError(actions); err != nil {
			return err
		}
	}
	if len(actions) != wantCount {
		return fmt.Errorf("unexpected distance graph length %d", len(actions))
	}
	producer, condition := actions[0], actions[1]
	p, c := producer.WFWorkflowActionParameters, condition.WFWorkflowActionParameters
	uuid, _ := p["UUID"].(string)
	if producer.WFWorkflowActionIdentifier != "is.workflow.actions.getdistance" ||
		p["WFAvoidHighways"] != false || p["WFAvoidTolls"] != false || uuid == "" {
		return fmt.Errorf("distance native identity/options changed: %s %#v", producer.WFWorkflowActionIdentifier, p)
	}
	if condition.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		p15PromptConditionUUID(c["WFInput"]) != uuid ||
		fmt.Sprint(c["WFNumberValue"]) != "7" || fmt.Sprint(c["WFCondition"]) != "2" ||
		c["WFConditionalActionString"] != nil || c["WFDate"] != nil {
		return fmt.Errorf("distance numeric consumer changed")
	}
	if actions[2].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		actions[3].WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		c["GroupingIdentifier"] != actions[3].WFWorkflowActionParameters["GroupingIdentifier"] {
		return fmt.Errorf("distance region changed or converter inserted")
	}
	return nil
}

func TestP15DistanceNumberProduction(t *testing.T) {
	source := "let distance: Number = getDistance(avoidHighways: false, avoidTolls: false)\nif distance > 7 { text(\"high\") }"
	program, analyzer := p15T08Analyzer(t, source)
	actual := analyzer.ExprTypes()[p15BindingValue(t, program, "distance")]
	if actual == nil || !actual.Equals(types.Number) {
		t.Fatalf("distance result got %v", actual)
	}
	for _, tc := range []struct {
		name, source string
		legacy       bool
	}{
		{"v2", source, false},
		{"legacy", "#include 'actions/location'\nconst distance = getDistance(nil, nil, nil, false, false)\nif distance > 7 { text(\"high\") }", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := compileDebugFixture(t, tc.source, tc.legacy)
			if err := p15DistanceWire(actions, tc.legacy); err != nil {
				t.Fatal(err)
			}
			p, c := actions[0].WFWorkflowActionParameters, actions[1].WFWorkflowActionParameters
			for _, fault := range []string{"producer", "mode", "rhs"} {
				uuid, highway, rhs := p["UUID"], p["WFAvoidHighways"], c["WFNumberValue"]
				switch fault {
				case "producer":
					p["UUID"] = "wrong-producer"
				case "mode":
					p["WFAvoidHighways"] = true
				case "rhs":
					c["WFNumberValue"] = 8.0
				}
				if p15DistanceWire(actions, tc.legacy) == nil {
					t.Fatalf("%s corruption passed", fault)
				}
				p["UUID"], p["WFAvoidHighways"], c["WFNumberValue"] = uuid, highway, rhs
			}
		})
	}
	svc := service.NewService(nil)
	uri := "file:///p15-distance-number.cherri"
	svc.OpenDocument(uri, 1, source)
	diagnostics, _ := svc.Analyze(uri)
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			t.Fatal(diagnostic)
		}
	}
	hover := svc.Hover(uri, 1, strings.Index(source, "getDistance")+1)
	if hover == nil || !strings.Contains(hover.Contents, "**Returns:** "+string(rune(96))+"Number"+string(rune(96))) {
		t.Fatalf("distance service return lost Number: %+v", hover)
	}
	// The correction is tied to this independently described producer; opaque
	// native/raw outputs still cannot satisfy an authored Number annotation.
	bad := "let value: Number = run(\"P15_OPAQUE_SYNTHETIC\")"
	raw, err := CompileSourceToPlist("p15-distance-opaque.cherri", bad)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
		t.Fatalf("unrelated Unknown result became Number: bytes=%d err=%v", len(raw), err)
	}
}
