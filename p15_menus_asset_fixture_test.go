package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/migrate"
	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

// The original Legacy fixture resolves this public asset from repository CWD.
// Its migrated V2 entry lives in a new workspace, so that workspace needs the
// same dependency at the same relative path. No compiler path fallback is used.
func p15StageMenusAsset(t *testing.T, project string) []byte {
	t.Helper()
	original := filepath.Join("assets", "cherri_icon.png")
	data, err := os.ReadFile(original)
	if err != nil || len(data) == 0 {
		t.Fatalf("original public menus asset unavailable: %v", err)
	}
	t.Cleanup(func() {
		after, err := os.ReadFile(original)
		if err != nil || !bytes.Equal(after, data) {
			t.Errorf("original public menus asset changed: %v", err)
		}
	})
	target := filepath.Join(project, original)
	if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, data, 0600); err != nil {
		t.Fatal(err)
	}
	staged, err := os.ReadFile(target)
	if err != nil || !bytes.Equal(staged, data) {
		t.Fatalf("migrated workspace asset differs from original: %v", err)
	}
	return data
}

func TestP15MenusMigrationAssetWorkspace(t *testing.T) {
	fixture := filepath.Join("tests", "menus.cherri")
	original, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		after, err := os.ReadFile(fixture)
		if err != nil || !bytes.Equal(after, original) {
			t.Errorf("original menus source changed: %v", err)
		}
	})
	migrated, diagnostics, err := migrate.MigrateSourceWithDiagnostics(string(original))
	if err != nil || len(diagnostics) != 0 {
		t.Fatalf("actual original migration: %v %v", err, diagnostics)
	}
	project, decoyCWD := t.TempDir(), t.TempDir()
	source := filepath.Join(project, "menus_v2.cherri")
	if err := os.WriteFile(source, []byte(migrated), 0600); err != nil {
		t.Fatal(err)
	}
	asset := p15StageMenusAsset(t, project)
	decoy := filepath.Join(decoyCWD, "assets", "cherri_icon.png")
	if err := os.MkdirAll(filepath.Dir(decoy), 0700); err != nil {
		t.Fatal(err)
	}
	decoyBytes := append(append([]byte{}, asset...), []byte("P15_UNRELATED_CWD_ASSET")...)
	if err := os.WriteFile(decoy, decoyBytes, 0600); err != nil {
		t.Fatal(err)
	}
	binary := roundTripBinary(t)
	legacy := filepath.Join(project, "legacy.shortcut")
	stdout, stderr, err := p15RunCLI(t, binary, "", nil, fixture, "--legacy", "--skip-sign", "--load-all-actions", "-o", legacy)
	if err != nil {
		t.Fatalf("actual unchanged Legacy fixture: %v\n%s\n%s", err, stdout, stderr)
	}
	compileParity := func(t *testing.T, name string) {
		t.Helper()
		output := filepath.Join(project, name+".shortcut")
		stdout, stderr, err := p15RunCLI(t, binary, decoyCWD, nil, "compile", source, "--skip-sign", "-o", output)
		if err != nil {
			t.Fatalf("actual isolated V2 compile: %v\n%s\n%s", err, stdout, stderr)
		}
		comparison, err := shortcutcompare.CompareFilesNormalized(legacy, output)
		if err != nil || !comparison.Equal {
			t.Fatalf("unchanged original asset/native parity lost: %v %+v", err, comparison)
		}
	}
	t.Run("entry_asset_wins_over_different_cwd_asset", func(t *testing.T) {
		compileParity(t, "entry-asset")
	})
	assetPath := filepath.Join(project, "assets", "cherri_icon.png")
	t.Run("missing_entry_asset_rejected_despite_cwd_asset", func(t *testing.T) {
		if err := os.Remove(assetPath); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(project, "missing-asset.shortcut")
		stdout, stderr, err := p15RunCLI(t, binary, decoyCWD, nil, "compile", source, "--skip-sign", "-o", output)
		message := string(stdout) + string(stderr)
		if err == nil || !strings.Contains(message, "E_WORKSPACE_ACCESS") || !strings.Contains(message, "file does not exist") {
			t.Fatalf("missing dependency used unrelated CWD or failed for another reason: %v\n%s", err, message)
		}
		if _, err := os.Stat(output); !os.IsNotExist(err) {
			t.Fatalf("missing dependency emitted an artifact: %v", err)
		}
	})
	t.Run("restored_exact_entry_asset_recovers", func(t *testing.T) {
		if err := os.WriteFile(assetPath, asset, 0600); err != nil {
			t.Fatal(err)
		}
		compileParity(t, "restored-asset")
	})
}
