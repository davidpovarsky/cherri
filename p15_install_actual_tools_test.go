package main

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

var p15InstallBuiltTools = struct {
	sync.Mutex
	bytes map[string][]byte
}{bytes: map[string][]byte{}}

// Each Go test process builds the actual clean source. Repeated consumers of
// that immutable build reuse bytes, never another test's temporary filename.
// Variants are distinct real builds, not edited or relabeled executables.
func p15InstallActualTool(t *testing.T, root, source, packagePath, variant string) string {
	t.Helper()
	defer p15PhaseBegin(t, "actual_tool_request", map[string]any{"root": root, "source": source, "package": packagePath, "variant": variant}).ScopeEnd(t)
	sourceBeforePhase := p15PhaseBegin(t, "actual_tool_source_before", nil)
	if p15ExtractedRevision(t, root) != source || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("installed consumer requires the exact clean source")
	}
	sourceBeforePhase.ScopeEnd(t)
	key := root + "\x00" + source + "\x00" + packagePath + "\x00" + variant
	cachePhase := p15PhaseBegin(t, "actual_tool_cache_lookup", map[string]any{"root": root, "source": source, "package": packagePath, "variant": variant})
	p15InstallBuiltTools.Lock()
	defer p15InstallBuiltTools.Unlock()
	data, exists := p15InstallBuiltTools.bytes[key]
	cachePhase.End(t, map[string]any{"cache_hit": exists, "cached_bytes": len(data), "observed_under_existing_mutex": true}, nil)
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
	copyPhase := p15PhaseBegin(t, "actual_tool_owned_copy", map[string]any{"bytes": len(data)})
	p15InstallWrite(t, path, data)
	copyPhase.ScopeEnd(t)
	role := "github.com/electrikmilk/cherri"
	if packagePath == "./cmd/cherri-migrate" {
		role += "/cmd/cherri-migrate"
	}
	rolePhase := p15PhaseBegin(t, "actual_tool_role_check", map[string]any{"role": role})
	p15ExtractedRequireRole(t, path, source, role)
	rolePhase.ScopeEnd(t)
	sourceAfterPhase := p15PhaseBegin(t, "actual_tool_source_after", nil)
	if p15ExtractedRevision(t, root) != source || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("installed consumer source changed during build reuse")
	}
	sourceAfterPhase.ScopeEnd(t)
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
