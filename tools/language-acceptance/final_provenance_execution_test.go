package main

import (
	"archive/zip"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"
)

// These are explicitly synthetic ORIGINAL-INTAKE controls. The temporary Git
// repositories are real; publication/API/process fixtures are models, never
// actual network publication, workflow, signing, or acceptance evidence.
type finalProvenanceFixture struct {
	root, originals, output string
	config                  finalProvenanceConfig
	expected                finalProvenanceExpected
	manifest                finalProvenanceManifest
	contracts               *ContractsSet
}

func finalProvenanceTestGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	a := append([]string{"-C", root, "-c", "commit.gpgsign=false", "-c", "user.name=Delivery fixture", "-c", "user.email=delivery-fixture@example.invalid"}, args...)
	cmd := exec.Command("git", a...)
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			cmd.Env = append(cmd.Env, e)
		}
	}
	cmd.Env = append(cmd.Env, "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL="+os.DevNull)
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("owned Git fixture failed: %v: %s", err, b)
	}
	return strings.TrimSpace(string(b))
}
func (f *finalProvenanceFixture) putBytes(t *testing.T, id string, b []byte) {
	t.Helper()
	p := filepath.Join(f.originals, id)
	if err := os.WriteFile(p, b, 0600); err != nil {
		t.Fatal(err)
	}
	f.expected.Files[id] = finalProvenanceFile{finalProvenanceDigest(b), int64(len(b))}
	f.manifest.Files[id] = p
}
func (f *finalProvenanceFixture) put(t *testing.T, id string, o any) {
	t.Helper()
	b, e := json.MarshalIndent(o, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	f.putBytes(t, id, append(b, '\n'))
}
func (f *finalProvenanceFixture) object(t *testing.T, id string) map[string]any {
	t.Helper()
	b, e := os.ReadFile(f.manifest.Files[id])
	if e != nil {
		t.Fatal(e)
	}
	o, e := decodeNativeEncodingObject(b)
	if e != nil {
		t.Fatal(e)
	}
	return o
}
func (f *finalProvenanceFixture) pin(t *testing.T) {
	t.Helper()
	b, e := json.MarshalIndent(f.manifest, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	b = append(b, '\n')
	if e = os.WriteFile(f.config.ManifestPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	f.expected.ManifestSHA256 = finalProvenanceDigest(b)
	b, e = json.MarshalIndent(f.expected, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	b = append(b, '\n')
	if e = os.WriteFile(f.config.ExpectedPath, b, 0600); e != nil {
		t.Fatal(e)
	}
	f.config.ExpectedSHA256 = finalProvenanceDigest(b)
}
func (f *finalProvenanceFixture) evaluate() finalProvenanceResult {
	return finalProvenanceEvaluate(f.config, f.manifest.SourceDirectories["implementation"], f.output, f.contracts)
}
func finalProvenanceFixtureNew(t *testing.T) *finalProvenanceFixture {
	t.Helper()
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	contractBytes := map[string][]byte{}
	for _, p := range []string{"tests/language-v2/acceptance-contract.json", "tests/language-v2/repair-regression-plan.json", "tests/language-v2/backend-recovery-gates.json"} {
		b, e := os.ReadFile(resolveRepoPath(p))
		if e != nil {
			t.Fatal(e)
		}
		contractBytes[p] = b
	}
	f := &finalProvenanceFixture{root: t.TempDir(), contracts: contracts}
	f.originals = filepath.Join(f.root, "originals")
	f.output = filepath.Join(f.root, "output")
	for _, p := range []string{f.originals, f.output} {
		if err = os.Mkdir(p, 0700); err != nil {
			t.Fatal(err)
		}
	}
	f.config = finalProvenanceConfig{ExpectedPath: filepath.Join(f.root, "operator-expected.json"), ManifestPath: filepath.Join(f.root, "delivery-manifest.json")}
	f.expected = finalProvenanceExpected{Schema: "cherri-final-provenance-expected/1", Sources: map[string]finalProvenanceSource{}, Files: map[string]finalProvenanceFile{}, Publications: map[string]finalProvenancePublication{}, Runs: map[string]finalProvenanceRun{}, Artifacts: map[string]finalProvenanceArtifact{}, AdditionalLimitations: []string{"Synthetic control; no real publication, Apple runtime, provider call, or global acceptance claim"}}
	f.manifest = finalProvenanceManifest{Schema: "cherri-final-provenance-delivery/1", SourceDirectories: map[string]string{}, Files: map[string]string{}, AdditionalLimitations: append([]string{}, f.expected.AdditionalLimitations...)}
	for _, role := range []string{"implementation", "compiler", "docs"} {
		dir := filepath.Join(f.root, role)
		if err = os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		finalProvenanceTestGit(t, dir, "init", "--template=", "-b", "fixture-"+role)
		if err = os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("owned "+role+" fixture\n"), 0600); err != nil {
			t.Fatal(err)
		}
		if role == "implementation" {
			if err = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture.invalid/delivery\n\ngo 1.24\n"), 0600); err != nil {
				t.Fatal(err)
			}
			for p, b := range contractBytes {
				full := filepath.Join(dir, p)
				if err = os.MkdirAll(filepath.Dir(full), 0700); err != nil {
					t.Fatal(err)
				}
				if err = os.WriteFile(full, b, 0600); err != nil {
					t.Fatal(err)
				}
			}
		}
		finalProvenanceTestGit(t, dir, "add", ".")
		finalProvenanceTestGit(t, dir, "commit", "-m", "Synthetic final delivery fixture")
		repository := "davidpovarsky/cherri"
		published := "refs/heads/fixture-" + role
		if role == "docs" {
			repository = "davidpovarsky/cherrilang.org"
			published = "refs/heads/main"
		}
		finalProvenanceTestGit(t, dir, "remote", "add", "origin", "https://github.com/"+repository+".git")
		f.expected.Sources[role] = finalProvenanceSource{repository, finalProvenanceTestGit(t, dir, "rev-parse", "HEAD"), finalProvenanceTestGit(t, dir, "rev-parse", "HEAD^{tree}"), "refs/heads/fixture-" + role, published}
		f.manifest.SourceDirectories[role] = dir
	}
	f.addWindowsPublication(t)
	f.addSourcePublication(t)
	f.addDocsPublication(t)
	f.addWorkflowAndArtifacts(t)
	f.addLedger(t)
	f.pin(t)
	return f
}
func (f *finalProvenanceFixture) addSourcePublication(t *testing.T) {
	s := f.expected.Sources["compiler"]
	ls := []string{"git", "ls-remote", "--heads", "origin", s.PublishedRef}
	push := []string{"git", "push", "--porcelain", "origin", s.SHA + ":" + s.PublishedRef}
	steps := []any{map[string]any{"command": ls, "exitCode": 0, "stdout": "", "stderr": ""}, map[string]any{"command": push, "exitCode": 0, "stdout": "To https://github.com/" + s.Repository + ".git\n*\t" + s.SHA + ":" + s.PublishedRef + "\t[new branch]\nDone\n", "stderr": ""}, map[string]any{"command": ls, "exitCode": 0, "stdout": s.SHA + "\t" + s.PublishedRef + "\n", "stderr": ""}}
	f.put(t, "compiler-publication", map[string]any{"sourceCommit": s.SHA, "ref": s.PublishedRef, "status": "PASS", "sourceCleanAfter": true, "startedAt": "2020-01-01T00:00:00Z", "finishedAt": "2020-01-01T00:01:00Z", "steps": steps})
	f.expected.Publications["compiler"] = finalProvenancePublication{Format: "source-create-only/1", Result: "compiler-publication"}
}
func (f *finalProvenanceFixture) addWindowsPublication(t *testing.T) {
	f.addWindowsPublicationForRole(t, "implementation")
}
func (f *finalProvenanceFixture) addWindowsPublicationForRole(t *testing.T, role string) {
	s := f.expected.Sources[role]
	p := finalProvenancePublication{Format: "windows-create-only/1", Result: role + "-publication", Streams: map[string]finalProvenanceStreams{}}
	streamPrefix := "publication-"
	if role != "implementation" {
		streamPrefix = role + "-publication-"
	}
	primary := map[string]any{"head": strings.Repeat("3", 40), "status": "?? synthetic-preserved-fixture.cherri", "preservedFixtureSHA256": strings.Repeat("8", 64)}
	names := []string{"01-origin", "02-base-refs", "03-primary-head-before", "04-primary-status-before", "05-transfer", "06-bundle-heads", "07-bundle-verify", "08-import", "09-commit", "10-tree", "11-ancestry", "12-connectivity", "13-imported-refs", "14-remote-before", "15-push", "16-remote-after", "17-api-ref", "18-api-commit", "21-refs-after-publication", "19-primary-head-after", "20-primary-status-after"}
	ref := map[string]any{"ref": s.PublishedRef, "sha": s.SHA, "type": "commit"}
	commit := map[string]any{"sha": s.SHA, "tree": s.Tree, "parents": []string{}}
	var steps []any
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range names {
		stdout := []byte{}
		argv := []string{"git", "observed-fixture-step", name}
		code := 0
		switch name {
		case "01-origin":
			stdout = []byte("https://github.com/" + s.Repository + ".git\n")
		case "03-primary-head-before", "19-primary-head-after":
			argv = []string{"git", "-C", "C:/synthetic-primary-checkout", "rev-parse", "HEAD"}
			stdout = []byte(primary["head"].(string) + "\n")
		case "04-primary-status-before", "20-primary-status-after":
			argv = []string{"git", "-C", "C:/synthetic-primary-checkout", "status", "--porcelain=v1"}
			stdout = []byte(primary["status"].(string) + "\n")
		case "14-remote-before":
			code = 2
			argv = []string{"git", "ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef}
		case "15-push":
			argv = []string{"git", "push", "--porcelain", "--force-with-lease=" + s.PublishedRef + ":", "origin", s.SHA + ":" + s.PublishedRef}
			stdout = []byte("synthetic successful push stream\n")
		case "16-remote-after":
			argv = []string{"git", "ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef}
			stdout = []byte(s.SHA + "\t" + s.PublishedRef + "\n")
		case "17-api-ref":
			stdout, _ = json.Marshal(ref)
		case "18-api-commit":
			stdout, _ = json.Marshal(commit)
		}
		pair := finalProvenanceStreams{streamPrefix + name + ".stdout", streamPrefix + name + ".stderr"}
		f.putBytes(t, pair.Stdout, stdout)
		f.putBytes(t, pair.Stderr, []byte{})
		p.Streams[name] = pair
		steps = append(steps, map[string]any{"name": name, "argv": argv, "exitCode": code, "timedOut": false, "stdoutSHA256": finalProvenanceDigest(stdout), "stderrSHA256": finalProvenanceDigest(nil), "stdoutBytes": len(stdout), "stderrBytes": 0, "startedAtUtc": start.Add(time.Duration(2*i) * time.Second).Format(time.RFC3339), "finishedAtUtc": start.Add(time.Duration(2*i+1) * time.Second).Format(time.RFC3339)})
	}
	f.put(t, p.Result, map[string]any{"sourceCommit": s.SHA, "sourceTree": s.Tree, "ref": s.PublishedRef, "status": "PUBLISHED_REMOTE_VERIFIED", "pushAttempted": true, "pushTimedOut": false, "pushAttemptLimit": 1, "pushExitCode": 0, "destinationMustBeAbsent": true, "primaryMutationAuthorized": false, "primaryUnchanged": true, "validation": map[string]any{"primaryBefore": primary}, "primaryAfter": primary, "steps": steps, "remoteBinding": map[string]any{"ref": ref, "commit": commit}})
	f.expected.Publications[role] = p
}
func (f *finalProvenanceFixture) addDocsPublication(t *testing.T) {
	s := f.expected.Sources["docs"]
	streams := finalProvenanceStreams{"docs-push.stdout", "docs-push.stderr"}
	f.putBytes(t, streams.Stdout, []byte{})
	f.putBytes(t, streams.Stderr, []byte("synthetic fast-forward output\n"))
	f.put(t, "docs-push", map[string]any{"schema": "cherri-docs-d12-main-push/1", "source": s.SHA, "argv": []string{"git", "push", "origin", s.SHA + ":" + s.PublishedRef}, "exitCode": 0, "attempt": 1, "force": false, "stdoutSHA256": f.expected.Files[streams.Stdout].SHA256, "stderrSHA256": f.expected.Files[streams.Stderr].SHA256})
	f.put(t, "docs-ref", map[string]any{"ref": s.PublishedRef, "url": "https://api.github.com/repos/" + s.Repository + "/git/refs/heads/main", "object": map[string]any{"sha": s.SHA, "type": "commit"}})
	f.put(t, "docs-commit", map[string]any{"sha": s.SHA, "tree": map[string]any{"sha": s.Tree}})
	f.put(t, "docs-publication", map[string]any{"status": "MAIN_FAST_FORWARD_CONFIRMED", "source": s.SHA, "tree": s.Tree, "sourceCleanAfter": true, "postRefSHA256": f.expected.Files["docs-ref"].SHA256, "pushResultSHA256": f.expected.Files["docs-push"].SHA256})
	f.expected.Publications["docs"] = finalProvenancePublication{Format: "docs-main-push/1", Result: "docs-publication", Push: "docs-push", Ref: "docs-ref", Commit: "docs-commit", PushStreams: &streams}
}
func (f *finalProvenanceFixture) addWorkflowAndArtifacts(t *testing.T) {
	s := f.expected.Sources["implementation"]
	run := finalProvenanceRun{Original: "run", Repository: s.Repository, ID: 9001, Attempt: 1, WorkflowID: 7001, WorkflowPath: ".github/workflows/build-test.yml", Event: "push", SourceSHA: s.SHA, Status: "completed", Conclusion: "failure"}
	f.expected.Runs["build"] = run
	f.put(t, "run", map[string]any{"id": run.ID, "run_attempt": run.Attempt, "workflow_id": run.WorkflowID, "head_sha": run.SourceSHA, "repository": map[string]any{"full_name": run.Repository}, "path": run.WorkflowPath, "event": run.Event, "status": run.Status, "conclusion": run.Conclusion, "html_url": "https://github.com/" + run.Repository + "/actions/runs/9001"})
	f.putBytes(t, "implementation-artifact", []byte("synthetic exact retained archive bytes\n"))
	pin := f.expected.Files["implementation-artifact"]
	f.put(t, "artifacts", map[string]any{"artifacts": []any{map[string]any{"id": 8001, "name": "build-evidence", "expired": false, "size_in_bytes": pin.Bytes, "digest": "sha256:" + pin.SHA256, "workflow_run": map[string]any{"id": run.ID, "head_sha": run.SourceSHA}, "url": "https://api.github.com/repos/" + run.Repository + "/actions/artifacts/8001", "archive_download_url": "https://api.github.com/repos/" + run.Repository + "/actions/artifacts/8001/zip"}}})
	f.expected.Artifacts["build-evidence"] = finalProvenanceArtifact{File: "implementation-artifact", Role: "implementation", SourceSHA: s.SHA, Availability: "github", Run: "build", Metadata: "artifacts", ID: 8001, Name: "build-evidence", URL: "https://github.com/" + run.Repository + "/actions/runs/9001/artifacts/8001"}
	for _, role := range []string{"compiler", "docs"} {
		id := role + "-artifact"
		f.putBytes(t, id, []byte("retained local "+role+" artifact\n"))
		f.expected.Artifacts[id] = finalProvenanceArtifact{File: id, Role: role, SourceSHA: f.expected.Sources[role].SHA, Availability: "local", Name: id}
	}
}
func (f *finalProvenanceFixture) addLedger(t *testing.T) {
	t.Helper()
	mapping, err := BuildRequirementsEvidenceMap(f.contracts)
	if err != nil {
		t.Fatal(err)
	}
	source := f.expected.Sources["implementation"].SHA
	var results []CaseResult
	var rows []ExecutionAuditRow
	var records []EvidenceRecord
	var ids []string
	counts := map[string]int{}
	for _, declared := range mapping.Records {
		status := "PASSED"
		if declared.ID == "AI01" || declared.ID == "RP50" {
			status = "FAILED"
		}
		if declared.ID == "END01" {
			status = "NOT_RUN"
		}
		counts[status]++
		detail := "Synthetic retained " + status + " observation for " + declared.ID
		levels := append([]string{}, declared.RequiredTiers...)
		evidence := []string{"acceptance-local-evidence.json#fixture-" + declared.ID}
		results = append(results, CaseResult{ID: declared.ID, Group: declared.Type, Requirement: declared.Title, TestLevel: strings.Join(declared.RequiredTiers, "+"), ExecutedLevels: levels, Passed: status == "PASSED", Status: status, Detail: detail})
		rows = append(rows, ExecutionAuditRow{ID: declared.ID, Requirement: declared.Title, Status: finalProvenanceAuditStatus(status), Tier: levels, RequiredTiers: append([]string{}, declared.RequiredTiers...), SourceSHA: source, Evidence: evidence, Detail: detail, Scope: "Synthetic original model"})
		records = append(records, EvidenceRecord{EvidenceID: "fixture-" + declared.ID, Requirements: []string{declared.ID}, ImplementationSHA: source, TestID: "synthetic-original:" + declared.ID, Result: "not_run", Detail: detail})
		projection := finalProvenanceLedgerRow{declared.ID, source, status, detail, evidence}
		f.manifest.Ledger = append(f.manifest.Ledger, projection)
		if status != "PASSED" {
			f.manifest.Limitations = append(f.manifest.Limitations, projection)
		}
		ids = append(ids, declared.ID)
	}
	f.putBytes(t, "ledger-artifact", []byte("synthetic retained producer artifact\n"))
	records[0].Artifacts = []EvidenceArtifact{{Path: f.manifest.Files["ledger-artifact"], SHA256: f.expected.Files["ledger-artifact"].SHA256}}
	sort.Slice(f.manifest.Ledger, func(i, j int) bool { return f.manifest.Ledger[i].ID < f.manifest.Ledger[j].ID })
	sort.Slice(f.manifest.Limitations, func(i, j int) bool { return f.manifest.Limitations[i].ID < f.manifest.Limitations[j].ID })
	sort.Strings(ids)
	f.put(t, "report", AcceptanceReport{Timestamp: "2020-01-02T00:01:00Z", Phase: "local", LanguageVersion: "2.0", SchemaFingerprint: "synthetic", TotalCases: 179, PassedCases: counts["PASSED"], FailedCases: counts["FAILED"], SkippedCases: counts["NOT_RUN"], Results: results})
	f.put(t, "audit", map[string]any{"schema_version": "1", "source_sha": source, "acceptance_count": 94, "repair_count": 52, "recovery_gate_count": 33, "closure_complete": false, "rows": rows})
	f.put(t, "local-evidence", records)
	f.put(t, "observations", map[string]any{"source_sha": source, "observations": []any{}})
	after := map[string]any{"contractIds": ids}
	for role, key := range map[string]string{"implementation": "implementation", "compiler": "compilerSource", "docs": "documentation"} {
		s := f.expected.Sources[role]
		after[key] = map[string]any{"head": s.SHA, "tree": s.Tree, "branch": strings.TrimPrefix(s.LocalRef, "refs/heads/"), "statusSha256": finalProvenanceDigest(nil)}
	}
	f.put(t, "source-after", after)
	process := map[string]any{"status": "PRODUCER_TERMINAL", "sourceSha": source, "producerStarted": true, "sourcesUnchanged": true, "gnuTimeoutExit124": false, "formalAcceptanceGranted": false, "hostSignalsObserved": []any{}, "producerExitCode": 1, "producerStartedAt": "2020-01-02T00:00:00Z", "producerFinishedAt": "2020-01-02T00:02:00Z", "finishedAt": "2020-01-02T00:03:00Z", "command": []string{"/usr/bin/timeout", "--signal=TERM", "--kill-after=30s", "80m", "/usr/local/bin/go", "run", "./tools/language-acceptance", "--phase", "local", "--out", f.manifest.Files["report"]}, "budgets": map[string]int{"producerSeconds": 4800, "producerTerminationGraceSeconds": 30, "outerRunSeconds": 5340, "outerStopSeconds": 30, "outerStopReceiptSeconds": 30, "outerTotalSeconds": 5400}, "report": map[string]any{"present": true, "sha256": f.expected.Files["report"].SHA256, "bytes": f.expected.Files["report"].Bytes, "exact179Ids": true, "declaredCountersMatch": true, "originalClosureComplete": false, "rowCount": 179, "uniqueIds": 179, "originalPassedCases": counts["PASSED"], "originalFailedCases": counts["FAILED"], "originalSkippedCases": counts["NOT_RUN"], "statusCounts": counts}}
	f.put(t, "process", process)
	f.expected.Ledger = &finalProvenanceLedger{SourceSHA: source, Report: "report", Audit: "audit", Process: "process", SourceAfter: "source-after", Evidence: "local-evidence", Observations: "observations", ArtifactRoots: []string{f.originals}}
}

func TestFinalProvenanceTruthfulFailedDelivery(t *testing.T) {
	f := finalProvenanceFixtureNew(t)
	originalReport, err := os.ReadFile(f.manifest.Files["report"])
	if err != nil {
		t.Fatal(err)
	}
	result := f.evaluate()
	if result.Status != "PASSED" {
		t.Fatal(result.Status, result.Detail)
	}
	if result.FormalAcceptanceGranted || result.CheckoutProvenanceGranted || result.OriginalClosureComplete || result.StatusCounts["FAILED"] != 2 || result.StatusCounts["NOT_RUN"] != 1 || len(result.Ledger) != 179 || len(result.Limitations) != 3 || result.Runs["build"].Conclusion != "failure" {
		t.Fatalf("delivery erased original failed/pending facts: %+v", result)
	}
	after, err := os.ReadFile(f.manifest.Files["report"])
	if err != nil || !reflect.DeepEqual(originalReport, after) {
		t.Fatal("delivery rewrote the finalized original report")
	}
	for role, before := range result.Before {
		after := result.After[role]
		if !before.Clean || !after.Clean || before.SHA != after.SHA || before.Tree != after.Tree || before.Ref != after.Ref {
			t.Fatal("actual owned Git source observations differ", role)
		}
	}
	t.Run("pending_workflow_remains_pending", func(t *testing.T) {
		run := f.expected.Runs["build"]
		run.Status = "in_progress"
		run.Conclusion = ""
		f.expected.Runs["build"] = run
		o := f.object(t, "run")
		o["status"] = "in_progress"
		o["conclusion"] = nil
		f.put(t, "run", o)
		f.pin(t)
		r := f.evaluate()
		if r.Status != "PASSED" || r.Runs["build"].Status != "in_progress" || r.Runs["build"].Conclusion != "" {
			t.Fatal("truthful pending workflow was promoted or discarded", r.Status, r.Detail)
		}
	})
}

func TestFinalProvenanceIndependentPinsAndSources(t *testing.T) {
	for _, name := range []string{"context_digest", "candidate_self_hash", "missing_original", "historical_ledger", "missing_current_ledger", "dirty_tracked", "staged_index", "hidden_index", "detached_head", "wrong_origin", "source_locator", "output_symlink_alias", "circular_original", "duplicate_original_json"} {
		t.Run(name, func(t *testing.T) {
			f := finalProvenanceFixtureNew(t)
			want := "FAILED"
			repin := true
			switch name {
			case "context_digest":
				f.config.ExpectedSHA256 = strings.Repeat("0", 64)
				repin = false
			case "candidate_self_hash":
				b, _ := os.ReadFile(f.config.ManifestPath)
				f.config.ExpectedSHA256 = finalProvenanceDigest(b)
				repin = false
			case "missing_original":
				if err := os.Remove(f.manifest.Files["run"]); err != nil {
					t.Fatal(err)
				}
				want = "NOT_RUN"
			case "historical_ledger":
				f.expected.Ledger.SourceSHA = strings.Repeat("1", 40)
			case "missing_current_ledger":
				f.expected.Ledger = nil
				want = "NOT_RUN"
			case "dirty_tracked":
				if err := os.WriteFile(filepath.Join(f.manifest.SourceDirectories["implementation"], "tracked.txt"), []byte("changed\n"), 0600); err != nil {
					t.Fatal(err)
				}
			case "staged_index":
				root := f.manifest.SourceDirectories["implementation"]
				p := filepath.Join(root, "tracked.txt")
				old, _ := os.ReadFile(p)
				os.WriteFile(p, []byte("staged\n"), 0600)
				finalProvenanceTestGit(t, root, "add", "tracked.txt")
				os.WriteFile(p, old, 0600)
			case "hidden_index":
				root := f.manifest.SourceDirectories["implementation"]
				finalProvenanceTestGit(t, root, "update-index", "--assume-unchanged", "tracked.txt")
				os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("hidden\n"), 0600)
			case "detached_head":
				finalProvenanceTestGit(t, f.manifest.SourceDirectories["implementation"], "checkout", "--detach")
			case "wrong_origin":
				finalProvenanceTestGit(t, f.manifest.SourceDirectories["implementation"], "remote", "set-url", "origin", "https://github.com/another/repository.git")
			case "source_locator":
				f.manifest.SourceDirectories["implementation"] = f.manifest.SourceDirectories["compiler"]
			case "output_symlink_alias":
				target := filepath.Join(f.manifest.SourceDirectories["compiler"], "empty-owned-output")
				if err := os.Mkdir(target, 0700); err != nil {
					t.Fatal(err)
				}
				alias := filepath.Join(f.root, "compiler-output-alias")
				if err := os.Symlink(target, alias); err != nil {
					t.Fatal(err)
				}
				f.output = alias
			case "circular_original":
				b, _ := os.ReadFile(f.manifest.Files["run"])
				p := filepath.Join(f.output, "run")
				os.WriteFile(p, b, 0600)
				f.manifest.Files["run"] = p
			case "duplicate_original_json":
				b, _ := os.ReadFile(f.manifest.Files["run"])
				b = append([]byte(`{"id":9002,`), b[1:]...)
				f.putBytes(t, "run", b)
			}
			if repin {
				f.pin(t)
			}
			result := f.evaluate()
			if result.Status != want {
				t.Fatalf("%s accepted or misclassified: got %s, wanted %s: %s", name, result.Status, want, result.Detail)
			}
		})
	}
}

func TestFinalProvenanceRejectsOriginalContradictions(t *testing.T) {
	for _, name := range []string{"push_failed", "push_retry", "remote_ref", "remote_readback", "raw_stream", "compiler_readback", "docs_force", "workflow_source", "workflow_attempt", "artifact_run", "artifact_digest", "artifact_link", "archive_bytes", "duplicate_run", "duplicate_artifact", "report_duplicate", "report_missing", "report_title", "report_counters", "report_closure", "audit_source", "audit_status", "audit_dangling", "process_exit", "process_timeout", "process_filtered", "future_process", "source_after", "concealed_failure", "record_artifact_digest", "record_artifact_escape"} {
		t.Run(name, func(t *testing.T) {
			f := finalProvenanceFixtureNew(t)
			switch name {
			case "push_failed", "push_retry":
				o := f.object(t, "implementation-publication")
				if name == "push_failed" {
					o["pushExitCode"] = 1
				} else {
					o["pushAttemptLimit"] = 2
				}
				f.put(t, "implementation-publication", o)
			case "remote_ref":
				p := f.expected.Publications["implementation"]
				o := f.object(t, p.Streams["17-api-ref"].Stdout)
				o["sha"] = strings.Repeat("a", 40)
				f.put(t, p.Streams["17-api-ref"].Stdout, o)
				publication := f.object(t, p.Result)
				finalProvenanceObject(publication, "remoteBinding")["ref"] = o
				for _, raw := range finalProvenanceArray(publication, "steps") {
					step := raw.(map[string]any)
					if finalProvenanceString(step, "name") == "17-api-ref" {
						pin := f.expected.Files[p.Streams["17-api-ref"].Stdout]
						step["stdoutSHA256"], step["stdoutBytes"] = pin.SHA256, pin.Bytes
					}
				}
				f.put(t, p.Result, publication)
			case "remote_readback":
				p := f.expected.Publications["implementation"]
				f.putBytes(t, p.Streams["16-remote-after"].Stdout, []byte(strings.Repeat("a", 40)+"\t"+f.expected.Sources["implementation"].PublishedRef+"\n"))
				publication := f.object(t, p.Result)
				for _, raw := range finalProvenanceArray(publication, "steps") {
					step := raw.(map[string]any)
					if finalProvenanceString(step, "name") == "16-remote-after" {
						pin := f.expected.Files[p.Streams["16-remote-after"].Stdout]
						step["stdoutSHA256"], step["stdoutBytes"] = pin.SHA256, pin.Bytes
					}
				}
				f.put(t, p.Result, publication)
			case "raw_stream":
				p := f.expected.Publications["implementation"]
				f.putBytes(t, p.Streams["16-remote-after"].Stdout, []byte("another ref\n"))
			case "compiler_readback":
				o := f.object(t, "compiler-publication")
				finalProvenanceArray(o, "steps")[2].(map[string]any)["stdout"] = "wrong\n"
				f.put(t, "compiler-publication", o)
			case "docs_force":
				o := f.object(t, "docs-push")
				o["force"] = true
				f.put(t, "docs-push", o)
			case "workflow_source", "workflow_attempt":
				o := f.object(t, "run")
				if name == "workflow_source" {
					o["head_sha"] = strings.Repeat("b", 40)
				} else {
					o["run_attempt"] = 2
				}
				f.put(t, "run", o)
			case "artifact_run", "artifact_digest":
				o := f.object(t, "artifacts")
				a := finalProvenanceArray(o, "artifacts")[0].(map[string]any)
				if name == "artifact_run" {
					finalProvenanceObject(a, "workflow_run")["id"] = 9002
				} else {
					a["digest"] = "sha256:" + strings.Repeat("b", 64)
				}
				f.put(t, "artifacts", o)
			case "artifact_link":
				a := f.expected.Artifacts["build-evidence"]
				a.URL = "https://example.invalid/fabricated"
				f.expected.Artifacts["build-evidence"] = a
			case "archive_bytes":
				p := f.manifest.Files["implementation-artifact"]
				b, _ := os.ReadFile(p)
				b[0] ^= 1
				os.WriteFile(p, b, 0600)
			case "duplicate_run":
				f.expected.Runs["duplicate"] = f.expected.Runs["build"]
			case "duplicate_artifact":
				f.expected.Artifacts["duplicate"] = f.expected.Artifacts["build-evidence"]
			case "report_duplicate", "report_missing", "report_title", "report_counters", "report_closure":
				o := f.object(t, "report")
				a := finalProvenanceArray(o, "results")
				switch name {
				case "report_duplicate":
					a[1] = a[0]
				case "report_missing":
					o["results"] = a[:len(a)-1]
				case "report_title":
					a[0].(map[string]any)["requirement"] = "changed original requirement title"
				case "report_counters":
					o["passed_cases"] = 179
				case "report_closure":
					o["closure_complete"] = true
				}
				f.put(t, "report", o)
				if name == "report_title" {
					process := f.object(t, "process")
					summary := finalProvenanceObject(process, "report")
					pin := f.expected.Files["report"]
					summary["sha256"], summary["bytes"] = pin.SHA256, pin.Bytes
					f.put(t, "process", process)
				}
			case "audit_source", "audit_status", "audit_dangling":
				o := f.object(t, "audit")
				a := finalProvenanceArray(o, "rows")[0].(map[string]any)
				switch name {
				case "audit_source":
					a["source_sha"] = strings.Repeat("a", 40)
				case "audit_status":
					a["status"] = "FAIL"
				case "audit_dangling":
					a["evidence"] = []string{"acceptance-local-evidence.json#missing"}
				}
				f.put(t, "audit", o)
			case "process_exit", "process_timeout", "process_filtered", "future_process":
				o := f.object(t, "process")
				switch name {
				case "process_exit":
					o["producerExitCode"] = 0
				case "process_timeout":
					o["gnuTimeoutExit124"] = true
				case "process_filtered":
					a := finalProvenanceArray(o, "command")
					o["command"] = append(a, "--evidence", "fabricated.json")
				case "future_process":
					o["finishedAt"] = time.Now().Add(time.Hour).UTC().Format(time.RFC3339)
				}
				f.put(t, "process", o)
			case "source_after":
				o := f.object(t, "source-after")
				finalProvenanceObject(o, "implementation")["head"] = strings.Repeat("a", 40)
				f.put(t, "source-after", o)
			case "concealed_failure":
				f.manifest.Limitations = f.manifest.Limitations[1:]
			case "record_artifact_digest":
				f.putBytes(t, "ledger-artifact", []byte("changed artifact bytes with updated outer pin\n"))
			case "record_artifact_escape":
				b, err := os.ReadFile(f.manifest.Files["local-evidence"])
				if err != nil {
					t.Fatal(err)
				}
				var records []EvidenceRecord
				if err = json.Unmarshal(b, &records); err != nil {
					t.Fatal(err)
				}
				path := filepath.Join(f.manifest.SourceDirectories["compiler"], "tracked.txt")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				records[0].Artifacts = []EvidenceArtifact{{Path: path, SHA256: finalProvenanceDigest(data)}}
				f.put(t, "local-evidence", records)
			}
			f.pin(t)
			result := f.evaluate()
			if result.Status != "FAILED" {
				t.Fatalf("original contradiction %s was accepted or skipped: %s %s", name, result.Status, result.Detail)
			}
		})
	}
}

func TestFinalProvenanceDedicatedEntryAndScope(t *testing.T) {
	f := finalProvenanceFixtureNew(t)
	t.Chdir(f.manifest.SourceDirectories["implementation"])
	for _, name := range finalProvenanceEnvironment {
		t.Setenv(name, "")
	}
	r := &Runner{implSHA: f.expected.Sources["implementation"].SHA, results: map[string]CaseResult{}}
	r.runFinalProvenanceRequirement()
	if r.results["END01"].Status != "NOT_RUN" || len(r.records) != 0 {
		t.Fatal("missing independent context granted delivery credit")
	}
	for _, id := range []string{"CI03", "CI04", "END01"} {
		if incompleteLocalAssertions[id] == "" {
			t.Fatal("generic incomplete-scope guard removed", id)
		}
	}
	r.Record("END01", "delivery", "Final pushed branches/artifacts/provenance and clean worktree", "repository", []string{"repository"}, true, "PASSED", "synthetic git status claim")
	if r.results["END01"].Passed || r.results["END01"].Status != "NOT_RUN" {
		t.Fatal("generic historical smoke bypassed the dedicated producer")
	}
	if handled, _ := finalProvenanceCommand([]string{"--phase", "local"}); handled {
		t.Fatal("delivery operation intercepted the original canonical full runner")
	}
	for _, args := range [][]string{{"--verify-final-delivery"}, {"--verify-final-delivery=1", "--out", filepath.Join(f.output, "bad.json")}, {"--verify-final-delivery", "--out", filepath.Join(f.output, "bad.json"), "--phase", "final"}, {"--verify-final-delivery", "--evidence", "forged.json"}} {
		handled, code := finalProvenanceCommand(args)
		if !handled || code != 1 {
			t.Fatal("mixed or incomplete operation did not fail closed", args, handled, code)
		}
	}
	t.Setenv(finalProvenanceEnvironment[0], f.config.ExpectedPath)
	t.Setenv(finalProvenanceEnvironment[1], f.config.ExpectedSHA256)
	t.Setenv(finalProvenanceEnvironment[2], f.config.ManifestPath)
	out := filepath.Join(f.output, "delivery.original.json")
	handled, code := finalProvenanceCommand([]string{"--verify-final-delivery", "--out", out})
	if !handled || code != 0 {
		t.Fatal("actual dedicated CLI route failed", handled, code)
	}
	b, err := os.ReadFile(filepath.Join(f.output, "final-provenance-evidence.json"))
	if err != nil {
		t.Fatal(err)
	}
	var records []EvidenceRecord
	if err = json.Unmarshal(b, &records); err != nil || len(records) != 1 {
		t.Fatal("dedicated entry did not retain one evidence record", err)
	}
	if err = validateFinalProvenanceRecord("END01", records[0]); err != nil {
		t.Fatal("actual record could not be independently revalidated", err)
	}
	t.Run("backdated_receipt_before_producer_completion", func(t *testing.T) {
		raw, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		var receipt finalProvenanceResult
		if err = json.Unmarshal(raw, &receipt); err != nil {
			t.Fatal(err)
		}
		ended, err := finalProvenanceTime(receipt.OriginalProcessFinishedUTC)
		if err != nil {
			t.Fatal(err)
		}
		receipt.StartedUTC = ended.Add(-time.Second).Format(time.RFC3339Nano)
		path := filepath.Join(f.output, "backdated.original.json")
		if err = finalProvenanceWriteNew(path, receipt); err != nil {
			t.Fatal(err)
		}
		raw, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		record := records[0]
		record.Artifacts = append([]EvidenceArtifact{}, record.Artifacts...)
		for i := range record.Artifacts {
			if record.Artifacts[i].Path == out {
				record.Artifacts[i].Path = path
				record.Artifacts[i].SHA256 = finalProvenanceDigest(raw)
			}
		}
		if err = validateFinalProvenanceRecord("END01", record); err == nil || !strings.Contains(err.Error(), "predates completion") {
			t.Fatal("coherently hashed backdated result did not fail the original chronology boundary", err)
		}
	})
	for _, id := range []string{"CI03", "CI04", "RP49", "BRG31"} {
		if err = validateFinalProvenanceRecord(id, records[0]); err == nil {
			t.Fatal("delivery intake granted unrelated coverage", id)
		}
	}
	_, code = finalProvenanceCommand([]string{"--verify-final-delivery", "--out", out})
	if code != 1 {
		t.Fatal("delivery operation overwrote an original receipt")
	}
	if !reflect.DeepEqual(deriveAcceptanceTestIDs("END01", "delivery"), []string{finalProvenanceTestID}) {
		t.Fatal("END01 mapping still points to the historical smoke")
	}
	if _, err = os.Stat(filepath.Join(f.output, "acceptance-report.json")); !os.IsNotExist(err) {
		t.Fatal("delivery-only operation synthesized a new 179 report")
	}
}

// This regression uses the authentic publisher's nested primary-state schema.
// Its payloads and repositories remain synthetic; no private file is opened.
func TestFinalProvenanceWindowsReceiptPrimaryState(t *testing.T) {
	cases := []struct {
		name, want string
	}{
		{"authentic_nested_implementation_receipt", ""},
		{"authentic_nested_compiler_receipt", ""},
		{"empty_status_is_preserved", ""},
		{"missing_validation", "Windows primary checkout observation is missing or malformed"},
		{"top_level_only_substitute", "Windows primary checkout observation is missing or malformed"},
		{"missing_nested_before", "Windows primary checkout observation is missing or malformed"},
		{"empty_both_observations", "Windows primary checkout observation is missing or malformed"},
		{"missing_both_observations", "Windows primary checkout observation is missing or malformed"},
		{"missing_after", "Windows primary checkout observation is missing or malformed"},
		{"wrong_head_type", "Windows primary checkout observation is missing or malformed"},
		{"malformed_head", "Windows primary checkout observation is missing or malformed"},
		{"missing_status", "Windows primary checkout observation is missing or malformed"},
		{"wrong_status_type", "Windows primary checkout observation is missing or malformed"},
		{"missing_fixture_hash", "Windows primary checkout observation is missing or malformed"},
		{"malformed_fixture_hash", "Windows primary checkout observation is missing or malformed"},
		{"different_after_head", "publication changed the unrelated primary checkout"},
		{"different_after_status", "publication changed the unrelated primary checkout"},
		{"different_after_fixture_hash", "publication changed the unrelated primary checkout"},
		{"wrong_before_head_stream", "Windows primary checkout objects disagree with original streams"},
		{"wrong_after_head_stream", "Windows primary checkout objects disagree with original streams"},
		{"wrong_before_status_stream", "Windows primary checkout objects disagree with original streams"},
		{"wrong_after_status_stream", "Windows primary checkout objects disagree with original streams"},
		{"wrong_head_command", "Windows primary checkout command differs from original observation"},
		{"different_primary_target", "Windows primary checkout commands target different checkouts"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := finalProvenanceFixtureNew(t)
			role := "implementation"
			if tc.name == "authentic_nested_compiler_receipt" {
				role = "compiler"
				f.addWindowsPublicationForRole(t, role)
			}
			publication := f.expected.Publications[role]
			o := f.object(t, publication.Result)
			validation := finalProvenanceObject(o, "validation")
			before := finalProvenanceObject(validation, "primaryBefore")
			after := finalProvenanceObject(o, "primaryAfter")
			if _, exists := o["primaryBefore"]; exists || len(before) != 3 || !reflect.DeepEqual(before, after) {
				t.Fatal("fixture does not model the authentic nested publication shape")
			}
			step := func(name string) map[string]any {
				for _, raw := range finalProvenanceArray(o, "steps") {
					item := raw.(map[string]any)
					if finalProvenanceString(item, "name") == name {
						return item
					}
				}
				t.Fatal("missing original synthetic publication step", name)
				return nil
			}
			stream := func(name, value string) {
				id := publication.Streams[name].Stdout
				f.putBytes(t, id, []byte(value))
				pin := f.expected.Files[id]
				item := step(name)
				item["stdoutSHA256"], item["stdoutBytes"] = pin.SHA256, pin.Bytes
			}
			switch tc.name {
			case "empty_status_is_preserved":
				before["status"], after["status"] = "", ""
				stream("04-primary-status-before", "")
				stream("20-primary-status-after", "")
			case "missing_validation":
				delete(o, "validation")
			case "top_level_only_substitute":
				o["primaryBefore"] = before
				delete(o, "validation")
			case "missing_nested_before":
				delete(validation, "primaryBefore")
			case "empty_both_observations":
				validation["primaryBefore"], o["primaryAfter"] = map[string]any{}, map[string]any{}
			case "missing_both_observations":
				delete(validation, "primaryBefore")
				delete(o, "primaryAfter")
			case "missing_after":
				delete(o, "primaryAfter")
			case "wrong_head_type":
				before["head"], after["head"] = 3, 3
			case "malformed_head":
				before["head"], after["head"] = strings.Repeat("g", 40), strings.Repeat("g", 40)
			case "missing_status":
				delete(before, "status")
				delete(after, "status")
			case "wrong_status_type":
				before["status"], after["status"] = false, false
			case "missing_fixture_hash":
				delete(before, "preservedFixtureSHA256")
				delete(after, "preservedFixtureSHA256")
			case "malformed_fixture_hash":
				before["preservedFixtureSHA256"], after["preservedFixtureSHA256"] = "bad", "bad"
			case "different_after_head":
				after["head"] = strings.Repeat("4", 40)
			case "different_after_status":
				after["status"] = "?? changed-synthetic-fixture.cherri"
			case "different_after_fixture_hash":
				after["preservedFixtureSHA256"] = strings.Repeat("9", 64)
			case "wrong_before_head_stream":
				stream("03-primary-head-before", strings.Repeat("4", 40)+"\n")
			case "wrong_after_head_stream":
				stream("19-primary-head-after", strings.Repeat("4", 40)+"\n")
			case "wrong_before_status_stream":
				stream("04-primary-status-before", "?? changed-synthetic-fixture.cherri\n")
			case "wrong_after_status_stream":
				stream("20-primary-status-after", "?? changed-synthetic-fixture.cherri\n")
			case "wrong_head_command":
				step("03-primary-head-before")["argv"] = []string{"git", "-C", "C:/synthetic-primary-checkout", "rev-parse", "HEAD^"}
			case "different_primary_target":
				step("20-primary-status-after")["argv"] = []string{"git", "-C", "C:/different-synthetic-checkout", "status", "--porcelain=v1"}
			}
			f.put(t, publication.Result, o)
			f.pin(t)
			in, err := finalProvenanceLoad(f.config, f.output)
			if err != nil {
				t.Fatal("coherent independently pinned fixture did not load", err)
			}
			err = finalProvenanceValidatePublication(in, role)
			if tc.want == "" {
				if err != nil {
					t.Fatal("authentic Windows publication shape was rejected", err)
				}
			} else if err == nil || err.Error() != tc.want {
				t.Fatalf("causal original publication boundary: got %v, want %q", err, tc.want)
			}
		})
	}
}

func TestFinalProvenanceRejectsAuditDetailDrift(t *testing.T) {
	var fixture = finalProvenanceFixtureNew(t)
	var baseline = fixture.evaluate()
	if baseline.Status != "PASSED" {
		t.Fatalf("unchanged synthetic delivery fixture was rejected: %s: %s", baseline.Status, baseline.Detail)
	}
	var audit = fixture.object(t, "audit")
	var first = audit["rows"].([]any)[0].(map[string]any)
	var id = first["id"].(string)
	first["detail"] = first["detail"].(string) + "; coherently pinned audit-only diagnostic"
	fixture.put(t, "audit", audit)
	// Rebind the original and both outer contexts so a stale digest cannot
	// mask removal of the canonical report/audit detail comparison.
	fixture.pin(t)
	var result = fixture.evaluate()
	var want = "original report/audit/contract disagreement at requirement " + id
	if result.Status != "FAILED" || result.Detail != want {
		t.Fatalf("audit-only detail drift was not rejected at its exact predicate: status=%s detail=%q want=%q", result.Status, result.Detail, want)
	}
}

// These fixtures model a failed publication followed by a separate read-only
// observation. They are never originals or network publication evidence.
type finalProvenanceReconciledFixture struct {
	f                    *finalProvenanceFixture
	p                    finalProvenancePublication
	original, supplement map[string]any
}

func finalProvenanceReconciledFixtureNew(t *testing.T) *finalProvenanceReconciledFixture {
	t.Helper()
	f := &finalProvenanceFixture{originals: t.TempDir(), expected: finalProvenanceExpected{Sources: map[string]finalProvenanceSource{}, Files: map[string]finalProvenanceFile{}, Publications: map[string]finalProvenancePublication{}}, manifest: finalProvenanceManifest{Files: map[string]string{}}}
	s := finalProvenanceSource{Repository: "davidpovarsky/cherri", SHA: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), LocalRef: "refs/heads/synthetic-source", PublishedRef: "refs/heads/synthetic-source"}
	f.expected.Sources["compiler"] = s
	r := &finalProvenanceReconciliation{Result: "reconciliation-result", OriginalArchive: "publication-archive", Archive: "reconciliation-archive", OriginalReceipts: map[string]string{}, Streams: map[string]finalProvenanceStreams{}, Receipts: map[string]string{}}
	p := finalProvenancePublication{Format: "windows-create-only-reconciled/1", Result: "publication-result", Streams: map[string]finalProvenanceStreams{}, Reconciliation: r}
	git, repo, primary, gh := "C:/synthetic/Git/git.exe", "C:/synthetic/publication.git", "C:/synthetic/unrelated-primary", "C:/synthetic/GitHub/gh.exe"
	bundle, prerequisite := "C:/synthetic/source.bundle", strings.Repeat("d", 40)
	gitArgs := func(args ...string) []string { return append([]string{git, "-C", repo}, args...) }
	primaryArgs := func(args ...string) []string { return append([]string{git, "-C", primary}, args...) }
	apiRef := []string{gh, "api", "-H", "Cache-Control: no-cache", "repos/" + s.Repository + "/git/ref/heads/synthetic-source", "--jq", "{ref: .ref, sha: .object.sha, type: .object.type}"}
	apiCommit := []string{gh, "api", "-H", "Cache-Control: no-cache", "repos/" + s.Repository + "/git/commits/" + s.SHA, "--jq", "{sha: .sha, tree: .tree.sha, parents: [.parents[].sha]}"}
	refsArgs := gitArgs("for-each-ref", "--format=%(refname) %(objectname)")
	remoteArgs := gitArgs("ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef)
	state := map[string]any{"head": strings.Repeat("3", 40), "status": "?? synthetic-preserved.cherri", "preservedFixtureSHA256": strings.Repeat("8", 64)}
	origin, readback := "https://github.com/"+s.Repository+".git", s.SHA+"\t"+s.PublishedRef
	base := []string{"refs/preparation/base " + prerequisite}
	imported := append(append([]string{}, base...), s.PublishedRef+" "+s.SHA)
	final := append(append([]string{}, imported...), "refs/remotes/origin/synthetic-source "+s.SHA)
	ref := map[string]any{"ref": s.PublishedRef, "sha": s.SHA, "type": "commit"}
	commit := map[string]any{"sha": s.SHA, "tree": s.Tree, "parents": []string{prerequisite}}
	refJSON, _ := json.Marshal(ref)
	commitJSON, _ := json.Marshal(commit)
	oldNames := []string{"01-origin", "02-base-refs", "03-primary-head-before", "04-primary-status-before", "05-transfer", "06-bundle-heads", "07-bundle-verify", "08-import", "09-commit", "10-tree", "11-ancestry", "12-connectivity", "13-imported-refs", "14-remote-before", "15-push", "16-remote-after", "17-api-ref", "19-primary-head-after", "20-primary-status-after"}
	newNames := []string{"01-origin", "02-primary-head-before", "03-primary-status-before", "04-remote-readback", "05-api-ref", "06-api-commit", "07-final-refs", "08-primary-head-after", "09-primary-status-after"}
	oldArgs := map[string][]string{
		"01-origin": gitArgs("remote", "get-url", "origin"), "02-base-refs": refsArgs,
		"03-primary-head-before": primaryArgs("rev-parse", "HEAD"), "04-primary-status-before": primaryArgs("status", "--porcelain=v1"),
		"05-transfer":     {"C:/synthetic/scp.exe", "-B", "-o", "BatchMode=yes", "-o", "ConnectTimeout=12", "torah-dev:/synthetic/source.bundle", "torah-dev:/synthetic/bundle-handoff.json", "."},
		"06-bundle-heads": gitArgs("bundle", "list-heads", bundle), "07-bundle-verify": gitArgs("bundle", "verify", bundle),
		"08-import": gitArgs("fetch", "--no-tags", "--no-recurse-submodules", bundle, s.PublishedRef+":"+s.PublishedRef),
		"09-commit": gitArgs("rev-parse", s.PublishedRef), "10-tree": gitArgs("rev-parse", s.PublishedRef+"^{tree}"),
		"11-ancestry": gitArgs("merge-base", "--is-ancestor", prerequisite, s.SHA), "12-connectivity": gitArgs("fsck", "--connectivity-only", "--no-reflogs", s.PublishedRef),
		"13-imported-refs": refsArgs, "14-remote-before": remoteArgs,
		"15-push":         gitArgs("push", "--porcelain", "--force-with-lease="+s.PublishedRef+":", "origin", s.SHA+":"+s.PublishedRef),
		"16-remote-after": remoteArgs, "17-api-ref": apiRef,
		"19-primary-head-after": primaryArgs("rev-parse", "HEAD"), "20-primary-status-after": primaryArgs("status", "--porcelain=v1"),
	}
	newArgs := map[string][]string{
		"01-origin": gitArgs("remote", "get-url", "origin"), "02-primary-head-before": primaryArgs("rev-parse", "HEAD"), "03-primary-status-before": primaryArgs("status", "--porcelain=v1"),
		"04-remote-readback": remoteArgs, "05-api-ref": apiRef, "06-api-commit": apiCommit, "07-final-refs": refsArgs,
		"08-primary-head-after": primaryArgs("rev-parse", "HEAD"), "09-primary-status-after": primaryArgs("status", "--porcelain=v1"),
	}
	oldOutput := map[string]string{"01-origin": origin + "\n", "02-base-refs": strings.Join(base, "\n") + "\n", "03-primary-head-before": state["head"].(string) + "\n", "04-primary-status-before": state["status"].(string) + "\n", "06-bundle-heads": s.SHA + " " + s.PublishedRef + "\n", "09-commit": s.SHA + "\n", "10-tree": s.Tree + "\n", "13-imported-refs": strings.Join(imported, "\n") + "\n", "15-push": "To " + origin + "\n*\t" + s.SHA + ":" + s.PublishedRef + "\t[new branch]\nDone\n", "16-remote-after": readback + "\n", "19-primary-head-after": state["head"].(string) + "\n", "20-primary-status-after": state["status"].(string) + "\n"}
	newOutput := map[string]string{"01-origin": origin + "\n", "02-primary-head-before": state["head"].(string) + "\n", "03-primary-status-before": state["status"].(string) + "\n", "04-remote-readback": readback + "\n", "05-api-ref": string(refJSON) + "\n", "06-api-commit": string(commitJSON) + "\n", "07-final-refs": strings.Join(final, "\n") + "\n", "08-primary-head-after": state["head"].(string) + "\n", "09-primary-status-after": state["status"].(string) + "\n"}
	makeSteps := func(names []string, args map[string][]string, outputs map[string]string, streams map[string]finalProvenanceStreams, receipts map[string]string, isOriginal bool) []any {
		var steps []any
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
		prefix := "old-"
		if !isOriginal {
			start = start.Add(2 * time.Minute)
			prefix = "new-"
		}
		for i, name := range names {
			pair := finalProvenanceStreams{prefix + name + ".stdout", prefix + name + ".stderr"}
			f.putBytes(t, pair.Stdout, []byte(outputs[name]))
			f.putBytes(t, pair.Stderr, []byte{})
			streams[name], receipts[name] = pair, prefix+name+".receipt"
			seconds, duration, limit, code, timedOut := i*2, 1, 30, 0, false
			var kill any
			if isOriginal {
				switch name {
				case "05-transfer", "15-push":
					limit = 120
				case "08-import", "12-connectivity":
					limit = 60
				case "14-remote-before":
					code = 2
				case "17-api-ref":
					code = 1
					timedOut = true
					duration = 31
					kill = 0
				}
				if i > 16 {
					seconds += 30
				}
			}
			cwd := any(nil)
			if isOriginal && name == "05-transfer" {
				cwd = "C:/synthetic"
			}
			steps = append(steps, map[string]any{"name": name, "argv": args[name], "cwd": cwd, "startedAtUtc": start.Add(time.Duration(seconds) * time.Second).Format(time.RFC3339), "finishedAtUtc": start.Add(time.Duration(seconds+duration) * time.Second).Format(time.RFC3339), "seconds": duration, "exitCode": code, "timeoutSeconds": limit, "timedOut": timedOut, "ownedTreeKillExit": kill, "stdoutSHA256": f.expected.Files[pair.Stdout].SHA256, "stderrSHA256": f.expected.Files[pair.Stderr].SHA256, "stdoutBytes": len(outputs[name]), "stderrBytes": 0})
		}
		return steps
	}
	oldSteps := makeSteps(oldNames, oldArgs, oldOutput, p.Streams, r.OriginalReceipts, true)
	newSteps := makeSteps(newNames, newArgs, newOutput, r.Streams, r.Receipts, false)
	var allowlist []any
	for _, name := range newNames {
		allowlist = append(allowlist, map[string]any{"name": name, "argv": newArgs[name], "timeoutSeconds": 30})
	}
	original := map[string]any{"startedAtUtc": "2020-01-01T00:00:00Z", "finishedAtUtc": "2020-01-01T00:01:30Z", "sourceCommit": s.SHA, "sourceTree": s.Tree, "ref": s.PublishedRef, "repo": repo, "prerequisites": []string{prerequisite}, "status": "PUSH_ATTEMPTED_REMOTE_OUTCOME_UNVERIFIED", "remoteVerificationError": "RuntimeError: 17-api-ref failed; exit=1 timeout=True", "pushAttempted": true, "pushTimedOut": false, "liveRemoteBuilderTouched": false, "pushAttemptLimit": 1, "pushExitCode": 0, "actionsDispatches": 0, "destinationMustBeAbsent": true, "primaryMutationAuthorized": false, "primaryUnchanged": true, "validation": map[string]any{"primaryBefore": state, "prerequisites": []string{prerequisite}, "importedCommit": s.SHA, "importedTree": s.Tree, "remoteRefAbsent": true, "localRefs": imported}, "primaryAfter": state, "remoteRefReadback": map[string]any{"code": 0, "timedOut": false, "stdout": readback}, "steps": oldSteps}
	supplement := map[string]any{"schema": "cherri-windows-publication-readonly-reconciliation/1", "startedAtUtc": "2020-01-01T00:02:00Z", "finishedAtUtc": "2020-01-01T00:02:20Z", "sourceCommit": s.SHA, "sourceTree": s.Tree, "ref": s.PublishedRef, "status": "READ_ONLY_REMOTE_IDENTITY_VERIFIED", "error": nil, "reconciliationAttemptLimit": 1, "pushPerformed": false, "importPerformed": false, "actionsDispatches": 0, "privatePayloadExported": false, "originalsUnchanged": true, "primaryUnchanged": true, "publicationOriginalStatusRewritten": false, "canonicalPublicationAcceptanceGranted": false, "origin": origin, "primaryBefore": state, "primaryAfter": state, "remoteRefReadback": map[string]any{"code": 0, "timedOut": false, "stdout": readback}, "remoteBinding": map[string]any{"ref": ref, "commit": commit, "verifiedAtUtc": "2020-01-01T00:02:19Z"}, "finalRefs": final, "readOnlyAllowlist": allowlist, "steps": newSteps}
	x := &finalProvenanceReconciledFixture{f: f, p: p, original: original, supplement: supplement}
	x.save(t)
	// Decode separate JSON objects so an adversarial change cannot accidentally
	// mutate all observations through a shared synthetic map.
	x.original = f.object(t, p.Result)
	x.supplement = f.object(t, r.Result)
	return x
}

func (x *finalProvenanceReconciledFixture) step(o map[string]any, name string) map[string]any {
	for _, raw := range finalProvenanceArray(o, "steps") {
		if s, ok := raw.(map[string]any); ok && finalProvenanceString(s, "name") == name {
			return s
		}
	}
	return nil
}

func (x *finalProvenanceReconciledFixture) archive(t *testing.T, original bool, mutate func(map[string][]byte)) {
	t.Helper()
	r := x.p.Reconciliation
	archive, result, resultMember, streams, receipts := r.Archive, r.Result, "reconciliation.result.json", r.Streams, r.Receipts
	files := map[string][]byte{"reconciliation.start.json": []byte("{\"synthetic\":true}\n")}
	if original {
		archive, result, resultMember, streams, receipts = r.OriginalArchive, x.p.Result, "publication.result.json", x.p.Streams, r.OriginalReceipts
		files = map[string][]byte{}
		for _, name := range []string{"preflight-context.original.json", "publication.push-attempt.json", "publication.start.json", "publication.validation.json"} {
			files[name] = []byte("{\"synthetic\":true}\n")
		}
	}
	read := func(id string) []byte {
		b, err := os.ReadFile(x.f.manifest.Files[id])
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	files[resultMember] = read(result)
	for name, pair := range streams {
		files[name+".stdout.original"] = read(pair.Stdout)
		files[name+".stderr.original"] = read(pair.Stderr)
		files[name+".receipt.json"] = read(receipts[name])
	}
	if mutate != nil {
		mutate(files)
	}
	data, inventory := ciTestZIP(t, files, zip.Deflate)
	if original {
		r.OriginalMembers = inventory
	} else {
		r.Members = inventory
	}
	x.f.putBytes(t, archive, data)
}

func (x *finalProvenanceReconciledFixture) save(t *testing.T) {
	t.Helper()
	r := x.p.Reconciliation
	for _, phase := range []struct {
		o        map[string]any
		receipts map[string]string
	}{{x.original, r.OriginalReceipts}, {x.supplement, r.Receipts}} {
		for _, raw := range finalProvenanceArray(phase.o, "steps") {
			step := raw.(map[string]any)
			if id := phase.receipts[finalProvenanceString(step, "name")]; id != "" {
				x.f.put(t, id, step)
			}
		}
	}
	x.f.put(t, x.p.Result, x.original)
	x.archive(t, true, nil)
	x.saveSupplementArchive(t)
	x.f.expected.Publications["compiler"] = x.p
}

func (x *finalProvenanceReconciledFixture) saveSupplementArchive(t *testing.T) {
	t.Helper()
	r := x.p.Reconciliation
	x.supplement["originalPublication"] = map[string]any{"status": x.original["status"], "finishedAtUtc": x.original["finishedAtUtc"], "result": x.f.expected.Files[x.p.Result], "archive": x.f.expected.Files[r.OriginalArchive]}
	x.f.put(t, r.Result, x.supplement)
	x.archive(t, false, nil)
}

func (x *finalProvenanceReconciledFixture) stream(t *testing.T, original bool, name, field string, b []byte) {
	t.Helper()
	o, streams := x.supplement, x.p.Reconciliation.Streams
	if original {
		o, streams = x.original, x.p.Streams
	}
	id := streams[name].Stdout
	if field == "stderr" {
		id = streams[name].Stderr
	}
	x.f.putBytes(t, id, b)
	step := x.step(o, name)
	step[field+"SHA256"] = finalProvenanceDigest(b)
	step[field+"Bytes"] = len(b)
}

func (x *finalProvenanceReconciledFixture) inputs(t *testing.T) *finalProvenanceInputs {
	t.Helper()
	in := &finalProvenanceInputs{Expected: x.f.expected, Manifest: x.f.manifest, JSON: map[string]map[string]any{}, Bytes: map[string][]byte{}}
	for id, pin := range x.f.expected.Files {
		b, err := finalProvenanceRead(x.f.manifest.Files[id], pin, true)
		if err != nil {
			t.Fatal(err)
		}
		in.Bytes[id] = b
	}
	return in
}

func TestFinalProvenanceReconciledWindowsPublication(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*finalProvenanceReconciledFixture)
		after      func(*finalProvenanceReconciledFixture)
	}{
		{name: "separate_failed_original_and_successful_read_only_supplement"},
		{name: "legacy_format_does_not_accept_failed_original", want: "Windows publication did not establish", after: func(x *finalProvenanceReconciledFixture) {
			p := x.f.expected.Publications["compiler"]
			p.Format = "windows-create-only/1"
			x.f.expected.Publications["compiler"] = p
		}},
		{name: "missing_supplement_binding", want: "requires separate", after: func(x *finalProvenanceReconciledFixture) {
			p := x.f.expected.Publications["compiler"]
			p.Reconciliation = nil
			x.f.expected.Publications["compiler"] = p
		}},
		{name: "missing_archive", want: "missing a pinned", after: func(x *finalProvenanceReconciledFixture) {
			delete(x.f.expected.Files, x.p.Reconciliation.OriginalArchive)
		}},
		{name: "rewritten_original_status", want: "narrow original API-timeout", mutate: func(x *finalProvenanceReconciledFixture) { x.original["status"] = "PUBLISHED_REMOTE_VERIFIED" }},
		{name: "different_original_failure", want: "narrow original API-timeout", mutate: func(x *finalProvenanceReconciledFixture) { x.original["remoteVerificationError"] = "other failure" }},
		{name: "failed_original_push", want: "exactly one successful original push", mutate: func(x *finalProvenanceReconciledFixture) { x.original["pushExitCode"] = 1 }},
		{name: "original_push_timeout", want: "narrow original API-timeout", mutate: func(x *finalProvenanceReconciledFixture) { x.original["pushTimedOut"] = true }},
		{name: "second_push_allowed", want: "exactly one successful original push", mutate: func(x *finalProvenanceReconciledFixture) { x.original["pushAttemptLimit"] = 2 }},
		{name: "wrong_supplement_schema", want: "schema or terminal outcome", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["schema"] = "made-up/1" }},
		{name: "supplement_not_verified", want: "schema or terminal outcome", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["status"] = "PENDING" }},
		{name: "supplement_repush", want: "operation or original-preservation scope", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["pushPerformed"] = true }},
		{name: "supplement_import", want: "operation or original-preservation scope", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["importPerformed"] = true }},
		{name: "originals_rewritten", want: "operation or original-preservation scope", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["originalsUnchanged"] = false }},
		{name: "invented_acceptance_in_supplement", want: "operation or original-preservation scope", mutate: func(x *finalProvenanceReconciledFixture) {
			x.supplement["canonicalPublicationAcceptanceGranted"] = true
		}},
		{name: "different_source", want: "source tuple differs", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["sourceCommit"] = strings.Repeat("f", 40) }},
		{name: "different_ref", want: "source tuple differs", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["ref"] = "refs/heads/other" }},
		{name: "different_tree", want: "source tuple differs", mutate: func(x *finalProvenanceReconciledFixture) { x.original["sourceTree"] = strings.Repeat("f", 40) }},
		{name: "wrong_original_digest", want: "different original result or archive", after: func(x *finalProvenanceReconciledFixture) {
			o := x.f.object(t, x.p.Reconciliation.Result)
			finalProvenanceObject(finalProvenanceObject(o, "originalPublication"), "result")["sha256"] = strings.Repeat("0", 64)
			x.f.put(t, x.p.Reconciliation.Result, o)
		}},
		{name: "wrong_original_archive_bytes", want: "different original result or archive", after: func(x *finalProvenanceReconciledFixture) {
			o := x.f.object(t, x.p.Reconciliation.Result)
			finalProvenanceObject(finalProvenanceObject(o, "originalPublication"), "archive")["bytes"] = 1
			x.f.put(t, x.p.Reconciliation.Result, o)
		}},
		{name: "different_original_finish", want: "original status and finish", after: func(x *finalProvenanceReconciledFixture) {
			o := x.f.object(t, x.p.Reconciliation.Result)
			finalProvenanceObject(o, "originalPublication")["finishedAtUtc"] = "2020-01-01T00:01:31Z"
			x.f.put(t, x.p.Reconciliation.Result, o)
		}},
		{name: "replayed_supplement_time", want: "operation chronology", mutate: func(x *finalProvenanceReconciledFixture) { x.supplement["startedAtUtc"] = "2020-01-01T00:01:00Z" }},
		{name: "missing_original_step", want: "inventory is incomplete", mutate: func(x *finalProvenanceReconciledFixture) {
			x.original["steps"] = finalProvenanceArray(x.original, "steps")[:18]
		}},
		{name: "missing_supplement_step", want: "inventory is incomplete", mutate: func(x *finalProvenanceReconciledFixture) {
			x.supplement["steps"] = finalProvenanceArray(x.supplement, "steps")[:8]
		}},
		{name: "reordered_supplement_steps", want: "identity or order differs", mutate: func(x *finalProvenanceReconciledFixture) {
			steps := finalProvenanceArray(x.supplement, "steps")
			steps[4], steps[5] = steps[5], steps[4]
		}},
		{name: "missing_original_stream", want: "inventory is incomplete", after: func(x *finalProvenanceReconciledFixture) { delete(x.p.Streams, "17-api-ref") }},
		{name: "replayed_receipt_identity", want: "absent or replayed", after: func(x *finalProvenanceReconciledFixture) {
			x.p.Reconciliation.Receipts["06-api-commit"] = x.p.Reconciliation.Receipts["05-api-ref"]
		}},
		{name: "changed_process_receipt", want: "differs from the original process receipt", after: func(x *finalProvenanceReconciledFixture) {
			id := x.p.Reconciliation.Receipts["05-api-ref"]
			o := x.f.object(t, id)
			o["seconds"] = 99
			x.f.put(t, id, o)
		}},
		{name: "changed_stream_hash", want: "not bound to original bytes", mutate: func(x *finalProvenanceReconciledFixture) {
			x.step(x.supplement, "05-api-ref")["stdoutSHA256"] = strings.Repeat("0", 64)
		}},
		{name: "raised_original_timeout", want: "original timeout budget differs", mutate: func(x *finalProvenanceReconciledFixture) { x.step(x.original, "17-api-ref")["timeoutSeconds"] = 60 }},
		{name: "nonempty_original_timeout_output", want: "empty-stream cleaned-up failure", mutate: func(x *finalProvenanceReconciledFixture) {
			x.stream(t, true, "17-api-ref", "stdout", []byte("partial response"))
		}},
		{name: "uncleaned_original_timeout", want: "empty-stream cleaned-up failure", mutate: func(x *finalProvenanceReconciledFixture) { x.step(x.original, "17-api-ref")["ownedTreeKillExit"] = 1 }},
		{name: "unverified_absent_ref", want: "original timeout budget differs", mutate: func(x *finalProvenanceReconciledFixture) { x.step(x.original, "14-remote-before")["exitCode"] = 0 }},
		{name: "second_timed_out_step", want: "original timeout budget differs", mutate: func(x *finalProvenanceReconciledFixture) { x.step(x.supplement, "05-api-ref")["timedOut"] = true }},
		{name: "different_create_only_lease", want: "single-source protocol", mutate: func(x *finalProvenanceReconciledFixture) {
			a, _ := finalProvenanceStrings(x.step(x.original, "15-push")["argv"])
			a[5] = "--force"
			x.step(x.original, "15-push")["argv"] = a
		}},
		{name: "hidden_push_in_transfer_step", want: "bounded two-public-file copy", mutate: func(x *finalProvenanceReconciledFixture) {
			x.step(x.original, "05-transfer")["argv"] = x.step(x.original, "15-push")["argv"]
		}},
		{name: "verification_before_observations_finish", want: "verification time is outside", mutate: func(x *finalProvenanceReconciledFixture) {
			finalProvenanceObject(x.supplement, "remoteBinding")["verifiedAtUtc"] = "2020-01-01T00:02:01Z"
		}},
		{name: "malformed_supplement_archive", want: "reconciled publication archive:", after: func(x *finalProvenanceReconciledFixture) {
			x.f.putBytes(t, x.p.Reconciliation.Archive, []byte("not a ZIP"))
		}},
		{name: "swapped_supplement_archive", want: "reconciled publication archive:", after: func(x *finalProvenanceReconciledFixture) {
			b, _ := os.ReadFile(x.f.manifest.Files[x.p.Reconciliation.OriginalArchive])
			x.f.putBytes(t, x.p.Reconciliation.Archive, b)
		}},
		{name: "original_archive_result_drift", want: "archive member differs from independent original", after: func(x *finalProvenanceReconciledFixture) {
			x.archive(t, true, func(files map[string][]byte) {
				files["publication.result.json"] = append(files["publication.result.json"], '\n')
			})
			x.saveSupplementArchive(t)
		}},
		{name: "supplement_archive_receipt_drift", want: "archive member differs from independent original", after: func(x *finalProvenanceReconciledFixture) {
			x.archive(t, false, func(files map[string][]byte) {
				files["05-api-ref.receipt.json"] = append(files["05-api-ref.receipt.json"], '\n')
			})
		}},
		{name: "supplement_archive_stream_drift", want: "archive member differs from independent original", after: func(x *finalProvenanceReconciledFixture) {
			x.archive(t, false, func(files map[string][]byte) {
				files["05-api-ref.stdout.original"] = append(files["05-api-ref.stdout.original"], '\n')
			})
		}},
		{name: "missing_verification_time", want: "verification time is outside", mutate: func(x *finalProvenanceReconciledFixture) {
			delete(finalProvenanceObject(x.supplement, "remoteBinding"), "verifiedAtUtc")
		}},
		{name: "verification_after_supplement_finish", want: "verification time is outside", mutate: func(x *finalProvenanceReconciledFixture) {
			finalProvenanceObject(x.supplement, "remoteBinding")["verifiedAtUtc"] = "2020-01-01T00:02:21Z"
		}},
		{name: "wrong_read_only_allowlist", want: "allowlisted commands or limits", mutate: func(x *finalProvenanceReconciledFixture) {
			finalProvenanceArray(x.supplement, "readOnlyAllowlist")[4].(map[string]any)["timeoutSeconds"] = 60
		}},
		{name: "mutating_api_command", want: "not the exact GET", mutate: func(x *finalProvenanceReconciledFixture) {
			a, _ := finalProvenanceStrings(x.step(x.supplement, "05-api-ref")["argv"])
			a = append(a, "--method", "POST")
			x.step(x.supplement, "05-api-ref")["argv"] = a
			finalProvenanceArray(x.supplement, "readOnlyAllowlist")[4].(map[string]any)["argv"] = a
		}},
		{name: "api_ref_wrong_source", want: "expected source and tree", mutate: func(x *finalProvenanceReconciledFixture) {
			o := map[string]any{"ref": x.f.expected.Sources["compiler"].PublishedRef, "sha": strings.Repeat("e", 40), "type": "commit"}
			b, _ := json.Marshal(o)
			x.stream(t, false, "05-api-ref", "stdout", b)
		}},
		{name: "changed_api_projection", want: "changed the raw API binding", mutate: func(x *finalProvenanceReconciledFixture) {
			finalProvenanceObject(finalProvenanceObject(x.supplement, "remoteBinding"), "commit")["tree"] = strings.Repeat("e", 40)
		}},
		{name: "changed_primary_fixture", want: "preserved primary checkout", mutate: func(x *finalProvenanceReconciledFixture) {
			finalProvenanceObject(x.supplement, "primaryAfter")["preservedFixtureSHA256"] = strings.Repeat("e", 64)
		}},
		{name: "different_primary_stream", want: "primary checkout commands or original streams", mutate: func(x *finalProvenanceReconciledFixture) {
			x.stream(t, false, "09-primary-status-after", "stdout", []byte(" M tracked.txt\n"))
		}},
		{name: "extra_final_ref", want: "beyond the sole source tracking ref", mutate: func(x *finalProvenanceReconciledFixture) {
			id := x.p.Reconciliation.Streams["07-final-refs"].Stdout
			b, _ := os.ReadFile(x.f.manifest.Files[id])
			x.stream(t, false, "07-final-refs", "stdout", append(b, []byte("refs/heads/unrelated "+strings.Repeat("e", 40)+"\n")...))
		}},
		{name: "missing_final_tracking_ref", want: "beyond the sole source tracking ref", mutate: func(x *finalProvenanceReconciledFixture) {
			x.stream(t, false, "07-final-refs", "stdout", []byte("refs/preparation/base "+strings.Repeat("d", 40)+"\n"+x.f.expected.Sources["compiler"].PublishedRef+" "+strings.Repeat("a", 40)+"\n"))
		}},
		{name: "changed_final_inventory_projection", want: "inventory disagrees", mutate: func(x *finalProvenanceReconciledFixture) {
			x.supplement["finalRefs"] = []string{"refs/heads/substitute " + strings.Repeat("e", 40)}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := finalProvenanceReconciledFixtureNew(t)
			if tc.mutate != nil {
				tc.mutate(x)
			}
			x.save(t)
			if tc.after != nil {
				tc.after(x)
			}
			err := finalProvenanceValidatePublication(x.inputs(t), "compiler")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("causal reconciliation boundary: got %v, want %q", err, tc.want)
			}
		})
	}
}

// An explicit external invocation can exercise real hash-pinned originals. The
// ordinary suite has no dependency on an operator's retained evidence directory.
func TestFinalProvenanceReconciledWindowsOriginalIntake(t *testing.T) {
	path, digest := os.Getenv("CHERRI_RECONCILED_PUBLICATION_EXPECTED"), os.Getenv("CHERRI_RECONCILED_PUBLICATION_EXPECTED_SHA256")
	if path == "" && digest == "" {
		t.Skip("no independently pinned original publication intake selected")
	}
	var spec struct {
		Schema      string                         `json:"schema"`
		Source      finalProvenanceSource          `json:"source"`
		Publication finalProvenancePublication     `json:"publication"`
		Files       map[string]finalProvenanceFile `json:"files"`
		Paths       map[string]string              `json:"paths"`
	}
	if err := finalProvenanceReadContext(path, digest, &spec); err != nil {
		t.Fatal(err)
	}
	if spec.Schema != "cherri-reconciled-publication-original-intake/1" || spec.Publication.Format != "windows-create-only-reconciled/1" || len(spec.Files) != len(spec.Paths) {
		t.Fatal("invalid external original-intake specification")
	}
	in := &finalProvenanceInputs{Expected: finalProvenanceExpected{Sources: map[string]finalProvenanceSource{"compiler": spec.Source}, Publications: map[string]finalProvenancePublication{"compiler": spec.Publication}, Files: spec.Files}, JSON: map[string]map[string]any{}, Bytes: map[string][]byte{}}
	for id, pin := range spec.Files {
		b, err := finalProvenanceRead(spec.Paths[id], pin, true)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		in.Bytes[id] = b
	}
	if err := finalProvenanceValidatePublication(in, "compiler"); err != nil {
		t.Fatal(err)
	}
}

// These authored receipts are test-only controls. They are never represented as
// an actual publication, a Windows receipt, or authenticated remote evidence.
type finalProvenanceOracleFixture struct {
	f *finalProvenanceFixture
	p finalProvenancePublication
	o map[string]any
}

func finalProvenanceOracleFixtureNew(t *testing.T) *finalProvenanceOracleFixture {
	t.Helper()
	f := &finalProvenanceFixture{originals: t.TempDir(), expected: finalProvenanceExpected{Sources: map[string]finalProvenanceSource{}, Files: map[string]finalProvenanceFile{}, Publications: map[string]finalProvenancePublication{}}, manifest: finalProvenanceManifest{Files: map[string]string{}}}
	s := finalProvenanceSource{Repository: "davidpovarsky/cherri", SHA: strings.Repeat("a", 40), Tree: strings.Repeat("b", 40), LocalRef: "refs/heads/agent/synthetic-source", PublishedRef: "refs/heads/agent/synthetic-publication"}
	f.expected.Sources["compiler"] = s
	p := finalProvenancePublication{Format: "oracle-atomic-create-only/1", Result: "oracle-result", PublisherRef: s.LocalRef, Streams: map[string]finalProvenanceStreams{}, Receipts: map[string]string{}}
	source, origin := "/mnt/development/projects/worktrees/synthetic-source", "https://github.com/davidpovarsky/cherri.git"
	names := []string{"01-origin", "02-push-origin", "03-head-before", "04-tree-before", "05-branch-before", "06-status-before", "07-local-refs-before", "08-remote-refs-before", "09-destination-before", "10-push", "11-destination-after", "12-api-ref", "13-api-commit", "14-remote-refs-after", "15-local-refs-after", "16-head-after", "17-tree-after", "18-branch-after", "19-status-after", "20-origin-after", "21-push-origin-after"}
	// Independently authored command inventory, not generated by the validator.
	git := func(a ...string) []string {
		return append([]string{"/usr/bin/git", "-C", source, "-c", "push.followTags=false", "-c", "push.recurseSubmodules=no", "-c", "remote.origin.mirror=false"}, a...)
	}
	auth := func(a ...string) []string {
		return append(git("-c", "credential.helper=", "-c", "credential.helper=!gh auth git-credential"), a...)
	}
	commands := map[string][]string{
		"01-origin": git("remote", "get-url", "--all", "origin"), "02-push-origin": git("remote", "get-url", "--push", "--all", "origin"),
		"03-head-before": git("rev-parse", "HEAD"), "04-tree-before": git("rev-parse", "HEAD^{tree}"), "05-branch-before": git("branch", "--show-current"), "06-status-before": git("status", "--porcelain=v1", "--untracked-files=all"),
		"07-local-refs-before": git("for-each-ref", "--format=%(refname) %(objectname)"), "08-remote-refs-before": auth("ls-remote", "--refs", "origin"), "09-destination-before": auth("ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef),
		"10-push": auth("push", "--porcelain", "--force-with-lease="+s.PublishedRef+":", "origin", s.SHA+":"+s.PublishedRef), "11-destination-after": auth("ls-remote", "--exit-code", "--refs", "origin", s.PublishedRef),
		"12-api-ref":           {"/usr/local/bin/gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", "repos/davidpovarsky/cherri/git/ref/heads/agent/synthetic-publication", "--jq", "{ref: .ref, sha: .object.sha, type: .object.type}"},
		"13-api-commit":        {"/usr/local/bin/gh", "api", "--method", "GET", "-H", "Cache-Control: no-cache", "repos/davidpovarsky/cherri/git/commits/" + s.SHA, "--jq", "{sha: .sha, tree: .tree.sha, parents: [.parents[].sha]}"},
		"14-remote-refs-after": auth("ls-remote", "--refs", "origin"), "15-local-refs-after": git("for-each-ref", "--format=%(refname) %(objectname)"),
		"16-head-after": git("rev-parse", "HEAD"), "17-tree-after": git("rev-parse", "HEAD^{tree}"), "18-branch-after": git("branch", "--show-current"), "19-status-after": git("status", "--porcelain=v1", "--untracked-files=all"),
		"20-origin-after": git("remote", "get-url", "--all", "origin"), "21-push-origin-after": git("remote", "get-url", "--push", "--all", "origin"),
	}
	local := s.LocalRef + " " + s.SHA + "\n"
	remote := strings.Repeat("d", 40) + "\trefs/heads/main\n"
	ref := map[string]any{"ref": s.PublishedRef, "sha": s.SHA, "type": "commit"}
	commit := map[string]any{"sha": s.SHA, "tree": s.Tree, "parents": []string{strings.Repeat("d", 40)}}
	refBytes, _ := json.Marshal(ref)
	commitBytes, _ := json.Marshal(commit)
	outputs := map[string]string{
		"01-origin": origin + "\n", "02-push-origin": origin + "\n", "03-head-before": s.SHA + "\n", "04-tree-before": s.Tree + "\n", "05-branch-before": "agent/synthetic-source\n", "07-local-refs-before": local,
		"08-remote-refs-before": remote, "10-push": "To " + origin + "\n*\t" + s.SHA + ":" + s.PublishedRef + "\t[new branch]\nDone\n", "11-destination-after": s.SHA + "\t" + s.PublishedRef + "\n", "12-api-ref": string(refBytes) + "\n", "13-api-commit": string(commitBytes) + "\n",
		"14-remote-refs-after": remote + s.SHA + "\t" + s.PublishedRef + "\n", "15-local-refs-after": local + "refs/remotes/origin/agent/synthetic-publication " + s.SHA + "\n", "16-head-after": s.SHA + "\n", "17-tree-after": s.Tree + "\n", "18-branch-after": "agent/synthetic-source\n", "20-origin-after": origin + "\n", "21-push-origin-after": origin + "\n",
	}
	var processes []any
	start := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, name := range names {
		pair := finalProvenanceStreams{"oracle-" + name + ".stdout", "oracle-" + name + ".stderr"}
		f.putBytes(t, pair.Stdout, []byte(outputs[name]))
		f.putBytes(t, pair.Stderr, []byte{})
		p.Streams[name], p.Receipts[name] = pair, "oracle-"+name+".process"
		limit, code := 30, 0
		if name == "10-push" {
			limit = 120
		}
		if name == "09-destination-before" {
			code = 2
		}
		pins := map[string]any{}
		for field, id := range map[string]string{"stdout": pair.Stdout, "stderr": pair.Stderr} {
			pin := f.expected.Files[id]
			pins[field] = map[string]any{"path": f.manifest.Files[id], "bytes": pin.Bytes, "sha256": pin.SHA256}
		}
		processes = append(processes, map[string]any{"label": name, "argv": commands[name], "cwd": source, "startedAt": start.Add(time.Duration(i*2) * time.Second).Format(time.RFC3339), "finishedAt": start.Add(time.Duration(i*2+1) * time.Second).Format(time.RFC3339), "seconds": 1, "limitSeconds": limit, "pid": 100 + i, "exitCode": code, "timedOut": false, "overflow": false, "signals": []any{}, "cleanup": map[string]any{"groupGone": true, "signals": []any{}}, "stdout": pins["stdout"], "stderr": pins["stderr"]})
	}
	state := map[string]any{"head": s.SHA, "tree": s.Tree, "branch": "agent/synthetic-source", "status": "", "origin": origin, "pushOrigin": origin}
	o := map[string]any{"schema": "cherri-oracle-atomic-create-only-publication/1", "status": "PUBLISHED_REMOTE_VERIFIED", "sourceDirectory": source, "sourceCommit": s.SHA, "sourceTree": s.Tree, "localRef": s.LocalRef, "ref": s.PublishedRef, "startedAt": start.Format(time.RFC3339), "finishedAt": start.Add(time.Minute + 125*time.Millisecond).Format(time.RFC3339Nano), "elapsedSeconds": 60.125, "signals": []any{}, "bounds": map[string]any{"readSeconds": 30, "pushSeconds": 120, "ownerAlarmSeconds": 690, "ownerBudgetSeconds": 700, "hostSeconds": 720, "termCleanupSeconds": 5, "killCleanupSeconds": 5}, "destinationMustBeAbsent": true, "pushAttempted": true, "pushTimedOut": false, "sourceUnchanged": true, "sourceInputsUnchanged": true, "credentialPersisted": false, "privatePayloadExported": false, "liveRemoteBuilderTouched": false, "pushAttemptLimit": 1, "pushAttempts": 1, "pushExitCode": 0, "retries": 0, "actionsDispatches": 0, "credentialMode": "command-scoped-gh-helper", "sourceBefore": state, "sourceAfter": state, "remoteBinding": map[string]any{"ref": ref, "commit": commit, "verifiedAtUtc": "2020-01-01T00:00:26Z"}, "processes": processes}
	x := &finalProvenanceOracleFixture{f: f, p: p, o: o}
	x.save(t)
	x.o = f.object(t, p.Result)
	return x
}

func (x *finalProvenanceOracleFixture) save(t *testing.T) {
	t.Helper()
	for _, raw := range finalProvenanceArray(x.o, "processes") {
		step := raw.(map[string]any)
		x.f.put(t, x.p.Receipts[finalProvenanceString(step, "label")], step)
	}
	x.f.put(t, x.p.Result, x.o)
	x.f.expected.Publications["compiler"] = x.p
}
func (x *finalProvenanceOracleFixture) step(name string) map[string]any {
	for _, raw := range finalProvenanceArray(x.o, "processes") {
		if step := raw.(map[string]any); finalProvenanceString(step, "label") == name {
			return step
		}
	}
	return nil
}
func (x *finalProvenanceOracleFixture) stream(t *testing.T, name, field string, b []byte) {
	t.Helper()
	pair := x.p.Streams[name]
	id := pair.Stdout
	if field == "stderr" {
		id = pair.Stderr
	}
	x.f.putBytes(t, id, b)
	pin := x.f.expected.Files[id]
	x.step(name)[field] = map[string]any{"path": x.f.manifest.Files[id], "bytes": pin.Bytes, "sha256": pin.SHA256}
}

func TestFinalProvenanceOracleAtomicPublication(t *testing.T) {
	cases := []struct {
		name, want string
		mutate     func(*finalProvenanceOracleFixture)
		after      func(*finalProvenanceOracleFixture)
	}{
		{name: "one_actual_shape_with_atomic_absence_and_independent_readback"},
		{name: "publisher_branch_may_differ_from_canonical_checkout", mutate: func(x *finalProvenanceOracleFixture) {
			s := x.f.expected.Sources["compiler"]
			s.LocalRef = s.PublishedRef
			x.f.expected.Sources["compiler"] = s
		}},
		{name: "publisher_branch_snapshot_drift", want: "source snapshot changed", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.o, "sourceBefore")["branch"] = "agent/other-publisher-branch"
		}},
		{name: "outer_owner_signal", want: "outer process outcome", mutate: func(x *finalProvenanceOracleFixture) { x.o["signals"] = []string{"SIGTERM"} }},
		{name: "outer_idle_gap_exceeds_budget", want: "outer process outcome", mutate: func(x *finalProvenanceOracleFixture) {
			x.o["finishedAt"] = "2020-01-01T00:11:41.125Z"
			x.o["elapsedSeconds"] = 701.125
		}},
		{name: "source_commit_changed", want: "source, platform", mutate: func(x *finalProvenanceOracleFixture) { x.o["sourceCommit"] = strings.Repeat("e", 40) }},
		{name: "source_tree_changed", want: "source, platform", mutate: func(x *finalProvenanceOracleFixture) { x.o["sourceTree"] = strings.Repeat("e", 40) }},
		{name: "published_ref_changed", want: "source, platform", mutate: func(x *finalProvenanceOracleFixture) { x.o["ref"] = "refs/heads/main" }},
		{name: "failed_original_cannot_pass", want: "source, platform", mutate: func(x *finalProvenanceOracleFixture) { x.o["status"] = "PUSH_ATTEMPTED_REMOTE_OUTCOME_UNVERIFIED" }},
		{name: "windows_source_directory_cannot_be_relabeled", want: "source, platform", mutate: func(x *finalProvenanceOracleFixture) { x.o["sourceDirectory"] = "C:/synthetic" }},
		{name: "absence_requirement_removed", want: "scope flag", mutate: func(x *finalProvenanceOracleFixture) { x.o["destinationMustBeAbsent"] = false }},
		{name: "second_push_not_admitted", want: "attempt or operation", mutate: func(x *finalProvenanceOracleFixture) { x.o["pushAttempts"] = 2 }},
		{name: "retry_not_admitted", want: "attempt or operation", mutate: func(x *finalProvenanceOracleFixture) { x.o["retries"] = 1 }},
		{name: "credential_persistence_not_admitted", want: "scope flag", mutate: func(x *finalProvenanceOracleFixture) { x.o["credentialPersisted"] = true }},
		{name: "missing_process", want: "inventory is incomplete", mutate: func(x *finalProvenanceOracleFixture) { x.o["processes"] = finalProvenanceArray(x.o, "processes")[:20] }},
		{name: "missing_receipt", want: "inventory is incomplete", after: func(x *finalProvenanceOracleFixture) {
			p := x.f.expected.Publications["compiler"]
			delete(p.Receipts, "10-push")
			x.f.expected.Publications["compiler"] = p
		}},
		{name: "replayed_stream_identity", want: "absent or replayed", after: func(x *finalProvenanceOracleFixture) {
			p := x.f.expected.Publications["compiler"]
			p.Streams["02-push-origin"] = p.Streams["01-origin"]
			x.f.expected.Publications["compiler"] = p
		}},
		{name: "process_receipt_drift", want: "original receipt", after: func(x *finalProvenanceOracleFixture) {
			r := x.f.object(t, x.p.Receipts["10-push"])
			r["pid"] = 999
			x.f.put(t, x.p.Receipts["10-push"], r)
		}},
		{name: "stream_byte_drift", want: "independently pinned bytes", after: func(x *finalProvenanceOracleFixture) {
			x.f.putBytes(t, x.p.Streams["10-push"].Stdout, []byte("changed\n"))
		}},
		{name: "failed_push_process", want: "process result", mutate: func(x *finalProvenanceOracleFixture) { x.step("10-push")["exitCode"] = 1 }},
		{name: "timed_out_process", want: "process result", mutate: func(x *finalProvenanceOracleFixture) { x.step("12-api-ref")["timedOut"] = true }},
		{name: "surviving_owned_group", want: "process result", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.step("10-push"), "cleanup")["groupGone"] = false
		}},
		{name: "cleanup_signal_on_claimed_success", want: "process result", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.step("10-push"), "cleanup")["signals"] = []string{"SIGTERM"}
		}},
		{name: "read_deadline_widened", want: "original deadline", mutate: func(x *finalProvenanceOracleFixture) { x.step("12-api-ref")["limitSeconds"] = 31 }},
		{name: "push_deadline_widened", want: "original deadline", mutate: func(x *finalProvenanceOracleFixture) { x.step("10-push")["limitSeconds"] = 121 }},
		{name: "overlapping_process_chronology", want: "process chronology", mutate: func(x *finalProvenanceOracleFixture) { x.step("10-push")["startedAt"] = "2020-01-01T00:00:00Z" }},
		{name: "empty_lease_removed", want: "exact command", mutate: func(x *finalProvenanceOracleFixture) {
			a, _ := finalProvenanceStrings(x.step("10-push")["argv"])
			for i, v := range a {
				if strings.HasPrefix(v, "--force-with-lease=") {
					a = append(a[:i], a[i+1:]...)
					break
				}
			}
			x.step("10-push")["argv"] = a
		}},
		{name: "nonempty_lease_not_create_only", want: "exact command", mutate: func(x *finalProvenanceOracleFixture) {
			a, _ := finalProvenanceStrings(x.step("10-push")["argv"])
			for i, v := range a {
				if strings.HasPrefix(v, "--force-with-lease=") {
					a[i] += strings.Repeat("d", 40)
				}
			}
			x.step("10-push")["argv"] = a
		}},
		{name: "extra_refspec", want: "exact command", mutate: func(x *finalProvenanceOracleFixture) {
			a, _ := finalProvenanceStrings(x.step("10-push")["argv"])
			x.step("10-push")["argv"] = append(a, "HEAD:refs/heads/extra")
		}},
		{name: "different_credential_helper", want: "exact command", mutate: func(x *finalProvenanceOracleFixture) {
			a, _ := finalProvenanceStrings(x.step("10-push")["argv"])
			for i, v := range a {
				if v == "credential.helper=!gh auth git-credential" {
					a[i] = "credential.helper=store"
				}
			}
			x.step("10-push")["argv"] = a
		}},
		{name: "push_destination_changed", want: "snapshot does not join", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "02-push-origin", "stdout", []byte("https://github.com/other/repository.git\n"))
		}},
		{name: "source_dirty_after", want: "source snapshot changed", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.o, "sourceAfter")["status"] = " M tracked.go"
		}},
		{name: "destination_already_present", want: "destination absence", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "09-destination-before", "stdout", []byte(strings.Repeat("a", 40)+"\trefs/heads/agent/synthetic-publication\n"))
		}},
		{name: "full_inventory_already_contains_destination", want: "destination was present", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "08-remote-refs-before", "stdout", []byte(strings.Repeat("d", 40)+"\trefs/heads/main\n"+strings.Repeat("a", 40)+"\trefs/heads/agent/synthetic-publication\n"))
		}},
		{name: "remote_ref_changed_outside_destination", want: "full remote inventory", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "14-remote-refs-after", "stdout", []byte(strings.Repeat("e", 40)+"\trefs/heads/main\n"+strings.Repeat("a", 40)+"\trefs/heads/agent/synthetic-publication\n"))
		}},
		{name: "duplicate_full_remote_ref", want: "duplicate original ref", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "08-remote-refs-before", "stdout", []byte(strings.Repeat("d", 40)+"\trefs/heads/main\n"+strings.Repeat("d", 40)+"\trefs/heads/main\n"))
		}},
		{name: "unrelated_local_ref_created", want: "unrelated local refs", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "15-local-refs-after", "stdout", []byte("refs/heads/agent/synthetic-source "+strings.Repeat("a", 40)+"\nrefs/heads/extra "+strings.Repeat("e", 40)+"\n"))
		}},
		{name: "wrong_tracking_ref_target", want: "unrelated local refs", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "15-local-refs-after", "stdout", []byte("refs/heads/agent/synthetic-source "+strings.Repeat("a", 40)+"\nrefs/remotes/origin/agent/synthetic-publication "+strings.Repeat("e", 40)+"\n"))
		}},
		{name: "push_reports_existing_branch", want: "selected new branch", mutate: func(x *finalProvenanceOracleFixture) {
			x.stream(t, "10-push", "stdout", []byte("Everything up-to-date\n"))
		}},
		{name: "different_api_endpoint", want: "exact command", mutate: func(x *finalProvenanceOracleFixture) {
			a, _ := finalProvenanceStrings(x.step("12-api-ref")["argv"])
			a[6] = "repos/davidpovarsky/cherri/git/ref/heads/main"
			x.step("12-api-ref")["argv"] = a
		}},
		{name: "api_tree_differs", want: "independent API", mutate: func(x *finalProvenanceOracleFixture) {
			b, _ := json.Marshal(map[string]any{"sha": strings.Repeat("a", 40), "tree": strings.Repeat("e", 40), "parents": []string{strings.Repeat("d", 40)}})
			x.stream(t, "13-api-commit", "stdout", b)
		}},
		{name: "remote_binding_changed", want: "remote binding", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(finalProvenanceObject(x.o, "remoteBinding"), "ref")["sha"] = strings.Repeat("e", 40)
		}},
		{name: "verification_time_missing", want: "verification time", mutate: func(x *finalProvenanceOracleFixture) {
			delete(finalProvenanceObject(x.o, "remoteBinding"), "verifiedAtUtc")
		}},
		{name: "verification_precedes_api_commit", want: "verification time", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.o, "remoteBinding")["verifiedAtUtc"] = "2020-01-01T00:00:24Z"
		}},
		{name: "verification_after_operation", want: "verification time", mutate: func(x *finalProvenanceOracleFixture) {
			finalProvenanceObject(x.o, "remoteBinding")["verifiedAtUtc"] = "2020-01-01T00:01:01Z"
		}},
		{name: "oracle_receipt_not_windows_publication", want: "Windows publication", after: func(x *finalProvenanceOracleFixture) {
			p := x.f.expected.Publications["compiler"]
			p.Format = "windows-create-only/1"
			x.f.expected.Publications["compiler"] = p
		}},
		{name: "oracle_receipt_not_legacy_publication", want: "source publication result", after: func(x *finalProvenanceOracleFixture) {
			p := x.f.expected.Publications["compiler"]
			p.Format = "source-create-only/1"
			x.f.expected.Publications["compiler"] = p
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			x := finalProvenanceOracleFixtureNew(t)
			if tc.mutate != nil {
				tc.mutate(x)
			}
			x.save(t)
			if tc.after != nil {
				tc.after(x)
			}
			in := (&finalProvenanceReconciledFixture{f: x.f}).inputs(t)
			err := finalProvenanceValidatePublication(in, "compiler")
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("want rejection %q; got %v", tc.want, err)
			}
		})
	}
}

func TestFinalProvenancePublicationFormatDispatch(t *testing.T) {
	f := finalProvenanceFixtureNew(t)
	in := (&finalProvenanceReconciledFixture{f: f}).inputs(t)
	for _, tc := range []struct{ name, role string }{{"existing_windows_strict", "implementation"}, {"existing_nonforce_source", "compiler"}, {"existing_docs_main", "docs"}} {
		t.Run(tc.name, func(t *testing.T) {
			if err := finalProvenanceValidatePublication(in, tc.role); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Run("existing_reconciled_windows", func(t *testing.T) {
		x := finalProvenanceReconciledFixtureNew(t)
		if err := finalProvenanceValidatePublication(x.inputs(t), "compiler"); err != nil {
			t.Fatal(err)
		}
	})
}
