package main

import "github.com/electrikmilk/cherri/internal/language/backend"

// A filter parameter carries a compile-time predicate specification. Preserve
// its semantic map/list through the existing native-parameter conversion so
// buildContentItemFilter can validate and encode it. Ordinary runtime
// Dictionaries continue through the typed literal codec.
func convertCallArgForCanonicalParameter(arg backend.CallArgument, wireKey string) actionArgument {
	if wireKey == "WFContentItemFilter" && !arg.Omitted && !arg.Value.Omitted {
		switch arg.Value.Kind {
		case backend.ValDict:
			return actionArgument{valueType: Dict, value: ConvertSemanticValueToParam(arg.Value)}
		case backend.ValList:
			return actionArgument{valueType: Arr, value: ConvertSemanticValueToParam(arg.Value)}
		}
	}
	return convertCallArgToActionArg(arg)
}
