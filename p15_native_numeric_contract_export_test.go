package main

// Preserve every positive source in the original numeric/index/Unicode cohorts.
// Full existing semantic, graph, corruption and roundtrip tests run alongside
// this exporter. Source-derived observations are not native execution evidence.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"howett.net/plist"
)

type p15NativeNumericFixture struct {
	Schema               string         `json:"schema"`
	Requirement          string         `json:"requirement"`
	CaseID               string         `json:"caseId"`
	OriginalUnitTest     string         `json:"originalUnitTest"`
	OriginalNativeTest   string         `json:"originalNativeTest"`
	OriginalGraphOracle  string         `json:"originalGraphOracle"`
	SourcePath           string         `json:"sourcePath"`
	SourceFileSHA256     string         `json:"sourceFileSha256"`
	CompileFilename      string         `json:"compileFilename"`
	SourceBase64         string         `json:"sourceBase64"`
	SourceSHA256         string         `json:"sourceSha256"`
	SourceBytes          int            `json:"sourceBytes"`
	UnsignedBase64       string         `json:"unsignedBase64"`
	UnsignedSHA256       string         `json:"unsignedSha256"`
	UnsignedBytes        int            `json:"unsignedBytes"`
	ActionCount          int            `json:"actionCount"`
	WorkflowName         string         `json:"workflowName"`
	WorkflowNamePresent  bool           `json:"workflowNamePresent"`
	ObservationMode      string         `json:"observationMode"`
	ExpectedObservations map[string]any `json:"expectedObservations"`
	NativeExecuted       bool           `json:"nativeExecuted"`
	FormalAccepted       bool           `json:"formalAcceptanceGranted"`
}

func p15NativeNumericSHA(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

type p15NativeNumericCase struct {
	id, name, unit, native, sourcePath, sourceFileSHA, filename, oracle, source string
	observations                                                                map[string]any
	indexSpec                                                                   *p15IXCase
}

func p15NativeNumericFile(t *testing.T, name, digest string, size int) []byte {
	t.Helper()
	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw) != size || p15NativeNumericSHA(raw) != digest {
		t.Fatal("original complete source file changed", name)
	}
	return raw
}

func p15NativeNumericCases(t *testing.T) []p15NativeNumericCase {
	t.Helper()
	var cases []p15NativeNumericCase
	for _, tc := range []struct {
		id, unit, native, path, digest, filename, oracle, expect string
		size                                                     int
	}{
		{"RP15", "TestP15RP15CompoundOnceUnit", "TestP15RP15CompoundOnceNative", "tests/language-v2/v2-rp15-compound-once.cherri", "56c9afc1083655ce5b7ca7545614b71e32addeb4dae6dd0f2c104d1c5042bb26", "combined-rp15.cherri", "p15AMGraph", "{\"return\":{\"kind\":\"Number\",\"presence\":\"one-item\",\"value\":\"11\"},\"clipboard\":{\"kind\":\"NumericText\",\"value\":\"41234\",\"meaning\":\"four RHS evaluations in source order 1,2,3,4\",\"nativeTextRendering\":\"UNOBSERVED; retain exact clipboard text\"}}", 741},
		{"RP17", "TestP15RP17ComparisonsArithmeticUnit", "TestP15RP17ComparisonsArithmeticNative", "tests/language-v2/v2-rp17-comparisons-arithmetic.cherri", "e5e43ba0823ea329304ecaf9e78f6a2f302acb9489bd0245f9616bd6bd6562ca", "combined-rp17.cherri", "p15AMGraph", "{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"template\":\"gt={greater};eq={equal};ne={unequal};div={divided};mod={remainder};mixed={mixed}\"},\"intermediate\":{\"greater\":{\"kind\":\"Bool\",\"value\":\"false\"},\"equal\":{\"kind\":\"Bool\",\"value\":\"true\"},\"unequal\":{\"kind\":\"Bool\",\"value\":\"false\"},\"divided\":{\"kind\":\"Number\",\"value\":\"2.5\"},\"remainder\":{\"kind\":\"Number\",\"value\":\"1\"},\"mixed\":{\"kind\":\"Number\",\"value\":\"6.5\"}},\"nativeTextRendering\":\"UNOBSERVED; retain exact returned text and authentic original producer values\"}", 350},
		{"RP19", "TestP15RP19LiteralUnicodeUnit", "TestP15RP19LiteralUnicodeNativeRoundTrip", "tests/language-v2/v2-rp19-literal-unicode.cherri", "57ecba5206a2418b595f5f5d023dc148fe96e4531ae80029ad7383d441913158", "rp19-joined.cherri", "p15UTGraph", "{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"value\":\"שָׁלוֹם👩‍💻é|42falseαב|éα|🌍Ω\\n\"},\"textComparison\":\"exact UTF-8; no normalization\"}", 241},
		{"BRG08", "TestBackendRecovery08InlineCodecUnit", "TestBackendRecovery08InlineCodecNativeRoundTrip", "tests/language-v2/v2-brg08-inline-codec.cherri", "5b59664946c8e2ecf9841512839db6670df622538f065aa408b696fc0cb72f0d", "brg08-joined.cherri", "p15UTGraph", "{\"return\":{\"kind\":\"Text\",\"presence\":\"one-item\",\"value\":\"שָׁלוֹם👩‍💻é|{literal}|ΔΔ|éΔ|🌍{seed}|JSON:{\\\"n\\\":42}|\\\\path\\\\d+{seed}\\\\n\\n\"},\"textComparison\":\"exact UTF-8; no normalization\"}", 658},
	} {
		raw := p15NativeNumericFile(t, tc.path, tc.digest, tc.size)
		var expectations map[string]any
		if err := json.Unmarshal([]byte(tc.expect), &expectations); err != nil {
			t.Fatal(err)
		}
		cases = append(cases, p15NativeNumericCase{id: tc.id, name: "full-original-scenario", unit: tc.unit, native: tc.native, sourcePath: tc.path, sourceFileSHA: tc.digest, filename: tc.filename, oracle: tc.oracle, source: string(raw), observations: expectations})
	}
	for _, tc := range []struct {
		id, unit, native, path, digest string
		size, count                    int
	}{
		{"RP21", "TestP15RP21SafeIndexUnit", "TestP15RP21SafeIndexNative", "tests/language-v2/v2-rp21-safe-indexing.json", "88a967f31f326b8e247e657964e4a06324b8d8f58f70d6b9a1437e0a009e17ac", 3604, 18},
		{"BRG16", "TestP15BRG16NumericBoundsUnit", "TestP15BRG16NumericBoundsNative", "tests/language-v2/v2-brg16-numeric-bounds.json", "a00fa42daf2f373930f5a82adf737955e5c409fab1ab4936a6224ac38ca38bc3", 7694, 15},
	} {
		p15NativeNumericFile(t, tc.path, tc.digest, tc.size)
		fixture := p15IXLoad(t, tc.id)
		if len(fixture.Cases) != tc.count || len(p15IXCases[tc.id]) != tc.count {
			t.Fatal("original positive index inventory changed")
		}
		for i, spec := range p15IXCases[tc.id] {
			var result map[string]any
			var lookup string
			if spec.Outcome == "None" {
				result = map[string]any{"kind": "None", "presence": "absent", "items": "0"}
				lookup = "forbidden"
			} else if strings.HasPrefix(spec.Outcome, "Text:") {
				result = map[string]any{"kind": "Text", "presence": "one-item", "value": strings.TrimPrefix(spec.Outcome, "Text:")}
				lookup = "required"
			} else {
				t.Fatal("unknown original outcome", spec.Outcome)
			}
			expected := map[string]any{"return": result, "index": map[string]any{"sourceLiteral": spec.Literal, "lookup": lookup}, "absenceAndEmptyTextAreDistinct": true}
			if tc.id == "BRG16" {
				expected["clipboard"] = map[string]any{"kind": "NumericText", "value": strings.TrimSuffix(spec.Literal, ".0"), "nativeTextRendering": "UNOBSERVED; retain exact clipboard text", "meaning": "actual fractional/Modulus/compound result before indexing"}
			}
			copied := spec
			cases = append(cases, p15NativeNumericCase{id: tc.id, name: spec.Name, unit: tc.unit, native: tc.native, sourcePath: fmt.Sprintf("%s#/cases/%d/source", tc.path, i), sourceFileSHA: tc.digest, filename: "combined-" + strings.ToLower(tc.id) + "-" + spec.Name + ".cherri", oracle: "p15IXGraph", source: fixture.Cases[i].Source, observations: expected, indexSpec: &copied})
		}
	}
	if len(cases) != 37 {
		t.Fatal("incomplete original positive source inventory", len(cases))
	}
	return cases
}

func TestP15NativeNumericContractFixtureExport(t *testing.T) {
	for _, tc := range p15NativeNumericCases(t) {
		t.Run(tc.id+"/"+tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist(tc.filename, tc.source)
			if err != nil {
				t.Fatal(err)
			}
			var workflow map[string]any
			if _, err = plist.Unmarshal(raw, &workflow); err != nil {
				t.Fatal(err)
			}
			actions, err := p15UTActions(workflow)
			if err != nil || len(actions) == 0 {
				t.Fatal("complete workflow actions", err)
			}
			switch tc.oracle {
			case "p15AMGraph":
				_, err = p15AMGraph(actions, tc.id)
			case "p15UTGraph":
				_, err = p15UTGraph(workflow, tc.id)
			case "p15IXGraph":
				if tc.indexSpec == nil {
					t.Fatal("original index case missing")
				}
				_, err = p15IXGraph(actions, tc.id, *tc.indexSpec)
			default:
				t.Fatal("unknown original graph oracle")
			}
			if err != nil {
				t.Fatal(err)
			}
			var name string
			nameValue, namePresent := workflow["WFWorkflowName"]
			if namePresent {
				var ok bool
				name, ok = nameValue.(string)
				if !ok {
					t.Fatal("workflow name is not text")
				}
			}
			fixture := p15NativeNumericFixture{
				Schema: "cherri-native-numeric-fixture/1", Requirement: tc.id, CaseID: tc.name,
				OriginalUnitTest: tc.unit, OriginalNativeTest: tc.native, OriginalGraphOracle: tc.oracle,
				SourcePath: tc.sourcePath, SourceFileSHA256: tc.sourceFileSHA, CompileFilename: tc.filename,
				SourceBase64: base64.StdEncoding.EncodeToString([]byte(tc.source)), SourceSHA256: p15NativeNumericSHA([]byte(tc.source)), SourceBytes: len([]byte(tc.source)),
				UnsignedBase64: base64.StdEncoding.EncodeToString(raw), UnsignedSHA256: p15NativeNumericSHA(raw), UnsignedBytes: len(raw), ActionCount: len(actions),
				WorkflowName: name, WorkflowNamePresent: namePresent, ObservationMode: "original-typed-output-and-side-effects",
				ExpectedObservations: tc.observations, NativeExecuted: false, FormalAccepted: false,
			}
			encoded, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			t.Logf("P15_NATIVE_NUMERIC_FIXTURE sha256=%s bytes=%d base64=%s", p15NativeNumericSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
