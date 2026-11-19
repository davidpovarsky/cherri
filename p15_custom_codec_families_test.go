package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"github.com/electrikmilk/cherri/internal/language/source"
	"github.com/electrikmilk/cherri/internal/language/syntax"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

const p15AllCodecFamilies = `enum P15CodecMode { Enabled = "native-enabled", Empty = "" }
action p15CodecFamilies(pText: Text, pSegments: Text, pNumber: Number, pFlag: Bool, pMap: Map<Text, AnyContent>, pArray: List<Text>, pToken: AnyContent, pMode: P15CodecMode) {
 identifier: "com.example.p15.codec-families"
 bindings: {
  pText: {key: "TextValue", codec: "text"},
  pSegments: {key: "SegmentsValue", codec: "textToken"},
  pNumber: {key: "NumberValue", codec: "number"},
  pFlag: {key: "BoolValue", codec: "bool"},
  pMap: {key: "DictionaryValue", codec: "dictionary"},
  pArray: {key: "ArrayValue", codec: "array"},
  pToken: {key: "TokenValue", codec: "token"},
  pMode: {key: "EnumValue", codec: "enum"}
 }
}
p15CodecFamilies("literal", f"head {ShortcutInput} tail", 2.5, false, {"zero": 0, "empty": "", "flag": false}, ["first", "", "last"], ShortcutInput, .Enabled)
`

var p15CodecFamilyFields = []struct{ label, key, codec string }{
	{"pText", "TextValue", "text"}, {"pSegments", "SegmentsValue", "textToken"},
	{"pNumber", "NumberValue", "number"}, {"pFlag", "BoolValue", "bool"},
	{"pMap", "DictionaryValue", "dictionary"}, {"pArray", "ArrayValue", "array"},
	{"pToken", "TokenValue", "token"}, {"pMode", "EnumValue", "enum"},
}

func p15CodecFamiliesParameters(root map[string]any) (map[string]any, error) {
	rows, ok := root["WFWorkflowActions"].([]any)
	if !ok || len(rows) != 1 {
		return nil, fmt.Errorf("expected exactly one custom native action")
	}
	row, ok := rows[0].(map[string]any)
	if !ok || row["WFWorkflowActionIdentifier"] != "com.example.p15.codec-families" {
		return nil, fmt.Errorf("wrong custom action identity")
	}
	params, ok := row["WFWorkflowActionParameters"].(map[string]any)
	if !ok || len(params) != 8 {
		return nil, fmt.Errorf("custom parameter cardinality changed")
	}
	return params, nil
}

func p15CodecFamiliesError(root map[string]any, numberKey string) error {
	params, err := p15CodecFamiliesParameters(root)
	if err != nil {
		return err
	}
	if text, ok := params["TextValue"].(string); !ok || text != "literal" {
		return fmt.Errorf("text codec changed literal")
	}
	if number, ok := params[numberKey].(float64); !ok || number != 2.5 {
		return fmt.Errorf("number codec changed native real")
	}
	if flag, ok := params["BoolValue"].(bool); !ok || flag {
		return fmt.Errorf("bool codec lost explicit false")
	}
	if params["EnumValue"] != "native-enabled" {
		return fmt.Errorf("enum codec emitted another domain value")
	}
	wantToken := map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": map[string]any{"Type": "ExtensionInput"}}
	if !plistValuesStrictlyEqual(params["TokenValue"], wantToken) {
		return fmt.Errorf("token codec changed system reference")
	}
	wantSegments := map[string]any{"WFSerializationType": "WFTextTokenString", "Value": map[string]any{
		"string": "head \uFFFC tail", "attachmentsByRange": map[string]any{"{5, 1}": map[string]any{"Type": "ExtensionInput"}},
	}}
	if !plistValuesStrictlyEqual(params["SegmentsValue"], wantSegments) {
		return fmt.Errorf("textToken codec changed bytes or UTF-16 attachment: %#v", params["SegmentsValue"])
	}
	dict, err := p15ReadDictionaryLiteral(params["DictionaryValue"])
	if err != nil || !reflect.DeepEqual(dict, map[string]any{"zero": p15LiteralNumber("0"), "empty": "", "flag": false}) {
		return fmt.Errorf("dictionary codec lost typed false/zero/empty: %#v %v", dict, err)
	}
	array, err := p15ReadDictionaryRow(map[string]any{"WFItemType": 2, "WFValue": params["ArrayValue"]})
	if err != nil || !reflect.DeepEqual(array, []any{"first", "", "last"}) {
		return fmt.Errorf("array codec lost order/cardinality/empty element: %#v %v", array, err)
	}
	return nil
}

func p15AnalyzeCodecFamilies(t *testing.T, src string) (*analysis.Analyzer, *schema.Registry) {
	t.Helper()
	base := schema.DefaultRegistry()
	parser := syntax.NewParser(source.NewFile("codec-families", "file:///codec-families.cherri", 1, src))
	program := parser.ParseProgram()
	if len(parser.Errors()) != 0 {
		t.Fatal(parser.Errors())
	}
	analyzer := analysis.NewAnalyzer(base)
	analyzer.Analyze(program)
	for _, d := range analyzer.Diagnostics() {
		if d.Severity == analysis.SeverityError {
			t.Fatal(d)
		}
	}
	return analyzer, base
}

func TestP15AC02AllCodecFamiliesNativeShape(t *testing.T) {
	root := p15CodecNative(t, p15AllCodecFamilies)
	if err := p15CodecFamiliesError(root, "NumberValue"); err != nil {
		t.Fatal(err)
	}
	for _, field := range p15CodecFamilyFields {
		for _, fault := range []string{"missing", "wrong_type_or_shape"} {
			t.Run(field.codec+"/"+fault, func(t *testing.T) {
				changed := deepClonePlistMap(root)
				params, err := p15CodecFamiliesParameters(changed)
				if err != nil {
					t.Fatal(err)
				}
				if fault == "missing" {
					delete(params, field.key)
				} else {
					params[field.key] = map[string]any{"P15Wrong": "shape"}
				}
				if p15CodecFamiliesError(changed, "NumberValue") == nil || shortcutcompare.CompareDocumentsStrict(root, changed).Equal {
					t.Fatal("corrupted codec family passed oracle")
				}
			})
		}
	}
	// The same authored field edit must reach the same schema model and the
	// actual native emission. No hand-built ActionSchema or second database is
	// substituted for either path.
	changedSource := strings.Replace(p15AllCodecFamilies, `key: "NumberValue"`, `key: "ChangedNumberValue"`, 1)
	changed := p15CodecNative(t, changedSource)
	if err := p15CodecFamiliesError(changed, "ChangedNumberValue"); err != nil {
		t.Fatal(err)
	}
	params, _ := p15CodecFamiliesParameters(changed)
	if _, exists := params["NumberValue"]; exists {
		t.Fatal("old binding survived actual source edit")
	}
	if p15CodecFamiliesError(changed, "NumberValue") == nil {
		t.Fatal("stale wire-key oracle accepted authored change")
	}
	analyzer, base := p15AnalyzeCodecFamilies(t, changedSource)
	custom, ok := analyzer.Registry().LookupAction("p15CodecFamilies")
	if !ok {
		t.Fatal("source declaration absent from shared schema")
	}
	field, ok := custom.ParameterByLabel("pNumber")
	if !ok || field.WireKey != "ChangedNumberValue" || field.Codec != "number" {
		t.Fatalf("schema retained stale authored binding: %+v", field)
	}
	if _, leaked := base.LookupAction("p15CodecFamilies"); leaked {
		t.Fatal("custom schema escaped its document")
	}
	restored := p15CodecNative(t, p15AllCodecFamilies)
	if !shortcutcompare.CompareDocumentsStrict(root, restored).Equal {
		t.Fatal("restored declaration did not restore native wire")
	}
}

func TestP15AC02DeclaredSchemaRegistryAndRejections(t *testing.T) {
	analyzer, base := p15AnalyzeCodecFamilies(t, p15AllCodecFamilies)
	custom, ok := analyzer.Registry().LookupAction("p15CodecFamilies")
	if !ok || !custom.Custom || custom.AppleIdentifier != "com.example.p15.codec-families" || len(custom.Parameters) != 8 {
		t.Fatalf("public declaration did not feed common ActionSchema: %+v", custom)
	}
	seen := map[string]bool{}
	for _, want := range p15CodecFamilyFields {
		param, ok := custom.ParameterByLabel(want.label)
		if !ok || param.WireKey != want.key || param.Codec != want.codec || !backend.IsRegisteredParameterCodec(param.Codec) {
			t.Fatalf("public schema field detached from declaration: %+v", param)
		}
		if seen[param.Codec] {
			t.Fatal("codec family duplicated")
		}
		seen[param.Codec] = true
	}
	mode, _ := custom.ParameterByLabel("pMode")
	if mode.EnumName != "P15CodecMode" || len(mode.EnumValues) != 2 || !((mode.EnumValues[0] == "native-enabled" && mode.EnumValues[1] == "") || (mode.EnumValues[1] == "native-enabled" && mode.EnumValues[0] == "")) {
		t.Fatalf("custom enum domain detached from source: %+v", mode)
	}
	if _, leaked := base.LookupAction("p15CodecFamilies"); leaked {
		t.Fatal("custom declaration polluted canonical registry")
	}
	for _, tc := range []struct{ name, source, code string }{
		{"wrong_type", strings.Replace(p15AllCodecFamilies, "pArray: List<Text>", "pArray: Bool", 1), analysis.CodeArgumentType},
		{"unknown_codec", strings.Replace(p15AllCodecFamilies, `codec: "number"`, `codec: "P15_UNREGISTERED_EXECUTABLE"`, 1), analysis.CodeSchemaMismatch},
		{"unknown_enum", strings.Replace(p15AllCodecFamilies, ".Enabled)", ".Missing)", 1), analysis.CodeEnumMember},
		{"foreign_document", "p15CodecFamilies()", analysis.CodeUnknownAction},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("invalid-codec-family.cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), tc.code) {
				t.Fatalf("invalid custom schema produced output or wrong diagnostic: %v", err)
			}
		})
	}
}
