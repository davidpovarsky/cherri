package protocol

import (
	"encoding/json"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"testing"
)

func TestCapabilitiesExposeCompilerMacrosWithoutInflatingNativeCatalog(t *testing.T) {
	reg := schema.DefaultRegistry()
	caps := BuildCapabilities(reg)
	if caps.ActionCount != 505 || caps.SchemaFingerprint != reg.Fingerprint() {
		t.Fatal("native contract changed")
	}
	if caps.MacroFingerprint != reg.CompilerMacroFingerprint() {
		t.Fatal("macro fingerprint not bound to schema")
	}
	if len(caps.CompilerMacros) != len(reg.CompilerMacros()) {
		t.Fatal("capabilities omitted compiler macros")
	}
	data, err := json.Marshal(caps)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err = json.Unmarshal(data, &wire); err != nil {
		t.Fatal(err)
	}
	if wire["macroFingerprint"] != caps.MacroFingerprint {
		t.Fatal("macro provenance missing on wire")
	}
	macros, ok := wire["compilerMacros"].([]any)
	if !ok || len(macros) == 0 {
		t.Fatal("wire macro descriptors missing")
	}
	found := false
	for _, m := range macros {
		obj := m.(map[string]any)
		if obj["callableName"] == "measure" {
			found = true
			if obj["scopedBlock"] != true {
				t.Fatal("V2 block requirement not exposed")
			}
		}
	}
	if !found {
		t.Fatal("measure not discoverable")
	}
}
