#!/usr/bin/env python3
"""Prepare an immutable Remote Builder tuple without activating it."""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import shutil
import subprocess


def run(args, cwd=None):
    result = subprocess.run([str(x) for x in args], cwd=cwd, check=True,
                            text=True, capture_output=True)
    return result.stdout.strip()


def digest(path):
    return hashlib.sha256(Path(path).read_bytes()).hexdigest()


def clean_head(repo):
    head = run(["git", "rev-parse", "HEAD"], repo)
    if run(["git", "status", "--porcelain"], repo):
        raise ValueError("Release source has uncommitted files: " + str(repo))
    return head


def checked_clone(source, destination, revision, canonical_origin):
    if canonical_origin not in ("https://github.com/davidpovarsky/cherri.git",
                                "https://github.com/davidpovarsky/cherrilang.org.git"):
        raise ValueError("Unexpected immutable clone origin")
    run(["git", "clone", "--no-checkout", "--local", source, destination])
    # Local objects may be hardlinked, but the release must not depend on the
    # temporary source checkout as its future origin or borrowed object store.
    run(["git", "remote", "set-url", "origin", canonical_origin], destination)
    if (destination / ".git/objects/info/alternates").exists():
        raise ValueError("Release clone must not borrow another object store")
    run(["git", "checkout", "--detach", revision], destination)
    if clean_head(destination) != revision:
        raise ValueError("Clone revision mismatch")


def inspect_runtime_pair(compiler, migrator, revision, go):
    """Inspect both exact snapshots with the package verifier's source/role rules."""
    from package_skill import inspect_binary
    compiler_data, provenance = inspect_binary(compiler, revision, go)
    migration_data, migration = inspect_binary(
        migrator, revision, go, "github.com/electrikmilk/cherri/cmd/cherri-migrate")
    if any(provenance[key] != migration[key] for key in ("goos", "goarch")):
        raise ValueError("Compiler and migration tool platforms differ")
    provenance["migration"] = migration
    return compiler_data, migration_data, provenance


def write_runtime_pair(runtime, compiler_data, migration_data, revision, provenance):
    for name, data, identity in (("cherri", compiler_data, provenance),
                                 ("cherri-migrate", migration_data, provenance["migration"])):
        if hashlib.sha256(data).hexdigest() != identity["sha256"]:
            raise ValueError("Verified runtime snapshot differs: " + name)
        tool = runtime / "bin" / name
        tool.write_bytes(data)
        tool.chmod(0o700)
        (runtime / "bin" / (name + ".commit")).write_text(revision + "\n")
        (runtime / "bin" / (name + ".sha256")).write_text(identity["sha256"] + "\n")


def immutable_release_reader(expected_tuple, tool_hashes):
    return "\n".join([
        "from pathlib import Path",
        "import hashlib,subprocess",
        "x=json.load(open(sys.argv[1]))",
        "expected=" + json.dumps(expected_tuple, sort_keys=True),
        "if any(x.get(key)!=value for key,value in expected.items()): raise SystemExit('IMMUTABLE_RELEASE_MISMATCH')",
        "base=Path(sys.argv[1]).parent",
        "runtime=base/expected['label']/'runtime'",
        "tool_hashes=" + json.dumps(tool_hashes, sort_keys=True),
        "for path,expected_hash in tool_hashes.items():",
        "    if hashlib.sha256((base/path).read_bytes()).hexdigest()!=expected_hash: raise SystemExit('TOOLCHAIN_DEPENDENCY_MISMATCH')",
        "for repo,revision in [(base/'source'/expected['label'],expected['skillSourceHead']),(runtime/'src',expected['compilerCommit']),(runtime/'docs',expected['docsCommit'])]:",
        "    head=subprocess.check_output(['git','rev-parse','HEAD'],cwd=repo,text=True).strip()",
        "    dirty=subprocess.check_output(['git','status','--porcelain'],cwd=repo,text=True).strip()",
        "    if head!=revision or dirty: raise SystemExit('IMMUTABLE_SOURCE_MISMATCH')",
        "actual=hashlib.sha256((runtime/'bin/cherri').read_bytes()).hexdigest()",
        "if actual!=expected['compilerSHA256']: raise SystemExit('IMMUTABLE_COMPILER_MISMATCH')",
        "actual=hashlib.sha256((runtime/'bin/cherri-migrate').read_bytes()).hexdigest()",
        "if actual!=expected['migrationSHA256']: raise SystemExit('IMMUTABLE_MIGRATION_MISMATCH')",
        "print('|'.join(x[k] for k in ('label','compilerCommit','docsCommit','schemaFingerprint','sourceBranch','skillSourceHead')))",
    ])


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("repo", "compiler-source", "compiler", "migrator", "docs", "live-home", "stage"):
        parser.add_argument("--" + name, type=Path, required=True)
    parser.add_argument("--go", required=True)
    parser.add_argument("--label", required=True)
    parser.add_argument("--branch", required=True)
    args = parser.parse_args()
    if not re.fullmatch(r"p15-[a-z0-9-]+", args.label):
        raise ValueError("Expected a unique p15 release label")
    if not re.fullmatch(r"agent/[A-Za-z0-9/_-]+", args.branch):
        raise ValueError("Invalid release branch label")
    args.repo, args.compiler_source, args.docs, args.live_home, args.stage = [
        path.resolve() for path in
        (args.repo, args.compiler_source, args.docs, args.live_home, args.stage)]
    if args.stage.exists():
        raise ValueError("Stage already exists; existing evidence is preserved")
    if not args.live_home.is_dir():
        raise ValueError("Invalid live directory")
    for protected_root in (args.live_home, args.repo, args.compiler_source, args.docs):
        if args.stage == protected_root or args.stage.is_relative_to(protected_root) or protected_root.is_relative_to(args.stage):
            raise ValueError("Staging must be separate from live and source trees")
    if Path(args.go).resolve() != (args.live_home / "tools/go/bin/go").resolve():
        raise ValueError("Expected the observed Remote Builder Go toolchain")
    source_sha = clean_head(args.compiler_source)
    skill_sha = clean_head(args.repo)
    docs_sha = clean_head(args.docs)
    manifest_path = args.repo / "skills/cherri-shortcuts/compatibility-manifest.json"
    manifest = json.loads(manifest_path.read_text())
    if manifest.get("compilerRepo") != "https://github.com/davidpovarsky/cherri.git" or manifest.get("docsRepo") != "https://github.com/davidpovarsky/cherrilang.org.git":
        raise ValueError("Committed manifest points to an unexpected project")
    if manifest["compilerCommit"] != source_sha or manifest["docsCommit"] != docs_sha:
        raise ValueError("Committed Skill/compiler/docs tuple does not agree")
    # Reuse the package verifier's real embedded VCS provenance contract.
    binary_data, migration_data, provenance = inspect_runtime_pair(
        args.compiler, args.migrator, source_sha, args.go)
    capabilities = json.loads(run([args.compiler, "--capabilities-json"]))
    for field in ("languageVersion", "catalogSchemaVersion", "schemaFingerprint",
                  "codecFingerprint", "macroFingerprint"):
        if capabilities.get(field) != manifest.get(field):
            raise ValueError("Compiler capability mismatch: " + field)
    protected = {
        "active.json": "ee9aac247c568fb743d4e505bcaaebe224bfca843d1091438701bd18273807d0",
        "bin/cherri-remote-runner.sh": "7ae689e9d494e7ddb9a7b9abfb5c5d96bb4321b069dc2847aec07271122cb954",
        "bin/cherri-remote-legacy.sh": "734507b7a981d7d4185fb3582bd9601d958db013dcd7b12b13201d0d3b0e0b4c",
        "bin/cherri-shortcut-lint.py": "51ffca98be2275f4015931e15d73251fff7f7041fb323c9b7aa6973eb57a454f",
    }
    for name, expected in protected.items():
        if digest(args.live_home / name) != expected:
            raise ValueError("Concurrent/unreviewed live change: " + name)
    mode = (args.live_home / "config/default-language-mode").read_bytes()
    if mode.strip() != b"legacy":
        raise ValueError("The observed explicit Legacy preference changed")
    protected["config/default-language-mode"] = hashlib.sha256(mode).hexdigest()
    tool_dir = Path(run([args.go, "env", "GOTOOLDIR"])).resolve()
    if not tool_dir.is_relative_to((args.live_home / "tools/go").resolve()):
        raise ValueError("Unexpected active Go tool directory")
    tool_paths = [args.live_home / "tools/go/bin/go", args.live_home / "tools/go/VERSION"]
    tool_paths += [tool_dir / name for name in ("compile", "link", "asm")]
    tool_hashes = {str(path.relative_to(args.live_home)): digest(path) for path in tool_paths}
    protected.update(tool_hashes)
    builder = args.live_home / "bin/cherri-remote-builder"
    if not builder.is_symlink() or builder.resolve() != (args.live_home / "bin/cherri-remote-legacy.sh").resolve():
        raise ValueError("Registered entrypoint differs from the observed Legacy route")
    os.umask(0o077)
    args.stage.mkdir(parents=True)
    candidate = args.stage / "remote-home"
    for name in ("bin", "config", "jobs", "outputs", "tmp", "source",
                 args.label + "/runtime/bin"):
        (candidate / name).mkdir(parents=True, exist_ok=True)
    source = candidate / "source" / args.label
    runtime = candidate / args.label / "runtime"
    checked_clone(args.repo, source, skill_sha, manifest["compilerRepo"])
    checked_clone(args.compiler_source, runtime / "src", source_sha, manifest["compilerRepo"])
    checked_clone(args.docs, runtime / "docs", docs_sha, manifest["docsRepo"])
    write_runtime_pair(runtime, binary_data, migration_data, source_sha, provenance)
    (candidate / "tools").symlink_to(args.live_home / "tools", target_is_directory=True)
    (candidate / "config/default-language-mode").write_bytes(mode)
    runner_text = (args.live_home / "bin/cherri-remote-runner.sh").read_text()
    # Preserve the real read/compile interfaces. Sync revalidates this immutable
    # release; it cannot fetch a moving P10/P9 branch or activate a fallback.
    start = runner_text.index("# A clone managed solely by this runner.")
    end = runner_text.index("active_env() {", start)
    sync = """sync_release() {
  case "__DOLLAR__{1:-}" in
    ""|--require-p10|--require-p15) ;;
    *) die "Unknown immutable-sync option" ;;
  esac
  active_env
  say "IMMUTABLE_RELEASE_VERIFIED __DOLLAR__LABEL __DOLLAR__COMPILER_COMMIT"
  cat "__DOLLAR__BASE/active.json"
}

""".replace("__DOLLAR__", "$")
    runner_text = runner_text[:start] + sync + runner_text[end:]
    runner_text = runner_text.replace("sync [--require-p10]", "sync [--require-p10|--require-p15]")
    runner_text = runner_text.replace(
        "P10 is enabled only after source Skill, compiler and docs agree and doctor passes.",
        "Sync verifies the installed immutable tuple. New releases use a separately validated package.")
    expected_tuple = dict(label=args.label, compilerCommit=source_sha,
                          docsCommit=docs_sha, schemaFingerprint=manifest["schemaFingerprint"],
                          codecFingerprint=manifest["codecFingerprint"],
                          macroFingerprint=manifest["macroFingerprint"],
                          compilerSHA256=provenance["sha256"],
                          migrationSHA256=provenance["migration"]["sha256"],
                          sourceBranch=args.branch, skillSourceHead=skill_sha)
    old_reader = "x=json.load(open(sys.argv[1])); print('|'.join(x[k] for k in ('label','compilerCommit','docsCommit','schemaFingerprint','sourceBranch','skillSourceHead')))"
    validator = immutable_release_reader(expected_tuple, tool_hashes)
    if runner_text.count(old_reader) != 1:
        raise ValueError("Observed runner active-release reader changed")
    runner_text = runner_text.replace(old_reader, validator)
    # Both language routes sign the exact native plist that passed preflight.
    # A second compile can change UUIDs or observe a concurrent source edit.
    old_preflight = 'preflight="__DOLLAR__BASE/tmp/__DOLLAR__{name}-__DOLLAR__{LABEL}-__DOLLAR__{stamp}-preflight.shortcut"'.replace("__DOLLAR__", "$")
    if runner_text.count(old_preflight) != 1:
        raise ValueError("Observed V2 preflight allocation changed")
    exact_preflight = """preflight="__DOLLAR__(mktemp "__DOLLAR__BASE/tmp/v2-preflight-XXXXXXXX.shortcut")"
    trap 'rm -f -- "__DOLLAR__preflight"' EXIT""".replace("__DOLLAR__", "$")
    runner_text = runner_text.replace(old_preflight, exact_preflight)
    signing_start = runner_text.index('    elif [ "__DOLLAR__LABEL" = \'p9\' ]; then'.replace("__DOLLAR__", "$"))
    signing_end = runner_text.index('    fi\n    sha256sum "__DOLLAR__out"'.replace("__DOLLAR__", "$"), signing_start)
    exact_signing = """    else
      "__DOLLAR__CHERRI_BIN" sign "__DOLLAR__preflight" --source-file="__DOLLAR__src" --output="__DOLLAR__out" --hubsign --share=anyone --no-ansi
""".replace("__DOLLAR__", "$")
    runner_text = runner_text[:signing_start] + exact_signing + runner_text[signing_end:]
    runner_text = runner_text.replace("stamp=$(date -u +%Y%m%dT%H%M%S)\n",
                                      "stamp=$(date -u +%Y%m%dT%H%M%S)-$$\n")
    runner = candidate / "bin/cherri-remote-runner.sh"
    runner.write_text(runner_text)
    runner.chmod(0o700)
    shutil.copy2(args.live_home / "bin/cherri-shortcut-lint.py",
                 candidate / "bin/cherri-shortcut-lint.py")
    legacy_text = (args.live_home / "bin/cherri-remote-legacy.sh").read_text()
    legacy_text = legacy_text.replace(protected["bin/cherri-remote-runner.sh"], digest(runner))
    legacy_text = legacy_text.replace("$BASE/p10/runtime", "$BASE/" + args.label + "/runtime")
    legacy_text = legacy_text.replace("x.get('label')!='p10'", "x.get('label')!='" + args.label + "'")
    legacy_text = legacy_text.replace("agent/cherri-p10-apple-actions-coverage", args.branch)
    legacy_text = legacy_text.replace(chr(36) + "{name}-p10-legacy-",
                                      chr(36) + "{name}-" + args.label + "-legacy-")
    legacy_text = legacy_text.replace("P10", "P10R15")
    output_start = legacy_text.index('workdir="$(dirname -- "$source")"')
    output_end = legacy_text.index('python3 "$VALIDATOR" "$preflight"', output_start)
    exact_output = """preflight="__DOLLAR__(mktemp "__DOLLAR__BASE/tmp/legacy-preflight-XXXXXXXX.shortcut")"
trap 'rm -f -- "__DOLLAR__preflight"' EXIT
"__DOLLAR__COMPILER" "__DOLLAR__source" --legacy --load-all-actions --skip-sign "--output=__DOLLAR__preflight" --no-ansi
[ -s "__DOLLAR__preflight" ] || fail "legacy compiler returned no exact preflight Shortcut"
""".replace("__DOLLAR__", "$")
    legacy_text = legacy_text[:output_start] + exact_output + legacy_text[output_end:]
    old_sign = '"$COMPILER" "$source" --legacy --load-all-actions --hubsign --share=anyone --output="$out" --no-ansi'
    new_sign = '"$COMPILER" sign "$preflight" --source-file="$source" --hubsign --share=anyone --output="$out" --no-ansi'
    if legacy_text.count(old_sign) != 1:
        raise ValueError("Observed Legacy signing command changed")
    legacy_text = legacy_text.replace(old_sign, new_sign)
    legacy = candidate / "bin/cherri-remote-legacy.sh"
    legacy.write_text(legacy_text)
    legacy.chmod(0o700)
    (candidate / "bin/cherri-remote-builder").symlink_to("cherri-remote-legacy.sh")
    active = dict(label=args.label, compilerCommit=source_sha, docsCommit=docs_sha,
                  schemaFingerprint=manifest["schemaFingerprint"],
                  codecFingerprint=manifest["codecFingerprint"],
                  macroFingerprint=manifest["macroFingerprint"],
                  sourceBranch=args.branch, skillSourceHead=skill_sha,
                  compilerSHA256=provenance["sha256"],
                  migrationSHA256=provenance["migration"]["sha256"])
    (candidate / "active.json").write_text(json.dumps(active, indent=2, sort_keys=True) + "\n")
    for script in (runner, legacy):
        run(["bash", "-n", script])
    after = {name: digest(args.live_home / name) for name in protected}
    if after != protected:
        raise ValueError("Live state changed during staging")
    receipt = dict(status="STAGED_NOT_ACTIVATED", candidateHome=str(candidate),
                   tuple=active, binaryProvenance=provenance, livePromotion=False,
                   liveHashesBefore=protected, liveHashesAfter=after,
                   candidateHashes={name: digest(candidate / name) for name in protected},
                   registeredEntrypoint="bin/cherri-remote-builder -> cherri-remote-legacy.sh",
                   externalToolchainDependencies=tool_hashes,
                   activationLayout="Move only source/label and label/runtime, then replace reviewed scripts and active.json; never rename candidate-home over live-home",
                   syncPolicy="Read-only verification of installed immutable tuple; no branch fallback")
    path = args.stage / "staging-receipt.json"
    path.write_text(json.dumps(receipt, indent=2, sort_keys=True) + "\n")
    print(json.dumps(dict(status=receipt["status"], candidateHome=str(candidate),
                         compilerCommit=source_sha, docsCommit=docs_sha, skillCommit=skill_sha)))


if __name__ == "__main__":
    main()
