package main

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
)

// Compare the exact native body with the independently checked standalone
// experiment. Only randomized UUIDs and the deliberately named final Text
// binding differ. No action, parameter, value, order or conditional is removed.
func p15LocalBodyIdentity(body []map[string]any) []map[string]any {
	encoded, _ := json.Marshal(body)
	var copy []map[string]any
	_ = json.Unmarshal(encoded, &copy)
	ids := map[string]string{}
	for _, action := range copy {
		for _, key := range []string{"UUID", "GroupingIdentifier"} {
			if id, ok := p15Param(action)[key].(string); ok && id != "" && ids[id] == "" {
				ids[id] = fmt.Sprintf("native-id-%d", len(ids))
			}
		}
	}
	var visit func(any)
	visit = func(value any) {
		switch node := value.(type) {
		case map[string]any:
			for key, item := range node {
				if key == "UUID" || key == "OutputUUID" || key == "GroupingIdentifier" {
					if id, ok := item.(string); ok && ids[id] != "" {
						node[key] = ids[id]
					}
				}
				visit(item)
			}
		case []any:
			for _, item := range node {
				visit(item)
			}
		}
	}
	for _, action := range copy {
		visit(action)
	}
	if len(copy) > 0 {
		delete(p15Param(copy[len(copy)-1]), "CustomOutputName")
	}
	return copy
}

func p15LocalWrappedGraph(standalone, wrapped []map[string]any) error {
	start, end := -1, -1
	outputs := map[string]map[string]any{}
	for i, action := range wrapped {
		if action["WFWorkflowActionIdentifier"] == "is.workflow.actions.runworkflow" {
			return fmt.Errorf("wrapper introduced Run Shortcut")
		}
		p := p15Param(action)
		if name, ok := p["CustomOutputName"].(string); ok {
			outputs[name] = p
			if name == "zero" {
				start = i
			}
			if name == "semanticResult" {
				end = i
			}
		}
	}
	if start < 0 || end <= start || len(standalone) < 2 {
		return fmt.Errorf("missing observable native experiment body")
	}
	if !reflect.DeepEqual(p15LocalBodyIdentity(standalone[:len(standalone)-1]), p15LocalBodyIdentity(wrapped[start:end+1])) {
		return fmt.Errorf("wrapper changed native counter experiment")
	}
	summary, exists := outputs["finalSummary"]
	if !exists || outputs["traceEnvelope"] == nil {
		return fmt.Errorf("missing explicit callback/trace producers")
	}
	token, _ := summary["WFTextActionText"].(map[string]any)
	value, _ := token["Value"].(map[string]any)
	refs, _ := value["attachmentsByRange"].(map[string]any)
	const prefix = "CHERRI_QA_F01_LOCAL_RESULT:"
	if token["WFSerializationType"] != "WFTextTokenString" || value["string"] != prefix+"\uFFFC\n\uFFFC" ||
		p15LocalAttachment(refs[fmt.Sprintf("{%d, 1}", len(prefix))])["OutputUUID"] != outputs["semanticResult"]["UUID"] ||
		p15LocalAttachment(refs[fmt.Sprintf("{%d, 1}", len(prefix)+2)])["OutputUUID"] != outputs["traceEnvelope"]["UUID"] {
		return fmt.Errorf("callback does not carry the original counter result and trace")
	}
	last := wrapped[len(wrapped)-1]
	output, _ := p15Param(last)["WFOutput"].(map[string]any)
	outputValue, _ := output["Value"].(map[string]any)
	outputRefs, _ := outputValue["attachmentsByRange"].(map[string]any)
	if last["WFWorkflowActionIdentifier"] != "is.workflow.actions.output" || p15LocalAttachment(outputRefs["{0, 1}"])["OutputUUID"] != summary["UUID"] {
		return fmt.Errorf("final output is disconnected from the callback summary")
	}
	return nil
}

func TestP15F01LocalRuntimeWrapperPreservesCounterProof(t *testing.T) {
	source, err := os.ReadFile("tests/language-v2/v2-short-circuit-local-runtime.cherri")
	if err != nil {
		t.Fatal(err)
	}
	wrapper, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/qa-short-circuit-local-v2.cherri")
	if err != nil {
		t.Fatal(err)
	}
	header := "shortcut \"Cherri P15 F01 Local\" {\n    version: \"27\"\n}\n"
	body := strings.TrimSuffix(strings.TrimPrefix(string(source), header), "return f\"{verdict}:calls={calls}\"\n")
	if !strings.Contains(string(wrapper), body) {
		t.Fatal("standalone source body changed in wrapper")
	}
	standalone := p15CompiledActions(t, string(source))
	if err := p15LocalCounterGraph(standalone); err != nil {
		t.Fatal(err)
	}
	wrapped := p15CompiledActions(t, string(wrapper))
	if err := p15LocalWrappedGraph(standalone, wrapped); err != nil {
		t.Fatal(err)
	}
	for _, broken := range []string{
		strings.Replace(string(wrapper), "calls += 1", "calls += 2", 1),
		strings.Replace(string(wrapper), `text(f"{verdict}:calls={calls}")`, `text("P15_F01_LOCAL_OK:calls=1")`, 1),
	} {
		if err := p15LocalWrappedGraph(standalone, p15CompiledActions(t, broken)); err == nil {
			t.Fatal("wrapper accepted a changed increment or hidden counter")
		}
	}
	t.Logf("standalone=%d, wrapped=%d native actions; signed runtime remains open", len(standalone), len(wrapped))
}
