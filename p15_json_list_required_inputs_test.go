package main

import (
	"bytes"
	"fmt"
	"testing"

	"howett.net/plist"
)

// These are authored language capability regressions, not a substitute for an
// independent complete tagged-value graph oracle or Apple execution. The cases
// retain ordinary language syntax and distinguish JsonNull from Optional None.
// Their clean baseline must fail while the required inputs are unsupported.
func TestP15JSONListRequiredAuthoredInputs(t *testing.T) {
	cases := []struct {
		name, source, requirement string
	}{
		{"null_scalar", "let value:JsonNull=null\nreturn value==null", "T05: JSON null is a present value"},
		{"null_optional", "let value:Optional<JsonValue>=null\nreturn value==none", "T05: JSON null differs from Optional absence"},
		{"null_function", "function echo(value:JsonValue)->JsonValue {return value}\nlet value=echo(null)\nreturn value==null", "BRG15: structured function transport preserves JSON null"},
		{"null_map", "let value:Map<Text,JsonValue>={\"n\":null}\nreturn value[\"n\"]==null", "T05/RP20: explicit JSON null must not erase a key"},
		{"optional_list", "let values:List<Optional<Bool>>=[none,false,true]\nreturn count(values)", "T05/RP20: Optional absence occupies its ordered List slot"},
		{"optional_map", "let values:Map<Text,Optional<Bool>>={\"absent\":none,\"false\":false}\nreturn values[\"absent\"]==none", "T05/RP20: present Map key with None differs from missing key"},
		{"optional_empty_text", "let values:List<Optional<Text>>=[none,\"\"]\nlet first=values[0]\nreturn first==none", "T05/RP20: None and present empty Text are distinct"},
		{"nested_literal_count", "let values=[[1,2],[]]\nreturn count(values)", "L01: each nested List occupies one outer slot"},
		{"nested_dynamic_count", "let inner=[1,2]\nlet values=[inner,inner,[]]\nreturn count(values)", "L01: dynamic and repeated nested List values preserve cardinality"},
		{"nested_checked_index", "let values=[[1,2],[]]\nlet selected=values[0]\nif selected!=none {return count(selected)} else {return 0}", "L01/RP20: checked index decodes the selected nested value"},
		{"nested_iteration", "let values=[[1,2],[]]\nvar total=0\nfor value in values {total+=count(value)}\nreturn total", "L01: iteration preserves nested values including empty List"},
		{"nested_mutable_snapshot", "let source=[[1,2],[]]\nvar current=source\nlet old=current\ncurrent=[[]]\nreturn count(old)*10+count(current)", "L01/RP20: immutable alias retains the before-write nested value"},
		{"nested_function", "function size(values:List<List<Number>>)->Number {return count(values)}\nreturn size([[1,2],[]])", "BRG15: structured function input retains nested List shape"},
		{"optional_list_function", "function size(values:List<Optional<Bool>>)->Number {return count(values)}\nreturn size([none,false])", "BRG15: structured function input retains Optional slots"},
		{"dynamic_loop_payload", "let inner=[1,2]\nlet values=repeat 2 {yield inner}\nreturn count(values)", "L01/F04: a dynamic List payload retains one result per iteration"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("required authored input only; no formal tier or Apple claim: %s", tc.requirement)
			data, err := CompileSourceToPlist("p15-required-"+tc.name+".cherri", tc.source)
			if err != nil {
				if len(data) != 0 {
					t.Fatalf("rejected input leaked %d native bytes: %v", len(data), err)
				}
				t.Fatalf("required authored capability is unsupported: %v", err)
			}
			if len(data) == 0 || bytes.Contains(data, []byte("<null")) {
				t.Fatal("compiler emitted no native artifact or fabricated a plist null")
			}
			var workflow map[string]any
			if _, err := plist.Unmarshal(data, &workflow); err != nil {
				t.Fatal("compiler output is not a real plist", err)
			}
			actions, okay := workflow["WFWorkflowActions"].([]any)
			if !okay || len(actions) == 0 {
				t.Fatal("compiler artifact lacks its action graph")
			}
			t.Log(fmt.Sprintf("authored input reached a native graph of %d actions; semantic graph/runtime validation remains separate", len(actions)))
		})
	}
}
