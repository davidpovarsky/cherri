package main

import (
	"fmt"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

func TestLocalGateMainOutputDependencies(t *testing.T) {
	required := []string{
		"internal/language/analysis/analyzer.go",
		"internal/language/analysis/diagnostics.go",
		"internal/language/backend/contracts.go",
		"internal/language/lower/lower.go",
		"internal/language/lower/main_output.go",
		"internal/language/lower/module_scope.go",
		"internal/language/migrate/main_output.go",
		"internal/language/migrate/migrate.go",
		"internal/language/service/module_analysis.go",
		"internal/language/syntax/main_output.go",
	}
	for id, paths := range map[string][]string{
		"BRG18": nativePathsProductionPaths,
		"BRG21": purePublicProductionPaths,
		"RP52":  publicFrontendProductionPaths,
		"BRG30": packagedSkillProductionPaths,
		"M02":   mainOutputProductionPaths,
	} {
		t.Run(id, func(t *testing.T) {
			seen := map[string]bool{}
			for _, name := range paths {
				if seen[name] {
					t.Fatal("duplicate immutable source path", name)
				}
				seen[name] = true
			}
			for _, name := range required {
				if !seen[name] {
					t.Fatal("main-output dependency missing from source proof", name)
				}
			}
			if id == "RP52" && !seen["p15_main_void_return_test.go"] {
				t.Fatal("public graph oracle lost its main-boundary dependency")
			}
			if id != "M02" && id != "BRG30" {
				for _, name := range []string{
					"tools/language-acceptance/local_gate_source_binding.go",
					"tools/language-acceptance/local_gate_source_binding_test.go",
				} {
					if !seen[name] {
						t.Fatal("bounded source acquisition is not source-bound", name)
					}
				}
			}
		})
	}
}

func TestLocalGateSourceBudgetControls(t *testing.T) {
	line := func(size int64) string { return strings.Repeat("a", 40) + " blob " + fmt.Sprint(size) + "\n" }
	valid := strings.Repeat(line(10), 73)
	if err := localGateSourceBudget([]byte(valid), 73); err != nil {
		t.Fatal("valid two-batch preflight rejected", err)
	}
	for name, input := range map[string]struct {
		raw   string
		count int
	}{
		"empty":                    {"", 73},
		"missing_tail":             {strings.Repeat(line(10), 72), 73},
		"unexpected_tail":          {valid + line(10), 73},
		"unterminated":             {strings.TrimSuffix(valid, "\n"), 73},
		"too_many_objects":         {strings.Repeat(line(10), 129), 129},
		"non_blob":                 {strings.Replace(valid, " blob ", " tree ", 1), 73},
		"malformed_hash":           {strings.Replace(valid, strings.Repeat("a", 40), strings.Repeat("x", 40), 1), 73},
		"negative_size":            {strings.Replace(valid, " blob 10", " blob -1", 1), 73},
		"oversized_object":         {line((4 << 20) + 1), 1},
		"aggregate_before_batches": {strings.Repeat(line(4<<20), 4) + strings.Repeat(line(0), 69), 73},
		"unknown_git_object":       {strings.Repeat(line(10), 72) + "missing missing\n", 73},
	} {
		t.Run(name, func(t *testing.T) {
			if err := localGateSourceBudget([]byte(input.raw), input.count); err == nil {
				t.Fatal("invalid whole-set preflight accepted")
			}
		})
	}
}

func TestLocalGateImmutableSourceBatches(t *testing.T) {
	root := resolveRepoRoot()
	rawCommit, err := exec.Command("git", "--no-replace-objects", "-C", root, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatal(err)
	}
	commit := strings.TrimSpace(string(rawCommit))
	names := append([]string(nil), publicFrontendProductionPaths...)
	if len(names) <= 64 {
		t.Fatal("control does not cross the real immutable-read boundary")
	}
	files, err := localGateSourceFiles(root, commit, names)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != len(names) {
		t.Fatal("bounded acquisition omitted a source object")
	}
	for _, index := range []int{0, 63, 64, len(names) - 1} {
		raw, err := exec.Command("git", "--no-replace-objects", "-C", root, "show", commit+":"+names[index]).Output()
		if err != nil {
			t.Fatal(err)
		}
		if files[index].Path != names[index] || files[index].SHA256 != editorSHA(raw) {
			t.Fatal("actual immutable blob differs across batch boundary", index)
		}
	}
	if err := localGateExactSourceFiles(commit, files, names); err != nil {
		t.Fatal("actual source witness rejected", err)
	}
	original := append([]editorSourceFile(nil), files...)
	files[64].SHA256 = strings.Repeat("0", 64)
	if err := localGateExactSourceFiles(commit, files, names); err == nil {
		t.Fatal("second-batch counterfeit source digest accepted")
	}
	again, err := localGateSourceFiles(root, commit, names)
	if err != nil || !reflect.DeepEqual(again, original) {
		t.Fatal("caller mutation contaminated immutable source cache", err)
	}
	for name, mutate := range map[string]func([]string){
		"cross_batch_duplicate": func(paths []string) { paths[64] = paths[0] },
		"traversal":             func(paths []string) { paths[64] = "../main.go" },
		"absolute":              func(paths []string) { paths[64] = "/main.go" },
		"query_injection":       func(paths []string) { paths[64] = "main.go\n" + commit + ":go.mod" },
		"windows_separator":     func(paths []string) { paths[64] = "internal\\language\\lower\\lower.go" },
		"revision_separator":    func(paths []string) { paths[64] = "HEAD:main.go" },
		"missing_second_batch":  func(paths []string) { paths[64] = "p15-no-such-source-object.go" },
	} {
		t.Run(name, func(t *testing.T) {
			paths := append([]string(nil), names...)
			mutate(paths)
			if _, err := localGateSourceFiles(root, commit, paths); err == nil {
				t.Fatal("invalid bounded immutable source request accepted")
			}
		})
	}
}
