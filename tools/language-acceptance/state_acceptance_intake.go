package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var stateRaceTests = []string{
	"TestBackendRecovery06MixedEntryPoints",
	"TestBackendRecovery06EmitterErrorAndCancellation",
	"TestBackendRecovery06EmitterErrorAndCancellation/emitter_error",
	"TestBackendRecovery06EmitterErrorAndCancellation/cancellation",
	"TestBackendRecovery06ImportEntryPoints",
	"TestBackendRecovery06MobilePanicRecovery",
}

var stateRaceCommand = []string{"go", "test", "-race", "-count=1", "-json", "-timeout=8m", "-run",
	"^(TestBackendRecovery06MixedEntryPoints|TestBackendRecovery06EmitterErrorAndCancellation|TestBackendRecovery06ImportEntryPoints|TestBackendRecovery06MobilePanicRecovery)$", "."}

var stateRaceSourcePaths = []string{
	"compiler_state.go", "language_backend_session.go", "language_backend_adapter.go",
	"language_emit_adapter.go", "language_registry_adapter.go", "mobile_compiler.go", "ios_bridge.go",
	"internal/language/service/service.go", "internal/language/service/registry_lifetime.go",
	"internal/language/schema/registry_clone.go", "internal/language/assets/cache.go",
	"hybrid_import.go", "semantic_import_planner.go", "reverse_action_registry.go",
	"p15_backend_import_state_test.go", "p15_backend_state_panic_test.go",
	"p15_backend_state_concurrency_test.go", "p15_state_race_enabled_test.go", "p15_state_race_disabled_test.go",
	"tools/language-acceptance/run_state_race.py", ".github/workflows/p10r15-state-race.yml",
}

var stateRaceArtifactNames = []string{"race-receipt.json", "environment.json", "source-files.json", "go-test.jsonl", "go-test.stderr"}

const stateRaceScope = "Actual unsigned canonical/mobile/decompile/catalog/service state isolation, emitter error/cancellation/panic recovery and Linux CGO race detector; no signing or Apple runtime claim"
const stateRaceOriginalScope = "Actual bounded race scenarios only. Formal BRG06/RP51 intake and RP51 benchmark evidence remain separate required gates."
const stateSecurityTest = "TestP15CLIAnalysisHasNoExternalEffects"
const stateSecurityScope = "Actual production analysis CLI with filesystem, live loopback signer and shortcuts/open observers plus positive controls; no signing success or Apple runtime claim"

type stateRaceReceipt struct {
	FormatVersion            int            `json:"format_version"`
	SourceCommit             string         `json:"source_commit"`
	SourceCleanBefore        bool           `json:"source_clean_before"`
	SourceCleanAfter         bool           `json:"source_clean_after"`
	Repository               string         `json:"repository"`
	RunID                    string         `json:"run_id"`
	RunAttempt               string         `json:"run_attempt"`
	Command                  []string       `json:"command"`
	StartedUTC               string         `json:"started_utc"`
	FinishedUTC              string         `json:"finished_utc"`
	ElapsedSeconds           float64        `json:"elapsed_seconds"`
	ExitCode                 *int           `json:"exit_code"`
	TimedOut                 *bool          `json:"timed_out"`
	RawSHA256                string         `json:"raw_sha256"`
	RawBytes                 int            `json:"raw_bytes"`
	StderrSHA256             string         `json:"stderr_sha256"`
	StderrBytes              int            `json:"stderr_bytes"`
	EnvironmentSHA256        string         `json:"environment_sha256"`
	SourceFilesSHA256        string         `json:"source_files_sha256"`
	Status                   string         `json:"status"`
	Issues                   []string       `json:"issues"`
	RequiredTests            []string       `json:"required_tests"`
	Observations             map[string]any `json:"observations"`
	FormalRequirementsProved []string       `json:"formal_requirements_proved"`
	Scope                    string         `json:"scope"`
}

func stateReadArtifacts(artifacts []EvidenceArtifact, names []string) (map[string][]byte, error) {
	if len(artifacts) != len(names) {
		return nil, fmt.Errorf("missing complete state process artifact set")
	}
	want := map[string]bool{}
	for _, name := range names {
		want[name] = true
	}
	data := map[string][]byte{}
	for _, artifact := range artifacts {
		name := filepath.Base(artifact.Path)
		_, duplicate := data[name]
		if !want[name] || duplicate {
			return nil, fmt.Errorf("foreign or duplicate state artifact")
		}
		info, err := os.Stat(artifact.Path)
		if err != nil || !info.Mode().IsRegular() || info.Size() > 4<<20 {
			return nil, fmt.Errorf("missing or oversized state artifact")
		}
		raw, err := os.ReadFile(artifact.Path)
		if err != nil || !editorDigest(artifact.SHA256) || editorSHA(raw) != artifact.SHA256 {
			return nil, fmt.Errorf("state artifact content digest mismatch")
		}
		if strings.HasSuffix(name, ".json") && !utf8.Valid(raw) {
			return nil, fmt.Errorf("invalid UTF-8 in state JSON artifact")
		}
		data[name] = raw
	}
	return data, nil
}

func stateExactSourceFiles(source string, files []editorSourceFile, paths []string) error {
	if len(files) != len(paths) {
		return fmt.Errorf("missing complete state source inventory")
	}
	expected, err := editorGitSourceFiles(resolveRepoRoot(), source, paths)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(files, expected) {
		return fmt.Errorf("state source path/digest differs from exact immutable Git blobs")
	}
	return nil
}

func stateTiming(start, finish string, elapsed, limit float64) error {
	s, e1 := time.Parse(time.RFC3339Nano, start)
	f, e2 := time.Parse(time.RFC3339Nano, finish)
	if e1 != nil || e2 != nil || !f.After(s) || elapsed <= 0 || elapsed > limit || math.Abs(f.Sub(s).Seconds()-elapsed) > 1 {
		return fmt.Errorf("invalid bounded state process timing")
	}
	return nil
}

// Reuse the authoritative actual runner's event/witness evaluation at its exact
// immutable source. Never execute a dirty replacement or infer pass from the
// receipt summary. All input files and source hashes are checked before replay.
func stateReplayRaceOutput(source string, data map[string][]byte, receipt stateRaceReceipt) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	git := exec.CommandContext(ctx, "git", "--no-replace-objects", "show", source+":tools/language-acceptance/run_state_race.py")
	git.Dir = resolveRepoRoot()
	git.WaitDelay = 2 * time.Second
	canonical, err := git.Output()
	if err != nil || len(canonical) == 0 || len(canonical) > 128<<10 {
		return fmt.Errorf("immutable race evaluator unavailable: %v", err)
	}
	dir, err := os.MkdirTemp("", "cherri-state-intake-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	path := filepath.Join(dir, "run_state_race.py")
	if err = os.WriteFile(path, canonical, 0600); err != nil {
		return err
	}
	python, err := acceptancePython3("CHERRI_STATE_PYTHON")
	if err != nil {
		return err
	}
	input, err := json.Marshal(map[string]any{"raw_base64": base64.StdEncoding.EncodeToString(data["go-test.jsonl"]), "stderr_base64": base64.StdEncoding.EncodeToString(data["go-test.stderr"]), "environment": json.RawMessage(data["environment.json"]), "exit_code": *receipt.ExitCode, "timed_out": *receipt.TimedOut, "clean_before": receipt.SourceCleanBefore, "clean_after": receipt.SourceCleanAfter})
	if err != nil {
		return err
	}
	const adapter = `import base64, json, runpy, sys
runner = runpy.run_path(sys.argv[1])
value = json.load(sys.stdin, object_pairs_hook=runner["unique_object"])
result = runner["evaluate"](base64.b64decode(value["raw_base64"], validate=True), base64.b64decode(value["stderr_base64"], validate=True), value["exit_code"], value["timed_out"], value["environment"], value["clean_before"], value["clean_after"])
print(json.dumps(result, sort_keys=True))
`
	command := exec.CommandContext(ctx, python, "-I", "-B", "-c", adapter, path)
	command.Stdin = bytes.NewReader(input)
	command.WaitDelay = time.Second
	output, err := command.Output()
	if err != nil || len(output) > 128<<10 {
		return fmt.Errorf("actual race witness replay failed: %v (context %v)", err, ctx.Err())
	}
	var actual struct {
		Status        string         `json:"status"`
		Issues        []string       `json:"issues"`
		RequiredTests []string       `json:"required_tests"`
		Observations  map[string]any `json:"observations"`
	}
	if err := decodeEditorJSON(output, &actual); err != nil {
		return err
	}
	if actual.Status != "PASSED" || len(actual.Issues) != 0 || !reflect.DeepEqual(actual.RequiredTests, receipt.RequiredTests) || !reflect.DeepEqual(actual.Observations, receipt.Observations) {
		return fmt.Errorf("raw race events do not prove the complete recorded execution: %v", actual.Issues)
	}
	return nil
}

func validateStateRaceRecord(id string, record EvidenceRecord) error {
	wantProducer := ""
	if id == "BRG06" && record.Tier == "unit" {
		wantProducer = stateRaceTests[1]
	}
	if id == "BRG06" && record.Tier == "integration" || id == "RP51" && record.Tier == "race" {
		wantProducer = stateRaceTests[0]
	}
	if wantProducer == "" {
		return fmt.Errorf("unexecuted state/race tier for %s", id)
	}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != "github.com/electrikmilk/cherri:"+wantProducer || !reflect.DeepEqual(record.Command, stateRaceCommand) || record.Assertions["required_production_tests"] != strings.Join(stateRaceTests, "\n") || record.Assertions["scope"] != stateRaceScope {
		return fmt.Errorf("detached or partial state/race producer")
	}
	data, err := stateReadArtifacts(record.Artifacts, stateRaceArtifactNames)
	if err != nil {
		return err
	}
	var receipt stateRaceReceipt
	if err = decodeEditorJSON(data["race-receipt.json"], &receipt); err != nil {
		return err
	}
	if receipt.FormatVersion != 1 || receipt.SourceCommit != record.ImplementationSHA || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter || receipt.Repository != "davidpovarsky/cherri" || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(receipt.RunID) || !regexp.MustCompile(`^[1-9][0-9]*$`).MatchString(receipt.RunAttempt) || !reflect.DeepEqual(receipt.Command, stateRaceCommand) || !reflect.DeepEqual(receipt.RequiredTests, stateRaceTests) || receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut || receipt.Status != "PASSED" || receipt.Issues == nil || len(receipt.Issues) != 0 || receipt.FormalRequirementsProved == nil || len(receipt.FormalRequirementsProved) != 0 || receipt.Scope != stateRaceOriginalScope {
		return fmt.Errorf("incomplete, failed, dirty or substituted race process receipt")
	}
	if receipt.RawSHA256 != editorSHA(data["go-test.jsonl"]) || receipt.RawBytes != len(data["go-test.jsonl"]) || receipt.StderrSHA256 != editorSHA(data["go-test.stderr"]) || receipt.StderrBytes != len(data["go-test.stderr"]) || receipt.EnvironmentSHA256 != editorSHA(data["environment.json"]) || receipt.SourceFilesSHA256 != editorSHA(data["source-files.json"]) || record.Assertions["race_run_id"] != receipt.RunID || record.Assertions["race_run_attempt"] != receipt.RunAttempt {
		return fmt.Errorf("unbound race process content or run identity")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, 600); err != nil {
		return err
	}
	var environment map[string]string
	if err = decodeEditorJSON(data["environment.json"], &environment); err != nil {
		return err
	}
	keys := []string{"CC", "CGO_ENABLED", "GOARCH", "GOOS", "GOVERSION", "platform", "python", "go_version", "gcc_version", "GOMAXPROCS", "GOFLAGS", "GORACE"}
	if len(environment) != len(keys) {
		return fmt.Errorf("incomplete race environment")
	}
	for _, key := range keys {
		if _, present := environment[key]; !present {
			return fmt.Errorf("missing race environment field %s", key)
		}
	}
	if environment["GOOS"] != "linux" || environment["CGO_ENABLED"] != "1" || environment["CC"] != "gcc" || (environment["GOARCH"] != "amd64" && environment["GOARCH"] != "arm64") || environment["GORACE"] != "" || !regexp.MustCompile(`^(-p=[1-9][0-9]*)?$`).MatchString(environment["GOFLAGS"]) {
		return fmt.Errorf("unsupported or suppressible race configuration")
	}
	if environment["GOMAXPROCS"] != "" {
		n, e := strconv.Atoi(environment["GOMAXPROCS"])
		if e != nil || n < 1 {
			return fmt.Errorf("invalid race concurrency configuration")
		}
	}
	if environment["GOVERSION"] == "" || environment["go_version"] != "go version "+environment["GOVERSION"]+" linux/"+environment["GOARCH"] || environment["platform"] == "" || environment["python"] == "" || environment["gcc_version"] == "" {
		return fmt.Errorf("inconsistent actual race toolchain metadata")
	}
	var sources struct {
		Files []editorSourceFile `json:"source_files"`
	}
	wrapped := append([]byte(`{"source_files":`), data["source-files.json"]...)
	wrapped = append(wrapped, '}')
	if err = decodeEditorJSON(wrapped, &sources); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, sources.Files, stateRaceSourcePaths); err != nil {
		return err
	}
	return stateReplayRaceOutput(record.ImplementationSHA, data, receipt)
}

func validateBackendStateRecord(id string, record EvidenceRecord) error {
	if id != "BRG06" && id != "RP51" {
		return nil
	}
	if id == "RP51" && record.Tier == "benchmark" {
		return nil
	} // Existing strict benchmark intake owns this tier.
	if id == "BRG06" && record.Tier == "security" {
		return validateStateSecurityRecord(record)
	}
	return validateStateRaceRecord(id, record)
}
