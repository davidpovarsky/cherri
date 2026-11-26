package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"howett.net/plist"
)

// This is the complete authored collection-producing region of both original
// probes: one tagged List per iteration, a separate scalar control, and both
// count consumers. Only compiler-owned variable/group/producer names are read
// from their required positions; payloads and every edge are authored here.
func p15F04ProbeCellRegionError(actions []map[string]any, empty bool) error {
	if len(actions) < 20 {
		return fmt.Errorf("original probe collection region is missing")
	}
	g := &p15FunctionGraph{actual: actions[5:20], groups: map[string]bool{}}
	input := g.value("list", map[string]any{"WFItems": []any{
		map[string]any{"WFItemType": 0, "WFValue": "LEFT"},
		map[string]any{"WFItemType": 0, "WFValue": "RIGHT"},
	}})
	accumulator := p15F04ClearedCollection(g, "loopResults")
	begin := func() string {
		group, _ := p15Param(g.actual[len(g.expected)])["GroupingIdentifier"].(string)
		if group == "" || g.groups[group] {
			g.err = fmt.Errorf("original probe repeat identity is absent or reused")
		}
		g.groups[group] = true
		g.emit("repeat.each", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 0, "WFInput": p15FunctionEdge(input)})
		g.emit("setvariable", map[string]any{"WFVariableName": "item", "WFInput": p15FunctionVariable("Repeat Item")})
		return group
	}
	group := begin()
	payload := p15F04ArrayValue()
	if !empty {
		payload = p15F04ArrayValue(p15FunctionArgument{true, 0, p15FunctionVariable("item")}, p15FunctionString("TAIL"))
	}
	p15F04AppendCell(g, accumulator, true, payload)
	g.emit("repeat.each", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2})
	cells := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(accumulator)})
	group = begin()
	g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("item")})
	scalars := g.value("repeat.each", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": 2})
	g.value("count", map[string]any{"Input": p15FunctionEdge(cells)})
	g.value("count", map[string]any{"Input": p15FunctionEdge(scalars)})
	return g.compare()
}

// ff04ee8's exact signed probes remain historical runtime failures. Current
// compilation supports their local tagged List transport; the private value
// must still fail closed at a public output boundary. No Apple run is inferred.
func TestP15F04NestedListProbesFailClosed(t *testing.T) {
	for _, tc := range []struct{ name, binding, digest string }{
		{"qa-f04-yield-list-v2", "pairs", "c0719b046c2ed724fccef9e5b09e313c7dd72c02dc2c72ce1abe8aab1592e091"},
		{"qa-f04-yield-empty-list-v2", "empties", "d1322f6c7c663d266b0e9a7e272a92ae331be0e411818dae0c7d5172587578c3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", tc.name+".cherri")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(source)) != tc.digest {
				t.Fatal("original native probe source changed")
			}
			for _, required := range []string{"ANCHOR_CARD_01_TOP_HEADER", "ANCHOR_CARD_28_BOTTOM_FOOTER", "debugClearLogs()", "debugTraceEvent(stage: \"before\"", "debugTraceEvent(stage: \"after\"", "debugExportTrace(runId: \"" + tc.name + "\""} {
				if !strings.Contains(string(source), required) {
					t.Fatal("historical native-probe correlation structure missing", required)
				}
			}
			for _, diagnostic := range p15TypeDiagnostics(t, string(source)) {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
			raw, err := CompileSourceToPlist(path, string(source))
			if err != nil || len(raw) == 0 {
				t.Fatalf("original local List probe did not compile: %v", err)
			}
			var document map[string]any
			if _, err := plist.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			var actions []map[string]any
			for _, action := range document["WFWorkflowActions"].([]any) {
				actions = append(actions, action.(map[string]any))
			}
			empty := tc.binding == "empties"
			if err := p15F04ProbeCellRegionError(actions, empty); err != nil {
				t.Fatal(err)
			}
			// Causal checks reject flattened/missing payloads, the wrong cell,
			// crossed loop groups, and a count of the scalar control instead.
			for _, fault := range []string{"payload", "append", "group", "count"} {
				data, err := json.Marshal(actions)
				if err != nil {
					t.Fatal(err)
				}
				var changed []map[string]any
				if err := json.Unmarshal(data, &changed); err != nil {
					t.Fatal(err)
				}
				switch fault {
				case "payload":
					fields := p15LiteralDictionaryFields(t, p15Param(changed[10])["WFItems"])
					value := fields["value"]["WFValue"].(map[string]any)
					delete(value, "Value")
				case "append":
					p15Param(changed[11])["WFInput"] = p15FunctionEdge(p15Param(changed[5])["UUID"].(string))
				case "group":
					p15Param(changed[12])["GroupingIdentifier"] = p15Param(changed[14])["GroupingIdentifier"]
				case "count":
					p15Param(changed[18])["Input"] = p15FunctionEdge(p15Param(changed[17])["UUID"].(string))
				}
				if p15F04ProbeCellRegionError(changed, empty) == nil {
					t.Fatal("original probe oracle accepted corruption", fault)
				}
			}
			private := strings.Replace(string(source), "output(finalSummary)", "output("+tc.binding+")", 1)
			if private == string(source) {
				t.Fatal("private output control was not formed")
			}
			rejected, err := CompileSourceToPlist(path, private)
			if err == nil || len(rejected) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("private List escaped into public output: bytes=%d err=%v", len(rejected), err)
			}
			t.Log("Compiler region and private-output rejection only; historical native results remain unchanged.")
		})
	}
}
