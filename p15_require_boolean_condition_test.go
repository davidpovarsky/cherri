package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

const p15RequireConditionMessage = "Condition false: quote \" slash \\ שלום"

type p15RequireConditionCase struct {
	name     string
	value    bool
	storage  string
	presence bool
	number   bool
}

func p15RequireConditionCases() []p15RequireConditionCase {
	return []p15RequireConditionCase{
		{name: "literal_true", value: true},
		{name: "literal_false"},
		{name: "immutable_true", value: true, storage: "immutable"},
		{name: "immutable_false", storage: "immutable"},
		{name: "mutable_true", value: true, storage: "mutable"},
		{name: "mutable_false", storage: "mutable"},
	}
}

func p15RequireConditionSource(tc p15RequireConditionCase, legacy bool) string {
	literal := fmt.Sprint(tc.value)
	if tc.number {
		literal = "0"
	}
	argument, prefix := literal, ""
	decl, mutable, variable := "let", "var flag", "flag"
	if legacy {
		decl, mutable, variable = "const", "@flag", "@flag"
	}
	if tc.storage == "immutable" {
		prefix = decl + " flag = " + literal + "\n"
		argument = "flag"
	} else if tc.storage == "mutable" {
		prefix = mutable + " = " + literal + "\n"
		argument = variable
	}
	call := "require(" + argument + ", message:" + fmt.Sprintf("%q", p15RequireConditionMessage) + ")"
	if tc.presence {
		call = "require(" + argument + ", nonEmpty:true, message:" + fmt.Sprintf("%q", p15RequireConditionMessage) + ")"
	}
	if legacy {
		call = fmt.Sprintf("require(%s, \"\", %t, %q)", argument, tc.presence, p15RequireConditionMessage)
	}
	return prefix + decl + " checked = " + call + "\ngetVariable(checked)\n"
}

func p15RequireConditionCompile(t *testing.T, text string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, text, legacy, profile)
	if dir := os.Getenv("P15_REQUIRE_CONDITION_OUT"); dir != "" {
		record := map[string]any{"test": t.Name(), "source": text, "legacy": legacy, "profile": profile, "actions": actions}
		if err != nil {
			record["error"] = err.Error()
		}
		data, marshalErr := json.MarshalIndent(record, "", "  ")
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		name := strings.NewReplacer("/", "__", "=", "-").Replace(t.Name()) + ".json"
		file, openErr := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if openErr != nil {
			t.Fatal(openErr)
		}
		if _, writeErr := file.Write(append(data, '\n')); writeErr != nil {
			file.Close()
			t.Fatal(writeErr)
		}
		if closeErr := file.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
	}
	return actions, err
}

func p15RequireConditionMaps(actions []ShortcutAction) []map[string]any {
	out := make([]map[string]any, len(actions))
	for i, a := range actions {
		out[i] = map[string]any{"WFWorkflowActionIdentifier": a.WFWorkflowActionIdentifier}
		if len(a.WFWorkflowActionParameters) > 0 {
			out[i]["WFWorkflowActionParameters"] = a.WFWorkflowActionParameters
		}
	}
	return out
}

func p15RequireConditionGraph(actions []ShortcutAction, tc p15RequireConditionCase, legacy bool) error {
	actual := p15RequireConditionMaps(actions)
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	if len(actual) == 0 {
		return fmt.Errorf("REQUIRE_CONDITION_GRAPH: empty native graph")
	}
	// Apple's Number field accepts a native numeric scalar or its literal
	// NumberString spelling. Admit only this authored Boolean's exact 0/1.
	want := 0
	if tc.value {
		want = 1
	}
	raw := p15Param(actual[0])["WFNumberActionNumber"]
	switch v := raw.(type) {
	case string:
		if v != fmt.Sprint(want) {
			return fmt.Errorf("REQUIRE_CONDITION_GRAPH: Boolean literal changed")
		}
	case int, int64, uint64, float64:
		if fmt.Sprint(v) != fmt.Sprint(want) {
			return fmt.Errorf("REQUIRE_CONDITION_GRAPH: Boolean numeric value changed")
		}
	default:
		return fmt.Errorf("REQUIRE_CONDITION_GRAPH: unexpected Number field representation %T", raw)
	}
	input := g.value("number", map[string]any{"WFNumberActionNumber": raw})
	if tc.storage == "mutable" {
		g.emit("setvariable", map[string]any{"WFVariableName": "flag", "WFInput": p15FunctionEdge(input)})
		input = g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("flag")})
	}
	guard := input
	if tc.presence {
		guard = g.value("count", map[string]any{"Input": p15FunctionEdge(input), "WFCountType": "Items"})
	}
	group := g.conditional(4, p15FunctionEdge(guard), "WFNumberValue", 0)
	at := len(g.expected)
	if at >= len(actual) {
		return fmt.Errorf("REQUIRE_CONDITION_GRAPH: missing failure alert")
	}
	message := p15Param(actual[at])["WFAlertActionMessage"]
	switch v := message.(type) {
	case string:
		if v != p15RequireConditionMessage {
			return fmt.Errorf("REQUIRE_CONDITION_GRAPH: failure message changed")
		}
	case map[string]any:
		want := p15FunctionText(p15RequireConditionMessage)
		got, _ := json.Marshal(v)
		expected, _ := json.Marshal(want)
		if string(got) != string(expected) {
			return fmt.Errorf("REQUIRE_CONDITION_GRAPH: failure text token changed")
		}
	default:
		return fmt.Errorf("REQUIRE_CONDITION_GRAPH: failure message has unexpected type")
	}
	g.emit("alert", map[string]any{"WFAlertActionMessage": message, "WFAlertActionCancelButtonShown": false})
	g.emit("exit", nil)
	g.boundary(group, 2, false)
	result := g.value("getvariable", map[string]any{"WFVariable": p15FunctionEdge(input)})
	// This authored terminal consumer has no downstream edge and needs no UUID.
	g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(result)})
	if !legacy {
		g.emit("nothing", nil)
	}
	if err := g.compare(); err != nil {
		return fmt.Errorf("REQUIRE_CONDITION_GRAPH: %w", err)
	}
	return nil
}

func p15RequireConditionAnalyzedBool(t *testing.T, text string) {
	t.Helper()
	p := syntax.NewParser(source.NewFile("require-condition.cherri", "require-condition.cherri", 1, text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	found := false
	for _, statement := range program.Statements {
		binding, ok := statement.(*syntax.BindingStmt)
		if !ok || binding.Name != "checked" {
			continue
		}
		got := analyzer.ExprTypes()[binding.Value]
		if got == nil || !types.Bool.Equals(got) {
			t.Fatalf("original Boolean result type changed: %v", got)
		}
		found = true
	}
	if !found {
		t.Fatal("original checked declaration absent")
	}
}

func TestP15RequireBooleanConditionGraphs(t *testing.T) {
	for _, tc := range p15RequireConditionCases() {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					text := p15RequireConditionSource(tc, legacy)
					actions, err := p15RequireConditionCompile(t, text, legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15RequireConditionGraph(actions, tc, legacy); err != nil {
						t.Fatal(err)
					}
					if !legacy {
						p15RequireConditionAnalyzedBool(t, text)
					}
				})
			}
		}
	}
}

func TestP15RequireBooleanConditionPreservesPresenceGuards(t *testing.T) {
	for _, tc := range []p15RequireConditionCase{
		{name: "false_is_present", presence: true},
		{name: "zero_is_present", presence: true, number: true},
	} {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					text := p15RequireConditionSource(tc, legacy)
					actions, err := p15RequireConditionCompile(t, text, legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err = p15RequireConditionGraph(actions, tc, legacy); err != nil {
						t.Fatal(err)
					}
					// A valid prior producer is still the wrong source here: this
					// consumer must read the require passthrough, not bypass it.
					raw, marshalErr := json.Marshal(actions)
					if marshalErr != nil {
						t.Fatal(marshalErr)
					}
					var wrong []ShortcutAction
					if unmarshalErr := json.Unmarshal(raw, &wrong); unmarshalErr != nil {
						t.Fatal(unmarshalErr)
					}
					consumer := len(wrong) - 1
					if !legacy {
						consumer--
					}
					wrong[consumer].WFWorkflowActionParameters["WFVariable"] = map[string]any{
						"WFSerializationType": "WFTextTokenAttachment",
						"Value":               map[string]any{"Type": "ActionOutput", "OutputUUID": wrong[0].WFWorkflowActionParameters["UUID"]},
					}
					if faultErr := p15RequireConditionGraph(wrong, tc, legacy); faultErr == nil {
						t.Fatal("REQUIRE_CONDITION_GRAPH accepted terminal consumer bypass of require result")
					}
				})
			}
		}
	}
}
