"""Causal external-wrapper controls. Fixtures do not claim compiler/native credit."""
import hashlib
import json
import os
from pathlib import Path
import plistlib
import subprocess
import tempfile
import unittest

import service
import pack_adapter
import assemble


class PureCheckoutTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.repository = self.root / "checkout"
        self.repository.mkdir()
        self.git("init", "-q")
        self.git("config", "user.name", "Cherri wrapper control")
        self.git("config", "user.email", "control@example.invalid")
        self.git("remote", "add", "origin", service.UPSTREAM[2])
        (self.repository / "source.txt").write_text("original source\n")
        (self.repository / ".gitignore").write_text("ignored.tmp\n")
        self.git("add", ".")
        self.git("commit", "-qm", "Retain synthetic wrapper control")
        self.expected = (self.git("rev-parse", "HEAD").strip(),
                         self.git("rev-parse", "HEAD^{tree}").strip(), service.UPSTREAM[2])

    def git(self, *arguments):
        return subprocess.check_output(["git", "-C", str(self.repository), *arguments],
                                       stderr=subprocess.PIPE).decode()

    def test_real_git_clean_checkout(self):
        self.assertEqual(service.verify_checkout(self.repository, self.expected), 2)

    def test_changed_tracked_file_rejected(self):
        (self.repository / "source.txt").write_text("corruption")
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_deleted_tracked_file_rejected(self):
        (self.repository / "source.txt").unlink()
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_untracked_file_rejected(self):
        (self.repository / "extra").write_text("unexpected")
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_ignored_file_rejected(self):
        (self.repository / "ignored.tmp").write_text("unexpected ignored data")
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_origin_drift_rejected(self):
        self.git("remote", "set-url", "origin", "https://example.invalid/not-upstream")
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_alternate_object_dependency_rejected(self):
        (self.repository / ".git/objects/info/alternates").write_text("/other/object/store\n")
        with self.assertRaises(service.ContractError):
            service.verify_checkout(self.repository, self.expected)

    def test_symlink_checkout_rejected(self):
        alias = self.root / "alias"
        alias.symlink_to(self.repository, target_is_directory=True)
        with self.assertRaises(service.ContractError):
            service.verify_checkout(alias, self.expected)

    def test_skipped_worktree_file_still_rejected(self):
        self.git("update-index", "--skip-worktree", "source.txt")
        (self.repository / "source.txt").unlink()
        self.assertEqual(self.git("status", "--porcelain"), "")
        with self.assertRaises(OSError):
            service.verify_checkout(self.repository, self.expected)


class PureProjectTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.root = Path(self.temporary.name)
        self.jobs = self.root / "jobs"
        self.project = self.jobs / "project"
        self.project.mkdir(parents=True)
        self.source = self.project / "main.cherri"
        self.source.write_text('show("pure")\n')

    def test_single_file_and_project_bytes_preserved(self):
        (self.project / "helper.cherri").write_text("nothing()\n")
        before = self.source.read_bytes()
        source, parent, files = service.project_snapshot(self.source, self.jobs)
        self.assertEqual(source, self.source)
        self.assertEqual(parent, self.project)
        self.assertEqual(set(files), {"main.cherri", "helper.cherri"})
        self.assertEqual(files["main.cherri"], before)
        self.assertEqual(self.source.read_bytes(), before)

    def test_legacy_home_alias_is_resolved_without_rejecting_owned_source(self):
        alias = self.root / "legacy-home"
        alias.symlink_to(self.jobs, target_is_directory=True)
        source, _, _ = service.project_snapshot(alias / "project/main.cherri", self.jobs)
        self.assertEqual(source, self.source)

    def test_outside_source_rejected(self):
        outside = self.root / "outside.cherri"
        outside.write_text("nothing()\n")
        with self.assertRaises(service.ContractError):
            service.project_snapshot(outside, self.jobs)

    def test_input_symlink_rejected(self):
        alias = self.project / "link.cherri"
        alias.symlink_to(self.source)
        with self.assertRaises(service.ContractError):
            service.project_snapshot(alias, self.jobs)

    def test_nested_symlink_rejected(self):
        (self.project / "external").symlink_to(self.root, target_is_directory=True)
        with self.assertRaises(service.ContractError):
            service.project_snapshot(self.source, self.jobs)

    def test_output_name_escape_rejected(self):
        self.source.write_text('#define name ../../escape\nshow("pure")\n')
        with self.assertRaises(service.ContractError):
            service.project_snapshot(self.source, self.jobs)

    def test_include_escape_rejected(self):
        self.source.write_text("#include '../other/private.cherri'\n")
        with self.assertRaises(service.ContractError):
            service.project_snapshot(self.source, self.jobs)

    def test_upstream_action_include_is_unchanged(self):
        text = "#include 'actions/storage'\nstoreValue(\"k\", \"v\")\n"
        self.source.write_text(text)
        _, _, files = service.project_snapshot(self.source, self.jobs)
        self.assertEqual(files["main.cherri"], text.encode())

    def test_root_job_does_not_collect_other_projects(self):
        source = self.jobs / "single.cherri"
        source.write_text("nothing()\n")
        _, _, files = service.project_snapshot(source, self.jobs)
        self.assertEqual(set(files), {"single.cherri"})


class PureArtifactTests(unittest.TestCase):
    def test_unsigned_native_graph(self):
        data = plistlib.dumps({"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "is.workflow.actions.showresult",
                                                    "WFWorkflowActionParameters": {"Text": "public control"}}]})
        self.assertEqual(service.unsigned_actions(data), 1)

    def test_valid_empty_graph_is_not_fabricated_as_one_action(self):
        self.assertEqual(service.unsigned_actions(plistlib.dumps({"WFWorkflowActions": []})), 0)

    def test_invalid_graphs_rejected(self):
        for value in ({}, {"WFWorkflowActions": {}}, {"WFWorkflowActions": [False]},
                      {"WFWorkflowActions": [{"WFWorkflowActionIdentifier": 5}]},
                      {"WFWorkflowActions": [{"WFWorkflowActionIdentifier": "x", "WFWorkflowActionParameters": "bad"}]}):
            with self.subTest(value=value), self.assertRaises(service.ContractError):
                service.unsigned_actions(plistlib.dumps(value))

    def test_signed_input_is_not_relabelled_unsigned(self):
        with self.assertRaises(service.ContractError):
            service.unsigned_actions(b"AEA1not-a-plist")


class OriginalPackAdapterTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.base = Path(self.temporary.name)
        self.root = self.base / "canonical root $literal"
        self.root.mkdir()
        self.original = (Path(__file__).parent / "fixtures/cherri-with-actions-0.1.1.sh").read_text()
        self.assertEqual(service.digest(self.original.encode()), pack_adapter.WRAPPER_SHA256)

    def test_exact_one_root_assignment_only(self):
        changed = pack_adapter.adapted_wrapper(self.original, self.root)
        original_lines, changed_lines = self.original.splitlines(), changed.splitlines()
        differences = [(a, b) for a, b in zip(original_lines, changed_lines) if a != b]
        self.assertEqual(len(original_lines), len(changed_lines))
        self.assertEqual(len(differences), 1)
        self.assertEqual(differences[0][0], pack_adapter.OLD_ROOT)
        subprocess.run(["bash", "-n"], input=changed.encode(), check=True)

    def test_modified_original_wrapper_rejected(self):
        for changed in (self.original + "\n", self.original.replace("set -Eeuo", "set -Euo"),
                        self.original.replace(pack_adapter.OLD_ROOT, 'ROOT="/other"')):
            with self.subTest(digest=service.digest(changed.encode())), self.assertRaises(service.ContractError):
                pack_adapter.adapted_wrapper(changed, self.root)

    def test_literal_root_is_not_shell_code(self):
        changed = pack_adapter.adapted_wrapper(self.original, self.root)
        assignment = next(line for line in changed.splitlines() if line.startswith("ROOT="))
        result = subprocess.check_output(["bash", "-c", assignment + '\nprintf "%s" "$ROOT"'])
        self.assertEqual(result.decode(), str(self.root))

    def test_original_trap_unsafe_root_rejected(self):
        for name in ("root with ' apostrophe", "root\nnewline", "root\rcarriage"):
            unsafe = self.base / name
            unsafe.mkdir()
            with self.subTest(name=repr(name)), self.assertRaisesRegex(service.ContractError, "Original Pack exit trap"):
                pack_adapter.adapted_wrapper(self.original, unsafe)

    def test_original_alias_guard_red_and_narrow_adapter_green(self):
        # This synthetic callee records invocation only; actual compiler proof is separate.
        pack = self.root / "apple-actions-pack"
        (pack / "modules").mkdir(parents=True)
        (self.root / "bin").mkdir()
        (self.root / "jobs").mkdir()
        (self.root / "outputs").mkdir()
        (pack / "manifest.json").write_text("{}")
        (pack / "checksums.sha256").write_text(hashlib.sha256(b"{}").hexdigest() + "  manifest.json\n")
        for i in range(20):
            (pack / "modules" / ("module%d.cherri" % i)).write_text("// synthetic module\n")
        trace = self.base / "callee.jsonl"
        callee = self.root / "bin/pure-cherri-service.sh"
        callee.write_text("#!/usr/bin/python3\nimport os,sys,json,pathlib\n"
                          "p=pathlib.Path(os.environ['PURE_CONTROL_TRACE'])\n"
                          "with p.open('a') as f:f.write(json.dumps(sys.argv[1:])+'\\n')\n")
        callee.chmod(0o700)
        home = self.base / "logical-home"
        logical = home / "workspaces/cherri-remote/pure-upstream"
        logical.parent.mkdir(parents=True)
        logical.symlink_to(self.root, target_is_directory=True)
        source = self.root / "jobs/example.cherri"
        source.write_text('show("public control")\n')
        original_bytes = source.read_bytes()
        env = dict(os.environ, HOME=str(home), PURE_CONTROL_TRACE=str(trace))
        command = ["bash", "-c", self.original, "original-pack", "enhanced", "compile", str(source)]
        red = subprocess.run(command, capture_output=True, env=env, timeout=15)
        self.assertNotEqual(red.returncode, 0)
        self.assertIn(b"Source must be under", red.stderr)
        self.assertEqual([json.loads(x) for x in trace.read_text().splitlines()], [["verify"]])
        trace.unlink()
        command[2] = pack_adapter.adapted_wrapper(self.original, self.root)
        green = subprocess.run(command, capture_output=True, env=env, timeout=15)
        self.assertEqual(green.returncode, 0, green.stderr)
        calls = [json.loads(x) for x in trace.read_text().splitlines()]
        self.assertEqual(calls[0], ["verify"])
        self.assertEqual(calls[1][0], "compile")
        self.assertTrue(Path(calls[1][1]).is_relative_to(self.root / "jobs"))
        self.assertEqual(source.read_bytes(), original_bytes)



class PureLauncherTests(unittest.TestCase):
    def test_unknown_module_and_non_digest_pins_rejected(self):
        for module, manifest, code in (("../escape.py", "a" * 64, "b" * 64),
                                       ("service.py", "$(touch wrong)", "b" * 64),
                                       ("service.py", "a" * 64, "not-a-digest")):
            with self.subTest(module=module, manifest=manifest), self.assertRaises(service.ContractError):
                assemble.launcher(module, manifest, code)

    def test_launcher_passes_literal_arguments_and_checks_code(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / "root with spaces"
            (root / "bin").mkdir(parents=True)
            (root / "lib").mkdir()
            control = b"import sys,json; print(json.dumps(sys.argv[1:]))\n"
            (root / "lib/service.py").write_bytes(control)
            script = root / "bin/pure-cherri-service.sh"
            script.write_text(assemble.launcher("service.py", "a" * 64, service.digest(control)))
            script.chmod(0o700)
            payload = "$(touch should-not-exist)"
            result = subprocess.check_output([str(script), "docs", payload])
            arguments = json.loads(result)
            self.assertEqual(arguments, ["--root", str(root), "--manifest-sha256", "a" * 64, "docs", payload])
            self.assertFalse((root / "should-not-exist").exists())
            (root / "lib/service.py").write_bytes(control + b"# changed\n")
            rejected = subprocess.run([str(script), "verify"], capture_output=True)
            self.assertNotEqual(rejected.returncode, 0)
            self.assertIn(b"code digest mismatch", rejected.stderr)

    def test_pack_launcher_keeps_original_flags_after_separator(self):
        text = assemble.launcher("pack_adapter.py", "a" * 64, "b" * 64, True, service_sha="c" * 64)
        self.assertTrue(text.endswith('-- "$@"\n'))
        subprocess.run(["sh", "-n"], input=text.encode(), check=True)

    def test_pack_dependency_is_verified_before_import(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / "root"
            (root / "bin").mkdir(parents=True)
            (root / "lib").mkdir()
            marker = root / "untrusted-import-side-effect"
            original_service = (Path(__file__).parent / "service.py").read_bytes()
            original_adapter = (Path(__file__).parent / "pack_adapter.py").read_bytes()
            (root / "lib/pack_adapter.py").write_bytes(original_adapter)
            (root / "lib/service.py").write_text(
                "from pathlib import Path\nPath(" + repr(str(marker)) + ").write_text('executed')\n"
            )
            script = root / "bin/cherri-with-actions.sh"
            script.write_text(assemble.launcher(
                "pack_adapter.py", "a" * 64, service.digest(original_adapter), True,
                service_sha=service.digest(original_service)))
            script.chmod(0o700)
            result = subprocess.run([str(script), "verify"], capture_output=True, timeout=10)
            self.assertNotEqual(result.returncode, 0)
            self.assertFalse(marker.exists(), "service.py executed before its digest was checked")
            self.assertIn(b"code digest mismatch", result.stderr)

if __name__ == "__main__":
    unittest.main()
