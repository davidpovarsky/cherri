package main

import (
	"fmt"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func TestP15PaginateTraversalInvalidContractsBothFrontends(t *testing.T) {
	cases := []struct{ name, args, prefix string }{
		{"unknown_mode", `"https://example.invalid/p", "items", "next", 3, 6, {}, "link"`, ""},
		{"url_query_key", `"https://example.invalid/p", "items", "next", 3, 6, {}, "url", "cursor"`, ""},
		{"page_missing", `"https://example.invalid/p", "items", "next", 3, 6, {}, "page"`, ""},
		{"page_zero", `"https://example.invalid/p?page=0", "items", "next", 3, 6, {}, "page"`, ""},
		{"offset_negative", `"https://example.invalid/p?offset=-1", "items", "next", 3, 6, {}, "offset"`, ""},
		{"duplicate_cursor_alias", `"https://example.invalid/p?cursor=a&%63ursor=b", "items", "next", 3, 6, {}, "cursor"`, ""},
		{"invalid_utf8_cursor", `"https://example.invalid/p?cursor=%FF", "items", "next", 3, 6, {}, "cursor"`, ""},
		{"empty_query_key", `"https://example.invalid/p", "items", "next", 3, 6, {}, "cursor", ""`, ""},
		{"fractional_page", `"https://example.invalid/p?page=1.5", "items", "next", 3, 6, {}, "page"`, ""},
		{"overflow_offset", `"https://example.invalid/p?offset=9007199254740992", "items", "next", 3, 6, {}, "offset"`, ""},
		{"dynamic_mode", `"https://example.invalid/p", "items", "next", 3, 6, {}, marker`, "marker = text(\"cursor\")"},
		{"dynamic_query_key", `"https://example.invalid/p", "items", "next", 3, 6, {}, "cursor", marker`, "marker = text(\"cursor\")"},
	}
	for _, tc := range cases {
		for _, legacy := range []bool{false, true} {
			for _, profile := range []BuildProfile{BuildProfileDebug, BuildProfileRelease} {
				t.Run(fmt.Sprintf("%s/legacy=%t/%s", tc.name, legacy, profile), func(t *testing.T) {
					source := "paginate(" + tc.args + ")\n"
					if tc.prefix != "" {
						decl := "let"
						if legacy {
							decl = "const"
						}
						source = decl + " " + tc.prefix + "\n" + source
					}
					_, err := compileBuildProfileFixture(t, source, legacy, profile)
					if err == nil {
						t.Fatal("invalid/dynamic traversal contract accepted")
					}
					if !legacy {
						svc := service.NewService(schema.DefaultRegistry())
						svc.OpenDocument("file:///paginate-modes-invalid.cherri", 1, source)
						diagnostics, _ := svc.Analyze("file:///paginate-modes-invalid.cherri")
						hasError := false
						for _, d := range diagnostics {
							hasError = hasError || d.Severity == analysis.SeverityError
						}
						if !hasError {
							t.Fatal("editor advertised an invalid traversal contract")
						}
					}
				})
			}
		}
	}
}
