#!/usr/bin/env python3
"""Causal local-Git controls for diagnostic component source receipts; no native run."""
import contextlib
import hashlib
import importlib.util
import io
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch


HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location(
    "workspace_source", HERE / "record_workspace_presentation_source.py")
source = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(source)
WORKFLOW = HERE.parents[1] / ".github/workflows/ios-build.yml"


class WorkspacePresentationSourceTests(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory(prefix="workspace-source-control-")
        self.addCleanup(self.temporary.cleanup)
        self.repo = Path(self.temporary.name).resolve()
        self.git("init", "--quiet")
        for name in source.SOURCE_FILES:
            path = self.repo / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text("synthetic source control: " + name + "\n", encoding="utf-8")
            path.chmod(0o644)
        self.git("add", "--", *source.SOURCE_FILES)
        self.git("commit", "--quiet", "-m", "Synthetic source receipt control")
        self.head = self.git("rev-parse", "HEAD").strip()
        self.environment = {
            "GITHUB_REPOSITORY": "davidpovarsky/cherri", "GITHUB_WORKFLOW": "iOS Build",
            "GITHUB_EVENT_NAME": "workflow_dispatch", "GITHUB_SHA": self.head,
            "GITHUB_WORKFLOW_SHA": self.head, "GITHUB_RUN_ID": "123", "GITHUB_RUN_ATTEMPT": "1",
        }
        self.context = source.context_from_environment(self.environment)

    def git(self, *args):
        result = subprocess.run(
            ["/usr/bin/git", "-c", "user.name=Component Control", "-c",
             "user.email=component-control@example.invalid", "-c", "core.hooksPath=/dev/null",
             "-c", "commit.gpgsign=false", "-C", str(self.repo), *args],
            env=dict(os.environ, GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull),
            stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=10, check=True)
        return result.stdout

    def record(self, phase, context=None):
        with contextlib.redirect_stdout(io.StringIO()):
            return source.record(self.repo, phase, self.context if context is None else context)

    def test_closed_modes_and_conflicts(self):
        for documentation in ("false", "true"):
            for startup in ("false", "true"):
                for presentation in ("false", "true"):
                    with self.subTest(documentation=documentation, startup=startup,
                                      presentation=presentation):
                        if presentation == "true" and (documentation == "true" or startup == "true"):
                            with self.assertRaisesRegex(ValueError, "COMPONENT_MODE_CONFLICT"):
                                source.validate_mode(documentation, startup, presentation)
                        else:
                            source.validate_mode(documentation, startup, presentation)
        for invalid in ("", "True", "1", True, None):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(ValueError, "COMPONENT_MODE_VALUE"):
                    source.validate_mode("false", "false", invalid)

    def test_source_run_and_workflow_contexts_are_exact(self):
        mutations = {
            "GITHUB_REPOSITORY": "other/cherri", "GITHUB_WORKFLOW": "Other Build",
            "GITHUB_EVENT_NAME": "push", "GITHUB_SHA": "abc",
            "GITHUB_WORKFLOW_SHA": "f" * 40, "GITHUB_RUN_ID": "0", "GITHUB_RUN_ATTEMPT": "0",
        }
        for key, value in mutations.items():
            with self.subTest(key=key):
                with self.assertRaises(ValueError):
                    source.context_from_environment(dict(self.environment, **{key: value}))

    def test_real_git_before_after_preserves_all_declared_bytes(self):
        before = self.record("before")
        after = self.record("after")
        self.assertEqual(before["context"], self.context)
        self.assertEqual(before["source"], after["source"])
        self.assertEqual(set(before["source"]["files"]), set(source.SOURCE_FILES))
        self.assertEqual(len(before["source"]["files"]), 9)
        self.assertEqual(before["methods"], list(source.METHODS))
        self.assertEqual(before["scope"], "declared-ui-component-source-only")
        self.assertIs(before["nativeAcceptanceGranted"], False)
        self.assertIs(after["nativeAcceptanceGranted"], False)
        original = self.repo / "ios/.build/workspace-presentation-source/before.original.json"
        self.assertEqual(after["beforeSHA256"], hashlib.sha256(original.read_bytes()).hexdigest())
        for name, witness in before["source"]["files"].items():
            self.assertEqual(witness["sha256"], hashlib.sha256((self.repo / name).read_bytes()).hexdigest())
        self.assertEqual(self.git("status", "--porcelain", "--untracked-files=no"), "")

    def test_actual_source_bytes_mode_and_missing_file_rejected(self):
        path = self.repo / "ios/CherriApp/App/WorkspaceView.swift"
        original = path.read_bytes()
        for fault in ("bytes", "mode", "missing"):
            with self.subTest(fault=fault):
                if fault == "bytes":
                    path.write_bytes(original + b"changed\n")
                elif fault == "mode":
                    path.chmod(0o755)
                else:
                    path.unlink()
                with self.assertRaises((ValueError, FileNotFoundError)):
                    self.record("before")
                path.write_bytes(original)
                path.chmod(0o644)
        self.assertFalse((self.repo / "ios/.build/workspace-presentation-source").exists())

    def test_same_declared_files_cannot_hide_a_source_ref_transition(self):
        original_git = source.git
        moved = []

        def transition(repo, args, deadline):
            result = original_git(repo, args, deadline)
            if args == ["rev-parse", "HEAD"] and not moved:
                self.git("commit", "--allow-empty", "--quiet", "-m", "Actual ref transition")
                moved.append(self.git("rev-parse", "HEAD").strip())
            return result

        with patch.object(source, "git", side_effect=transition):
            with self.assertRaisesRegex(ValueError, "COMPONENT_SOURCE_CHANGED"):
                self.record("before")
        self.assertEqual(len(moved), 1)
        self.assertNotEqual(moved[0], self.head)
        self.assertEqual(self.git("diff", self.head, moved[0], "--", *source.SOURCE_FILES), "")
        self.assertFalse((self.repo / "ios/.build/workspace-presentation-source").exists())

    def test_wrong_head_and_alias_source_rejected(self):
        with self.assertRaisesRegex(ValueError, "COMPONENT_SOURCE_CHANGED"):
            self.record("before", dict(self.context, source="0" * 40))
        path = self.repo / "ios/CherriApp/App/WorkspaceView.swift"
        original = path.read_bytes()
        target = self.repo / "same-bytes.swift"
        target.write_bytes(original)
        path.unlink()
        path.symlink_to(target)
        with self.assertRaisesRegex(ValueError, "COMPONENT_REGULAR_FILE"):
            self.record("before")
        path.unlink()
        path.write_bytes(original)
        path.chmod(0o644)
        alias = self.repo / "root-alias"
        alias.symlink_to(self.repo, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "COMPONENT_SOURCE_ROOT"):
            source.source_snapshot(alias, self.context)

    def test_original_receipt_tamper_duplicate_and_missing_rejected(self):
        with self.assertRaisesRegex(ValueError, "COMPONENT_BEFORE_REQUIRED"):
            self.record("after")
        before = self.record("before")
        path = self.repo / "ios/.build/workspace-presentation-source/before.original.json"
        original = path.read_bytes()
        with self.assertRaises(FileExistsError):
            self.record("before")
        self.assertEqual(path.read_bytes(), original)
        changed = dict(before, nativeAcceptanceGranted=True)
        path.write_text(json.dumps(changed, sort_keys=True, indent=2) + "\n", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "COMPONENT_BEFORE_CHANGED"):
            self.record("after")
        path.write_bytes(original)
        self.record("after")
        after_path = path.with_name("after.original.json")
        after = after_path.read_bytes()
        with self.assertRaises(FileExistsError):
            self.record("after")
        self.assertEqual(after_path.read_bytes(), after)

    def test_output_alias_rejected_before_original_write(self):
        target = self.repo / "other-output"
        target.mkdir()
        output = self.repo / "ios/.build/workspace-presentation-source"
        output.parent.mkdir(parents=True)
        output.symlink_to(target, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "COMPONENT_OUTPUT_ALIAS"):
            self.record("before")
        self.assertEqual(list(target.iterdir()), [])

    def test_workflow_fixed_selection_and_diagnostic_only_mode(self):
        workflow = WORKFLOW.read_text(encoding="utf-8")
        selection = workflow.split("          STARTUP_SELECTION=()\n", 1)[1].split(
            "\n          defaults write com.apple.iphonesimulator", 1)[0]
        diagnostic = workflow.split("          DIAGNOSTIC_ARGS=()\n", 1)[1].split(
            "\n          python3 scripts/collect_ios_test_evidence.py", 1)[0]
        original_startup = [
            "CherriUITests/CherriEditorUITests/testBRG27EditorRealWorldWorkflowUI",
            "CherriUITests/CherriEditorUITests/testD03CurrentDocumentationSearchUI",
            "CherriUITests/CherriEditorUITests/testE01CompletionDisplayAndInsertionUI",
        ]
        for startup, presentation, expected in (
                ("false", "false", []), ("true", "false", original_startup),
                ("false", "true", list(source.METHODS))):
            with self.subTest(startup=startup, presentation=presentation):
                script = "STARTUP_SELECTION=()\n" + selection + "\nprintf '%s\\0' \"${STARTUP_SELECTION[@]}\"\n"
                result = subprocess.run(["/bin/bash", "-c", script],
                    env=dict(os.environ, STARTUP_OBSERVATION_ONLY=startup,
                             WORKSPACE_PRESENTATION_ONLY=presentation),
                    capture_output=True, timeout=5, check=True)
                actual = [x.decode() for x in result.stdout.split(b"\0") if x]
                self.assertEqual(actual, ["-only-testing:" + x for x in expected])
                script = "DIAGNOSTIC_ARGS=()\n" + diagnostic + "\nprintf '%s\\0' \"${DIAGNOSTIC_ARGS[@]}\"\n"
                result = subprocess.run(["/bin/bash", "-c", script],
                    env=dict(os.environ, STARTUP_OBSERVATION_ONLY=startup,
                             WORKSPACE_PRESENTATION_ONLY=presentation),
                    capture_output=True, timeout=5, check=True)
                self.assertEqual([x.decode() for x in result.stdout.split(b"\0") if x],
                                 ["--diagnostic-only"] if expected else [])
        self.assertEqual(workflow.count("timeout-minutes: 75"), 1)
        self.assertEqual(workflow.count("record_workspace_presentation_source.py mode"), 2)
        self.assertIn("default: false\n      runner_label:", workflow)


    def enable_joined_profile(self):
        for name in source.JOINED_SOURCE_FILES:
            path = self.repo / name
            if not path.exists():
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text("synthetic joined component control: " + name + "\n", encoding="utf-8")
                path.chmod(0o644)
        self.git("add", "--", *source.JOINED_SOURCE_FILES)
        self.git("commit", "--quiet", "-m", "Synthetic dedicated component source")
        self.head = self.git("rev-parse", "HEAD").strip()
        self.environment.update(GITHUB_SHA=self.head, GITHUB_WORKFLOW_SHA=self.head)
        self.context = source.context_from_environment(self.environment)

    def test_joined_profile_binds_exact_five_methods_and_current_sources(self):
        self.enable_joined_profile()
        expected = [
            "CherriTests/CherriWorkspacePresentationTests/testCompactPaneTransitionsKeepNativeViewsAndDocumentState",
            "CherriTests/CherriWorkspacePresentationTests/testCompactUncompiledDocumentsDoNotInstantiateWebKit",
            "CherriTests/CherriWorkspacePresentationTests/testDocumentationSearchUsesLiteralNativeInputTraits",
            "CherriUITests/CherriJoinedContractUITests/testRP39ActualCombinedEditorAndDelayedAnalysisUI",
            "CherriUITests/CherriPreviewDocumentUITests/testRP41ActualMappedAndGeneratedPreviewSourceUI",
        ]
        with contextlib.redirect_stdout(io.StringIO()):
            before = source.record(self.repo, "before", self.context, joined=True)
            after = source.record(self.repo, "after", self.context, joined=True)
        self.assertEqual(before["methods"], expected)
        self.assertEqual(before["schema"], "cherri-joined-editor-component-source/1")
        self.assertEqual(before["scope"], "declared-joined-editor-component-source-only")
        self.assertIs(before["nativeAcceptanceGranted"], False)
        self.assertIs(after["nativeAcceptanceGranted"], False)
        self.assertEqual(before["source"], after["source"])
        self.assertEqual(len(before["source"]["files"]), 20)
        for path in (
                "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift",
                "ios/CherriApp/Compiler/CherriCompiler.swift",
                "ios/CherriApp/Compiler/CherriDocumentCompileContext.swift",
                "ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
                "ios/CherriApp/Preview/CherriPreviewEditTransaction.swift",
                "ios/CherriApp/Preview/CherriPreviewRenderState.swift",
                "ios/CherriUITests/CherriJoinedContractUITests.swift",
                "ios/CherriUITests/CherriPreviewDocumentUITests.swift",
                "ios/TestFixtures/joined-editor-contract.json",
                "ios/scripts/joined_editor_evidence.py"):
            self.assertIn(path, before["source"]["files"])
        self.assertTrue((self.repo / "ios/.build/joined-editor-component-source/after.original.json").is_file())
        self.assertFalse((self.repo / "ios/.build/workspace-presentation-source").exists())

    def test_joined_profile_rejects_changed_or_missing_real_analysis_observer(self):
        self.enable_joined_profile()
        path = self.repo / "ios/CherriApp/Diagnostics/EditorDeliveryObservation.swift"
        original = path.read_bytes()
        healthy = source.source_snapshot(self.repo, self.context, source.JOINED_SOURCE_FILES)
        self.assertEqual(len(healthy["files"]), 20)
        for fault in ("bytes", "mode", "missing"):
            with self.subTest(fault=fault):
                if fault == "bytes":
                    path.write_bytes(original + b"changed\n")
                elif fault == "mode":
                    path.chmod(0o755)
                else:
                    path.unlink()
                with self.assertRaises((ValueError, FileNotFoundError)):
                    source.record(self.repo, "before", self.context, joined=True)
                path.write_bytes(original)
                path.chmod(0o644)
        self.assertFalse((self.repo / "ios/.build/joined-editor-component-source").exists())

    def test_joined_mode_selects_only_fixed_five_and_remains_diagnostic(self):
        source.validate_mode("false", "false", "false", "true")
        for documentation, startup, presentation in (
                ("true", "false", "false"), ("false", "true", "false"),
                ("false", "false", "true"), ("true", "true", "true")):
            with self.subTest(documentation=documentation, startup=startup, presentation=presentation):
                with self.assertRaisesRegex(ValueError, "COMPONENT_MODE_CONFLICT"):
                    source.validate_mode(documentation, startup, presentation, "true")
        for invalid in ("", "True", "1", True, None):
            with self.subTest(invalid=invalid):
                with self.assertRaisesRegex(ValueError, "COMPONENT_MODE_VALUE"):
                    source.validate_mode("false", "false", "false", invalid)
        workflow = WORKFLOW.read_text(encoding="utf-8")
        selection = workflow.split("          STARTUP_SELECTION=()\n", 1)[1].split(
            "\n          defaults write com.apple.iphonesimulator", 1)[0]
        diagnostic = workflow.split("          DIAGNOSTIC_ARGS=()\n", 1)[1].split(
            "\n          python3 scripts/collect_ios_test_evidence.py", 1)[0]
        env = dict(os.environ, STARTUP_OBSERVATION_ONLY="false",
                   WORKSPACE_PRESENTATION_ONLY="false", JOINED_EDITOR_COMPONENT_ONLY="true")
        selected = subprocess.run(["/bin/bash", "-c", "STARTUP_SELECTION=()\n" + selection
            + "\nprintf '%s\\0' \"${STARTUP_SELECTION[@]}\"\n"],
            env=env, capture_output=True, timeout=5, check=True)
        self.assertEqual([v.decode() for v in selected.stdout.split(b"\0") if v],
                         ["-only-testing:" + v for v in source.JOINED_METHODS])
        observed = subprocess.run(["/bin/bash", "-c", "DIAGNOSTIC_ARGS=()\n" + diagnostic
            + "\nprintf '%s\\0' \"${DIAGNOSTIC_ARGS[@]}\"\n"],
            env=env, capture_output=True, timeout=5, check=True)
        self.assertEqual([v.decode() for v in observed.stdout.split(b"\0") if v], ["--diagnostic-only"])
        self.assertEqual(workflow.count("timeout-minutes: 75"), 1)
        self.assertEqual(len(source.METHODS), 3)


if __name__ == "__main__":
    unittest.main(verbosity=2)
