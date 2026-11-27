package main

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// Metadata stays separate from native log items. Every private variable is
// scoped to this expansion, and no metadata branch writes the shared buffers.
type debugExportMetadata struct {
	b                              *debugExportExpansion
	options                        debugExportOptions
	prefix                         string
	textType, dateType, numberType WFTextTokenAttachment
	dictionaryType                 WFTextTokenAttachment
}

func (m *debugExportMetadata) variable(suffix string) string { return m.prefix + suffix }

func (b *debugExportExpansion) readVariable(name string) WFTextTokenAttachment {
	return b.produce("getvariable", map[string]any{"WFVariable": debugLogValueRef(name)})
}

func (b *debugExportExpansion) setVariable(name string, value any) {
	b.add("setvariable", map[string]any{"WFVariableName": name, "WFInput": value})
}

func (b *debugExportExpansion) number(value float64) WFTextTokenAttachment {
	return b.produce("number", map[string]any{"WFNumberActionNumber": value})
}

func (b *debugExportExpansion) text(value string) WFTextTokenAttachment {
	return b.produce("gettext", map[string]any{"WFTextActionText": value})
}

func (b *debugExportExpansion) count(value any) WFTextTokenAttachment {
	return b.produce("count", map[string]any{"Input": value, "WFCountType": "Items"})
}

func (b *debugExportExpansion) kind(value any) WFTextTokenAttachment {
	return b.produce("getitemtype", map[string]any{"WFInput": value})
}

func (b *debugExportExpansion) math(value any, operation string, operand any) WFTextTokenAttachment {
	return b.produce("math", map[string]any{"WFInput": value, "WFMathOperation": operation, "WFMathOperand": operand})
}

func (b *debugExportExpansion) numberBranch(input any, code int, comparison any, yes, no func()) WFTextTokenAttachment {
	group := strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
	b.add("conditional", backend.BuildIfStartTyped(group,
		map[string]any{"Type": "Variable", "Variable": input}, code, comparison, nil, backend.OperandNumber, true))
	yes()
	if no != nil {
		b.add("conditional", BuildIfElse(group))
		no()
	}
	return b.produce("conditional", BuildIfEnd(group, ""))
}

func (b *debugExportExpansion) typeBranch(actual, witness WFTextTokenAttachment, yes, no func()) WFTextTokenAttachment {
	group := strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
	b.add("conditional", backend.BuildIfStartTyped(group,
		map[string]any{"Type": "Variable", "Variable": actual}, 4, requireComparisonText(witness), nil, backend.OperandString, true))
	yes()
	if no != nil {
		b.add("conditional", BuildIfElse(group))
		no()
	}
	return b.produce("conditional", BuildIfEnd(group, ""))
}

func (b *debugExportExpansion) dictionaryField(input any, key string) WFTextTokenAttachment {
	return b.produce("getvalueforkey", map[string]any{
		"WFInput": input, "WFGetDictionaryValueType": "Value", "WFDictionaryKey": key,
	})
}

func (b *debugExportExpansion) listItem(input any, index WFTextTokenAttachment) WFTextTokenAttachment {
	return b.produce("getitemfromlist", map[string]any{
		"WFInput": input, "WFItemSpecifier": "Item At Index", "WFItemIndex": debugDateFieldToken(index.Value),
	})
}

func debugExportMetadataStatus(status string) string {
	return `{"status":"` + status + `","value":null}`
}

func (m *debugExportMetadata) date(item WFTextTokenAttachment) WFTextTokenAttachment {
	b := m.b
	// Reading the actual item's property is distinct from assigning a log-time
	// timestamp. No detect.date action runs ahead of the absence/type guards.
	date := b.produce("getvariable", map[string]any{
		"WFVariable": debugLogDateSourceRef(item.Value.OutputUUID, item.Value.OutputName),
	})
	count := b.count(date)
	absent := func() { b.text(`{"source":"source-creation-date","status":"ABSENT","value":null}`) }
	ambiguous := func() { b.text(`{"source":"source-creation-date","status":"AMBIGUOUS","value":null}`) }
	unsupported := func() { b.text(`{"source":"source-creation-date","status":"UNSUPPORTED_DATE_TYPE","value":null}`) }
	return b.numberBranch(count, 4, 0, absent, func() {
		b.numberBranch(count, 4, 1, func() {
			actual := b.kind(date)
			b.typeBranch(actual, m.dateType, func() {
				formatted := b.produce("format.date", map[string]any{
					"WFDate":            debugDateFieldToken(date.Value),
					"WFDateFormatStyle": "ISO 8601", "WFISO8601IncludeTime": true,
				})
				encoded := b.base64(formatted)
				b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(
					`{"source":"source-creation-date","status":"PRESENT","format":"iso8601","encoding":"base64-utf8","value":"`, encoded, `"}`)})
			}, unsupported)
		}, ambiguous)
	})
}

func (m *debugExportMetadata) entry(item WFTextTokenAttachment, label any) WFTextTokenAttachment {
	b, options := m.b, m.options
	var actualType WFTextTokenAttachment
	if options.includeTypes || m.payloadEnabled() {
		actualType = b.kind(item)
	}
	var typeValue *WFTextTokenAttachment
	if options.includeTypes {
		encoded := b.base64(actualType)
		typeValue = &encoded
	}
	var metadata []any
	if options.includeDates {
		metadata = append(metadata, `,"date":`, m.date(item))
	}
	if options.includeLabels {
		metadata = append(metadata, `,"label":`, label)
	}
	if options.redact && len(options.redactKeys) == 0 {
		return b.row("REDACTED", typeValue, nil, metadata...)
	}
	return m.selectedPayload(item, actualType, typeValue, metadata...)
}

// Validate the source-owned range metadata once. Wrong native types and
// cardinalities cannot reach arithmetic or dictionary/list coercion. An invalid
// or overlapping range remains observable; the exporter never guesses a label.
func (m *debugExportMetadata) validateLabels(itemCount WFTextTokenAttachment) {
	b := m.b
	status, previous, valid := m.variable("label_status"), m.variable("previous_last"), m.variable("range_valid")
	b.setVariable(status, b.number(0))
	b.setVariable(previous, b.number(0))
	unsupported := func() { b.setVariable(status, b.number(2)) }
	ambiguous := func() {
		b.numberBranch(b.readVariable(status), 4, 0, func() { b.setVariable(status, b.number(1)) }, nil)
	}
	invalidRange := func() { b.setVariable(valid, b.number(0)) }
	group := strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
	b.add("repeat.each", backend.BuildRepeatEachStart(group, debugLogValueRef(debugLabelVariableName)))
	item := b.readVariable("Repeat Item")
	b.typeBranch(b.kind(item), m.dictionaryType, func() {
		first, last, label := b.dictionaryField(item, "first"), b.dictionaryField(item, "last"), b.dictionaryField(item, "label")
		b.setVariable(valid, b.number(1))
		for _, field := range []struct {
			value, witness WFTextTokenAttachment
		}{{first, m.numberType}, {last, m.numberType}, {label, m.textType}} {
			field := field
			b.numberBranch(b.count(field.value), 4, 1, func() {
				b.typeBranch(b.kind(field.value), field.witness, func() { b.add("nothing", map[string]any{}) }, invalidRange)
			}, invalidRange)
		}
		b.numberBranch(b.readVariable(valid), 4, 1, func() {
			for _, field := range []WFTextTokenAttachment{first, last} {
				rounded := b.produce("round", map[string]any{
					"WFInput": debugDateFieldToken(field.Value), "WFRoundTo": "Ones Place", "WFRoundMode": "Always Round Down",
				})
				b.numberBranch(field, 5, debugDateFieldToken(rounded.Value), invalidRange, nil)
			}
			b.numberBranch(first, 0, 1, invalidRange, nil)
			b.numberBranch(last, 0, 0, invalidRange, nil)
			b.numberBranch(last, 2, debugDateFieldToken(itemCount.Value), invalidRange, nil)
			afterLast := b.math(last, "+", 1.0)
			b.numberBranch(first, 2, debugDateFieldToken(afterLast.Value), invalidRange, nil)
			b.numberBranch(b.readVariable(valid), 4, 1, func() {
				oldLast := b.readVariable(previous)
				b.numberBranch(last, 0, debugDateFieldToken(oldLast.Value), ambiguous, nil)
				b.numberBranch(first, 1, debugDateFieldToken(oldLast.Value), func() {
					b.numberBranch(last, 3, debugDateFieldToken(first.Value), ambiguous, nil)
				}, nil)
				b.setVariable(previous, last)
			}, unsupported)
		}, unsupported)
	}, unsupported)
	// Helper outputs are never label records or Repeat Results.
	b.add("nothing", map[string]any{})
	b.add("repeat.each", backend.BuildLoopEnd(group, ""))
}

// A monotone merge of N native items and M ordered ranges. Each iteration emits
// one item or advances one range; at most N+M iterations are needed. This avoids
// rescanning all labels for every item and preserves native item order.
func (m *debugExportMetadata) labeledRows() WFTextTokenAttachment {
	b := m.b
	n := b.count(debugLogValueRef(debugLogVariableName))
	labelCount := b.count(debugLogValueRef(debugLabelVariableName))
	m.validateLabels(n)
	itemIndex, labelIndex := m.variable("item_index"), m.variable("label_index")
	needsLoad, first, last := m.variable("needs_load"), m.variable("first"), m.variable("last")
	cached, rowLabel, emit := m.variable("cached_label"), m.variable("row_label"), m.variable("emit")
	rows := m.variable("rows")
	for _, name := range []string{itemIndex, labelIndex, needsLoad} {
		b.setVariable(name, b.number(1))
	}
	for _, name := range []string{first, last, emit} {
		b.setVariable(name, b.number(0))
	}
	b.setVariable(cached, b.text(debugExportMetadataStatus("ABSENT")))
	b.setVariable(rowLabel, b.text(debugExportMetadataStatus("ABSENT")))
	b.add("nothing", map[string]any{})
	b.add("setvariable", map[string]any{"WFVariableName": rows})

	iterations := b.math(n, "+", debugDateFieldToken(labelCount.Value))
	group := strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
	b.add("repeat.count", backend.BuildRepeatCountStart(group, debugDateFieldToken(iterations.Value)))
	b.numberBranch(b.readVariable(itemIndex), 1, debugDateFieldToken(n.Value), func() {
		b.setVariable(emit, b.number(0))
		b.setVariable(rowLabel, b.text(debugExportMetadataStatus("ABSENT")))
		status := b.readVariable(m.variable("label_status"))
		b.numberBranch(status, 4, 0, func() {
			b.numberBranch(b.readVariable(labelIndex), 1, debugDateFieldToken(labelCount.Value), func() {
				b.numberBranch(b.readVariable(needsLoad), 4, 1, func() {
					metadata := b.listItem(debugLogValueRef(debugLabelVariableName), b.readVariable(labelIndex))
					b.setVariable(first, b.dictionaryField(metadata, "first"))
					b.setVariable(last, b.dictionaryField(metadata, "last"))
					encoded := b.base64(b.dictionaryField(metadata, "label"))
					label := b.produce("gettext", map[string]any{"WFTextActionText": debugExportTextToken(
						`{"status":"PRESENT","encoding":"base64-utf8","value":"`, encoded, `"}`)})
					b.setVariable(cached, label)
					b.setVariable(needsLoad, b.number(0))
				}, nil)
				b.numberBranch(b.readVariable(itemIndex), 2, debugDateFieldToken(b.readVariable(last).Value), func() {
					next := b.math(b.readVariable(labelIndex), "+", 1.0)
					b.setVariable(labelIndex, next)
					b.setVariable(needsLoad, b.number(1))
				}, func() {
					b.numberBranch(b.readVariable(itemIndex), 3, debugDateFieldToken(b.readVariable(first).Value), func() {
						b.setVariable(rowLabel, b.readVariable(cached))
					}, nil)
					b.setVariable(emit, b.number(1))
				})
			}, func() { b.setVariable(emit, b.number(1)) })
		}, func() {
			b.numberBranch(status, 4, 1, func() {
				b.setVariable(rowLabel, b.text(debugExportMetadataStatus("AMBIGUOUS")))
			}, func() {
				b.setVariable(rowLabel, b.text(debugExportMetadataStatus("UNSUPPORTED_LABEL_METADATA")))
			})
			b.setVariable(emit, b.number(1))
		})
		b.numberBranch(b.readVariable(emit), 4, 1, func() {
			item := b.listItem(debugLogValueRef(debugLogVariableName), b.readVariable(itemIndex))
			row := m.entry(item, b.readVariable(rowLabel))
			b.add("appendvariable", map[string]any{"WFVariableName": rows, "WFInput": row})
			next := b.math(b.readVariable(itemIndex), "+", 1.0)
			b.setVariable(itemIndex, next)
		}, nil)
	}, nil)
	b.add("nothing", map[string]any{})
	b.add("repeat.count", backend.BuildLoopEnd(group, ""))
	return b.readVariable(rows)
}

func buildDebugExportMetadataActions(options debugExportOptions, resultUUID, resultName string) []ShortcutAction {
	b := &debugExportExpansion{}
	m := &debugExportMetadata{b: b, options: options, prefix: "__cherri_debug_export_" + strings.ReplaceAll(strings.ToUpper(nextGeneratedUUID("debug-export-metadata")), "-", "") + "_"}
	if m.payloadEnabled() {
		m.textType = b.kind(b.text("Cherri export Text witness"))
		m.numberType = b.kind(b.number(0))
		var fields any = []WFDictionaryFieldValueItem{}
		dictionary := b.produce("dictionary", map[string]any{"WFItems": makeExplicitDictionaryValue(&fields)})
		m.dictionaryType = b.kind(dictionary)
	}
	if options.includeDates || m.payloadEnabled() {
		// This witness establishes native Date class, never a fabricated log date.
		m.dateType = b.kind(debugCurrentDateRef())
	}
	var rows WFTextTokenAttachment
	if options.includeLabels && !options.redact {
		rows = m.labeledRows()
	} else {
		group := strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
		b.add("repeat.each", backend.BuildRepeatEachStart(group, debugLogValueRef(debugLogVariableName)))
		item := b.readVariable("Repeat Item")
		var label any
		if options.includeLabels {
			label = debugExportMetadataStatus("REDACTED")
		}
		m.entry(item, label)
		rows = b.produce("repeat.each", backend.BuildLoopEnd(group, ""))
	}
	separator := ","
	if options.format == "text" {
		separator = "\n"
	}
	combined := b.produce("text.combine", map[string]any{
		"text": rows, "WFTextSeparator": "Custom", "WFTextCustomSeparator": separator,
	})
	policy, dates, labels := "redacted", "not-collected", "not-collected"
	if !options.redact {
		policy = "native-scalar-and-dictionary-diagnostics"
	} else if len(options.redactKeys) > 0 {
		policy = "selective-dictionary-diagnostics"
	}
	if options.includeDates {
		dates = "source-creation-date"
	}
	if options.includeLabels {
		labels = "redacted"
		if !options.redact {
			labels = "source-range-metadata"
		}
	}
	var boundaries = ""
	if m.payloadEnabled() {
		boundaries = `,"sourceTypePolicy":"source-types-not-reconstructed","collectionPolicy":"native-item-stream","nullPolicy":"absence-not-reconstructed"`
	}
	var envelope any
	if options.format == "json" {
		envelope = debugExportTextToken(
			`{"schemaVersion":"`+debugExportSchemaVersion+`","scope":"run-local-native-items","payloadPolicy":"`+policy+
				`","dates":"`+dates+`","labels":"`+labels+`"`+boundaries+`,"entries":[`, combined, "]}")
	} else {
		envelope = debugExportTextToken("Cherri Debug Export 1\nScope: run-local native items\nPayload policy: "+policy+
			"\nDates: "+dates+"\nLabels: "+labels+"\nEntry encoding: JSON lines; scalar presentations use base64-utf8\nSource types: source-types-not-reconstructed\nCollection policy: native-item-stream\n\n", combined)
	}
	content := b.produce("gettext", map[string]any{"WFTextActionText": envelope})
	encoded := b.base64(content)
	materialized := b.produce("base64encode", map[string]any{"WFInput": encoded, "WFEncodeMode": "Decode"})
	if resultUUID == "" {
		resultUUID = strings.ToUpper(nextGeneratedUUID("debug-export-metadata"))
	}
	extension := "json"
	if options.format == "text" {
		extension = "txt"
	}
	params := map[string]any{
		"WFInput": materialized, "WFName": "cherri-debug." + extension,
		"WFDontIncludeFileExtension": false, "UUID": resultUUID,
	}
	if resultName != "" {
		params["CustomOutputName"] = resultName
	}
	b.add("setitemname", params)
	return b.actions
}

func debugExportRequestsLabelStorage(name string, params map[string]any) bool {
	if name != "debugExport" {
		return false
	}
	options, err := parseDebugExportOptions(params)
	return err == nil && options.includeLabels && !options.redact
}
