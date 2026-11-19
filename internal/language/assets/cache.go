// Package assets caches authorized local embedFile bytes and their canonical
// base64 representation. It does not decode or reinterpret text or binary data.
package assets

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/electrikmilk/cherri/internal/language/workspace"
)

const DefaultMaxFileBytes int64 = 64 << 20
const DefaultMaxCacheBytes int64 = 128 << 20
const DefaultMaxEntries = 64

var ErrTooLarge = errors.New("embedded asset exceeds the configured byte limit")
var ErrChanged = errors.New("embedded asset changed while it was being read")

type Snapshot struct {
	RelativePath string    `json:"relative_path"`
	Bytes        int64     `json:"bytes"`
	SHA256       string    `json:"sha256"`
	Kind         string    `json:"kind"`
	Base64       string    `json:"-"`
	Modified     time.Time `json:"modified"`
}

type Metrics struct {
	MetadataChecks uint64 `json:"metadata_checks"`
	Hits           uint64 `json:"hits"`
	Misses         uint64 `json:"misses"`
	Reads          uint64 `json:"reads"`
	ReadBytes      uint64 `json:"read_bytes"`
	Encodes        uint64 `json:"encodes"`
	Invalidations  uint64 `json:"invalidations"`
	Evictions      uint64 `json:"evictions"`
	ResidentBytes  int64  `json:"resident_bytes"`
	Entries        int    `json:"entries"`
}

type cached struct {
	snapshot Snapshot
	info     os.FileInfo
	used     uint64
}

type Cache struct {
	mu            sync.Mutex
	entries       map[string]*cached
	maxFileBytes  int64
	maxCacheBytes int64
	maxEntries    int
	clock         uint64
	metrics       Metrics
}

func New(maxFileBytes, maxCacheBytes int64, maxEntries int) *Cache {
	if maxFileBytes <= 0 {
		maxFileBytes = DefaultMaxFileBytes
	}
	if maxCacheBytes <= 0 {
		maxCacheBytes = DefaultMaxCacheBytes
	}
	if maxEntries <= 0 {
		maxEntries = DefaultMaxEntries
	}
	return &Cache{entries: map[string]*cached{}, maxFileBytes: maxFileBytes, maxCacheBytes: maxCacheBytes, maxEntries: maxEntries}
}

var shared = New(0, 0, 0)

func Shared() *Cache { return shared }

func (cache *Cache) Metrics() Metrics {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	var result = cache.metrics
	result.Entries = len(cache.entries)
	return result
}

// Invalidate is for explicit filesystem-change notifications. It covers
// same-size rewrites with preserved timestamps and symlink aliases as well.
func (cache *Cache) Invalidate() {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.entries = map[string]*cached{}
	cache.metrics.ResidentBytes = 0
	cache.metrics.Invalidations++
}

type Scope struct {
	root      *workspace.Root
	entry     string
	directory string
	cache     *Cache
}

func OpenScope(entryDocument string, cache *Cache) (*Scope, error) {
	var path, local = workspace.FilePath(entryDocument)
	if !local {
		return nil, workspace.ErrAccess
	}
	var root, err = workspace.Open(entryDocument)
	if err != nil {
		return nil, err
	}
	if cache == nil {
		cache = Shared()
	}
	return &Scope{root: root, entry: entryDocument, directory: filepath.Dir(path), cache: cache}, nil
}

func (scope *Scope) Close() error { return scope.root.Close() }

func (scope *Scope) fromDocument(document string) (string, error) {
	if document == "" || document == scope.entry {
		var entry, _ = workspace.FilePath(scope.entry)
		return filepath.Base(entry), nil
	}
	if filepath.IsAbs(filepath.FromSlash(document)) || strings.HasPrefix(strings.ToLower(document), "file:") {
		var path, local = workspace.FilePath(document)
		if !local {
			return "", workspace.ErrAccess
		}
		var relative, err = filepath.Rel(scope.directory, path)
		if err != nil || !filepath.IsLocal(relative) {
			return "", workspace.ErrAccess
		}
		return filepath.ToSlash(relative), nil
	}
	if !filepath.IsLocal(filepath.FromSlash(document)) {
		return "", workspace.ErrAccess
	}
	return filepath.ToSlash(document), nil
}

func (scope *Scope) Read(fromDocument, requested string) (Snapshot, error) {
	var from, err = scope.fromDocument(fromDocument)
	if err != nil {
		return Snapshot{}, err
	}
	var relative, relErr = workspace.Relative(from, requested)
	if relErr != nil {
		return Snapshot{}, relErr
	}
	var key = scope.directory + "\x00" + relative

	// Re-authorize and inspect the opened handle even for a cache hit. A cached
	// path never permits a later symlink escape or suppresses a missing file.
	var resource, openErr = scope.root.Open(from, requested)
	var cache = scope.cache
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.metrics.MetadataChecks++
	if openErr != nil {
		cache.remove(key, false)
		return Snapshot{}, openErr
	}
	defer resource.Close()
	var info = resource.Info
	cache.clock++
	if existing := cache.entries[key]; existing != nil && os.SameFile(existing.info, info) && existing.info.Size() == info.Size() && existing.info.ModTime().Equal(info.ModTime()) {
		existing.used = cache.clock
		cache.metrics.Hits++
		return existing.snapshot, nil
	}
	cache.metrics.Misses++
	cache.remove(key, false)
	if info.Size() > cache.maxFileBytes {
		return Snapshot{}, ErrTooLarge
	}
	cache.metrics.Reads++
	var raw, readErr = io.ReadAll(io.LimitReader(resource.File, cache.maxFileBytes+1))
	cache.metrics.ReadBytes += uint64(len(raw))
	if readErr != nil {
		return Snapshot{}, fmt.Errorf("cannot read embedded asset")
	}
	if int64(len(raw)) > cache.maxFileBytes {
		return Snapshot{}, ErrTooLarge
	}
	var after, statErr = resource.File.Stat()
	if statErr != nil || !os.SameFile(info, after) || info.Size() != after.Size() || !info.ModTime().Equal(after.ModTime()) || after.Size() != int64(len(raw)) {
		return Snapshot{}, ErrChanged
	}
	var digest = sha256.Sum256(raw)
	var kind = "binary"
	if utf8.Valid(raw) {
		kind = "text"
	}
	var snapshot = Snapshot{RelativePath: relative, Bytes: int64(len(raw)), SHA256: hex.EncodeToString(digest[:]), Kind: kind, Base64: base64.StdEncoding.EncodeToString(raw), Modified: after.ModTime()}
	cache.metrics.Encodes++
	var size = int64(len(snapshot.Base64))
	if size <= cache.maxCacheBytes {
		for len(cache.entries) >= cache.maxEntries || cache.metrics.ResidentBytes+size > cache.maxCacheBytes {
			var oldest string
			var used = ^uint64(0)
			for candidate, entry := range cache.entries {
				if entry.used < used {
					oldest = candidate
					used = entry.used
				}
			}
			if oldest == "" {
				break
			}
			cache.remove(oldest, true)
		}
		cache.entries[key] = &cached{snapshot: snapshot, info: after, used: cache.clock}
		cache.metrics.ResidentBytes += size
	}
	return snapshot, nil
}

func (cache *Cache) remove(key string, eviction bool) {
	if existing := cache.entries[key]; existing != nil {
		cache.metrics.ResidentBytes -= int64(len(existing.snapshot.Base64))
		delete(cache.entries, key)
		if eviction {
			cache.metrics.Evictions++
		}
	}
}
