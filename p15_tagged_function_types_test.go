package main

import (
	"strings"
	"testing"
)

// Generic Optional and postfix ? have the same checked source contract.
// Empty literal knowledge is recursive, while native Unknown remains opaque.
func TestP15TaggedFunctionTypeContracts(t *testing.T) {
	for name, source := range map[string]string{
		"generic_optional_false":    "function present(value: Optional<Bool>)->Bool { return value != none }\nreturn present(false)",
		"generic_optional_none":     "function absent(value: Optional<Bool>)->Bool { return value == none }\nreturn absent(none)",
		"nested_empty_argument":     "function size(values:List<List<Number>>)->Number { return count(values) }\nreturn size([[1,2],[]])",
		"nested_empty_binding":      "let values:List<List<Number>>=[[1,2],[]]\nreturn count(values)",
		"optional_slots_argument":   "function size(values:List<Optional<Bool>>)->Number { return count(values) }\nreturn size([none,false,true])",
		"nested_map_empty_argument": "function size(values:List<Map<Text,Number>>)->Number { return count(values) }\nreturn size([{\"x\":1},{}])",
		"postfix_optional_control":  "function present(value:Bool?)->Bool { return value != none }\nreturn present(false)",
	} {
		t.Run(name, func(t *testing.T) { p15CompiledActions(t, source) })
	}
	for name, source := range map[string]string{
		"wrong_inner_number":   "function size(values:List<List<Number>>)->Number { return count(values) }\nreturn size([[1], [\"bad\"]])",
		"wrong_inner_optional": "function size(values:List<Optional<Bool>>)->Number { return count(values) }\nreturn size([none, \"bad\"])",
		"opaque_nested_list":   "function size(values:List<List<Number>>)->Number { return count(values) }\nlet value=getVariable(\"opaque\")\nreturn size(value)",
		"opaque_slot":          "function size(values:List<List<Number>>)->Number { return count(values) }\nlet value=getVariable(\"opaque\")\nreturn size([value,[]])",
		"wrong_map_value":      "function size(values:List<Map<Text,Number>>)->Number { return count(values) }\nreturn size([{\"x\":\"bad\"},{}])",
		"optional_arity":       "function size(value:Optional<Bool,Number>)->Number { return 1 }\nreturn size(false)",
	} {
		t.Run(name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("p15-tagged-function-type.cherri", source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") {
				t.Fatalf("a concrete type contract was not preserved: bytes=%d err=%v", len(raw), err)
			}
		})
	}
}
