#!/usr/bin/env python3
"""Collector controls only; synthetic process payloads never count as compiler evidence."""
import copy
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
sys.path.insert(0, str(HERE))
import documentation_recovery_intake as intake
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
        for invalid in ({}, False, ""):
            with self.subTest(value=repr(invalid)):
                row = self.execute({"valid": True, "diagnostics": invalid})
                self.assertFalse(row["passed"], "non-array diagnostics were accepted")

    def test_documented_nil_slice_wire_is_empty_only_when_valid(self):
        # The actual f05 public check emits diagnostics:null for no diagnostics;
        # its original bytes are retained in the component evidence.
        self.assertTrue(self.execute({"valid": True, "diagnostics": None})["passed"])
        self.assertFalse(self.execute({"valid": False, "diagnostics": None})["passed"])
        self.assertFalse(self.execute({"valid": True})["passed"])

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


class PinnedInventoryControls(unittest.TestCase):
    """Real Git source controls; these metadata fixtures grant no execution credit."""

    def git(self, root, *args):
        return subprocess.run(["git", "--no-replace-objects",
            "-c", "user.name=Public inventory control",
            "-c", "user.email=inventory-control@example.invalid", *args],
            cwd=root, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
            check=True, timeout=30).stdout

    def write(self, root, path, text):
        target = root / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding="utf8")

    def commit(self, root):
        self.git(root, "add", "--all")
        self.git(root, "commit", "--quiet", "-m", "Public source inventory fixture")
        return self.git(root, "rev-parse", "HEAD").decode().strip()

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        base = Path(self.temp.name)
        self.compiler, self.docs, self.consumer = (base / role for role in ("compiler", "docs", "consumer"))
        for root in (self.compiler, self.docs, self.consumer):
            root.mkdir()
            self.git(root, "init", "--quiet")
        self.valid = FENCE + 'cherri\ntext("PUBLIC_INVENTORY")\n' + FENCE + "\n"
        self.invalid = ('<!-- cherri-test: diagnostic E_ARGUMENT_TYPE -->\n' +
            FENCE + 'cherri\nwait("PUBLIC_INTENDED_ERROR")\n' + FENCE + "\n")
        self.write(self.compiler, collector.COMPILER_GUIDES[0], self.valid + self.invalid)
        self.compiler_sha = self.commit(self.compiler)
        for index, path in enumerate(collector.DOCS_GUIDES):
            self.write(self.docs, path, self.valid.replace("cherri\n", "cherri-legacy\n", 1)
                       if index == 0 else self.valid)
        self.docs_sha = self.commit(self.docs)
        for path in collector.SKILL_GUIDES:
            self.write(self.consumer, path, self.valid + '\n## Public context\n' +
                       chr(96) + 'debugLog("PUBLIC_CONTEXT")' + chr(96) + '\n')
        self.pin_consumer()

    def pin_consumer(self):
        self.pins = {**PINS, "compilerCommit": self.compiler_sha, "docsCommit": self.docs_sha}
        self.pin_raw = (json.dumps(self.pins, sort_keys=True) + "\n").encode()
        self.write(self.consumer, "skills/cherri-shortcuts/compatibility-manifest.json",
                   self.pin_raw.decode())
        self.source = self.commit(self.consumer)

    def report(self):
        cases, sources, tokens = [], [], []
        for root, commit, role, paths in (
            (self.compiler, self.compiler_sha, "compiler", collector.COMPILER_GUIDES),
            (self.docs, self.docs_sha, "docs", collector.DOCS_GUIDES),
            (self.consumer, self.source, "consumer", collector.SKILL_GUIDES)):
            for path in paths:
                raw = self.git(root, "show", commit + ":" + path)
                sources.append({"repository": role, "commit": commit, "path": path,
                                "bytes": len(raw), "sha256": collector.digest(raw)})
                cases += collector.fence_examples(raw.decode(), role, path)
                if role == "consumer":
                    inline, inventory = collector.skill_examples(raw.decode(), path)
                    cases += inline
                    tokens += inventory
        return {"implementationSha": self.source, "compilerCommit": self.compiler_sha,
            "docsCommit": self.docs_sha, "manifestSha256": collector.digest(self.pin_raw),
            "formalAcceptanceGranted": False, "nativeExecution": "NOT_RUN",
            "sources": sources, "inlineInventory": tokens,
            "examples": [{**case, "passed": True, "failures": []} for case in cases],
            "summary": {"executed": len(cases), "passed": len(cases), "failed": 0,
                "intendedErrorExamples": sum(bool(c["expectedErrors"]) for c in cases),
                "explicitLegacyExamples": sum(c["mode"] == "legacy" for c in cases),
                "contextualExamples": sum(c["classification"] == "contextual" for c in cases)}}

    def verify(self, report):
        return intake.verify_example_inventory(report, self.pins, self.consumer,
            self.docs, self.source, compiler_repo=self.compiler)

    def test_complete_inventory_follows_new_immutable_source(self):
        before = self.report()
        self.assertEqual(len(self.verify(before)), 16)
        guide = collector.COMPILER_GUIDES[0]
        self.write(self.compiler, guide, self.valid + self.valid + self.invalid)
        self.compiler_sha = self.commit(self.compiler)
        self.pin_consumer()
        after = self.report()
        self.assertEqual(len(self.verify(after)), 17)
        self.assertFalse(after["formalAcceptanceGranted"])
        self.assertEqual(after["nativeExecution"], "NOT_RUN")
        with self.assertRaisesRegex(ValueError, "tuple"):
            self.verify(before)

    def test_omitted_duplicate_extra_misbound_and_failed_cases_rejected(self):
        mutations = {
            "omitted": lambda r: r["examples"].pop(),
            "duplicate": lambda r: r["examples"].__setitem__(1, copy.deepcopy(r["examples"][0])),
            "extra": lambda r: r["examples"].append(copy.deepcopy(r["examples"][0])),
            "misbound": lambda r: r["examples"][0].__setitem__("path", "different-guide.md"),
            "reordered": lambda r: r["examples"].reverse(),
            "boolean-ordinal": lambda r: r["examples"][0].__setitem__("ordinal", True),
            "false-result": lambda r: r["examples"][0].__setitem__("passed", False),
            "hidden-failure": lambda r: r["examples"][0].__setitem__("failures", ["PUBLIC_FAILURE"]),
            "different-code": lambda r: r["examples"][0].__setitem__("source", 'text("ALTERED")\n'),
            "summary-count": lambda r: r["summary"].__setitem__("passed", 88),
            "boolean-summary": lambda r: r["summary"].__setitem__("failed", False),
            "foreign-manifest": lambda r: r.__setitem__("manifestSha256", "f" * 64),
            "foreign-source": lambda r: r["sources"][0].__setitem__("commit", "f" * 40),
            "omitted-source": lambda r: r["sources"].pop(),
            "omitted-inline-token": lambda r: r["inlineInventory"].pop(),
            "fabricated-native": lambda r: r.__setitem__("nativeExecution", "PASSED"),
            "fabricated-closure": lambda r: r.__setitem__("formalAcceptanceGranted", True),
        }
        original = self.report()
        self.verify(original)
        for name, mutate in mutations.items():
            with self.subTest(name=name):
                changed = copy.deepcopy(original)
                mutate(changed)
                with self.assertRaises(ValueError):
                    self.verify(changed)

    def test_relabeling_old_cases_to_different_source_is_rejected(self):
        old = self.report()
        path = collector.DOCS_GUIDES[1]
        self.write(self.docs, path, self.valid.replace("PUBLIC_INVENTORY", "OTHER_PINNED_SOURCE"))
        self.docs_sha = self.commit(self.docs)
        self.pin_consumer()
        current = self.report()
        old.update({key: current[key] for key in ("implementationSha", "compilerCommit", "docsCommit",
                                                "manifestSha256", "sources")})
        with self.assertRaisesRegex(ValueError, "altered original example inventory"):
            self.verify(old)

    def test_undeclared_alternative_cannot_borrow_a_fence_identity(self):
        report = self.report()
        report["examples"][0]["variant"] = 0
        with self.assertRaisesRegex(ValueError, "alternative identity"):
            self.verify(report)


if __name__ == "__main__":
    unittest.main(verbosity=2)
