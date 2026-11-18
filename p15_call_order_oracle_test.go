package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/service"
	"howett.net/plist"
)

func p15C01DefaultGuards(actions []map[string]any) (map[string]int, error) {
	type frame struct {
		group, key string
		branch     int
	}
	getters := map[string]string{}
	guards := map[string]int{}
	seen := map[string]int{}
	var stack []frame
	for index, action := range actions {
		p := p15C01Params(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" {
			key, _ := p["WFDictionaryKey"].(string)
			if strings.HasPrefix(key, "has_arg_") {
				if uuid, ok := p["UUID"].(string); ok {
					getters[uuid] = key
				}
			}
		}
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
			group, _ := p["GroupingIdentifier"].(string)
			switch p15Int(p["WFControlFlowMode"]) {
			case 0:
				key := ""
				outputs, variables := p15V02NativeReferences(p["WFInput"])
				if len(outputs) == 1 {
					key = getters[outputs[0]]
				}
				if len(variables) == 1 && variables[0] == "_cherri_is_fn" {
					key = "dispatcher"
				}
				if key != "" {
					if p15Int(p["WFCondition"]) != 4 || p["WFConditionalActionString"] != "1" {
						return nil, fmt.Errorf("presence/dispatch guard %s does not test exact supplied=1: %+v", key, p)
					}
					guards[key] = index
				}
				stack = append(stack, frame{group, key, 0})
			case 1:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					return nil, fmt.Errorf("unmatched else")
				}
				stack[len(stack)-1].branch = 1
			case 2:
				if len(stack) == 0 || stack[len(stack)-1].group != group {
					return nil, fmt.Errorf("unmatched end")
				}
				stack = stack[:len(stack)-1]
			}
		}
		if marker, ok := p["WFTextActionText"].(string); ok && strings.HasPrefix(marker, "P15_C01_DEFAULT_") {
			want := "has_arg_0"
			if marker == "P15_C01_DEFAULT_B" {
				want = "has_arg_1"
			}
			correctDefault, correctDispatcher := false, false
			for _, scope := range stack {
				if scope.key == want && scope.branch == 1 {
					correctDefault = true
				}
				if scope.key == "dispatcher" && scope.branch == 0 {
					correctDispatcher = true
				}
			}
			if !correctDefault || !correctDispatcher {
				return nil, fmt.Errorf("default %s is outside its omitted-argument dispatcher branch", marker)
			}
			seen[marker]++
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed control")
	}
	for _, marker := range []string{"P15_C01_DEFAULT_A", "P15_C01_DEFAULT_B"} {
		if seen[marker] != 1 {
			return nil, fmt.Errorf("guarded default %s count=%d", marker, seen[marker])
		}
	}
	if guards["has_arg_0"] >= guards["has_arg_1"] {
		return nil, fmt.Errorf("defaults reordered against parameter dependencies")
	}
	return guards, nil
}

func TestP15C01DefaultGuardStructure(t *testing.T) {
	prefix := "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(\"P15_C01_DEFAULT_B\"))->Text{return b}\n"
	for _, call := range []string{"c01Defaults()", "c01Defaults(text(\"explicit\"))", "c01Defaults(b:text(\"explicit\"))", "c01Defaults(b:text(\"second\"),a:text(\"first\"))"} {
		actions := p15CompiledActions(t, prefix+call)
		guards, err := p15C01DefaultGuards(actions)
		if err != nil {
			t.Fatal(call, err)
		}
		for _, key := range []string{"has_arg_0", "has_arg_1", "dispatcher"} {
			mutated := p15C01Clone(t, actions)
			p15C01Params(mutated[guards[key]])["WFConditionalActionString"] = "0"
			if _, err := p15C01DefaultGuards(mutated); err == nil {
				t.Fatal("actual default guard mutation passed", key)
			}
		}
		moved := p15C01Clone(t, actions)
		at := -1
		for i, a := range moved {
			if p15C01Params(a)["WFTextActionText"] == "P15_C01_DEFAULT_A" {
				at = i
				break
			}
		}
		if at < 0 {
			t.Fatal("default producer absent")
		}
		producer := moved[at]
		moved = append(moved[:at], moved[at+1:]...)
		moved = append(moved, producer)
		if _, err := p15C01DefaultGuards(moved); err == nil {
			t.Fatal("default moved to caller/outside guard passed")
		}
	}
	for _, caller := range []string{"", "var a=text(\"CALLER_A\")\n"} {
		actions := p15CompiledActions(t, "function c01Defaults(a:Text=text(\"P15_C01_DEFAULT_A\"),b:Text=text(a))->Text{return b}\n"+caller+"c01Defaults()")
		hasA := ""
		groupA := ""
		resultA := ""
		matched := 0
		for _, a := range actions {
			p := p15C01Params(a)
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && p["WFDictionaryKey"] == "has_arg_0" {
				hasA, _ = p["UUID"].(string)
			}
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
				outputs, _ := p15V02NativeReferences(p["WFInput"])
				if p15Int(p["WFControlFlowMode"]) == 0 && len(outputs) == 1 && outputs[0] == hasA {
					groupA, _ = p["GroupingIdentifier"].(string)
				}
				if p15Int(p["WFControlFlowMode"]) == 2 && p["GroupingIdentifier"] == groupA {
					resultA, _ = p["UUID"].(string)
				}
			}
			if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" {
				outputs, variables := p15V02NativeReferences(p["WFTextActionText"])
				if len(outputs) == 1 && outputs[0] == resultA && resultA != "" && len(variables) == 0 {
					matched++
				}
			}
		}
		if matched != 1 {
			t.Fatalf("earlier parameter default reference count=%d; caller=%q", matched, caller)
		}
	}
}

func p15C01ChangeUUID(value any, before, after string) int {
	changes := 0
	switch node := value.(type) {
	case map[string]any:
		if node["Type"] == "ActionOutput" && node["OutputUUID"] == before {
			node["OutputUUID"] = after
			changes++
		}
		for _, child := range node {
			changes += p15C01ChangeUUID(child, before, after)
		}
	case []any:
		for _, child := range node {
			changes += p15C01ChangeUUID(child, before, after)
		}
	}
	return changes
}

func TestP15C01CLIAndService(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	dir := t.TempDir()
	prefix := "// \U0001f9ea\nfunction c01Flow(a:Number,b:Number)->Number{return a}\nvar value:Number\n"
	svc := service.NewService(nil)
	uri := "file:///p15-c01-order.cherri"
	svc.OpenDocument(uri, 1, "number(0)")
	for index, call := range []string{"c01Flow(b:measure(\"init\"){value=1;value},a:value)", "c01Flow(b:value,a:measure(\"init\"){value=1;value})"} {
		source := prefix + call
		path := filepath.Join(dir, fmt.Sprintf("flow-%d.cherri", index))
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		output, stderr, err := p15RunCLI(t, binary, dir, nil, "check", path, "--json")
		var result protocol.AnalyzeResponse
		if decodeErr := json.Unmarshal(output, &result); decodeErr != nil {
			t.Fatal(decodeErr, string(output), stderr)
		}
		svc.ChangeDocument(uri, index+2, source)
		diagnostics, _ := svc.Analyze(uri)
		var errors []analysis.Diagnostic
		for _, d := range diagnostics {
			if d.Severity == analysis.SeverityError {
				errors = append(errors, d)
			}
		}
		if index == 0 {
			if err != nil || !result.Valid || len(errors) != 0 {
				t.Fatalf("actual source-ordered CLI/service rejected valid call: %v %+v %+v", err, result, errors)
			}
		} else {
			var cliErrors []protocol.DiagnosticItem
			for _, d := range result.Diagnostics {
				if d.Severity == "ERROR" {
					cliErrors = append(cliErrors, d)
				}
			}
			start := strings.Index(call, "value")
			want := protocol.Range{Start: protocol.Position{Line: 3, Character: start}, End: protocol.Position{Line: 3, Character: start + len("value")}}
			if err == nil || result.Valid || len(cliErrors) != 1 || cliErrors[0].Code != analysis.CodeReadBeforeInit || cliErrors[0].Range != want || len(errors) != 1 || errors[0].Code != analysis.CodeReadBeforeInit || errors[0].Span.Start.ByteOffset != len(prefix)+start {
				t.Fatalf("source-first invalid read CLI/service mismatch: %v %+v %+v", err, result, errors)
			}
		}
	}
	source := p15C01Function + "c01Function(c:text(\"P15_C01_C\"),b:text(\"P15_C01_B\"),a:text(\"P15_C01_A\"))"
	path := filepath.Join(dir, "source-order.cherri")
	target := filepath.Join(dir, "source-order.shortcut")
	if err := os.WriteFile(path, []byte(source), 0600); err != nil {
		t.Fatal(err)
	}
	output, stderr, err := p15RunCLI(t, binary, dir, nil, "compile", path, "--skip-sign", "--output="+target)
	if err != nil {
		t.Fatal(err, string(output), stderr)
	}
	raw, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if _, err := plist.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	native := doc["WFWorkflowActions"].([]any)
	actions := make([]map[string]any, len(native))
	for i, a := range native {
		actions[i] = a.(map[string]any)
	}
	ids, err := p15C01MarkerOrder(actions, []string{"P15_C01_C", "P15_C01_B", "P15_C01_A"})
	if err != nil {
		t.Fatal(err)
	}
	if err := p15C01References(p15C01CallFields(t, actions, false), map[string]string{"arg_0": "P15_C01_A", "arg_1": "P15_C01_B", "arg_2": "P15_C01_C"}, ids); err != nil {
		t.Fatal(err)
	}
}

func TestP15C01SnapshotBoundaries(t *testing.T) {
	actions := p15CompiledActions(t, "var dimension=number(12)\nresizeImage(ShortcutInput,width:dimension,height:measure(\"change\"){dimension=number(99);dimension})")
	var resize map[string]any
	mutation := -1
	positions := map[string]int{}
	for i, a := range actions {
		p := p15C01Params(a)
		if uuid, ok := p["UUID"].(string); ok {
			positions[uuid] = i
		}
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.image.resize" {
			resize = p
		}
		if a["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && p["WFVariableName"] == "dimension" {
			mutation = i
		}
	}
	outputs, variables := p15V02NativeReferences(resize["WFImageResizeWidth"])
	if len(outputs) != 1 || len(variables) != 0 || positions[outputs[0]] >= mutation {
		t.Fatalf("canonical argument snapshot lost: outputs=%v variables=%v mutation=%d", outputs, variables, mutation)
	}
	for _, body := range []string{"var value=text(\"local\");value=text(\"changed\");value", "other=text(\"changed\");other"} {
		source := "function c01Pair(a:Text,b:Text)->Text{return a}\nvar value=text(\"outer\")\nvar other=text(\"other\")\nc01Pair(a:value,b:measure(\"local\"){" + body + "})"
		actions := p15CompiledActions(t, source)
		field := p15C01CallFields(t, actions, false)["arg_0"]
		outputs, variables := p15V02NativeReferences(field)
		if len(outputs) != 0 || !reflect.DeepEqual(variables, []string{"value"}) {
			t.Fatalf("unrelated/local shadow write caused unnecessary outer capture: %v %v", outputs, variables)
		}
	}
	standalone := p15CompiledActions(t, "text(\"unused\")")
	if len(standalone) != 1 || p15C01Params(standalone[0])["UUID"] != nil {
		t.Fatal("unused standalone output identity changed")
	}
}

func TestP15C01NestedCustomResult(t *testing.T) {
	source := "function c01Pair(a:Text,b:Text)->Text{return a}\n" + p15C01Custom + "c01Pair(b:text(\"P15_C01_FIRST\"),a:c01Native(text(\"P15_C01_SECOND\"),text(\"P15_C01_THIRD\")))"
	actions := p15CompiledActions(t, source)
	if _, err := p15C01MarkerOrder(actions, []string{"P15_C01_FIRST", "P15_C01_SECOND", "P15_C01_THIRD"}); err != nil {
		t.Fatal(err)
	}
	fields := p15C01CallFields(t, actions, false)
	outputs, variables := p15V02NativeReferences(fields["arg_0"])
	if len(outputs) != 1 || len(variables) != 0 {
		t.Fatalf("nested custom result reference absent: %v %v", outputs, variables)
	}
	found := 0
	for _, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "com.example.c01.order" {
			found++
			if p15C01Params(action)["UUID"] != outputs[0] {
				t.Fatalf("consumed custom result has dangling UUID %s; producer UUID=%v", outputs[0], p15C01Params(action)["UUID"])
			}
		}
	}
	if found != 1 {
		t.Fatalf("custom argument executed %d times", found)
	}
}

func TestP15C01AggregateArgumentSnapshots(t *testing.T) {
	for _, tc := range []struct{ name, typ, value, native string }{
		{"list", "List<Text>", "[\"prefix\", value]", "is.workflow.actions.list"},
		{"map", "Map<Text, Text>", "{\"key\": value}", "is.workflow.actions.dictionary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source := "function c01Aggregate(a:" + tc.typ + ",b:Text)->Text{return b}\nvar value=text(\"initial\")\nc01Aggregate(a:" + tc.value + ",b:measure(\"change\"){value=text(\"changed\");value})"
			actions := p15CompiledActions(t, source)
			outputs, variables := p15V02NativeReferences(p15C01CallFields(t, actions, false)["arg_0"])
			if len(outputs) != 1 || len(variables) != 0 {
				t.Fatalf("aggregate argument is not a captured producer: %v %v", outputs, variables)
			}
			producer, mutation := -1, -1
			for i, action := range actions {
				parameters := p15C01Params(action)
				if parameters["UUID"] == outputs[0] {
					if action["WFWorkflowActionIdentifier"] != tc.native {
						t.Fatalf("aggregate was coerced to a different native type: %v", action["WFWorkflowActionIdentifier"])
					}
					_, names := p15V02NativeReferences(parameters)
					if len(names) != 1 || names[0] != "value" {
						t.Fatalf("aggregate source value lost: %v", names)
					}
					producer = i
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && parameters["WFVariableName"] == "value" {
					mutation = i
				}
			}
			if producer < 0 || mutation < 0 || producer >= mutation {
				t.Fatalf("aggregate producer=%d mutation=%d", producer, mutation)
			}
		})
	}
}
