package main

import (
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// Item selection shares the original Dictionary/page traversal. Values leave
// this boundary only as their original native reference; it is not a JSON
// serializer or a conversion from private List/Optional/JsonNull cells.
func (b *paginateExpansion) itemTypeWitness(dictionary, dictionaryType, textType, numberType WFTextTokenAttachment) WFTextTokenAttachment {
	switch b.options.ItemType {
	case "text":
		return textType
	case "number", "bool":
		var diagnostic = "PAGINATE_" + strings.ToUpper(b.options.ItemType) + "_TYPE_UNAVAILABLE"
		var value = b.lookup(dictionary, "cherri_paginate_type_witness")
		b.failIf(b.count(value, "Items"), 5, 1, backend.OperandNumber, diagnostic)
		var witness = b.produce("getitemtype", map[string]any{"WFInput": value})
		// If Boolean and Number share one native category, neither typed
		// selection can distinguish JSON false/true from numeric 0/1.
		// Fail before any request rather than grant a false source type.
		b.failIf(witness, 4, paginateText("", numberType), backend.OperandString, diagnostic)
		if b.options.ItemType == "number" {
			return numberType
		}
		return witness
	default:
		return dictionaryType
	}
}

func (b *paginateExpansion) selectItem(item, dictionaryType, keyHash WFTextTokenAttachment) WFTextTokenAttachment {
	if b.options.ItemKey == "" {
		return item
	}
	b.requireCategory(item, dictionaryType, "PAGINATE_ITEM_DICTIONARY_REQUIRED")
	b.set("hasSelectedKey", b.zero)
	var keys = b.produce("getvalueforkey", map[string]any{"WFInput": item, "WFGetDictionaryValueType": "All Keys"})
	var group = b.each(keys)
	var found = b.begin(b.digest(b.repeatItem()), 4, paginateText("", keyHash), backend.OperandString)
	b.set("hasSelectedKey", b.one)
	b.end(found)
	b.eachEnd(group)
	b.failIf(b.get("hasSelectedKey"), 5, 1, backend.OperandNumber, "PAGINATE_ITEM_KEY_REQUIRED")
	return b.lookup(item, b.options.ItemKey)
}

func (b *paginateExpansion) requireSelectedItem(item, witness WFTextTokenAttachment) {
	var diagnostic = "PAGINATE_ITEM_" + strings.ToUpper(b.options.ItemType) + "_REQUIRED"
	b.requireCategory(item, witness, diagnostic)
}
