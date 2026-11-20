package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15FilterCase struct {
	name, filter, compound string
	templates              []map[string]any
}

func p15FilterCases() []p15FilterCase {
	return []p15FilterCase{
		{"compound_map", `{"compound":"or","predicates":[{"property":"Name","operator":"contains","value":"{owner}\\path"},{"property":"File Size","operator":"equals","value":0}]}`, "1",
			[]map[string]any{{"Property": "Name", "Operator": 99, "Values": map[string]any{"String": `{owner}\path`}, "Removable": true}, {"Property": "File Size", "Operator": 4, "Values": map[string]any{"Number": "0"}, "Removable": true}}},
		{"predicate_array", `[{"property":"Name","operator":"equals","value":"plain"}]`, "0",
			[]map[string]any{{"Property": "Name", "Operator": 4, "Values": map[string]any{"String": "plain"}, "Removable": true}}},
		{"shorthand_map", `{"Name":"{owner}","File Size":0}`, "0",
			[]map[string]any{{"Property": "File Size", "Operator": 4, "Values": map[string]any{"Number": "0"}, "Removable": true}, {"Property": "Name", "Operator": 4, "Values": map[string]any{"String": "{owner}"}, "Removable": true}}},
		{"empty_map", `{}`, "0", []map[string]any{}},
		{"empty_array", `[]`, "0", []map[string]any{}},
	}
}

func p15FilterSource(filter string) string {
	return "let input=text(\"P15_FILTER_INPUT\")\nfilterFiles(input, filter: " + filter + ")\n"
}

func p15CompileFilterNative(source string) (root map[string]any, err error) {
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("production filter panic: %v", p)
		}
	}()
	data, err := CompileSourceToPlist("p15-filter-metadata.cherri", source)
	if err != nil {
		return nil, err
	}
	_, err = plist.Unmarshal(data, &root)
	return root, err
}

func p15FilterNative(t *testing.T, source string) map[string]any {
	t.Helper()
	root, err := p15CompileFilterNative(source)
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func p15FilterParams(root map[string]any, id string) (map[string]any, error) {
	raw, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing native actions")
	}
	var params map[string]any
	for _, a := range raw {
		action := a.(map[string]any)
		switch action["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.dictionary", "is.workflow.actions.list":
			return nil, fmt.Errorf("compile-time filter data emitted a runtime collection action")
		}
		if action["WFWorkflowActionIdentifier"] == id {
			if params != nil {
				return nil, fmt.Errorf("duplicate filter action")
			}
			params = p15Param(action)
		}
	}
	if params == nil {
		return nil, fmt.Errorf("missing filter action")
	}
	return params, nil
}

func p15FilterTableError(root map[string]any, tc p15FilterCase) error {
	params, err := p15FilterParams(root, "is.workflow.actions.filter.files")
	if err != nil {
		return err
	}
	if params["WFCompoundType"] != tc.compound {
		return fmt.Errorf("compound semantics changed")
	}
	raw := root["WFWorkflowActions"].([]any)
	if err := p15M02MainVoidRowsError(raw); err != nil {
		return err
	}
	if len(raw) != 3 || raw[0].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
		raw[1].(map[string]any)["WFWorkflowActionIdentifier"] != "is.workflow.actions.filter.files" {
		return fmt.Errorf("filter input producer changed")
	}
	origin := p15Param(raw[0].(map[string]any))["UUID"]
	ref := p15DiagnosticValue(params["WFContentItemInputParameter"])
	if ref["Type"] != "ActionOutput" || ref["OutputUUID"] != origin {
		return fmt.Errorf("filter input identity changed")
	}
	table, ok := params["WFContentItemFilter"].(map[string]any)
	if !ok || table["WFSerializationType"] != "WFContentPredicateTableTemplate" {
		return fmt.Errorf("missing predicate table encoding")
	}
	value, ok := table["Value"].(map[string]any)
	if !ok || p15Int(value["WFActionParameterFilterPrefix"]) != 1 || value["WFContentPredicateBoundedDate"] != false {
		return fmt.Errorf("table metadata changed")
	}
	templates, ok := value["WFActionParameterFilterTemplates"].([]any)
	if !ok || len(templates) != len(tc.templates) {
		return fmt.Errorf("predicate count changed")
	}
	for i, item := range templates {
		got, ok := item.(map[string]any)
		want := tc.templates[i]
		if !ok || len(got) != len(want) || got["Property"] != want["Property"] || p15Int(got["Operator"]) != want["Operator"] ||
			got["Removable"] != want["Removable"] || !reflect.DeepEqual(got["Values"], want["Values"]) {
			return fmt.Errorf("predicate %d changed: %#v", i, got)
		}
	}
	return nil
}

func TestP15FilterCanonicalMetadataTransport(t *testing.T) {
	for _, tc := range p15FilterCases() {
		t.Run(tc.name, func(t *testing.T) {
			root := p15FilterNative(t, p15FilterSource(tc.filter))
			if err := p15FilterTableError(root, tc); err != nil {
				t.Fatal(err)
			}
		})
	}
	root := p15FilterNative(t, `filterReminders(filter: {"predicates":[{"property":"Is Completed","operator":"equals","value":false}]})`)
	params, err := p15FilterParams(root, "is.workflow.actions.filter.reminders")
	if err != nil {
		t.Fatal(err)
	}
	table := params["WFContentItemFilter"].(map[string]any)["Value"].(map[string]any)
	template := table["WFActionParameterFilterTemplates"].([]any)[0].(map[string]any)
	if !reflect.DeepEqual(template["Values"], map[string]any{"Bool": false}) {
		t.Fatalf("false predicate became text or disappeared: %#v", template)
	}
}

func TestP15FilterMetadataMalformedFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, filter, detail string }{
		{"bad_predicate_list", `{"predicates":"invalid"}`, "invalid predicates specification"},
		{"bad_predicate_item", `{"predicates":[1]}`, "invalid filter predicate item"},
		{"unknown_operator", `{"predicates":[{"property":"Name","operator":"nonsense","value":"x"}]}`, "unrecognized filter operator"},
		{"nested_compound", `{"predicates":[{"compound":"or","predicates":[]}]}`, "nested compound filters"},
		{"missing_property", `{"predicates":[{"operator":"equals","value":"x"}]}`, "missing required 'property'"},
		{"short_between", `{"predicates":[{"property":"File Size","operator":"between","value":[1]}]}`, "requires exactly two bounds"},
		{"serialized_bad_value", `{"WFSerializationType":"WFContentPredicateTableTemplate","Value":"invalid"}`, "Value must be a map"},
		{"serialized_missing_templates", `{"WFSerializationType":"WFContentPredicateTableTemplate","Value":{}}`, "missing WFActionParameterFilterTemplates"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := p15CompileFilterNative(p15FilterSource(tc.filter))
			if err == nil || !strings.Contains(err.Error(), tc.detail) {
				t.Fatalf("malformed filter did not fail through existing validator: %v", err)
			}
		})
	}
}

const p15SerializedFilter = `{"WFSerializationType":"WFContentPredicateTableTemplate","WFCompoundType":"1","P15Unknown":[false,0,""],"Value":{"WFActionParameterFilterPrefix":7,"WFContentPredicateBoundedDate":true,"WFActionParameterFilterTemplates":[{"Property":"Name","Operator":4,"Values":{"String":"{literal}"},"Removable":false,"P15Nested":{"WFItemType":"ordinary","WFSerializationType":"ordinary","Value":[0,false,""]}}]}}`

func p15ExpectedSerializedFilter() map[string]any {
	return map[string]any{
		"WFSerializationType": "WFContentPredicateTableTemplate", "WFCompoundType": "1", "P15Unknown": []any{false, int64(0), ""},
		"Value": map[string]any{
			"WFActionParameterFilterPrefix": int64(7), "WFContentPredicateBoundedDate": true,
			"WFActionParameterFilterTemplates": []any{map[string]any{
				"Property": "Name", "Operator": int64(4), "Values": map[string]any{"String": "{literal}"}, "Removable": false,
				"P15Nested": map[string]any{"WFItemType": "ordinary", "WFSerializationType": "ordinary", "Value": []any{int64(0), false, ""}},
			}},
		},
	}
}

// Legacy's existing JSON parser stores numeric metadata as float64. The
// retained pre-fix CLI independently confirms these four exact real values.
// Keep that wire representation distinct from the V2 integer expectation.
func p15ExpectedLegacySerializedFilter() map[string]any {
	table := p15ExpectedSerializedFilter()
	table["P15Unknown"].([]any)[1] = float64(0)
	value := table["Value"].(map[string]any)
	value["WFActionParameterFilterPrefix"] = float64(7)
	row := value["WFActionParameterFilterTemplates"].([]any)[0].(map[string]any)
	row["Operator"] = float64(4)
	row["P15Nested"].(map[string]any)["Value"].([]any)[0] = float64(0)
	return table
}

func TestP15FilterSerializedNativeMetadataPreserved(t *testing.T) {
	root := p15FilterNative(t, p15FilterSource(p15SerializedFilter))
	params, err := p15FilterParams(root, "is.workflow.actions.filter.files")
	if err != nil {
		t.Fatal(err)
	}
	want := p15LiteralClone(t, p15ExpectedSerializedFilter())
	if !reflect.DeepEqual(params["WFContentItemFilter"], want) || params["WFCompoundType"] != "1" {
		t.Fatalf("explicit native filter data changed: %#v", params)
	}
	// Use the actual Legacy compiler too. Its existing native metadata input
	// never goes through the new V2 parameter-aware bridge.
	legacy, err := compileLegacySourceToRawMapInMemory("const input = text(\"P15_FILTER_INPUT\")\nfilterFiles(input, " + p15SerializedFilter + ")\n")
	if err != nil {
		t.Fatal(err)
	}
	legacyParams, err := p15FilterParams(legacy, "is.workflow.actions.filter.files")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(legacyParams["WFContentItemFilter"], p15LiteralClone(t, p15ExpectedLegacySerializedFilter())) {
		t.Fatalf("Legacy native filter changed: %#v", legacyParams)
	}
}

func TestP15FilterMetadataOracleRejectsCorruption(t *testing.T) {
	tc := p15FilterCases()[0]
	base := p15FilterNative(t, p15FilterSource(tc.filter))
	for _, fault := range []string{"compound", "reorder", "zero_as_text", "literal_interpolated", "missing_predicate", "wrong_operator", "wrong_input", "runtime_dictionary"} {
		t.Run(fault, func(t *testing.T) {
			root := p15LiteralClone(t, base).(map[string]any)
			params, err := p15FilterParams(root, "is.workflow.actions.filter.files")
			if err != nil {
				t.Fatal(err)
			}
			value := params["WFContentItemFilter"].(map[string]any)["Value"].(map[string]any)
			rows := value["WFActionParameterFilterTemplates"].([]any)
			switch fault {
			case "compound":
				params["WFCompoundType"] = "0"
			case "reorder":
				rows[0], rows[1] = rows[1], rows[0]
			case "zero_as_text":
				rows[1].(map[string]any)["Values"] = map[string]any{"String": "0"}
			case "literal_interpolated":
				rows[0].(map[string]any)["Values"] = map[string]any{"String": "owner"}
			case "missing_predicate":
				value["WFActionParameterFilterTemplates"] = rows[:1]
			case "wrong_operator":
				rows[0].(map[string]any)["Operator"] = int64(4)
			case "wrong_input":
				params["WFContentItemInputParameter"] = p15AcceptanceRef("missing-input", nil)
			case "runtime_dictionary":
				root["WFWorkflowActions"] = append(root["WFWorkflowActions"].([]any), p15AcceptanceAction("is.workflow.actions.dictionary", map[string]any{}))
			}
			if err := p15FilterTableError(root, tc); err == nil {
				t.Fatal("accepted " + fault)
			}
		})
	}
}

func TestP15FilterMetadataNativeRoundTrip(t *testing.T) {
	for _, filter := range []string{p15FilterCases()[0].filter, p15SerializedFilter} {
		root := p15FilterNative(t, p15FilterSource(filter))
		p15NativeImportRoundTrip(t, "filter-metadata", root, plist.XMLFormat, plist.BinaryFormat)
	}
}
