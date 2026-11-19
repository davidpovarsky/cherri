package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Explicitly requested corpus validation must execute the complete corpus or
// fail. A green skip or a partial inventory cannot stand in for that run.
func TestP10PrivateCorpusOptInFailsClosed(t *testing.T) {
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"missing", "empty", "incomplete"} {
		t.Run(name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "corpus")
			expected := "PRIVATE_CORPUS_UNAVAILABLE"
			if name != "missing" {
				if err := os.Mkdir(directory, 0700); err != nil {
					t.Fatal(err)
				}
				expected = "PRIVATE_CORPUS_INCOMPLETE"
			}
			fixture := filepath.Join(directory, "synthetic.plist")
			original := []byte("The count gate must reject this before parsing.")
			if name == "incomplete" {
				if err := os.WriteFile(fixture, original, 0600); err != nil {
					t.Fatal(err)
				}
			}
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, binary, "-test.run=^TestP10R10CorpusMeasure$", "-test.v")
			for _, item := range os.Environ() {
				if !strings.HasPrefix(item, "CHERRI_P10R10_CORPUS=") &&
					!strings.HasPrefix(item, "CHERRI_P10R10_CORPUS_DIR=") {
					cmd.Env = append(cmd.Env, item)
				}
			}
			cmd.Env = append(cmd.Env, "CHERRI_P10R10_CORPUS=1", "CHERRI_P10R10_CORPUS_DIR="+directory)
			output, runErr := cmd.CombinedOutput()
			if ctx.Err() != nil {
				t.Fatalf("corpus gate did not terminate: %v", ctx.Err())
			}
			if runErr == nil || !bytes.Contains(output, []byte(expected)) ||
				bytes.Contains(output, []byte("--- SKIP:")) {
				t.Fatalf("expected a discovered failing corpus gate %s, got %v: %s", expected, runErr, output)
			}
			if name == "incomplete" {
				actual, err := os.ReadFile(fixture)
				if err != nil || !bytes.Equal(actual, original) {
					t.Fatalf("corpus input changed: %v", err)
				}
			}
		})
	}
}
