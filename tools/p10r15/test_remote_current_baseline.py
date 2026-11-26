"""Real filesystem/Git controls for a current installed before-state.

All fixtures are synthetic and private to this test. No compiler, Pack module,
Go tool, signer, live installation or historical acceptance is executed.
"""
import copy
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

import remote_current_baseline as current
import remote_stage_baseline as baseline
from remote_release_gates import PROTECTED, TOOL_PATHS


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def git(path, *args):
    return subprocess.check_output(["git", "-C", str(path), *args],
                                   stderr=subprocess.PIPE, text=True, timeout=15).strip()


def repository(path, origin):
    path.mkdir(parents=True)
    git(path, "init", "-q")
    git(path, "config", "user.name", "Current baseline fixture")
    git(path, "config", "user.email", "current-baseline@invalid.example")
    (path / "README").write_text("Explicitly synthetic immutable source.\n")
    git(path, "add", "README")
    git(path, "commit", "-q", "-m", "Synthetic source")
    git(path, "remote", "add", "origin", origin)
    return {"commit": git(path, "rev-parse", "HEAD"),
            "tree": git(path, "rev-parse", "HEAD^{tree}"), "origin": origin}


class CurrentInstalledBaseline(unittest.TestCase):
    def setUp(self):
        temporary = tempfile.TemporaryDirectory(prefix="p15-current-baseline-")
        self.addCleanup(temporary.cleanup)
        self.root = Path(temporary.name)
        self.live = self.root / "live"
        self.label = "p15-synthetic-current"
        self.runtime = self.live / self.label / "runtime"
        compiler = repository(self.runtime / "src", current.CANONICAL)
        docs = repository(self.runtime / "docs", current.DOCS)
        skill = repository(self.live / "source" / self.label, current.CANONICAL)
        self.release = {
            "label": self.label, "compilerCommit": compiler["commit"],
            "docsCommit": docs["commit"], "skillSourceHead": skill["commit"],
            "sourceBranch": "agent/synthetic-current", "schemaFingerprint": "4" * 64,
            "codecFingerprint": "5" * 64, "macroFingerprint": "6" * 64,
            "compilerSHA256": digest(b"synthetic compiler"),
            "migrationSHA256": digest(b"synthetic migrator")}
        for relative in sorted(set(PROTECTED) | TOOL_PATHS):
            path = self.live / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            raw = (json.dumps(self.release).encode() if relative == "active.json" else
                   b"legacy\n" if relative == "config/default-language-mode" else
                   ("synthetic current file " + relative).encode())
            path.write_bytes(raw)
        self.linter = digest((self.live / "bin/cherri-shortcut-lint.py").read_bytes())
        self.addCleanup(patch.stopall)
        patch.object(current, "LINTER_SHA256", self.linter).start()
        patch.dict(baseline.ORIGINAL_LIVE_HASHES,
                   {"bin/cherri-shortcut-lint.py": self.linter}).start()
        (self.live / "bin/cherri-remote-builder").symlink_to("cherri-remote-legacy.sh")
        self.alias = self.root / "public-home"
        self.alias.symlink_to(self.live, target_is_directory=True)
        (self.runtime / "bin").mkdir()
        for name, raw, key in (("cherri", b"synthetic compiler", "compilerSHA256"),
                               ("cherri-migrate", b"synthetic migrator", "migrationSHA256")):
            (self.runtime / "bin" / name).write_bytes(raw)
            (self.runtime / "bin" / (name + ".commit")).write_text(compiler["commit"] + "\n")
            (self.runtime / "bin" / (name + ".sha256")).write_text(self.release[key] + "\n")
        pure = self.live / "pure-upstream"
        pure_engine = repository(pure / "src", current.PURE)
        pure_docs = repository(pure / "docs", current.PURE_DOCS)
        hashes = {}
        for relative in sorted(current.PURE_FILES | {
                "bin/pure-cherri-service.sh", "bin/cherri-with-actions.sh"}):
            path = pure / relative
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(("synthetic separate Pure " + relative).encode())
            if relative in current.PURE_FILES:
                hashes[relative] = digest(path.read_bytes())
        (pure / "installation.json").write_text(json.dumps({
            "schema": "cherri-pure-cli-installation/1", "freshBuild": True,
            "historicalBinaryRestored": False, "files": hashes,
            "engine": pure_engine, "docs": pure_docs}))
        pack = pure / "apple-actions-pack"
        pack.mkdir()
        (pack / "manifest.json").write_text('{"synthetic":true}\n')
        (pack / "checksums.sha256").write_text(
            digest((pack / "manifest.json").read_bytes()) + "  manifest.json\n")
        self.record = current.capture(self.live, self.alias)
        self.receipt = self.root / "current.json"
        self.save()

    def save(self):
        self.receipt.write_text(json.dumps(self.record))
        self.sha = digest(self.receipt.read_bytes())

    def load(self):
        return baseline.wrapper_templates(
            self.live, None, None, None, None, installed=True,
            current_baseline=self.receipt, current_sha=self.sha)

    def test_actual_current_mode_is_read_only_and_keeps_history_unavailable(self):
        before = {str(p): p.read_bytes() for p in self.root.rglob("*")
                  if p.is_file() and not p.is_symlink()}
        hashes, witness, values, templates = self.load()
        self.assertEqual(set(hashes), set(PROTECTED) | TOOL_PATHS)
        self.assertEqual(witness["kind"], "CURRENT_INSTALLED")
        self.assertTrue(witness["historyUnavailable"])
        self.assertFalse(witness["acceptanceGranted"])
        self.assertEqual(witness["tuple"], self.release)
        self.assertEqual(witness["sha256"], self.sha)
        for role in ("runner", "legacy"):
            relative = "bin/cherri-remote-" + role + ".sh"
            self.assertEqual(values[role], (self.live / relative).read_text())
            self.assertEqual(templates[role]["sha256"], hashes[relative])
        self.assertEqual(before, {str(p): p.read_bytes() for p in self.root.rglob("*")
                                 if p.is_file() and not p.is_symlink()})

    def test_each_protected_live_file_remains_bound(self):
        for relative in sorted(set(PROTECTED) | TOOL_PATHS):
            with self.subTest(file=relative):
                path = self.live / relative
                raw = path.read_bytes()
                path.write_bytes(raw + b"changed")
                try:
                    with self.assertRaises((ValueError, json.JSONDecodeError)):
                        self.load()
                    self.assertEqual(path.read_bytes(), raw + b"changed")
                finally:
                    path.write_bytes(raw)

    def test_missing_file_and_permissions_change_are_rejected(self):
        path = self.live / "bin/cherri-remote-runner.sh"
        raw, mode = path.read_bytes(), path.stat().st_mode & 0o777
        path.unlink()
        with self.assertRaises(FileNotFoundError):
            self.load()
        path.write_bytes(raw)
        path.chmod(mode ^ 0o100)
        with self.assertRaisesRegex(ValueError, "stale"):
            self.load()

    def test_missing_extra_and_tampered_captured_inventory_are_rejected(self):
        original = copy.deepcopy(self.record)
        for fault in ("missing", "extra", "digest"):
            with self.subTest(fault=fault):
                self.record = copy.deepcopy(original)
                entries = self.record["state"]["protected"]
                if fault == "missing":
                    entries.pop("active.json")
                elif fault == "extra":
                    entries["extra"] = entries["active.json"]
                else:
                    entries["active.json"]["sha256"] = "0" * 64
                self.save()
                with self.assertRaisesRegex(ValueError, "stale"):
                    self.load()

    def test_receipt_digest_and_duplicate_keys_are_checked(self):
        self.receipt.write_text(self.receipt.read_text() + "\n")
        with self.assertRaisesRegex(ValueError, "digest differs"):
            self.load()
        self.receipt.write_text('{"kind":"forged",' + self.receipt.read_text()[1:])
        self.sha = digest(self.receipt.read_bytes())
        with self.assertRaisesRegex(ValueError, "Duplicate JSON key"):
            self.load()

    def test_history_claims_and_foreign_home_are_rejected(self):
        original = copy.deepcopy(self.record)
        for field, value in (("kind", "STAGED_NOT_ACTIVATED"),
                             ("historyUnavailable", False), ("historicalAcceptance", True),
                             ("livePromotion", True)):
            with self.subTest(field=field):
                self.record = dict(original, **{field: value})
                self.save()
                with self.assertRaisesRegex(ValueError, "overclaims history"):
                    self.load()
        self.record = copy.deepcopy(original)
        self.record["state"]["liveHome"] = str(self.root / "other")
        self.save()
        with self.assertRaisesRegex(ValueError, "another home"):
            self.load()

    def test_registered_and_public_alias_changes_are_rejected(self):
        for path, target in ((self.live / "bin/cherri-remote-builder", "other.sh"),
                             (self.alias, self.root / "elsewhere")):
            with self.subTest(alias=str(path)):
                old = os.readlink(path)
                path.unlink()
                path.symlink_to(target)
                try:
                    with self.assertRaisesRegex(ValueError, "alias differs"):
                        self.load()
                    self.assertEqual(os.readlink(path), str(target))
                finally:
                    path.unlink()
                    path.symlink_to(old)

    def test_dirty_or_foreign_source_and_borrowed_objects_are_rejected(self):
        source = self.runtime / "src"
        (source / "unexpected").write_text("dirty")
        with self.assertRaisesRegex(ValueError, "identity/cleanliness"):
            self.load()
        (source / "unexpected").unlink()
        git(source, "remote", "set-url", "origin", "https://example.invalid/other.git")
        with self.assertRaisesRegex(ValueError, "identity/cleanliness"):
            self.load()
        git(source, "remote", "set-url", "origin", current.CANONICAL)
        alternate = source / ".git/objects/info/alternates"
        alternate.write_text("/not-an-owned-object-store\n")
        with self.assertRaisesRegex(ValueError, "borrows"):
            self.load()

    def test_each_runtime_binary_and_sidecar_remains_bound(self):
        for path in sorted((self.runtime / "bin").iterdir()):
            with self.subTest(path=path.name):
                original = path.read_bytes()
                path.write_bytes(original + b"changed")
                try:
                    with self.assertRaisesRegex(ValueError, "runtime binary/sidecar"):
                        self.load()
                finally:
                    path.write_bytes(original)

    def test_pure_and_opt_in_pack_are_preserved(self):
        pure = self.live / "pure-upstream"
        for relative in ("bin/cherri", "lib/service.py", "bin/pure-cherri-service.sh",
                         "apple-actions-pack/manifest.json"):
            with self.subTest(path=relative):
                path = pure / relative
                original = path.read_bytes()
                path.write_bytes(original + b"changed")
                try:
                    with self.assertRaisesRegex(ValueError, "Pure file|Pack checksum|stale"):
                        self.load()
                finally:
                    path.write_bytes(original)

    def test_current_options_cannot_mix_with_historical_or_initial_mode(self):
        for installed, previous, previous_sha, runner, legacy, record, sha in (
                (False, None, None, None, None, self.receipt, self.sha),
                (True, self.receipt, self.sha, None, None, self.receipt, self.sha),
                (True, None, None, self.receipt, None, self.receipt, self.sha),
                (True, None, None, None, self.receipt, self.receipt, self.sha),
                (True, None, None, None, None, None, self.sha),
                (True, None, None, None, None, self.receipt, None)):
            with self.subTest(installed=installed, previous=previous, runner=runner,
                              legacy=legacy, record=record, sha=sha):
                with self.assertRaisesRegex(ValueError, "Current baseline"):
                    baseline.wrapper_templates(self.live, previous, previous_sha, runner, legacy,
                                               installed=installed, current_baseline=record,
                                               current_sha=sha)

    def test_capture_output_cannot_write_through_the_live_alias(self):
        target = self.alias / "unexpected-baseline.json"
        argv = ["remote_current_baseline.py", "--live-home", str(self.live),
                "--public-alias", str(self.alias), "--output", str(target)]
        with patch.object(sys, "argv", argv):
            with self.assertRaisesRegex(ValueError, "outside live"):
                current.main()
        self.assertFalse(target.exists())

    def test_captured_source_and_pure_inventory_cannot_be_relabelled(self):
        original = copy.deepcopy(self.record)
        for section, key in (("sources", "compiler"), ("runtimeFiles", "cherri"),
                             ("pure", "files")):
            with self.subTest(section=section):
                self.record = copy.deepcopy(original)
                self.record["state"][section].pop(key)
                self.save()
                with self.assertRaisesRegex(ValueError, "stale"):
                    self.load()


if __name__ == "__main__":
    unittest.main(verbosity=2)
