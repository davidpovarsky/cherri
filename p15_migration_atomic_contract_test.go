package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func p15RP34NoPartialFiles(t *testing.T, directory string) {
	t.Helper()
	var files, err = filepath.Glob(filepath.Join(directory, "cherri-atomic-*.tmp"))
	if err != nil || len(files) != 0 {
		t.Fatalf("failed migration leaked staged source files: %v %v", files, err)
	}
}

func TestP15RP34RejectedMigrationPreservesOriginal(t *testing.T) {
	var binary = p15AcceptanceCLI(t)
	var cases = []struct{ name, source string }{
		{"unknown_directive", "#p15Unsupported unsafe\n@value = 1\n"},
		{"unknown_trigger", "#trigger p15Unsupported value\n@value = 1\n"},
		{"unterminated_text", "@value = \"unterminated\n"},
		{"incomplete_expression", "@value = 1 +\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var directory = t.TempDir()
			var path = filepath.Join(directory, "original.cherri")
			var original = []byte(tc.source)
			p15SchemaWrite(t, path, original)
			var out, stderr, err = p15RunCLI(t, binary, directory, nil, "migrate", path, "--write")
			if err == nil || !strings.Contains(string(stderr), "Migration") {
				t.Fatalf("unrepresentable source was overwritten or lacked a migration failure: error=%v stdout=%s stderr=%s", err, out, stderr)
			}
			var after, readErr = os.ReadFile(path)
			if readErr != nil || !bytes.Equal(after, original) {
				t.Fatal("failed conversion changed original source")
			}
			p15RP34NoPartialFiles(t, directory)
			var healthy = []byte("@value = 1\nshow(@value)\n")
			p15SchemaWrite(t, path, healthy)
			out, stderr, err = p15RunCLI(t, binary, directory, nil, "migrate", path, "--write")
			if err != nil {
				t.Fatalf("restored migration failed: %v %s %s", err, out, stderr)
			}
			after, readErr = os.ReadFile(path)
			if readErr != nil || bytes.Equal(after, healthy) || !strings.Contains(string(after), "let value") {
				t.Fatal("restored migration did not write converted source")
			}
			if _, errors := p15ParseProgram(string(after)); len(errors) != 0 {
				t.Fatalf("successful migration wrote unparsable source: %v", errors)
			}
			p15RP34NoPartialFiles(t, directory)
		})
	}
}

func TestP15RP34ActualWriteFailurePreservesOriginal(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	// The isolated source copy changes only the final rename destination to a
	// child of the existing source file. This creates a real OS write failure
	// after conversion, temporary-file write, flush and close on every platform.
	// No fixture, original source, production contract or writer is replaced.
	var replica = p15SchemaReplica(t)
	var sourcePath = filepath.Join(replica, "language_cli_adapter.go")
	var originalSource, err = os.ReadFile(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	var old = "os.Rename(tmpPath, filePath)"
	var injected = "os.Rename(tmpPath, filepath.Join(filePath, \"P15_NOT_A_DIRECTORY\"))"
	if bytes.Count(originalSource, []byte(old)) != 1 {
		t.Fatal("atomic production rename changed; fault not applied")
	}
	p15SchemaWrite(t, sourcePath, []byte(strings.Replace(string(originalSource), old, injected, 1)))
	var binary = filepath.Join(t.TempDir(), "faulted-migration.exe")
	p15SchemaRun(t, replica, "go", "build", "-buildvcs=false", "-o", binary, ".")
	var directory = t.TempDir()
	var path = filepath.Join(directory, "original.cherri")
	var original = []byte("@value = 1\nshow(@value)\n")
	p15SchemaWrite(t, path, original)
	var out, stderr, cliErr = p15RunCLI(t, binary, directory, nil, "migrate", path, "--write")
	if cliErr == nil || !strings.Contains(string(stderr), "failed to atomically rename") {
		t.Fatalf("actual failed destination was not rejected: error=%v stdout=%s stderr=%s", cliErr, out, stderr)
	}
	var after, readErr = os.ReadFile(path)
	if readErr != nil || !bytes.Equal(after, original) {
		t.Fatal("rename failure changed the original source")
	}
	p15RP34NoPartialFiles(t, directory)
	p15SchemaWrite(t, sourcePath, originalSource)
	p15SchemaRun(t, replica, "go", "build", "-buildvcs=false", "-o", binary, ".")
	out, stderr, cliErr = p15RunCLI(t, binary, directory, nil, "migrate", path, "--write")
	if cliErr != nil {
		t.Fatalf("restored exact production writer failed: %v %s %s", cliErr, out, stderr)
	}
	after, readErr = os.ReadFile(path)
	if readErr != nil || bytes.Equal(after, original) || !strings.Contains(string(after), "let value") {
		t.Fatal("restored writer did not commit migration")
	}
	p15RP34NoPartialFiles(t, directory)
	t.Log("Actual public --write migration reached a fault-injected OS rename failure; original bytes survived and temporary file was removed; restored unmodified production writer committed the same source.")
}
