package lower

import (
	"fmt"
	"sort"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/ir"
)

// Decoded raw data bypasses authored-literal analysis. Validate it before the
// plist serializer can omit a nil dictionary value or shrink an array.
func validatedNativePlistValue(value any) (*ir.RawPlistValue, error) {
	if path := nativePlistNullPath(value, "$"); path != "" {
		return nil, fmt.Errorf("%s: native.plist null at %s has no supported native value encoding; retain JSON as Text instead", analysis.CodeNativeJSONNull, path)
	}
	return &ir.RawPlistValue{Data: value}, nil
}

func nativePlistNullPath(value any, path string) string {
	switch value := value.(type) {
	case nil:
		return path
	case map[string]any:
		keys := make([]string, 0, len(value))
		for key := range value {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if found := nativePlistNullPath(value[key], fmt.Sprintf("%s[%q]", path, key)); found != "" {
				return found
			}
		}
	case []any:
		for index, child := range value {
			if found := nativePlistNullPath(child, fmt.Sprintf("%s[%d]", path, index)); found != "" {
				return found
			}
		}
	}
	return ""
}
