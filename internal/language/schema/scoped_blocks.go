package schema

import "github.com/electrikmilk/cherri/internal/language/types"

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
