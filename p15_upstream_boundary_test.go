package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

var p15U01Baselines = map[string]string{
	"upstream":        "a66db15b7f247f3121726c2a2b72becfeef3d96b",
	"language":        "559abceadda1dbf0cd7d7deb1a26315b8ac44c3f",
	"continuation":    "d7052f4568db26dba169bdefc72623e48c04968a",
	"recorded_source": "8e66254d56ea3e4ac70f6cda6ef9d31217e0937c",
}

type p15U01Version struct {
	Bytes  int    `json:"bytes"`
	SHA256 string `json:"sha256"`
}
type p15U01Patch struct {
	Before string `json:"before"`
	After  string `json:"after"`
	Reason string `json:"reason"`
}
type p15U01File struct {
	Path                 string                   `json:"path"`
	Versions             map[string]p15U01Version `json:"versions"`
	ContinuationClass    string                   `json:"continuation_class"`
	UpstreamIdentical    bool                     `json:"upstream_identical"`
	LanguageIdentical    bool                     `json:"language_baseline_identical"`
	ReviewedPatch        *p15U01Patch             `json:"reviewed_patch,omitempty"`
	ImplementationCommit string                   `json:"implementation_commit,omitempty"`
}
type p15U01Manifest struct {
	SchemaVersion           int               `json:"schema_version"`
	RequirementID           string            `json:"requirement_id"`
	Baselines               map[string]string `json:"baselines"`
	ActionFileCount         int               `json:"action_file_count"`
	AdditionalPreservedFile string            `json:"additional_preserved_file"`
	Scope                   string            `json:"scope"`
	Files                   []p15U01File      `json:"files"`
}
type p15U01Snapshot struct {
	Head       string
	Inventory  []string
	Historical map[string][]byte
	Current    map[string][]byte
	Tracked    map[string][]byte
	Status     []byte
}

// This independent allowlist is the reviewed two-token source correction,
// not an automatically learned diff from the current checkout.
func p15U01AllowedPatch(path string) (string, string, bool) {
	switch path {
	case "actions/device.cherri":
		return "text ?startTime: 'endTime'", "text ?endTime: 'endTime'", true
	case "actions/location.cherri":
		return "text addressLine2: 'WFAddressLine1'", "text addressLine1: 'WFAddressLine1'", true
	default:
		return "", "", false
	}
}

func p15U01Git(t *testing.T, input []byte, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", args...)
	command.Stdin = bytes.NewReader(input)
	var errors bytes.Buffer
	command.Stderr = &errors
	raw, err := command.Output()
	if err != nil {
		t.Fatalf("actual git %v failed: %v %s", args, err, errors.String())
	}
	return raw
}

func p15U01ReadManifest(t *testing.T) p15U01Manifest {
	t.Helper()
	raw, err := os.ReadFile("docs/language-v2/upstream-preservation-boundary.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest p15U01Manifest
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		t.Fatal(err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		t.Fatal("trailing boundary manifest data")
	}
	return manifest
}

func p15U01ReadSnapshot(t *testing.T) p15U01Snapshot {
	t.Helper()
	head := strings.TrimSpace(string(p15U01Git(t, nil, "rev-parse", "HEAD")))
	if len(head) != 40 {
		t.Fatal("missing full current commit")
	}
	p15U01Git(t, nil, "merge-base", "--is-ancestor", p15U01Baselines["recorded_source"], head)
	status := p15U01Git(t, nil, "status", "--porcelain")
	if len(status) != 0 {
		t.Fatal("repository boundary requires clean tracked source", string(status))
	}
	inventory := strings.Fields(string(p15U01Git(t, nil, "ls-tree", "-r", "--name-only", head, "--", "actions")))
	var paths []string
	for _, path := range inventory {
		if strings.HasSuffix(path, ".cherri") {
			paths = append(paths, path)
		}
	}
	paths = append(paths, "stdfunc.cherri")
	sort.Strings(paths)
	if len(paths) != 26 {
		t.Fatal("actual action/std source inventory changed", paths)
	}
	var requests []string
	for _, role := range []string{"upstream", "language", "continuation", "recorded_source"} {
		for _, path := range paths {
			requests = append(requests, p15U01Baselines[role]+":"+path)
		}
	}
	for _, path := range paths {
		requests = append(requests, head+":"+path)
	}
	raw := p15U01Git(t, []byte(strings.Join(requests, "\n")+"\n"), "cat-file", "--batch")
	reader := bufio.NewReader(bytes.NewReader(raw))
	blobs := make(map[string][]byte)
	for _, request := range requests {
		header, err := reader.ReadString('\n')
		if err != nil {
			t.Fatal("truncated actual Git object stream", err)
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || len(fields[0]) != 40 || fields[1] != "blob" {
			t.Fatal("actual Git object is not a complete blob", request, header)
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 || size > 1<<20 {
			t.Fatal("invalid declaration blob size", request, header)
		}
		data := make([]byte, size)
		if _, err := io.ReadFull(reader, data); err != nil {
			t.Fatal(err)
		}
		separator, err := reader.ReadByte()
		if err != nil || separator != '\n' {
			t.Fatal("bad Git blob separator")
		}
		blobs[request] = data
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		t.Fatal("unrequested Git object output")
	}
	current, tracked := make(map[string][]byte), make(map[string][]byte)
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		current[path] = data
		tracked[path] = blobs[head+":"+path]
	}
	if !bytes.Equal(status, p15U01Git(t, nil, "status", "--porcelain")) || head != strings.TrimSpace(string(p15U01Git(t, nil, "rev-parse", "HEAD"))) {
		t.Fatal("source changed during repository observation")
	}
	return p15U01Snapshot{Head: head, Inventory: paths, Historical: blobs, Current: current, Tracked: tracked, Status: status}
}

func p15U01Digest(raw []byte) string { sum := sha256.Sum256(raw); return hex.EncodeToString(sum[:]) }

func p15U01Validate(manifest p15U01Manifest, snapshot p15U01Snapshot) error {
	if manifest.SchemaVersion != 1 || manifest.RequirementID != "U01" || !reflect.DeepEqual(manifest.Baselines, p15U01Baselines) || manifest.ActionFileCount != 25 || manifest.AdditionalPreservedFile != "stdfunc.cherri" || manifest.Scope == "" {
		return fmt.Errorf("boundary identity does not match the reviewed immutable baselines")
	}
	if len(manifest.Files) != 26 || len(snapshot.Inventory) != 26 || len(snapshot.Current) != 26 || len(snapshot.Tracked) != 26 {
		return fmt.Errorf("declaration inventory is incomplete or extended")
	}
	seen := map[string]bool{}
	changed := 0
	for _, file := range manifest.Files {
		if seen[file.Path] {
			return fmt.Errorf("duplicate boundary file %s", file.Path)
		}
		seen[file.Path] = true
		current, exists := snapshot.Current[file.Path]
		if !exists || !bytes.Equal(current, snapshot.Tracked[file.Path]) {
			return fmt.Errorf("working source differs from its immutable Git blob: %s", file.Path)
		}
		if len(file.Versions) != 4 {
			return fmt.Errorf("incomplete baseline set: %s", file.Path)
		}
		for role, sha := range p15U01Baselines {
			blob, exists := snapshot.Historical[sha+":"+file.Path]
			version, recorded := file.Versions[role]
			if !exists || !recorded || version.Bytes != len(blob) || version.SHA256 != p15U01Digest(blob) {
				return fmt.Errorf("baseline byte identity mismatch for %s/%s", role, file.Path)
			}
		}
		recorded := snapshot.Historical[p15U01Baselines["recorded_source"]+":"+file.Path]
		if !bytes.Equal(current, recorded) {
			return fmt.Errorf("current declaration has an unreviewed source change: %s", file.Path)
		}
		if file.UpstreamIdentical != bytes.Equal(current, snapshot.Historical[p15U01Baselines["upstream"]+":"+file.Path]) || file.LanguageIdentical != bytes.Equal(current, snapshot.Historical[p15U01Baselines["language"]+":"+file.Path]) {
			return fmt.Errorf("historical equality classification is false: %s", file.Path)
		}
		before := snapshot.Historical[p15U01Baselines["continuation"]+":"+file.Path]
		oldToken, newToken, corrected := p15U01AllowedPatch(file.Path)
		if corrected {
			changed++
			if file.ContinuationClass != "parameter-label-correction" || file.ReviewedPatch == nil || file.ReviewedPatch.Before != oldToken || file.ReviewedPatch.After != newToken || file.ReviewedPatch.Reason == "" || file.ImplementationCommit != "dc86a87de5cd6b2f2e795236ac1fa25e28853ade" {
				return fmt.Errorf("unreviewed correction metadata: %s", file.Path)
			}
			if bytes.Count(before, []byte(oldToken)) != 1 || !bytes.Equal(bytes.Replace(before, []byte(oldToken), []byte(newToken), 1), current) {
				return fmt.Errorf("correction altered bytes outside the approved parameter label: %s", file.Path)
			}
		} else if file.ContinuationClass != "unchanged" || file.ReviewedPatch != nil || file.ImplementationCommit != "" || !bytes.Equal(before, current) {
			return fmt.Errorf("preserved declaration was changed or misclassified: %s", file.Path)
		}
	}
	if changed != 2 {
		return fmt.Errorf("reviewed correction count changed")
	}
	for _, path := range snapshot.Inventory {
		if !seen[path] {
			return fmt.Errorf("actual tracked declaration omitted from boundary: %s", path)
		}
	}
	return nil
}

func TestP15U01ActualDeclarationBoundary(t *testing.T) {
	manifest, snapshot := p15U01ReadManifest(t), p15U01ReadSnapshot(t)
	if err := p15U01Validate(manifest, snapshot); err != nil {
		t.Fatal(err)
	}
	// The complete allowed delta is independently compared by bytes above; this
	// original Git diff is retained as an inspectable repository observation.
	diff := p15U01Git(t, nil, "diff", "--numstat", p15U01Baselines["continuation"], snapshot.Head, "--", "actions", "stdfunc.cherri")
	want := "1\t1\tactions/device.cherri\n1\t1\tactions/location.cherri\n"
	if string(diff) != want {
		t.Fatalf("actual repository diff exceeds the reviewed boundary: %q", diff)
	}
	raw, err := json.Marshal(map[string]any{"source_sha": snapshot.Head, "baselines": manifest.Baselines, "tracked_files": manifest.Files, "actual_numstat": string(diff), "source_clean": len(snapshot.Status) == 0, "scope": manifest.Scope})
	if err != nil {
		t.Fatal(err)
	}
	t.Log("P15_U01_REPOSITORY_BOUNDARY:" + string(raw))
}

func TestP15U01DeclarationBoundaryControls(t *testing.T) {
	original, snapshot := p15U01ReadManifest(t), p15U01ReadSnapshot(t)
	if err := p15U01Validate(original, snapshot); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, fault := range []string{"missing_current_file", "extra_current_file", "missing_manifest_file", "duplicate_manifest_file", "wrong_base", "wrong_baseline_digest", "wrong_baseline_size", "rewritten_unchanged_file", "forged_recorded_hash", "wrong_correction", "false_upstream_equality", "false_unchanged_class"} {
		t.Run(fault, func(t *testing.T) {
			var manifest p15U01Manifest
			if err := json.Unmarshal(encoded, &manifest); err != nil {
				t.Fatal(err)
			}
			copy := snapshot
			copy.Current = make(map[string][]byte)
			for path, data := range snapshot.Current {
				copy.Current[path] = append([]byte{}, data...)
			}
			index := -1
			for i := range manifest.Files {
				if manifest.Files[i].Path == "actions/device.cherri" {
					index = i
				}
			}
			if index < 0 {
				t.Fatal("missing independent target correction")
			}
			switch fault {
			case "missing_current_file":
				delete(copy.Current, "actions/basic.cherri")
			case "extra_current_file":
				copy.Current["actions/unreviewed.cherri"] = []byte("action extra()")
			case "missing_manifest_file":
				manifest.Files = manifest.Files[1:]
			case "duplicate_manifest_file":
				manifest.Files[1] = manifest.Files[0]
			case "wrong_base":
				manifest.Baselines["continuation"] = manifest.Baselines["language"]
			case "wrong_baseline_digest":
				v := manifest.Files[0].Versions["upstream"]
				v.SHA256 = strings.Repeat("0", 64)
				manifest.Files[0].Versions["upstream"] = v
			case "wrong_baseline_size":
				v := manifest.Files[0].Versions["language"]
				v.Bytes++
				manifest.Files[0].Versions["language"] = v
			case "rewritten_unchanged_file":
				copy.Current["actions/basic.cherri"] = append(copy.Current["actions/basic.cherri"], []byte("\n// unreviewed conversion\n")...)
			case "forged_recorded_hash":
				copy.Current[manifest.Files[index].Path] = append(copy.Current[manifest.Files[index].Path], ' ')
				v := manifest.Files[index].Versions["recorded_source"]
				v.SHA256 = p15U01Digest(copy.Current[manifest.Files[index].Path])
				v.Bytes++
				manifest.Files[index].Versions["recorded_source"] = v
			case "wrong_correction":
				manifest.Files[index].ReviewedPatch.After = "text ?endTime: 'unrelatedNativeKey'"
			case "false_upstream_equality":
				manifest.Files[index].UpstreamIdentical = !manifest.Files[index].UpstreamIdentical
			case "false_unchanged_class":
				manifest.Files[index].ContinuationClass = "unchanged"
			}
			if p15U01Validate(manifest, copy) == nil {
				t.Fatal("damaged repository boundary accepted", fault)
			}
		})
	}
}

func TestP15U01GitReplacementIsolation(t *testing.T) {
	repository := t.TempDir()
	// Register automatic restoration, then remove the variable entirely: Git
	// versions may interpret mere presence as disabling replacement objects.
	t.Setenv("GIT_NO_REPLACE_OBJECTS", "0")
	if err := os.Unsetenv("GIT_NO_REPLACE_OBJECTS"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GIT_REPLACE_REF_BASE", "refs/replace/")
	p15U01Git(t, nil, "init", "--quiet", repository)
	original, forged := []byte("original declaration\n"), []byte("forged declaration\n")
	makeCommit := func(data []byte, label string) (string, string, string) {
		blob := strings.TrimSpace(string(p15U01Git(t, data, "-C", repository, "hash-object", "-w", "--stdin")))
		tree := strings.TrimSpace(string(p15U01Git(t, []byte("100644 blob "+blob+"\tdeclaration.cherri\n"), "-C", repository, "mktree")))
		commit := strings.TrimSpace(string(p15U01Git(t, nil, "-C", repository,
			"-c", "user.name=U01 isolated fixture", "-c", "user.email=u01-fixture@example.invalid",
			"-c", "commit.gpgsign=false", "commit-tree", tree, "-m", label)))
		return commit, tree, blob
	}
	before, _, originalBlob := makeCommit(original, "original synthetic declaration")
	replacement, _, _ := makeCommit(forged, "replacement synthetic declaration")
	p15U01Git(t, nil, "-C", repository, "replace", before, replacement)
	// Verify that the fault is active using an intentionally unhardened read in
	// this disposable repository. The real shared repository is never replaced.
	raw := exec.Command("git", "-C", repository, "show", before+":declaration.cherri")
	output, err := raw.Output()
	if err != nil || !bytes.Equal(output, forged) {
		t.Fatalf("isolated replacement-ref fault was not active: %v %q", err, output)
	}
	for _, operation := range []string{"show", "cat_file_batch", "ls_tree", "diff"} {
		t.Run(operation, func(t *testing.T) {
			switch operation {
			case "show":
				got := p15U01Git(t, nil, "-C", repository, "show", before+":declaration.cherri")
				if !bytes.Equal(got, original) {
					t.Fatalf("immutable show followed replacement ref: %q", got)
				}
			case "cat_file_batch":
				got := p15U01Git(t, []byte(before+":declaration.cherri\n"), "-C", repository, "cat-file", "--batch")
				want := []byte(originalBlob + " blob " + strconv.Itoa(len(original)) + "\n" + string(original) + "\n")
				if !bytes.Equal(got, want) {
					t.Fatalf("immutable batch object followed replacement ref: %q", got)
				}
			case "ls_tree":
				got := p15U01Git(t, nil, "-C", repository, "ls-tree", before, "--", "declaration.cherri")
				want := []byte("100644 blob " + originalBlob + "\tdeclaration.cherri\n")
				if !bytes.Equal(got, want) {
					t.Fatalf("immutable tree followed replacement ref: %q", got)
				}
			case "diff":
				got := p15U01Git(t, nil, "-C", repository, "diff", "--numstat", before, replacement, "--", "declaration.cherri")
				if string(got) != "1\t1\tdeclaration.cherri\n" {
					t.Fatalf("immutable diff concealed replacement: %q", got)
				}
			}
		})
	}
}
