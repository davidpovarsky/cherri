package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

const p15RP20Fixture = "testdata/p15/rp20-joined-values.cherri"
const p15RP20FixtureSHA = "7b4226b10fcfa5f8c3d303e6408b7275ce6e76c9023bcbdf330d251e092dcf91"

// RP20 is one authored, joined scenario. A successful serialization proves the
// private value graph; it does not establish Apple's native runtime behavior.
func p15RP20Source(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(p15RP20Fixture)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	if hex.EncodeToString(sum[:]) != p15RP20FixtureSHA {
		t.Fatal("RP20 joined source identity changed")
	}
	return string(raw)
}

func TestP15RP20JoinedValueSemantics(t *testing.T) {
	p, a := p15F03Analysis(t, p15RP20Source(t))
	if errors := p15F03Errors(a); len(errors) != 0 {
		t.Fatal(errors)
	}
	items, ok := p15BindingValue(t, p, "items").(*syntax.ListExpr)
	if !ok || len(items.Elements) != 7 {
		t.Fatal("joined value list lost seven distinct slots")
	}
	for _, tc := range []struct {
		name   string
		at     int
		token  syntax.TokenType
		want   types.Type
		absent bool
	}{
		{"none", 0, syntax.TokenNone, nil, true},
		{"false", 1, syntax.TokenFalse, types.Bool, false},
		{"zero", 2, syntax.TokenNumber, types.Number, false},
		{"empty_text", 3, syntax.TokenString, types.Text, false},
		{"json_null", 6, syntax.TokenNull, types.JsonNull, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			literal, okay := items.Elements[tc.at].(*syntax.LiteralExpr)
			if !okay || literal.Type != tc.token {
				t.Fatal("source literal kind/order changed")
			}
			actual := a.ExprTypes()[literal]
			if actual == nil || tc.want != nil && !actual.Equals(tc.want) ||
				types.MayBeAbsent(actual) != tc.absent || types.DefinitelyPresent(actual) == tc.absent {
				t.Fatalf("source presence/type changed for %s: %v", tc.name, actual)
			}
		})
	}
	t.Run("empty_container_identity", func(t *testing.T) {
		list, listOK := items.Elements[4].(*syntax.ListExpr)
		dictionary, mapOK := items.Elements[5].(*syntax.MapExpr)
		if !listOK || len(list.Elements) != 0 || !mapOK || len(dictionary.Entries) != 0 {
			t.Fatal("empty List/Map shape changed")
		}
		listType, mapType := a.ExprTypes()[list], a.ExprTypes()[dictionary]
		if listType == nil || mapType == nil || listType.Kind() != types.KindList || mapType.Kind() != types.KindMap ||
			listType.Equals(mapType) || !types.DefinitelyPresent(listType) || !types.DefinitelyPresent(mapType) {
			t.Fatal("empty containers collapsed or inherited absence")
		}
	})
	t.Run("declared_optional_hierarchy", func(t *testing.T) {
		rows, okay := p15BindingValue(t, p, "rows").(*syntax.ListExpr)
		if !okay || len(rows.Elements) != 3 {
			t.Fatal("nested outer cardinality changed")
		}
		for i, width := range []int{2, 0, 1} {
			row, rowOK := rows.Elements[i].(*syntax.ListExpr)
			if !rowOK || len(row.Elements) != width {
				t.Fatalf("nested row %d cardinality changed", i)
			}
		}
		alias := a.ExprTypes()[p15BindingValue(t, p, "rowsAlias")]
		want := types.NewList(types.NewList(types.NewOptional(types.Bool)))
		if alias == nil || !alias.Equals(want) || !types.DefinitelyPresent(alias) {
			t.Fatalf("declared Optional hierarchy lost: %v", alias)
		}
		if alias.Equals(types.NewList(types.NewOptional(types.Bool))) {
			t.Fatal("flattened Optional hierarchy accepted")
		}
	})
	t.Run("explicit_json_and_joined_aliases", func(t *testing.T) {
		jsonLiteral := a.ExprTypes()[p15BindingValue(t, p, "json")]
		jsonAlias := a.ExprTypes()[p15BindingValue(t, p, "jsonAlias")]
		if jsonLiteral == nil || !jsonLiteral.Equals(types.JsonNull) || jsonAlias == nil || !jsonAlias.Equals(types.JsonValue) ||
			!types.DefinitelyPresent(jsonAlias) || jsonLiteral.AssignableTo(types.AnyContent) {
			t.Fatal("explicit present JSON null lost its type boundary")
		}
		joined, okay := p15BindingValue(t, p, "joined").(*syntax.MapExpr)
		if !okay || len(joined.Entries) != 3 {
			t.Fatal("joined Map lost its three fields")
		}
		for i, pair := range [][2]string{{"items", "items"}, {"rows", "rows"}, {"json", "json"}} {
			key, keyOK := joined.Entries[i].Key.(*syntax.LiteralExpr)
			value, valueOK := joined.Entries[i].Value.(*syntax.IdentExpr)
			if !keyOK || key.Type != syntax.TokenString || key.Value != pair[0] || !valueOK || value.Name != pair[1] {
				t.Fatalf("joined Map field %d no longer binds its original source", i)
			}
		}
		joinedType := a.ExprTypes()[joined]
		joinedAlias := a.ExprTypes()[p15BindingValue(t, p, "joinedAlias")]
		itemsType := a.ExprTypes()[items]
		itemsAlias := a.ExprTypes()[p15BindingValue(t, p, "itemsAlias")]
		if joinedType == nil || joinedType.Kind() != types.KindMap || joinedAlias == nil || !joinedAlias.Equals(joinedType) ||
			!types.DefinitelyPresent(joinedAlias) || !types.PrivateValueTransport(joinedAlias) ||
			itemsType == nil || itemsAlias == nil || !itemsAlias.Equals(itemsType) {
			t.Fatal("joined shape/alias/presence erased")
		}
	})
}

// Independently author all operations and payloads. Only UUID alpha names are
// read at their exact required action positions; no expected semantic value is
// copied from the compiler's graph.
func p15RP20Graph(actual []map[string]any) (map[string]int, error) {
	g := &p15FunctionGraph{actual: actual, groups: map[string]bool{}}
	pos := map[string]int{}
	at := func(name string) { pos[name] = len(g.expected) }
	cellList := func(cells ...string) string {
		rows := []any{}
		for _, cell := range cells {
			rows = append(rows, map[string]any{"WFItemType": 0, "WFValue": p15FunctionEdge(cell)})
		}
		return g.value("list", map[string]any{"WFItems": rows})
	}
	edgeValue := func(id string) p15FunctionArgument { return p15FunctionArgument{true, 0, p15FunctionEdge(id)} }
	at("none")
	absent := p15TaggedAbsentCellGraph(g)
	at("false")
	falsity := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	at("zero")
	zero := p15TaggedCellGraph(g, "number", p15OptionalJoinValue("zero"))
	at("empty_text")
	text := p15TaggedCellGraph(g, "text", p15OptionalJoinValue("empty_text"))
	at("empty_list")
	list := p15TaggedCellGraph(g, "list", p15OptionalJoinValue("empty_list"))
	at("empty_map")
	emptyMap := g.value("dictionary", map[string]any{"WFItems": p15OptionalJoinValue("empty_map").value})
	at("map_cell")
	dictionary := p15TaggedCellGraph(g, "map", edgeValue(emptyMap))
	at("null")
	null := p15TaggedNullCell(g)
	at("items")
	items := cellList(absent, falsity, zero, text, list, dictionary, null)
	at("row_false")
	rowFalse := p15TaggedCellGraph(g, "bool", p15OptionalJoinValue("false"))
	at("row_none")
	rowNone := p15TaggedAbsentCellGraph(g)
	at("row_first")
	first := p15TaggedCellGraph(g, "list", p15F04ArrayValue(edgeValue(rowFalse), edgeValue(rowNone)))
	at("row_empty")
	second := p15TaggedCellGraph(g, "list", p15F04ArrayValue())
	at("row_true")
	rowTrue := p15TaggedCellGraph(g, "bool", p15FunctionArgument{true, 4, map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": true}})
	at("row_last")
	third := p15TaggedCellGraph(g, "list", p15F04ArrayValue(edgeValue(rowTrue)))
	at("rows")
	rows := cellList(first, second, third)
	at("explicit_json")
	json := p15TaggedNullCell(g)
	at("items_join_cell")
	itemsCell := p15TaggedCellGraph(g, "list", edgeValue(items))
	at("rows_join_cell")
	rowsCell := p15TaggedCellGraph(g, "list", edgeValue(rows))
	at("joined")
	joined := g.value("dictionary", map[string]any{"WFItems": map[string]any{
		"WFSerializationType": "WFDictionaryFieldValue",
		"Value": map[string]any{"WFDictionaryFieldValueItems": []any{
			map[string]any{"WFKey": p15FunctionText("items"), "WFItemType": 0, "WFValue": p15FunctionEdge(itemsCell)},
			map[string]any{"WFKey": p15FunctionText("rows"), "WFItemType": 0, "WFValue": p15FunctionEdge(rowsCell)},
			map[string]any{"WFKey": p15FunctionText("json"), "WFItemType": 0, "WFValue": p15FunctionEdge(json)},
		}},
	}})
	at("count")
	count := g.value("count", map[string]any{"Input": p15FunctionEdge(joined)})
	at("output")
	g.output(count)
	return pos, g.compare()
}

func p15RP20Compile(t *testing.T, profile BuildProfile) ([]map[string]any, map[string]any) {
	t.Helper()
	source := p15RP20Source(t)
	raw, err := CompileSourceToPlistWithOptions("p15-rp20-joined-values.cherri", source, CompileOptions{BuildProfile: profile})
	if err != nil {
		t.Fatal(err)
	}
	if directory := os.Getenv("CHERRI_STRUCTURED_VALUE_ARTIFACTS"); directory != "" {
		caseDir, e := os.MkdirTemp(directory, "rp20-"+string(profile)+"-")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(caseDir, "source.cherri"), []byte(source), 0600); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(caseDir, "actual.shortcut"), raw, 0600); e != nil {
			t.Fatal(e)
		}
		sum := sha256.Sum256(raw)
		t.Logf("RP20_JOINED_NATIVE_ARTIFACT source_sha256=%s profile=%s sha256=%s path=%s", p15RP20FixtureSHA, profile, hex.EncodeToString(sum[:]), filepath.Join(caseDir, "actual.shortcut"))
	}
	var document map[string]any
	if _, err = plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	rows, okay := document["WFWorkflowActions"].([]any)
	if !okay {
		t.Fatal("actual plist lost its action array")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		action, ok := row.(map[string]any)
		if !ok {
			t.Fatalf("action %d lost Dictionary identity", i)
		}
		actions[i] = action
	}
	return actions, document
}

func TestP15RP20JoinedNativeStructure(t *testing.T) {
	for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
		t.Run(string(profile), func(t *testing.T) {
			actions, document := p15RP20Compile(t, profile)
			if _, err := p15RP20Graph(actions); err != nil {
				t.Fatal(err)
			}
			for _, format := range []int{plist.XMLFormat, plist.BinaryFormat} {
				raw, err := plist.Marshal(document, format)
				if err != nil {
					t.Fatal(err)
				}
				var restored map[string]any
				if _, err = plist.Unmarshal(raw, &restored); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(document, restored) {
					t.Fatalf("plist format %d lost exact hierarchy/value/identity", format)
				}
			}
		})
	}
}

func TestP15RP20JoinedGraphCausalFaults(t *testing.T) {
	actions, _ := p15RP20Compile(t, BuildProfileRelease)
	pos, err := p15RP20Graph(actions)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{
		"false_to_true", "bool_wrapper_dropped", "false_absent", "zero_to_one", "empty_text_changed", "empty_text_payload_dropped",
		"empty_list_to_map", "empty_list_payload_dropped", "empty_map_extra_field", "empty_map_ref_rebound",
		"null_to_absent", "null_fake_payload", "none_to_null", "none_fake_payload",
		"items_reordered", "items_dropped", "items_duplicated",
		"nested_list_flattened", "empty_row_removed", "optional_slot_to_null", "true_to_false",
		"items_join_rebound", "rows_join_rebound", "joined_key_changed", "joined_field_dropped", "joined_field_duplicated",
		"explicit_json_rebound", "count_wrong_container", "private_payload_exported", "forward_reference", "duplicate_uuid", "extra_action",
	} {
		t.Run(fault, func(t *testing.T) {
			broken := p15TaggedClone(t, actions)
			fields := func(name string) map[string]map[string]any {
				value, e := p15ContainerFields(p15Param(broken[pos[name]])["WFItems"])
				if e != nil {
					t.Fatal(e)
				}
				return value
			}
			id := func(name string) string { return p15Param(broken[pos[name]])["UUID"].(string) }
			dictRows := func(name string) []any {
				return p15Param(broken[pos[name]])["WFItems"].(map[string]any)["Value"].(map[string]any)["WFDictionaryFieldValueItems"].([]any)
			}
			setDictRows := func(name string, rows []any) {
				p15Param(broken[pos[name]])["WFItems"].(map[string]any)["Value"].(map[string]any)["WFDictionaryFieldValueItems"] = rows
			}
			switch fault {
			case "false_to_true":
				fields("false")["value"]["WFValue"] = map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": true}
			case "bool_wrapper_dropped":
				fields("false")["value"]["WFValue"] = false
			case "false_absent":
				fields("false")["has_value"]["WFValue"] = p15FunctionText("0")
			case "zero_to_one":
				fields("zero")["value"]["WFValue"] = p15FunctionText("1")
			case "empty_text_changed":
				fields("empty_text")["value"]["WFValue"] = p15FunctionText("nonempty")
			case "empty_text_payload_dropped":
				setDictRows("empty_text", dictRows("empty_text")[:3])
			case "empty_list_to_map":
				fields("empty_list")["kind"]["WFValue"] = p15FunctionText("map")
			case "empty_list_payload_dropped":
				setDictRows("empty_list", dictRows("empty_list")[:3])
			case "empty_map_extra_field":
				setDictRows("empty_map", []any{map[string]any{"WFKey": p15FunctionText("unexpected"), "WFItemType": 0, "WFValue": p15FunctionText("x")}})
			case "empty_map_ref_rebound":
				fields("map_cell")["value"]["WFValue"] = p15FunctionEdge(id("empty_list"))
			case "null_to_absent":
				fields("null")["has_value"]["WFValue"] = p15FunctionText("0")
			case "null_fake_payload":
				setDictRows("null", append(dictRows("null"), map[string]any{"WFKey": p15FunctionText("value"), "WFItemType": 0, "WFValue": nil}))
			case "none_to_null":
				fields("none")["kind"]["WFValue"] = p15FunctionText("null")
			case "none_fake_payload":
				setDictRows("none", append(dictRows("none"), map[string]any{"WFKey": p15FunctionText("value"), "WFItemType": 0, "WFValue": p15FunctionText("")}))
			case "items_reordered":
				rows := p15Param(broken[pos["items"]])["WFItems"].([]any)
				rows[0], rows[6] = rows[6], rows[0]
			case "items_dropped":
				rows := p15Param(broken[pos["items"]])["WFItems"].([]any)
				p15Param(broken[pos["items"]])["WFItems"] = rows[:6]
			case "items_duplicated":
				rows := p15Param(broken[pos["items"]])["WFItems"].([]any)
				p15Param(broken[pos["items"]])["WFItems"] = append(rows, rows[0])
			case "nested_list_flattened":
				fields("row_first")["value"]["WFValue"] = p15FunctionEdge(id("row_false"))
			case "empty_row_removed":
				rows := p15Param(broken[pos["rows"]])["WFItems"].([]any)
				p15Param(broken[pos["rows"]])["WFItems"] = []any{rows[0], rows[2]}
			case "optional_slot_to_null":
				fields("row_none")["kind"]["WFValue"] = p15FunctionText("null")
			case "true_to_false":
				fields("row_true")["value"]["WFValue"] = map[string]any{"WFSerializationType": "WFNumberSubstitutableState", "Value": false}
			case "items_join_rebound":
				fields("items_join_cell")["value"]["WFValue"] = p15FunctionEdge(id("rows"))
			case "rows_join_rebound":
				fields("rows_join_cell")["value"]["WFValue"] = p15FunctionEdge(id("items"))
			case "joined_key_changed":
				dictRows("joined")[0].(map[string]any)["WFKey"] = p15FunctionText("rows")
			case "joined_field_dropped":
				setDictRows("joined", dictRows("joined")[:2])
			case "joined_field_duplicated":
				setDictRows("joined", append(dictRows("joined"), dictRows("joined")[0]))
			case "explicit_json_rebound":
				dictRows("joined")[2].(map[string]any)["WFValue"] = p15FunctionEdge(id("none"))
			case "count_wrong_container":
				p15Param(broken[pos["count"]])["Input"] = p15FunctionEdge(id("items"))
			case "private_payload_exported":
				p15Param(broken[pos["output"]])["WFOutput"] = p15FunctionText("\ufffc", id("joined"))
			case "forward_reference":
				fields("map_cell")["value"]["WFValue"] = p15FunctionEdge(id("count"))
			case "duplicate_uuid":
				p15Param(broken[pos["false"]])["UUID"] = id("none")
			case "extra_action":
				broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			default:
				t.Fatal(fmt.Errorf("unregistered RP20 fault %q", fault))
			}
			if _, err := p15RP20Graph(broken); err == nil {
				t.Fatal("accepted semantic/structural corruption: " + fault)
			}
		})
	}
}
