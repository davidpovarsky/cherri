"""Actual shell orchestration controls; stubs never contact Apple or a signer."""
import os
from pathlib import Path
import re
import shutil
import subprocess
import tempfile
import unittest

REPO = Path(__file__).resolve().parents[2]


def function_source(name):
    source = (REPO / "scripts/ios27_runtime_poc.sh").read_text(encoding="utf-8")
    match = re.search(r"(?m)^" + name + r"\(\) \{\n.*?^\}", source, re.S)
    if not match:
        raise AssertionError("missing actual shell function: " + name)
    return match.group(0)


class FormalPoCShellTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)
        self.bash = shutil.which("bash")
        if os.name == "nt" and shutil.which("git"):
            git_bash = Path(shutil.which("git")).resolve().parents[1] / "bin/bash.exe"
            if git_bash.is_file():
                self.bash = str(git_bash)
        self.assertIsNotNone(self.bash, "bash is required for the actual shell controls")
        (self.root / "artifacts").mkdir()
        (self.root / "artifacts/unsigned.shortcut").write_bytes(b"PUBLIC_SYNTHETIC_UNSIGNED_CONTROL")
        signer = """#!/usr/bin/env bash
printf 'remote' >> events.log
printf '\\t%s' "$@" >> events.log
printf '\\n' >> events.log
[ "$1" = sign ] || exit 70
case " $* " in *" --hubsign "*) ;; *) exit 71;; esac
if [ "${REMOTE_EXIT:-0}" != 0 ]; then exit "$REMOTE_EXIT"; fi
for arg in "$@"; do
  case "$arg" in --output=*) printf 'AEA1SYNTHETIC_SHELL_CONTROL' > "${arg#--output=}";; esac
done
"""
        (self.root / "cherri").write_bytes(signer.encode("utf-8"))
        (self.root / "cherri").chmod(0o755)

    def run_shell(self, body, **env):
        controls = r"""
set -e
POC_REPO_ROOT="$PWD"
POC_TOOLS="$PWD/tools"
LOG_PID=""
HTTP_PID=""
SIM_UDID=""
OWNED_SIM_UDID="SYNTHETIC_RUN_OWNED_UDID"
python3() {
  printf 'python\t%s\n' "$2" >> events.log
  if [ "$2" = check-sources ]; then return "${SOURCE_GATE_EXIT:-0}"; fi
  if [ "$2" = cleanup ]; then return "${CLEANUP_EXIT:-0}"; fi
}
git() { printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\n'; }
shortcuts() {
  printf 'native\n' >> events.log
  if [ "${NATIVE_EXIT:-1}" = 0 ]; then
    printf 'AEA1SYNTHETIC_NATIVE_CONTROL' > artifacts/signed.shortcut
  fi
  return "${NATIVE_EXIT:-1}"
}
take_failure_screenshot() { :; }
"""
        script = controls + "\n" + body
        path = self.root / "control.sh"
        path.write_bytes(script.encode("utf-8"))
        result = subprocess.run([self.bash, "control.sh"], cwd=self.root,
                                env={**os.environ, **env}, capture_output=True,
                                text=True, encoding="utf-8", timeout=45)
        events = (self.root / "events.log").read_text(encoding="utf-8") if (self.root / "events.log").exists() else ""
        return result, events

    def sign(self, **env):
        body = function_source("sign_and_verify") + """
sign_and_verify artifacts/unsigned.shortcut artifacts/signed.shortcut tests/runtime_poc/CherriRuntimePOC.cherri
"""
        return self.run_shell(body, **env)

    def test_default_failure_never_calls_external_signer(self):
        result, events = self.sign(CHERRI_ALLOW_EXTERNAL_SIGNING="0")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertNotIn("remote", events)
        self.assertFalse((self.root / "artifacts/signed.shortcut").exists())

    def test_explicit_public_opt_in_signs_exact_unsigned_file(self):
        result, events = self.sign(CHERRI_ALLOW_EXTERNAL_SIGNING="1")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("python\tcheck-sources\n", events)
        self.assertLess(events.index("python\tcheck-sources"), events.index("remote\t"))
        remote = next(line for line in events.splitlines() if line.startswith("remote\t")).split("\t")[1:]
        self.assertEqual(remote[:2], ["sign", "artifacts/unsigned.shortcut"])
        self.assertIn("--hubsign", remote)
        self.assertIn("--output=artifacts/signed.shortcut", remote)
        self.assertIn("--source-file=tests/runtime_poc/CherriRuntimePOC.cherri", remote)
        self.assertIn("--compiler-sha=" + "a" * 40, remote)
        self.assertTrue(any(arg.startswith("--manifest-out=") for arg in remote))
        self.assertNotIn("-d", remote, "fallback must not recompile or retry implicit native signing")
        self.assertIn("python\trecord-signing", events)

    def test_source_gate_failure_prevents_external_call(self):
        result, events = self.sign(CHERRI_ALLOW_EXTERNAL_SIGNING="1", SOURCE_GATE_EXIT="2")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertNotIn("remote", events)
        self.assertNotIn("record-signing", events)

    def test_failed_remote_signing_cannot_record_success(self):
        result, events = self.sign(CHERRI_ALLOW_EXTERNAL_SIGNING="1", REMOTE_EXIT="9")
        self.assertNotEqual(result.returncode, 0)
        self.assertIn("remote\tsign", events)
        self.assertNotIn("record-signing", events)

    def test_native_success_needs_no_external_opt_in(self):
        result, events = self.sign(CHERRI_ALLOW_EXTERNAL_SIGNING="0", NATIVE_EXIT="0")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertNotIn("remote", events)
        self.assertIn("python\trecord-signing", events)

    def test_manual_cleanup_continues_to_evidence_generation(self):
        result, events = self.run_shell(function_source("cleanup") + """
cleanup
printf 'EVIDENCE_GENERATION_REACHED\\n'
""")
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        self.assertIn("EVIDENCE_GENERATION_REACHED", result.stdout)
        self.assertEqual(events.splitlines(), ["python\tcleanup"])

    def test_exit_trap_preserves_original_failure(self):
        source = (REPO / "scripts/ios27_runtime_poc.sh").read_text(encoding="utf-8")
        trap_line = next(line for line in source.splitlines() if line.startswith("trap ") and line.endswith(" EXIT"))
        result, events = self.run_shell(function_source("cleanup") + "\n" + trap_line + "\nexit 7\n")
        self.assertEqual(result.returncode, 7, result.stdout + result.stderr)
        self.assertEqual(events.splitlines(), ["python\tcleanup"])

    def test_cleanup_failure_prevents_successful_evidence_generation(self):
        result, events = self.run_shell(function_source("cleanup") + """
cleanup
printf 'EVIDENCE_GENERATION_REACHED\\n'
""", CLEANUP_EXIT="2")
        self.assertEqual(result.returncode, 2, result.stdout + result.stderr)
        self.assertNotIn("EVIDENCE_GENERATION_REACHED", result.stdout)
        self.assertEqual(events.splitlines(), ["python\tcleanup"])


if __name__ == "__main__":
    unittest.main()
