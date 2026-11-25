package main

import (
	"fmt"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

func legacyCacheDefinition() *actionDefinition {
	return &actionDefinition{minVersion: 27, outputType: Variable, parameters: []parameterDefinition{
		{name: "key", validType: String},
		{name: "ttl", validType: Float},
		{name: "global", validType: Bool, optional: true},
		{name: "sensitive", validType: Bool, optional: true},
		{name: "allowSensitive", validType: Bool, optional: true},
	}}
}
func legacyCacheOptions(arguments []actionArgument) (schema.CacheOptions, error) {
	values := map[string]any{}
	names := []string{"key", "ttl", "global", "sensitive", "allowSensitive"}
	if len(arguments) > len(names) {
		return schema.CacheOptions{}, fmt.Errorf("E_CACHE_OPTION: too many cache arguments")
	}
	for index, argument := range arguments {
		if argument.valueType == Nil && index >= 2 {
			continue
		}
		switch argument.valueType {
		case String, RawString:
			text, okay := argument.value.(string)
			if !okay || (argument.valueType == String && checkInlineVarRegex.MatchString(text)) {
				return schema.CacheOptions{}, fmt.Errorf("E_CACHE_LITERAL: cache options cannot interpolate values")
			}
			values[names[index]] = text
		case Integer, Float, Bool:
			values[names[index]] = argument.value
		default:
			return schema.CacheOptions{}, fmt.Errorf("E_CACHE_LITERAL: cache options must be literal")
		}
	}
	return schema.ResolveCacheOptions(values)
}
func legacyCacheKind(value tokenType) string {
	switch value {
	case String, RawString:
		return "text"
	case Integer, Float:
		return "number"
	case Bool:
		return "bool"
	default:
		return ""
	}
}
func emitLegacyCacheBlock(block *legacyScopedBlock, reference *WFActionReference) {
	options := *block.cache
	kind := legacyCacheKind(block.resultType)
	counter := 0
	nextID := func() string {
		counter++
		salt := fmt.Sprintf("%s_native_%d", block.prefix, counter)
		return createUUID(&salt)
	}
	result, err := backend.EmitScopedCache(
		backend.ScopedCacheConfig{Namespace: options.Namespace(kind), Kind: kind, TTL: options.TTL, Global: options.Global},
		block.prefix,
		nextID,
		func(id string, params map[string]any, uuid, name string) error {
			// Reuse the canonical parameter codec for every cache action. The parsed
			// body still emits through its ordinary Legacy carrier at this exact point.
			session := &CanonicalBackendSession{shortcut: shortcut}
			if err := session.EmitRawAction(id, params, uuid, name, ""); err != nil {
				return err
			}
			shortcut = session.shortcut
			return nil
		},
		func() (*backend.Reference, error) {
			value, err := emitLegacyScopedBodyValue(block)
			if err != nil {
				return nil, err
			}
			source := value.Value
			switch source.Type {
			case "ActionOutput":
				return &backend.Reference{Kind: backend.RefActionResult, ProducerID: source.OutputUUID, ProducerName: source.OutputName}, nil
			case "Variable":
				return &backend.Reference{Kind: backend.RefMutableBinding, ProducerName: source.VariableName}, nil
			default:
				return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: unsupported native scalar source reference")
			}
		},
	)
	if err != nil {
		exit(err.Error())
		return
	}
	if reference != nil && (reference.UUID != "" || reference.CustomOutputName != "") {
		id, name := reference.UUID, reference.CustomOutputName
		if id == "" {
			id = nextID()
		}
		token := EncodeReferenceAttachment(ReferenceToDescriptor(result), "WFTextTokenAttachment")
		addStdAction("getvariable", attachReferenceToParams(map[string]any{"WFVariable": token}, &WFActionReference{UUID: id, CustomOutputName: name}))
	}
}
