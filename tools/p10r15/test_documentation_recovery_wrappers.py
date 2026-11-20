#!/usr/bin/env python3
"""Causal collector controls. Synthetic fixtures are not compiler/consumer evidence."""
from __future__ import annotations

import copy
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import time
import unittest

import documentation_recovery_wrappers as subject


class DocumentationWrapperControls(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-doc-wrapper-control-")
        self.root = Path(self.temp.name)

    def tearDown(self):
        self.temp.cleanup()

    def manifest(self):
        return {"languageVersion": "2.0", "catalogSchemaVersion": "3.0",
                "protocolVersion": "1.0", "schemaFingerprint": "a" * 64,
                "codecFingerprint": "b" * 64, "macroFingerprint": "c" * 64}

    def capabilities(self):
        return {**self.manifest(), "actionCount": 2}

    def catalog(self):
        return {"ok": True, "version": "2", "count": 2,
                "actions": [{"name": "first"}, {"name": "second"}]}

    def git(self, repo, *args):
        result = subprocess.run(["git", "--no-replace-objects", "-C", str(repo), *args],
                                capture_output=True, timeout=20)
        self.assertEqual(result.returncode, 0, result.stderr.decode("utf-8", errors="replace"))
        return result.stdout

    def repository(self):
        repo = self.root / "repo"
        repo.mkdir()
        self.git(repo, "init", "-q")
        self.git(repo, "config", "user.name", "Cherri causal control")
        self.git(repo, "config", "user.email", "qa@example.invalid")
        self.git(repo, "config", "core.autocrlf", "false")
        self.git(repo, "config", "commit.gpgsign", "false")
        hooks = self.root / "empty-hooks"
        hooks.mkdir()
        self.git(repo, "config", "core.hooksPath", str(hooks))
        (repo / "doc.txt").write_bytes(b"original public control\n")
        self.git(repo, "add", "doc.txt")
        self.git(repo, "commit", "-qm", "Original public control")
        return repo, self.git(repo, "rev-parse", "HEAD").decode().strip()

    def test_strict_json_rejects_duplicate_nonfinite_invalid_utf8_and_trailing_data(self):
        for raw in (b'{"name":1,"name":2}', b'{"x":NaN}', b'{"x":Infinity}',
                    b'{"x":"\xff"}', b'{} trailing', b'{"x":'):
            with self.subTest(raw=raw):
                with self.assertRaises((ValueError, UnicodeError)):
                    subject.strict_json(raw)
        self.assertEqual(subject.strict_json(b'{"present":false,"zero":0,"empty":""}'),
                         {"present": False, "zero": 0, "empty": ""})

    def test_all_capability_identity_fields_are_required_and_exact(self):
        subject.validate_capabilities(self.capabilities(), self.manifest())
        for field in subject.IDENTITY_FIELDS:
            for replacement in (None, "", "foreign"):
                with self.subTest(field=field, replacement=replacement):
                    value = self.capabilities()
                    value[field] = replacement
                    with self.assertRaises(ValueError):
                        subject.validate_capabilities(value, self.manifest())
        for count in (True, 0, -1, 2.0, "2"):
            value = self.capabilities()
            value["actionCount"] = count
            with self.assertRaises(ValueError):
                subject.validate_capabilities(value, self.manifest())

    def test_complete_catalog_cannot_omit_duplicate_or_invent_inventory(self):
        self.assertEqual(set(subject.validate_catalog(self.catalog(), self.capabilities())),
                         {"first", "second"})
        faults = []
        value = self.catalog(); value["actions"].pop(); faults.append(value)
        value = self.catalog(); value["actions"][1]["name"] = "first"; faults.append(value)
        value = self.catalog(); value["actions"][1] = None; faults.append(value)
        value = self.catalog(); value["count"] = True; faults.append(value)
        value = self.catalog(); value["version"] = "legacy"; faults.append(value)
        value = self.catalog(); value["ok"] = False; faults.append(value)
        value = self.catalog(); value["actions"][1]["name"] = ""; faults.append(value)
        for value in faults:
            with self.subTest(value=value):
                with self.assertRaises(ValueError):
                    subject.validate_catalog(value, self.capabilities())

    def test_catalog_filter_uses_real_compact_or_formatted_bytes_without_empty_credit(self):
        for original in (b'{"name":"resizeImage","title":"Resize Image"}\n',
                         b'{\n  "name": "resizeImage",\n  "title": "Resize Image"\n}\n'):
            expected = b"".join(line + b"\n" for line in original.replace(b",", b"\n").splitlines()
                                if b"resizeImage" in line)
            subject.verify_catalog_filter(expected, original, "resizeImage")
            for changed in (b"", expected + b"extra\n", expected.replace(b"resizeImage", b"foreign")):
                with self.assertRaises(ValueError):
                    subject.verify_catalog_filter(changed, original, "resizeImage")
        with self.assertRaises(ValueError):
            subject.verify_catalog_filter(b"", b'{"name":"resizeImage"}\n', '"name": "resizeImage"')

    def reference(self):
        tick = chr(96)
        body = ("# Actual synthetic reference control\n"
                "**Schema Fingerprint:** " + tick + "a" * 64 + tick + "\n"
                "**Total Actions:** 2\n"
                "### " + tick + "first" + tick + "\nfirst(value: Text)\n"
                "### " + tick + "second" + tick + "\nsecond(value: Number)\n"
                "## Compiler macros\n"
                "**Compiler Macro Fingerprint:** " + tick + "c" * 64 + tick + "\n")
        return body.encode()

    def test_reference_compares_entire_pinned_body_not_only_fingerprints(self):
        body = self.reference()
        docs = b"---\ntitle: Synthetic control\n---\n\n" + body
        catalog = subject.validate_catalog(self.catalog(), self.capabilities())
        result = subject.verify_reference(docs, body, self.capabilities(), catalog)
        self.assertEqual(result["sha256"], subject.digest(body))
        for changed in (docs.replace(b"value: Text", b"value: Number"),
                        docs + b"\nextra\n", docs[:-1], docs.replace(b"\n\n#", b"\n#")):
            with self.subTest(changed=changed):
                with self.assertRaises(ValueError):
                    subject.verify_reference(changed, body, self.capabilities(), catalog)
        for changed in (body.replace(b"first", b"foreign"),
                        body.replace(b"a" * 64, b"d" * 64),
                        body.replace(b"**Total Actions:** 2", b"**Total Actions:** 3"),
                        body + b"## Compiler macros\n"):
            with self.subTest(body=changed):
                with self.assertRaises(ValueError):
                    subject.verify_reference(b"---\ntitle: Control\n---\n\n" + changed,
                                             changed, self.capabilities(), catalog)

    def test_unpacked_inventory_checks_original_bytes_missing_and_added_members(self):
        package = self.root / "package"
        package.mkdir()
        expected = {"cherri-shortcuts/scripts/action.sh": (b"original action\n", 0o100755),
                    "cherri-shortcuts/compatibility-manifest.json": (b'{"synthetic":true}\n', 0o100644)}
        for name, (data, mode) in expected.items():
            path = package / name.removeprefix("cherri-shortcuts/")
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            path.chmod(mode & 0o777)
        before = subject.verify_package(package, expected)
        self.assertEqual(len(before), 2)
        target = package / "scripts/action.sh"
        original = target.read_bytes()
        target.write_bytes(original + b"changed\n")
        with self.assertRaises(ValueError):
            subject.verify_package(package, expected)
        target.write_bytes(original)
        extra = package / "extra.txt"; extra.write_bytes(b"unreviewed")
        with self.assertRaises(ValueError):
            subject.verify_package(package, expected)
        extra.unlink()
        target.unlink()
        with self.assertRaises(ValueError):
            subject.verify_package(package, expected)

    def test_git_replacement_does_not_substitute_immutable_source(self):
        repo, original = self.repository()
        (repo / "doc.txt").write_bytes(b"replacement control\n")
        self.git(repo, "add", "doc.txt")
        self.git(repo, "commit", "-qm", "Different public control")
        replacement = self.git(repo, "rev-parse", "HEAD").decode().strip()
        self.git(repo, "replace", original, replacement)
        files = subject.git_files(repo, original, ("doc.txt",))
        self.assertEqual(files["doc.txt"][0], b"original public control\n")
        self.assertEqual(self.git(repo, "replace", "-l").decode().strip(), original)

    def test_repository_rejects_hidden_index_flags_without_changing_them(self):
        repo, head = self.repository()
        subject.repository_state(repo, head)
        for flag, undo in (("--assume-unchanged", "--no-assume-unchanged"),
                           ("--skip-worktree", "--no-skip-worktree")):
            self.git(repo, "update-index", flag, "doc.txt")
            before = self.git(repo, "ls-files", "-v", "-z")
            with self.assertRaisesRegex(ValueError, "index flag"):
                subject.repository_state(repo, head)
            self.assertEqual(self.git(repo, "ls-files", "-v", "-z"), before)
            self.git(repo, "update-index", undo, "doc.txt")
        (repo / "doc.txt").write_bytes(b"changed\n")
        with self.assertRaisesRegex(ValueError, "clean"):
            subject.repository_state(repo, head)

    def test_git_source_budget_is_aggregate_and_object_size_bound(self):
        repo, head = self.repository()
        original_object, original_total = subject.MAX_OBJECT, subject.MAX_TOTAL
        try:
            subject.MAX_OBJECT = 4
            with self.assertRaisesRegex(ValueError, "object exceeds"):
                subject.git_files(repo, head, ("doc.txt",))
            subject.MAX_OBJECT = original_object
            subject.MAX_TOTAL = 4
            with self.assertRaisesRegex(ValueError, "combined"):
                subject.git_files(repo, head, ("doc.txt",))
        finally:
            subject.MAX_OBJECT, subject.MAX_TOTAL = original_object, original_total

    def test_search_rows_bind_exact_scope_line_bytes_and_complete_match_count(self):
        docs = self.root / "docs"
        path = docs / "fork-language/guide.md"
        path.parent.mkdir(parents=True)
        data = b"V2 Optional values\nNo match\nOptional return\n"
        path.write_bytes(data)
        source = {"fork-language/guide.md": (data, 0o100644, "a" * 40)}
        raw = ((path.as_posix() + ":1:V2 Optional values\n" +
                path.as_posix() + ":3:Optional return\n").encode())
        self.assertEqual(subject.verify_search(raw, docs, "fork-language", "Optional", source)["matches"], 2)
        faults = [raw.splitlines(keepends=True)[0], raw + raw.splitlines(keepends=True)[0],
                  raw.replace(b":3:Optional return", b":2:Optional return"),
                  raw.replace(b":1:V2 Optional values", b":1:Legacy Optional values"),
                  raw.replace(b"fork-language", b"language")]
        for fault in faults:
            with self.subTest(raw=fault):
                with self.assertRaises(ValueError):
                    subject.verify_search(fault, docs, "fork-language", "Optional", source)
        self.assertEqual(subject.verify_search(b"", docs, "fork-language", "never", source)["matches"], 0)

    def test_actual_subprocess_retains_exit_stdout_stderr_and_hashes(self):
        recorder = subject.Recorder(self.root / "calls")
        command = [sys.executable, "-c",
                   "import sys;sys.stdout.buffer.write(b'actual stdout\\x00\\n');sys.stderr.write('actual stderr\\n');sys.exit(7)"]
        result = recorder.run("real-controlled-process", command, self.root, os.environ.copy())
        row, stdout, stderr = result
        self.assertEqual(row["exitCode"], 7)
        self.assertEqual(stdout, b"actual stdout\0\n")
        self.assertEqual(stderr, b"actual stderr\r\n" if os.name == "nt" else b"actual stderr\n")
        self.assertEqual(row["stdout"]["sha256"], subject.digest(stdout))
        self.assertEqual(row["stderr"]["sha256"], subject.digest(stderr))
        self.assertFalse(row["timedOut"])
        with self.assertRaises(ValueError):
            subject.succeeded(result)
        subject.succeeded(result, 7)
        self.assertEqual(json.loads((recorder.directory / "real-controlled-process.process.json").read_bytes()), row)

    def test_actual_timeout_kills_owned_subprocess_tree_and_cannot_succeed(self):
        recorder = subject.Recorder(self.root / "calls")
        started, later = self.root / "child-started", self.root / "child-too-late"
        child = ("from pathlib import Path;import time;"
                 "Path(" + repr(str(started)) + ").write_text('started');"
                 "time.sleep(1.8);Path(" + repr(str(later)) + ").write_text('late')")
        parent = ("import subprocess,sys,time;from pathlib import Path;"
                  "subprocess.Popen([sys.executable,'-c'," + repr(child) + "]);"
                  "time.sleep(10)")
        result = recorder.run("real-timeout", [sys.executable, "-c", parent],
                              self.root, os.environ.copy(), timeout=0.8)
        self.assertTrue(started.exists(), "child must actually start for this tree control")
        self.assertTrue(result[0]["timedOut"])
        with self.assertRaises(ValueError):
            subject.succeeded(result, result[0]["exitCode"])
        time.sleep(1.3)
        self.assertFalse(later.exists(), "owned child survived timeout cleanup")

    def test_late_exit_zero_cannot_satisfy_original_deadline(self):
        from unittest import mock
        recorder = subject.Recorder(self.root / "calls")
        actual_popen = subject.subprocess.Popen

        def delayed_launch_return(*args, **kwargs):
            process = actual_popen(*args, **kwargs)
            self.assertEqual(process.wait(timeout=5), 0)
            time.sleep(0.04)
            return process

        with mock.patch.object(subject.subprocess, "Popen", side_effect=delayed_launch_return):
            result = recorder.run("actual-child-delayed-launch-return",
                                  [sys.executable, "-c", "print('actual public child completed')"],
                                  self.root, os.environ.copy(), timeout=0.01)
        self.assertEqual(result[0]["exitCode"], 0)
        self.assertGreater(result[0]["elapsedSeconds"], result[0]["timeoutSeconds"])
        print("CHERRI_DOC_WRAPPER_LATE_CHILD_CONTROL=" + json.dumps(result[0], sort_keys=True))
        with self.assertRaises(ValueError):
            subject.succeeded(result)

    def test_success_requires_finite_nonnegative_elapsed_within_original_allowance(self):
        base = {"id": "synthetic-time-control", "timedOut": False, "exitCode": 0,
                "elapsedSeconds": 0.1, "timeoutSeconds": 1.0}
        subject.succeeded((base, b"", b""))
        for field, values in (("elapsedSeconds", (None, True, -1, 2, float("nan"), float("inf"))),
                              ("timeoutSeconds", (None, True, 0, -1, float("nan"), float("inf")))):
            for value in values:
                changed = dict(base)
                changed[field] = value
                with self.subTest(field=field, value=value):
                    with self.assertRaises(ValueError):
                        subject.succeeded((changed, b"", b""))

    def test_environment_cannot_inherit_caller_pin_or_compiler_overrides(self):
        old = dict(os.environ)
        try:
            os.environ.update({"CHERRI_REF": "foreign", "CHERRI_BIN": "foreign",
                               "CHERRI_DOCS_REF": "foreign", "BASH_ENV": "foreign",
                               "GIT_REPLACE_REF_BASE": "foreign"})
            value = subject.wrapper_environment(self.root / "home", self.root / "real", self.root / "docs")
            self.assertNotIn("CHERRI_REF", value)
            self.assertNotIn("CHERRI_DOCS_REF", value)
            self.assertNotIn("BASH_ENV", value)
            self.assertNotIn("GIT_REPLACE_REF_BASE", value)
            self.assertEqual(value["CHERRI_BIN"], (self.root / "real").as_posix())
        finally:
            os.environ.clear()
            os.environ.update(old)

    def test_failed_preflight_retains_report_without_calls_or_acceptance(self):
        report = subject.collect(self.root, "not-an-immutable-source", self.root,
                                 self.root / "absent-tool", self.root, self.root / "evidence")
        self.assertEqual(report["status"], "FAIL_PACKAGED_DOCUMENTATION_WRAPPERS")
        self.assertEqual(report["calls"], [])
        self.assertEqual(report["requirementsClosed"], [])
        self.assertFalse(report["formalAcceptanceGranted"])
        self.assertIsNone(report["inputsUnchangedAfter"])
        self.assertEqual(json.loads((self.root / "evidence/report.json").read_bytes()), report)


if __name__ == "__main__":
    unittest.main(verbosity=2)
