"""Actual observer controls using synthetic local files; never Apple evidence."""
import copy
import json
import os
from pathlib import Path
import shlex
import shutil
import subprocess
import sys
import tempfile
import unittest

import runtime_poc_coverage as coverage
import runtime_poc_publication as publication

REPO = Path(__file__).resolve().parents[2]
UDID = "12345678-1234-1234-1234-1234567890ab"


def create_coverage_control(destination, source_repo=REPO):
    """Make a real temporary Git checkout and header-only signing control."""
    destination = Path(destination)
    repo, root = destination / "repo", destination / "artifacts"
    repo.mkdir(parents=True)
    root.mkdir()
    paths = ["tests/runtime_poc/" + name + ".cherri" for name in publication.SOURCE_HASHES]
    paths += [coverage.EXPECTED_PATH]
    for relative in paths:
        target = repo / relative
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes((Path(source_repo) / relative).read_bytes())
    helper = repo / "tools/shortcuts-simulator-qa/runtime_poc_coverage.py"
    helper.parent.mkdir(parents=True)
    helper.write_bytes(Path(coverage.__file__).read_bytes())
    hooks = destination / "empty-hooks"
    hooks.mkdir()
    subprocess.run(["git", "init", "-q", str(repo)], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(repo), "config", "core.autocrlf", "false"], check=True)
    subprocess.run(["git", "-C", str(repo), "add", "."], check=True, capture_output=True)
    subprocess.run(["git", "-C", str(repo), "-c", "user.name=P15 observer control",
                    "-c", "user.email=control@example.invalid", "-c", "core.hooksPath=" + str(hooks),
                    "commit", "-qm", "Synthetic observer control"], check=True, capture_output=True)
    commit = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
    control = {"repo": repo, "root": root, "commit": commit, "run_id": "900000005", "attempt": "1"}
    publication.check_sources(repo, root, commit, control["run_id"], control["attempt"])
    write_control_identity(control)
    return control


def write_control_identity(control):
    """Write explicit synthetic identity inputs for the real observer functions."""
    root, repo = Path(control["root"]), Path(control["repo"])
    rows = []
    for name, digest in publication.SOURCE_HASHES.items():
        (root / (name + ".cherri")).write_bytes((repo / "tests/runtime_poc" / (name + ".cherri")).read_bytes())
        signed = b"AEA1-SYNTHETIC-OBSERVER-CONTROL-NOT-APPLE-VALIDITY-" + name.encode()
        (root / (name + ".shortcut")).write_bytes(signed)
        rows.append({"fixtureId": name, "sourceSha256": digest,
                     "unsignedSha256": "d" * 64, "signedSha256": publication.sha(signed),
                     "signedArtifact": name + ".shortcut", "signedMagicVerified": True,
                     "signingMethod": "native"})
    base = {"implementationSha": control["commit"], "runId": control["run_id"], "runAttempt": control["attempt"]}
    (root / publication.SIGNING_RECEIPT).write_bytes(coverage.encoded({**base, "fixtures": rows}))
    (root / publication.DEVICE_RECEIPT).write_bytes(coverage.encoded({
        **base, "createdByRun": True, "cleanupStatus": "PENDING", "udid": UDID,
    }))


def args(control):
    return (control["repo"], control["root"], control["commit"], control["run_id"], control["attempt"])


def mark_deleted(control):
    path = Path(control["root"]) / publication.DEVICE_RECEIPT
    value = json.loads(path.read_bytes())
    value["cleanupStatus"] = "DELETED"
    path.write_bytes(coverage.encoded(value))


def write_control_observations(control, observed=None):
    """Record synthetic bytes through actual prepare/reset/result entry points."""
    common = args(control)
    root = Path(control["root"])
    coverage.prepare(*common)
    (root / coverage.RESET_OBSERVED).write_bytes((root / coverage.RESET_REQUEST).read_bytes())
    coverage.record_reset(*common)
    if observed is None:
        expected = json.loads((Path(control["repo"]) / coverage.EXPECTED_PATH).read_bytes())
        observed = expected["expected_clipboard"].encode("utf-8")
    (root / coverage.OBSERVED).write_bytes(observed)
    result = coverage.observe(*common, write_result=True)
    mark_deleted(control)
    return result


class CoverageObservationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="cherri-coverage-control-")
        self.addCleanup(self.temp.cleanup)
        self.control = create_coverage_control(self.temp.name)
        self.common = args(self.control)
        self.root = self.control["root"]
        self.expected = json.loads((self.control["repo"] / coverage.EXPECTED_PATH).read_bytes())["expected_clipboard"].encode("utf-8")

    def test_exact_bytes_reset_identity_and_deleted_device_replay(self):
        result = write_control_observations(self.control)
        before = {p.name: p.read_bytes() for p in self.root.iterdir() if p.is_file()}
        self.assertEqual(coverage.verify_result(*self.common), result)
        self.assertEqual(result["result"], "passed")
        self.assertEqual(result["observed_byte_count"], 313)
        self.assertEqual(result["observed_clipboard_sha256"], "f2e79540f2c95547ac9c83d59f25b63ce4167153d81731a5e00b746f041037f9")
        self.assertEqual(before, {p.name: p.read_bytes() for p in self.root.iterdir() if p.is_file()})

    def test_wrong_newline_unicode_trace_and_missing_output_stay_failed(self):
        write_control_observations(self.control)
        for observed in (b"", b"BEFORE_S01_F01", self.expected + b"\n",
                         self.expected.replace("👩‍💻".encode(), b"?"),
                         self.expected.replace(b"|OR_TRUE|OR_FALSE", b"|OR_FALSE|OR_TRUE"),
                         self.expected.replace(b"outer_or=T", b"outer_or=F")):
            with self.subTest(observed=observed[:25]):
                (self.root / coverage.OBSERVED).write_bytes(observed)
                result = coverage.observe(*self.common, allow_deleted=True)
                self.assertEqual((result["result"], result["exit_code"], result["exact_output_match"]), ("failed", 1, False))
                (self.root / coverage.RESULT).write_bytes(coverage.encoded(result))
                self.assertEqual(coverage.verify_result(*self.common), result)

    def test_stale_reset_observation_blocks_even_exact_final_output(self):
        coverage.prepare(*self.common)
        (self.root / coverage.RESET_OBSERVED).write_bytes(b"BEFORE_S01_F01_STALE_NONCE")
        (self.root / coverage.OBSERVED).write_bytes(self.expected)
        with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_FRESH_RESET_NOT_OBSERVED"):
            coverage.record_reset(*self.common)
        self.assertFalse((self.root / coverage.RESET_PROOF).exists())

    def test_missing_reset_proof_cannot_be_replaced_by_golden_output(self):
        coverage.prepare(*self.common)
        (self.root / coverage.OBSERVED).write_bytes(self.expected)
        with self.assertRaises(publication.PublicationBlocked):
            coverage.observe(*self.common, write_result=True)
        self.assertFalse((self.root / coverage.RESULT).exists())

    def test_context_run_source_device_signing_and_reset_mutations_reject(self):
        write_control_observations(self.control)
        changes = [
            (coverage.CONTEXT, "implementation_sha", "f" * 40),
            (coverage.CONTEXT, "run_id", "900000006"),
            (coverage.CONTEXT, "run_attempt", "2"),
            (coverage.CONTEXT, "source_sha256", "f" * 64),
            (coverage.CONTEXT, "device_udid", "22345678-1234-1234-1234-1234567890ab"),
            (coverage.CONTEXT, "signed_artifact_sha256", "f" * 64),
            (coverage.RESET_PROOF, "fresh_reset_observed", False),
            (coverage.RESET_PROOF, "reset_observed_sha256", "f" * 64),
        ]
        for name, key, value in changes:
            with self.subTest(name=name, key=key):
                path = self.root / name
                original = path.read_bytes()
                changed = json.loads(original)
                changed[key] = value
                path.write_bytes(coverage.encoded(changed))
                with self.assertRaises(publication.PublicationBlocked):
                    coverage.verify_result(*self.common)
                path.write_bytes(original)
        path = self.root / (coverage.FIXTURE + ".shortcut")
        path.write_bytes(path.read_bytes() + b"changed")
        with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_SIGNED_ARTIFACT_MISMATCH"):
            coverage.verify_result(*self.common)

    def test_recorded_result_cannot_be_upgraded_or_detached(self):
        write_control_observations(self.control, b"native failure observation")
        path = self.root / coverage.RESULT
        value = json.loads(path.read_bytes())
        value.update(result="passed", exit_code=0, exact_output_match=True)
        path.write_bytes(coverage.encoded(value))
        with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_RESULT_REPLAY_MISMATCH"):
            coverage.verify_result(*self.common)

    def test_final_replay_requires_successful_owned_device_cleanup(self):
        write_control_observations(self.control)
        path = self.root / publication.DEVICE_RECEIPT
        value = json.loads(path.read_bytes())
        for state in ("PENDING", "FAILED"):
            value["cleanupStatus"] = state
            path.write_bytes(coverage.encoded(value))
            with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_OWNED_RUNNING_DEVICE_REQUIRED"):
                coverage.verify_result(*self.common)

    def test_changed_expected_contract_is_not_accepted(self):
        path = self.control["repo"] / coverage.EXPECTED_PATH
        path.write_bytes(path.read_bytes() + b"\n")
        with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_EXPECTED_CONTRACT_CHANGED"):
            coverage.prepare(*self.common)
        self.assertFalse((self.root / coverage.CONTEXT).exists())

    def test_existing_context_is_never_reused(self):
        first = coverage.prepare(*self.common)
        with self.assertRaisesRegex(publication.PublicationBlocked, "COVERAGE_OUTPUT_MUST_BE_NEW"):
            coverage.prepare(*self.common)
        self.assertEqual(json.loads((self.root / coverage.CONTEXT).read_bytes()), first)

    def test_actual_cli_failure_writes_failed_receipt_and_returns_one(self):
        coverage.prepare(*self.common)
        (self.root / coverage.RESET_OBSERVED).write_bytes((self.root / coverage.RESET_REQUEST).read_bytes())
        coverage.record_reset(*self.common)
        (self.root / coverage.OBSERVED).write_bytes(b"observed synthetic failure")
        command = [sys.executable, str(Path(coverage.__file__)), "record-result",
                   "--repo", str(self.control["repo"]), "--root", str(self.root),
                   "--commit", self.control["commit"], "--run-id", self.control["run_id"], "--attempt", "1"]
        result = subprocess.run(command, capture_output=True, text=True, encoding="utf-8")
        self.assertEqual(result.returncode, 1, result.stdout + result.stderr)
        self.assertEqual(json.loads(result.stdout)["coverage_result"], "failed")
        mark_deleted(self.control)
        self.assertEqual(coverage.verify_result(*self.common)["result"], "failed")


class ActualCoverageShellTests(unittest.TestCase):
    """Execute the checked-in coverage block with local Apple-command controls."""
    def run_block(self, output_suffix=b"", stale_reset=False):
        with tempfile.TemporaryDirectory(prefix="cherri-coverage-shell-control-") as temp:
            base = Path(temp)
            control = create_coverage_control(base)
            script = (REPO / "scripts/ios27_runtime_poc.sh").read_text(encoding="utf-8")
            start = '# [9] FIXTURE 5: precise Unicode attachments and mixed Boolean effects'
            end = '# [10] COLLECT ARTIFACTS & GENERATE EVIDENCE JSON'
            self.assertEqual(script.count(start), 1)
            self.assertEqual(script.count(end), 1)
            block = script.split(start, 1)[1].split(end, 1)[0]
            source_dir = base / "tests/runtime_poc"
            source_dir.mkdir(parents=True)
            shutil.copyfile(REPO / "tests/runtime_poc" / (coverage.FIXTURE + ".cherri"),
                            source_dir / (coverage.FIXTURE + ".cherri"))
            compiler = base / "cherri"
            compiler.write_bytes(b'#!/bin/sh\nprintf synthetic-unsigned > tests/runtime_poc/CherriS01F01CoveragePOC_unsigned.shortcut\n')
            compiler.chmod(0o700)
            expected = json.loads((control["repo"] / coverage.EXPECTED_PATH).read_bytes())["expected_clipboard"].encode("utf-8")
            (base / "synthetic-output.txt").write_bytes(expected + output_suffix)
            (base / "synthetic-clipboard.txt").write_bytes(b"stale synthetic clipboard")
            bash = shutil.which("bash")
            if os.name == "nt":
                git = Path(shutil.which("git"))
                candidate = git.parent.parent / "bin/bash.exe"
                if candidate.is_file():
                    bash = str(candidate)
            self.assertTrue(bash, "Bash is required for actual shell controls")
            q = lambda path: shlex.quote(Path(path).as_posix())
            prefix = "set -e\n"
            prefix += "POC_REPO_ROOT=" + q(control["repo"]) + "\n"
            prefix += "POC_TOOLS=" + q(Path(coverage.__file__).parent) + "\n"
            prefix += "SIM_UDID=" + UDID + "\n"
            prefix += "python3() { " + q(sys.executable) + ' "$@"; }\n'
            prefix += "reset_shortcuts_app() { :; }\nsign_and_verify() { :; }\nimport_and_confirm() { :; }\nrun_ui_helper() { :; }\nsleep() { :; }\nseq() { printf '1\\n'; }\n"
            prefix += "xcrun() {\n printf '%s\\n' \"$2\" >> synthetic-calls.txt\n case \"$2\" in\n"
            prefix += "  pbcopy) " + ("cat >/dev/null" if stale_reset else "cat > synthetic-clipboard.txt") + ";;\n"
            prefix += "  pbpaste) cat synthetic-clipboard.txt;;\n  openurl) cat synthetic-output.txt > synthetic-clipboard.txt;;\n  *) return 88;;\n esac\n}\n"
            harness = base / "harness.sh"
            harness.write_bytes((prefix + block + '\nprintf "%s" "$COVERAGE_PASSED" > synthetic-status.txt\n').encode("utf-8"))
            env = dict(os.environ, GITHUB_SHA=control["commit"], GITHUB_RUN_ID=control["run_id"], GITHUB_RUN_ATTEMPT=control["attempt"])
            result = subprocess.run([bash, str(harness)], cwd=base, env=env,
                                    capture_output=True, text=True, encoding="utf-8", timeout=90)
            calls = (base / "synthetic-calls.txt").read_text().splitlines()
            if stale_reset:
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn("openurl", calls)
                self.assertFalse((control["root"] / coverage.RESULT).exists())
            else:
                self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
                self.assertEqual(calls[:3], ["pbcopy", "pbpaste", "openurl"])
                self.assertEqual((control["root"] / coverage.OBSERVED).read_bytes(), expected + output_suffix)
                self.assertEqual((base / "synthetic-status.txt").read_text(), "false" if output_suffix else "true")
                mark_deleted(control)
                observed = coverage.verify_result(*args(control))
                self.assertEqual(observed["result"], "failed" if output_suffix else "passed")

    def test_actual_shell_preserves_exact_native_observation_bytes(self):
        self.run_block()

    def test_actual_shell_does_not_hide_trailing_newline_failure(self):
        self.run_block(output_suffix=b"\n")

    def test_actual_shell_rejects_stale_reset_before_opening_shortcut(self):
        self.run_block(stale_reset=True)


if __name__ == "__main__":
    unittest.main()
