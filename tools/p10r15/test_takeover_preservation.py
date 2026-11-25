#!/usr/bin/env python3
"""Real temporary Git transactions exercise the operational takeover boundary."""
import copy
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock

sys.dont_write_bytecode = True
HERE = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location("takeover", HERE / "takeover_preservation.py")
takeover = importlib.util.module_from_spec(spec)
spec.loader.exec_module(takeover)

class TakeoverPreservationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-takeover-control-")
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name).resolve()
        self.repositories = []
        for role, repository in takeover.ROLES.items():
            root = self.base / role
            root.mkdir()
            self.git(root, "init", "-b", "main")
            self.git(root, "remote", "add", "origin", "https://github.com/" + repository + ".git")
            (root / "seed.txt").write_bytes(b"original\n")
            (root / "delete.txt").write_bytes(b"delete-original\n")
            (root / "old-name.txt").write_bytes(b"rename-original\n")
            if role == "compiler":
                source = root / takeover.SOURCE
                source.parent.mkdir(parents=True)
                source.write_bytes((HERE / "takeover_preservation.py").read_bytes())
            self.git(root, "add", ".")
            self.git(root, "commit", "-m", "fixture baseline")
            head = self.git(root, "rev-parse", "HEAD").decode().strip()
            self.git(root, "branch", "unrelated-agent", head)
            # A real index/worktree split, staged rename, deletion and Unicode
            # binary untracked file must survive the actual production helper.
            (root / "seed.txt").write_bytes(b"index-version\n")
            self.git(root, "add", "seed.txt")
            (root / "seed.txt").write_bytes("working-א😀\x00version\n".encode())
            self.git(root, "mv", "old-name.txt", "new-name.txt")
            self.git(root, "rm", "delete.txt")
            (root / "private draft.txt").write_bytes(b"unpublished\x00binary\n")
            self.repositories.append({"role": role, "repository": repository, "root": str(root),
                                      "head": head, "branch": "refs/heads/main",
                                      "recoveryRef": "refs/heads/agent/cherri-takeover-control"})
        self.implementation = self.repositories[0]["head"]
        self.spec_path = self.base / "spec.json"
        self.spec_value = {"schema": takeover.SPEC_SCHEMA, "implementationSHA": self.implementation,
                           "repositories": self.repositories}
        self.spec_path.write_bytes(takeover.canonical(self.spec_value))
        self.spec_sha = takeover.digest(self.spec_path.read_bytes())
        self.output = self.base / "originals"
        self.expected = self.base / "expected.json"

    def git(self, root, *args):
        result = subprocess.run(takeover.git_command(root, list(args)), capture_output=True, timeout=15)
        self.assertEqual(result.returncode, 0, result.stderr.decode(errors="replace"))
        return result.stdout

    def produce(self):
        result = takeover.produce(self.spec_path, self.spec_sha, self.output)
        self.assertEqual(result["status"], "PASSED")
        return result

    def selected(self, suffix=""):
        target = self.base / ("expected" + suffix + ".json")
        takeover.selection(self.output, target)
        return target, takeover.digest(target.read_bytes())

    def verify(self, target=None):
        if target is None:
            target = self.expected
        return takeover.verify(target, takeover.digest(target.read_bytes()), self.implementation)

    def rewrite_receipt(self, edit, suffix):
        path = self.output / "takeover.original.json"
        report = takeover.read_json(path)
        edit(report)
        path.write_bytes(takeover.canonical(report))
        return self.selected(suffix)[0]

    def assert_no_recovery_refs(self):
        for repo in self.repositories:
            refs = takeover.ref_rows(self.git(repo["root"], "for-each-ref", "--format=%(objectname) %(refname)"))
            self.assertNotIn(repo["recoveryRef"], refs)
            self.assertEqual(refs["refs/heads/main"], repo["head"])
            self.assertEqual(refs["refs/heads/unrelated-agent"], repo["head"])

    def test_actual_dirty_dual_repository_transaction(self):
        receipt = self.produce()
        self.selected()
        result = self.verify()
        self.assertEqual(result["commands"], 58)
        self.assertEqual(result["repositories"], 2)
        self.assertGreaterEqual(result["unpublishedBackups"], 6)
        self.assertFalse(result["closureComplete"])
        for row, repo in zip(receipt["repositories"], self.repositories):
            self.assertEqual(row["before"]["statusSHA256"], row["after"]["statusSHA256"])
            self.assertEqual(row["before"]["workDiffSHA256"], row["after"]["workDiffSHA256"])
            self.assertEqual(row["before"]["indexDiffSHA256"], row["after"]["indexDiffSHA256"])
            self.assertEqual(row["after"]["refs"]["refs/heads/unrelated-agent"], repo["head"])
            self.assertEqual(set(row["backups"]), {"seed.txt", "new-name.txt", "private draft.txt"})
            self.assertEqual(row["before"]["files"]["delete.txt"], {"missing": True})
            self.assertEqual(row["before"]["files"]["old-name.txt"], {"missing": True})
            for name, backup in row["backups"].items():
                self.assertEqual((self.output / backup["file"]).read_bytes(), (Path(repo["root"]) / name).read_bytes())

    def test_existing_destination_rejected_before_either_write(self):
        repo = self.repositories[1]
        self.git(repo["root"], "update-ref", repo["recoveryRef"], repo["head"])
        with self.assertRaisesRegex(takeover.Rejected, "RECOVERY_REF_ALREADY_EXISTS"):
            self.produce()
        first = self.repositories[0]
        refs = takeover.ref_rows(self.git(first["root"], "for-each-ref", "--format=%(objectname) %(refname)"))
        self.assertNotIn(first["recoveryRef"], refs)
        self.assertEqual(refs["refs/heads/main"], first["head"])

    def test_stale_source_rejected_before_ref_write(self):
        repo = self.repositories[0]
        self.git(repo["root"], "commit", "-m", "later staged work")
        with self.assertRaisesRegex(takeover.Rejected, "SOURCE_IDENTITY"):
            self.produce()
        for item in self.repositories:
            refs = takeover.ref_rows(self.git(item["root"], "for-each-ref", "--format=%(objectname) %(refname)"))
            self.assertNotIn(item["recoveryRef"], refs)

    def test_source_overlap_rejected_before_output(self):
        target = Path(self.repositories[0]["root"]) / "generated-evidence"
        with self.assertRaisesRegex(takeover.Rejected, "OUTPUT_OVERLAP"):
            takeover.produce(self.spec_path, self.spec_sha, target)
        self.assertFalse(target.exists())
        self.assert_no_recovery_refs()

    def test_symlink_unpublished_file_rejected_before_ref_write(self):
        (Path(self.repositories[0]["root"]) / "redirect").symlink_to(self.spec_path)
        with self.assertRaisesRegex(takeover.Rejected, "NONREGULAR_OR_REDIRECTED_FILE"):
            self.produce()
        self.assert_no_recovery_refs()

    def test_omitted_backup_rejected_even_with_reselected_hashes(self):
        self.produce()
        target = self.rewrite_receipt(lambda r: r["repositories"][0]["backups"].pop("seed.txt"), "-missing")
        with self.assertRaisesRegex(takeover.Rejected, "MISSING_OR_EXTRA_UNPUBLISHED_BACKUP"):
            self.verify(target)

    def test_corrupted_backup_rejected_even_with_reselected_hashes(self):
        report = self.produce()
        backup = report["repositories"][0]["backups"]["seed.txt"]
        (self.output / backup["file"]).write_bytes(b"wrong-original")
        self.selected()
        with self.assertRaisesRegex(takeover.Rejected, "BACKUP_IDENTITY"):
            self.verify()

    def test_source_change_after_takeover_rejected(self):
        self.produce()
        self.selected()
        (Path(self.repositories[0]["root"]) / "seed.txt").write_bytes(b"lost-user-work")
        with self.assertRaisesRegex(takeover.Rejected, "CURRENT_SOURCE_OR_REFS_CHANGED"):
            self.verify()

    def test_unrelated_branch_movement_rejected(self):
        self.produce()
        self.selected()
        repo = self.repositories[1]
        self.git(repo["root"], "update-ref", "-d", "refs/heads/unrelated-agent", repo["head"])
        with self.assertRaisesRegex(takeover.Rejected, "CURRENT_SOURCE_OR_REFS_CHANGED"):
            self.verify()

    def test_wrong_protected_snapshot_rejected(self):
        self.produce()
        target = self.rewrite_receipt(lambda r: r["repositories"][0]["before"]["refs"].pop("refs/heads/unrelated-agent"), "-snapshot")
        with self.assertRaisesRegex(takeover.Rejected, "PROTECTED_SOURCE_OR_REFS"):
            self.verify(target)

    def test_non_create_only_ref_command_rejected(self):
        self.produce()
        def edit(report):
            command = next(c for c in report["commands"] if c["id"] == "compiler-create-ref")
            command["command"] = command["command"][:-1]
            (self.output / "commands/compiler-create-ref.process.json").write_bytes(takeover.canonical(command))
        target = self.rewrite_receipt(edit, "-unsafe")
        with self.assertRaisesRegex(takeover.Rejected, "EXACT_ORIGINAL_COMMAND"):
            self.verify(target)

    def test_changed_read_command_rejected(self):
        self.produce()
        def edit(report):
            command = report["commands"][0]
            command["command"][-1] = "refs/heads/unrelated-agent"
            (self.output / ("commands/" + command["id"] + ".process.json")).write_bytes(takeover.canonical(command))
        target = self.rewrite_receipt(edit, "-query")
        with self.assertRaisesRegex(takeover.Rejected, "EXACT_ORIGINAL_COMMAND"):
            self.verify(target)

    def test_missing_command_rejected(self):
        self.produce()
        target = self.rewrite_receipt(lambda r: r["commands"].pop(), "-command")
        with self.assertRaisesRegex(takeover.Rejected, "COMPLETE_EXECUTION_INVENTORY"):
            self.verify(target)

    def test_boolean_exit_status_rejected(self):
        self.produce()
        def edit(report):
            command = report["commands"][0]
            command["exitCode"] = False
            (self.output / ("commands/" + command["id"] + ".process.json")).write_bytes(takeover.canonical(command))
        target = self.rewrite_receipt(edit, "-boolean")
        with self.assertRaisesRegex(takeover.Rejected, "COMMAND_OUTCOME"):
            self.verify(target)

    def test_original_hash_tampering_rejected(self):
        self.produce()
        self.selected()
        with (self.output / "commands/compiler-before-head.stdout").open("ab") as stream:
            stream.write(b"tampered")
        with self.assertRaisesRegex(takeover.Rejected, "ORIGINAL_CHANGED"):
            self.verify()

    def test_fixture_cannot_be_retagged_as_production_candidate(self):
        self.produce()
        self.selected()
        with self.assertRaisesRegex(takeover.Rejected, "EXPECTED_CONTEXT_SCOPE"):
            takeover.verify(self.expected, takeover.digest(self.expected.read_bytes()), "f" * 40)

    def test_duplicate_json_and_unpinned_context_rejected(self):
        duplicate = self.base / "duplicate.json"
        duplicate.write_text('{"schema":1,"schema":2}')
        with self.assertRaisesRegex(takeover.Rejected, "DUPLICATE_JSON_KEY"):
            takeover.read_json(duplicate)
        with self.assertRaisesRegex(takeover.Rejected, "EXPECTED_CONTEXT_HASH"):
            takeover.verify(duplicate, "0" * 64, self.implementation)


    def test_ambient_git_redirection_cannot_escape_selected_repositories(self):
        poison = self.base / "poison-index"
        poison.write_bytes(b"unrelated index must survive")
        env = {"GIT_DIR": str(Path(self.repositories[1]["root"]) / ".git"),
               "GIT_WORK_TREE": self.repositories[1]["root"],
               "GIT_INDEX_FILE": str(poison), "GIT_NAMESPACE": "wrong-namespace",
               "GIT_OBJECT_DIRECTORY": str(self.base / "missing-objects"),
               "GIT_ALTERNATE_OBJECT_DIRECTORIES": str(self.base / "missing-alternate"),
               "GIT_CONFIG_COUNT": "1", "GIT_CONFIG_KEY_0": "core.worktree",
               "GIT_CONFIG_VALUE_0": self.repositories[1]["root"]}
        with mock.patch.dict(os.environ, env):
            self.produce()
            self.selected()
            result = self.verify()
        self.assertEqual(result["status"], "PASSED")
        self.assertEqual(poison.read_bytes(), b"unrelated index must survive")
        for repo in self.repositories:
            refs = takeover.ref_rows(self.git(repo["root"], "for-each-ref", "--format=%(objectname) %(refname)"))
            self.assertEqual(refs[repo["recoveryRef"]], repo["head"])
            self.assertEqual(refs["refs/heads/main"], repo["head"])

    def test_hidden_index_flags_rejected_before_any_ref_write(self):
        root = self.repositories[0]["root"]
        for flag in ("--assume-unchanged", "--skip-worktree"):
            with self.subTest(flag=flag):
                self.git(root, "update-index", flag, "seed.txt")
                with self.assertRaisesRegex(takeover.Rejected, "HIDDEN_OR_UNMERGED_INDEX_ENTRY"):
                    takeover.produce(self.spec_path, self.spec_sha, self.base / ("flag-" + flag[2:]))
                self.git(root, "update-index", "--no-assume-unchanged", "--no-skip-worktree", "seed.txt")
                self.assert_no_recovery_refs()

    def test_object_history_redirections_rejected_before_any_ref_write(self):
        root = Path(self.repositories[0]["root"])
        for name in ("objects/info/alternates", "info/grafts"):
            with self.subTest(name=name):
                p = root / ".git" / name
                p.write_bytes(b"")
                with self.assertRaisesRegex(takeover.Rejected, "REDIRECTED_OBJECT_HISTORY"):
                    takeover.produce(self.spec_path, self.spec_sha, self.base / ("history-" + p.name))
                p.unlink()
                self.assert_no_recovery_refs()

    def test_replacement_refs_rejected_before_any_recovery_write(self):
        repo = self.repositories[0]
        self.git(repo["root"], "update-ref", "refs/replace/" + repo["head"], repo["head"])
        with self.assertRaisesRegex(takeover.Rejected, "REPLACEMENT_REFS"):
            self.produce()
        self.git(repo["root"], "update-ref", "-d", "refs/replace/" + repo["head"])
        self.assert_no_recovery_refs()

    def test_retagged_scope_rejected_even_with_reselected_pins(self):
        self.produce()
        target = self.rewrite_receipt(lambda r: r.update(scope="All acceptance passed"), "-scope")
        with self.assertRaisesRegex(takeover.Rejected, "PRODUCER_SCOPE"):
            self.verify(target)

if __name__ == "__main__":
    unittest.main()
