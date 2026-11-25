package main

import (
	"fmt"
	"strings"
	"testing"
)

func legacyCompoundReference(t *testing.T, value any, name string) {
	t.Helper()
	var attachment = debugParamMap(value)
	var reference = debugParamMap(attachment["Value"])
	if attachment["WFSerializationType"] != "WFTextTokenAttachment" || reference["Type"] != "Variable" || reference["VariableName"] != name {
		t.Fatalf("expected native variable %q, got %#v", name, value)
	}
}

func TestLegacyLoopReferenceCompoundArithmetic(t *testing.T) {
	for _, prefix := range []string{"", "@"} {
		for _, operator := range []struct{ source, native string }{{"+=", "+"}, {"-=", "-"}, {"*=", "×"}, {"/=", "÷"}} {
			t.Run(prefix+operator.source, func(t *testing.T) {
				var source = fmt.Sprintf("@sum = 10\nrepeat i for 5 {\n @sum %s %si\n}\noutput(\"{@sum}\")\n", operator.source, prefix)
				var actions = compileDebugFixture(t, source, true)
				var mathIndex = -1
				var bindingIndex = -1
				for index, action := range actions {
					var parameters = action.WFWorkflowActionParameters
					if parameters["WFVariableName"] == "i" {
						legacyCompoundReference(t, parameters["WFInput"], "Repeat Index")
						bindingIndex = index
					}
					if action.WFWorkflowActionIdentifier == "is.workflow.actions.math" {
						if mathIndex != -1 {
							t.Fatal("duplicate compound Math action")
						}
						mathIndex = index
						if parameters["WFMathOperation"] != operator.native {
							t.Fatalf("wrong operation: %#v", parameters)
						}
						legacyCompoundReference(t, parameters["WFInput"], "sum")
						legacyCompoundReference(t, parameters["WFMathOperand"], "i")
					}
				}
				if bindingIndex < 0 || mathIndex <= bindingIndex || mathIndex+1 >= len(actions) {
					t.Fatalf("missing ordered loop binding/Math/assignment: %#v", actions)
				}
				var assignment = actions[mathIndex+1]
				var input = debugParamMap(debugParamMap(assignment.WFWorkflowActionParameters["WFInput"])["Value"])
				if assignment.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" || assignment.WFWorkflowActionParameters["WFVariableName"] != "sum" || input["Type"] != "ActionOutput" || input["OutputUUID"] != actions[mathIndex].WFWorkflowActionParameters["UUID"] {
					t.Fatalf("compound assignment bypassed its Math output: %#v", assignment)
				}
			})
		}
	}
}

func TestLegacyCompoundNumericReferencesAndArrayAppend(t *testing.T) {
	for _, setup := range []string{"@operand = 2", "@operand = 2.5", "const operand = 1 + 1", "const operand = round(2.2)", "@original = 2\n@operand = @original"} {
		t.Run(setup, func(t *testing.T) {
			var reference = "@operand"
			if strings.HasPrefix(setup, "const") {
				reference = "operand"
			}
			var actions = compileDebugFixture(t, "#include 'actions/math'\n"+setup+"\n@sum = 10\n@sum += "+reference+"\n", true)
			var lastMath = -1
			for index, action := range actions {
				if action.WFWorkflowActionIdentifier == "is.workflow.actions.math" {
					lastMath = index
				}
			}
			if lastMath < 0 || lastMath+1 >= len(actions) {
				t.Fatal("compound reference emitted no arithmetic")
			}
			var assignment = actions[lastMath+1].WFWorkflowActionParameters
			var input = debugParamMap(debugParamMap(assignment["WFInput"])["Value"])
			if assignment["WFVariableName"] != "sum" || input["Type"] != "ActionOutput" || input["OutputUUID"] != actions[lastMath].WFWorkflowActionParameters["UUID"] {
				t.Fatalf("not assigned from computed output: %#v", assignment)
			}
		})
	}
	var actions = compileDebugFixture(t, "@items = []\n@value = 2\n@items += @value\n", true)
	var appended = 0
	for _, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.math" {
			t.Fatal("array append became arithmetic")
		}
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" {
			appended++
			legacyCompoundReference(t, action.WFWorkflowActionParameters["WFInput"], "value")
		}
	}
	if appended != 1 {
		t.Fatalf("array append count: %d", appended)
	}
}

func TestLegacyLoopReferenceSyntaxBoundaries(t *testing.T) {
	for _, prefix := range []string{"", "@"} {
		compileDebugFixture(t, "@items = [1, 2]\nfor item in @items {\n show("+prefix+"item)\n}\n", true)
		compileDebugFixture(t, "repeat i for 2 {\n repeat j for 3 {\n  const combined = "+prefix+"i + "+prefix+"j\n  show(combined)\n }\n}\n", true)
	}
	for _, source := range []string{"@value = 3\nnumber(value)\n", "repeat i for 1 {\n show(i)\n}\n@i = 7\nnumber(i)\n"} {
		var raw, err = compileLegacySourceToRawMapInMemory(source)
		if err == nil || raw != nil || !strings.Contains(err.Error(), "prepended with @") {
			t.Fatalf("ordinary mutable reference lost @ requirement: %v", err)
		}
	}
}

func TestLegacyCompoundTextReferencePreservesBothOperands(t *testing.T) {
	var actions = compileDebugFixture(t, "@value = \"left\"\n@other = \"right\"\n@value += @other\n", true)
	var matched = false
	for index, action := range actions {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.gettext" {
			continue
		}
		var field = debugParamMap(action.WFWorkflowActionParameters["WFTextActionText"])
		var text = debugParamMap(field["Value"])
		var attachments = debugParamMap(text["attachmentsByRange"])
		if len(attachments) != 2 {
			continue
		}
		matched = true
		if text["string"] != "\ufffc\ufffc" || debugParamMap(attachments["{0, 1}"])["VariableName"] != "value" || debugParamMap(attachments["{1, 1}"])["VariableName"] != "other" {
			t.Fatalf("wrong compound text: %#v", text)
		}
		var assignment = actions[index+1].WFWorkflowActionParameters
		var reference = debugParamMap(debugParamMap(assignment["WFInput"])["Value"])
		if reference["Type"] != "ActionOutput" || reference["OutputUUID"] != action.WFWorkflowActionParameters["UUID"] {
			t.Fatalf("text assignment bypassed concatenation: %#v", assignment)
		}
	}
	if !matched {
		t.Fatal("missing native text concatenation")
	}
}
