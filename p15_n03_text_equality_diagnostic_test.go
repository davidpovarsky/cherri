package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"
)

const p15N03Diagnostic = "qa-n03-text-equality-diagnostic-v2"
const p15N03DiagnosticSourceSHA = "e4acefccc8a81c77a8336fa7a5f07eacaf84fc25a6dc251eb8150c607fac80ab"

type p15N03Contract struct {
	SchemaVersion            int               `json:"schema_version"`
	FixtureID                string            `json:"fixture_id"`
	SourcePath               string            `json:"source_path"`
	SourceSHA256             string            `json:"source_sha256"`
	DiagnosticPrefix         string            `json:"diagnostic_prefix"`
	FieldOrder               []string          `json:"field_order"`
	ExpectedFields           map[string]string `json:"expected_fields_for_exact_equality"`
	RequirementCandidates    []string          `json:"requirement_candidates"`
	FormalRequirementsProved []string          `json:"formal_requirements_proved"`
	RuntimeStatus            string            `json:"runtime_status"`
	Scope                    string            `json:"scope"`
}

func p15N03Fields() []string {
	return []string{"LEFT64", "SAME64", "CASE64", "FOLD_SAME_EQ", "FOLD_SAME_NE", "FOLD_CASE_EQ", "FOLD_CASE_NE", "DYN_SAME_EQ", "DYN_SAME_NE", "DYN_CASE_EQ", "DYN_CASE_NE", "IF_SAME_EQ", "IF_SAME_NE", "IF_CASE_EQ", "IF_CASE_NE", "MATCH_SAME_CS", "MATCH_CASE_CS", "MATCH_CASE_CI"}
}

func p15N03ValidateContract(data []byte, source string) error {
	var contract p15N03Contract
	var decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("extra contract content")
	}
	var expected = map[string]string{
		"LEFT64": "YWJj", "SAME64": "YWJj", "CASE64": "QUJD",
		"FOLD_SAME_EQ": "Y", "FOLD_SAME_NE": "N", "FOLD_CASE_EQ": "N", "FOLD_CASE_NE": "Y",
		"DYN_SAME_EQ": "Y", "DYN_SAME_NE": "N", "DYN_CASE_EQ": "N", "DYN_CASE_NE": "Y",
		"IF_SAME_EQ": "Y", "IF_SAME_NE": "N", "IF_CASE_EQ": "N", "IF_CASE_NE": "Y",
		"MATCH_SAME_CS": "1", "MATCH_CASE_CS": "0", "MATCH_CASE_CI": "1",
	}
	if contract.SchemaVersion != 1 || contract.FixtureID != p15N03Diagnostic ||
		contract.SourcePath != "tools/shortcuts-simulator-qa/fixtures/"+p15N03Diagnostic+".cherri" ||
		contract.SourceSHA256 != p15N03DiagnosticSourceSHA || fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != p15N03DiagnosticSourceSHA ||
		contract.DiagnosticPrefix != "CHERRI_QA_N03_TEXT_EQ_DIAG:" || !reflect.DeepEqual(contract.FieldOrder, p15N03Fields()) ||
		!reflect.DeepEqual(contract.ExpectedFields, expected) || !reflect.DeepEqual(contract.RequirementCandidates, []string{"N03"}) ||
		contract.FormalRequirementsProved == nil || len(contract.FormalRequirementsProved) != 0 || contract.RuntimeStatus != "NOT_RUN" ||
		!strings.Contains(contract.Scope, "does not establish legacy migration behavior or close N03") {
		return fmt.Errorf("source-bound N03 diagnostic contract changed or gained unearned acceptance")
	}
	return nil
}

func p15N03Attachment(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": ref}
}

func p15N03TokenString(ref map[string]any) map[string]any {
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": "\uFFFC", "attachmentsByRange": map[string]any{"{0, 1}": ref}}}
}

type p15N03Cursor struct {
	actions p15DiagnosticGraph
	index   int
	err     error
	groups  map[string]bool
}

// Only generated identities are read from the result. Every action, field,
// operand, branch value, variable store and producer edge is independently specified.
func (c *p15N03Cursor) take(identifier, outputName string, fields map[string]any, hasUUID bool) map[string]any {
	if c.err != nil {
		return nil
	}
	if c.index >= len(c.actions) {
		c.err = fmt.Errorf("missing action %d", c.index)
		return nil
	}
	var params = p15DiagnosticParams(c.actions, c.index)
	var ref map[string]any
	if hasUUID {
		var uuid, ok = params["UUID"].(string)
		if !ok || uuid == "" {
			c.err = fmt.Errorf("action %d has no output identity", c.index)
			return nil
		}
		fields["UUID"] = uuid
		ref = map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": outputName}
	}
	if outputName != "" {
		fields["CustomOutputName"] = outputName
	}
	if c.actions[c.index]["WFWorkflowActionIdentifier"] != "is.workflow.actions."+identifier || !reflect.DeepEqual(params, fields) {
		c.err = fmt.Errorf("N03 action %d differs from independent %s/%s contract: got %#v, want %#v", c.index, identifier, outputName, params, fields)
		return nil
	}
	c.index++
	return ref
}

func (c *p15N03Cursor) start(input map[string]any, code float64, rhsKey string, rhs any) string {
	if c.err != nil {
		return ""
	}
	var group, ok = p15DiagnosticParams(c.actions, c.index)["GroupingIdentifier"].(string)
	if !ok || group == "" || c.groups[group] {
		c.err = fmt.Errorf("missing or reused N03 group")
		return ""
	}
	c.groups[group] = true
	c.take("conditional", "", map[string]any{
		"GroupingIdentifier": group, "WFControlFlowMode": float64(0), "WFCondition": code,
		"WFConditionalLegacyComparisonBehavior": true,
		"WFInput":                               map[string]any{"Type": "Variable", "Variable": p15N03Attachment(input)}, rhsKey: rhs,
	}, false)
	return group
}

func (c *p15N03Cursor) valueComparison(name string, left map[string]any, code float64, right any) map[string]any {
	var group = c.start(left, code, "WFConditionalActionString", right)
	c.take("number", "", map[string]any{"WFNumberActionNumber": float64(1)}, false)
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(1)}, false)
	c.take("number", "", map[string]any{"WFNumberActionNumber": float64(0)}, false)
	return c.take("conditional", name, map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(2)}, true)
}

func (c *p15N03Cursor) flag(name string, input map[string]any, code float64, rhsKey string, rhs any) map[string]any {
	var no = c.take("gettext", "Text", map[string]any{"WFTextActionText": "N"}, true)
	c.take("setvariable", "", map[string]any{"WFVariableName": name, "WFInput": p15N03Attachment(no)}, false)
	var group = c.start(input, code, rhsKey, rhs)
	var yes = c.take("gettext", "Text", map[string]any{"WFTextActionText": "Y"}, true)
	c.take("setvariable", "", map[string]any{"WFVariableName": name, "WFInput": p15N03Attachment(yes)}, false)
	c.take("conditional", "", map[string]any{"GroupingIdentifier": group, "WFControlFlowMode": float64(2)}, false)
	return map[string]any{"Type": "Variable", "VariableName": name}
}

func p15N03NamedIndex(actions p15DiagnosticGraph, name string) int {
	for i := range actions {
		if p15DiagnosticParams(actions, i)["CustomOutputName"] == name {
			return i
		}
	}
	return -1
}

func p15N03ValidateGraph(actions p15DiagnosticGraph) error {
	if err := p15DiagnosticReferencesValid(actions); err != nil {
		return err
	}
	var begin = p15N03NamedIndex(actions, "n03Left")
	if begin != 5 {
		return fmt.Errorf("diagnostic seed and before-trace prefix changed")
	}
	var prefix = []string{"comment", "nothing", "setvariable", "gettext", "appendvariable"}
	for i, identifier := range prefix {
		if actions[i]["WFWorkflowActionIdentifier"] != "is.workflow.actions."+identifier {
			return fmt.Errorf("unexpected diagnostic prefix action")
		}
	}
	if p15DiagnosticParams(actions, 0)["WFCommentActionText"] != "ANCHOR_CARD_01_TOP_HEADER" ||
		!reflect.DeepEqual(p15DiagnosticParams(actions, 2), map[string]any{"WFVariableName": "__cherri_debug_log"}) ||
		p15DiagnosticParams(actions, 4)["WFVariableName"] != "__cherri_qa_trace" {
		return fmt.Errorf("diagnostic prefix writes unrelated state")
	}
	var c = p15N03Cursor{actions: actions, index: begin, groups: map[string]bool{}}
	var refs = map[string]map[string]any{}
	for _, seed := range []struct{ name, value string }{{"n03Left", "abc"}, {"n03Same", "abc"}, {"n03Case", "ABC"}} {
		refs[seed.name] = c.take("gettext", seed.name, map[string]any{"WFTextActionText": seed.value}, true)
	}
	var observations []map[string]any
	for _, name := range []string{"n03Left", "n03Same", "n03Case"} {
		observations = append(observations, c.take("base64encode", name+"64", map[string]any{
			"WFInput": p15N03Attachment(refs[name]), "WFBase64LineBreakMode": "None", "WFEncodeMode": "Encode", "input": "Encode",
		}, true))
	}
	for _, mode := range []string{"Fold", "Dyn", "If"} {
		if mode == "If" {
			c.take("comment", "", map[string]any{"WFCommentActionText": "ANCHOR_CARD_15_MID_CENTER"}, false)
		}
		for _, pair := range []struct{ kind, rhs, literal string }{{"Same", "n03Same", "abc"}, {"Case", "n03Case", "ABC"}} {
			for _, op := range []struct {
				name string
				code float64
			}{{"Eq", 4}, {"Ne", 5}} {
				var stem = "n03" + mode + pair.kind + op.name
				if mode == "If" {
					observations = append(observations, c.flag(stem+"Flag", refs["n03Left"], op.code, "WFConditionalActionString", pair.literal))
					continue
				}
				var value map[string]any
				if mode == "Fold" {
					var result float64
					if (pair.kind == "Same") == (op.name == "Eq") {
						result = 1
					}
					value = c.take("number", stem+"Value", map[string]any{"WFNumberActionNumber": result}, true)
				} else {
					value = c.valueComparison(stem+"Value", refs["n03Left"], op.code, p15N03TokenString(refs[pair.rhs]))
				}
				observations = append(observations, c.flag(stem+"Flag", value, 4, "WFNumberValue", float64(1)))
			}
		}
	}
	for _, match := range []struct {
		suffix, subject string
		sensitive       bool
	}{{"SameCs", "n03Same", true}, {"CaseCs", "n03Case", true}, {"CaseCi", "n03Case", false}} {
		var result = c.take("text.match", "n03Match"+match.suffix, map[string]any{
			"WFMatchTextPattern": "^abc$", "WFMatchTextCaseSensitive": match.sensitive, "text": p15N03TokenString(refs[match.subject]),
		}, true)
		observations = append(observations, c.take("count", "n03Count"+match.suffix, map[string]any{"Input": p15N03Attachment(result)}, true))
	}
	if c.err != nil {
		return c.err
	}
	// The only suffix actions transport the trace and actual observed values.
	var suffix = []string{"gettext", "appendvariable", "text.combine", "gettext", "gettext", "comment", "output"}
	if len(actions)-c.index != len(suffix) {
		return fmt.Errorf("extra or missing diagnostic suffix action")
	}
	for n, identifier := range suffix {
		if actions[c.index+n]["WFWorkflowActionIdentifier"] != "is.workflow.actions."+identifier {
			return fmt.Errorf("unexpected diagnostic suffix action")
		}
	}
	if p15DiagnosticParams(actions, c.index+1)["WFVariableName"] != "__cherri_qa_trace" {
		return fmt.Errorf("trace suffix modifies observed flags")
	}
	var trace = p15DiagnosticParams(actions, c.index+3)
	if trace["CustomOutputName"] != "n03Trace" {
		return fmt.Errorf("trace producer missing")
	}
	var traceRef = map[string]any{"Type": "ActionOutput", "OutputUUID": trace["UUID"], "OutputName": "n03Trace"}
	var text strings.Builder
	text.WriteString("CHERRI_QA_N03_TEXT_EQ_DIAG:")
	var attachments = map[string]any{}
	for i, field := range p15N03Fields() {
		if i > 0 {
			text.WriteByte(':')
		}
		text.WriteString(field + "=")
		attachments[fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(text.String()))))] = observations[i]
		text.WriteRune('\uFFFC')
	}
	text.WriteByte('\n')
	attachments[fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(text.String()))))] = traceRef
	text.WriteRune('\uFFFC')
	c.index += 4
	var summary = c.take("gettext", "n03Summary", map[string]any{"WFTextActionText": map[string]any{
		"WFSerializationType": "WFTextTokenString", "Value": map[string]any{"string": text.String(), "attachmentsByRange": attachments},
	}}, true)
	c.take("comment", "", map[string]any{"WFCommentActionText": "ANCHOR_CARD_28_BOTTOM_FOOTER"}, false)
	c.take("output", "", map[string]any{"WFOutput": p15N03TokenString(summary)}, false)
	return c.err
}

func TestP15N03TextEqualityDiagnosticSourceAndGraph(t *testing.T) {
	var actions, source = p15CompileDiagnosticGraph(t, p15N03Diagnostic)
	var contract, err = os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + p15N03Diagnostic + ".expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = p15N03ValidateContract(contract, source); err != nil {
		t.Fatal(err)
	}
	if err = p15N03ValidateGraph(actions); err != nil {
		t.Fatal(err)
	}
	t.Logf("N03 diagnostic source and %d native actions validated; Apple runtime NOT_RUN; no formal requirement credit", len(actions))
}

func TestP15N03TextEqualityDiagnosticRejectsContractDrift(t *testing.T) {
	var _, source = p15CompileDiagnosticGraph(t, p15N03Diagnostic)
	var data, err = os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + p15N03Diagnostic + ".expected.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"source", "case_witness", "case_equality", "missing_field", "runtime_credit", "formal_credit", "unknown_field"} {
		t.Run(name, func(t *testing.T) {
			var contract map[string]any
			if err := json.Unmarshal(data, &contract); err != nil {
				t.Fatal(err)
			}
			var candidateSource = source
			switch name {
			case "source":
				candidateSource = strings.Replace(source, "text(\"ABC\")", "text(\"abc\")", 1)
			case "case_witness":
				contract["expected_fields_for_exact_equality"].(map[string]any)["CASE64"] = "YWJj"
			case "case_equality":
				contract["expected_fields_for_exact_equality"].(map[string]any)["DYN_CASE_EQ"] = "Y"
			case "missing_field":
				delete(contract["expected_fields_for_exact_equality"].(map[string]any), "DYN_CASE_NE")
			case "runtime_credit":
				contract["runtime_status"] = "PASSED"
			case "formal_credit":
				contract["formal_requirements_proved"] = []string{"N03"}
			case "unknown_field":
				contract["passed"] = true
			}
			var mutated, marshalErr = json.Marshal(contract)
			if marshalErr != nil {
				t.Fatal(marshalErr)
			}
			if err := p15N03ValidateContract(mutated, candidateSource); err == nil {
				t.Fatal("accepted changed N03 diagnostic contract")
			}
		})
	}
}

func TestP15N03TextEqualityDiagnosticRejectsGraphFaults(t *testing.T) {
	var good, _ = p15CompileDiagnosticGraph(t, p15N03Diagnostic)
	var names = []string{"case_operand", "base64_witness", "folded_result", "dynamic_rhs", "inequality_code", "numeric_yields", "missing_compatibility", "invented_case_field", "flag_seed", "flag_store", "match_case_flag", "match_pattern", "match_subject", "count_input", "constant_summary", "summary_offset", "output_bypass"}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			var raw, err = json.Marshal(good)
			if err != nil {
				t.Fatal(err)
			}
			var actions p15DiagnosticGraph
			if err = json.Unmarshal(raw, &actions); err != nil {
				t.Fatal(err)
			}
			var named = func(n string) map[string]any { return p15DiagnosticParams(actions, p15N03NamedIndex(actions, n)) }
			var dynamic = p15N03NamedIndex(actions, "n03DynCaseEqValue") - 4
			var originalCase = map[string]any{"Type": "ActionOutput", "OutputUUID": named("n03Case")["UUID"], "OutputName": "n03Case"}
			var originalSame = map[string]any{"Type": "ActionOutput", "OutputUUID": named("n03Same")["UUID"], "OutputName": "n03Same"}
			switch name {
			case "case_operand":
				named("n03Case")["WFTextActionText"] = "abc"
			case "base64_witness":
				named("n03Case64")["WFInput"] = p15N03Attachment(originalSame)
			case "folded_result":
				named("n03FoldCaseEqValue")["WFNumberActionNumber"] = float64(1)
			case "dynamic_rhs":
				p15DiagnosticParams(actions, dynamic)["WFConditionalActionString"] = p15N03TokenString(originalSame)
			case "inequality_code":
				p15DiagnosticParams(actions, p15N03NamedIndex(actions, "n03DynCaseNeValue")-4)["WFCondition"] = float64(4)
			case "numeric_yields":
				actions[dynamic+1], actions[dynamic+3] = actions[dynamic+3], actions[dynamic+1]
			case "missing_compatibility":
				delete(p15DiagnosticParams(actions, dynamic), "WFConditionalLegacyComparisonBehavior")
			case "invented_case_field":
				p15DiagnosticParams(actions, dynamic)["WFConditionalCaseSensitive"] = true
			case "flag_seed":
				p15DiagnosticParams(actions, p15N03NamedIndex(actions, "n03FoldCaseEqValue")+1)["WFTextActionText"] = "Y"
			case "flag_store":
				for i := range actions {
					if p15DiagnosticParams(actions, i)["WFVariableName"] == "n03DynCaseEqFlag" {
						p15DiagnosticParams(actions, i)["WFVariableName"] = "n03DynSameEqFlag"
						break
					}
				}
			case "match_case_flag":
				named("n03MatchCaseCs")["WFMatchTextCaseSensitive"] = false
			case "match_pattern":
				named("n03MatchCaseCs")["WFMatchTextPattern"] = "abc"
			case "match_subject":
				named("n03MatchSameCs")["text"] = p15N03TokenString(originalCase)
			case "count_input":
				named("n03CountCaseCs")["Input"] = p15N03Attachment(originalCase)
			case "constant_summary":
				named("n03Summary")["WFTextActionText"] = "CHERRI_QA_N03_TEXT_EQ_DIAG:all-good"
			case "summary_offset":
				var value = named("n03Summary")["WFTextActionText"].(map[string]any)["Value"].(map[string]any)
				var attachments = value["attachmentsByRange"].(map[string]any)
				attachments["{35, 1}"] = attachments["{34, 1}"]
				delete(attachments, "{34, 1}")
			case "output_bypass":
				p15DiagnosticParams(actions, len(actions)-1)["WFOutput"] = p15N03TokenString(originalCase)
			}
			if err = p15N03ValidateGraph(actions); err == nil {
				t.Fatal("accepted causal N03 diagnostic graph fault")
			}
		})
	}
}
