package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/service"
)

const p15T07Bounds = `{"platformVersions":{"ios":{"minVersion":"18.10","maxVersion":"18.12.2"},"macos":{"minVersion":"14.2","maxVersion":"15.10"},"ipados":{"minVersion":"19.1"}}}`

// The facts in these tests are explicitly synthetic. The existing canonical
// isCharging descriptor and native builder are retained; no Apple availability
// fact is invented or written into the generated catalog.
func p15T07Registry(t *testing.T, payload string) *schema.Registry {
	t.Helper()
	schema.ResetDefaultRegistry()
	base := schema.DefaultRegistry()
	action, ok := base.LookupAction("isCharging")
	if !ok {
		t.Fatal("missing canonical isCharging descriptor")
	}
	copy := *action
	if err := json.Unmarshal([]byte(payload), &copy); err != nil {
		t.Fatal(err)
	}
	actions := base.AllActions()
	for index, entry := range actions {
		if entry.CallableName == copy.CallableName {
			actions[index] = &copy
		}
	}
	return schema.NewRegistry(actions, base.AllEnums())
}

func p15T07Source(target, minimum, maximum string) string {
	fields := ""
	if target != "" {
		fields += "target: " + fmt.Sprintf("%q", target) + "\n"
	}
	if minimum != "" {
		fields += "minVersion: " + fmt.Sprintf("%q", minimum) + "\n"
	}
	if maximum != "" {
		fields += "maxVersion: " + fmt.Sprintf("%q", maximum) + "\n"
	}
	return "shortcut \"synthetic platform version\" {\n" + fields + "}\nisCharging()"
}

func p15T07Diagnostics(t *testing.T, registry *schema.Registry, source string) []analysis.Diagnostic {
	t.Helper()
	program, errors := p15ParseProgram(source)
	if len(errors) != 0 {
		t.Fatal(errors)
	}
	analyzer := analysis.NewAnalyzer(registry)
	analyzer.Analyze(program)
	return analyzer.Diagnostics()
}

func p15T07Decision(diagnostics []analysis.Diagnostic, source, code, platform string) error {
	found := 0
	start := strings.LastIndex(source, "isCharging()")
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError && diagnostic.Code != code {
			return fmt.Errorf("unexpected error: %+v", diagnostic)
		}
		if diagnostic.Code != code || code == "" {
			continue
		}
		found++
		if diagnostic.Span.Start.ByteOffset != start || diagnostic.Span.End.ByteOffset != start+len("isCharging()") {
			return fmt.Errorf("platform decision lost exact call span: %+v", diagnostic)
		}
		if platform != "" && !strings.Contains(strings.ToLower(diagnostic.Message), platform) {
			return fmt.Errorf("decision lost platform %s: %+v", platform, diagnostic)
		}
	}
	if (code == "" && found != 0) || (code != "" && found != 1) {
		return fmt.Errorf("want code %q exactly once when nonempty; got %+v", code, diagnostics)
	}
	if code == "" {
		for _, diagnostic := range diagnostics {
			if diagnostic.Code == "W_PLATFORM_VERSION_UNVERIFIED" {
				return fmt.Errorf("expected a verified synthetic bound, got %+v", diagnostic)
			}
		}
	}
	return nil
}

func TestP15T07PerPlatformVersionProduction(t *testing.T) {
	for _, tc := range []struct {
		name, target, minimum, maximum, code string
	}{
		{"minor_component_not_decimal", "ios", "18.2", "18.12.2", analysis.CodeUnsupportedTarget},
		{"18_1_is_not_18_10", "ios", "18.1", "18.12.2", analysis.CodeUnsupportedTarget},
		{"minimum_equal", "ios", "18.10", "18.12.2", ""},
		{"leading_zero_components", "ios", "18.010.0", "18.012.02", ""},
		{"patch_before_minimum", "ios", "18.9.999", "18.12.2", analysis.CodeUnsupportedTarget},
		{"patch_after_maximum", "ios", "18.10", "18.12.3", analysis.CodeUnsupportedTarget},
		{"new_major_after_maximum", "ios", "19.0", "19.0", analysis.CodeUnsupportedTarget},
		{"macos_independent_minimum", "macos", "14.2", "15.10", ""},
		{"mac_alias", "mac", "14.2", "15.10", ""},
		{"macos_18_is_not_ios_18", "macos", "18.10", "18.10", analysis.CodeUnsupportedTarget},
		{"ipados_not_ios", "ipados", "18.10", "", analysis.CodeUnsupportedTarget},
		{"ipados_own_bound", "ipados", "19.1", "", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := p15T07Registry(t, p15T07Bounds)
			t.Cleanup(schema.ResetDefaultRegistry)
			source := p15T07Source(tc.target, tc.minimum, tc.maximum)
			platform := tc.target
			if platform == "mac" {
				platform = "macos"
			}
			diagnostics := p15T07Diagnostics(t, registry, source)
			if err := p15T07Decision(diagnostics, source, tc.code, platform); err != nil {
				t.Fatal(err)
			}
			descriptor, _ := registry.LookupAction("isCharging")
			schema.DefaultRegistry().RegisterCustomAction(descriptor)
			raw, err := CompileSourceToPlist("p15-platform-version.cherri", source)
			if tc.code != "" {
				if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
					t.Fatalf("unsupported platform/version emitted bytes=%d err=%v", len(raw), err)
				}
				changed := append([]analysis.Diagnostic(nil), diagnostics...)
				for index := range changed {
					if changed[index].Code == tc.code {
						changed[index].Code = analysis.CodeUnknownName
					}
				}
				if p15T07Decision(changed, source, tc.code, platform) == nil {
					t.Fatal("unrelated diagnostic satisfied platform-version proof")
				}
				return
			}
			if err != nil || len(raw) == 0 {
				t.Fatalf("verified synthetic bound failed canonical emission: bytes=%d err=%v", len(raw), err)
			}
			root, err := decodeShortcutRawMap(raw)
			if err != nil {
				t.Fatal(err)
			}
			actions, _ := root["WFWorkflowActions"].([]any)
			if len(actions) != 1 {
				t.Fatalf("version checking changed native action count: %d", len(actions))
			}
			action := actions[0].(map[string]any)
			parameters := action["WFWorkflowActionParameters"].(map[string]any)
			if action["WFWorkflowActionIdentifier"] != "is.workflow.actions.getbatterylevel" || parameters["Subject"] != "Is Charging" {
				t.Fatalf("platform validation changed canonical action: %+v", action)
			}
		})
	}
}

func TestP15T07MissingPlatformIsUnverified(t *testing.T) {
	for _, tc := range []struct{ name, payload, target, minimum, code string }{
		{"ios_not_inferred_from_mac", `{"minVersion":99,"platformVersions":{"macos":{"minVersion":"14.2"}}}`, "ios", "18.10", "W_PLATFORM_VERSION_UNVERIFIED"},
		{"mac_not_inferred_from_ios", `{"platformVersions":{"ios":{"minVersion":"18.10"}}}`, "macos", "14.2", "W_PLATFORM_VERSION_UNVERIFIED"},
		{"empty_facet_unknown", `{"platformVersions":{"ios":{}}}`, "ios", "18.10", "W_PLATFORM_VERSION_UNVERIFIED"},
		{"target_missing", p15T07Bounds, "", "18.10", "W_PLATFORM_VERSION_UNVERIFIED"},
		{"version_missing", p15T07Bounds, "ios", "", "W_PLATFORM_VERSION_UNVERIFIED"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := p15T07Registry(t, tc.payload)
			source := p15T07Source(tc.target, tc.minimum, "")
			diagnostics := p15T07Diagnostics(t, registry, source)
			if err := p15T07Decision(diagnostics, source, tc.code, tc.target); err != nil {
				t.Fatal(err)
			}
			// Missing facts cannot turn into an UnsupportedTarget error or an
			// inferred successful availability guarantee.
			for _, diagnostic := range diagnostics {
				if diagnostic.Severity == analysis.SeverityError {
					t.Fatal(diagnostic)
				}
			}
		})
	}
}

func TestP15T07MalformedPlatformMetadataFailsClosed(t *testing.T) {
	for _, payload := range []string{
		`{"platformVersions":{"ios":{"minVersion":"18..10"}}}`,
		`{"platformVersions":{"ios":{"minVersion":"18.10junk"}}}`,
		`{"platformVersions":{"ios":{"minVersion":" 18.10"}}}`,
		`{"platformVersions":{"ios":{"minVersion":"18.10 "}}}`,
		`{"platformVersions":{"ios":{"maxVersion":"18.1e2"}}}`,
		`{"platformVersions":{"ios":{"minVersion":"999999999999999999999999999999999"}}}`,
		`{"platformVersions":{"ios":{"minVersion":"18.10","maxVersion":"18.2"}}}`,
		`{"platformVersions":{"macOS":{"minVersion":"14.2"}}}`,
		`{"platformVersions":{"":{"minVersion":"18.10"}}}`,
	} {
		t.Run(payload, func(t *testing.T) {
			registry := p15T07Registry(t, payload)
			source := p15T07Source("ios", "18.10", "18.12.2")
			diagnostics := p15T07Diagnostics(t, registry, source)
			if err := p15T07Decision(diagnostics, source, "E_PLATFORM_VERSION_METADATA", ""); err != nil {
				t.Fatal(err)
			}
			svc := service.NewService(registry)
			uri := "file:///p15-platform-metadata.cherri"
			svc.OpenDocument(uri, 1, source)
			diagnostics, _ = svc.Analyze(uri)
			if err := p15T07Decision(diagnostics, source, "E_PLATFORM_VERSION_METADATA", ""); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestP15T07PlatformVersionService(t *testing.T) {
	registry := p15T07Registry(t, p15T07Bounds)
	svc := service.NewService(registry)
	uri := "file:///p15-platform-version-service.cherri"
	for index, tc := range []struct{ target, minimum, maximum, code string }{
		{"ios", "18.2", "18.12.2", analysis.CodeUnsupportedTarget},
		{"ios", "18.10", "18.12.2", ""},
		{"macos", "14.2", "15.10", ""},
		{"ipados", "18.10", "", analysis.CodeUnsupportedTarget},
		{"watchos", "18.10", "", "W_PLATFORM_VERSION_UNVERIFIED"},
	} {
		source := p15T07Source(tc.target, tc.minimum, tc.maximum)
		if index == 0 {
			svc.OpenDocument(uri, 1, source)
		} else {
			svc.ChangeDocument(uri, index+1, source)
		}
		diagnostics, fingerprint := svc.Analyze(uri)
		if fingerprint != registry.Fingerprint() {
			t.Fatal("service detached platform metadata fingerprint")
		}
		if err := p15T07Decision(diagnostics, source, tc.code, tc.target); err != nil {
			t.Fatal(err)
		}
	}
	// Two documents must retain their independent platform selections.
	other := "file:///p15-platform-version-other.cherri"
	svc.OpenDocument(other, 1, p15T07Source("macos", "14.2", "15.10"))
	source := p15T07Source("watchos", "18.10", "")
	diagnostics, _ := svc.Analyze(uri)
	if err := p15T07Decision(diagnostics, source, "W_PLATFORM_VERSION_UNVERIFIED", "watchos"); err != nil {
		t.Fatal(err)
	}
}

func TestP15T07PlatformVersionSchemaIdentity(t *testing.T) {
	base := p15T07Registry(t, `{}`)
	empty := p15T07Registry(t, `{"platformVersions":{}}`)
	if base.Fingerprint() != empty.Fingerprint() || len(base.AllActions()) != 505 {
		t.Fatal("absent/empty optional facet changed the existing catalog")
	}
	a := p15T07Registry(t, `{"platformVersions":{"ios":{"minVersion":"18.10"},"macos":{"minVersion":"14.2"}}}`)
	b := p15T07Registry(t, `{"platformVersions":{"macos":{"minVersion":"14.2"},"ios":{"minVersion":"18.10"}}}`)
	changedBound := p15T07Registry(t, `{"platformVersions":{"ios":{"minVersion":"18.1"},"macos":{"minVersion":"14.2"}}}`)
	changedPlatform := p15T07Registry(t, `{"platformVersions":{"ipados":{"minVersion":"18.10"},"macos":{"minVersion":"14.2"}}}`)
	if a.Fingerprint() != b.Fingerprint() || a.Fingerprint() == base.Fingerprint() || a.Fingerprint() == changedBound.Fingerprint() || a.Fingerprint() == changedPlatform.Fingerprint() {
		t.Fatal("platform facet fingerprint is missing, unordered, rounded, or detached from platform identity")
	}
	action, _ := a.LookupAction("isCharging")
	raw, err := json.Marshal(action)
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	facets, ok := decoded["platformVersions"].(map[string]any)
	if !ok || facets["ios"].(map[string]any)["minVersion"] != "18.10" {
		t.Fatalf("canonical JSON lost exact component strings: %s", raw)
	}
	clone := a.Clone()
	clonedAction, _ := clone.LookupAction("isCharging")
	if err := json.Unmarshal([]byte(`{"platformVersions":{"ios":{"minVersion":"99"}}}`), clonedAction); err != nil {
		t.Fatal(err)
	}
	originalRaw, _ := json.Marshal(action)
	if string(originalRaw) != string(raw) {
		t.Fatal("clone platform map aliases the source catalog")
	}
}

// Exercise the original definition -> --actions-json catalog -> production schema
// generator path. These availability bounds are synthetic and restored at exit.
func TestP15T07CanonicalCatalogPropagation(t *testing.T) {
	loadTestStandardActions()
	original := actions["isCharging"]
	if original == nil {
		t.Fatal("missing original canonical action")
	}
	copy := *original
	want := map[string]schema.PlatformVersionBounds{
		"ios":   {MinVersion: "18.10", MaxVersion: "18.12.2"},
		"macos": {MinVersion: "14.2", MaxVersion: "15.10"},
	}
	copy.platformVersions = maps.Clone(want)
	actions["isCharging"] = &copy
	t.Cleanup(func() { actions["isCharging"] = original })

	catalog := buildActionCatalog()
	found := false
	for _, entry := range catalog {
		if entry.Name != "isCharging" {
			continue
		}
		found = true
		if !reflect.DeepEqual(entry.PlatformVersions, want) {
			t.Fatalf("original definition lost in machine catalog: %+v", entry.PlatformVersions)
		}
		entry.PlatformVersions["ios"] = schema.PlatformVersionBounds{MinVersion: "99"}
		if !reflect.DeepEqual(copy.platformVersions, want) {
			t.Fatal("machine catalog aliases original definition platform facts")
		}
	}
	if !found {
		t.Fatal("canonical action disappeared from catalog")
	}

	temp := t.TempDir()
	input, output := filepath.Join(temp, "catalog.json"), filepath.Join(temp, "generated.go")
	generator := filepath.Join(temp, "generator.exe")
	writeCatalog := func() {
		t.Helper()
		raw, err := json.Marshal(struct {
			Actions []catalogActionInfo `json:"actions"`
		}{buildActionCatalog()})
		if err != nil {
			t.Fatal(err)
		}
		p15SchemaWrite(t, input, raw)
	}
	writeCatalog()
	p15SchemaRun(t, ".", "go", "build", "-buildvcs=false", "-o", generator, "./tools/language-schema")
	p15SchemaRun(t, ".", generator, input, output)
	raw, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if got := p15T07GeneratedBounds(t, raw, "isCharging"); !reflect.DeepEqual(got, want) {
		t.Fatalf("generator dropped, rounded, or moved original platform facts: got=%+v want=%+v", got, want)
	}
	second := filepath.Join(temp, "second.go")
	p15SchemaRun(t, ".", generator, input, second)
	repeated, err := os.ReadFile(second)
	if err != nil || !bytes.Equal(raw, repeated) {
		t.Fatalf("identical platform facts generated different source: %v", err)
	}

	// The production generator must reject malformed upstream facts before
	// publishing a file, rather than dropping them or inventing a valid bound.
	copy.platformVersions["ios"] = schema.PlatformVersionBounds{MinVersion: "18..10"}
	writeCatalog()
	rejected := filepath.Join(temp, "rejected.go")
	message, err := p15SchemaCommand(t, ".", nil, generator, input, rejected)
	if err == nil || !strings.Contains(string(message), "Facet validation failed") || !strings.Contains(string(message), "invalid minimum") {
		t.Fatalf("invalid original metadata was accepted by generator: err=%v output=%s", err, message)
	}
	if _, err := os.Stat(rejected); !os.IsNotExist(err) {
		t.Fatalf("invalid upstream metadata published a schema file: %v", err)
	}
}

func p15T07GeneratedBounds(t *testing.T, raw []byte, name string) map[string]schema.PlatformVersionBounds {
	t.Helper()
	file, err := goparser.ParseFile(gotoken.NewFileSet(), "generated.go", raw, 0)
	if err != nil {
		t.Fatalf("production generator emitted invalid Go syntax: %v", err)
	}
	stringValue := func(expression goast.Expr) string {
		t.Helper()
		literal, ok := expression.(*goast.BasicLit)
		if !ok || literal.Kind != gotoken.STRING {
			t.Fatal("generated version component or platform is not an exact string literal")
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	var got map[string]schema.PlatformVersionBounds
	matches := 0
	goast.Inspect(file, func(node goast.Node) bool {
		literal, ok := node.(*goast.CompositeLit)
		if !ok {
			return true
		}
		fields := make(map[string]goast.Expr)
		for _, item := range literal.Elts {
			if field, ok := item.(*goast.KeyValueExpr); ok {
				if key, ok := field.Key.(*goast.Ident); ok {
					fields[key.Name] = field.Value
				}
			}
		}
		if fields["CallableName"] == nil || stringValue(fields["CallableName"]) != name {
			return true
		}
		matches++
		facets, ok := fields["PlatformVersions"].(*goast.CompositeLit)
		if !ok {
			t.Fatal("generated canonical action has no platform facet map")
		}
		got = make(map[string]schema.PlatformVersionBounds)
		for _, item := range facets.Elts {
			entry, ok := item.(*goast.KeyValueExpr)
			if !ok {
				t.Fatal("generated platform facet is not keyed")
			}
			platform := stringValue(entry.Key)
			if _, duplicate := got[platform]; duplicate {
				t.Fatalf("generated duplicate platform %q", platform)
			}
			bounds, ok := entry.Value.(*goast.CompositeLit)
			if !ok {
				t.Fatal("generated platform bound is not a field initializer")
			}
			var value schema.PlatformVersionBounds
			for _, item := range bounds.Elts {
				field, ok := item.(*goast.KeyValueExpr)
				if !ok {
					t.Fatal("generated bound field is not keyed")
				}
				key, ok := field.Key.(*goast.Ident)
				if !ok {
					t.Fatal("generated bound field has no name")
				}
				switch key.Name {
				case "MinVersion":
					value.MinVersion = stringValue(field.Value)
				case "MaxVersion":
					value.MaxVersion = stringValue(field.Value)
				default:
					t.Fatalf("unexpected generated platform bound field %q", key.Name)
				}
			}
			got[platform] = value
		}
		return false
	})
	if matches != 1 {
		t.Fatalf("wanted one generated canonical descriptor %q, found %d", name, matches)
	}
	return got
}
