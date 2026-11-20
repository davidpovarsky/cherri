package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// The oracle comes from the real WFDateFieldParameterState A/B observation in
// run 44696174000. It must not accept a bare attachment merely because its
// inner reference happens to identify the correct Date producer.
func p15DateFieldReferenceError(value any, want map[string]any) error {
	var wrapper, ok = value.(map[string]any)
	if !ok || len(wrapper) != 2 || wrapper["WFSerializationType"] != "WFTextTokenString" {
		return fmt.Errorf("Date reference must use WFTextTokenString: %#v", value)
	}
	var text, textOK = wrapper["Value"].(map[string]any)
	if !textOK || len(text) != 2 || text["string"] != "\uFFFC" {
		return fmt.Errorf("Date reference needs one exact object placeholder: %#v", text)
	}
	var ranges, rangesOK = text["attachmentsByRange"].(map[string]any)
	if !rangesOK || len(ranges) != 1 || !reflect.DeepEqual(ranges["{0, 1}"], want) {
		return fmt.Errorf("Date placeholder must retain the exact reference: %#v", ranges)
	}
	return nil
}

func p15DateFieldActions(t *testing.T, source string, legacy bool) []map[string]any {
	t.Helper()
	var root, err = compileHybridSource(source, legacy)
	if err != nil {
		t.Fatalf("actual production compilation: %v", err)
	}
	var raw, ok = root["WFWorkflowActions"].([]any)
	if !ok || len(raw) == 0 {
		t.Fatal("actual compiled action list missing")
	}
	var actions = make([]map[string]any, len(raw))
	for i, value := range raw {
		actions[i] = value.(map[string]any)
	}
	return actions
}

func TestP15DateFieldCanonicalProductionReferences(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%v", legacy), func(t *testing.T) {
			var source = "let first = date(\"2001-02-03T04:05:06Z\")\n" +
				"let second = date(\"2001-02-03T04:06:06Z\")\n" +
				"getTimeBetweenDates(first, second, \"Seconds\")\n" +
				"getTimeBetweenDates(first, first, \"Minutes\")\n" +
				"formatTimestamp(first, dateFormat: \"Custom\", timeFormat: \"None\", customDateFormat: \"yyyy-MM-dd'T'HH:mm:ssXXXXX\", locale: \"en_US_POSIX\")\n"
			if legacy {
				source = "#include 'actions/calendar'\n" + strings.ReplaceAll(source, "let ", "const ")
				source = strings.NewReplacer("dateFormat: ", "", "timeFormat: ", "", "customDateFormat: ", "", "locale: ", "false, ").Replace(source)
			}
			var actions = p15DateFieldActions(t, source, legacy)
			wantCount := 5
			if !legacy {
				wantCount++
				if err := p15M02MainVoidBoundaryError(actions); err != nil {
					t.Fatal(err)
				}
			}
			if len(actions) != wantCount {
				t.Fatalf("unexpected producer/effect/main-boundary count: %d", len(actions))
			}
			var refs = make([]map[string]any, 2)
			for i, wantDate := range []string{"2001-02-03T04:05:06Z", "2001-02-03T04:06:06Z"} {
				var p = p15Param(actions[i])
				if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions.date" || p["WFDateActionMode"] != "Specified Date" || p["WFDateActionDate"] != wantDate || p["UUID"] == "" {
					t.Fatalf("independent Date producer %d changed: %#v", i, actions[i])
				}
				refs[i] = map[string]any{"Type": "ActionOutput", "OutputUUID": p["UUID"], "OutputName": p["CustomOutputName"]}
			}
			for i, unit := range []string{"Seconds", "Minutes"} {
				var p = p15Param(actions[i+2])
				if actions[i+2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettimebetweendates" || p["WFTimeUntilUnit"] != unit {
					t.Fatalf("interval identity/units changed: %#v", actions[i+2])
				}
				var end = refs[1-i]
				for key, want := range map[string]map[string]any{"WFTimeUntilFromDate": refs[0], "WFInput": end} {
					if err := p15DateFieldReferenceError(p[key], want); err != nil {
						t.Errorf("interval %d %s: %v", i, key, err)
					}
				}
			}
			var format = p15Param(actions[4])
			if actions[4]["WFWorkflowActionIdentifier"] != "is.workflow.actions.format.date" || format["WFDateFormat"] != "yyyy-MM-dd'T'HH:mm:ssXXXXX" || format["WFLocale"] != "en_US_POSIX" || format["WFDateFormatStyle"] != "Custom" || format["WFTimeFormatStyle"] != "None" {
				t.Fatalf("unchanged independent format control: %#v", actions[4])
			}
			if err := p15DateFieldReferenceError(format["WFDate"], refs[0]); err != nil {
				t.Errorf("existing positive format control: %v", err)
			}
		})
	}
}

func TestP15DateFieldCanonicalMutableAndOmitted(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		t.Run(fmt.Sprintf("legacy_%v", legacy), func(t *testing.T) {
			var source = "var selected = date(\"2001-02-03T04:05:06Z\")\ngetTimeBetweenDates(selected, selected, \"Seconds\")\ngetTimeBetweenDates()\n"
			if legacy {
				source = "#include 'actions/calendar'\n@selected = date(\"2001-02-03T04:05:06Z\")\ngetTimeBetweenDates(@selected, @selected, \"Seconds\")\ngetTimeBetweenDates()\n"
			}
			var actions = p15DateFieldActions(t, source, legacy)
			wantCount := 4
			if !legacy {
				wantCount++
				if err := p15M02MainVoidBoundaryError(actions); err != nil {
					t.Fatal(err)
				}
			}
			if len(actions) != wantCount || actions[1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p15Param(actions[1])["WFVariableName"] != "selected" {
				t.Fatalf("mutable Date producer/store changed: %#v", actions)
			}
			var want = map[string]any{"Type": "Variable", "VariableName": "selected"}
			for _, key := range []string{"WFTimeUntilFromDate", "WFInput"} {
				if err := p15DateFieldReferenceError(p15Param(actions[2])[key], want); err != nil {
					t.Errorf("mutable %s: %v", key, err)
				}
				if _, present := p15Param(actions[3])[key]; present {
					t.Errorf("omitted Date field %s was invented", key)
				}
			}
		})
	}
}

func TestP15DateFieldNativeOracleRejectsCausalCorruption(t *testing.T) {
	var want = map[string]any{"Type": "ActionOutput", "OutputUUID": "10000000-0000-4000-8000-000000000001", "OutputName": "first"}
	var control = map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "\uFFFC", "attachmentsByRange": map[string]any{"{0, 1}": want}}}
	if err := p15DateFieldReferenceError(control, want); err != nil {
		t.Fatal(err)
	}
	var corruptions = map[string]func(map[string]any){
		"bare-attachment": func(m map[string]any) { m["WFSerializationType"] = "WFTextTokenAttachment"; m["Value"] = want },
		"literal-escape":  func(m map[string]any) { m["Value"].(map[string]any)["string"] = "\\uFFFC" },
		"missing-token":   func(m map[string]any) { m["Value"].(map[string]any)["attachmentsByRange"] = map[string]any{} },
		"wrong-utf16-range": func(m map[string]any) {
			m["Value"].(map[string]any)["attachmentsByRange"] = map[string]any{"{1, 1}": want}
		},
		"borrowed-date": func(m map[string]any) {
			m["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["OutputUUID"] = "10000000-0000-4000-8000-000000000002"
		},
		"borrowed-name": func(m map[string]any) {
			m["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["OutputName"] = "second"
		},
		"extra-state-field": func(m map[string]any) { m["Value"].(map[string]any)["extra"] = true },
	}
	for name, mutate := range corruptions {
		t.Run(name, func(t *testing.T) {
			var changed = deepClonePlistMap(control)
			mutate(changed)
			if err := p15DateFieldReferenceError(changed, want); err == nil {
				t.Fatal("causal Date reference corruption accepted")
			}
		})
	}
}

func p15DateFieldLiteral(value any) (string, bool) {
	if text, ok := value.(string); ok {
		return text, true
	}
	if wrapper, ok := value.(map[string]any); ok && wrapper["WFSerializationType"] == "WFTextTokenString" {
		if text, ok := wrapper["Value"].(map[string]any); ok {
			var ranges, _ = text["attachmentsByRange"].(map[string]any)
			var literal, literalOK = text["string"].(string)
			return literal, literalOK && len(ranges) == 0
		}
	}
	return "", false
}

func TestP15DateFieldLiteralAndOmittedSlots(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, omit := range []string{"neither", "first", "second"} {
			t.Run(fmt.Sprintf("legacy_%v/%s", legacy, omit), func(t *testing.T) {
				var first = "2001-02-03T04:05:06Z"
				var second = "2001-02-03T04:06:06Z"
				var source = fmt.Sprintf("getTimeBetweenDates(%q, %q, \"Seconds\")\n", first, second)
				if omit == "first" {
					source = fmt.Sprintf("getTimeBetweenDates(endDate: %q, unit: \"Seconds\")\n", second)
				} else if omit == "second" {
					source = fmt.Sprintf("getTimeBetweenDates(%q, unit: \"Seconds\")\n", first)
				}
				if legacy {
					if omit == "first" {
						source = fmt.Sprintf("getTimeBetweenDates(nil, %q, \"Seconds\")\n", second)
					} else if omit == "second" {
						source = fmt.Sprintf("getTimeBetweenDates(%q, nil, \"Seconds\")\n", first)
					}
					source = "#include 'actions/calendar'\n" + source
				} else if omit != "first" {
					// The existing V2 API requires a reference in the first slot.
					// Preserve that restriction while testing one-sided omission.
					source = fmt.Sprintf("let literalFirst = date(%q)\n", first) + strings.Replace(source, fmt.Sprintf("%q", first), "literalFirst", 1)
				}
				var actions = p15DateFieldActions(t, source, legacy)
				intervalIndex := len(actions) - 1
				if !legacy {
					if err := p15M02MainVoidBoundaryError(actions); err != nil {
						t.Fatal(err)
					}
					intervalIndex--
				}
				var interval = actions[intervalIndex]
				var p = p15Param(interval)
				if interval["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettimebetweendates" || p["WFTimeUntilUnit"] != "Seconds" {
					t.Fatalf("interval or units changed: %#v", interval)
				}
				for i, key := range []string{"WFTimeUntilFromDate", "WFInput"} {
					var value, present = p[key]
					if omit == []string{"first", "second"}[i] {
						if present {
							t.Fatalf("omitted Date field %s was invented: %#v", key, value)
						}
						continue
					}
					var want = []string{first, second}[i]
					if literal, ok := p15DateFieldLiteral(value); ok && literal == want {
						continue
					}
					// Verify the real Date/Text producer rather than inventing an
					// expected UUID or permitting a borrowed field value.
					var matched bool
					for _, producer := range actions[:intervalIndex] {
						var parameters = p15Param(producer)
						var literal, ok = p15DateFieldLiteral(parameters["WFTextActionText"])
						var producerOK = producer["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && ok && literal == want
						if producer["WFWorkflowActionIdentifier"] == "is.workflow.actions.date" {
							producerOK = parameters["WFDateActionMode"] == "Specified Date" && parameters["WFDateActionDate"] == want
						}
						if !producerOK {
							continue
						}
						var token = map[string]any{"Type": "ActionOutput", "OutputUUID": parameters["UUID"], "OutputName": parameters["CustomOutputName"]}
						matched = p15DateFieldReferenceError(value, token) == nil
					}
					if !matched {
						t.Errorf("%s lost authored literal %q: %#v", key, want, value)
					}
				}
			})
		}
	}
}

func TestP15DateFieldPreservesV2FirstSlotReferenceRequirement(t *testing.T) {
	var _, err = CompileSourceToPlist("date-literal-restriction.cherri", "getTimeBetweenDates(\"2001-02-03T04:05:06Z\")\n")
	if err == nil || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") || !strings.Contains(err.Error(), "startDate") {
		t.Fatalf("existing first-slot reference restriction changed: %v", err)
	}
}

func TestP15DateFieldDoesNotWrapUnrelatedInputs(t *testing.T) {
	var actions = p15DateFieldActions(t, "let d = date(\"2001-02-03T04:05:06Z\")\ntypeOf(d)\ncount(d, \"Items\")\n", false)
	if len(actions) != 4 {
		t.Fatalf("unrelated observation/main-boundary graph changed: %#v", actions)
	}
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		t.Fatal(err)
	}
	for i, key := range []string{"WFInput", "Input"} {
		var value, ok = p15Param(actions[i+1])[key].(map[string]any)
		if !ok || value["WFSerializationType"] != "WFTextTokenAttachment" {
			t.Errorf("unrelated %s parameter was wrapped as Date text: %#v", key, value)
		}
	}
}
