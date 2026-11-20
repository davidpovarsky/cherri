#!/usr/bin/env python3
"""Collector controls only; synthetic process payloads never count as compiler evidence."""
import importlib.util
import json
from pathlib import Path
import plistlib
import subprocess
import sys
import tempfile
import time
import unittest
from unittest.mock import patch

HERE = Path(__file__).resolve().parent
SPEC = importlib.util.spec_from_file_location("documentation_recovery_examples", HERE / "documentation_recovery_examples.py")
collector = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(collector)
SOURCE = "a" * 40
FENCE = chr(96) * 3
PINS = {
    "languageVersion": "2.0", "catalogSchemaVersion": "3.0",
    "schemaFingerprint": "b" * 64, "codecFingerprint": "c" * 64,
    "macroFingerprint": "d" * 64,
}
CAPABILITIES = {**PINS, "protocolVersion": "1.0"}
BUILD_INFO = (
    "public-control: go1.25.6\n"
    "\tpath\tgithub.com/electrikmilk/cherri\n"
    "\tbuild\tvcs=git\n"
    "\tbuild\tvcs.revision=" + SOURCE + "\n"
    "\tbuild\tvcs.modified=false\n"
    "\tbuild\tGOOS=linux\n\tbuild\tGOARCH=amd64\n"
).encode()


def synthetic_receipt(command, cwd, code=0):
    return {
        "command": list(command), "cwd": str(cwd),
        "exitCode": code, "timedOut": False, "budgetSeconds": 30,
        "elapsedSeconds": 0.001,
        "stdout": {"path": "synthetic.stdout", "bytes": 0, "sha256": collector.digest(b"")},
        "stderr": {"path": "synthetic.stderr", "bytes": 0, "sha256": collector.digest(b"")},
    }


class IdentityControls(unittest.TestCase):
    def identity(self, caps, pins=None, metadata=BUILD_INFO):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            binary = root / "synthetic-nonexecuted-binary"
            binary.write_bytes(b"PUBLIC_SYNTHETIC_IDENTITY_CONTROL")
            def observed(output, name, command, cwd, budget=30):
                raw = metadata if name == "compiler-build-info" else json.dumps(caps).encode()
                return synthetic_receipt(command, cwd), raw, b""
            with patch.object(collector, "process", observed):
                return collector.compiler_identity(binary, SOURCE, pins or PINS, root)

    def test_documented_capabilities_do_not_invent_compiler_commit_field(self):
        self.assertNotIn("compilerCommit", CAPABILITIES)
        self.identity(CAPABILITIES)

    def test_protocol_is_supported_independently_of_manifest_omission(self):
        caps = {**CAPABILITIES, "compilerCommit": SOURCE}
        self.assertNotIn("protocolVersion", PINS)
        self.identity(caps)

    def test_wrong_vcs_and_capability_values_are_rejected(self):
        pins = {**PINS, "protocolVersion": "1.0"}
        caps = {**CAPABILITIES, "compilerCommit": SOURCE}
        for name, metadata, values in (
            ("wrong-vcs", BUILD_INFO.replace(SOURCE.encode(), b"e" * 40), caps),
            ("dirty-vcs", BUILD_INFO.replace(b"vcs.modified=false", b"vcs.modified=true"), caps),
            ("wrong-role", BUILD_INFO.replace(b"github.com/electrikmilk/cherri\n", b"github.com/electrikmilk/cherri/cmd/cherri-migrate\n"), caps),
            ("wrong-codec", BUILD_INFO, {**caps, "codecFingerprint": "e" * 64}),
            ("wrong-protocol", BUILD_INFO, {**caps, "protocolVersion": "2.0"}),
        ):
            with self.subTest(name=name):
                with self.assertRaises(ValueError):
                    self.identity(values, pins, metadata)


class DeadlineControls(unittest.TestCase):
    def test_real_child_exit_zero_after_original_budget_is_not_success(self):
        original = subprocess.Popen
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            marker = root / "child-started"
            def delayed_launch(*args, **kwargs):
                child = original(*args, **kwargs)
                end = time.monotonic() + 2
                while not marker.exists() and time.monotonic() < end:
                    time.sleep(0.005)
                self.assertTrue(marker.exists(), "the actual child must run")
                time.sleep(0.08)
                return child
            with patch.object(collector.subprocess, "Popen", delayed_launch):
                row, _, _ = collector.process(root, "late-exit-zero",
                    [sys.executable, "-c", "from pathlib import Path;Path(" + repr(str(marker)) + ").write_text('PUBLIC_STARTED')"],
                    root, budget=0.02)
            self.assertGreater(row["elapsedSeconds"], row["budgetSeconds"])
            self.assertTrue(row["timedOut"],
                "late real exit zero cannot satisfy the original deadline")

    def test_real_child_control_is_observed_and_raw_bytes_retained(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            row, out, err = collector.process(root, "real-control",
                [sys.executable, "-c", "import sys;sys.stdout.buffer.write(b'PUBLIC_CONTROL\\x00\\n');sys.stderr.write('PUBLIC_STDERR\\n')"],
                root, budget=5)
            self.assertEqual(row["exitCode"], 0)
            self.assertFalse(row["timedOut"])
            self.assertLessEqual(row["elapsedSeconds"], row["budgetSeconds"])
            self.assertEqual(out, b"PUBLIC_CONTROL\x00\n")
            self.assertEqual(err.replace(b"\r\n", b"\n"), b"PUBLIC_STDERR\n")
            self.assertEqual(row["stdout"]["sha256"], collector.digest(out))
            self.assertEqual(row["stderr"]["sha256"], collector.digest(err))


class DiagnosticControls(unittest.TestCase):
    def execute(self, diagnostic, expected=(), compile_error=b""):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            case = {"mode": "v2", "source": 'text("PUBLIC_SYNTHETIC")\n',
                    "expectedErrors": list(expected)}
            def observed(output, name, command, cwd, budget=30):
                if name == "check":
                    code = 1 if expected else 0
                    return synthetic_receipt(command, cwd, code), json.dumps(diagnostic).encode(), b""
                if not expected:
                    (Path(output) / "snippet_unsigned.shortcut").write_bytes(plistlib.dumps({
                        "WFWorkflowActions": [{"WFWorkflowActionIdentifier": "is.workflow.actions.nothing"}]}))
                return synthetic_receipt(command, cwd, 1 if expected else 0), b"", compile_error
            with patch.object(collector, "process", observed):
                return collector.execute_example(root / "not-executed", case, root / "example")

    def test_malformed_empty_diagnostic_container_cannot_pass(self):
        for invalid in ({}, False, "", None):
            with self.subTest(value=repr(invalid)):
                row = self.execute({"valid": True, "diagnostics": invalid})
                self.assertFalse(row["passed"], "non-array diagnostics were accepted")

    def test_compile_failure_must_match_the_intended_diagnostic(self):
        row = self.execute({"valid": False, "diagnostics": [
            {"severity": "ERROR", "code": "E_ARGUMENT_TYPE", "message": "PUBLIC_TYPE_ERROR"}]},
            expected=("E_ARGUMENT_TYPE",), compile_error=b"UNRELATED_CRASH_OR_PATH_FAILURE\n")
        self.assertFalse(row["passed"], "an unrelated compile failure counted as an intended-error example")

    def test_synthetic_valid_control_is_not_runtime_or_compiler_credit(self):
        row = self.execute({"valid": True, "diagnostics": []})
        self.assertTrue(row["passed"])


class InventoryControls(unittest.TestCase):
    def test_unclosed_cherri_fence_cannot_disappear_from_inventory(self):
        with self.assertRaises(ValueError):
            collector.fence_examples("Heading\n" + FENCE + 'cherri\ntext("PUBLIC")\n', "compiler", "guide.md")

    def test_exact_original_fenced_bytes_are_retained(self):
        raw = '\n  text("PUBLIC")  \n'
        cases = collector.fence_examples(FENCE + "cherri\n" + raw + FENCE + "\n", "compiler", "guide.md")
        self.assertEqual(len(cases), 1)
        self.assertEqual(cases[0]["originalSnippet"], raw)
        self.assertEqual(cases[0]["originalSnippetSHA256"], collector.digest(raw.encode()))

    def test_complete_valid_and_intended_error_inventory_controls(self):
        text = (FENCE + 'cherri\ntext("PUBLIC")\n' + FENCE + '\n'
                '<!-- cherri-test: diagnostic E_ARGUMENT_TYPE -->\n'
                + FENCE + 'cherri\nwait("wrong")\n' + FENCE + '\n')
        cases = collector.fence_examples(text, "compiler", "guide.md")
        self.assertEqual(len(cases), 2)
        self.assertEqual(cases[0]["expectedErrors"], [])
        self.assertEqual(cases[1]["expectedErrors"], ["E_ARGUMENT_TYPE"])


if __name__ == "__main__":
    unittest.main(verbosity=2)
