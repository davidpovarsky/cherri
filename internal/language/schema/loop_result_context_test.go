package schema

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"testing"
)

func explicitContextCall(t *testing.T, source string) *syntax.CallExpr {
	t.Helper()
	program := scopedPlanProgram(t, source)
	return program.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr)
}

func TestLoopResultExplicitContextArgumentIdentity(t *testing.T) {
	for _, tc := range []struct {
		name, source, action string
		count                int
	}{
		{"comment_primary", "comment(\"bound\")", "comment", 1},
		{"comment_named", "comment(text:\"bound\")", "comment", 1},
		{"text_primary", "text(\"bound\")", "text", 1},
		{"text_named", "text(text:\"bound\")", "text", 1},
		{"output_primary", "output(\"bound\")", "output", 1},
		{"output_named", "output(output:\"bound\")", "output", 1},
		{"clear_logs", "debugClearLogs()", "debugClearLogs", 0},
		{"trace_primary", "debugTraceEvent(\"before\")", "debugTraceEvent", 1},
		{"trace_named", "debugTraceEvent(label:\"bound\",stage:\"before\")", "debugTraceEvent", 2},
		{"trace_export_default", "debugExportTrace()", "debugExportTrace", 0},
		{"trace_export_primary", "debugExportTrace(\"bound\")", "debugExportTrace", 1},
		{"trace_export_named", "debugExportTrace(runId:\"bound\")", "debugExportTrace", 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program := scopedPlanProgram(t, tc.source)
			call := program.Statements[0].(*syntax.ExprStmt).Expr.(*syntax.CallExpr)
			action, found := DefaultRegistry().LookupAction(tc.action)
			if !found {
				t.Fatal("canonical action missing")
			}
			before := syntax.Format(program)
			arguments, okay := LoopResultExplicitContextArguments(call, action)
			if !okay || len(arguments) != tc.count {
				t.Fatalf("explicit source arguments not admitted: %v %d", okay, len(arguments))
			}
			var actual []syntax.Expression
			if call.PrimaryArg != nil {
				actual = append(actual, call.PrimaryArg)
			}
			for _, named := range call.NamedArgs {
				actual = append(actual, named.Value)
			}
			for index, argument := range arguments {
				if argument != actual[index] {
					t.Fatal("source argument identity replaced")
				}
			}
			if syntax.Format(program) != before {
				t.Fatal("source text changed")
			}
		})
	}
}

func TestLoopResultExplicitContextExactDescriptors(t *testing.T) {
	call := explicitContextCall(t, "text(\"bound\")")
	original, _ := DefaultRegistry().LookupAction("text")
	if _, okay := LoopResultExplicitContextArguments(call, original); !okay {
		t.Fatal("canonical text rejected")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*ActionSchema)
	}{
		{"id", func(a *ActionSchema) { a.ID = "foreign" }},
		{"callable_name", func(a *ActionSchema) { a.CallableName = "foreign" }},
		{"module", func(a *ActionSchema) { a.Module = "foreign" }},
		{"native_id", func(a *ActionSchema) { a.AppleIdentifier = "example.foreign" }},
		{"variant", func(a *ActionSchema) { a.Variant = "foreign" }},
		{"intent", func(a *ActionSchema) { a.AppIntent = &AppIntentDescriptor{Name: "foreign"} }},
		{"static", func(a *ActionSchema) { a.StaticParameters = map[string]any{"WFInput": "foreign"} }},
		{"emitted_keys", func(a *ActionSchema) { a.EmittedKeys = []string{"WFInput"} }},
		{"builtin", func(a *ActionSchema) { a.Builtin = true }},
		{"custom", func(a *ActionSchema) { a.Custom = false }},
		{"construct", func(a *ActionSchema) { a.CompilerConstruct = true }},
		{"scoped_block", func(a *ActionSchema) { a.ScopedBlock = true }},
		{"scoped_optional", func(a *ActionSchema) { a.ScopedBlockOptional = true }},
		{"platform", func(a *ActionSchema) { a.PlatformVersions = map[string]PlatformVersionBounds{"iOS": {}} }},
		{"minimum", func(a *ActionSchema) { a.MinVersion = 1 }},
		{"maximum", func(a *ActionSchema) { a.MaxVersion = 1 }},
		{"mac_only", func(a *ActionSchema) { a.MacOnly = true }},
		{"non_mac_only", func(a *ActionSchema) { a.NonMacOnly = true }},
		{"output_type", func(a *ActionSchema) { a.OutputType = types.AnyContent }},
		{"nil_output", func(a *ActionSchema) { a.OutputType = nil }},
		{"output_name", func(a *ActionSchema) { a.OutputTypeName = "foreign" }},
		{"raw_output", func(a *ActionSchema) { a.RawOutputType = "foreign" }},
		{"primary", func(a *ActionSchema) { a.PrimaryParameterID = "foreign" }},
		{"parameter_count", func(a *ActionSchema) { a.Parameters = nil }},
		{"parameter_id", func(a *ActionSchema) { a.Parameters[0].ID = "foreign" }},
		{"parameter_label", func(a *ActionSchema) { a.Parameters[0].Label = "foreign" }},
		{"parameter_type", func(a *ActionSchema) { a.Parameters[0].Type = types.AnyContent }},
		{"parameter_type_name", func(a *ActionSchema) { a.Parameters[0].TypeName = "foreign" }},
		{"parameter_raw_type", func(a *ActionSchema) { a.Parameters[0].RawType = "foreign" }},
		{"wire", func(a *ActionSchema) { a.Parameters[0].WireKey = "WFInput" }},
		{"codec", func(a *ActionSchema) { a.Parameters[0].Codec = "token" }},
		{"optional", func(a *ActionSchema) { a.Parameters[0].Optional = true }},
		{"reference", func(a *ActionSchema) { a.Parameters[0].Reference = true }},
		{"literal", func(a *ActionSchema) { a.Parameters[0].Literal = true }},
		{"variadic", func(a *ActionSchema) { a.Parameters[0].Variadic = true }},
		{"default", func(a *ActionSchema) { a.Parameters[0].DefaultValue = StringDefault("foreign") }},
		{"omission", func(a *ActionSchema) { a.Parameters[0].OmissionPolicy = OmissionPolicy("foreign") }},
		{"accepted_forms", func(a *ActionSchema) { a.Parameters[0].AcceptedForms = []string{"attachment(output)"} }},
		{"enum_name", func(a *ActionSchema) { a.Parameters[0].EnumName = "foreign" }},
		{"enum_values", func(a *ActionSchema) { a.Parameters[0].EnumValues = []string{"foreign"} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := cloneActionSchema(original)
			tc.mutate(a)
			if _, okay := LoopResultExplicitContextArguments(call, a); okay {
				t.Fatal("changed native descriptor gained private-storage context admission")
			}
		})
	}
}

func TestLoopResultExplicitContextTraceDefaults(t *testing.T) {
	for _, tc := range []struct {
		name, action, source string
		mutate               func(*ActionSchema)
	}{
		{"label_default", "debugTraceEvent", "debugTraceEvent(stage:\"before\")", func(a *ActionSchema) { a.Parameters[1].DefaultValue = StringDefault("foreign") }},
		{"label_omission", "debugTraceEvent", "debugTraceEvent(stage:\"before\")", func(a *ActionSchema) { a.Parameters[1].OmissionPolicy = OmitPolicyEmit }},
		{"label_required", "debugTraceEvent", "debugTraceEvent(stage:\"before\")", func(a *ActionSchema) { a.Parameters[1].Optional = false }},
		{"trace_emitted", "debugTraceEvent", "debugTraceEvent(stage:\"before\")", func(a *ActionSchema) { a.EmittedKeys = []string{"WFInput"} }},
		{"export_default", "debugExportTrace", "debugExportTrace()", func(a *ActionSchema) { a.Parameters[0].DefaultValue = StringDefault("foreign") }},
		{"export_wire", "debugExportTrace", "debugExportTrace()", func(a *ActionSchema) { a.Parameters[0].WireKey = "WFInput" }},
		{"clear_parameters", "debugClearLogs", "debugClearLogs()", func(a *ActionSchema) {
			a.Parameters = []ParameterSchema{{ID: "input", Label: "input", Type: types.Text}}
		}},
		{"clear_output", "debugClearLogs", "debugClearLogs()", func(a *ActionSchema) { a.OutputType = types.Text }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := explicitContextCall(t, tc.source)
			original, _ := DefaultRegistry().LookupAction(tc.action)
			if _, okay := LoopResultExplicitContextArguments(call, original); !okay {
				t.Fatal("canonical trace contract rejected")
			}
			a := cloneActionSchema(original)
			tc.mutate(a)
			if _, okay := LoopResultExplicitContextArguments(call, a); okay {
				t.Fatal("changed trace/default/implicit-input contract was admitted")
			}
		})
	}
}

func TestLoopResultExplicitContextMalformedCalls(t *testing.T) {
	for _, tc := range []struct{ name, source, action string }{
		{"alias", "other(\"bound\")", "text"},
		{"omitted_text", "text()", "text"},
		{"omitted_output", "output()", "output"},
		{"duplicate_primary_named", "text(\"first\",text:\"again\")", "text"},
		{"duplicate_named", "text(text:\"first\",text:\"again\")", "text"},
		{"unknown_label", "text(foreign:\"bound\")", "text"},
		{"trace_missing_stage", "debugTraceEvent(label:\"bound\")", "debugTraceEvent"},
		{"clear_extra_input", "debugClearLogs(\"bound\")", "debugClearLogs"},
		{"export_unknown_label", "debugExportTrace(foreign:\"bound\")", "debugExportTrace"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := explicitContextCall(t, tc.source)
			a, _ := DefaultRegistry().LookupAction(tc.action)
			if _, okay := LoopResultExplicitContextArguments(call, a); okay {
				t.Fatal("unbound/ambiguous call admitted")
			}
		})
	}
	for _, fault := range []string{"nil_call", "nil_schema", "nil_named_value", "block", "nonidentifier_callee"} {
		t.Run(fault, func(t *testing.T) {
			c := explicitContextCall(t, "text(text:\"bound\")")
			a, _ := DefaultRegistry().LookupAction("text")
			switch fault {
			case "nil_call":
				c = nil
			case "nil_schema":
				a = nil
			case "nil_named_value":
				c.NamedArgs[0].Value = nil
			case "block":
				c.Block = &syntax.ScopedBlock{}
			case "nonidentifier_callee":
				c.Callee = &syntax.CallExpr{}
			}
			if _, okay := LoopResultExplicitContextArguments(c, a); okay {
				t.Fatal("malformed AST admitted")
			}
		})
	}
}

func TestLoopResultExplicitContextPlannerBoundaries(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"shadow_variable", "let text=\"shadow\"\nlet values=repeat 2 {yield []}\ntext(\"bound\")\nreturn count(values)"},
		{"shadow_function", "function text(value:Text)->Text{return value}\nlet values=repeat 2 {yield []}\ntext(\"bound\")\nreturn count(values)"},
		{"private_alias", "let values=repeat 2 {yield []}\nlet alias=values\ntext(alias)\nreturn count(values)"},
		{"private_named", "let values=repeat 2 {yield []}\ntext(text:values)\nreturn count(values)"},
		{"private_nested", "let values=repeat 2 {yield []}\ntext([values])\nreturn count(values)"},
		{"implicit_text", "let values=repeat 2 {yield []}\ntext()\nreturn count(values)"},
		{"implicit_output", "let values=repeat 2 {yield []}\noutput()\nreturn count(values)"},
		{"call_inside_producer", "let values=repeat 2 {text(\"bound\")\nyield []}\nreturn count(values)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program := scopedPlanProgram(t, tc.source)
			plan := PlanLoopResultLocals(program, DefaultRegistry())
			if tc.name == "call_inside_producer" {
				binding := program.Statements[0].(*syntax.BindingStmt)
				producer := binding.Value.(*syntax.RepeatStmt)
				// A plan may track the value origin without authorizing its body.
				// The body capability is the boundary used by analysis/lowering.
				if LoopResultLocalBody(producer) != nil || (plan != nil && plan.Bodies[producer.BodyBlock]) {
					t.Fatal("call-bearing producer gained private-cell body authority")
				}
				return
			}
			if plan != nil {
				t.Fatal("shadow or private dataflow boundary escaped")
			}
		})
	}
}
