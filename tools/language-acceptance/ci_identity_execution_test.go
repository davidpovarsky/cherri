package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// All authority/API/publication rows in these controls are synthetic. Temporary
// Git repositories and the two tiny Go binaries are real and locally owned.
// No fixture is a real workflow, publication, signing or acceptance receipt.
func ciTestPut(t *testing.T, in *finalProvenanceInputs, root, id string, data []byte) {
	t.Helper()
	name := filepath.Join(root, id)
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	in.Expected.Files[id] = finalProvenanceFile{ciIdentityDigest(data), int64(len(data))}
	in.Manifest.Files[id] = name
	in.Bytes[id] = append([]byte{}, data...)
	delete(in.JSON, id)
}

func ciTestObject(t *testing.T, in *finalProvenanceInputs, root, id string, object any) {
	t.Helper()
	data, err := json.Marshal(object)
	if err != nil {
		t.Fatal(err)
	}
	ciTestPut(t, in, root, id, data)
}

func ciTestInputs(t *testing.T) (*finalProvenanceInputs, string) {
	t.Helper()
	return &finalProvenanceInputs{
		Expected: finalProvenanceExpected{Files: map[string]finalProvenanceFile{}, Sources: map[string]finalProvenanceSource{}, Runs: map[string]finalProvenanceRun{}, Artifacts: map[string]finalProvenanceArtifact{}},
		Manifest: finalProvenanceManifest{Files: map[string]string{}, SourceDirectories: map[string]string{}},
		JSON:     map[string]map[string]any{}, Bytes: map[string][]byte{},
	}, t.TempDir()
}

func ciTestZIP(t *testing.T, files map[string][]byte, method uint16) ([]byte, map[string]ciArchiveMember) {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	inventory := map[string]ciArchiveMember{}
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		header := &zip.FileHeader{Name: name, Method: method}
		header.SetMode(0644)
		stream, err := writer.CreateHeader(header)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = stream.Write(files[name]); err != nil {
			t.Fatal(err)
		}
		inventory[name] = ciArchiveMember{ciIdentityDigest(files[name]), int64(len(files[name])), 0644}
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes(), inventory
}

func TestCIIdentityIndependentContext(t *testing.T) {
	t.Run("absence remains not run", func(t *testing.T) {
		for _, key := range []string{"CHERRI_CI03_EXPECTED", "CHERRI_CI03_EXPECTED_SHA256", "CHERRI_CI03_MANIFEST"} {
			t.Setenv(key, "")
		}
		runner := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
		runner.runCIIdentityRequirement()
		if runner.results["CI03"].Status != "NOT_RUN" || len(runner.records) != 0 {
			t.Fatal("absent authority acquired credit")
		}
	})
	t.Run("bindings need no finalized ledger", func(t *testing.T) {
		fixture := finalProvenanceFixtureNew(t)
		fixture.expected.Ledger = nil
		fixture.manifest.Ledger, fixture.manifest.Limitations = nil, nil
		manifest := ciIdentityManifest{Schema: "cherri-ci03-locators/1", Bindings: fixture.manifest}
		mraw, _ := json.Marshal(manifest)
		if err := os.WriteFile(fixture.config.ManifestPath, mraw, 0600); err != nil {
			t.Fatal(err)
		}
		expected := ciIdentityExpected{Schema: "cherri-ci03-expected/1", ManifestSHA256: ciIdentityDigest(mraw), Bindings: fixture.expected}
		eraw, _ := json.Marshal(expected)
		if err := os.WriteFile(fixture.config.ExpectedPath, eraw, 0600); err != nil {
			t.Fatal(err)
		}
		config := ciIdentityConfig{fixture.config.ExpectedPath, ciIdentityDigest(eraw), fixture.config.ManifestPath}
		if _, _, err := ciIdentityLoad(config, fixture.output); err != nil {
			t.Fatalf("independent binding-only context rejected: %v", err)
		}
		changed := append([]byte{}, mraw...)
		changed = append(changed, ' ')
		if err := os.WriteFile(config.ManifestPath, changed, 0600); err != nil {
			t.Fatal(err)
		}
		if _, _, err := ciIdentityLoad(config, fixture.output); err == nil {
			t.Fatal("candidate locator bytes changed without independent authorization")
		}
		if err := os.WriteFile(config.ManifestPath, mraw, 0600); err != nil {
			t.Fatal(err)
		}
		config.ExpectedSHA256 = strings.Repeat("f", 64)
		if _, _, err := ciIdentityLoad(config, fixture.output); err == nil {
			t.Fatal("wrong independent expected digest accepted")
		}
	})
	t.Run("generic successful workflows cannot override dedicated absence or failure", func(t *testing.T) {
		original := ciRunDiscoveryFunc
		t.Cleanup(func() { ciRunDiscoveryFunc = original })
		ciRunDiscoveryFunc = func(source string) ([]CIRunRecord, error) {
			var runs []CIRunRecord
			for _, name := range []string{"Build & Test", "OpenMinis Skill", "iOS Build", "iOS 27 Shortcuts Runtime PoC"} {
				runs = append(runs, CIRunRecord{HeadSHA: source, WorkflowName: name, RunID: "synthetic", Conclusion: "success"})
			}
			return runs, nil
		}
		for _, status := range []string{"NOT_RUN", "FAILED"} {
			runner := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{"CI03": {ID: "CI03", Status: status, Detail: "dedicated original result"}}}
			runner.runCIVerification()
			if result := runner.results["CI03"]; result.Status != status || result.Passed || result.Detail != "dedicated original result" {
				t.Fatal("generic CI upgraded dedicated identity result")
			}
			for _, record := range runner.records {
				for _, id := range record.Requirements {
					if id == "CI03" {
						t.Fatal("generic workflow record grants CI03")
					}
				}
			}
		}
	})
	t.Run("historical smoke guard and cross requirement rejection remain", func(t *testing.T) {
		runner := &Runner{results: map[string]CaseResult{}}
		runner.Record("CI03", "verification", "identity", "repository+CI", []string{"repository"}, true, "PASSED", "forty-character SHA")
		if runner.results["CI03"].Status != "NOT_RUN" || len(runner.records) != 0 {
			t.Fatal("weak historical assertion acquired credit")
		}
		if err := validateCIIdentityRecord("RP49", EvidenceRecord{TestID: ciIdentityTestID}); err == nil {
			t.Fatal("identity evidence conferred closure")
		}
	})
	t.Run("evidence map permits only the dedicated intake", func(t *testing.T) {
		contracts, err := LoadAllContracts("", "", "")
		if err != nil {
			t.Fatal(err)
		}
		mapping, err := BuildRequirementsEvidenceMap(contracts)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(mapping.Mappings["CI03"].TestIDs, []string{ciIdentityTestID}) || !reflect.DeepEqual(mapping.Mappings["CI04"].TestIDs, []string{publicationSecurityTestID}) {
			t.Fatal("generic test authorization leaked into dedicated map")
		}
	})
}

func TestCIIdentityOriginalCheckoutCommands(t *testing.T) {
	source := strings.Repeat("a", 40)
	const start, finish = "2020-01-01T00:00:10Z", "2020-01-01T00:00:15Z"
	good := "2020-01-01T00:00:15.1202000Z [command]/opt/homebrew/bin/git log -1 --format=%H\n2020-01-01T00:00:15.1346280Z " + source + "\n"
	controls := []struct {
		name, raw, expected string
		accepted            bool
		pending             bool
	}{
		{"actual command at API second precision", good, source, true, false},
		{"BOM before every timestamp", "\ufeff" + strings.ReplaceAll(good, "\n2020", "\n\ufeff2020"), source, true, false},
		{"head-only metadata", "2020-01-01T00:00:14Z head_sha=" + source + "\n", source, false, true},
		{"echoed source", "2020-01-01T00:00:14Z [command]/bin/echo " + source + "\n", source, false, true},
		{"wrong actual source", good, strings.Repeat("b", 40), false, false},
		{"duplicate command", good + good, source, false, false},
		{"missing command output", strings.Split(good, "\n")[0] + "\n", source, false, false},
		{"foreign next output", strings.Replace(good, source, "not a commit", 1), source, false, false},
		{"outside checkout step", strings.ReplaceAll(good, "00:00:15", "00:00:16"), source, false, true},
		{"relative git text", strings.Replace(good, "/opt/homebrew/bin/git", "git", 1), source, false, true},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			got, err := ciIdentityCheckoutSHA([]byte(control.raw), start, finish, control.expected)
			if control.accepted {
				if err != nil || got != source {
					t.Fatalf("original observation rejected: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("unobserved checkout accepted")
			}
			var pending finalProvenancePending
			if errors.As(err, &pending) != control.pending {
				t.Fatalf("missing/contradictory classification: %v", err)
			}
		})
	}
}

func ciTestCheckoutFixture(t *testing.T) (*finalProvenanceInputs, ciIdentityExpected, string) {
	t.Helper()
	in, originals := ciTestInputs(t)
	expected := ciIdentityExpected{Checkouts: map[string][]ciCheckoutObservation{}, WorkflowSHA256: map[string]string{}}
	for _, role := range []string{"implementation", "docs"} {
		root := t.TempDir()
		finalProvenanceTestGit(t, root, "init", "--template=", "-b", "fixture")
		for surface, file := range ciIdentityRequiredWorkflows {
			if (role == "docs") != (surface == "docs") {
				continue
			}
			body := []byte("name: Synthetic " + surface + "\njobs:\n")
			for _, name := range ciIdentityRequiredJobs[surface] {
				body = append(body, []byte("  "+name+":\n    steps:\n      - uses: actions/checkout@v6\n")...)
			}
			full := filepath.Join(root, filepath.FromSlash(file))
			if err := os.MkdirAll(filepath.Dir(full), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(full, body, 0600); err != nil {
				t.Fatal(err)
			}
			expected.WorkflowSHA256[surface] = ciIdentityDigest(body)
		}
		finalProvenanceTestGit(t, root, "add", ".")
		finalProvenanceTestGit(t, root, "commit", "-m", "Synthetic observed workflow sources")
		repository := "davidpovarsky/cherri"
		if role == "docs" {
			repository = "davidpovarsky/cherrilang.org"
		}
		in.Expected.Sources[role] = finalProvenanceSource{Repository: repository, SHA: finalProvenanceTestGit(t, root, "rev-parse", "HEAD"), Tree: finalProvenanceTestGit(t, root, "rev-parse", "HEAD^{tree}"), LocalRef: "refs/heads/fixture", PublishedRef: "refs/heads/fixture"}
		in.Manifest.SourceDirectories[role] = root
	}
	runID := int64(1000)
	for _, role := range []string{"build", "skill", "app", "runtime", "docs"} {
		sourceRole := "implementation"
		if role == "docs" {
			sourceRole = "docs"
		}
		source := in.Expected.Sources[sourceRole]
		runID++
		run := finalProvenanceRun{Original: role + "-run", Repository: source.Repository, ID: runID, Attempt: 1, WorkflowID: runID + 100, WorkflowPath: ciIdentityRequiredWorkflows[role], Event: "workflow_dispatch", SourceSHA: source.SHA, Status: "completed", Conclusion: "failure"}
		in.Expected.Runs[role] = run
		ciTestObject(t, in, originals, run.Original, map[string]any{"head_branch": "fixture", "updated_at": "2020-01-01T00:00:30Z"})
		var jobs []any
		for i, name := range ciIdentityRequiredJobs[role] {
			id := runID*10 + int64(i)
			logID := role + "-" + name + ".log"
			w := ciCheckoutObservation{Run: role, JobsFile: role + "-jobs", JobID: id, JobName: name, LogFile: logID, CheckoutStep: 2, CheckoutName: "Checkout"}
			expected.Checkouts[role] = append(expected.Checkouts[role], w)
			jobs = append(jobs, map[string]any{"id": id, "name": name, "run_id": run.ID, "run_attempt": 1, "head_sha": source.SHA, "status": "completed", "conclusion": "failure", "completed_at": "2020-01-01T00:00:25Z", "steps": []any{map[string]any{"number": 2, "name": "Checkout", "status": "completed", "conclusion": "success", "started_at": "2020-01-01T00:00:10Z", "completed_at": "2020-01-01T00:00:15Z"}}})
			log := "2020-01-01T00:00:14Z [command]/usr/bin/git log -1 --format=%H\n2020-01-01T00:00:14.1Z " + source.SHA + "\n"
			ciTestPut(t, in, originals, logID, []byte(log))
		}
		ciTestObject(t, in, originals, role+"-jobs", map[string]any{"total_count": len(jobs), "jobs": jobs})
	}
	return in, expected, originals
}

func TestCIIdentityAllRequiredCheckoutJobs(t *testing.T) {
	controls := []struct {
		name              string
		mutate            func(*testing.T, *finalProvenanceInputs, *ciIdentityExpected, string)
		accepted, pending bool
	}{
		{"all matrix jobs observed despite failed conclusions", nil, true, false},
		{"omit Nix witness", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			e.Checkouts["build"] = e.Checkouts["build"][:2]
		}, false, true},
		{"Nix checked out another source", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			w := e.Checkouts["build"][2]
			b := in.Bytes[w.LogFile]
			ciTestPut(t, in, root, w.LogFile, bytes.ReplaceAll(b, []byte(in.Expected.Sources["implementation"].SHA), []byte(strings.Repeat("f", 40))))
		}, false, false},
		{"configured wrong job name", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			o, _ := in.object("build-jobs")
			o["jobs"].([]any)[0].(map[string]any)["name"] = "wrong"
			ciTestObject(t, in, root, "build-jobs", o)
		}, false, false},
		{"configured wrong checkout step", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			o, _ := in.object("build-jobs")
			o["jobs"].([]any)[0].(map[string]any)["steps"].([]any)[0].(map[string]any)["name"] = "wrong"
			ciTestObject(t, in, root, "build-jobs", o)
		}, false, false},
		{"missing step remains unobserved", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			o, _ := in.object("build-jobs")
			o["jobs"].([]any)[0].(map[string]any)["steps"] = []any{}
			ciTestObject(t, in, root, "build-jobs", o)
		}, false, true},
		{"duplicate hidden job name", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			o, _ := in.object("build-jobs")
			j := o["jobs"].([]any)
			copy := map[string]any{}
			for k, v := range j[0].(map[string]any) {
				copy[k] = v
			}
			copy["id"] = int64(999999)
			o["jobs"] = append(j, copy)
			o["total_count"] = len(j) + 1
			ciTestObject(t, in, root, "build-jobs", o)
		}, false, false},
		{"wrong attempt", func(t *testing.T, in *finalProvenanceInputs, e *ciIdentityExpected, root string) {
			o, _ := in.object("build-jobs")
			o["jobs"].([]any)[2].(map[string]any)["run_attempt"] = 2
			ciTestObject(t, in, root, "build-jobs", o)
		}, false, false},
	}
	for _, control := range controls {
		t.Run(control.name, func(t *testing.T) {
			in, expected, root := ciTestCheckoutFixture(t)
			if control.mutate != nil {
				control.mutate(t, in, &expected, root)
			}
			observed, runs, jobs, err := ciIdentityValidateCheckouts(context.Background(), in, expected)
			if control.accepted {
				if err != nil || len(observed) != 7 || len(jobs) != 7 || runs["build"] != "failure" || jobs["build/build-test-nix"] != "failure" {
					t.Fatalf("complete original identity join: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("incomplete/contradictory checkout identity accepted")
			}
			var pending finalProvenancePending
			if errors.As(err, &pending) != control.pending {
				t.Fatalf("wrong failure class: %v", err)
			}
		})
	}
}

func TestCIIdentityArchiveAndOriginalBudgets(t *testing.T) {
	raw, pins := ciTestZIP(t, map[string][]byte{"evidence.json": []byte("{}\n")}, zip.Deflate)
	t.Run("missing macro proof differs from configured wrong workflow", func(t *testing.T) {
		in, _ := ciTestInputs(t)
		in.Expected.Artifacts["macro"] = finalProvenanceArtifact{Run: "skill"}
		in.Expected.Artifacts["other"] = finalProvenanceArtifact{Run: "build"}
		for _, control := range []struct {
			ref     ciMemberReference
			pending bool
		}{{ciMemberReference{}, true}, {ciMemberReference{Artifact: "unknown", Member: "compiler"}, false}, {ciMemberReference{Artifact: "other", Member: "compiler"}, false}} {
			err := ciIdentitySurfaceArtifact(in, control.ref, "skill")
			var pending finalProvenancePending
			if err == nil || errors.As(err, &pending) != control.pending {
				t.Fatalf("missing versus contradictory proof: %v", err)
			}
		}
		if err := ciIdentitySurfaceArtifact(in, ciMemberReference{Artifact: "macro", Member: "compiler"}, "skill"); err != nil {
			t.Fatal(err)
		}
		_, err := ciIdentityMember(map[string]map[string][]byte{"macro": {}}, ciMemberReference{Artifact: "macro", Member: "compiler"})
		var pending finalProvenancePending
		if !errors.As(err, &pending) {
			t.Fatalf("absent original compiler did not remain NOT_RUN: %v", err)
		}
	})
	t.Run("exact archive and cumulative second-archive boundary", func(t *testing.T) {
		budget := &ciExpandedBudget{Limit: 3}
		if _, err := ciIdentityArchive(raw, pins, budget); err != nil {
			t.Fatal(err)
		}
		if _, err := ciIdentityArchive(raw, pins, budget); err == nil || err.Error() != "CI_AGGREGATE_EXPANDED_LIMIT" {
			t.Fatalf("aggregate cap not causal: %v", err)
		}
	})
	t.Run("altered member digest", func(t *testing.T) {
		changed := map[string]ciArchiveMember{"evidence.json": {strings.Repeat("f", 64), 3, 0644}}
		if _, err := ciIdentityArchive(raw, changed, &ciExpandedBudget{Limit: 10}); err == nil {
			t.Fatal("digest drift accepted")
		}
	})
	t.Run("unlisted member", func(t *testing.T) {
		changed, _ := ciTestZIP(t, map[string][]byte{"evidence.json": []byte("{}\n"), "unexpected": []byte("x")}, zip.Store)
		if _, err := ciIdentityArchive(changed, pins, &ciExpandedBudget{Limit: 10}); err == nil {
			t.Fatal("extra member accepted")
		}
	})
	for _, name := range []string{"../private", "/absolute", "a/../b", "a\\b"} {
		t.Run("reject path "+name, func(t *testing.T) {
			changed, pins := ciTestZIP(t, map[string][]byte{name: []byte("x")}, zip.Store)
			if _, err := ciIdentityArchive(changed, pins, &ciExpandedBudget{Limit: 10}); err == nil {
				t.Fatal("unsafe archive path accepted")
			}
		})
	}
	t.Run("symlink original", func(t *testing.T) {
		in, root := ciTestInputs(t)
		ciTestPut(t, in, root, "original", []byte("public fixture"))
		alias := filepath.Join(root, "alias")
		if err := os.Symlink(in.Manifest.Files["original"], alias); err != nil {
			t.Fatal(err)
		}
		in.Manifest.Files["original"] = alias
		if _, err := ciIdentityReadOriginal(in, "original", 100); err == nil {
			t.Fatal("symlink original accepted")
		}
	})
}

func ciTestBuildBinary(t *testing.T, root, output, packagePath string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-buildvcs=true", "-trimpath", "-o", output, packagePath)
	command.Dir = root
	command.WaitDelay = 2 * time.Second
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if strings.HasPrefix(key, "GIT_") || key == "GOFLAGS" || key == "GOWORK" || key == "GOOS" || key == "GOARCH" || key == "CGO_ENABLED" || key == "GOPROXY" || key == "GOSUMDB" {
			continue
		}
		command.Env = append(command.Env, entry)
	}
	command.Env = append(command.Env, "GOWORK=off", "GOFLAGS=", "GOOS=linux", "GOARCH=arm64", "CGO_ENABLED=0", "GOPROXY=off", "GOSUMDB=off", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	outputBytes, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("owned no-network binary fixture failed: %v: %s", err, outputBytes)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestCIIdentityActualBinaryRolesAndMacroBytes(t *testing.T) {
	root, outputs := t.TempDir(), t.TempDir()
	finalProvenanceTestGit(t, root, "init", "--template=", "-b", "fixture")
	for name, body := range map[string]string{"go.mod": "module github.com/electrikmilk/cherri\n\ngo 1.24.0\n", "main.go": "package main\nfunc main() {}\n", "cmd/cherri-migrate/main.go": "package main\nfunc main() {}\n"} {
		name = filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(name), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	finalProvenanceTestGit(t, root, "add", ".")
	finalProvenanceTestGit(t, root, "commit", "-m", "Synthetic compiler role control")
	source := finalProvenanceTestGit(t, root, "rev-parse", "HEAD")
	compiler := ciTestBuildBinary(t, root, filepath.Join(outputs, "compiler"), ".")
	migrator := ciTestBuildBinary(t, root, filepath.Join(outputs, "migrator"), "./cmd/cherri-migrate")
	t.Run("actual compiler and migrator roles", func(t *testing.T) {
		for _, item := range []struct {
			body []byte
			role string
		}{{compiler, "github.com/electrikmilk/cherri"}, {migrator, "github.com/electrikmilk/cherri/cmd/cherri-migrate"}} {
			meta, err := ciIdentityBinary(item.body, source, item.role)
			if err != nil || meta["GOOS"] != "linux" || meta["GOARCH"] != "arm64" {
				t.Fatalf("real fixture rejected: %v", err)
			}
		}
	})
	t.Run("exact macro compiler", func(t *testing.T) {
		if err := ciIdentityMacroCompiler(map[string]any{"compilerSHA256": ciIdentityDigest(compiler)}, compiler, source); err != nil {
			t.Fatal(err)
		}
		if err := ciIdentityMacroCompiler(map[string]any{"compilerSHA256": strings.Repeat("e", 64)}, compiler, source); err == nil {
			t.Fatal("hex-only macro digest accepted")
		}
	})
	t.Run("wrong role and source", func(t *testing.T) {
		if _, err := ciIdentityBinary(migrator, source, "github.com/electrikmilk/cherri"); err == nil {
			t.Fatal("migrator substituted for compiler")
		}
		if _, err := ciIdentityBinary(compiler, strings.Repeat("f", 40), "github.com/electrikmilk/cherri"); err == nil {
			t.Fatal("wrong compiler source accepted")
		}
	})
	if status := finalProvenanceTestGit(t, root, "status", "--porcelain"); status != "" {
		t.Fatal("binary controls changed their source")
	}
}

func TestCIIdentityWholeObservationAndChronology(t *testing.T) {
	revalidated := ciIdentityObservation{Schema: ciIdentitySchema, Status: "PASSED", ImplementationSHA: strings.Repeat("a", 40), ExpectedSHA256: strings.Repeat("b", 64), ManifestSHA256: strings.Repeat("c", 64), InputsCompletedUTC: "2020-01-01T00:01:00Z", StartedUTC: "2020-01-01T00:03:00Z", FinishedUTC: "2020-01-01T00:03:01Z", ElapsedSeconds: 1, Scope: ciIdentityScope, WorkflowConclusions: map[string]string{"build": "failure"}}
	original := revalidated
	original.StartedUTC = "2020-01-01T00:02:00Z"
	original.FinishedUTC = "2020-01-01T00:02:01Z"
	if err := ciIdentityCompareObservation(original, revalidated); err != nil {
		t.Fatal(err)
	}
	for _, control := range []struct {
		name   string
		change func(*ciIdentityObservation)
	}{
		{"coherently restamped manifest", func(o *ciIdentityObservation) { o.ManifestSHA256 = strings.Repeat("d", 64) }},
		{"hidden original failure", func(o *ciIdentityObservation) { o.WorkflowConclusions = map[string]string{"build": "success"} }},
		{"backdated receipt", func(o *ciIdentityObservation) { o.StartedUTC = "2020-01-01T00:00:59Z" }},
		{"false closure", func(o *ciIdentityObservation) { o.ClosureComplete = true }},
	} {
		t.Run(control.name, func(t *testing.T) {
			changed := original
			control.change(&changed)
			if err := ciIdentityCompareObservation(changed, revalidated); err == nil {
				t.Fatal("changed original observation accepted")
			}
		})
	}
	if reflect.DeepEqual(original, revalidated) {
		t.Fatal("timing control did not change execution timestamps")
	}
}
