package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"go/version"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const strictGoOutputTypePackage = "example.invalid/strict-go-output"
const strictGoOutputTypeTest = "TestGoEventProducer"

func strictGoOutputTypeEvents(t *testing.T, change func([]map[string]any)) []byte {
	t.Helper()
	event := func(action, name string) map[string]any {
		value := map[string]any{"Time": "2026-11-25T00:00:00Z", "Action": action, "Package": strictGoOutputTypePackage}
		if name != "" {
			value["Test"] = name
		}
		if action == "pass" {
			value["Elapsed"] = 0.001
		}
		return value
	}
	events := []map[string]any{
		event("start", ""), event("run", strictGoOutputTypeTest),
		event("output", strictGoOutputTypeTest), event("pass", strictGoOutputTypeTest),
		event("output", ""), event("pass", ""),
	}
	events[2]["Output"] = "=== RUN   " + strictGoOutputTypeTest + "\n"
	events[4]["Output"] = "PASS\n"
	if change != nil {
		change(events)
	}
	var raw bytes.Buffer
	encoder := json.NewEncoder(&raw)
	for _, value := range events {
		if err := encoder.Encode(value); err != nil {
			t.Fatal(err)
		}
	}
	return raw.Bytes()
}

func TestStrictGoEventsOutputTypeCompatibility(t *testing.T) {
	for _, kind := range []string{"omitted", "", "frame"} {
		t.Run(kind, func(t *testing.T) {
			raw := strictGoOutputTypeEvents(t, func(events []map[string]any) {
				if kind != "omitted" {
					events[2]["OutputType"] = kind
					events[4]["OutputType"] = kind
				}
			})
			if err := parseStrictNamedProductionOutputInPackage(strictGoOutputTypePackage, []string{strictGoOutputTypeTest}, time.Minute, raw); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestStrictGoEventsOutputTypeRejectsInvalidEvidence(t *testing.T) {
	cases := []struct {
		name   string
		change func([]map[string]any)
	}{
		{"unknown_class", func(e []map[string]any) { e[2]["OutputType"] = "success" }},
		{"number", func(e []map[string]any) { e[2]["OutputType"] = 1 }},
		{"bool", func(e []map[string]any) { e[2]["OutputType"] = true }},
		{"null", func(e []map[string]any) { e[2]["OutputType"] = nil }},
		{"object", func(e []map[string]any) { e[2]["OutputType"] = map[string]any{} }},
		{"array", func(e []map[string]any) { e[2]["OutputType"] = []string{"frame"} }},
		{"non_output", func(e []map[string]any) { e[1]["OutputType"] = "frame" }},
		{"non_output_blank", func(e []map[string]any) { e[1]["OutputType"] = "" }},
		{"error_despite_pass", func(e []map[string]any) { e[2]["OutputType"] = "error" }},
		{"error_continuation_despite_pass", func(e []map[string]any) { e[2]["OutputType"] = "error-continue" }},
		{"wrong_field_case", func(e []map[string]any) { e[2]["outputType"] = "frame" }},
		{"unrecognized_field", func(e []map[string]any) { e[2]["OutputKind"] = "frame" }},
		{"unrecognized_field_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; e[2]["Result"] = "passed" }},
		{"foreign_package_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; e[2]["Package"] = "example.invalid/foreign" }},
		{"unselected_test_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; e[2]["Test"] = "TestUnselected" }},
		{"failed_status_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; e[3]["Action"] = "fail" }},
		{"skipped_status_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; e[3]["Action"] = "skip" }},
		{"missing_elapsed_with_frame", func(e []map[string]any) { e[2]["OutputType"] = "frame"; delete(e[3], "Elapsed") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			raw := strictGoOutputTypeEvents(t, tc.change)
			if err := parseStrictNamedProductionOutputInPackage(strictGoOutputTypePackage, []string{strictGoOutputTypeTest}, time.Minute, raw); err == nil {
				t.Fatal("invalid or unproved production evidence accepted")
			}
		})
	}
	t.Run("duplicate_output_type", func(t *testing.T) {
		raw := strictGoOutputTypeEvents(t, func(e []map[string]any) { e[2]["OutputType"] = "frame" })
		raw = bytes.Replace(raw, []byte("\"OutputType\":\"frame\""), []byte("\"OutputType\":\"error\",\"OutputType\":\"frame\""), 1)
		if err := parseStrictNamedProductionOutputInPackage(strictGoOutputTypePackage, []string{strictGoOutputTypeTest}, time.Minute, raw); err == nil {
			t.Fatal("duplicate field concealed an error classification")
		}
	})
}

// This is an actual go test -json producer, not a hand-authored event receipt.
// Its sole fixture is an acceptance-parser probe and supplies no language or
// Apple-runtime evidence.
func TestStrictGoEventsActualToolchainProducer(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module "+strictGoOutputTypePackage+"\n\ngo 1.24.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "probe_test.go"), []byte("package probe\nimport \"testing\"\nfunc TestGoEventProducer(t *testing.T) { t.Log(\"actual producer output\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	command := exec.CommandContext(ctx, "go", "test", "-count=1", "-json", "-timeout=30s", "-run", "^"+strictGoOutputTypeTest+"$", ".")
	command.Dir = dir
	raw, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("actual Go producer failed: %v context=%v\n%s", err, ctx.Err(), raw)
	}
	if err := parseStrictNamedProductionOutputInPackage(strictGoOutputTypePackage, []string{strictGoOutputTypeTest}, time.Minute, raw); err != nil {
		t.Fatalf("actual Go producer was rejected: %v\n%s", err, raw)
	}
	frames := 0
	for _, line := range bytes.Split(bytes.TrimSpace(raw), []byte("\n")) {
		var event struct{ Action, OutputType string }
		if err := json.Unmarshal(line, &event); err != nil {
			t.Fatal(err)
		}
		if event.Action == "output" && event.OutputType == "frame" {
			frames++
		}
	}
	toolchain := exec.CommandContext(ctx, "go", "env", "GOVERSION")
	toolchain.Dir = dir
	toolchainVersion, err := toolchain.Output()
	if err != nil {
		t.Fatal(err)
	}
	if version.Compare(strings.TrimSpace(string(toolchainVersion)), "go1.27") >= 0 && frames == 0 {
		t.Fatal("Go 1.27 producer did not exercise OutputType framing")
	}
	t.Logf("actual producer=%s test_binary=%s frames=%d stream_bytes=%d sha256=%x", strings.TrimSpace(string(toolchainVersion)), runtime.Version(), frames, len(raw), sha256.Sum256(raw))
}
