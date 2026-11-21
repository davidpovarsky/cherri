package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"howett.net/plist"
)

const p15SchemaOptionalProbe = "package main\nimport (\n \"encoding/json\"\n \"os\"\n \"github.com/electrikmilk/cherri/internal/language/analysis\"\n \"github.com/electrikmilk/cherri/internal/language/schema\"\n \"github.com/electrikmilk/cherri/internal/language/service\"\n \"github.com/electrikmilk/cherri/internal/language/source\"\n \"github.com/electrikmilk/cherri/internal/language/syntax\"\n)\nfunc main() {\n reg:=schema.DefaultRegistry()\n action,_:=reg.LookupAction(\"quicklook\")\n param,has:=action.ParameterByLabel(\"rehearsalScale\")\n key,kind,def,optional:=\"\",\"\",\"\",false\n if has {if param.DefaultValue==nil {panic(\"changed optional declaration lost explicit zero default\")};key=param.WireKey;kind=param.Type.String();def=*param.DefaultValue;optional=param.Optional}\n diagnostics:=func(code string) []string {\n  parser:=syntax.NewParser(source.NewFile(\"probe\",\"file:///probe.cherri\",1,code))\n  program:=parser.ParseProgram()\n  out:=make([]string,0)\n  for _,d:=range parser.Errors(){out=append(out,d.String())}\n  analyzer:=analysis.NewAnalyzer(reg);analyzer.Analyze(program)\n  for _,d:=range analyzer.Diagnostics(){if d.Severity==analysis.SeverityError{out=append(out,d.Code+\": \"+d.Message)}}\n  return out\n }\n svc:=service.NewService(reg);uri:=\"file:///completion.cherri\";svc.OpenDocument(uri,1,\"quicklook(\\n\")\n found:=false\n for _,item:=range svc.Complete(uri,1,11){if item.Label==\"rehearsalScale\" && item.Kind==service.CompletionKindParameter{found=true}}\n json.NewEncoder(os.Stdout).Encode(map[string]any{\n  \"fingerprint\":reg.Fingerprint(),\"hasParameter\":has,\"wireKey\":key,\"type\":kind,\"optional\":optional,\"default\":def,\"completion\":found,\n  \"validDiagnostics\":diagnostics(\"quicklook(ShortcutInput, rehearsalScale: 0)\"),\n  \"wrongTypeDiagnostics\":diagnostics(\"quicklook(ShortcutInput, rehearsalScale: false)\"),\n  \"oldLabelDiagnostics\":diagnostics(\"quicklook(ShortcutInput, fullScreen: false)\"),\n })\n}\n"

func p15SchemaIsNumericZero(value any) bool {
	switch v := value.(type) {
	case uint64:
		return v == 0
	case int64:
		return v == 0
	case float64:
		return v == 0
	default:
		return false
	}
}

// RP06 / BRG23 exercise an actual inherited optional declaration in a detached
// worktree. The Number replacement is an intentional temporary mutation, not a
// claim that Apple accepts a Number for Quick Look's ordinary Bool parameter.
func TestP15SchemaOptionalDeclarationPropagation(t *testing.T) {
	repo := p15SchemaAcceptanceWorktree(t)
	actionPath := filepath.Join(repo, "actions/basic.cherri")
	original, err := os.ReadFile(actionPath)
	if err != nil {
		t.Fatal(err)
	}
	registryPath := filepath.Join(repo, "internal/language/schema/generated_registry.go")
	originalRegistry, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}
	probeDir := filepath.Join(repo, "p15schemaoptionalprobe")
	if err := os.Mkdir(probeDir, 0755); err != nil {
		t.Fatal(err)
	}
	p15SchemaWrite(t, filepath.Join(probeDir, "main.go"), []byte(p15SchemaOptionalProbe))
	readProbe := func() map[string]any {
		return p15SchemaJSON(t, p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./p15schemaoptionalprobe"))
	}
	readDocs := func(name string) []byte {
		target := filepath.Join(t.TempDir(), name)
		p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-docs", target)
		data, err := os.ReadFile(target)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	baseline := readProbe()
	beforeDocs := readDocs("before.md")
	if baseline["hasParameter"] != false || baseline["completion"] != false || len(baseline["validDiagnostics"].([]any)) == 0 ||
		len(baseline["oldLabelDiagnostics"].([]any)) != 0 || bytes.Contains(beforeDocs, []byte("rehearsalScale")) {
		t.Fatal("baseline unexpectedly sees the future optional parameter")
	}
	old := "bool ?fullScreen: 'WFQuickLookActionFullScreen' = false"
	changed := "number ?rehearsalScale: 'WFQuickLookActionFullScreen' = 0"
	if bytes.Count(original, []byte(old)) != 1 {
		t.Fatal("inherited optional declaration changed")
	}
	p15SchemaWrite(t, actionPath, []byte(strings.Replace(string(original), old, changed, 1)))
	catalog := p15SchemaJSON(t, p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", ".", "--actions-json"))
	action := p15SchemaAction(t, catalog, "quicklook")
	found := false
	for _, raw := range action["parameters"].([]any) {
		p := raw.(map[string]any)
		if p["name"] == "rehearsalScale" {
			found = true
			if p["type"] != "number" || p["key"] != "WFQuickLookActionFullScreen" || p["optional"] != true || p["default"] != "0" {
				t.Fatalf("live exporter lost the actual edit: %+v", p)
			}
		}
	}
	if !found {
		t.Fatal("actual exported definitions did not see the declaration edit")
	}
	if stale := readProbe(); !reflect.DeepEqual(stale, baseline) {
		t.Fatalf("stale generated registry control changed before generation: %+v", stale)
	}
	p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-schema")
	actual := readProbe()
	if actual["hasParameter"] != true || actual["wireKey"] != "WFQuickLookActionFullScreen" || actual["type"] != "Number" ||
		actual["optional"] != true || actual["default"] != "0" || actual["completion"] != true || len(actual["validDiagnostics"].([]any)) != 0 ||
		len(actual["wrongTypeDiagnostics"].([]any)) == 0 || len(actual["oldLabelDiagnostics"].([]any)) == 0 ||
		actual["fingerprint"] == baseline["fingerprint"] {
		t.Fatalf("actual regenerated schema/analyzer/completion did not propagate every changed fact: %+v", actual)
	}
	wrong, _ := json.Marshal(actual["wrongTypeDiagnostics"])
	if !bytes.Contains(wrong, []byte("Number")) {
		t.Fatalf("wrong-type control did not report the changed Number requirement: %s", wrong)
	}
	docs := readDocs("changed.md")
	expectedRow := "| " + string(rune(96)) + "rehearsalScale" + string(rune(96)) + " | " + string(rune(96)) + "Number" + string(rune(96)) + " | No | " + string(rune(96)) + "0" + string(rune(96)) + " | " + string(rune(96)) + "WFQuickLookActionFullScreen" + string(rune(96)) + " |"
	if !bytes.Contains(docs, []byte(expectedRow)) || !bytes.Contains(docs, []byte(actual["fingerprint"].(string))) {
		t.Fatal("actual documentation generator lost changed type, optionality, default, wire key or fingerprint")
	}
	compiler := filepath.Join(t.TempDir(), "current-compiler.exe")
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", compiler, ".")
	input, output := filepath.Join(t.TempDir(), "changed.cherri"), filepath.Join(t.TempDir(), "changed.shortcut")
	p15SchemaWrite(t, input, []byte("quicklook(ShortcutInput, rehearsalScale: 0)\n"))
	p15SchemaRun(t, repo, compiler, "compile", input, "--skip-sign", "--output="+output)
	native, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if _, err := plist.Unmarshal(native, &workflow); err != nil {
		t.Fatal(err)
	}
	actions, ok := workflow["WFWorkflowActions"].([]any)
	if !ok || len(actions) != 2 {
		t.Fatalf("actual CLI emitted the wrong action cardinality: %+v", workflow)
	}
	if err := p15M02MainVoidRowsError(actions); err != nil {
		t.Fatal(err)
	}
	nativeAction := actions[0].(map[string]any)
	params, ok := nativeAction["WFWorkflowActionParameters"].(map[string]any)
	if nativeAction["WFWorkflowActionIdentifier"] != "is.workflow.actions.previewdocument" || !ok {
		t.Fatal("actual compiler did not use the canonical changed action")
	}
	value, present := params["WFQuickLookActionFullScreen"]
	if !present || !p15SchemaIsNumericZero(value) {
		t.Fatalf("actual compiler lost numeric zero presence/type: %T %#v", value, value)
	}
	for _, bad := range []any{false, "0", "", nil, float64(1)} {
		if p15SchemaIsNumericZero(bad) {
			t.Fatalf("typed value oracle accepted %T %#v", bad, bad)
		}
	}
	p15SchemaWrite(t, input, []byte("quicklook(ShortcutInput, rehearsalScale: false)\n"))
	invalidOutput := filepath.Join(t.TempDir(), "invalid.shortcut")
	invalid, compileErr := p15SchemaCommand(t, repo, nil, compiler, "compile", input, "--skip-sign", "--output="+invalidOutput)
	if compileErr == nil || !bytes.Contains(invalid, []byte("Number")) {
		t.Fatalf("actual compiler accepted the wrong type: %v %s", compileErr, invalid)
	}
	if _, err := os.Stat(invalidOutput); !os.IsNotExist(err) {
		t.Fatal("rejected wrong type left a native artifact")
	}
	// Restore the declaration and use the real live generator to restore output.
	p15SchemaWrite(t, actionPath, original)
	p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-schema")
	restoredRegistry, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Equal(originalRegistry, restoredRegistry) || !reflect.DeepEqual(baseline, readProbe()) || !bytes.Equal(beforeDocs, readDocs("restored.md")) {
		t.Fatal("restored live generation did not restore the actual consumer outputs")
	}
	calendarPath := filepath.Join(repo, "actions/calendar.cherri")
	calendar, err := os.ReadFile(calendarPath)
	if err != nil {
		t.Fatal(err)
	}
	target := "action adjustDate(text date: 'WFDate',"
	if bytes.Count(calendar, []byte(target)) != 1 {
		t.Fatal("actual facet declaration changed")
	}
	p15SchemaWrite(t, calendarPath, []byte(strings.Replace(string(calendar), target, "action adjustDate(text rehearsalDate: 'WFDate',", 1)))
	invalid, genErr := p15SchemaCommand(t, repo, nil, "go", "run", "-buildvcs=false", "./tools/language-schema")
	if genErr == nil || !bytes.Contains(invalid, []byte("Facet validation failed:")) || !bytes.Contains(invalid, []byte("unknown primary parameter \"date\"")) {
		t.Fatalf("actual declaration edit silently retained a stale facet: %v %s", genErr, invalid)
	}
	preserved, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Equal(originalRegistry, preserved) {
		t.Fatal("failed facet generation overwrote the restored registry")
	}
	p15SchemaWrite(t, calendarPath, calendar)
	p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-schema", "--verify-generated")
	t.Log("Actual detached worktree declaration/export/generation/analyzer/completion/docs/CLI native-value pipeline passed, including stale-registry control, wrong-type no-artifact rejection, live restoration and invalid facet rejection.")
}
