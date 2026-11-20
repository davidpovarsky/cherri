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

const p15OptionalDiagnostic = "qa-optional-forwarding-diagnostic-v2"
const p15OptionalDiagnosticSourceSHA = "e6f195aadcd2d46f25c6fe1de5165798c18a40f4c1942785b9e39171b1cc2139"
const p15OptionalDiagnosticPrefix = "CHERRI_OPTIONAL_FORWARDING_DIAGNOSTIC_V1:"

func p15OptionalDiagnosticFields() [][2]string {
	return [][2]string{
		{"NONE", "NONE"}, {"EMPTY_TEXT", "TEXT[]"}, {"FALSE", "BOOL[false]"},
		{"ZERO", "NUMBER[0]"}, {"EMPTY_LIST", "LIST[0]"}, {"EMPTY_MAP", "MAP[0]"},
		{"LIST", "LIST[2|A|B]"}, {"MAP", "MAP[1|VALUE|Dictionary]"},
		{"SNAPSHOT", "NONE"}, {"WRITTEN", "TEXT[]"}, {"DEFAULT", "TEXT[DEFAULT]"},
	}
}

type p15OptionalDiagnosticContract struct {
	SchemaVersion            int               `json:"schema_version"`
	FixtureID                string            `json:"fixture_id"`
	SourcePath               string            `json:"source_path"`
	SourceSHA256             string            `json:"source_sha256"`
	DiagnosticPrefix         string            `json:"diagnostic_prefix"`
	FieldOrder               []string          `json:"field_order"`
	ExpectedFields           map[string]string `json:"expected_fields"`
	ExpectedSummary          string            `json:"expected_summary"`
	ExpectedSummarySHA256    string            `json:"expected_summary_sha256"`
	TraceRunID               string            `json:"trace_run_id"`
	TraceLabel               string            `json:"trace_label"`
	FormalRequirementsProved []string          `json:"formal_requirements_proved"`
	RuntimeStatus            string            `json:"runtime_status"`
	Scope                    string            `json:"scope"`
}

func p15OptionalDiagnosticContractError(data []byte, source string) error {
	var c p15OptionalDiagnosticContract
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&c); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return fmt.Errorf("extra contract content")
	}
	fields := map[string]string{}
	order := []string{}
	parts := []string{}
	for _, field := range p15OptionalDiagnosticFields() {
		fields[field[0]] = field[1]
		order = append(order, field[0])
		parts = append(parts, field[0]+"="+field[1])
	}
	expected := p15OptionalDiagnosticPrefix + strings.Join(parts, ":")
	if c.SchemaVersion != 1 || c.FixtureID != p15OptionalDiagnostic ||
		c.SourcePath != "tools/shortcuts-simulator-qa/fixtures/"+p15OptionalDiagnostic+".cherri" ||
		c.SourceSHA256 != p15OptionalDiagnosticSourceSHA || fmt.Sprintf("%x", sha256.Sum256([]byte(source))) != p15OptionalDiagnosticSourceSHA ||
		c.DiagnosticPrefix != p15OptionalDiagnosticPrefix || !reflect.DeepEqual(c.FieldOrder, order) || !reflect.DeepEqual(c.ExpectedFields, fields) ||
		c.ExpectedSummary != expected || c.ExpectedSummarySHA256 != fmt.Sprintf("%x", sha256.Sum256([]byte(expected))) ||
		c.TraceRunID != p15OptionalDiagnostic || c.TraceLabel != "optional-forwarding" ||
		c.FormalRequirementsProved == nil || len(c.FormalRequirementsProved) != 0 || c.RuntimeStatus != "NOT_RUN" ||
		c.Scope != "Public synthetic diagnostic of tagged Optional argument/result forwarding, explicit None versus empty Text/false/zero/empty containers, populated flat container payloads, omitted default and earlier-argument snapshot. Expected Dictionary is the English native type label for the populated map. Actual observed fields and NDJSON trace must be preserved even on contradiction. No general value-join, nested-container, File, native acceptance or full requirement closure is established." {
		return fmt.Errorf("Optional diagnostic source/expected output/scope changed")
	}
	return nil
}

// An independent token-string oracle: literal chunks and exact UTF-16 ranges
// are authored here. Neither production encoding helpers nor observed fields
// determine the expected representation.
func p15OptionalDiagnosticText(chunks []string, attachments ...any) map[string]any {
	if len(chunks) != len(attachments)+1 {
		panic("invalid authored token oracle")
	}
	var text strings.Builder
	refs := map[string]any{}
	for i, ref := range attachments {
		text.WriteString(chunks[i])
		refs[fmt.Sprintf("{%d, 1}", len(utf16.Encode([]rune(text.String()))))] = ref
		text.WriteRune('\ufffc')
	}
	text.WriteString(chunks[len(chunks)-1])
	value := map[string]any{"string": text.String()}
	if len(refs) > 0 {
		value["attachmentsByRange"] = refs
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString", "Value": value}
}
func p15OptionalDiagnosticRef(uuid string) any {
	return map[string]any{"Type": "ActionOutput", "OutputUUID": uuid}
}
func p15OptionalDiagnosticVar(name string) any {
	return map[string]any{"Type": "Variable", "VariableName": name}
}

func p15OptionalDiagnosticTrace(g *p15FunctionGraph, stage, previous string) {
	// The trace protocol stores the real preceding native action count.
	prefix := fmt.Sprintf(`{"schemaVersion":"1.4","kind":"INLINE_DEBUG_ACTION","stage":"%s","label":"optional-forwarding","stepSequence":%d,"actionIdentifier":"is.workflow.actions.%s","timestampPrecision":"apple-currentdate","valueType":"UNKNOWN_AT_TRACE_EVENT","timestamp":"`, stage, len(g.expected), previous)
	line := g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText([]string{prefix, `"}`}, map[string]any{"Type": "CurrentDate"})})
	g.emit("appendvariable", map[string]any{"WFVariableName": "__cherri_qa_trace", "WFInput": p15FunctionEdge(line)})
}
func p15OptionalDiagnosticTraceExport(g *p15FunctionGraph) string {
	combined := g.value("text.combine", map[string]any{"text": p15FunctionVariable("__cherri_qa_trace"), "WFTextSeparator": "New Lines"})
	header := `{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_begin","runId":"` + p15OptionalDiagnostic + `","traceTransport":"ndjson-stream","collectorSentinel":"===CHERRI_QA_TRACE_NDJSON_V14==="}` + "\n"
	footer := "\n" + `{"schemaVersion":"1.4","kind":"COLLECTOR_EVENT","stage":"collector_end","runId":"` + p15OptionalDiagnostic + `","traceTransport":"ndjson-stream","collectorSentinel":"===CHERRI_QA_TRACE_NDJSON_V14==="}`
	return g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText([]string{header, footer}, p15OptionalDiagnosticRef(combined))})
}
func p15OptionalDiagnosticStore(g *p15FunctionGraph, name, value string) {
	g.emit("setvariable", map[string]any{"WFVariableName": name, "WFInput": p15FunctionEdge(value)})
}
func p15OptionalDiagnosticObservation(g *p15FunctionGraph, name, kind string, value any, presence any) {
	p15OptionalDiagnosticStore(g, name, g.value("gettext", map[string]any{"WFTextActionText": "NONE"}))
	group := g.conditional(5, presence, "WFConditionalActionString", "0")
	var observation string
	descriptor := value.(map[string]any)["Value"]
	switch kind {
	case "text", "number":
		prefix := "TEXT["
		if kind == "number" {
			prefix = "NUMBER["
		}
		observation = g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText([]string{prefix, "]"}, descriptor)})
		p15OptionalDiagnosticStore(g, name, observation)
	case "bool":
		truth := g.conditional(4, value, "WFNumberValue", float64(1))
		p15OptionalDiagnosticStore(g, name, g.value("gettext", map[string]any{"WFTextActionText": "BOOL[true]"}))
		g.boundary(truth, 1, false)
		p15OptionalDiagnosticStore(g, name, g.value("gettext", map[string]any{"WFTextActionText": "BOOL[false]"}))
		g.boundary(truth, 2, false)
	case "empty_list", "list":
		count := g.value("count", map[string]any{"Input": value})
		refs := []any{p15OptionalDiagnosticRef(count)}
		chunks := []string{"LIST[", "]"}
		if kind == "list" {
			first := g.value("getitemfromlist", map[string]any{"WFInput": value, "WFItemSpecifier": "First Item"})
			second := g.value("getitemfromlist", map[string]any{"WFInput": value, "WFItemSpecifier": "Last Item"})
			refs = append(refs, p15OptionalDiagnosticRef(first), p15OptionalDiagnosticRef(second))
			chunks = []string{"LIST[", "|", "|", "]"}
		}
		observation = g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText(chunks, refs...)})
		p15OptionalDiagnosticStore(g, name, observation)
	case "empty_map", "map":
		keys := g.value("getvalueforkey", map[string]any{"WFInput": value, "WFGetDictionaryValueType": "All Keys"})
		count := g.value("count", map[string]any{"Input": p15FunctionEdge(keys)})
		refs := []any{p15OptionalDiagnosticRef(count)}
		chunks := []string{"MAP[", "]"}
		if kind == "map" {
			item := g.value("getvalueforkey", map[string]any{"WFInput": value, "WFDictionaryKey": "key", "WFGetDictionaryValueType": "Value"})
			nativeType := g.value("getitemtype", map[string]any{"WFInput": value})
			refs = append(refs, p15OptionalDiagnosticRef(item), p15OptionalDiagnosticRef(nativeType))
			chunks = []string{"MAP[", "|", "|", "]"}
		}
		observation = g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText(chunks, refs...)})
		p15OptionalDiagnosticStore(g, name, observation)
	default:
		g.err = fmt.Errorf("unknown authored diagnostic observation %s", kind)
	}
	g.boundary(group, 2, false)
}

func p15OptionalDiagnosticGraphError(actions []map[string]any) error {
	g := &p15FunctionGraph{actual: actions, groups: map[string]bool{}, workflowName: p15OptionalDiagnostic}
	functions := []string{"optionalText", "optionalBool", "optionalNumber", "optionalList", "optionalMap", "optionalPick"}
	envelope := g.header(functions[0])
	for i, name := range functions {
		if i > 0 {
			g.nextFunction(name)
		}
		value, presence := p15ForwardParameterGraph(g, envelope, 0, i == 0)
		if name == "optionalPick" {
			g.get("arg_1", envelope)
		}
		p15ForwardResultGraph(g, value, presence)
	}
	g.caller()
	g.emit("comment", map[string]any{"WFCommentActionText": "ANCHOR_CARD_01_TOP_HEADER"})
	g.value("nothing", map[string]any{"UUID": "alpha-only"})
	g.emit("setvariable", map[string]any{"WFVariableName": "__cherri_debug_log"})
	p15OptionalDiagnosticTrace(g, "before", "setvariable")
	names := []string{"noneObserved", "emptyTextObserved", "falseObserved", "zeroObserved", "emptyListObserved", "emptyMapObserved"}
	kinds := []string{"text", "text", "bool", "number", "empty_list", "empty_map"}
	fnNames := []string{"optionalText", "optionalText", "optionalBool", "optionalNumber", "optionalList", "optionalMap"}
	for i, tc := range p15FunctionOptionalCases()[:6] {
		flag := "0"
		if tc.valuePresent {
			flag = "1"
		}
		value, presence := p15ForwardCallGraph(g, fnNames[i], tc.value, flag, nil, nil)
		value, presence = p15ForwardCallGraph(g, fnNames[i], p15FunctionArgument{true, 0, p15FunctionEdge(value)}, p15FunctionEdge(presence), nil, nil)
		p15OptionalDiagnosticObservation(g, names[i], kinds[i], p15FunctionEdge(value), p15FunctionEdge(presence))
	}
	list := p15FunctionArgument{true, 2, map[string]any{"WFSerializationType": "WFArrayParameterState", "Value": []any{
		map[string]any{"WFItemType": 0, "WFValue": p15FunctionText("A")}, map[string]any{"WFItemType": 0, "WFValue": p15FunctionText("B")}}}}
	value, presence := p15ForwardCallGraph(g, "optionalList", list, "1", nil, nil)
	p15OptionalDiagnosticObservation(g, "listObserved", "list", p15FunctionEdge(value), p15FunctionEdge(presence))
	dictionary := p15FunctionArgument{true, 1, map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{
		map[string]any{"WFKey": p15FunctionText("key"), "WFItemType": 0, "WFValue": p15FunctionText("VALUE")}}}}}
	value, presence = p15ForwardCallGraph(g, "optionalMap", dictionary, "1", nil, nil)
	p15OptionalDiagnosticObservation(g, "mapObserved", "map", p15FunctionEdge(value), p15FunctionEdge(presence))
	flag := p15MutableOptionalAssignGraph(g, "", false, p15MutableOptionalPresentValues()[0])
	oldPresence := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable(flag)})
	oldValue := g.value("getvariable", map[string]any{"WFVariable": p15FunctionVariable("candidate")})
	condition := g.value("number", map[string]any{"WFNumberActionNumber": float64(1)})
	writeGroup := g.conditional(4, p15FunctionEdge(condition), "WFNumberValue", float64(1))
	p15MutableOptionalAssignGraph(g, flag, true, p15MutableOptionalPresentValues()[0])
	g.value("gettext", map[string]any{"WFTextActionText": "WRITTEN"})
	g.boundary(writeGroup, 1, false)
	g.value("gettext", map[string]any{"WFTextActionText": "UNUSED"})
	later := g.boundary(writeGroup, 2, true)
	value, presence = p15ForwardCallGraph(g, "optionalPick", p15FunctionArgument{true, 0, p15FunctionEdge(oldValue)}, p15FunctionEdge(oldPresence), nil, []p15FunctionArgument{{true, 0, p15FunctionEdge(later)}})
	p15OptionalDiagnosticObservation(g, "snapshotObserved", "text", p15FunctionEdge(value), p15FunctionEdge(presence))
	p15OptionalDiagnosticObservation(g, "writtenObserved", "text", p15FunctionVariable("candidate"), p15FunctionVariable(flag))
	result := g.call(g.payload("optionalText", p15FunctionArgument{}))
	value, presence = p15DecodeOptionalGraph(g, result, "has_value", "value")
	p15OptionalDiagnosticObservation(g, "defaultObserved", "text", p15FunctionEdge(value), p15FunctionEdge(presence))
	g.emit("comment", map[string]any{"WFCommentActionText": "ANCHOR_CARD_15_MID_CENTER"})
	p15OptionalDiagnosticTrace(g, "after", "comment")
	trace := p15OptionalDiagnosticTraceExport(g)
	variables := append(append([]string{}, names...), "listObserved", "mapObserved", "snapshotObserved", "writtenObserved", "defaultObserved")
	chunks := []string{}
	refs := []any{}
	for i, field := range p15OptionalDiagnosticFields() {
		prefix := ":"
		if i == 0 {
			prefix = p15OptionalDiagnosticPrefix
		}
		chunks = append(chunks, prefix+field[0]+"=")
		refs = append(refs, p15OptionalDiagnosticVar(variables[i]))
	}
	chunks = append(chunks, "\n", "")
	refs = append(refs, p15OptionalDiagnosticRef(trace))
	summary := g.value("gettext", map[string]any{"WFTextActionText": p15OptionalDiagnosticText(chunks, refs...)})
	g.emit("comment", map[string]any{"WFCommentActionText": "ANCHOR_CARD_28_BOTTOM_FOOTER"})
	g.output(summary)
	return g.finish()
}

func TestP15OptionalNativeDiagnosticSourceAndGraph(t *testing.T) {
	actions, source := p15CompileDiagnosticGraph(t, p15OptionalDiagnostic)
	data, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + p15OptionalDiagnostic + ".expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = p15OptionalDiagnosticContractError(data, source); err != nil {
		t.Fatal(err)
	}
	if err = p15OptionalDiagnosticGraphError(actions); err != nil {
		// Preserve the concrete draft mismatch for inspection; this is no
		// runtime evidence and cannot be consumed by the formal registrar.
		if os.MkdirAll("dist/p15-optional-diagnostic", 0700) == nil {
			if raw, e := json.MarshalIndent(actions, "", "  "); e == nil {
				_ = os.WriteFile("dist/p15-optional-diagnostic/draft-graph.json", raw, 0600)
			}
		}
		t.Fatal(err)
	}
	t.Logf("%d independently checked native actions; Apple runtime NOT_RUN; formal_requirements_proved=[]", len(actions))
}

func TestP15OptionalNativeDiagnosticRejectsContractDrift(t *testing.T) {
	source, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + p15OptionalDiagnostic + ".cherri")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("tools/shortcuts-simulator-qa/fixtures/" + p15OptionalDiagnostic + ".expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = p15OptionalDiagnosticContractError(raw, string(source)); err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"source_rebound", "none_as_empty", "false_as_none", "zero_as_none", "empty_list_as_none", "empty_map_as_none", "list_order", "map_payload", "later_write_snapshot", "omitted_default", "missing_field", "reordered_field", "trace_id", "runtime_credit", "formal_credit", "scope", "unknown_field", "trailing_json"} {
		t.Run(fault, func(t *testing.T) {
			var c map[string]any
			if err := json.Unmarshal(raw, &c); err != nil {
				t.Fatal(err)
			}
			candidate := string(source)
			fields := c["expected_fields"].(map[string]any)
			switch fault {
			case "source_rebound":
				candidate = strings.Replace(candidate, "optionalText(none)", "optionalText(\"\")", 1)
				c["source_sha256"] = fmt.Sprintf("%x", sha256.Sum256([]byte(candidate)))
			case "none_as_empty":
				fields["NONE"] = "TEXT[]"
			case "false_as_none":
				fields["FALSE"] = "NONE"
			case "zero_as_none":
				fields["ZERO"] = "NONE"
			case "empty_list_as_none":
				fields["EMPTY_LIST"] = "NONE"
			case "empty_map_as_none":
				fields["EMPTY_MAP"] = "NONE"
			case "list_order":
				fields["LIST"] = "LIST[2|B|A]"
			case "map_payload":
				fields["MAP"] = "MAP[1|WRONG|Dictionary]"
			case "later_write_snapshot":
				fields["SNAPSHOT"] = "TEXT[]"
			case "omitted_default":
				fields["DEFAULT"] = "NONE"
			case "missing_field":
				delete(fields, "WRITTEN")
			case "reordered_field":
				order := c["field_order"].([]any)
				order[0], order[1] = order[1], order[0]
			case "trace_id":
				c["trace_run_id"] = "unrelated"
			case "runtime_credit":
				c["runtime_status"] = "PASSED"
			case "formal_credit":
				c["formal_requirements_proved"] = []string{"FN05"}
			case "scope":
				c["scope"] = "general Optional acceptance"
			case "unknown_field":
				c["passed"] = true
			}
			// Reconciling derivative summary/hash cannot rewrite the independent
			// expected semantic values, their order, source or acceptance scope.
			parts := []string{}
			for _, k := range c["field_order"].([]any) {
				key := k.(string)
				if v, ok := fields[key]; ok {
					parts = append(parts, key+"="+v.(string))
				}
			}
			summary := c["diagnostic_prefix"].(string) + strings.Join(parts, ":")
			c["expected_summary"] = summary
			c["expected_summary_sha256"] = fmt.Sprintf("%x", sha256.Sum256([]byte(summary)))
			bad, err := json.Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if fault == "trailing_json" {
				bad = append(bad, []byte(" {}")...)
			}
			if p15OptionalDiagnosticContractError(bad, candidate) == nil {
				t.Fatal("changed diagnostic contract was accepted")
			}
		})
	}
}

func p15OptionalDiagnosticMutate(actions []map[string]any, fault string) ([]map[string]any, bool) {
	if strings.HasPrefix(fault, "forward/") {
		return p15MutateOptionalForwardingGraph(actions, strings.TrimPrefix(fault, "forward/"))
	}
	var visit func(any) bool
	visit = func(value any) bool {
		switch v := value.(type) {
		case map[string]any:
			if fault == "empty_list_omitted" && v["WFSerializationType"] == "WFArrayParameterState" {
				if rows, ok := v["Value"].([]any); ok && len(rows) == 0 {
					delete(v, "Value")
					return true
				}
			}
			if fault == "list_order" && v["WFSerializationType"] == "WFArrayParameterState" {
				if rows, ok := v["Value"].([]any); ok && len(rows) == 2 {
					raw, _ := json.Marshal(rows[0])
					if strings.Contains(string(raw), `"string":"A"`) {
						rows[0], rows[1] = rows[1], rows[0]
						return true
					}
				}
			}
			if fault == "map_payload" && v["string"] == "VALUE" {
				v["string"] = "WRONG"
				return true
			}
			for _, child := range v {
				if visit(child) {
					return true
				}
			}
		case []any:
			for _, child := range v {
				if visit(child) {
					return true
				}
			}
		}
		return false
	}
	if fault == "empty_list_omitted" || fault == "list_order" || fault == "map_payload" {
		return actions, visit(p15OptionalDiagnosticActionsToAny(actions))
	}
	for i, a := range actions {
		p := p15Param(a)
		id, _ := a["WFWorkflowActionIdentifier"].(string)
		switch fault {
		case "default_uses_value_presence":
			if p["WFDictionaryKey"] == "has_arg_0" {
				p["WFDictionaryKey"] = "has_value"
				return actions, true
			}
		case "first_item":
			if p["WFItemSpecifier"] == "First Item" {
				p["WFItemSpecifier"] = "Last Item"
				return actions, true
			}
		case "dictionary_keys":
			if p["WFGetDictionaryValueType"] == "All Keys" {
				p["WFGetDictionaryValueType"] = "All Values"
				return actions, true
			}
		case "map_type_input":
			if id == "is.workflow.actions.getitemtype" {
				p["WFInput"] = p15FunctionVariable("mapObserved")
				return actions, true
			}
		case "snapshot_reads_data":
			if id == "is.workflow.actions.getvariable" {
				wire, ok := p["WFVariable"].(map[string]any)
				if ok {
					descriptor, ok := wire["Value"].(map[string]any)
					if ok {
						n, _ := descriptor["VariableName"].(string)
						if strings.HasPrefix(n, "_cherri_optional_presence_") {
							p["WFVariable"] = p15FunctionVariable("candidate")
							return actions, true
						}
					}
				}
			}
		case "wrong_workflow":
			if id == "is.workflow.actions.runworkflow" {
				p["WFWorkflowName"] = "unrelated"
				return actions, true
			}
		case "trace_store":
			if id == "is.workflow.actions.appendvariable" && p["WFVariableName"] == "__cherri_qa_trace" {
				p["WFVariableName"] = "noneObserved"
				return actions, true
			}
		case "trace_combine":
			if id == "is.workflow.actions.text.combine" {
				p["text"] = p15FunctionVariable("noneObserved")
				return actions, true
			}
		case "clear_stale_input":
			if id == "is.workflow.actions.setvariable" && p["WFVariableName"] == "__cherri_debug_log" {
				p["WFInput"] = p15FunctionVariable("__cherri_debug_log")
				return actions, true
			}
		case "anchor":
			if p["WFCommentActionText"] == "ANCHOR_CARD_15_MID_CENTER" {
				p["WFCommentActionText"] = "UNREVIEWED"
				return actions, true
			}
		case "summary_constant":
			if p["CustomOutputName"] == "optionalSummary" {
				p["WFTextActionText"] = "all-good"
				return actions, true
			}
		case "summary_edge", "summary_offset":
			if p["CustomOutputName"] == "optionalSummary" {
				v := p["WFTextActionText"].(map[string]any)["Value"].(map[string]any)
				refs := v["attachmentsByRange"].(map[string]any)
				if fault == "summary_edge" {
					refs["{46, 1}"] = p15OptionalDiagnosticVar("emptyTextObserved")
				} else {
					refs["{47, 1}"] = refs["{46, 1}"]
					delete(refs, "{46, 1}")
				}
				return actions, true
			}
		case "output_bypass":
			if id == "is.workflow.actions.output" && i == len(actions)-2 {
				p["WFOutput"] = p15FunctionText("FORGED")
				return actions, true
			}
		}
		if strings.HasPrefix(fault, "trace_") {
			wire, ok := p["WFTextActionText"].(map[string]any)
			if !ok {
				continue
			}
			v, ok := wire["Value"].(map[string]any)
			if !ok {
				continue
			}
			text, _ := v["string"].(string)
			switch fault {
			case "trace_stage":
				if strings.Contains(text, `"stage":"before"`) {
					v["string"] = strings.Replace(text, `"stage":"before"`, `"stage":"after"`, 1)
					return actions, true
				}
			case "trace_count":
				if strings.Contains(text, `"stage":"before"`) {
					v["string"] = strings.Replace(text, fmt.Sprintf(`"stepSequence":%d`, i), `"stepSequence":0`, 1)
					return actions, true
				}
			case "trace_timestamp":
				if strings.Contains(text, `"stage":"before"`) {
					refs := v["attachmentsByRange"].(map[string]any)
					for k := range refs {
						refs[k] = map[string]any{"Type": "Variable", "VariableName": "noneObserved"}
						return actions, true
					}
				}
			case "trace_run_id":
				if strings.Contains(text, `"stage":"collector_end"`) {
					v["string"] = strings.ReplaceAll(text, p15OptionalDiagnostic, "unrelated")
					return actions, true
				}
			}
		}
	}
	return actions, false
}

// The walk above changes maps in the original graph without copying them.
func p15OptionalDiagnosticActionsToAny(actions []map[string]any) []any {
	result := make([]any, len(actions))
	for i, a := range actions {
		result[i] = a
	}
	return result
}

func TestP15OptionalNativeDiagnosticRejectsGraphFaults(t *testing.T) {
	good, _ := p15CompileDiagnosticGraph(t, p15OptionalDiagnostic)
	if err := p15OptionalDiagnosticGraphError(good); err != nil {
		t.Fatal("unchanged source must pass before negative controls", err)
	}
	faults := []string{"default_uses_value_presence", "empty_list_omitted", "list_order", "map_payload", "first_item", "dictionary_keys", "map_type_input", "snapshot_reads_data", "wrong_workflow", "trace_store", "trace_combine", "clear_stale_input", "anchor", "summary_constant", "summary_edge", "summary_offset", "output_bypass", "trace_stage", "trace_count", "trace_timestamp", "trace_run_id"}
	for _, f := range []string{"carrier_marker", "value_flag", "flag_type", "empty_text_alias", "argument_omission", "carrier_edge", "decode_value_as_presence", "truthiness", "none_as_text", "wrong_join", "call_input", "duplicate_call"} {
		faults = append(faults, "forward/"+f)
	}
	for _, fault := range faults {
		t.Run(fault, func(t *testing.T) {
			bad := p15ContainerCloneNative(t, good)
			if err := p15OptionalDiagnosticGraphError(bad); err != nil {
				t.Fatal("unchanged clone must pass", err)
			}
			original, _ := json.Marshal(bad)
			bad, changed := p15OptionalDiagnosticMutate(bad, fault)
			if !changed {
				t.Fatal("missing precise mutation target")
			}
			mutated, _ := json.Marshal(bad)
			if bytes.Equal(original, mutated) {
				t.Fatal("mutation did not change actual native bytes")
			}
			if p15OptionalDiagnosticGraphError(bad) == nil {
				t.Fatal("causal diagnostic graph corruption accepted")
			}
		})
	}
}
