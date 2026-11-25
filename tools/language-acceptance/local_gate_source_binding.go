package main

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"
)

var localGateSourceCache sync.Map

// These gate inventories exceed one immutable Git read after adding the shared
// main-output dependencies. Bound the whole set before reading any blob bytes;
// splitting the request must not multiply the existing resource limits.
func localGateSourceFiles(repository, commit string, names []string) ([]editorSourceFile, error) {
	return localGateSourceFilesBounded(repository, commit, names, 128)
}

// Only an explicit complete multi-scenario producer may opt into 256 witnesses.
// The total 16MiB and per-object 4MiB preflight bounds remain unchanged.
func localGateSourceFilesBounded(repository, commit string, names []string, witnessLimit int) ([]editorSourceFile, error) {
	if (witnessLimit != 128 && witnessLimit != 256) || !documentationHash(commit, 40) || len(names) == 0 || len(names) > witnessLimit {
		return nil, fmt.Errorf("invalid bounded local-gate source request")
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return nil, err
	}
	var requests strings.Builder
	seen := map[string]bool{}
	for _, name := range names {
		if name == "." || name == ".." || path.Clean(name) != name || path.IsAbs(name) ||
			strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\x00\r\n\\:") || seen[name] {
			return nil, fmt.Errorf("invalid bounded local-gate source path: %q", name)
		}
		seen[name] = true
		requests.WriteString(commit + ":" + name + "\n")
	}
	key := root + "\x00" + requests.String()
	if cached, found := localGateSourceCache.Load(key); found {
		return append([]editorSourceFile(nil), cached.([]editorSourceFile)...), nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "git", "--no-replace-objects", "-C", root, "cat-file", "--batch-check")
	command.Stdin = strings.NewReader(requests.String())
	command.WaitDelay = 2 * time.Second
	var output editorGitSourceOutput
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = &output, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("bounded immutable source size preflight unavailable: %w", err)
	}
	if err := localGateSourceBudgetBounded(output.Bytes(), len(names), witnessLimit); err != nil {
		return nil, err
	}
	var files []editorSourceFile
	for start := 0; start < len(names); start += 64 {
		end := start + 64
		if end > len(names) {
			end = len(names)
		}
		batch, err := editorGitSourceFiles(root, commit, names[start:end])
		if err != nil {
			return nil, err
		}
		files = append(files, batch...)
	}
	localGateSourceCache.Store(key, append([]editorSourceFile(nil), files...))
	return files, nil
}

func localGateSourceBudget(raw []byte, required int) error {
	return localGateSourceBudgetBounded(raw, required, 128)
}

func localGateSourceBudgetBounded(raw []byte, required, witnessLimit int) error {
	if (witnessLimit != 128 && witnessLimit != 256) || required < 1 || required > witnessLimit || len(raw) == 0 || raw[len(raw)-1] != '\n' {
		return fmt.Errorf("invalid bounded local-gate size preflight")
	}
	lines := bytes.Split(raw[:len(raw)-1], []byte("\n"))
	if len(lines) != required {
		return fmt.Errorf("incomplete bounded local-gate source inventory")
	}
	var total int64
	for _, line := range lines {
		header := strings.Fields(string(line))
		if len(header) != 3 || !documentationHash(header[0], 40) || header[1] != "blob" {
			return fmt.Errorf("invalid bounded local-gate immutable source object")
		}
		size, err := strconv.ParseInt(header[2], 10, 64)
		if err != nil || size < 0 || size > 4<<20 {
			return fmt.Errorf("invalid bounded local-gate source object size")
		}
		total += int64(len(line)) + 1 + size + 1
		if total > 16<<20 {
			return fmt.Errorf("bounded local-gate source aggregate exceeds 16MiB")
		}
	}
	return nil
}

func localGateExactSourceFiles(commit string, recorded []editorSourceFile, names []string) error {
	return localGateExactSourceFilesBounded(commit, recorded, names, 128)
}
func localGateExactSourceFilesBounded(commit string, recorded []editorSourceFile, names []string, witnessLimit int) error {
	immutable, err := localGateSourceFilesBounded(resolveRepoRoot(), commit, names, witnessLimit)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(recorded, immutable) {
		return fmt.Errorf("bounded local-gate immutable source manifest mismatch")
	}
	return nil
}
