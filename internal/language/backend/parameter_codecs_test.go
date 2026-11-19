package backend_test

import (
	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
	"testing"
)

func TestCanonicalParameterCodecVocabulary(t *testing.T) {
	seen := map[string]bool{}
	for _, action := range schema.DefaultRegistry().AllActions() {
		for _, parameter := range action.Parameters {
			if parameter.Codec == "" {
				continue
			}
			if !backend.IsRegisteredParameterCodec(parameter.Codec) {
				t.Fatalf("generated canonical codec %q is not registered (%s.%s)", parameter.Codec, action.CallableName, parameter.Label)
			}
			seen[parameter.Codec] = true
		}
	}
	for _, name := range []string{"text", "textToken", "number", "bool", "dictionary", "array", "token", "enum"} {
		if !backend.IsRegisteredParameterCodec(name) {
			t.Fatal("existing codec rejected", name)
		}
	}
	if len(seen) < 7 {
		t.Fatal("canonical vocabulary test did not inspect generated action parameters", seen)
	}
	for _, name := range []string{"", "Text", "unknown", "text ", "P15_MISSING_CODEC"} {
		if backend.IsRegisteredParameterCodec(name) {
			t.Fatal("unregistered codec accepted", name)
		}
	}
}
