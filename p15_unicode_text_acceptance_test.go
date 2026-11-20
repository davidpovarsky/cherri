package main

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15UnicodeTextCase struct {
	name, interpolation, text string
	literalPrefix             string
	offsets                   []int
}

func p15UnicodeTextCases() []p15UnicodeTextCase {
	return []p15UnicodeTextCase{
		{
			name:          "hebrew_combining_zwj_and_repeated_producer",
			interpolation: "שָׁלוֹם👩‍💻é|{first}|אב{mutable}|é{first}|🌍{שֵׁם}\n",
			text:          "שָׁלוֹם👩‍💻é|\uFFFC|אב\uFFFC|é\uFFFC|🌍\uFFFC\n",
			// Independently counted UTF-16 code units, not byte or rune indexes.
			offsets: []int{15, 19, 22, 26},
		},
		{
			name:          "literal_object_replacement_and_surrogate_pair",
			interpolation: "\uFFFC🙂{first}{mutable}é{first}שָׁ{שֵׁם}",
			text:          "\uFFFC\uFFFC\uFFFCé\uFFFCשָׁ\uFFFC",
			literalPrefix: "\uFFFC🙂",
			// The authored prefix stays in a plain Text producer. Its single
			// attachment occupies one UTF-16 unit in the native token template.
			offsets: []int{0, 1, 2, 5, 9},
		},
	}
}

func p15UnicodeTextSource(tc p15UnicodeTextCase, route string) string {
	prelude := "let first=text(\"α\")\nvar mutable=text(\"ב\")\nlet שֵׁם=text(\"🌍\")\n"
	value := "f" + strconv.Quote(tc.interpolation)
	switch route {
	case "argument":
		return prelude + "text(" + value + ")\n"
	case "immutable":
		return prelude + "let message=" + value + "\ngetVariable(message)\n"
	case "mutable":
		return prelude + "var message=" + value + "\ngetVariable(message)\n"
	case "assignment":
		return prelude + "var message=\"old\"\nmessage=" + value + "\ngetVariable(message)\n"
	case "list":
		return prelude + "let message=[" + value + "]\ngetVariable(message)\n"
	case "dictionary":
		return prelude + "let message={\"payload\":" + value + "}\ngetVariable(message)\n"
	default:
		panic("unknown synthetic Unicode route")
	}
}

func p15UnicodeTextValue(actions []map[string]any, route string) (any, error) {
	var values []any
	for _, action := range actions {
		params := p15Param(action)
		switch {
		case route == "list" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.list":
			rows, ok := params["WFItems"].([]any)
			if !ok || len(rows) != 1 {
				return nil, fmt.Errorf("Unicode List lost its exact one field")
			}
			row, ok := rows[0].(map[string]any)
			if !ok || p15Int(row["WFItemType"]) != 0 {
				return nil, fmt.Errorf("Unicode List field type changed")
			}
			values = append(values, row["WFValue"])
		case route == "dictionary" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary":
			outer, _ := params["WFItems"].(map[string]any)
			body, _ := outer["Value"].(map[string]any)
			rows, ok := body["WFDictionaryFieldValueItems"].([]any)
			if !ok || len(rows) != 1 {
				return nil, fmt.Errorf("Unicode Dictionary lost its exact one field")
			}
			row, _ := rows[0].(map[string]any)
			key, err := p15LiteralText(row["WFKey"])
			if err != nil || key != "payload" || p15Int(row["WFItemType"]) != 0 {
				return nil, fmt.Errorf("Unicode Dictionary field changed")
			}
			values = append(values, row["WFValue"])
		case route != "list" && route != "dictionary" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext":
			if value, ok := params["WFTextActionText"].(map[string]any); ok && value["WFSerializationType"] == "WFTextTokenString" {
				values = append(values, value)
			}
		}
	}
	if len(values) != 1 {
		return nil, fmt.Errorf("expected one native interpolated value, got %d", len(values))
	}
	return values[0], nil
}

func p15UnicodeExpectedRefs(actions []map[string]any, tc p15UnicodeTextCase) ([]map[string]any, error) {
	seeds := map[string]map[string]any{}
	for _, action := range actions {
		params := p15Param(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
			if text, ok := params["WFTextActionText"].(string); ok && (text == "α" || text == "ב" || text == "🌍") {
				if seeds[text] != nil {
					return nil, fmt.Errorf("duplicate Unicode seed %q", text)
				}
				seeds[text] = params
			}
		}
	}
	for _, text := range []string{"α", "ב", "🌍"} {
		if uuid, _ := seeds[text]["UUID"].(string); uuid == "" {
			return nil, fmt.Errorf("Unicode seed %q has no exact producer", text)
		}
	}
	stores := 0
	for _, action := range actions {
		params := p15Param(action)
		if params["WFVariableName"] == "mutable" {
			stores++
			ref := p15DiagnosticValue(params["WFInput"])
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.setvariable" ||
				ref["Type"] != "ActionOutput" || ref["OutputUUID"] != seeds["ב"]["UUID"] {
				return nil, fmt.Errorf("mutable Unicode seed was not captured from its producer")
			}
		}
	}
	if stores != 1 {
		return nil, fmt.Errorf("mutable Unicode seed has %d assignments", stores)
	}
	first := map[string]any{"Type": "ActionOutput", "OutputUUID": seeds["α"]["UUID"], "OutputName": "first"}
	refs := []map[string]any{
		first,
		{"Type": "Variable", "VariableName": "mutable"},
		first,
		{"Type": "ActionOutput", "OutputUUID": seeds["🌍"]["UUID"], "OutputName": "שֵׁם"},
	}
	if tc.literalPrefix != "" {
		carrier, err := p15S01LiteralCarrierReference(actions, tc.literalPrefix)
		if err != nil {
			return nil, err
		}
		refs = append([]map[string]any{carrier}, refs...)
	}
	return refs, nil
}

// The oracle reads the emitted native object, checks exact UTF-8 literal bytes
// and independent numeric UTF-16 ranges, and binds each range to its real source.
// It does not call the production attachment encoder or normalize text.
func p15UnicodeTextError(value any, tc p15UnicodeTextCase, refs []map[string]any) error {
	wrapper, ok := value.(map[string]any)
	if !ok || wrapper["WFSerializationType"] != "WFTextTokenString" {
		return fmt.Errorf("missing native token string")
	}
	body, ok := wrapper["Value"].(map[string]any)
	if !ok || body["string"] != tc.text {
		return fmt.Errorf("Unicode literal bytes or normalization changed: %#v", body["string"])
	}
	attachments, ok := body["attachmentsByRange"].(map[string]any)
	if !ok || len(attachments) != len(tc.offsets) || len(refs) != len(tc.offsets) {
		return fmt.Errorf("native attachment cardinality changed")
	}
	for i, offset := range tc.offsets {
		key := fmt.Sprintf("{%d, 1}", offset)
		actual, ok := attachments[key].(map[string]any)
		if !ok || !reflect.DeepEqual(actual, refs[i]) {
			return fmt.Errorf("UTF-16 range %s has wrong or missing original producer: %#v", key, actual)
		}
	}
	return nil
}

func TestP15S01ExactUTF16TokensAndLiteralBytes(t *testing.T) {
	for _, tc := range p15UnicodeTextCases() {
		for _, route := range []string{"argument", "immutable", "mutable", "assignment", "list", "dictionary"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				actions := p15CompiledActions(t, p15UnicodeTextSource(tc, route))
				value, err := p15UnicodeTextValue(actions, route)
				if err != nil {
					t.Fatal(err)
				}
				refs, err := p15UnicodeExpectedRefs(actions, tc)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15UnicodeTextError(value, tc, refs); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15S01UnicodeOracleRejectsCorruption(t *testing.T) {
	tc := p15UnicodeTextCases()[0]
	actions := p15CompiledActions(t, p15UnicodeTextSource(tc, "immutable"))
	value, err := p15UnicodeTextValue(actions, "immutable")
	if err != nil {
		t.Fatal(err)
	}
	refs, err := p15UnicodeExpectedRefs(actions, tc)
	if err != nil {
		t.Fatal(err)
	}
	if err := p15UnicodeTextError(value, tc, refs); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"rune_offsets", "byte_offsets", "shift_one", "normalize_combining", "drop_hebrew_marks", "drop_zwj", "missing_token", "swapped_producer", "wrong_variable", "duplicate_range_length"} {
		t.Run(fault, func(t *testing.T) {
			mutated := p15LiteralClone(t, value).(map[string]any)
			body := mutated["Value"].(map[string]any)
			attachments := body["attachmentsByRange"].(map[string]any)
			first, second := "{15, 1}", "{19, 1}"
			switch fault {
			case "rune_offsets":
				attachments["{13, 1}"] = attachments[first]
				delete(attachments, first)
			case "byte_offsets":
				attachments["{29, 1}"] = attachments[first]
				delete(attachments, first)
			case "shift_one":
				attachments["{16, 1}"] = attachments[first]
				delete(attachments, first)
			case "normalize_combining":
				body["string"] = strings.ReplaceAll(tc.text, "é", "é")
			case "drop_hebrew_marks":
				body["string"] = strings.ReplaceAll(tc.text, "שָׁלוֹם", "שלום")
			case "drop_zwj":
				body["string"] = strings.ReplaceAll(tc.text, "\u200d", "")
			case "missing_token":
				delete(attachments, first)
			case "swapped_producer":
				attachments[first], attachments[second] = attachments[second], attachments[first]
			case "wrong_variable":
				attachments[second].(map[string]any)["VariableName"] = "first"
			case "duplicate_range_length":
				attachments["{15, 2}"] = attachments[first]
				delete(attachments, first)
			}
			if err := p15UnicodeTextError(mutated, tc, refs); err == nil {
				t.Fatal("accepted Unicode corruption " + fault)
			}
		})
	}
}

func TestP15S01UnicodeNativeRoundTrip(t *testing.T) {
	for _, tc := range p15UnicodeTextCases() {
		for _, route := range []string{"immutable", "dictionary", "list"} {
			root := p15MetadataNative(t, "s01-"+tc.name+"-"+route, p15UnicodeTextSource(tc, route))
			rawActions := root["WFWorkflowActions"].([]any)
			actions := make([]map[string]any, len(rawActions))
			for i, a := range rawActions {
				actions[i] = a.(map[string]any)
			}
			value, err := p15UnicodeTextValue(actions, route)
			if err != nil {
				t.Fatal(err)
			}
			refs, err := p15UnicodeExpectedRefs(actions, tc)
			if err != nil {
				t.Fatal(err)
			}
			if err := p15UnicodeTextError(value, tc, refs); err != nil {
				t.Fatal(err)
			}
			for _, obs := range p15NativeImportRoundTrip(t, "s01-"+tc.name+"-"+route, root, plist.XMLFormat, plist.BinaryFormat) {
				raw := obs.compiled["WFWorkflowActions"].([]any)
				out := make([]map[string]any, len(raw))
				for i, a := range raw {
					out[i] = a.(map[string]any)
				}
				actual, err := p15UnicodeTextValue(out, route)
				if err != nil {
					t.Fatal(err)
				}
				if err := p15UnicodeTextError(actual, tc, refs); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
