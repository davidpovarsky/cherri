"""Real-Git mutation tests for reviewed R23 protected-file evolution."""

import copy
import hashlib
import json
import os
import pathlib
import re
import subprocess
import tempfile
import unittest
from unittest import mock

import protected_ios


class TestReviewedProtectedIOS(unittest.TestCase):
    def test_reviewed_evolution_and_unreviewed_mutations(self):
        env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
        env.update({
            "GIT_CONFIG_NOSYSTEM": "1", "GIT_CONFIG_GLOBAL": os.devnull,
            "GIT_AUTHOR_NAME": "R23 review fixture",
            "GIT_AUTHOR_EMAIL": "r23@example.invalid",
            "GIT_COMMITTER_NAME": "R23 review fixture",
            "GIT_COMMITTER_EMAIL": "r23@example.invalid",
        })
        with tempfile.TemporaryDirectory(prefix="cherri-r23-review-") as directory, \
                mock.patch.dict(os.environ, env, clear=True):
            root = pathlib.Path(directory)
            protected = ["ios/Tests.swift", "ios/Absent.swift"]
            review_path = "review.json"
            def git(*args):
                p = subprocess.run(["git", *args], cwd=root, capture_output=True, timeout=10)
                self.assertEqual(p.returncode, 0, (args, p.stdout, p.stderr))
                return p.stdout
            def commit(message):
                git("add", "-A")
                git("commit", "-m", message)
                return git("rev-parse", "HEAD").decode().strip()
            def snapshot(revision, path):
                entry = git("ls-tree", "-z", revision, "--", path)
                if not entry:
                    return {"blob": None, "sha256": None, "tests": []}
                blob = entry.split(b"\t")[0].split()[2].decode()
                raw = git("show", revision + ":" + path)
                names = sorted(re.findall(r"func (test\w+)\(", raw.decode()))
                return {"blob": blob, "sha256": hashlib.sha256(raw).hexdigest(), "tests": names}
            def manifest(base, reviewed):
                return {
                    "schemaVersion": 1, "baseline": base, "reviewedCommit": reviewed,
                    "reason": "Reviewed additive fixture test",
                    "reviewEvidence": "Retains testOriginal and adds testNew",
                    "files": [{"path": p, "baseline": snapshot(base, p),
                               "reviewed": snapshot(reviewed, p)} for p in protected],
                }
            def write_review(value):
                (root / review_path).write_text(json.dumps(value) + "\n")
            def observe(expected, pattern=None):
                result = protected_ios.execute_reviewed_gate(root, "R23", definition)
                self.assertEqual(result["status"], expected, result)
                if pattern:
                    self.assertIn(pattern, result["error"])
                return result
            git("init", "--initial-branch=r23-fixture")
            git("config", "--local", "commit.gpgsign", "false")
            git("config", "--local", "core.hooksPath", str(root / "no-hooks"))
            (root / "ios").mkdir()
            path = root / protected[0]
            original = "func testOriginal() { XCTAssertTrue(true) }\n"
            approved = original + "func testNew() { XCTAssertEqual(1, 1) }\n"
            path.write_text(original)
            baseline = commit("Baseline")
            path.write_text(approved)
            reviewed = commit("Reviewed additive test")
            definition = {
                "name": "Fixture reviewed protection", "tier": "PRODUCTION_PATH",
                "sourceFile": "git", "upstreamBase": baseline,
                "checkProtectedIOSFiles": protected, "protectedIOSReview": review_path,
            }
            accepted = manifest(baseline, reviewed)
            write_review(accepted)
            accepted_commit = commit("Review exact evolution")
            with self.subTest("reviewed_change_with_historical_failure"):
                result = observe("PASS")
                self.assertEqual(result["historicalBaselineResult"], "FAIL")
                self.assertEqual(result["protectedFiles"][0]["baselineTestsRetained"], 1)
            for name, staged in [("unstaged", False), ("staged", True)]:
                with self.subTest(name):
                    path.write_text(approved + "// mutation\n")
                    if staged:
                        git("add", "--", protected[0])
                    observe("FAIL", "uncommitted")
                    git("reset", "--hard", accepted_commit)
            with self.subTest("committed_change_clean_tree"):
                path.write_text(approved + "// committed mutation\n")
                commit("Unreviewed change")
                observe("FAIL", "Unreviewed committed")
                git("reset", "--hard", accepted_commit)
            with self.subTest("protected_file_deletion"):
                path.unlink()
                commit("Unreviewed deletion")
                observe("FAIL", "Unreviewed committed")
                git("reset", "--hard", accepted_commit)
            with self.subTest("protected_absence_creation"):
                absent = root / protected[1]
                absent.write_text("// new\n")
                observe("FAIL", "uncommitted")
                commit("Unreviewed creation")
                observe("FAIL", "Unreviewed committed")
                git("reset", "--hard", accepted_commit)
            mutations = [
                ("baseline", lambda v: v.update(baseline="0" * 40), "immutable"),
                ("unavailable_commit", lambda v: v.update(reviewedCommit="0" * 40), "Git verification"),
                ("missing_path", lambda v: v["files"].pop(), "exactly"),
                ("duplicate_path", lambda v: v["files"].__setitem__(1, v["files"][0]), "exactly"),
                ("wrong_blob", lambda v: v["files"][0]["reviewed"].update(blob="0" * 40), "mismatch"),
                ("wrong_sha", lambda v: v["files"][0]["reviewed"].update(sha256="0" * 64), "mismatch"),
                ("wrong_test_inventory", lambda v: v["files"][0]["reviewed"].update(tests=[]), "mismatch"),
                ("missing_reason", lambda v: v.pop("reason"), "reason"),
            ]
            for name, mutate, error in mutations:
                with self.subTest(name):
                    changed = copy.deepcopy(accepted)
                    mutate(changed)
                    write_review(changed)
                    commit("Mutate review " + name)
                    observe("FAIL", error)
                    git("reset", "--hard", accepted_commit)
            with self.subTest("dirty_review"):
                write_review({**accepted, "reason": "uncommitted"})
                observe("FAIL", "uncommitted")
                git("reset", "--hard", accepted_commit)
            with self.subTest("missing_review"):
                (root / review_path).unlink()
                observe("FAIL")
                git("reset", "--hard", accepted_commit)
            with self.subTest("review_cannot_drop_original_tests"):
                path.write_text("func testNew() { XCTAssertEqual(1, 1) }\n")
                bad_reviewed = commit("Remove original test")
                write_review(manifest(baseline, bad_reviewed))
                commit("Attempt review of removal")
                observe("FAIL", "removes original XCTest")
                git("reset", "--hard", accepted_commit)
            with self.subTest("review_cannot_remove_protected_file"):
                path.unlink()
                bad_reviewed = commit("Remove protected source")
                write_review(manifest(baseline, bad_reviewed))
                commit("Attempt review of file deletion")
                observe("FAIL")
                git("reset", "--hard", accepted_commit)
            with self.subTest("unrelated_committed_change"):
                (root / "unrelated.txt").write_text("unrelated\n")
                commit("Unrelated change")
                observe("PASS")

    def test_method_inventory_ignores_comments_and_handles_modifiers(self):
        source = b"""// func testComment() {}
    func testOriginal() throws {}
    private func testPrivate() {}
    @MainActor
    func testActor() async throws {}
"""
        self.assertEqual(protected_ios.test_methods(source),
                         [b"testActor", b"testOriginal", b"testPrivate"])
