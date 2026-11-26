package main

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

// These source-authored expectations consume every native action. Only UUID
// alpha names are learned from the actual graph. Native scalar kinds, names,
// argument fields, control ancestry and every consumer edge remain exact.
// This is a structure oracle, not an Apple interpreter or runtime assertion.
type p15FTCursor struct {
	p15SCCursor
	points             map[string]int
	outer, body        string
	function, envelope map[string]any
}

func p15FTNew(actions []map[string]any) *p15FTCursor {
	var c = &p15FTCursor{p15SCCursor: p15SCCursor{actions: actions, groups: map[string]bool{}}, points: map[string]int{}}
	c.err = p15SCReferences(actions)
	return c
}
func (c *p15FTCursor) mark(name string) { c.points[name] = c.index }
func p15FTUUIDOnly(ref map[string]any) map[string]any {
	return map[string]any{"Type": "ActionOutput", "OutputUUID": ref["OutputUUID"]}
}
func (c *p15FTCursor) bare(kind string) {
	if c.err != nil {
		return
	}
	if c.index >= len(c.actions) || !reflect.DeepEqual(c.actions[c.index], map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + kind}) {
		c.err = fmt.Errorf("bare %s envelope differs at %d", kind, c.index)
		return
	}
	c.index++
}
func (c *p15FTCursor) condition(input map[string]any, code uint64, key string, right any) string {
	if key != "" {
		return c.start(input, code, key, right)
	}
	if c.err != nil {
		return ""
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing native condition")
		return ""
	}
	var group, ok = p15SCParams(c.actions[c.index])["GroupingIdentifier"].(string)
	if !ok || !p15SCCanonicalUUID(group) || c.groups[strings.ToLower(group)] {
		c.err = fmt.Errorf("invalid or reused native group")
		return ""
	}
	c.groups[strings.ToLower(group)] = true
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": uint64(0), "WFCondition": code, "WFConditionalLegacyComparisonBehavior": true, "WFInput": map[string]any{"Type": "Variable", "Variable": p15SCAttachment(input)}}, false)
	return group
}
func (c *p15FTCursor) field(key string, dictionary map[string]any, name string) map[string]any {
	// Compiler-internal dictionary reads deliberately carry the UUID only.
	return c.take("getvalueforkey", name, map[string]any{"WFDictionaryKey": key, "WFGetDictionaryValueType": "Value", "WFInput": p15SCAttachment(p15FTUUIDOnly(dictionary))}, true)
}
func (c *p15FTCursor) dictionary(name string, fields map[string]any) map[string]any {
	var _, wire = p15ContainerFieldValue(fields)
	return c.take("dictionary", name, map[string]any{"WFItems": wire}, true)
}
func (c *p15FTCursor) output(ref map[string]any) {
	c.take("output", "", map[string]any{"WFOutput": p15SCRefToken(ref)}, false)
}
func (c *p15FTCursor) numberCast(ref map[string]any, name string) map[string]any {
	return c.take("detect.number", name, map[string]any{"WFInput": p15SCAttachment(p15FTUUIDOnly(ref))}, true)
}
func (c *p15FTCursor) math(operation string, left map[string]any, right any, name string) map[string]any {
	return c.take("math", name, map[string]any{"WFInput": p15SCAttachment(left), "WFMathOperation": operation, "WFMathOperand": right}, true)
}

func (c *p15FTCursor) header() {
	var input = map[string]any{"Type": "ExtensionInput"}
	var zero = c.take("gettext", "", map[string]any{"WFTextActionText": "0"}, true)
	c.store("_cherri_is_fn", zero)
	c.store("_cherri_caller_input", input)
	var provided = c.condition(input, 100, "", nil)
	var possible = c.take("detect.dictionary", "", map[string]any{"WFInput": p15SCAttachment(input)}, true)
	var marker = c.field("cherri_functions", possible, "")
	var magic = c.field("_cherri_magic", possible, "")
	var valid = c.condition(magic, 4, "WFConditionalActionString", "__cherri_fn_envelope_v2__")
	var marked = c.condition(marker, 4, "WFConditionalActionString", "1")
	var one = c.take("gettext", "", map[string]any{"WFTextActionText": "1"}, true)
	c.store("_cherri_is_fn", one)
	var caller = c.field("_cherri_caller_input", possible, "")
	c.store("_cherri_caller_input", caller)
	c.end(marked, "", false)
	c.end(valid, "", false)
	c.end(provided, "", false)
	c.outer = c.condition(p15SCVariable("_cherri_is_fn"), 4, "WFConditionalActionString", "1")
	c.envelope = c.take("detect.dictionary", "", map[string]any{"WFInput": p15SCAttachment(input)}, true)
	c.function = c.field("function", c.envelope, "")
	c.field("arguments", c.envelope, "")
	// This matches the currently generated well-formed-call path. It grants no
	// RP26/BRG15 malformed-envelope credit. The independently verified dual-marker
	// repair is an existing dependency, not new function-transport acceptance.
}
func (c *p15FTCursor) beginFunction(name string) {
	if c.body != "" {
		c.end(c.body, "", false)
	}
	c.body = c.condition(c.function, 4, "WFConditionalActionString", name)
}
func (c *p15FTCursor) caller() {
	c.end(c.body, "", false)
	c.bare("exit")
	c.alternate(c.outer)
}

type p15FTArgument struct {
	supplied bool
	value    any
}

func (c *p15FTCursor) payload(name string, args ...p15FTArgument) map[string]any {
	var fields = map[string]any{"_cherri_caller_input": p15ContainerReference(p15SCVariable("_cherri_caller_input")), "_cherri_magic": "__cherri_fn_envelope_v2__", "cherri_functions": "1", "function": name}
	var values = []any{}
	for i, arg := range args {
		var flag = "0"
		var value = any("")
		if arg.supplied {
			flag = "1"
			value = arg.value
			fields[fmt.Sprintf("arg_%d", i)] = value
		}
		fields[fmt.Sprintf("has_arg_%d", i)] = flag
		values = append(values, value)
	}
	fields["arguments"] = values
	var _, wire = p15ContainerFieldValue(fields)
	return c.take("dictionary", "FunctionPayload", map[string]any{"WFItems": wire}, true)
}
func (c *p15FTCursor) call(payload map[string]any, name string) map[string]any {
	return c.take("runworkflow", name, map[string]any{"WFInput": p15SCAttachment(payload), "WFWorkflowName": "p15-function-transport", "WFWorkflow": map[string]any{"isSelf": true, "workflowName": "p15-function-transport"}}, true)
}
func (c *p15FTCursor) optionalCarrier(marker string, present bool, value any) map[string]any {
	var fields = map[string]any{marker: "1", "has_value": "0"}
	if present {
		fields["has_value"] = "1"
		fields["value"] = value
	}
	return c.dictionary("OptionalValue", fields)
}
func (c *p15FTCursor) decodeOptional(dictionary map[string]any, name, prefix string) (map[string]any, map[string]any) {
	c.mark(prefix + "_presence")
	var presence = c.field("has_value", dictionary, "")
	c.mark(prefix + "_guard")
	var group = c.condition(presence, 4, "WFConditionalActionString", "1")
	c.field("value", dictionary, "")
	c.alternate(group)
	c.mark(prefix + "_nothing")
	c.bare("nothing")
	c.mark(prefix + "_value")
	var value = c.end(group, name, true)
	return value, presence
}
func (c *p15FTCursor) finish() error {
	c.end(c.outer, "", false)
	if c.err != nil {
		return c.err
	}
	if c.index != len(c.actions) {
		return fmt.Errorf("extra native actions: consumed %d of %d", c.index, len(c.actions))
	}
	return nil
}
func p15FTCompile(t *testing.T, source string) []map[string]any {
	t.Helper()
	var raw, err = CompileSourceToPlist("p15-function-transport.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P15_FUNCTION_NATIVE source_sha256=%x native_sha256=%x bytes=%d base64=%s", sha256.Sum256([]byte(source)), sha256.Sum256(raw), len(raw), base64.StdEncoding.EncodeToString(raw))
	var root map[string]any
	if _, err = plist.Unmarshal(raw, &root); err != nil {
		t.Fatal(err)
	}
	var rows, ok = root["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("native action array absent")
	}
	var actions = make([]map[string]any, len(rows))
	for i, row := range rows {
		var yes bool
		actions[i], yes = row.(map[string]any)
		if !yes {
			t.Fatalf("native action %d is not an envelope", i)
		}
	}
	return actions
}
func p15FTRewrite(value any, key string, old, want any) {
	switch v := value.(type) {
	case map[string]any:
		for k, item := range v {
			if k == key && reflect.DeepEqual(item, old) {
				v[k] = want
			} else {
				p15FTRewrite(item, key, old, want)
			}
		}
	case []any:
		for _, item := range v {
			p15FTRewrite(item, key, old, want)
		}
	}
}
func p15FTReconcileIdentity(actions []map[string]any, index int, want string) {
	var old = p15SCParams(actions[index])["UUID"]
	p15SCParams(actions[index])["UUID"] = want
	for _, action := range actions {
		p15FTRewrite(action, "OutputUUID", old, want)
	}
}
func p15FTCommonControls(t *testing.T, actions []map[string]any, verify func([]map[string]any) error) {
	t.Helper()
	for _, tc := range []struct {
		name   string
		mutate func([]map[string]any) []map[string]any
	}{
		{"missing_action", func(a []map[string]any) []map[string]any { return a[1:] }},
		{"extra_action", func(a []map[string]any) []map[string]any {
			return append(a, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
		}},
		{"zero_producer_reconciled", func(a []map[string]any) []map[string]any {
			p15FTReconcileIdentity(a, 0, "00000000-0000-0000-0000-000000000000")
			return a
		}},
		{"malformed_producer_reconciled", func(a []map[string]any) []map[string]any {
			p15FTReconcileIdentity(a, 0, "synthetic-invalid-uuid")
			return a
		}},
		{"duplicate_producer", func(a []map[string]any) []map[string]any {
			p15SCParams(a[4])["UUID"] = p15SCParams(a[0])["UUID"]
			return a
		}},
		{"zero_group_reconciled", func(a []map[string]any) []map[string]any {
			var old = p15SCParams(a[3])["GroupingIdentifier"]
			for _, row := range a {
				p15FTRewrite(row, "GroupingIdentifier", old, "00000000-0000-0000-0000-000000000000")
			}
			return a
		}},
		{"malformed_group_reconciled", func(a []map[string]any) []map[string]any {
			var old = p15SCParams(a[3])["GroupingIdentifier"]
			for _, row := range a {
				p15FTRewrite(row, "GroupingIdentifier", old, "synthetic-invalid-group")
			}
			return a
		}},
		{"real_control_code", func(a []map[string]any) []map[string]any { p15SCParams(a[3])["WFCondition"] = float64(100); return a }},
	} {
		t.Run("oracle/"+tc.name, func(t *testing.T) {
			var changed = tc.mutate(p15SCClone(t, actions))
			if verify(changed) == nil {
				t.Fatal("damaged whole typed function graph accepted")
			}
		})
	}
}
