package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func p15L01DirectMapSource(t *testing.T, route string) string {
	t.Helper()
	switch route {
	case "immutable":
		return "let data=" + p15L01NestedDictionarySource + "\ngetVariable(data)"
	case "mutable":
		return "var data=" + p15L01NestedDictionarySource + "\ngetVariable(data)"
	case "assignment":
		return "var data={}\ndata=" + p15L01NestedDictionarySource + "\ngetVariable(data)"
	}
	t.Fatalf("unknown direct Map route %q", route)
	return ""
}

// The original payload reader supplies an independent semantic expectation.
// This separate oracle closes the missing store/final-consumer checks for the
// three already supported direct Map routes. It does not prove a nested native
// List representation or grant the full L01 requirement.
func p15L01DirectMapGraphError(actions []map[string]any, route string) error {
	if route != "immutable" && route != "mutable" && route != "assignment" {
		return fmt.Errorf("unknown direct Map graph route %q", route)
	}
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	dictionary := func(want map[string]any) string {
		index := len(g.expected)
		var fields any
		if index < len(actions) {
			fields = p15Param(actions[index])["WFItems"]
		}
		got, err := p15ReadDictionaryLiteral(fields)
		if err != nil || !reflect.DeepEqual(got, want) {
			g.err = fmt.Errorf("direct Map payload at %d differs: got %#v want %#v: %v", index, got, want, err)
		}
		// Only the independently checked value is carried through. Dictionary
		// key order is not turned into an additional language requirement.
		return g.value("dictionary", map[string]any{"WFItems": fields})
	}
	store := func(value string) {
		g.emit("setvariable", map[string]any{"WFVariableName": "data", "WFInput": p15FunctionEdge(value)})
	}
	if route == "assignment" {
		store(dictionary(map[string]any{}))
	}
	value := dictionary(p15L01ExpectedNestedDictionary())
	if route == "immutable" {
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	} else {
		store(value)
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionVariable("data")})
	}
	g.emit("nothing", nil)
	if err := p15S01CompleteGraph(g); err != nil {
		return err
	}
	return p15M02MainVoidBoundaryError(actions)
}

func TestP15L01DirectMapStoresAndConsumers(t *testing.T) {
	for _, route := range []string{"immutable", "mutable", "assignment"} {
		t.Run(route, func(t *testing.T) {
			original := p15CompiledActions(t, p15L01DirectMapSource(t, route))
			if err := p15L01DirectMapGraphError(original, route); err != nil {
				t.Fatal("actual complete supported Map graph", err)
			}
			var dictionaries, stores []int
			consumer := -1
			for index, action := range original {
				switch action["WFWorkflowActionIdentifier"] {
				case "is.workflow.actions.dictionary":
					dictionaries = append(dictionaries, index)
				case "is.workflow.actions.setvariable":
					stores = append(stores, index)
				case "is.workflow.actions.getvariable":
					if consumer >= 0 {
						t.Fatal("actual source has a duplicated final consumer")
					}
					consumer = index
				}
			}
			wantDictionaries, wantStores := 1, 0
			if route == "mutable" {
				wantStores = 1
			}
			if route == "assignment" {
				wantDictionaries, wantStores = 2, 2
			}
			if len(dictionaries) != wantDictionaries || len(stores) != wantStores || consumer != len(original)-2 {
				t.Fatal("actual direct Map source inventory differs", dictionaries, stores, consumer)
			}
			faults := []string{"consumer_reads_other", "producer_kind", "extra_action", "missing_main_boundary", "terminal_input"}
			switch route {
			case "immutable":
				faults = append(faults, "consumer_inlines_dictionary")
			case "mutable":
				faults = append(faults, "store_reads_foreign", "store_name_changed")
			case "assignment":
				faults = append(faults, "replacement_reads_stale_map", "initial_store_reads_future", "duplicate_producer_identity")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					broken := p15C01Clone(t, original)
					if err := p15L01DirectMapGraphError(broken, route); err != nil {
						t.Fatal("unchanged actual graph clone rejected before mutation", err)
					}
					beforeMutation := p15C01Clone(t, broken)
					lastDictionary := dictionaries[len(dictionaries)-1]
					wantError := "complete action/parameter/edge mismatch"
					switch fault {
					case "consumer_reads_other":
						p15Param(broken[consumer])["WFVariable"] = p15FunctionVariable("unrelated")
					case "producer_kind":
						broken[lastDictionary]["WFWorkflowActionIdentifier"] = "is.workflow.actions.gettext"
					case "extra_action":
						broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
						wantError = "complete graph length"
					case "missing_main_boundary":
						broken = broken[:len(broken)-1]
						wantError = "missing native action"
					case "terminal_input":
						broken[len(broken)-1]["WFWorkflowActionParameters"] = map[string]any{"WFInput": "UNAUTHORED_INPUT"}
					case "consumer_inlines_dictionary":
						p15Param(broken[consumer])["WFVariable"] = p15Param(broken[lastDictionary])["WFItems"]
					case "store_reads_foreign":
						p15Param(broken[stores[0]])["WFInput"] = p15FunctionEdge("MISSING_MAP_PRODUCER")
					case "store_name_changed":
						p15Param(broken[stores[0]])["WFVariableName"] = "unrelated"
					case "replacement_reads_stale_map":
						old, ok := p15Param(broken[dictionaries[0]])["UUID"].(string)
						if !ok || old == "" {
							t.Fatal("actual initial Map identity missing")
						}
						p15Param(broken[stores[1]])["WFInput"] = p15FunctionEdge(old)
					case "initial_store_reads_future":
						next, ok := p15Param(broken[lastDictionary])["UUID"].(string)
						if !ok || next == "" {
							t.Fatal("actual replacement Map identity missing")
						}
						p15Param(broken[stores[0]])["WFInput"] = p15FunctionEdge(next)
					case "duplicate_producer_identity":
						old, ok := p15Param(broken[dictionaries[0]])["UUID"].(string)
						if !ok || old == "" {
							t.Fatal("actual initial Map identity missing")
						}
						p15Param(broken[lastDictionary])["UUID"] = old
						p15Param(broken[stores[1]])["WFInput"] = p15FunctionEdge(old)
						wantError = "duplicate producer identity"
					}
					if reflect.DeepEqual(broken, beforeMutation) {
						t.Fatal("causal mutation changed no field", fault)
					}
					if err := p15L01DirectMapGraphError(broken, route); err == nil || !strings.Contains(err.Error(), wantError) {
						t.Fatal("direct Map mutation did not reach its intended rejection", fault, wantError, err)
					}
				})
			}
		})
	}
}
