package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"
)

type finalGateNativeExpected struct {
	EncodingGate     string `json:"encodingGate"`
	EncodingArtifact string `json:"encodingArtifact"`
	EncodingStep     int64  `json:"encodingStep"`
	PreludeSHA256    string `json:"preludeSha256"`
	RuntimeGate      string `json:"runtimeGate"`
	RuntimeArtifact  string `json:"runtimeArtifact"`
	RuntimeStep      int64  `json:"runtimeStep"`
}

type finalGateNativeObservation struct {
	EncodingRun                 string            `json:"encodingRun"`
	EncodingAttempt             int               `json:"encodingAttempt"`
	EncodingSummarySHA256       string            `json:"encodingSummarySha256"`
	EncodingRequiredCases       int               `json:"encodingRequiredCases"`
	EncodingPositiveExit        int               `json:"encodingPositiveExit"`
	EncodingCorruptExit         int               `json:"encodingCorruptExit"`
	EncodingRestoredExit        int               `json:"encodingRestoredExit"`
	EncodingSourceFiles         map[string]string `json:"encodingSourceFiles"`
	RuntimeRun                  string            `json:"runtimeRun"`
	RuntimeAttempt              int               `json:"runtimeAttempt"`
	RuntimeOriginals            map[string]string `json:"runtimeOriginals"`
	ComputedFields              map[string]string `json:"computedFields"`
	RuntimeDiagnosticIDs        []string          `json:"runtimeDiagnosticIds"`
	RuntimeSourceFiles          map[string]string `json:"runtimeSourceFiles"`
	RuntimeNegativeControls     []string          `json:"runtimeNegativeControls"`
	FullRuntimeContractVerified bool              `json:"fullRuntimeContractVerified"`
	RuntimeCoverageBoundary     string            `json:"runtimeCoverageBoundary"`
	RelatedRequirementsGranted  []string          `json:"relatedRequirementsGranted"`
}

var finalGateRuntimeSources = map[string]string{
	"CherriS01F01CoveragePOC":    "4a13f90af2bdc326e24f9dbf785f76a1ae41043e176b5a3188ab794694a19aa3",
	"CherriRuntimePOC":           "cf5cb580a75833ffd022f4c4b4e8fa0e0e52f101c7f584d2726833000adf050b",
	"CherriDictionaryRuntimePOC": "fc7ce1f3daa3fffa33ab1b1af5e8c86c68fee1719f2f6121dbce6a6611be741c",
	"CherriMenuRuntimePOC":       "a021541d5d6a9b87a3c885f419d1e2f286ab3eb7b300e985372095c95ad646c0",
	"CherriLegacyIfPOC":          "4364d76ac8f4fdd1bbe049cf0f58d182f056772f79906f27564d66857b84cce7",
}

var finalGatePrimaryFields = [][2]string{
	{"VAR", "updated"}, {"CAPTURED_BEFORE", "initial"}, {"NUM_CAPTURED", "1"}, {"CLIP_CAPTURED", "clip_initial"},
	{"UNICODE", "שָׁלוֹם:🌍:1"}, {"IDX", "alpha:gamma:::::beta"}, {"MATH", "2.5:1:11:0:1:0"}, {"IF", "true"},
	{"SC", "START|RAN_RHS:0:1:1"}, {"LOOPS", "0:A:0:1|0:A:1:2|1:B:0:1|1:B:1:2|"}, {"VAL_BLOCKS", "then_val:<X><Y>"},
	{"FUNCTION", "21"}, {"FN_CONT", "22"}, {"DEFAULTS_EXPLICIT", "0:0::"}, {"DEFAULTS_OMITTED", "1:9:def:"},
	{"FACT", "6"}, {"VOID_OUT", ""}, {"ENV_SAFE", "multiply"}, {"BASE64", "Q0hFUlJJ"},
}

func finalGateFields(raw []byte, sentinel string, names []string, finalLF bool) (map[string]string, error) {
	if len(raw) == 0 || len(raw) > 1024*1024 || !utf8.Valid(raw) || bytes.IndexByte(raw, 0) >= 0 {
		return nil, fmt.Errorf("BRG31_RUNTIME_TEXT")
	}
	text := string(raw)
	if finalLF {
		if !strings.HasSuffix(text, "\n") {
			return nil, fmt.Errorf("BRG31_RUNTIME_OUTPUT_BOUNDARY")
		}
		text = strings.TrimSuffix(text, "\n")
	}
	lines := strings.Split(text, "\n")
	if len(lines) != len(names)+1 || lines[0] != sentinel {
		return nil, fmt.Errorf("BRG31_RUNTIME_FIELD_INVENTORY")
	}
	fields := map[string]string{}
	for i, name := range names {
		prefix := name + "="
		if !strings.HasPrefix(lines[i+1], prefix) {
			return nil, fmt.Errorf("BRG31_RUNTIME_FIELD_ORDER")
		}
		if _, exists := fields[name]; exists {
			return nil, fmt.Errorf("BRG31_RUNTIME_DUPLICATE_FIELD")
		}
		fields[name] = strings.TrimPrefix(lines[i+1], prefix)
	}
	return fields, nil
}

func finalGateNativeStep(in *finalProvenanceInputs, selection finalGateSelection, jobName string, number int64, name string) ([]byte, error) {
	if number <= 0 {
		return nil, fmt.Errorf("BRG31_NATIVE_STEP_SELECTION")
	}
	var selected *ciCheckoutObservation
	for i := range selection.Checkouts {
		if selection.Checkouts[i].JobName == jobName {
			if selected != nil {
				return nil, fmt.Errorf("BRG31_DUPLICATE_NATIVE_JOB")
			}
			copy := selection.Checkouts[i]
			selected = &copy
		}
	}
	if selected == nil {
		return nil, finalProvenancePending("BRG31_NATIVE_JOB_MISSING")
	}
	job, _, err := ciIdentityJob(in, *selected)
	if err != nil {
		return nil, err
	}
	var step map[string]any
	for _, raw := range finalProvenanceArray(job, "steps") {
		item, ok := raw.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("BRG31_NATIVE_STEP_SCHEMA")
		}
		n, ok := finalProvenanceNumber(item, "number")
		if ok && n == number {
			if step != nil {
				return nil, fmt.Errorf("BRG31_NATIVE_STEP_DUPLICATE")
			}
			step = item
		}
	}
	if step == nil {
		return nil, finalProvenancePending("BRG31_NATIVE_STEP_UNAVAILABLE")
	}
	if finalProvenanceString(step, "name") != name || finalProvenanceString(step, "status") != "completed" || finalProvenanceString(step, "conclusion") != "success" {
		return nil, fmt.Errorf("BRG31_NATIVE_STEP_NOT_SUCCESSFUL")
	}
	start, end, err := ciIdentityTimeRange(finalProvenanceString(step, "started_at"), finalProvenanceString(step, "completed_at"))
	if err != nil {
		return nil, err
	}
	log, err := ciIdentityReadOriginal(in, selected.LogFile, 64*1024*1024)
	if err != nil {
		return nil, err
	}
	var body []byte
	for _, line := range strings.Split(string(log), "\n") {
		at, text, ok := ciIdentityRunnerLine(line)
		if ok && !at.Before(start) && at.Before(end) {
			body = append(body, []byte(text+"\n")...)
		}
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("BRG31_NATIVE_STEP_ORIGINAL_LOG_MISSING")
	}
	return body, nil
}

func finalGateEncoding(ctx context.Context, in *finalProvenanceInputs, expected finalGateExpected, archives map[string]map[string][]byte, out *finalGateNativeObservation) error {
	native := expected.Native
	selection, ok := expected.Selections[native.EncodingGate]
	if !ok || !finalProvenanceHex(native.PreludeSHA256, 64) {
		return fmt.Errorf("BRG31_ENCODING_SELECTION")
	}
	run := in.Expected.Runs[selection.Run]
	artifact, ok := in.Expected.Artifacts[native.EncodingArtifact]
	if !ok || artifact.Run != selection.Run || run.WorkflowPath != ".github/workflows/build-test.yml" || run.SourceSHA != in.Expected.Sources["implementation"].SHA {
		return fmt.Errorf("BRG31_ENCODING_RUN_BINDING")
	}
	if !finalGateSelectionContainsArtifact(selection, native.EncodingArtifact) {
		return fmt.Errorf("BRG31_ENCODING_NOT_PREDECLARED")
	}
	log, err := finalGateNativeStep(in, selection, "build-test-macos", native.EncodingStep, "Mandatory WorkflowKit REQUIRED Encoding Gate")
	if err != nil {
		return err
	}
	if !bytes.Contains(log, []byte("required_encoding_gate.py")) {
		return fmt.Errorf("BRG31_ENCODING_COMMAND_ORIGINAL")
	}
	witnesses := map[string]string{}
	for _, path := range []string{"tools/shortcut-runtime/jxa/verify_encodings.js", "tools/shortcut-runtime/required-encoding-expectations.json", "tools/shortcut-runtime/required_encoding_gate.py", "docs/apple-runtime/parameter-encodings.json", "docs/apple-runtime/encoding-table.json"} {
		data, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", run.SourceSHA+":"+path)
		if err != nil {
			return err
		}
		witnesses[path] = ciIdentityDigest(data)
	}
	return finalGateEncodingOriginals(archives[native.EncodingArtifact], run.SourceSHA, strconv.FormatInt(run.ID, 10), run.Attempt, native.PreludeSHA256, witnesses, out)
}

// Called only after the selected final workflow, exact native job, original
// command, artifact identity and current Git source witnesses have been joined.
// A historical member set can exercise this parser without gaining authority.
func finalGateEncodingOriginals(members map[string][]byte, source, runID string, attempt int, prelude string, witnesses map[string]string, out *finalGateNativeObservation) error {
	if !finalProvenanceHex(source, 40) || runID == "" || attempt < 1 || !finalProvenanceHex(prelude, 64) || len(witnesses) != 5 {
		return fmt.Errorf("BRG31_ENCODING_SOURCE_INVENTORY")
	}
	summaryRaw := members["required-encoding-gate.json"]
	summary, err := decodeNativeEncodingObject(summaryRaw)
	if err != nil {
		return fmt.Errorf("BRG31_ENCODING_SUMMARY")
	}
	host := finalProvenanceObject(summary, "host")
	if finalProvenanceString(host, "system") != "Darwin" || finalProvenanceString(host, "macOSVersion") == "" || finalProvenanceString(summary, "preludeCommit") != "ee52c32937053268e4a035eba4b372fe77ec6411" {
		return fmt.Errorf("BRG31_ENCODING_NATIVE_PROVENANCE")
	}
	inputs := finalProvenanceObject(summary, "inputs")
	if len(inputs) != 6 {
		return fmt.Errorf("BRG31_ENCODING_SOURCE_INVENTORY")
	}
	seen := map[string]bool{}
	for rawPath, value := range inputs {
		digest, good := value.(string)
		if !good || !finalProvenanceHex(digest, 64) {
			return fmt.Errorf("BRG31_ENCODING_SOURCE_PIN")
		}
		matched := ""
		for path, want := range witnesses {
			if strings.HasSuffix(rawPath, "/"+path) && digest == want {
				matched = path
			}
		}
		if strings.HasSuffix(rawPath, "/tools/jxa-prelude.js") && digest == prelude {
			matched = "ShortcutKit@ee52c32937053268e4a035eba4b372fe77ec6411:tools/jxa-prelude.js"
		}
		if matched == "" || seen[matched] {
			return fmt.Errorf("BRG31_ENCODING_SOURCE_PIN")
		}
		seen[matched] = true
	}
	if len(seen) != 6 {
		return fmt.Errorf("BRG31_ENCODING_SOURCE_PIN")
	}
	var envelope struct {
		Records []EvidenceRecord `json:"records"`
	}
	if decodeEditorJSON(members["required-encoding-evidence.json"], &envelope) != nil || len(envelope.Records) != 2 {
		return fmt.Errorf("BRG31_ENCODING_RECORD_INVENTORY")
	}
	// Only exact raw copies go to an owned temporary directory to reuse the
	// existing strict per-form/process intake. No original locator or byte changes.
	temporary, err := os.MkdirTemp("", "cherri-brg31-encoding-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temporary)
	seenRecords := map[string]bool{}
	for _, record := range envelope.Records {
		if len(record.Requirements) != 1 {
			return fmt.Errorf("BRG31_ENCODING_PRODUCER_IDENTITY")
		}
		id := record.Requirements[0]
		if (id != "CI01" && id != "RP47") || seenRecords[id] || record.TestID != nativeEncodingTestID(id) || record.ImplementationSHA != source || record.Result != "passed" || record.ExitCode != 0 || len(record.Artifacts) != 13 {
			return fmt.Errorf("BRG31_ENCODING_PRODUCER_IDENTITY")
		}
		seenRecords[id] = true
		if !reflect.DeepEqual(record.Command, []string{"python3", "tools/shortcut-runtime/required_encoding_gate.py", "--source-sha", source, "--run-id", runID}) {
			return fmt.Errorf("BRG31_ENCODING_PRODUCER_COMMAND")
		}
		for i, item := range record.Artifacts {
			name := filepath.Base(item.Path)
			data, exists := members[name]
			if !exists || item.Path != "artifacts/backend-recovery/ci-artifacts/required-encoding-gate/"+name || item.RunID != runID || ciIdentityDigest(data) != item.SHA256 {
				return fmt.Errorf("BRG31_ENCODING_ARTIFACT_ORIGINAL")
			}
			path := filepath.Join(temporary, name)
			if prior, readErr := os.ReadFile(path); readErr == nil {
				if !bytes.Equal(prior, data) {
					return fmt.Errorf("BRG31_ENCODING_COPY_COLLISION")
				}
			} else {
				file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
				if err != nil {
					return err
				}
				_, writeErr := file.Write(data)
				closeErr := file.Close()
				if writeErr != nil {
					return writeErr
				}
				if closeErr != nil {
					return closeErr
				}
			}
			record.Artifacts[i].Path = path
		}
		if err := validateNativeEncodingRecord(id, record); err != nil {
			return fmt.Errorf("BRG31_ENCODING_REAL_TRIPLET: %w", err)
		}
	}
	out.EncodingRun = runID
	out.EncodingAttempt = attempt
	out.EncodingSummarySHA256 = ciIdentityDigest(summaryRaw)
	out.EncodingRequiredCases = 23
	out.EncodingPositiveExit = 0
	out.EncodingCorruptExit = 1
	out.EncodingRestoredExit = 0
	out.EncodingSourceFiles = map[string]string{}
	for name, sha := range witnesses {
		out.EncodingSourceFiles[name] = sha
	}
	out.EncodingSourceFiles["ShortcutKit@ee52c32937053268e4a035eba4b372fe77ec6411:tools/jxa-prelude.js"] = prelude
	return nil
}

var finalGateMenuMarker = regexp.MustCompile(`(?m)^CHERRI_EXACT_MENU_TAP target=(\S+) actual_label=(\S+) app=(\S+) kind=(\S+) stage=(\S+) result=(\S+)\r?$`)
var finalGateMenuCase = regexp.MustCompile(`(?m)^Test Case '-\[(?:ImportHelperUITests\.)?ImportHelperUITests testInteractWithShortcuts\]' (started\.|passed \([0-9.]+ seconds\)\.)\r?$`)
var finalGateDeviceID = regexp.MustCompile(`^[0-9A-Fa-f]{8}(?:-[0-9A-Fa-f]{4}){3}-[0-9A-Fa-f]{12}$`)

func finalGateSelectionContainsArtifact(selection finalGateSelection, artifact string) bool {
	count := 0
	for _, key := range selection.Artifacts {
		if key == artifact {
			count++
		}
	}
	return artifact != "" && count == 1
}

func finalGateMenu(members map[string][]byte, target, source, run string) (map[string]any, error) {
	if target != "Alpha" && target != "Beta" {
		return nil, fmt.Errorf("BRG31_MENU_TARGET")
	}
	lower := strings.ToLower(target)
	stage := "menu_select_" + lower
	proof, err := decodeNativeEncodingObject(members["menu-"+lower+"-selection-proof.json"])
	if err != nil {
		return nil, fmt.Errorf("BRG31_MENU_PROOF")
	}
	log := members["ui_helper_"+stage+".log"]
	marks := finalGateMenuMarker.FindAllSubmatch(log, -1)
	cases := finalGateMenuCase.FindAllSubmatch(log, -1)
	markBounds := finalGateMenuMarker.FindAllIndex(log, -1)
	caseBounds := finalGateMenuCase.FindAllIndex(log, -1)
	caseLines := 0
	for _, line := range strings.Split(string(log), "\n") {
		if strings.HasPrefix(line, "Test Case '") {
			caseLines++
		}
	}
	if len(marks) != 1 || len(cases) != 2 || caseLines != 2 || string(cases[0][1]) != "started." || !strings.HasPrefix(string(cases[1][1]), "passed ") || caseBounds[0][1] >= markBounds[0][0] || markBounds[0][1] >= caseBounds[1][0] || bytes.Count(log, []byte("** TEST EXECUTE SUCCEEDED **")) != 1 || bytes.Contains(log, []byte("** TEST EXECUTE FAILED **")) || bytes.Contains(log, []byte("** TEST FAILED **")) {
		return nil, fmt.Errorf("BRG31_MENU_ACTUAL_TEST_BOUNDARY")
	}
	marker := marks[0]
	app, kind := string(marker[3]), string(marker[4])
	allowedKind := map[string]bool{"staticText": true, "cell": true, "cellContainingText": true, "button": true, "descendant": true}
	if string(marker[1]) != target || string(marker[2]) != target || (app != "SpringBoard" && app != "Shortcuts") || !allowedKind[kind] || string(marker[5]) != stage || string(marker[6]) != "tapped" {
		return nil, fmt.Errorf("BRG31_MENU_EXACT_TAP")
	}
	clipboard := "CHERRI_MENU_" + strings.ToUpper(target)
	if !bytes.Equal(members["menu-"+lower+"-clipboard-result.txt"], []byte(clipboard+"\n")) {
		return nil, fmt.Errorf("BRG31_MENU_COMPUTED_RESULT")
	}
	want := map[string]any{"target": target, "actual_label": target, "app": app, "kind": kind, "stage": stage, "result": "tapped", "xctest_passed": true, "exact_tap_confirmed": true, "clipboard_sentinel": "BEFORE_MENU_" + strings.ToUpper(target), "clipboard_result": clipboard, "runtime_pass": true, "head_sha": source, "run_id": run, "log_file": "artifacts/ui_helper_" + stage + ".log", "log_sha256": ciIdentityDigest(log), "clipboard_file": "artifacts/menu-" + lower + "-clipboard-result.txt", "clipboard_sha256": ciIdentityDigest(members["menu-"+lower+"-clipboard-result.txt"])}
	if !reflect.DeepEqual(proof, want) {
		return nil, fmt.Errorf("BRG31_MENU_PROOF_DETACHED")
	}
	return map[string]any{"target": target, "selected": "true", "exact_tap": true, "actual_label": target, "host_app": app}, nil
}

func finalGateRuntimeComputed(members map[string][]byte, source, run, attempt string) (map[string]string, []string, error) {
	var primaryNames []string
	for _, field := range finalGatePrimaryFields {
		primaryNames = append(primaryNames, field[0])
	}
	primary, err := finalGateFields(members["clipboard-result.txt"], "CHERRI_IOS27_RUNTIME_OK", primaryNames, true)
	if err != nil {
		return nil, nil, err
	}
	for _, field := range finalGatePrimaryFields {
		if primary[field[0]] != field[1] {
			return nil, nil, fmt.Errorf("BRG31_PRIMARY_COMPUTED_ASSERTION")
		}
	}
	dictionary, err := finalGateFields(members["dict-clipboard-result.txt"], "CHERRI_DICT_RUNTIME_OK", []string{"DICT_A", "DICT_B_LET", "DICT_B_VAR", "DICT_C_NAME", "DICT_D"}, true)
	if err != nil {
		return nil, nil, err
	}
	for key, want := range map[string]string{"DICT_A": "valA", "DICT_B_LET": "valBLet", "DICT_B_VAR": "valBVar", "DICT_D": "valD"} {
		if dictionary[key] != want {
			return nil, nil, fmt.Errorf("BRG31_DICTIONARY_COMPUTED_ASSERTION")
		}
	}
	if dictionary["DICT_C_NAME"] != "sample" && dictionary["DICT_C_NAME"] != "sample.json" {
		return nil, nil, fmt.Errorf("BRG31_DICTIONARY_NAME_RESULT")
	}
	if !bytes.Equal(members["legacy-if-clipboard-result.txt"], []byte("CHERRI_LEGACY_IF_OK\n")) {
		return nil, nil, fmt.Errorf("BRG31_LEGACY_COMPUTED_ASSERTION")
	}
	alpha, err := finalGateMenu(members, "Alpha", source, run)
	if err != nil {
		return nil, nil, err
	}
	beta, err := finalGateMenu(members, "Beta", source, run)
	if err != nil {
		return nil, nil, err
	}
	wanted := map[string]struct {
		Test       string
		Assertions map[string]any
	}{
		"ev-runtime-smoke":               {"smoke", map[string]any{"status": "CHERRI_IOS27_RUNTIME_OK"}},
		"ev-runtime-variables":           {"variables", map[string]any{"variable_result": primary["VAR"], "captured_before": primary["CAPTURED_BEFORE"], "num_captured": primary["NUM_CAPTURED"], "clip_captured": primary["CLIP_CAPTURED"]}},
		"ev-runtime-unicode":             {"unicode", map[string]any{"unicode_output": primary["UNICODE"]}},
		"ev-runtime-indexing":            {"indexing", map[string]any{"index_trace": primary["IDX"]}},
		"ev-runtime-numbers":             {"numbers", map[string]any{"math_trace": primary["MATH"]}},
		"ev-runtime-conditionals":        {"conditionals", map[string]any{"conditional_result": primary["IF"], "short_circuit_trace": primary["SC"]}},
		"ev-runtime-loops":               {"nested-loops", map[string]any{"loop_trace": primary["LOOPS"]}},
		"ev-runtime-functions":           {"functions", map[string]any{"function_result": primary["FUNCTION"], "caller_continuation": primary["FN_CONT"], "defaults_explicit": primary["DEFAULTS_EXPLICIT"], "defaults_omitted": primary["DEFAULTS_OMITTED"], "recursion_fact3": primary["FACT"], "void_output": primary["VOID_OUT"], "partial_envelope_safe": primary["ENV_SAFE"]}},
		"ev-runtime-dict-approach-a":     {"dict-approach-a", map[string]any{"dict_a": dictionary["DICT_A"]}},
		"ev-runtime-dict-approach-b-let": {"dict-approach-b-let", map[string]any{"dict_b_let": dictionary["DICT_B_LET"]}},
		"ev-runtime-dict-approach-b-var": {"dict-approach-b-var", map[string]any{"dict_b_var": dictionary["DICT_B_VAR"]}},
		"ev-runtime-dict-approach-c":     {"dict-approach-c", map[string]any{"dict_c_name": dictionary["DICT_C_NAME"]}},
		"ev-runtime-dict-approach-d":     {"dict-approach-d", map[string]any{"dict_d": dictionary["DICT_D"]}},
		"ev-runtime-menu-structural":     {"menu-structural", map[string]any{"menu_start": true, "cases": []any{"Alpha", "Beta"}}},
		"ev-runtime-menu-alpha":          {"menu-alpha", alpha}, "ev-runtime-menu-beta": {"menu-beta", beta},
		"ev-runtime-legacy-if": {"legacy-if", map[string]any{"legacy_if_result": "CHERRI_LEGACY_IF_OK"}},
	}
	type diagnostic struct {
		EvidenceID        string             `json:"evidence_id"`
		Requirements      []string           `json:"requirements"`
		Tier              string             `json:"tier"`
		TestID            string             `json:"test_id"`
		ImplementationSHA string             `json:"implementation_sha"`
		Result            string             `json:"result"`
		ExitCode          *int               `json:"exit_code"`
		StructuralPass    bool               `json:"structural_pass"`
		RuntimePass       bool               `json:"runtime_pass"`
		Assertions        map[string]any     `json:"assertions"`
		Artifacts         []EvidenceArtifact `json:"artifacts"`
		Detail            string             `json:"detail"`
	}
	var envelope struct {
		Records []diagnostic `json:"records"`
	}
	rawDiagnostics := members["runtime-diagnostics.json"]
	if len(rawDiagnostics) == 0 || len(rawDiagnostics) > 1024*1024 {
		return nil, nil, fmt.Errorf("BRG31_COMPLETE_DIAGNOSTIC_INVENTORY")
	}
	// Keep the original array unchanged; the bounded object wrapper lets the
	// existing strict decoder reject duplicate keys and unknown record fields.
	wrapped := append([]byte(`{"records":`), rawDiagnostics...)
	wrapped = append(wrapped, '}')
	if decodeEditorJSON(wrapped, &envelope) != nil || len(envelope.Records) != len(wanted) {
		return nil, nil, fmt.Errorf("BRG31_COMPLETE_DIAGNOSTIC_INVENTORY")
	}
	records := envelope.Records
	formalNames := []string{"coverage-clipboard-result.txt", "clipboard-result.txt", "dict-clipboard-result.txt", "legacy-if-clipboard-result.txt", "menu-alpha-clipboard-result.txt", "menu-beta-clipboard-result.txt", "menu-alpha-selection-proof.json", "menu-beta-selection-proof.json", "ui_helper_menu_select_alpha.log", "ui_helper_menu_select_beta.log"}
	for name := range finalGateRuntimeSources {
		formalNames = append(formalNames, name+".cherri", name+".shortcut")
	}
	seen := map[string]bool{}
	var ids []string
	for _, record := range records {
		want, ok := wanted[record.EvidenceID]
		if !ok || seen[record.EvidenceID] || record.TestID != "ios27-runtime-poc:"+want.Test || record.ImplementationSHA != source || record.Tier != "ios-runtime" || record.Result != "passed" || record.ExitCode == nil || *record.ExitCode != 0 || !record.StructuralPass || !record.RuntimePass || !reflect.DeepEqual(record.Assertions, want.Assertions) {
			return nil, nil, fmt.Errorf("BRG31_COMPUTED_DIAGNOSTIC_CONTRADICTION")
		}
		seen[record.EvidenceID] = true
		ids = append(ids, record.EvidenceID)
		if len(record.Artifacts) != len(formalNames) {
			return nil, nil, fmt.Errorf("BRG31_RUNTIME_RAW_ARTIFACT_INVENTORY")
		}
		artifactSeen := map[string]bool{}
		for _, artifact := range record.Artifacts {
			prefix := "artifacts/backend-recovery/ci-artifacts/ios27-runtime-poc/"
			name := strings.TrimPrefix(artifact.Path, prefix)
			allowed := false
			for _, known := range formalNames {
				if name == known {
					allowed = true
				}
			}
			data, exists := members[name]
			if !strings.HasPrefix(artifact.Path, prefix) || !allowed || !exists || artifactSeen[name] || artifact.RunID != run || artifact.SHA256 != ciIdentityDigest(data) {
				return nil, nil, fmt.Errorf("BRG31_RUNTIME_RAW_ARTIFACT_BINDING")
			}
			artifactSeen[name] = true
		}
	}
	coverage, err := finalGateCoverage(members, source, run, attempt)
	if err != nil {
		return nil, nil, err
	}
	result := map[string]string{}
	for key, value := range primary {
		result["primary/"+key] = value
	}
	for key, value := range dictionary {
		result["dictionary/"+key] = value
	}
	for key, value := range coverage {
		result["coverage/"+key] = value
	}
	result["menu/Alpha"] = "CHERRI_MENU_ALPHA"
	result["menu/Beta"] = "CHERRI_MENU_BETA"
	result["legacy/result"] = "CHERRI_LEGACY_IF_OK"
	sort.Strings(ids)
	return result, ids, nil
}

func finalGateCoverage(members map[string][]byte, source, run, attempt string) (map[string]string, error) {
	expectedRaw := members["coverage-expected.json"]
	if ciIdentityDigest(expectedRaw) != runtimePOCCoverageExpectedContractSHA256 {
		return nil, fmt.Errorf("BRG31_COVERAGE_ORIGINAL_CONTRACT")
	}
	expected, err := decodeNativeEncodingObject(expectedRaw)
	if err != nil {
		return nil, err
	}
	clipboard := members["coverage-clipboard-result.txt"]
	if !bytes.Equal(clipboard, []byte(finalProvenanceString(expected, "expected_clipboard"))) || ciIdentityDigest(clipboard) != finalProvenanceString(expected, "expected_clipboard_sha256") {
		return nil, fmt.Errorf("BRG31_COVERAGE_COMPUTED_ASSERTIONS")
	}
	fields, err := finalGateFields(clipboard, "CHERRI_S01_F01_COVERAGE_OK", []string{"S01_OUTPUT", "F01_TRACE", "F01_RESULTS"}, false)
	if err != nil {
		return nil, err
	}
	if fields["S01_OUTPUT"] != runtimePOCS01UnicodeOutput || fields["F01_TRACE"] != runtimePOCF01EffectTrace || fields["F01_RESULTS"] != runtimePOCF01BooleanResults {
		return nil, fmt.Errorf("BRG31_COVERAGE_GOLDEN_FIELDS")
	}
	contextRaw := members["coverage-run-context.json"]
	context, err := decodeNativeEncodingObject(contextRaw)
	if err != nil {
		return nil, err
	}
	device, err := decodeNativeEncodingObject(members["runtime-poc-device.json"])
	if err != nil {
		return nil, fmt.Errorf("BRG31_RUNTIME_DEVICE_ORIGINAL")
	}
	// The device filename is fixed by runtime_poc_device.RECEIPT, checked again
	// by source pins and the original publication provenance at the caller.
	shutdown, shutdownOK := finalProvenanceNumber(device, "shutdownExitCode")
	deleted, deletedOK := finalProvenanceNumber(device, "deleteExitCode")
	if len(device) != 12 || finalProvenanceString(device, "schemaVersion") != "1" || finalProvenanceString(device, "implementationSha") != source || finalProvenanceString(device, "runId") != run || finalProvenanceString(device, "runAttempt") != attempt || !finalProvenanceBool(device, "createdByRun", true) || finalProvenanceString(device, "cleanupStatus") != "DELETED" || !shutdownOK || shutdown != 0 || !deletedOK || deleted != 0 || !finalGateDeviceID.MatchString(finalProvenanceString(device, "udid")) || !strings.HasPrefix(finalProvenanceString(device, "deviceName"), "Cherri-PoC-"+run+"-"+attempt+"-") || !strings.HasPrefix(finalProvenanceString(device, "deviceType"), "com.apple.CoreSimulator.SimDeviceType.") || !strings.HasPrefix(finalProvenanceString(device, "runtimeIdentifier"), "com.apple.CoreSimulator.SimRuntime.iOS-27-") {
		return nil, fmt.Errorf("BRG31_RUNTIME_OWNED_DEVICE")
	}
	nonce := finalProvenanceString(context, "nonce")
	version, versionOK := finalProvenanceNumber(context, "schema_version")
	if len(context) != 12 || !versionOK || version != 1 || !finalProvenanceHex(nonce, 32) || finalProvenanceString(context, "implementation_sha") != source || finalProvenanceString(context, "run_id") != run || finalProvenanceString(context, "run_attempt") != attempt || finalProvenanceString(context, "fixture_id") != "CherriS01F01CoveragePOC" || finalProvenanceString(context, "source_sha256") != runtimePOCCoverageSourceSHA256 || finalProvenanceString(context, "expected_contract_sha256") != ciIdentityDigest(expectedRaw) || finalProvenanceString(context, "source_receipt_sha256") != ciIdentityDigest(members["runtime-poc-source-provenance.json"]) || finalProvenanceString(context, "signed_artifact_sha256") != ciIdentityDigest(members["CherriS01F01CoveragePOC.shortcut"]) || finalProvenanceString(context, "device_udid") != finalProvenanceString(device, "udid") {
		return nil, fmt.Errorf("BRG31_COVERAGE_CONTEXT_IDENTITY")
	}
	reset := []byte("BEFORE_S01_F01_" + run + "_" + attempt + "_" + nonce)
	if !bytes.Equal(members["coverage-reset-request.txt"], reset) || !bytes.Equal(members["coverage-reset-observed.txt"], reset) || finalProvenanceString(context, "reset_request_sha256") != ciIdentityDigest(reset) {
		return nil, fmt.Errorf("BRG31_COVERAGE_FRESH_RESET")
	}
	proofRaw := members["coverage-reset-proof.json"]
	proof, err := decodeNativeEncodingObject(proofRaw)
	if err != nil {
		return nil, err
	}
	wantProof := map[string]any{"implementation_sha": source, "run_id": run, "run_attempt": attempt, "context_sha256": ciIdentityDigest(contextRaw), "reset_request_sha256": ciIdentityDigest(reset), "reset_observed_sha256": ciIdentityDigest(reset), "fresh_reset_observed": true}
	if !reflect.DeepEqual(proof, wantProof) {
		return nil, fmt.Errorf("BRG31_COVERAGE_RESET_PROOF")
	}
	result, err := decodeNativeEncodingObject(members["coverage-result.json"])
	if err != nil {
		return nil, err
	}
	want := map[string]any{}
	for key, value := range context {
		want[key] = value
	}
	observedFields := map[string]any{}
	for key, value := range fields {
		observedFields[key] = value
	}
	for key, value := range map[string]any{"context_sha256": ciIdentityDigest(contextRaw), "reset_proof_sha256": ciIdentityDigest(proofRaw), "reset_observed_sha256": ciIdentityDigest(reset), "fresh_reset_observed": true, "observed_clipboard_sha256": ciIdentityDigest(clipboard), "expected_clipboard_sha256": ciIdentityDigest(clipboard), "observed_byte_count": json.Number(strconv.Itoa(len(clipboard))), "expected_byte_count": json.Number(strconv.Itoa(len(clipboard))), "exact_output_match": true, "observed_fields": observedFields, "result": "passed", "exit_code": json.Number("0"), "scope": "SOURCE_BOUND_SIMULATOR_CLIPBOARD_OBSERVATION"} {
		want[key] = value
	}
	if !reflect.DeepEqual(result, want) {
		return nil, fmt.Errorf("BRG31_COVERAGE_RESULT_REPLAY")
	}
	return fields, nil
}

func finalGateValidateNative(ctx context.Context, in *finalProvenanceInputs, expected finalGateExpected, archives map[string]map[string][]byte) (finalGateNativeObservation, error) {
	out := finalGateNativeObservation{RuntimeOriginals: map[string]string{}, RuntimeSourceFiles: map[string]string{}, RelatedRequirementsGranted: []string{}, RuntimeCoverageBoundary: "Five source-bound PoC fixtures and 17 computed diagnostic rows, including the original S01/F01 reset proof. These originals do not establish per-case reset/compile/import/execution for every required safe fixture in RP48; no complete CI02/RP48 or BRG31 runtime closure is inferred."}
	if expected.Native == nil {
		return out, finalProvenancePending("BRG31_NATIVE_CONTEXT_MISSING")
	}
	if err := finalGateEncoding(ctx, in, expected, archives, &out); err != nil {
		return out, err
	}
	native := expected.Native
	selection, ok := expected.Selections[native.RuntimeGate]
	run := in.Expected.Runs[selection.Run]
	artifact, artifactOK := in.Expected.Artifacts[native.RuntimeArtifact]
	if !ok || !artifactOK || artifact.Run != selection.Run || run.WorkflowPath != ".github/workflows/ios27-shortcuts-runtime-poc.yml" || run.SourceSHA != in.Expected.Sources["implementation"].SHA {
		return out, fmt.Errorf("BRG31_RUNTIME_RUN_BINDING")
	}
	if !finalGateSelectionContainsArtifact(selection, native.RuntimeArtifact) {
		return out, fmt.Errorf("BRG31_RUNTIME_NOT_PREDECLARED")
	}
	log, err := finalGateNativeStep(in, selection, "ios27-runtime-poc", native.RuntimeStep, "Run iOS 27 Shortcuts Runtime PoC")
	if err != nil {
		return out, err
	}
	marker := "IOS27_SHORTCUTS_RUNTIME_POC=PASS (All 5 fixtures verified pass)"
	count := 0
	for _, line := range strings.Split(string(log), "\n") {
		if line == marker {
			count++
		}
	}
	if count != 1 || !bytes.Contains(log, []byte("scripts/ios27_runtime_poc.sh")) {
		return out, fmt.Errorf("BRG31_RUNTIME_ORIGINAL_PROCESS_COMPLETION")
	}
	members := archives[native.RuntimeArtifact]
	for name, want := range finalGateRuntimeSources {
		path := "tests/runtime_poc/" + name + ".cherri"
		source, _, err := finalProvenanceGit(ctx, in.Manifest.SourceDirectories["implementation"], "show", run.SourceSHA+":"+path)
		if err != nil || ciIdentityDigest(source) != want || !bytes.Equal(members[name+".cherri"], source) {
			return out, fmt.Errorf("BRG31_RUNTIME_SOURCE_ORIGINAL")
		}
		if data := members[name+".shortcut"]; len(data) < 16 || !bytes.HasPrefix(data, []byte("AEA1")) {
			return out, fmt.Errorf("BRG31_RUNTIME_SIGNED_BYTES_MISSING")
		}
		out.RuntimeSourceFiles[path] = want
	}
	if err := finalGateRuntimeSourceReceipt(members, run.SourceSHA, strconv.FormatInt(run.ID, 10), strconv.Itoa(run.Attempt)); err != nil {
		return out, err
	}
	fields, ids, err := finalGateRuntimeComputed(members, run.SourceSHA, strconv.FormatInt(run.ID, 10), strconv.Itoa(run.Attempt))
	if err != nil {
		return out, err
	}
	for name, data := range members {
		out.RuntimeOriginals[name] = ciIdentityDigest(data)
	}
	out.RuntimeRun = strconv.FormatInt(run.ID, 10)
	out.RuntimeAttempt = run.Attempt
	out.ComputedFields = fields
	out.RuntimeDiagnosticIDs = ids
	// Actual source-bound negatives replay the same observed member set in memory.
	for _, fault := range []string{"computed_output_changed", "missing_reset", "wrong_menu_terminal"} {
		mutant := map[string][]byte{}
		for name, data := range members {
			mutant[name] = data
		}
		switch fault {
		case "computed_output_changed":
			mutant["clipboard-result.txt"] = bytes.Replace(members["clipboard-result.txt"], []byte("FUNCTION=21"), []byte("FUNCTION=22"), 1)
		case "missing_reset":
			delete(mutant, "coverage-reset-observed.txt")
		case "wrong_menu_terminal":
			mutant["ui_helper_menu_select_alpha.log"] = bytes.Replace(members["ui_helper_menu_select_alpha.log"], []byte("testInteractWithShortcuts]' passed"), []byte("testUnrelated]' passed"), 1)
		}
		if _, _, err := finalGateRuntimeComputed(mutant, run.SourceSHA, strconv.FormatInt(run.ID, 10), strconv.Itoa(run.Attempt)); err == nil {
			return out, fmt.Errorf("BRG31_RUNTIME_CAUSAL_NEGATIVE_ACCEPTED")
		}
		out.RuntimeNegativeControls = append(out.RuntimeNegativeControls, fault)
	}
	return out, nil
}

func finalGateRuntimeSourceReceipt(members map[string][]byte, source, run, attempt string) error {
	actual, err := decodeNativeEncodingObject(members["runtime-poc-source-provenance.json"])
	if err != nil {
		return fmt.Errorf("BRG31_RUNTIME_SOURCE_RECEIPT")
	}
	var rows []any
	for _, name := range []string{"CherriS01F01CoveragePOC", "CherriRuntimePOC", "CherriDictionaryRuntimePOC", "CherriMenuRuntimePOC", "CherriLegacyIfPOC"} {
		rows = append(rows, map[string]any{"fixtureId": name, "sourcePath": "tests/runtime_poc/" + name + ".cherri", "sourceSha256": finalGateRuntimeSources[name]})
	}
	wanted := map[string]any{"schemaVersion": "1", "implementationSha": source, "runId": run, "runAttempt": attempt, "syntheticSourcesVerified": true, "sources": rows}
	if !reflect.DeepEqual(actual, wanted) {
		return fmt.Errorf("BRG31_RUNTIME_SOURCE_RECEIPT")
	}
	return nil
}
