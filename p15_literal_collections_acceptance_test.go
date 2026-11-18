package main

import (
	"bytes"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

// These readers examine the actual serialized plist. They do not call the
// compiler's string, dictionary, array or reference encoders.
func p15LiteralText(value any) (string, error) {
	if s, ok := value.(string); ok {
		return s, nil
	}
	token, ok := value.(map[string]any)
	if !ok || token["WFSerializationType"] != "WFTextTokenString" {
		return "", fmt.Errorf("literal text became %T %#v", value, value)
	}
	body, ok := token["Value"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("text body missing")
	}
	if attachments, exists := body["attachmentsByRange"]; exists {
		if m, ok := attachments.(map[string]any); !ok || len(m) != 0 {
			return "", fmt.Errorf("ordinary literal acquired native attachments: %#v", attachments)
		}
	}
	s, ok := body["string"].(string)
	if !ok {
		return "", fmt.Errorf("explicit text bytes missing")
	}
	return s, nil
}

func p15CheckLiteralText(value any, want string) error {
	got, err := p15LiteralText(value)
	if err != nil {
		return err
	}
	if !bytes.Equal([]byte(got), []byte(want)) {
		return fmt.Errorf("literal bytes changed: got %q, want %q", got, want)
	}
	return nil
}

type p15LiteralNumber string

func p15ReadDictionaryLiteral(value any) (map[string]any, error) {
	outer, ok := value.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("missing typed Dictionary: %#v", value)
	}
	body, ok := outer["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing Dictionary body")
	}
	rows, ok := body["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing explicit Dictionary rows: %#v", body)
	}
	result := make(map[string]any, len(rows))
	for i, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("Dictionary row %d is not a map", i)
		}
		key, err := p15LiteralText(row["WFKey"])
		if err != nil {
			return nil, fmt.Errorf("Dictionary key %d: %w", i, err)
		}
		if _, exists := result[key]; exists {
			return nil, fmt.Errorf("duplicate serialized Dictionary key %q", key)
		}
		decoded, err := p15ReadDictionaryRow(row)
		if err != nil {
			return nil, fmt.Errorf("Dictionary key %q: %w", key, err)
		}
		result[key] = decoded
	}
	return result, nil
}

func p15ReadDictionaryRow(row map[string]any) (any, error) {
	value, exists := row["WFValue"]
	if !exists {
		return nil, fmt.Errorf("typed field lost WFValue")
	}
	switch p15Int(row["WFItemType"]) {
	case 0:
		return p15LiteralText(value)
	case 1:
		return p15ReadDictionaryLiteral(value)
	case 2:
		array, ok := value.(map[string]any)
		if !ok || array["WFSerializationType"] != "WFArrayParameterState" {
			return nil, fmt.Errorf("missing typed array")
		}
		rows, ok := array["Value"].([]any)
		if !ok {
			return nil, fmt.Errorf("missing explicit array elements: %#v", array)
		}
		items := make([]any, len(rows))
		for i, raw := range rows {
			row, ok := raw.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("array item %d is not a typed field", i)
			}
			value, err := p15ReadDictionaryRow(row)
			if err != nil {
				return nil, fmt.Errorf("array item %d: %w", i, err)
			}
			items[i] = value
		}
		return items, nil
	case 3:
		number, err := p15LiteralText(value)
		if err != nil {
			return nil, err
		}
		return p15LiteralNumber(number), nil
	case 4:
		boolean, ok := value.(map[string]any)
		if !ok || boolean["WFSerializationType"] != "WFNumberSubstitutableState" {
			return nil, fmt.Errorf("missing typed Boolean")
		}
		actual, exists := boolean["Value"]
		b, ok := actual.(bool)
		if !exists || !ok {
			return nil, fmt.Errorf("explicit Boolean value missing")
		}
		return b, nil
	default:
		return nil, fmt.Errorf("unrecognized Dictionary field type %#v", row["WFItemType"])
	}
}

func TestP15S02ExactLiteralBytes(t *testing.T) {
	cases := []struct{ name, expression, want string }{
		{"raw_regex", `r"\b(?:foo){2,3}\s+\{name\}\\end"`, `\b(?:foo){2,3}\s+\{name\}\\end`},
		{"json", "", `{"name":"{name}","key":"C:\\temp\\file","zero":0,"flag":false}`},
		{"javascript", "", `const name = {value: "${name}"}; /\w{2,4}/g.test(name); // @mail #include`},
		{"css", "", `@media (width > 1px) { .x::after { content: "{name}"; --path: "C:\temp"; } }`},
		{"native_looking_braces", "", `{name} {@name} {name['key']} {name.text} {{name}} ${name}`},
		{"unicode_escaped_controls", "", "שָׁלוֹם👩‍💻e\u0301\r\n\t\"\\ {name} \uFFFC"},
	}
	for _, tc := range cases {
		expression := tc.expression
		if expression == "" {
			expression = strconv.Quote(tc.want)
		}
		for _, route := range []string{"action_argument", "immutable", "mutable", "list", "dictionary", "dictionary_mutable", "dictionary_assignment", "dictionary_return", "dictionary_yield", "dictionary_in_list"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				body := ""
				switch route {
				case "action_argument":
					body = "text(" + expression + ")"
				case "immutable":
					body = "let literal=" + expression + "\ngetVariable(literal)"
				case "mutable":
					body = "var literal=" + expression + "\ngetVariable(literal)"
				case "list":
					body = "let literal=[" + expression + "]\ngetVariable(literal)"
				case "dictionary":
					body = "let literal={\"payload\":" + expression + "}\ngetVariable(literal)"
				case "dictionary_mutable":
					body = "var literal={\"payload\":" + expression + "}\ngetVariable(literal)"
				case "dictionary_assignment":
					body = "var literal={}\nliteral={\"payload\":" + expression + "}\ngetVariable(literal)"
				case "dictionary_return":
					body = "return {\"payload\":" + expression + "}"
				case "dictionary_yield":
					body = "let literal=if true { yield {\"payload\":" + expression + "} } else { yield {\"payload\":" + expression + "} }\ngetVariable(literal)"
				case "dictionary_in_list":
					body = "let literal=[{\"payload\":" + expression + "}]\ngetVariable(literal)"
				}
				actions := p15CompiledActions(t, "let name=text(\"EXPANSION_MUST_NOT_HAPPEN\")\n"+body)
				var value any
				for _, action := range actions {
					params := p15Param(action)
					switch {
					case strings.Contains(route, "dictionary") && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary":
						dictionary, err := p15ReadDictionaryLiteral(params["WFItems"])
						if err != nil {
							t.Fatal(err)
						}
						value = dictionary["payload"]
					case route == "list" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.list":
						rows, ok := params["WFItems"].([]any)
						if !ok || len(rows) != 1 {
							t.Fatalf("literal List cardinality: %#v", params)
						}
						value = rows[0].(map[string]any)["WFValue"]
					case !strings.Contains(route, "dictionary") && route != "list" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext":
						value = params["WFTextActionText"]
					}
				}
				if err := p15CheckLiteralText(value, tc.want); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	t.Run("literal_dictionary_key", func(t *testing.T) {
		actions := p15CompiledActions(t, "let name=text(\"EXPANSION_MUST_NOT_HAPPEN\")\nlet data={\"{name}\":\"kept\"}\ngetVariable(data)")
		for _, action := range actions {
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
				dict, err := p15ReadDictionaryLiteral(p15Param(action)["WFItems"])
				if err != nil || !reflect.DeepEqual(dict, map[string]any{"{name}": "kept"}) {
					t.Fatalf("literal Dictionary key was interpreted: value=%#v error=%v", dict, err)
				}
				return
			}
		}
		t.Fatal("missing Dictionary producer")
	})
}

func p15L01ExpectedNestedDictionary() map[string]any {
	return map[string]any{
		"outer": []any{
			map[string]any{"row": []any{false, p15LiteralNumber("0"), "", []any{}, map[string]any{}}},
			map[string]any{"row": []any{p15LiteralNumber("1"), p15LiteralNumber("2"), p15LiteralNumber("3")}},
		},
		"emptyList": []any{},
		"emptyMap":  map[string]any{},
	}
}

const p15L01NestedDictionarySource = `{"outer":[{"row":[false,0,"",[],{}]},{"row":[1,2,3]}],"emptyList":[],"emptyMap":{}}`

func TestP15L01NestedDictionaryShape(t *testing.T) {
	for _, route := range []string{"immutable", "mutable", "assignment", "dictionary_in_list"} {
		t.Run(route, func(t *testing.T) {
			source := ""
			switch route {
			case "immutable":
				source = "let data=" + p15L01NestedDictionarySource
			case "mutable":
				source = "var data=" + p15L01NestedDictionarySource
			case "assignment":
				source = "var data={}\ndata=" + p15L01NestedDictionarySource
			case "dictionary_in_list":
				source = "let data=[" + p15L01NestedDictionarySource + "]"
			}
			actions := p15CompiledActions(t, source+"\ngetVariable(data)")
			var found map[string]any
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
					var err error
					found, err = p15ReadDictionaryLiteral(p15Param(action)["WFItems"])
					if err != nil {
						t.Fatal(err)
					}
				}
			}
			if !reflect.DeepEqual(found, p15L01ExpectedNestedDictionary()) {
				t.Fatalf("nested shape/cardinality/type/order changed:\n got %#v\nwant %#v", found, p15L01ExpectedNestedDictionary())
			}
		})
	}
}

func TestP15L01UnverifiedNativeNestedLiteralFailsClosed(t *testing.T) {
	for _, source := range []string{
		"let data=[[],[1,2],[[3],[]]]\ngetVariable(data)",
		"var data=[[1,2]]\ngetVariable(data)",
		"var data=[]\ndata=[[]]\ngetVariable(data)",
		"return [[1,2]]",
	} {
		data, err := CompileSourceToPlist("p15-nested-native-list.cherri", source)
		if err == nil || len(data) != 0 || !strings.Contains(err.Error(), "E_NESTED_NATIVE_LIST") {
			t.Fatalf("unverified nested arrays reached native text rows: bytes=%d error=%v", len(data), err)
		}
	}
	// Existing Dictionary array fields have an independently typed container
	// codec. This positive control does not certify native List<List> runtime.
	data, err := CompileSourceToPlist("p15-nested-dictionary-array.cherri", "let data={\"rows\":[[],[1,2],[[3],[]]]}\ngetVariable(data)")
	if err != nil || len(data) == 0 {
		t.Fatalf("typed Dictionary arrays were blocked: %v", err)
	}
}

func TestP15L03DuplicateAndInvalidIndexDiagnostics(t *testing.T) {
	cases := []struct{ name, source, code, text string }{
		{"duplicate", "let data={\"x\":1,\"x\":2}", "E_DUPLICATE_KEY", "duplicate key"},
		{"duplicate_raw_spelling", "let data={\"x\":1,r\"x\":2}", "E_DUPLICATE_KEY", "duplicate key"},
		{"duplicate_nested", "let data={\"outer\":[{\"x\":1,\"x\":2}]}", "E_DUPLICATE_KEY", "duplicate key"},
		{"list_text", "let data=[1,2]\nlet result=data[\"x\"]", analysis.CodeArgumentType, "index expression requires Number"},
		{"list_bool", "let data=[1,2]\nlet result=data[false]", analysis.CodeArgumentType, "index expression requires Number"},
		{"list_list", "let data=[1,2]\nlet result=data[[0]]", analysis.CodeArgumentType, "index expression requires Number"},
		{"list_map", "let data=[1,2]\nlet result=data[{\"x\":0}]", analysis.CodeArgumentType, "index expression requires Number"},
		{"list_dynamic_text", "let data=[1,2]\nlet key=text(\"x\")\nlet result=data[key]", analysis.CodeArgumentType, "index expression requires Number"},
		{"map_number", "let data={\"x\":1}\nlet result=data[0]", analysis.CodeArgumentType, "map index requires Text"},
		{"map_bool", "let data={\"x\":1}\nlet result=data[false]", analysis.CodeArgumentType, "map index requires Text"},
		{"map_list", "let data={\"x\":1}\nlet result=data[[0]]", analysis.CodeArgumentType, "map index requires Text"},
		{"map_map", "let data={\"x\":1}\nlet result=data[{\"x\":0}]", analysis.CodeArgumentType, "map index requires Text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var previous string
			for pass := 0; pass < 2; pass++ {
				data, err := CompileSourceToPlist("p15-l03-"+tc.name+".cherri", tc.source)
				if err == nil || len(data) != 0 || !strings.Contains(err.Error(), tc.code) || !strings.Contains(err.Error(), tc.text) {
					t.Fatalf("invalid source reached native output or wrong diagnostic: bytes=%d error=%v", len(data), err)
				}
				if pass > 0 && err.Error() != previous {
					t.Fatalf("nondeterministic diagnostic: %q vs %q", previous, err.Error())
				}
				previous = err.Error()
			}
			svc := service.NewService(nil)
			uri := "file:///p15-l03-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			diags, _ := svc.Analyze(uri)
			found := false
			for _, diagnostic := range diags {
				if diagnostic.Code == tc.code && diagnostic.Severity == analysis.SeverityError && strings.Contains(diagnostic.Message, tc.text) {
					found = true
				}
			}
			if !found {
				t.Fatalf("editor disagrees with invalid-source diagnostic: %#v", diags)
			}
		})
	}
	t.Run("valid_controls", func(t *testing.T) {
		for _, source := range []string{
			"let data=[1,2]\nlet result=data[0]\ngetVariable(result)",
			"let data=[1,2]\nlet index=number(1)\nlet result=data[index]\ngetVariable(result)",
			"let data={\"x\":1}\nlet result=data[\"x\"]\ngetVariable(result)",
			"let data={\"x\":1}\nlet key=text(\"x\")\nlet result=data[key]\ngetVariable(result)",
			"let data={\"a\":{\"x\":1},\"b\":{\"x\":2}}\ngetVariable(data)",
		} {
			data, err := CompileSourceToPlist("p15-l03-valid.cherri", source)
			var doc map[string]any
			if err != nil {
				t.Fatal(err)
			}
			if _, err := plist.Unmarshal(data, &doc); err != nil || len(doc) == 0 {
				t.Fatalf("valid control did not produce real plist: %v", err)
			}
		}
	})
}
