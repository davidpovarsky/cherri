package main

// Export the complete, unchanged loop scenarios for a separately owned Apple
// run. The existing graph oracle must accept these exact bytes before export.
// This test neither observes native execution nor grants an acceptance tier.
import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	"howett.net/plist"
)

const p15NativeLoopFixtureSchema = "cherri-native-loop-fixture/1"

type p15NativeLoopFixture struct {
	Schema              string   `json:"schema"`
	Requirement         string   `json:"requirement"`
	CaseID              string   `json:"caseId"`
	OriginalUnitTest    string   `json:"originalUnitTest"`
	OriginalNativeTest  string   `json:"originalNativeTest"`
	OriginalGraphOracle string   `json:"originalGraphOracle"`
	CompileFilename     string   `json:"compileFilename"`
	SourceBase64        string   `json:"sourceBase64"`
	SourceSHA256        string   `json:"sourceSha256"`
	SourceBytes         int      `json:"sourceBytes"`
	UnsignedBase64      string   `json:"unsignedBase64"`
	UnsignedSHA256      string   `json:"unsignedSha256"`
	UnsignedBytes       int      `json:"unsignedBytes"`
	ActionCount         int      `json:"actionCount"`
	WorkflowName        string   `json:"workflowName"`
	WorkflowNamePresent bool     `json:"workflowNamePresent"`
	ObservationMode     string   `json:"observationMode"`
	ExpectedDisplays    []string `json:"expectedDisplays"`
	NativeExecuted      bool     `json:"nativeExecuted"`
	FormalAccepted      bool     `json:"formalAcceptanceGranted"`
}

func p15NativeLoopSHA(raw []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(raw))
}

func TestP15NativeLoopContractFixtureExport(t *testing.T) {
	for _, tc := range []struct {
		id, unit, native, sourceSHA string
		displays                    []string
	}{
		{"F02", "TestP15F02LoopScopeUnit", "TestP15F02LoopNativeGraph", "1246d05cfdf2b9da82ecb8e29a5a0b44404e15cd1079fe63077962728e88fd39", []string{
			"F02_PAIR:A:1:0:0",
			"F02_PAIR:A:2:0:1",
			"F02_AFTER:A:0",
			"F02_PAIR:B:1:1:0",
			"F02_PAIR:B:2:1:1",
			"F02_AFTER:B:1",
			"F02_REPEAT:0",
			"F02_REPEAT:1",
		}},
		{"RP22", "TestP15RP22LoopBindingsUnit", "TestP15RP22LoopNativeGraph", "951c2c2db1b72bb97ca0a64a5e14a87b2529e5c786794272f1e54df6d0425e8b", []string{
			"RP22_OUTER:0:A",
			"RP22_REPEAT:0:A",
			"RP22_INNER:0:1",
			"RP22_INNER:1:2",
			"RP22_REPEAT_AFTER:0:A",
			"RP22_REPEAT:1:A",
			"RP22_INNER:0:1",
			"RP22_INNER:1:2",
			"RP22_REPEAT_AFTER:1:A",
			"RP22_OUTER_AFTER:0:A",
			"RP22_OUTER:1:B",
			"RP22_REPEAT:0:B",
			"RP22_INNER:0:1",
			"RP22_INNER:1:2",
			"RP22_REPEAT_AFTER:0:B",
			"RP22_REPEAT:1:B",
			"RP22_INNER:0:1",
			"RP22_INNER:1:2",
			"RP22_REPEAT_AFTER:1:B",
			"RP22_OUTER_AFTER:1:B",
			"RP22_RESTORED:99:outside",
		}},
		{"BRG12", "TestBackendRecovery12LoopIdentityUnit", "TestBackendRecovery12LoopNativeGraph", "4239cc938ac02c736c1fd19aeb3d9e1c0a1d74f9f417be2236111b5a1d506d53", []string{
			"BRG12_PAIR:A:1:0:0",
			"BRG12_PAIR:A:2:0:1",
			"BRG12_MIXED:A:0:0",
			"BRG12_MIXED:A:0:1",
			"BRG12_AFTER:A:0",
			"BRG12_PAIR:B:1:1:0",
			"BRG12_PAIR:B:2:1:1",
			"BRG12_MIXED:B:1:0",
			"BRG12_MIXED:B:1:1",
			"BRG12_AFTER:B:1",
			"BRG12_SIBLING:C:0",
			"BRG12_SIBLING:D:1",
			"BRG12_REVERSE:0:0:X",
			"BRG12_REVERSE:0:1:Y",
			"BRG12_REVERSE_AFTER:0",
			"BRG12_REVERSE:1:0:X",
			"BRG12_REVERSE:1:1:Y",
			"BRG12_REVERSE_AFTER:1",
			"BRG12_SHADOW_INNER:0:S",
			"BRG12_SHADOW_INNER:1:S",
			"BRG12_SHADOW_OUTER:0:S",
			"BRG12_RESTORED:77:outside",
		}},
	} {
		t.Run(tc.id, func(t *testing.T) {
			var source = p15LoopSource(tc.id)
			if p15NativeLoopSHA([]byte(source)) != tc.sourceSHA {
				t.Fatal("original loop source changed; revise its contract explicitly")
			}
			const filename = "p15-short-circuit.cherri"
			var raw, err = CompileSourceToPlist(filename, source)
			if err != nil {
				t.Fatal(err)
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
			if err = p15LoopNativeError(tc.id, actions); err != nil {
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
			var fixture = p15NativeLoopFixture{
				Schema: p15NativeLoopFixtureSchema, Requirement: tc.id,
				CaseID: "full-original-scenario", OriginalUnitTest: tc.unit,
				OriginalNativeTest: tc.native, OriginalGraphOracle: "p15LoopNativeError",
				CompileFilename: filename, SourceBase64: base64.StdEncoding.EncodeToString([]byte(source)),
				SourceSHA256: p15NativeLoopSHA([]byte(source)), SourceBytes: len([]byte(source)),
				UnsignedBase64: base64.StdEncoding.EncodeToString(raw),
				UnsignedSHA256: p15NativeLoopSHA(raw), UnsignedBytes: len(raw),
				ActionCount: len(actions), WorkflowName: name, WorkflowNamePresent: namePresent,
				ObservationMode:  "ordered-original-show-result-sequence",
				ExpectedDisplays: tc.displays, NativeExecuted: false, FormalAccepted: false,
			}
			var encoded, encodeError = json.Marshal(fixture)
			if encodeError != nil {
				t.Fatal(encodeError)
			}
			t.Logf("P15_NATIVE_LOOP_FIXTURE sha256=%s bytes=%d base64=%s",
				p15NativeLoopSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
