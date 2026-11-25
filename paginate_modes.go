package main

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

func (b *paginateExpansion) requireMarkerTextPattern(value WFTextTokenAttachment, pattern, diagnostic string) {
	rest := b.produce("text.replace", map[string]any{
		"WFInput": value, "WFReplaceTextFind": pattern, "WFReplaceTextReplace": "",
		"WFReplaceTextCaseSensitive": true, "WFReplaceTextRegularExpression": true,
	})
	b.failIf(b.count(rest, "Characters"), 5, 0, backend.OperandNumber, diagnostic)
}

func (b *paginateExpansion) initializeMarkers() {
	empty := b.produce("list", map[string]any{"WFItems": []WFDictionaryFieldValueItem{}})
	b.set("seenMarkers", empty)
	if b.options.HasInitialMarker {
		marker := b.produce("gettext", map[string]any{"WFTextActionText": b.options.InitialMarker})
		b.append("seenMarkers", b.digest(marker))
	}
}

func (b *paginateExpansion) queryMarker(marker WFTextTokenAttachment, encode bool) {
	hash := b.digest(marker)
	seen := b.each(b.get("seenMarkers"))
	equal := b.begin(hash, 4, paginateText("", b.repeatItem()), backend.OperandString)
	b.set("active", b.zero)
	b.end(equal)
	b.eachEnd(seen)

	fresh := b.begin(b.get("active"), 4, 1, backend.OperandNumber)
	component := marker
	if encode {
		component = b.produce("urlencode", map[string]any{"WFInput": marker, "WFEncodeMode": "Encode"})
		// A query component must not introduce a separator, fragment or path.
		// Check the actual native encoder's output instead of assuming its
		// platform-specific allowed-character set is exactly the one we need.
		b.requireMarkerTextPattern(component, "^(?:[A-Za-z0-9._~-]|%[0-9A-Fa-f]{2})+$", "PAGINATE_CURSOR_ENCODING_INVALID")
		decoded := b.produce("urlencode", map[string]any{"WFInput": component, "WFEncodeMode": "Decode"})
		text := b.produce("gettext", map[string]any{"WFTextActionText": paginateText("", decoded)})
		b.failIf(b.digest(text), 5, paginateText("", hash), backend.OperandString, "PAGINATE_CURSOR_ENCODING_CHANGED")
	}
	resolved := EncodeTextSegmentsShared([]backend.TextSegment{
		{Text: b.options.QueryPrefix},
		{IsExpr: true, Reference: &backend.Reference{
			Kind: backend.RefActionResult, ProducerID: component.Value.OutputUUID, ProducerName: component.Value.OutputName,
		}},
		{Text: b.options.QuerySuffix},
	})
	url := b.produce("gettext", map[string]any{"WFTextActionText": resolved})
	b.set("url", url)
	b.append("seenMarkers", hash)
	b.end(fresh)
}

func (b *paginateExpansion) nextMarker(value, textType, numberType WFTextTokenAttachment) {
	if b.options.Mode == "cursor" {
		b.requireCategory(value, textType, "PAGINATE_NEXT_TEXT_REQUIRED")
		nonempty := b.begin(b.count(value, "Characters"), 2, 0, backend.OperandNumber)
		b.queryMarker(value, true)
		b.otherwise(nonempty)
		b.set("active", b.zero)
		b.end(nonempty)
		return
	}
	b.requireCategory(value, numberType, "PAGINATE_NEXT_NUMBER_REQUIRED")
	minimum := 0
	if b.options.Mode == "page" {
		minimum = 1
	}
	b.failIf(value, 0, minimum, backend.OperandNumber, "PAGINATE_NEXT_INTEGER_RANGE")
	b.failIf(value, 2, schema.PaginateMaxMarker, backend.OperandNumber, "PAGINATE_NEXT_INTEGER_RANGE")
	integer := b.produce("round", map[string]any{"WFInput": value, "WFRoundTo": "Ones Place", "WFRoundMode": "Normal"})
	b.failIf(value, 5, integer, backend.OperandNumber, "PAGINATE_NEXT_INTEGER_REQUIRED")
	text := b.produce("gettext", map[string]any{"WFTextActionText": paginateText("", value)})
	// Do not send a locale/grouping/exponent representation as a page marker.
	// The original native Number is checked before this textual URL boundary.
	b.requireMarkerTextPattern(text, "^(?:0|[1-9][0-9]{0,15})$", "PAGINATE_NEXT_INTEGER_TEXT_REQUIRED")
	b.queryMarker(text, false)
}
