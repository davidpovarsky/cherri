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

	"howett.net/plist"
)

// This is a pending causal regression, not a Local179 producer. A native
// activation must be guarded by both exact private marker values. That is
// necessary, but does not prove complete function/argument/caller envelopes.
type p15FEGuard struct {
	Group, Field, Value string
	Then bool
}
type p15FEWitness struct {
	Activation int
	Guards []p15FEGuard
}
func p15FERef(value any) (map[string]any, bool) {
	wrapper, ok := value.(map[string]any)
	if !ok || len(wrapper)!=2 || wrapper["WFSerializationType"]!="WFTextTokenAttachment" { return nil,false }
	ref, ok := wrapper["Value"].(map[string]any)
	return ref,ok
}
func p15FEActivationWitness(actions []map[string]any) (p15FEWitness,error) {
	var witness p15FEWitness
	if err:=p15SCReferences(actions);err!=nil{return witness,err}
	producers:=map[string]int{}
	for i,action:=range actions {
		if id,ok:=p15SCParams(action)["UUID"].(string);ok{producers[strings.ToLower(id)]=i}
	}
	producer:=func(value any)(int,bool){
		ref,ok:=p15FERef(value);if !ok||ref["Type"]!="ActionOutput"{return 0,false}
		id,ok:=ref["OutputUUID"].(string);if !ok{return 0,false}
		i,found:=producers[strings.ToLower(id)];return i,found
	}
	field:=func(value any) string{
		i,ok:=producer(value);if !ok{return ""}
		action:=actions[i];p:=p15SCParams(action)
		if action["WFWorkflowActionIdentifier"]!="is.workflow.actions.getvalueforkey" || p["WFGetDictionaryValueType"]!="Value"{return ""}
		d,ok:=producer(p["WFInput"]);if !ok{return ""}
		if actions[d]["WFWorkflowActionIdentifier"]!="is.workflow.actions.detect.dictionary"{return ""}
		ref,ok:=p15FERef(p15SCParams(actions[d])["WFInput"]);if !ok||len(ref)!=1||ref["Type"]!="ExtensionInput"{return ""}
		name,_:=p["WFDictionaryKey"].(string);return name
	}
	var stack []p15FEGuard
	usedGroups:=map[string]bool{}
	activations:=0
	for i,action:=range actions{
		p:=p15SCParams(action)
		if action["WFWorkflowActionIdentifier"]=="is.workflow.actions.conditional"{
			group,ok:=p["GroupingIdentifier"].(string)
			if !ok||!p15SCCanonicalUUID(group){return witness,fmt.Errorf("invalid dispatch group")}
			mode,ok:=p["WFControlFlowMode"].(uint64);if !ok{return witness,fmt.Errorf("dispatch control kind")}
			switch mode{
			case 0:
				if usedGroups[strings.ToLower(group)]{return witness,fmt.Errorf("duplicate dispatch group")}
				usedGroups[strings.ToLower(group)]=true
				var guard=p15FEGuard{Group:group,Then:true}
				input,ok:=p["WFInput"].(map[string]any)
				if ok&&len(input)==2&&input["Type"]=="Variable"&&p["WFCondition"]==uint64(4)&&p["WFConditionalLegacyComparisonBehavior"]==true{
					guard.Field=field(input["Variable"])
					guard.Value,_=p["WFConditionalActionString"].(string)
				}
				stack=append(stack,guard)
			case 1,2:
				if len(stack)==0||stack[len(stack)-1].Group!=group{return witness,fmt.Errorf("dispatch boundary ancestry")}
				if mode==1{
					if !stack[len(stack)-1].Then{return witness,fmt.Errorf("duplicate dispatch else")}
					stack[len(stack)-1].Then=false
				}else{stack=stack[:len(stack)-1]}
			default:return witness,fmt.Errorf("invalid dispatch mode")
			}
		}
		if action["WFWorkflowActionIdentifier"]!="is.workflow.actions.setvariable"||p["WFVariableName"]!="_cherri_is_fn"{continue}
		j,ok:=producer(p["WFInput"]);if !ok{return witness,fmt.Errorf("dispatch state has no explicit producer")}
		if actions[j]["WFWorkflowActionIdentifier"]!="is.workflow.actions.gettext"{return witness,fmt.Errorf("dispatch state is not explicit Text")}
		value,ok:=p15SCParams(actions[j])["WFTextActionText"].(string)
		if !ok{return witness,fmt.Errorf("dispatch state literal missing")}
		if value=="0"{continue}
		if value!="1"{return witness,fmt.Errorf("unknown dispatch state")}
		activations++
		if activations!=1{return witness,fmt.Errorf("duplicate dispatch activation")}
		witness=p15FEWitness{Activation:i,Guards:append([]p15FEGuard(nil),stack...)}
		required:=map[string]string{"_cherri_magic":"__cherri_fn_envelope_v2__","cherri_functions":"1"}
		for _,guard:=range stack{
			if guard.Then&&required[guard.Field]==guard.Value&&guard.Value!=""{delete(required,guard.Field)}
		}
		for _,name:=range []string{"_cherri_magic","cherri_functions"}{
			if _,missing:=required[name];missing{return witness,fmt.Errorf("dispatch activation lacks exact true-branch %s guard",name)}
		}
	}
	if activations==0{return witness,fmt.Errorf("dispatch activation missing")}
	if len(stack)!=0{return witness,fmt.Errorf("dispatch guard boundary incomplete")}
	return witness,nil
}

func p15FESyntheticGuardGraph() []map[string]any {
	id:=func(i int)string{return fmt.Sprintf("11111111-1111-4111-8111-%012d",i)}
	ref:=func(i int)map[string]any{return map[string]any{"Type":"ActionOutput","OutputUUID":id(i)}}
	wrap:=func(r map[string]any)map[string]any{return map[string]any{"WFSerializationType":"WFTextTokenAttachment","Value":r}}
	act:=func(kind string,p map[string]any)map[string]any{return map[string]any{"WFWorkflowActionIdentifier":"is.workflow.actions."+kind,"WFWorkflowActionParameters":p}}
	get:=func(i int,key string)map[string]any{return act("getvalueforkey",map[string]any{"UUID":id(i),"WFDictionaryKey":key,"WFGetDictionaryValueType":"Value","WFInput":wrap(ref(1))})}
	condition:=func(g,n int,value string)map[string]any{return act("conditional",map[string]any{"GroupingIdentifier":id(g),"WFControlFlowMode":uint64(0),"WFCondition":uint64(4),"WFConditionalLegacyComparisonBehavior":true,"WFConditionalActionString":value,"WFInput":map[string]any{"Type":"Variable","Variable":wrap(ref(n))}})}
	return []map[string]any{
		act("detect.dictionary",map[string]any{"UUID":id(1),"WFInput":wrap(map[string]any{"Type":"ExtensionInput"})}),
		get(2,"_cherri_magic"),condition(20,2,"__cherri_fn_envelope_v2__"),
		get(3,"cherri_functions"),condition(21,3,"1"),
		act("gettext",map[string]any{"UUID":id(4),"WFTextActionText":"1"}),
		act("setvariable",map[string]any{"WFVariableName":"_cherri_is_fn","WFInput":wrap(ref(4))}),
		act("conditional",map[string]any{"GroupingIdentifier":id(21),"WFControlFlowMode":uint64(2)}),
		act("conditional",map[string]any{"GroupingIdentifier":id(20),"WFControlFlowMode":uint64(2)}),
	}
}
func TestP15RP26DispatchGuardObserverControls(t *testing.T){
	// Synthetic observer controls are explicitly separate from the actual
	// compiler regression below and never supply requirement-tier evidence.
	original:=p15FESyntheticGuardGraph()
	if _,err:=p15FEActivationWitness(original);err!=nil{t.Fatal(err)}
	for _,fault:=range []string{"magic_field","magic_value","marker_field","marker_value","marker_else","magic_else","foreign_dictionary","wrong_kind"}{
		t.Run(fault,func(t *testing.T){
			bad:=p15SCClone(t,original)
			switch fault{
			case "magic_field":p15SCParams(bad[1])["WFDictionaryKey"]="foreign"
			case "magic_value":p15SCParams(bad[2])["WFConditionalActionString"]="other"
			case "marker_field":p15SCParams(bad[3])["WFDictionaryKey"]="arguments"
			case "marker_value":p15SCParams(bad[4])["WFConditionalActionString"]="0"
			case "marker_else":
				group:=p15SCParams(bad[4])["GroupingIdentifier"]
				extra:=map[string]any{"WFWorkflowActionIdentifier":"is.workflow.actions.conditional","WFWorkflowActionParameters":map[string]any{"GroupingIdentifier":group,"WFControlFlowMode":uint64(1)}}
				bad=append(bad[:5],append([]map[string]any{extra},bad[5:]...)...)
			case "magic_else":
				group:=p15SCParams(bad[2])["GroupingIdentifier"]
				extra:=map[string]any{"WFWorkflowActionIdentifier":"is.workflow.actions.conditional","WFWorkflowActionParameters":map[string]any{"GroupingIdentifier":group,"WFControlFlowMode":uint64(1)}}
				bad=append(bad[:3],append([]map[string]any{extra},bad[3:]...)...)
			case "foreign_dictionary":
				p15SCParams(bad[0])["WFInput"]=map[string]any{"WFSerializationType":"WFTextTokenAttachment","Value":map[string]any{"Type":"Variable","VariableName":"other"}}
			case "wrong_kind":p15SCParams(bad[4])["WFCondition"]=float64(4)
			}
			if _,err:=p15FEActivationWitness(bad);err==nil{t.Fatal("observer accepted causal damage",fault)}
		})
	}
}
func TestP15RP26IncompleteEnvelopeRequiresGuard(t *testing.T){
	data,err:=os.ReadFile(filepath.Join("tests","language-v2","v2-rp26-envelope-regression.cherri"))
	if err!=nil{t.Fatal(err)}
	actions:=p15FECompile(t,string(data))
	witness,err:=p15FEActivationWitness(actions)
	encoded,marshalErr:=json.Marshal(witness);if marshalErr!=nil{t.Fatal(marshalErr)}
	t.Logf("RP26_DISPATCH_GUARD_WITNESS %s",encoded)
	if err!=nil{t.Fatal(err)}
	t.Log("Necessary dual-marker native dominance only; complete envelope validation, full RP26/BRG15 joined scenarios and Apple behavior remain unproved.")
}

// Every compilation here is the public canonical production path. Originals
// are retained in the direct Go JSON stream even when a later assertion fails.
func p15FECompile(t *testing.T, source string) []map[string]any {
	t.Helper()
	raw, err := CompileSourceToPlist("p15-rp26-envelope-regression.cherri", source)
	if err != nil { t.Fatal(err) }
	t.Logf("RP26_BASELINE_NATIVE source_sha256=%x native_sha256=%x bytes=%d plist_base64=%s", sha256.Sum256([]byte(source)), sha256.Sum256(raw), len(raw), base64.StdEncoding.EncodeToString(raw))
	var document map[string]any
	if _, err = plist.Unmarshal(raw, &document); err != nil { t.Fatal(err) }
	values, ok := document["WFWorkflowActions"].([]any)
	if !ok { t.Fatal("native action list missing") }
	actions := make([]map[string]any, len(values))
	for i, value := range values {
		var valid bool
		actions[i], valid = value.(map[string]any)
		if !valid { t.Fatal("native action malformed", i) }
	}
	return actions
}

type p15FEHealthyCursor struct {
	p15SCCursor
	positions map[string]int
}

func (c *p15FEHealthyCursor) mark(name string) { c.positions[name] = c.index }
func (c *p15FEHealthyCursor) field(key string, source map[string]any, name string) map[string]any {
	return c.take("getvalueforkey", name, map[string]any{
		"WFDictionaryKey": key, "WFGetDictionaryValueType": "Value",
		"WFInput": p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": source["OutputUUID"]}),
	}, true)
}
func (c *p15FEHealthyCursor) output(source map[string]any) {
	c.take("output", "", map[string]any{"WFOutput": p15SCRefToken(source)}, false)
}
func (c *p15FEHealthyCursor) bare(kind string) {
	if c.err != nil { return }
	if c.index >= len(c.actions) || !reflect.DeepEqual(c.actions[c.index], map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions." + kind}) {
		c.err = fmt.Errorf("bare %s envelope differs at %d", kind, c.index)
		return
	}
	c.index++
}
func (c *p15FEHealthyCursor) hasInput(input map[string]any) string {
	if c.err != nil { return "" }
	if c.index >= len(c.actions) { c.err = fmt.Errorf("missing caller-input guard"); return "" }
	group, ok := p15SCParams(c.actions[c.index])["GroupingIdentifier"].(string)
	if !ok || !p15SCCanonicalUUID(group) || c.groups[strings.ToLower(group)] {
		c.err = fmt.Errorf("invalid caller-input guard identity"); return ""
	}
	c.groups[strings.ToLower(group)] = true
	c.take("conditional", "", map[string]any{
		"GroupingIdentifier": group, "WFControlFlowMode": uint64(0), "WFCondition": uint64(100),
		"WFConditionalLegacyComparisonBehavior": true,
		"WFInput": map[string]any{"Type": "Variable", "Variable": p15SCAttachment(input)},
	}, false)
	return group
}
func (c *p15FEHealthyCursor) payload(function string, argument any) map[string]any {
	_, fields := p15ContainerFieldValue(map[string]any{
		"_cherri_caller_input": p15ContainerReference(p15SCVariable("_cherri_caller_input")),
		"_cherri_magic": "__cherri_fn_envelope_v2__", "cherri_functions": "1",
		"function": function, "has_arg_0": "1", "arg_0": argument, "arguments": []any{argument},
	})
	return c.take("dictionary", "FunctionPayload", map[string]any{"WFItems": fields}, true)
}
func (c *p15FEHealthyCursor) call(payload map[string]any, name string) map[string]any {
	const workflow = "p15-rp26-envelope-regression"
	return c.take("runworkflow", name, map[string]any{
		"WFInput": p15SCAttachment(payload), "WFWorkflowName": workflow,
		"WFWorkflow": map[string]any{"isSelf": true, "workflowName": workflow},
	}, true)
}
func p15FEHealthyText(refs ...map[string]any) map[string]any {
	attachments := map[string]any{}
	parts := make([]string, len(refs))
	for i, ref := range refs {
		parts[i] = "\ufffc"
		attachments[fmt.Sprintf("{%d, 1}", i*2)] = ref
	}
	return p15SCToken(strings.Join(parts, "|"), attachments)
}
func p15FEHealthySource(t *testing.T, kind string) string {
	t.Helper()
	switch kind {
	case "direct":
		return "function inspect(value:Text)->Text{return f\"{system.input}|{ShortcutInput}|{value}\"}\n" +
			"let before=f\"{system.input}|{ShortcutInput}\"\nlet result=inspect(\"VALUE\")\n" +
			"return f\"{before}|{result}|{system.input}|{ShortcutInput}\"\n"
	case "nested":
		data, err := os.ReadFile(filepath.Join("tests", "language-v2", "v2-rp26-envelope-regression.cherri"))
		if err != nil { t.Fatal(err) }
		return string(data)
	case "without_functions":
		return "return f\"{system.input}|{ShortcutInput}\"\n"
	default:
		t.Fatal("unknown fixed healthy case", kind)
		return ""
	}
}

// The baseline healthy graph pins today's complete native action sequence for
// authentic generated calls. Its exact historical magic-only header is not a
// malformed-envelope acceptance policy: the separate necessary-guard test is
// deliberately expected to expose that omission on the current source.
func p15FEHealthyGraph(actions []map[string]any, kind string) (*p15FEHealthyCursor, error) {
	c := &p15FEHealthyCursor{
		p15SCCursor: p15SCCursor{actions: actions, groups: map[string]bool{}},
		positions: map[string]int{},
	}
	c.err = p15SCReferences(actions)
	input := map[string]any{"Type": "ExtensionInput"}
	if kind == "without_functions" {
		c.mark("observation")
		text := c.take("gettext", "Text", map[string]any{"WFTextActionText": p15FEHealthyText(input, input)}, true)
		c.mark("output"); c.output(text)
	} else {
		zero := c.take("gettext", "", map[string]any{"WFTextActionText": "0"}, true)
		c.store("_cherri_is_fn", zero)
		c.mark("snapshot"); c.store("_cherri_caller_input", input)
		provided := c.hasInput(input)
		possible := c.take("detect.dictionary", "", map[string]any{"WFInput": p15SCAttachment(input)}, true)
		c.field("cherri_functions", possible, "")
		magic := c.field("_cherri_magic", possible, "")
		valid := c.start(magic, 4, "WFConditionalActionString", "__cherri_fn_envelope_v2__")
		one := c.take("gettext", "", map[string]any{"WFTextActionText": "1"}, true)
		c.store("_cherri_is_fn", one)
		c.mark("restore_read"); callerValue := c.field("_cherri_caller_input", possible, "")
		c.mark("restore"); c.store("_cherri_caller_input", callerValue)
		c.end(valid, "", false); c.end(provided, "", false)
		outer := c.start(p15SCVariable("_cherri_is_fn"), 4, "WFConditionalActionString", "1")
		envelope := c.take("detect.dictionary", "", map[string]any{"WFInput": p15SCAttachment(input)}, true)
		function := c.field("function", envelope, "")
		c.field("arguments", envelope, "")
		body := c.start(function, 4, "WFConditionalActionString", "inspect")
		c.mark("argument"); value := c.field("arg_0", envelope, "value")
		caller := p15SCVariable("_cherri_caller_input")
		c.mark("body")
		text := c.take("gettext", "Text", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller, value)}, true)
		c.output(text)
		c.end(body, "", false)
		callee := "inspect"
		if kind == "nested" {
			body = c.start(function, 4, "WFConditionalActionString", "outer")
			argument := c.field("arg_0", envelope, "value")
			c.mark("nested_payload"); payload := c.payload("inspect", p15ContainerReference(argument))
			inner := c.call(payload, "inner")
			c.mark("nested_body")
			text = c.take("gettext", "Text", map[string]any{"WFTextActionText": p15FEHealthyText(caller, inner, caller)}, true)
			c.output(text); c.end(body, "", false)
			callee = "outer"
		}
		c.bare("exit"); c.alternate(outer)
		c.mark("before")
		before := c.take("gettext", "before", map[string]any{"WFTextActionText": p15FEHealthyText(caller, caller)}, true)
		c.mark("payload"); payload := c.payload(callee, "VALUE")
		c.mark("call"); result := c.call(payload, "result")
		c.mark("observation")
		text = c.take("gettext", "Text", map[string]any{"WFTextActionText": p15FEHealthyText(before, result, caller, caller)}, true)
		c.mark("output"); c.output(text)
		c.end(outer, "", false)
	}
	if c.err != nil { return c, c.err }
	if c.index != len(actions) { return c, fmt.Errorf("unmodeled healthy action: consumed %d of %d", c.index, len(actions)) }
	return c, nil
}

func p15FEHealthyFields(t *testing.T, actions []map[string]any, index int) map[string]map[string]any {
	t.Helper()
	fields, err := p15ContainerFields(p15SCParams(actions[index])["WFItems"])
	if err != nil { t.Fatal(err) }
	return fields
}
func p15FEReplaceAttachment(actions []map[string]any, index int, key, rangeKey string, value map[string]any) {
	inner := p15SCParams(actions[index])[key].(map[string]any)["Value"].(map[string]any)
	inner["attachmentsByRange"].(map[string]any)[rangeKey] = value
}
func p15FEReconcileName(value any, before, after string) {
	switch value := value.(type) {
	case map[string]any:
		for key, item := range value {
			if (key == "CustomOutputName" || key == "OutputName") && item == before {
				value[key] = after
			} else { p15FEReconcileName(item, before, after) }
		}
	case []any:
		for _, item := range value { p15FEReconcileName(item, before, after) }
	}
}

func TestP15FunctionEnvelopeHealthyNativeControls(t *testing.T) {
	for _, kind := range []string{"direct", "nested", "without_functions"} {
		t.Run(kind, func(t *testing.T) {
			actions := p15FECompile(t, p15FEHealthySource(t, kind))
			view, err := p15FEHealthyGraph(actions, kind)
			if err != nil { t.Fatal(err) }
			faults := []string{"input_alias_lost", "static_observation", "output_detached", "extra_action"}
			if kind != "without_functions" {
				faults = []string{"initial_snapshot", "restore_arguments", "restore_envelope", "body_reads_transport", "main_before_reads_transport", "payload_loses_caller", "caller_consumes_transport", "call_uses_wrong_input", "call_wrong_target", "arg_wrong_slot", "result_name_reconciled", "missing_action", "extra_action", "wrong_field_kind", "caller_output_static", "static_observation"}
				if kind == "nested" { faults = append(faults, "nested_payload_loses_caller", "nested_body_reads_transport") }
			}
			for _, fault := range faults {
				t.Run(fault, func(t *testing.T) {
					bad := p15SCClone(t, actions)
					transport := map[string]any{"Type": "ExtensionInput"}
					switch fault {
					case "initial_snapshot": p15SCParams(bad[view.positions["snapshot"]])["WFInput"] = p15SCAttachment(p15SCVariable("unrelated"))
					case "restore_arguments": p15SCParams(bad[view.positions["restore_read"]])["WFDictionaryKey"] = "arguments"
					case "restore_envelope": p15SCParams(bad[view.positions["restore"]])["WFInput"] = p15SCAttachment(transport)
					case "body_reads_transport": p15FEReplaceAttachment(bad, view.positions["body"], "WFTextActionText", "{0, 1}", transport)
					case "main_before_reads_transport": p15FEReplaceAttachment(bad, view.positions["before"], "WFTextActionText", "{0, 1}", transport)
					case "payload_loses_caller": p15FEHealthyFields(t, bad, view.positions["payload"])["_cherri_caller_input"]["WFValue"] = p15SCAttachment(transport)
					case "caller_consumes_transport": p15FEReplaceAttachment(bad, view.positions["observation"], "WFTextActionText", "{4, 1}", transport)
					case "call_uses_wrong_input": p15SCParams(bad[view.positions["call"]])["WFInput"] = p15SCAttachment(transport)
					case "call_wrong_target": p15SCParams(bad[view.positions["call"]])["WFWorkflow"].(map[string]any)["isSelf"] = false
					case "arg_wrong_slot": p15SCParams(bad[view.positions["argument"]])["WFDictionaryKey"] = "arg_1"
					case "result_name_reconciled":
						for _, action := range bad { p15FEReconcileName(action, "result", "unrelated") }
					case "missing_action": bad = bad[1:]
					case "extra_action": bad = append(bad, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"})
					case "wrong_field_kind": p15FEHealthyFields(t, bad, view.positions["payload"])["has_arg_0"]["WFItemType"] = float64(0)
					case "caller_output_static", "output_detached": p15SCParams(bad[view.positions["output"]])["WFOutput"] = p15ContainerText("static")
					case "static_observation": p15SCParams(bad[view.positions["observation"]])["WFTextActionText"] = p15ContainerText("static")
					case "input_alias_lost": p15FEReplaceAttachment(bad, view.positions["observation"], "WFTextActionText", "{2, 1}", p15SCVariable("unrelated"))
					case "nested_payload_loses_caller": p15FEHealthyFields(t, bad, view.positions["nested_payload"])["_cherri_caller_input"]["WFValue"] = p15SCAttachment(transport)
					case "nested_body_reads_transport": p15FEReplaceAttachment(bad, view.positions["nested_body"], "WFTextActionText", "{0, 1}", transport)
					default: t.Fatal("unknown fixed healthy corruption", fault)
					}
					if _, err := p15FEHealthyGraph(bad, kind); err == nil { t.Fatal("damaged healthy graph accepted", fault) }
				})
			}
			t.Logf("RP26_BASELINE_HEALTHY kind=%s actions=%d; actual native structure only, no Apple or complete-envelope credit", kind, len(actions))
		})
	}
}

