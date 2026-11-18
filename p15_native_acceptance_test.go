package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"howett.net/plist"
)

type p15NativeImportObservation struct {
	mode, fidelity string
	source         string
	compiled       map[string]any
	plan           *HybridImportPlan
}

// Exercise actual filesystem intake, shared planning/rendering and each real
// frontend. Expected data comes from independently authored native input, never
// from a second rendering of the same source.
func p15NativeImportRoundTrip(t *testing.T, name string, root map[string]any, formats ...int) []p15NativeImportObservation {
	t.Helper()
	if len(formats) == 0 {
		formats = []int{plist.XMLFormat}
	}
	var observations []p15NativeImportObservation
	for _, format := range formats {
		raw := buildNativePlistBytes(t, root, format)
		doc, err := DecodeNativeShortcutDocument(raw)
		if err != nil {
			t.Fatal(err)
		}
		dir := t.TempDir()
		input := filepath.Join(dir, name+".shortcut")
		if err := os.WriteFile(input, raw, 0600); err != nil {
			t.Fatal(err)
		}
		for _, fidelity := range []string{"hybrid", "lossless"} {
			for _, mode := range []string{"legacy", "v2"} {
				t.Run(fmt.Sprintf("%s/%d/%s/%s", name, format, fidelity, mode), func(t *testing.T) {
					out := filepath.Join(dir, name+"-"+fidelity+"-"+mode+".cherri")
					result, err := ExecuteImportPipeline(ImportOptions{InputPath: input, OutputPath: out, Mode: mode, Fidelity: fidelity})
					if err != nil {
						t.Fatalf("%s format=%d %s/%s import: %v", name, format, fidelity, mode, err)
					}
					written, err := os.ReadFile(out)
					if err != nil || string(written) != result.FinalSource || len(written) == 0 {
						t.Fatalf("actual import output missing or changed: %v", err)
					}
					compiled, err := compileHybridSource(result.FinalSource, mode == "legacy")
					if err != nil {
						t.Fatalf("%s %s/%s compile: %v\n%s", name, fidelity, mode, err, result.FinalSource)
					}
					if err := p15NativePreservationError(doc.RawRoot, compiled); err != nil {
						t.Fatalf("%s format=%d %s/%s: %v", name, format, fidelity, mode, err)
					}
					if result.Plan != nil && result.Plan.ExactNativeCount+result.Plan.NaturalVerifiedCount+result.Plan.SourceInvalidCount != len(doc.Actions) {
						t.Fatalf("planner omitted an action: %+v", result.Plan)
					}
					inputSHA, sourceSHA := sha256.Sum256(raw), sha256.Sum256(written)
					encoded, err := plist.Marshal(compiled, plist.XMLFormat)
					if err != nil {
						t.Fatal(err)
					}
					outputSHA := sha256.Sum256(encoded)
					receipt, _ := json.Marshal(map[string]any{
						"name": name, "format": format, "mode": mode, "fidelity": fidelity,
						"input_sha256": hex.EncodeToString(inputSHA[:]), "source_sha256": hex.EncodeToString(sourceSHA[:]),
						"output_sha256": hex.EncodeToString(outputSHA[:]), "action_count": len(doc.Actions),
						"exact_tree": true, "runtime_observed": false,
					})
					t.Log("P15_NATIVE_SPECIMEN:" + string(receipt))
					observations = append(observations, p15NativeImportObservation{mode, fidelity, result.FinalSource, compiled, result.Plan})
				})
			}
		}
	}
	return observations
}

func p15NativePreservationError(original, actual map[string]any) error {
	if hasImportProvenanceKey(actual) {
		return fmt.Errorf("compiler provenance leaked into native output")
	}
	if !plistValuesStrictlyEqual(original, actual) {
		return fmt.Errorf("native tree changed at %s", firstPlistDiffPath(original, actual, ""))
	}
	return nil
}

func p15AcceptanceAction(id string, params map[string]any) map[string]any {
	return map[string]any{"WFWorkflowActionIdentifier": id, "WFWorkflowActionParameters": params}
}

func p15AcceptanceRef(uuid string, aggrandizements []any) map[string]any {
	token := map[string]any{"Type": "ActionOutput", "OutputUUID": uuid, "OutputName": "Synthetic output"}
	if aggrandizements != nil {
		token["Aggrandizements"] = aggrandizements
	}
	return map[string]any{"WFSerializationType": "WFTextTokenAttachment", "Value": token}
}

func TestP15C05UnknownEnumNativeImport(t *testing.T) {
	for _, mode := range []string{"P15_FUTURE_CASE_A", "P15_FUTURE_CASE_B"} {
		root := baseTestHeader()
		root["WFWorkflowActions"] = []any{
			p15AcceptanceAction("is.workflow.actions.text.changecase", map[string]any{"text": "literal", "Show-text": true, "WFCaseType": mode}),
			p15AcceptanceAction("is.workflow.actions.text.changecase", map[string]any{"text": "known", "Show-text": true, "WFCaseType": "UPPERCASE"}),
		}
		for _, result := range p15NativeImportRoundTrip(t, mode, root) {
			if result.plan != nil {
				if result.plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
					t.Fatalf("unknown enum was guessed as a natural variant: %+v", result.plan.Decisions[0])
				}
				if result.plan.Decisions[1].Classification != "NATURAL_VERIFIED" {
					t.Fatalf("known enum control lost readability: %+v", result.plan.Decisions[1])
				}
			}
			params := result.compiled["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
			if params["WFCaseType"] != mode {
				t.Fatal("unknown wire enum replaced with a known member")
			}
		}
	}
}

func p15UnknownNativeAcceptanceRoot() map[string]any {
	const producer = "01234567-89ab-4cde-8123-0123456789ab"
	root := baseTestHeader()
	root["P15UnknownTopLevel"] = map[string]any{
		"ordered":         []any{"first", int64(9007199254740993), false, "last"},
		"nested":          map[string]any{"bytes": []byte{0, 1, 254, 255}, "date": time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)},
		"uuidLikeLiteral": producer,
	}
	unknown := p15AcceptanceAction("com.example.p15.unknown.action", map[string]any{
		"UUID": producer, "CustomOutputName": "Synthetic output",
		"P15Payload": map[string]any{"empty": "", "list": []any{}, "map": map[string]any{}, "values": []any{int64(0), false, "0"}},
	})
	unknown["P15UnknownActionField"] = []any{"a", map[string]any{"b": []any{"c", "d"}}}
	root["WFWorkflowActions"] = []any{
		unknown,
		p15AcceptanceAction("is.workflow.actions.getvariable", map[string]any{"WFVariable": p15AcceptanceRef(producer, nil)}),
	}
	return root
}

func TestP15R01UnknownNativeFieldsRoundTrip(t *testing.T) {
	t.Run("without_action_envelope_control", func(t *testing.T) {
		root := p15UnknownNativeAcceptanceRoot()
		delete(root["WFWorkflowActions"].([]any)[0].(map[string]any), "P15UnknownActionField")
		p15NativeImportRoundTrip(t, "unknown-tree-control", root, plist.XMLFormat, plist.BinaryFormat)
	})
	root := p15UnknownNativeAcceptanceRoot()
	for _, result := range p15NativeImportRoundTrip(t, "unknown-tree", root, plist.XMLFormat, plist.BinaryFormat) {
		if result.plan != nil && result.plan.Decisions[0].Classification != "EXACT_NATIVE_FALLBACK" {
			t.Fatalf("unknown action must retain the exact native route: %+v", result.plan.Decisions[0])
		}
		actions := result.compiled["WFWorkflowActions"].([]any)
		first := actions[0].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		token := actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariable"].(map[string]any)["Value"].(map[string]any)
		if token["OutputUUID"] != first["UUID"] {
			t.Fatal("cross-action output edge changed")
		}
	}
}

func TestP15R02KnownNativeUnmodeledVariants(t *testing.T) {
	root := baseTestHeader()
	root["WFWorkflowActions"] = []any{
		p15AcceptanceAction("is.workflow.actions.appearance", map[string]any{"operation": "set", "style": "dark"}),
		p15AcceptanceAction("is.workflow.actions.appearance", map[string]any{}),
		p15AcceptanceAction("is.workflow.actions.comment", map[string]any{
			"WFCommentActionText": "known action with unknown future key",
			"WFP15Future":         map[string]any{"nested": []any{"first", false, int64(0)}, "literal": "WFTextTokenAttachment"},
		}),
		p15AcceptanceAction("is.workflow.actions.text.changecase", map[string]any{
			"WFCaseType": "UPPERCASE", "Show-text": true,
			"text": map[string]any{"FutureTextShape": []any{"left", "right"}},
		}),
	}
	for _, result := range p15NativeImportRoundTrip(t, "known-future-variant", root) {
		if result.plan == nil {
			continue
		}
		if result.plan.Decisions[0].Classification != "NATURAL_VERIFIED" {
			t.Fatalf("representable static variant control demoted: %+v", result.plan.Decisions[0])
		}
		for _, index := range []int{1, 2, 3} {
			if result.plan.Decisions[index].Classification != "EXACT_NATIVE_FALLBACK" {
				t.Fatalf("unmodeled native variant/value guessed at %d: %+v", index, result.plan.Decisions[index])
			}
		}
		if result.plan.Decisions[1].ReasonCode != "AMBIGUOUS_VARIANT" {
			t.Fatalf("missing variant discriminator was not distinguished: %+v", result.plan.Decisions[1])
		}
	}
}

func TestP15R03OrderedAggrandizementsRoundTrip(t *testing.T) {
	const id = "12345678-1234-4234-8234-123456789abc"
	coerce := map[string]any{"Type": "WFCoercionVariableAggrandizement", "CoercionItemClass": "WFDictionaryContentItem"}
	property := map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name"}
	key := map[string]any{"Type": "WFDictionaryValueVariableAggrandizement", "DictionaryKey": "payload"}
	for name, chain := range map[string][]any{
		"coercion":               {coerce},
		"property":               {property},
		"ordered-chain":          {coerce, key, property},
		"property-future-fields": {map[string]any{"Type": "WFPropertyVariableAggrandizement", "PropertyName": "Name", "P15Future": []any{int64(0), false}}},
	} {
		t.Run(name, func(t *testing.T) {
			root := baseTestHeader()
			root["WFWorkflowActions"] = []any{
				p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": "source", "UUID": id, "CustomOutputName": "Synthetic output"}),
				p15AcceptanceAction("is.workflow.actions.setvariable", map[string]any{"WFVariableName": "result", "WFInput": p15AcceptanceRef(id, chain)}),
			}
			for _, result := range p15NativeImportRoundTrip(t, name, root) {
				token := result.compiled["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)["Value"].(map[string]any)
				// Compare against the independently decoded input, including ordered
				// dictionaries whose native numeric category can differ from Go literals.
				wantDoc, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
				if err != nil {
					t.Fatal(err)
				}
				want := wantDoc.RawRoot["WFWorkflowActions"].([]any)[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFInput"].(map[string]any)["Value"].(map[string]any)
				if !plistValuesStrictlyEqual(want["Aggrandizements"], token["Aggrandizements"]) || token["OutputUUID"] != id {
					t.Fatal("native transformations or producer identity changed")
				}
			}
		})
	}
}

func TestP15R04ImportedOmissionAndExplicitValues(t *testing.T) {
	root := baseTestHeader()
	root["WFWorkflowActions"] = []any{
		p15AcceptanceAction("is.workflow.actions.notification", map[string]any{"WFNotificationActionBody": "sound omitted"}),
		p15AcceptanceAction("is.workflow.actions.notification", map[string]any{"WFNotificationActionBody": "sound false", "WFNotificationActionSound": false}),
		p15AcceptanceAction("is.workflow.actions.delay", map[string]any{"WFDelayTime": int64(0)}),
		p15AcceptanceAction("is.workflow.actions.delay", map[string]any{}),
		p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{"WFTextActionText": ""}),
		p15AcceptanceAction("is.workflow.actions.gettext", map[string]any{}),
		p15AcceptanceAction("com.example.p15.presence", map[string]any{"False": false, "Zero": int64(0), "EmptyText": "", "EmptyList": []any{}, "EmptyMap": map[string]any{}}),
		p15AcceptanceAction("is.workflow.actions.exit", map[string]any{}),
		map[string]any{"WFWorkflowActionIdentifier": "is.workflow.actions.exit"},
	}
	for _, result := range p15NativeImportRoundTrip(t, "omission-presence", root) {
		actions := result.compiled["WFWorkflowActions"].([]any)
		params := func(i int) map[string]any {
			return actions[i].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)
		}
		if _, exists := params(0)["WFNotificationActionSound"]; exists {
			t.Fatal("omitted sound default-filled")
		}
		if value, exists := params(1)["WFNotificationActionSound"]; !exists || value != false {
			t.Fatal("explicit false lost")
		}
		if value, exists := params(2)["WFDelayTime"]; !exists || fmt.Sprint(value) != "0" {
			t.Fatal("explicit zero lost")
		}
		if _, exists := params(3)["WFDelayTime"]; exists {
			t.Fatal("omitted number default-filled")
		}
		if value, exists := params(4)["WFTextActionText"]; !exists || value != "" {
			t.Fatal("explicit empty Text lost")
		}
		if _, exists := params(5)["WFTextActionText"]; exists {
			t.Fatal("omitted Text filled")
		}
		if _, exists := params(6)["Absent"]; exists {
			t.Fatal("absent field invented")
		}
		if value, ok := params(6)["EmptyList"].([]any); !ok || len(value) != 0 {
			t.Fatal("empty List changed")
		}
		if value, ok := params(6)["EmptyMap"].(map[string]any); !ok || len(value) != 0 {
			t.Fatal("empty Map changed")
		}
		if _, exists := actions[7].(map[string]any)["WFWorkflowActionParameters"]; !exists {
			t.Fatal("empty parameters lost")
		}
		if _, exists := actions[8].(map[string]any)["WFWorkflowActionParameters"]; exists {
			t.Fatal("missing parameters default-filled")
		}
	}
}

func TestP15NativeRoundTripOracleRejectsCorruption(t *testing.T) {
	root := p15UnknownNativeAcceptanceRoot()
	original, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"top_level_key", "nested_bytes", "large_integer", "array_order", "bool_as_zero", "empty_as_absent", "producer_uuid_case", "consumer_uuid", "unknown_action_field", "provenance"} {
		t.Run(fault, func(t *testing.T) {
			cloned, err := DecodeNativeShortcutDocument(buildNativePlistBytes(t, root, plist.XMLFormat))
			if err != nil {
				t.Fatal(err)
			}
			actual := cloned.RawRoot
			top := actual["P15UnknownTopLevel"].(map[string]any)
			actions := actual["WFWorkflowActions"].([]any)
			first := actions[0].(map[string]any)
			params := first["WFWorkflowActionParameters"].(map[string]any)
			switch fault {
			case "top_level_key":
				delete(actual, "P15UnknownTopLevel")
			case "nested_bytes":
				top["nested"].(map[string]any)["bytes"] = []byte{0, 1, 255, 254}
			case "large_integer":
				top["ordered"].([]any)[1] = int64(9007199254740992)
			case "array_order":
				items := top["ordered"].([]any)
				items[0], items[3] = items[3], items[0]
			case "bool_as_zero":
				params["P15Payload"].(map[string]any)["values"].([]any)[1] = int64(0)
			case "empty_as_absent":
				delete(params["P15Payload"].(map[string]any), "empty")
			case "producer_uuid_case":
				params["UUID"] = strings.ToUpper(params["UUID"].(string))
			case "consumer_uuid":
				actions[1].(map[string]any)["WFWorkflowActionParameters"].(map[string]any)["WFVariable"].(map[string]any)["Value"].(map[string]any)["OutputUUID"] = "MISSING"
			case "unknown_action_field":
				delete(first, "P15UnknownActionField")
			case "provenance":
				actual[importProvenanceKey] = map[string]any{"leaked": true}
			}
			if err := p15NativePreservationError(original.RawRoot, actual); err == nil {
				t.Fatalf("accepted actual native tree corruption %s", fault)
			}
		})
	}
}

const p15V04Source = `let origin = text("P15_V04_ORIGIN")
let values = [0, 1, 9007199254740993]
let info = {"name": "synthetic", "flag": false, "zero": 0, "items": ["a", "b"], "child": {"active": true}, "output": origin}
let listCount = count(values)
let mapCount = count(info)
let alias = origin
output(f"{listCount}:{mapCount}:{alias}")
`

func p15V04DictionaryFields(value any) (map[string]map[string]any, error) {
	outer, ok := value.(map[string]any)
	if !ok || outer["WFSerializationType"] != "WFDictionaryFieldValue" {
		return nil, fmt.Errorf("not native Dictionary")
	}
	body, ok := outer["Value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("missing Dictionary body")
	}
	rows, ok := body["WFDictionaryFieldValueItems"].([]any)
	if !ok {
		return nil, fmt.Errorf("missing Dictionary rows")
	}
	fields := map[string]map[string]any{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid Dictionary row")
		}
		key := extractTokenStr(row["WFKey"])
		if key == "" || fields[key] != nil {
			return nil, fmt.Errorf("missing/duplicate key")
		}
		fields[key] = row
	}
	return fields, nil
}

func p15V04Refs(value any) []string {
	var refs []string
	var walk func(any)
	walk = func(value any) {
		switch v := value.(type) {
		case map[string]any:
			if v["Type"] == "ActionOutput" {
				if id, ok := v["OutputUUID"].(string); ok {
					refs = append(refs, id)
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(value)
	return refs
}

func p15V04BindingOracle(actions []ShortcutAction) error {
	numbers, err := p15NumericListValues(actions)
	if err != nil || !reflect.DeepEqual(numbers, []string{"0", "1", "9007199254740993"}) {
		return fmt.Errorf("Number List values/order: %v %v", numbers, err)
	}
	producers := map[string]int{}
	origin, list, dict, originCount := "", "", "", 0
	var fields map[string]map[string]any
	for i, a := range actions {
		p := a.WFWorkflowActionParameters
		if id, ok := p["UUID"].(string); ok {
			if _, duplicate := producers[id]; duplicate {
				return fmt.Errorf("duplicate producer")
			}
			producers[id] = i
		}
		switch a.WFWorkflowActionIdentifier {
		case "is.workflow.actions.gettext":
			if extractTokenStr(p["WFTextActionText"]) == "P15_V04_ORIGIN" {
				originCount++
				origin, _ = p["UUID"].(string)
			}
		case "is.workflow.actions.list":
			list, _ = p["UUID"].(string)
		case "is.workflow.actions.dictionary":
			dict, _ = p["UUID"].(string)
			fields, err = p15V04DictionaryFields(p["WFItems"])
			if err != nil {
				return err
			}
		}
	}
	if originCount != 1 || origin == "" || list == "" || dict == "" || len(fields) != 6 {
		return fmt.Errorf("missing/reexecuted typed producers: origin=%d dictionaryFields=%d", originCount, len(fields))
	}
	if p15Int(fields["name"]["WFItemType"]) != 0 || extractTokenStr(fields["name"]["WFValue"]) != "synthetic" {
		return fmt.Errorf("Text field changed")
	}
	flag, ok := fields["flag"]["WFValue"].(map[string]any)
	if !ok || p15Int(fields["flag"]["WFItemType"]) != 4 || flag["Value"] != false {
		return fmt.Errorf("Bool false field changed")
	}
	if p15Int(fields["zero"]["WFItemType"]) != 3 || extractTokenStr(fields["zero"]["WFValue"]) != "0" {
		return fmt.Errorf("Number zero field changed")
	}
	if !reflect.DeepEqual(p15V04Refs(fields["output"]), []string{origin}) {
		return fmt.Errorf("Map action-output edge changed")
	}
	child, err := p15V04DictionaryFields(fields["child"]["WFValue"])
	if err != nil || p15Int(fields["child"]["WFItemType"]) != 1 || len(child) != 1 {
		return fmt.Errorf("nested Dictionary shape: %v", err)
	}
	active, ok := child["active"]["WFValue"].(map[string]any)
	if !ok || active["Value"] != true || p15Int(child["active"]["WFItemType"]) != 4 {
		return fmt.Errorf("nested Bool changed")
	}
	array, ok := fields["items"]["WFValue"].(map[string]any)
	if !ok || p15Int(fields["items"]["WFItemType"]) != 2 || array["WFSerializationType"] != "WFArrayParameterState" {
		return fmt.Errorf("nested List shape changed")
	}
	rows, ok := array["Value"].([]any)
	if !ok || len(rows) != 2 {
		return fmt.Errorf("nested List cardinality changed")
	}
	for i, want := range []string{"a", "b"} {
		row, ok := rows[i].(map[string]any)
		if !ok || extractTokenStr(row["WFValue"]) != want {
			return fmt.Errorf("nested List order/value changed")
		}
	}
	countInputs := map[string]int{}
	finalRefs := map[string]int{}
	for i, a := range actions {
		for _, ref := range p15V04Refs(a.WFWorkflowActionParameters) {
			producer, ok := producers[ref]
			if !ok || producer >= i {
				return fmt.Errorf("dangling/forward action output")
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.count" {
				countInputs[ref]++
			}
			if a.WFWorkflowActionIdentifier == "is.workflow.actions.output" {
				finalRefs[ref]++
			}
		}
	}
	if countInputs[list] != 1 || countInputs[dict] != 1 {
		return fmt.Errorf("let collection consumers do not reference original List/Dictionary")
	}
	if finalRefs[origin] != 1 {
		return fmt.Errorf("let alias consumer lost original producer")
	}
	return nil
}

func TestP15V04CollectionAndOutputBindings(t *testing.T) {
	actions := compileDebugFixture(t, p15V04Source, false)
	if err := p15V04BindingOracle(actions); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range []string{"raw_number", "false_to_true", "nested_order", "dangling_alias", "reexecuted_origin"} {
		t.Run(mutation, func(t *testing.T) {
			raw, err := plist.Marshal(actions, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			var broken []ShortcutAction
			if _, err := plist.Unmarshal(raw, &broken); err != nil {
				t.Fatal(err)
			}
			for i := range broken {
				a := &broken[i]
				switch {
				case mutation == "raw_number" && a.WFWorkflowActionIdentifier == "is.workflow.actions.list":
					a.WFWorkflowActionParameters["WFItems"].([]any)[0].(map[string]any)["WFValue"] = int64(0)
				case (mutation == "false_to_true" || mutation == "nested_order") && a.WFWorkflowActionIdentifier == "is.workflow.actions.dictionary":
					fields, err := p15V04DictionaryFields(a.WFWorkflowActionParameters["WFItems"])
					if err != nil {
						t.Fatal(err)
					}
					if mutation == "false_to_true" {
						fields["flag"]["WFValue"].(map[string]any)["Value"] = true
					} else {
						rows := fields["items"]["WFValue"].(map[string]any)["Value"].([]any)
						rows[0], rows[1] = rows[1], rows[0]
					}
				case mutation == "dangling_alias" && a.WFWorkflowActionIdentifier == "is.workflow.actions.output":
					var corrupt func(any)
					corrupt = func(v any) {
						switch value := v.(type) {
						case map[string]any:
							if value["Type"] == "ActionOutput" {
								value["OutputUUID"] = "MISSING"
							}
							for _, child := range value {
								corrupt(child)
							}
						case []any:
							for _, child := range value {
								corrupt(child)
							}
						}
					}
					corrupt(a.WFWorkflowActionParameters)
				}
			}
			if mutation == "reexecuted_origin" {
				for _, a := range actions {
					if a.WFWorkflowActionIdentifier == "is.workflow.actions.gettext" && extractTokenStr(a.WFWorkflowActionParameters["WFTextActionText"]) == "P15_V04_ORIGIN" {
						broken = append(broken, a)
						break
					}
				}
			}
			if err := p15V04BindingOracle(broken); err == nil {
				t.Fatalf("accepted %s", mutation)
			}
		})
	}
}
