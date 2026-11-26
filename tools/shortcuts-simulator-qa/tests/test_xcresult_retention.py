"""Execute the actual shell retention boundary without invoking XCTest."""

import hashlib
import json
import os
import pathlib
import re
import subprocess
import tempfile
import unittest


REPO = pathlib.Path(__file__).resolve().parents[3]
SHELL = pathlib.Path(os.environ.get("CHERRI_RETENTION_TEST_SOURCE", REPO / "scripts/ios27_shortcuts_simulator_qa.sh"))
TEXT = SHELL.read_text(encoding="utf-8")
FUNCTION = "\n".join(re.findall(
    r"^(?:retain_xcresult_originals|cleanup_qa_scratch)\(\) \{\n.*?^\}\n",
    TEXT, re.M | re.S))


class XCTestRetentionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-xcresult-retention-")
        self.addCleanup(self.temp.cleanup)
        self.base = pathlib.Path(self.temp.name)
        self.root = self.base / "artifacts"
        self.scratch = self.base / "scratch"
        self.private = self.root / "builds/xcresult-originals"
        for p in (self.root / "tests", self.root / "builds", self.scratch):
            p.mkdir(parents=True, mode=0o700)

    def bundle(self, name="CherriSimulatorQA.xcresult", value=b"original XCTest bytes\0\xff"):
        p = self.scratch / name
        (p / "Data").mkdir(parents=True)
        (p / "Data/original").write_bytes(value)
        return p

    def run_shell(self, body, function=FUNCTION):
        env = dict(os.environ, ARTIFACT_DIR=str(self.root), QA_SCRATCH_DIR=str(self.scratch))
        result = subprocess.run(["/bin/bash", "-c", "set -uo pipefail\n" + function + "\n" + body],
                                stdin=subprocess.DEVNULL, capture_output=True, env=env, timeout=15)
        return result

    def receipts(self):
        return [json.loads(p.read_bytes()) for p in sorted(self.private.glob("retention-*/result.original.json"))]

    def normal_cleanup(self):
        anchor = "# Keep private build originals for hash-bound receipt validation."
        section = TEXT[TEXT.index(anchor):].split("# Bind the actual current invocation", 1)[0]
        return "\n".join(line for line in section.splitlines() if not line.lstrip().startswith("#"))

    def test_completed_bundles_survive_actual_cleanup_with_exact_bytes_and_modes(self):
        originals = {}
        for name in ("CherriSimulatorQA.xcresult", "CherriSimulatorQA-Part1.xcresult", "CherriSimulatorQAConfirm.xcresult"):
            p = self.bundle(name, name.encode() + b"\0private")
            (p / "Data/original").chmod(0o640)
            originals[name] = (p.stat().st_ino, (p / "Data/original").read_bytes())
        for name in ("xcresult-part1.json", "xcresult-part2.json", "xcodebuild-test.log"):
            (self.root / "tests" / name).write_bytes(name.encode() + b"\n")
        result = self.run_shell(self.normal_cleanup())
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertFalse(self.scratch.exists())
        for name, (inode, raw) in originals.items():
            p = self.private / name
            self.assertTrue(p.is_dir(), "raw XCTest original was discarded")
            self.assertEqual(p.stat().st_ino, inode)
            self.assertEqual((p / "Data/original").read_bytes(), raw)
            self.assertEqual((p / "Data/original").stat().st_mode & 0o777, 0o640)
        for name in ("xcresult-part1.json", "xcresult-part2.json", "xcodebuild-test.log"):
            self.assertEqual((self.private / name).read_bytes(), name.encode() + b"\n")
        receipt = self.receipts()[0]
        self.assertEqual(receipt["status"], "AVAILABLE_ORIGINALS_RETAINED")
        self.assertFalse(receipt["nativeCompletionInferred"])
        self.assertEqual(self.private.stat().st_mode & 0o777, 0o700)

    def test_failed_exit_trap_preserves_partial_bundle_and_original_exit(self):
        self.bundle(value=b"incomplete bundle is still original")
        trap = next(line.strip() for line in TEXT.splitlines() if line.strip().startswith("trap '") and "cleanup_handshake; seal_hosted_import" in line)
        body = "stop_http_probe(){ :; }; cleanup_handshake(){ :; }; seal_hosted_import(){ :; };\n" + trap + "\nexit 23\n"
        result = self.run_shell(body)
        self.assertEqual(result.returncode, 23, result.stderr)
        self.assertEqual((self.private / "CherriSimulatorQA.xcresult/Data/original").read_bytes(), b"incomplete bundle is still original")
        self.assertFalse(self.scratch.exists())

    def test_collision_rejects_without_clobber_or_scratch_cleanup(self):
        original = self.bundle(value=b"new original")
        self.private.mkdir(mode=0o700)
        existing = self.private / "CherriSimulatorQA.xcresult"
        existing.mkdir()
        (existing / "old").write_bytes(b"retained old original")
        result = self.run_shell(self.normal_cleanup())
        self.assertEqual(result.returncode, 70, result.stderr)
        self.assertEqual((original / "Data/original").read_bytes(), b"new original")
        self.assertEqual((existing / "old").read_bytes(), b"retained old original")
        self.assertEqual(self.receipts()[0]["error"], "RETENTION_DESTINATION_EXISTS")

    def test_original_symlink_rejected_and_foreign_target_untouched(self):
        foreign = self.base / "foreign"
        foreign.mkdir()
        (foreign / "original").write_bytes(b"untouched")
        (self.scratch / "CherriSimulatorQA.xcresult").symlink_to(foreign, target_is_directory=True)
        result = self.run_shell(self.normal_cleanup())
        self.assertEqual(result.returncode, 70, result.stderr)
        self.assertTrue((self.scratch / "CherriSimulatorQA.xcresult").is_symlink())
        self.assertEqual((foreign / "original").read_bytes(), b"untouched")

    def test_private_directory_symlink_rejected_without_foreign_write(self):
        original = self.bundle()
        foreign = self.base / "foreign"
        foreign.mkdir()
        self.private.symlink_to(foreign, target_is_directory=True)
        result = self.run_shell(self.normal_cleanup())
        self.assertEqual(result.returncode, 70, result.stderr)
        self.assertTrue(original.is_dir())
        self.assertEqual(list(foreign.iterdir()), [])

    def test_confirm_and_metadata_cleanup_retains_only_after_original_collection(self):
        self.assertRegex(TEXT, r'POST_TEST_INSPECTION_RC=\$\?.*?')
        start = TEXT.index('    PART1_JSON="$ARTIFACT_DIR/tests/xcresult-part1.json"')
        stop = TEXT.index("    # Extract receiver callback events", start)
        section = TEXT[start:stop]
        self.assertLess(section.index('qa_terminal_receipts.py" collect'), section.index("retain_xcresult_originals || exit 70"))
        self.assertNotIn('rm -rf "$XCRESULT_PATH"', section)
        start = TEXT.index('          CONFIRM_XCRESULT="$QA_SCRATCH_DIR/CherriSimulatorQAConfirm.xcresult"')
        stop = TEXT.index("          for poll_idx", start)
        section = TEXT[start:stop]
        self.assertLess(section.index("run_cmd_timeout 120 xcodebuild"), section.index("retain_xcresult_originals || exit 70"))
        self.assertNotIn('rm -rf "$CONFIRM_XCRESULT" "$ARTIFACT_DIR/editor/confirm-dialog-mode.txt"', section)

    def test_repeated_retention_keeps_old_objects_and_adds_new_confirmation(self):
        self.bundle(value=b"first")
        one = self.run_shell("retain_xcresult_originals")
        self.assertEqual(one.returncode, 0, one.stderr)
        self.bundle("CherriSimulatorQAConfirm.xcresult", b"confirmed")
        two = self.run_shell("retain_xcresult_originals")
        self.assertEqual(two.returncode, 0, two.stderr)
        self.assertEqual((self.private / "CherriSimulatorQA.xcresult/Data/original").read_bytes(), b"first")
        self.assertEqual((self.private / "CherriSimulatorQAConfirm.xcresult/Data/original").read_bytes(), b"confirmed")
        self.assertEqual(len(self.receipts()), 2)

    def test_normal_cleanup_then_installed_exit_cleanup_is_idempotent(self):
        self.bundle(value=b"completed original")
        trap = next(line.strip() for line in TEXT.splitlines() if line.strip().startswith("trap '") and "cleanup_handshake; seal_hosted_import" in line)
        body = "XCTEST_SCRATCH_CLEANED=false\nstop_http_probe(){ :; }; cleanup_handshake(){ :; }; seal_hosted_import(){ :; };\n" + trap + "\n" + self.normal_cleanup() + "\nexit 0\n"
        result = self.run_shell(body)
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stderr, b"", "completed cleanup must not become a false retention error at EXIT")
        self.assertFalse(self.scratch.exists())
        self.assertEqual((self.private / "CherriSimulatorQA.xcresult/Data/original").read_bytes(), b"completed original")

    def test_original_closing_receipt_io_still_has_the_retention_deadline(self):
        self.bundle()
        needle = "            os.fsync(stream.fileno())"
        self.assertEqual(FUNCTION.count(needle), 1)
        instrumented = FUNCTION.replace(needle,
            "            assert signal.getitimer(signal.ITIMER_REAL)[0] > 0, 'CLOSING_IO_OUTSIDE_RETENTION_DEADLINE'\n" + needle)
        result = self.run_shell("retain_xcresult_originals", function=instrumented)
        self.assertEqual(result.returncode, 0, result.stderr)


if __name__ == "__main__":
    unittest.main(verbosity=2)
