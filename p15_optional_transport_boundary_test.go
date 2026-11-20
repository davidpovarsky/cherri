package main

import (
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/service"
)

type p15OptionalTransportCase struct{ name, source, target, context string }

func p15OptionalTransportCases() []p15OptionalTransportCase {
	const function = "function echo(value:Text?)->Text?{return value}\n"
	return []p15OptionalTransportCase{
		{"function_bound", function + "let candidate:Text?=none\ngetVariable(echo(candidate))", "candidate", "function argument"},
		{"function_mutable", function + "var candidate:Text?=none\ngetVariable(echo(candidate))", "candidate", "function argument"},
		{"function_result", function + "function absent()->Text?{return none}\ngetVariable(echo(absent()))", "absent()", "function argument"},
		{"function_forward", function + "function forward(candidate:Text?)->Text?{return echo(candidate)}\ngetVariable(forward(\"\"))", "candidate", "function argument"},
		{"map_literal", "let value={\"present\":none,\"empty\":\"\"}\ngetVariable(value)", "none", "Dictionary value"},
		{"map_nested", "let value={\"rows\":[{\"absent\":none}]}\ngetVariable(value)", "none", "Dictionary value"},
		{"list_literal", "let value=[false,none,\"\"]\ngetVariable(value)", "none", "List element"},
		{"map_array", "let value={\"rows\":[0,none,\"\"]}\ngetVariable(value)", "none", "List element"},
		{"map_bound", "let candidate:Text?=none\nlet value={\"present\":candidate}\ngetVariable(value)", "candidate", "Dictionary value"},
		{"list_bound", "let candidate:Text?=none\nlet value=[candidate,\"\"]\ngetVariable(value)", "candidate", "List element"},
		{"map_inferred_absence", "let candidate=none\nlet value={\"present\":candidate}\ngetVariable(value)", "candidate", "Dictionary value"},
		{"list_inferred_absence", "let candidate=none\nlet value=[candidate,\"\"]\ngetVariable(value)", "candidate", "List element"},
		{"map_key", "let value={none:\"value\"}\ngetVariable(value)", "none", "Dictionary key"},
		{"map_optional_key", "let candidate:Text?=none\nlet value={candidate:\"value\"}\ngetVariable(value)", "candidate", "Dictionary key"},
		{"map_value_branch", "let value={\"present\":if true{yield none}else{yield \"\"}}\ngetVariable(value)", "if true{yield none}else{yield \"\"}", "Dictionary value"},
		{"map_bound_branch", "let candidate=if true{yield none}else{yield \"\"}\nlet value={\"present\":candidate}\ngetVariable(value)", "candidate", "Dictionary value"},
		{"after_effect", "text(\"BEFORE_UNSUPPORTED_OPTIONAL\")\nlet value={\"present\":none}\ngetVariable(value)", "none", "Dictionary value"},
	}
}
func p15OptionalTransportDiagnostic(diagnostics []analysis.Diagnostic, tc p15OptionalTransportCase) error {
	var errors []analysis.Diagnostic
	for _, d := range diagnostics {
		if d.Severity == analysis.SeverityError {
			errors = append(errors, d)
		}
	}
	start := strings.LastIndex(tc.source, tc.target)
	if start < 0 || len(errors) != 1 {
		return fmt.Errorf("want one exact optional transport diagnostic, got %#v", errors)
	}
	d := errors[0]
	if d.Code != analysis.CodeTargetValueShape || d.Span.Start.ByteOffset != start || d.Span.End.ByteOffset != start+len(tc.target) ||
		!strings.Contains(d.Message, "current") || !strings.Contains(d.Message, tc.context) || !strings.Contains(d.Message, "optional absence") {
		return fmt.Errorf("optional transport code/span/context changed: %#v", d)
	}
	return nil
}
func TestP15OptionalTransportFailsPrecisely(t *testing.T) {
	for _, tc := range p15OptionalTransportCases() {
		if tc.context == "function argument" {
			continue
		} // exercised as positive forwarding routes
		t.Run(tc.name, func(t *testing.T) {
			diagnostics := p15TypeDiagnostics(t, tc.source)
			diagnosticErr := p15OptionalTransportDiagnostic(diagnostics, tc)
			if diagnosticErr != nil {
				t.Error(diagnosticErr)
			}
			raw, err := CompileSourceToPlist("p15-optional-transport.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				if len(raw) > 0 {
					t.Log("P15_OPTIONAL_TRANSPORT_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
				}
				t.Errorf("possibly absent value crossed lossy native transport: bytes=%d error=%v", len(raw), err)
			}
			svc := service.NewService(nil)
			uri := "file:///p15-optional-transport-" + tc.name + ".cherri"
			svc.OpenDocument(uri, 1, tc.source)
			editorDiagnostics, _ := svc.Analyze(uri)
			if err := p15OptionalTransportDiagnostic(editorDiagnostics, tc); err != nil {
				t.Error(err)
			}
			if diagnosticErr != nil {
				return
			}
			for _, fault := range []string{"code", "span", "context"} {
				t.Run("oracle/"+fault, func(t *testing.T) {
					bad := append([]analysis.Diagnostic(nil), diagnostics...)
					for i := range bad {
						if bad[i].Severity != analysis.SeverityError {
							continue
						}
						switch fault {
						case "code":
							bad[i].Code = "E_WRONG_CONTROL"
						case "span":
							bad[i].Span.Start.ByteOffset++
						case "context":
							bad[i].Message = "unsupported"
						}
					}
					if p15OptionalTransportDiagnostic(bad, tc) == nil {
						t.Fatal("damaged diagnostic accepted")
					}
				})
			}
		})
	}
}
func TestP15OptionalTransportKeepsPresentValues(t *testing.T) {
	for _, tc := range []struct {
		name, typ, literal, action, key string
		wantWire, wantProducer          any
	}{
		{"false", "Bool", "false", "is.workflow.actions.number", "WFNumberActionNumber", p15ContainerNumber("0"), float64(0)},
		{"zero", "Number", "0", "is.workflow.actions.number", "WFNumberActionNumber", p15ContainerNumber("0"), uint64(0)},
		{"empty", "Text", "\"\"", "is.workflow.actions.gettext", "WFTextActionText", "", ""},
	} {
		for _, route := range []string{"literal", "bound"} {
			t.Run(tc.name+"/"+route, func(t *testing.T) {
				source := "function echo(value:" + tc.typ + "?)->" + tc.typ + "?{return value}\n"
				argument := tc.literal
				if route == "bound" {
					source += "let supplied:" + tc.typ + "?=" + tc.literal + "\n"
					argument = "supplied"
				}
				source += "getVariable(echo(" + argument + "))"
				if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
					t.Fatal(diagnostics)
				}
				actions := p15CompiledActions(t, source)
				fields, err := p15FunctionContainerPayload(actions)
				if err != nil {
					t.Fatal(err)
				}
				want := tc.wantWire
				if route == "bound" {
					found := 0
					for _, action := range actions {
						p := p15Param(action)
						if p["CustomOutputName"] != "supplied" {
							continue
						}
						found++
						if action["WFWorkflowActionIdentifier"] != tc.action || !reflect.DeepEqual(p[tc.key], tc.wantProducer) {
							t.Fatal("present binding changed", action)
						}
						uuid, ok := p["UUID"].(string)
						if !ok || uuid == "" {
							t.Fatal("present binding lost identity")
						}
						want = p15ContainerReference{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "supplied"}
					}
					if found != 1 {
						t.Fatal("missing or duplicated present producer")
					}
				}
				inner, err := p15OptionalArgumentFields(actions, 0)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(inner["has_value"]["WFValue"], p15FunctionText("1")) {
					t.Fatal("present value became absent")
				}
				kind, wire := p15ContainerFieldValue(want)
				row := inner["value"]
				if row == nil || row["WFItemType"] != kind || !reflect.DeepEqual(row["WFValue"], wire) {
					t.Fatal("present argument lost native value", row, wire)
				}
				if !reflect.DeepEqual(fields["has_arg_0"]["WFValue"], p15ContainerText("1")) {
					t.Fatal("present value lost explicit argument presence")
				}
			})
		}
	}
}
func TestP15OptionalTransportKeepsNarrowedAndDefaultValues(t *testing.T) {
	t.Run("narrowed", func(t *testing.T) {
		source := "function echo(value:Text)->Text{return value}\nfunction forward(candidate:Text?)->Text{if candidate!=none{return echo(candidate)};return \"fallback\"}\ngetVariable(forward(\"\"))"
		if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		p15CompiledActions(t, source)
	})
	t.Run("omitted_none_default", func(t *testing.T) {
		source := "function echo(value:Text?=none)->Text?{return value}\ngetVariable(echo())"
		if diagnostics := p15TypeDiagnostics(t, source); len(diagnostics) != 0 {
			t.Fatal(diagnostics)
		}
		actions := p15CompiledActions(t, source)
		fields, err := p15FunctionContainerPayload(actions)
		if err != nil {
			t.Fatal(err)
		}
		if _, exists := fields["arg_0"]; exists || !reflect.DeepEqual(fields["has_arg_0"]["WFValue"], p15ContainerText("0")) {
			t.Fatal("omitted argument became explicit empty value")
		}
		// The new supplied-None branch and the omitted-None default each have
		// their own Nothing under different presence guards. Check the entire
		// graph instead of the historical single-Nothing physical count.
		if err := p15FunctionOptionalError(actions, p15FunctionOptionalCase{typ: "Text", fallback: "none"}); err != nil {
			t.Fatal("None default/value presence graph changed", err)
		}
	})
}
func p15LazyKeyError(actions []map[string]any, nested bool) error {
	if len(actions) != 3 || actions[0]["WFWorkflowActionIdentifier"] != "com.example.p15.key" ||
		actions[1]["WFWorkflowActionIdentifier"] != "is.workflow.actions.dictionary" || actions[2]["WFWorkflowActionIdentifier"] != "is.workflow.actions.getvariable" {
		return fmt.Errorf("key producer, Dictionary and final consumer order/cardinality changed")
	}
	p := p15Param(actions[0])
	uuid, ok := p["UUID"].(string)
	if !ok || uuid == "" {
		return fmt.Errorf("lazy key producer has no concrete identity")
	}
	for key := range p {
		if key != "UUID" && key != "CustomOutputName" {
			return fmt.Errorf("raw key acquired unauthored parameters")
		}
	}
	// An authored native action without an output name produces an intentionally
	// nameless attachment. A later binding label must not invent an OutputName.
	ref := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid}
	row := map[string]any{"WFItemType": uint64(0), "WFKey": p15ComputedKeyText(ref, "", "", 0), "WFValue": p15ContainerText("P15_LAZY_KEY")}
	wrap := func(row map[string]any) map[string]any {
		return map[string]any{"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": []any{row}}}
	}
	want := wrap(row)
	if nested {
		want = wrap(map[string]any{"WFItemType": uint64(1), "WFKey": p15ContainerText("nested"), "WFValue": want})
	}
	if !reflect.DeepEqual(p15Param(actions[1])["WFItems"], want) {
		return fmt.Errorf("lazy key typed payload changed: actual=%#v want=%#v", p15Param(actions[1])["WFItems"], want)
	}
	dictionaryID, _ := p15Param(actions[1])["UUID"].(string)
	if dictionaryID == "" || dictionaryID == uuid {
		return fmt.Errorf("Dictionary identity absent or aliased")
	}
	expected := map[string]any{"Type": "ActionOutput", "OutputUUID": dictionaryID}
	if name, exists := p15Param(actions[1])["CustomOutputName"]; exists {
		expected["OutputName"] = name
	}
	if !reflect.DeepEqual(p15Param(actions[2])["WFVariable"], p15FunctionAttachment(expected)) {
		return fmt.Errorf("final consumer lost Dictionary output")
	}
	return nil
}
func TestP15ComputedKeysResolveLazyNativeActions(t *testing.T) {
	for _, nested := range []bool{false, true} {
		t.Run(fmt.Sprint(nested), func(t *testing.T) {
			literal := "{native.action(\"com.example.p15.key\",{}):\"P15_LAZY_KEY\"}"
			if nested {
				literal = "{\"nested\":" + literal + "}"
			}
			source := "let value=" + literal + "\ngetVariable(value)"
			actions := p15CompiledActions(t, source)
			if err := p15LazyKeyError(actions, nested); err != nil {
				t.Fatal(err)
			}
			for _, fault := range []string{"missing_uuid", "wrong_reference", "missing_range", "literal_key", "changed_value", "duplicate_producer"} {
				t.Run(fault, func(t *testing.T) {
					bad := p15ContainerCloneNative(t, actions)
					if err := p15LazyKeyError(bad, nested); err != nil {
						t.Fatalf("unchanged native clone must pass before mutation: %v", err)
					}
					switch fault {
					case "missing_uuid":
						delete(p15Param(bad[0]), "UUID")
					case "duplicate_producer":
						bad = append([]map[string]any{bad[0]}, bad...)
					default:
						rows := p15ComputedMarkerRows(p15Param(bad[1])["WFItems"], "P15_LAZY_KEY")
						if len(rows) != 1 {
							t.Fatal("mutation target missing")
						}
						row := rows[0]
						switch fault {
						case "wrong_reference":
							key := row["WFKey"].(map[string]any)["Value"].(map[string]any)
							key["attachmentsByRange"].(map[string]any)["{0, 1}"].(map[string]any)["OutputUUID"] = "11111111-1111-4111-8111-111111111111"
						case "missing_range":
							delete(row["WFKey"].(map[string]any)["Value"].(map[string]any), "attachmentsByRange")
						case "literal_key":
							row["WFKey"] = p15ContainerText("not a reference")
						case "changed_value":
							row["WFValue"] = p15ContainerText("CORRUPTED")
						}
					}
					if p15LazyKeyError(bad, nested) == nil {
						t.Fatal("damaged lazy native-key identity accepted")
					}
				})
			}
		})
	}
}
