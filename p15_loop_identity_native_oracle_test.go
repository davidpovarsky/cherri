package main

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode/utf16"
)

var p15LoopUUIDPattern = regexp.MustCompile("^[0-9A-Fa-f]{8}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{4}-[0-9A-Fa-f]{12}$")
var p15LoopShadowPattern = regexp.MustCompile("^__cherri_scope_[0-9]+_(index|item)$")

type p15LoopOutput struct{ UUID, Name string }
type p15LoopOwner struct{ ID, Group string }
type p15LoopCursor struct {
	actions    []map[string]any
	at         int
	err        error
	identities map[string]bool
	names      map[string]bool
	bindings   map[string]string
	stack      []p15LoopOwner
}

func p15LoopAttachment(kind, identity, name string) map[string]any {
	value := map[string]any{"Type": kind}
	if kind == "Variable" {
		value["VariableName"] = identity
	} else {
		value["OutputUUID"] = identity
		if name != "" {
			value["OutputName"] = name
		}
	}
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": value}
}

func (output p15LoopOutput) edge() map[string]any {
	return p15LoopAttachment("ActionOutput", output.UUID, output.Name)
}

func (c *p15LoopCursor) fail(format string, args ...any) {
	if c.err == nil {
		c.err = fmt.Errorf("native loop graph at action %d: %s", c.at, fmt.Sprintf(format, args...))
	}
}

func (c *p15LoopCursor) peek() map[string]any {
	if c.at >= len(c.actions) {
		c.fail("missing required action")
		return nil
	}
	params, ok := c.actions[c.at]["WFWorkflowActionParameters"].(map[string]any)
	if !ok {
		c.fail("missing parameter dictionary")
		return nil
	}
	return params
}

func (c *p15LoopCursor) take(identifier string, want map[string]any) {
	if c.err != nil {
		return
	}
	actual := c.peek()
	if c.err != nil {
		return
	}
	action := c.actions[c.at]
	if len(action) != 2 || action["WFWorkflowActionIdentifier"] != identifier || !reflect.DeepEqual(actual, want) {
		c.fail("want %s %#v, got %#v", identifier, want, action)
		return
	}
	c.at++
}

func (c *p15LoopCursor) identity(id string) {
	if !p15LoopUUIDPattern.MatchString(id) || c.identities[id] {
		c.fail("missing, malformed or repeated producer/loop identity %q", id)
		return
	}
	c.identities[id] = true
}

func (c *p15LoopCursor) produce(identifier string, parameters map[string]any, displayName bool) p15LoopOutput {
	if c.err != nil {
		return p15LoopOutput{}
	}
	params := c.peek()
	uuid, _ := params["UUID"].(string)
	name, _ := params["CustomOutputName"].(string)
	c.identity(uuid)
	if displayName && name == "" {
		c.fail("native producer lost display identity")
	}
	output := p15LoopOutput{uuid, name}
	parameters["UUID"] = uuid
	if displayName {
		parameters["CustomOutputName"] = name
	}
	c.take(identifier, parameters)
	return output
}

func (c *p15LoopCursor) bind(logical, sourceName string, shadow bool, input map[string]any) {
	if c.err != nil {
		return
	}
	name, _ := c.peek()["WFVariableName"].(string)
	if shadow {
		if !p15LoopShadowPattern.MatchString(name) || !strings.HasSuffix(name, "_"+sourceName) {
			c.fail("shadow binding %s lost distinct scoped identity: %q", logical, name)
		}
	} else if name != sourceName {
		c.fail("binding %s uses %q, want %q", logical, name, sourceName)
	}
	if c.names[name] || c.bindings[logical] != "" {
		c.fail("binding identities collide for %s: %q", logical, name)
	}
	c.names[name] = true
	c.bindings[logical] = name
	c.take("is.workflow.actions.setvariable", map[string]any{"WFVariableName": name, "WFInput": input})
}

func (c *p15LoopCursor) nativeIndex() string {
	if len(c.stack) == 1 {
		return "Repeat Index"
	}
	return fmt.Sprintf("Repeat Index %d", len(c.stack))
}
func (c *p15LoopCursor) nativeItem() string {
	if len(c.stack) == 1 {
		return "Repeat Item"
	}
	return fmt.Sprintf("Repeat Item %d", len(c.stack))
}
func (c *p15LoopCursor) captureIndex(logical, name string, shadow bool) {
	converted := c.produce("is.workflow.actions.math", map[string]any{
		"WFInput":         p15LoopAttachment("Variable", c.nativeIndex(), ""),
		"WFMathOperation": "-", "WFMathOperand": float64(1),
	}, false)
	c.bind(logical+".index", name, shadow, converted.edge())
}

func (c *p15LoopCursor) begin(identifier string, parameters map[string]any) {
	if c.err != nil {
		return
	}
	group, _ := c.peek()["GroupingIdentifier"].(string)
	c.identity(group)
	parameters["GroupingIdentifier"] = group
	parameters["WFControlFlowMode"] = uint64(0)
	c.take(identifier, parameters)
	c.stack = append(c.stack, p15LoopOwner{identifier, group})
}

func (c *p15LoopCursor) forLoop(logical, index, item string, shadow bool, values ...any) {
	rows := make([]any, 0, len(values))
	for _, value := range values {
		var field any
		switch typed := value.(type) {
		case string:
			field = typed
		case int:
			field = c.produce("is.workflow.actions.number", map[string]any{"WFNumberActionNumber": uint64(typed)}, true).edge()
		default:
			c.fail("unsupported authored iterator oracle value %T", value)
		}
		rows = append(rows, map[string]any{"WFItemType": uint64(0), "WFValue": field})
	}
	list := c.produce("is.workflow.actions.list", map[string]any{"WFItems": rows}, true)
	c.begin("is.workflow.actions.repeat.each", map[string]any{"WFInput": list.edge()})
	c.bind(logical+".item", item, shadow, p15LoopAttachment("Variable", c.nativeItem(), ""))
	c.captureIndex(logical, index, shadow)
}
func (c *p15LoopCursor) repeat(logical, index string, shadow bool, count int) {
	c.begin("is.workflow.actions.repeat.count", map[string]any{"WFRepeatCount": uint64(count)})
	c.captureIndex(logical, index, shadow)
}
func (c *p15LoopCursor) end() {
	if len(c.stack) == 0 {
		c.fail("unowned loop end")
		return
	}
	owner := c.stack[len(c.stack)-1]
	c.take(owner.ID, map[string]any{"GroupingIdentifier": owner.Group, "WFControlFlowMode": uint64(2)})
	c.stack = c.stack[:len(c.stack)-1]
}
func (c *p15LoopCursor) globals(index int) {
	n := c.produce("is.workflow.actions.number", map[string]any{"WFNumberActionNumber": uint64(index)}, true)
	c.bind("global.index", "index", false, n.edge())
	s := c.produce("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "outside"}, true)
	c.bind("global.item", "item", false, s.edge())
}
func (c *p15LoopCursor) show(marker string, logical ...string) {
	text := marker + ":"
	attachments := map[string]any{}
	for i, key := range logical {
		if i > 0 {
			text += ":"
		}
		name, found := c.bindings[key]
		if !found {
			c.fail("missing authored binding %s", key)
		}
		attachments[fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(text))))] = map[string]any{"Type": "Variable", "VariableName": name}
		text += "\ufffc"
	}
	c.take("is.workflow.actions.showresult", map[string]any{"Text": map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value":               map[string]any{"string": text, "attachmentsByRange": attachments},
	}})
}

// These are independently authored complete native graphs for the three source
// scenarios. They check serialized owner/capture/consumer edges and literal
// iterator cardinality/order. They do not simulate or claim Apple execution.
func p15LoopNativeError(id string, actions []map[string]any) error {
	c := &p15LoopCursor{actions: actions, identities: map[string]bool{}, names: map[string]bool{}, bindings: map[string]string{}}
	switch id {
	case "F02":
		c.forLoop("outer", "outerIndex", "outerItem", false, "A", "B")
		c.forLoop("inner", "innerIndex", "innerItem", false, 1, 2)
		c.show("F02_PAIR", "outer.item", "inner.item", "outer.index", "inner.index")
		c.end()
		c.show("F02_AFTER", "outer.item", "outer.index")
		c.end()
		c.repeat("repeat", "repeatIndex", false, 2)
		c.show("F02_REPEAT", "repeat.index")
		c.end()
	case "RP22":
		c.globals(99)
		c.forLoop("outer", "index", "item", true, "A", "B")
		c.show("RP22_OUTER", "outer.index", "outer.item")
		c.repeat("middle", "index", true, 2)
		c.show("RP22_REPEAT", "middle.index", "outer.item")
		c.forLoop("inner", "index", "item", true, 1, 2)
		c.show("RP22_INNER", "inner.index", "inner.item")
		c.end()
		c.show("RP22_REPEAT_AFTER", "middle.index", "outer.item")
		c.end()
		c.show("RP22_OUTER_AFTER", "outer.index", "outer.item")
		c.end()
		c.show("RP22_RESTORED", "global.index", "global.item")
	case "BRG12":
		c.forLoop("outer", "outerIndex", "outerItem", false, "A", "B")
		c.forLoop("inner", "innerIndex", "innerItem", false, 1, 2)
		c.show("BRG12_PAIR", "outer.item", "inner.item", "outer.index", "inner.index")
		c.end()
		c.repeat("middle", "repeatIndex", false, 2)
		c.show("BRG12_MIXED", "outer.item", "outer.index", "middle.index")
		c.end()
		c.show("BRG12_AFTER", "outer.item", "outer.index")
		c.end()
		c.forLoop("sibling", "siblingIndex", "siblingItem", false, "C", "D")
		c.show("BRG12_SIBLING", "sibling.item", "sibling.index")
		c.end()
		c.repeat("reverse", "finalIndex", false, 2)
		c.forLoop("nested", "nestedIndex", "nestedItem", false, "X", "Y")
		c.show("BRG12_REVERSE", "reverse.index", "nested.index", "nested.item")
		c.end()
		c.show("BRG12_REVERSE_AFTER", "reverse.index")
		c.end()
		c.globals(77)
		c.forLoop("shadowOuter", "index", "item", true, "S")
		c.repeat("shadowInner", "index", true, 2)
		c.show("BRG12_SHADOW_INNER", "shadowInner.index", "shadowOuter.item")
		c.end()
		c.show("BRG12_SHADOW_OUTER", "shadowOuter.index", "shadowOuter.item")
		c.end()
		c.show("BRG12_RESTORED", "global.index", "global.item")
	default:
		return fmt.Errorf("unknown loop scenario %s", id)
	}
	if c.err != nil {
		return c.err
	}
	if err := p15M02MainVoidBoundaryError(actions); err != nil {
		return err
	}
	if c.at != len(actions)-1 || len(c.stack) != 0 {
		return fmt.Errorf("unconsumed or unclosed native loop graph: at=%d actions=%d owners=%d", c.at, len(actions), len(c.stack))
	}
	return nil
}

func p15LoopClone(value any) any {
	switch v := value.(type) {
	case []map[string]any:
		out := make([]map[string]any, len(v))
		for i, x := range v {
			out[i] = p15LoopClone(x).(map[string]any)
		}
		return out
	case []any:
		out := make([]any, len(v))
		for i, x := range v {
			out[i] = p15LoopClone(x)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for k, x := range v {
			out[k] = p15LoopClone(x)
		}
		return out
	}
	return value
}

func p15LoopRewrite(value any, rewrite func(string) string) {
	switch v := value.(type) {
	case []map[string]any:
		for _, x := range v {
			p15LoopRewrite(x, rewrite)
		}
	case []any:
		for i, x := range v {
			if s, ok := x.(string); ok {
				v[i] = rewrite(s)
			} else {
				p15LoopRewrite(x, rewrite)
			}
		}
	case map[string]any:
		for k, x := range v {
			if s, ok := x.(string); ok {
				v[k] = rewrite(s)
			} else {
				p15LoopRewrite(x, rewrite)
			}
		}
	}
}

func p15LoopRenameIdentities(actions []map[string]any, shadow bool) {
	replacements := map[string]string{}
	p15LoopRewrite(actions, func(value string) string {
		relevant := p15LoopUUIDPattern.MatchString(value)
		if shadow {
			relevant = p15LoopShadowPattern.MatchString(value)
		}
		if !relevant {
			return value
		}
		if replacement, found := replacements[value]; found {
			return replacement
		}
		next := len(replacements) + 100
		replacement := fmt.Sprintf("00000000-0000-4000-8000-%012d", next)
		if shadow {
			replacement = fmt.Sprintf("__cherri_scope_%d_%s", next, value[strings.LastIndex(value, "_")+1:])
		}
		replacements[value] = replacement
		return replacement
	})
}

func p15LoopMutationNames(id string) []string {
	names := []string{
		"one_based_capture", "wrong_index_owner", "wrong_item_owner", "wrong_offset", "wrong_direction",
		"string_offset", "dangling_index_edge", "duplicate_producer_identity", "reused_loop_owner", "wrong_end_owner",
		"iteration_order", "iteration_cardinality", "repeat_count", "consumer_wrong_owner", "post_inner_owner_lost",
		"capture_escaped_loop", "missing_action", "extra_action",
	}
	if id != "F02" {
		names = append(names, "shadow_leaks_after_scope")
	}
	return names
}

func p15LoopMutate(actions []map[string]any, fault string) []map[string]any {
	var maths, items, begins, ends, lists, counts, shows []int
	for i, action := range actions {
		p := p15Param(action)
		switch action["WFWorkflowActionIdentifier"] {
		case "is.workflow.actions.math":
			maths = append(maths, i)
		case "is.workflow.actions.list":
			lists = append(lists, i)
		case "is.workflow.actions.showresult":
			shows = append(shows, i)
		case "is.workflow.actions.repeat.each", "is.workflow.actions.repeat.count":
			if p15Int(p["WFControlFlowMode"]) == 0 {
				begins = append(begins, i)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.repeat.count" {
					counts = append(counts, i)
				}
			} else {
				ends = append(ends, i)
			}
		case "is.workflow.actions.setvariable":
			if envelope, ok := p["WFInput"].(map[string]any); ok {
				value, _ := envelope["Value"].(map[string]any)
				name, _ := value["VariableName"].(string)
				if strings.HasPrefix(name, "Repeat Item") {
					items = append(items, i)
				}
			}
		}
	}
	math := p15Param(actions[maths[0]])
	capture := p15Param(actions[maths[0]+1])
	switch fault {
	case "one_based_capture":
		capture["WFInput"] = p15LoopClone(math["WFInput"])
	case "wrong_index_owner":
		math["WFInput"] = p15LoopAttachment("Variable", "Repeat Index 99", "")
	case "wrong_item_owner":
		p15Param(actions[items[0]])["WFInput"] = p15LoopAttachment("Variable", "Repeat Item 99", "")
	case "wrong_offset":
		math["WFMathOperand"] = float64(2)
	case "wrong_direction":
		math["WFMathOperation"] = "+"
	case "string_offset":
		math["WFMathOperand"] = "1"
	case "dangling_index_edge":
		capture["WFInput"] = p15LoopAttachment("ActionOutput", "FFFFFFFF-FFFF-4FFF-8FFF-FFFFFFFFFFFF", "")
	case "duplicate_producer_identity":
		old := p15Param(actions[maths[1]])["UUID"].(string)
		replacement := math["UUID"].(string)
		p15LoopRewrite(actions, func(s string) string {
			if s == old {
				return replacement
			}
			return s
		})
	case "reused_loop_owner":
		old := p15Param(actions[begins[1]])["GroupingIdentifier"].(string)
		replacement := p15Param(actions[begins[0]])["GroupingIdentifier"].(string)
		p15LoopRewrite(actions, func(s string) string {
			if s == old {
				return replacement
			}
			return s
		})
	case "wrong_end_owner":
		p15Param(actions[ends[0]])["GroupingIdentifier"] = "FFFFFFFF-FFFF-4FFF-8FFF-FFFFFFFFFFFF"
	case "iteration_order":
		rows := p15Param(actions[lists[0]])["WFItems"].([]any)
		rows[0], rows[1] = rows[1], rows[0]
	case "iteration_cardinality":
		rows := p15Param(actions[lists[0]])["WFItems"].([]any)
		p15Param(actions[lists[0]])["WFItems"] = rows[:len(rows)-1]
	case "repeat_count":
		p15Param(actions[counts[0]])["WFRepeatCount"] = uint64(3)
	case "consumer_wrong_owner":
		p15LoopRewrite(p15Param(actions[shows[0]])["Text"], func(s string) string {
			if s == capture["WFVariableName"] {
				return "wrongLoopOwner"
			}
			return s
		})
	case "post_inner_owner_lost":
		for _, i := range shows {
			text := p15Param(actions[i])["Text"].(map[string]any)["Value"].(map[string]any)
			if strings.Contains(text["string"].(string), "_AFTER:") {
				refs := text["attachmentsByRange"].(map[string]any)
				wrong := p15Param(actions[maths[1]+1])["WFVariableName"]
				for _, ref := range refs {
					ref.(map[string]any)["VariableName"] = wrong
				}
				break
			}
		}
	case "capture_escaped_loop":
		at := maths[0] + 1
		action := actions[at]
		actions = append(actions[:at], actions[at+1:]...)
		actions = append(actions, action)
	case "missing_action":
		actions = append(actions[:shows[0]], actions[shows[0]+1:]...)
	case "extra_action":
		actions = append(actions, p15LoopClone(actions[shows[0]]).(map[string]any))
	case "shadow_leaks_after_scope":
		last := p15Param(actions[shows[len(shows)-1]])["Text"]
		shadow := capture["WFVariableName"].(string)
		if !p15LoopShadowPattern.MatchString(shadow) {
			for _, i := range maths {
				name := p15Param(actions[i+1])["WFVariableName"].(string)
				if p15LoopShadowPattern.MatchString(name) {
					shadow = name
					break
				}
			}
		}
		p15LoopRewrite(last, func(s string) string {
			if s == "index" {
				return shadow
			}
			return s
		})
	default:
		panic("unknown loop causal mutation")
	}
	return actions
}
