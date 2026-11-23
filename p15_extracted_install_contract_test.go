package main

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"debug/buildinfo"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/electrikmilk/cherri/internal/shortcutcompare"
)

// This fixture derives a disposable compatibility tuple from the actual clean
// source, its real capabilities command and freshly generated reference. It
// never edits or claims the independently pinned published Skill/docs tuple.
type p15ExtractedInstall struct {
	packageRepo                        string
	root, sourceSHA, skillSHA, docsSHA string
	skill, home, compiler, migration   string
	env                                map[string]string
	receipt                            map[string]any
}

func p15ExtractedGit(t *testing.T, root string, args ...string) []byte {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"--no-replace-objects", "-C", root}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("immutable fixture git %v: %v (context %v)\n%s", args, err, ctx.Err(), out)
	}
	return out
}

func p15ExtractedRevision(t *testing.T, root string) string {
	t.Helper()
	return strings.TrimSpace(string(p15ExtractedGit(t, root, "rev-parse", "HEAD")))
}

func p15ExtractedBuild(t *testing.T, root, sourceSHA, packagePath string, extraFlags ...string) string {
	t.Helper()
	if p15ExtractedRevision(t, root) != sourceSHA || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("real component build requires the exact clean source")
	}
	buildRoot := root
	gitEntry, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	if !gitEntry.IsDir() {
		// Go 1.25's VCS root discovery recognizes only a .git directory. A
		// linked worktree therefore silently omits build identity, even with
		// -buildvcs=true. Preserve the identity gate by building the exact clean
		// commit in a short-lived shared clone, never by relabeling a binary.
		buildRoot = filepath.Join(t.TempDir(), "source")
		p15ExtractedGit(t, root, "clone", "--quiet", "--shared", "--no-checkout", root, buildRoot)
		defer func() {
			if err := os.RemoveAll(buildRoot); err != nil {
				t.Error(err)
			}
		}()
		p15ExtractedGit(t, buildRoot, "config", "core.autocrlf", "false")
		p15ExtractedGit(t, buildRoot, "checkout", "--quiet", "--detach", sourceSHA)
		if p15ExtractedRevision(t, buildRoot) != sourceSHA || strings.TrimSpace(string(p15ExtractedGit(t, buildRoot, "status", "--porcelain"))) != "" {
			t.Fatal("build clone does not preserve clean source identity")
		}
	}
	path := filepath.Join(t.TempDir(), "tool")
	if isWindowsRuntime() {
		path += ".exe"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	args := append([]string{"build", "-buildvcs=true", "-trimpath"}, extraFlags...)
	args = append(args, "-o", path, packagePath)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = buildRoot
	cmd.Env = append(p15InstallEnv(nil), "GOWORK=off", "GIT_NO_REPLACE_OBJECTS=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("actual clean %s build: %v (context %v)\n%s", packagePath, err, ctx.Err(), out)
	}
	p15ExtractedBuildIdentity(t, path, sourceSHA)
	if p15ExtractedRevision(t, root) != sourceSHA || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("component source changed during build")
	}
	return path
}

func p15ExtractedBuildIdentity(t *testing.T, path, sourceSHA string) {
	t.Helper()
	info, err := buildinfo.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := map[string]string{}
	for _, item := range info.Settings {
		settings[item.Key] = item.Value
	}
	if settings["vcs"] != "git" || settings["vcs.revision"] != sourceSHA || settings["vcs.modified"] != "false" {
		t.Fatalf("actual installed tool lacks exact clean build identity: %v", settings)
	}
}

func p15ExtractedHash(t *testing.T, path string) string {
	t.Helper()
	return fmt.Sprintf("%x", sha256.Sum256(p15InstallRead(t, path)))
}

func p15ExtractedCommit(t *testing.T, root, message string) string {
	t.Helper()
	p15ExtractedGit(t, root, "init", "-q")
	p15ExtractedGit(t, root, "config", "core.autocrlf", "false")
	p15ExtractedGit(t, root, "add", ".")
	p15ExtractedGit(t, root, "-c", "user.name=Cherri-QA", "-c", "user.email=qa@example.invalid", "commit", "-qm", message)
	return p15ExtractedRevision(t, root)
}

func p15ExtractedFixture(t *testing.T, requirement string, install bool) *p15ExtractedInstall {
	t.Helper()
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	f := &p15ExtractedInstall{root: root, sourceSHA: p15ExtractedRevision(t, root), receipt: map[string]any{"requirement": requirement, "component_tuple_only": true}}
	if strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
		t.Fatal("component fixture requires clean committed source")
	}
	manifestPath := filepath.Join(root, "skills", "cherri-shortcuts", "compatibility-manifest.json")
	manifestBefore := p15InstallRead(t, manifestPath)
	t.Cleanup(func() {
		if !bytes.Equal(p15InstallRead(t, manifestPath), manifestBefore) || p15ExtractedRevision(t, root) != f.sourceSHA || strings.TrimSpace(string(p15ExtractedGit(t, root, "status", "--porcelain"))) != "" {
			t.Error("component fixture changed the real source/consumer tuple")
		}
		data, e := json.Marshal(f.receipt)
		if e != nil {
			t.Error(e)
			return
		}
		t.Logf("P15_EXTRACTED_COMPONENT_RECEIPT %s", data)
	})
	binary := p15InstallActualTool(t, root, f.sourceSHA, ".", "")
	out, stderr, err := p15RunCLI(t, binary, t.TempDir(), nil, "--capabilities-json")
	if err != nil || len(stderr) != 0 {
		t.Fatalf("actual source capabilities: %v %s", err, stderr)
	}
	caps, okay := p15DecodeOneJSON(t, out).(map[string]any)
	if !okay {
		t.Fatal("capabilities must be a JSON object")
	}
	var manifest map[string]any
	if err := json.Unmarshal(manifestBefore, &manifest); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"languageVersion", "catalogSchemaVersion", "protocolVersion", "schemaFingerprint", "codecFingerprint", "macroFingerprint"} {
		value, ok := caps[key].(string)
		if !ok || value == "" {
			t.Fatalf("actual capabilities missing %s", key)
		}
		manifest[key] = value
	}
	tmp := t.TempDir()
	docs := filepath.Join(tmp, "docs-repository")
	docsPath := filepath.Join(docs, "fork-language", "actions.md")
	if err := os.MkdirAll(filepath.Dir(docsPath), 0755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	cmd := exec.CommandContext(ctx, "go", "run", "./tools/language-docs", docsPath)
	cmd.Dir = root
	cmd.Env = append(p15InstallEnv(nil), "GOWORK=off", "GIT_NO_REPLACE_OBJECTS=1")
	docOutput, docErr := cmd.CombinedOutput()
	cancel()
	if docErr != nil {
		t.Fatalf("actual reference generation: %v\n%s", docErr, docOutput)
	}
	f.docsSHA = p15ExtractedCommit(t, docs, "Disposable actual generated component reference")
	fixtureRepo := filepath.Join(tmp, "skill-repository")
	if err := os.MkdirAll(fixtureRepo, 0755); err != nil {
		t.Fatal(err)
	}
	archiveBytes := p15ExtractedGit(t, root, "archive", f.sourceSHA, "skills/cherri-shortcuts", "LICENSE")
	reader := tar.NewReader(bytes.NewReader(archiveBytes))
	for {
		header, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag == tar.TypeXGlobalHeader {
			if header.Name != "pax_global_header" || header.Size != 0 || len(header.PAXRecords) != 1 || header.PAXRecords["comment"] != f.sourceSHA {
				t.Fatalf("Git archive metadata does not bind the exact source: %+v", header)
			}
			continue
		}
		if header.Typeflag == tar.TypeDir {
			continue
		}
		clean := filepath.ToSlash(filepath.Clean(header.Name))
		if header.Typeflag != tar.TypeReg || (clean != "LICENSE" && !strings.HasPrefix(clean, "skills/cherri-shortcuts/")) || clean != header.Name || strings.Contains(clean, "..") || header.Size > 4<<20 {
			t.Fatalf("unexpected committed Skill archive entry: %+v", header)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			t.Fatal(err)
		}
		p15InstallWrite(t, filepath.Join(fixtureRepo, filepath.FromSlash(clean)), data)
		if err := os.Chmod(filepath.Join(fixtureRepo, filepath.FromSlash(clean)), os.FileMode(header.Mode)); err != nil {
			t.Fatal(err)
		}
	}
	manifest["compilerCommit"], manifest["docsCommit"] = f.sourceSHA, f.docsSHA
	manifest["compilerRepo"], manifest["docsRepo"] = filepath.ToSlash(root), filepath.ToSlash(docs)
	derivedManifest, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	p15InstallWrite(t, filepath.Join(fixtureRepo, "skills", "cherri-shortcuts", "compatibility-manifest.json"), append(derivedManifest, '\n'))
	f.skillSHA = p15ExtractedCommit(t, fixtureRepo, "Disposable actual source/docs component tuple; not a release")
	f.packageRepo = fixtureRepo
	zipPath := filepath.Join(tmp, "source-skill.zip")
	unpack := filepath.Join(tmp, "extracted")
	args := []string{"--repo", fixtureRepo, "--skill-commit", f.skillSHA, "--kind", "source", "--archive", zipPath}
	if output, err := p15PackageRun(t, root, append([]string{"create"}, args...)...); err != nil {
		t.Fatalf("actual source package create: %v\n%s", err, output)
	}
	verify := append(append([]string{"verify"}, args...), "--unpack", unpack)
	if output, err := p15PackageRun(t, root, verify...); err != nil {
		t.Fatalf("actual package verify/extract: %v\n%s", err, output)
	}
	f.skill, f.home = filepath.Join(unpack, "cherri-shortcuts"), filepath.Join(tmp, "installed-home")
	f.compiler, f.migration = filepath.Join(f.home, "bin", "cherri"), filepath.Join(f.home, "bin", "cherri-migrate")
	f.env = map[string]string{"SKILL_DIR": filepath.ToSlash(f.skill), "CHERRI_HOME": filepath.ToSlash(f.home), "CHERRI_REPO": filepath.ToSlash(root), "CHERRI_REF": f.sourceSHA, "CHERRI_DOCS_REPO": filepath.ToSlash(docs), "CHERRI_DOCS_REF": f.docsSHA}
	f.receipt["source_commit"], f.receipt["skill_fixture_commit"], f.receipt["docs_fixture_commit"] = f.sourceSHA, f.skillSHA, f.docsSHA
	f.receipt["capabilities_sha256"] = fmt.Sprintf("%x", sha256.Sum256(out))
	f.receipt["reference_compiler_sha256"], f.receipt["archive_sha256"], f.receipt["generated_docs_sha256"] = p15ExtractedHash(t, binary), p15ExtractedHash(t, zipPath), p15ExtractedHash(t, docsPath)
	f.receipt["component_manifest"] = manifest
	if install {
		output, err := p15InstallRun(t, filepath.Join(f.skill, "scripts", "setup.sh"), f.env)
		f.receipt["setup_output_base64"] = base64.StdEncoding.EncodeToString([]byte(output))
		if err != nil {
			t.Fatalf("actual extracted source install: %v\n%s", err, output)
		}
		p15ExtractedBuildIdentity(t, f.compiler, f.sourceSHA)
		if string(p15InstallRead(t, f.compiler+".commit")) != f.sourceSHA+"\n" || strings.TrimSpace(string(p15InstallRead(t, f.compiler+".sha256"))) != p15ExtractedHash(t, f.compiler) {
			t.Fatal("installed compiler source/digest receipts differ from actual bytes")
		}
		f.receipt["installed_compiler_sha256"] = p15ExtractedHash(t, f.compiler)
	} else {
		p15InstallWrite(t, f.compiler, p15InstallRead(t, binary))
		p15InstallWrite(t, f.compiler+".commit", []byte(f.sourceSHA+"\n"))
		p15InstallWrite(t, f.compiler+".sha256", []byte(p15ExtractedHash(t, f.compiler)+"\n"))
		p15ExtractedGit(t, root, "clone", "--quiet", docs, filepath.Join(f.home, "docs"))
	}
	return f
}

func (f *p15ExtractedInstall) run(t *testing.T, binary, cwd string, args ...string) ([]byte, []byte, error) {
	t.Helper()
	// The Skill distributes POSIX-named executable files, including on Git
	// Bash. Windows Go LookPath refuses extensionless files even when their
	// actual PE bytes are executable. Invoke the exact installed file through
	// the Skill's real shell; do not copy/rename it or substitute another tool.
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	command := []string{"--noprofile", "--norc", "-c", "exec \"$@\"", "cherri-installed-cli", filepath.ToSlash(binary)}
	command = append(command, args...)
	cmd := exec.CommandContext(ctx, p15SkillBash(t), command...)
	cmd.Dir, cmd.Env = cwd, p15InstallEnv(nil)
	var stdout, stderrBuffer bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderrBuffer
	err := cmd.Run()
	if ctx.Err() != nil {
		t.Fatalf("installed public command timeout is not an accepted result: %v", ctx.Err())
	}
	out, stderr := stdout.Bytes(), stderrBuffer.Bytes()
	record := map[string]any{"args": args, "stdout_base64": base64.StdEncoding.EncodeToString(out), "stderr_base64": base64.StdEncoding.EncodeToString(stderr), "success": err == nil}
	commands, _ := f.receipt["commands"].([]any)
	f.receipt["commands"] = append(commands, record)
	return out, stderr, err
}

func p15ExtractedNativeEqual(t *testing.T, first, second string) {
	t.Helper()
	a, err := shortcutcompare.LoadShortcutDocument(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := shortcutcompare.LoadShortcutDocument(second)
	if err != nil {
		t.Fatal(err)
	}
	if result := shortcutcompare.CompareDocumentsNormalized(a, b); !result.Equal {
		t.Fatalf("public installed routes changed native semantics: %+v", result)
	}
}

func TestP15RP42ExtractedInstallPublicCLI(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP42", true)
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "public.cherri")
	source := []byte("let marker=text(\"RP42 actual installed command\")\nshow(marker)\n")
	p15InstallWrite(t, sourcePath, source)
	native := filepath.Join(dir, "compiled.shortcut")
	t.Run("help_and_schema", func(t *testing.T) {
		out, stderr, err := f.run(t, f.compiler, dir, "--help", "--no-ansi")
		if err != nil {
			t.Fatalf("installed help: %v %s", err, stderr)
		}
		for _, item := range []string{"--check", "--format", "--import", "--action-schema-json", "--capabilities-json"} {
			if !strings.Contains(string(out)+string(stderr), item) {
				t.Errorf("public help omitted %s", item)
			}
		}
		for _, arg := range []string{"--capabilities-json", "--actions-json", "--action-schema-json", "--action-schema-json=show"} {
			out, stderr, err = f.run(t, f.compiler, dir, arg)
			if err != nil || len(stderr) != 0 {
				t.Fatalf("installed %s: %v %s", arg, err, stderr)
			}
			p15DecodeOneJSON(t, out)
		}
	})
	t.Run("check_and_alias", func(t *testing.T) {
		for _, args := range [][]string{{"check", sourcePath, "--json"}, {"--check=" + sourcePath, "--json"}} {
			out, stderr, err := f.run(t, f.compiler, dir, args...)
			if err != nil {
				t.Fatalf("installed check: %v %s", err, stderr)
			}
			obj, ok := p15DecodeOneJSON(t, out).(map[string]any)
			if !ok || obj["valid"] != true {
				t.Fatalf("check protocol: %v", obj)
			}
		}
		bad := filepath.Join(dir, "invalid.cherri")
		p15InstallWrite(t, bad, []byte("missingRP42Action()\n"))
		for _, args := range [][]string{{"check", bad, "--json"}, {"--check=" + bad, "--json"}} {
			out, _, err := f.run(t, f.compiler, dir, args...)
			obj, ok := p15DecodeOneJSON(t, out).(map[string]any)
			if err == nil || !ok || obj["valid"] != false {
				t.Fatalf("invalid check accepted: %v %v", err, obj)
			}
		}
		if !bytes.Equal(p15InstallRead(t, sourcePath), source) {
			t.Fatal("analysis wrote source")
		}
	})
	t.Run("format_and_aliases", func(t *testing.T) {
		var formatted []byte
		for _, args := range [][]string{{"format", sourcePath}, {"fmt", sourcePath}, {"--format=" + sourcePath}} {
			out, stderr, err := f.run(t, f.compiler, dir, args...)
			if err != nil || len(out) == 0 {
				t.Fatalf("installed format: %v %s", err, stderr)
			}
			if formatted == nil {
				formatted = out
			} else if !bytes.Equal(formatted, out) {
				t.Fatal("format alias diverged")
			}
		}
		if !bytes.Equal(p15InstallRead(t, sourcePath), source) {
			t.Fatal("format preview wrote source")
		}
		if _, _, err := f.run(t, f.compiler, dir, "format", sourcePath, "--check"); err == nil {
			t.Fatal("unformatted source passed check")
		}
		if _, stderr, err := f.run(t, f.compiler, dir, "format", sourcePath, "--write"); err != nil {
			t.Fatalf("format write: %v %s", err, stderr)
		}
		if !bytes.Equal(p15InstallRead(t, sourcePath), formatted) {
			t.Fatal("format write differs from preview")
		}
		if _, stderr, err := f.run(t, f.compiler, dir, "fmt", sourcePath, "--check"); err != nil {
			t.Fatalf("formatted check: %v %s", err, stderr)
		}
	})
	t.Run("compile_and_file_alias", func(t *testing.T) {
		if _, stderr, err := f.run(t, f.compiler, dir, "compile", sourcePath, "--skip-sign", "--output="+native); err != nil {
			t.Fatalf("installed compile: %v %s", err, stderr)
		}
		alias := filepath.Join(dir, "file-alias.shortcut")
		// The actual public help documents -o=VALUE for the file-form alias.
		if _, stderr, err := f.run(t, f.compiler, dir, sourcePath, "--skip-sign", "-o="+alias); err != nil {
			t.Fatalf("installed file alias: %v %s", err, stderr)
		}
		p15ExtractedNativeEqual(t, native, alias)
		f.receipt["native_original_base64"] = base64.StdEncoding.EncodeToString(p15InstallRead(t, native))
	})
	t.Run("import_and_alias", func(t *testing.T) {
		for index, args := range [][]string{{"import", native}, {"--import=" + native}} {
			path := filepath.Join(dir, fmt.Sprintf("imported-%d.cherri", index))
			args = append(args, "--output="+path, "--fidelity=lossless")
			if _, stderr, err := f.run(t, f.compiler, dir, args...); err != nil {
				t.Fatalf("installed import: %v %s", err, stderr)
			}
			out, stderr, err := f.run(t, f.compiler, dir, "check", path, "--json")
			if err != nil {
				t.Fatalf("default imported V2 source: %v %s", err, stderr)
			}
			if obj := p15DecodeOneJSON(t, out).(map[string]any); obj["valid"] != true {
				t.Fatal("import output did not analyze")
			}
			compiled := filepath.Join(dir, fmt.Sprintf("roundtrip-%d.shortcut", index))
			if _, stderr, err := f.run(t, f.compiler, dir, "compile", path, "--skip-sign", "--output="+compiled); err != nil {
				t.Fatalf("installed import recompile: %v %s", err, stderr)
			}
			p15ExtractedNativeEqual(t, native, compiled)
		}
	})
	t.Run("installed_migration_tool", func(t *testing.T) {
		if _, err := os.Stat(f.migration); err != nil {
			t.Fatalf("extracted source installation omitted the required cherri-migrate companion: %v", err)
		}
		p15ExtractedBuildIdentity(t, f.migration, f.sourceSHA)
		legacy := filepath.Join(dir, "legacy.cherri")
		original := []byte("const marker = text(\"RP42 migration\")\nshow(marker)\n")
		p15InstallWrite(t, legacy, original)
		mainOut, stderr, err := f.run(t, f.compiler, dir, "migrate", legacy)
		if err != nil {
			t.Fatalf("main migration: %v %s", err, stderr)
		}
		toolOut, stderr, err := f.run(t, f.migration, dir, legacy)
		if err != nil || !bytes.Equal(toolOut, mainOut) {
			t.Fatalf("migration binary pipeline differs: %v %s", err, stderr)
		}
		if !bytes.Equal(p15InstallRead(t, legacy), original) {
			t.Fatal("migration preview changed source")
		}
		out, stderr, err := f.run(t, f.migration, dir, legacy, "--check")
		if err != nil || !strings.Contains(string(out), "can be migrated cleanly") || !bytes.Equal(p15InstallRead(t, legacy), original) {
			t.Fatalf("documented migration check: %v %s %s", err, out, stderr)
		}
		out, stderr, err = f.run(t, f.migration, dir, legacy, "--write")
		if err != nil || !strings.Contains(string(out), "Migrated and updated") || !bytes.Equal(p15InstallRead(t, legacy), mainOut) {
			t.Fatalf("documented migration write: %v %s %s", err, out, stderr)
		}
		f.receipt["installed_migration_sha256"] = p15ExtractedHash(t, f.migration)
	})
}

func TestP15RP42MigrationDocumentedFlags(t *testing.T) {
	root, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	head := p15ExtractedRevision(t, root)
	binary := p15InstallActualTool(t, root, head, "./cmd/cherri-migrate", "")
	for _, flag := range []string{"--check", "--write"} {
		t.Run(strings.TrimPrefix(flag, "--"), func(t *testing.T) {
			for _, after := range []bool{false, true} {
				name := "before_file"
				if after {
					name = "documented_after_file"
				}
				t.Run(name, func(t *testing.T) {
					dir := t.TempDir()
					path := filepath.Join(dir, "legacy.cherri")
					source := []byte("const marker = text(\"RP42 flags\")\nshow(marker)\n")
					p15InstallWrite(t, path, source)
					preview, stderr, err := p15RunCLI(t, binary, dir, nil, path)
					if err != nil {
						t.Fatalf("healthy migration preview: %v %s", err, stderr)
					}
					args := []string{flag, path}
					if after {
						args = []string{path, flag}
					}
					out, stderr, err := p15RunCLI(t, binary, dir, nil, args...)
					want := source
					message := "can be migrated cleanly"
					if flag == "--write" {
						want = preview
						message = "Migrated and updated"
					}
					if err != nil || !bytes.Equal(p15InstallRead(t, path), want) || !strings.Contains(string(out), message) {
						t.Fatalf("documented flag was not applied: args=%v err=%v stdout=%s stderr=%s source=%s", args, err, out, stderr, p15InstallRead(t, path))
					}
				})
			}
		})
	}
}

func p15ExtractedShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func TestP15RP44ExtractedCapabilitiesRejection(t *testing.T) {
	p15FullSuiteSubprocessParallel(t)
	f := p15ExtractedFixture(t, "RP44", false)
	healthyOut, stderr, err := f.run(t, f.compiler, t.TempDir(), "--capabilities-json")
	if err != nil || len(stderr) != 0 {
		t.Fatalf("healthy actual capabilities: %v %s", err, stderr)
	}
	healthy := p15DecodeOneJSON(t, healthyOut).(map[string]any)
	// The healthy candidate is an actual current compiler. Only the adversarial
	// candidates intercept one protocol response, while delegating version to
	// those same real bytes. These controls never mint compiler/native evidence.
	realBinary := filepath.Join(t.TempDir(), "actual-cherri")
	if isWindowsRuntime() {
		realBinary += ".exe"
	}
	p15InstallWrite(t, realBinary, p15InstallRead(t, f.compiler))
	realMigration := p15InstallActualTool(t, f.root, f.sourceSHA, "./cmd/cherri-migrate", "")
	p15InstallWrite(t, f.migration, p15InstallRead(t, realMigration))
	p15InstallWrite(t, f.migration+".commit", []byte(f.sourceSHA+"\n"))
	p15InstallWrite(t, f.migration+".sha256", []byte(p15ExtractedHash(t, f.migration)+"\n"))
	doctor := filepath.Join(f.skill, "scripts", "doctor.sh")
	f.env["CHERRI_BIN"] = filepath.ToSlash(f.compiler)
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy extracted doctor must pass before corruption: %v\n%s", err, output)
	}
	flat := map[string]any{}
	for _, key := range []string{"languageVersion", "catalogSchemaVersion", "protocolVersion", "schemaFingerprint", "codecFingerprint", "macroFingerprint"} {
		flat[key] = healthy[key]
	}
	jsonBytes := func(value any) []byte {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	change := func(key string, value any, remove bool) []byte {
		out := map[string]any{}
		for k, v := range flat {
			out[k] = v
		}
		if remove {
			delete(out, key)
		} else {
			out[key] = value
		}
		return jsonBytes(out)
	}
	plain := jsonBytes(flat)
	cases := []struct {
		name       string
		payload    []byte
		diagnostic string
	}{
		{"missing_comma", []byte(strings.Replace(string(plain), ",\n", "\n", 1)), "malformed JSON"},
		{"trailing_comma", []byte(strings.TrimSuffix(string(plain), "}") + ",\n}"), "malformed JSON"},
		{"duplicate_schema", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"schemaFingerprint\": \"" + flat["schemaFingerprint"].(string) + "\"\n}"), "malformed JSON"},
		{"duplicate_escaped_schema", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"schema\\u0046ingerprint\": \"" + flat["schemaFingerprint"].(string) + "\"\n}"), "malformed JSON"},
		{"invalid_escape", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"unused\": \"\\x\"\n}"), "malformed JSON"},
		{"invalid_number", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"unused\": 01\n}"), "malformed JSON"},
		{"unpaired_surrogate", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"unused\": \"\\ud800\"\n}"), "malformed JSON"},
		{"invalid_utf8", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"unused\": \"\xff\"\n}"), "malformed JSON"},
		{"raw_nul", []byte(strings.TrimSuffix(string(plain), "}") + ",\n  \"unused\": \"\x00\"\n}"), "malformed JSON"},
		{"multiple_objects", append(append([]byte{}, plain...), plain...), "malformed JSON"},
		{"nested_fields_only", jsonBytes(map[string]any{"nested": flat}), "languageVersion"},
		{"missing_protocol", change("protocolVersion", nil, true), "protocolVersion"},
		{"wrong_protocol", change("protocolVersion", "0.0", false), "protocolVersion"},
		{"missing_catalog", change("catalogSchemaVersion", nil, true), "catalogSchemaVersion"},
		{"wrong_catalog", change("catalogSchemaVersion", "0.0", false), "catalogSchemaVersion"},
		{"missing_codec", change("codecFingerprint", nil, true), "codecFingerprint"},
		{"wrong_codec", change("codecFingerprint", strings.Repeat("0", 64), false), "codecFingerprint"},
		{"missing_macro", change("macroFingerprint", nil, true), "MACRO_FINGERPRINT_MISSING"},
		{"wrong_schema_same_language", change("schemaFingerprint", strings.Repeat("0", 64), false), "schemaFingerprint"},
		{"not_json", []byte("not JSON"), "malformed JSON"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			original := p15InstallRead(t, f.compiler)
			originalSum := p15InstallRead(t, f.compiler+".sha256")
			t.Cleanup(func() {
				p15InstallWrite(t, f.compiler, original)
				p15InstallWrite(t, f.compiler+".sha256", originalSum)
			})
			probeDir := t.TempDir()
			payload := filepath.Join(probeDir, "capabilities.json")
			marker := filepath.Join(probeDir, "observed.txt")
			p15InstallWrite(t, payload, tc.payload)
			wrapper := "#!/bin/sh\nif [ \"${1:-}\" = '--capabilities-json' ]; then\n printf '%s\\n' observed >> " + p15ExtractedShellQuote(filepath.ToSlash(marker)) + "\n cat " + p15ExtractedShellQuote(filepath.ToSlash(payload)) + "\n exit 0\nfi\nexec " + p15ExtractedShellQuote(filepath.ToSlash(realBinary)) + " \"$@\"\n"
			p15InstallWrite(t, f.compiler, []byte(wrapper))
			p15InstallWrite(t, f.compiler+".sha256", []byte(p15ExtractedHash(t, f.compiler)+"\n"))
			output, err := p15InstallRun(t, doctor, f.env)
			observed := p15InstallRead(t, marker)
			controls, _ := f.receipt["capability_controls"].([]any)
			f.receipt["capability_controls"] = append(controls, map[string]any{"name": tc.name, "payload_base64": base64.StdEncoding.EncodeToString(tc.payload), "output_base64": base64.StdEncoding.EncodeToString([]byte(output)), "success": err == nil, "response_observations": bytes.Count(observed, []byte("observed"))})
			if !bytes.Contains(observed, []byte("observed")) {
				t.Fatal("bad capability response was not actually requested")
			}
			if err == nil || !strings.Contains(output, tc.diagnostic) {
				t.Fatalf("extracted doctor accepted or misdiagnosed adversarial %s: err=%v expected=%s\n%s", tc.name, err, tc.diagnostic, output)
			}
			t.Logf("RP44_CAPABILITY_REJECTION name=%s payload_sha256=%x response_observed=%d", tc.name, sha256.Sum256(tc.payload), bytes.Count(observed, []byte("observed")))
		})
	}
	if output, err := p15InstallRun(t, doctor, f.env); err != nil {
		t.Fatalf("healthy actual compiler did not recover after every protocol control: %v\n%s", err, output)
	}
}
