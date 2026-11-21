package main

import (
	"fmt"
	"math"
	"reflect"
	"testing"

	"howett.net/plist"
)

// This oracle reads the serialized production graph, including the exact
// lookup inputs and the control-flow ancestry of that lookup. It does not
// execute Apple actions or substitute a second lowering implementation.
type p15IndexGroup struct {
	start, otherwise, end int
	name                  string
}
type p15IndexShape struct {
	add, count, diff, modulo, lookup, consumer int
	groups                                     []p15IndexGroup
}

func p15IndexNumber(value any) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case int64:
		return float64(v), true
	case uint64:
		return float64(v), true
	case int:
		return float64(v), true
	}
	return 0, false
}

func p15IndexRef(value any) map[string]any {
	m, _ := value.(map[string]any)
	if m["Type"] == "Variable" && m["Variable"] != nil {
		return p15IndexRef(m["Variable"])
	}
	return p15DiagnosticValue(value)
}

func p15IndexOutput(value any, uuid any) bool {
	r := p15IndexRef(value)
	return uuid != nil && r["Type"] == "ActionOutput" && r["OutputUUID"] == uuid && r["Aggrandizements"] == nil
}

func p15IndexGraph(actions []map[string]any, integral bool) (p15IndexShape, error) {
	s := p15IndexShape{add: -1, count: -1, diff: -1, modulo: -1, lookup: -1, consumer: -1}
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		return s, err
	}
	type frame struct {
		group  int
		branch int
	}
	var stack []frame
	var lookupFrames []frame
	target := -1
	for i, action := range actions {
		p := p15Param(action)
		id := action["WFWorkflowActionIdentifier"]
		if p["CustomOutputName"] == "items" {
			if target >= 0 {
				return s, fmt.Errorf("duplicate collection producer")
			}
			target = i
		}
		slot := (*int)(nil)
		switch id {
		case "is.workflow.actions.math":
			switch p["WFMathOperation"] {
			case "+":
				slot = &s.add
			case "-":
				slot = &s.diff
			case "…":
				if p["WFScientificMathOperation"] == "Modulus" {
					slot = &s.modulo
				}
			}
		case "is.workflow.actions.count":
			slot = &s.count
		case "is.workflow.actions.getitemfromlist":
			slot = &s.lookup
			lookupFrames = append([]frame{}, stack...)
		case "is.workflow.actions.getvariable":
			s.consumer = i
		case "is.workflow.actions.conditional":
			name, _ := p["GroupingIdentifier"].(string)
			switch p15Int(p["WFControlFlowMode"]) {
			case 0:
				if name == "" {
					return s, fmt.Errorf("missing group identity")
				}
				for _, g := range s.groups {
					if g.name == name {
						return s, fmt.Errorf("duplicate group")
					}
				}
				s.groups = append(s.groups, p15IndexGroup{i, -1, -1, name})
				stack = append(stack, frame{len(s.groups) - 1, 0})
			case 1, 2:
				if len(stack) == 0 {
					return s, fmt.Errorf("unmatched branch/end")
				}
				f := &stack[len(stack)-1]
				g := &s.groups[f.group]
				if g.name != name {
					return s, fmt.Errorf("crossed groups")
				}
				if p15Int(p["WFControlFlowMode"]) == 1 {
					if f.branch != 0 {
						return s, fmt.Errorf("duplicate else")
					}
					f.branch = 1
					g.otherwise = i
				} else {
					if f.branch != 1 {
						return s, fmt.Errorf("missing else")
					}
					g.end = i
					stack = stack[:len(stack)-1]
				}
			default:
				return s, fmt.Errorf("invalid control mode")
			}
		}
		if slot != nil {
			if *slot >= 0 {
				return s, fmt.Errorf("duplicate index operation")
			}
			*slot = i
		}
	}
	if len(stack) != 0 || target < 0 || s.add < 0 || s.count < 0 || s.diff < 0 || s.lookup < 0 || s.consumer <= s.lookup {
		return s, fmt.Errorf("incomplete guarded index graph")
	}
	p := func(i int) map[string]any { return p15Param(actions[i]) }
	one, oneOK := p15IndexNumber(p(s.add)["WFMathOperand"])
	if !oneOK || one != 1 || p(s.count)["WFCountType"] != "Items" ||
		!p15IndexOutput(p(s.count)["Input"], p(target)["UUID"]) ||
		!p15IndexOutput(p(s.lookup)["WFInput"], p(target)["UUID"]) ||
		!p15IndexOutput(p(s.diff)["WFInput"], p(s.count)["UUID"]) ||
		!p15IndexOutput(p(s.diff)["WFMathOperand"], p(s.add)["UUID"]) ||
		!p15IndexOutput(p(s.lookup)["WFItemIndex"], p(s.add)["UUID"]) ||
		p(s.lookup)["WFItemSpecifier"] != "Item At Index" {
		return s, fmt.Errorf("index arithmetic or original collection edge changed")
	}
	wantGroups := 2
	if integral {
		wantGroups = 3
	}
	if len(s.groups) != wantGroups || len(lookupFrames) != wantGroups {
		return s, fmt.Errorf("lookup needs %d exact guard ancestors, found %d", wantGroups, len(lookupFrames))
	}
	for i, f := range lookupFrames {
		g := s.groups[f.group]
		if f.branch != 0 || f.group != i || g.start >= s.lookup || g.otherwise <= s.lookup ||
			g.end != g.otherwise+2 || actions[g.otherwise+1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.nothing" {
			return s, fmt.Errorf("lookup not in valid then branch with Nothing fallback")
		}
	}
	check := func(group, producer, condition int, threshold float64) bool {
		g := p(s.groups[group].start)
		n, ok := p15IndexNumber(g["WFNumberValue"])
		return ok && n == threshold && p15Int(g["WFCondition"]) == condition &&
			g["WFConditionalLegacyComparisonBehavior"] == true &&
			p15IndexOutput(g["WFInput"], p(producer)["UUID"])
	}
	offset := 0
	if integral {
		if s.modulo < 0 || s.modulo >= s.groups[0].start || s.groups[0].start >= s.add {
			return s, fmt.Errorf("dynamic index lacks pre-bounds integrality guard")
		}
		divisor, ok := p15IndexNumber(p(s.modulo)["WFScientificMathOperand"])
		if !ok || divisor != 1 || !reflect.DeepEqual(p(s.modulo)["WFInput"], p(s.add)["WFInput"]) ||
			!check(0, s.modulo, 4, 0) {
			return s, fmt.Errorf("integrality guard is not exact same index modulo one equals zero")
		}
		offset = 1
	} else if s.modulo >= 0 {
		return s, fmt.Errorf("known integer gained a redundant integrality action")
	}
	if !check(offset, s.add, 3, 1) || !check(offset+1, s.diff, 3, 0) ||
		!(s.add < s.count && s.count < s.diff && s.diff < s.groups[offset].start) {
		return s, fmt.Errorf("lower or upper index bound changed")
	}
	end := s.groups[0].end
	if s.consumer <= end || p(end)["CustomOutputName"] != "observed" ||
		!p15IndexOutput(p(s.consumer)["WFVariable"], p(end)["UUID"]) {
		return s, fmt.Errorf("consumer does not retain the complete guarded result")
	}
	return s, nil
}

func p15IndexCollection(actions []map[string]any, shape p15IndexShape, want []string) error {
	input := p15Param(actions[shape.count])["Input"]
	if len(want) == 0 {
		_, err := p15EmptyOrigin(actions, shape.count, input)
		return err
	}
	uuid := p15IndexRef(input)["OutputUUID"]
	for _, action := range actions {
		p := p15Param(action)
		if p["UUID"] != uuid {
			continue
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" {
			return fmt.Errorf("collection producer is not List")
		}
		rows, ok := p["WFItems"].([]any)
		if !ok || len(rows) != len(want) {
			return fmt.Errorf("collection cardinality changed")
		}
		for i, item := range rows {
			row, ok := item.(map[string]any)
			if !ok || p15Int(row["WFItemType"]) != 0 || row["WFValue"] != want[i] {
				return fmt.Errorf("collection element %d changed", i)
			}
		}
		return nil
	}
	return fmt.Errorf("missing collection producer")
}

func p15IndexSource(items, setup, index string) string {
	return "let items = " + items + "\n" + setup + "let observed = items[" + index + "]\ngetVariable(observed)\n"
}

func TestP15L04DynamicFractionalIndexGuard(t *testing.T) {
	src := p15IndexSource(`["alpha","beta","gamma"]`, "var index = number(0.5)\n", "index")
	if _, err := p15IndexGraph(p15CompiledActions(t, src), true); err != nil {
		t.Fatal(err)
	}
}

func TestP15L02IndexBoundaryWire(t *testing.T) {
	for _, tc := range []struct {
		name, items, setup, index string
		dynamic                   bool
	}{
		{"first", `["alpha","beta","gamma"]`, "", "0", false},
		{"last", `["alpha","beta","gamma"]`, "", "2", false},
		{"negative", `["alpha","beta","gamma"]`, "", "-1", false},
		{"out_of_range", `["alpha","beta","gamma"]`, "", "99", false},
		{"empty", "[]", "", "0", false},
		{"uint64_max", `["alpha","beta","gamma"]`, "", "18446744073709551615", false},
		{"dynamic_first", `["alpha","beta","gamma"]`, "var index = number(0)\n", "index", true},
		{"dynamic_last", `["alpha","beta","gamma"]`, "var index = number(2)\n", "index", true},
		{"dynamic_negative", `["alpha","beta","gamma"]`, "var index = number(-1)\n", "index", true},
		{"dynamic_out_of_range", `["alpha","beta","gamma"]`, "var index = number(99)\n", "index", true},
		{"dynamic_fraction", `["alpha","beta","gamma"]`, "var index = number(0.5)\n", "index", true},
		{"dynamic_negative_fraction", `["alpha","beta","gamma"]`, "var index = number(-0.5)\n", "index", true},
		{"dynamic_empty", "[]", "var index = number(0)\n", "index", true},
		{"inline_action", `["alpha","beta","gamma"]`, "", "number(0.5)", true},
		{"empty_text_item", `[""]`, "var index = number(0)\n", "index", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			actions := p15CompiledActions(t, p15IndexSource(tc.items, tc.setup, tc.index))
			s, err := p15IndexGraph(actions, tc.dynamic)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{"alpha", "beta", "gamma"}
			if tc.items == "[]" {
				want = nil
			} else if tc.items == `[""]` {
				want = []string{""}
			}
			if err := p15IndexCollection(actions, s, want); err != nil {
				t.Fatal(err)
			}
			if tc.dynamic {
				numbers := 0
				var numberUUID any
				for _, a := range actions {
					if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" {
						numbers++
						numberUUID = p15Param(a)["UUID"]
					}
				}
				if numbers != 1 {
					t.Fatalf("index action evaluated %d times", numbers)
				}
				ref := p15IndexRef(p15Param(actions[s.add])["WFInput"])
				if tc.setup == "" {
					if !p15IndexOutput(p15Param(actions[s.add])["WFInput"], numberUUID) {
						t.Fatal("inline action output was replaced")
					}
				} else if ref["Type"] != "Variable" || ref["VariableName"] != "index" {
					t.Fatal("mutable index input was replaced")
				}
			}
			if tc.name == "uint64_max" && p15Param(actions[s.add])["WFInput"] != uint64(math.MaxUint64) {
				t.Fatal("known integer was truncated or coerced before index arithmetic")
			}
		})
	}
	for _, index := range []string{"0.5", "-0.5", "2.5"} {
		t.Run("literal_fraction_"+index, func(t *testing.T) {
			actions := p15CompiledActions(t, p15IndexSource(`["alpha","beta","gamma"]`, "", index))
			if err := p15M02MainVoidBoundaryError(actions); err != nil {
				t.Fatal(err)
			}
			nothing, emptyValues := -1, 0
			for i, a := range actions {
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.getitemfromlist" {
					t.Fatal("literal fraction executes lookup")
				}
				if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.nothing" && i != len(actions)-1 {
					nothing = i
					emptyValues++
				}
			}
			if emptyValues != 1 || nothing != len(actions)-3 ||
				actions[len(actions)-2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" ||
				!p15IndexOutput(p15Param(actions[len(actions)-2])["WFVariable"], p15Param(actions[nothing])["UUID"]) {
				t.Fatal("literal fraction consumer lost Nothing")
			}
		})
	}
}

func TestP15L04IndexOracleRejectsCorruption(t *testing.T) {
	base := p15CompiledActions(t, p15IndexSource(`["alpha","beta","gamma"]`, "var index = number(0.5)\n", "index"))
	shape, err := p15IndexGraph(base, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_modulus", "wrong_divisor", "different_index", "round_instead", "wrong_integral_condition", "wrong_integral_threshold", "fraction_else_lookup", "wrong_lower_bound", "wrong_upper_bound", "wrong_index_offset", "wrong_lookup_index", "wrong_collection", "missing_nothing", "crossed_group", "inner_result_alias", "drop_collection_item", "reorder_collection", "change_collection_value"} {
		t.Run(fault, func(t *testing.T) {
			raw := p15LiteralClone(t, base).([]any)
			a := make([]map[string]any, len(raw))
			for i, v := range raw {
				a[i] = v.(map[string]any)
			}
			p := func(i int) map[string]any { return p15Param(a[i]) }
			switch fault {
			case "missing_modulus":
				a[shape.modulo]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
			case "wrong_divisor":
				p(shape.modulo)["WFScientificMathOperand"] = int64(2)
			case "different_index":
				p(shape.modulo)["WFInput"] = int64(0)
			case "round_instead":
				p(shape.modulo)["WFScientificMathOperation"] = "Round"
			case "wrong_integral_condition":
				p(shape.groups[0].start)["WFCondition"] = int64(3)
			case "wrong_integral_threshold":
				p(shape.groups[0].start)["WFNumberValue"] = int64(1)
			case "fraction_else_lookup":
				a[shape.lookup], a[shape.groups[0].otherwise+1] = a[shape.groups[0].otherwise+1], a[shape.lookup]
			case "wrong_lower_bound":
				p(shape.groups[1].start)["WFNumberValue"] = int64(0)
			case "wrong_upper_bound":
				p(shape.groups[2].start)["WFCondition"] = int64(4)
			case "wrong_index_offset":
				p(shape.add)["WFMathOperand"] = int64(0)
			case "wrong_lookup_index":
				p(shape.lookup)["WFItemIndex"] = p(shape.diff)["WFInput"]
			case "wrong_collection":
				p(shape.lookup)["WFInput"] = p(shape.diff)["WFInput"]
			case "missing_nothing":
				a[shape.groups[0].otherwise+1]["WFWorkflowActionIdentifier"] = "is.workflow.actions.comment"
			case "crossed_group":
				p(shape.groups[0].end)["GroupingIdentifier"] = p(shape.groups[1].end)["GroupingIdentifier"]
			case "inner_result_alias":
				p(shape.consumer)["WFVariable"] = p15AcceptanceRef(p(shape.groups[1].end)["UUID"].(string), nil)
			case "drop_collection_item", "reorder_collection", "change_collection_value":
				for i, action := range a {
					if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" {
						continue
					}
					rows := p(i)["WFItems"].([]any)
					switch fault {
					case "drop_collection_item":
						p(i)["WFItems"] = rows[:2]
					case "reorder_collection":
						rows[0], rows[2] = rows[2], rows[0]
					case "change_collection_value":
						rows[0].(map[string]any)["WFValue"] = "changed"
					}
				}
			}
			changed, graphErr := p15IndexGraph(a, true)
			if graphErr == nil && p15IndexCollection(a, changed, []string{"alpha", "beta", "gamma"}) == nil {
				t.Fatal("accepted " + fault)
			}
		})
	}
}

func TestP15L02L04IndexNativeRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name, items, setup, index string
		dynamic                   bool
	}{
		{"first", `["alpha","beta","gamma"]`, "", "0", false},
		{"fraction", `["alpha","beta","gamma"]`, "var index=number(0.5)\n", "index", true},
		{"negative", `["alpha","beta","gamma"]`, "var index=number(-1)\n", "index", true},
		{"empty", "[]", "var index=number(0)\n", "index", true},
	} {
		root := p15MetadataNative(t, "index-"+tc.name, p15IndexSource(tc.items, tc.setup, tc.index))
		for _, obs := range p15NativeImportRoundTrip(t, "index-"+tc.name, root, plist.XMLFormat, plist.BinaryFormat) {
			raw := obs.compiled["WFWorkflowActions"].([]any)
			actions := make([]map[string]any, len(raw))
			for i, v := range raw {
				actions[i] = v.(map[string]any)
			}
			if _, err := p15IndexGraph(actions, tc.dynamic); err != nil {
				t.Fatal(err)
			}
		}
	}
}
