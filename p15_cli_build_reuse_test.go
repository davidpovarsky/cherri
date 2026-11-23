package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestP15AcceptanceCLIImmutableReuseControls(t *testing.T) {
	// These owned real Git/Go fixtures test build reuse only. Their programs are
	// deliberately not Cherri acceptance or native-runtime evidence.
	var root = filepath.Join(t.TempDir(), "source")
	p15InstallWrite(t, filepath.Join(root, "go.mod"), []byte("module github.com/electrikmilk/cherri\n\ngo 1.24.0\n"))
	var program = func(value string) []byte {
		return []byte(fmt.Sprintf("package main\nimport \"fmt\"\nvar buildMarker string\nfunc main() { fmt.Print(%q + buildMarker) }\n", value))
	}
	var mainPath = filepath.Join(root, "main.go")
	p15InstallWrite(t, mainPath, program("CLI_REUSE_ONE"))
	p15ExtractedCommit(t, root, "Owned build-reuse source")
	var traceFlags = strings.TrimSpace(os.Getenv("GOFLAGS") + " -x")
	t.Setenv("GOFLAGS", traceFlags)
	t.Setenv("GOWORK", "off")
	t.Setenv("CGO_ENABLED", "0")
	var linkCommand = regexp.MustCompile("(?m)^[^\\r\\n]*[/\\\\]link(?:\\.exe)?[\"']? -o ")
	var probe = func(t *testing.T, sourceRoot, want string, wantSeed bool, wantLinks int) string {
		t.Helper()
		var observed bool
		var output []byte
		var binary = p15AcceptanceCLIAtRoot(t, sourceRoot, func(seeded bool, actual []byte) {
			observed, output = seeded, actual
		})
		var links = len(linkCommand.FindAll(output, -1))
		if observed != wantSeed || links != wantLinks {
			t.Fatalf("actual Go validation seeded=%t link commands=%d; want %t/%d\n%s", observed, links, wantSeed, wantLinks, output)
		}
		var stdout, stderr, err = p15RunCLI(t, binary, t.TempDir(), nil)
		if err != nil || len(stderr) != 0 || string(stdout) != want {
			t.Fatalf("actual owned program: %v stdout=%q stderr=%s", err, stdout, stderr)
		}
		t.Logf("actual Go command retained; seeded=%t link commands=%d output=%q", observed, links, stdout)
		return binary
	}
	var original []byte
	t.Run("real_miss_then_seeded_validation", func(t *testing.T) {
		var first = probe(t, root, "CLI_REUSE_ONE", false, 1)
		original = append([]byte(nil), p15InstallRead(t, first)...)
		var second = probe(t, root, "CLI_REUSE_ONE", true, 0)
		if first == second || !bytes.Equal(original, p15InstallRead(t, second)) {
			t.Fatal("reuse did not retain separate owned files with the exact actual build bytes")
		}
		// Corrupt only a caller-owned copy. This is not a claim that Go detects
		// arbitrary body corruption with an unchanged embedded build ID.
		p15InstallWrite(t, first, []byte("PRIVATE_COPY_CORRUPTED"))
		var third = probe(t, root, "CLI_REUSE_ONE", true, 0)
		if third == first || third == second || !bytes.Equal(original, p15InstallRead(t, third)) {
			t.Fatal("a private output mutation contaminated immutable cache bytes")
		}
	})
	t.Run("actual_new_source_commit", func(t *testing.T) {
		var before = p15ExtractedRevision(t, root)
		p15InstallWrite(t, mainPath, program("CLI_REUSE_TWO"))
		var after = p15ExtractedCommit(t, root, "Owned changed build-reuse source")
		if before == after {
			t.Fatal("source control did not create a distinct actual commit")
		}
		var binary = probe(t, root, "CLI_REUSE_TWO", false, 1)
		if bytes.Equal(original, p15InstallRead(t, binary)) {
			t.Fatal("new actual source reused old executable bytes")
		}
	})
	t.Run("effective_build_environment", func(t *testing.T) {
		t.Setenv("GOFLAGS", traceFlags+" -ldflags=-X=main.buildMarker=_ENV")
		probe(t, root, "CLI_REUSE_TWO_ENV", false, 1)
		probe(t, root, "CLI_REUSE_TWO_ENV", true, 0)
	})
	t.Run("malformed_build_id_requires_real_link", func(t *testing.T) {
		var _, key = p15AcceptanceCLIReuseIdentity(root, os.Environ())
		if key == "" {
			t.Fatal("clean ordinary fixture lost candidate eligibility")
		}
		// Deliberately synthesize a coherent cache entry without any Go build ID.
		// The unchanged real Go command must link it, not trust our lookup key.
		p15InstallBuiltTools.Lock()
		p15InstallBuiltTools.entries[key] = p15NewInstallBuiltTool([]byte("NOT_A_GO_EXECUTABLE"))
		p15InstallBuiltTools.Unlock()
		probe(t, root, "CLI_REUSE_TWO", true, 1)
	})
	t.Run("dirty_source_remains_uncached", func(t *testing.T) {
		p15InstallWrite(t, mainPath, program("CLI_REUSE_DIRTY"))
		t.Cleanup(func() { p15ExtractedGit(t, root, "checkout", "--", "main.go") })
		probe(t, root, "CLI_REUSE_DIRTY", false, 1)
		probe(t, root, "CLI_REUSE_DIRTY", false, 1)
	})
	t.Run("linked_worktree_remains_uncached", func(t *testing.T) {
		var linked = filepath.Join(t.TempDir(), "linked")
		p15ExtractedGit(t, root, "worktree", "add", "--quiet", "--detach", linked, "HEAD")
		t.Cleanup(func() { p15ExtractedGit(t, root, "worktree", "remove", linked) })
		probe(t, linked, "CLI_REUSE_TWO", false, 1)
	})
	t.Run("unverified_source_remains_uncached", func(t *testing.T) {
		var sourceRoot = filepath.Join(t.TempDir(), "not-a-git-repository")
		p15InstallWrite(t, filepath.Join(sourceRoot, "go.mod"), p15InstallRead(t, filepath.Join(root, "go.mod")))
		p15InstallWrite(t, filepath.Join(sourceRoot, "main.go"), program("CLI_REUSE_UNVERIFIED"))
		probe(t, sourceRoot, "CLI_REUSE_UNVERIFIED", false, 1)
	})
	t.Run("digest_and_owned_bytes", func(t *testing.T) {
		var input = []byte("OWNED_ACTUAL_TOOL_BYTES")
		var entry = p15NewInstallBuiltTool(input)
		input[0] ^= 1
		if err := entry.verify(); err != nil || string(entry.data) != "OWNED_ACTUAL_TOOL_BYTES" {
			t.Fatalf("cache construction retained a caller-owned slice: %v", err)
		}
		entry.data[len(entry.data)-1] ^= 1
		if entry.verify() == nil {
			t.Fatal("cache byte corruption with the original digest was accepted")
		}
	})
}
