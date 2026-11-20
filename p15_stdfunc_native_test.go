package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

const p15StdfuncScript = "document.body.append(document.createTextNode('#probe'));"
const p15StdfuncPrompt = "Choose public item"

var p15StdfuncCards = []string{
	"BEGIN:VCARD\nVERSION:3.0\nFN:One\nEND:VCARD",
	"BEGIN:VCARD\nVERSION:3.0\nFN:Two\nEND:VCARD",
}

func p15StdfuncSource(alias string) string {
	return fmt.Sprintf("import stdfunc as %s\nlet js=%s.runJS(%q)\nlet cards=[%q,%q]\nlet chosen=%s.chooseFromVCard(cards,prompt:%q)\nshow(f\"{js}|{chosen}\")\n",
		alias, alias, p15StdfuncScript, p15StdfuncCards[0], p15StdfuncCards[1], alias, p15StdfuncPrompt)
}

// This executes the real generator, then inspects its Go constant independently.
// The original source and committed generated snapshot are never modified.
func TestP15RP28StandardLibraryGeneration(t *testing.T) {
	var source, err = os.ReadFile("stdfunc.cherri")
	if err != nil {
		t.Fatal(err)
	}
	var tracked, readErr = os.ReadFile("internal/language/modules/stdfunc_source_generated.go")
	if readErr != nil {
		t.Fatal(readErr)
	}
	var directory = t.TempDir()
	var binary = filepath.Join(directory, "language-stdlib")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	var build = exec.CommandContext(ctx, "go", "build", "-o", binary, "./tools/language-stdlib")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build actual generator: %v\n%s", err, output)
	}
	var run = func(wantSuccess bool, arguments ...string) {
		t.Helper()
		var output, err = exec.CommandContext(ctx, binary, arguments...).CombinedOutput()
		if (err == nil) != wantSuccess {
			t.Fatalf("generator success=%t want %t: %v\n%s", err == nil, wantSuccess, err, output)
		}
	}
	var generated = filepath.Join(directory, "generated.go")
	run(true, "-source", "stdfunc.cherri", "-output", generated)
	var actual, actualErr = os.ReadFile(generated)
	if actualErr != nil || !bytes.Equal(actual, tracked) {
		t.Fatalf("tracked generated snapshot is not actual current generator output: %v", actualErr)
	}
	run(true, "-source", "stdfunc.cherri", "-output", generated, "-check")
	run(true, "-source", "stdfunc.cherri", "-output", generated)
	var second, _ = os.ReadFile(generated)
	if !bytes.Equal(actual, second) {
		t.Fatal("unchanged generator input is nondeterministic")
	}
	var parsed, parseErr = goparser.ParseFile(gotoken.NewFileSet(), generated, actual, goparser.ParseComments)
	if parseErr != nil {
		t.Fatal(parseErr)
	}
	var decoded string
	var constants int
	ast.Inspect(parsed, func(node ast.Node) bool {
		var value, ok = node.(*ast.ValueSpec)
		if !ok || len(value.Names) != 1 || value.Names[0].Name != "standardLibrarySource" || len(value.Values) != 1 {
			return true
		}
		var literal, literalOK = value.Values[0].(*ast.BasicLit)
		if !literalOK || literal.Kind != gotoken.STRING {
			t.Fatal("generated library is not the preserved literal")
		}
		decoded, err = strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		constants++
		return true
	})
	if constants != 1 || decoded != string(source) || modules.StandardLibrarySource() != decoded ||
		!bytes.Contains(actual, []byte(fmt.Sprintf("SHA256: %x.", sha256.Sum256(source)))) {
		t.Fatal("generator, source hash and actual imported library bytes are not bound")
	}

	var changedSource = filepath.Join(directory, "changed.cherri")
	var changed = bytes.Replace(source, []byte("cherri_chooseFromVCard.vcf"), []byte("changed_public_probe.vcf"), 1)
	if bytes.Equal(source, changed) {
		t.Fatal("source mutation target missing")
	}
	if err := os.WriteFile(changedSource, changed, 0600); err != nil {
		t.Fatal(err)
	}
	run(false, "-source", changedSource, "-output", generated, "-check")
	var afterRejected, _ = os.ReadFile(generated)
	if !bytes.Equal(actual, afterRejected) {
		t.Fatal("check mode overwrote a stale snapshot")
	}
	var regenerated = filepath.Join(directory, "changed.go")
	run(true, "-source", changedSource, "-output", regenerated)
	var changedOutput, _ = os.ReadFile(regenerated)
	if bytes.Equal(actual, changedOutput) || !bytes.Contains(changedOutput, []byte(fmt.Sprintf("SHA256: %x.", sha256.Sum256(changed)))) ||
		!bytes.Contains(changedOutput, []byte("changed_public_probe.vcf")) {
		t.Fatal("real generator ignored a behavior-bearing source change")
	}
	for _, invalid := range [][]byte{nil, {0xff, 0xfe}} {
		if err := os.WriteFile(changedSource, invalid, 0600); err != nil {
			t.Fatal(err)
		}
		run(false, "-source", changedSource, "-output", generated)
		var afterInvalid, _ = os.ReadFile(generated)
		if !bytes.Equal(actual, afterInvalid) {
			t.Fatal("invalid source changed an existing generated destination")
		}
	}
}

type p15StdfuncEdge struct {
	uuid, coercion string
}

func p15StdfuncDescriptorError(value any, want p15StdfuncEdge) error {
	var descriptor, ok = value.(map[string]any)
	if !ok || descriptor["Type"] != "ActionOutput" || descriptor["OutputUUID"] != want.uuid || want.uuid == "" {
		return fmt.Errorf("wrong or missing native producer edge")
	}
	for key := range descriptor {
		if key != "Type" && key != "OutputUUID" && key != "OutputName" && key != "Aggrandizements" {
			return fmt.Errorf("unmodeled reference field %s", key)
		}
	}
	var aggrandizements, present = descriptor["Aggrandizements"]
	if want.coercion == "" {
		if present {
			return fmt.Errorf("unexpected reference transformation")
		}
	} else if !reflect.DeepEqual(aggrandizements, []any{map[string]any{
		"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": want.coercion,
	}}) {
		return fmt.Errorf("missing or changed native coercion %s", want.coercion)
	}
	return nil
}

func p15StdfuncAttachmentError(value any, want p15StdfuncEdge) error {
	var attachment, ok = value.(map[string]any)
	if !ok || len(attachment) != 2 || attachment["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("missing native attachment envelope")
	}
	return p15StdfuncDescriptorError(attachment["Value"], want)
}

func p15StdfuncTextError(value any, text string, edges map[string]p15StdfuncEdge) error {
	var actual, attachments, ok = p15BindingTokenText(value)
	if !ok || actual != text || len(attachments) != len(edges) {
		return fmt.Errorf("native text literal/range/cardinality changed")
	}
	for key, edge := range edges {
		if err := p15StdfuncDescriptorError(attachments[key], edge); err != nil {
			return fmt.Errorf("text range %s: %w", key, err)
		}
	}
	return nil
}

func p15StdfuncFields(value any) (map[string]any, error) {
	var outer, _ = value.(map[string]any)
	var inner, _ = outer["Value"].(map[string]any)
	var rows, ok = inner["WFDictionaryFieldValueItems"].([]any)
	if !ok || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("missing native function payload")
	}
	var result = map[string]any{}
	for _, raw := range rows {
		var row, _ = raw.(map[string]any)
		var name, err = p15LiteralText(row["WFKey"])
		if err != nil {
			return nil, err
		}
		if _, present := result[name]; present {
			return nil, fmt.Errorf("duplicate native payload field")
		}
		result[name] = row["WFValue"]
	}
	return result, nil
}

// The oracle reads the raw serialized actions, not parity-normalized output.
// Its expected operations and edges follow the preserved stdfunc source.
func p15StdfuncNativeError(actions []map[string]any) error {
	var positions = map[string]int{}
	var named = map[string][]map[string]any{}
	var functionGetter, envelope string
	for index, action := range actions {
		var p = p15C01Params(action)
		if uuid, ok := p["UUID"].(string); ok && uuid != "" {
			if _, duplicate := positions[uuid]; duplicate {
				return fmt.Errorf("duplicate native producer UUID")
			}
			positions[uuid] = index
		}
		if name, ok := p["CustomOutputName"].(string); ok {
			named[name] = append(named[name], action)
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && p["WFDictionaryKey"] == "function" {
			if functionGetter != "" {
				return fmt.Errorf("duplicate dispatcher getter")
			}
			functionGetter, _ = p["UUID"].(string)
			var input, _ = p["WFInput"].(map[string]any)
			var value, _ = input["Value"].(map[string]any)
			envelope, _ = value["OutputUUID"].(string)
		}
	}
	if functionGetter == "" || envelope == "" {
		return fmt.Errorf("missing actual function dispatcher")
	}
	var envelopeIndex, envelopeFound = positions[envelope]
	if !envelopeFound || envelopeIndex >= positions[functionGetter] ||
		actions[envelopeIndex]["WFWorkflowActionIdentifier"] != "is.workflow.actions.detect.dictionary" {
		return fmt.Errorf("dispatcher lost the preceding native input dictionary")
	}
	var input, _ = p15C01Params(actions[envelopeIndex])["WFInput"].(map[string]any)
	if !reflect.DeepEqual(input, map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ExtensionInput"}}) {
		return fmt.Errorf("dispatcher source is not actual Shortcut input")
	}
	var bodies = map[string][]map[string]any{}
	for index, action := range actions {
		var p = p15C01Params(action)
		var name, _ = p["WFConditionalActionString"].(string)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" || !strings.HasPrefix(name, "module:builtin:stdfunc::") {
			continue
		}
		if p15Int(p["WFControlFlowMode"]) != 0 || p15Int(p["WFCondition"]) != 4 || bodies[name] != nil {
			return fmt.Errorf("wrong or duplicate stdfunc dispatch")
		}
		var input, _ = p["WFInput"].(map[string]any)
		if err := p15StdfuncAttachmentError(input["Variable"], p15StdfuncEdge{uuid: functionGetter}); err != nil {
			return err
		}
		var end = index + 1
		for ; end < len(actions); end++ {
			var q = p15C01Params(actions[end])
			if actions[end]["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && q["GroupingIdentifier"] == p["GroupingIdentifier"] && p15Int(q["WFControlFlowMode"]) == 2 {
				break
			}
		}
		if end == len(actions) {
			return fmt.Errorf("unclosed stdfunc body")
		}
		bodies[name] = actions[index+1 : end]
	}
	var sequences = map[string][]string{
		"runJS":           {"getvalueforkey", "text.replace", "url", "urlencode", "gettext", "output"},
		"chooseFromVCard": {"getvalueforkey", "getvalueforkey", "gettext", "setitemname", "choosefromlist", "gettext", "output"},
	}
	var allowed = map[string][]string{
		"getvalueforkey": {"UUID", "CustomOutputName", "WFDictionaryKey", "WFGetDictionaryValueType", "WFInput"},
		"text.replace":   {"UUID", "CustomOutputName", "WFInput", "WFReplaceTextFind", "WFReplaceTextReplace"},
		"url":            {"UUID", "CustomOutputName", "WFURLActionURL", "Show-WFURLActionURL"},
		"urlencode":      {"UUID", "CustomOutputName", "WFInput", "WFEncodeMode"},
		"gettext":        {"UUID", "CustomOutputName", "WFTextActionText"},
		"output":         {"WFOutput"},
		"setitemname":    {"UUID", "CustomOutputName", "WFInput", "WFName"},
		"choosefromlist": {"UUID", "CustomOutputName", "WFInput", "WFChooseFromListActionPrompt"},
	}
	if len(bodies) != len(sequences) {
		return fmt.Errorf("stdlib declaration set changed")
	}
	for name, sequence := range sequences {
		var body = bodies["module:builtin:stdfunc::"+name]
		if len(body) != len(sequence) {
			return fmt.Errorf("%s native body changed cardinality", name)
		}
		for index, kind := range sequence {
			var action = body[index]
			var p = p15C01Params(action)
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions."+kind || len(p) != len(allowed[kind]) {
				return fmt.Errorf("%s body operation/parameter set changed at %d", name, index)
			}
			for _, key := range allowed[kind] {
				if _, ok := p[key]; !ok {
					return fmt.Errorf("%s missing native field %s", name, key)
				}
			}
			if kind == "getvalueforkey" {
				if p["WFDictionaryKey"] != fmt.Sprintf("arg_%d", index) || p["WFGetDictionaryValueType"] != "Value" {
					return fmt.Errorf("%s wrong argument slot", name)
				}
				if err := p15StdfuncAttachmentError(p["WFInput"], p15StdfuncEdge{uuid: envelope}); err != nil {
					return err
				}
			}
		}
	}
	var js = bodies["module:builtin:stdfunc::runJS"]
	var card = bodies["module:builtin:stdfunc::chooseFromVCard"]
	var id = func(action map[string]any) string { var uuid, _ = p15C01Params(action)["UUID"].(string); return uuid }
	var one = func(uuid, coercion string) map[string]p15StdfuncEdge {
		return map[string]p15StdfuncEdge{"{0, 1}": {uuid: uuid, coercion: coercion}}
	}
	var replace = p15C01Params(js[1])
	// The preserved Legacy source contains "\u0023". Its lexer decodes only
	// newline, carriage-return and tab escapes; this literal is therefore "u0023".
	if replace["WFReplaceTextFind"] != "#" || replace["WFReplaceTextReplace"] != "u0023" {
		return fmt.Errorf("preserved JavaScript replacement changed")
	}
	if err := p15StdfuncTextError(replace["WFInput"], "\ufffc", one(id(js[0]), "")); err != nil {
		return err
	}
	var url = p15C01Params(js[2])
	var values, ok = url["WFURLActionURL"].([]any)
	if !ok || len(values) != 1 || url["Show-WFURLActionURL"] != true {
		return fmt.Errorf("JavaScript URL carrier changed")
	}
	if err := p15StdfuncTextError(values[0], "data:text/html;charset=utf-8,<body/><script>\ufffc</script>",
		map[string]p15StdfuncEdge{"{44, 1}": {uuid: id(js[1])}}); err != nil {
		return err
	}
	if p15C01Params(js[3])["WFEncodeMode"] != "Decode" {
		return fmt.Errorf("JavaScript URL decoding changed")
	}
	for _, check := range []struct {
		value any
		text  string
		edges map[string]p15StdfuncEdge
	}{
		{p15C01Params(js[3])["WFInput"], "\ufffc", one(id(js[2]), "WFRichTextContentItem")},
		{p15C01Params(js[4])["WFTextActionText"], "\ufffc", one(id(js[3]), "")},
		{p15C01Params(js[5])["WFOutput"], "\ufffc", one(id(js[4]), "")},
		{p15C01Params(card[2])["WFTextActionText"], "\ufffc", one(id(card[0]), "")},
		{p15C01Params(card[4])["WFChooseFromListActionPrompt"], "\ufffc", one(id(card[1]), "")},
		{p15C01Params(card[5])["WFTextActionText"], "\ufffc", one(id(card[4]), "")},
		{p15C01Params(card[6])["WFOutput"], "\ufffc", one(id(card[5]), "")},
	} {
		if err := p15StdfuncTextError(check.value, check.text, check.edges); err != nil {
			return err
		}
	}
	if p15C01Params(card[3])["WFName"] != "cherri_chooseFromVCard.vcf" {
		return fmt.Errorf("vCard filename changed")
	}
	if err := p15StdfuncAttachmentError(p15C01Params(card[3])["WFInput"], p15StdfuncEdge{uuid: id(card[2])}); err != nil {
		return err
	}
	if err := p15StdfuncAttachmentError(p15C01Params(card[4])["WFInput"], p15StdfuncEdge{uuid: id(card[3]), coercion: "WFContactContentItem"}); err != nil {
		return err
	}

	for _, name := range []string{"js", "cards", "chosen"} {
		if len(named[name]) != 1 || id(named[name][0]) == "" {
			return fmt.Errorf("missing/duplicate named caller value %s", name)
		}
	}
	var cards = named["cards"][0]
	var items, itemsOK = p15C01Params(cards)["WFItems"].([]any)
	if cards["WFWorkflowActionIdentifier"] != "is.workflow.actions.list" || !itemsOK || len(items) != len(p15StdfuncCards) {
		return fmt.Errorf("caller vCard list shape changed")
	}
	for index, raw := range items {
		var row, _ = raw.(map[string]any)
		if p15Int(row["WFItemType"]) != 0 || row["WFValue"] != p15StdfuncCards[index] {
			return fmt.Errorf("caller vCard list content/order changed")
		}
	}
	for _, call := range []struct {
		variable, function string
		arguments          []any
	}{
		{"js", "runJS", []any{p15StdfuncScript}},
		{"chosen", "chooseFromVCard", []any{p15StdfuncEdge{uuid: id(cards)}, p15StdfuncPrompt}},
	} {
		var action = named[call.variable][0]
		var p = p15C01Params(action)
		var workflow, _ = p["WFWorkflow"].(map[string]any)
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.runworkflow" || workflow["isSelf"] != true {
			return fmt.Errorf("caller no longer invokes its actual generated function")
		}
		var input, _ = p["WFInput"].(map[string]any)
		var descriptor, _ = input["Value"].(map[string]any)
		var payloadID, _ = descriptor["OutputUUID"].(string)
		var at, found = positions[payloadID]
		if !found || at >= positions[id(action)] || actions[at]["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			return fmt.Errorf("call lost preceding payload")
		}
		if err := p15StdfuncAttachmentError(p["WFInput"], p15StdfuncEdge{uuid: payloadID}); err != nil {
			return err
		}
		var fields, err = p15StdfuncFields(p15C01Params(actions[at])["WFItems"])
		if err != nil {
			return err
		}
		for field, want := range map[string]string{"function": "module:builtin:stdfunc::" + call.function, "_cherri_magic": "__cherri_fn_envelope_v2__", "cherri_functions": "1"} {
			if err := p15CheckLiteralText(fields[field], want); err != nil {
				return err
			}
		}
		var array, _ = fields["arguments"].(map[string]any)
		var rows, rowsOK = array["Value"].([]any)
		if array["WFSerializationType"] != "WFArrayParameterState" || !rowsOK || len(rows) != len(call.arguments) {
			return fmt.Errorf("call argument cardinality changed")
		}
		for index, expected := range call.arguments {
			if err := p15CheckLiteralText(fields[fmt.Sprintf("has_arg_%d", index)], "1"); err != nil {
				return err
			}
			var row, _ = rows[index].(map[string]any)
			for _, value := range []any{fields[fmt.Sprintf("arg_%d", index)], row["WFValue"]} {
				if edge, ok := expected.(p15StdfuncEdge); ok {
					if err := p15StdfuncAttachmentError(value, edge); err != nil {
						return err
					}
				} else if err := p15CheckLiteralText(value, expected.(string)); err != nil {
					return err
				}
			}
		}
	}
	var showCount int
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.showresult" {
			showCount++
			if err := p15StdfuncTextError(p15C01Params(action)["Text"], "\ufffc|\ufffc",
				map[string]p15StdfuncEdge{"{0, 1}": {uuid: id(named["js"][0])}, "{2, 1}": {uuid: id(named["chosen"][0])}}); err != nil {
				return err
			}
		}
	}
	if showCount != 1 {
		return fmt.Errorf("caller dropped or duplicated returned values")
	}
	return nil
}

func TestP15IM03StandardFunctionNativeBehavior(t *testing.T) {
	for _, alias := range []string{"std", "Std"} {
		t.Run(alias, func(t *testing.T) {
			var actions = p15CompiledActions(t, p15StdfuncSource(alias))
			if err := p15StdfuncNativeError(actions); err != nil {
				t.Fatal(err)
			}
			for _, mutation := range []string{"replacement", "replacement_unicode_reinterpretation", "html", "decode", "richtext", "filename", "contact", "prompt_edge", "parameter_slot", "call_target", "call_value", "return_edge", "drop_operation", "duplicate_uuid", "unknown_parameter"} {
				t.Run(mutation, func(t *testing.T) {
					var broken = p15C01Clone(t, actions)
					var changed bool
					for index, action := range broken {
						var p = p15C01Params(action)
						var name, _ = p["CustomOutputName"].(string)
						switch {
						case mutation == "replacement" && name == "_ios17bug":
							p["WFReplaceTextReplace"] = "changed"
							changed = true
						case mutation == "replacement_unicode_reinterpretation" && name == "_ios17bug":
							p["WFReplaceTextReplace"] = "#"
							changed = true
						case mutation == "html" && name == "_uri":
							p["WFURLActionURL"] = []any{"https://example.invalid/changed"}
							changed = true
						case mutation == "decode" && name == "_decoded":
							p["WFEncodeMode"] = "Encode"
							changed = true
						case mutation == "richtext" && name == "_decoded":
							var _, refs, _ = p15BindingTokenText(p["WFInput"])
							delete(refs["{0, 1}"].(map[string]any), "Aggrandizements")
							changed = true
						case mutation == "filename" && name == "_vcf":
							p["WFName"] = "wrong.txt"
							changed = true
						case mutation == "contact" && name == "_chosenItem":
							delete(p["WFInput"].(map[string]any)["Value"].(map[string]any), "Aggrandizements")
							changed = true
						case mutation == "prompt_edge" && name == "_chosenItem":
							var _, refs, _ = p15BindingTokenText(p["WFChooseFromListActionPrompt"])
							refs["{0, 1}"].(map[string]any)["OutputUUID"] = "wrong-prompt"
							changed = true
						case mutation == "parameter_slot" && name == "_prompt":
							p["WFDictionaryKey"] = "arg_0"
							changed = true
						case (mutation == "call_target" || mutation == "call_value") && name == "FunctionPayload":
							var fields = p15LiteralDictionaryFields(t, p["WFItems"])
							if mutation == "call_target" {
								fields["function"]["WFValue"] = "module:file:wrong::runJS"
							} else {
								fields["arg_0"]["WFValue"] = "hardcoded wrong argument"
							}
							changed = true
						case mutation == "return_edge" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.output":
							p["WFOutput"] = "hardcoded result"
							changed = true
						case mutation == "drop_operation" && name == "_decoded":
							broken = append(broken[:index], broken[index+1:]...)
							changed = true
						case mutation == "duplicate_uuid" && name == "_uri":
							p["UUID"] = p15C01Params(broken[index-1])["UUID"]
							changed = true
						case mutation == "unknown_parameter" && name == "_ios17bug":
							p["WFReplaceTextRegularExpression"] = true
							changed = true
						}
						if changed {
							break
						}
					}
					if !changed {
						t.Fatal("actual emitted mutation target missing")
					}
					if p15StdfuncNativeError(broken) == nil {
						t.Fatalf("native stdfunc corruption %s was accepted", mutation)
					}
				})
			}
		})
	}
}

func TestP15RP28StandardFunctionLegacyDifferential(t *testing.T) {
	var legacySource = fmt.Sprintf("#include 'actions/text'\n#include 'actions/web'\n#include 'stdfunc'\nconst js = runJS(%q)\n@cards = [%q,%q]\nconst chosen = chooseFromVCard(@cards, %q)\nshow(\"{js}|{chosen}\")\n",
		p15StdfuncScript, p15StdfuncCards[0], p15StdfuncCards[1], p15StdfuncPrompt)
	var legacy, err = compileLegacySourceToRawMapInMemory(legacySource)
	if err != nil {
		t.Fatal(err)
	}
	for _, dependency := range []string{"actions/text", "actions/web"} {
		t.Run("missing_"+strings.TrimPrefix(dependency, "actions/"), func(t *testing.T) {
			var missing = strings.Replace(legacySource, "#include '"+dependency+"'\n", "", 1)
			var output, err = compileLegacySourceToRawMapInMemory(missing)
			if err == nil || output != nil || !strings.Contains(err.Error(), dependency) {
				t.Fatalf("missing actual stdfunc dependency was silently accepted: %v", err)
			}
		})
	}
	var path, text = p15ModuleProject(t, map[string]string{"main.cherri": p15StdfuncSource("std")})
	var candidate = p15ModuleCanonicalDocument(t, path, text)
	// The embedded legacy API omits default workflow headers. Compare its
	// entire emitted action graph to the V2 graph at the same API boundary.
	// Raw V2 parameter/coercion edges are independently checked above.
	var legacyActions, legacyOK = legacy["WFWorkflowActions"].([]any)
	var candidateActions, candidateOK = candidate["WFWorkflowActions"].([]any)
	if !legacyOK || !candidateOK || len(legacyActions) == 0 || len(candidateActions) == 0 {
		t.Fatal("actual compiler returned no native action graph")
	}
	// Assert the independently observed Legacy literal before comparing graphs,
	// so equal but reinterpreted outputs cannot satisfy this differential.
	for _, compiled := range []struct {
		name    string
		actions []any
	}{{"legacy_literal_replacement", legacyActions}, {"v2_literal_replacement", candidateActions}} {
		t.Run(compiled.name, func(t *testing.T) {
			var replacements int
			for _, raw := range compiled.actions {
				var action, _ = raw.(map[string]any)
				if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.text.replace" {
					continue
				}
				replacements++
				var params, _ = action["WFWorkflowActionParameters"].(map[string]any)
				if params["WFReplaceTextFind"] != "#" || params["WFReplaceTextReplace"] != "u0023" {
					t.Fatalf("preserved Legacy literal changed: find=%#v replace=%#v",
						params["WFReplaceTextFind"], params["WFReplaceTextReplace"])
				}
			}
			if replacements != 1 {
				t.Fatalf("want one actual replacement operation, got %d", replacements)
			}
		})
	}
	if difference := shortcutcompare.CompareDocumentsNormalized(
		map[string]any{"WFWorkflowActions": legacyActions},
		map[string]any{"WFWorkflowActions": candidateActions}); !difference.Equal {
		t.Fatalf("preserved stdfunc differential changed native action semantics: %+v", difference.Differences)
	}
}
