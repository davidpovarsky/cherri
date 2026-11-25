package main

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

// These complete API/job/checkout fixtures are synthetic parser controls. They
// never invoke a workflow, publish a branch, sign a shortcut, or grant a tier.
type finalGateFixture struct {
	in         *finalProvenanceInputs
	root       string
	plan       finalGatePlan
	selections map[string]finalGateSelection
	identity   ciIdentityExpected
}

func newFinalGateFixture(t *testing.T) *finalGateFixture {
	t.Helper()
	in, root := ciTestInputs(t)
	f := &finalGateFixture{in: in, root: root, selections: map[string]finalGateSelection{}, identity: ciIdentityExpected{Checkouts: map[string][]ciCheckoutObservation{}}}
	for i, role := range []string{"implementation", "compiler", "docs"} {
		in.Expected.Sources[role] = finalProvenanceSource{Repository: "fixture/" + role, SHA: strings.Repeat(string(rune('a'+i)), 40), Tree: strings.Repeat(string(rune('d'+i)), 40), LocalRef: "refs/heads/fixture", PublishedRef: "refs/heads/fixture"}
	}
	f.plan = finalGatePlan{Schema: finalGatePlanSchema, PreparedUTC: "2020-01-01T00:00:00Z", Sources: map[string]string{}}
	for role, source := range in.Expected.Sources {
		f.plan.Sources[role] = source.SHA
	}
	for i, role := range []string{"build", "skill", "app", "runtime", "docs", "build"} {
		key := role
		event, ref := "workflow_dispatch", "refs/heads/fixture"
		if i == 5 {
			key = "post-main"
			event, ref = "push", "refs/heads/main"
		}
		gate := finalGateIntent{Key: key, Role: role, WorkflowPath: ciIdentityRequiredWorkflows[role], Event: event, Ref: ref, Attempt: 1, RequiredJobs: append([]string{}, finalGateRequiredJobs(role)...), RequiredArtifacts: []finalGateArtifactIntent{{Key: "original", Name: key + "-{run_id}-{attempt}"}}}
		f.plan.Required = append(f.plan.Required, gate)
		source := in.Expected.Sources[finalGateSourceRole(role)]
		run := finalProvenanceRun{Original: key + "-run", Repository: source.Repository, ID: int64(100 + i), Attempt: 1, WorkflowID: int64(200 + i), WorkflowPath: gate.WorkflowPath, Event: event, SourceSHA: source.SHA, Status: "completed", Conclusion: "success"}
		in.Expected.Runs[key] = run
		row := map[string]any{"id": run.ID, "run_attempt": 1, "workflow_id": run.WorkflowID, "head_sha": run.SourceSHA, "head_branch": strings.TrimPrefix(ref, "refs/heads/"), "path": run.WorkflowPath, "event": event, "status": "completed", "conclusion": "success", "repository": map[string]any{"full_name": run.Repository}, "html_url": fmt.Sprintf("https://github.com/%s/actions/runs/%d", run.Repository, run.ID), "created_at": "2020-01-01T00:00:05Z", "run_started_at": "2020-01-01T00:00:10Z", "updated_at": "2020-01-01T00:00:40Z"}
		ciTestObject(t, in, root, run.Original, row)
		selection := finalGateSelection{Run: key, JobsFile: key + "-jobs", RunInventory: key + "-run-inventory", Artifacts: map[string]string{"original": key + "-artifact"}}
		ciTestObject(t, in, root, selection.RunInventory, map[string]any{"total_count": 1, "workflow_runs": []any{row}})
		var jobs []any
		for j, name := range gate.RequiredJobs {
			jobID := int64(1000 + i*10 + j)
			step := map[string]any{"number": 2, "name": "Checkout", "status": "completed", "conclusion": "success", "started_at": "2020-01-01T00:00:12Z", "completed_at": "2020-01-01T00:00:15Z"}
			job := map[string]any{"id": jobID, "name": name, "run_id": run.ID, "run_attempt": 1, "head_sha": run.SourceSHA, "status": "completed", "conclusion": "success", "started_at": "2020-01-01T00:00:10Z", "completed_at": "2020-01-01T00:00:35Z", "steps": []any{step}}
			if role == "docs" && name == "deploy" {
				job["steps"] = []any{}
			} else {
				w := ciCheckoutObservation{Run: key, JobsFile: selection.JobsFile, JobID: jobID, JobName: name, LogFile: key + "-" + name + ".log", CheckoutStep: 2, CheckoutName: "Checkout"}
				selection.Checkouts = append(selection.Checkouts, w)
				ciTestPut(t, in, root, w.LogFile, []byte("2020-01-01T00:00:14Z [command]/usr/bin/git log -1 --format=%H\n2020-01-01T00:00:14.1Z "+run.SourceSHA+"\n"))
			}
			jobs = append(jobs, job)
		}
		ciTestObject(t, in, root, selection.JobsFile, map[string]any{"total_count": len(jobs), "jobs": jobs})
		f.selections[key] = selection
		if i != 5 {
			f.identity.Checkouts[role] = append([]ciCheckoutObservation{}, selection.Checkouts...)
		}
		file := key + "-archive"
		ciTestPut(t, in, root, file, []byte("synthetic parser artifact for "+key))
		artifactID := int64(3000 + i)
		artifactName := finalGateArtifactName(gate.RequiredArtifacts[0].Name, run)
		apiURL := fmt.Sprintf("https://api.github.com/repos/%s/actions/artifacts/%d", run.Repository, artifactID)
		meta := map[string]any{"id": artifactID, "name": artifactName, "expired": false, "size_in_bytes": in.Expected.Files[file].Bytes, "digest": "sha256:" + in.Expected.Files[file].SHA256, "workflow_run": map[string]any{"id": run.ID, "head_sha": run.SourceSHA}, "url": apiURL, "archive_download_url": apiURL + "/zip", "created_at": "2020-01-01T00:00:30Z"}
		ciTestObject(t, in, root, key+"-artifact-metadata", meta)
		in.Expected.Artifacts[key+"-artifact"] = finalProvenanceArtifact{File: file, Role: finalGateSourceRole(role), SourceSHA: run.SourceSHA, Availability: "github", Run: key, Metadata: key + "-artifact-metadata", ID: artifactID, Name: artifactName, URL: fmt.Sprintf("https://github.com/%s/actions/runs/%d/artifacts/%d", run.Repository, run.ID, artifactID)}
	}
	ciTestPut(t, in, root, "compiler-original", []byte("synthetic local compiler source artifact"))
	in.Expected.Artifacts["compiler-original"] = finalProvenanceArtifact{File: "compiler-original", Role: "compiler", SourceSHA: in.Expected.Sources["compiler"].SHA, Availability: "local", Name: "compiler-original"}
	// An earlier failed run is retained, not selected or relabeled as historical.
	prior := in.Expected.Runs["build"]
	prior.Original = "build-prior-run"
	prior.ID = 90
	prior.Conclusion = "failure"
	in.Expected.Runs["build-prior"] = prior
	row := f.object(t, "build-run")
	row["id"] = json.Number("90")
	row["conclusion"] = "failure"
	row["html_url"] = "https://github.com/fixture/implementation/actions/runs/90"
	ciTestObject(t, in, root, prior.Original, row)
	list := f.object(t, "build-run-inventory")
	list["total_count"] = json.Number("2")
	list["workflow_runs"] = append(list["workflow_runs"].([]any), row)
	ciTestObject(t, in, root, "build-run-inventory", list)
	f.savePlan(t)
	return f
}

func (f *finalGateFixture) object(t *testing.T, id string) map[string]any {
	t.Helper()
	data := f.in.Bytes[id]
	value, err := decodeNativeEncodingObject(data)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
func (f *finalGateFixture) change(t *testing.T, id string, fn func(map[string]any)) {
	t.Helper()
	o := f.object(t, id)
	fn(o)
	ciTestObject(t, f.in, f.root, id, o)
}
func (f *finalGateFixture) savePlan(t *testing.T) {
	t.Helper()
	ciTestObject(t, f.in, f.root, "plan", f.plan)
}
func (f *finalGateFixture) validate() (finalGateClosure, error) {
	return finalGateValidateClosure(context.Background(), f.in, "plan", f.selections)
}

func TestFinalRequiredGatesPredeclaredClosure(t *testing.T) {
	f := newFinalGateFixture(t)
	out, err := f.validate()
	if err != nil {
		t.Fatal(err)
	}
	if len(out.SelectedRuns) != 6 || len(out.ObservedAttempts) != 7 || len(out.Jobs) != 11 || len(out.Artifacts) != 6 || out.PostMainBuild != "post-main" || out.InputsCompletedUTC != "2020-01-01T00:00:40Z" {
		t.Fatalf("incomplete observed closure: %#v", out)
	}
	if out.ObservedAttempts["fixture/implementation:90:1"].Conclusion != "failure" {
		t.Fatal("prior failed run was erased")
	}
	if err = finalGateValidateIdentitySelection(f.in, f.identity, "plan", f.selections); err != nil {
		t.Fatal(err)
	}
	before, _ := json.Marshal(f.in.JSON)
	originalBytes, _ := json.Marshal(f.in.Bytes)
	negatives, err := finalGateNegativeControls(context.Background(), f.in, "plan", f.selections)
	if err != nil || len(negatives) != 2 || !negatives[0].Rejected || !negatives[1].Rejected {
		t.Fatalf("actual candidate negatives: %v %#v", err, negatives)
	}
	after, _ := json.Marshal(f.in.JSON)
	afterBytes, _ := json.Marshal(f.in.Bytes)
	if string(before) != string(after) || string(originalBytes) != string(afterBytes) {
		t.Fatal("causal intake mutated retained originals")
	}
}

func TestFinalRequiredGatesClosureCorruptions(t *testing.T) {
	cases := []struct {
		name       string
		mutate     func(*testing.T, *finalGateFixture)
		diagnostic string
	}{
		{"missing_post_main", func(t *testing.T, f *finalGateFixture) {
			f.plan.Required = f.plan.Required[:5]
			delete(f.selections, "post-main")
			f.savePlan(t)
		}, "FINAL_GATE_FIVE_ROLES"},
		{"missing_docs", func(t *testing.T, f *finalGateFixture) {
			f.plan.Required = append(f.plan.Required[:4], f.plan.Required[5:]...)
			delete(f.selections, "docs")
			f.savePlan(t)
		}, "FINAL_GATE_FIVE_ROLES"},
		{"plan_after_enqueue", func(t *testing.T, f *finalGateFixture) { f.plan.PreparedUTC = "2020-01-01T00:00:07Z"; f.savePlan(t) }, "FINAL_GATE_SELECTION_NOT_PREDECLARED"},
		{"plan_after_start", func(t *testing.T, f *finalGateFixture) { f.plan.PreparedUTC = "2020-01-01T00:00:20Z"; f.savePlan(t) }, "FINAL_GATE_SELECTION_NOT_PREDECLARED"},
		{"select_after_outcome", func(t *testing.T, f *finalGateFixture) { f.plan.SelectAfterOutcome = true; f.savePlan(t) }, "FINAL_GATE_PLAN_EXPLICIT_BOUNDARY"},
		{"missing_selection_flag", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "plan", func(o map[string]any) { delete(o, "selectAfterOutcome") })
		}, "FINAL_GATE_PLAN_EXPLICIT_BOUNDARY"},
		{"global_closure_claim", func(t *testing.T, f *finalGateFixture) { f.plan.ClosureComplete = true; f.savePlan(t) }, "FINAL_GATE_PLAN_EXPLICIT_BOUNDARY"},
		{"wrong_ref", func(t *testing.T, f *finalGateFixture) { f.plan.Required[1].Ref = "refs/heads/other"; f.savePlan(t) }, "FINAL_GATE_SELECTED_REF"},
		{"wrong_event", func(t *testing.T, f *finalGateFixture) { f.plan.Required[1].Event = "push"; f.savePlan(t) }, "FINAL_GATE_SELECTED_INTENT_IDENTITY"},
		{"wrong_attempt", func(t *testing.T, f *finalGateFixture) { f.plan.Required[1].Attempt = 2; f.savePlan(t) }, "FINAL_GATE_SELECTED_INTENT_IDENTITY"},
		{"wrong_source", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "skill-run", func(o map[string]any) { o["head_sha"] = strings.Repeat("0", 40) })
		}, "FINAL_GATE_ORIGINAL_BINDINGS"},
		{"missing_selected_run", func(t *testing.T, f *finalGateFixture) { delete(f.in.Expected.Runs, "skill") }, "FINAL_GATE_ORIGINAL_BINDINGS"},
		{"undeclared_selection", func(t *testing.T, f *finalGateFixture) { f.selections["extra"] = f.selections["skill"] }, "FINAL_GATE_SELECTION_INVENTORY"},
		{"unobserved_prior_failure", func(t *testing.T, f *finalGateFixture) { delete(f.in.Expected.Runs, "build-prior") }, "FINAL_GATE_PRIOR_ATTEMPT_ORIGINAL_MISSING"},
		{"truncated_run_list", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "build-run-inventory", func(o map[string]any) { o["total_count"] = json.Number("3") })
		}, "FINAL_GATE_RUN_INVENTORY_INCOMPLETE"},
		{"duplicate_run_row", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "skill-run-inventory", func(o map[string]any) {
				o["total_count"] = json.Number("2")
				o["workflow_runs"] = append(o["workflow_runs"].([]any), o["workflow_runs"].([]any)[0])
			})
		}, "FINAL_GATE_RUN_INVENTORY_IDENTITY"},
		{"contradicting_run_list", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "skill-run-inventory", func(o map[string]any) { o["workflow_runs"].([]any)[0].(map[string]any)["conclusion"] = "failure" })
		}, "FINAL_GATE_RUN_LIST_ORIGINAL_CONTRADICTION"},
		{"failed_required_job", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "build-jobs", func(o map[string]any) { o["jobs"].([]any)[0].(map[string]any)["conclusion"] = "failure" })
		}, "FINAL_GATE_REQUIRED_JOB_NOT_SUCCESSFUL"},
		{"cancelled_required_job", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "build-jobs", func(o map[string]any) { o["jobs"].([]any)[1].(map[string]any)["conclusion"] = "cancelled" })
		}, "FINAL_GATE_REQUIRED_JOB_NOT_SUCCESSFUL"},
		{"skipped_docs_deploy", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "docs-jobs", func(o map[string]any) { o["jobs"].([]any)[1].(map[string]any)["conclusion"] = "skipped" })
		}, "FINAL_GATE_REQUIRED_JOB_NOT_SUCCESSFUL"},
		{"pending_required_job", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "runtime-jobs", func(o map[string]any) { o["jobs"].([]any)[0].(map[string]any)["status"] = "in_progress" })
		}, "FINAL_GATE_REQUIRED_JOB_NOT_COMPLETED"},
		{"truncated_jobs", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "build-jobs", func(o map[string]any) { o["jobs"] = o["jobs"].([]any)[:2] })
		}, "FINAL_GATE_JOB_INVENTORY_INCOMPLETE"},
		{"missing_docs_deploy", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "docs-jobs", func(o map[string]any) { o["jobs"] = o["jobs"].([]any)[:1]; o["total_count"] = json.Number("1") })
		}, "FINAL_GATE_REQUIRED_JOB_MISSING"},
		{"job_after_run", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "app-jobs", func(o map[string]any) { o["jobs"].([]any)[0].(map[string]any)["completed_at"] = "2020-01-01T00:00:50Z" })
		}, "FINAL_GATE_JOB_TIME"},
		{"job_before_start", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "app-jobs", func(o map[string]any) { o["jobs"].([]any)[0].(map[string]any)["completed_at"] = "2020-01-01T00:00:09Z" })
		}, "FINAL_GATE_JOB_TIME"},
		{"checkout_outside_job", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "app-jobs", func(o map[string]any) {
				o["jobs"].([]any)[0].(map[string]any)["steps"].([]any)[0].(map[string]any)["started_at"] = "2020-01-01T00:00:09Z"
			})
		}, "FINAL_GATE_CHECKOUT_JOB_TIME"},
		{"wrong_checkout_sha", func(t *testing.T, f *finalGateFixture) {
			w := f.selections["app"].Checkouts[0]
			ciTestPut(t, f.in, f.root, w.LogFile, []byte(strings.ReplaceAll(string(f.in.Bytes[w.LogFile]), f.in.Expected.Sources["implementation"].SHA, strings.Repeat("0", 40))))
		}, "CI_"},
		{"missing_checkout_job", func(t *testing.T, f *finalGateFixture) {
			s := f.selections["build"]
			s.Checkouts = s.Checkouts[:2]
			f.selections["build"] = s
		}, "FINAL_GATE_COMPLETE_CHECKOUT_INVENTORY"},
		{"missing_artifact", func(t *testing.T, f *finalGateFixture) { delete(f.in.Expected.Artifacts, "app-artifact") }, "FINAL_GATE_REQUIRED_ARTIFACT_MISSING_OR_DETACHED"},
		{"wrong_artifact_attempt", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "runtime-artifact-metadata", func(o map[string]any) { o["created_at"] = "2020-01-01T00:00:00Z" })
		}, "CI_ARTIFACT_FROM_ANOTHER_ATTEMPT"},
		{"expired_artifact", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "runtime-artifact-metadata", func(o map[string]any) { o["expired"] = true })
		}, "FINAL_GATE_ORIGINAL_BINDINGS"},
		{"wrong_server_digest", func(t *testing.T, f *finalGateFixture) {
			f.change(t, "skill-artifact-metadata", func(o map[string]any) { o["digest"] = "sha256:" + strings.Repeat("0", 64) })
		}, "FINAL_GATE_ORIGINAL_BINDINGS"},
		{"unknown_artifact_placeholder", func(t *testing.T, f *finalGateFixture) {
			f.plan.Required[0].RequiredArtifacts[0].Name = "result-{anything}"
			f.savePlan(t)
		}, "FINAL_GATE_REQUIRED_ARTIFACT_INVENTORY"},
		{"duplicate_required_job", func(t *testing.T, f *finalGateFixture) {
			f.plan.Required[0].RequiredJobs = append(f.plan.Required[0].RequiredJobs, f.plan.Required[0].RequiredJobs[0])
			f.savePlan(t)
		}, "FINAL_GATE_REQUIRED_ROLE_INVENTORY"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := newFinalGateFixture(t)
			if _, err := f.validate(); err != nil {
				t.Fatalf("baseline: %v", err)
			}
			tc.mutate(t, f)
			_, err := f.validate()
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("want %s, got %v", tc.diagnostic, err)
			}
		})
	}
}

func TestFinalRequiredGatesContextAndContract(t *testing.T) {
	contracts, err := LoadAllContracts("", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err = finalGateValidateContracts(contracts); err != nil {
		t.Fatal(err)
	}
	t.Run("no_context_no_credit", func(t *testing.T) {
		for _, key := range []string{"CHERRI_FINAL_GATES_EXPECTED", "CHERRI_FINAL_GATES_EXPECTED_SHA256", "CHERRI_FINAL_GATES_MANIFEST"} {
			t.Setenv(key, "")
		}
		r := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
		r.runFinalRequiredGateRequirements(contracts)
		for _, id := range []string{"RP49", "BRG31"} {
			if r.results[id].Status != "NOT_RUN" || r.results[id].Passed {
				t.Fatalf("%s got credit", id)
			}
		}
		if len(r.records) != 0 {
			t.Fatal("absent context generated records")
		}
	})
	t.Run("malformed_context_fails", func(t *testing.T) {
		t.Setenv("CHERRI_FINAL_GATES_EXPECTED", "relative")
		t.Setenv("CHERRI_FINAL_GATES_EXPECTED_SHA256", strings.Repeat("a", 64))
		t.Setenv("CHERRI_FINAL_GATES_MANIFEST", "relative2")
		r := &Runner{implSHA: strings.Repeat("a", 40), results: map[string]CaseResult{}}
		r.runFinalRequiredGateRequirements(contracts)
		if r.results["RP49"].Status != "FAILED" || len(r.records) != 0 {
			t.Fatal("malformed context acquired credit")
		}
	})
	t.Run("immutable_contract_required", func(t *testing.T) {
		if finalGateValidateContracts(nil) == nil {
			t.Fatal("nil contracts accepted")
		}
		b, _ := json.Marshal(contracts)
		var changed ContractsSet
		if json.Unmarshal(b, &changed) != nil {
			t.Fatal("contract clone")
		}
		for i := range changed.Repair.Cases {
			if changed.Repair.Cases[i].ID == "RP49" {
				changed.Repair.Cases[i].ExpectedObservation = "generic green is sufficient"
			}
		}
		if finalGateValidateContracts(&changed) == nil {
			t.Fatal("weakened original clause accepted")
		}
	})
	t.Run("CI03_selection_exact", func(t *testing.T) {
		f := newFinalGateFixture(t)
		if finalGateValidateIdentitySelection(f.in, f.identity, "plan", f.selections) != nil {
			t.Fatal("baseline")
		}
		f.identity.Checkouts["build"] = f.selections["post-main"].Checkouts
		if finalGateValidateIdentitySelection(f.in, f.identity, "plan", f.selections) != nil {
			t.Fatal("exact post-main selection should be a valid chosen build")
		}
		f.identity.Checkouts["build"] = append([]ciCheckoutObservation{}, f.selections["build"].Checkouts...)
		f.identity.Checkouts["build"][0].LogFile = "unrelated"
		if finalGateValidateIdentitySelection(f.in, f.identity, "plan", f.selections) == nil {
			t.Fatal("CI03 detached from selected checkout")
		}
	})
	for _, id := range []string{"CI01", "CI02", "CI03", "CI04", "END01", "RP47", "RP48"} {
		t.Run("no_related_"+id, func(t *testing.T) {
			for _, owner := range []string{"RP49", "BRG31"} {
				if validateFinalRequiredGateRecord(id, EvidenceRecord{TestID: finalGateTestID(owner)}) == nil {
					t.Fatal("related ID inherited dedicated record")
				}
			}
		})
	}
	t.Run("no_generic_record", func(t *testing.T) {
		for _, id := range []string{"RP49", "BRG31"} {
			if validateFinalRequiredGateRecord(id, EvidenceRecord{Requirements: []string{id}, Tier: "ci", TestID: "github-actions:Build & Test", Result: "passed"}) == nil {
				t.Fatal("generic workflow claimed complete gate")
			}
		}
	})
}

func TestFinalRequiredGatesContextJoins(t *testing.T) {
	f := newFinalGateFixture(t)
	for _, field := range []string{"sources", "files", "publications", "runs", "artifacts"} {
		t.Run(field, func(t *testing.T) {
			b, _ := json.Marshal(f.in.Expected)
			var other finalProvenanceExpected
			if json.Unmarshal(b, &other) != nil {
				t.Fatal("clone")
			}
			if !finalGateSameBindings(f.in.Expected, other) {
				t.Fatal("baseline")
			}
			switch field {
			case "sources":
				s := other.Sources["compiler"]
				s.SHA = strings.Repeat("0", 40)
				other.Sources["compiler"] = s
			case "files":
				p := other.Files["plan"]
				p.SHA256 = strings.Repeat("0", 64)
				other.Files["plan"] = p
			case "publications":
				other.Publications = map[string]finalProvenancePublication{"implementation": {Format: "unrelated"}}
			case "runs":
				r := other.Runs["skill"]
				r.Attempt++
				other.Runs["skill"] = r
			case "artifacts":
				a := other.Artifacts["app-artifact"]
				a.File = "other"
				other.Artifacts["app-artifact"] = a
			}
			if finalGateSameBindings(f.in.Expected, other) {
				t.Fatal("inconsistent CI03/CI04 contexts accepted")
			}
		})
	}
	t.Run("signed_fixture_identity_complete", func(t *testing.T) {
		e := finalGateExpected{Native: &finalGateNativeExpected{RuntimeArtifact: "runtime"}}
		p := publicationSecurityExpected{}
		for id, sha := range finalGateRuntimeSources {
			p.SigningIntents = append(p.SigningIntents, publicationSigningIntent{Artifact: "runtime", Member: id + ".shortcut", FixtureID: id, FixtureSHA256: sha})
		}
		if err := finalGateSigningSelection(e, p); err != nil {
			t.Fatal(err)
		}
		p.SigningIntents = p.SigningIntents[:4]
		if finalGateSigningSelection(e, p) == nil {
			t.Fatal("partial signed native fixture inventory accepted")
		}
	})
	t.Run("comparison_cannot_remove_failure", func(t *testing.T) {
		a := finalGateObservation{Statuses: map[string]string{"RP49": "PASSED", "BRG31": "FAILED"}}
		b := a
		b.Statuses = map[string]string{"RP49": "PASSED", "BRG31": "PASSED"}
		if reflect.DeepEqual(a, b) {
			t.Fatal("fixture setup")
		}
		if finalGateCompareObservations(a, b) == nil {
			t.Fatal("contradicting original accepted")
		}
	})
}
