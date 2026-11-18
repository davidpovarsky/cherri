package main

// Bounded pagination composes existing native actions and shared control/text
// codecs. It does not implement an HTTP client, serializer, catch or retry layer.
import (
	"strconv"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/google/uuid"
)

func registerPaginateMacro() {
	actions["paginate"] = &actionDefinition{
		identifier: "getvariable", outputType: Arr,
		doc: selfDoc{
			title: "Paginate Bounded Dictionary Pages", category: "web",
			description: "Collect ordered native Dictionary items from bounded GET page bodies. Literal startURL/selectors/caps; same-origin absolute next URL or simple sibling filename. No HTTP status verification, headers, retry, transport timeout or trailing block. Native runtime acceptance remains required.",
		},
		parameters: []parameterDefinition{
			{name: "startURL", key: "WFPaginateStartURL", validType: RawString},
			{name: "itemsKey", key: "WFPaginateItemsKey", validType: RawString, optional: true, defaultValue: "items"},
			{name: "nextKey", key: "WFPaginateNextKey", validType: RawString, optional: true, defaultValue: "next"},
			{name: "maxPages", key: "WFPaginateMaxPages", validType: Integer, optional: true, defaultValue: 20},
			{name: "maxItems", key: "WFPaginateMaxItems", validType: Integer, optional: true, defaultValue: 1000},
		},
		makeParams: func(args []actionArgument) map[string]any {
			out := make(map[string]any)
			keys := []string{"WFPaginateStartURL", "WFPaginateItemsKey", "WFPaginateNextKey", "WFPaginateMaxPages", "WFPaginateMaxItems"}
			for index, key := range keys {
				if index >= len(args) {
					break
				}
				if args[index].valueType == Nil || args[index].value == nil {
					// Positional holes represent omitted optional arguments in the
					// canonical binder (also when V2 binds a later named option).
					if index == 0 {
						out[key] = nil
					}
				} else {
					out[key] = argumentValue(args, index)
				}
			}
			return out
		},
	}
}

type paginateExpansion struct {
	actions []ShortcutAction
	prefix  string
	options schema.PaginateOptions
	zero    WFTextTokenAttachment
	one     WFTextTokenAttachment
}

func (b *paginateExpansion) add(id string, params map[string]any) {
	b.actions = append(b.actions, ShortcutAction{
		WFWorkflowActionIdentifier: "is.workflow.actions." + id,
		WFWorkflowActionParameters: params,
	})
}

func (b *paginateExpansion) produce(id string, params map[string]any) WFTextTokenAttachment {
	output := strings.ToUpper(uuid.NewString())
	name := b.prefix + "_" + strconv.Itoa(len(b.actions))
	params["UUID"], params["CustomOutputName"] = output, name
	b.add(id, params)
	return debugOutputRefNamed(output, name)
}

func (b *paginateExpansion) get(name string) WFTextTokenAttachment {
	return b.produce("getvariable", map[string]any{"WFVariable": debugLogValueRef(b.prefix + "_" + name)})
}

func (b *paginateExpansion) set(name string, value any) {
	b.add("setvariable", map[string]any{"WFVariableName": b.prefix + "_" + name, "WFInput": value})
}

func (b *paginateExpansion) append(name string, value any) {
	b.add("appendvariable", map[string]any{"WFVariableName": b.prefix + "_" + name, "WFInput": value})
}

func paginateText(prefix string, value WFTextTokenAttachment) any {
	return EncodeTextSegmentsShared([]backend.TextSegment{
		{Text: prefix},
		{IsExpr: true, Reference: &backend.Reference{
			Kind: backend.RefActionResult, ProducerID: value.Value.OutputUUID, ProducerName: value.Value.OutputName,
		}},
	})
}

func (b *paginateExpansion) begin(input any, code int, value any, kind backend.ConditionOperandType) string {
	group := strings.ToUpper(uuid.NewString())
	b.add("conditional", backend.BuildIfStartTyped(group,
		map[string]any{"Type": "Variable", "Variable": input}, code, value, nil, kind, true))
	return group
}

func (b *paginateExpansion) end(group string) {
	b.add("conditional", backend.BuildIfEnd(group, ""))
}

func (b *paginateExpansion) otherwise(group string) {
	b.add("conditional", backend.BuildIfElse(group))
}

func (b *paginateExpansion) failIf(input any, code int, value any, kind backend.ConditionOperandType, diagnostic string) {
	group := b.begin(input, code, value, kind)
	b.add("alert", map[string]any{"WFAlertActionMessage": diagnostic, "WFAlertActionCancelButtonShown": false})
	b.add("exit", map[string]any{})
	b.end(group)
}

func (b *paginateExpansion) count(input any, kind string) WFTextTokenAttachment {
	return b.produce("count", map[string]any{"Input": input, "WFCountType": kind})
}

func (b *paginateExpansion) digest(input any) WFTextTokenAttachment {
	return b.produce("hash", map[string]any{"WFInput": input, "WFHashType": "SHA256"})
}

func (b *paginateExpansion) each(input any) string {
	group := strings.ToUpper(uuid.NewString())
	b.add("repeat.each", backend.BuildRepeatEachStart(group, input))
	return group
}

func (b *paginateExpansion) eachEnd(group string) {
	b.add("repeat.each", backend.BuildLoopEnd(group, ""))
}

func (b *paginateExpansion) repeatItem() WFTextTokenAttachment {
	// Capture Repeat Item before another loop can change the magic variable.
	return b.produce("getvariable", map[string]any{"WFVariable": debugLogValueRef("Repeat Item")})
}

func (b *paginateExpansion) requireCategory(input, witness WFTextTokenAttachment, diagnostic string) {
	b.failIf(b.count(input, "Items"), 5, 1, backend.OperandNumber, diagnostic)
	actual := b.produce("getitemtype", map[string]any{"WFInput": input})
	b.failIf(actual, 5, paginateText("", witness), backend.OperandString, diagnostic)
}

func (b *paginateExpansion) lookup(input WFTextTokenAttachment, key string) WFTextTokenAttachment {
	return b.produce("getvalueforkey", map[string]any{
		"WFInput": input, "WFDictionaryKey": key, "WFGetDictionaryValueType": "Value",
	})
}

func (b *paginateExpansion) nextURL(value WFTextTokenAttachment) {
	// Never include an authenticated URL, cursor or response body in an alert.
	for _, forbidden := range []string{" ", "\t", "\n", "\r", "\\", "#"} {
		b.failIf(value, 99, forbidden, backend.OperandString, "PAGINATE_NEXT_URL_INVALID")
	}
	absolute := b.begin(value, 8, b.options.Origin+"/", backend.OperandString)
	b.set("url", value)
	b.otherwise(absolute)
	// A sibling filename cannot change origin, escape the fixed directory or
	// introduce a query, percent-encoded path or fragment. Other cursor formats
	// are deliberately unsupported until their resolution contract is defined.
	for _, forbidden := range []string{":", "/", "?", "%"} {
		b.failIf(value, 99, forbidden, backend.OperandString, "PAGINATE_NEXT_URL_UNSUPPORTED")
	}
	for _, forbidden := range []string{".", ".."} {
		b.failIf(value, 4, forbidden, backend.OperandString, "PAGINATE_NEXT_URL_UNSUPPORTED")
	}
	resolved := b.produce("gettext", map[string]any{"WFTextActionText": paginateText(b.options.SiblingBaseURL, value)})
	b.set("url", resolved)
	b.end(absolute)
}

func (b *paginateExpansion) finish(resultUUID, resultName string) []ShortcutAction {
	if resultUUID == "" {
		resultUUID = strings.ToUpper(uuid.NewString())
	}
	if resultName == "" {
		resultName = "Paginated Items"
	}
	b.add("getvariable", map[string]any{
		"WFVariable": debugLogValueRef(b.prefix + "_results"), "UUID": resultUUID, "CustomOutputName": resultName,
	})
	return b.actions
}

func buildPaginateMacroActions(params map[string]any, resultUUID, resultName string) ([]ShortcutAction, error) {
	options, err := schema.ResolvePaginateOptions(params)
	if err != nil {
		return nil, err
	}
	b := &paginateExpansion{options: options, prefix: "__cherri_paginate_" + strings.ReplaceAll(uuid.NewString(), "-", "")}
	empty := b.produce("list", map[string]any{"WFItems": []WFDictionaryFieldValueItem{}})
	b.set("results", empty)
	if options.MaxPages == 0 || options.MaxItems == 0 {
		return b.finish(resultUUID, resultName), nil
	}
	emptySeen := b.produce("list", map[string]any{"WFItems": []WFDictionaryFieldValueItem{}})
	b.set("seen", emptySeen)
	b.zero = b.produce("number", map[string]any{"WFNumberActionNumber": "0"})
	b.one = b.produce("number", map[string]any{"WFNumberActionNumber": "1"})
	b.set("active", b.one)
	start := b.produce("gettext", map[string]any{"WFTextActionText": options.StartURL})
	b.set("url", start)

	var sample any = map[string]any{"cherri_paginate_type_witness": true}
	dictionary := b.produce("dictionary", map[string]any{"WFItems": makeExplicitDictionaryValue(&sample)})
	dictionaryType := b.produce("getitemtype", map[string]any{"WFInput": dictionary})
	textType := b.produce("getitemtype", map[string]any{"WFInput": start})
	itemsKey := b.produce("gettext", map[string]any{"WFTextActionText": options.ItemsKey})
	itemsHash := b.digest(itemsKey)
	nextKey := b.produce("gettext", map[string]any{"WFTextActionText": options.NextKey})
	nextHash := b.digest(nextKey)

	pages := strings.ToUpper(uuid.NewString())
	b.add("repeat.count", backend.BuildRepeatCountStart(pages, options.MaxPages))
	active := b.begin(b.get("active"), 4, 1, backend.OperandNumber)
	currentURL := b.get("url")
	currentHash := b.digest(currentURL)
	seen := b.each(b.get("seen"))
	equal := b.begin(currentHash, 4, paginateText("", b.repeatItem()), backend.OperandString)
	b.set("active", b.zero)
	b.end(equal)
	b.eachEnd(seen)

	unvisited := b.begin(b.get("active"), 4, 1, backend.OperandNumber)
	b.append("seen", currentHash)
	response := b.produce("downloadurl", map[string]any{"WFURL": paginateText("", currentURL), "WFHTTPMethod": "GET"})
	page := b.produce("detect.dictionary", map[string]any{"WFInput": response})
	b.requireCategory(page, dictionaryType, "PAGINATE_PAGE_DICTIONARY_REQUIRED")

	// Check actual keys before reading an optional next key. Hash comparisons
	// preserve key case even if native Text equality uses locale/case folding.
	keys := b.produce("getvalueforkey", map[string]any{"WFInput": page, "WFGetDictionaryValueType": "All Keys"})
	b.set("hasItems", b.zero)
	b.set("hasNext", b.zero)
	keyLoop := b.each(keys)
	keyHash := b.digest(b.repeatItem())
	for _, field := range []struct {
		name string
		hash WFTextTokenAttachment
	}{{"hasItems", itemsHash}, {"hasNext", nextHash}} {
		found := b.begin(keyHash, 4, paginateText("", field.hash), backend.OperandString)
		b.set(field.name, b.one)
		b.end(found)
	}
	b.eachEnd(keyLoop)
	b.failIf(b.get("hasItems"), 5, 1, backend.OperandNumber, "PAGINATE_ITEMS_KEY_REQUIRED")

	items := b.lookup(page, options.ItemsKey)
	itemCount := b.count(items, "Items")
	nonempty := b.begin(itemCount, 2, 0, backend.OperandNumber)
	limit := b.produce("number", map[string]any{"WFNumberActionNumber": strconv.Itoa(options.MaxItems)})
	b.set("pageLimit", limit)
	smaller := b.begin(itemCount, 0, options.MaxItems, backend.OperandNumber)
	b.set("pageLimit", itemCount)
	b.end(smaller)
	bounded := b.produce("getitemfromlist", map[string]any{
		"WFInput": items, "WFItemSpecifier": "Items in Range", "WFItemRangeStart": 1, "WFItemRangeEnd": b.get("pageLimit"),
	})
	itemLoop := b.each(bounded)
	room := b.begin(b.count(b.get("results"), "Items"), 0, options.MaxItems, backend.OperandNumber)
	item := b.repeatItem()
	b.requireCategory(item, dictionaryType, "PAGINATE_ITEM_DICTIONARY_REQUIRED")
	// Append the original native Dictionary reference, never Text/JSON or a
	// Dictionary-field wrapper that could coerce its nested values.
	b.append("results", item)
	b.end(room)
	b.eachEnd(itemLoop)
	b.end(nonempty)

	full := b.begin(b.count(b.get("results"), "Items"), 3, options.MaxItems, backend.OperandNumber)
	b.set("active", b.zero)
	b.end(full)
	needNext := b.begin(b.get("active"), 4, 1, backend.OperandNumber)
	hasNext := b.begin(b.get("hasNext"), 4, 1, backend.OperandNumber)
	next := b.lookup(page, options.NextKey)
	hasValue := b.begin(b.count(next, "Items"), 2, 0, backend.OperandNumber)
	b.requireCategory(next, textType, "PAGINATE_NEXT_TEXT_REQUIRED")
	nonemptyNext := b.begin(b.count(next, "Characters"), 2, 0, backend.OperandNumber)
	b.nextURL(next)
	b.otherwise(nonemptyNext)
	b.set("active", b.zero)
	b.end(nonemptyNext)
	b.otherwise(hasValue)
	b.set("active", b.zero)
	b.end(hasValue)
	b.otherwise(hasNext)
	b.set("active", b.zero)
	b.end(hasNext)
	b.end(needNext)
	b.end(unvisited)
	b.end(active)
	b.add("repeat.count", backend.BuildLoopEnd(pages, ""))
	return b.finish(resultUUID, resultName), nil
}
