package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"
	"unicode/utf8"
)

var stateSecurityArtifactNames = []string{"state-security-process.json", "state-security-go.jsonl", "state-security-stderr.log"}

func stateSecuritySourcePaths() []string {
	return append(append([]string{}, stateRaceSourcePaths...), "p15_cli_acceptance_test.go", "main.go", "language_cli_adapter.go")
}

func parseStateSecurityOutput(raw []byte) error {
	if !utf8.Valid(raw) {
		return fmt.Errorf("invalid UTF-8 in security Go events")
	}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	scanner.Buffer(make([]byte, 4096), 4<<20)
	runs, passes, packages := 0, 0, 0
	for scanner.Scan() {
		if packages != 0 {
			return fmt.Errorf("state security output continues after package completion")
		}
		if _, err := decodeNativeEncodingObject(scanner.Bytes()); err != nil {
			return err
		}
		var event struct{ Action, Package, Test, Output string }
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			return err
		}
		if event.Package != "github.com/electrikmilk/cherri" || event.Test != "" && event.Test != stateSecurityTest {
			return fmt.Errorf("foreign or unselected security test")
		}
		switch event.Action {
		case "run":
			if event.Test != stateSecurityTest || runs != 0 {
				return fmt.Errorf("duplicate or unnamed security execution")
			}
			runs++
		case "pass":
			if event.Test == "" {
				if runs != 1 || passes != 1 {
					return fmt.Errorf("security package completed without required test")
				}
				packages++
			} else {
				if runs != 1 || passes != 0 {
					return fmt.Errorf("security test passed without exact prior execution")
				}
				passes++
			}
		case "start", "output":
		default:
			return fmt.Errorf("state security test failed, skipped or used an unselected event")
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if runs != 1 || passes != 1 || packages != 1 {
		return fmt.Errorf("missing actual complete security test execution")
	}
	return nil
}

func validateStateSecurityRecord(record EvidenceRecord) error {
	tests := []string{stateSecurityTest}
	if record.Result != "passed" || record.ExitCode != 0 || !reflect.DeepEqual(record.Requirements, []string{"BRG06"}) || record.Tier != "security" || record.TestID != "github.com/electrikmilk/cherri:"+stateSecurityTest || !reflect.DeepEqual(record.Command, productionTestCommand(tests)) || record.Assertions["required_production_tests"] != stateSecurityTest || record.Assertions["scope"] != stateSecurityScope {
		return fmt.Errorf("detached state analysis-side-effect producer")
	}
	data, err := stateReadArtifacts(record.Artifacts, stateSecurityArtifactNames)
	if err != nil {
		return err
	}
	var receipt editorProcessReceipt
	if err = decodeEditorJSON(data["state-security-process.json"], &receipt); err != nil {
		return err
	}
	raw, stderr := data["state-security-go.jsonl"], data["state-security-stderr.log"]
	if receipt.FormatVersion != 1 || receipt.Requirement != "BRG06" || receipt.SourceCommit != record.ImplementationSHA || !receipt.SourceCleanBefore || !receipt.SourceCleanAfter || !reflect.DeepEqual(receipt.Command, record.Command) || !reflect.DeepEqual(receipt.RequiredTests, tests) || !reflect.DeepEqual(receipt.DiscoveredTests, tests) || receipt.ExitCode == nil || *receipt.ExitCode != 0 || receipt.TimedOut == nil || *receipt.TimedOut || receipt.RawSHA256 != editorSHA(raw) || receipt.RawBytes != len(raw) || receipt.StderrSHA256 != editorSHA(stderr) || receipt.StderrBytes != len(stderr) || receipt.BenchmarkSHA256 != "" || receipt.BenchmarkBytes != 0 {
		return fmt.Errorf("incomplete state security process/source binding")
	}
	if err = stateTiming(receipt.StartedUTC, receipt.FinishedUTC, receipt.ElapsedSeconds, (productionTestTimeout(tests) + time.Minute).Seconds()); err != nil {
		return err
	}
	if err = stateExactSourceFiles(record.ImplementationSHA, receipt.SourceFiles, stateSecuritySourcePaths()); err != nil {
		return err
	}
	return parseStateSecurityOutput(raw)
}

func (r *Runner) executeStateSecurity() ([]EvidenceArtifact, string, error) {
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return nil, "", err
	}
	before, clean, err := editorSourceState(root)
	if err != nil || !clean || before != r.implSHA {
		return nil, "", fmt.Errorf("state security producer requires exact clean source: %s clean=%t error=%v", before, clean, err)
	}
	directory, err := stateEvidenceDirectory(r, "state-security-")
	if err != nil {
		return nil, "", err
	}
	tests := []string{stateSecurityTest}
	receipt := editorProcessReceipt{FormatVersion: 1, Requirement: "BRG06", SourceCommit: before, SourceCleanBefore: true, Command: productionTestCommand(tests), RequiredTests: tests}
	receipt.SourceFiles, err = editorGitSourceFiles(root, before, stateSecuritySourcePaths())
	if err != nil {
		return nil, "", err
	}
	ctx, cancel := context.WithTimeout(context.Background(), productionTestTimeout(tests)+time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, receipt.Command[0], receipt.Command[1:]...)
	command.Dir = root
	configureEditorProductionProcess(command)
	var out, errors bytes.Buffer
	command.Stdout = &out
	command.Stderr = &errors
	started := time.Now()
	receipt.StartedUTC = started.UTC().Format(time.RFC3339Nano)
	processErr := command.Run()
	receipt.FinishedUTC = time.Now().UTC().Format(time.RFC3339Nano)
	receipt.ElapsedSeconds = time.Since(started).Seconds()
	exitCode := -1
	if command.ProcessState != nil {
		exitCode = command.ProcessState.ExitCode()
	}
	timedOut := ctx.Err() != nil
	receipt.ExitCode = &exitCode
	receipt.TimedOut = &timedOut
	after, afterClean, stateErr := editorSourceState(root)
	receipt.SourceCleanAfter = stateErr == nil && afterClean && after == before
	raw, stderr := out.Bytes(), errors.Bytes()
	receipt.RawSHA256 = editorSHA(raw)
	receipt.RawBytes = len(raw)
	receipt.StderrSHA256 = editorSHA(stderr)
	receipt.StderrBytes = len(stderr)
	parseErr := parseStateSecurityOutput(raw)
	if parseErr == nil {
		receipt.DiscoveredTests = append([]string{}, tests...)
	}
	encoded, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, "", err
	}
	contents := map[string][]byte{"state-security-process.json": append(encoded, '\n'), "state-security-go.jsonl": raw, "state-security-stderr.log": stderr}
	var artifacts []EvidenceArtifact
	for _, name := range stateSecurityArtifactNames {
		path := filepath.Join(directory, name)
		if err = os.WriteFile(path, contents[name], 0600); err != nil {
			return artifacts, "", err
		}
		artifacts = append(artifacts, EvidenceArtifact{Path: path, SHA256: editorSHA(contents[name])})
	}
	if processErr != nil || parseErr != nil || timedOut || !receipt.SourceCleanAfter {
		return artifacts, "", fmt.Errorf("actual state security process failed: process=%v parse=%v timeout=%t clean=%t stderr=%s", processErr, parseErr, timedOut, receipt.SourceCleanAfter, strings.TrimSpace(string(stderr)))
	}
	return artifacts, "Executed actual " + stateSecurityTest + " with operational filesystem, live signer and command observers; no analysis side effects", nil
}
