package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var documentationRecoveryIntakeSources = []string{
	"documentation_recovery_intake.py", "documentation_recovery_examples.py",
	"documentation_recovery_wrappers.py", "package_skill.py",
}

type documentationRecoveryNestedReceipt struct {
	SchemaVersion     int
	Requirement       string
	Tuple             map[string]string
	Modes             []string
	ReportSHA256      map[string]string
	ArchiveSHA256     string
	OriginalFiles     int
	ExecutionInterval map[string]string
	Status            string
}

// Stage the verifier and its reused parser/package helpers from Git K.
// Relocated evidence never executes a caller-supplied script or executable.
func documentationRecoveryNestedOperation(source, operation, archive, directory, id, startedUTC, finishedUTC string) (documentationRecoveryNestedReceipt, error) {
	var result documentationRecoveryNestedReceipt
	python, err := acceptancePython3("CHERRI_DOCS_PYTHON")
	if err != nil {
		return result, err
	}
	docs := os.Getenv("CHERRI_D03_DOCS_REPO")
	if docs == "" {
		return result, fmt.Errorf("immutable docs Git repository is required for nested intake")
	}
	root, err := filepath.Abs(resolveRepoRoot())
	if err != nil {
		return result, err
	}
	stage, err := os.MkdirTemp("", "cherri-docs-immutable-intake-")
	if err != nil {
		return result, err
	}
	defer os.RemoveAll(stage)
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	for _, name := range documentationRecoveryIntakeSources {
		cmd := exec.CommandContext(ctx, "git", "--no-replace-objects", "show", source+":tools/p10r15/"+name)
		cmd.Dir = root
		raw, readErr := cmd.Output()
		if readErr != nil || len(raw) == 0 || len(raw) > 128<<10 {
			return result, fmt.Errorf("immutable documentation verifier source missing: %s %v", name, readErr)
		}
		if err := os.WriteFile(filepath.Join(stage, name), raw, 0600); err != nil {
			return result, err
		}
	}
	bootstrap := "import runpy,sys;sys.path.insert(0,sys.argv[1]);sys.argv=sys.argv[2:];runpy.run_path(sys.argv[0],run_name='__main__')"
	args := []string{"-I", "-B", "-c", bootstrap, stage, filepath.Join(stage, "documentation_recovery_intake.py"),
		operation, "--repo", root, "--source", source, "--docs-repo", docs, "--archive", archive, "--requirement", id,
		"--started-utc", startedUTC, "--finished-utc", finishedUTC}
	if operation == "pack" {
		args = append(args, "--directory", directory)
	}
	cmd := exec.CommandContext(ctx, python, args...)
	cmd.Dir = root
	configureEditorProductionProcess(cmd)
	var output, errors loopBoundedOutput
	cmd.Stdout, cmd.Stderr = &output, &errors
	if err := cmd.Run(); err != nil {
		return result, fmt.Errorf("nested documentation %s failed: %v %s", operation, err, errors.Bytes())
	}
	if _, err := decodeNativeEncodingObject(bytes.TrimSpace(output.Bytes())); err != nil {
		return result, err
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		return result, err
	}
	if result.SchemaVersion != 1 || result.Requirement != id || result.Status != "VERIFIED_NESTED_DOCUMENTATION_EVIDENCE" ||
		result.OriginalFiles < 700 || result.Tuple["K"] != source || !documentationHash(result.Tuple["S"], 40) ||
		!documentationHash(result.Tuple["D"], 40) || !editorDigest(result.ArchiveSHA256) ||
		result.ExecutionInterval["startedUTC"] != startedUTC || result.ExecutionInterval["finishedUTC"] != finishedUTC {
		return result, fmt.Errorf("incomplete independently validated nested documentation evidence")
	}
	return result, nil
}

func documentationRecoveryBindNestedReports(receipt documentationRecoveryNestedReceipt, raw []byte) error {
	var output strings.Builder
	for _, line := range bytes.Split(raw, []byte{'\n'}) {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var event struct{ Output string }
		if err := json.Unmarshal(line, &event); err != nil {
			return err
		}
		output.WriteString(event.Output)
	}
	for _, mode := range receipt.Modes {
		digest := receipt.ReportSHA256[mode]
		if !editorDigest(digest) || strings.Count(output.String(), "P15_DOCUMENTATION_RECOVERY_REPORT_SHA256:"+mode+":"+digest+"\n") != 1 {
			return fmt.Errorf("nested %s report differs from original source-bound Go process", mode)
		}
	}
	return nil
}
