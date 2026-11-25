package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"howett.net/plist"
)

// This test exports the real compiler artifact for an independent graph/runtime
// intake. Compilation is not evidence that Apple's native carriers preserve it.
func TestP15TaggedCarrierEngineeringCompile(t *testing.T) {
	const sourcePath = "tools/shortcuts-simulator-qa/fixtures/qa-tagged-value-carrier-v2.cherri"
	const expectedPath = "tools/shortcuts-simulator-qa/fixtures/qa-tagged-value-carrier-v2.expected.json"
	const expectedSource = "8abc203ba48ba97e6c3c28240dd861e3a6b8b4a0b5ac91f7e82334ca528a46ec"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256(source)) != expectedSource {
		t.Fatal("reviewed engineering fixture changed")
	}
	contractRaw, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]any
	if err := json.Unmarshal(contractRaw, &contract); err != nil {
		t.Fatal(err)
	}
	if contract["source_sha256"] != expectedSource || contract["diagnostic_prefix"] != "CHERRI_TAGGED_VALUE_CARRIER_V1:" || contract["fixture"] != "qa-tagged-value-carrier-v2" {
		t.Fatal("engineering contract/source identity differs")
	}
	raw, err := CompileSourceToPlist(sourcePath, string(source))
	if err != nil {
		if len(raw) != 0 {
			t.Fatalf("rejected engineering source leaked native bytes: %d", len(raw))
		}
		t.Fatalf("engineering fixture compilation: %v", err)
	}
	var workflow map[string]any
	if _, err := plist.Unmarshal(raw, &workflow); err != nil {
		t.Fatal(err)
	}
	actions, okay := workflow["WFWorkflowActions"].([]any)
	if !okay || len(actions) == 0 || bytes.Contains(raw, []byte("<null")) {
		t.Fatal("compiler artifact is absent or contains fabricated plist null")
	}
	t.Logf("SOURCE_GRAPH_ONLY: %d real native actions; Apple execution and independent UUID/guard validation are still required", len(actions))
	t.Log("P15_TAGGED_CARRIER_NATIVE_BASE64:" + base64.StdEncoding.EncodeToString(raw))
}
