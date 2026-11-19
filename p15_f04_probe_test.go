package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

// ff04ee8's exact signed probes remain historical runtime controls. Current
// compilation rejects their unsupported List-valued loop boundary instead of
// generating the already disproven nested-cardinality promise.
func TestP15F04NestedListProbesFailClosed(t *testing.T) {
	for name, result := range map[string]string{
		"qa-f04-yield-list-v2":       "[item, \"TAIL\"]",
		"qa-f04-yield-empty-list-v2": "[]",
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join("tools", "shortcuts-simulator-qa", "fixtures", name+".cherri")
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			for _, required := range []string{"ANCHOR_CARD_01_TOP_HEADER", "ANCHOR_CARD_28_BOTTOM_FOOTER", "debugClearLogs()", "debugTraceEvent(stage: \"before\"", "debugTraceEvent(stage: \"after\"", "debugExportTrace(runId: \"" + name + "\""} {
				if !strings.Contains(string(source), required) {
					t.Fatal("historical native-probe correlation structure missing", required)
				}
			}
			diagnostics := p15TypeDiagnostics(t, string(source))
			if err := p15ExactLiteralDiagnostic(diagnostics, string(source), result, analysis.CodeTargetValueShape); err != nil {
				t.Fatal(err)
			}
			raw, err := CompileSourceToPlist(path, string(source))
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("unsupported original probe reached native emission: bytes=%d err=%v", len(raw), err)
			}
			t.Log("Expected compiler rejection only. F04 native nested cardinality remains failed; no native success is minted.")
		})
	}
}
