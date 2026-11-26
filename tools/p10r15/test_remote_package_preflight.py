"""Exercise the real committed source-package path at the promotion boundary.

Only unrelated deployment/report prerequisites are synthetic here. Package
creation, committed-file reads, archive validation and the package-kind contract
use the production implementation. This does not authorize a live deployment.
"""
import copy
from contextlib import ExitStack
import json
import os
from pathlib import Path
import shutil
import stat
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import package_skill
import promote_remote_release as promotion
import stage_remote_release as staging


def git(repo, *args):
    return subprocess.check_output(["git", "--no-replace-objects", "-C", str(repo), *args],
                                   stderr=subprocess.PIPE, text=True, timeout=30).strip()


def commit_fixture(repo, origin):
    git(repo, "init", "-q")
    git(repo, "config", "user.name", "Cherri package test")
    git(repo, "config", "user.email", "package-test@invalid.example")
    git(repo, "config", "core.autocrlf", "false")
    git(repo, "remote", "add", "origin", origin)
    git(repo, "add", ".")
    git(repo, "commit", "-q", "-m", "Create explicitly synthetic package fixture")
    return git(repo, "rev-parse", "HEAD")


class PairedPackagePreflight(unittest.TestCase):
    """Actual Go/ZIP/Git plumbing with synthetic main programs, not Cherri acceptance."""

    @classmethod
    def setUpClass(cls):
        cls.temporary = tempfile.TemporaryDirectory(prefix="p15-paired-package-test-")
        cls.addClassCleanup(cls.temporary.cleanup)
        cls.root = Path(cls.temporary.name)
        cls.go = shutil.which("go")
        if not cls.go:
            raise RuntimeError("The real paired-package regression requires Go")
        cls.source = cls.root / "synthetic-go-source"
        (cls.source / "cmd/cherri-migrate").mkdir(parents=True)
        (cls.source / "go.mod").write_text("module github.com/electrikmilk/cherri\n\ngo 1.24.0\n")
        (cls.source / "main.go").write_text('package main\nfunc main(){println("synthetic compiler role fixture")}\n')
        (cls.source / "cmd/cherri-migrate/main.go").write_text(
            'package main\nfunc main(){println("synthetic migration role fixture")}\n')
        cls.revision = commit_fixture(cls.source, promotion.CANONICAL)
        cls.binaries = {}
        env = dict(os.environ, GOTOOLCHAIN="local", GOWORK="off", GIT_NO_REPLACE_OBJECTS="1",
                   GOFLAGS="-p=2", GOMAXPROCS="2", CGO_ENABLED="0")
        for name, main in (("cherri", "."), ("cherri-migrate", "./cmd/cherri-migrate")):
            path = cls.root / name
            subprocess.run([cls.go, "build", "-buildvcs=true", "-trimpath", "-o", str(path), main],
                           cwd=cls.source, env=env, check=True, capture_output=True, timeout=120)
            cls.binaries[name] = path
        cls.other_platform = cls.root / "migration-other-platform"
        architecture = subprocess.check_output([cls.go, "env", "GOARCH"], env=env, text=True, timeout=30).strip()
        subprocess.run([cls.go, "build", "-buildvcs=true", "-trimpath", "-o", str(cls.other_platform),
                        "./cmd/cherri-migrate"], cwd=cls.source,
                       env=dict(env, GOARCH="amd64" if architecture == "arm64" else "arm64"),
                       check=True, capture_output=True, timeout=120)
        git(cls.source, "commit", "--allow-empty", "-q", "-m", "Create foreign source fixture")
        cls.other_source = cls.root / "migration-other-source"
        subprocess.run([cls.go, "build", "-buildvcs=true", "-trimpath", "-o", str(cls.other_source),
                        "./cmd/cherri-migrate"], cwd=cls.source, env=env,
                       check=True, capture_output=True, timeout=120)
        git(cls.source, "reset", "--hard", cls.revision)
        cls.docs = cls.root / "synthetic-docs"
        cls.docs.mkdir()
        (cls.docs / "README.md").write_text("Explicitly synthetic docs checkout for package plumbing.\n")
        cls.docs_revision = commit_fixture(cls.docs, promotion.DOCS)
        repo = Path(os.environ.get("CHERRI_PROMOTION_PACKAGE_REPO",
                                   str(Path(__file__).resolve().parents[2])))
        files, pins = package_skill.committed_files(repo, git(repo, "rev-parse", "HEAD"))
        cls.skill = cls.root / "committed-skill-fixture"
        for name, (data, mode) in files.items():
            relative = name.removeprefix(package_skill.PREFIX)
            path = cls.skill / ("LICENSE" if relative == "LICENSE.cherri" else
                                "skills/cherri-shortcuts/" + relative)
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            path.chmod(stat.S_IMODE(mode))
        pins.update(compilerCommit=cls.revision, docsCommit=cls.docs_revision)
        cls.pins = pins
        (cls.skill / "skills/cherri-shortcuts/compatibility-manifest.json").write_bytes(package_skill.canonical(pins))
        cls.skill_revision = commit_fixture(cls.skill, promotion.CANONICAL)

    def test_staging_inspects_actual_roles_source_and_platform(self):
        for name, migration, message in (("wrong_role", self.binaries["cherri"], "main package mismatch"),
                ("wrong_source", self.other_source, "revision mismatch"),
                ("wrong_platform", self.other_platform, "platforms differ")):
            with self.subTest(rejected=name):
                with self.assertRaisesRegex(ValueError, message):
                    staging.inspect_runtime_pair(self.binaries["cherri"], migration, self.revision, self.go)

    def test_real_paired_archive_passes_promotion_package_boundary(self):
        with tempfile.TemporaryDirectory(prefix="p15-real-pair-preflight-") as temporary:
            root = Path(temporary)
            stage, live = root / "stage", root / "live"
            candidate = stage / "remote-home"
            label = "p15-synthetic-paired-package"
            runtime = candidate / label / "runtime"
            (runtime / "bin").mkdir(parents=True)
            (candidate / "source").mkdir()
            staging.checked_clone(self.skill, candidate / "source" / label,
                                  self.skill_revision, promotion.CANONICAL)
            staging.checked_clone(self.source, runtime / "src", self.revision, promotion.CANONICAL)
            staging.checked_clone(self.docs, runtime / "docs", self.docs_revision, promotion.DOCS)
            release = {"label": label, "skillSourceHead": self.skill_revision,
                       "compilerCommit": self.revision, "docsCommit": self.docs_revision,
                       "sourceBranch": "agent/synthetic-package-fixture",
                       **{key: self.pins[key] for key in
                          ("schemaFingerprint", "codecFingerprint", "macroFingerprint")}}
            compiler_data, migration_data, provenance = staging.inspect_runtime_pair(
                self.binaries["cherri"], self.binaries["cherri-migrate"], self.revision, self.go)
            staging.write_runtime_pair(runtime, compiler_data, migration_data, self.revision, provenance)
            for name, key in (("cherri", "compilerSHA256"), ("cherri-migrate", "migrationSHA256")):
                raw = self.binaries[name].read_bytes()
                release[key] = package_skill.digest(raw)
                self.assertEqual((runtime / "bin" / name).read_bytes(), raw)
                self.assertEqual((runtime / "bin" / name).stat().st_mode & 0o777, 0o700)
                self.assertEqual((runtime / "bin" / (name + ".commit")).read_text(), self.revision + "\n")
                self.assertEqual((runtime / "bin" / (name + ".sha256")).read_text(), release[key] + "\n")
            for home in (live, candidate):
                for relative in promotion.PROTECTED:
                    target = home / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(b"legacy\n" if relative == promotion.PROTECTED[4]
                                       else b"synthetic protected unit fixture\n")
                (home / promotion.LINK).symlink_to(promotion.LINK_TARGET)
            (live / "tools/go/bin").mkdir(parents=True)
            (live / "tools/go/bin/go").symlink_to(self.go)
            (candidate / "tools").symlink_to(live / "tools", target_is_directory=True)
            (candidate / "active.json").write_bytes(package_skill.canonical(release))
            reports = {"stage": {"tuple": release,
                       "liveHashesBefore": {p: promotion.sha((live / p).read_bytes()) for p in promotion.PROTECTED},
                       "candidateHashes": {p: promotion.sha((candidate / p).read_bytes()) for p in promotion.PROTECTED}}}
            paths = {}
            committed, _ = package_skill.committed_files(self.skill, self.skill_revision)
            source_names = set(committed) | {package_skill.RECEIPT}
            runtime_names = {package_skill.PREFIX + "bin/" + name + suffix
                             for name in ("cherri", "cherri-migrate")
                             for suffix in ("", ".commit", ".sha256")}
            for kind, expected_names in (("source", source_names),
                                         ("runtime", source_names | runtime_names)):
                kwargs = {"compiler": self.binaries["cherri"], "migrator": self.binaries["cherri-migrate"],
                          "go": self.go} if kind == "runtime" else {}
                files, receipt = package_skill.expected_package(self.skill, self.skill_revision, kind, **kwargs)
                self.assertEqual(set(files), expected_names)
                archive = root / (kind + ".zip")
                package_skill.create_archive(archive, files)
                paths[kind + "Archive"] = archive
                reports[kind + "Integrity"] = {"archiveSha256": promotion.sha(archive.read_bytes()),
                                                "runtime": receipt["runtime"]}
            # Only unrelated live deployment/signing evidence is modeled. Real
            # checkout, paired tool, archive and sidecar checks are not patched.
            with patch.object(promotion, "load_evidence", return_value=({}, paths, reports)), \
                    patch.object(promotion, "validate_reports"):
                promotion.preflight(stage, live, root / "unused-unit-evidence")
                guard = [sys.executable, "-B", "-c", "import json,sys\n" +
                         staging.immutable_release_reader(release, {}), str(candidate / "active.json")]
                observed = subprocess.run(guard, capture_output=True, text=True, timeout=30)
                self.assertEqual(observed.returncode, 0, observed.stderr)
                for suffix, changed, message in (("", b"changed migration bytes", "migration bytes"),
                        (".commit", b"0" * 40 + b"\n", "migration commit"),
                        (".sha256", b"0" * 64 + b"\n", "migration hash")):
                    with self.subTest(migration_corruption=suffix or "binary"):
                        target = runtime / "bin" / ("cherri-migrate" + suffix)
                        original = target.read_bytes()
                        try:
                            target.write_bytes(changed)
                            with self.assertRaisesRegex(ValueError, message):
                                promotion.preflight(stage, live, root / "unused-unit-evidence")
                            if not suffix:
                                observed = subprocess.run(guard, capture_output=True, text=True, timeout=30)
                                self.assertNotEqual(observed.returncode, 0)
                                self.assertIn("IMMUTABLE_MIGRATION_MISMATCH", observed.stderr)
                        finally:
                            target.write_bytes(original)
                promotion.preflight(stage, live, root / "unused-unit-evidence")
                observed = subprocess.run(guard, capture_output=True, text=True, timeout=30)
                self.assertEqual(observed.returncode, 0, observed.stderr)


class SourcePackagePreflight(unittest.TestCase):
    def test_real_source_archive_reaches_required_runtime_boundary(self):
        repo = Path(os.environ.get("CHERRI_PROMOTION_PACKAGE_REPO",
                                   str(Path(__file__).resolve().parents[2])))
        commit = subprocess.check_output(
            ["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
        source_files, source_receipt = package_skill.expected_package(repo, commit, "source")
        committed, _ = package_skill.committed_files(repo, commit)
        self.assertEqual(set(source_files), set(committed) | {package_skill.RECEIPT})
        self.assertIsNone(source_receipt["runtime"])
        self.assertNotIn(package_skill.PREFIX + "bin/cherri", source_files)

        with tempfile.TemporaryDirectory(prefix="p15-package-preflight-") as temporary:
            root = Path(temporary)
            stage, live = root / "stage", root / "live"
            candidate = stage / "remote-home"
            label = "unit-package-boundary"
            release = {"label": label, "skillSourceHead": commit}
            for home in (live, candidate):
                for relative in promotion.PROTECTED:
                    target = home / relative
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.write_bytes(b"legacy\n" if relative == promotion.PROTECTED[4]
                                       else b"synthetic protected unit fixture\n")
                (home / promotion.LINK).symlink_to(promotion.LINK_TARGET)
            (live / "tools").mkdir()
            (candidate / "tools").symlink_to(live / "tools", target_is_directory=True)
            skill = candidate / "source" / label
            skill.parent.mkdir(parents=True)
            skill.symlink_to(repo.resolve(), target_is_directory=True)
            (candidate / "active.json").write_text(json.dumps(release))
            archive = root / "source.zip"
            package_skill.create_archive(archive, source_files)
            reports = {
                "stage": {"tuple": release,
                          "liveHashesBefore": {p: promotion.sha((live / p).read_bytes())
                                               for p in promotion.PROTECTED},
                          "candidateHashes": {p: promotion.sha((candidate / p).read_bytes())
                                              for p in promotion.PROTECTED}},
                "sourceIntegrity": {"archiveSha256": promotion.sha(archive.read_bytes()),
                                    "runtime": None},
            }
            paths = {"sourceArchive": archive, "runtimeArchive": root / "runtime.zip"}
            with ExitStack() as stack:
                stack.enter_context(patch.object(
                    promotion, "load_evidence", return_value=({}, paths, reports)))
                stack.enter_context(patch.object(promotion, "validate_reports"))
                stack.enter_context(patch.object(promotion, "verify_runtime"))
                checked = stack.enter_context(patch.object(
                    promotion, "verify_archive", wraps=package_skill.verify_archive))

                # The absent runtime is an intentional next boundary: source
                # validation must finish first, without a compiler argument.
                with self.subTest("exact_source_then_missing_runtime"):
                    with self.assertRaises(FileNotFoundError) as failure:
                        promotion.preflight(stage, live, root / "unused-unit-evidence")
                    self.assertEqual(Path(failure.exception.filename),
                                     candidate / label / "runtime/bin/cherri")
                    self.assertEqual(checked.call_count, 1)
                    self.assertEqual(checked.call_args.args[0], archive)
                    self.assertEqual(checked.call_args.args[1], source_files)

                with self.subTest("source_runtime_receipt_rejected"):
                    reports["sourceIntegrity"]["runtime"] = {"compilerCommit": "0" * 40}
                    with self.assertRaisesRegex(ValueError, "Binary package provenance differs"):
                        promotion.preflight(stage, live, root / "unused-unit-evidence")
                    reports["sourceIntegrity"]["runtime"] = None

                with self.subTest("rehash_cannot_hide_changed_source_bytes"):
                    changed = copy.deepcopy(source_files)
                    name = package_skill.PREFIX + "SKILL.md"
                    changed[name] = (changed[name][0] + b"\ncausal mutation\n", changed[name][1])
                    archive.unlink()
                    package_skill.create_archive(archive, changed)
                    reports["sourceIntegrity"]["archiveSha256"] = promotion.sha(archive.read_bytes())
                    with self.assertRaisesRegex(ValueError, "archive (size/mode|bytes) mismatch"):
                        promotion.preflight(stage, live, root / "unused-unit-evidence")


if __name__ == "__main__":
    unittest.main(verbosity=2)
