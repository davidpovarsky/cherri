package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

const p15F02FullLoopSource = `for (outerIndex, outerItem) in ["A", "B"] {
    for (innerIndex, innerItem) in [1, 2] {
        show(f"F02_PAIR:{outerItem}:{innerItem}:{outerIndex}:{innerIndex}")
    }
    show(f"F02_AFTER:{outerItem}:{outerIndex}")
}
repeat 2 as repeatIndex { show(f"F02_REPEAT:{repeatIndex}") }
`

const p15RP22FullLoopSource = `var index = 99
var item = "outside"
for (index, item) in ["A", "B"] {
    show(f"RP22_OUTER:{index}:{item}")
    repeat 2 as index {
        show(f"RP22_REPEAT:{index}:{item}")
        for (index, item) in [1, 2] { show(f"RP22_INNER:{index}:{item}") }
        show(f"RP22_REPEAT_AFTER:{index}:{item}")
    }
    show(f"RP22_OUTER_AFTER:{index}:{item}")
}
show(f"RP22_RESTORED:{index}:{item}")
`

const p15BRG12FullLoopSource = `for (outerIndex, outerItem) in ["A", "B"] {
    for (innerIndex, innerItem) in [1, 2] {
        show(f"BRG12_PAIR:{outerItem}:{innerItem}:{outerIndex}:{innerIndex}")
    }
    repeat 2 as repeatIndex { show(f"BRG12_MIXED:{outerItem}:{outerIndex}:{repeatIndex}") }
    show(f"BRG12_AFTER:{outerItem}:{outerIndex}")
}
for (siblingIndex, siblingItem) in ["C", "D"] { show(f"BRG12_SIBLING:{siblingItem}:{siblingIndex}") }
repeat 2 as finalIndex {
    for (nestedIndex, nestedItem) in ["X", "Y"] { show(f"BRG12_REVERSE:{finalIndex}:{nestedIndex}:{nestedItem}") }
    show(f"BRG12_REVERSE_AFTER:{finalIndex}")
}
var index = 77
var item = "outside"
for (index, item) in ["S"] {
    repeat 2 as index { show(f"BRG12_SHADOW_INNER:{index}:{item}") }
    show(f"BRG12_SHADOW_OUTER:{index}:{item}")
}
show(f"BRG12_RESTORED:{index}:{item}")
`

type p15LoopMarker struct {
	Name         string
	Names, Types []string
}

func p15LoopSource(id string) string {
	switch id {
	case "F02":
		return p15F02FullLoopSource
	case "RP22":
		return p15RP22FullLoopSource
	case "BRG12":
		return p15BRG12FullLoopSource
	}
	panic("unknown loop requirement")
}

func p15LoopMarkers(id string) []p15LoopMarker {
	switch id {
	case "F02":
		return []p15LoopMarker{
			{"F02_PAIR", []string{"outerItem", "innerItem", "outerIndex", "innerIndex"}, []string{"Text", "Number", "Number", "Number"}},
			{"F02_AFTER", []string{"outerItem", "outerIndex"}, []string{"Text", "Number"}},
			{"F02_REPEAT", []string{"repeatIndex"}, []string{"Number"}},
		}
	case "RP22":
		return []p15LoopMarker{
			{"RP22_OUTER", []string{"index", "item"}, []string{"Number", "Text"}},
			{"RP22_REPEAT", []string{"index", "item"}, []string{"Number", "Text"}},
			{"RP22_INNER", []string{"index", "item"}, []string{"Number", "Number"}},
			{"RP22_REPEAT_AFTER", []string{"index", "item"}, []string{"Number", "Text"}},
			{"RP22_OUTER_AFTER", []string{"index", "item"}, []string{"Number", "Text"}},
			{"RP22_RESTORED", []string{"index", "item"}, []string{"Number", "Text"}},
		}
	case "BRG12":
		return []p15LoopMarker{
			{"BRG12_PAIR", []string{"outerItem", "innerItem", "outerIndex", "innerIndex"}, []string{"Text", "Number", "Number", "Number"}},
			{"BRG12_MIXED", []string{"outerItem", "outerIndex", "repeatIndex"}, []string{"Text", "Number", "Number"}},
			{"BRG12_AFTER", []string{"outerItem", "outerIndex"}, []string{"Text", "Number"}},
			{"BRG12_SIBLING", []string{"siblingItem", "siblingIndex"}, []string{"Text", "Number"}},
			{"BRG12_REVERSE", []string{"finalIndex", "nestedIndex", "nestedItem"}, []string{"Number", "Number", "Text"}},
			{"BRG12_REVERSE_AFTER", []string{"finalIndex"}, []string{"Number"}},
			{"BRG12_SHADOW_INNER", []string{"index", "item"}, []string{"Number", "Text"}},
			{"BRG12_SHADOW_OUTER", []string{"index", "item"}, []string{"Number", "Text"}},
			{"BRG12_RESTORED", []string{"index", "item"}, []string{"Number", "Text"}},
		}
	}
	panic("unknown loop requirement")
}

func p15LoopScopeTypes(t *testing.T, id string) {
	t.Helper()
	source := p15LoopSource(id)
	program, parseErrors := p15ParseProgram(source)
	if len(parseErrors) != 0 {
		t.Fatal(parseErrors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	if diagnostics := analyzer.Diagnostics(); len(diagnostics) != 0 {
		t.Fatalf("valid loop scenario rejected: %#v", diagnostics)
	}
	for _, marker := range p15LoopMarkers(id) {
		prefix := marker.Name + ":"
		start := strings.Index(source, prefix)
		if start < 0 || strings.Count(source, prefix) != 1 {
			t.Fatal("ambiguous authored marker", marker.Name)
		}
		lineEnd := strings.Index(source[start:], "\"")
		if lineEnd < 0 {
			t.Fatal("unterminated authored marker")
		}
		line := source[start : start+lineEnd]
		for i, name := range marker.Names {
			offset := strings.Index(line, "{"+name+"}")
			if offset < 0 || strings.Count(line, "{"+name+"}") != 1 {
				t.Fatal("ambiguous scoped use", marker.Name, name)
			}
			wantStart := start + offset + 1
			found := 0
			for expr, inferred := range analyzer.ExprTypes() {
				span := expr.NodeSpan()
				if span.Start.ByteOffset == wantStart && span.End.ByteOffset == wantStart+len(name) {
					found++
					if fmt.Sprint(inferred) != marker.Types[i] {
						t.Fatalf("%s.%s inferred %v, want %s", marker.Name, name, inferred, marker.Types[i])
					}
				}
			}
			if found != 1 {
				t.Fatalf("%s.%s has %d exact AST type observations", marker.Name, name, found)
			}
		}
	}
}

func p15LoopRejectsLeak(t *testing.T, source, name string) {
	t.Helper()
	diagnostics := p15TypeDiagnostics(t, source)
	start := strings.LastIndex(source, name)
	if len(diagnostics) != 1 || diagnostics[0].Code != analysis.CodeUnknownName ||
		diagnostics[0].Severity != analysis.SeverityError || diagnostics[0].Span.Start.ByteOffset != start ||
		diagnostics[0].Span.End.ByteOffset != start+len(name) {
		t.Fatalf("loop-local name did not produce its exact out-of-scope diagnostic: %#v", diagnostics)
	}
	data, err := CompileSourceToPlist("p15-loop-leak.cherri", source)
	if err == nil || len(data) != 0 || !strings.Contains(err.Error(), analysis.CodeUnknownName) {
		t.Fatalf("out-of-scope loop reference reached native emission: bytes=%d err=%v", len(data), err)
	}
}

func p15LoopUnitScenario(t *testing.T, id string) {
	t.Helper()
	t.Run("scoped_types", func(t *testing.T) { p15LoopScopeTypes(t, id) })
	for _, tc := range []struct{ name, source, leak string }{
		{"tuple_index_does_not_escape", "for (loopIndex,loopItem) in [\"A\"] { show(loopIndex);show(loopItem) }\nshow(loopIndex)", "loopIndex"},
		{"tuple_item_does_not_escape", "for (loopIndex,loopItem) in [\"A\"] { show(loopIndex);show(loopItem) }\nshow(loopItem)", "loopItem"},
		{"named_repeat_index_does_not_escape", "repeat 2 as loopIndex { show(loopIndex) }\nshow(loopIndex)", "loopIndex"},
	} {
		t.Run(tc.name, func(t *testing.T) { p15LoopRejectsLeak(t, tc.source, tc.leak) })
	}
	t.Run("healthy_after_rejection", func(t *testing.T) {
		if err := p15LoopNativeError(id, p15CompiledActions(t, p15LoopSource(id))); err != nil {
			t.Fatal(err)
		}
	})
}

func TestP15F02LoopScopeUnit(t *testing.T)               { p15LoopUnitScenario(t, "F02") }
func TestP15RP22LoopBindingsUnit(t *testing.T)           { p15LoopUnitScenario(t, "RP22") }
func TestBackendRecovery12LoopIdentityUnit(t *testing.T) { p15LoopUnitScenario(t, "BRG12") }

func p15LoopNativeScenario(t *testing.T, id string) {
	t.Helper()
	actions := p15CompiledActions(t, p15LoopSource(id))
	if err := p15LoopNativeError(id, actions); err != nil {
		t.Fatal(err)
	}
	t.Run("consistent_uuid_renaming", func(t *testing.T) {
		changed := p15LoopClone(actions).([]map[string]any)
		p15LoopRenameIdentities(changed, false)
		if err := p15LoopNativeError(id, changed); err != nil {
			t.Fatal("identity-preserving alpha rename rejected", err)
		}
	})
	if id != "F02" {
		t.Run("consistent_shadow_renaming", func(t *testing.T) {
			changed := p15LoopClone(actions).([]map[string]any)
			p15LoopRenameIdentities(changed, true)
			if err := p15LoopNativeError(id, changed); err != nil {
				t.Fatal("scope-preserving alpha rename rejected", err)
			}
		})
	}
	for _, fault := range p15LoopMutationNames(id) {
		t.Run(fault, func(t *testing.T) {
			changed := p15LoopClone(actions).([]map[string]any)
			changed = p15LoopMutate(changed, fault)
			if reflect.DeepEqual(changed, actions) {
				t.Fatal("control did not change actual native graph", fault)
			}
			if p15LoopNativeError(id, changed) == nil {
				t.Fatal("causal loop graph corruption accepted", fault)
			}
		})
	}
}

func TestP15F02LoopNativeGraph(t *testing.T)            { p15LoopNativeScenario(t, "F02") }
func TestP15RP22LoopNativeGraph(t *testing.T)           { p15LoopNativeScenario(t, "RP22") }
func TestBackendRecovery12LoopNativeGraph(t *testing.T) { p15LoopNativeScenario(t, "BRG12") }
