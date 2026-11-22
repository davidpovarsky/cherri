package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

const p15OptionalUnsignedGraphMarker = "CHERRI_OPTIONAL_UNSIGNED_GRAPH_V1="
const p15OptionalUnsignedMaximumBytes = 4 * 1024 * 1024

// The independently checked current graph adds the exact marker start/end to
// the prior 400-action M02 graph. Historical 399/400-action artifacts keep their
// original compiler/source identities; this gate accepts only the new graph.
const p15OptionalPreparedActionCount = 402

// Reuse the independent complete graph oracle for the CLI artifact that will
// actually be signed. Errors reveal neither supplied file contents nor paths.
func p15OptionalPreparedUnsignedArtifact(path, expectedSHA string, contract []byte) (map[string]any, error) {
	decoded, err := hex.DecodeString(expectedSHA)
	if err != nil || len(decoded) != sha256.Size || strings.ToLower(expectedSHA) != expectedSHA {
		return nil, fmt.Errorf("prepared Optional artifact requires an exact SHA256")
	}
	if filepath.Base(path) != p15OptionalDiagnostic+".shortcut" {
		return nil, fmt.Errorf("prepared Optional artifact name mismatch")
	}
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > p15OptionalUnsignedMaximumBytes {
		return nil, fmt.Errorf("prepared Optional artifact must be a bounded regular file")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("prepared Optional artifact could not be opened")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) || !opened.Mode().IsRegular() {
		return nil, fmt.Errorf("prepared Optional artifact identity changed")
	}
	data, err := io.ReadAll(io.LimitReader(file, p15OptionalUnsignedMaximumBytes+1))
	if err != nil || len(data) == 0 || len(data) > p15OptionalUnsignedMaximumBytes || int64(len(data)) != info.Size() {
		return nil, fmt.Errorf("prepared Optional artifact size changed")
	}
	actualSHA := fmt.Sprintf("%x", sha256.Sum256(data))
	if actualSHA != expectedSHA {
		return nil, fmt.Errorf("prepared Optional artifact SHA256 mismatch")
	}
	var workflow map[string]any
	if _, err = plist.Unmarshal(data, &workflow); err != nil || workflow == nil {
		return nil, fmt.Errorf("prepared Optional artifact is not a workflow plist")
	}
	raw, err := json.Marshal(workflow["WFWorkflowActions"])
	if err != nil {
		return nil, fmt.Errorf("prepared Optional artifact actions could not be decoded")
	}
	var actions []map[string]any
	if json.Unmarshal(raw, &actions) != nil || len(actions) != p15OptionalPreparedActionCount {
		return nil, fmt.Errorf("prepared Optional artifact action count or representation changed")
	}
	if p15OptionalDiagnosticGraphError(actions) != nil {
		return nil, fmt.Errorf("prepared Optional artifact violates the independently reviewed graph")
	}
	return map[string]any{
		"schemaVersion": "1.0", "fixtureId": p15OptionalDiagnostic,
		"status": "PASS_NATIVE_STRUCTURE_ONLY", "sourceSha256": p15OptionalDiagnosticSourceSHA,
		"expectedContractSha256": fmt.Sprintf("%x", sha256.Sum256(contract)),
		"unsignedPlistSha256":    actualSHA, "nativeActionCount": len(actions),
		"appleRuntime": "NOT_RUN", "formalAcceptanceGranted": false,
	}, nil
}

func TestP15OptionalNativeDiagnosticPreparedArtifactControls(t *testing.T) {
	sourcePath := "tools/shortcuts-simulator-qa/fixtures/" + p15OptionalDiagnostic + ".cherri"
	source, err := os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	contract, err := os.ReadFile(strings.TrimSuffix(sourcePath, ".cherri") + ".expected.json")
	if err != nil {
		t.Fatal(err)
	}
	if err = p15OptionalDiagnosticContractError(contract, string(source)); err != nil {
		t.Fatal(err)
	}
	data, err := CompileSourceToPlist(sourcePath, string(source))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), p15OptionalDiagnostic+".shortcut")
	if err = os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(data))
	receipt, err := p15OptionalPreparedUnsignedArtifact(path, digest, contract)
	if err != nil || receipt["unsignedPlistSha256"] != digest || receipt["nativeActionCount"] != p15OptionalPreparedActionCount || receipt["formalAcceptanceGranted"] != false {
		t.Fatalf("real unsigned artifact control failed: %v", err)
	}
	for _, bad := range []string{"", strings.Repeat("0", 64), strings.ToUpper(digest), "not-a-digest"} {
		t.Run("digest-"+fmt.Sprint(len(bad)), func(t *testing.T) {
			if _, err = p15OptionalPreparedUnsignedArtifact(path, bad, contract); err == nil {
				t.Fatal("invalid or substituted artifact digest accepted")
			}
		})
	}
	t.Run("wrong-name", func(t *testing.T) {
		other := filepath.Join(t.TempDir(), "unrelated.shortcut")
		if err = os.WriteFile(other, data, 0600); err != nil {
			t.Fatal(err)
		}
		if _, err = p15OptionalPreparedUnsignedArtifact(other, digest, contract); err == nil {
			t.Fatal("unrelated artifact name accepted")
		}
	})
	t.Run("nonregular", func(t *testing.T) {
		directory := filepath.Join(t.TempDir(), p15OptionalDiagnostic+".shortcut")
		if err = os.Mkdir(directory, 0700); err != nil {
			t.Fatal(err)
		}
		if _, err = p15OptionalPreparedUnsignedArtifact(directory, digest, contract); err == nil {
			t.Fatal("directory accepted as artifact")
		}
	})
	var document map[string]any
	if _, err = plist.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	emptyActions, err := plist.Marshal(map[string]any{"WFWorkflowActions": []any{}}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	document["WFWorkflowActions"].([]any)[0].(map[string]any)["WFWorkflowActionIdentifier"] = "is.workflow.actions.getcontentsofurl"
	wrongGraph, err := plist.Marshal(document, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	for name, bad := range map[string][]byte{"empty": {}, "too-large": make([]byte, p15OptionalUnsignedMaximumBytes+1),
		"invalid-plist": []byte("PRIVATE_ARTIFACT_CANARY"), "zero-actions": emptyActions, "changed-graph": wrongGraph} {
		t.Run(name, func(t *testing.T) {
			if err = os.WriteFile(path, bad, 0600); err != nil {
				t.Fatal(err)
			}
			_, problem := p15OptionalPreparedUnsignedArtifact(path, fmt.Sprintf("%x", sha256.Sum256(bad)), contract)
			if problem == nil || strings.Contains(problem.Error(), "PRIVATE_ARTIFACT_CANARY") {
				t.Fatal("corrupt artifact accepted or private contents exposed")
			}
		})
	}
}
