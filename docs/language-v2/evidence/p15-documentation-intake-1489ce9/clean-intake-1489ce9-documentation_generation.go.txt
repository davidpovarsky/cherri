package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const documentationGeneratorPath = "ios/scripts/build_documentation_index.py"

func documentationPython() (string, error) {
	if configured := os.Getenv("CHERRI_D03_PYTHON"); configured != "" {
		if path, err := exec.LookPath(configured); err == nil {
			return path, nil
		}
		return "", fmt.Errorf("configured D03 Python interpreter is unavailable")
	}
	for _, candidate := range []string{"python3", "python"} {
		if path, err := exec.LookPath(candidate); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("canonical documentation verification requires Python")
}

// The canonical generator comes from the same immutable K that ran the app.
// Its pure helper consumes the exact four D Git blobs already checked above;
// declared source hashes alone cannot authorize invented or relabeled content.
// This adapter deliberately contains no Markdown parser or second serializer.
func canonicalDocumentationIndex(repository, implementation string, manifest []byte, sources map[string][]byte) ([]byte, error) {
	var generator, sourceErr = documentationGitBlob(repository, implementation, documentationGeneratorPath)
	if sourceErr != nil {
		return nil, fmt.Errorf("canonical documentation generator is unavailable at K: %w", sourceErr)
	}
	if len(sources) != len(documentationSourcePaths) {
		return nil, fmt.Errorf("canonical documentation verification needs exactly four D blobs")
	}
	var python, pythonErr = documentationPython()
	if pythonErr != nil {
		return nil, pythonErr
	}
	var directory, directoryErr = os.MkdirTemp("", "cherri-d03-canonical-")
	if directoryErr != nil {
		return nil, fmt.Errorf("canonical documentation temporary directory: %w", directoryErr)
	}
	defer os.RemoveAll(directory)
	var inputs = map[string][]byte{
		"build_documentation_index.py": generator,
		"manifest.json":                manifest,
	}
	for _, name := range documentationSourcePaths {
		var data, present = sources[name]
		if !present || len(data) > 4*1024*1024 {
			return nil, fmt.Errorf("canonical documentation input is absent or oversized: %s", name)
		}
		inputs[filepath.Join("sources", filepath.FromSlash(name))] = data
	}
	for name, data := range inputs {
		var destination = filepath.Join(directory, name)
		if err := os.MkdirAll(filepath.Dir(destination), 0700); err != nil {
			return nil, fmt.Errorf("canonical documentation input directory: %w", err)
		}
		if err := os.WriteFile(destination, data, 0600); err != nil {
			return nil, fmt.Errorf("canonical documentation input: %w", err)
		}
	}
	const adapter = `import json, runpy, sys
from pathlib import Path
root = Path(sys.argv[1])
canonical = runpy.run_path(str(root / "build_documentation_index.py"))
manifest = canonical["read_manifest"](root / "manifest.json")
sources = {name: (root / "sources" / name).read_bytes() for name in canonical["SOURCES"]}
index = canonical["build_index_from_sources"](sources, manifest)
(root / "canonical-index.json").write_bytes(
    (json.dumps(index, ensure_ascii=False, indent=2) + "\n").encode("utf-8"))
`
	var ctx, cancel = context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, python, "-I", "-B", "-c", adapter, directory)
	command.Dir = directory
	command.WaitDelay = 2 * time.Second
	var output, runErr = command.CombinedOutput()
	if runErr != nil {
		if len(output) > 1000 {
			output = output[len(output)-1000:]
		}
		return nil, fmt.Errorf("canonical documentation regeneration failed: %v: %s", runErr, strings.TrimSpace(string(output)))
	}
	var generatedPath = filepath.Join(directory, "canonical-index.json")
	var info, statErr = os.Stat(generatedPath)
	if statErr != nil || !info.Mode().IsRegular() || info.Size() > 2_000_000 {
		return nil, fmt.Errorf("canonical documentation regeneration produced no bounded bundle")
	}
	var generated, readErr = os.ReadFile(generatedPath)
	if readErr != nil {
		return nil, fmt.Errorf("canonical documentation regeneration output: %w", readErr)
	}
	return generated, nil
}
