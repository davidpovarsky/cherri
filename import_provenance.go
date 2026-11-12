package main

import (
	"strings"
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

// isImportProvenanceMetaKey reports whether an action parameter key is import metadata that may be
// restored from provenance. Semantic parameter values are never restored from provenance.
func isImportProvenanceMetaKey(k string) bool {
	switch k {
	case "UUID", "CustomOutputName", "GroupingIdentifier":
		return true
	}
	return strings.HasPrefix(k, "Show")
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
			"Index":      int64(i),
			"Identifier": rec.Identifier,
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

// applyImportProvenanceToCompiledActions restores metadata keys onto compiled actions. It is a
// no-op unless the action count and each entry's identifier match, so structural edits that change
// the action list never receive stale metadata.
func applyImportProvenanceToCompiledActions(prov map[string]any, compiled []any) {
	if prov == nil {
		return
	}
	count, ok := provInt(prov["Count"])
	if !ok || count != len(compiled) {
		return
	}
	entries, _ := prov["Actions"].([]any)
	for _, raw := range entries {
		entry, _ := raw.(map[string]any)
		if entry == nil {
			continue
		}
		idxv, ok := provInt(entry["Index"])
		if !ok || idxv < 0 || idxv >= len(compiled) {
			continue
		}
		act, _ := compiled[idxv].(map[string]any)
		if act == nil {
			continue
		}
		ident, _ := entry["Identifier"].(string)
		if cur, _ := act["WFWorkflowActionIdentifier"].(string); cur != ident {
			continue
		}
		meta, _ := entry["Meta"].(map[string]any)
		absentCustom, _ := entry["AbsentCustomOutputName"].(bool)
		absentUUID, _ := entry["AbsentUUID"].(bool)
		absentTopSer, _ := entry["AbsentSerializationType"].(bool)
		if len(meta) == 0 && !absentCustom && !absentUUID && !absentTopSer {
			continue
		}
		params, _ := act["WFWorkflowActionParameters"].(map[string]any)
		if len(meta) > 0 {
			if params == nil {
				params = map[string]any{}
			}
			for k, v := range meta {
				if isImportProvenanceMetaKey(k) {
					params[k] = deepClonePlistValue(v)
				}
			}
		}
		if absentCustom && params != nil {
			delete(params, "CustomOutputName")
		}
		if absentUUID && params != nil {
			if u, hasU := params["UUID"].(string); hasU && u != "" && !isUUIDReferencedInCompiledActions(compiled, idxv, u) {
				delete(params, "UUID")
			}
		}
		if absentTopSer && params != nil {
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
