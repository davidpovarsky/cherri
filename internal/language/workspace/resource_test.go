package workspace

import (
	"errors"
	"io"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, path, value string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(value), 0600); err != nil {
		t.Fatal(err)
	}
}

func linkFixture(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err == nil {
		return
	}
	if runtime.GOOS != "windows" {
		t.Fatal("required symlink fixture unavailable")
	}
	out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", link, target).CombinedOutput()
	if err != nil {
		t.Fatalf("required junction fixture unavailable: %v %s", err, out)
	}
}

func TestRootedExactResourceReads(t *testing.T) {
	container := t.TempDir()
	inside, outside := filepath.Join(container, "inside folder"), filepath.Join(container, "outside")
	entry := filepath.Join(inside, "entry.cherri")
	writeFixture(t, entry, "// entry")
	writeFixture(t, filepath.Join(inside, "asset.txt"), "PUBLIC_ALLOWED")
	writeFixture(t, filepath.Join(inside, "..safe", "asset.txt"), "PUBLIC_ALLOWED")
	writeFixture(t, filepath.Join(outside, "asset.txt"), "PUBLIC_DENIED")
	linkFixture(t, outside, filepath.Join(inside, "escape"))
	linkFixture(t, filepath.Join(inside, "..safe"), filepath.Join(inside, "internal-link"))
	targetInfo, err := os.Stat(filepath.Join(outside, "asset.txt"))
	if err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Stat(filepath.Join(inside, "escape", "asset.txt"))
	if err != nil || !os.SameFile(targetInfo, linkInfo) {
		t.Fatal("outside link did not resolve to the real target")
	}
	root, err := Open(entry)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()

	for _, tc := range []struct{ from, requested string }{
		{"entry.cherri", "./asset.txt"},
		{"sub/consumer.cherri", "../asset.txt"},
		{"entry.cherri", "./..safe/asset.txt"},
	} {
		t.Run("allowed_"+strings.ReplaceAll(tc.requested, "/", "_"), func(t *testing.T) {
			resource, err := root.Open(tc.from, tc.requested)
			if err != nil {
				t.Fatal(err)
			}
			defer resource.Close()
			actualInfo, err := resource.File.Stat()
			if err != nil || !os.SameFile(resource.Info, actualInfo) {
				t.Fatal("stat/read identity changed")
			}
			content, err := io.ReadAll(io.LimitReader(resource.File, 64))
			if err != nil || string(content) != "PUBLIC_ALLOWED" {
				t.Fatalf("wrong authorized bytes: %q %v", content, err)
			}
			if filepath.IsAbs(resource.RelativePath) || !resource.Info.Mode().IsRegular() {
				t.Fatal("resource identity is not a regular workspace-relative file")
			}
		})
	}
	for _, requested := range []string{"../outside/asset.txt", "./escape/asset.txt", "./internal-link/asset.txt", filepath.Join(outside, "asset.txt"), "https://example.invalid/asset.txt"} {
		t.Run("denied_"+strings.ReplaceAll(filepath.Base(requested), ":", "_"), func(t *testing.T) {
			resource, err := root.Open("entry.cherri", requested)
			if err == nil || resource != nil {
				t.Fatalf("unauthorized resource opened: %v", err)
			}
			if strings.Contains(err.Error(), outside) || strings.Contains(err.Error(), "PUBLIC_DENIED") {
				t.Fatal("resolved target information leaked")
			}
		})
	}
	t.Run("exact_path_has_no_extension_fallback", func(t *testing.T) {
		resource, err := root.Open("entry.cherri", "asset")
		if resource != nil || !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("missing exact resource inferred an extension: %v", err)
		}
	})
	t.Run("regular_file_required", func(t *testing.T) {
		resource, err := root.Open("entry.cherri", "..safe")
		if resource != nil || !errors.Is(err, ErrNotRegular) {
			t.Fatalf("directory accepted as resource: %v", err)
		}
	})
	t.Run("file_uri_and_resource_ownership", func(t *testing.T) {
		path := filepath.ToSlash(entry)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		uri := (&url.URL{Scheme: "file", Path: path}).String()
		uriRoot, err := Open(uri)
		if err != nil {
			t.Fatal(err)
		}
		resource, err := uriRoot.Open("entry.cherri", "asset.txt")
		if err != nil {
			t.Fatal(err)
		}
		if err := uriRoot.Close(); err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(resource.File)
		if err != nil || string(raw) != "PUBLIC_ALLOWED" {
			t.Fatal("closing the root invalidated the separately owned file handle")
		}
		if err := resource.Close(); err != nil {
			t.Fatal(err)
		}
		if _, err := resource.File.Read(make([]byte, 1)); err == nil {
			t.Fatal("closed resource remains readable")
		}
	})
}
