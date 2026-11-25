package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

const previewNativeContractPath = "ios/TestFixtures/preview-document-contract.json"
const previewNativeContractSHA = "2233988245dde41e7e7b8d8b7e874bbc74008b46e6c54e19baa7d90778f3c845"
const previewNativeGatePath = "tests/language-v2/backend-recovery-gates.json"
const previewNativeGateSHA = "e7fbb3ad177c5142935f71a46ca7e89b84822061bfc0919eec59708811ab7e7c"
const previewNativeIntegration = "CherriPreviewDocumentContextTests/testBRG28ActualPreviewTransactionPreservesDocumentAndRejectsContextRelabels"
const previewNativeBenchmark = "CherriPreviewDocumentContextTests/testBRG28NativeSmallLargeAndEmbeddedAssetMeasurements"
const previewNativeUI = "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI"
const previewNativeUIPrefix = "CHERRI_NATIVE_PREVIEW_UI "
const previewNativeBenchmarkPrefix = "CHERRI_NATIVE_PREVIEW_BENCHMARK "
const previewNativeCheckpoint = "CHERRI_NATIVE_PREVIEW_UI_CHECKPOINT=BRG28_WEBKIT_FIELDS_COMPLETE"
const previewNativeRejected = "CHERRI_NATIVE_PREVIEW_REJECTED "
const previewNativeBenchmarkScope = "actual-swift-cabi-compilation-no-render-or-shortcut-runtime"
const previewNativeUISource = "// BRG28 public source preserved\nshow(\"BRG28_KEEP\")\nshow(\"BRG28_EDIT\")\nwait(2)\n// שלום 🍒 preserve tail\n"

var previewNativeMethods = []string{
	"CherriPreviewDocumentContextTests/testBRG28ActualPreviewTransactionPreservesDocumentAndRejectsContextRelabels",
	"CherriPreviewDocumentContextTests/testBRG28CancelledCompileCannotReturnAnArtifact",
	"CherriPreviewDocumentContextTests/testBRG28CoordinatedAssetNotificationsInvalidateSameSizeAndTimestamp",
	"CherriPreviewDocumentContextTests/testBRG28NativeSmallLargeAndEmbeddedAssetMeasurements",
	"CherriPreviewDocumentContextTests/testBRG28SavedDocumentAssetsAndImportedOriginsAcrossBothDialects",
	"CherriPreviewDocumentContextTests/testBRG28UnsavedMissingAndEscapingResourcesFailThroughActualBridge",
	"CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI",
}
var previewNativeSourcePaths = []string{
	".github/workflows/ios-build.yml",
	"action.go",
	"actions_std.go",
	"build_profile.go",
	"copy_paste.go",
	"functions.go",
	"includes.go",
	"internal/language/assets/cache.go",
	"ios/CherriApp/App/WorkspaceView.swift",
	"ios/CherriApp/Compiler/CherriCompiler.swift",
	"ios/CherriApp/Compiler/CherriDocumentCompileContext.swift",
	"ios/CherriApp/Documents/CherriDocument.swift",
	"ios/CherriApp/Documents/CherriProjectAssetObserver.swift",
	"ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
	"ios/CherriApp/Editor/CherriEditorView.swift",
	"ios/CherriApp/Preview/CherriPreviewEditTransaction.swift",
	"ios/CherriApp/Preview/CherriPreviewRenderState.swift",
	"ios/CherriApp/Preview/ShortcutPlistEditor.swift",
	"ios/CherriApp/Preview/ShortcutPreviewView.swift",
	"ios/CherriTests/CherriPreviewDocumentContextTests.swift",
	"ios/CherriUITests/CherriPreviewDocumentUITests.swift",
	"ios/CherriUITests/EditorStartupDeadline.swift",
	"ios/TestFixtures/preview-document-contract.json",
	"ios/TestSupport/CherriJoinedContract.swift",
	"ios/WebPreview/package-lock.json",
	"ios/WebPreview/package.json",
	"ios/WebPreview/src/main.ts",
	"ios/WebPreview/src/primitive-parameter-editing.mjs",
	"ios/WebPreview/tests/primitive-parameter-editing.test.mjs",
	"ios/project.yml",
	"ios/scripts/collect_ios_test_evidence.py",
	"ios/scripts/preview_native_evidence.py",
	"ios/scripts/test_preview_native_evidence.py",
	"ios_bridge.go",
	"language_emit_adapter.go",
	"mobile_compiler.go",
	"mobile_document_context.go",
	"native_document_legacy.go",
	"parser.go",
	"preview_source_mapping.go",
	"shortcutgen.go",
	"tests/language-v2/backend-recovery-gates.json",
}
var previewNativeStages = []string{"actual-source-built", "source-and-native-rebuild-published", "new-native-graph-rendered", "invalid-dom-number-preserved-source"}

type previewNativeProfile struct {
	Tier, Fixture, Scope string
	Assertions           map[string]string
}

func previewNativeProfileFor(test string) (previewNativeProfile, bool) {
	switch test {
	case "CherriPreviewDocumentContextTests/testBRG28ActualPreviewTransactionPreservesDocumentAndRejectsContextRelabels":
		return previewNativeProfile{Tier: "integration", Fixture: "ios/CherriTests/CherriPreviewDocumentContextTests.swift", Scope: "Dedicated native Swift/C ABI cohort verifies saved and unsaved document context, both dialects' relative resource origins, missing and escaping resource rejection, exact preview transaction source/context, URI/UUID/revision/asset-revision relabel rejection, coordinated same-size/same-mtime asset invalidation and pre-entry cancellation without an artifact. It does not claim interruption inside a running C call, Shortcut execution or physical-device performance.", Assertions: map[string]string{"same_run_seven_dedicated_methods": "true", "native_document_and_asset_context_bound": "true", "native_preview_transaction_source_exact": "true", "context_relabels_rejected": "true", "both_dialects_relative_resources": "true", "missing_unsaved_escape_rejected": "true", "same_metadata_asset_invalidation_observed": "true", "pre_entry_cancellation_no_artifact": "true"}}, true
	case "CherriPreviewDocumentUITests/testBRG28ActualWebKitFieldEditAndNumericRejectionUI":
		return previewNativeProfile{Tier: "ios-ui", Fixture: "ios/CherriUITests/CherriPreviewDocumentUITests.swift", Scope: "Dedicated XCUITest types a fixed Unicode/comment source in an actual document, builds it, edits the actual WebKit contentEditable Text field through the keyboard, blurs through another actual field, checks exact source and rebuilt graph, then rejects invalid numeric DOM input while retaining the committed source and no stale signature. Convenience preview buttons and generic E07 results supply no BRG28 tier.", Assertions: map[string]string{"same_run_seven_dedicated_methods": "true", "actual_webkit_contenteditable_keyboard": "true", "only_intended_source_node_changed": "true", "comments_unicode_and_other_nodes_preserved": "true", "native_rebuild_and_render_observed": "true", "invalid_numeric_dom_edit_rejected": "true", "no_stale_signed_artifact": "true"}}, true
	case "CherriPreviewDocumentContextTests/testBRG28NativeSmallLargeAndEmbeddedAssetMeasurements":
		return previewNativeProfile{Tier: "benchmark", Fixture: "ios/CherriTests/CherriPreviewDocumentContextTests.swift", Scope: "Dedicated native Swift/C ABI benchmark records twenty actual compile durations each for a one-action source, a 200-action source and a 2MiB embedded asset across distinct unrelated source revisions, with exact fixture/sample hashes, OS/model, independently checked p50/p95 and cache reads/encodes/hits. It measures compilation, not WebKit rendering, Shortcut runtime, a latency budget or the separate physical iPad Debug/Release comparison.", Assertions: map[string]string{"same_run_seven_dedicated_methods": "true", "sixty_original_native_compile_samples": "true", "small_large_embedded_fixture_hashes": "true", "independently_recomputed_p50_p95": "true", "actual_native_cache_deltas": "true", "recorded_os_and_simulator_model": "true"}}, true
	}
	return previewNativeProfile{}, false
}

// Only these three records are enrolled. Every record independently requires
// the same complete seven-method native cohort; generic E07 is never inherited.
func previewNativeTestIDs(id string) []string {
	if id != "BRG28" {
		return nil
	}
	return []string{previewNativeIntegration, previewNativeUI, previewNativeBenchmark}
}
func previewNativeUIAssertions(id string) map[string]string {
	if id != "BRG28" {
		return nil
	}
	p, _ := previewNativeProfileFor(previewNativeUI)
	return p.Assertions
}
func previewNativeOwned(test string) bool {
	for _, method := range previewNativeMethods {
		if method == test {
			return true
		}
	}
	return false
}
func previewNativeStrings(values []string) []any {
	out := make([]any, len(values))
	for i, value := range values {
		out[i] = value
	}
	return out
}
func validatePreviewNativeContract(raw []byte) error {
	// Pin the original row and exact authored contract bytes. No later collector
	// can change a required tier, fixture, sample count or scope in its own favor.
	if len(raw) == 0 || len(raw) > 64*1024 || nativeEncodingDigest(raw) != previewNativeContractSHA {
		return fmt.Errorf("PREVIEW_CONTRACT: immutable contract bytes differ")
	}
	if _, err := decodeNativeEncodingObject(raw); err != nil {
		return fmt.Errorf("PREVIEW_CONTRACT: %w", err)
	}
	current, err := os.ReadFile(resolveRepoPath(previewNativeContractPath))
	gate, gateErr := os.ReadFile(resolveRepoPath(previewNativeGatePath))
	if err != nil || gateErr != nil || !bytes.Equal(current, raw) || nativeEncodingDigest(gate) != previewNativeGateSHA {
		return fmt.Errorf("PREVIEW_CONTRACT: current contract or original requirement differs")
	}
	return nil
}

type previewNativeFixture struct{ Name, Source, AssetSHA string }

func previewNativeFixtureAt(index int) (previewNativeFixture, error) {
	switch index {
	case 0:
		return previewNativeFixture{"small", "show(\"BRG28_SMALL\")\n", ""}, nil
	case 1:
		var source strings.Builder
		for i := 0; i < 200; i++ {
			fmt.Fprintf(&source, "show(\"BRG28_LARGE_%d\")\n", i)
		}
		return previewNativeFixture{"large", source.String(), ""}, nil
	case 2:
		return previewNativeFixture{"embedded-2mib", "embedFile(\"payload.dat\")\n", "45026c02eaf4771246fe89c562f9b0d346943247669f7051a047a10f040deda0"}, nil
	}
	return previewNativeFixture{}, fmt.Errorf("PREVIEW_BENCHMARK: extra observation")
}
func previewNativePositive(value any) (float64, bool) {
	number, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	f, err := number.Float64()
	return f, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) && f > 0
}
func previewNativeNonnegativeInteger(value any) (int64, bool) {
	n, ok := value.(json.Number)
	if !ok {
		return 0, false
	}
	i, err := n.Int64()
	return i, err == nil && i >= 0
}
func validatePreviewNativeUIEvent(value map[string]any, index int) error {
	if index < 0 || index >= len(previewNativeStages) {
		return fmt.Errorf("PREVIEW_UI: extra event")
	}
	source := previewNativeUISource
	if index > 0 {
		source = strings.Replace(source, `show("BRG28_EDIT")`, `show("BRG28_CHANGED")`, 1)
	}
	expected := map[string]any{"schema": "cherri-native-preview-ui/1", "testId": previewNativeUI,
		"stage": previewNativeStages[index], "sourceSha256": nativeEncodingDigest([]byte(source))}
	if index == 1 {
		if _, ok := previewNativePositive(value["elapsedSeconds"]); !ok {
			return fmt.Errorf("PREVIEW_UI: actual positive duration required")
		}
		expected["elapsedSeconds"] = value["elapsedSeconds"]
	}
	if !reflect.DeepEqual(value, expected) {
		return fmt.Errorf("PREVIEW_UI: exact original stage order or source differs")
	}
	return nil
}
func validatePreviewNativeBenchmark(value map[string]any, index int) error {
	fixture, err := previewNativeFixtureAt(index)
	if err != nil {
		return err
	}
	keys := []string{"schema", "fixture", "sourceSha256", "sampleSourceSha256", "assetSha256", "sampleCount",
		"totalCompileMilliseconds", "p50Milliseconds", "p95Milliseconds", "osVersion", "simulatorModel",
		"mode", "buildProfile", "cacheReadsDelta", "cacheEncodesDelta", "cacheHitsDelta", "scope"}
	if len(value) != len(keys) {
		return fmt.Errorf("PREVIEW_BENCHMARK: exact observation fields required")
	}
	for _, key := range keys {
		if _, ok := value[key]; !ok {
			return fmt.Errorf("PREVIEW_BENCHMARK: missing field %s", key)
		}
	}
	if value["schema"] != "cherri-native-preview-benchmark/1" || value["fixture"] != fixture.Name ||
		value["sourceSha256"] != nativeEncodingDigest([]byte(fixture.Source)) || value["assetSha256"] != fixture.AssetSHA ||
		value["mode"] != "v2" || value["buildProfile"] != "debug" || value["scope"] != previewNativeBenchmarkScope ||
		!nativeEncodingInteger(value["sampleCount"], 20) {
		return fmt.Errorf("PREVIEW_BENCHMARK: fixture, source, asset or measured scope differs")
	}
	expected := []any{}
	for i := 0; i < 20; i++ {
		expected = append(expected, nativeEncodingDigest([]byte(fixture.Source+fmt.Sprintf("// measured unrelated revision %d\n", i))))
	}
	if !reflect.DeepEqual(value["sampleSourceSha256"], expected) {
		return fmt.Errorf("PREVIEW_BENCHMARK: actual revision sources differ")
	}
	samples, ok := value["totalCompileMilliseconds"].([]any)
	if !ok || len(samples) != 20 {
		return fmt.Errorf("PREVIEW_BENCHMARK: twenty original timings required")
	}
	ordered := make([]float64, 20)
	for i, sample := range samples {
		duration, ok := previewNativePositive(sample)
		if !ok {
			return fmt.Errorf("PREVIEW_BENCHMARK: invalid measured duration")
		}
		ordered[i] = duration
	}
	sort.Float64s(ordered)
	p50, ok50 := previewNativePositive(value["p50Milliseconds"])
	p95, ok95 := previewNativePositive(value["p95Milliseconds"])
	if !ok50 || !ok95 || p50 != ordered[9] || p95 != ordered[18] {
		return fmt.Errorf("PREVIEW_BENCHMARK: recomputed percentiles differ")
	}
	osVersion, okOS := value["osVersion"].(string)
	model, okModel := value["simulatorModel"].(string)
	if !okOS || !okModel || !regexp.MustCompile(`^Version [0-9.]{1,20} \(Build [A-Za-z0-9]{1,24}\)$`).MatchString(osVersion) ||
		!regexp.MustCompile(`^(iPhone|iPad)[0-9]{1,3},[0-9]{1,3}$`).MatchString(model) {
		return fmt.Errorf("PREVIEW_BENCHMARK: actual recorded platform required")
	}
	reads, okR := previewNativeNonnegativeInteger(value["cacheReadsDelta"])
	encodes, okE := previewNativeNonnegativeInteger(value["cacheEncodesDelta"])
	hits, okH := previewNativeNonnegativeInteger(value["cacheHitsDelta"])
	if !okR || !okE || !okH {
		return fmt.Errorf("PREVIEW_BENCHMARK: invalid cache counters")
	}
	if (fixture.AssetSHA == "" && (reads != 0 || encodes != 0 || hits != 0)) ||
		(fixture.AssetSHA != "" && (reads != 1 || encodes != 1 || hits < 19)) {
		return fmt.Errorf("PREVIEW_BENCHMARK: real cache deltas differ")
	}
	return nil
}

var previewNativeCaseLine = regexp.MustCompile(`^Test Case '-\[([A-Za-z0-9_]+)\.([A-Za-z0-9_]+) (test[A-Za-z0-9_]+)\]' (started\.|(passed|failed) \([0-9.]+ seconds\)\.)$`)

// Reconstruct original ownership intervals and all measurements independently.
// A proof JSON, aggregate count or collector assertion cannot replace a line.
func parsePreviewNativeLog(raw []byte) (map[string]any, error) {
	if len(raw) == 0 || len(raw) > 1200000 || !utf8.Valid(raw) {
		return nil, fmt.Errorf("PREVIEW_LOG: invalid original log")
	}
	active := ""
	started, passed := map[string]bool{}, map[string]bool{}
	ui, benchmarks := []any{}, []any{}
	checkpoint := false
	for _, original := range strings.Split(string(raw), "\n") {
		line := strings.TrimSuffix(original, "\r")
		stripped := strings.TrimSpace(line)
		if match := previewNativeCaseLine.FindStringSubmatch(stripped); match != nil {
			target, test := match[1], match[2]+"/"+match[3]
			known := previewNativeOwned(test)
			if match[4] == "started." {
				if previewNativeOwned(active) {
					return nil, fmt.Errorf("PREVIEW_CASE: interleaved original case")
				}
				active = test
				if known {
					expected := "CherriTests"
					if test == previewNativeUI {
						expected = "CherriUITests"
					}
					if target != expected || started[test] {
						return nil, fmt.Errorf("PREVIEW_CASE: duplicate or foreign target")
					}
					started[test] = true
				}
			} else {
				if known {
					expected := "CherriTests"
					if test == previewNativeUI {
						expected = "CherriUITests"
					}
					if target != expected || active != test || !started[test] || passed[test] || match[5] != "passed" {
						return nil, fmt.Errorf("PREVIEW_CASE: exact passing terminal required")
					}
					passed[test] = true
				}
				active = ""
			}
			continue
		}
		if strings.HasPrefix(stripped, "Test Case '") {
			if previewNativeOwned(active) || strings.Contains(stripped, "CherriPreviewDocument") {
				return nil, fmt.Errorf("PREVIEW_CASE: unsupported original boundary")
			}
			active = ""
			continue
		}
		if strings.HasPrefix(stripped, previewNativeRejected) {
			return nil, fmt.Errorf("PREVIEW_LOG: rejected original observation")
		}
		if strings.HasPrefix(stripped, previewNativeUIPrefix) {
			if line != stripped || active != previewNativeUI || checkpoint {
				return nil, fmt.Errorf("PREVIEW_UI: event outside owned case")
			}
			value, err := decodeNativeEncodingObject([]byte(strings.TrimPrefix(line, previewNativeUIPrefix)))
			if err != nil {
				return nil, fmt.Errorf("PREVIEW_UI: malformed original event")
			}
			if err := validatePreviewNativeUIEvent(value, len(ui)); err != nil {
				return nil, err
			}
			ui = append(ui, value)
			continue
		}
		if strings.HasPrefix(stripped, previewNativeBenchmarkPrefix) {
			if line != stripped || active != previewNativeBenchmark {
				return nil, fmt.Errorf("PREVIEW_BENCHMARK: event outside measured case")
			}
			value, err := decodeNativeEncodingObject([]byte(strings.TrimPrefix(line, previewNativeBenchmarkPrefix)))
			if err != nil {
				return nil, fmt.Errorf("PREVIEW_BENCHMARK: malformed original observation")
			}
			if err := validatePreviewNativeBenchmark(value, len(benchmarks)); err != nil {
				return nil, err
			}
			benchmarks = append(benchmarks, value)
			continue
		}
		if strings.HasPrefix(stripped, "CHERRI_NATIVE_PREVIEW_UI_CHECKPOINT") {
			if line != previewNativeCheckpoint || active != previewNativeUI || checkpoint || len(ui) != 4 {
				return nil, fmt.Errorf("PREVIEW_CHECKPOINT: exact complete owned checkpoint required")
			}
			checkpoint = true
		}
	}
	if previewNativeOwned(active) || len(started) != 7 || len(passed) != 7 {
		return nil, fmt.Errorf("PREVIEW_COHORT: all seven original dedicated cases required")
	}
	for _, test := range previewNativeMethods {
		if !started[test] || !passed[test] {
			return nil, fmt.Errorf("PREVIEW_COHORT: original case missing")
		}
	}
	if len(ui) != 4 || !checkpoint || len(benchmarks) != 3 {
		return nil, fmt.Errorf("PREVIEW_COHORT: complete actual UI and sixty measurements required")
	}
	return map[string]any{"passedMethods": previewNativeStrings(previewNativeMethods), "uiEvents": ui,
		"uiCheckpoint": previewNativeCheckpoint, "benchmarkObservations": benchmarks}, nil
}

func previewNativeWithoutNumbers(value map[string]any, keys ...string) map[string]any {
	out := map[string]any{}
	for key, item := range value {
		out[key] = item
	}
	for _, key := range keys {
		delete(out, key)
	}
	return out
}
func previewNativeSamePositiveNumber(a, b any) bool {
	x, okX := previewNativePositive(a)
	y, okY := previewNativePositive(b)
	return okX && okY && x == y
}

// Python JSON serialization may spell the same finite Double using another
// exponent form. Validate both observation schemas before comparing actual
// numeric values; integer cache counters and all provenance remain exact.
func previewNativeObservationsEqual(candidate any, expected map[string]any) bool {
	value, ok := candidate.(map[string]any)
	if !ok || len(value) != 4 || !reflect.DeepEqual(value["passedMethods"], expected["passedMethods"]) ||
		value["uiCheckpoint"] != previewNativeCheckpoint {
		return false
	}
	ui, okU := value["uiEvents"].([]any)
	bench, okB := value["benchmarkObservations"].([]any)
	if !okU || !okB || len(ui) != 4 || len(bench) != 3 {
		return false
	}
	expectedUI := expected["uiEvents"].([]any)
	expectedBench := expected["benchmarkObservations"].([]any)
	for i, row := range ui {
		actual, ok := row.(map[string]any)
		if !ok || validatePreviewNativeUIEvent(actual, i) != nil {
			return false
		}
		original := expectedUI[i].(map[string]any)
		if !reflect.DeepEqual(previewNativeWithoutNumbers(actual, "elapsedSeconds"), previewNativeWithoutNumbers(original, "elapsedSeconds")) {
			return false
		}
		if i == 1 && !previewNativeSamePositiveNumber(actual["elapsedSeconds"], original["elapsedSeconds"]) {
			return false
		}
	}
	for i, row := range bench {
		actual, ok := row.(map[string]any)
		if !ok || validatePreviewNativeBenchmark(actual, i) != nil {
			return false
		}
		original := expectedBench[i].(map[string]any)
		keys := []string{"totalCompileMilliseconds", "p50Milliseconds", "p95Milliseconds"}
		if !reflect.DeepEqual(previewNativeWithoutNumbers(actual, keys...), previewNativeWithoutNumbers(original, keys...)) {
			return false
		}
		a, b := actual["totalCompileMilliseconds"].([]any), original["totalCompileMilliseconds"].([]any)
		for j := range a {
			if !previewNativeSamePositiveNumber(a[j], b[j]) {
				return false
			}
		}
		if !previewNativeSamePositiveNumber(actual["p50Milliseconds"], original["p50Milliseconds"]) ||
			!previewNativeSamePositiveNumber(actual["p95Milliseconds"], original["p95Milliseconds"]) {
			return false
		}
	}
	return true
}

func previewNativeCurrentPins() ([]any, error) {
	paths := append([]string{}, previewNativeSourcePaths...)
	sort.Strings(paths)
	pins := []any{}
	for _, name := range paths {
		path := resolveRepoPath(name)
		info, err := os.Lstat(path)
		if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > 2*1024*1024 {
			return nil, fmt.Errorf("PREVIEW_SOURCE: invalid actual producer %s", name)
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		h := sha1.Sum(append([]byte("blob "+strconv.Itoa(len(raw))+"\x00"), raw...))
		mode := "100644"
		if info.Mode()&0100 != 0 {
			mode = "100755"
		}
		pins = append(pins, map[string]any{"path": name, "bytes": json.Number(strconv.Itoa(len(raw))), "sha256": nativeEncodingDigest(raw), "gitBlob": hex.EncodeToString(h[:]), "gitMode": mode})
	}
	return pins, nil
}
func validatePreviewNativeRecord(id string, record EvidenceRecord, manifest *EvidenceManifest) error {
	if id != "BRG28" {
		return nil
	}
	profile, known := previewNativeProfileFor(record.TestID)
	if manifest == nil || !known || record.Tier != profile.Tier ||
		!regexp.MustCompile("^[0-9a-f]{40}$").MatchString(record.ImplementationSHA) ||
		record.ImplementationSHA != manifest.ImplementationSHA || !reflect.DeepEqual(record.Requirements, []string{"BRG28"}) ||
		record.ExitCode != 0 || record.Result != "passed" ||
		!reflect.DeepEqual(record.Command, []string{"xcodebuild", "test", "-scheme", "Cherri"}) ||
		!reflect.DeepEqual(record.Assertions, profile.Assertions) || record.Detail != profile.Scope {
		return fmt.Errorf("PREVIEW_RECORD: only three dedicated native tiers are enrolled")
	}
	fixture, err := os.ReadFile(resolveRepoPath(profile.Fixture))
	if err != nil || len(fixture) == 0 || record.FixtureSHA256 != nativeEncodingDigest(fixture) {
		return fmt.Errorf("PREVIEW_FIXTURE: current native producer differs")
	}
	if len(record.Artifacts) != 4 {
		return fmt.Errorf("PREVIEW_ARTIFACTS: four exact originals required")
	}
	limits := map[string]int{"xcodetest.log": 1200000, "ios-test-diagnostics.json": 4 * 1024 * 1024, "preview-document-contract.json": 64 * 1024, "preview-native-proof.json": 1024 * 1024}
	data, hashes := map[string][]byte{}, map[string]string{}
	runID := ""
	for _, artifact := range record.Artifacts {
		name := filepath.Base(artifact.Path)
		limit, allowed := limits[name]
		info, err := os.Lstat(artifact.Path)
		if !allowed || data[name] != nil || !regexp.MustCompile("^[0-9]{1,24}$").MatchString(artifact.RunID) ||
			(runID != "" && runID != artifact.RunID) || err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > int64(limit) {
			return fmt.Errorf("PREVIEW_ARTIFACTS: missing, duplicate, mixed-run or invalid original")
		}
		raw, err := os.ReadFile(artifact.Path)
		if err != nil || len(raw) > limit || nativeEncodingDigest(raw) != artifact.SHA256 {
			return fmt.Errorf("PREVIEW_ARTIFACT_DIGEST: original bytes differ")
		}
		data[name], hashes[name], runID = raw, artifact.SHA256, artifact.RunID
	}
	matches, attempt := 0, 0
	for _, run := range manifest.Runs {
		if run.RunID != runID {
			continue
		}
		matches++
		attempt = run.Attempt
		if run.Repository != "davidpovarsky/cherri" || run.WorkflowName != "iOS Build" ||
			run.HeadSHA != record.ImplementationSHA || run.CheckoutSHA != record.ImplementationSHA ||
			(run.Conclusion != "success" && run.Conclusion != "SUCCESS") || run.Attempt < 1 || run.Attempt > 9999 ||
			(run.Event != "push" && run.Event != "workflow_dispatch") {
			return fmt.Errorf("PREVIEW_RUN: successful exact candidate checkout and attempt required")
		}
	}
	if matches != 1 {
		return fmt.Errorf("PREVIEW_RUN: absent or duplicate original run")
	}
	if err := validatePreviewNativeContract(data["preview-document-contract.json"]); err != nil {
		return err
	}
	observations, err := parsePreviewNativeLog(data["xcodetest.log"])
	if err != nil {
		return err
	}
	proof, err := decodeNativeEncodingObject(data["preview-native-proof.json"])
	pins, pinErr := previewNativeCurrentPins()
	if err != nil || pinErr != nil || len(proof) != 11 || proof["schema"] != "cherri-native-preview-proof/1" ||
		proof["implementationSha"] != record.ImplementationSHA || proof["runId"] != runID ||
		!nativeEncodingInteger(proof["runAttempt"], attempt) || !nativeEncodingInteger(proof["xcodebuildExitCode"], 0) ||
		proof["logSha256"] != hashes["xcodetest.log"] || proof["contractSha256"] != previewNativeContractSHA ||
		proof["originalGateSha256"] != previewNativeGateSHA || proof["noCrossTierInheritance"] != true ||
		!reflect.DeepEqual(proof["sourcePins"], pins) || !previewNativeObservationsEqual(proof["observations"], observations) {
		return fmt.Errorf("PREVIEW_PROOF: source, attempt, process or original observations differ")
	}
	diagnostic, err := decodeNativeEncodingObject(data["ios-test-diagnostics.json"])
	if err != nil || diagnostic["schemaVersion"] != "1.0" || diagnostic["implementationSha"] != record.ImplementationSHA ||
		diagnostic["runId"] != runID || !nativeEncodingInteger(diagnostic["xcodebuildExitCode"], 0) ||
		diagnostic["runStatus"] != "PASSED" || diagnostic["rawLogPresent"] != true || diagnostic["diagnosticOnly"] != false ||
		diagnostic["sanitizedLogSha256"] != hashes["xcodetest.log"] {
		return fmt.Errorf("PREVIEW_DIAGNOSTIC: actual complete Xcode process differs")
	}
	records := previewNativeTestIDs("BRG28")
	sort.Strings(records)
	expectedSummary := map[string]any{"status": "VERIFIED_NATIVE_PREVIEW", "errorCodes": []any{}, "noCrossTierInheritance": true,
		"runAttempt": json.Number(strconv.Itoa(attempt)), "contractSha256": previewNativeContractSHA, "originalGateSha256": previewNativeGateSHA,
		"proofSha256": hashes["preview-native-proof.json"], "verifiedTests": previewNativeStrings(previewNativeMethods), "recordTestIds": previewNativeStrings(records)}
	if !reflect.DeepEqual(diagnostic["previewNativeEvidence"], expectedSummary) {
		return fmt.Errorf("PREVIEW_DIAGNOSTIC: dedicated complete scope differs")
	}
	results, ok := diagnostic["testResults"].([]any)
	seen := map[string]bool{}
	if !ok || len(results) == 0 {
		return fmt.Errorf("PREVIEW_DIAGNOSTIC: original test outcomes absent")
	}
	for _, row := range results {
		value, ok := row.(map[string]any)
		if !ok {
			return fmt.Errorf("PREVIEW_DIAGNOSTIC: malformed outcome")
		}
		test, named := value["testId"].(string)
		if !named || test == "" || seen[test] || value["observedStatus"] != "passed" {
			return fmt.Errorf("PREVIEW_DIAGNOSTIC: duplicate, failed or incomplete outcome")
		}
		seen[test] = true
	}
	for _, test := range previewNativeMethods {
		if !seen[test] {
			return fmt.Errorf("PREVIEW_DIAGNOSTIC: original dedicated method outcome missing")
		}
	}
	return nil
}
