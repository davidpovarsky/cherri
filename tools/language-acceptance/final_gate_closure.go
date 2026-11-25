package main

// RP49/BRG31 have their own closure contract. CI03 deliberately records real
// conclusions (including failure); this layer additionally requires the exact
// positive executions chosen in an independently pinned, pre-dispatch plan.
// Related requirement names never confer authorization on an evidence record.

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"time"
)

const finalGatePlanSchema = "cherri-final-required-gate-plan/1"

type finalGateArtifactIntent struct {
	Key  string `json:"key"`
	Name string `json:"name"`
}

type finalGateIntent struct {
	Key                         string                    `json:"key"`
	Role                        string                    `json:"role"`
	WorkflowPath                string                    `json:"workflowPath"`
	WorkflowSHA256              string                    `json:"workflowSha256,omitempty"`
	Event                       string                    `json:"event"`
	Ref                         string                    `json:"ref"`
	Attempt                     int                       `json:"attempt"`
	RequiredJobs                []string                  `json:"requiredJobs"`
	RequiredArtifacts           []finalGateArtifactIntent `json:"requiredArtifacts"`
	AdditionalNativeRequirement string                    `json:"additionalNativeRequirement,omitempty"`
}

type finalGatePlan struct {
	Schema             string            `json:"schema"`
	PreparedUTC        string            `json:"preparedUtc"`
	Sources            map[string]string `json:"sources"`
	Required           []finalGateIntent `json:"required"`
	SelectAfterOutcome bool              `json:"selectAfterOutcome"`
	ClosureComplete    bool              `json:"closureComplete"`
}

type finalGateSelection struct {
	Run          string                  `json:"run"`
	JobsFile     string                  `json:"jobsFile"`
	RunInventory string                  `json:"runInventory"`
	Checkouts    []ciCheckoutObservation `json:"checkouts"`
	Artifacts    map[string]string       `json:"artifacts"`
}

type finalGateClosure struct {
	PlanSHA256         string                        `json:"planSha256"`
	SelectedRuns       map[string]finalProvenanceRun `json:"selectedRuns"`
	ObservedAttempts   map[string]finalProvenanceRun `json:"observedAttempts"`
	Jobs               map[string]string             `json:"jobs"`
	Artifacts          map[string]string             `json:"artifacts"`
	PostMainBuild      string                        `json:"postMainBuild"`
	InputsCompletedUTC string                        `json:"inputsCompletedUtc"`
}

func finalGateSourceRole(role string) string {
	if role == "docs" {
		return "docs"
	}
	return "implementation"
}

func finalGateRequiredJobs(role string) []string {
	if role == "docs" {
		return []string{"build", "deploy"}
	}
	return ciIdentityRequiredJobs[role]
}

func finalGateSameSet(actual, required []string) bool {
	if len(actual) != len(required) {
		return false
	}
	a, b := append([]string{}, actual...), append([]string{}, required...)
	sort.Strings(a)
	sort.Strings(b)
	for i := range a {
		if a[i] == "" || a[i] != b[i] || (i > 0 && a[i] == a[i-1]) {
			return false
		}
	}
	return true
}

func finalGateReadPlan(in *finalProvenanceInputs, file string) (finalGatePlan, error) {
	var plan finalGatePlan
	raw, err := ciIdentityReadOriginal(in, file, 1024*1024)
	if err != nil {
		return plan, err
	}
	object, objectErr := decodeNativeEncodingObject(raw)
	if objectErr != nil || !finalProvenanceBool(object, "selectAfterOutcome", false) || !finalProvenanceBool(object, "closureComplete", false) {
		return plan, fmt.Errorf("FINAL_GATE_PLAN_EXPLICIT_BOUNDARY")
	}
	if err = decodeEditorJSON(raw, &plan); err != nil {
		return plan, fmt.Errorf("FINAL_GATE_PLAN_JSON")
	}
	if plan.Schema != finalGatePlanSchema || plan.SelectAfterOutcome || plan.ClosureComplete || len(plan.Sources) != 3 || len(plan.Required) < 5 || len(plan.Required) > 16 {
		return plan, fmt.Errorf("FINAL_GATE_PLAN_SCHEMA")
	}
	if _, err := finalProvenanceTime(plan.PreparedUTC); err != nil {
		return plan, fmt.Errorf("FINAL_GATE_PLAN_TIME")
	}
	for _, role := range []string{"implementation", "compiler", "docs"} {
		if plan.Sources[role] != in.Expected.Sources[role].SHA || !finalProvenanceHex(plan.Sources[role], 40) {
			return plan, fmt.Errorf("FINAL_GATE_PLAN_SOURCE")
		}
	}
	roles, keys, nativeRequirements := map[string]bool{}, map[string]bool{}, map[string]bool{}
	hasPostMain := false
	for _, gate := range plan.Required {
		if gate.Key == "" || len(gate.Key) > 80 || keys[gate.Key] || gate.Attempt <= 0 || gate.Attempt > 100 ||
			(gate.Event != "push" && gate.Event != "workflow_dispatch") || !strings.HasPrefix(gate.Ref, "refs/heads/") || strings.TrimPrefix(gate.Ref, "refs/heads/") == "" {
			return plan, fmt.Errorf("FINAL_GATE_PLAN_INTENT")
		}
		keys[gate.Key] = true
		if gate.Role == "additional-native" {
			if (gate.AdditionalNativeRequirement != "CI02" && gate.AdditionalNativeRequirement != "RP48") || nativeRequirements[gate.AdditionalNativeRequirement] ||
				path.Dir(gate.WorkflowPath) != ".github/workflows" || (path.Ext(gate.WorkflowPath) != ".yml" && path.Ext(gate.WorkflowPath) != ".yaml") || !finalProvenanceHex(gate.WorkflowSHA256, 64) || len(gate.RequiredJobs) == 0 || len(gate.RequiredJobs) > 16 || !finalGateSameSet(gate.RequiredJobs, gate.RequiredJobs) {
				return plan, fmt.Errorf("FINAL_GATE_ADDITIONAL_NATIVE_INTENT")
			}
			nativeRequirements[gate.AdditionalNativeRequirement] = true
		} else {
			workflow, known := ciIdentityRequiredWorkflows[gate.Role]
			if !known || gate.AdditionalNativeRequirement != "" || workflow != gate.WorkflowPath || !finalGateSameSet(gate.RequiredJobs, finalGateRequiredJobs(gate.Role)) {
				return plan, fmt.Errorf("FINAL_GATE_REQUIRED_ROLE_INVENTORY")
			}
			roles[gate.Role] = true
		}
		if gate.Role == "build" && gate.Event == "push" && gate.Ref == "refs/heads/main" {
			hasPostMain = true
		}
		if len(gate.RequiredArtifacts) == 0 || len(gate.RequiredArtifacts) > 32 {
			return plan, fmt.Errorf("FINAL_GATE_REQUIRED_ARTIFACT_INVENTORY")
		}
		seenKeys, seenNames := map[string]bool{}, map[string]bool{}
		for _, artifact := range gate.RequiredArtifacts {
			name := strings.ReplaceAll(strings.ReplaceAll(artifact.Name, "{run_id}", "1"), "{attempt}", "1")
			if artifact.Key == "" || artifact.Name == "" || seenKeys[artifact.Key] || seenNames[artifact.Name] || strings.ContainsAny(name, "{}\\/\x00\r\n") {
				return plan, fmt.Errorf("FINAL_GATE_REQUIRED_ARTIFACT_INVENTORY")
			}
			seenKeys[artifact.Key], seenNames[artifact.Name] = true, true
		}
	}
	for role := range ciIdentityRequiredWorkflows {
		if !roles[role] {
			return plan, finalProvenancePending("FINAL_GATE_FIVE_ROLES_AND_POST_MAIN_BUILD_REQUIRED")
		}
	}
	if !hasPostMain {
		return plan, finalProvenancePending("FINAL_GATE_FIVE_ROLES_AND_POST_MAIN_BUILD_REQUIRED")
	}
	return plan, nil
}

func finalGateAttemptIdentity(run finalProvenanceRun) string {
	return run.Repository + ":" + strconv.FormatInt(run.ID, 10) + ":" + strconv.Itoa(run.Attempt)
}

// All rows in the independently pinned current-source/ref inventory are kept,
// including unsuccessful runs. Every attempt 1..latest must have its own actual
// API original; a later green attempt cannot erase an earlier failure.
func finalGateValidateAttemptInventory(in *finalProvenanceInputs, gate finalGateIntent, selection finalGateSelection, seen map[string]finalProvenanceRun) error {
	inventory, err := in.object(selection.RunInventory)
	if err != nil {
		return err
	}
	rows, ok := inventory["workflow_runs"].([]any)
	count, countOK := finalProvenanceNumber(inventory, "total_count")
	if !ok || !countOK || count != int64(len(rows)) || count <= 0 || count > 100 {
		return fmt.Errorf("FINAL_GATE_RUN_INVENTORY_INCOMPLETE")
	}
	selected := in.Expected.Runs[selection.Run]
	byIdentity := map[string]string{}
	for key, run := range in.Expected.Runs {
		identity := finalGateAttemptIdentity(run)
		if byIdentity[identity] != "" {
			return fmt.Errorf("FINAL_GATE_DUPLICATE_ATTEMPT")
		}
		byIdentity[identity] = key
	}
	seenRuns, selectedSeen := map[int64]bool{}, false
	for _, value := range rows {
		row, good := value.(map[string]any)
		if !good {
			return fmt.Errorf("FINAL_GATE_RUN_INVENTORY_SCHEMA")
		}
		id, idOK := finalProvenanceNumber(row, "id")
		attempt, attemptOK := finalProvenanceNumber(row, "run_attempt")
		if !idOK || id <= 0 || seenRuns[id] || !attemptOK || attempt <= 0 || attempt > 100 ||
			finalProvenanceString(row, "head_sha") != selected.SourceSHA || finalProvenanceString(row, "head_branch") != strings.TrimPrefix(gate.Ref, "refs/heads/") ||
			finalProvenanceString(row, "path") != gate.WorkflowPath || finalProvenanceString(finalProvenanceObject(row, "repository"), "full_name") != selected.Repository {
			return fmt.Errorf("FINAL_GATE_RUN_INVENTORY_IDENTITY")
		}
		seenRuns[id] = true
		for current := 1; current <= int(attempt); current++ {
			identity := selected.Repository + ":" + strconv.FormatInt(id, 10) + ":" + strconv.Itoa(current)
			key := byIdentity[identity]
			if key == "" {
				return finalProvenancePending("FINAL_GATE_PRIOR_ATTEMPT_ORIGINAL_MISSING")
			}
			run := in.Expected.Runs[key]
			original, err := in.object(run.Original)
			if err != nil {
				return err
			}
			if run.SourceSHA != selected.SourceSHA || run.WorkflowPath != gate.WorkflowPath || run.Historical ||
				finalProvenanceString(original, "head_branch") != strings.TrimPrefix(gate.Ref, "refs/heads/") {
				return fmt.Errorf("FINAL_GATE_PRIOR_ATTEMPT_IDENTITY")
			}
			if current == int(attempt) {
				for _, field := range []string{"id", "run_attempt", "workflow_id", "head_sha", "head_branch", "path", "event", "status", "conclusion", "updated_at"} {
					if !reflect.DeepEqual(original[field], row[field]) {
						return fmt.Errorf("FINAL_GATE_RUN_LIST_ORIGINAL_CONTRADICTION")
					}
				}
			}
			seen[identity] = run
		}
		if id == selected.ID {
			if attempt != int64(gate.Attempt) {
				return fmt.Errorf("FINAL_GATE_SELECTED_ATTEMPT_SUPERSEDED")
			}
			selectedSeen = true
		}
	}
	if !selectedSeen {
		return finalProvenancePending("FINAL_GATE_SELECTED_RUN_ABSENT")
	}
	return nil
}

func finalGateArtifactName(template string, run finalProvenanceRun) string {
	name := strings.ReplaceAll(template, "{run_id}", strconv.FormatInt(run.ID, 10))
	return strings.ReplaceAll(name, "{attempt}", strconv.Itoa(run.Attempt))
}

func finalGateValidateSuccessfulJobs(in *finalProvenanceInputs, gate finalGateIntent, selection finalGateSelection) (map[string]string, []string, error) {
	run := in.Expected.Runs[selection.Run]
	runOriginal, err := in.object(run.Original)
	if err != nil {
		return nil, nil, err
	}
	runStart, runEnd, err := ciIdentityTimeRange(finalProvenanceString(runOriginal, "run_started_at"), finalProvenanceString(runOriginal, "updated_at"))
	if err != nil {
		return nil, nil, err
	}
	object, err := in.object(selection.JobsFile)
	if err != nil {
		return nil, nil, err
	}
	jobs, ok := object["jobs"].([]any)
	count, countOK := finalProvenanceNumber(object, "total_count")
	if !ok || !countOK || count != int64(len(jobs)) || count <= 0 || count > 100 {
		return nil, nil, fmt.Errorf("FINAL_GATE_JOB_INVENTORY_INCOMPLETE")
	}
	wanted, seen, ids, result := map[string]bool{}, map[string]bool{}, map[int64]bool{}, map[string]string{}
	for _, name := range gate.RequiredJobs {
		wanted[name] = true
	}
	var completed []string
	for _, value := range jobs {
		job, ok := value.(map[string]any)
		if !ok {
			return nil, nil, fmt.Errorf("FINAL_GATE_JOB_SCHEMA")
		}
		name := finalProvenanceString(job, "name")
		id, idOK := finalProvenanceNumber(job, "id")
		runID, runOK := finalProvenanceNumber(job, "run_id")
		attempt, attemptOK := finalProvenanceNumber(job, "run_attempt")
		if !idOK || id <= 0 || ids[id] || seen[name] || name == "" || !runOK || runID != run.ID || !attemptOK || attempt != int64(run.Attempt) || finalProvenanceString(job, "head_sha") != run.SourceSHA {
			return nil, nil, fmt.Errorf("FINAL_GATE_JOB_IDENTITY")
		}
		seen[name], ids[id] = true, true
		// The unselected documentation-component branch is not a required full-app
		// execution. No required or actually started job gets this exception.
		if !wanted[name] && gate.Role == "app" && name == "documentation-component" && finalProvenanceString(job, "status") == "completed" && finalProvenanceString(job, "conclusion") == "skipped" && len(finalProvenanceArray(job, "steps")) == 0 {
			continue
		}
		if !wanted[name] {
			return nil, nil, fmt.Errorf("FINAL_GATE_UNDECLARED_JOB")
		}
		if finalProvenanceString(job, "status") != "completed" {
			return nil, nil, finalProvenancePending("FINAL_GATE_REQUIRED_JOB_NOT_COMPLETED")
		}
		if finalProvenanceString(job, "conclusion") != "success" {
			return nil, nil, fmt.Errorf("FINAL_GATE_REQUIRED_JOB_NOT_SUCCESSFUL")
		}
		jobStart, jobEnd, timeErr := ciIdentityTimeRange(finalProvenanceString(job, "started_at"), finalProvenanceString(job, "completed_at"))
		if timeErr != nil || jobStart.Before(runStart) || jobEnd.After(runEnd) {
			return nil, nil, fmt.Errorf("FINAL_GATE_JOB_TIME")
		}
		completed = append(completed, finalProvenanceString(job, "completed_at"))
		result[gate.Key+"/"+name] = "success"
	}
	if len(result) != len(wanted) {
		return nil, nil, finalProvenancePending("FINAL_GATE_REQUIRED_JOB_MISSING")
	}
	checkoutNames := []string{}
	for _, witness := range selection.Checkouts {
		if witness.Run != selection.Run || witness.JobsFile != selection.JobsFile {
			return nil, nil, fmt.Errorf("FINAL_GATE_CHECKOUT_SELECTION")
		}
		job, step, err := ciIdentityJob(in, witness)
		if err != nil {
			return nil, nil, err
		}
		jobStart, jobEnd, jobErr := ciIdentityTimeRange(finalProvenanceString(job, "started_at"), finalProvenanceString(job, "completed_at"))
		stepStart, stepEnd, stepErr := ciIdentityTimeRange(finalProvenanceString(step, "started_at"), finalProvenanceString(step, "completed_at"))
		if jobErr != nil || stepErr != nil || stepStart.Before(jobStart) || stepEnd.After(jobEnd) {
			return nil, nil, fmt.Errorf("FINAL_GATE_CHECKOUT_JOB_TIME")
		}
		log, err := ciIdentityReadOriginal(in, witness.LogFile, 64*1024*1024)
		if err != nil {
			return nil, nil, err
		}
		if _, err = ciIdentityCheckoutSHA(log, finalProvenanceString(step, "started_at"), finalProvenanceString(step, "completed_at"), run.SourceSHA); err != nil {
			return nil, nil, err
		}
		checkoutNames = append(checkoutNames, witness.JobName)
	}
	checkoutWant := gate.RequiredJobs
	if gate.Role == "docs" {
		checkoutWant = []string{"build"}
	}
	if !finalGateSameSet(checkoutNames, checkoutWant) {
		return nil, nil, fmt.Errorf("FINAL_GATE_COMPLETE_CHECKOUT_INVENTORY")
	}
	return result, completed, nil
}

func finalGateValidateClosure(ctx context.Context, in *finalProvenanceInputs, planFile string, selections map[string]finalGateSelection) (finalGateClosure, error) {
	out := finalGateClosure{PlanSHA256: in.Expected.Files[planFile].SHA256, SelectedRuns: map[string]finalProvenanceRun{}, ObservedAttempts: map[string]finalProvenanceRun{}, Jobs: map[string]string{}, Artifacts: map[string]string{}}
	plan, err := finalGateReadPlan(in, planFile)
	if err != nil {
		return out, err
	}
	if len(selections) != len(plan.Required) {
		return out, fmt.Errorf("FINAL_GATE_SELECTION_INVENTORY")
	}
	if err := finalProvenanceValidateRunsAndArtifacts(in); err != nil {
		return out, fmt.Errorf("FINAL_GATE_ORIGINAL_BINDINGS: %w", err)
	}
	prepared, _ := time.Parse(time.RFC3339Nano, plan.PreparedUTC)
	selectedIDs := map[string]bool{}
	var completions []string
	for _, gate := range plan.Required {
		if ctx.Err() != nil {
			return out, fmt.Errorf("FINAL_GATE_DEADLINE")
		}
		selection, exists := selections[gate.Key]
		run, runExists := in.Expected.Runs[selection.Run]
		source := in.Expected.Sources[finalGateSourceRole(gate.Role)]
		if !exists || !runExists || run.Historical || run.SourceSHA != source.SHA || run.Repository != source.Repository || run.WorkflowPath != gate.WorkflowPath || run.Event != gate.Event || run.Attempt != gate.Attempt || selectedIDs[finalGateAttemptIdentity(run)] {
			return out, fmt.Errorf("FINAL_GATE_SELECTED_INTENT_IDENTITY")
		}
		selectedIDs[finalGateAttemptIdentity(run)] = true
		original, err := in.object(run.Original)
		if err != nil {
			return out, err
		}
		if finalProvenanceString(original, "head_branch") != strings.TrimPrefix(gate.Ref, "refs/heads/") {
			return out, fmt.Errorf("FINAL_GATE_SELECTED_REF")
		}
		started, timeErr := finalProvenanceTime(finalProvenanceString(original, "run_started_at"))
		created, createdErr := finalProvenanceTime(finalProvenanceString(original, "created_at"))
		// created_at establishes new-run activation, unlike run_started_at which
		// can be much later after a queue. A later attempt cannot retrospectively
		// rewrite this plan; its initial run and all previous attempts are retained.
		if timeErr != nil || createdErr != nil || created.After(started) || !prepared.Before(created) || !prepared.Before(started) {
			return out, fmt.Errorf("FINAL_GATE_SELECTION_NOT_PREDECLARED")
		}
		if gate.Role == "additional-native" {
			workflow, _, readErr := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", source.SHA+":"+gate.WorkflowPath)
			if readErr != nil || ciIdentityDigest(workflow) != gate.WorkflowSHA256 {
				return out, fmt.Errorf("FINAL_GATE_ADDITIONAL_NATIVE_WORKFLOW_SOURCE")
			}
		}
		if run.Status != "completed" {
			return out, finalProvenancePending("FINAL_GATE_REQUIRED_RUN_NOT_COMPLETED")
		}
		if run.Conclusion != "success" {
			return out, fmt.Errorf("FINAL_GATE_REQUIRED_RUN_NOT_SUCCESSFUL")
		}
		if err = finalGateValidateAttemptInventory(in, gate, selection, out.ObservedAttempts); err != nil {
			return out, err
		}
		jobs, ends, err := finalGateValidateSuccessfulJobs(in, gate, selection)
		if err != nil {
			return out, err
		}
		for key, value := range jobs {
			out.Jobs[key] = value
		}
		completions = append(completions, ends...)
		if len(selection.Artifacts) != len(gate.RequiredArtifacts) {
			return out, fmt.Errorf("FINAL_GATE_SELECTED_ARTIFACT_INVENTORY")
		}
		seenArtifacts := map[string]bool{}
		for _, wanted := range gate.RequiredArtifacts {
			key, ok := selection.Artifacts[wanted.Key]
			artifact, artifactOK := in.Expected.Artifacts[key]
			if !ok || !artifactOK || seenArtifacts[key] || artifact.Run != selection.Run || artifact.Historical || artifact.Availability != "github" || artifact.SourceSHA != source.SHA || artifact.Role != finalGateSourceRole(gate.Role) || artifact.Name != finalGateArtifactName(wanted.Name, run) {
				return out, fmt.Errorf("FINAL_GATE_REQUIRED_ARTIFACT_MISSING_OR_DETACHED")
			}
			if err := ciIdentityArtifactAttempt(in, artifact); err != nil {
				return out, err
			}
			seenArtifacts[key] = true
			out.Artifacts[gate.Key+"/"+wanted.Key] = in.Expected.Files[artifact.File].SHA256
		}
		if gate.Role == "build" && gate.Event == "push" && gate.Ref == "refs/heads/main" {
			out.PostMainBuild = selection.Run
		}
		out.SelectedRuns[gate.Key] = run
	}
	// Retain every independently declared run, including historical diagnostic
	// runs and failed attempts. None can stand in for a selected final execution.
	for _, run := range in.Expected.Runs {
		out.ObservedAttempts[finalGateAttemptIdentity(run)] = run
	}
	out.InputsCompletedUTC, err = ciIdentityInputsCompleted(in, completions...)
	return out, err
}

type finalGateNegative struct {
	Fault      string `json:"fault"`
	Target     string `json:"target"`
	Rejected   bool   `json:"rejected"`
	Diagnostic string `json:"diagnostic"`
}

// These causal candidate injections run against the actual selected originals,
// without changing a single file or expected-context pin. They are intake
// negatives, not invented remote workflow attempts or Apple executions.
func finalGateNegativeControls(ctx context.Context, in *finalProvenanceInputs, planFile string, selections map[string]finalGateSelection) ([]finalGateNegative, error) {
	keys := make([]string, 0, len(selections))
	for key := range selections {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return nil, fmt.Errorf("FINAL_GATE_NEGATIVE_BASELINE")
	}
	var results []finalGateNegative
	for _, fault := range []string{"earlier_sha", "missing_artifact"} {
		clone := &finalProvenanceInputs{Expected: in.Expected, Manifest: in.Manifest, Bytes: in.Bytes, JSON: map[string]map[string]any{}}
		for key, value := range in.JSON {
			raw, err := json.Marshal(value)
			if err != nil {
				return nil, err
			}
			copy, err := decodeNativeEncodingObject(raw)
			if err != nil {
				return nil, err
			}
			clone.JSON[key] = copy
		}
		selection := selections[keys[0]]
		run := in.Expected.Runs[selection.Run]
		if fault == "earlier_sha" {
			row, err := clone.object(run.Original)
			if err != nil {
				return nil, err
			}
			changed := strings.Repeat("0", 40)
			if run.SourceSHA == changed {
				changed = strings.Repeat("1", 40)
			}
			row["head_sha"] = changed
		} else {
			artifactKeys := make([]string, 0, len(selection.Artifacts))
			for key := range selection.Artifacts {
				artifactKeys = append(artifactKeys, key)
			}
			sort.Strings(artifactKeys)
			if len(artifactKeys) == 0 {
				return nil, fmt.Errorf("FINAL_GATE_NEGATIVE_BASELINE")
			}
			artifact := in.Expected.Artifacts[selection.Artifacts[artifactKeys[0]]]
			row, err := clone.object(artifact.Metadata)
			if err != nil {
				return nil, err
			}
			if _, listed := row["artifacts"]; listed {
				row["artifacts"] = []any{}
				row["total_count"] = json.Number("0")
			} else {
				row["id"] = json.Number("0")
			}
		}
		_, err := finalGateValidateClosure(ctx, clone, planFile, selections)
		if err == nil || !strings.HasPrefix(err.Error(), "FINAL_GATE_ORIGINAL_BINDINGS:") {
			return nil, fmt.Errorf("FINAL_GATE_CAUSAL_NEGATIVE_DID_NOT_REJECT")
		}
		results = append(results, finalGateNegative{Fault: fault, Target: keys[0], Rejected: true, Diagnostic: "FINAL_GATE_ORIGINAL_BINDINGS"})
	}
	return results, nil
}
