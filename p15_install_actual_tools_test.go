package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var p15InstallBuiltTools = struct {
	sync.Mutex
	bytes map[string][]byte
}{bytes: map[string][]byte{}}

// Only the complete ordinary suite opts reviewed subprocess parents into Go's
// bounded scheduler. Selected acceptance producers must retain their strict
// sequential JSON event stream, including when they inherit the suite marker.
func p15FullSuiteSubprocessParallel(t *testing.T) {
	t.Helper()
	var enabled, err = p15FullSuiteSubprocessParallelPolicy(
		os.Getenv("CHERRI_FULL_SUITE_SUBPROCESS_PARALLEL"), t.Name(),
		flag.Lookup("test.run").Value.String(), flag.Lookup("test.parallel").Value.String())
	if err != nil {
		t.Fatal(err)
	}
	if enabled {
		t.Parallel()
	}
}

func p15FullSuiteSubprocessParallelPolicy(marker, name, selection, limit string) (bool, error) {
	if marker != "2" || selection != "" || name == "" || strings.Contains(name, "/") {
		return false, nil
	}
	if limit != "2" {
		return false, fmt.Errorf("full-suite subprocess scheduling requires -parallel=2; got %q", limit)
	}
	return true, nil
}

func TestP15FullSuiteSubprocessParallelPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, marker, test, selection, limit string
		enabled, rejected                    bool
	}{
		{"ordinary", "2", "TestOwnedSubprocess", "", "2", true, false},
		{"no_opt_in", "", "TestOwnedSubprocess", "", "2", false, false},
		{"different_marker", "1", "TestOwnedSubprocess", "", "2", false, false},
		{"padded_marker", "2 ", "TestOwnedSubprocess", "", "2", false, false},
		{"selected_producer", "2", "TestOwnedSubprocess", "^TestOwnedSubprocess$", "2", false, false},
		{"selected_all", "2", "TestOwnedSubprocess", ".*", "2", false, false},
		{"nested_child", "2", "TestOwnedSubprocess/child", "", "2", false, false},
		{"missing_test", "2", "", "", "2", false, false},
		{"missing_limit", "2", "TestOwnedSubprocess", "", "", false, true},
		{"serial_limit", "2", "TestOwnedSubprocess", "", "1", false, true},
		{"excessive_limit", "2", "TestOwnedSubprocess", "", "3", false, true},
		{"selected_producer_unchanged", "2", "TestOwnedSubprocess", "^TestOwnedSubprocess$", "8", false, false},
		{"nested_child_unchanged", "2", "TestOwnedSubprocess/child", "", "8", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var enabled, err = p15FullSuiteSubprocessParallelPolicy(tc.marker, tc.test, tc.selection, tc.limit)
			if enabled != tc.enabled || (err != nil) != tc.rejected {
				t.Fatalf("parallel policy enabled=%t error=%v; want enabled=%t rejected=%t", enabled, err, tc.enabled, tc.rejected)
			}
		})
	}
}

// Each Go test process builds the actual clean source. Repeated consumers of
// that immutable build reuse bytes, never another test's temporary filename.
// Variants are distinct real builds, not edited or relabeled executables.
func p15InstallActualTool(t *testing.T, root, source, packagePath, variant string) string {
	t.Helper()
	if p15ExtractedRevision(t, root) != source || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("installed consumer requires the exact clean source")
	}
	key := root + "\x00" + source + "\x00" + packagePath + "\x00" + variant
	p15InstallBuiltTools.Lock()
	defer p15InstallBuiltTools.Unlock()
	data, exists := p15InstallBuiltTools.bytes[key]
	if !exists {
		var flags []string
		if variant != "" {
			flags = []string{"-ldflags=-buildid=" + variant}
		}
		built := p15ExtractedBuild(t, root, source, packagePath, flags...)
		data = append([]byte(nil), p15InstallRead(t, built)...)
		p15InstallBuiltTools.bytes[key] = data
	}
	path := filepath.Join(t.TempDir(), "actual-tool")
	if isWindowsRuntime() {
		path += ".exe"
	}
	p15InstallWrite(t, path, data)
	role := "github.com/electrikmilk/cherri"
	if packagePath == "./cmd/cherri-migrate" {
		role += "/cmd/cherri-migrate"
	}
	p15ExtractedRequireRole(t, path, source, role)
	if p15ExtractedRevision(t, root) != source || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("installed consumer source changed during build reuse")
	}
	return path
}

func p15InstallActualCompiler(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	return p15InstallActualTool(t, root, p15ExtractedRevision(t, root), ".", "")
}

func p15InstallSeedTool(t *testing.T, destination, binary, source, role string) {
	t.Helper()
	p15ExtractedRequireRole(t, binary, source, role)
	data := p15InstallRead(t, binary)
	p15InstallWrite(t, destination, data)
	p15InstallWrite(t, destination+".commit", []byte(source+"\n"))
	p15InstallWrite(t, destination+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(data))))
}

func p15InstallCopyPair(t *testing.T, from, to string) {
	t.Helper()
	for _, tool := range []string{"cherri", "cherri-migrate"} {
		for _, suffix := range []string{"", ".commit", ".sha256"} {
			p15InstallWrite(t, filepath.Join(to, tool+suffix), p15InstallRead(t, filepath.Join(from, tool+suffix)))
		}
	}
}

// Compare the selected file identity, not Git Bash's /tmp vs Windows spelling.
func p15AssertDoctorMigration(t *testing.T, output, expected string) {
	t.Helper()
	chosen := ""
	for _, line := range strings.Split(output, "\n") {
		if !strings.HasPrefix(line, "Migration tool: ") {
			continue
		}
		if chosen != "" || !strings.HasSuffix(line, " (source and role verified)") {
			t.Fatalf("missing/duplicate migration identity assertion: %s", output)
		}
		chosen = strings.TrimSuffix(strings.TrimPrefix(line, "Migration tool: "), " (source and role verified)")
	}
	if chosen == "" {
		t.Fatalf("doctor omitted selected migration file: %s", output)
	}
	p15AssertDoctorBinary(t, "Cherri: "+chosen+"\n", expected)
}
