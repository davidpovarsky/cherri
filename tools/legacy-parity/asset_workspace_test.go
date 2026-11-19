package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

func TestGate_MenusAssetWorkspace(t *testing.T) {
	var repoRoot = findRepoRoot()
	var fixture = filepath.Join(repoRoot, "tests", "menus.cherri")
	var originalAsset = filepath.Join(repoRoot, "assets", "cherri_icon.png")
	var originals = map[string][]byte{}
	for _, path := range []string{fixture, originalAsset} {
		var data, err = os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		originals[path] = data
	}
	t.Cleanup(func() {
		for path, before := range originals {
			var after, err = os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Errorf("canonical fixture dependency changed: %s: %v", path, err)
			}
		}
	})
	var binary, err = ensureCompilerBinary()
	if err != nil {
		t.Fatal(err)
	}
	var workspace = t.TempDir()
	var result = testFixture(binary, fixture, workspace, repoRoot)
	if !result.Equal || (result.Status != "VERIFIED_PARITY" && result.Status != "PROVEN_EQUIVALENT") {
		t.Fatalf("unchanged menus fixture failed in differential runner: %s: %s %+v", result.Status, result.Detail, result.Differences)
	}
	var stagedAsset = filepath.Join(workspace, "assets", "cherri_icon.png")
	var staged, readErr = os.ReadFile(stagedAsset)
	if readErr != nil || !bytes.Equal(staged, originals[originalAsset]) {
		t.Fatalf("runner did not preserve the original public asset bytes: %v", readErr)
	}

	var decoyCWD = t.TempDir()
	var decoyAsset = filepath.Join(decoyCWD, "assets", "cherri_icon.png")
	if err := os.MkdirAll(filepath.Dir(decoyAsset), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(decoyAsset, []byte("UNRELATED_CWD_ASSET"), 0600); err != nil {
		t.Fatal(err)
	}
	var compile = func(name string) (string, []byte, error) {
		var output = filepath.Join(workspace, name+".shortcut")
		var cmd = exec.Command(binary, "compile", filepath.Join(workspace, "menus_v2.cherri"), "--skip-sign", "-o", output)
		cmd.Dir = decoyCWD
		var data, err = cmd.CombinedOutput()
		return output, data, err
	}
	var assertParity = func(t *testing.T, name string) {
		t.Helper()
		var output, data, err = compile(name)
		if err != nil {
			t.Fatalf("isolated compile: %v\n%s", err, data)
		}
		var comparison, compareErr = shortcutcompare.CompareFilesNormalized(filepath.Join(workspace, "menus_legacy.shortcut"), output)
		if compareErr != nil || !comparison.Equal {
			t.Fatalf("canonical asset semantics changed: %v %+v", compareErr, comparison)
		}
	}
	t.Run("entry_asset_wins_over_decoy_cwd", func(t *testing.T) {
		assertParity(t, "decoy-cwd")
	})
	t.Run("missing_entry_asset_fails_closed", func(t *testing.T) {
		if err := os.Remove(stagedAsset); err != nil {
			t.Fatal(err)
		}
		var output, data, err = compile("missing")
		if err == nil || !strings.Contains(string(data), "E_WORKSPACE_ACCESS") || !strings.Contains(string(data), "file does not exist") {
			t.Fatalf("missing entry asset did not fail closed: %v\n%s", err, data)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("missing dependency emitted output: %v", err)
		}
	})
	t.Run("restored_exact_asset_recovers", func(t *testing.T) {
		if err := os.WriteFile(stagedAsset, originals[originalAsset], 0600); err != nil {
			t.Fatal(err)
		}
		assertParity(t, "restored")
	})
}

func TestGate_MenusMissingCanonicalAssetFailsClosed(t *testing.T) {
	var workspace = t.TempDir()
	var emptyRepository = t.TempDir()
	if err := stageFixtureAssets("menus.cherri", workspace, emptyRepository); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing canonical dependency did not preserve the missing-file error: %v", err)
	}
	if _, err := os.Stat(filepath.Join(workspace, "assets", "cherri_icon.png")); !os.IsNotExist(err) {
		t.Fatalf("missing dependency unexpectedly staged an artifact: %v", err)
	}
}
