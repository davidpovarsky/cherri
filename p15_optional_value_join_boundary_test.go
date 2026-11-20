package main

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

type p15OptionalValueJoinCase struct {
	name, source string
}

// This first boundary test specifies source-owned branch presence. It is not a
// full native graph or an Apple execution; those need separate causal proof.
func p15OptionalJoinPresenceBoundary(actions []map[string]any) error {
	flags := map[string]bool{}
	readsPresence := false
	for _, action := range actions {
		params := p15C01Params(action)
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvalueforkey" && params["WFDictionaryKey"] == "has_value" {
			readsPresence = true
		}
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" {
			continue
		}
		fields, err := p15ContainerFields(params["WFItems"])
		if err != nil {
			continue
		}
		marker, exists := fields["_cherri_optional_join_v1"]
		if !exists {
			continue
		}
		if fmt.Sprint(marker["WFItemType"]) != "0" || !p15OptionalJoinLiteralText(marker["WFValue"], "1") {
			return fmt.Errorf("join carrier marker is not exact compiler-owned Text")
		}
		presence, exists := fields["has_value"]
		if !exists || fmt.Sprint(presence["WFItemType"]) != "0" {
			return fmt.Errorf("join has no independent Text presence field")
		}
		for _, flag := range []string{"0", "1"} {
			if p15OptionalJoinLiteralText(presence["WFValue"], flag) {
				if flag == "0" {
					if len(fields) != 2 {
						return fmt.Errorf("absent join carrier fabricated a value")
					}
				} else if _, exists := fields["value"]; !exists || len(fields) != 3 {
					return fmt.Errorf("present join carrier omitted its value")
				}
				flags[flag] = true
			}
		}
	}
	if !flags["0"] || !flags["1"] || !readsPresence {
		return fmt.Errorf("None/present control paths lack separate observed carrier flags and presence read: flags=%v read=%t", flags, readsPresence)
	}
	return nil
}

func p15OptionalJoinLiteralText(value any, want string) bool {
	wrapper, ok := value.(map[string]any)
	if !ok || len(wrapper) != 2 || wrapper["WFSerializationType"] != "WFTextTokenString" {
		return false
	}
	payload, ok := wrapper["Value"].(map[string]any)
	return ok && len(payload) == 1 && payload["string"] == want
}

func p15OptionalValueJoinCases() []p15OptionalValueJoinCase {
	var cases []p15OptionalValueJoinCase
	for _, value := range []struct{ name, expression string }{
		{"empty_text", `""`}, {"false", "false"}, {"zero", "0"}, {"empty_list", "[]"}, {"empty_map", "{}"},
	} {
		for _, control := range []string{"if", "menu"} {
			for _, first := range []bool{false, true} {
				left, right := "none", value.expression
				if first {
					left, right = right, left
				}
				expression := "if flag {yield " + left + "} else {yield " + right + "}"
				if control == "menu" {
					expression = `menu("Pick"){case "A"{yield ` + left + `} case "B"{yield ` + right + `}}`
				}
				source := "var flag:Bool=true\nlet candidate=" + expression + "\ngetVariable(candidate)\nlet absent=candidate==none\ngetVariable(absent)"
				cases = append(cases, p15OptionalValueJoinCase{fmt.Sprintf("%s/%s/present_first_%t", control, value.name, first), source})
			}
		}
	}
	return cases
}

func TestP15OptionalValueJoinsPreservePresence(t *testing.T) {
	for _, tc := range p15OptionalValueJoinCases() {
		t.Run(tc.name, func(t *testing.T) {
			p15FunctionTypedProgram(t, tc.source, map[string]string{})
			actions := p15CompiledActions(t, tc.source)
			p15FunctionBoundaryRetain(t, "optional-join-"+strings.ReplaceAll(tc.name, "/", "-"), actions)
			if err := p15OptionalJoinPresenceBoundary(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalValueJoinsTypedConsumers(t *testing.T) {
	const choose = `if flag {yield none} else {yield ""}`
	const echo = "function echo(value:Text?)->Text?{return value}\n"
	for _, tc := range []p15OptionalValueJoinCase{
		{"annotation", "var flag:Bool=true\nlet value:Text? = " + choose + "\ngetVariable(value==none)"},
		{"return", "function pick(flag:Bool)->Text?{return " + choose + "}\ngetVariable(pick(true))"},
		{"direct_argument", echo + "var flag:Bool=true\ngetVariable(echo(" + choose + "))"},
		{"inferred_argument", echo + "var flag:Bool=true\nlet value=" + choose + "\ngetVariable(echo(value))"},
		{"alias_argument", echo + "var flag:Bool=true\nlet initial=" + choose + "\nlet value=initial\ngetVariable(echo(value))"},
		{"reassignment", echo + "var flag:Bool=true\nvar value:Text?=none\nvalue=" + choose + "\ngetVariable(echo(value))"},
		{"nested_if", "var flag:Bool=true\nlet value=if flag {if flag {yield none}else{yield \"\"}}else{yield \"END\"}\ngetVariable(value==none)"},
		{"yielded_if", "var flag:Bool=true\nlet value=if flag {yield " + choose + "}else{yield \"END\"}\ngetVariable(value==none)"},
		{"nested_menu", "var flag:Bool=true\nlet value=if flag {menu(\"Pick\"){case \"A\"{yield none}case \"B\"{yield \"\"}}}else{yield \"END\"}\ngetVariable(value==none)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p15FunctionTypedProgram(t, tc.source, map[string]string{})
			actions := p15CompiledActions(t, tc.source)
			p15FunctionBoundaryRetain(t, "optional-join-"+tc.name, actions)
			if err := p15OptionalJoinPresenceBoundary(actions); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15OptionalValueJoinsKeepUnconsumedNoneClear(t *testing.T) {
	for _, expression := range []string{
		"if flag {text(\"NOISE\");yield none} else {yield \"NORMAL\"}",
		`menu("Pick"){case "A"{text("NOISE");yield none}case "B"{yield "NORMAL"}}`,
	} {
		source := "let flag=true\nlet value=" + expression + "\ngetVariable(value)"
		actions, err := p15C03Actions(source, false)
		if err != nil {
			t.Fatal(err)
		}
		if err := p15F03NoneBranchOracle(actions); err != nil {
			t.Fatal(err)
		}
		for _, action := range actions {
			params := action.WFWorkflowActionParameters
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.getvalueforkey" && params["WFDictionaryKey"] == "has_value" {
				t.Fatal("unconsumed Optional join gained presence metadata")
			}
		}
	}
}

func TestP15OptionalValueJoinsDoNotInventTypeOrPresence(t *testing.T) {
	for _, tc := range []struct{ name, source, code string }{
		{"wrong_present_type", "function echo(value:Text?)->Text?{return value}\ngetVariable(echo(if true {yield 0}else{yield none}))", analysis.CodeArgumentType},
		{"opaque_native", "function echo(value:AnyContent?)->AnyContent?{return value}\ngetVariable(echo(if true {yield native.action(\"is.workflow.actions.getclipboard\")}else{yield none}))", analysis.CodeTargetValueShape},
		{"missing_yield", "let value=if true {yield none}else{text(\"UNKNOWN\")}\ngetVariable(value==none)", analysis.CodeYieldContext},
	} {
		t.Run(tc.name, func(t *testing.T) {
			found := false
			for _, d := range p15TypeDiagnostics(t, tc.source) {
				if d.Severity == analysis.SeverityError && d.Code == tc.code {
					found = true
				}
			}
			if !found {
				t.Fatal("unsupported value acquired fabricated type/presence evidence", p15TypeDiagnostics(t, tc.source))
			}
			raw, err := CompileSourceToPlist("p15-optional-join-rejection.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("invalid join emitted native bytes=%d err=%v", len(raw), err)
			}
		})
	}
}
