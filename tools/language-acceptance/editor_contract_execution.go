package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

//go:embed editor_contract_inventory.json
var editorContractInventory []byte

type editorContractStep struct {
	Name      string   `json:"name"`
	Package   string   `json:"package"`
	PackageID string   `json:"package_id"`
	Tests     []string `json:"tests"`
	Events    []string `json:"events"`
}
type editorContractScenario struct {
	Title      string               `json:"title"`
	Tier       string               `json:"tier"`
	Level      string               `json:"level"`
	Scope      string               `json:"scope"`
	Steps      []editorContractStep `json:"steps"`
	SwiftProbe string               `json:"swift_probe,omitempty"`
}
type editorContractConfig struct {
	Schema      int                               `json:"schema"`
	Scenarios   map[string]editorContractScenario `json:"scenarios"`
	SourcePaths []string                          `json:"source_paths"`
}

var editorContracts = func() editorContractConfig {
	var config editorContractConfig
	if err := json.Unmarshal(editorContractInventory, &config); err != nil {
		panic(err)
	}
	return config
}()

func editorContractTestIDs(id string) []string {
	var ids []string
	for _, step := range editorContracts.Scenarios[id].Steps {
		for _, test := range step.Tests {
			ids = append(ids, step.PackageID+":"+test)
		}
	}
	return ids
}
func editorContractPrefix(id, step string) string { return strings.ToLower(id) + "-" + step }
func editorContractArtifacts(prefix string) []string {
	return []string{prefix + "-process.json", prefix + "-go.jsonl", prefix + "-stderr.log"}
}
func editorContractCommand(step editorContractStep) []string {
	return productionTestCommandInPackage(step.Package, step.Tests)
}
func editorContractBinding(spec editorContractScenario) string {
	raw, _ := json.Marshal(spec)
	return string(raw)
}

func editorContractRun(root, source, id, prefix, directory string, command []string, timeout time.Duration, events []string) (editorProcessReceipt, []EvidenceArtifact, error) {
	return runExactSourceProcess(root, source, id, prefix, directory, command, timeout, events, editorContracts.SourcePaths)
}

func runExactSourceProcess(root, source, id, prefix, directory string, command []string, timeout time.Duration, events, sourcePaths []string) (editorProcessReceipt, []EvidenceArtifact, error) {
	var receipt = editorProcessReceipt{FormatVersion: 1, Requirement: id, SourceCommit: source, SourceCleanBefore: true, Command: command, RequiredTests: events}
	var err error
	receipt.SourceFiles, err = localGateSourceFiles(root, source, sourcePaths)
	if err != nil {
		return receipt, nil, err
	}
	var before, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || before != source {
		return receipt, nil, fmt.Errorf("editor source changed before process")
	}
	var ctx, cancel = context.WithTimeout(context.Background(), timeout)
	defer cancel()
	var cmd = exec.CommandContext(ctx, command[0], command[1:]...)
	cmd.Dir = root
	configureEditorProductionProcess(cmd)
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	var stdout, stderr loopBoundedOutput
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	var started = time.Now()
	receipt.StartedUTC = started.UTC().Format(time.RFC3339Nano)
	var processErr = cmd.Run()
	receipt.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.ElapsedSeconds = time.Since(started).Seconds()
	var code = -1
	if cmd.ProcessState != nil {
		code = cmd.ProcessState.ExitCode()
	}
	var timedOut = ctx.Err() != nil
	receipt.ExitCode = &code
	receipt.TimedOut = &timedOut
	var after, afterClean, afterErr = editorSourceState(root)
	receipt.SourceCleanAfter = afterErr == nil && afterClean && after == source
	var raw, errorOutput = stdout.Bytes(), stderr.Bytes()
	receipt.RawSHA256 = editorSHA(raw)
	receipt.RawBytes = len(raw)
	receipt.StderrSHA256 = editorSHA(errorOutput)
	receipt.StderrBytes = len(errorOutput)
	if processErr == nil {
		if command[0] == "go" {
			var observed []string
			for _, line := range bytes.Split(raw, []byte{'\n'}) {
				var event struct{ Action, Test string }
				if json.Unmarshal(line, &event) == nil && event.Action == "pass" && event.Test != "" {
					observed = append(observed, event.Test)
				}
			}
			sort.Strings(observed)
			receipt.DiscoveredTests = observed
		} else {
			var observed struct{ Passed bool }
			if json.Unmarshal(raw, &observed) == nil && observed.Passed {
				receipt.DiscoveredTests = append([]string{}, events...)
			}
		}
	}
	var encoded, encodeErr = json.MarshalIndent(receipt, "", "  ")
	if encodeErr != nil {
		return receipt, nil, encodeErr
	}
	var names = editorContractArtifacts(prefix)
	var payloads = [][]byte{append(encoded, '\n'), raw, errorOutput}
	var artifacts []EvidenceArtifact
	for i, name := range names {
		var path = filepath.Join(directory, name)
		if err = os.WriteFile(path, payloads[i], 0600); err != nil {
			return receipt, artifacts, err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(payloads[i])})
	}
	if processErr != nil || timedOut || !receipt.SourceCleanAfter {
		return receipt, artifacts, fmt.Errorf("editor process failed: process=%v timeout=%t clean_after=%t", processErr, timedOut, receipt.SourceCleanAfter)
	}
	return receipt, artifacts, nil
}
func validateEditorContractProcess(id, source string, receipt editorProcessReceipt, command []string, events []string, raw, stderr []byte, timeout time.Duration) error {
	return validateExactSourceProcess(id, source, receipt, command, events, raw, stderr, timeout, editorContracts.SourcePaths)
}

func validateExactSourceProcess(id, source string, receipt editorProcessReceipt, command []string, events []string, raw, stderr []byte, timeout time.Duration, sourcePaths []string) error {
	if receipt.FormatVersion != 1 || receipt.Requirement != id || receipt.SourceCommit != source ||
		!receipt.SourceCleanBefore || !receipt.SourceCleanAfter ||
		!reflect.DeepEqual(receipt.Command, command) || !reflect.DeepEqual(receipt.RequiredTests, events) || !reflect.DeepEqual(receipt.DiscoveredTests, events) ||
		receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut ||
		receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) ||
		receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 || !utf8.Valid(stderr) {
		return fmt.Errorf("incomplete editor process/source/raw binding")
	}
	if err := stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, timeout.Seconds()); err != nil {
		return err
	}
	return localGateExactSourceFiles(source, receipt.SourceFiles, sourcePaths)
}
func editorContractSwiftFiles(kind string, exe bool) []string {
	var stem, generated = "position-probe", "position-generated.swift"
	if kind == "publication" {
		stem = "editor-publication-probe"
		generated = "editor-publication-generated.swift"
	}
	if exe {
		stem += ".exe"
	}
	return []string{"receipt.json", "compile.stdout", "compile.stderr", "execute.stdout", "execute.stderr", generated, stem}
}
func editorContractReadSwift(record EvidenceRecord, spec editorContractScenario) (string, bool, error) {
	var directory string
	var exe bool
	for _, artifact := range record.Artifacts {
		if filepath.Base(artifact.Path) == "receipt.json" {
			directory = filepath.Dir(artifact.Path)
		}
		if strings.HasSuffix(artifact.Path, "-probe.exe") {
			exe = true
		}
	}
	if directory == "" {
		return "", false, fmt.Errorf("missing actual Swift receipt")
	}
	for _, name := range editorContractSwiftFiles(spec.SwiftProbe, exe) {
		found := false
		for _, artifact := range record.Artifacts {
			if filepath.Base(artifact.Path) == name {
				if filepath.Dir(artifact.Path) != directory {
					return "", false, fmt.Errorf("detached Swift artifact")
				}
				found = true
			}
		}
		if !found {
			return "", false, fmt.Errorf("missing actual Swift artifact %s", name)
		}
	}
	return directory, exe, nil
}
func editorContractValidateSwift(root, source, directory, kind, executedRoot, executedDirectory string) error {
	var python, err = acceptancePython3("CHERRI_EDITOR_PYTHON")
	if err != nil {
		return err
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var cmd = exec.CommandContext(ctx, python, "-B", "tools/p10r15/editor_swift_intake.py", "--repo", root, "--source", source, "--directory", directory, "--kind", kind, "--executed-repo", executedRoot, "--executed-directory", executedDirectory)
	cmd.Dir = root
	var output, runErr = cmd.CombinedOutput()
	if runErr != nil {
		return fmt.Errorf("actual Swift control intake: %v %s", runErr, output)
	}
	return nil
}

// Match the producer's physical-path operation before recording an invocation.
// This accepts only the two existing owned directories and proves file identity;
// archived command vectors are still compared exactly by the unchanged intake.
func editorCanonicalSwiftDirectories(python, root, directory string) (string, string, error) {
	if !filepath.IsAbs(root) || !filepath.IsAbs(directory) {
		return "", "", fmt.Errorf("Swift execution directories must be absolute")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const script = "import json,sys; from pathlib import Path; print(json.dumps([str(Path(p).resolve(strict=True)) for p in sys.argv[1:]]))"
	command := exec.CommandContext(ctx, python, "-B", "-c", script, root, directory)
	command.Dir = root
	var stdout, stderr loopBoundedOutput
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", "", fmt.Errorf("resolve owned Swift execution directories: %w", err)
	}
	var resolved []string
	if err := json.Unmarshal(stdout.Bytes(), &resolved); err != nil || len(resolved) != 2 {
		return "", "", fmt.Errorf("invalid physical Swift execution directories")
	}
	for index, original := range []string{root, directory} {
		before, beforeErr := os.Stat(original)
		after, afterErr := os.Stat(resolved[index])
		if beforeErr != nil || afterErr != nil || !filepath.IsAbs(resolved[index]) ||
			!before.IsDir() || !after.IsDir() || !os.SameFile(before, after) {
			return "", "", fmt.Errorf("resolved Swift execution directory changed identity")
		}
	}
	return resolved[0], resolved[1], nil
}

func (r *Runner) executeEditorContract(id string) ([]EvidenceArtifact, string, bool, error) {
	var spec, ok = editorContracts.Scenarios[id]
	if !ok {
		return nil, "", false, fmt.Errorf("unknown editor requirement")
	}
	var root, err = filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", false, err
	}
	var source, clean, stateErr = editorSourceState(root)
	if stateErr != nil || !clean || source != r.implSHA {
		return nil, "", false, fmt.Errorf("editor producer requires exact clean source")
	}
	var python string
	if spec.SwiftProbe != "" {
		python, err = acceptancePython3("CHERRI_EDITOR_PYTHON")
		if err != nil {
			return nil, "", false, err
		}
		if _, err = exec.LookPath("swiftc"); err != nil {
			return nil, "", false, fmt.Errorf("actual portable Swift compiler unavailable; no full local editor tier executed")
		}
	}
	var directory, dirErr = stateEvidenceDirectory(r, "editor-contract-"+strings.ToLower(id)+"-")
	if dirErr != nil {
		return nil, "", false, dirErr
	}
	if spec.SwiftProbe != "" {
		root, directory, err = editorCanonicalSwiftDirectories(python, root, directory)
		if err != nil {
			return nil, "", false, err
		}
	}
	var artifacts []EvidenceArtifact
	for _, step := range spec.Steps {
		var receipt, files, runErr = editorContractRun(root, source, id, editorContractPrefix(id, step.Name), directory, editorContractCommand(step), productionTestTimeout(step.Tests)+time.Minute, step.Events)
		artifacts = append(artifacts, files...)
		if runErr != nil {
			return artifacts, "", true, runErr
		}
		raw, readErr := os.ReadFile(filepath.Join(directory, editorContractPrefix(id, step.Name)+"-go.jsonl"))
		if readErr != nil {
			return artifacts, "", true, readErr
		}
		if err = parseStrictNamedProductionOutputInPackage(step.PackageID, step.Events, productionTestTimeout(step.Tests)+time.Minute, raw); err != nil {
			return artifacts, "", true, err
		}
		if err = loopOutputWithinProcess(raw, receipt); err != nil {
			return artifacts, "", true, err
		}
	}
	if spec.SwiftProbe != "" {
		var probeDir = filepath.Join(directory, "swift")
		var cmd = []string{python, "-B", "tools/p10r15/editor_" + spec.SwiftProbe + "_probe.py", "--repo", root, "--output", probeDir}
		_, files, runErr := editorContractRun(root, source, id, editorContractPrefix(id, "swift"), directory, cmd, 180*time.Second, []string{spec.SwiftProbe})
		artifacts = append(artifacts, files...)
		var exe = filepath.Ext(os.Args[0]) == ".exe"
		for _, name := range editorContractSwiftFiles(spec.SwiftProbe, exe) {
			var path = filepath.Join(probeDir, name)
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				if runErr == nil {
					runErr = readErr
				}
				continue
			}
			artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(raw)})
		}
		if runErr != nil {
			return artifacts, "", true, runErr
		}
		if err = editorContractValidateSwift(root, source, probeDir, spec.SwiftProbe, root, probeDir); err != nil {
			return artifacts, "", true, err
		}
	}
	return artifacts, spec.Scope, true, nil
}
func (r *Runner) runEditorContract(id string) {
	var spec, ok = editorContracts.Scenarios[id]
	if !ok {
		return
	}
	var artifacts, detail, executed, err = r.executeEditorContract(id)
	if err != nil {
		detail = err.Error()
	}
	if !executed {
		r.results[id] = CaseResult{ID: id, Group: "editor", Requirement: spec.Title, TestLevel: spec.Level, Status: "NOT_RUN", Detail: detail}
		return
	}
	var first = len(r.records)
	r.Record(id, "editor", spec.Title, spec.Level, []string{spec.Tier}, err == nil, "PASSED", detail)
	for i := first; i < len(r.records); i++ {
		var record = &r.records[i]
		record.TestID = editorContractTestIDs(id)[0]
		record.Command = editorContractCommand(spec.Steps[0])
		record.Artifacts = artifacts
		record.Assertions["editor_contract_scenario"] = editorContractBinding(spec)
		record.Assertions["scope"] = spec.Scope
	}
}
func validateEditorContractRecord(id string, record EvidenceRecord) error {
	var spec, ok = editorContracts.Scenarios[id]
	if !ok {
		return nil
	}
	// No complete editor XCTest intake exists yet; external tiers cannot
	// borrow local evidence or generic UI metadata. They remain unproved.
	if record.Tier != spec.Tier || record.Result != "passed" || record.ExitCode != 0 ||
		!reflect.DeepEqual(record.Requirements, []string{id}) || record.TestID != editorContractTestIDs(id)[0] ||
		!reflect.DeepEqual(record.Command, editorContractCommand(spec.Steps[0])) ||
		record.Assertions["editor_contract_scenario"] != editorContractBinding(spec) || record.Assertions["scope"] != spec.Scope {
		return fmt.Errorf("detached editor local contract %s", id)
	}
	var names []string
	for _, step := range spec.Steps {
		names = append(names, editorContractArtifacts(editorContractPrefix(id, step.Name))...)
	}
	var swiftDir string
	if spec.SwiftProbe != "" {
		var exe bool
		var err error
		swiftDir, exe, err = editorContractReadSwift(record, spec)
		if err != nil {
			return err
		}
		names = append(names, editorContractArtifacts(editorContractPrefix(id, "swift"))...)
		names = append(names, editorContractSwiftFiles(spec.SwiftProbe, exe)...)
	}
	var data, err = stateReadArtifacts(record.Artifacts, names)
	if err != nil {
		return err
	}
	for _, step := range spec.Steps {
		var prefix = editorContractPrefix(id, step.Name)
		var receipt editorProcessReceipt
		if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
			return err
		}
		var raw, stderr = data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"]
		if err = validateEditorContractProcess(id, record.ImplementationSHA, receipt, editorContractCommand(step), step.Events, raw, stderr, productionTestTimeout(step.Tests)+time.Minute); err != nil {
			return err
		}
		if err = parseStrictNamedProductionOutputInPackage(step.PackageID, step.Events, productionTestTimeout(step.Tests)+time.Minute, raw); err != nil {
			return err
		}
		if err = loopOutputWithinProcess(raw, receipt); err != nil {
			return err
		}
	}
	if spec.SwiftProbe != "" {
		var prefix = editorContractPrefix(id, "swift")
		var receipt editorProcessReceipt
		if err = decodeEditorJSON(data[prefix+"-process.json"], &receipt); err != nil {
			return err
		}
		var command = receipt.Command
		var root, rootErr = filepath.Abs(resolveRepoRoot())
		if rootErr != nil {
			return rootErr
		}
		if len(command) != 7 || !strings.HasPrefix(strings.ToLower(filepath.Base(command[0])), "python") || command[1] != "-B" || command[2] != "tools/p10r15/editor_"+spec.SwiftProbe+"_probe.py" ||
			command[3] != "--repo" || !filepath.IsAbs(command[4]) || command[5] != "--output" || !filepath.IsAbs(command[6]) {
			// Kept below as a closed exact vector check; no command is executed from the receipt.
			return fmt.Errorf("detached editor Swift process selection")
		}
		if err = validateEditorContractProcess(id, record.ImplementationSHA, receipt, command, []string{spec.SwiftProbe}, data[prefix+"-go.jsonl"], data[prefix+"-stderr.log"], 180*time.Second); err != nil {
			return err
		}
		if err = editorContractValidateSwift(root, record.ImplementationSHA, swiftDir, spec.SwiftProbe, command[4], command[6]); err != nil {
			return err
		}
	}
	return nil
}

func editorContractSortedIDs() []string {
	var ids []string
	for id := range editorContracts.Scenarios {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
