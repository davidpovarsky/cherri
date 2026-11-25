package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const finalProvenanceTestID = "tools/language-acceptance:verifyFinalDelivery"
const finalProvenanceScope = "Repository delivery integrity only; no CI03, CI04, RP49, BRG31, Apple, model-quality, or global closure credit"
const finalProvenanceTrustBoundary = "independent authenticated operator context; file hashes are integrity checks, not authentication"

var finalProvenanceEnvironment = []string{
	"CHERRI_FINAL_PROVENANCE_EXPECTED", "CHERRI_FINAL_PROVENANCE_EXPECTED_SHA256", "CHERRI_FINAL_PROVENANCE_MANIFEST",
}
var finalProvenanceSources = []string{
	"tools/language-acceptance/docs_publication_execution.go",
	"tools/language-acceptance/docs_publication_execution_test.go",
	"tools/language-acceptance/final_provenance_execution.go",
	"tools/language-acceptance/final_provenance_execution_test.go",
	"tools/language-acceptance/ci_identity_execution.go",
	"tools/language-acceptance/ci_identity_execution_test.go",
	"tools/language-acceptance/main.go", "tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go", "tools/language-acceptance/assertion_scope.go",
	"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json", "tests/language-v2/backend-recovery-gates.json",
}

// This context is supplied through an independent operator channel. The delivery
// manifest cannot supply its own expected context, hashes, source identities,
// authenticated API originals, or required artifact inventory.
type finalProvenanceConfig struct{ ExpectedPath, ExpectedSHA256, ManifestPath string }
type finalProvenanceFile struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
}
type finalProvenanceSource struct {
	Repository   string `json:"repository"`
	SHA          string `json:"sha"`
	Tree         string `json:"tree"`
	LocalRef     string `json:"local_ref"`
	PublishedRef string `json:"published_ref"`
}
type finalProvenanceStreams struct {
	Stdout string `json:"stdout"`
	Stderr string `json:"stderr"`
}
type finalProvenanceReconciliation struct {
	Result           string                            `json:"result"`
	OriginalArchive  string                            `json:"original_archive"`
	Archive          string                            `json:"archive"`
	OriginalReceipts map[string]string                 `json:"original_receipts"`
	Streams          map[string]finalProvenanceStreams `json:"streams"`
	Receipts         map[string]string                 `json:"receipts"`
	OriginalMembers  map[string]ciArchiveMember        `json:"original_members"`
	Members          map[string]ciArchiveMember        `json:"members"`
}
type finalProvenancePublication struct {
	Format         string                            `json:"format"`
	Result         string                            `json:"result"`
	Streams        map[string]finalProvenanceStreams `json:"streams,omitempty"`
	Push           string                            `json:"push,omitempty"`
	Ref            string                            `json:"ref,omitempty"`
	Commit         string                            `json:"commit,omitempty"`
	PushStreams    *finalProvenanceStreams           `json:"push_streams,omitempty"`
	Receipts       map[string]string                 `json:"receipts,omitempty"`
	PublisherRef   string                            `json:"publisher_local_ref,omitempty"`
	Reconciliation *finalProvenanceReconciliation    `json:"reconciliation,omitempty"`
}
type finalProvenanceRun struct {
	Original     string `json:"original"`
	Repository   string `json:"repository"`
	ID           int64  `json:"id"`
	Attempt      int    `json:"attempt"`
	WorkflowID   int64  `json:"workflow_id"`
	WorkflowPath string `json:"workflow_path"`
	Event        string `json:"event"`
	SourceSHA    string `json:"source_sha"`
	Status       string `json:"status"`
	Conclusion   string `json:"conclusion"`
	Historical   bool   `json:"historical"`
}
type finalProvenanceArtifact struct {
	File         string `json:"file"`
	Role         string `json:"role"`
	SourceSHA    string `json:"source_sha"`
	Historical   bool   `json:"historical"`
	Availability string `json:"availability"` // github or local; local is never a published link
	Run          string `json:"run,omitempty"`
	Metadata     string `json:"metadata,omitempty"`
	ID           int64  `json:"id,omitempty"`
	Name         string `json:"name"`
	URL          string `json:"url,omitempty"`
}
type finalProvenanceLedger struct {
	SourceSHA     string   `json:"source_sha"`
	Report        string   `json:"report"`
	Audit         string   `json:"audit"`
	Process       string   `json:"process"`
	SourceAfter   string   `json:"source_after"`
	Evidence      string   `json:"evidence"`
	Observations  string   `json:"observations"`
	ArtifactRoots []string `json:"artifact_roots"`
}
type finalProvenanceExpected struct {
	Schema                string                                `json:"schema"`
	ManifestSHA256        string                                `json:"manifest_sha256"`
	Sources               map[string]finalProvenanceSource      `json:"sources"`
	Files                 map[string]finalProvenanceFile        `json:"files"`
	Publications          map[string]finalProvenancePublication `json:"publications"`
	Runs                  map[string]finalProvenanceRun         `json:"runs"`
	Artifacts             map[string]finalProvenanceArtifact    `json:"artifacts"`
	Ledger                *finalProvenanceLedger                `json:"ledger"`
	AdditionalLimitations []string                              `json:"additional_limitations"`
}
type finalProvenanceLedgerRow struct {
	ID        string   `json:"id"`
	SourceSHA string   `json:"source_sha"`
	Status    string   `json:"status"`
	Detail    string   `json:"detail"`
	Evidence  []string `json:"evidence"`
}
type finalProvenanceManifest struct {
	Schema                string                     `json:"schema"`
	SourceDirectories     map[string]string          `json:"source_directories"`
	Files                 map[string]string          `json:"files"`
	Ledger                []finalProvenanceLedgerRow `json:"requirement_ledger"`
	Limitations           []finalProvenanceLedgerRow `json:"unresolved_requirements"`
	AdditionalLimitations []string                   `json:"additional_limitations"`
	ClosureComplete       bool                       `json:"closure_complete"`
}
type finalProvenanceGitCommand struct {
	Args         []string `json:"args"`
	ExitCode     int      `json:"exit_code"`
	TimedOut     bool     `json:"timed_out"`
	StdoutSHA256 string   `json:"stdout_sha256"`
	StderrSHA256 string   `json:"stderr_sha256"`
}
type finalProvenanceSnapshot struct {
	SHA      string                      `json:"sha"`
	Tree     string                      `json:"tree"`
	Ref      string                      `json:"ref"`
	Origin   string                      `json:"origin"`
	Clean    bool                        `json:"clean"`
	Commands []finalProvenanceGitCommand `json:"commands"`
}
type finalProvenanceResult struct {
	Schema                     string                             `json:"schema"`
	StartedUTC                 string                             `json:"started_utc"`
	FinishedUTC                string                             `json:"finished_utc"`
	Status                     string                             `json:"status"`
	Detail                     string                             `json:"detail"`
	SourceSHA                  string                             `json:"source_sha"`
	LedgerSourceSHA            string                             `json:"ledger_source_sha,omitempty"`
	ExpectedSHA256             string                             `json:"expected_sha256,omitempty"`
	ManifestSHA256             string                             `json:"manifest_sha256,omitempty"`
	Before                     map[string]finalProvenanceSnapshot `json:"source_before,omitempty"`
	After                      map[string]finalProvenanceSnapshot `json:"source_after,omitempty"`
	ReportSHA256               string                             `json:"report_sha256,omitempty"`
	AuditSHA256                string                             `json:"audit_sha256,omitempty"`
	ProcessSHA256              string                             `json:"process_sha256,omitempty"`
	OriginalProcessFinishedUTC string                             `json:"original_process_finished_utc,omitempty"`
	StatusCounts               map[string]int                     `json:"original_status_counts,omitempty"`
	OriginalClosureComplete    bool                               `json:"original_closure_complete"`
	Ledger                     []finalProvenanceLedgerRow         `json:"original_requirement_ledger,omitempty"`
	Limitations                []finalProvenanceLedgerRow         `json:"original_unresolved_requirements,omitempty"`
	Runs                       map[string]finalProvenanceRun      `json:"original_runs,omitempty"`
	Artifacts                  map[string]finalProvenanceArtifact `json:"delivered_artifacts,omitempty"`
	AdditionalLimitations      []string                           `json:"additional_limitations,omitempty"`
	AuthenticationBoundary     string                             `json:"authentication_boundary"`
	CheckoutProvenanceGranted  bool                               `json:"ci_checkout_provenance_granted"`
	FormalAcceptanceGranted    bool                               `json:"formal_acceptance_granted"`
	Scope                      string                             `json:"scope"`
}
type finalProvenancePending string

func (e finalProvenancePending) Error() string { return string(e) }

func finalProvenanceDigest(b []byte) string { s := sha256.Sum256(b); return hex.EncodeToString(s[:]) }
func finalProvenanceHex(s string, n int) bool {
	return len(s) == n && strings.Trim(s, "0123456789abcdef") == ""
}
func finalProvenanceConfiguration() (finalProvenanceConfig, bool, error) {
	c := finalProvenanceConfig{os.Getenv(finalProvenanceEnvironment[0]), os.Getenv(finalProvenanceEnvironment[1]), os.Getenv(finalProvenanceEnvironment[2])}
	if c.ExpectedPath == "" && c.ExpectedSHA256 == "" && c.ManifestPath == "" {
		return c, false, nil
	}
	if !filepath.IsAbs(c.ExpectedPath) || !filepath.IsAbs(c.ManifestPath) || !finalProvenanceHex(c.ExpectedSHA256, 64) || filepath.Clean(c.ExpectedPath) == filepath.Clean(c.ManifestPath) {
		return c, true, fmt.Errorf("final delivery requires separate absolute expected-context and manifest paths and an independently supplied context digest")
	}
	return c, true, nil
}
func finalProvenanceRead(path string, pin finalProvenanceFile, retain bool) ([]byte, error) {
	if !filepath.IsAbs(path) || !finalProvenanceHex(pin.SHA256, 64) || pin.Bytes < 0 || pin.Bytes > 512*1024*1024 {
		return nil, fmt.Errorf("invalid original file identity")
	}
	before, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return nil, finalProvenancePending("a required independently pinned original is absent")
	}
	if err != nil || !before.Mode().IsRegular() || before.Mode()&os.ModeSymlink != 0 || before.Size() != pin.Bytes {
		return nil, fmt.Errorf("original is not the pinned regular file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("original is not readable")
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("original identity changed before read")
	}
	h := sha256.New()
	var content bytes.Buffer
	var destination io.Writer = h
	if retain {
		if pin.Bytes > 4*1024*1024 {
			return nil, fmt.Errorf("oversized JSON or process original")
		}
		destination = io.MultiWriter(h, &content)
	}
	n, err := io.Copy(destination, io.LimitReader(f, pin.Bytes+1))
	after, statErr := os.Lstat(path)
	if err != nil || statErr != nil || n != pin.Bytes || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || hex.EncodeToString(h.Sum(nil)) != pin.SHA256 {
		return nil, fmt.Errorf("original bytes or identity disagree with independent pin")
	}
	return content.Bytes(), nil
}
func finalProvenanceReadContext(path, digest string, value any) error {
	i, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return finalProvenancePending("independent delivery context or manifest is absent")
	}
	if err != nil {
		return fmt.Errorf("delivery context is unreadable")
	}
	b, err := finalProvenanceRead(path, finalProvenanceFile{digest, i.Size()}, true)
	if err != nil {
		return err
	}
	if _, err = decodeNativeEncodingObject(b); err != nil {
		return fmt.Errorf("ambiguous delivery JSON: %w", err)
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(value); err != nil {
		return fmt.Errorf("invalid delivery schema: %w", err)
	}
	return nil
}
func finalProvenanceWithin(parent, child string) bool {
	r, err := filepath.Rel(parent, child)
	return err == nil && (r == "." || (r != ".." && !strings.HasPrefix(r, ".."+string(filepath.Separator))))
}

// Local Git inspection only. Routing variables and optional process helpers may
// not redirect the intended checkout or suppress the dirty-worktree observation.
func finalProvenanceGit(ctx context.Context, root string, args ...string) ([]byte, finalProvenanceGitCommand, error) {
	child, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	argv := append([]string{"--no-replace-objects", "-C", root, "-c", "core.fsmonitor=false", "-c", "core.untrackedCache=false", "-c", "core.ignoreStat=false"}, args...)
	cmd := exec.CommandContext(child, "git", argv...)
	for _, entry := range os.Environ() {
		if !strings.HasPrefix(entry, "GIT_") {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_OPTIONAL_LOCKS=0", "GIT_NO_REPLACE_OBJECTS=1")
	var out, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &stderr
	err := cmd.Run()
	code := 0
	if err != nil {
		code = -1
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
	}
	receipt := finalProvenanceGitCommand{argv, code, child.Err() != nil, finalProvenanceDigest(out.Bytes()), finalProvenanceDigest(stderr.Bytes())}
	if err != nil {
		return nil, receipt, fmt.Errorf("required local Git observation failed")
	}
	return out.Bytes(), receipt, nil
}
func finalProvenanceSnapshotSource(ctx context.Context, root string, expected finalProvenanceSource) (finalProvenanceSnapshot, error) {
	var snapshot finalProvenanceSnapshot
	if !filepath.IsAbs(root) {
		return snapshot, fmt.Errorf("source directory must be absolute")
	}
	resolved, err := filepath.EvalSymlinks(root)
	if err != nil {
		return snapshot, finalProvenancePending("required source checkout is absent")
	}
	get := func(args ...string) (string, error) {
		b, r, e := finalProvenanceGit(ctx, root, args...)
		snapshot.Commands = append(snapshot.Commands, r)
		return strings.TrimSpace(string(b)), e
	}
	top, err := get("rev-parse", "--show-toplevel")
	if err != nil || filepath.Clean(top) != filepath.Clean(resolved) {
		return snapshot, fmt.Errorf("source directory is not the actual Git worktree root")
	}
	if snapshot.SHA, err = get("rev-parse", "HEAD"); err != nil || snapshot.SHA != expected.SHA {
		return snapshot, fmt.Errorf("source HEAD differs from independently expected source")
	}
	if snapshot.Tree, err = get("rev-parse", "HEAD^{tree}"); err != nil || snapshot.Tree != expected.Tree {
		return snapshot, fmt.Errorf("source tree differs from independently expected tree")
	}
	if snapshot.Ref, err = get("symbolic-ref", "-q", "HEAD"); err != nil || snapshot.Ref != expected.LocalRef {
		return snapshot, fmt.Errorf("source is detached or on the wrong named branch")
	}
	if snapshot.Origin, err = get("remote", "get-url", "origin"); err != nil {
		return snapshot, err
	}
	canonical := "https://github.com/" + expected.Repository
	if snapshot.Origin != canonical && snapshot.Origin != canonical+".git" && snapshot.Origin != "git@github.com:"+expected.Repository+".git" {
		return snapshot, fmt.Errorf("source origin is not the expected canonical repository")
	}
	status, err := get("status", "--porcelain=v1", "--untracked-files=all", "--ignore-submodules=none")
	if err != nil || status != "" {
		return snapshot, fmt.Errorf("source worktree or index is not clean")
	}
	flags, err := get("ls-files", "-v", "-z")
	if err != nil {
		return snapshot, err
	}
	for _, item := range strings.Split(flags, "\x00") {
		if item != "" && (item[0] == 'S' || item[0] >= 'a' && item[0] <= 'z') {
			return snapshot, fmt.Errorf("source index hides tracked working-tree content")
		}
	}
	for _, arg := range [][]string{{"for-each-ref", "--format=%(refname)", "refs/replace/"}, {"diff", "--name-only", "HEAD", "--"}} {
		text, err := get(arg...)
		if err != nil || text != "" {
			return snapshot, fmt.Errorf("source has replacement refs or unobserved tracked changes")
		}
	}
	snapshot.Clean = true
	return snapshot, nil
}

type finalProvenanceInputs struct {
	Expected finalProvenanceExpected
	Manifest finalProvenanceManifest
	JSON     map[string]map[string]any
	Bytes    map[string][]byte
}

func (in *finalProvenanceInputs) object(id string) (map[string]any, error) {
	if object := in.JSON[id]; object != nil {
		return object, nil
	}
	b, exists := in.Bytes[id]
	if !exists {
		return nil, fmt.Errorf("required original object is not in the independent inventory")
	}
	o, err := decodeNativeEncodingObject(b)
	if err != nil {
		return nil, fmt.Errorf("malformed original JSON for %s", id)
	}
	in.JSON[id] = o
	return o, nil
}
func finalProvenanceString(o map[string]any, key string) string { s, _ := o[key].(string); return s }
func finalProvenanceNumber(o map[string]any, key string) (int64, bool) {
	n, ok := o[key].(json.Number)
	if !ok {
		return 0, false
	}
	v, e := n.Int64()
	return v, e == nil
}
func finalProvenanceBool(o map[string]any, key string, value bool) bool {
	b, ok := o[key].(bool)
	return ok && b == value
}
func finalProvenanceObject(o map[string]any, key string) map[string]any {
	p, _ := o[key].(map[string]any)
	return p
}
func finalProvenanceArray(o map[string]any, key string) []any { p, _ := o[key].([]any); return p }
func finalProvenanceTime(s string) (time.Time, error) {
	t, e := time.Parse(time.RFC3339Nano, s)
	if e != nil || t.IsZero() {
		return t, fmt.Errorf("invalid original timestamp")
	}
	return t, nil
}
func finalProvenanceStrings(v any) ([]string, bool) {
	a, ok := v.([]any)
	if !ok {
		return nil, false
	}
	r := make([]string, len(a))
	for i, x := range a {
		s, ok := x.(string)
		if !ok {
			return nil, false
		}
		r[i] = s
	}
	return r, true
}

func finalProvenanceLoad(config finalProvenanceConfig, outputDirectory string) (*finalProvenanceInputs, error) {
	in := &finalProvenanceInputs{JSON: map[string]map[string]any{}, Bytes: map[string][]byte{}}
	if err := finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &in.Expected); err != nil {
		return nil, err
	}
	e := &in.Expected
	if e.Schema != "cherri-final-provenance-expected/1" || !finalProvenanceHex(e.ManifestSHA256, 64) {
		return nil, fmt.Errorf("invalid independently pinned END01 context")
	}
	if e.Ledger == nil {
		return in, finalProvenancePending("a finalized 179 ledger at the delivered implementation SHA is required")
	}
	if e.Ledger.SourceSHA != e.Sources["implementation"].SHA {
		return in, fmt.Errorf("configured finalized ledger is for a different implementation SHA; a historical ledger cannot substitute")
	}
	if err := finalProvenanceReadContext(config.ManifestPath, e.ManifestSHA256, &in.Manifest); err != nil {
		return nil, err
	}
	if in.Manifest.Schema != "cherri-final-provenance-delivery/1" || !reflect.DeepEqual(in.Manifest.AdditionalLimitations, e.AdditionalLimitations) {
		return nil, fmt.Errorf("delivery manifest schema or limitations differ from independent context")
	}
	if err := finalProvenanceLoadBindings(in, outputDirectory); err != nil {
		return nil, err
	}
	return in, nil
}

// Low-level integrity joins only. A dedicated caller (END01, CI03, or CI04)
// must first authenticate its own independently pinned expected context and
// decode its own strict schema. This helper does not authenticate a candidate,
// require a completed ledger, grant a tier, or certify checkout/privacy scope.
func finalProvenanceLoadBindings(in *finalProvenanceInputs, outputDirectory string) error {
	e, m := &in.Expected, &in.Manifest
	if in.JSON == nil {
		in.JSON = map[string]map[string]any{}
	}
	if in.Bytes == nil {
		in.Bytes = map[string][]byte{}
	}
	if len(e.Sources) != 3 || len(e.Publications) != 3 || len(e.Files) == 0 || len(e.Files) > 512 || len(e.Artifacts) == 0 || len(e.Artifacts) > 64 || len(e.Runs) == 0 || len(e.Runs) > 32 {
		return fmt.Errorf("incomplete independent delivery inventory")
	}
	for _, role := range []string{"implementation", "compiler", "docs"} {
		s, ok := e.Sources[role]
		expectedRepo := "davidpovarsky/cherri"
		if role == "docs" {
			expectedRepo = "davidpovarsky/cherrilang.org"
		}
		if !ok || s.Repository != expectedRepo || !finalProvenanceHex(s.SHA, 40) || !finalProvenanceHex(s.Tree, 40) || !strings.HasPrefix(s.LocalRef, "refs/heads/") || !strings.HasPrefix(s.PublishedRef, "refs/heads/") {
			return fmt.Errorf("invalid independently expected source tuple")
		}
		if _, ok = e.Publications[role]; !ok {
			return fmt.Errorf("missing source publication binding")
		}
	}
	if len(m.SourceDirectories) != 3 || len(m.Files) != len(e.Files) {
		return fmt.Errorf("delivery manifest does not match the independent inventory")
	}
	for _, s := range e.AdditionalLimitations {
		if strings.TrimSpace(s) == "" {
			return fmt.Errorf("empty delivery limitation")
		}
	}
	resolvedOutput := ""
	if outputDirectory != "" {
		if !filepath.IsAbs(outputDirectory) {
			return fmt.Errorf("delivery output directory must be absolute")
		}
		var err error
		resolvedOutput, err = filepath.EvalSymlinks(outputDirectory)
		if err != nil {
			return fmt.Errorf("delivery output directory is unavailable")
		}
		info, err := os.Stat(resolvedOutput)
		if err != nil || !info.IsDir() {
			return fmt.Errorf("delivery output parent is not a directory")
		}
	}
	for role := range e.Sources {
		if !filepath.IsAbs(m.SourceDirectories[role]) {
			return fmt.Errorf("missing source directory locator")
		}
		resolvedSource, err := filepath.EvalSymlinks(m.SourceDirectories[role])
		if os.IsNotExist(err) {
			return finalProvenancePending("required source checkout is absent")
		}
		if err != nil {
			return fmt.Errorf("source directory cannot be resolved")
		}
		if resolvedOutput != "" && finalProvenanceWithin(resolvedSource, resolvedOutput) {
			return fmt.Errorf("delivery evidence must not be written inside a delivered source checkout")
		}
	}
	for id, pin := range e.Files {
		if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,100}$`).MatchString(id) {
			return fmt.Errorf("invalid original inventory identifier")
		}
		path, ok := m.Files[id]
		if !ok || !filepath.IsAbs(path) {
			return fmt.Errorf("original locator is absent or circular")
		}
		resolvedPath, err := filepath.EvalSymlinks(path)
		if os.IsNotExist(err) {
			return finalProvenancePending("a required independently pinned original is absent")
		}
		if err != nil {
			return fmt.Errorf("original locator cannot be resolved")
		}
		if resolvedOutput != "" && finalProvenanceWithin(resolvedOutput, resolvedPath) {
			return fmt.Errorf("original locator is absent or circular")
		}
		retain := pin.Bytes <= 4*1024*1024
		b, err := finalProvenanceRead(path, pin, retain)
		if err != nil {
			return fmt.Errorf("%s: %w", id, err)
		}
		if retain {
			in.Bytes[id] = b
		}
	}
	return nil
}

func finalProvenanceCommandTail(value any, verb string) ([]string, bool) {
	a, ok := finalProvenanceStrings(value)
	if !ok {
		return nil, false
	}
	index := -1
	for i, s := range a {
		if s == verb {
			if index != -1 {
				return nil, false
			}
			index = i
		}
	}
	if index < 0 {
		return nil, false
	}
	return a[index:], true
}
func finalProvenanceStreamCheck(in *finalProvenanceInputs, step map[string]any, streams finalProvenanceStreams) error {
	for key, id := range map[string]string{"stdout": streams.Stdout, "stderr": streams.Stderr} {
		pin, ok := in.Expected.Files[id]
		if !ok || finalProvenanceString(step, key+"SHA256") != pin.SHA256 {
			return fmt.Errorf("publication stream is not bound to original bytes")
		}
		if n, present := finalProvenanceNumber(step, key+"Bytes"); present && n != pin.Bytes {
			return fmt.Errorf("publication stream byte count mismatch")
		}
	}
	return nil
}
func finalProvenanceValidatePublication(in *finalProvenanceInputs, role string) error {
	p := in.Expected.Publications[role]
	s := in.Expected.Sources[role]
	o, err := in.object(p.Result)
	if err != nil {
		return err
	}
	readback := s.SHA + "\t" + s.PublishedRef
	switch p.Format {
	case "oracle-atomic-create-only/1":
		return finalProvenanceOraclePublication(in, p, s, o)
	case "windows-create-only-reconciled/1":
		return finalProvenanceReconciledWindowsPublication(in, p, s, o)
	case "windows-create-only/1":
		if finalProvenanceString(o, "sourceCommit") != s.SHA || finalProvenanceString(o, "sourceTree") != s.Tree || finalProvenanceString(o, "ref") != s.PublishedRef || finalProvenanceString(o, "status") != "PUBLISHED_REMOTE_VERIFIED" || !finalProvenanceBool(o, "pushAttempted", true) || !finalProvenanceBool(o, "pushTimedOut", false) || !finalProvenanceBool(o, "destinationMustBeAbsent", true) || !finalProvenanceBool(o, "primaryMutationAuthorized", false) || !finalProvenanceBool(o, "primaryUnchanged", true) {
			return fmt.Errorf("Windows publication did not establish the expected single source publication")
		}
		limit, ok := finalProvenanceNumber(o, "pushAttemptLimit")
		code, codeOK := finalProvenanceNumber(o, "pushExitCode")
		if !ok || limit != 1 || !codeOK || code != 0 {
			return fmt.Errorf("Windows publication push did not succeed once")
		}
		names := []string{"01-origin", "02-base-refs", "03-primary-head-before", "04-primary-status-before", "05-transfer", "06-bundle-heads", "07-bundle-verify", "08-import", "09-commit", "10-tree", "11-ancestry", "12-connectivity", "13-imported-refs", "14-remote-before", "15-push", "16-remote-after", "17-api-ref", "18-api-commit", "21-refs-after-publication", "19-primary-head-after", "20-primary-status-after"}
		steps := finalProvenanceArray(o, "steps")
		if len(steps) != len(names) || len(p.Streams) != len(names) {
			return fmt.Errorf("incomplete original publication step inventory")
		}
		byName := map[string]map[string]any{}
		var previous time.Time
		for i, raw := range steps {
			step, ok := raw.(map[string]any)
			if !ok || finalProvenanceString(step, "name") != names[i] {
				return fmt.Errorf("publication step identity or order differs")
			}
			name := names[i]
			streams, exists := p.Streams[name]
			if !exists {
				return fmt.Errorf("missing publication stream pair")
			}
			if err = finalProvenanceStreamCheck(in, step, streams); err != nil {
				return err
			}
			want := int64(0)
			if name == "14-remote-before" {
				want = 2
			}
			code, ok := finalProvenanceNumber(step, "exitCode")
			if !ok || code != want || !finalProvenanceBool(step, "timedOut", false) {
				return fmt.Errorf("publication contains a failed or timed-out prerequisite")
			}
			started, e1 := finalProvenanceTime(finalProvenanceString(step, "startedAtUtc"))
			ended, e2 := finalProvenanceTime(finalProvenanceString(step, "finishedAtUtc"))
			if e1 != nil || e2 != nil || ended.Before(started) || (!previous.IsZero() && started.Before(previous)) {
				return fmt.Errorf("publication step chronology is inconsistent")
			}
			previous = ended
			byName[name] = step
		}
		origin := strings.TrimSpace(string(in.Bytes[p.Streams["01-origin"].Stdout]))
		if origin != "https://github.com/"+s.Repository+".git" && origin != "https://github.com/"+s.Repository {
			return fmt.Errorf("publication origin mismatch")
		}
		push, ok := finalProvenanceCommandTail(byName["15-push"]["argv"], "push")
		if !ok || !reflect.DeepEqual(push, []string{"push", "--porcelain", "--force-with-lease=" + s.PublishedRef + ":", "origin", s.SHA + ":" + s.PublishedRef}) {
			return fmt.Errorf("publication push scope differs from the sole create-only ref")
		}
		for _, name := range []string{"14-remote-before", "16-remote-after"} {
			tail, ok := finalProvenanceCommandTail(byName[name]["argv"], "ls-remote")
			if !ok || !reflect.DeepEqual(tail, []string{"ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef}) {
				return fmt.Errorf("publication remote observation targets another ref")
			}
		}
		if len(in.Bytes[p.Streams["14-remote-before"].Stdout]) != 0 || strings.TrimSpace(string(in.Bytes[p.Streams["16-remote-after"].Stdout])) != readback {
			return fmt.Errorf("publication absence/readback originals disagree")
		}
		ref, err := in.object(p.Streams["17-api-ref"].Stdout)
		if err != nil {
			return err
		}
		commit, err := in.object(p.Streams["18-api-commit"].Stdout)
		if err != nil {
			return err
		}
		if finalProvenanceString(ref, "ref") != s.PublishedRef || finalProvenanceString(ref, "sha") != s.SHA || finalProvenanceString(ref, "type") != "commit" || finalProvenanceString(commit, "sha") != s.SHA || finalProvenanceString(commit, "tree") != s.Tree {
			return fmt.Errorf("independent GitHub ref/commit readback does not bind source and tree")
		}
		remote := finalProvenanceObject(o, "remoteBinding")
		if !reflect.DeepEqual(finalProvenanceObject(remote, "ref"), ref) || !reflect.DeepEqual(finalProvenanceObject(remote, "commit"), commit) {
			return fmt.Errorf("publication result changed the raw remote binding")
		}
		primaryBefore := finalProvenanceObject(finalProvenanceObject(o, "validation"), "primaryBefore")
		primaryAfter := finalProvenanceObject(o, "primaryAfter")
		for _, primary := range []map[string]any{primaryBefore, primaryAfter} {
			head, headOK := primary["head"].(string)
			_, statusOK := primary["status"].(string)
			fixture, fixtureOK := primary["preservedFixtureSHA256"].(string)
			if len(primary) != 3 || !headOK || !finalProvenanceHex(head, 40) || !statusOK || !fixtureOK || !finalProvenanceHex(fixture, 64) {
				return fmt.Errorf("Windows primary checkout observation is missing or malformed")
			}
		}
		if !reflect.DeepEqual(primaryBefore, primaryAfter) {
			return fmt.Errorf("publication changed the unrelated primary checkout")
		}
		var primaryCommand []string
		for _, original := range []struct {
			name, field string
			command     []string
			primary     map[string]any
		}{
			{"03-primary-head-before", "head", []string{"rev-parse", "HEAD"}, primaryBefore},
			{"04-primary-status-before", "status", []string{"status", "--porcelain=v1"}, primaryBefore},
			{"19-primary-head-after", "head", []string{"rev-parse", "HEAD"}, primaryAfter},
			{"20-primary-status-after", "status", []string{"status", "--porcelain=v1"}, primaryAfter},
		} {
			argv, ok := finalProvenanceStrings(byName[original.name]["argv"])
			if !ok || len(argv) != 5 || argv[0] == "" || argv[1] != "-C" || argv[2] == "" || !reflect.DeepEqual(argv[3:], original.command) {
				return fmt.Errorf("Windows primary checkout command differs from original observation")
			}
			if primaryCommand == nil {
				primaryCommand = argv[:3]
			} else if !reflect.DeepEqual(primaryCommand, argv[:3]) {
				return fmt.Errorf("Windows primary checkout commands target different checkouts")
			}
			if strings.TrimSpace(string(in.Bytes[p.Streams[original.name].Stdout])) != original.primary[original.field] {
				return fmt.Errorf("Windows primary checkout objects disagree with original streams")
			}
		}
	case "source-create-only/1":
		if finalProvenanceString(o, "sourceCommit") != s.SHA || finalProvenanceString(o, "ref") != s.PublishedRef || finalProvenanceString(o, "status") != "PASS" || !finalProvenanceBool(o, "sourceCleanAfter", true) {
			return fmt.Errorf("source publication result does not bind the expected source")
		}
		steps := finalProvenanceArray(o, "steps")
		if len(steps) != 3 {
			return fmt.Errorf("source publication requires its three original operations")
		}
		for i, raw := range steps {
			step, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid source publication operation")
			}
			code, ok := finalProvenanceNumber(step, "exitCode")
			if !ok || code != 0 {
				return fmt.Errorf("source publication operation failed")
			}
			if i == 1 {
				tail, ok := finalProvenanceCommandTail(step["command"], "push")
				if !ok || !reflect.DeepEqual(tail, []string{"push", "--porcelain", "origin", s.SHA + ":" + s.PublishedRef}) {
					return fmt.Errorf("source publication is not the original nonforce single-ref push")
				}
				out := finalProvenanceString(step, "stdout")
				if !strings.Contains(out, "To https://github.com/"+s.Repository+".git\n") || !strings.Contains(out, "*\t"+s.SHA+":"+s.PublishedRef+"\t[new branch]\n") {
					return fmt.Errorf("source push did not report the expected new branch")
				}
			} else {
				tail, ok := finalProvenanceCommandTail(step["command"], "ls-remote")
				if !ok || !reflect.DeepEqual(tail, []string{"ls-remote", "--heads", "origin", s.PublishedRef}) {
					return fmt.Errorf("source readback targeted another ref")
				}
				want := ""
				if i == 2 {
					want = readback
				}
				if strings.TrimSpace(finalProvenanceString(step, "stdout")) != want {
					return fmt.Errorf("source absence/readback does not match")
				}
			}
		}
		start, e1 := finalProvenanceTime(finalProvenanceString(o, "startedAt"))
		end, e2 := finalProvenanceTime(finalProvenanceString(o, "finishedAt"))
		if e1 != nil || e2 != nil || end.Before(start) {
			return fmt.Errorf("invalid source publication chronology")
		}
	case docsProcessPublicationFormat:
		if role != "docs" {
			return fmt.Errorf("docs process publication format used for another role")
		}
		return finalProvenanceDocsProcessPublication(in, p, s, o)
	case "docs-main-push/1":
		if role != "docs" || s.PublishedRef != "refs/heads/main" || p.PushStreams == nil {
			return fmt.Errorf("docs publication format used for another source or ref")
		}
		push, err := in.object(p.Push)
		if err != nil {
			return err
		}
		ref, err := in.object(p.Ref)
		if err != nil {
			return err
		}
		commit, err := in.object(p.Commit)
		if err != nil {
			return err
		}
		code, ok := finalProvenanceNumber(push, "exitCode")
		attempt, aok := finalProvenanceNumber(push, "attempt")
		if !ok || code != 0 || !aok || attempt != 1 || !finalProvenanceBool(push, "force", false) || finalProvenanceString(push, "source") != s.SHA {
			return fmt.Errorf("docs original push did not succeed once without force")
		}
		tail, ok := finalProvenanceCommandTail(push["argv"], "push")
		if !ok || !reflect.DeepEqual(tail, []string{"push", "origin", s.SHA + ":" + s.PublishedRef}) {
			return fmt.Errorf("docs push scope mismatch")
		}
		if err = finalProvenanceStreamCheck(in, push, *p.PushStreams); err != nil {
			return err
		}
		object := finalProvenanceObject(ref, "object")
		tree := finalProvenanceObject(commit, "tree")
		if finalProvenanceString(ref, "ref") != s.PublishedRef || finalProvenanceString(ref, "url") != "https://api.github.com/repos/"+s.Repository+"/git/refs/heads/main" || finalProvenanceString(object, "sha") != s.SHA || finalProvenanceString(object, "type") != "commit" || finalProvenanceString(commit, "sha") != s.SHA || finalProvenanceString(tree, "sha") != s.Tree {
			return fmt.Errorf("docs original remote ref and commit do not bind the expected source")
		}
		if finalProvenanceString(o, "status") != "MAIN_FAST_FORWARD_CONFIRMED" || finalProvenanceString(o, "source") != s.SHA || finalProvenanceString(o, "tree") != s.Tree || !finalProvenanceBool(o, "sourceCleanAfter", true) || finalProvenanceString(o, "postRefSHA256") != in.Expected.Files[p.Ref].SHA256 || finalProvenanceString(o, "pushResultSHA256") != in.Expected.Files[p.Push].SHA256 {
			return fmt.Errorf("docs publication summary disagrees with original push/readback")
		}
	default:
		return fmt.Errorf("unreviewed publication-original format")
	}
	return nil
}

func finalProvenanceCurrentSource(e finalProvenanceExpected, repository, sha string) bool {
	for _, s := range e.Sources {
		if s.Repository == repository && s.SHA == sha {
			return true
		}
	}
	return false
}
func finalProvenanceValidateRunsAndArtifacts(in *finalProvenanceInputs) error {
	seenRuns := map[string]bool{}
	for id, pin := range in.Expected.Runs {
		identity := pin.Repository + ":" + strconv.FormatInt(pin.ID, 10) + ":" + strconv.Itoa(pin.Attempt)
		if seenRuns[identity] {
			return fmt.Errorf("duplicate workflow observation identity")
		}
		seenRuns[identity] = true
		o, err := in.object(pin.Original)
		if err != nil {
			return err
		}
		rid, ok := finalProvenanceNumber(o, "id")
		attempt, aok := finalProvenanceNumber(o, "run_attempt")
		wid, wok := finalProvenanceNumber(o, "workflow_id")
		if !ok || !aok || !wok || rid != pin.ID || pin.ID <= 0 || attempt != int64(pin.Attempt) || pin.Attempt <= 0 || wid != pin.WorkflowID || pin.WorkflowID <= 0 || !finalProvenanceHex(pin.SourceSHA, 40) || finalProvenanceString(o, "head_sha") != pin.SourceSHA || finalProvenanceString(finalProvenanceObject(o, "repository"), "full_name") != pin.Repository || finalProvenanceString(o, "path") != pin.WorkflowPath || !strings.HasPrefix(pin.WorkflowPath, ".github/workflows/") || finalProvenanceString(o, "event") != pin.Event || pin.Event == "" || finalProvenanceString(o, "status") != pin.Status || finalProvenanceString(o, "conclusion") != pin.Conclusion || finalProvenanceString(o, "html_url") != "https://github.com/"+pin.Repository+"/actions/runs/"+strconv.FormatInt(pin.ID, 10) {
			return fmt.Errorf("original workflow tuple differs from independent context: %s", id)
		}
		if pin.Historical == finalProvenanceCurrentSource(in.Expected, pin.Repository, pin.SourceSHA) {
			return fmt.Errorf("workflow historical/current source label is false")
		}
		if pin.Status == "completed" {
			switch pin.Conclusion {
			case "success", "failure", "cancelled", "timed_out", "action_required", "neutral", "skipped", "stale":
			default:
				return fmt.Errorf("completed workflow lacks its actual conclusion")
			}
		} else {
			if (pin.Status != "queued" && pin.Status != "in_progress" && pin.Status != "waiting" && pin.Status != "pending" && pin.Status != "requested") || pin.Conclusion != "" {
				return fmt.Errorf("invalid nonterminal workflow state")
			}
		}
	}
	publicCount := 0
	roles := map[string]bool{}
	seenArtifacts := map[string]bool{}
	for id, a := range in.Expected.Artifacts {
		pin, exists := in.Expected.Files[a.File]
		source, roleOK := in.Expected.Sources[a.Role]
		if !exists || pin.Bytes <= 0 || !roleOK || !finalProvenanceHex(a.SourceSHA, 40) || a.Name == "" || a.Historical == (a.SourceSHA == source.SHA) {
			return fmt.Errorf("artifact lacks exact role/source/bytes or mislabels historical scope: %s", id)
		}
		roles[a.Role] = true
		if a.Availability == "local" {
			if a.URL != "" || a.Run != "" || a.Metadata != "" || a.ID != 0 {
				return fmt.Errorf("a local artifact falsely claims remote publication")
			}
			continue
		}
		if a.Availability != "github" {
			return fmt.Errorf("unknown artifact availability")
		}
		publicCount++
		run, ok := in.Expected.Runs[a.Run]
		if !ok || run.SourceSHA != a.SourceSHA || run.Historical != a.Historical || run.Repository != source.Repository {
			return fmt.Errorf("artifact is detached from its original workflow source")
		}
		identity := run.Repository + ":" + strconv.FormatInt(a.ID, 10)
		if seenArtifacts[identity] {
			return fmt.Errorf("duplicate published artifact identity")
		}
		seenArtifacts[identity] = true
		o, err := in.object(a.Metadata)
		if err != nil {
			return err
		}
		var candidates []any
		if values, exists := o["artifacts"]; exists {
			var ok bool
			candidates, ok = values.([]any)
			if !ok {
				return fmt.Errorf("invalid original artifact list")
			}
		} else {
			candidates = []any{o}
		}
		var artifact map[string]any
		for _, raw := range candidates {
			x, ok := raw.(map[string]any)
			if !ok {
				return fmt.Errorf("malformed artifact metadata")
			}
			n, ok := finalProvenanceNumber(x, "id")
			if ok && n == a.ID {
				if artifact != nil {
					return fmt.Errorf("duplicate artifact identity")
				}
				artifact = x
			}
		}
		if artifact == nil || a.ID <= 0 || finalProvenanceString(artifact, "name") != a.Name || !finalProvenanceBool(artifact, "expired", false) {
			return fmt.Errorf("expected published artifact absent, expired or renamed")
		}
		n, ok := finalProvenanceNumber(artifact, "size_in_bytes")
		workflow := finalProvenanceObject(artifact, "workflow_run")
		rid, rok := finalProvenanceNumber(workflow, "id")
		if !ok || n != pin.Bytes || !rok || rid != run.ID || finalProvenanceString(workflow, "head_sha") != a.SourceSHA {
			return fmt.Errorf("artifact original size/run/source does not match exact bytes")
		}
		apiURL := "https://api.github.com/repos/" + run.Repository + "/actions/artifacts/" + strconv.FormatInt(a.ID, 10)
		if finalProvenanceString(artifact, "url") != apiURL || finalProvenanceString(artifact, "archive_download_url") != apiURL+"/zip" || a.URL != "https://github.com/"+run.Repository+"/actions/runs/"+strconv.FormatInt(run.ID, 10)+"/artifacts/"+strconv.FormatInt(a.ID, 10) {
			return fmt.Errorf("artifact link is not the authenticated run/artifact identity")
		}
		if digest := finalProvenanceString(artifact, "digest"); digest != "" && digest != "sha256:"+pin.SHA256 {
			return fmt.Errorf("artifact server digest contradicts original archive bytes")
		}
	}
	if publicCount == 0 || len(roles) != 3 {
		return fmt.Errorf("delivery must enumerate artifacts for each source role and actual published artifact links")
	}
	return nil
}

func finalProvenanceCanonicalProducerCommand(value any, reportPath string) bool {
	a, ok := finalProvenanceStrings(value)
	return ok && len(a) == 11 && filepath.Base(a[0]) == "timeout" && reflect.DeepEqual(a[1:4], []string{"--signal=TERM", "--kill-after=30s", "80m"}) && filepath.Base(a[4]) == "go" && reflect.DeepEqual(a[5:], []string{"run", "./tools/language-acceptance", "--phase", "local", "--out", reportPath})
}
func finalProvenanceDecodeArray(b []byte, out any) error {
	wrapped := append([]byte(`{"items":`), b...)
	wrapped = append(wrapped, '}')
	o, err := decodeNativeEncodingObject(wrapped)
	if err != nil {
		return err
	}
	if _, ok := o["items"].([]any); !ok {
		return fmt.Errorf("original must be an array")
	}
	return json.Unmarshal(b, out)
}
func finalProvenanceEqualStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
func finalProvenanceAuditStatus(status string) string {
	switch status {
	case "PASSED":
		return "PASS"
	case "FAILED":
		return "FAIL"
	case "PENDING_EXTERNAL", "AI_EVAL_NOT_RUN", "BLOCKED_EXTERNAL":
		return "BLOCKED_EXTERNAL"
	case "NOT_RUN":
		return "UNVERIFIED"
	}
	return ""
}

func finalProvenanceValidateLedger(in *finalProvenanceInputs, contracts *ContractsSet, started time.Time, result *finalProvenanceResult) error {
	l := in.Expected.Ledger
	source := in.Expected.Sources["implementation"].SHA
	if l == nil {
		return finalProvenancePending("current-source finalized 179 ledger unavailable")
	}
	if l.SourceSHA != source {
		return fmt.Errorf("configured finalized ledger is for a different implementation SHA; a historical ledger cannot substitute")
	}
	reportObject, err := in.object(l.Report)
	if err != nil {
		return err
	}
	auditObject, err := in.object(l.Audit)
	if err != nil {
		return err
	}
	process, err := in.object(l.Process)
	if err != nil {
		return err
	}
	after, err := in.object(l.SourceAfter)
	if err != nil {
		return err
	}
	var report AcceptanceReport
	if err = json.Unmarshal(in.Bytes[l.Report], &report); err != nil {
		return fmt.Errorf("invalid original acceptance report")
	}
	var audit struct {
		SchemaVersion   string              `json:"schema_version"`
		SourceSHA       string              `json:"source_sha"`
		AcceptanceCount int                 `json:"acceptance_count"`
		RepairCount     int                 `json:"repair_count"`
		RecoveryCount   int                 `json:"recovery_gate_count"`
		Rows            []ExecutionAuditRow `json:"rows"`
	}
	if err = json.Unmarshal(in.Bytes[l.Audit], &audit); err != nil {
		return fmt.Errorf("invalid original execution audit")
	}
	mapping, err := BuildRequirementsEvidenceMap(contracts)
	if err != nil {
		return err
	}
	if mapping.AcceptanceCount != 94 || mapping.RepairCount != 52 || mapping.GatesCount != 33 || mapping.TotalRequirements != 179 || report.TotalCases != 179 || len(report.Results) != 179 || len(audit.Rows) != 179 || audit.AcceptanceCount != 94 || audit.RepairCount != 52 || audit.RecoveryCount != 33 || audit.SourceSHA != source || audit.SchemaVersion != "1" {
		return fmt.Errorf("original ledger is not the complete exact-source 94/52/33 inventory")
	}
	if report.Phase != "local" || report.ClosureComplete || !finalProvenanceBool(reportObject, "closure_complete", false) || !finalProvenanceBool(auditObject, "closure_complete", false) {
		return fmt.Errorf("delivery requires the original local 179 producer ledger, without closure promotion")
	}
	if finalProvenanceString(process, "status") != "PRODUCER_TERMINAL" || finalProvenanceString(process, "sourceSha") != source || !finalProvenanceBool(process, "producerStarted", true) || !finalProvenanceBool(process, "sourcesUnchanged", true) || !finalProvenanceBool(process, "gnuTimeoutExit124", false) || !finalProvenanceBool(process, "formalAcceptanceGranted", false) || !finalProvenanceCanonicalProducerCommand(process["command"], in.Manifest.Files[l.Report]) {
		return fmt.Errorf("original process is not the completed unchanged-source canonical 179 command")
	}
	if signals, ok := process["hostSignalsObserved"].([]any); !ok || len(signals) != 0 {
		return fmt.Errorf("original producer was interrupted or lacks its terminal signal accounting")
	}
	budgets := finalProvenanceObject(process, "budgets")
	for k, want := range map[string]int64{"producerSeconds": 4800, "producerTerminationGraceSeconds": 30, "outerRunSeconds": 5340, "outerStopSeconds": 30, "outerStopReceiptSeconds": 30, "outerTotalSeconds": 5400} {
		got, ok := finalProvenanceNumber(budgets, k)
		if !ok || got != want {
			return fmt.Errorf("original producer envelope differs from the reviewed complete run")
		}
	}
	begin, e1 := finalProvenanceTime(finalProvenanceString(process, "producerStartedAt"))
	end, e2 := finalProvenanceTime(finalProvenanceString(process, "producerFinishedAt"))
	finish, e3 := finalProvenanceTime(finalProvenanceString(process, "finishedAt"))
	stamp, e4 := finalProvenanceTime(report.Timestamp)
	if e1 != nil || e2 != nil || e3 != nil || e4 != nil || end.Before(begin) || finish.Before(end) || finish.After(started) || stamp.Before(begin) || stamp.After(end) {
		return fmt.Errorf("ledger is unfinished, future-dated or not from the original completed process")
	}
	for role, key := range map[string]string{"implementation": "implementation", "compiler": "compilerSource", "docs": "documentation"} {
		o := finalProvenanceObject(after, key)
		s := in.Expected.Sources[role]
		if finalProvenanceString(o, "head") != s.SHA || finalProvenanceString(o, "tree") != s.Tree || finalProvenanceString(o, "branch") != strings.TrimPrefix(s.LocalRef, "refs/heads/") || finalProvenanceString(o, "statusSha256") != finalProvenanceDigest(nil) {
			return fmt.Errorf("original final source snapshot differs from delivered tuple")
		}
	}
	var records []EvidenceRecord
	if err = finalProvenanceDecodeArray(in.Bytes[l.Evidence], &records); err != nil {
		return fmt.Errorf("invalid original local-evidence record array")
	}
	recordsByID := map[string]EvidenceRecord{}
	for _, record := range records {
		if record.EvidenceID == "" || record.TestID == "" || recordsByID[record.EvidenceID].EvidenceID != "" || record.ImplementationSHA != source {
			return fmt.Errorf("duplicate or foreign-source original evidence record")
		}
		recordsByID[record.EvidenceID] = record
	}
	observations, err := in.object(l.Observations)
	if err != nil {
		return err
	}
	if finalProvenanceString(observations, "source_sha") != source {
		return fmt.Errorf("original observations have another source")
	}
	observationIDs := map[string]bool{}
	for _, raw := range finalProvenanceArray(observations, "observations") {
		o, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("invalid original smoke observation")
		}
		id := finalProvenanceString(o, "id")
		if id == "" || observationIDs[id] {
			return fmt.Errorf("ambiguous original smoke observation")
		}
		observationIDs[id] = true
	}
	auditByID := map[string]ExecutionAuditRow{}
	for _, row := range audit.Rows {
		if row.ID == "" || auditByID[row.ID].ID != "" {
			return fmt.Errorf("duplicate original audit requirement")
		}
		auditByID[row.ID] = row
	}
	counts := map[string]int{}
	seen := map[string]bool{}
	var ledger, limitations []finalProvenanceLedgerRow
	for _, row := range report.Results {
		declared, known := mapping.Mappings[row.ID]
		a, found := auditByID[row.ID]
		if !known || seen[row.ID] || !found || a.SourceSHA != source || row.Requirement != declared.Title || a.Requirement != declared.Title || !finalProvenanceEqualStrings(a.RequiredTiers, declared.RequiredTiers) || a.Status != finalProvenanceAuditStatus(row.Status) || a.Status == "" || a.Detail != row.Detail || !finalProvenanceEqualStrings(a.Tier, row.ExecutedLevels) || row.Passed != (row.Status == "PASSED") {
			return fmt.Errorf("original report/audit/contract disagreement at requirement %s", row.ID)
		}
		seen[row.ID] = true
		counts[row.Status]++
		for _, reference := range a.Evidence {
			parts := strings.Split(reference, "#")
			if len(parts) != 2 || parts[1] == "" {
				return fmt.Errorf("malformed original requirement evidence reference")
			}
			switch parts[0] {
			case "acceptance-local-evidence.json":
				record, ok := recordsByID[parts[1]]
				if !ok {
					return fmt.Errorf("original ledger contains a dangling evidence record")
				}
				joined := false
				for _, id := range record.Requirements {
					joined = joined || id == row.ID
				}
				if !joined {
					return fmt.Errorf("evidence record belongs to another requirement")
				}
			case "acceptance-local-observations.json":
				if parts[1] != row.ID || !observationIDs[parts[1]] {
					return fmt.Errorf("original ledger contains a dangling observation")
				}
			default:
				return fmt.Errorf("unrecognized original ledger reference type")
			}
		}
		projection := finalProvenanceLedgerRow{row.ID, source, row.Status, row.Detail, append([]string{}, a.Evidence...)}
		ledger = append(ledger, projection)
		if row.Status != "PASSED" {
			limitations = append(limitations, projection)
		}
	}
	for id := range mapping.Mappings {
		if !seen[id] {
			return fmt.Errorf("a required ledger ID is missing")
		}
	}
	passed, failed := counts["PASSED"], counts["FAILED"]
	skipped := 179 - passed - failed
	if report.PassedCases != passed || report.FailedCases != failed || report.SkippedCases != skipped {
		return fmt.Errorf("original report counters disagree with all 179 rows")
	}
	code, ok := finalProvenanceNumber(process, "producerExitCode")
	wanted := int64(0)
	if failed > 0 || len(report.ValidationErrors) > 0 {
		wanted = 1
	}
	if !ok || code != wanted {
		return fmt.Errorf("original exit code contradicts the retained report")
	}
	summary := finalProvenanceObject(process, "report")
	if finalProvenanceString(summary, "sha256") != in.Expected.Files[l.Report].SHA256 || !finalProvenanceBool(summary, "present", true) || !finalProvenanceBool(summary, "exact179Ids", true) || !finalProvenanceBool(summary, "declaredCountersMatch", true) || !finalProvenanceBool(summary, "originalClosureComplete", false) {
		return fmt.Errorf("terminal process did not retain this exact report")
	}
	for key, want := range map[string]int64{"bytes": in.Expected.Files[l.Report].Bytes, "rowCount": 179, "uniqueIds": 179, "originalPassedCases": int64(passed), "originalFailedCases": int64(failed), "originalSkippedCases": int64(skipped)} {
		n, ok := finalProvenanceNumber(summary, key)
		if !ok || n != want {
			return fmt.Errorf("terminal report accounting disagrees with the original ledger")
		}
	}
	countObject := finalProvenanceObject(summary, "statusCounts")
	if len(countObject) != len(counts) {
		return fmt.Errorf("terminal status inventory disagrees")
	}
	for status, n := range counts {
		v, ok := finalProvenanceNumber(countObject, status)
		if !ok || v != int64(n) {
			return fmt.Errorf("terminal status count contradicts original rows")
		}
	}
	ids, ok := finalProvenanceStrings(after["contractIds"])
	if !ok || len(ids) != 179 {
		return fmt.Errorf("source-after contract inventory is incomplete")
	}
	idSet := map[string]bool{}
	for _, id := range ids {
		if !seen[id] || idSet[id] {
			return fmt.Errorf("source-after contract inventory differs")
		}
		idSet[id] = true
	}
	sort.Slice(ledger, func(i, j int) bool { return ledger[i].ID < ledger[j].ID })
	sort.Slice(limitations, func(i, j int) bool { return limitations[i].ID < limitations[j].ID })
	if !reflect.DeepEqual(in.Manifest.Ledger, ledger) || !reflect.DeepEqual(in.Manifest.Limitations, limitations) || in.Manifest.ClosureComplete != report.ClosureComplete {
		return fmt.Errorf("delivery projection conceals or changes original requirements or limitations")
	}
	// Only paths already declared by the independently pinned original record
	// inventory may be followed, and only beneath separately approved evidence
	// roots. No directory crawler or source/privacy certification is performed.
	for _, root := range l.ArtifactRoots {
		if !filepath.IsAbs(root) || filepath.Clean(root) == string(filepath.Separator) {
			return fmt.Errorf("invalid independently approved ledger-artifact root")
		}
	}
	for _, record := range records {
		for _, artifact := range record.Artifacts {
			path := artifact.Path
			if !filepath.IsAbs(path) {
				path = filepath.Join(filepath.Dir(in.Manifest.Files[l.Report]), path)
			}
			allowed := false
			resolvedPath, pathErr := filepath.EvalSymlinks(path)
			if os.IsNotExist(pathErr) {
				return finalProvenancePending("an original ledger artifact is no longer retained")
			}
			for _, root := range l.ArtifactRoots {
				resolvedRoot, rootErr := filepath.EvalSymlinks(root)
				if pathErr == nil && rootErr == nil && finalProvenanceWithin(resolvedRoot, resolvedPath) {
					allowed = true
				}
			}
			if !allowed || !finalProvenanceHex(artifact.SHA256, 64) {
				return fmt.Errorf("original record artifact is outside the approved delivery boundary or unhashed")
			}
			info, err := os.Lstat(path)
			if os.IsNotExist(err) {
				return finalProvenancePending("an original ledger artifact is no longer retained")
			}
			if err != nil {
				return fmt.Errorf("original ledger artifact is unreadable")
			}
			if _, err = finalProvenanceRead(path, finalProvenanceFile{artifact.SHA256, info.Size()}, false); err != nil {
				return err
			}
		}
	}
	result.LedgerSourceSHA = source
	result.ReportSHA256 = in.Expected.Files[l.Report].SHA256
	result.AuditSHA256 = in.Expected.Files[l.Audit].SHA256
	result.ProcessSHA256 = in.Expected.Files[l.Process].SHA256
	result.OriginalProcessFinishedUTC = finalProvenanceString(process, "finishedAt")
	result.StatusCounts = counts
	result.Ledger = ledger
	result.Limitations = limitations
	result.OriginalClosureComplete = report.ClosureComplete
	return nil
}

func finalProvenanceEvaluate(config finalProvenanceConfig, implementationRoot, outputDirectory string, contracts *ContractsSet) (result finalProvenanceResult) {
	started := time.Now().UTC()
	result = finalProvenanceResult{Schema: "cherri-final-provenance-result/1", StartedUTC: started.Format(time.RFC3339Nano), Status: "FAILED", ExpectedSHA256: config.ExpectedSHA256, AuthenticationBoundary: finalProvenanceTrustBoundary, Scope: finalProvenanceScope}
	var failure error
	defer func() {
		result.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
		if failure != nil {
			result.Detail = failure.Error()
			var pending finalProvenancePending
			if errors.As(failure, &pending) {
				result.Status = "NOT_RUN"
			}
		}
	}()
	in, err := finalProvenanceLoad(config, outputDirectory)
	if in != nil {
		result.SourceSHA = in.Expected.Sources["implementation"].SHA
		result.ManifestSHA256 = in.Expected.ManifestSHA256
		if in.Expected.Ledger != nil {
			result.LedgerSourceSHA = in.Expected.Ledger.SourceSHA
		}
	}
	if err != nil {
		failure = err
		return result
	}
	actualRoot, e1 := filepath.EvalSymlinks(implementationRoot)
	suppliedRoot, e2 := filepath.EvalSymlinks(in.Manifest.SourceDirectories["implementation"])
	if e1 != nil || e2 != nil || actualRoot != suppliedRoot {
		failure = fmt.Errorf("delivery implementation checkout is not the executing source checkout")
		return result
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result.Before = map[string]finalProvenanceSnapshot{}
	result.After = map[string]finalProvenanceSnapshot{}
	for _, role := range []string{"implementation", "compiler", "docs"} {
		s, e := finalProvenanceSnapshotSource(ctx, in.Manifest.SourceDirectories[role], in.Expected.Sources[role])
		result.Before[role] = s
		if e != nil {
			failure = e
			return result
		}
	}
	defer func() {
		for _, role := range []string{"implementation", "compiler", "docs"} {
			s, e := finalProvenanceSnapshotSource(ctx, in.Manifest.SourceDirectories[role], in.Expected.Sources[role])
			result.After[role] = s
			if e != nil {
				failure = e
				result.Status = "FAILED"
			}
			before := result.Before[role]
			if before.SHA != s.SHA || before.Tree != s.Tree || before.Ref != s.Ref || before.Origin != s.Origin || !s.Clean {
				failure = fmt.Errorf("delivered source changed during provenance intake")
				result.Status = "FAILED"
			}
		}
	}()
	for _, role := range []string{"implementation", "compiler", "docs"} {
		if err = finalProvenanceValidatePublication(in, role); err != nil {
			failure = err
			return result
		}
	}
	if err = finalProvenanceValidateRunsAndArtifacts(in); err != nil {
		failure = err
		return result
	}
	if err = finalProvenanceValidateLedger(in, contracts, started, &result); err != nil {
		failure = err
		return result
	}
	result.Runs = in.Expected.Runs
	result.Artifacts = in.Expected.Artifacts
	result.AdditionalLimitations = append([]string{}, in.Expected.AdditionalLimitations...)
	result.Status = "PASSED"
	result.Detail = "Exact current-source delivery verified against independent publication/run/artifact originals and the completed 179 ledger; original failed/pending rows retained without acceptance or closure promotion"
	return result
}

func finalProvenanceWriteNew(path string, value any) error {
	b, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("delivery output already exists or cannot be created")
	}
	_, writeErr := f.Write(b)
	closeErr := f.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}
func finalProvenanceOutputGuard(config finalProvenanceConfig, outputDirectory string) error {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return err
	}
	resolvedOutput, err := filepath.EvalSymlinks(outputDirectory)
	if err != nil {
		return fmt.Errorf("delivery output parent is unavailable")
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil || finalProvenanceWithin(resolvedRoot, resolvedOutput) {
		return fmt.Errorf("delivery output must be outside the executing source checkout")
	}
	var e finalProvenanceExpected
	var m finalProvenanceManifest
	if err = finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &e); err != nil {
		return err
	}
	if err = finalProvenanceReadContext(config.ManifestPath, e.ManifestSHA256, &m); err != nil {
		return err
	}
	for _, sourceDirectory := range m.SourceDirectories {
		resolvedSource, e := filepath.EvalSymlinks(sourceDirectory)
		if e == nil && finalProvenanceWithin(resolvedSource, resolvedOutput) {
			return fmt.Errorf("delivery output must be outside every delivered source checkout")
		}
	}
	return nil
}
func finalProvenanceAssertions(result finalProvenanceResult) map[string]string {
	return map[string]string{"delivery_source_sha": result.SourceSHA, "ledger_source_sha": result.LedgerSourceSHA, "expected_sha256": result.ExpectedSHA256, "manifest_sha256": result.ManifestSHA256, "report_sha256": result.ReportSHA256, "audit_sha256": result.AuditSHA256, "process_sha256": result.ProcessSHA256, "same_source_finalized_ledger": "true", "all_179_original_results_preserved": "true", "formal_acceptance_granted": "false", "ci_checkout_provenance_granted": "false"}
}
func finalProvenanceRecord(result finalProvenanceResult, config finalProvenanceConfig, path string) (EvidenceRecord, error) {
	record := EvidenceRecord{EvidenceID: "ev-final-provenance-END01", Requirements: []string{"END01"}, Tier: "repository", ImplementationSHA: result.SourceSHA, TestID: finalProvenanceTestID, Command: []string{finalProvenanceTestID}, Result: "passed", Detail: result.Detail}
	if result.Status != "PASSED" {
		record.Result = "rejected"
		record.ExitCode = -1
		if result.Status == "NOT_RUN" {
			record.Result = "not_run"
		}
		record.Assertions = map[string]string{"delivery_integrity_verified": "false", "formal_acceptance_granted": "false"}
	} else {
		record.Assertions = finalProvenanceAssertions(result)
	}
	for _, artifactPath := range []string{path, config.ExpectedPath, config.ManifestPath} {
		b, err := os.ReadFile(artifactPath)
		if err != nil {
			if result.Status != "PASSED" && artifactPath != path {
				continue
			}
			return record, fmt.Errorf("delivery result artifact cannot be retained")
		}
		record.Artifacts = append(record.Artifacts, EvidenceArtifact{Path: artifactPath, SHA256: finalProvenanceDigest(b)})
	}
	return record, nil
}
func finalProvenanceCase(result finalProvenanceResult) CaseResult {
	row := CaseResult{ID: "END01", Group: "delivery", Requirement: "Final pushed branches/artifacts/provenance and clean worktree", TestLevel: "repository", Status: result.Status, Passed: result.Status == "PASSED", Detail: result.Detail}
	if row.Passed {
		row.ExecutedLevels = []string{"repository"}
	}
	return row
}
func (r *Runner) runFinalProvenanceRequirement() {
	config, configured, err := finalProvenanceConfiguration()
	if !configured {
		r.results["END01"] = finalProvenanceCase(finalProvenanceResult{Status: "NOT_RUN", Detail: "No independently pinned final-delivery context and finalized same-source 179 ledger are configured; a historical report or successful git status cannot supply END01"})
		return
	}
	result := finalProvenanceResult{Schema: "cherri-final-provenance-result/1", Status: "FAILED", SourceSHA: r.implSHA, Scope: finalProvenanceScope, AuthenticationBoundary: finalProvenanceTrustBoundary}
	var directory string
	if err == nil {
		if r.artifactDir == "" {
			err = fmt.Errorf("configured final delivery requires an explicit external artifact directory")
		} else {
			err = finalProvenanceOutputGuard(config, r.artifactDir)
			if err == nil {
				directory, err = os.MkdirTemp(r.artifactDir, "final-provenance-")
			}
		}
	}
	if err == nil {
		var contracts *ContractsSet
		contracts, err = LoadAllContracts("", "", "")
		if err == nil {
			var root string
			root, err = filepath.Abs(resolveRepoRoot())
			if err == nil {
				result = finalProvenanceEvaluate(config, root, directory, contracts)
			}
		}
	}
	if err != nil {
		result.Detail = err.Error()
		var pending finalProvenancePending
		if errors.As(err, &pending) {
			result.Status = "NOT_RUN"
		}
	}
	if directory != "" {
		path := filepath.Join(directory, "final-provenance-result.json")
		if err = finalProvenanceWriteNew(path, result); err == nil {
			var record EvidenceRecord
			record, err = finalProvenanceRecord(result, config, path)
			if err == nil {
				r.records = append(r.records, record)
			}
		}
		if err != nil {
			result.Status = "FAILED"
			result.Detail = err.Error()
		}
	}
	r.finalDelivery = &result
	r.results["END01"] = finalProvenanceCase(result)
}

// The generic historical Record("END01", ...) guard remains active. Only the
// dedicated intake above creates a delivery result, and its audit is kept apart
// from both the old git-status smoke and the immutable input 179 report.
func applyFinalProvenanceAudit(row *ExecutionAuditRow, runner *Runner) {
	if row.ID != "END01" || runner.finalDelivery == nil {
		return
	}
	row.TestNames = []string{finalProvenanceTestID}
	row.ProductionPath = append([]string{}, finalProvenanceSources...)
	row.Scope = finalProvenanceScope
	row.PositiveAndNegative = false
	row.Blocker = ""
	if runner.finalDelivery.Status != "PASSED" {
		row.Blocker = runner.finalDelivery.Detail
	}
}

func validateFinalProvenanceRecord(id string, record EvidenceRecord) error {
	if id != "END01" {
		if record.TestID == finalProvenanceTestID {
			return fmt.Errorf("final delivery evidence cannot confer credit on another requirement")
		}
		return nil
	}
	if record.TestID != finalProvenanceTestID || record.EvidenceID != "ev-final-provenance-END01" || !reflect.DeepEqual(record.Requirements, []string{"END01"}) || record.Tier != "repository" || record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Command, []string{finalProvenanceTestID}) || len(record.Artifacts) != 3 {
		return fmt.Errorf("END01 requires an actual dedicated final-delivery intake record")
	}
	config, configured, err := finalProvenanceConfiguration()
	if err != nil || !configured {
		return fmt.Errorf("END01 requires independently configured delivery authority")
	}
	var resultPath string
	seenPaths := map[string]bool{}
	expectedSeen, manifestSeen := false, false
	for _, artifact := range record.Artifacts {
		if artifact.RunID != "" || !filepath.IsAbs(artifact.Path) || !finalProvenanceHex(artifact.SHA256, 64) || seenPaths[artifact.Path] {
			return fmt.Errorf("ambiguous final-delivery evidence artifact")
		}
		seenPaths[artifact.Path] = true
		i, err := os.Lstat(artifact.Path)
		if err != nil {
			return fmt.Errorf("missing final-delivery evidence original")
		}
		if _, err = finalProvenanceRead(artifact.Path, finalProvenanceFile{artifact.SHA256, i.Size()}, true); err != nil {
			return err
		}
		if artifact.Path == config.ExpectedPath {
			expectedSeen = true
			if artifact.SHA256 != config.ExpectedSHA256 {
				return fmt.Errorf("candidate evidence changed independent context digest")
			}
		} else if artifact.Path == config.ManifestPath {
			manifestSeen = true
		} else {
			if resultPath != "" {
				return fmt.Errorf("duplicate final-delivery result artifact")
			}
			resultPath = artifact.Path
		}
	}
	if resultPath == "" || !expectedSeen || !manifestSeen {
		return fmt.Errorf("final-delivery result or independent input artifact absent")
	}
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		return err
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return err
	}
	current := finalProvenanceEvaluate(config, root, filepath.Dir(resultPath), contracts)
	if current.Status != "PASSED" || record.ImplementationSHA != current.SourceSHA || !reflect.DeepEqual(record.Assertions, finalProvenanceAssertions(current)) {
		return fmt.Errorf("final-delivery originals or assertions do not pass the independent intake")
	}
	b, err := os.ReadFile(resultPath)
	if err != nil {
		return err
	}
	var original finalProvenanceResult
	if _, err = decodeNativeEncodingObject(b); err != nil {
		return err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err = d.Decode(&original); err != nil {
		return err
	}
	start, e1 := finalProvenanceTime(original.StartedUTC)
	finish, e2 := finalProvenanceTime(original.FinishedUTC)
	if e1 != nil || e2 != nil || finish.Before(start) || finish.After(time.Now().UTC()) {
		return fmt.Errorf("invalid final-delivery execution interval")
	}
	producerFinished, e3 := finalProvenanceTime(current.OriginalProcessFinishedUTC)
	if e3 != nil || start.Before(producerFinished) {
		return fmt.Errorf("original final-delivery intake predates completion of its original 179 producer")
	}
	current.StartedUTC, current.FinishedUTC = original.StartedUTC, original.FinishedUTC
	if !reflect.DeepEqual(original, current) {
		return fmt.Errorf("final-delivery receipt changed verified originals or source observations")
	}
	return nil
}

// Explicit delivery-only operation, after the same source's original 179 run is
// finalized. This does not run acceptance groups, replace their report, update
// their END01 row, or generate a final-acceptance manifest. Missing proof exits 2.
func finalProvenanceCommand(args []string) (bool, int) {
	handled := false
	for _, arg := range args {
		handled = handled || strings.HasPrefix(arg, "--verify-final-delivery")
	}
	if !handled {
		return false, 0
	}
	if len(args) != 3 || args[0] != "--verify-final-delivery" || args[1] != "--out" || !filepath.IsAbs(args[2]) {
		fmt.Fprintln(os.Stderr, "Use only --verify-final-delivery --out <new absolute JSON path>.")
		return true, 1
	}
	out := args[2]
	parent := filepath.Dir(out)
	evidencePath := filepath.Join(parent, "final-provenance-evidence.json")
	switch filepath.Base(out) {
	case "acceptance-report.json", "acceptance-results.json", "acceptance-execution-audit.json", "acceptance-local-evidence.json", "acceptance-local-observations.json", "local-runner-evidence.json", "requirements-evidence-map.json":
		fmt.Fprintln(os.Stderr, "Delivery-only output must not use an original 179 report or companion filename.")
		return true, 1
	}
	if out == evidencePath {
		fmt.Fprintln(os.Stderr, "Delivery result and record must be distinct.")
		return true, 1
	}
	for _, path := range []string{out, evidencePath} {
		if _, err := os.Lstat(path); !os.IsNotExist(err) {
			fmt.Fprintln(os.Stderr, "Delivery outputs must be fresh.")
			return true, 1
		}
	}
	info, err := os.Lstat(parent)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		fmt.Fprintln(os.Stderr, "Delivery output directory must already exist.")
		return true, 1
	}
	config, configured, err := finalProvenanceConfiguration()
	root, _ := filepath.Abs(resolveRepoRoot())
	resolvedRoot, _ := filepath.EvalSymlinks(root)
	resolvedParent, _ := filepath.EvalSymlinks(parent)
	if resolvedRoot != "" && finalProvenanceWithin(resolvedRoot, resolvedParent) {
		fmt.Fprintln(os.Stderr, "Delivery output must be outside the executing source checkout.")
		return true, 1
	}
	if err == nil && configured {
		guardErr := finalProvenanceOutputGuard(config, parent)
		var pending finalProvenancePending
		if guardErr != nil && !errors.As(guardErr, &pending) {
			fmt.Fprintln(os.Stderr, guardErr)
			return true, 1
		}
	}
	result := finalProvenanceResult{Schema: "cherri-final-provenance-result/1", Status: "NOT_RUN", Detail: "Independent delivery inputs are not configured", Scope: finalProvenanceScope, AuthenticationBoundary: finalProvenanceTrustBoundary}
	if err != nil {
		result.Status = "FAILED"
		result.Detail = err.Error()
	} else if configured {
		contracts, e := LoadAllContracts("", "", "")
		if e != nil {
			err = e
		} else {
			var root string
			root, err = filepath.Abs(resolveRepoRoot())
			if err == nil {
				result = finalProvenanceEvaluate(config, root, parent, contracts)
			}
		}
		if err != nil {
			result.Status = "FAILED"
			result.Detail = err.Error()
		}
	}
	if err = finalProvenanceWriteNew(out, result); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return true, 1
	}
	if configured {
		record, e := finalProvenanceRecord(result, config, out)
		if e != nil {
			fmt.Fprintln(os.Stderr, e)
			return true, 1
		}
		if e = finalProvenanceWriteNew(evidencePath, []EvidenceRecord{record}); e != nil {
			fmt.Fprintln(os.Stderr, e)
			return true, 1
		}
	}
	fmt.Printf("END01 %s; %s\n", result.Status, finalProvenanceScope)
	if result.Status == "PASSED" {
		return true, 0
	}
	if result.Status == "NOT_RUN" {
		return true, 2
	}
	return true, 1
}

// A reconciliation supplements the retained failed attempt. It never rewrites
// that result or represents the missing operations as original successful steps.
func finalProvenanceReconciledWindowsPublication(in *finalProvenanceInputs, p finalProvenancePublication, s finalProvenanceSource, original map[string]any) error {
	r := p.Reconciliation
	if r == nil || r.Result == p.Result || r.OriginalArchive == "" || r.Archive == "" || r.Archive == r.OriginalArchive {
		return fmt.Errorf("reconciled publication requires separate original and supplementary bindings")
	}
	for _, id := range []string{p.Result, r.OriginalArchive, r.Result, r.Archive} {
		pin, ok := in.Expected.Files[id]
		if !ok || pin.Bytes <= 0 || !finalProvenanceHex(pin.SHA256, 64) {
			return fmt.Errorf("reconciled publication is missing a pinned result or archive")
		}
	}
	supplement, err := in.object(r.Result)
	if err != nil {
		return err
	}
	const failedStatus = "PUSH_ATTEMPTED_REMOTE_OUTCOME_UNVERIFIED"
	if finalProvenanceString(original, "status") != failedStatus || finalProvenanceString(original, "remoteVerificationError") != "RuntimeError: 17-api-ref failed; exit=1 timeout=True" || original["remoteBinding"] != nil || !finalProvenanceBool(original, "pushAttempted", true) || !finalProvenanceBool(original, "pushTimedOut", false) || !finalProvenanceBool(original, "destinationMustBeAbsent", true) || !finalProvenanceBool(original, "primaryMutationAuthorized", false) || !finalProvenanceBool(original, "primaryUnchanged", true) || !finalProvenanceBool(original, "liveRemoteBuilderTouched", false) {
		return fmt.Errorf("reconciled publication does not retain the narrow original API-timeout failure")
	}
	for key, want := range map[string]int64{"pushAttemptLimit": 1, "pushExitCode": 0, "actionsDispatches": 0} {
		value, ok := finalProvenanceNumber(original, key)
		if !ok || value != want {
			return fmt.Errorf("reconciled publication did not make exactly one successful original push")
		}
	}
	if finalProvenanceString(supplement, "schema") != "cherri-windows-publication-readonly-reconciliation/1" || finalProvenanceString(supplement, "status") != "READ_ONLY_REMOTE_IDENTITY_VERIFIED" || supplement["error"] != nil {
		return fmt.Errorf("read-only reconciliation schema or terminal outcome differs")
	}
	for key, want := range map[string]bool{"pushPerformed": false, "importPerformed": false, "privatePayloadExported": false, "originalsUnchanged": true, "primaryUnchanged": true, "publicationOriginalStatusRewritten": false, "canonicalPublicationAcceptanceGranted": false} {
		if !finalProvenanceBool(supplement, key, want) {
			return fmt.Errorf("read-only reconciliation changed its operation or original-preservation scope")
		}
	}
	for key, want := range map[string]int64{"reconciliationAttemptLimit": 1, "actionsDispatches": 0} {
		value, ok := finalProvenanceNumber(supplement, key)
		if !ok || value != want {
			return fmt.Errorf("read-only reconciliation attempt or dispatch count differs")
		}
	}
	for _, o := range []map[string]any{original, supplement} {
		if finalProvenanceString(o, "sourceCommit") != s.SHA || finalProvenanceString(o, "sourceTree") != s.Tree || finalProvenanceString(o, "ref") != s.PublishedRef {
			return fmt.Errorf("reconciled publication source tuple differs from independent context")
		}
	}
	prior := finalProvenanceObject(supplement, "originalPublication")
	if finalProvenanceString(prior, "status") != failedStatus || finalProvenanceString(prior, "finishedAtUtc") != finalProvenanceString(original, "finishedAtUtc") {
		return fmt.Errorf("reconciliation does not bind the original status and finish time")
	}
	for key, id := range map[string]string{"result": p.Result, "archive": r.OriginalArchive} {
		binding := finalProvenanceObject(prior, key)
		pin := in.Expected.Files[id]
		n, ok := finalProvenanceNumber(binding, "bytes")
		if finalProvenanceString(binding, "sha256") != pin.SHA256 || !ok || n != pin.Bytes {
			return fmt.Errorf("reconciliation is bound to different original result or archive bytes")
		}
	}
	originalNames := []string{"01-origin", "02-base-refs", "03-primary-head-before", "04-primary-status-before", "05-transfer", "06-bundle-heads", "07-bundle-verify", "08-import", "09-commit", "10-tree", "11-ancestry", "12-connectivity", "13-imported-refs", "14-remote-before", "15-push", "16-remote-after", "17-api-ref", "19-primary-head-after", "20-primary-status-after"}
	readOnlyNames := []string{"01-origin", "02-primary-head-before", "03-primary-status-before", "04-remote-readback", "05-api-ref", "06-api-commit", "07-final-refs", "08-primary-head-after", "09-primary-status-after"}
	oldSteps, oldEnd, err := finalProvenanceReconciledSteps(in, original, originalNames, p.Streams, r.OriginalReceipts, time.Time{}, true)
	if err != nil {
		return err
	}
	newSteps, _, err := finalProvenanceReconciledSteps(in, supplement, readOnlyNames, r.Streams, r.Receipts, oldEnd, false)
	if err != nil {
		return err
	}
	origin := strings.TrimSpace(string(in.Bytes[p.Streams["01-origin"].Stdout]))
	if (origin != "https://github.com/"+s.Repository+".git" && origin != "https://github.com/"+s.Repository) || strings.TrimSpace(string(in.Bytes[r.Streams["01-origin"].Stdout])) != origin || finalProvenanceString(supplement, "origin") != origin {
		return fmt.Errorf("reconciled publication origin differs from original streams")
	}
	gitCommand, ok := finalProvenanceStrings(oldSteps["01-origin"]["argv"])
	if !ok || len(gitCommand) != 6 || gitCommand[1] != "-C" || gitCommand[2] == "" || gitCommand[2] != finalProvenanceString(original, "repo") || !strings.HasSuffix(strings.ReplaceAll(strings.ToLower(gitCommand[0]), `\`, "/"), "/git.exe") {
		return fmt.Errorf("original publication Git target is malformed")
	}
	gitPrefix := gitCommand[:3]
	primaryCommand, ok := finalProvenanceStrings(oldSteps["03-primary-head-before"]["argv"])
	if !ok || len(primaryCommand) != 5 || primaryCommand[0] != gitPrefix[0] || primaryCommand[1] != "-C" || primaryCommand[2] == "" || primaryCommand[2] == gitPrefix[2] {
		return fmt.Errorf("original primary checkout command target is malformed")
	}
	primaryPrefix := primaryCommand[:3]
	apiCommand, ok := finalProvenanceStrings(oldSteps["17-api-ref"]["argv"])
	if !ok || len(apiCommand) != 7 || !strings.HasSuffix(strings.ReplaceAll(strings.ToLower(apiCommand[0]), `\`, "/"), "/gh.exe") {
		return fmt.Errorf("original publication API command is malformed")
	}
	apiRef := []string{"api", "-H", "Cache-Control: no-cache", "repos/" + s.Repository + "/git/ref/" + strings.TrimPrefix(s.PublishedRef, "refs/"), "--jq", "{ref: .ref, sha: .object.sha, type: .object.type}"}
	apiCommit := []string{"api", "-H", "Cache-Control: no-cache", "repos/" + s.Repository + "/git/commits/" + s.SHA, "--jq", "{sha: .sha, tree: .tree.sha, parents: [.parents[].sha]}"}
	refsCommand := []string{"for-each-ref", "--format=%(refname) %(objectname)"}
	remoteCommand := []string{"ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef}
	validation := finalProvenanceObject(original, "validation")
	prerequisites, ok := finalProvenanceStrings(original["prerequisites"])
	if !ok || len(prerequisites) != 1 || !finalProvenanceHex(prerequisites[0], 40) || !reflect.DeepEqual(validation["prerequisites"], original["prerequisites"]) || finalProvenanceString(validation, "importedCommit") != s.SHA || finalProvenanceString(validation, "importedTree") != s.Tree || !finalProvenanceBool(validation, "remoteRefAbsent", true) {
		return fmt.Errorf("original import validation does not bind the source and prerequisite")
	}
	bundleCommand, ok := finalProvenanceStrings(oldSteps["06-bundle-heads"]["argv"])
	if !ok || len(bundleCommand) != 6 || bundleCommand[5] == "" {
		return fmt.Errorf("original publication bundle command is malformed")
	}
	bundle := bundleCommand[5]
	transfer, ok := finalProvenanceStrings(oldSteps["05-transfer"]["argv"])
	normalBundle := strings.ReplaceAll(bundle, `\`, "/")
	slash := strings.LastIndex(normalBundle, "/")
	transferDirectory := strings.ReplaceAll(finalProvenanceString(oldSteps["05-transfer"], "cwd"), `\`, "/")
	if !ok || len(transfer) != 9 || !strings.HasSuffix(strings.ReplaceAll(strings.ToLower(transfer[0]), `\`, "/"), "/scp.exe") || !reflect.DeepEqual(transfer[1:6], []string{"-B", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12"}) || slash < 1 || transferDirectory != normalBundle[:slash] || !strings.HasPrefix(transfer[6], "torah-dev:/") || !strings.HasSuffix(transfer[6], "/"+normalBundle[slash+1:]) || !strings.HasPrefix(transfer[7], "torah-dev:/") || !strings.HasSuffix(transfer[7], "/bundle-handoff.json") || transfer[8] != "." {
		return fmt.Errorf("original transfer differs from the bounded two-public-file copy")
	}
	commands := map[string][]string{
		"01-origin": {"remote", "get-url", "origin"}, "02-base-refs": refsCommand,
		"06-bundle-heads": {"bundle", "list-heads", bundle}, "07-bundle-verify": {"bundle", "verify", bundle},
		"08-import": {"fetch", "--no-tags", "--no-recurse-submodules", bundle, s.PublishedRef + ":" + s.PublishedRef},
		"09-commit": {"rev-parse", s.PublishedRef}, "10-tree": {"rev-parse", s.PublishedRef + "^{tree}"},
		"11-ancestry":     {"merge-base", "--is-ancestor", prerequisites[0], s.SHA},
		"12-connectivity": {"fsck", "--connectivity-only", "--no-reflogs", s.PublishedRef}, "13-imported-refs": refsCommand,
		"14-remote-before": remoteCommand, "15-push": {"push", "--porcelain", "--force-with-lease=" + s.PublishedRef + ":", "origin", s.SHA + ":" + s.PublishedRef}, "16-remote-after": remoteCommand,
	}
	for name, tail := range commands {
		if !finalProvenanceReconciledCommand(oldSteps[name], gitPrefix, tail) {
			return fmt.Errorf("original publication command differs from the single-source protocol: %s", name)
		}
	}
	if !finalProvenanceReconciledCommand(oldSteps["17-api-ref"], apiCommand[:1], apiRef) {
		return fmt.Errorf("original timed-out API command targets another source")
	}
	readback := s.SHA + "\t" + s.PublishedRef
	for name, want := range map[string]string{"06-bundle-heads": s.SHA + " " + s.PublishedRef, "09-commit": s.SHA, "10-tree": s.Tree, "14-remote-before": "", "16-remote-after": readback} {
		if strings.TrimSpace(string(in.Bytes[p.Streams[name].Stdout])) != want {
			return fmt.Errorf("original publication source, absence, or readback stream differs: %s", name)
		}
	}
	if !strings.Contains(string(in.Bytes[p.Streams["15-push"].Stdout]), "*\t"+s.SHA+":"+s.PublishedRef+"\t[new branch]") {
		return fmt.Errorf("original push stream does not report the sole new branch")
	}
	for _, o := range []map[string]any{original, supplement} {
		read := finalProvenanceObject(o, "remoteRefReadback")
		code, ok := finalProvenanceNumber(read, "code")
		if !ok || code != 0 || !finalProvenanceBool(read, "timedOut", false) || finalProvenanceString(read, "stdout") != readback {
			return fmt.Errorf("reconciled publication changed the original Git readback")
		}
	}
	allowlist := finalProvenanceArray(supplement, "readOnlyAllowlist")
	if len(allowlist) != len(readOnlyNames) {
		return fmt.Errorf("read-only reconciliation allowlist is incomplete")
	}
	for i, name := range readOnlyNames {
		allowed, ok := allowlist[i].(map[string]any)
		timeout, timeoutOK := finalProvenanceNumber(allowed, "timeoutSeconds")
		if !ok || len(allowed) != 3 || finalProvenanceString(allowed, "name") != name || !reflect.DeepEqual(allowed["argv"], newSteps[name]["argv"]) || !timeoutOK || timeout != 30 {
			return fmt.Errorf("read-only reconciliation changed its allowlisted commands or limits")
		}
	}
	for name, tail := range map[string][]string{"01-origin": {"remote", "get-url", "origin"}, "04-remote-readback": remoteCommand, "07-final-refs": refsCommand} {
		if !finalProvenanceReconciledCommand(newSteps[name], gitPrefix, tail) {
			return fmt.Errorf("read-only reconciliation Git command differs: %s", name)
		}
	}
	for name, tail := range map[string][]string{"05-api-ref": apiRef, "06-api-commit": apiCommit} {
		if !finalProvenanceReconciledCommand(newSteps[name], apiCommand[:1], tail) {
			return fmt.Errorf("read-only reconciliation API command is not the exact GET: %s", name)
		}
	}
	if strings.TrimSpace(string(in.Bytes[r.Streams["04-remote-readback"].Stdout])) != readback {
		return fmt.Errorf("read-only reconciliation Git stream identifies another source")
	}
	ref, err := in.object(r.Streams["05-api-ref"].Stdout)
	if err != nil {
		return err
	}
	commit, err := in.object(r.Streams["06-api-commit"].Stdout)
	if err != nil {
		return err
	}
	if finalProvenanceString(ref, "ref") != s.PublishedRef || finalProvenanceString(ref, "sha") != s.SHA || finalProvenanceString(ref, "type") != "commit" || finalProvenanceString(commit, "sha") != s.SHA || finalProvenanceString(commit, "tree") != s.Tree {
		return fmt.Errorf("reconciliation API originals do not bind the expected source and tree")
	}
	remote := finalProvenanceObject(supplement, "remoteBinding")
	if !reflect.DeepEqual(finalProvenanceObject(remote, "ref"), ref) || !reflect.DeepEqual(finalProvenanceObject(remote, "commit"), commit) {
		return fmt.Errorf("reconciliation result changed the raw API binding")
	}
	verified, e1 := finalProvenanceTime(finalProvenanceString(remote, "verifiedAtUtc"))
	lastStep, e2 := finalProvenanceTime(finalProvenanceString(newSteps["09-primary-status-after"], "finishedAtUtc"))
	finished, e3 := finalProvenanceTime(finalProvenanceString(supplement, "finishedAtUtc"))
	if e1 != nil || e2 != nil || e3 != nil || verified.Before(lastStep) || verified.After(finished) {
		return fmt.Errorf("reconciliation verification time is outside its completed observations")
	}
	primaryBefore := finalProvenanceObject(validation, "primaryBefore")
	for _, primary := range []map[string]any{primaryBefore, finalProvenanceObject(original, "primaryAfter"), finalProvenanceObject(supplement, "primaryBefore"), finalProvenanceObject(supplement, "primaryAfter")} {
		_, statusOK := primary["status"].(string)
		if len(primary) != 3 || !finalProvenanceHex(finalProvenanceString(primary, "head"), 40) || !statusOK || !finalProvenanceHex(finalProvenanceString(primary, "preservedFixtureSHA256"), 64) || !reflect.DeepEqual(primaryBefore, primary) {
			return fmt.Errorf("reconciled publication changed or omitted the preserved primary checkout")
		}
	}
	for _, phase := range []struct {
		steps   map[string]map[string]any
		streams map[string]finalProvenanceStreams
		names   []string
	}{{oldSteps, p.Streams, []string{"03-primary-head-before", "04-primary-status-before", "19-primary-head-after", "20-primary-status-after"}}, {newSteps, r.Streams, []string{"02-primary-head-before", "03-primary-status-before", "08-primary-head-after", "09-primary-status-after"}}} {
		for i, name := range phase.names {
			field, tail := "head", []string{"rev-parse", "HEAD"}
			if i%2 == 1 {
				field, tail = "status", []string{"status", "--porcelain=v1"}
			}
			if !finalProvenanceReconciledCommand(phase.steps[name], primaryPrefix, tail) || strings.TrimSpace(string(in.Bytes[phase.streams[name].Stdout])) != primaryBefore[field] {
				return fmt.Errorf("reconciled primary checkout commands or original streams differ")
			}
		}
	}
	base, err := finalProvenanceReconciledRefs(in.Bytes[p.Streams["02-base-refs"].Stdout])
	if err != nil {
		return err
	}
	imported, err := finalProvenanceReconciledRefs(in.Bytes[p.Streams["13-imported-refs"].Stdout])
	if err != nil {
		return err
	}
	final, err := finalProvenanceReconciledRefs(in.Bytes[r.Streams["07-final-refs"].Stdout])
	if err != nil {
		return err
	}
	originRef := "refs/remotes/origin/" + strings.TrimPrefix(s.PublishedRef, "refs/heads/")
	if _, exists := base[s.PublishedRef]; exists {
		return fmt.Errorf("publication local source ref was not absent before import")
	}
	if _, exists := base[originRef]; exists {
		return fmt.Errorf("publication tracking ref existed before the sole push")
	}
	base[s.PublishedRef] = s.SHA
	if !reflect.DeepEqual(base, imported) {
		return fmt.Errorf("original import changed refs beyond the source ref")
	}
	base[originRef] = s.SHA
	if !reflect.DeepEqual(base, final) {
		return fmt.Errorf("reconciliation refs differ beyond the sole source tracking ref")
	}
	for _, item := range []struct {
		value any
		refs  map[string]string
	}{{validation["localRefs"], imported}, {supplement["finalRefs"], final}} {
		lines, ok := finalProvenanceStrings(item.value)
		if !ok {
			return fmt.Errorf("reconciled ref inventory is not an original string array")
		}
		parsed, err := finalProvenanceReconciledRefs([]byte(strings.Join(lines, "\n")))
		if err != nil || !reflect.DeepEqual(parsed, item.refs) {
			return fmt.Errorf("reconciled ref inventory disagrees with original streams")
		}
	}
	return finalProvenanceReconciledArchives(in, p)
}

func finalProvenanceReconciledCommand(step map[string]any, prefix, tail []string) bool {
	argv, ok := finalProvenanceStrings(step["argv"])
	want := append(append([]string{}, prefix...), tail...)
	cwd, present := step["cwd"]
	return ok && reflect.DeepEqual(argv, want) && present && cwd == nil
}

func finalProvenanceReconciledRefs(b []byte) (map[string]string, error) {
	refs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || !strings.HasPrefix(fields[0], "refs/") || !finalProvenanceHex(fields[1], 40) || refs[fields[0]] != "" {
			return nil, fmt.Errorf("malformed or duplicate original ref inventory")
		}
		refs[fields[0]] = fields[1]
	}
	return refs, nil
}

func finalProvenanceReconciledSteps(in *finalProvenanceInputs, o map[string]any, names []string, streams map[string]finalProvenanceStreams, receipts map[string]string, after time.Time, original bool) (map[string]map[string]any, time.Time, error) {
	fail := func(detail string) (map[string]map[string]any, time.Time, error) {
		return nil, time.Time{}, fmt.Errorf("reconciled publication %s", detail)
	}
	start, e1 := finalProvenanceTime(finalProvenanceString(o, "startedAtUtc"))
	end, e2 := finalProvenanceTime(finalProvenanceString(o, "finishedAtUtc"))
	if e1 != nil || e2 != nil || end.Before(start) || (!after.IsZero() && start.Before(after)) {
		return fail("operation chronology is inconsistent")
	}
	steps := finalProvenanceArray(o, "steps")
	if len(steps) != len(names) || len(streams) != len(names) || len(receipts) != len(names) {
		return fail("original step, receipt, or stream inventory is incomplete")
	}
	byName := map[string]map[string]any{}
	seen := map[string]bool{}
	previous := start
	for i, name := range names {
		step, ok := steps[i].(map[string]any)
		if !ok || finalProvenanceString(step, "name") != name {
			return fail("original step identity or order differs")
		}
		pair, present := streams[name]
		if !present {
			return fail("original stream pair is absent")
		}
		for _, id := range []string{pair.Stdout, pair.Stderr, receipts[name]} {
			if id == "" || seen[id] {
				return fail("original object identity is absent or replayed")
			}
			seen[id] = true
		}
		if err := finalProvenanceStreamCheck(in, step, pair); err != nil {
			return nil, time.Time{}, err
		}
		for field, id := range map[string]string{"stdout": pair.Stdout, "stderr": pair.Stderr} {
			n, ok := finalProvenanceNumber(step, field+"Bytes")
			if !ok || n != in.Expected.Files[id].Bytes {
				return fail("original stream byte count is missing or different")
			}
		}
		receipt, err := in.object(receipts[name])
		if err != nil {
			return nil, time.Time{}, err
		}
		if !reflect.DeepEqual(receipt, step) {
			return fail("result step differs from the original process receipt")
		}
		timeout := int64(30)
		if original {
			switch name {
			case "05-transfer", "15-push":
				timeout = 120
			case "08-import", "12-connectivity":
				timeout = 60
			}
		}
		limit, limitOK := finalProvenanceNumber(step, "timeoutSeconds")
		code, codeOK := finalProvenanceNumber(step, "exitCode")
		wantCode := int64(0)
		if original && name == "14-remote-before" {
			wantCode = 2
		}
		timedOut := original && name == "17-api-ref"
		if timedOut {
			wantCode = 1
		}
		if !limitOK || limit != timeout || !codeOK || code != wantCode || !finalProvenanceBool(step, "timedOut", timedOut) {
			return fail("process result or original timeout budget differs")
		}
		kill, killPresent := step["ownedTreeKillExit"]
		if timedOut {
			killCode, ok := finalProvenanceNumber(step, "ownedTreeKillExit")
			if !ok || killCode != 0 || in.Expected.Files[pair.Stdout].Bytes != 0 || in.Expected.Files[pair.Stderr].Bytes != 0 {
				return fail("original API timeout is not the empty-stream cleaned-up failure")
			}
		} else if !killPresent || kill != nil {
			return fail("successful process unexpectedly required tree cleanup")
		}
		began, e1 := finalProvenanceTime(finalProvenanceString(step, "startedAtUtc"))
		ended, e2 := finalProvenanceTime(finalProvenanceString(step, "finishedAtUtc"))
		if e1 != nil || e2 != nil || began.Before(previous) || ended.Before(began) || ended.After(end) || ended.Sub(began) > time.Duration(timeout+25)*time.Second || (timedOut && ended.Sub(began) < time.Duration(timeout)*time.Second) {
			return fail("process chronology is inconsistent with its original limits")
		}
		previous, byName[name] = ended, step
	}
	return byName, end, nil
}

// Reuse the canonical bounded ZIP reader, then join every accepted process
// observation to the independent originals; an archive pin alone is insufficient.
func finalProvenanceReconciledArchives(in *finalProvenanceInputs, p finalProvenancePublication) error {
	r := p.Reconciliation
	budget := &ciExpandedBudget{Limit: 4 * 1024 * 1024}
	for _, phase := range []struct {
		archive, result, resultMember string
		inventory                     map[string]ciArchiveMember
		streams                       map[string]finalProvenanceStreams
		receipts                      map[string]string
	}{
		{r.OriginalArchive, p.Result, "publication.result.json", r.OriginalMembers, p.Streams, r.OriginalReceipts},
		{r.Archive, r.Result, "reconciliation.result.json", r.Members, r.Streams, r.Receipts},
	} {
		data, ok := in.Bytes[phase.archive]
		if !ok || len(data) == 0 || len(data) > 4*1024*1024 {
			return fmt.Errorf("reconciled publication archive is absent or exceeds its retained-byte limit")
		}
		members, err := ciIdentityArchive(data, phase.inventory, budget)
		if err != nil {
			return fmt.Errorf("reconciled publication archive: %w", err)
		}
		bindings := map[string]string{phase.resultMember: phase.result}
		for name, streams := range phase.streams {
			bindings[name+".stdout.original"] = streams.Stdout
			bindings[name+".stderr.original"] = streams.Stderr
			bindings[name+".receipt.json"] = phase.receipts[name]
		}
		for name, id := range bindings {
			archived, found := members[name]
			original, retained := in.Bytes[id]
			if !found || !retained || !bytes.Equal(archived, original) {
				return fmt.Errorf("reconciled publication archive member differs from independent original: %s", name)
			}
		}
	}
	return nil
}

// Oracle publication has its own actual process-receipt schema. The Windows
// originals and the older nonforce publication contract remain separate.
func finalProvenanceOraclePublication(in *finalProvenanceInputs, p finalProvenancePublication, s finalProvenanceSource, o map[string]any) error {
	fail := func(detail string) error { return fmt.Errorf("Oracle atomic publication %s", detail) }
	source := finalProvenanceString(o, "sourceDirectory")
	if finalProvenanceString(o, "schema") != "cherri-oracle-atomic-create-only-publication/1" || finalProvenanceString(o, "status") != "PUBLISHED_REMOTE_VERIFIED" || finalProvenanceString(o, "sourceCommit") != s.SHA || finalProvenanceString(o, "sourceTree") != s.Tree || finalProvenanceString(o, "localRef") != p.PublisherRef || finalProvenanceString(o, "ref") != s.PublishedRef || s.Repository != "davidpovarsky/cherri" || !strings.HasPrefix(source, "/mnt/development/projects/") || filepath.Clean(source) != source || !strings.HasPrefix(p.PublisherRef, "refs/heads/") || (!strings.HasPrefix(s.PublishedRef, "refs/heads/agent/") && !strings.HasPrefix(s.PublishedRef, "refs/heads/audit/")) {
		return fail("source, platform, scope or successful status differs")
	}
	for key, want := range map[string]bool{"destinationMustBeAbsent": true, "pushAttempted": true, "pushTimedOut": false, "sourceUnchanged": true, "sourceInputsUnchanged": true, "credentialPersisted": false, "privatePayloadExported": false, "liveRemoteBuilderTouched": false} {
		if !finalProvenanceBool(o, key, want) {
			return fail("scope flag differs: " + key)
		}
	}
	for key, want := range map[string]int64{"pushAttemptLimit": 1, "pushAttempts": 1, "pushExitCode": 0, "retries": 0, "actionsDispatches": 0} {
		if n, ok := finalProvenanceNumber(o, key); !ok || n != want {
			return fail("attempt or operation count differs: " + key)
		}
	}
	if finalProvenanceString(o, "credentialMode") != "command-scoped-gh-helper" || p.Reconciliation != nil {
		return fail("credential mode or publication binding differs")
	}
	start, e1 := finalProvenanceTime(finalProvenanceString(o, "startedAt"))
	end, e2 := finalProvenanceTime(finalProvenanceString(o, "finishedAt"))
	if e1 != nil || e2 != nil || end.Before(start) {
		return fail("operation chronology differs")
	}
	bounds := finalProvenanceObject(o, "bounds")
	if len(bounds) != 7 {
		return fail("selected outer bounds are incomplete")
	}
	for key, want := range map[string]int64{"readSeconds": 30, "pushSeconds": 120, "ownerAlarmSeconds": 690, "ownerBudgetSeconds": 700, "hostSeconds": 720, "termCleanupSeconds": 5, "killCleanupSeconds": 5} {
		if n, ok := finalProvenanceNumber(bounds, key); !ok || n != want {
			return fail("selected outer bounds differ")
		}
	}
	elapsedNumber, elapsedOK := o["elapsedSeconds"].(json.Number)
	elapsed, elapsedErr := elapsedNumber.Float64()
	outerSignals, signalsOK := finalProvenanceStrings(o["signals"])
	wall := end.Sub(start).Seconds()
	if !elapsedOK || elapsedErr != nil || elapsed < 0 || elapsed > 700 || wall > 700 || elapsed-wall < -1 || elapsed-wall > 1 || !signalsOK || len(outerSignals) != 0 {
		return fail("outer process outcome or deadline differs")
	}
	names, commands := finalProvenanceOracleCommands(source, s)
	steps := finalProvenanceArray(o, "processes")
	if len(steps) != len(names) || len(p.Streams) != len(names) || len(p.Receipts) != len(names) {
		return fail("process, receipt or stream inventory is incomplete")
	}
	byName := map[string]map[string]any{}
	seen := map[string]bool{p.Result: true}
	previous := start
	for i, name := range names {
		step, ok := steps[i].(map[string]any)
		if !ok || finalProvenanceString(step, "label") != name {
			return fail("process identity or order differs")
		}
		argv, ok := finalProvenanceStrings(step["argv"])
		if !ok || !reflect.DeepEqual(argv, commands[name]) || finalProvenanceString(step, "cwd") != source {
			return fail("exact command or source directory differs: " + name)
		}
		pair, present := p.Streams[name]
		if !present {
			return fail("stream pair is absent")
		}
		for _, id := range []string{pair.Stdout, pair.Stderr, p.Receipts[name]} {
			if id == "" || seen[id] {
				return fail("original identity is absent or replayed")
			}
			seen[id] = true
			if _, ok := in.Expected.Files[id]; !ok {
				return fail("original is not independently pinned")
			}
		}
		for field, id := range map[string]string{"stdout": pair.Stdout, "stderr": pair.Stderr} {
			observed := finalProvenanceObject(step, field)
			pin := in.Expected.Files[id]
			n, ok := finalProvenanceNumber(observed, "bytes")
			b, retained := in.Bytes[id]
			if len(observed) != 3 || !ok || n != pin.Bytes || finalProvenanceString(observed, "sha256") != pin.SHA256 || finalProvenanceString(observed, "path") != in.Manifest.Files[id] || !retained || int64(len(b)) != pin.Bytes || finalProvenanceDigest(b) != pin.SHA256 {
				return fail("stream does not join its independently pinned bytes")
			}
		}
		receipt, err := in.object(p.Receipts[name])
		if err != nil {
			return err
		}
		if !reflect.DeepEqual(receipt, step) {
			return fail("result process differs from its original receipt")
		}
		limit, wantCode := int64(30), int64(0)
		if name == "10-push" {
			limit = 120
		}
		if name == "09-destination-before" {
			wantCode = 2
		}
		n, limitOK := finalProvenanceNumber(step, "limitSeconds")
		code, codeOK := finalProvenanceNumber(step, "exitCode")
		pid, pidOK := finalProvenanceNumber(step, "pid")
		cleanup := finalProvenanceObject(step, "cleanup")
		signals, signalsOK := finalProvenanceStrings(step["signals"])
		cleanSignals, cleanOK := finalProvenanceStrings(cleanup["signals"])
		if !limitOK || n != limit || !codeOK || code != wantCode || !pidOK || pid <= 0 || !finalProvenanceBool(step, "timedOut", false) || !finalProvenanceBool(step, "overflow", false) || !signalsOK || len(signals) != 0 || !cleanOK || len(cleanSignals) != 0 || !finalProvenanceBool(cleanup, "groupGone", true) {
			return fail("process result, cleanup or original deadline differs")
		}
		began, e1 := finalProvenanceTime(finalProvenanceString(step, "startedAt"))
		ended, e2 := finalProvenanceTime(finalProvenanceString(step, "finishedAt"))
		if e1 != nil || e2 != nil || began.Before(previous) || ended.Before(began) || ended.After(end) || ended.Sub(began) > time.Duration(limit+25)*time.Second {
			return fail("process chronology differs")
		}
		previous, byName[name] = ended, step
	}
	out := func(name string) []byte { return in.Bytes[p.Streams[name].Stdout] }
	origin := "https://github.com/" + s.Repository + ".git"
	state := map[string]any{"head": s.SHA, "tree": s.Tree, "branch": strings.TrimPrefix(p.PublisherRef, "refs/heads/"), "status": "", "origin": origin, "pushOrigin": origin}
	if !reflect.DeepEqual(finalProvenanceObject(o, "sourceBefore"), state) || !reflect.DeepEqual(finalProvenanceObject(o, "sourceAfter"), state) {
		return fail("source snapshot changed or is incomplete")
	}
	for name, want := range map[string]string{"01-origin": origin, "02-push-origin": origin, "03-head-before": s.SHA, "04-tree-before": s.Tree, "05-branch-before": state["branch"].(string), "16-head-after": s.SHA, "17-tree-after": s.Tree, "18-branch-after": state["branch"].(string), "20-origin-after": origin, "21-push-origin-after": origin} {
		if string(out(name)) != want+"\n" {
			return fail("source snapshot does not join original streams")
		}
	}
	for _, name := range []string{"06-status-before", "19-status-after", "09-destination-before"} {
		if len(out(name)) != 0 || len(in.Bytes[p.Streams[name].Stderr]) != 0 {
			return fail("source cleanliness or destination absence differs")
		}
	}
	if string(out("11-destination-after")) != s.SHA+"\t"+s.PublishedRef+"\n" {
		return fail("destination readback differs")
	}
	if string(out("10-push")) != "To "+origin+"\n*\t"+s.SHA+":"+s.PublishedRef+"\t[new branch]\nDone\n" {
		return fail("push did not report exactly the selected new branch")
	}
	refs := map[string]map[string]string{}
	for name, remote := range map[string]bool{"07-local-refs-before": false, "08-remote-refs-before": true, "14-remote-refs-after": true, "15-local-refs-after": false} {
		observed, err := finalProvenanceOracleRefs(out(name), remote)
		if err != nil {
			return err
		}
		refs[name] = observed
	}
	localBefore, localAfter := refs["07-local-refs-before"], refs["15-local-refs-after"]
	tracking := "refs/remotes/origin/" + strings.TrimPrefix(s.PublishedRef, "refs/heads/")
	if localBefore[p.PublisherRef] != s.SHA || localBefore[tracking] != "" {
		return fail("local source or new tracking-ref precondition differs")
	}
	for ref := range localBefore {
		if strings.HasPrefix(ref, "refs/replace/") {
			return fail("local replacement ref is present")
		}
	}
	if localAfter[tracking] == s.SHA {
		delete(localAfter, tracking)
	}
	if !reflect.DeepEqual(localBefore, localAfter) {
		return fail("unrelated local refs changed")
	}
	remoteBefore, remoteAfter := refs["08-remote-refs-before"], refs["14-remote-refs-after"]
	if remoteBefore[s.PublishedRef] != "" {
		return fail("destination was present in the full remote inventory")
	}
	remoteBefore[s.PublishedRef] = s.SHA
	if !reflect.DeepEqual(remoteBefore, remoteAfter) {
		return fail("full remote inventory changed outside the selected new ref")
	}
	ref, err := in.object(p.Streams["12-api-ref"].Stdout)
	if err != nil {
		return err
	}
	commit, err := in.object(p.Streams["13-api-commit"].Stdout)
	if err != nil {
		return err
	}
	if len(ref) != 3 || finalProvenanceString(ref, "ref") != s.PublishedRef || finalProvenanceString(ref, "sha") != s.SHA || finalProvenanceString(ref, "type") != "commit" || finalProvenanceString(commit, "sha") != s.SHA || finalProvenanceString(commit, "tree") != s.Tree {
		return fail("independent API ref, commit or tree differs")
	}
	binding := finalProvenanceObject(o, "remoteBinding")
	verified, e1 := finalProvenanceTime(finalProvenanceString(binding, "verifiedAtUtc"))
	apiFinished, e2 := finalProvenanceTime(finalProvenanceString(byName["13-api-commit"], "finishedAt"))
	if len(binding) != 3 || !reflect.DeepEqual(finalProvenanceObject(binding, "ref"), ref) || !reflect.DeepEqual(finalProvenanceObject(binding, "commit"), commit) || e1 != nil || e2 != nil || verified.Before(apiFinished) || verified.After(end) {
		return fail("remote binding or verification time differs from originals")
	}
	return nil
}

func finalProvenanceOracleCommands(source string, s finalProvenanceSource) ([]string, map[string][]string) {
	names := []string{"01-origin", "02-push-origin", "03-head-before", "04-tree-before", "05-branch-before", "06-status-before", "07-local-refs-before", "08-remote-refs-before", "09-destination-before", "10-push", "11-destination-after", "12-api-ref", "13-api-commit", "14-remote-refs-after", "15-local-refs-after", "16-head-after", "17-tree-after", "18-branch-after", "19-status-after", "20-origin-after", "21-push-origin-after"}
	prefix := []string{"/usr/bin/git", "-C", source, "-c", "push.followTags=false", "-c", "push.recurseSubmodules=no", "-c", "remote.origin.mirror=false"}
	local := func(a ...string) []string { return append(append([]string{}, prefix...), a...) }
	remote := func(a ...string) []string {
		return append(local("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential"), a...)
	}
	api := func(endpoint, projection string) []string {
		return []string{"/usr/local/bin/gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", endpoint, "--jq", projection}
	}
	commands := map[string][]string{
		"01-origin": local("remote", "get-url", "--all", "origin"), "02-push-origin": local("remote", "get-url", "--push", "--all", "origin"),
		"03-head-before": local("rev-parse", "HEAD"), "04-tree-before": local("rev-parse", "HEAD^{tree}"), "05-branch-before": local("branch", "--show-current"), "06-status-before": local("status", "--porcelain=v1", "--untracked-files=all"),
		"07-local-refs-before": local("for-each-ref", "--format=%(refname) %(objectname)"), "08-remote-refs-before": remote("ls-remote", "--refs", "origin"), "09-destination-before": remote("ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef),
		"10-push": remote("push", "--porcelain", "--force-with-lease="+s.PublishedRef+":", "origin", s.SHA+":"+s.PublishedRef), "11-destination-after": remote("ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef),
		"12-api-ref": api("repos/"+s.Repository+"/git/ref/"+strings.TrimPrefix(s.PublishedRef, "refs/"), "{ref: .ref, sha: .object.sha, type: .object.type}"), "13-api-commit": api("repos/"+s.Repository+"/git/commits/"+s.SHA, "{sha: .sha, tree: .tree.sha, parents: [.parents[].sha]}"),
		"14-remote-refs-after": remote("ls-remote", "--refs", "origin"), "15-local-refs-after": local("for-each-ref", "--format=%(refname) %(objectname)"),
		"16-head-after": local("rev-parse", "HEAD"), "17-tree-after": local("rev-parse", "HEAD^{tree}"), "18-branch-after": local("branch", "--show-current"), "19-status-after": local("status", "--porcelain=v1", "--untracked-files=all"),
		"20-origin-after": local("remote", "get-url", "--all", "origin"), "21-push-origin-after": local("remote", "get-url", "--push", "--all", "origin"),
	}
	return names, commands
}

func finalProvenanceOracleRefs(b []byte, remote bool) (map[string]string, error) {
	if !remote {
		return finalProvenanceReconciledRefs(b)
	}
	var normalized []string
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("Oracle atomic publication malformed remote ref inventory")
		}
		normalized = append(normalized, fields[1]+" "+fields[0])
	}
	return finalProvenanceReconciledRefs([]byte(strings.Join(normalized, "\n")))
}
