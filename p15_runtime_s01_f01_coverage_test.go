package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

const p15CoveragePath = "tests/runtime_poc/CherriS01F01CoveragePOC.cherri"
const p15CoverageSourceSHA = "4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3"
const p15CoverageUnicode = "שָׁלוֹם👩‍💻é|α|אבב|éα|🌍🌍||\uFFFC🙂αבéαשָׁ🌍"
const p15CoverageTrace = "START|OR_TRUE|OR_FALSE|NEST_OR|NEST_AND|MIX_LEFT|MIX_RIGHT"
const p15CoverageResults = "and_skip=F|or_skip=T|false_or_true=T|false_or_false=F|nested_or=T|nested_and=F|outer_and=F|outer_or=T|mixed_run=T"

// These are expectations, never an execution receipt. Native execution must
// separately compare the complete clipboard bytes with this public contract.
func p15CoverageContractError(source, encoded []byte) error {
	if fmt.Sprintf("%x", sha256.Sum256(source)) != p15CoverageSourceSHA {
		return fmt.Errorf("coverage source differs from the reviewed public case")
	}
	var actual map[string]any
	if err := json.Unmarshal(encoded, &actual); err != nil {
		return err
	}
	var clipboard = "CHERRI_S01_F01_COVERAGE_OK\nS01_OUTPUT=" + p15CoverageUnicode +
		"\nF01_TRACE=" + p15CoverageTrace + "\nF01_RESULTS=" + p15CoverageResults
	var cases []any
	for _, scenario := range p15CoverageCases() {
		cases = append(cases, scenario.name)
	}
	var expected = map[string]any{
		"schema_version": float64(1), "source_path": p15CoveragePath, "source_sha256": p15CoverageSourceSHA,
		"requirements": []any{"S01", "F01"}, "runtime_status": "NOT_RUN",
		"expected_clipboard": clipboard, "expected_clipboard_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(clipboard))),
		"expected_fields": map[string]any{"S01_OUTPUT": p15CoverageUnicode, "F01_TRACE": p15CoverageTrace, "F01_RESULTS": p15CoverageResults},
		"coverage": map[string]any{
			"S01": []any{"Hebrew marks before tokens", "astral and ZWJ characters before tokens", "combining marks before tokens", "repeated producer", "mutable producer", "literal U+FFFC is not an attachment"},
			"F01": cases,
		},
	}
	if !reflect.DeepEqual(actual, expected) {
		return fmt.Errorf("coverage expectations, scope, bytes or hashes changed")
	}
	return nil
}

type p15CoverageExpr struct {
	op, name, marker string
	left, right      *p15CoverageExpr
}

func p15CoverageVariable(name string) *p15CoverageExpr {
	return &p15CoverageExpr{op: "variable", name: name}
}

func p15CoverageEffect(marker, name string) *p15CoverageExpr {
	return &p15CoverageExpr{op: "effect", marker: marker, name: name}
}

func p15CoverageBinary(op string, left, right *p15CoverageExpr) *p15CoverageExpr {
	return &p15CoverageExpr{op: op, left: left, right: right}
}

type p15CoverageCase struct {
	name string
	expr *p15CoverageExpr
}

// Independently stated logical trees. Matching these against serialized native
// groups checks lazy placement and data flow, not simulated Apple execution.
func p15CoverageCases() []p15CoverageCase {
	var flag, truth = p15CoverageVariable("flag"), p15CoverageVariable("truth")
	return []p15CoverageCase{
		{"and_skip", p15CoverageBinary("&&", flag, p15CoverageEffect("BAD_AND", "truth"))},
		{"or_skip", p15CoverageBinary("||", truth, p15CoverageEffect("BAD_OR", "flag"))},
		{"false_or_true", p15CoverageBinary("||", flag, p15CoverageEffect("OR_TRUE", "truth"))},
		{"false_or_false", p15CoverageBinary("||", flag, p15CoverageEffect("OR_FALSE", "flag"))},
		{"nested_or", p15CoverageBinary("||",
			p15CoverageBinary("&&", flag, p15CoverageEffect("BAD_NEST_OR", "truth")),
			p15CoverageBinary("&&", truth, p15CoverageEffect("NEST_OR", "truth")))},
		{"nested_and", p15CoverageBinary("&&",
			p15CoverageBinary("||", truth, p15CoverageEffect("BAD_NEST_AND", "flag")),
			p15CoverageBinary("||", flag, p15CoverageEffect("NEST_AND", "flag")))},
		{"outer_and", p15CoverageBinary("&&", flag,
			p15CoverageBinary("||", p15CoverageEffect("BAD_OUTER_AND_LEFT", "truth"), p15CoverageEffect("BAD_OUTER_AND_RIGHT", "flag")))},
		{"outer_or", p15CoverageBinary("||", truth,
			p15CoverageBinary("&&", p15CoverageEffect("BAD_OUTER_OR_LEFT", "flag"), p15CoverageEffect("BAD_OUTER_OR_RIGHT", "truth")))},
		{"mixed_run", p15CoverageBinary("&&", truth,
			p15CoverageBinary("||", p15CoverageEffect("MIX_LEFT", "flag"), p15CoverageEffect("MIX_RIGHT", "truth")))},
	}
}

type p15CoverageNode struct {
	action          map[string]any
	then, otherwise []*p15CoverageNode
	end             map[string]any
	hasElse         bool
}

func (n *p15CoverageNode) kind() string {
	return strings.TrimPrefix(n.action["WFWorkflowActionIdentifier"].(string), "is.workflow.actions.")
}
func (n *p15CoverageNode) params() map[string]any { return p15Param(n.action) }

func p15CoverageTree(actions []map[string]any) ([]*p15CoverageNode, error) {
	var roots, stack []*p15CoverageNode
	var groups, identities = map[string]bool{}, map[string]bool{}
	var allowed = map[string]bool{"gettext": true, "setvariable": true, "number": true, "conditional": true, "comment": true, "setclipboard": true}
	var appendNode = func(node *p15CoverageNode) {
		if len(stack) == 0 {
			roots = append(roots, node)
		} else {
			var parent = stack[len(stack)-1]
			if parent.hasElse {
				parent.otherwise = append(parent.otherwise, node)
			} else {
				parent.then = append(parent.then, node)
			}
		}
	}
	for _, action := range actions {
		var identifier, ok = action["WFWorkflowActionIdentifier"].(string)
		if !ok || !strings.HasPrefix(identifier, "is.workflow.actions.") || !allowed[strings.TrimPrefix(identifier, "is.workflow.actions.")] {
			return nil, fmt.Errorf("unexpected external or unmodeled native action")
		}
		var p = p15Param(action)
		if uuid, ok := p["UUID"].(string); ok {
			if uuid == "" || identities[uuid] {
				return nil, fmt.Errorf("empty or duplicated native identity")
			}
			identities[uuid] = true
		}
		if identifier != "is.workflow.actions.conditional" {
			appendNode(&p15CoverageNode{action: action})
			continue
		}
		var group, _ = p["GroupingIdentifier"].(string)
		if !p15CoverageInteger(p["WFControlFlowMode"], 0) && !p15CoverageInteger(p["WFControlFlowMode"], 1) && !p15CoverageInteger(p["WFControlFlowMode"], 2) {
			return nil, fmt.Errorf("native control mode is not an exact integer")
		}
		switch p15Int(p["WFControlFlowMode"]) {
		case 0:
			if group == "" || groups[group] {
				return nil, fmt.Errorf("empty or repeated control group")
			}
			groups[group] = true
			var node = &p15CoverageNode{action: action}
			appendNode(node)
			stack = append(stack, node)
		case 1, 2:
			if len(stack) == 0 || stack[len(stack)-1].params()["GroupingIdentifier"] != group {
				return nil, fmt.Errorf("unmatched native branch")
			}
			var node = stack[len(stack)-1]
			if p15Int(p["WFControlFlowMode"]) == 1 {
				if node.hasElse {
					return nil, fmt.Errorf("duplicate native else")
				}
				node.hasElse = true
			} else {
				if !node.hasElse {
					return nil, fmt.Errorf("missing native alternate")
				}
				node.end = p
				stack = stack[:len(stack)-1]
			}
		default:
			return nil, fmt.Errorf("unmodeled control mode")
		}
	}
	if len(stack) != 0 {
		return nil, fmt.Errorf("unclosed native control tree")
	}
	return roots, nil
}

func p15CoverageOutput(p map[string]any) map[string]any {
	return map[string]any{"Type": "ActionOutput", "OutputUUID": p["UUID"]}
}

func p15CoverageRefError(value any, want map[string]any) error {
	var actual, ok = value.(map[string]any)
	if !ok {
		return fmt.Errorf("missing native descriptor")
	}
	if want["Type"] == "ActionOutput" && (want["OutputUUID"] == nil || want["OutputUUID"] == "") {
		return fmt.Errorf("missing native source identity")
	}
	for key, value := range want {
		if actual[key] != value {
			return fmt.Errorf("native reference changed %s", key)
		}
	}
	for key := range actual {
		if _, ok := want[key]; !ok && key != "OutputName" {
			return fmt.Errorf("unexpected native reference transformation")
		}
	}
	return nil
}

func p15CoverageAttachmentError(value any, want map[string]any) error {
	var outer, ok = value.(map[string]any)
	if !ok || len(outer) != 2 || outer["WFSerializationType"] != "WFTextTokenAttachment" {
		return fmt.Errorf("missing native reference envelope")
	}
	return p15CoverageRefError(outer["Value"], want)
}

func p15CoverageInteger(value any, want int) bool {
	switch number := value.(type) {
	case int:
		return number == want
	case uint64:
		return want >= 0 && number == uint64(want)
	case int64:
		return number == int64(want)
	case float64:
		return number == float64(want)
	}
	return false
}

func p15CoverageTextError(value any, text string, refs map[string]map[string]any) error {
	var actual, attachments, ok = p15BindingTokenText(value)
	if !ok || actual != text || len(attachments) != len(refs) {
		return fmt.Errorf("observable text or attachment count changed")
	}
	for key, want := range refs {
		if err := p15CoverageRefError(attachments[key], want); err != nil {
			return err
		}
	}
	return nil
}

func p15CoveragePredicate(node *p15CoverageNode, input map[string]any) error {
	var p = node.params()
	if node.kind() != "conditional" || !p15CoverageInteger(p["WFCondition"], 4) || !p15CoverageInteger(p["WFNumberValue"], 1) ||
		p["WFConditionalLegacyComparisonBehavior"] != true || p["WFConditions"] != nil || node.end == nil {
		return fmt.Errorf("Boolean predicate is no longer exact native equality to true")
	}
	var outer, ok = p["WFInput"].(map[string]any)
	if !ok || len(outer) != 2 || outer["Type"] != "Variable" {
		return fmt.Errorf("predicate input lost its native wrapper")
	}
	return p15CoverageAttachmentError(outer["Variable"], input)
}

func p15CoverageNumber(nodes []*p15CoverageNode, want int) error {
	if len(nodes) != 1 || nodes[0].kind() != "number" || !p15CoverageInteger(nodes[0].params()["WFNumberActionNumber"], want) {
		return fmt.Errorf("native Boolean yield changed")
	}
	return nil
}

func p15CoverageEffectBranch(nodes []*p15CoverageNode, marker string, result int) error {
	if len(nodes) != 3 || nodes[0].kind() != "gettext" || nodes[1].kind() != "setvariable" ||
		nodes[1].params()["WFVariableName"] != "trace" {
		return fmt.Errorf("effect is missing, duplicated or outside its branch")
	}
	if err := p15CoverageTextError(nodes[0].params()["WFTextActionText"], "\uFFFC|"+marker,
		map[string]map[string]any{"{0, 1}": {"Type": "Variable", "VariableName": "trace"}}); err != nil {
		return err
	}
	if err := p15CoverageAttachmentError(nodes[1].params()["WFInput"], p15CoverageOutput(nodes[0].params())); err != nil {
		return err
	}
	return p15CoverageNumber(nodes[2:], result)
}

func p15CoverageMatch(expr *p15CoverageExpr, nodes []*p15CoverageNode) ([]*p15CoverageNode, map[string]any, error) {
	if expr.op == "variable" {
		return nodes, map[string]any{"Type": "Variable", "VariableName": expr.name}, nil
	}
	if expr.op == "effect" {
		if len(nodes) == 0 {
			return nil, nil, fmt.Errorf("missing effect %s", expr.marker)
		}
		var node = nodes[0]
		if err := p15CoveragePredicate(node, map[string]any{"Type": "Variable", "VariableName": expr.name}); err != nil {
			return nil, nil, err
		}
		if err := p15CoverageEffectBranch(node.then, expr.marker, 1); err != nil {
			return nil, nil, err
		}
		if err := p15CoverageEffectBranch(node.otherwise, expr.marker, 0); err != nil {
			return nil, nil, err
		}
		return nodes[1:], p15CoverageOutput(node.end), nil
	}
	var remaining, left, err = p15CoverageMatch(expr.left, nodes)
	if err != nil || len(remaining) == 0 {
		return nil, nil, fmt.Errorf("missing lazy logical group: %v", err)
	}
	var node = remaining[0]
	if err := p15CoveragePredicate(node, left); err != nil {
		return nil, nil, err
	}
	var rightNodes = node.then
	var skipped = node.otherwise
	var skipValue = 0
	if expr.op == "||" {
		rightNodes, skipped, skipValue = node.otherwise, node.then, 1
	} else if expr.op != "&&" {
		return nil, nil, fmt.Errorf("unknown independent logical case")
	}
	if err := p15CoverageNumber(skipped, skipValue); err != nil {
		return nil, nil, err
	}
	if tail, _, err := p15CoverageMatch(expr.right, rightNodes); err != nil || len(tail) != 0 {
		return nil, nil, fmt.Errorf("RHS escaped or changed its lazy branch: %v", err)
	}
	return remaining[1:], p15CoverageOutput(node.end), nil
}

func p15CoverageGraphError(actions []map[string]any) error {
	var roots, err = p15CoverageTree(actions)
	if err != nil {
		return err
	}
	var named = map[string]map[string]any{}
	var positions = map[string]int{}
	for index, action := range actions {
		if uuid, ok := p15Param(action)["UUID"].(string); ok && uuid != "" {
			positions[uuid] = index
		}
	}
	var checkOrder func(any, int) error
	checkOrder = func(value any, index int) error {
		switch node := value.(type) {
		case map[string]any:
			if node["Type"] == "ActionOutput" {
				var uuid, _ = node["OutputUUID"].(string)
				var producer, found = positions[uuid]
				if !found || producer >= index {
					return fmt.Errorf("native reference is detached or precedes its producer")
				}
			}
			for _, child := range node {
				if err := checkOrder(child, index); err != nil {
					return err
				}
			}
		case []any:
			for _, child := range node {
				if err := checkOrder(child, index); err != nil {
					return err
				}
			}
		}
		return nil
	}
	for index, action := range actions {
		if err := checkOrder(p15Param(action), index); err != nil {
			return err
		}
	}
	for _, action := range actions {
		var p = p15Param(action)
		var name, _ = p["CustomOutputName"].(string)
		if name == "first" || name == "unicodeFirst" || name == "unicodeSecond" || name == "unicodeOutput" || name == "zero" || name == "one" || name == "results" || name == "finalResult" {
			if named[name] != nil {
				return fmt.Errorf("duplicated named observed producer")
			}
			var expectedKind = "is.workflow.actions.gettext"
			if name == "zero" || name == "one" {
				expectedKind = "is.workflow.actions.number"
			}
			if action["WFWorkflowActionIdentifier"] != expectedKind {
				return fmt.Errorf("observed producer action kind changed")
			}
			named[name] = p
		}
	}
	for _, name := range []string{"first", "unicodeFirst", "unicodeSecond", "unicodeOutput", "zero", "one", "results", "finalResult"} {
		if named[name] == nil || named[name]["UUID"] == nil || named[name]["UUID"] == "" {
			return fmt.Errorf("missing actual producer %s", name)
		}
	}
	for index, name := range []string{"unicodeFirst", "unicodeSecond"} {
		var tc = p15UnicodeTextCases()[index]
		var refs, refsErr = p15UnicodeExpectedRefs(actions, tc)
		if refsErr != nil {
			return refsErr
		}
		tc.text = strings.TrimSuffix(tc.text, "\n")
		if err := p15UnicodeTextError(named[name]["WFTextActionText"], tc, refs); err != nil {
			return err
		}
	}
	if err := p15CoverageTextError(named["unicodeOutput"]["WFTextActionText"], "\uFFFC||\uFFFC",
		map[string]map[string]any{"{0, 1}": p15CoverageOutput(named["unicodeFirst"]), "{3, 1}": p15CoverageOutput(named["unicodeSecond"])}); err != nil {
		return err
	}
	var markers []int
	for index, node := range roots {
		if node.kind() == "comment" {
			markers = append(markers, index)
		}
	}
	var scenarios = p15CoverageCases()
	if len(markers) != len(scenarios)+1 || markers[0] != 16 {
		return fmt.Errorf("case boundaries or seed prefix changed")
	}
	var prefix = roots[:markers[0]]
	for _, seed := range []struct {
		name, variable string
		value          int
	}{{"zero", "flag", 0}, {"one", "truth", 1}} {
		if !p15CoverageInteger(named[seed.name]["WFNumberActionNumber"], seed.value) {
			return fmt.Errorf("action-derived Boolean seed changed")
		}
		var stores int
		for index, node := range prefix {
			if node.kind() == "setvariable" && node.params()["WFVariableName"] == seed.variable {
				stores++
				if index == 0 {
					return fmt.Errorf("missing seed comparison")
				}
				var comparison = prefix[index-1]
				if err := p15CoveragePredicate(comparison, p15CoverageOutput(named[seed.name])); err != nil {
					return err
				}
				if err := p15CoverageNumber(comparison.then, 1); err != nil {
					return err
				}
				if err := p15CoverageNumber(comparison.otherwise, 0); err != nil {
					return err
				}
				if err := p15CoverageAttachmentError(node.params()["WFInput"], p15CoverageOutput(comparison.end)); err != nil {
					return err
				}
			}
		}
		if stores != 1 {
			return fmt.Errorf("missing or duplicated dynamic Boolean binding")
		}
	}
	if prefix[len(prefix)-2].kind() != "gettext" || prefix[len(prefix)-2].params()["WFTextActionText"] != "START" ||
		prefix[len(prefix)-1].kind() != "setvariable" || prefix[len(prefix)-1].params()["WFVariableName"] != "trace" {
		return fmt.Errorf("trace seed is not the explicit public sentinel")
	}
	if err := p15CoverageAttachmentError(prefix[len(prefix)-1].params()["WFInput"], p15CoverageOutput(prefix[len(prefix)-2].params())); err != nil {
		return err
	}
	var outputs = map[string]map[string]any{}
	for index, scenario := range scenarios {
		var start, end = markers[index], markers[index+1]
		if roots[start].params()["WFCommentActionText"] != "P15_F01_CASE_"+scenario.name {
			return fmt.Errorf("case marker changed")
		}
		var segment = roots[start+1 : end]
		var tail, output, err = p15CoverageMatch(scenario.expr, segment)
		if err != nil || len(tail) != 0 || len(segment) == 0 || segment[len(segment)-1].end["CustomOutputName"] != scenario.name {
			return fmt.Errorf("case %s lost its complete logical tree: %v", scenario.name, err)
		}
		outputs[scenario.name] = output
	}
	var start = markers[len(markers)-1]
	if roots[start].params()["WFCommentActionText"] != "P15_F01_RESULTS" {
		return fmt.Errorf("missing results boundary")
	}
	var resultNodes = roots[start+1:]
	if len(resultNodes) != len(scenarios)+3 {
		return fmt.Errorf("missing or duplicated observation nodes")
	}
	var resultText string
	var resultRefs = map[string]map[string]any{}
	var offset int
	for index, scenario := range scenarios {
		var node = resultNodes[index]
		if err := p15CoveragePredicate(node, outputs[scenario.name]); err != nil {
			return err
		}
		if len(node.then) != 1 || len(node.otherwise) != 1 || node.then[0].kind() != "gettext" || node.otherwise[0].kind() != "gettext" ||
			node.then[0].params()["WFTextActionText"] != "T" || node.otherwise[0].params()["WFTextActionText"] != "F" ||
			node.end["CustomOutputName"] != "result_"+scenario.name {
			return fmt.Errorf("result flags no longer observe actual Boolean values")
		}
		if index > 0 {
			resultText += "|"
			offset++
		}
		resultText += scenario.name + "="
		offset += len(scenario.name) + 1 // All keys are fixed ASCII; offsets are independent of the encoder.
		resultRefs[fmt.Sprintf("{%d, 1}", offset)] = p15CoverageOutput(node.end)
		resultText += "\uFFFC"
		offset++
	}
	if resultNodes[len(scenarios)].kind() != "gettext" || resultNodes[len(scenarios)+1].kind() != "gettext" ||
		resultNodes[len(scenarios)+2].kind() != "setclipboard" {
		return fmt.Errorf("final observation action sequence changed")
	}
	if err := p15CoverageTextError(named["results"]["WFTextActionText"], resultText, resultRefs); err != nil {
		return err
	}
	if err := p15CoverageTextError(named["finalResult"]["WFTextActionText"],
		"CHERRI_S01_F01_COVERAGE_OK\nS01_OUTPUT=\uFFFC\nF01_TRACE=\uFFFC\nF01_RESULTS=\uFFFC",
		map[string]map[string]any{
			"{38, 1}": p15CoverageOutput(named["unicodeOutput"]),
			"{50, 1}": {"Type": "Variable", "VariableName": "trace"},
			"{64, 1}": p15CoverageOutput(named["results"]),
		}); err != nil {
		return err
	}
	if len(resultNodes[len(resultNodes)-1].params()) != 1 {
		return fmt.Errorf("clipboard observation parameters changed")
	}
	return p15CoverageAttachmentError(resultNodes[len(resultNodes)-1].params()["WFInput"], p15CoverageOutput(named["finalResult"]))
}

func p15CoverageLoad(t *testing.T) ([]byte, []byte) {
	t.Helper()
	var source, sourceErr = os.ReadFile(p15CoveragePath)
	var contract, contractErr = os.ReadFile("tests/runtime_poc/CherriS01F01CoveragePOC.expected.json")
	if sourceErr != nil || contractErr != nil {
		t.Fatalf("read public coverage source/contract: %v %v", sourceErr, contractErr)
	}
	if err := p15CoverageContractError(source, contract); err != nil {
		t.Fatal(err)
	}
	return source, contract
}

func TestP15S01F01NativeCoverageFixture(t *testing.T) {
	var source, _ = p15CoverageLoad(t)
	var actions = p15CompiledActions(t, string(source))
	if err := p15CoverageGraphError(actions); err != nil {
		t.Fatal(err)
	}
	t.Logf("reviewed public source %s; actual compiled native graph %d actions; exact runtime clipboard remains a separate Apple execution requirement", p15CoverageSourceSHA, len(actions))
}

func TestP15S01F01CoverageContractRejectsDrift(t *testing.T) {
	var source, encoded = p15CoverageLoad(t)
	for _, fault := range []string{"source", "expected_text", "expected_trace", "trailing_newline", "extra_requirement", "claimed_execution", "source_hash", "expected_hash"} {
		t.Run(fault, func(t *testing.T) {
			var actual map[string]any
			if err := json.Unmarshal(encoded, &actual); err != nil {
				t.Fatal(err)
			}
			var changedSource = append([]byte{}, source...)
			switch fault {
			case "source":
				changedSource = append(changedSource, []byte("\n// altered source\n")...)
			case "expected_text":
				actual["expected_fields"].(map[string]any)["S01_OUTPUT"] = "hardcoded wrong text"
			case "expected_trace":
				actual["expected_fields"].(map[string]any)["F01_TRACE"] = "START"
			case "trailing_newline":
				actual["expected_clipboard"] = actual["expected_clipboard"].(string) + "\n"
			case "extra_requirement":
				actual["requirements"] = []any{"S01", "F01", "RP19"}
			case "claimed_execution":
				actual["runtime_status"] = "PASS"
			case "source_hash":
				actual["source_sha256"] = strings.Repeat("0", 64)
			case "expected_hash":
				actual["expected_clipboard_sha256"] = strings.Repeat("0", 64)
			}
			var changed, err = json.Marshal(actual)
			if err != nil {
				t.Fatal(err)
			}
			if p15CoverageContractError(changedSource, changed) == nil {
				t.Fatal("drifted or self-crediting public contract accepted")
			}
		})
	}
}

func TestP15S01F01CoverageGraphRejectsCausalMutations(t *testing.T) {
	var source, _ = p15CoverageLoad(t)
	var actions = p15CompiledActions(t, string(source))
	for _, fault := range []string{"rune_offset", "normalized_combining", "wrong_unicode_source", "hardcoded_unicode", "eager_effect", "wrong_bool_test", "fractional_bool", "missing_native_wrapper", "false_or_dropped", "nested_branch_swap", "crosswired_store", "hidden_trace", "hardcoded_results", "wrong_clipboard_source", "external_dependency"} {
		t.Run(fault, func(t *testing.T) {
			var broken = p15C01Clone(t, actions)
			var changed bool
			for index, action := range broken {
				var p = p15Param(action)
				var name, _ = p["CustomOutputName"].(string)
				var text, refs, token = p15BindingTokenText(p["WFTextActionText"])
				switch {
				case fault == "rune_offset" && name == "unicodeFirst":
					refs["{13, 1}"] = refs["{15, 1}"]
					delete(refs, "{15, 1}")
					changed = true
				case fault == "normalized_combining" && name == "unicodeFirst":
					p["WFTextActionText"].(map[string]any)["Value"].(map[string]any)["string"] = strings.ReplaceAll(text, "é", "é")
					changed = true
				case fault == "wrong_unicode_source" && name == "unicodeFirst":
					refs["{15, 1}"], refs["{19, 1}"] = refs["{19, 1}"], refs["{15, 1}"]
					changed = true
				case fault == "hardcoded_unicode" && name == "unicodeOutput":
					p["WFTextActionText"] = p15CoverageUnicode
					changed = true
				case fault == "eager_effect" && token && text == "\uFFFC|BAD_AND":
					broken = append([]map[string]any{action, broken[index+1]}, append(broken[:index], broken[index+2:]...)...)
					changed = true
				case fault == "wrong_bool_test" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0:
					p["WFCondition"] = 100
					changed = true
				case fault == "fractional_bool" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(p["WFControlFlowMode"]) == 0:
					p["WFNumberValue"] = 1.5
					changed = true
				case fault == "missing_native_wrapper" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setclipboard":
					p["WFInput"] = p15LocalAttachment(p["WFInput"])
					changed = true
				case fault == "false_or_dropped" && token && text == "\uFFFC|OR_FALSE":
					broken = append(broken[:index], broken[index+1:]...)
					changed = true
				case fault == "nested_branch_swap" && name == "nested_and":
					var group = p["GroupingIdentifier"]
					var start, alternate = -1, -1
					for candidateIndex, candidate := range broken {
						var q = p15Param(candidate)
						if q["GroupingIdentifier"] == group {
							if p15Int(q["WFControlFlowMode"]) == 0 {
								start = candidateIndex
							} else if p15Int(q["WFControlFlowMode"]) == 1 {
								alternate = candidateIndex
							}
						}
					}
					if start < 0 || alternate <= start || alternate >= index {
						t.Fatal("nested branch mutation target is incomplete")
					}
					var swapped = append([]map[string]any{}, broken[:start+1]...)
					swapped = append(swapped, broken[alternate+1:index]...)
					swapped = append(swapped, broken[alternate])
					swapped = append(swapped, broken[start+1:alternate]...)
					swapped = append(swapped, broken[index:]...)
					broken, changed = swapped, true
				case fault == "crosswired_store" && token && text == "\uFFFC|MIX_LEFT":
					p15LocalAttachment(p15Param(broken[index+1])["WFInput"])["OutputUUID"] = "UNRELATED"
					changed = true
				case fault == "hidden_trace" && name == "finalResult":
					refs["{50, 1}"] = map[string]any{"Type": "Variable", "VariableName": "truth"}
					changed = true
				case fault == "hardcoded_results" && name == "results":
					p["WFTextActionText"] = p15CoverageResults
					changed = true
				case fault == "wrong_clipboard_source" && action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setclipboard":
					p["WFInput"] = "CHERRI_S01_F01_COVERAGE_OK"
					changed = true
				}
				if changed {
					break
				}
			}
			if fault == "external_dependency" {
				broken = append(broken, map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.runworkflow", "WFWorkflowActionParameters": map[string]any{"WFWorkflowName": "MISSING"}})
				changed = true
			}
			if !changed {
				t.Fatal("actual mutation target not emitted")
			}
			if err := p15CoverageGraphError(broken); err == nil {
				t.Fatal("causal native graph mutation accepted")
			}
		})
	}
}
