package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

type p15SCExpr struct {
	kind, variable, marker string
	left, right            *p15SCExpr
}
type p15SCCase struct {
	name string
	expr *p15SCExpr
	want bool
}

func p15SCVar(truth bool) *p15SCExpr {
	name := "flagFalse"
	if truth {
		name = "flagTrue"
	}
	return &p15SCExpr{kind: "variable", variable: name}
}
func p15SCEffect(marker string, truth bool) *p15SCExpr {
	return &p15SCExpr{kind: "effect", variable: p15SCVar(truth).variable, marker: marker}
}
func p15SCAnd(left, right *p15SCExpr) *p15SCExpr {
	return &p15SCExpr{kind: "and", left: left, right: right}
}
func p15SCOr(left, right *p15SCExpr) *p15SCExpr {
	return &p15SCExpr{kind: "or", left: left, right: right}
}

// Fixed truth cases and mixed trees, independent of the compiled graph. These
// describe required branch placement; they do not emulate the Apple runtime.
func p15SCCases() []p15SCCase {
	return []p15SCCase{
		{"andFF", p15SCAnd(p15SCVar(false), p15SCEffect("AND_FF", false)), false},
		{"andFT", p15SCAnd(p15SCVar(false), p15SCEffect("AND_FT", true)), false},
		{"andTF", p15SCAnd(p15SCVar(true), p15SCEffect("AND_TF", false)), false},
		{"andTT", p15SCAnd(p15SCVar(true), p15SCEffect("AND_TT", true)), true},
		{"orFF", p15SCOr(p15SCVar(false), p15SCEffect("OR_FF", false)), false},
		{"orFT", p15SCOr(p15SCVar(false), p15SCEffect("OR_FT", true)), true},
		{"orTF", p15SCOr(p15SCVar(true), p15SCEffect("OR_TF", false)), true},
		{"orTT", p15SCOr(p15SCVar(true), p15SCEffect("OR_TT", true)), true},
		{"mixedAnd", p15SCAnd(p15SCVar(true), p15SCAnd(p15SCEffect("MIX_A", true), p15SCOr(p15SCEffect("MIX_B", false), p15SCEffect("MIX_C", true)))), true},
		{"mixedOr", p15SCOr(p15SCVar(false), p15SCOr(p15SCEffect("MIX_D", false), p15SCAnd(p15SCEffect("MIX_E", true), p15SCEffect("MIX_F", true)))), true},
		{"mixedSkip", p15SCOr(p15SCAnd(p15SCVar(false), p15SCEffect("SKIP_G", true)), p15SCAnd(p15SCVar(true), p15SCEffect("NEED_H", false))), false},
		{"mixedKeep", p15SCAnd(p15SCOr(p15SCVar(true), p15SCEffect("SKIP_I", false)), p15SCOr(p15SCVar(false), p15SCEffect("NEED_J", true))), true},
	}
}

const p15SCExpectedOrder = "|AND_TF|AND_TT|OR_FF|OR_FT|MIX_A|MIX_B|MIX_C|MIX_D|MIX_E|MIX_F|NEED_H|NEED_J"

type p15SCComparison struct {
	name, left, right, key string
	code                   uint64
	want                   bool
}

func p15SCComparisons() []p15SCComparison {
	return []p15SCComparison{
		{"textSameEqual", "textLeft", "textSame", "WFConditionalActionString", 4, true},
		{"textCaseEqual", "textLeft", "textCase", "WFConditionalActionString", 4, false},
		{"textSameNotEqual", "textLeft", "textSame", "WFConditionalActionString", 5, false},
		{"textCaseNotEqual", "textLeft", "textCase", "WFConditionalActionString", 5, true},
		{"numericEqual", "numberLeft", "numberRight", "WFNumberValue", 4, false},
		{"numericNotEqual", "numberLeft", "numberRight", "WFNumberValue", 5, true},
		{"numericLess", "numberLeft", "numberRight", "WFNumberValue", 0, true},
		{"numericLessEqual", "numberLeft", "numberRight", "WFNumberValue", 1, true},
		{"numericGreater", "numberLeft", "numberRight", "WFNumberValue", 2, false},
		{"numericGreaterEqual", "numberLeft", "numberRight", "WFNumberValue", 3, false},
	}
}
func p15CombinedShortCircuitSource(t *testing.T, typed bool) string {
	t.Helper()
	name := "v2-rp18-observable-short-circuit.cherri"
	if typed {
		name = "v2-brg13-typed-short-circuit.cherri"
	}
	data, err := os.ReadFile(filepath.Join("tests", "language-v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func p15SCVariable(name string) map[string]any {
	return map[string]any{"Type": "Variable", "VariableName": name}
}
func p15SCAttachment(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
}
func p15SCToken(text string, refs map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": text, "attachmentsByRange": refs}}
}
func p15SCRefToken(ref map[string]any) map[string]any {
	return p15SCToken("\ufffc", map[string]any{"{0, 1}": ref})
}
func p15SCParams(action map[string]any) map[string]any {
	parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return parameters
}
func p15SCCopy(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			result[key] = p15SCCopy(child)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for i, child := range value {
			result[i] = p15SCCopy(child)
		}
		return result
	default:
		return value
	}
}
func p15SCClone(t *testing.T, actions []map[string]any) []map[string]any {
	t.Helper()
	result := make([]map[string]any, len(actions))
	for i, action := range actions {
		result[i] = p15SCCopy(action).(map[string]any)
	}
	return result
}
func p15SCCanonicalUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	nonzero := false
	for i, ch := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if ch != '-' {
				return false
			}
			continue
		}
		if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
			return false
		}
		nonzero = nonzero || ch != '0'
	}
	return nonzero
}
func p15SCReferences(actions []map[string]any) error {
	positions := map[string]int{}
	for i, action := range actions {
		if raw, present := p15SCParams(action)["UUID"]; present {
			uuid, ok := raw.(string)
			if !ok || !p15SCCanonicalUUID(uuid) {
				return fmt.Errorf("invalid native UUID at %d", i)
			}
			if _, duplicate := positions[strings.ToLower(uuid)]; duplicate {
				return fmt.Errorf("duplicate native UUID at %d", i)
			}
			positions[strings.ToLower(uuid)] = i
		}
	}
	var walk func(any, int) error
	walk = func(value any, consumer int) error {
		switch v := value.(type) {
		case map[string]any:
			for key, child := range v {
				if key == "OutputUUID" {
					uuid, ok := child.(string)
					producer, found := positions[strings.ToLower(uuid)]
					if !ok || !found || producer >= consumer {
						return fmt.Errorf("missing or late native producer at %d", consumer)
					}
				}
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range v {
				if err := walk(child, consumer); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for i, action := range actions {
		if err := walk(p15SCParams(action), i); err != nil {
			return err
		}
	}
	return nil
}

type p15SCView struct {
	guards                                 map[string]int
	effects                                map[string][]int
	results                                map[string]map[string]any
	checks                                 map[string]int
	comparisons                            map[string]int
	initialCounter, finalText, finalOutput int
}
type p15SCCursor struct {
	actions []map[string]any
	index   int
	groups  map[string]bool
	view    p15SCView
	err     error
}

func (c *p15SCCursor) take(kind, name string, fields map[string]any, output bool) map[string]any {
	if c.err != nil {
		return nil
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing native %s action at %d", kind, c.index)
		return nil
	}
	action := c.actions[c.index]
	parameters := p15SCParams(action)
	ref := map[string]any(nil)
	if output {
		uuid, ok := parameters["UUID"].(string)
		if !ok || !p15SCCanonicalUUID(uuid) {
			c.err = fmt.Errorf("missing native %s output at %d", kind, c.index)
			return nil
		}
		fields["UUID"] = uuid
		ref = map[string]any{"Type": "ActionOutput", "OutputUUID": uuid}
		if name != "" {
			ref["OutputName"] = name
		}
	}
	if name != "" {
		fields["CustomOutputName"] = name
	}
	if len(action) != 2 || action["WFWorkflowActionIdentifier"] != "is.workflow.actions."+kind || !reflect.DeepEqual(parameters, fields) {
		c.err = fmt.Errorf("native %s action differs at %d: got %#v want %#v", kind, c.index, parameters, fields)
		return nil
	}
	c.index++
	return ref
}
func (c *p15SCCursor) start(input map[string]any, code uint64, key string, right any) string {
	if c.err != nil {
		return ""
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing native condition")
		return ""
	}
	group, ok := p15SCParams(c.actions[c.index])["GroupingIdentifier"].(string)
	if !ok || !p15SCCanonicalUUID(group) {
		c.err = fmt.Errorf("invalid native condition group")
		return ""
	}
	if c.groups[strings.ToLower(group)] {
		c.err = fmt.Errorf("missing or reused native condition group")
		return ""
	}
	c.groups[strings.ToLower(group)] = true
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(0), "WFCondition": code, "WFConditionalLegacyComparisonBehavior": true, "WFInput": map[string]any{"Type": "Variable", "Variable": p15SCAttachment(input)}, key: right}, false)
	return group
}
func (c *p15SCCursor) alternate(group string) {
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(1)}, false)
}
func (c *p15SCCursor) end(group, name string, output bool) map[string]any {
	return c.take("conditional", name, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(2)}, output)
}

// Integer source literals and native control codes preserve their plist integer kind.
// Boolean value carriers below are explicitly real 0/1, paired with semantic Bool tests.
func (c *p15SCCursor) number(value uint64, name string, output bool) map[string]any {
	return c.take("number", name, map[string]any{"WFNumberActionNumber": value}, output)
}
func (c *p15SCCursor) boolean(value float64, name string, output bool) map[string]any {
	return c.take("number", name, map[string]any{"WFNumberActionNumber": value}, output)
}
func (c *p15SCCursor) store(name string, ref map[string]any) {
	c.take("setvariable", "", map[string]any{"WFVariableName": name, "WFInput": p15SCAttachment(ref)}, false)
}
func (c *p15SCCursor) comparison(name string, left map[string]any, code uint64, key string, right any) map[string]any {
	c.view.comparisons[name] = c.index
	group := c.start(left, code, key, right)
	c.boolean(1, "", false)
	c.alternate(group)
	c.boolean(0, "", false)
	return c.end(group, name, true)
}
func (c *p15SCCursor) effect(marker, variable, owner string) map[string]any {
	group := c.start(p15SCVariable(variable), 4, "WFNumberValue", float64(1))
	for branch := 0; branch < 2; branch++ {
		if branch == 1 {
			c.alternate(group)
		}
		c.view.effects[owner] = append(c.view.effects[owner], c.index)
		counter := c.take("math", "", map[string]any{"WFInput": p15SCAttachment(p15SCVariable("calls")), "WFMathOperation": "+", "WFMathOperand": uint64(1)}, true)
		c.store("calls", counter)
		order := c.take("gettext", "Text", map[string]any{"WFTextActionText": p15SCToken("\ufffc|"+marker, map[string]any{"{0, 1}": p15SCVariable("order")})}, true)
		c.store("order", order)
		c.boolean(float64(1-branch), "", true)
	}
	return c.end(group, "", true)
}
func (c *p15SCCursor) expression(expr *p15SCExpr, path, owner, name string) map[string]any {
	switch expr.kind {
	case "variable":
		return p15SCVariable(expr.variable)
	case "effect":
		return c.effect(expr.marker, expr.variable, owner)
	case "and", "or":
		left := c.expression(expr.left, path+"/left", owner, "")
		c.view.guards[path] = c.index
		group := c.start(left, 4, "WFNumberValue", float64(1))
		if expr.kind == "and" {
			c.expression(expr.right, path+"/right", owner, "")
			c.alternate(group)
			c.boolean(0, "", false)
		} else {
			c.boolean(1, "", false)
			c.alternate(group)
			c.expression(expr.right, path+"/right", owner, "")
		}
		return c.end(group, name, true)
	}
	c.err = fmt.Errorf("unknown independent expression")
	return nil
}
func (c *p15SCCursor) check(name string, ref map[string]any, key string, want any) {
	c.view.checks[name] = c.index
	group := c.start(ref, 5, key, want)
	marker := c.take("gettext", "Text", map[string]any{"WFTextActionText": "FAIL_" + name}, true)
	c.store("verdict", marker)
	c.end(group, "", false)
}
func p15SCBoolNumber(value bool) float64 {
	if value {
		return 1
	}
	return 0
}

// The cursor consumes every action in the independent contract order. Recursive
// regions bind all ancestors, each counter/order write, skipped values, typed
// comparison operands and the actual returned observations. No native value is
// executed or inferred here; Apple Bool transport and case sensitivity stay open.
func p15SCGraph(actions []map[string]any, typed bool) (p15SCView, error) {
	view := p15SCView{guards: map[string]int{}, effects: map[string][]int{}, results: map[string]map[string]any{}, checks: map[string]int{}, comparisons: map[string]int{}}
	if err := p15SCReferences(actions); err != nil {
		return view, err
	}
	c := &p15SCCursor{actions: actions, groups: map[string]bool{}, view: view}
	zero := c.number(0, "zero", true)
	one := c.number(1, "one", true)
	for _, seed := range []struct {
		name string
		ref  map[string]any
	}{{"flagFalse", zero}, {"flagTrue", one}} {
		group := c.start(seed.ref, 4, "WFNumberValue", uint64(1))
		c.boolean(1, "", false)
		c.alternate(group)
		c.boolean(0, "", false)
		c.store(seed.name, c.end(group, "", true))
	}
	if typed {
		refs := map[string]map[string]any{}
		for _, seed := range []struct{ name, value string }{{"textLeft", "abc"}, {"textSame", "abc"}, {"textCase", "ABC"}} {
			refs[seed.name] = c.take("gettext", seed.name, map[string]any{"WFTextActionText": seed.value}, true)
		}
		refs["numberLeft"] = c.number(2, "numberLeft", true)
		refs["numberRight"] = c.number(3, "numberRight", true)
		for _, spec := range p15SCComparisons() {
			right := any(p15SCAttachment(refs[spec.right]))
			if spec.key == "WFConditionalActionString" {
				right = p15SCRefToken(refs[spec.right])
			}
			c.view.results[spec.name] = c.comparison(spec.name, refs[spec.left], spec.code, spec.key, right)
		}
		c.view.results["foldedCaseEqual"] = c.boolean(0, "foldedCaseEqual", true)
	}
	c.view.initialCounter = c.index
	c.store("calls", c.number(0, "Number", true))
	c.store("order", c.take("gettext", "Text", map[string]any{"WFTextActionText": ""}, true))
	success := "P15_RP18_LOCAL_OK"
	if typed {
		success = "P15_BRG13_LOCAL_OK"
	}
	c.store("verdict", c.take("gettext", "Text", map[string]any{"WFTextActionText": success}, true))
	for _, spec := range p15SCCases() {
		c.view.results[spec.name] = c.expression(spec.expr, spec.name, spec.name, spec.name)
	}
	for _, spec := range p15SCCases() {
		c.check(spec.name, c.view.results[spec.name], "WFNumberValue", p15SCBoolNumber(spec.want))
	}
	if typed {
		for _, spec := range p15SCComparisons() {
			c.check(spec.name, c.view.results[spec.name], "WFNumberValue", p15SCBoolNumber(spec.want))
		}
		c.check("foldedCaseEqual", c.view.results["foldedCaseEqual"], "WFNumberValue", float64(0))
	}
	c.check("CALLS", p15SCVariable("calls"), "WFNumberValue", uint64(12))
	c.check("ORDER", p15SCVariable("order"), "WFConditionalActionString", p15SCExpectedOrder)
	c.view.finalText = c.index
	result := c.take("gettext", "Text", map[string]any{"WFTextActionText": p15SCToken("\ufffc:calls=\ufffc:order=\ufffc", map[string]any{"{0, 1}": p15SCVariable("verdict"), "{8, 1}": p15SCVariable("calls"), "{16, 1}": p15SCVariable("order")})}, true)
	c.view.finalOutput = c.index
	c.take("output", "", map[string]any{"WFOutput": p15SCRefToken(result)}, false)
	if c.err == nil && c.index != len(actions) {
		c.err = fmt.Errorf("unexpected native actions after complete scenario")
	}
	return c.view, c.err
}

func p15SCRequireType(t *testing.T, expr syntax.Expression, want types.Type, analyzer *analysis.Analyzer) {
	t.Helper()
	got := analyzer.ExprTypes()[expr]
	if got == nil || !got.Equals(want) {
		t.Fatalf("typed condition/effect value: got %v want %v", got, want)
	}
}
func p15SCTypeTree(t *testing.T, expr syntax.Expression, spec *p15SCExpr, analyzer *analysis.Analyzer) {
	t.Helper()
	p15SCRequireType(t, expr, types.Bool, analyzer)
	switch spec.kind {
	case "variable":
		ident, ok := expr.(*syntax.IdentExpr)
		if !ok || ident.Name != spec.variable {
			t.Fatal("wrong typed guard binding")
		}
	case "effect":
		helper, ok := expr.(*syntax.IfStmt)
		if !ok {
			t.Fatal("RHS is not the self-contained expression helper")
		}
		p15SCTypeTree(t, helper.Condition, &p15SCExpr{kind: "variable", variable: spec.variable}, analyzer)
		other, ok := helper.ElseBlock.(*syntax.BlockStmt)
		if !ok || len(helper.ThenBlock.Statements) != 3 || len(other.Statements) != 3 {
			t.Fatal("incomplete RHS branches")
		}
		for _, block := range []*syntax.BlockStmt{helper.ThenBlock, other} {
			update, ok := block.Statements[0].(*syntax.AssignStmt)
			if !ok || update.Name != "calls" {
				t.Fatal("missing counter update")
			}
			p15SCRequireType(t, update.Value, types.Number, analyzer)
			order, ok := block.Statements[1].(*syntax.AssignStmt)
			if !ok || order.Name != "order" {
				t.Fatal("missing order update")
			}
			p15SCRequireType(t, order.Value, types.Text, analyzer)
			yielded, ok := block.Statements[2].(*syntax.YieldStmt)
			if !ok {
				t.Fatal("missing RHS yield")
			}
			p15SCRequireType(t, yielded.Value, types.Bool, analyzer)
		}
	default:
		binary, ok := expr.(*syntax.BinaryExpr)
		op := syntax.TokenAnd
		if spec.kind == "or" {
			op = syntax.TokenOr
		}
		if !ok || binary.Op != op {
			t.Fatal("mixed Boolean source tree changed")
		}
		p15SCTypeTree(t, binary.Left, spec.left, analyzer)
		p15SCTypeTree(t, binary.Right, spec.right, analyzer)
	}
}
func p15SCUnit(t *testing.T, typed bool) {
	source := p15CombinedShortCircuitSource(t, typed)
	program, analyzer := p15T08Analyzer(t, source)
	for _, spec := range p15SCCases() {
		t.Run(spec.name, func(t *testing.T) { p15SCTypeTree(t, p15BindingValue(t, program, spec.name), spec.expr, analyzer) })
	}
	expected := map[string]types.Type{"zero": types.Number, "one": types.Number, "flagFalse": types.Bool, "flagTrue": types.Bool, "calls": types.Number, "order": types.Text, "verdict": types.Text}
	if typed {
		for _, name := range []string{"textLeft", "textSame", "textCase"} {
			expected[name] = types.Text
		}
		for _, name := range []string{"numberLeft", "numberRight"} {
			expected[name] = types.Number
		}
		for _, spec := range p15SCComparisons() {
			expected[spec.name] = types.Bool
		}
		expected["foldedCaseEqual"] = types.Bool
	}
	names := make([]string, 0, len(expected))
	for name := range expected {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run("type_"+name, func(t *testing.T) { p15SCRequireType(t, p15BindingValue(t, program, name), expected[name], analyzer) })
	}
	negatives := []struct{ name, before, after string }{
		{"number_is_not_bool", "var flagFalse: Bool = zero == 1", "var flagFalse: Bool = zero"},
		{"text_is_not_bool", "var flagTrue: Bool = one == 1", "var flagTrue: Bool = text(\"1\")"},
		{"counter_is_not_text", "var calls = 0", "var calls: Number = \"0\""},
		{"order_is_not_number", "var order = \"\"", "var order: Text = 0"},
	}
	if typed {
		negatives = append(negatives, struct{ name, before, after string }{"text_is_not_ordered_number", "let numberLeft = number(2)", "let numberLeft = text(\"2\")"})
		negatives = append(negatives, struct{ name, before, after string }{"bool_is_not_ordered_number", "let numberRight = number(3)", "let numberRight = true"})
		negatives = append(negatives, struct{ name, before, after string }{"unknown_is_not_ordered_number", "let numberLeft = number(2)", "let numberLeft = ShortcutInput"})
	}
	for _, fault := range negatives {
		t.Run(fault.name, func(t *testing.T) {
			changed := strings.Replace(source, fault.before, fault.after, 1)
			if changed == source {
				t.Fatal("source control did not change")
			}
			found := false
			for _, d := range p15TypeDiagnostics(t, changed) {
				found = found || d.Severity == analysis.SeverityError
			}
			data, err := CompileSourceToPlist("invalid-combined-short-circuit.cherri", changed)
			if !found || err == nil || len(data) != 0 {
				t.Fatal("invalid typed input emitted a workflow")
			}
		})
	}
}
func TestP15RP18ObservableShortCircuitUnit(t *testing.T)      { p15SCUnit(t, false) }
func TestBackendRecovery13TypedShortCircuitUnit(t *testing.T) { p15SCUnit(t, true) }
func p15SCNative(t *testing.T, typed bool) {
	source := p15CombinedShortCircuitSource(t, typed)
	actions := p15CompiledActions(t, source)
	kinds := map[string]string{}
	for _, action := range actions {
		for key, value := range p15SCParams(action) {
			if key == "WFNumberActionNumber" || key == "WFCondition" || key == "WFControlFlowMode" || key == "WFMathOperand" || key == "WFNumberValue" {
				kinds[key] = fmt.Sprintf("%T", value)
			}
		}
	}
	t.Logf("decoded native numeric kinds: %v", kinds)
	view, err := p15SCGraph(actions, typed)
	if err != nil {
		t.Fatal(err)
	}
	if directory := os.Getenv("CHERRI_COMBINED_SHORT_CIRCUIT_DUMP"); directory != "" {
		if err := os.MkdirAll(directory, 0755); err != nil {
			t.Fatal(err)
		}
		name := "rp18-actions.json"
		if typed {
			name = "brg13-actions.json"
		}
		data, err := json.MarshalIndent(actions, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(filepath.Join(directory, name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	p15SCNativeCorruptions(t, actions, view, typed)
	t.Logf("Complete independent local graph: %d actions; Apple effect counts, order, Bool transport and dynamic Text case sensitivity not executed", len(actions))
}
func TestP15RP18ObservableShortCircuitNative(t *testing.T)      { p15SCNative(t, false) }
func TestBackendRecovery13TypedShortCircuitNative(t *testing.T) { p15SCNative(t, true) }

func p15SCResultIndex(t *testing.T, actions []map[string]any, ref map[string]any) int {
	t.Helper()
	for i, action := range actions {
		if p15SCParams(action)["UUID"] == ref["OutputUUID"] {
			return i
		}
	}
	t.Fatal("positive control result missing")
	return -1
}
func p15SCReject(t *testing.T, actions []map[string]any, typed bool, cause string) {
	t.Helper()
	_, err := p15SCGraph(actions, typed)
	if err == nil || !strings.Contains(err.Error(), cause) {
		t.Fatalf("native control did not reach %q: %v", cause, err)
	}
}
func p15SCNativeCorruptions(t *testing.T, actions []map[string]any, view p15SCView, typed bool) {
	zero := map[string]any{"Type": "ActionOutput", "OutputUUID": p15SCParams(actions[0])["UUID"], "OutputName": "zero"}
	for _, spec := range p15SCCases() {
		for _, fault := range []string{"outer_guard", "counter_twice", "order_marker", "hidden_result"} {
			t.Run(spec.name+"/"+fault, func(t *testing.T) {
				broken := p15SCClone(t, actions)
				effect := view.effects[spec.name][0]
				cause := ""
				switch fault {
				case "outer_guard":
					p15SCParams(broken[view.guards[spec.name]])["WFCondition"] = uint64(5)
					cause = "native conditional action differs"
				case "counter_twice":
					p15SCParams(broken[effect])["WFMathOperand"] = uint64(2)
					cause = "native math action differs"
				case "order_marker":
					p15SCParams(broken[effect+2])["WFTextActionText"] = p15SCToken("\ufffc|WRONG", map[string]any{"{0, 1}": p15SCVariable("order")})
					cause = "native gettext action differs"
				case "hidden_result":
					p15SCParams(broken[view.checks[spec.name]])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15SCAttachment(zero)}
					cause = "native conditional action differs"
				}
				if err := p15SCReferences(broken); err != nil {
					t.Fatal("mutation accidentally broke graph identity before intended gate", err)
				}
				p15SCReject(t, broken, typed, cause)
			})
		}
	}
	for _, fault := range []string{"initial_counter", "skipped_result", "bool_carrier", "eager_effect", "duplicate_effect", "mixed_outer_producer", "crosswired_store", "late_counter_producer", "count_observer", "order_observer", "hidden_output", "missing_output", "extra_effect", "foreign_action", "stale_group", "malformed_uuid", "zero_uuid", "malformed_group", "zero_group", "wrong_native_numeric_kind", "casefold_uuid_collision", "casefold_group_collision"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15SCClone(t, actions)
			first := view.effects["andFF"][0]
			cause := ""
			switch fault {
			case "casefold_uuid_collision":
				// First prove that a valid, reconciled identity rename is accepted.
				// Then collide a second real producer with its case-only alias.
				// Fixed letters avoid depending on the random UUID's hex digits.
				base := "4fe62a08-dbd6-4f58-afd0-000000000002"
				old := p15SCParams(broken[0])["UUID"].(string)
				if p15SCReplaceIdentity(broken, old, base) != 2 {
					t.Fatal("valid producer rename did not reconcile owner and consumer")
				}
				if _, err := p15SCGraph(broken, typed); err != nil {
					t.Fatal("valid producer identity rename rejected", err)
				}
				old = p15SCParams(broken[first])["UUID"].(string)
				if p15SCReplaceIdentity(broken, old, strings.ToUpper(base)) != 2 {
					t.Fatal("case-only producer collision did not reconcile owner and consumer")
				}
				cause = "duplicate native UUID"
			case "casefold_group_collision":
				base := "5fe62a08-dbd6-4f58-afd0-000000000003"
				old := p15SCParams(broken[2])["GroupingIdentifier"].(string)
				if p15SCReplaceIdentity(broken, old, base) != 3 {
					t.Fatal("valid group rename did not reconcile start/else/end")
				}
				if _, err := p15SCGraph(broken, typed); err != nil {
					t.Fatal("valid group identity rename rejected", err)
				}
				old = p15SCParams(broken[view.guards["andFF"]])["GroupingIdentifier"].(string)
				if p15SCReplaceIdentity(broken, old, strings.ToUpper(base)) != 3 {
					t.Fatal("case-only group collision did not reconcile start/else/end")
				}
				cause = "missing or reused native condition group"
			case "malformed_uuid", "zero_uuid":
				old, _ := p15SCParams(broken[first])["UUID"].(string)
				replacement := "not-a-native-uuid"
				if fault == "zero_uuid" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				if p15SCReplaceIdentity(broken, old, replacement) != 2 {
					t.Fatal("producer corruption did not reconcile owner and consumer")
				}
				cause = "invalid native UUID"
			case "malformed_group", "zero_group":
				old, _ := p15SCParams(broken[view.guards["andFF"]])["GroupingIdentifier"].(string)
				replacement := "not-a-native-group"
				if fault == "zero_group" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				if p15SCReplaceIdentity(broken, old, replacement) != 3 {
					t.Fatal("group corruption did not reconcile start/else/end")
				}
				cause = "invalid native condition group"
			case "wrong_native_numeric_kind":
				params := p15SCParams(broken[view.guards["andFF"]])
				if original, ok := params["WFControlFlowMode"].(uint64); !ok || original != 0 {
					t.Fatal("positive native control mode is not integer zero")
				}
				params["WFControlFlowMode"] = float64(0)
				cause = "native conditional action differs"
			case "initial_counter":
				p15SCParams(broken[view.initialCounter])["WFNumberActionNumber"] = uint64(1)
				cause = "native number action differs"
			case "skipped_result":
				end := p15SCResultIndex(t, actions, view.results["andFF"])
				p15SCParams(broken[end-1])["WFNumberActionNumber"] = float64(1)
				cause = "native number action differs"
			case "bool_carrier":
				p15SCParams(broken[first+4])["WFNumberActionNumber"] = true
				cause = "native number action differs"
			case "eager_effect":
				pair := append([]map[string]any(nil), broken[first:first+2]...)
				broken = append(broken[:first], broken[first+2:]...)
				before := view.guards["andFF"]
				tail := append([]map[string]any(nil), broken[before:]...)
				broken = append(append(broken[:before], pair...), tail...)
				cause = "invalid native condition group"
			case "duplicate_effect":
				pair := p15SCClone(t, broken[first:first+2])
				p15SCParams(pair[0])["UUID"] = "4fe62a08-dbd6-4f58-afd0-000000000001"
				p15SCParams(pair[1])["WFInput"] = p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": "4fe62a08-dbd6-4f58-afd0-000000000001"})
				tail := append([]map[string]any(nil), broken[first+2:]...)
				broken = append(append(broken[:first+2], pair...), tail...)
				cause = "native gettext action differs"
			case "mixed_outer_producer":
				// The innermost branches/groups are unchanged. Only the
				// ancestor's actual Boolean producer is replaced by an earlier
				// valid Bool output, so simple innermost-mode checks miss this.
				p15SCParams(broken[view.guards["mixedAnd/right"]])["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15SCAttachment(view.results["andFF"])}
				cause = "native conditional action differs"
			case "crosswired_store":
				p15SCParams(broken[first+1])["WFInput"] = p15SCAttachment(zero)
				cause = "native setvariable action differs"
			case "late_counter_producer":
				broken[first], broken[first+1] = broken[first+1], broken[first]
				cause = "missing or late native producer"
			case "count_observer":
				p15SCParams(broken[view.checks["CALLS"]])["WFNumberValue"] = uint64(11)
				cause = "native conditional action differs"
			case "order_observer":
				p15SCParams(broken[view.checks["ORDER"]])["WFConditionalActionString"] = strings.Replace(p15SCExpectedOrder, "|MIX_A|MIX_B", "|MIX_B|MIX_A", 1)
				cause = "native conditional action differs"
			case "hidden_output":
				p15SCParams(broken[view.finalText])["WFTextActionText"] = "P15_RP18_LOCAL_OK:calls=12"
				cause = "native gettext action differs"
			case "missing_output":
				broken = broken[:view.finalOutput]
				cause = "missing native output action"
			case "extra_effect":
				broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.comment", "WFWorkflowActionParameters": map[string]any{"WFCommentActionText": "foreign"}})
				cause = "unexpected native actions after complete scenario"
			case "foreign_action":
				broken[first]["WFWorkflowActionIdentifier"] = "is.workflow.actions.runworkflow"
				cause = "native math action differs"
			case "stale_group":
				end := p15SCResultIndex(t, actions, view.results["andFF"])
				p15SCParams(broken[end-2])["GroupingIdentifier"] = "FOREIGN-GROUP"
				cause = "native conditional action differs"
			}
			if fault != "late_counter_producer" && fault != "malformed_uuid" && fault != "zero_uuid" && fault != "casefold_uuid_collision" {
				if err := p15SCReferences(broken); err != nil {
					t.Fatal("mutation broke identity before intended causal gate", err)
				}
			}
			p15SCReject(t, broken, typed, cause)
		})
	}
	if !typed {
		return
	}
	for _, spec := range p15SCComparisons() {
		for _, fault := range []string{"operator", "right_producer"} {
			t.Run(spec.name+"/"+fault, func(t *testing.T) {
				broken := p15SCClone(t, actions)
				params := p15SCParams(broken[view.comparisons[spec.name]])
				if fault == "operator" {
					code := uint64(4)
					if spec.code == code {
						code = 5
					}
					params["WFCondition"] = code
				} else {
					left := p15LocalAttachment(params["WFInput"])
					if spec.key == "WFConditionalActionString" {
						params[spec.key] = p15SCRefToken(left)
					} else {
						params[spec.key] = p15SCAttachment(left)
					}
				}
				if err := p15SCReferences(broken); err != nil {
					t.Fatal(err)
				}
				p15SCReject(t, broken, true, "native conditional action differs")
			})
		}
	}
	for _, fault := range []string{"number_operand_as_text", "text_operand_as_number", "comparison_as_addition", "folded_case_equal"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15SCClone(t, actions)
			cause := "native conditional action differs"
			switch fault {
			case "number_operand_as_text":
				p15SCParams(broken[view.comparisons["numericLess"]])["WFNumberValue"] = "3"
			case "text_operand_as_number":
				p15SCParams(broken[view.comparisons["textCaseEqual"]])["WFNumberValue"] = uint64(0)
			case "comparison_as_addition":
				index := view.comparisons["numericLess"]
				broken[index]["WFWorkflowActionIdentifier"] = "is.workflow.actions.math"
				p15SCParams(broken[index])["WFMathOperation"] = "+"
			case "folded_case_equal":
				index := p15SCResultIndex(t, actions, view.results["foldedCaseEqual"])
				p15SCParams(broken[index])["WFNumberActionNumber"] = float64(1)
				cause = "native number action differs"
			}
			p15SCReject(t, broken, true, cause)
		})
	}
}

func p15SCReplaceIdentity(actions []map[string]any, old, replacement string) int {
	count := 0
	var walk func(any)
	walk = func(value any) {
		switch value := value.(type) {
		case map[string]any:
			for key, child := range value {
				if (key == "UUID" || key == "OutputUUID" || key == "GroupingIdentifier") && child == old {
					value[key] = replacement
					count++
				} else {
					walk(child)
				}
			}
		case []any:
			for _, child := range value {
				walk(child)
			}
		}
	}
	for _, action := range actions {
		walk(action)
	}
	return count
}
