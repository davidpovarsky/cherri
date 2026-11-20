package main

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestP15RP44ExtractedCacheAndBundleSelection(t *testing.T) {
	f := p15ExtractedFixture(t, "RP44", false)
	migration := p15ExtractedBuild(t, f.root, f.sourceSHA, "./cmd/cherri-migrate")
	p15ExtractedRequireRole(t, migration, f.sourceSHA, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
	current := map[string][]byte{"cherri": p15InstallRead(t, f.compiler), "cherri-migrate": p15InstallRead(t, migration)}
	oldSource := "d7052f4568db26dba169bdefc72623e48c04968a"
	oldBinary := p15PackageBuild(t, f.root, oldSource)
	p15ExtractedRequireRole(t, oldBinary, oldSource, "github.com/electrikmilk/cherri")
	old := p15InstallRead(t, oldBinary)
	if oldSource == f.sourceSHA || bytes.Equal(old, current["cherri"]) {
		t.Fatal("outdated compiler control does not use distinct real clean source/bytes")
	}
	bundle := filepath.Join(f.skill, "bin", "cherri")
	paths := []string{}
	for _, base := range []string{filepath.Dir(f.compiler), filepath.Dir(bundle)} {
		for _, name := range []string{"cherri", "cherri-migrate"} {
			path := filepath.Join(base, name)
			p15InstallWrite(t, path, current[name])
			p15InstallWrite(t, path+".commit", []byte(f.sourceSHA+"\n"))
			p15InstallWrite(t, path+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(current[name]))))
			paths = append(paths, path, path+".commit", path+".sha256")
		}
	}
	seed := map[string][]byte{}
	for _, path := range paths {
		seed[path] = p15InstallRead(t, path)
	}
	setOld := func(t *testing.T, path string) {
		p15InstallWrite(t, path, old)
		p15InstallWrite(t, path+".commit", []byte(oldSource+"\n"))
		p15InstallWrite(t, path+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(old))))
	}
	setup, doctor := filepath.Join(f.skill, "scripts", "setup.sh"), filepath.Join(f.skill, "scripts", "doctor.sh")
	if out, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy actual complete pair before selection: %v\n%s", err, out)
	}
	for _, name := range []string{"outdated_bundle_prefers_cache", "outdated_cache_uses_bundle", "unselected_bundle_not_executed", "checksum_invalid_cache_not_executed", "incomplete_cache_uses_bundle", "both_outdated_rejected", "explicit_outdated_never_falls_back"} {
		t.Run(name, func(t *testing.T) {
			t.Cleanup(func() {
				for path, data := range seed {
					p15InstallWrite(t, path, data)
				}
			})
			env := p15InstallEnvCopy(f.env)
			selected := f.compiler
			wantFailure := ""
			marker := filepath.Join(t.TempDir(), "ineligible-tool-executed")
			payload := []byte("#!/bin/sh\nprintf executed > " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\nexit 92\n")
			switch name {
			case "outdated_bundle_prefers_cache":
				setOld(t, bundle)
			case "outdated_cache_uses_bundle":
				setOld(t, f.compiler)
				selected = bundle
			case "unselected_bundle_not_executed":
				p15InstallWrite(t, bundle, payload)
				p15InstallWrite(t, bundle+".sha256", []byte(fmt.Sprintf("%x\n", sha256.Sum256(payload))))
			case "checksum_invalid_cache_not_executed":
				p15InstallWrite(t, f.compiler, payload)
				selected = bundle
			case "incomplete_cache_uses_bundle":
				if err := os.Remove(f.migration); err != nil {
					t.Fatal(err)
				}
				selected = bundle
			case "both_outdated_rejected":
				setOld(t, f.compiler)
				setOld(t, bundle)
				wantFailure = "no compatible cached or bundled binary available"
				shim := filepath.Join(t.TempDir(), "path")
				p15InstallWrite(t, filepath.Join(shim, "cherri"), payload)
				env["P15_INSTALL_SHIM_DIR"], env["P15_INSTALL_SHIM_COMMAND"] = shim, "cherri"
			case "explicit_outdated_never_falls_back":
				explicit := filepath.Join(t.TempDir(), "explicit-cherri")
				setOld(t, explicit)
				env["CHERRI_BIN"] = filepath.ToSlash(explicit)
				wantFailure = "Explicit CHERRI_BIN failed compatibility validation"
			}
			before := map[string][]byte{}
			missing := map[string]bool{}
			for _, path := range paths {
				data, err := os.ReadFile(path)
				if os.IsNotExist(err) {
					missing[path] = true
				} else if err != nil {
					t.Fatal(err)
				} else {
					before[path] = data
				}
			}
			output, err := p15InstallRun(t, setup, env, "--offline")
			doctorOutput, doctorErr := p15InstallRun(t, doctor, env)
			p15ExtractedRecordControl(f, name+"_setup", []byte(output), err)
			p15ExtractedRecordControl(f, name+"_doctor", []byte(doctorOutput), doctorErr)
			if wantFailure != "" {
				if err == nil || !strings.Contains(output, wantFailure) || doctorErr == nil || !strings.Contains(doctorOutput, "MISSING or INCOMPATIBLE") {
					t.Fatalf("wrong isolated rejection: setup=%v doctor=%v\n%s\n%s", err, doctorErr, output, doctorOutput)
				}
			} else {
				if err != nil || doctorErr != nil {
					t.Fatalf("verified selection failed: setup=%v doctor=%v\n%s\n%s", err, doctorErr, output, doctorOutput)
				}
				p15AssertDoctorBinary(t, doctorOutput, selected)
				wantMigration := filepath.ToSlash(filepath.Join(filepath.Dir(selected), "cherri-migrate"))
				if !strings.Contains(strings.ReplaceAll(doctorOutput, "\\", "/"), "Migration tool: "+wantMigration+" (source and role verified)") {
					t.Fatalf("doctor selected an unrelated migration companion: %s", doctorOutput)
				}
			}
			for path, data := range before {
				if !bytes.Equal(p15InstallRead(t, path), data) {
					t.Errorf("offline selection changed original bytes: %s", path)
				}
			}
			for path := range missing {
				if _, err := os.Stat(path); !os.IsNotExist(err) {
					t.Errorf("offline selection manufactured a missing candidate: %s %v", path, err)
				}
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Errorf("ineligible tool executed: %v", err)
			}
			if p15ExtractedRevision(t, filepath.Join(f.home, "docs")) != f.docsSHA {
				t.Error("selection moved the exact documentation pin")
			}
			t.Logf("RP44_ACTUAL_SELECTION case=%s old_source=%s actual_candidate_source=%s exact_selection_or_rejection=true", name, oldSource, f.sourceSHA)
		})
	}
	if out, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy complete pair did not recover after selection controls: %v\n%s", err, out)
	}
}
