package main

import (
	"archive/zip"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func p15PackageRun(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	python := "python3"
	all := []string{filepath.Join(root, "tools", "p10r15", "package_skill.py")}
	if isWindowsRuntime() {
		python = "py"
		all = append([]string{"-3"}, all...)
	}
	cmd := exec.CommandContext(ctx, python, append(all, args...)...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func p15PackageBuild(t *testing.T, root, commit string, packagePaths ...string) string {
	t.Helper()
	packagePath := "."
	if len(packagePaths) > 1 {
		t.Fatal("one Go main package is required")
	}
	if len(packagePaths) == 1 {
		packagePath = packagePaths[0]
	}
	tmp := t.TempDir()
	clone := filepath.Join(tmp, "source")
	p15InstallGit(t, root, "clone", "--quiet", "--shared", "--no-checkout", root, clone)
	p15InstallGit(t, clone, "config", "core.autocrlf", "false")
	p15InstallGit(t, clone, "checkout", "--quiet", "--detach", commit)
	if p15InstallGit(t, clone, "rev-parse", "HEAD") != commit || p15InstallGit(t, clone, "status", "--porcelain") != "" {
		t.Fatal("pinned compiler clone is not exact and clean")
	}
	binary := filepath.Join(tmp, "compiler")
	if isWindowsRuntime() {
		binary += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-buildvcs=true", "-trimpath", "-o", binary, packagePath)
	cmd.Dir = clone
	cmd.Env = append(p15InstallEnv(nil), "GOFLAGS=", "GOWORK=off", "GIT_NO_REPLACE_OBJECTS=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual clean compiler build: %v\n%s", err, out)
	}
	info, err := buildinfo.ReadFile(binary)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, setting := range info.Settings {
		settings[setting.Key] = setting.Value
	}
	if settings["vcs.revision"] != commit || settings["vcs.modified"] != "false" {
		t.Fatalf("actual binary provenance differs: %v", settings)
	}
	if p15InstallGit(t, clone, "status", "--porcelain") != "" {
		t.Fatal("compiler source changed during build")
	}
	return binary
}

type p15ZipEntry struct {
	header zip.FileHeader
	data   []byte
}

func p15PackageEntries(t *testing.T, archive string) []p15ZipEntry {
	t.Helper()
	r, err := zip.OpenReader(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var entries []p15ZipEntry
	for _, file := range r.File {
		body, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(body)
		_ = body.Close()
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, p15ZipEntry{file.FileHeader, raw})
	}
	return entries
}

func p15PackageWriteZip(t *testing.T, path string, entries []p15ZipEntry) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	w := zip.NewWriter(f)
	for _, entry := range entries {
		h := entry.header
		dst, err := w.CreateHeader(&h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := dst.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestP15K01UnpackedPackageContract(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	head := p15InstallGit(t, root, "rev-parse", "HEAD")
	var manifest map[string]any
	if err := json.Unmarshal(p15InstallRead(t, filepath.Join(root, "skills", "cherri-shortcuts", "compatibility-manifest.json")), &manifest); err != nil {
		t.Fatal(err)
	}
	pin := manifest["compilerCommit"].(string)
	binary := p15PackageBuild(t, root, pin)
	migration := p15PackageBuild(t, root, pin, "./cmd/cherri-migrate")
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "runtime.zip")
	args := []string{"--repo", root, "--skill-commit", head, "--kind", "runtime", "--compiler", binary, "--migrator", migration}
	create := append([]string{"create"}, args...)
	create = append(create, "--archive", archive)
	if out, err := p15PackageRun(t, root, create...); err != nil {
		t.Fatalf("actual pinned package creation: %v\n%s", err, out)
	}
	t.Run("unpacked_actual_self_test", func(t *testing.T) {
		unpack := filepath.Join(t.TempDir(), "unpacked")
		verify := append([]string{"verify"}, args...)
		verify = append(verify, "--archive", archive, "--unpack", unpack)
		if out, err := p15PackageRun(t, root, verify...); err != nil {
			t.Fatalf("independent package verification: %v\n%s", err, out)
		}
		skill := filepath.Join(unpack, "cherri-shortcuts")
		packaged := p15InstallRead(t, filepath.Join(skill, "bin", "cherri"))
		if sha256.Sum256(packaged) != sha256.Sum256(p15InstallRead(t, binary)) {
			t.Fatal("unpacked binary bytes differ from trusted actual compiler")
		}
		if string(p15InstallRead(t, filepath.Join(skill, "bin", "cherri.commit"))) != pin+"\n" {
			t.Fatal("unpacked binary commit receipt differs")
		}
		for _, tool := range []struct{ name, input, role string }{{"cherri", binary, "github.com/electrikmilk/cherri"}, {"cherri-migrate", migration, "github.com/electrikmilk/cherri/cmd/cherri-migrate"}} {
			path := filepath.Join(skill, "bin", tool.name)
			data := p15InstallRead(t, path)
			if sha256.Sum256(data) != sha256.Sum256(p15InstallRead(t, tool.input)) || string(p15InstallRead(t, path+".commit")) != pin+"\n" || string(p15InstallRead(t, path+".sha256")) != fmt.Sprintf("%x\n", sha256.Sum256(data)) {
				t.Fatalf("unpacked %s bytes/source/digest differ", tool.name)
			}
			p15ExtractedRequireRole(t, path, pin, tool.role)
		}
		env := map[string]string{"SKILL_DIR": filepath.ToSlash(skill),
			"CHERRI_HOME": filepath.ToSlash(filepath.Join(t.TempDir(), "empty-home")),
			// Exercise the macOS-style trailing separator on every platform.
			// The actual prepare/edit wrapper reports canonical absolute paths.
			"TMPDIR": filepath.ToSlash(t.TempDir()) + "/"}
		observations := filepath.Join(t.TempDir(), "wrapper-observations.json")
		out, err := p15InstallRun(t, filepath.Join(skill, "scripts", "self-test.sh"), env, "--skip-setup", "--evidence-out", filepath.ToSlash(observations))
		if err != nil || !strings.Contains(out, "Cherri Shortcuts skill self-test passed.") {
			t.Fatalf("unpacked actual wrapper self-test: %v\n%s", err, out)
		}
		var observed map[string]any
		if err := json.Unmarshal(p15InstallRead(t, observations), &observed); err != nil {
			t.Fatal(err)
		}
		records, ok := observed["records"].([]any)
		if !ok || len(records) != 0 {
			t.Fatal("wrapper smoke minted broad formal evidence")
		}
		obs, ok := observed["observations"].(map[string]any)
		if !ok || obs["mock_installation_scenarios"] != float64(10) {
			t.Fatal("mock and actual scopes are not explicit")
		}
		for _, marker := range []string{"Testing unsigned compile wrapper", "Testing decompile wrapper",
			"Testing decompiled-source recompilation", "Testing existing-Shortcut prepare/edit workflow",
			"Testing AEA1 signed-input rejection"} {
			if !strings.Contains(out, marker) {
				t.Fatalf("required actual unpacked operation not observed: %s\n%s", marker, out)
			}
		}
		t.Logf("K01_UNPACKED actual_binary_sha256=%x compiler=%s skill=%s", sha256.Sum256(packaged), pin, head)
	})
	t.Run("source_and_reproducible_runtime_inventory", func(t *testing.T) {
		source := filepath.Join(t.TempDir(), "source.zip")
		if out, err := p15PackageRun(t, root, "create", "--repo", root, "--skill-commit", head,
			"--kind", "source", "--archive", source); err != nil {
			t.Fatalf("source package: %v\n%s", err, out)
		}
		for _, e := range p15PackageEntries(t, source) {
			if strings.Contains(e.header.Name, "/bin/") {
				t.Fatal("source package contains a compiler binary")
			}
		}
		twin := filepath.Join(t.TempDir(), "runtime.zip")
		c := append([]string{"create"}, args...)
		c = append(c, "--archive", twin)
		if out, err := p15PackageRun(t, root, c...); err != nil {
			t.Fatalf("repeated exact package: %v\n%s", err, out)
		}
		if sha256.Sum256(p15InstallRead(t, archive)) != sha256.Sum256(p15InstallRead(t, twin)) {
			t.Fatal("same committed input and binary did not produce identical ZIP bytes")
		}
	})
	for _, fault := range []string{"missing_wrapper", "unexpected_file", "duplicate_path", "traversal",
		"binary_bytes", "binary_executable_mode", "compiler_commit_receipt", "forged_internal_hash"} {
		t.Run("reject_"+fault, func(t *testing.T) {
			entries := p15PackageEntries(t, archive)
			var broken []p15ZipEntry
			for _, entry := range entries {
				e := entry
				switch fault {
				case "missing_wrapper":
					if e.header.Name == "cherri-shortcuts/scripts/build.sh" {
						continue
					}
				case "binary_bytes":
					if e.header.Name == "cherri-shortcuts/bin/cherri" {
						e.data = append([]byte(nil), e.data...)
						e.data[len(e.data)/2] ^= 1
					}
				case "binary_executable_mode":
					if e.header.Name == "cherri-shortcuts/bin/cherri" {
						e.header.SetMode(0644)
					}
				case "compiler_commit_receipt":
					if e.header.Name == "cherri-shortcuts/bin/cherri.commit" {
						e.data = []byte(strings.Repeat("0", 40) + "\n")
					}
				case "forged_internal_hash":
					if e.header.Name == "cherri-shortcuts/scripts/build.sh" {
						e.data = append(e.data, []byte("\n# synthetic altered source\n")...)
					}
				}
				broken = append(broken, e)
			}
			switch fault {
			case "unexpected_file", "duplicate_path", "traversal":
				extra := entries[0]
				extra.data = []byte("SYNTHETIC_PACKAGE_FAULT")
				if fault == "unexpected_file" {
					extra.header.Name = "cherri-shortcuts/untracked.txt"
				} else if fault == "traversal" {
					extra.header.Name = "cherri-shortcuts/../escape.txt"
				}
				broken = append(broken, extra)
			case "forged_internal_hash":
				var changed p15ZipEntry
				for _, e := range broken {
					if e.header.Name == "cherri-shortcuts/scripts/build.sh" {
						changed = e
					}
				}
				for i := range broken {
					if broken[i].header.Name != "cherri-shortcuts/package-manifest.json" {
						continue
					}
					var receipt map[string]any
					if err := json.Unmarshal(broken[i].data, &receipt); err != nil {
						t.Fatal(err)
					}
					for _, row := range receipt["files"].([]any) {
						f := row.(map[string]any)
						if f["path"] == changed.header.Name {
							f["sha256"] = fmt.Sprintf("%x", sha256.Sum256(changed.data))
							f["bytes"] = len(changed.data)
						}
					}
					raw, err := json.MarshalIndent(receipt, "", "  ")
					if err != nil {
						t.Fatal(err)
					}
					broken[i].data = append(raw, '\n')
				}
			}
			path := filepath.Join(t.TempDir(), fault+".zip")
			p15PackageWriteZip(t, path, broken)
			verify := append([]string{"verify"}, args...)
			verify = append(verify, "--archive", path)
			if out, err := p15PackageRun(t, root, verify...); err == nil || !strings.Contains(out, "CHERRI_PACKAGE_INVALID") {
				t.Fatalf("real archive fault accepted: %v\n%s", err, out)
			}
		})
	}
	t.Run("occupied_unpack_destination_preserved", func(t *testing.T) {
		dest := t.TempDir()
		sentinel := filepath.Join(dest, "owned.txt")
		p15InstallWrite(t, sentinel, []byte("preserve existing bytes"))
		verify := append([]string{"verify"}, args...)
		verify = append(verify, "--archive", archive, "--unpack", dest)
		if out, err := p15PackageRun(t, root, verify...); err == nil || !strings.Contains(out, "new or empty") {
			t.Fatalf("occupied destination was not rejected: %v\n%s", err, out)
		}
		if string(p15InstallRead(t, sentinel)) != "preserve existing bytes" {
			t.Fatal("package verification altered preexisting destination")
		}
	})
	t.Run("current_compiler_cannot_impersonate_manifest_pin", func(t *testing.T) {
		if pin == head {
			t.Fatal("counterexample requires different exact compiler and Skill commits")
		}
		wrong := p15PackageBuild(t, root, head)
		path := filepath.Join(t.TempDir(), "wrong-source.zip")
		out, err := p15PackageRun(t, root, "create", "--repo", root, "--skill-commit", head,
			"--kind", "runtime", "--compiler", wrong, "--migrator", migration, "--archive", path)
		if err == nil || !strings.Contains(out, "compiler revision mismatch") {
			t.Fatalf("real current-HEAD binary falsely matched manifest pin: %v\n%s", err, out)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("invalid compiler source produced a package artifact")
		}
	})
}
