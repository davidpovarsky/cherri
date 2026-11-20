package main

import (
	"fmt"
	"testing"
)

func TestP15DebugExportUsesBase64FileMaterialization(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		for _, format := range []string{"json", "text"} {
			for _, redact := range []bool{false, true} {
				t.Run(fmt.Sprintf("legacy=%t/format=%s/redact=%t", legacy, format, redact), func(t *testing.T) {
					src := "debugClearLogs()\ntext(\"reviewed synthetic text\")\ndebugLog()\n" +
						fmt.Sprintf("debugExport(%q, false, true, %t)\n", format, redact)
					actions := compileDebugFixture(t, src, legacy)
					materializationEnd := len(actions)
					if !legacy {
						checkDebugMainVoidBoundary(t, actions)
						materializationEnd--
					}
					if materializationEnd < 4 {
						t.Fatal("missing final File materialization")
					}
					tail := actions[materializationEnd-4 : materializationEnd]
					expected := []string{"gettext", "base64encode", "base64encode", "setitemname"}
					for i, suffix := range expected {
						if tail[i].WFWorkflowActionIdentifier != "is.workflow.actions."+suffix {
							t.Fatalf("File boundary action %d = %s, want %s", i, tail[i].WFWorkflowActionIdentifier, suffix)
						}
						if i > 0 {
							input := debugParamMap(debugParamMap(tail[i].WFWorkflowActionParameters["WFInput"])["Value"])
							producer := tail[i-1].WFWorkflowActionParameters["UUID"]
							if input["Type"] != "ActionOutput" || producer == nil || input["OutputUUID"] != producer {
								t.Fatalf("File boundary %d is not linked to exact preceding producer", i)
							}
						}
					}
					encoder, decoder := tail[1].WFWorkflowActionParameters, tail[2].WFWorkflowActionParameters
					if encoder["WFEncodeMode"] != "Encode" || encoder["WFBase64LineBreakMode"] != "None" ||
						decoder["WFEncodeMode"] != "Decode" {
						t.Fatal("File bytes require Text -> unwrapped Base64 Encode -> Base64 Decode")
					}
					decodes := 0
					for _, action := range actions {
						if action.WFWorkflowActionIdentifier == "is.workflow.actions.gettypeaction" {
							t.Fatal("unproved Get File of Type Text conversion remains")
						}
						if action.WFWorkflowActionIdentifier == "is.workflow.actions.base64encode" &&
							action.WFWorkflowActionParameters["WFEncodeMode"] == "Decode" {
							decodes++
						}
					}
					if decodes != 1 {
						t.Fatalf("expected one final File materializer, got %d", decodes)
					}
				})
			}
		}
	}
}
