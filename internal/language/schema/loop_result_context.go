package schema

import (
	"reflect"

	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/language/types"
)

// LoopResultExplicitContextArguments admits ordinary, explicitly bound core
// values and the compiler's input-free trace bookkeeping around local loop
// storage. This authorizes no use of the private sequence: the planner still
// visits every returned source argument with allowCells=false, and native
// tagged-value validation remains a separate boundary.
//
// Unknown/native/user calls and omitted Previous consumers are not transparent
// context. Descriptor checks bind the actual resolved action, not its spelling.
func LoopResultExplicitContextArguments(call *syntax.CallExpr, action *ActionSchema) ([]syntax.Expression, bool) {
	if call == nil || action == nil || call.Block != nil || action.Variant != "" ||
		action.CompilerConstruct || action.ScopedBlock || action.ScopedBlockOptional ||
		action.AppIntent != nil || len(action.StaticParameters) != 0 ||
		action.MinVersion != 0 || action.MaxVersion != 0 || action.MacOnly || action.NonMacOnly ||
		len(action.PlatformVersions) != 0 {
		return nil, false
	}
	name, okay := call.Callee.(*syntax.IdentExpr)
	if !okay || name.Name != action.ID || action.CallableName != action.ID {
		return nil, false
	}
	switch action.ID {
	case "comment", "text", "output":
		if !loopResultExplicitCoreDescriptor(action) {
			return nil, false
		}
	case "debugClearLogs", "debugTraceEvent", "debugExportTrace":
		if !loopResultTraceDescriptor(action) {
			return nil, false
		}
	default:
		return nil, false
	}
	seen := map[string]bool{}
	var arguments []syntax.Expression
	if call.PrimaryArg != nil {
		if action.PrimaryParameterID == "" {
			return nil, false
		}
		seen[action.PrimaryParameterID] = true
		arguments = append(arguments, call.PrimaryArg)
	}
	for _, argument := range call.NamedArgs {
		parameter, found := action.ParameterByLabel(argument.Label)
		if !found || argument.Value == nil || seen[parameter.ID] {
			return nil, false
		}
		seen[parameter.ID] = true
		arguments = append(arguments, argument.Value)
	}
	for _, parameter := range action.Parameters {
		if !seen[parameter.ID] && (!parameter.Optional || parameter.DefaultValue == nil) {
			return nil, false
		}
	}
	return arguments, true
}

func loopResultExplicitCoreDescriptor(action *ActionSchema) bool {
	if action.Module != "" || !action.Custom || action.Builtin ||
		len(action.EmittedKeys) != 0 || len(action.Parameters) != 1 || action.OutputType == nil {
		return false
	}
	primary, identifier, wire, codec := "text", "gettext", "WFTextActionText", "text"
	rawType, typeName := "text", "Text"
	expectedType := types.Text
	outputName, rawOutput := "Text", "text"
	expectedOutput := types.Text
	switch action.ID {
	case "comment":
		identifier, wire, codec = "comment", "WFCommentActionText", "token"
		rawType, typeName, expectedType = "rawtext", "Unknown", types.Unknown
		outputName, rawOutput, expectedOutput = "Unknown", "", types.Unknown
	case "output":
		primary, identifier, wire = "output", "output", "WFOutput"
		outputName, rawOutput, expectedOutput = "Unknown", "", types.Unknown
	}
	if action.AppleIdentifier != "is.workflow.actions."+identifier ||
		action.PrimaryParameterID != primary || action.OutputTypeName != outputName ||
		action.RawOutputType != rawOutput || !reflect.DeepEqual(action.OutputType, expectedOutput) {
		return false
	}
	parameter := action.Parameters[0]
	return parameter.ID == primary && parameter.Label == primary &&
		reflect.DeepEqual(parameter.Type, expectedType) && parameter.TypeName == typeName &&
		parameter.RawType == rawType && parameter.WireKey == wire && parameter.Codec == codec &&
		!parameter.Optional && !parameter.Reference && !parameter.Literal && !parameter.Variadic &&
		parameter.DefaultValue == nil && parameter.OmissionPolicy == "" &&
		parameter.EnumName == "" && len(parameter.EnumValues) == 0 &&
		reflect.DeepEqual(parameter.AcceptedForms, []string{"literal", "tokenString"})
}

func loopResultTraceDescriptor(action *ActionSchema) bool {
	if action.Module != "debug" || !action.Builtin || action.Custom || action.OutputType == nil {
		return false
	}
	if action.ID == "debugClearLogs" {
		return action.AppleIdentifier == "is.workflow.actions.setvariable" &&
			action.PrimaryParameterID == "" && len(action.Parameters) == 0 &&
			len(action.EmittedKeys) == 0 && reflect.DeepEqual(action.OutputType, types.Void) &&
			action.OutputTypeName == "" && action.RawOutputType == ""
	}
	if !reflect.DeepEqual(action.OutputType, types.Text) ||
		action.OutputTypeName != "text" || action.RawOutputType != "string" {
		return false
	}
	parameter := func(id, wire string, optional bool, fallback *string, omission OmissionPolicy) ParameterSchema {
		return ParameterSchema{ID: id, Label: id, Type: types.Text, TypeName: "text", RawType: "string",
			Optional: optional, AcceptedForms: []string{"string"}, DefaultValue: fallback,
			OmissionPolicy: omission, WireKey: wire}
	}
	var expected []ParameterSchema
	if action.ID == "debugTraceEvent" {
		if action.AppleIdentifier != "is.workflow.actions.appendvariable" || action.PrimaryParameterID != "stage" ||
			!reflect.DeepEqual(action.EmittedKeys, []string{"WFDebugTraceLabel", "WFDebugTraceStage"}) {
			return false
		}
		expected = []ParameterSchema{
			parameter("stage", "WFDebugTraceStage", false, nil, OmitPolicyEmit),
			parameter("label", "WFDebugTraceLabel", true, StringDefault(""), OmitPolicyOmit),
		}
	} else {
		if action.AppleIdentifier != "is.workflow.actions.gettext" || action.PrimaryParameterID != "runId" ||
			!reflect.DeepEqual(action.EmittedKeys, []string{"WFDebugTraceRunID"}) {
			return false
		}
		expected = []ParameterSchema{
			parameter("runId", "WFDebugTraceRunID", true, StringDefault("cherri-qa-run"), OmitPolicyOmit),
		}
	}
	if len(action.Parameters) != len(expected) {
		return false
	}
	for index, original := range action.Parameters {
		// Display labels and evidence annotations do not determine native input.
		original.DisplayName, original.EvidenceStatus = "", ""
		if !reflect.DeepEqual(original, expected[index]) {
			return false
		}
	}
	return true
}
