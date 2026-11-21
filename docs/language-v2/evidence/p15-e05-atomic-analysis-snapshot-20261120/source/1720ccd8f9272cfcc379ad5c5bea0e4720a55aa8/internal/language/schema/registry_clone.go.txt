package schema

import (
	"reflect"

	"github.com/electrikmilk/cherri/internal/language/types"
)

// Clone creates a catalog snapshot for one analysis or language-service base.
// Source declarations may replace entries in the snapshot without changing the
// supplied catalog. Metadata containers and structural type containers are
// copied; canonical scalar type values and compiler macro definitions are
// immutable and remain shared.
func (r *Registry) Clone() *Registry {
	if r == nil {
		return nil
	}
	actions := r.AllActions()
	for i, action := range actions {
		actions[i] = cloneActionSchema(action)
	}
	return NewRegistry(actions, r.AllEnums())
}

func cloneActionSchema(action *ActionSchema) *ActionSchema {
	copy := *action
	copy.Parameters = append([]ParameterSchema(nil), action.Parameters...)
	for i := range copy.Parameters {
		if value := action.Parameters[i].DefaultValue; value != nil {
			copy.Parameters[i].DefaultValue = StringDefault(*value)
		}
		copy.Parameters[i].AcceptedForms = append([]string(nil), action.Parameters[i].AcceptedForms...)
		copy.Parameters[i].EnumValues = append([]string(nil), action.Parameters[i].EnumValues...)
		copy.Parameters[i].Type = cloneRegistryType(action.Parameters[i].Type)
	}
	copy.EmittedKeys = append([]string(nil), action.EmittedKeys...)
	copy.Docs.Keywords = append([]string(nil), action.Docs.Keywords...)
	copy.OutputType = cloneRegistryType(action.OutputType)
	if action.PlatformVersions != nil {
		copy.PlatformVersions = make(map[string]PlatformVersionBounds, len(action.PlatformVersions))
		for platform, bounds := range action.PlatformVersions {
			copy.PlatformVersions[platform] = bounds
		}
	}
	if action.AppIntent != nil {
		intent := *action.AppIntent
		if intent.RequiresAppInstallation != nil {
			required := *intent.RequiresAppInstallation
			intent.RequiresAppInstallation = &required
		}
		copy.AppIntent = &intent
	}
	if action.StaticParameters != nil {
		copy.StaticParameters = cloneRegistryValue(reflect.ValueOf(action.StaticParameters)).Interface().(map[string]any)
	}
	return &copy
}

func cloneRegistryType(value types.Type) types.Type {
	switch value := value.(type) {
	case types.ListType:
		return types.NewList(cloneRegistryType(value.Element))
	case types.MapType:
		return types.NewMap(cloneRegistryType(value.Key), cloneRegistryType(value.Value))
	case types.OptionalType:
		value.Inner = cloneRegistryType(value.Inner)
		return value
	case types.UnionType:
		value.Members = append([]types.Type(nil), value.Members...)
		for i := range value.Members {
			value.Members[i] = cloneRegistryType(value.Members[i])
		}
		return value
	case types.RecordType:
		fields := make(map[string]types.RecordField, len(value.Fields))
		for name, field := range value.Fields {
			field.Type = cloneRegistryType(field.Type)
			fields[name] = field
		}
		return types.RecordType{Fields: fields}
	default:
		return value
	}
}

// Static parameters are acyclic serialized values. Keep their exact map/slice
// types (including plist data) instead of a JSON round trip or scalar coercion.
func cloneRegistryValue(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.New(value.Type()).Elem()
		result.Set(cloneRegistryValue(value.Elem()))
		return result
	case reflect.Map:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeMapWithSize(value.Type(), value.Len())
		iter := value.MapRange()
		for iter.Next() {
			result.SetMapIndex(iter.Key(), cloneRegistryValue(iter.Value()))
		}
		return result
	case reflect.Slice:
		if value.IsNil() {
			return reflect.Zero(value.Type())
		}
		result := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneRegistryValue(value.Index(i)))
		}
		return result
	case reflect.Array:
		result := reflect.New(value.Type()).Elem()
		for i := 0; i < value.Len(); i++ {
			result.Index(i).Set(cloneRegistryValue(value.Index(i)))
		}
		return result
	default:
		return value
	}
}
