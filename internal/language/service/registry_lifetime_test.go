package service

import (
	"fmt"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

func registryDoc(parameter, identifier string) string {
	return fmt.Sprintf("action url(%s: Text) -> Text {\nidentifier: %q\nprimary: %q\nbindings: {%s: {key: \"Value\", codec: \"text\"}}\n}\nurl(%s: \"LOCAL\")\n", parameter, identifier, parameter, parameter, parameter)
}

func checkRegistryDocument(t *testing.T, svc *Service, uri, text, parameter, id string) {
	t.Helper()
	diags, _ := svc.Analyze(uri)
	for _, diagnostic := range diags {
		if diagnostic.Severity == analysis.SeverityError {
			t.Errorf("%s semantic context: %v", uri, diagnostic)
		}
	}
	callLine := strings.Count(text[:strings.LastIndex(text, "url(")], "\n") + 1
	hover := svc.Hover(uri, callLine, 2)
	if hover == nil || !strings.Contains(hover.Contents, id) || !strings.Contains(hover.Contents, parameter) {
		t.Errorf("%s hover lost its own action: %#v", uri, hover)
	}
	signature := svc.SignatureHelp(uri, callLine, 5)
	if signature == nil || !strings.HasPrefix(signature.Label, "url("+parameter+":") {
		t.Errorf("%s signature used another document's parameter: %#v", uri, signature)
	}
	found := false
	for _, item := range svc.Complete(uri, callLine, 5) {
		if item.Kind == CompletionKindParameter && item.Label == parameter {
			found = true
		}
		if item.Kind == CompletionKindParameter && item.Label != parameter {
			t.Errorf("%s completion leaked parameter %q", uri, item.Label)
		}
	}
	if !found {
		t.Errorf("%s completion did not include its own parameter %q", uri, parameter)
	}
}

func TestServiceDocumentRegistryIsolation(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	fingerprint := registry.Fingerprint()
	svc := NewService(registry)
	first, second := registryDoc("value", "com.example.first"), registryDoc("other", "com.example.second")
	canonical := "url(url: [\"https://example.invalid/base\"])\n"
	svc.OpenDocument("first", 1, first)
	svc.OpenDocument("base", 1, canonical)
	svc.OpenDocument("second", 1, second)
	checkRegistryDocument(t, svc, "first", first, "value", "com.example.first")
	checkRegistryDocument(t, svc, "base", canonical, "url", "is.workflow.actions.url")
	checkRegistryDocument(t, svc, "second", second, "other", "com.example.second")
	action, _ := registry.LookupAction("url")
	if action.AppleIdentifier != "is.workflow.actions.url" || registry.Fingerprint() != fingerprint || svc.SchemaFingerprint() != fingerprint {
		t.Error("document declarations modified the provided/base catalog")
	}
	independent := NewService(registry)
	independent.OpenDocument("base", 1, canonical)
	checkRegistryDocument(t, independent, "base", canonical, "url", "is.workflow.actions.url")
}

func TestServiceRegistryRevisionAndClose(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	svc := NewService(registry)
	canonical := "url(url: [\"https://example.invalid/base\"])\n"
	custom := registryDoc("value", "com.example.local")
	svc.OpenDocument("edited", 1, custom)
	svc.ChangeDocument("edited", 2, canonical)
	checkRegistryDocument(t, svc, "edited", canonical, "url", "is.workflow.actions.url")
	svc.ChangeDocument("edited", 1, custom)
	checkRegistryDocument(t, svc, "edited", canonical, "url", "is.workflow.actions.url")
	svc.ChangeDocument("edited", 3, custom)
	checkRegistryDocument(t, svc, "edited", custom, "value", "com.example.local")
	svc.CloseDocument("edited")
	svc.OpenDocument("edited", 1, canonical)
	checkRegistryDocument(t, svc, "edited", canonical, "url", "is.workflow.actions.url")
}

func TestServiceVisualEditUsesSourceRegistry(t *testing.T) {
	registry := schema.NewRegistry(schema.DefaultRegistry().AllActions(), schema.DefaultRegistry().AllEnums())
	svc := NewService(registry)
	svc.OpenDocument("unrelated", 1, registryDoc("other", "com.example.unrelated"))
	own := registryDoc("value", "com.example.own")
	got, err := svc.ApplyVisualSourceEdit(own, 0, "Value", "\"EDITED\"")
	if err != nil || got != strings.Replace(own, "value: \"LOCAL\"", "value: \"EDITED\"", 1) {
		t.Errorf("visual edit read an unrelated document's registry: err=%v got=%q", err, got)
	}
	canonical := "url(url: [\"https://example.invalid/original\"])\n"
	got, err = svc.ApplyVisualSourceEdit(canonical, 0, "url", "[\"https://example.invalid/edited\"]")
	if err != nil || !strings.Contains(got, "url: [\"https://example.invalid/edited\"]") {
		t.Errorf("canonical visual edit used departed custom schema: err=%v got=%q", err, got)
	}
}
