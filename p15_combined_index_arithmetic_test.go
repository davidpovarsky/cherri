package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

// RP21 and BRG16 execute their own complete sources. The arithmetic cursor is
// reused only for exact native field/identity checking; no prior requirement's
// events, outcomes, runtime traces, or acceptance are inherited.
type p15IXCase struct {
	Name, Mode, Literal, Outcome string
	Value                        any
	Items                        []string
	Offset                       uint64
}

var p15IXCases = map[string][]p15IXCase{
	"RP21": {
		{Name: "static_first", Mode: "int", Literal: "0", Value: uint64(0), Items: []string{"alpha", "beta", "gamma"}, Outcome: "Text:alpha", Offset: uint64(0)},
		{Name: "static_last", Mode: "int", Literal: "2", Value: uint64(2), Items: []string{"alpha", "beta", "gamma"}, Outcome: "Text:gamma", Offset: uint64(0)},
		{Name: "static_empty", Mode: "int", Literal: "0", Value: uint64(0), Items: []string{}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_negative", Mode: "int", Literal: "-1", Value: int64(-1), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_out_of_range", Mode: "int", Literal: "3", Value: uint64(3), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_uint64_max", Mode: "int", Literal: "18446744073709551615", Value: uint64(18446744073709551615), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_fraction", Mode: "real", Literal: "0.5", Value: float64(0.5), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_negative_fraction", Mode: "real", Literal: "-0.5", Value: float64(-0.5), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "static_integral_real", Mode: "real", Literal: "2.0", Value: float64(2.0), Items: []string{"alpha", "beta", "gamma"}, Outcome: "Text:gamma", Offset: uint64(0)},
		{Name: "dynamic_first", Mode: "var", Literal: "0", Value: uint64(0), Items: []string{"alpha", "beta", "gamma"}, Outcome: "Text:alpha", Offset: uint64(0)},
		{Name: "dynamic_last", Mode: "var", Literal: "2", Value: uint64(2), Items: []string{"alpha", "beta", "gamma"}, Outcome: "Text:gamma", Offset: uint64(0)},
		{Name: "dynamic_empty", Mode: "var", Literal: "0", Value: uint64(0), Items: []string{}, Outcome: "None", Offset: uint64(0)},
		{Name: "dynamic_negative", Mode: "var", Literal: "-1", Value: int64(-1), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "dynamic_out_of_range", Mode: "var", Literal: "3", Value: uint64(3), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "dynamic_fraction", Mode: "var", Literal: "0.5", Value: float64(0.5), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "dynamic_negative_fraction", Mode: "var", Literal: "-0.5", Value: float64(-0.5), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "inline_fraction", Mode: "inline", Literal: "0.5", Value: float64(0.5), Items: []string{"alpha", "beta", "gamma"}, Outcome: "None", Offset: uint64(0)},
		{Name: "dynamic_empty_text", Mode: "var", Literal: "0", Value: uint64(0), Items: []string{""}, Outcome: "Text:", Offset: uint64(0)},
	},
	"BRG16": {
		{Name: "static_first", Mode: "int", Literal: "0", Value: uint64(0), Items: []string{"start", "middle", "end"}, Outcome: "Text:start", Offset: uint64(7)},
		{Name: "static_last", Mode: "int", Literal: "2", Value: uint64(2), Items: []string{"start", "middle", "end"}, Outcome: "Text:end", Offset: uint64(3)},
		{Name: "static_empty", Mode: "int", Literal: "0", Value: uint64(0), Items: []string{}, Outcome: "None", Offset: uint64(7)},
		{Name: "static_negative", Mode: "int", Literal: "-1", Value: int64(-1), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(9)},
		{Name: "static_out_of_range", Mode: "int", Literal: "3", Value: uint64(3), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(1)},
		{Name: "static_fraction", Mode: "real", Literal: "0.5", Value: float64(0.5), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(6)},
		{Name: "static_integral_real", Mode: "real", Literal: "2.0", Value: float64(2.0), Items: []string{"start", "middle", "end"}, Outcome: "Text:end", Offset: uint64(3)},
		{Name: "dynamic_first", Mode: "compound", Literal: "0", Value: uint64(0), Items: []string{"start", "middle", "end"}, Outcome: "Text:start", Offset: uint64(7)},
		{Name: "dynamic_last", Mode: "compound", Literal: "2", Value: uint64(2), Items: []string{"start", "middle", "end"}, Outcome: "Text:end", Offset: uint64(3)},
		{Name: "dynamic_empty", Mode: "compound", Literal: "0", Value: uint64(0), Items: []string{}, Outcome: "None", Offset: uint64(7)},
		{Name: "dynamic_negative", Mode: "compound", Literal: "-1", Value: int64(-1), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(9)},
		{Name: "dynamic_out_of_range", Mode: "compound", Literal: "3", Value: uint64(3), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(1)},
		{Name: "dynamic_fraction", Mode: "compound", Literal: "0.5", Value: float64(0.5), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(6)},
		{Name: "dynamic_negative_fraction", Mode: "compound", Literal: "-0.5", Value: float64(-0.5), Items: []string{"start", "middle", "end"}, Outcome: "None", Offset: uint64(8)},
		{Name: "dynamic_empty_text", Mode: "compound", Literal: "0", Value: uint64(0), Items: []string{""}, Outcome: "Text:", Offset: uint64(7)},
	},
}

type p15IXFixture struct {
	Schema      string `json:"schema"`
	Requirement string `json:"requirement"`
	Cases       []struct {
		Name   string `json:"name"`
		Source string `json:"source"`
	} `json:"cases"`
	Invalid []struct {
		Name    string `json:"name"`
		Operand string `json:"operand"`
		Source  string `json:"source"`
	} `json:"invalid"`
}

func p15IXLoad(t *testing.T, id string) p15IXFixture {
	t.Helper()
	name := map[string]string{"RP21": "v2-rp21-safe-indexing.json", "BRG16": "v2-brg16-numeric-bounds.json"}[id]
	if name == "" {
		t.Fatal("unknown distinct index requirement")
	}
	raw, err := os.ReadFile(filepath.Join("tests", "language-v2", name))
	if err != nil {
		t.Fatal(err)
	}
	var fixture p15IXFixture
	d := json.NewDecoder(bytes.NewReader(raw))
	d.DisallowUnknownFields()
	if err = d.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		t.Fatal("trailing index fixture data")
	}
	if fixture.Schema != "cherri.p15.combined-index/v1" || fixture.Requirement != id || len(fixture.Cases) != len(p15IXCases[id]) || len(fixture.Invalid) != 2 {
		t.Fatal("changed fixed index scenario")
	}
	for i, spec := range p15IXCases[id] {
		if fixture.Cases[i].Name != spec.Name || fixture.Cases[i].Source == "" {
			t.Fatal("changed fixed index source order")
		}
	}
	for i, name := range []string{"text_index", "bool_index"} {
		if fixture.Invalid[i].Name != name || fixture.Invalid[i].Source == "" {
			t.Fatal("changed fixed invalid-index source order")
		}
	}
	if fixture.Invalid[0].Operand != `"not-an-index"` || fixture.Invalid[1].Operand != "false" {
		t.Fatal("changed nonnumeric operands")
	}
	return fixture
}
func p15IXLiteralShape(literal string) string {
	if strings.HasPrefix(literal, "-") {
		return "(- " + strings.TrimPrefix(literal, "-") + ")"
	}
	return literal
}
func p15IXUnit(t *testing.T, id string) {
	fixture := p15IXLoad(t, id)
	for i, spec := range p15IXCases[id] {
		t.Run(spec.Name, func(t *testing.T) {
			source := fixture.Cases[i].Source
			program, a := p15T08Analyzer(t, source)
			p15AMHealthyCompile(t, source)
			list, ok := p15BindingValue(t, program, "items").(*syntax.ListExpr)
			if !ok || len(list.Elements) != len(spec.Items) {
				t.Fatal("changed index collection cardinality")
			}
			for i, value := range list.Elements {
				literal, ok := value.(*syntax.LiteralExpr)
				if !ok || literal.Value != spec.Items[i] {
					t.Fatal("changed exact Text collection element")
				}
				p15AMRequireType(t, value, types.Text, a)
			}
			observed, ok := p15BindingValue(t, program, "observed").(*syntax.IndexExpr)
			if !ok {
				t.Fatal("missing actual list index")
			}
			p15AMRequireShape(t, observed.Target, "items")
			p15AMRequireType(t, observed.Target, types.NewList(types.Text), a)
			p15AMRequireType(t, observed.Index, types.Number, a)
			p15AMRequireType(t, observed, types.NewOptional(types.Text), a)
			indexShape := p15IXLiteralShape(spec.Literal)
			statementCount := 3
			switch spec.Mode {
			case "var":
				statementCount = 4
				value := p15BindingValue(t, program, "index")
				p15AMRequireShape(t, value, "(call number "+indexShape+")")
				p15AMRequireType(t, value, types.Number, a)
				indexShape = "index"
			case "inline":
				indexShape = "(call number " + indexShape + ")"
			case "compound":
				indexShape = "index"
			case "int", "real":
			default:
				t.Fatal("unknown index mode")
			}
			if id == "BRG16" {
				statementCount = 15
				for _, seed := range []struct{ name, shape string }{
					{"numerator", "(call number 5)"}, {"denominator", "(call number 2)"},
					{"fraction", "(/ numerator denominator)"}, {"dividend", "(call number 10)"},
					{"divisor", "(call number 3)"}, {"remainder", "(% dividend divisor)"}, {"index", "fraction"},
				} {
					value := p15BindingValue(t, program, seed.name)
					p15AMRequireType(t, value, types.Number, a)
					p15AMRequireShape(t, value, seed.shape)
				}
				var updates []*syntax.AssignStmt
				for _, statement := range program.Statements {
					if update, ok := statement.(*syntax.AssignStmt); ok {
						updates = append(updates, update)
					}
				}
				if len(updates) != 4 {
					t.Fatal("joined bounds scenario lost four compound assignments")
				}
				for j, step := range []struct {
					op  syntax.TokenType
					rhs string
				}{
					{syntax.TokenPlusAssign, "remainder"}, {syntax.TokenStarAssign, "2"},
					{syntax.TokenMinusAssign, fmt.Sprint(spec.Offset)}, {syntax.TokenSlashAssign, "2"},
				} {
					update := updates[j]
					if update.Name != "index" || update.Op != step.op {
						t.Fatal("numeric producer/operator no longer feeds index")
					}
					p15AMRequireType(t, update.Value, types.Number, a)
					p15AMRequireShape(t, update.Value, step.rhs)
				}
				observation, ok := program.Statements[12].(*syntax.ExprStmt)
				if !ok {
					t.Fatal("missing derived-number observation")
				}
				p15AMRequireShape(t, observation.Expr, "(call setClipboard index)")
			}
			if len(program.Statements) != statementCount {
				t.Fatal("unexpected statement outside fixed index scenario")
			}
			p15AMRequireShape(t, observed.Index, indexShape)
			returned, ok := program.Statements[len(program.Statements)-1].(*syntax.ReturnStmt)
			if !ok {
				t.Fatal("index result not actually returned")
			}
			p15AMRequireShape(t, returned.Value, "observed")
			p15AMRequireType(t, returned.Value, types.NewOptional(types.Text), a)
			t.Logf("%s %s contract result %q; semantic Text? and fixed numeric input checked; no Apple result is asserted", id, spec.Name, spec.Outcome)
		})
	}
	for _, invalid := range fixture.Invalid {
		t.Run("reject_"+invalid.Name, func(t *testing.T) { p15AMRejectTypedOperand(t, invalid.Source, invalid.Operand) })
	}
}
func TestP15RP21SafeIndexUnit(t *testing.T)      { p15IXUnit(t, "RP21") }
func TestP15BRG16NumericBoundsUnit(t *testing.T) { p15IXUnit(t, "BRG16") }

func p15IXBareNothing(c *p15AMCursor, mark string) {
	c.mark(mark)
	if c.err != nil {
		return
	}
	if c.index >= len(c.actions) || !reflect.DeepEqual(c.actions[c.index], map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}) {
		c.err = fmt.Errorf("expected exact bare Nothing at %d", c.index)
		return
	}
	c.index++
}
func p15IXCollection(c *p15AMCursor, spec p15IXCase) map[string]any {
	c.mark("collection")
	if len(spec.Items) == 0 {
		if c.err != nil {
			return nil
		}
		if c.index+2 >= len(c.actions) {
			c.err = fmt.Errorf("missing exact empty List bridge")
			return nil
		}
		uuid, ok := p15AMParams(c.actions[c.index+2])["UUID"].(string)
		if !ok || !p15AMCanonicalUUID(uuid) {
			c.err = fmt.Errorf("invalid empty List producer")
			return nil
		}
		// This fixture has no prior lexical scope: bind the private cleared name
		// to the real producer UUID, not an arbitrary observed variable spelling.
		name := "__cherri_scope_1_emptyList_" + uuid
		p15IXBareNothing(c, "empty_clear")
		c.mark("empty_store")
		c.take("setvariable", "", map[string]any{"WFVariableName": name}, false)
		c.mark("collection_output")
		return c.take("getvariable", "items", map[string]any{"WFVariable": p15AMAttachment(p15AMVariable(name))}, true)
	}
	items := make([]any, len(spec.Items))
	for i, value := range spec.Items {
		items[i] = map[string]any{"WFItemType": uint64(0), "WFValue": value}
	}
	c.mark("collection_output")
	return c.take("list", "items", map[string]any{"WFItems": items}, true)
}
func p15IXArithmeticPrefix(c *p15AMCursor, spec p15IXCase) {
	numerator := c.number(uint64(5), "numerator", true)
	denominator := c.number(uint64(2), "denominator", true)
	fraction := c.math("fraction", "fraction", "÷", p15AMAttachment(numerator), p15AMAttachment(denominator))
	dividend := c.number(uint64(10), "dividend", true)
	divisor := c.number(uint64(3), "divisor", true)
	c.mark("remainder")
	remainder := c.take("math", "remainder", map[string]any{"WFMathOperation": "…", "WFScientificMathOperation": "Modulus", "WFInput": p15AMAttachment(dividend), "WFScientificMathOperand": p15AMAttachment(divisor)}, true)
	c.mark("index_initial_store")
	c.store("index", fraction)
	for _, step := range []struct {
		name, op string
		right    any
	}{
		{"compound_add", "+", p15AMAttachment(remainder)}, {"compound_multiply", "×", uint64(2)},
		{"compound_subtract", "-", spec.Offset}, {"compound_divide", "÷", uint64(2)},
	} {
		result := c.math(step.name, "", step.op, p15AMAttachment(p15AMVariable("index")), step.right)
		c.mark(step.name + "_store")
		c.store("index", result)
	}
	c.mark("clipboard")
	c.take("setclipboard", "", map[string]any{"WFInput": p15AMAttachment(p15AMVariable("index"))}, false)
}
func p15IXGraph(actions []map[string]any, id string, spec p15IXCase) (p15AMView, error) {
	view := p15AMView{marks: map[string]int{}, results: map[string]map[string]any{}}
	if id != "RP21" && id != "BRG16" {
		return view, fmt.Errorf("unknown complete index scenario")
	}
	if err := p15AMReferences(actions); err != nil {
		return view, err
	}
	c := &p15AMCursor{actions: actions, groups: map[string]bool{}, view: view}
	collection := p15IXCollection(c, spec)
	c.view.results["collection"] = collection
	if id == "BRG16" {
		p15IXArithmeticPrefix(c, spec)
	}
	var index any = spec.Value
	switch spec.Mode {
	case "var":
		c.mark("index_number")
		value := c.number(spec.Value, "Number", true)
		c.mark("index_store")
		c.store("index", value)
		index = p15AMAttachment(p15AMVariable("index"))
	case "inline":
		c.mark("index_number")
		index = p15AMAttachment(c.number(spec.Value, "observed", true))
	case "compound":
		index = p15AMAttachment(p15AMVariable("index"))
	}
	var result map[string]any
	if spec.Mode == "real" {
		value := spec.Value.(float64)
		if value < 0 || value != float64(int64(value)) {
			c.mark("static_absence")
			result = c.take("nothing", "observed", map[string]any{}, true)
		} else {
			oneBased := value + 1.0
			c.mark("count")
			count := c.take("count", "", map[string]any{"Input": p15AMAttachment(collection), "WFCountType": "Items"}, true)
			c.mark("upper_guard")
			group := c.start(count, 3, "WFNumberValue", oneBased)
			c.mark("lookup")
			c.take("getitemfromlist", "", map[string]any{"WFInput": p15AMAttachment(collection), "WFItemSpecifier": "Item At Index", "WFItemIndex": oneBased}, true)
			c.mark("upper_else")
			c.alternate(group)
			p15IXBareNothing(c, "upper_fallback")
			c.mark("result")
			result = c.end(group, "observed", true)
		}
	} else {
		integral := ""
		if spec.Mode != "int" {
			c.mark("integral_modulo")
			remainder := c.take("math", "", map[string]any{"WFMathOperation": "…", "WFScientificMathOperation": "Modulus", "WFInput": index, "WFScientificMathOperand": float64(1)}, true)
			c.mark("integral_guard")
			integral = c.start(remainder, 4, "WFNumberValue", float64(0))
		}
		oneBased := c.math("one_based", "", "+", index, float64(1))
		c.mark("count")
		count := c.take("count", "", map[string]any{"Input": p15AMAttachment(collection), "WFCountType": "Items"}, true)
		difference := c.math("difference", "", "-", p15AMAttachment(count), p15AMAttachment(oneBased))
		c.mark("lower_guard")
		lower := c.start(oneBased, 3, "WFNumberValue", float64(1))
		c.mark("upper_guard")
		upper := c.start(difference, 3, "WFNumberValue", float64(0))
		c.mark("lookup")
		c.take("getitemfromlist", "", map[string]any{"WFInput": p15AMAttachment(collection), "WFItemSpecifier": "Item At Index", "WFItemIndex": p15AMAttachment(oneBased)}, true)
		c.mark("upper_else")
		c.alternate(upper)
		p15IXBareNothing(c, "upper_fallback")
		c.end(upper, "", true)
		c.mark("lower_else")
		c.alternate(lower)
		p15IXBareNothing(c, "lower_fallback")
		name := "observed"
		if integral != "" {
			name = ""
		}
		c.mark("bounded_result")
		result = c.end(lower, name, true)
		if integral != "" {
			c.mark("integral_else")
			c.alternate(integral)
			p15IXBareNothing(c, "integral_fallback")
			c.mark("result")
			result = c.end(integral, "observed", true)
		} else {
			c.view.marks["result"] = c.view.marks["bounded_result"]
		}
	}
	c.view.results["result"] = result
	c.mark("output")
	c.take("output", "", map[string]any{"WFOutput": p15AMRefToken(result)}, false)
	if c.err == nil && c.index != len(actions) {
		c.err = fmt.Errorf("extra action outside complete numeric-index graph")
	}
	return c.view, c.err
}
func p15IXCompile(t *testing.T, id, name, source string) []map[string]any {
	t.Helper()
	raw, err := CompileSourceToPlist("combined-"+strings.ToLower(id)+"-"+name+".cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if _, err = plist.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	rows, ok := workflow["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing actual native rows")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		actions[i], ok = row.(map[string]any)
		if !ok {
			t.Fatal("invalid native row")
		}
	}
	receipt, err := json.Marshal(map[string]any{"requirement": id, "case": name, "source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(source))), "native_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "native_base64": base64.StdEncoding.EncodeToString(raw), "actions": len(actions), "apple_runtime_observed": false})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_COMBINED_INDEX:" + string(receipt))
	return actions
}
func p15IXFaults(spec p15IXCase) []string {
	faults := []string{"output_bypasses_guarded_result", "collection_changed"}
	if spec.Mode == "real" && (spec.Value.(float64) < 0 || spec.Value.(float64) != float64(int64(spec.Value.(float64)))) {
		return append(faults, "unsafe_lookup_added", "absence_became_empty_text")
	}
	faults = append(faults, "lookup_before_bounds", "wrong_lookup_index", "wrong_collection", "upper_bound_strict", "fallback_not_none")
	if spec.Mode != "real" {
		faults = append(faults, "wrong_one_based", "lower_bound_threshold")
	}
	if spec.Mode != "int" && spec.Mode != "real" {
		faults = append(faults, "integral_divisor", "integral_input_diverged", "integral_branch_reversed")
	}
	return faults
}
func p15IXReject(t *testing.T, actions []map[string]any, id string, spec p15IXCase) {
	t.Helper()
	if _, err := p15IXGraph(actions, id, spec); err == nil {
		t.Fatal("damaged complete index graph accepted")
	}
}
func p15IXCaseFault(t *testing.T, actions []map[string]any, view p15AMView, id string, spec p15IXCase, fault string) {
	t.Helper()
	changed := p15AMClone(t, actions)
	params := func(mark string) map[string]any {
		index, ok := view.marks[mark]
		if !ok {
			t.Fatalf("fault did not select actual %s", mark)
		}
		return p15AMParams(changed[index])
	}
	switch fault {
	case "output_bypasses_guarded_result":
		params("output")["WFOutput"] = p15AMRefToken(view.results["collection"])
	case "collection_changed":
		if len(spec.Items) == 0 {
			params("empty_store")["WFInput"] = p15AMAttachment(p15AMVariable("foreign"))
		} else {
			params("collection")["WFItems"].([]any)[0].(map[string]any)["WFValue"] = "changed item"
		}
	case "unsafe_lookup_added", "absence_became_empty_text":
		index := view.marks["static_absence"]
		old := p15AMParams(changed[index])
		replacement := map[string]any{"UUID": old["UUID"], "CustomOutputName": "observed"}
		kind := "getitemfromlist"
		if fault == "unsafe_lookup_added" {
			replacement["WFInput"] = p15AMAttachment(view.results["collection"])
			replacement["WFItemSpecifier"] = "Item At Index"
			replacement["WFItemIndex"] = float64(1)
		} else {
			kind = "gettext"
			replacement["WFTextActionText"] = ""
		}
		changed[index] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + kind, "WFWorkflowActionParameters": replacement}
	case "lookup_before_bounds":
		from := view.marks["lookup"]
		to, ok := view.marks["lower_guard"]
		if !ok {
			to = view.marks["upper_guard"]
		}
		moved := changed[from]
		copy(changed[to+1:from+1], changed[to:from])
		changed[to] = moved
		if err := p15AMReferences(changed); err != nil {
			t.Fatal("guard-escape control failed earlier at producer ordering", err)
		}
	case "wrong_lookup_index":
		params("lookup")["WFItemIndex"] = float64(0)
	case "wrong_collection":
		params("lookup")["WFInput"] = p15AMAttachment(p15AMVariable("foreign"))
	case "upper_bound_strict":
		params("upper_guard")["WFCondition"] = uint64(2)
	case "fallback_not_none":
		changed[view.marks["upper_fallback"]] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.number", "WFWorkflowActionParameters": map[string]any{"WFNumberActionNumber": float64(0)}}
	case "wrong_one_based":
		params("one_based")["WFMathOperand"] = float64(2)
	case "lower_bound_threshold":
		params("lower_guard")["WFNumberValue"] = float64(0)
	case "integral_divisor":
		params("integral_modulo")["WFScientificMathOperand"] = float64(2)
	case "integral_input_diverged":
		params("integral_modulo")["WFInput"] = uint64(0)
	case "integral_branch_reversed":
		params("integral_guard")["WFCondition"] = uint64(5)
	default:
		t.Fatal("unknown fixed native mutation")
	}
	if reflect.DeepEqual(changed, actions) {
		t.Fatal("control did not mutate the actual compiled graph")
	}
	p15IXReject(t, changed, id, spec)
}

var p15IXIdentityFaults = []string{"consistent_alpha_uuid", "reconciled_malformed_uuid", "reconciled_zero_uuid", "reconciled_malformed_group", "reconciled_zero_group", "duplicate_uuid", "casefold_duplicate_uuid", "late_producer", "guard_native_integer_kind", "integral_native_real_kind", "extra_action", "missing_output", "lookup_outside_integral"}

func p15IXIdentityControls(t *testing.T, actions []map[string]any, view p15AMView, id string, spec p15IXCase) {
	for _, fault := range p15IXIdentityFaults {
		t.Run("identity/"+fault, func(t *testing.T) {
			changed := p15AMClone(t, actions)
			switch fault {
			case "consistent_alpha_uuid", "reconciled_malformed_uuid", "reconciled_zero_uuid":
				old := p15AMParams(changed[view.marks["collection_output"]])["UUID"].(string)
				replacement := "12345678-1234-4234-8234-123456789abc"
				if fault == "reconciled_malformed_uuid" {
					replacement = "not-a-native-uuid"
				}
				if fault == "reconciled_zero_uuid" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				count := 0
				for _, row := range changed {
					count += p15AMRewriteIdentity(row, old, replacement)
				}
				if count < 3 {
					t.Fatal("collection identity control did not reconcile both actual consumers")
				}
			case "reconciled_malformed_group", "reconciled_zero_group":
				old := p15AMParams(changed[view.marks["integral_guard"]])["GroupingIdentifier"].(string)
				replacement := "not-a-native-group"
				if fault == "reconciled_zero_group" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				count := 0
				for _, row := range changed {
					count += p15AMRewriteIdentity(row, old, replacement)
				}
				if count != 3 {
					t.Fatal("group control missed exact three branch boundaries")
				}
			case "duplicate_uuid", "casefold_duplicate_uuid":
				old := p15AMParams(changed[view.marks["collection_output"]])["UUID"].(string)
				if fault == "casefold_duplicate_uuid" {
					replacement := "abcdef01-2345-4678-89ab-cdef01234567"
					count := 0
					for _, row := range changed {
						count += p15AMRewriteIdentity(row, old, replacement)
					}
					if count < 3 {
						t.Fatal("case-fold duplicate control missed original collection consumers")
					}
					old = strings.ToUpper(replacement)
				}
				p15AMParams(changed[view.marks["integral_modulo"]])["UUID"] = old
			case "late_producer":
				i, j := view.marks["collection_output"], view.marks["lookup"]
				changed[i], changed[j] = changed[j], changed[i]
			case "guard_native_integer_kind":
				p15AMParams(changed[view.marks["integral_guard"]])["WFCondition"] = float64(4)
			case "integral_native_real_kind":
				p15AMParams(changed[view.marks["integral_modulo"]])["WFScientificMathOperand"] = uint64(1)
			case "extra_action":
				changed = append(changed, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			case "missing_output":
				changed = changed[:len(changed)-1]
			case "lookup_outside_integral":
				// Relocate the complete pure one-based/count/difference/lookup chain.
				// Its producers still precede every use; only the protecting ancestry
				// and required action order are removed, so a reference-only oracle fails.
				from := view.marks["one_based"]
				lookup := view.marks["lookup"]
				to := view.marks["integral_modulo"]
				lifted := append([]map[string]any{}, changed[from:from+3]...)
				lifted = append(lifted, changed[lookup])
				rest := append([]map[string]any{}, changed[:from]...)
				rest = append(rest, changed[from+3:lookup]...)
				rest = append(rest, changed[lookup+1:]...)
				changed = append(append(append([]map[string]any{}, rest[:to]...), lifted...), rest[to:]...)
				if err := p15AMReferences(changed); err != nil {
					t.Fatal("integral-escape control failed at identity instead of ancestry", err)
				}
			}
			if reflect.DeepEqual(changed, actions) {
				t.Fatal("identity control did not mutate actual graph")
			}
			if fault == "consistent_alpha_uuid" {
				if _, err := p15IXGraph(changed, id, spec); err != nil {
					t.Fatal("consistent canonical alpha identity rejected", err)
				}
			} else {
				p15IXReject(t, changed, id, spec)
			}
		})
	}
}

var p15IXArithmeticFaults = []string{"division_addition", "division_truncated", "modulus_addition", "modulus_wrong_divisor", "compound_operator", "compound_target", "compound_operand", "compound_store", "detached_initial_fraction", "detached_clipboard", "rounding_inserted", "integer_literal_changed_to_real"}

func p15IXArithmeticControls(t *testing.T, actions []map[string]any, view p15AMView, spec p15IXCase) {
	for _, fault := range p15IXArithmeticFaults {
		t.Run("arithmetic/"+fault, func(t *testing.T) {
			changed := p15AMClone(t, actions)
			p := func(mark string) map[string]any { return p15AMParams(changed[view.marks[mark]]) }
			switch fault {
			case "division_addition":
				p("fraction")["WFMathOperation"] = "+"
			case "division_truncated":
				params := p("fraction")
				changed[view.marks["fraction"]] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.number", "WFWorkflowActionParameters": map[string]any{"UUID": params["UUID"], "CustomOutputName": "fraction", "WFNumberActionNumber": uint64(2)}}
			case "modulus_addition":
				params := p("remainder")
				params["WFMathOperation"] = "+"
				params["WFMathOperand"] = params["WFScientificMathOperand"]
				delete(params, "WFScientificMathOperation")
				delete(params, "WFScientificMathOperand")
			case "modulus_wrong_divisor":
				p("remainder")["WFScientificMathOperand"] = uint64(2)
			case "compound_operator":
				p("compound_divide")["WFMathOperation"] = "+"
			case "compound_target":
				p("compound_add")["WFInput"] = p15AMAttachment(p15AMVariable("foreign"))
			case "compound_operand":
				p("compound_subtract")["WFMathOperand"] = uint64(0)
			case "compound_store":
				p("compound_multiply_store")["WFVariableName"] = "foreign"
			case "detached_initial_fraction":
				p("index_initial_store")["WFInput"] = uint64(2)
			case "detached_clipboard":
				p("clipboard")["WFInput"] = uint64(2)
			case "rounding_inserted":
				i := view.marks["index_initial_store"]
				changed = append(changed[:i], append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.round", "WFWorkflowActionParameters": map[string]any{}}}, changed[i:]...)...)
			case "integer_literal_changed_to_real":
				p("compound_multiply")["WFMathOperand"] = float64(2)
			}
			if reflect.DeepEqual(changed, actions) {
				t.Fatal("arithmetic control did not change actual graph")
			}
			p15IXReject(t, changed, "BRG16", spec)
		})
	}
}
func p15IXNative(t *testing.T, id string) {
	fixture := p15IXLoad(t, id)
	var identityActions []map[string]any
	var identityView p15AMView
	var identitySpec p15IXCase
	for i, spec := range p15IXCases[id] {
		t.Run(spec.Name, func(t *testing.T) {
			actions := p15IXCompile(t, id, spec.Name, fixture.Cases[i].Source)
			view, err := p15IXGraph(actions, id, spec)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range p15IXFaults(spec) {
				t.Run(fault, func(t *testing.T) { p15IXCaseFault(t, actions, view, id, spec, fault) })
			}
			if _, err = p15IXGraph(actions, id, spec); err != nil {
				t.Fatal("fault controls changed original compiled graph", err)
			}
			if spec.Name == "dynamic_last" {
				identityActions = actions
				identityView = view
				identitySpec = spec
			}
		})
	}
	if identityActions == nil {
		t.Fatal("no actual dynamic graph for fixed identity controls")
	}
	p15IXIdentityControls(t, identityActions, identityView, id, identitySpec)
	if id == "BRG16" {
		p15IXArithmeticControls(t, identityActions, identityView, identitySpec)
	}
	for _, invalid := range fixture.Invalid {
		t.Run("reject_"+invalid.Name, func(t *testing.T) { p15AMRejectTypedOperand(t, invalid.Source, invalid.Operand) })
	}
	t.Logf("%s distinct complete local sources retain exact Text? result references and all guard ancestors. Source-specified expected Text/None values and derived numeric results are not Apple runtime observations; ios-runtime remains pending.", id)
}
func TestP15RP21SafeIndexNative(t *testing.T)      { p15IXNative(t, "RP21") }
func TestP15BRG16NumericBoundsNative(t *testing.T) { p15IXNative(t, "BRG16") }
