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
				actions := p15CompiledActions(t, p15S02LiteralRouteSource(route, expression))
				if strings.ContainsRune(tc.want, '\uFFFC') && strings.Contains(route, "dictionary") {
					if err := p15S02LiteralCarrierGraphError(actions, route, tc.want); err != nil {
						t.Fatal(err)
					}
					return
				}
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

// The payload reader independently checks typed Dictionary values. The complete
// graph then checks the authored outer List rows, producer references, stores
// and final consumer. Repeated aliases may share a producer; producer
// definitions themselves must remain unique. This is native-structure evidence,
// not Apple execution or a claim about unverified native List<List> shapes.
const p15L01DictionaryListTailSource = `{"tail":[3,2,1],"empty":[]}`

func p15L01DictionaryListSource(t *testing.T, route string) string {
	t.Helper()
	pair := "[" + p15L01NestedDictionarySource + "," + p15L01DictionaryListTailSource + "]"
	switch route {
	case "dictionary_in_list":
		return "let data=[" + p15L01NestedDictionarySource + "]\ngetVariable(data)"
	case "dictionary_in_list_ordered":
		return "let data=" + pair + "\ngetVariable(data)"
	case "dictionary_in_list_mutable":
		return "var data=" + pair + "\ngetVariable(data)"
	case "dictionary_in_list_assignment":
		return "var data=[" + p15L01DictionaryListTailSource + "," + p15L01NestedDictionarySource + "]\ndata=" + pair + "\ngetVariable(data)"
	case "dictionary_in_list_aliases":
		return "let first=" + p15L01NestedDictionarySource + "\nlet second=" + p15L01DictionaryListTailSource +
			"\nlet data=[first,second,first]\ngetVariable(data)"
	default:
		t.Fatalf("unknown Dictionary-in-List fixture %q", route)
		return ""
	}
}

func p15L01DictionaryListGraphError(actions []map[string]any, route string) error {
	switch route {
	case "dictionary_in_list", "dictionary_in_list_ordered", "dictionary_in_list_mutable",
		"dictionary_in_list_assignment", "dictionary_in_list_aliases":
	default:
		return fmt.Errorf("unknown Dictionary-in-List graph %q", route)
	}
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	dictionary := func(want map[string]any) string {
		index := len(g.expected)
		var items any
		if index < len(actions) {
			items = p15Param(actions[index])["WFItems"]
		}
		got, err := p15ReadDictionaryLiteral(items)
		if err != nil || !reflect.DeepEqual(got, want) {
			g.err = fmt.Errorf("Dictionary payload at %d changed: got %#v want %#v: %v", index, got, want, err)
		}
		// Map field order is not used as an oracle. Only the payload already
		// checked above is carried into the independent action/edge graph.
		return g.value("dictionary", map[string]any{"WFItems": items})
	}
	list := func(ids ...string) string {
		rows := make([]any, len(ids))
		for i, id := range ids {
			rows[i] = map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(id)}
		}
		return g.value("list", map[string]any{"WFItems": rows})
	}
	store := func(id string) {
		g.emit("setvariable", map[string]any{"WFVariableName": "data", "WFInput": p15FunctionEdge(id)})
	}
	tail := map[string]any{
		"tail":  []any{p15LiteralNumber("3"), p15LiteralNumber("2"), p15LiteralNumber("1")},
		"empty": []any{},
	}
	if route == "dictionary_in_list_assignment" {
		store(list(dictionary(tail), dictionary(p15L01ExpectedNestedDictionary())))
	}
	first := dictionary(p15L01ExpectedNestedDictionary())
	ids := []string{first}
	if route != "dictionary_in_list" {
		ids = append(ids, dictionary(tail))
	}
	if route == "dictionary_in_list_aliases" {
		ids = append(ids, first)
	}
	value := list(ids...)
	if route == "dictionary_in_list_mutable" || route == "dictionary_in_list_assignment" {
		store(value)
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionVariable("data")})
	} else {
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(value)})
	}
	g.emit("nothing", nil)
	if err := p15S01CompleteGraph(g); err != nil {
		return err
	}
	return p15M02MainVoidBoundaryError(actions)
}

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
			if route == "dictionary_in_list" {
				if err := p15L01DictionaryListGraphError(actions, route); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
	for _, route := range []string{
		"dictionary_in_list_ordered", "dictionary_in_list_mutable",
		"dictionary_in_list_assignment", "dictionary_in_list_aliases",
	} {
		t.Run(route, func(t *testing.T) {
			actions := p15CompiledActions(t, p15L01DictionaryListSource(t, route))
			if err := p15L01DictionaryListGraphError(actions, route); err != nil {
				t.Fatal(err)
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

func p15S02LiteralRouteSource(route, expression string) string {
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
	return "let name=text(\"EXPANSION_MUST_NOT_HAPPEN\")\n" + body
}

// The S01 native transport repair lifts only literal U+FFFC text in typed
// Dictionary fields into a plain String Text producer. The general literal
// reader above stays attachment-free. These six authored complete graphs bind
// the exact original bytes, carrier identity/range, every store and consumer,
// and explicit-return versus empty-main behavior.
func p15S02LiteralCarrierGraphError(actions []map[string]any, route, want string) error {
	if !strings.ContainsRune(want, '\uFFFC') {
		return fmt.Errorf("carrier oracle requires the literal U+FFFC case")
	}
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}}
	g.value("gettext", map[string]any{"WFTextActionText": "EXPANSION_MUST_NOT_HAPPEN"})
	payload := func() string {
		carrier := p15S01Carrier(g, want)
		return p15S01Dictionary(g, p15S01ContainerRow(p15FunctionText("payload"), 0, p15FunctionText("\uFFFC", carrier)))
	}
	consume := func(id string) { g.emit("getvariable", map[string]any{"WFVariable": p15FunctionEdge(id)}) }
	switch route {
	case "dictionary_yield":
		yes := g.value("number", map[string]any{"WFNumberActionNumber": 1})
		group := g.conditional(4, p15FunctionEdge(yes), "WFNumberValue", 1)
		payload()
		g.boundary(group, 1, false)
		payload()
		consume(g.boundary(group, 2, true))
	case "dictionary_assignment":
		empty := p15S01Dictionary(g, []any{}...)
		g.emit("setvariable", map[string]any{"WFVariableName": "literal", "WFInput": p15FunctionEdge(empty)})
		value := payload()
		g.emit("setvariable", map[string]any{"WFVariableName": "literal", "WFInput": p15FunctionEdge(value)})
		g.emit("getvariable", map[string]any{"WFVariable": p15FunctionVariable("literal")})
	case "dictionary", "dictionary_mutable", "dictionary_return", "dictionary_in_list":
		value := payload()
		switch route {
		case "dictionary":
			consume(value)
		case "dictionary_mutable":
			g.emit("setvariable", map[string]any{"WFVariableName": "literal", "WFInput": p15FunctionEdge(value)})
			g.emit("getvariable", map[string]any{"WFVariable": p15FunctionVariable("literal")})
		case "dictionary_return":
			g.output(value)
		case "dictionary_in_list":
			list := g.value("list", map[string]any{"WFItems": []any{map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(value)}}})
			consume(list)
		}
	default:
		return fmt.Errorf("unknown literal Dictionary route %q", route)
	}
	if route != "dictionary_return" {
		g.emit("nothing", nil)
		if err := p15M02MainVoidBoundaryError(actions); err != nil {
			return err
		}
	}
	if err := p15S01CompleteGraph(g); err != nil {
		return err
	}
	for _, action := range actions {
		if err := p15S01CompletePlaceholderCoverage(action); err != nil {
			return err
		}
	}
	return nil
}

func TestP15S02LiteralCarrierGraphsRejectCausalMutations(t *testing.T) {
	want := "שָׁלוֹם👩‍💻e\u0301\r\n\t\"\\ {name} \uFFFC"
	for _, route := range []string{"dictionary", "dictionary_mutable", "dictionary_assignment", "dictionary_return", "dictionary_yield", "dictionary_in_list"} {
		t.Run(route, func(t *testing.T) {
			original := p15CompiledActions(t, p15S02LiteralRouteSource(route, strconv.Quote(want)))
			if err := p15S02LiteralCarrierGraphError(original, route, want); err != nil {
				t.Fatal("healthy complete literal graph", err)
			}
			carriers := []int{}
			dictionary := -1
			for index, action := range original {
				if p15Param(action)["WFTextActionText"] == want && p15Param(action)["CustomOutputName"] == "Text" {
					carriers = append(carriers, index)
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.dictionary" {
					dictionary = index
				}
			}
			expected := 1
			if route == "dictionary_yield" {
				expected = 2
			}
			if len(carriers) != expected || dictionary < 0 {
				t.Fatal("actual literal producers missing")
			}
			faults := []string{"literal_bytes", "producer_kind", "producer_name", "producer_uuid", "consumer_uuid", "consumer_name", "attachment_range", "field_type", "field_key", "consumed_result_uuid", "consumer_edge", "helper_reordered", "duplicate_uuid", "main_vs_return", "terminal_input"}
			if route == "dictionary_yield" {
				faults = append(faults, "second_branch_bytes")
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					broken := p15C01Clone(t, original)
					carrier := carriers[0]
					id := p15Param(broken[carrier])["UUID"]
					changed := true
					switch fault {
					case "literal_bytes":
						p15Param(broken[carrier])["WFTextActionText"] = strings.ReplaceAll(want, "\uFFFC", "")
					case "second_branch_bytes":
						p15Param(broken[carriers[1]])["WFTextActionText"] = strings.ReplaceAll(want, "e\u0301", "é")
					case "producer_kind":
						broken[carrier]["WFWorkflowActionIdentifier"] = "is.workflow.actions.number"
					case "producer_name":
						p15Param(broken[carrier])["CustomOutputName"] = "Wrong"
					case "producer_uuid":
						delete(p15Param(broken[carrier]), "UUID")
					case "consumer_uuid", "consumer_name":
						changed = false
						for _, action := range broken {
							if p15S01ChangeFirst(action, func(item map[string]any) bool {
								if item["Type"] != "ActionOutput" || item["OutputUUID"] != id {
									return false
								}
								if fault == "consumer_uuid" {
									item["OutputUUID"] = "FOREIGN"
								} else {
									item["OutputName"] = "Wrong"
								}
								return true
							}) {
								changed = true
								break
							}
						}
					case "attachment_range":
						changed = false
						for _, action := range broken {
							if p15S01ChangeFirst(action, func(item map[string]any) bool {
								if item["WFSerializationType"] != "WFTextTokenString" {
									return false
								}
								body, _ := item["Value"].(map[string]any)
								ranges, _ := body["attachmentsByRange"].(map[string]any)
								ref, _ := ranges["{0, 1}"].(map[string]any)
								if ref["OutputUUID"] != id {
									return false
								}
								delete(ranges, "{0, 1}")
								ranges["{1, 1}"] = ref
								return true
							}) {
								changed = true
								break
							}
						}
					case "field_type", "field_key":
						changed = false
						for _, action := range broken {
							if p15S01ChangeFirst(action, func(item map[string]any) bool {
								key, err := p15LiteralText(item["WFKey"])
								if err != nil || key != "payload" {
									return false
								}
								if fault == "field_type" {
									item["WFItemType"] = 3
								} else {
									item["WFKey"] = p15FunctionText("WRONG")
								}
								return true
							}) {
								changed = true
								break
							}
						}
					case "consumed_result_uuid":
						producer := dictionary
						if route == "dictionary_yield" {
							producer = -1
							for index, action := range broken {
								if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p15Param(action)["WFControlFlowMode"]) == 2 {
									producer = index
								}
							}
							if producer < 0 {
								t.Fatal("actual yielded result producer absent")
							}
						}
						p15Param(broken[producer])["UUID"] = "FOREIGN_CONSUMED_RESULT"
					case "consumer_edge":
						index := len(broken) - 2
						if route == "dictionary_return" {
							index++
							p15Param(broken[index])["WFOutput"] = p15FunctionText("\uFFFC", p15Param(broken[0])["UUID"].(string))
						} else {
							p15Param(broken[index])["WFVariable"] = p15FunctionEdge(p15Param(broken[0])["UUID"].(string))
						}
					case "helper_reordered":
						broken[carrier], broken[carrier+1] = broken[carrier+1], broken[carrier]
					case "duplicate_uuid":
						p15Param(broken[dictionary])["UUID"] = id
					case "main_vs_return":
						if route == "dictionary_return" {
							broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
						} else {
							broken = broken[:len(broken)-1]
						}
					case "terminal_input":
						broken[len(broken)-1]["WFWorkflowActionParameters"] = map[string]any{"WFInput": "LEAK"}
					}
					if !changed {
						t.Fatal("causal mutation target absent", fault)
					}
					if err := p15S02LiteralCarrierGraphError(broken, route, want); err == nil {
						t.Fatal("accepted actual literal graph corruption", fault)
					}
				})
			}
		})
	}
}
