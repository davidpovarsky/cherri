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
        self.assertEqual(len(controls), 26)
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


if __name__ == "__main__":
    unittest.main()
