package main

import (
	"bytes"
	"debug/buildinfo"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestP15RP45IdentityValidationOrder(t *testing.T) {
	f := p15ExtractedFixture(t, "RP45", false)
	migration := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15InstallWrite(t, f.migration, p15InstallRead(t, migration))
	p15InstallWrite(t, f.migration+".commit", []byte(f.sourceSHA+"\n"))
	p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
	f.env["CHERRI_BIN"] = filepath.ToSlash(f.compiler)
	doctor := filepath.Join(f.skill, "scripts", "doctor.sh")
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy real identity pair: %v\n%s", err, output)
	}
	for _, name := range []string{"missing_checksum_before_execution", "migration_wrapper_cannot_borrow_identity", "different_platform_metadata"} {
		t.Run(name, func(t *testing.T) {
			original := map[string][]byte{}
			for _, path := range []string{f.compiler, f.compiler + ".commit", f.compiler + ".sha256", f.migration, f.migration + ".commit", f.migration + ".sha256"} {
				original[path] = p15InstallRead(t, path)
			}
			t.Cleanup(func() {
				for path, data := range original {
					p15InstallWrite(t, path, data)
				}
			})
			marker := filepath.Join(t.TempDir(), "must-not-execute")
			diagnostic := ""
			switch name {
			case "missing_checksum_before_execution":
				actual := filepath.Join(t.TempDir(), "actual-compiler")
				if isWindowsRuntime() {
					actual += ".exe"
				}
				p15InstallWrite(t, actual, original[f.compiler])
				p15InstallWrite(t, f.compiler, []byte("#!/bin/sh\nprintf observed > "+p15ExtractedShellQuote(filepath.ToSlash(marker))+"\nexec "+p15ExtractedShellQuote(filepath.ToSlash(actual))+" \"$@\"\n"))
				if err := os.Remove(f.compiler + ".sha256"); err != nil {
					t.Fatal(err)
				}
				diagnostic = "RECEIPT_MISSING"
			case "migration_wrapper_cannot_borrow_identity":
				// The actual current migrator reports its own identity; a shell
				// script forwarding its commands is a different file and role.
				p15InstallWrite(t, f.migration, []byte("#!/bin/sh\nexec "+p15ExtractedShellQuote(filepath.ToSlash(migration))+" \"$@\"\n"))
				p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
				diagnostic = "tool role"
			case "different_platform_metadata":
				other := "arm64"
				if runtime.GOARCH == "arm64" {
					other = "amd64"
				}
				before, after := []byte("build\tGOARCH="+runtime.GOARCH+"\n"), []byte("build\tGOARCH="+other+"\n")
				if len(before) != len(after) || bytes.Count(original[f.compiler], before) != 1 {
					t.Fatal("platform mutation requires one independently observed same-length build setting")
				}
				changed := bytes.Replace(original[f.compiler], before, after, 1)
				p15InstallWrite(t, f.compiler, changed)
				info, err := buildinfo.ReadFile(f.compiler)
				if err != nil {
					t.Fatal(err)
				}
				settings := map[string]string{}
				for _, setting := range info.Settings {
					settings[setting.Key] = setting.Value
				}
				if info.Path != "github.com/electrikmilk/cherri" || settings["GOARCH"] != other || settings["vcs.revision"] != f.sourceSHA || settings["vcs.modified"] != "false" {
					t.Fatalf("platform-only metadata control changed other identity: %s %v", info.Path, settings)
				}
				p15InstallWrite(t, f.compiler+".sha256", []byte(p15ExtractedHash(t, f.compiler)+"\n"))
				diagnostic = "PLATFORM_MISMATCH"
			}
			output, err := p15InstallRun(t, doctor, f.env)
			p15ExtractedRecordControl(f, name, []byte(output), err)
			if err == nil || !strings.Contains(output, diagnostic) {
				t.Errorf("identity gate accepted/misdiagnosed %s: %v expected=%s\n%s", name, err, diagnostic, output)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("unverified compiler executed before mandatory byte receipt validation: %v", err)
			}
		})
	}
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy real identity pair did not recover: %v\n%s", err, output)
	}
}
