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
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func p15F03Analysis(t *testing.T, src string) (*syntax.Program, *analysis.Analyzer) {
	t.Helper()
	program, errors := p15ParseProgram(src)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(schema.DefaultRegistry())
	analyzer.Analyze(program)
	return program, analyzer
}

func p15F03Errors(analyzer *analysis.Analyzer) []analysis.Diagnostic {
	var out []analysis.Diagnostic
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			out = append(out, d)
		}
	}
	return out
}

func TestP15F03YieldTypesAndNormalPaths(t *testing.T) {
	union := types.NewUnion(types.Text, types.Number)
	for _, tc := range []struct {
		name, expression string
		want             types.Type
	}{
		{"if_text", "if flag {yield \"THEN\"} else {yield \"ELSE\"}", types.Text},
		{"if_number", "if flag {yield 0} else {yield 17}", types.Number},
		{"if_false", "if flag {yield false} else {yield true}", types.Bool},
		{"if_list", "if flag {yield [\"A\",\"B\"]} else {yield [\"C\"]}", types.NewList(types.Text)},
		{"if_map", "if flag {yield {\"flag\":false}} else {yield {\"flag\":true}}", types.NewMap(types.Text, types.Bool)},
		{"if_union", "if flag {yield \"A\"} else {yield 2}", union},
		{"else_if", "if flag {yield \"A\"} else if flag {yield \"B\"} else {yield \"C\"}", types.Text},
		{"nested_if", "if flag {if flag {yield \"A\"} else {yield \"B\"}} else {yield \"C\"}", types.Text},
		{"menu_text", "menu(\"Pick\"){case \"A\"{yield \"A\"} case \"B\"{yield \"B\"}}", types.Text},
		{"menu_union", "menu(\"Pick\"){case \"A\"{yield \"A\"} case \"B\"{yield 2}}", union},
		{"menu_nested_if", "menu(\"Pick\"){case \"A\"{if flag {yield \"A\"} else {yield \"B\"}} case \"C\"{yield \"C\"}}", types.Text},
		{"menu_nested_menu", "menu(\"Outer\"){case \"A\"{menu(\"Inner\"){case \"X\"{yield \"X\"} case \"Y\"{yield \"Y\"}}} case \"B\"{yield \"B\"}}", types.Text},
	} {
		t.Run(tc.name, func(t *testing.T) {
			src := "let flag=true\nlet value=" + tc.expression + "\ngetVariable(value)\n"
			program, analyzer := p15F03Analysis(t, src)
			if errors := p15F03Errors(analyzer); len(errors) != 0 {
				t.Fatal(errors)
			}
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, "value")]
			if actual == nil || !actual.Equals(tc.want) {
				t.Fatalf("full normal-path result type: got %v want %v", actual, tc.want)
			}
			actions, err := p15C03Actions(src, false)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15LegacyInlineReferenceOracle(actions); err != nil {
				t.Fatal(err)
			}
			consumer := actions[len(actions)-1]
			if consumer.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
				t.Fatal("value consumer missing")
			}
			ref := p15OutputReference(consumer.WFWorkflowActionParameters["WFVariable"])
			if ref["Type"] != "ActionOutput" || ref["OutputUUID"] == "" {
				t.Fatal("control result lost its typed native reference", ref)
			}
			found := false
			for _, action := range actions[:len(actions)-1] {
				if action.WFWorkflowActionParameters["UUID"] == ref["OutputUUID"] {
					found = (action.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" || action.WFWorkflowActionIdentifier == "is.workflow.actions.choosefrommenu") && p15Int(action.WFWorkflowActionParameters["WFControlFlowMode"]) == 2
				}
			}
			if !found {
				t.Fatal("value consumer does not refer to its End If/Menu producer")
			}
		})
	}
}

func TestP15F03TerminatedPathsDoNotBecomeValues(t *testing.T) {
	for _, expression := range []string{
		"if flag {return \"EARLY\"} else {yield \"NORMAL\"}",
		"if flag {yield \"NORMAL\"} else {return \"EARLY\"}",
		"menu(\"Pick\"){case \"exit\"{return \"EARLY\"} case \"value\"{yield \"NORMAL\"}}",
		"if flag {if flag {return \"EARLY\"} else {yield \"NORMAL\"}} else {yield \"OTHER\"}",
	} {
		src := "function choose(flag:Bool)->Text{\nlet value:Text=" + expression + "\nreturn value\n}\nlet selected=choose(true)\ntext(selected)\n"
		_, analyzer := p15F03Analysis(t, src)
		if errors := p15F03Errors(analyzer); len(errors) != 0 {
			t.Fatal("terminating paths must not contaminate the normal result", errors)
		}
		actions, err := p15C03Actions(src, false)
		if err != nil {
			t.Fatal(err)
		}
		early := ""
		earlyOutput := 0
		for _, a := range actions {
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && a.WFWorkflowActionParameters["WFTextActionText"] == "EARLY" {
				early, _ = a.WFWorkflowActionParameters["UUID"].(string)
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.output" {
				ref := p15F03ReturnReference(a.WFWorkflowActionParameters["WFOutput"])
				if early != "" && ref["OutputUUID"] == early {
					earlyOutput++
				}
			}
		}
		if early == "" || earlyOutput != 1 {
			t.Fatalf("early path no longer returns its own native value: uuid=%q outputs=%d", early, earlyOutput)
		}
	}
}

// Output preserves its existing single-token string wire shape. Inspect that
// native field rather than treating it as a direct attachment wrapper.
func p15F03ReturnReference(raw any) map[string]any {
	wrapper, _ := raw.(map[string]any)
	value, _ := wrapper["Value"].(map[string]any)
	attachments, _ := value["attachmentsByRange"].(map[string]any)
	if wrapper["WFSerializationType"] != "WFTextTokenString" || value["string"] != "\uFFFC" || len(attachments) != 1 {
		return nil
	}
	ref, _ := attachments["{0, 1}"].(map[string]any)
	if ref["Type"] != "ActionOutput" || ref["Aggrandizements"] != nil {
		return nil
	}
	return ref
}

type p15F03Invalid struct{ name, source, code, spanText string }

var p15F03InvalidCases = []p15F03Invalid{
	{"missing_else", "let value=if true {yield \"A\"}\n", analysis.CodeYieldContext, "if true"},
	{"missing_normal_branch", "let value=if true {yield \"A\"} else {text(\"B\")}\ngetVariable(value)", analysis.CodeYieldContext, "{text(\"B\")}"},
	{"missing_nested_branch", "let value=if true {if false {yield \"A\"} else {text(\"B\")}} else {yield \"C\"}\n", analysis.CodeYieldContext, "{text(\"B\")}"},
	{"overwritten_yield", "let value=if true {yield \"A\";text(\"NOT_THE_YIELD\")} else {yield \"B\"}\n", analysis.CodeYieldContext, "{yield \"A\";text(\"NOT_THE_YIELD\")}"},
	{"menu_missing", "let value=menu(\"Pick\"){case \"A\"{yield \"A\"} case \"B\"{text(\"B\")}}\n", analysis.CodeYieldContext, "{text(\"B\")}"},
	{"void_if_yield", "let value=if true {yield show(\"A\")} else {yield \"B\"}\n", analysis.CodeYieldContext, "yield show(\"A\")"},
	{"void_menu_yield", "let value=menu(\"Pick\"){case \"A\"{yield \"A\"} case \"B\"{yield show(\"B\")}}\n", analysis.CodeYieldContext, "yield show(\"B\")"},
	{"wrong_annotated_branch", "let value:Number=if true {yield 1} else {yield \"B\"}\n", analysis.CodeArgumentType, "if true"},
	{"opaque_cannot_claim_text", "let value:Text=if true {yield ShortcutInput} else {yield \"B\"}\n", analysis.CodeArgumentType, "if true"},
	{"yield_outside", "yield \"A\"\n", analysis.CodeYieldContext, "yield \"A\""},
}

func p15F03DiagnosticOracle(src string, diagnostics []analysis.Diagnostic, code, spanText string) error {
	target := strings.Index(src, spanText)
	if target < 0 {
		return fmt.Errorf("diagnostic source marker missing")
	}
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError && d.Code == code && d.Span.Start.ByteOffset <= target && d.Span.End.ByteOffset >= target+len(spanText) {
			return nil
		}
	}
	return fmt.Errorf("missing source-bound %s diagnostic at %q: %+v", code, spanText, diagnostics)
}

func TestP15F03YieldDiagnosticsProduction(t *testing.T) {
	for _, tc := range p15F03InvalidCases {
		t.Run(tc.name, func(t *testing.T) {
			_, analyzer := p15F03Analysis(t, tc.source)
			diagnostics := analyzer.Diagnostics()
			if err := p15F03DiagnosticOracle(tc.source, diagnostics, tc.code, tc.spanText); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist("p15-f03-negative.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatal("invalid yield reached native output", err)
			}
			broken := append([]analysis.Diagnostic(nil), diagnostics...)
			for i := range broken {
				if broken[i].Code == tc.code {
					broken[i].Code = analysis.CodeUnknownName
				}
			}
			if p15F03DiagnosticOracle(tc.source, broken, tc.code, tc.spanText) == nil {
				t.Fatal("wrong diagnostic class passed")
			}
			for i := range broken {
				broken[i] = diagnostics[i]
				broken[i].Span.Start.ByteOffset = len(tc.source)
				broken[i].Span.End.ByteOffset = len(tc.source)
			}
			if p15F03DiagnosticOracle(tc.source, broken, tc.code, tc.spanText) == nil {
				t.Fatal("foreign diagnostic source location passed")
			}
		})
	}
}

func TestP15F03ImplicitResultBoundary(t *testing.T) {
	for _, expression := range []string{
		"if true {text(\"A\")} else {text(\"B\")}",
		"menu(\"Pick\"){case \"A\"{text(\"A\")} case \"B\"{text(\"B\")}}",
	} {
		src := "let value=" + expression + "\ngetVariable(value)\n"
		program, analyzer := p15F03Analysis(t, src)
		if errors := p15F03Errors(analyzer); len(errors) != 0 {
			t.Fatal(errors)
		}
		typ := analyzer.ExprTypes()[p15BindingValue(t, program, "value")]
		if typ == nil || !typ.Equals(types.AnyContent) {
			t.Fatal("implicit native last action acquired a typed yield promise", typ)
		}
		if _, err := p15C03Actions(src, false); err != nil {
			t.Fatal(err)
		}
		strict := "let value:Text=" + expression + "\n"
		raw, err := CompileSourceToPlist("p15-f03-implicit.cherri", strict)
		if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeArgumentType) {
			t.Fatal("implicit result bypassed strict authored type evidence", err)
		}
	}
}

type p15F03NativeGraph struct {
	actions   []ShortcutAction
	producers map[string]int
}

func p15F03Graph(actions []ShortcutAction) (*p15F03NativeGraph, error) {
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return nil, err
	}
	g := &p15F03NativeGraph{actions: actions, producers: map[string]int{}}
	for i, a := range actions {
		if id, ok := a.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			g.producers[id] = i
		}
	}
	return g, nil
}

func (g *p15F03NativeGraph) origins(index int, seen map[int]bool) ([]string, error) {
	if index < 0 || index >= len(g.actions) || seen[index] {
		return nil, fmt.Errorf("missing or cyclic result producer")
	}
	path := map[int]bool{}
	for k, v := range seen {
		path[k] = v
	}
	path[index] = true
	a := g.actions[index]
	p := a.WFWorkflowActionParameters
	switch a.WFWorkflowActionIdentifier {
	case "is.workflow.actions.gettext":
		value, ok := p["WFTextActionText"].(string)
		if !ok {
			return nil, fmt.Errorf("expected exact literal Text producer")
		}
		return []string{value}, nil
	case "is.workflow.actions.getvariable":
		ref := p15OutputReference(p["WFVariable"])
		id, _ := ref["OutputUUID"].(string)
		producer, found := g.producers[id]
		if ref["Type"] != "ActionOutput" || !found || producer >= index {
			return nil, fmt.Errorf("result bridge lost its original earlier producer")
		}
		return g.origins(producer, path)
	case "is.workflow.actions.output":
		return nil, nil // this branch terminates instead of reaching the join
	case "is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu":
		if p15Int(p["WFControlFlowMode"]) != 2 {
			return nil, fmt.Errorf("result is not an End If/Menu")
		}
		group, ok := p["GroupingIdentifier"].(string)
		if !ok || group == "" {
			return nil, fmt.Errorf("control result has no group identity")
		}
		start := -1
		var separators []int
		for i := 0; i < index; i++ {
			candidate := g.actions[i]
			if candidate.WFWorkflowActionIdentifier != a.WFWorkflowActionIdentifier || candidate.WFWorkflowActionParameters["GroupingIdentifier"] != group {
				continue
			}
			switch p15Int(candidate.WFWorkflowActionParameters["WFControlFlowMode"]) {
			case 0:
				if start >= 0 {
					return nil, fmt.Errorf("duplicate control start")
				}
				start = i
			case 1:
				separators = append(separators, i)
			case 2:
				return nil, fmt.Errorf("duplicate earlier control end")
			}
		}
		if start < 0 || len(separators) == 0 {
			return nil, fmt.Errorf("missing value path separator")
		}
		var ends []int
		if a.WFWorkflowActionIdentifier == "is.workflow.actions.conditional" {
			if len(separators) != 1 {
				return nil, fmt.Errorf("If requires exactly one Else")
			}
			ends = []int{separators[0] - 1, index - 1}
		} else {
			if separators[0] != start+1 {
				return nil, fmt.Errorf("menu has effects outside a case")
			}
			for _, next := range separators[1:] {
				ends = append(ends, next-1)
			}
			ends = append(ends, index-1)
		}
		var values []string
		for _, end := range ends {
			if end <= start {
				return nil, fmt.Errorf("normal branch has no result")
			}
			child, err := g.origins(end, path)
			if err != nil {
				return nil, err
			}
			values = append(values, child...)
		}
		return values, nil
	default:
		return nil, fmt.Errorf("unexpected final result action %s", a.WFWorkflowActionIdentifier)
	}
}

func p15F03NativeResultOracle(actions []ShortcutAction, want []string) error {
	g, err := p15F03Graph(actions)
	if err != nil {
		return err
	}
	if len(actions) == 0 {
		return fmt.Errorf("missing native output")
	}
	last := len(actions) - 1
	if actions[last].WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" {
		return fmt.Errorf("missing final result consumer")
	}
	ref := p15OutputReference(actions[last].WFWorkflowActionParameters["WFVariable"])
	id, _ := ref["OutputUUID"].(string)
	producer, found := g.producers[id]
	if ref["Type"] != "ActionOutput" || !found || producer >= last {
		return fmt.Errorf("final result ref is not earlier control output")
	}
	output := actions[producer]
	if output.WFWorkflowActionIdentifier != "is.workflow.actions.conditional" && output.WFWorkflowActionIdentifier != "is.workflow.actions.choosefrommenu" {
		return fmt.Errorf("consumer bypassed control result")
	}
	got, err := g.origins(producer, map[int]bool{})
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("normal-path outputs got %v want %v", got, want)
	}
	return nil
}

var p15F03WireCases = []struct {
	name, expression string
	origins          []string
}{
	{"if", "if flag {text(\"NOISE\");yield chosen} else {yield text(\"ELSE\")}", []string{"CHOSEN", "ELSE"}},
	{"else_if", "if flag {yield chosen} else if flag {yield text(\"MIDDLE\")} else {yield text(\"ELSE\")}", []string{"CHOSEN", "MIDDLE", "ELSE"}},
	{"nested_if", "if flag {if flag {yield chosen} else {yield text(\"INNER\")}} else {yield text(\"ELSE\")}", []string{"CHOSEN", "INNER", "ELSE"}},
	{"menu", "menu(\"Pick\"){case \"A\"{text(\"NOISE\");yield chosen} case \"B\"{yield text(\"ELSE\")}}", []string{"CHOSEN", "ELSE"}},
	{"nested_menu", "menu(\"Outer\"){case \"A\"{menu(\"Inner\"){case \"X\"{yield chosen} case \"Y\"{yield text(\"INNER\")}}} case \"B\"{yield text(\"ELSE\")}}", []string{"CHOSEN", "INNER", "ELSE"}},
}

func p15F03WireSource(expression string) string {
	return "let flag=true\nlet chosen=text(\"CHOSEN\")\nlet value=" + expression + "\ngetVariable(value)\n"
}

func TestP15F03NativeBranchResults(t *testing.T) {
	for _, tc := range p15F03WireCases {
		t.Run(tc.name, func(t *testing.T) {
			actions, err := p15C03Actions(p15F03WireSource(tc.expression), false)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15F03NativeResultOracle(actions, tc.origins); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15F03WireOracleMutations(t *testing.T) {
	tc := p15F03WireCases[0]
	actions, err := p15C03Actions(p15F03WireSource(tc.expression), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = p15F03NativeResultOracle(actions, tc.origins); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"wrong_branch_value", "foreign_final_result", "wrong_end_group", "missing_yield_bridge", "duplicate_uuid", "removed_else"} {
		t.Run(fault, func(t *testing.T) {
			broken := p15C03Clone(t, actions)
			switch fault {
			case "wrong_branch_value":
				for i := range broken {
					if broken[i].WFWorkflowActionParameters["WFTextActionText"] == "ELSE" {
						broken[i].WFWorkflowActionParameters["WFTextActionText"] = "CORRUPTED"
					}
				}
			case "foreign_final_result":
				p15OutputReference(broken[len(broken)-1].WFWorkflowActionParameters["WFVariable"])["OutputUUID"] = "foreign"
			case "wrong_end_group":
				for i := range broken {
					if broken[i].WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && p15Int(broken[i].WFWorkflowActionParameters["WFControlFlowMode"]) == 2 {
						broken[i].WFWorkflowActionParameters["GroupingIdentifier"] = "foreign"
					}
				}
			case "missing_yield_bridge":
				for i := 0; i < len(broken)-1; i++ {
					if broken[i].WFWorkflowActionIdentifier == "is.workflow.actions.getvariable" {
						broken = append(broken[:i], broken[i+1:]...)
						break
					}
				}
			case "duplicate_uuid":
				broken[len(broken)-1].WFWorkflowActionParameters["UUID"] = broken[0].WFWorkflowActionParameters["UUID"]
			case "removed_else":
				for i := range broken {
					if broken[i].WFWorkflowActionIdentifier == "is.workflow.actions.conditional" && p15Int(broken[i].WFWorkflowActionParameters["WFControlFlowMode"]) == 1 {
						broken = append(broken[:i], broken[i+1:]...)
						break
					}
				}
			}
			if p15F03NativeResultOracle(broken, tc.origins) == nil {
				t.Fatal("causal native-result mutation passed")
			}
		})
	}
}

func TestP15F03CLIAndService(t *testing.T) {
	bin := p15AcceptanceCLI(t)
	svc := service.NewService(nil)
	uri := "file:///p15-f03-values.cherri"
	healthy := p15F03WireSource(p15F03WireCases[0].expression)
	svc.OpenDocument(uri, 1, healthy)
	for i, src := range []string{healthy, p15F03InvalidCases[1].source} {
		dir := t.TempDir()
		file, target := filepath.Join(dir, "flow.cherri"), filepath.Join(dir, "flow.shortcut")
		if err := os.WriteFile(file, []byte(src), 0600); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, err := p15RunCLI(t, bin, dir, nil, "check", file, "--json")
		var result protocol.AnalyzeResponse
		if decode := json.Unmarshal(stdout, &result); decode != nil {
			t.Fatal(decode, string(stdout), stderr)
		}
		svc.ChangeDocument(uri, i+2, src)
		diagnostics, fingerprint := svc.Analyze(uri)
		if fingerprint == "" {
			t.Fatal("service analysis omitted its registry identity")
		}
		if i == 0 {
			if err != nil || !result.Valid {
				t.Fatal("healthy actual CLI rejected", err, result)
			}
			for _, d := range diagnostics {
				if d.Severity == analysis.SeverityError {
					t.Fatal(d)
				}
			}
		} else {
			if err == nil || result.Valid {
				t.Fatal("missing yield passed actual CLI")
			}
			if e := p15F03DiagnosticOracle(src, diagnostics, analysis.CodeYieldContext, "{text(\"B\")}"); e != nil {
				t.Fatal(e)
			}
			found := false
			for _, d := range result.Diagnostics {
				if d.Severity == "ERROR" && d.Code == analysis.CodeYieldContext {
					found = true
				}
			}
			if !found {
				t.Fatal("CLI missing precise yield diagnostic")
			}
		}
		_, stderr, compileErr := p15RunCLI(t, bin, dir, nil, "compile", file, "--skip-sign", "-o", target)
		raw, readErr := os.ReadFile(target)
		if i == 0 {
			if compileErr != nil || readErr != nil {
				t.Fatal(compileErr, readErr, stderr)
			}
			doc, e := decodeShortcutRawMap(raw)
			if e != nil {
				t.Fatal(e)
			}
			var actions []ShortcutAction
			for _, entry := range p15C03Array(doc["WFWorkflowActions"]) {
				row := debugParamMap(entry)
				actions = append(actions, ShortcutAction{WFWorkflowActionIdentifier: fmt.Sprint(row["WFWorkflowActionIdentifier"]), WFWorkflowActionParameters: debugParamMap(row["WFWorkflowActionParameters"])})
			}
			if e := p15F03NativeResultOracle(actions, p15F03WireCases[0].origins); e != nil {
				t.Fatal(e)
			}
		} else if compileErr == nil || !os.IsNotExist(readErr) {
			t.Fatal("rejected yield wrote output", compileErr, readErr)
		}
	}
}

func p15F03NoneBranchOracle(actions []ShortcutAction) error {
	if len(actions) < 2 {
		return fmt.Errorf("missing None branch")
	}
	if err := p15LegacyInlineReferenceOracle(actions); err != nil {
		return err
	}
	final := actions[len(actions)-1]
	ref := p15OutputReference(final.WFWorkflowActionParameters["WFVariable"])
	end := -1
	for index, action := range actions[:len(actions)-1] {
		if action.WFWorkflowActionParameters["UUID"] == ref["OutputUUID"] {
			end = index
		}
	}
	if final.WFWorkflowActionIdentifier != "is.workflow.actions.getvariable" || ref["Type"] != "ActionOutput" ||
		end < 0 || p15Int(actions[end].WFWorkflowActionParameters["WFControlFlowMode"]) != 2 {
		return fmt.Errorf("None result is not the exact control-boundary output")
	}
	control := actions[end]
	group := control.WFWorkflowActionParameters["GroupingIdentifier"]
	var separators []int
	for index, action := range actions[:end] {
		if action.WFWorkflowActionIdentifier == control.WFWorkflowActionIdentifier &&
			action.WFWorkflowActionParameters["GroupingIdentifier"] == group &&
			p15Int(action.WFWorkflowActionParameters["WFControlFlowMode"]) == 1 {
			separators = append(separators, index)
		}
	}
	firstEnd := -1
	switch control.WFWorkflowActionIdentifier {
	case "is.workflow.actions.conditional":
		if len(separators) == 1 {
			firstEnd = separators[0] - 1
		}
	case "is.workflow.actions.choosefrommenu":
		if len(separators) == 2 {
			firstEnd = separators[1] - 1
		}
	}
	if firstEnd < 1 || actions[firstEnd-1].WFWorkflowActionParameters["WFTextActionText"] != "NOISE" {
		return fmt.Errorf("None yield inherited the preceding nonempty native output")
	}
	switch actions[firstEnd].WFWorkflowActionIdentifier {
	case "is.workflow.actions.nothing":
	case "is.workflow.actions.getvariable":
		// The existing nil binding uses Set Variable without WFInput. Its
		// empty behavior has a separate native fixture; do not replace it
		// with a different encoding merely to match a yield oracle.
		reference := p15OutputReference(actions[firstEnd].WFWorkflowActionParameters["WFVariable"])
		if reference["Type"] != "Variable" || reference["VariableName"] != "absent" || reference["Aggrandizements"] != nil {
			return fmt.Errorf("bound None yielded another reference")
		}
		found := false
		for _, action := range actions[:firstEnd] {
			if action.WFWorkflowActionParameters["WFVariableName"] != "absent" {
				continue
			}
			_, hasInput := action.WFWorkflowActionParameters["WFInput"]
			if found || action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" || hasInput {
				return fmt.Errorf("bound None no longer uses its original empty binding")
			}
			found = true
		}
		if !found {
			return fmt.Errorf("bound None lost its original empty binding")
		}
	default:
		return fmt.Errorf("None yield did not materialize an empty native result")
	}
	if actions[end-1].WFWorkflowActionIdentifier != "is.workflow.actions.gettext" ||
		actions[end-1].WFWorkflowActionParameters["WFTextActionText"] != "NORMAL" {
		return fmt.Errorf("present branch no longer retains its Text value")
	}
	return nil
}

func TestP15F03NoneYieldClearsPrecedingOutput(t *testing.T) {
	for _, value := range []string{"none", "absent"} {
		for _, expression := range []string{
			"if flag {text(\"NOISE\");yield " + value + "} else {yield \"NORMAL\"}",
			"menu(\"Pick\"){case \"none\"{text(\"NOISE\");yield " + value + "} case \"value\"{yield \"NORMAL\"}}",
		} {
			src := "let flag=true\nlet absent=none\nlet value=" + expression + "\ngetVariable(value)\n"
			program, analyzer := p15F03Analysis(t, src)
			if errors := p15F03Errors(analyzer); len(errors) != 0 {
				t.Fatal(errors)
			}
			want := types.NewUnion(types.NewOptional(types.Unknown), types.Text)
			actual := analyzer.ExprTypes()[p15BindingValue(t, program, "value")]
			if actual == nil || !actual.Equals(want) {
				t.Fatalf("None became Void/present Text: got %v want %v", actual, want)
			}
			actions, err := p15C03Actions(src, false)
			if err != nil {
				t.Fatal(err)
			}
			if err = p15F03NoneBranchOracle(actions); err != nil {
				t.Fatal(err)
			}
			broken := p15C03Clone(t, actions)
			changed := false
			for i := range broken {
				if broken[i].WFWorkflowActionParameters["WFTextActionText"] == "NOISE" && i+1 < len(broken) {
					broken = append(broken[:i+1], broken[i+2:]...)
					changed = true
					break
				}
			}
			if !changed || p15F03NoneBranchOracle(broken) == nil {
				t.Fatal("omitted None materializer passed")
			}
		}
	}
}
