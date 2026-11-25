package main

// CI03 is an identity join. It neither chooses a successful workflow attempt nor
// grants RP49/BRG31, native, signing, AI, or final-release acceptance.
// This proposed file depends on END01's ledger-independent bindings loader.

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const ciIdentitySchema = "cherri-ci03-exact-identity/1"
const ciIdentityTestID = "tools/language-acceptance:CI03ExactSourceIdentity"
const ciIdentityScope = "Actual independently bound checkout, workflow, artifact and K/S/D identities only; original workflow conclusions remain unchanged. No successful-global-closure or linked-requirement credit."
const ciIdentityTimeout = 120 * time.Second
const ciIdentityArchiveLimit = 256 * 1024 * 1024

var ciIdentitySources = []string{
	"tools/language-acceptance/docs_publication_execution.go",
	"tools/language-acceptance/docs_publication_execution_test.go",
	"tools/language-acceptance/ci_identity_execution.go",
	"tools/language-acceptance/ci_identity_execution_test.go",
	"tools/language-acceptance/final_provenance_execution.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
	"tools/language-acceptance/documentation_intake.go",
	"tools/p10r15/generate_macro_contract_coverage.py",
	"skills/cherri-shortcuts/compatibility-manifest.json",
	".github/workflows/build-test.yml",
	".github/workflows/openminis-skill.yml",
	".github/workflows/ios-build.yml",
	".github/workflows/ios27-shortcuts-runtime-poc.yml",
	"tests/language-v2/acceptance-contract.json",
}

var ciIdentityRequiredWorkflows = map[string]string{
	"build":   ".github/workflows/build-test.yml",
	"skill":   ".github/workflows/openminis-skill.yml",
	"app":     ".github/workflows/ios-build.yml",
	"runtime": ".github/workflows/ios27-shortcuts-runtime-poc.yml",
	"docs":    ".github/workflows/pages.yml",
}

// These are the actual checkout-bearing jobs in the five pinned workflow
// sources. Unknown new jobs stay unqualified until their source is reviewed.
var ciIdentityRequiredJobs = map[string][]string{
	"build":   {"build-test-linux", "build-test-macos", "build-test-nix"},
	"skill":   {"build-test-package"},
	"app":     {"ios-build"},
	"runtime": {"ios27-runtime-poc"},
	"docs":    {"build"},
}

type ciIdentityConfig struct {
	ExpectedPath, ExpectedSHA256, ManifestPath string
}

type ciArchiveMember struct {
	SHA256 string `json:"sha256"`
	Bytes  int64  `json:"bytes"`
	Mode   uint32 `json:"mode"`
}

type ciMemberReference struct {
	Artifact string `json:"artifact"`
	Member   string `json:"member"`
}

type ciCheckoutObservation struct {
	Run          string `json:"run"`
	JobsFile     string `json:"jobsFile"`
	JobID        int64  `json:"jobId"`
	JobName      string `json:"jobName"`
	LogFile      string `json:"logFile"`
	CheckoutStep int64  `json:"checkoutStep"`
	CheckoutName string `json:"checkoutName"`
}

type ciIdentityExpected struct {
	Schema                 string                                `json:"schema"`
	ManifestSHA256         string                                `json:"manifestSha256"`
	Bindings               finalProvenanceExpected               `json:"bindings"`
	Checkouts              map[string][]ciCheckoutObservation    `json:"checkouts"`
	WorkflowSHA256         map[string]string                     `json:"workflowSha256"`
	Archives               map[string]map[string]ciArchiveMember `json:"archives"`
	Macro                  ciMemberReference                     `json:"macro"`
	MacroSourceFingerprint string                                `json:"macroSourceFingerprint"`
	DocsBuild              ciMemberReference                     `json:"docsBuild"`
	SkillPackage           ciMemberReference                     `json:"skillPackage"`
	SkillMembers           map[string]ciArchiveMember            `json:"skillMembers"`
	ImplementationCompiler ciMemberReference                     `json:"implementationCompiler"`
	MacroCompiler          ciMemberReference                     `json:"macroCompiler"`
	CompilerFile           string                                `json:"compilerFile"`
	MigratorFile           string                                `json:"migratorFile"`
}

type ciIdentityManifest struct {
	Schema   string                  `json:"schema"`
	Bindings finalProvenanceManifest `json:"bindings"`
}

type ciIdentityObservation struct {
	Schema              string            `json:"schema"`
	ImplementationSHA   string            `json:"implementationSha"`
	ExpectedSHA256      string            `json:"expectedSha256"`
	ManifestSHA256      string            `json:"manifestSha256"`
	StartedUTC          string            `json:"startedUtc"`
	FinishedUTC         string            `json:"finishedUtc"`
	ElapsedSeconds      float64           `json:"elapsedSeconds"`
	InputsCompletedUTC  string            `json:"inputsCompletedUtc"`
	Checkouts           map[string]string `json:"observedCheckouts"`
	WorkflowConclusions map[string]string `json:"workflowConclusions"`
	JobConclusions      map[string]string `json:"jobConclusions"`
	CompilerSource      string            `json:"compilerSource"`
	DocsSource          string            `json:"docsSource"`
	Status              string            `json:"status"`
	Scope               string            `json:"scope"`
	ClosureComplete     bool              `json:"closureComplete"`
	Failure             string            `json:"failure,omitempty"`
}

func ciIdentityEnvironment() (ciIdentityConfig, bool, error) {
	values := []string{os.Getenv("CHERRI_CI03_EXPECTED"), os.Getenv("CHERRI_CI03_EXPECTED_SHA256"), os.Getenv("CHERRI_CI03_MANIFEST")}
	configured := 0
	for _, value := range values {
		if value != "" {
			configured++
		}
	}
	if configured == 0 {
		return ciIdentityConfig{}, false, nil
	}
	if configured != 3 || !finalProvenanceHex(values[1], 64) {
		return ciIdentityConfig{}, true, fmt.Errorf("CI03_INCOMPLETE_CALLER_CONTEXT")
	}
	return ciIdentityConfig{values[0], values[1], values[2]}, true, nil
}

func ciIdentityLoad(config ciIdentityConfig, outputDirectory string) (*finalProvenanceInputs, ciIdentityExpected, error) {
	var expected ciIdentityExpected
	var manifest ciIdentityManifest
	if err := finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &expected); err != nil {
		return nil, expected, err
	}
	if expected.Schema != "cherri-ci03-expected/1" || !finalProvenanceHex(expected.ManifestSHA256, 64) {
		return nil, expected, fmt.Errorf("CI03_EXPECTED_SCHEMA")
	}
	if err := finalProvenanceReadContext(config.ManifestPath, expected.ManifestSHA256, &manifest); err != nil {
		return nil, expected, err
	}
	if manifest.Schema != "cherri-ci03-locators/1" {
		return nil, expected, fmt.Errorf("CI03_MANIFEST_SCHEMA")
	}
	if expected.Bindings.Ledger != nil || len(manifest.Bindings.Ledger) != 0 || len(manifest.Bindings.Limitations) != 0 || manifest.Bindings.ClosureComplete {
		return nil, expected, fmt.Errorf("CI03_DOES_NOT_CONSUME_OR_GRANT_FINAL_LEDGER")
	}
	in := &finalProvenanceInputs{Expected: expected.Bindings, Manifest: manifest.Bindings, JSON: map[string]map[string]any{}, Bytes: map[string][]byte{}}
	if err := finalProvenanceLoadBindings(in, outputDirectory); err != nil {
		return nil, expected, err
	}
	return in, expected, nil
}

func ciIdentityDigest(data []byte) string {
	value := sha256.Sum256(data)
	return hex.EncodeToString(value[:])
}

type ciExpandedBudget struct {
	Used  int64
	Limit int64
}

func (budget *ciExpandedBudget) consume(size int64) error {
	if budget == nil || size < 0 || budget.Limit <= 0 || size > budget.Limit-budget.Used {
		return fmt.Errorf("CI_AGGREGATE_EXPANDED_LIMIT")
	}
	budget.Used += size
	return nil
}

// Retained originals may be larger than the shared four-MiB JSON cap. This
// bounded reader still verifies the independent digest and the same inode.
func ciIdentityReadOriginal(in *finalProvenanceInputs, id string, limit int64) ([]byte, error) {
	pin, ok := in.Expected.Files[id]
	if !ok || pin.Bytes < 0 || pin.Bytes > limit || !finalProvenanceHex(pin.SHA256, 64) {
		return nil, fmt.Errorf("CI_ORIGINAL_INVENTORY")
	}
	name, ok := in.Manifest.Files[id]
	if !ok || !filepath.IsAbs(name) {
		return nil, finalProvenancePending("CI_ORIGINAL_UNAVAILABLE")
	}
	before, err := os.Lstat(name)
	if os.IsNotExist(err) {
		return nil, finalProvenancePending("CI_ORIGINAL_UNAVAILABLE")
	}
	if err != nil || !before.Mode().IsRegular() || before.Size() != pin.Bytes {
		return nil, fmt.Errorf("CI_ORIGINAL_FILE")
	}
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("CI_ORIGINAL_OPEN")
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return nil, fmt.Errorf("CI_ORIGINAL_REPLACED")
	}
	data, err := io.ReadAll(io.LimitReader(file, limit+1))
	after, afterErr := file.Stat()
	pathAfter, pathErr := os.Lstat(name)
	if err != nil || afterErr != nil || pathErr != nil || int64(len(data)) != pin.Bytes || int64(len(data)) > limit ||
		!os.SameFile(before, after) || !os.SameFile(before, pathAfter) || before.Size() != after.Size() ||
		before.Mode() != after.Mode() || !before.ModTime().Equal(after.ModTime()) || ciIdentityDigest(data) != pin.SHA256 {
		return nil, fmt.Errorf("CI_ORIGINAL_BYTES_CHANGED")
	}
	return data, nil
}

func ciIdentitySafeMember(name string) bool {
	return name != "" && utf8.ValidString(name) && !strings.ContainsAny(name, "\\\x00\r\n") &&
		!strings.HasPrefix(name, "/") && path.Clean(name) == name && name != "." && name != ".." && !strings.HasPrefix(name, "../")
}

// Every member, including unused members, is independently enumerated and
// hashed. No extraction, symlink following, executable loading or key access.
func ciIdentityArchive(data []byte, inventory map[string]ciArchiveMember, budget *ciExpandedBudget) (map[string][]byte, error) {
	if len(data) == 0 || len(data) > ciIdentityArchiveLimit || len(inventory) == 0 || len(inventory) > 4096 {
		return nil, fmt.Errorf("CI_ARCHIVE_LIMIT")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) != len(inventory) {
		return nil, fmt.Errorf("CI_ARCHIVE_INVENTORY")
	}
	result := make(map[string][]byte, len(inventory))
	var total uint64
	for _, file := range reader.File {
		pin, exists := inventory[file.Name]
		_, duplicate := result[file.Name]
		if !exists || duplicate || !ciIdentitySafeMember(file.Name) || !file.Mode().IsRegular() ||
			file.Flags&1 != 0 || (file.Method != zip.Store && file.Method != zip.Deflate) || pin.Bytes < 0 ||
			pin.Bytes > ciIdentityArchiveLimit || file.UncompressedSize64 != uint64(pin.Bytes) ||
			uint32(file.Mode().Perm()) != pin.Mode || !finalProvenanceHex(pin.SHA256, 64) {
			return nil, fmt.Errorf("CI_ARCHIVE_MEMBER")
		}
		total += file.UncompressedSize64
		if total > ciIdentityArchiveLimit {
			return nil, fmt.Errorf("CI_ARCHIVE_EXPANDED_LIMIT")
		}
		if err = budget.consume(pin.Bytes); err != nil {
			return nil, err
		}
		stream, openErr := file.Open()
		if openErr != nil {
			return nil, fmt.Errorf("CI_ARCHIVE_OPEN")
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, pin.Bytes+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || int64(len(body)) != pin.Bytes || ciIdentityDigest(body) != pin.SHA256 {
			return nil, fmt.Errorf("CI_ARCHIVE_DIGEST")
		}
		result[file.Name] = body
	}
	return result, nil
}

func ciIdentityArchiveInputs(in *finalProvenanceInputs, expected ciIdentityExpected, budget *ciExpandedBudget) (map[string]map[string][]byte, error) {
	if len(expected.Archives) == 0 || len(expected.Archives) > 32 {
		return nil, fmt.Errorf("CI03_ARCHIVES_MISSING")
	}
	result := map[string]map[string][]byte{}
	var total int64
	for id, inventory := range expected.Archives {
		artifact, exists := in.Expected.Artifacts[id]
		if !exists || artifact.Availability != "github" || artifact.Historical {
			return nil, fmt.Errorf("CI03_ARCHIVE_NOT_CURRENT_AUTHENTICATED_ARTIFACT")
		}
		if err := ciIdentityArtifactAttempt(in, artifact); err != nil {
			return nil, err
		}
		pin := in.Expected.Files[artifact.File]
		total += pin.Bytes
		if total > ciIdentityArchiveLimit {
			return nil, fmt.Errorf("CI03_TOTAL_ARCHIVE_LIMIT")
		}
		body, err := ciIdentityReadOriginal(in, artifact.File, ciIdentityArchiveLimit)
		if err != nil {
			return nil, err
		}
		members, err := ciIdentityArchive(body, inventory, budget)
		if err != nil {
			return nil, err
		}
		result[id] = members
	}
	return result, nil
}

func ciIdentityArtifactAttempt(in *finalProvenanceInputs, artifact finalProvenanceArtifact) error {
	run, exists := in.Expected.Runs[artifact.Run]
	if !exists {
		return fmt.Errorf("CI_ARTIFACT_RUN")
	}
	rawRun, err := in.object(run.Original)
	if err != nil {
		return err
	}
	start, end, err := ciIdentityTimeRange(finalProvenanceString(rawRun, "run_started_at"), finalProvenanceString(rawRun, "updated_at"))
	if err != nil {
		return err
	}
	metadata, err := in.object(artifact.Metadata)
	if err != nil {
		return err
	}
	rows := finalProvenanceArray(metadata, "artifacts")
	if _, list := metadata["artifacts"]; !list {
		rows = []any{metadata}
	}
	var selected map[string]any
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("CI_ARTIFACT_SCHEMA")
		}
		id, ok := finalProvenanceNumber(row, "id")
		if ok && id == artifact.ID {
			if selected != nil {
				return fmt.Errorf("CI_DUPLICATE_ARTIFACT")
			}
			selected = row
		}
	}
	if selected == nil {
		return finalProvenancePending("CI_ARTIFACT_MISSING")
	}
	created, err := finalProvenanceTime(finalProvenanceString(selected, "created_at"))
	if err != nil || created.Before(start) || !created.Before(end) {
		return fmt.Errorf("CI_ARTIFACT_FROM_ANOTHER_ATTEMPT")
	}
	return nil
}

func ciIdentityMember(archives map[string]map[string][]byte, ref ciMemberReference) ([]byte, error) {
	data, exists := archives[ref.Artifact][ref.Member]
	if !exists {
		return nil, finalProvenancePending("CI03_REQUIRED_ORIGINAL_MEMBER_MISSING")
	}
	return data, nil
}

// GitHub job timestamps are displayed at their stated precision. An API end
// second denotes that whole second, not an invented subsecond cutoff.
func ciIdentityTimeRange(start, end string) (time.Time, time.Time, error) {
	a, err := time.Parse(time.RFC3339Nano, start)
	b, other := time.Parse(time.RFC3339Nano, end)
	if err != nil || other != nil || b.Before(a) {
		return a, b, fmt.Errorf("CI_JOB_TIME")
	}
	precision := time.Second
	if dot := strings.IndexByte(end, '.'); dot >= 0 {
		last := strings.IndexAny(end[dot+1:], "Z+-")
		if last < 1 || last > 9 {
			return a, b, fmt.Errorf("CI_JOB_TIME_PRECISION")
		}
		for i := 0; i < last; i++ {
			precision /= 10
		}
	}
	return a, b.Add(precision), nil
}

var ciRunnerTimestamp = regexp.MustCompile(`^([0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(?:\.[0-9]{1,9})?Z) (.*)$`)
var ciCheckoutCommand = regexp.MustCompile(`^\[command\]/(?:[^/\s]+/)*git log -1 --format=%H$`)

func ciIdentityRunnerLine(line string) (time.Time, string, bool) {
	line = strings.TrimPrefix(line, "\ufeff") // Original BOM may precede each timestamp.
	match := ciRunnerTimestamp.FindStringSubmatch(line)
	if match == nil {
		return time.Time{}, "", false
	}
	t, err := time.Parse(time.RFC3339Nano, match[1])
	return t, strings.TrimSuffix(match[2], "\r"), err == nil
}

func ciIdentityCheckoutSHA(raw []byte, started, completed, expected string) (string, error) {
	if len(raw) == 0 || len(raw) > 64*1024*1024 || !utf8.Valid(raw) || !finalProvenanceHex(expected, 40) {
		return "", fmt.Errorf("CI_CHECKOUT_LOG")
	}
	a, b, err := ciIdentityTimeRange(started, completed)
	if err != nil {
		return "", err
	}
	commands, awaiting, observed := 0, false, ""
	for _, line := range strings.Split(string(raw), "\n") {
		t, body, ok := ciIdentityRunnerLine(line)
		if !ok || t.Before(a) || !t.Before(b) {
			continue
		}
		if ciCheckoutCommand.MatchString(body) {
			commands++
			awaiting = true
			continue
		}
		if awaiting {
			if body == "" {
				continue
			}
			if !finalProvenanceHex(body, 40) {
				return "", fmt.Errorf("CI_CHECKOUT_COMMAND_OUTPUT")
			}
			observed, awaiting = body, false
		}
	}
	if commands == 0 {
		return "", finalProvenancePending("CI_CHECKOUT_COMMAND_NOT_OBSERVED")
	}
	if commands != 1 || awaiting || observed != expected {
		return "", fmt.Errorf("CI_CHECKOUT_SOURCE_MISMATCH_OR_AMBIGUITY")
	}
	return observed, nil
}

func ciIdentityJob(in *finalProvenanceInputs, witness ciCheckoutObservation) (map[string]any, map[string]any, error) {
	run, exists := in.Expected.Runs[witness.Run]
	if !exists || witness.JobID <= 0 || witness.CheckoutStep <= 0 || witness.JobName == "" || witness.CheckoutName == "" {
		return nil, nil, fmt.Errorf("CI_CHECKOUT_INVENTORY")
	}
	object, err := in.object(witness.JobsFile)
	if err != nil {
		return nil, nil, err
	}
	jobs, ok := object["jobs"].([]any)
	count, countOK := finalProvenanceNumber(object, "total_count")
	if !ok || !countOK || count != int64(len(jobs)) || count <= 0 || count > 100 {
		return nil, nil, fmt.Errorf("CI_JOBS_INCOMPLETE")
	}
	var selected map[string]any
	jobIDs, jobNames := map[int64]bool{}, map[string]bool{}
	for _, raw := range jobs {
		job, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("CI_JOB_SCHEMA")
		}
		id, idOK := finalProvenanceNumber(job, "id")
		rid, ridOK := finalProvenanceNumber(job, "run_id")
		attempt, attemptOK := finalProvenanceNumber(job, "run_attempt")
		name := finalProvenanceString(job, "name")
		if !idOK || id <= 0 || !ridOK || !attemptOK || rid != run.ID || attempt != int64(run.Attempt) || finalProvenanceString(job, "head_sha") != run.SourceSHA || name == "" || jobIDs[id] || jobNames[name] {
			return nil, nil, fmt.Errorf("CI_JOB_SOURCE_OR_ATTEMPT")
		}
		jobIDs[id], jobNames[name] = true, true
		if id == witness.JobID {
			if selected != nil {
				return nil, nil, fmt.Errorf("CI_DUPLICATE_JOB")
			}
			selected = job
		}
	}
	if selected == nil {
		return nil, nil, finalProvenancePending("CI_CHECKOUT_JOB_UNAVAILABLE")
	}
	if finalProvenanceString(selected, "name") != witness.JobName {
		return nil, nil, fmt.Errorf("CI_CHECKOUT_JOB_NAME_MISMATCH")
	}
	if finalProvenanceString(selected, "status") != "completed" {
		return nil, nil, finalProvenancePending("CI_CHECKOUT_JOB_NOT_COMPLETED")
	}
	var step map[string]any
	for _, raw := range finalProvenanceArray(selected, "steps") {
		candidate, ok := raw.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("CI_STEP_SCHEMA")
		}
		number, ok := finalProvenanceNumber(candidate, "number")
		if !ok {
			return nil, nil, fmt.Errorf("CI_STEP_NUMBER")
		}
		if number == witness.CheckoutStep {
			if step != nil {
				return nil, nil, fmt.Errorf("CI_DUPLICATE_CHECKOUT_STEP")
			}
			step = candidate
		}
	}
	if step == nil {
		return nil, nil, finalProvenancePending("CI_CHECKOUT_STEP_UNAVAILABLE")
	}
	if finalProvenanceString(step, "name") != witness.CheckoutName {
		return nil, nil, fmt.Errorf("CI_CHECKOUT_STEP_NAME_MISMATCH")
	}
	if finalProvenanceString(step, "status") != "completed" {
		return nil, nil, finalProvenancePending("CI_CHECKOUT_STEP_NOT_COMPLETED")
	}
	if finalProvenanceString(step, "conclusion") != "success" {
		return nil, nil, fmt.Errorf("CI_CHECKOUT_STEP_DID_NOT_SUCCEED")
	}
	return selected, step, nil
}

func ciIdentityValidateCheckouts(ctx context.Context, in *finalProvenanceInputs, expected ciIdentityExpected) (map[string]string, map[string]string, map[string]string, error) {
	if len(expected.Checkouts) != len(ciIdentityRequiredWorkflows) || len(expected.WorkflowSHA256) != len(ciIdentityRequiredWorkflows) {
		return nil, nil, nil, finalProvenancePending("CI03_FIVE_ROLE_WORKFLOWS_REQUIRED")
	}
	observed, conclusions, jobConclusions := map[string]string{}, map[string]string{}, map[string]string{}
	used := map[string]bool{}
	for role, workflowPath := range ciIdentityRequiredWorkflows {
		witnesses, exists := expected.Checkouts[role]
		if !exists || len(witnesses) != len(ciIdentityRequiredJobs[role]) {
			return nil, nil, nil, finalProvenancePending("CI03_COMPLETE_CHECKOUT_JOB_INVENTORY_REQUIRED")
		}
		first := witnesses[0]
		run, runOK := in.Expected.Runs[first.Run]
		sourceRole := "implementation"
		if role == "docs" {
			sourceRole = "docs"
		}
		source := in.Expected.Sources[sourceRole]
		if !runOK || used[first.Run] || run.Historical || run.Repository != source.Repository || run.SourceSHA != source.SHA || run.WorkflowPath != workflowPath {
			return nil, nil, nil, fmt.Errorf("CI03_REQUIRED_WORKFLOW_IDENTITY")
		}
		used[first.Run] = true
		if run.Status != "completed" {
			return nil, nil, nil, finalProvenancePending("CI03_REQUIRED_WORKFLOW_NOT_TERMINAL")
		}
		rawRun, err := in.object(run.Original)
		if err != nil {
			return nil, nil, nil, err
		}
		if finalProvenanceString(rawRun, "head_branch") != strings.TrimPrefix(source.PublishedRef, "refs/heads/") {
			return nil, nil, nil, fmt.Errorf("CI03_RUN_REF_MISMATCH")
		}
		workflow, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories[sourceRole], "show", source.SHA+":"+workflowPath)
		if err != nil || !finalProvenanceHex(expected.WorkflowSHA256[role], 64) || ciIdentityDigest(workflow) != expected.WorkflowSHA256[role] {
			return nil, nil, nil, fmt.Errorf("CI03_WORKFLOW_SOURCE_BYTES")
		}
		seen := map[string]bool{}
		for _, witness := range witnesses {
			if witness.Run != first.Run || witness.JobsFile != first.JobsFile || seen[witness.JobName] {
				return nil, nil, nil, fmt.Errorf("CI03_CHECKOUT_JOB_INVENTORY_CONTRADICTION")
			}
			allowed := false
			for _, name := range ciIdentityRequiredJobs[role] {
				if name == witness.JobName {
					allowed = true
				}
			}
			if !allowed {
				return nil, nil, nil, fmt.Errorf("CI03_UNEXPECTED_CHECKOUT_JOB")
			}
			job, step, err := ciIdentityJob(in, witness)
			if err != nil {
				return nil, nil, nil, err
			}
			log, err := ciIdentityReadOriginal(in, witness.LogFile, 64*1024*1024)
			if err != nil {
				return nil, nil, nil, err
			}
			sha, err := ciIdentityCheckoutSHA(log, finalProvenanceString(step, "started_at"), finalProvenanceString(step, "completed_at"), source.SHA)
			if err != nil {
				return nil, nil, nil, err
			}
			key := role + "/" + witness.JobName
			observed[key], jobConclusions[key], seen[witness.JobName] = sha, finalProvenanceString(job, "conclusion"), true
		}
		jobs, err := in.object(first.JobsFile)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, value := range finalProvenanceArray(jobs, "jobs") {
			job, ok := value.(map[string]any)
			if !ok {
				return nil, nil, nil, fmt.Errorf("CI03_JOB_INVENTORY_SCHEMA")
			}
			name := finalProvenanceString(job, "name")
			if seen[name] {
				continue
			}
			// The unselected component branch has no checkout execution. Pages'
			// deploy job downloads the source-bound build artifact, without checkout.
			if role == "app" && name == "documentation-component" && finalProvenanceString(job, "conclusion") == "skipped" && len(finalProvenanceArray(job, "steps")) == 0 {
				jobConclusions[role+"/"+name] = "skipped"
				continue
			}
			if role == "docs" && name == "deploy" && finalProvenanceString(job, "status") == "completed" {
				for _, value := range finalProvenanceArray(job, "steps") {
					step, ok := value.(map[string]any)
					if !ok || strings.Contains(strings.ToLower(finalProvenanceString(step, "name")), "checkout") {
						return nil, nil, nil, fmt.Errorf("CI03_UNOBSERVED_DEPLOY_CHECKOUT")
					}
				}
				jobConclusions[role+"/"+name] = finalProvenanceString(job, "conclusion")
				continue
			}
			return nil, nil, nil, finalProvenancePending("CI03_UNREVIEWED_WORKFLOW_JOB")
		}
		conclusions[role] = run.Conclusion
	}
	return observed, conclusions, jobConclusions, nil
}

func ciIdentityBinary(data []byte, source, mainPackage string) (map[string]string, error) {
	if len(data) == 0 || len(data) > 128*1024*1024 {
		return nil, fmt.Errorf("CI_BINARY_LIMIT")
	}
	info, err := buildinfo.Read(bytes.NewReader(data))
	if err != nil || info.Path != mainPackage {
		return nil, fmt.Errorf("CI_BINARY_ROLE")
	}
	settings := map[string]string{}
	for _, item := range info.Settings {
		if _, exists := settings[item.Key]; exists {
			return nil, fmt.Errorf("CI_BINARY_DUPLICATE_SETTING")
		}
		settings[item.Key] = item.Value
	}
	if settings["vcs"] != "git" || settings["vcs.revision"] != source || settings["vcs.modified"] != "false" || settings["GOOS"] == "" || settings["GOARCH"] == "" {
		return nil, fmt.Errorf("CI_BINARY_SOURCE_OR_PLATFORM")
	}
	return settings, nil
}

func ciIdentityMacroCompiler(macro map[string]any, binary []byte, source string) error {
	if _, err := ciIdentityBinary(binary, source, "github.com/electrikmilk/cherri"); err != nil {
		return err
	}
	if finalProvenanceString(macro, "compilerSHA256") != ciIdentityDigest(binary) {
		return fmt.Errorf("CI03_EXACT_MACRO_COMPILER_MISMATCH")
	}
	return nil
}

func ciIdentitySurfaceArtifact(in *finalProvenanceInputs, ref ciMemberReference, expectedRun string) error {
	if ref.Artifact == "" || ref.Member == "" {
		return finalProvenancePending("CI03_REQUIRED_ORIGINAL_MEMBER_MISSING")
	}
	artifact, exists := in.Expected.Artifacts[ref.Artifact]
	if !exists || expectedRun == "" || artifact.Run != expectedRun {
		return fmt.Errorf("CI03_SURFACE_FROM_ANOTHER_WORKFLOW")
	}
	return nil
}

func ciIdentityPins(ctx context.Context, in *finalProvenanceInputs) (documentationPins, []byte, error) {
	var pins documentationPins
	k := in.Expected.Sources["implementation"]
	raw, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", k.SHA+":skills/cherri-shortcuts/compatibility-manifest.json")
	if err != nil {
		return pins, nil, err
	}
	if err = decodeDocumentationJSON(raw, &pins); err != nil {
		return pins, nil, err
	}
	contract, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", k.SHA+":tests/language-v2/acceptance-contract.json")
	if err != nil || ciIdentityDigest(contract) != "9fdfa8910b09824dc61395d8ddf4aa8fe8914619af0fdde59c63b67ab10b895e" {
		return pins, nil, fmt.Errorf("CI03_IMMUTABLE_CONTRACT_CHANGED")
	}
	if pins.CompilerCommit != in.Expected.Sources["compiler"].SHA || pins.DocsCommit != in.Expected.Sources["docs"].SHA ||
		pins.CompilerRepo != "https://github.com/davidpovarsky/cherri.git" || pins.DocsRepo != "https://github.com/davidpovarsky/cherrilang.org.git" ||
		!finalProvenanceHex(pins.SchemaFingerprint, 64) || !finalProvenanceHex(pins.CodecFingerprint, 64) || !finalProvenanceHex(pins.MacroFingerprint, 64) || pins.LanguageVersion != "2.0" {
		return pins, nil, fmt.Errorf("CI03_DECLARED_K_S_D_TUPLE")
	}
	if _, _, err = finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "merge-base", "--is-ancestor", pins.CompilerCommit, k.SHA); err != nil {
		return pins, nil, fmt.Errorf("CI03_COMPILER_NOT_ANCESTOR")
	}
	return pins, raw, nil
}

// The remaining surface checks are intentionally separate from workflow success.
// Native success is not inferred from an app checkout or from any build ID.
func ciIdentityValidateSurfaces(ctx context.Context, in *finalProvenanceInputs, expected ciIdentityExpected, archives map[string]map[string][]byte, budget *ciExpandedBudget) error {
	pins, manifestBytes, err := ciIdentityPins(ctx, in)
	if err != nil {
		return err
	}
	k := in.Expected.Sources["implementation"].SHA
	for _, item := range []struct {
		ref      ciMemberReference
		workflow string
	}{{expected.Macro, "skill"}, {expected.SkillPackage, "skill"}, {expected.ImplementationCompiler, "skill"}, {expected.MacroCompiler, "skill"}, {expected.DocsBuild, "docs"}} {
		if len(expected.Checkouts[item.workflow]) == 0 {
			return finalProvenancePending("CI03_REQUIRED_WORKFLOW_CHECKOUT_MISSING")
		}
		if err := ciIdentitySurfaceArtifact(in, item.ref, expected.Checkouts[item.workflow][0].Run); err != nil {
			return err
		}
	}
	macroBytes, err := ciIdentityMember(archives, expected.Macro)
	if err != nil {
		return err
	}
	macro, err := decodeNativeEncodingObject(macroBytes)
	if err != nil || finalProvenanceString(macro, "sourceSha") != k || !finalProvenanceBool(macro, "worktreeDirtyBeforeExecution", false) ||
		finalProvenanceString(macro, "languageVersion") != pins.LanguageVersion || finalProvenanceString(macro, "schemaFingerprint") != pins.SchemaFingerprint ||
		finalProvenanceString(macro, "macroFingerprint") != pins.MacroFingerprint || !finalProvenanceHex(finalProvenanceString(macro, "compilerSHA256"), 64) ||
		!finalProvenanceHex(expected.MacroSourceFingerprint, 64) || finalProvenanceString(macro, "sourceFingerprint") != expected.MacroSourceFingerprint {
		return fmt.Errorf("CI03_LANGUAGE_IDENTITY")
	}
	macroProducer, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", k+":tools/p10r15/generate_macro_contract_coverage.py")
	if err != nil || finalProvenanceString(macro, "generatorSHA256") != ciIdentityDigest(bytes.ReplaceAll(macroProducer, []byte("\r\n"), []byte("\n"))) {
		return fmt.Errorf("CI03_LANGUAGE_PRODUCER")
	}
	docsBytes, err := ciIdentityMember(archives, expected.DocsBuild)
	if err != nil {
		return err
	}
	var docs documentationSiteBuild
	if err = decodeDocumentationJSON(docsBytes, &docs); err != nil || docs.FormatVersion != 2 || !docs.SourceTreeClean || docs.DocsCommit != pins.DocsCommit || docs.CompilerCommit != pins.CompilerCommit {
		return fmt.Errorf("CI03_DOCS_BUILD_IDENTITY")
	}
	got := append([]string{}, docs.SchemaFingerprints...)
	want := []string{pins.SchemaFingerprint, pins.MacroFingerprint}
	sort.Strings(got)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		return fmt.Errorf("CI03_DOCS_FINGERPRINTS")
	}
	implementationBinary, err := ciIdentityMember(archives, expected.ImplementationCompiler)
	if err != nil {
		return err
	}
	if _, err = ciIdentityBinary(implementationBinary, k, "github.com/electrikmilk/cherri"); err != nil {
		return err
	}
	macroBinary, err := ciIdentityMember(archives, expected.MacroCompiler)
	if err != nil {
		return err
	}
	if err = ciIdentityMacroCompiler(macro, macroBinary, k); err != nil {
		return err
	}
	compiler, err := ciIdentityReadOriginal(in, expected.CompilerFile, 128*1024*1024)
	if err != nil {
		return err
	}
	migrator, err := ciIdentityReadOriginal(in, expected.MigratorFile, 128*1024*1024)
	if err != nil {
		return err
	}
	cmeta, err := ciIdentityBinary(compiler, pins.CompilerCommit, "github.com/electrikmilk/cherri")
	if err != nil {
		return err
	}
	mmeta, err := ciIdentityBinary(migrator, pins.CompilerCommit, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	if err != nil || cmeta["GOOS"] != mmeta["GOOS"] || cmeta["GOARCH"] != mmeta["GOARCH"] {
		return fmt.Errorf("CI03_COMPILER_PAIR_PLATFORM")
	}
	packageBytes, err := ciIdentityMember(archives, expected.SkillPackage)
	if err != nil {
		return err
	}
	members, err := ciIdentityArchive(packageBytes, expected.SkillMembers, budget)
	if err != nil {
		return err
	}
	if !bytes.Equal(members["cherri-shortcuts/compatibility-manifest.json"], manifestBytes) {
		return fmt.Errorf("CI03_PACKAGED_MANIFEST_DIFFERS_FROM_K")
	}
	packageManifest, err := decodeNativeEncodingObject(members["cherri-shortcuts/package-manifest.json"])
	if err != nil || finalProvenanceString(packageManifest, "kind") != "runtime" || finalProvenanceString(packageManifest, "skillCommit") != k ||
		finalProvenanceString(packageManifest, "compilerCommit") != pins.CompilerCommit || finalProvenanceString(packageManifest, "docsCommit") != pins.DocsCommit {
		return fmt.Errorf("CI03_SKILL_PACKAGE_TUPLE")
	}
	for _, role := range []struct{ name, main string }{{"cherri", "github.com/electrikmilk/cherri"}, {"cherri-migrate", "github.com/electrikmilk/cherri/cmd/cherri-migrate"}} {
		name := "cherri-shortcuts/bin/" + role.name
		binary, exists := members[name]
		if !exists {
			return finalProvenancePending("CI03_PACKAGED_RUNTIME_MISSING")
		}
		meta, err := ciIdentityBinary(binary, pins.CompilerCommit, role.main)
		if err != nil || meta["GOOS"] != "linux" || meta["GOARCH"] != "arm64" {
			return fmt.Errorf("CI03_PACKAGED_RUNTIME_IDENTITY")
		}
		if string(members[name+".commit"]) != pins.CompilerCommit+"\n" || string(members[name+".sha256"]) != ciIdentityDigest(binary)+"\n" {
			return fmt.Errorf("CI03_PACKAGED_RUNTIME_SIDECARS")
		}
	}
	// package_skill.py's receipt inventory is checked against every actual member.
	rows := finalProvenanceArray(packageManifest, "files")
	if len(rows)+1 != len(members) {
		return fmt.Errorf("CI03_PACKAGE_RECEIPT_INVENTORY")
	}
	gitInventory, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "ls-tree", "-r", "-z", k, "--", "skills/cherri-shortcuts", "LICENSE")
	if err != nil {
		return err
	}
	gitMembers := map[string]string{}
	gitModes := map[string]uint32{}
	for _, line := range bytes.Split(gitInventory, []byte{0}) {
		if len(line) == 0 {
			continue
		}
		pair := bytes.SplitN(line, []byte{'\t'}, 2)
		if len(pair) != 2 {
			return fmt.Errorf("CI03_SKILL_GIT_INVENTORY")
		}
		fields := strings.Fields(string(pair[0]))
		if len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
			return fmt.Errorf("CI03_SKILL_GIT_MODE")
		}
		gitPath := string(pair[1])
		name := "cherri-shortcuts/" + strings.TrimPrefix(gitPath, "skills/cherri-shortcuts/")
		if gitPath == "LICENSE" {
			name = "cherri-shortcuts/LICENSE.cherri"
		}
		mode, _ := strconv.ParseUint(fields[0], 8, 32)
		gitMembers[name], gitModes[name] = gitPath, uint32(mode)&0o777
	}
	generated := map[string]bool{}
	for _, name := range []string{"cherri", "cherri-migrate"} {
		for _, suffix := range []string{"", ".commit", ".sha256"} {
			generated["cherri-shortcuts/bin/"+name+suffix] = true
		}
	}
	if len(members) != len(gitMembers)+len(generated)+1 {
		return fmt.Errorf("CI03_SKILL_COMPLETE_SOURCE_INVENTORY")
	}
	seen := map[string]bool{}
	for _, raw := range rows {
		row, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("CI03_PACKAGE_RECEIPT_ROW")
		}
		name := finalProvenanceString(row, "path")
		data, exists := members[name]
		n, nOK := finalProvenanceNumber(row, "bytes")
		mode, modeErr := strconv.ParseUint(finalProvenanceString(row, "mode"), 0, 32)
		if !exists || seen[name] || name == "cherri-shortcuts/package-manifest.json" || !nOK || n != int64(len(data)) ||
			finalProvenanceString(row, "sha256") != ciIdentityDigest(data) || modeErr != nil || uint32(mode)&0o777 != expected.SkillMembers[name].Mode {
			return fmt.Errorf("CI03_PACKAGE_RECEIPT_BYTES")
		}
		seen[name] = true
		if generated[name] {
			continue
		}
		gitPath, exists := gitMembers[name]
		if !exists || expected.SkillMembers[name].Mode != gitModes[name] {
			return fmt.Errorf("CI03_PACKAGE_SOURCE_MEMBERSHIP")
		}
		original, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", k+":"+gitPath)
		if err != nil || !bytes.Equal(original, data) {
			return fmt.Errorf("CI03_PACKAGE_SOURCE_BYTES")
		}
	}
	return nil
}

func executeCIIdentity(config ciIdentityConfig, implementationSHA, outputDirectory string) (ciIdentityObservation, error) {
	start := time.Now()
	observation := ciIdentityObservation{Schema: ciIdentitySchema, ImplementationSHA: implementationSHA, ExpectedSHA256: config.ExpectedSHA256,
		StartedUTC: start.UTC().Format(time.RFC3339Nano), Scope: ciIdentityScope, Status: "FAILED"}
	ctx, cancel := context.WithTimeout(context.Background(), ciIdentityTimeout)
	defer cancel()
	in, expected, err := ciIdentityLoad(config, outputDirectory)
	if err == nil {
		observation.ManifestSHA256 = expected.ManifestSHA256
		if in.Expected.Sources["implementation"].SHA != implementationSHA {
			err = fmt.Errorf("CI03_CALLER_IMPLEMENTATION_MISMATCH")
		}
	}
	if err == nil {
		for _, role := range []string{"implementation", "compiler", "docs"} {
			if _, err = finalProvenanceSnapshotSource(ctx, in.Manifest.SourceDirectories[role], in.Expected.Sources[role]); err != nil {
				break
			}
			if err = finalProvenanceValidatePublication(in, role); err != nil {
				break
			}
		}
	}
	if err == nil {
		err = finalProvenanceValidateRunsAndArtifacts(in)
	}
	if err == nil {
		observation.Checkouts, observation.WorkflowConclusions, observation.JobConclusions, err = ciIdentityValidateCheckouts(ctx, in, expected)
	}
	if err == nil {
		var archives map[string]map[string][]byte
		budget := &ciExpandedBudget{Limit: ciIdentityArchiveLimit}
		archives, err = ciIdentityArchiveInputs(in, expected, budget)
		if err == nil {
			err = ciIdentityValidateSurfaces(ctx, in, expected, archives, budget)
		}
	}
	if err == nil {
		for _, role := range []string{"implementation", "compiler", "docs"} {
			if _, err = finalProvenanceSnapshotSource(ctx, in.Manifest.SourceDirectories[role], in.Expected.Sources[role]); err != nil {
				break
			}
		}
	}
	if err == nil {
		var completions []string
		for _, witnesses := range expected.Checkouts {
			for _, witness := range witnesses {
				job, _, jobErr := ciIdentityJob(in, witness)
				if jobErr != nil {
					err = jobErr
					break
				}
				completions = append(completions, finalProvenanceString(job, "completed_at"))
			}
			if err != nil {
				break
			}
		}
		if err == nil {
			observation.InputsCompletedUTC, err = ciIdentityInputsCompleted(in, completions...)
		}
	}
	if err == nil {
		err = ciIdentityNotBackdated(observation.StartedUTC, observation.InputsCompletedUTC)
	}
	if err == nil && ctx.Err() != nil {
		err = fmt.Errorf("CI03_IDENTITY_DEADLINE")
	}
	if err == nil {
		observation.Status = "PASSED"
		observation.CompilerSource = in.Expected.Sources["compiler"].SHA
		observation.DocsSource = in.Expected.Sources["docs"].SHA
	} else {
		var pending finalProvenancePending
		if errors.As(err, &pending) {
			observation.Status = "NOT_RUN"
		}
		// Never print caller paths or raw response/command bodies.
		observation.Failure = "CI03_IDENTITY_INPUTS_REJECTED_OR_INCOMPLETE"
	}
	observation.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	observation.ElapsedSeconds = time.Since(start).Seconds()
	return observation, err
}

func (r *Runner) runCIIdentityRequirement() {
	r.ciIdentityEvaluated = true
	config, configured, configErr := ciIdentityEnvironment()
	if !configured {
		r.results["CI03"] = CaseResult{ID: "CI03", Group: "verification", Requirement: "Final required workflows use exact final SHA", TestLevel: "repository+CI", Status: "NOT_RUN", Detail: "CI03 independently authenticated checkout/artifact context is not configured"}
		return
	}
	if configErr != nil {
		r.results["CI03"] = CaseResult{ID: "CI03", Group: "verification", Requirement: "Final required workflows use exact final SHA", TestLevel: "repository+CI", Status: "FAILED", Detail: "CI03 caller context is malformed"}
		return
	}
	directory, err := stateEvidenceDirectory(r, "ci03-")
	if err != nil {
		r.results["CI03"] = CaseResult{ID: "CI03", Group: "verification", Requirement: "Final required workflows use exact final SHA", TestLevel: "repository+CI", Status: "FAILED", Detail: "CI03 evidence directory is unavailable"}
		return
	}
	observed, executionErr := executeCIIdentity(config, r.implSHA, directory)
	encoded, err := json.MarshalIndent(observed, "", "  ")
	name := filepath.Join(directory, "ci03-observation.json")
	if err == nil {
		err = os.WriteFile(name, append(encoded, '\n'), 0o600)
	}
	if err != nil {
		observed.Status = "FAILED"
		executionErr = err
	}
	passed := observed.Status == "PASSED" && executionErr == nil
	levels := []string(nil)
	if observed.Status != "NOT_RUN" {
		levels = []string{"repository", "ci"}
	}
	r.results["CI03"] = CaseResult{ID: "CI03", Group: "verification", Requirement: "Final required workflows use exact final SHA", TestLevel: "repository+CI", ExecutedLevels: levels, Passed: passed, Status: observed.Status, Detail: ciIdentityScope}
	if !passed {
		return
	}
	for _, tier := range levels {
		r.records = append(r.records, EvidenceRecord{EvidenceID: "ev-ci03-identity-" + tier, Requirements: []string{"CI03"}, Tier: tier, ImplementationSHA: r.implSHA,
			TestID: ciIdentityTestID, Command: []string{"internal", ciIdentitySchema}, ExitCode: 0, Result: "passed",
			Assertions: map[string]string{"scope": ciIdentityScope, "expected_context_sha256": config.ExpectedSHA256, "manifest_sha256": observed.ManifestSHA256, "closure_complete": "false"},
			Artifacts:  []EvidenceArtifact{{Path: name, SHA256: ciIdentityDigest(append(encoded, '\n'))}}, Detail: ciIdentityScope})
	}
}

func validateCIIdentityRecord(id string, record EvidenceRecord) error {
	if id != "CI03" {
		if record.TestID == ciIdentityTestID {
			return fmt.Errorf("CI03_IDENTITY_DOES_NOT_GRANT_ANOTHER_REQUIREMENT")
		}
		return nil
	}
	if record.TestID != ciIdentityTestID || !reflect.DeepEqual(record.Requirements, []string{"CI03"}) || (record.Tier != "repository" && record.Tier != "ci") ||
		record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Command, []string{"internal", ciIdentitySchema}) ||
		record.Assertions["scope"] != ciIdentityScope || record.Assertions["closure_complete"] != "false" {
		return fmt.Errorf("CI03_RECORD_NOT_DEDICATED_IDENTITY")
	}
	config, configured, err := ciIdentityEnvironment()
	if err != nil || !configured || record.Assertions["expected_context_sha256"] != config.ExpectedSHA256 {
		return fmt.Errorf("CI03_INDEPENDENT_CONTEXT_UNAVAILABLE")
	}
	data, err := stateReadArtifacts(record.Artifacts, []string{"ci03-observation.json"})
	if err != nil {
		return err
	}
	var original ciIdentityObservation
	if err = decodeEditorJSON(data["ci03-observation.json"], &original); err != nil {
		return err
	}
	if original.Schema != ciIdentitySchema || original.Status != "PASSED" || original.ClosureComplete || original.Scope != ciIdentityScope || original.ImplementationSHA != record.ImplementationSHA ||
		original.ExpectedSHA256 != config.ExpectedSHA256 || original.ManifestSHA256 != record.Assertions["manifest_sha256"] || original.Failure != "" {
		return fmt.Errorf("CI03_ORIGINAL_OBSERVATION")
	}
	if err = stateTiming(original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds, ciIdentityTimeout.Seconds()); err != nil {
		return err
	}
	// Revalidation reads the independently pinned originals again. No workflow,
	// native, compiler, test or other producer is launched.
	revalidated, err := executeCIIdentity(config, record.ImplementationSHA, "")
	if err != nil || revalidated.Status != "PASSED" {
		return fmt.Errorf("CI03_ORIGINAL_IDENTITY_REVALIDATION")
	}
	return ciIdentityCompareObservation(original, revalidated)
}

func applyCIIdentityAudit(row *ExecutionAuditRow, runner *Runner) {
	if row.ID != "CI03" || !runner.ciIdentityEvaluated {
		return
	}
	row.TestNames = []string{ciIdentityTestID}
	row.ProductionPath = append([]string{}, ciIdentitySources...)
	row.Scope = ciIdentityScope
	row.PositiveAndNegative = false
	row.Blocker = ""
	if result := runner.results["CI03"]; result.Status != "PASSED" {
		row.Blocker = result.Detail
	}
}

func ciIdentityCompareObservation(original, revalidated ciIdentityObservation) error {
	if err := ciIdentityNotBackdated(original.StartedUTC, revalidated.InputsCompletedUTC); err != nil {
		return err
	}
	original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds = revalidated.StartedUTC, revalidated.FinishedUTC, revalidated.ElapsedSeconds
	if !reflect.DeepEqual(original, revalidated) {
		return fmt.Errorf("CI03_ORIGINAL_IDENTITY_REVALIDATION")
	}
	return nil
}

func ciIdentityInputsCompleted(in *finalProvenanceInputs, additional ...string) (string, error) {
	var latest time.Time
	for _, run := range in.Expected.Runs {
		if run.Historical {
			continue
		}
		if run.Status != "completed" {
			return "", finalProvenancePending("CI_REQUIRED_ORIGINAL_NOT_TERMINAL")
		}
		object, err := in.object(run.Original)
		if err != nil {
			return "", err
		}
		additional = append(additional, finalProvenanceString(object, "updated_at"))
	}
	for _, value := range additional {
		observed, err := time.Parse(time.RFC3339Nano, value)
		if err != nil {
			return "", fmt.Errorf("CI_ORIGINAL_COMPLETION_TIME")
		}
		if observed.After(latest) {
			latest = observed
		}
	}
	if latest.IsZero() {
		return "", finalProvenancePending("CI_ORIGINAL_COMPLETION_NOT_OBSERVED")
	}
	return latest.UTC().Format(time.RFC3339Nano), nil
}

func ciIdentityNotBackdated(start, completed string) error {
	a, first := time.Parse(time.RFC3339Nano, start)
	b, second := time.Parse(time.RFC3339Nano, completed)
	if first != nil || second != nil || a.Before(b) {
		return fmt.Errorf("CI_INTAKE_PREDATES_ORIGINAL_COMPLETION")
	}
	return nil
}
