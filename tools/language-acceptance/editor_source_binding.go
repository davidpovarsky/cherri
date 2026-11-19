package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

var editorGitSourceCache sync.Map

type editorGitSourceOutput struct{ bytes.Buffer }

func (output *editorGitSourceOutput) Write(value []byte) (int, error) {
	if output.Len()+len(value) > 16<<20 {
		return 0, fmt.Errorf("immutable source object set exceeds limit")
	}
	return output.Buffer.Write(value)
}

// One Git process reads raw immutable blobs. Working-tree filters and a receipt's
// self-reported source hashes must not decide which source the proof belongs to.
func editorGitSourceFiles(repository, commit string, names []string) ([]editorSourceFile, error) {
	if !documentationHash(commit, 40) || len(names) == 0 || len(names) > 64 {
		return nil, fmt.Errorf("invalid immutable source request")
	}
	var root, err = filepath.Abs(repository)
	if err != nil {
		return nil, err
	}
	var requests strings.Builder
	var seen = make(map[string]bool)
	for _, name := range names {
		if name == "." || name == ".." || path.Clean(name) != name || path.IsAbs(name) ||
			strings.HasPrefix(name, "../") || strings.ContainsAny(name, "\x00\r\n\\:") || seen[name] {
			return nil, fmt.Errorf("invalid immutable source path: %q", name)
		}
		seen[name] = true
		requests.WriteString(commit + ":" + name + "\n")
	}
	var key = root + "\x00" + requests.String()
	if cached, found := editorGitSourceCache.Load(key); found {
		return append([]editorSourceFile(nil), cached.([]editorSourceFile)...), nil
	}
	var ctx, cancel = context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var command = exec.CommandContext(ctx, "git", "--no-replace-objects", "-C", root, "cat-file", "--batch")
	command.Stdin = strings.NewReader(requests.String())
	command.WaitDelay = 2 * time.Second
	var output editorGitSourceOutput
	var stderr bytes.Buffer
	command.Stdout, command.Stderr = &output, &stderr
	if err := command.Run(); err != nil {
		return nil, fmt.Errorf("immutable Git source unavailable: %v", err)
	}
	var reader = bufio.NewReader(bytes.NewReader(output.Bytes()))
	var files []editorSourceFile
	for _, name := range names {
		var line, err = reader.ReadString('\n')
		var header = strings.Fields(line)
		if err != nil || len(header) != 3 || !documentationHash(header[0], 40) || header[1] != "blob" {
			return nil, fmt.Errorf("immutable Git source blob unavailable: %s", name)
		}
		var size, sizeErr = strconv.ParseInt(header[2], 10, 64)
		if sizeErr != nil || size < 0 || size > 4<<20 {
			return nil, fmt.Errorf("immutable Git source blob size invalid: %s", name)
		}
		var data = make([]byte, int(size))
		if _, err := io.ReadFull(reader, data); err != nil {
			return nil, fmt.Errorf("immutable Git source blob truncated: %s", name)
		}
		if separator, err := reader.ReadByte(); err != nil || separator != '\n' {
			return nil, fmt.Errorf("immutable Git source blob boundary invalid: %s", name)
		}
		files = append(files, editorSourceFile{Path: name, SHA256: editorSHA(data)})
	}
	if _, err := reader.ReadByte(); err != io.EOF {
		return nil, fmt.Errorf("unexpected immutable Git source output")
	}
	editorGitSourceCache.Store(key, append([]editorSourceFile(nil), files...))
	return files, nil
}
