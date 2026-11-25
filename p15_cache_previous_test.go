package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type p15CachePreviousCase struct {
	name, prefix, consumer string
	nested, local          bool
}

func p15CachePreviousCases() []p15CachePreviousCase {
	return []p15CachePreviousCase{
		{name: "date_input", prefix: "date(\"2000-01-01T00:00:00Z\")\n", consumer: "date"},
		{name: "entry_input", consumer: "date"},
		{name: "debug_previous", prefix: "text(\"P15_CACHE_AUTHOR_INPUT\")\n", consumer: "log"},
		{name: "body_owned_previous", prefix: "text(\"P15_CACHE_AUTHOR_INPUT\")\n", consumer: "log", local: true},
		{name: "nested_date_input", prefix: "date(\"2000-01-01T00:00:00Z\")\n", consumer: "date", nested: true},
	}
}
func p15CachePreviousSource(tc p15CachePreviousCase, legacy bool, ttl int, cached bool) string {
	declaration, header, tail := "let", "shortcut \"cache previous\" { version:\"27\" }\n", "\nreturn result\n"
	if legacy {
		declaration, header, tail = "const", "#define version 27\n", "\ngetVariable(result)\n"
		if tc.consumer == "date" {
			header += "#include 'actions/calendar'\n"
		}
	}
	body, result := "", "getTimeBetweenDates()"
	if tc.consumer == "log" {
		if tc.local {
			body += "text(\"P15_CACHE_BODY_INPUT\")\n"
		}
		body += "debugLog()\n"
		result = "7"
	}
	if !cached {
		return header + tc.prefix + body + declaration + " result = " + result + tail
	}
	if tc.nested {
		inner := fmt.Sprintf("cache(key:\"P15_CACHE_INNER_PREVIOUS\",ttl:%d)", ttl)
		if legacy {
			inner = fmt.Sprintf("cache(\"P15_CACHE_INNER_PREVIOUS\",%d)", ttl)
		}
		body += declaration + " inner = " + inner + " {\nyield getTimeBetweenDates()\n}\n"
		result = "inner"
	}
	call := fmt.Sprintf("cache(key:\"P15_CACHE_OUTER_PREVIOUS\",ttl:%d)", ttl)
	if legacy {
		call = fmt.Sprintf("cache(\"P15_CACHE_OUTER_PREVIOUS\",%d)", ttl)
	}
	return header + tc.prefix + declaration + " result = " + call + " {\n" + body + "yield " + result + "\n}\n" + tail
}
func p15CachePreviousCompile(t *testing.T, source, role string, legacy bool, profile BuildProfile) ([]ShortcutAction, error) {
	t.Helper()
	actions, err := compileBuildProfileFixture(t, source, legacy, profile)
	if dir := os.Getenv("P15_CACHE_PREVIOUS_OUT"); dir != "" {
		r := map[string]any{"test": t.Name(), "role": role, "source": source, "legacy": legacy, "profile": profile, "actions": actions}
		if err != nil {
			r["error"] = err.Error()
		}
		b, e := json.MarshalIndent(r, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		name := strings.NewReplacer("/", "_", "\\", "_").Replace(t.Name()) + "_" + role + ".json"
		if e = os.WriteFile(filepath.Join(dir, name), append(b, '\n'), 0600); e != nil {
			t.Fatal(e)
		}
	}
	return actions, err
}

type p15PreviousGraph struct {
	actions []ShortcutAction
	ids     map[string]int
}

func p15PreviousGraphFor(actions []ShortcutAction) (*p15PreviousGraph, error) {
	g := &p15PreviousGraph{actions: actions, ids: map[string]int{}}
	for i, a := range actions {
		id, _ := a.WFWorkflowActionParameters["UUID"].(string)
		if id == "" {
			continue
		}
		if _, found := g.ids[id]; found {
			return nil, fmt.Errorf("duplicate producer %s", id)
		}
		g.ids[id] = i
	}
	return g, nil
}
func p15PreviousObject(v any) (map[string]any, error) {
	p, okay := v.(map[string]any)
	if !okay {
		return nil, fmt.Errorf("not a typed reference: %T", v)
	}
	if p["WFSerializationType"] == "WFTextTokenAttachment" {
		value, okay := p["Value"].(map[string]any)
		if !okay {
			return nil, fmt.Errorf("invalid typed attachment")
		}
		return value, nil
	}
	return nil, fmt.Errorf("reference lost WFTextTokenAttachment")
}
func (g *p15PreviousGraph) reference(v any, consumer int, seen map[int]bool) (string, error) {
	p, err := p15PreviousObject(v)
	if err != nil {
		return "", err
	}
	if _, decorated := p["Aggrandizements"]; decorated {
		return "", fmt.Errorf("input was transformed")
	}
	switch p["Type"] {
	case "ActionOutput":
		id, _ := p["OutputUUID"].(string)
		index, found := g.ids[id]
		if !found || index >= consumer {
			return "", fmt.Errorf("missing or forward producer %s", id)
		}
		return g.origin(index, seen)
	case "Variable":
		name, _ := p["VariableName"].(string)
		index := -1
		for i := 0; i < consumer; i++ {
			a := g.actions[i]
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && a.WFWorkflowActionParameters["WFVariableName"] == name {
				if index >= 0 {
					return "", fmt.Errorf("ambiguous input slot %s", name)
				}
				index = i
			}
		}
		if index < 0 {
			return "", fmt.Errorf("unbound input slot %s", name)
		}
		return g.origin(index, seen)
	case "ExtensionInput":
		return "workflow-entry-input", nil
	default:
		return "", fmt.Errorf("unsupported previous source %v", p)
	}
}
func (g *p15PreviousGraph) origin(index int, seen map[int]bool) (string, error) {
	if index < 0 {
		return "workflow-entry-input", nil
	}
	if seen[index] {
		return "", fmt.Errorf("cyclic input ancestry")
	}
	seen[index] = true
	defer delete(seen, index)
	a := g.actions[index]
	p := a.WFWorkflowActionParameters
	switch a.WFWorkflowActionIdentifier {
	case "is.workflow.actions.getvariable":
		return g.reference(p["WFVariable"], index, seen)
	case "is.workflow.actions.setvariable":
		if input, present := p["WFInput"]; present {
			return g.reference(input, index, seen)
		}
		return g.origin(index-1, seen)
	case "is.workflow.actions.nothing":
		return "no-content", nil
	case "is.workflow.actions.gettext", "is.workflow.actions.date", "is.workflow.actions.number":
		clean := map[string]any{}
		for k, v := range p {
			if k != "UUID" && k != "CustomOutputName" {
				clean[k] = v
			}
		}
		b, err := json.Marshal(clean)
		return a.WFWorkflowActionIdentifier + ":" + string(b), err
	default:
		return "", fmt.Errorf("private or unproven predecessor %s at %d", a.WFWorkflowActionIdentifier, index)
	}
}
func p15PreviousTargets(actions []ShortcutAction, consumer string) []int {
	var out []int
	for i, a := range actions {
		p := a.WFWorkflowActionParameters
		if consumer == "date" && a.WFWorkflowActionIdentifier == "is.workflow.actions.gettimebetweendates" {
			if _, present := p["WFInput"]; !present {
				out = append(out, i)
			}
		}
		if consumer == "log" && a.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" && p["WFVariableName"] == debugLogVariableName {
			out = append(out, i)
		}
	}
	return out
}
func p15PreviousEquivalence(baseline, actual []ShortcutAction, tc p15CachePreviousCase, profile BuildProfile) error {
	bg, err := p15PreviousGraphFor(baseline)
	if err != nil {
		return err
	}
	ag, err := p15PreviousGraphFor(actual)
	if err != nil {
		return err
	}
	b, a := p15PreviousTargets(baseline, tc.consumer), p15PreviousTargets(actual, tc.consumer)
	want := 1
	if tc.consumer == "log" && profile == BuildProfileRelease {
		want = 0
	}
	if len(b) != want || len(a) != want {
		return fmt.Errorf("authored consumer inventory %d/%d, want %d", len(b), len(a), want)
	}
	if want == 0 {
		return nil
	}
	input := func(g *p15PreviousGraph, i int) (string, error) {
		if tc.consumer == "log" {
			return g.reference(g.actions[i].WFWorkflowActionParameters["WFInput"], i, map[int]bool{})
		}
		return g.origin(i-1, map[int]bool{})
	}
	expected, err := input(bg, b[0])
	if err != nil {
		return fmt.Errorf("baseline input: %w", err)
	}
	observed, err := input(ag, a[0])
	if err != nil {
		return fmt.Errorf("cache input: %w", err)
	}
	if expected != observed {
		return fmt.Errorf("authored Previous changed: %s -> %s", expected, observed)
	}
	clean := func(p map[string]any) map[string]any {
		c := map[string]any{}
		for k, v := range p {
			if k != "UUID" && k != "CustomOutputName" && !(tc.consumer == "log" && k == "WFInput") {
				c[k] = v
			}
		}
		return c
	}
	if !reflect.DeepEqual(clean(baseline[b[0]].WFWorkflowActionParameters), clean(actual[a[0]].WFWorkflowActionParameters)) {
		return fmt.Errorf("authored consumer parameters changed")
	}
	return nil
}
func p15CachePreviousSnapshotShape(actions []ShortcutAction, tc p15CachePreviousCase, ttl int) error {
	var captures []int
	for i, a := range actions {
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" {
			name, _ := a.WFWorkflowActionParameters["WFVariableName"].(string)
			if strings.HasSuffix(name, "_input") && (strings.HasPrefix(name, "__cherri_cache_") || strings.HasPrefix(name, "__cherri_block_")) {
				captures = append(captures, i)
			}
		}
	}
	want := 0
	if ttl > 0 {
		want = 1
		if tc.nested {
			want = 2
		}
	}
	if len(captures) != want {
		return fmt.Errorf("input snapshot inventory %d, want %d", len(captures), want)
	}
	slots := map[string]bool{}
	for ordinal, index := range captures {
		p := actions[index].WFWorkflowActionParameters
		name := p["WFVariableName"].(string)
		if slots[name] {
			return fmt.Errorf("input capture slot collision")
		}
		slots[name] = true
		if _, present := p["WFInput"]; present {
			return fmt.Errorf("input capture manufactured an explicit value")
		}
		if index+1 >= len(actions) || actions[index+1].WFWorkflowActionIdentifier != "is.workflow.actions.number" || fmt.Sprint(actions[index+1].WFWorkflowActionParameters["WFNumberActionNumber"]) != "0" {
			return fmt.Errorf("input snapshot was not before cache helpers")
		}
		restored := -1
		for j, a := range actions {
			if a.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
				continue
			}
			ref, err := p15PreviousObject(a.WFWorkflowActionParameters["WFVariable"])
			if err != nil {
				continue
			}
			if ref["Type"] == "Variable" && ref["VariableName"] == name {
				if restored >= 0 {
					return fmt.Errorf("duplicate input restore")
				}
				restored = j
			}
		}
		if restored <= index || restored+1 >= len(actions) {
			return fmt.Errorf("missing delayed input restore")
		}
		before := actions[restored-1]
		bp := before.WFWorkflowActionParameters
		if before.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" || fmt.Sprint(bp["WFControlFlowMode"]) != "0" || fmt.Sprint(bp["WFCondition"]) != "4" || fmt.Sprint(bp["WFNumberValue"]) != "0" {
			return fmt.Errorf("input restore was not inside exact miss branch")
		}
		expected := "is.workflow.actions.gettimebetweendates"
		if tc.consumer == "log" {
			expected = "is.workflow.actions.appendvariable"
			if tc.local {
				expected = "is.workflow.actions.gettext"
			}
		}
		if tc.nested && ordinal == 0 {
			expected = "is.workflow.actions.setvariable"
		}
		// Release removes debugLog; its source yield is the first authored runtime value.
		if tc.consumer == "log" && !tc.local && actions[restored+1].WFWorkflowActionIdentifier == "is.workflow.actions.number" {
			if fmt.Sprint(actions[restored+1].WFWorkflowActionParameters["WFNumberActionNumber"]) != "7" {
				return fmt.Errorf("helper replaced released body")
			}
		} else if actions[restored+1].WFWorkflowActionIdentifier != expected {
			return fmt.Errorf("helper intervened between input restore and authored body: %s", actions[restored+1].WFWorkflowActionIdentifier)
		}
	}
	return nil
}
func TestP15CachePreviousMatchesAuthoredBaseline(t *testing.T) {
	for _, tc := range p15CachePreviousCases() {
		for _, ttl := range []int{0, 300} {
			for _, legacy := range []bool{false, true} {
				for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
					t.Run(fmt.Sprintf("%s/ttl=%d/legacy=%t/%s", tc.name, ttl, legacy, profile), func(t *testing.T) {
						baseline, err := p15CachePreviousCompile(t, p15CachePreviousSource(tc, legacy, ttl, false), "baseline", legacy, profile)
						if err != nil {
							t.Fatalf("authored baseline failed: %v", err)
						}
						if err = p15PreviousEquivalence(baseline, baseline, tc, profile); err != nil {
							t.Fatal(err)
						}
						actual, err := p15CachePreviousCompile(t, p15CachePreviousSource(tc, legacy, ttl, true), "cache", legacy, profile)
						if err != nil {
							t.Fatalf("cache source failed: %v", err)
						}
						if err = p15PreviousEquivalence(baseline, actual, tc, profile); err != nil {
							t.Fatal(err)
						}
						if err = p15CachePreviousSnapshotShape(actual, tc, ttl); err != nil {
							t.Fatal(err)
						}
					})
				}
			}
		}
	}
}
func TestP15CachePreviousBaselineOracleFaults(t *testing.T) {
	for _, tc := range []p15CachePreviousCase{p15CachePreviousCases()[0], p15CachePreviousCases()[2]} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", tc.name, legacy), func(t *testing.T) {
				baseline, err := p15CachePreviousCompile(t, p15CachePreviousSource(tc, legacy, 0, false), "baseline-control", legacy, BuildProfileDebug)
				if err != nil {
					t.Fatal(err)
				}
				for _, fault := range []string{"changed_source", "removed_consumer", "dangling_reference", "wrong_consumer_parameters"} {
					t.Run(fault, func(t *testing.T) {
						raw, _ := json.Marshal(baseline)
						var altered []ShortcutAction
						if err := json.Unmarshal(raw, &altered); err != nil {
							t.Fatal(err)
						}
						targets := p15PreviousTargets(altered, tc.consumer)
						if len(targets) != 1 {
							t.Fatal("control missing actual consumer")
						}
						target := targets[0]
						switch fault {
						case "changed_source":
							altered[0].WFWorkflowActionIdentifier = "is.workflow.actions.number"
							altered[0].WFWorkflowActionParameters = map[string]any{"WFNumberActionNumber": 99, "UUID": altered[0].WFWorkflowActionParameters["UUID"]}
						case "removed_consumer":
							altered = append(altered[:target], altered[target+1:]...)
						case "dangling_reference":
							ref := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ActionOutput", "OutputUUID": "MISSING-AUTHORED-PRODUCER"}}
							if tc.consumer == "log" {
								altered[target].WFWorkflowActionParameters["WFInput"] = ref
							} else {
								altered[target-1].WFWorkflowActionIdentifier = "is.workflow.actions.getvariable"
								altered[target-1].WFWorkflowActionParameters = map[string]any{"WFVariable": ref}
							}
						case "wrong_consumer_parameters":
							altered[target].WFWorkflowActionParameters["FOREIGN_PARAMETER"] = "changed"
						}
						if p15PreviousEquivalence(baseline, altered, tc, BuildProfileDebug) == nil {
							t.Fatal("accepted causal fault " + fault)
						}
					})
				}
			})
		}
	}
}
