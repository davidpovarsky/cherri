package schema

import (
	"testing"

	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func scopedPlanProgram(t *testing.T, text string) *syntax.Program {
	t.Helper()
	p := syntax.NewParser(source.NewFile("scoped-plan.cherri", "scoped-plan.cherri", 1, text))
	program := p.ParseProgram()
	if len(p.Errors()) != 0 {
		t.Fatal(p.Errors())
	}
	return program
}

func TestScopedMeasureLoopPlanKeepsDeclarationAndBodyIdentity(t *testing.T) {
	program := scopedPlanProgram(t, "var values=repeat 0 {yield []}\nlet size=measure(label:\"scope\") {let before=values\nvalues=repeat 2 {yield [\"x\",\"y\"]}\nlet values=repeat 1 {yield none}\ncount(values)}\nreturn count(values)")
	before := syntax.Format(program)
	outer := program.Statements[0].(*syntax.BindingStmt)
	call := program.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
	assignment := call.Block.Body.Statements[1].(*syntax.AssignStmt)
	inner := call.Block.Body.Statements[2].(*syntax.BindingStmt)
	plan := PlanLoopResultLocals(program, DefaultRegistry())
	if plan == nil || plan.ScopedBodies[call] != call.Block.Body ||
		plan.Assignments[assignment] != outer || !plan.MutableBindings[outer] ||
		!plan.BindingValueSourceMatches(outer) || !plan.BindingValueSourceMatches(inner) {
		t.Fatal("captured write, lexical shadow or actual body lost source ownership")
	}
	for _, binding := range []*syntax.BindingStmt{outer, inner} {
		body := binding.Value.(*syntax.RepeatStmt).BodyBlock
		if !plan.Bodies[body] || !plan.BindingValues[binding.Value] {
			t.Fatal("stored or consumed source origin was not admitted")
		}
		clone := *binding
		if plan.BindingValueSourceMatches(&clone) {
			t.Fatal("same-text declaration acquired cell authority")
		}
	}
	clone := *call
	if plan.ScopedBodies[&clone] != nil {
		t.Fatal("same-text call acquired source body ownership")
	}
	if syntax.Format(program) != before {
		t.Fatal("planning mutated authored source")
	}
}

func TestScopedMeasureLoopPlanRetainsClosedBoundaries(t *testing.T) {
	for name, sourceText := range map[string]string{
		"escaping_cells":     "return measure(\"scope\"){let values=repeat 1 {yield []}\nvalues}",
		"implicit_action":    "return measure(\"scope\"){let values=repeat 1 {yield []}\ntext()\ncount(values)}",
		"opaque_action":      "return measure(\"scope\"){let values=repeat 1 {yield []}\nnative.action(\"example.raw\",{})\ncount(values)}",
		"user_action":        "function measure(label:Text)->Number{return 1}\nlet values=repeat 1 {yield []}\nreturn count(values)",
		"shadowed_construct": "let measure=\"shadow\"\nreturn measure(\"scope\"){let values=repeat 1 {yield []}\ncount(values)}",
	} {
		t.Run(name, func(t *testing.T) {
			if plan := PlanLoopResultLocals(scopedPlanProgram(t, sourceText), DefaultRegistry()); plan != nil {
				t.Fatal("unsupported action or escaping cells gained a private storage plan")
			}
		})
	}
}

func TestScopedMeasureExactLifecycleSchemaGuard(t *testing.T) {
	program := scopedPlanProgram(t, "return measure(\"scope\"){1}")
	call := program.Statements[0].(*syntax.ReturnStmt).Value.(*syntax.CallExpr)
	original, found := DefaultRegistry().LookupAction("measure")
	if !found || LoopResultScopedMeasureBody(call, original) != call.Block.Body {
		t.Fatal("canonical measure contract not recognized")
	}
	for name, mutate := range map[string]func(*ActionSchema){
		"id":                 func(a *ActionSchema) { a.ID = "foreign" },
		"name":               func(a *ActionSchema) { a.CallableName = "foreign" },
		"module":             func(a *ActionSchema) { a.Module = "foreign" },
		"native":             func(a *ActionSchema) { a.AppleIdentifier = "example.native" },
		"variant":            func(a *ActionSchema) { a.Variant = "other" },
		"intent":             func(a *ActionSchema) { a.AppIntent = &AppIntentDescriptor{Name: "other"} },
		"static":             func(a *ActionSchema) { a.StaticParameters = map[string]any{"WFInput": "other"} },
		"emitted_keys":       func(a *ActionSchema) { a.EmittedKeys = []string{"WFInput"} },
		"builtin":            func(a *ActionSchema) { a.Builtin = false },
		"custom":             func(a *ActionSchema) { a.Custom = true },
		"construct":          func(a *ActionSchema) { a.CompilerConstruct = false },
		"block":              func(a *ActionSchema) { a.ScopedBlock = false },
		"primary":            func(a *ActionSchema) { a.PrimaryParameterID = "input" },
		"output":             func(a *ActionSchema) { a.OutputType = types.Number },
		"nil_output":         func(a *ActionSchema) { a.OutputType = nil },
		"parameter_count":    func(a *ActionSchema) { a.Parameters = nil },
		"parameter_id":       func(a *ActionSchema) { a.Parameters[0].ID = "input" },
		"parameter_label":    func(a *ActionSchema) { a.Parameters[0].Label = "input" },
		"parameter_type":     func(a *ActionSchema) { a.Parameters[0].Type = types.AnyContent },
		"parameter_nil_type": func(a *ActionSchema) { a.Parameters[0].Type = nil },
		"literal":            func(a *ActionSchema) { a.Parameters[0].Literal = false },
		"optional":           func(a *ActionSchema) { a.Parameters[0].Optional = true },
		"reference":          func(a *ActionSchema) { a.Parameters[0].Reference = true },
		"variadic":           func(a *ActionSchema) { a.Parameters[0].Variadic = true },
		"default":            func(a *ActionSchema) { v := "scope"; a.Parameters[0].DefaultValue = &v },
		"wire_key":           func(a *ActionSchema) { a.Parameters[0].WireKey = "WFInput" },
		"codec":              func(a *ActionSchema) { a.Parameters[0].Codec = "token" },
		"enum_name":          func(a *ActionSchema) { a.Parameters[0].EnumName = "foreign" },
		"enum_values":        func(a *ActionSchema) { a.Parameters[0].EnumValues = []string{"scope"} },
		"accepted_forms":     func(a *ActionSchema) { a.Parameters[0].AcceptedForms = []string{"token"} },
	} {
		t.Run(name, func(t *testing.T) {
			a := *original
			a.Parameters = append([]ParameterSchema(nil), original.Parameters...)
			mutate(&a)
			if LoopResultScopedMeasureBody(call, &a) != nil {
				t.Fatal("altered lifecycle schema was trusted")
			}
		})
	}
	for name, text := range map[string]string{
		"named_label":     "return measure(label:\"scope\"){1}",
		"empty":           "return measure(\"\"){1}",
		"whitespace":      "return measure(\"  \"){1}",
		"no_label":        "return measure(){1}",
		"duplicate_label": "return measure(\"scope\",label:\"again\"){1}",
		"wrong_label":     "return measure(other:\"scope\"){1}",
		"dynamic_label":   "return measure(label){1}",
		"foreign_callee":  "return other(\"scope\"){1}",
	} {
		t.Run(name, func(t *testing.T) {
			p := scopedPlanProgram(t, text)
			c := p.Statements[0].(*syntax.ReturnStmt).Value.(*syntax.CallExpr)
			body := LoopResultScopedMeasureBody(c, original)
			if name == "named_label" {
				if body != c.Block.Body {
					t.Fatal("named literal label rejected")
				}
			} else if body != nil {
				t.Fatal("invalid source call was trusted")
			}
		})
	}
}

func TestScopedMeasureLoopPlanVisitsOrdinaryNestedExpressions(t *testing.T) {
	for name, expression := range map[string]string{
		"count":  "count(measure(\"ordinary\"){[1,2]})",
		"list":   "[measure(\"ordinary\"){1}]",
		"binary": "measure(\"ordinary\"){1}+2",
	} {
		for _, mode := range []string{"ordinary", "with_loop"} {
			t.Run(name+"/"+mode, func(t *testing.T) {
				text := "let other=" + expression + "\nreturn 1"
				if mode == "with_loop" {
					text = "let values=repeat 2 {yield none}\nlet other=" + expression + "\nreturn count(values)"
				}
				program := scopedPlanProgram(t, text)
				index := 0
				if mode == "with_loop" {
					index = 1
				}
				var call *syntax.CallExpr
				switch e := program.Statements[index].(*syntax.BindingStmt).Value.(type) {
				case *syntax.CallExpr:
					call = e.PrimaryArg.(*syntax.CallExpr)
				case *syntax.ListExpr:
					call = e.Elements[0].(*syntax.CallExpr)
				case *syntax.BinaryExpr:
					call = e.Left.(*syntax.CallExpr)
				default:
					t.Fatal("fixture lost its nested expression")
				}
				plan := PlanLoopResultLocals(program, DefaultRegistry())
				if mode == "ordinary" {
					if plan != nil {
						t.Fatal("ordinary values unexpectedly gained a cell plan")
					}
				} else if plan == nil || plan.ScopedBodies[call] != call.Block.Body {
					t.Fatal("ordinary nested measure was omitted from the existing plan")
				}
			})
		}
	}
}
