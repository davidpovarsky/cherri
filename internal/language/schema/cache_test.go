package schema

import (
	"math"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

func TestCacheOptionsAndScalarKinds(t *testing.T) {
	valid := map[string]any{"key": "קובץ:🍒", "ttl": 300.0}
	options, err := ResolveCacheOptions(valid)
	if err != nil {
		t.Fatal(err)
	}
	for name, change := range map[string]func(map[string]any){
		"missing_key":                  func(p map[string]any) { delete(p, "key") },
		"empty_key":                    func(p map[string]any) { p["key"] = " " },
		"missing_ttl":                  func(p map[string]any) { delete(p, "ttl") },
		"negative_ttl":                 func(p map[string]any) { p["ttl"] = -1.0 },
		"nan_ttl":                      func(p map[string]any) { p["ttl"] = math.NaN() },
		"infinite_ttl":                 func(p map[string]any) { p["ttl"] = math.Inf(1) },
		"text_ttl":                     func(p map[string]any) { p["ttl"] = "300" },
		"numeric_global":               func(p map[string]any) { p["global"] = 1 },
		"sensitive_without_permission": func(p map[string]any) { p["sensitive"] = true },
		"text_permission":              func(p map[string]any) { p["allowSensitive"] = "true" },
		"unknown_option":               func(p map[string]any) { p["opaque"] = true },
	} {
		t.Run(name, func(t *testing.T) {
			p := map[string]any{"key": "קובץ:🍒", "ttl": 300.0}
			change(p)
			if _, err := ResolveCacheOptions(p); err == nil {
				t.Fatal("invalid literal contract accepted")
			}
		})
	}
	for _, ttl := range []float64{0, 0.25, 300} {
		p := map[string]any{"key": "key", "ttl": ttl, "global": true, "sensitive": true, "allowSensitive": true}
		result, err := ResolveCacheOptions(p)
		if err != nil || result.TTL != ttl || !result.Global || !result.Sensitive || !result.AllowSensitive {
			t.Fatal(result, err)
		}
	}
	seen := map[string]bool{}
	for _, global := range []bool{false, true} {
		options.Global = global
		for _, kind := range []string{"text", "number", "bool"} {
			name := options.Namespace(kind)
			if seen[name] || strings.Contains(name, options.Key) {
				t.Fatal("scope/type collision or unhashed key", name)
			}
			seen[name] = true
		}
	}
	for _, entry := range []struct {
		value types.Type
		kind  string
	}{{types.Text, "text"}, {types.Number, "number"}, {types.Bool, "bool"}, {types.Void, ""}, {types.JsonNull, ""}, {types.JsonValue, ""}, {types.AnyContent, ""}, {types.NewOptional(types.Text), ""}, {types.NewList(types.Number), ""}, {types.NewMap(types.Text, types.Number), ""}} {
		if got := CacheValueKind(entry.value); got != entry.kind {
			t.Fatalf("%s: %q", entry.value, got)
		}
	}
}
func TestCacheLoopPlanKeepsLazySourceIdentity(t *testing.T) {
	for _, ttl := range []string{"0", "300"} {
		t.Run(ttl, func(t *testing.T) {
			program := scopedPlanProgram(t, "var values=repeat 0 {yield []}\nlet size=cache(key:\"key\",ttl:"+ttl+"){values=repeat 2 {yield []}\nlet saved=values\ncount(saved)}\nreturn count(values)")
			before := syntax.Format(program)
			binding := program.Statements[0].(*syntax.BindingStmt)
			call := program.Statements[1].(*syntax.BindingStmt).Value.(*syntax.CallExpr)
			assignment := call.Block.Body.Statements[0].(*syntax.AssignStmt)
			plan := PlanLoopResultLocals(program, DefaultRegistry())
			if plan == nil || plan.ScopedBodies[call] != call.Block.Body || plan.Assignments[assignment] != binding || !plan.AssignmentSourceMatches(assignment) {
				t.Fatal("lazy body, captured target or actual RHS lost source ownership")
			}
			if !plan.Bodies[binding.Value.(*syntax.RepeatStmt).BodyBlock] || !plan.Bodies[assignment.Value.(*syntax.RepeatStmt).BodyBlock] {
				t.Fatal("entry or miss origin lost its native cell representation")
			}
			clone := *call
			if plan.ScopedBodies[&clone] != nil {
				t.Fatal("copied call acquired original body authority")
			}
			copiedAssignment := *assignment
			if plan.AssignmentSourceMatches(&copiedAssignment) {
				t.Fatal("copied assignment acquired source authority")
			}
			if syntax.Format(program) != before {
				t.Fatal("cache planning rewrote the source AST")
			}
		})
	}
}
func TestCacheLoopContractRejectsChangedMetadata(t *testing.T) {
	program := scopedPlanProgram(t, "return cache(key:\"key\",ttl:300){1}")
	call := program.Statements[0].(*syntax.ReturnStmt).Value.(*syntax.CallExpr)
	canonical, _ := DefaultRegistry().LookupAction("cache")
	if LoopResultScopedCacheBody(call, canonical) != call.Block.Body {
		t.Fatal("canonical cache not recognized")
	}
	for name, change := range map[string]func(*ActionSchema){
		"name":              func(a *ActionSchema) { a.CallableName = "other" },
		"module":            func(a *ActionSchema) { a.Module = "foreign" },
		"native":            func(a *ActionSchema) { a.AppleIdentifier = "is.workflow.actions.getvariable" },
		"custom":            func(a *ActionSchema) { a.Custom = true },
		"not_builtin":       func(a *ActionSchema) { a.Builtin = false },
		"not_compiler":      func(a *ActionSchema) { a.CompilerConstruct = false },
		"not_scoped":        func(a *ActionSchema) { a.ScopedBlock = false },
		"target":            func(a *ActionSchema) { a.MinVersion = 26 },
		"max_target":        func(a *ActionSchema) { a.MaxVersion = 28 },
		"primary":           func(a *ActionSchema) { a.PrimaryParameterID = "ttl" },
		"output":            func(a *ActionSchema) { a.OutputType = types.Text },
		"removed_parameter": func(a *ActionSchema) { a.Parameters = a.Parameters[:4] },
	} {
		t.Run(name, func(t *testing.T) {
			altered := *canonical
			change(&altered)
			if LoopResultScopedCacheBody(call, &altered) != nil {
				t.Fatal("changed metadata acquired lazy-body authority")
			}
		})
	}
}
