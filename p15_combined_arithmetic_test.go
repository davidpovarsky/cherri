package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"howett.net/plist"
)

func p15AMVariable(name string) map[string]any {
	return map[string]any{"Type": "Variable", "VariableName": name}
}
func p15AMAttachment(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
}
func p15AMToken(text string, refs map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": text, "attachmentsByRange": refs}}
}
func p15AMRefToken(ref map[string]any) map[string]any {
	return p15AMToken("\ufffc", map[string]any{"{0, 1}": ref})
}
func p15AMParams(action map[string]any) map[string]any {
	parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return parameters
}
func p15AMCopy(value any) any {
	switch value := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(value))
		for key, child := range value {
			result[key] = p15AMCopy(child)
		}
		return result
	case []any:
		result := make([]any, len(value))
		for i, child := range value {
			result[i] = p15AMCopy(child)
		}
		return result
	default:
		return value
	}
}
func p15AMClone(t *testing.T, actions []map[string]any) []map[string]any {
	t.Helper()
	result := make([]map[string]any, len(actions))
	for i, action := range actions {
		result[i] = p15AMCopy(action).(map[string]any)
	}
	return result
}
func p15AMCanonicalUUID(value string) bool {
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
func p15AMReferences(actions []map[string]any) error {
	positions := map[string]int{}
	for i, action := range actions {
		if raw, present := p15AMParams(action)["UUID"]; present {
			uuid, ok := raw.(string)
			if !ok || !p15AMCanonicalUUID(uuid) {
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
		if err := walk(p15AMParams(action), i); err != nil {
			return err
		}
	}
	return nil
}

type p15AMView struct {
	marks   map[string]int
	results map[string]map[string]any
}
type p15AMCursor struct {
	actions []map[string]any
	index   int
	groups  map[string]bool
	view    p15AMView
	err     error
}

func (c *p15AMCursor) take(kind, name string, fields map[string]any, output bool) map[string]any {
	if c.err != nil {
		return nil
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing native %s action at %d", kind, c.index)
		return nil
	}
	action := c.actions[c.index]
	parameters := p15AMParams(action)
	ref := map[string]any(nil)
	if output {
		uuid, ok := parameters["UUID"].(string)
		if !ok || !p15AMCanonicalUUID(uuid) {
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
func (c *p15AMCursor) start(input map[string]any, code uint64, key string, right any) string {
	if c.err != nil {
		return ""
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing native condition")
		return ""
	}
	group, ok := p15AMParams(c.actions[c.index])["GroupingIdentifier"].(string)
	if !ok || !p15AMCanonicalUUID(group) {
		c.err = fmt.Errorf("invalid native condition group")
		return ""
	}
	if c.groups[strings.ToLower(group)] {
		c.err = fmt.Errorf("missing or reused native condition group")
		return ""
	}
	c.groups[strings.ToLower(group)] = true
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(0), "WFCondition": code, "WFConditionalLegacyComparisonBehavior": true, "WFInput": map[string]any{"Type": "Variable", "Variable": p15AMAttachment(input)}, key: right}, false)
	return group
}

func (c *p15AMCursor) mark(name string) { c.view.marks[name] = c.index }
func (c *p15AMCursor) alternate(group string) {
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(1)}, false)
}
func (c *p15AMCursor) end(group, name string, output bool) map[string]any {
	return c.take("conditional", name, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(2)}, output)
}
func (c *p15AMCursor) number(value any, name string, output bool) map[string]any {
	return c.take("number", name, map[string]any{"WFNumberActionNumber": value}, output)
}
func (c *p15AMCursor) store(name string, ref map[string]any) {
	c.take("setvariable", "", map[string]any{"WFVariableName": name, "WFInput": p15AMAttachment(ref)}, false)
}
func (c *p15AMCursor) math(mark, name, operation string, left, right any) map[string]any {
	c.mark(mark)
	return c.take("math", name, map[string]any{"WFMathOperation": operation, "WFInput": left, "WFMathOperand": right}, true)
}
func (c *p15AMCursor) comparison(name string, left map[string]any, code uint64, right any) map[string]any {
	c.mark(name)
	group := c.start(left, code, "WFNumberValue", right)
	c.mark(name + "_true")
	c.number(float64(1), "", false)
	c.alternate(group)
	c.mark(name + "_false")
	c.number(float64(0), "", false)
	return c.end(group, name, true)
}
func p15AMSource(t *testing.T, id string) string {
	t.Helper()
	name := map[string]string{"RP15": "v2-rp15-compound-once.cherri", "RP17": "v2-rp17-comparisons-arithmetic.cherri"}[id]
	if name == "" {
		t.Fatal("unknown arithmetic requirement")
	}
	raw, err := os.ReadFile(filepath.Join("tests", "language-v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
func p15AMGraph(actions []map[string]any, id string) (p15AMView, error) {
	view := p15AMView{marks: map[string]int{}, results: map[string]map[string]any{}}
	if err := p15AMReferences(actions); err != nil {
		return view, err
	}
	c := &p15AMCursor{actions: actions, groups: map[string]bool{}, view: view}
	if id == "RP15" {
		selector := c.number(uint64(1), "selector", true)
		c.mark("initial_accumulator")
		c.store("accumulator", c.number(uint64(10), "Number", true))
		c.mark("initial_calls")
		c.store("calls", c.number(uint64(0), "Number1", true))
		c.mark("initial_order")
		c.store("order", c.number(uint64(0), "Number2", true))
		for i, step := range []struct {
			name, operation string
			value           uint64
		}{{"add", "+", 3}, {"multiply", "×", 2}, {"subtract", "-", 4}, {"divide", "÷", 2}} {
			c.mark(step.name + "_guard")
			group := c.start(selector, 4, "WFNumberValue", float64(1))
			for branch := 0; branch < 2; branch++ {
				if branch == 1 {
					c.alternate(group)
				}
				prefix := fmt.Sprintf("%s_branch%d", step.name, branch)
				count := c.math(prefix+"_counter", "", "+", p15AMAttachment(p15AMVariable("calls")), uint64(1))
				c.mark(prefix + "_counter_store")
				c.store("calls", count)
				tens := c.math(prefix+"_order_times", "", "×", p15AMAttachment(p15AMVariable("order")), uint64(10))
				order := c.math(prefix+"_order_add", "", "+", p15AMAttachment(tens), uint64(i+1))
				c.mark(prefix + "_order_store")
				c.store("order", order)
				c.mark(prefix + "_yield")
				c.number(step.value, fmt.Sprintf("Number%d", 3+2*i+branch), true)
			}
			rhs := c.end(group, "", true)
			value := c.math(step.name+"_compound", "", step.operation, p15AMAttachment(p15AMVariable("accumulator")), p15AMAttachment(rhs))
			c.mark(step.name + "_store")
			c.store("accumulator", value)
		}
		count := c.math("observation_count", "", "×", p15AMAttachment(p15AMVariable("calls")), uint64(10000))
		observation := c.math("observation_order", "", "+", p15AMAttachment(count), p15AMAttachment(p15AMVariable("order")))
		c.mark("clipboard")
		c.take("setclipboard", "", map[string]any{"WFInput": p15AMAttachment(observation)}, false)
		c.mark("output")
		c.take("output", "", map[string]any{"WFOutput": p15AMRefToken(p15AMVariable("accumulator"))}, false)
	} else if id == "RP17" {
		refs := map[string]map[string]any{}
		for _, seed := range []struct {
			name  string
			value uint64
		}{{"two", 2}, {"three", 3}, {"five", 5}, {"ten", 10}} {
			c.mark("seed_" + seed.name)
			refs[seed.name] = c.number(seed.value, seed.name, true)
		}
		c.view.results["greater"] = c.comparison("greater", refs["two"], 2, p15AMAttachment(refs["three"]))
		c.view.results["equal"] = c.comparison("equal", refs["two"], 4, p15AMAttachment(refs["two"]))
		c.view.results["unequal"] = c.comparison("unequal", refs["two"], 5, p15AMAttachment(refs["two"]))
		c.view.results["divided"] = c.math("divided", "divided", "÷", p15AMAttachment(refs["five"]), p15AMAttachment(refs["two"]))
		c.mark("remainder")
		c.view.results["remainder"] = c.take("math", "remainder", map[string]any{"WFMathOperation": "…", "WFScientificMathOperation": "Modulus", "WFInput": p15AMAttachment(refs["ten"]), "WFScientificMathOperand": p15AMAttachment(refs["three"])}, true)
		half := c.math("mixed_divide", "", "÷", p15AMAttachment(refs["five"]), p15AMAttachment(refs["two"]))
		negative := c.math("mixed_negate", "", "-", float64(0), p15AMAttachment(half))
		inside := c.math("mixed_parentheses", "", "+", p15AMAttachment(refs["two"]), uint64(1))
		product := c.math("mixed_multiply", "", "×", p15AMAttachment(refs["three"]), p15AMAttachment(inside))
		c.view.results["mixed"] = c.math("mixed", "mixed", "+", p15AMAttachment(negative), p15AMAttachment(product))
		c.mark("summary")
		refsByRange := map[string]any{}
		for i, name := range []string{"greater", "equal", "unequal", "divided", "remainder", "mixed"} {
			refsByRange[fmt.Sprintf("{%d, 1}", []int{3, 8, 13, 19, 25, 33}[i])] = c.view.results[name]
		}
		text := c.take("gettext", "Text", map[string]any{"WFTextActionText": p15AMToken("gt=\ufffc;eq=\ufffc;ne=\ufffc;div=\ufffc;mod=\ufffc;mixed=\ufffc", refsByRange)}, true)
		c.mark("output")
		c.take("output", "", map[string]any{"WFOutput": p15AMRefToken(text)}, false)
	} else {
		return view, fmt.Errorf("unknown arithmetic graph")
	}
	if c.err == nil && c.index != len(actions) {
		c.err = fmt.Errorf("extra native action after complete arithmetic scenario")
	}
	return c.view, c.err
}
func p15AMRequireType(t *testing.T, expr syntax.Expression, want types.Type, a *analysis.Analyzer) {
	t.Helper()
	got := a.ExprTypes()[expr]
	if got == nil || !got.Equals(want) {
		t.Fatalf("semantic arithmetic type got %v want %v", got, want)
	}
}
func p15AMRequireShape(t *testing.T, expr syntax.Expression, want string) {
	t.Helper()
	if got := p15ExpressionShape(expr); got != want {
		t.Fatalf("arithmetic syntax got %s want %s", got, want)
	}
}
func p15AMHealthyCompile(t *testing.T, source string) {
	t.Helper()
	raw, err := CompileSourceToPlist("healthy-arithmetic.cherri", source)
	if err != nil || len(raw) == 0 {
		t.Fatalf("healthy arithmetic source failed: %v", err)
	}
}
func p15AMRejectTypedOperand(t *testing.T, source, operand string) {
	t.Helper()
	diagnostics := p15TypeDiagnostics(t, source)
	if err := p15ExactLiteralDiagnostic(diagnostics, source, operand, analysis.CodeArgumentType); err != nil {
		t.Fatal(err)
	}
	raw, err := CompileSourceToPlist("invalid-arithmetic.cherri", source)
	if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
		t.Fatalf("invalid numeric operand emitted native output: %v", err)
	}
}
func TestP15RP15CompoundOnceUnit(t *testing.T) {
	source := p15AMSource(t, "RP15")
	program, a := p15T08Analyzer(t, source)
	p15AMHealthyCompile(t, source)
	for _, name := range []string{"selector", "accumulator", "calls", "order"} {
		t.Run("type_"+name, func(t *testing.T) { p15AMRequireType(t, p15BindingValue(t, program, name), types.Number, a) })
	}
	var updates []*syntax.AssignStmt
	for _, stmt := range program.Statements {
		if update, ok := stmt.(*syntax.AssignStmt); ok && update.Name == "accumulator" {
			updates = append(updates, update)
		}
	}
	if len(updates) != 4 {
		t.Fatal("missing four compound operations")
	}
	for i, spec := range []struct {
		name, value string
		op          syntax.TokenType
	}{{"add", "3", syntax.TokenPlusAssign}, {"multiply", "2", syntax.TokenStarAssign}, {"subtract", "4", syntax.TokenMinusAssign}, {"divide", "2", syntax.TokenSlashAssign}} {
		t.Run("step_"+spec.name, func(t *testing.T) {
			update := updates[i]
			if update.Op != spec.op {
				t.Fatal("wrong compound operator")
			}
			p15AMRequireType(t, update.Value, types.Number, a)
			rhs, ok := update.Value.(*syntax.IfStmt)
			if !ok {
				t.Fatal("RHS helper is not an explicit lexical expression")
			}
			p15AMRequireType(t, rhs.Condition, types.Bool, a)
			p15AMRequireShape(t, rhs.Condition, "(== selector 1)")
			other, ok := rhs.ElseBlock.(*syntax.BlockStmt)
			if !ok {
				t.Fatal("missing alternate RHS branch")
			}
			for _, block := range []*syntax.BlockStmt{rhs.ThenBlock, other} {
				if len(block.Statements) != 3 {
					t.Fatal("RHS must contain exactly one counter, one order write and one yield")
				}
				count, ok := block.Statements[0].(*syntax.AssignStmt)
				if !ok || count.Name != "calls" || count.Op != syntax.TokenPlusAssign {
					t.Fatal("missing once-only counter")
				}
				p15AMRequireType(t, count.Value, types.Number, a)
				p15AMRequireShape(t, count.Value, "1")
				order, ok := block.Statements[1].(*syntax.AssignStmt)
				if !ok || order.Name != "order" || order.Op != syntax.TokenAssign {
					t.Fatal("missing ordered effect")
				}
				p15AMRequireType(t, order.Value, types.Number, a)
				p15AMRequireShape(t, order.Value, fmt.Sprintf("(+ (* order 10) %d)", i+1))
				yielded, ok := block.Statements[2].(*syntax.YieldStmt)
				if !ok {
					t.Fatal("missing actual RHS yield")
				}
				p15AMRequireType(t, yielded.Value, types.Number, a)
				p15AMRequireShape(t, yielded.Value, spec.value)
			}
		})
	}
	t.Run("typed_final_number_and_effect_observation", func(t *testing.T) {
		if len(program.Statements) != 10 {
			t.Fatal("unexpected statement outside complete compound scenario")
		}
		observation, ok := program.Statements[8].(*syntax.ExprStmt)
		if !ok {
			t.Fatal("missing actual effect observation")
		}
		p15AMRequireShape(t, observation.Expr, "(call setClipboard (+ (* calls 10000) order))")
		call := observation.Expr.(*syntax.CallExpr)
		p15AMRequireType(t, call.PrimaryArg, types.Number, a)
		returned, ok := program.Statements[9].(*syntax.ReturnStmt)
		if !ok {
			t.Fatal("missing explicit main Number return")
		}
		p15AMRequireShape(t, returned.Value, "accumulator")
		p15AMRequireType(t, returned.Value, types.Number, a)
	})
	for _, op := range []struct{ name, token string }{{"add", "+="}, {"multiply", "*="}, {"subtract", "-="}, {"divide", "/="}} {
		t.Run("reject_bool_"+op.name, func(t *testing.T) {
			p15AMRejectTypedOperand(t, "var value = 10\nvalue "+op.token+" false\nreturn value\n", "false")
		})
	}
	p15AMHealthyCompile(t, source)
}
func TestP15RP17ComparisonsArithmeticUnit(t *testing.T) {
	source := p15AMSource(t, "RP17")
	program, a := p15T08Analyzer(t, source)
	p15AMHealthyCompile(t, source)
	for _, name := range []string{"two", "three", "five", "ten"} {
		t.Run("type_"+name, func(t *testing.T) { p15AMRequireType(t, p15BindingValue(t, program, name), types.Number, a) })
	}
	for _, spec := range []struct {
		name, shape string
		kind        types.Type
	}{
		{"greater", "(> two three)", types.Bool}, {"equal", "(== two two)", types.Bool}, {"unequal", "(!= two two)", types.Bool},
		{"divided", "(/ five two)", types.Number}, {"remainder", "(% ten three)", types.Number}, {"mixed", "(+ (- (/ five two)) (* three (+ two 1)))", types.Number},
	} {
		t.Run("typed_"+spec.name, func(t *testing.T) {
			value := p15BindingValue(t, program, spec.name)
			p15AMRequireType(t, value, spec.kind, a)
			p15AMRequireShape(t, value, spec.shape)
		})
	}
	for _, spec := range []struct{ name, expression, operand string }{
		{"text_divisor", "5 / \"two\"", "\"two\""}, {"bool_modulus", "10 % false", "false"}, {"zero_divisor", "5 / 0", "0"},
	} {
		t.Run("reject_"+spec.name, func(t *testing.T) {
			p15AMRejectTypedOperand(t, "let result = "+spec.expression+"\nreturn result\n", spec.operand)
		})
	}
	for _, spec := range []struct{ name, expression string }{{"double_star", "5 ** 2"}, {"ampersand", "5 & 2"}} {
		t.Run("unknown_operator_"+spec.name, func(t *testing.T) {
			invalid := "let result = " + spec.expression + "\noutput(result)\n"
			_, parseErrors := p15ParseProgram(invalid)
			raw, err := CompileSourceToPlist("unknown-arithmetic-operator.cherri", invalid)
			if len(parseErrors) == 0 || err == nil || len(raw) != 0 {
				t.Fatal("unknown public operator silently produced a native operation")
			}
		})
	}
	p15AMHealthyCompile(t, source)
}
func p15AMCompile(t *testing.T, id string) []map[string]any {
	t.Helper()
	source := p15AMSource(t, id)
	raw, err := CompileSourceToPlist("combined-"+strings.ToLower(id)+".cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var root map[string]any
	if _, err = plist.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	rows, ok := root["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("actual native action array missing")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		actions[i], ok = row.(map[string]any)
		if !ok {
			t.Fatal("malformed actual action")
		}
	}
	receipt, err := json.Marshal(map[string]any{"requirement": id, "source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(source))), "native_sha256": fmt.Sprintf("%x", sha256.Sum256(raw)), "native_base64": base64.StdEncoding.EncodeToString(raw), "actions": len(actions), "apple_runtime_observed": false})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_COMBINED_ARITHMETIC:" + string(receipt))
	return actions
}
func p15AMRewriteIdentity(value any, old, replacement string) int {
	count := 0
	switch v := value.(type) {
	case map[string]any:
		for key, child := range v {
			if (key == "UUID" || key == "OutputUUID" || key == "GroupingIdentifier") && child == old {
				v[key] = replacement
				count++
			} else {
				count += p15AMRewriteIdentity(child, old, replacement)
			}
		}
	case []any:
		for _, child := range v {
			count += p15AMRewriteIdentity(child, old, replacement)
		}
	}
	return count
}
func p15AMRejectGraph(t *testing.T, actions []map[string]any, id string) {
	t.Helper()
	if _, err := p15AMGraph(actions, id); err == nil {
		t.Fatal("damaged complete arithmetic graph was accepted")
	}
}
func p15AMCommonFaults(t *testing.T, actions []map[string]any, view p15AMView, id string) {

	t.Run("consistent_alpha_uuid", func(t *testing.T) {
		changed := p15AMClone(t, actions)
		old := p15AMParams(changed[0])["UUID"].(string)
		replacement := "12345678-1234-4234-8234-123456789abc"
		n := 0
		for _, row := range changed {
			n += p15AMRewriteIdentity(row, old, replacement)
		}
		if n < 2 {
			t.Fatal("alpha UUID control missed a producer or consumer")
		}
		if _, err := p15AMGraph(changed, id); err != nil {
			t.Fatalf("consistent valid UUID rename failed: %v", err)
		}
	})
	for _, fault := range []string{"extra_action", "missing_output", "wrong_output_reference", "reconciled_malformed_uuid", "reconciled_zero_uuid", "reconciled_malformed_group", "reconciled_zero_group", "duplicate_uuid", "casefold_duplicate_uuid", "late_producer", "native_integer_kind"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15AMClone(t, actions)
			switch fault {
			case "extra_action":
				changed = append(changed, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
			case "missing_output":
				changed = changed[:len(changed)-1]
			case "wrong_output_reference":
				field := p15AMParams(changed[view.marks["output"]])["WFOutput"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)
				if id == "RP15" {
					field["VariableName"] = "calls"
				} else {
					field["OutputUUID"] = p15AMParams(changed[0])["UUID"]
				}
			case "reconciled_malformed_uuid", "reconciled_zero_uuid":
				old := p15AMParams(changed[0])["UUID"].(string)
				replacement := "not-a-native-uuid"
				if fault == "reconciled_zero_uuid" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				n := 0
				for _, row := range changed {
					n += p15AMRewriteIdentity(row, old, replacement)
				}
				if n < 2 {
					t.Fatal("identity control did not reconcile producer and consumer")
				}
			case "reconciled_malformed_group", "reconciled_zero_group":
				index := view.marks["greater"]
				if id == "RP15" {
					index = view.marks["add_guard"]
				}
				old := p15AMParams(changed[index])["GroupingIdentifier"].(string)
				replacement := "not-a-native-group"
				if fault == "reconciled_zero_group" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				n := 0
				for _, row := range changed {
					n += p15AMRewriteIdentity(row, old, replacement)
				}
				if n < 3 {
					t.Fatal("group control did not reconcile all branch boundaries")
				}
			case "duplicate_uuid":
				p15AMParams(changed[1])["UUID"] = p15AMParams(changed[0])["UUID"]
			case "casefold_duplicate_uuid":
				p15AMParams(changed[1])["UUID"] = strings.ToUpper(p15AMParams(changed[0])["UUID"].(string))
			case "late_producer":
				changed[0], changed[len(changed)-1] = changed[len(changed)-1], changed[0]
			case "native_integer_kind":
				p15AMParams(changed[0])["WFNumberActionNumber"] = float64(1)
				if id == "RP17" {
					p15AMParams(changed[0])["WFNumberActionNumber"] = float64(2)
				}
			}
			p15AMRejectGraph(t, changed, id)
		})
	}
}
func TestP15RP15CompoundOnceNative(t *testing.T) {
	actions := p15AMCompile(t, "RP15")
	view, err := p15AMGraph(actions, "RP15")
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []string{"add", "multiply", "subtract", "divide"} {
		for _, fault := range []string{"operator", "target", "rhs_reference", "store_reference", "counter_twice", "counter_missing", "counter_outside_guard", "alternate_counter_twice", "order_marker", "rhs_value", "alternate_rhs_value", "wrong_branch_group"} {
			t.Run(step+"/"+fault, func(t *testing.T) {
				changed := p15AMClone(t, actions)
				compound := p15AMParams(changed[view.marks[step+"_compound"]])
				prefix := step + "_branch0"
				switch fault {
				case "operator":
					compound["WFMathOperation"] = "invalid"
				case "target":
					compound["WFInput"] = p15AMAttachment(p15AMVariable("calls"))
				case "rhs_reference":
					compound["WFMathOperand"] = p15AMAttachment(p15AMVariable("calls"))
				case "store_reference":
					p15AMParams(changed[view.marks[step+"_store"]])["WFInput"] = p15AMAttachment(p15AMVariable("calls"))
				case "counter_twice":
					p15AMParams(changed[view.marks[prefix+"_counter"]])["WFMathOperand"] = uint64(2)
				case "counter_missing":
					index := view.marks[prefix+"_counter"]
					changed = append(changed[:index], changed[index+1:]...)
				case "counter_outside_guard":
					i, j := view.marks[step+"_guard"], view.marks[prefix+"_counter"]
					changed[i], changed[j] = changed[j], changed[i]
				case "alternate_counter_twice":
					p15AMParams(changed[view.marks[step+"_branch1_counter"]])["WFMathOperand"] = uint64(2)
				case "order_marker":
					p15AMParams(changed[view.marks[prefix+"_order_add"]])["WFMathOperand"] = uint64(9)
				case "rhs_value":
					p15AMParams(changed[view.marks[prefix+"_yield"]])["WFNumberActionNumber"] = uint64(99)
				case "alternate_rhs_value":
					p15AMParams(changed[view.marks[step+"_branch1_yield"]])["WFNumberActionNumber"] = uint64(99)
				case "wrong_branch_group":
					p15AMParams(changed[view.marks[step+"_guard"]])["GroupingIdentifier"] = "11111111-2222-4333-8444-555555555555"
				}
				p15AMRejectGraph(t, changed, "RP15")
			})
		}
	}
	for _, fault := range []string{"initial_value", "initial_counter", "initial_order", "detached_counter_observation", "missing_order_observation"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15AMClone(t, actions)
			switch fault {
			case "initial_value":
				p15AMParams(changed[view.marks["initial_accumulator"]])["WFNumberActionNumber"] = uint64(11)
			case "initial_counter":
				p15AMParams(changed[view.marks["initial_calls"]])["WFNumberActionNumber"] = uint64(1)
			case "initial_order":
				p15AMParams(changed[view.marks["initial_order"]])["WFNumberActionNumber"] = uint64(1)
			case "detached_counter_observation":
				p15AMParams(changed[view.marks["clipboard"]])["WFInput"] = uint64(41234)
			case "missing_order_observation":
				p15AMParams(changed[view.marks["observation_order"]])["WFMathOperand"] = uint64(1234)
			}
			p15AMRejectGraph(t, changed, "RP15")
		})
	}
	p15AMCommonFaults(t, actions, view, "RP15")
	t.Log("Fixed local compound graph: 10 +=3 *=2 -=4 /=2; mathematical result 11; each selected RHS has one counter/order write; expected calls=4/order=1234/clipboard=41234. Actual Apple return type/value and effects remain pending.")
}
func TestP15RP17ComparisonsArithmeticNative(t *testing.T) {
	actions := p15AMCompile(t, "RP17")
	view, err := p15AMGraph(actions, "RP17")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"greater", "equal", "unequal"} {
		for _, fault := range []string{"operator", "right_reference", "bool_value", "bool_integer_kind", "addition_fallthrough"} {
			t.Run(name+"/"+fault, func(t *testing.T) {
				changed := p15AMClone(t, actions)
				params := p15AMParams(changed[view.marks[name]])
				switch fault {
				case "operator":
					params["WFCondition"] = uint64(0)
				case "right_reference":
					params["WFNumberValue"] = p15AMAttachment(p15AMVariable("foreign"))
				case "bool_value":
					p15AMParams(changed[view.marks[name+"_true"]])["WFNumberActionNumber"] = float64(0)
				case "bool_integer_kind":
					p15AMParams(changed[view.marks[name+"_true"]])["WFNumberActionNumber"] = uint64(1)
				case "addition_fallthrough":
					changed[view.marks[name]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.math"
					params["WFMathOperation"] = "+"
				}
				p15AMRejectGraph(t, changed, "RP17")
			})
		}
	}
	for _, fault := range []string{"division_addition", "division_integer_fold", "division_wrong_divisor", "modulus_addition", "modulus_operation", "modulus_missing_operand", "unary_addition", "unary_wrong_zero_kind", "parentheses_order", "parentheses_operand", "missing_summary_value", "wrong_summary_value", "extra_rounding"} {
		t.Run(fault, func(t *testing.T) {
			changed := p15AMClone(t, actions)
			switch fault {
			case "division_addition":
				p15AMParams(changed[view.marks["divided"]])["WFMathOperation"] = "+"
			case "division_integer_fold":
				params := p15AMParams(changed[view.marks["divided"]])
				changed[view.marks["divided"]] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.number", "WFWorkflowActionParameters": map[string]any{"UUID": params["UUID"], "CustomOutputName": "divided", "WFNumberActionNumber": uint64(2)}}
			case "division_wrong_divisor":
				p15AMParams(changed[view.marks["divided"]])["WFMathOperand"] = uint64(5)
			case "modulus_addition":
				params := p15AMParams(changed[view.marks["remainder"]])
				params["WFMathOperation"] = "+"
				params["WFMathOperand"] = params["WFScientificMathOperand"]
				delete(params, "WFScientificMathOperand")
				delete(params, "WFScientificMathOperation")
			case "modulus_operation":
				p15AMParams(changed[view.marks["remainder"]])["WFScientificMathOperation"] = "Power"
			case "modulus_missing_operand":
				delete(p15AMParams(changed[view.marks["remainder"]]), "WFScientificMathOperand")
			case "unary_addition":
				p15AMParams(changed[view.marks["mixed_negate"]])["WFMathOperation"] = "+"
			case "unary_wrong_zero_kind":
				p15AMParams(changed[view.marks["mixed_negate"]])["WFInput"] = uint64(0)
			case "parentheses_order":
				i, j := view.marks["mixed_parentheses"], view.marks["mixed_multiply"]
				changed[i], changed[j] = changed[j], changed[i]
			case "parentheses_operand":
				p15AMParams(changed[view.marks["mixed_parentheses"]])["WFMathOperand"] = uint64(2)
			case "missing_summary_value":
				refs := p15AMParams(changed[view.marks["summary"]])["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
				delete(refs, "{19, 1}")
			case "wrong_summary_value":
				refs := p15AMParams(changed[view.marks["summary"]])["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
				refs["{19, 1}"] = refs["{25, 1}"]
			case "extra_rounding":
				i := view.marks["summary"]
				changed = append(changed[:i], append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.round", "WFWorkflowActionParameters": map[string]any{}}}, changed[i:]...)...)
			}
			p15AMRejectGraph(t, changed, "RP17")
		})
	}
	p15AMCommonFaults(t, actions, view, "RP17")
	t.Log("Fixed local arithmetic graph: false/true/false comparisons, division 2.5, modulus 1 and mixed expression 6.5. Semantic Bool is independent of native real 0/1 carriers. Actual Apple Bool/Number results and return transport remain pending.")
}
