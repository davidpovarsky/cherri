package assets

import (
	"bytes"
	"encoding/base64"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/language/workspace"
)

func writeAsset(t *testing.T, path string, data []byte, version int) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Unix(1700000000+int64(version), 0)
	if err := os.Chtimes(path, stamp, stamp); err != nil {
		t.Fatal(err)
	}
}

func readAsset(t *testing.T, scope *Scope, from, path string, expected []byte) Snapshot {
	t.Helper()
	result, err := scope.Read(from, path)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(result.Base64)
	if err != nil || !bytes.Equal(decoded, expected) || result.Bytes != int64(len(expected)) {
		t.Fatal("cached canonical encoding changed file bytes")
	}
	return result
}

func TestAssetCacheReuseAndExplicitInvalidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
		kind string
	}{
		{"utf8_text", bytes.Repeat([]byte("public-שלום\n"), 20000), "text"},
		{"binary", bytes.Repeat([]byte{0, 1, 255, 128, 10}, 40000), "binary"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "payload.dat")
			writeAsset(t, path, tc.data, 1)
			cache := New(0, 0, 0)
			scope, err := OpenScope(filepath.Join(dir, "entry.cherri"), cache)
			if err != nil {
				t.Fatal(err)
			}
			defer scope.Close()
			first := readAsset(t, scope, "", "payload.dat", tc.data)
			for n := 0; n < 25; n++ {
				next := readAsset(t, scope, "sub/helper.cherri", "../payload.dat", tc.data)
				if next.SHA256 != first.SHA256 || next.Kind != tc.kind {
					t.Fatal("cache identity changed")
				}
			}
			metrics := cache.Metrics()
			if metrics.Reads != 1 || metrics.Encodes != 1 || metrics.ReadBytes != uint64(len(tc.data)) || metrics.Hits != 25 {
				t.Fatalf("unrelated reads repeated heavy work: %+v", metrics)
			}
			// Causal control: the same production read with explicit
			// invalidation must perform new I/O/encoding, not report fake hits.
			for n := 0; n < 3; n++ {
				cache.Invalidate()
				readAsset(t, scope, "", "payload.dat", tc.data)
			}
			metrics = cache.Metrics()
			if metrics.Reads != 4 || metrics.Encodes != 4 || metrics.Invalidations != 3 {
				t.Fatalf("invalidation did not cause actual heavy work: %+v", metrics)
			}
		})
	}
}

func TestAssetCacheDetectsChangesMissingAndReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "payload.dat")
	cache := New(0, 0, 0)
	scope, err := OpenScope(filepath.Join(dir, "entry.cherri"), cache)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	original := []byte("PUBLIC_INITIAL")
	writeAsset(t, path, original, 1)
	first := readAsset(t, scope, "", "payload.dat", original)
	changed := []byte("PUBLIC_CHANGED")
	writeAsset(t, path, changed, 2)
	second := readAsset(t, scope, "", "payload.dat", changed)
	if first.SHA256 == second.SHA256 || cache.Metrics().Reads != 2 {
		t.Fatal("mtime change reused old bytes")
	}

	// A real replacement with the same timestamp/size has a new file identity.
	replacement := []byte("PUBLIC_REPLACE")
	swap := filepath.Join(dir, "swap.dat")
	writeAsset(t, swap, replacement, 2)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(swap, path); err != nil {
		t.Fatal(err)
	}
	readAsset(t, scope, "", "payload.dat", replacement)
	if cache.Metrics().Reads != 3 {
		t.Fatal("replacement identity reused old bytes")
	}

	// Ordinary size changes also invalidate even if the timestamp is retained.
	larger := append(append([]byte(nil), replacement...), []byte("_LARGER")...)
	writeAsset(t, path, larger, 2)
	readAsset(t, scope, "", "payload.dat", larger)
	if cache.Metrics().Reads != 4 {
		t.Fatal("size change reused old bytes")
	}

	// Explicit watcher events cover deliberate same-size, same-time rewrites.
	preserved := bytes.Repeat([]byte("Z"), len(larger))
	writeAsset(t, path, preserved, 2)
	cache.Invalidate()
	readAsset(t, scope, "", "payload.dat", preserved)
	if cache.Metrics().Reads != 5 {
		t.Fatal("watcher invalidation reused old bytes")
	}

	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Read("", "payload.dat"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("missing file suppressed: %v", err)
	}
	if cache.Metrics().Entries != 0 {
		t.Fatal("missing dependency remained cached")
	}
	writeAsset(t, path, original, 3)
	readAsset(t, scope, "", "payload.dat", original)
	if cache.Metrics().Reads != 6 {
		t.Fatal("recreated dependency reused deleted bytes")
	}
}

func TestAssetCacheBoundsAndWorkspaceIsolation(t *testing.T) {
	dir := t.TempDir()
	cache := New(12, 16, 1)
	scope, err := OpenScope(filepath.Join(dir, "entry.cherri"), cache)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	one, two := []byte("PUBLIC_FIRST"), []byte("PUBLIC_OTHER")
	writeAsset(t, filepath.Join(dir, "one.dat"), one, 1)
	writeAsset(t, filepath.Join(dir, "two.dat"), two, 1)
	readAsset(t, scope, "", "one.dat", one)
	readAsset(t, scope, "", "two.dat", two)
	readAsset(t, scope, "", "one.dat", one)
	metrics := cache.Metrics()
	if metrics.Entries != 1 || metrics.ResidentBytes > 16 || metrics.Reads != 3 || metrics.Evictions != 2 {
		t.Fatalf("cache bounds or eviction did not cause real rereads: %+v", metrics)
	}
	writeAsset(t, filepath.Join(dir, "large.dat"), bytes.Repeat([]byte{1}, 13), 1)
	if _, err := scope.Read("", "large.dat"); !errors.Is(err, ErrTooLarge) {
		t.Fatal("file byte limit not enforced")
	}
	if cache.Metrics().Reads != 3 {
		t.Fatal("oversize metadata rejection still read file content")
	}
	writeAsset(t, filepath.Join(dir, "empty.dat"), nil, 1)
	readAsset(t, scope, "", "empty.dat", nil)

	other := t.TempDir()
	writeAsset(t, filepath.Join(other, "one.dat"), two, 1)
	otherScope, err := OpenScope(filepath.Join(other, "entry.cherri"), cache)
	if err != nil {
		t.Fatal(err)
	}
	defer otherScope.Close()
	readAsset(t, otherScope, "", "one.dat", two)
	readAsset(t, scope, "", "one.dat", one)
}

func TestAssetCacheReauthorizesCachedPath(t *testing.T) {
	parent := t.TempDir()
	inside, outside := filepath.Join(parent, "inside"), filepath.Join(parent, "outside")
	cachedDir := filepath.Join(inside, "resource")
	writeAsset(t, filepath.Join(cachedDir, "payload.dat"), []byte("PUBLIC_ALLOWED"), 1)
	writeAsset(t, filepath.Join(outside, "payload.dat"), []byte("PUBLIC_DENIED"), 1)
	cache := New(0, 0, 0)
	scope, err := OpenScope(filepath.Join(inside, "entry.cherri"), cache)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	readAsset(t, scope, "", "resource/payload.dat", []byte("PUBLIC_ALLOWED"))
	if err := os.Remove(filepath.Join(cachedDir, "payload.dat")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(cachedDir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, cachedDir); err != nil {
		if runtime.GOOS != "windows" {
			t.Fatal(err)
		}
		out, err := exec.Command("cmd.exe", "/c", "mklink", "/J", cachedDir, outside).CombinedOutput()
		if err != nil {
			t.Fatalf("real junction control unavailable: %v %s", err, out)
		}
	}
	if _, err := scope.Read("", "resource/payload.dat"); !errors.Is(err, workspace.ErrAccess) {
		t.Fatalf("cached path bypassed rooted authorization: %v", err)
	}
	if cache.Metrics().Reads != 1 || cache.Metrics().Entries != 0 {
		t.Fatal("escaped file was read or old entry survived")
	}
	for _, path := range []string{"../outside/payload.dat", "https://example.invalid/payload.dat", filepath.Join(outside, "payload.dat")} {
		if _, err := scope.Read("", path); err == nil {
			t.Fatal("unauthorized path accepted")
		}
	}
}

func TestAssetCacheConcurrentReadersShareOneEncoding(t *testing.T) {
	dir := t.TempDir()
	data := bytes.Repeat([]byte{0, 255, 'A'}, 100000)
	writeAsset(t, filepath.Join(dir, "payload.dat"), data, 1)
	cache := New(0, 0, 0)
	scope, err := OpenScope(filepath.Join(dir, "entry.cherri"), cache)
	if err != nil {
		t.Fatal(err)
	}
	defer scope.Close()
	var wait sync.WaitGroup
	errors := make(chan error, 16)
	for n := 0; n < 16; n++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			snapshot, err := scope.Read("", "payload.dat")
			if err == nil && snapshot.Base64 != base64.StdEncoding.EncodeToString(data) {
				err = ErrChanged
			}
			errors <- err
		}()
	}
	wait.Wait()
	close(errors)
	for err := range errors {
		if err != nil {
			t.Fatal(err)
		}
	}
	metrics := cache.Metrics()
	if metrics.Reads != 1 || metrics.Encodes != 1 || metrics.Hits != 15 {
		t.Fatalf("concurrent readers repeated heavy work: %+v", metrics)
	}
}
