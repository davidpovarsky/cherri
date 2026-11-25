package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
)

func TestP15F04ExplicitContextOriginalFixtures(t *testing.T) {
	for _, fixture := range []struct{ name, path, digest string }{
		{"nested_list", "tools/shortcuts-simulator-qa/fixtures/qa-f04-yield-list-v2.cherri", "c0719b046c2ed724fccef9e5b09e313c7dd72c02dc2c72ce1abe8aab1592e091"},
		{"empty_list", "tools/shortcuts-simulator-qa/fixtures/qa-f04-yield-empty-list-v2.cherri", "d1322f6c7c663d266b0e9a7e272a92ae331be0e411818dae0c7d5172587578c3"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			source, err := os.ReadFile(fixture.path)
			if err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(source)) != fixture.digest {
				t.Fatal("the original QA source changed")
			}
			raw, err := CompileSourceToPlist(fixture.path, string(source))
			if err != nil || len(raw) == 0 {
				t.Fatalf("original explicit trace/output context still rejects loop values: %v", err)
			}
		})
	}
}

func TestP15F04ExplicitContextCapabilities(t *testing.T) {
	const body = "let values=repeat 2 {yield []}\nlet observed=count(values)\n"
	for _, tc := range []struct{ name, source string }{
		{"comment", "comment(\"explicit comment\")\n" + body + "return observed"},
		{"text", body + "let summary=text(f\"count={observed}\")\nreturn summary"},
		{"output", body + "output(f\"count={observed}\")"},
		{"trace_event", "debugTraceEvent(stage:\"before\",label:\"loop\")\n" + body + "return observed"},
		{"trace_export", body + "let trace=debugExportTrace(runId:\"loop\")\nreturn observed"},
		{"clear_logs", "debugClearLogs()\n" + body + "return observed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("explicit-context-"+tc.name+".cherri", tc.source)
			if err != nil || len(raw) == 0 {
				t.Fatalf("ordinary explicit context rejected: %v", err)
			}
		})
	}
}

func TestP15F04ExplicitContextPrivateBoundary(t *testing.T) {
	const body = "let values=repeat 2 {yield []}\n"
	for _, tc := range []struct{ name, source string }{
		{"private_text", body + "let visible=text(values)\nreturn count(values)"},
		{"private_output", body + "count(values)\noutput(values)"},
		{"private_interpolation", body + "let visible=text(f\"{values}\")\nreturn count(values)"},
		{"private_trace_label", body + "debugTraceEvent(stage:\"after\",label:values)\nreturn count(values)"},
		{"omitted_previous_debug", body + "debugLog()\nreturn count(values)"},
		{"omitted_previous_native", body + "native.action(\"is.workflow.actions.detect.text\", {})\nreturn count(values)"},
		{"closed_producer_body", "let values=repeat 2 {text(\"bound\")\nyield []}\nreturn count(values)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("private-context-"+tc.name+".cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), analysis.CodeTargetValueShape) {
				t.Fatalf("private or implicit Previous boundary changed: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}

func TestP15F04ExplicitContextOutputTypeContract(t *testing.T) {
	for _, tc := range []struct{ name, source string }{
		{"ordinary", "let observed=2\noutput(observed)"},
		{"with_loop", "let values=repeat 2 {yield []}\nlet observed=count(values)\noutput(observed)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, err := CompileSourceToPlist("output-type-"+tc.name+".cherri", tc.source)
			if err == nil || len(raw) != 0 || !strings.Contains(err.Error(), "E_ARGUMENT_TYPE") || !strings.Contains(err.Error(), "expects Text, got Number") {
				t.Fatalf("the ordinary output Text contract changed: bytes=%d error=%v", len(raw), err)
			}
		})
	}
}
