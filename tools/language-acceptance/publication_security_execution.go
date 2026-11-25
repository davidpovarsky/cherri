package main

// CI04 audits only the independently approved current source/publication scope.
// It is not a whole-disk secret scan or a guarantee about unknown private data.

import (
	"archive/tar"
	"archive/zip"
	"bufio"
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

const publicationSecuritySchema = "cherri-ci04-publication-security/1"
const publicationSecurityTestID = "tools/language-acceptance:CI04PublicationSecurity"
const publicationSecurityTimeout = 180 * time.Second
const publicationSecurityScope = "Exact independently approved K/S/D source trees and declared public artifact inventories, known raw-corpus whole-file digests, actual public sanitizer controls, and the unchanged current-source RP43 signing boundary. No arbitrary-disk, unknown-private-data, Apple-signature, native or global-closure claim."

var publicationSecuritySources = []string{
	"tools/language-acceptance/docs_publication_execution.go",
	"tools/language-acceptance/docs_publication_execution_test.go",
	"tools/language-acceptance/publication_security_execution.go",
	"tools/language-acceptance/publication_security_execution_test.go",
	"tools/language-acceptance/ci_identity_execution.go",
	"tools/language-acceptance/final_provenance_execution.go",
	"tools/language-acceptance/output_signing_execution.go",
	"tools/language-acceptance/main.go",
	"tools/language-acceptance/evidence_map.go",
	"tools/language-acceptance/execution_audit.go",
	"tools/language-acceptance/assertion_scope.go",
	"tools/shortcut-corpus/main.go",
	"tools/shortcuts-simulator-qa/project_prerequisite.py",
	"tools/shortcuts-simulator-qa/project_prerequisite_observation.py",
	"tools/shortcuts-simulator-qa/qa_terminal_receipts.py",
	"tools/shortcuts-simulator-qa/simulator_session_recovery.py",
	"ios/scripts/verify_ios_xcresult_summary.py",
	".github/workflows/build-test.yml",
	"tests/language-v2/acceptance-contract.json",
	"tests/language-v2/repair-regression-plan.json",
}

type publicationSecurityConfig struct{ ExpectedPath, ExpectedSHA256, ManifestPath string }

type publicationSourceMember struct {
	SHA256  string `json:"sha256"`
	Bytes   int64  `json:"bytes"`
	GitBlob string `json:"gitBlob"`
	GitMode string `json:"gitMode"`
}

type publicationPublicMember struct {
	SHA256        string                             `json:"sha256"`
	Bytes         int64                              `json:"bytes"`
	Mode          uint32                             `json:"mode"`
	Class         string                             `json:"class"`
	ArchiveFormat string                             `json:"archiveFormat,omitempty"`
	Members       map[string]publicationPublicMember `json:"members,omitempty"`
}

type publicationSigningIntent struct {
	Artifact               string `json:"artifact"`
	Member                 string `json:"member"`
	SignedSHA256           string `json:"signedSha256"`
	FixtureSHA256          string `json:"fixtureSha256"`
	Method                 string `json:"method"`
	ProvenanceMember       string `json:"provenanceMember"`
	SourceProvenanceMember string `json:"sourceProvenanceMember"`
	FixtureID              string `json:"fixtureId"`
}

type publicationSigningAuthority struct {
	Format          string `json:"format"`
	HandoffSchema   string `json:"handoffSchema"`
	PlanSchema      string `json:"planSchema"`
	Handoff         string `json:"handoff"`
	Plan            string `json:"plan"`
	Request         string `json:"request"`
	Intent          string `json:"intent"`
	Result          string `json:"result"`
	Binding         string `json:"binding"`
	PreDispatchRuns string `json:"preDispatchRuns"`
	BindingRuns     string `json:"bindingRuns"`
}

type publicationSealedProof struct {
	Protocol                 string `json:"protocol"`
	Member                   string `json:"member"`
	PublicVerificationSchema string `json:"publicVerificationSchema"`
	PublicVerification       string `json:"publicVerification"`
	EnvelopeBinding          string `json:"envelopeBinding,omitempty"`
	PublicStdout             string `json:"publicStdout,omitempty"`
	PublicStderr             string `json:"publicStderr,omitempty"`
	ExpectedContext          string `json:"expectedContext,omitempty"`
}

type publicationSanitizerObservation struct {
	Checkout      ciCheckoutObservation `json:"checkout"`
	SuiteReport   ciMemberReference     `json:"suiteReport"`
	SuiteStep     int64                 `json:"suiteStep"`
	SuiteStepName string                `json:"suiteStepName"`
}

type publicationSecurityExpected struct {
	Schema             string                                        `json:"schema"`
	ManifestSHA256     string                                        `json:"manifestSha256"`
	Bindings           finalProvenanceExpected                       `json:"bindings"`
	SourceInventories  map[string]string                             `json:"sourceInventories"`
	KnownRawInventory  string                                        `json:"knownRawInventory"`
	KnownRawCount      int                                           `json:"knownRawCount"`
	ArtifactLists      map[string]string                             `json:"artifactLists"`
	PublicArchives     map[string]map[string]publicationPublicMember `json:"publicArchives"`
	SigningIntents     []publicationSigningIntent                    `json:"signingIntents"`
	SigningAuthorities map[string]publicationSigningAuthority        `json:"signingAuthorities"`
	SealedProofs       map[string]publicationSealedProof             `json:"sealedProofs"`
	Sanitizer          publicationSanitizerObservation               `json:"sanitizer"`
}

type publicationSecurityManifest struct {
	Schema   string                  `json:"schema"`
	Bindings finalProvenanceManifest `json:"bindings"`
}

type publicationRawInventory struct {
	Schema string                `json:"schema"`
	Files  []finalProvenanceFile `json:"files"`
}

type publicationSourceInventory struct {
	Schema    string                             `json:"schema"`
	SourceSHA string                             `json:"sourceSha"`
	Files     map[string]publicationSourceMember `json:"files"`
}

type publicationSecurityObservation struct {
	Schema             string          `json:"schema"`
	ImplementationSHA  string          `json:"implementationSha"`
	ExpectedSHA256     string          `json:"expectedSha256"`
	ManifestSHA256     string          `json:"manifestSha256"`
	StartedUTC         string          `json:"startedUtc"`
	FinishedUTC        string          `json:"finishedUtc"`
	ElapsedSeconds     float64         `json:"elapsedSeconds"`
	InputsCompletedUTC string          `json:"inputsCompletedUtc"`
	SourceFiles        int             `json:"sourceFiles"`
	PublicMembers      int             `json:"publicMembers"`
	KnownRawCount      int             `json:"knownRawCount"`
	SanitizerTests     []string        `json:"sanitizerTests"`
	RP43               *EvidenceRecord `json:"rp43Original,omitempty"`
	Status             string          `json:"status"`
	Scope              string          `json:"scope"`
	ClosureComplete    bool            `json:"closureComplete"`
	Failure            string          `json:"failure,omitempty"`
}

var publicationSanitizerTests = []string{
	"TestSanitizeTextRedactsPersonalValues",
	"TestSanitizeTextPreservesSystemConstants",
	"TestNormalizeValueShapes",
	"TestAnalyzeClassificationsAndEvidence",
}

func publicationSecurityEnvironment() (publicationSecurityConfig, bool, error) {
	values := []string{os.Getenv("CHERRI_CI04_EXPECTED"), os.Getenv("CHERRI_CI04_EXPECTED_SHA256"), os.Getenv("CHERRI_CI04_MANIFEST")}
	count := 0
	for _, value := range values {
		if value != "" {
			count++
		}
	}
	if count == 0 {
		return publicationSecurityConfig{}, false, nil
	}
	if count != 3 || !finalProvenanceHex(values[1], 64) {
		return publicationSecurityConfig{}, true, fmt.Errorf("CI04_INCOMPLETE_CALLER_CONTEXT")
	}
	return publicationSecurityConfig{values[0], values[1], values[2]}, true, nil
}

func publicationSecurityLoad(config publicationSecurityConfig, output string) (*finalProvenanceInputs, publicationSecurityExpected, error) {
	var expected publicationSecurityExpected
	var manifest publicationSecurityManifest
	if err := finalProvenanceReadContext(config.ExpectedPath, config.ExpectedSHA256, &expected); err != nil {
		return nil, expected, err
	}
	if expected.Schema != "cherri-ci04-expected/1" || !finalProvenanceHex(expected.ManifestSHA256, 64) {
		return nil, expected, fmt.Errorf("CI04_EXPECTED_SCHEMA")
	}
	if err := finalProvenanceReadContext(config.ManifestPath, expected.ManifestSHA256, &manifest); err != nil {
		return nil, expected, err
	}
	if manifest.Schema != "cherri-ci04-locators/1" || expected.Bindings.Ledger != nil || len(manifest.Bindings.Ledger) != 0 || len(manifest.Bindings.Limitations) != 0 || manifest.Bindings.ClosureComplete {
		return nil, expected, fmt.Errorf("CI04_MANIFEST_OR_CIRCULAR_LEDGER")
	}
	in := &finalProvenanceInputs{Expected: expected.Bindings, Manifest: manifest.Bindings, JSON: map[string]map[string]any{}, Bytes: map[string][]byte{}}
	if err := finalProvenanceLoadBindings(in, output); err != nil {
		return nil, expected, err
	}
	return in, expected, nil
}

func publicationKnownRaw(in *finalProvenanceInputs, expected publicationSecurityExpected) (map[string]int64, error) {
	if expected.KnownRawCount <= 0 || expected.KnownRawCount > 10000 {
		return nil, finalProvenancePending("CI04_INDEPENDENT_KNOWN_RAW_INVENTORY_REQUIRED")
	}
	data, err := ciIdentityReadOriginal(in, expected.KnownRawInventory, 2*1024*1024)
	if err != nil {
		return nil, err
	}
	var inventory publicationRawInventory
	if err = decodeEditorJSON(data, &inventory); err != nil || inventory.Schema != "cherri-known-raw-digests/1" || len(inventory.Files) != expected.KnownRawCount {
		return nil, fmt.Errorf("CI04_KNOWN_RAW_SCHEMA")
	}
	result := map[string]int64{}
	for _, file := range inventory.Files {
		if !finalProvenanceHex(file.SHA256, 64) || file.Bytes <= 0 {
			return nil, fmt.Errorf("CI04_KNOWN_RAW_DIGEST")
		}
		if _, duplicate := result[file.SHA256]; duplicate {
			return nil, fmt.Errorf("CI04_KNOWN_RAW_DUPLICATE")
		}
		result[file.SHA256] = file.Bytes
	}
	return result, nil
}

func publicationGitEnvironment() []string {
	var result []string
	for _, key := range []string{"PATH", "HOME", "LANG", "LC_ALL", "TMPDIR"} {
		if value, exists := os.LookupEnv(key); exists {
			result = append(result, key+"="+value)
		}
	}
	return append(result, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0", "GIT_NO_LAZY_FETCH=1")
}

type publicationBlobDigest struct {
	SHA256 string
	Bytes  int64
}

func publicationRejectPartialSource(ctx context.Context, root string) error {
	// Config-only preflight precedes every object-reading Git operation.
	partial := exec.CommandContext(ctx, "git", "-C", root, "config", "--local", "--get-regexp", `(^extensions\.partialclone$|^remote\..*\.promisor$)`)
	partial.Env = publicationGitEnvironment()
	partial.Stdin = nil
	partial.WaitDelay = 2 * time.Second
	partialOutput, partialErr := partial.Output()
	var exit *exec.ExitError
	if partialErr == nil || len(partialOutput) != 0 || !errors.As(partialErr, &exit) || exit.ExitCode() != 1 {
		return fmt.Errorf("CI04_PARTIAL_SOURCE_UNSUPPORTED")
	}
	return nil
}

// Read Git objects once as bounded hashes. No raw corpus locator is opened and
// no source/content/private filename is written to the observation or stdout.
func publicationHashGitBlobs(parent context.Context, root string, objects []string) (map[string]publicationBlobDigest, error) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "-C", root, "cat-file", "--batch")
	command.Env = publicationGitEnvironment()
	command.Stdin = strings.NewReader(strings.Join(objects, "\n") + "\n")
	command.WaitDelay = 2 * time.Second
	stdout, err := command.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("CI04_GIT_OBJECT_PIPE")
	}
	var stderr loopBoundedOutput
	command.Stderr = &stderr
	if err = command.Start(); err != nil {
		return nil, fmt.Errorf("CI04_GIT_OBJECT_START")
	}
	reader := bufio.NewReaderSize(stdout, 4096)
	result := map[string]publicationBlobDigest{}
	var total int64
	var parseErr error
	for _, object := range objects {
		headerBytes, readErr := reader.ReadSlice('\n')
		header := string(headerBytes)
		fields := strings.Fields(header)
		if readErr != nil || len(header) > 200 || len(fields) != 3 || fields[0] != object || fields[1] != "blob" {
			parseErr = fmt.Errorf("CI04_GIT_OBJECT_HEADER")
			break
		}
		size, sizeErr := strconv.ParseInt(fields[2], 10, 64)
		if sizeErr != nil || size < 0 || size > 128*1024*1024 || size > 512*1024*1024-total {
			parseErr = fmt.Errorf("CI04_SOURCE_BYTE_LIMIT")
			break
		}
		total += size
		hash := sha256.New()
		if _, err = io.CopyN(hash, reader, size); err != nil {
			parseErr = fmt.Errorf("CI04_GIT_OBJECT_SHORT_READ")
			break
		}
		end, readErr := reader.ReadByte()
		if readErr != nil || end != '\n' {
			parseErr = fmt.Errorf("CI04_GIT_OBJECT_TRAILER")
			break
		}
		result[object] = publicationBlobDigest{hex.EncodeToString(hash.Sum(nil)), size}
	}
	if parseErr != nil {
		cancel()
	}
	if parseErr == nil {
		if _, err = reader.ReadByte(); err != io.EOF {
			parseErr = fmt.Errorf("CI04_GIT_OBJECT_EXTRA_OUTPUT")
			cancel()
		}
	}
	waitErr := command.Wait()
	if parseErr != nil {
		return nil, parseErr
	}
	if waitErr != nil || len(stderr.Bytes()) != 0 {
		return nil, fmt.Errorf("CI04_GIT_OBJECT_PROCESS")
	}
	return result, nil
}

func publicationValidateSource(ctx context.Context, in *finalProvenanceInputs, expected publicationSecurityExpected, raw map[string]int64) (int, error) {
	if len(expected.SourceInventories) != 3 {
		return 0, finalProvenancePending("CI04_INDEPENDENT_SOURCE_INVENTORY_REQUIRED")
	}
	total := 0
	for _, role := range []string{"implementation", "compiler", "docs"} {
		source := in.Expected.Sources[role]
		inventoryID, exists := expected.SourceInventories[role]
		if !exists {
			return 0, fmt.Errorf("CI04_SOURCE_INVENTORY")
		}
		data, err := ciIdentityReadOriginal(in, inventoryID, 4*1024*1024)
		if err != nil {
			return 0, err
		}
		var document publicationSourceInventory
		if err = decodeEditorJSON(data, &document); err != nil || document.Schema != "cherri-approved-public-source/1" || document.SourceSHA != source.SHA || len(document.Files) == 0 || len(document.Files) > 20000 {
			return 0, fmt.Errorf("CI04_SOURCE_INVENTORY_SCHEMA")
		}
		inventory := document.Files
		root := in.Manifest.SourceDirectories[role]
		// A read-only identity intake must not cause a promisor object fetch.
		if err = publicationRejectPartialSource(ctx, root); err != nil {
			return 0, err
		}
		tree, _, err := finalProvenanceGit(ctx, root, "ls-tree", "-r", "-z", source.SHA)
		if err != nil {
			return 0, err
		}
		seen, objects := map[string]bool{}, map[string]bool{}
		for _, line := range bytes.Split(tree, []byte{0}) {
			if len(line) == 0 {
				continue
			}
			pair := bytes.SplitN(line, []byte{'\t'}, 2)
			if len(pair) != 2 {
				return 0, fmt.Errorf("CI04_SOURCE_TREE")
			}
			fields := strings.Fields(string(pair[0]))
			name := string(pair[1])
			pin, ok := inventory[name]
			if len(fields) != 3 || fields[1] != "blob" || !ok || seen[name] || pin.GitMode != fields[0] || pin.GitBlob != fields[2] ||
				!finalProvenanceHex(pin.SHA256, 64) || !finalProvenanceHex(pin.GitBlob, 40) || pin.Bytes < 0 || !ciIdentitySafeMember(name) {
				return 0, fmt.Errorf("CI04_SOURCE_NOT_IN_APPROVED_INVENTORY")
			}
			seen[name], objects[fields[2]] = true, true
		}
		if len(seen) != len(inventory) {
			return 0, fmt.Errorf("CI04_INCOMPLETE_SOURCE_INVENTORY")
		}
		names := make([]string, 0, len(objects))
		for object := range objects {
			names = append(names, object)
		}
		sort.Strings(names)
		observed, err := publicationHashGitBlobs(ctx, root, names)
		if err != nil {
			return 0, err
		}
		for _, pin := range inventory {
			digest := observed[pin.GitBlob]
			if digest.SHA256 != pin.SHA256 || digest.Bytes != pin.Bytes {
				return 0, fmt.Errorf("CI04_APPROVED_SOURCE_BYTES_MISMATCH")
			}
			if raw[digest.SHA256] == digest.Bytes && digest.Bytes > 0 {
				return 0, fmt.Errorf("CI04_KNOWN_RAW_CORPUS_IN_SOURCE")
			}
		}
		total += len(inventory)
	}
	return total, nil
}

func publicationValidateArtifactInventory(in *finalProvenanceInputs, expected publicationSecurityExpected) error {
	if len(expected.ArtifactLists) == 0 || len(expected.ArtifactLists) > 32 {
		return finalProvenancePending("CI04_COMPLETE_PUBLIC_RUN_INVENTORY_REQUIRED")
	}
	count := 0
	for runKey, originalID := range expected.ArtifactLists {
		run, ok := in.Expected.Runs[runKey]
		if !ok || run.Historical || run.Status != "completed" {
			return fmt.Errorf("CI04_PUBLIC_RUN_IDENTITY")
		}
		object, err := in.object(originalID)
		if err != nil {
			return err
		}
		rows, ok := object["artifacts"].([]any)
		n, valid := finalProvenanceNumber(object, "total_count")
		if !ok || !valid || n != int64(len(rows)) || n < 0 || n > 100 {
			return fmt.Errorf("CI04_PUBLIC_ARTIFACT_LIST_INCOMPLETE")
		}
		actual := map[int64]bool{}
		for _, value := range rows {
			row, ok := value.(map[string]any)
			if !ok {
				return fmt.Errorf("CI04_PUBLIC_ARTIFACT_METADATA")
			}
			id, ok := finalProvenanceNumber(row, "id")
			if !ok || id <= 0 || actual[id] {
				return fmt.Errorf("CI04_PUBLIC_ARTIFACT_DUPLICATE")
			}
			actual[id] = true
		}
		declared := map[int64]bool{}
		for id, artifact := range in.Expected.Artifacts {
			if artifact.Availability != "github" || artifact.Run != runKey {
				continue
			}
			if artifact.Historical || artifact.Metadata != originalID || declared[artifact.ID] {
				return fmt.Errorf("CI04_PUBLIC_ARTIFACT_BINDING")
			}
			if _, exists := expected.PublicArchives[id]; !exists {
				return finalProvenancePending("CI04_UNREVIEWED_PUBLIC_ARCHIVE")
			}
			declared[artifact.ID] = true
			count++
		}
		if !reflect.DeepEqual(actual, declared) {
			return fmt.Errorf("CI04_PUBLISHED_ARTIFACT_OMITTED")
		}
	}
	if count != len(expected.PublicArchives) {
		return fmt.Errorf("CI04_PUBLIC_ARCHIVE_WITHOUT_AUTHENTICATED_RUN")
	}
	for key, run := range in.Expected.Runs {
		if !run.Historical && expected.ArtifactLists[key] == "" {
			return fmt.Errorf("CI04_CURRENT_RUN_PUBLICATION_LIST_OMITTED")
		}
	}
	for _, artifact := range in.Expected.Artifacts {
		if artifact.Availability == "github" && expected.ArtifactLists[artifact.Run] == "" {
			return fmt.Errorf("CI04_PUBLISHED_RUN_OMITTED")
		}
	}
	return nil
}

func publicationInspectMembers(data []byte, inventory map[string]publicationPublicMember, raw map[string]int64, depth int, budget *ciExpandedBudget) (map[string][]byte, int, error) {
	return publicationInspectContainer(data, inventory, raw, depth, budget, "zip")
}

// The already-published Pages brand-kit ZIP has one real directory entry.
// CI04 admits only explicitly inventoried zero-byte directories; CI03's
// regular-only ZIP contract is unchanged. No member is extracted or executed.
func publicationZIPArchive(data []byte, inventory map[string]publicationPublicMember, budget *ciExpandedBudget) (map[string][]byte, error) {
	if len(data) == 0 || len(data) > ciIdentityArchiveLimit || len(inventory) == 0 || len(inventory) > 4096 {
		return nil, fmt.Errorf("CI04_ZIP_LIMIT")
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil || len(reader.File) != len(inventory) {
		return nil, fmt.Errorf("CI04_ZIP_INVENTORY")
	}
	result, canonical := map[string][]byte{}, map[string]bool{}
	for _, file := range reader.File {
		pin, exists := inventory[file.Name]
		_, duplicate := result[file.Name]
		directory := file.Mode().IsDir()
		name := file.Name
		if directory {
			name = strings.TrimSuffix(name, "/")
		}
		if !exists || duplicate || canonical[name] || !ciIdentitySafeMember(name) || (!file.Mode().IsRegular() && !directory) || file.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || file.Flags&1 != 0 ||
			(file.Method != zip.Store && file.Method != zip.Deflate) || pin.Bytes < 0 || pin.Bytes > ciIdentityArchiveLimit || file.UncompressedSize64 != uint64(pin.Bytes) ||
			uint32(file.Mode().Perm()) != pin.Mode || !finalProvenanceHex(pin.SHA256, 64) {
			return nil, fmt.Errorf("CI04_ZIP_MEMBER")
		}
		if directory && (pin.Class != "reviewed-public-directory" || pin.Bytes != 0 || pin.SHA256 != ciIdentityDigest(nil)) {
			return nil, fmt.Errorf("CI04_ZIP_DIRECTORY")
		}
		if !directory && pin.Class == "reviewed-public-directory" {
			return nil, fmt.Errorf("CI04_ZIP_DIRECTORY_TYPE")
		}
		if err = budget.consume(pin.Bytes); err != nil {
			return nil, err
		}
		stream, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("CI04_ZIP_OPEN")
		}
		body, readErr := io.ReadAll(io.LimitReader(stream, pin.Bytes+1))
		closeErr := stream.Close()
		if readErr != nil || closeErr != nil || int64(len(body)) != pin.Bytes || ciIdentityDigest(body) != pin.SHA256 {
			return nil, fmt.Errorf("CI04_ZIP_DIGEST")
		}
		result[file.Name], canonical[name] = body, true
	}
	return result, nil
}

// Pages publishes an original artifact.tar inside its GitHub ZIP. Preserve its
// exact names and bytes without extraction; normalized names only detect alias
// collisions. PAX size/path handling is delegated to archive/tar, while sparse,
// links, devices, extra payloads and nonzero trailing archives are rejected.
func publicationTARArchive(data []byte, inventory map[string]publicationPublicMember, budget *ciExpandedBudget) (map[string][]byte, error) {
	if len(data) == 0 || len(data) > ciIdentityArchiveLimit || len(inventory) == 0 || len(inventory) > 4096 {
		return nil, fmt.Errorf("CI04_TAR_LIMIT")
	}
	input := bytes.NewReader(data)
	reader := tar.NewReader(input)
	result, canonical := map[string][]byte{}, map[string]bool{}
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("CI04_TAR_HEADER")
		}
		pin, exists := inventory[header.Name]
		_, duplicate := result[header.Name]
		directory := header.Typeflag == tar.TypeDir
		name := strings.TrimPrefix(strings.TrimSuffix(header.Name, "/"), "./")
		rootDirectory := directory && (header.Name == "." || header.Name == "./")
		if rootDirectory {
			name = "."
		}
		if !exists || duplicate || canonical[name] || (!rootDirectory && !ciIdentitySafeMember(name)) || header.Linkname != "" ||
			(header.Typeflag != tar.TypeReg && header.Typeflag != tar.TypeRegA && !directory) || header.Mode < 0 || header.Mode&^0o777 != 0 ||
			uint32(header.Mode) != pin.Mode || header.Size < 0 || header.Size != pin.Bytes || header.Size > ciIdentityArchiveLimit || !finalProvenanceHex(pin.SHA256, 64) {
			return nil, fmt.Errorf("CI04_TAR_MEMBER")
		}
		for key := range header.PAXRecords {
			if strings.Contains(strings.ToLower(key), "sparse") {
				return nil, fmt.Errorf("CI04_TAR_SPARSE_UNSUPPORTED")
			}
		}
		if directory && (pin.Class != "reviewed-public-directory" || pin.Bytes != 0 || pin.SHA256 != ciIdentityDigest(nil)) {
			return nil, fmt.Errorf("CI04_TAR_DIRECTORY")
		}
		if !directory && pin.Class == "reviewed-public-directory" {
			return nil, fmt.Errorf("CI04_TAR_DIRECTORY_TYPE")
		}
		if err = budget.consume(pin.Bytes); err != nil {
			return nil, err
		}
		body, err := io.ReadAll(io.LimitReader(reader, pin.Bytes+1))
		if err != nil || int64(len(body)) != pin.Bytes || ciIdentityDigest(body) != pin.SHA256 {
			return nil, fmt.Errorf("CI04_TAR_MEMBER_DIGEST")
		}
		result[header.Name], canonical[name] = body, true
	}
	if len(result) != len(inventory) {
		return nil, fmt.Errorf("CI04_TAR_INVENTORY")
	}
	trailing := data[len(data)-input.Len():]
	if len(bytes.Trim(trailing, "\x00")) != 0 {
		return nil, fmt.Errorf("CI04_TAR_TRAILING_CONTENT")
	}
	return result, nil
}

func publicationInspectContainer(data []byte, inventory map[string]publicationPublicMember, raw map[string]int64, depth int, budget *ciExpandedBudget, format string) (map[string][]byte, int, error) {
	if depth > 3 {
		return nil, 0, fmt.Errorf("CI04_PUBLIC_NESTING_LIMIT")
	}
	var members map[string][]byte
	var err error
	switch format {
	case "zip":
		members, err = publicationZIPArchive(data, inventory, budget)
	case "tar":
		members, err = publicationTARArchive(data, inventory, budget)
	default:
		return nil, 0, finalProvenancePending("CI04_PUBLIC_CONTAINER_FORMAT_UNSUPPORTED")
	}
	if err != nil {
		return nil, 0, err
	}
	count := len(members)
	for name, body := range members {
		item := inventory[name]
		if depth != 0 && (item.Class == "authorized-signed-fixture" || item.Class == "sealed-private-ciphertext") {
			return nil, 0, fmt.Errorf("CI04_NESTED_SIGNING_OR_SEALING_UNSUPPORTED")
		}
		if raw[item.SHA256] == item.Bytes && item.Bytes > 0 {
			return nil, 0, fmt.Errorf("CI04_KNOWN_RAW_CORPUS_IN_PUBLIC_ARTIFACT")
		}
		switch item.Class {
		case "reviewed-public-source", "reviewed-synthetic-fixture", "sanitized-public-report", "public-metadata", "runtime-binary", "sealed-private-ciphertext", "authorized-signed-fixture", "reviewed-public-directory":
			if len(item.Members) != 0 || item.ArchiveFormat != "" {
				return nil, 0, fmt.Errorf("CI04_NON_ARCHIVE_MEMBER_INVENTORY")
			}
			if publicationContainerBytes(name, body) {
				return nil, 0, finalProvenancePending("CI04_CONTAINER_MUST_HAVE_MEMBER_INVENTORY")
			}
		case "archive":
			_, nested, err := publicationInspectContainer(body, item.Members, raw, depth+1, budget, item.ArchiveFormat)
			if err != nil {
				return nil, 0, err
			}
			count += nested
		default:
			return nil, 0, fmt.Errorf("CI04_UNREVIEWED_PUBLIC_CLASSIFICATION")
		}
		if bytes.HasPrefix(body, []byte("AEA1")) && item.Class != "authorized-signed-fixture" {
			return nil, 0, fmt.Errorf("CI04_SIGNED_BYTES_WITHOUT_EXPLICIT_INTENT_CLASS")
		}
	}
	return members, count, nil
}

func publicationContainerBytes(name string, body []byte) bool {
	lower := strings.ToLower(name)
	for _, suffix := range []string{".zip", ".tar", ".tgz", ".tar.gz", ".tar.xz", ".tar.bz2", ".7z", ".rar"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return bytes.HasPrefix(body, []byte("PK\x03\x04")) || bytes.HasPrefix(body, []byte("PK\x05\x06")) || bytes.HasPrefix(body, []byte{0x1f, 0x8b}) ||
		(len(body) >= 262 && string(body[257:262]) == "ustar")
}

// The reviewed dispatcher retains one complete branch-filtered page before the
// POST and one at binding; it refuses pagination rather than guessing absence.
func publicationSigningRunList(in *finalProvenanceInputs, id, repository, ref string) ([]map[string]any, error) {
	if id == "" {
		return nil, finalProvenancePending("CI04_SIGNING_RUN_LIST_ORIGINAL_UNAVAILABLE")
	}
	object, err := in.object(id)
	if err != nil {
		return nil, err
	}
	total, totalOK := finalProvenanceNumber(object, "total_count")
	values, listOK := object["workflow_runs"].([]any)
	if !totalOK || !listOK || total < 0 || total > 100 || int64(len(values)) != total {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_LIST_INCOMPLETE")
	}
	seen := map[int64]bool{}
	var rows []map[string]any
	for _, value := range values {
		row, ok := value.(map[string]any)
		id, idOK := finalProvenanceNumber(row, "id")
		workflow, workflowOK := finalProvenanceNumber(row, "workflow_id")
		if !ok || !idOK || id <= 0 || seen[id] || !workflowOK || workflow <= 0 ||
			finalProvenanceString(finalProvenanceObject(row, "repository"), "full_name") != repository ||
			finalProvenanceString(row, "head_branch") != ref || !finalProvenanceHex(finalProvenanceString(row, "head_sha"), 40) {
			return nil, fmt.Errorf("CI04_SIGNING_RUN_LIST_IDENTITY")
		}
		seen[id] = true
		rows = append(rows, row)
	}
	return rows, nil
}

func publicationSigningApproval(ctx context.Context, in *finalProvenanceInputs, runKey string, authority publicationSigningAuthority) (map[string]string, error) {
	if authority.Format != "root-reviewed-native-plan/1" || authority.HandoffSchema == "" || authority.PlanSchema == "" {
		return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_FORMAT")
	}
	run, ok := in.Expected.Runs[runKey]
	source := in.Expected.Sources["implementation"]
	if !ok || run.Historical || run.SourceSHA != source.SHA || run.Event != "workflow_dispatch" || run.WorkflowPath != ".github/workflows/ios27-shortcuts-runtime-poc.yml" {
		return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_RUN")
	}
	objects := map[string]map[string]any{}
	for label, id := range map[string]string{"handoff": authority.Handoff, "plan": authority.Plan, "request": authority.Request, "intent": authority.Intent, "result": authority.Result, "binding": authority.Binding} {
		if id == "" {
			return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_ORIGINAL_UNDECLARED")
		}
		object, err := in.object(id)
		if err != nil {
			return nil, err
		}
		objects[label] = object
	}
	handoff, plan, request, intent, result, binding := objects["handoff"], objects["plan"], objects["request"], objects["intent"], objects["result"], objects["binding"]
	ref := strings.TrimPrefix(source.PublishedRef, "refs/heads/")
	if finalProvenanceString(handoff, "schema") != authority.HandoffSchema || finalProvenanceString(plan, "schema") != authority.PlanSchema ||
		finalProvenanceString(handoff, "planSha256") != in.Expected.Files[authority.Plan].SHA256 ||
		!finalProvenanceBool(handoff, "fullScriptAndPlanReviewed", true) || !finalProvenanceBool(handoff, "remoteRefVerified", true) ||
		!finalProvenanceBool(handoff, "singlePostPerWorkflow", true) || !finalProvenanceBool(handoff, "nativeRerunsOrCancellation", false) {
		return nil, fmt.Errorf("CI04_SIGNING_ROOT_REVIEW_CHAIN")
	}
	for _, object := range []map[string]any{handoff, plan, intent, binding} {
		if finalProvenanceString(object, "sourceCommit") != source.SHA {
			return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_SOURCE")
		}
	}
	for _, object := range []map[string]any{handoff, plan} {
		if finalProvenanceString(object, "sourceTree") != source.Tree || finalProvenanceString(object, "ref") != ref {
			return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_TREE_OR_REF")
		}
	}
	cases, valid := finalProvenanceStrings(handoff["allowedCases"])
	allowed := 0
	for _, name := range cases {
		if name == "ios27-runtime-poc" {
			allowed++
		}
	}
	if !valid || allowed != 1 {
		return nil, fmt.Errorf("CI04_SIGNING_CASE_NOT_AUTHORIZED")
	}
	inputs := finalProvenanceObject(request, "inputs")
	if len(request) != 2 || len(inputs) != 1 || !finalProvenanceBool(inputs, "allow_public_fixture_hubsign", true) || finalProvenanceString(request, "ref") != ref {
		return nil, fmt.Errorf("CI04_SIGNING_NOT_EXPLICITLY_REQUESTED")
	}
	poc := finalProvenanceObject(plan, "runtimePoc")
	payload := finalProvenanceObject(poc, "payload")
	workflowID, workflowOK := finalProvenanceNumber(poc, "workflowId")
	if !reflect.DeepEqual(payload, request) || !workflowOK || workflowID != run.WorkflowID || finalProvenanceString(poc, "workflowPath") != run.WorkflowPath {
		return nil, fmt.Errorf("CI04_SIGNING_PLAN_REQUEST_MISMATCH")
	}
	workflow, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", source.SHA+":"+run.WorkflowPath)
	if err != nil || finalProvenanceString(poc, "workflowSha256") != ciIdentityDigest(workflow) {
		return nil, fmt.Errorf("CI04_SIGNING_WORKFLOW_SOURCE")
	}
	intentWorkflow, intentOK := finalProvenanceNumber(intent, "workflowId")
	if !intentOK || intentWorkflow != run.WorkflowID || finalProvenanceString(intent, "case") != "ios27-runtime-poc" || finalProvenanceString(intent, "ref") != ref || !finalProvenanceBool(intent, "onePostOnly", true) {
		return nil, fmt.Errorf("CI04_SIGNING_DISPATCH_INTENT")
	}
	posts, postsOK := finalProvenanceNumber(result, "postCount")
	code, codeOK := finalProvenanceNumber(result, "returncode")
	if !postsOK || posts != 1 || !codeOK || code != 0 || !finalProvenanceBool(result, "accepted204", true) {
		return nil, fmt.Errorf("CI04_SIGNING_DISPATCH_RESULT")
	}
	id, idOK := finalProvenanceNumber(binding, "runId")
	attempt, attemptOK := finalProvenanceNumber(binding, "attempt")
	bindingWorkflow, bindingOK := finalProvenanceNumber(binding, "workflowId")
	dispatches, dispatchOK := finalProvenanceNumber(binding, "dispatchCount")
	if !idOK || id != run.ID || !attemptOK || attempt != int64(run.Attempt) || !bindingOK || bindingWorkflow != run.WorkflowID || !dispatchOK || dispatches != 1 ||
		finalProvenanceString(binding, "case") != "ios27-runtime-poc" || finalProvenanceString(binding, "event") != run.Event || finalProvenanceString(binding, "sourceTree") != source.Tree ||
		finalProvenanceString(binding, "headBranch") != ref || finalProvenanceString(binding, "workflowPath") != run.WorkflowPath ||
		finalProvenanceString(binding, "requestSha256") != in.Expected.Files[authority.Request].SHA256 || !reflect.DeepEqual(finalProvenanceObject(binding, "inputs"), inputs) {
		return nil, fmt.Errorf("CI04_SIGNING_ACTUAL_RUN_BINDING")
	}
	var previous time.Time
	var authorityTimes []time.Time
	for _, value := range []string{finalProvenanceString(handoff, "recordedUTC"), finalProvenanceString(intent, "beforePostUtc"), finalProvenanceString(result, "recordedUtc"), finalProvenanceString(binding, "boundUtc")} {
		timestamp, err := time.Parse(time.RFC3339Nano, value)
		if err != nil || timestamp.Before(previous) {
			return nil, fmt.Errorf("CI04_SIGNING_AUTHORITY_CHRONOLOGY")
		}
		previous = timestamp
		authorityTimes = append(authorityTimes, timestamp)
	}
	actualRun, err := in.object(run.Original)
	if err != nil {
		return nil, err
	}
	if finalProvenanceString(actualRun, "head_branch") != ref {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_REF_MISMATCH")
	}
	before, err := publicationSigningRunList(in, authority.PreDispatchRuns, run.Repository, ref)
	if err != nil {
		return nil, err
	}
	priorIDs, priorOK := intent["priorRunIds"].([]any)
	if !priorOK || len(priorIDs) != len(before) {
		return nil, fmt.Errorf("CI04_SIGNING_PRIOR_RUN_IDS_MISMATCH")
	}
	prior := map[int64]bool{}
	for index, row := range before {
		id, _ := finalProvenanceNumber(row, "id")
		intentID, ok := finalProvenanceNumber(map[string]any{"id": priorIDs[index]}, "id")
		if !ok || intentID != id {
			return nil, fmt.Errorf("CI04_SIGNING_PRIOR_RUN_IDS_MISMATCH")
		}
		prior[id] = true
		if id == run.ID {
			return nil, fmt.Errorf("CI04_SIGNING_RUN_ALREADY_PRESENT")
		}
		workflow, _ := finalProvenanceNumber(row, "workflow_id")
		if workflow == run.WorkflowID {
			return nil, fmt.Errorf("CI04_SIGNING_PREEXISTING_WORKFLOW")
		}
	}
	bound, err := publicationSigningRunList(in, authority.BindingRuns, run.Repository, ref)
	if err != nil {
		return nil, err
	}
	var matching []map[string]any
	for _, row := range bound {
		id, _ := finalProvenanceNumber(row, "id")
		workflow, _ := finalProvenanceNumber(row, "workflow_id")
		if !prior[id] && workflow == run.WorkflowID && finalProvenanceString(row, "head_sha") == source.SHA {
			matching = append(matching, row)
		}
	}
	if len(matching) != 1 {
		return nil, fmt.Errorf("CI04_SIGNING_UNIQUE_NEW_RUN")
	}
	boundID, _ := finalProvenanceNumber(matching[0], "id")
	boundAttempt, attemptOK := finalProvenanceNumber(matching[0], "run_attempt")
	if boundID != run.ID || !attemptOK || boundAttempt != int64(run.Attempt) ||
		finalProvenanceString(matching[0], "event") != run.Event || finalProvenanceString(matching[0], "path") != run.WorkflowPath {
		return nil, fmt.Errorf("CI04_SIGNING_UNIQUE_NEW_RUN")
	}
	created := finalProvenanceString(actualRun, "created_at")
	started := finalProvenanceString(actualRun, "run_started_at")
	if finalProvenanceString(binding, "createdAt") != created || finalProvenanceString(matching[0], "created_at") != created {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_CREATED_AT_MISMATCH")
	}
	createdFrom, _, createdErr := ciIdentityTimeRange(created, created)
	_, startedUntil, startedErr := ciIdentityTimeRange(started, started)
	if createdErr != nil || startedErr != nil {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_TIME")
	}
	// Preserve owned_ci.py's exact parsed created_at >= beforePost - 5s rule,
	// together with its original prior-ID exclusion and unique postlist join.
	// Creation may precede accepted204; a queued run may start after binding.
	if createdFrom.Before(authorityTimes[1].Add(-5 * time.Second)) {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_CREATION_OUTSIDE_DISPATCH")
	}
	if !startedUntil.After(createdFrom) {
		return nil, fmt.Errorf("CI04_SIGNING_RUN_START_PRECEDES_CREATION")
	}
	reviewed := finalProvenanceObject(poc, "reviewedPublicFixtureHashes")
	count, countOK := finalProvenanceNumber(poc, "publicFixtureCount")
	names := []string{"CherriDictionaryRuntimePOC", "CherriLegacyIfPOC", "CherriMenuRuntimePOC", "CherriRuntimePOC", "CherriS01F01CoveragePOC"}
	if !countOK || count != int64(len(names)) || len(reviewed) != len(names) {
		return nil, fmt.Errorf("CI04_SIGNING_PUBLIC_FIXTURE_INVENTORY")
	}
	approved := map[string]string{}
	for _, name := range names {
		digest := finalProvenanceString(reviewed, name)
		body, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", source.SHA+":tests/runtime_poc/"+name+".cherri")
		if err != nil || !finalProvenanceHex(digest, 64) || ciIdentityDigest(body) != digest {
			return nil, fmt.Errorf("CI04_SIGNING_PUBLIC_SOURCE_MISMATCH")
		}
		approved[name] = digest
	}
	return approved, nil
}

func publicationValidateSigning(ctx context.Context, in *finalProvenanceInputs, expected publicationSecurityExpected, archives map[string]map[string][]byte) error {
	// The expected-context digest is supplied independently by the caller. The
	// candidate locators cannot grant permission by adding an intent to themselves.
	wants := map[string]publicationSigningIntent{}
	for _, intent := range expected.SigningIntents {
		key := intent.Artifact + "\x00" + intent.Member
		if _, duplicate := wants[key]; duplicate {
			return fmt.Errorf("CI04_DUPLICATE_SIGNING_INTENT")
		}
		if !finalProvenanceHex(intent.SignedSHA256, 64) || !finalProvenanceHex(intent.FixtureSHA256, 64) ||
			(intent.Method != "native" && intent.Method != "hubsign") || intent.FixtureID == "" || intent.ProvenanceMember == "" || intent.SourceProvenanceMember == "" {
			return fmt.Errorf("CI04_SIGNING_INTENT")
		}
		wants[key] = intent
	}
	seen := map[string]bool{}
	approved := map[string]map[string]string{}
	for runKey, authority := range expected.SigningAuthorities {
		rows, err := publicationSigningApproval(ctx, in, runKey, authority)
		if err != nil {
			return err
		}
		approved[runKey] = rows
	}
	usedAuthorities := map[string]map[string]bool{}
	for artifactID, inventory := range expected.PublicArchives {
		artifact := in.Expected.Artifacts[artifactID]
		run := in.Expected.Runs[artifact.Run]
		for name, pin := range inventory {
			if pin.Class != "authorized-signed-fixture" {
				continue
			}
			key := artifactID + "\x00" + name
			intent, exists := wants[key]
			body := archives[artifactID][name]
			if !exists || seen[key] || ciIdentityDigest(body) != intent.SignedSHA256 || len(body) < 16 || !bytes.HasPrefix(body, []byte("AEA1")) {
				return fmt.Errorf("CI04_UNREQUESTED_OR_FALSE_SIGNED_ARTIFACT")
			}
			if approved[artifact.Run] == nil {
				return finalProvenancePending("CI04_INDEPENDENT_SIGNING_AUTHORIZATION_UNAVAILABLE")
			}
			if approved[artifact.Run][intent.FixtureID] != intent.FixtureSHA256 {
				return fmt.Errorf("CI04_SIGNED_FIXTURE_NOT_OWNER_REVIEWED")
			}
			if usedAuthorities[artifact.Run] == nil {
				usedAuthorities[artifact.Run] = map[string]bool{}
			}
			usedAuthorities[artifact.Run][intent.FixtureID] = true
			provenance, err := decodeNativeEncodingObject(archives[artifactID][intent.ProvenanceMember])
			if err != nil || finalProvenanceString(provenance, "schemaVersion") != "1" || finalProvenanceString(provenance, "implementationSha") != run.SourceSHA || finalProvenanceString(provenance, "runId") != strconv.FormatInt(run.ID, 10) ||
				finalProvenanceString(provenance, "runAttempt") != strconv.Itoa(run.Attempt) {
				return fmt.Errorf("CI04_SIGNING_PROVENANCE_SOURCE")
			}
			if len(finalProvenanceArray(provenance, "fixtures")) != len(approved[artifact.Run]) {
				return fmt.Errorf("CI04_SIGNING_PROVENANCE_INVENTORY")
			}
			sources, err := decodeNativeEncodingObject(archives[artifactID][intent.SourceProvenanceMember])
			if err != nil || finalProvenanceString(sources, "schemaVersion") != "1" || finalProvenanceString(sources, "implementationSha") != run.SourceSHA ||
				finalProvenanceString(sources, "runId") != strconv.FormatInt(run.ID, 10) || finalProvenanceString(sources, "runAttempt") != strconv.Itoa(run.Attempt) || !finalProvenanceBool(sources, "syntheticSourcesVerified", true) {
				return fmt.Errorf("CI04_SIGNING_SYNTHETIC_SOURCE_PROVENANCE")
			}
			sourceRows := finalProvenanceArray(sources, "sources")
			if len(sourceRows) != len(approved[artifact.Run]) {
				return fmt.Errorf("CI04_SIGNING_SOURCE_ROW_INVENTORY")
			}
			sourceSeen := map[string]bool{}
			for _, value := range sourceRows {
				row, ok := value.(map[string]any)
				if !ok {
					return fmt.Errorf("CI04_SIGNING_SOURCE_ROW")
				}
				id := finalProvenanceString(row, "fixtureId")
				if sourceSeen[id] || approved[artifact.Run][id] == "" || finalProvenanceString(row, "sourceSha256") != approved[artifact.Run][id] || finalProvenanceString(row, "sourcePath") != "tests/runtime_poc/"+id+".cherri" {
					return fmt.Errorf("CI04_SIGNING_SOURCE_ROW_MISMATCH")
				}
				sourceSeen[id] = true
			}
			matches := 0
			for _, rawRow := range finalProvenanceArray(provenance, "fixtures") {
				row, ok := rawRow.(map[string]any)
				if !ok {
					return fmt.Errorf("CI04_SIGNING_PROVENANCE_ROW")
				}
				if finalProvenanceString(row, "fixtureId") == intent.FixtureID {
					matches++
					if finalProvenanceString(row, "signedArtifact") != name || finalProvenanceString(row, "sourceSha256") != intent.FixtureSHA256 ||
						finalProvenanceString(row, "signedSha256") != intent.SignedSHA256 || finalProvenanceString(row, "signingMethod") != intent.Method || !finalProvenanceBool(row, "signedMagicVerified", true) ||
						!finalProvenanceHex(finalProvenanceString(row, "unsignedSha256"), 64) {
						return fmt.Errorf("CI04_SIGNING_EXACT_BYTES")
					}
					if intent.Method == "hubsign" && (!finalProvenanceHex(finalProvenanceString(row, "exactFileSigningReceiptSha256"), 64) || finalProvenanceString(row, "signingVerificationTier") != "AEA1_HEADER_PASS") {
						return fmt.Errorf("CI04_SIGNING_EXACT_FILE_RECEIPT")
					}
				}
			}
			if matches != 1 {
				return fmt.Errorf("CI04_SIGNING_PROVENANCE_INVENTORY")
			}
			seen[key] = true
		}
	}
	if len(seen) != len(wants) {
		return fmt.Errorf("CI04_UNUSED_OR_UNOBSERVED_SIGNING_INTENT")
	}
	for runKey, fixtures := range approved {
		if len(usedAuthorities[runKey]) != len(fixtures) {
			return fmt.Errorf("CI04_SIGNING_AUTHORIZED_INVENTORY_NOT_ACCOUNTED")
		}
	}
	return nil
}

func publicationHelperHashes(ctx context.Context, in *finalProvenanceInputs, files map[string]any, required []string, records bool) error {
	if len(files) != len(required) {
		return fmt.Errorf("CI04_PUBLIC_VALIDATOR_SOURCE_INVENTORY")
	}
	root, source := in.Manifest.SourceDirectories["implementation"], in.Expected.Sources["implementation"].SHA
	for _, name := range required {
		value, exists := files[name]
		if !exists {
			return fmt.Errorf("CI04_PUBLIC_VALIDATOR_SOURCE_MISSING")
		}
		digest, ok := value.(string)
		var row map[string]any
		if records {
			row, ok = value.(map[string]any)
			digest = finalProvenanceString(row, "sha256")
		}
		body, _, err := finalProvenanceGit(ctx, root, "show", source+":"+name)
		if err != nil || !ok || !finalProvenanceHex(digest, 64) || ciIdentityDigest(body) != digest {
			return fmt.Errorf("CI04_PUBLIC_VALIDATOR_SOURCE_CHANGED")
		}
		if records {
			size, valid := finalProvenanceNumber(row, "bytes")
			tree, _, err := finalProvenanceGit(ctx, root, "ls-tree", source, "--", name)
			fields := strings.Fields(string(tree))
			if !valid || size != int64(len(body)) || err != nil || len(fields) != 4 || fields[1] != "blob" || fields[3] != name || fields[0] != finalProvenanceString(row, "mode") || fields[2] != finalProvenanceString(row, "blob") {
				return fmt.Errorf("CI04_PUBLIC_VALIDATOR_GIT_RECORD")
			}
		}
	}
	return nil
}

func publicationValidateSealed(ctx context.Context, in *finalProvenanceInputs, expected publicationSecurityExpected, archives map[string]map[string][]byte) error {
	seen := map[string]bool{}
	for artifactID, inventory := range expected.PublicArchives {
		for member, item := range inventory {
			if item.Class != "sealed-private-ciphertext" {
				continue
			}
			proof, exists := expected.SealedProofs[artifactID]
			if !exists {
				return finalProvenancePending("CI04_MANDATORY_PUBLIC_SEAL_VALIDATION_UNAVAILABLE")
			}
			if proof.Member != member || seen[artifactID] {
				return fmt.Errorf("CI04_SEALED_MEMBER_INVENTORY")
			}
			seen[artifactID] = true
			artifact := in.Expected.Artifacts[artifactID]
			run := in.Expected.Runs[artifact.Run]
			original, err := in.object(proof.PublicVerification)
			if err != nil {
				return err
			}
			source := in.Expected.Sources["implementation"]
			if run.Historical || run.SourceSHA != source.SHA {
				return fmt.Errorf("CI04_HISTORICAL_SEAL_PROOF")
			}
			cipher := archives[artifactID][member]
			switch proof.Protocol {
			case "cherri-project-prerequisite-observation/3":
				if proof.EnvelopeBinding == "" {
					return finalProvenancePending("CI04_OLD23_PUBLIC_ENVELOPE_JOIN_UNAVAILABLE")
				}
				joined, err := in.object(proof.EnvelopeBinding)
				if err != nil {
					return err
				}
				joinID, joinIDOK := finalProvenanceNumber(joined, "runId")
				joinAttempt, joinAttemptOK := finalProvenanceNumber(joined, "attempt")
				joinVerification := finalProvenanceObject(joined, "verification")
				if finalProvenanceString(joined, "schema") != "cherri-windows-private-prerequisite-intake/1" || !joinIDOK || joinID != run.ID || !joinAttemptOK || joinAttempt != int64(run.Attempt) ||
					finalProvenanceString(joined, "sourceCommit") != source.SHA || finalProvenanceString(joined, "publicArtifactSha256") != in.Expected.Files[artifact.File].SHA256 ||
					finalProvenanceString(joined, "mandatoryPublicValidationSha256") != in.Expected.Files[proof.PublicVerification].SHA256 || !finalProvenanceBool(joined, "privatePlaintextOrKeyExported", false) ||
					!finalProvenanceBool(joinVerification, "plaintextOrKeyTransferred", false) || !finalProvenanceBool(joinVerification, "prerequisiteOnly", true) || !finalProvenanceBool(joinVerification, "nativeRuntimeAcceptanceClaimed", false) ||
					!finalProvenanceBool(joinVerification, "formalAcceptanceGranted", false) || finalProvenanceString(joinVerification, "ciphertextSha256") != ciIdentityDigest(cipher) {
					return fmt.Errorf("CI04_OLD23_PUBLIC_ENVELOPE_JOIN")
				}
				id, idOK := finalProvenanceNumber(original, "runId")
				attempt, attemptOK := finalProvenanceNumber(original, "attempt")
				if proof.PublicVerificationSchema == "" || finalProvenanceString(original, "schema") != proof.PublicVerificationSchema || !idOK || id != run.ID || !attemptOK || attempt != int64(run.Attempt) ||
					finalProvenanceString(original, "sourceCommit") != source.SHA || !finalProvenanceBool(original, "allPublicationByteHashesMatched", true) {
					return fmt.Errorf("CI04_QA_PUBLIC_SEAL_IDENTITY")
				}
				public := finalProvenanceObject(original, "prerequisitePublic")
				metadata := finalProvenanceObject(public, "ciphertext")
				size, sizeOK := finalProvenanceNumber(metadata, "bytes")
				members, membersOK := finalProvenanceNumber(public, "originalMembers")
				if finalProvenanceString(public, "schema") != proof.Protocol || finalProvenanceString(public, "executionScope") != "CURRENT_PREREQUISITE_CALL_ONLY" ||
					finalProvenanceString(public, "strictPublicEnvelope") != "PASS" || finalProvenanceString(public, "strictPublicValidator") != "PASS" || !finalProvenanceBool(public, "sourceAndEnvironmentMatched", true) ||
					!finalProvenanceBool(public, "formalAcceptanceGranted", false) || !membersOK || members != 23 || !sizeOK || size != int64(len(cipher)) ||
					finalProvenanceString(metadata, "sha256") != ciIdentityDigest(cipher) || !finalProvenanceHex(finalProvenanceString(public, "expectedContextSha256"), 64) {
					return fmt.Errorf("CI04_QA_PUBLIC_SEAL_BOUNDARY")
				}
				if finalProvenanceString(joinVerification, "expectedContextSha256") != finalProvenanceString(public, "expectedContextSha256") || finalProvenanceString(joinVerification, "archiveSha256") != finalProvenanceString(finalProvenanceObject(public, "plaintextArchive"), "sha256") {
					return fmt.Errorf("CI04_OLD23_PUBLIC_PRIVATE_CONTEXT_JOIN")
				}
				paths := []string{"tools/shortcuts-simulator-qa/project_prerequisite.py", "tools/shortcuts-simulator-qa/project_prerequisite_observation.py", "tools/shortcuts-simulator-qa/qa_terminal_receipts.py", "tools/shortcuts-simulator-qa/simulator_session_recovery.py"}
				if err = publicationHelperHashes(ctx, in, finalProvenanceObject(original, "sourceBoundValidationHelpers"), paths, false); err != nil {
					return err
				}
			case "cherri-ios-xcresult-summary/1":
				code, codeOK := finalProvenanceNumber(original, "exitCode")
				stdout, err := ciIdentityReadOriginal(in, proof.PublicStdout, 32*1024)
				if err != nil {
					return err
				}
				stderr, err := ciIdentityReadOriginal(in, proof.PublicStderr, 32*1024)
				if err != nil {
					return err
				}
				result, err := decodeNativeEncodingObject(stdout)
				if err != nil || !codeOK || code != 0 || len(stderr) != 0 || finalProvenanceString(original, "stdoutSha256") != ciIdentityDigest(stdout) || finalProvenanceString(original, "stderrSha256") != ciIdentityDigest(stderr) ||
					!reflect.DeepEqual(finalProvenanceObject(original, "stdout"), result) || finalProvenanceString(result, "schema") != proof.Protocol || finalProvenanceString(result, "status") != "PUBLIC_CIPHERTEXT_VERIFIED" ||
					!finalProvenanceBool(result, "privateIntakePerformed", false) || !finalProvenanceBool(result, "formalAcceptanceGranted", false) || finalProvenanceString(result, "artifactSha256") != in.Expected.Files[artifact.File].SHA256 {
					return fmt.Errorf("CI04_XCRESULT_PUBLIC_SEAL_PROCESS")
				}
				caller, err := in.object(proof.ExpectedContext)
				if err != nil {
					return err
				}
				if finalProvenanceString(original, "expectedSha256") != in.Expected.Files[proof.ExpectedContext].SHA256 || finalProvenanceString(caller, "schema") != "cherri-ios-xcresult-summary-expected/1" {
					return fmt.Errorf("CI04_XCRESULT_PUBLIC_CALLER_AUTHORITY")
				}
				context := finalProvenanceObject(caller, "context")
				id, idOK := finalProvenanceNumber(context, "runId")
				attempt, attemptOK := finalProvenanceNumber(context, "attempt")
				workflow, workflowOK := finalProvenanceNumber(caller, "workflowId")
				if !idOK || id != run.ID || !attemptOK || attempt != int64(run.Attempt) || !workflowOK || workflow != run.WorkflowID || finalProvenanceString(context, "repository") != run.Repository ||
					finalProvenanceString(context, "sourceSha") != source.SHA || finalProvenanceString(context, "workflowPath") != run.WorkflowPath || finalProvenanceString(context, "event") != run.Event {
					return fmt.Errorf("CI04_XCRESULT_PUBLIC_CONTEXT")
				}
				art := finalProvenanceObject(caller, "artifact")
				artifactID, artifactOK := finalProvenanceNumber(art, "id")
				artifactSize, sizeOK := finalProvenanceNumber(art, "bytes")
				if !artifactOK || artifactID != artifact.ID || !sizeOK || artifactSize != in.Expected.Files[artifact.File].Bytes || finalProvenanceString(art, "sha256") != in.Expected.Files[artifact.File].SHA256 {
					return fmt.Errorf("CI04_XCRESULT_PUBLIC_ARTIFACT")
				}
				sources := finalProvenanceObject(caller, "sources")
				if finalProvenanceString(sources, "sourceSha") != source.SHA || finalProvenanceString(sources, "tree") != source.Tree {
					return fmt.Errorf("CI04_XCRESULT_PUBLIC_SOURCE")
				}
				paths := []string{".github/workflows/ios-build.yml", "ios/CherriTests/CherriEditorInputTests.swift", "ios/CherriUITests/CherriEditorUITests.swift", "ios/CherriUITests/EditorStartupDeadline.swift", "ios/project.yml", "ios/scripts/collect_ios_xcresult_summary.py", "ios/scripts/test_ios_xcresult_summary.py", "ios/scripts/verify_ios_xcresult_summary.py", "tools/shortcuts-simulator-qa/project_prerequisite.py", "tools/shortcuts-simulator-qa/project_prerequisite_observation.py", "tools/shortcuts-simulator-qa/qa_terminal_receipts.py", "tools/shortcuts-simulator-qa/simulator_session_recovery.py", "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"}
				if err = publicationHelperHashes(ctx, in, finalProvenanceObject(sources, "files"), paths, true); err != nil {
					return err
				}
				absent, valid := finalProvenanceStrings(sources["absent"])
				if !valid || !reflect.DeepEqual(absent, []string{"ios/CherriApp/Views/CodeEditorView.swift"}) {
					return fmt.Errorf("CI04_XCRESULT_ABSENT_SOURCE_INVENTORY")
				}
				row, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "ls-tree", source.SHA, "--", absent[0])
				if err != nil || len(row) != 0 {
					return fmt.Errorf("CI04_XCRESULT_ABSENT_SOURCE_CHANGED")
				}
			default:
				return finalProvenancePending("CI04_PUBLIC_SEAL_PROTOCOL_NOT_SUPPORTED")
			}
		}
	}
	if len(seen) != len(expected.SealedProofs) {
		return fmt.Errorf("CI04_UNUSED_PUBLIC_SEAL_PROOF")
	}
	return nil
}

var publicationGoSummary = regexp.MustCompile(`^(ok|\?|FAIL)[\t ]+(github\.com/electrikmilk/cherri[^\t ]*)[\t ]*(.*)$`)
var publicationGoPass = regexp.MustCompile(`^--- PASS: (Test[A-Za-z0-9_]+) \([0-9.]+s\)$`)
var publicationGoRejected = regexp.MustCompile(`^--- (?:FAIL|SKIP): (Test[A-Za-z0-9_]+)(?:[/ ]|$)`)

func publicationSanitizerEvents(raw []byte, started, completed string) error {
	a, b, err := ciIdentityTimeRange(started, completed)
	if err != nil {
		return err
	}
	const target = "github.com/electrikmilk/cherri/tools/shortcut-corpus"
	active, ordinaryMarkers, targetPackages := false, 0, 0
	var block []string
	for _, line := range strings.Split(string(raw), "\n") {
		t, body, ok := ciIdentityRunnerLine(line)
		if !ok || t.Before(a) || !t.Before(b) {
			continue
		}
		if strings.HasPrefix(body, "GO_SUITE_EXECUTE ") {
			object, err := decodeNativeEncodingObject([]byte(strings.TrimPrefix(body, "GO_SUITE_EXECUTE ")))
			if err != nil {
				return fmt.Errorf("CI04_GO_PLAN_JSON")
			}
			packages, ok := finalProvenanceStrings(object["packages"])
			if !ok {
				return fmt.Errorf("CI04_GO_PLAN_PACKAGES")
			}
			active = false
			block = nil
			for _, pkg := range packages {
				if pkg == target {
					active = true
				}
			}
			if active {
				ordinaryMarkers++
				argv, ok := finalProvenanceStrings(object["argv"])
				want := append([]string{"go", "test", "-count=1", "-timeout=15m", "-parallel=2", "-v"}, packages...)
				if !ok || !reflect.DeepEqual(argv, want) || finalProvenanceString(object, "timeout") != "15m" {
					return fmt.Errorf("CI04_ORIGINAL_GO_COMMAND_CHANGED")
				}
			}
			continue
		}
		if !active {
			continue
		}
		if summary := publicationGoSummary.FindStringSubmatch(body); summary != nil {
			if summary[2] == target {
				targetPackages++
				if summary[1] != "ok" || strings.Contains(summary[3], "cached") {
					return fmt.Errorf("CI04_SANITIZER_PACKAGE_DID_NOT_PASS")
				}
				run, passed := map[string]int{}, map[string]int{}
				for _, entry := range block {
					if strings.HasPrefix(entry, "=== RUN   ") {
						run[strings.TrimPrefix(entry, "=== RUN   ")]++
					}
					if match := publicationGoPass.FindStringSubmatch(entry); match != nil {
						passed[match[1]]++
					}
					if match := publicationGoRejected.FindStringSubmatch(strings.TrimSpace(entry)); match != nil {
						for _, name := range publicationSanitizerTests {
							if match[1] == name {
								return fmt.Errorf("CI04_SANITIZER_FAILURE_OR_SKIP")
							}
						}
					}
				}
				for _, name := range publicationSanitizerTests {
					if run[name] != 1 || passed[name] != 1 {
						return fmt.Errorf("CI04_SANITIZER_ORIGINAL_CONTROL_MISSING_OR_DUPLICATE")
					}
				}
			}
			block = nil
		} else {
			block = append(block, body)
		}
	}
	if ordinaryMarkers != 1 || targetPackages != 1 {
		return finalProvenancePending("CI04_SANITIZER_ACTUAL_PACKAGE_NOT_OBSERVED")
	}
	return nil
}

func publicationValidateSanitizer(in *finalProvenanceInputs, expected publicationSecurityExpected, archives map[string]map[string][]byte) error {
	witness := expected.Sanitizer
	run, exists := in.Expected.Runs[witness.Checkout.Run]
	if !exists || run.Historical || run.SourceSHA != in.Expected.Sources["implementation"].SHA || run.WorkflowPath != ".github/workflows/build-test.yml" {
		return fmt.Errorf("CI04_SANITIZER_WORKFLOW_SOURCE")
	}
	job, checkout, err := ciIdentityJob(in, witness.Checkout)
	if err != nil {
		return err
	}
	raw, err := ciIdentityReadOriginal(in, witness.Checkout.LogFile, 64*1024*1024)
	if err != nil {
		return err
	}
	if _, err = ciIdentityCheckoutSHA(raw, finalProvenanceString(checkout, "started_at"), finalProvenanceString(checkout, "completed_at"), run.SourceSHA); err != nil {
		return err
	}
	artifact, exists := in.Expected.Artifacts[witness.SuiteReport.Artifact]
	if !exists || artifact.Run != witness.Checkout.Run {
		return fmt.Errorf("CI04_SUITE_REPORT_RUN")
	}
	reportBytes := archives[witness.SuiteReport.Artifact][witness.SuiteReport.Member]
	report, err := decodeNativeEncodingObject(reportBytes)
	before, beforeOK := report["source_status_before"].(string)
	after, afterOK := report["source_status_after"].(string)
	if err != nil || finalProvenanceString(report, "source_sha") != run.SourceSHA || !beforeOK || before != "" ||
		!afterOK || after != "" || !finalProvenanceBool(report, "all_packages_executed_once", true) {
		return fmt.Errorf("CI04_SUITE_ACTUAL_SOURCE")
	}
	var step map[string]any
	for _, rawStep := range finalProvenanceArray(job, "steps") {
		candidate, ok := rawStep.(map[string]any)
		if !ok {
			return fmt.Errorf("CI04_SUITE_STEP_SCHEMA")
		}
		number, ok := finalProvenanceNumber(candidate, "number")
		if ok && number == witness.SuiteStep {
			if step != nil {
				return fmt.Errorf("CI04_SUITE_STEP_DUPLICATE")
			}
			step = candidate
		}
	}
	if step == nil {
		return finalProvenancePending("CI04_SUITE_ORIGINAL_STEP_UNAVAILABLE")
	}
	if witness.SuiteStepName != "Full Go Test Suite" || finalProvenanceString(step, "name") != witness.SuiteStepName {
		return fmt.Errorf("CI04_SUITE_STEP_NAME_MISMATCH")
	}
	if finalProvenanceString(step, "status") != "completed" {
		return finalProvenancePending("CI04_SUITE_ORIGINAL_STEP_NOT_COMPLETED")
	}
	return publicationSanitizerEvents(raw, finalProvenanceString(step, "started_at"), finalProvenanceString(step, "completed_at"))
}

func publicationRP43(records []EvidenceRecord, source string) (*EvidenceRecord, error) {
	var selected *EvidenceRecord
	for _, record := range records {
		if !reflect.DeepEqual(record.Requirements, []string{"RP43"}) || record.Tier != "security" {
			continue
		}
		if selected != nil {
			return nil, fmt.Errorf("CI04_DUPLICATE_RP43_COMPONENT")
		}
		copy := record
		selected = &copy
	}
	if selected == nil {
		return nil, finalProvenancePending("CI04_ACTUAL_RP43_ORIGINALS_NOT_AVAILABLE")
	}
	if selected.ImplementationSHA != source {
		return nil, fmt.Errorf("CI04_HISTORICAL_RP43_COMPONENT")
	}
	if err := validateOutputSigningRecord("RP43", *selected); err != nil {
		return nil, fmt.Errorf("CI04_RP43_ORIGINALS_REJECTED")
	}
	return selected, nil
}

func executePublicationSecurity(config publicationSecurityConfig, source, output string, records []EvidenceRecord) (publicationSecurityObservation, error) {
	start := time.Now()
	observed := publicationSecurityObservation{Schema: publicationSecuritySchema, ImplementationSHA: source, ExpectedSHA256: config.ExpectedSHA256,
		StartedUTC: start.UTC().Format(time.RFC3339Nano), Status: "FAILED", Scope: publicationSecurityScope}
	ctx, cancel := context.WithTimeout(context.Background(), publicationSecurityTimeout)
	defer cancel()
	in, expected, err := publicationSecurityLoad(config, output)
	if err == nil {
		observed.ManifestSHA256 = expected.ManifestSHA256
		if in.Expected.Sources["implementation"].SHA != source {
			err = fmt.Errorf("CI04_CALLER_SOURCE_MISMATCH")
		}
	}
	if err == nil {
		for _, role := range []string{"implementation", "compiler", "docs"} {
			if err = publicationRejectPartialSource(ctx, in.Manifest.SourceDirectories[role]); err != nil {
				break
			}
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
	var raw map[string]int64
	if err == nil {
		raw, err = publicationKnownRaw(in, expected)
		observed.KnownRawCount = len(raw)
	}
	if err == nil {
		observed.SourceFiles, err = publicationValidateSource(ctx, in, expected, raw)
	}
	if err == nil {
		err = publicationValidateArtifactInventory(in, expected)
	}
	archives := map[string]map[string][]byte{}
	if err == nil {
		var total int64
		budget := &ciExpandedBudget{Limit: ciIdentityArchiveLimit}
		for id, inventory := range expected.PublicArchives {
			artifact := in.Expected.Artifacts[id]
			if err = ciIdentityArtifactAttempt(in, artifact); err != nil {
				break
			}
			total += in.Expected.Files[artifact.File].Bytes
			if total > ciIdentityArchiveLimit {
				err = fmt.Errorf("CI04_PUBLIC_BYTE_LIMIT")
				break
			}
			var data []byte
			data, err = ciIdentityReadOriginal(in, artifact.File, ciIdentityArchiveLimit)
			if err != nil {
				break
			}
			var count int
			archives[id], count, err = publicationInspectMembers(data, inventory, raw, 0, budget)
			if err != nil {
				break
			}
			observed.PublicMembers += count
		}
	}
	if err == nil {
		err = publicationValidateSigning(ctx, in, expected, archives)
	}
	if err == nil {
		err = publicationValidateSealed(ctx, in, expected, archives)
	}
	if err == nil {
		err = publicationValidateSanitizer(in, expected, archives)
		if err == nil {
			observed.SanitizerTests = append([]string{}, publicationSanitizerTests...)
		}
	}
	if err == nil {
		observed.RP43, err = publicationRP43(records, source)
	}
	if err == nil {
		data, readErr := stateReadArtifacts(observed.RP43.Artifacts, outputSigningArtifacts)
		var receipt outputSigningProcessReceipt
		if readErr != nil {
			err = readErr
		} else if err = decodeEditorJSON(data["rp43-process.json"], &receipt); err == nil {
			job, _, jobErr := ciIdentityJob(in, expected.Sanitizer.Checkout)
			if jobErr != nil {
				err = jobErr
			} else {
				observed.InputsCompletedUTC, err = ciIdentityInputsCompleted(in, receipt.FinishedUTC, finalProvenanceString(job, "completed_at"))
			}
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
		err = ciIdentityNotBackdated(observed.StartedUTC, observed.InputsCompletedUTC)
	}
	if err == nil && ctx.Err() != nil {
		err = fmt.Errorf("CI04_PUBLICATION_DEADLINE")
	}
	if err == nil {
		observed.Status = "PASSED"
	} else {
		var pending finalProvenancePending
		if errors.As(err, &pending) {
			observed.Status = "NOT_RUN"
		}
		observed.Failure = "CI04_PUBLICATION_INPUTS_REJECTED_OR_INCOMPLETE"
	}
	observed.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	observed.ElapsedSeconds = time.Since(start).Seconds()
	return observed, err
}

// Called once after runRepairCases/runRecoveryGates. It consumes their actual
// RP43 record and cannot invoke the six-parent signing producer again.
func (r *Runner) runPublicationSecurityRequirement() {
	r.publicationSecurityEvaluated = true
	config, configured, configErr := publicationSecurityEnvironment()
	if !configured {
		r.results["CI04"] = CaseResult{ID: "CI04", Group: "verification", Requirement: "No private raw corpus or unrequested signing", TestLevel: "security+repository", Status: "NOT_RUN", Detail: "CI04 independent publication/privacy inventory is not configured"}
		return
	}
	if configErr != nil {
		r.results["CI04"] = CaseResult{ID: "CI04", Group: "verification", Requirement: "No private raw corpus or unrequested signing", TestLevel: "security+repository", Status: "FAILED", Detail: "CI04 caller context is malformed"}
		return
	}
	directory, err := stateEvidenceDirectory(r, "ci04-")
	if err != nil {
		r.results["CI04"] = CaseResult{ID: "CI04", Group: "verification", Requirement: "No private raw corpus or unrequested signing", TestLevel: "security+repository", Status: "FAILED", Detail: "CI04 evidence directory is unavailable"}
		return
	}
	observed, executionErr := executePublicationSecurity(config, r.implSHA, directory, r.records)
	encoded, err := json.MarshalIndent(observed, "", "  ")
	name := filepath.Join(directory, "ci04-observation.json")
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
		levels = []string{"security", "repository"}
	}
	r.results["CI04"] = CaseResult{ID: "CI04", Group: "verification", Requirement: "No private raw corpus or unrequested signing", TestLevel: "security+repository", ExecutedLevels: levels, Passed: passed, Status: observed.Status, Detail: publicationSecurityScope}
	if !passed {
		return
	}
	for _, tier := range levels {
		r.records = append(r.records, EvidenceRecord{EvidenceID: "ev-ci04-publication-" + tier, Requirements: []string{"CI04"}, Tier: tier, ImplementationSHA: r.implSHA,
			TestID: publicationSecurityTestID, Command: []string{"internal", publicationSecuritySchema}, ExitCode: 0, Result: "passed",
			Assertions: map[string]string{"scope": publicationSecurityScope, "expected_context_sha256": config.ExpectedSHA256, "manifest_sha256": observed.ManifestSHA256, "closure_complete": "false"},
			Artifacts:  []EvidenceArtifact{{Path: name, SHA256: ciIdentityDigest(append(encoded, '\n'))}}, Detail: publicationSecurityScope})
	}
}

func validatePublicationSecurityRecord(id string, record EvidenceRecord) error {
	if id != "CI04" {
		if record.TestID == publicationSecurityTestID {
			return fmt.Errorf("CI04_PUBLICATION_SCOPE_DOES_NOT_GRANT_ANOTHER_REQUIREMENT")
		}
		return nil
	}
	if record.TestID != publicationSecurityTestID || !reflect.DeepEqual(record.Requirements, []string{"CI04"}) || (record.Tier != "repository" && record.Tier != "security") ||
		record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Command, []string{"internal", publicationSecuritySchema}) ||
		record.Assertions["scope"] != publicationSecurityScope || record.Assertions["closure_complete"] != "false" {
		return fmt.Errorf("CI04_RECORD_NOT_DEDICATED_PUBLICATION_AUDIT")
	}
	config, configured, err := publicationSecurityEnvironment()
	if err != nil || !configured || record.Assertions["expected_context_sha256"] != config.ExpectedSHA256 {
		return fmt.Errorf("CI04_INDEPENDENT_CONTEXT_UNAVAILABLE")
	}
	data, err := stateReadArtifacts(record.Artifacts, []string{"ci04-observation.json"})
	if err != nil {
		return err
	}
	var original publicationSecurityObservation
	if err = decodeEditorJSON(data["ci04-observation.json"], &original); err != nil {
		return err
	}
	if original.Schema != publicationSecuritySchema || original.Status != "PASSED" || original.ClosureComplete || original.Scope != publicationSecurityScope || original.ImplementationSHA != record.ImplementationSHA ||
		original.ExpectedSHA256 != config.ExpectedSHA256 || original.ManifestSHA256 != record.Assertions["manifest_sha256"] || original.Failure != "" || original.RP43 == nil {
		return fmt.Errorf("CI04_ORIGINAL_OBSERVATION")
	}
	if err = stateTiming(original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds, publicationSecurityTimeout.Seconds()); err != nil {
		return err
	}
	revalidated, err := executePublicationSecurity(config, record.ImplementationSHA, "", []EvidenceRecord{*original.RP43})
	if err != nil || revalidated.Status != "PASSED" {
		return fmt.Errorf("CI04_ORIGINAL_PUBLICATION_REVALIDATION")
	}
	return publicationCompareObservation(original, revalidated)
}

func applyPublicationSecurityAudit(row *ExecutionAuditRow, runner *Runner) {
	if row.ID != "CI04" || !runner.publicationSecurityEvaluated {
		return
	}
	row.TestNames = []string{publicationSecurityTestID}
	row.ProductionPath = append([]string{}, publicationSecuritySources...)
	row.Scope = publicationSecurityScope
	result := runner.results["CI04"]
	row.PositiveAndNegative = result.Passed
	row.Blocker = ""
	if result.Status != "PASSED" {
		row.Blocker = result.Detail
	}
}

func publicationCompareObservation(original, revalidated publicationSecurityObservation) error {
	if err := ciIdentityNotBackdated(original.StartedUTC, revalidated.InputsCompletedUTC); err != nil {
		return err
	}
	original.StartedUTC, original.FinishedUTC, original.ElapsedSeconds = revalidated.StartedUTC, revalidated.FinishedUTC, revalidated.ElapsedSeconds
	if !reflect.DeepEqual(original, revalidated) {
		return fmt.Errorf("CI04_ORIGINAL_PUBLICATION_REVALIDATION")
	}
	return nil
}
