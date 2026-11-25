package main

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

const debugLabelVariableName = "__cherri_debug_log_labels"
const debugLabelDisplayVariablePrefix = "__cherri_debug_label_display_"
const debugLabelContextKey = "_cherriDebugLabelContext"

// Omission and an explicit empty label are distinct. Literal labels cannot
// evaluate another action and accidentally replace the previous output.
func debugLiteralLogLabel(params map[string]any, name string) (string, bool, error) {
	value, present := params["WFDebugLogLabel"]
	if !present {
		return "", false, nil
	}
	label, ok := value.(string)
	if !ok {
		return "", false, fmt.Errorf("%s: label must be literal Text", name)
	}
	return label, true, nil
}

func debugOwnedInlineAction(actions []ShortcutAction, index int) bool {
	if index < 0 || index >= len(actions) || !currentDebugInstrumentation.inlineIndices[index] {
		return false
	}
	id, _ := actions[index].WFWorkflowActionParameters["UUID"].(string)
	return id == "" || currentDebugInstrumentation.inlineUUIDs[id]
}

func debugPriorLabels(actions []ShortcutAction) bool {
	for i, action := range actions {
		if action.WFWorkflowActionIdentifier == "is.workflow.actions.appendvariable" &&
			action.WFWorkflowActionParameters["WFVariableName"] == debugLabelVariableName &&
			debugOwnedInlineAction(actions, i) {
			return true
		}
	}
	return false
}

func debugMacroRequestsLabels(name string, params map[string]any) bool {
	if name != "debugLog" && name != "debugLogPrevious" {
		return false
	}
	_, present, err := debugLiteralLogLabel(params, name)
	return present && err == nil
}

// Both frontends pass their literal bytes through this macro-only encoder.
// Ordinary Legacy strings normally use interpolation encoding; labels must not
// resolve braces as source variable names or change the preceding action.
func debugLogMacroParameters(arguments []actionArgument) map[string]any {
	args := append([]actionArgument(nil), arguments...)
	invalid := false
	if len(args) > 3 && args[3].valueType != Nil {
		_, literal := args[3].value.(string)
		invalid = !literal || (args[3].valueType != String && args[3].valueType != RawString)
		if invalid {
			// The shared macro resolver returns a regular compiler diagnostic
			// for both profiles. Do not terminate embedded compiler callers.
			args[3] = actionArgument{valueType: Nil}
		} else {
			args[3].valueType = RawString
		}
	}
	params := map[string]any{}
	makeActionParams(args, params)
	if invalid {
		params["WFDebugLogLabel"] = struct{}{}
	}
	return params
}

type debugLabelExpansion struct{ actions []ShortcutAction }

func (b *debugLabelExpansion) add(id string, params map[string]any) WFTextTokenAttachment {
	output, _ := params["UUID"].(string)
	if output == "" {
		output = strings.ToUpper(nextGeneratedUUID("debug-labels"))
		params["UUID"] = output
	}
	b.actions = append(b.actions, ShortcutAction{
		WFWorkflowActionIdentifier: "is.workflow.actions." + id,
		WFWorkflowActionParameters: params,
	})
	return debugOutputRef(output)
}

func (b *debugLabelExpansion) count(value any, kind string) WFTextTokenAttachment {
	return b.add("count", map[string]any{"Input": value, "WFCountType": kind})
}

func (b *debugLabelExpansion) beginNumber(input any, code int, value any) string {
	group := strings.ToUpper(nextGeneratedUUID("debug-labels"))
	b.add("conditional", backend.BuildIfStartTyped(group,
		map[string]any{"Type": "Variable", "Variable": input},
		code, value, nil, backend.OperandNumber, true))
	return group
}

func (b *debugLabelExpansion) otherwise(group string) {
	b.add("conditional", BuildIfElse(group))
}

func (b *debugLabelExpansion) end(group string) WFTextTokenAttachment {
	return b.add("conditional", BuildIfEnd(group, ""))
}

func debugLabelClearActions() []ShortcutAction {
	b := &debugLabelExpansion{}
	b.add("nothing", map[string]any{})
	// Keep WFInput absent. Empty Text would be a real metadata item.
	b.add("setvariable", map[string]any{"WFVariableName": debugLabelVariableName})
	return b.actions
}

func debugLabelClearParameters(action ShortcutAction, setVariable bool) bool {
	for key := range action.WFWorkflowActionParameters {
		if key == "UUID" || key == "CustomOutputName" || (setVariable && key == "WFVariableName") {
			continue
		}
		return false
	}
	return true
}

// A clear compiled before the first label must clear the eventual metadata too,
// including when the clear and label are in different runtime branches. Only
// compiler-owned, exact zero-output clears are eligible; raw lookalikes stay raw.
func backfillDebugLogLabels(prior []ShortcutAction) ([]ShortcutAction, int, error) {
	after := map[int][]ShortcutAction{}
	for i, action := range prior {
		p := action.WFWorkflowActionParameters
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.setvariable" ||
			p["WFVariableName"] != debugLogVariableName || !debugOwnedInlineAction(prior, i) {
			continue
		}
		_, hasInput := p["WFInput"]
		if i == 0 || hasInput || prior[i-1].WFWorkflowActionIdentifier != "is.workflow.actions.nothing" ||
			!debugOwnedInlineAction(prior, i-1) || !debugLabelClearParameters(prior[i-1], false) || !debugLabelClearParameters(action, true) {
			return nil, 0, fmt.Errorf("debugLog label metadata requires the exact owned zero-output clear")
		}
		if i+2 < len(prior) && prior[i+1].WFWorkflowActionIdentifier == "is.workflow.actions.nothing" &&
			prior[i+2].WFWorkflowActionIdentifier == "is.workflow.actions.setvariable" &&
			prior[i+2].WFWorkflowActionParameters["WFVariableName"] == debugLabelVariableName &&
			debugOwnedInlineAction(prior, i+1) && debugOwnedInlineAction(prior, i+2) {
			if !debugLabelClearParameters(prior[i+1], false) || !debugLabelClearParameters(prior[i+2], true) {
				return nil, 0, fmt.Errorf("debugLog label metadata clear must have no input or unreviewed fields")
			}
			continue
		}
		after[i] = debugLabelClearActions()
	}
	if len(after) == 0 {
		return prior, 0, nil
	}
	inserted := 0
	for _, actions := range after {
		for _, action := range actions {
			recordDebugBackfilledActionUUID(action.WFWorkflowActionParameters["UUID"].(string))
			inserted++
		}
	}
	shiftDebugManifestIndicesAfterBackfill(after, len(prior))
	result := make([]ShortcutAction, 0, len(prior)+inserted)
	for i, action := range prior {
		result = append(result, action)
		result = append(result, after[i]...)
	}
	return result, inserted, nil
}

func debugLabelIsVariable(raw any, name string) bool {
	switch value := raw.(type) {
	case WFTextTokenAttachment:
		return value.WFSerializationType == "WFTextTokenAttachment" && value.Value.Type == "Variable" && value.Value.VariableName == name && len(value.Value.Aggrandizements) == 0
	case map[string]any:
		v, ok := value["Value"].(map[string]any)
		return ok && value["WFSerializationType"] == "WFTextTokenAttachment" && v["Type"] == "Variable" && v["VariableName"] == name && v["Aggrandizements"] == nil
	}
	return false
}

// A preview earlier in source can execute after a label in a later loop
// iteration. Rewrite only exact compiler-owned preview expansions when the
// first label is encountered; ordinary/raw Quick Look actions stay untouched.
func backfillDebugLabelContext(prior []ShortcutAction) ([]ShortcutAction, int, error) {
	hadLabels := debugPriorLabels(prior)
	result, inserted, err := backfillDebugLogLabels(prior)
	if err != nil || hadLabels {
		return result, inserted, err
	}
	prior = result
	type replacement struct {
		end     int
		actions []ShortcutAction
	}
	replacements := map[int]replacement{}
	for i, action := range prior {
		if action.WFWorkflowActionIdentifier != "is.workflow.actions.previewdocument" || !debugOwnedInlineAction(prior, i) {
			continue
		}
		p := action.WFWorkflowActionParameters
		start, dates, text := i, false, false
		if !debugLabelIsVariable(p["WFInput"], debugLogVariableName) {
			ref, ok := debugLogOutputRef(p["WFInput"])
			if !ok || i == 0 || !debugOwnedInlineAction(prior, i-1) || prior[i-1].WFWorkflowActionParameters["UUID"] != ref.OutputUUID {
				continue
			}
			previous := prior[i-1]
			pp := previous.WFWorkflowActionParameters
			if previous.WFWorkflowActionIdentifier == "is.workflow.actions.text.combine" && debugLabelIsVariable(pp["text"], debugLogVariableName) && pp["WFTextSeparator"] == "New Lines" {
				start, text = i-1, true
			} else if previous.WFWorkflowActionIdentifier == "is.workflow.actions.repeat.each" && i >= 3 &&
				pp["WFControlFlowMode"] == 2 && debugOwnedInlineAction(prior, i-2) && debugOwnedInlineAction(prior, i-3) {
				loop, body := prior[i-3], prior[i-2]
				lp, bp := loop.WFWorkflowActionParameters, body.WFWorkflowActionParameters
				if loop.WFWorkflowActionIdentifier != "is.workflow.actions.repeat.each" || lp["WFControlFlowMode"] != 0 || lp["GroupingIdentifier"] != pp["GroupingIdentifier"] || !debugLabelIsVariable(lp["WFInput"], debugLogVariableName) {
					continue
				}
				if body.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && reflect.DeepEqual(bp["WFTextActionText"], debugDatedLogRow()) {
					text = true
				} else if body.WFWorkflowActionIdentifier != "is.workflow.actions.setitemname" || !debugLabelIsVariable(bp["WFInput"], "Repeat Item") || !reflect.DeepEqual(bp["WFName"], debugDatedNamedLogItem()) {
					continue
				}
				start, dates = i-3, true
			} else {
				continue
			}
		}
		output, _ := p["UUID"].(string)
		name, _ := p["CustomOutputName"].(string)
		expanded, err := buildDebugLabeledPreview(dates, text, output, name)
		if err != nil {
			return nil, 0, err
		}
		replacements[start] = replacement{end: i, actions: expanded}
	}
	if len(replacements) == 0 {
		return prior, inserted, nil
	}
	result = make([]ShortcutAction, 0, len(prior))
	inline, collector := map[int]bool{}, map[int]bool{}
	for i := 0; i < len(prior); i++ {
		if replacement, ok := replacements[i]; ok {
			for removed := i; removed <= replacement.end; removed++ {
				if id, ok := prior[removed].WFWorkflowActionParameters["UUID"].(string); ok {
					delete(currentDebugInstrumentation.inlineUUIDs, id)
				}
			}
			for _, action := range replacement.actions {
				inline[len(result)] = true
				recordDebugBackfilledActionUUID(action.WFWorkflowActionParameters["UUID"].(string))
				result = append(result, action)
			}
			inserted += len(replacement.actions) - (replacement.end - i + 1)
			i = replacement.end
			continue
		}
		if currentDebugInstrumentation.inlineIndices[i] {
			inline[len(result)] = true
		}
		if currentDebugInstrumentation.collectorIdxs[i] {
			collector[len(result)] = true
		}
		result = append(result, prior[i])
	}
	currentDebugInstrumentation.inlineIndices, currentDebugInstrumentation.collectorIdxs = inline, collector
	return result, inserted, nil
}

func debugLabelLiteral(label string) map[string]any {
	return map[string]any{
		"WFSerializationType": "WFTextTokenString",
		"Value":               map[string]any{"string": label},
	}
}

// Labels are scalar metadata for the native buffer range appended by this call.
// The Dictionary never contains the raw logged value. Count/append cardinality
// across native content classes still needs source-bound Apple verification.
func debugLabelMetadataActions(value any, label string) []ShortcutAction {
	b := &debugLabelExpansion{}
	last := b.count(debugLogValueRef(debugLogVariableName), "Items")
	size := b.count(value, "Items")
	before := b.add("math", map[string]any{
		"WFInput": last, "WFMathOperation": "-", "WFMathOperand": debugDateFieldToken(size.Value),
	})
	first := b.add("math", map[string]any{
		"WFInput": before, "WFMathOperation": "+", "WFMathOperand": 1.0,
	})
	var fields any = []WFDictionaryFieldValueItem{
		buildDictionaryItem("first", itemTypeNumber, debugDateFieldToken(first.Value)),
		buildDictionaryItem("label", itemTypeText, debugLabelLiteral(label)),
		buildDictionaryItem("last", itemTypeNumber, debugDateFieldToken(last.Value)),
	}
	metadata := b.add("dictionary", map[string]any{"WFItems": makeExplicitDictionaryValue(&fields)})
	b.add("appendvariable", map[string]any{"WFVariableName": debugLabelVariableName, "WFInput": metadata})
	return b.actions
}

func debugLabelToken(ref WFTextTokenAttachment) map[string]any {
	value := map[string]any{"Type": ref.Value.Type}
	if ref.Value.Type == "Variable" {
		value["VariableName"] = ref.Value.VariableName
	} else {
		value["OutputUUID"] = ref.Value.OutputUUID
		value["OutputName"] = ref.Value.OutputName
	}
	return value
}

func debugLabelDateToken(ref WFTextTokenAttachment) map[string]any {
	value := debugLabelToken(ref)
	value["Aggrandizements"] = []any{
		map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Creation Date", "PropertyUserInfo": "WFFileCreationDate"},
		map[string]any{"Type": "WFDateFormatVariableAggrandizement", "WFDateFormatStyle": "Medium", "WFTimeFormatStyle": "Long", "WFISO8601IncludeTime": false},
	}
	return value
}

// Native object/name tokens use exact UTF-16 offsets and lowercase wire keys.
// A label's braces are literal metadata, never source interpolation syntax.
func debugLabelText(parts ...any) map[string]any {
	var text strings.Builder
	attachments := map[string]any{}
	for _, part := range parts {
		switch value := part.(type) {
		case string:
			text.WriteString(value)
		case map[string]any:
			offset := utf16CodeUnitCount(text.String())
			attachments[fmt.Sprintf("{%d, 1}", offset)] = value
			text.WriteString("\uFFFC")
		default:
			panic("invalid internal debug label text segment")
		}
	}
	return map[string]any{"WFSerializationType": "WFTextTokenString",
		"Value": map[string]any{"string": text.String(), "attachmentsByRange": attachments}}
}

func (b *debugLabelExpansion) previewItem(item WFTextTokenAttachment, labelVariable string, dates, text, labeled bool) {
	var parts []any
	if labeled {
		parts = append(parts, debugLabelToken(debugLogValueRef(labelVariable)))
		if text {
			parts = append(parts, ": ")
		}
	}
	if text {
		parts = append(parts, debugLabelToken(item))
	}
	if dates {
		if len(parts) > 0 {
			parts = append(parts, " - ")
		}
		parts = append(parts, debugLabelDateToken(item))
	}
	if text {
		b.add("gettext", map[string]any{"WFTextActionText": debugLabelText(parts...)})
	} else if labeled || dates {
		// Presentation names a copy. Raw buffer and original producer stay exact.
		b.add("setitemname", map[string]any{"WFInput": item, "WFName": debugLabelText(parts...)})
	} else {
		b.add("getvariable", map[string]any{"WFVariable": item})
	}
}

func buildDebugLabeledPreview(dates, text bool, resultUUID, resultName string) ([]ShortcutAction, error) {
	b := &debugLabelExpansion{}
	metadataCount := b.count(debugLogValueRef(debugLabelVariableName), "Items")
	hasMetadata := b.beginNumber(metadataCount, 2, 0)
	outer := strings.ToUpper(nextGeneratedUUID("debug-labels"))
	b.add("repeat.each", backend.BuildRepeatEachStart(outer, debugLogValueRef(debugLogVariableName)))
	// Snapshot the actual outer values before the metadata loop changes magic variables.
	item := b.add("getvariable", map[string]any{"WFVariable": debugLogValueRef("Repeat Item")})
	index := b.add("getvariable", map[string]any{"WFVariable": debugLogValueRef("Repeat Index")})
	labelVariable := debugLabelDisplayVariablePrefix + strings.ReplaceAll(outer, "-", "")
	empty := b.add("gettext", map[string]any{"WFTextActionText": debugLabelLiteral("")})
	b.add("setvariable", map[string]any{"WFVariableName": labelVariable, "WFInput": empty})
	inner := strings.ToUpper(nextGeneratedUUID("debug-labels"))
	b.add("repeat.each", backend.BuildRepeatEachStart(inner, debugLogValueRef(debugLabelVariableName)))
	field := func(key string) WFTextTokenAttachment {
		return b.add("getvalueforkey", map[string]any{
			"WFInput": debugLogValueRef("Repeat Item"), "WFGetDictionaryValueType": "Value", "WFDictionaryKey": key,
		})
	}
	first, last := field("first"), field("last")
	before := b.beginNumber(index, 0, debugDateFieldToken(first.Value))
	b.add("nothing", map[string]any{})
	b.otherwise(before)
	after := b.beginNumber(index, 2, debugDateFieldToken(last.Value))
	b.add("nothing", map[string]any{})
	b.otherwise(after)
	label := field("label")
	b.add("setvariable", map[string]any{"WFVariableName": labelVariable, "WFInput": label})
	b.end(after)
	b.end(before)
	b.add("repeat.each", backend.BuildLoopEnd(inner, ""))
	labelLength := b.count(debugLogValueRef(labelVariable), "Characters")
	hasLabel := b.beginNumber(labelLength, 2, 0)
	b.previewItem(item, labelVariable, dates, text, true)
	b.otherwise(hasLabel)
	b.previewItem(item, labelVariable, dates, text, false)
	b.end(hasLabel)
	rows := b.add("repeat.each", backend.BuildLoopEnd(outer, ""))
	b.add("previewdocument", map[string]any{"WFInput": rows})
	b.otherwise(hasMetadata)
	fallback, err := buildDebugStdlibActions("debugShowLogs", map[string]any{
		"WFDebugWithDates": dates, "WFDebugShowLogsAsText": text,
	}, "", "")
	if err != nil {
		return nil, err
	}
	// Existing unlabeled preview behavior is the runtime fallback for skipped
	// label branches or cleared metadata. Every helper retains compiler provenance.
	for _, action := range fallback {
		b.add(strings.TrimPrefix(action.WFWorkflowActionIdentifier, "is.workflow.actions."), action.WFWorkflowActionParameters)
	}
	end := BuildIfEnd(hasMetadata, resultUUID)
	if resultName != "" {
		end["CustomOutputName"] = resultName
	}
	b.add("conditional", end)
	return b.actions, nil
}
