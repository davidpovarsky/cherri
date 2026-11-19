package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"howett.net/plist"
)

func p15WorkspaceLink(t *testing.T, destination, link string) {
	t.Helper()
	if err := os.Symlink(destination, link); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Fatal("required symlink control could not be created")
	}
	output, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, destination).CombinedOutput()
	if err != nil {
		t.Fatalf("required Windows junction control could not be created: %v %s", err, output)
	}
}

func p15ModuleWrite(t *testing.T, file, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(file), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte(text), 0600); err != nil {
		t.Fatal(err)
	}
}

func p15ModuleContains(t *testing.T, data []byte, marker string) bool {
	t.Helper()
	var document map[string]any
	if _, err := plist.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(encoded), marker)
}

func TestP15IM04ActualWorkspaceResourceBoundaries(t *testing.T) {
	container := t.TempDir()
	workspace := filepath.Join(container, "workspace")
	outside := filepath.Join(container, "workspace-other")
	mainPath := filepath.Join(workspace, "main.cherri")
	p15ModuleWrite(t, mainPath, "// public synthetic fixture\n")
	insideMarker, outsideMarker := "P15_ALLOWED_LOCAL_VALUE", "P15_SYNTHETIC_OUTSIDE_VALUE"
	module := func(marker string) string { return fmt.Sprintf("function message() -> Text { return %q }\n", marker) }
	p15ModuleWrite(t, filepath.Join(workspace, "local", "helper.cherri"), module(insideMarker))
	p15ModuleWrite(t, filepath.Join(workspace, "..safe", "helper.cherri"), module(insideMarker))
	p15ModuleWrite(t, filepath.Join(outside, "helper.cherri"), module(outsideMarker))
	escape := filepath.Join(workspace, "escape")
	p15WorkspaceLink(t, outside, escape)
	actual, err := os.Stat(filepath.Join(escape, "helper.cherri"))
	target, targetErr := os.Stat(filepath.Join(outside, "helper.cherri"))
	if err != nil || targetErr != nil || !os.SameFile(actual, target) {
		t.Fatal("escape fixture does not actually resolve to the outside file")
	}
	source := func(path string) string {
		return fmt.Sprintf("import %q as localModule\nlet value = localModule.message()\nshow(value)\n", filepath.ToSlash(path))
	}
	for _, path := range []string{"./local/helper.cherri", "./local/helper", "./..safe/helper.cherri"} {
		t.Run("allowed_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			data, err := CompileSourceToPlist(mainPath, source(path))
			if err != nil {
				t.Fatalf("valid local path was rejected: %v", err)
			}
			if !p15ModuleContains(t, data, insideMarker) || p15ModuleContains(t, data, outsideMarker) {
				t.Fatal("actual local module did not supply its own native value")
			}
		})
	}
	for _, path := range []string{"./escape/helper.cherri", "./escape/helper", "../workspace-other/helper.cherri"} {
		t.Run("reject_"+strings.ReplaceAll(path, "/", "_"), func(t *testing.T) {
			data, err := CompileSourceToPlist(mainPath, source(path))
			if err == nil || len(data) != 0 {
				t.Fatalf("unauthorized outside module compiled: error=%v, emitted_bytes=%d", err, len(data))
			}
			if !strings.Contains(err.Error(), "workspace") {
				t.Fatalf("missing workspace denial: %v", err)
			}
			if strings.Contains(err.Error(), outsideMarker) || strings.Contains(err.Error(), outside) {
				t.Fatalf("outside target content or resolved absolute path leaked: %v", err)
			}
		})
	}
	t.Run("missing_module", func(t *testing.T) {
		data, err := CompileSourceToPlist(mainPath, source("./missing.cherri"))
		if err == nil || len(data) != 0 || !strings.Contains(err.Error(), "cannot resolve module") {
			t.Fatalf("missing local dependency not diagnosed: %v", err)
		}
		if strings.Contains(err.Error(), container) {
			t.Fatalf("resolved host path leaked: %v", err)
		}
	})
}
