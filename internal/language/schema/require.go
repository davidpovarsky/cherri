package schema

import "github.com/electrikmilk/cherri/internal/language/types"

func init() {
	compilerMacroSchemas["require"] = &ActionSchema{
		ID: "require", CallableName: "require", Module: "control", CompilerConstruct: true,
		AppleIdentifier: "is.workflow.actions.getvariable", PrimaryParameterID: "value",
		Parameters: []ParameterSchema{
			{ID: "value", Label: "value", DisplayName: "Value", Type: types.AnyContent, TypeName: "AnyContent", RawType: "variable", WireKey: "WFInput", OmissionPolicy: OmitPolicyEmit, EvidenceStatus: EvidenceInferred},
			{ID: "type", Label: "type", DisplayName: "Native Category", Type: types.Text, TypeName: "Text", RawType: "rawstring", WireKey: "WFRequireType", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "nonEmpty", Label: "nonEmpty", DisplayName: "Nonempty", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFRequireNonEmpty", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "message", Label: "message", DisplayName: "Failure Message", Type: types.Text, TypeName: "Text", RawType: "string", WireKey: "WFAlertActionMessage", Optional: true, Codec: "textToken", OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
		},
		OutputType: types.AnyContent, OutputTypeName: "AnyContent", RawOutputType: "variable",
		Docs:    ActionDocs{Title: "Require Condition or Typed Value", Description: "Require a source-proved Boolean condition, or validate text, number or dictionary native content and optional nonempty character/key/item count, then return the original value. Type and nonEmpty are compile-time literals.", Category: "control", InsertionSnippet: "require(<#value#>, nonEmpty: true)"},
		Builtin: true, EvidenceStatus: EvidenceInferred,
	}
}
