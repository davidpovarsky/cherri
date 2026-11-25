package lower

import (
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// A cell's authorial union member determines its recursive payload layout.
// Equal outer Kind is insufficient when two List or Map members use different
// representations; the exact source member must win independently of order.
func TestTaggedUnionPayloadSelection(t *testing.T) {
	jsonList := types.NewList(types.JsonNull)
	optionalList := types.NewList(types.NewOptional(types.Bool))
	jsonMap := types.NewMap(types.Text, types.JsonNull)
	optionalMap := types.NewMap(types.Text, types.NewOptional(types.Bool))
	nestedJSON := types.NewList(jsonList)
	nestedOptional := types.NewList(optionalList)
	cases := []struct {
		name                       string
		expected, actual, selected types.Type
		failure                    bool
	}{
		{"list_json_exact", types.NewUnion(jsonList, optionalList), jsonList, jsonList, false},
		{"list_optional_exact", types.NewUnion(jsonList, optionalList), optionalList, optionalList, false},
		{"list_reversed", types.NewUnion(optionalList, jsonList), optionalList, optionalList, false},
		{"map_json_exact", types.NewUnion(jsonMap, optionalMap), jsonMap, jsonMap, false},
		{"map_optional_exact", types.NewUnion(jsonMap, optionalMap), optionalMap, optionalMap, false},
		{"map_reversed", types.NewUnion(optionalMap, jsonMap), jsonMap, jsonMap, false},
		{"nested_list_exact", types.NewUnion(nestedJSON, nestedOptional), nestedOptional, nestedOptional, false},
		{"optional_outer", types.NewOptional(types.NewUnion(jsonList, optionalList)), optionalList, optionalList, false},
		{"only_assignable_member", types.NewUnion(jsonList, optionalList), types.NewList(types.Bool), optionalList, false},
		{"ambiguous_unknown_list", types.NewUnion(jsonList, optionalList), types.NewList(types.Unknown), nil, true},
		{"ambiguous_unknown_map", types.NewUnion(jsonMap, optionalMap), types.NewMap(types.Text, types.Unknown), nil, true},
		{"ordinary_unchanged", types.NewList(types.Number), types.NewList(types.Number), types.NewList(types.Number), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := taggedPayloadTarget(tc.expected, tc.actual)
			if (err != nil) != tc.failure {
				t.Fatalf("target=%v error=%v; expected target=%v failure=%t", result, err, tc.selected, tc.failure)
			}
			if tc.failure {
				if !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
					t.Fatal(err)
				}
			} else if result == nil || !result.Equals(tc.selected) {
				t.Fatalf("selected %v; expected exact member %v", result, tc.selected)
			}
		})
	}
}

// This is an independent lower-layer boundary check. A native Items Count is
// allowed to observe the existing outer List/Dictionary content object only.
// No JSON cell, alternate Count mode, source action or mutated schema inherits
// that permission. It does not assert Dictionary key cardinality or runtime.
func TestTaggedContainerCountSchemaBoundary(t *testing.T) {
	privateMap := types.NewMap(types.Text, types.JsonNull)
	privateList := types.NewList(types.JsonNull)
	cases := []struct {
		name   string
		value  types.Type
		mutate func(*schema.ActionSchema, *syntax.CallExpr, *Lowerer)
		allow  bool
	}{
		{"map", privateMap, nil, true},
		{"list", privateList, nil, true},
		{"json_value", types.JsonValue, nil, false},
		{"json_null", types.JsonNull, nil, false},
		{"wrong_id", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) { a.ID = "other" }, false},
		{"wrong_native_action", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) {
			a.AppleIdentifier = "is.workflow.actions.gettext"
		}, false},
		{"wrong_output", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) { a.OutputType = types.Text }, false},
		{"wrong_input_wire", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) { a.Parameters[0].WireKey = "WFInput" }, false},
		{"wrong_default", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) {
			a.Parameters[1].DefaultValue = schema.StringDefault("Characters")
		}, false},
		{"static_parameter", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) {
			a.StaticParameters = map[string]any{"WFCountType": "Words"}
		}, false},
		{"compiler_construct", privateList, func(a *schema.ActionSchema, _ *syntax.CallExpr, _ *Lowerer) { a.CompilerConstruct = true }, false},
		{"named_mode", privateMap, func(_ *schema.ActionSchema, c *syntax.CallExpr, _ *Lowerer) {
			c.NamedArgs = []syntax.NamedArg{{Label: "type", Value: &syntax.LiteralExpr{Type: syntax.TokenString, Value: "Characters"}}}
		}, false},
		{"scoped_call", privateList, func(_ *schema.ActionSchema, c *syntax.CallExpr, _ *Lowerer) {
			c.Block = &syntax.ScopedBlock{Body: &syntax.BlockStmt{}}
		}, false},
		{"source_action", privateMap, func(_ *schema.ActionSchema, _ *syntax.CallExpr, l *Lowerer) {
			l.actions["count"] = &syntax.ActionDecl{}
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original, okay := schema.DefaultRegistry().LookupAction("count")
			if !okay {
				t.Fatal("actual Count schema missing")
			}
			action := *original
			action.Parameters = append([]schema.ParameterSchema(nil), original.Parameters...)
			l := NewLowerer(nil)
			expr := &syntax.IdentExpr{Name: "value"}
			call := &syntax.CallExpr{Callee: &syntax.IdentExpr{Name: "count"}, PrimaryArg: expr}
			l.bindings["value"] = BindingRef{Type: tc.value, TaggedTransport: tc.value}
			l.exprTypes = map[syntax.Expression]types.Type{expr: tc.value, call: types.Number}
			if tc.mutate != nil {
				tc.mutate(&action, call, l)
			}
			l.registry = schema.NewRegistry([]*schema.ActionSchema{&action}, nil)
			err := l.validateTaggedNativeBoundary("count", call)
			if (err == nil) != tc.allow {
				t.Fatalf("allow=%t error=%v", tc.allow, err)
			}
			if err != nil && !strings.Contains(err.Error(), "E_TARGET_VALUE_SHAPE") {
				t.Fatal(err)
			}
		})
	}
}
