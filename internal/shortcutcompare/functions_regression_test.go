package shortcutcompare

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
)

func TestFunctionBodyMutation_DetectedByNormalizedComparator(t *testing.T) {
	absRepoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("failed to resolve repo root: %v", err)
	}
	functionsFixture := filepath.Join(absRepoRoot, "tests", "functions.cherri")
	originalSrc, err := os.ReadFile(functionsFixture)
	if err != nil {
		t.Fatalf("failed to read %s: %v", functionsFixture, err)
	}

	tmpDir := t.TempDir()
	binPath := filepath.Join(tmpDir, "cherri_test_bin.exe")
	buildCmd := exec.Command("go", "build", "-o", binPath, ".")
	buildCmd.Dir = absRepoRoot
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to build cherri binary: %v\n%s", err, string(out))
	}

	// 1. Compile original fixture with --legacy
	legacyOut := filepath.Join(tmpDir, "functions_legacy.shortcut")
	cmdLegacy := exec.Command(binPath, functionsFixture, "--legacy", "--skip-sign", "--load-all-actions", "-o", legacyOut)
	cmdLegacy.Dir = absRepoRoot
	if out, err := cmdLegacy.CombinedOutput(); err != nil {
		t.Fatalf("legacy compile failed: %v\n%s", err, string(out))
	}

	// 2. Unmutated fixture: MUST prove semantic equivalence under CompareFilesNormalized
	migratedOrig, diagsOrig, errOrig := migrate.MigrateSourceWithDiagnostics(string(originalSrc))
	if errOrig != nil || len(diagsOrig) > 0 {
		t.Fatalf("migration of original fixture failed: %v, diags: %v", errOrig, diagsOrig)
	}
	v2OrigSrcPath := filepath.Join(tmpDir, "functions_orig_v2.cherri")
	if err := os.WriteFile(v2OrigSrcPath, []byte(migratedOrig), 0644); err != nil {
		t.Fatalf("failed to write original v2 source: %v", err)
	}
	v2OrigOut := filepath.Join(tmpDir, "functions_orig_v2.shortcut")
	cmdV2Orig := exec.Command(binPath, v2OrigSrcPath, "--skip-sign", "-o", v2OrigOut)
	cmdV2Orig.Dir = absRepoRoot
	if out, err := cmdV2Orig.CombinedOutput(); err != nil {
		t.Fatalf("v2 compile of original source failed: %v\n%s", err, string(out))
	}

	normCmpOrig, errNormOrig := CompareFilesNormalized(legacyOut, v2OrigOut)
	if errNormOrig != nil {
		t.Fatalf("CompareFilesNormalized on unmutated fixture failed with error: %v", errNormOrig)
	}
	docLegacy, _ := LoadShortcutDocument(legacyOut)
	actionsLegacy, _ := docLegacy["WFWorkflowActions"].([]any)
	funcsLegacy, _ := parseFunctionsLegacy(actionsLegacy)
	var legacyFnNames []string
	for k := range funcsLegacy {
		legacyFnNames = append(legacyFnNames, k)
	}
	t.Logf("Legacy function names: %v", legacyFnNames)

	docV2, _ := LoadShortcutDocument(v2OrigOut)
	actionsV2, _ := docV2["WFWorkflowActions"].([]any)
	funcsV2, _ := parseFunctionsV2(actionsV2)
	var v2FnNames []string
	for k := range funcsV2 {
		v2FnNames = append(v2FnNames, k)
	}
	t.Logf("Legacy total actions: %d, V2 total actions: %d", len(actionsLegacy), len(actionsV2))

	for i, d := range normCmpOrig.Differences {
		if i < 15 {
			t.Logf("diff %d: path=%s kind=%s A=%v B=%v", i, d.Path, d.Kind, d.A, d.B)
		}
	}

	if !normCmpOrig.Equal {
		t.Fatalf("unmutated functions fixture failed semantic equivalence! Diffs: %v", normCmpOrig.Differences)
	}

	// 3. Table of mutations: EVERY mutation MUST be detected (must NOT report false positive equivalence)
	mutations := []struct {
		name        string
		origText    string
		mutatedText string
	}{
		{
			name:        "greetDefault_default_value",
			origText:    `text salutation = "Hello"`,
			mutatedText: `text salutation = "Goodbye"`,
		},
		{
			name:        "double_body_math_operator",
			origText:    `const result = @value * 2`,
			mutatedText: `const result = @value * 3`,
		},
		{
			name:        "fibonacci_body_condition_threshold",
			origText:    `if @n <= 1 {`,
			mutatedText: `if @n <= 2 {`,
		},
	}

	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			mutatedSrc := strings.Replace(string(originalSrc), tc.origText, tc.mutatedText, 1)
			if mutatedSrc == string(originalSrc) {
				t.Fatalf("mutation pattern %q not found in fixture", tc.origText)
			}

			migrated, diags, errMig := migrate.MigrateSourceWithDiagnostics(mutatedSrc)
			if errMig != nil || len(diags) > 0 {
				t.Fatalf("migration failed: %v, diags: %v", errMig, diags)
			}

			v2MutSrcPath := filepath.Join(tmpDir, tc.name+".cherri")
			if err := os.WriteFile(v2MutSrcPath, []byte(migrated), 0644); err != nil {
				t.Fatalf("failed to write mutated v2 source: %v", err)
			}

			v2MutOut := filepath.Join(tmpDir, tc.name+".shortcut")
			cmdV2Mut := exec.Command(binPath, v2MutSrcPath, "--skip-sign", "-o", v2MutOut)
			cmdV2Mut.Dir = absRepoRoot
			if out, err := cmdV2Mut.CombinedOutput(); err != nil {
				t.Fatalf("v2 compile failed: %v\n%s", err, string(out))
			}

			// CompareFilesNormalized MUST detect difference
			normCmp, errNorm := CompareFilesNormalized(legacyOut, v2MutOut)
			if errNorm != nil {
				t.Fatalf("CompareFilesNormalized error: %v", errNorm)
			}
			if normCmp.Equal {
				t.Fatalf("CRITICAL BUG: CompareFilesNormalized declared false positive equivalence for mutation %s (%s -> %s)!",
					tc.name, tc.origText, tc.mutatedText)
			}
		})
	}
}
