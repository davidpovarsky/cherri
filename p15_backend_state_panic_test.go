package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/backend"
)

// This is an actual emission panic after a first action has been appended. The
// mobile public boundary must recover, discard partial output and release its
// state ownership before the following canonical and mobile requests.
func TestBackendRecovery06MobilePanicRecovery(t *testing.T) {
	var previousHook = backendEmitHook
	defer func() { backendEmitHook = previousHook }()
	var calls int
	backendEmitHook = func(call backend.ResolvedCall) error {
		calls++
		if calls == 2 {
			panic("P15_EMITTER_PANIC_SENTINEL")
		}
		return nil
	}
	var response = compileForMobileMode("text(\"P15_FIRST\")\ntext(\"P15_SECOND\")\ntext(\"P15_NEVER\")\n", "P15_Panic_Recovery", "v2", false)
	backendEmitHook = previousHook
	if response.OK || !strings.Contains(response.Error, "P15_EMITTER_PANIC_SENTINEL") || response.PlistBase64 != "" || response.SignedBase64 != "" || calls != 2 {
		t.Fatalf("mobile panic was swallowed, returned partial bytes or continued: calls=%d response=%#v", calls, response)
	}
	if err := p15StateCompile("P15_CANONICAL_AFTER_PANIC", "canonical"); err != nil {
		t.Fatal(err)
	}
	if err := p15StateCompile("P15_MOBILE_AFTER_PANIC", "v2"); err != nil {
		t.Fatal(err)
	}
	var observation = map[string]any{"race_instrumented": p15StateRaceInstrumented, "emitted_before_panic": 1, "emitter_calls": calls, "partial_output_rejected": true, "recovery_requests": []string{"canonical", "mobile-v2"}}
	var data, err = json.Marshal(observation)
	if err != nil {
		t.Fatal(err)
	}
	t.Log(p15StateMarker + string(data))
}
