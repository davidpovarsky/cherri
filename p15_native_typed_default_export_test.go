package main

// Export complete original typed-default and parameter-form programs. The
// existing full graph oracles remain authoritative; this is not native evidence.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"

	"howett.net/plist"
)

const p15NativeTypedCatalogPath = "tests/native-runtime/typed-default-original-cases.json"
const p15NativeTypedCatalogSHA = "89312716103525e1744de81da556d92f7b14b393ffa0ad686e41954b6e4f8786"

func p15NativeTypedSHA(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func p15NativeTypedExport(t *testing.T, spec map[string]any, source string, verify func([]map[string]any) (*p15FTCursor, error)) {
	t.Helper()
	if spec["source"] != source || spec["compileFilename"] != "p15-function-transport.cherri" {
		t.Fatal("original full typed source differs")
	}
	raw, err := CompileSourceToPlist("p15-function-transport.cherri", source)
	if err != nil {
		t.Fatal(err)
	}
	var workflow map[string]any
	if _, err = plist.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	rows, ok := workflow["WFWorkflowActions"].([]any)
	if !ok || len(rows) == 0 {
		t.Fatal("original full action list missing")
	}
	actions := make([]map[string]any, len(rows))
	for i, row := range rows {
		action, valid := row.(map[string]any)
		if !valid {
			t.Fatal("invalid original action envelope", i)
		}
		actions[i] = action
	}
	cursor, err := verify(actions)
	if err != nil {
		t.Fatal(err)
	}
	points := map[string]any{}
	for name, index := range cursor.points {
		if index < 0 || index >= len(actions) {
			t.Fatal("original oracle point outside workflow", name, index)
		}
		parameters, _ := actions[index]["WFWorkflowActionParameters"].(map[string]any)
		uuidValue, present := parameters["UUID"]
		uuid := ""
		if present {
			var valid bool
			uuid, valid = uuidValue.(string)
			if !valid {
				t.Fatal("original point UUID is not text", name)
			}
		}
		points[name] = map[string]any{"index": index, "identifier": actions[index]["WFWorkflowActionIdentifier"], "uuidPresent": present, "uuid": uuid}
	}
	name := ""
	nameValue, namePresent := workflow["WFWorkflowName"]
	if namePresent {
		var valid bool
		name, valid = nameValue.(string)
		if !valid {
			t.Fatal("invalid original workflow name")
		}
	}
	authority, err := os.ReadFile(spec["sourceAuthorityPath"].(string))
	if err != nil {
		t.Fatal(err)
	}
	sourceBytes := []byte(source)
	fixture := map[string]any{
		"schema": "cherri-native-typed-default-fixture/1", "dialect": "v2",
		"requirement": spec["requirement"], "caseId": spec["caseId"],
		"catalogPath": p15NativeTypedCatalogPath, "catalogSha256": p15NativeTypedCatalogSHA,
		"sourceAuthorityPath": spec["sourceAuthorityPath"], "sourceAuthoritySha256": p15NativeTypedSHA(authority),
		"originalUnitTest": spec["originalUnitTest"], "originalNativeTest": spec["originalNativeTest"],
		"originalSourceHelper": spec["originalSourceHelper"], "originalGraphOracle": spec["originalGraphOracle"],
		"compileFilename": spec["compileFilename"], "sourceCase": spec["sourceCase"],
		"sourceBase64": base64.StdEncoding.EncodeToString(sourceBytes), "sourceSha256": p15NativeTypedSHA(sourceBytes), "sourceBytes": len(sourceBytes),
		"unsignedBase64": base64.StdEncoding.EncodeToString(raw), "unsignedSha256": p15NativeTypedSHA(raw), "unsignedBytes": len(raw),
		"actionCount": len(actions), "actionPoints": points, "workflowName": name, "workflowNamePresent": namePresent,
		"observationMode": spec["observationMode"], "expectedLanguageSemantics": spec["expectedLanguageSemantics"],
		"requiredEffects": spec["requiredEffects"], "nativeExecuted": false, "rawNativeTypesObserved": false,
		"rawNativeValuesObserved": false, "formalAcceptanceGranted": false,
	}
	encoded, err := json.Marshal(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("P15_NATIVE_TYPED_DEFAULT_FIXTURE sha256=%s bytes=%d base64=%s", p15NativeTypedSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
}

func TestP15NativeTypedDefaultFixtureExport(t *testing.T) {
	raw, err := os.ReadFile(p15NativeTypedCatalogPath)
	if err != nil || p15NativeTypedSHA(raw) != p15NativeTypedCatalogSHA {
		t.Fatal("original typed case catalogue changed", err)
	}
	var catalog struct {
		Schema      string            `json:"schema"`
		Authorities map[string]string `json:"authorities"`
		Cases       []map[string]any  `json:"cases"`
	}
	if err = json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != "cherri-native-typed-default-catalog/1" || len(catalog.Cases) != 14 || len(catalog.Authorities) != 3 {
		t.Fatal("original typed case inventory differs")
	}
	for path, digest := range catalog.Authorities {
		original, readErr := os.ReadFile(path)
		if readErr != nil || p15NativeTypedSHA(original) != digest {
			t.Fatal("original complete typed authority changed", path, readErr)
		}
	}
	originalCases := p15RP25Cases()
	if len(originalCases) != 13 {
		t.Fatal("original defaults case count differs")
	}
	for i, tc := range originalCases {
		spec := catalog.Cases[i]
		if spec["id"] != "RP25/"+tc.name || spec["requirement"] != "RP25" || spec["caseId"] != tc.name ||
			spec["originalSourceHelper"] != "p15RP25Source" || spec["originalGraphOracle"] != "p15RP25Graph" ||
			!reflect.DeepEqual(spec["sourceCase"], map[string]any{"type": tc.typ, "supplied": tc.supplied, "fallback": tc.fallback, "optional": tc.optional, "fallbackPresent": tc.fallbackPresent}) {
			t.Fatal("original typed case binding differs", tc.name)
		}
		t.Run("RP25/"+tc.name, func(t *testing.T) {
			p15NativeTypedExport(t, spec, p15RP25Source(tc), func(a []map[string]any) (*p15FTCursor, error) { return p15RP25Graph(a, tc) })
		})
	}
	spec := catalog.Cases[13]
	if spec["id"] != "BRG11/full-original-scenario" || spec["requirement"] != "BRG11" ||
		spec["caseId"] != "full-original-scenario" || spec["originalSourceHelper"] != "p15BRG11Source" || spec["originalGraphOracle"] != "p15BRG11Graph" {
		t.Fatal("original parameter forms case differs")
	}
	t.Run("BRG11/full-original-scenario", func(t *testing.T) { p15NativeTypedExport(t, spec, p15BRG11Source, p15BRG11Graph) })
}
