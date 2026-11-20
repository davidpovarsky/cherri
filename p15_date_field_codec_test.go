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
			if len(actions) != 5 {
				t.Fatalf("unexpected producer/effect count: %d", len(actions))
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
			if len(actions) != 4 || actions[1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" || p15Param(actions[1])["WFVariableName"] != "selected" {
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
