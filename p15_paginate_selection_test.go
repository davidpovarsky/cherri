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

func p15SelectionSource(kind, key string, legacy bool, cap int) string {
	var declaration = "let"
	if legacy {
		declaration = "const"
	}
	var args = fmt.Sprintf("%q, \"items\", \"next\", %d, 6, nil, \"url\", nil, ", "https://example.invalid/pages/one.json", cap)
	if key == "" {
		args += "nil"
	} else {
		args += fmt.Sprintf("%q", key)
	}
	args += fmt.Sprintf(", %q", kind)
	return declaration + " selected = paginate(" + args + ")\ngetVariable(selected)\n"
}

func p15SelectionRef(value any, kind string) (string, error) {
	var outer = debugParamMap(value)
	var ref = debugParamMap(outer["Value"])
	if outer["WFSerializationType"] != "WFTextTokenAttachment" || ref["Type"] != kind {
		return "", fmt.Errorf("not an exact %s attachment", kind)
	}
	var key = "OutputUUID"
	if kind == "Variable" {
		key = "VariableName"
	}
	var identity, ok = ref[key].(string)
	if !ok || identity == "" {
		return "", fmt.Errorf("missing concrete %s", key)
	}
	for field := range ref {
		if field != "Type" && field != key && !(kind == "ActionOutput" && field == "OutputName") {
			return "", fmt.Errorf("unexpected reference transformation %s", field)
		}
	}
	return identity, nil
}

func p15SelectionTextRef(value any) (string, error) {
	var outer = debugParamMap(value)
	var payload = debugParamMap(outer["Value"])
	var refs = debugParamMap(payload["attachmentsByRange"])
	if outer["WFSerializationType"] != "WFTextTokenString" || payload["string"] != "\ufffc" || len(refs) != 1 || len(payload) != 2 {
		return "", fmt.Errorf("type comparison is not the exact witness reference")
	}
	var ref = debugParamMap(refs["{0, 1}"])
	return p15SelectionRef(map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}, "ActionOutput")
}

func p15SelectionGuard(actions []ShortcutAction, at int, input string, code int, diagnostic string) (map[string]any, error) {
	if at < 0 || at+3 >= len(actions) {
		return nil, fmt.Errorf("guard is incomplete")
	}
	var p = actions[at].WFWorkflowActionParameters
	var wrapped = debugParamMap(p["WFInput"])
	var identity, e = p15SelectionRef(wrapped["Variable"], "ActionOutput")
	var group, okay = p["GroupingIdentifier"].(string)
	if actions[at].WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		fmt.Sprint(p["WFControlFlowMode"]) != "0" || fmt.Sprint(p["WFCondition"]) != fmt.Sprint(code) ||
		!okay || group == "" || wrapped["Type"] != "Variable" || len(wrapped) != 2 || e != nil || identity != input {
		return nil, fmt.Errorf("guard is detached or its predicate changed")
	}
	var alert = actions[at+1]
	if alert.WFWorkflowActionIdentifier != "is.workflow.actions.alert" ||
		alert.WFWorkflowActionParameters["WFAlertActionMessage"] != diagnostic ||
		alert.WFWorkflowActionParameters["WFAlertActionCancelButtonShown"] != false {
		return nil, fmt.Errorf("guard lost its fixed noncancellable diagnostic")
	}
	if actions[at+2].WFWorkflowActionIdentifier != "is.workflow.actions.exit" || len(actions[at+2].WFWorkflowActionParameters) != 0 {
		return nil, fmt.Errorf("guard no longer stops execution")
	}
	var end = actions[at+3]
	if end.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" ||
		end.WFWorkflowActionParameters["GroupingIdentifier"] != group || fmt.Sprint(end.WFWorkflowActionParameters["WFControlFlowMode"]) != "2" {
		return nil, fmt.Errorf("guard lost its exact control boundary")
	}
	return p, nil
}

func p15SelectionWitness(actions []ShortcutAction, uuid, kind string) error {
	var producer = func(identity string) *ShortcutAction {
		var found *ShortcutAction
		for index := range actions {
			if actions[index].WFWorkflowActionParameters["UUID"] == identity {
				if found != nil {
					return nil
				}
				found = &actions[index]
			}
		}
		return found
	}
	var witness = producer(uuid)
	if witness == nil || witness.WFWorkflowActionIdentifier != "is.workflow.actions.getitemtype" {
		return fmt.Errorf("expected runtime kind witness missing")
	}
	var sourceID, e = p15SelectionRef(witness.WFWorkflowActionParameters["WFInput"], "ActionOutput")
	if e != nil {
		return e
	}
	var source = producer(sourceID)
	if source == nil {
		return fmt.Errorf("witness has no source")
	}
	switch kind {
	case "text":
		if source.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" || source.WFWorkflowActionParameters["WFTextActionText"] != "https://example.invalid/pages/one.json" {
			return fmt.Errorf("Text witness is not an authored native Text")
		}
	case "number":
		if source.WFWorkflowActionIdentifier != "is.workflow.actions.number" || source.WFWorkflowActionParameters["WFNumberActionNumber"] != "1" {
			return fmt.Errorf("Number witness is not an authored native Number")
		}
	case "bool":
		if source.WFWorkflowActionIdentifier != "is.workflow.actions.getvalueforkey" || source.WFWorkflowActionParameters["WFDictionaryKey"] != "cherri_paginate_type_witness" || source.WFWorkflowActionParameters["WFGetDictionaryValueType"] != "Value" {
			return fmt.Errorf("Bool witness lost its exact authored Boolean field")
		}
		var parentID, e = p15SelectionRef(source.WFWorkflowActionParameters["WFInput"], "ActionOutput")
		if e != nil {
			return e
		}
		source = producer(parentID)
		fallthrough
	case "dictionary":
		if source == nil || source.WFWorkflowActionIdentifier != "is.workflow.actions.dictionary" {
			return fmt.Errorf("Dictionary/Bool witness has no authored Dictionary")
		}
		var outer = debugParamMap(source.WFWorkflowActionParameters["WFItems"])
		var payload = debugParamMap(outer["Value"])
		var raw, e = json.Marshal(payload["WFDictionaryFieldValueItems"])
		if e != nil {
			return e
		}
		var fields []map[string]any
		if e := json.Unmarshal(raw, &fields); e != nil || len(fields) != 1 {
			return fmt.Errorf("witness field structure changed")
		}
		if fmt.Sprint(fields[0]["WFItemType"]) != "4" || debugParamMap(fields[0]["WFValue"])["Value"] != true {
			return fmt.Errorf("Boolean witness became a Number or Text")
		}
	default:
		return fmt.Errorf("unreviewed item kind %s", kind)
	}
	return nil
}

func p15SelectionGraphError(actions []ShortcutAction, key, kind string) error {
	var resultName string
	var resultAppend = -1
	for index, action := range actions {
		var p = action.WFWorkflowActionParameters
		var name, _ = p["WFVariableName"].(string)
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" && strings.HasSuffix(name, "_results") {
			if resultName != "" {
				return fmt.Errorf("multiple result declarations")
			}
			resultName = name
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" && name == resultName && resultName != "" {
			if resultAppend != -1 {
				return fmt.Errorf("multiple result appends")
			}
			resultAppend = index
		}
	}
	if resultName == "" || resultAppend < 0 {
		return fmt.Errorf("missing result aggregate")
	}
	var selected, err = p15SelectionRef(actions[resultAppend].WFWorkflowActionParameters["WFInput"], "ActionOutput")
	if err != nil {
		return err
	}
	var selectedIndex = -1
	for index, action := range actions[:resultAppend] {
		if action.WFWorkflowActionParameters["UUID"] == selected {
			if selectedIndex != -1 {
				return fmt.Errorf("duplicate selected UUID")
			}
			selectedIndex = index
		}
	}
	if selectedIndex < 0 {
		return fmt.Errorf("append has no selected producer")
	}
	var source = actions[selectedIndex]
	if key == "" {
		var variable, e = p15SelectionRef(source.WFWorkflowActionParameters["WFVariable"], "Variable")
		if e != nil || source.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || variable != "Repeat Item" {
			return fmt.Errorf("original item was replaced or coerced")
		}
	} else {
		if source.WFWorkflowActionIdentifier != "is.workflow.actions.getvalueforkey" ||
			source.WFWorkflowActionParameters["WFDictionaryKey"] != key ||
			source.WFWorkflowActionParameters["WFGetDictionaryValueType"] != "Value" {
			return fmt.Errorf("selector lost exact key/value operation")
		}
		var input, e = p15SelectionRef(source.WFWorkflowActionParameters["WFInput"], "ActionOutput")
		if e != nil {
			return e
		}
		var found = false
		for _, prior := range actions[:selectedIndex] {
			if prior.WFWorkflowActionParameters["UUID"] != input {
				continue
			}
			var variable, e = p15SelectionRef(prior.WFWorkflowActionParameters["WFVariable"], "Variable")
			found = e == nil && variable == "Repeat Item" && prior.WFWorkflowActionIdentifier == "is.workflow.actions.getvariable"
		}
		if !found {
			return fmt.Errorf("selector is detached from original Dictionary item")
		}
		var guarded = false
		for _, prior := range actions[:selectedIndex] {
			if prior.WFWorkflowActionIdentifier == "is.workflow.actions.alert" &&
				prior.WFWorkflowActionParameters["WFAlertActionMessage"] == "PAGINATE_ITEM_KEY_REQUIRED" {
				guarded = true
			}
		}
		if !guarded {
			return fmt.Errorf("missing selector key guard")
		}
	}

	var diagnostic = "PAGINATE_ITEM_" + strings.ToUpper(kind) + "_REQUIRED"
	// The actual selected native value must flow through both independent
	// singleton and category guards, each with a terminating failure branch.
	if resultAppend-selectedIndex != 11 {
		return fmt.Errorf("selected boundary inserted or lost an operation")
	}
	var counted = actions[selectedIndex+1]
	var countID, e = p15SelectionRef(counted.WFWorkflowActionParameters["Input"], "ActionOutput")
	if e != nil || countID != selected || counted.WFWorkflowActionIdentifier != "is.workflow.actions.count" || counted.WFWorkflowActionParameters["WFCountType"] != "Items" {
		return fmt.Errorf("selected cardinality guard lost original input")
	}
	var countUUID, _ = counted.WFWorkflowActionParameters["UUID"].(string)
	var countGuard, e2 = p15SelectionGuard(actions, selectedIndex+2, countUUID, 5, diagnostic)
	if e2 != nil || fmt.Sprint(countGuard["WFNumberValue"]) != "1" {
		return fmt.Errorf("selected singleton guard changed: %v", e2)
	}
	var typed = actions[selectedIndex+6]
	var typedID, e3 = p15SelectionRef(typed.WFWorkflowActionParameters["WFInput"], "ActionOutput")
	if e3 != nil || typedID != selected || typed.WFWorkflowActionIdentifier != "is.workflow.actions.getitemtype" {
		return fmt.Errorf("selected category guard lost original input")
	}
	var typeUUID, _ = typed.WFWorkflowActionParameters["UUID"].(string)
	var typeGuard, e4 = p15SelectionGuard(actions, selectedIndex+7, typeUUID, 5, diagnostic)
	if e4 != nil {
		return e4
	}
	var witness, e5 = p15SelectionTextRef(typeGuard["WFConditionalActionString"])
	if e5 != nil {
		return e5
	}
	return p15SelectionWitness(actions[:selectedIndex], witness, kind)
}

func TestP15PaginateSelectionTypedGraphs(t *testing.T) {
	for _, kind := range []string{"dictionary", "text", "number", "bool"} {
		for _, key := range []string{"", "payload"} {
			for _, legacy := range []bool{false, true} {
				for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
					t.Run(fmt.Sprintf("%s/key=%s/legacy=%t/%s", kind, key, legacy, profile), func(t *testing.T) {
						var source = p15SelectionSource(kind, key, legacy, 3)
						var actions, err = compileBuildProfileFixture(t, source, legacy, profile)
						if err != nil {
							t.Fatal(err)
						}
						if err := p15SelectionGraphError(actions, key, kind); err != nil {
							t.Fatal(err)
						}
						if dir := os.Getenv("P15_SELECTION_PRODUCTS"); dir != "" {
							var raw, e = json.MarshalIndent(map[string]any{"test": t.Name(), "source": source, "legacy": legacy, "profile": profile, "actions": actions}, "", "  ")
							if e != nil {
								t.Fatal(e)
							}
							var name = strings.NewReplacer("/", "__", "=", "-").Replace(t.Name()) + ".json"
							if e := os.WriteFile(filepath.Join(dir, name), append(raw, '\n'), 0644); e != nil {
								t.Fatal(e)
							}
						}
					})
				}
			}
		}
	}
}

func TestP15PaginateSelectionDownstreamTypes(t *testing.T) {
	for _, row := range []struct{ kind, sourceType, wrong string }{
		{"dictionary", "Map<Text, AnyContent>", "Text"},
		{"text", "Text", "Number"},
		{"number", "Number", "Text"},
		{"bool", "Bool", "Text"},
	} {
		t.Run(row.kind, func(t *testing.T) {
			var call = fmt.Sprintf("paginate(\"https://example.invalid/p\", itemKey:\"payload\", itemType:%q)", row.kind)
			var source = fmt.Sprintf("let rows: List<%s> = %s\nfor row in rows { let checked: %s = row\ngetVariable(checked)\n}\n", row.sourceType, call, row.sourceType)
			if _, err := compileBuildProfileFixture(t, source, false, BuildProfileRelease); err != nil {
				t.Fatal(err)
			}
			var incorrect = fmt.Sprintf("let rows: List<%s> = %s\n", row.wrong, call)
			if _, err := compileBuildProfileFixture(t, incorrect, false, BuildProfileRelease); err == nil {
				t.Fatal("incompatible concrete downstream result type accepted")
			}
		})
	}
}

func TestP15PaginateSelectionDefaultSnapshot(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		var declaration = "let"
		if legacy {
			declaration = "const"
		}
		var source = declaration + " selected = paginate(\"https://example.invalid/pages/one.json\", \"items\", \"next\", 3, 6)\ngetVariable(selected)\n"
		var actions, err = compileBuildProfileFixture(t, source, legacy, BuildProfileRelease)
		if err != nil {
			t.Fatal(err)
		}
		var identity, e = p15PaginateDefaultGraphIdentity(actions)
		if e != nil {
			t.Fatal(e)
		}
		var raw, e2 = json.MarshalIndent(identity, "", "  ")
		if e2 != nil {
			t.Fatal(e2)
		}
		if dir := os.Getenv("P15_SELECTION_BASELINE"); dir != "" {
			var path = filepath.Join(dir, fmt.Sprintf("default-legacy-%t.json", legacy))
			if os.Getenv("P15_SELECTION_CAPTURE") == "1" {
				if e := os.WriteFile(path, append(raw, '\n'), 0644); e != nil {
					t.Fatal(e)
				}
			} else {
				var before, e = os.ReadFile(path)
				if e != nil {
					t.Fatal(e)
				}
				if !reflect.DeepEqual(append(raw, '\n'), before) {
					t.Fatal("default native graph differs from frozen pre-implementation source")
				}
			}
		}
	}
}

func TestP15PaginateSelectionZeroCaps(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, kind := range []string{"dictionary", "text", "number", "bool"} {
			t.Run(fmt.Sprintf("%s/legacy=%t", kind, legacy), func(t *testing.T) {
				var actions, err = compileBuildProfileFixture(t, p15SelectionSource(kind, "payload", legacy, 0), legacy, BuildProfileDebug)
				if err != nil {
					t.Fatal(err)
				}
				var want = []string{"list", "setvariable", "getvariable", "getvariable"}
				if !legacy {
					want = append(want, "nothing")
				}
				if got := actionSuffixes(actions); !reflect.DeepEqual(got, want) {
					t.Fatalf("zero cap performed work: %v", got)
				}
			})
		}
	}
}

func TestP15PaginateSelectionInvalidOptions(t *testing.T) {
	for _, options := range []string{
		"itemType:\"list\"", "itemType:\"optional\"", "itemType:\"null\"", "itemType:\"any\"",
		"itemType:\"\"", "itemType:\"Text\"", "itemKey:\"\"", "itemKey:\"a.b\"",
		"itemKey:\"bad/key\"", "itemType:kind", "itemKey:kind",
	} {
		t.Run(options, func(t *testing.T) {
			var source = "let kind=text(\"text\")\npaginate(\"https://example.invalid/p\", " + options + ")\n"
			if _, err := compileBuildProfileFixture(t, source, false, BuildProfileRelease); err == nil {
				t.Fatal("unsupported/dynamic selector accepted")
			}
		})
	}
}

func TestP15PaginateSelectionOracleControls(t *testing.T) {
	var actions, err = compileBuildProfileFixture(t, p15SelectionSource("text", "payload", false, 3), false, BuildProfileRelease)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15SelectionGraphError(actions, "payload", "text"); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"foreign_append", "changed_key", "missing_key_guard", "missing_type_guard", "missing_count_guard", "type_guard_predicate", "count_guard_predicate", "type_guard_exit", "count_guard_exit", "foreign_type_witness"} {
		t.Run(fault, func(t *testing.T) {
			var raw, e = json.Marshal(actions)
			if e != nil {
				t.Fatal(e)
			}
			var broken []ShortcutAction
			if e := json.Unmarshal(raw, &broken); e != nil {
				t.Fatal(e)
			}
			var changed = false
			for index := range broken {
				var action = &broken[index]
				var p = action.WFWorkflowActionParameters
				var id = action.WFWorkflowActionIdentifier
				var name, _ = p["WFVariableName"].(string)
				if id == "is.workflow.actions.alert" && p["WFAlertActionMessage"] == "PAGINATE_ITEM_TEXT_REQUIRED" && index > 0 {
					var prior = broken[index-1].WFWorkflowActionParameters
					_, typed := prior["WFConditionalActionString"]
					if (fault == "type_guard_predicate" && typed) || (fault == "count_guard_predicate" && !typed) {
						prior["WFCondition"] = 4
						changed = true
						break
					}
					if (fault == "type_guard_exit" && typed) || (fault == "count_guard_exit" && !typed) {
						broken[index+1].WFWorkflowActionIdentifier = "is.workflow.actions.nothing"
						changed = true
						break
					}
					if fault == "foreign_type_witness" && typed {
						prior["WFConditionalActionString"] = "Text"
						changed = true
						break
					}
				}
				if fault == "foreign_append" && id == "is.workflow.actions.appendvariable" && strings.HasSuffix(name, "_results") {
					p["WFInput"] = p13TokenAttachment("unrelated-selection", "Text")
					changed = true
					break
				}
				if fault == "changed_key" && id == "is.workflow.actions.getvalueforkey" && p["WFDictionaryKey"] == "payload" {
					p["WFDictionaryKey"] = "wrong"
					changed = true
					break
				}
				if fault == "missing_key_guard" && id == "is.workflow.actions.alert" && p["WFAlertActionMessage"] == "PAGINATE_ITEM_KEY_REQUIRED" {
					p["WFAlertActionMessage"] = "UNRELATED"
					changed = true
					break
				}
				if fault == "missing_type_guard" && id == "is.workflow.actions.getitemtype" {
					var selected = false
					var ref, e = p15SelectionRef(p["WFInput"], "ActionOutput")
					if e == nil {
						for _, prior := range broken[:index] {
							selected = selected || prior.WFWorkflowActionParameters["UUID"] == ref && prior.WFWorkflowActionIdentifier == "is.workflow.actions.getvalueforkey" && prior.WFWorkflowActionParameters["WFDictionaryKey"] == "payload"
						}
					}
					if selected {
						p["WFInput"] = p13TokenAttachment("unrelated-selection", "Text")
						changed = true
						break
					}
				}
				if fault == "missing_count_guard" && id == "is.workflow.actions.count" {
					var selected = false
					var ref, e = p15SelectionRef(p["Input"], "ActionOutput")
					if e == nil {
						for _, prior := range broken[:index] {
							selected = selected || prior.WFWorkflowActionParameters["UUID"] == ref && prior.WFWorkflowActionIdentifier == "is.workflow.actions.getvalueforkey" && prior.WFWorkflowActionParameters["WFDictionaryKey"] == "payload"
						}
					}
					if selected {
						p["Input"] = p13TokenAttachment("unrelated-selection", "Text")
						changed = true
						break
					}
				}
			}
			if !changed {
				t.Fatal("fault did not mutate actual selected path")
			}
			if p15SelectionGraphError(broken, "payload", "text") == nil {
				t.Fatal("mutated selector graph accepted")
			}
		})
	}
}

func TestP15PaginateSelectionCapabilitiesPrecedeRequests(t *testing.T) {
	for _, kind := range []string{"number", "bool"} {
		for _, legacy := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/legacy=%t", kind, legacy), func(t *testing.T) {
				var actions, err = compileBuildProfileFixture(t, p15SelectionSource(kind, "payload", legacy, 3), legacy, BuildProfileRelease)
				if err != nil {
					t.Fatal(err)
				}
				var boundary = len(actions)
				for index, action := range actions {
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.count" || action.WFWorkflowActionIdentifier == "is.workflow.actions.downloadurl" {
						boundary = index
						break
					}
				}
				var diagnostic = "PAGINATE_" + strings.ToUpper(kind) + "_TYPE_UNAVAILABLE"
				var seen = 0
				for index, action := range actions {
					if action.WFWorkflowActionIdentifier != "is.workflow.actions.alert" || action.WFWorkflowActionParameters["WFAlertActionMessage"] != diagnostic {
						continue
					}
					if index < 1 || index+2 >= boundary {
						t.Fatal("capability guard occurs after traversal begins")
					}
					var start = actions[index-1].WFWorkflowActionParameters
					var wrapped = debugParamMap(start["WFInput"])
					var input, e = p15SelectionRef(wrapped["Variable"], "ActionOutput")
					if e != nil {
						t.Fatal(e)
					}
					var code = 5
					if _, okay := start["WFConditionalActionString"]; okay {
						code = 4
					}
					if _, e := p15SelectionGuard(actions, index-1, input, code, diagnostic); e != nil {
						t.Fatal(e)
					}
					if code == 4 {
						if err := p15SelectionWitness(actions[:index-1], input, "bool"); err != nil {
							t.Fatal(err)
						}
						var comparison, e = p15SelectionTextRef(start["WFConditionalActionString"])
						if e != nil {
							t.Fatal(e)
						}
						if err := p15SelectionWitness(actions[:index-1], comparison, "number"); err != nil {
							t.Fatal(err)
						}
					} else if fmt.Sprint(start["WFNumberValue"]) != "1" {
						t.Fatal("Boolean witness singleton check changed")
					}
					seen++
				}
				if seen != 2 {
					t.Fatalf("expected singleton and alias capability guards, got %d", seen)
				}
			})
		}
	}
}

func TestP15PaginateSelectionRetainsPageCallback(t *testing.T) {
	for _, kind := range []string{"dictionary", "text", "number", "bool"} {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", kind, legacy, profile), func(t *testing.T) {
					var source = strings.Replace(p15SelectionSource(kind, "payload", legacy, 3), ")\ngetVariable", ") { page in wait(0) }\ngetVariable", 1)
					if !legacy {
						var element = map[string]string{"dictionary": "Map<Text, AnyContent>", "text": "Text", "number": "Number", "bool": "Bool"}[kind]
						source = strings.Replace(source, "let selected =", "let selected: List<"+element+"> =", 1)
					}
					var actions, err = compileBuildProfileFixture(t, source, legacy, profile)
					if err != nil {
						t.Fatal(err)
					}
					if err := p15SelectionGraphError(actions, "payload", kind); err != nil {
						t.Fatal(err)
					}
					var requests, callbacks, lookup = -1, 0, -1
					for index, action := range actions {
						switch action.WFWorkflowActionIdentifier {
						case "is.workflow.actions.downloadurl":
							requests = index
						case "is.workflow.actions.delay":
							callbacks++
							if requests < 0 || lookup < 0 {
								t.Fatal("page callback moved outside validated page/item order")
							}
						case "is.workflow.actions.getvalueforkey":
							if action.WFWorkflowActionParameters["WFDictionaryKey"] == "payload" {
								lookup = index
							}
						}
					}
					if callbacks != 1 || lookup < 0 {
						t.Fatal("typed selection lost its original page callback")
					}
				})
			}
		}
	}
}
