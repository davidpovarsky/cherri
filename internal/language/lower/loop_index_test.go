package lower

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/ir"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
)

func p15F02Lower(t *testing.T, text string) *ir.NativeWorkflow {
	t.Helper()
	var parser = syntax.NewParser(source.NewFile("loop-index", "loop-index.cherri", 1, text))
	var program = parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatalf("parse loop source: %v", parser.Errors())
	}
	var workflow, err = NewLowerer(schema.DefaultRegistry()).LowerProgram(program)
	if err != nil {
		t.Fatal(err)
	}
	return workflow
}

// Read the actual lowerer graph. Native Repeat Index is one-based (also observed
// in the retained e61 iOS trace); named V2 indices must consume its minus-one
// producer in the correct loop body. This is not an iOS execution oracle.
func p15F02IndexGraph(workflow *ir.NativeWorkflow) ([]string, error) {
	var stack []string
	var seenGroups = map[string]bool{}
	var result []string
	for i, action := range workflow.Actions {
		if action.AppleIdentifier == "is.workflow.actions.repeat.count" || action.AppleIdentifier == "is.workflow.actions.repeat.each" {
			switch action.ControlFlowMode {
			case 0:
				if action.GroupingIdentifier == "" || seenGroups[action.GroupingIdentifier] {
					return nil, fmt.Errorf("loop %d has missing/reused group", i)
				}
				seenGroups[action.GroupingIdentifier] = true
				stack = append(stack, action.GroupingIdentifier)
			case 2:
				if len(stack) == 0 || stack[len(stack)-1] != action.GroupingIdentifier {
					return nil, fmt.Errorf("loop %d closes the wrong owner", i)
				}
				stack = stack[:len(stack)-1]
			default:
				return nil, fmt.Errorf("loop %d has unexpected mode", i)
			}
			continue
		}
		if action.AppleIdentifier != "is.workflow.actions.math" {
			continue
		}
		if len(stack) == 0 || action.Parameters["WFMathOperation"] != "-" || action.Parameters["WFMathOperand"] != float64(1) {
			return nil, fmt.Errorf("index conversion %d must subtract exactly one inside its loop", i)
		}
		var name = "Repeat Index"
		if len(stack) > 1 {
			name = fmt.Sprintf("Repeat Index %d", len(stack))
		}
		var input, ok = action.Parameters["WFInput"].(*ir.AttachmentToken)
		if !ok || input.Type != "Variable" || input.OutputName != name || input.OutputUUID != "" {
			return nil, fmt.Errorf("index conversion %d reads wrong native owner: %+v, want %s", i, input, name)
		}
		if action.OutputUUID == "" || action.OutputUUID != action.NodeID || i+1 >= len(workflow.Actions) {
			return nil, fmt.Errorf("index conversion %d has no distinct producer identity", i)
		}
		var binding = workflow.Actions[i+1]
		var value, attached = binding.Parameters["WFInput"].(*ir.AttachmentToken)
		var destination, named = binding.Parameters["WFVariableName"].(string)
		if binding.AppleIdentifier != "is.workflow.actions.setvariable" || !attached || value.Type != "ActionOutput" || value.OutputUUID != action.OutputUUID || !named || destination == "" {
			return nil, fmt.Errorf("index conversion %d is not captured by its named binding", i)
		}
		result = append(result, destination+"@"+name)
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed loop groups: %v", stack)
	}
	return result, nil
}

func TestP15F02NamedIndicesUseZeroBasedNativeProducer(t *testing.T) {
	var cases = []struct {
		name, text string
		want       []string
	}{
		{"named_repeat", `repeat 3 as index { show(index) }`, []string{"index@Repeat Index"}},
		{"tuple_for", `for (index, item) in ["A", "B"] { show(index); show(item) }`, []string{"index@Repeat Index"}},
		{"mixed_nested_and_siblings", `for (outerIndex, outerItem) in ["A", "B"] {
    repeat 2 as middleIndex {
        for (innerIndex, innerItem) in ["1", "2"] { show(f"{outerIndex}:{outerItem}:{middleIndex}:{innerIndex}:{innerItem}") }
        show(middleIndex)
    }
    show(outerIndex)
}
for (siblingIndex, siblingItem) in ["C"] { show(siblingIndex) }
repeat 1 as lastIndex { show(lastIndex) }`, []string{"outerIndex@Repeat Index", "middleIndex@Repeat Index 2", "innerIndex@Repeat Index 3", "siblingIndex@Repeat Index", "lastIndex@Repeat Index"}},
		{"ignored_indices", `repeat 2 { show(RepeatIndex) }
repeat 2 as _ { show(RepeatIndex) }
for (_, item) in ["A"] { show(item) }
for item in ["B"] { show(item) }`, nil},
	}
	for _, sample := range cases {
		t.Run(sample.name, func(t *testing.T) {
			var got, err = p15F02IndexGraph(p15F02Lower(t, sample.text))
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, sample.want) {
				t.Fatalf("zero-based native index bindings: got %v, want %v", got, sample.want)
			}
		})
	}
}

func TestP15F02IndexGraphRejectsCausalMutations(t *testing.T) {
	var sample = `repeat 2 as outerIndex { for (innerIndex, item) in ["A", "B"] { show(innerIndex) } }`
	var cases = map[string]func(*ir.NativeWorkflow, int){
		"one_based_binding": func(w *ir.NativeWorkflow, i int) {
			w.Actions[i+1].Parameters["WFInput"] = w.Actions[i].Parameters["WFInput"]
		},
		"wrong_depth": func(w *ir.NativeWorkflow, i int) {
			w.Actions[i].Parameters["WFInput"] = &ir.AttachmentToken{Type: "Variable", OutputName: "Repeat Index 99"}
		},
		"wrong_offset":    func(w *ir.NativeWorkflow, i int) { w.Actions[i].Parameters["WFMathOperand"] = float64(2) },
		"wrong_direction": func(w *ir.NativeWorkflow, i int) { w.Actions[i].Parameters["WFMathOperation"] = "+" },
		"dangling_capture": func(w *ir.NativeWorkflow, i int) {
			w.Actions[i+1].Parameters["WFInput"] = &ir.AttachmentToken{Type: "ActionOutput", OutputUUID: "absent"}
		},
		"wrong_loop_end": func(w *ir.NativeWorkflow, i int) {
			for _, action := range w.Actions {
				if action.AppleIdentifier == "is.workflow.actions.repeat.each" && action.ControlFlowMode == 2 {
					action.GroupingIdentifier = "absent"
					return
				}
			}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			var workflow = p15F02Lower(t, sample)
			var index = -1
			for i, action := range workflow.Actions {
				if action.AppleIdentifier == "is.workflow.actions.math" {
					index = i
					break
				}
			}
			if index < 0 {
				t.Fatal("production graph has no zero-base conversion to mutate")
			}
			if _, err := p15F02IndexGraph(workflow); err != nil {
				t.Fatal(err)
			}
			mutate(workflow, index)
			if _, err := p15F02IndexGraph(workflow); err == nil {
				t.Fatal("causal index mutation was accepted")
			}
		})
	}
}
