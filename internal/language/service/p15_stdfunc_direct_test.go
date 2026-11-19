package service_test

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/modules"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func TestP15IM03DirectServiceStandardLibrary(t *testing.T) {
	t.Run("preserved_source_without_main", func(t *testing.T) {
		original, err := os.ReadFile(filepath.Join("..", "..", "..", "stdfunc.cherri"))
		if err != nil {
			t.Fatal(err)
		}
		if modules.StandardLibrarySource() != string(original) {
			t.Fatal("direct service package has no byte-identical preserved standard library source")
		}
	})
	cases := []struct {
		name, code string
		wantError  bool
	}{
		{"stdfunc_alias", "import stdfunc as std\nlet result = std.runJS(\"document.title\")\n", false},
		{"stdlib_alias", "import stdlib as std\nlet result = std.chooseFromVCard([\"One\", \"Two\"], prompt: \"Choose\")\n", false},
		{"legacy_bare_callable", "import stdfunc as std\nlet result = runJS(\"document.title\")\n", false},
		{"unknown_export_rejected", "import stdfunc as std\nlet result = std.missing()\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			directory := t.TempDir()
			entry := filepath.Join(directory, "main.cherri")
			if err := os.WriteFile(entry, []byte(tc.code), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.ToSlash(entry)
			if !strings.HasPrefix(path, "/") {
				path = "/" + path
			}
			uri := (&url.URL{Scheme: "file", Path: path}).String()
			svc := service.NewService(schema.DefaultRegistry())
			svc.OpenDocument(uri, 1, tc.code)
			diagnostics, _ := svc.Analyze(uri)
			errors := []analysis.Diagnostic{}
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					errors = append(errors, diagnostic)
				}
			}
			if tc.wantError {
				found := false
				for _, diagnostic := range errors {
					found = found || (diagnostic.Code == analysis.CodeUnknownAction && strings.Contains(diagnostic.Message, "does not export callable"))
				}
				if !found {
					t.Fatalf("missing actual module-export diagnostic: %v", errors)
				}
			} else if len(errors) != 0 {
				t.Fatalf("direct service could not use preserved stdfunc without compiler-main initialization: %v", errors)
			}
			entries, err := os.ReadDir(directory)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "main.cherri" {
				t.Fatal("standard library analysis installed local source or produced runtime artifacts")
			}
		})
	}
}
