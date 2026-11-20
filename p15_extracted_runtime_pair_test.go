package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP15RP45ExtractedRuntimePair(t *testing.T) {
	f := p15ExtractedFixture(t, "RP45", false)
	migration := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15ExtractedRequireRole(t, migration, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	archive := filepath.Join(t.TempDir(), "actual-runtime.zip")
	unpack := filepath.Join(t.TempDir(), "verified-runtime")
	args := []string{"--repo", f.packageRepo, "--skill-commit", f.skillSHA, "--kind", "runtime", "--compiler", f.compiler, "--migrator", migration, "--archive", archive}
	if out, err := p15PackageRun(t, f.root, append([]string{"create"}, args...)...); err != nil {
		t.Fatalf("actual complete runtime package create: %v\n%s", err, out)
	}
	verify := append(append([]string{"verify"}, args...), "--unpack", unpack)
	if out, err := p15PackageRun(t, f.root, verify...); err != nil {
		t.Fatalf("actual complete runtime package verify/extract: %v\n%s", err, out)
	}
	skill := filepath.Join(unpack, "cherri-shortcuts")
	compiler, helper := filepath.Join(skill, "bin", "cherri"), filepath.Join(skill, "bin", "cherri-migrate")
	t.Run("exact_extracted_pair", func(t *testing.T) {
		for _, item := range []struct{ path, original, role string }{{compiler, f.compiler, "github.com/electrikmilk/cherri"}, {helper, migration, "github.com/electrikmilk/cherri/cmd/cherri-migrate"}} {
			data := p15InstallRead(t, item.path)
			if !bytes.Equal(data, p15InstallRead(t, item.original)) {
				t.Fatalf("package changed authentic tool bytes: %s", item.path)
			}
			p15ExtractedRequireRole(t, item.path, f.sourceSHA, item.role)
			if string(p15InstallRead(t, item.path+".commit")) != f.sourceSHA+"\n" || string(p15InstallRead(t, item.path+".sha256")) != fmt.Sprintf("%x\n", sha256.Sum256(data)) {
				t.Fatalf("package changed tool source/digest receipt: %s", item.path)
			}
		}
	})
	home := filepath.Join(t.TempDir(), "runtime-home")
	docs := filepath.Join(home, "docs")
	p15ExtractedGit(t, f.root, "clone", "--quiet", "--no-checkout", f.env["CHERRI_DOCS_REPO"], docs)
	p15ExtractedGit(t, docs, "config", "core.autocrlf", "false")
	p15ExtractedGit(t, docs, "checkout", "--quiet", "--detach", f.docsSHA)
	env := p15InstallEnvCopy(f.env)
	env["SKILL_DIR"], env["CHERRI_HOME"] = filepath.ToSlash(skill), filepath.ToSlash(home)
	t.Run("actual_offline_bundled_install", func(t *testing.T) {
		out, err := p15InstallRun(t, filepath.Join(skill, "scripts", "setup.sh"), env, "--offline")
		p15ExtractedRecordControl(f, t.Name(), []byte(out), err)
		if err != nil {
			t.Fatalf("actual extracted complete runtime offline install: %v\n%s", err, out)
		}
		out, err = p15InstallRun(t, filepath.Join(skill, "scripts", "doctor.sh"), env)
		if err != nil {
			t.Fatalf("actual extracted runtime doctor: %v\n%s", err, out)
		}
		p15AssertDoctorBinary(t, out, compiler)
		p15AssertDoctorMigration(t, out, helper)
		if _, err := os.Stat(filepath.Join(home, "bin", "cherri")); !os.IsNotExist(err) {
			t.Fatalf("runtime positive accidentally used a built/cached compiler: %v", err)
		}
	})
	t.Run("shipped_compiler_and_migrator_execute", func(t *testing.T) {
		dir := t.TempDir()
		legacy := filepath.Join(dir, "runtime-legacy.cherri")
		original := []byte("const marker = text(\"RP45 runtime migration\")\nshow(marker)\n")
		p15InstallWrite(t, legacy, original)
		mainOut, stderr, err := f.run(t, compiler, dir, "migrate", legacy)
		if err != nil {
			t.Fatalf("shipped compiler migration: %v %s", err, stderr)
		}
		toolOut, stderr, err := f.run(t, helper, dir, legacy)
		if err != nil || !bytes.Equal(toolOut, mainOut) || !bytes.Equal(p15InstallRead(t, legacy), original) {
			t.Fatalf("shipped companion migration differs or wrote in preview: %v %s", err, stderr)
		}
		out, stderr, err := f.run(t, helper, dir, legacy, "--check")
		if err != nil || !strings.Contains(string(out), "can be migrated cleanly") || !bytes.Equal(p15InstallRead(t, legacy), original) {
			t.Fatalf("shipped migration check: %v %s %s", err, out, stderr)
		}
		out, stderr, err = f.run(t, helper, dir, legacy, "--write")
		if err != nil || !strings.Contains(string(out), "Migrated and updated") || !bytes.Equal(p15InstallRead(t, legacy), mainOut) {
			t.Fatalf("shipped migration write: %v %s %s", err, out, stderr)
		}
		out, stderr, err = f.run(t, compiler, dir, "check", legacy, "--json")
		obj, ok := p15DecodeOneJSON(t, out).(map[string]any)
		if err != nil || !ok || obj["valid"] != true {
			t.Fatalf("shipped compiler rejected shipped migration output: %v %s %v", err, stderr, obj)
		}
	})
	t.Run("controlled_installation_self_test", func(t *testing.T) {
		observations := filepath.Join(t.TempDir(), "wrapper-observations.json")
		out, err := p15InstallRun(t, filepath.Join(skill, "scripts", "self-test.sh"), env, "--skip-setup", "--evidence-out", filepath.ToSlash(observations))
		p15ExtractedRecordControl(f, t.Name(), []byte(out), err)
		if err != nil || !strings.Contains(out, "Cherri Shortcuts skill self-test passed.") {
			t.Fatalf("actual extracted package self-test: %v\n%s", err, out)
		}
		var evidence map[string]any
		if err := json.Unmarshal(p15InstallRead(t, observations), &evidence); err != nil {
			t.Fatal(err)
		}
		records, ok := evidence["records"].([]any)
		if !ok || len(records) != 0 {
			t.Fatal("wrapper/controlled transport checks minted broad formal acceptance records")
		}
		observed, ok := evidence["observations"].(map[string]any)
		if !ok || observed["mock_installation_scenarios"] != float64(10) {
			t.Fatal("controlled transport scope was not kept separate and explicit")
		}
	})
	f.receipt["runtime_archive_sha256"] = p15ExtractedHash(t, archive)
	f.receipt["runtime_tools"] = map[string]string{"cherri": p15ExtractedHash(t, compiler), "cherri-migrate": p15ExtractedHash(t, helper)}
}
