package main

// Cherri Debug Standard Library: compiler-expanded, runtime-local helpers.
//
// Both Legacy and v2 invoke buildDebugStdlibActions. These helpers generate
// native Apple Shortcuts actions directly in the caller's workflow (never Run
// Shortcut/Run Self and never iOS Stored Content). Their state lives only for
// the current Shortcut execution.
import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const debugLogVariableName = "__cherri_debug_log"
const debugTimerVariablePrefix = "__cherri_debug_timer_"
const debugQATraceVariableName = "__cherri_qa_trace"

func debugTimerVariableName(label string) (string, error) {
	if strings.TrimSpace(label) == "" {
		return "", fmt.Errorf("debug timer label must not be empty")
	}
	sum := sha256.Sum256([]byte(label))
	return debugTimerVariablePrefix + hex.EncodeToString(sum[:8]), nil
}

// Compile-time named timers deliberately accept only literal labels so the
// same key is reproducible at start and stop without runtime name lookups.
func debugTimerLabel(params map[string]any) (string, error) {
	s, ok := params["WFDebugTimerLabel"].(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("debug timer requires a non-empty literal label")
	}
	return debugTimerVariableName(s)
}

// debugDateFieldToken matches Apple's native gettimebetweendates parameter
// encoding (both WFTimeUntilFromDate and WFInput): a WFTextTokenString with
// one U+FFFC attachment at UTF-16 range {0, 1}. Unlike generic WFInput
// parameters, date-input fields do not accept WFTextTokenAttachment.
func debugDateFieldToken(v Value) map[string]any {
	// This *must* use literal lowercase plist keys in the nested map.
	// The generic struct encoder can write uppercase "String" and
	// "AttachmentsByRange", which looks plausible in Go but is not
	// the wire format accepted by Apple Shortcuts' date-input fields.
	var token map[string]any
	switch v.Type {
	case "Variable":
		token = map[string]any{"Type": "Variable", "VariableName": v.VariableName}
	case "ActionOutput":
		token = map[string]any{"Type": "ActionOutput", "OutputUUID": v.OutputUUID, "OutputName": v.OutputName}
	default:
		token = map[string]any{"Type": v.Type}
	}
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string":             "\uFFFC",
			"attachmentsByRange": map[string]any{"{0, 1}": token},
		},
	}
}

// A stable, private variable name is deliberately reserved. It must never
// depend on a global persistent Storage key.
func debugLogValueRef(name string) WFTextTokenAttachment {
	return WFTextTokenAttachment{
		WFSerializationType: "WFTextTokenAttachment",
		Value:               Value{Type: "Variable", VariableName: name},
	}
}

func debugOutputRef(id string) WFTextTokenAttachment {
	return debugOutputRefNamed(id, "")
}

// debugCurrentDateRef uses Apple's built-in Current Date magic variable,
// eliminating a redundant native Get Current Date action.
func debugCurrentDateRef() map[string]any {
	return map[string]any{
		"WFSerializationType": "WFTextTokenAttachment",
		"Value":               map[string]any{"Type": "CurrentDate"},
	}
}

// debugDatedLogRow matches the user's corrected on-device Shortcut export:
// the value is Repeat Item, and the date is a PROPERTY of that SAME item.
// No date list, matching-index lookup, or destructive list conversion.
func debugDatedLogRow() map[string]any {
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "\uFFFC - \uFFFC",
			"attachmentsByRange": map[string]any{
				"{0, 1}": map[string]any{
					"Type":         "Variable",
					"VariableName": "Repeat Item",
				},
				"{4, 1}": map[string]any{
					"Type":         "Variable",
					"VariableName": "Repeat Item",
					"Aggrandizements": []any{
						map[string]any{
							"Type":             "WFPropertyVariableAggrandizement",
							"PropertyName":     "Creation Date",
							"PropertyUserInfo": "WFFileCreationDate",
						},
						map[string]any{
							"Type":                 "WFDateFormatVariableAggrandizement",
							"WFDateFormatStyle":    "Medium",
							"WFTimeFormatStyle":    "Long",
							"WFISO8601IncludeTime": false,
						},
					},
				},
			},
		},
	}
}

// debugDatedNamedLogItem reproduces the user's exported native Shortcuts
// Set Name action. The original object's type is not converted into text.
// WFName is the same Repeat Item with Creation Date and Medium/Long format,
// exactly as in the supplied "משתני לוג-2.json" shortcut.
func debugDatedNamedLogItem() map[string]any {
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": "\uFFFC",
			"attachmentsByRange": map[string]any{
				"{0, 1}": map[string]any{
					"Type":         "Variable",
					"VariableName": "Repeat Item",
					"Aggrandizements": []any{
						map[string]any{
							"Type":             "WFPropertyVariableAggrandizement",
							"PropertyName":     "Creation Date",
							"PropertyUserInfo": "WFFileCreationDate",
						},
						map[string]any{
							"Type":                 "WFDateFormatVariableAggrandizement",
							"WFDateFormatStyle":    "Medium",
							"WFTimeFormatStyle":    "Long",
							"WFISO8601IncludeTime": false,
						},
					},
				},
			},
		},
	}
}

func debugOutputRefNamed(id, outputName string) WFTextTokenAttachment {
	return WFTextTokenAttachment{
		WFSerializationType: "WFTextTokenAttachment",
		Value:               Value{Type: "ActionOutput", OutputUUID: id, OutputName: outputName},
	}
}

// debugLogDateSourceRef uses the exact native Apple structure observed in
// the user's iPad-exported shortcut. The date action must execute immediately
// after the original output producer, NOT during later log presentation.
func debugLogDateSourceRef(sourceUUID, sourceName string) WFTextTokenAttachment {
	return WFTextTokenAttachment{
		WFSerializationType: "WFTextTokenAttachment",
		Value: Value{
			Type:       "ActionOutput",
			OutputUUID: sourceUUID,
			OutputName: sourceName,
			Aggrandizements: []Aggrandizement{{
				Type:             "WFPropertyVariableAggrandizement",
				PropertyName:     "Creation Date",
				PropertyUserInfo: "WFFileCreationDate",
			}},
		},
	}
}

// debugLogOutputRef returns the unmodified native output reference that the
// existing Add to Variable action already uses.
func debugLogOutputRef(raw any) (Value, bool) {
	switch ref := raw.(type) {
	case WFTextTokenAttachment:
		if ref.WFSerializationType == "WFTextTokenAttachment" && ref.Value.Type == "ActionOutput" {
			return ref.Value, true
		}
	case *WFTextTokenAttachment:
		if ref != nil && ref.WFSerializationType == "WFTextTokenAttachment" && ref.Value.Type == "ActionOutput" {
			return ref.Value, true
		}
	case map[string]any:
		if ref["WFSerializationType"] == "WFTextTokenAttachment" {
			if v, ok := ref["Value"].(map[string]any); ok && v["Type"] == "ActionOutput" {
				uuid, _ := v["OutputUUID"].(string)
				name, _ := v["OutputName"].(string)
				return Value{Type: "ActionOutput", OutputUUID: uuid, OutputName: name}, uuid != ""
			}
		}
	}
	return Value{}, false
}

// debugLiteralFlag rejects magic-variable/nonliteral flags at compile time.
// In a compiler macro these options must have stable native action counts.
func debugLiteralFlag(params map[string]any, key, name string) (bool, error) {
	value, passed := params[key]
	if !passed || value == nil {
		return false, nil
	}
	b, ok := value.(bool)
	if !ok {
		return false, fmt.Errorf("%s requires literal Boolean %s", name, key)
	}
	return b, nil
}

// Some callers want the timestamps displayed at the end of the session even
// if earlier debugLog calls lacked withDates:true. Revisit the already-emitted
// log actions and inject a native date DETECTOR immediately after each source.
// The pass is strictly opt-in, idempotent, and never builds a second list.
func debugMacroRequestsDates(name string, params map[string]any) bool {
	dates, _ := debugLiteralFlag(params, "WFDebugWithDates", name)
	if !dates {
		return false
	}
	if name == "debugShowLogs" || name == "debugFinish" {
		return true
	}
	if name == "debugLog" || name == "debugLogPrevious" {
		display, _ := debugLiteralFlag(params, "WFDebugShowLogs", name)
		textDisplay, _ := debugLiteralFlag(params, "WFDebugShowLogsAsText", name)
		return display || textDisplay
	}
	return false
}

func backfillDebugLogDates(prior []ShortcutAction) ([]ShortcutAction, int, error) {
	makeAction := func(id string, p map[string]any) ShortcutAction {
		return ShortcutAction{
			WFWorkflowActionIdentifier: "is.workflow.actions." + id,
			WFWorkflowActionParameters: p,
		}
	}
	producerIndex := map[string]int{}
	datedProducer := map[string]bool{}
	for i, a := range prior {
		params := a.WFWorkflowActionParameters
		if uuid, ok := params["UUID"].(string); ok && uuid != "" {
			producerIndex[uuid] = i
		}
		if a.WFWorkflowActionIdentifier != "is.workflow.actions.detect.date" {
			continue
		}
		source, ok := debugLogOutputRef(params["WFInput"])
		if !ok {
			continue
		}
		switch tok := params["WFInput"].(type) {
		case WFTextTokenAttachment:
			if len(tok.Value.Aggrandizements) == 1 && tok.Value.Aggrandizements[0].PropertyUserInfo == "WFFileCreationDate" {
				datedProducer[source.OutputUUID] = true
			}
		}
	}
	after := map[int][]ShortcutAction{}
	for i, a := range prior {
		params := a.WFWorkflowActionParameters
		if a.WFWorkflowActionIdentifier != "is.workflow.actions.appendvariable" || params["WFVariableName"] != debugLogVariableName {
			continue
		}
		source, ok := debugLogOutputRef(params["WFInput"])
		if !ok || source.OutputUUID == "" {
			return nil, 0, fmt.Errorf("dated log requires native ActionOutput at action %d", i)
		}
		producer, exists := producerIndex[source.OutputUUID]
		if !exists || producer >= i {
			return nil, 0, fmt.Errorf("dated log producer missing or forward %q", source.OutputUUID)
		}
		if datedProducer[source.OutputUUID] {
			continue
		}
		after[producer] = append(after[producer], makeAction("detect.date", map[string]any{
			"WFInput": debugLogDateSourceRef(source.OutputUUID, source.OutputName),
			"UUID":    strings.ToUpper(uuid.NewString()),
		}))
		datedProducer[source.OutputUUID] = true
	}
	inserted := 0
	for _, list := range after {
		inserted += len(list)
	}
	if inserted == 0 {
		return prior, 0, nil
	}
	result := make([]ShortcutAction, 0, len(prior)+inserted)
	for i, a := range prior {
		result = append(result, a)
		result = append(result, after[i]...)
	}
	return result, inserted, nil
}

// captureDebugPreviousInput preserves a *real* native action-output reference.
// The Apple Shortcuts editor needs a serialized WFInput token; omitting the
// key does NOT automatically bind Add to Variable to the preceding action.
func captureDebugPreviousInput(actions []ShortcutAction) (any, error) {
	if len(actions) == 0 {
		return nil, fmt.Errorf("debugLog() requires a preceding output-producing action")
	}
	previous := &actions[len(actions)-1]
	identifier := previous.WFWorkflowActionIdentifier
	switch identifier {
	case "is.workflow.actions.exit", "is.workflow.actions.nothing",
		"is.workflow.actions.setvariable", "is.workflow.actions.appendvariable",
		"is.workflow.actions.conditional", "is.workflow.actions.choosefrommenu",
		"is.workflow.actions.previewdocument", "is.workflow.actions.showresult",
		"is.workflow.actions.alert":
		return nil, fmt.Errorf("debugLog() cannot capture output from %s; pass an explicit value", identifier)
	}
	if previous.WFWorkflowActionParameters == nil {
		previous.WFWorkflowActionParameters = make(map[string]any)
	}
	id, _ := previous.WFWorkflowActionParameters["UUID"].(string)
	if id == "" {
		// Only generate an ID when the predecessor has none. Do not rewrite
		// existing UUIDs: the compiler may have already issued references to
		// this producer, and rewriting its case breaks those references.
		id = strings.ToUpper(uuid.NewString())
		previous.WFWorkflowActionParameters["UUID"] = id
	}
	name, _ := previous.WFWorkflowActionParameters["CustomOutputName"].(string)
	if name == "" {
		switch identifier {
		case "is.workflow.actions.gettext":
			name = "Text"
		case "is.workflow.actions.number":
			name = "Number"
		case "is.workflow.actions.dictionary":
			name = "Dictionary"
		case "is.workflow.actions.list":
			name = "List"
		case "is.workflow.actions.date":
			name = "Date"
		case "is.workflow.actions.downloadurl":
			name = "Contents of URL"
		default:
			name = defaultActionOutputName(identifier)
		}
	}
	return WFTextTokenAttachment{
		WFSerializationType: "WFTextTokenAttachment",
		Value:               Value{Type: "ActionOutput", OutputUUID: id, OutputName: name},
	}, nil
}

// resolveDebugMacroCall is shared by Legacy and v2.
// Every debugLog() / debugLogPrevious() emits EXACTLY ONE Add to Variable
// and its WFInput is a magic-variable token linked to the preceding action.
// An explicit message is not accepted: the user must emit it themselves
// as a normal action if desired; logging does not synthesize Text actions.
func resolveDebugMacroCall(name string, params map[string]any, prior []ShortcutAction, outputUUID, outputName string) ([]ShortcutAction, error) {
	if name == "debugLog" || name == "debugLogPrevious" {
		if value, passed := params["WFInput"]; passed && value != nil {
			return nil, fmt.Errorf("%s() accepts only withDates/showLogs flags; it captures the previous native output", name)
		}
		if _, err := debugLiteralFlag(params, "WFDebugWithDates", name); err != nil {
			return nil, err
		}
		if _, err := debugLiteralFlag(params, "WFDebugShowLogs", name); err != nil {
			return nil, err
		}
		if _, err := debugLiteralFlag(params, "WFDebugShowLogsAsText", name); err != nil {
			return nil, err
		}
		ref, err := captureDebugPreviousInput(prior)
		if err != nil {
			return nil, err
		}
		flags := map[string]any{"WFInput": ref}
		for _, k := range []string{"WFDebugWithDates", "WFDebugShowLogs", "WFDebugShowLogsAsText"} {
			if value, exists := params[k]; exists {
				flags[k] = value
			}
		}
		return buildDebugStdlibActions("debugLog", flags, outputUUID, outputName)
	}
	if name == "debugTraceEvent" {
		stage, _ := params["WFDebugTraceStage"].(string)
		if strings.TrimSpace(stage) == "" {
			return nil, fmt.Errorf("debugTraceEvent requires a non-empty literal stage (before|after|error|end)")
		}
		switch stage {
		case "before", "after", "error", "end":
		default:
			return nil, fmt.Errorf("debugTraceEvent stage must be one of before, after, error, end; got %q", stage)
		}
		label, _ := params["WFDebugTraceLabel"].(string)
		cloned := map[string]any{
			"WFDebugTraceStage":        stage,
			"WFDebugTraceLabel":        label,
			"WFDebugTraceStepSequence": len(prior),
		}
		if len(prior) > 0 {
			prev := prior[len(prior)-1]
			cloned["WFDebugTracePrevActionID"] = prev.WFWorkflowActionIdentifier
			if stage == "after" {
				if ref, err := captureDebugPreviousInput(prior); err == nil {
					cloned["WFDebugTracePrevOutputRef"] = ref
				}
			}
		}
		return buildDebugStdlibActions("debugTraceEvent", cloned, outputUUID, outputName)
	}
	return buildDebugStdlibActions(name, params, outputUUID, outputName)
}

// A logged value must be an Apple WFTextTokenAttachment (not a string).
// Non-token input is rejected, rather than silently creating Text actions.
func isDirectDebugLogInput(value any) bool {
	switch v := value.(type) {
	case WFTextTokenAttachment:
		return v.WFSerializationType == "WFTextTokenAttachment"
	case *WFTextTokenAttachment:
		return v != nil && v.WFSerializationType == "WFTextTokenAttachment"
	case map[string]any:
		return v["WFSerializationType"] == "WFTextTokenAttachment"
	default:
		return false
	}
}

// These are public compiler macros rather than ordinary Apple action aliases:
// one call expands to several Apple actions at the same position. Keep them
// defined in the canonical Go action catalog so both frontends resolve them,
// while excluding them from the 505 Apple action catalog surface.
func init() { registerDebugStdlibActions() }

func registerDebugStdlibActions() {
	actions["debugLog"] = &actionDefinition{
		identifier:  "appendvariable",
		doc:         selfDoc{title: "Debug Log", description: "Append previous native output without coercion. Optional withDates captures original Creation Date at source; showLogs previews native objects (with Set Name by Creation Date when enabled); showLogsAsText opts into Combine Text or dated text rows.", category: "debug"},
		outputType:  String,
		emittedKeys: []string{"WFVariableName"},
		parameters: []parameterDefinition{
			{name: "withDates", key: "WFDebugWithDates", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogs", key: "WFDebugShowLogs", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogsAsText", key: "WFDebugShowLogsAsText", validType: Bool, optional: true, defaultValue: false},
		},
	}
	actions["debugShowLogs"] = &actionDefinition{
		identifier: "previewdocument",
		doc:        selfDoc{title: "Show Debug Logs", description: "Preview native objects, naming items by Creation Date when enabled; use showLogsAsText for text formatting.", category: "debug"},
		parameters: []parameterDefinition{
			{name: "withDates", key: "WFDebugWithDates", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogsAsText", key: "WFDebugShowLogsAsText", validType: Bool, optional: true, defaultValue: false},
		},
	}
	actions["debugClearLogs"] = &actionDefinition{
		identifier: "setvariable",
		doc:        selfDoc{title: "Clear Debug Logs", description: "Reset the execution-local debug log to an empty value.", category: "debug"},
	}

	actions["debugTrace"] = &actionDefinition{
		identifier: "appendvariable",
		doc:        selfDoc{title: "Trace Value and Time", description: "Append current time followed by a value to the execution-local debug log.", category: "debug"},
		parameters: []parameterDefinition{{name: "value", key: "WFInput", validType: String}},
		outputType: String,
	}
	actions["debugAssert"] = &actionDefinition{
		identifier: "conditional",
		doc:        selfDoc{title: "Assert Condition", description: "Stop the Shortcut with a message when the condition is false.", category: "debug"},
		parameters: []parameterDefinition{
			{name: "condition", key: "WFInput", validType: Bool},
			{name: "message", key: "WFAlertActionMessage", validType: String, optional: true, defaultValue: "Assertion failed"},
		},
	}
	actions["debugStartTimer"] = &actionDefinition{
		identifier: "date",
		doc:        selfDoc{title: "Start Debug Timer", description: "Remember the current time under a compile-time label.", category: "debug"},
		parameters: []parameterDefinition{{name: "label", key: "WFDebugTimerLabel", validType: String}},
	}
	actions["debugStopTimer"] = &actionDefinition{
		identifier: "gettimebetweendates",
		doc:        selfDoc{title: "Stop Debug Timer", description: "Measure elapsed seconds and append the result to the execution-local log.", category: "debug"},
		parameters: []parameterDefinition{{name: "label", key: "WFDebugTimerLabel", validType: String}},
		outputType: Float,
	}
	actions["debugFinish"] = &actionDefinition{
		identifier: "previewdocument",
		doc:        selfDoc{title: "Finish Debug Session", description: "Preview native objects with optional Creation Date names or display text with showLogsAsText.", category: "debug"},
		parameters: []parameterDefinition{
			{name: "withDates", key: "WFDebugWithDates", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogsAsText", key: "WFDebugShowLogsAsText", validType: Bool, optional: true, defaultValue: false},
		},
	}
	actions["debugLogPrevious"] = &actionDefinition{
		identifier: "appendvariable",
		doc:        selfDoc{title: "Log Previous Result", description: "Alias for debugLog with optional withDates/showLogs/showLogsAsText.", category: "debug"},
		outputType: String,
		parameters: []parameterDefinition{
			{name: "withDates", key: "WFDebugWithDates", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogs", key: "WFDebugShowLogs", validType: Bool, optional: true, defaultValue: false},
			{name: "showLogsAsText", key: "WFDebugShowLogsAsText", validType: Bool, optional: true, defaultValue: false},
		},
	}
	actions["debugTraceEvent"] = &actionDefinition{
		identifier: "appendvariable",
		doc:        selfDoc{title: "Record QA Structured Trace Event", description: "Append a structured JSONL trace record (stage, label, step sequence, and CurrentDate timestamp) to the execution-local QA trace buffer without mutating __cherri_debug_log.", category: "debug"},
		outputType: String,
		parameters: []parameterDefinition{
			{name: "stage", key: "WFDebugTraceStage", validType: String},
			{name: "label", key: "WFDebugTraceLabel", validType: String, optional: true, defaultValue: ""},
		},
	}
	actions["debugExportTrace"] = &actionDefinition{
		identifier: "gettext",
		doc:        selfDoc{title: "Export QA Structured Trace Snapshot", description: "Combine the execution-local QA trace buffer into a deterministic JSON envelope for QA inspection or callback export without mutating __cherri_debug_log.", category: "debug"},
		outputType: String,
		parameters: []parameterDefinition{
			{name: "runId", key: "WFDebugTraceRunID", validType: String, optional: true, defaultValue: "cherri-qa-run"},
		},
	}
}

// isDebugStdlibMacro is intentionally an exact-name predicate, so arbitrary
// third-party App Intents and similarly named user functions remain untouched.
func isDebugStdlibMacro(name string) bool {
	switch name {
	case "debugLog", "debugShowLogs", "debugClearLogs", "debugTrace", "debugAssert", "debugStartTimer", "debugStopTimer", "debugFinish", "debugLogPrevious", "debugTraceEvent", "debugExportTrace":
		return true
	default:
		return false
	}
}

// debugTraceEventToken builds a deterministic WFTextTokenString JSONL line with
// a live CurrentDate token so the exported trace records actual execution order.
func debugTraceEventToken(stage, label, prevActionID string, stepSeq int) map[string]any {
	safeStage := strings.ReplaceAll(strings.ReplaceAll(stage, `\`, `\\`), `"`, `\"`)
	safeLabel := strings.ReplaceAll(strings.ReplaceAll(label, `\`, `\\`), `"`, `\"`)
	safePrev := strings.ReplaceAll(strings.ReplaceAll(prevActionID, `\`, `\\`), `"`, `\"`)
	prefix := fmt.Sprintf(`{"schemaVersion":"1.0","kind":"INLINE_DEBUG_ACTION","stage":"%s","label":"%s","stepSequence":%d,"actionIdentifier":"%s","timestamp":"`,
		safeStage, safeLabel, stepSeq, safePrev)
	suffix := `"}`
	offset := len([]rune(prefix))
	rangeKey := fmt.Sprintf("{%d, 1}", offset)
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": prefix + "\uFFFC" + suffix,
			"attachmentsByRange": map[string]any{
				rangeKey: map[string]any{
					"Type": "CurrentDate",
				},
			},
		},
	}
}

// debugExportTraceEnvelopeToken wraps the combined JSONL trace lines in a
// deterministic JSON snapshot envelope correlated by runId.
func debugExportTraceEnvelopeToken(runID, combinedUUID string) map[string]any {
	safeRunID := strings.ReplaceAll(strings.ReplaceAll(runID, `\`, `\\`), `"`, `\"`)
	prefix := fmt.Sprintf(`{"schemaVersion":"1.0","kind":"COLLECTOR_EVENT","runId":"%s","traceTransport":"export-snapshot","eventsJsonl":"`, safeRunID)
	suffix := `"}`
	offset := len([]rune(prefix))
	rangeKey := fmt.Sprintf("{%d, 1}", offset)
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{
			"string": prefix + "\uFFFC" + suffix,
			"attachmentsByRange": map[string]any{
				rangeKey: map[string]any{
					"Type":       "ActionOutput",
					"OutputUUID": combinedUUID,
					"OutputName": "Combined Text",
				},
			},
		},
	}
}

// buildDebugStdlibActions is shared by Legacy and V2; no parallel emitters.
// resultUUID and resultName are optional metadata from the source frontend.
// debugLog() emits one Add to Variable action bound to the preceding result.
// debugLog(literal) is invalid: we do not emit Text actions to manufacture input.
func buildDebugStdlibActions(name string, params map[string]any, resultUUID, resultName string) ([]ShortcutAction, error) {
	add := func(id string, p map[string]any) ShortcutAction {
		return ShortcutAction{
			WFWorkflowActionIdentifier: "is.workflow.actions." + id,
			WFWorkflowActionParameters: p,
		}
	}
	switch name {
	case "debugLog":
		value, ok := params["WFInput"]
		if !ok || !isDirectDebugLogInput(value) {
			return nil, fmt.Errorf("debugLog requires preceding native ActionOutput")
		}
		withDates, err := debugLiteralFlag(params, "WFDebugWithDates", "debugLog")
		if err != nil {
			return nil, err
		}
		showLogs, err := debugLiteralFlag(params, "WFDebugShowLogs", "debugLog")
		if err != nil {
			return nil, err
		}
		showLogsAsText, err := debugLiteralFlag(params, "WFDebugShowLogsAsText", "debugLog")
		if err != nil {
			return nil, err
		}
		if showLogs && showLogsAsText {
			return nil, fmt.Errorf("debugLog: showLogs and showLogsAsText are alternative display modes; select only one")
		}
		appendParams := map[string]any{
			"WFVariableName": debugLogVariableName,
			"WFInput":        value,
		}
		if resultUUID != "" {
			appendParams["UUID"] = strings.ToUpper(resultUUID)
		}
		if resultName != "" {
			appendParams["CustomOutputName"] = resultName
		}
		result := make([]ShortcutAction, 0, 7)
		if withDates {
			source, ok := debugLogOutputRef(value)
			if !ok || source.OutputUUID == "" {
				return nil, fmt.Errorf("debugLog(withDates:true) requires previous output UUID")
			}
			// Must be inserted here, directly after the producer, BEFORE
			// even the Add to Variable action. This Date action is not the
			// logger output and never affects the typed original value.
			result = append(result, add("detect.date", map[string]any{
				"WFInput": debugLogDateSourceRef(source.OutputUUID, source.OutputName),
				"UUID":    strings.ToUpper(uuid.NewString()),
			}))
		}
		result = append(result, add("appendvariable", appendParams))
		if showLogs || showLogsAsText {
			preview, err := buildDebugStdlibActions("debugShowLogs",
				map[string]any{
					"WFDebugWithDates":      withDates,
					"WFDebugShowLogsAsText": showLogsAsText,
				}, "", "")
			if err != nil {
				return nil, err
			}
			result = append(result, preview...)
		}
		return result, nil

	case "debugTrace":
		value, ok := params["WFInput"]
		if !ok || value == nil {
			return nil, fmt.Errorf("debugTrace(value) requires an input")
		}
		nowID := strings.ToUpper(uuid.NewString())
		formatID := strings.ToUpper(uuid.NewString())
		currentTime := add("date", map[string]any{
			"WFDateActionMode": "Current Date",
			"UUID":             nowID,
		})
		formattedTime := add("format.date", map[string]any{
			"WFDate":            debugOutputRef(nowID),
			"WFDateFormatStyle": "None",
			"WFTimeFormatStyle": "Short",
			"UUID":              formatID,
		})
		appendTime := add("appendvariable", map[string]any{
			"WFVariableName": debugLogVariableName,
			"WFInput":        debugOutputRefNamed(formatID, "Formatted Date"),
		})
		if isDirectDebugLogInput(value) {
			rest, err := buildDebugStdlibActions("debugLog", map[string]any{"WFInput": value}, resultUUID, resultName)
			if err != nil {
				return nil, err
			}
			return append([]ShortcutAction{currentTime, formattedTime, appendTime}, rest...), nil
		}
		// debugTrace(value) is a SEPARATE opt-in formatting macro; unlike
		// debugLog(), its literal argument explicitly requests a Text producer.
		valueUUID := strings.ToUpper(uuid.NewString())
		rest, err := buildDebugStdlibActions("debugLog", map[string]any{"WFInput": debugOutputRefNamed(valueUUID, "Text")}, resultUUID, resultName)
		if err != nil {
			return nil, err
		}
		prefix := []ShortcutAction{currentTime, formattedTime, appendTime,
			add("gettext", map[string]any{"WFTextActionText": value, "UUID": valueUUID})}
		return append(prefix, rest...), nil
	case "debugAssert":
		condition, ok := params["WFInput"]
		if !ok || condition == nil {
			return nil, fmt.Errorf("debugAssert(condition, message) requires a condition")
		}
		msg := params["WFAlertActionMessage"]
		if msg == nil {
			msg = "Assertion failed"
		}
		alert := add("alert", map[string]any{
			"WFAlertActionMessage":           msg,
			"WFAlertActionCancelButtonShown": false,
		})
		stop := add("exit", map[string]any{})
		if literal, ok := condition.(bool); ok {
			if literal {
				return []ShortcutAction{}, nil
			}
			return []ShortcutAction{alert, stop}, nil
		}
		group := strings.ToUpper(uuid.NewString())
		start := add("conditional", BuildIfStart(group, map[string]any{
			"Type":     "Variable",
			"Variable": condition,
		}, 4, 0, true))
		end := add("conditional", BuildIfEnd(group, ""))
		return []ShortcutAction{start, alert, stop, end}, nil
	case "debugStartTimer":
		variableName, err := debugTimerLabel(params)
		if err != nil {
			return nil, err
		}
		// Apple's native Current Date magic variable can be used directly
		// as WFInput of Set Variable, without a preceding Date action.
		return []ShortcutAction{
			add("setvariable", map[string]any{
				"WFVariableName": variableName,
				"WFInput":        debugCurrentDateRef(),
			}),
		}, nil
	case "debugStopTimer":
		variableName, err := debugTimerLabel(params)
		if err != nil {
			return nil, err
		}
		elapsedID := strings.ToUpper(uuid.NewString())
		if resultUUID == "" {
			resultUUID = strings.ToUpper(uuid.NewString())
		}
		paramsOut := map[string]any{
			"WFVariable": debugOutputRefNamed(elapsedID, "Time Between Dates"),
			"UUID":       resultUUID,
		}
		if resultName != "" {
			paramsOut["CustomOutputName"] = resultName
		}
		return []ShortcutAction{
			add("gettimebetweendates", map[string]any{
				"WFTimeUntilFromDate": debugDateFieldToken(Value{Type: "Variable", VariableName: variableName}),
				"WFInput":             debugDateFieldToken(Value{Type: "CurrentDate"}),
				"WFTimeUntilUnit":     "Seconds",
				"UUID":                elapsedID,
			}),
			add("appendvariable", map[string]any{
				"WFVariableName": debugLogVariableName,
				"WFInput":        debugOutputRefNamed(elapsedID, "Time Between Dates"),
			}),
			add("getvariable", paramsOut),
		}, nil
	case "debugFinish":
		return buildDebugStdlibActions("debugShowLogs", params, resultUUID, resultName)
	case "debugLogPrevious":
		return nil, fmt.Errorf("debugLogPrevious requires compiler context to bind preceding output UUID")
	case "debugShowLogs":
		withDates, err := debugLiteralFlag(params, "WFDebugWithDates", "debugShowLogs")
		if err != nil {
			return nil, err
		}
		asText, err := debugLiteralFlag(params, "WFDebugShowLogsAsText", "debugShowLogs")
		if err != nil {
			return nil, err
		}
		var body []ShortcutAction
		var input any
		if asText && !withDates {
			// Only the explicit text mode performs Combine Text.
			combinedUUID := strings.ToUpper(uuid.NewString())
			body = []ShortcutAction{
				add("text.combine", map[string]any{
					"text":            debugLogValueRef(debugLogVariableName),
					"WFTextSeparator": "New Lines",
					"UUID":            combinedUUID,
				}),
			}
			input = debugOutputRefNamed(combinedUUID, "Combined Text")
		} else if !withDates {
			// Normal object preview has zero formatting/conversion actions.
			input = debugLogValueRef(debugLogVariableName)
		} else {
			group := strings.ToUpper(uuid.NewString())
			outputUUID := strings.ToUpper(uuid.NewString())
			itemUUID := strings.ToUpper(uuid.NewString())
			body = append(body, add("repeat.each", map[string]any{
				"WFInput":            debugLogValueRef(debugLogVariableName),
				"GroupingIdentifier": group,
				"WFControlFlowMode":  0,
			}))
			if asText {
				// Opt-in formatted text: Repeat Item - Creation Date.
				body = append(body, add("gettext", map[string]any{
					"WFTextActionText": debugDatedLogRow(),
					"UUID":             itemUUID,
				}))
			} else {
				// EXACTLY the user's attached iPadOS Shortcut: keep the item,
				// give it a Creation Date name and preview the Repeat Results.
				body = append(body, add("setitemname", map[string]any{
					"WFInput": debugLogValueRef("Repeat Item"),
					"WFName":  debugDatedNamedLogItem(),
					"UUID":    itemUUID,
				}))
			}
			body = append(body, add("repeat.each", map[string]any{
				"WFControlFlowMode":  2,
				"GroupingIdentifier": group,
				"UUID":               outputUUID,
			}))
			input = debugOutputRefNamed(outputUUID, "Repeat Results")
		}
		previewParams := map[string]any{"WFInput": input}
		if resultUUID != "" {
			previewParams["UUID"] = resultUUID
		}
		if resultName != "" {
			previewParams["CustomOutputName"] = resultName
		}
		return append(body, add("previewdocument", previewParams)), nil

	case "debugClearLogs":
		id := strings.ToUpper(uuid.NewString())
		return []ShortcutAction{
			add("gettext", map[string]any{
				"WFTextActionText": "",
				"UUID":             id,
			}),
			add("setvariable", map[string]any{
				"WFVariableName": debugLogVariableName,
				"WFInput":        debugOutputRefNamed(id, "Text"),
			}),
		}, nil

	case "debugTraceEvent":
		stage, _ := params["WFDebugTraceStage"].(string)
		if strings.TrimSpace(stage) == "" {
			return nil, fmt.Errorf("debugTraceEvent requires a non-empty literal stage")
		}
		label, _ := params["WFDebugTraceLabel"].(string)
		stepSeq, _ := params["WFDebugTraceStepSequence"].(int)
		prevID, _ := params["WFDebugTracePrevActionID"].(string)
		lineUUID := strings.ToUpper(uuid.NewString())
		appendParams := map[string]any{
			"WFVariableName": debugQATraceVariableName,
			"WFInput":        debugOutputRefNamed(lineUUID, "Text"),
		}
		if resultUUID != "" {
			appendParams["UUID"] = strings.ToUpper(resultUUID)
		}
		if resultName != "" {
			appendParams["CustomOutputName"] = resultName
		}
		return []ShortcutAction{
			add("gettext", map[string]any{
				"WFTextActionText": debugTraceEventToken(stage, label, prevID, stepSeq),
				"UUID":             lineUUID,
			}),
			add("appendvariable", appendParams),
		}, nil

	case "debugExportTrace":
		runID, _ := params["WFDebugTraceRunID"].(string)
		if strings.TrimSpace(runID) == "" {
			runID = "cherri-qa-run"
		}
		combinedUUID := strings.ToUpper(uuid.NewString())
		envelopeUUID := resultUUID
		if envelopeUUID == "" {
			envelopeUUID = strings.ToUpper(uuid.NewString())
		} else {
			envelopeUUID = strings.ToUpper(envelopeUUID)
		}
		envelopeParams := map[string]any{
			"WFTextActionText": debugExportTraceEnvelopeToken(runID, combinedUUID),
			"UUID":             envelopeUUID,
		}
		if resultName != "" {
			envelopeParams["CustomOutputName"] = resultName
		}
		return []ShortcutAction{
			add("text.combine", map[string]any{
				"text":            debugLogValueRef(debugQATraceVariableName),
				"WFTextSeparator": "New Lines",
				"UUID":            combinedUUID,
			}),
			add("gettext", envelopeParams),
		}, nil

	default:
		return nil, fmt.Errorf("unknown debug macro %q", name)
	}
}

// DebugTraceActionMapping records the relationship between an action in a compiled
// Shortcut and whether it belongs to the user's source workflow or an injected
// debug/trace expansion.
type DebugTraceActionMapping struct {
	NativeActionIndex int    `json:"nativeActionIndex"`
	SourceActionIndex int    `json:"sourceActionIndex"` // -1 for injected debug actions
	ActionIdentifier  string `json:"actionIdentifier"`
	Kind              string `json:"kind"` // SOURCE_ACTION | INLINE_DEBUG_ACTION | COLLECTOR_EVENT
	VariableTarget    string `json:"variableTarget,omitempty"`
	PreservesProducer bool   `json:"preservesProducer"`
}

// ClassifyShortcutTraceActions classifies the native actions of a compiled Shortcut
// into SOURCE_ACTION, INLINE_DEBUG_ACTION, and COLLECTOR_EVENT so QA harnesses
// never confuse injected debug/trace helpers with user source actions.
func ClassifyShortcutTraceActions(actions []ShortcutAction) []DebugTraceActionMapping {
	mappings := make([]DebugTraceActionMapping, 0, len(actions))
	sourceIdx := 0
	for i, act := range actions {
		id := act.WFWorkflowActionIdentifier
		params := act.WFWorkflowActionParameters
		varName, _ := params["WFVariableName"].(string)
		kind := "SOURCE_ACTION"
		srcIdx := sourceIdx
		preserves := true

		isDebugVar := varName == debugLogVariableName || varName == debugQATraceVariableName || strings.HasPrefix(varName, debugTimerVariablePrefix)
		if isDebugVar {
			kind = "INLINE_DEBUG_ACTION"
			srcIdx = -1
		} else if id == "is.workflow.actions.detect.date" {
			if src, ok := debugLogOutputRef(params["WFInput"]); ok && src.OutputUUID != "" {
				if tok, okTok := params["WFInput"].(WFTextTokenAttachment); okTok && len(tok.Value.Aggrandizements) == 1 && tok.Value.Aggrandizements[0].PropertyUserInfo == "WFFileCreationDate" {
					kind = "INLINE_DEBUG_ACTION"
					srcIdx = -1
				} else if m, okMap := params["WFInput"].(map[string]any); okMap {
					if v, okV := m["Value"].(map[string]any); okV {
						if aggs, okA := v["Aggrandizements"].([]any); okA && len(aggs) == 1 {
							if a0, ok0 := aggs[0].(map[string]any); ok0 && a0["PropertyUserInfo"] == "WFFileCreationDate" {
								kind = "INLINE_DEBUG_ACTION"
								srcIdx = -1
							}
						}
					}
				}
			}
		} else if id == "is.workflow.actions.text.combine" {
			if m, okMap := params["text"].(map[string]any); okMap {
				if v, okV := m["Value"].(map[string]any); okV {
					if vn, _ := v["VariableName"].(string); vn == debugLogVariableName || vn == debugQATraceVariableName {
						kind = "COLLECTOR_EVENT"
						srcIdx = -1
					}
				}
			} else if tok, okTok := params["text"].(WFTextTokenAttachment); okTok {
				if tok.Value.VariableName == debugLogVariableName || tok.Value.VariableName == debugQATraceVariableName {
					kind = "COLLECTOR_EVENT"
					srcIdx = -1
				}
			}
		} else if id == "is.workflow.actions.gettext" {
			if m, okMap := params["WFTextActionText"].(map[string]any); okMap {
				if v, okV := m["Value"].(map[string]any); okV {
					if s, _ := v["string"].(string); strings.Contains(s, `"kind":"INLINE_DEBUG_ACTION"`) {
						kind = "INLINE_DEBUG_ACTION"
						srcIdx = -1
					} else if strings.Contains(s, `"kind":"COLLECTOR_EVENT"`) {
						kind = "COLLECTOR_EVENT"
						srcIdx = -1
					}
				}
			} else if s, okStr := params["WFTextActionText"].(string); okStr && s == "" {
				if i+1 < len(actions) && actions[i+1].WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" &&
					actions[i+1].WFWorkflowActionParameters["WFVariableName"] == debugLogVariableName {
					kind = "INLINE_DEBUG_ACTION"
					srcIdx = -1
				}
			}
		}

		if kind == "SOURCE_ACTION" {
			sourceIdx++
		}
		mappings = append(mappings, DebugTraceActionMapping{
			NativeActionIndex: i,
			SourceActionIndex: srcIdx,
			ActionIdentifier:  id,
			Kind:              kind,
			VariableTarget:    varName,
			PreservesProducer: preserves,
		})
	}
	return mappings
}
