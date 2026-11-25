package schema

import (
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
	"strings"
)

func init() {
	// Compiler metadata belongs to the existing macro catalog, not the native
	// Apple action inventory. The measured body is lowered through the same backend.
	compilerMacroSchemas["measure"] = &ActionSchema{
		ID: "measure", CallableName: "measure", Module: "debug",
		PrimaryParameterID: "label",
		Parameters: []ParameterSchema{{
			ID: "label", Label: "label", DisplayName: "Label", Type: types.Text,
			TypeName: "text", RawType: "string", Literal: true, AcceptedForms: []string{"literal"},
			EvidenceStatus: EvidenceInferred,
		}},
		OutputType: types.AnyContent, OutputTypeName: "AnyContent",
		Builtin: true, CompilerConstruct: true, ScopedBlock: true,
		EvidenceStatus: EvidenceInferred,
		Docs: ActionDocs{
			Title: "Measure Block", Category: "debug",
			Description:      "V2 scoped value block. Evaluate once, log elapsed seconds through existing native timers, and return the original typed value. Runtime timing accuracy remains platform-dependent.",
			InsertionSnippet: "measure(\"label\") { <#expression#> }",
		},
	}
}

// LoopResultScopedMeasureBody recognizes the canonical inline, exactly-once
// measurement contract. It does not authorize a user action with the same
// spelling, an omitted label/Previous argument, or a different block lifecycle.
func LoopResultScopedMeasureBody(call *syntax.CallExpr, action *ActionSchema) *syntax.BlockStmt {
	if call == nil || call.Block == nil || call.Block.Body == nil || action == nil ||
		action.ID != "measure" || action.CallableName != "measure" || action.Module != "debug" ||
		!action.Builtin || action.Custom || !action.CompilerConstruct || !action.ScopedBlock ||
		action.AppleIdentifier != "" || action.Variant != "" || action.AppIntent != nil ||
		len(action.StaticParameters) != 0 || len(action.EmittedKeys) != 0 ||
		action.PrimaryParameterID != "label" || action.OutputType == nil ||
		!types.AnyContent.Equals(action.OutputType) || len(action.Parameters) != 1 {
		return nil
	}
	callee, okay := call.Callee.(*syntax.IdentExpr)
	if !okay || callee.Name != "measure" {
		return nil
	}
	parameter := action.Parameters[0]
	if parameter.ID != "label" || parameter.Label != "label" ||
		parameter.Type == nil || !types.Text.Equals(parameter.Type) ||
		!parameter.Literal || parameter.Optional || parameter.Reference || parameter.Variadic ||
		parameter.DefaultValue != nil || parameter.WireKey != "" || parameter.Codec != "" ||
		parameter.EnumName != "" || len(parameter.EnumValues) != 0 ||
		len(parameter.AcceptedForms) != 1 || parameter.AcceptedForms[0] != "literal" {
		return nil
	}
	label := call.PrimaryArg
	for _, argument := range call.NamedArgs {
		if argument.Label != "label" || label != nil {
			return nil
		}
		label = argument.Value
	}
	literal, okay := label.(*syntax.LiteralExpr)
	if !okay || (literal.Type != syntax.TokenString && literal.Type != syntax.TokenRawString) ||
		strings.TrimSpace(literal.Value) == "" {
		return nil
	}
	return call.Block.Body
}
