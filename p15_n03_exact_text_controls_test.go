package main

import (
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func p15N03ExactQuotedSource(value string) string {
	var out strings.Builder
	out.WriteByte('"')
	for _, r := range value {
		switch r {
		case '\\', '"':
			out.WriteByte('\\')
			out.WriteRune(r)
		case '\n':
			out.WriteString("\\n")
		case '\r':
			out.WriteString("\\r")
		case '\t':
			out.WriteString("\\t")
		default:
			out.WriteRune(r)
		}
	}
	out.WriteByte('"')
	return out.String()
}

// Go's independent regexp engine checks the quoted pattern language here;
// it is not a Shortcuts evaluator. Each row also compiles the real native
// route and checks original operands, exact transport and operator polarity.
func TestP15N03ExactTextQuotedTransport(t *testing.T) {
	cases := []struct {
		name, left, right string
		equal             bool
	}{
		{"same_case", "AbC", "AbC", true},
		{"different_case", "AbC", "abc", false},
		{"metacharacters", ".^$*+?()[]{}|", ".^$*+?()[]{}|", true},
		{"metacharacters_not_regex", "anything", ".*", false},
		{"embedded_quote_end", "a\\Eb", "a\\Eb", true},
		{"multiple_quote_ends", "\\E\\E", "\\E\\E", true},
		{"quote_end_not_option", "ABC", "\\E(?i)abc", false},
		{"quote_start", "\\Q", "\\Q", true},
		{"trailing_backslash", "tail\\", "tail\\", true},
		{"empty", "", "", true},
		{"empty_nonempty", "", "~", false},
		{"sentinel_literal", "~x", "~x", true},
		{"sentinel_no_collision", "x", "~x", false},
		{"newline", "line\nnext", "line\nnext", true},
		{"final_newline_distinct", "line\n", "line", false},
		{"carriage_return", "line\rnext", "line\rnext", true},
		{"crlf_distinct", "line\r\n", "line\n", false},
		{"unicode", "שלום🙂", "שלום🙂", true},
		{"combining_identical", "e\u0301", "e\u0301", true},
		{"normalization_distinct", "\u00e9", "e\u0301", false},
		{"literal_object_character", "\uFFFC🙂", "\uFFFC🙂", true},
	}
	for _, test := range cases {
		for _, operator := range []struct {
			name, text string
			code       int
		}{{"equal", "==", 4}, {"not_equal", "!=", 5}} {
			t.Run(test.name+"/"+operator.name, func(t *testing.T) {
				pattern := "\\A\\Q~" + strings.ReplaceAll(test.right, "\\E", "\\E\\\\E\\Q") + "\\E\\z"
				expression, err := regexp.Compile(pattern)
				if err != nil {
					t.Fatal(err)
				}
				match := expression.MatchString("~" + test.left)
				if match != test.equal || test.equal != (test.left == test.right) {
					t.Fatalf("quoted-language equality differs: got %v want %v", match, test.equal)
				}
				wantBoolean := test.equal
				if operator.code == 5 {
					wantBoolean = !wantBoolean
				}
				modelBoolean := match
				if operator.code == 5 {
					modelBoolean = !modelBoolean
				}
				if modelBoolean != wantBoolean {
					t.Fatal("authored operator truth table differs")
				}
				src := "let left=text(" + p15N03ExactQuotedSource(test.left) + ")\nlet right=text(" + p15N03ExactQuotedSource(test.right) + ")\nlet answer=left " + operator.text + " right\ngetVariable(answer)\n"
				actions := p15N03ExactCompile(t, src)
				if err := p15N03ExactSourcePair(actions, test.left, test.right, operator.code); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}

func TestP15N03ExactTextCausalGraphControls(t *testing.T) {
	src := "let left=text(\"AbC\")\nlet right=text(\"a\\\\E[.]\\\\\")\nlet answer=left!=right\ngetVariable(answer)\n"
	good := p15N03ExactCompile(t, src)
	if err := p15N03ExactSourcePair(good, "AbC", "a\\E[.]\\", 5); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"case_flag", "relative_anchor", "quote_end", "quote_regex_mode", "quote_case_mode", "left_edge", "right_sentinel", "count_edge", "count_units", "numeric_rhs", "invented_case_field", "operator_code", "boolean_yields", "left_operand", "right_operand", "final_result", "group_identity"} {
		t.Run(fault, func(t *testing.T) {
			raw, err := json.Marshal(good)
			if err != nil {
				t.Fatal(err)
			}
			var actions []map[string]any
			if err := json.Unmarshal(raw, &actions); err != nil {
				t.Fatal(err)
			}
			var match, quote, count, right, guard map[string]any
			guardIndex := -1
			for _, action := range actions {
				params := p15N03ExactParams(action)
				switch params["CustomOutputName"] {
				case "_n03_exact_match":
					match = params
				case "_n03_exact_quote":
					quote = params
				case "_n03_exact_count":
					count = params
				case "_n03_exact_right":
					right = params
				}
			}
			if match == nil || quote == nil || count == nil || right == nil {
				t.Fatal("actual pipeline control fixture missing")
			}
			for index, action := range actions {
				params := p15N03ExactParams(action)
				edge, _ := p15N03ExactToken(params["WFInput"])
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && p15Int(params["WFControlFlowMode"]) == 0 && edge == count["UUID"] {
					if guardIndex >= 0 {
						t.Fatal("duplicate actual count guard")
					}
					guard = params
					guardIndex = index
				}
			}
			if guard == nil {
				t.Fatal("actual count guard missing")
			}
			switch fault {
			case "case_flag":
				match["WFMatchTextCaseSensitive"] = false
			case "relative_anchor":
				match["WFMatchTextPattern"].(map[string]any)["Value"].(map[string]any)["string"] = "^\\Q\uFFFC\\E$"
			case "quote_end":
				quote["WFReplaceTextReplace"] = "\\E"
			case "quote_regex_mode":
				quote["WFReplaceTextRegularExpression"] = true
			case "quote_case_mode":
				quote["WFReplaceTextCaseSensitive"] = false
			case "left_edge":
				match["text"] = quote["WFInput"]
			case "right_sentinel":
				right["WFTextActionText"] = ""
			case "count_edge":
				count["Input"] = quote["WFInput"]
			case "count_units":
				count["WFCountType"] = "Characters"
			case "numeric_rhs":
				guard["WFNumberValue"] = 0
			case "invented_case_field":
				guard["WFConditionalCaseSensitive"] = true
			case "operator_code":
				guard["WFCondition"] = 4
			case "boolean_yields":
				p15N03ExactParams(actions[guardIndex+1])["WFNumberActionNumber"] = 0
			case "left_operand":
				ref, _, err := p15N03ExactNamedReference(actions, "right")
				if err != nil {
					t.Fatal(err)
				}
				_, index, err := p15N03ExactNamedReference(actions, "_n03_exact_left")
				if err != nil {
					t.Fatal(err)
				}
				p15N03ExactParams(actions[index])["WFTextActionText"] = p15N03AffixedTokenString("~", ref, "")
			case "right_operand":
				ref, _, err := p15N03ExactNamedReference(actions, "left")
				if err != nil {
					t.Fatal(err)
				}
				right["WFTextActionText"] = p15N03AffixedTokenString("~", ref, "")
			case "final_result":
				ref, _, err := p15N03ExactNamedReference(actions, "left")
				if err != nil {
					t.Fatal(err)
				}
				for _, action := range actions {
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.getvariable" {
						p15N03ExactParams(action)["WFVariable"] = p15N03Attachment(ref)
					}
				}
			case "group_identity":
				guard["GroupingIdentifier"] = "N03_DIFFERENT_GROUP"
			}
			if err := p15N03ExactSourcePair(actions, "AbC", "a\\E[.]\\", 5); err == nil {
				t.Fatal("accepted exact-comparison corruption", fault)
			}
		})
	}
}

func TestP15N03ExactTextEvaluationAndPreviewBoundaries(t *testing.T) {
	t.Run("operands_once_in_source_order", func(t *testing.T) {
		actions := p15N03ExactCompile(t, "let answer=text(\"N03_LEFT_CALL\")==text(\"N03_RIGHT_CALL\")\ngetVariable(answer)\n")
		left, right, snapshot := -1, -1, -1
		for index, action := range actions {
			params := p15N03ExactParams(action)
			if params["WFTextActionText"] == "N03_LEFT_CALL" {
				if left >= 0 {
					t.Fatal("left source call duplicated")
				}
				left = index
			}
			if params["WFTextActionText"] == "N03_RIGHT_CALL" {
				if right >= 0 {
					t.Fatal("right source call duplicated")
				}
				right = index
			}
			if params["CustomOutputName"] == "_n03_exact_left" {
				snapshot = index
			}
		}
		if left < 0 || snapshot <= left || right <= snapshot {
			t.Fatal("left value was not captured before right source effect")
		}
		leftID, _ := p15N03ExactParams(actions[left])["UUID"].(string)
		edge, err := p15N03ExactSnapshotEdge(p15N03ExactParams(actions[snapshot])["WFTextActionText"])
		if err != nil || leftID == "" || edge != leftID {
			t.Fatal("left snapshot lost actual source call edge")
		}
		_, rhs, err := p15N03ExactNamedReference(actions, "_n03_exact_right")
		if err != nil {
			t.Fatal(err)
		}
		rightID, _ := p15N03ExactParams(actions[right])["UUID"].(string)
		edge, err = p15N03ExactSnapshotEdge(p15N03ExactParams(actions[rhs])["WFTextActionText"])
		if err != nil || rightID == "" || edge != rightID {
			t.Fatal("right snapshot lost actual source call edge")
		}
		if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("mutable_left_before_rhs_assignment", func(t *testing.T) {
		src := "var left:Text=\"BEFORE\"\nlet answer=left==(if true { left=\"AFTER\"; yield \"BEFORE\" } else { yield \"OTHER\" })\ngetVariable(answer)\n"
		actions := p15N03ExactCompile(t, src)
		snapshot, update := -1, -1
		for index, action := range actions {
			params := p15N03ExactParams(action)
			if params["CustomOutputName"] == "_n03_exact_left" {
				snapshot = index
			}
			if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.setvariable" && params["WFVariableName"] == "left" {
				update = index
			}
		}
		if snapshot < 0 || update <= snapshot {
			t.Fatal("mutable left was read after RHS assignment")
		}
		want := p15N03AffixedTokenString("~", map[string]any{"Type": "Variable", "VariableName": "left"}, "")
		if !reflect.DeepEqual(p15N03ExactParams(actions[snapshot])["WFTextActionText"], want) {
			t.Fatal("mutable left snapshot identity changed")
		}
		if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
			t.Fatal(err)
		}
	})
	for _, test := range []struct {
		name, source string
		rhsMode      int
	}{
		{"lazy_and", "let flag=number(1)==1\nlet left=text(\"L\")\nlet answer=flag && left==text(\"N03_LAZY_RHS\")\ngetVariable(answer)\n", 0},
		{"lazy_or", "let flag=number(1)==1\nlet left=text(\"L\")\nlet answer=flag || left==text(\"N03_LAZY_RHS\")\ngetVariable(answer)\n", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, test.source)
			type frame struct {
				group string
				mode  int
			}
			var stack []frame
			seen := 0
			for _, action := range actions {
				params := p15N03ExactParams(action)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" {
					group, _ := params["GroupingIdentifier"].(string)
					switch p15Int(params["WFControlFlowMode"]) {
					case 0:
						stack = append(stack, frame{group, 0})
					case 1:
						if len(stack) == 0 || stack[len(stack)-1].group != group {
							t.Fatal("unmatched Else")
						}
						stack[len(stack)-1].mode = 1
					case 2:
						if len(stack) == 0 || stack[len(stack)-1].group != group {
							t.Fatal("unmatched End")
						}
						stack = stack[:len(stack)-1]
					}
				}
				if params["WFTextActionText"] == "N03_LAZY_RHS" {
					seen++
					if len(stack) == 0 || stack[len(stack)-1].mode != test.rhsMode {
						t.Fatal("RHS escaped its lazy branch")
					}
				}
			}
			if seen != 1 || len(stack) != 0 {
				t.Fatal("RHS count/control scope differs", seen)
			}
			if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("helper_readonly_authored_wait_still_mapped", func(t *testing.T) {
		const src = "// preserve\nlet left=text(\"L\")\nlet right=text(\"R\")\nlet answer=left==right\nwait(2) // target\n"
		preview, actions := rp41PreviewControlCompile(t, src, CompileOptions{})
		waitIndex, helpers := -1, 0
		for index, action := range actions {
			if action.WFWorkflowActionIdentifier == "is.workflow.actions.delay" {
				waitIndex = index
			}
			name, _ := action.WFWorkflowActionParameters["CustomOutputName"].(string)
			if !strings.HasPrefix(name, "_n03_exact_") {
				continue
			}
			helpers++
			for _, entry := range preview.PreviewSourceMap.Entries {
				if entry.ActionIndex == index {
					t.Fatal("helper gained authored preview provenance")
				}
			}
			key := "WFTextActionText"
			switch action.WFWorkflowActionIdentifier {
			case "is.workflow.actions.text.replace":
				key = "WFReplaceTextFind"
			case "is.workflow.actions.text.match":
				key = "WFMatchTextPattern"
			case "is.workflow.actions.count":
				key = "Input"
			}
			updated, err := rp41ApplyPreviewEdit(t, src, preview, index, key, "\"changed\"")
			if err == nil || !strings.Contains(err.Error(), "read-only") || updated != src {
				t.Fatal("generated helper edit was not explained/read-only")
			}
		}
		if waitIndex < 0 || helpers != 5 {
			t.Fatal("helper/authored target inventory differs", helpers, waitIndex)
		}
		updated, err := rp41ApplyPreviewEdit(t, src, preview, waitIndex, "WFDelayTime", "9")
		if err != nil || updated != strings.Replace(src, "wait(2)", "wait(9)", 1) {
			t.Fatal("authored Wait no longer maps exactly", err)
		}
	})
}

func TestP15N03ExactTextNativeCompatibilityModes(t *testing.T) {
	for _, mode := range []string{"modern", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			text := "let subject=text(\"AbC\")\nif " + mode + ": subject==\"abc\" { text(\"YES\") }\n"
			parser := syntax.NewParser(source.NewFile("n03-modes", "n03.cherri", 1, text))
			program := parser.ParseProgram()
			if len(parser.Errors()) != 0 {
				t.Fatal(parser.Errors())
			}
			condition, okay := program.Statements[1].(*syntax.IfStmt)
			if !okay || condition.Modern != (mode == "modern") || condition.Legacy != (mode == "legacy") {
				t.Fatal("explicit compatibility AST differs")
			}
			actions := p15N03ExactCompile(t, syntax.Format(program))
			modes, err := p15N03MigrationModes(actions)
			var native any
			if mode == "legacy" {
				native = true
			}
			if err != nil || !reflect.DeepEqual(modes, []any{[]any{4, native}}) {
				t.Fatal("explicit native mode changed", modes, err)
			}
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
					t.Fatal("explicit native comparison was rewritten")
				}
			}
		})
	}
	t.Run("unmarked_nested_body_stays_exact", func(t *testing.T) {
		actions := p15N03ExactCompile(t, "let subject=text(\"AbC\")\nif modern: subject==\"AbC\" { if subject==\"abc\" { text(\"YES\") } }\n")
		if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("function_body_has_own_authored_intent", func(t *testing.T) {
		actions := p15N03ExactCompile(t, "function equal(a:Text,b:Text)->Bool { return a==b }\nif modern: equal(text(\"AbC\"),text(\"abc\")) { text(\"YES\") }\n")
		if err := p15N03ExactPrimitiveError(actions, 1); err != nil {
			t.Fatal(err)
		}
	})
	for _, test := range []struct{ name, source string }{
		{"numeric", "let left=number(2)\nlet right=number(3)\nif left!=right { text(\"YES\") }\n"},
		{"any_content", "let left:AnyContent=ShortcutInput\nif left==\"abc\" { text(\"YES\") }\n"},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, test.source)
			for _, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.text.match" {
					t.Fatal("non-Text comparison was rewritten")
				}
			}
			explicit := p15N03ExactCompile(t, strings.Replace(test.source, "if ", "if legacy: ", 1))
			first, second := make([]any, len(actions)), make([]any, len(explicit))
			for index, action := range actions {
				first[index] = action
			}
			for index, action := range explicit {
				second[index] = action
			}
			comparison := shortcutcompare.CompareDocumentsStrict(map[string]any{"WFWorkflowActions": first}, map[string]any{"WFWorkflowActions": second})
			if !comparison.Equal {
				t.Fatal("non-Text graph differs from preserved native mode", comparison.Differences)
			}
		})
	}
}

func p15N03ExactNamedReference(actions []map[string]any, name string) (map[string]any, int, error) {
	var result map[string]any
	index := -1
	for at, action := range actions {
		params := p15N03ExactParams(action)
		if params["CustomOutputName"] != name {
			continue
		}
		if index >= 0 {
			return nil, -1, fmt.Errorf("duplicate source/output identity %s", name)
		}
		id, okay := params["UUID"].(string)
		if !okay || id == "" {
			return nil, -1, fmt.Errorf("missing output identity %s", name)
		}
		result = map[string]any{"Type": "ActionOutput", "OutputUUID": id, "OutputName": name}
		index = at
	}
	if index < 0 {
		return nil, -1, fmt.Errorf("missing source/output %s", name)
	}
	return result, index, nil
}

func p15N03ExactSequence(actions []map[string]any, left map[string]any, right any, code int, resultName string) (map[string]any, int, error) {
	if err := p15N03ExactControlGroupsError(actions); err != nil {
		return nil, 0, err
	}
	_, start, err := p15N03ExactNamedReference(actions, "_n03_exact_left")
	if err != nil {
		return nil, 0, err
	}
	cursor := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: start, groups: map[string]bool{}}
	result := cursor.exactValueComparison(resultName, left, float64(code), right)
	return result, cursor.index, cursor.err
}

func p15N03ExactSourcePair(actions []map[string]any, leftValue, rightValue string, code int) error {
	left, li, err := p15N03ExactNamedReference(actions, "left")
	if err != nil {
		return err
	}
	right, ri, err := p15N03ExactNamedReference(actions, "right")
	if err != nil {
		return err
	}
	if li >= ri || actions[li]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
		actions[ri]["WFWorkflowActionIdentifier"] != "is.workflow.actions.gettext" ||
		p15N03ExactParams(actions[li])["WFTextActionText"] != leftValue ||
		p15N03ExactParams(actions[ri])["WFTextActionText"] != rightValue {
		return fmt.Errorf("actual source Text producer identities/payloads/order differ")
	}
	result, end, err := p15N03ExactSequence(actions, left, right, code, "answer")
	if err != nil {
		return err
	}
	if err := p15N03ExactResultConsumer(actions, end, result); err != nil {
		return err
	}
	return p15N03ExactPrimitiveError(actions, 1)
}

func p15N03ExactResultConsumer(actions []map[string]any, start int, result map[string]any) error {
	count := 0
	for _, action := range actions[start:] {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
			continue
		}
		params := p15N03ExactParams(action)
		if reflect.DeepEqual(params["WFVariable"], p15N03Attachment(result)) {
			count++
		}
	}
	if count != 1 {
		return fmt.Errorf("actual source getVariable did not consume the exact Boolean result once: %d", count)
	}
	return nil
}

// This fixture has two distinct outputs named "flag": the authored number
// producer and the Boolean comparison result. Validate the complete original
// numeric comparison and follow its result UUID into the actual logical guard;
// a display name alone is never its identity.
func p15N03ExactLogicalFlag(actions []map[string]any) (map[string]any, int, error) {
	if err := p15DiagnosticReferencesValid(p15DiagnosticGraph(actions)); err != nil {
		return nil, -1, err
	}
	if err := p15N03ExactControlGroupsError(actions); err != nil {
		return nil, -1, err
	}
	start := -1
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.number" &&
			p15N03ExactParams(action)["CustomOutputName"] == "flag" {
			if start >= 0 {
				return nil, -1, fmt.Errorf("duplicate authored flag number producer")
			}
			start = index
		}
	}
	if start < 0 {
		return nil, -1, fmt.Errorf("authored flag number producer missing")
	}
	// The authored Number action uses a plist integer; retained JSON uses a
	// floating representation of the same exact integer. Admit only this
	// identified producer field, and keep every other field/type check strict.
	sourceNumber := p15N03ExactParams(actions[start])["WFNumberActionNumber"]
	if value, ok := p15N03ControlInteger(sourceNumber); !ok || value != 1 {
		return nil, -1, fmt.Errorf("authored flag number producer is not the exact integer 1")
	}
	c := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: start, groups: map[string]bool{}}
	input := c.take("number", "flag", map[string]any{"WFNumberActionNumber": sourceNumber}, true)
	group := c.start(input, 4, "WFNumberValue", float64(1))
	c.take("number", "", map[string]any{"WFNumberActionNumber": float64(1)}, false)
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(1)}, false)
	c.take("number", "", map[string]any{"WFNumberActionNumber": float64(0)}, false)
	result := c.take("conditional", "flag", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(2)}, true)
	if c.err != nil {
		return nil, -1, c.err
	}
	if input["OutputUUID"] == result["OutputUUID"] {
		return nil, -1, fmt.Errorf("flag number and Boolean result share an output identity")
	}
	guard := -1
	for index, action := range actions {
		params := p15N03ExactParams(action)
		edge, err := p15N03ExactToken(params["WFInput"])
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" &&
			p15Int(params["WFControlFlowMode"]) == 0 && err == nil && edge == result["OutputUUID"] {
			if guard >= 0 || index < c.index {
				return nil, -1, fmt.Errorf("flag result logical consumer is duplicated or out of source order")
			}
			guard = index
		}
	}
	if guard < 0 {
		return nil, -1, fmt.Errorf("actual logical guard does not consume the Boolean flag result")
	}
	consumer := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: guard, groups: map[string]bool{}}
	consumer.start(result, 4, "WFNumberValue", float64(1))
	if consumer.err != nil {
		return nil, -1, consumer.err
	}
	return result, guard, nil
}

// The seven original RED children remain unchanged. These separate controls
// bind each authored operator, the complete count->Boolean sequence, outer
// negation, the actual final consumer, and both short-circuit branch results.
func TestP15N03ExactTextOperatorAndBranchBindings(t *testing.T) {
	const operands = "let left=text(\"AbC\")\nlet right=text(\"abc\")\n"
	for _, test := range []struct {
		name, body      string
		code            int
		negated, direct bool
	}{
		{"value_equal", "let answer=left==right\ngetVariable(answer)\n", 4, false, false},
		{"value_not_equal", "let answer=left!=right\ngetVariable(answer)\n", 5, false, false},
		{"direct_equal", "if left==right { text(\"N03_TRUE\") } else { text(\"N03_FALSE\") }\n", 4, false, true},
		{"direct_not_equal", "if left!=right { text(\"N03_TRUE\") } else { text(\"N03_FALSE\") }\n", 5, false, true},
		{"negated_equal", "let answer=!(left==right)\ngetVariable(answer)\n", 4, true, false},
		{"negated_not_equal", "let answer=!(left!=right)\ngetVariable(answer)\n", 5, true, false},
		{"direct_negated_equal", "if !(left==right) { text(\"N03_TRUE\") } else { text(\"N03_FALSE\") }\n", 4, true, true},
		{"direct_negated_not_equal", "if !(left!=right) { text(\"N03_TRUE\") } else { text(\"N03_FALSE\") }\n", 5, true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, operands+test.body)
			left, _, err := p15N03ExactNamedReference(actions, "left")
			if err != nil {
				t.Fatal(err)
			}
			right, _, err := p15N03ExactNamedReference(actions, "right")
			if err != nil {
				t.Fatal(err)
			}
			resultName := "answer"
			if test.negated || test.direct {
				resultName = ""
			}
			result, end, err := p15N03ExactSequence(actions, left, right, test.code, resultName)
			if err != nil {
				t.Fatal(err)
			}
			c := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: end, groups: map[string]bool{}}
			if test.negated {
				group := c.start(result, 4, "WFNumberValue", float64(1))
				c.take("number", "", map[string]any{"WFNumberActionNumber": float64(0)}, false)
				c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(1)}, false)
				c.take("number", "", map[string]any{"WFNumberActionNumber": float64(1)}, false)
				name := "answer"
				if test.direct {
					name = ""
				}
				result = c.take("conditional", name, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(2)}, true)
			}
			if test.direct {
				group := c.start(result, 4, "WFNumberValue", float64(1))
				if c.err != nil {
					t.Fatal(c.err)
				}
				branch, closed := 0, false
				seen := map[string]int{}
				for _, action := range actions[c.index:] {
					params := p15N03ExactParams(action)
					if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.conditional" && params["GroupingIdentifier"] == group {
						switch p15Int(params["WFControlFlowMode"]) {
						case 1:
							branch = 1
						case 2:
							closed = true
						}
					}
					if literal, okay := params["WFTextActionText"].(string); okay && (literal == "N03_TRUE" || literal == "N03_FALSE") {
						if closed || (literal == "N03_TRUE") != (branch == 0) {
							t.Fatal("authored true/false branch changed")
						}
						seen[literal]++
					}
				}
				if !closed || seen["N03_TRUE"] != 1 || seen["N03_FALSE"] != 1 {
					t.Fatal("authored branch/result inventory changed", seen)
				}
			} else if err := p15N03ExactResultConsumer(actions, c.index, result); err != nil {
				t.Fatal(err)
			}
			if c.err != nil {
				t.Fatal(c.err)
			}
		})
	}
	for _, logical := range []string{"&&", "||"} {
		for _, op := range []struct {
			text string
			code int
		}{{"==", 4}, {"!=", 5}} {
			name := "and"
			if logical == "||" {
				name = "or"
			}
			if op.code == 5 {
				name += "_not_equal"
			} else {
				name += "_equal"
			}
			t.Run(name, func(t *testing.T) {
				actions := p15N03ExactCompile(t, "let flag=number(1)==1\n"+operands+"let answer=flag "+logical+" (left"+op.text+"right)\ngetVariable(answer)\n")
				flag, start, err := p15N03ExactLogicalFlag(actions)
				if err != nil {
					t.Fatal(err)
				}
				left, _, err := p15N03ExactNamedReference(actions, "left")
				if err != nil {
					t.Fatal(err)
				}
				right, _, err := p15N03ExactNamedReference(actions, "right")
				if err != nil {
					t.Fatal(err)
				}
				c := p15N03Cursor{actions: p15DiagnosticGraph(actions), index: start, groups: map[string]bool{}}
				group := c.start(flag, 4, "WFNumberValue", float64(1))
				if logical == "||" {
					c.take("number", "", map[string]any{"WFNumberActionNumber": float64(1)}, false)
					c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(1)}, false)
				}
				c.exactValueComparison("", left, float64(op.code), right)
				if logical == "&&" {
					c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(1)}, false)
					c.take("number", "", map[string]any{"WFNumberActionNumber": float64(0)}, false)
				}
				result := c.take("conditional", "answer", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(2)}, true)
				if c.err != nil {
					t.Fatal(c.err)
				}
				if err := p15N03ExactResultConsumer(actions, c.index, result); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	for _, op := range []struct {
		text string
		code int
	}{{"==", 4}, {"!=", 5}} {
		t.Run(fmt.Sprintf("literal_rhs_%d", op.code), func(t *testing.T) {
			actions := p15N03ExactCompile(t, "let left=text(\"a\\\\Eb\")\nlet answer=left"+op.text+"\"a\\\\Eb\"\ngetVariable(answer)\n")
			left, _, err := p15N03ExactNamedReference(actions, "left")
			if err != nil {
				t.Fatal(err)
			}
			result, end, err := p15N03ExactSequence(actions, left, "a\\Eb", op.code, "answer")
			if err != nil {
				t.Fatal(err)
			}
			if err := p15N03ExactResultConsumer(actions, end, result); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func p15N03ExactControlGroupsError(actions []map[string]any) error {
	seen := map[string]bool{}
	var stack []string
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
			continue
		}
		params := p15N03ExactParams(action)
		id, okay := params["GroupingIdentifier"].(string)
		if !okay || id == "" {
			return fmt.Errorf("conditional %d has no group identity", index)
		}
		switch p15Int(params["WFControlFlowMode"]) {
		case 0:
			if seen[id] {
				return fmt.Errorf("conditional group reused")
			}
			seen[id] = true
			stack = append(stack, id)
		case 1:
			if len(stack) == 0 || stack[len(stack)-1] != id {
				return fmt.Errorf("Else escaped its exact group")
			}
		case 2:
			if len(stack) == 0 || stack[len(stack)-1] != id {
				return fmt.Errorf("End escaped its exact group")
			}
			stack = stack[:len(stack)-1]
		default:
			return fmt.Errorf("invalid conditional mode")
		}
	}
	if len(stack) != 0 {
		return fmt.Errorf("unclosed conditional group")
	}
	return nil
}

func p15N03ExactSnapshotEdge(payload any) (string, error) {
	outer, okay := payload.(map[string]any)
	if !okay || outer["WFSerializationType"] != "WFTextTokenString" {
		return "", fmt.Errorf("snapshot lost typed Text transport")
	}
	body, okay := outer["Value"].(map[string]any)
	if !okay || body["string"] != "~\uFFFC" {
		return "", fmt.Errorf("snapshot lost exact sentinel/reference content")
	}
	attachments, okay := body["attachmentsByRange"].(map[string]any)
	if !okay || len(attachments) != 1 {
		return "", fmt.Errorf("snapshot attachment inventory differs")
	}
	return p15N03ExactToken(attachments["{1, 1}"])
}

func p15N03ExactDetectorCursorError(actions []map[string]any, expectedDetectors int) error {
	seed, _, err := p15N03ExactNamedReference(actions, "seed")
	if err != nil {
		return err
	}
	var calls []int
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.text" {
			calls = append(calls, index)
		}
	}
	if len(calls) != expectedDetectors {
		return fmt.Errorf("all detector producers, including explicit duplicates: got %d want %d", len(calls), expectedDetectors)
	}
	first := calls[0]
	if first < 1 {
		return fmt.Errorf("omitted-input source detector has no predecessor")
	}
	if _, explicit := p15N03ExactParams(actions[first])["WFInput"]; explicit {
		return fmt.Errorf("source omitted-input detector became explicit")
	}
	previous := actions[first-1]
	if previous["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
		return fmt.Errorf("source cursor was not restored at actual detector boundary")
	}
	restored, err := p15N03ExactToken(p15N03ExactParams(previous)["WFVariable"])
	if err != nil || restored != seed["OutputUUID"] {
		return fmt.Errorf("detector Previous input is not the exact source seed")
	}
	last := calls[len(calls)-1]
	if expectedDetectors == 2 {
		firstID, _ := p15N03ExactParams(actions[first])["UUID"].(string)
		edge, err := p15N03ExactToken(p15N03ExactParams(actions[last])["WFInput"])
		if firstID == "" || err != nil || edge != firstID || last != first+1 {
			return fmt.Errorf("explicit wrapper no longer consumes the sole omitted-input detector")
		}
	}
	lastID, _ := p15N03ExactParams(actions[last])["UUID"].(string)
	_, rhs, err := p15N03ExactNamedReference(actions, "_n03_exact_right")
	if err != nil {
		return err
	}
	edge, err := p15N03ExactSnapshotEdge(p15N03ExactParams(actions[rhs])["WFTextActionText"])
	if lastID == "" || err != nil || edge != lastID || rhs <= last {
		return fmt.Errorf("RHS snapshot does not consume the actual detector result")
	}
	_, lhs, err := p15N03ExactNamedReference(actions, "_n03_exact_left")
	if err != nil {
		return err
	}
	wantLeft := p15N03AffixedTokenString("~", map[string]any{"Type": "Variable", "VariableName": "left"}, "")
	if !reflect.DeepEqual(p15N03ExactParams(actions[lhs])["WFTextActionText"], wantLeft) || lhs >= first {
		return fmt.Errorf("left mutable operand snapshot changed identity/order")
	}
	return p15N03ExactPrimitiveError(actions, 1)
}

func TestP15N03ExactTextOwnedSourceCursor(t *testing.T) {
	const declaration = "action previousText() -> Text {\nidentifier: \"is.workflow.actions.detect.text\"\n}\n"
	const prefix = "var left:Text=\"LEFT\"\nlet seed=text(\"N03_PREVIOUS_INPUT\")\n"
	var causal []map[string]any
	for _, test := range []struct {
		name, decl, rhs string
		detectors       int
	}{
		{"fstring_raw", "", `f"{native.action("is.workflow.actions.detect.text", {})}"`, 1},
		{"registered_explicit_wrapper", "", `getText(native.action("is.workflow.actions.detect.text", {}))`, 2},
		{"declared_omitted_input", declaration, "previousText()", 1},
	} {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, test.decl+prefix+"let answer=left=="+test.rhs+"\ngetVariable(answer)\n")
			if err := p15N03ExactDetectorCursorError(actions, test.detectors); err != nil {
				t.Fatal(err)
			}
			if test.name == "declared_omitted_input" {
				causal = actions
			}
		})
	}
	if causal == nil {
		t.Fatal("actual custom-action causal fixture did not compile")
	}
	for _, fault := range []string{"explicit_duplicate", "missing_restore", "restored_snapshot_instead_of_seed", "rhs_uses_seed", "left_literal_substitution"} {
		t.Run(fault, func(t *testing.T) {
			raw, err := json.Marshal(causal)
			if err != nil {
				t.Fatal(err)
			}
			var actions []map[string]any
			if err := json.Unmarshal(raw, &actions); err != nil {
				t.Fatal(err)
			}
			call := -1
			for index, action := range actions {
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.text" {
					call = index
				}
			}
			if call < 1 {
				t.Fatal("causal detector missing")
			}
			seed, _, err := p15N03ExactNamedReference(actions, "seed")
			if err != nil {
				t.Fatal(err)
			}
			left, li, err := p15N03ExactNamedReference(actions, "_n03_exact_left")
			if err != nil {
				t.Fatal(err)
			}
			_, ri, err := p15N03ExactNamedReference(actions, "_n03_exact_right")
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "explicit_duplicate":
				duplicate := map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.detect.text", "WFWorkflowActionParameters": map[string]any{"WFInput": p15N03Attachment(seed), "UUID": "N03_DUPLICATE_PUBLIC_CONTROL"}}
				actions = append(actions, duplicate)
			case "missing_restore":
				actions = append(actions[:call-1], actions[call:]...)
			case "restored_snapshot_instead_of_seed":
				p15N03ExactParams(actions[call-1])["WFVariable"] = p15N03Attachment(left)
			case "rhs_uses_seed":
				p15N03ExactParams(actions[ri])["WFTextActionText"] = p15N03AffixedTokenString("~", seed, "")
			case "left_literal_substitution":
				p15N03ExactParams(actions[li])["WFTextActionText"] = "~LEFT"
			}
			if err := p15N03ExactDetectorCursorError(actions, 1); err == nil {
				t.Fatal("accepted cursor/producer corruption", fault)
			}
		})
	}
	t.Run("declared_explicit_input_retained", func(t *testing.T) {
		decl := "action explicitText(value:AnyContent) -> Text {\nidentifier: \"is.workflow.actions.detect.text\"\nprimary: \"value\"\nbindings: {value: {key: \"WFInput\", codec: \"token\"}}\n}\n"
		actions := p15N03ExactCompile(t, decl+prefix+"let answer=left==explicitText(ShortcutInput)\ngetVariable(answer)\n")
		seen := 0
		for _, action := range actions {
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.detect.text" {
				continue
			}
			seen++
			want := p15N03Attachment(map[string]any{"Type": "ExtensionInput"})
			if !reflect.DeepEqual(p15N03ExactParams(action)["WFInput"], want) {
				t.Fatal("explicit custom input changed")
			}
		}
		if seen != 1 {
			t.Fatal("explicit custom call count changed", seen)
		}
	})
}

func TestP15N03ExactTextSnapshotEnrollment(t *testing.T) {
	makeSession := func(t *testing.T) *CanonicalBackendSession {
		t.Helper()
		s := &CanonicalBackendSession{}
		if err := s.EmitRawAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "SOURCE"}, "source", "", ""); err != nil {
			t.Fatal(err)
		}
		if err := s.EmitRawAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "~LEFT"}, "snapshot", "", ""); err != nil {
			t.Fatal(err)
		}
		return s
	}
	for _, fault := range []string{"owned", "wrong_uuid", "different_payload", "marker_only", "unknown_previous"} {
		t.Run(fault, func(t *testing.T) {
			s := makeSession(t)
			if fault == "marker_only" {
				if s.HasCurrentExactTextSnapshot() {
					t.Fatal("content marker enrolled an authored action")
				}
				_, found, err := s.CurrentLiteralCarrierInput()
				if err != nil || found {
					t.Fatal("unowned marker acquired a source cursor")
				}
				return
			}
			id, payload := "snapshot", "~LEFT"
			if fault == "wrong_uuid" {
				id = "other"
			}
			if fault == "different_payload" {
				payload = "~OTHER"
			}
			known := func(identifier string) bool {
				return identifier == "is.workflow.actions.gettext" && fault != "unknown_previous"
			}
			err := s.BindExactTextSnapshotInput(id, payload, "private-reference", "default", known)
			if fault == "wrong_uuid" || fault == "different_payload" {
				if err == nil || s.HasCurrentExactTextSnapshot() {
					t.Fatal("mismatched action/payload enrolled")
				}
				return
			}
			if err != nil || !s.HasCurrentExactTextSnapshot() {
				t.Fatal("actual owned snapshot failed enrollment", err)
			}
			ref, found, err := s.CurrentLiteralCarrierInput()
			if fault == "unknown_previous" {
				if err == nil || !found || !strings.Contains(err.Error(), "S01_IMPLICIT_INPUT_UNAVAILABLE") {
					t.Fatal("unknown previous output guessed", err)
				}
				return
			}
			if err != nil || !found || ref == nil {
				t.Fatal("owned source cursor unavailable", err)
			}
			actual, err := s.consumeNativeActionResult(ref.ProducerID)
			if err != nil || actual != "source" {
				t.Fatal("snapshot cursor does not resolve to actual canonical source producer", err)
			}
		})
	}
}
