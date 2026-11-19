package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
)

// This consumer is compiled from the generated registry in the disposable
// repository. It does not assemble an ActionSchema or copy a fingerprint oracle.
const p15RP08GeneratedConsumer = `package main
import (
 "encoding/json"
 "os"
 "github.com/electrikmilk/cherri/internal/language/analysis"
 "github.com/electrikmilk/cherri/internal/language/backend"
 "github.com/electrikmilk/cherri/internal/language/protocol"
 "github.com/electrikmilk/cherri/internal/language/schema"
 "github.com/electrikmilk/cherri/internal/language/source"
 "github.com/electrikmilk/cherri/internal/language/syntax"
)
func main() {
 var reg = schema.DefaultRegistry()
 var action, ok = reg.LookupAction("openURL")
 if !ok { panic("canonical fixture action missing") }
 var parameters = map[string]any{}
 for _, name := range []string{"rp08Flag", "rp08Zero", "rp08Empty", "rp08Absent"} {
  var parameter, found = action.ParameterByLabel(name)
  if !found || parameter.Type == nil { panic("generated parameter missing: " + name) }
  parameters[name] = map[string]any{"schema": parameter, "semanticType": parameter.Type.String()}
 }
 var errors = func(code string) int {
  var parser = syntax.NewParser(source.NewFile("rp08", "file:///rp08.cherri", 1, code))
  var program = parser.ParseProgram()
  var count = len(parser.Errors())
  var analyzer = analysis.NewAnalyzer(reg)
  analyzer.Analyze(program)
  for _, diagnostic := range analyzer.Diagnostics() { if diagnostic.Severity == analysis.SeverityError { count++ } }
  return count
 }
 var capabilities = protocol.BuildCapabilities(reg)
 json.NewEncoder(os.Stdout).Encode(map[string]any{
  "schemaFingerprint": reg.Fingerprint(), "codecFingerprint": backend.CodecFingerprint(),
  "capabilitySchema": capabilities.SchemaFingerprint, "capabilityCodec": capabilities.CodecFingerprint,
  "parameters": parameters, "static": action.StaticParameters,
  "validErrors": errors("openURL(url: \"https://example.invalid\", rp08Flag: false, rp08Zero: 0, rp08Empty: \"\")"),
  "wrongTypeErrors": errors("openURL(url: \"https://example.invalid\", rp08Flag: \"not-bool\")"),
 })
}
`

type p15RP08Observation struct {
	catalog, generated []byte
	exported, consumer map[string]any
}

func p15RP08Hash(data []byte) string {
	var sum = sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func p15RP08Action(t *testing.T, catalog map[string]any) map[string]any {
	t.Helper()
	var actions, ok = catalog["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatal("actual live exporter returned no actions")
	}
	var result map[string]any
	for _, raw := range actions {
		var action, valid = raw.(map[string]any)
		if !valid {
			t.Fatal("malformed actual action metadata")
		}
		if action["name"] == "openURL" {
			if result != nil {
				t.Fatal("duplicate canonical fixture action")
			}
			result = action
		}
	}
	if result == nil {
		t.Fatal("live exporter lost canonical fixture action")
	}
	return result
}

func p15RP08ExportedParameter(t *testing.T, action map[string]any, name string) map[string]any {
	t.Helper()
	var parameters, ok = action["parameters"].([]any)
	if !ok {
		t.Fatal("actual live action has no parameters")
	}
	for _, raw := range parameters {
		var parameter, valid = raw.(map[string]any)
		if !valid {
			t.Fatal("malformed actual parameter metadata")
		}
		if parameter["name"] == name {
			return parameter
		}
	}
	t.Fatalf("actual live parameter %s missing", name)
	return nil
}

func p15RP08Default(t *testing.T, parameter map[string]any, present bool, value string, boundary string) {
	t.Helper()
	var actual, found = parameter["default"]
	if found != present || present && actual != value {
		t.Errorf("%s default presence/value: got (%t, %#v), want (%t, %q)", boundary, found, actual, present, value)
	}
}

func TestP15RP08SchemaIdentityProduction(t *testing.T) {
	var repo = p15SchemaReplica(t)
	var binaries = t.TempDir()
	var compiler = filepath.Join(binaries, "compiler.exe")
	var generator = filepath.Join(binaries, "generator.exe")
	var actionPath = filepath.Join(repo, "actions/web.cherri")
	var original, err = os.ReadFile(actionPath)
	if err != nil {
		t.Fatal(err)
	}
	var oldHeader = "action openURL(text url: 'WFInput') {"
	var oldStatic = "\t\"Show-WFInput\": true\n}"
	var oldAction = oldHeader + "\n" + oldStatic
	if bytes.Count(original, []byte(oldAction)) != 1 {
		t.Fatal("original declaration fixture changed")
	}
	var codecPath = filepath.Join(repo, "internal/language/backend/contracts.go")
	var originalCodec, codecErr = os.ReadFile(codecPath)
	if codecErr != nil {
		t.Fatal(codecErr)
	}
	var codecVersion = "cherri-canonical-codec:v2.0"
	if bytes.Count(originalCodec, []byte(codecVersion)) != 1 {
		t.Fatal("actual codec version fixture changed")
	}
	var consumerDir = filepath.Join(repo, "p15rp08consumer")
	if err := os.Mkdir(consumerDir, 0755); err != nil {
		t.Fatal(err)
	}
	p15SchemaWrite(t, filepath.Join(consumerDir, "main.go"), []byte(p15RP08GeneratedConsumer))
	p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", generator, "./tools/language-schema")

	var declaration = func(flag, constant string, absentHasEmptyDefault bool) []byte {
		var absent = "text ?rp08Absent: 'WFRP08Absent'"
		if absentHasEmptyDefault {
			absent += ` = ""`
		}
		var header = "action openURL(text url: 'WFInput', bool ?rp08Flag: 'WFRP08Flag' = " + flag + ", number ?rp08Zero: 'WFRP08Zero' = 0, text ?rp08Empty: 'WFRP08Empty' = \"\", " + absent + ") {"
		var replacement = header + "\n\t\"Show-WFInput\": true,\n\t\"WFRP08IdentityConstant\": " + constant + "\n}"
		var text = strings.Replace(string(original), oldAction, replacement, 1)
		return []byte(text)
	}
	var observations = map[string]any{}
	var observe = func(t *testing.T, label string, action, codec []byte, twice bool) p15RP08Observation {
		t.Helper()
		p15SchemaWrite(t, actionPath, action)
		p15SchemaWrite(t, codecPath, codec)
		p15SchemaRun(t, repo, "go", "build", "-buildvcs=false", "-o", compiler, ".")
		var catalog = p15SchemaRun(t, repo, compiler, "--actions-json")
		if twice && !bytes.Equal(catalog, p15SchemaRun(t, repo, compiler, "--actions-json")) {
			t.Fatal("unchanged live export is nondeterministic")
		}
		var outputDir = t.TempDir()
		var input = filepath.Join(outputDir, "live-catalog.json")
		var output = filepath.Join(outputDir, "generated.go")
		p15SchemaWrite(t, input, catalog)
		p15SchemaRun(t, repo, generator, input, output)
		var generated, err = os.ReadFile(output)
		if err != nil {
			t.Fatal(err)
		}
		if twice {
			var second = filepath.Join(outputDir, "second.go")
			p15SchemaRun(t, repo, generator, input, second)
			var duplicate, err = os.ReadFile(second)
			if err != nil || !bytes.Equal(generated, duplicate) {
				t.Fatal("unchanged semantic generation is nondeterministic")
			}
		}
		p15SchemaWrite(t, filepath.Join(repo, "internal/language/schema/generated_registry.go"), generated)
		var actual = p15SchemaJSON(t, p15SchemaRun(t, repo, "go", "run", "-buildvcs=false", "./p15rp08consumer"))
		for _, field := range []string{"schemaFingerprint", "codecFingerprint"} {
			var identity, ok = actual[field].(string)
			if !ok || !regexp.MustCompile(`^[0-9a-f]{64}$`).MatchString(identity) {
				t.Fatalf("compiled identity missing: %+v", actual)
			}
		}
		if actual["capabilitySchema"] != actual["schemaFingerprint"] || actual["capabilityCodec"] != actual["codecFingerprint"] {
			t.Fatal("actual protocol consumer did not preserve the declared identity tuple")
		}
		if actual["validErrors"] != json.Number("0") || actual["wrongTypeErrors"] == json.Number("0") {
			t.Fatalf("actual generated registry failed typed analyzer controls: %+v", actual)
		}
		var exported = p15RP08Action(t, p15SchemaJSON(t, catalog))
		observations[label] = map[string]any{"declaration_sha256": p15RP08Hash(action), "catalog_sha256": p15RP08Hash(catalog), "generated_sha256": p15RP08Hash(generated), "schemaFingerprint": actual["schemaFingerprint"], "codecFingerprint": actual["codecFingerprint"], "exported": exported, "consumer": actual}
		return p15RP08Observation{catalog, generated, exported, actual}
	}
	var baselineSource = declaration("false", "false", false)
	var baseline = observe(t, "unchanged", baselineSource, originalCodec, true)
	t.Run("typed_default_presence_through_live_generation", func(t *testing.T) {
		var cases = []struct {
			name, kind, value string
			present           bool
		}{{"rp08Flag", "Bool", "false", true}, {"rp08Zero", "Number", "0", true}, {"rp08Empty", "Text", "", true}, {"rp08Absent", "Text", "", false}}
		var parameters = baseline.consumer["parameters"].(map[string]any)
		for _, sample := range cases {
			var exported = p15RP08ExportedParameter(t, baseline.exported, sample.name)
			p15RP08Default(t, exported, sample.present, sample.value, "live "+sample.name)
			var compiled = parameters[sample.name].(map[string]any)
			if compiled["semanticType"] != sample.kind {
				t.Errorf("%s lost semantic type: %+v", sample.name, compiled)
			}
			p15RP08Default(t, compiled["schema"].(map[string]any), sample.present, sample.value, "compiled "+sample.name)
		}
	})
	t.Run("changed_default_changes_schema_identity", func(t *testing.T) {
		var changed = observe(t, "default", declaration("true", "false", false), originalCodec, false)
		if changed.consumer["schemaFingerprint"] == baseline.consumer["schemaFingerprint"] || changed.consumer["codecFingerprint"] != baseline.consumer["codecFingerprint"] {
			t.Fatal("semantic default change did not change precisely the schema identity component")
		}
		p15RP08Default(t, p15RP08ExportedParameter(t, changed.exported, "rp08Flag"), true, "true", "changed live Bool")
	})
	t.Run("absent_and_explicit_empty_have_distinct_identity", func(t *testing.T) {
		var changed = observe(t, "explicit_empty", declaration("false", "false", true), originalCodec, false)
		p15RP08Default(t, p15RP08ExportedParameter(t, changed.exported, "rp08Absent"), true, "", "new explicit empty")
		if changed.consumer["schemaFingerprint"] == baseline.consumer["schemaFingerprint"] {
			t.Fatal("absent and explicit empty default collapsed to the same compiled identity")
		}
	})
	t.Run("changed_constant_changes_schema_identity", func(t *testing.T) {
		var changed = observe(t, "constant_value", declaration("false", "true", false), originalCodec, false)
		if changed.consumer["static"].(map[string]any)["WFRP08IdentityConstant"] != true {
			t.Fatal("real constant mutation did not reach the compiled registry")
		}
		if changed.consumer["schemaFingerprint"] == baseline.consumer["schemaFingerprint"] {
			t.Fatal("changed semantic constant retained schema identity")
		}
	})
	t.Run("typed_constant_identity_does_not_alias", func(t *testing.T) {
		var changed = observe(t, "constant_type", declaration("false", `"false"`, false), originalCodec, false)
		if baseline.consumer["static"].(map[string]any)["WFRP08IdentityConstant"] != false || changed.consumer["static"].(map[string]any)["WFRP08IdentityConstant"] != "false" {
			t.Fatal("Bool/Text constant mutation did not reach the actual compiled registry")
		}
		if changed.consumer["schemaFingerprint"] == baseline.consumer["schemaFingerprint"] {
			t.Fatal("native Bool false and Text false constants alias the same semantic identity")
		}
	})
	t.Run("codec_version_changes_identity_tuple", func(t *testing.T) {
		var changedCodec = bytes.Replace(originalCodec, []byte(codecVersion), []byte(codecVersion+"-rp08-disposable"), 1)
		var changed = observe(t, "codec", baselineSource, changedCodec, false)
		if !bytes.Equal(changed.generated, baseline.generated) || changed.consumer["schemaFingerprint"] != baseline.consumer["schemaFingerprint"] || changed.consumer["codecFingerprint"] == baseline.consumer["codecFingerprint"] {
			t.Fatal("codec version did not change the distinct codec identity while retaining unchanged schema semantics")
		}
	})
	t.Run("restored_actual_generation", func(t *testing.T) {
		var restored = observe(t, "restored", baselineSource, originalCodec, false)
		if !bytes.Equal(restored.catalog, baseline.catalog) || !bytes.Equal(restored.generated, baseline.generated) || !reflect.DeepEqual(restored.consumer, baseline.consumer) {
			t.Fatal("restored actual production inputs did not restore exact semantic output and identity")
		}
	})
	var proof, proofErr = json.Marshal(observations)
	if proofErr != nil {
		t.Fatal(proofErr)
	}
	t.Logf("P15_RP08_PRODUCTION_OBSERVATIONS %s", proof)
}
