package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/electrikmilk/cherri/internal/language/analysis"
	"github.com/electrikmilk/cherri/internal/language/assets"
	"github.com/electrikmilk/cherri/internal/language/service"
)

func p15RP29Allowed(t *testing.T, svc *service.Service, cache *assets.Cache, uri, source string, expected []byte, kind string) {
	t.Helper()
	p15AssetAnalyze(t, svc, uri)
	dependencies := svc.AssetDependencies(uri)
	if len(dependencies) != 1 || dependencies[0].SHA256 != p15AssetSHA(expected) || dependencies[0].Kind != kind || dependencies[0].Base64 != "" {
		t.Fatalf("authorized resource identity or text/binary kind changed: %+v", dependencies)
	}
	output, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
	if err != nil {
		t.Fatalf("authorized canonical compile rejected: %v", err)
	}
	decoded, err := base64.StdEncoding.DecodeString(p15AssetText(t, output))
	if err != nil || !bytes.Equal(decoded, expected) {
		t.Fatal("canonical compile did not preserve exact authorized resource bytes")
	}
}

func p15RP29Denied(t *testing.T, svc *service.Service, cache *assets.Cache, uri, source, outside string) {
	t.Helper()
	before := cache.Metrics()
	diagnostics, _ := svc.Analyze(uri)
	var errors []analysis.Diagnostic
	for _, diagnostic := range diagnostics {
		if diagnostic.Severity == analysis.SeverityError {
			errors = append(errors, diagnostic)
		}
	}
	if len(errors) != 1 || errors[0].Code != analysis.CodeWorkspaceAccess || errors[0].Span.Start.Line != 1 {
		t.Fatalf("unauthorized resource did not produce its exact source diagnostic: %+v", errors)
	}
	if strings.Contains(errors[0].Message, outside) {
		t.Fatal("service diagnostic leaked the resolved outside resource path")
	}
	if len(svc.AssetDependencies(uri)) != 0 {
		t.Fatal("denied resource retained an authorized dependency")
	}
	output, err := CompileSourceToPlistWithOptions(uri, source, CompileOptions{AssetCache: cache})
	if err == nil || len(output) != 0 || !strings.Contains(err.Error(), analysis.CodeWorkspaceAccess) {
		t.Fatalf("canonical compile accepted denied resource or emitted bytes: bytes=%d error=%v", len(output), err)
	}
	if strings.Contains(err.Error(), outside) {
		t.Fatal("compile error leaked the resolved outside resource path")
	}
	after := cache.Metrics()
	if after.Reads != before.Reads || after.ReadBytes != before.ReadBytes || after.Encodes != before.Encodes {
		t.Fatalf("denied resource reached content I/O or encoding: before=%+v after=%+v", before, after)
	}
}

// Every negative names a real resource outside the entry workspace, except the
// explicit missing-file case. A missing outside file alone would not prove that
// the path authorization boundary was exercised.
func TestP15RP29AssetAuthorizationAcrossServiceAndCompile(t *testing.T) {
	for _, specimen := range []struct {
		name string
		data []byte
		kind string
	}{
		{"allowed_text", []byte("PUBLIC_RP29_UTF8\n"), "text"},
		{"allowed_binary", []byte{0, 255, 128, 1, 2, 'A'}, "binary"},
	} {
		t.Run(specimen.name, func(t *testing.T) {
			root := t.TempDir()
			uri := filepath.Join(root, "entry.cherri")
			p15AssetWrite(t, filepath.Join(root, "inside.dat"), specimen.data, 1)
			cache := assets.New(0, 0, 0)
			svc := service.NewServiceWithAssetCache(nil, cache)
			source := "embedFile(\"inside.dat\")\n"
			svc.OpenDocument(uri, 1, source)
			p15RP29Allowed(t, svc, cache, uri, source, specimen.data, specimen.kind)
			if metrics := cache.Metrics(); metrics.Reads != 1 || metrics.Encodes != 1 {
				t.Fatalf("service and canonical compile did not share one authorized read: %+v", metrics)
			}
		})
	}

	for _, scenario := range []string{"missing", "existing_parent_escape", "file_symlink_escape", "directory_symlink_escape"} {
		t.Run(scenario, func(t *testing.T) {
			base := t.TempDir()
			root := filepath.Join(base, "workspace")
			outsideDir := filepath.Join(base, "outside")
			outside := filepath.Join(outsideDir, "payload.dat")
			allowed := []byte("PUBLIC_RP29_RECOVERY")
			p15AssetWrite(t, filepath.Join(root, "inside.dat"), allowed, 1)
			p15AssetWrite(t, outside, []byte("PUBLIC_RP29_MUST_NOT_BE_READ"), 1)
			var relative string
			switch scenario {
			case "missing":
				relative = "missing.dat"
			case "existing_parent_escape":
				relative = "../outside/payload.dat"
			case "file_symlink_escape":
				relative = "linked.dat"
				if err := os.Symlink(outside, filepath.Join(root, relative)); err != nil {
					t.Skipf("actual filesystem cannot create required symlink; RP29 needs a supported host: %v", err)
				}
			case "directory_symlink_escape":
				relative = "linked/payload.dat"
				if err := os.Symlink(outsideDir, filepath.Join(root, "linked")); err != nil {
					t.Skipf("actual filesystem cannot create required directory symlink; RP29 needs a supported host: %v", err)
				}
			}
			if scenario != "missing" {
				// Prove that failure is not just a nonexistent test resource.
				data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(relative)))
				if err != nil || string(data) != "PUBLIC_RP29_MUST_NOT_BE_READ" {
					t.Fatal("authorization negative did not address the real outside witness")
				}
			}
			cache := assets.New(0, 0, 0)
			svc := service.NewServiceWithAssetCache(nil, cache)
			uri := filepath.Join(root, "entry.cherri")
			source := fmt.Sprintf("embedFile(%q)\n", relative)
			svc.OpenDocument(uri, 1, source)
			p15RP29Denied(t, svc, cache, uri, source, outside)
			recovery := "embedFile(\"inside.dat\")\n"
			svc.ChangeDocument(uri, 2, recovery)
			p15RP29Allowed(t, svc, cache, uri, recovery, allowed, "text")
		})
	}
}

// Cached bytes must not authorize a path after it becomes an outside symlink.
// The authorizer has to run before the cache can serve a former healthy result.
func TestP15RP29CachedResourceIsReauthorizedAfterSymlinkReplacement(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "workspace")
	path := filepath.Join(root, "inside.dat")
	outside := filepath.Join(base, "outside.dat")
	initial, restored := []byte("PUBLIC_RP29_INITIAL"), []byte("PUBLIC_RP29_RESTORED")
	p15AssetWrite(t, path, initial, 1)
	p15AssetWrite(t, outside, []byte("PUBLIC_RP29_OUTSIDE_REPLACEMENT"), 1)
	uri := filepath.Join(root, "entry.cherri")
	source := "embedFile(\"inside.dat\")\n"
	cache := assets.New(0, 0, 0)
	svc := service.NewServiceWithAssetCache(nil, cache)
	svc.OpenDocument(uri, 1, source)
	p15RP29Allowed(t, svc, cache, uri, source, initial, "text")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, path); err != nil {
		t.Skipf("actual filesystem cannot create required replacement symlink; RP29 needs a supported host: %v", err)
	}
	p15RP29Denied(t, svc, cache, uri, source, outside)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	p15AssetWrite(t, path, restored, 2)
	svc.InvalidateAssets()
	p15RP29Allowed(t, svc, cache, uri, source, restored, "text")
}
