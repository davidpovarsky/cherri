package backend

import (
	"fmt"
	"math"
	"strconv"
)

type ScopedCacheConfig struct {
	Namespace, Kind string
	TTL             float64
	Global          bool
}
type ScopedCacheEmitter func(identifier string, parameters map[string]any, outputUUID, outputName string) error

// EmitScopedCache generates the same lazy native lifecycle for both dialects.
// It does not execute storage in Go, add a backend or claim catch/atomicity.
// Malformed JSON and native I/O failures retain their native abort behavior.
func EmitScopedCache(config ScopedCacheConfig, prefix string, nextID func() string, emit ScopedCacheEmitter, body func() (*Reference, error)) (*Reference, error) {
	if config.Namespace == "" || prefix == "" || nextID == nil || emit == nil || body == nil {
		return nil, fmt.Errorf("E_CACHE_CONTRACT: incomplete native lifecycle")
	}
	if math.IsNaN(config.TTL) || math.IsInf(config.TTL, 0) || config.TTL < 0 {
		return nil, fmt.Errorf("E_CACHE_TTL: invalid native lifecycle TTL")
	}
	if config.Kind != "text" && config.Kind != "number" && config.Kind != "bool" && config.Kind != "date" {
		return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: unsupported archive type")
	}
	b := &scopedCacheEmitter{config: config, prefix: prefix, nextID: nextID, emit: emit}
	if config.TTL == 0 {
		value, err := body()
		if err != nil {
			return nil, err
		}
		if value == nil {
			return nil, fmt.Errorf("E_CACHE_VALUE_TYPE: body did not produce a scalar")
		}
		return b.produce("getvariable", map[string]any{"WFVariable": cacheReference(value)}), b.err
	}
	// Capture the actual native input before any cache helper. An omitted
	// WFInput preserves the complete content sequence, including no content;
	// it is not a fabricated Text, Dictionary or null payload.
	b.add("setvariable", map[string]any{"WFVariableName": b.variable("input")})
	b.zero = b.number(0)
	b.one = b.number(1)
	b.set("hit", b.zero)
	text := b.produce("gettext", map[string]any{"WFTextActionText": "Cherri cache type witness"})
	b.textType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(text)})
	b.numberType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(b.zero)})
	dictionary := b.produce("dictionary", map[string]any{"WFItems": SemanticValue{Kind: ValDict, DictVal: []DictEntry{{Key: "witness", Value: SemanticValue{Kind: ValBool, BoolVal: true}}}}})
	b.dictionaryType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(dictionary)})
	b.epoch = b.produce("date", map[string]any{"WFDateActionMode": "Specified Date", "WFDateActionDate": "1970-01-01T00:00:00Z"})
	if config.Kind == "date" {
		b.dateType = b.produce("getitemtype", map[string]any{"WFInput": cacheReference(b.epoch)})
	}
	now := b.now()
	stored := b.produce("getstoredcontent", map[string]any{"WFStoredContentKey": config.Namespace, "WFStoredContentGlobalValue": config.Global})
	b.when(b.count(stored), 4, 1, OperandNumber, func() {
		b.withType(stored, b.textType, func() {
			envelope := b.produce("detect.dictionary", map[string]any{"WFInput": cacheReference(stored)})
			b.when(b.count(envelope), 4, 1, OperandNumber, func() {
				b.withType(envelope, b.dictionaryType, func() { b.readEnvelope(envelope, now) })
			})
		})
	})
	b.when(b.get("hit"), 4, 0, OperandNumber, func() {
		if b.err != nil {
			return
		}
		// Restore only on the lazy miss path, immediately before authored body actions.
		b.produce("getvariable", map[string]any{"WFVariable": cacheVariable(b.variable("input"))})
		if b.err != nil {
			return
		}
		value, err := body()
		if err != nil {
			b.err = err
			return
		}
		if value == nil {
			b.err = fmt.Errorf("E_CACHE_VALUE_TYPE: body did not produce a scalar")
			return
		}
		// Snapshot an actual mutable/system value before validation, clocks or I/O.
		value = b.produce("getvariable", map[string]any{"WFVariable": cacheReference(value)})
		b.set("result", value)
		b.storeEnvelope(value)
	})
	result := b.produce("getvariable", map[string]any{"WFVariable": cacheVariable(b.variable("result"))})
	return result, b.err
}

type scopedCacheEmitter struct {
	config                                                           ScopedCacheConfig
	prefix                                                           string
	nextID                                                           func() string
	emit                                                             ScopedCacheEmitter
	err                                                              error
	zero, one, textType, numberType, dictionaryType, dateType, epoch *Reference
}

func cacheReference(value *Reference) SemanticValue {
	return SemanticValue{Kind: ValReference, Ref: value}
}
func cacheVariable(name string) SemanticValue {
	return cacheReference(&Reference{Kind: RefMutableBinding, ProducerName: name})
}
func cacheText(parts ...any) SemanticValue {
	var segments []TextSegment
	for _, part := range parts {
		switch value := part.(type) {
		case string:
			segments = append(segments, TextSegment{Text: value})
		case *Reference:
			segments = append(segments, TextSegment{IsExpr: true, Reference: value})
		default:
			panic("invalid internal cache text part")
		}
	}
	return SemanticValue{Kind: ValTextSegments, Segments: segments}
}
func (b *scopedCacheEmitter) variable(suffix string) string { return b.prefix + "_" + suffix }
func (b *scopedCacheEmitter) add(id string, p map[string]any) {
	if b.err == nil {
		b.err = b.emit("is.workflow.actions."+id, p, "", "")
	}
}
func (b *scopedCacheEmitter) produce(id string, p map[string]any) *Reference {
	out := b.nextID()
	name := b.prefix + "_" + out
	if b.err == nil {
		b.err = b.emit("is.workflow.actions."+id, p, out, name)
	}
	return &Reference{Kind: RefActionResult, ProducerID: out, ProducerName: name}
}
func (b *scopedCacheEmitter) number(value float64) *Reference {
	return b.produce("number", map[string]any{"WFNumberActionNumber": strconv.FormatFloat(value, 'g', -1, 64)})
}
func (b *scopedCacheEmitter) set(name string, value *Reference) {
	b.add("setvariable", map[string]any{"WFVariableName": b.variable(name), "WFInput": cacheReference(value)})
}
func (b *scopedCacheEmitter) get(name string) *Reference {
	return b.produce("getvariable", map[string]any{"WFVariable": cacheVariable(b.variable(name))})
}
func (b *scopedCacheEmitter) count(value *Reference) *Reference {
	return b.produce("count", map[string]any{"Input": cacheReference(value), "WFCountType": "Items"})
}
func (b *scopedCacheEmitter) hash(value *Reference) *Reference {
	return b.produce("hash", map[string]any{"WFInput": cacheReference(value), "WFHashType": "SHA256"})
}
func (b *scopedCacheEmitter) lookup(value *Reference, key string) *Reference {
	return b.produce("getvalueforkey", map[string]any{"WFInput": cacheReference(value), "WFDictionaryKey": key, "WFGetDictionaryValueType": "Value"})
}
func (b *scopedCacheEmitter) math(value *Reference, operation string, operand any) *Reference {
	return b.produce("math", map[string]any{"WFInput": cacheReference(value), "WFMathOperation": operation, "WFMathOperand": operand})
}
func (b *scopedCacheEmitter) when(value *Reference, code int, comparison any, kind ConditionOperandType, body func()) {
	group := b.nextID()
	b.add("conditional", BuildIfStartTyped(group, cacheReference(value), code, comparison, nil, kind, true))
	body()
	b.add("conditional", BuildIfEnd(group, ""))
}
func (b *scopedCacheEmitter) withType(value, witness *Reference, body func()) {
	actual := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(value)})
	b.when(actual, 4, cacheText(witness), OperandString, body)
}
func (b *scopedCacheEmitter) rejectIf(value *Reference, code int, comparison any, kind ConditionOperandType) {
	b.when(value, code, comparison, kind, func() { b.set("valid", b.zero) })
}
func (b *scopedCacheEmitter) now() *Reference {
	current := b.produce("date", map[string]any{"WFDateActionMode": "Current Date"})
	return b.produce("gettimebetweendates", map[string]any{
		"WFTimeUntilFromDate": cacheText(b.epoch), "WFInput": cacheText(current), "WFTimeUntilUnit": "Seconds",
	})
}
func (b *scopedCacheEmitter) readEnvelope(envelope, now *Reference) {
	// Inspect the complete key set before any lookup. A missing field is not
	// confused with false, zero, empty text or an implicit Previous input.
	keys := b.produce("getvalueforkey", map[string]any{"WFInput": cacheReference(envelope), "WFGetDictionaryValueType": "All Keys"})
	b.when(b.count(keys), 4, 7, OperandNumber, func() {
		b.set("keys", b.zero)
		group := b.nextID()
		b.add("repeat.each", BuildRepeatEachStart(group, cacheReference(keys)))
		item := b.produce("getvariable", map[string]any{"WFVariable": cacheVariable("Repeat Item")})
		digest := b.hash(item)
		for _, key := range []string{"version", "typeTag", "created", "expires", "ttl", "value", "namespace"} {
			literal := b.produce("gettext", map[string]any{"WFTextActionText": key})
			b.when(digest, 4, cacheText(b.hash(literal)), OperandString, func() {
				b.set("keys", b.math(b.get("keys"), "+", 1))
			})
		}
		b.add("nothing", map[string]any{})
		b.add("repeat.each", BuildLoopEnd(group, ""))
		b.when(b.get("keys"), 4, 7, OperandNumber, func() { b.readKnownEnvelope(envelope, now) })
	})
}
func (b *scopedCacheEmitter) readKnownEnvelope(envelope, now *Reference) {
	b.set("valid", b.one)
	fields := map[string]*Reference{}
	for _, key := range []string{"version", "typeTag", "created", "expires", "ttl", "value", "namespace"} {
		fields[key] = b.lookup(envelope, key)
		b.rejectIf(b.count(fields[key]), 5, 1, OperandNumber)
	}
	for _, key := range []string{"version", "created", "expires", "ttl"} {
		actual := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(fields[key])})
		b.rejectIf(actual, 5, cacheText(b.numberType), OperandString)
	}
	for _, key := range []string{"typeTag", "namespace"} {
		actual := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(fields[key])})
		b.rejectIf(actual, 5, cacheText(b.textType), OperandString)
	}
	b.when(b.get("valid"), 4, 1, OperandNumber, func() {
		b.rejectIf(fields["version"], 5, 1, OperandNumber)
		b.rejectIf(fields["ttl"], 5, b.config.TTL, OperandNumber)
		for _, field := range []struct{ key, expected string }{{"typeTag", b.config.Kind}, {"namespace", b.config.Namespace}} {
			key, expected := field.key, field.expected
			literal := b.produce("gettext", map[string]any{"WFTextActionText": expected})
			b.rejectIf(b.hash(fields[key]), 5, cacheText(b.hash(literal)), OperandString)
		}
		b.rejectIf(now, 0, cacheText(fields["created"]), OperandNumber)
		b.rejectIf(now, 3, cacheText(fields["expires"]), OperandNumber)
		expectedExpiry := b.math(fields["created"], "+", b.config.TTL)
		b.rejectIf(fields["expires"], 5, cacheText(expectedExpiry), OperandNumber)
		valueType := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(fields["value"])})
		witness := b.numberType
		if b.config.Kind == "text" {
			witness = b.textType
		}
		b.rejectIf(valueType, 5, cacheText(witness), OperandString)
		b.when(b.get("valid"), 4, 1, OperandNumber, func() {
			value := fields["value"]
			if b.config.Kind == "text" {
				unmatched := b.produce("text.replace", map[string]any{"WFInput": cacheReference(value), "WFReplaceTextFind": "^v:(?:[A-Za-z0-9+/]{4})*(?:[A-Za-z0-9+/]{2}==|[A-Za-z0-9+/]{3}=)?$", "WFReplaceTextReplace": "", "WFReplaceTextRegularExpression": true, "WFReplaceTextCaseSensitive": true})
				remaining := b.produce("count", map[string]any{"Input": cacheReference(unmatched), "WFCountType": "Characters"})
				b.rejectIf(remaining, 5, 0, OperandNumber)
			} else if b.config.Kind == "bool" {
				b.when(value, 5, 0, OperandNumber, func() { b.rejectIf(value, 5, 1, OperandNumber) })
			}
			b.when(b.get("valid"), 4, 1, OperandNumber, func() {
				if b.config.Kind == "text" {
					characters := b.produce("count", map[string]any{"Input": cacheReference(value), "WFCountType": "Characters"})
					b.when(characters, 4, 2, OperandNumber, func() {
						b.set("result", b.produce("gettext", map[string]any{"WFTextActionText": ""}))
						b.set("hit", b.one)
					})
					b.when(characters, 2, 2, OperandNumber, func() {
						encoded := b.produce("text.replace", map[string]any{"WFInput": cacheReference(value), "WFReplaceTextFind": "^v:", "WFReplaceTextReplace": "", "WFReplaceTextRegularExpression": true, "WFReplaceTextCaseSensitive": true})
						decoded := b.produce("base64encode", map[string]any{"WFInput": cacheReference(encoded), "WFEncodeMode": "Decode"})
						text := b.produce("gettext", map[string]any{"WFTextActionText": cacheText(decoded)})
						// Only an exact canonical UTF-8/base64 roundtrip earns a
						// cache hit. Invalid UTF-8 or noncanonical pad bits must
						// not become silently changed Text through Get Text.
						reencoded := b.produce("base64encode", map[string]any{"WFInput": cacheReference(text), "WFEncodeMode": "Encode", "WFBase64LineBreakMode": "None"})
						b.when(b.hash(encoded), 4, cacheText(b.hash(reencoded)), OperandString, func() {
							b.set("result", text)
							b.set("hit", b.one)
						})
					})
				} else if b.config.Kind == "date" {
					b.readDatePayload(value)
				} else {
					b.set("result", value)
					b.set("hit", b.one)
				}
			})
		})
	})
}
func (b *scopedCacheEmitter) storeEnvelope(value *Reference) {
	// Native scalar contracts are checked before storage. Failure is explicit:
	// no typed source promise is satisfied by coercing an unexpected collection.
	if b.config.Kind != "text" {
		b.failIf(b.count(value), 5, 1, OperandNumber)
		actual := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(value)})
		witness := b.numberType
		if b.config.Kind == "date" {
			witness = b.dateType
		}
		b.failIf(actual, 5, cacheText(witness), OperandString)
		if b.config.Kind == "bool" {
			b.when(value, 5, 0, OperandNumber, func() { b.failIf(value, 5, 1, OperandNumber) })
		}
	}
	payload := value
	if b.config.Kind == "date" {
		payload = b.writeDatePayload(value)
	}
	created := b.now()
	expires := b.math(created, "+", b.config.TTL)
	if b.config.Kind == "text" {
		// Native empty Text may expose zero content items. Source type/presence is
		// already checked; archive that precise empty value without a required-input
		// base64 operation. Nonempty values must be one native Text item.
		b.set("payload", b.produce("gettext", map[string]any{"WFTextActionText": "v:"}))
		count := b.count(value)
		b.failIf(count, 2, 1, OperandNumber)
		b.when(count, 4, 1, OperandNumber, func() {
			actual := b.produce("getitemtype", map[string]any{"WFInput": cacheReference(value)})
			b.failIf(actual, 5, cacheText(b.textType), OperandString)
			encoded := b.produce("base64encode", map[string]any{"WFInput": cacheReference(value), "WFEncodeMode": "Encode", "WFBase64LineBreakMode": "None"})
			b.set("payload", b.produce("gettext", map[string]any{"WFTextActionText": cacheText("v:", encoded)}))
		})
		payload = b.get("payload")
	}
	// Typed native Number rows prevent the dynamic reference from becoming a
	// Text dictionary field. Text payloads are explicitly base64 UTF-8 archives.
	fields := []any{
		cacheField("version", 3, "1"),
		cacheField("typeTag", 0, b.config.Kind),
		cacheField("created", 3, cacheText(created)),
		cacheField("expires", 3, cacheText(expires)),
		cacheField("ttl", 3, strconv.FormatFloat(b.config.TTL, 'g', -1, 64)),
		cacheField("namespace", 0, b.config.Namespace),
	}
	valueType := 3
	if b.config.Kind == "text" {
		valueType = 0
	}
	fields = append(fields, cacheField("value", valueType, cacheText(payload)))
	envelope := b.produce("dictionary", map[string]any{"WFItems": map[string]any{
		"WFSerializationType": "WFDictionaryFieldValue", "Value": map[string]any{"WFDictionaryFieldValueItems": fields},
	}})
	archive := b.produce("gettext", map[string]any{"WFTextActionText": cacheText(envelope)})
	// A positive TTL smaller than native clock precision must not create an
	// immortal/negative entry. The body value still returns without persistence.
	b.when(expires, 2, cacheText(created), OperandNumber, func() {
		b.add("setstoredcontent", map[string]any{"WFStoredContentKey": b.config.Namespace, "WFStoredContentGlobalValue": b.config.Global, "WFInput": cacheReference(archive)})
	})
}
func (b *scopedCacheEmitter) failIf(value *Reference, code int, comparison any, kind ConditionOperandType) {
	b.when(value, code, comparison, kind, func() {
		b.add("alert", map[string]any{"WFAlertActionMessage": "CACHE_SCALAR_CONTRACT_FAILED", "WFAlertActionCancelButtonShown": false})
		b.add("exit", map[string]any{})
	})
}
func cacheField(key string, kind int, value any) map[string]any {
	if text, okay := value.(string); okay {
		value = SemanticValue{Kind: ValString, StrVal: text}
	}
	return map[string]any{
		"WFItemType": kind,
		"WFKey":      SemanticValue{Kind: ValString, StrVal: key},
		"WFValue":    value,
	}
}
