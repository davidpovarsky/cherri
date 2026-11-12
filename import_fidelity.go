/*
 * Copyright (c) Cherri
 */

package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/electrikmilk/args-parser"
	"howett.net/plist"
)

// Lossless import/compile transient compiler state (reset by resetCompilerState).
var importedWorkflowHeader map[string]any
var exactActionParamsPresence map[int]bool

// rawPlistDict preserves an exact plist parameter dictionary without legacy dictionary or scalar coercion.
type rawPlistDict struct {
	Data map[string]any
}

// ControlFlowViolationInfo records a structural control-flow defect in a source Shortcut.
type ControlFlowViolationInfo struct {
	Code        string `json:"code"`
	ActionIndex int    `json:"actionIndex"`
	Identifier  string `json:"identifier"`
	GroupingID  string `json:"groupingIdentifier,omitempty"`
	Message     string `json:"message"`
}

// NativeActionRecord represents a single validated native Shortcut action from a decoded Shortcut document.
type NativeActionRecord struct {
	Index               int
	Identifier          string
	Parameters          map[string]any
	HadParametersKey    bool
	UUID                string
	CustomOutputName    string
	GroupingIdentifier  string
	ControlFlowMode     *int
	AppIntentDescriptor map[string]any
	ReferencedUUIDs     []string
	ReferencedVars      []string
	SourceValidity      string // "VALID" or "SOURCE_INVALID:<reason>"
}

// NativeShortcutDocument is the single shared native representation decoded by all import paths.
type NativeShortcutDocument struct {
	RawRoot              map[string]any
	Header               map[string]any
	HasActionsKey        bool
	Actions              []NativeActionRecord
	SourceFormat         string // "binary-plist", "xml-plist", "json"
	SourceHash           string // SHA-256 hex of raw input bytes
	ControlFlowValid     bool
	ControlFlowViolation *ControlFlowViolationInfo
	ProducerUUIDIndex    map[string]int   // UUID -> action index
	ConsumerUUIDIndex    map[string][]int // UUID -> consuming action indices
}

func isLosslessImportFidelity() bool {
	f := strings.TrimSpace(args.Value("fidelity"))
	if f == "" {
		f = strings.TrimSpace(args.Value("import-fidelity"))
	}
	return strings.EqualFold(f, "lossless")
}

func isHybridImportFidelity() bool {
	f := strings.TrimSpace(args.Value("fidelity"))
	if f == "" {
		f = strings.TrimSpace(args.Value("import-fidelity"))
	}
	return strings.EqualFold(f, "hybrid")
}

// DecodeNativeShortcutDocument decodes and strictly validates an unsigned Shortcut from binary plist,
// XML plist, or JSON into the shared NativeShortcutDocument representation.
func DecodeNativeShortcutDocument(b []byte) (*NativeShortcutDocument, error) {
	if len(b) >= 4 && string(b[:4]) == "AEA1" {
		return nil, fmt.Errorf("signed Shortcut container (AEA1) is not supported: provide an unsigned plist/JSON or iCloud link")
	}
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty shortcut input")
	}

	sum := sha256.Sum256(b)
	sourceHash := hex.EncodeToString(sum[:])

	var raw map[string]any
	var sourceFormat string

	if trimmed[0] == '{' || trimmed[0] == '[' {
		if trimmed[0] != '{' {
			return nil, fmt.Errorf("invalid JSON shortcut at root: expected JSON object, got array")
		}
		sourceFormat = "json"
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		var decodedAny any
		if err := dec.Decode(&decodedAny); err != nil {
			return nil, fmt.Errorf("invalid JSON shortcut: %w", err)
		}
		var trailing any
		if err := dec.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("invalid JSON shortcut: trailing data after root JSON object")
		}
		decodedMap, ok := decodedAny.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid JSON shortcut at root: expected dictionary object")
		}
		normalized, _ := normalizeRawJSONNumbers(decodedMap).(map[string]any)
		raw = normalized
	} else {
		if hasSignedBytes(b) {
			return nil, fmt.Errorf("signed or unrecognized binary Shortcut container is not supported: expected unsigned bplist00, XML plist, or JSON")
		}
		plistInput := trimmed
		if bytes.HasPrefix(b, []byte("bplist00")) {
			plistInput = b
		}
		var decodedAny any
		format, err := plist.Unmarshal(plistInput, &decodedAny)
		if err != nil {
			return nil, fmt.Errorf("invalid plist shortcut: %w", err)
		}
		if decodedAny == nil {
			raw = map[string]any{}
		} else {
			decodedMap, ok := decodedAny.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("invalid plist shortcut at root: expected dictionary, got %T", decodedAny)
			}
			raw = decodedMap
		}
		if format == plist.BinaryFormat || bytes.HasPrefix(b, []byte("bplist00")) {
			sourceFormat = "binary-plist"
		} else {
			sourceFormat = "xml-plist"
		}
	}

	if raw == nil {
		raw = map[string]any{}
	}

	header := make(map[string]any, len(raw))
	for k, v := range raw {
		if k == "WFWorkflowActions" {
			continue
		}
		header[k] = deepClonePlistValue(v)
	}

	rawActs, hasActions := raw["WFWorkflowActions"]
	var rawActionList []any
	if hasActions {
		switch typed := rawActs.(type) {
		case []any:
			rawActionList = typed
		case []map[string]any:
			rawActionList = make([]any, len(typed))
			for i, m := range typed {
				rawActionList[i] = m
			}
		default:
			return nil, fmt.Errorf("invalid WFWorkflowActions at root.WFWorkflowActions: expected array of action dictionaries, got %T", rawActs)
		}
	}

	records := make([]NativeActionRecord, 0, len(rawActionList))
	producerUUIDIndex := make(map[string]int)
	consumerUUIDIndex := make(map[string][]int)

	type cfFrame struct {
		groupingID string
		actionIdx  int
		identifier string
	}
	var cfStack []cfFrame
	cfValid := true
	var cfViolation *ControlFlowViolationInfo

	for idx, rawItem := range rawActionList {
		actMap, ok := rawItem.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid action at WFWorkflowActions[%d]: expected dictionary, got %T", idx, rawItem)
		}
		rawIdent, hasIdent := actMap["WFWorkflowActionIdentifier"]
		if !hasIdent {
			return nil, fmt.Errorf("invalid action at WFWorkflowActions[%d].WFWorkflowActionIdentifier: missing required string key", idx)
		}
		ident, ok := rawIdent.(string)
		if !ok || strings.TrimSpace(ident) == "" {
			return nil, fmt.Errorf("invalid action at WFWorkflowActions[%d].WFWorkflowActionIdentifier: expected non-empty string, got %T", idx, rawIdent)
		}

		var params map[string]any
		rawParams, hasParams := actMap["WFWorkflowActionParameters"]
		if hasParams {
			if rawParams == nil {
				params = map[string]any{}
			} else if pm, ok := rawParams.(map[string]any); ok {
				params = deepClonePlistMap(pm)
			} else {
				return nil, fmt.Errorf("invalid action at WFWorkflowActions[%d].WFWorkflowActionParameters (%s): expected dictionary, got %T", idx, ident, rawParams)
			}
		}

		rec := NativeActionRecord{
			Index:            idx,
			Identifier:       ident,
			Parameters:       params,
			HadParametersKey: hasParams,
			SourceValidity:   "VALID",
		}

		if params != nil {
			if u, ok := params["UUID"].(string); ok && u != "" {
				rec.UUID = u
				if _, exists := producerUUIDIndex[u]; !exists {
					producerUUIDIndex[u] = idx
				}
			}
			if cn, ok := params["CustomOutputName"].(string); ok && cn != "" {
				rec.CustomOutputName = cn
			}
			if gid, ok := params["GroupingIdentifier"].(string); ok && gid != "" {
				rec.GroupingIdentifier = gid
			}
			if rawCF, hasCF := params["WFControlFlowMode"]; hasCF {
				if iv, ok := asSignedOrUnsignedInt(rawCF); ok {
					var cfInt int
					if iv.signed {
						cfInt = int(iv.sVal)
					} else {
						cfInt = int(iv.uVal)
					}
					rec.ControlFlowMode = &cfInt
				}
			}
			if desc, ok := params["AppIntentDescriptor"].(map[string]any); ok {
				rec.AppIntentDescriptor = deepClonePlistMap(desc)
			}

			uuidSet := make(map[string]struct{})
			varSet := make(map[string]struct{})
			collectActionReferencesRecursive(params, uuidSet, varSet)
			if len(uuidSet) > 0 {
				rec.ReferencedUUIDs = make([]string, 0, len(uuidSet))
				for u := range uuidSet {
					rec.ReferencedUUIDs = append(rec.ReferencedUUIDs, u)
				}
				sort.Strings(rec.ReferencedUUIDs)
				for _, u := range rec.ReferencedUUIDs {
					consumerUUIDIndex[u] = append(consumerUUIDIndex[u], idx)
				}
			}
			if len(varSet) > 0 {
				rec.ReferencedVars = make([]string, 0, len(varSet))
				for vn := range varSet {
					rec.ReferencedVars = append(rec.ReferencedVars, vn)
				}
				sort.Strings(rec.ReferencedVars)
			}
		}

		if rec.ControlFlowMode != nil {
			modeVal := *rec.ControlFlowMode
			switch modeVal {
			case 0:
				cfStack = append(cfStack, cfFrame{
					groupingID: rec.GroupingIdentifier,
					actionIdx:  idx,
					identifier: ident,
				})
			case 1:
				if len(cfStack) == 0 || cfStack[len(cfStack)-1].groupingID != rec.GroupingIdentifier {
					cfValid = false
					rec.SourceValidity = "SOURCE_INVALID:CONTROL_FLOW_ORPHAN_MIDDLE"
					if cfViolation == nil {
						cfViolation = &ControlFlowViolationInfo{
							Code:        "CONTROL_FLOW_ORPHAN_MIDDLE",
							ActionIndex: idx,
							Identifier:  ident,
							GroupingID:  rec.GroupingIdentifier,
							Message:     fmt.Sprintf("action %d (%s) has WFControlFlowMode=1 without matching open GroupingIdentifier %q", idx, ident, rec.GroupingIdentifier),
						}
					}
				}
			case 2:
				if len(cfStack) == 0 || cfStack[len(cfStack)-1].groupingID != rec.GroupingIdentifier {
					cfValid = false
					rec.SourceValidity = "SOURCE_INVALID:CONTROL_FLOW_ORPHAN_END"
					if cfViolation == nil {
						cfViolation = &ControlFlowViolationInfo{
							Code:        "CONTROL_FLOW_ORPHAN_END",
							ActionIndex: idx,
							Identifier:  ident,
							GroupingID:  rec.GroupingIdentifier,
							Message:     fmt.Sprintf("action %d (%s) has WFControlFlowMode=2 without matching open GroupingIdentifier %q", idx, ident, rec.GroupingIdentifier),
						}
					}
				} else {
					cfStack = cfStack[:len(cfStack)-1]
				}
			}
		}

		records = append(records, rec)
	}

	if len(cfStack) > 0 {
		cfValid = false
		top := cfStack[len(cfStack)-1]
		if top.actionIdx >= 0 && top.actionIdx < len(records) && records[top.actionIdx].SourceValidity == "VALID" {
			records[top.actionIdx].SourceValidity = "SOURCE_INVALID:CONTROL_FLOW_UNCLOSED_START"
		}
		if cfViolation == nil {
			cfViolation = &ControlFlowViolationInfo{
				Code:        "CONTROL_FLOW_UNCLOSED_START",
				ActionIndex: top.actionIdx,
				Identifier:  top.identifier,
				GroupingID:  top.groupingID,
				Message:     fmt.Sprintf("action %d (%s) opened GroupingIdentifier %q without matching WFControlFlowMode=2", top.actionIdx, top.identifier, top.groupingID),
			}
		}
	}

	return &NativeShortcutDocument{
		RawRoot:              deepClonePlistMap(raw),
		Header:               header,
		HasActionsKey:        hasActions,
		Actions:              records,
		SourceFormat:         sourceFormat,
		SourceHash:           sourceHash,
		ControlFlowValid:     cfValid,
		ControlFlowViolation: cfViolation,
		ProducerUUIDIndex:    producerUUIDIndex,
		ConsumerUUIDIndex:    consumerUUIDIndex,
	}, nil
}

func collectActionReferencesRecursive(v any, uuidSet map[string]struct{}, varSet map[string]struct{}) {
	switch x := v.(type) {
	case map[string]any:
		if outUUID, ok := x["OutputUUID"].(string); ok && outUUID != "" {
			uuidSet[outUUID] = struct{}{}
		}
		if vn, ok := x["VariableName"].(string); ok && vn != "" {
			varSet[vn] = struct{}{}
		}
		if wvn, ok := x["WFVariableName"].(string); ok && wvn != "" {
			varSet[wvn] = struct{}{}
		}
		for k, val := range x {
			if k == "UUID" {
				continue
			}
			collectActionReferencesRecursive(val, uuidSet, varSet)
		}
	case []any:
		for _, item := range x {
			collectActionReferencesRecursive(item, uuidSet, varSet)
		}
	}
}

func decodeShortcutRawMap(b []byte) (map[string]any, error) {
	trimmed := bytes.TrimSpace(b)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty shortcut input")
	}

	if trimmed[0] == '{' {
		dec := json.NewDecoder(bytes.NewReader(trimmed))
		dec.UseNumber()
		var raw map[string]any
		if err := dec.Decode(&raw); err != nil {
			return nil, fmt.Errorf("invalid JSON shortcut: %w", err)
		}
		var trailing any
		if err := dec.Decode(&trailing); err != io.EOF {
			return nil, fmt.Errorf("invalid JSON shortcut: trailing data after root JSON object")
		}
		if raw == nil {
			return map[string]any{}, nil
		}
		normalized, _ := normalizeRawJSONNumbers(raw).(map[string]any)
		return normalized, nil
	}

	plistInput := trimmed
	if bytes.HasPrefix(b, []byte("bplist00")) {
		plistInput = b
	}
	var raw map[string]any
	if _, err := plist.Unmarshal(plistInput, &raw); err != nil {
		return nil, fmt.Errorf("invalid plist shortcut: %w", err)
	}
	if raw == nil {
		return map[string]any{}, nil
	}
	return raw, nil
}

func normalizeRawJSONNumbers(v any) any {
	switch x := v.(type) {
	case json.Number:
		s := x.String()
		if !strings.ContainsAny(s, ".eE") {
			if i, err := strconv.ParseInt(s, 10, 64); err == nil {
				return i
			}
			if u, err := strconv.ParseUint(s, 10, 64); err == nil {
				return u
			}
		}
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return f
		}
		return s
	case map[string]any:
		res := make(map[string]any, len(x))
		for k, val := range x {
			res[k] = normalizeRawJSONNumbers(val)
		}
		return res
	case []any:
		res := make([]any, len(x))
		for i, val := range x {
			res[i] = normalizeRawJSONNumbers(val)
		}
		return res
	default:
		return v
	}
}

func splitWorkflowHeaderAndActions(doc map[string]any) (map[string]any, []map[string]any, bool) {
	header := make(map[string]any, len(doc))
	for k, v := range doc {
		if k == "WFWorkflowActions" {
			continue
		}
		header[k] = deepClonePlistValue(v)
	}

	rawActs, hasActions := doc["WFWorkflowActions"]
	if !hasActions {
		return header, nil, false
	}

	var actions []map[string]any
	switch typed := rawActs.(type) {
	case []map[string]any:
		actions = make([]map[string]any, 0, len(typed))
		for _, a := range typed {
			actions = append(actions, deepClonePlistMap(a))
		}
	case []any:
		actions = make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				actions = append(actions, deepClonePlistMap(m))
			} else if m, ok := item.(map[string]interface{}); ok {
				actions = append(actions, deepClonePlistMap(m))
			}
		}
	}
	return header, actions, true
}

// PreOverlayActionHook is an optional test/audit hook called with compiled actions immediately
// before applyImportProvenanceToCompiledActions overlays metadata onto them.
var PreOverlayActionHook func(compiledActions []any)

func deepClonePlistSlice(s []any) []any {
	if s == nil {
		return nil
	}
	res := make([]any, len(s))
	for i, item := range s {
		res[i] = deepClonePlistValue(item)
	}
	return res
}

func deepClonePlistMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	res := make(map[string]any, len(m))
	for k, v := range m {
		res[k] = deepClonePlistValue(v)
	}
	return res
}

func deepClonePlistValue(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case rawPlistDict:
		return deepClonePlistMap(x.Data)
	case *rawPlistDict:
		if x == nil {
			return nil
		}
		return deepClonePlistMap(x.Data)
	case map[string]any:
		return deepClonePlistMap(x)
	case []any:
		res := make([]any, len(x))
		for i, item := range x {
			res[i] = deepClonePlistValue(item)
		}
		return res
	case []map[string]any:
		res := make([]any, len(x))
		for i, item := range x {
			res[i] = deepClonePlistMap(item)
		}
		return res
	case []string:
		res := make([]any, len(x))
		for i, item := range x {
			res[i] = item
		}
		return res
	case []byte:
		res := make([]byte, len(x))
		copy(res, x)
		return res
	default:
		return v
	}
}

// mergeWorkflowHeaderMap merges exact or metadata-level raw workflow headers with compiled actions.
// When fillMissingDefaults is false, every header key and empty-vs-absent distinction from rawHeader
// is preserved verbatim and WFWorkflowActionParameters empty-dict vs absent presence is respected.
func mergeWorkflowHeaderMap(
	rawHeader map[string]any,
	actions []ShortcutAction,
	exactParamsPresence map[int]bool,
	fallback Shortcut,
	fillMissingDefaults bool,
) map[string]any {
	compiledActions := make([]any, 0, len(actions))
	for i, act := range actions {
		actMap := map[string]any{
			"WFWorkflowActionIdentifier": act.WFWorkflowActionIdentifier,
		}
		if len(act.WFWorkflowActionParameters) > 0 {
			actMap["WFWorkflowActionParameters"] = act.WFWorkflowActionParameters
		} else if exactParamsPresence != nil {
			if hasParams, exists := exactParamsPresence[i]; exists {
				if hasParams {
					actMap["WFWorkflowActionParameters"] = map[string]any{}
				}
			} else if !fillMissingDefaults {
				actMap["WFWorkflowActionParameters"] = map[string]any{}
			}
		} else if !fillMissingDefaults {
			actMap["WFWorkflowActionParameters"] = map[string]any{}
		}
		compiledActions = append(compiledActions, actMap)
	}

	if !fillMissingDefaults {
		merged := deepClonePlistMap(rawHeader)
		if merged == nil {
			merged = make(map[string]any)
		}
		prov := extractAndStripImportProvenance(merged)
		if PreOverlayActionHook != nil {
			PreOverlayActionHook(deepClonePlistSlice(compiledActions))
		}
		applyImportProvenanceToCompiledActions(prov, compiledActions)
		merged["WFWorkflowActions"] = compiledActions
		return merged
	}

	merged := make(map[string]any, len(rawHeader)+12)
	var sBuf bytes.Buffer
	if err := plist.NewEncoder(&sBuf).Encode(fallback); err == nil {
		var sMap map[string]any
		if _, err := plist.Unmarshal(sBuf.Bytes(), &sMap); err == nil {
			for k, v := range sMap {
				merged[k] = v
			}
		}
	}
	for k, v := range rawHeader {
		if k == "WFWorkflowActions" {
			continue
		}
		merged[k] = deepClonePlistValue(v)
	}
	prov := extractAndStripImportProvenance(merged)
	if PreOverlayActionHook != nil {
		PreOverlayActionHook(deepClonePlistSlice(compiledActions))
	}
	applyImportProvenanceToCompiledActions(prov, compiledActions)
	merged["WFWorkflowActions"] = compiledActions
	if inputs, ok := merged["WFWorkflowInputContentItemClasses"].([]any); !ok || len(inputs) == 0 {
		if _, has := merged["WFWorkflowInputContentItemClasses"]; !has {
			merged["WFWorkflowInputContentItemClasses"] = fallback.WFWorkflowInputContentItemClasses
		}
	}
	if types, ok := merged["WFWorkflowTypes"].([]any); !ok || len(types) == 0 {
		if _, has := merged["WFWorkflowTypes"]; !has {
			merged["WFWorkflowTypes"] = fallback.WFWorkflowTypes
		}
	}
	return merged
}

func encodePlistValueToLiteral(v map[string]any) (string, error) {
	if v == nil {
		v = map[string]any{}
	}

	var xmlBuf bytes.Buffer
	xmlEnc := plist.NewEncoderForFormat(&xmlBuf, plist.XMLFormat)
	if err := xmlEnc.Encode(v); err == nil {
		xmlBytes := xmlBuf.Bytes()
		if !hasUnsafePlistStringBytes(xmlBytes) {
			xmlStr := xmlBuf.String()
			quoted := formatCherriDoubleQuotedString(xmlStr)
			unescaped, uErr := unescapeCherriDoubleQuotedString(quoted[1 : len(quoted)-1])
			if uErr == nil && unescaped == xmlStr {
				var round map[string]any
				if _, decErr := plist.Unmarshal([]byte(unescaped), &round); decErr == nil && plistValuesStrictlyEqual(v, round) {
					return xmlStr, nil
				}
			}
		}
	}

	var binBuf bytes.Buffer
	binEnc := plist.NewEncoderForFormat(&binBuf, plist.BinaryFormat)
	if err := binEnc.Encode(v); err != nil {
		return "", fmt.Errorf("binary plist encode failed: %w", err)
	}
	return "b64:" + base64.StdEncoding.EncodeToString(binBuf.Bytes()), nil
}

func decodeRawPlistValueString(raw string) (map[string]any, error) {
	if strings.HasPrefix(raw, "b64:") {
		payload := strings.TrimPrefix(raw, "b64:")
		b, err := base64.StdEncoding.DecodeString(payload)
		if err != nil {
			return nil, fmt.Errorf("invalid base64 plist payload: %w", err)
		}
		return decodeShortcutRawMap(b)
	}
	return decodeShortcutRawMap([]byte(raw))
}

func hasUnsafePlistStringBytes(b []byte) bool {
	if !utf8.Valid(b) {
		return true
	}
	for _, c := range b {
		if c < 0x20 && c != '\n' && c != '\r' && c != '\t' {
			return true
		}
	}
	return false
}

func formatCherriDoubleQuotedString(s string) string {
	var b strings.Builder
	b.Grow(len(s) + 8)
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString(`\\`)
		case '"':
			b.WriteString(`\"`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			b.WriteRune(r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

func unescapeCherriDoubleQuotedString(s string) (string, error) {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			switch s[i] {
			case '"':
				b.WriteByte('"')
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			case '\\':
				b.WriteByte('\\')
			default:
				b.WriteByte(s[i])
			}
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String(), nil
}

func plistValuesStrictlyEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}

	if ia, okA := asSignedOrUnsignedInt(a); okA {
		ib, okB := asSignedOrUnsignedInt(b)
		return okB && ia.signed == ib.signed && ia.sVal == ib.sVal && ia.uVal == ib.uVal
	}

	switch x := a.(type) {
	case bool:
		y, ok := b.(bool)
		return ok && x == y
	case string:
		y, ok := b.(string)
		return ok && x == y
	case float32:
		switch y := b.(type) {
		case float32:
			return float64(x) == float64(y) && math.Signbit(float64(x)) == math.Signbit(float64(y))
		case float64:
			return float64(x) == y && math.Signbit(float64(x)) == math.Signbit(y)
		}
		return false
	case float64:
		switch y := b.(type) {
		case float64:
			return (math.IsNaN(x) && math.IsNaN(y)) || (x == y && math.Signbit(x) == math.Signbit(y))
		case float32:
			return (math.IsNaN(x) && math.IsNaN(float64(y))) || (x == float64(y) && math.Signbit(x) == math.Signbit(float64(y)))
		}
		return false
	case []byte:
		y, ok := b.([]byte)
		return ok && bytes.Equal(x, y)
	case time.Time:
		y, ok := b.(time.Time)
		return ok && x.Equal(y)
	case map[string]any:
		y, ok := b.(map[string]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for k, xv := range x {
			yv, exists := y[k]
			if !exists || !plistValuesStrictlyEqual(xv, yv) {
				return false
			}
		}
		return true
	case []any:
		y, ok := b.([]any)
		if !ok || len(x) != len(y) {
			return false
		}
		for i := range x {
			if !plistValuesStrictlyEqual(x[i], y[i]) {
				return false
			}
		}
		return true
	case []string:
		y, ok := b.([]any)
		if ok {
			if len(x) != len(y) {
				return false
			}
			for i := range x {
				if ys, ok := y[i].(string); !ok || x[i] != ys {
					return false
				}
			}
			return true
		}
		ys, ok := b.([]string)
		if !ok || len(x) != len(ys) {
			return false
		}
		for i := range x {
			if x[i] != ys[i] {
				return false
			}
		}
		return true
	default:
		return false
	}
}

type plistIntVal struct {
	signed bool
	sVal   int64
	uVal   uint64
}

func asSignedOrUnsignedInt(v any) (plistIntVal, bool) {
	switch n := v.(type) {
	case int:
		if n < 0 {
			return plistIntVal{signed: true, sVal: int64(n)}, true
		}
		return plistIntVal{uVal: uint64(n)}, true
	case int8:
		if n < 0 {
			return plistIntVal{signed: true, sVal: int64(n)}, true
		}
		return plistIntVal{uVal: uint64(n)}, true
	case int16:
		if n < 0 {
			return plistIntVal{signed: true, sVal: int64(n)}, true
		}
		return plistIntVal{uVal: uint64(n)}, true
	case int32:
		if n < 0 {
			return plistIntVal{signed: true, sVal: int64(n)}, true
		}
		return plistIntVal{uVal: uint64(n)}, true
	case int64:
		if n < 0 {
			return plistIntVal{signed: true, sVal: n}, true
		}
		return plistIntVal{uVal: uint64(n)}, true
	case uint:
		return plistIntVal{uVal: uint64(n)}, true
	case uint8:
		return plistIntVal{uVal: uint64(n)}, true
	case uint16:
		return plistIntVal{uVal: uint64(n)}, true
	case uint32:
		return plistIntVal{uVal: uint64(n)}, true
	case uint64:
		return plistIntVal{uVal: n}, true
	default:
		return plistIntVal{}, false
	}
}

func RenderLosslessLegacySourceFromDocument(doc *NativeShortcutDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("nil NativeShortcutDocument")
	}
	headerLit, err := encodePlistValueToLiteral(doc.Header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("#workflowheader ")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteByte('\n')

	for _, rec := range doc.Actions {
		rawLine, err := formatRawLegacyActionCall(rec)
		if err != nil {
			return "", err
		}
		out.WriteString(rawLine)
		out.WriteByte('\n')
	}

	return out.String(), nil
}

func RenderLosslessV2SourceFromDocument(doc *NativeShortcutDocument) (string, error) {
	if doc == nil {
		return "", fmt.Errorf("nil NativeShortcutDocument")
	}
	headerLit, err := encodePlistValueToLiteral(doc.Header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("native.workflow(native.plist(")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteString("), exact: true)\n")

	for _, rec := range doc.Actions {
		rawLine, err := formatRawV2ActionCall(rec)
		if err != nil {
			return "", err
		}
		out.WriteString(rawLine)
		out.WriteByte('\n')
	}

	return out.String(), nil
}

func formatRawLegacyActionCall(rec NativeActionRecord) (string, error) {
	var out strings.Builder
	out.WriteString("rawAction(")
	out.WriteString(formatCherriDoubleQuotedString(rec.Identifier))
	if rec.HadParametersKey {
		params := rec.Parameters
		if params == nil {
			params = map[string]any{}
		}
		pLit, err := encodePlistValueToLiteral(params)
		if err != nil {
			return "", fmt.Errorf("encode action %d (%s) params: %w", rec.Index, rec.Identifier, err)
		}
		out.WriteString(", rawPlist(")
		out.WriteString(formatCherriDoubleQuotedString(pLit))
		out.WriteByte(')')
	}
	out.WriteByte(')')
	return out.String(), nil
}

func formatRawV2ActionCall(rec NativeActionRecord) (string, error) {
	var out strings.Builder
	out.WriteString("native.action(")
	out.WriteString(formatCherriDoubleQuotedString(rec.Identifier))
	if rec.HadParametersKey {
		params := rec.Parameters
		if params == nil {
			params = map[string]any{}
		}
		pLit, err := encodePlistValueToLiteral(params)
		if err != nil {
			return "", fmt.Errorf("encode action %d (%s) params: %w", rec.Index, rec.Identifier, err)
		}
		out.WriteString(", parameters: native.plist(")
		out.WriteString(formatCherriDoubleQuotedString(pLit))
		out.WriteByte(')')
	}
	out.WriteByte(')')
	return out.String(), nil
}

func renderLosslessLegacySource(doc map[string]any) (string, error) {
	header, actions, _ := splitWorkflowHeaderAndActions(doc)
	headerLit, err := encodePlistValueToLiteral(header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("#workflowheader ")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteByte('\n')

	for idx, act := range actions {
		ident, _ := act["WFWorkflowActionIdentifier"].(string)
		out.WriteString("rawAction(")
		out.WriteString(formatCherriDoubleQuotedString(ident))

		if rawParams, hasParams := act["WFWorkflowActionParameters"]; hasParams {
			params, ok := rawParams.(map[string]any)
			if !ok && rawParams == nil {
				params = map[string]any{}
				ok = true
			}
			if !ok {
				return "", fmt.Errorf("action %d (%s) WFWorkflowActionParameters is not a dictionary", idx, ident)
			}
			pLit, err := encodePlistValueToLiteral(params)
			if err != nil {
				return "", fmt.Errorf("encode action %d (%s) params: %w", idx, ident, err)
			}
			out.WriteString(", rawPlist(")
			out.WriteString(formatCherriDoubleQuotedString(pLit))
			out.WriteByte(')')
		}
		out.WriteString(")\n")
	}

	return out.String(), nil
}

func renderLosslessV2Source(doc map[string]any) (string, error) {
	header, actions, _ := splitWorkflowHeaderAndActions(doc)
	headerLit, err := encodePlistValueToLiteral(header)
	if err != nil {
		return "", fmt.Errorf("encode workflow header: %w", err)
	}

	var out strings.Builder
	out.WriteString("native.workflow(native.plist(")
	out.WriteString(formatCherriDoubleQuotedString(headerLit))
	out.WriteString("), exact: true)\n")

	for idx, act := range actions {
		ident, _ := act["WFWorkflowActionIdentifier"].(string)
		out.WriteString("native.action(")
		out.WriteString(formatCherriDoubleQuotedString(ident))

		if rawParams, hasParams := act["WFWorkflowActionParameters"]; hasParams {
			params, ok := rawParams.(map[string]any)
			if !ok && rawParams == nil {
				params = map[string]any{}
				ok = true
			}
			if !ok {
				return "", fmt.Errorf("action %d (%s) WFWorkflowActionParameters is not a dictionary", idx, ident)
			}
			pLit, err := encodePlistValueToLiteral(params)
			if err != nil {
				return "", fmt.Errorf("encode action %d (%s) params: %w", idx, ident, err)
			}
			out.WriteString(", parameters: native.plist(")
			out.WriteString(formatCherriDoubleQuotedString(pLit))
			out.WriteByte(')')
		}
		out.WriteString(")\n")
	}

	return out.String(), nil
}

func collectWorkflowHeaderDirective() {
	skipWhitespace()
	if char != '"' {
		parserError(fmt.Sprintf("Expected double-quoted string (\") for #workflowheader, got: %c", char))
	}
	advance()
	rawStr := collectString()
	hdr, err := decodeRawPlistValueString(rawStr)
	if err != nil {
		parserError(fmt.Sprintf("Invalid #workflowheader payload: %s", err))
	}
	importedWorkflowHeader = hdr
	applyLegacyHeaderImportProvenance(hdr)
}

func collectDeclareDirective() {
	skipWhitespace()
	ident := collectIdentifier()
	if ident == "" {
		parserError("Expected identifier after #declare")
	}
	if _, found := variables[ident]; !found {
		variables[ident] = varValue{
			variableType: "Variable",
			value:        ident,
		}
	}
}

func collectRawPlistValue(value *any) {
	skipWhitespace()
	if char != '"' {
		parserError(fmt.Sprintf("Expected double-quoted string (\") inside rawPlist(...), got: %c", char))
	}
	advance()
	rawStr := collectString()
	skipWhitespace()
	if char != ')' {
		parserError(fmt.Sprintf("Expected ')' after rawPlist string literal, got: %c", char))
	}
	advance()
	decoded, err := decodeRawPlistValueString(rawStr)
	if err != nil {
		parserError(fmt.Sprintf("Invalid rawPlist payload: %s", err))
	}
	*value = rawPlistDict{Data: decoded}
}
