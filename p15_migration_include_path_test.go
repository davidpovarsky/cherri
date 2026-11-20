package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func TestP15RP33SourceRelativeInclude(t *testing.T) {
	binary := p15AcceptanceCLI(t)
	for _, mode := range []string{"preview", "write"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			sourceDirectory, workingDirectory := filepath.Join(root, "source"), filepath.Join(root, "unrelated-cwd")
			for _, path := range []string{sourceDirectory, workingDirectory} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			original, helper := p15RP33FullProgram()
			decoy := "function rp33Greet(text value): text {\n    output(\"DECOY:{@value}\")\n}\n"
			input := filepath.Join(sourceDirectory, "original.cherri")
			for path, data := range map[string]string{input: original, filepath.Join(sourceDirectory, "helper.cherri"): helper, filepath.Join(workingDirectory, "helper.cherri"): decoy} {
				if err := os.WriteFile(path, []byte(data), 0600); err != nil {
					t.Fatal(err)
				}
			}
			beforePath := filepath.Join(sourceDirectory, "before.shortcut")
			stdout, stderr, err := p15RunCLI(t, binary, sourceDirectory, nil, input, "--legacy", "--skip-sign", "--output="+beforePath)
			if err != nil {
				t.Fatal("original source include failed", err, string(stdout), string(stderr))
			}
			args := []string{"migrate", input}
			if mode == "write" {
				args = append(args, "--write")
			}
			converted, stderr, err := p15RunCLI(t, binary, workingDirectory, nil, args...)
			if err != nil {
				t.Fatal("source-relative include migration failed", err, string(converted), string(stderr))
			}
			if mode == "write" {
				converted, err = os.ReadFile(input)
				if err != nil {
					t.Fatal(err)
				}
			} else if current, readErr := os.ReadFile(input); readErr != nil || !bytes.Equal(current, []byte(original)) {
				t.Fatal("preview changed the original", readErr)
			}
			if err := p15RP33CheckStructure(p15RP33Parse(t, string(converted))); err != nil {
				t.Errorf("working-directory decoy replaced the source dependency: %v", err)
			}
			afterSource := filepath.Join(sourceDirectory, "migrated.cherri")
			if err := os.WriteFile(afterSource, converted, 0600); err != nil {
				t.Fatal(err)
			}
			afterPath := filepath.Join(sourceDirectory, "after.shortcut")
			stdout, stderr, err = p15RunCLI(t, binary, workingDirectory, nil, "compile", afterSource, "--skip-sign", "--output="+afterPath)
			if err != nil {
				t.Fatal("migrated public V2 compile failed", err, string(stdout), string(stderr))
			}
			comparison, err := shortcutcompare.CompareFilesNormalized(beforePath, afterPath)
			if err != nil || !comparison.Equal {
				t.Errorf("relative include changed actual native meaning: error=%v differences=%v", err, comparison.Differences)
			}
			for path, expected := range map[string]string{filepath.Join(sourceDirectory, "helper.cherri"): helper, filepath.Join(workingDirectory, "helper.cherri"): decoy} {
				got, readErr := os.ReadFile(path)
				if readErr != nil || !bytes.Equal(got, []byte(expected)) {
					t.Error("migration changed an include or decoy", path, readErr)
				}
			}
		})
	}
}
