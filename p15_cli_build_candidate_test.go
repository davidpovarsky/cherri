package main

import (
	"bytes"
	"debug/buildinfo"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// A seed is only a candidate for the real Go command. Go refuses to replace a
// nonempty non-object output, so discard that owned seed before invoking Go.
// Parsing build information here never establishes source or build identity.
func p15StageAcceptanceCLISeed(path string, data []byte) error {
	if _, err := os.Lstat(path); !os.IsNotExist(err) {
		if err != nil {
			return err
		}
		return fmt.Errorf("CLI build candidate already exists: %s", path)
	}
	if err := atomicWriteFile(path, data, 0755); err != nil {
		return err
	}
	if _, err := buildinfo.Read(bytes.NewReader(data)); err != nil {
		return os.Remove(path)
	}
	return nil
}

func p15CLISeedFixture(t *testing.T, value string) (string, string) {
	t.Helper()
	var root = filepath.Join(t.TempDir(), "source")
	p15InstallWrite(t, filepath.Join(root, "go.mod"), []byte("module github.com/electrikmilk/cherri\n\ngo 1.24.0\n"))
	p15InstallWrite(t, filepath.Join(root, "main.go"), []byte(fmt.Sprintf("package main\nimport \"fmt\"\nfunc main() { fmt.Print(%q) }\n", value)))
	return root, p15ExtractedCommit(t, root, "Owned candidate publication source")
}

func TestP15AcceptanceCLISeedCandidateSafety(t *testing.T) {
	t.Run("non_object_is_removed_before_real_build", func(t *testing.T) {
		var directory = t.TempDir()
		var path = filepath.Join(directory, "candidate.exe")
		var unrelated = filepath.Join(directory, "retained")
		p15InstallWrite(t, unrelated, []byte("UNRELATED"))
		if err := p15StageAcceptanceCLISeed(path, []byte("NOT_A_GO_EXECUTABLE")); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			t.Fatalf("unusable candidate still exists: %v", err)
		}
		if string(p15InstallRead(t, unrelated)) != "UNRELATED" {
			t.Fatal("discarding an owned candidate changed a different file")
		}
	})
	t.Run("existing_file_is_not_replaced", func(t *testing.T) {
		var path = filepath.Join(t.TempDir(), "candidate.exe")
		p15InstallWrite(t, path, []byte("RETAINED"))
		if err := p15StageAcceptanceCLISeed(path, []byte("NOT_A_GO_EXECUTABLE")); err == nil {
			t.Fatal("an existing candidate path was accepted")
		}
		if string(p15InstallRead(t, path)) != "RETAINED" {
			t.Fatal("an existing file was changed")
		}
	})
	t.Run("symlink_is_rejected_without_touching_target", func(t *testing.T) {
		var directory = t.TempDir()
		var target = filepath.Join(directory, "retained")
		var path = filepath.Join(directory, "candidate.exe")
		p15InstallWrite(t, target, []byte("SYMLINK_TARGET"))
		if err := os.Symlink(target, path); err != nil {
			t.Fatal(err)
		}
		if err := p15StageAcceptanceCLISeed(path, []byte("NOT_A_GO_EXECUTABLE")); err == nil {
			t.Fatal("a symlink candidate was accepted")
		}
		var info, err = os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 || string(p15InstallRead(t, target)) != "SYMLINK_TARGET" {
			t.Fatalf("rejected symlink or target changed: %v", err)
		}
	})
}

func TestP15AcceptanceCLIStaleRealSeedRequiresLink(t *testing.T) {
	var root, _ = p15CLISeedFixture(t, "OLD_ACTUAL_PROGRAM")
	t.Setenv("GOFLAGS", strings.TrimSpace(os.Getenv("GOFLAGS")+" -x"))
	t.Setenv("GOWORK", "off")
	t.Setenv("CGO_ENABLED", "0")
	var original = p15InstallRead(t, p15AcceptanceCLIAtRoot(t, root, nil))
	p15InstallWrite(t, filepath.Join(root, "main.go"), []byte("package main\nimport \"fmt\"\nfunc main() { fmt.Print(\"CURRENT_ACTUAL_PROGRAM\") }\n"))
	p15ExtractedCommit(t, root, "Owned changed candidate source")
	var _, key = p15AcceptanceCLIReuseIdentity(root, os.Environ())
	if key == "" {
		t.Fatal("clean ordinary fixture lost candidate eligibility")
	}
	p15InstallBuiltTools.Lock()
	p15InstallBuiltTools.entries[key] = p15NewInstallBuiltTool(original)
	p15InstallBuiltTools.Unlock()
	var seeded bool
	var trace []byte
	var binary = p15AcceptanceCLIAtRoot(t, root, func(actualSeeded bool, output []byte) {
		seeded, trace = actualSeeded, output
	})
	var links = len(regexp.MustCompile(`(?m)^[^\r\n]*[/\\]link(?:\.exe)?["']? -o `).FindAll(trace, -1))
	if !seeded || links != 1 {
		t.Fatalf("stale real executable bypassed Go validation: seeded=%t links=%d\n%s", seeded, links, trace)
	}
	var stdout, stderr, err = p15RunCLI(t, binary, t.TempDir(), nil)
	if err != nil || len(stderr) != 0 || string(stdout) != "CURRENT_ACTUAL_PROGRAM" || bytes.Equal(original, p15InstallRead(t, binary)) {
		t.Fatalf("stale candidate reached the caller: %v stdout=%q stderr=%s", err, stdout, stderr)
	}
	t.Log("coherent cached bytes from an actual older executable caused one real link and current program execution")
}

func TestP15AcceptanceCLIConcurrentPrivateOutputs(t *testing.T) {
	var root, _ = p15CLISeedFixture(t, "CONCURRENT_ACTUAL_PROGRAM")
	type observation struct {
		path   string
		seeded bool
		data   []byte
	}
	var observed = make(chan observation, 3)
	t.Run("consumers", func(t *testing.T) {
		for i := 0; i < cap(observed); i++ {
			t.Run(fmt.Sprint(i), func(t *testing.T) {
				t.Parallel()
				var seeded bool
				var binary = p15AcceptanceCLIAtRoot(t, root, func(actual bool, _ []byte) { seeded = actual })
				var data = p15InstallRead(t, binary)
				var stdout, stderr, err = p15RunCLI(t, binary, t.TempDir(), nil)
				if err != nil || len(stderr) != 0 || string(stdout) != "CONCURRENT_ACTUAL_PROGRAM" {
					t.Fatalf("concurrent owned program: %v stdout=%q stderr=%s", err, stdout, stderr)
				}
				p15InstallWrite(t, binary, []byte("CALLER_PRIVATE_MUTATION"))
				observed <- observation{path: binary, seeded: seeded, data: data}
			})
		}
	})
	close(observed)
	var paths = map[string]bool{}
	var expected []byte
	var seeds int
	for item := range observed {
		if paths[item.path] {
			t.Fatal("concurrent callers received the same output path")
		}
		paths[item.path] = true
		if item.seeded {
			seeds++
		}
		if expected == nil {
			expected = item.data
		} else if !bytes.Equal(expected, item.data) {
			t.Fatal("a private caller mutation contaminated another build candidate")
		}
	}
	if len(paths) != 3 || seeds != 2 {
		t.Fatalf("concurrent validated copies: outputs=%d seeded=%d; want 3/2", len(paths), seeds)
	}
	t.Log("three concurrent callers executed separate verified outputs; two immutable seeds survived private caller mutations")
}
