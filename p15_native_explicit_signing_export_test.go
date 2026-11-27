package main

// Export both original BRG26 A-then-B compilations while the actual ambient
// paths, name and workflow still point to A. Live signing/native execution is
// a separate stage and cannot be granted by these byte-preservation fixtures.
import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"howett.net/plist"
)

func p15NativeSigningSHA(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }

func p15NativeSigningBlob(t *testing.T, raw []byte, mode, literal string) map[string]any {
	t.Helper()
	p15BRG26NativeLiteral(t, raw, mode, literal)
	var document map[string]any
	if _, err := plist.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	actions, ok := document["WFWorkflowActions"].([]any)
	if !ok {
		t.Fatal("missing original action list")
	}
	nameValue, namePresent := document["WFWorkflowName"]
	name := ""
	if namePresent {
		var ok bool
		name, ok = nameValue.(string)
		if !ok {
			t.Fatal("invalid original workflow name")
		}
	}
	return map[string]any{"base64": base64.StdEncoding.EncodeToString(raw), "sha256": p15NativeSigningSHA(raw), "bytes": len(raw), "actionCount": len(actions), "workflowNamePresent": namePresent, "workflowName": name}
}

func TestP15NativeExplicitSigningFixtureExport(t *testing.T) {
	authority, err := os.ReadFile("p15_signing_explicit_joined_test.go")
	if err != nil || len(authority) != 13121 || p15NativeSigningSHA(authority) != "77b8616bbcc6dc93c5edd33ff9cfd2acffba31f553e5e60e7785e7a00e3f8e29" {
		t.Fatal("original complete signing producer changed", err)
	}
	for _, mode := range []string{"v2", "legacy"} {
		t.Run(mode, func(t *testing.T) {
			a, b, directory, input, output := p15BRG26StaleState(t, mode)
			before := p15DirectoryDigest(t, directory)
			if basename != "BRG26_STALE_A_NAME" || workflowName != "BRG26_STALE_A_WORKFLOW" ||
				filepath.Base(inputPath) != "A earlier.shortcut" || filepath.Base(outputPath) != "A stale output.shortcut" ||
				filepath.Base(input) != "B current unsigned.shortcut" || filepath.Base(output) != p15BRG26Name+".shortcut" {
				t.Fatal("actual stale A/explicit B state was not retained")
			}
			earlier, err := os.ReadFile(inputPath)
			if err != nil || !bytes.Equal(earlier, a) {
				t.Fatal("actual stale A path differs", err)
			}
			current, err := os.ReadFile(input)
			if err != nil || !bytes.Equal(current, b) || bytes.Equal(a, b) {
				t.Fatal("actual explicit B file differs", err)
			}
			existing, err := os.ReadFile(output)
			if err != nil || string(existing) != "BRG26_EXISTING_DESTINATION" {
				t.Fatal("original existing output sentinel differs", err)
			}
			ambient, err := plist.Marshal(shortcut, plist.XMLFormat)
			if err != nil {
				t.Fatal(err)
			}
			sourceA := []byte("text(" + fmt.Sprintf("%q", p15BRG26Earlier) + ")\n")
			sourceB := []byte("text(" + fmt.Sprintf("%q", p15BRG26Current) + ")\n")
			source := func(raw []byte) map[string]any {
				return map[string]any{"base64": base64.StdEncoding.EncodeToString(raw), "sha256": p15NativeSigningSHA(raw), "bytes": len(raw)}
			}
			final := map[string]any{"kind": "Text", "presence": "one-item", "value": p15BRG26Current}
			if mode == "v2" {
				final = map[string]any{"kind": "None", "presence": "absent", "items": 0}
			}
			fixture := map[string]any{
				"schema": "cherri-native-explicit-signing-fixture/1", "requirement": "BRG26", "caseId": mode, "dialect": mode,
				"originalHelper": "p15BRG26StaleState", "originalGraphOracle": "p15BRG26NativeLiteral",
				"sourceAuthorityPath": "p15_signing_explicit_joined_test.go", "sourceAuthoritySha256": p15NativeSigningSHA(authority),
				"originalProductionTests": []string{"TestBackendRecovery26ExplicitBytesAfterSequentialCompilation", "TestBackendRecovery26ExplicitFileAfterSequentialCompilation", "TestBackendRecovery26MobileAfterSequentialCompilation"},
				"compileOrder":            []string{"A", "B"}, "earlierName": "BRG26 A earlier", "currentName": p15BRG26Name,
				"earlierSource": source(sourceA), "currentSource": source(sourceB),
				"earlierUnsigned":      p15NativeSigningBlob(t, a, mode, p15BRG26Earlier),
				"currentUnsigned":      p15NativeSigningBlob(t, b, mode, p15BRG26Current),
				"ambientUnsigned":      p15NativeSigningBlob(t, ambient, mode, p15BRG26Earlier),
				"ambientInputBasename": filepath.Base(inputPath), "ambientOutputBasename": filepath.Base(outputPath),
				"ambientBasename": basename, "ambientWorkflowName": workflowName,
				"explicitInputBasename": filepath.Base(input), "explicitOutputBasename": filepath.Base(output),
				"existingDestination":  string(existing),
				"expectedObservations": map[string]any{"intermediateText": p15BRG26Current, "finalReturn": final, "earlierTextForbidden": p15BRG26Earlier, "originalV2FallthroughPreserved": mode == "v2"},
				"liveSigningExecuted":  false, "nativeExecuted": false, "formalAcceptanceGranted": false,
			}
			encoded, err := json.Marshal(fixture)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, p15DirectoryDigest(t, directory)) {
				t.Fatal("export mutated A, B or existing destination")
			}
			t.Logf("P15_NATIVE_EXPLICIT_SIGNING_FIXTURE sha256=%s bytes=%d base64=%s", p15NativeSigningSHA(encoded), len(encoded), base64.StdEncoding.EncodeToString(encoded))
		})
	}
}
