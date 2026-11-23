package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPublicationSecurityIndependentContext(t *testing.T) {
	for _, key := range []string{"CHERRI_CI04_EXPECTED", "CHERRI_CI04_EXPECTED_SHA256", "CHERRI_CI04_MANIFEST"} {
		t.Setenv(key, "")
	}
	runner := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
	runner.runPublicationSecurityRequirement()
	if runner.results["CI04"].Status != "NOT_RUN" || len(runner.records) != 0 || !runner.publicationSecurityEvaluated {
		t.Fatal("absent independent scope acquired credit")
	}
	row := ExecutionAuditRow{ID: "CI04", Blocker: "historical smoke"}
	applyPublicationSecurityAudit(&row, runner)
	if row.Scope != publicationSecurityScope || row.Blocker != runner.results["CI04"].Detail || row.PositiveAndNegative {
		t.Fatal("dedicated absence relabeled by generic audit")
	}
	t.Run("partial caller context fails closed", func(t *testing.T) {
		t.Setenv("CHERRI_CI04_EXPECTED", "missing-context")
		runner := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
		runner.runPublicationSecurityRequirement()
		if runner.results["CI04"].Status != "FAILED" {
			t.Fatal("malformed configured authority treated as success or absence")
		}
	})
	t.Run("binding-only context does not need final ledger", func(t *testing.T) {
		fixture := finalProvenanceFixtureNew(t)
		fixture.expected.Ledger = nil
		fixture.manifest.Ledger = nil
		fixture.manifest.Limitations = nil
		manifest := publicationSecurityManifest{Schema: "cherri-ci04-locators/1", Bindings: fixture.manifest}
		mraw, _ := json.Marshal(manifest)
		if err := os.WriteFile(fixture.config.ManifestPath, mraw, 0600); err != nil {
			t.Fatal(err)
		}
		expected := publicationSecurityExpected{Schema: "cherri-ci04-expected/1", ManifestSHA256: ciIdentityDigest(mraw), Bindings: fixture.expected}
		eraw, _ := json.Marshal(expected)
		if err := os.WriteFile(fixture.config.ExpectedPath, eraw, 0600); err != nil {
			t.Fatal(err)
		}
		config := publicationSecurityConfig{fixture.config.ExpectedPath, ciIdentityDigest(eraw), fixture.config.ManifestPath}
		if _, _, err := publicationSecurityLoad(config, fixture.output); err != nil {
			t.Fatal(err)
		}
		config.ExpectedSHA256 = strings.Repeat("f", 64)
		if _, _, err := publicationSecurityLoad(config, fixture.output); err == nil {
			t.Fatal("candidate-only expected authority accepted")
		}
	})
	t.Run("historical filename smoke cannot replace scope", func(t *testing.T) {
		runner := &Runner{results: map[string]CaseResult{}}
		runner.Record("CI04", "verification", "privacy", "security+repository", []string{"security", "repository"}, true, "PASSED", "private_key.pem absent")
		if runner.results["CI04"].Status != "NOT_RUN" || len(runner.records) != 0 {
			t.Fatal("single filename absence acquired privacy credit")
		}
		if err := validatePublicationSecurityRecord("BRG31", EvidenceRecord{TestID: publicationSecurityTestID}); err == nil {
			t.Fatal("scoped privacy evidence acquired combined gate credit")
		}
	})
	t.Run("known raw inventory is independently pinned and exact", func(t *testing.T) {
		in, root := ciTestInputs(t)
		expected := publicationSecurityExpected{KnownRawInventory: "known-raw", KnownRawCount: 1}
		ciTestObject(t, in, root, "known-raw", publicationRawInventory{Schema: "cherri-known-raw-digests/1", Files: []finalProvenanceFile{{strings.Repeat("a", 64), 123}}})
		if values, err := publicationKnownRaw(in, expected); err != nil || values[strings.Repeat("a", 64)] != 123 {
			t.Fatalf("exact digest inventory: %v", err)
		}
		expected.KnownRawCount = 0
		_, err := publicationKnownRaw(in, expected)
		var pending finalProvenancePending
		if !errors.As(err, &pending) {
			t.Fatalf("missing authority treated as a clean scan: %v", err)
		}
		expected.KnownRawCount = 2
		ciTestObject(t, in, root, "known-raw", publicationRawInventory{Schema: "cherri-known-raw-digests/1", Files: []finalProvenanceFile{{strings.Repeat("a", 64), 123}, {strings.Repeat("a", 64), 123}}})
		if _, err := publicationKnownRaw(in, expected); err == nil || err.Error() != "CI04_KNOWN_RAW_DUPLICATE" {
			t.Fatalf("duplicate digest count accepted: %v", err)
		}
	})
}

// Synthetic originals exercise the intake boundary. No private corpus, signer,
// provider, native workflow, public-key operation or network request is used.
func publicationTestInventory(t *testing.T, in *finalProvenanceInputs, originals, role string) string {
	t.Helper()
	source := in.Expected.Sources[role]
	root := in.Manifest.SourceDirectories[role]
	tree := finalProvenanceTestGit(t, root, "ls-tree", "-r", "-z", source.SHA)
	document := publicationSourceInventory{Schema: "cherri-approved-public-source/1", SourceSHA: source.SHA, Files: map[string]publicationSourceMember{}}
	for _, line := range strings.Split(tree, "\x00") {
		if line == "" {
			continue
		}
		pair := strings.SplitN(line, "\t", 2)
		if len(pair) != 2 {
			t.Fatal("fixture Git tree")
		}
		fields := strings.Fields(pair[0])
		if len(fields) != 3 {
			t.Fatal("fixture tree fields")
		}
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(pair[1])))
		if err != nil {
			t.Fatal(err)
		}
		document.Files[pair[1]] = publicationSourceMember{ciIdentityDigest(data), int64(len(data)), fields[2], fields[0]}
	}
	id := role + "-approved-source"
	ciTestObject(t, in, originals, id, document)
	return id
}

func TestPublicationSecuritySourceAndRawBoundaries(t *testing.T) {
	in, originals := ciTestInputs(t)
	expected := publicationSecurityExpected{SourceInventories: map[string]string{}}
	for _, role := range []string{"implementation", "compiler", "docs"} {
		root := t.TempDir()
		finalProvenanceTestGit(t, root, "init", "--template=", "-b", "fixture")
		if err := os.WriteFile(filepath.Join(root, "public.txt"), []byte("owned public "+role+" control"), 0600); err != nil {
			t.Fatal(err)
		}
		finalProvenanceTestGit(t, root, "add", ".")
		finalProvenanceTestGit(t, root, "commit", "-m", "Synthetic public boundary")
		in.Expected.Sources[role] = finalProvenanceSource{SHA: finalProvenanceTestGit(t, root, "rev-parse", "HEAD")}
		in.Manifest.SourceDirectories[role] = root
		expected.SourceInventories[role] = publicationTestInventory(t, in, originals, role)
	}
	t.Run("actual three immutable Git inventories", func(t *testing.T) {
		count, err := publicationValidateSource(context.Background(), in, expected, map[string]int64{})
		if err != nil || count != 3 {
			t.Fatalf("public source join: %d %v", count, err)
		}
	})
	t.Run("known raw whole file hidden under public name", func(t *testing.T) {
		data := []byte("owned public implementation control")
		_, err := publicationValidateSource(context.Background(), in, expected, map[string]int64{ciIdentityDigest(data): int64(len(data))})
		if err == nil || err.Error() != "CI04_KNOWN_RAW_CORPUS_IN_SOURCE" {
			t.Fatalf("raw digest rejection was not causal: %v", err)
		}
	})
	t.Run("independently approved inventory cannot omit actual source", func(t *testing.T) {
		id := expected.SourceInventories["implementation"]
		original := append([]byte{}, in.Bytes[id]...)
		defer ciTestPut(t, in, originals, id, original)
		var document publicationSourceInventory
		if err := json.Unmarshal(original, &document); err != nil {
			t.Fatal(err)
		}
		delete(document.Files, "public.txt")
		document.Files["other.txt"] = publicationSourceMember{strings.Repeat("a", 64), 1, strings.Repeat("b", 40), "100644"}
		ciTestObject(t, in, originals, id, document)
		if _, err := publicationValidateSource(context.Background(), in, expected, map[string]int64{}); err == nil {
			t.Fatal("incomplete approved tree accepted")
		}
	})
	t.Run("partial source cannot trigger object download", func(t *testing.T) {
		root := in.Manifest.SourceDirectories["implementation"]
		finalProvenanceTestGit(t, root, "config", "remote.fixture.promisor", "true")
		defer finalProvenanceTestGit(t, root, "config", "--unset", "remote.fixture.promisor")
		if _, err := publicationValidateSource(context.Background(), in, expected, map[string]int64{}); err == nil || err.Error() != "CI04_PARTIAL_SOURCE_UNSUPPORTED" {
			t.Fatalf("partial-source boundary: %v", err)
		}
	})
	t.Run("full entry rejects partial docs before first source snapshot", func(t *testing.T) {
		fixture := finalProvenanceFixtureNew(t)
		fixture.expected.Ledger = nil
		fixture.manifest.Ledger = nil
		fixture.manifest.Limitations = nil
		manifest := publicationSecurityManifest{Schema: "cherri-ci04-locators/1", Bindings: fixture.manifest}
		mraw, _ := json.Marshal(manifest)
		if err := os.WriteFile(fixture.config.ManifestPath, mraw, 0600); err != nil {
			t.Fatal(err)
		}
		expected := publicationSecurityExpected{Schema: "cherri-ci04-expected/1", ManifestSHA256: ciIdentityDigest(mraw), Bindings: fixture.expected}
		eraw, _ := json.Marshal(expected)
		if err := os.WriteFile(fixture.config.ExpectedPath, eraw, 0600); err != nil {
			t.Fatal(err)
		}
		config := publicationSecurityConfig{fixture.config.ExpectedPath, ciIdentityDigest(eraw), fixture.config.ManifestPath}
		finalProvenanceTestGit(t, fixture.manifest.SourceDirectories["docs"], "config", "remote.fixture.promisor", "true")
		if err := os.WriteFile(filepath.Join(fixture.manifest.SourceDirectories["implementation"], "tracked.txt"), []byte("deliberately dirty before any snapshot"), 0600); err != nil {
			t.Fatal(err)
		}
		observed, err := executePublicationSecurity(config, fixture.expected.Sources["implementation"].SHA, fixture.output, nil)
		if err == nil || err.Error() != "CI04_PARTIAL_SOURCE_UNSUPPORTED" || observed.Status != "FAILED" || observed.RP43 != nil {
			t.Fatalf("full entry did not reject partial docs before the dirty implementation snapshot: %v %+v", err, observed)
		}
	})
}

func publicationTestMembers(pins map[string]ciArchiveMember, class string) map[string]publicationPublicMember {
	result := map[string]publicationPublicMember{}
	for name, pin := range pins {
		result[name] = publicationPublicMember{SHA256: pin.SHA256, Bytes: pin.Bytes, Mode: pin.Mode, Class: class}
	}
	return result
}

func TestPublicationSecurityArchiveBoundaries(t *testing.T) {
	public := []byte("SYNTHETIC_PRIVATE_CANARY_FOR_DIGEST_CONTROL")
	raw, pins := ciTestZIP(t, map[string][]byte{"renamed-public.txt": public}, zip.Deflate)
	inventory := publicationTestMembers(pins, "reviewed-synthetic-fixture")
	t.Run("reviewed synthetic bytes", func(t *testing.T) {
		if _, count, err := publicationInspectMembers(raw, inventory, map[string]int64{}, 0, &ciExpandedBudget{Limit: 1000}); err != nil || count != 1 {
			t.Fatal(err)
		}
	})
	t.Run("known raw bytes rejected despite public class", func(t *testing.T) {
		_, _, err := publicationInspectMembers(raw, inventory, map[string]int64{ciIdentityDigest(public): int64(len(public))}, 0, &ciExpandedBudget{Limit: 1000})
		if err == nil || err.Error() != "CI04_KNOWN_RAW_CORPUS_IN_PUBLIC_ARTIFACT" {
			t.Fatalf("raw control hit another predicate: %v", err)
		}
	})
	t.Run("signed header without explicit signing class", func(t *testing.T) {
		raw, pins := ciTestZIP(t, map[string][]byte{"result.shortcut": []byte("AEA1synthetic protocol only")}, zip.Store)
		_, _, err := publicationInspectMembers(raw, publicationTestMembers(pins, "public-metadata"), map[string]int64{}, 0, &ciExpandedBudget{Limit: 1000})
		if err == nil || err.Error() != "CI04_SIGNED_BYTES_WITHOUT_EXPLICIT_INTENT_CLASS" {
			t.Fatalf("unsigned label masked signing: %v", err)
		}
	})
	for _, class := range []string{"authorized-signed-fixture", "sealed-private-ciphertext"} {
		t.Run("unsupported nested "+class, func(t *testing.T) {
			inner, innerPins := ciTestZIP(t, map[string][]byte{"leaf": []byte("bounded synthetic leaf")}, zip.Store)
			outer, outerPins := ciTestZIP(t, map[string][]byte{"nested.zip": inner}, zip.Deflate)
			inventory := publicationTestMembers(outerPins, "archive")
			item := inventory["nested.zip"]
			item.ArchiveFormat = "zip"
			item.Members = publicationTestMembers(innerPins, class)
			inventory["nested.zip"] = item
			_, _, err := publicationInspectMembers(outer, inventory, map[string]int64{}, 0, &ciExpandedBudget{Limit: 10000})
			if err == nil || err.Error() != "CI04_NESTED_SIGNING_OR_SEALING_UNSUPPORTED" {
				t.Fatalf("nested special leaf escaped its validator: %v", err)
			}
		})
	}
	t.Run("nested expanded aggregate shared across containers", func(t *testing.T) {
		inner, innerPins := ciTestZIP(t, map[string][]byte{"leaf": []byte("1234")}, zip.Deflate)
		outer, outerPins := ciTestZIP(t, map[string][]byte{"nested.zip": inner}, zip.Deflate)
		inventory := publicationTestMembers(outerPins, "archive")
		item := inventory["nested.zip"]
		item.ArchiveFormat = "zip"
		item.Members = publicationTestMembers(innerPins, "public-metadata")
		inventory["nested.zip"] = item
		_, _, err := publicationInspectMembers(outer, inventory, map[string]int64{}, 0, &ciExpandedBudget{Limit: int64(len(inner) + 3)})
		if err == nil || err.Error() != "CI_AGGREGATE_EXPANDED_LIMIT" {
			t.Fatalf("nested aggregate bound not causal: %v", err)
		}
	})
	t.Run("observed nested ZIP directory is exact and zero sized", func(t *testing.T) {
		var data bytes.Buffer
		writer := zip.NewWriter(&data)
		dir := &zip.FileHeader{Name: "public-kit/", Method: zip.Store}
		dir.SetMode(os.ModeDir | 0755)
		if _, err := writer.CreateHeader(dir); err != nil {
			t.Fatal(err)
		}
		file := &zip.FileHeader{Name: "public-kit/asset.txt", Method: zip.Deflate}
		file.SetMode(0644)
		stream, err := writer.CreateHeader(file)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = stream.Write([]byte("safe")); err != nil {
			t.Fatal(err)
		}
		if err = writer.Close(); err != nil {
			t.Fatal(err)
		}
		inventory := map[string]publicationPublicMember{"public-kit/": {SHA256: ciIdentityDigest(nil), Mode: 0755, Class: "reviewed-public-directory"}, "public-kit/asset.txt": {SHA256: ciIdentityDigest([]byte("safe")), Bytes: 4, Mode: 0644, Class: "reviewed-synthetic-fixture"}}
		if _, count, err := publicationInspectMembers(data.Bytes(), inventory, map[string]int64{}, 2, &ciExpandedBudget{Limit: 4}); err != nil || count != 2 {
			t.Fatalf("observed ZIP directory convention: %v", err)
		}
		pin := inventory["public-kit/"]
		pin.Class = "public-metadata"
		inventory["public-kit/"] = pin
		if _, err := publicationZIPArchive(data.Bytes(), inventory, &ciExpandedBudget{Limit: 4}); err == nil || err.Error() != "CI04_ZIP_DIRECTORY" {
			t.Fatalf("unreviewed directory class accepted: %v", err)
		}
		regularOnly := map[string]ciArchiveMember{}
		for name, row := range inventory {
			regularOnly[name] = ciArchiveMember{row.SHA256, row.Bytes, row.Mode}
		}
		if _, err := ciIdentityArchive(data.Bytes(), regularOnly, &ciExpandedBudget{Limit: 4}); err == nil {
			t.Fatal("CI03 regular-only archive boundary was broadened")
		}
	})
	t.Run("ZIP directory file alias and symlink reject", func(t *testing.T) {
		for _, kind := range []string{"alias", "symlink"} {
			t.Run(kind, func(t *testing.T) {
				var data bytes.Buffer
				writer := zip.NewWriter(&data)
				inventory := map[string]publicationPublicMember{}
				entries := []struct {
					name  string
					mode  os.FileMode
					class string
					body  []byte
				}{{"same", 0644, "public-metadata", []byte("x")}, {"same/", os.ModeDir | 0755, "reviewed-public-directory", nil}}
				if kind == "symlink" {
					entries = []struct {
						name  string
						mode  os.FileMode
						class string
						body  []byte
					}{{"link", os.ModeSymlink | 0777, "public-metadata", []byte("target")}}
				}
				for _, entry := range entries {
					header := &zip.FileHeader{Name: entry.name, Method: zip.Store}
					header.SetMode(entry.mode)
					stream, err := writer.CreateHeader(header)
					if err != nil {
						t.Fatal(err)
					}
					if len(entry.body) > 0 {
						if _, err = stream.Write(entry.body); err != nil {
							t.Fatal(err)
						}
					}
					inventory[entry.name] = publicationPublicMember{SHA256: ciIdentityDigest(entry.body), Bytes: int64(len(entry.body)), Mode: uint32(entry.mode.Perm()), Class: entry.class}
				}
				if err := writer.Close(); err != nil {
					t.Fatal(err)
				}
				if _, err := publicationZIPArchive(data.Bytes(), inventory, &ciExpandedBudget{Limit: 100}); err == nil {
					t.Fatal("unsafe ZIP type/alias accepted")
				}
			})
		}
	})
}

type publicationTestTarMember struct {
	Name string
	Type byte
	Body []byte
	Link string
}

func publicationTestTAR(t *testing.T, rows []publicationTestTarMember) ([]byte, map[string]publicationPublicMember) {
	t.Helper()
	var output bytes.Buffer
	writer := tar.NewWriter(&output)
	inventory := map[string]publicationPublicMember{}
	for _, row := range rows {
		mode := int64(0644)
		class := "public-metadata"
		if row.Type == tar.TypeDir {
			mode = 0755
			class = "reviewed-public-directory"
		}
		header := &tar.Header{Name: row.Name, Typeflag: row.Type, Mode: mode, Size: int64(len(row.Body)), Linkname: row.Link}
		if err := writer.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if len(row.Body) > 0 {
			if _, err := writer.Write(row.Body); err != nil {
				t.Fatal(err)
			}
		}
		inventory[row.Name] = publicationPublicMember{SHA256: ciIdentityDigest(row.Body), Bytes: int64(len(row.Body)), Mode: uint32(mode), Class: class}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return output.Bytes(), inventory
}

func TestPublicationSecurityPagesTAR(t *testing.T) {
	canary := []byte("SYNTHETIC_KNOWN_RAW_TAR_CONTROL")
	rows := []publicationTestTarMember{{".", tar.TypeDir, nil, ""}, {"./site", tar.TypeDir, nil, ""}, {"./site/public.txt", tar.TypeReg, canary, ""}}
	t.Run("original Pages shape and exact directories", func(t *testing.T) {
		data, pins := publicationTestTAR(t, rows)
		if _, count, err := publicationInspectContainer(data, pins, map[string]int64{}, 1, &ciExpandedBudget{Limit: int64(len(canary))}, "tar"); err != nil || count != 3 {
			t.Fatalf("bounded original TAR shape: %d %v", count, err)
		}
	})
	t.Run("raw file inside TAR is not opaque", func(t *testing.T) {
		data, pins := publicationTestTAR(t, rows)
		_, _, err := publicationInspectContainer(data, pins, map[string]int64{ciIdentityDigest(canary): int64(len(canary))}, 1, &ciExpandedBudget{Limit: 1000}, "tar")
		if err == nil || err.Error() != "CI04_KNOWN_RAW_CORPUS_IN_PUBLIC_ARTIFACT" {
			t.Fatalf("hidden raw control was not causal: %v", err)
		}
	})
	t.Run("ZIP and nested TAR share expanded cap", func(t *testing.T) {
		data, pins := publicationTestTAR(t, rows)
		outer, outerPins := ciTestZIP(t, map[string][]byte{"artifact.tar": data}, zip.Deflate)
		inventory := publicationTestMembers(outerPins, "archive")
		item := inventory["artifact.tar"]
		item.ArchiveFormat = "tar"
		item.Members = pins
		inventory["artifact.tar"] = item
		_, _, err := publicationInspectMembers(outer, inventory, map[string]int64{}, 0, &ciExpandedBudget{Limit: int64(len(data) + len(canary) - 1)})
		if err == nil || err.Error() != "CI_AGGREGATE_EXPANDED_LIMIT" {
			t.Fatalf("TAR aggregate cap was not causal: %v", err)
		}
	})
	t.Run("container cannot be relabeled public report", func(t *testing.T) {
		data, _ := publicationTestTAR(t, rows)
		outer, pins := ciTestZIP(t, map[string][]byte{"artifact.tar": data}, zip.Store)
		_, _, err := publicationInspectMembers(outer, publicationTestMembers(pins, "sanitized-public-report"), map[string]int64{}, 0, &ciExpandedBudget{Limit: 100000})
		var pending finalProvenancePending
		if !errors.As(err, &pending) {
			t.Fatalf("opaque TAR falsely qualified: %v", err)
		}
	})
	for _, control := range []publicationTestTarMember{{"../escape", tar.TypeReg, []byte("x"), ""}, {"/absolute", tar.TypeReg, []byte("x"), ""}, {"link", tar.TypeSymlink, nil, "target"}, {"hard", tar.TypeLink, nil, "target"}, {"device", tar.TypeChar, nil, ""}} {
		t.Run("reject "+control.Name, func(t *testing.T) {
			data, pins := publicationTestTAR(t, []publicationTestTarMember{control})
			if _, err := publicationTARArchive(data, pins, &ciExpandedBudget{Limit: 1000}); err == nil {
				t.Fatal("unsafe TAR path/type accepted")
			}
		})
	}
	t.Run("normalized duplicate member", func(t *testing.T) {
		data, pins := publicationTestTAR(t, []publicationTestTarMember{{"same", tar.TypeReg, []byte("x"), ""}, {"./same", tar.TypeReg, []byte("x"), ""}})
		if _, err := publicationTARArchive(data, pins, &ciExpandedBudget{Limit: 1000}); err == nil {
			t.Fatal("TAR alias collision accepted")
		}
	})
	t.Run("second archive after terminator", func(t *testing.T) {
		data, pins := publicationTestTAR(t, rows)
		tail, _ := publicationTestTAR(t, []publicationTestTarMember{{"unlisted", tar.TypeReg, []byte("x"), ""}})
		if _, err := publicationTARArchive(append(data, tail...), pins, &ciExpandedBudget{Limit: 1000}); err == nil || err.Error() != "CI04_TAR_TRAILING_CONTENT" {
			t.Fatalf("concatenated TAR not rejected: %v", err)
		}
	})
}

func publicationTestNativeFixture(t *testing.T) (*finalProvenanceInputs, publicationSecurityExpected, map[string]map[string][]byte, string) {
	t.Helper()
	in, originals := ciTestInputs(t)
	root := t.TempDir()
	finalProvenanceTestGit(t, root, "init", "--template=", "-b", "fixture")
	fixtures := []string{"CherriDictionaryRuntimePOC", "CherriLegacyIfPOC", "CherriMenuRuntimePOC", "CherriRuntimePOC", "CherriS01F01CoveragePOC"}
	paths := []string{".github/workflows/ios27-shortcuts-runtime-poc.yml", ".github/workflows/ios-build.yml", "ios/CherriTests/CherriEditorInputTests.swift", "ios/CherriUITests/CherriEditorUITests.swift", "ios/CherriUITests/EditorStartupDeadline.swift", "ios/project.yml", "ios/scripts/collect_ios_xcresult_summary.py", "ios/scripts/test_ios_xcresult_summary.py", "ios/scripts/verify_ios_xcresult_summary.py", "tools/shortcuts-simulator-qa/project_prerequisite.py", "tools/shortcuts-simulator-qa/project_prerequisite_observation.py", "tools/shortcuts-simulator-qa/qa_terminal_receipts.py", "tools/shortcuts-simulator-qa/simulator_session_recovery.py", "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"}
	for _, id := range fixtures {
		paths = append(paths, "tests/runtime_poc/"+id+".cherri")
	}
	for _, name := range paths {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("owned synthetic "+name+"\n"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	finalProvenanceTestGit(t, root, "add", ".")
	finalProvenanceTestGit(t, root, "commit", "-m", "Synthetic public source and authority")
	source := finalProvenanceSource{Repository: "davidpovarsky/cherri", SHA: finalProvenanceTestGit(t, root, "rev-parse", "HEAD"), Tree: finalProvenanceTestGit(t, root, "rev-parse", "HEAD^{tree}"), LocalRef: "refs/heads/fixture", PublishedRef: "refs/heads/fixture"}
	in.Expected.Sources["implementation"] = source
	in.Manifest.SourceDirectories["implementation"] = root
	run := finalProvenanceRun{Original: "run", Repository: source.Repository, ID: 1001, Attempt: 1, WorkflowID: 375700807, WorkflowPath: ".github/workflows/ios27-shortcuts-runtime-poc.yml", Event: "workflow_dispatch", SourceSHA: source.SHA, Status: "completed", Conclusion: "failure"}
	in.Expected.Runs["runtime"] = run
	actualRun := map[string]any{"id": run.ID, "repository": map[string]any{"full_name": run.Repository}, "head_sha": source.SHA, "head_branch": "fixture", "workflow_id": run.WorkflowID, "path": run.WorkflowPath, "event": run.Event, "run_attempt": 1, "created_at": "2020-01-01T00:01:00Z", "run_started_at": "2020-01-01T00:01:00Z", "updated_at": "2020-01-01T00:05:00Z"}
	priorRun := map[string]any{"id": 800, "repository": map[string]any{"full_name": run.Repository}, "head_sha": source.SHA, "head_branch": "fixture", "workflow_id": 337107928}
	ciTestObject(t, in, originals, "run", actualRun)
	ciTestObject(t, in, originals, "pre-runs", map[string]any{"total_count": 1, "workflow_runs": []any{priorRun}})
	ciTestObject(t, in, originals, "binding-runs", map[string]any{"total_count": 2, "workflow_runs": []any{actualRun, priorRun}})
	request := map[string]any{"ref": "fixture", "inputs": map[string]any{"allow_public_fixture_hubsign": true}}
	ciTestObject(t, in, originals, "request", request)
	reviewed := map[string]string{}
	for _, id := range fixtures {
		body, err := os.ReadFile(filepath.Join(root, "tests/runtime_poc/"+id+".cherri"))
		if err != nil {
			t.Fatal(err)
		}
		reviewed[id] = ciIdentityDigest(body)
	}
	workflow, _ := os.ReadFile(filepath.Join(root, filepath.FromSlash(run.WorkflowPath)))
	ciTestObject(t, in, originals, "plan", map[string]any{"schema": "synthetic-native-plan/1", "sourceCommit": source.SHA, "sourceTree": source.Tree, "ref": "fixture", "runtimePoc": map[string]any{"payload": request, "workflowId": run.WorkflowID, "workflowPath": run.WorkflowPath, "workflowSha256": ciIdentityDigest(workflow), "publicFixtureCount": 5, "reviewedPublicFixtureHashes": reviewed}})
	ciTestObject(t, in, originals, "handoff", map[string]any{"schema": "synthetic-native-handoff/1", "sourceCommit": source.SHA, "sourceTree": source.Tree, "ref": "fixture", "planSha256": in.Expected.Files["plan"].SHA256, "fullScriptAndPlanReviewed": true, "remoteRefVerified": true, "singlePostPerWorkflow": true, "nativeRerunsOrCancellation": false, "allowedCases": []string{"ios27-runtime-poc"}, "recordedUTC": "2020-01-01T00:00:00Z"})
	ciTestObject(t, in, originals, "intent", map[string]any{"sourceCommit": source.SHA, "workflowId": run.WorkflowID, "case": "ios27-runtime-poc", "ref": "fixture", "onePostOnly": true, "beforePostUtc": "2020-01-01T00:01:00Z", "priorRunIds": []int{800}})
	ciTestObject(t, in, originals, "dispatch-result", map[string]any{"postCount": 1, "returncode": 0, "accepted204": true, "recordedUtc": "2020-01-01T00:01:01Z"})
	ciTestObject(t, in, originals, "binding", map[string]any{"sourceCommit": source.SHA, "sourceTree": source.Tree, "runId": run.ID, "attempt": run.Attempt, "workflowId": run.WorkflowID, "workflowPath": run.WorkflowPath, "dispatchCount": 1, "case": "ios27-runtime-poc", "event": run.Event, "headBranch": "fixture", "requestSha256": in.Expected.Files["request"].SHA256, "inputs": request["inputs"], "createdAt": actualRun["created_at"], "boundUtc": "2020-01-01T00:01:02Z"})
	expected := publicationSecurityExpected{PublicArchives: map[string]map[string]publicationPublicMember{"signed": {}}, SigningAuthorities: map[string]publicationSigningAuthority{"runtime": {Format: "root-reviewed-native-plan/1", HandoffSchema: "synthetic-native-handoff/1", PlanSchema: "synthetic-native-plan/1", Handoff: "handoff", Plan: "plan", Request: "request", Intent: "intent", Result: "dispatch-result", Binding: "binding", PreDispatchRuns: "pre-runs", BindingRuns: "binding-runs"}}, SealedProofs: map[string]publicationSealedProof{}}
	archives := map[string]map[string][]byte{"signed": {}}
	in.Expected.Artifacts["signed"] = finalProvenanceArtifact{Run: "runtime", SourceSHA: source.SHA, Role: "implementation", Availability: "github"}
	var signingRows, sourceRows []any
	for _, id := range fixtures {
		name := id + ".shortcut"
		body := []byte("AEA1synthetic non-Apple bytes " + id)
		archives["signed"][name] = body
		expected.PublicArchives["signed"][name] = publicationPublicMember{SHA256: ciIdentityDigest(body), Bytes: int64(len(body)), Mode: 0644, Class: "authorized-signed-fixture"}
		expected.SigningIntents = append(expected.SigningIntents, publicationSigningIntent{Artifact: "signed", Member: name, SignedSHA256: ciIdentityDigest(body), FixtureSHA256: reviewed[id], Method: "hubsign", ProvenanceMember: "signing.json", SourceProvenanceMember: "sources.json", FixtureID: id})
		signingRows = append(signingRows, map[string]any{"fixtureId": id, "signedArtifact": name, "sourceSha256": reviewed[id], "unsignedSha256": ciIdentityDigest([]byte("unsigned " + id)), "signedSha256": ciIdentityDigest(body), "signingMethod": "hubsign", "signedMagicVerified": true, "exactFileSigningReceiptSha256": strings.Repeat("a", 64), "signingVerificationTier": "AEA1_HEADER_PASS"})
		sourceRows = append(sourceRows, map[string]any{"fixtureId": id, "sourcePath": "tests/runtime_poc/" + id + ".cherri", "sourceSha256": reviewed[id]})
	}
	for name, object := range map[string]any{"signing.json": map[string]any{"schemaVersion": "1", "implementationSha": source.SHA, "runId": "1001", "runAttempt": "1", "fixtures": signingRows}, "sources.json": map[string]any{"schemaVersion": "1", "implementationSha": source.SHA, "runId": "1001", "runAttempt": "1", "syntheticSourcesVerified": true, "sources": sourceRows}} {
		body, err := json.Marshal(object)
		if err != nil {
			t.Fatal(err)
		}
		archives["signed"][name] = body
		expected.PublicArchives["signed"][name] = publicationPublicMember{SHA256: ciIdentityDigest(body), Bytes: int64(len(body)), Mode: 0644, Class: "public-metadata"}
	}
	return in, expected, archives, originals
}

func publicationTestRunTimes(t *testing.T, in *finalProvenanceInputs, root, created, started string) {
	t.Helper()
	run, err := in.object("run")
	if err != nil {
		t.Fatal(err)
	}
	run["created_at"], run["run_started_at"] = created, started
	ciTestObject(t, in, root, "run", run)
	binding, err := in.object("binding")
	if err != nil {
		t.Fatal(err)
	}
	binding["createdAt"] = created
	ciTestObject(t, in, root, "binding", binding)
	listing, err := in.object("binding-runs")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range listing["workflow_runs"].([]any) {
		row := value.(map[string]any)
		id, _ := finalProvenanceNumber(row, "id")
		if id == in.Expected.Runs["runtime"].ID {
			row["created_at"], row["run_started_at"] = created, started
		}
	}
	ciTestObject(t, in, root, "binding-runs", listing)
}

func TestPublicationSecurityExplicitSigningAuthority(t *testing.T) {
	controls := []struct {
		name              string
		change            func(*testing.T, *finalProvenanceInputs, *publicationSecurityExpected, map[string]map[string][]byte, string)
		positive, pending bool
	}{
		{"actual source plus complete synthetic original authority", nil, true, false},
		{"missing independent authority", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			e.SigningAuthorities = nil
		}, false, true},
		{"missing original prelisting", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			authority := e.SigningAuthorities["runtime"]
			authority.PreDispatchRuns = ""
			e.SigningAuthorities["runtime"] = authority
		}, false, true},
		{"missing original binding list", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			authority := e.SigningAuthorities["runtime"]
			authority.BindingRuns = ""
			e.SigningAuthorities["runtime"] = authority
		}, false, true},
		{"explicit request false", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			request, _ := in.object("request")
			request["inputs"].(map[string]any)["allow_public_fixture_hubsign"] = false
			ciTestObject(t, in, r, "request", request)
			plan, _ := in.object("plan")
			plan["runtimePoc"].(map[string]any)["payload"] = request
			ciTestObject(t, in, r, "plan", plan)
			handoff, _ := in.object("handoff")
			handoff["planSha256"] = in.Expected.Files["plan"].SHA256
			ciTestObject(t, in, r, "handoff", handoff)
			binding, _ := in.object("binding")
			binding["inputs"] = request["inputs"]
			binding["requestSha256"] = in.Expected.Files["request"].SHA256
			ciTestObject(t, in, r, "binding", binding)
		}, false, false},
		{"plan hash alone is not approval", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("handoff")
			o["fullScriptAndPlanReviewed"] = false
			ciTestObject(t, in, r, "handoff", o)
		}, false, false},
		{"source rebind", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("binding")
			o["sourceCommit"] = strings.Repeat("f", 40)
			ciTestObject(t, in, r, "binding", o)
		}, false, false},
		{"duplicate post", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("dispatch-result")
			o["postCount"] = 2
			ciTestObject(t, in, r, "dispatch-result", o)
		}, false, false},
		{"authorization after dispatch", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("handoff")
			o["recordedUTC"] = "2020-01-01T00:02:00Z"
			ciTestObject(t, in, r, "handoff", o)
		}, false, false},
		{"coherent later authority cannot authorize old run", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			for _, change := range []struct{ id, key, value string }{{"handoff", "recordedUTC", "2020-01-01T00:02:00Z"}, {"intent", "beforePostUtc", "2020-01-01T00:03:00Z"}, {"dispatch-result", "recordedUtc", "2020-01-01T00:03:01Z"}, {"binding", "boundUtc", "2020-01-01T00:03:02Z"}} {
				o, _ := in.object(change.id)
				o[change.key] = change.value
				ciTestObject(t, in, r, change.id, o)
			}
		}, false, false},
		{"actual original ref mismatch", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("run")
			o["head_branch"] = "another-published-branch"
			ciTestObject(t, in, r, "run", o)
		}, false, false},
		{"intent prior IDs disagree with original list", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("intent")
			o["priorRunIds"] = []int{801}
			ciTestObject(t, in, r, "intent", o)
		}, false, false},
		{"actual run was already in coherent original prelisting", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			run, _ := in.object("run")
			listing, _ := in.object("pre-runs")
			listing["workflow_runs"] = append(listing["workflow_runs"].([]any), run)
			listing["total_count"] = 2
			ciTestObject(t, in, r, "pre-runs", listing)
			intent, _ := in.object("intent")
			intent["priorRunIds"] = []int{800, 1001}
			ciTestObject(t, in, r, "intent", intent)
		}, false, false},
		{"incomplete original prelisting", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("pre-runs")
			o["total_count"] = 2
			ciTestObject(t, in, r, "pre-runs", o)
		}, false, false},
		{"two newly observed matching runs", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			run, _ := in.object("run")
			run["id"] = 1002
			listing, _ := in.object("binding-runs")
			listing["workflow_runs"] = append(listing["workflow_runs"].([]any), run)
			listing["total_count"] = 3
			ciTestObject(t, in, r, "binding-runs", listing)
		}, false, false},
		{"seconds precision overlaps pre-POST boundary", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("intent")
			o["beforePostUtc"] = "2020-01-01T00:01:00.750Z"
			ciTestObject(t, in, r, "intent", o)
		}, true, false},
		{"fractional API precision overlaps pre-POST boundary", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("intent")
			o["beforePostUtc"] = "2020-01-01T00:01:00.750Z"
			ciTestObject(t, in, r, "intent", o)
			publicationTestRunTimes(t, in, r, "2020-01-01T00:01:00.7Z", "2020-01-01T00:01:00.79Z")
		}, true, false},
		{"queued start after binding", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			publicationTestRunTimes(t, in, r, "2020-01-01T00:01:00Z", "2020-01-01T00:04:00Z")
		}, true, false},
		{"exact original five-second allowance", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			publicationTestRunTimes(t, in, r, "2020-01-01T00:00:55Z", "2020-01-01T00:00:55Z")
		}, true, false},
		{"outside original five-second allowance", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			publicationTestRunTimes(t, in, r, "2020-01-01T00:00:54.999999999Z", "2020-01-01T00:00:55Z")
		}, false, false},
		{"binding createdAt differs from original run", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("binding")
			o["createdAt"] = "2020-01-01T00:00:59Z"
			ciTestObject(t, in, r, "binding", o)
		}, false, false},
		{"run starts entirely before creation", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			o, _ := in.object("run")
			o["run_started_at"] = "2020-01-01T00:00:59Z"
			ciTestObject(t, in, r, "run", o)
		}, false, false},
		{"one unrelated signing fixture", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			e.SigningIntents[0].FixtureSHA256 = strings.Repeat("e", 64)
		}, false, false},
		{"extra signed artifact has no intent", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			body := []byte("AEA1extra synthetic object")
			a["signed"]["extra.shortcut"] = body
			e.PublicArchives["signed"]["extra.shortcut"] = publicationPublicMember{SHA256: ciIdentityDigest(body), Bytes: int64(len(body)), Mode: 0644, Class: "authorized-signed-fixture"}
		}, false, false},
		{"signed body mutated", func(t *testing.T, in *finalProvenanceInputs, e *publicationSecurityExpected, a map[string]map[string][]byte, r string) {
			a["signed"][e.SigningIntents[0].Member] = []byte("AEA1different synthetic body")
		}, false, false},
	}
	wantErrors := map[string]string{
		"explicit request false":                                 "CI04_SIGNING_NOT_EXPLICITLY_REQUESTED",
		"coherent later authority cannot authorize old run":      "CI04_SIGNING_RUN_CREATION_OUTSIDE_DISPATCH",
		"actual original ref mismatch":                           "CI04_SIGNING_RUN_REF_MISMATCH",
		"intent prior IDs disagree with original list":           "CI04_SIGNING_PRIOR_RUN_IDS_MISMATCH",
		"actual run was already in coherent original prelisting": "CI04_SIGNING_RUN_ALREADY_PRESENT",
		"incomplete original prelisting":                         "CI04_SIGNING_RUN_LIST_INCOMPLETE",
		"two newly observed matching runs":                       "CI04_SIGNING_UNIQUE_NEW_RUN",
		"outside original five-second allowance":                 "CI04_SIGNING_RUN_CREATION_OUTSIDE_DISPATCH",
		"binding createdAt differs from original run":            "CI04_SIGNING_RUN_CREATED_AT_MISMATCH",
		"run starts entirely before creation":                    "CI04_SIGNING_RUN_START_PRECEDES_CREATION",
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			in, expected, archives, root := publicationTestNativeFixture(t)
			if control.change != nil {
				control.change(t, in, &expected, archives, root)
			}
			err := publicationValidateSigning(context.Background(), in, expected, archives)
			if control.positive {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil {
				t.Fatal("unrequested or unbound signing accepted")
			}
			if want := wantErrors[control.name]; want != "" && err.Error() != want {
				t.Fatalf("expected causal %s, got %v", want, err)
			}
			var pending finalProvenancePending
			if errors.As(err, &pending) != control.pending {
				t.Fatalf("wrong authority status: %v", err)
			}
		})
	}
}

func TestPublicationSecurityCompleteArtifactInventory(t *testing.T) {
	in, root := ciTestInputs(t)
	in.Expected.Runs["build"] = finalProvenanceRun{ID: 10, Status: "completed"}
	in.Expected.Artifacts["report"] = finalProvenanceArtifact{Availability: "github", Run: "build", Metadata: "list", ID: 20}
	ciTestObject(t, in, root, "list", map[string]any{"total_count": 1, "artifacts": []any{map[string]any{"id": 20}}})
	expected := publicationSecurityExpected{ArtifactLists: map[string]string{"build": "list"}, PublicArchives: map[string]map[string]publicationPublicMember{"report": {"report.json": {Class: "public-metadata"}}}}
	if err := publicationValidateArtifactInventory(in, expected); err != nil {
		t.Fatal(err)
	}
	t.Run("new published artifact cannot be omitted", func(t *testing.T) {
		ciTestObject(t, in, root, "list", map[string]any{"total_count": 2, "artifacts": []any{map[string]any{"id": 20}, map[string]any{"id": 21}}})
		if err := publicationValidateArtifactInventory(in, expected); err == nil {
			t.Fatal("omitted original artifact accepted")
		}
		ciTestObject(t, in, root, "list", map[string]any{"total_count": 1, "artifacts": []any{map[string]any{"id": 20}}})
	})
	t.Run("a current zero-artifact run still needs its original list", func(t *testing.T) {
		in.Expected.Runs["app"] = finalProvenanceRun{ID: 11, Status: "completed"}
		defer delete(in.Expected.Runs, "app")
		if err := publicationValidateArtifactInventory(in, expected); err == nil || err.Error() != "CI04_CURRENT_RUN_PUBLICATION_LIST_OMITTED" {
			t.Fatalf("missing run listing: %v", err)
		}
	})
}

func publicationTestSealedFixture(t *testing.T, protocol string) (*finalProvenanceInputs, publicationSecurityExpected, map[string]map[string][]byte, string) {
	t.Helper()
	in, expected, _, originals := publicationTestNativeFixture(t)
	source := in.Expected.Sources["implementation"]
	run := finalProvenanceRun{Original: "sealed-run", Repository: source.Repository, ID: 2002, Attempt: 1, WorkflowID: 337107435, WorkflowPath: ".github/workflows/ios-build.yml", Event: "workflow_dispatch", SourceSHA: source.SHA, Status: "completed", Conclusion: "failure"}
	in.Expected.Runs["sealed-run"] = run
	cipher := []byte("SYNTHETIC_OPAQUE_BYTES_NOT_ENCRYPTED_OR_NATIVE_EVIDENCE")
	archive, pins := ciTestZIP(t, map[string][]byte{"cipher.p7m": cipher, "receipt.json": []byte("{}\n")}, zip.Store)
	ciTestPut(t, in, originals, "sealed-archive", archive)
	in.Expected.Artifacts["sealed"] = finalProvenanceArtifact{ID: 3003, Run: "sealed-run", File: "sealed-archive", Role: "implementation", SourceSHA: source.SHA, Availability: "github"}
	inventory := publicationTestMembers(pins, "public-metadata")
	item := inventory["cipher.p7m"]
	item.Class = "sealed-private-ciphertext"
	inventory["cipher.p7m"] = item
	expected.PublicArchives = map[string]map[string]publicationPublicMember{"sealed": inventory}
	archives := map[string]map[string][]byte{"sealed": {"cipher.p7m": cipher, "receipt.json": []byte("{}\n")}}
	proof := publicationSealedProof{Protocol: protocol, Member: "cipher.p7m", PublicVerification: "public-verification"}
	if protocol == "cherri-project-prerequisite-observation/3" {
		proof.PublicVerificationSchema = "synthetic-public-semantic-intake/1"
		proof.EnvelopeBinding = "public-envelope-binding"
		helpers := map[string]string{}
		for _, name := range []string{"tools/shortcuts-simulator-qa/project_prerequisite.py", "tools/shortcuts-simulator-qa/project_prerequisite_observation.py", "tools/shortcuts-simulator-qa/qa_terminal_receipts.py", "tools/shortcuts-simulator-qa/simulator_session_recovery.py"} {
			body, err := os.ReadFile(filepath.Join(in.Manifest.SourceDirectories["implementation"], filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			helpers[name] = ciIdentityDigest(body)
		}
		ciTestObject(t, in, originals, proof.PublicVerification, map[string]any{
			"schema": proof.PublicVerificationSchema, "runId": run.ID, "attempt": run.Attempt, "sourceCommit": source.SHA, "allPublicationByteHashesMatched": true, "sourceBoundValidationHelpers": helpers,
			"prerequisitePublic": map[string]any{"schema": protocol, "executionScope": "CURRENT_PREREQUISITE_CALL_ONLY", "strictPublicEnvelope": "PASS", "strictPublicValidator": "PASS", "sourceAndEnvironmentMatched": true, "formalAcceptanceGranted": false, "originalMembers": 23, "expectedContextSha256": strings.Repeat("e", 64), "privateInPlaceVerification": "PENDING", "ciphertext": map[string]any{"bytes": len(cipher), "sha256": ciIdentityDigest(cipher)}, "plaintextArchive": map[string]any{"sha256": strings.Repeat("f", 64)}},
		})
		ciTestObject(t, in, originals, proof.EnvelopeBinding, map[string]any{
			"schema": "cherri-windows-private-prerequisite-intake/1", "runId": run.ID, "attempt": run.Attempt, "sourceCommit": source.SHA, "publicArtifactSha256": ciIdentityDigest(archive), "mandatoryPublicValidationSha256": in.Expected.Files[proof.PublicVerification].SHA256, "privatePlaintextOrKeyExported": false,
			"verification": map[string]any{"localVerificationPassed": true, "originalMembersVerified": 23, "plaintextOrKeyTransferred": false, "prerequisiteOnly": true, "nativeRuntimeAcceptanceClaimed": false, "formalAcceptanceGranted": false, "ciphertextSha256": ciIdentityDigest(cipher), "expectedContextSha256": strings.Repeat("e", 64), "archiveSha256": strings.Repeat("f", 64)},
		})
	} else {
		proof.PublicStdout = "public-stdout"
		proof.PublicStderr = "public-stderr"
		proof.ExpectedContext = "public-expected"
		paths := []string{".github/workflows/ios-build.yml", "ios/CherriTests/CherriEditorInputTests.swift", "ios/CherriUITests/CherriEditorUITests.swift", "ios/CherriUITests/EditorStartupDeadline.swift", "ios/project.yml", "ios/scripts/collect_ios_xcresult_summary.py", "ios/scripts/test_ios_xcresult_summary.py", "ios/scripts/verify_ios_xcresult_summary.py", "tools/shortcuts-simulator-qa/project_prerequisite.py", "tools/shortcuts-simulator-qa/project_prerequisite_observation.py", "tools/shortcuts-simulator-qa/qa_terminal_receipts.py", "tools/shortcuts-simulator-qa/simulator_session_recovery.py", "tools/shortcuts-simulator-qa/xcode_environment_inventory_recipient.pem"}
		files := map[string]any{}
		root := in.Manifest.SourceDirectories["implementation"]
		for _, name := range paths {
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
			if err != nil {
				t.Fatal(err)
			}
			files[name] = map[string]any{"sha256": ciIdentityDigest(body), "bytes": len(body), "mode": "100644", "blob": finalProvenanceTestGit(t, root, "rev-parse", source.SHA+":"+name)}
		}
		ciTestObject(t, in, originals, proof.ExpectedContext, map[string]any{
			"schema": "cherri-ios-xcresult-summary-expected/1", "workflowId": run.WorkflowID,
			"context":  map[string]any{"runId": run.ID, "attempt": run.Attempt, "sourceSha": source.SHA, "repository": run.Repository, "workflowPath": run.WorkflowPath, "event": run.Event},
			"artifact": map[string]any{"id": 3003, "bytes": len(archive), "sha256": ciIdentityDigest(archive)},
			"sources":  map[string]any{"sourceSha": source.SHA, "tree": source.Tree, "files": files, "absent": []string{"ios/CherriApp/Views/CodeEditorView.swift"}},
		})
		stdout := map[string]any{"schema": protocol, "status": "PUBLIC_CIPHERTEXT_VERIFIED", "artifactSha256": ciIdentityDigest(archive), "privateIntakePerformed": false, "formalAcceptanceGranted": false}
		ciTestObject(t, in, originals, proof.PublicStdout, stdout)
		ciTestPut(t, in, originals, proof.PublicStderr, nil)
		ciTestObject(t, in, originals, proof.PublicVerification, map[string]any{"exitCode": 0, "expectedSha256": in.Expected.Files[proof.ExpectedContext].SHA256, "stdoutSha256": in.Expected.Files[proof.PublicStdout].SHA256, "stderrSha256": in.Expected.Files[proof.PublicStderr].SHA256, "stdout": stdout})
	}
	expected.SealedProofs = map[string]publicationSealedProof{"sealed": proof}
	return in, expected, archives, originals
}

func TestPublicationSecurityOriginalPublicSealJoins(t *testing.T) {
	for _, protocol := range []string{"cherri-project-prerequisite-observation/3", "cherri-ios-xcresult-summary/1"} {
		t.Run(protocol, func(t *testing.T) {
			in, expected, archives, originals := publicationTestSealedFixture(t, protocol)
			if err := publicationValidateSealed(context.Background(), in, expected, archives); err != nil {
				t.Fatalf("exact original public joins: %v", err)
			}
			proof := expected.SealedProofs["sealed"]
			if protocol == "cherri-project-prerequisite-observation/3" {
				t.Run("missing outer envelope join remains not run", func(t *testing.T) {
					changed := proof
					changed.EnvelopeBinding = ""
					expected.SealedProofs["sealed"] = changed
					defer func() { expected.SealedProofs["sealed"] = proof }()
					err := publicationValidateSealed(context.Background(), in, expected, archives)
					var pending finalProvenancePending
					if !errors.As(err, &pending) {
						t.Fatalf("missing old23 authority: %v", err)
					}
				})
				t.Run("same ciphertext in a different outer artifact", func(t *testing.T) {
					original := append([]byte{}, in.Bytes["sealed-archive"]...)
					defer ciTestPut(t, in, originals, "sealed-archive", original)
					changed, _ := ciTestZIP(t, map[string][]byte{"cipher.p7m": archives["sealed"]["cipher.p7m"], "receipt.json": []byte("{\"changed\":true}\n")}, zip.Store)
					ciTestPut(t, in, originals, "sealed-archive", changed)
					if err := publicationValidateSealed(context.Background(), in, expected, archives); err == nil || err.Error() != "CI04_OLD23_PUBLIC_ENVELOPE_JOIN" {
						t.Fatalf("outer original binding not causal: %v", err)
					}
				})
				t.Run("public validator failure cannot be hidden by rehashed join", func(t *testing.T) {
					oldPublic, oldJoin := append([]byte{}, in.Bytes[proof.PublicVerification]...), append([]byte{}, in.Bytes[proof.EnvelopeBinding]...)
					defer ciTestPut(t, in, originals, proof.PublicVerification, oldPublic)
					defer ciTestPut(t, in, originals, proof.EnvelopeBinding, oldJoin)
					object, _ := in.object(proof.PublicVerification)
					object["prerequisitePublic"].(map[string]any)["strictPublicValidator"] = "FAIL"
					ciTestObject(t, in, originals, proof.PublicVerification, object)
					join, _ := in.object(proof.EnvelopeBinding)
					join["mandatoryPublicValidationSha256"] = in.Expected.Files[proof.PublicVerification].SHA256
					ciTestObject(t, in, originals, proof.EnvelopeBinding, join)
					if err := publicationValidateSealed(context.Background(), in, expected, archives); err == nil || err.Error() != "CI04_QA_PUBLIC_SEAL_BOUNDARY" {
						t.Fatalf("failed public validation accepted: %v", err)
					}
				})
				t.Run("independent context mismatch", func(t *testing.T) {
					old := append([]byte{}, in.Bytes[proof.EnvelopeBinding]...)
					defer ciTestPut(t, in, originals, proof.EnvelopeBinding, old)
					object, _ := in.object(proof.EnvelopeBinding)
					object["verification"].(map[string]any)["expectedContextSha256"] = strings.Repeat("a", 64)
					ciTestObject(t, in, originals, proof.EnvelopeBinding, object)
					if err := publicationValidateSealed(context.Background(), in, expected, archives); err == nil || err.Error() != "CI04_OLD23_PUBLIC_PRIVATE_CONTEXT_JOIN" {
						t.Fatalf("cross-context old23 receipt: %v", err)
					}
				})
			} else {
				for _, control := range []struct {
					name   string
					mutate func(map[string]any)
					code   string
				}{
					{"wrong source context", func(o map[string]any) { o["context"].(map[string]any)["sourceSha"] = strings.Repeat("f", 40) }, "CI04_XCRESULT_PUBLIC_CONTEXT"},
					{"wrong exact source helper", func(o map[string]any) {
						o["sources"].(map[string]any)["files"].(map[string]any)["ios/scripts/verify_ios_xcresult_summary.py"].(map[string]any)["sha256"] = strings.Repeat("f", 64)
					}, "CI04_PUBLIC_VALIDATOR_SOURCE_CHANGED"},
					{"wrong authenticated artifact", func(o map[string]any) { o["artifact"].(map[string]any)["sha256"] = strings.Repeat("f", 64) }, "CI04_XCRESULT_PUBLIC_ARTIFACT"},
				} {
					t.Run(control.name, func(t *testing.T) {
						oldContext, oldPublic := append([]byte{}, in.Bytes[proof.ExpectedContext]...), append([]byte{}, in.Bytes[proof.PublicVerification]...)
						defer ciTestPut(t, in, originals, proof.ExpectedContext, oldContext)
						defer ciTestPut(t, in, originals, proof.PublicVerification, oldPublic)
						object, _ := in.object(proof.ExpectedContext)
						control.mutate(object)
						ciTestObject(t, in, originals, proof.ExpectedContext, object)
						public, _ := in.object(proof.PublicVerification)
						public["expectedSha256"] = in.Expected.Files[proof.ExpectedContext].SHA256
						ciTestObject(t, in, originals, proof.PublicVerification, public)
						if err := publicationValidateSealed(context.Background(), in, expected, archives); err == nil || err.Error() != control.code {
							t.Fatalf("specific original join was not causal: %v", err)
						}
					})
				}
				t.Run("nonempty original verifier stderr", func(t *testing.T) {
					oldPublic := append([]byte{}, in.Bytes[proof.PublicVerification]...)
					defer ciTestPut(t, in, originals, proof.PublicStderr, nil)
					defer ciTestPut(t, in, originals, proof.PublicVerification, oldPublic)
					ciTestPut(t, in, originals, proof.PublicStderr, []byte("synthetic failure"))
					public, _ := in.object(proof.PublicVerification)
					public["stderrSha256"] = in.Expected.Files[proof.PublicStderr].SHA256
					ciTestObject(t, in, originals, proof.PublicVerification, public)
					if err := publicationValidateSealed(context.Background(), in, expected, archives); err == nil || err.Error() != "CI04_XCRESULT_PUBLIC_SEAL_PROCESS" {
						t.Fatalf("failed original process acquired seal credit: %v", err)
					}
				})
			}
		})
	}
}

func publicationTestSanitizerLog(t *testing.T) string {
	t.Helper()
	const target = "github.com/electrikmilk/cherri/tools/shortcut-corpus"
	packages := []string{"github.com/electrikmilk/cherri", target}
	plan, err := json.Marshal(map[string]any{"packages": packages, "argv": append([]string{"go", "test", "-count=1", "-timeout=15m", "-parallel=2", "-v"}, packages...), "timeout": "15m"})
	if err != nil {
		t.Fatal(err)
	}
	lines := []string{"GO_SUITE_EXECUTE " + string(plan), "=== RUN   TestOther", "--- PASS: TestOther (0.00s)", "PASS", "ok\tgithub.com/electrikmilk/cherri\t0.001s"}
	for _, name := range publicationSanitizerTests {
		lines = append(lines, "=== RUN   "+name, "--- PASS: "+name+" (0.00s)")
	}
	lines = append(lines, "PASS", "ok\t"+target+"\t0.002s")
	var output strings.Builder
	for _, line := range lines {
		output.WriteString("\ufeff2020-01-01T00:00:15.100Z " + line + "\n")
	}
	return output.String()
}

func TestPublicationSecurityActualSanitizerEvents(t *testing.T) {
	raw := publicationTestSanitizerLog(t)
	controls := []struct {
		name     string
		change   func(string) string
		positive bool
	}{
		{"original package controls with line BOM", func(s string) string { return s }, true},
		{"missing selected parent", func(s string) string { return strings.ReplaceAll(s, publicationSanitizerTests[0], "TestUnrelated") }, false},
		{"duplicate selected pass", func(s string) string {
			return strings.Replace(s, "--- PASS: "+publicationSanitizerTests[0]+" (0.00s)", "--- PASS: "+publicationSanitizerTests[0]+" (0.00s)\n2020-01-01T00:00:15.1Z --- PASS: "+publicationSanitizerTests[0]+" (0.00s)", 1)
		}, false},
		{"contradictory failure", func(s string) string {
			return strings.Replace(s, "--- PASS: "+publicationSanitizerTests[0]+" (0.00s)", "--- PASS: "+publicationSanitizerTests[0]+" (0.00s)\n2020-01-01T00:00:15.1Z --- FAIL: "+publicationSanitizerTests[0]+" (0.00s)", 1)
		}, false},
		{"selected skipped", func(s string) string {
			return strings.Replace(s, "--- PASS: "+publicationSanitizerTests[0], "--- SKIP: "+publicationSanitizerTests[0], 1)
		}, false},
		{"borrowed from another package", func(s string) string {
			return strings.Replace(s, "ok\tgithub.com/electrikmilk/cherri/tools/shortcut-corpus", "ok\tgithub.com/electrikmilk/cherri/tools/other", 1)
		}, false},
		{"cached result", func(s string) string { return strings.Replace(s, "\t0.002s", "\t(cached)", 1) }, false},
		{"changed original deadline", func(s string) string { return strings.ReplaceAll(s, "15m", "30m") }, false},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			err := publicationSanitizerEvents([]byte(control.change(raw)), "2020-01-01T00:00:10Z", "2020-01-01T00:00:20Z")
			if control.positive {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil {
				t.Fatal("unobserved sanitizer controls acquired credit")
			}
		})
	}
}

func TestPublicationSecurityReuseAndWholeObservation(t *testing.T) {
	source := strings.Repeat("a", 40)
	t.Run("missing RP43 remains not run", func(t *testing.T) {
		_, err := publicationRP43(nil, source)
		var pending finalProvenancePending
		if !errors.As(err, &pending) {
			t.Fatalf("missing component: %v", err)
		}
	})
	t.Run("historical RP43 is not reused", func(t *testing.T) {
		_, err := publicationRP43([]EvidenceRecord{{Requirements: []string{"RP43"}, Tier: "security", ImplementationSHA: strings.Repeat("b", 40)}}, source)
		if err == nil || err.Error() != "CI04_HISTORICAL_RP43_COMPONENT" {
			t.Fatalf("historical component: %v", err)
		}
	})
	t.Run("claim without original process is rejected", func(t *testing.T) {
		_, err := publicationRP43([]EvidenceRecord{{Requirements: []string{"RP43"}, Tier: "security", ImplementationSHA: source, Result: "passed"}}, source)
		if err == nil {
			t.Fatal("RP43 claim replaced actual six-parent originals")
		}
	})
	revalidated := publicationSecurityObservation{Schema: publicationSecuritySchema, ImplementationSHA: source, ExpectedSHA256: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), Status: "PASSED", Scope: publicationSecurityScope, InputsCompletedUTC: "2020-01-01T00:01:00Z", StartedUTC: "2020-01-01T00:03:00Z", FinishedUTC: "2020-01-01T00:03:01Z", ElapsedSeconds: 1, RP43: &EvidenceRecord{ImplementationSHA: source}}
	original := revalidated
	original.StartedUTC = "2020-01-01T00:02:00Z"
	original.FinishedUTC = "2020-01-01T00:02:01Z"
	if err := publicationCompareObservation(original, revalidated); err != nil {
		t.Fatal(err)
	}
	for _, control := range []struct {
		name   string
		change func(*publicationSecurityObservation)
	}{
		{"restamped manifest", func(o *publicationSecurityObservation) { o.ManifestSHA256 = strings.Repeat("d", 64) }},
		{"backdated before RP43", func(o *publicationSecurityObservation) { o.StartedUTC = "2020-01-01T00:00:59Z" }},
		{"changed counts", func(o *publicationSecurityObservation) { o.PublicMembers++ }},
		{"borrowed component", func(o *publicationSecurityObservation) {
			copy := *o.RP43
			copy.ImplementationSHA = strings.Repeat("f", 40)
			o.RP43 = &copy
		}},
	} {
		t.Run(control.name, func(t *testing.T) {
			changed := original
			control.change(&changed)
			if err := publicationCompareObservation(changed, revalidated); err == nil {
				t.Fatal("changed original accepted")
			}
		})
	}
	if reflect.DeepEqual(original, revalidated) {
		t.Fatal("timing-only positive did not vary execution times")
	}
}
