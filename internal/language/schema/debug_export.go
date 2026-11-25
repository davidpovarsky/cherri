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
			{ID: "redactSensitive", Label: "redactSensitive", DisplayName: "Redact All Payloads", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportRedact", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("true"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
			{ID: "includeLabels", Label: "includeLabels", DisplayName: "Include Log Labels", Type: types.Bool, TypeName: "Bool", RawType: "bool", WireKey: "WFDebugExportLabels", Optional: true, Literal: true, AcceptedForms: []string{"literal"}, DefaultValue: StringDefault("false"), OmissionPolicy: OmitPolicyOmit, EvidenceStatus: EvidenceInferred},
		},
		OutputType: types.File, OutputTypeName: "File", RawOutputType: "variable",
		Docs:    ActionDocs{Title: "Export Local Debug File", Description: "Return one local JSON/text File without saving, sharing or uploading. Fully redacts payloads by default. Explicit redactSensitive:false exports only Text as base64-encoded UTF-8; other payloads report UNSUPPORTED_PAYLOAD_TYPE. includeDates:true reads each native item's Creation Date with explicit absent/ambiguous/unsupported statuses. includeLabels:true reads source-owned label ranges only when redactSensitive:false; otherwise labels remain redacted. Options are compile-time literals. Native runtime acceptance remains required.", Category: "debug", InsertionSnippet: "debugExport()"},
		Builtin: true, EvidenceStatus: EvidenceInferred,
	}
}
