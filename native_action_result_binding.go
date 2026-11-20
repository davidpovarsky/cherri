package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// A native action's compiler-private result handle is distinct from its wire
// UUID. Only an actually emitted consumer resolves the handle. This permits
// unused exact raw actions and aliases without changing their opaque parameters.
type nativeActionResultBinding struct {
	params         map[string]any
	exact          bool
	defaultUUID    string
	explicitUUID   bool
	requestedUUID  string
	declaredUUID   string
	declaredUsable bool
	consumedUUID   string
}

func (s *CanonicalBackendSession) BindNativeActionResult(referenceUUID, defaultUUID string, exact, explicitUUID bool, requestedUUID string) error {
	return s.bindNativeActionResultAt(len(s.shortcut.WFWorkflowActions)-1, referenceUUID, defaultUUID, exact, explicitUUID, requestedUUID)
}

func (s *CanonicalBackendSession) bindNativeActionResultAt(index int, referenceUUID, defaultUUID string, exact, explicitUUID bool, requestedUUID string) error {
	if referenceUUID == "" || index < 0 || index >= len(s.shortcut.WFWorkflowActions) {
		return fmt.Errorf("NATIVE_OUTPUT_BINDING_INTERNAL: native result has no emitted producer")
	}
	if s.nativeActionResults == nil {
		s.nativeActionResults = make(map[string]*nativeActionResultBinding)
	}
	if _, exists := s.nativeActionResults[referenceUUID]; exists {
		return fmt.Errorf("NATIVE_OUTPUT_BINDING_INTERNAL: duplicate compiler result handle")
	}
	params := s.shortcut.WFWorkflowActions[index].WFWorkflowActionParameters
	declared, ok := params["UUID"].(string)
	s.nativeActionResults[referenceUUID] = &nativeActionResultBinding{
		params: params, exact: exact, defaultUUID: defaultUUID,
		explicitUUID: explicitUUID, requestedUUID: requestedUUID,
		declaredUUID: declared, declaredUsable: ok && strings.TrimSpace(declared) != "",
	}
	return nil
}

func (s *CanonicalBackendSession) rememberExactNativeParameters(params map[string]any) {
	if params == nil {
		return
	}
	if s.exactNativeParameterMaps == nil {
		s.exactNativeParameterMaps = make(map[uintptr]bool)
	}
	s.exactNativeParameterMaps[reflect.ValueOf(params).Pointer()] = true
}

func (s *CanonicalBackendSession) isExactNativeParameters(params map[string]any) bool {
	return params != nil && s.exactNativeParameterMaps[reflect.ValueOf(params).Pointer()]
}

func (s *CanonicalBackendSession) consumeNativeActionResult(referenceUUID string) (string, error) {
	binding, found := s.nativeActionResults[referenceUUID]
	if !found {
		return referenceUUID, nil
	}
	actual, valid := binding.params["UUID"].(string)
	valid = valid && strings.TrimSpace(actual) != ""
	if binding.exact {
		// UUID arguments outside native.plist cannot override its exact bytes.
		if binding.explicitUUID && (!binding.declaredUsable || binding.requestedUUID != binding.declaredUUID) {
			return "", fmt.Errorf("NATIVE_OUTPUT_IDENTITY_CONFLICT: an exact native result's explicit output UUID must equal the UUID inside its raw parameters")
		}
		if !binding.declaredUsable {
			return "", fmt.Errorf("NATIVE_OUTPUT_ID_MISSING: consumed exact native output requires a non-empty string UUID in its raw parameters; leave it unused or provide that explicit identity")
		}
		if !valid || actual != binding.declaredUUID {
			return "", fmt.Errorf("NATIVE_OUTPUT_IDENTITY_CONFLICT: exact native producer identity changed after emission")
		}
	} else if !valid {
		if _, present := binding.params["UUID"]; present || binding.explicitUUID || binding.params == nil {
			return "", fmt.Errorf("NATIVE_OUTPUT_ID_MISSING: consumed native output has an invalid explicit UUID")
		}
		// Ordinary native parameters are normalized by the existing backend.
		// Add an output identity only when an emitted consumer actually needs it.
		actual = binding.defaultUUID
		binding.params["UUID"] = actual
	}
	if binding.explicitUUID && binding.requestedUUID != actual {
		return "", fmt.Errorf("NATIVE_OUTPUT_IDENTITY_CONFLICT: explicit native output UUID differs from its emitted producer")
	}
	if binding.consumedUUID != "" && binding.consumedUUID != actual {
		return "", fmt.Errorf("NATIVE_OUTPUT_IDENTITY_CONFLICT: native output identity changed between consumers")
	}
	binding.consumedUUID = actual
	return actual, nil
}

// This is a value-preserving traversal of existing canonical argument values,
// not a serializer. Only compiler-private ActionOutput handles are rewritten.
// Opaque scalar values, native UUID strings, dates, bytes, numeric types, and
// untouched maps/slices retain their original Go representation.
func (s *CanonicalBackendSession) resolveNativeActionResultValue(value any) (any, error) {
	if value == nil || len(s.nativeActionResults) == 0 {
		return value, nil
	}
	resolved, _, err := s.visitNativeActionResultValue(reflect.ValueOf(value), true)
	if err != nil {
		return nil, err
	}
	return resolved.Interface(), nil
}

func nativeResultString(value reflect.Value) (string, bool) {
	for value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			return "", false
		}
		value = value.Elem()
	}
	if value.IsValid() && value.Kind() == reflect.String {
		return value.String(), true
	}
	return "", false
}

func nativeResultReplaceString(value reflect.Value, replacement string) reflect.Value {
	result := reflect.New(value.Type()).Elem()
	if value.Kind() == reflect.Interface {
		result.Set(nativeResultReplaceString(value.Elem(), replacement))
	} else {
		result.SetString(replacement)
	}
	return result
}

var nativeResultBackendReferenceType = reflect.TypeOf(backend.Reference{})

func (s *CanonicalBackendSession) visitNativeActionResultValue(value reflect.Value, resolve bool) (reflect.Value, bool, error) {
	if !value.IsValid() {
		return value, false, nil
	}
	resolveID := func(id string) (string, bool, error) {
		if _, tracked := s.nativeActionResults[id]; !tracked {
			return id, false, nil
		}
		if !resolve {
			return "", false, fmt.Errorf("NATIVE_OUTPUT_REFERENCE_UNRESOLVED: an exact or unsupported raw payload contains a compiler result handle; use an explicit native.ref identity in opaque parameters")
		}
		actual, err := s.consumeNativeActionResult(id)
		return actual, err == nil && actual != id, err
	}
	switch value.Kind() {
	case reflect.Interface, reflect.Pointer:
		if value.IsNil() {
			return value, false, nil
		}
		child, changed, err := s.visitNativeActionResultValue(value.Elem(), resolve)
		if err != nil || !changed {
			return value, false, err
		}
		if value.Kind() == reflect.Pointer {
			copy := reflect.New(value.Type().Elem())
			copy.Elem().Set(child)
			return copy, true, nil
		}
		copy := reflect.New(value.Type()).Elem()
		copy.Set(child)
		return copy, true, nil
	case reflect.Map:
		if value.IsNil() || value.Type().Key().Kind() != reflect.String {
			return value, false, nil
		}
		copy := value
		changed := false
		set := func(key, child reflect.Value) {
			if !changed {
				copy = reflect.MakeMapWithSize(value.Type(), value.Len())
				iter := value.MapRange()
				for iter.Next() {
					copy.SetMapIndex(iter.Key(), iter.Value())
				}
				changed = true
			}
			copy.SetMapIndex(key, child)
		}
		key := func(name string) reflect.Value { return reflect.ValueOf(name).Convert(value.Type().Key()) }
		if kind, _ := nativeResultString(value.MapIndex(key("Type"))); kind == "ActionOutput" {
			idValue := value.MapIndex(key("OutputUUID"))
			if id, ok := nativeResultString(idValue); ok {
				actual, replaced, err := resolveID(id)
				if err != nil {
					return value, false, err
				}
				if replaced {
					set(key("OutputUUID"), nativeResultReplaceString(idValue, actual))
					nameValue := value.MapIndex(key("OutputName"))
					if name, ok := nativeResultString(nameValue); ok && name == id {
						set(key("OutputName"), nativeResultReplaceString(nameValue, actual))
					}
				}
			}
		}
		iter := copy.MapRange()
		for iter.Next() {
			child, childChanged, err := s.visitNativeActionResultValue(iter.Value(), resolve)
			if err != nil {
				return value, false, err
			}
			if childChanged {
				set(iter.Key(), child)
			}
		}
		return copy, changed, nil
	case reflect.Struct:
		copy := value
		changed := false
		set := func(index int, child reflect.Value) {
			if !changed {
				copy = reflect.New(value.Type()).Elem()
				copy.Set(value)
				changed = true
			}
			copy.Field(index).Set(child)
		}
		idName, outputName := "", ""
		if value.Type() == nativeResultBackendReferenceType {
			kind := backend.ReferenceKind(value.FieldByName("Kind").Int())
			if kind == backend.RefActionResult || kind == backend.RefLoopResult {
				idName, outputName = "ProducerID", "ProducerName"
			}
		} else if kind, _ := nativeResultString(value.FieldByName("Type")); kind == "ActionOutput" {
			idName, outputName = "OutputUUID", "OutputName"
		}
		if idName != "" {
			if field, ok := value.Type().FieldByName(idName); ok && len(field.Index) == 1 {
				idValue := value.FieldByIndex(field.Index)
				if id, ok := nativeResultString(idValue); ok {
					actual, replaced, err := resolveID(id)
					if err != nil {
						return value, false, err
					}
					if replaced {
						set(field.Index[0], nativeResultReplaceString(idValue, actual))
						if nameField, exists := value.Type().FieldByName(outputName); exists && len(nameField.Index) == 1 {
							nameValue := value.FieldByIndex(nameField.Index)
							if name, ok := nativeResultString(nameValue); ok && name == id {
								set(nameField.Index[0], nativeResultReplaceString(nameValue, actual))
							}
						}
					}
				}
			}
		}
		for index := 0; index < value.NumField(); index++ {
			if value.Type().Field(index).PkgPath != "" {
				continue
			}
			child, childChanged, err := s.visitNativeActionResultValue(copy.Field(index), resolve)
			if err != nil {
				return value, false, err
			}
			if childChanged {
				set(index, child)
			}
		}
		return copy, changed, nil
	case reflect.Slice, reflect.Array:
		if value.Type().Elem().Kind() == reflect.Uint8 || value.Kind() == reflect.Slice && value.IsNil() {
			return value, false, nil
		}
		copy := value
		changed := false
		for index := 0; index < value.Len(); index++ {
			child, childChanged, err := s.visitNativeActionResultValue(value.Index(index), resolve)
			if err != nil {
				return value, false, err
			}
			if childChanged {
				if !changed {
					if value.Kind() == reflect.Slice {
						copy = reflect.MakeSlice(value.Type(), value.Len(), value.Len())
						reflect.Copy(copy, value)
					} else {
						copy = reflect.New(value.Type()).Elem()
						copy.Set(value)
					}
					changed = true
				}
				copy.Index(index).Set(child)
			}
		}
		return copy, changed, nil
	default:
		return value, false, nil
	}
}

func (s *CanonicalBackendSession) validateNativeActionResults() error {
	if len(s.nativeActionResults) == 0 {
		return nil
	}
	producers := make(map[string]int)
	for _, action := range s.shortcut.WFWorkflowActions {
		if id, ok := action.WFWorkflowActionParameters["UUID"].(string); ok && id != "" {
			producers[id]++
		}
		if _, _, err := s.visitNativeActionResultValue(reflect.ValueOf(action.WFWorkflowActionParameters), false); err != nil {
			return err
		}
		if action.rawNativeEnvelope != nil {
			if _, _, err := s.visitNativeActionResultValue(reflect.ValueOf(action.rawNativeEnvelope), false); err != nil {
				return err
			}
		}
	}
	for _, binding := range s.nativeActionResults {
		if binding.consumedUUID == "" {
			continue
		}
		if actual, _ := binding.params["UUID"].(string); actual != binding.consumedUUID {
			return fmt.Errorf("NATIVE_OUTPUT_IDENTITY_CONFLICT: consumed producer identity changed before serialization")
		}
		switch producers[binding.consumedUUID] {
		case 0:
			return fmt.Errorf("NATIVE_OUTPUT_ID_MISSING: consumed native producer is absent from the emitted workflow")
		case 1:
		default:
			return fmt.Errorf("NATIVE_OUTPUT_AMBIGUOUS: consumed native output UUID identifies more than one emitted producer")
		}
	}
	return nil
}
