package main

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

const p15BRG11Source = `function forms(flag:Bool=true,count:Number=9,label:Text="DEFAULT",items:List<Text> = ["DEFAULT"],mapping:Map<Text,Number> = {"DEFAULT":9})->Map<Text,AnyContent>{
return {"flag":flag,"count":count,"label":label,"items":items,"mapping":mapping}
}
let omitted=forms()
let explicit=forms(false,count:0,label:"",items:[],mapping:{})
let named=forms(mapping:{},items:[],label:"",count:0,flag:false)
setBackgroundSound(sound:"Oc"+"ean")
setBackgroundSound(sound:.Rain)
let token=getText(input:"")
let source=text("TOKEN")
let reference=getText(source)
let converted=number("-5.125")
let rounded=round(converted,roundTo:.Integer)
let dynamic=text("DYNAMIC")
let packed=list(listItem:["FIRST",dynamic,"LAST"])
return {"omitted":omitted,"explicit":explicit,"named":named,"token":token,"reference":reference,"converted":converted,"rounded":rounded,"packed":packed}
`

func TestP15BRG11CombinedParameterFormsUnit(t *testing.T) {
	var registry = schema.DefaultRegistry()
	var literal, exists = registry.LookupAction("setBackgroundSound")
	if !exists {
		t.Fatal("canonical literal-only enum action missing")
	}
	var parameter, found = literal.PrimaryParameter()
	if !found || parameter.EvidenceStatus != schema.EvidenceConfirmed || !reflect.DeepEqual(parameter.AcceptedForms, []string{"literal"}) {
		t.Fatal("confirmed literal-only canonical binding changed")
	}
	var reference, present = registry.LookupAction("getText")
	if !present || reference.AppleIdentifier != "is.workflow.actions.detect.text" {
		t.Fatal("canonical reference codec action changed")
	}
	parameter, found = reference.PrimaryParameter()
	if !found || parameter.Codec != "token" || parameter.WireKey != "WFInput" {
		t.Fatal("reference wire must use the canonical token codec")
	}
	p15FunctionTypedProgram(t, p15BRG11Source, map[string]string{
		"forms()": "Map<Text, AnyContent>", "forms(false,count:0,label:\"\",items:[],mapping:{})": "Map<Text, AnyContent>",
		"forms(mapping:{},items:[],label:\"\",count:0,flag:false)": "Map<Text, AnyContent>",
		"flag": "Bool", "count": "Number", "label": "Text", "items": "List<Text>", "mapping": "Map<Text, Number>",
		"getText(input:\"\")": "Text", "getText(source)": "Text", "number(\"-5.125\")": "Number",
		"round(converted,roundTo:.Integer)": "Number", "list(listItem:[\"FIRST\",dynamic,\"LAST\"])": "List<Text>",
	})
	for _, tc := range []struct{ name, source, target, code string }{
		{"literal_binding", "let mode=\"Ocean\"\nsetBackgroundSound(mode)", "mode", analysis.CodeLiteralRequired},
		{"literal_runtime", "setBackgroundSound(sound:text(\"Ocean\"))", "text(\"Ocean\")", analysis.CodeLiteralRequired},
		{"enum_invalid", "setBackgroundSound(\"Not\"+\"ASound\")", "\"Not\"+\"ASound\"", analysis.CodeEnumMember},
		{"numeric_text_without_conversion", "wait(text(\"42\"))", "text(\"42\")", analysis.CodeArgumentType},
		{"native_extra", "number(1,2)", "2", analysis.CodePositionalArg},
		{"native_extra_after_named", "number(number:1,2)", "2", analysis.CodePositionalArg},
		{"duplicate_primary", "number(1,number:2)", "number:2", analysis.CodeDuplicateArgument},
		{"unknown_label", "number(number:1,unknown:2)", "unknown:2", analysis.CodeUnknownArgument},
		{"missing_required", "number()", "number()", analysis.CodeMissingArgument},
		{"function_reserved_slot", "function choose(a:Number,b:Number=9)->Number{return a}\nchoose(1,2,b:3)", "2", analysis.CodePositionalArg},
		// Named-pack binding reports the first ambiguous explicit pack.
		{"duplicate_variadic_pack", "list(listItem:[],listItem:[\"x\"])", "listItem:[]", analysis.CodeDuplicateArgument},
	} {
		t.Run(tc.name, func(t *testing.T) { p15FunctionRejects(t, tc.source, tc.target, tc.code, "") })
	}
	for _, tc := range []struct{ name, typ, invalid string }{
		{"bool", "Bool", `"false"`}, {"number", "Number", `"0"`}, {"text", "Text", "false"},
		{"list", "List<Text>", "[1]"}, {"map", "Map<Text,Number>", `{"key":false}`},
	} {
		t.Run("wrong_default/"+tc.name, func(t *testing.T) {
			var source = fmt.Sprintf("function choose(value:%s = %s)->%s{return value}\nreturn choose()", tc.typ, tc.invalid, tc.typ)
			p15FunctionRejects(t, source, tc.invalid, analysis.CodeArgumentType, "default value of type")
		})
	}
}

var p15BRG11ParameterNames = []string{"flag", "count", "label", "items", "mapping"}

func p15BRG11ExternalAction(c *p15FTCursor, sound string) {
	if c.err != nil {
		return
	}
	var expected = map[string]any{"WFWorkflowActionIdentifier": "com.apple.UniversalAccess.UASettingsShortcuts.UASetBackgroundSoundIntent", "WFWorkflowActionParameters": map[string]any{"backgroundSound": sound}}
	if c.index >= len(c.actions) || !reflect.DeepEqual(c.actions[c.index], expected) {
		c.err = fmt.Errorf("canonical literal/enum action differs at %d", c.index)
		return
	}
	c.index++
}

func p15BRG11Graph(actions []map[string]any) (*p15FTCursor, error) {
	var c = p15FTNew(actions)
	c.header()
	c.beginFunction("forms")
	var values = map[string]any{}
	for index, name := range p15BRG11ParameterNames {
		c.mark(name + "_supplied")
		var supplied = c.field(fmt.Sprintf("has_arg_%d", index), c.envelope, "")
		c.mark(name + "_guard")
		var branch = c.condition(supplied, 4, "WFConditionalActionString", "1")
		c.mark(name + "_slot")
		c.field(fmt.Sprintf("arg_%d", index), c.envelope, "")
		c.alternate(branch)
		c.mark(name + "_fallback")
		p15RP25Default(c, p15RP25Cases()[index])
		c.mark(name + "_join")
		var value = c.end(branch, name, true)
		if index < 2 {
			value = c.numberCast(value, name)
		}
		values[name] = p15ContainerReference(value)
	}
	c.mark("child_result")
	var returned = c.dictionary("Dictionary", values)
	c.output(returned)
	c.caller()
	var observed = map[string]any{}
	for _, name := range []string{"omitted", "explicit", "named"} {
		var args = make([]p15FTArgument, 5)
		if name != "omitted" {
			for index, tc := range p15RP25Cases()[:5] {
				args[index] = p15FTArgument{true, tc.value}
			}
		}
		c.mark(name + "_payload")
		var payload = c.payload("forms", args...)
		c.mark(name + "_call")
		observed[name] = p15ContainerReference(c.call(payload, name))
	}
	c.mark("literal")
	p15BRG11ExternalAction(c, "Ocean")
	c.mark("enum")
	p15BRG11ExternalAction(c, "Rain")
	c.mark("token_literal")
	var empty = c.take("gettext", "Text", map[string]any{"WFTextActionText": ""}, true)
	c.mark("token_consumer")
	observed["token"] = p15ContainerReference(c.take("detect.text", "token", map[string]any{"WFInput": p15SCAttachment(empty)}, true))
	var source = c.take("gettext", "source", map[string]any{"WFTextActionText": "TOKEN"}, true)
	c.mark("reference_consumer")
	observed["reference"] = p15ContainerReference(c.take("detect.text", "reference", map[string]any{"WFInput": p15SCAttachment(source)}, true))
	c.mark("numeric_text")
	var converted = c.take("number", "converted", map[string]any{"WFNumberActionNumber": "-5.125"}, true)
	observed["converted"] = p15ContainerReference(converted)
	c.mark("numeric_consumer")
	observed["rounded"] = p15ContainerReference(c.take("round", "rounded", map[string]any{"WFInput": p15SCAttachment(converted), "WFRoundTo": "Integer", "WFRoundMode": "Normal"}, true))
	var dynamic = c.take("gettext", "dynamic", map[string]any{"WFTextActionText": "DYNAMIC"}, true)
	c.mark("variadic")
	observed["packed"] = p15ContainerReference(c.take("list", "packed", map[string]any{"WFItems": []any{
		map[string]any{"WFItemType": uint64(0), "WFValue": "FIRST"},
		map[string]any{"WFItemType": uint64(0), "WFValue": p15SCRefToken(dynamic)},
		map[string]any{"WFItemType": uint64(0), "WFValue": "LAST"},
	}}, true))
	c.mark("observation")
	var result = c.dictionary("Dictionary", observed)
	c.mark("final_return")
	c.output(result)
	return c, c.finish()
}

func TestP15BRG11CombinedParameterFormsNative(t *testing.T) {
	var actions = p15FTCompile(t, p15BRG11Source)
	var view, err = p15BRG11Graph(actions)
	if err != nil {
		t.Fatal(err)
	}
	var verify = func(a []map[string]any) error { _, err := p15BRG11Graph(a); return err }
	p15FTCommonControls(t, actions, verify)
	for index, name := range p15BRG11ParameterNames {
		for _, fault := range []string{"slot", "presence_slot", "truthiness", "omitted_present", "explicit_absent", "named_absent", "fallback_kind", "join"} {
			t.Run("defaults/"+name+"/"+fault, func(t *testing.T) {
				var changed = p15SCClone(t, actions)
				switch fault {
				case "slot":
					p15SCParams(changed[view.points[name+"_slot"]])["WFDictionaryKey"] = fmt.Sprintf("arg_%d", (index+1)%5)
				case "presence_slot":
					p15SCParams(changed[view.points[name+"_supplied"]])["WFDictionaryKey"] = fmt.Sprintf("arg_%d", index)
				case "truthiness":
					p15SCParams(changed[view.points[name+"_guard"]])["WFCondition"] = uint64(100)
				case "omitted_present", "explicit_absent", "named_absent":
					var call, flag = "omitted", "1"
					if fault == "explicit_absent" {
						call, flag = "explicit", "0"
					} else if fault == "named_absent" {
						call, flag = "named", "0"
					}
					p15FN05Fields(t, changed, view.points[call+"_payload"])[fmt.Sprintf("has_arg_%d", index)]["WFValue"] = p15ContainerText(flag)
				case "fallback_kind":
					changed[view.points[name+"_fallback"]]["WFWorkflowActionIdentifier"] = "is.workflow.actions.exit"
				case "join":
					p15SCParams(changed[view.points[name+"_join"]])["GroupingIdentifier"] = p15SCParams(changed[3])["GroupingIdentifier"]
				}
				if verify(changed) == nil {
					t.Fatal("joined typed-default corruption accepted", name, fault)
				}
			})
		}
	}
	for _, fault := range []string{"literal_value", "enum_value", "literal_as_reference", "token_as_literal", "reference_producer", "number_as_real", "numeric_text_changed", "numeric_consumer", "variadic_cardinality", "variadic_order", "variadic_reference", "variadic_native_kind", "structured_result", "final_return"} {
		t.Run("forms/"+fault, func(t *testing.T) {
			var changed = p15SCClone(t, actions)
			var parameters = func(point string) map[string]any { return p15SCParams(changed[view.points[point]]) }
			switch fault {
			case "literal_value":
				parameters("literal")["backgroundSound"] = "Rain"
			case "enum_value":
				parameters("enum")["backgroundSound"] = "NotASound"
			case "literal_as_reference":
				parameters("literal")["backgroundSound"] = p15SCAttachment(p15SCVariable("dynamic"))
			case "token_as_literal":
				parameters("token_consumer")["WFInput"] = ""
			case "reference_producer":
				parameters("reference_consumer")["WFInput"] = p15SCAttachment(map[string]any{"Type": "ActionOutput", "OutputUUID": parameters("token_literal")["UUID"], "OutputName": "Text"})
			case "number_as_real":
				parameters("numeric_text")["WFNumberActionNumber"] = float64(-5.125)
			case "numeric_text_changed":
				parameters("numeric_text")["WFNumberActionNumber"] = "-5.1250"
			case "numeric_consumer":
				parameters("numeric_consumer")["WFInput"] = "-5.125"
			case "variadic_cardinality":
				parameters("variadic")["WFItems"] = parameters("variadic")["WFItems"].([]any)[:2]
			case "variadic_order":
				var rows = parameters("variadic")["WFItems"].([]any)
				rows[0], rows[2] = rows[2], rows[0]
			case "variadic_reference":
				parameters("variadic")["WFItems"].([]any)[1].(map[string]any)["WFValue"] = "DYNAMIC"
			case "variadic_native_kind":
				parameters("variadic")["WFItems"].([]any)[0].(map[string]any)["WFItemType"] = float64(0)
			case "structured_result":
				p15FN05Fields(t, changed, view.points["child_result"])["items"]["WFValue"] = p15ContainerText("DEFAULT")
			case "final_return":
				parameters("final_return")["WFOutput"] = p15ContainerText("PRECOMPUTED")
			}
			if verify(changed) == nil {
				t.Fatal("joined parameter-form corruption accepted", fault)
			}
		})
	}
}
