package shortcutcompare

import (
	"fmt"
	"reflect"
	"strconv"
)

// Go integer widths describe the same native plist integer type. Comparing
// their exact decimal values avoids lossy float64 conversion above 2^53.
func strictNativeScalarKind(value any) string {
	if _, ok := value.([]byte); ok {
		return "data"
	}
	switch reflect.ValueOf(value).Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return "integer"
	case reflect.Float32, reflect.Float64:
		return "real"
	default:
		return fmt.Sprintf("%T", value)
	}
}

func strictNativeInteger(value any) string {
	var reflected = reflect.ValueOf(value)
	if reflected.Kind() >= reflect.Int && reflected.Kind() <= reflect.Int64 {
		return strconv.FormatInt(reflected.Int(), 10)
	}
	return strconv.FormatUint(reflected.Uint(), 10)
}

func compareStrictNativeScalars(a, b any, path string, result *ComparisonResult) {
	var kindA, kindB = strictNativeScalarKind(a), strictNativeScalarKind(b)
	if kindA != kindB {
		addDiff(result, path, "type",
			fmt.Sprintf("%s(%v)", kindA, describe(a)),
			fmt.Sprintf("%s(%v)", kindB, describe(b)))
		return
	}
	var equal bool
	if kindA == "integer" {
		equal = strictNativeInteger(a) == strictNativeInteger(b)
	} else {
		// Preserve literal strings, booleans, real values, data and dates.
		// Formatting is diagnostic only and cannot establish equality.
		equal = reflect.DeepEqual(a, b)
	}
	if !equal {
		addDiff(result, path, "value", describe(a), describe(b))
	}
}
