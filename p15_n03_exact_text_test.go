package main

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"
)

// These are production unsigned-graph assertions, not an execution of Apple's
// regex engine. A RED result means the explicit exact-comparison route is absent;
// it does not infer case-insensitive runtime behavior from generic condition 4/5.
func p15N03ExactCompile(t *testing.T, source string) []map[string]any {
	t.Helper()
	actions := p15N03MigrationCompile(t, source, "v2")
	t.Logf("actual_unsigned_v2_source_sha256=%x native_actions=%d", sha256.Sum256([]byte(source)), len(actions))
	return actions
}

func p15N03ExactParams(action map[string]any) map[string]any {
	parameters, _ := action["WFWorkflowActionParameters"].(map[string]any)
	return parameters
}

func p15N03ExactToken(value any) (string, error) {
	item, okay := value.(map[string]any)
	if !okay {
		return "", fmt.Errorf("native reference is %T", value)
	}
	switch item["WFSerializationType"] {
	case "WFTextTokenAttachment":
		return p15N03ExactToken(item["Value"])
	case "WFTextTokenString":
		body, okay := item["Value"].(map[string]any)
		if !okay || body["string"] != "\uFFFC" {
			return "", fmt.Errorf("expected exactly one native text reference")
		}
		attachments, okay := body["attachmentsByRange"].(map[string]any)
		if !okay || len(attachments) != 1 {
			return "", fmt.Errorf("text reference attachment inventory differs")
		}
		return p15N03ExactToken(attachments["{0, 1}"])
	}
	if item["Type"] == "Variable" && item["Variable"] != nil {
		return p15N03ExactToken(item["Variable"])
	}
	if item["Type"] != "ActionOutput" {
		return "", fmt.Errorf("native edge is not an ActionOutput: %#v", item)
	}
	uuid, okay := item["OutputUUID"].(string)
	if !okay || uuid == "" {
		return "", fmt.Errorf("native edge has no output identity")
	}
	return uuid, nil
}

func p15N03ExactPattern(value any) (string, error) {
	outer, okay := value.(map[string]any)
	if !okay || outer["WFSerializationType"] != "WFTextTokenString" {
		return "", fmt.Errorf("pattern is not the explicit native text template")
	}
	body, okay := outer["Value"].(map[string]any)
	if !okay || body["string"] != "\\A\\Q\uFFFC\\E\\z" {
		return "", fmt.Errorf("pattern lacks absolute anchors and literal quotation: %#v", body)
	}
	attachments, okay := body["attachmentsByRange"].(map[string]any)
	if !okay || len(attachments) != 1 {
		return "", fmt.Errorf("pattern attachment inventory differs")
	}
	return p15N03ExactToken(attachments["{4, 1}"])
}

func p15N03ExactPrefixedText(value any) bool {
	switch text := value.(type) {
	case string:
		return strings.HasPrefix(text, "~")
	case map[string]any:
		body, okay := text["Value"].(map[string]any)
		actual, valid := body["string"].(string)
		return text["WFSerializationType"] == "WFTextTokenString" && okay && valid && strings.HasPrefix(actual, "~")
	}
	return false
}

func p15N03ExactPrimitiveError(actions []map[string]any, expected int) error {
	producers := map[string]int{}
	for index, action := range actions {
		if uuid, okay := p15N03ExactParams(action)["UUID"].(string); okay && uuid != "" {
			if _, duplicate := producers[uuid]; duplicate {
				return fmt.Errorf("duplicate producer identity %q", uuid)
			}
			producers[uuid] = index
		}
	}
	lookup := func(uuid string, before int, identifier string) (int, map[string]any, error) {
		index, okay := producers[uuid]
		if !okay || index >= before || actions[index]["WFWorkflowActionIdentifier"] != identifier {
			return 0, nil, fmt.Errorf("missing ordered %s producer for %q", identifier, uuid)
		}
		return index, p15N03ExactParams(actions[index]), nil
	}
	found := 0
	for index, action := range actions {
		if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.text.match" {
			continue
		}
		found++
		parameters := p15N03ExactParams(action)
		if parameters["WFMatchTextCaseSensitive"] != true {
			return fmt.Errorf("Match Text must explicitly request case-sensitive matching")
		}
		quotedID, err := p15N03ExactPattern(parameters["WFMatchTextPattern"])
		if err != nil {
			return err
		}
		quotedIndex, quoted, err := lookup(quotedID, index, "is.workflow.actions.text.replace")
		if err != nil {
			return err
		}
		if quoted["WFReplaceTextFind"] != "\\E" || quoted["WFReplaceTextReplace"] != "\\E\\\\E\\Q" ||
			quoted["WFReplaceTextCaseSensitive"] != true || quoted["WFReplaceTextRegularExpression"] != false {
			return fmt.Errorf("dynamic RHS is not quoted through the exact literal replacement")
		}
		rightID, err := p15N03ExactToken(quoted["WFInput"])
		if err != nil {
			return err
		}
		rightIndex, right, err := lookup(rightID, quotedIndex, "is.workflow.actions.gettext")
		if err != nil || !p15N03ExactPrefixedText(right["WFTextActionText"]) {
			return fmt.Errorf("RHS nonempty Text snapshot differs: %v", err)
		}
		leftID, err := p15N03ExactToken(parameters["text"])
		if err != nil {
			return err
		}
		_, left, err := lookup(leftID, rightIndex, "is.workflow.actions.gettext")
		if err != nil || !p15N03ExactPrefixedText(left["WFTextActionText"]) {
			return fmt.Errorf("LHS nonempty Text snapshot/order differs: %v", err)
		}
		matchID, _ := parameters["UUID"].(string)
		countIndex, countID := -1, ""
		for next := index + 1; next < len(actions); next++ {
			if actions[next]["WFWorkflowActionIdentifier"] != "is.workflow.actions.count" {
				continue
			}
			count := p15N03ExactParams(actions[next])
			edge, edgeErr := p15N03ExactToken(count["Input"])
			if edgeErr == nil && edge == matchID {
				if countIndex >= 0 {
					return fmt.Errorf("match result is counted more than once")
				}
				countIndex = next
				countID, _ = count["UUID"].(string)
				if kind, present := count["WFCountType"]; present && kind != "Items" {
					return fmt.Errorf("comparison counts text units instead of match items")
				}
			}
		}
		if countIndex < 0 || countID == "" {
			return fmt.Errorf("exact comparison has no explicit match-item count")
		}
		guards := 0
		for next := countIndex + 1; next < len(actions); next++ {
			if actions[next]["WFWorkflowActionIdentifier"] != "is.workflow.actions.conditional" {
				continue
			}
			guard := p15N03ExactParams(actions[next])
			if p15Int(guard["WFControlFlowMode"]) != 0 {
				continue
			}
			edge, edgeErr := p15N03ExactToken(guard["WFInput"])
			if edgeErr != nil || edge != countID {
				continue
			}
			code := p15Int(guard["WFCondition"])
			if (code != 4 && code != 5) || p15Int(guard["WFNumberValue"]) != 1 {
				return fmt.Errorf("match count is not compared with numeric one")
			}
			for _, invented := range []string{"WFConditionalActionString", "WFConditionalCaseSensitive", "WFConditions"} {
				if _, exists := guard[invented]; exists {
					return fmt.Errorf("exact comparison acquired inappropriate condition field %s", invented)
				}
			}
			guards++
		}
		if guards != 1 {
			return fmt.Errorf("match count has %d controlling value comparisons", guards)
		}
	}
	if found != expected {
		return fmt.Errorf("explicit exact Text primitive count=%d, required=%d", found, expected)
	}
	return nil
}

func TestP15N03ExactTextPrimitive(t *testing.T) {
	const prefix = "let left = text(\"AbC\")\nlet right = text(\"a.c\\\\E[tail]\\\\\")\n"
	cases := []struct {
		name, body string
		matches    int
	}{
		{"direct_equal_dynamic", "if left == right { text(\"YES\") } else { text(\"NO\") }\n", 1},
		{"direct_not_equal_literal", "if left != \"abc\" { text(\"YES\") }\n", 1},
		{"boolean_equal", "let answer = left == right\ngetVariable(answer)\n", 1},
		{"boolean_not_equal", "let answer = left != right\ngetVariable(answer)\n", 1},
		{"compound_and", "if left == right && left != \"OTHER\" { text(\"YES\") }\n", 2},
		{"compound_or_value", "let answer = left == right || left != \"OTHER\"\ngetVariable(answer)\n", 2},
		{"negated_direct", "if !(left == right) { text(\"YES\") }\n", 1},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, prefix+test.body)
			if err := p15N03ExactPrimitiveError(actions, test.matches); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// The typed FString is an existing language route: its native.action child may
// be AnyContent, but the complete RHS is Text. Compare actual producer identities;
// a "~" marker or matching literal value does not prove the source input cursor.
func TestP15N03ExactTextImplicitSourceCursor(t *testing.T) {
	const prefix = "var left: Text = \"LEFT\"\nlet seed = text(\"N03_PREVIOUS_INPUT\")\n"
	const raw = "native.action(\"is.workflow.actions.detect.text\", {})"
	cases := []struct{ name, expression string }{
		{"fstring_rhs", "left == f\"{" + raw + "}\""},
		{"registered_text_rhs", "left == getText(" + raw + ")"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			actions := p15N03ExactCompile(t, prefix+"let answer = "+test.expression+"\ngetVariable(answer)\n")
			seed, target := -1, -1
			for index, action := range actions {
				parameters := p15N03ExactParams(action)
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.gettext" && parameters["WFTextActionText"] == "N03_PREVIOUS_INPUT" {
					seed = index
				}
				if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.detect.text" {
					if _, explicit := parameters["WFInput"]; !explicit {
						if target >= 0 {
							t.Fatal("raw source action was evaluated more than once")
						}
						target = index
					}
				}
			}
			if seed < 0 || target <= seed {
				t.Fatal("actual typed RHS did not preserve the reachable raw source fixture")
			}
			if target != seed+1 {
				previous := actions[target-1]
				edge, err := p15N03ExactToken(p15N03ExactParams(previous)["WFVariable"])
				seedID, _ := p15N03ExactParams(actions[seed])["UUID"].(string)
				if previous["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" || err != nil || edge != seedID {
					t.Fatalf("comparison helper replaced the source-owned Previous input: seed=%d target=%d error=%v", seed, target, err)
				}
			}
		})
	}
}
