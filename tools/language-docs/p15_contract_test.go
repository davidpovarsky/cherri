package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/schema"
)

func p15DocsSection(t *testing.T, text, name string) string {
	t.Helper()
	header := "### `" + name + "`\n"
	if strings.Count(text, header) != 1 {
		t.Fatalf("expected one section for %s, found %d", name, strings.Count(text, header))
	}
	section := strings.SplitN(text, header, 2)[1]
	if next := strings.Index(section, "\n### "); next >= 0 {
		section = section[:next]
	}
	if next := strings.Index(section, "\n## "); next >= 0 {
		section = section[:next]
	}
	return section
}

func TestP15D01PrimaryParameterContract(t *testing.T) {
	reg := schema.DefaultRegistry()
	var base *schema.ActionSchema
	for _, action := range reg.AllActions() {
		if action.CallableName == "selectPhotos" {
			base = action
			break
		}
	}
	if base == nil {
		t.Fatal("missing selectPhotos control")
	}
	if got := buildSignature(base); got != "selectPhotos(selectMultiple?: Bool = false, types?: Text)" {
		t.Fatalf("optional primary and its typed default disappeared: %s", got)
	}
	copyAction := *base
	copyAction.CallableName = "example"
	copyAction.OutputTypeName = "Void"
	copyAction.Parameters = append(base.Parameters[:0:0], base.Parameters[:1]...)
	copyAction.Parameters[0].ID = "value"
	copyAction.Parameters[0].Label = "value"
	copyAction.PrimaryParameterID = "value"
	for _, tc := range []struct{ kind, value, signature string }{
		{"Number", "0", "example(value?: Number = 0)"},
		{"Number", "-2.5", "example(value?: Number = -2.5)"},
		{"Bool", "false", "example(value?: Bool = false)"},
		{"Text", "", "example(value?: Text = \"\")"},
		{"Text", "two words", "example(value?: Text = \"two words\")"},
	} {
		copyAction.Parameters[0].TypeName = tc.kind
		copyAction.Parameters[0].DefaultValue = schema.StringDefault(tc.value)
		if got := buildSignature(&copyAction); got != tc.signature {
			t.Errorf("%s default %q rendered as %q, want %q", tc.kind, tc.value, got, tc.signature)
		}
	}
	copyAction.Parameters[0].DefaultValue = nil
	if got := buildSignature(&copyAction); got != "example(value?: Text)" {
		t.Fatalf("absent default rendered as present: %s", got)
	}
}

func TestP15D01AllGeneratedSignaturesAndCategories(t *testing.T) {
	reg := schema.DefaultRegistry()
	text := renderReference(reg)
	all := append(reg.AllActions()[:0:0], reg.AllActions()...)
	all = append(all, reg.CompilerMacros()...)
	wantMacros := []string{
		"cache", "debugAssert", "debugClearLogs", "debugExport", "debugExportTrace",
		"debugFinish", "debugLog", "debugLogPrevious", "debugShowLogs", "debugStartTimer",
		"debugStopTimer", "debugTrace", "debugTraceEvent", "measure", "paginate", "require",
	}
	var macroNames []string
	for _, macro := range reg.CompilerMacros() {
		macroNames = append(macroNames, macro.CallableName)
	}
	if len(reg.AllActions()) != 505 || strings.Join(macroNames, "\x00") != strings.Join(wantMacros, "\x00") {
		t.Fatalf("catalog/control inventory changed: %d native, macros=%v", len(reg.AllActions()), macroNames)
	}
	if strings.Count(text, "\n### `") != len(all) {
		t.Fatal("generated headings do not account for every native action and separate macro")
	}
	categories := map[string]int{}
	for _, action := range reg.AllActions() {
		category := action.Module
		if category == "" {
			category = action.Docs.Category
		}
		if category == "" {
			category = "general"
		}
		categories[category]++
	}
	for category, count := range categories {
		toc := fmt.Sprintf("- [%s](#module-%s) (%d actions)\n", strings.Title(category), strings.ToLower(category), count)
		if strings.Count(text, toc) != 1 || strings.Count(text, "## Module: "+strings.Title(category)+"\n") != 1 {
			t.Errorf("category/count mismatch for %s", category)
		}
	}
	for _, action := range all {
		section := p15DocsSection(t, text, action.CallableName)
		start := strings.Index(section, "```cherri\n")
		if start < 0 {
			t.Errorf("%s has no signature", action.CallableName)
			continue
		}
		signature := strings.SplitN(section[start+len("```cherri\n"):], "\n```", 2)[0]
		if !strings.HasPrefix(signature, action.CallableName+"(") {
			t.Errorf("%s signature identity mismatch: %s", action.CallableName, signature)
		}
		for _, parameter := range action.Parameters {
			re := regexp.MustCompile("(?:\\(|, )" + regexp.QuoteMeta(parameter.Label) + "(\\??): " + regexp.QuoteMeta(parameter.TypeName) + "(?:[ ,)=]|$)")
			matches := re.FindAllStringSubmatch(signature, -1)
			if len(matches) != 1 {
				t.Errorf("%s parameter %s/type %s missing or duplicated in %s", action.CallableName, parameter.Label, parameter.TypeName, signature)
				continue
			}
			if (matches[0][1] == "?") != parameter.Optional {
				t.Errorf("%s parameter %s optional=%t, signature=%s", action.CallableName, parameter.Label, parameter.Optional, signature)
			}
			if parameter.DefaultValue != nil {
				// Independently require that this parameter, including a primary,
				// carries a default before the following labelled parameter.
				reDefault := regexp.MustCompile(regexp.QuoteMeta(parameter.Label) + "\\??: " + regexp.QuoteMeta(parameter.TypeName) + " = ")
				if !reDefault.MatchString(signature) {
					t.Errorf("%s parameter %s loses schema default %q", action.CallableName, parameter.Label, *parameter.DefaultValue)
				}
			}
		}
		if action.OutputTypeName != "" && action.OutputTypeName != "Void" && action.OutputTypeName != "Unknown" {
			if !strings.HasSuffix(signature, " -> "+action.OutputTypeName) {
				t.Errorf("%s return type mismatch: %s", action.CallableName, signature)
			}
		}
	}
	// The checked-in reference must be the complete generator output, not just
	// a matching fingerprint header with stale individual signatures.
	path := filepath.Join("..", "..", "docs", "language-v2", "actions-reference.md")
	checked, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(checked) != text {
		t.Fatal("checked-in full reference differs from generated schema output")
	}
}

func TestP15D01CheckGateProducesAndRejectsFaults(t *testing.T) {
	dir := t.TempDir()
	binary := filepath.Join(dir, "language-docs.exe")
	build := exec.Command("go", "build", "-o", binary, ".")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build real generator: %v\n%s", err, out)
	}
	target := filepath.Join(dir, "reference.md")
	if out, err := exec.Command(binary, target).CombinedOutput(); err != nil {
		t.Fatalf("generate actual reference: %v\n%s", err, out)
	}
	healthy, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(binary, "--check", target).CombinedOutput(); err != nil {
		t.Fatalf("healthy complete reference rejected: %v\n%s", err, out)
	}
	mutations := []struct{ name, old, new string }{
		{"optional_primary", "selectPhotos(selectMultiple?: Bool", "selectPhotos(selectMultiple: Bool"},
		{"typed_default", "selectPhotos(selectMultiple?: Bool = false", "selectPhotos(selectMultiple?: Bool = true"},
		{"callable_identity", "### `text`", "### `otherText`"},
		{"wire_key", "`WFSelectMultiplePhotos`", "`WrongWireKey`"},
		{"return_type", " -> Text\n```", " -> Number\n```"},
		{"macro_identity", "### `debugLog`", "### `wrongDebugLog`"},
		{"missing_category", "## Module: General", "## Absent: General"},
		{"extra_section", "## Compiler macros", "### `duplicate`\n\n## Compiler macros"},
	}
	for _, mutation := range mutations {
		t.Run(mutation.name, func(t *testing.T) {
			changed := strings.Replace(string(healthy), mutation.old, mutation.new, 1)
			if changed == string(healthy) {
				t.Fatal("mutation did not change the real reference")
			}
			if err := os.WriteFile(target, []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command(binary, "--check", target).CombinedOutput()
			if err == nil || !strings.Contains(string(out), "differs from the complete assembled schema") {
				t.Fatalf("rendered drift was not rejected by real CLI: %v\n%s", err, out)
			}
			after, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != changed {
				t.Fatal("--check mutated the caller's reference")
			}
		})
	}
}
