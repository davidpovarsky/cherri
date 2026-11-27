package schema

import (
	"fmt"

	"github.com/electrikmilk/cherri/internal/language/types"
)

func resolvePaginateItemOptions(out *PaginateOptions, params map[string]any) error {
	if value, present := params["WFPaginateItemKey"]; present {
		var key, okay = value.(string)
		if !okay || !paginateKeyPattern.MatchString(key) {
			return fmt.Errorf("PAGINATE_ITEM_KEY: itemKey must be a literal key matching [A-Za-z_][A-Za-z0-9_-]{0,63}")
		}
		out.ItemKey = key
	}
	if value, present := params["WFPaginateItemType"]; present {
		var kind, okay = value.(string)
		if !okay {
			return fmt.Errorf("PAGINATE_ITEM_TYPE: itemType must be a literal dictionary, text, number or bool")
		}
		switch kind {
		case "dictionary", "text", "number", "bool":
			out.ItemType = kind
		default:
			return fmt.Errorf("PAGINATE_ITEM_TYPE: unsupported itemType %q; nested List, Optional, JsonNull and mixed result storage require an explicit lossless native codec", kind)
		}
	}
	return nil
}

// The output contract depends on the same literal selector used by the native
// emitter. A declared List annotation must not invent a different result kind.
func (options PaginateOptions) ResultType() types.Type {
	var element types.Type = types.NewMap(types.Text, types.AnyContent)
	switch options.ItemType {
	case "text":
		element = types.Text
	case "number":
		element = types.Number
	case "bool":
		element = types.Bool
	}
	return types.NewList(element)
}
