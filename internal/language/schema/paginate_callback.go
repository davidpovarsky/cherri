package schema

import (
	"reflect"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// PaginatePageParameter accepts one immutable, implicitly typed page binding.
// Typed selectors/returned items and first-class closures are separate contracts.
func PaginatePageParameter(call *syntax.CallExpr) *syntax.ParameterDecl {
	if call == nil || call.Block == nil || call.Block.Body == nil ||
		len(call.Block.Parameters) != 1 {
		return nil
	}
	parameter := call.Block.Parameters[0]
	if parameter == nil || parameter.Name == "" || parameter.Optional ||
		parameter.TypeExpr != nil || parameter.DefaultExpr != nil {
		return nil
	}
	return parameter
}

// LoopResultScopedPaginateBody binds the actual compiler-owned descriptor,
// authored call, parameter and body. A similarly named user/native action does
// not inherit the closed explicit-input loop-cell storage contract.
func LoopResultScopedPaginateBody(call *syntax.CallExpr, action *ActionSchema) *syntax.BlockStmt {
	if PaginatePageParameter(call) == nil || action == nil {
		return nil
	}
	callee, okay := call.Callee.(*syntax.IdentExpr)
	if !okay || callee.Name != "paginate" {
		return nil
	}
	canonical := compilerMacroSchemas["paginate"]
	if canonical == nil || !action.ScopedBlock || !action.ScopedBlockOptional ||
		!reflect.DeepEqual(action, canonical) {
		return nil
	}
	return call.Block.Body
}

// PageCallbackSource keeps the exact lexical parameter identity in addition
// to the existing scoped-body identity. Spelling or a copied body is not proof.
type PageCallbackSource struct {
	Block     *syntax.ScopedBlock
	Body      *syntax.BlockStmt
	Parameter *syntax.ParameterDecl
	Name      string
}

func (plan *LoopResultLocalPlan) PageCallbackSourceMatches(call *syntax.CallExpr) bool {
	if plan == nil || call == nil {
		return false
	}
	original, found := plan.PageCallbacks[call]
	parameter := PaginatePageParameter(call)
	return found && parameter != nil &&
		original.Block == call.Block && original.Body == call.Block.Body &&
		original.Parameter == parameter && original.Name == parameter.Name &&
		plan.ScopedBodies[call] == call.Block.Body
}

// LoopResultExplicitDelay permits only the canonical numeric Delay descriptor
// with one explicit scalar input. It grants no omitted Previous or cell escape.
func LoopResultExplicitDelay(call *syntax.CallExpr, action *ActionSchema) bool {
	if call == nil || action == nil || call.Block != nil || call.PrimaryArg == nil ||
		len(call.NamedArgs) != 0 || action.ID != "wait" ||
		action.CallableName != "wait" || action.Module != "" ||
		action.AppleIdentifier != "is.workflow.actions.delay" ||
		action.PrimaryParameterID != "seconds" || action.OutputType == nil ||
		!types.Void.Equals(action.OutputType) || action.OutputTypeName != "Void" || !action.Custom || action.Builtin || action.CompilerConstruct ||
		action.ScopedBlock || action.ScopedBlockOptional || action.AppIntent != nil || action.Variant != "" ||
		len(action.StaticParameters) != 0 || len(action.Parameters) != 1 {
		return false
	}
	name, okay := call.Callee.(*syntax.IdentExpr)
	if !okay || name.Name != "wait" {
		return false
	}
	parameter := action.Parameters[0]
	return parameter.ID == "seconds" && parameter.Label == "seconds" &&
		parameter.WireKey == "WFDelayTime" && parameter.Codec == "number" &&
		parameter.Type != nil && types.Number.Equals(parameter.Type) && parameter.RawType == "number" &&
		reflect.DeepEqual(parameter.AcceptedForms, []string{"literal", "attachment(output)", "attachment(variable)"}) &&
		!parameter.Optional && !parameter.Reference && !parameter.Variadic &&
		parameter.DefaultValue == nil && parameter.EnumName == "" &&
		len(parameter.EnumValues) == 0
}
