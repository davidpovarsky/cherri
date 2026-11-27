package main

// Export the full original return/recursion and observable short-circuit
// scenarios. Their complete existing graph oracles must accept these exact
// unsigned bytes. Expected text is a contract, never an observed Apple result.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"howett.net/plist"
)

type p15NativeFlowFixture struct {
	Schema              string `json:"schema"`
	Requirement         string `json:"requirement"`
	CaseID              string `json:"caseId"`
	OriginalUnitTest    string `json:"originalUnitTest"`
	OriginalNativeTest  string `json:"originalNativeTest"`
	OriginalGraphOracle string `json:"originalGraphOracle"`
	SourcePath          string `json:"sourcePath"`
	CompileFilename     string `json:"compileFilename"`
	SourceBase64        string `json:"sourceBase64"`
	SourceSHA256        string `json:"sourceSha256"`
	SourceBytes         int    `json:"sourceBytes"`
	UnsignedBase64      string `json:"unsignedBase64"`
	UnsignedSHA256      string `json:"unsignedSha256"`
	UnsignedBytes       int    `json:"unsignedBytes"`
	ActionCount         int    `json:"actionCount"`
	WorkflowName        string `json:"workflowName"`
	WorkflowNamePresent bool   `json:"workflowNamePresent"`
	ObservationMode     string `json:"observationMode"`
	ExpectedType        string `json:"expectedType"`
	ExpectedText        string `json:"expectedText"`
	NativeExecuted      bool   `json:"nativeExecuted"`
	FormalAccepted      bool   `json:"formalAcceptanceGranted"`
}

func p15NativeFlowSHA(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func TestP15NativeFlowContractFixtureExport(t *testing.T) {
	for _, tc := range []struct {
		id, unit, native, path, sourceSHA, filename, oracle, expected string
		sourceBytes                                                   int
		typed                                                         bool
	}{
		{
			"RP24", "TestP15RP24CombinedReturnRecursionUnit", "TestP15RP24CombinedReturnRecursionNative",
			"tests/language-v2/v2-rp24-return-recursion.cherri", "ff0ab9a41e1cffeea4860ce185a176e8430d8b9dee4520216994191c7ab6cc04",
			"p15-function-transport.cherri", "p15RP24Graph", "10|6|7", 303, false,
		},
		{
			"RP18", "TestP15RP18ObservableShortCircuitUnit", "TestP15RP18ObservableShortCircuitNative",
			"tests/language-v2/v2-rp18-observable-short-circuit.cherri", "fd40f18e0a8711a2a21fa90e417b60acf057706f82698eeb3c4da774cee1504b",
			"p15-short-circuit.cherri", "p15SCGraph",
			"P15_RP18_LOCAL_OK:calls=12:order=|AND_TF|AND_TT|OR_FF|OR_FT|MIX_A|MIX_B|MIX_C|MIX_D|MIX_E|MIX_F|NEED_H|NEED_J", 4037, false,
		},
		{
			"BRG13", "TestBackendRecovery13TypedShortCircuitUnit", "TestBackendRecovery13TypedShortCircuitNative",
			"tests/language-v2/v2-brg13-typed-short-circuit.cherri", "4d445f0548ee8a5432582582fcabf8a3f00b9f49bcdfe8182bccf2a27fa1e396",
			"p15-short-circuit.cherri", "p15SCGraph",
			"P15_BRG13_LOCAL_OK:calls=12:order=|AND_TF|AND_TT|OR_FF|OR_FT|MIX_A|MIX_B|MIX_C|MIX_D|MIX_E|MIX_F|NEED_H|NEED_J", 5364, true,
		},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var source, err = os.ReadFile(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			if len(source) != tc.sourceBytes || p15NativeFlowSHA(source) != tc.sourceSHA {
				t.Fatal("original full source changed; revise its contract explicitly")
			}
			var raw, compileErr = CompileSourceToPlist(tc.filename, string(source))
			if compileErr != nil {
				t.Fatal(compileErr)
			}
			var workflow map[string]any
			if _, err = plist.Unmarshal(raw, &workflow); err != nil {
				t.Fatal(err)
			}
			var items, ok = workflow["WFWorkflowActions"].([]any)
			if !ok || len(items) == 0 {
				t.Fatal("original full workflow has no actions")
			}
			var actions = make([]map[string]any, len(items))
			for i, item := range items {
				var action, valid = item.(map[string]any)
				if !valid {
					t.Fatalf("action %d is not a native envelope", i)
				}
				actions[i] = action
			}
			if tc.id == "RP24" {
				_, err = p15RP24Graph(actions)
			} else {
				_, err = p15SCGraph(actions, tc.typed)
			}
			if err != nil {
				t.Fatal(err)
			}
			var name string
			var nameValue, namePresent = workflow["WFWorkflowName"]
			if namePresent {
				var valid bool
				name, valid = nameValue.(string)
				if !valid {
					t.Fatal("workflow name is not text")
				}
			}
			var fixture = p15NativeFlowFixture{
				Schema: "cherri-native-flow-fixture/1", Requirement: tc.id,
				CaseID: "full-original-scenario", OriginalUnitTest: tc.unit,
				OriginalNativeTest: tc.native, OriginalGraphOracle: tc.oracle,
				SourcePath: tc.path, CompileFilename: tc.filename,
				SourceBase64: base64.StdEncoding.EncodeToString(source),
				SourceSHA256: p15NativeFlowSHA(source), SourceBytes: len(source),
				UnsignedBase64: base64.StdEncoding.EncodeToString(raw),
				UnsignedSHA256: p15NativeFlowSHA(raw), UnsignedBytes: len(raw),
				ActionCount: len(actions), WorkflowName: name, WorkflowNamePresent: namePresent,
				ObservationMode: "exact-original-returned-text", ExpectedType: "Text",
				ExpectedText: tc.expected, NativeExecuted: false, FormalAccepted: false,
			}
			var encoded, encodeErr = json.Marshal(fixture)
			if encodeErr != nil {
				t.Fatal(encodeErr)
			}
			t.Logf("P15_NATIVE_FLOW_FIXTURE sha256=%s bytes=%d base64=%s",
				p15NativeFlowSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
