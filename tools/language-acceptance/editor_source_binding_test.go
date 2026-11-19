package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Replay actual archived Linux receipts as intake controls only. Copying their
// bytes to a temporary directory does not establish a new production execution.
func editorArchivedActualRecord(t *testing.T, id string) EvidenceRecord {
	t.Helper()
	var filename = filepath.Join(resolveRepoRoot(), "docs", "language-v2", "evidence", "p15-formal-local-e61-20261119", "p10r15-local-acceptance-44621856878.zip")
	var raw, err = os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if editorSHA(raw) != "af9408fc41f90248ccbec2bff05ff32846f3c41f5a08ef22ea37edbaa4d81cfa" {
		t.Fatal("actual historical artifact changed")
	}
	var archive, archiveErr = zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	if archiveErr != nil {
		t.Fatal(archiveErr)
	}
	var entries = make(map[string]*zip.File)
	for _, file := range archive.File {
		if entries[file.Name] != nil {
			t.Fatal("duplicate archive entry", file.Name)
		}
		entries[file.Name] = file
	}
	var read = func(name string) []byte {
		var file = entries[name]
		if file == nil || file.UncompressedSize64 > 8<<20 {
			t.Fatal("missing or oversized actual archive member", name)
		}
		var reader, err = file.Open()
		if err != nil {
			t.Fatal(err)
		}
		defer reader.Close()
		var data, readErr = io.ReadAll(reader)
		if readErr != nil {
			t.Fatal(readErr)
		}
		return data
	}
	var records []EvidenceRecord
	if err := json.Unmarshal(read("acceptance-local-evidence.json"), &records); err != nil {
		t.Fatal(err)
	}
	var directory = t.TempDir()
	for _, record := range records {
		if len(record.Requirements) != 1 || record.Requirements[0] != id || record.Tier != "benchmark" {
			continue
		}
		if record.ImplementationSHA != "e61e0d306bb363c39ade612ef35ccfc8051ddf3c" || record.ExitCode != 0 || record.Result != "passed" {
			t.Fatal("historical source or result changed")
		}
		for index := range record.Artifacts {
			var artifact = &record.Artifacts[index]
			var components = strings.Split(strings.ReplaceAll(artifact.Path, "\\", "/"), "/")
			if len(components) < 2 {
				t.Fatal("historical artifact path lost")
			}
			var name = strings.Join(components[len(components)-2:], "/")
			var data = read(name)
			if editorSHA(data) != artifact.SHA256 {
				t.Fatal("historical artifact digest changed", name)
			}
			artifact.Path = filepath.Join(directory, components[len(components)-1])
			if err := os.WriteFile(artifact.Path, data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		return record
	}
	t.Fatal("actual benchmark record absent", id)
	return EvidenceRecord{}
}

func TestEditorActualReceiptSourceDigestBinding(t *testing.T) {
	for _, id := range []string{"S03", "E08"} {
		var original = editorArchivedActualRecord(t, id)
		if err := validateEditorPerformanceRecord(id, original); err != nil {
			t.Fatal("actual historical receipt rejected", err)
		}
		editorAssertActualSourceDigestMutations(t, id, original)
	}
}

func editorAssertActualSourceDigestMutations(t *testing.T, id string, original EvidenceRecord) {
	t.Helper()
	for index, path := range editorProductionPathsForID(id) {
		t.Run(fmt.Sprintf("%s/source_file_%02d", id, index), func(t *testing.T) {
			var changed = original
			changed.Artifacts = append([]EvidenceArtifact(nil), original.Artifacts...)
			var directory = t.TempDir()
			for artifactIndex := range changed.Artifacts {
				var artifact = &changed.Artifacts[artifactIndex]
				var data, err = os.ReadFile(artifact.Path)
				if err != nil || editorSHA(data) != artifact.SHA256 {
					t.Fatal("actual source artifact changed before mutation", err)
				}
				artifact.Path = filepath.Join(directory, filepath.Base(artifact.Path))
				if err := os.WriteFile(artifact.Path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			editorMutateModelFile(t, &changed, "-process.json", func(raw []byte) []byte {
				var receipt editorProcessReceipt
				if err := json.Unmarshal(raw, &receipt); err != nil {
					t.Fatal(err)
				}
				if receipt.SourceFiles[index].Path != path {
					t.Fatal("source control targets a different field")
				}
				receipt.SourceFiles[index].SHA256 = strings.Repeat("f", 64)
				var data, err = json.Marshal(receipt)
				if err != nil {
					t.Fatal(err)
				}
				return data
			})
			var err = validateEditorPerformanceRecord(id, changed)
			if err == nil || !strings.Contains(err.Error(), "source file digest disagrees with immutable Git blob") {
				t.Fatalf("counterfeit source digest accepted after actual process artifact hash was reconciled: %s: %v", path, err)
			}
		})
	}
}

func TestEditorImmutableGitSourceFiles(t *testing.T) {
	var repository = t.TempDir()
	var git = func(args ...string) string {
		var command = exec.Command("git", append([]string{"-C", repository}, args...)...)
		var output, err = command.CombinedOutput()
		if err != nil {
			t.Fatalf("Git source fixture: %v: %v: %s", args, err, output)
		}
		return strings.TrimSpace(string(output))
	}
	git("init", "--quiet")
	var original = []byte("original source\n")
	var binary = []byte{'x', 0, '\r', '\n', 'y', '\n'}
	for name, data := range map[string][]byte{".gitattributes": []byte("* -text\nsource.txt text eol=crlf\n"), "source.txt": []byte("original source\r\n"), "nested/binary": binary} {
		var filename = filepath.Join(repository, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(filename), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	git("add", ".")
	git("-c", "user.name=Cherri intake test", "-c", "user.email=intake-test@example.invalid", "commit", "--quiet", "-m", "Original source")
	var first = git("rev-parse", "HEAD")
	if data, err := os.ReadFile(filepath.Join(repository, "source.txt")); err != nil || !bytes.Equal(data, []byte("original source\r\n")) {
		t.Fatal("Git control did not retain exact CRLF worktree bytes", err)
	}
	if status := git("status", "--porcelain"); status != "" {
		t.Fatal("CRLF control is not a clean converted worktree", status)
	}
	var paths = []string{"source.txt", "nested/binary"}
	var files, err = editorGitSourceFiles(repository, first, paths)
	if err != nil || len(files) != 2 || files[0].SHA256 != editorSHA(original) || files[1].SHA256 != editorSHA(binary) {
		t.Fatalf("read working tree or altered exact binary Git blob: %+v %v", files, err)
	}
	files[0].SHA256 = strings.Repeat("f", 64)
	var repeated, repeatedErr = editorGitSourceFiles(repository, first, paths)
	if repeatedErr != nil || repeated[0].SHA256 != editorSHA(original) {
		t.Fatal("receipt caller can corrupt cached source identity", repeatedErr)
	}
	var changed = []byte("uncommitted bytes must not replace the pinned source\r\n")
	if err := os.WriteFile(filepath.Join(repository, "source.txt"), changed, 0600); err != nil {
		t.Fatal(err)
	}
	var stillPinned, pinnedErr = editorGitSourceFiles(repository, first, []string{"source.txt"})
	if pinnedErr != nil || stillPinned[0].SHA256 != editorSHA(original) {
		t.Fatal("dirty working-tree contents replaced immutable source", pinnedErr)
	}
	git("add", "source.txt")
	git("-c", "user.name=Cherri intake test", "-c", "user.email=intake-test@example.invalid", "commit", "--quiet", "-m", "Changed source")
	var second = git("rev-parse", "HEAD")
	var newer, newerErr = editorGitSourceFiles(repository, second, paths)
	if newerErr != nil || newer[0].SHA256 != editorSHA([]byte("uncommitted bytes must not replace the pinned source\n")) || first == second {
		t.Fatal("cache confused immutable revisions", newerErr)
	}
	for _, control := range []struct {
		name, commit string
		paths        []string
	}{
		{"floating_revision", "HEAD", paths},
		{"missing_revision", strings.Repeat("e", 40), paths},
		{"missing_source", first, []string{"missing.txt"}},
		{"tree_instead_of_blob", first, []string{"nested"}},
		{"parent_path", first, []string{"../source.txt"}},
		{"new_line_path", first, []string{"source.txt\nnested/binary"}},
		{"duplicate_source", first, []string{"source.txt", "source.txt"}},
	} {
		t.Run(control.name, func(t *testing.T) {
			if _, err := editorGitSourceFiles(repository, control.commit, control.paths); err == nil {
				t.Fatal("invalid source request accepted")
			}
		})
	}
}
