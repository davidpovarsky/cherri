package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"time"
)

// A private replica prevents generation rehearsals from changing sources while
// other packages are being tested. Only tracked repository files are copied;
// the private corpus and unrelated worktrees are never traversed.
func p15SchemaReplica(t *testing.T) string {
	t.Helper()
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	paths, err := cmd.Output()
	if err != nil {
		t.Fatalf("tracked source inventory: %v", err)
	}
	replica := filepath.Join(t.TempDir(), "replica")
	for _, raw := range bytes.Split(paths, []byte{0}) {
		if len(raw) == 0 {
			continue
		}
		rel := filepath.FromSlash(string(raw))
		if filepath.IsAbs(rel) || strings.HasPrefix(filepath.Clean(rel), "..") {
			t.Fatal("unsafe tracked source path")
		}
		source := filepath.Join(root, rel)
		info, err := os.Lstat(source)
		if err != nil || !info.Mode().IsRegular() {
			t.Fatalf("tracked source is not a regular file: %s", rel)
		}
		data, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		destination := filepath.Join(replica, rel)
		if err := os.MkdirAll(filepath.Dir(destination), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, data, info.Mode().Perm()); err != nil {
			t.Fatal(err)
		}
	}
	return replica
}

func p15SchemaCommand(t *testing.T, dir string, extraEnv []string, command ...string) ([]byte, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	// These rehearsals rebuild real source in fresh, isolated directories.
	// Trim only filesystem paths from build identities so unchanged packages
	// can share Go's content-addressed compilation cache across those roots.
	// Source/dependency edits still invalidate that cache; test callers keep
	// -count=1 and every real child process and mutation assertion remains.
	if len(command) > 1 && command[0] == "go" {
		switch command[1] {
		case "build", "run", "test":
			args := make([]string, 0, len(command)+1)
			args = append(args, command[:2]...)
			args = append(args, "-trimpath")
			command = append(args, command[2:]...)
		}
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = dir
	for _, item := range os.Environ() {
		if !strings.HasPrefix(item, "CHERRI_LIVE_CATALOG_CMD=") {
			cmd.Env = append(cmd.Env, item)
		}
	}
	cmd.Env = append(cmd.Env, extraEnv...)
	out, err := p15PhaseCombinedOutput(t, "actual_schema_command", cmd)
	if err != nil {
		return out, fmt.Errorf("actual schema subprocess %q (context=%v): %w", command, ctx.Err(), err)
	}
	return out, nil
}

func p15SchemaRun(t *testing.T, dir string, command ...string) []byte {
	t.Helper()
	out, err := p15SchemaCommand(t, dir, nil, command...)
	if err != nil {
		t.Fatalf("schema production command %s: %v\n%s", command[0], err, out)
	}
	return out
}

func p15SchemaJSON(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var value map[string]any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		t.Fatalf("invalid actual CLI JSON: %v", err)
	}
	return value
}

func p15SchemaWrite(t *testing.T, name string, data []byte) {
	t.Helper()
	if err := os.WriteFile(name, data, 0644); err != nil {
		t.Fatal(err)
	}
}

func TestP15U02GeneratedSchemaProduction(t *testing.T) {
	repo := p15SchemaReplica(t)
	compiler, generator := filepath.Join(t.TempDir(), "compiler.exe"), filepath.Join(t.TempDir(), "schema-generator.exe")
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", compiler, ".")
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", generator, "./tools/language-schema")
	firstCatalog := p15SchemaRun(t, repo, compiler, "--actions-json")
	secondCatalog := p15SchemaRun(t, repo, compiler, "--actions-json")
	if !bytes.Equal(firstCatalog, secondCatalog) {
		t.Fatal("identical declaration inputs emitted different catalogs")
	}
	catalog := p15SchemaJSON(t, firstCatalog)
	actions, ok := catalog["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatal("live catalog contains no action definitions")
	}
	firstCapabilities := p15SchemaJSON(t, p15SchemaRun(t, repo, compiler, "--capabilities-json"))
	secondCapabilities := p15SchemaJSON(t, p15SchemaRun(t, repo, compiler, "--capabilities-json"))
	fingerprint, ok := firstCapabilities["schemaFingerprint"].(string)
	if !ok || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(fingerprint) || secondCapabilities["schemaFingerprint"] != fingerprint {
		t.Fatal("actual compiler semantic fingerprint is invalid or nondeterministic")
	}
	input := filepath.Join(t.TempDir(), "live-catalog.json")
	p15SchemaWrite(t, input, firstCatalog)
	first, second := filepath.Join(t.TempDir(), "first.go"), filepath.Join(t.TempDir(), "second.go")
	p15SchemaRun(t, repo, generator, input, first)
	p15SchemaRun(t, repo, generator, input, second)
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	checked, err := os.ReadFile(filepath.Join(repo, "internal/language/schema/generated_registry.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(a, b) || !bytes.Equal(a, checked) {
		t.Fatal("actual regeneration is nondeterministic or differs from the compiler's checked-in registry")
	}
	p15SchemaRun(t, repo, generator, input, first, "--verify-generated")
	// Mutate a real canonical wire key. The same verifier must now reject the
	// formerly healthy generated file, not silently consult a baseline catalog.
	mutated := false
	for _, raw := range actions {
		action := raw.(map[string]any)
		if action["name"] != "text" {
			continue
		}
		parameters := action["parameters"].([]any)
		if len(parameters) == 0 {
			t.Fatal("text has no canonical parameter")
		}
		parameters[0].(map[string]any)["key"] = "WF_P15_REHEARSAL_DIFFERENT_KEY"
		mutated = true
	}
	if !mutated {
		t.Fatal("causal action mutation was not applied")
	}
	changed, err := json.Marshal(catalog)
	if err != nil {
		t.Fatal(err)
	}
	p15SchemaWrite(t, input, changed)
	out, err := p15SchemaCommand(t, repo, nil, generator, input, first, "--verify-generated")
	if err == nil || !bytes.Contains(out, []byte("Drift detected!")) {
		t.Fatalf("changed canonical input did not reject the stale generated registry: %v\n%s", err, out)
	}
	out, err = p15SchemaCommand(t, repo, []string{"CHERRI_LIVE_CATALOG_CMD=go version-nonexistent-flag"}, generator, "--verify-generated")
	if err == nil || !bytes.Contains(out, []byte("fail-closed, no baseline fallback")) {
		t.Fatalf("failed live extraction used an unverified fallback: %v\n%s", err, out)
	}
	p15SchemaWrite(t, input, firstCatalog)
	p15SchemaRun(t, repo, generator, input, first, "--verify-generated")
	t.Logf("Actual declaration catalog=%d; two fresh generations match checked-in bytes; semantic fingerprint=%s; changed-key and failed-extraction controls rejected; restored control passed", len(actions), fingerprint)
}

const p15GeneratedRegistryProbe = `package main
import (
 "encoding/json"
 "os"
 "github.com/electrikmilk/cherri/internal/language/analysis"
 "github.com/electrikmilk/cherri/internal/language/schema"
 "github.com/electrikmilk/cherri/internal/language/service"
 "github.com/electrikmilk/cherri/internal/language/source"
 "github.com/electrikmilk/cherri/internal/language/syntax"
)
func main() {
 reg:=schema.DefaultRegistry()
 action,_:=reg.LookupAction("openURL")
 param,has:=action.ParameterByLabel("rehearsalDryRun")
 key,kind:="",""
 if has { key=param.WireKey;kind=param.Type.String() }
 errors:=func(code string) int {
  parser:=syntax.NewParser(source.NewFile("probe","file:///probe.cherri",1,code))
  program:=parser.ParseProgram();n:=len(parser.Errors())
  a:=analysis.NewAnalyzer(reg);a.Analyze(program)
  for _,d:=range a.Diagnostics(){if d.Severity==analysis.SeverityError{n++}}
  return n
 }
 svc:=service.NewService(reg);uri:="file:///completion.cherri";svc.OpenDocument(uri,1,"openURL(\n")
 found:=false
 for _,item:=range svc.Complete(uri,1,9){if item.Label=="rehearsalDryRun" && item.Kind==service.CompletionKindParameter{found=true}}
 json.NewEncoder(os.Stdout).Encode(map[string]any{"fingerprint":reg.Fingerprint(),"hasParameter":has,"wireKey":key,"type":kind,"completion":found,
 "validErrors":errors("openURL(url: \"https://example.invalid\", rehearsalDryRun: true)"),
 "wrongTypeErrors":errors("openURL(url: \"https://example.invalid\", rehearsalDryRun: \"not-bool\")")})
}
`

func p15UpstreamProductionRehearsal(t *testing.T) {
	repo := p15SchemaReplica(t)
	probe := filepath.Join(repo, "p15generatedprobe")
	if err := os.Mkdir(probe, 0755); err != nil {
		t.Fatal(err)
	}
	p15SchemaWrite(t, filepath.Join(probe, "main.go"), []byte(p15GeneratedRegistryProbe))
	readProbe := func() map[string]any {
		return p15SchemaJSON(t, p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./p15generatedprobe"))
	}
	baseline := readProbe()
	if baseline["hasParameter"] != false || baseline["completion"] != false || baseline["validErrors"] == json.Number("0") {
		t.Fatal("healthy baseline unexpectedly contains the future upstream parameter")
	}
	actionPath := filepath.Join(repo, "actions/web.cherri")
	original, err := os.ReadFile(actionPath)
	if err != nil {
		t.Fatal(err)
	}
	target := "action openURL(text url: 'WFInput') {"
	if bytes.Count(original, []byte(target)) != 1 {
		t.Fatal("upstream declaration fixture changed")
	}
	modified := strings.Replace(string(original), target, "action openURL(text url: 'WFInput', bool rehearsalDryRun: 'WFRehearsalDryRun') {", 1)
	p15SchemaWrite(t, actionPath, []byte(modified))
	// The declaration edit alone must not magically modify the old assembled
	// registry or completions. The next generation step is causally necessary.
	stale := readProbe()
	if stale["hasParameter"] != false || stale["completion"] != false {
		t.Fatal("stale registry control was not isolated")
	}
	registry := filepath.Join(repo, "internal/language/schema/generated_registry.go")
	originalRegistry, err := os.ReadFile(registry)
	if err != nil {
		t.Fatal(err)
	}
	p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-schema")
	actual := readProbe()
	if actual["hasParameter"] != true || actual["wireKey"] != "WFRehearsalDryRun" || actual["type"] != "Bool" || actual["completion"] != true || actual["validErrors"] != json.Number("0") || actual["wrongTypeErrors"] == json.Number("0") {
		t.Fatalf("actual regenerated registry did not drive analyzer and service correctly: %+v", actual)
	}
	if actual["fingerprint"] == baseline["fingerprint"] {
		t.Fatal("semantic declaration edit did not change the actual generated registry fingerprint")
	}
	p15SchemaWrite(t, actionPath, original)
	p15SchemaWrite(t, registry, originalRegistry)
	restored := readProbe()
	if !reflect.DeepEqual(restored, baseline) {
		t.Fatalf("restored production registry differs: before=%+v after=%+v", baseline, restored)
	}
	// Exercise an actual upstream parameter referenced by a narrow facet. A
	// renamed declaration must fail facet validation without writing a registry.
	calendarPath := filepath.Join(repo, "actions/calendar.cherri")
	calendar, err := os.ReadFile(calendarPath)
	if err != nil {
		t.Fatal(err)
	}
	old := "action adjustDate(text date: 'WFDate',"
	if bytes.Count(calendar, []byte(old)) != 1 {
		t.Fatal("facet declaration fixture changed")
	}
	p15SchemaWrite(t, calendarPath, []byte(strings.Replace(string(calendar), old, "action adjustDate(text rehearsalDate: 'WFDate',", 1)))
	facetOutput, facetErr := p15SchemaCommand(t, repo, nil, "go", "run", "-buildvcs=false", "./tools/language-schema")
	if facetErr == nil || !bytes.Contains(facetOutput, []byte("Facet validation failed:")) || !bytes.Contains(facetOutput, []byte(`references unknown primary parameter "date"`)) {
		t.Fatalf("upstream rename silently retained a stale facet: %v\n%s", facetErr, facetOutput)
	}
	unchanged, err := os.ReadFile(registry)
	if err != nil || !bytes.Equal(unchanged, originalRegistry) {
		t.Fatal("failed facet generation overwrote the healthy registry")
	}
	p15SchemaWrite(t, calendarPath, calendar)
	p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./tools/language-schema", "--verify-generated")
	t.Log("Actual declaration edit, live generation, compiled registry, analyzer, completion, stale-facet rejection and restore passed in an isolated tracked-source replica; no hand-assembled ActionSchema was substituted")
}
