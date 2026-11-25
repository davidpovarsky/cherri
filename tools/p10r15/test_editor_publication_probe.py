import copy
import json
from pathlib import Path
import sys
import tempfile
import unittest

import editor_publication_probe as probe


class EditorPublicationProbeIntakeTests(unittest.TestCase):
    def control_rows(self):
        # Synthetic intake controls only; never passed to a production registrar.
        return [
            {"event": "control", "name": name, "result": "PASS",
             "scope": "controlled adapter; actual production method body", "observation": {}}
            for name in sorted(probe.CONTROL_NAMES)
        ] + [{"event": "summary", "controls": len(probe.CONTROL_NAMES), "failed": 0, "uiExecuted": False, "realCompilerExecuted": False}]

    def test_synthetic_complete_parser_control(self):
        controls, summary = probe.inspect_output("\n".join(json.dumps(row) for row in self.control_rows()).encode())
        self.assertEqual(len(probe.ORIGINAL_CONTROL_NAMES), 26)
        self.assertEqual(len(probe.DOCUMENT_CONTROL_NAMES), 12)
        self.assertEqual(len(controls), 38)
        self.assertEqual(summary["failed"], 0)

    def test_rejects_selection_scope_and_summary_mutations(self):
        for name in ("missing", "duplicate", "unknown", "wrong_event", "unearned_ui", "unearned_compiler", "dropped_failure", "boolean_count", "invalid_result"):
            with self.subTest(name=name):
                rows = self.control_rows()
                if name == "missing": rows.pop(0)
                elif name == "duplicate": rows[1] = copy.deepcopy(rows[0])
                elif name == "unknown": rows[0]["name"] = "foreign"
                elif name == "wrong_event": rows[0]["event"] = "unused"
                elif name == "unearned_ui": rows[-1]["uiExecuted"] = True
                elif name == "unearned_compiler": rows[-1]["realCompilerExecuted"] = True
                elif name == "dropped_failure": rows[0]["result"] = "FAIL"
                elif name == "boolean_count": rows[-1]["failed"] = False
                elif name == "invalid_result": rows[0]["result"] = "SKIP"
                with self.assertRaises(ValueError):
                    probe.inspect_output("\n".join(json.dumps(row) for row in rows).encode())

    def test_preserves_failed_actual_control_outcome(self):
        rows = self.control_rows()
        rows[0]["result"] = "FAIL"
        rows[-1]["failed"] = 1
        controls, summary = probe.inspect_output("\n".join(json.dumps(row) for row in rows).encode())
        self.assertEqual(controls[0]["result"], "FAIL")
        self.assertEqual(summary["failed"], 1)

    def test_region_requires_unique_unconditional_ordered_ownership(self):
        self.assertEqual(probe.region("A\nbody\nB", "A", "B"), "A\nbody")
        for text in ("A A body B", "A body", "B body A", "A\n#if false\nbody\nB"):
            with self.subTest(text=text), self.assertRaises(ValueError):
                probe.region(text, "A", "B")

    def test_rejects_invalid_process_time_or_status(self):
        original = {"exit_code": 0, "timed_out": False, "within_budget": True, "duration_seconds": 1.0, "timeout_seconds": 2.0}
        self.assertTrue(probe.successful(original))
        for field, value in [("exit_code", False), ("exit_code", 1), ("timed_out", True), ("within_budget", False), ("duration_seconds", 3), ("duration_seconds", -1), ("duration_seconds", float("nan")), ("duration_seconds", float("inf")), ("timeout_seconds", 0)]:
            changed = dict(original); changed[field] = value
            with self.subTest(field=field, value=value):
                self.assertFalse(probe.successful(changed))

    def test_actual_finite_child(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            result = probe.invoke([sys.executable, "-c", "print('OWNED_CONTROL')"], root, root, "child", 5)
            self.assertTrue(probe.successful(result), result)
            self.assertEqual((root / "child.stdout").read_bytes().strip(), b"OWNED_CONTROL")

    def test_actual_timed_out_owned_child_is_not_success(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            result = probe.invoke([sys.executable, "-c", "import time; time.sleep(5)"], root, root, "child", 0.05)
            self.assertTrue(result["timed_out"], result)
            self.assertFalse(probe.successful(result), result)


    def test_actual_generation_owns_request_context_and_wire_dependencies(self):
        repo = Path(__file__).resolve().parents[2]
        generated, files = probe.generate(repo)
        self.assertEqual(set(files), {
            "ios/CherriApp/App/WorkspaceView.swift",
            "ios/CherriApp/Editor/CherriEditorView.swift",
            "ios/CherriApp/Editor/CherriCompletionRequest.swift",
            "ios/CherriApp/Editor/CherriEditorBuildRequest.swift",
            "ios/CherriApp/Compiler/CherriCompiler.swift",
            "ios/CherriApp/Compiler/CherriDocumentCompileContext.swift",
            "tools/p10r15/editor_publication_probe.py",
            "tools/p10r15/editor_publication_probe.swift",
        })
        workspace = files["ios/CherriApp/App/WorkspaceView.swift"].decode()
        request = probe.region(workspace, "    private var currentBuildRequest:",
                               "\n    @MainActor\n    private func formatSource")
        self.assertIn(request.replace("private var ", "var ", 1), generated)
        compiler = files["ios/CherriApp/Compiler/CherriCompiler.swift"].decode()
        self.assertIn(probe.completion_types(compiler), generated)
        for name in ("CherriCompletionItem", "CherriWirePosition", "CherriWireRange", "CherriTextEditItem"):
            self.assertEqual(generated.count("struct "+name+":"), 1)
        self.assertNotIn("__ACTUAL_", generated)

    def test_wire_dependency_ownership_rejects_missing_duplicate_and_conditional_regions(self):
        repo = Path(__file__).resolve().parents[2]
        original = (repo/"ios/CherriApp/Compiler/CherriCompiler.swift").read_text()
        for marker in ("struct CherriCompletionItem:", "struct CherriWirePosition:", "struct CherriTextEditItem:"):
            for changed in (original.replace(marker, "struct MissingDependency:", 1),
                            original + "\n"+marker,
                            original.replace(marker, marker+"\n#if false\n", 1)):
                with self.subTest(marker=marker, changed=changed[-80:]), self.assertRaises(ValueError):
                    probe.completion_types(changed)


if __name__ == "__main__":
    unittest.main()
