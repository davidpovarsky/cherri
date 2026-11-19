package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const documentationAppTestID = "CherriUITests/testD03CurrentDocumentationSearchUI"
const documentationConsumerTestID = "tools/p10r15:verify_documentation_consumers"
const documentationCheckpoint = "[CHECKPOINT:D03_CURRENT_DOCS_SEARCH_VERIFIED]"
const documentationReceiptPrefix = "[CHERRI_DOCS_RECEIPT] "

var documentationSourcePaths = []string{
	"fork-language/index.md", "fork-language/guide.md",
	"fork-language/actions.md", "fork-language/migration.md",
}

var documentationAppTests = []string{
	"DocumentationIndexTests/testD03BundledCurrentDocumentationSearch",
	"DocumentationIndexTests/testD03RejectsHistoricalAndDamagedBundleRows",
	"CherriEditorUITests/testD03CurrentDocumentationSearchUI",
}

var documentationCaseEvent = regexp.MustCompile("^Test Case '-\\[([A-Za-z][A-Za-z0-9_]*\\.[A-Za-z][A-Za-z0-9_]*) (test[A-Za-z0-9_]+)\\]' (started\\.|(?:passed|failed) \\([0-9.]+ seconds\\)\\.)$")
var documentationLanguage = regexp.MustCompile("^[a-z0-9_+-]*$")
var documentationHex = regexp.MustCompile("^[0-9a-f]+$")

type documentationPins struct {
	SkillVersion         string `json:"skillVersion"`
	LanguageVersion      string `json:"languageVersion"`
	CatalogSchemaVersion string `json:"catalogSchemaVersion"`
	SchemaFingerprint    string `json:"schemaFingerprint"`
	CodecFingerprint     string `json:"codecFingerprint"`
	CompilerRepo         string `json:"compilerRepo"`
	CompilerCommit       string `json:"compilerCommit"`
	DocsRepo             string `json:"docsRepo"`
	DocsCommit           string `json:"docsCommit"`
	MacroFingerprint     string `json:"macroFingerprint"`
}

type documentationSourceFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type documentationIndex struct {
	FormatVersion     int                       `json:"formatVersion"`
	LanguageVersion   string                    `json:"languageVersion"`
	DocsCommit        string                    `json:"docsCommit"`
	CompilerCommit    string                    `json:"compilerCommit"`
	SchemaFingerprint string                    `json:"schemaFingerprint"`
	MacroFingerprint  string                    `json:"macroFingerprint"`
	SourceFiles       []documentationSourceFile `json:"sourceFiles"`
	Sections          []struct {
		ID            string `json:"id"`
		SourcePath    string `json:"sourcePath"`
		SourceLine    int    `json:"sourceLine"`
		DocumentTitle string `json:"documentTitle"`
		Title         string `json:"title"`
		MigrationOnly bool   `json:"migrationOnly"`
		Blocks        []struct {
			Kind     string `json:"kind"`
			Language string `json:"language"`
			Text     string `json:"text"`
		} `json:"blocks"`
	} `json:"sections"`
}

type documentationAppReceipt struct {
	SchemaVersion            string   `json:"schemaVersion"`
	DocsCommit               string   `json:"docsCommit"`
	CompilerCommit           string   `json:"compilerCommit"`
	SchemaFingerprint        string   `json:"schemaFingerprint"`
	MacroFingerprint         string   `json:"macroFingerprint"`
	BundleSHA256             string   `json:"bundleSHA256"`
	SourcePaths              []string `json:"sourcePaths"`
	SourceHashes             []string `json:"sourceHashes"`
	Sections                 int      `json:"sections"`
	ExactActionFirst         string   `json:"exactActionFirst"`
	DefaultMigrationResults  *int     `json:"defaultMigrationResults"`
	ExplicitMigrationResults int      `json:"explicitMigrationResults"`
	MissingResults           *int     `json:"missingResults"`
}

type documentationGenerationReceipt struct {
	DocsCommit     string `json:"docsCommit"`
	CompilerCommit string `json:"compilerCommit"`
	SourceFiles    int    `json:"sourceFiles"`
	Sections       int    `json:"sections"`
	Bytes          int    `json:"bytes"`
	SHA256         string `json:"sha256"`
}

func documentationHash(value string, length int) bool {
	return len(value) == length && documentationHex.MatchString(value)
}

func decodeDocumentationJSON(data []byte, value any) error {
	if _, err := decodeNativeEncodingObject(data); err != nil {
		return err
	}
	var decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	return decoder.Decode(value)
}

func documentationGitBlob(repository, commit, name string) ([]byte, error) {
	if !documentationHash(commit, 40) {
		return nil, fmt.Errorf("non-immutable source revision")
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, "git", "-C", repository, "show", commit+":"+name)
	command.WaitDelay = 2 * time.Second
	var data, err = command.Output()
	if err != nil || len(data) > 4*1024*1024 {
		return nil, fmt.Errorf("pinned Git object is unavailable: %s", name)
	}
	return data, nil
}

// Every artifact is read once into the same bounded hash-checked byte set used
// below. A summary cannot stand in for the actual process and bundle artifacts.
func documentationArtifacts(record EvidenceRecord) (map[string][]byte, map[string]EvidenceArtifact, error) {
	if len(record.Artifacts) == 0 || len(record.Artifacts) > 48 {
		return nil, nil, fmt.Errorf("missing or oversized artifact set")
	}
	var data = make(map[string][]byte)
	var identities = make(map[string]EvidenceArtifact)
	for _, artifact := range record.Artifacts {
		var name = filepath.Base(artifact.Path)
		if _, duplicate := data[name]; duplicate || !documentationHash(artifact.SHA256, 64) {
			return nil, nil, fmt.Errorf("duplicate artifact name or missing digest")
		}
		var info, statErr = os.Stat(artifact.Path)
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > 4*1024*1024 {
			return nil, nil, fmt.Errorf("unavailable or oversized artifact %s", name)
		}
		var content, err = os.ReadFile(artifact.Path)
		if err != nil || nativeEncodingDigest(content) != artifact.SHA256 {
			return nil, nil, fmt.Errorf("artifact digest mismatch: %s", name)
		}
		data[name], identities[name] = content, artifact
	}
	return data, identities, nil
}

func documentationManifestPins(record EvidenceRecord, manifest *EvidenceManifest, data []byte) (documentationPins, error) {
	var pins documentationPins
	if err := decodeDocumentationJSON(data, &pins); err != nil {
		return pins, fmt.Errorf("invalid documentation compatibility manifest: %w", err)
	}
	if pins.CompilerRepo != "https://github.com/davidpovarsky/cherri.git" ||
		pins.DocsRepo != "https://github.com/davidpovarsky/cherrilang.org.git" ||
		pins.LanguageVersion != "2.0" || !documentationHash(pins.CompilerCommit, 40) ||
		!documentationHash(pins.DocsCommit, 40) || !documentationHash(pins.SchemaFingerprint, 64) ||
		!documentationHash(pins.CodecFingerprint, 64) || !documentationHash(pins.MacroFingerprint, 64) {
		return pins, fmt.Errorf("non-canonical or incomplete documentation pins")
	}
	var source, err = documentationGitBlob(resolveRepoRoot(), record.ImplementationSHA, "skills/cherri-shortcuts/compatibility-manifest.json")
	if err != nil || !bytes.Equal(data, source) {
		return pins, fmt.Errorf("manifest artifact differs from the immutable implementation Git object")
	}
	if manifest == nil || manifest.ImplementationSHA != record.ImplementationSHA ||
		manifest.DocsSHA != pins.DocsCommit || manifest.SchemaFingerprint != pins.SchemaFingerprint ||
		manifest.CodecFingerprint != pins.CodecFingerprint {
		return pins, fmt.Errorf("documentation pins differ from the acceptance manifest")
	}
	return pins, nil
}

func documentationSourceMap(files []documentationSourceFile) (map[string]string, error) {
	var expected = make(map[string]bool)
	for _, name := range documentationSourcePaths {
		expected[name] = true
	}
	var result = make(map[string]string)
	if len(files) != len(expected) {
		return nil, fmt.Errorf("documentation source count differs from four canonical paths")
	}
	for _, file := range files {
		if !expected[file.Path] || result[file.Path] != "" || !documentationHash(file.SHA256, 64) {
			return nil, fmt.Errorf("historical, duplicated, or unhashed documentation source")
		}
		result[file.Path] = file.SHA256
	}
	return result, nil
}

func validateDocumentationIndex(data []byte, pins documentationPins) (documentationIndex, int, error) {
	var index documentationIndex
	if len(data) > 2_000_000 {
		return index, 0, fmt.Errorf("bundled documentation exceeds the app limit")
	}
	if err := decodeDocumentationJSON(data, &index); err != nil {
		return index, 0, fmt.Errorf("invalid bundled documentation: %w", err)
	}
	if index.FormatVersion != 1 || index.LanguageVersion != "2.0" ||
		index.DocsCommit != pins.DocsCommit || index.CompilerCommit != pins.CompilerCommit ||
		index.SchemaFingerprint != pins.SchemaFingerprint || index.MacroFingerprint != pins.MacroFingerprint ||
		len(index.Sections) <= 500 {
		return index, 0, fmt.Errorf("bundled documentation tuple or complete section set differs")
	}
	var sources, sourceErr = documentationSourceMap(index.SourceFiles)
	if sourceErr != nil {
		return index, 0, sourceErr
	}
	var seen = make(map[string]bool)
	var migrationCount int
	var showFound bool
	for _, section := range index.Sections {
		if sources[section.SourcePath] == "" || section.SourceLine <= 0 ||
			section.ID != section.SourcePath+":"+strconv.Itoa(section.SourceLine) || seen[section.ID] ||
			section.MigrationOnly != (section.SourcePath == "fork-language/migration.md") ||
			section.Title == "" || section.DocumentTitle == "" || len(section.Blocks) == 0 {
			return index, 0, fmt.Errorf("historical, duplicated, or malformed documentation section")
		}
		seen[section.ID] = true
		if section.MigrationOnly {
			migrationCount++
		}
		for _, block := range section.Blocks {
			if (block.Kind != "prose" && block.Kind != "code") || !documentationLanguage.MatchString(block.Language) ||
				(block.Language == "cherri-legacy" && !section.MigrationOnly) {
				return index, 0, fmt.Errorf("invalid or silently active Legacy documentation block")
			}
			if section.Title == "show" && section.SourcePath == "fork-language/actions.md" &&
				block.Kind == "code" && strings.Contains(block.Text, "show(") {
				showFound = true
			}
		}
	}
	if !showFound || migrationCount == 0 {
		return index, 0, fmt.Errorf("current action or explicit migration documentation is missing")
	}
	return index, migrationCount, nil
}

func parseDocumentationAppLog(data []byte) (documentationAppReceipt, error) {
	var receipt documentationAppReceipt
	var active string
	var started = make(map[string]int)
	var passed = make(map[string]int)
	var receiptCount, checkpointCount int
	for _, line := range strings.Split(string(data), "\n") {
		// The canonical collector accepts surrounding whitespace while retaining
		// the original artifact bytes. Match its event syntax after the digest
		// has been checked; never normalize the evidence artifact itself.
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[CHERRI_DOCS_REJECTED] ") {
			return receipt, fmt.Errorf("collector retained a rejected documentation event")
		}
		if event := documentationCaseEvent.FindStringSubmatch(line); event != nil {
			var suite = strings.Split(event[1], ".")
			var id = suite[1] + "/" + event[2]
			for i, required := range documentationAppTests {
				if id == required {
					var expectedTarget = "CherriTests"
					if i == 2 {
						expectedTarget = "CherriUITests"
					}
					if suite[0] != expectedTarget {
						return receipt, fmt.Errorf("required documentation test ran in the wrong target")
					}
				}
			}
			if event[3] == "started." {
				if active != "" {
					return receipt, fmt.Errorf("interleaved XCTest blocks")
				}
				active = event[1] + "/" + event[2]
				started[id]++
			} else {
				if active != event[1]+"/"+event[2] || !strings.HasPrefix(event[3], "passed ") {
					return receipt, fmt.Errorf("failed or unmatched XCTest completion")
				}
				passed[id]++
				active = ""
			}
			continue
		}
		if strings.HasPrefix(line, documentationReceiptPrefix) {
			if active != "CherriTests.DocumentationIndexTests/testD03BundledCurrentDocumentationSearch" {
				return receipt, fmt.Errorf("documentation receipt is outside the positive unit test")
			}
			receiptCount++
			if err := decodeDocumentationJSON([]byte(strings.TrimPrefix(line, documentationReceiptPrefix)), &receipt); err != nil {
				return receipt, fmt.Errorf("invalid raw documentation receipt: %w", err)
			}
		}
		if strings.HasPrefix(line, "[CHECKPOINT:D03") {
			if line != documentationCheckpoint || active != "CherriUITests.CherriEditorUITests/testD03CurrentDocumentationSearchUI" {
				return receipt, fmt.Errorf("documentation UI checkpoint is not in its exact test block")
			}
			checkpointCount++
		}
	}
	if active != "" || receiptCount != 1 || checkpointCount != 1 {
		return receipt, fmt.Errorf("incomplete test blocks or missing/duplicated documentation receipts")
	}
	for _, id := range documentationAppTests {
		if started[id] != 1 || passed[id] != 1 {
			return receipt, fmt.Errorf("required exact XCTest did not run and pass once: %s", id)
		}
	}
	return receipt, nil
}

func validateDocumentationApp(record EvidenceRecord, manifest *EvidenceManifest, artifacts map[string][]byte, identities map[string]EvidenceArtifact, pins documentationPins) error {
	if len(artifacts) != 5 {
		return fmt.Errorf("D03 app proof requires the exact five collector artifacts")
	}
	var required = []string{"xcodetest.log", "ios-test-diagnostics.json", "documentation-index.json",
		"documentation-receipt.json", "documentation-compatibility-manifest.json"}
	var runID string
	for _, name := range required {
		var artifact, present = identities[name]
		if !present || artifact.RunID == "" || (runID != "" && artifact.RunID != runID) {
			return fmt.Errorf("D03 app artifacts are absent or from mixed CI runs")
		}
		runID = artifact.RunID
	}
	var matchingRuns int
	for _, run := range manifest.Runs {
		if run.RunID == runID {
			matchingRuns++
			if run.Repository != "davidpovarsky/cherri" || run.WorkflowName != "iOS Build" ||
				run.HeadSHA != record.ImplementationSHA || run.CheckoutSHA != record.ImplementationSHA ||
				(run.Conclusion != "success" && run.Conclusion != "SUCCESS") {
				return fmt.Errorf("D03 requires the actual successful source-bound iOS Build run")
			}
		}
	}
	if matchingRuns != 1 {
		return fmt.Errorf("D03 app run is missing or duplicated")
	}
	var diagnostic, err = decodeNativeEncodingObject(artifacts["ios-test-diagnostics.json"])
	if err != nil || diagnostic["schemaVersion"] != "1.0" ||
		diagnostic["implementationSha"] != record.ImplementationSHA || diagnostic["runId"] != runID ||
		!nativeEncodingInteger(diagnostic["xcodebuildExitCode"], 0) || diagnostic["runStatus"] != "PASSED" ||
		diagnostic["rawLogPresent"] != true || diagnostic["sanitizedLogSha256"] != identities["xcodetest.log"].SHA256 {
		return fmt.Errorf("D03 actual Xcode process receipt is missing, failed, or stale")
	}
	var results, ok = diagnostic["testResults"].([]any)
	var observed = make(map[string]bool)
	if !ok || len(results) == 0 {
		return fmt.Errorf("D03 exact test outcomes are missing")
	}
	for _, value := range results {
		var result, valid = value.(map[string]any)
		var id, stringID = result["testId"].(string)
		if !valid || !stringID || id == "" || observed[id] || result["observedStatus"] != "passed" {
			return fmt.Errorf("D03 process has duplicated, failed or incomplete test outcomes")
		}
		observed[id] = true
	}
	for _, id := range documentationAppTests {
		if !observed[id] {
			return fmt.Errorf("D03 diagnostics omit required test %s", id)
		}
	}
	var receipt, receiptErr = parseDocumentationAppLog(artifacts["xcodetest.log"])
	if receiptErr != nil {
		return receiptErr
	}
	var index, migrations, indexErr = validateDocumentationIndex(artifacts["documentation-index.json"], pins)
	if indexErr != nil {
		return indexErr
	}
	var generated documentationGenerationReceipt
	if err := decodeDocumentationJSON(artifacts["documentation-receipt.json"], &generated); err != nil ||
		generated.DocsCommit != pins.DocsCommit || generated.CompilerCommit != pins.CompilerCommit ||
		generated.SourceFiles != 4 || generated.Sections != len(index.Sections) ||
		generated.Bytes != len(artifacts["documentation-index.json"]) ||
		generated.SHA256 != identities["documentation-index.json"].SHA256 {
		return fmt.Errorf("D03 canonical generation receipt does not bind the exact bundle")
	}
	if receipt.SchemaVersion != "1.0" || receipt.DocsCommit != pins.DocsCommit ||
		receipt.CompilerCommit != pins.CompilerCommit || receipt.SchemaFingerprint != pins.SchemaFingerprint ||
		receipt.MacroFingerprint != pins.MacroFingerprint || receipt.BundleSHA256 != generated.SHA256 ||
		receipt.Sections != len(index.Sections) || receipt.ExactActionFirst != "show" ||
		receipt.DefaultMigrationResults == nil || *receipt.DefaultMigrationResults != 0 ||
		receipt.MissingResults == nil || *receipt.MissingResults != 0 ||
		receipt.ExplicitMigrationResults != migrations {
		return fmt.Errorf("D03 runtime bundle receipt does not establish exact current/default/explicit search results")
	}
	var pairs []documentationSourceFile
	if len(receipt.SourcePaths) != len(receipt.SourceHashes) {
		return fmt.Errorf("D03 source path/hash pairs have different lengths")
	}
	for i, name := range receipt.SourcePaths {
		pairs = append(pairs, documentationSourceFile{Path: name, SHA256: receipt.SourceHashes[i]})
	}
	var observedSources, sourceErr = documentationSourceMap(pairs)
	var indexSources, _ = documentationSourceMap(index.SourceFiles)
	if sourceErr != nil || !reflect.DeepEqual(observedSources, indexSources) {
		return fmt.Errorf("D03 observed bundled sources differ from the exact index")
	}
	var docsRoot, docsErr = documentationDocsRepository(pins)
	if docsErr != nil {
		return docsErr
	}
	var canonicalSources = make(map[string][]byte)
	for _, name := range documentationSourcePaths {
		var original, objectErr = documentationGitBlob(docsRoot, pins.DocsCommit, name)
		if objectErr != nil || nativeEncodingDigest(original) != indexSources[name] {
			return fmt.Errorf("D03 bundled source hash differs from the pinned docs Git blob: %s", name)
		}
		canonicalSources[name] = original
	}
	var canonical, generationErr = canonicalDocumentationIndex(resolveRepoRoot(), record.ImplementationSHA,
		artifacts["documentation-compatibility-manifest.json"], canonicalSources)
	if generationErr != nil {
		return generationErr
	}
	if !bytes.Equal(canonical, artifacts["documentation-index.json"]) {
		return fmt.Errorf("D03 bundled content differs from the canonical K generator applied to the exact D Git blobs")
	}
	// Once the companion consumer proof exists, its actual pinned Git blob
	// hashes also bind the app's source list. Neither consumer can drift alone.
	for _, other := range manifest.Records {
		if other.TestID != documentationConsumerTestID || other.Tier != "docs" {
			continue
		}
		var otherArtifacts, _, artifactErr = documentationArtifacts(other)
		if artifactErr != nil {
			return fmt.Errorf("D03 companion consumer artifacts are invalid")
		}
		var consumers documentationConsumerReport
		if err := decodeDocumentationJSON(otherArtifacts["documentation-consumers.json"], &consumers); err != nil {
			return fmt.Errorf("D03 companion consumer report is invalid")
		}
		var sourceMap, sourceMapErr = documentationSourceMap(consumers.SourceFiles)
		if sourceMapErr != nil || !reflect.DeepEqual(sourceMap, indexSources) {
			return fmt.Errorf("D03 app and installed/public consumers disagree on pinned source bytes")
		}
	}
	return nil
}

func validateDocumentationRecord(id string, record EvidenceRecord, manifest *EvidenceManifest) error {
	if id != "D03" {
		return nil
	}
	var fail = func(err error) error {
		return fmt.Errorf("D03 documentation record %s: %w", record.EvidenceID, err)
	}
	if len(record.Requirements) != 1 || record.Requirements[0] != "D03" ||
		record.Result != "passed" || record.ExitCode != 0 {
		return fail(fmt.Errorf("only a dedicated actually passed D03 producer is authorized"))
	}
	var keys []string
	switch {
	case record.Tier == "integration" && record.TestID == documentationAppTestID:
		keys = []string{"current_docs_bundle", "historical_docs_rejected", "current_docs_ui_search"}
		if !reflect.DeepEqual(record.Command, []string{"xcodebuild", "test", "-scheme", "Cherri"}) {
			return fail(fmt.Errorf("app proof lacks the actual Xcode test command"))
		}
	case record.Tier == "docs" && record.TestID == documentationConsumerTestID:
		keys = []string{"published_current_docs", "installed_skill_current_docs", "historical_docs_explicit_only"}
	default:
		return fail(fmt.Errorf("test identity or tier does not establish published/installed docs or app integration"))
	}
	for _, key := range keys {
		if record.Assertions[key] != "true" {
			return fail(fmt.Errorf("missing true assertion %s", key))
		}
	}
	var artifacts, identities, err = documentationArtifacts(record)
	if err != nil {
		return fail(err)
	}
	var pins, pinsErr = documentationManifestPins(record, manifest, artifacts["documentation-compatibility-manifest.json"])
	if pinsErr != nil {
		return fail(pinsErr)
	}
	if record.Tier == "integration" {
		err = validateDocumentationApp(record, manifest, artifacts, identities, pins)
	} else {
		err = validateDocumentationConsumers(record, artifacts, identities, pins)
	}
	if err != nil {
		return fail(err)
	}
	return nil
}

func (r *Runner) runDocumentationSeparationRequirement() {
	r.recordUnexecutedRequirement("D03", "docs", "Active fork docs separated from upstream reference", []string{"docs", "integration"})
	var result = r.results["D03"]
	result.Detail = "Requires final-tuple public site bytes and installed Skill searches plus the same-run exact app unit, negative and UI tests. Local file existence or modeled receipts do not establish either consumer tier."
	r.results["D03"] = result
}
