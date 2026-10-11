package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/electrikmilk/cherri/internal/language/backend"
	"github.com/electrikmilk/cherri/internal/language/schema"
)

var (
	ciRunDiscoveryFunc       = discoverCIRuns
	defaultEvidencePathsFunc = defaultEvidencePaths
)

func fileArtifactIfPresent(relPath string, runID string) *EvidenceArtifact {
	content, err := os.ReadFile(relPath)
	if err != nil {
		return nil
	}
	h := sha256.Sum256(content)
	return &EvidenceArtifact{
		Path:   filepath.ToSlash(relPath),
		SHA256: hex.EncodeToString(h[:]),
		RunID:  runID,
	}
}

func discoverCIRuns(commitSHA string) ([]CIRunRecord, error) {
	cmd := exec.Command("gh", "run", "list", "--commit", commitSHA, "--json", "databaseId,name,conclusion,event,attempt,headSha")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("CI evidence unavailable: gh run list failed: %w", err)
	}
	var ghRuns []struct {
		DatabaseID int64  `json:"databaseId"`
		Name       string `json:"name"`
		Conclusion string `json:"conclusion"`
		Event      string `json:"event"`
		Attempt    int    `json:"attempt"`
		HeadSHA    string `json:"headSha"`
	}
	if err := json.Unmarshal(out, &ghRuns); err != nil {
		return nil, fmt.Errorf("CI evidence unavailable: failed to parse gh run list: %w", err)
	}
	if len(ghRuns) == 0 {
		return nil, fmt.Errorf("CI evidence unavailable: no CI runs found for commit %s", commitSHA)
	}
	var records []CIRunRecord
	for _, r := range ghRuns {
		records = append(records, CIRunRecord{
			RunID:        fmt.Sprint(r.DatabaseID),
			Repository:   "davidpovarsky/cherri",
			Event:        r.Event,
			Attempt:      r.Attempt,
			HeadSHA:      r.HeadSHA,
			CheckoutSHA:  r.HeadSHA,
			Conclusion:   r.Conclusion,
			WorkflowName: r.Name,
		})
	}
	return records, nil
}

func ingestEvidenceFile(path string) ([]EvidenceRecord, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var recs []EvidenceRecord
	if err := json.Unmarshal(data, &recs); err == nil && len(recs) > 0 {
		return recs, nil
	}
	var wrapper struct {
		Records []EvidenceRecord `json:"records"`
	}
	if err := json.Unmarshal(data, &wrapper); err == nil && len(wrapper.Records) > 0 {
		return wrapper.Records, nil
	}
	return nil, fmt.Errorf("no evidence records parsed from %s", path)
}

func firstExistingFile(paths ...string) string {
	for _, p := range paths {
		if info, err := os.Stat(p); err == nil && !info.IsDir() {
			return p
		}
	}
	return ""
}

func defaultEvidencePaths() []string {
	var files []string

	if p := firstExistingFile(
		"artifacts/backend-recovery/ci-artifacts/ios27-runtime-poc/ios27-shortcuts-runtime-poc-artifacts/runtime-evidence.json",
		"artifacts/runtime-evidence.json",
	); p != "" {
		files = append(files, p)
	}

	if p := firstExistingFile(
		"artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios-test-evidence.json",
		"artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios/ios-test-evidence.json",
		"artifacts/backend-recovery/ci-artifacts/ios-build/ios-build-evidence/ios/.build/ios-test-evidence.json",
		"ios/ios-test-evidence.json",
	); p != "" {
		files = append(files, p)
	}

	if p := firstExistingFile(
		"artifacts/backend-recovery/ci-artifacts/openminis-skill/Cherri-OpenMinis-Skill/skill-evidence.json",
		"artifacts/backend-recovery/ci-artifacts/openminis-skill/openminis-skill-evidence/skill-evidence.json",
		"artifacts/evidence/skill-evidence.json",
	); p != "" {
		files = append(files, p)
	}

	for _, dir := range []string{"artifacts/backend-recovery/evidence"} {
		entries, err := os.ReadDir(dir)
		if err == nil {
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
					files = append(files, filepath.Join(dir, entry.Name()))
				}
			}
		}
	}
	return files
}

func BuildFinalEvidenceManifest(contracts *ContractsSet, implSHA, docsSHA, outPath string) (*EvidenceManifest, error) {
	if contracts == nil || contracts.Acceptance == nil || contracts.Repair == nil || contracts.Gates == nil {
		return nil, fmt.Errorf("all contracts must be loaded to generate final evidence manifest")
	}

	runs, err := ciRunDiscoveryFunc(implSHA)
	if err != nil {
		return nil, err
	}

	manifest := &EvidenceManifest{
		SchemaVersion:     "1",
		ImplementationSHA: implSHA,
		DocsSHA:           docsSHA,
		SchemaFingerprint: schema.DefaultRegistry().Fingerprint(),
		CodecFingerprint:  backend.CodecFingerprint(),
		Records:           make([]EvidenceRecord, 0, 250),
		Runs:              runs,
	}

	candidateEvidenceFiles := defaultEvidencePathsFunc()
	seenFiles := make(map[string]bool)
	for _, f := range candidateEvidenceFiles {
		cleanPath := filepath.Clean(f)
		if seenFiles[cleanPath] {
			continue
		}
		seenFiles[cleanPath] = true
		recs, err := ingestEvidenceFile(cleanPath)
		if err == nil {
			manifest.Records = append(manifest.Records, recs...)
		}
	}

	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal evidence manifest: %w", err)
	}

	if outPath != "" {
		if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
			return nil, fmt.Errorf("failed to create directory for %s: %w", outPath, err)
		}
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write evidence manifest to %s: %w", outPath, err)
		}
	}

	return manifest, nil
}

