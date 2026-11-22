package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const p15CanonicalRebuildProbe = "package main\nimport (\n \"encoding/json\"\n \"os\"\n \"testing\"\n)\nfunc TestP15BRG24RebuiltNativeProbe(t *testing.T) {\n cases:=[]struct{name,source,include,identifier string}{\n  {\"static\", \"base64Encode(ShortcutInput)\", \"actions/crypto\", \"is.workflow.actions.base64encode\"},\n  {\"callback\", \"getPDFText(ShortcutInput)\", \"actions/pdf\", \"is.workflow.actions.gettextfrompdf\"},\n  {\"codec\", \"quicklook(ShortcutInput)\", \"\", \"is.workflow.actions.previewdocument\"},\n }\n result:=make(map[string]map[string]map[string]any)\n for _,tc:=range cases {\n  routes:=make(map[string]map[string]any)\n  for _,legacy:=range []bool{false,true} {\n   src,route:=tc.source,\"v2\"\n   if legacy {if tc.include!=\"\" {src=\"#include '\"+tc.include+\"'\\n\"+src};route=\"canonical\"}\n   acts:=compileDebugFixture(t,src,legacy)\n   count:=1\n   if !legacy { count=2 }\n   if len(acts)!=count || acts[0].WFWorkflowActionIdentifier!=tc.identifier {\n    t.Fatalf(\"%s/%s action identity or cardinality changed: %+v\",tc.name,route,acts)\n   }\n   if !legacy && (acts[1].WFWorkflowActionIdentifier!=\"is.workflow.actions.nothing\" || len(acts[1].WFWorkflowActionParameters)!=0) {\n    t.Fatalf(\"%s/%s missing exact empty main completion: %+v\",tc.name,route,acts)\n   }\n   params:=deepClonePlistMap(acts[0].WFWorkflowActionParameters)\n   delete(params,\"UUID\")\n   routes[route]=map[string]any{\"identifier\":tc.identifier,\"parameters\":params}\n  }\n  result[tc.name]=routes\n }\n raw,err:=json.MarshalIndent(result,\"\",\"  \")\n if err!=nil {t.Fatal(err)}\n target:=os.Getenv(\"P15_BRG24_OUTPUT\")\n if target==\"\" {t.Fatal(\"missing isolated output path\")}\n if err:=os.WriteFile(target,append(raw,'\\n'),0600);err!=nil {t.Fatal(err)}\n}\n"

type p15CanonicalRebuildResult map[string]map[string]map[string]any

func p15CanonicalRebuildRead(t *testing.T, repo, phase string) p15CanonicalRebuildResult {
	t.Helper()
	output := filepath.Join(t.TempDir(), phase+".json")
	raw, err := p15SchemaCommand(t, repo, []string{"P15_BRG24_OUTPUT=" + output}, "go", "test", "-buildvcs=false", "-count=1", "-json", "-timeout=90s", "-run", "^TestP15BRG24RebuiltNativeProbe$", ".")
	if err != nil {
		t.Fatalf("rebuilt native probe %s failed: %v\n%s", phase, err, raw)
	}
	runs, passes, packages := 0, 0, 0
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event struct{ Action, Test, Package string }
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Package != "github.com/electrikmilk/cherri" || event.Test != "" && event.Test != "TestP15BRG24RebuiltNativeProbe" {
			t.Fatalf("foreign probe event: %s", line)
		}
		switch event.Action {
		case "run":
			runs++
		case "pass":
			if event.Test == "" {
				packages++
			} else {
				passes++
			}
		case "output", "start":
		default:
			t.Fatalf("incomplete or failed rebuilt probe: %s", line)
		}
	}
	if runs != 1 || passes != 1 || packages != 1 {
		t.Fatalf("missing actual selected probe execution: %d/%d/%d", runs, passes, packages)
	}
	native, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var result p15CanonicalRebuildResult
	if err := json.Unmarshal(native, &result); err != nil {
		t.Fatal(err)
	}
	if len(result) != 3 {
		t.Fatal("missing actual native scenarios")
	}
	for name, routes := range result {
		if len(routes) != 2 || routes["canonical"] == nil || routes["v2"] == nil || !reflect.DeepEqual(routes["canonical"], routes["v2"]) {
			t.Fatalf("%s did not propagate identically to the rebuilt canonical and V2 routes: %+v", name, routes)
		}
	}
	t.Logf("phase=%s actual child Go JSON SHA256=%x emitted native tree SHA256=%x", phase, sha256.Sum256(raw), sha256.Sum256(native))
	return result
}

func p15CanonicalRebuildClone(t *testing.T, value p15CanonicalRebuildResult) p15CanonicalRebuildResult {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	var clone p15CanonicalRebuildResult
	if err := json.Unmarshal(raw, &clone); err != nil {
		t.Fatal(err)
	}
	return clone
}

func p15CanonicalRebuildParams(t *testing.T, result p15CanonicalRebuildResult, scenario, route string) map[string]any {
	t.Helper()
	params, ok := result[scenario][route]["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("%s/%s lacks actual native parameters", scenario, route)
	}
	return params
}

func p15CanonicalRebuildReferenceType(t *testing.T, params map[string]any, want string) {
	t.Helper()
	input, ok := params["WFInput"].(map[string]any)
	if !ok {
		t.Fatal("native input attachment missing")
	}
	value, ok := input["Value"].(map[string]any)
	if !ok || input["WFSerializationType"] != "WFTextTokenAttachment" || value["Type"] != "ExtensionInput" {
		t.Fatalf("expected exact original canonical ExtensionInput attachment: %#v", input)
	}
	value["Type"] = want
}

// BRG24 changes actual committed canonical source in an owned detached checkout.
// Both compilers are rebuilt from those edits. No hook or generated metadata is
// changed, and the complete native parameter maps must show only each intended
// effect. These deliberate invalid native markers are propagation controls,
// never Apple runtime fixtures or changes to shipping compiler semantics.
func TestP15CanonicalBehaviorSourcePropagation(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	repo := p15SchemaAcceptanceWorktree(t)
	probe := filepath.Join(repo, "p15_brg24_rebuilt_probe_test.go")
	p15SchemaWrite(t, probe, []byte(p15CanonicalRebuildProbe))
	baseline := p15CanonicalRebuildRead(t, repo, "baseline")
	for _, route := range []string{"canonical", "v2"} {
		if p15CanonicalRebuildParams(t, baseline, "static", route)["WFEncodeMode"] != "Encode" ||
			p15CanonicalRebuildParams(t, baseline, "callback", route)["WFGetTextFromPDFTextType"] != "Text" {
			t.Fatal("canonical baseline fixed behavior changed")
		}
		check := p15CanonicalRebuildClone(t, baseline)
		for _, name := range []string{"static", "callback", "codec"} {
			p15CanonicalRebuildReferenceType(t, p15CanonicalRebuildParams(t, check, name, route), "ExtensionInput")
		}
	}
	unchanged := map[string][]byte{}
	for _, name := range []string{"internal/language/schema/generated_registry.go", "language_backend_adapter.go", "language_emit_adapter.go", "language_registry_adapter.go", "internal/language/lower/lower.go"} {
		data, err := os.ReadFile(filepath.Join(repo, name))
		if err != nil {
			t.Fatal(err)
		}
		unchanged[name] = data
	}
	cases := []struct {
		name, path, old, replacement string
		expected                     func(*testing.T, p15CanonicalRebuildResult)
	}{
		{"static", "actions/crypto.cherri", "\"WFEncodeMode\": \"Encode\"", "\"WFEncodeMode\": \"P15_STATIC_PROPAGATION\"",
			func(t *testing.T, expected p15CanonicalRebuildResult) {
				for _, route := range []string{"canonical", "v2"} {
					p15CanonicalRebuildParams(t, expected, "static", route)["WFEncodeMode"] = "P15_STATIC_PROPAGATION"
				}
			}},
		{"callback", "actions_std.go", "\"WFGetTextFromPDFTextType\": \"Text\"", "\"WFGetTextFromPDFTextType\": \"P15_CALLBACK_PROPAGATION\"",
			func(t *testing.T, expected p15CanonicalRebuildResult) {
				for _, route := range []string{"canonical", "v2"} {
					p15CanonicalRebuildParams(t, expected, "callback", route)["WFGetTextFromPDFTextType"] = "P15_CALLBACK_PROPAGATION"
				}
			}},
		{"codec", "shared_codec.go", "\tif SharedCodecHook != nil {", "\tif desc.Kind == \"ExtensionInput\" { val.Type = \"P15_CODEC_PROPAGATION\" }\n\tif SharedCodecHook != nil {",
			func(t *testing.T, expected p15CanonicalRebuildResult) {
				for _, name := range []string{"static", "callback", "codec"} {
					for _, route := range []string{"canonical", "v2"} {
						p15CanonicalRebuildReferenceType(t, p15CanonicalRebuildParams(t, expected, name, route), "P15_CODEC_PROPAGATION")
					}
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(repo, tc.path)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Count(original, []byte(tc.old)) != 1 {
				t.Fatalf("canonical source anchor changed: %s", tc.path)
			}
			p15SchemaWrite(t, path, []byte(strings.Replace(string(original), tc.old, tc.replacement, 1)))
			defer p15SchemaWrite(t, path, original)
			changedPaths := strings.TrimSpace(string(p15SchemaRun(t, repo, "git", "diff", "--name-only")))
			if changedPaths != tc.path {
				t.Fatalf("canonical edit changed unrelated tracked source: %s", changedPaths)
			}
			changedSource, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("actual canonical edit path=%s before_SHA256=%x after_SHA256=%x", tc.path, sha256.Sum256(original), sha256.Sum256(changedSource))
			expected := p15CanonicalRebuildClone(t, baseline)
			tc.expected(t, expected)
			if reflect.DeepEqual(expected, baseline) {
				t.Fatal("propagation mutation had no native effect")
			}
			actual := p15CanonicalRebuildRead(t, repo, tc.name)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("native propagation differs from exact intended change\nactual=%+v\nexpected=%+v", actual, expected)
			}
			for name, data := range unchanged {
				got, err := os.ReadFile(filepath.Join(repo, name))
				if err != nil || !bytes.Equal(got, data) {
					t.Fatalf("propagation changed forbidden V2/generated input %s", name)
				}
			}
			// Incomplete propagation, stale native output and wrong unrelated output
			// must fail the full-tree oracle even if both compilers are made to agree.
			for _, route := range []string{"canonical", "v2"} {
				incomplete := p15CanonicalRebuildClone(t, actual)
				for name := range incomplete {
					incomplete[name][route] = p15CanonicalRebuildClone(t, baseline)[name][route]
				}
				if reflect.DeepEqual(incomplete, expected) {
					t.Fatal("one stale route passed exact native oracle")
				}
			}
			corrupt := p15CanonicalRebuildClone(t, actual)
			for _, route := range []string{"canonical", "v2"} {
				p15CanonicalRebuildParams(t, corrupt, "codec", route)["P15UnrelatedField"] = false
			}
			if reflect.DeepEqual(corrupt, expected) {
				t.Fatal("matching unrelated corruption passed exact native oracle")
			}
			p15SchemaWrite(t, path, original)
			restored := p15CanonicalRebuildRead(t, repo, tc.name+"-restored")
			if !reflect.DeepEqual(restored, baseline) {
				t.Fatal("restoring canonical source did not restore both native outputs")
			}
		})
	}
	if t.Failed() {
		return
	}
	if err := os.Remove(probe); err != nil {
		t.Fatal(err)
	}
	status := strings.TrimSpace(string(p15SchemaRun(t, repo, "git", "status", "--porcelain", "--untracked-files=normal")))
	if status != "" {
		t.Fatal(fmt.Sprintf("owned replica was not fully restored: %s", status))
	}
	t.Log("Actual static declaration, canonical callback and shared codec source mutations propagated through both rebuilt compiler routes; exact native controls, restoration, unchanged V2/generated sources and original worktree preservation passed.")
}
