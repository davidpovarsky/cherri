package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	"howett.net/plist"
)

// importProvenanceKey is the reserved workflow-header key under which import tooling stores
// action metadata (UUID, CustomOutputName, GroupingIdentifier, Show-* flags) and symbol UI bindings
// that a natural Cherri statement does not express. It is consumed and stripped by
// mergeWorkflowHeaderMap and must never appear in a compiled Shortcut plist.
const importProvenanceKey = "__CherriImportProvenance"

type importedOutputNameSpec struct {
	name    string
	hasName bool
}

var (
	importedSymbolOutputNames   map[string]importedOutputNameSpec
	importedSymbolVariableNames map[string]string
	importedSymbolValueTypes    map[string]tokenType
)

// verifiedImportMetadataKeys defines the strict allowlist of action parameter keys that
// may be treated as non-semantic import metadata.
// An unknown key (including any unknown key starting with "Show") is considered
// semantic/opaque and MUST NEVER be automatically treated as import metadata.
var verifiedImportMetadataKeys = map[string]bool{
	"UUID":                true,
	"CustomOutputName":    true,
	"GroupingIdentifier":  true,
	"ShowWhenRun":         true,
	"Show-WFInput":        true,
	"Show-WFURLActionURL": true,
	"Show-text":           true,
}

// isImportProvenanceMetaKey reports whether an action parameter key is import metadata that may be
// restored from provenance. Semantic parameter values are never restored from provenance.
func isImportProvenanceMetaKey(k string) bool {
	return verifiedImportMetadataKeys[k]
}

// deterministicSemanticActionHash computes a canonical digest of the semantic parameters of an action,
// excluding non-semantic import metadata keys and Legacy variable-action serialization wrappers.
func deterministicSemanticActionHash(identifier string, params map[string]any) string {
	var buf bytes.Buffer
	buf.WriteString(identifier)
	buf.WriteByte(':')
	keys := make([]string, 0, len(params))
	for k := range params {
		if isImportProvenanceMetaKey(k) {
			continue
		}
		if (identifier == "is.workflow.actions.setvariable" || identifier == "is.workflow.actions.appendvariable") && k == "WFSerializationType" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for i, k := range keys {
		if i > 0 {
			buf.WriteByte(';')
		}
		buf.WriteString(k)
		buf.WriteByte('=')
		writeCanonicalPlistValue(&buf, params[k])
	}
	sum := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(sum[:16])
}

func canonicalizePlistValue(v any) any {
	if v == nil {
		return nil
	}
	switch x := v.(type) {
	case string, bool, float64:
		return v
	case int:
		return int64(x)
	case uint64:
		return int64(x)
	case int64:
		return x
	}
	b, err := plist.Marshal(v, plist.BinaryFormat)
	if err != nil {
		return v
	}
	var out any
	if _, err := plist.Unmarshal(b, &out); err != nil {
		return v
	}
	return out
}

func writeCanonicalPlistValue(buf *bytes.Buffer, v any) {
	v = canonicalizePlistValue(v)
	if v == nil {
		buf.WriteString("<nil>")
		return
	}
	switch val := v.(type) {
	case string:
		buf.WriteString("s:")
		buf.WriteString(val)
	case bool:
		if val {
			buf.WriteString("b:1")
		} else {
			buf.WriteString("b:0")
		}
	case int:
		fmt.Fprintf(buf, "i:%d", val)
	case int64:
		fmt.Fprintf(buf, "i:%d", val)
	case uint64:
		fmt.Fprintf(buf, "i:%d", val)
	case float64:
		fmt.Fprintf(buf, "f:%v", val)
	case []byte:
		fmt.Fprintf(buf, "d:%x", val)
	case []any:
		buf.WriteString("a:[")
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			writeCanonicalPlistValue(buf, item)
		}
		buf.WriteByte(']')
	case []string:
		buf.WriteString("as:[")
		for i, item := range val {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(item)
		}
		buf.WriteByte(']')
	case map[string]any:
		keys := make([]string, 0, len(val))
		for k := range val {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		buf.WriteString("m:{")
		for i, k := range keys {
			if i > 0 {
				buf.WriteByte(',')
			}
			buf.WriteString(k)
			buf.WriteByte('=')
			writeCanonicalPlistValue(buf, val[k])
		}
		buf.WriteByte('}')
	default:
		fmt.Fprintf(buf, "v:%v", val)
	}
}

// buildImportProvenance builds a sparse provenance record for the supplied actions.
func buildImportProvenance(recs []NativeActionRecord) map[string]any {
	entries := make([]any, 0)
	for i, rec := range recs {
		meta := map[string]any{}
		for k, v := range rec.Parameters {
			if isImportProvenanceMetaKey(k) {
				meta[k] = deepClonePlistValue(v)
			}
		}
		_, hasUUID := rec.Parameters["UUID"]
		_, hasCustomOutput := rec.Parameters["CustomOutputName"]
		absentCustom := hasUUID && !hasCustomOutput
		absentUUID := !hasUUID
		_, hasTopSer := rec.Parameters["WFSerializationType"]
		absentTopSer := !hasTopSer && (rec.Identifier == "is.workflow.actions.setvariable" || rec.Identifier == "is.workflow.actions.appendvariable")
		if len(meta) == 0 && !absentCustom && !absentUUID && !absentTopSer {
			continue
		}
		entry := map[string]any{
			"Index":        int64(i),
			"Identifier":   rec.Identifier,
			"SemanticHash": deterministicSemanticActionHash(rec.Identifier, rec.Parameters),
		}
		if len(meta) > 0 {
			entry["Meta"] = meta
		}
		if absentCustom {
			entry["AbsentCustomOutputName"] = true
		}
		if absentUUID {
			entry["AbsentUUID"] = true
		}
		if absentTopSer {
			entry["AbsentSerializationType"] = true
		}
		entries = append(entries, entry)
	}
	return map[string]any{
		"Count":   int64(len(recs)),
		"Actions": entries,
	}
}

// headerWithImportProvenance returns a deep copy of header carrying provenance for recs.
func headerWithImportProvenance(header map[string]any, recs []NativeActionRecord) map[string]any {
	return headerWithImportProvenanceWithSymbols(header, recs, nil)
}

// headerWithImportProvenanceWithSymbols returns a deep copy of header carrying action and symbol provenance.
func headerWithImportProvenanceWithSymbols(header map[string]any, recs []NativeActionRecord, symbols map[string]map[string]any) map[string]any {
	out := deepClonePlistMap(header)
	if out == nil {
		out = map[string]any{}
	}
	prov := buildImportProvenance(recs)
	if len(symbols) > 0 {
		symAny := make(map[string]any, len(symbols))
		for k, v := range symbols {
			symAny[k] = deepClonePlistMap(v)
		}
		prov["Symbols"] = symAny
	}
	entries, _ := prov["Actions"].([]any)
	if len(entries) > 0 || len(symbols) > 0 {
		out[importProvenanceKey] = prov
	}
	return out
}

func isUUIDReferencedInCompiledActions(compiled []any, skipIdx int, targetUUID string) bool {
	if targetUUID == "" {
		return false
	}
	var scan func(v any) bool
	scan = func(v any) bool {
		switch x := v.(type) {
		case string:
			return x == targetUUID
		case map[string]any:
			for _, mv := range x {
				if scan(mv) {
					return true
				}
			}
		case []any:
			for _, item := range x {
				if scan(item) {
					return true
				}
			}
		}
		return false
	}
	for i, rawAct := range compiled {
		if i == skipIdx {
			continue
		}
		act, _ := rawAct.(map[string]any)
		if act == nil {
			continue
		}
		if scan(act["WFWorkflowActionParameters"]) {
			return true
		}
	}
	return false
}

type parsedProvEntry struct {
	index        int
	identifier   string
	semanticHash string
	meta         map[string]any
	absentCustom bool
	absentUUID   bool
	absentTopSer bool
}

// applyImportProvenanceToCompiledActions restores metadata keys onto compiled actions. It is a
// no-op unless the action count and each entry's identifier match, so structural edits that change
// the action list never receive stale metadata.
// It uses deterministic semantic hashing to bind metadata to the exact corresponding action,
// preventing stale metadata application on action reordering or replacement.
func applyImportProvenanceToCompiledActions(prov map[string]any, compiled []any) {
	if prov == nil {
		return
	}
	count, ok := provInt(prov["Count"])
	if !ok || count != len(compiled) {
		return
	}
	rawEntries, _ := prov["Actions"].([]any)
	if len(rawEntries) == 0 {
		return
	}

	entries := make([]*parsedProvEntry, 0, len(rawEntries))
	for _, raw := range rawEntries {
		entryMap, _ := raw.(map[string]any)
		if entryMap == nil {
			continue
		}
		idxv, ok := provInt(entryMap["Index"])
		if !ok || idxv < 0 || idxv >= len(compiled) {
			continue
		}
		ident, _ := entryMap["Identifier"].(string)
		semHash, _ := entryMap["SemanticHash"].(string)
		meta, _ := entryMap["Meta"].(map[string]any)
		absentCustom, _ := entryMap["AbsentCustomOutputName"].(bool)
		absentUUID, _ := entryMap["AbsentUUID"].(bool)
		absentTopSer, _ := entryMap["AbsentSerializationType"].(bool)
		if len(meta) == 0 && !absentCustom && !absentUUID && !absentTopSer {
			continue
		}
		entries = append(entries, &parsedProvEntry{
			index:        idxv,
			identifier:   ident,
			semanticHash: semHash,
			meta:         meta,
			absentCustom: absentCustom,
			absentUUID:   absentUUID,
			absentTopSer: absentTopSer,
		})
	}

	// Compute compiled hashes for all compiled actions
	compiledHashes := make([]string, len(compiled))
	compiledIdents := make([]string, len(compiled))
	for j, rawAct := range compiled {
		act, _ := rawAct.(map[string]any)
		if act == nil {
			continue
		}
		compiledIdents[j], _ = act["WFWorkflowActionIdentifier"].(string)
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		compiledHashes[j] = deterministicSemanticActionHash(compiledIdents[j], params)
	}

	// Track matching: compiled index -> entry
	matchedCompiled := make(map[int]*parsedProvEntry, len(entries))
	usedEntries := make(map[int]bool, len(entries))

	// Pass 1: Exact in-place match (same index, same identifier, same semantic hash)
	for eIdx, e := range entries {
		j := e.index
		if j < len(compiled) && compiledIdents[j] == e.identifier {
			if e.semanticHash != "" && compiledHashes[j] == e.semanticHash {
				matchedCompiled[j] = e
				usedEntries[eIdx] = true
			}
		}
	}

	// Pass 2: Reordered/moved actions (unmatched entry whose semantic hash uniquely matches an unmatched compiled action)
	for eIdx, e := range entries {
		if usedEntries[eIdx] || e.semanticHash == "" {
			continue
		}
		matchingCompiledIdx := -1
		matchCount := 0
		for j := 0; j < len(compiled); j++ {
			if matchedCompiled[j] == nil && compiledIdents[j] == e.identifier && compiledHashes[j] == e.semanticHash {
				matchingCompiledIdx = j
				matchCount++
			}
		}
		if matchCount == 1 {
			matchedCompiled[matchingCompiledIdx] = e
			usedEntries[eIdx] = true
		}
	}

	// Pass 3: Backwards compatibility for legacy provenance records without SemanticHash.
	// When SemanticHash is present, matching strictly requires hash equality (preventing stale
	// metadata inheritance on action replacement, tampering, or duplication).
	for eIdx, e := range entries {
		if usedEntries[eIdx] || e.semanticHash != "" {
			continue
		}
		j := e.index
		if j < len(compiled) && matchedCompiled[j] == nil && compiledIdents[j] == e.identifier {
			matchedCompiled[j] = e
			usedEntries[eIdx] = true
		}
	}

	// Apply matched metadata
	for j, e := range matchedCompiled {
		act, _ := compiled[j].(map[string]any)
		if act == nil {
			continue
		}
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if len(e.meta) > 0 {
			if params == nil {
				params = map[string]any{}
			}
			for k, v := range e.meta {
				if isImportProvenanceMetaKey(k) {
					params[k] = deepClonePlistValue(v)
				}
			}
		}
		if e.absentCustom && params != nil {
			delete(params, "CustomOutputName")
		}
		if e.absentUUID && params != nil {
			if u, hasU := params["UUID"].(string); hasU && u != "" && !isUUIDReferencedInCompiledActions(compiled, j, u) {
				delete(params, "UUID")
			}
		}
		if e.absentTopSer && params != nil {
			delete(params, "WFSerializationType")
		}
		if params != nil {
			act["WFWorkflowActionParameters"] = params
		}
	}
}

// extractAndStripImportProvenance removes provenance from header (in place) and returns it.
func extractAndStripImportProvenance(header map[string]any) map[string]any {
	if header == nil {
		return nil
	}
	prov, _ := header[importProvenanceKey].(map[string]any)
	delete(header, importProvenanceKey)
	return prov
}

// applyLegacyHeaderImportProvenance seeds Legacy compiler symbol/UUID state from #workflowheader provenance.
func applyLegacyHeaderImportProvenance(hdr map[string]any) {
	if hdr == nil {
		return
	}
	prov, ok := hdr[importProvenanceKey].(map[string]any)
	if !ok || prov == nil {
		return
	}
	syms, ok := prov["Symbols"].(map[string]any)
	if !ok || len(syms) == 0 {
		return
	}
	if uuids == nil {
		uuids = map[string]string{}
	}
	if variables == nil {
		variables = map[string]varValue{}
	}
	if importedSymbolOutputNames == nil {
		importedSymbolOutputNames = map[string]importedOutputNameSpec{}
	}
	if importedSymbolVariableNames == nil {
		importedSymbolVariableNames = map[string]string{}
	}
	if importedSymbolValueTypes == nil {
		importedSymbolValueTypes = map[string]tokenType{}
	}
	for symName, rawEntry := range syms {
		entry, ok := rawEntry.(map[string]any)
		if !ok || symName == "" {
			continue
		}
		kind, _ := entry["Kind"].(string)
		uuidStr, _ := entry["UUID"].(string)
		outName, _ := entry["OutputName"].(string)
		hasOutName := true
		if b, exists := entry["HasOutputName"].(bool); exists {
			hasOutName = b
		}
		varName, _ := entry["VariableName"].(string)
		producerRaw, _ := entry["ProducerRaw"].(bool)
		vtStr, _ := entry["ValueType"].(string)
		vtTok := tokenTypeFromProvenanceValueType(vtStr)
		if vtTok != "" {
			importedSymbolValueTypes[symName] = vtTok
		}

		if uuidStr != "" {
			uuids[symName] = uuidStr
		}
		if kind == "Variable" {
			if varName != "" && varName != symName {
				importedSymbolVariableNames[symName] = varName
			}
			if producerRaw {
				if _, found := variables[symName]; !found {
					variables[symName] = varValue{
						variableType: "Variable",
						valueType:    vtTok,
						value:        symName,
						constant:     false,
					}
				}
			}
		} else {
			importedSymbolOutputNames[symName] = importedOutputNameSpec{
				name:    outName,
				hasName: hasOutName,
			}
			if producerRaw {
				if _, found := variables[symName]; !found {
					variables[symName] = varValue{
						variableType: "Variable",
						valueType:    vtTok,
						value:        symName,
						constant:     true,
					}
				}
			}
		}
	}
}

func tokenTypeFromProvenanceValueType(vt string) tokenType {
	switch vt {
	case "Dictionary", "Dict":
		return Dict
	case "Number", "Integer":
		return Integer
	case "Float":
		return Float
	case "List", "Arr":
		return Arr
	case "Bool", "Boolean":
		return Bool
	case "Date":
		return Date
	case "Text", "String":
		return String
	default:
		return ""
	}
}

func resolveLegacyOutputName(identifier string) string {
	if spec, ok := importedSymbolOutputNames[identifier]; ok {
		if !spec.hasName {
			return ""
		}
		if spec.name != "" {
			return spec.name
		}
	}
	return identifier
}

func resolveLegacyVariableName(identifier string) string {
	if orig, ok := importedSymbolVariableNames[identifier]; ok && orig != "" {
		return orig
	}
	return identifier
}

func provInt(v any) (int, bool) {
	iv, ok := asSignedOrUnsignedInt(v)
	if !ok {
		return 0, false
	}
	if iv.signed {
		return int(iv.sVal), true
	}
	return int(iv.uVal), true
}

// hasImportProvenanceKey reports whether importProvenanceKey appears anywhere inside a decoded plist tree.
func hasImportProvenanceKey(v any) bool {
	switch tv := v.(type) {
	case map[string]any:
		if _, exists := tv[importProvenanceKey]; exists {
			return true
		}
		for _, sub := range tv {
			if hasImportProvenanceKey(sub) {
				return true
			}
		}
	case []any:
		for _, item := range tv {
			if hasImportProvenanceKey(item) {
				return true
			}
		}
	}
	return false
}
