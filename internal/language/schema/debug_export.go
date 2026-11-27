package schema

import "github.com/electrikmilk/cherri/internal/language/types"

func init() {
	compilerMacroSchemas["debugExport"] = &ActionSchema{
		ID: "debugExport", CallableName: "debugExport", Module: "debug", CompilerConstruct: true,
		AppleIdentifier: "is.workflow.actions.setitemname", PrimaryParameterID: "format",
		Parameters: []ParameterSchema{
			{ID: "format", Label: "format", DisplayName: "Format", Type: types.Text, TypeName: "Text", RawType: "rawstring", WireKey: "WFDebugExportFormat", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("json"), EnumValues: []string{"json", "text"}, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "includeDates", Label: "includeDates", DisplayName: "Include Item Creation Dates", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportDates", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "includeTypes", Label: "includeTypes", DisplayName: "Include Native Types", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportTypes", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("true"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "redactSensitive", Label: "redactSensitive", DisplayName: "Redact Sensitive Payloads", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportRedact", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("true"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "includeLabels", Label: "includeLabels", DisplayName: "Include Log Labels", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportLabels", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "redactKeys", Label: "redactKeys", DisplayName: "Sensitive Dictionary Keys", Type: types.NewList(types.Text), TypeName: "List<Text>", RawType: "array", WireKey: "WFDebugExportRedactKeys", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
		},
		OutputType: types.File, OutputTypeName: "File", RawOutputType: "variable",
		Docs:    ActionDocs{Title: "Export Local Debug File", Description: "Return one local JSON/text artifact without saving, sharing or uploading. Payloads remain fully redacted by default. Explicit redactSensitive:false exports Text, native Number display text, ISO 8601 Dates and bounded Dictionary field diagnostics. Source Bool, null and List boundaries are not reconstructed. Nonempty literal redactKeys opts into structural field filtering, including credential and URL/query keys; with redactSensitive:true Text and non-Dictionary top-level payloads stay redacted. Empty keys preserve default disclosure. Unsupported native/nested values have explicit statuses. Dates and labels remain opt-in. Native File acceptance is separate.", Category: "debug", InsertionSnippet: "debugExport()"},
		Builtin: true, EvidenceStatus: EvidenceInferred,
	}
}
