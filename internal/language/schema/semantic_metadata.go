package schema

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"
)

// StringDefault preserves whether a default was declared, including empty text.
// ParameterSchema.Type describes the semantic type; the string representation
// remains compatible with the shared action catalog.
func StringDefault(value string) *string {
	return &value
}

func fingerprintDefault(value *string) string {
	if value == nil {
		return "null"
	}
	return fmt.Sprintf("%q", *value)
}

type semanticIdentityValue struct {
	Type  string `json:"type"`
	Value any    `json:"value"`
}

// fingerprintJSON preserves native scalar types, container boundaries and
// escaped text. Plain JSON would alias plist Data with base64 Text, and an
// integer with a real number. Unsupported or cyclic metadata cannot receive a
// usable identity. The public catalog representation is unchanged.
func fingerprintJSON(value any) string {
	if _, err := json.Marshal(value); err != nil {
		panic(fmt.Errorf("schema fingerprint: cannot encode semantic metadata: %w", err))
	}
	var encoded, err = json.Marshal(fingerprintValue(reflect.ValueOf(value)))
	if err != nil {
		panic(fmt.Errorf("schema fingerprint: cannot encode semantic metadata: %w", err))
	}
	return string(encoded)
}

func fingerprintValue(value reflect.Value) semanticIdentityValue {
	if !value.IsValid() {
		return semanticIdentityValue{Type: "nil"}
	}
	if value.Kind() == reflect.Interface {
		return fingerprintValue(value.Elem())
	}
	var out = semanticIdentityValue{Type: value.Type().String()}
	if date, ok := value.Interface().(time.Time); ok {
		out.Value = date.UTC().Format(time.RFC3339Nano)
		return out
	}
	switch value.Kind() {
	case reflect.Bool:
		out.Value = value.Bool()
	case reflect.String:
		out.Value = value.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		out.Value = value.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		out.Value = value.Uint()
	case reflect.Float32, reflect.Float64:
		out.Value = value.Float()
	case reflect.Pointer:
		if value.Type().PkgPath() != "" {
			panic(fmt.Errorf("schema fingerprint: unsupported semantic metadata type %s", value.Type()))
		}
		if !value.IsNil() {
			out.Value = fingerprintValue(value.Elem())
		}
	case reflect.Map:
		if value.Type().PkgPath() != "" || value.Type().Key().Kind() != reflect.String {
			panic(fmt.Errorf("schema fingerprint: unsupported semantic metadata type %s", value.Type()))
		}
		if value.IsNil() {
			break
		}
		var entries = make(map[string]semanticIdentityValue, value.Len())
		var iterator = value.MapRange()
		for iterator.Next() {
			entries[iterator.Key().String()] = fingerprintValue(iterator.Value())
		}
		out.Value = entries
	case reflect.Slice, reflect.Array:
		if value.Type().PkgPath() != "" {
			panic(fmt.Errorf("schema fingerprint: unsupported semantic metadata type %s", value.Type()))
		}
		if value.Kind() == reflect.Slice && value.IsNil() {
			break
		}
		var entries = make([]semanticIdentityValue, value.Len())
		for i := range entries {
			entries[i] = fingerprintValue(value.Index(i))
		}
		out.Value = entries
	default:
		panic(fmt.Errorf("schema fingerprint: unsupported semantic metadata type %s", value.Type()))
	}
	return out
}
