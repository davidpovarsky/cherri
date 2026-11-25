package schema

import (
	"strings"
	"testing"
)

func TestPaginateMarkerSourceResolutionPreservesAuthoredQuery(t *testing.T) {
	cases := []struct {
		name, start, mode, key, prefix, suffix, initial string
		present                                         bool
	}{
		{"cursor_absent", "https://example.invalid/p?filter=kept", "cursor", "", "https://example.invalid/p?filter=kept&cursor=", "", "", false},
		{"cursor_initial", "https://example.invalid/p?cursor=Ab%2B%2f%3d&filter=kept", "cursor", "", "https://example.invalid/p?cursor=", "&filter=kept", "Ab+/=", true},
		{"cursor_empty", "https://example.invalid/p?cursor=&filter=kept", "cursor", "", "https://example.invalid/p?cursor=", "&filter=kept", "", false},
		{"key_percent_spelling", "https://example.invalid/p?%70age%5Bcursor%5D=x%2By&raw=%2f+%20", "cursor", "page[cursor]", "https://example.invalid/p?%70age%5Bcursor%5D=", "&raw=%2f+%20", "x+y", true},
		{"unicode_key", "https://example.invalid/p", "cursor", "café", "https://example.invalid/p?caf%C3%A9=", "", "", false},
		{"trailing_separator", "https://example.invalid/p?a=%2f+%20&", "cursor", "", "https://example.invalid/p?a=%2f+%20&cursor=", "", "", false},
		{"page_order", "https://example.invalid/p?z=%2f+%20&&page=1&a=last", "page", "", "https://example.invalid/p?z=%2f+%20&&page=", "&a=last", "1", true},
		{"offset_zero", "https://example.invalid/p?offset=0&x=a&x=b", "offset", "", "https://example.invalid/p?offset=", "&x=a&x=b", "0", true},
		{"numeric_maximum", "https://example.invalid/p?page=9007199254740991", "page", "", "https://example.invalid/p?page=", "", "9007199254740991", true},
		{"custom_page", "https://example.invalid/p?where=first&p=2&where=last", "page", "p", "https://example.invalid/p?where=first&p=", "&where=last", "2", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{"WFPaginateStartURL": tc.start, "WFPaginateMode": tc.mode}
			if tc.key != "" {
				params["WFPaginateQueryKey"] = tc.key
			}
			out, err := ResolvePaginateOptions(params)
			if err != nil {
				t.Fatal(err)
			}
			key := tc.key
			if key == "" {
				key = tc.mode
			}
			if out.StartURL != tc.start || out.Mode != tc.mode || out.QueryKey != key || out.QueryPrefix != tc.prefix ||
				out.QuerySuffix != tc.suffix || out.InitialMarker != tc.initial || out.HasInitialMarker != tc.present {
				t.Fatalf("authored query/state changed: %+v", out)
			}
			if out.Origin != "https://example.invalid" || out.MaxPages != 20 || out.MaxItems != 1000 ||
				out.ItemsKey != "items" || out.NextKey != "next" {
				t.Fatalf("existing traversal defaults changed: %+v", out)
			}
		})
	}
}

func TestPaginateMarkerInvalidSourceContractsFailClosed(t *testing.T) {
	cases := []struct {
		name, start string
		mode, key   any
	}{
		{"unknown_mode", "https://example.invalid/p", "link", nil},
		{"nonliteral_mode", "https://example.invalid/p", 1, nil},
		{"url_query_key", "https://example.invalid/p", "url", "cursor"},
		{"empty_key", "https://example.invalid/p", "cursor", ""},
		{"control_key", "https://example.invalid/p", "cursor", "x\n"},
		{"invalid_utf8_key", "https://example.invalid/p", "cursor", string([]byte{255})},
		{"oversized_key", "https://example.invalid/p", "cursor", strings.Repeat("x", 129)},
		{"nonliteral_key", "https://example.invalid/p", "cursor", 1},
		{"bad_escape", "https://example.invalid/p?cursor=%GG", "cursor", nil},
		{"bad_utf8_marker", "https://example.invalid/p?cursor=%FF", "cursor", nil},
		{"duplicate_cursor", "https://example.invalid/p?cursor=a&%63ursor=b", "cursor", nil},
		{"missing_page", "https://example.invalid/p", "page", nil},
		{"missing_offset", "https://example.invalid/p", "offset", nil},
		{"empty_page", "https://example.invalid/p?page=", "page", nil},
		{"zero_page", "https://example.invalid/p?page=0", "page", nil},
		{"negative_offset", "https://example.invalid/p?offset=-1", "offset", nil},
		{"leading_zero", "https://example.invalid/p?page=01", "page", nil},
		{"fractional_page", "https://example.invalid/p?page=1.5", "page", nil},
		{"exponent_page", "https://example.invalid/p?page=1e2", "page", nil},
		{"overflow_offset", "https://example.invalid/p?offset=9007199254740992", "offset", nil},
		{"duplicate_page", "https://example.invalid/p?page=1&%70age=2", "page", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			params := map[string]any{"WFPaginateStartURL": tc.start, "WFPaginateMode": tc.mode}
			if tc.key != nil {
				params["WFPaginateQueryKey"] = tc.key
			}
			if _, err := ResolvePaginateOptions(params); err == nil || !strings.HasPrefix(err.Error(), "PAGINATE_") {
				t.Fatalf("invalid marker contract was accepted or lost its fixed diagnostic: %v", err)
			}
		})
	}
}
