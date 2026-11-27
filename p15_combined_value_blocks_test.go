package main

import (
	"crypto/sha256"
	"encoding/base64"
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

// These are independently authored local semantic and complete native graph
// expectations. They do not simulate Apple's Repeat cardinality or return frames.
// Unsupported nested List/Optional elements remain an explicit F04 gap.
type p15VBCase struct {
	name, source string
	model        func(*p15VBCursor)
}
type p15VBCursor struct {
	p15SCCursor
	marks               map[string]int
	refs                map[string]map[string]any
	outer, functionBody string
}

func p15VBNew(actions []map[string]any) *p15VBCursor {
	return &p15VBCursor{p15SCCursor: p15SCCursor{actions: actions, groups: map[string]bool{}}, marks: map[string]int{}, refs: map[string]map[string]any{}}
}
func p15VBToken(value string, refs ...map[string]any) map[string]any {
	var body = map[string]any{"string": value}
	if len(refs) > 0 {
		var attachments = map[string]any{}
		for i, ref := range refs {
			attachments[fmt.Sprintf("{%d, 1}", 2*i)] = ref
		}
		body["attachmentsByRange"] = attachments
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": body}
}
func p15VBAttachment(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
}
func p15VBVariable(name string) map[string]any {
	return map[string]any{"Type": "Variable", "VariableName": name}
}
func (c *p15VBCursor) mark(name string) { c.marks[name] = c.index }
func (c *p15VBCursor) bare(kind string) {
	if c.err != nil {
		return
	}
	if c.index >= len(c.actions) || !reflect.DeepEqual(c.actions[c.index], map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + kind}) {
		c.err = fmt.Errorf("value-block exact bare %s at %d", kind, c.index)
		return
	}
	c.index++
}
func (c *p15VBCursor) group(kind string, fields map[string]any) string {
	if c.err != nil {
		return ""
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("value-block missing %s group", kind)
		return ""
	}
	var id, ok = p15SCParams(c.actions[c.index])["GroupingIdentifier"].(string)
	if !ok || !p15SCCanonicalUUID(id) || c.groups[strings.ToLower(id)] {
		c.err = fmt.Errorf("value-block invalid or duplicate group")
		return ""
	}
	c.groups[strings.ToLower(id)] = true
	fields["GroupingIdentifier"] = id
	fields["WFControlFlowMode"] = uint64(0)
	c.take(kind, "", fields, false)
	return id
}
func (c *p15VBCursor) condition(ref map[string]any, code uint64, key string, value any) string {
	var fields = map[string]any{"WFCondition": code, "WFConditionalLegacyComparisonBehavior": true, "WFInput": map[string]any{"Type": "Variable", "Variable": p15VBAttachment(ref)}}
	if key != "" {
		fields[key] = value
	}
	return c.group("conditional", fields)
}
func (c *p15VBCursor) boundary(kind, group string, mode uint64, output bool) map[string]any {
	return c.take(kind, "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": mode}, output)
}
func (c *p15VBCursor) boundResult(kind, group, name string) map[string]any {
	return c.take(kind, name, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(2)}, true)
}
func (c *p15VBCursor) literal(kind, name, key string, value any, output bool) map[string]any {
	return c.take(kind, name, map[string]any{key: value}, output)
}
func (c *p15VBCursor) text(value, name string, output bool) map[string]any {
	return c.literal("gettext", name, "WFTextActionText", value, output)
}
func (c *p15VBCursor) output(ref map[string]any) {
	c.take("output", "", map[string]any{"WFOutput": p15VBToken("\ufffc", ref)}, false)
}
func (c *p15VBCursor) menu(prompt string, labels ...string) string {
	var rows = []any{}
	for _, label := range labels {
		rows = append(rows, map[string]any{"WFItemType": uint64(0), "WFValue": label})
	}
	return c.group("choosefrommenu", map[string]any{"WFMenuPrompt": prompt, "WFMenuItems": rows})
}
func (c *p15VBCursor) menuCase(group, label string) {
	c.take("choosefrommenu", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(1), "WFMenuItemTitle": label, "WFMenuItemAttributedTitle": label}, false)
}
func (c *p15VBCursor) repeat(count uint64) string {
	return c.group("repeat.count", map[string]any{"WFRepeatCount": count})
}
func (c *p15VBCursor) each(items []string) string {
	var rows = []any{}
	for _, item := range items {
		rows = append(rows, map[string]any{"WFItemType": uint64(0), "WFValue": item})
	}
	var input = c.take("list", "List", map[string]any{"WFItems": rows}, true)
	var group = c.group("repeat.each", map[string]any{"WFInput": p15VBAttachment(input)})
	c.take("setvariable", "", map[string]any{"WFVariableName": "item", "WFInput": p15VBAttachment(p15VBVariable("Repeat Item"))}, false)
	return group
}
func (c *p15VBCursor) header(function string) {
	var input = map[string]any{"Type": "ExtensionInput"}
	var zero = c.text("0", "", true)
	c.store("_cherri_is_fn", zero)
	c.store("_cherri_caller_input", input)
	var provided = c.condition(input, 100, "", nil)
	var possible = c.take("detect.dictionary", "", map[string]any{"WFInput": p15VBAttachment(input)}, true)
	var marker = c.get("cherri_functions", possible)
	var magic = c.get("_cherri_magic", possible)
	c.refs["dispatch_marker"] = marker
	c.refs["dispatch_magic"] = magic
	var valid = c.condition(magic, 4, "WFConditionalActionString", "__cherri_fn_envelope_v2__")
	c.mark("dispatch_marker_guard")
	var marked = c.condition(marker, 4, "WFConditionalActionString", "1")
	var one = c.text("1", "", true)
	c.store("_cherri_is_fn", one)
	var caller = c.get("_cherri_caller_input", possible)
	c.store("_cherri_caller_input", caller)
	c.mark("dispatch_marker_end")
	c.boundary("conditional", marked, 2, false)
	c.boundary("conditional", valid, 2, false)
	c.boundary("conditional", provided, 2, false)
	c.mark("dispatch_outer")
	c.outer = c.condition(p15VBVariable("_cherri_is_fn"), 4, "WFConditionalActionString", "1")
	var envelope = c.take("detect.dictionary", "", map[string]any{"WFInput": p15VBAttachment(input)}, true)
	var name = c.get("function", envelope)
	c.get("arguments", envelope)
	c.mark("function_frame")
	c.functionBody = c.condition(name, 4, "WFConditionalActionString", function)
}
func (c *p15VBCursor) get(key string, source map[string]any) map[string]any {
	return c.take("getvalueforkey", "", map[string]any{"WFInput": p15VBAttachment(source), "WFDictionaryKey": key, "WFGetDictionaryValueType": "Value"}, true)
}
func (c *p15VBCursor) caller(function, name string) map[string]any {
	c.boundary("conditional", c.functionBody, 2, false)
	c.mark("child_exit")
	c.bare("exit")
	c.boundary("conditional", c.outer, 1, false)
	var fields = []struct {
		key   string
		kind  uint64
		value any
	}{
		{"_cherri_caller_input", 0, p15VBAttachment(p15VBVariable("_cherri_caller_input"))},
		{"_cherri_magic", 0, p15VBToken("__cherri_fn_envelope_v2__")},
		{"arguments", 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{}}},
		{"cherri_functions", 0, p15VBToken("1")},
		{"function", 0, p15VBToken(function)},
	}
	var rows = []any{}
	for _, field := range fields {
		rows = append(rows, map[string]any{"WFKey": p15VBToken(field.key), "WFItemType": field.kind, "WFValue": field.value})
	}
	var payload = c.take("dictionary", "FunctionPayload", map[string]any{"WFItems": map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": rows}}}, true)
	c.mark("child_call")
	return c.take("runworkflow", name, map[string]any{"WFInput": p15VBAttachment(payload), "WFWorkflowName": "p15-value-block-combined", "WFWorkflow": map[string]any{"isSelf": true, "workflowName": "p15-value-block-combined"}}, true)
}
func p15VBGraph(actions []map[string]any, spec p15VBCase) (*p15VBCursor, error) {
	var c = p15VBNew(actions)
	if err := p15SCReferences(actions); err != nil {
		return c, err
	}
	spec.model(c)
	if c.err != nil {
		return c, c.err
	}
	if c.index != len(actions) {
		return c, fmt.Errorf("value-block unconsumed native actions: %d/%d", c.index, len(actions))
	}
	return c, nil
}
func p15VBSource(t *testing.T, id string) string {
	t.Helper()
	var name = map[string]string{"RP23": "v2-rp23-value-block-yields.cherri", "BRG14": "v2-brg14-value-block-return.cherri"}[id]
	if name == "" {
		t.Fatal("unknown value-block fixture", id)
	}
	var data, err = os.ReadFile(filepath.Join("tests", "language-v2", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
func p15VBJoined(c *p15VBCursor, id string) {
	c.mark("seed")
	var seed = c.literal("number", "seed", "WFNumberActionNumber", uint64(1), true)
	c.mark("if_start")
	var branch = c.condition(seed, 4, "WFNumberValue", float64(1))
	c.mark("then_yield")
	c.literal("number", "Number", "WFNumberActionNumber", uint64(0), true)
	c.boundary("conditional", branch, 1, false)
	c.mark("else_yield")
	c.literal("number", "Number1", "WFNumberActionNumber", uint64(7), true)
	c.mark("if_result")
	var branchRef = c.boundResult("conditional", branch, "branch")
	c.refs["branch"] = branchRef
	c.mark("menu_start")
	var menu = c.menu(id, "A", "B")
	c.menuCase(menu, "A")
	c.mark("false_yield")
	c.literal("number", "", "WFNumberActionNumber", float64(0), true)
	c.menuCase(menu, "B")
	c.mark("true_yield")
	c.literal("number", "", "WFNumberActionNumber", float64(1), true)
	c.mark("menu_result")
	var menuRef = c.boundResult("choosefrommenu", menu, "choice")
	c.refs["menu"] = menuRef
	c.mark("zero_start")
	var zero = c.repeat(0)
	c.text("unexecuted", "Text", true)
	c.mark("zero_result")
	var zeroRef = c.boundResult("repeat.count", zero, "empty")
	c.refs["zero"] = zeroRef
	c.mark("many_start")
	var many = c.repeat(3)
	c.mark("empty_yield")
	c.text("", "Text", true)
	c.mark("many_result")
	var manyRef = c.boundResult("repeat.count", many, "values")
	c.refs["many"] = manyRef
	c.mark("each_list")
	var each = c.each([]string{"A", "B"})
	c.mark("each_yield")
	c.take("getvariable", "", map[string]any{"WFVariable": p15VBAttachment(p15VBVariable("item"))}, true)
	c.mark("each_result")
	var eachRef = c.boundResult("repeat.each", each, "each")
	c.refs["each"] = eachRef
	c.mark("joined_text")
	var result = c.take("gettext", "observed", map[string]any{"WFTextActionText": p15VBToken("\ufffc|\ufffc|\ufffc|\ufffc|\ufffc", branchRef, menuRef, zeroRef, manyRef, eachRef)}, true)
	c.refs["joined"] = result
	if id == "RP23" {
		c.mark("final_output")
		c.output(result)
	} else {
		c.mark("clipboard")
		c.take("setclipboard", "", map[string]any{"WFInput": p15VBAttachment(result)}, false)
		c.mark("unused_final")
		c.text("BRG14_UNUSED", "", false)
		c.mark("void_boundary")
		c.bare("nothing")
	}
}
func p15VBLoopCases() []p15VBCase {
	var cases = []p15VBCase{}
	for _, spec := range []struct {
		name, literal, kind, key, outputName string
		count                                uint64
		value                                any
	}{
		{"zero_iterations", "\"row\"", "gettext", "WFTextActionText", "Text", 0, "row"},
		{"false_elements", "false", "number", "WFNumberActionNumber", "", 3, float64(0)},
		{"zero_elements", "0", "number", "WFNumberActionNumber", "Number", 3, uint64(0)},
		{"empty_text_elements", "\"\"", "gettext", "WFTextActionText", "Text", 3, ""},
	} {
		var spec = spec
		cases = append(cases, p15VBCase{spec.name, fmt.Sprintf("let values=repeat %d {yield %s}\nreturn values\n", spec.count, spec.literal), func(c *p15VBCursor) {
			c.mark("loop_start")
			var group = c.repeat(spec.count)
			c.mark("yield")
			c.literal(spec.kind, spec.outputName, spec.key, spec.value, true)
			c.mark("loop_result")
			var result = c.boundResult("repeat.count", group, "values")
			c.refs["loop"] = result
			c.mark("final_output")
			c.output(result)
		}})
	}
	cases = append(cases, p15VBCase{"for_items", "let values=for item in [\"A\",\"B\"] {yield item}\nreturn values\n", func(c *p15VBCursor) {
		c.mark("loop_list")
		var group = c.each([]string{"A", "B"})
		c.mark("yield")
		c.take("getvariable", "", map[string]any{"WFVariable": p15VBAttachment(p15VBVariable("item"))}, true)
		c.mark("loop_result")
		var result = c.boundResult("repeat.each", group, "values")
		c.refs["loop"] = result
		c.mark("final_output")
		c.output(result)
	}})
	return cases
}
func p15VBCases(t *testing.T, id string) []p15VBCase {
	t.Helper()
	if id == "F04" {
		return p15VBLoopCases()
	}
	var cases = []p15VBCase{{"joined_values", p15VBSource(t, id), func(c *p15VBCursor) { p15VBJoined(c, id) }}}
	if id == "RP23" {
		return cases
	}
	if id != "BRG14" {
		t.Fatal("unknown value-block scenario", id)
	}
	cases = append(cases,
		p15VBCase{"main_early_return", "let seed=number(0)\nif seed == 1 {return \"EARLY\"}\ntext(\"MAIN_CONTINUED\")\n", func(c *p15VBCursor) {
			var seed = c.literal("number", "seed", "WFNumberActionNumber", uint64(0), true)
			c.mark("return_guard")
			var group = c.condition(seed, 4, "WFNumberValue", float64(1))
			var early = c.text("EARLY", "Text", true)
			c.mark("early_output")
			c.output(early)
			c.boundary("conditional", group, 2, false)
			c.mark("continued")
			c.text("MAIN_CONTINUED", "", false)
			c.mark("void_boundary")
			c.bare("nothing")
		}},
		p15VBCase{"main_bare_return", "text(\"MAIN_NOISE\")\nreturn\n", func(c *p15VBCursor) {
			c.text("MAIN_NOISE", "", false)
			c.mark("clear_return")
			c.bare("nothing")
			c.mark("bare_output")
			c.bare("exit")
		}},
		p15VBCase{"both_branches_return", "let seed=number(1)\nif seed == 1 {return \"THEN\"} else {return \"ELSE\"}\n", func(c *p15VBCursor) {
			var seed = c.literal("number", "seed", "WFNumberActionNumber", uint64(1), true)
			c.mark("return_guard")
			var group = c.condition(seed, 4, "WFNumberValue", float64(1))
			var then = c.text("THEN", "Text", true)
			c.mark("early_output")
			c.output(then)
			c.boundary("conditional", group, 1, false)
			var other = c.text("ELSE", "Text", true)
			c.output(other)
			c.boundary("conditional", group, 2, false)
		}},
		p15VBCase{"zero_iteration_return", "repeat 0 {return \"UNREACHED\"}\n", func(c *p15VBCursor) {
			c.mark("loop_start")
			var group = c.repeat(0)
			var value = c.text("UNREACHED", "Text", true)
			c.output(value)
			c.boundary("repeat.count", group, 2, false)
			c.mark("void_boundary")
			c.bare("nothing")
		}},
		p15VBCase{"child_early_return", "function child()->Text {let seed=number(1)\nif seed == 1 {return \"CHILD_EARLY\"}\nreturn \"CHILD_NORMAL\"}\nlet unused=child()\ntext(\"PARENT_CONTINUED\")\n", func(c *p15VBCursor) {
			c.header("child")
			var seed = c.literal("number", "seed", "WFNumberActionNumber", uint64(1), true)
			c.mark("return_guard")
			var group = c.condition(seed, 4, "WFNumberValue", float64(1))
			var early = c.text("CHILD_EARLY", "Text", true)
			c.mark("early_output")
			c.output(early)
			c.boundary("conditional", group, 2, false)
			var normal = c.text("CHILD_NORMAL", "Text", true)
			c.output(normal)
			c.caller("child", "unused")
			c.mark("continued")
			c.text("PARENT_CONTINUED", "", false)
			c.boundary("conditional", c.outer, 2, false)
			c.mark("void_boundary")
			c.bare("nothing")
		}},
		p15VBCase{"child_bare_return", "function child()->Void {text(\"CHILD_NOISE\")\nreturn}\nchild()\ntext(\"PARENT_CONTINUED\")\n", func(c *p15VBCursor) {
			c.header("child")
			c.text("CHILD_NOISE", "", false)
			c.mark("clear_return")
			c.bare("nothing")
			c.mark("bare_output")
			c.bare("exit")
			c.caller("child", "childResult")
			c.mark("continued")
			c.text("PARENT_CONTINUED", "", false)
			c.boundary("conditional", c.outer, 2, false)
			c.mark("void_boundary")
			c.bare("nothing")
		}},
	)
	return cases
}
func p15VBCompile(t *testing.T, spec p15VBCase, id string) []map[string]any {
	t.Helper()
	var raw, err = CompileSourceToPlist("p15-value-block-combined.cherri", spec.source)
	if err != nil {
		t.Fatal("actual value-block compile:", err)
	}
	var doc map[string]any
	if _, err = plist.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	var values, ok = doc["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("actual native action array missing")
	}
	var actions = []map[string]any{}
	for _, value := range values {
		var action, ok = value.(map[string]any)
		if !ok {
			t.Fatal("actual native action malformed")
		}
		actions = append(actions, action)
	}
	t.Logf("VALUE_BLOCK_NATIVE id=%s case=%s source_sha256=%x native_sha256=%x bytes=%d plist_base64=%s", id, spec.name, sha256.Sum256([]byte(spec.source)), sha256.Sum256(raw), len(raw), base64.StdEncoding.EncodeToString(raw))
	return actions
}
func p15VBTypeError(program *syntax.Program, analyzer *analysis.Analyzer, wants map[string]types.Type) error {
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			return fmt.Errorf("value-block unexpected diagnostic: %+v", d)
		}
	}
	var seen = map[string]bool{}
	for _, stmt := range program.Statements {
		var binding, ok = stmt.(*syntax.BindingStmt)
		if !ok {
			continue
		}
		var want, selected = wants[binding.Name]
		if !selected {
			continue
		}
		var got = analyzer.ExprTypes()[binding.Value]
		if got == nil || !got.Equals(want) {
			return fmt.Errorf("value-block type %s: got %v want %v", binding.Name, got, want)
		}
		seen[binding.Name] = true
	}
	if len(seen) != len(wants) {
		return fmt.Errorf("value-block binding inventory incomplete")
	}
	return nil
}
func p15VBTypes(t *testing.T, id string) {
	t.Helper()
	if id == "F04" {
		for _, spec := range p15VBLoopCases() {
			t.Run(spec.name, func(t *testing.T) {
				var program, analyzer = p15F03Analysis(t, spec.source)
				var element types.Type = types.Text
				if spec.name == "false_elements" {
					element = types.Bool
				}
				if spec.name == "zero_elements" {
					element = types.Number
				}
				if err := p15VBTypeError(program, analyzer, map[string]types.Type{"values": types.NewList(element)}); err != nil {
					t.Fatal(err)
				}
			})
		}
		return
	}
	var source = p15VBSource(t, id)
	var wants = map[string]types.Type{"seed": types.Number, "branch": types.Number, "choice": types.Bool, "empty": types.NewList(types.Text), "values": types.NewList(types.Text), "each": types.NewList(types.Text)}
	var program, analyzer = p15F03Analysis(t, source)
	for _, name := range []string{"seed", "branch", "choice", "empty", "values", "each"} {
		t.Run("type_"+name, func(t *testing.T) {
			if err := p15VBTypeError(program, analyzer, map[string]types.Type{name: wants[name]}); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, fault := range []struct{ name, before, after string }{
		{"wrong_branch_type", "yield 7", "yield \"seven\""},
		{"wrong_menu_type", "yield true", "yield 1"},
		{"wrong_loop_element", "yield \"\"", "yield false"},
	} {
		t.Run(fault.name, func(t *testing.T) {
			var changed = strings.Replace(source, fault.before, fault.after, 1)
			if changed == source {
				t.Fatal("unit contrary did not change source")
			}
			var bad, checked = p15F03Analysis(t, changed)
			if err := p15VBTypeError(bad, checked, wants); err == nil || !strings.Contains(err.Error(), "value-block type") {
				t.Fatal("wrong type did not reach semantic expectation", err)
			}
		})
	}
	if id == "BRG14" {
		for _, spec := range p15VBCases(t, id)[1:] {
			t.Run("semantic_"+spec.name, func(t *testing.T) {
				var _, checked = p15F03Analysis(t, spec.source)
				if failures := p15F03Errors(checked); len(failures) != 0 {
					t.Fatal(failures)
				}
			})
		}
	}
}

type p15VBInvalid struct{ name, source, code, marker string }

func p15VBInvalidCases(id string) []p15VBInvalid {
	if id == "F04" {
		return []p15VBInvalid{
			{"unsupported_nested_list", "let values=repeat 2 {yield [\"A\",\"B\"]}\nreturn values", analysis.CodeTargetValueShape, "[\"A\",\"B\"]"},
			{"unsupported_optional", "let values=repeat 2 {yield none}\nreturn values", analysis.CodeTargetValueShape, "none"},
		}
	}
	return []p15VBInvalid{
		{"missing_if_branch", "let values=if true {yield \"A\"} else {text(\"B\")}\nreturn values", analysis.CodeYieldContext, "{text(\"B\")}"},
		{"missing_menu_branch", "let values=menu(\"Pick\"){case \"A\"{yield \"A\"} case \"B\"{text(\"B\")}}\nreturn values", analysis.CodeYieldContext, "{text(\"B\")}"},
		{"illegal_loop_yield", "let values=repeat 2 {yield show(\"B\")}\nreturn values", analysis.CodeYieldContext, "yield show(\"B\")"},
		{"illegal_yield", "yield \"outside\"\n", analysis.CodeYieldContext, "yield \"outside\""},
		{"void_yield", "let values=if true {yield show(\"A\")} else {yield \"B\"}\nreturn values", analysis.CodeYieldContext, "yield show(\"A\")"},
		{"overwritten_yield", "let values=if true {yield \"A\";text(\"discarded\")} else {yield \"B\"}\nreturn values", analysis.CodeYieldContext, "{yield \"A\";text(\"discarded\")}"},
	}
}
func p15VBDiagnosticError(diagnostics []analysis.Diagnostic, spec p15VBInvalid) error {
	var count = 0
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			count++
			if d.Code != spec.code {
				return fmt.Errorf("value-block wrong diagnostic code")
			}
		}
	}
	if count != 1 {
		return fmt.Errorf("value-block exact diagnostic count: %d", count)
	}
	return p15F03DiagnosticOracle(spec.source, diagnostics, spec.code, spec.marker)
}
func p15VBDiagnostics(t *testing.T, id string) {
	t.Helper()
	for _, spec := range p15VBInvalidCases(id) {
		t.Run(spec.name, func(t *testing.T) {
			var diagnostics = p15TypeDiagnostics(t, spec.source)
			if err := p15VBDiagnosticError(diagnostics, spec); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"missing", "wrong_code", "outside_span"} {
				t.Run(fault, func(t *testing.T) {
					var broken = append([]analysis.Diagnostic(nil), diagnostics...)
					for i := range broken {
						if broken[i].Severity == analysis.SeverityError {
							if fault == "wrong_code" {
								broken[i].Code = "E_UNRELATED"
							}
							if fault == "outside_span" {
								broken[i].Span.Start.ByteOffset = len(spec.source)
								broken[i].Span.End.ByteOffset = len(spec.source)
							}
						}
					}
					if fault == "missing" {
						broken = nil
					}
					if p15VBDiagnosticError(broken, spec) == nil {
						t.Fatal("diagnostic contrary accepted", fault)
					}
				})
			}
			var raw, err = CompileSourceToPlist("p15-value-block-negative.cherri", spec.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), spec.code) {
				t.Fatalf("invalid value block emitted native output: %d bytes %v", len(raw), err)
			}
			if id == "F04" {
				t.Logf("F04_LOCAL_GAP name=%s source_sha256=%x diagnostic=%s: target rejects element; no nested cardinality or optional-presence acceptance", spec.name, sha256.Sum256([]byte(spec.source)), spec.code)
			}
		})
	}
}
func TestP15F04ValueLoopBoundaryUnit(t *testing.T) { p15VBTypes(t, "F04"); p15VBDiagnostics(t, "F04") }
func TestP15RP23ValueBlockLoopUnit(t *testing.T)   { p15VBTypes(t, "RP23"); p15VBDiagnostics(t, "RP23") }
func TestBackendRecovery14ValueBlockReturnUnit(t *testing.T) {
	p15VBTypes(t, "BRG14")
	p15VBDiagnostics(t, "BRG14")
}

func p15VBRewrite(value any, before, after string, groups bool) int {
	var count = 0
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if (key == "UUID" || key == "OutputUUID" || groups && key == "GroupingIdentifier") && child == before {
				value[key] = after
				count++
			} else {
				count += p15VBRewrite(child, before, after, groups)
			}
		}
	case []any:
		for _, child := range value {
			count += p15VBRewrite(child, before, after, groups)
		}
	}
	return count
}
func p15VBContainsReference(value any, uuid string) bool {
	switch value := value.(type) {
	case map[string]any:
		for key, child := range value {
			if key == "OutputUUID" {
				if text, ok := child.(string); ok && strings.EqualFold(text, uuid) {
					return true
				}
			}
			if p15VBContainsReference(child, uuid) {
				return true
			}
		}
	case []any:
		for _, child := range value {
			if p15VBContainsReference(child, uuid) {
				return true
			}
		}
	}
	return false
}
func p15VBMove(actions []map[string]any, from, to int) []map[string]any {
	var moved = actions[from]
	var rest = append([]map[string]any{}, actions[:from]...)
	rest = append(rest, actions[from+1:]...)
	if to > from {
		to--
	}
	var result = append([]map[string]any{}, rest[:to]...)
	result = append(result, moved)
	return append(result, rest[to:]...)
}
func p15VBReferenceAt(actions []map[string]any, index int) map[string]any {
	var params = p15SCParams(actions[index])
	var ref = map[string]any{"Type": "ActionOutput", "OutputUUID": params["UUID"]}
	if name, ok := params["CustomOutputName"].(string); ok && name != "" {
		ref["OutputName"] = name
	}
	return ref
}

var p15VBIdentityFaults = []string{"canonical_alpha", "malformed_uuid", "zero_uuid", "casefold_uuid_collision", "malformed_group", "zero_group", "wrong_integer_kind", "late_producer", "extra_action", "missing_final_action", "unexpected_native_parameter", "reconciled_bound_output_name"}

func p15VBIdentityControls(t *testing.T, actions []map[string]any, spec p15VBCase) {
	t.Helper()
	var producers = []int{}
	var groups = []int{}
	for i, a := range actions {
		var p = p15SCParams(a)
		if _, ok := p["UUID"]; ok {
			producers = append(producers, i)
		}
		if p["WFControlFlowMode"] == uint64(0) {
			groups = append(groups, i)
		}
	}
	if len(producers) < 2 || len(groups) == 0 {
		t.Fatal("actual scenario lacks identity-control subjects")
	}
	var faults = append([]string{}, p15VBIdentityFaults...)
	if len(groups) > 1 {
		faults = append(faults, "casefold_group_collision")
	}
	for _, fault := range faults {
		t.Run("identity/"+fault, func(t *testing.T) {
			var broken = p15SCClone(t, actions)
			var intended = ""
			switch fault {
			case "canonical_alpha", "malformed_uuid", "zero_uuid":
				var before = p15SCParams(broken[producers[0]])["UUID"].(string)
				var replacement = "12345678-1234-4234-8234-123456789abc"
				if fault == "malformed_uuid" {
					replacement = "not-a-native-uuid"
					intended = "invalid native UUID"
				}
				if fault == "zero_uuid" {
					replacement = "00000000-0000-0000-0000-000000000000"
					intended = "invalid native UUID"
				}
				var count = 0
				for _, a := range broken {
					count += p15VBRewrite(a, before, replacement, false)
				}
				if count == 0 {
					t.Fatal("UUID control did not replace producer")
				}
			case "casefold_uuid_collision":
				var first = p15SCParams(broken[producers[0]])["UUID"].(string)
				var second = p15SCParams(broken[producers[1]])["UUID"].(string)
				var replacement = "abcdef01-2345-4678-89ab-cdef01234567"
				for _, a := range broken {
					p15VBRewrite(a, first, replacement, false)
					p15VBRewrite(a, second, strings.ToUpper(replacement), false)
				}
				intended = "duplicate native UUID"
			case "malformed_group", "zero_group":
				var before = p15SCParams(broken[groups[0]])["GroupingIdentifier"].(string)
				var replacement = "not-a-native-group"
				if fault == "zero_group" {
					replacement = "00000000-0000-0000-0000-000000000000"
				}
				var count = 0
				for _, a := range broken {
					count += p15VBRewrite(a, before, replacement, true)
				}
				if count < 2 {
					t.Fatal("group control missed its boundaries")
				}
				intended = "value-block invalid or duplicate group"
			case "casefold_group_collision":
				var first = p15SCParams(broken[groups[0]])["GroupingIdentifier"].(string)
				var second = p15SCParams(broken[groups[1]])["GroupingIdentifier"].(string)
				var replacement = "abcdef01-2345-4678-89ab-cdef01234567"
				for _, a := range broken {
					p15VBRewrite(a, first, replacement, true)
					p15VBRewrite(a, second, strings.ToUpper(replacement), true)
				}
				intended = "value-block invalid or duplicate group"
			case "wrong_integer_kind":
				p15SCParams(broken[groups[0]])["WFControlFlowMode"] = float64(0)
				intended = "native"
			case "late_producer":
				var moved = false
				for _, producer := range producers {
					var uuid = p15SCParams(broken[producer])["UUID"].(string)
					for consumer := producer + 1; consumer < len(broken); consumer++ {
						if p15VBContainsReference(broken[consumer], uuid) {
							broken = p15VBMove(broken, producer, consumer+1)
							moved = true
							break
						}
					}
					if moved {
						break
					}
				}
				if !moved {
					t.Fatal("late-producer control found no actual data edge")
				}
				intended = "missing or late native producer"
			case "extra_action":
				broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
				intended = "unconsumed native actions"
			case "missing_final_action":
				broken = broken[:len(broken)-1]
				intended = "missing"
				if spec.name == "joined_values" && strings.Contains(spec.source, "BRG14_UNUSED") {
					intended = "exact bare nothing"
				}
			case "unexpected_native_parameter":
				p15SCParams(broken[producers[0]])["UnexpectedValueBlockField"] = true
				intended = "native"
			case "reconciled_bound_output_name":
				var name, kind string
				switch spec.name {
				case "zero_iterations":
					name, kind = "values", "repeat.count"
				case "joined_values":
					name, kind = "branch", "conditional"
				default:
					t.Fatal("unexpected fixed bound-name specimen", spec.name)
				}
				var target = -1
				for _, producer := range producers {
					var params = p15SCParams(broken[producer])
					if params["CustomOutputName"] == name && params["WFControlFlowMode"] == uint64(2) && broken[producer]["WFWorkflowActionIdentifier"] == "is.workflow.actions."+kind {
						if target != -1 {
							t.Fatal("ambiguous authored bound output", name)
						}
						target = producer
					}
				}
				if target < 0 {
					t.Fatal("missing actual authored bound output", name)
				}
				var uuid = p15SCParams(broken[target])["UUID"].(string)
				var replacement = "__wrong_bound_result__"
				p15SCParams(broken[target])["CustomOutputName"] = replacement
				var renamed = 0
				var rewrite func(any)
				rewrite = func(value any) {
					switch value := value.(type) {
					case map[string]any:
						if reference, ok := value["OutputUUID"].(string); ok && value["Type"] == "ActionOutput" && strings.EqualFold(reference, uuid) {
							if value["OutputName"] != name {
								t.Fatal("actual bound consumer name differs before mutation", value)
							}
							value["OutputName"] = replacement
							renamed++
						}
						for _, child := range value {
							rewrite(child)
						}
					case []any:
						for _, child := range value {
							rewrite(child)
						}
					}
				}
				for _, action := range broken {
					rewrite(action)
				}
				if renamed != 1 {
					t.Fatalf("fixed bound-name control reconciled %d consumers, want one", renamed)
				}
				if err := p15SCReferences(broken); err != nil {
					t.Fatal("bound-name control failed before exact native fields gate", err)
				}
				intended = fmt.Sprintf("native %s action differs at %d:", kind, target)
			}
			if reflect.DeepEqual(broken, actions) {
				t.Fatal("identity control did not change actual graph")
			}
			var _, err = p15VBGraph(broken, spec)
			if fault == "canonical_alpha" {
				if err != nil {
					t.Fatal("canonical reconciled UUID rename rejected", err)
				}
			} else if err == nil || !strings.Contains(err.Error(), intended) {
				t.Fatalf("identity control missed intended %q gate: %v", intended, err)
			}
		})
	}
}
func p15VBCaseFaults(id, name string) []string {
	if id == "F04" {
		var faults = []string{"changed_yield", "returned_element_not_collection"}
		if name != "for_items" {
			faults = append(faults, "changed_count")
		} else {
			faults = append(faults, "changed_each_input")
		}
		return faults
	}
	if name == "joined_values" {
		var faults = []string{"swapped_branch_values", "bool_integer_carrier", "wrong_menu_label", "zero_became_one", "wrong_multiple_count", "overwritten_loop_yield", "wrong_each_variable", "swapped_join_references", "bypassed_join_result"}
		if id == "BRG14" {
			faults = append(faults, "clipboard_token_string_envelope", "missing_void_boundary", "nonbare_void_boundary", "unused_value_leak")
		}
		return faults
	}
	switch name {
	case "main_bare_return":
		return []string{"missing_clear", "output_implicit_input", "noise_after_clear"}
	case "main_early_return":
		return []string{"reversed_return_guard", "return_outside_guard", "missing_void_boundary"}
	case "both_branches_return":
		return []string{"reversed_return_guard", "return_outside_guard", "extra_void_after_return"}
	case "zero_iteration_return":
		return []string{"zero_became_one", "missing_void_boundary", "nonbare_void_boundary"}
	case "child_early_return":
		return []string{"wrong_dispatch_marker_reference", "wrong_dispatch_marker_value", "wrong_dispatch_marker_branch", "removed_dispatch_marker_guard", "reversed_return_guard", "return_outside_guard", "missing_child_exit", "wrong_function_target", "wrong_self_flag", "parent_before_call"}
	case "child_bare_return":
		return []string{"wrong_dispatch_marker_reference", "wrong_dispatch_marker_value", "wrong_dispatch_marker_branch", "removed_dispatch_marker_guard", "missing_clear", "output_implicit_input", "missing_child_exit", "wrong_function_target", "wrong_self_flag", "parent_before_call"}
	default:
		panic("unknown fixed value-block case")
	}
}
func p15VBCaseFault(t *testing.T, id string, actions []map[string]any, view *p15VBCursor, spec p15VBCase, fault string) {
	t.Helper()
	var broken = p15SCClone(t, actions)
	var intended string
	var index = func(mark string) int {
		i, ok := view.marks[mark]
		if !ok {
			t.Fatalf("missing actual control target %s", mark)
		}
		return i
	}
	var p = func(mark string) map[string]any { return p15SCParams(broken[index(mark)]) }
	switch fault {
	case "changed_count":
		var before = p("loop_start")["WFRepeatCount"].(uint64)
		p("loop_start")["WFRepeatCount"] = before + 1
	case "changed_yield":
		var q = p("yield")
		if _, found := q["WFNumberActionNumber"]; found {
			q["WFNumberActionNumber"] = float64(9)
		} else if _, found = q["WFTextActionText"]; found {
			q["WFTextActionText"] = "different"
		} else {
			q["WFVariable"] = p15VBAttachment(p15VBVariable("foreign"))
		}
	case "returned_element_not_collection":
		p("final_output")["WFOutput"] = p15VBToken("\ufffc", p15VBReferenceAt(broken, index("yield")))
	case "changed_each_input":
		p("loop_list")["WFItems"].([]any)[0].(map[string]any)["WFValue"] = "foreign"
	case "swapped_branch_values":
		p("then_yield")["WFNumberActionNumber"], p("else_yield")["WFNumberActionNumber"] = uint64(7), uint64(0)
	case "bool_integer_carrier":
		if p("false_yield")["WFNumberActionNumber"] != float64(0) {
			t.Fatal("actual false carrier was not real zero")
		}
		p("false_yield")["WFNumberActionNumber"] = uint64(0)
	case "wrong_menu_label":
		p15SCParams(broken[index("menu_start")+1])["WFMenuItemTitle"] = "changed"
	case "zero_became_one":
		var mark = "zero_start"
		if spec.name == "zero_iteration_return" {
			mark = "loop_start"
		}
		p(mark)["WFRepeatCount"] = uint64(1)
	case "wrong_multiple_count":
		p("many_start")["WFRepeatCount"] = uint64(2)
	case "overwritten_loop_yield":
		var at = index("many_result")
		broken = append(broken[:at], append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "overwrite"}}}, broken[at:]...)...)
	case "wrong_each_variable":
		p("each_yield")["WFVariable"] = p15VBAttachment(p15VBVariable("foreign"))
	case "swapped_join_references":
		var refs = p("joined_text")["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["attachmentsByRange"].(map[string]any)
		refs["{0, 1}"], refs["{2, 1}"] = refs["{2, 1}"], refs["{0, 1}"]
	case "bypassed_join_result", "clipboard_token_string_envelope":
		if id == "RP23" {
			if fault != "bypassed_join_result" {
				t.Fatal("Clipboard envelope control has no RP23 target")
			}
			p("final_output")["WFOutput"] = p15VBToken("\ufffc", view.refs["branch"])
		} else {
			var input = p("clipboard")
			if id != "BRG14" || spec.name != "joined_values" || !reflect.DeepEqual(input["WFInput"], p15VBAttachment(view.refs["joined"])) {
				t.Fatal("actual Clipboard input is not the exact direct joined-result attachment")
			}
			if fault == "bypassed_join_result" {
				input["WFInput"] = p15VBAttachment(view.refs["branch"])
			} else {
				input["WFInput"] = p15VBToken("\ufffc", view.refs["joined"])
			}
			if err := p15SCReferences(broken); err != nil {
				t.Fatal("Clipboard control failed before exact native input gate", err)
			}
			intended = fmt.Sprintf("native setclipboard action differs at %d:", index("clipboard"))
		}
	case "missing_void_boundary":
		var at = index("void_boundary")
		broken = append(broken[:at], broken[at+1:]...)
	case "nonbare_void_boundary":
		broken[index("void_boundary")]["WFWorkflowActionParameters"] = map[string]any{"WFInput": "noise"}
	case "unused_value_leak":
		broken[index("void_boundary")] = map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "BRG14_UNUSED"}}
	case "missing_clear":
		var at = index("clear_return")
		broken = append(broken[:at], broken[at+1:]...)
	case "output_implicit_input":
		broken[index("bare_output")]["WFWorkflowActionParameters"] = map[string]any{"WFOutput": p15VBToken("noise")}
	case "noise_after_clear":
		var at = index("bare_output")
		broken = append(broken[:at], append([]map[string]any{{"WFWorkflowActionIdentifier": "is.workflow.actions.gettext", "WFWorkflowActionParameters": map[string]any{"WFTextActionText": "noise"}}}, broken[at:]...)...)
	case "reversed_return_guard":
		p("return_guard")["WFCondition"] = uint64(5)
	case "return_outside_guard":
		var at = index("early_output")
		broken = p15VBMove(broken, at, at+2)
		if err := p15SCReferences(broken); err != nil {
			t.Fatal("return-frame control failed before ancestry gate", err)
		}
	case "extra_void_after_return":
		broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
	case "wrong_dispatch_marker_reference", "wrong_dispatch_marker_value", "wrong_dispatch_marker_branch", "removed_dispatch_marker_guard":
		if id != "BRG14" || spec.name != "child_early_return" && spec.name != "child_bare_return" {
			t.Fatal("dispatch marker control has no authored child-return target")
		}
		var start, end = index("dispatch_marker_guard"), index("dispatch_marker_end")
		var guard, close = p("dispatch_marker_guard"), p("dispatch_marker_end")
		var expectedInput = map[string]any{"Type": "Variable", "Variable": p15VBAttachment(view.refs["dispatch_marker"])}
		if start >= end || !reflect.DeepEqual(guard["WFInput"], expectedInput) ||
			guard["WFCondition"] != uint64(4) || guard["WFConditionalActionString"] != "1" ||
			guard["WFControlFlowMode"] != uint64(0) ||
			!reflect.DeepEqual(close, map[string]any{"GroupingIdentifier": guard["GroupingIdentifier"], "WFControlFlowMode": uint64(2)}) {
			t.Fatal("actual child dispatch marker is not the exact nested true branch")
		}
		intended = fmt.Sprintf("native conditional action differs at %d:", start)
		switch fault {
		case "wrong_dispatch_marker_reference":
			guard["WFInput"] = map[string]any{"Type": "Variable", "Variable": p15VBAttachment(view.refs["dispatch_magic"])}
		case "wrong_dispatch_marker_value":
			guard["WFConditionalActionString"] = "0"
		case "wrong_dispatch_marker_branch":
			guard["WFControlFlowMode"] = uint64(1)
		case "removed_dispatch_marker_guard":
			broken = append(broken[:end], broken[end+1:]...)
			broken = append(broken[:start], broken[start+1:]...)
			intended = "value-block invalid or duplicate group"
		}
		if err := p15SCReferences(broken); err != nil {
			t.Fatal("dispatch marker control failed before exact nested guard gate", err)
		}
	case "missing_child_exit":
		var at = index("child_exit")
		broken = append(broken[:at], broken[at+1:]...)
	case "wrong_function_target":
		var rows = p15SCParams(broken[index("child_call")-1])["WFItems"].(map[string]any)["Value"].(map[string]any)["WFDictionaryFieldValueItems"].([]any)
		rows[len(rows)-1].(map[string]any)["WFValue"] = p15VBToken("foreign")
	case "wrong_self_flag":
		p("child_call")["WFWorkflow"].(map[string]any)["isSelf"] = false
	case "parent_before_call":
		broken = p15VBMove(broken, index("continued"), index("child_call"))
		if err := p15SCReferences(broken); err != nil {
			t.Fatal("caller-order control failed before complete graph gate", err)
		}
	default:
		t.Fatal("unknown value-block mutation", fault)
	}
	if reflect.DeepEqual(broken, actions) {
		t.Fatal("value-block control did not mutate original")
	}
	if _, err := p15VBGraph(broken, spec); err == nil || intended != "" && !strings.Contains(err.Error(), intended) {
		t.Fatalf("value-block causal corruption missed intended %q gate: %s: %v", intended, fault, err)
	}
}
func p15VBNative(t *testing.T, id string) {
	t.Helper()
	var identityActions []map[string]any
	var identitySpec p15VBCase
	var identityName = "joined_values"
	if id == "F04" {
		identityName = "zero_iterations"
	}
	for _, spec := range p15VBCases(t, id) {
		t.Run(spec.name, func(t *testing.T) {
			var actions = p15VBCompile(t, spec, id)
			var view, err = p15VBGraph(actions, spec)
			if err != nil {
				t.Fatal(err)
			}
			for _, fault := range p15VBCaseFaults(id, spec.name) {
				t.Run(fault, func(t *testing.T) { p15VBCaseFault(t, id, actions, view, spec, fault) })
			}
			if _, err = p15VBGraph(actions, spec); err != nil {
				t.Fatal("negative controls changed original", err)
			}
			if spec.name == identityName {
				identityActions = actions
				identitySpec = spec
			}
		})
	}
	if identityActions == nil {
		t.Fatal("required actual value-block identity specimen did not pass", identityName)
	}
	p15VBIdentityControls(t, identityActions, identitySpec)
	t.Logf("%s local typed semantic/native structure only. No Apple loop cardinality, menu selection, frame exit or rendered output was observed. F04 nested List/Optional one-element transport remains unsupported.", id)
}
func TestP15F04ValueLoopBoundaryNative(t *testing.T)           { p15VBNative(t, "F04") }
func TestP15RP23ValueBlockLoopNative(t *testing.T)             { p15VBNative(t, "RP23") }
func TestBackendRecovery14ValueBlockReturnNative(t *testing.T) { p15VBNative(t, "BRG14") }
