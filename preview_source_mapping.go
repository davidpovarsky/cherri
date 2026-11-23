package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/protocol"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
	"github.com/electrikmilk/cherri/internal/language/source"
)

// previewSourceMap is an immutable companion to one mobile compile result.
// It never enters the Apple plist or the WebView edit message. The integrity
// digest detects accidental mutation; it is not authentication of a C caller.
type previewSourceMap struct {
	Version         int                  `json:"version"`
	Name            string               `json:"name"`
	Mode            string               `json:"mode"`
	BuildProfile    string               `json:"buildProfile"`
	SourceSHA256    string               `json:"sourceSHA256"`
	PlistSHA256     string               `json:"plistSHA256"`
	ActionCount     int                  `json:"actionCount"`
	Entries         []previewSourceEntry `json:"entries"`
	IntegritySHA256 string               `json:"integritySHA256"`
}

type previewSourceEntry struct {
	ActionIndex      int                      `json:"actionIndex"`
	ActionIdentifier string                   `json:"actionIdentifier"`
	CallStart        int                      `json:"callStart"`
	CallEnd          int                      `json:"callEnd"`
	Parameters       []previewSourceParameter `json:"parameters"`
}

type previewSourceParameter struct {
	WireKey       string `json:"wireKey"`
	ArgumentStart int    `json:"argumentStart"`
	ArgumentEnd   int    `json:"argumentEnd"`
}

type previewSourceCandidate struct {
	parameters map[string]any
	identifier string
	callSpan   source.Span
	arguments  map[string]source.Span
}

// These writes come from makeActionParamsWithOrigins, after its skip guards.
// Custom makeParams builders never report an ordinary argument-to-wire write.
type previewParameterWrite struct {
	position int
	key      string
}

// rememberPreviewSourceCall runs only at the ordinary canonical action append.
// Generated/raw/macro actions never receive this origin. Positions come from
// the actual canonical binder, and wire keys from its actual parameter writer.
// No native payload, UUID or action-count heuristic is used to infer a join.
func (s *CanonicalBackendSession) rememberPreviewSourceCall(call backend.ResolvedCall, parameters map[string]any, identifier string, canonicalPositions []int, writes []previewParameterWrite) {
	origin := call.VisualSourceOrigin
	if origin == nil || parameters == nil || len(origin.ArgumentSpans) != len(call.Arguments) || len(canonicalPositions) != len(call.Arguments) {
		return
	}
	written := make(map[int]string)
	writeCounts := make(map[string]int)
	for _, write := range writes {
		written[write.position] = write.key
		writeCounts[write.key]++
	}
	canonicalWireKeys := make([]string, len(call.Arguments))
	for index, position := range canonicalPositions {
		if key, exists := written[position]; position >= 0 && exists && writeCounts[key] == 1 {
			canonicalWireKeys[index] = key
		}
	}
	counts := make(map[string]int)
	for _, key := range canonicalWireKeys {
		if key != "" {
			counts[key]++
		}
	}
	arguments := make(map[string]source.Span)
	for index, key := range canonicalWireKeys {
		span := origin.ArgumentSpans[index]
		if key == "" || counts[key] != 1 || !previewEditableWireKey(key) ||
			span.Document != origin.CallSpan.Document || span.Start.ByteOffset < origin.CallSpan.Start.ByteOffset ||
			span.End.ByteOffset > origin.CallSpan.End.ByteOffset || span.Start.ByteOffset >= span.End.ByteOffset {
			continue
		}
		value, exists := parameters[key]
		if exists && previewEditableScalar(value) {
			arguments[key] = span
		}
	}
	if len(arguments) != 0 {
		s.previewSourceCandidates = append(s.previewSourceCandidates, previewSourceCandidate{
			parameters: parameters, identifier: identifier, callSpan: origin.CallSpan, arguments: arguments,
		})
	}
}

// finalizePreviewSourceMap follows the same compiler-private parameter-map
// identity boundary as setup_question_binding.go. Later instrumentation may
// insert earlier actions; Void calls need no added native UUID for this join.
func (s *CanonicalBackendSession) finalizePreviewSourceMap(filePath, content string, profile BuildProfile, plistBytes []byte) previewSourceMap {
	result := previewSourceMap{
		Version: 1, Name: strings.TrimSuffix(filePath, ".cherri"), Mode: "v2", BuildProfile: string(profile),
		SourceSHA256: previewBytesSHA([]byte(content)), PlistSHA256: previewBytesSHA(plistBytes),
		ActionCount: len(s.shortcut.WFWorkflowActions), Entries: []previewSourceEntry{},
	}
	indices := make(map[uintptr]int)
	for index, action := range s.shortcut.WFWorkflowActions {
		if action.WFWorkflowActionParameters == nil {
			continue
		}
		identity := reflect.ValueOf(action.WFWorkflowActionParameters).Pointer()
		if _, exists := indices[identity]; exists {
			indices[identity] = -1
		} else {
			indices[identity] = index
		}
	}
	if !s.hasRawWorkflow {
		for _, candidate := range s.previewSourceCandidates {
			index, exists := indices[reflect.ValueOf(candidate.parameters).Pointer()]
			span := candidate.callSpan
			if !exists || index < 0 || span.Document != source.DocumentID(filePath) ||
				span.Start.ByteOffset < 0 || span.End.ByteOffset > len(content) || span.Start.ByteOffset >= span.End.ByteOffset {
				continue
			}
			action := s.shortcut.WFWorkflowActions[index]
			if action.WFWorkflowActionIdentifier != candidate.identifier {
				continue
			}
			entry := previewSourceEntry{ActionIndex: index, ActionIdentifier: candidate.identifier,
				CallStart: span.Start.ByteOffset, CallEnd: span.End.ByteOffset, Parameters: []previewSourceParameter{}}
			for key, argument := range candidate.arguments {
				if value, exists := action.WFWorkflowActionParameters[key]; exists && previewEditableScalar(value) {
					entry.Parameters = append(entry.Parameters, previewSourceParameter{
						WireKey: key, ArgumentStart: argument.Start.ByteOffset, ArgumentEnd: argument.End.ByteOffset,
					})
				}
			}
			sort.Slice(entry.Parameters, func(i, j int) bool { return entry.Parameters[i].WireKey < entry.Parameters[j].WireKey })
			if len(entry.Parameters) != 0 {
				result.Entries = append(result.Entries, entry)
			}
		}
	}
	sort.Slice(result.Entries, func(i, j int) bool { return result.Entries[i].ActionIndex < result.Entries[j].ActionIndex })
	// Multiple candidates for one physical action are ambiguous, hence read-only.
	unique := result.Entries[:0]
	for start := 0; start < len(result.Entries); {
		end := start + 1
		for end < len(result.Entries) && result.Entries[end].ActionIndex == result.Entries[start].ActionIndex {
			end++
		}
		if end == start+1 {
			unique = append(unique, result.Entries[start])
		}
		start = end
	}
	result.Entries = unique
	result.IntegritySHA256 = previewMapIntegrity(result)
	return result
}

func previewBytesSHA(data []byte) string { return fmt.Sprintf("%x", sha256.Sum256(data)) }

func previewMapIntegrity(mapping previewSourceMap) string {
	mapping.IntegritySHA256 = ""
	data, _ := json.Marshal(mapping) // Only strings, integers and slices of those fields.
	return previewBytesSHA(data)
}

func previewEditableWireKey(key string) bool {
	switch key {
	case "", "UUID", "CustomOutputName", "WFControlFlowMode", "GroupingIdentifier":
		return false
	}
	return true
}

func previewEditableScalar(value any) bool {
	if text, ok := value.(string); ok {
		return text != "" && !strings.ContainsAny(text, "<>")
	}
	if value == nil {
		return false
	}
	reflected := reflect.ValueOf(value)
	switch reflected.Kind() {
	case reflect.Bool:
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Float32, reflect.Float64:
		return !math.IsNaN(reflected.Float()) && !math.IsInf(reflected.Float(), 0)
	}
	return false
}

type previewSourceEditRequest struct {
	Source           string            `json:"source"`
	Name             string            `json:"name"`
	Mode             string            `json:"mode"`
	BuildProfile     string            `json:"buildProfile"`
	PlistBase64      string            `json:"plistBase64"`
	PreviewSourceMap *previewSourceMap `json:"previewSourceMap"`
	ActionIndex      int               `json:"actionIndex"`
	ParameterKey     string            `json:"parameterKey"`
	NewExpression    string            `json:"newExpression"`
}

// editPreviewSourceForMobile is also the exact implementation behind the new
// JSON C ABI. No compilation, signing, network call or global map lookup occurs.
// Callers must obtain the sidecar from CompiledShortcut, never from WebView data.
func editPreviewSourceForMobile(requestJSON string) protocol.SourceEditResponse {
	var request previewSourceEditRequest
	decoder := json.NewDecoder(strings.NewReader(requestJSON))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		return protocol.SourceEditResponse{OK: false, Source: request.Source, Error: "invalid native preview edit request"}
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return protocol.SourceEditResponse{OK: false, Source: request.Source, Error: "native preview edit request has trailing data"}
	}
	updated, err := applyPreviewSourceEdit(request)
	if err != nil {
		return protocol.SourceEditResponse{OK: false, Source: request.Source, Error: err.Error()}
	}
	return protocol.SourceEditResponse{OK: true, Source: updated}
}

func applyPreviewSourceEdit(request previewSourceEditRequest) (string, error) {
	mapping := request.PreviewSourceMap
	if mapping == nil || mapping.Version != 1 || mapping.Mode != "v2" || request.Mode != mapping.Mode ||
		request.Name != mapping.Name || request.BuildProfile != mapping.BuildProfile ||
		(mapping.BuildProfile != string(BuildProfileDebug) && mapping.BuildProfile != string(BuildProfileRelease)) {
		return request.Source, fmt.Errorf("native preview source map is missing or does not match name, mode and profile; source preserved")
	}
	if mapping.SourceSHA256 != previewBytesSHA([]byte(request.Source)) {
		return request.Source, fmt.Errorf("native preview belongs to different source bytes; rebuild the preview before editing")
	}
	if mapping.IntegritySHA256 != previewMapIntegrity(*mapping) {
		return request.Source, fmt.Errorf("native preview source map integrity check failed; source preserved")
	}
	plistBytes, err := base64.StdEncoding.DecodeString(request.PlistBase64)
	if err != nil || mapping.PlistSHA256 != previewBytesSHA(plistBytes) {
		return request.Source, fmt.Errorf("native preview plist does not match its source map; source preserved")
	}
	workflow, err := decodeShortcutRawMap(plistBytes)
	if err != nil {
		return request.Source, fmt.Errorf("native preview plist is invalid; source preserved")
	}
	actions, ok := workflow["WFWorkflowActions"].([]any)
	if !ok || len(actions) != mapping.ActionCount || request.ActionIndex < 0 || request.ActionIndex >= len(actions) {
		return request.Source, fmt.Errorf("native preview action index or count is invalid; source preserved")
	}
	var selected *previewSourceEntry
	previousIndex := -1
	for index := range mapping.Entries {
		entry := &mapping.Entries[index]
		if entry.ActionIndex <= previousIndex || entry.ActionIndex < 0 || entry.ActionIndex >= len(actions) ||
			entry.CallStart < 0 || entry.CallStart >= entry.CallEnd || entry.CallEnd > len(request.Source) || len(entry.Parameters) == 0 {
			return request.Source, fmt.Errorf("native preview source map contains an invalid or ambiguous action origin")
		}
		previousIndex = entry.ActionIndex
		action, ok := actions[entry.ActionIndex].(map[string]any)
		if !ok || action["WFWorkflowActionIdentifier"] != entry.ActionIdentifier {
			return request.Source, fmt.Errorf("native preview action identity does not match its source map")
		}
		parameters, ok := action["WFWorkflowActionParameters"].(map[string]any)
		if !ok {
			return request.Source, fmt.Errorf("native preview mapped action has no native parameters")
		}
		previousKey := ""
		for _, parameter := range entry.Parameters {
			if parameter.WireKey <= previousKey || !previewEditableWireKey(parameter.WireKey) ||
				parameter.ArgumentStart < entry.CallStart || parameter.ArgumentStart >= parameter.ArgumentEnd || parameter.ArgumentEnd > entry.CallEnd {
				return request.Source, fmt.Errorf("native preview source map contains an invalid or ambiguous argument origin")
			}
			previousKey = parameter.WireKey
			value, exists := parameters[parameter.WireKey]
			if !exists || !previewEditableScalar(value) {
				return request.Source, fmt.Errorf("native preview mapped parameter is missing or read-only")
			}
		}
		if entry.ActionIndex == request.ActionIndex {
			selected = entry
		}
	}
	if selected == nil {
		return request.Source, fmt.Errorf("this native action is generated or has no unique authored source origin; it is read-only")
	}
	for _, parameter := range selected.Parameters {
		if parameter.WireKey == request.ParameterKey {
			svc := service.NewService(schema.DefaultRegistry())
			return svc.ApplyVisualSourceArgument(request.Source, selected.CallStart, selected.CallEnd, parameter.ArgumentStart, parameter.ArgumentEnd, request.NewExpression)
		}
	}
	return request.Source, fmt.Errorf("this native wire parameter has no unique existing authored argument; it is read-only")
}
